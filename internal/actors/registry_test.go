package actors

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidatesActorIDsAndUniqueUserIDs(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "users.yaml")
	data := []byte("version: 1\nusers:\n  streamer:\n    user_id: 100\n    username: streamer\n    channel_slug: streamer\n    is_verified: false\n  viewer:\n    user_id: 200\n    username: viewer\n    channel_slug: viewer\n    is_verified: true\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	registry, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := registry.IDs(); len(got) != 2 || got[0] != "streamer" || got[1] != "viewer" {
		t.Fatalf("IDs() = %#v", got)
	}

	duplicate := []byte("version: 1\nusers:\n  one:\n    user_id: 100\n    username: one\n    channel_slug: one\n    is_verified: false\n  two:\n    user_id: 100\n    username: two\n    channel_slug: two\n    is_verified: false\n")
	if err := os.WriteFile(path, duplicate, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted duplicate user IDs")
	}
}

func TestMissingRegistryIsEmpty(t *testing.T) {
	t.Parallel()
	registry, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.List()) != 0 {
		t.Fatalf("List() = %#v", registry.List())
	}
}
