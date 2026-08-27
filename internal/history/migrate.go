package history

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

func (store *Store) prepare(existed bool) error {
	ctx := context.Background()
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
	// Existing databases are backed up only when a migration will run; newer
	// schemas are rejected without touching the file.
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
