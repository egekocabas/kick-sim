package openapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/history"
	"github.com/egekocabas/kick-sim/internal/suite"
	"github.com/egekocabas/kick-sim/internal/workflow"
)

// Register attaches every Studio operation to api.
func Register(api huma.API, backend Backend) {
	register[struct{}, Bootstrap](api, http.MethodGet, "/api/bootstrap", "getBootstrap", "Get Studio bootstrap data", func(ctx context.Context, _ *struct{}) (Bootstrap, error) {
		return backend.Bootstrap(ctx)
	})
	register[struct{}, Workspace](api, http.MethodGet, "/api/workspace", "getWorkspace", "Get the active workspace", func(ctx context.Context, _ *struct{}) (Workspace, error) {
		return backend.Workspace(ctx)
	})
	register[struct{}, ValidResponse](api, http.MethodPost, "/api/workspace/validate", "validateWorkspace", "Validate the active workspace", func(ctx context.Context, _ *struct{}) (ValidResponse, error) {
		return ValidResponse{Valid: true}, backend.ValidateWorkspace(ctx)
	})
	register[struct{}, CapabilitiesResponse](api, http.MethodGet, "/api/capabilities", "getCapabilities", "List implemented capabilities", func(ctx context.Context, _ *struct{}) (CapabilitiesResponse, error) {
		bootstrap, err := backend.Bootstrap(ctx)
		return CapabilitiesResponse{APIVersion: bootstrap.APIVersion, Capabilities: bootstrap.Capabilities}, err
	})
	register[struct{}, EventsResponse](api, http.MethodGet, "/api/events", "listEvents", "List event contracts", func(ctx context.Context, _ *struct{}) (EventsResponse, error) {
		items, err := backend.ListEvents(ctx)
		return EventsResponse{Items: items}, err
	})
	register[EventInput, EventContract](api, http.MethodGet, "/api/events/{type}/versions/{version}", "getEvent", "Get an event contract", func(ctx context.Context, input *EventInput) (EventContract, error) {
		return backend.GetEvent(ctx, input.Type, input.Version)
	})
	registerBody[EventPayloadRequest, ValidResponse](api, http.MethodPost, "/api/events/validate", "validateEvent", "Validate an event payload", func(ctx context.Context, body EventPayloadRequest) (ValidResponse, error) {
		return ValidResponse{Valid: true}, backend.ValidatePayload(ctx, body)
	})
	registerBody[EventDeliveryRequest, GeneratedEvent](api, http.MethodPost, "/api/events/generate", "generateEvent", "Generate and sign an event preview", func(ctx context.Context, body EventDeliveryRequest) (GeneratedEvent, error) {
		return backend.GenerateEvent(ctx, body)
	})
	registerBody[EventDeliveryRequest, DeliveryResult](api, http.MethodPost, "/api/events/trigger", "triggerEvent", "Generate and deliver an event", func(ctx context.Context, body EventDeliveryRequest) (DeliveryResult, error) {
		return backend.TriggerEvent(ctx, body)
	})
	register[struct{}, ScenariosResponse](api, http.MethodGet, "/api/scenarios", "listScenarios", "List built-in and custom scenarios", func(ctx context.Context, _ *struct{}) (ScenariosResponse, error) {
		items, err := backend.ListScenarios(ctx)
		return ScenariosResponse{Items: items}, err
	})
	register[ScenarioInput, ScenarioDetail](api, http.MethodGet, "/api/scenario", "getScenario", "Get a scenario", func(ctx context.Context, input *ScenarioInput) (ScenarioDetail, error) {
		return backend.GetScenario(ctx, input.ID)
	})
	registerBody[ScenarioCopyRequest, ScenarioDetail](api, http.MethodPost, "/api/scenario-duplicates", "duplicateScenario", "Save a scenario working copy", func(ctx context.Context, body ScenarioCopyRequest) (ScenarioDetail, error) {
		return backend.SaveScenarioCopy(ctx, body)
	})
	registerBody[ScenarioSourceSaveRequest, ScenarioDetail](api, http.MethodPut, "/api/scenario", "updateScenarioSource", "Validate and save scenario source", func(ctx context.Context, body ScenarioSourceSaveRequest) (ScenarioDetail, error) {
		return backend.SaveScenarioSource(ctx, body)
	})
	registerBody[ScenarioSourceCopyRequest, ScenarioDetail](api, http.MethodPost, "/api/scenario-source-copies", "saveScenarioSourceCopy", "Validate and save scenario source as a canonical copy", func(ctx context.Context, body ScenarioSourceCopyRequest) (ScenarioDetail, error) {
		return backend.SaveScenarioSourceCopy(ctx, body)
	})
	registerBody[ScenarioRunRequest, DeliveryResult](api, http.MethodPost, "/api/scenario-runs", "runScenario", "Run a scenario working copy", func(ctx context.Context, body ScenarioRunRequest) (DeliveryResult, error) {
		return backend.RunScenario(ctx, body)
	})
	register[struct{}, ActorsResponse](api, http.MethodGet, "/api/actors", "listActors", "List reusable workspace actors", func(ctx context.Context, _ *struct{}) (ActorsResponse, error) {
		items, err := backend.ListActors(ctx)
		return ActorsResponse{Items: items}, err
	})
	registerBody[ScenarioRunRequest, workflow.WorkflowResult](api, http.MethodPost, "/api/workflow-runs", "runWorkflow", "Run a single-event or timeline scenario", func(ctx context.Context, body ScenarioRunRequest) (workflow.WorkflowResult, error) {
		return backend.RunWorkflow(ctx, body)
	})
	register[struct{}, SuitesResponse](api, http.MethodGet, "/api/suites", "listSuites", "List built-in and custom suites", func(ctx context.Context, _ *struct{}) (SuitesResponse, error) {
		items, err := backend.ListSuites(ctx)
		return SuitesResponse{Items: items}, err
	})
	register[SuiteInput, SuiteDetail](api, http.MethodGet, "/api/suite", "getSuite", "Get a suite", func(ctx context.Context, input *SuiteInput) (SuiteDetail, error) {
		return backend.GetSuite(ctx, input.ID)
	})
	registerBody[SuiteRunRequest, suite.SuiteResult](api, http.MethodPost, "/api/suite-runs", "runSuite", "Run a suite and enforce its thresholds", func(ctx context.Context, body SuiteRunRequest) (suite.SuiteResult, error) {
		return backend.RunSuite(ctx, body)
	})
	register[ActivityInput, ActivityResponse](api, http.MethodGet, "/api/runs", "listRuns", "List chronological delivery activity", func(ctx context.Context, input *ActivityInput) (ActivityResponse, error) {
		items, err := backend.ListActivity(ctx, input.Limit, input.Offset)
		return ActivityResponse{Items: items}, err
	})
	register[AttemptInput, DeliveryAttemptDetail](api, http.MethodGet, "/api/delivery-attempts/{id}", "getDeliveryAttempt", "Inspect a delivery attempt", func(ctx context.Context, input *AttemptInput) (DeliveryAttemptDetail, error) {
		return backend.GetAttempt(ctx, input.ID)
	})
	registerBodyWithPath[ReplayRequest, DeliveryResult](api, http.MethodPost, "/api/delivery-attempts/{id}/replay", "replayDeliveryAttempt", "Replay a delivery attempt", func(ctx context.Context, id string, body ReplayRequest) (DeliveryResult, error) {
		return backend.ReplayAttempt(ctx, id, body.Mode)
	})
	register[RunInput, EmptyResponse](api, http.MethodDelete, "/api/runs/{id}", "deleteRun", "Delete a retained run", func(ctx context.Context, input *RunInput) (EmptyResponse, error) {
		return EmptyResponse{}, backend.DeleteRun(ctx, input.ID)
	})
	register[struct{}, PublicKey](api, http.MethodGet, "/api/keys/public", "getSimulatorPublicKey", "Get the simulator public key", func(ctx context.Context, _ *struct{}) (PublicKey, error) {
		return backend.PublicKey(ctx)
	})
	register[struct{}, KeyInfo](api, http.MethodGet, "/api/keys/info", "getSimulatorKeyInfo", "Get simulator key information", func(ctx context.Context, _ *struct{}) (KeyInfo, error) {
		return backend.KeyInfo(ctx)
	})
	register[struct{}, KeyInfo](api, http.MethodPost, "/api/keys/rotate", "rotateSimulatorKey", "Rotate the simulator key pair", func(ctx context.Context, _ *struct{}) (KeyInfo, error) {
		return backend.RotateKey(ctx)
	})
}

// ValidResponse acknowledges successful validation.
type ValidResponse struct {
	Valid bool `json:"valid"`
}

// CapabilitiesResponse advertises supported API features.
type CapabilitiesResponse struct {
	APIVersion   int      `json:"apiVersion"`
	Capabilities []string `json:"capabilities"`
}

// EventsResponse wraps the event-contract collection.
type EventsResponse struct {
	Items []EventContract `json:"items"`
}

// ScenariosResponse wraps the scenario collection.
type ScenariosResponse struct {
	Items []ScenarioSummary `json:"items"`
}

// ActorsResponse wraps reusable actors by identifier.
type ActorsResponse struct {
	Items map[string]actors.User `json:"items"`
}

// SuitesResponse wraps the suite collection.
type SuitesResponse struct {
	Items []SuiteSummary `json:"items"`
}

// ActivityResponse wraps a page of retained activity.
type ActivityResponse struct {
	Items []history.Activity `json:"items"`
}

// EmptyResponse represents a successful operation with no response fields.
type EmptyResponse struct{}

// EventInput captures an event identifier from route parameters.
type EventInput struct {
	Type    string `path:"type"`
	Version int    `path:"version" minimum:"1"`
}

// ScenarioInput captures a scenario identifier from the query string.
type ScenarioInput struct {
	ID string `query:"id" minLength:"1"`
}

// SuiteInput captures a suite identifier from the query string.
type SuiteInput struct {
	ID string `query:"id" minLength:"1"`
}

// AttemptInput captures a delivery-attempt identifier from the route.
type AttemptInput struct {
	ID string `path:"id" minLength:"26" maxLength:"26"`
}

// RunInput captures a run identifier from the route.
type RunInput struct {
	ID string `path:"id" minLength:"26" maxLength:"26"`
}

// ActivityInput contains pagination controls for retained activity.
type ActivityInput struct {
	Limit  int `query:"limit" minimum:"1" maximum:"100" default:"50"`
	Offset int `query:"offset" minimum:"0" default:"0"`
}

func register[I any, O any](api huma.API, method, path, operationID, summary string, handler func(context.Context, *I) (O, error)) {
	huma.Register(api, huma.Operation{OperationID: operationID, Method: method, Path: path, Summary: summary}, func(ctx context.Context, input *I) (*struct{ Body O }, error) {
		body, err := handler(ctx, input)
		if err != nil {
			type statusCoder interface{ HTTPStatus() int }
			if problem, ok := err.(statusCoder); ok {
				return nil, huma.NewError(problem.HTTPStatus(), err.Error(), err)
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body O }{Body: body}, nil
	})
}

func registerBody[B any, O any](api huma.API, method, path, operationID, summary string, handler func(context.Context, B) (O, error)) {
	type input struct{ Body B }
	register[input, O](api, method, path, operationID, summary, func(ctx context.Context, value *input) (O, error) {
		return handler(ctx, value.Body)
	})
}

func registerBodyWithPath[B any, O any](api huma.API, method, path, operationID, summary string, handler func(context.Context, string, B) (O, error)) {
	type input struct {
		ID   string `path:"id"`
		Body B
	}
	register[input, O](api, method, path, operationID, summary, func(ctx context.Context, value *input) (O, error) {
		return handler(ctx, value.ID, value.Body)
	})
}
