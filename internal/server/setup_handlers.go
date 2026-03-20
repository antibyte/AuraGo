package server

import (
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
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
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
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

		// Deep merge the setup patch into existing config
		deepMerge(rawCfg, patch, "")

		// Write back
		out, err := yaml.Marshal(rawCfg)
		if err != nil {
			s.Logger.Error("[Setup] Failed to marshal config", "error", err)
			http.Error(w, "Failed to save config", http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(configPath, out, 0644); err != nil {
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

			// Re-create BudgetTracker
			s.BudgetTracker = budget.NewTracker(newCfg, s.Logger, newCfg.Directories.DataDir)

			// Reconfigure the live LLM client with the new API key / base URL / model.
			// Without this the old client (created at startup with an empty key from
			// config_template.yaml) would be used for the first chat after setup.
			if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
				fm.Reconfigure(s.Cfg)
				s.Logger.Info("[Setup] LLM client reconfigured",
					"provider", newCfg.LLM.ProviderType,
					"base_url", newCfg.LLM.BaseURL)
			}

			s.Logger.Info("[Setup] Configuration hot-reloaded successfully")
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
				"message":       "Setup abgeschlossen! Konfiguration gespeichert und angewendet.",
				"needs_restart": false,
			})
		}
	}
}

// needsSetup returns true if the Quick Setup wizard should be shown.
// We check that at least one provider with a non-empty API key (or OAuth)
// exists and is referenced by the main LLM slot.
func needsSetup(cfg *config.Config) bool {
	// If the LLM has a resolved API key, setup is complete.
	// This covers new-format configs where the key is loaded from vault.
	if cfg.LLM.APIKey != "" {
		return false
	}
	// If the setup wizard recently saved a key in legacy inline format
	// (llm.api_key in YAML) and the provider entry hasn't been migrated yet.
	if cfg.LLM.LegacyAPIKey != "" {
		return false
	}
	if len(cfg.Providers) == 0 {
		return true
	}
	if cfg.LLM.Provider == "" {
		return true
	}
	// Walk providers: accept any that has a key, OAuth, or is a key-less
	// local endpoint (Ollama) identified by type="ollama" with URL and model set.
	// NOTE: cloud providers (openrouter, openai, etc.) with BaseURL+model but no key
	// must NOT be treated as configured — they still need an API key entered by the user.
	for _, p := range cfg.Providers {
		if p.APIKey != "" || p.AuthType == "oauth2" {
			return false
		}
		if p.Type == "ollama" && p.BaseURL != "" && p.Model != "" {
			return false // key-less local provider (Ollama)
		}
	}
	return true // all providers lack usable credentials → still needs setup
}
