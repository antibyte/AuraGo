package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/systemworld"
)

func TestContainerToolErrorsUseBadGatewayAndKeepTheBody(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = refusingDockerAPI(t)
	defer replaceContainerProtection(containerProtection{})()

	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		req     *http.Request
	}{
		{"list", handleContainersList(s), httptest.NewRequest(http.MethodGet, "/api/containers", nil)},
		{"start", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/start", nil)},
		{"stop", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/stop", nil)},
		{"restart", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil)},
		{"update", handleContainerAction(s), httptest.NewRequest(http.MethodPost, "/api/containers/demo/update", nil)},
		{"logs", handleContainerAction(s), httptest.NewRequest(http.MethodGet, "/api/containers/demo/logs", nil)},
		{"inspect", handleContainerAction(s), httptest.NewRequest(http.MethodGet, "/api/containers/demo/inspect", nil)},
		{"stats", handleContainerAction(s), httptest.NewRequest(http.MethodGet, "/api/containers/demo/stats", nil)},
		{"remove", handleContainerAction(s), httptest.NewRequest(http.MethodDelete, "/api/containers/demo", nil)},
	} {
		rec := httptest.NewRecorder()
		tc.handler(rec, tc.req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("%s status = %d, want 502; body=%s", tc.name, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("%s Content-Type = %q", tc.name, got)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "error" || !strings.Contains(rec.Body.String(), "engine refused") {
			t.Fatalf("%s body = %s (%v); want the tool error JSON", tc.name, rec.Body.String(), err)
		}
	}
}

func TestContainerToolSuccessAndGuardsKeepTheirStatus(t *testing.T) {
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/containers/demo/restart"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/containers/json"):
			_, _ = w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	})

	rec := httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("restart = %d %s, want 200 ok", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("list = %d %s, want 200 ok", rec.Code, rec.Body.String())
	}

	s.Cfg.Docker.ReadOnly = true
	rec = httptest.NewRecorder()
	handleContainerAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/containers/demo/restart", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("read-only restart = %d, want 403", rec.Code)
	}
	s.Cfg.Docker.Enabled = false
	rec = httptest.NewRecorder()
	handleContainersList(s)(rec, httptest.NewRequest(http.MethodGet, "/api/containers", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled list = %d, want 503", rec.Code)
	}
}

func TestWriteContainerToolResultOnlyMapsErrorEnvelopesTo502(t *testing.T) {
	largeLogs := strings.Repeat("log line with \\\"quotes\\\" and ünïcode\\n", 40000)
	largeOK := `{"logs":"` + largeLogs + `","status":"ok"}`
	largeError := `{"message":"` + largeLogs + `","status":"error"}`
	if len(largeOK) < 1<<20 || len(largeError) < 1<<20 {
		t.Fatalf("large bodies must exceed 1 MiB, got %d and %d", len(largeOK), len(largeError))
	}

	for _, tc := range []struct {
		name   string
		result string
		want   int
	}{
		{"plain text", "not json", http.StatusOK},
		{"empty", "", http.StatusOK},
		{"json array", `[]`, http.StatusOK},
		{"non-string status", `{"status":5}`, http.StatusOK},
		{"ok", `{"status":"ok"}`, http.StatusOK},
		{"ok result that carries error text", `{"status":"ok","logs":"{\"status\":\"error\"}"}`, http.StatusOK},
		{"error", `{"status":"error","message":"engine refused"}`, http.StatusBadGateway},
		{"error with extra fields", `{"code":"x","message":"m","status":"error"}`, http.StatusBadGateway},
		{"large ok", largeOK, http.StatusOK},
		{"large error", largeError, http.StatusBadGateway},
	} {
		rec := httptest.NewRecorder()
		writeContainerToolResult(rec, tc.result)
		if rec.Code != tc.want {
			t.Fatalf("%s: status = %d, want %d", tc.name, rec.Code, tc.want)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("%s: Content-Type = %q", tc.name, got)
		}
		if rec.Body.String() != tc.result {
			t.Fatalf("%s: body changed (%d bytes in, %d bytes out)", tc.name, len(tc.result), rec.Body.Len())
		}
	}
}

// System World treats every non-2xx answer of handleContainerAction as a failed
// action. A Docker refusal now answers 502 and must still read as failed.
func TestSystemWorldContainerActionStaysFailedWhenDockerRefuses(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	s.Cfg.Docker.Enabled = true
	var restarts atomic.Int32
	s.Cfg.Docker.Host = newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/restart") {
			restarts.Add(1)
		}
		http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
	})
	runtime := s.worldRuntime()
	runtime.mu.Lock()
	runtime.set(systemworld.Entity{ID: "container:demo", Kind: "container", District: "infra", State: "running", At: time.Now().UnixMilli()})
	runtime.mu.Unlock()

	rec := httptest.NewRecorder()
	handleSystemWorldAction(s)(rec, httptest.NewRequest(http.MethodPost, "/api/desktop/system-world/actions",
		strings.NewReader(`{"entity":"container:demo","action":"restart","request_id":"refused-request-012345","confirmed":true}`)))
	if restarts.Load() != 1 {
		t.Fatalf("the refusing Docker API saw %d restart requests, want 1", restarts.Load())
	}
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"failed"`) || strings.Contains(rec.Body.String(), `"completed"`) {
		t.Fatalf("refused restart = %d %s, want 409 failed", rec.Code, rec.Body.String())
	}
}
