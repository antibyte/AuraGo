package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// vaultRateLimiter enforces a per-IP sliding-window rate limit on vault API endpoints.
// Limit: 30 requests per minute per IP — sufficient for normal UI interactions, blocks automated attacks.
var (
	vaultRateMu      sync.Mutex
	vaultRateWindows = make(map[string][]time.Time)
)

func vaultAllowRequest(r *http.Request, behindProxy bool) bool {
	const maxPerMinute = 30
	ip := ClientIP(r, behindProxy)
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	vaultRateMu.Lock()
	defer vaultRateMu.Unlock()

	ts := vaultRateWindows[ip]
	i := 0
	for i < len(ts) && ts[i].Before(cutoff) {
		i++
	}
	ts = ts[i:]
	if len(ts) >= maxPerMinute {
		vaultRateWindows[ip] = ts
		return false
	}
	vaultRateWindows[ip] = append(ts, now)
	return true
}

// vaultSecretJSON is the API representation of a single vault secret.
type vaultSecretJSON struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"` // only populated on explicit single-get (never in list)
}

func canonicalVaultSecretKey(key string) string {
	if vaultKey, ok := vaultKeyMap[key]; ok {
		return vaultKey
	}
	return key
}

func vaultSecretDeleteKeys(key string) []string {
	key = canonicalVaultSecretKey(key)
	keys := []string{key}
	for yamlPath, vaultKey := range vaultKeyMap {
		if vaultKey == key && yamlPath != key {
			keys = append(keys, yamlPath)
		}
	}
	return keys
}

// handleVaultSecrets dispatches GET / POST / DELETE for /api/vault/secrets.
func handleVaultSecrets(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := s.ConfigSnapshot()
		if !vaultAllowRequest(r, cfg != nil && cfg.Server.HTTPS.BehindProxy) {
			jsonError(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
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
	req.Key = canonicalVaultSecretKey(req.Key)
	if req.Value == "" {
		jsonError(w, "Secret value must not be empty", http.StatusBadRequest)
		return
	}
	if req.Key == config.MQTTPasswordVaultKey {
		// Serialize with config save/extraction so an older password cannot be
		// published after a newer Vault change.
		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
		security.RegisterSensitive(req.Value)
	}
	if isTsNetAuthVaultKey(req.Key) {
		req.Value = strings.TrimSpace(req.Value)
		if err := validateTsNetAuthKey(req.Value); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		security.RegisterSensitive(req.Value)
	}

	if err := s.Vault.WriteSecret(req.Key, req.Value); err != nil {
		jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to write secret", "[Vault] Failed to write secret", err, "key", req.Key)
		return
	}

	// Immediately inject the new secret into the live config so it takes effect
	// without requiring a full config save / hot-reload cycle.
	if req.Key == config.MQTTPasswordVaultKey {
		if err := s.refreshMQTTPassword(); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "MQTT credential refresh failed", "[MQTT] Credential refresh failed", err)
			return
		}
	} else {
		s.CfgMu.Lock()
		if current := s.ConfigSnapshot(); current != nil {
			next := current.Clone()
			next.ApplyVaultSecrets(s.Vault)
			s.replaceConfigSnapshot(next)
		}
		s.CfgMu.Unlock()
	}
	applyTsNetCredentialMutation(s, req.Key, req.Value)
	if req.Key == "sudo_password" {
		virtualComputersRetryAutoSetupAfterSudoCredentialChange(s)
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
	key = canonicalVaultSecretKey(key)
	if key == config.MQTTPasswordVaultKey {
		s.CfgSaveMu.Lock()
		defer s.CfgSaveMu.Unlock()
	}

	for _, deleteKey := range vaultSecretDeleteKeys(key) {
		if err := s.Vault.DeleteSecret(deleteKey); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete secret", "[Vault] Failed to delete secret", err, "key", deleteKey)
			return
		}
	}
	if key == config.MQTTPasswordVaultKey {
		if err := s.refreshMQTTPassword(); err != nil {
			jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "MQTT credential refresh failed", "[MQTT] Credential refresh failed", err)
			return
		}
	}
	applyTsNetCredentialMutation(s, key, "")

	s.Logger.Info("[Vault] Secret deleted via Web UI", "key", key)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "key": key})
}

// refreshMQTTPassword publishes only a scalar change; all shared collections in
// the old snapshot remain untouched. Missing credentials never retain old data.
// The caller holds CfgSaveMu, consistently with config-file publication.
func (s *Server) refreshMQTTPassword() error {
	password, _, err := config.ResolveMQTTPassword(s.Vault)
	if err != nil {
		password = "" // Fail closed if storage cannot establish the current credential.
	}
	security.RegisterSensitive(password)
	s.CfgMu.Lock()
	defer s.CfgMu.Unlock()
	if current := s.ConfigSnapshot(); current != nil {
		next := *current
		next.MQTT.Password = password
		s.replaceConfigSnapshot(&next)
	}
	return err
}
