package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/egekocabas/kick-sim/internal/signing"
)

func TestInitCreatesProtectedKeyPairWithoutOverwriting(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	paths, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := signing.ReadPrivateKey(paths.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := signing.ReadPublicKey(paths.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if privateKey.PublicKey.N.Cmp(publicKey.N) != 0 {
		t.Fatal("workspace public and private keys do not match")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(paths.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("private key permissions = %o, want 600", got)
		}
	}

	originalPrivateKey, err := os.ReadFile(paths.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Init(root); err == nil {
		t.Fatal("second Init() unexpectedly overwrote the workspace")
	}
	afterSecondInit, err := os.ReadFile(paths.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(originalPrivateKey, afterSecondInit) {
		t.Fatal("second Init() changed the existing private key")
	}

	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gitignore) != "/keys/private-key.pem\n/.runtime/\n" {
		t.Fatalf("workspace .gitignore = %q", gitignore)
	}
}

func TestResolveUsesExplicitEnvironmentAndNearestWorkspace(t *testing.T) {
	base := t.TempDir()
	nearest := filepath.Join(base, ".kick-sim")
	if err := os.Mkdir(nearest, 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(base, "src", "handler")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	resolved, err := Resolve(ResolveOptions{Start: nested})
	if err != nil {
		t.Fatal(err)
	}
	if resolved != nearest {
		t.Fatalf("nearest workspace = %q, want %q", resolved, nearest)
	}

	environment := filepath.Join(base, "environment")
	t.Setenv(EnvironmentVariable, environment)
	resolved, err = Resolve(ResolveOptions{Start: nested})
	if err != nil {
		t.Fatal(err)
	}
	if resolved != environment {
		t.Fatalf("environment workspace = %q, want %q", resolved, environment)
	}

	explicit := filepath.Join(base, "explicit")
	resolved, err = Resolve(ResolveOptions{Explicit: explicit, Start: nested})
	if err != nil {
		t.Fatal(err)
	}
	if resolved != explicit {
		t.Fatalf("explicit workspace = %q, want %q", resolved, explicit)
	}
}

func TestInitPreservesExistingGitIgnoreRules(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("custom-rule\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	want := "custom-rule\n/keys/private-key.pem\n/.runtime/\n"
	if string(data) != want {
		t.Fatalf(".gitignore = %q, want %q", data, want)
	}
}

func TestRotateKeysReplacesMatchingPair(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	paths, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(paths.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := RotateKeys(root); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(paths.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, after) {
		t.Fatal("RotateKeys() did not replace the public key")
	}
	if err := Validate(root); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceKeyPairRollsBackAfterInstallFailure(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	paths, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}
	originalPrivate, err := os.ReadFile(paths.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	originalPublic, err := os.ReadFile(paths.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	key, err := signing.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	privatePEM, err := signing.MarshalPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicPEM, err := signing.MarshalPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	renames := 0
	operations := osKeyFileOperations()
	operations.rename = func(source, target string) error {
		renames++
		if renames == 4 {
			return errors.New("injected public install failure")
		}
		return os.Rename(source, target)
	}
	if err := replaceKeyPair(paths, privatePEM, publicPEM, operations); err == nil {
		t.Fatal("replaceKeyPair() succeeded after an injected failure")
	}
	afterPrivate, err := os.ReadFile(paths.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	afterPublic, err := os.ReadFile(paths.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterPrivate, originalPrivate) || !bytes.Equal(afterPublic, originalPublic) {
		t.Fatal("replaceKeyPair() did not restore the original pair")
	}
	if err := Validate(root); err != nil {
		t.Fatal(err)
	}
}
