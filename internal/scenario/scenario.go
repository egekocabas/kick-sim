package scenario

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/events"
)

const FormatVersion = 1

type Scenario struct {
	Version     int      `yaml:"version" json:"version"`
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Metadata    Metadata `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Request     Request  `yaml:"request" json:"request"`
}

type Metadata struct {
	Source        string `yaml:"source,omitempty" json:"source,omitempty"`
	SourceVersion int    `yaml:"sourceVersion,omitempty" json:"sourceVersion,omitempty"`
	CreatedWith   string `yaml:"createdWith,omitempty" json:"createdWith,omitempty"`
}

type Request struct {
	Event    Event          `yaml:"event" json:"event"`
	Payload  map[string]any `yaml:"payload" json:"payload"`
	Omit     []string       `yaml:"omit,omitempty" json:"omit,omitempty"`
	Delivery Delivery       `yaml:"delivery" json:"delivery"`
}

type Event struct {
	Type    string `yaml:"type" json:"type"`
	Version int    `yaml:"version" json:"version"`
}

type Delivery struct {
	Mode           string      `yaml:"mode" json:"mode"`
	Destination    string      `yaml:"destination" json:"destination"`
	SubscriptionID string      `yaml:"subscriptionId,omitempty" json:"subscriptionId,omitempty"`
	Expect         Expectation `yaml:"expect" json:"expect"`
}

type Expectation struct {
	Statuses []int `yaml:"statuses" json:"statuses"`
}

func (value Scenario) Validate(registry *events.Registry, configuration config.Config) error {
	var problems []error
	if value.Version != FormatVersion {
		problems = append(problems, fmt.Errorf("unsupported scenario version %d", value.Version))
	}
	if value.Name == "" {
		problems = append(problems, errors.New("scenario name is required"))
	}
	definition, err := registry.Get(value.Request.Event.Type, value.Request.Event.Version)
	if err != nil {
		problems = append(problems, err)
		return errors.Join(problems...)
	}
	if value.Request.Delivery.Mode != "" && value.Request.Delivery.Mode != "direct" {
		problems = append(problems, errors.New("scenario delivery mode must be direct"))
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
	payload, err := value.Compose(definition, configuration.Defaults)
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

func (value Scenario) Compose(definition events.Definition, defaults config.Defaults) (map[string]any, error) {
	payload := events.Compose(definition, defaults, value.Request.Payload)
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
