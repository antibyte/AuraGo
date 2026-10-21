package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"
)

// useNetlifyTestBase points the Netlify API at a test server for one test.
func useNetlifyTestBase(t *testing.T, baseURL string) {
	t.Helper()
	previous := netlifyBaseURL
	netlifyBaseURL = baseURL
	t.Cleanup(func() { netlifyBaseURL = previous })
}

func decodeNetlifyJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v (%q)", err, raw)
	}
	return parsed
}

func TestNetlifyClientRefusesCrossOriginRedirect(t *testing.T) {
	var leaked atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer foreign.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/steal", http.StatusFound)
	}))
	defer api.Close()
	useNetlifyTestBase(t, api.URL)

	if _, _, err := netlifyRequest(NetlifyConfig{Token: "token"}, http.MethodGet, "/sites", nil); err == nil {
		t.Fatal("expected the same-origin redirect policy to refuse a foreign target")
	}
	if got := leaked.Load(); got != 0 {
		t.Fatalf("the bearer token was sent to a foreign origin (%d requests)", got)
	}
}

func TestNetlifyErrorEnvelopeIsAlwaysValidJSON(t *testing.T) {
	cases := []struct {
		name string
		code int
		body []byte
	}{
		{"control characters", http.StatusBadGateway, []byte("bad\x07byte\x0bhere")},
		{"invalid utf-8", http.StatusBadGateway, []byte{'x', 0xff, 'y'}},
		{"html page", http.StatusBadGateway, []byte("<html>" + strings.Repeat("a", 4000) + "</html>")},
		{"json message", http.StatusNotFound, []byte(`{"message":"Site not found"}`)},
		{"json errors", http.StatusUnprocessableEntity, []byte(`{"errors":[{"title":"Key is invalid"}]}`)},
		{"empty body", http.StatusInternalServerError, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := netlifyHTTPError(tc.code, tc.body)
			parsed := decodeNetlifyJSON(t, raw)
			if parsed["status"] != "error" {
				t.Fatalf("status = %v, want error; raw=%s", parsed["status"], raw)
			}
			if parsed["http_code"] != float64(tc.code) {
				t.Fatalf("http_code = %v, want %d; raw=%s", parsed["http_code"], tc.code, raw)
			}
			message, _ := parsed["message"].(string)
			if message == "" {
				t.Fatalf("message is empty; raw=%s", raw)
			}
			if !utf8.ValidString(message) {
				t.Fatalf("message is not valid UTF-8: %q", message)
			}
			if runes := utf8.RuneCountInString(message); runes > netlifyErrorTextMaxRunes+1 {
				t.Fatalf("message has %d runes, want at most %d", runes, netlifyErrorTextMaxRunes+1)
			}
		})
	}
}

func TestNetlifyNextPageLinkParsesHeader(t *testing.T) {
	header := `<https://api.netlify.com/api/v1/sites?page=1>; rel="prev", <https://api.netlify.com/api/v1/sites?page=3>; rel="next"`
	if got := netlifyNextPageLink(header); got != "https://api.netlify.com/api/v1/sites?page=3" {
		t.Fatalf("next link = %q", got)
	}
	if got := netlifyNextPageLink(`<https://api.netlify.com/api/v1/sites?page=1>; rel="last"`); got != "" {
		t.Fatalf("expected no next link, got %q", got)
	}
	if got := netlifyNextPageLink(""); got != "" {
		t.Fatalf("expected no next link for an empty header, got %q", got)
	}
}

func TestNetlifyListSitesFollowsLinkHeader(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "" {
			w.Header().Set("Link", "<"+server.URL+"/sites?page=2&per_page=100>; rel=\"next\"")
			_, _ = w.Write([]byte(`[{"id":"site-1","name":"one"}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"site-2","name":"two"}]`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	result := NetlifyListSites(NetlifyConfig{Token: "token"})
	parsed := decodeNetlifyJSON(t, result)
	if parsed["status"] != "ok" {
		t.Fatalf("expected ok, got %s", result)
	}
	if parsed["count"] != float64(2) {
		t.Fatalf("count = %v, want 2 (both pages); result=%s", parsed["count"], result)
	}
	if _, truncated := parsed["truncated"]; truncated {
		t.Fatalf("a complete listing must not be marked truncated: %s", result)
	}
}

func TestNetlifyListSitesRefusesForeignNextLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", `<https://evil.example/api/v1/sites?page=2>; rel="next"`)
		_, _ = w.Write([]byte(`[{"id":"site-1","name":"one"}]`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	parsed := decodeNetlifyJSON(t, NetlifyListSites(NetlifyConfig{Token: "token"}))
	if parsed["count"] != float64(1) {
		t.Fatalf("count = %v, want 1", parsed["count"])
	}
	if parsed["truncated"] != true {
		t.Fatalf("a refused next link must mark the listing truncated")
	}
}

func TestNetlifyGetPagesStopsAtCap(t *testing.T) {
	var requests atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Link", "<"+server.URL+"/sites?page=2>; rel=\"next\"")
		_, _ = w.Write([]byte(`[{"id":"site"}]`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	page := netlifyGetPages(NetlifyConfig{Token: "token"}, "/sites", 2)
	if page.err != nil || page.code != http.StatusOK {
		t.Fatalf("unexpected page result: err=%v code=%d", page.err, page.code)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requested %d pages, want 2", got)
	}
	if len(page.items) != 2 || !page.truncated {
		t.Fatalf("items=%d truncated=%v, want 2 and true", len(page.items), page.truncated)
	}
}

func TestNetlifyRequestRetriesGetOn429(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	if _, code, err := netlifyRequest(NetlifyConfig{Token: "token"}, http.MethodGet, "/sites", nil); err != nil || code != http.StatusOK {
		t.Fatalf("code=%d err=%v, want 200 after one retry", code, err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("made %d requests, want 2", got)
	}
}

func TestNetlifyRequestNeverRetriesMutations(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"temporarily unavailable"}`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	if _, code, err := netlifyRequest(NetlifyConfig{Token: "token"}, http.MethodPost, "/sites", map[string]interface{}{"name": "x"}); err != nil || code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d err=%v, want the provider answer", code, err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("mutation was sent %d times, want exactly 1", got)
	}
}

func TestNetlifyDeployZipDoesNotRetryServerErrors(t *testing.T) {
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	result := NetlifyDeployZip(NetlifyConfig{Token: "token", AllowDeploy: true}, "site-1", "", false, []byte("zip"))
	if got := posts.Load(); got != 1 {
		t.Fatalf("deploy upload was sent %d times, want exactly 1", got)
	}
	parsed := decodeNetlifyJSON(t, result)
	if parsed["http_code"] != float64(http.StatusInternalServerError) || parsed["message"] != "boom" {
		t.Fatalf("unexpected error envelope: %s", result)
	}
}

func TestNetlifyEnvVarsResolveSingleAccount(t *testing.T) {
	var usedAccountEndpoint atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/accounts":
			_, _ = w.Write([]byte(`[{"slug":"team-one"}]`))
		case "/accounts/team-one/env":
			usedAccountEndpoint.Store(true)
			_, _ = w.Write([]byte(`[{"key":"API_TOKEN","scopes":["builds"],"values":[{"context":"all"}]}]`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	result := NetlifyListEnvVars(NetlifyConfig{Token: "token", DefaultSiteID: "site-1"}, "")
	if !usedAccountEndpoint.Load() {
		t.Fatalf("expected the account env endpoint, got %s", result)
	}
	parsed := decodeNetlifyJSON(t, result)
	if parsed["status"] != "ok" || parsed["count"] != float64(1) {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestNetlifyEnvVarsReportAmbiguousAccounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"slug":"team-one"},{"slug":"team-two"}]`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	result := NetlifyListEnvVars(NetlifyConfig{Token: "token", DefaultSiteID: "site-1"}, "")
	parsed := decodeNetlifyJSON(t, result)
	message, _ := parsed["message"].(string)
	if parsed["status"] != "error" || !strings.Contains(message, "team_slug") {
		t.Fatalf("expected a team_slug hint, got %s", result)
	}
}

func TestNetlifySetEnvVarDoesNotRetryOnAuthError(t *testing.T) {
	var posts, patches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			posts.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"invalid token"}`))
		case http.MethodPatch:
			patches.Add(1)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	cfg := NetlifyConfig{Token: "token", TeamSlug: "team-one", AllowEnvManagement: true}
	result := NetlifySetEnvVar(cfg, "site-1", "KEY", "value", "")
	if posts.Load() != 1 || patches.Load() != 0 {
		t.Fatalf("posts=%d patches=%d, want 1 and 0", posts.Load(), patches.Load())
	}
	parsed := decodeNetlifyJSON(t, result)
	if parsed["http_code"] != float64(http.StatusUnauthorized) || parsed["message"] != "invalid token" {
		t.Fatalf("the original error must survive: %s", result)
	}
}

func TestNetlifySetEnvVarUpdatesExistingVariable(t *testing.T) {
	var patches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"key already exists"}`))
		case http.MethodPatch:
			patches.Add(1)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	cfg := NetlifyConfig{Token: "token", TeamSlug: "team-one", AllowEnvManagement: true}
	parsed := decodeNetlifyJSON(t, NetlifySetEnvVar(cfg, "site-1", "KEY", "value", ""))
	if patches.Load() != 1 || parsed["status"] != "ok" {
		t.Fatalf("patches=%d result=%v, want one single-key update", patches.Load(), parsed["status"])
	}
}

func TestNetlifyTestConnectionUsesConfiguredBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Fatalf("unexpected request %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"email":"me@example.test","slug":"me"}`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	parsed := decodeNetlifyJSON(t, NetlifyTestConnection(NetlifyConfig{Token: "token"}))
	if parsed["status"] != "ok" {
		t.Fatalf("expected ok, got %v", parsed)
	}
	if parsed["account_email"] != "me@example.test" || parsed["account_slug"] != "me" {
		t.Fatalf("unexpected account info: %v", parsed)
	}
	if parsed["dns_ok"] != true || parsed["tcp_ok"] != true || parsed["api_ok"] != true {
		t.Fatalf("expected all probes to pass: %v", parsed)
	}
}

func TestNetlifyTestConnectionHonoursCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	useNetlifyTestBase(t, "http://127.0.0.1:9")

	parsed := decodeNetlifyJSON(t, NetlifyTestConnection(NetlifyConfig{Token: "token", Context: ctx}))
	if parsed["status"] != "error" {
		t.Fatalf("a cancelled context must not report a healthy connection: %v", parsed)
	}
	if parsed["tcp_ok"] != false {
		t.Fatalf("tcp_ok = %v, want false", parsed["tcp_ok"])
	}
}

func TestNetlifyResolveNameToIDUsesPages(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "" {
			w.Header().Set("Link", "<"+server.URL+"/sites?page=2&per_page=100>; rel=\"next\"")
			_, _ = w.Write([]byte(`[{"id":"id-a","name":"site-a"}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"id-b","name":"site-b","custom_domain":"www.example.test"}]`))
	}))
	defer server.Close()
	useNetlifyTestBase(t, server.URL)

	if got := netlifyResolveNameToID(NetlifyConfig{Token: "token"}, "site-b"); got != "id-b" {
		t.Fatalf("resolved %q, want id-b (second page)", got)
	}
	if got := netlifyResolveNameToID(NetlifyConfig{Token: "token"}, "www.example.test"); got != "id-b" {
		t.Fatalf("resolved %q by custom domain, want id-b", got)
	}
}
