package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
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

func TestSIPAPIRoutesAreAdminProtected(t *testing.T) {
	for _, path := range []string{"/api/sip/config", "/api/sip/status", "/api/sip/calls", "/api/sip/events"} {
		if !isAdminProtectedPath(path) {
			t.Fatalf("%s is not administrator protected", path)
		}
	}
}
