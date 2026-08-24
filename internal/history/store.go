package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egekocabas/kick-sim/internal/config"
	_ "modernc.org/sqlite"
)

const schemaVersion = 1

type Store struct {
	db       *sql.DB
	path     string
	settings config.History
}

// OpenExisting opens retained history without creating runtime state for reads.
func OpenExisting(path string, settings config.History) (*Store, error) {
	if !settings.Enabled {
		return &Store{path: path, settings: settings}, nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return &Store{path: path, settings: settings}, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect history database: %w", err)
	}
	return Open(path, settings)
}

func Open(path string, settings config.History) (*Store, error) {
	if !settings.Enabled {
		return &Store{path: path, settings: settings}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create runtime directory: %w", err)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve history database path: %w", err)
	}
	info, statErr := os.Stat(absolute)
	existed := statErr == nil && info.Size() > 0
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect history database: %w", statErr)
	}

	databasePath := filepath.ToSlash(absolute)
	if filepath.VolumeName(absolute) != "" && !strings.HasPrefix(databasePath, "/") {
		databasePath = "/" + databasePath
	}
	databaseURL := &url.URL{Scheme: "file", Path: databasePath}
	query := databaseURL.Query()
	query.Set("_foreign_keys", "on")
	query.Set("_journal_mode", "WAL")
	query.Set("_busy_timeout", "5000")
	query.Set("_synchronous", "NORMAL")
	databaseURL.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return nil, fmt.Errorf("open history database: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	store := &Store{db: db, path: absolute, settings: settings}
	if err := store.prepare(context.Background(), existed); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (store *Store) Close() error {
	if store == nil || store.db == nil {
		return nil
	}
	return store.db.Close()
}

func (store *Store) Enabled() bool {
	return store != nil && store.db != nil && store.settings.Enabled
}

func (store *Store) Save(ctx context.Context, record Record) error {
	if !store.Enabled() {
		return nil
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history write: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO scenario_runs (
			id, scenario_definition_id, scenario_source_version, created_at
		) VALUES (?, ?, ?, ?)`,
		record.Run.ID,
		nullIfEmpty(record.Run.ScenarioDefinitionID),
		record.Run.ScenarioSourceVersion,
		formatTime(record.Run.CreatedAt),
	); err != nil {
		return fmt.Errorf("save scenario run: %w", err)
	}
	payload, err := json.Marshal(record.Event.Payload)
	if err != nil {
		return fmt.Errorf("serialize retained payload: %w", err)
	}
	headers, err := json.Marshal(record.Event.Headers)
	if err != nil {
		return fmt.Errorf("serialize retained headers: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO generated_events (
			id, run_id, event_type, event_version, broadcaster_user_id,
			logical_subscription, message_timestamp, payload_json, raw_body,
			headers_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Event.ID,
		record.Event.RunID,
		record.Event.EventType,
		record.Event.EventVersion,
		record.Event.BroadcasterUserID,
		record.Event.LogicalSubscription,
		record.Event.MessageTimestamp,
		payload,
		[]byte(record.Event.RawBody),
		headers,
		formatTime(record.Event.CreatedAt),
	); err != nil {
		return fmt.Errorf("save generated event: %w", err)
	}
	requestHeaders, err := json.Marshal(record.Attempt.RequestHeaders)
	if err != nil {
		return fmt.Errorf("serialize request headers: %w", err)
	}
	responseHeaders, err := json.Marshal(record.Attempt.ResponseHeaders)
	if err != nil {
		return fmt.Errorf("serialize response headers: %w", err)
	}
	responseBody := record.Attempt.ResponseBody
	if !store.settings.StoreResponseBodies {
		responseBody = ""
	}
	if limit := store.settings.MaxResponseBodyBytes; limit >= 0 && len(responseBody) > limit {
		responseBody = responseBody[:limit]
		record.Attempt.ResponseTruncated = true
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO delivery_attempts (
			id, generated_event_id, replay_of_id, replay_mode, destination, url,
			method, request_headers_json, response_status, response_headers_json,
			response_body, response_truncated, transport_started_at, duration_ms,
			outcome, error_text, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.Attempt.ID,
		record.Attempt.GeneratedEventID,
		nullIfEmpty(record.Attempt.ReplayOfID),
		nullIfEmpty(record.Attempt.ReplayMode),
		record.Attempt.Destination,
		record.Attempt.URL,
		record.Attempt.Method,
		requestHeaders,
		nullIfZero(record.Attempt.ResponseStatus),
		responseHeaders,
		[]byte(responseBody),
		boolInt(record.Attempt.ResponseTruncated),
		formatTime(record.Attempt.TransportStartedAt),
		record.Attempt.DurationMS,
		record.Attempt.Outcome,
		nullIfEmpty(record.Attempt.Error),
		formatTime(record.Attempt.CreatedAt),
	); err != nil {
		return fmt.Errorf("save delivery attempt: %w", err)
	}
	if err := store.prune(ctx, tx, record.Attempt.CreatedAt); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history write: %w", err)
	}
	return nil
}

func (store *Store) ListActivity(ctx context.Context, limit, offset int) ([]Activity, error) {
	if !store.Enabled() {
		return []Activity{}, nil
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := store.db.QueryContext(ctx, `
		SELECT a.id, r.id, COALESCE(r.scenario_definition_id, ''), e.event_type,
			e.event_version, COALESCE(a.response_status, 0), a.outcome,
			a.duration_ms, a.created_at
		FROM delivery_attempts a
		JOIN generated_events e ON e.id = a.generated_event_id
		JOIN scenario_runs r ON r.id = e.run_id
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list activity: %w", err)
	}
	defer rows.Close()
	activities := make([]Activity, 0, limit)
	for rows.Next() {
		var item Activity
		var createdAt string
		if err := rows.Scan(
			&item.AttemptID,
			&item.RunID,
			&item.ScenarioDefinitionID,
			&item.EventType,
			&item.EventVersion,
			&item.Status,
			&item.Outcome,
			&item.DurationMS,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		item.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse activity time: %w", err)
		}
		activities = append(activities, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read activity: %w", err)
	}
	return activities, nil
}

func (store *Store) GetAttempt(ctx context.Context, id string) (AttemptDetail, error) {
	if !store.Enabled() {
		return AttemptDetail{}, errors.New("history is disabled or empty")
	}
	row := store.db.QueryRowContext(ctx, `
		SELECT
			r.id, COALESCE(r.scenario_definition_id, ''), r.scenario_source_version, r.created_at,
			e.id, e.run_id, e.event_type, e.event_version, e.broadcaster_user_id,
			e.logical_subscription, e.message_timestamp, e.payload_json, e.raw_body,
			e.headers_json, e.created_at,
			a.id, a.generated_event_id, COALESCE(a.replay_of_id, ''), COALESCE(a.replay_mode, ''),
			a.destination, a.url, a.method, a.request_headers_json,
			COALESCE(a.response_status, 0), a.response_headers_json, a.response_body,
			a.response_truncated, a.transport_started_at, a.duration_ms, a.outcome,
			COALESCE(a.error_text, ''), a.created_at
		FROM delivery_attempts a
		JOIN generated_events e ON e.id = a.generated_event_id
		JOIN scenario_runs r ON r.id = e.run_id
		WHERE a.id = ?`, id)
	var detail AttemptDetail
	var runCreated, eventCreated, attemptCreated, transportStarted string
	var payloadJSON, eventHeadersJSON, requestHeadersJSON, responseHeadersJSON []byte
	var rawBody, responseBody []byte
	var responseTruncated int
	err := row.Scan(
		&detail.Run.ID,
		&detail.Run.ScenarioDefinitionID,
		&detail.Run.ScenarioSourceVersion,
		&runCreated,
		&detail.Event.ID,
		&detail.Event.RunID,
		&detail.Event.EventType,
		&detail.Event.EventVersion,
		&detail.Event.BroadcasterUserID,
		&detail.Event.LogicalSubscription,
		&detail.Event.MessageTimestamp,
		&payloadJSON,
		&rawBody,
		&eventHeadersJSON,
		&eventCreated,
		&detail.Attempt.ID,
		&detail.Attempt.GeneratedEventID,
		&detail.Attempt.ReplayOfID,
		&detail.Attempt.ReplayMode,
		&detail.Attempt.Destination,
		&detail.Attempt.URL,
		&detail.Attempt.Method,
		&requestHeadersJSON,
		&detail.Attempt.ResponseStatus,
		&responseHeadersJSON,
		&responseBody,
		&responseTruncated,
		&transportStarted,
		&detail.Attempt.DurationMS,
		&detail.Attempt.Outcome,
		&detail.Attempt.Error,
		&attemptCreated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AttemptDetail{}, fmt.Errorf("delivery attempt %q was not found", id)
	}
	if err != nil {
		return AttemptDetail{}, fmt.Errorf("read delivery attempt: %w", err)
	}
	if err := json.Unmarshal(payloadJSON, &detail.Event.Payload); err != nil {
		return AttemptDetail{}, fmt.Errorf("decode retained payload: %w", err)
	}
	if err := json.Unmarshal(eventHeadersJSON, &detail.Event.Headers); err != nil {
		return AttemptDetail{}, fmt.Errorf("decode retained event headers: %w", err)
	}
	if err := json.Unmarshal(requestHeadersJSON, &detail.Attempt.RequestHeaders); err != nil {
		return AttemptDetail{}, fmt.Errorf("decode retained request headers: %w", err)
	}
	if err := json.Unmarshal(responseHeadersJSON, &detail.Attempt.ResponseHeaders); err != nil {
		return AttemptDetail{}, fmt.Errorf("decode retained response headers: %w", err)
	}
	detail.Event.RawBody = string(rawBody)
	detail.Attempt.ResponseBody = string(responseBody)
	detail.Attempt.ResponseTruncated = responseTruncated != 0
	for _, item := range []struct {
		raw    string
		target *time.Time
	}{
		{runCreated, &detail.Run.CreatedAt},
		{eventCreated, &detail.Event.CreatedAt},
		{transportStarted, &detail.Attempt.TransportStartedAt},
		{attemptCreated, &detail.Attempt.CreatedAt},
	} {
		parsed, parseErr := time.Parse(time.RFC3339Nano, item.raw)
		if parseErr != nil {
			return AttemptDetail{}, fmt.Errorf("parse retained timestamp: %w", parseErr)
		}
		*item.target = parsed
	}
	return detail, nil
}

func (store *Store) DeleteRun(ctx context.Context, id string) error {
	if !store.Enabled() {
		return errors.New("history is disabled or empty")
	}
	result, err := store.db.ExecContext(ctx, `DELETE FROM scenario_runs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete run: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted run count: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("run %q was not found", id)
	}
	return nil
}

func (store *Store) prepare(ctx context.Context, existed bool) error {
	if err := store.db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to history database: %w", err)
	}
	var integrity string
	if err := store.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("validate history database integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("history database integrity check failed: %s", integrity)
	}
	if _, err := store.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		return fmt.Errorf("prepare migration registry: %w", err)
	}
	var current int
	if err := store.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("read history schema version: %w", err)
	}
	if current > schemaVersion {
		return fmt.Errorf("history database schema %d is newer than supported schema %d", current, schemaVersion)
	}
	if current == schemaVersion {
		return nil
	}
	if existed {
		if err := store.backup(ctx); err != nil {
			return err
		}
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin history migration: %w", err)
	}
	defer tx.Rollback()
	if current < 1 {
		if _, err := tx.ExecContext(ctx, migrationOne); err != nil {
			return fmt.Errorf("apply history migration: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, 1, formatTime(time.Now().UTC())); err != nil {
			return fmt.Errorf("record history migration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit history migration: %w", err)
	}
	return nil
}

func (store *Store) backup(ctx context.Context) error {
	if _, err := store.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return fmt.Errorf("checkpoint history database before migration: %w", err)
	}
	source, err := os.Open(store.path)
	if err != nil {
		return fmt.Errorf("open history database for backup: %w", err)
	}
	defer source.Close()
	backupPath := fmt.Sprintf("%s.backup-%s", store.path, time.Now().UTC().Format("20060102T150405.000000000Z"))
	destination, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create history migration backup: %w", err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		_ = destination.Close()
		_ = os.Remove(backupPath)
		return fmt.Errorf("write history migration backup: %w", err)
	}
	if err := destination.Sync(); err != nil {
		_ = destination.Close()
		_ = os.Remove(backupPath)
		return fmt.Errorf("sync history migration backup: %w", err)
	}
	if err := destination.Close(); err != nil {
		_ = os.Remove(backupPath)
		return fmt.Errorf("close history migration backup: %w", err)
	}
	return nil
}

func (store *Store) prune(ctx context.Context, tx *sql.Tx, now time.Time) error {
	maxAge, err := time.ParseDuration(store.settings.MaxAge)
	if err != nil {
		return fmt.Errorf("parse history retention age: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM scenario_runs WHERE created_at < ?`, formatTime(now.Add(-maxAge))); err != nil {
		return fmt.Errorf("prune history by age: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM scenario_runs
		WHERE id IN (
			SELECT id FROM scenario_runs
			ORDER BY created_at DESC, id DESC
			LIMIT -1 OFFSET ?
		)`, store.settings.MaxFunctionalRuns); err != nil {
		return fmt.Errorf("prune history by count: %w", err)
	}
	return nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		value = time.Now().UTC()
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullIfZero(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

const migrationOne = `
CREATE TABLE scenario_runs (
	id TEXT PRIMARY KEY,
	scenario_definition_id TEXT,
	scenario_source_version INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);

CREATE TABLE generated_events (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL REFERENCES scenario_runs(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL,
	event_version INTEGER NOT NULL,
	broadcaster_user_id INTEGER NOT NULL,
	logical_subscription TEXT NOT NULL,
	message_timestamp TEXT NOT NULL,
	payload_json BLOB NOT NULL,
	raw_body BLOB NOT NULL,
	headers_json BLOB NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE delivery_attempts (
	id TEXT PRIMARY KEY,
	generated_event_id TEXT NOT NULL REFERENCES generated_events(id) ON DELETE CASCADE,
	replay_of_id TEXT REFERENCES delivery_attempts(id) ON DELETE SET NULL,
	replay_mode TEXT,
	destination TEXT NOT NULL,
	url TEXT NOT NULL,
	method TEXT NOT NULL,
	request_headers_json BLOB NOT NULL,
	response_status INTEGER,
	response_headers_json BLOB NOT NULL,
	response_body BLOB NOT NULL,
	response_truncated INTEGER NOT NULL DEFAULT 0,
	transport_started_at TEXT NOT NULL,
	duration_ms REAL NOT NULL,
	outcome TEXT NOT NULL,
	error_text TEXT,
	created_at TEXT NOT NULL
);

CREATE INDEX delivery_attempts_activity_idx ON delivery_attempts(created_at DESC, id DESC);
CREATE INDEX generated_events_run_idx ON generated_events(run_id);
`
