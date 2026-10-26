package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

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
