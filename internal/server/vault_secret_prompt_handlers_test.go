package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/vaultprompt"
)

type acceptingVaultPromptSender struct{}

func (acceptingVaultPromptSender) SendTyped(string, interface{}) bool { return true }

func newVaultPromptTestServer(t *testing.T) (*Server, *security.Vault) {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "vault-prompt-test-session-secret"
	cfg.Tools.SecretsVault.Enabled = true
	server := &Server{Cfg: cfg, Vault: vault, Logger: slog.Default()}
	server.VaultSecretPrompter = vaultprompt.NewManager(vault, time.Second)
	return server, vault
}

func authenticatedVaultPromptRequest(s *Server, method, target string, body []byte) *http.Request {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	request.Host = "example.com"
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour)),
	})
	return request
}

func startVaultPromptForTest(t *testing.T, s *Server, sessionID, key string) <-chan vaultprompt.Result {
	t.Helper()
	result := make(chan vaultprompt.Result, 1)
	go func() {
		result <- s.VaultSecretPrompter.Request(context.Background(), vaultprompt.Target{
			Channel:         "web_chat",
			ClientSessionID: sessionID,
			ConversationID:  sessionID,
		}, vaultprompt.Request{
			Prompt:   "Enter the test credential.",
			VaultKey: key,
			Replace:  true,
		}, acceptingVaultPromptSender{})
	}()
	deadline := time.Now().Add(time.Second)
	for s.VaultSecretPrompter.Status(sessionID, sessionID) == nil {
		if time.Now().After(deadline) {
			t.Fatal("prompt did not become pending")
		}
		time.Sleep(time.Millisecond)
	}
	return result
}

func TestVaultSecretPromptAPIFailsClosedWithoutAuthentication(t *testing.T) {
	s, _ := newVaultPromptTestServer(t)
	s.Cfg.Auth.Enabled = false
	request := httptest.NewRequest(http.MethodGet, "/api/agent/vault-secret/status?session_id=default", nil)
	recorder := httptest.NewRecorder()

	handleVaultSecretPromptStatus(s).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestVaultSecretPromptMutationRequiresSameOrigin(t *testing.T) {
	s, _ := newVaultPromptTestServer(t)
	request := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/cancel", []byte(`{"session_id":"default","request_id":"vsreq-test"}`))
	request.Header.Set("Origin", "https://attacker.example")
	recorder := httptest.NewRecorder()

	handleVaultSecretPromptCancel(s).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}

	noOriginRequest := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/cancel", []byte(`{"session_id":"default","request_id":"vsreq-test"}`))
	noOriginRequest.Header.Del("Origin")
	noOriginRecorder := httptest.NewRecorder()
	handleVaultSecretPromptCancel(s).ServeHTTP(noOriginRecorder, noOriginRequest)
	if noOriginRecorder.Code != http.StatusForbidden {
		t.Fatalf("missing-origin status = %d, want %d", noOriginRecorder.Code, http.StatusForbidden)
	}
}

func TestVaultSecretPromptSubmitStoresHiddenValueWithoutEcho(t *testing.T) {
	s, vault := newVaultPromptTestServer(t)
	resultCh := startVaultPromptForTest(t, s, "chat-1", "service_api_key")
	prompt := s.VaultSecretPrompter.Status("chat-1", "chat-1")
	const sentinel = "vault-secret-sentinel-do-not-echo"
	body, _ := json.Marshal(map[string]string{
		"session_id": "chat-1",
		"request_id": prompt.RequestID,
		"vault_key":  prompt.VaultKey,
		"value":      sentinel,
	})
	request := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/submit", body)
	recorder := httptest.NewRecorder()

	handleVaultSecretPromptSubmit(s).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), sentinel) {
		t.Fatal("response echoed the secret")
	}
	if got, err := vault.ReadSecret("SERVICE_API_KEY"); err != nil || got != sentinel {
		t.Fatalf("stored secret = %q, err = %v", got, err)
	}
	present, readable, err := vault.AgentSecretInfo("SERVICE_API_KEY")
	if err != nil || !present || readable {
		t.Fatalf("agent secret info = present %v readable %v err %v", present, readable, err)
	}
	select {
	case result := <-resultCh:
		if result.Status != "stored" || result.VaultKey != "SERVICE_API_KEY" || !result.Present {
			t.Fatalf("tool result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("tool result did not resolve")
	}
}

func TestVaultSecretPromptSubmitRejectsCrossSessionAndUnknownFields(t *testing.T) {
	s, vault := newVaultPromptTestServer(t)
	_ = startVaultPromptForTest(t, s, "chat-owner", "PRIVATE_KEY")
	prompt := s.VaultSecretPrompter.Status("chat-owner", "chat-owner")

	crossSessionBody, _ := json.Marshal(map[string]string{
		"session_id": "chat-other",
		"request_id": prompt.RequestID,
		"vault_key":  prompt.VaultKey,
		"value":      "must-not-store",
	})
	crossRequest := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/submit", crossSessionBody)
	crossRecorder := httptest.NewRecorder()
	handleVaultSecretPromptSubmit(s).ServeHTTP(crossRecorder, crossRequest)
	if crossRecorder.Code != http.StatusConflict {
		t.Fatalf("cross-session status = %d, want %d", crossRecorder.Code, http.StatusConflict)
	}
	if _, err := vault.ReadSecret("PRIVATE_KEY"); err == nil {
		t.Fatal("cross-session submission wrote the Vault")
	}
	if got := security.Scrub("must-not-store"); got != "must-not-store" {
		t.Fatalf("cross-session submission mutated redaction registry: %q", got)
	}

	strictBody := []byte(`{"session_id":"chat-owner","request_id":"` + prompt.RequestID + `","vault_key":"PRIVATE_KEY","value":"x","extra":true}`)
	strictRequest := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/submit", strictBody)
	strictRecorder := httptest.NewRecorder()
	handleVaultSecretPromptSubmit(s).ServeHTTP(strictRecorder, strictRequest)
	if strictRecorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown-field status = %d, want %d", strictRecorder.Code, http.StatusBadRequest)
	}

	nonObjectRequest := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/submit", []byte(`null`))
	nonObjectRecorder := httptest.NewRecorder()
	handleVaultSecretPromptSubmit(s).ServeHTTP(nonObjectRecorder, nonObjectRequest)
	if nonObjectRecorder.Code != http.StatusBadRequest {
		t.Fatalf("non-object status = %d, want %d", nonObjectRecorder.Code, http.StatusBadRequest)
	}
}

func TestVaultSecretPromptCapabilityChangeResolvesPendingRequest(t *testing.T) {
	s, vault := newVaultPromptTestServer(t)
	resultCh := startVaultPromptForTest(t, s, "chat-capability", "TOKEN")
	prompt := s.VaultSecretPrompter.Status("chat-capability", "chat-capability")
	s.Cfg.Tools.SecretsVault.ReadOnly = true

	body, _ := json.Marshal(map[string]string{
		"session_id": "chat-capability",
		"request_id": prompt.RequestID,
		"vault_key":  prompt.VaultKey,
		"value":      "must-not-store-after-capability-change",
	})
	request := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/submit", body)
	request.RemoteAddr = "198.51.100.77:1234"
	recorder := httptest.NewRecorder()
	handleVaultSecretPromptSubmit(s).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	select {
	case result := <-resultCh:
		if result.Status != "error" || result.ErrorCode != vaultprompt.ErrorUnsupportedCapability {
			t.Fatalf("tool result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("capability change did not resolve pending prompt")
	}
	if _, err := vault.ReadSecret("TOKEN"); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("disabled capability wrote Vault: %v", err)
	}
}

func TestVaultSecretPromptSubmitAndCancelUseVaultRateLimiter(t *testing.T) {
	s, _ := newVaultPromptTestServer(t)
	const remoteIP = "198.51.100.88"
	defer func() {
		vaultRateMu.Lock()
		delete(vaultRateWindows, remoteIP)
		vaultRateMu.Unlock()
	}()
	for i := 0; i < 30; i++ {
		request := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/cancel", []byte(`{"session_id":"none","request_id":"none"}`))
		request.RemoteAddr = remoteIP + ":1234"
		recorder := httptest.NewRecorder()
		handleVaultSecretPromptCancel(s).ServeHTTP(recorder, request)
		if recorder.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was rate limited early", i+1)
		}
	}
	request := authenticatedVaultPromptRequest(s, http.MethodPost, "/api/agent/vault-secret/submit", []byte(`{"session_id":"none","request_id":"none","vault_key":"TOKEN","value":"secret"}`))
	request.RemoteAddr = remoteIP + ":1234"
	recorder := httptest.NewRecorder()
	handleVaultSecretPromptSubmit(s).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}

func TestWebChatInterruptCancelsPendingVaultSecretPrompt(t *testing.T) {
	s, _ := newVaultPromptTestServer(t)
	resultCh := startVaultPromptForTest(t, s, "chat-cancel", "TOKEN")
	request := httptest.NewRequest(http.MethodPost, "/api/interrupt", strings.NewReader(`{"session_id":"chat-cancel"}`))
	recorder := httptest.NewRecorder()

	handleInterrupt(s).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("interrupt status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	select {
	case result := <-resultCh:
		if result.Status != "cancelled" {
			t.Fatalf("tool result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("pending prompt did not resolve after chat interrupt")
	}
}
