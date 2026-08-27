package studio

import (
	"context"
	"fmt"

	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/events"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/suite"
	"github.com/egekocabas/kick-sim/internal/version"
	"github.com/egekocabas/kick-sim/internal/workflow"
)

func (backend *Backend) ListScenarios(context.Context) ([]kickopenapi.ScenarioSummary, error) {
	entries, err := backend.scenarios.List()
	if err != nil {
		return nil, err
	}
	items := make([]kickopenapi.ScenarioSummary, len(entries))
	for index, entry := range entries {
		items[index] = scenarioSummary(entry)
	}
	return items, nil
}

func (backend *Backend) GetScenario(_ context.Context, id string) (kickopenapi.ScenarioDetail, error) {
	entry, err := backend.scenarios.Get(id)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) SaveScenarioCopy(_ context.Context, request kickopenapi.ScenarioCopyRequest) (kickopenapi.ScenarioDetail, error) {
	source, err := backend.scenarios.Get(request.SourceID)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	if err := backend.service.EventRegistry().Validate(source.Scenario.Request.Event.Type, source.Scenario.Request.Event.Version, request.Payload); err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	entry, err := backend.scenarios.SaveAsCopy(request.SourceID, request.TargetID, request.Payload, "kick-sim@"+version.Version)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) SaveScenarioSource(_ context.Context, request kickopenapi.ScenarioSourceSaveRequest) (kickopenapi.ScenarioDetail, error) {
	entry, err := backend.scenarios.SaveSource(request.ID, request.Revision, []byte(request.Source))
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) SaveScenarioSourceCopy(_ context.Context, request kickopenapi.ScenarioSourceCopyRequest) (kickopenapi.ScenarioDetail, error) {
	entry, err := backend.scenarios.SaveSourceAsCopy(request.SourceID, request.TargetID, []byte(request.Source), "kick-sim@"+version.Version)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) RunScenario(ctx context.Context, request kickopenapi.ScenarioRunRequest) (kickopenapi.DeliveryResult, error) {
	entry, err := backend.scenarios.Get(request.ScenarioID)
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	if err := backend.scenarios.Validate(entry); err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	if entry.Scenario.Kind() != "single" {
		return kickopenapi.DeliveryResult{}, fmt.Errorf("scenario %q is a timeline; use the workflow runner", entry.ID)
	}
	payload := request.Payload
	if payload == nil {
		payload, err = backend.service.GeneratePayload(app.PayloadOptions{
			EventType: entry.Scenario.Request.Event.Type, EventVersion: entry.Scenario.Request.Event.Version,
			Scenario: entry.Scenario.Request.Payload, Omit: entry.Scenario.Request.Omit,
		})
		if err != nil {
			return kickopenapi.DeliveryResult{}, err
		}
	}
	options, err := backend.deliveryOptions(kickopenapi.EventPayloadRequest{
		EventType: entry.Scenario.Request.Event.Type, EventVersion: entry.Scenario.Request.Event.Version, Payload: payload,
	})
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	options.ScenarioDefinitionID = entry.ID
	options.ScenarioSourceVersion = entry.SourceVersion
	subscriptionID := request.SubscriptionID
	if subscriptionID == "" {
		subscriptionID = entry.Scenario.Request.Delivery.SubscriptionID
	}
	generated, err := backend.service.Generate(options, subscriptionID)
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	destinationName := request.Destination
	if destinationName == "" {
		destinationName = entry.Scenario.Request.Delivery.Destination
	}
	result, err := backend.service.Deliver(ctx, generated, destinationName, request.DestinationURL, entry.Scenario.Request.Delivery.Expect.Statuses)
	return deliveryDTO(result), retainDeliveryResult(result, err)
}

func (backend *Backend) ListActors(context.Context) (map[string]actors.User, error) {
	return backend.service.ActorRegistry().List(), nil
}

func (backend *Backend) RunWorkflow(ctx context.Context, request kickopenapi.ScenarioRunRequest) (workflow.WorkflowResult, error) {
	entry, err := backend.scenarios.Get(request.ScenarioID)
	if err != nil {
		return workflow.WorkflowResult{}, err
	}
	if err := backend.scenarios.Validate(entry); err != nil {
		return workflow.WorkflowResult{}, err
	}
	return workflow.Run(ctx, backend.service, entry, workflow.Options{
		Destination: request.Destination, DestinationURL: request.DestinationURL,
		SubscriptionID: request.SubscriptionID, Payload: request.Payload,
	})
}

func (backend *Backend) ListSuites(context.Context) ([]kickopenapi.SuiteSummary, error) {
	entries, err := backend.suites.List()
	if err != nil {
		return nil, err
	}
	items := make([]kickopenapi.SuiteSummary, 0, len(entries))
	for _, entry := range entries {
		item := suiteSummary(entry)
		if validationErr := backend.suites.Validate(entry); validationErr != nil {
			item.Valid = false
			item.Error = validationErr.Error()
		}
		items = append(items, item)
	}
	return items, nil
}

func (backend *Backend) GetSuite(_ context.Context, id string) (kickopenapi.SuiteDetail, error) {
	entry, err := backend.suites.Get(id)
	if err != nil {
		return kickopenapi.SuiteDetail{}, err
	}
	summary := suiteSummary(entry)
	if validationErr := backend.suites.Validate(entry); validationErr != nil {
		summary.Valid = false
		summary.Error = validationErr.Error()
	}
	return kickopenapi.SuiteDetail{SuiteSummary: summary, Source: string(entry.Source)}, nil
}

func (backend *Backend) RunSuite(ctx context.Context, request kickopenapi.SuiteRunRequest) (suite.SuiteResult, error) {
	entry, err := backend.suites.Get(request.SuiteID)
	if err != nil {
		return suite.SuiteResult{}, err
	}
	if err := backend.suites.Validate(entry); err != nil {
		return suite.SuiteResult{}, err
	}
	return suite.Run(ctx, backend.service, backend.scenarios, entry, suite.RunOptions{Destination: request.Destination, DestinationURL: request.DestinationURL})
}

func (backend *Backend) scenarioDetail(entry scenario.Entry) (kickopenapi.ScenarioDetail, error) {
	detail := kickopenapi.ScenarioDetail{ScenarioSummary: scenarioSummary(entry), Source: string(entry.Source)}
	if err := backend.scenarios.Validate(entry); err != nil {
		return detail, nil
	}
	if entry.Scenario.Kind() == "timeline" {
		detail.Destination = entry.Scenario.Defaults.Destination
		detail.ExpectedStatuses = append([]int(nil), entry.Scenario.Defaults.Expect.Statuses...)
		return detail, nil
	}
	draft, err := backend.service.GeneratePayload(app.PayloadOptions{
		EventType: entry.Scenario.Request.Event.Type, EventVersion: entry.Scenario.Request.Event.Version,
		Scenario: entry.Scenario.Request.Payload, Omit: entry.Scenario.Request.Omit, Actors: entry.Scenario.Actors,
	})
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	detail.Payload = events.DeepCopyMap(entry.Scenario.Request.Payload)
	detail.DraftPayload = draft
	detail.Destination = entry.Scenario.Request.Delivery.Destination
	detail.ExpectedStatuses = append([]int(nil), entry.Scenario.Request.Delivery.Expect.Statuses...)
	return detail, nil
}
