package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
)

func newYepAPITestClient(handler func(*http.Request) (int, string)) *YepAPIClient {
	client := NewYepAPIClient("test_key")
	client.baseURL = "https://yepapi.test"
	client.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := handler(r)
		return &http.Response{
			StatusCode: status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
	return client
}

func TestYepAPIClient_Post(t *testing.T) {
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		status := http.StatusOK
		body := `{"ok":false,"error":{"code":"NOT_FOUND","message":"Unknown endpoint"}}`
		if r.Header.Get("x-api-key") != "test_key" {
			status = http.StatusUnauthorized
			body = `{"ok":false,"error":{"code":"INVALID_API_KEY","message":"Invalid key"}}`
		} else if r.URL.Path == "/v1/seo/keywords" {
			body = `{"ok":true,"data":[{"keyword":"test","searchVolume":1000}]}`
		} else if r.URL.Path == "/v1/serp/google" {
			body = `{"ok":true,"data":{"query":"hello","items":[{"title":"Result"}]}}`
		}
		return status, body
	})

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
		badClient.baseURL = client.baseURL
		badClient.client = client.client
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
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		return http.StatusOK, `{"ok":true,"data":{"domain":"example.com","organicTraffic":50000}}`
	})

	ctx := context.Background()

	t.Run("domain_overview", func(t *testing.T) {
		res, err := DispatchYepAPISEO(ctx, client, "domain_overview", map[string]interface{}{"domain": "example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("keywords", func(t *testing.T) {
		res, err := DispatchYepAPISEO(ctx, client, "keywords", map[string]interface{}{"keywords": []string{"seo", "marketing"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("missing_domain", func(t *testing.T) {
		res, err := DispatchYepAPISEO(ctx, client, "domain_overview", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		json.Unmarshal([]byte(res), &out)
		if out["status"] != "error" {
			t.Fatalf("expected error for missing domain, got: %s", res)
		}
	})
}

func TestDispatchYepAPISERP(t *testing.T) {
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		body := map[string]interface{}{"query": "test", "items": []interface{}{}}
		b, _ := json.Marshal(body)
		return http.StatusOK, `{"ok":true,"data":` + string(b) + `}`
	})

	ctx := context.Background()

	t.Run("google", func(t *testing.T) {
		res, err := DispatchYepAPISERP(ctx, client, "google", map[string]interface{}{"query": "test"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("google_maps", func(t *testing.T) {
		res, err := DispatchYepAPISERP(ctx, client, "google_maps", map[string]interface{}{"query": "restaurants", "limit": 5.0, "open_now": true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("missing_query", func(t *testing.T) {
		res, err := DispatchYepAPISERP(ctx, client, "google", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		json.Unmarshal([]byte(res), &out)
		if out["status"] != "error" {
			t.Fatalf("expected error for missing query, got: %s", res)
		}
	})
}

func TestDispatchYepAPIScrape(t *testing.T) {
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		return http.StatusOK, `{"ok":true,"data":{"url":"https://example.com","content":"# Hello"}}`
	})

	ctx := context.Background()

	t.Run("scrape", func(t *testing.T) {
		res, err := DispatchYepAPIScrape(ctx, client, "scrape", map[string]interface{}{"url": "https://example.com"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("ssrf_rejection", func(t *testing.T) {
		res, err := DispatchYepAPIScrape(ctx, client, "scrape", map[string]interface{}{"url": "file:///etc/passwd"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		json.Unmarshal([]byte(res), &out)
		if out["status"] != "error" {
			t.Fatalf("expected SSRF rejection, got: %s", res)
		}
	})

	t.Run("malformed_http_url_rejection", func(t *testing.T) {
		res, err := DispatchYepAPIScrape(ctx, client, "scrape", map[string]interface{}{"url": "https://"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		json.Unmarshal([]byte(res), &out)
		if out["status"] != "error" {
			t.Fatalf("expected malformed URL rejection, got: %s", res)
		}
	})

	t.Run("stealth_format", func(t *testing.T) {
		res, err := DispatchYepAPIScrape(ctx, client, "stealth", map[string]interface{}{"url": "https://example.com", "format": "html"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("missing_url", func(t *testing.T) {
		res, err := DispatchYepAPIScrape(ctx, client, "scrape", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var out map[string]interface{}
		json.Unmarshal([]byte(res), &out)
		if out["status"] != "error" {
			t.Fatalf("expected error for missing url, got: %s", res)
		}
	})
}

func TestDispatchYepAPIYouTube(t *testing.T) {
	type yepAPIRequest struct {
		Path    string
		Payload map[string]interface{}
	}
	requests := make(chan yepAPIRequest, 4)
	client := NewYepAPIClient("test_key")
	client.baseURL = "https://yepapi.test"
	client.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		requests <- yepAPIRequest{Path: r.URL.Path, Payload: payload}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"data":{"items":[]}}`)),
			Request:    r,
		}, nil
	})}

	ctx := context.Background()

	t.Run("search", func(t *testing.T) {
		res, err := DispatchYepAPIYouTube(ctx, client, "search", map[string]interface{}{"query": "golang"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/youtube/search" || got.Payload["query"] != "golang" {
			t.Fatalf("request = %+v, want search endpoint and query", got)
		}
	})

	t.Run("video", func(t *testing.T) {
		res, err := DispatchYepAPIYouTube(ctx, client, "video", map[string]interface{}{"video_id": "dQw4w9WgXcQ"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/youtube/video" || got.Payload["videoId"] != "dQw4w9WgXcQ" {
			t.Fatalf("request = %+v, want video endpoint and videoId", got)
		}
	})

	t.Run("search_with_limit", func(t *testing.T) {
		res, err := DispatchYepAPIYouTube(ctx, client, "search", map[string]interface{}{"query": "golang", "limit": 5.0})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/youtube/search" || got.Payload["query"] != "golang" || got.Payload["limit"] != float64(5) {
			t.Fatalf("request = %+v, want search endpoint with limit", got)
		}
	})
}

func TestDispatchYepAPITikTok(t *testing.T) {
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		return http.StatusOK, `{"ok":true,"data":{}}`
	})

	ctx := context.Background()

	t.Run("search", func(t *testing.T) {
		res, err := DispatchYepAPITikTok(ctx, client, "search", map[string]interface{}{"query": "cooking"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})
}

func TestDispatchYepAPIInstagram(t *testing.T) {
	type yepAPIRequest struct {
		Path    string
		Payload map[string]interface{}
	}
	requests := make(chan yepAPIRequest, 3)
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests <- yepAPIRequest{Path: r.URL.Path, Payload: payload}
		return http.StatusOK, `{"ok":true,"data":{}}`
	})

	ctx := context.Background()

	t.Run("user_maps_username_to_api_field", func(t *testing.T) {
		res, err := DispatchYepAPIInstagram(ctx, client, "user", map[string]interface{}{"username": "natgeo"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/instagram/user" || got.Payload["username"] != "natgeo" {
			t.Fatalf("request = %+v, want user endpoint with username", got)
		}
		if _, ok := got.Payload["username_or_url"]; ok {
			t.Fatalf("request = %+v, did not expect username_or_url field to be sent to API", got)
		}
	})

	t.Run("user_accepts_username_or_url_alias", func(t *testing.T) {
		res, err := DispatchYepAPIInstagram(ctx, client, "user", map[string]interface{}{"username_or_url": "https://www.instagram.com/natgeo/"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/instagram/user" || got.Payload["username"] != "https://www.instagram.com/natgeo/" {
			t.Fatalf("request = %+v, want user endpoint with username alias value", got)
		}
	})

	t.Run("user_posts_maps_username_to_api_field", func(t *testing.T) {
		res, err := DispatchYepAPIInstagram(ctx, client, "user_posts", map[string]interface{}{"username": "natgeo", "limit": 5.0})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/instagram/user-posts" || got.Payload["username"] != "natgeo" || got.Payload["limit"] != float64(5) {
			t.Fatalf("request = %+v, want user-posts endpoint with username and limit", got)
		}
		if _, ok := got.Payload["username_or_url"]; ok {
			t.Fatalf("request = %+v, did not expect username_or_url field to be sent to API", got)
		}
	})

	t.Run("user_reels_maps_username_to_api_field", func(t *testing.T) {
		res, err := DispatchYepAPIInstagram(ctx, client, "user_reels", map[string]interface{}{"username": "natgeo"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/instagram/user-reels" || got.Payload["username"] != "natgeo" {
			t.Fatalf("request = %+v, want user-reels endpoint with username", got)
		}
		if _, ok := got.Payload["username_or_url"]; ok {
			t.Fatalf("request = %+v, did not expect username_or_url field to be sent to API", got)
		}
	})
}

func TestYepAPIFormatSuccessPromotesEmbeddedErrors(t *testing.T) {
	res := yepAPIFormatSuccess(json.RawMessage(`{"error":"username_or_url is required"}`))
	if !strings.Contains(res, `"status":"error"`) {
		t.Fatalf("expected embedded data.error to become tool error, got %s", res)
	}
	if !strings.Contains(res, "username_or_url is required") {
		t.Fatalf("expected original error message, got %s", res)
	}
}

func TestDispatchYepAPIAmazon(t *testing.T) {
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		return http.StatusOK, `{"ok":true,"data":{}}`
	})

	ctx := context.Background()

	t.Run("search", func(t *testing.T) {
		res, err := DispatchYepAPIAmazon(ctx, client, "search", map[string]interface{}{"query": "phone"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("product", func(t *testing.T) {
		res, err := DispatchYepAPIAmazon(ctx, client, "product", map[string]interface{}{"asin": "B07ZPKBL9V"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("reviews_with_country", func(t *testing.T) {
		res, err := DispatchYepAPIAmazon(ctx, client, "reviews", map[string]interface{}{"asin": "B07ZPKBL9V", "country": "DE"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("deals_default_country", func(t *testing.T) {
		res, err := DispatchYepAPIAmazon(ctx, client, "deals", map[string]interface{}{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})

	t.Run("best_sellers_with_category", func(t *testing.T) {
		res, err := DispatchYepAPIAmazon(ctx, client, "best_sellers", map[string]interface{}{"category": "electronics", "country": "UK"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
	})
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

	t.Run("explicit_provider", func(t *testing.T) {
		cfg := &config.Config{
			YepAPI: config.YepAPIConfig{Provider: "my_yep"},
			Providers: []config.ProviderEntry{
				{ID: "other", Type: "yepapi", APIKey: "other_key"},
				{ID: "my_yep", Type: "yepapi", APIKey: "explicit_key"},
			},
		}
		vault := &mockSecretReader{secrets: map[string]string{}}
		key, err := ResolveYepAPIKey(cfg, vault)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "explicit_key" {
			t.Fatalf("expected 'explicit_key', got %q", key)
		}
	})

	t.Run("explicit_provider_wrong_type_rejected", func(t *testing.T) {
		cfg := &config.Config{
			YepAPI: config.YepAPIConfig{Provider: "main_llm"},
			Providers: []config.ProviderEntry{
				{ID: "main_llm", Type: "openrouter", APIKey: "llm_key"},
			},
		}
		vault := &mockSecretReader{secrets: map[string]string{}}
		_, err := ResolveYepAPIKey(cfg, vault)
		if err == nil {
			t.Fatal("expected error when explicit provider is not type yepapi")
		}
		if !strings.Contains(err.Error(), "expected provider type 'yepapi'") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("explicit_provider_from_vault", func(t *testing.T) {
		cfg := &config.Config{
			YepAPI: config.YepAPIConfig{Provider: "my_yep"},
			Providers: []config.ProviderEntry{
				{ID: "my_yep", Type: "yepapi", APIKey: ""},
			},
		}
		vault := &mockSecretReader{secrets: map[string]string{"provider_my_yep_api_key": "vault_prov_key"}}
		key, err := ResolveYepAPIKey(cfg, vault)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "vault_prov_key" {
			t.Fatalf("expected 'vault_prov_key', got %q", key)
		}
	})

	t.Run("nil_vault_provider_plaintext_key", func(t *testing.T) {
		cfg := &config.Config{
			Providers: []config.ProviderEntry{
				{ID: "yepapi_main", Type: "yepapi", APIKey: "provider_key"},
			},
		}
		key, err := ResolveYepAPIKey(cfg, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "provider_key" {
			t.Fatalf("expected 'provider_key', got %q", key)
		}
	})

	t.Run("nil_vault_no_key_errors_without_panic", func(t *testing.T) {
		cfg := &config.Config{}
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ResolveYepAPIKey should not panic with nil vault: %v", r)
			}
		}()
		_, err := ResolveYepAPIKey(cfg, nil)
		if err == nil {
			t.Fatal("expected error when key is not found")
		}
	})

	t.Run("explicit_provider_not_found", func(t *testing.T) {
		cfg := &config.Config{
			YepAPI:    config.YepAPIConfig{Provider: "missing"},
			Providers: []config.ProviderEntry{},
		}
		vault := &mockSecretReader{secrets: map[string]string{}}
		_, err := ResolveYepAPIKey(cfg, vault)
		if err == nil {
			t.Fatal("expected error when explicit provider not found")
		}
	})

	t.Run("explicit_provider_no_key", func(t *testing.T) {
		cfg := &config.Config{
			YepAPI: config.YepAPIConfig{Provider: "my_yep"},
			Providers: []config.ProviderEntry{
				{ID: "my_yep", Type: "yepapi", APIKey: ""},
			},
		}
		vault := &mockSecretReader{secrets: map[string]string{}}
		_, err := ResolveYepAPIKey(cfg, vault)
		if err == nil {
			t.Fatal("expected error when explicit provider has no key")
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
