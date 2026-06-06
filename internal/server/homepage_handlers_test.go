package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tsnetnode"
)

func TestHomepageBrowserURLUsesRequestHost(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/status", nil)
	req.Host = "192.168.1.50:8443"

	got := homepageBrowserURLForRequest(req, 5173)
	if got != "http://192.168.1.50:5173" {
		t.Fatalf("homepageBrowserURLForRequest() = %q, want request host URL", got)
	}
}

func TestHomepageBrowserURLDoesNotInventTailscalePort(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/status", nil)
	req.Host = "aurago.taild1480.ts.net"

	got := homepageBrowserURLForRequest(req, 5173)
	if got != "" {
		t.Fatalf("homepageBrowserURLForRequest() = %q, want no local URL over Tailscale", got)
	}
}

func TestHomepageStatusBrowserURLUsesTailscaleHomepageExposure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Homepage.WebServerPort = 8080
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ExposeHomepage = true
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	s.TsNetManager = tsnetnode.NewManager(cfg, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/homepage/status", nil)
	req.Host = "aurago.taild1480.ts.net"

	got := homepageStatusBrowserURL(s, cfg, req)
	if got != "https://aurago.taild1480.ts.net:8443" {
		t.Fatalf("homepageStatusBrowserURL() = %q, want Tailscale Homepage URL", got)
	}
}

func TestEnrichHomepageStatusUsesTailscaleBrowserURL(t *testing.T) {
	t.Parallel()

	payload := map[string]interface{}{
		"web_container": map[string]interface{}{"running": true, "exists": true},
	}
	enrichHomepageStatusForRequest(payload, "https://aurago.taild1480.ts.net:8443")

	if payload["preview_url"] != "https://aurago.taild1480.ts.net:8443" {
		t.Fatalf("preview_url = %#v, want Tailscale URL", payload["preview_url"])
	}
	web, ok := payload["web_container"].(map[string]interface{})
	if !ok || web["browser_url"] != "https://aurago.taild1480.ts.net:8443" {
		t.Fatalf("web_container browser_url = %#v", payload["web_container"])
	}
}
