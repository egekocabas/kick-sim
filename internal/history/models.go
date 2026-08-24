package history

import "time"

type Record struct {
	Run     Run
	Event   Event
	Attempt Attempt
}

type Run struct {
	ID                    string    `json:"id"`
	ScenarioDefinitionID  string    `json:"scenarioDefinitionId,omitempty"`
	ScenarioSourceVersion int       `json:"scenarioSourceVersion,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
}

type Event struct {
	ID                  string            `json:"id"`
	RunID               string            `json:"runId"`
	EventType           string            `json:"eventType"`
	EventVersion        int               `json:"eventVersion"`
	BroadcasterUserID   int64             `json:"broadcasterUserId"`
	LogicalSubscription string            `json:"logicalSubscription"`
	MessageTimestamp    string            `json:"messageTimestamp"`
	Payload             map[string]any    `json:"payload"`
	RawBody             string            `json:"rawBody"`
	Headers             map[string]string `json:"headers"`
	CreatedAt           time.Time         `json:"createdAt"`
}

type Attempt struct {
	ID                 string              `json:"id"`
	GeneratedEventID   string              `json:"generatedEventId"`
	ReplayOfID         string              `json:"replayOfId,omitempty"`
	ReplayMode         string              `json:"replayMode,omitempty"`
	Destination        string              `json:"destination"`
	URL                string              `json:"url"`
	Method             string              `json:"method"`
	RequestHeaders     map[string]string   `json:"requestHeaders"`
	ResponseStatus     int                 `json:"responseStatus,omitempty"`
	ResponseHeaders    map[string][]string `json:"responseHeaders,omitempty"`
	ResponseBody       string              `json:"responseBody,omitempty"`
	ResponseTruncated  bool                `json:"responseTruncated"`
	TransportStartedAt time.Time           `json:"transportStartedAt"`
	DurationMS         float64             `json:"durationMs"`
	Outcome            string              `json:"outcome"`
	Error              string              `json:"error,omitempty"`
	CreatedAt          time.Time           `json:"createdAt"`
}

type AttemptDetail struct {
	Run     Run     `json:"run"`
	Event   Event   `json:"event"`
	Attempt Attempt `json:"attempt"`
}

type Activity struct {
	AttemptID            string    `json:"attemptId"`
	RunID                string    `json:"runId"`
	ScenarioDefinitionID string    `json:"scenarioDefinitionId,omitempty"`
	EventType            string    `json:"eventType"`
	EventVersion         int       `json:"eventVersion"`
	Status               int       `json:"status,omitempty"`
	Outcome              string    `json:"outcome"`
	DurationMS           float64   `json:"durationMs"`
	CreatedAt            time.Time `json:"createdAt"`
}
