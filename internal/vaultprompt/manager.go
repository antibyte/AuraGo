package vaultprompt

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

const (
	EventPrompt = "vault.secret.prompt"
	EventAck    = "vault.secret.ack"

	ErrorKeyInvalid            = "VAULT_KEY_INVALID"
	ErrorTimeout               = "VAULT_SECRET_TIMEOUT"
	ErrorCancelled             = "VAULT_SECRET_CANCELLED"
	ErrorWriteFailed           = "VAULT_WRITE_FAILED"
	ErrorUnsupportedCapability = "UNSUPPORTED_CAPABILITY"

	MaxPromptRunes = 2000
	MaxSecretBytes = 64 * 1024
)

var vaultKeyPattern = regexp.MustCompile(`^[A-Z0-9_]{1,64}$`)

type Sender interface {
	SendTyped(event string, payload interface{}) bool
}

type ContextSender interface {
	SendTypedContext(ctx context.Context, event string, payload interface{}) bool
}

type userSecretWriter interface {
	WriteUserSecretContext(ctx context.Context, key, value string, replace bool) error
}

type Target struct {
	Channel         string
	ClientSessionID string
	TransportID     string
	ConversationID  string
}

type Request struct {
	Prompt   string
	VaultKey string
	Replace  bool
}

type PromptPayload struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Prompt    string `json:"prompt"`
	VaultKey  string `json:"vault_key"`
}

type AckPayload struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	VaultKey  string `json:"vault_key,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type Result struct {
	Status    string `json:"status"`
	VaultKey  string `json:"vault_key,omitempty"`
	Present   bool   `json:"present,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type OperationError struct {
	Code string
}

func (e *OperationError) Error() string {
	if e == nil || strings.TrimSpace(e.Code) == "" {
		return ErrorWriteFailed
	}
	return e.Code
}

func ErrorCode(err error) string {
	var operationErr *OperationError
	if errors.As(err, &operationErr) && operationErr.Code != "" {
		return operationErr.Code
	}
	return ErrorWriteFailed
}

type pendingState uint8

const (
	pendingStatePending pendingState = iota
	pendingStateSubmitting
	pendingStateResolved
)

type pendingRequest struct {
	target       Target
	prompt       PromptPayload
	replace      bool
	sender       Sender
	done         chan struct{}
	result       Result
	ctx          context.Context
	cancel       context.CancelFunc
	cancelSend   context.CancelFunc
	state        pendingState
	cancelResult *Result
}

type clientGate struct {
	ch   chan struct{}
	refs int
}

type Manager struct {
	vault        userSecretWriter
	timeout      time.Duration
	sendTimeout  time.Duration
	writeTimeout time.Duration

	mu      sync.Mutex
	pending map[string]*pendingRequest
	gates   map[string]*clientGate
	closed  bool
	submits sync.WaitGroup
}

func NewManager(vault *security.Vault, timeout time.Duration) *Manager {
	return newManager(vault, timeout)
}

func newManager(vault userSecretWriter, timeout time.Duration) *Manager {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &Manager{
		vault:        vault,
		timeout:      timeout,
		sendTimeout:  10 * time.Second,
		writeTimeout: 15 * time.Second,
		pending:      make(map[string]*pendingRequest),
		gates:        make(map[string]*clientGate),
	}
}

func NormalizeVaultKey(raw string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(raw))
	if !vaultKeyPattern.MatchString(key) {
		return "", &OperationError{Code: ErrorKeyInvalid}
	}
	lower := strings.ToLower(key)
	for _, prefix := range []string{"provider_", "oauth_", "remote_shared_key_", "__aurago_"} {
		if strings.HasPrefix(lower, prefix) {
			return "", &OperationError{Code: ErrorKeyInvalid}
		}
	}
	return key, nil
}

func normalizePrompt(raw string) (string, error) {
	prompt := strings.TrimSpace(raw)
	if prompt == "" {
		return "", &OperationError{Code: ErrorWriteFailed}
	}
	if utf8.RuneCountInString(prompt) > MaxPromptRunes {
		runes := []rune(prompt)
		prompt = string(runes[:MaxPromptRunes])
	}
	return prompt, nil
}

func (m *Manager) Request(ctx context.Context, target Target, req Request, sender Sender) Result {
	if m == nil || m.vault == nil || sender == nil {
		return Result{Status: "error", ErrorCode: ErrorUnsupportedCapability}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	target.ClientSessionID = strings.TrimSpace(target.ClientSessionID)
	target.TransportID = strings.TrimSpace(target.TransportID)
	target.ConversationID = strings.TrimSpace(target.ConversationID)
	if target.ClientSessionID == "" || target.ConversationID == "" {
		return Result{Status: "error", ErrorCode: ErrorUnsupportedCapability}
	}
	if target.TransportID == "" {
		target.TransportID = target.ClientSessionID
	}
	prompt, err := normalizePrompt(req.Prompt)
	if err != nil {
		return Result{Status: "error", ErrorCode: ErrorCode(err)}
	}
	key, err := NormalizeVaultKey(req.VaultKey)
	if err != nil {
		return Result{Status: "error", ErrorCode: ErrorCode(err)}
	}

	release, err := m.acquireClient(ctx, target.ClientSessionID)
	if err != nil {
		return Result{Status: "cancelled"}
	}
	defer release()

	requestID, err := newRequestID()
	if err != nil {
		return Result{Status: "error", ErrorCode: ErrorWriteFailed}
	}
	promptLifetimeCtx, cancelPromptLifetime := context.WithTimeout(ctx, m.timeout)
	defer cancelPromptLifetime()
	operationCtx, cancelOperation := context.WithCancel(promptLifetimeCtx)
	promptSendCtx, cancelPromptSend := context.WithCancel(promptLifetimeCtx)
	defer cancelPromptSend()
	pending := &pendingRequest{
		target: target,
		prompt: PromptPayload{
			SessionID: target.ClientSessionID,
			RequestID: requestID,
			Prompt:    prompt,
			VaultKey:  key,
		},
		replace:    req.Replace,
		sender:     sender,
		done:       make(chan struct{}),
		ctx:        operationCtx,
		cancel:     cancelOperation,
		cancelSend: cancelPromptSend,
		state:      pendingStatePending,
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancelOperation()
		return Result{Status: "error", ErrorCode: ErrorUnsupportedCapability}
	}
	m.pending[requestID] = pending
	m.mu.Unlock()

	if !m.sendTyped(promptSendCtx, sender, EventPrompt, pending.prompt) {
		return m.resolve(requestID, Result{Status: "error", ErrorCode: ErrorUnsupportedCapability})
	}

	select {
	case <-pending.done:
		return pending.result
	case <-promptLifetimeCtx.Done():
		result := Result{Status: "cancelled"}
		if errors.Is(promptLifetimeCtx.Err(), context.DeadlineExceeded) {
			result = Result{Status: "error", ErrorCode: ErrorTimeout}
		}
		m.requestResolution(requestID, result)
		wait := time.NewTimer(m.writeTimeout)
		defer wait.Stop()
		select {
		case <-pending.done:
			return pending.result
		case <-wait.C:
			return m.resolve(requestID, result)
		}
	}
}

// Submit is retained for internal compatibility. Interactive transports should
// use SubmitContext so disconnects and request deadlines can cancel a Vault
// write before its atomic commit.
func (m *Manager) Submit(clientSessionID, requestID, echoedKey, value string) (Result, error) {
	return m.SubmitContext(context.Background(), clientSessionID, clientSessionID, requestID, echoedKey, value)
}

func (m *Manager) SubmitContext(ctx context.Context, clientSessionID, transportID, requestID, echoedKey, value string) (Result, error) {
	if m == nil || m.vault == nil {
		return Result{}, &OperationError{Code: ErrorUnsupportedCapability}
	}
	if value == "" || len(value) > MaxSecretBytes {
		return Result{}, &OperationError{Code: ErrorWriteFailed}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	clientSessionID = strings.TrimSpace(clientSessionID)
	transportID = strings.TrimSpace(transportID)
	if transportID == "" {
		transportID = clientSessionID
	}
	requestID = strings.TrimSpace(requestID)
	echoedKey = strings.TrimSpace(echoedKey)

	m.mu.Lock()
	pending := m.pending[requestID]
	if m.closed || pending == nil || pending.target.ClientSessionID != clientSessionID ||
		pending.target.TransportID != transportID || pending.state != pendingStatePending ||
		pending.prompt.VaultKey != echoedKey {
		m.mu.Unlock()
		return Result{}, &OperationError{Code: ErrorUnsupportedCapability}
	}
	pending.state = pendingStateSubmitting
	m.submits.Add(1)
	m.mu.Unlock()
	defer m.submits.Done()

	writeCtx, cancelWrite := context.WithTimeout(pending.ctx, m.writeTimeout)
	stopRequestCancel := context.AfterFunc(ctx, cancelWrite)
	releaseSensitive := security.RegisterScopedSensitiveExact(value)
	err := m.vault.WriteUserSecretContext(writeCtx, pending.prompt.VaultKey, value, pending.replace)
	value = ""
	writeContextErr := writeCtx.Err()
	stopRequestCancel()
	cancelWrite()
	if err != nil {
		result := m.resultAfterWriteFailure(pending.prompt.RequestID, writeContextErr)
		result = m.resolve(pending.prompt.RequestID, result)
		releaseSensitive()
		if result.Status == "cancelled" {
			return result, nil
		}
		code := result.ErrorCode
		if code == "" {
			code = ErrorWriteFailed
		}
		return result, &OperationError{Code: code}
	}
	result := m.resolve(pending.prompt.RequestID, Result{
		Status:   "stored",
		VaultKey: pending.prompt.VaultKey,
		Present:  true,
	})
	releaseSensitive()
	return result, nil
}

func (m *Manager) Cancel(clientSessionID, requestID string) (Result, error) {
	return m.CancelTransportContext(context.Background(), clientSessionID, clientSessionID, requestID)
}

func (m *Manager) CancelTransport(clientSessionID, transportID, requestID string) (Result, error) {
	return m.CancelTransportContext(context.Background(), clientSessionID, transportID, requestID)
}

func (m *Manager) CancelTransportContext(ctx context.Context, clientSessionID, transportID, requestID string) (Result, error) {
	return m.resolveCorrelated(ctx, clientSessionID, transportID, requestID, Result{Status: "cancelled"})
}

func (m *Manager) resolveCorrelated(ctx context.Context, clientSessionID, transportID, requestID string, result Result) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	clientSessionID = strings.TrimSpace(clientSessionID)
	transportID = strings.TrimSpace(transportID)
	if transportID == "" {
		transportID = clientSessionID
	}
	requestID = strings.TrimSpace(requestID)
	m.mu.Lock()
	pending := m.pending[strings.TrimSpace(requestID)]
	valid := pending != nil && pending.target.ClientSessionID == clientSessionID &&
		pending.target.TransportID == transportID && pending.state != pendingStateResolved
	if !valid {
		m.mu.Unlock()
		return Result{}, &OperationError{Code: ErrorUnsupportedCapability}
	}
	if pending.state == pendingStateSubmitting {
		if pending.cancelResult == nil {
			copy := result
			pending.cancelResult = &copy
		}
		pending.cancel()
		pending.cancelSend()
		done := pending.done
		m.mu.Unlock()

		waitCtx, cancel := context.WithTimeout(ctx, m.writeTimeout+m.sendTimeout)
		defer cancel()
		select {
		case <-done:
			actual := pending.result
			if actual.Status == "error" {
				return actual, &OperationError{Code: actual.ErrorCode}
			}
			return actual, nil
		case <-waitCtx.Done():
			if result.Status == "error" {
				return result, &OperationError{Code: result.ErrorCode}
			}
			return result, nil
		}
	}
	delete(m.pending, pending.prompt.RequestID)
	pending.state = pendingStateResolved
	pending.cancel()
	pending.cancelSend()
	m.mu.Unlock()
	return m.deliver(pending, result), nil
}

func (m *Manager) CancelConversation(conversationID string) {
	m.cancelMatching(func(p *pendingRequest) bool {
		return p.target.ConversationID == strings.TrimSpace(conversationID)
	})
}

func (m *Manager) CancelClient(clientSessionID string) {
	m.cancelMatching(func(p *pendingRequest) bool {
		return p.target.ClientSessionID == strings.TrimSpace(clientSessionID)
	})
}

// DisconnectClient fails prompts whose transport can no longer deliver a
// secure dialog. It is intentionally distinct from a user-initiated cancel.
func (m *Manager) DisconnectClient(clientSessionID string) {
	m.resolveMatching(func(p *pendingRequest) bool {
		return p.target.ClientSessionID == strings.TrimSpace(clientSessionID)
	}, Result{Status: "error", ErrorCode: ErrorUnsupportedCapability})
}

// DisconnectTransport resolves only prompts belonging to one concrete transport
// generation. A delayed cleanup from an older AgoDesk WebSocket therefore
// cannot cancel prompts created by its replacement.
func (m *Manager) DisconnectTransport(clientSessionID, transportID string) {
	clientSessionID = strings.TrimSpace(clientSessionID)
	transportID = strings.TrimSpace(transportID)
	m.resolveMatching(func(p *pendingRequest) bool {
		return p.target.ClientSessionID == clientSessionID && p.target.TransportID == transportID
	}, Result{Status: "error", ErrorCode: ErrorUnsupportedCapability})
}

// RejectUnsupported resolves one correctly correlated prompt immediately when
// its transport loses the secure-write capability.
func (m *Manager) RejectUnsupported(clientSessionID, transportID, requestID string) (Result, error) {
	return m.RejectUnsupportedContext(context.Background(), clientSessionID, transportID, requestID)
}

func (m *Manager) RejectUnsupportedContext(ctx context.Context, clientSessionID, transportID, requestID string) (Result, error) {
	return m.resolveCorrelated(ctx, clientSessionID, transportID, requestID, Result{
		Status: "error", ErrorCode: ErrorUnsupportedCapability,
	})
}

func (m *Manager) Shutdown(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	shutdownResult := Result{
		Status:    "error",
		ErrorCode: ErrorUnsupportedCapability,
	}
	m.mu.Lock()
	m.closed = true
	detached := make([]*pendingRequest, 0, len(m.pending))
	for requestID, pending := range m.pending {
		if pending.state == pendingStateSubmitting {
			if pending.cancelResult == nil {
				copy := shutdownResult
				pending.cancelResult = &copy
			}
			pending.cancel()
			pending.cancelSend()
			continue
		}
		delete(m.pending, requestID)
		pending.state = pendingStateResolved
		pending.cancel()
		pending.cancelSend()
		detached = append(detached, pending)
	}
	m.mu.Unlock()

	var deliveries sync.WaitGroup
	for _, pending := range detached {
		deliveries.Add(1)
		go func(request *pendingRequest) {
			defer deliveries.Done()
			m.deliver(request, shutdownResult)
		}(pending)
	}
	done := make(chan struct{})
	go func() {
		m.submits.Wait()
		deliveries.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func (m *Manager) Status(clientSessionID, conversationID string) *PromptPayload {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pending := range m.pending {
		if pending.target.ClientSessionID == strings.TrimSpace(clientSessionID) &&
			pending.target.ConversationID == strings.TrimSpace(conversationID) {
			copy := pending.prompt
			return &copy
		}
	}
	return nil
}

func (m *Manager) cancelMatching(match func(*pendingRequest) bool) {
	m.resolveMatching(match, Result{Status: "cancelled"})
}

func (m *Manager) resolveMatching(match func(*pendingRequest) bool, result Result) {
	m.mu.Lock()
	ids := make([]string, 0)
	for id, pending := range m.pending {
		if match(pending) {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.requestResolution(id, result)
	}
}

func (m *Manager) resolve(requestID string, result Result) Result {
	m.mu.Lock()
	pending := m.pending[strings.TrimSpace(requestID)]
	if pending == nil {
		m.mu.Unlock()
		return result
	}
	delete(m.pending, pending.prompt.RequestID)
	pending.state = pendingStateResolved
	m.mu.Unlock()
	pending.cancel()
	return m.deliver(pending, result)
}

func (m *Manager) deliver(pending *pendingRequest, result Result) Result {
	ack := AckPayload{
		SessionID: pending.target.ClientSessionID,
		RequestID: pending.prompt.RequestID,
		Status:    result.Status,
		VaultKey:  result.VaultKey,
		ErrorCode: result.ErrorCode,
	}
	m.sendTyped(context.Background(), pending.sender, EventAck, ack)
	pending.result = result
	close(pending.done)
	return result
}

func (m *Manager) requestResolution(requestID string, result Result) {
	m.mu.Lock()
	pending := m.pending[strings.TrimSpace(requestID)]
	if pending == nil || pending.state == pendingStateResolved {
		m.mu.Unlock()
		return
	}
	if pending.state == pendingStateSubmitting {
		if pending.cancelResult == nil {
			copy := result
			pending.cancelResult = &copy
		}
		pending.cancel()
		pending.cancelSend()
		m.mu.Unlock()
		return
	}
	delete(m.pending, pending.prompt.RequestID)
	pending.state = pendingStateResolved
	pending.cancelSend()
	m.mu.Unlock()
	pending.cancel()
	m.deliver(pending, result)
}

func (m *Manager) resultAfterWriteFailure(requestID string, writeErr error) Result {
	m.mu.Lock()
	defer m.mu.Unlock()
	pending := m.pending[strings.TrimSpace(requestID)]
	if pending != nil && pending.cancelResult != nil {
		return *pending.cancelResult
	}
	if errors.Is(writeErr, context.DeadlineExceeded) {
		return Result{Status: "error", ErrorCode: ErrorTimeout}
	}
	if errors.Is(writeErr, context.Canceled) {
		return Result{Status: "cancelled"}
	}
	return Result{Status: "error", ErrorCode: ErrorWriteFailed}
}

func (m *Manager) sendTyped(parent context.Context, sender Sender, event string, payload interface{}) bool {
	if sender == nil {
		return false
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, m.sendTimeout)
	defer cancel()
	if contextSender, ok := sender.(ContextSender); ok {
		return contextSender.SendTypedContext(ctx, event, payload)
	}
	result := make(chan bool, 1)
	go func() {
		result <- sender.SendTyped(event, payload)
	}()
	select {
	case allowed := <-result:
		return allowed
	case <-ctx.Done():
		return false
	}
}

func (m *Manager) acquireClient(ctx context.Context, clientSessionID string) (func(), error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, context.Canceled
	}
	gate := m.gates[clientSessionID]
	if gate == nil {
		gate = &clientGate{ch: make(chan struct{}, 1)}
		m.gates[clientSessionID] = gate
	}
	gate.refs++
	m.mu.Unlock()

	select {
	case gate.ch <- struct{}{}:
		return func() {
			<-gate.ch
			m.mu.Lock()
			gate.refs--
			if gate.refs == 0 {
				delete(m.gates, clientSessionID)
			}
			m.mu.Unlock()
		}, nil
	case <-ctx.Done():
		m.mu.Lock()
		gate.refs--
		if gate.refs == 0 {
			delete(m.gates, clientSessionID)
		}
		m.mu.Unlock()
		return nil, ctx.Err()
	}
}

func newRequestID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "vsreq-" + hex.EncodeToString(raw[:]), nil
}
