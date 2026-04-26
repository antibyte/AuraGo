package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestYepAPIClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test_key" {
			http.Error(w, `{"ok":false,"error":{"code":"INVALID_API_KEY","message":"Invalid key"}}`, http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/v1/seo/keywords" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"ok":true,"data":[{"keyword":"test","searchVolume":1000}]}`)
			return
		}
		if r.URL.Path == "/v1/serp/google" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"ok":true,"data":{"query":"hello","items":[{"title":"Result"}]}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":false,"error":{"code":"NOT_FOUND","message":"Unknown endpoint"}}`)
	}))
	defer server.Close()

	client := NewYepAPIClient("test_key")
	client.baseURL = server.URL

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		data, err := client.Post(ctx, "/v1/seo/keywords", map[string]interface{}{"keywords": []string{"test"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed []map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if len(parsed) != 1 {
			t.Fatalf("expected 1 result, got %d", len(parsed))
		}
	})

	t.Run("invalid_api_key", func(t *testing.T) {
		badClient := NewYepAPIClient("wrong_key")
		badClient.baseURL = server.URL
		_, err := badClient.Post(ctx, "/v1/seo/keywords", map[string]interface{}{"keywords": []string{"test"}})
		if err == nil {
			t.Fatal("expected error for invalid API key")
		}
	})

	t.Run("error_response", func(t *testing.T) {
		_, err := client.Post(ctx, "/v1/unknown", nil)
		if err == nil {
			t.Fatal("expected error for unknown endpoint")
		}
	})
}

func TestDispatchYepAPISEO(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"data":{"domain":"example.com","organicTraffic":50000}}`)
	}))
	defer server.Close()

	client := NewYepAPIClient("test_key")
	client.baseURL = server.URL

	ctx := context.Background()
	res, err := DispatchYepAPISEO(ctx, client, "domain_overview", map[string]interface{}{"domain": "example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestDispatchYepAPISERP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"data":{"query":"test","items":[]}}`)
	}))
	defer server.Close()

	client := NewYepAPIClient("test_key")
	client.baseURL = server.URL

	ctx := context.Background()
	res, err := DispatchYepAPISERP(ctx, client, "google", map[string]interface{}{"query": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestDispatchYepAPIScrape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true,"data":{"url":"https://example.com","content":"# Hello"}}`)
	}))
	defer server.Close()

	client := NewYepAPIClient("test_key")
	client.baseURL = server.URL

	ctx := context.Background()
	res, err := DispatchYepAPIScrape(ctx, client, "scrape", map[string]interface{}{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == "" {
		t.Fatal("expected non-empty result")
	}
}

func TestResolveYepAPIKey(t *testing.T) {
	t.Run("from_provider", func(t *testing.T) {
		cfg := &config.Config{
			Providers: []config.ProviderEntry{
				{ID: "yepapi_main", Type: "yepapi", APIKey: "provider_key"},
			},
		}
		vault := &mockSecretReader{secrets: map[string]string{}}
		key, err := ResolveYepAPIKey(cfg, vault)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "provider_key" {
			t.Fatalf("expected 'provider_key', got %q", key)
		}
	})

	t.Run("from_vault", func(t *testing.T) {
		cfg := &config.Config{}
		vault := &mockSecretReader{secrets: map[string]string{"yepapi_api_key": "vault_key"}}
		key, err := ResolveYepAPIKey(cfg, vault)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "vault_key" {
			t.Fatalf("expected 'vault_key', got %q", key)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		cfg := &config.Config{}
		vault := &mockSecretReader{secrets: map[string]string{}}
		_, err := ResolveYepAPIKey(cfg, vault)
		if err == nil {
			t.Fatal("expected error when key is not found")
		}
	})
}

type mockSecretReader struct {
	secrets map[string]string
}

func (m *mockSecretReader) ReadSecret(key string) (string, error) {
	if v, ok := m.secrets[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("secret not found: %s", key)
}
