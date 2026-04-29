package telnyx

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
)

const (
	maxWebhookBodySize = 256 * 1024 // 256 KB
	signatureMaxAge    = 5 * time.Minute
)

// TelnyxPublicKeyBase64 is Telnyx's Ed25519 public key for webhook signature verification.
// See https://developers.telnyx.com/docs/api/v2/overview#webhook-signing
const TelnyxPublicKeyBase64 = "lYf5jEOTv8mUb7LGFzaO3MV08Fa7b5lNdRiMPsOEiis="

// WebhookHandler processes incoming Telnyx webhook events.
type WebhookHandler struct {
	cfg         *config.Config
	logger      *slog.Logger
	client      *Client
	onSMS       func(from, text string, mediaURLs []string) // callback for incoming SMS
	onCallEvent func(event *WebhookEvent)                   // callback for call events

	activeCalls  map[string]*CallSession
	mu           sync.RWMutex
	smsLimiter   *rateLimiter
	seenEventIDs map[string]time.Time
	seenMu       sync.Mutex
}

// NewWebhookHandler creates a webhook handler.
func NewWebhookHandler(cfg *config.Config, logger *slog.Logger, onSMS func(string, string, []string), onCallEvent func(*WebhookEvent)) *WebhookHandler {
	h := &WebhookHandler{
		cfg:          cfg,
		logger:       logger,
		client:       NewClient(cfg.Telnyx.APIKey, logger),
		onSMS:        onSMS,
		onCallEvent:  onCallEvent,
		activeCalls:  make(map[string]*CallSession),
		smsLimiter:   newRateLimiter(cfg.Telnyx.MaxSMSPerMinute, time.Minute),
		seenEventIDs: make(map[string]time.Time),
	}
	return h
}

// HandleWebhook is the HTTP handler for incoming Telnyx webhook events.
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize))
	if err != nil {
		h.logger.Warn("Telnyx webhook: failed to read body", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Signature verification is mandatory — reject if not configured.
	if h.cfg.Telnyx.APISecret == "" {
		h.logger.Warn("Telnyx webhook: API secret not configured, rejecting webhook")
		http.Error(w, "Webhook verification not configured", http.StatusServiceUnavailable)
		return
	}
	if !verifyWebhookSignature(r, body) {
		h.logger.Warn("Telnyx webhook: invalid signature")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Always respond 200 immediately; process async
	w.WriteHeader(http.StatusOK)

	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Warn("Telnyx webhook: failed to parse event", "error", err)
		return
	}

	h.logger.Info("Telnyx webhook event received",
		"event_type", event.Data.EventType,
		"id", event.Data.ID,
	)

	// Deduplicate events by ID within the signature replay window.
	if event.Data.ID != "" && h.isDuplicateEvent(event.Data.ID) {
		h.logger.Info("Telnyx webhook: duplicate event, skipping", "id", event.Data.ID)
		return
	}

	go h.processEvent(&event)
}

// processEvent routes the webhook event to the appropriate handler.
func (h *WebhookHandler) processEvent(event *WebhookEvent) {
	// Validate sender against allowed numbers
	from := event.Data.Payload.From
	if from != "" && !h.isAllowedNumber(from) {
		h.logger.Warn("Telnyx webhook: number not in allowed list", "from", from)
		return
	}

	switch event.Data.EventType {
	case EventMessageReceived:
		h.handleIncomingSMS(event)
	case EventCallInitiated:
		h.handleCallInitiated(event)
	case EventCallAnswered:
		h.handleCallAnswered(event)
	case EventCallHangup:
		h.handleCallHangup(event)
	case EventCallGatherEnded:
		h.handleGatherResult(event)
	case EventCallSpeakEnded, EventCallPlaybackEnded:
		h.handleMediaEnded(event)
	case EventCallRecordingSaved:
		h.handleRecordingSaved(event)
	case EventMessageSent, EventMessageFinalized:
		h.logger.Debug("Telnyx message status update", "event_type", event.Data.EventType, "status", event.Data.Payload.Status)
	default:
		h.logger.Debug("Telnyx webhook: unhandled event type", "event_type", event.Data.EventType)
	}
}

// handleIncomingSMS processes an incoming SMS and optionally forwards to the agent.
func (h *WebhookHandler) handleIncomingSMS(event *WebhookEvent) {
	from := event.Data.Payload.From
	text := event.Data.Payload.Text

	h.logger.Info("Telnyx incoming SMS", "from", from, "length", len(text))
	if h.smsLimiter != nil && !h.smsLimiter.allow() {
		h.logger.Warn("Telnyx incoming SMS rejected by rate limiter", "from", from)
		return
	}

	// Collect media URLs from MMS
	var mediaURLs []string
	for _, m := range event.Data.Payload.MediaURLs {
		mediaURLs = append(mediaURLs, m.URL)
	}

	if h.onSMS != nil && h.cfg.Telnyx.RelayToAgent {
		h.onSMS(from, text, mediaURLs)
	}
}

// handleCallInitiated processes an incoming call event.
func (h *WebhookHandler) handleCallInitiated(event *WebhookEvent) {
	payload := event.Data.Payload
	if payload.Direction != "incoming" {
		return
	}

	h.logger.Info("Telnyx incoming call", "from", payload.From, "call_control_id", payload.CallControlID)

	session := &CallSession{
		CallControlID: payload.CallControlID,
		CallerNumber:  payload.From,
		State:         CallStateRinging,
		StartedAt:     time.Now(),
		LastActivity:  time.Now(),
	}

	h.mu.Lock()
	if len(h.activeCalls) >= h.cfg.Telnyx.MaxConcurrentCalls {
		h.mu.Unlock()
		h.logger.Warn("Telnyx: max concurrent calls reached, rejecting", "from", payload.From)
		return
	}
	h.activeCalls[payload.CallControlID] = session
	h.mu.Unlock()

	if h.onCallEvent != nil {
		h.onCallEvent(event)
	}
}

// handleCallAnswered processes a call answered event.
func (h *WebhookHandler) handleCallAnswered(event *WebhookEvent) {
	ccID := event.Data.Payload.CallControlID
	h.mu.Lock()
	if session, ok := h.activeCalls[ccID]; ok {
		session.State = CallStateGreeting
		session.LastActivity = time.Now()
	}
	h.mu.Unlock()

	if h.onCallEvent != nil {
		h.onCallEvent(event)
	}
}

// handleCallHangup cleans up the call session.
func (h *WebhookHandler) handleCallHangup(event *WebhookEvent) {
	ccID := event.Data.Payload.CallControlID
	h.mu.Lock()
	delete(h.activeCalls, ccID)
	h.mu.Unlock()

	h.logger.Info("Telnyx call ended",
		"call_control_id", ccID,
		"hangup_cause", event.Data.Payload.HangupCause,
	)

	if h.onCallEvent != nil {
		h.onCallEvent(event)
	}
}

// handleGatherResult processes DTMF gather results.
func (h *WebhookHandler) handleGatherResult(event *WebhookEvent) {
	ccID := event.Data.Payload.CallControlID
	h.mu.Lock()
	if session, ok := h.activeCalls[ccID]; ok {
		session.State = CallStateProcessing
		session.LastActivity = time.Now()
	}
	h.mu.Unlock()

	h.logger.Info("Telnyx DTMF gathered",
		"call_control_id", ccID,
		"digits", event.Data.Payload.Digits,
		"result", event.Data.Payload.Result,
	)

	if h.onCallEvent != nil {
		h.onCallEvent(event)
	}
}

// handleMediaEnded processes speak/playback completion events.
func (h *WebhookHandler) handleMediaEnded(event *WebhookEvent) {
	ccID := event.Data.Payload.CallControlID
	h.mu.Lock()
	if session, ok := h.activeCalls[ccID]; ok {
		session.LastActivity = time.Now()
		if session.State == CallStateResponding {
			session.State = CallStateListening
		}
	}
	h.mu.Unlock()

	if h.onCallEvent != nil {
		h.onCallEvent(event)
	}
}

// handleRecordingSaved processes completed recordings.
func (h *WebhookHandler) handleRecordingSaved(event *WebhookEvent) {
	payload := event.Data.Payload
	h.logger.Info("Telnyx recording saved",
		"call_control_id", payload.CallControlID,
		"duration_ms", payload.Duration,
	)

	if h.onCallEvent != nil {
		h.onCallEvent(event)
	}
}

// GetActiveCalls returns a snapshot of all active calls.
func (h *WebhookHandler) GetActiveCalls() []CallSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	calls := make([]CallSession, 0, len(h.activeCalls))
	for _, s := range h.activeCalls {
		calls = append(calls, *s)
	}
	return calls
}

// normalizePhone strips all formatting characters, keeping only '+' and digits.
func normalizePhone(number string) string {
	return strings.Map(func(r rune) rune {
		if r == '+' || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, number)
}

// isAllowedNumber checks if a phone number is in the allowed list.
func (h *WebhookHandler) isAllowedNumber(number string) bool {
	if len(h.cfg.Telnyx.AllowedNumbers) == 0 {
		return false
	}
	clean := normalizePhone(number)
	for _, allowed := range h.cfg.Telnyx.AllowedNumbers {
		if clean == normalizePhone(allowed) {
			return true
		}
	}
	return false
}

// isDuplicateEvent checks the in-memory dedup cache for a previously processed event ID.
// Returns true if the event was already seen within the signature replay window.
func (h *WebhookHandler) isDuplicateEvent(eventID string) bool {
	h.seenMu.Lock()
	defer h.seenMu.Unlock()

	now := time.Now()
	// Prune expired entries.
	for id, seen := range h.seenEventIDs {
		if now.Sub(seen) > signatureMaxAge {
			delete(h.seenEventIDs, id)
		}
	}

	if _, exists := h.seenEventIDs[eventID]; exists {
		return true
	}
	h.seenEventIDs[eventID] = now
	return false
}

// ── Webhook Signature Verification ──────────────────────────────────────

// verifyWebhookSignature verifies Telnyx's Ed25519 webhook signature.
func verifyWebhookSignature(r *http.Request, body []byte) bool {
	sigBase64 := r.Header.Get(SignatureHeader)
	timestampStr := r.Header.Get(TimestampHeader)

	if sigBase64 == "" || timestampStr == "" {
		return false
	}

	// Replay protection: reject timestamps > 5 minutes old
	ts, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		// Try unix timestamp as fallback
		var unix float64
		if _, err2 := fmt.Sscanf(timestampStr, "%f", &unix); err2 != nil {
			return false
		}
		sec := int64(unix)
		nsec := int64((unix - float64(sec)) * 1e9)
		ts = time.Unix(sec, nsec)
	}
	if time.Since(ts).Abs() > signatureMaxAge {
		return false
	}

	// Decode public key
	pubKeyBytes, err := base64.StdEncoding.DecodeString(TelnyxPublicKeyBase64)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}
	pubKey := ed25519.PublicKey(pubKeyBytes)

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(sigBase64)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return false
	}

	// Verify: message = timestamp + "|" + body
	message := []byte(timestampStr + "|")
	message = append(message, body...)
	return ed25519.Verify(pubKey, message, sigBytes)
}

// ── Rate Limiter ────────────────────────────────────────────────────────

type rateLimiter struct {
	max         int
	window      time.Duration
	count       int
	windowStart time.Time
	mu          sync.Mutex
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, windowStart: time.Now()}
}

func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if now.Sub(rl.windowStart) > rl.window {
		rl.count = 0
		rl.windowStart = now
	}
	if rl.count >= rl.max {
		return false
	}
	rl.count++
	return true
}

// ── Call Session ─────────────────────────────────────────────────────────

// CallState represents the current state of a call session.
type CallState int

const (
	CallStateRinging CallState = iota
	CallStateGreeting
	CallStateListening
	CallStateProcessing
	CallStateResponding
	CallStateEnded
)

// CallSession tracks state for an active call.
type CallSession struct {
	CallControlID string
	CallerNumber  string
	State         CallState
	StartedAt     time.Time
	LastActivity  time.Time
}
