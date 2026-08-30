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

// Entry combines a parsed suite with its source identity.
type Entry struct {
	ID      string `json:"id"`
	BuiltIn bool   `json:"builtIn"`
	Path    string `json:"path,omitempty"`
	Suite   Suite  `json:"suite"`
	Source  []byte `json:"-"`
}

// Store loads built-in and workspace suites and validates their scenarios.
type Store struct {
	workspaceRoot string
	scenarios     *scenario.Store
}

// NewStore constructs a suite store for a workspace and scenario catalog.
func NewStore(workspaceRoot string, scenarios *scenario.Store) *Store {
	return &Store{workspaceRoot: workspaceRoot, scenarios: scenarios}
}

// List returns built-in and custom suites in stable identifier order.
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

// Get resolves a built-in or workspace suite by identifier.
func (store *Store) Get(id string) (Entry, error) {
	if strings.HasPrefix(id, "builtin:") {
		return store.getBuiltIn(strings.TrimPrefix(id, "builtin:"))
	}
	return store.getCustom(id)
}

func (store *Store) getBuiltIn(id string) (Entry, error) {
	if err := validateID(id); err != nil {
		return Entry{}, err
	}
	path := "suites/" + id + ".yaml"
	data, err := assets.Files.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Entry{}, fmt.Errorf("suite %q was not found", "builtin:"+id)
	}
	if err != nil {
		return Entry{}, fmt.Errorf("read built-in suite: %w", err)
	}
	value, err := parse(data)
	if err != nil {
		return Entry{}, fmt.Errorf("parse built-in %s: %w", path, err)
	}
	return Entry{ID: "builtin:" + id, BuiltIn: true, Suite: value, Source: data}, nil
}

func (store *Store) getCustom(id string) (Entry, error) {
	if err := validateID(id); err != nil {
		return Entry{}, err
	}
	root := workspace.PathsFor(store.workspaceRoot).Suites
	base := filepath.Join(root, filepath.FromSlash(id))
	var matches []string
	for _, extension := range []string{".yaml", ".yml", ".json"} {
		path := base + extension
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Entry{}, fmt.Errorf("inspect suite: %w", err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return Entry{}, errors.New("suite source must be a regular file")
		}
		matches = append(matches, path)
	}
	if len(matches) == 0 {
		return Entry{}, fmt.Errorf("suite %q was not found", id)
	}
	if len(matches) > 1 {
		return Entry{}, fmt.Errorf("suite ID %q is defined by both %s and %s", id, matches[0], matches[1])
	}
	if err := ensureNoSymlinkComponents(root, matches[0]); err != nil {
		return Entry{}, err
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return Entry{}, fmt.Errorf("read suite: %w", err)
	}
	value, err := parse(data)
	if err != nil {
		return Entry{}, fmt.Errorf("parse suite %s: %w", matches[0], err)
	}
	return Entry{ID: id, Path: matches[0], Suite: value, Source: data}, nil
}

// Validate checks a suite and all scenarios it references.
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
	seen := map[string]string{}
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
		if prior, duplicate := seen[id]; duplicate {
			return fmt.Errorf("suite ID %q is defined by both %s and %s", id, prior, path)
		}
		seen[id] = path
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

func ensureNoSymlinkComponents(root, target string) error {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return fmt.Errorf("inspect suites root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("suites directory may not be a symlink")
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
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("suite path may not contain symlink %s", current)
		}
	}
	return nil
}
