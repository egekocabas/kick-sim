package studio

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/events"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/suite"
)

func eventContract(definition events.Definition) kickopenapi.EventContract {
	return kickopenapi.EventContract{
		Type: definition.Type, Version: definition.Version, Description: definition.Description,
		EventSchema: definition.Schema, Defaults: events.DeepCopyMap(definition.Defaults),
	}
}

func scenarioSummary(entry scenario.Entry) kickopenapi.ScenarioSummary {
	return kickopenapi.ScenarioSummary{
		ID: entry.ID, Name: entry.Scenario.Name, Description: entry.Scenario.Description,
		BuiltIn: entry.BuiltIn, Kind: entry.Scenario.Kind(), EventType: entry.Scenario.Request.Event.Type,
		EventVersion: entry.Scenario.Request.Event.Version, Revision: entry.Revision,
		SourceVersion: entry.SourceVersion, SourceFormat: entry.SourceFormat,
		Valid: len(entry.ValidationErrors) == 0, ValidationErrors: append([]string(nil), entry.ValidationErrors...),
	}
}

func suiteSummary(entry suite.Entry) kickopenapi.SuiteSummary {
	return kickopenapi.SuiteSummary{
		ID: entry.ID, Name: entry.Suite.Name, Description: entry.Suite.Description,
		BuiltIn: entry.BuiltIn, Cases: len(entry.Suite.Cases), Valid: true,
	}
}

func generatedDTO(generated app.Generated, destinationURL string) kickopenapi.GeneratedEvent {
	return kickopenapi.GeneratedEvent{
		RunID: generated.RunID, GeneratedEventID: generated.GeneratedEventID,
		EventType: generated.EventType, EventVersion: generated.EventVersion,
		BroadcasterUserID: generated.BroadcasterUserID, LogicalSubscription: generated.LogicalSubscription,
		MessageID: generated.MessageID, SubscriptionID: generated.SubscriptionID,
		MessageTimestamp: generated.MessageTimestamp, Headers: generated.Headers,
		Payload: generated.Payload, RawBody: generated.RawBody,
		RawHTTP: rawHTTPRequest(destinationURL, generated.Headers, generated.RawBody),
	}
}

func deliveryDTO(result app.RunResult) kickopenapi.DeliveryResult {
	generated := generatedDTO(result.Generated, result.URL)
	return kickopenapi.DeliveryResult{
		GeneratedEvent: generated, AttemptID: result.AttemptID, ReplayOfID: result.ReplayOfID,
		ReplayMode: result.ReplayMode, Destination: result.Destination, URL: result.URL,
		Method: result.Method, Status: result.Status, DurationMS: result.DurationMS,
		TransportStartedAt: result.TransportStartedAt, Outcome: result.Outcome,
		ResponseHeaders: result.ResponseHeaders, ResponseBody: result.ResponseBody,
		ResponseTruncated: result.ResponseTruncated, Error: result.Error,
	}
}

func retainDeliveryResult(result app.RunResult, err error) error {
	if err == nil {
		return nil
	}
	if result.AttemptID != "" && result.Error != "" {
		return nil
	}
	return err
}

func rawHTTPRequest(destinationURL string, headers map[string]string, body string) string {
	parsed, err := url.Parse(destinationURL)
	if err != nil {
		return body
	}
	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s %s HTTP/1.1\r\n", http.MethodPost, path)
	fmt.Fprintf(&builder, "Host: %s\r\n", parsed.Host)
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&builder, "%s: %s\r\n", key, headers[key])
	}
	builder.WriteString("\r\n")
	builder.WriteString(body)
	return builder.String()
}
