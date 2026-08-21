package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
)

const hereNowTestMasterKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func newHereNowHandlerTestVault(t *testing.T) *security.Vault {
	t.Helper()
	vault, err := security.NewVault(hereNowTestMasterKey, filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	return vault
}

func decodeHereNowHandlerBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
	return body
}

func TestHandleHereNowStatusIsLocalAndRedacted(t *testing.T) {
	cfg := &config.Config{}
	vault := newHereNowHandlerTestVault(t)
	server := &Server{Cfg: cfg, Vault: vault}

	recorder := httptest.NewRecorder()
	handleHereNowStatus(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/here-now/status", nil))
	if body := decodeHereNowHandlerBody(t, recorder); body["status"] != "disabled" || body["key_present"] != false {
		t.Fatalf("disabled status = %#v", body)
	}

	cfg.HereNow.Enabled = true
	recorder = httptest.NewRecorder()
	handleHereNowStatus(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/here-now/status", nil))
	if body := decodeHereNowHandlerBody(t, recorder); body["status"] != "no_key" || body["key_present"] != false {
		t.Fatalf("missing-key status = %#v", body)
	}

	if err := vault.WriteSecret("here_now_api_key", "must-never-appear"); err != nil {
		t.Fatal(err)
	}
	cfg.HereNow.DefaultAccount = "workspace-id"
	recorder = httptest.NewRecorder()
	handleHereNowStatus(server).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/here-now/status", nil))
	body := decodeHereNowHandlerBody(t, recorder)
	if body["status"] != "ready" || body["key_present"] != true || body["default_account"] != "workspace-id" {
		t.Fatalf("ready status = %#v", body)
	}
	if _, leaked := body["api_key"]; leaked || strings.Contains(recorder.Body.String(), "must-never-appear") {
		t.Fatalf("status leaked API key: %s", recorder.Body.String())
	}
}

func TestHereNowAdminHandlersRejectMissingCredentialsAndWrongMethods(t *testing.T) {
	server := &Server{Cfg: &config.Config{}, Vault: newHereNowHandlerTestVault(t)}
	server.Cfg.HereNow.Enabled = true

	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
		method  string
		want    int
	}{
		{"test_missing_key", handleHereNowTestConnection(server), http.MethodPost, http.StatusBadRequest},
		{"accounts_missing_key", handleHereNowAccounts(server), http.MethodGet, http.StatusBadRequest},
		{"status_wrong_method", handleHereNowStatus(server), http.MethodPost, http.StatusMethodNotAllowed},
		{"test_wrong_method", handleHereNowTestConnection(server), http.MethodGet, http.StatusMethodNotAllowed},
		{"accounts_wrong_method", handleHereNowAccounts(server), http.MethodPost, http.StatusMethodNotAllowed},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			test.handler.ServeHTTP(recorder, httptest.NewRequest(test.method, "/api/here-now/test", nil))
			if recorder.Code != test.want {
				t.Fatalf("status = %d body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestInjectHereNowDefaultsExposesEffectiveDenyByDefaultConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.HereNow.ReadOnly = true
	raw := map[string]interface{}{}
	injectHereNowDefaults(raw, cfg)
	section, ok := raw["here_now"].(map[string]interface{})
	if !ok {
		t.Fatalf("here_now section = %#v", raw["here_now"])
	}
	if section["readonly"] != true || section["enabled"] != false || section["allow_publish"] != false {
		t.Fatalf("unsafe effective here.now UI defaults: %#v", section)
	}
	if _, present := section["api_key"]; present {
		t.Fatalf("Vault secret leaked into effective config: %#v", section)
	}
}
