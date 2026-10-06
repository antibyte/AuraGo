package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

// Eggs pin the self-signed certificate, so regenerating it must tell the
// operator to restart the master and safe-reconfigure every Egg.
func TestCertRegenerateTellsOperatorToSafeReconfigureEggs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	var logs bytes.Buffer
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(&logs, nil))}

	rec := httptest.NewRecorder()
	handleCertRegenerate(s)(rec, httptest.NewRequest(http.MethodPost, "/api/cert/regenerate", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(body["message"], "Restart AuraGo") || !strings.Contains(body["message"], "safe-reconfigure for each Egg") {
		t.Fatalf("message = %q, want restart and per-Egg safe-reconfigure instructions", body["message"])
	}
	if !strings.Contains(logs.String(), "level=WARN") || !strings.Contains(logs.String(), "safe-reconfigure") {
		t.Fatalf("want a warning about safe-reconfiguring the Eggs, logs:\n%s", logs.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.Directories.DataDir, "certs", "selfsigned.crt")); err != nil {
		t.Fatalf("certificate not regenerated: %v", err)
	}
}
