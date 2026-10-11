package config

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type probeListener struct {
	net.Listener
	closed bool
}

func TestHTTPSLegacyDefaultsAndExplicitZero(t *testing.T) {
	for _, tc := range []struct {
		yaml     string
		enabled  bool
		httpPort int
	}{
		{"server: {}", false, 0},
		{"server:\n  https:\n    enabled: true\n    domain: aurago.example.com", true, 80},
		{"server:\n  https:\n    enabled: true\n    domain: aurago.example.com\n    http_port: 0", true, 0},
		{"server:\n  https:\n    enabled: true\n    domain: aurago.example.com\n    http_port: null", true, 80},
		{"server:\n  https:\n    enabled: true\n    cert_mode: selfsigned\n    domain: aurago.example.com", true, 0},
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(tc.yaml), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Server.HTTPS.Enabled != tc.enabled || cfg.Server.HTTPS.HTTPPort != tc.httpPort {
			t.Fatalf("%s: %+v", tc.yaml, cfg.Server.HTTPS)
		}
		data, _ := os.ReadFile(path)
		if string(data) != tc.yaml {
			t.Fatal("loading rewrote config")
		}
	}
}

func (l *probeListener) Close() error { l.closed = true; return nil }

func TestSelectInitialHTTPSPort(t *testing.T) {
	for _, tc := range []struct {
		name, host string
		failures   int
		want       int
		addresses  []string
	}{
		{"standard", "127.0.0.1", 0, 443, []string{"127.0.0.1:443"}},
		{"busy", "0.0.0.0", 1, 8443, []string{"0.0.0.0:443", "0.0.0.0:8443"}},
		{"permission denied", "::1", 1, 8443, []string{"[::1]:443", "[::1]:8443"}},
		{"both blocked", "127.0.0.1", 2, 0, []string{"127.0.0.1:443", "127.0.0.1:8443"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var addresses []string
			ln := &probeListener{}
			port, err := selectInitialHTTPSPort(tc.host, func(network, addr string) (net.Listener, error) {
				addresses = append(addresses, addr)
				if len(addresses) <= tc.failures {
					return nil, errors.New(tc.name)
				}
				return ln, nil
			})
			if port != tc.want || !reflect.DeepEqual(addresses, tc.addresses) {
				t.Fatalf("port=%d addresses=%v err=%v", port, addresses, err)
			}
			if tc.want == 0 {
				if err == nil || !strings.Contains(err.Error(), "8443") {
					t.Fatalf("missing actionable error: %v", err)
				}
			} else if err != nil || !ln.closed {
				t.Fatalf("probe not released: %v", err)
			}
		})
	}
}
