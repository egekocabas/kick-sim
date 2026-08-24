package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/signing"
)

const EnvironmentVariable = "KICK_SIM_WORKSPACE"

type Paths struct {
	Root       string
	Config     string
	Scenarios  string
	Runtime    string
	Database   string
	PrivateKey string
	PublicKey  string
	GitIgnore  string
}

type ResolveOptions struct {
	Explicit   string
	Start      string
	Initialize bool
}

func PathsFor(root string) Paths {
	keysDirectory := filepath.Join(root, "keys")
	return Paths{
		Root:       root,
		Config:     filepath.Join(root, "config.yaml"),
		Scenarios:  filepath.Join(root, "scenarios"),
		Runtime:    filepath.Join(root, ".runtime"),
		Database:   filepath.Join(root, ".runtime", "kick-sim.db"),
		PrivateKey: filepath.Join(keysDirectory, "private-key.pem"),
		PublicKey:  filepath.Join(keysDirectory, "public-key.pem"),
		GitIgnore:  filepath.Join(root, ".gitignore"),
	}
}

func Resolve(options ResolveOptions) (string, error) {
	start := options.Start
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve current directory: %w", err)
		}
	}
	if options.Explicit != "" {
		return absolute(options.Explicit, start)
	}
	if environment := os.Getenv(EnvironmentVariable); environment != "" {
		return absolute(environment, start)
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolve workspace search path: %w", err)
	}
	for {
		candidate := filepath.Join(current, ".kick-sim")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect workspace candidate: %w", err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	if options.Initialize {
		return filepath.Join(mustAbsolute(start), ".kick-sim"), nil
	}
	return "", errors.New("no .kick-sim workspace found; run kick-sim init or use --workspace")
}

func Init(root string) (Paths, error) {
	paths := PathsFor(root)
	for _, path := range []string{paths.Config, paths.PrivateKey, paths.PublicKey} {
		if _, err := os.Lstat(path); err == nil {
			return Paths{}, fmt.Errorf("refusing to overwrite existing workspace file %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return Paths{}, fmt.Errorf("inspect workspace path %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(paths.Scenarios, 0o755); err != nil {
		return Paths{}, fmt.Errorf("create scenarios directory: %w", err)
	}
	if err := InitKeys(root); err != nil {
		return Paths{}, err
	}
	configuration, err := config.Marshal(config.Default())
	if err != nil {
		cleanupKeys(paths)
		return Paths{}, err
	}
	if err := writeExclusive(paths.Config, configuration, 0o644); err != nil {
		cleanupKeys(paths)
		return Paths{}, err
	}
	if err := ensureGitIgnore(paths.GitIgnore); err != nil {
		_ = os.Remove(paths.Config)
		cleanupKeys(paths)
		return Paths{}, err
	}
	return paths, nil
}

func InitKeys(root string) error {
	paths := PathsFor(root)
	for _, path := range []string{paths.PrivateKey, paths.PublicKey} {
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing key %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect key path %s: %w", path, err)
		}
	}
	return writeNewKeyPair(paths, false)
}

func RotateKeys(root string) error {
	paths := PathsFor(root)
	if _, err := os.Stat(paths.PrivateKey); err != nil {
		return fmt.Errorf("existing private key is required for rotation: %w", err)
	}
	return writeNewKeyPair(paths, true)
}

func Validate(root string) error {
	paths := PathsFor(root)
	configuration, err := config.Load(paths.Config)
	if err != nil {
		return err
	}
	privatePath := config.ResolvePath(root, configuration.Signing.PrivateKey)
	publicPath := config.ResolvePath(root, configuration.Signing.PublicKey)
	if err := ValidatePrivateKeyPermissions(privatePath); err != nil {
		return err
	}
	privateKey, err := signing.ReadPrivateKey(privatePath)
	if err != nil {
		return fmt.Errorf("read private key: %w", err)
	}
	publicKey, err := signing.ReadPublicKey(publicPath)
	if err != nil {
		return fmt.Errorf("read public key: %w", err)
	}
	if privateKey.PublicKey.N.Cmp(publicKey.N) != 0 {
		return errors.New("workspace public and private keys do not match")
	}
	return nil
}

func ensureGitIgnore(path string) error {
	const required = "/keys/private-key.pem\n/.runtime/\n"
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return writeExclusive(path, []byte(required), 0o644)
	}
	if err != nil {
		return fmt.Errorf("read workspace .gitignore: %w", err)
	}
	content := string(data)
	for _, rule := range strings.Split(strings.TrimSpace(required), "\n") {
		found := false
		for _, line := range strings.Split(content, "\n") {
			if strings.TrimSpace(line) == rule {
				found = true
				break
			}
		}
		if !found {
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += rule + "\n"
		}
	}
	if content == string(data) {
		return nil
	}
	return writeAtomic(path, []byte(content), 0o644)
}

func writeNewKeyPair(paths Paths, replace bool) error {
	privateKey, err := signing.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate simulator key: %w", err)
	}
	privatePEM, err := signing.MarshalPrivateKey(privateKey)
	if err != nil {
		return err
	}
	publicPEM, err := signing.MarshalPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	keysDirectory := filepath.Dir(paths.PrivateKey)
	if err := os.MkdirAll(keysDirectory, 0o700); err != nil {
		return fmt.Errorf("create keys directory: %w", err)
	}
	if replace {
		if err := writeAtomic(paths.PrivateKey, privatePEM, 0o600); err != nil {
			return err
		}
		if err := writeAtomic(paths.PublicKey, publicPEM, 0o644); err != nil {
			return err
		}
		return nil
	}
	if err := writeExclusive(paths.PrivateKey, privatePEM, 0o600); err != nil {
		return err
	}
	if err := writeExclusive(paths.PublicKey, publicPEM, 0o644); err != nil {
		_ = os.Remove(paths.PrivateKey)
		return err
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

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".kick-sim-write-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func cleanupKeys(paths Paths) {
	_ = os.Remove(paths.PrivateKey)
	_ = os.Remove(paths.PublicKey)
}

func absolute(path, start string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(start, path)
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	return filepath.Clean(absolutePath), nil
}

func mustAbsolute(path string) string {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return absolutePath
}
