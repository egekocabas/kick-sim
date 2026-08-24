package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/egekocabas/kick-sim/assets"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	ChatMessageSentType    = "chat.message.sent"
	ChatMessageSentVersion = 1
	ChannelFollowedType    = "channel.followed"
	LivestreamStatusType   = "livestream.status.updated"
	ModerationBannedType   = "moderation.banned"
)

type ActorRole struct {
	DefaultSource string `json:"defaultSource,omitempty"`
}

type Definition struct {
	Type        string               `json:"type"`
	Version     int                  `json:"version"`
	Description string               `json:"description"`
	Schema      json.RawMessage      `json:"schema,omitempty"`
	Defaults    map[string]any       `json:"-"`
	ActorRoles  map[string]ActorRole `json:"actorRoles,omitempty"`
	validator   *jsonschema.Schema
}

type Registry struct {
	definitions map[string]Definition
}

func NewRegistry() (*Registry, error) {
	specifications := []struct {
		typeName, description string
		version               int
		roles                 map[string]ActorRole
	}{
		{ChatMessageSentType, "Fired when a message has been sent in a stream's chat.", 1, map[string]ActorRole{"broadcaster": {DefaultSource: "broadcaster"}, "sender": {DefaultSource: "sender"}}},
		{ChannelFollowedType, "Fired when a user follows a channel.", 1, map[string]ActorRole{"broadcaster": {DefaultSource: "broadcaster"}, "follower": {DefaultSource: "sender"}}},
		{LivestreamStatusType, "Fired when a livestream starts or ends.", 1, map[string]ActorRole{"broadcaster": {DefaultSource: "broadcaster"}}},
		{ModerationBannedType, "Fired when a user is banned from a channel.", 1, map[string]ActorRole{"broadcaster": {DefaultSource: "broadcaster"}, "moderator": {DefaultSource: "sender"}, "banned_user": {DefaultSource: "sender"}}},
	}
	definitions := make(map[string]Definition, len(specifications))
	for _, specification := range specifications {
		base := fmt.Sprintf("events/%s.v%d", specification.typeName, specification.version)
		schemaData, err := assets.Files.ReadFile(base + ".schema.json")
		if err != nil {
			return nil, fmt.Errorf("read event schema: %w", err)
		}
		defaultsData, err := assets.Files.ReadFile(base + ".defaults.json")
		if err != nil {
			return nil, fmt.Errorf("read event defaults: %w", err)
		}
		var defaults map[string]any
		if err := json.Unmarshal(defaultsData, &defaults); err != nil {
			return nil, fmt.Errorf("parse event defaults: %w", err)
		}
		var schemaDocument any
		if err := json.Unmarshal(schemaData, &schemaDocument); err != nil {
			return nil, fmt.Errorf("parse event schema: %w", err)
		}
		compiler := jsonschema.NewCompiler()
		resource := "kick-sim://" + base + ".schema.json"
		if err := compiler.AddResource(resource, schemaDocument); err != nil {
			return nil, fmt.Errorf("register event schema: %w", err)
		}
		validator, err := compiler.Compile(resource)
		if err != nil {
			return nil, fmt.Errorf("compile event schema: %w", err)
		}
		definition := Definition{Type: specification.typeName, Version: specification.version, Description: specification.description, Schema: append(json.RawMessage(nil), schemaData...), Defaults: defaults, ActorRoles: specification.roles, validator: validator}
		definitions[key(definition.Type, definition.Version)] = definition
	}
	return &Registry{definitions: definitions}, nil
}

func (registry *Registry) List() []Definition {
	definitions := make([]Definition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		definition.Defaults = nil
		definition.validator = nil
		definition.ActorRoles = cloneRoles(definition.ActorRoles)
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].Type == definitions[j].Type {
			return definitions[i].Version < definitions[j].Version
		}
		return definitions[i].Type < definitions[j].Type
	})
	return definitions
}

func (registry *Registry) Get(eventType string, version int) (Definition, error) {
	definition, ok := registry.definitions[key(eventType, version)]
	if !ok {
		return Definition{}, fmt.Errorf("unsupported event %s@%d", eventType, version)
	}
	definition.Defaults = DeepCopyMap(definition.Defaults)
	definition.Schema = append(json.RawMessage(nil), definition.Schema...)
	definition.ActorRoles = cloneRoles(definition.ActorRoles)
	return definition, nil
}

func cloneRoles(value map[string]ActorRole) map[string]ActorRole {
	result := make(map[string]ActorRole, len(value))
	for role, definition := range value {
		result[role] = definition
	}
	return result
}

func (registry *Registry) Validate(eventType string, version int, payload any) error {
	definition, err := registry.Get(eventType, version)
	if err != nil {
		return err
	}
	if err := definition.validator.Validate(payload); err != nil {
		return fmt.Errorf("payload does not match %s@%d: %w", eventType, version, err)
	}
	return nil
}

func (registry *Registry) ValidateJSON(eventType string, version int, raw []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse event payload: %w", err)
	}
	if payload == nil {
		return nil, errors.New("event payload must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("event payload must contain exactly one JSON value")
	}
	if err := registry.Validate(eventType, version, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func key(eventType string, version int) string {
	return fmt.Sprintf("%s@%d", eventType, version)
}
