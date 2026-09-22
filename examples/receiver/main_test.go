package main

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/egekocabas/kick-sim/internal/app"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/signing"
	"github.com/egekocabas/kick-sim/internal/suite"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

func TestExampleReceiverPassesSecuritySuite(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".kick-sim")
	if _, err := workspace.Init(root); err != nil {
		t.Fatal(err)
	}
	service, err := app.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := signing.ReadPublicKey(workspace.PathsFor(root).PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	receiver := httptest.NewServer(receiverHandler(publicKey))
	defer receiver.Close()
	scenarios := scenario.NewStore(root, service.EventRegistry(), service.Configuration(), service.ActorRegistry())
	suites := suite.NewStore(root, scenarios)
	entry, err := suites.Get("builtin:security")
	if err != nil {
		t.Fatal(err)
	}
	report, err := suite.Run(context.Background(), service, scenarios, entry, suite.RunOptions{DestinationURL: receiver.URL + "/webhooks/kick"})
	if err != nil {
		t.Fatalf("security suite: %v", err)
	}
	if report.PassedCases != 5 || report.FailedCases != 0 {
		t.Fatalf("report = %+v", report)
	}
}
