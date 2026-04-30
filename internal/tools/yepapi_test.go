package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

type yepAPIRecordedRequest struct {
	Path    string
	Payload map[string]interface{}
}

func newRecordingYepAPITestClient(t *testing.T, capacity int) (*YepAPIClient, <-chan yepAPIRecordedRequest) {
	t.Helper()
	requests := make(chan yepAPIRecordedRequest, capacity)
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		var payload map[string]interface{}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&payload)
		}
		requests <- yepAPIRecordedRequest{Path: r.URL.Path, Payload: payload}
		return http.StatusOK, `{"ok":true,"data":{"ok":true}}`
	})
	return client, requests
}

func requireYepAPIRequest(t *testing.T, requests <-chan yepAPIRecordedRequest, wantPath string, wantPayload map[string]interface{}) {
	t.Helper()
	req := <-requests
	if req.Path != wantPath {
		t.Fatalf("expected path %s, got %s", wantPath, req.Path)
	}
	for key, want := range wantPayload {
		if got := req.Payload[key]; got != want {
			t.Fatalf("expected payload[%s]=%v, got %v (payload=%v)", key, want, got, req.Payload)
		}
	}
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

func TestYepAPIClientPostLogsMarshaledBodyKeys(t *testing.T) {
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		return http.StatusOK, `{"ok":true,"data":{}}`
	})

	ctx := WithYepAPILogger(context.Background(), logger)
	_, err := client.Post(ctx, "/v1/instagram/user", map[string]interface{}{
		"username":        "natgeo",
		"username_or_url": "natgeo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := logs.String()
	if !strings.Contains(got, "[YepAPI] Marshaled request body") {
		t.Fatalf("logs = %q, want marshaled body diagnostic", got)
	}
	if !strings.Contains(got, "/v1/instagram/user") {
		t.Fatalf("logs = %q, want endpoint", got)
	}
	if !strings.Contains(got, "username") || !strings.Contains(got, "username_or_url") {
		t.Fatalf("logs = %q, want marshaled body keys", got)
	}
	if strings.Contains(got, "natgeo") {
		t.Fatalf("logs = %q, should not contain raw body values", got)
	}
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
			t.Fatalf("request = %+v, username_or_url should only be used for fallback", got)
		}
	})

	t.Run("user_accepts_username_or_url_alias_and_normalizes_profile_url", func(t *testing.T) {
		res, err := DispatchYepAPIInstagram(ctx, client, "user", map[string]interface{}{"username_or_url": "https://www.instagram.com/natgeo/"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res == "" {
			t.Fatal("expected non-empty result")
		}
		got := <-requests
		if got.Path != "/v1/instagram/user" || got.Payload["username"] != "natgeo" {
			t.Fatalf("request = %+v, want user endpoint with normalized username", got)
		}
		if _, ok := got.Payload["username_or_url"]; ok {
			t.Fatalf("request = %+v, username_or_url should only be used for fallback", got)
		}
	})

	t.Run("userinfo_alias_maps_to_user_endpoint", func(t *testing.T) {
		res, err := DispatchYepAPIInstagram(ctx, client, "userinfo", map[string]interface{}{"username": "natgeo"})
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
			t.Fatalf("request = %+v, username_or_url should only be used for fallback", got)
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
			t.Fatalf("request = %+v, username_or_url should only be used for fallback", got)
		}
	})
}

func TestDispatchYepAPIInstagramRetriesLiveValidationFallbacks(t *testing.T) {
	ctx := context.Background()

	t.Run("user_sends_canonical_username_only_and_accepts_url_alias", func(t *testing.T) {
		requests := make(chan yepAPIRecordedRequest, 1)
		client := newYepAPITestClient(func(r *http.Request) (int, string) {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			requests <- yepAPIRecordedRequest{Path: r.URL.Path, Payload: payload}
			return http.StatusOK, `{"ok":true,"data":{"username":"natgeo"}}`
		})

		res, err := DispatchYepAPIInstagram(ctx, client, "user", map[string]interface{}{"username_or_url": "https://www.instagram.com/natgeo/"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, `"status":"success"`) {
			t.Fatalf("expected success, got %s", res)
		}
		requireYepAPIRequest(t, requests, "/v1/instagram/user", map[string]interface{}{"username": "natgeo"})
		select {
		case extra := <-requests:
			t.Fatalf("unexpected second request: %+v", extra)
		default:
		}
	})

	t.Run("search_accepts_search_query_alias_and_sends_canonical_query", func(t *testing.T) {
		requests := make(chan yepAPIRecordedRequest, 1)
		client := newYepAPITestClient(func(r *http.Request) (int, string) {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			requests <- yepAPIRecordedRequest{Path: r.URL.Path, Payload: payload}
			return http.StatusOK, `{"ok":true,"data":{"users":[]}}`
		})

		res, err := DispatchYepAPIInstagram(ctx, client, "search", map[string]interface{}{"search_query": "natgeo"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res, `"status":"success"`) {
			t.Fatalf("expected success, got %s", res)
		}
		requireYepAPIRequest(t, requests, "/v1/instagram/search", map[string]interface{}{"query": "natgeo"})
	})
}

func TestDispatchYepAPIInstagramLogsPayloadShape(t *testing.T) {
	var logs strings.Builder
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	requests := make(chan yepAPIRecordedRequest, 1)
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests <- yepAPIRecordedRequest{Path: r.URL.Path, Payload: payload}
		return http.StatusOK, `{"ok":true,"data":{}}`
	})

	ctx := WithYepAPILogger(context.Background(), logger)
	if _, err := DispatchYepAPIInstagram(ctx, client, "user", map[string]interface{}{"username": "natgeo"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-requests

	got := logs.String()
	if !strings.Contains(got, "[YepAPI] Prepared request payload") {
		t.Fatalf("logs = %q, want payload diagnostic", got)
	}
	if !strings.Contains(got, "/v1/instagram/user") {
		t.Fatalf("logs = %q, want endpoint", got)
	}
	if !strings.Contains(got, "username") {
		t.Fatalf("logs = %q, want username payload key", got)
	}
	if strings.Contains(got, "username_or_url") {
		t.Fatalf("logs = %q, username_or_url should only appear during fallback", got)
	}
	if strings.Contains(got, "natgeo") {
		t.Fatalf("logs = %q, should not contain raw payload value", got)
	}
}

func TestDispatchYepAPIInstagramErrorIncludesSentPayloadKeys(t *testing.T) {
	client := newYepAPITestClient(func(r *http.Request) (int, string) {
		return http.StatusOK, `{"ok":true,"data":{"error":"profile unavailable"}}`
	})

	res, err := DispatchYepAPIInstagram(context.Background(), client, "user", map[string]interface{}{"username": "natgeo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(res), &payload); err != nil {
		t.Fatalf("decode result %q: %v", res, err)
	}
	if payload["status"] != "error" {
		t.Fatalf("status = %v, want error", payload["status"])
	}
	keys, ok := payload["sent_payload_keys"].([]interface{})
	if !ok {
		t.Fatalf("sent_payload_keys missing from %v", payload)
	}
	gotKeys := fmt.Sprint(keys)
	if !strings.Contains(gotKeys, "username") {
		t.Fatalf("sent_payload_keys = %v, want username", keys)
	}
	if strings.Contains(gotKeys, "username_or_url") {
		t.Fatalf("sent_payload_keys = %v, username_or_url should only appear during fallback", keys)
	}
	if fmt.Sprint(payload) == "" || strings.Contains(fmt.Sprint(payload), "natgeo") {
		t.Fatalf("diagnostic payload should not include raw values: %v", payload)
	}
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

func TestDispatchYepAPIScrapeParityEndpoints(t *testing.T) {
	client, requests := newRecordingYepAPITestClient(t, 2)
	ctx := context.Background()

	if _, err := DispatchYepAPIScrape(ctx, client, "extract", map[string]interface{}{"url": "https://example.com", "selector": "h1"}); err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	requireYepAPIRequest(t, requests, "/v1/scrape/extract", map[string]interface{}{"url": "https://example.com", "selector": "h1"})

	if _, err := DispatchYepAPIScrape(ctx, client, "search_google", map[string]interface{}{"query": "aurago", "limit": 3.0}); err != nil {
		t.Fatalf("search_google failed: %v", err)
	}
	requireYepAPIRequest(t, requests, "/v1/search/google", map[string]interface{}{"query": "aurago", "limit": 3.0})
}

func TestDispatchYepAPIYouTubeParityEndpoints(t *testing.T) {
	client, requests := newRecordingYepAPITestClient(t, 22)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]interface{}
		path    string
		payload map[string]interface{}
	}{
		{"video_info", map[string]interface{}{"video_id": "vid"}, "/v1/youtube/video-info", map[string]interface{}{"videoId": "vid"}},
		{"metadata", map[string]interface{}{"video_id": "vid"}, "/v1/youtube/metadata", map[string]interface{}{"videoId": "vid"}},
		{"subtitles", map[string]interface{}{"video_id": "vid"}, "/v1/youtube/subtitles", map[string]interface{}{"videoId": "vid"}},
		{"channel_shorts", map[string]interface{}{"channel_id": "chan", "limit": 4.0}, "/v1/youtube/channel-shorts", map[string]interface{}{"channelId": "chan", "limit": 4.0}},
		{"channel_livestreams", map[string]interface{}{"channel_id": "chan"}, "/v1/youtube/channel-livestreams", map[string]interface{}{"channelId": "chan"}},
		{"channel_playlists", map[string]interface{}{"channel_id": "chan"}, "/v1/youtube/channel-playlists", map[string]interface{}{"channelId": "chan"}},
		{"channel_community", map[string]interface{}{"channel_id": "chan"}, "/v1/youtube/channel-community", map[string]interface{}{"channelId": "chan"}},
		{"channel_about", map[string]interface{}{"channel_id": "chan"}, "/v1/youtube/channel-about", map[string]interface{}{"channelId": "chan"}},
		{"channel_channels", map[string]interface{}{"channel_id": "chan"}, "/v1/youtube/channel-channels", map[string]interface{}{"channelId": "chan"}},
		{"channel_store", map[string]interface{}{"channel_id": "chan"}, "/v1/youtube/channel-store", map[string]interface{}{"channelId": "chan"}},
		{"channel_search", map[string]interface{}{"channel_id": "chan", "query": "go"}, "/v1/youtube/channel-search", map[string]interface{}{"channelId": "chan", "query": "go"}},
		{"playlist_info", map[string]interface{}{"playlist_id": "pl"}, "/v1/youtube/playlist-info", map[string]interface{}{"playlistId": "pl"}},
		{"related", map[string]interface{}{"video_id": "vid", "limit": 2.0}, "/v1/youtube/related", map[string]interface{}{"videoId": "vid", "limit": 2.0}},
		{"screenshot", map[string]interface{}{"video_id": "vid"}, "/v1/youtube/screenshot", map[string]interface{}{"videoId": "vid"}},
		{"shorts_info", map[string]interface{}{"video_id": "vid"}, "/v1/youtube/shorts-info", map[string]interface{}{"videoId": "vid"}},
		{"hashtag", map[string]interface{}{"tag": "golang"}, "/v1/youtube/hashtag", map[string]interface{}{"tag": "golang"}},
		{"post", map[string]interface{}{"post_id": "post1"}, "/v1/youtube/post", map[string]interface{}{"postId": "post1"}},
		{"post_comments", map[string]interface{}{"post_id": "post1"}, "/v1/youtube/post-comments", map[string]interface{}{"postId": "post1"}},
		{"home", map[string]interface{}{"country": "DE"}, "/v1/youtube/home", map[string]interface{}{"country": "DE"}},
		{"hype", map[string]interface{}{"language": "de"}, "/v1/youtube/hype", map[string]interface{}{"language": "de"}},
		{"resolve", map[string]interface{}{"url": "https://youtu.be/vid"}, "/v1/youtube/resolve", map[string]interface{}{"url": "https://youtu.be/vid"}},
	}

	for _, tt := range tests {
		if _, err := DispatchYepAPIYouTube(ctx, client, tt.name, tt.args); err != nil {
			t.Fatalf("%s failed: %v", tt.name, err)
		}
		requireYepAPIRequest(t, requests, tt.path, tt.payload)
	}
}

func TestDispatchYepAPITikTokParityEndpoints(t *testing.T) {
	client, requests := newRecordingYepAPITestClient(t, 12)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]interface{}
		path    string
		payload map[string]interface{}
	}{
		{"search_challenge", map[string]interface{}{"query": "dance"}, "/v1/tiktok/search-challenge", map[string]interface{}{"keyword": "dance"}},
		{"search_photo", map[string]interface{}{"query": "food"}, "/v1/tiktok/search-photo", map[string]interface{}{"keywords": "food"}},
		{"user_followers", map[string]interface{}{"username": "user", "limit": 5.0}, "/v1/tiktok/user-followers", map[string]interface{}{"unique_id": "user", "count": 5.0}},
		{"user_following", map[string]interface{}{"username": "user"}, "/v1/tiktok/user-following", map[string]interface{}{"unique_id": "user"}},
		{"user_favorites", map[string]interface{}{"username": "user"}, "/v1/tiktok/user-favorites", map[string]interface{}{"unique_id": "user"}},
		{"user_reposts", map[string]interface{}{"username": "user"}, "/v1/tiktok/user-reposts", map[string]interface{}{"unique_id": "user"}},
		{"user_story", map[string]interface{}{"username": "user"}, "/v1/tiktok/user-story", map[string]interface{}{"unique_id": "user"}},
		{"comment_replies", map[string]interface{}{"comment_id": "123", "url": "https://tiktok.example/v/1"}, "/v1/tiktok/comment-replies", map[string]interface{}{"comment_id": "123", "url": "https://tiktok.example/v/1"}},
		{"music_videos", map[string]interface{}{"url": "https://tiktok.example/music/1"}, "/v1/tiktok/music-videos", map[string]interface{}{"url": "https://tiktok.example/music/1"}},
		{"challenge_videos", map[string]interface{}{"name": "dance"}, "/v1/tiktok/challenge-videos", map[string]interface{}{"name": "dance"}},
	}

	for _, tt := range tests {
		if _, err := DispatchYepAPITikTok(ctx, client, tt.name, tt.args); err != nil {
			t.Fatalf("%s failed: %v", tt.name, err)
		}
		requireYepAPIRequest(t, requests, tt.path, tt.payload)
	}
}

func TestDispatchYepAPIInstagramParityEndpoints(t *testing.T) {
	client, requests := newRecordingYepAPITestClient(t, 10)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]interface{}
		path    string
		payload map[string]interface{}
	}{
		{"user_about", map[string]interface{}{"username": "natgeo"}, "/v1/instagram/user-about", map[string]interface{}{"username": "natgeo"}},
		{"user_stories", map[string]interface{}{"username": "natgeo"}, "/v1/instagram/user-stories", map[string]interface{}{"username": "natgeo"}},
		{"user_highlights", map[string]interface{}{"username": "natgeo"}, "/v1/instagram/user-highlights", map[string]interface{}{"username": "natgeo"}},
		{"user_tagged", map[string]interface{}{"username": "natgeo", "limit": 4.0}, "/v1/instagram/user-tagged", map[string]interface{}{"username": "natgeo", "limit": 4.0}},
		{"user_followers", map[string]interface{}{"username": "natgeo"}, "/v1/instagram/user-followers", map[string]interface{}{"username": "natgeo"}},
		{"user_similar", map[string]interface{}{"username": "natgeo"}, "/v1/instagram/user-similar", map[string]interface{}{"username": "natgeo"}},
		{"post_likers", map[string]interface{}{"shortcode": "abc"}, "/v1/instagram/post-likers", map[string]interface{}{"shortcode": "abc"}},
		{"media_id", map[string]interface{}{"shortcode": "abc"}, "/v1/instagram/media-id", map[string]interface{}{"shortcode": "abc"}},
	}

	for _, tt := range tests {
		if _, err := DispatchYepAPIInstagram(ctx, client, tt.name, tt.args); err != nil {
			t.Fatalf("%s failed: %v", tt.name, err)
		}
		requireYepAPIRequest(t, requests, tt.path, tt.payload)
	}
}

func TestDispatchYepAPIAmazonParityEndpoints(t *testing.T) {
	client, requests := newRecordingYepAPITestClient(t, 8)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]interface{}
		path    string
		payload map[string]interface{}
	}{
		{"product_offers", map[string]interface{}{"asin": "B000", "country": "DE", "page": 2.0}, "/v1/amazon/product-offers", map[string]interface{}{"asin": "B000", "country": "DE", "page": 2.0}},
		{"products_by_category", map[string]interface{}{"category": "electronics"}, "/v1/amazon/products-by-category", map[string]interface{}{"category": "electronics", "country": "US"}},
		{"categories", map[string]interface{}{"country": "UK"}, "/v1/amazon/categories", map[string]interface{}{"country": "UK"}},
		{"influencer", map[string]interface{}{"handle": "creator"}, "/v1/amazon/influencer", map[string]interface{}{"handle": "creator", "country": "US"}},
		{"seller", map[string]interface{}{"seller_id": "SELLER"}, "/v1/amazon/seller", map[string]interface{}{"seller_id": "SELLER", "country": "US"}},
		{"seller_reviews", map[string]interface{}{"seller_id": "SELLER", "limit": 3.0}, "/v1/amazon/seller-reviews", map[string]interface{}{"seller_id": "SELLER", "country": "US", "limit": 3.0}},
	}

	for _, tt := range tests {
		if _, err := DispatchYepAPIAmazon(ctx, client, tt.name, tt.args); err != nil {
			t.Fatalf("%s failed: %v", tt.name, err)
		}
		requireYepAPIRequest(t, requests, tt.path, tt.payload)
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
