package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/egekocabas/kick-sim/internal/signing"
)

type Paths struct {
	Root       string
	PrivateKey string
	PublicKey  string
}

func PathsFor(root string) Paths {
	keysDirectory := filepath.Join(root, "keys")
	return Paths{
		Root:       root,
		PrivateKey: filepath.Join(keysDirectory, "private-key.pem"),
		PublicKey:  filepath.Join(keysDirectory, "public-key.pem"),
	}
}

func Init(root string) (Paths, error) {
	paths := PathsFor(root)
	if err := rejectExistingKeys(paths); err != nil {
		return Paths{}, err
	}

	privateKey, err := signing.GenerateKey()
	if err != nil {
		return Paths{}, fmt.Errorf("generate simulator key: %w", err)
	}
	publicPEM, err := signing.MarshalPublicKey(&privateKey.PublicKey)
	if err != nil {
		return Paths{}, err
	}

	keysDirectory := filepath.Dir(paths.PrivateKey)
	if err := os.MkdirAll(keysDirectory, 0o700); err != nil {
		return Paths{}, fmt.Errorf("create keys directory: %w", err)
	}
	if err := writeExclusive(paths.PrivateKey, signing.MarshalPrivateKey(privateKey), 0o600); err != nil {
		return Paths{}, err
	}
	if err := writeExclusive(paths.PublicKey, publicPEM, 0o644); err != nil {
		_ = os.Remove(paths.PrivateKey)
		return Paths{}, err
	}
	if err := writeExclusive(filepath.Join(keysDirectory, ".gitignore"), []byte("private-key.pem\n"), 0o644); err != nil && !errors.Is(err, os.ErrExist) {
		_ = os.Remove(paths.PrivateKey)
		_ = os.Remove(paths.PublicKey)
		return Paths{}, err
	}

	return paths, nil
}

func rejectExistingKeys(paths Paths) error {
	for _, path := range []string{paths.PrivateKey, paths.PublicKey} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing key %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect key path %s: %w", path, err)
		}
	}
	return nil
}

func writeExclusive(path string, data []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
