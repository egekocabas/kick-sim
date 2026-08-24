package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestReplaySurvivesRestartAndPreservesOrRegeneratesIdentity(t *testing.T) {
	t.Parallel()
	type received struct {
		body    string
		headers http.Header
	}
	requests := make(chan received, 3)
	receiver := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		requests <- received{body: string(body), headers: request.Header.Clone()}
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer receiver.Close()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := service.Generate(PayloadOptions{EventType: events.ChatMessageSentType, EventVersion: events.ChatMessageSentVersion}, "")
	if err != nil {
		t.Fatal(err)
	}
	originalResult, err := service.Deliver(context.Background(), generated, "", receiver.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := <-requests

	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	exactResult, err := reopened.Replay(context.Background(), originalResult.AttemptID, "exact")
	if err != nil {
		t.Fatal(err)
	}
	exact := <-requests
	if exact.body != original.body || exact.headers.Get(delivery.HeaderMessageID) != original.headers.Get(delivery.HeaderMessageID) || exact.headers.Get(delivery.HeaderSignature) != original.headers.Get(delivery.HeaderSignature) {
		t.Fatalf("exact replay changed signed request\noriginal: %#v\nexact: %#v", original, exact)
	}
	if exactResult.ReplayOfID != originalResult.AttemptID || exactResult.ReplayMode != "exact" {
		t.Fatalf("exact replay provenance = %#v", exactResult)
	}

	regeneratedResult, err := reopened.Replay(context.Background(), originalResult.AttemptID, "regenerated")
	if err != nil {
		t.Fatal(err)
	}
	regenerated := <-requests
	if regenerated.headers.Get(delivery.HeaderMessageID) == original.headers.Get(delivery.HeaderMessageID) || regenerated.headers.Get(delivery.HeaderSignature) == original.headers.Get(delivery.HeaderSignature) {
		t.Fatal("regenerated replay reused signed identity")
	}
	if regeneratedResult.ReplayOfID != originalResult.AttemptID || regeneratedResult.ReplayMode != "regenerated" {
		t.Fatalf("regenerated replay provenance = %#v", regeneratedResult)
	}
}

func TestGenerateUsesOneLogicalTimestampAndExactSignedBody(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	fixedTime := time.Date(2026, time.August, 24, 10, 15, 30, 123000000, time.UTC)
	service.Now = func() time.Time { return fixedTime }
	ids := []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"01ARZ3NDEKTSV4RRFFQ69G5FAW",
		"01ARZ3NDEKTSV4RRFFQ69G5FAX",
		"01ARZ3NDEKTSV4RRFFQ69G5FAY",
		"01ARZ3NDEKTSV4RRFFQ69G5FAZ",
	}
	service.NewID = func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}

	generated, err := service.Generate(PayloadOptions{
		EventType:    events.ChatMessageSentType,
		EventVersion: events.ChatMessageSentVersion,
		SemanticValues: map[string]any{
			"/payload/content": "deterministic",
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if generated.Payload["created_at"] != generated.MessageTimestamp {
		t.Fatalf("payload timestamp = %v, header timestamp = %s", generated.Payload["created_at"], generated.MessageTimestamp)
	}
	privateKey, err := signing.ReadPrivateKey(workspace.PathsFor(root).PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := signing.Verify(
		&privateKey.PublicKey,
		generated.MessageID,
		generated.MessageTimestamp,
		[]byte(generated.RawBody),
		generated.Headers[delivery.HeaderSignature],
	); err != nil {
		t.Fatal(err)
	}
	if generated.Payload["content"] != "deterministic" {
		t.Fatalf("content = %v", generated.Payload["content"])
	}
}

func TestDeliverUsesScenarioExpectedStatuses(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := service.Generate(PayloadOptions{
		EventType:    events.ChatMessageSentType,
		EventVersion: events.ChatMessageSentVersion,
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	receiver := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusConflict)
	}))
	defer receiver.Close()

	result, err := service.Deliver(context.Background(), generated, "", receiver.URL, []int{http.StatusConflict})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != http.StatusConflict {
		t.Fatalf("status = %d, want %d", result.Status, http.StatusConflict)
	}
	if _, err := service.Deliver(context.Background(), generated, "", receiver.URL, nil); err == nil {
		t.Fatal("Deliver() accepted a non-success status without an explicit expectation")
	}
}

func TestGenerateRejectsConflictingOverrides(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.GeneratePayload(PayloadOptions{
		EventType:      events.ChatMessageSentType,
		EventVersion:   events.ChatMessageSentVersion,
		SemanticValues: map[string]any{"/payload/content": "one"},
		StringValues:   map[string]string{"/payload/content": "two"},
	})
	if err == nil {
		t.Fatal("GeneratePayload() accepted conflicting overrides")
	}
}

func TestBroadcasterIDRequiresPositiveInteger(t *testing.T) {
	t.Parallel()

	for _, value := range []any{int(0), int64(-1), uint64(0), float64(0)} {
		_, err := broadcasterID(map[string]any{"broadcaster": map[string]any{"user_id": value}})
		if err == nil {
			t.Fatalf("broadcasterID(%T(%v)) accepted a non-positive value", value, value)
		}
	}
}
