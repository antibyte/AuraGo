package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
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
			http.Error(w, "Vault not initialized (master key missing)", http.StatusServiceUnavailable)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleListVaultSecrets returns all secret keys (without values!) sorted alphabetically.
func handleListVaultSecrets(s *Server, w http.ResponseWriter, _ *http.Request) {
	keys, err := s.Vault.ListKeys()
	if err != nil {
		s.Logger.Error("[Vault] Failed to list keys", "error", err)
		http.Error(w, "Failed to list secrets: "+err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Strings(keys)

	out := make([]vaultSecretJSON, len(keys))
	for i, k := range keys {
		out[i] = vaultSecretJSON{Key: k}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handleSetVaultSecret creates or updates a single secret.
// Request body: {"key": "...", "value": "..."}
func handleSetVaultSecret(s *Server, w http.ResponseWriter, r *http.Request) {
	var req vaultSecretJSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	req.Key = strings.TrimSpace(req.Key)
	if req.Key == "" {
		http.Error(w, "Secret key must not be empty", http.StatusBadRequest)
		return
	}
	if req.Value == "" {
		http.Error(w, "Secret value must not be empty", http.StatusBadRequest)
		return
	}

	if err := s.Vault.WriteSecret(req.Key, req.Value); err != nil {
		s.Logger.Error("[Vault] Failed to write secret", "key", req.Key, "error", err)
		http.Error(w, "Failed to write secret: "+err.Error(), http.StatusInternalServerError)
		return
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
		http.Error(w, "Missing ?key= parameter", http.StatusBadRequest)
		return
	}

	if err := s.Vault.DeleteSecret(key); err != nil {
		s.Logger.Error("[Vault] Failed to delete secret", "key", key, "error", err)
		http.Error(w, "Failed to delete secret: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.Logger.Info("[Vault] Secret deleted via Web UI", "key", key)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "key": key})
}
