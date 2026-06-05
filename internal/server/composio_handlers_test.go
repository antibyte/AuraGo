package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestHandleComposioStatusMasksConfiguration(t *testing.T) {
	s := &Server{Cfg: &config.Config{
		Composio: config.ComposioConfig{
			Enabled: true,
			APIKey:  "cmp-secret",
			Toolkits: []config.ComposioToolkitConfig{
				{Slug: "github", Enabled: true},
				{Slug: "gmail", Enabled: false},
			},
		},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/composio/status", nil)
	handleComposioStatus(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["status"] != "ready" || got["configured"] != true {
		t.Fatalf("unexpected status payload: %#v", got)
	}
	if got["selected_count"].(float64) != 1 {
		t.Fatalf("selected_count = %#v, want 1", got["selected_count"])
	}
	if _, leaked := got["api_key"]; leaked {
		t.Fatalf("status leaked api_key: %#v", got)
	}
}

func TestHandleComposioToolkitsUsesAPIKeyEvenWhenFeatureDisabled(t *testing.T) {
	var sawAPIKey bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/toolkits" {
			t.Fatalf("path = %s, want /toolkits", r.URL.Path)
		}
		sawAPIKey = r.Header.Get("x-api-key") == "cmp-secret"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"slug":"github","name":"GitHub"}],"next_cursor":"next"}`))
	}))
	defer upstream.Close()

	s := &Server{Cfg: &config.Config{
		Composio: config.ComposioConfig{
			Enabled:               false,
			APIKey:                "cmp-secret",
			BaseURL:               upstream.URL,
			RequestTimeoutSeconds: 2,
			MaxResultBytes:        4096,
		},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/composio/toolkits?limit=5", nil)
	handleComposioToolkits(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !sawAPIKey {
		t.Fatal("upstream did not receive x-api-key")
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items := got["items"].([]interface{})
	if len(items) != 1 || got["next_cursor"] != "next" {
		t.Fatalf("unexpected toolkits response: %#v", got)
	}
}
