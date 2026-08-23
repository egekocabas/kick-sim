package app

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/oklog/ulid/v2"
)

const maxRequestBody = 1 << 20

type Service struct {
	Workspace string
	Config    config.Config
	Events    *events.Registry
	Now       func() time.Time
	NewID     func() string
}

type PayloadOptions struct {
	EventType      string
	EventVersion   int
	Scenario       map[string]any
	Omit           []string
	StringValues   map[string]string
	JSONValues     map[string]any
	Unset          []string
	SemanticValues map[string]any
}

type Generated struct {
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
	body                []byte
}

type RunResult struct {
	Generated
	Destination  string        `json:"destination"`
	URL          string        `json:"url"`
	Status       int           `json:"status"`
	Duration     time.Duration `json:"-"`
	DurationMS   float64       `json:"durationMs"`
	ResponseBody string        `json:"responseBody,omitempty"`
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
	return &Service{
		Workspace: workspaceRoot,
		Config:    configuration,
		Events:    registry,
		Now:       time.Now,
		NewID:     func() string { return ulid.Make().String() },
	}, nil
}

func (service *Service) GeneratePayload(options PayloadOptions) (map[string]any, error) {
	return service.generatePayloadAt(options, service.Now().UTC())
}

func (service *Service) generatePayloadAt(options PayloadOptions, now time.Time) (map[string]any, error) {
	definition, err := service.Events.Get(options.EventType, options.EventVersion)
	if err != nil {
		return nil, err
	}
	payload := events.Compose(definition, service.Config.Defaults, options.Scenario)
	document := map[string]any{"payload": payload}

	seen := map[string]struct{}{}
	apply := func(pointer string, value any) error {
		if _, duplicate := seen[pointer]; duplicate {
			return fmt.Errorf("multiple overrides target %s", pointer)
		}
		seen[pointer] = struct{}{}
		return events.SetPointer(document, pointer, value)
	}
	for pointer, value := range options.SemanticValues {
		if err := apply(pointer, value); err != nil {
			return nil, err
		}
	}
	for pointer, value := range options.StringValues {
		if err := apply(pointer, value); err != nil {
			return nil, err
		}
	}
	for pointer, value := range options.JSONValues {
		if err := apply(pointer, value); err != nil {
			return nil, err
		}
	}
	for _, pointer := range options.Omit {
		fullPointer := "/payload" + pointer
		if _, duplicate := seen[fullPointer]; duplicate {
			return nil, fmt.Errorf("multiple overrides target %s", fullPointer)
		}
		seen[fullPointer] = struct{}{}
		if err := events.UnsetPointer(document, fullPointer); err != nil {
			return nil, err
		}
	}
	for _, pointer := range options.Unset {
		if _, duplicate := seen[pointer]; duplicate {
			return nil, fmt.Errorf("multiple overrides target %s", pointer)
		}
		seen[pointer] = struct{}{}
		if err := events.UnsetPointer(document, pointer); err != nil {
			return nil, err
		}
	}

	resolved, ok := events.ResolveDynamic(payload, service.NewID, now).(map[string]any)
	if !ok {
		return nil, errors.New("resolved payload is not an object")
	}
	if err := service.Events.Validate(options.EventType, options.EventVersion, resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}

func (service *Service) Generate(options PayloadOptions, subscriptionOverride string) (Generated, error) {
	now := service.Now().UTC()
	payload, err := service.generatePayloadAt(options, now)
	if err != nil {
		return Generated{}, err
	}
	body, err := events.Marshal(payload, false)
	if err != nil {
		return Generated{}, err
	}
	if len(body) > maxRequestBody {
		return Generated{}, fmt.Errorf("generated request body exceeds %d bytes", maxRequestBody)
	}
	broadcasterUserID, err := broadcasterID(payload)
	if err != nil {
		return Generated{}, err
	}
	logicalSubscription := fmt.Sprintf("%d:%s:%d", broadcasterUserID, options.EventType, options.EventVersion)
	subscriptionID := subscriptionOverride
	if subscriptionID != "" {
		if _, err := ulid.ParseStrict(subscriptionID); err != nil {
			return Generated{}, fmt.Errorf("invalid subscription ID: %w", err)
		}
	} else {
		subscriptionID = service.NewID()
	}

	privateKey, err := service.privateKey()
	if err != nil {
		return Generated{}, err
	}
	messageID := service.NewID()
	timestamp := now.Format(time.RFC3339Nano)
	signature, err := signing.Sign(privateKey, messageID, timestamp, body)
	if err != nil {
		return Generated{}, err
	}
	headers := map[string]string{
		delivery.HeaderMessageID:      messageID,
		delivery.HeaderSubscriptionID: subscriptionID,
		delivery.HeaderSignature:      signature,
		delivery.HeaderTimestamp:      timestamp,
		delivery.HeaderEventType:      options.EventType,
		delivery.HeaderEventVersion:   fmt.Sprint(options.EventVersion),
		"Content-Type":                "application/json",
		"Kick-Simulator":              "kick-sim",
	}
	return Generated{
		EventType:           options.EventType,
		EventVersion:        options.EventVersion,
		BroadcasterUserID:   broadcasterUserID,
		LogicalSubscription: logicalSubscription,
		MessageID:           messageID,
		SubscriptionID:      subscriptionID,
		MessageTimestamp:    timestamp,
		Headers:             headers,
		Payload:             payload,
		RawBody:             string(body),
		body:                body,
	}, nil
}

func (service *Service) Deliver(ctx context.Context, generated Generated, destinationName, temporaryURL string, expectedStatuses []int) (RunResult, error) {
	destination, err := service.Config.Destination(destinationName)
	if err != nil {
		return RunResult{}, err
	}
	if temporaryURL != "" {
		destination.URL = temporaryURL
		destinationName = "temporary"
	}
	timeout, err := time.ParseDuration(destination.Timeout)
	if err != nil {
		return RunResult{}, fmt.Errorf("parse destination timeout: %w", err)
	}
	result, err := delivery.Send(ctx, delivery.NewLoopbackClient(timeout), destination.URL, delivery.Metadata{
		MessageID:      generated.MessageID,
		SubscriptionID: generated.SubscriptionID,
		Signature:      generated.Headers[delivery.HeaderSignature],
		Timestamp:      generated.MessageTimestamp,
		EventType:      generated.EventType,
		EventVersion:   fmt.Sprint(generated.EventVersion),
	}, generated.body)
	run := RunResult{
		Generated:    generated,
		Destination:  destinationName,
		URL:          destination.URL,
		Status:       result.StatusCode,
		Duration:     result.Duration,
		DurationMS:   float64(result.Duration) / float64(time.Millisecond),
		ResponseBody: string(result.ResponseBody),
	}
	if err != nil {
		return run, err
	}
	if len(expectedStatuses) == 0 && (result.StatusCode < 200 || result.StatusCode >= 300) {
		return run, fmt.Errorf("webhook receiver returned HTTP %d", result.StatusCode)
	}
	if len(expectedStatuses) > 0 && !containsStatus(expectedStatuses, result.StatusCode) {
		return run, fmt.Errorf("webhook receiver returned HTTP %d; expected one of %v", result.StatusCode, expectedStatuses)
	}
	return run, nil
}

func (service *Service) privateKey() (*rsa.PrivateKey, error) {
	path := config.ResolvePath(service.Workspace, service.Config.Signing.PrivateKey)
	if err := workspace.ValidatePrivateKeyPermissions(path); err != nil {
		return nil, err
	}
	privateKey, err := signing.ReadPrivateKey(path)
	if err != nil {
		return nil, fmt.Errorf("load simulator private key: %w", err)
	}
	return privateKey, nil
}

func broadcasterID(payload map[string]any) (int64, error) {
	broadcaster, ok := payload["broadcaster"].(map[string]any)
	if !ok {
		return 0, errors.New("payload broadcaster must be an object")
	}
	switch value := broadcaster["user_id"].(type) {
	case int:
		return positiveBroadcasterID(int64(value))
	case int64:
		return positiveBroadcasterID(value)
	case uint64:
		if value > math.MaxInt64 {
			return 0, errors.New("broadcaster user_id is too large")
		}
		return positiveBroadcasterID(int64(value))
	case float64:
		if math.Trunc(value) != value || value > math.MaxInt64 {
			return 0, errors.New("payload broadcaster.user_id must be a positive integer")
		}
		return positiveBroadcasterID(int64(value))
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0, fmt.Errorf("payload broadcaster.user_id must be a positive integer: %w", err)
		}
		return positiveBroadcasterID(parsed)
	default:
		return 0, errors.New("payload broadcaster.user_id must be an integer")
	}
}

func positiveBroadcasterID(value int64) (int64, error) {
	if value < 1 {
		return 0, errors.New("payload broadcaster.user_id must be a positive integer")
	}
	return value, nil
}

func containsStatus(statuses []int, actual int) bool {
	for _, status := range statuses {
		if status == actual {
			return true
		}
	}
	return false
}
