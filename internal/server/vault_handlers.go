package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"aurago/internal/tools"
)

// vaultSecretJSON is the API representation of a single vault secret.
type vaultSecretJSON struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"` // only populated on explicit single-get (never in list)
}

// handleVaultSecrets dispatches GET / POST / DELETE for /api/vault/secrets.
func handleVaultSecrets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Vault == nil {
			jsonError(w, "Vault not initialized (master key missing)", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleListVaultSecrets(s, w, r)
		case http.MethodPost:
			handleSetVaultSecret(s, w, r)
		case http.MethodDelete:
			handleDeleteVaultSecret(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleListVaultSecrets returns all secret keys (without values!) sorted alphabetically.
// When the query parameter ?filter=user is present, internal/system secrets are excluded.
func handleListVaultSecrets(s *Server, w http.ResponseWriter, r *http.Request) {
	keys, err := s.Vault.ListKeys()
	if err != nil {
		jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list secrets", "[Vault] Failed to list keys", err)
		return
	}
	sort.Strings(keys)

	filterUser := r.URL.Query().Get("filter") == "user"

	out := make([]vaultSecretJSON, 0, len(keys))
	for _, k := range keys {
		if filterUser && !tools.IsPythonAccessibleSecret(k) {
			continue
		}
		out = append(out, vaultSecretJSON{Key: k})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handleSetVaultSecret creates or updates a single secret.
// Request body: {"key": "...", "value": "..."}
func handleSetVaultSecret(s *Server, w http.ResponseWriter, r *http.Request) {
	var req vaultSecretJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	req.Key = strings.TrimSpace(req.Key)
	if req.Key == "" {
		jsonError(w, "Secret key must not be empty", http.StatusBadRequest)
		return
	}
	if req.Value == "" {
		jsonError(w, "Secret value must not be empty", http.StatusBadRequest)
		return
	}

	if err := s.Vault.WriteSecret(req.Key, req.Value); err != nil {
		jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to write secret", "[Vault] Failed to write secret", err, "key", req.Key)
		return
	}

	// Immediately inject the new secret into the live config so it takes effect
	// without requiring a full config save / hot-reload cycle.
	if s.Cfg != nil {
		s.CfgMu.Lock()
		s.Cfg.ApplyVaultSecrets(s.Vault)
		s.CfgMu.Unlock()
	}

	s.Logger.Info("[Vault] Secret written via Web UI", "key", req.Key)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "key": req.Key})
}

// handleDeleteVaultSecret removes a single secret.
// Expects ?key=<secret_key> query parameter.
func handleDeleteVaultSecret(s *Server, w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		jsonError(w, "Missing ?key= parameter", http.StatusBadRequest)
		return
	}

	if err := s.Vault.DeleteSecret(key); err != nil {
		jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete secret", "[Vault] Failed to delete secret", err, "key", key)
		return
	}

	s.Logger.Info("[Vault] Secret deleted via Web UI", "key", key)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "key": key})
}
