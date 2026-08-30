package scenario

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/events"
)

// FormatVersion is the only scenario format understood by this release.
const FormatVersion = 1

// Scenario describes either one webhook request or a timeline of events and waits.
type Scenario struct {
	Version     int               `yaml:"version" json:"version"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Metadata    Metadata          `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Actors      map[string]string `yaml:"actors,omitempty" json:"actors,omitempty"`
	Request     Request           `yaml:"request,omitempty" json:"request,omitempty"`
	Defaults    TimelineDefaults  `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Steps       []Step            `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// TimelineDefaults supplies delivery expectations inherited by timeline steps.
type TimelineDefaults struct {
	Destination string      `yaml:"destination,omitempty" json:"destination,omitempty"`
	Expect      Expectation `yaml:"expect,omitempty" json:"expect,omitempty"`
}

// Step describes either a timed wait or one repeatable event delivery.
type Step struct {
	Event    string            `yaml:"event,omitempty" json:"event,omitempty"`
	Version  int               `yaml:"version,omitempty" json:"version,omitempty"`
	Payload  map[string]any    `yaml:"payload,omitempty" json:"payload,omitempty"`
	Omit     []string          `yaml:"omit,omitempty" json:"omit,omitempty"`
	Actors   map[string]string `yaml:"actors,omitempty" json:"actors,omitempty"`
	Repeat   int               `yaml:"repeat,omitempty" json:"repeat,omitempty"`
	Interval string            `yaml:"interval,omitempty" json:"interval,omitempty"`
	Wait     string            `yaml:"wait,omitempty" json:"wait,omitempty"`
	Delivery Delivery          `yaml:"delivery,omitempty" json:"delivery,omitempty"`
}

// Metadata records the provenance of a copied scenario.
type Metadata struct {
	Source        string `yaml:"source,omitempty" json:"source,omitempty"`
	SourceVersion int    `yaml:"sourceVersion,omitempty" json:"sourceVersion,omitempty"`
	CreatedWith   string `yaml:"createdWith,omitempty" json:"createdWith,omitempty"`
}

// Request describes the event and delivery behavior of a single scenario.
type Request struct {
	Event    Event          `yaml:"event" json:"event"`
	Payload  map[string]any `yaml:"payload" json:"payload"`
	Omit     []string       `yaml:"omit,omitempty" json:"omit,omitempty"`
	Delivery Delivery       `yaml:"delivery" json:"delivery"`
}

// Event identifies a versioned event contract.
type Event struct {
	Type    string `yaml:"type" json:"type"`
	Version int    `yaml:"version" json:"version"`
}

// Delivery configures destination, fault injection, retries, and expectations.
type Delivery struct {
	Mode           string      `yaml:"mode" json:"mode"`
	Destination    string      `yaml:"destination" json:"destination"`
	SubscriptionID string      `yaml:"subscriptionId,omitempty" json:"subscriptionId,omitempty"`
	Failure        string      `yaml:"failure,omitempty" json:"failure,omitempty"`
	Attempts       int         `yaml:"attempts,omitempty" json:"attempts,omitempty"`
	SameMessageID  bool        `yaml:"sameMessageId,omitempty" json:"sameMessageId,omitempty"`
	Expect         Expectation `yaml:"expect" json:"expect"`
}

// Expectation lists the HTTP response statuses accepted by a delivery.
type Expectation struct {
	Statuses []int `yaml:"statuses" json:"statuses"`
}

// Validate reports all detectable structural, contract, actor, and delivery errors.
func (value Scenario) Validate(registry *events.Registry, configuration config.Config, actorRegistries ...*actors.Registry) error {
	var problems []error
	if value.Version != FormatVersion {
		problems = append(problems, fmt.Errorf("unsupported scenario version %d", value.Version))
	}
	if value.Name == "" {
		problems = append(problems, errors.New("scenario name is required"))
	}
	if len(value.Steps) > 0 {
		if value.Request.Event.Type != "" {
			problems = append(problems, errors.New("scenario cannot contain both request and steps"))
		}
		if err := value.validateTimeline(registry, configuration, actorRegistryFrom(actorRegistries)); err != nil {
			problems = append(problems, err)
		}
		return errors.Join(problems...)
	}
	if value.Request.Event.Type == "" {
		problems = append(problems, errors.New("scenario requires request or steps"))
		return errors.Join(problems...)
	}
	definition, err := registry.Get(value.Request.Event.Type, value.Request.Event.Version)
	if err != nil {
		problems = append(problems, err)
		return errors.Join(problems...)
	}
	if value.Request.Delivery.Mode != "" && value.Request.Delivery.Mode != "direct" {
		problems = append(problems, errors.New("scenario delivery mode must be direct"))
	}
	if err := validateDeliveryControls(value.Request.Delivery); err != nil {
		problems = append(problems, err)
	}
	if _, err := configuration.Destination(value.Request.Delivery.Destination); err != nil {
		problems = append(problems, err)
	}
	for _, status := range value.Request.Delivery.Expect.Statuses {
		if status < 100 || status > 599 {
			problems = append(problems, fmt.Errorf("invalid expected HTTP status %d", status))
		}
	}
	if err := ValidateTemplates(value.Request.Payload); err != nil {
		problems = append(problems, err)
	}
	actorRegistry := actorRegistryFrom(actorRegistries)
	payload, err := value.Compose(definition, configuration.Defaults, actorRegistry)
	if err != nil {
		problems = append(problems, err)
		return errors.Join(problems...)
	}
	resolved := events.ResolveDynamic(payload, func() string { return "01ARZ3NDEKTSV4RRFFQ69G5FAV" }, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := registry.Validate(value.Request.Event.Type, value.Request.Event.Version, resolved); err != nil {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

func validateDeliveryControls(delivery Delivery) error {
	if delivery.Attempts < 0 {
		return errors.New("scenario delivery attempts must be positive")
	}
	switch delivery.Failure {
	case "", "invalid-signature", "missing-signature", "stale-timestamp", "modified-body", "malformed-json":
		return nil
	default:
		return fmt.Errorf("unsupported delivery failure %q", delivery.Failure)
	}
}

// Kind reports whether the scenario is single-request or timeline based.
func (value Scenario) Kind() string {
	if len(value.Steps) > 0 {
		return "timeline"
	}
	return "single"
}

func (value Scenario) validateTimeline(registry *events.Registry, configuration config.Config, actorRegistry *actors.Registry) error {
	var problems []error
	if _, err := configuration.Destination(value.Defaults.Destination); err != nil {
		problems = append(problems, err)
	}
	for index, step := range value.Steps {
		if step.Wait != "" {
			if step.Event != "" {
				problems = append(problems, fmt.Errorf("step %d cannot contain both wait and event", index+1))
			}
			if duration, err := time.ParseDuration(step.Wait); err != nil || duration < 0 {
				problems = append(problems, fmt.Errorf("step %d has invalid wait %q", index+1, step.Wait))
			}
			continue
		}
		if step.Event == "" {
			problems = append(problems, fmt.Errorf("step %d requires event or wait", index+1))
			continue
		}
		version := step.Version
		if version == 0 {
			version = 1
		}
		definition, err := registry.Get(step.Event, version)
		if err != nil {
			problems = append(problems, fmt.Errorf("step %d: %w", index+1, err))
			continue
		}
		if step.Repeat < 0 {
			problems = append(problems, fmt.Errorf("step %d repeat must be positive", index+1))
		}
		if step.Interval != "" {
			if duration, err := time.ParseDuration(step.Interval); err != nil || duration < 0 {
				problems = append(problems, fmt.Errorf("step %d has invalid interval %q", index+1, step.Interval))
			}
		}
		if err := ValidateTemplates(step.Payload); err != nil {
			problems = append(problems, fmt.Errorf("step %d: %w", index+1, err))
		}
		bindings := mergeBindings(value.Actors, step.Actors)
		payload, err := events.ComposeWithActors(definition, configuration.Defaults, actorRegistry, bindings, step.Payload)
		if err != nil {
			problems = append(problems, fmt.Errorf("step %d: %w", index+1, err))
			continue
		}
		document := map[string]any{"payload": payload}
		for _, pointer := range step.Omit {
			if err := events.UnsetPointer(document, "/payload"+pointer); err != nil {
				problems = append(problems, fmt.Errorf("step %d omit %q: %w", index+1, pointer, err))
			}
		}
		resolved := events.ResolveDynamic(payload, func() string { return "01ARZ3NDEKTSV4RRFFQ69G5FAV" }, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		if err := registry.Validate(step.Event, version, resolved); err != nil {
			problems = append(problems, fmt.Errorf("step %d: %w", index+1, err))
		}
		destination := step.Delivery.Destination
		if destination == "" {
			destination = value.Defaults.Destination
		}
		if _, err := configuration.Destination(destination); err != nil {
			problems = append(problems, fmt.Errorf("step %d: %w", index+1, err))
		}
		if err := validateDeliveryControls(step.Delivery); err != nil {
			problems = append(problems, fmt.Errorf("step %d: %w", index+1, err))
		}
		for _, status := range append(value.Defaults.Expect.Statuses, step.Delivery.Expect.Statuses...) {
			if status < 100 || status > 599 {
				problems = append(problems, fmt.Errorf("step %d has invalid expected HTTP status %d", index+1, status))
			}
		}
	}
	return errors.Join(problems...)
}

func actorRegistryFrom(values []*actors.Registry) *actors.Registry {
	if len(values) > 0 && values[0] != nil {
		return values[0]
	}
	return actors.Empty()
}

func mergeBindings(base, override map[string]string) map[string]string {
	result := make(map[string]string, len(base)+len(override))
	for role, id := range base {
		result[role] = id
	}
	for role, id := range override {
		result[role] = id
	}
	return result
}

// StepActors overlays a timeline step's actor bindings on the scenario defaults.
func (value Scenario) StepActors(step Step) map[string]string {
	return mergeBindings(value.Actors, step.Actors)
}

// Compose builds a single-request payload and applies its omit pointers.
func (value Scenario) Compose(definition events.Definition, defaults config.Defaults, actorRegistries ...*actors.Registry) (map[string]any, error) {
	actorRegistry := actorRegistryFrom(actorRegistries)
	payload, err := events.ComposeWithActors(definition, defaults, actorRegistry, value.Actors, value.Request.Payload)
	if err != nil {
		return nil, err
	}
	document := map[string]any{"payload": payload}
	seen := map[string]struct{}{}
	for _, pointer := range value.Request.Omit {
		if _, duplicate := seen[pointer]; duplicate {
			return nil, fmt.Errorf("duplicate omit pointer %q", pointer)
		}
		seen[pointer] = struct{}{}
		if err := events.UnsetPointer(document, "/payload"+pointer); err != nil {
			return nil, fmt.Errorf("apply omit %q: %w", pointer, err)
		}
	}
	return payload, nil
}

// ValidateTemplates recursively rejects template expressions the simulator cannot resolve.
func ValidateTemplates(value any) error {
	switch typed := value.(type) {
	case map[string]any:
		for _, child := range typed {
			if err := ValidateTemplates(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := ValidateTemplates(child); err != nil {
				return err
			}
		}
	case string:
		if strings.Contains(typed, "{{") && typed != "{{ ulid() }}" && typed != "{{ now() }}" {
			return fmt.Errorf("unsupported template expression %q", typed)
		}
	}
	return nil
}
