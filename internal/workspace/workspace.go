package workspace

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/signing"
)

const EnvironmentVariable = "KICK_SIM_WORKSPACE"

var keyFiles sync.RWMutex

const defaultActors = `version: 1
users:
  streamer:
    user_id: 100
    username: test_streamer
    channel_slug: test-streamer
    is_verified: false
  viewer:
    user_id: 200
    username: test_viewer
    channel_slug: test-viewer
    is_verified: false
  moderator:
    user_id: 300
    username: test_mod
    channel_slug: test-mod
    is_verified: false
`

type Paths struct {
	Root       string
	Config     string
	Scenarios  string
	Data       string
	Users      string
	Suites     string
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
	dataDirectory := filepath.Join(root, "data")
	return Paths{
		Root:       root,
		Config:     filepath.Join(root, "config.yaml"),
		Scenarios:  filepath.Join(root, "scenarios"),
		Data:       dataDirectory,
		Users:      filepath.Join(dataDirectory, "users.yaml"),
		Suites:     filepath.Join(root, "suites"),
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
	for _, path := range []string{paths.Config, paths.Users, paths.PrivateKey, paths.PublicKey} {
		if _, err := os.Lstat(path); err == nil {
			return Paths{}, fmt.Errorf("refusing to overwrite existing workspace file %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return Paths{}, fmt.Errorf("inspect workspace path %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(paths.Scenarios, 0o755); err != nil {
		return Paths{}, fmt.Errorf("create scenarios directory: %w", err)
	}
	if err := os.MkdirAll(paths.Data, 0o755); err != nil {
		return Paths{}, fmt.Errorf("create actor data directory: %w", err)
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
	if err := writeExclusive(paths.Users, []byte(defaultActors), 0o644); err != nil {
		_ = os.Remove(paths.Config)
		cleanupKeys(paths)
		return Paths{}, err
	}
	if err := ensureGitIgnore(paths.GitIgnore); err != nil {
		_ = os.Remove(paths.Config)
		_ = os.Remove(paths.Users)
		cleanupKeys(paths)
		return Paths{}, err
	}
	return paths, nil
}

func InitKeys(root string) error {
	keyFiles.Lock()
	defer keyFiles.Unlock()
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
	keyFiles.Lock()
	defer keyFiles.Unlock()
	paths := PathsFor(root)
	if _, _, err := readKeyPair(paths.PrivateKey, paths.PublicKey); err != nil {
		return fmt.Errorf("existing matching key pair is required for rotation: %w", err)
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
	_, _, err = ReadKeyPair(privatePath, publicPath)
	return err
}

// ReadPrivateKey reads one key while excluding in-process key rotation.
func ReadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyFiles.RLock()
	defer keyFiles.RUnlock()
	if err := ValidatePrivateKeyPermissions(path); err != nil {
		return nil, err
	}
	return signing.ReadPrivateKey(path)
}

// ReadPublicKey reads one key while excluding in-process key rotation.
func ReadPublicKey(path string) (*rsa.PublicKey, error) {
	keyFiles.RLock()
	defer keyFiles.RUnlock()
	return signing.ReadPublicKey(path)
}

// ReadPublicKeyPEM returns the public-key file while excluding rotation.
func ReadPublicKeyPEM(path string) ([]byte, error) {
	keyFiles.RLock()
	defer keyFiles.RUnlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	return data, nil
}

// InspectKeyPair returns the readable public key and whether its private key matches.
func InspectKeyPair(privatePath, publicPath string) (*rsa.PublicKey, bool, error) {
	keyFiles.RLock()
	defer keyFiles.RUnlock()
	publicKey, err := signing.ReadPublicKey(publicPath)
	if err != nil {
		return nil, false, err
	}
	if err := ValidatePrivateKeyPermissions(privatePath); err != nil {
		return publicKey, false, nil
	}
	privateKey, err := signing.ReadPrivateKey(privatePath)
	if err != nil {
		return publicKey, false, nil
	}
	return publicKey, privateKey.PublicKey.N.Cmp(publicKey.N) == 0, nil
}

// ReadKeyPair reads and verifies a matching pair as one in-process operation.
func ReadKeyPair(privatePath, publicPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	keyFiles.RLock()
	defer keyFiles.RUnlock()
	return readKeyPair(privatePath, publicPath)
}

func readKeyPair(privatePath, publicPath string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	if err := ValidatePrivateKeyPermissions(privatePath); err != nil {
		return nil, nil, err
	}
	privateKey, err := signing.ReadPrivateKey(privatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read private key: %w", err)
	}
	publicKey, err := signing.ReadPublicKey(publicPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read public key: %w", err)
	}
	if privateKey.PublicKey.N.Cmp(publicKey.N) != 0 {
		return nil, nil, errors.New("workspace public and private keys do not match")
	}
	return privateKey, publicKey, nil
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
		return replaceKeyPair(paths, privatePEM, publicPEM, osKeyFileOperations())
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

type keyFileOperations struct {
	rename func(string, string) error
	remove func(string) error
}

func osKeyFileOperations() keyFileOperations {
	return keyFileOperations{rename: os.Rename, remove: os.Remove}
}

func replaceKeyPair(paths Paths, privatePEM, publicPEM []byte, operations keyFileOperations) error {
	directory := filepath.Dir(paths.PrivateKey)
	stagedPrivate, err := writeTemporary(directory, privatePEM, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(stagedPrivate)
	stagedPublic, err := writeTemporary(directory, publicPEM, 0o644)
	if err != nil {
		return err
	}
	defer os.Remove(stagedPublic)

	privateBackup, err := reserveBackupPath(directory, ".kick-sim-private-backup-*")
	if err != nil {
		return err
	}
	defer operations.remove(privateBackup)
	publicBackup, err := reserveBackupPath(directory, ".kick-sim-public-backup-*")
	if err != nil {
		return err
	}
	defer operations.remove(publicBackup)

	privateMoved, publicMoved := false, false
	privateInstalled, publicInstalled := false, false
	rollback := func() error {
		var problems []error
		if privateInstalled {
			problems = append(problems, ignoreNotExist(operations.remove(paths.PrivateKey)))
		}
		if publicInstalled {
			problems = append(problems, ignoreNotExist(operations.remove(paths.PublicKey)))
		}
		if privateMoved {
			problems = append(problems, operations.rename(privateBackup, paths.PrivateKey))
		}
		if publicMoved {
			problems = append(problems, operations.rename(publicBackup, paths.PublicKey))
		}
		problems = append(problems, syncDirectory(directory))
		return errors.Join(problems...)
	}
	fail := func(action string, operationErr error) error {
		return errors.Join(fmt.Errorf("%s: %w", action, operationErr), rollback())
	}

	if err := operations.rename(paths.PrivateKey, privateBackup); err != nil {
		return fmt.Errorf("back up private key: %w", err)
	}
	privateMoved = true
	if err := operations.rename(paths.PublicKey, publicBackup); err != nil {
		return fail("back up public key", err)
	}
	publicMoved = true
	if err := operations.rename(stagedPrivate, paths.PrivateKey); err != nil {
		return fail("install private key", err)
	}
	privateInstalled = true
	if err := operations.rename(stagedPublic, paths.PublicKey); err != nil {
		return fail("install public key", err)
	}
	publicInstalled = true
	if _, _, err := readKeyPair(paths.PrivateKey, paths.PublicKey); err != nil {
		return fail("verify rotated key pair", err)
	}
	if err := syncDirectory(directory); err != nil {
		return fail("sync rotated key pair", err)
	}
	if err := operations.remove(privateBackup); err != nil {
		return fmt.Errorf("remove private key backup: %w", err)
	}
	privateMoved = false
	if err := operations.remove(publicBackup); err != nil {
		return fmt.Errorf("remove public key backup: %w", err)
	}
	publicMoved = false
	return syncDirectory(directory)
}

func reserveBackupPath(directory, pattern string) (string, error) {
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", fmt.Errorf("reserve key backup path: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close key backup placeholder: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("prepare key backup path: %w", err)
	}
	return path, nil
}

func ignoreNotExist(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func writeTemporary(directory string, data []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(directory, ".kick-sim-key-*")
	if err != nil {
		return "", fmt.Errorf("create temporary key: %w", err)
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
		return "", fmt.Errorf("set temporary key permissions: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("write temporary key: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("sync temporary key: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close temporary key: %w", err)
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
		return fmt.Errorf("open key directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync key directory: %w", err)
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
