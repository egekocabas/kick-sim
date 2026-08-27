package suite

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestDeliverySuiteVerifiesDuplicateResponseBehavior(t *testing.T) {
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
	scenarios := scenario.NewStore(root, service.EventRegistry(), service.Configuration(), service.ActorRegistry())
	store := NewStore(root, scenarios)
	entry, err := store.Get("builtin:delivery")
	if err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), service, scenarios, entry, RunOptions{DestinationURL: receiver.URL, SkipWait: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || report.PassedCases != 2 || report.FailedCases != 0 {
		t.Fatalf("report = %#v", report)
	}
	duplicate := report.Cases[1].Workflow.Deliveries
	if len(duplicate) != 2 || duplicate[0].Result.MessageID != duplicate[1].Result.MessageID {
		t.Fatalf("duplicate deliveries = %#v", duplicate)
	}
}
