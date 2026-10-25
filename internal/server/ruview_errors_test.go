package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ruview"
	"aurago/internal/security"
)

func TestRuViewErrorCodesMapToHTTP(t *testing.T) {
	want := map[string]int{
		ruview.CodeDisabled: 403, ruview.CodeReadOnly: 403, ruview.CodeExternalRuntime: 403, ruview.CodeDockerReadOnly: 403,
		ruview.CodeAlreadyRunning: 409, ruview.CodeNotInstalled: 409, ruview.CodeBusy: 409, ruview.CodeCancelled: 409,
		ruview.CodeContainerConflict: 409, ruview.CodePlatformUnsupported: 409,
		ruview.CodeInvalidNodeID: 400, ruview.CodeInvalidConfig: 400,
		ruview.CodeUnreachable: 502, ruview.CodeUpstreamAuth: 502, ruview.CodeUpstreamError: 502, ruview.CodeDockerUnavailable: 502,
		ruview.CodeImageDenied: 502, ruview.CodeImagePullFailed: 502, ruview.CodeContractUnsupported: 502, ruview.CodeDownloadFailed: 502,
		ruview.CodeStartFailed: 502, ruview.CodeStopFailed: 502, ruview.CodeProcessExited: 502, ruview.CodeSourceMismatch: 502,
		ruview.CodeUnexpectedSimulation: 502,
		ruview.CodeTimeout:              504,
		ruview.CodeStorageUnreadable:    503,
		ruview.CodeStorageFull:          507,
		ruview.CodeArtifactMissing:      500, ruview.CodeUnknown: 500,
	}
	codes := ruview.ErrorCodes()
	if len(want) != len(codes) {
		t.Fatalf("the table lists %d codes, the package declares %d: %v", len(want), len(codes), codes)
	}
	for _, code := range codes {
		status, ok := want[code]
		if !ok {
			t.Errorf("code %q is not in the table", code)
			continue
		}
		if got := ruview.HTTPStatus(code); got != status {
			t.Errorf("HTTPStatus(%q) = %d, want %d", code, got, status)
		}
		w := httptest.NewRecorder()
		ruviewError(w, &ruview.Error{Code: code, Detail: "HTTP 403: registry denied"})
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if w.Code != status || body["error_code"] != code || body["error"] == "" || body["detail"] != "HTTP 403: registry denied" {
			t.Errorf("%s: HTTP %d %v", code, w.Code, body)
		}
		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s: content type %q", code, w.Header().Get("Content-Type"))
		}
	}
	if got := ruview.HTTPStatus("no_such_code"); got != 500 {
		t.Errorf("an unknown code must be 500, got %d", got)
	}
	// An untyped error is unknown/500 and its detail carries no path.
	w := httptest.NewRecorder()
	ruviewError(w, errors.New("boom at /etc/secret"))
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 500 || body["error_code"] != ruview.CodeUnknown || strings.Contains(body["detail"], "/etc") || strings.Contains(body["error"], "/etc") {
		t.Fatalf("untyped error: %d %v", w.Code, body)
	}
	w = httptest.NewRecorder()
	ruviewError(w, context.Canceled)
	if w.Code != 409 || !strings.Contains(w.Body.String(), `"error_code":"cancelled"`) {
		t.Fatalf("a cancelled context must be 409 cancelled: %d %s", w.Code, w.Body.String())
	}
}

func TestRuViewRouteRefusalsCarryCodes(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.RuView = config.DefaultRuViewConfig()
	s.RuView = ruview.New(s.ConfigSnapshot, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer s.RuView.Close()
	mux := http.NewServeMux()
	registerRuViewRoutes(mux, s)
	handler := desktopIntegrationHandler(s, mux)
	admin, _, err := s.TokenManager.Create("ruview error fixture", []string{desktopScopeAdmin, "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, route, body string) (int, string) {
		t.Helper()
		r := httptest.NewRequest(method, "http://aurago.test/api/desktop/integrations/ruview/"+route, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+admin)
		r.Header.Set("Origin", "http://aurago.test")
		w := httptest.NewRecorder()
		handler(w, r)
		var parsed map[string]string
		_ = json.Unmarshal(w.Body.Bytes(), &parsed)
		return w.Code, parsed["error_code"]
	}
	check := func(name, method, route, body string, status int, code string) {
		t.Helper()
		if gotStatus, gotCode := call(method, route, body); gotStatus != status || gotCode != code {
			t.Errorf("%s: %s %s = %d %q, want %d %q", name, method, route, gotStatus, gotCode, status, code)
		}
	}

	s.Cfg.RuView.Enabled, s.Cfg.RuView.ReadOnly, s.Cfg.RuView.Mode = true, true, "docker"
	check("read-only docker", "POST", "runtime/start", "", 403, ruview.CodeReadOnly)

	s.Cfg.RuView.Enabled = false
	check("disabled start", "POST", "runtime/start", "", 403, ruview.CodeDisabled)
	check("disabled nodes", "GET", "nodes", "", 403, ruview.CodeDisabled)

	s.Cfg.RuView.Enabled, s.Cfg.RuView.ReadOnly, s.Cfg.RuView.Mode, s.Cfg.RuView.URL = true, false, "external", "https://example.invalid"
	check("external stop", "POST", "runtime/stop", "", 403, ruview.CodeExternalRuntime)
	check("external start", "POST", "runtime/start", "", 403, ruview.CodeExternalRuntime)

	check("node id 0", "POST", "nodes", `{"node_id":0}`, 400, ruview.CodeInvalidNodeID)
	check("node id 256", "POST", "nodes", `{"node_id":256}`, 400, ruview.CodeInvalidNodeID)
}

// The read-only switch of the "Allow changes…" action saves one field. The
// deep merge must keep every other RuView setting and the save must not need
// anything else (design D4).
func TestRuViewReadOnlySingleFieldSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	initial := "ruview:\n  enabled: true\n  readonly: true\n  agent_access: true\n  mode: docker\n  source: esp32\n  storage_mb: 512\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	vault, err := security.NewVault(strings.Repeat("1", 64), filepath.Join(filepath.Dir(path), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	w := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"ruview":{"readonly":false}}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("single-field save failed: %d %s", w.Code, w.Body.String())
	}
	saved, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	r := saved.RuView
	if r.ReadOnly || !r.Enabled || !r.AgentAccess || r.StorageMB != 512 || r.Mode != "docker" || r.Source != "esp32" {
		t.Fatalf("the save changed more than readonly: %+v", r)
	}
}
