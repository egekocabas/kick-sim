package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

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
		if err := rows.Scan(&item.AttemptID, &item.RunID, &item.ScenarioDefinitionID,
			&item.EventType, &item.EventVersion, &item.Status, &item.Outcome,
			&item.DurationMS, &createdAt); err != nil {
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
		&detail.Run.ID, &detail.Run.ScenarioDefinitionID, &detail.Run.ScenarioSourceVersion, &runCreated,
		&detail.Event.ID, &detail.Event.RunID, &detail.Event.EventType, &detail.Event.EventVersion,
		&detail.Event.BroadcasterUserID, &detail.Event.LogicalSubscription, &detail.Event.MessageTimestamp,
		&payloadJSON, &rawBody, &eventHeadersJSON, &eventCreated,
		&detail.Attempt.ID, &detail.Attempt.GeneratedEventID, &detail.Attempt.ReplayOfID, &detail.Attempt.ReplayMode,
		&detail.Attempt.Destination, &detail.Attempt.URL, &detail.Attempt.Method, &requestHeadersJSON,
		&detail.Attempt.ResponseStatus, &responseHeadersJSON, &responseBody, &responseTruncated,
		&transportStarted, &detail.Attempt.DurationMS, &detail.Attempt.Outcome, &detail.Attempt.Error, &attemptCreated,
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
		{runCreated, &detail.Run.CreatedAt}, {eventCreated, &detail.Event.CreatedAt},
		{transportStarted, &detail.Attempt.TransportStartedAt}, {attemptCreated, &detail.Attempt.CreatedAt},
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
