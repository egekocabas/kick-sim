package history

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egekocabas/kick-sim/internal/config"
)

func TestNewerDatabaseIsRejectedWithoutMigration(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "kick-sim.db")
	settings := config.Default().History
	store, err := Open(path, settings)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE schema_migrations SET version = ?`, schemaVersion+1); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(path, settings); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("Open() error = %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion+1 {
		t.Fatalf("schema version = %d, want %d", version, schemaVersion+1)
	}
	backups, err := filepath.Glob(path + ".backup-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 0 {
		t.Fatalf("newer database created migration backup: %#v", backups)
	}
}

func TestExistingDatabaseIsBackedUpBeforeMigration(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "kick-sim.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
		CREATE TABLE legacy_marker (value TEXT NOT NULL);
		INSERT INTO legacy_marker (value) VALUES ('preserve-me');
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path, config.Default().History)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".backup-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("migration backups = %#v", backups)
	}
	backup, err := sql.Open("sqlite", backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var marker string
	if err := backup.QueryRow(`SELECT value FROM legacy_marker`).Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if marker != "preserve-me" {
		t.Fatalf("backup marker = %q", marker)
	}
}

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

func TestStoreRejectsMissingRequiredTimestamps(t *testing.T) {
	t.Parallel()
	store, err := Open(filepath.Join(t.TempDir(), "kick-sim.db"), config.Default().History)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	record := fixtureRecord(time.Now().UTC())
	record.Attempt.TransportStartedAt = time.Time{}
	if err := store.Save(context.Background(), record); err == nil || !strings.Contains(err.Error(), "transport_started_at") {
		t.Fatalf("Save() error = %v", err)
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
