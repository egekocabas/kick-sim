package studio

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/config"
	"github.com/egekocabas/kick-sim/internal/delivery"
	"github.com/egekocabas/kick-sim/internal/events"
	"github.com/egekocabas/kick-sim/internal/history"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/version"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

var capabilities = []string{
	"studio.dashboard",
	"studio.event-builder",
	"studio.scenarios",
	"studio.scenario-source-editor",
	"studio.activity",
	"studio.delivery-inspection",
	"studio.replay.exact",
	"studio.replay.regenerated",
	"studio.openapi",
}

type Backend struct {
	service *app.Service
}

func NewBackend(service *app.Service) *Backend {
	return &Backend{service: service}
}

func (backend *Backend) Bootstrap(ctx context.Context) (kickopenapi.Bootstrap, error) {
	workspaceInfo, err := backend.Workspace(ctx)
	if err != nil {
		return kickopenapi.Bootstrap{}, err
	}
	key, err := backend.KeyInfo(ctx)
	if err != nil {
		return kickopenapi.Bootstrap{}, err
	}
	recent, err := backend.ListActivity(ctx, 8, 0)
	if err != nil {
		return kickopenapi.Bootstrap{}, err
	}
	return kickopenapi.Bootstrap{
		APIVersion:     1,
		Capabilities:   append([]string(nil), capabilities...),
		ProductVersion: version.Version,
		Workspace:      workspaceInfo,
		Key:            key,
		RecentActivity: recent,
	}, nil
}

func (backend *Backend) Workspace(context.Context) (kickopenapi.Workspace, error) {
	destination, err := backend.service.Config.Destination("")
	if err != nil {
		return kickopenapi.Workspace{}, err
	}
	paths := workspace.PathsFor(backend.service.Workspace)
	return kickopenapi.Workspace{
		Path:               backend.service.Workspace,
		DefaultDestination: backend.service.Config.DefaultDestination,
		DestinationURL:     destination.URL,
		HistoryEnabled:     backend.service.Config.History.Enabled,
		DatabasePath:       paths.Database,
	}, nil
}

func (backend *Backend) ValidateWorkspace(context.Context) error {
	return workspace.Validate(backend.service.Workspace)
}

func (backend *Backend) ListEvents(context.Context) ([]kickopenapi.EventContract, error) {
	definitions := backend.service.Events.List()
	items := make([]kickopenapi.EventContract, 0, len(definitions))
	for _, listed := range definitions {
		definition, err := backend.service.Events.Get(listed.Type, listed.Version)
		if err != nil {
			return nil, err
		}
		items = append(items, eventContract(definition))
	}
	return items, nil
}

func (backend *Backend) GetEvent(_ context.Context, eventType string, eventVersion int) (kickopenapi.EventContract, error) {
	definition, err := backend.service.Events.Get(eventType, eventVersion)
	if err != nil {
		return kickopenapi.EventContract{}, err
	}
	return eventContract(definition), nil
}

func (backend *Backend) ValidatePayload(_ context.Context, request kickopenapi.EventPayloadRequest) error {
	return backend.service.Events.Validate(request.EventType, request.EventVersion, request.Payload)
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

func (backend *Backend) ListScenarios(context.Context) ([]kickopenapi.ScenarioSummary, error) {
	entries, err := backend.scenarios().List()
	if err != nil {
		return nil, err
	}
	items := make([]kickopenapi.ScenarioSummary, len(entries))
	for index, entry := range entries {
		items[index] = scenarioSummary(entry)
	}
	return items, nil
}

func (backend *Backend) GetScenario(_ context.Context, id string) (kickopenapi.ScenarioDetail, error) {
	entry, err := backend.scenarios().Get(id)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) SaveScenarioCopy(_ context.Context, request kickopenapi.ScenarioCopyRequest) (kickopenapi.ScenarioDetail, error) {
	source, err := backend.scenarios().Get(request.SourceID)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	if err := backend.service.Events.Validate(source.Scenario.Request.Event.Type, source.Scenario.Request.Event.Version, request.Payload); err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	entry, err := backend.scenarios().SaveAsCopy(request.SourceID, request.TargetID, request.Payload, "kick-sim@"+version.Version)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) SaveScenarioSource(_ context.Context, request kickopenapi.ScenarioSourceSaveRequest) (kickopenapi.ScenarioDetail, error) {
	entry, err := backend.scenarios().SaveSource(request.ID, request.Revision, []byte(request.Source))
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) SaveScenarioSourceCopy(_ context.Context, request kickopenapi.ScenarioSourceCopyRequest) (kickopenapi.ScenarioDetail, error) {
	entry, err := backend.scenarios().SaveSourceAsCopy(request.SourceID, request.TargetID, []byte(request.Source), "kick-sim@"+version.Version)
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	return backend.scenarioDetail(entry)
}

func (backend *Backend) RunScenario(ctx context.Context, request kickopenapi.ScenarioRunRequest) (kickopenapi.DeliveryResult, error) {
	entry, err := backend.scenarios().Get(request.ScenarioID)
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	if err := backend.scenarios().Validate(entry); err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	payload := request.Payload
	if payload == nil {
		payload, err = backend.service.GeneratePayload(app.PayloadOptions{
			EventType:    entry.Scenario.Request.Event.Type,
			EventVersion: entry.Scenario.Request.Event.Version,
			Scenario:     entry.Scenario.Request.Payload,
			Omit:         entry.Scenario.Request.Omit,
		})
		if err != nil {
			return kickopenapi.DeliveryResult{}, err
		}
	}
	options, err := backend.deliveryOptions(kickopenapi.EventPayloadRequest{
		EventType:    entry.Scenario.Request.Event.Type,
		EventVersion: entry.Scenario.Request.Event.Version,
		Payload:      payload,
	})
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	options.ScenarioDefinitionID = entry.ID
	options.ScenarioSourceVersion = entry.SourceVersion
	subscriptionID := request.SubscriptionID
	if subscriptionID == "" {
		subscriptionID = entry.Scenario.Request.Delivery.SubscriptionID
	}
	generated, err := backend.service.Generate(options, subscriptionID)
	if err != nil {
		return kickopenapi.DeliveryResult{}, err
	}
	destinationName := request.Destination
	if destinationName == "" {
		destinationName = entry.Scenario.Request.Delivery.Destination
	}
	result, err := backend.service.Deliver(ctx, generated, destinationName, request.DestinationURL, entry.Scenario.Request.Delivery.Expect.Statuses)
	return deliveryDTO(result), retainDeliveryResult(result, err)
}

func (backend *Backend) ListActivity(ctx context.Context, limit, offset int) ([]history.Activity, error) {
	store, err := backend.service.History()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.ListActivity(ctx, limit, offset)
}

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
		detail.Event.Headers[delivery.HeaderMessageID],
		detail.Event.MessageTimestamp,
		[]byte(detail.Event.RawBody),
	)
	return kickopenapi.DeliveryAttemptDetail{
		Run:            detail.Run,
		Event:          detail.Event,
		Attempt:        detail.Attempt,
		SignatureInput: string(input),
	}, nil
}

func (backend *Backend) ReplayAttempt(ctx context.Context, id, mode string) (kickopenapi.DeliveryResult, error) {
	result, err := backend.service.Replay(ctx, id, mode)
	return deliveryDTO(result), retainDeliveryResult(result, err)
}

func (backend *Backend) DeleteRun(ctx context.Context, id string) error {
	store, err := backend.service.History()
	if err != nil {
		return err
	}
	defer store.Close()
	return store.DeleteRun(ctx, id)
}

func (backend *Backend) PublicKey(context.Context) (kickopenapi.PublicKey, error) {
	path := config.ResolvePath(backend.service.Workspace, backend.service.Config.Signing.PublicKey)
	data, err := os.ReadFile(path)
	if err != nil {
		return kickopenapi.PublicKey{}, fmt.Errorf("read public key: %w", err)
	}
	return kickopenapi.PublicKey{Path: path, PEM: string(data)}, nil
}

func (backend *Backend) KeyInfo(context.Context) (kickopenapi.KeyInfo, error) {
	publicPath := config.ResolvePath(backend.service.Workspace, backend.service.Config.Signing.PublicKey)
	privatePath := config.ResolvePath(backend.service.Workspace, backend.service.Config.Signing.PrivateKey)
	publicKey, err := signing.ReadPublicKey(publicPath)
	if err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	fingerprint, err := signing.Fingerprint(publicKey)
	if err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	info, err := os.Stat(publicPath)
	if err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	privateKey, privateErr := signing.ReadPrivateKey(privatePath)
	return kickopenapi.KeyInfo{
		Path:               publicPath,
		Algorithm:          "RSA",
		Bits:               publicKey.N.BitLen(),
		Fingerprint:        fingerprint,
		ModifiedAt:         info.ModTime().UTC(),
		MatchingPrivateKey: privateErr == nil && samePublicKey(&privateKey.PublicKey, publicKey),
	}, nil
}

func (backend *Backend) RotateKey(ctx context.Context) (kickopenapi.KeyInfo, error) {
	if err := workspace.RotateKeys(backend.service.Workspace); err != nil {
		return kickopenapi.KeyInfo{}, err
	}
	return backend.KeyInfo(ctx)
}

func (backend *Backend) scenarios() *scenario.Store {
	return scenario.NewStore(backend.service.Workspace, backend.service.Events, backend.service.Config)
}

func (backend *Backend) scenarioDetail(entry scenario.Entry) (kickopenapi.ScenarioDetail, error) {
	detail := kickopenapi.ScenarioDetail{
		ScenarioSummary: scenarioSummary(entry),
		Source:          string(entry.Source),
	}
	if err := backend.scenarios().Validate(entry); err != nil {
		return detail, nil
	}
	draft, err := backend.service.GeneratePayload(app.PayloadOptions{
		EventType:    entry.Scenario.Request.Event.Type,
		EventVersion: entry.Scenario.Request.Event.Version,
		Scenario:     entry.Scenario.Request.Payload,
		Omit:         entry.Scenario.Request.Omit,
	})
	if err != nil {
		return kickopenapi.ScenarioDetail{}, err
	}
	detail.Payload = events.DeepCopyMap(entry.Scenario.Request.Payload)
	detail.DraftPayload = draft
	detail.Destination = entry.Scenario.Request.Delivery.Destination
	detail.ExpectedStatuses = append([]int(nil), entry.Scenario.Request.Delivery.Expect.Statuses...)
	return detail, nil
}

func (backend *Backend) deliveryOptions(request kickopenapi.EventPayloadRequest) (app.PayloadOptions, error) {
	if err := backend.service.Events.Validate(request.EventType, request.EventVersion, request.Payload); err != nil {
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
	destination, err := backend.service.Config.Destination(name)
	if err != nil {
		return "", err
	}
	return destination.URL, nil
}

func eventContract(definition events.Definition) kickopenapi.EventContract {
	return kickopenapi.EventContract{
		Type:        definition.Type,
		Version:     definition.Version,
		Description: definition.Description,
		EventSchema: definition.Schema,
		Defaults:    events.DeepCopyMap(definition.Defaults),
	}
}

func scenarioSummary(entry scenario.Entry) kickopenapi.ScenarioSummary {
	return kickopenapi.ScenarioSummary{
		ID:               entry.ID,
		Name:             entry.Scenario.Name,
		Description:      entry.Scenario.Description,
		BuiltIn:          entry.BuiltIn,
		EventType:        entry.Scenario.Request.Event.Type,
		EventVersion:     entry.Scenario.Request.Event.Version,
		Revision:         entry.Revision,
		SourceVersion:    entry.SourceVersion,
		SourceFormat:     entry.SourceFormat,
		Valid:            len(entry.ValidationErrors) == 0,
		ValidationErrors: append([]string(nil), entry.ValidationErrors...),
	}
}

func generatedDTO(generated app.Generated, destinationURL string) kickopenapi.GeneratedEvent {
	return kickopenapi.GeneratedEvent{
		RunID:               generated.RunID,
		GeneratedEventID:    generated.GeneratedEventID,
		EventType:           generated.EventType,
		EventVersion:        generated.EventVersion,
		BroadcasterUserID:   generated.BroadcasterUserID,
		LogicalSubscription: generated.LogicalSubscription,
		MessageID:           generated.MessageID,
		SubscriptionID:      generated.SubscriptionID,
		MessageTimestamp:    generated.MessageTimestamp,
		Headers:             generated.Headers,
		Payload:             generated.Payload,
		RawBody:             generated.RawBody,
		RawHTTP:             rawHTTPRequest(destinationURL, generated.Headers, generated.RawBody),
	}
}

func deliveryDTO(result app.RunResult) kickopenapi.DeliveryResult {
	generated := generatedDTO(result.Generated, result.URL)
	return kickopenapi.DeliveryResult{
		GeneratedEvent:     generated,
		AttemptID:          result.AttemptID,
		ReplayOfID:         result.ReplayOfID,
		ReplayMode:         result.ReplayMode,
		Destination:        result.Destination,
		URL:                result.URL,
		Method:             result.Method,
		Status:             result.Status,
		DurationMS:         result.DurationMS,
		TransportStartedAt: result.TransportStartedAt,
		Outcome:            result.Outcome,
		ResponseHeaders:    result.ResponseHeaders,
		ResponseBody:       result.ResponseBody,
		ResponseTruncated:  result.ResponseTruncated,
		Error:              result.Error,
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

func samePublicKey(left, right *rsa.PublicKey) bool {
	return left != nil && right != nil && left.E == right.E && left.N.Cmp(right.N) == 0
}

var _ kickopenapi.Backend = (*Backend)(nil)
