package realtimespeech

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

const (
	sessionLeaseTTL = 35 * time.Minute
	turnDedupeTTL   = 24 * time.Hour
)

// Session records only lifecycle metadata. Audio and transcripts are never
// stored in this registry.
type Session struct {
	ID               string    `json:"id"`
	ClientID         string    `json:"-"`
	ProfileID        string    `json:"profile_id"`
	Provider         string    `json:"provider"`
	ChatSessionID    string    `json:"chat_session_id"`
	Surface          string    `json:"surface"`
	State            string    `json:"state"`
	ConversationID   string    `json:"-"`
	ResumptionHandle string    `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	LastActiveAt     time.Time `json:"last_active_at"`
	parkedAt         time.Time
}

type actionLease struct {
	sessionID     string
	chatSessionID string
	startedAt     time.Time
}

type clientRate struct {
	window time.Time
	count  int
}

// Registry coordinates browser microphone leases, action cancellation, turn
// idempotency, and privacy-safe operational telemetry.
type Registry struct {
	mu                sync.Mutex
	now               func() time.Time
	sessions          map[string]Session
	clientSessions    map[string]string
	actions           map[string]actionLease
	seenTurns         map[string]time.Time
	rates             map[string]clientRate
	reconnects        uint64
	parkedTransitions uint64
	parkedDuration    time.Duration
	wakeLatencyTotal  time.Duration
	wakeLatencyCount  uint64
	usageEvents       uint64
	usageMetrics      map[string]float64
	errors            uint64
}

// NewRegistry creates an isolated registry. Supplying nil uses time.Now.
func NewRegistry(now func() time.Time) *Registry {
	if now == nil {
		now = time.Now
	}
	return &Registry{
		now:            now,
		sessions:       make(map[string]Session),
		clientSessions: make(map[string]string),
		actions:        make(map[string]actionLease),
		seenTurns:      make(map[string]time.Time),
		rates:          make(map[string]clientRate),
		usageMetrics:   make(map[string]float64),
	}
}

// Acquire starts or resumes a microphone lease. A client ID is shared through
// localStorage by same-origin tabs, while BroadcastChannel handles the friendly
// takeover prompt in the UI.
func (r *Registry) Acquire(clientID string, session Session, takeover bool) (Session, string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" || len(clientID) > 128 {
		return Session{}, "", fmt.Errorf("valid client_id is required")
	}
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)

	existing, resuming := r.sessions[session.ID]
	if resuming && existing.ClientID != clientID {
		return Session{}, "", fmt.Errorf("session does not belong to this browser")
	}
	if !resuming && !r.allowStartLocked(clientID, now) {
		return Session{}, "", fmt.Errorf("too many realtime speech session starts; try again shortly")
	}
	if existingID := r.clientSessions[clientID]; existingID != "" && existingID != session.ID {
		if _, ok := r.sessions[existingID]; ok {
			if !takeover {
				return Session{}, existingID, fmt.Errorf("another microphone session is active")
			}
			delete(r.sessions, existingID)
		}
		delete(r.clientSessions, clientID)
	}

	if session.ID == "" {
		session.ID = randomID("rts")
		session.CreatedAt = now
	}
	if resuming {
		session.CreatedAt = existing.CreatedAt
		if session.ConversationID == "" {
			session.ConversationID = existing.ConversationID
		}
		if session.ResumptionHandle == "" {
			session.ResumptionHandle = existing.ResumptionHandle
		}
		r.reconnects++
	}
	session.ClientID = clientID
	session.LastActiveAt = now
	if session.State == "" {
		session.State = "connecting"
	}
	r.sessions[session.ID] = session
	r.clientSessions[clientID] = session.ID
	return session, "", nil
}

func (r *Registry) allowStartLocked(clientID string, now time.Time) bool {
	rate := r.rates[clientID]
	if rate.window.IsZero() || now.Sub(rate.window) >= time.Minute {
		r.rates[clientID] = clientRate{window: now, count: 1}
		return true
	}
	if rate.count >= 20 {
		return false
	}
	rate.count++
	r.rates[clientID] = rate
	return true
}

// UpdateState records state and provider-specific resume metadata.
func (r *Registry) UpdateState(id, clientID, state, conversationID, resumptionHandle string) (Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok || session.ClientID != clientID {
		return Session{}, fmt.Errorf("realtime speech session not found")
	}
	now := r.now()
	if state != "" {
		if state == "parked" && session.State != "parked" {
			r.parkedTransitions++
			session.parkedAt = now
		} else if session.State == "parked" && state != "parked" {
			r.finishParkLocked(session, now)
			session.parkedAt = time.Time{}
		}
		session.State = state
	}
	if conversationID != "" {
		session.ConversationID = conversationID
	}
	if resumptionHandle != "" {
		session.ResumptionHandle = resumptionHandle
	}
	session.LastActiveAt = now
	r.sessions[id] = session
	return session, nil
}

// Get returns a session owned by a browser client.
func (r *Registry) Get(id, clientID string) (Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(r.now())
	session, ok := r.sessions[id]
	if !ok || (clientID != "" && session.ClientID != clientID) {
		return Session{}, false
	}
	return session, true
}

// Release removes a microphone lease and any linked action metadata.
func (r *Registry) Release(id, clientID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	session, ok := r.sessions[id]
	if !ok || (clientID != "" && session.ClientID != clientID) {
		return false
	}
	r.finishParkLocked(session, r.now())
	delete(r.sessions, id)
	if r.clientSessions[session.ClientID] == id {
		delete(r.clientSessions, session.ClientID)
	}
	for requestID, action := range r.actions {
		if action.sessionID == id {
			delete(r.actions, requestID)
		}
	}
	return true
}

// BeginAction links a voice request ID to the authoritative AuraGo chat
// session used by agent.InterruptSession.
func (r *Registry) BeginAction(requestID, sessionID, clientID, chatSessionID string) error {
	if strings.TrimSpace(requestID) == "" || len(requestID) > 128 {
		return fmt.Errorf("valid request_id is required")
	}
	if _, ok := r.Get(sessionID, clientID); !ok {
		return fmt.Errorf("realtime speech session not found")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.actions[requestID]; exists {
		return fmt.Errorf("request_id is already active")
	}
	r.actions[requestID] = actionLease{sessionID: sessionID, chatSessionID: chatSessionID, startedAt: r.now()}
	if session, ok := r.sessions[sessionID]; ok {
		session.State = "executing"
		session.LastActiveAt = r.now()
		r.sessions[sessionID] = session
	}
	return nil
}

// EndAction clears an action and returns the voice session to listening.
func (r *Registry) EndAction(requestID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	action, ok := r.actions[requestID]
	if !ok {
		return
	}
	delete(r.actions, requestID)
	if session, exists := r.sessions[action.sessionID]; exists {
		session.State = "listening"
		session.LastActiveAt = r.now()
		r.sessions[action.sessionID] = session
	}
}

// ActionSession resolves a cancellable action.
func (r *Registry) ActionSession(requestID, clientID string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	action, ok := r.actions[requestID]
	if !ok {
		return "", false
	}
	session, ok := r.sessions[action.sessionID]
	if !ok || session.ClientID != clientID {
		return "", false
	}
	return action.chatSessionID, true
}

// MarkTurn returns false for an already persisted turn ID.
func (r *Registry) MarkTurn(sessionID, clientID, turnID string) bool {
	if turnID == "" || len(turnID) > 128 {
		return false
	}
	if _, ok := r.Get(sessionID, clientID); !ok {
		return false
	}
	key := sessionID + "\x00" + turnID
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	if _, exists := r.seenTurns[key]; exists {
		return false
	}
	r.seenTurns[key] = now
	return true
}

// ForgetTurn allows a failed persistence attempt to be retried.
func (r *Registry) ForgetTurn(sessionID, turnID string) {
	r.mu.Lock()
	delete(r.seenTurns, sessionID+"\x00"+turnID)
	r.mu.Unlock()
}

// RecordError increments privacy-safe telemetry.
func (r *Registry) RecordError() {
	r.mu.Lock()
	r.errors++
	r.mu.Unlock()
}

// RecordWakeLatency records only a bounded reconnect duration.
func (r *Registry) RecordWakeLatency(milliseconds int64) {
	if milliseconds < 0 || milliseconds > 30000 {
		return
	}
	r.mu.Lock()
	r.wakeLatencyTotal += time.Duration(milliseconds) * time.Millisecond
	r.wakeLatencyCount++
	r.mu.Unlock()
}

// RecordUsage aggregates numeric provider usage metadata without retaining
// transcripts, audio, or arbitrary string values supplied by a provider.
func (r *Registry) RecordUsage(provider string, usage map[string]interface{}) {
	provider = usageMetricKey(provider)
	if provider == "" || len(usage) == 0 {
		return
	}
	r.mu.Lock()
	r.usageEvents++
	collectUsageMetrics(r.usageMetrics, provider, usage, 0)
	r.mu.Unlock()
}

// Status returns aggregate state without client IDs, transcripts, or audio.
func (r *Registry) Status() map[string]interface{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	r.pruneLocked(now)
	activeProfile := ""
	state := "idle"
	parkedDuration := r.parkedDuration
	for _, session := range r.sessions {
		activeProfile = session.ProfileID
		state = session.State
		if session.State == "parked" && !session.parkedAt.IsZero() {
			parkedDuration += now.Sub(session.parkedAt)
		}
	}
	averageWakeLatency := int64(0)
	if r.wakeLatencyCount > 0 {
		averageWakeLatency = (r.wakeLatencyTotal / time.Duration(r.wakeLatencyCount)).Milliseconds()
	}
	usage := make(map[string]float64, len(r.usageMetrics))
	for key, value := range r.usageMetrics {
		usage[key] = value
	}
	return map[string]interface{}{
		"active_sessions":    len(r.sessions),
		"active_actions":     len(r.actions),
		"active_profile":     activeProfile,
		"session_state":      state,
		"reconnects":         r.reconnects,
		"parked_transitions": r.parkedTransitions,
		"parked_duration_ms": parkedDuration.Milliseconds(),
		"wake_latency_ms":    averageWakeLatency,
		"wake_samples":       r.wakeLatencyCount,
		"usage_events":       r.usageEvents,
		"usage_metrics":      usage,
		"errors":             r.errors,
	}
}

func (r *Registry) pruneLocked(now time.Time) {
	for id, session := range r.sessions {
		if now.Sub(session.LastActiveAt) <= sessionLeaseTTL {
			continue
		}
		r.finishParkLocked(session, now)
		delete(r.sessions, id)
		if r.clientSessions[session.ClientID] == id {
			delete(r.clientSessions, session.ClientID)
		}
	}
	for key, seenAt := range r.seenTurns {
		if now.Sub(seenAt) > turnDedupeTTL {
			delete(r.seenTurns, key)
		}
	}
	for key, rate := range r.rates {
		if now.Sub(rate.window) > 2*time.Minute {
			delete(r.rates, key)
		}
	}
	for requestID, action := range r.actions {
		if now.Sub(action.startedAt) > sessionLeaseTTL {
			delete(r.actions, requestID)
		}
	}
}

func (r *Registry) finishParkLocked(session Session, now time.Time) {
	if session.State == "parked" && !session.parkedAt.IsZero() && now.After(session.parkedAt) {
		r.parkedDuration += now.Sub(session.parkedAt)
	}
}

func collectUsageMetrics(target map[string]float64, prefix string, value interface{}, depth int) {
	if depth > 4 || len(target) >= 64 {
		return
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, nested := range typed {
			normalized := usageMetricKey(key)
			if normalized == "" {
				continue
			}
			collectUsageMetrics(target, prefix+"."+normalized, nested, depth+1)
			if len(target) >= 64 {
				return
			}
		}
	case float64:
		if typed >= 0 && !math.IsNaN(typed) && !math.IsInf(typed, 0) {
			target[prefix] += typed
		}
	}
}

func usageMetricKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || len(value) > 64 {
		return ""
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return ""
	}
	return value
}

func randomID(prefix string) string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(bytes[:])
}
