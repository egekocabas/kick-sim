package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCommandCreatesWorkspace(t *testing.T) {
	t.Parallel()

	workspacePath := filepath.Join(t.TempDir(), ".kick-sim")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := NewRootCommand(&stdout, &stderr)

	if err := executeForTest(context.Background(), command, "--workspace", workspacePath, "init"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Workspace created: "+workspacePath) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
