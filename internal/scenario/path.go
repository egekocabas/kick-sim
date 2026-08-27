package scenario

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var scenarioSegment = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

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
