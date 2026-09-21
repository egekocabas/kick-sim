package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestHumanListsHaveAlignedColumns(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"event", "scenario", "suite"} {
		output, _, err := runCommand("--workspace", root, command, "list")
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) < 2 || !strings.HasPrefix(lines[0], strings.ToUpper(command)) || strings.Contains(output, "\t") {
			t.Fatalf("unaligned %s list: %s", command, output)
		}
		if command == "event" {
			column := strings.Index(lines[0], "DESCRIPTION")
			for _, line := range lines[1:] {
				if strings.Index(line, "Fired") != column {
					t.Fatalf("description not aligned: %q", line)
				}
			}
		}
	}
}

func TestHistoryExplainsResponsesAndTransportErrors(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{204, 200, 401, 0} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Receiver", "regression")
				w.WriteHeader(status)
				if status == 401 {
					_, _ = w.Write([]byte("rejected by receiver"))
				}
			}))
			defer receiver.Close()
			if status == 0 {
				receiver.Close()
			}
			generated, err := service.Generate(app.PayloadOptions{EventType: events.ChatMessageSentType, EventVersion: 1}, "")
			if err != nil {
				t.Fatal(err)
			}
			result, sendErr := service.Deliver(context.Background(), generated, "", receiver.URL, nil)
			if status == 0 && sendErr == nil {
				t.Fatal("expected connection failure")
			}
			output, _, err := runCommand("--workspace", root, "history", "show", result.AttemptID)
			if err != nil {
				t.Fatal(err)
			}
			input := string(signing.SignatureInput(generated.MessageID, generated.MessageTimestamp, []byte(generated.RawBody)))
			if !strings.Contains(output, "Signature input:\n"+input) {
				t.Fatalf("incorrect signature input: %s", output)
			}
			for _, want := range map[int][]string{
				204: {"HTTP 204 No Content", "X-Receiver: regression", "(empty — HTTP 204 No Content)"},
				200: {"HTTP 200 OK", "Response body:\n(empty)"},
				401: {"HTTP 401 Unauthorized", "rejected by receiver"},
				0:   {"no HTTP response", "No HTTP response received", "Error: ", "connection refused"},
			}[status] {
				if !strings.Contains(output, want) {
					t.Errorf("missing %q in %s", want, output)
				}
			}
			machine, _, err := runCommand("--workspace", root, "--output", "json", "history", "show", result.AttemptID)
			if err != nil || !json.Valid([]byte(machine)) {
				t.Fatalf("JSON output: %s, %v", machine, err)
			}
		})
	}
	output, _, err := runCommand("--workspace", root, "history", "list")
	if err != nil || !strings.HasPrefix(output, "ATTEMPT") || strings.Contains(output, "\t") {
		t.Fatalf("history list: %s, %v", output, err)
	}
}

func TestScenarioRunInjectsStaleTimestamp(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	timestamps := make(chan time.Time, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamp, err := time.Parse(time.RFC3339Nano, r.Header.Get("Kick-Event-Message-Timestamp"))
		if err != nil {
			t.Error(err)
		}
		timestamps <- timestamp
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer receiver.Close()
	output, _, err := runCommand("--workspace", root, "scenario", "run", "builtin:delivery/stale-timestamp", "--destination-url", receiver.URL)
	if err != nil {
		t.Fatalf("scenario: %s, %v", output, err)
	}
	if age := time.Since(<-timestamps); age < 23*time.Hour || age > 25*time.Hour {
		t.Fatalf("timestamp age = %s", age)
	}
}
