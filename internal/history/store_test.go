package history

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egekocabas/kick-sim/internal/config"
)

func TestReadDoesNotCreateDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".runtime", "kick-sim.db")
	store, err := OpenExisting(path, config.Default().History)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.ListActivity(context.Background(), 10, 0)
	if err != nil || len(items) != 0 {
		t.Fatalf("ListActivity() = %#v, %v", items, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("read created database: %v", err)
	}
}

func TestStorePersistsActivityAndAttemptAcrossRestart(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".runtime", "kick-sim.db")
	settings := config.Default().History
	now := time.Date(2026, time.August, 24, 10, 15, 30, 123000000, time.UTC)
	record := fixtureRecord(now)
	store, err := Open(path, settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path, settings)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	activity, err := reopened.ListActivity(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(activity) != 1 || activity[0].AttemptID != record.Attempt.ID {
		t.Fatalf("activity = %#v", activity)
	}
	detail, err := reopened.GetAttempt(context.Background(), record.Attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Event.RawBody != record.Event.RawBody || detail.Attempt.ResponseBody != record.Attempt.ResponseBody {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestStorePrunesFunctionalRunsByCount(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kick-sim.db")
	settings := config.Default().History
	settings.MaxFunctionalRuns = 1
	store, err := Open(path, settings)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first := fixtureRecord(time.Date(2026, time.August, 24, 10, 0, 0, 0, time.UTC))
	second := fixtureRecord(time.Date(2026, time.August, 24, 11, 0, 0, 0, time.UTC))
	second.Run.ID = "01ARZ3NDEKTSV4RRFFQ69G5FB1"
	second.Event.ID = "01ARZ3NDEKTSV4RRFFQ69G5FB2"
	second.Event.RunID = second.Run.ID
	second.Attempt.ID = "01ARZ3NDEKTSV4RRFFQ69G5FB3"
	second.Attempt.GeneratedEventID = second.Event.ID
	if err := store.Save(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAttempt(context.Background(), first.Attempt.ID); err == nil {
		t.Fatal("old run was not pruned")
	}
	if _, err := store.GetAttempt(context.Background(), second.Attempt.ID); err != nil {
		t.Fatal(err)
	}
}

func fixtureRecord(now time.Time) Record {
	return Record{
		Run: Run{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", ScenarioDefinitionID: "builtin:chat/basic-message", ScenarioSourceVersion: 1, CreatedAt: now},
		Event: Event{
			ID: "01ARZ3NDEKTSV4RRFFQ69G5FAW", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
			EventType: "chat.message.sent", EventVersion: 1, BroadcasterUserID: 123,
			LogicalSubscription: "123:chat.message.sent:1", MessageTimestamp: now.Format(time.RFC3339Nano),
			Payload: map[string]any{"content": "hello"}, RawBody: `{"content":"hello"}`,
			Headers: map[string]string{"Content-Type": "application/json"}, CreatedAt: now,
		},
		Attempt: Attempt{
			ID: "01ARZ3NDEKTSV4RRFFQ69G5FAX", GeneratedEventID: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
			Destination: "local", URL: "http://127.0.0.1:3000/webhooks/kick", Method: "POST",
			RequestHeaders: map[string]string{"Content-Type": "application/json"}, ResponseStatus: 202,
			ResponseHeaders: map[string][]string{"Content-Type": {"application/json"}}, ResponseBody: `{"ok":true}`,
			TransportStartedAt: now, DurationMS: 12.5, Outcome: "http_accepted", CreatedAt: now,
		},
	}
}
