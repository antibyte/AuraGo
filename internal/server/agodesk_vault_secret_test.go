package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/agodesk"
	"aurago/internal/security"
	"aurago/internal/vaultprompt"
)

func TestAgodeskVaultSecretPromptCapabilityAndRoundTrip(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	s.Cfg.Tools.SecretsVault.Enabled = true
	s.VaultSecretPrompter = vaultprompt.NewManager(s.Vault, time.Second)

	conn, cleanup, accepted := pairAgodeskTestClient(t, s, "vault-secret-round-trip", []string{
		"chat.full_response",
		agodesk.CapabilityVaultSecretPrompt,
	})
	defer func() {
		cleanup()
		deadline := time.Now().Add(time.Second)
		for ensureAgodeskDesktopBroker(s).session(accepted.DeviceID) != nil && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
	}()
	if !agodeskTestContainsString(accepted.AdvertisedCapabilities, agodesk.CapabilityVaultSecretPrompt) {
		t.Fatalf("advertised capabilities = %v, missing %s", accepted.AdvertisedCapabilities, agodesk.CapabilityVaultSecretPrompt)
	}

	desktopSession := ensureAgodeskDesktopBroker(s).session(accepted.DeviceID)
	if desktopSession == nil || !agodeskStateAllowsVaultSecret(desktopSession.state) {
		t.Fatal("paired writable AgoDesk session did not enable secure Vault prompts")
	}
	broker := &agodeskChatBroker{
		server:         s,
		conn:           desktopSession.conn,
		state:          desktopSession.state,
		sessionID:      accepted.SessionID,
		conversationID: "agodesk-vault-conversation",
		requestID:      "agodesk-vault-agent-request",
		logger:         s.Logger,
	}
	resultCh := make(chan vaultprompt.Result, 1)
	go func() {
		transportID, _ := agodeskStateTransport(desktopSession.state)
		resultCh <- s.VaultSecretPrompter.Request(context.Background(), vaultprompt.Target{
			Channel:         "agodesk",
			ClientSessionID: accepted.SessionID,
			TransportID:     transportID,
			ConversationID:  "agodesk-vault-conversation",
		}, vaultprompt.Request{
			Prompt:   "Enter the integration credential.",
			VaultKey: "integration_api_key",
			Replace:  true,
		}, broker)
	}()

	promptEnvelope := readAgodeskTestEnvelope(t, conn)
	if promptEnvelope.Type != agodesk.TypeVaultSecretPrompt {
		t.Fatalf("prompt type = %q, want %q", promptEnvelope.Type, agodesk.TypeVaultSecretPrompt)
	}
	var prompt agodesk.VaultSecretPromptPayload
	decodeAgodeskTestPayload(t, promptEnvelope, &prompt)
	if prompt.SessionID != accepted.SessionID || prompt.VaultKey != "INTEGRATION_API_KEY" {
		t.Fatalf("prompt payload = %+v", prompt)
	}

	const sentinel = "agodesk-secret-must-not-echo"
	submit, err := agodesk.NewEnvelope(agodesk.TypeVaultSecretSubmit, agodesk.VaultSecretSubmitPayload{
		SessionID: accepted.SessionID,
		RequestID: prompt.RequestID,
		VaultKey:  prompt.VaultKey,
		Value:     sentinel,
	})
	if err != nil {
		t.Fatalf("NewEnvelope submit: %v", err)
	}
	if err := conn.WriteJSON(submit); err != nil {
		t.Fatalf("write vault secret submit: %v", err)
	}

	ackEnvelope := readAgodeskTestEnvelope(t, conn)
	if ackEnvelope.Type != agodesk.TypeVaultSecretAck {
		t.Fatalf("ack type = %q, want %q", ackEnvelope.Type, agodesk.TypeVaultSecretAck)
	}
	if strings.Contains(string(ackEnvelope.Payload), sentinel) {
		t.Fatal("ack payload echoed the secret")
	}
	var ack agodesk.VaultSecretAckPayload
	decodeAgodeskTestPayload(t, ackEnvelope, &ack)
	if ack.Status != "stored" || ack.VaultKey != prompt.VaultKey || ack.RequestID != prompt.RequestID {
		t.Fatalf("ack = %+v", ack)
	}

	select {
	case result := <-resultCh:
		if result.Status != "stored" || !result.Present || result.VaultKey != prompt.VaultKey {
			t.Fatalf("tool result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("tool result did not resolve")
	}
	value, err := s.Vault.ReadSecret(prompt.VaultKey)
	if err != nil || value != sentinel {
		t.Fatalf("Vault value = %q, err = %v", value, err)
	}
	if readable, err := s.Vault.AgentCanReadSecret(prompt.VaultKey); err != nil || readable {
		t.Fatalf("modal secret readable by agent = %v, err = %v", readable, err)
	}
}

func TestAgodeskVaultSecretCapabilityRequiresWritableVaultAndWritableDevice(t *testing.T) {
	s := newAgodeskHandlerTestServer()
	if agodeskTestContainsString(agodeskServerCapabilities(s), agodesk.CapabilityVaultSecretPrompt) {
		t.Fatal("Vault capability advertised without a Vault and enabled write path")
	}

	pairingServer := newAgodeskPairingTestServer(t)
	pairingServer.Cfg.Tools.SecretsVault.Enabled = true
	if !agodeskTestContainsString(agodeskServerCapabilities(pairingServer), agodesk.CapabilityVaultSecretPrompt) {
		t.Fatal("writable Vault capability was not advertised")
	}
	if agodeskTestContainsString(agodeskServerCapabilitiesForDevice(pairingServer, true), agodesk.CapabilityVaultSecretPrompt) {
		t.Fatal("read-only AgoDesk device retained Vault write capability")
	}
}

func TestAgodeskVaultSecretCapabilityChangeResolvesOnlyMatchingPrompt(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	s.Cfg.Tools.SecretsVault.Enabled = true
	s.VaultSecretPrompter = vaultprompt.NewManager(s.Vault, time.Second)
	state := &agodeskConnectionState{
		sessionID:    "agodesk:device",
		transportID:  "transport-current",
		paired:       true,
		capabilities: map[string]struct{}{agodesk.CapabilityVaultSecretPrompt: {}},
		transportCtx: context.Background(),
	}
	resultCh := make(chan vaultprompt.Result, 1)
	go func() {
		resultCh <- s.VaultSecretPrompter.Request(context.Background(), vaultprompt.Target{
			Channel:         "agodesk",
			ClientSessionID: state.sessionID,
			TransportID:     state.transportID,
			ConversationID:  "conversation",
		}, vaultprompt.Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, acceptingVaultPromptSender{})
	}()
	deadline := time.Now().Add(time.Second)
	for s.VaultSecretPrompter.Status(state.sessionID, "conversation") == nil {
		if time.Now().After(deadline) {
			t.Fatal("prompt did not become pending")
		}
		time.Sleep(time.Millisecond)
	}
	prompt := s.VaultSecretPrompter.Status(state.sessionID, "conversation")
	state.mu.Lock()
	state.readOnly = true
	state.mu.Unlock()
	const sentinel = "agodesk-capability-change-sentinel"
	handleAgodeskVaultSecretSubmit(s, nil, state, "envelope", agodesk.VaultSecretSubmitPayload{
		SessionID: state.sessionID,
		RequestID: prompt.RequestID,
		VaultKey:  prompt.VaultKey,
		Value:     sentinel,
	})
	select {
	case result := <-resultCh:
		if result.Status != "error" || result.ErrorCode != vaultprompt.ErrorUnsupportedCapability {
			t.Fatalf("result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("capability change did not resolve prompt")
	}
	if _, err := s.Vault.ReadSecret("TOKEN"); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("read-only capability wrote Vault: %v", err)
	}
	if got := security.Scrub(sentinel); got != sentinel {
		t.Fatalf("rejected value mutated redaction registry: %q", got)
	}
}

func TestAgodeskVaultSecretRateLimitIsPerTransport(t *testing.T) {
	first := &agodeskConnectionState{}
	for i := 0; i < 30; i++ {
		if !agodeskVaultSecretRateAllowed(first) {
			t.Fatalf("request %d was rate limited early", i+1)
		}
	}
	if agodeskVaultSecretRateAllowed(first) {
		t.Fatal("31st request was not rate limited")
	}
	if !agodeskVaultSecretRateAllowed(&agodeskConnectionState{}) {
		t.Fatal("separate transport inherited another transport's rate limit")
	}
}

func TestAgodeskWriteLockHonorsContextDeadline(t *testing.T) {
	state := &agodeskConnectionState{}
	state.writeMu.Lock()
	defer state.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := writeAgodeskEnvelopeLockedContext(ctx, nil, state, agodesk.TypeVaultSecretAck, agodesk.VaultSecretAckPayload{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("write error = %v", err)
	}
}
