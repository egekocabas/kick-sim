package tests

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/signing"
)

func TestCLIExecutableWorkflow(t *testing.T) {
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binaryName := "kick-sim"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), binaryName)
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/kick-sim")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v\n%s", err, output)
	}

	workspaceRoot := filepath.Join(t.TempDir(), ".kick-sim")
	run(t, binary, "--workspace", workspaceRoot, "init")
	run(t, binary, "--workspace", workspaceRoot, "workspace", "validate")
	run(t, binary, "--workspace", workspaceRoot, "scenario", "copy", "builtin:chat/basic-message", "regressions/basic")
	run(t, binary, "--workspace", workspaceRoot, "scenario", "validate", "regressions/basic")

	generated := run(t, binary,
		"--workspace", workspaceRoot,
		"--output", "json",
		"event", "generate", "chat.message.sent",
		"--content", "black-box",
	)
	var payload map[string]any
	if err := json.Unmarshal([]byte(generated), &payload); err != nil {
		t.Fatalf("decode generated payload: %v\n%s", err, generated)
	}
	if payload["content"] != "black-box" {
		t.Fatalf("generated content = %v", payload["content"])
	}

	publicKey, err := signing.ReadPublicKey(filepath.Join(workspaceRoot, "keys", "public-key.pem"))
	if err != nil {
		t.Fatal(err)
	}
	receiverErrors := make(chan error, 1)
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(io.LimitReader(request.Body, 1<<20))
		if err == nil {
			err = signing.Verify(
				publicKey,
				request.Header.Get(delivery.HeaderMessageID),
				request.Header.Get(delivery.HeaderTimestamp),
				body,
				request.Header.Get(delivery.HeaderSignature),
			)
		}
		if err != nil {
			receiverErrors <- err
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.Header.Get(delivery.HeaderSimulator) != "kick-sim" {
			receiverErrors <- fmt.Errorf("missing simulator marker")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receiver := &httptest.Server{Listener: listener, Config: &http.Server{Handler: handler}}
	receiver.Start()
	defer receiver.Close()

	delivered := run(t, binary,
		"--workspace", workspaceRoot,
		"--output", "json",
		"scenario", "run", "regressions/basic",
		"--destination-url", receiver.URL+"/webhooks/kick",
	)
	var result map[string]any
	if err := json.Unmarshal([]byte(delivered), &result); err != nil {
		t.Fatalf("decode delivery result: %v\n%s", err, delivered)
	}
	if result["status"] != float64(http.StatusNoContent) {
		t.Fatalf("delivery status = %v", result["status"])
	}
	select {
	case err := <-receiverErrors:
		t.Fatal(err)
	default:
	}
}

func TestCLIExitCodes(t *testing.T) {
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "kick-sim")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/kick-sim")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v\n%s", err, output)
	}

	command := exec.Command(binary, "--output", "xml", "version")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("invalid output format unexpectedly succeeded")
	}
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 2 {
		t.Fatalf("invalid usage exit = %v, output = %s", err, output)
	}

	command = exec.Command(binary, "workspace", "validate")
	command.Dir = t.TempDir()
	output, err = command.CombinedOutput()
	if err == nil {
		t.Fatal("missing workspace unexpectedly succeeded")
	}
	exitError, ok = err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 3 {
		t.Fatalf("workspace error exit = %v, output = %s", err, output)
	}
}

func run(t *testing.T, binary string, args ...string) string {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "KICK_SIM_WORKSPACE=")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", binary, strings.Join(args, " "), err, output)
	}
	return string(output)
}
