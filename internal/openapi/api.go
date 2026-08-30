package openapi

import (
	"encoding/json"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

// Version is the Studio API contract version emitted in OpenAPI documents.
const Version = "1.0.0"

// New registers the Studio API on router using backend as its implementation.
func New(router chi.Router, backend Backend) huma.API {
	configuration := huma.DefaultConfig("Kick Sim Studio API", Version)
	configuration.DocsPath = ""
	configuration.SchemasPath = ""
	configuration.OpenAPIPath = "/api/openapi"
	configuration.OpenAPI.Servers = nil
	configuration.RejectUnknownQueryParameters = true
	api := humachi.New(router, configuration)
	Register(api, backend)
	return api
}

// Generate returns the canonical formatted OpenAPI document.
func Generate() ([]byte, error) {
	router := chi.NewMux()
	api := New(router, nil)
	data, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize OpenAPI document: %w", err)
	}
	return append(data, '\n'), nil
}
