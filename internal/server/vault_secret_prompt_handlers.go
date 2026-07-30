package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"aurago/internal/vaultprompt"
)

// JSON escaping can expand a one-byte control character to six bytes. Keep the
// body bounded while still accepting every valid 64 KiB secret value.
const vaultSecretPromptBodyLimit = int64(6*vaultprompt.MaxSecretBytes + 4096)

type vaultSecretSubmitRequest struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	VaultKey  string `json:"vault_key"`
	Value     string `json:"value"`
}

type vaultSecretCancelRequest struct {
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
}

func ensureVaultSecretPrompter(s *Server) *vaultprompt.Manager {
	if s == nil || s.Vault == nil {
		return nil
	}
	s.vaultSecretPromptMu.Lock()
	defer s.vaultSecretPromptMu.Unlock()
	if s.VaultSecretPrompter == nil {
		s.VaultSecretPrompter = vaultprompt.NewManager(s.Vault, 5*time.Minute)
	}
	return s.VaultSecretPrompter
}

func currentVaultSecretPrompter(s *Server) *vaultprompt.Manager {
	if s == nil {
		return nil
	}
	s.vaultSecretPromptMu.Lock()
	defer s.vaultSecretPromptMu.Unlock()
	return s.VaultSecretPrompter
}

func vaultSecretPromptWriteEnabled(s *Server) bool {
	if s == nil || s.Cfg == nil || s.Vault == nil {
		return false
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return s.Cfg.Tools.SecretsVault.Enabled && !s.Cfg.Tools.SecretsVault.ReadOnly
}

func vaultSecretPromptRateAllowed(s *Server, r *http.Request) bool {
	s.CfgMu.RLock()
	behindProxy := s.Cfg.Server.HTTPS.BehindProxy
	s.CfgMu.RUnlock()
	return vaultAllowRequest(r, behindProxy)
}

func webVaultSecretPromptAuthenticated(s *Server, r *http.Request) bool {
	if s == nil || s.Cfg == nil || r == nil {
		return false
	}
	s.CfgMu.RLock()
	authEnabled := s.Cfg.Auth.Enabled
	sessionSecret := s.Cfg.Auth.SessionSecret
	s.CfgMu.RUnlock()
	return authEnabled && IsAuthenticated(r, sessionSecret)
}

func requireWebVaultSecretPromptSession(s *Server, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !webVaultSecretPromptAuthenticated(s, r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized","redirect":"/auth/login"}`))
			return
		}
		if !isSafeMethod(r.Method) {
			if !checkCSRFOriginWithPolicy(r, true) {
				jsonError(w, "Invalid request origin", http.StatusForbidden)
				return
			}
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				jsonError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
		}
		next(w, r)
	}
}

func decodeVaultSecretPromptJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, vaultSecretPromptBodyLimit)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer clear(raw)
	raw = bytes.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return errors.New("exactly one JSON object is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra interface{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func handleVaultSecretPromptStatus(s *Server) http.HandlerFunc {
	return requireWebVaultSecretPromptSession(s, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
		if sessionID == "" || len(sessionID) > 120 {
			jsonError(w, "Invalid session_id", http.StatusBadRequest)
			return
		}
		if !vaultSecretPromptWriteEnabled(s) {
			if manager := currentVaultSecretPrompter(s); manager != nil {
				manager.DisconnectTransport(sessionID, sessionID)
			}
			writeJSON(w, map[string]interface{}{"status": "unavailable"})
			return
		}
		manager := ensureVaultSecretPrompter(s)
		if manager == nil {
			writeJSON(w, map[string]interface{}{"status": "unavailable"})
			return
		}
		prompt := manager.Status(sessionID, sessionID)
		if prompt == nil {
			writeJSON(w, map[string]interface{}{"status": "none"})
			return
		}
		writeJSON(w, map[string]interface{}{"status": "pending", "prompt": prompt})
	})
}

func handleVaultSecretPromptSubmit(s *Server) http.HandlerFunc {
	return requireWebVaultSecretPromptSession(s, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req vaultSecretSubmitRequest
		if err := decodeVaultSecretPromptJSON(w, r, &req); err != nil {
			req.Value = ""
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if !vaultSecretPromptWriteEnabled(s) {
			req.Value = ""
			if manager := currentVaultSecretPrompter(s); manager != nil {
				_, _ = manager.RejectUnsupportedContext(r.Context(), req.SessionID, req.SessionID, req.RequestID)
			}
			writeVaultSecretPromptError(w, vaultprompt.ErrorUnsupportedCapability, http.StatusConflict)
			return
		}
		if !vaultSecretPromptRateAllowed(s, r) {
			req.Value = ""
			jsonError(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		manager := ensureVaultSecretPrompter(s)
		if manager == nil {
			req.Value = ""
			writeVaultSecretPromptError(w, vaultprompt.ErrorUnsupportedCapability, http.StatusConflict)
			return
		}

		value := req.Value
		req.Value = ""
		result, err := manager.SubmitContext(r.Context(), req.SessionID, req.SessionID, req.RequestID, req.VaultKey, value)
		value = ""
		if err != nil {
			code := vaultprompt.ErrorCode(err)
			status := http.StatusConflict
			if code == vaultprompt.ErrorWriteFailed {
				status = http.StatusInternalServerError
			}
			writeVaultSecretPromptError(w, code, status)
			return
		}
		writeJSON(w, result)
	})
}

func handleVaultSecretPromptCancel(s *Server) http.HandlerFunc {
	return requireWebVaultSecretPromptSession(s, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req vaultSecretCancelRequest
		if err := decodeVaultSecretPromptJSON(w, r, &req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if !vaultSecretPromptWriteEnabled(s) {
			if manager := currentVaultSecretPrompter(s); manager != nil {
				_, _ = manager.RejectUnsupportedContext(r.Context(), req.SessionID, req.SessionID, req.RequestID)
			}
			writeVaultSecretPromptError(w, vaultprompt.ErrorUnsupportedCapability, http.StatusConflict)
			return
		}
		if !vaultSecretPromptRateAllowed(s, r) {
			jsonError(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		manager := ensureVaultSecretPrompter(s)
		if manager == nil {
			writeVaultSecretPromptError(w, vaultprompt.ErrorUnsupportedCapability, http.StatusConflict)
			return
		}
		result, err := manager.CancelTransportContext(r.Context(), req.SessionID, req.SessionID, req.RequestID)
		if err != nil {
			writeVaultSecretPromptError(w, vaultprompt.ErrorCode(err), http.StatusConflict)
			return
		}
		writeJSON(w, result)
	})
}

func writeVaultSecretPromptError(w http.ResponseWriter, code string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":     "error",
		"error_code": code,
	})
}
