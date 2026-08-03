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
	if r == nil || strings.TrimSpace(token) == "" {
		return false
	}
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked(now)
	record, ok := r.tokens[token]
	if !ok {
		return false
	}
	delete(r.tokens, token)
	if sessionID == "" {
		sessionID = "default"
	}
	return record.sessionID == sessionID && record.transcriptHash == sha256.Sum256([]byte(transcript)) && now.Before(record.expiresAt)
}

func (r *speechLabTurnTokenRegistry) pruneLocked(now time.Time) {
	for token, record := range r.tokens {
		if !now.Before(record.expiresAt) {
			delete(r.tokens, token)
		}
	}
}
