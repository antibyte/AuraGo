package server

import (
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/setup"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"gopkg.in/yaml.v3"
)

// setupCSRF holds the active CSRF token for the setup wizard.
// It is generated once when the status endpoint is first called and
// validated on every state-changing request (POST).
var (
	setupCSRFToken string
	setupCSRFOnce  sync.Once
)

// generateSetupCSRF creates a cryptographically random 32-byte hex token.
func generateSetupCSRF() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Should never happen — fall back to time-based token.
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

// handleSetupStatus returns whether the setup wizard should be shown.
func handleSetupStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		show := needsSetup(s.Cfg)
		s.CfgMu.RUnlock()

		resp := map[string]interface{}{
			"needs_setup": show,
		}

		// Include CSRF token only when setup is still needed.
		if show {
			setupCSRFOnce.Do(func() { setupCSRFToken = generateSetupCSRF() })
			resp["csrf_token"] = setupCSRFToken
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// handleSetupSave processes the Quick Setup wizard form submission.
// It uses the same deep-merge strategy as handleUpdateConfig to safely
// patch the running config without losing existing values.
func handleSetupSave(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, "Setup already completed", http.StatusForbidden)
			return
		}

		// CSRF protection: the token was issued via GET /api/setup/status.
		if token := r.Header.Get("X-CSRF-Token"); token == "" || token != setupCSRFToken {
			s.Logger.Warn("[Setup] CSRF token mismatch")
			jsonError(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			jsonError(w, "Config path not set", http.StatusInternalServerError)
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
			jsonError(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var patch map[string]interface{}
		if err := json.Unmarshal(body, &patch); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		setupPassword, authEnabled, err := extractSetupAdminPassword(patch, s.Cfg.Auth.Enabled, s.Cfg.Auth.PasswordHash != "")
		if err != nil {
			jsonError(w, setupValidationMessage(err), http.StatusBadRequest)
			return
		}

		// Read current config file
		data, err := os.ReadFile(configPath)
		if err != nil {
			s.Logger.Error("[Setup] Failed to read config file", "error", err)
			jsonError(w, "Failed to read config", http.StatusInternalServerError)
			return
		}

		var rawCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &rawCfg); err != nil {
			s.Logger.Error("[Setup] Failed to parse config", "error", err)
			jsonError(w, "Failed to parse config", http.StatusInternalServerError)
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

			// Extract TTS API keys into vault. TTS has its own config structure
			// (tts.minimax.api_key, tts.elevenlabs.api_key) separate from the
			// provider system. The vault keys match config_migrate.go conventions.
			if tts, ok := patch["tts"].(map[string]interface{}); ok {
				for _, sub := range []struct {
					key      string
					vaultKey string
				}{
					{"minimax", "tts_minimax_api_key"},
					{"elevenlabs", "tts_elevenlabs_api_key"},
				} {
					if section, ok := tts[sub.key].(map[string]interface{}); ok {
						if key, _ := section["api_key"].(string); key != "" {
							if werr := s.Vault.WriteSecret(sub.vaultKey, key); werr != nil {
								s.Logger.Warn("[Setup] Failed to write TTS API key to vault", "provider", sub.key, "error", werr)
							} else {
								s.Logger.Info("[Setup] TTS API key stored in vault", "provider", sub.key)
							}
							delete(section, "api_key")
						}
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
			jsonError(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
			s.Logger.Error("[Setup] Failed to write config", "error", err)
			jsonError(w, "Failed to write config", http.StatusInternalServerError)
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
			restartReasons = append(restartReasons, "config reload failed")
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

			// Re-create BudgetTracker and re-register MissionManagerV2 callback.
			s.reinitBudgetTracker(s.Cfg)

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
				jsonError(w, "Failed to hash admin password", http.StatusInternalServerError)
				return
			}
			newSecret, err := GenerateRandomHex(32)
			if err != nil {
				s.Logger.Error("[Setup] Failed to generate session secret", "error", err)
				jsonError(w, "Failed to generate session secret", http.StatusInternalServerError)
				return
			}
			if err := patchAuthConfig(s, map[string]interface{}{
				"enabled":        true,
				"password_hash":  newHash,
				"session_secret": newSecret,
			}); err != nil {
				s.Logger.Error("[Setup] Failed to persist admin password", "error", err)
				jsonError(w, "Failed to save admin password", http.StatusInternalServerError)
				return
			}
			s.Logger.Info("[Setup] Admin password initialized")
		}

		w.Header().Set("Content-Type", "application/json")
		if needsRestart {
			msg := fmt.Sprintf("Saved. Restart required for: %s", strings.Join(restartReasons, ", "))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         "saved",
				"message":        msg,
				"needs_restart":  true,
				"restart_reason": restartReasons,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "saved",
				"message":       "Setup complete! Configuration saved and applied.",
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

// handleSetupTestConnection performs a lightweight LLM connectivity test using
// the provider details supplied by the setup wizard. It creates a temporary
// client, sends a minimal completion request, and returns success or an error
// message so the user can verify their API key/URL before saving.
func handleSetupTestConnection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		show := needsSetup(s.Cfg)
		s.CfgMu.RUnlock()
		if !show {
			jsonError(w, "Setup already completed", http.StatusForbidden)
			return
		}

		var req struct {
			ProviderType string `json:"provider_type"`
			BaseURL      string `json:"base_url"`
			APIKey       string `json:"api_key"`
			Model        string `json:"model"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		if req.Model == "" {
			jsonError(w, "Model is required", http.StatusBadRequest)
			return
		}

		client := llm.NewClientFromProvider(req.ProviderType, req.BaseURL, req.APIKey)

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		_, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model: req.Model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleUser, Content: "respond with ok"},
			},
			MaxTokens: 5,
		})

		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			s.Logger.Warn("[Setup] Test connection failed", "provider", req.ProviderType, "error", err)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":      true,
			"message": "Connection successful",
		})
	}
}

// handleSetupProfiles returns the list of pre-configured provider profiles
// for the setup wizard plan selection step.
func handleSetupProfiles(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		profiles := setup.LoadProfiles("", s.Logger)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"profiles": profiles,
		})
	}
}
