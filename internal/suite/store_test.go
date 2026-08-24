package suite

import (
	"path/filepath"
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
	scenarios := scenario.NewStore(root, service.Events, service.Config, service.Actors)
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
