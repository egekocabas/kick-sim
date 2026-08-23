package app

import (
	"context"
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
