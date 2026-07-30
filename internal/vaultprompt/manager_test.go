package vaultprompt

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/security"
)

type captureSender struct {
	mu     sync.Mutex
	events []string
	items  []interface{}
	notify chan struct{}
	allow  bool
}

func newCaptureSender() *captureSender {
	return &captureSender{notify: make(chan struct{}, 8), allow: true}
}

func (s *captureSender) SendTyped(event string, payload interface{}) bool {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.items = append(s.items, payload)
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
	return s.allow
}

func (s *captureSender) waitPrompt(t *testing.T) PromptPayload {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-s.notify:
			s.mu.Lock()
			for i, event := range s.events {
				if event == EventPrompt {
					payload := s.items[i].(PromptPayload)
					s.mu.Unlock()
					return payload
				}
			}
			s.mu.Unlock()
		case <-deadline:
			t.Fatal("timed out waiting for prompt")
		}
	}
}

func newTestManager(t *testing.T, timeout time.Duration) (*Manager, *security.Vault) {
	t.Helper()
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	return NewManager(vault, timeout), vault
}

func TestRequestSubmitStoresHiddenUserSecret(t *testing.T) {
	manager, vault := newTestManager(t, time.Second)
	sender := newCaptureSender()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), Target{
			Channel: "web_chat", ClientSessionID: "chat-1", ConversationID: "chat-1",
		}, Request{Prompt: "Enter the API key", VaultKey: "api_key", Replace: true}, sender)
	}()

	prompt := sender.waitPrompt(t)
	if prompt.VaultKey != "API_KEY" || prompt.Prompt != "Enter the API key" {
		t.Fatalf("prompt = %+v", prompt)
	}
	submitResult, err := manager.Submit("chat-1", prompt.RequestID, "API_KEY", "top-secret")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submitResult.Status != "stored" || !submitResult.Present {
		t.Fatalf("Submit result = %+v", submitResult)
	}
	if result := <-resultCh; result != submitResult {
		t.Fatalf("tool result = %+v, submit result = %+v", result, submitResult)
	}
	value, err := vault.ReadSecret("API_KEY")
	if err != nil || value != "top-secret" {
		t.Fatalf("vault value = %q, %v", value, err)
	}
	if _, err := vault.ReadSecretForAgent("API_KEY"); !errors.Is(err, security.ErrSecretAgentAccessDenied) {
		t.Fatalf("modal secret is agent-readable: %v", err)
	}
	sender.mu.Lock()
	transportJSON, _ := json.Marshal(sender.items)
	sender.mu.Unlock()
	resultJSON, _ := json.Marshal(submitResult)
	if strings.Contains(string(transportJSON), "top-secret") || strings.Contains(string(resultJSON), "top-secret") {
		t.Fatal("secret leaked into prompt transport or sanitized result")
	}
	if !strings.Contains(security.Scrub("prefix top-secret suffix"), "top-secret") {
		t.Fatal("submitted secret remained in the global redaction registry")
	}
}

func TestRequestRejectsReservedKeyWithoutPrompt(t *testing.T) {
	manager, _ := newTestManager(t, time.Second)
	sender := newCaptureSender()
	result := manager.Request(context.Background(), Target{
		ClientSessionID: "chat-1", ConversationID: "chat-1",
	}, Request{Prompt: "Enter it", VaultKey: "provider_main_api_key", Replace: true}, sender)
	if result.ErrorCode != ErrorKeyInvalid {
		t.Fatalf("result = %+v", result)
	}
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.events) != 0 {
		t.Fatalf("unexpected events: %v", sender.events)
	}
}

func TestRequestBoundsAgentPromptToTwoThousandRunes(t *testing.T) {
	manager, _ := newTestManager(t, time.Second)
	sender := newCaptureSender()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation",
		}, Request{Prompt: strings.Repeat("ä", MaxPromptRunes+25), VaultKey: "TOKEN", Replace: true}, sender)
	}()
	prompt := sender.waitPrompt(t)
	if got := len([]rune(prompt.Prompt)); got != MaxPromptRunes {
		t.Fatalf("prompt rune count = %d, want %d", got, MaxPromptRunes)
	}
	if _, err := manager.Cancel("client", prompt.RequestID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	<-resultCh
}

func TestRequestCancelAndTimeoutAreSanitized(t *testing.T) {
	t.Run("cancel", func(t *testing.T) {
		manager, _ := newTestManager(t, time.Second)
		sender := newCaptureSender()
		resultCh := make(chan Result, 1)
		go func() {
			resultCh <- manager.Request(context.Background(), Target{
				ClientSessionID: "client", ConversationID: "conversation",
			}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
		}()
		prompt := sender.waitPrompt(t)
		if _, err := manager.Cancel("client", prompt.RequestID); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		if result := <-resultCh; result.Status != "cancelled" || result.VaultKey != "" {
			t.Fatalf("result = %+v", result)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		manager, _ := newTestManager(t, 10*time.Millisecond)
		sender := newCaptureSender()
		result := manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation",
		}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
		if result.Status != "error" || result.ErrorCode != ErrorTimeout {
			t.Fatalf("result = %+v", result)
		}
	})
}

func TestSubmitRejectsWrongSessionKeyAndReplay(t *testing.T) {
	manager, _ := newTestManager(t, time.Second)
	sender := newCaptureSender()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation",
		}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
	}()
	prompt := sender.waitPrompt(t)

	if _, err := manager.Submit("other", prompt.RequestID, "TOKEN", "secret"); ErrorCode(err) != ErrorUnsupportedCapability {
		t.Fatalf("wrong-session error = %v", err)
	}
	if _, err := manager.Submit("client", prompt.RequestID, "OTHER", "secret"); ErrorCode(err) != ErrorUnsupportedCapability {
		t.Fatalf("wrong-key error = %v", err)
	}
	if _, err := manager.Submit("client", prompt.RequestID, "token", "secret"); ErrorCode(err) != ErrorUnsupportedCapability {
		t.Fatalf("non-canonical key error = %v", err)
	}
	if _, err := manager.Submit("client", prompt.RequestID, "TOKEN", "secret"); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	<-resultCh
	if _, err := manager.Submit("client", prompt.RequestID, "TOKEN", "secret"); ErrorCode(err) != ErrorUnsupportedCapability {
		t.Fatalf("replay error = %v", err)
	}
}

func TestReplaceFalsePreservesExistingValue(t *testing.T) {
	manager, vault := newTestManager(t, time.Second)
	if err := vault.WriteSecret("TOKEN", "first"); err != nil {
		t.Fatal(err)
	}
	sender := newCaptureSender()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation",
		}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: false}, sender)
	}()
	prompt := sender.waitPrompt(t)
	if _, err := manager.Submit("client", prompt.RequestID, "TOKEN", "second"); ErrorCode(err) != ErrorWriteFailed {
		t.Fatalf("Submit error = %v", err)
	}
	if result := <-resultCh; result.ErrorCode != ErrorWriteFailed {
		t.Fatalf("tool result = %+v", result)
	}
	value, err := vault.ReadSecret("TOKEN")
	if err != nil || value != "first" {
		t.Fatalf("vault value = %q, %v", value, err)
	}
}

func TestRequestsForSameClientAreSerializedAndDisconnectFailsThem(t *testing.T) {
	manager, _ := newTestManager(t, time.Second)
	firstSender := newCaptureSender()
	secondSender := newCaptureSender()
	firstResult := make(chan Result, 1)
	secondResult := make(chan Result, 1)
	go func() {
		firstResult <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation-one",
		}, Request{Prompt: "First", VaultKey: "FIRST", Replace: true}, firstSender)
	}()
	firstPrompt := firstSender.waitPrompt(t)
	go func() {
		secondResult <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation-two",
		}, Request{Prompt: "Second", VaultKey: "SECOND", Replace: true}, secondSender)
	}()
	select {
	case <-secondSender.notify:
		t.Fatal("second prompt was emitted while the first prompt was still pending")
	case <-time.After(20 * time.Millisecond):
	}

	if _, err := manager.Cancel("client", firstPrompt.RequestID); err != nil {
		t.Fatalf("Cancel first: %v", err)
	}
	if result := <-firstResult; result.Status != "cancelled" {
		t.Fatalf("first result = %+v", result)
	}
	_ = secondSender.waitPrompt(t)
	manager.DisconnectClient("client")
	if result := <-secondResult; result.Status != "error" || result.ErrorCode != ErrorUnsupportedCapability {
		t.Fatalf("disconnect result = %+v", result)
	}
}

func TestSubmitEnforcesSecretByteLimitWithoutResolvingValidPrompt(t *testing.T) {
	manager, _ := newTestManager(t, time.Second)
	sender := newCaptureSender()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation",
		}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
	}()
	prompt := sender.waitPrompt(t)

	if _, err := manager.Submit("client", prompt.RequestID, prompt.VaultKey, strings.Repeat("x", MaxSecretBytes+1)); ErrorCode(err) != ErrorWriteFailed {
		t.Fatalf("oversized Submit error = %v", err)
	}
	if manager.Status("client", "conversation") == nil {
		t.Fatal("oversized attempt consumed the pending prompt")
	}
	if _, err := manager.Submit("client", prompt.RequestID, prompt.VaultKey, strings.Repeat("x", MaxSecretBytes)); err != nil {
		t.Fatalf("max-sized Submit: %v", err)
	}
	if result := <-resultCh; result.Status != "stored" {
		t.Fatalf("result = %+v", result)
	}
}

type blockingUserSecretWriter struct {
	started   chan struct{}
	release   chan error
	startOnce sync.Once
}

func newBlockingUserSecretWriter() *blockingUserSecretWriter {
	return &blockingUserSecretWriter{
		started: make(chan struct{}),
		release: make(chan error, 1),
	}
}

func (w *blockingUserSecretWriter) WriteUserSecretContext(ctx context.Context, _, _ string, _ bool) error {
	w.startOnce.Do(func() { close(w.started) })
	select {
	case err := <-w.release:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type blockingContextSender struct{}

func (blockingContextSender) SendTyped(string, interface{}) bool {
	panic("context-aware sender used non-context path")
}

func (blockingContextSender) SendTypedContext(ctx context.Context, _ string, _ interface{}) bool {
	<-ctx.Done()
	return false
}

type committedUserSecretWriter struct {
	committed chan struct{}
	returnNow chan struct{}
	once      sync.Once
}

func (w *committedUserSecretWriter) WriteUserSecretContext(context.Context, string, string, bool) error {
	w.once.Do(func() { close(w.committed) })
	<-w.returnNow
	return nil
}

func TestPromptSendAndVaultWriteRespectDeadlines(t *testing.T) {
	t.Run("prompt_send", func(t *testing.T) {
		manager, _ := newTestManager(t, time.Second)
		manager.sendTimeout = 15 * time.Millisecond
		start := time.Now()
		result := manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation",
		}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, blockingContextSender{})
		if result.ErrorCode != ErrorUnsupportedCapability {
			t.Fatalf("result = %+v", result)
		}
		if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
			t.Fatalf("prompt send exceeded deadline: %v", elapsed)
		}
	})

	t.Run("vault_write", func(t *testing.T) {
		writer := newBlockingUserSecretWriter()
		manager := newManager(writer, time.Second)
		manager.writeTimeout = 20 * time.Millisecond
		sender := newCaptureSender()
		resultCh := make(chan Result, 1)
		go func() {
			resultCh <- manager.Request(context.Background(), Target{
				ClientSessionID: "client", ConversationID: "conversation",
			}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
		}()
		prompt := sender.waitPrompt(t)
		submitResult, err := manager.SubmitContext(context.Background(), "client", "client", prompt.RequestID, prompt.VaultKey, "deadline-secret")
		if ErrorCode(err) != ErrorTimeout || submitResult.ErrorCode != ErrorTimeout {
			t.Fatalf("submit = %+v, err = %v", submitResult, err)
		}
		if result := <-resultCh; result.ErrorCode != ErrorTimeout {
			t.Fatalf("tool result = %+v", result)
		}
	})
}

func TestInvalidCorrelationDoesNotRegisterSensitiveValue(t *testing.T) {
	manager, _ := newTestManager(t, time.Second)
	sender := newCaptureSender()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", TransportID: "transport-a", ConversationID: "conversation",
		}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
	}()
	prompt := sender.waitPrompt(t)
	const sentinel = "cross-session-redaction-sentinel"
	if _, err := manager.SubmitContext(context.Background(), "client", "transport-b", prompt.RequestID, prompt.VaultKey, sentinel); ErrorCode(err) != ErrorUnsupportedCapability {
		t.Fatalf("cross-transport error = %v", err)
	}
	if got := security.Scrub(sentinel); got != sentinel {
		t.Fatalf("invalid correlation mutated redaction registry: %q", got)
	}
	if _, err := manager.CancelTransport("client", "transport-a", prompt.RequestID); err != nil {
		t.Fatal(err)
	}
	<-resultCh
}

func TestScopedRedactionExistsOnlyDuringCorrelatedVaultWrite(t *testing.T) {
	writer := newBlockingUserSecretWriter()
	manager := newManager(writer, time.Second)
	sender := newCaptureSender()
	resultCh := make(chan Result, 1)
	go func() {
		resultCh <- manager.Request(context.Background(), Target{
			ClientSessionID: "client", ConversationID: "conversation",
		}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
	}()
	prompt := sender.waitPrompt(t)
	const sentinel = "scoped-vault-write-sentinel"
	submitDone := make(chan Result, 1)
	go func() {
		result, _ := manager.SubmitContext(context.Background(), "client", "client", prompt.RequestID, prompt.VaultKey, sentinel)
		submitDone <- result
	}()
	<-writer.started
	if got := security.Scrub(sentinel); strings.Contains(got, sentinel) {
		t.Fatalf("in-flight secret was not redacted: %q", got)
	}
	writer.release <- nil
	if result := <-submitDone; result.Status != "stored" {
		t.Fatalf("submit result = %+v", result)
	}
	<-resultCh
	if got := security.Scrub(sentinel); got != sentinel {
		t.Fatalf("completed modal secret remained registered: %q", got)
	}
}

func TestCancelDuringSubmitAndCommittedWriteHaveSingleTerminalResult(t *testing.T) {
	t.Run("cancel_before_commit", func(t *testing.T) {
		writer := newBlockingUserSecretWriter()
		manager := newManager(writer, time.Second)
		sender := newCaptureSender()
		resultCh := make(chan Result, 1)
		go func() {
			resultCh <- manager.Request(context.Background(), Target{
				ClientSessionID: "client", ConversationID: "conversation",
			}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
		}()
		prompt := sender.waitPrompt(t)
		submitDone := make(chan Result, 1)
		go func() {
			result, _ := manager.SubmitContext(context.Background(), "client", "client", prompt.RequestID, prompt.VaultKey, "cancel-before-commit")
			submitDone <- result
		}()
		<-writer.started
		if _, err := manager.Cancel("client", prompt.RequestID); err != nil {
			t.Fatal(err)
		}
		if result := <-submitDone; result.Status != "cancelled" {
			t.Fatalf("submit result = %+v", result)
		}
		if result := <-resultCh; result.Status != "cancelled" {
			t.Fatalf("tool result = %+v", result)
		}
		sender.mu.Lock()
		ackCount := 0
		for _, event := range sender.events {
			if event == EventAck {
				ackCount++
			}
		}
		sender.mu.Unlock()
		if ackCount != 1 {
			t.Fatalf("ack count = %d, want 1", ackCount)
		}
	})

	t.Run("commit_wins", func(t *testing.T) {
		writer := &committedUserSecretWriter{
			committed: make(chan struct{}),
			returnNow: make(chan struct{}),
		}
		manager := newManager(writer, time.Second)
		sender := newCaptureSender()
		resultCh := make(chan Result, 1)
		go func() {
			resultCh <- manager.Request(context.Background(), Target{
				ClientSessionID: "client", ConversationID: "conversation",
			}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
		}()
		prompt := sender.waitPrompt(t)
		submitDone := make(chan Result, 1)
		go func() {
			result, _ := manager.SubmitContext(context.Background(), "client", "client", prompt.RequestID, prompt.VaultKey, "committed-value")
			submitDone <- result
		}()
		<-writer.committed
		cancelDone := make(chan Result, 1)
		go func() {
			result, _ := manager.Cancel("client", prompt.RequestID)
			cancelDone <- result
		}()
		deadline := time.Now().Add(time.Second)
		for {
			manager.mu.Lock()
			pending := manager.pending[prompt.RequestID]
			cancelRegistered := pending != nil && pending.cancelResult != nil
			manager.mu.Unlock()
			if cancelRegistered {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("cancel did not reach submitting state")
			}
			time.Sleep(time.Millisecond)
		}
		close(writer.returnNow)
		if result := <-submitDone; result.Status != "stored" {
			t.Fatalf("submit result = %+v", result)
		}
		if result := <-cancelDone; result.Status != "stored" {
			t.Fatalf("cancel result = %+v", result)
		}
		if result := <-resultCh; result.Status != "stored" {
			t.Fatalf("tool result = %+v", result)
		}
	})
}

func TestTransportGenerationCleanupCannotCancelReplacementPrompt(t *testing.T) {
	manager, _ := newTestManager(t, time.Second)
	oldSender := newCaptureSender()
	oldResult := make(chan Result, 1)
	go func() {
		oldResult <- manager.Request(context.Background(), Target{
			ClientSessionID: "agodesk:device", TransportID: "old", ConversationID: "old-chat",
		}, Request{Prompt: "Old", VaultKey: "OLD_KEY", Replace: true}, oldSender)
	}()
	_ = oldSender.waitPrompt(t)
	manager.DisconnectTransport("agodesk:device", "old")
	if result := <-oldResult; result.ErrorCode != ErrorUnsupportedCapability {
		t.Fatalf("old result = %+v", result)
	}

	newSender := newCaptureSender()
	newResult := make(chan Result, 1)
	go func() {
		newResult <- manager.Request(context.Background(), Target{
			ClientSessionID: "agodesk:device", TransportID: "new", ConversationID: "new-chat",
		}, Request{Prompt: "New", VaultKey: "NEW_KEY", Replace: true}, newSender)
	}()
	prompt := newSender.waitPrompt(t)
	manager.DisconnectTransport("agodesk:device", "old")
	if manager.Status("agodesk:device", "new-chat") == nil {
		t.Fatal("old cleanup removed replacement prompt")
	}
	if _, err := manager.SubmitContext(context.Background(), "agodesk:device", "new", prompt.RequestID, prompt.VaultKey, "new-transport-secret"); err != nil {
		t.Fatal(err)
	}
	if result := <-newResult; result.Status != "stored" {
		t.Fatalf("new result = %+v", result)
	}
}

func TestShutdownHonorsDeadlineAndCancelsSubmittingWrite(t *testing.T) {
	writer := newBlockingUserSecretWriter()
	manager := newManager(writer, time.Second)
	sender := newCaptureSender()
	go manager.Request(context.Background(), Target{
		ClientSessionID: "client", ConversationID: "conversation",
	}, Request{Prompt: "Enter it", VaultKey: "TOKEN", Replace: true}, sender)
	prompt := sender.waitPrompt(t)
	submitDone := make(chan struct{})
	go func() {
		_, _ = manager.SubmitContext(context.Background(), "client", "client", prompt.RequestID, prompt.VaultKey, "shutdown-secret")
		close(submitDone)
	}()
	<-writer.started
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	manager.Shutdown(ctx)
	select {
	case <-submitDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("shutdown did not cancel the Vault write")
	}
}
