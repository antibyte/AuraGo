package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/dbutil"
	"aurago/internal/security"
)

func TestDurableRuntimeStatusReadAuthorizationAndUnknownCounts(t *testing.T) {
	s, readToken, writeToken := testDesktopPermissionServer(t)
	s.Cfg.Flows.DurableEnabled = true
	request := func(method, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://example.test/api/desktop/flow-runtime", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.handleFlowRuntime(w, r)
		return w
	}
	if w := request(http.MethodGet, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status: %d", w.Code)
	}
	if w := request(http.MethodGet, writeToken); w.Code != http.StatusForbidden {
		t.Fatalf("write-only status: %d", w.Code)
	}
	w := request(http.MethodGet, readToken)
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status: %d, cache %q", w.Code, w.Header().Get("Cache-Control"))
	}
	var result flowDurableCapability
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Configured || result.Effective || result.RecoveryAvailable || result.Reason != "service_unavailable" || result.WaitingCount != nil || result.ClaimedCount != nil || result.BlockedCount != nil {
		t.Fatalf("unavailable capability: %+v", result)
	}
	if w := request(http.MethodPost, readToken); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status write: %d", w.Code)
	}
}

func TestDurableRuntimeStatusDefaultsOff(t *testing.T) {
	s := c16StartedServer(t, nil)
	status := s.flowDurableCapability(context.Background())
	if status.Configured || status.Effective || status.RecoveryAvailable || status.Reason != "disabled" || status.WaitingCount == nil || status.ClaimedCount == nil || status.BlockedCount == nil {
		t.Fatalf("default durable capability: %+v", status)
	}
}

func TestDurableConfigStatusIsTransient(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("flows:\n  enabled: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("d", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	w := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("get config = %d %s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	status, ok := result["_flows_durable_status"].(map[string]any)
	if !ok || status["reason"] != "service_unavailable" || status["waiting_count"] != nil {
		t.Fatalf("config capability = %v", result["_flows_durable_status"])
	}
	w = httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"_flows_durable_status":{"configured":true},"flows":{"durable_enabled":false}}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("update config = %d %s", w.Code, w.Body.String())
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "_flows_durable_status") {
		t.Fatal("transient status was saved to config")
	}
}

func TestDurableLegacyCancelCannotBypassGeneration(t *testing.T) {
	s := c16StartedServer(t, nil)
	s.Cfg.Auth.Enabled = false
	rec := c16PublishedFlow(t, s, greetFlowJSON)
	db, err := dbutil.Open(s.flowsDBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	started := time.Now().UTC().Format(time.RFC3339Nano)
	for _, run := range []struct {
		id               string
		durable          int
		legacyCheckpoint bool
	}{
		{"run_marked", 1, false},
		{"run_legacy", 0, true},
	} {
		if _, err := db.Exec(`INSERT INTO flow_runs(id,flow_id,revision,mode,status,started_at,durable) VALUES(?,?,?,'live','running',?,?)`, run.id, rec.ID, rec.LiveRevision, started, run.durable); err != nil {
			t.Fatal(err)
		}
		if run.legacyCheckpoint {
			if _, err := db.Exec(`INSERT INTO flow_checkpoints(run_id,generation,state,wake_at,deadline,ciphertext,updated_at) VALUES(?,1,'waiting',?,?,?,?)`, run.id, started, started, []byte("opaque"), started); err != nil {
				t.Fatal(err)
			}
		}
		r := httptest.NewRequest(http.MethodPost, "http://example.test/api/desktop/flows/runs/"+run.id+"/cancel", nil)
		r.Header.Set("Origin", "http://example.test")
		w := httptest.NewRecorder()
		s.handleFlows(w, r)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "FLOW_DURABLE_GENERATION_CONFLICT") {
			t.Fatalf("%s legacy cancel = %d %s", run.id, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest(http.MethodGet, "http://example.test/api/desktop/flows/runs/run_legacy", nil)
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"blocked_code":"incompatible"`) || !strings.Contains(w.Body.String(), `"cancel_available":true`) {
		t.Fatalf("legacy checkpoint detail = %d %s", w.Code, w.Body.String())
	}
	// Mission Control's aggregate stop must not bypass the same generation fence.
	s.MissionManagerV2.FlowRunStarted(rec.MissionID, "manual", "")
	r = httptest.NewRequest(http.MethodPost, "http://example.test/api/missions/"+rec.MissionID+"/cancel", nil)
	w = httptest.NewRecorder()
	handleMissionCancelV2(s, w, r, rec.MissionID)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "FLOW_DURABLE_GENERATION_CONFLICT") {
		t.Fatalf("mission aggregate cancel = %d %s", w.Code, w.Body.String())
	}
}
