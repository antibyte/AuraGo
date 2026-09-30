package server

import (
	"aurago/internal/i18n"
	"aurago/internal/security"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
)

// setupBootstrapHeader carries the one-time bootstrap token. The setup wizard
// and the first admin password are reachable without a session, so on any
// listener other devices can reach the caller must prove it can read the
// server log. Loopback peers of a local-only listener are exempt: nothing else
// can reach them, the same trust validateRemoteAuthExposure grants.
const setupBootstrapHeader = "X-Setup-Token"

// ensureSetupBootstrapToken returns the active token, creating and logging a
// new one when none exists.
func (s *Server) ensureSetupBootstrapToken() string {
	s.setupBootstrapMu.Lock()
	defer s.setupBootstrapMu.Unlock()
	if s.setupBootstrapValue != "" {
		return s.setupBootstrapValue
	}
	token, err := GenerateRandomHex(32)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("[Setup] Failed to generate bootstrap token", "error", err)
		}
		return ""
	}
	s.setupBootstrapValue = token
	if s.Logger != nil {
		s.Logger.Warn("[Setup] Setup is waiting for its owner. Setup from another device needs this one-time bootstrap token.",
			"bootstrap_token", token, "setup_path", "/setup#bootstrap="+token)
	}
	return token
}

// announceSetupBootstrap logs the token at startup when an unconfigured or
// locked-down instance is reachable from other devices, so the owner finds it
// in the log before anyone else opens the setup wizard.
func (s *Server) announceSetupBootstrap() {
	s.CfgMu.RLock()
	pending := needsSetup(s.Cfg)
	remoteIngress := configAllowsRemoteIngress(s.Cfg)
	s.CfgMu.RUnlock()
	if pending && remoteIngress && !setupVaultUnreadable(s) {
		s.ensureSetupBootstrapToken()
	}
}

func (s *Server) validSetupBootstrapToken(candidate string) bool {
	if candidate == "" {
		return false
	}
	s.setupBootstrapMu.Lock()
	current := s.setupBootstrapValue
	s.setupBootstrapMu.Unlock()
	return current != "" && subtle.ConstantTimeCompare([]byte(candidate), []byte(current)) == 1
}

// clearSetupBootstrapToken retires the token once the instance has an owner.
func (s *Server) clearSetupBootstrapToken() {
	s.setupBootstrapMu.Lock()
	s.setupBootstrapValue = ""
	s.setupBootstrapMu.Unlock()
}

func setupBootstrapTokenRequired(s *Server, r *http.Request) bool {
	s.CfgMu.RLock()
	remoteIngress := configAllowsRemoteIngress(s.Cfg)
	s.CfgMu.RUnlock()
	if remoteIngress {
		return true
	}
	peer, err := netip.ParseAddr(remoteIP(r.RemoteAddr))
	return err != nil || !peer.Unmap().IsLoopback()
}

// setupVaultUnreadable reports a vault that exists but cannot be read,
// typically after a restart with a different AURAGO_MASTER_KEY. The lockdown
// then hides a configured owner's password instead of marking a fresh install.
func setupVaultUnreadable(s *Server) bool {
	if s == nil || s.Vault == nil {
		return false
	}
	_, err := s.Vault.ReadSecret("auth_password_hash")
	return err != nil && !errors.Is(err, security.ErrSecretNotFound)
}

// authorizeSetupBootstrap gates every unauthenticated request that could claim
// the instance. It writes the error response and returns false when the
// request must stop.
func authorizeSetupBootstrap(s *Server, w http.ResponseWriter, r *http.Request) bool {
	lang := s.Cfg.Server.UILanguage
	if setupVaultUnreadable(s) {
		if s.Logger != nil {
			s.Logger.Error("[Setup] Setup request refused: the vault cannot be decrypted (check AURAGO_MASTER_KEY)", "path", r.URL.Path)
		}
		writeSetupGateError(w, http.StatusServiceUnavailable, "setup_vault_locked", i18n.T(lang, "backend.setup_vault_locked"))
		return false
	}
	if !setupBootstrapTokenRequired(s, r) || s.validSetupBootstrapToken(r.Header.Get(setupBootstrapHeader)) {
		return true
	}
	if s.Logger != nil {
		s.Logger.Warn("[Setup] Request without valid bootstrap token rejected", "path", r.URL.Path, "remote", remoteIP(r.RemoteAddr))
	}
	writeSetupGateError(w, http.StatusForbidden, "setup_bootstrap_token_required", i18n.T(lang, "backend.setup_bootstrap_token_required"))
	return false
}

func writeSetupGateError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
