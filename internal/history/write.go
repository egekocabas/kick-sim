package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (store *Store) Save(ctx context.Context, record Record) error {
	if !store.Enabled() {
		return nil
	}
	for _, required := range []struct {
		name  string
		value time.Time
	}{
		{"run created_at", record.Run.CreatedAt},
		{"event created_at", record.Event.CreatedAt},
		{"attempt transport_started_at", record.Attempt.TransportStartedAt},
		{"attempt created_at", record.Attempt.CreatedAt},
	} {
		if required.value.IsZero() {
			return fmt.Errorf("history %s is required", required.name)
		}
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
		record.Run.ID, nullIfEmpty(record.Run.ScenarioDefinitionID),
		record.Run.ScenarioSourceVersion, formatTime(record.Run.CreatedAt),
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
		record.Event.ID, record.Event.RunID, record.Event.EventType, record.Event.EventVersion,
		record.Event.BroadcasterUserID, record.Event.LogicalSubscription, record.Event.MessageTimestamp,
		payload, []byte(record.Event.RawBody), headers, formatTime(record.Event.CreatedAt),
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
		record.Attempt.ID, record.Attempt.GeneratedEventID, nullIfEmpty(record.Attempt.ReplayOfID),
		nullIfEmpty(record.Attempt.ReplayMode), record.Attempt.Destination, record.Attempt.URL,
		record.Attempt.Method, requestHeaders, nullIfZero(record.Attempt.ResponseStatus), responseHeaders,
		[]byte(responseBody), boolInt(record.Attempt.ResponseTruncated),
		formatTime(record.Attempt.TransportStartedAt), record.Attempt.DurationMS,
		record.Attempt.Outcome, nullIfEmpty(record.Attempt.Error), formatTime(record.Attempt.CreatedAt),
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

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

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
