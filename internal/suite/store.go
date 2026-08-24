package suite

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/egekocabas/kick-sim/assets"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/goccy/go-yaml"
)

var idSegment = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

type Entry struct {
	ID      string `json:"id"`
	BuiltIn bool   `json:"builtIn"`
	Path    string `json:"path,omitempty"`
	Suite   Suite  `json:"suite"`
	Source  []byte `json:"-"`
}

type Store struct {
	workspaceRoot string
	scenarios     *scenario.Store
}

func NewStore(workspaceRoot string, scenarios *scenario.Store) *Store {
	return &Store{workspaceRoot: workspaceRoot, scenarios: scenarios}
}

func (store *Store) List() ([]Entry, error) {
	entries, err := store.builtIns()
	if err != nil {
		return nil, err
	}
	custom, err := store.custom()
	if err != nil {
		return nil, err
	}
	entries = append(entries, custom...)
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
	return Entry{}, fmt.Errorf("suite %q was not found", id)
}

func (store *Store) Validate(entry Entry) error { return entry.Suite.Validate(store.scenarios) }

func (store *Store) builtIns() ([]Entry, error) {
	var entries []Entry
	err := fs.WalkDir(assets.Files, "suites", func(path string, item fs.DirEntry, walkErr error) error {
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
		id := "builtin:" + strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(path), "suites/"), ".yaml")
		entries = append(entries, Entry{ID: id, BuiltIn: true, Suite: value, Source: data})
		return nil
	})
	return entries, err
}

func (store *Store) custom() ([]Entry, error) {
	root := workspace.PathsFor(store.workspaceRoot).Suites
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var entries []Entry
	err := filepath.WalkDir(root, func(path string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("suite paths may not contain symlinks: %s", path)
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
		if err := validateID(id); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		value, err := parse(data)
		if err != nil {
			return fmt.Errorf("parse suite %s: %w", path, err)
		}
		entries = append(entries, Entry{ID: id, Path: path, Suite: value, Source: data})
		return nil
	})
	return entries, err
}

func parse(data []byte) (Suite, error) {
	var value Suite
	if err := yaml.UnmarshalWithOptions(data, &value, yaml.Strict()); err != nil {
		return Suite{}, err
	}
	return value, nil
}

func validateID(id string) error {
	if id == "" || filepath.IsAbs(id) || strings.Contains(id, "\\") {
		return fmt.Errorf("invalid suite ID %q", id)
	}
	for _, segment := range strings.Split(id, "/") {
		if !idSegment.MatchString(segment) {
			return fmt.Errorf("invalid suite ID segment %q", segment)
		}
	}
	return nil
}
