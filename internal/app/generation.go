package app

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/workspace"
	"github.com/oklog/ulid/v2"
)

// GeneratePayload composes, resolves, and validates an event payload without
// signing it or creating delivery metadata.
func (service *Service) GeneratePayload(options PayloadOptions) (map[string]any, error) {
	return service.generatePayloadAt(options, service.Now().UTC())
}

func (service *Service) generatePayloadAt(options PayloadOptions, now time.Time) (map[string]any, error) {
	definition, err := service.events.Get(options.EventType, options.EventVersion)
	if err != nil {
		return nil, err
	}

	// Payload precedence is intentional: contract defaults, workspace actors,
	// bound actors, scenario values, explicit overrides, then dynamic values.
	payload, err := events.ComposeWithActors(definition, service.configuration.Defaults, service.actors, options.Actors, options.Scenario)
	if err != nil {
		return nil, err
	}
	document := map[string]any{"payload": payload}
	seen := map[string]struct{}{}
	apply := func(pointer string, value any) error {
		if _, duplicate := seen[pointer]; duplicate {
			return fmt.Errorf("multiple overrides target %s", pointer)
		}
		seen[pointer] = struct{}{}
		return events.SetPointer(document, pointer, value)
	}
	for _, pointer := range sortedKeys(options.SemanticValues) {
		if err := apply(pointer, options.SemanticValues[pointer]); err != nil {
			return nil, err
		}
	}
	for _, pointer := range sortedKeys(options.StringValues) {
		if err := apply(pointer, options.StringValues[pointer]); err != nil {
			return nil, err
		}
	}
	for _, pointer := range sortedKeys(options.JSONValues) {
		if err := apply(pointer, options.JSONValues[pointer]); err != nil {
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
	if err := events.ValidateActorOwned(definition, service.actors, options.Actors, payload); err != nil {
		return nil, err
	}

	resolved, ok := events.ResolveDynamic(payload, service.newID, now).(map[string]any)
	if !ok {
		return nil, errors.New("resolved payload is not an object")
	}
	if err := service.events.Validate(options.EventType, options.EventVersion, resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}

// Generate composes and signs an event using the service clock.
func (service *Service) Generate(options PayloadOptions, subscriptionOverride string) (Generated, error) {
	return service.GenerateAt(options, subscriptionOverride, service.Now().UTC())
}

// GenerateAt composes and signs an event at a caller-supplied logical time.
// The logical time drives both dynamic payload values and signature metadata.
func (service *Service) GenerateAt(options PayloadOptions, subscriptionOverride string, logicalTime time.Time) (Generated, error) {
	now := logicalTime.UTC()
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
	runID := service.newID()
	generatedEventID := service.newID()
	subscriptionID := subscriptionOverride
	if subscriptionID != "" {
		if _, err := ulid.ParseStrict(subscriptionID); err != nil {
			return Generated{}, fmt.Errorf("invalid subscription ID: %w", err)
		}
	} else {
		subscriptionID = service.newID()
	}

	privateKey, err := service.privateKey()
	if err != nil {
		return Generated{}, err
	}
	messageID := service.newID()
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
		RunID: runID, GeneratedEventID: generatedEventID,
		ScenarioDefinitionID: options.ScenarioDefinitionID, ScenarioSourceVersion: options.ScenarioSourceVersion,
		EventType: options.EventType, EventVersion: options.EventVersion,
		BroadcasterUserID: broadcasterUserID, LogicalSubscription: logicalSubscription,
		MessageID: messageID, SubscriptionID: subscriptionID, MessageTimestamp: timestamp,
		Headers: headers, Payload: payload, RawBody: string(body), CreatedAt: now, body: body,
	}, nil
}

func (service *Service) privateKey() (*rsa.PrivateKey, error) {
	path := config.ResolvePath(service.workspaceRoot, service.configuration.Signing.PrivateKey)
	privateKey, err := workspace.ReadPrivateKey(path)
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

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
