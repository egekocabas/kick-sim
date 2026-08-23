package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigurationRoundTrip(t *testing.T) {
	t.Parallel()

	data, err := Marshal(Default())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultDestination != "local" || loaded.Destinations["local"].Class != "loopback" {
		t.Fatalf("unexpected default configuration: %#v", loaded)
	}
}

func TestValidateRejectsRemoteDestination(t *testing.T) {
	t.Parallel()

	value := Default()
	destination := value.Destinations["local"]
	destination.URL = "https://example.com/webhooks/kick"
	value.Destinations["local"] = destination
	if err := Validate(value); err == nil {
		t.Fatal("Validate() accepted a remote destination")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	data, err := Marshal(Default())
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("unknownField: true\n")...)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("Load() accepted an unknown configuration field")
	}
}
