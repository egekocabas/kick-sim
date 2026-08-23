package workspace

import (
	"bytes"
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

	gitignore, err := os.ReadFile(filepath.Join(root, "keys", ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gitignore) != "private-key.pem\n" {
		t.Fatalf("keys .gitignore = %q", gitignore)
	}
}
