package openapi

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
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
	var generatedDocument any
	var committedDocument any
	if err := json.Unmarshal(first, &generatedDocument); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(committed, &committedDocument); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(generatedDocument, committedDocument) {
		t.Fatal("web/openapi.json is stale; run make generate")
	}
}
