package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/egekocabas/kick-sim/assets"
	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/goccy/go-yaml"
)

var scenarioSegment = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

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
}

func NewStore(workspaceRoot string, registry *events.Registry, configuration config.Config) *Store {
	return &Store{workspaceRoot: workspaceRoot, registry: registry, configuration: configuration}
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
	entries, err := store.List()
	if err != nil {
		return Entry{}, err
	}
	for _, entry := range entries {
		if entry.ID == id {
			return entry, nil
		}
	}
	return Entry{}, fmt.Errorf("scenario %q was not found", id)
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
		value.Request.Payload = events.DeepCopyMap(payload)
		value.Request.Payload["message_id"] = "{{ ulid() }}"
		value.Request.Payload["created_at"] = "{{ now() }}"
	}
	value.Metadata = Metadata{Source: sourceID, SourceVersion: source.SourceVersion, CreatedWith: createdWith}
	if err := value.Validate(store.registry, store.configuration); err != nil {
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
	return entry.Scenario.Validate(store.registry, store.configuration)
}

func ValidateID(id string) error {
	if id == "" || strings.Contains(id, "\\") || filepath.IsAbs(id) || filepath.VolumeName(id) != "" {
		return fmt.Errorf("invalid scenario ID %q", id)
	}
	for _, segment := range strings.Split(id, "/") {
		if !scenarioSegment.MatchString(segment) {
			return fmt.Errorf("invalid scenario ID segment %q", segment)
		}
	}
	return nil
}

func (store *Store) builtIns() ([]Entry, error) {
	var entries []Entry
	err := fs.WalkDir(assets.Files, "scenarios", func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		data, err := assets.Files.ReadFile(path)
		if err != nil {
			return err
		}
		value, err := parse(data)
		if err != nil {
			return fmt.Errorf("parse built-in %s: %w", path, err)
		}
		id := "builtin:" + strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(path), "scenarios/"), ".yaml")
		entries = append(entries, newEntry(id, true, "", value, data, 1))
		return nil
	})
	return entries, err
}

func (store *Store) custom() ([]Entry, error) {
	root := filepath.Join(store.workspaceRoot, "scenarios")
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect scenarios directory: %w", err)
	}

	seen := map[string]string{}
	var entries []Entry
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("scenario paths may not contain symlinks: %s", path)
		}
		if item.IsDir() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".yaml" && extension != ".yml" && extension != ".json" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		id := strings.TrimSuffix(filepath.ToSlash(relative), extension)
		if err := ValidateID(id); err != nil {
			return err
		}
		if prior, duplicate := seen[id]; duplicate {
			return fmt.Errorf("scenario ID %q is defined by both %s and %s", id, prior, path)
		}
		seen[id] = path
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		value, validationErr := store.parseAndValidate(data)
		entry := newEntry(id, false, path, value, data, 1)
		if validationErr != nil {
			entry.validationError = validationErr
			entry.ValidationErrors = errorMessages(validationErr)
		}
		entries = append(entries, entry)
		return nil
	})
	return entries, err
}

func newEntry(id string, builtIn bool, path string, value Scenario, source []byte, sourceVersion int) Entry {
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if format == "" || format == "yml" {
		format = "yaml"
	}
	return Entry{
		ID:            id,
		BuiltIn:       builtIn,
		Path:          path,
		Scenario:      value,
		Source:        append([]byte(nil), source...),
		SourceFormat:  format,
		Revision:      revision(source),
		SourceVersion: sourceVersion,
	}
}

func (store *Store) parseAndValidate(data []byte) (Scenario, error) {
	value, err := parse(data)
	if err != nil {
		return Scenario{}, &SourceValidationError{Problems: []string{err.Error()}}
	}
	if err := value.Validate(store.registry, store.configuration); err != nil {
		return value, &SourceValidationError{Problems: errorMessages(err)}
	}
	return value, nil
}

func errorMessages(err error) []string {
	if problem, ok := err.(*SourceValidationError); ok {
		return append([]string(nil), problem.Problems...)
	}
	type joined interface{ Unwrap() []error }
	if group, ok := err.(joined); ok {
		var messages []string
		for _, child := range group.Unwrap() {
			messages = append(messages, errorMessages(child)...)
		}
		return messages
	}
	return []string{err.Error()}
}

func revision(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func parse(data []byte) (Scenario, error) {
	var value Scenario
	if err := yaml.UnmarshalWithOptions(data, &value, yaml.Strict()); err != nil {
		return Scenario{}, err
	}
	if value.Request.Payload == nil {
		value.Request.Payload = map[string]any{}
	}
	return value, nil
}

func ensureNoSymlinkComponents(root, target string) error {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect scenarios root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("scenarios directory may not be a symlink")
	}
	relative, err := filepath.Rel(root, filepath.Dir(target))
	if err != nil {
		return err
	}
	current := root
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		if segment == "." || segment == "" {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("scenario path may not contain symlink %s", current)
		}
	}
	return nil
}

func (store *Store) customPath(id, extension string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	root := filepath.Join(store.workspaceRoot, "scenarios")
	path := filepath.Join(root, filepath.FromSlash(id)+extension)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("scenario path escapes the workspace")
	}
	return path, nil
}

func rejectScenarioCollision(base string) error {
	for _, extension := range []string{".yaml", ".yml", ".json"} {
		if _, err := os.Lstat(base + extension); err == nil {
			return fmt.Errorf("scenario already exists: %s", base+extension)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (store *Store) writeNew(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create scenario directory: %w", err)
	}
	if err := ensureNoSymlinkComponents(filepath.Join(store.workspaceRoot, "scenarios"), path); err != nil {
		return err
	}
	return writeAtomicNew(path, data, 0o644)
}

func writeAtomicNew(path string, data []byte, mode os.FileMode) error {
	temporaryPath, err := writeTemporary(filepath.Dir(path), data, mode)
	if err != nil {
		return err
	}
	defer os.Remove(temporaryPath)
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("create scenario: %w", err)
	}
	return syncDirectory(filepath.Dir(path))
}

func writeAtomicReplace(path string, data []byte, mode os.FileMode, expectedRevision string) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read current scenario: %w", err)
	}
	actualRevision := revision(current)
	if actualRevision != expectedRevision {
		return &RevisionConflictError{Expected: expectedRevision, Actual: actualRevision}
	}
	temporaryPath, err := writeTemporary(filepath.Dir(path), data, mode)
	if err != nil {
		return err
	}
	defer os.Remove(temporaryPath)

	current, err = os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("re-read current scenario: %w", err)
	}
	actualRevision = revision(current)
	if actualRevision != expectedRevision {
		return &RevisionConflictError{Expected: expectedRevision, Actual: actualRevision}
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace scenario: %w", err)
	}
	return syncDirectory(filepath.Dir(path))
}

func writeTemporary(directory string, data []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(directory, ".kick-sim-scenario-*")
	if err != nil {
		return "", fmt.Errorf("create temporary scenario: %w", err)
	}
	path := file.Name()
	failed := true
	defer func() {
		if failed {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(mode); err != nil {
		return "", fmt.Errorf("set temporary scenario permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("write temporary scenario: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary scenario: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close temporary scenario: %w", err)
	}
	failed = false
	return path, nil
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open scenario directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync scenario directory: %w", err)
	}
	return nil
}
