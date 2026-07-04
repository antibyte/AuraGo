package server

import (
	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/setup"
	"aurago/internal/warnings"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const setupCSRFTokenTTL = 30 * time.Minute

var validateSetupProviderSSRF = security.ValidateSSRF

var (
	setupProfilesCache     []setup.SetupProfile
	setupProfilesCacheOnce sync.Once
)

// debugForceSetup, when true, makes needsSetup always return true so the Quick
// Setup wizard is shown on every request. Used as a temporary debugging aid
// while iterating on the setup UI. Revert (set to false) once debugging is
// complete — never ship with this enabled.
var debugForceSetup = false

// loadCachedSetupProfiles returns the embedded setup profiles, parsed once.
// Safe to call from multiple goroutines concurrently.
func loadCachedSetupProfiles(logger *slog.Logger) []setup.SetupProfile {
	setupProfilesCacheOnce.Do(func() {
		setupProfilesCache = setup.LoadProfiles("", logger)
	})
	return setupProfilesCache
}

// startSetupCSRFCleanup launches a background goroutine that prunes expired
// tokens every 5 minutes. It runs until the process exits and is safe to call
// from any code path that issues or validates tokens.
func startSetupCSRFCleanup(s *Server) {
	s.SetupCSRFCleanupOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for now := range ticker.C {
				s.SetupCSRFMu.Lock()
				pruneExpiredSetupCSRFTokensLocked(s.SetupCSRFTokens, now)
				s.SetupCSRFMu.Unlock()
			}
		}()
	})
}

// generateSetupCSRF creates a cryptographically random 32-byte hex token.
func generateSetupCSRF() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Should never happen — fall back to time-based token.
		return hex.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return hex.EncodeToString(b)
}

func issueSetupCSRFToken(s *Server) string {
	startSetupCSRFCleanup(s) // idempotent
	token := generateSetupCSRF()
	now := time.Now()
	s.SetupCSRFMu.Lock()
	defer s.SetupCSRFMu.Unlock()
	if s.SetupCSRFTokens == nil {
		s.SetupCSRFTokens = make(map[string]time.Time)
	}
	pruneExpiredSetupCSRFTokensLocked(s.SetupCSRFTokens, now)
	s.SetupCSRFTokens[token] = now.Add(setupCSRFTokenTTL)
	return token
}

func validateSetupCSRFToken(s *Server, token string, consume bool) bool {
	if token == "" {
		return false
	}
	now := time.Now()
	s.SetupCSRFMu.Lock()
	defer s.SetupCSRFMu.Unlock()
	if s.SetupCSRFTokens == nil {
		s.SetupCSRFTokens = make(map[string]time.Time)
	}
	pruneExpiredSetupCSRFTokensLocked(s.SetupCSRFTokens, now)
	expiry, ok := s.SetupCSRFTokens[token]
	if !ok || now.After(expiry) {
		delete(s.SetupCSRFTokens, token)
		return false
	}
	if consume {
		delete(s.SetupCSRFTokens, token)
	}
	return true
}

func pruneExpiredSetupCSRFTokensLocked(tokens map[string]time.Time, now time.Time) {
	for token, expiry := range tokens {
		if now.After(expiry) {
			delete(tokens, token)
		}
	}
}

// handleSetupStatus returns whether the setup wizard should be shown.
func handleSetupStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		show := needsSetup(s.Cfg)
		s.CfgMu.RUnlock()

		resp := map[string]interface{}{
			"needs_setup":     show,
			"is_docker":       s.Cfg.Runtime.IsDocker,
			"ollama_base_url": setupOllamaBaseURL(s.Cfg.Runtime.IsDocker),
		}

		// Issue a fresh CSRF token on every status request when setup is needed.
		if show {
			resp["csrf_token"] = issueSetupCSRFToken(s)
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
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
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
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_already_completed"), http.StatusForbidden)
			return
		}

		// CSRF protection: the token was issued via GET /api/setup/status.
		// Final setup saves consume the token to prevent replay.
		if !validateSetupCSRFToken(s, r.Header.Get("X-CSRF-Token"), true) {
			s.Logger.Warn("[Setup] CSRF token mismatch")
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_invalid_csrf_token"), http.StatusForbidden)
			return
		}

		configPath := s.Cfg.ConfigPath
		if configPath == "" {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_config_path_not_set"), http.StatusInternalServerError)
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
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_failed_read_request_body"), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var patch map[string]interface{}
		if err := json.Unmarshal(body, &patch); err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_invalid_json"), http.StatusBadRequest)
			return
		}

		authPatch, _ := patch["auth"].(map[string]interface{})
		setupPassword, authEnabled, err := validateSetupAdminPassword(authPatch, s.Cfg.Auth.Enabled, s.Cfg.Auth.PasswordHash != "")
		if err != nil {
			jsonError(w, setupValidationMessage(err), http.StatusBadRequest)
			return
		}
		stripSetupAdminPassword(authPatch)

		// Pre-extract provider API keys into the vault BEFORE applyConfigPatch.
		// Provider keys live at dynamic paths (providers[N].api_key) which the
		// generic extractSecretsToVault helper cannot reach — vault keys are
		// derived as "provider_<id>_api_key". Strip them from the patch so they
		// never reach config.yaml (ProviderEntry.APIKey has yaml:"-").
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
						delete(prov, "api_key")
					}
				}
			}
		}

		// Apply the patch: extract secrets to vault (TTS keys via vaultKeyMap),
		// deep-merge, write to disk, reload and resolve vault secrets. This is the
		// shared read/merge/write/reload sequence used by /api/setup and /api/config.
		reloadedCfg, err := applyConfigPatch(s, patch)
		if err != nil {
			s.Logger.Error("[Setup] Failed to apply config patch", "error", err)
			// Marshal/YAML errors are user-input issues (e.g., bad profile data
			// with mismatched quotes). Other errors (read/write/disk) are
			// server-side and should keep their 500 status.
			msg := err.Error()
			isUserError := strings.Contains(msg, "marshal") || strings.Contains(msg, "yaml") || strings.Contains(msg, "unmarshal")
			status := http.StatusInternalServerError
			if isUserError {
				status = http.StatusBadRequest
			}
			if isUserError {
				jsonErrorWithDetails(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_failed_save_config"), msg, status)
			} else {
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_failed_save_config"), status)
			}
			return
		}

		// Managed-Docker backend validation runs AFTER the write/reload so the
		// merged config (not the raw patch) is what gets validated. This matches
		// the behavior of handleUpdateConfig.
		s.CfgMu.RLock()
		runtimeSnapshot := s.Cfg.Runtime
		s.CfgMu.RUnlock()
		if managedDockerErr := validateManagedDockerBackends(*reloadedCfg, runtimeSnapshot); managedDockerErr != nil {
			s.Logger.Error("[Setup] Managed Docker backend unavailable — save rejected", "error", managedDockerErr)
			jsonError(w, managedDockerErr.Error(), http.StatusBadRequest)
			return
		}

		s.Logger.Info("[Setup] Configuration saved via Quick Setup wizard")

		if authEnabled && s.Cfg.Auth.PasswordHash == "" {
			newHash, err := HashPassword(setupPassword)
			if err != nil {
				s.Logger.Error("[Setup] Failed to hash admin password", "error", err)
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_internal_error"), http.StatusInternalServerError)
				return
			}
			newSecret, err := GenerateRandomHex(32)
			if err != nil {
				s.Logger.Error("[Setup] Failed to generate session secret", "error", err)
				jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_failed_generate_secret"), http.StatusInternalServerError)
				return
			}
			if err := patchAuthConfig(s, map[string]interface{}{
				"enabled":        true,
				"password_hash":  newHash,
				"session_secret": newSecret,
			}); err != nil {
				s.Logger.Error("[Setup] Failed to persist admin password", "error", err)
				jsonErrorWithDetails(w, i18n.T(s.Cfg.Server.UILanguage, "backend.auth_failed_save_config"), err.Error(), http.StatusInternalServerError)
				return
			}
			s.Logger.Info("[Setup] Admin password initialized")
		}

		// Hot-reload: the config has already been read, vault-secrets applied,
		// and providers resolved by applyConfigPatch. We just need to swap the
		// in-memory snapshot and rewire the live subsystems.
		needsRestart := false
		restartReasons := []string{}
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					s.Logger.Error("[Setup] Panic during hot-reload after save", "panic", rec)
					needsRestart = true
					restartReasons = append(restartReasons, "live reload panic")
				}
			}()

			s.CfgMu.Lock()
			defer s.CfgMu.Unlock()

			newCfg := reloadedCfg // already loaded + vault-secrets applied

			s.replaceConfigSnapshot(newCfg)

			// Re-create BudgetTracker and re-register MissionManagerV2 callback.
			s.reinitBudgetTracker(newCfg)

			// Reconfigure the live LLM client with the new API key / base URL / model.
			// Without this the old client (created at startup with an empty key from
			// config_template.yaml) would be used for the first chat after setup.
			if fm, ok := s.LLMClient.(*llm.FailoverManager); ok {
				fm.Reconfigure(newCfg)
				s.Logger.Info("[Setup] LLM client reconfigured",
					"provider", newCfg.LLM.ProviderType,
					"base_url", newCfg.LLM.BaseURL)
			} else {
				// s.LLMClient is not a FailoverManager (e.g., a future client type
				// or a test double). The setup-saved config is correct on disk and
				// in s.Cfg, but the in-memory client still uses the old values. A
				// process restart is required for the new API key to take effect.
				s.Logger.Warn("[Setup] LLM client is not a FailoverManager; restart may be required for new API key to take effect",
					"client_type", fmt.Sprintf("%T", s.LLMClient))
				needsRestart = true
				restartReasons = append(restartReasons, "llm client not FailoverManager")
			}

			// Re-initialize the VectorDB (LTM / embeddings) if it was disabled at
			// startup because no API key was available yet, but the setup wizard
			// has now configured one.  Without this, long-term memory stays broken
			// until the next process restart.
			if s.LongTermMem != nil && s.LongTermMem.IsDisabled() &&
				newCfg.Embeddings.Provider != "" && newCfg.Embeddings.Provider != "disabled" {
				if newVDB, vdbErr := memory.NewChromemVectorDB(newCfg, s.Logger); vdbErr == nil {
					if closeErr := s.LongTermMem.Close(); closeErr != nil {
						s.Logger.Warn("[Setup] Failed to close previous disabled VectorDB during re-initialization", "error", closeErr)
					}
					s.LongTermMem = newVDB
					if cols, colsErr := s.ShortTermMem.GetIndexedCollections(); colsErr == nil {
						s.LongTermMem.RegisterCollections(cols)
					}
					toolGuidesDir := filepath.Join(newCfg.Directories.PromptsDir, "tools_manuals")
					newVDB.IndexToolGuidesAsync(toolGuidesDir, false)
					docDir := filepath.Join(filepath.Dir(newCfg.ConfigPath), "documentation")
					if _, statErr := os.Stat(docDir); statErr == nil {
						newVDB.IndexDirectoryAsync(docDir, "documentation", s.ShortTermMem, false)
					}
					warnings.WatchVectorDBRecovery(s.WarningsRegistry, newCfg, newVDB, s.Logger)
					s.Logger.Info("[Setup] VectorDB re-initialized with embedding provider",
						"provider", newCfg.Embeddings.Provider)
				} else {
					s.Logger.Warn("[Setup] VectorDB re-initialization failed — embeddings unavailable until restart", "error", vdbErr)
				}
			}

			s.Logger.Info("[Setup] Configuration hot-reloaded successfully")
		}()

		w.Header().Set("Content-Type", "application/json")
		if needsRestart {
			msg := i18n.T(s.Cfg.Server.UILanguage, "backend.setup_restart_required", strings.Join(restartReasons, ", "))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         "saved",
				"message":        msg,
				"needs_restart":  true,
				"restart_reason": restartReasons,
			})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":        "saved",
				"message":       i18n.T(s.Cfg.Server.UILanguage, "backend.setup_complete_message"),
				"needs_restart": false,
			})
		}
	}
}

func applySetupProfileConfigPatch(patch map[string]interface{}, s *Server) {
	rawProfileID, ok := patch["_setup_profile_id"]
	if ok {
		delete(patch, "_setup_profile_id")
	}
	profileID, _ := rawProfileID.(string)
	profileID = strings.TrimSpace(profileID)
	if profileID == "" || profileID == "custom" {
		return
	}

	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}

	for _, profile := range setup.LoadProfiles("", logger) {
		if profile.ID != profileID || len(profile.ConfigPatch) == 0 {
			continue
		}
		merged := make(map[string]interface{}, len(profile.ConfigPatch)+len(patch))
		deepMerge(merged, profile.ConfigPatch, "")
		deepMerge(merged, patch, "")
		for key := range patch {
			delete(patch, key)
		}
		for key, value := range merged {
			patch[key] = value
		}
		logger.Info("[Setup] Applied setup profile config_patch", "profile", profileID)
		return
	}
}

// validateSetupAdminPassword inspects the auth block of a setup patch and
// returns the password (if any), whether auth should remain enabled, and an
// error if validation fails. Does not mutate the patch.
//
// The caller is responsible for stripping the temporary admin_password field
// from the patch (see stripSetupAdminPassword) before persisting it.
func validateSetupAdminPassword(authPatch map[string]interface{}, currentAuthEnabled bool, currentPasswordSet bool) (string, bool, error) {
	authEnabled := currentAuthEnabled
	if authPatch == nil {
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

// stripSetupAdminPassword removes the temporary admin_password field from the
// auth block of a setup patch so it is never written to disk. Call this AFTER
// validateSetupAdminPassword has extracted the password.
func stripSetupAdminPassword(authPatch map[string]interface{}) {
	if authPatch != nil {
		delete(authPatch, "admin_password")
	}
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
// We check that at least one provider with a usable API key exists. OAuth2
// providers only count after their access token has been applied to LLM.APIKey.
//
// DEBUG OVERRIDE: while the debugForceSetup flag is true the setup wizard is
// shown on every request, regardless of the current configuration. This is a
// temporary switch used to iterate on the setup UI without first clearing the
// provider/auth state. Remember to revert before shipping.
func needsSetup(cfg *config.Config) bool {
	if debugForceSetup {
		return true
	}
	return needsSetupReal(cfg)
}

func needsSetupReal(cfg *config.Config) bool {
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
		for _, p := range cfg.Providers {
			if p.APIKey != "" {
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
	// Setup is complete when at least one LLM provider is reachable AND auth is
	// either disabled by the operator or has a password configured.
	//
	// Deliberately NOT requiring setup when Auth.Enabled is false and no password
	// is set: operators who intentionally disable auth (e.g., single-user LAN
	// deployments, OAuth2 setups that don't need a UI password) are considered
	// configured. The config UI can be used to re-enable auth and set a password
	// at any time. See TestNeedsSetupAcceptsOAuthProviderWithAppliedToken and
	// TestHandleSetupStatusNoCSRFWhenConfigured for the codified behavior.
	return cfg.Auth.Enabled && cfg.Auth.PasswordHash == ""
}

func setupOllamaBaseURL(isDocker bool) string {
	if isDocker {
		return "http://host.docker.internal:11434/v1"
	}
	return "http://localhost:11434/v1"
}

// handleSetupTestConnection performs a lightweight LLM connectivity test using
// the provider details supplied by the setup wizard. It creates a temporary
// client, sends a minimal completion request, and returns success or an error
// message so the user can verify their API key/URL before saving.
func handleSetupTestConnection(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		show := needsSetup(s.Cfg)
		s.CfgMu.RUnlock()
		if !show {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_already_completed"), http.StatusForbidden)
			return
		}
		// The test endpoint performs an outbound request using user-supplied
		// connection details, so it must be protected even though setup itself is
		// available before login. Do not consume the token: users often test and
		// then save from the same setup page.
		if !validateSetupCSRFToken(s, r.Header.Get("X-CSRF-Token"), false) {
			s.Logger.Warn("[Setup] CSRF token mismatch on test connection")
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_invalid_csrf_token"), http.StatusForbidden)
			return
		}

		var req struct {
			ProviderType string `json:"provider_type"`
			BaseURL      string `json:"base_url"`
			APIKey       string `json:"api_key"`
			AccountID    string `json:"account_id"`
			Model        string `json:"model"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_invalid_request"), http.StatusBadRequest)
			return
		}

		if req.Model == "" {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.setup_model_required"), http.StatusBadRequest)
			return
		}
		if err := validateSetupTestBaseURL(req.ProviderType, req.BaseURL); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		client := llm.NewClientFromProviderDetails(req.ProviderType, req.BaseURL, req.APIKey, req.AccountID)

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
			"message": i18n.T(s.Cfg.Server.UILanguage, "backend.setup_connection_successful"),
		})
	}
}

func validateSetupTestBaseURL(providerType, rawURL string) error {
	providerType = strings.ToLower(strings.TrimSpace(providerType))
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("base_url is required")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("base_url must be a valid absolute URL")
	}
	if parsed.User != nil {
		return fmt.Errorf("base_url must not include credentials")
	}
	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	port := parsed.Port()

	if providerType == "ollama" {
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("ollama base_url must use http or https")
		}
		if isAllowedOllamaSetupHost(host) {
			return nil
		}
		return fmt.Errorf("ollama setup test only allows localhost or host.docker.internal")
	}

	if scheme != "https" {
		return fmt.Errorf("setup test only allows HTTPS provider URLs")
	}
	if port != "" && port != "443" {
		return fmt.Errorf("setup test only allows the default HTTPS port")
	}
	if isLocalOrPrivateSetupHost(host) {
		return fmt.Errorf("setup test does not allow local or private base_url hosts")
	}
	if providerType == "custom" {
		if err := validateSetupProviderSSRF(rawURL); err != nil {
			return fmt.Errorf("setup test does not allow local or private base_url hosts: %w", err)
		}
		return nil
	}
	if !isAllowedSetupProviderHost(host) {
		return fmt.Errorf("setup test only allows known setup provider hosts")
	}
	return nil
}

func isAllowedOllamaSetupHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	default:
		return false
	}
}

func isLocalOrPrivateSetupHost(host string) bool {
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return true
		}
		return !addr.IsGlobalUnicast() ||
			addr.IsPrivate() ||
			addr.IsLoopback() ||
			addr.IsLinkLocalUnicast() ||
			addr.IsLinkLocalMulticast()
	}
	return false
}

func isAllowedSetupProviderHost(host string) bool {
	allowed := []string{
		// Major SaaS providers
		"api.openai.com",
		"api.anthropic.com",
		"generativelanguage.googleapis.com",
		"openrouter.ai",
		"api.minimax.io",
		"api.minimaxi.com",
		"dashscope-intl.aliyuncs.com",
		"open.bigmodel.cn",
		"api.stepfun.ai",
		"api.moonshot.cn",
		// Common self-hosted providers (typically behind reverse proxy).
		// Note: "localhost", "127.0.0.1", "::1" are only reachable via the
		// Ollama code path because validateSetupTestBaseURL rejects
		// non-Ollama requests with isLocalOrPrivateSetupHost before
		// consulting this list.
		"localhost",
		"127.0.0.1",
		"::1",
		// Public inference endpoints frequently used by self-hosters
		"api.deepinfra.com",
		"api.together.xyz",
		"api.fireworks.ai",
		"inference.friendli.ai",
		"api.mistral.ai",
		"api.cohere.ai",
		"api.groq.com",
		"api.perplexity.ai",
		"api.deepseek.com",
		"api.x.ai",
	}
	for _, allowedHost := range allowed {
		if host == allowedHost {
			return true
		}
	}
	return false
}

// handleSetupProfiles returns the list of pre-configured provider profiles
// for the setup wizard plan selection step.
func handleSetupProfiles(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, i18n.T(s.Cfg.Server.UILanguage, "backend.http_method_not_allowed"), http.StatusMethodNotAllowed)
			return
		}

		profiles := loadCachedSetupProfiles(s.Logger)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"profiles": profiles,
		})
	}
}
