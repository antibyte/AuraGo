package server

import (
	"aurago/internal/config"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Session identities are retained until every issued renewal has expired. The
// registry survives config/token-manager replacement within this process.
var revokedSessions struct {
	sync.Mutex
	entries map[[32]byte]time.Time
}

func sessionIsRevoked(secret, value string) bool {
	revokedSessions.Lock()
	defer revokedSessions.Unlock()
	return sessionIsRevokedLocked(secret, value)
}

func sessionRevocationKey(secret, value string) [32]byte {
	return sha256.Sum256([]byte(secret + "\x00session\x00" + sessionNonce(value)))
}

func sessionIsRevokedLocked(secret, value string) bool {
	// Continue honoring revocations persisted before session identities were stable.
	legacy := sha256.Sum256([]byte(secret + "\x00" + value))
	for _, key := range [][32]byte{sessionRevocationKey(secret, value), legacy} {
		if expiry, ok := revokedSessions.entries[key]; ok && time.Now().Before(expiry) {
			return true
		}
		delete(revokedSessions.entries, key)
	}
	return false
}

func revokeRequestSession(s *Server, r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return true
	}
	if s == nil || s.Cfg == nil {
		return true
	}
	s.CfgMu.RLock()
	secret, dir := s.Cfg.Auth.SessionSecret, s.Cfg.Directories.DataDir
	s.CfgMu.RUnlock()
	expiry := sessionExpiry(secret, cookie.Value)
	if expiry.IsZero() {
		return true
	}
	key := sessionRevocationKey(secret, cookie.Value)
	revokedSessions.Lock()
	defer revokedSessions.Unlock()
	if revokedSessions.entries == nil {
		revokedSessions.entries = make(map[[32]byte]time.Time)
	}
	now := time.Now()
	// An older cookie must also revoke its later ten-minute renewals.
	if renewedUntil := now.Add(authSessionRenewalDuration); expiry.Before(renewedUntil) {
		expiry = renewedUntil
	}
	for k, until := range revokedSessions.entries {
		if !now.Before(until) {
			delete(revokedSessions.entries, k)
		}
	}
	if _, exists := revokedSessions.entries[key]; !exists && len(revokedSessions.entries) >= 8192 {
		return false
	}
	previous, previouslyRevoked := revokedSessions.entries[key]
	if previous.After(expiry) {
		expiry = previous
	}
	revokedSessions.entries[key] = expiry
	if dir != "" {
		entries := make(map[string]time.Time, len(revokedSessions.entries))
		for hash, expiry := range revokedSessions.entries {
			entries[hex.EncodeToString(hash[:])] = expiry
		}
		body, err := json.Marshal(entries)
		if err != nil || config.WriteFileAtomic(filepath.Join(dir, "auth_session_revocations.json"), body, 0600) != nil {
			if previouslyRevoked {
				revokedSessions.entries[key] = previous
			} else {
				delete(revokedSessions.entries, key)
			}
			return false
		}
	}
	return true
}

func loadSessionRevocations(s *Server) error {
	dir := s.Cfg.Directories.DataDir
	if dir == "" {
		return nil
	}
	path := filepath.Join(dir, "auth_session_revocations.json")
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat session revocations: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 2<<20 {
		return fmt.Errorf("invalid session revocation file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read session revocations: %w", err)
	}
	var entries map[string]time.Time
	if err := json.Unmarshal(body, &entries); err != nil {
		return fmt.Errorf("decode session revocations: %w", err)
	}
	if len(entries) > 8192 {
		return fmt.Errorf("too many session revocations")
	}
	revokedSessions.Lock()
	defer revokedSessions.Unlock()
	if revokedSessions.entries == nil {
		revokedSessions.entries = make(map[[32]byte]time.Time)
	}
	for hash, expiry := range entries {
		bytes, err := hex.DecodeString(hash)
		if err != nil || len(bytes) != 32 {
			return fmt.Errorf("invalid session revocation hash")
		}
		if time.Now().Before(expiry) {
			var key [32]byte
			copy(key[:], bytes)
			revokedSessions.entries[key] = expiry
		}
	}
	return nil
}
