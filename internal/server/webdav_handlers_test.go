package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func TestWebDAVStatusReportsConfigurationStates(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.Config
		wantStatus string
	}{
		{
			name:       "disabled",
			cfg:        config.Config{},
			wantStatus: "disabled",
		},
		{
			name: "missing url",
			cfg: config.Config{
				WebDAV: struct {
					Enabled  bool   `yaml:"enabled"`
					ReadOnly bool   `yaml:"readonly"`
					AuthType string `yaml:"auth_type"`
					URL      string `yaml:"url"`
					Username string `yaml:"username"`
					Password string `yaml:"-" json:"-"`
					Token    string `yaml:"-" json:"-"`
				}{Enabled: true, AuthType: "basic", Username: "alice", Password: "secret"},
			},
			wantStatus: "no_url",
		},
		{
			name: "missing basic credentials",
			cfg: config.Config{
				WebDAV: struct {
					Enabled  bool   `yaml:"enabled"`
					ReadOnly bool   `yaml:"readonly"`
					AuthType string `yaml:"auth_type"`
					URL      string `yaml:"url"`
					Username string `yaml:"username"`
					Password string `yaml:"-" json:"-"`
					Token    string `yaml:"-" json:"-"`
				}{Enabled: true, AuthType: "basic", URL: "https://dav.example.test/root"},
			},
			wantStatus: "no_credentials",
		},
		{
			name: "missing bearer token",
			cfg: config.Config{
				WebDAV: struct {
					Enabled  bool   `yaml:"enabled"`
					ReadOnly bool   `yaml:"readonly"`
					AuthType string `yaml:"auth_type"`
					URL      string `yaml:"url"`
					Username string `yaml:"username"`
					Password string `yaml:"-" json:"-"`
					Token    string `yaml:"-" json:"-"`
				}{Enabled: true, AuthType: "bearer", URL: "https://dav.example.test/root"},
			},
			wantStatus: "no_credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{Cfg: &tt.cfg}
			rec := httptest.NewRecorder()
			handleWebDAVStatus(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/webdav/status", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			payload := decodeWebDAVProbePayload(t, rec)
			if payload["status"] != tt.wantStatus {
				t.Fatalf("status = %v, want %s; body=%s", payload["status"], tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestWebDAVTestReportsSuccessAndFailure(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	successUpstream := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Fatalf("method = %s, want PROPFIND", r.Method)
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/root/</d:href>
    <d:propstat>
      <d:prop><d:displayname>root</d:displayname><d:resourcetype><d:collection/></d:resourcetype></d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`)
	}))
	defer successUpstream.Close()

	successCfg := &config.Config{}
	successCfg.WebDAV.Enabled = true
	successCfg.WebDAV.AuthType = "basic"
	successCfg.WebDAV.URL = successUpstream.URL + "/root"
	successCfg.WebDAV.Username = "alice"
	successCfg.WebDAV.Password = "secret"

	rec := httptest.NewRecorder()
	handleWebDAVTest(&Server{Cfg: successCfg}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/webdav/test", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("success status code = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	payload := decodeWebDAVProbePayload(t, rec)
	if payload["status"] != "ok" {
		t.Fatalf("success payload = %#v, want ok", payload)
	}

	failureUpstream := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusBadGateway)
	}))
	defer failureUpstream.Close()

	failureCfg := &config.Config{}
	failureCfg.WebDAV.Enabled = true
	failureCfg.WebDAV.AuthType = "basic"
	failureCfg.WebDAV.URL = failureUpstream.URL
	failureCfg.WebDAV.Username = "alice"
	failureCfg.WebDAV.Password = "secret"

	rec = httptest.NewRecorder()
	handleWebDAVTest(&Server{Cfg: failureCfg}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/webdav/test", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("failure status code = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	payload = decodeWebDAVProbePayload(t, rec)
	if payload["status"] != "error" {
		t.Fatalf("failure payload = %#v, want error", payload)
	}
}

func decodeWebDAVProbePayload(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode probe payload %q: %v", rec.Body.String(), err)
	}
	return payload
}
