package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/egekocabas/kick-sim/assets"
	"github.com/goccy/go-yaml"
)

func (store *Store) getBuiltIn(id string) (Entry, error) {
	if err := ValidateID(id); err != nil {
		return Entry{}, err
	}
	path := "scenarios/" + id + ".yaml"
	data, err := assets.Files.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Entry{}, fmt.Errorf("scenario %q was not found", "builtin:"+id)
	}
	if err != nil {
		return Entry{}, fmt.Errorf("read built-in scenario: %w", err)
	}
	value, err := parse(data)
	if err != nil {
		return Entry{}, fmt.Errorf("parse built-in %s: %w", path, err)
	}
	return newEntry("builtin:"+id, true, "", value, data, 1), nil
}

func (store *Store) getCustom(id string) (Entry, error) {
	if err := ValidateID(id); err != nil {
		return Entry{}, err
	}
	root := filepath.Join(store.workspaceRoot, "scenarios")
	var matches []string
	for _, extension := range []string{".yaml", ".yml", ".json"} {
		path, err := store.customPath(id, extension)
		if err != nil {
			return Entry{}, err
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return Entry{}, fmt.Errorf("inspect scenario: %w", err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return Entry{}, errors.New("scenario source must be a regular file")
		}
		matches = append(matches, path)
	}
	if len(matches) == 0 {
		return Entry{}, fmt.Errorf("scenario %q was not found", id)
	}
	if len(matches) > 1 {
		return Entry{}, fmt.Errorf("scenario ID %q is defined by both %s and %s", id, matches[0], matches[1])
	}
	path := matches[0]
	if err := ensureNoSymlinkComponents(root, path); err != nil {
		return Entry{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, fmt.Errorf("read scenario: %w", err)
	}
	value, validationErr := store.parseAndValidate(data)
	entry := newEntry(id, false, path, value, data, 1)
	if validationErr != nil {
		entry.validationError = validationErr
		entry.ValidationErrors = errorMessages(validationErr)
	}
	return entry, nil
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
	return Entry{ID: id, BuiltIn: builtIn, Path: path, Scenario: value,
		Source: append([]byte(nil), source...), SourceFormat: format,
		Revision: revision(source), SourceVersion: sourceVersion}
}

func (store *Store) parseAndValidate(data []byte) (Scenario, error) {
	value, err := parse(data)
	if err != nil {
		return Scenario{}, &SourceValidationError{Problems: []string{err.Error()}}
	}
	if err := value.Validate(store.registry, store.configuration, store.actors); err != nil {
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
	for index := range value.Steps {
		if value.Steps[index].Payload == nil {
			value.Steps[index].Payload = map[string]any{}
		}
	}
	return value, nil
}
