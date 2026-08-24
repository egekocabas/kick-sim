package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestAssetsContainStudioEntryPoint(t *testing.T) {
	assets, _ := Assets()
	data, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Kick Sim Studio") {
		t.Fatalf("embedded index does not identify the Studio: %s", data)
	}
}
