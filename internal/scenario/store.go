// Package scenario loads, validates, and safely edits workspace scenario sources.
package scenario

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/goccy/go-yaml"
)

type Entry struct {
	ID               string   `json:"id"`
	BuiltIn          bool     `json:"builtIn"`
	Path             string   `json:"path,omitempty"`
	Scenario         Scenario `json:"scenario"`
	Source           []byte   `json:"-"`
	SourceFormat     string   `json:"sourceFormat"`
	Revision         string   `json:"revision"`
	SourceVersion    int      `json:"sourceVersion"`
	ValidationErrors []string `json:"validationErrors,omitempty"`
	validationError  error
}

type RevisionConflictError struct {
	Expected string
	Actual   string
}

func (problem *RevisionConflictError) Error() string {
	return fmt.Sprintf("scenario changed on disk: expected revision %s, found %s", problem.Expected, problem.Actual)
}

func (problem *RevisionConflictError) HTTPStatus() int { return 409 }

type SourceValidationError struct {
	Problems []string
}

func (problem *SourceValidationError) Error() string {
	return "scenario source is invalid: " + strings.Join(problem.Problems, "; ")
}

type Store struct {
	workspaceRoot string
	registry      *events.Registry
	configuration config.Config
	actors        *actors.Registry
}

func NewStore(workspaceRoot string, registry *events.Registry, configuration config.Config, actorRegistries ...*actors.Registry) *Store {
	actorRegistry := actors.Empty()
	if len(actorRegistries) > 0 && actorRegistries[0] != nil {
		actorRegistry = actorRegistries[0]
	}
	return &Store{workspaceRoot: workspaceRoot, registry: registry, configuration: config.Clone(configuration), actors: actorRegistry}
}

func (store *Store) List() ([]Entry, error) {
	builtIns, err := store.builtIns()
	if err != nil {
		return nil, err
	}
	custom, err := store.custom()
	if err != nil {
		return nil, err
	}
	entries := append(builtIns, custom...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

func (store *Store) Get(id string) (Entry, error) {
	if strings.HasPrefix(id, "builtin:") {
		return store.getBuiltIn(strings.TrimPrefix(id, "builtin:"))
	}
	return store.getCustom(id)
}

func (store *Store) Copy(sourceID, targetID, createdWith string) (Entry, error) {
	return store.SaveAsCopy(sourceID, targetID, nil, createdWith)
}

func (store *Store) SaveAsCopy(sourceID, targetID string, payload map[string]any, createdWith string) (Entry, error) {
	if strings.HasPrefix(targetID, "builtin:") {
		return Entry{}, errors.New("custom scenario ID cannot use the reserved builtin: prefix")
	}
	if err := ValidateID(targetID); err != nil {
		return Entry{}, err
	}
	source, err := store.Get(sourceID)
	if err != nil {
		return Entry{}, err
	}
	targetPath, err := store.customPath(targetID, ".yaml")
	if err != nil {
		return Entry{}, err
	}
	if err := rejectScenarioCollision(strings.TrimSuffix(targetPath, ".yaml")); err != nil {
		return Entry{}, err
	}

	value := source.Scenario
	if payload != nil {
		if value.Kind() != "single" {
			return Entry{}, errors.New("timeline scenarios must be copied from their complete source")
		}
		value.Request.Payload = events.DeepCopyMap(payload)
		value.Request.Payload["message_id"] = "{{ ulid() }}"
		value.Request.Payload["created_at"] = "{{ now() }}"
	}
	value.Metadata = Metadata{Source: sourceID, SourceVersion: source.SourceVersion, CreatedWith: createdWith}
	if err := value.Validate(store.registry, store.configuration, store.actors); err != nil {
		return Entry{}, err
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return Entry{}, fmt.Errorf("marshal scenario copy: %w", err)
	}
	if err := store.writeNew(targetPath, data); err != nil {
		return Entry{}, err
	}
	return newEntry(targetID, false, targetPath, value, data, 1), nil
}

func (store *Store) SaveSource(id, expectedRevision string, source []byte) (Entry, error) {
	if strings.HasPrefix(id, "builtin:") {
		return Entry{}, errors.New("built-in scenarios are read-only")
	}
	entry, err := store.Get(id)
	if err != nil {
		return Entry{}, err
	}
	if entry.BuiltIn {
		return Entry{}, errors.New("built-in scenarios are read-only")
	}
	value, err := store.parseAndValidate(source)
	if err != nil {
		return Entry{}, err
	}
	info, err := os.Lstat(entry.Path)
	if err != nil {
		return Entry{}, fmt.Errorf("inspect scenario: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Entry{}, errors.New("scenario source must be a regular file")
	}
	if err := ensureNoSymlinkComponents(filepath.Join(store.workspaceRoot, "scenarios"), entry.Path); err != nil {
		return Entry{}, err
	}
	if err := writeAtomicReplace(entry.Path, source, info.Mode().Perm(), expectedRevision); err != nil {
		return Entry{}, err
	}
	return newEntry(id, false, entry.Path, value, source, entry.SourceVersion), nil
}

func (store *Store) SaveSourceAsCopy(sourceID, targetID string, source []byte, createdWith string) (Entry, error) {
	if strings.HasPrefix(targetID, "builtin:") {
		return Entry{}, errors.New("custom scenario ID cannot use the reserved builtin: prefix")
	}
	if err := ValidateID(targetID); err != nil {
		return Entry{}, err
	}
	value, err := store.parseAndValidate(source)
	if err != nil {
		return Entry{}, err
	}
	value.Metadata = Metadata{Source: sourceID, SourceVersion: 1, CreatedWith: createdWith}
	data, err := yaml.Marshal(value)
	if err != nil {
		return Entry{}, fmt.Errorf("marshal scenario copy: %w", err)
	}
	targetPath, err := store.customPath(targetID, ".yaml")
	if err != nil {
		return Entry{}, err
	}
	if err := rejectScenarioCollision(strings.TrimSuffix(targetPath, ".yaml")); err != nil {
		return Entry{}, err
	}
	if err := store.writeNew(targetPath, data); err != nil {
		return Entry{}, err
	}
	return newEntry(targetID, false, targetPath, value, data, 1), nil
}

func (store *Store) Validate(entry Entry) error {
	if entry.validationError != nil {
		return entry.validationError
	}
	return entry.Scenario.Validate(store.registry, store.configuration, store.actors)
}
