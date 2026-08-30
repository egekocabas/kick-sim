package studio

import (
	"context"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/history"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/signing"
)

// ListActivity returns a page of retained delivery summaries.
func (backend *Backend) ListActivity(ctx context.Context, limit, offset int) ([]history.Activity, error) {
	store, err := backend.service.History()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.ListActivity(ctx, limit, offset)
}

// GetAttempt returns one retained delivery and reconstructs its signing input.
func (backend *Backend) GetAttempt(ctx context.Context, id string) (kickopenapi.DeliveryAttemptDetail, error) {
	store, err := backend.service.History()
	if err != nil {
		return kickopenapi.DeliveryAttemptDetail{}, err
	}
	defer store.Close()
	detail, err := store.GetAttempt(ctx, id)
	if err != nil {
		return kickopenapi.DeliveryAttemptDetail{}, err
	}
	input := signing.SignatureInput(
		detail.Event.Headers[delivery.HeaderMessageID], detail.Event.MessageTimestamp, []byte(detail.Event.RawBody),
	)
	return kickopenapi.DeliveryAttemptDetail{
		Run: detail.Run, Event: detail.Event, Attempt: detail.Attempt, SignatureInput: string(input),
	}, nil
}

// ReplayAttempt redelivers a retained attempt in exact or regenerated mode.
func (backend *Backend) ReplayAttempt(ctx context.Context, id, mode string) (kickopenapi.DeliveryResult, error) {
	result, err := backend.service.Replay(ctx, id, mode)
	return deliveryDTO(result), retainDeliveryResult(result, err)
}

// DeleteRun removes a retained run and its related events and attempts.
func (backend *Backend) DeleteRun(ctx context.Context, id string) error {
	store, err := backend.service.History()
	if err != nil {
		return err
	}
	defer store.Close()
	return store.DeleteRun(ctx, id)
}
