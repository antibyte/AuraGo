package evomap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientStatusFetchesStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/a2a/stats" {
			t.Fatalf("request = %s %s, want GET /a2a/stats", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"nodes":3,"capsules":9}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	status, err := client.Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Status != "ok" {
		t.Fatalf("status = %q, want ok", status.Status)
	}
	if !strings.Contains(string(status.Raw), `"capsules":9`) {
		t.Fatalf("raw stats missing server payload: %s", status.Raw)
	}
}

func TestClientRegisterNodePostsHelloEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/a2a/hello" {
			t.Fatalf("request = %s %s, want POST /a2a/hello", r.Method, r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["protocol"] != ProtocolName || payload["version"] != ProtocolVersion {
			t.Fatalf("protocol envelope = %#v", payload)
		}
		if payload["node_id"] != "node-existing" {
			t.Fatalf("node_id = %#v, want node-existing", payload["node_id"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"node_id":"node-new","node_secret":"secret-new","claim_url":"https://evomap.ai/claim/node-new"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, NodeID: "node-existing", HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.RegisterNode(context.Background(), RegisterRequest{Capabilities: []string{"fetch"}})
	if err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}
	if resp.NodeID != "node-new" || resp.NodeSecret != "secret-new" || resp.ClaimURL == "" {
		t.Fatalf("unexpected registration response: %+v", resp)
	}
}

func TestClientFetchCapsulesUsesExternalDataQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/a2a/fetch" {
			t.Fatalf("request = %s %s, want POST /a2a/fetch", r.Method, r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["node_secret"] != "node-secret" {
			t.Fatalf("node_secret missing from authenticated fetch envelope: %#v", payload)
		}
		if payload["problem"] != "repair recurring error" {
			t.Fatalf("problem = %#v", payload["problem"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"capsules":[{"id":"cap-1","summary":"do not execute me"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, NodeID: "node-1", NodeSecret: "node-secret", HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	resp, err := client.FetchCapsules(context.Background(), FetchRequest{Problem: "repair recurring error", Limit: 1})
	if err != nil {
		t.Fatalf("FetchCapsules() error = %v", err)
	}
	if !strings.Contains(string(resp.Raw), `"cap-1"`) {
		t.Fatalf("fetch raw response missing capsule: %s", resp.Raw)
	}
}

func TestClientKGQueryRequiresAPIKeyAndUsesBearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/kg/query" {
			t.Fatalf("request = %s %s, want POST /kg/query", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer kg-secret" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answers":[{"text":"stored insight"}]}`))
	}))
	defer server.Close()

	noKey, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := noKey.KGQuery(context.Background(), KGQueryRequest{Question: "anything"}); err == nil {
		t.Fatal("expected KGQuery without API key to fail")
	}

	client, err := NewClient(Config{BaseURL: server.URL, APIKey: "kg-secret", HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	resp, err := client.KGQuery(context.Background(), KGQueryRequest{Question: "anything"})
	if err != nil {
		t.Fatalf("KGQuery() error = %v", err)
	}
	if !strings.Contains(string(resp.Raw), "stored insight") {
		t.Fatalf("KG raw response missing answer: %s", resp.Raw)
	}
}

func TestClientRejectsLoopbackBaseURLWhenNoHTTPClientOverride(t *testing.T) {
	if _, err := NewClient(Config{BaseURL: "http://127.0.0.1:8080"}); err == nil {
		t.Fatal("expected loopback base URL to be rejected by SSRF protection")
	}
}

func TestClientEnforcesMaxResultBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"payload":"this response is intentionally too large"}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{BaseURL: server.URL, HTTPClient: server.Client(), Timeout: time.Second, MaxResultBytes: 12})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Status(context.Background()); err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("expected response size error, got %v", err)
	}
}
