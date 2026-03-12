package server

import (
	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/meshcentral"
	"aurago/internal/security"
	"aurago/internal/services"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// handleGetConfig returns the current config as JSON with sensitive fields masked.
func handleGetConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			http.Error(w, "Config path not set", http.StatusInternalServerError)
			return
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			s.Logger.Error("Failed to read config file", "error", err)
			http.Error(w, "Failed to read config", http.StatusInternalServerError)
			return
		}

		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			s.Logger.Error("Failed to parse config", "error", err)
			http.Error(w, "Failed to parse config", http.StatusInternalServerError)
			return
		}

		// Mask sensitive fields
		maskSensitiveFields(rawCfg)

		// Inject masked indicators for vault-only secrets so the UI
		// shows “••••••••” for fields that have a value stored in the vault.
		injectVaultIndicators(rawCfg, s.Vault)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rawCfg)
	}
}

// handleUILanguage updates the UI language independently from the main config patch.
func handleUILanguage(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Language == "" {
			http.Error(w, "Language required", http.StatusBadRequest)
			return
		}

		s.CfgMu.Lock()
		s.Cfg.Server.UILanguage = body.Language
		if err := s.Cfg.Save(s.Cfg.ConfigPath); err != nil {
			s.Logger.Error("Failed to save UI language", "error", err)
			s.CfgMu.Unlock()
			http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
			return
		}
		s.CfgMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	}
}

// handleUpdateConfig patches the config.yaml with the provided JSON values.
func handleUpdateConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			http.Error(w, "Config path not set", http.StatusInternalServerError)
			return
		}

		// Read the incoming patch (with size limit to prevent OOM)
		maxBody := s.Cfg.Server.MaxBodyBytes
		if maxBody <= 0 {
			maxBody = 10 << 20 // 10 MB default
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var patch map[string]interface{}
		if err := json.Unmarshal(body, &patch); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Read the current config
		data, err := os.ReadFile(configPath)
		if err != nil {
			s.Logger.Error("Failed to read config file for patching", "error", err)
			http.Error(w, "Failed to read config", http.StatusInternalServerError)
			return
		}

		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			s.Logger.Error("Failed to parse config for patching", "error", err)
			http.Error(w, "Failed to parse config", http.StatusInternalServerError)
			return
		}

		// Deep merge the patch into the existing config, skipping masked password values.
		// Before merging, extract any secrets from the patch and write them to the vault
		// so they never end up in config.yaml.
		if vaultErr := extractSecretsToVault(patch, s.Vault, s.Logger); vaultErr != nil {
			s.Logger.Error("[Config] Credential could not be saved to vault", "error", vaultErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": vaultErr.Error(),
			})
			return
		}
		deepMerge(rawCfg, patch)

		// Write back
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			s.Logger.Error("Failed to marshal patched config", "error", err)
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(configPath, out, 0644); err != nil {
			s.Logger.Error("Failed to write config file", "error", err)
			http.Error(w, "Failed to write config", http.StatusInternalServerError)
			return
		}

		// Hot-reload: re-parse config and apply to running instance
		s.CfgMu.Lock()
		oldCfg := *s.Cfg // snapshot before reload
		newCfg, loadErr := config.Load(configPath)

		needsRestart := false
		restartReasons := []string{}

		if loadErr != nil {
			s.Logger.Warn("[Config UI] Hot-reload failed, changes saved but require restart", "error", loadErr)
			needsRestart = true
			restartReasons = append(restartReasons, "Parse-Fehler beim Reload")
		} else {
			// Apply vault secrets before comparison so vault-only fields match
			newCfg.ApplyVaultSecrets(s.Vault)
			newCfg.ResolveProviders()
			newCfg.ApplyOAuthTokens(s.Vault)

			// Detect sections that need restart
			if oldCfg.Server != newCfg.Server {
				needsRestart = true
				restartReasons = append(restartReasons, "Server (Host/Port)")
			}
			if oldCfg.Telegram != newCfg.Telegram {
				needsRestart = true
				restartReasons = append(restartReasons, "Telegram")
			}
			if oldCfg.Discord != newCfg.Discord {
				needsRestart = true
				restartReasons = append(restartReasons, "Discord")
			}
			if oldCfg.SQLite != newCfg.SQLite {
				needsRestart = true
				restartReasons = append(restartReasons, "Datenbanken")
			}
			if oldCfg.Directories != newCfg.Directories {
				needsRestart = true
				restartReasons = append(restartReasons, "Verzeichnisse")
			}
			if oldCfg.Chromecast != newCfg.Chromecast {
				needsRestart = true
				restartReasons = append(restartReasons, "Chromecast/TTS Server")
			}
			if oldCfg.Webhooks.Enabled != newCfg.Webhooks.Enabled {
				needsRestart = true
				restartReasons = append(restartReasons, "Webhooks (enabled/disabled)")
			}
			if oldCfg.MQTT.Enabled != newCfg.MQTT.Enabled || oldCfg.MQTT.Broker != newCfg.MQTT.Broker || oldCfg.MQTT.ClientID != newCfg.MQTT.ClientID || oldCfg.MQTT.Username != newCfg.MQTT.Username {
				needsRestart = true
				restartReasons = append(restartReasons, "MQTT")
			}

			// Apply hot-reload: copy all new fields into the live config pointer
			savedPath := s.Cfg.ConfigPath
			*s.Cfg = *newCfg
			s.Cfg.ConfigPath = savedPath

			// Reconfigure the live LLM client when model, API key, base URL,
			// provider or fallback settings have changed.  This ensures that model
			// name changes in the web UI take effect immediately without a restart.
			if oldCfg.LLM != newCfg.LLM || oldCfg.FallbackLLM != newCfg.FallbackLLM {
				if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
					fm.Reconfigure(s.Cfg)
					s.Logger.Info("[Config UI] LLM client reconfigured",
						"model", newCfg.LLM.Model,
						"provider", newCfg.LLM.ProviderType,
						"base_url", newCfg.LLM.BaseURL)
				}
			}

			// Sync the global debug-mode flag used by the agent.
			if oldCfg.Agent.DebugMode != newCfg.Agent.DebugMode {
				agent.SetDebugMode(newCfg.Agent.DebugMode)
				s.Logger.Info("[Config UI] Debug mode updated", "enabled", newCfg.Agent.DebugMode)
			}

			// Update co-agent concurrency limit immediately.
			if oldCfg.CoAgents.MaxConcurrent != newCfg.CoAgents.MaxConcurrent {
				s.CoAgentRegistry.SetMaxSlots(newCfg.CoAgents.MaxConcurrent)
				s.Logger.Info("[Config UI] Co-agent max_concurrent updated", "slots", newCfg.CoAgents.MaxConcurrent)
			}

			// Update webhook payload / rate limits without restart.
			// Toggling webhooks.enabled requires restart (route registered at startup).
			if s.WebhookHandler != nil &&
				(oldCfg.Webhooks.MaxPayloadSize != newCfg.Webhooks.MaxPayloadSize ||
					oldCfg.Webhooks.RateLimit != newCfg.Webhooks.RateLimit) {
				s.WebhookHandler.Reconfigure(int64(newCfg.Webhooks.MaxPayloadSize), newCfg.Webhooks.RateLimit)
				s.Logger.Info("[Config UI] WebhookHandler reconfigured",
					"max_payload", newCfg.Webhooks.MaxPayloadSize,
					"rate_limit", newCfg.Webhooks.RateLimit)
			}

			// Always re-create BudgetTracker after a config reload so that
			// toggling budget.enabled or changing limits takes effect immediately.
			s.BudgetTracker = budget.NewTracker(newCfg, s.Logger, newCfg.Directories.DataDir)
			if newCfg.Budget.Enabled {
				s.Logger.Info("[Config UI] BudgetTracker re-initialized", "enabled", true)
			} else {
				s.Logger.Info("[Config UI] BudgetTracker disabled")
			}

			// Hot-reload File Indexer: start/stop based on enabled flag change
			if oldCfg.Indexing.Enabled != newCfg.Indexing.Enabled {
				if newCfg.Indexing.Enabled && s.FileIndexer == nil {
					s.FileIndexer = services.NewFileIndexer(newCfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)
					s.FileIndexer.Start(context.Background())
					s.Logger.Info("[Config UI] File indexer started")
				} else if !newCfg.Indexing.Enabled && s.FileIndexer != nil {
					s.FileIndexer.Stop()
					s.FileIndexer = nil
					s.Logger.Info("[Config UI] File indexer stopped")
				}
			}

			s.Logger.Info("[Config UI] Configuration hot-reloaded successfully")
		}
		s.CfgMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if needsRestart {
			msg := fmt.Sprintf("Gespeichert. Neustart nötig für: %s", strings.Join(restartReasons, ", "))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         "saved",
				"message":        msg,
				"needs_restart":  true,
				"restart_reason": restartReasons,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "saved",
				"message":       "Konfiguration gespeichert und sofort angewendet.",
				"needs_restart": false,
			})
		}
	}
}

// sensitiveKeys are YAML keys whose values should be masked in the API response.
var sensitiveKeys = map[string]bool{
	"api_key":        true,
	"bot_token":      true,
	"password":       true,
	"app_password":   true,
	"access_token":   true,
	"token":          true,
	"user_key":       true,
	"app_token":      true,
	"login_token":    true, // MeshCentral token
	"master_key":     true, // vault AES-256 key — never expose
	"password_hash":  true, // auth: bcrypt hash — never expose
	"session_secret": true, // auth: HMAC signing key — never expose
	"totp_secret":    true, // auth: TOTP base32 secret — never expose
}

// handleVaultStatus returns whether the vault is available.
// The vault is available when s.Vault != nil (i.e. AURAGO_MASTER_KEY was
// provided at startup). The vault.bin file is created lazily on the first
// write, so checking for file existence would yield a false negative on a
// fresh installation where no secrets have been stored yet.
func handleVaultStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"exists": s.Vault != nil})
	}
}

// handleVaultDelete deletes vault.bin (and its lockfile).
func handleVaultDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vaultPath := filepath.Join(s.Cfg.Directories.DataDir, "vault.bin")
		lockPath := vaultPath + ".lock"

		// Delete vault files
		if err := os.Remove(vaultPath); err != nil && !os.IsNotExist(err) {
			s.Logger.Error("[Vault] Failed to delete vault file", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"message": "Vault-Datei konnte nicht gelöscht werden."})
			return
		}
		os.Remove(lockPath) // best-effort

		// Update in-memory config
		s.CfgMu.Lock()
		s.Cfg.Server.MasterKey = ""
		s.CfgMu.Unlock()

		s.Logger.Info("[Vault] Vault deleted via Web UI")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Vault gelöscht."})
	}
}

// maskSensitiveFields recursively masks sensitive string values in a config map.
func maskSensitiveFields(m map[string]interface{}) {
	for key, val := range m {
		switch v := val.(type) {
		case map[string]interface{}:
			maskSensitiveFields(v)
		case string:
			if sensitiveKeys[key] && v != "" {
				m[key] = "••••••••"
			}
		}
	}
}

// deepMerge recursively merges src into dst. Masked values ("••••••••") and empty
// strings for sensitive fields are skipped to avoid overwriting real secrets.
func deepMerge(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		switch sv := srcVal.(type) {
		case map[string]interface{}:
			// Recurse into nested maps
			if dstMap, ok := dst[key].(map[string]interface{}); ok {
				deepMerge(dstMap, sv)
			} else {
				dst[key] = srcVal
			}
		case []interface{}:
			// JSON arrays: only accept if all elements are valid (not JS stringified objects)
			valid := true
			for _, elem := range sv {
				if s, ok := elem.(string); ok && strings.HasPrefix(s, "[object") {
					valid = false
					break
				}
			}
			if valid {
				// Special handling for budget.models: ensure all items are proper objects
				if key == "models" {
					cleanModels := make([]interface{}, 0, len(sv))
					for _, elem := range sv {
						if obj, ok := elem.(map[string]interface{}); ok {
							// Ensure required fields exist
							if _, hasName := obj["name"]; hasName {
								cleanModels = append(cleanModels, obj)
							}
						}
					}
					// Always set the models array (even if empty) to avoid corruption
					dst[key] = cleanModels
				} else {
					dst[key] = srcVal
				}
			}
		case string:
			// Always skip sensitive fields — extractSecretsToVault already handled them
			// (moved to vault or stripped). Never allow credentials into config.yaml.
			if sensitiveKeys[key] {
				continue
			}
			// Skip JavaScript-stringified values like "[object Object]"
			if strings.HasPrefix(sv, "[object") {
				continue
			}
			dst[key] = srcVal
		default:
			dst[key] = srcVal
		}
	}
}

// vaultKeyMap maps dotted YAML paths to vault key names for static config fields.
// Dynamic fields (providers, email accounts) are handled in their own PUT handlers.
var vaultKeyMap = map[string]string{
	"telegram.bot_token":               "telegram_bot_token",
	"discord.bot_token":                "discord_bot_token",
	"meshcentral.password":             "meshcentral_password",
	"meshcentral.login_token":          "meshcentral_token",
	"tailscale.api_key":                "tailscale_api_key",
	"ansible.token":                    "ansible_token",
	"virustotal.api_key":               "virustotal_api_key",
	"brave_search.api_key":             "brave_search_api_key",
	"tts.elevenlabs.api_key":           "tts_elevenlabs_api_key",
	"notifications.ntfy.token":         "ntfy_token",
	"auth.password_hash":               "auth_password_hash",
	"auth.session_secret":              "auth_session_secret",
	"auth.totp_secret":                 "auth_totp_secret",
	"home_assistant.access_token":      "home_assistant_access_token",
	"webdav.password":                  "webdav_password",
	"koofr.app_password":               "koofr_password",
	"proxmox.secret":                   "proxmox_secret",
	"github.token":                     "github_token",
	"rocketchat.auth_token":            "rocketchat_auth_token",
	"mqtt.password":                    "mqtt_password",
	"email.password":                   "email_password",
	"notifications.pushover.user_key":  "pushover_user_key",
	"notifications.pushover.app_token": "pushover_app_token",
}

// extractSecretsToVault walks a JSON patch map and moves sensitive values into the vault.
// Sensitive keys are removed from the patch so they never reach config.yaml.
// extractSecretsToVault moves sensitive credential fields out of the patch map
// and into the vault. It always strips sensitive fields from the patch to
// ensure they never reach config.yaml, even when no vault is available.
// Returns an error if any credential could not be written to the vault so that
// the caller can surface the failure instead of silently discarding credentials.
func extractSecretsToVault(patch map[string]interface{}, vault *security.Vault, logger *slog.Logger) error {
	return extractRecursive(patch, "", vault, logger)
}

func extractRecursive(m map[string]interface{}, prefix string, vault *security.Vault, logger *slog.Logger) error {
	var firstErr error
	for key, val := range m {
		fullPath := key
		if prefix != "" {
			fullPath = prefix + "." + key
		}

		switch v := val.(type) {
		case map[string]interface{}:
			if err := extractRecursive(v, fullPath, vault, logger); err != nil && firstErr == nil {
				firstErr = err
			}
		case string:
			if !sensitiveKeys[key] {
				continue
			}
			// Always remove from patch — sensitive fields must never reach config.yaml.
			delete(m, key)
			// Empty or masked values are just removed, nothing to store.
			if v == "" || v == "••••••••" {
				continue
			}
			if vault == nil {
				// No vault available — credential is stripped but cannot be persisted.
				// Return an error so the caller can inform the user.
				if firstErr == nil {
					firstErr = fmt.Errorf("credential '%s' cannot be saved: no vault configured (AURAGO_MASTER_KEY required)", fullPath)
				}
				continue
			}
			// Check if this path maps to a known vault key
			vaultKey, ok := vaultKeyMap[fullPath]
			if !ok {
				// Unknown sensitive path — stripped from YAML, not stored
				logger.Warn("[Config] Sensitive field not in vault map, stripping from YAML", "path", fullPath)
				continue
			}
			if err := vault.WriteSecret(vaultKey, v); err != nil {
				logger.Error("[Config] Failed to write secret to vault", "key", vaultKey, "error", err)
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to save credential '%s' to vault: %w", fullPath, err)
				}
			} else {
				logger.Info("[Config] Secret saved to vault", "key", vaultKey)
			}
		}
	}
	return firstErr
}

// injectVaultIndicators adds masked placeholder values ("••••••••") into the
// raw config map for vault-only fields that have a stored secret. This ensures
// the UI shows that a value is configured even though it's not in the YAML.
func injectVaultIndicators(rawCfg map[string]interface{}, vault *security.Vault) {
	if vault == nil {
		return
	}
	for yamlPath, vaultKey := range vaultKeyMap {
		parts := strings.Split(yamlPath, ".")
		if vaultKey == "" {
			continue
		}
		val, err := vault.ReadSecret(vaultKey)
		if err != nil || val == "" {
			continue
		}
		// Navigate to the parent map, creating intermediate maps as needed
		m := rawCfg
		for i := 0; i < len(parts)-1; i++ {
			sub, ok := m[parts[i]].(map[string]interface{})
			if !ok {
				sub = make(map[string]interface{})
				m[parts[i]] = sub
			}
			m = sub
		}
		m[parts[len(parts)-1]] = "••••••••"
	}
}

// getConfigSchema returns a JSON schema describing the config structure for the UI.
// It reflects the Config struct to produce field metadata (type, yaml key).
func handleGetConfigSchema(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		schema := buildSchema(reflect.TypeOf(*s.Cfg), "")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(schema)
	}
}

// SchemaField describes a single config field for the UI renderer.
type SchemaField struct {
	Key       string        `json:"key"`
	YAMLKey   string        `json:"yaml_key"`
	Type      string        `json:"type"` // "string", "int", "float", "bool", "object", "array"
	Sensitive bool          `json:"sensitive,omitempty"`
	Children  []SchemaField `json:"children,omitempty"`
}

func buildSchema(t reflect.Type, prefix string) []SchemaField {
	var fields []SchemaField

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		yamlTag := f.Tag.Get("yaml")
		vaultTag := f.Tag.Get("vault")
		// Skip fields excluded from YAML unless they have a vault tag
		if yamlTag == "-" {
			if vaultTag == "" {
				continue
			}
			yamlTag = vaultTag // use vault key name as display key
		}
		if yamlTag == "" {
			yamlTag = strings.ToLower(f.Name)
		}
		// Strip tag options
		if idx := strings.Index(yamlTag, ","); idx >= 0 {
			yamlTag = yamlTag[:idx]
		}

		fullKey := yamlTag
		if prefix != "" {
			fullKey = prefix + "." + yamlTag
		}

		sf := SchemaField{
			Key:     fullKey,
			YAMLKey: yamlTag,
		}

		ft := f.Type
		if ft.Kind() == reflect.Struct {
			sf.Type = "object"
			sf.Children = buildSchema(ft, fullKey)
		} else if ft.Kind() == reflect.Slice {
			sf.Type = "array"
		} else if ft.Kind() == reflect.Bool {
			sf.Type = "bool"
		} else if ft.Kind() == reflect.Int || ft.Kind() == reflect.Int64 || ft.Kind() == reflect.Int32 {
			sf.Type = "int"
		} else if ft.Kind() == reflect.Float64 || ft.Kind() == reflect.Float32 {
			sf.Type = "float"
		} else {
			sf.Type = "string"
		}

		// Mark sensitive fields
		if sensitiveKeys[yamlTag] || vaultTag != "" {
			sf.Sensitive = true
		}

		fields = append(fields, sf)
	}

	return fields
}

// handleRestart triggers an application restart by exiting with code 42
func handleRestart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.Logger.Info("[Config UI] Restart requested via Web UI")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "AuraGo wird neu gestartet...",
		})

		// Give the HTTP response time to flush before killing the process.
		go func() {
			time.Sleep(300 * time.Millisecond)
			os.Exit(42) // 42 → systemd Restart=on-failure / start.bat loop
		}()
	}
}

// handleOllamaModels proxies GET /api/tags on an Ollama host and
// returns the list of locally installed model names.
// Accepts an optional ?url= query parameter with the Ollama base URL.
// If omitted, falls back to the configured main LLM provider (must be ollama).
func handleOllamaModels(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		explicitURL := strings.TrimSpace(r.URL.Query().Get("url"))

		var ollamaHost string
		if explicitURL != "" {
			// Caller supplied a URL directly (e.g. from provider modal)
			ollamaHost = strings.TrimRight(explicitURL, "/")
			ollamaHost = strings.TrimSuffix(ollamaHost, "/v1")
			ollamaHost = strings.TrimRight(ollamaHost, "/")
		} else {
			// Fall back to the saved main LLM config
			s.CfgMu.RLock()
			provider := s.Cfg.LLM.ProviderType
			baseURL := s.Cfg.LLM.BaseURL
			s.CfgMu.RUnlock()

			if !strings.EqualFold(provider, "ollama") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"available": false,
					"reason":    "provider is not ollama",
				})
				return
			}
			ollamaHost = strings.TrimRight(baseURL, "/")
			ollamaHost = strings.TrimSuffix(ollamaHost, "/v1")
			ollamaHost = strings.TrimRight(ollamaHost, "/")
		}

		if ollamaHost == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    "no Ollama URL provided",
			})
			return
		}

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(ollamaHost + "/api/tags")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    "Failed to reach Ollama: " + err.Error(),
			})
			return
		}
		defer resp.Body.Close()

		var tagsResp struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
			http.Error(w, "Failed to parse Ollama response: "+err.Error(), http.StatusBadGateway)
			return
		}

		names := make([]string, 0, len(tagsResp.Models))
		for _, m := range tagsResp.Models {
			names = append(names, m.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"models": names,
		})
	}
}

// handleOpenRouterModels fetches the public model list from the OpenRouter API
// and returns it as JSON. The frontend uses this to display a model browser.
// No API key is required — the endpoint is public.
func handleOpenRouterModels(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get("https://openrouter.ai/api/v1/models")
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    "Failed to reach OpenRouter: " + err.Error(),
			})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"available": false,
				"reason":    fmt.Sprintf("OpenRouter returned HTTP %d", resp.StatusCode),
			})
			return
		}

		// Stream the response directly to reduce memory usage
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
	}
}

// handleMeshCentralTest attempts a login + WebSocket handshake against the MeshCentral server.
// Accepts an optional JSON body with fields {url, username, password}; any empty/omitted field
// falls back to the saved config value (password also falls back to the vault).
func handleMeshCentralTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		// Parse optional override values from request body.
		// Always attempt decode regardless of ContentLength (HTTP/2 may omit it).
		var body struct {
			URL        string `json:"url"`
			Username   string `json:"username"`
			Password   string `json:"password"`
			LoginToken string `json:"login_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		// Fall back to saved config
		s.CfgMu.RLock()
		url := body.URL
		if url == "" {
			url = s.Cfg.MeshCentral.URL
		}
		username := body.Username
		if username == "" {
			username = s.Cfg.MeshCentral.Username
		}
		password := body.Password
		if password == "" {
			password = s.Cfg.MeshCentral.Password
		}
		loginToken := body.LoginToken
		if loginToken == "" {
			loginToken = s.Cfg.MeshCentral.LoginToken
		}
		s.CfgMu.RUnlock()

		// Vault fallback for password / token
		if s.Vault != nil {
			if password == "" {
				if v, _ := s.Vault.ReadSecret("meshcentral_password"); v != "" {
					s.Logger.Info("[MeshCentral Test] Found password in vault")
					password = v
				}
			}
			if loginToken == "" {
				if v, _ := s.Vault.ReadSecret("meshcentral_token"); v != "" {
					s.Logger.Info("[MeshCentral Test] Found login token in vault", "tokenLength", len(v))
					loginToken = v
				} else {
					s.Logger.Info("[MeshCentral Test] No login token found in vault")
				}
			}
		} else {
			s.Logger.Info("[MeshCentral Test] Vault is nil")
		}

		// URL is always required. Username is required only when no login token is set.
		if url == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "URL is required (not set in config)",
			})
			return
		}
		if username == "" && loginToken == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Username or Login Token is required (not set in config)",
			})
			return
		}

		mc := meshcentral.NewClient(url, username, password, loginToken, s.Cfg.MeshCentral.Insecure)
		mc.SetLogger(s.Logger)
		if err := mc.Connect(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
		mc.Close()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"message": "Login and WebSocket handshake successful.",
		})
	}
}
