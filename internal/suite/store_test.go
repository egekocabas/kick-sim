package suite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestBuiltInSuitesValidate(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	scenarios := scenario.NewStore(root, service.EventRegistry(), service.Configuration(), service.ActorRegistry())
	store := NewStore(root, scenarios)
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Fatalf("built-in suites = %d", len(entries))
	}
	for _, entry := range entries {
		if err := store.Validate(entry); err != nil {
			t.Fatalf("validate %s: %v", entry.ID, err)
		}
	}
}

func TestGetRejectsDuplicateCustomSourceExtensions(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	scenarios := scenario.NewStore(root, service.EventRegistry(), service.Configuration(), service.ActorRegistry())
	store := NewStore(root, scenarios)
	builtIn, err := store.Get("builtin:delivery")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, "suites", "duplicate")
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+".yaml", builtIn.Source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+".json", builtIn.Source, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("duplicate"); err == nil || !strings.Contains(err.Error(), "defined by both") {
		t.Fatalf("Get() error = %v", err)
	}
}
