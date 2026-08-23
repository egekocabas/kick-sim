package cli

import (
	"bytes"
	"context"
	"encoding/json"
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
	if !strings.Contains(stdout.String(), "Kick Sim workspace created:\n  "+workspacePath) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRequiredCommandSurface(t *testing.T) {
	t.Parallel()

	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	for _, path := range []string{
		"init",
		"workspace path", "workspace validate", "workspace info",
		"event list", "event show", "event generate", "event trigger", "event validate",
		"scenario list", "scenario show", "scenario copy", "scenario validate", "scenario run",
		"keys init", "keys public", "keys info", "keys rotate",
		"config show", "config validate",
		"compatibility", "version",
	} {
		found, _, err := command.Find(strings.Fields(path))
		if err != nil || found.CommandPath() != "kick-sim "+path {
			t.Errorf("command %q not found: %v", path, err)
		}
	}
	for _, unavailable := range []string{"studio", "history", "suite", "load", "platform"} {
		if found, _, err := command.Find([]string{unavailable}); err == nil && found.Name() == unavailable {
			t.Errorf("unavailable command %q is visible", unavailable)
		}
	}
}

func TestScenarioAndEventCommandsUseMachineReadableOutput(t *testing.T) {
	t.Parallel()

	workspacePath := filepath.Join(t.TempDir(), ".kick-sim")
	if _, _, err := runCommand("--workspace", workspacePath, "init"); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runCommand("--workspace", workspacePath, "--output", "json", "scenario", "list")
	if err != nil {
		t.Fatal(err)
	}
	var scenarios []map[string]any
	if err := json.Unmarshal([]byte(stdout), &scenarios); err != nil {
		t.Fatalf("scenario list output is not JSON: %v\n%s", err, stdout)
	}
	if len(scenarios) < 5 {
		t.Fatalf("scenario list length = %d", len(scenarios))
	}

	if _, _, err := runCommand("--workspace", workspacePath, "scenario", "copy", "builtin:chat/basic-message", "regressions/basic"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCommand("--workspace", workspacePath, "scenario", "validate", "regressions/basic"); err != nil {
		t.Fatal(err)
	}

	stdout, _, err = runCommand(
		"--workspace", workspacePath,
		"--output", "json",
		"event", "generate", "chat.message.sent",
		"--content", "from-test",
	)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("event generate output is not JSON: %v\n%s", err, stdout)
	}
	if payload["content"] != "from-test" {
		t.Fatalf("generated content = %v", payload["content"])
	}
}

func runCommand(args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := NewRootCommand(&stdout, &stderr)
	err := executeForTest(context.Background(), command, args...)
	return stdout.String(), stderr.String(), err
}
