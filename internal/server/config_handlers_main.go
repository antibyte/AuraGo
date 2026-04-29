package server

import (
	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/sqlconnections"
	"aurago/internal/tools"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			jsonError(w, "Config path not set", http.StatusInternalServerError)
			return
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			s.Logger.Error("Failed to read config file", "error", err)
			jsonError(w, "Failed to read config", http.StatusInternalServerError)
			return
		}

		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			s.Logger.Error("Failed to parse config", "error", err)
			jsonError(w, "Failed to parse config", http.StatusInternalServerError)
			return
		}

		// Inject personality section from in-memory config when it is absent
		// from the raw YAML (migration scenario: old config only has agent.personality_*).
		if _, ok := rawCfg["personality"]; !ok {
			p := s.Cfg.Personality
			rawCfg["personality"] = map[string]interface{}{
				"engine":                   p.Engine,
				"engine_v2":                p.EngineV2,
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
				delete(pMap, "v2_provider")
				delete(pMap, "v2_model")
				delete(pMap, "v2_url")
				delete(pMap, "v2_api_key")
				delete(pMap, "v2_timeout_secs")
			}
		}

		if maSection, ok := rawCfg["memory_analysis"]; ok {
			if maMap, ok := maSection.(map[string]interface{}); ok {
				delete(maMap, "provider")
				delete(maMap, "model")
			}
		}

		if toolsSection, ok := rawCfg["tools"]; ok {
			if toolsMap, ok := toolsSection.(map[string]interface{}); ok {
				for _, key := range []string{"web_scraper", "wikipedia", "ddg_search", "pdf_extractor"} {
					if toolSection, ok := toolsMap[key]; ok {
						if toolMap, ok := toolSection.(map[string]interface{}); ok {
							delete(toolMap, "summary_provider")
						}
					}
				}
			}
		}
		injectDefaultToolPermissions(rawCfg, s.Cfg)
		injectRuntimeDockerDefaults(rawCfg, s.Cfg)

		// Mask sensitive fields
		maskSensitiveFields(rawCfg)

		// Inject masked indicators for vault-only secrets so the UI
		// shows "••••••••" for fields that have a value stored in the vault.
		injectVaultIndicators(rawCfg, s.Vault)
		// Inject feature availability flags so the UI can gray out
		// sections that are not functional in the current runtime.
		injectFeatureAvailability(rawCfg, s.Cfg.Runtime, s.Cfg.Agent.SudoEnabled)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rawCfg)
	}
}

func injectDefaultToolPermissions(rawCfg map[string]interface{}, cfg *config.Config) {
	if cfg == nil {
		return
	}
	toolsSection, ok := rawCfg["tools"].(map[string]interface{})
	if !ok {
		toolsSection = make(map[string]interface{})
		rawCfg["tools"] = toolsSection
	}
	kgSection, ok := toolsSection["knowledge_graph"].(map[string]interface{})
	if !ok {
		kgSection = make(map[string]interface{})
		toolsSection["knowledge_graph"] = kgSection
	}
	setDefaultBool(kgSection, "enabled", cfg.Tools.KnowledgeGraph.Enabled)
	setDefaultBool(kgSection, "readonly", cfg.Tools.KnowledgeGraph.ReadOnly)
	setDefaultBool(kgSection, "auto_extraction", cfg.Tools.KnowledgeGraph.AutoExtraction)
	setDefaultBool(kgSection, "prompt_injection", cfg.Tools.KnowledgeGraph.PromptInjection)
	setDefaultInt(kgSection, "max_prompt_nodes", cfg.Tools.KnowledgeGraph.MaxPromptNodes)
	setDefaultInt(kgSection, "max_prompt_chars", cfg.Tools.KnowledgeGraph.MaxPromptChars)
	setDefaultBool(kgSection, "retrieval_fusion", cfg.Tools.KnowledgeGraph.RetrievalFusion)
}

func setDefaultBool(section map[string]interface{}, key string, value bool) {
	if _, ok := section[key]; !ok {
		section[key] = value
	}
}

func setDefaultInt(section map[string]interface{}, key string, value int) {
	if _, ok := section[key]; !ok {
		section[key] = value
	}
}

func injectRuntimeDockerDefaults(rawCfg map[string]interface{}, cfg *config.Config) {
	if cfg == nil {
		return
	}
	host := strings.TrimSpace(cfg.Docker.Host)
	if host == "" {
		return
	}
	dockerSection, ok := rawCfg["docker"].(map[string]interface{})
	if !ok {
		dockerSection = make(map[string]interface{})
		rawCfg["docker"] = dockerSection
	}
	if rawHost, ok := dockerSection["host"]; !ok || strings.TrimSpace(fmt.Sprint(rawHost)) == "" {
		dockerSection["host"] = host
	}
	if _, ok := dockerSection["enabled"]; !ok {
		dockerSection["enabled"] = cfg.Docker.Enabled
	}
}

// handleUILanguage updates the UI language independently from the main config patch.
func handleUILanguage(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Language == "" {
			jsonError(w, "Language required", http.StatusBadRequest)
			return
		}

		s.CfgMu.Lock()
		s.Cfg.Server.UILanguage = body.Language
		if err := s.Cfg.Save(s.Cfg.ConfigPath); err != nil {
			s.Logger.Error("Failed to save UI language", "error", err)
			s.CfgMu.Unlock()
			jsonError(w, "Failed to save configuration", http.StatusInternalServerError)
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			jsonError(w, "Config path not set", http.StatusInternalServerError)
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
			jsonError(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var patch map[string]interface{}
		if err := json.Unmarshal(body, &patch); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Read the current config
		data, err := os.ReadFile(configPath)
		if err != nil {
			s.Logger.Error("Failed to read config file for patching", "error", err)
			jsonError(w, "Failed to read config", http.StatusInternalServerError)
			return
		}

		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			s.Logger.Error("Failed to parse config for patching", "error", err)
			jsonError(w, "Failed to parse config", http.StatusInternalServerError)
			return
		}
		rawCfg = normalizeConfigYAMLMap(rawCfg)

		// Deep merge the patch into the existing config, skipping masked password values.
		// Before merging, extract any secrets from the patch and write them to the vault
		// so they never end up in config.yaml.
		if vaultErr := extractSecretsToVault(patch, s.Vault, s.Logger); vaultErr != nil {
			s.Logger.Error("[Config] Credential could not be saved to vault", "error", vaultErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Credential could not be saved to vault.",
			})
			return
		}
		deepMerge(rawCfg, patch, "")
		rawCfg = normalizeConfigYAMLMap(rawCfg)

		// Write back
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			s.Logger.Error("Failed to marshal patched config", "error", err)
			jsonError(w, "Failed to save config", http.StatusInternalServerError)
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
				"message": "Config validation failed. Save rejected — your existing config is unchanged.",
			})
			return
		}

		// Mutual exclusion: Security Proxy (Caddy) and built-in HTTPS both want port 443.
		// If the Security Proxy is enabled, AuraGo runs as a plain HTTP backend behind it.
		// Enabling both at the same time will always cause a port conflict.
		if validateCfg.Server.HTTPS.Enabled && validateCfg.SecurityProxy.Enabled {
			s.Logger.Error("[Config] Security Proxy and built-in HTTPS are both enabled — save rejected")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "error",
				"message": "Security Proxy and built-in HTTPS cannot both be active: both compete for port 443. " +
					"The Security Proxy (Caddy) already handles TLS — AuraGo runs as plain HTTP behind it. " +
					"Disable either security_proxy or server.https.",
			})
			return
		}

		s.CfgMu.RLock()
		runtimeSnapshot := s.Cfg.Runtime
		s.CfgMu.RUnlock()
		if managedDockerErr := validateManagedDockerBackends(validateCfg, runtimeSnapshot); managedDockerErr != nil {
			s.Logger.Error("[Config] Managed Docker backend unavailable — save rejected", "error", managedDockerErr)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": managedDockerErr.Error(),
			})
			return
		}

		// Pre-flight: if HTTPS is being newly enabled or the port is changing, verify the
		// configured port is actually bindable RIGHT NOW. Reject the save if not.
		// Skip this check if HTTPS was already active on the same port — AuraGo itself
		// holds that port, so net.Listen would always fail even though everything is fine.
		if validateCfg.Server.HTTPS.Enabled {
			httpsPort := validateCfg.Server.HTTPS.HTTPSPort
			if httpsPort <= 0 {
				httpsPort = 443
			}
			s.CfgMu.RLock()
			currentHTTPS := s.Cfg.Server.HTTPS.Enabled
			currentPort := s.Cfg.Server.HTTPS.HTTPSPort
			s.CfgMu.RUnlock()
			if currentPort <= 0 {
				currentPort = 443
			}
			// Only run the bind-test if HTTPS is being switched on or the port changed
			runBindTest := !currentHTTPS || (currentPort != httpsPort)
			if runBindTest {
				if ln, bindErr := net.Listen("tcp", fmt.Sprintf(":%d", httpsPort)); bindErr != nil {
					errMsg := bindErr.Error()
					var userMsg string
					if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "access is denied") {
						userMsg = fmt.Sprintf(
							"Cannot enable HTTPS: port %d requires root or CAP_NET_BIND_SERVICE. "+
								"Use an unprivileged port (e.g. 8443) or run: sudo setcap cap_net_bind_service=+ep %s",
							httpsPort, os.Args[0])
					} else if strings.Contains(errMsg, "address already in use") || strings.Contains(errMsg, "bind: address already") {
						altPort := 8443
						if httpsPort == 8443 {
							altPort = 8444
						}
						userMsg = fmt.Sprintf(
							"Cannot enable HTTPS: port %d is already in use by another process (e.g. Security Proxy, Caddy, nginx). "+
								"Stop the conflicting service or use a different port (e.g. %d).",
							httpsPort, altPort)
					} else {
						userMsg = fmt.Sprintf("Cannot enable HTTPS: port %d is not available: %s", httpsPort, errMsg)
					}
					s.Logger.Error("[Config] HTTPS port pre-flight check failed — save rejected",
						"port", httpsPort, "error", bindErr)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"status":  "error",
						"message": userMsg,
					})
					return
				} else {
					ln.Close()
				}
			}
		}

		perm := os.FileMode(0o600)
		if info, statErr := os.Stat(configPath); statErr == nil {
			perm = info.Mode().Perm()
			if perm == 0 {
				perm = 0o600
			}
		}

		// Serialize concurrent config saves to prevent TOCTOU: read-modify-write race.
		s.CfgSaveMu.Lock()
		writeErr := config.WriteFileAtomic(configPath, out, perm)
		s.CfgSaveMu.Unlock()
		if writeErr != nil {
			s.Logger.Error("Failed to write config file", "error", writeErr)
			jsonError(w, "Failed to write config", http.StatusInternalServerError)
			return
		}

		// Snapshot old config under read lock, then do file I/O without holding any lock.
		s.CfgMu.RLock()
		oldCfg := *s.Cfg // snapshot before reload
		s.CfgMu.RUnlock()

		// Load new config outside any lock (disk I/O must not block readers).
		newCfg, loadErr := config.Load(configPath)

		// Hot-reload: re-parse config and apply to running instance
		s.CfgMu.Lock()

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
			newCfg.Runtime = oldCfg.Runtime

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
			if oldCfg.Tools.DaemonSkills != newCfg.Tools.DaemonSkills {
				needsRestart = true
				restartReasons = append(restartReasons, "Daemon Skills")
			}
			if oldCfg.Agent.SudoUnrestricted != newCfg.Agent.SudoUnrestricted {
				if newCfg.Agent.SudoUnrestricted && newCfg.Runtime.ProtectSystemStrict {
					needsRestart = true
					restartReasons = append(restartReasons, "Sudo system-wide write access (systemd unit update required)")
				}
			}

			newCfg.ConfigPath = s.Cfg.ConfigPath

			// Reconfigure the live LLM client when model, API key, base URL,
			// provider or fallback settings have changed. This ensures that model
			// name changes in the web UI take effect immediately without a restart.
			if llmHotReloadChanged(oldCfg, *newCfg) || oldCfg.FallbackLLM != newCfg.FallbackLLM {
				if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
					fm.Reconfigure(newCfg)
					s.Logger.Info("[Config UI] LLM client reconfigured",
						"model", newCfg.LLM.Model,
						"provider", newCfg.LLM.ProviderType,
						"base_url", newCfg.LLM.BaseURL)
				}

				// Re-detect context window and recalculate budget when model
				// changes and the user has budget set to automatic (0).
				if newCfg.Agent.SystemPromptTokenBudgetAuto {
					detected := llm.DetectContextWindow(
						newCfg.LLM.BaseURL, newCfg.LLM.APIKey,
						newCfg.LLM.Model, newCfg.LLM.ProviderType, s.Logger)
					if detected > 0 {
						newCfg.Agent.ContextWindow = detected
					}
					if newCfg.Agent.ContextWindow > 0 {
						newCfg.Agent.SystemPromptTokenBudget, newCfg.Agent.ContextWindow =
							llm.AutoConfigureBudget(newCfg.Agent.ContextWindow, newCfg.Agent.SystemPromptTokenBudget, s.Logger)
					}
				}
			}

			// Apply hot-reload by publishing a new immutable config snapshot after
			// all synchronous auto-detection adjustments are complete.
			s.replaceConfigSnapshot(newCfg)

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
			if oldCfg.CoAgents.CleanupIntervalMins != newCfg.CoAgents.CleanupIntervalMins ||
				oldCfg.CoAgents.CleanupMaxAgeMins != newCfg.CoAgents.CleanupMaxAgeMins {
				s.CoAgentRegistry.ConfigureLifecycle(
					time.Duration(newCfg.CoAgents.CleanupIntervalMins)*time.Minute,
					time.Duration(newCfg.CoAgents.CleanupMaxAgeMins)*time.Minute,
				)
				s.Logger.Info("[Config UI] Co-agent cleanup lifecycle updated",
					"interval_minutes", newCfg.CoAgents.CleanupIntervalMins,
					"max_age_minutes", newCfg.CoAgents.CleanupMaxAgeMins)
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
			// reinitBudgetTracker also re-registers the MissionManagerV2 callback.
			s.reinitBudgetTracker(newCfg)
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

			// Hot-reload Heartbeat scheduler when heartbeat settings change
			if oldCfg.Heartbeat != newCfg.Heartbeat {
				if s.HeartbeatScheduler != nil {
					s.HeartbeatScheduler.Restart(newCfg)
					s.Logger.Info("[Config UI] Heartbeat scheduler restarted")
				}
			}

			if oldCfg.UptimeKuma != newCfg.UptimeKuma {
				s.restartUptimeKumaPoller()
				s.Logger.Info("[Config UI] Uptime Kuma poller restarted")
			}

			// Hot-reload File Indexer: start/stop based on enabled flag change
			if oldCfg.Indexing.Enabled != newCfg.Indexing.Enabled {
				if newCfg.Indexing.Enabled && s.FileIndexer == nil {
					s.FileIndexer = services.NewFileIndexer(newCfg, &s.CfgMu, s.LongTermMem, s.ShortTermMem, s.Logger)
					s.attachFileKGSyncer()
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

			// Auto-start Browser Automation sidecar when the integration becomes active
			// or relevant sidecar settings change.
			browserAutomationChanged := oldCfg.BrowserAutomation != newCfg.BrowserAutomation ||
				oldCfg.Tools.BrowserAutomation.Enabled != newCfg.Tools.BrowserAutomation.Enabled ||
				oldCfg.Directories.WorkspaceDir != newCfg.Directories.WorkspaceDir
			if browserAutomationChanged &&
				newCfg.BrowserAutomation.Enabled &&
				newCfg.BrowserAutomation.AutoStart &&
				newCfg.Tools.BrowserAutomation.Enabled &&
				strings.EqualFold(newCfg.BrowserAutomation.Mode, "sidecar") {
				if sidecarCfg, err := tools.ResolveBrowserAutomationSidecarConfig(newCfg); err != nil {
					s.Logger.Warn("[Config UI] Failed to resolve browser automation sidecar config", "error", err)
				} else {
					go func() {
						// Stop and remove the old container so it gets recreated
						// with updated env vars (viewport, TTL, read-only, etc.).
						tools.StopBrowserAutomationSidecar(newCfg.Docker.Host, sidecarCfg, s.Logger)
						tools.EnsureBrowserAutomationSidecarRunning(newCfg.Docker.Host, sidecarCfg, s.Logger)
					}()
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

			// Auto-start / stop Homepage dev container when homepage.enabled flag changes.
			homepageDevToggled := oldCfg.Homepage.Enabled != newCfg.Homepage.Enabled
			homepageDevPathChanged := newCfg.Homepage.Enabled && oldCfg.Homepage.WorkspacePath != newCfg.Homepage.WorkspacePath
			if homepageDevToggled || homepageDevPathChanged {
				if newCfg.Homepage.Enabled && newCfg.Homepage.WorkspacePath != "" {
					go func() {
						homepageCfg := tools.HomepageConfig{
							DockerHost:       newCfg.Docker.Host,
							WorkspacePath:    newCfg.Homepage.WorkspacePath,
							WebServerPort:    newCfg.Homepage.WebServerPort,
							AllowLocalServer: newCfg.Homepage.AllowLocalServer,
						}
						result := tools.HomepageInit(homepageCfg, s.Logger)
						s.Logger.Info("[Config UI] Homepage dev container auto-started", "result", result)
					}()
				} else if newCfg.Homepage.Enabled && newCfg.Homepage.WorkspacePath == "" {
					s.Logger.Warn("[Config UI] Homepage dev container enabled but workspace_path is not set — cannot start")
				} else {
					go func() {
						homepageCfg := tools.HomepageConfig{DockerHost: newCfg.Docker.Host}
						tools.HomepageStop(homepageCfg, s.Logger)
						s.Logger.Info("[Config UI] Homepage dev container stopped")
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

			// Hot-reload SQL Connections pool: create when enabled, close when disabled.
			if newCfg.SQLConnections.Enabled != oldCfg.SQLConnections.Enabled {
				if newCfg.SQLConnections.Enabled && s.SQLConnectionsDB != nil && s.SQLConnectionPool == nil {
					pool := sqlconnections.NewConnectionPool(
						s.SQLConnectionsDB, s.Vault,
						newCfg.SQLConnections.MaxPoolSize,
						newCfg.SQLConnections.ConnectionTimeoutSec,
						s.Logger,
					)
					if newCfg.SQLConnections.RateLimitWindowSec > 0 {
						pool.SetRateLimit(newCfg.SQLConnections.RateLimitWindowSec)
					}
					if newCfg.SQLConnections.IdleTTLSec > 0 {
						pool.SetIdleTTL(time.Duration(newCfg.SQLConnections.IdleTTLSec) * time.Second)
					}
					s.SQLConnectionPool = pool
					s.Logger.Info("[Config UI] SQL connection pool created")
				} else if !newCfg.SQLConnections.Enabled && s.SQLConnectionPool != nil {
					s.SQLConnectionPool.CloseAll()
					s.SQLConnectionPool = nil
					s.Logger.Info("[Config UI] SQL connection pool closed")
				}
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

			// Hot-reload Cloudflare Tunnel: stop immediately when disabled, start when enabled.
			cfEnabledChanged := oldCfg.CloudflareTunnel.Enabled != newCfg.CloudflareTunnel.Enabled
			if cfEnabledChanged {
				cfBaseCfg := tools.CloudflareTunnelConfig{
					Enabled:        newCfg.CloudflareTunnel.Enabled,
					Mode:           newCfg.CloudflareTunnel.Mode,
					AuthMethod:     newCfg.CloudflareTunnel.AuthMethod,
					TunnelName:     newCfg.CloudflareTunnel.TunnelName,
					AccountID:      newCfg.CloudflareTunnel.AccountID,
					TunnelID:       newCfg.CloudflareTunnel.TunnelID,
					LoopbackPort:   newCfg.CloudflareTunnel.LoopbackPort,
					ExposeWebUI:    newCfg.CloudflareTunnel.ExposeWebUI,
					ExposeHomepage: newCfg.CloudflareTunnel.ExposeHomepage,
					MetricsPort:    newCfg.CloudflareTunnel.MetricsPort,
					LogLevel:       newCfg.CloudflareTunnel.LogLevel,
					DockerHost:     newCfg.Docker.Host,
					DataDir:        newCfg.Directories.DataDir,
					WebUIPort:      newCfg.Server.Port,
					HomepagePort:   newCfg.Homepage.WebServerPort,
					HTTPSEnabled:   newCfg.Server.HTTPS.Enabled,
					HTTPSPort:      newCfg.Server.HTTPS.HTTPSPort,
				}
				vault := s.Vault
				reg := s.Registry
				log := s.Logger
				if !newCfg.CloudflareTunnel.Enabled {
					// Disabled → stop the tunnel immediately (security: no tunnel without explicit enable).
					go func() {
						result := tools.CloudflareTunnelStop(cfBaseCfg, reg, log)
						log.Info("[CloudflareTunnel] Hot-reload: tunnel stopped because cloudflare_tunnel.enabled=false", "result", result)
					}()
				} else if newCfg.CloudflareTunnel.AutoStart {
					// Enabled with auto_start → start immediately.
					go func() {
						result := tools.CloudflareTunnelStart(cfBaseCfg, vault, reg, log)
						log.Info("[CloudflareTunnel] Hot-reload: tunnel started because cloudflare_tunnel.enabled=true", "result", result)
					}()
				}
			}

			// Hot-reload Cloudflare Tunnel: restart when the expose target (web UI vs. homepage)
			// changes so the dynamic loopback proxy picks up the new setting immediately.
			cfExposeChanged := oldCfg.CloudflareTunnel.ExposeWebUI != newCfg.CloudflareTunnel.ExposeWebUI ||
				oldCfg.CloudflareTunnel.ExposeHomepage != newCfg.CloudflareTunnel.ExposeHomepage
			if cfExposeChanged && newCfg.CloudflareTunnel.Enabled {
				cfTunnelCfg := tools.CloudflareTunnelConfig{
					Enabled:        newCfg.CloudflareTunnel.Enabled,
					ReadOnly:       newCfg.CloudflareTunnel.ReadOnly,
					Mode:           newCfg.CloudflareTunnel.Mode,
					AutoStart:      newCfg.CloudflareTunnel.AutoStart,
					AuthMethod:     newCfg.CloudflareTunnel.AuthMethod,
					TunnelName:     newCfg.CloudflareTunnel.TunnelName,
					AccountID:      newCfg.CloudflareTunnel.AccountID,
					TunnelID:       newCfg.CloudflareTunnel.TunnelID,
					LoopbackPort:   newCfg.CloudflareTunnel.LoopbackPort,
					ExposeWebUI:    newCfg.CloudflareTunnel.ExposeWebUI,
					ExposeHomepage: newCfg.CloudflareTunnel.ExposeHomepage,
					MetricsPort:    newCfg.CloudflareTunnel.MetricsPort,
					LogLevel:       newCfg.CloudflareTunnel.LogLevel,
					DockerHost:     newCfg.Docker.Host,
					DataDir:        newCfg.Directories.DataDir,
					WebUIPort:      newCfg.Server.Port,
					HomepagePort:   newCfg.Homepage.WebServerPort,
					HTTPSEnabled:   newCfg.Server.HTTPS.Enabled,
					HTTPSPort:      newCfg.Server.HTTPS.HTTPSPort,
				}
				vault := s.Vault
				reg := s.Registry
				log := s.Logger
				go func() {
					result := tools.CloudflareTunnelRestart(cfTunnelCfg, vault, reg, log)
					log.Info("[CloudflareTunnel] Hot-reload: tunnel restarted due to expose target change", "result", result)
				}()
			}

			loopbackPortChanged := DedicatedInternalLoopbackPort(&oldCfg) != DedicatedInternalLoopbackPort(newCfg)
			if loopbackPortChanged && s.loopbackHandler != nil {
				// Stop the old listener if it exists.
				if s.loopbackSrv != nil {
					s.loopbackSrv.Close()
					s.loopbackSrv = nil
				}
				newPort := DedicatedInternalLoopbackPort(newCfg)
				if newPort > 0 {
					bindAddr := fmt.Sprintf("127.0.0.1:%d", newPort)
					if ln, bindErr := net.Listen("tcp4", bindAddr); bindErr != nil {
						s.Logger.Warn("[Loopback] Hot-reload: could not bind internal listener",
							"addr", bindAddr, "error", bindErr)
					} else {
						s.Logger.Info("[Loopback] Hot-reload: internal HTTP listener started", "port", newPort)
						s.loopbackSrv = &http.Server{
							Handler:      s.loopbackHandler,
							ReadTimeout:  30 * time.Second,
							WriteTimeout: 5 * time.Minute,
							IdleTimeout:  2 * time.Minute,
						}
						go func() {
							if serveErr := s.loopbackSrv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
								s.Logger.Warn("[Loopback] Hot-reload: internal listener stopped", "error", serveErr)
							}
						}()
					}
				} else if newPort == 0 {
					s.Logger.Info("[Loopback] Hot-reload: internal listener disabled")
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

func validateManagedDockerBackends(cfg config.Config, rt config.Runtime) error {
	if strings.TrimSpace(cfg.Docker.Host) == "" {
		cfg.Docker.Host = strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	}
	needsManagedDocker := cfg.Embeddings.LocalOllama.Enabled || cfg.Ollama.ManagedInstance.Enabled
	if !needsManagedDocker {
		return nil
	}
	if !cfg.Docker.Enabled {
		return fmt.Errorf("Docker integration is disabled. Enable Docker before using managed Ollama containers for local models or embeddings")
	}
	if strings.TrimSpace(cfg.Docker.Host) != "" {
		return nil
	}
	if rt.IsDocker && !rt.DockerSocketOK {
		return fmt.Errorf("Docker endpoint not reachable. Start the docker-proxy, mount /var/run/docker.sock, or configure docker.host before using managed Ollama containers")
	}
	return nil
}

func managedDockerConfigFromRaw(rawCfg map[string]interface{}) config.Config {
	var cfg config.Config
	if dockerSection := rawMap(rawCfg, "docker"); dockerSection != nil {
		cfg.Docker.Enabled = rawBool(dockerSection, "enabled")
		cfg.Docker.Host = rawString(dockerSection, "host")
	}
	if strings.TrimSpace(cfg.Docker.Host) == "" {
		cfg.Docker.Host = strings.TrimSpace(os.Getenv("DOCKER_HOST"))
	}
	if embeddingsSection := rawMap(rawCfg, "embeddings"); embeddingsSection != nil {
		if localOllama := rawMap(embeddingsSection, "local_ollama"); localOllama != nil {
			cfg.Embeddings.LocalOllama.Enabled = rawBool(localOllama, "enabled")
		}
	}
	if ollamaSection := rawMap(rawCfg, "ollama"); ollamaSection != nil {
		if managedInstance := rawMap(ollamaSection, "managed_instance"); managedInstance != nil {
			cfg.Ollama.ManagedInstance.Enabled = rawBool(managedInstance, "enabled")
		}
	}
	return cfg
}

func rawMap(parent map[string]interface{}, key string) map[string]interface{} {
	value, ok := parent[key]
	if !ok {
		return nil
	}
	if m, ok := value.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func rawBool(parent map[string]interface{}, key string) bool {
	value, ok := parent[key]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func rawString(parent map[string]interface{}, key string) string {
	value, ok := parent[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func llmHotReloadChanged(oldCfg config.Config, newCfg config.Config) bool {
	type llmFingerprint struct {
		Provider                     string
		LegacyURL                    string
		LegacyAPIKey                 string
		LegacyModel                  string
		HelperEnabled                bool
		HelperProvider               string
		HelperModel                  string
		UseNativeFunctions           bool
		Temperature                  float64
		StructuredOutputs            bool
		Multimodal                   bool
		MultimodalProviderTypesExtra []string
	}
	oldFP := llmFingerprint{
		Provider:                     oldCfg.LLM.Provider,
		LegacyURL:                    oldCfg.LLM.LegacyURL,
		LegacyAPIKey:                 oldCfg.LLM.LegacyAPIKey,
		LegacyModel:                  oldCfg.LLM.LegacyModel,
		HelperEnabled:                oldCfg.LLM.HelperEnabled,
		HelperProvider:               oldCfg.LLM.HelperProvider,
		HelperModel:                  oldCfg.LLM.HelperModel,
		UseNativeFunctions:           oldCfg.LLM.UseNativeFunctions,
		Temperature:                  oldCfg.LLM.Temperature,
		StructuredOutputs:            oldCfg.LLM.StructuredOutputs,
		Multimodal:                   oldCfg.LLM.Multimodal,
		MultimodalProviderTypesExtra: oldCfg.LLM.MultimodalProviderTypesExtra,
	}
	newFP := llmFingerprint{
		Provider:                     newCfg.LLM.Provider,
		LegacyURL:                    newCfg.LLM.LegacyURL,
		LegacyAPIKey:                 newCfg.LLM.LegacyAPIKey,
		LegacyModel:                  newCfg.LLM.LegacyModel,
		HelperEnabled:                newCfg.LLM.HelperEnabled,
		HelperProvider:               newCfg.LLM.HelperProvider,
		HelperModel:                  newCfg.LLM.HelperModel,
		UseNativeFunctions:           newCfg.LLM.UseNativeFunctions,
		Temperature:                  newCfg.LLM.Temperature,
		StructuredOutputs:            newCfg.LLM.StructuredOutputs,
		Multimodal:                   newCfg.LLM.Multimodal,
		MultimodalProviderTypesExtra: newCfg.LLM.MultimodalProviderTypesExtra,
	}
	return !reflect.DeepEqual(oldFP, newFP)
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
	"shared_key":     true, // egg mode shared AES key — never expose
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
				// Special handling for providers: merge by ID to preserve existing providers
				if fullPath == "providers" {
					mergeProvidersByID(dst, sv)
					continue
				}
				// Special handling for budget.models: ensure all items are proper objects
				if fullPath == "budget.models" {
					// Protect against clearing non-empty models array with empty incoming
					if len(sv) == 0 {
						if existing, ok := dst[key].([]interface{}); ok && len(existing) > 0 {
							continue // keep existing non-empty array
						}
					}
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

// mergeProvidersByID merges an incoming providers array into the existing config
// by provider ID, preserving providers not present in the incoming list.
func mergeProvidersByID(dst map[string]interface{}, incoming []interface{}) {
	existing, _ := dst["providers"].([]interface{})

	// Build map of existing providers by ID
	byID := make(map[string]map[string]interface{})
	order := make([]string, 0)
	seen := make(map[string]bool)
	for _, item := range existing {
		if p, ok := item.(map[string]interface{}); ok {
			if id, _ := p["id"].(string); id != "" && !seen[id] {
				byID[id] = p
				order = append(order, id)
				seen[id] = true
			}
		}
	}

	// Update existing or insert new providers
	for _, item := range incoming {
		if p, ok := item.(map[string]interface{}); ok {
			id, _ := p["id"].(string)
			if id == "" {
				continue
			}
			if existing, exists := byID[id]; exists {
				// Merge fields into existing provider entry
				for k, v := range p {
					existing[k] = v
				}
			} else {
				copy := make(map[string]interface{}, len(p))
				for k, v := range p {
					copy[k] = v
				}
				byID[id] = copy
				order = append(order, id)
			}
		}
	}

	// Rebuild ordered list
	merged := make([]interface{}, 0, len(order))
	for _, id := range order {
		if p, ok := byID[id]; ok {
			merged = append(merged, p)
		}
	}
	dst["providers"] = merged
}

// vaultKeyMap maps dotted YAML paths to vault key names for static config fields.
// Dynamic fields (providers, email accounts) are handled in their own PUT handlers.
var vaultKeyMap = map[string]string{
	"ai_gateway.token":                 "ai_gateway_token",
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
	"tts.minimax.api_key":              "tts_minimax_api_key",
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
	"uptime_kuma.api_key":              "uptime_kuma_api_key",
	"egg_mode.shared_key":              "egg_shared_key",
	"google_workspace.client_secret":   "google_workspace_client_secret",
	"onedrive.client_secret":           "onedrive_client_secret",
	"paperless_ngx.api_token":          "paperless_ngx_api_token",
	"netlify.token":                    "netlify_token",
	"vercel.token":                     "vercel_token",
	"telnyx.api_key":                   "telnyx_api_key",
	"cloudflare_tunnel.token":          "cloudflared_token",
	"a2a.auth.api_key":                 "a2a_api_key",
	"a2a.auth.bearer_secret":           "a2a_bearer_secret",
	"truenas.api_key":                  "truenas_api_key",
	"jellyfin.api_key":                 "jellyfin_api_key",
	"obsidian.api_key":                 "obsidian_api_key",
	"ldap.bind_password":               "ldap_bind_password",
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
