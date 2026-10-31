package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ldap"
	"aurago/internal/testutil"
)

// Review fixes for config UX wave 1 (batches 1-3).

func countingServer(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// C1: provider keys must not follow a changed YepAPI base URL.
func TestHandleYepAPITestRefusesChangedBaseURLBeforeResolvingKeys(t *testing.T) {
	saved, savedHits := countingServer(t)
	other, otherHits := countingServer(t)

	cfg := &config.Config{}
	cfg.YepAPI.Enabled = true
	cfg.YepAPI.BaseURL = saved.URL
	cfg.YepAPI.Provider = "saved"
	cfg.Providers = []config.ProviderEntry{{ID: "saved", Type: "yepapi", APIKey: "saved-key"}}
	s := &Server{Cfg: cfg, Logger: testServerLogger()}

	rec := postDraft(handleYepAPITest(s), "/api/yepapi/test", `{"draft":{"yepapi":{"base_url":"`+other.URL+`"}}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"error"`) || strings.Contains(rec.Body.String(), "saved-key") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	if atomic.LoadInt32(otherHits) != 0 || atomic.LoadInt32(savedHits) != 0 {
		t.Fatalf("upstream was reached: other=%d saved=%d", *otherHits, *savedHits)
	}

	// Same target (the default spelled out) still works with the saved provider key.
	rec = postDraft(handleYepAPITest(s), "/api/yepapi/test", `{"draft":{"yepapi":{"base_url":"`+saved.URL+`/"}}}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("unchanged target: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

// I2: downgrading the LDAP transport counts as a target change.
func TestHandleLDAPTestRefusesTLSDowngradeWithMaskedPassword(t *testing.T) {
	for name, draft := range map[string]string{
		"use_tls":  `{"use_tls":false,"bind_password":"••••••••"}`,
		"tls_mode": `{"tls_mode":"plain","bind_password":"••••••••"}`,
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.LDAP.Enabled = true
			cfg.LDAP.Host = "ldap.example.com"
			cfg.LDAP.BindDN = "cn=svc,dc=example,dc=com"
			cfg.LDAP.BindPassword = "saved-ldap-secret"
			cfg.LDAP.UseTLS = true
			cfg.LDAP.TLSMode = "ldaps"

			called := false
			orig := newLDAPTestClient
			newLDAPTestClient = func(ldap.LDAPConfig) ldapTestClient {
				called = true
				return &fakeLDAPHandlerClient{}
			}
			defer func() { newLDAPTestClient = orig }()

			s := &Server{Cfg: cfg}
			rec := postDraft(handleLDAPTest(s), "/api/ldap/test", `{"draft":{"ldap":`+draft+`}}`)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			if called {
				t.Fatal("the LDAP client was created with the saved password")
			}
			if strings.Contains(rec.Body.String(), "saved-ldap-secret") {
				t.Fatalf("body leaks the password: %s", rec.Body.String())
			}
		})
	}
}

// I3: the MQTT broker password is saved verbatim, other secrets are trimmed.
func TestExtractRecursiveKeepsMQTTPasswordWhitespace(t *testing.T) {
	collected := draftSecretCollector{}
	patch := map[string]interface{}{
		"mqtt": map[string]interface{}{"password": "  pass word \n"},
		"ldap": map[string]interface{}{"bind_password": "  trimmed \n"},
	}
	if err := extractRecursive(patch, "", collected, discardLogger()); err != nil {
		t.Fatal(err)
	}
	if got := collected["mqtt_password"]; got != "  pass word \n" {
		t.Fatalf("mqtt password = %q, want it untouched", got)
	}
	if got := collected["ldap_bind_password"]; got != "trimmed" {
		t.Fatalf("ldap password = %q, want trimmed", got)
	}
	if _, left := patch["mqtt"].(map[string]interface{})["password"]; left {
		t.Fatal("the password must be removed from the patch")
	}

	blank := draftSecretCollector{}
	_ = extractRecursive(map[string]interface{}{"mqtt": map[string]interface{}{"password": "   "}}, "", blank, discardLogger())
	_ = extractRecursive(map[string]interface{}{"mqtt": map[string]interface{}{"password": "••••••••"}}, "", blank, discardLogger())
	if len(blank) != 0 {
		t.Fatalf("blank or masked passwords must not be stored: %v", blank)
	}
}

func TestDraftTestConfigKeepsMQTTPasswordWhitespace(t *testing.T) {
	s := newDraftTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/mqtt/test", strings.NewReader(`{"draft":{"mqtt":{"password":"  draft-only mqtt pw Zq7x  "}}}`))
	cfg, err := s.draftTestConfig(req)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MQTT.Password != "  draft-only mqtt pw Zq7x  " {
		t.Fatalf("draft mqtt password = %q", cfg.MQTT.Password)
	}
}

// I4: saved provider keys must not be used for another account or gateway.
func TestHandleAIGatewayTestRefusesChangedAccountOrGateway(t *testing.T) {
	for name, draft := range map[string]string{
		"account": `{"ai_gateway":{"account_id":"other-acct"}}`,
		"gateway": `{"ai_gateway":{"gateway_id":"other-gw"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var hits int32
			oldTransport := http.DefaultTransport
			http.DefaultTransport = aiGatewayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&hits, 1)
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = oldTransport })

			s := &Server{Cfg: &config.Config{}, Logger: testServerLogger()}
			s.Cfg.LLM.Provider = "main"
			s.Cfg.AIGateway.Enabled = true
			s.Cfg.AIGateway.AccountID = "acct"
			s.Cfg.AIGateway.GatewayID = "gw"
			s.Cfg.Providers = []config.ProviderEntry{{ID: "router", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: "provider-secret"}}

			rec := postDraft(handleAIGatewayTest(s), "/api/ai-gateway/test", `{"provider_id":"router","draft":`+draft+`}`)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			var body map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "error" || body["message"] == "" {
				t.Fatalf("body = %s", rec.Body.String())
			}
			if atomic.LoadInt32(&hits) != 0 {
				t.Fatal("the gateway was probed with the saved provider key")
			}
			if strings.Contains(rec.Body.String(), "provider-secret") {
				t.Fatalf("body leaks the key: %s", rec.Body.String())
			}
		})
	}
}

// Restored batch 2 coverage on the draft path: web_port 0 and manifest routing.
func TestDraftTestConfigAllowsFritzBoxWebPortZero(t *testing.T) {
	s := newDraftTestServer(t)
	s.Cfg.FritzBox.WebPort = 8443
	s.initConfigSnapshot()
	req := httptest.NewRequest(http.MethodPost, "/api/fritzbox/test", strings.NewReader(`{"draft":{"fritzbox":{"web_port":0,"password":"typed"}}}`))
	cfg, err := s.draftTestConfig(req)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.FritzBox.WebPort != 0 {
		t.Fatalf("web_port = %d, want 0", cfg.FritzBox.WebPort)
	}
}

func TestDraftTestConfigAppliesManifestRoutingFalseAndNormalizesMode(t *testing.T) {
	s := newDraftTestServer(t)
	s.Cfg.Manifest.Routing.Enabled = true
	s.Cfg.Manifest.Routing.SpecificityMode = "fixed"
	s.Cfg.Manifest.Routing.Specificity = "coding"
	s.initConfigSnapshot()
	body := `{"draft":{"manifest":{"routing":{"enabled":false,"specificity_mode":"surprise","specificity":"root_shell","headers":{"x-aurago-task":"coding"}}}}}`
	cfg, err := s.draftTestConfig(httptest.NewRequest(http.MethodPost, "/api/manifest/test", strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	config.NormalizeManifestRoutingConfig(&cfg.Manifest.Routing)
	if cfg.Manifest.Routing.Enabled {
		t.Fatal("manifest.routing.enabled = true, want false")
	}
	if cfg.Manifest.Routing.SpecificityMode != "off" || cfg.Manifest.Routing.Specificity != "" {
		t.Fatalf("mode/specificity = %q/%q, want off/empty", cfg.Manifest.Routing.SpecificityMode, cfg.Manifest.Routing.Specificity)
	}
	if got := cfg.Manifest.Routing.Headers["x-aurago-task"]; got != "coding" {
		t.Fatalf("routing header = %q, want coding", got)
	}
	if !s.Cfg.Manifest.Routing.Enabled {
		t.Fatal("saved config was modified")
	}
}
