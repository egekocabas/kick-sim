package openapi

import (
	"encoding/json"
	"fmt"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
)

const Version = "1.0.0"

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

func Generate() ([]byte, error) {
	router := chi.NewMux()
	api := New(router, nil)
	data, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize OpenAPI document: %w", err)
	}
	return append(data, '\n'), nil
}
