package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
)

func TestHTTPServerErrorLogLevels(t *testing.T) {
	for _, tc := range []struct {
		name, message, want string
		level               slog.Level
	}{
		{"EOF at normal level", "http: TLS handshake error from 192.0.2.1:1234: EOF", "", slog.LevelInfo},
		{"EOF at debug level", "http: TLS handshake error from [::1]:1234: EOF", "DEBUG", slog.LevelDebug},
		{"truncated handshake", "http: TLS handshake error from 192.0.2.1:1234: unexpected EOF", "ERROR", slog.LevelInfo},
		{"certificate rejection", "http: TLS handshake error from 192.0.2.1:1234: remote error: tls: bad certificate", "ERROR", slog.LevelInfo},
		{"other EOF", "http: proxy error: EOF", "ERROR", slog.LevelInfo},
		{"handler panic", "http: panic serving 192.0.2.1:1234: fixture failure", "ERROR", slog.LevelInfo},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: tc.level}))
			newHTTPServerErrorLog(logger).Print(tc.message)
			if tc.want == "" {
				if output.Len() != 0 {
					t.Fatalf("normal log contains a client EOF: %s", &output)
				}
				return
			}
			var record struct{ Level, Msg string }
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if record.Level != tc.want || record.Msg != tc.message {
				t.Fatalf("log = %+v; want %s %q", record, tc.want, tc.message)
			}
		})
	}
}

func TestHTTPServerErrorLogConfiguredForEveryTLSMode(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	for _, mode := range []TLSMode{TLSModeSelfSigned, TLSModeCustom, TLSModeAuto} {
		for _, httpPort := range []int{0, 80} {
			cfg := &TLSConfig{Mode: mode, Host: "127.0.0.1", Domain: "example.test", HTTPSPort: 8443, HTTPPort: httpPort,
				CertDir: dir, CertFile: filepath.Join(dir, "cert.pem"), KeyFile: filepath.Join(dir, "key.pem")}
			https, redirect, err := SetupServers(cfg, http.NotFoundHandler(), logger)
			if err != nil {
				t.Fatal(err)
			}
			if https.ErrorLog == nil || (redirect != nil && redirect.ErrorLog != https.ErrorLog) {
				t.Fatalf("mode %v, HTTP port %d: TLS or redirect error logger is missing", mode, httpPort)
			}
		}
	}
}
