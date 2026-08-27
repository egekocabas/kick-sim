// Package app coordinates event generation, signing, delivery, and retained history.
package app

import (
	"time"

	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/oklog/ulid/v2"
)

const maxRequestBody = 1 << 20

type dependencies struct {
	now   func() time.Time
	newID func() string
}

// Service owns the immutable dependencies used by simulator operations.
type Service struct {
	workspaceRoot string
	configuration config.Config
	events        *events.Registry
	actors        *actors.Registry
	dependencies  dependencies
}

type PayloadOptions struct {
	EventType             string
	EventVersion          int
	Scenario              map[string]any
	Actors                map[string]string
	Omit                  []string
	StringValues          map[string]string
	JSONValues            map[string]any
	Unset                 []string
	SemanticValues        map[string]any
	ScenarioDefinitionID  string
	ScenarioSourceVersion int
}

type Generated struct {
	RunID                 string            `json:"runId"`
	GeneratedEventID      string            `json:"generatedEventId"`
	ScenarioDefinitionID  string            `json:"scenarioDefinitionId,omitempty"`
	ScenarioSourceVersion int               `json:"scenarioSourceVersion,omitempty"`
	EventType             string            `json:"eventType"`
	EventVersion          int               `json:"eventVersion"`
	BroadcasterUserID     int64             `json:"broadcasterUserId"`
	LogicalSubscription   string            `json:"logicalSubscription"`
	MessageID             string            `json:"messageId"`
	SubscriptionID        string            `json:"subscriptionId"`
	MessageTimestamp      string            `json:"messageTimestamp"`
	Headers               map[string]string `json:"headers"`
	Payload               map[string]any    `json:"payload"`
	RawBody               string            `json:"rawBody"`
	CreatedAt             time.Time         `json:"createdAt"`
	body                  []byte
}

type RunResult struct {
	Generated
	AttemptID          string              `json:"attemptId"`
	ReplayOfID         string              `json:"replayOfId,omitempty"`
	ReplayMode         string              `json:"replayMode,omitempty"`
	Destination        string              `json:"destination"`
	URL                string              `json:"url"`
	Method             string              `json:"method"`
	Status             int                 `json:"status"`
	Duration           time.Duration       `json:"-"`
	DurationMS         float64             `json:"durationMs"`
	TransportStartedAt time.Time           `json:"transportStartedAt"`
	Outcome            string              `json:"outcome"`
	ResponseHeaders    map[string][]string `json:"responseHeaders,omitempty"`
	ResponseBody       string              `json:"responseBody,omitempty"`
	ResponseTruncated  bool                `json:"responseTruncated"`
	Error              string              `json:"error,omitempty"`
}

func Open(workspaceRoot string) (*Service, error) {
	configuration, err := config.Load(workspace.PathsFor(workspaceRoot).Config)
	if err != nil {
		return nil, err
	}
	registry, err := events.NewRegistry()
	if err != nil {
		return nil, err
	}
	actorRegistry, err := actors.Load(workspace.PathsFor(workspaceRoot).Users)
	if err != nil {
		return nil, err
	}
	return newService(workspaceRoot, configuration, registry, actorRegistry, dependencies{
		now:   time.Now,
		newID: func() string { return ulid.Make().String() },
	}), nil
}

func newService(workspaceRoot string, configuration config.Config, registry *events.Registry, actorRegistry *actors.Registry, deps dependencies) *Service {
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.newID == nil {
		deps.newID = func() string { return ulid.Make().String() }
	}
	return &Service{
		workspaceRoot: workspaceRoot,
		configuration: config.Clone(configuration),
		events:        registry,
		actors:        actorRegistry,
		dependencies:  deps,
	}
}

func (service *Service) WorkspaceRoot() string { return service.workspaceRoot }

func (service *Service) Configuration() config.Config { return config.Clone(service.configuration) }

func (service *Service) EventRegistry() *events.Registry { return service.events }

func (service *Service) ActorRegistry() *actors.Registry { return service.actors }

func (service *Service) Now() time.Time { return service.dependencies.now() }

func (service *Service) newID() string { return service.dependencies.newID() }
