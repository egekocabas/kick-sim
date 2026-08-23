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
)

type Definition struct {
	Type        string          `json:"type"`
	Version     int             `json:"version"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Defaults    map[string]any  `json:"-"`
	validator   *jsonschema.Schema
}

type Registry struct {
	definitions map[string]Definition
}

func NewRegistry() (*Registry, error) {
	const schemaPath = "events/chat.message.sent.v1.schema.json"
	schemaData, err := assets.Files.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read event schema: %w", err)
	}
	defaultsData, err := assets.Files.ReadFile("events/chat.message.sent.v1.defaults.json")
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
	if err := compiler.AddResource("kick-sim://"+schemaPath, schemaDocument); err != nil {
		return nil, fmt.Errorf("register event schema: %w", err)
	}
	validator, err := compiler.Compile("kick-sim://" + schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile event schema: %w", err)
	}

	definition := Definition{
		Type:        ChatMessageSentType,
		Version:     ChatMessageSentVersion,
		Description: "Fired when a message has been sent in a stream's chat.",
		Schema:      append(json.RawMessage(nil), schemaData...),
		Defaults:    defaults,
		validator:   validator,
	}
	return &Registry{definitions: map[string]Definition{key(definition.Type, definition.Version): definition}}, nil
}

func (registry *Registry) List() []Definition {
	definitions := make([]Definition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		definition.Defaults = nil
		definition.validator = nil
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
	return definition, nil
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
