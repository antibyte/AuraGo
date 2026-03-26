package server

import (
	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/tools"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

		// Inject personality section from in-memory config when it is absent
		// from the raw YAML (migration scenario: old config only has agent.personality_*).
		if _, ok := rawCfg["personality"]; !ok {
			p := s.Cfg.Personality
			rawCfg["personality"] = map[string]interface{}{
				"engine":                   p.Engine,
				"engine_v2":                p.EngineV2,
				"v2_provider":              p.V2Provider,
				"core_personality":         p.CorePersonality,
				"user_profiling":           p.UserProfiling,
				"user_profiling_threshold": p.UserProfilingThreshold,
				"emotion_synthesizer": map[string]interface{}{
					"enabled":                p.EmotionSynthesizer.Enabled,
					"min_interval_seconds":   p.EmotionSynthesizer.MinIntervalSecs,
					"max_history_entries":    p.EmotionSynthesizer.MaxHistoryEntries,
					"trigger_on_mood_change": p.EmotionSynthesizer.TriggerOnMoodChange,
					"trigger_always":         p.EmotionSynthesizer.TriggerAlways,
				},
			}
		}

		// Strip legacy personality fields that are no longer shown in the UI.
		// These fields (v2_model, v2_url, v2_api_key, v2_timeout_secs) may still
		// exist in the raw YAML of older configs but must not be rendered by the UI.
		if pSection, ok := rawCfg["personality"]; ok {
			if pMap, ok := pSection.(map[string]interface{}); ok {
				delete(pMap, "v2_model")
				delete(pMap, "v2_url")
				delete(pMap, "v2_api_key")
				delete(pMap, "v2_timeout_secs")
			}
		}

		// Mask sensitive fields
		maskSensitiveFields(rawCfg)

		// Inject masked indicators for vault-only secrets so the UI
		// shows “••••••••” for fields that have a value stored in the vault.
		injectVaultIndicators(rawCfg, s.Vault)
		// Inject feature availability flags so the UI can gray out
		// sections that are not functional in the current runtime.
		injectFeatureAvailability(rawCfg, s.Cfg.Runtime)
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
		deepMerge(rawCfg, patch, "")

		// Write back
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			s.Logger.Error("Failed to marshal patched config", "error", err)
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		// Safety net: validate that the marshaled YAML can still be loaded
		// into a Config struct. If not, reject the save and keep the old file.
		var validateCfg config.Config
		if valErr := yaml.Unmarshal(out, &validateCfg); valErr != nil {
			s.Logger.Error("[Config] Pre-write validation failed — save rejected to protect config", "error", valErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Config validation failed: " + valErr.Error() + ". Save rejected — your existing config is unchanged.",
			})
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
		embeddingsChanged := false

		if loadErr != nil {
			s.Logger.Warn("[Config UI] Hot-reload failed, changes saved but require restart", "error", loadErr)
			needsRestart = true
			restartReasons = append(restartReasons, "Parse-Fehler beim Reload")
		} else {
			// Apply vault secrets before comparison so vault-only fields match
			newCfg.ApplyVaultSecrets(s.Vault)
			newCfg.ResolveProviders()
			newCfg.ApplyOAuthTokens(s.Vault)

			// Carry over runtime detection (computed once at startup, not on reload)
			newCfg.Runtime = s.Cfg.Runtime

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
			if embeddingsConfigChanged(oldCfg, *newCfg) {
				embeddingsChanged = true
				needsRestart = true
				restartReasons = append(restartReasons, "Embeddings / Langzeitgedächtnis")
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

			// Re-create LLMGuardian when its settings change so the new model,
			// provider and protection level take effect without a restart.
			oldG, newG := oldCfg.LLMGuardian, newCfg.LLMGuardian
			guardianChanged := oldG.Enabled != newG.Enabled ||
				oldG.Provider != newG.Provider ||
				oldG.Model != newG.Model ||
				oldG.DefaultLevel != newG.DefaultLevel ||
				oldG.FailSafe != newG.FailSafe ||
				oldG.TimeoutSecs != newG.TimeoutSecs ||
				oldG.CacheTTL != newG.CacheTTL
			if guardianChanged {
				s.LLMGuardian = security.NewLLMGuardian(newCfg, s.Logger)
				if newCfg.LLMGuardian.Enabled {
					s.Logger.Info("[Config UI] LLMGuardian reconfigured",
						"model", newCfg.LLMGuardian.ResolvedModel,
						"level", newCfg.LLMGuardian.DefaultLevel,
						"provider", newCfg.LLMGuardian.Provider)
				} else {
					s.Logger.Info("[Config UI] LLMGuardian disabled")
				}
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

			// Auto-start Gotenberg container if document_creator just became active
			if newCfg.Tools.DocumentCreator.Enabled && strings.EqualFold(newCfg.Tools.DocumentCreator.Backend, "gotenberg") {
				if !oldCfg.Tools.DocumentCreator.Enabled || !strings.EqualFold(oldCfg.Tools.DocumentCreator.Backend, "gotenberg") {
					go tools.EnsureGotenbergRunning(newCfg.Docker.Host, s.Logger)
				}
			}

			// Auto-start / stop Security Proxy (Caddy) container when enabled flag changes
			if oldCfg.SecurityProxy.Enabled != newCfg.SecurityProxy.Enabled {
				if newCfg.SecurityProxy.Enabled {
					go func() {
						if err := s.ProxyManager.Start(); err != nil {
							s.Logger.Error("[Config UI] Failed to auto-start security proxy", "error", err)
						} else {
							s.Logger.Info("[Config UI] Security proxy auto-started")
						}
					}()
				} else {
					go func() {
						if err := s.ProxyManager.Stop(); err != nil {
							s.Logger.Warn("[Config UI] Failed to stop security proxy", "error", err)
						} else {
							s.Logger.Info("[Config UI] Security proxy stopped")
						}
					}()
				}
			}

			// Auto-start / stop Homepage web server (Caddy) when webserver_enabled flag changes.
			// Also restart if workspace_path changed while webserver is enabled.
			webserverToggled := oldCfg.Homepage.WebServerEnabled != newCfg.Homepage.WebServerEnabled
			webserverPathChanged := newCfg.Homepage.WebServerEnabled && oldCfg.Homepage.WorkspacePath != newCfg.Homepage.WorkspacePath
			if webserverToggled || webserverPathChanged {
				if newCfg.Homepage.WebServerEnabled && newCfg.Homepage.WorkspacePath != "" {
					go func() {
						homepageCfg := tools.HomepageConfig{
							DockerHost:            newCfg.Docker.Host,
							WorkspacePath:         newCfg.Homepage.WorkspacePath,
							WebServerPort:         newCfg.Homepage.WebServerPort,
							WebServerDomain:       newCfg.Homepage.WebServerDomain,
							WebServerInternalOnly: newCfg.Homepage.WebServerInternalOnly,
							AllowLocalServer:      newCfg.Homepage.AllowLocalServer,
						}
						result := tools.HomepageWebServerStart(homepageCfg, "", "", s.Logger)
						s.Logger.Info("[Config UI] Homepage web server auto-started", "result", result)
					}()
				} else if newCfg.Homepage.WebServerEnabled && newCfg.Homepage.WorkspacePath == "" {
					s.Logger.Warn("[Config UI] Homepage web server enabled but workspace_path is not set — cannot start")
				} else {
					go func() {
						homepageCfg := tools.HomepageConfig{DockerHost: newCfg.Docker.Host}
						tools.HomepageWebServerStop(homepageCfg, s.Logger)
						s.Logger.Info("[Config UI] Homepage web server stopped")
					}()
				}
			}

			// Auto-start local Ollama embeddings container if just enabled
			if newCfg.Embeddings.LocalOllama.Enabled && !oldCfg.Embeddings.LocalOllama.Enabled {
				go tools.EnsureOllamaEmbeddingsRunning(newCfg, s.Logger)
			}

			// Auto-start managed Ollama container if just enabled
			if newCfg.Ollama.ManagedInstance.Enabled && !oldCfg.Ollama.ManagedInstance.Enabled {
				go tools.EnsureOllamaManagedRunning(newCfg, s.Logger)
			}
			// Stop managed Ollama container if just disabled
			if !newCfg.Ollama.ManagedInstance.Enabled && oldCfg.Ollama.ManagedInstance.Enabled {
				go func() {
					tools.StopOllamaManagedContainer(newCfg.Docker.Host)
					s.Logger.Info("[Config UI] Managed Ollama container stopped")
				}()
			}

			// Auto-start Piper TTS container if just enabled
			if newCfg.TTS.Piper.Enabled && !oldCfg.TTS.Piper.Enabled {
				go tools.EnsurePiperRunning(newCfg, s.Logger)
			}

			// Ansible sidecar lifecycle management
			if newCfg.Ansible.Enabled && newCfg.Ansible.Mode == "sidecar" {
				inventoryDir := ""
				if newCfg.Ansible.DefaultInventory != "" {
					inventoryDir = filepath.Dir(newCfg.Ansible.DefaultInventory)
				}
				sidecarCfg := tools.AnsibleSidecarConfig{
					Token:         newCfg.Ansible.Token,
					Timeout:       newCfg.Ansible.Timeout,
					Image:         newCfg.Ansible.Image,
					ContainerName: newCfg.Ansible.ContainerName,
					PlaybooksDir:  newCfg.Ansible.PlaybooksDir,
					InventoryDir:  inventoryDir,
					AutoBuild:     newCfg.Ansible.AutoBuild,
					DockerfileDir: newCfg.Ansible.DockerfileDir,
				}
				if !oldCfg.Ansible.Enabled || oldCfg.Ansible.Mode != "sidecar" {
					// Newly enabled — create/start container
					go tools.EnsureAnsibleSidecarRunning(newCfg.Docker.Host, sidecarCfg, s.Logger)
				} else if newCfg.Ansible.Token != oldCfg.Ansible.Token && newCfg.Ansible.Token != "" {
					// Token changed while already running — recreate container to apply new token
					go tools.ReapplyAnsibleToken(newCfg.Docker.Host, sidecarCfg, s.Logger)
				}
			}

			// Reconcile tsnet exposure live when the web exposure toggles change
			// while the node is already connected to the Tailscale network.
			tsExposeChanged := oldCfg.Tailscale.TsNet.ServeHTTP != newCfg.Tailscale.TsNet.ServeHTTP ||
				oldCfg.Tailscale.TsNet.ExposeHomepage != newCfg.Tailscale.TsNet.ExposeHomepage ||
				oldCfg.Tailscale.TsNet.Funnel != newCfg.Tailscale.TsNet.Funnel ||
				oldCfg.Homepage.WebServerEnabled != newCfg.Homepage.WebServerEnabled ||
				oldCfg.Homepage.WebServerPort != newCfg.Homepage.WebServerPort
			if s.TsNetManager != nil && tsExposeChanged {
				tsStatus := s.TsNetManager.GetStatus()
				if tsStatus.Running {
					if s.tsNetHandler != nil {
						go func() {
							if err := s.TsNetManager.ReconfigureExposure(s.tsNetHandler); err != nil {
								s.Logger.Warn("[Config UI] tsnet exposure reconfigure failed", "error", err)
							} else {
								s.Logger.Info("[Config UI] tsnet exposure reconfigured")
							}
						}()
					} else if !newCfg.Tailscale.TsNet.ServeHTTP && !newCfg.Tailscale.TsNet.ExposeHomepage {
						go func() {
							if err := s.TsNetManager.DowngradeToNetworkOnly(); err != nil {
								s.Logger.Warn("[Config UI] tsnet downgrade to network-only failed", "error", err)
							} else {
								s.Logger.Info("[Config UI] tsnet downgraded to network-only mode")
							}
						}()
					}
				}
			}

			s.Logger.Info("[Config UI] Configuration hot-reloaded successfully")
		}
		s.CfgMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if needsRestart {
			msg := fmt.Sprintf("Gespeichert. Neustart nötig für: %s", strings.Join(restartReasons, ", "))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":             "saved",
				"message":            msg,
				"needs_restart":      true,
				"restart_reason":     restartReasons,
				"embeddings_changed": embeddingsChanged,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":             "saved",
				"message":            "Konfiguration gespeichert und sofort angewendet.",
				"needs_restart":      false,
				"embeddings_changed": embeddingsChanged,
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
	"access_key":     true, // S3 access key ID
	"secret_key":     true, // S3 secret access key
	"secret":         true, // Proxmox token secret and similar vault-backed secrets
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
// path tracks the dotted YAML path for context-aware decisions.
func deepMerge(dst, src map[string]interface{}, path string) {
	for key, srcVal := range src {
		fullPath := key
		if path != "" {
			fullPath = path + "." + key
		}
		switch sv := srcVal.(type) {
		case map[string]interface{}:
			// Recurse into nested maps
			if dstMap, ok := dst[key].(map[string]interface{}); ok {
				deepMerge(dstMap, sv, fullPath)
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
				if fullPath == "budget.models" {
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
					// Protect against empty arrays overwriting non-empty existing arrays.
					// This prevents accidental clearing of configured lists when saving
					// a section where the field happened to be empty in the DOM.
					if len(sv) == 0 {
						if existing, ok := dst[key].([]interface{}); ok && len(existing) > 0 {
							continue // keep existing non-empty array
						}
					}
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
			// If the existing value is a slice, never overwrite it with a plain string.
			// This happens e.g. when an empty textarea is saved without data-type="array-lines":
			// the JS sends "" but the YAML field is []string.
			if _, dstIsSlice := dst[key].([]interface{}); dstIsSlice {
				if sv == "" {
					// keep existing empty slice, don't corrupt it to a string
					continue
				}
				// non-empty string for a slice field: ignore silently
				continue
			}
			dst[key] = srcVal
		default:
			// JSON numbers decode as float64 in Go. If the value is an exact integer,
			// store it as int so yaml.Marshal writes it as an integer (e.g. 123456789)
			// rather than scientific notation (1.23456789e+08). The latter cannot be
			// unmarshaled back into int64 config fields, causing them to reset to 0.
			if f, ok := srcVal.(float64); ok && !math.IsInf(f, 0) && !math.IsNaN(f) && math.Trunc(f) == f {
				dst[key] = int(f)
			} else {
				dst[key] = srcVal
			}
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
	"tailscale.tsnet.auth_key":         "tailscale_tsnet_authkey",
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
	"webdav.token":                     "webdav_token",
	"koofr.app_password":               "koofr_password",
	"s3.access_key":                    "s3_access_key",
	"s3.secret_key":                    "s3_secret_key",
	"proxmox.secret":                   "proxmox_secret",
	"github.token":                     "github_token",
	"rocketchat.auth_token":            "rocketchat_auth_token",
	"mqtt.password":                    "mqtt_password",
	"email.password":                   "email_password",
	"notifications.pushover.user_key":  "pushover_user_key",
	"notifications.pushover.app_token": "pushover_app_token",
	"adguard.password":                 "adguard_password",
	"google_workspace.client_secret":   "google_workspace_client_secret",
	"onedrive.client_secret":           "onedrive_client_secret",
	"paperless_ngx.api_token":          "paperless_ngx_api_token",
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
			_, isMappedVaultPath := vaultKeyMap[fullPath]
			if !sensitiveKeys[key] && !isMappedVaultPath {
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

// handleSecurityHints returns the current list of security hints for the running config.
// Requires an active session (auth-gated via server_routes.go WebConfig.Enabled block).
func handleSecurityHints(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		hints := CheckSecurity(s.Cfg)
		facing := isInternetFacing(s.Cfg)
		s.CfgMu.RUnlock()

		// Build a serialisable view (strip FixPatch — applied server-side only)
		type hintView struct {
			ID          string `json:"id"`
			Severity    string `json:"severity"`
			Title       string `json:"title"`
			Description string `json:"description"`
			AutoFixable bool   `json:"auto_fixable"`
		}
		views := make([]hintView, len(hints))
		for i, h := range hints {
			views[i] = hintView{
				ID:          h.ID,
				Severity:    h.Severity,
				Title:       h.Title,
				Description: h.Description,
				AutoFixable: h.AutoFixable,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hints":           views,
			"internet_facing": facing,
		})
	}
}

// handleSecurityHarden applies auto-fixable hardening patches selected by the user.
// Expects JSON body: {"ids": ["auth_disabled", "n8n_no_token", ...]}.
func handleSecurityHarden(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		applied, err := ApplyHardening(s, req.IDs)
		if err != nil {
			s.Logger.Error("[Security] Hardening failed", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"applied": applied,
			"message": fmt.Sprintf("%d hardening measure(s) applied.", len(applied)),
		})
	}
}

// handleAnsibleGenerateToken generates a cryptographically secure random token,
// saves it to the vault, and (if the sidecar is already running) recreates the
// container so the new token is active immediately.
func handleAnsibleGenerateToken(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Generate a 32-byte cryptographically secure random token
		b := make([]byte, 32)
		if _, err := crand.Read(b); err != nil {
			s.Logger.Error("[Ansible] Failed to generate token", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate token"})
			return
		}
		token := hex.EncodeToString(b)

		// Persist to vault
		if s.Vault != nil {
			if err := s.Vault.WriteSecret("ansible_token", token); err != nil {
				s.Logger.Error("[Ansible] Failed to save token to vault", "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save token to vault"})
				return
			}
			s.Logger.Info("[Config] Secret saved to vault", "key", "ansible_token")
		}

		// Update live config
		s.CfgMu.Lock()
		s.Cfg.Ansible.Token = token
		cfg := *s.Cfg
		s.CfgMu.Unlock()

		// If sidecar is enabled, recreate the container with the new token
		if cfg.Ansible.Enabled && cfg.Ansible.Mode == "sidecar" {
			inventoryDir := ""
			if cfg.Ansible.DefaultInventory != "" {
				inventoryDir = filepath.Dir(cfg.Ansible.DefaultInventory)
			}
			go tools.ReapplyAnsibleToken(cfg.Docker.Host, tools.AnsibleSidecarConfig{
				Token:         token,
				Timeout:       cfg.Ansible.Timeout,
				Image:         cfg.Ansible.Image,
				ContainerName: cfg.Ansible.ContainerName,
				PlaybooksDir:  cfg.Ansible.PlaybooksDir,
				InventoryDir:  inventoryDir,
				AutoBuild:     cfg.Ansible.AutoBuild,
				DockerfileDir: cfg.Ansible.DockerfileDir,
			}, s.Logger)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
			"token":  token,
		})
	}
}

// handleOllamaManagedStatus returns the current status of the managed Ollama container.
func handleOllamaManagedStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		dockerHost := s.Cfg.Docker.Host
		managed := s.Cfg.Ollama.ManagedInstance.Enabled
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if !managed {
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "disabled"})
			return
		}
		result := tools.OllamaManagedContainerStatus(dockerHost)
		w.Write([]byte(result))
	}
}

// handleOllamaManagedRecreate calls EnsureOllamaManagedRunning to create/start
// the managed Ollama container. This allows the user to recover after the
// container was manually deleted via the container management page.
func handleOllamaManagedRecreate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		cfg := *s.Cfg
		s.CfgMu.RUnlock()

		if !cfg.Ollama.ManagedInstance.Enabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Managed Ollama instance is not enabled."})
			return
		}

		s.Logger.Info("[Config UI] Ollama managed container recreate requested via Web UI")
		go tools.EnsureOllamaManagedRunning(&cfg, s.Logger)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "Container creation started in background.",
		})
	}
}
