package server

import (
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// handleSetupStatus returns whether the setup wizard should be shown.
func handleSetupStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		show := needsSetup(s.Cfg)
		s.CfgMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"needs_setup": show,
		})
	}
}

// handleSetupSave processes the Quick Setup wizard form submission.
// It uses the same deep-merge strategy as handleUpdateConfig to safely
// patch the running config without losing existing values.
func handleSetupSave(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Security: refuse setup writes once initial configuration is complete.
		// The endpoint is unauthenticated (required for first-run wizard), so we
		// must prevent it from being abused to overwrite a fully-configured instance.
		// An already-authenticated request (e.g. from the Config UI) can still
		// reach handleUpdateConfig which IS behind the auth middleware.
		s.CfgMu.RLock()
		alreadyConfigured := !needsSetup(s.Cfg)
		s.CfgMu.RUnlock()
		if alreadyConfigured {
			s.Logger.Warn("[Setup] POST to /api/setup rejected — setup already completed")
			http.Error(w, "Setup already completed", http.StatusForbidden)
			return
		}

		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			http.Error(w, "Config path not set", http.StatusInternalServerError)
			return
		}

		// Read body with size limit
		maxBody := s.Cfg.Server.MaxBodyBytes
		if maxBody <= 0 {
			maxBody = 10 << 20
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
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		setupPassword, authEnabled, err := extractSetupAdminPassword(patch, s.Cfg.Auth.Enabled, s.Cfg.Auth.PasswordHash != "")
		if err != nil {
			http.Error(w, setupValidationMessage(err), http.StatusBadRequest)
			return
		}

		// Read current config file
		data, err := os.ReadFile(configPath)
		if err != nil {
			s.Logger.Error("[Setup] Failed to read config file", "error", err)
			http.Error(w, "Failed to read config", http.StatusInternalServerError)
			return
		}

		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			s.Logger.Error("[Setup] Failed to parse config", "error", err)
			http.Error(w, "Failed to parse config", http.StatusInternalServerError)
			return
		}

		// Extract and persist provider API keys into the vault BEFORE merging
		// into the YAML. ProviderEntry.APIKey has yaml:"-" so it is vault-only
		// and must never be stored as plaintext in config.yaml.
		if s.Vault != nil {
			if providers, ok := patch["providers"].([]interface{}); ok {
				for _, item := range providers {
					if prov, ok := item.(map[string]interface{}); ok {
						id, _ := prov["id"].(string)
						key, _ := prov["api_key"].(string)
						if id != "" && key != "" {
							vaultKey := "provider_" + id + "_api_key"
							if werr := s.Vault.WriteSecret(vaultKey, key); werr != nil {
								s.Logger.Warn("[Setup] Failed to write API key to vault", "provider", id, "error", werr)
							} else {
								s.Logger.Info("[Setup] Provider API key stored in vault", "provider", id)
							}
						}
						// Remove api_key from the map so it is never written to YAML
						delete(prov, "api_key")
					}
				}
			}
		}

		// Deep merge the setup patch into existing config
		deepMerge(rawCfg, patch, "")

		// Write back
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			s.Logger.Error("[Setup] Failed to marshal config", "error", err)
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
			s.Logger.Error("[Setup] Failed to write config", "error", err)
			http.Error(w, "Failed to write config", http.StatusInternalServerError)
			return
		}

		s.Logger.Info("[Setup] Configuration saved via Quick Setup wizard")

		// Hot-reload: re-parse and apply
		s.CfgMu.Lock()
		newCfg, loadErr := config.Load(configPath)

		needsRestart := false
		restartReasons := []string{}

		if loadErr != nil {
			s.Logger.Warn("[Setup] Hot-reload failed, changes saved but require restart", "error", loadErr)
			needsRestart = true
			restartReasons = append(restartReasons, "Parse-Fehler beim Reload")
		} else {
			savedPath := s.Cfg.ConfigPath
			*s.Cfg = *newCfg
			s.Cfg.ConfigPath = savedPath

			// Apply vault secrets (including the provider API keys just saved above)
			// so the in-memory config reflects the full resolved configuration and
			// needsSetup() returns false on the next request.
			s.Cfg.ApplyVaultSecrets(s.Vault)
			// Re-resolve providers so vault API keys propagate into cfg.LLM.APIKey etc.
			// (same sequence as main.go: ApplyVaultSecrets → ResolveProviders)
			s.Cfg.ResolveProviders()

			// Re-create BudgetTracker
			s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, s.Cfg.Directories.DataDir)

			// Reconfigure the live LLM client with the new API key / base URL / model.
			// Without this the old client (created at startup with an empty key from
			// config_template.yaml) would be used for the first chat after setup.
			if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
				fm.Reconfigure(s.Cfg)
				s.Logger.Info("[Setup] LLM client reconfigured",
					"provider", s.Cfg.LLM.ProviderType,
					"base_url", s.Cfg.LLM.BaseURL)
			}

			// Re-initialize the VectorDB (LTM / embeddings) if it was disabled at
			// startup because no API key was available yet, but the setup wizard
			// has now configured one.  Without this, long-term memory stays broken
			// until the next process restart.
			if s.LongTermMem != nil && s.LongTermMem.IsDisabled() &&
				s.Cfg.Embeddings.Provider != "" && s.Cfg.Embeddings.Provider != "disabled" {
				if newVDB, vdbErr := memory.NewChromemVectorDB(s.Cfg, s.Logger); vdbErr == nil {
					s.LongTermMem = newVDB
					s.Logger.Info("[Setup] VectorDB re-initialized with embedding provider",
						"provider", s.Cfg.Embeddings.Provider)
				} else {
					s.Logger.Warn("[Setup] VectorDB re-initialization failed — embeddings unavailable until restart", "error", vdbErr)
				}
			}

			s.Logger.Info("[Setup] Configuration hot-reloaded successfully")
		}
		s.CfgMu.Unlock()

		if authEnabled && s.Cfg.Auth.PasswordHash == "" {
			newHash, err := HashPassword(setupPassword)
			if err != nil {
				s.Logger.Error("[Setup] Failed to hash admin password", "error", err)
				http.Error(w, "Failed to hash admin password", http.StatusInternalServerError)
				return
			}
			newSecret, err := GenerateRandomHex(32)
			if err != nil {
				s.Logger.Error("[Setup] Failed to generate session secret", "error", err)
				http.Error(w, "Failed to generate session secret", http.StatusInternalServerError)
				return
			}
			if err := patchAuthConfig(s, map[string]interface{}{
				"enabled":        true,
				"password_hash":  newHash,
				"session_secret": newSecret,
			}); err != nil {
				s.Logger.Error("[Setup] Failed to persist admin password", "error", err)
				http.Error(w, "Failed to save admin password", http.StatusInternalServerError)
				return
			}
			s.Logger.Info("[Setup] Admin password initialized")
		}

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
				"message":       "Setup abgeschlossen! Konfiguration gespeichert und angewendet.",
				"needs_restart": false,
			})
		}
	}
}

func extractSetupAdminPassword(patch map[string]interface{}, currentAuthEnabled bool, currentPasswordSet bool) (string, bool, error) {
	authEnabled := currentAuthEnabled
	authPatch, ok := patch["auth"].(map[string]interface{})
	if !ok || authPatch == nil {
		if authEnabled && !currentPasswordSet {
			return "", authEnabled, fmt.Errorf("admin password is required")
		}
		return "", authEnabled, nil
	}
	if rawEnabled, exists := authPatch["enabled"]; exists {
		enabled, ok := rawEnabled.(bool)
		if !ok {
			return "", authEnabled, fmt.Errorf("auth.enabled must be a boolean")
		}
		authEnabled = enabled
	}
	rawPassword, hasPassword := authPatch["admin_password"]
	delete(authPatch, "admin_password")

	if !authEnabled {
		return "", false, nil
	}
	if !hasPassword {
		if currentPasswordSet {
			return "", true, nil
		}
		return "", true, fmt.Errorf("admin password is required")
	}
	password, ok := rawPassword.(string)
	if !ok {
		return "", true, fmt.Errorf("admin password must be a string")
	}
	password = strings.TrimSpace(password)
	if len(password) < 8 {
		return "", true, fmt.Errorf("admin password must be at least 8 characters long")
	}
	return password, true, nil
}

func setupValidationMessage(err error) string {
	if err == nil {
		return "Invalid setup configuration"
	}
	switch err.Error() {
	case "admin password is required",
		"auth.enabled must be a boolean",
		"admin password must be a string",
		"admin password must be at least 8 characters long":
		return err.Error()
	default:
		return "Invalid setup configuration"
	}
}

// needsSetup returns true if the Quick Setup wizard should be shown.
// We check that at least one provider with a non-empty API key (or OAuth)
// exists and is referenced by the main LLM slot.
func needsSetup(cfg *config.Config) bool {
	llmConfigured := false
	// If the LLM has a resolved API key, the provider side is configured.
	// This covers new-format configs where the key is loaded from vault.
	if cfg.LLM.APIKey != "" || cfg.LLM.LegacyAPIKey != "" {
		llmConfigured = true
	}
	if !llmConfigured {
		if len(cfg.Providers) == 0 || cfg.LLM.Provider == "" {
			return true
		}
		// Walk providers: accept any that has a key, OAuth, or is a key-less
		// local endpoint (Ollama) identified by type="ollama" with URL and model set.
		// NOTE: cloud providers (openrouter, openai, etc.) with BaseURL+model but no key
		// must NOT be treated as configured — they still need an API key entered by the user.
		for _, p := range cfg.Providers {
			if p.APIKey != "" || p.AuthType == "oauth2" {
				llmConfigured = true
				break
			}
			if p.Type == "ollama" && p.BaseURL != "" && p.Model != "" {
				llmConfigured = true
				break
			}
		}
	}
	if !llmConfigured {
		return true
	}
	return cfg.Auth.Enabled && cfg.Auth.PasswordHash == ""
}
