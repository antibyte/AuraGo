package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
