package scenario

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	store := NewStore(root, service.EventRegistry(), service.Configuration())
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
	store := NewStore(root, service.EventRegistry(), service.Configuration())
	if _, err := store.Copy("builtin:chat/basic-message", "escape/copied", "kick-sim@test"); err == nil {
		t.Fatal("Copy() followed a symlink outside the scenarios directory")
	}
}

func TestSaveSourceValidatesAndAtomicallyPreservesExactText(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	entry, err := store.Copy("builtin:chat/basic-message", "editing/basic", "kick-sim@test")
	if err != nil {
		t.Fatal(err)
	}
	updated := []byte("# kept exactly as submitted\n" + strings.Replace(string(entry.Source), "Hello from Kick Sim", "Edited in Studio", 1))
	saved, err := store.SaveSource(entry.ID, entry.Revision, updated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved.Source, updated) {
		t.Fatal("SaveSource() did not retain the submitted source bytes")
	}
	onDisk, err := os.ReadFile(entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, updated) {
		t.Fatal("saved file differs from submitted source")
	}
	if saved.Revision == entry.Revision {
		t.Fatal("source revision did not change")
	}
	if temporary, err := filepath.Glob(filepath.Join(filepath.Dir(entry.Path), ".kick-sim-scenario-*")); err != nil || len(temporary) != 0 {
		t.Fatalf("temporary scenario files remain after save: %v, %v", temporary, err)
	}

	invalid := []byte("version: [\n")
	if _, err := store.SaveSource(entry.ID, saved.Revision, invalid); err == nil {
		t.Fatal("SaveSource() accepted invalid source")
	} else {
		var validation *SourceValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("invalid source error = %T, want *SourceValidationError", err)
		}
	}
	afterInvalid, err := os.ReadFile(entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterInvalid, updated) {
		t.Fatal("invalid save changed the valid file")
	}
}

func TestSaveSourceRejectsExternalRevisionAndReportsInvalidFiles(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	entry, err := store.Copy("builtin:chat/basic-message", "editing/conflict", "kick-sim@test")
	if err != nil {
		t.Fatal(err)
	}
	external := []byte(strings.Replace(string(entry.Source), "Hello from Kick Sim", "Changed outside Studio", 1))
	if err := os.WriteFile(entry.Path, external, 0o644); err != nil {
		t.Fatal(err)
	}
	studioDraft := []byte(strings.Replace(string(entry.Source), "Hello from Kick Sim", "Changed in Studio", 1))
	if _, err := store.SaveSource(entry.ID, entry.Revision, studioDraft); err == nil {
		t.Fatal("SaveSource() overwrote an external revision")
	} else {
		var conflict *RevisionConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("conflict error = %T, want *RevisionConflictError", err)
		}
		if conflict.Expected != entry.Revision || conflict.Actual != revision(external) {
			t.Fatalf("conflict = %#v", conflict)
		}
	}
	onDisk, err := os.ReadFile(entry.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, external) {
		t.Fatal("revision conflict changed the external file")
	}

	invalid := []byte("not: [valid\n")
	if err := os.WriteFile(entry.Path, invalid, 0o644); err != nil {
		t.Fatal(err)
	}
	listed, err := store.Get(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.ValidationErrors) == 0 || !bytes.Equal(listed.Source, invalid) {
		t.Fatalf("invalid external scenario = %#v", listed)
	}
	if err := store.Validate(listed); err == nil {
		t.Fatal("invalid external scenario is executable")
	}
}

func TestSaveSourceAsCopyUsesCanonicalFormattingAndDoesNotOverwrite(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	source, err := store.Get("builtin:chat/basic-message")
	if err != nil {
		t.Fatal(err)
	}
	copy, err := store.SaveSourceAsCopy(source.ID, "editing/source-copy", source.Source, "kick-sim@test")
	if err != nil {
		t.Fatal(err)
	}
	if copy.Scenario.Metadata.Source != source.ID || copy.Scenario.Metadata.CreatedWith != "kick-sim@test" {
		t.Fatalf("copy metadata = %#v", copy.Scenario.Metadata)
	}
	if _, err := store.SaveSourceAsCopy(source.ID, copy.ID, source.Source, "kick-sim@test"); err == nil {
		t.Fatal("SaveSourceAsCopy() overwrote an existing scenario")
	}
}

func TestSaveSourceSupportsJSONScenarioFiles(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	builtIn, err := store.Get("builtin:chat/basic-message")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(builtIn.Scenario, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join(store.workspaceRoot, "scenarios", "editing", "json-message.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	entry, err := store.Get("editing/json-message")
	if err != nil {
		t.Fatal(err)
	}
	if entry.SourceFormat != "json" {
		t.Fatalf("source format = %q", entry.SourceFormat)
	}
	updated := bytes.Replace(data, []byte("Basic chat message"), []byte("Edited JSON message"), 1)
	saved, err := store.SaveSource(entry.ID, entry.Revision, updated)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SourceFormat != "json" || !bytes.Equal(saved.Source, updated) {
		t.Fatalf("saved JSON entry = %#v", saved)
	}
}

func TestGetRejectsDuplicateCustomSourceExtensions(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	source, err := store.Get("builtin:chat/basic-message")
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(store.workspaceRoot, "scenarios", "duplicate")
	if err := os.WriteFile(base+".yaml", source.Source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+".json", source.Source, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("duplicate"); err == nil || !strings.Contains(err.Error(), "defined by both") {
		t.Fatalf("Get() error = %v", err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(root, service.EventRegistry(), service.Configuration())
}
