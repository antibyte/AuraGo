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

func TestHandleComposioToolsPreviewEnablesViewedToolkitOnlyForPolicyPreview(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tools" {
			t.Fatalf("path = %s, want /tools", r.URL.Path)
		}
		if r.URL.Query().Get("toolkit_slug") != "github" {
			t.Fatalf("toolkit_slug = %q, want github", r.URL.Query().Get("toolkit_slug"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"slug":"GITHUB_GET_REPOSITORY","description":"Read a repository","toolkit":{"slug":"github"}},
			{"slug":"GITHUB_CREATE_ISSUE","description":"Create an issue","toolkit":{"slug":"github"}}
		]}`))
	}))
	defer upstream.Close()

	s := &Server{Cfg: &config.Config{
		Composio: config.ComposioConfig{
			Enabled:               false,
			APIKey:                "cmp-secret",
			BaseURL:               upstream.URL,
			ReadOnly:              true,
			RequestTimeoutSeconds: 2,
			MaxResultBytes:        4096,
		},
	}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/composio/tools?toolkit_slug=github&limit=20&preview=1", nil)
	handleComposioTools(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Items []struct {
			PolicyDecision struct {
				Allowed bool   `json:"allowed"`
				Reason  string `json:"reason"`
			} `json:"policy_decision"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("item count = %d, want 2", len(got.Items))
	}
	if !got.Items[0].PolicyDecision.Allowed {
		t.Fatalf("read tool should be allowed in preview, got %+v", got.Items[0].PolicyDecision)
	}
	if got.Items[1].PolicyDecision.Allowed || got.Items[1].PolicyDecision.Reason == "" {
		t.Fatalf("write tool should stay blocked by read-only preview policy, got %+v", got.Items[1].PolicyDecision)
	}
}
