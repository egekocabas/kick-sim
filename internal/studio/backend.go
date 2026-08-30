// Package studio serves the loopback-only API and browser application.
package studio

import (
	"context"

	"github.com/egekocabas/kick-sim/internal/app"
	kickopenapi "github.com/egekocabas/kick-sim/internal/openapi"
	"github.com/egekocabas/kick-sim/internal/scenario"
	"github.com/egekocabas/kick-sim/internal/suite"
	"github.com/egekocabas/kick-sim/internal/version"
	"github.com/egekocabas/kick-sim/internal/workspace"
)

var capabilities = []string{
	"studio.dashboard", "studio.event-builder", "studio.scenarios",
	"studio.scenario-source-editor", "studio.workflows", "studio.suites",
	"actors", "studio.activity", "studio.delivery-inspection",
	"studio.replay.exact", "studio.replay.regenerated", "studio.openapi",
}

// Backend adapts simulator services to the Studio API contract.
type Backend struct {
	service   *app.Service
	scenarios *scenario.Store
	suites    *suite.Store
}

// NewBackend constructs a Studio backend for service.
func NewBackend(service *app.Service) *Backend {
	scenarios := scenario.NewStore(service.WorkspaceRoot(), service.EventRegistry(), service.Configuration(), service.ActorRegistry())
	return &Backend{service: service, scenarios: scenarios, suites: suite.NewStore(service.WorkspaceRoot(), scenarios)}
}

// Bootstrap returns the initial workspace, key, capability, and activity state.
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
		APIVersion: 1, Capabilities: append([]string(nil), capabilities...),
		ProductVersion: version.Version, Workspace: workspaceInfo, Key: key, RecentActivity: recent,
	}, nil
}

// Workspace returns the non-secret active workspace summary.
func (backend *Backend) Workspace(context.Context) (kickopenapi.Workspace, error) {
	configuration := backend.service.Configuration()
	destination, err := configuration.Destination("")
	if err != nil {
		return kickopenapi.Workspace{}, err
	}
	paths := workspace.PathsFor(backend.service.WorkspaceRoot())
	return kickopenapi.Workspace{
		Path: backend.service.WorkspaceRoot(), DefaultDestination: configuration.DefaultDestination,
		DestinationURL: destination.URL, HistoryEnabled: configuration.History.Enabled, DatabasePath: paths.Database,
	}, nil
}

// ValidateWorkspace checks the active configuration and signing key pair.
func (backend *Backend) ValidateWorkspace(context.Context) error {
	return workspace.Validate(backend.service.WorkspaceRoot())
}

var _ kickopenapi.Backend = (*Backend)(nil)
