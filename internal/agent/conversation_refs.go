package agent

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const (
	conversationReferenceTTL = 15 * time.Minute
	conversationReferenceMax = 4096
)

type conversationReference struct {
	sessionID string
	entryID   string
	expiresAt time.Time
}

var conversationReferences = struct {
	sync.Mutex
	entries map[string]conversationReference
}{entries: make(map[string]conversationReference)}

func registerConversationReference(sessionID, entryID string) string {
	sessionID, entryID = strings.TrimSpace(sessionID), strings.TrimSpace(entryID)
	if sessionID == "" || entryID == "" {
		return ""
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return ""
	}
	token := hex.EncodeToString(random)
	now := time.Now()
	conversationReferences.Lock()
	for key, entry := range conversationReferences.entries {
		if now.After(entry.expiresAt) {
			delete(conversationReferences.entries, key)
		}
	}
	if len(conversationReferences.entries) >= conversationReferenceMax {
		oldestKey := ""
		var oldestExpiry time.Time
		for key, entry := range conversationReferences.entries {
			if oldestKey == "" || entry.expiresAt.Before(oldestExpiry) {
				oldestKey, oldestExpiry = key, entry.expiresAt
			}
		}
		delete(conversationReferences.entries, oldestKey)
	}
	conversationReferences.entries[token] = conversationReference{sessionID: sessionID, entryID: entryID, expiresAt: now.Add(conversationReferenceTTL)}
	conversationReferences.Unlock()
	return token
}

func resolveConversationReference(sessionID, token string) (string, bool) {
	sessionID, token = strings.TrimSpace(sessionID), strings.TrimSpace(token)
	conversationReferences.Lock()
	defer conversationReferences.Unlock()
	entry, ok := conversationReferences.entries[token]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(conversationReferences.entries, token)
		return "", false
	}
	if entry.sessionID != sessionID {
		return "", false
	}
	return entry.entryID, true
}
