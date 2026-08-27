package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/signing"
)

func (service *Service) Deliver(ctx context.Context, generated Generated, destinationName, temporaryURL string, expectedStatuses []int) (RunResult, error) {
	return service.deliver(ctx, generated, destinationName, temporaryURL, expectedStatuses, "", "")
}

func (service *Service) ApplyDeliveryFailure(generated Generated, failure string) (Generated, error) {
	generated.Headers = cloneStringMap(generated.Headers)
	generated.Payload = events.DeepCopyMap(generated.Payload)
	generated.body = append([]byte(nil), generated.body...)
	switch failure {
	case "":
		return generated, nil
	case "invalid-signature":
		generated.Headers[delivery.HeaderSignature] = "invalid"
	case "missing-signature":
		delete(generated.Headers, delivery.HeaderSignature)
	case "modified-body":
		generated.RawBody += " "
		generated.body = []byte(generated.RawBody)
	case "malformed-json":
		generated.RawBody = "{"
		generated.body = []byte(generated.RawBody)
		privateKey, err := service.privateKey()
		if err != nil {
			return Generated{}, err
		}
		signature, err := signing.Sign(privateKey, generated.MessageID, generated.MessageTimestamp, generated.body)
		if err != nil {
			return Generated{}, err
		}
		generated.Headers[delivery.HeaderSignature] = signature
	default:
		return Generated{}, fmt.Errorf("unsupported delivery failure %q", failure)
	}
	return generated, nil
}

func (service *Service) deliver(ctx context.Context, generated Generated, destinationName, temporaryURL string, expectedStatuses []int, replayOfID, replayMode string) (RunResult, error) {
	configuredName := destinationName
	if temporaryURL != "" && (configuredName == "" || configuredName == "temporary") {
		configuredName = service.configuration.DefaultDestination
	}
	destination, err := service.configuration.Destination(configuredName)
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
	if len(generated.body) == 0 {
		generated.body = []byte(generated.RawBody)
	}
	attemptID := service.newID()
	client := delivery.NewLoopbackClient(timeout)
	defer client.CloseIdleConnections()
	result, sendErr := delivery.Send(ctx, client, destination.URL, delivery.Metadata{
		MessageID: generated.MessageID, SubscriptionID: generated.SubscriptionID,
		Signature: generated.Headers[delivery.HeaderSignature], Timestamp: generated.MessageTimestamp,
		EventType: generated.EventType, EventVersion: fmt.Sprint(generated.EventVersion),
		OmitSignature: !hasHeader(generated.Headers, delivery.HeaderSignature),
	}, generated.body)
	run := RunResult{
		Generated: generated, AttemptID: attemptID, ReplayOfID: replayOfID, ReplayMode: replayMode,
		Destination: destinationName, URL: destination.URL, Method: http.MethodPost,
		Status: result.StatusCode, Duration: result.Duration,
		DurationMS:         float64(result.Duration) / float64(time.Millisecond),
		TransportStartedAt: result.StartedAt, ResponseHeaders: cloneHeader(result.Headers),
		ResponseBody: string(result.ResponseBody), ResponseTruncated: result.BodyTruncated,
	}
	var deliveryErr error
	if sendErr != nil {
		run.Outcome = "transport_failed"
		deliveryErr = sendErr
	} else if result.StatusCode >= 200 && result.StatusCode < 300 {
		run.Outcome = "http_accepted"
	} else {
		run.Outcome = "http_rejected"
	}
	if deliveryErr == nil && len(expectedStatuses) == 0 && (result.StatusCode < 200 || result.StatusCode >= 300) {
		deliveryErr = fmt.Errorf("webhook receiver returned HTTP %d", result.StatusCode)
	}
	if deliveryErr == nil && len(expectedStatuses) > 0 && !containsStatus(expectedStatuses, result.StatusCode) {
		deliveryErr = fmt.Errorf("webhook receiver returned HTTP %d; expected one of %v", result.StatusCode, expectedStatuses)
	}
	if deliveryErr != nil {
		run.Error = deliveryErr.Error()
	}
	historyErr := service.record(ctx, run)
	if historyErr != nil {
		run.Error = errors.Join(deliveryErr, fmt.Errorf("retain delivery history: %w", historyErr)).Error()
	}
	return run, errors.Join(deliveryErr, historyErr)
}

func cloneHeader(header http.Header) map[string][]string {
	if header == nil {
		return map[string][]string{}
	}
	cloned := make(map[string][]string, len(header))
	for key, values := range header {
		cloned[key] = append([]string(nil), values...)
	}
	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func containsStatus(statuses []int, actual int) bool {
	for _, status := range statuses {
		if status == actual {
			return true
		}
	}
	return false
}

func hasHeader(headers map[string]string, name string) bool {
	_, exists := headers[name]
	return exists
}
