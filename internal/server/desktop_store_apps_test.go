package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"aurago/internal/desktopstore"
	"aurago/internal/tools"
)

func TestStoreAppExecutionNeedsVerifiedLiveGrant(t *testing.T) {
	var starts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login":
			w.Write([]byte(`{"type":"credentials"}`))
		case "/flows":
			w.Write([]byte(`[{"id":"tab","type":"tab","label":"Demo"},{"id":"allowed","type":"inject","z":"tab"}]`))
		case "/inject/allowed":
			starts.Add(1)
		default:
			w.WriteHeader(404)
		}
	}))
	defer upstream.Close()
	s, store := storeIntegrationFixture(t, "node-red")
	req := storeIntegrationUpdate{URL: upstream.URL, Credential: "node-red-test-token", Read: true, AllowedIDs: []string{"allowed"}}
	if err := s.saveStoreIntegration(t.Context(), store, "node-red", req, false); err != nil {
		t.Fatal(err)
	}
	call := tools.StoreAppRequest{Operation: "trigger", NodeID: "allowed"}
	if _, err := s.executeStoreApplication(t.Context(), "node-red", call); err == nil {
		t.Fatal("unverified execution")
	}
	settings, _ := desktopstore.ReadIntegration(s.Vault, "node-red")
	if err := s.testStoreIntegration(t.Context(), store, "node-red", settings); err != nil {
		t.Fatal(err)
	}
	if _, err := s.executeStoreApplication(t.Context(), "node-red", call); err == nil {
		t.Fatal("read grant executed")
	}
	req.Revision, req.Execute = settings.Revision, true
	if err := s.saveStoreIntegration(t.Context(), store, "node-red", req, false); err != nil {
		t.Fatal(err)
	}
	settings, _ = desktopstore.ReadIntegration(s.Vault, "node-red")
	if err := s.testStoreIntegration(t.Context(), store, "node-red", settings); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 0 {
		t.Fatal("test executed a flow")
	}
	if _, err := s.executeStoreApplication(t.Context(), "node-red", call); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 1 {
		t.Fatal("missing trigger")
	}
	s.revokeStoreIntegration("node-red")
	if _, err := s.executeStoreApplication(t.Context(), "node-red", call); err == nil {
		t.Fatal("revoked execution")
	}
	if starts.Load() != 1 {
		t.Fatal("revoked trigger reached upstream")
	}
}

func TestStoreMCPExactCapabilitiesAndMissingTool(t *testing.T) {
	s, _ := storeIntegrationFixture(t, "n8n")
	original := storeMCPProbeTools
	t.Cleanup(func() { storeMCPProbeTools = original })
	var names []string
	storeMCPProbeTools = func(ctx context.Context, cfg tools.MCPServerConfig, logger *slog.Logger) ([]string, error) {
		return names, nil
	}
	settings := desktopstore.IntegrationSettings{URL: "http://localhost:5678", Credential: "mcp-test-token", Read: true}
	for _, app := range []string{"n8n", "bytestash"} {
		cfg, err := s.storeMCPConfig(app, settings)
		if err != nil {
			t.Fatal(err)
		}
		names = cfg.AllowedTools
		if cfg.AllowDestructive || slices.Contains(names, "execute_workflow") || slices.Contains(names, "create_snippet") {
			t.Fatal("read-only MCP writes")
		}
		if err = s.probeStoreMCP(t.Context(), app, settings); err != nil {
			t.Fatal(err)
		}
		names = names[:len(names)-1]
		if err = s.probeStoreMCP(t.Context(), app, settings); err == nil {
			t.Fatal("missing capability accepted")
		}
	}
	settings.Execute = true
	cfg, err := s.storeMCPConfig("n8n", settings)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowDestructive || !slices.Equal(cfg.AllowedTools, []string{"search_workflows", "get_workflow_details", "execute_workflow"}) {
		t.Fatalf("wrong n8n allowlist: %v", cfg.AllowedTools)
	}
	settings.Execute = false
	settings.Create = true
	cfg, err = s.storeMCPConfig("bytestash", settings)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.AllowedTools, []string{"list_snippets", "get_snippet", "list_metadata", "create_snippet"}) {
		t.Fatal("wrong ByteStash allowlist")
	}
}
