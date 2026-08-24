package openapi

import (
	"bytes"
	"os"
	"testing"
)

func TestGeneratedDocumentIsDeterministicAndCommitted(t *testing.T) {
	first, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generated OpenAPI document is not deterministic")
	}
	committed, err := os.ReadFile("../../web/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, committed) {
		t.Fatal("web/openapi.json is stale; run make generate")
	}
}
