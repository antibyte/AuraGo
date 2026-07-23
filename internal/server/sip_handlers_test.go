package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/sipphone"
)

func TestSIPConfigResponseMasksPassword(t *testing.T) {
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	sipCfg.Password = "super-secret-password"
	server := &Server{Cfg: &config.Config{SIP: sipCfg}}
	recorder := httptest.NewRecorder()
	handleSIPConfig(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sip/config", nil))
	if strings.Contains(recorder.Body.String(), sipCfg.Password) {
		t.Fatal("SIP password leaked in config response")
	}
	var payload sipConfigPayload
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.PasswordSet || payload.Password != "" {
		t.Fatalf("unexpected secret mask state: %+v", payload)
	}
}

func TestSIPProviderCatalogIsPubliclySecretFree(t *testing.T) {
	recorder := httptest.NewRecorder()
	handleSIPProviders().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sip/providers", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Providers []sipphone.SIPProviderPreset `json:"providers"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Providers) < 50 {
		t.Fatalf("provider catalog is unexpectedly small: %d", len(payload.Providers))
	}
	if strings.Contains(strings.ToLower(recorder.Body.String()), `"password":"`) {
		t.Fatal("provider catalog exposed a password value")
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("provider catalog must not be cached")
	}
}

func TestMarshalConfigWithSIPNeverWritesRuntimePassword(t *testing.T) {
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	sipCfg.Password = "must-not-reach-yaml"
	output, err := marshalConfigWithSIP([]byte("agent:\n  system_language: de\n"), sipCfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), sipCfg.Password) || strings.Contains(string(output), "password:") {
		t.Fatalf("secret leaked into YAML: %s", output)
	}
	if !strings.Contains(string(output), "sip:") || !strings.Contains(string(output), "readonly: true") {
		t.Fatalf("SIP block missing: %s", output)
	}
}

func TestSIPConfigMutationRequiresSameOrigin(t *testing.T) {
	server := &Server{Cfg: &config.Config{}}
	request := httptest.NewRequest(http.MethodPut, "https://aurago.local/api/sip/config", strings.NewReader(`{}`))
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handleSIPConfig(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSIPSetupMutationRequiresSameOrigin(t *testing.T) {
	server := &Server{Cfg: &config.Config{}}
	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/setup", strings.NewReader(`{"provider_id":"fritzbox"}`))
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()
	handleSIPSetup(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSIPSetupRejectsTrailingJSON(t *testing.T) {
	server := &Server{Cfg: &config.Config{}}
	body := `{"provider_id":"fritzbox","values":{"server":"fritz.box","username":"desk"}} {}`
	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/setup", strings.NewReader(body))
	request.Header.Set("Origin", "https://aurago.local")
	recorder := httptest.NewRecorder()
	handleSIPSetup(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if sipConfigHasAccount(sipConfigSnapshot(server)) {
		t.Fatal("trailing JSON must be rejected before changing SIP configuration")
	}
}

func TestSIPSetupAppliesPresetAndStoresPasswordOnlyInVault(t *testing.T) {
	template, err := os.ReadFile(filepath.Join("..", "..", "config_template.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(configPath, template, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loaded.ConfigPath = configPath
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tempDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{Cfg: loaded, Vault: vault}
	body := `{"provider_id":"fritzbox","values":{"server":"fritz.box","username":"aurago-phone","display_name":"AuraGo"},"password":" setup-secret "}`
	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/setup", strings.NewReader(body))
	request.Header.Set("Origin", "https://aurago.local")
	recorder := httptest.NewRecorder()
	handleSIPSetup(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	snapshot := sipConfigSnapshot(server)
	if snapshot.PresetID != "fritzbox" || snapshot.Registrar != "fritz.box" || !snapshot.Enabled || !snapshot.ReadOnly {
		t.Fatalf("unexpected saved preset: %+v", snapshot)
	}
	if snapshot.Permissions.AnswerInbound || snapshot.Permissions.OriginateOutbound {
		t.Fatalf("setup granted call permissions: %+v", snapshot.Permissions)
	}
	secret, err := vault.ReadSecret(config.SIPPasswordVaultKey)
	if err != nil || secret != " setup-secret " {
		t.Fatalf("SIP password not stored in Vault: value=%q err=%v", secret, err)
	}
	savedYAML, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(savedYAML), "setup-secret") {
		t.Fatal("SIP password leaked into config.yaml")
	}
}

func TestSIPSetupRequiresNewPasswordWhenProviderChanges(t *testing.T) {
	var current config.SIPConfig
	config.ApplySIPDefaults(&current)
	current.PresetID = "sipgate-de"
	current.Registrar = "sipgate.de"
	current.Username = "old-user"
	current.Password = "old-password"
	server := &Server{Cfg: &config.Config{SIP: current}}
	body := `{"provider_id":"fritzbox","confirm_replace":true,"values":{"server":"fritz.box","username":"new-user"}}`
	request := httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/setup", strings.NewReader(body))
	request.Header.Set("Origin", "https://aurago.local")
	recorder := httptest.NewRecorder()
	handleSIPSetup(server).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSIPAPIRoutesAreAdminProtected(t *testing.T) {
	for _, path := range []string{
		"/api/sip/config",
		"/api/sip/providers",
		"/api/sip/setup",
		"/api/sip/status",
		"/api/sip/calls",
		"/api/sip/events",
		"/api/sip/app/state",
		"/api/sip/browser-media/sessions",
	} {
		if !isAdminProtectedPath(path) {
			t.Fatalf("%s is not administrator protected", path)
		}
	}
}

func TestSIPAppStateReportsDisabledOutboundCalling(t *testing.T) {
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	sipCfg.Enabled = true
	sipCfg.ReadOnly = false
	sipCfg.Registrar = "pbx.example"
	sipCfg.Domain = "pbx.example"
	sipCfg.Username = "desk"
	manager, err := sipphone.NewManager(sipCfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = manager.Close()
	})

	server := &Server{SIPPhone: manager}
	recorder := httptest.NewRecorder()
	handleSIPAppState(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/sip/app/state", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload struct {
		Blockers     []string        `json:"blockers"`
		Capabilities map[string]bool `json:"capabilities"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, blocker := range payload.Blockers {
		if blocker == "outbound_disabled" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing outbound_disabled blocker: %+v", payload.Blockers)
	}
	if payload.Capabilities["dial"] {
		t.Fatal("dial capability must remain disabled without explicit outbound permission")
	}
}

func TestSIPBrowserMediaRejectsBearerAndCrossOrigin(t *testing.T) {
	server := newSIPBrowserHandlerTestServer(t)
	mux := http.NewServeMux()
	registerSIPHandlers(mux, server)

	bearer := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/sip/browser-media/sessions", strings.NewReader(`{}`))
	bearer.Header.Set("Authorization", "Bearer admin-token")
	bearerRecorder := httptest.NewRecorder()
	mux.ServeHTTP(bearerRecorder, bearer)
	if bearerRecorder.Code != http.StatusForbidden {
		t.Fatalf("bearer status=%d body=%s", bearerRecorder.Code, bearerRecorder.Body.String())
	}

	crossOrigin := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/sip/browser-media/sessions", strings.NewReader(`{}`))
	crossOrigin.Header.Set("Origin", "https://attacker.example")
	crossOriginRecorder := httptest.NewRecorder()
	mux.ServeHTTP(crossOriginRecorder, crossOrigin)
	if crossOriginRecorder.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status=%d body=%s", crossOriginRecorder.Code, crossOriginRecorder.Body.String())
	}

	missingOrigin := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/sip/browser-media/sessions", strings.NewReader(`{}`))
	missingOriginRecorder := httptest.NewRecorder()
	mux.ServeHTTP(missingOriginRecorder, missingOrigin)
	if missingOriginRecorder.Code != http.StatusForbidden {
		t.Fatalf("missing-origin status=%d body=%s", missingOriginRecorder.Code, missingOriginRecorder.Body.String())
	}
}

func TestSIPBrowserMediaRequiresExactOrigin(t *testing.T) {
	tests := []struct {
		name    string
		request *http.Request
		origin  string
		want    bool
	}{
		{
			name:    "same HTTPS origin",
			request: httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/browser-media/sessions", nil),
			origin:  "https://aurago.local",
			want:    true,
		},
		{
			name:    "scheme downgrade",
			request: httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/browser-media/sessions", nil),
			origin:  "http://aurago.local",
			want:    false,
		},
		{
			name:    "different port",
			request: httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/browser-media/sessions", nil),
			origin:  "https://aurago.local:8443",
			want:    false,
		},
		{
			name:    "origin with path",
			request: httptest.NewRequest(http.MethodPost, "https://aurago.local/api/sip/browser-media/sessions", nil),
			origin:  "https://aurago.local/path",
			want:    false,
		},
		{
			name: "trusted forwarded request origin",
			request: func() *http.Request {
				request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/sip/browser-media/sessions", nil)
				request.Header.Set("X-Forwarded-Proto", "https")
				request.Header.Set("X-Forwarded-Host", "aurago.local")
				return request
			}(),
			origin: "https://aurago.local",
			want:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.request.Header.Set("Origin", test.origin)
			if got := sameOriginSIPBrowserRequest(test.request); got != test.want {
				t.Fatalf("sameOriginSIPBrowserRequest()=%v want %v", got, test.want)
			}
		})
	}
}

func TestSIPBrowserMediaHonorsCurrentConfiguration(t *testing.T) {
	server := newSIPBrowserHandlerTestServer(t)
	server.Cfg.SIP.BrowserMedia.Enabled = false
	mux := http.NewServeMux()
	registerSIPHandlers(mux, server)

	request := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/sip/browser-media/sessions", strings.NewReader(`{"client_id":"client-12345678","offer_sdp":"not-reached"}`))
	request.Header.Set("Origin", "http://aurago.local")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled browser media status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSIPBrowserMediaRejectsStaleRuntimeConfiguration(t *testing.T) {
	server := newSIPBrowserHandlerTestServer(t)
	server.Cfg.SIP.BrowserMedia.UDPPort++
	recorder := httptest.NewRecorder()
	if service, unavailable := sipBrowserMedia(server, recorder); !unavailable || service != nil {
		t.Fatal("stale browser media runtime remained available")
	}
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("stale runtime status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSIPBrowserMediaRestartDetection(t *testing.T) {
	var old config.SIPConfig
	config.ApplySIPDefaults(&old)
	old.Enabled = true
	old.BindHost = "127.0.0.1"
	old.BrowserMedia.Enabled = true

	tests := []struct {
		name string
		edit func(*config.SIPConfig)
		want bool
	}{
		{name: "unchanged", edit: func(*config.SIPConfig) {}, want: false},
		{name: "unrelated", edit: func(cfg *config.SIPConfig) { cfg.HistoryRetentionDays++ }, want: false},
		{name: "disable", edit: func(cfg *config.SIPConfig) { cfg.BrowserMedia.Enabled = false }, want: true},
		{name: "port", edit: func(cfg *config.SIPConfig) { cfg.BrowserMedia.UDPPort++ }, want: true},
		{name: "inherited bind", edit: func(cfg *config.SIPConfig) { cfg.BindHost = "127.0.0.2" }, want: true},
		{name: "explicit bind ignores signaling bind", edit: func(cfg *config.SIPConfig) {
			cfg.BrowserMedia.BindHost = "127.0.0.1"
			cfg.BindHost = "127.0.0.2"
		}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next := old
			test.edit(&next)
			if got := sipBrowserMediaRestartRequired(old, next); got != test.want {
				t.Fatalf("sipBrowserMediaRestartRequired()=%v want %v", got, test.want)
			}
		})
	}
}

func TestSIPBrowserMediaRejectsOversizedAndInvalidSDPWithoutEcho(t *testing.T) {
	server := newSIPBrowserHandlerTestServer(t)
	mux := http.NewServeMux()
	registerSIPHandlers(mux, server)

	marker := "private-ice-credential"
	invalidBody := `{"client_id":"client-12345678","offer_sdp":"` + marker + `"}`
	invalid := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/sip/browser-media/sessions", strings.NewReader(invalidBody))
	invalid.Header.Set("Origin", "http://aurago.local")
	invalidRecorder := httptest.NewRecorder()
	mux.ServeHTTP(invalidRecorder, invalid)
	if invalidRecorder.Code != http.StatusBadGateway {
		t.Fatalf("invalid SDP status=%d body=%s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
	if strings.Contains(invalidRecorder.Body.String(), marker) {
		t.Fatal("invalid SDP was echoed in the error response")
	}

	oversizedBody := `{"client_id":"client-12345678","offer_sdp":"` + strings.Repeat("x", sipBrowserSDPBodyLimit) + `"}`
	oversized := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/sip/browser-media/sessions", strings.NewReader(oversizedBody))
	oversized.Header.Set("Origin", "http://aurago.local")
	oversizedRecorder := httptest.NewRecorder()
	mux.ServeHTTP(oversizedRecorder, oversized)
	if oversizedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("oversized SDP status=%d body=%s", oversizedRecorder.Code, oversizedRecorder.Body.String())
	}
}

func TestSIPBrowserMediaRejectsForeignSessionAndRateLimits(t *testing.T) {
	server := newSIPBrowserHandlerTestServer(t)
	mux := http.NewServeMux()
	registerSIPHandlers(mux, server)

	foreign := httptest.NewRequest(http.MethodDelete, "http://aurago.local/api/sip/browser-media/sessions/not-owned", nil)
	foreign.Header.Set("Origin", "http://aurago.local")
	foreign.Header.Set(sipBrowserClientIDHeader, "client-12345678")
	foreignRecorder := httptest.NewRecorder()
	mux.ServeHTTP(foreignRecorder, foreign)
	if foreignRecorder.Code != http.StatusNotFound {
		t.Fatalf("foreign session status=%d body=%s", foreignRecorder.Code, foreignRecorder.Body.String())
	}

	rateServer := newSIPBrowserHandlerTestServer(t)
	rateMux := http.NewServeMux()
	registerSIPHandlers(rateMux, rateServer)
	for i := 0; i <= sipBrowserMediaRateLimit; i++ {
		request := httptest.NewRequest(http.MethodPost, "http://aurago.local/api/sip/browser-media/sessions", strings.NewReader(`{}`))
		request.Header.Set("Origin", "http://aurago.local")
		recorder := httptest.NewRecorder()
		rateMux.ServeHTTP(recorder, request)
		if i == sipBrowserMediaRateLimit && recorder.Code != http.StatusTooManyRequests {
			t.Fatalf("rate-limit status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
}

func newSIPBrowserHandlerTestServer(t *testing.T) *Server {
	t.Helper()
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	sipCfg.Enabled = true
	sipCfg.BindHost = "127.0.0.1"
	sipCfg.BrowserMedia.Enabled = true
	sipCfg.BrowserMedia.BindHost = "127.0.0.1"
	sipCfg.BrowserMedia.UDPPort = 0
	service, err := sipphone.NewBrowserMediaService(sipCfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})
	return &Server{
		Cfg:             &config.Config{SIP: sipCfg},
		SIPBrowserMedia: service,
	}
}
