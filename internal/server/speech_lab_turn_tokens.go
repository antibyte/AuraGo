package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const (
	speechLabTurnTokenTTL = 5 * time.Minute
	speechLabTurnTokenMax = 1024
)

type speechLabTurnToken struct {
	sessionID      string
	transcriptHash [sha256.Size]byte
	expiresAt      time.Time
	issuedAt       time.Time
	reserved       bool
}

type speechLabTurnTokenRegistry struct {
	mu     sync.Mutex
	now    func() time.Time
	tokens map[string]speechLabTurnToken
}

func newSpeechLabTurnTokenRegistry(now func() time.Time) *speechLabTurnTokenRegistry {
	if now == nil {
		now = time.Now
	}
	return &speechLabTurnTokenRegistry{now: now, tokens: make(map[string]speechLabTurnToken)}
}

func (r *speechLabTurnTokenRegistry) Issue(sessionID, transcript string) string {
	if r == nil || strings.TrimSpace(transcript) == "" {
		return ""
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	now := r.now()
	token := hex.EncodeToString(bytes)
	record := speechLabTurnToken{
		sessionID: strings.TrimSpace(sessionID), transcriptHash: sha256.Sum256([]byte(transcript)),
		issuedAt: now, expiresAt: now.Add(speechLabTurnTokenTTL),
	}
	if record.sessionID == "" {
		record.sessionID = "default"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	for len(r.tokens) >= speechLabTurnTokenMax {
		oldestToken := ""
		var oldest time.Time
		for candidate, value := range r.tokens {
			if oldestToken == "" || value.issuedAt.Before(oldest) {
				oldestToken, oldest = candidate, value.issuedAt
			}
		}
		delete(r.tokens, oldestToken)
	}
	r.tokens[token] = record
	return token
}

func (r *speechLabTurnTokenRegistry) Consume(token, sessionID, transcript string) bool {
	reservation, ok := r.Reserve(token, sessionID, transcript)
	if !ok {
		return false
	}
	return reservation.commit()
}

type speechLabTurnTokenReservation struct {
	registry *speechLabTurnTokenRegistry
	token    string
	done     bool
}

// Reserve validates a single-use token without consuming it. The caller must
// commit after request preflight succeeds or release it on every failure path.
func (r *speechLabTurnTokenRegistry) Reserve(token, sessionID, transcript string) (*speechLabTurnTokenReservation, bool) {
	if r == nil || strings.TrimSpace(token) == "" {
		return nil, false
	}
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	record, ok := r.tokens[token]
	if !ok {
		return nil, false
	}
	if sessionID == "" {
		sessionID = "default"
	}
	if record.sessionID != sessionID || record.transcriptHash != sha256.Sum256([]byte(transcript)) || !now.Before(record.expiresAt) {
		// Preserve the existing fail-closed one-shot behavior for mismatched
		// attempts: a token presented with the wrong binding is burned.
		delete(r.tokens, token)
		return nil, false
	}
	if record.reserved {
		return nil, false
	}
	record.reserved = true
	r.tokens[token] = record
	return &speechLabTurnTokenReservation{registry: r, token: token}, true
}

func (r *speechLabTurnTokenReservation) commit() bool {
	if r == nil || r.registry == nil || r.done {
		return false
	}
	r.registry.mu.Lock()
	defer r.registry.mu.Unlock()
	record, ok := r.registry.tokens[r.token]
	if !ok || !record.reserved {
		return false
	}
	delete(r.registry.tokens, r.token)
	r.done = true
	return true
}

func (r *speechLabTurnTokenReservation) release() {
	if r == nil || r.registry == nil || r.done {
		return
	}
	r.registry.mu.Lock()
	record, ok := r.registry.tokens[r.token]
	if ok && record.reserved {
		record.reserved = false
		r.registry.tokens[r.token] = record
	}
	r.registry.mu.Unlock()
	r.done = true
}

func (r *speechLabTurnTokenRegistry) pruneLocked(now time.Time) {
	for token, record := range r.tokens {
		if !now.Before(record.expiresAt) {
			delete(r.tokens, token)
		}
	}
}
