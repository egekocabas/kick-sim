package events

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/egekocabas/kick-sim/internal/actors"
	"github.com/egekocabas/kick-sim/internal/config"
)

var actorOwnedFields = []string{"user_id", "username", "channel_slug", "is_verified", "profile_picture"}

func ComposeWithActors(definition Definition, workspaceDefaults config.Defaults, registry *actors.Registry, bindings map[string]string, scenarioPayload map[string]any) (map[string]any, error) {
	payload := DeepCopyMap(definition.Defaults)
	for role, roleDefinition := range definition.ActorRoles {
		user := workspaceDefaults.Sender
		if roleDefinition.DefaultSource == "broadcaster" {
			user = workspaceDefaults.Broadcaster
		}
		mergeUser(payload, role, user)
	}
	for role, actorID := range bindings {
		if _, supported := definition.ActorRoles[role]; !supported {
			return nil, fmt.Errorf("event %s@%d does not support actor role %q", definition.Type, definition.Version, role)
		}
		actor, err := registry.Get(actorID)
		if err != nil {
			return nil, fmt.Errorf("resolve %s actor: %w", role, err)
		}
		if supplied, ok := toStringMap(scenarioPayload[role]); ok {
			for _, field := range actorOwnedFields {
				if _, exists := supplied[field]; exists {
					return nil, fmt.Errorf("payload field %s.%s is actor-owned; change actor %q or its registry data", role, field, actorID)
				}
			}
		}
		mergeActor(payload, role, actor)
	}
	DeepMerge(payload, scenarioPayload)
	return payload, nil
}

func ValidateActorOwned(definition Definition, registry *actors.Registry, bindings map[string]string, payload map[string]any) error {
	for role, actorID := range bindings {
		if _, supported := definition.ActorRoles[role]; !supported {
			return fmt.Errorf("event %s@%d does not support actor role %q", definition.Type, definition.Version, role)
		}
		actor, err := registry.Get(actorID)
		if err != nil {
			return fmt.Errorf("resolve %s actor: %w", role, err)
		}
		expectedDocument := map[string]any{}
		mergeActor(expectedDocument, role, actor)
		expected, _ := toStringMap(expectedDocument[role])
		actual, ok := toStringMap(payload[role])
		if !ok {
			return fmt.Errorf("payload role %s is actor-owned and must remain an object", role)
		}
		for _, field := range actorOwnedFields {
			if !Equal(actual[field], expected[field]) {
				return fmt.Errorf("payload field %s.%s is actor-owned; change actor %q or its registry data", role, field, actorID)
			}
		}
	}
	return nil
}

func ResolveDynamic(value any, newID func() string, now time.Time) any {
	switch typed := value.(type) {
	case map[string]any:
		resolved := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := typed[key]
			resolved[key] = ResolveDynamic(child, newID, now)
		}
		return resolved
	case []any:
		resolved := make([]any, len(typed))
		for index, child := range typed {
			resolved[index] = ResolveDynamic(child, newID, now)
		}
		return resolved
	case string:
		switch typed {
		case "{{ ulid() }}":
			return newID()
		case "{{ now() }}":
			return now.UTC().Format(time.RFC3339Nano)
		default:
			return typed
		}
	default:
		return value
	}
}

func DeepMerge(destination, source map[string]any) {
	for key, sourceValue := range source {
		sourceObject, sourceIsObject := toStringMap(sourceValue)
		destinationObject, destinationIsObject := toStringMap(destination[key])
		if sourceIsObject && destinationIsObject {
			DeepMerge(destinationObject, sourceObject)
			destination[key] = destinationObject
			continue
		}
		destination[key] = deepCopy(sourceValue)
	}
}

func DeepCopyMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	copy := make(map[string]any, len(value))
	for key, child := range value {
		copy[key] = deepCopy(child)
	}
	return copy
}

func Equal(left, right any) bool {
	return reflect.DeepEqual(left, right)
}

func Marshal(payload map[string]any, pretty bool) ([]byte, error) {
	var (
		data []byte
		err  error
	)
	if pretty {
		data, err = json.MarshalIndent(payload, "", "  ")
	} else {
		data, err = json.Marshal(payload)
	}
	if err != nil {
		return nil, fmt.Errorf("serialize event payload: %w", err)
	}
	return data, nil
}

func mergeUser(payload map[string]any, role string, user config.User) {
	profilePicture := user.ProfilePicture
	if profilePicture == "" {
		profilePicture = fmt.Sprintf("https://example.invalid/kick-sim/%s.png", user.Username)
	}
	DeepMerge(payload, map[string]any{
		role: map[string]any{
			"user_id":         user.UserID,
			"username":        user.Username,
			"channel_slug":    user.ChannelSlug,
			"is_verified":     user.IsVerified,
			"profile_picture": profilePicture,
		},
	})
}

func mergeActor(payload map[string]any, role string, user actors.User) {
	profilePicture := user.ProfilePicture
	if profilePicture == "" {
		profilePicture = fmt.Sprintf("https://example.invalid/kick-sim/%s.png", user.Username)
	}
	DeepMerge(payload, map[string]any{role: map[string]any{
		"user_id": user.UserID, "username": user.Username, "channel_slug": user.ChannelSlug,
		"is_verified": user.IsVerified, "profile_picture": profilePicture,
	}})
}

func deepCopy(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return DeepCopyMap(typed)
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for key, child := range typed {
			converted[fmt.Sprint(key)] = deepCopy(child)
		}
		return converted
	case []any:
		copy := make([]any, len(typed))
		for index, child := range typed {
			copy[index] = deepCopy(child)
		}
		return copy
	default:
		return value
	}
}

func toStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for key, child := range typed {
			converted[fmt.Sprint(key)] = child
		}
		return converted, true
	default:
		return nil, false
	}
}
