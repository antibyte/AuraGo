package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxmoxErrorsAreValidJSON(t *testing.T) {
	cfg := ProxmoxConfig{URL: "http://invalid.test/\"quoted\"", Node: "node", AllowDestructive: true}
	results := []string{
		ProxmoxListNodes(cfg), ProxmoxListVMs(cfg, "node"), ProxmoxListContainers(cfg, "node"),
		ProxmoxGetStatus(cfg, "node", "qemu", "101"), ProxmoxVMAction(cfg, "node", "qemu", "101", "start"),
		ProxmoxVMAction(cfg, "node", "qemu", "101", "bad\"action\n"), ProxmoxNodeStatus(cfg, "node"),
		ProxmoxOverview(cfg, "node"), ProxmoxClusterResources(cfg, "vm"), ProxmoxGetStorage(cfg, "node"),
		ProxmoxGetTaskLog(cfg, "node", "task"), ProxmoxCreateSnapshot(cfg, "node", "qemu", "101", "snapshot", ""),
		ProxmoxListSnapshots(cfg, "node", "qemu", "101"),
	}
	for i, result := range results {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result), &decoded); err != nil || decoded["status"] != "error" {
			t.Fatalf("operation %d invalid error JSON: %s", i, result)
		}
	}
}

func TestYepAPIPostDoesNotRepeatUncertainBillableRequests(t *testing.T) {
	for _, status := range []int{0, 429, 503} {
		requests := 0
		c := NewYepAPIClient("fixture")
		c.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests++
			if status == 0 {
				return nil, errors.New("connection lost after acceptance")
			}
			return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":false}`))}, nil
		})}
		if _, err := c.Post(context.Background(), "/v1/serp/google", map[string]string{"query": "fixture"}); err == nil || requests != 1 {
			t.Fatalf("status %d: requests=%d err=%v", status, requests, err)
		}
	}
}

func TestIntegrationKeysDoNotFollowCrossOriginRedirects(t *testing.T) {
	for _, provider := range []string{"yepapi", "dograh"} {
		t.Run(provider, func(t *testing.T) {
			leaked := false
			target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true; w.Write([]byte(`[]`)) }))
			defer target.Close()
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			}))
			defer origin.Close()
			var err error
			if provider == "yepapi" {
				err = NewYepAPIClientWithBaseURL("fixture", origin.URL).Probe(context.Background())
			} else {
				_, err = (DograhAPIClient{BaseURL: origin.URL, APIKey: "fixture"}).do(context.Background(), http.MethodGet, "/api/v1/node-types", nil, nil)
			}
			if err == nil || leaked {
				t.Fatalf("cross-origin redirect followed: leaked=%v err=%v", leaked, err)
			}
		})
	}
}
