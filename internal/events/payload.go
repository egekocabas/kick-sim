package events

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/egekocabas/kick-sim/internal/config"
)

func Compose(definition Definition, workspaceDefaults config.Defaults, scenarioPayload map[string]any) map[string]any {
	payload := DeepCopyMap(definition.Defaults)
	mergeUser(payload, "broadcaster", workspaceDefaults.Broadcaster)
	mergeUser(payload, "sender", workspaceDefaults.Sender)
	DeepMerge(payload, scenarioPayload)
	return payload
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
