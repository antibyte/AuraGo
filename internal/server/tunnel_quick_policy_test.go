package server

import (
	"aurago/internal/config"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudflareQuickAPIRejectsLegacyPortsAndMalformedJSON(t *testing.T) {
	cfg := &config.Config{}
	cfg.CloudflareTunnel.Enabled = true
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	for _, body := range []string{`{"port":2375}`, `{`} {
		w := httptest.NewRecorder()
		handleTunnelQuick(s)(w, httptest.NewRequest(http.MethodPost, "/api/tunnel/quick", strings.NewReader(body)))
		if body == "{" {
			if w.Code != 400 {
				t.Fatal(w.Code)
			}
			continue
		}
		if !strings.Contains(w.Body.String(), "no longer accepts a port") {
			t.Fatal(w.Body.String())
		}
	}
}
