package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleModelCatalogDoesNotExposeSecretsAndMarksAvailability(t *testing.T) {
	server, vault := newProviderTestServer(t, `
model_catalog:
  enabled: true
  catalog_only_visible: true
  disabled_providers:
    - openrouter
providers:
  - id: openai-main
    name: OpenAI
    type: openai
    base_url: https://api.openai.com/v1
    model: gpt-4o
llm:
  provider: openai-main
`)
	if err := vault.WriteSecret("provider_openai-main_api_key", "super-secret-api-key"); err != nil {
		t.Fatalf("WriteSecret() error = %v", err)
	}
	server.Cfg.ApplyVaultSecrets(vault)
	server.Cfg.ResolveProviders()

	req := httptest.NewRequest(http.MethodGet, "/api/models/catalog", nil)
	rec := httptest.NewRecorder()
	handleModelCatalog(server).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "super-secret-api-key") {
		t.Fatal("catalog response exposed a provider secret")
	}

	var body modelCatalogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !body.Enabled {
		t.Fatal("expected catalog response enabled")
	}

	openai := findCatalogProvider(body.Providers, "openai")
	if openai == nil {
		t.Fatal("expected openai provider in catalog")
	}
	if openai.Availability != "available" || !openai.Available {
		t.Fatalf("openai availability = %+v", openai)
	}

	openrouter := findCatalogProvider(body.Providers, "openrouter")
	if openrouter == nil {
		t.Fatal("expected openrouter provider in catalog")
	}
	if openrouter.Availability != "disabled" || openrouter.Available {
		t.Fatalf("openrouter availability = %+v", openrouter)
	}

	if catalogOnly := firstCatalogOnlyProvider(body.Providers); catalogOnly == nil {
		t.Fatal("expected at least one catalog_only provider")
	} else if catalogOnly.Available {
		t.Fatalf("catalog-only provider should not be runtime available: %+v", catalogOnly)
	}
}

func findCatalogProvider(providers []modelCatalogProviderResponse, id string) *modelCatalogProviderResponse {
	for i := range providers {
		if providers[i].ID == id || providers[i].AuraProviderType == id {
			return &providers[i]
		}
	}
	return nil
}

func firstCatalogOnlyProvider(providers []modelCatalogProviderResponse) *modelCatalogProviderResponse {
	for i := range providers {
		if providers[i].CatalogOnly {
			return &providers[i]
		}
	}
	return nil
}
