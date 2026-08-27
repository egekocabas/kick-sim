package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/history"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func (service *Service) History() (*history.Store, error) {
	return history.OpenExisting(workspace.PathsFor(service.workspaceRoot).Database, service.configuration.History)
}

func (service *Service) Replay(ctx context.Context, attemptID, mode string) (RunResult, error) {
	store, err := service.History()
	if err != nil {
		return RunResult{}, err
	}
	defer store.Close()
	detail, err := store.GetAttempt(ctx, attemptID)
	if err != nil {
		return RunResult{}, err
	}
	switch mode {
	case "exact":
		generated := Generated{
			RunID: detail.Run.ID, GeneratedEventID: detail.Event.ID,
			ScenarioDefinitionID: detail.Run.ScenarioDefinitionID, ScenarioSourceVersion: detail.Run.ScenarioSourceVersion,
			EventType: detail.Event.EventType, EventVersion: detail.Event.EventVersion,
			BroadcasterUserID: detail.Event.BroadcasterUserID, LogicalSubscription: detail.Event.LogicalSubscription,
			MessageID: detail.Event.Headers[delivery.HeaderMessageID], SubscriptionID: detail.Event.Headers[delivery.HeaderSubscriptionID],
			MessageTimestamp: detail.Event.MessageTimestamp, Headers: detail.Event.Headers,
			Payload: detail.Event.Payload, RawBody: detail.Event.RawBody, CreatedAt: detail.Event.CreatedAt,
			body: []byte(detail.Event.RawBody),
		}
		return service.deliver(ctx, generated, detail.Attempt.Destination, detail.Attempt.URL, nil, attemptID, mode)
	case "regenerated":
		payload := events.DeepCopyMap(detail.Event.Payload)
		if _, exists := payload["message_id"]; exists {
			payload["message_id"] = "{{ ulid() }}"
		}
		if _, exists := payload["created_at"]; exists {
			payload["created_at"] = "{{ now() }}"
		}
		generated, err := service.Generate(PayloadOptions{
			EventType: detail.Event.EventType, EventVersion: detail.Event.EventVersion, Scenario: payload,
			ScenarioDefinitionID: detail.Run.ScenarioDefinitionID, ScenarioSourceVersion: detail.Run.ScenarioSourceVersion,
		}, "")
		if err != nil {
			return RunResult{}, err
		}
		return service.deliver(ctx, generated, detail.Attempt.Destination, detail.Attempt.URL, nil, attemptID, mode)
	default:
		return RunResult{}, errors.New("replay mode must be exact or regenerated")
	}
}

func (service *Service) record(ctx context.Context, run RunResult) error {
	if !service.configuration.History.Enabled {
		return nil
	}
	store, err := history.Open(workspace.PathsFor(service.workspaceRoot).Database, service.configuration.History)
	if err != nil {
		return fmt.Errorf("open delivery history: %w", err)
	}
	defer store.Close()
	createdAt := service.Now().UTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = createdAt
	}
	return store.Save(ctx, history.Record{
		Run: history.Run{ID: run.RunID, ScenarioDefinitionID: run.ScenarioDefinitionID, ScenarioSourceVersion: run.ScenarioSourceVersion, CreatedAt: run.CreatedAt},
		Event: history.Event{
			ID: run.GeneratedEventID, RunID: run.RunID, EventType: run.EventType, EventVersion: run.EventVersion,
			BroadcasterUserID: run.BroadcasterUserID, LogicalSubscription: run.LogicalSubscription,
			MessageTimestamp: run.MessageTimestamp, Payload: run.Payload, RawBody: run.RawBody,
			Headers: run.Headers, CreatedAt: run.CreatedAt,
		},
		Attempt: history.Attempt{
			ID: run.AttemptID, GeneratedEventID: run.GeneratedEventID, ReplayOfID: run.ReplayOfID, ReplayMode: run.ReplayMode,
			Destination: run.Destination, URL: run.URL, Method: run.Method, RequestHeaders: run.Headers,
			ResponseStatus: run.Status, ResponseHeaders: run.ResponseHeaders, ResponseBody: run.ResponseBody,
			ResponseTruncated: run.ResponseTruncated, TransportStartedAt: run.TransportStartedAt,
			DurationMS: run.DurationMS, Outcome: run.Outcome, Error: run.Error, CreatedAt: createdAt,
		},
	})
}
