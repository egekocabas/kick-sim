package openapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/history"
	"github.com/egekocabas/kick-sim/internal/suite"
	"github.com/egekocabas/kick-sim/internal/workflow"
)

// Backend defines the application operations exposed by the Studio HTTP API.
type Backend interface {
	Bootstrap(context.Context) (Bootstrap, error)
	Workspace(context.Context) (Workspace, error)
	ValidateWorkspace(context.Context) error
	ListEvents(context.Context) ([]EventContract, error)
	GetEvent(context.Context, string, int) (EventContract, error)
	ValidatePayload(context.Context, EventPayloadRequest) error
	GenerateEvent(context.Context, EventDeliveryRequest) (GeneratedEvent, error)
	TriggerEvent(context.Context, EventDeliveryRequest) (DeliveryResult, error)
	ListScenarios(context.Context) ([]ScenarioSummary, error)
	GetScenario(context.Context, string) (ScenarioDetail, error)
	SaveScenarioCopy(context.Context, ScenarioCopyRequest) (ScenarioDetail, error)
	SaveScenarioSource(context.Context, ScenarioSourceSaveRequest) (ScenarioDetail, error)
	SaveScenarioSourceCopy(context.Context, ScenarioSourceCopyRequest) (ScenarioDetail, error)
	RunScenario(context.Context, ScenarioRunRequest) (DeliveryResult, error)
	ListActors(context.Context) (map[string]actors.User, error)
	RunWorkflow(context.Context, ScenarioRunRequest) (workflow.WorkflowResult, error)
	ListSuites(context.Context) ([]SuiteSummary, error)
	GetSuite(context.Context, string) (SuiteDetail, error)
	RunSuite(context.Context, SuiteRunRequest) (suite.SuiteResult, error)
	ListActivity(context.Context, int, int) ([]history.Activity, error)
	GetAttempt(context.Context, string) (DeliveryAttemptDetail, error)
	ReplayAttempt(context.Context, string, string) (DeliveryResult, error)
	DeleteRun(context.Context, string) error
	PublicKey(context.Context) (PublicKey, error)
	KeyInfo(context.Context) (KeyInfo, error)
	RotateKey(context.Context) (KeyInfo, error)
}

// Bootstrap contains the initial state required to render Studio.
type Bootstrap struct {
	APIVersion     int                `json:"apiVersion"`
	Capabilities   []string           `json:"capabilities"`
	ProductVersion string             `json:"productVersion"`
	Workspace      Workspace          `json:"workspace"`
	Key            KeyInfo            `json:"key"`
	RecentActivity []history.Activity `json:"recentActivity"`
}

// Workspace describes the active workspace without exposing secret key material.
type Workspace struct {
	Path               string `json:"path"`
	DefaultDestination string `json:"defaultDestination"`
	DestinationURL     string `json:"destinationUrl"`
	HistoryEnabled     bool   `json:"historyEnabled"`
	DatabasePath       string `json:"databasePath"`
}

// EventContract is the public schema and defaults for one event version.
type EventContract struct {
	Type        string          `json:"type"`
	Version     int             `json:"version"`
	Description string          `json:"description"`
	EventSchema json.RawMessage `json:"schema"`
	Defaults    map[string]any  `json:"defaults"`
}

// EventPayloadRequest submits a decoded payload for a versioned event contract.
type EventPayloadRequest struct {
	EventType    string         `json:"eventType" minLength:"1"`
	EventVersion int            `json:"eventVersion" minimum:"1"`
	Payload      map[string]any `json:"payload"`
}

// EventDeliveryRequest adds delivery overrides to an event payload.
type EventDeliveryRequest struct {
	EventPayloadRequest
	Destination    string `json:"destination,omitempty"`
	DestinationURL string `json:"destinationUrl,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
}

// GeneratedEvent is the API representation of a signed event preview.
type GeneratedEvent struct {
	RunID               string            `json:"runId"`
	GeneratedEventID    string            `json:"generatedEventId"`
	EventType           string            `json:"eventType"`
	EventVersion        int               `json:"eventVersion"`
	BroadcasterUserID   int64             `json:"broadcasterUserId"`
	LogicalSubscription string            `json:"logicalSubscription"`
	MessageID           string            `json:"messageId"`
	SubscriptionID      string            `json:"subscriptionId"`
	MessageTimestamp    string            `json:"messageTimestamp"`
	Headers             map[string]string `json:"headers"`
	Payload             map[string]any    `json:"payload"`
	RawBody             string            `json:"rawBody"`
	RawHTTP             string            `json:"rawHttp"`
}

// DeliveryResult combines a generated event with one HTTP delivery outcome.
type DeliveryResult struct {
	GeneratedEvent
	AttemptID          string              `json:"attemptId"`
	ReplayOfID         string              `json:"replayOfId,omitempty"`
	ReplayMode         string              `json:"replayMode,omitempty"`
	Destination        string              `json:"destination"`
	URL                string              `json:"url"`
	Method             string              `json:"method"`
	Status             int                 `json:"status,omitempty"`
	DurationMS         float64             `json:"durationMs"`
	TransportStartedAt time.Time           `json:"transportStartedAt"`
	Outcome            string              `json:"outcome"`
	ResponseHeaders    map[string][]string `json:"responseHeaders,omitempty"`
	ResponseBody       string              `json:"responseBody,omitempty"`
	ResponseTruncated  bool                `json:"responseTruncated"`
	Error              string              `json:"error,omitempty"`
}

// ScenarioSummary is the list representation of a built-in or custom scenario.
type ScenarioSummary struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	BuiltIn          bool     `json:"builtIn"`
	Kind             string   `json:"kind" enum:"single,timeline"`
	EventType        string   `json:"eventType"`
	EventVersion     int      `json:"eventVersion"`
	Revision         string   `json:"revision"`
	SourceVersion    int      `json:"sourceVersion"`
	SourceFormat     string   `json:"sourceFormat" enum:"yaml,json"`
	Valid            bool     `json:"valid"`
	ValidationErrors []string `json:"validationErrors,omitempty"`
}

// SuiteSummary is the list representation of a built-in or custom suite.
type SuiteSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	BuiltIn     bool   `json:"builtIn"`
	Cases       int    `json:"cases"`
	Valid       bool   `json:"valid"`
	Error       string `json:"error,omitempty"`
}

// SuiteDetail adds editable source to a suite summary.
type SuiteDetail struct {
	SuiteSummary
	Source string `json:"source"`
}

// ScenarioDetail adds source, payload, and delivery defaults to a scenario summary.
type ScenarioDetail struct {
	ScenarioSummary
	Source           string         `json:"source"`
	Payload          map[string]any `json:"payload,omitempty"`
	DraftPayload     map[string]any `json:"draftPayload,omitempty"`
	Destination      string         `json:"destination,omitempty"`
	ExpectedStatuses []int          `json:"expectedStatuses,omitempty"`
}

// ScenarioCopyRequest creates a custom scenario working copy.
type ScenarioCopyRequest struct {
	SourceID string         `json:"sourceId" minLength:"1"`
	TargetID string         `json:"targetId" minLength:"1"`
	Payload  map[string]any `json:"payload"`
}

// ScenarioSourceSaveRequest replaces source using an expected revision.
type ScenarioSourceSaveRequest struct {
	ID       string `json:"id" minLength:"1"`
	Revision string `json:"revision" pattern:"^sha256:[a-f0-9]{64}$"`
	Source   string `json:"source" minLength:"1"`
}

// ScenarioSourceCopyRequest saves edited source under a new scenario identifier.
type ScenarioSourceCopyRequest struct {
	SourceID string `json:"sourceId" minLength:"1"`
	TargetID string `json:"targetId" minLength:"1"`
	Source   string `json:"source" minLength:"1"`
}

// ScenarioRunRequest contains runtime overrides for a scenario or workflow run.
type ScenarioRunRequest struct {
	ScenarioID     string         `json:"scenarioId" minLength:"1"`
	Payload        map[string]any `json:"payload,omitempty"`
	Destination    string         `json:"destination,omitempty"`
	DestinationURL string         `json:"destinationUrl,omitempty"`
	SubscriptionID string         `json:"subscriptionId,omitempty"`
}

// SuiteRunRequest contains runtime destination overrides for a suite run.
type SuiteRunRequest struct {
	SuiteID        string `json:"suiteId" minLength:"1"`
	Destination    string `json:"destination,omitempty"`
	DestinationURL string `json:"destinationUrl,omitempty"`
}

// ReplayRequest selects exact or regenerated delivery replay.
type ReplayRequest struct {
	Mode string `json:"mode" enum:"exact,regenerated"`
}

// DeliveryAttemptDetail exposes a retained attempt and its signing input.
type DeliveryAttemptDetail struct {
	Run            history.Run     `json:"run"`
	Event          history.Event   `json:"event"`
	Attempt        history.Attempt `json:"attempt"`
	SignatureInput string          `json:"signatureInput"`
}

// PublicKey contains the simulator public key and its workspace path.
type PublicKey struct {
	Path string `json:"path"`
	PEM  string `json:"pem"`
}

// KeyInfo describes the active key pair without exposing private material.
type KeyInfo struct {
	Path               string    `json:"path"`
	Algorithm          string    `json:"algorithm"`
	Bits               int       `json:"bits"`
	Fingerprint        string    `json:"fingerprint"`
	ModifiedAt         time.Time `json:"modifiedAt"`
	MatchingPrivateKey bool      `json:"matchingPrivateKey"`
}
