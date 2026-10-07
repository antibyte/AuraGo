package tools

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// FF1: when a redirect to another host, port or scheme made api_request drop the caller's
// headers, the result says so (headers_dropped_on_redirect, final_url), so the agent can
// call the final URL directly; a Debug line names the final host. A same-origin redirect,
// a chain without caller headers to drop and a request without a redirect report nothing.
func TestFF1APIRequestReportsDroppedHeaders(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	ConfigureRuntimePermissions(RuntimePermissions{AllowNetworkRequests: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()
	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/away":
			http.Redirect(w, r, target.URL+"/landing?x=1", http.StatusFound)
		case "/here":
			http.Redirect(w, r, "/landing", http.StatusFound)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer start.Close()
	secret := map[string]string{"X-API-Key": "ff1-key"}

	out := c13APIRequest(t, start.URL+"/away", secret)
	if out.Status != "success" || !out.HeadersDroppedOnRedirect || out.FinalURL != target.URL+"/landing?x=1" {
		t.Fatalf("redirect to another origin = %+v", out)
	}
	if !strings.Contains(logs.String(), "level=DEBUG") || !strings.Contains(logs.String(), "headers_dropped") ||
		strings.Contains(logs.String(), "ff1-key") {
		t.Fatalf("debug log = %s", logs.String())
	}

	for name, call := range map[string]func() APIResult{
		"same origin": func() APIResult { return c13APIRequest(t, start.URL+"/here", secret) },
		"no redirect": func() APIResult { return c13APIRequest(t, start.URL+"/plain", secret) },
		"nothing to drop": func() APIResult {
			return c13APIRequest(t, start.URL+"/away", map[string]string{"Accept": "application/json"})
		},
		"no headers": func() APIResult { return c13APIRequest(t, start.URL+"/away", nil) },
	} {
		if got := call(); got.Status != "success" || got.HeadersDroppedOnRedirect || got.FinalURL != "" {
			t.Errorf("%s = %+v", name, got)
		}
	}
}
