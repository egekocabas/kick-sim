package events

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/config"
)

func TestDefaultPayloadResolvesAndValidates(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := registry.Get(ChatMessageSentType, ChatMessageSentVersion)
	if err != nil {
		t.Fatal(err)
	}
	payload := Compose(definition, config.Default().Defaults, map[string]any{"content": "hello"})
	resolved := ResolveDynamic(
		payload,
		func() string { return "01ARZ3NDEKTSV4RRFFQ69G5FAV" },
		time.Date(2026, time.August, 24, 10, 15, 30, 0, time.UTC),
	)
	if err := registry.Validate(ChatMessageSentType, ChatMessageSentVersion, resolved); err != nil {
		t.Fatal(err)
	}
}

func TestAllBundledEventDefaultsResolveAndValidate(t *testing.T) {
	t.Parallel()
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(registry.List()); got != 4 {
		t.Fatalf("event definitions = %d, want 4", got)
	}
	for _, listed := range registry.List() {
		definition, err := registry.Get(listed.Type, listed.Version)
		if err != nil {
			t.Fatal(err)
		}
		payload := Compose(definition, config.Default().Defaults, nil)
		resolved := ResolveDynamic(payload, func() string { return "01ARZ3NDEKTSV4RRFFQ69G5FAV" }, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC))
		if err := registry.Validate(listed.Type, listed.Version, resolved); err != nil {
			t.Fatalf("validate %s@%d: %v", listed.Type, listed.Version, err)
		}
	}
}

func TestActorProjectionProtectsOwnedFieldsAndPreservesChatContext(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "users.yaml")
	if err := os.WriteFile(path, []byte("version: 1\nusers:\n  moderator:\n    user_id: 300\n    username: test_mod\n    channel_slug: test-mod\n    is_verified: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	actorRegistry, err := actors.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	definition, err := registry.Get(ChatMessageSentType, 1)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := ComposeWithActors(definition, config.Default().Defaults, actorRegistry, map[string]string{"sender": "moderator"}, map[string]any{"sender": map[string]any{"identity": map[string]any{"username_color": "#53FC18", "badges": []any{}}}})
	if err != nil {
		t.Fatal(err)
	}
	sender := payload["sender"].(map[string]any)
	if sender["user_id"] != int64(300) || sender["username"] != "test_mod" {
		t.Fatalf("sender = %#v", sender)
	}
	if sender["identity"].(map[string]any)["username_color"] != "#53FC18" {
		t.Fatalf("identity = %#v", sender["identity"])
	}
	if _, err := ComposeWithActors(definition, config.Default().Defaults, actorRegistry, map[string]string{"sender": "moderator"}, map[string]any{"sender": map[string]any{"username": "override"}}); err == nil {
		t.Fatal("ComposeWithActors() accepted actor-owned payload data")
	}
	payload["sender"].(map[string]any)["username"] = "override"
	if err := ValidateActorOwned(definition, actorRegistry, map[string]string{"sender": "moderator"}, payload); err == nil {
		t.Fatal("ValidateActorOwned() accepted an override")
	}
}

func TestResolveDynamicAssignsIDsInSortedObjectOrder(t *testing.T) {
	t.Parallel()

	ids := []string{"first", "second"}
	resolved := ResolveDynamic(map[string]any{
		"z": "{{ ulid() }}",
		"a": "{{ ulid() }}",
	}, func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}, time.Time{})

	want := map[string]any{"a": "first", "z": "second"}
	if !reflect.DeepEqual(resolved, want) {
		t.Fatalf("ResolveDynamic() = %#v, want %#v", resolved, want)
	}
}

func TestDeepMergeReplacesArraysAndMergesObjects(t *testing.T) {
	t.Parallel()

	destination := map[string]any{
		"sender": map[string]any{"username": "before", "user_id": int64(1)},
		"emotes": []any{"before"},
	}
	DeepMerge(destination, map[string]any{
		"sender": map[string]any{"username": "after"},
		"emotes": []any{"after"},
	})
	sender := destination["sender"].(map[string]any)
	if sender["username"] != "after" || sender["user_id"] != int64(1) {
		t.Fatalf("object merge = %#v", sender)
	}
	emotes := destination["emotes"].([]any)
	if len(emotes) != 1 || emotes[0] != "after" {
		t.Fatalf("array merge = %#v", emotes)
	}
}

func TestPointerOverridesAreTypedAndBounded(t *testing.T) {
	t.Parallel()

	document := map[string]any{"payload": map[string]any{"content": "before", "sender": map[string]any{"user_id": int64(1)}}}
	if err := SetPointer(document, "/payload/sender/user_id", int64(42)); err != nil {
		t.Fatal(err)
	}
	if err := UnsetPointer(document, "/payload/content"); err != nil {
		t.Fatal(err)
	}
	if err := SetPointer(document, "/delivery/url", "bad"); err == nil {
		t.Fatal("SetPointer() accepted a delivery override")
	}
	payload := document["payload"].(map[string]any)
	if _, exists := payload["content"]; exists {
		t.Fatal("UnsetPointer() did not remove content")
	}
}

func TestPointerOverridesDecodeEscapedTokens(t *testing.T) {
	t.Parallel()

	document := map[string]any{
		"payload": map[string]any{
			"a/b": map[string]any{"~key": "before"},
		},
	}
	if err := SetPointer(document, "/payload/a~1b/~0key", "after"); err != nil {
		t.Fatal(err)
	}
	value := document["payload"].(map[string]any)["a/b"].(map[string]any)["~key"]
	if value != "after" {
		t.Fatalf("escaped pointer value = %v", value)
	}
	if err := SetPointer(document, "/payload/a~2b/~0key", "bad"); err == nil {
		t.Fatal("SetPointer() accepted an invalid escape")
	}
}
