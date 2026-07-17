package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/realtimespeech"
	"aurago/internal/security"

	"github.com/sashabaranov/go-openai"
)

const (
	realtimeSpeechRequestBodyLimit = 256 << 10
	realtimeSpeechContextMessages  = 20
	realtimeSpeechContextChars     = 20000
	realtimeSpeechTurnChars        = 20000
)

var realtimeSpeechSessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type realtimeSpeechProfileJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Voice       string `json:"voice"`
	Enabled     bool   `json:"enabled"`
	APIKey      string `json:"api_key,omitempty"`
	APIKeySet   bool   `json:"api_key_set"`
	ClearAPIKey bool   `json:"clear_api_key,omitempty"`
}

type realtimeSpeechConfigJSON struct {
	Enabled          bool                        `json:"enabled"`
	DefaultProfile   string                      `json:"default_profile"`
	ParkAfterSeconds int                         `json:"park_after_seconds"`
	Profiles         []realtimeSpeechProfileJSON `json:"profiles"`
}

type realtimeSpeechSessionRequest struct {
	SessionID        string                 `json:"session_id"`
	ClientID         string                 `json:"client_id"`
	ProfileID        string                 `json:"profile_id"`
	Surface          string                 `json:"surface"`
	ChatSessionID    string                 `json:"chat_session_id"`
	OfferSDP         string                 `json:"offer_sdp"`
	Takeover         bool                   `json:"takeover"`
	State            string                 `json:"state"`
	ConversationID   string                 `json:"conversation_id"`
	ResumptionHandle string                 `json:"resumption_handle"`
	WakeLatencyMS    int64                  `json:"wake_latency_ms"`
	Usage            map[string]interface{} `json:"usage"`
	ErrorMessage     string                 `json:"error_message"`
}

type realtimeSpeechContextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// registerRealtimeSpeechHandlers installs the complete realtime speech API as
// one service so all route closures share microphone leases and idempotency.
func registerRealtimeSpeechHandlers(mux *http.ServeMux, s *Server, sse *SSEBroadcaster) {
	registry := realtimespeech.NewRegistry(nil)
	client := realtimespeech.NewClient()
	webAction := handleChatCompletions(s, sse)
	desktopAction := handleDesktopChatStream(s)

	mux.HandleFunc("/api/realtime-speech/config", handleRealtimeSpeechConfig(s))
	mux.HandleFunc("/api/realtime-speech/catalog", handleRealtimeSpeechCatalog(s, client))
	mux.HandleFunc("/api/realtime-speech/test", handleRealtimeSpeechTest(s, client))
	mux.HandleFunc("/api/realtime-speech/status", handleRealtimeSpeechStatus(s, registry))
	mux.HandleFunc("/api/realtime-speech/sessions", handleRealtimeSpeechSessions(s, registry, client))
	mux.HandleFunc("/api/realtime-speech/sessions/", handleRealtimeSpeechSessionByID(s, registry))
	mux.HandleFunc("/api/realtime-speech/actions", handleRealtimeSpeechActions(s, registry, webAction, desktopAction))
	mux.HandleFunc("/api/realtime-speech/actions/", handleRealtimeSpeechActionByID(registry))
	mux.HandleFunc("/api/realtime-speech/turns", handleRealtimeSpeechTurns(s, registry))
}

func handleRealtimeSpeechConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeRealtimeSpeechConfig(w, realtimeSpeechConfigSnapshot(s))
		case http.MethodPut:
			handlePutRealtimeSpeechConfig(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func realtimeSpeechConfigSnapshot(s *Server) config.RealtimeSpeechConfig {
	if s == nil || s.Cfg == nil {
		return config.RealtimeSpeechConfig{ParkAfterSeconds: config.DefaultRealtimeSpeechParkAfterSeconds}
	}
	s.CfgMu.RLock()
	snapshot := s.Cfg.RealtimeSpeech
	snapshot.Profiles = append([]config.RealtimeSpeechProfile(nil), s.Cfg.RealtimeSpeech.Profiles...)
	s.CfgMu.RUnlock()
	config.NormalizeRealtimeSpeechConfig(&snapshot)
	return snapshot
}

func writeRealtimeSpeechConfig(w http.ResponseWriter, cfg config.RealtimeSpeechConfig) {
	output := realtimeSpeechConfigJSON{
		Enabled:          cfg.Enabled,
		DefaultProfile:   cfg.DefaultProfile,
		ParkAfterSeconds: cfg.ParkAfterSeconds,
		Profiles:         make([]realtimeSpeechProfileJSON, len(cfg.Profiles)),
	}
	for i, profile := range cfg.Profiles {
		output.Profiles[i] = realtimeSpeechProfileJSON{
			ID:        profile.ID,
			Name:      profile.Name,
			Provider:  profile.Provider,
			Model:     profile.Model,
			Voice:     profile.Voice,
			Enabled:   profile.Enabled,
			APIKeySet: strings.TrimSpace(profile.APIKey) != "",
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(output)
}

func handlePutRealtimeSpeechConfig(s *Server, w http.ResponseWriter, r *http.Request) {
	if !sameOriginOrNoOrigin(r) {
		jsonError(w, "Request origin does not match server host", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, realtimeSpeechRequestBodyLimit)
	var incoming realtimeSpeechConfigJSON
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	oldConfig := realtimeSpeechConfigSnapshot(s)
	existing := make(map[string]config.RealtimeSpeechProfile, len(oldConfig.Profiles))
	for _, profile := range oldConfig.Profiles {
		existing[profile.ID] = profile
	}
	next := config.RealtimeSpeechConfig{
		Enabled:          incoming.Enabled,
		DefaultProfile:   incoming.DefaultProfile,
		ParkAfterSeconds: incoming.ParkAfterSeconds,
		Profiles:         make([]config.RealtimeSpeechProfile, len(incoming.Profiles)),
	}
	mutations := make([]vaultMutation, 0, len(incoming.Profiles)+len(oldConfig.Profiles))
	nextIDs := make(map[string]struct{}, len(incoming.Profiles))
	for i, profile := range incoming.Profiles {
		profile.ID = strings.TrimSpace(profile.ID)
		if _, duplicate := nextIDs[profile.ID]; duplicate {
			jsonError(w, "Duplicate realtime speech profile ID: "+profile.ID, http.StatusBadRequest)
			return
		}
		nextIDs[profile.ID] = struct{}{}
		if profile.ClearAPIKey && strings.TrimSpace(profile.APIKey) != "" && profile.APIKey != maskedKey {
			jsonError(w, "api_key and clear_api_key cannot be used together", http.StatusBadRequest)
			return
		}
		entry := config.RealtimeSpeechProfile{
			ID:       profile.ID,
			Name:     profile.Name,
			Provider: profile.Provider,
			Model:    profile.Model,
			Voice:    profile.Voice,
			Enabled:  profile.Enabled,
		}
		old := existing[profile.ID]
		entry.APIKey = old.APIKey
		key := config.RealtimeSpeechProfileAPIKeyVaultKey(profile.ID)
		switch {
		case profile.ClearAPIKey:
			if key != "" {
				mutations = append(mutations, vaultMutation{key: key, delete: true})
			}
			entry.APIKey = ""
		case strings.TrimSpace(profile.APIKey) != "" && profile.APIKey != maskedKey:
			if s == nil || s.Vault == nil {
				jsonError(w, "Vault is required to store realtime speech API keys", http.StatusServiceUnavailable)
				return
			}
			if key == "" {
				jsonError(w, "Invalid realtime speech profile ID", http.StatusBadRequest)
				return
			}
			entry.APIKey = strings.TrimSpace(profile.APIKey)
			security.RegisterSensitive(entry.APIKey)
			mutations = append(mutations, vaultMutation{key: key, value: entry.APIKey})
		}
		next.Profiles[i] = entry
	}
	for _, old := range oldConfig.Profiles {
		if _, keep := nextIDs[old.ID]; keep {
			continue
		}
		if key := config.RealtimeSpeechProfileAPIKeyVaultKey(old.ID); key != "" {
			mutations = append(mutations, vaultMutation{key: key, delete: true})
		}
	}
	if len(mutations) > 0 && (s == nil || s.Vault == nil) {
		jsonError(w, "Vault is required to update realtime speech API keys", http.StatusServiceUnavailable)
		return
	}

	normalized, err := realtimespeech.NormalizeAndValidateConfig(next, existing)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := persistRealtimeSpeechConfig(s, normalized, mutations); err != nil {
		if s != nil && s.Logger != nil {
			s.Logger.Error("[RealtimeSpeech] Failed to update configuration", "error", err)
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeRealtimeSpeechConfig(w, realtimeSpeechConfigSnapshot(s))
}

func persistRealtimeSpeechConfig(s *Server, next config.RealtimeSpeechConfig, mutations []vaultMutation) error {
	if s == nil || s.Cfg == nil {
		return fmt.Errorf("server config is not available")
	}
	s.CfgMu.RLock()
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("config path not set")
	}

	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	original, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	output, err := marshalConfigWithRealtimeSpeech(original, next)
	if err != nil {
		return err
	}
	if err := config.WriteFileAtomic(configPath, output, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	vaultSnapshots, err := applyVaultMutations(s.Vault, mutations)
	if err != nil {
		_ = config.WriteFileAtomic(configPath, original, 0o600)
		return fmt.Errorf("update realtime speech secrets: %w", err)
	}

	newConfig, err := config.Load(configPath)
	if err != nil {
		_ = config.WriteFileAtomic(configPath, original, 0o600)
		_ = restoreVaultSecrets(s.Vault, vaultSnapshots)
		return fmt.Errorf("reload realtime speech config: %w", err)
	}
	newConfig.ConfigPath = configPath
	newConfig.ApplyVaultSecrets(s.Vault)
	newConfig.ResolveProviders()
	newConfig.ApplyOAuthTokens(s.Vault)
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(newConfig)
	s.CfgMu.Unlock()
	if s.Logger != nil {
		s.Logger.Info("[RealtimeSpeech] Configuration updated", "profiles", len(next.Profiles), "enabled", next.Enabled)
	}
	return nil
}

func handleRealtimeSpeechCatalog(s *Server, client *realtimespeech.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		providers := realtimespeech.Catalog()
		response := map[string]interface{}{
			"version":   realtimespeech.CatalogVersion,
			"providers": providers,
		}
		profileID := strings.TrimSpace(r.URL.Query().Get("profile_id"))
		if profileID != "" {
			profile, ok := realtimeSpeechProfileByID(s, profileID)
			if ok && profile.Provider == realtimespeech.ProviderXAI && strings.TrimSpace(profile.APIKey) != "" {
				security.RegisterSensitive(profile.APIKey)
				ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
				voices, err := client.FetchXAIVoices(ctx, profile.APIKey)
				cancel()
				if err != nil {
					response["voice_catalog_error"] = security.Scrub(err.Error())
				} else {
					for i := range providers {
						if providers[i].ID == realtimespeech.ProviderXAI {
							providers[i].Voices = voices
						}
					}
					response["providers"] = providers
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

func handleRealtimeSpeechTest(s *Server, client *realtimespeech.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, realtimeSpeechRequestBodyLimit)
		var body realtimeSpeechProfileJSON
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		profile, exists := realtimeSpeechProfileByID(s, body.ID)
		if !exists {
			profile = config.RealtimeSpeechProfile{ID: "test-profile", Name: "Connection test", Enabled: true}
		}
		if strings.TrimSpace(body.Provider) != "" {
			profile.Provider = body.Provider
		}
		if strings.TrimSpace(body.Model) != "" {
			profile.Model = body.Model
		}
		if strings.TrimSpace(body.Voice) != "" {
			profile.Voice = body.Voice
		}
		if strings.TrimSpace(body.APIKey) != "" && body.APIKey != maskedKey {
			profile.APIKey = strings.TrimSpace(body.APIKey)
		}
		profile.Name = "Connection test"
		profile.Enabled = true
		if strings.TrimSpace(profile.APIKey) == "" {
			jsonError(w, "API key is not configured", http.StatusBadRequest)
			return
		}
		security.RegisterSensitive(profile.APIKey)
		validated, err := realtimespeech.NormalizeAndValidateConfig(config.RealtimeSpeechConfig{
			ParkAfterSeconds: config.DefaultRealtimeSpeechParkAfterSeconds,
			DefaultProfile:   profile.ID,
			Profiles:         []config.RealtimeSpeechProfile{profile},
		}, map[string]config.RealtimeSpeechProfile{profile.ID: profile})
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		profile = validated.Profiles[0]
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		voices, err := client.TestProfile(ctx, profile)
		cancel()
		if err != nil {
			jsonError(w, security.Scrub(err.Error()), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"provider": profile.Provider,
			"model":    profile.Model,
			"voice":    profile.Voice,
			"voices":   voices,
		})
	}
}

func handleRealtimeSpeechStatus(s *Server, registry *realtimespeech.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := realtimeSpeechConfigSnapshot(s)
		status := registry.Status()
		status["enabled"] = cfg.Enabled
		status["profile_count"] = len(cfg.Profiles)
		status["default_profile"] = cfg.DefaultProfile
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}
}

func handleRealtimeSpeechSessions(s *Server, registry *realtimespeech.Registry, client *realtimespeech.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, realtimeSpeechRequestBodyLimit)
		var body realtimeSpeechSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		clientID, err := realtimeSpeechClientID(r, body.ClientID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if id := strings.TrimSpace(body.SessionID); id != "" && !realtimeSpeechSessionIDPattern.MatchString(id) {
			jsonError(w, "Invalid realtime speech session_id", http.StatusBadRequest)
			return
		}
		cfg := realtimeSpeechConfigSnapshot(s)
		if !cfg.Enabled {
			jsonError(w, "Realtime speech is disabled", http.StatusConflict)
			return
		}
		profileID := strings.TrimSpace(body.ProfileID)
		if profileID == "" {
			profileID = cfg.DefaultProfile
		}
		profile, ok := profileFromConfig(cfg, profileID)
		if !ok || !profile.Enabled {
			jsonError(w, "Realtime speech profile is not available", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(profile.APIKey) == "" {
			jsonError(w, "Realtime speech profile has no API key", http.StatusBadRequest)
			return
		}
		security.RegisterSensitive(profile.APIKey)
		surface, chatSessionID, err := normalizeRealtimeSpeechSurface(body.Surface, body.ChatSessionID, r.Header.Get("X-Session-ID"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		session, conflictID, err := registry.Acquire(clientID, realtimespeech.Session{
			ID:               strings.TrimSpace(body.SessionID),
			ProfileID:        profile.ID,
			Provider:         profile.Provider,
			ChatSessionID:    chatSessionID,
			Surface:          surface,
			State:            realtimeSpeechFirstNonEmpty(strings.TrimSpace(body.State), "connecting"),
			ConversationID:   strings.TrimSpace(body.ConversationID),
			ResumptionHandle: strings.TrimSpace(body.ResumptionHandle),
		}, body.Takeover)
		if err != nil {
			status := http.StatusConflict
			if strings.Contains(err.Error(), "too many") {
				status = http.StatusTooManyRequests
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":              err.Error(),
				"active_session_id":  conflictID,
				"takeover_available": conflictID != "",
			})
			return
		}

		history := realtimeSpeechVisibleContext(s, chatSessionID)
		response := map[string]interface{}{
			"session_id":         session.ID,
			"profile_id":         profile.ID,
			"provider":           profile.Provider,
			"model":              profile.Model,
			"voice":              profile.Voice,
			"park_after_seconds": cfg.ParkAfterSeconds,
			"context":            history,
			"catalog_version":    realtimespeech.CatalogVersion,
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		switch profile.Provider {
		case realtimespeech.ProviderOpenAI:
			answerSDP, exchangeErr := client.ExchangeOpenAISDP(ctx, profile, body.OfferSDP)
			if exchangeErr != nil {
				registry.Release(session.ID, clientID)
				registry.RecordError()
				jsonError(w, security.Scrub(exchangeErr.Error()), http.StatusBadGateway)
				return
			}
			response["transport"] = "webrtc"
			response["answer_sdp"] = answerSDP
			response["park_strategy"] = "warm_audio_gate"
		case realtimespeech.ProviderXAI:
			secret, secretErr := client.CreateXAIClientSecret(ctx, profile.APIKey)
			if secretErr != nil {
				registry.Release(session.ID, clientID)
				registry.RecordError()
				jsonError(w, security.Scrub(secretErr.Error()), http.StatusBadGateway)
				return
			}
			security.RegisterSensitive(secret.Value)
			websocketURL := "wss://api.x.ai/v1/realtime?model=" + urlQueryEscape(profile.Model)
			if session.ConversationID != "" {
				websocketURL += "&conversation_id=" + urlQueryEscape(session.ConversationID)
			}
			response["transport"] = "websocket"
			response["websocket_url"] = websocketURL
			response["websocket_protocol"] = "xai-client-secret." + secret.Value
			response["expires_at"] = secret.ExpiresAt
			response["session_config"] = realtimespeech.XAISessionConfig(profile)
			response["park_strategy"] = "conversation_resume"
		case realtimespeech.ProviderGemini:
			token, tokenErr := client.CreateGeminiEphemeralToken(ctx, profile)
			if tokenErr != nil {
				registry.Release(session.ID, clientID)
				registry.RecordError()
				jsonError(w, security.Scrub(tokenErr.Error()), http.StatusBadGateway)
				return
			}
			security.RegisterSensitive(token.Value)
			setup := realtimespeech.GeminiSessionSetup(profile)
			if session.ResumptionHandle != "" {
				setup["sessionResumption"] = map[string]interface{}{"handle": session.ResumptionHandle}
			}
			response["transport"] = "websocket"
			response["websocket_url"] = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1alpha.GenerativeService.BidiGenerateContentConstrained"
			response["access_token"] = token.Value
			response["expires_at"] = token.ExpiresAt
			response["new_session_expires_at"] = token.NewSessionExpiresAt
			response["setup"] = setup
			response["park_strategy"] = "resumption_handle"
		default:
			registry.Release(session.ID, clientID)
			jsonError(w, "Unsupported realtime speech provider", http.StatusBadRequest)
			return
		}
		_, _ = registry.UpdateState(session.ID, clientID, "listening", session.ConversationID, session.ResumptionHandle)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

func handleRealtimeSpeechSessionByID(s *Server, registry *realtimespeech.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete && r.Method != http.MethodPatch {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/realtime-speech/sessions/"))
		clientID, err := realtimeSpeechClientID(r, r.URL.Query().Get("client_id"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodPatch {
			r.Body = http.MaxBytesReader(w, r.Body, realtimeSpeechRequestBodyLimit)
			var body realtimeSpeechSessionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			state := strings.ToLower(strings.TrimSpace(body.State))
			if !slices.Contains([]string{"connecting", "listening", "speaking", "executing", "parked", "reconnecting", "error"}, state) {
				jsonError(w, "Invalid realtime speech session state", http.StatusBadRequest)
				return
			}
			updated, err := registry.UpdateState(
				id,
				clientID,
				state,
				strings.TrimSpace(body.ConversationID),
				strings.TrimSpace(body.ResumptionHandle),
			)
			if err != nil {
				jsonError(w, err.Error(), http.StatusNotFound)
				return
			}
			if body.WakeLatencyMS > 0 {
				registry.RecordWakeLatency(body.WakeLatencyMS)
			}
			if len(body.Usage) > 0 {
				registry.RecordUsage(updated.Provider, body.Usage)
			}
			if state == "error" {
				registry.RecordError()
				message := strings.TrimSpace(security.Scrub(body.ErrorMessage))
				if message == "" {
					message = "unspecified provider connection failure"
				}
				if runes := []rune(message); len(runes) > 500 {
					message = string(runes[:500])
				}
				if s != nil && s.Logger != nil {
					s.Logger.Error(
						"[RealtimeSpeech] Browser provider session failed",
						"session_id", id,
						"profile_id", updated.ProfileID,
						"provider", updated.Provider,
						"surface", updated.Surface,
						"error", message,
					)
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if id == "" || !registry.Release(id, clientID) {
			jsonError(w, "Realtime speech session not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleRealtimeSpeechActions(s *Server, registry *realtimespeech.Registry, webAction, desktopAction http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, realtimeSpeechRequestBodyLimit)
		var body struct {
			SessionID string `json:"session_id"`
			ClientID  string `json:"client_id"`
			RequestID string `json:"request_id"`
			Request   string `json:"request"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		clientID, err := realtimeSpeechClientID(r, body.ClientID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		session, ok := registry.Get(strings.TrimSpace(body.SessionID), clientID)
		if !ok {
			jsonError(w, "Realtime speech session not found", http.StatusNotFound)
			return
		}
		requestText := strings.TrimSpace(body.Request)
		if requestText == "" || utf8.RuneCountInString(requestText) > realtimeSpeechTurnChars {
			jsonError(w, "Request must contain 1-20000 characters", http.StatusBadRequest)
			return
		}
		requestID := strings.TrimSpace(body.RequestID)
		if !realtimeSpeechSessionIDPattern.MatchString(requestID) {
			jsonError(w, "Invalid realtime speech request_id", http.StatusBadRequest)
			return
		}
		if err := registry.BeginAction(requestID, session.ID, clientID, session.ChatSessionID); err != nil {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		defer registry.EndAction(requestID)
		w.Header().Set("X-Realtime-Speech-Request-ID", requestID)
		w.Header().Set("Cache-Control", "no-store")

		suppressedContext := agent.WithVoiceOutputSuppressed(r.Context())
		if session.Surface == "desktop" {
			payload, _ := json.Marshal(map[string]interface{}{
				"message": requestText,
				"context": map[string]interface{}{
					"source":     "realtime-speech",
					"origin_app": "live-speech",
				},
			})
			inner := r.Clone(suppressedContext)
			inner.Method = http.MethodPost
			inner.URL.Path = "/api/desktop/chat/stream"
			inner.Body = ioNopCloser(bytes.NewReader(payload))
			inner.ContentLength = int64(len(payload))
			inner.Header = r.Header.Clone()
			inner.Header.Set("Content-Type", "application/json")
			desktopAction.ServeHTTP(w, inner)
			return
		}

		payload, _ := json.Marshal(openai.ChatCompletionRequest{
			Messages: []openai.ChatCompletionMessage{{
				Role:    openai.ChatMessageRoleUser,
				Content: requestText,
			}},
			Stream: true,
		})
		inner := r.Clone(suppressedContext)
		inner.Method = http.MethodPost
		inner.URL.Path = "/v1/chat/completions"
		inner.Body = ioNopCloser(bytes.NewReader(payload))
		inner.ContentLength = int64(len(payload))
		inner.Header = r.Header.Clone()
		inner.Header.Set("Content-Type", "application/json")
		inner.Header.Set("X-Session-ID", session.ChatSessionID)
		webAction.ServeHTTP(w, inner)
	}
}

func handleRealtimeSpeechActionByID(registry *realtimespeech.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		requestID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/realtime-speech/actions/"))
		clientID, err := realtimeSpeechClientID(r, r.URL.Query().Get("client_id"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		chatSessionID, ok := registry.ActionSession(requestID, clientID)
		if !ok {
			jsonError(w, "Realtime speech action not found", http.StatusNotFound)
			return
		}
		agent.InterruptSession(chatSessionID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "cancelled", "request_id": requestID})
	}
}

func handleRealtimeSpeechTurns(s *Server, registry *realtimespeech.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, realtimeSpeechRequestBodyLimit)
		var body struct {
			SessionID           string `json:"session_id"`
			ClientID            string `json:"client_id"`
			TurnID              string `json:"turn_id"`
			UserTranscript      string `json:"user_transcript"`
			AssistantTranscript string `json:"assistant_transcript"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		clientID, err := realtimeSpeechClientID(r, body.ClientID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		session, ok := registry.Get(strings.TrimSpace(body.SessionID), clientID)
		if !ok {
			jsonError(w, "Realtime speech session not found", http.StatusNotFound)
			return
		}
		userText := sanitizeRealtimeSpeechTranscript(body.UserTranscript)
		assistantText := sanitizeRealtimeSpeechTranscript(body.AssistantTranscript)
		if userText == "" || assistantText == "" || utf8.RuneCountInString(userText) > realtimeSpeechTurnChars || utf8.RuneCountInString(assistantText) > realtimeSpeechTurnChars {
			jsonError(w, "Final user and assistant transcripts are required and limited to 20000 characters", http.StatusBadRequest)
			return
		}
		unlock := lockSessionRequest(session.ChatSessionID)
		defer unlock()
		if !registry.MarkTurn(session.ID, clientID, strings.TrimSpace(body.TurnID)) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "duplicate": true})
			return
		}
		if s.Guardian != nil {
			s.Guardian.ScanUserInput(userText)
		}
		if s.ShortTermMem == nil {
			registry.ForgetTurn(session.ID, strings.TrimSpace(body.TurnID))
			jsonError(w, "Chat memory is unavailable", http.StatusServiceUnavailable)
			return
		}
		userID, err := s.ShortTermMem.InsertMessage(session.ChatSessionID, openai.ChatMessageRoleUser, userText, false, false)
		if err != nil {
			registry.ForgetTurn(session.ID, strings.TrimSpace(body.TurnID))
			jsonError(w, "Failed to persist realtime speech turn", http.StatusInternalServerError)
			return
		}
		assistantID, err := s.ShortTermMem.InsertMessage(session.ChatSessionID, openai.ChatMessageRoleAssistant, assistantText, false, false)
		if err != nil {
			_ = s.ShortTermMem.DeleteMessagesByID(session.ChatSessionID, []int64{userID})
			registry.ForgetTurn(session.ID, strings.TrimSpace(body.TurnID))
			jsonError(w, "Failed to persist realtime speech turn", http.StatusInternalServerError)
			return
		}
		if session.ChatSessionID == "default" && s.HistoryManager != nil {
			_ = s.HistoryManager.Add(openai.ChatMessageRoleUser, userText, userID, false, false)
			_ = s.HistoryManager.Add(openai.ChatMessageRoleAssistant, assistantText, assistantID, false, false)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "ok",
			"duplicate":    false,
			"user_id":      userID,
			"assistant_id": assistantID,
		})
	}
}

func realtimeSpeechProfileByID(s *Server, id string) (config.RealtimeSpeechProfile, bool) {
	return profileFromConfig(realtimeSpeechConfigSnapshot(s), strings.TrimSpace(id))
}

func profileFromConfig(cfg config.RealtimeSpeechConfig, id string) (config.RealtimeSpeechProfile, bool) {
	index := slices.IndexFunc(cfg.Profiles, func(profile config.RealtimeSpeechProfile) bool {
		return profile.ID == id
	})
	if index < 0 {
		return config.RealtimeSpeechProfile{}, false
	}
	return cfg.Profiles[index], true
}

func realtimeSpeechClientID(r *http.Request, bodyID string) (string, error) {
	headerID := strings.TrimSpace(r.Header.Get("X-Realtime-Speech-Client-ID"))
	bodyID = strings.TrimSpace(bodyID)
	if headerID != "" && bodyID != "" && headerID != bodyID {
		return "", fmt.Errorf("client_id does not match request header")
	}
	id := realtimeSpeechFirstNonEmpty(bodyID, headerID)
	if id == "" || len(id) > 128 || strings.ContainsAny(id, "\x00\r\n") {
		return "", fmt.Errorf("valid client_id is required")
	}
	return id, nil
}

func normalizeRealtimeSpeechSurface(surface, requestedSessionID, headerSessionID string) (string, string, error) {
	surface = strings.ToLower(strings.TrimSpace(surface))
	if surface == "" {
		surface = "webchat"
	}
	if surface == "desktop" || surface == "virtual-desktop" {
		return "desktop", desktopChatSessionID, nil
	}
	if surface != "webchat" {
		return "", "", fmt.Errorf("surface must be webchat or desktop")
	}
	sessionID := realtimeSpeechFirstNonEmpty(strings.TrimSpace(requestedSessionID), strings.TrimSpace(headerSessionID), "default")
	if !realtimeSpeechSessionIDPattern.MatchString(sessionID) {
		return "", "", fmt.Errorf("invalid chat_session_id")
	}
	return "webchat", sessionID, nil
}

func realtimeSpeechVisibleContext(s *Server, sessionID string) []realtimeSpeechContextMessage {
	if s == nil || s.ShortTermMem == nil {
		return []realtimeSpeechContextMessage{}
	}
	messages, err := s.ShortTermMem.GetSessionMessages(sessionID)
	if err != nil {
		return []realtimeSpeechContextMessage{}
	}
	collected := make([]realtimeSpeechContextMessage, 0, realtimeSpeechContextMessages)
	remaining := realtimeSpeechContextChars
	for i := len(messages) - 1; i >= 0 && len(collected) < realtimeSpeechContextMessages && remaining > 0; i-- {
		message := messages[i]
		if message.Role != openai.ChatMessageRoleUser && message.Role != openai.ChatMessageRoleAssistant {
			continue
		}
		content := sanitizeRealtimeSpeechTranscript(message.Content)
		if content == "" {
			continue
		}
		contentRunes := []rune(content)
		if len(contentRunes) > remaining {
			contentRunes = contentRunes[len(contentRunes)-remaining:]
			content = string(contentRunes)
		}
		collected = append(collected, realtimeSpeechContextMessage{Role: message.Role, Content: content})
		remaining -= len(contentRunes)
	}
	slices.Reverse(collected)
	return collected
}

func sanitizeRealtimeSpeechTranscript(value string) string {
	value = security.Scrub(value)
	value = security.StripThinkingTags(value)
	value = strings.ReplaceAll(value, "\x00", "")
	return strings.TrimSpace(value)
}

func realtimeSpeechFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"&", "%26",
		"=", "%3D",
		"?", "%3F",
		"#", "%23",
	)
	return replacer.Replace(value)
}

type readCloser struct {
	*bytes.Reader
}

func (readCloser) Close() error { return nil }

func ioNopCloser(reader *bytes.Reader) readCloser {
	return readCloser{Reader: reader}
}
