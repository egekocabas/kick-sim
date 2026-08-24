package workflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestTimelineUsesOneRunnerAndDeterministicLogicalTime(t *testing.T) {
	t.Parallel()
	receiver := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusAccepted) }))
	defer receiver.Close()
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	store := scenario.NewStore(root, service.Events, service.Config, service.Actors)
	entry, err := store.Get("builtin:workflows/complete-stream-session")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Validate(entry); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	report, err := Run(context.Background(), service, entry, Options{DestinationURL: receiver.URL, Start: start, SkipWait: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || len(report.Deliveries) != 7 {
		t.Fatalf("report = %#v", report)
	}
	if report.Deliveries[0].Result.MessageTimestamp != start.Format(time.RFC3339Nano) {
		t.Fatalf("first timestamp = %s", report.Deliveries[0].Result.MessageTimestamp)
	}
	wantLast := start.Add(450 * time.Millisecond).Format(time.RFC3339Nano)
	if report.Deliveries[len(report.Deliveries)-1].Result.MessageTimestamp != wantLast {
		t.Fatalf("last timestamp = %s, want %s", report.Deliveries[len(report.Deliveries)-1].Result.MessageTimestamp, wantLast)
	}
}
