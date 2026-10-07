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

// The status reports the certificate the server actually serves (and Eggs
// pin): auto, an empty or an unknown mode without a domain fall back to the
// self-signed certificate, so the status shows its cert_info instead of
// Let's Encrypt fields. auto with a domain still reports Let's Encrypt.
func TestCertStatusReportsTheSelfSignedFallback(t *testing.T) {
	for _, tc := range []struct {
		name, mode, domain string
		selfSigned         bool
	}{
		{"auto without domain", "auto", "", true},
		{"padded auto without domain", " Auto ", "", true},
		{"empty mode without domain", "", "", true},
		{"unknown mode without domain", "bogus", "", true},
		{"selfsigned", "selfsigned", "aurago.example.com", true},
		{"auto with domain", "auto", "aurago.example.com", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Directories.DataDir = t.TempDir()
			cfg.Server.HTTPS.Enabled = true
			cfg.Server.HTTPS.CertMode = tc.mode
			cfg.Server.HTTPS.Domain = tc.domain
			cfg.Server.HTTPS.Email = "ops@example.com"
			s := &Server{Cfg: cfg, Logger: slog.Default()}

			rec := httptest.NewRecorder()
			handleCertStatus(s)(rec, httptest.NewRequest(http.MethodGet, "/api/cert/status", nil))
			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v (%s)", err, rec.Body.String())
			}
			_, hasCertInfo := body["cert_info"]
			_, hasCertDir := body["cert_dir"]
			_, hasEmail := body["email"]
			if tc.selfSigned {
				if !hasCertInfo || hasCertDir || hasEmail {
					t.Fatalf("status = %v, want the self-signed cert_info and no Let's Encrypt fields", body)
				}
				return
			}
			if hasCertInfo || !hasCertDir || !hasEmail {
				t.Fatalf("status = %v, want Let's Encrypt fields", body)
			}
		})
	}
}
