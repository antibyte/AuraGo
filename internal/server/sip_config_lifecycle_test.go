package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/sipphone"
)

func newSIPLifecycleTestServer(t *testing.T) *Server {
	t.Helper()
	s, _, vault := newConfigListPatchTestServer(t)
	cfg, err := sipphone.ApplySIPProviderPreset("fritzbox", map[string]string{"server": "192.0.2.10", "username": "old-user"})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Password = "lifecycle-test-password"
	cfg.ReadOnly = false
	cfg.Inbound.Route = "agent"
	cfg.Inbound.AutoAnswerDelayMS = 4321
	cfg.Inbound.TrustedPeerCIDRs = []string{"198.51.100.7/32"}
	cfg.Inbound.AllowedCallers = []string{"old-user"}
	cfg.Permissions.AnswerInbound = true
	cfg.Voice.AgentProviderID = "phone-agent"
	cfg.Voice.Classic.ASRProviderID = "phone-asr"
	cfg.Voice.Classic.ASRMode = "whisper"
	cfg.Voice.Classic.TTSProvider = "google"
	cfg.Voice.Behavior.Purpose = "Preserve this telephone profile"
	cfg.Voice.AllowedTools = []string{"list_processes"}
	cfg.Voice.PersistTranscripts = true
	cfg.Voice.MaxOutboundCallsPerDay = 23
	s.Cfg.SIP = cfg
	s.Cfg.Runtime.IsDocker = true
	if err := vault.WriteSecret(config.SIPPasswordVaultKey, cfg.Password); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSIPSetupKeepsTelephoneProfileAndExplicitRouting(t *testing.T) {
	for _, tc := range []struct {
		name, provider, scope, route string
		newAccount                   bool
	}{
		{"existing agent", "fritzbox", "all", "agent", false},
		{"incoming disabled", "fritzbox", "disabled", "reject", false},
		{"registration only", "fritzbox", "", "reject", false},
		{"provider replacement", "voip-ms", "all", "agent", false},
		{"new account", "fritzbox", "all", "manual", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newSIPLifecycleTestServer(t)
			voiceBefore := s.Cfg.SIP.Voice
			if tc.newAccount {
				s.Cfg.SIP.PresetID = ""
				s.Cfg.SIP.Registrar = ""
				s.Cfg.SIP.Username = ""
				s.Cfg.SIP.Domain = ""
			}
			payload := map[string]any{
				"provider_id": tc.provider, "confirm_replace": true,
				"values":   map[string]string{"server": "192.0.2.20", "username": "new-user"},
				"password": "new-lifecycle-password",
			}
			if tc.scope != "" {
				payload["activation"] = map[string]string{"outbound_scope": "disabled", "inbound_scope": tc.scope}
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			handleSIPSetup(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/sip/setup", bytes.NewReader(body)))
			if recorder.Code != http.StatusOK && recorder.Code != http.StatusAccepted {
				t.Fatalf("setup status %d: %s", recorder.Code, recorder.Body.String())
			}
			got := sipConfigSnapshot(s)
			if !reflect.DeepEqual(got.Voice, voiceBefore) || got.Inbound.AutoAnswerDelayMS != 4321 || got.Inbound.Route != tc.route {
				t.Fatalf("telephone profile or routing changed: route=%s delay=%d", got.Inbound.Route, got.Inbound.AutoAnswerDelayMS)
			}
			if got.Permissions.AnswerInbound != (tc.scope == "all") {
				t.Fatal("setup did not honor explicit incoming activation")
			}
			if strings.Contains(strings.Join(got.Inbound.TrustedPeerCIDRs, ","), "198.51.100.7") {
				t.Fatal("setup retained peers from the old account")
			}
		})
	}
}

func TestSIPDeleteKeepsTelephoneProfileAndDisablesAccount(t *testing.T) {
	s := newSIPLifecycleTestServer(t)
	voiceBefore := s.Cfg.SIP.Voice
	recorder := httptest.NewRecorder()
	handleSIPConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/sip/config?purge_history=true", nil))
	if recorder.Code != http.StatusOK && recorder.Code != http.StatusAccepted {
		t.Fatalf("delete status %d: %s", recorder.Code, recorder.Body.String())
	}
	got := sipConfigSnapshot(s)
	if !reflect.DeepEqual(got.Voice, voiceBefore) || got.Inbound.AutoAnswerDelayMS != 4321 {
		t.Fatal("deleting the account discarded the telephone profile")
	}
	if got.Enabled || sipConfigHasAccount(got) || got.Inbound.Route != "reject" || got.Password != "" {
		t.Fatal("deleted account still allows calls")
	}
	if _, err := s.Vault.ReadSecret(config.SIPPasswordVaultKey); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatal("deleted account retained its password")
	}
}

func TestSIPSetupAndDeleteRespectReconfigurationReservation(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			s := newSIPLifecycleTestServer(t)
			manager, err := sipphone.NewManager(s.Cfg.SIP, t.TempDir(), nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			s.SIPPhone = manager
			release, err := manager.ReserveReconfigure()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			before, err := os.ReadFile(s.Cfg.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			if method == http.MethodPost {
				body := `{"provider_id":"fritzbox","values":{"server":"192.0.2.20","username":"new-user"},"password":"replacement-password"}`
				handleSIPSetup(s).ServeHTTP(recorder, httptest.NewRequest(method, "/api/sip/setup", strings.NewReader(body)))
			} else {
				handleSIPConfig(s).ServeHTTP(recorder, httptest.NewRequest(method, "/api/sip/config", nil))
			}
			after, err := os.ReadFile(s.Cfg.ConfigPath)
			if err != nil || !bytes.Equal(before, after) || recorder.Code != http.StatusConflict {
				t.Fatalf("reserved mutation changed config or returned status %d", recorder.Code)
			}
			if secret, err := s.Vault.ReadSecret(config.SIPPasswordVaultKey); err != nil || secret != "lifecycle-test-password" {
				t.Fatal("reserved mutation changed the password")
			}
		})
	}
}

func TestConfigPatchRejectsSIPBeforeAnyMutation(t *testing.T) {
	for _, value := range []string{`null`, `{}`, `{"inbound":{"route":"bogus"}}`, `[]`, `"invalid"`} {
		t.Run(value, func(t *testing.T) {
			s := newSIPLifecycleTestServer(t)
			before, err := os.ReadFile(s.Cfg.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			cfgBefore := s.ConfigSnapshot()
			body := `{"sip":` + value + `,"telegram":{"bot_token":"must-not-be-written"}}`
			recorder := httptest.NewRecorder()
			handleUpdateConfig(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(body)))
			after, err := os.ReadFile(s.Cfg.ConfigPath)
			if err != nil || !bytes.Equal(before, after) || s.ConfigSnapshot() != cfgBefore {
				t.Fatal("rejected patch changed the config")
			}
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "sip_use_dedicated_api") {
				t.Fatalf("unexpected rejection: %d %s", recorder.Code, recorder.Body.String())
			}
			if _, err := s.Vault.ReadSecret("telegram_bot_token"); !errors.Is(err, security.ErrSecretNotFound) {
				t.Fatal("rejected patch wrote an unrelated credential")
			}
		})
	}
}

func TestSIPPersistenceErrorsDoNotExposePrivatePaths(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			s := newSIPLifecycleTestServer(t)
			s.Cfg.ConfigPath = filepath.Join(t.TempDir(), "private-location", "missing.yaml")
			body := []byte(`{"provider_id":"fritzbox","values":{"server":"192.0.2.20","username":"new-user"}}`)
			if method == http.MethodPut {
				var err error
				body, err = json.Marshal(sipConfigPayload{SIPConfig: s.Cfg.SIP})
				if err != nil {
					t.Fatal(err)
				}
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(method, "/api/sip/config", bytes.NewReader(body))
			if method == http.MethodPost {
				handleSIPSetup(s).ServeHTTP(recorder, request)
			} else {
				handleSIPConfig(s).ServeHTTP(recorder, request)
			}
			if recorder.Code != http.StatusInternalServerError || strings.Contains(recorder.Body.String(), "private-location") || strings.Contains(recorder.Body.String(), "read config:") {
				t.Fatalf("unsafe persistence error: %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSIPRollbackRestoresBothStoresAndReportsBothFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	vault := &failingProviderVault{values: map[string]string{"secret": "changed"}}
	snapshots := map[string]vaultSnapshot{"secret": {exists: true, value: "original"}}
	if err := rollbackSIPConfig(path, []byte("original config"), vault, snapshots); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original config" || vault.values["secret"] != "original" {
		t.Fatal("rollback did not restore config and credentials")
	}
	vault.writes = 1
	vault.failRollback = true
	err = rollbackSIPConfig(t.TempDir(), []byte("original config"), vault, snapshots)
	if err == nil || !strings.Contains(err.Error(), "restore SIP config") || !strings.Contains(err.Error(), "restore SIP credentials") || vault.writes != 2 {
		t.Fatalf("rollback hid a failure or skipped credentials: %v", err)
	}
}
