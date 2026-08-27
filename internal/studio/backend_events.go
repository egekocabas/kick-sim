package studio

import (
	"context"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/events"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
)

func (backend *Backend) ListEvents(context.Context) ([]kickopenapi.EventContract, error) {
	definitions := backend.service.EventRegistry().List()
	items := make([]kickopenapi.EventContract, 0, len(definitions))
	for _, listed := range definitions {
		definition, err := backend.service.EventRegistry().Get(listed.Type, listed.Version)
		if err != nil {
			return nil, err
		}
		items = append(items, eventContract(definition))
	}
	return items, nil
}

func (backend *Backend) GetEvent(_ context.Context, eventType string, eventVersion int) (kickopenapi.EventContract, error) {
	definition, err := backend.service.EventRegistry().Get(eventType, eventVersion)
	if err != nil {
		return kickopenapi.EventContract{}, err
	}
	return eventContract(definition), nil
}

func (backend *Backend) ValidatePayload(_ context.Context, request kickopenapi.EventPayloadRequest) error {
	return backend.service.EventRegistry().Validate(request.EventType, request.EventVersion, request.Payload)
}

func (backend *Backend) GenerateEvent(_ context.Context, request kickopenapi.EventDeliveryRequest) (kickopenapi.GeneratedEvent, error) {
	options, err := backend.deliveryOptions(request.EventPayloadRequest)
	if err != nil {
		return kickopenapi.GeneratedEvent{}, err
	}
	generated, err := backend.service.Generate(options, request.SubscriptionID)
	if err != nil {
		return kickopenapi.GeneratedEvent{}, err
	}
	destinationURL, err := backend.destinationURL(request.Destination, request.DestinationURL)
	if err != nil {
		return kickopenapi.GeneratedEvent{}, err
	}
	return generatedDTO(generated, destinationURL), nil
}

func (backend *Backend) TriggerEvent(ctx context.Context, request kickopenapi.EventDeliveryRequest) (kickopenapi.DeliveryResult, error) {
	options, err := backend.deliveryOptions(request.EventPayloadRequest)
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	generated, err := backend.service.Generate(options, request.SubscriptionID)
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	result, err := backend.service.Deliver(ctx, generated, request.Destination, request.DestinationURL, nil)
	return deliveryDTO(result), retainDeliveryResult(result, err)
}

func (backend *Backend) deliveryOptions(request kickopenapi.EventPayloadRequest) (app.PayloadOptions, error) {
	if err := backend.service.EventRegistry().Validate(request.EventType, request.EventVersion, request.Payload); err != nil {
		return app.PayloadOptions{}, err
	}
	payload := events.DeepCopyMap(request.Payload)
	payload["message_id"] = "{{ ulid() }}"
	payload["created_at"] = "{{ now() }}"
	return app.PayloadOptions{EventType: request.EventType, EventVersion: request.EventVersion, Scenario: payload}, nil
}

func (backend *Backend) destinationURL(name, temporary string) (string, error) {
	if temporary != "" {
		return temporary, nil
	}
	destination, err := backend.service.Configuration().Destination(name)
	if err != nil {
		return "", err
	}
	return destination.URL, nil
}
