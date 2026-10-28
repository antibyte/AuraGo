package tools

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func TestGotenbergDoesNotReplayConversionRedirects(t *testing.T) {
	var uploads, replays atomic.Int64
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if testutil.ServeGotenbergDebug(w, r) {
			return
		}
		if r.URL.Path == "/replayed" {
			replays.Add(1)
			_, _ = io.WriteString(w, "%PDF-replayed")
			return
		}
		uploads.Add(1)
		http.Redirect(w, r, "/replayed", http.StatusTemporaryRedirect)
	}))
	defer sidecar.Close()
	_, err := gotenbergRequest(t.Context(), &config.GotenbergConfig{URL: sidecar.URL, Timeout: 2}, "/forms/chromium/convert/html", nil, map[string][]byte{"index.html": []byte("hello")})
	if err == nil || uploads.Load() != 1 || replays.Load() != 0 {
		t.Fatalf("error=%v uploads=%d replays=%d", err, uploads.Load(), replays.Load())
	}
}

func TestGotenbergSecurityRefusesUnsafeDiagnosticsBeforeUpload(t *testing.T) {
	for _, tc := range []struct {
		name, flag string
		value      any
		version    string
	}{
		{name: "missing flag", flag: "chromium-deny-private-ips", value: nil},
		{name: "private destinations", flag: "libreoffice-deny-private-ips", value: "false"},
		{name: "metadata fetches", flag: "api-download-from-deny-private-ips", value: "false"},
		{name: "webhook fetches", flag: "webhook-deny-private-ips", value: "false"},
		{name: "wrong type", flag: "chromium-deny-private-ips", value: true},
		{name: "allowlist bypass", flag: "chromium-allow-list", value: "[^https://trusted]"},
		{name: "resolver bypass", flag: "chromium-host-resolver-rules", value: "MAP * 127.0.0.1"},
		{name: "proxy bypass", flag: "chromium-proxy-server", value: "http://private-proxy"},
		{name: "environment proxy", flag: "webhook-enable-environment-proxy", value: "true"},
		{name: "disabled web security", flag: "chromium-disable-web-security", value: "true"},
		{name: "shared browser", flag: "chromium-shared-browser-context", value: "true"},
		{name: "foreign files", flag: "chromium-allow-file-access-from-files", value: "true"},
		{name: "old version", version: "8.37.9"}, {name: "new major", version: "9.0.0"},
		{name: "prerelease", version: "8.38.0-rc1"}, {name: "malformed version", version: "8.38.0.secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var diagnostic map[string]any
			if err := json.Unmarshal([]byte(testutil.GotenbergDebugResponse), &diagnostic); err != nil {
				t.Fatal(err)
			}
			flags := diagnostic["flags"].(map[string]any)
			if tc.flag != "" {
				if tc.value == nil {
					delete(flags, tc.flag)
				} else {
					flags[tc.flag] = tc.value
				}
			}
			if tc.version != "" {
				diagnostic["version"] = tc.version
			}
			diagnostic["unrelated_secret"] = "diagnostic-secret-marker"
			data, _ := json.Marshal(diagnostic)
			var uploads atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					uploads.Add(1)
					t.Error("unverified renderer received an upload")
				}
				_, _ = w.Write(data)
			}))
			defer srv.Close()
			_, err := gotenbergRequest(t.Context(), &config.GotenbergConfig{URL: srv.URL, Timeout: 2}, "/forms/chromium/convert/html", nil, map[string][]byte{"index.html": []byte("private document")})
			if err == nil || uploads.Load() != 0 || strings.Contains(err.Error(), "diagnostic-secret-marker") {
				t.Fatalf("unsafe diagnostic result: %v, uploads=%d", err, uploads.Load())
			}
		})
	}
}

func TestGotenbergSecurityDiagnosticsAreBoundedAndPrivate(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"absent", "private-error-marker", 404}, {"invalid", "private-error-marker", 200},
		{"oversized", strings.Repeat("private-error-marker", gotenbergDiagnosticLimit/20+1), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/debug" {
					t.Error("unverified renderer received an upload")
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			_, err := verifyGotenbergSecurity(t.Context(), &config.GotenbergConfig{URL: srv.URL, Timeout: 2})
			if err == nil || strings.Contains(err.Error(), "private-error-marker") {
				t.Fatalf("diagnostics leaked or passed: %v", err)
			}
		})
	}
}

func TestGotenbergSecurityIsCheckedBeforeEveryPost(t *testing.T) {
	var diagnostics, uploads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/debug" {
			body := testutil.GotenbergDebugResponse
			if diagnostics.Add(1) > 1 {
				body = strings.Replace(body, `"chromium-deny-private-ips":"true"`, `"chromium-deny-private-ips":"false"`, 1)
			}
			_, _ = io.WriteString(w, body)
			return
		}
		uploads.Add(1)
		_, _ = io.WriteString(w, "%PDF-fixture")
	}))
	defer srv.Close()
	cfg := &config.GotenbergConfig{URL: srv.URL, Timeout: 2}
	if _, err := gotenbergRequest(t.Context(), cfg, "/render", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := gotenbergRequest(t.Context(), cfg, "/render", nil, nil); err == nil {
		t.Fatal("cached a positive security result")
	}
	if diagnostics.Load() != 2 || uploads.Load() != 1 {
		t.Fatalf("checks=%d uploads=%d", diagnostics.Load(), uploads.Load())
	}
}

func TestDocumentCreatorAdminRemotePolicy(t *testing.T) {
	for _, operation := range []string{"html_to_pdf", "markdown_to_pdf", "screenshot_html", "create_pdf"} {
		for _, allow := range []bool{false, true} {
			for _, block := range []bool{false, true} {
				t.Run(operation+"/allow="+strconv.FormatBool(allow)+"/block="+strconv.FormatBool(block), func(t *testing.T) {
					cfg, calls := c102FakeGotenberg(t)
					cfg.AllowRemoteResources = allow
					out := ExecuteDocumentCreator(t.Context(), cfg, operation, "Title", `<img src="https://example.com/x">`, "", "", "A4", false, "", "", DocumentCreatorOptions{BlockRemoteContent: block})
					if c102DecodeResult(t, out)["status"] != "success" {
						t.Fatal(out)
					}
					if len(calls()) != 1 {
						t.Fatal("missing conversion")
					}
					blocked := strings.Contains(calls()[0].files["index.html"], "Content-Security-Policy")
					if blocked != (!allow || block) {
						t.Fatalf("allow=%v block=%v policy=%v", allow, block, blocked)
					}
				})
			}
		}
	}
}

func TestDocumentCreatorAdminPolicyKeepsURLCapturesAvailable(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")
	for _, operation := range []string{"url_to_pdf", "screenshot_url"} {
		cfg, calls := c102FakeGotenberg(t)
		out := ExecuteDocumentCreator(t.Context(), cfg, operation, "", "", cfg.Gotenberg.URL, "", "A4", false, "", "")
		if c102DecodeResult(t, out)["status"] != "success" || len(calls()) != 1 {
			t.Fatal(out)
		}
	}
}

func TestGotenbergHealthDistinguishesReachableAndVerified(t *testing.T) {
	var safe atomic.Bool
	safe.Store(true)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_, _ = io.WriteString(w, `{"status":"up"}`)
			return
		}
		if safe.Load() {
			testutil.ServeGotenbergDebug(w, r)
			return
		}
		http.Error(w, "private-error-marker", 404)
	}))
	defer srv.Close()
	for _, verified := range []bool{true, false} {
		safe.Store(verified)
		out := c102DecodeResult(t, GotenbergHealth(t.Context(), &config.GotenbergConfig{URL: srv.URL, Timeout: 2}))
		if out["reachable"] != true || out["security_verified"] != verified {
			t.Fatal(out)
		}
	}
}

func TestGotenbergPDFResponseLimitExplainsTheOutputSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if testutil.ServeGotenbergDebug(w, r) {
			return
		}
		_, _ = io.WriteString(w, strings.Repeat("x", int(maxHTTPResponseSize)+1))
	}))
	defer srv.Close()
	_, err := gotenbergRequest(t.Context(), &config.GotenbergConfig{URL: srv.URL, Timeout: 2}, "/render", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "PDF/image output (maximum 10 MiB)") {
		t.Fatal(err)
	}
}
