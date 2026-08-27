package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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

	// Re-check immediately before rename so concurrent Studio writers cannot
	// silently replace a source revision they did not load.
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
