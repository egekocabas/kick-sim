package scenario

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestBuiltInsValidateAndCopyToCustomScenario(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(root, service.Events, service.Config)
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	builtIns := 0
	for _, entry := range entries {
		if !entry.BuiltIn {
			continue
		}
		builtIns++
		if err := store.Validate(entry); err != nil {
			t.Fatalf("validate %s: %v", entry.ID, err)
		}
	}
	if builtIns < 5 {
		t.Fatalf("built-in count = %d, want at least 5", builtIns)
	}

	copy, err := store.Copy("builtin:chat/moderator-message", "regressions/moderator", "kick-sim@test")
	if err != nil {
		t.Fatal(err)
	}
	if copy.Scenario.Metadata.Source != "builtin:chat/moderator-message" {
		t.Fatalf("copy source = %q", copy.Scenario.Metadata.Source)
	}
	if err := store.Validate(copy); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Copy("builtin:chat/basic-message", "regressions/moderator", "kick-sim@test"); err == nil {
		t.Fatal("Copy() overwrote an existing scenario")
	}
	source, err := store.Get("builtin:chat/basic-message")
	if err != nil {
		t.Fatal(err)
	}
	modified := map[string]any{}
	for key, value := range source.Scenario.Request.Payload {
		modified[key] = value
	}
	modified["content"] = "saved working copy"
	saved, err := store.SaveAsCopy(source.ID, "working/basic", modified, "kick-sim@test")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Scenario.Request.Payload["content"] != "saved working copy" {
		t.Fatalf("saved payload = %#v", saved.Scenario.Request.Payload)
	}
	reloadedSource, err := store.Get(source.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedSource.Revision != source.Revision {
		t.Fatal("SaveAsCopy() rewrote the source scenario")
	}
}

func TestValidateIDRejectsTraversalAndPlatformSpecificPaths(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"", "../outside", "a//b", "Uppercase", `a\\b`, "/absolute", "builtin:chat/basic"} {
		if err := ValidateID(id); err == nil {
			t.Errorf("ValidateID(%q) returned nil", id)
		}
	}
}

func TestCopyRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires additional Windows privileges")
	}

	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "scenarios", "escape")); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(root, service.Events, service.Config)
	if _, err := store.Copy("builtin:chat/basic-message", "escape/copied", "kick-sim@test"); err == nil {
		t.Fatal("Copy() followed a symlink outside the scenarios directory")
	}
}
