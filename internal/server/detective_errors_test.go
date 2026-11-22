package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/detective"
)

// Every refusal carries a stable code next to the server text, so the app can show a
// translated sentence instead of the English message (audit 4.14).
func TestDetectiveErrorCodes(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not found", detective.ErrNotFound, 404, "not_found"},
		{"conflict", detective.ErrConflict, 409, "conflict"},
		{"budget", fmt.Errorf("start: %w", detective.ErrBudget), 409, "budget_exhausted"},
		{"permission", detective.ErrPermission, 403, "permission_revoked"},
		{"closed", detective.ErrClosed, 503, "unavailable"},
		{"format", fmt.Errorf("%w %q", detective.ErrExportFormat, "xls"), 400, "unsupported_format"},
		{"too large", detective.ErrExportTooLarge, 400, "export_too_large"},
		{"font", fmt.Errorf("%w U+0930", detective.ErrPDFFont), 400, "pdf_unsupported_font"},
		{"copy", errDetectiveCopyExists, 409, "copy_exists"},
		{"plain", errors.New("topic required"), 400, "invalid_request"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		detectiveError(w, c.err)
		if w.Code != c.status {
			t.Errorf("%s: status %d, want %d", c.name, w.Code, c.status)
		}
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if body["code"] != c.code {
			t.Errorf("%s: code %q, want %q", c.name, body["code"], c.code)
		}
		if body["error"] != c.err.Error() {
			t.Errorf("%s: error %q, want %q", c.name, body["error"], c.err.Error())
		}
	}
}

// The app orders the export formats and shows its PDF hint by this capability, so it
// must name the writer the service really uses (audit 4.14).
func TestDetectivePDFSupportFollowsTheConfiguredRenderer(t *testing.T) {
	if backend, scripts := detectivePDFSupport(nil); backend != "builtin" || scripts {
		t.Fatalf("nil config: %q %v", backend, scripts)
	}
	cfg := &config.Config{}
	if backend, scripts := detectivePDFSupport(cfg); backend != "builtin" || scripts {
		t.Fatalf("default: %q %v", backend, scripts)
	}
	cfg.Tools.DocumentCreator.Enabled = true
	cfg.Tools.DocumentCreator.Backend = "gotenberg"
	if backend, scripts := detectivePDFSupport(cfg); backend != "gotenberg" || !scripts {
		t.Fatalf("gotenberg: %q %v", backend, scripts)
	}
	cfg.Tools.DocumentCreator.Enabled = false
	if backend, scripts := detectivePDFSupport(cfg); backend != "builtin" || scripts {
		t.Fatalf("disabled: %q %v", backend, scripts)
	}
}

func TestDetectiveCapabilitiesAdvertiseThePDFWriter(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Enabled = false
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Detective.Enabled = true
	s := &Server{Cfg: cfg}
	r := httptest.NewRequest("GET", "/api/desktop/detective/capabilities", nil)
	w := httptest.NewRecorder()
	s.handleDetective(w, r)
	if w.Code != 200 {
		t.Fatalf("capabilities: %d %s", w.Code, w.Body.String())
	}
	var caps map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &caps); err != nil {
		t.Fatal(err)
	}
	if caps["pdf_backend"] != "builtin" {
		t.Fatalf("pdf_backend=%v", caps["pdf_backend"])
	}
	if scripts, ok := caps["pdf_scripts"].(bool); !ok || scripts {
		t.Fatalf("pdf_scripts=%v", caps["pdf_scripts"])
	}
}

// detectiveRefusalServer builds a writable Desktop with a live Detective service.
func detectiveRefusalServer(t *testing.T) (*Server, *detective.Service) {
	t.Helper()
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Detective.Enabled = true
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "refusal.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return &Server{Cfg: cfg, Detective: svc}, svc
}

func detectiveBody(t *testing.T, s *Server, method, target string) (int, map[string]string) {
	t.Helper()
	r := httptest.NewRequest(method, target, strings.NewReader(`{"topic":"refusal"}`))
	w := httptest.NewRecorder()
	s.handleDetective(w, r)
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	return w.Code, body
}

// The coarse gates answer with the documented {error, code} envelope, so the app can
// translate the reason instead of showing the English sentence (audit 2026-11-22 F3).
func TestDetectiveRefusalsCarryStableCodes(t *testing.T) {
	t.Run("desktop off", func(t *testing.T) {
		s, _ := detectiveRefusalServer(t)
		s.Cfg.VirtualDesktop.Enabled = false
		status, body := detectiveBody(t, s, http.MethodGet, "/api/desktop/detective/cases")
		if status != http.StatusServiceUnavailable || body["code"] != "unavailable" {
			t.Fatalf("got %d %v", status, body)
		}
	})
	t.Run("detective off", func(t *testing.T) {
		s, _ := detectiveRefusalServer(t)
		s.Cfg.Detective.Enabled = false
		status, body := detectiveBody(t, s, http.MethodGet, "/api/desktop/detective/cases")
		if status != http.StatusServiceUnavailable || body["code"] != "unavailable" {
			t.Fatalf("got %d %v", status, body)
		}
	})
	t.Run("read only", func(t *testing.T) {
		s, _ := detectiveRefusalServer(t)
		s.Cfg.Detective.ReadOnly = true
		status, body := detectiveBody(t, s, http.MethodPost, "/api/desktop/detective/cases")
		if status != http.StatusForbidden || body["code"] != "read_only" {
			t.Fatalf("got %d %v", status, body)
		}
	})
	t.Run("method not allowed", func(t *testing.T) {
		s, _ := detectiveRefusalServer(t)
		status, body := detectiveBody(t, s, http.MethodPut, "/api/desktop/detective/cases")
		if status != http.StatusMethodNotAllowed || body["code"] != "method_not_allowed" {
			t.Fatalf("got %d %v", status, body)
		}
	})
}

// The export route names a refused format through the documented code and serves the
// cached artifact with its checksum and attachment headers (audit 2026-11-22 F2).
func TestDetectiveExportRouteAnswersWithCodes(t *testing.T) {
	s, _, _ := notifyFixture(t, true)
	svc := notifyTestService(t, s)
	s.Detective = svc
	s.Cfg.VirtualDesktop.AllowAgentControl = true
	publishOnce(t, svc, "export-route")
	var id string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		cases, err := svc.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(cases) != 1 {
			t.Fatalf("cases: %d", len(cases))
		}
		id = cases[0].ID
		if cases[0].Run.Status == "completed" && cases[0].LatestRevision >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	c, err := svc.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if c.Run.Status != "completed" || c.LatestRevision < 1 {
		t.Fatalf("case did not settle: %s rev=%d", c.Run.Status, c.LatestRevision)
	}
	base := "/api/desktop/detective/cases/" + id + "/export"
	status, body := detectiveBody(t, s, http.MethodGet, base+"?revision=1&format=xls")
	if status != http.StatusBadRequest || body["code"] != "unsupported_format" {
		t.Fatalf("format: got %d %v", status, body)
	}
	status, body = detectiveBody(t, s, http.MethodGet, base+"?revision=0&format=md")
	if status != http.StatusNotFound || body["code"] != "not_found" {
		t.Fatalf("revision: got %d %v", status, body)
	}
	w := httptest.NewRecorder()
	s.handleDetective(w, httptest.NewRequest(http.MethodGet, base+"?revision=1&format=md", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("ETag"), `"`) || !strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("headers: etag=%q disposition=%q", w.Header().Get("ETag"), w.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(w.Body.String(), "42 units") {
		t.Fatal("export lost the reported measurement")
	}
}
