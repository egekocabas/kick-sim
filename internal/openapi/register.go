package openapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egekocabas/kick-sim/internal/history"
)

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

type ValidResponse struct {
	Valid bool `json:"valid"`
}
type CapabilitiesResponse struct {
	APIVersion   int      `json:"apiVersion"`
	Capabilities []string `json:"capabilities"`
}
type EventsResponse struct {
	Items []EventContract `json:"items"`
}
type ScenariosResponse struct {
	Items []ScenarioSummary `json:"items"`
}
type ActivityResponse struct {
	Items []history.Activity `json:"items"`
}
type EmptyResponse struct{}

type EventInput struct {
	Type    string `path:"type"`
	Version int    `path:"version" minimum:"1"`
}
type ScenarioInput struct {
	ID string `query:"id" minLength:"1"`
}
type AttemptInput struct {
	ID string `path:"id" minLength:"26" maxLength:"26"`
}
type RunInput struct {
	ID string `path:"id" minLength:"26" maxLength:"26"`
}
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
