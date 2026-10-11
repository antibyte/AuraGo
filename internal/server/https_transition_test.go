package server

import (
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

func TestHTTPSListenerHostRedirectAndDisabledHTTP(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "0.0.0.0"} {
		t.Run(host, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Server.Host = host
			cfg.Server.HTTPS.Enabled = true
			cfg.Server.HTTPS.HTTPSPort = 8443
			cfg.Server.HTTPS.CertMode = "selfsigned"
			tlsCfg := NewTLSConfigFromConfig(cfg, t.TempDir())
			server, redirect, err := SetupServers(tlsCfg, http.NotFoundHandler(), slog.Default())
			if err != nil {
				t.Fatal(err)
			}
			if redirect != nil {
				t.Fatal("http_port: 0 created a listener")
			}
			want := host + ":8443"
			if host == "::1" {
				want = "[::1]:8443"
			}
			if server.Addr != want {
				t.Fatalf("TLS bound %s, want %s", server.Addr, want)
			}
			tlsCfg.Mode = TLSModeCustom
			tlsCfg.HTTPPort = 8080
			_, redirect, err = SetupServers(tlsCfg, http.NotFoundHandler(), slog.Default())
			if err != nil || redirect == nil {
				t.Fatalf("explicit redirect: %v", err)
			}
			tlsCfg.Mode = TLSModeAuto
			tlsCfg.Domain = "aurago.example.com"
			tlsCfg.HTTPPort = 0
			server, redirect, err = SetupServers(tlsCfg, http.NotFoundHandler(), slog.Default())
			if err != nil || redirect != nil || server.Addr != want {
				t.Fatalf("auto mode ignored disabled HTTP or host: %v", err)
			}
		})
	}
}

func TestHTTPSRedirectPreservesAuthorityAndEscaping(t *testing.T) {
	for _, tc := range []struct {
		domain, request, want string
		port                  int
	}{
		{"", "http://localhost:8080/a%2Fb?q=x%2Fy", "https://localhost:8443/a%2Fb?q=x%2Fy", 8443},
		{"", "http://[::1]:8080/a?b=c", "https://[::1]:8443/a?b=c", 8443},
		{"aurago.example.com", "http://localhost:8080/a", "https://aurago.example.com/a", 443},
	} {
		rr := httptest.NewRecorder()
		httpRedirectHandler(tc.domain, tc.port).ServeHTTP(rr, httptest.NewRequest("GET", tc.request, nil))
		if rr.Code != http.StatusMovedPermanently || rr.Header().Get("Location") != tc.want {
			t.Fatalf("redirect: %d %s", rr.Code, rr.Header().Get("Location"))
		}
	}
}

func TestHTTPSDisableSaveClearsHSTSUntilRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server:\n  host: 127.0.0.1\n  port: 8088\n  https:\n    enabled: true\n    cert_mode: selfsigned\n    https_port: 8443\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.Default(), Vault: newSetupTestVault(t)}
	handler := securityHeadersMiddleware(s.httpsTransitionMiddleware(handleUpdateConfig(s)), true, false)
	req := httptest.NewRequest("PUT", "https://localhost:8443/api/config", strings.NewReader(`{"server":{"https":{"enabled":false}}}`))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("save: %d %s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Strict-Transport-Security") != "max-age=0" {
		t.Fatalf("save retained HSTS: %s", rr.Header().Get("Strict-Transport-Security"))
	}
	var result map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &result)
	if result["needs_restart"] != true || result["restart_url"] != "http://localhost:8088/config" {
		t.Fatalf("restart response: %v", result)
	}
	loaded, err := config.Load(path)
	if err != nil || loaded.Server.HTTPS.Enabled {
		t.Fatalf("disable not saved: %v", err)
	}
	for i := 0; i < 2; i++ {
		rr = httptest.NewRecorder()
		securityHeadersMiddleware(s.httpsTransitionMiddleware(http.NotFoundHandler()), true, false).ServeHTTP(rr, httptest.NewRequest("GET", "https://localhost:8443/api/config", nil))
		if rr.Header().Get("Strict-Transport-Security") != "max-age=0" {
			t.Fatal("refresh re-enabled HSTS")
		}
		if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatal("other security headers lost")
		}
	}
}

func TestConfigRestartURLPreservesProxyOrigin(t *testing.T) {
	old := &config.Config{}
	old.Server.Host = "127.0.0.1"
	old.Server.Port = 8088
	old.Server.HTTPS.Enabled = true
	old.Server.HTTPS.HTTPSPort = 8443
	next := *old
	next.Server.HTTPS.Enabled = false
	for _, raw := range []string{"https://public.example.com", "http://localhost:8088"} {
		req := httptest.NewRequest("PUT", raw+"/api/config", nil)
		if raw == "http://localhost:8088" {
			old.Server.HTTPS.BehindProxy = true
		}
		if got := configRestartURL(req, old, &next); got != "" {
			t.Fatalf("proxy redirected to %s", got)
		}
	}
	old.Server.HTTPS.BehindProxy = false
	req := httptest.NewRequest("PUT", "https://[::1]:8443/api/config", nil)
	if got := configRestartURL(req, old, &next); got != "http://[::1]:8088/config" {
		t.Fatalf("IPv6 URL: %s", got)
	}
}

func TestSetupHTTPSStatusIsReadOnlyAndSafe(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.HTTPSPort = 8443
	cfg.Server.HTTPS.KeyFile = "private-key-path"
	cfg.Server.HTTPS.CertFile = "private-cert-path"
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	rr := httptest.NewRecorder()
	handleSetupStatus(s).ServeHTTP(rr, httptest.NewRequest("GET", "https://localhost/api/setup/status", nil))
	var payload struct {
		HTTPS map[string]interface{} `json:"https"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.HTTPS) != 3 || payload.HTTPS["enabled"] != true || payload.HTTPS["port"] != float64(8443) || payload.HTTPS["cert_mode"] != "selfsigned" {
		t.Fatalf("HTTPS status: %v", payload.HTTPS)
	}
	if strings.Contains(rr.Body.String(), "private-") {
		t.Fatal("private config disclosed")
	}
}
