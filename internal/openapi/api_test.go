package openapi

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestPublicAPIContract(t *testing.T) {
	document, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(document, &parsed); err != nil {
		t.Fatal(err)
	}
	if Version != "1.0.0" || parsed.Info.Version != Version {
		t.Fatalf("API version = %q, generated version = %q", Version, parsed.Info.Version)
	}

	operationIDs := make([]string, 0)
	for _, operations := range parsed.Paths {
		for _, operation := range operations {
			if operation.OperationID != "" {
				operationIDs = append(operationIDs, operation.OperationID)
			}
		}
	}
	sort.Strings(operationIDs)
	want := []string{
		"deleteRun",
		"duplicateScenario",
		"generateEvent",
		"getBootstrap",
		"getCapabilities",
		"getDeliveryAttempt",
		"getEvent",
		"getScenario",
		"getSimulatorKeyInfo",
		"getSimulatorPublicKey",
		"getSuite",
		"getWorkspace",
		"listActors",
		"listEvents",
		"listRuns",
		"listScenarios",
		"listSuites",
		"replayDeliveryAttempt",
		"rotateSimulatorKey",
		"runScenario",
		"runSuite",
		"runWorkflow",
		"saveScenarioSourceCopy",
		"triggerEvent",
		"updateScenarioSource",
		"validateEvent",
		"validateWorkspace",
	}
	if !reflect.DeepEqual(operationIDs, want) {
		t.Fatalf("operation IDs = %#v, want %#v", operationIDs, want)
	}
}

func TestGeneratedDocumentIsDeterministicAndCommitted(t *testing.T) {
	first, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generated OpenAPI document is not deterministic")
	}
	committed, err := os.ReadFile("../../web/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var generatedDocument any
	var committedDocument any
	if err := json.Unmarshal(first, &generatedDocument); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(committed, &committedDocument); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(generatedDocument, committedDocument) {
		t.Fatal("web/openapi.json is stale; run make generate")
	}
}
