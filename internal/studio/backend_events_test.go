package studio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/egekocabas/kick-sim/internal/app"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestStudioPreviewSendAndCopyEveryEventContract(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	backend := NewBackend(service)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if err := service.EventRegistry().Validate(r.Header.Get("Kick-Event-Type"), 1, payload); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()
	ctx := context.Background()
	for _, id := range []string{"chat/basic-message", "channel/new-follower", "livestream/started", "livestream/stopped", "moderation/ban"} {
		t.Run(id, func(t *testing.T) {
			detail, err := backend.GetScenario(ctx, "builtin:"+id)
			if err != nil {
				t.Fatal(err)
			}
			original, _ := json.Marshal(detail.DraftPayload)
			request := kickopenapi.EventDeliveryRequest{EventPayloadRequest: kickopenapi.EventPayloadRequest{EventType: detail.EventType, EventVersion: detail.EventVersion, Payload: detail.DraftPayload}, DestinationURL: receiver.URL}
			preview, err := backend.GenerateEvent(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(preview.RawHTTP, "Kick-Event-Signature:") {
				t.Fatalf("unsigned preview: %s", preview.RawHTTP)
			}
			result, err := backend.TriggerEvent(ctx, request)
			if err != nil || result.Status != 204 {
				t.Fatalf("send: %+v, %v", result, err)
			}
			after, _ := json.Marshal(detail.DraftPayload)
			if string(after) != string(original) {
				t.Fatal("request payload mutated")
			}
			saved, err := backend.SaveScenarioCopy(ctx, kickopenapi.ScenarioCopyRequest{SourceID: detail.ID, TargetID: id + "-copy", Name: "My " + id, Payload: detail.DraftPayload})
			if err != nil {
				t.Fatal(err)
			}
			if saved.BuiltIn || saved.Name != "My "+id || !saved.Valid {
				t.Fatalf("copy: %+v", saved)
			}
			if !reflect.DeepEqual(saved.Payload["content"], detail.DraftPayload["content"]) {
				t.Fatal("copy lost payload edits")
			}
			sourceCopy, err := backend.SaveScenarioSourceCopy(ctx, kickopenapi.ScenarioSourceCopyRequest{SourceID: saved.ID, TargetID: id + "-source-copy", Name: "Source copy", Source: saved.Source})
			if err != nil || sourceCopy.Name != "Source copy" {
				t.Fatalf("source copy: %+v, %v", sourceCopy, err)
			}
		})
	}
}
