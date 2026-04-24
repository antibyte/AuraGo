package server

import (
	"net/http"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestNewInternalHTTPClientDisablesHTTP2AndKeepAlive(t *testing.T) {
	client := NewInternalHTTPClient(42 * time.Second)
	if client.Timeout != 42*time.Second {
		t.Fatalf("timeout = %v, want 42s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.ForceAttemptHTTP2 {
		t.Fatal("expected ForceAttemptHTTP2 to be false")
	}
	if !transport.DisableKeepAlives {
		t.Fatal("expected DisableKeepAlives to be true")
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be set")
	}
	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected InsecureSkipVerify to be true for loopback self-signed TLS")
	}
}

func TestInternalAPIURLUsesDedicatedHTTPWhenHTTPSEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 8088
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443

	if got := InternalAPIURL(cfg); got != "http://127.0.0.1:8088" {
		t.Fatalf("InternalAPIURL = %q, want http://127.0.0.1:8088", got)
	}
}

func TestInternalAPIURLUsesExplicitLoopbackPort(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 8088
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	cfg.CloudflareTunnel.LoopbackPort = 18080

	if got := InternalAPIURL(cfg); got != "http://127.0.0.1:18080" {
		t.Fatalf("InternalAPIURL = %q, want http://127.0.0.1:18080", got)
	}
}

func TestInternalAPIURLFallsBackToHTTPSWhenNoDedicatedLoopbackPort(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 8443
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443

	if got := InternalAPIURL(cfg); got != "https://127.0.0.1:8443" {
		t.Fatalf("InternalAPIURL = %q, want https://127.0.0.1:8443", got)
	}
}

func TestInternalAPIURLUsesHTTPWhenHTTPSDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.Port = 8080
	cfg.Server.HTTPS.Enabled = false
	cfg.Server.HTTPS.HTTPSPort = 0

	if got := InternalAPIURL(cfg); got != "http://127.0.0.1:8080" {
		t.Fatalf("InternalAPIURL = %q, want http://127.0.0.1:8080", got)
	}
}
