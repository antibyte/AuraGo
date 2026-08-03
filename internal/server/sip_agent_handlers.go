package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/realtimespeech"
	"aurago/internal/tools"
	"aurago/internal/voice"

	"github.com/sashabaranov/go-openai"
)

type sipAgentPayload struct {
	InboundRoute      string                `json:"inbound_route"`
	AutoAnswerDelayMS int                   `json:"auto_answer_delay_ms"`
	Voice             config.SIPVoiceConfig `json:"voice"`
}

type sipAgentProviderOption struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Model  string `json:"model"`
	Ready  bool   `json:"ready"`
	Reason string `json:"reason,omitempty"`
}

type sipAgentNamedOption struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model,omitempty"`
	Ready bool   `json:"ready"`
}

type sipAgentToolOption struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
}

func handleSIPAgent(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeSIPAgent(w, s, sipConfigSnapshot(s), nil)
		case http.MethodPut:
			if !sameOriginOrNoOrigin(r) {
				jsonError(w, "Request origin does not match server host", http.StatusForbidden)
				return
			}
			if s != nil {
				s.SIPConfigMu.Lock()
				defer s.SIPConfigMu.Unlock()
			}
			var incoming sipAgentPayload
			if err := decodeSIPAgentJSON(w, r, &incoming); err != nil {
				jsonError(w, "Invalid telephone agent configuration JSON", http.StatusBadRequest)
				return
			}
			old := sipConfigSnapshot(s)
			next := old
			next.Inbound.Route = incoming.InboundRoute
			next.Inbound.AutoAnswerDelayMS = incoming.AutoAnswerDelayMS
			next.Voice = incoming.Voice
			serverCfg := s.ConfigSnapshot()
			next.Voice = effectiveSIPVoiceConfig(serverCfg, next.Voice)
			config.NormalizeSIPConfig(&next)
			if err := config.ValidateSIPRuntimeConfig(next); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			if next.Voice.Behavior.UnavailableRequestBehavior == "explain_and_end" && !next.Permissions.AgentHangup {
				writeSIPJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "agent_hangup_required", "message": "Explain and end requires agent_hangup permission"})
				return
			}
			if err := validateSIPAgentReferencesWithServer(r.Context(), s, serverCfg, next.Voice, false); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := validateSIPAgentToolScope(s, serverCfg, next.Voice.AllowedTools); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := persistSIPConfig(s, next, nil); err != nil {
				jsonError(w, "Failed to save telephone agent configuration", http.StatusInternalServerError)
				return
			}
			if s != nil && s.SIPPhone != nil {
				s.SIPPhone.UpdateAgentConfig(next)
			}
			writeSIPAgent(w, s, sipConfigSnapshot(s), map[string]bool{
				"agent_provider_id": false, "asr_provider_id": false, "asr_mode": false, "tts_provider": false,
			})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleSIPAgentCatalog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "Runtime configuration is unavailable", http.StatusServiceUnavailable)
			return
		}
		providers := make([]sipAgentProviderOption, 0, len(cfg.Providers))
		for _, provider := range cfg.Providers {
			resolution := resolveTelephoneProvider(r.Context(), s, cfg, provider.ID, false)
			providers = append(providers, sipAgentProviderOption{
				ID: provider.ID, Name: provider.Name, Type: provider.Type, Model: provider.Model,
				Ready: resolution.Ready, Reason: resolution.Reason,
			})
		}
		sort.Slice(providers, func(i, j int) bool {
			return strings.ToLower(providers[i].Name+providers[i].ID) < strings.ToLower(providers[j].Name+providers[j].ID)
		})
		realtime := make([]sipAgentNamedOption, 0)
		for _, profile := range cfg.RealtimeSpeech.Profiles {
			if profile.Provider != realtimespeech.ProviderGemini {
				continue
			}
			realtime = append(realtime, sipAgentNamedOption{
				ID: profile.ID, Name: profile.Name, Model: profile.Model,
				Ready: profile.Enabled && strings.TrimSpace(profile.APIKey) != "",
			})
		}
		ttsProviders := []string{"google", "elevenlabs", "minimax", "mistral", "piper", "supertonic"}
		asrModes := []string{"whisper", "multimodal", "local"}
		if cfg.SpeechLab.Active() && cfg.SpeechLab.SIPEnabled {
			ttsProviders = append(ttsProviders, "speech_lab")
			asrModes = append(asrModes, "speech_lab")
		}
		tts := make([]sipAgentNamedOption, 0, len(ttsProviders))
		for _, provider := range ttsProviders {
			testCfg := *cfg
			testCfg.TTS.Provider = provider
			tts = append(tts, sipAgentNamedOption{ID: provider, Name: provider, Ready: chatVoiceOutputTTSConfigured(&testCfg)})
		}
		writeSIPJSON(w, map[string]any{
			"providers":         providers,
			"realtime_profiles": realtime,
			"tts_providers":     tts,
			"asr_modes":         asrModes,
			"tools":             sipAgentToolCatalog(s, cfg),
		})
	}
}

func handleSIPAgentTest(s *Server, limiter *sipRequestLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		var request struct {
			Live        bool `json:"live"`
			ConfirmLive bool `json:"confirm_live"`
		}
		if err := decodeSIPAgentJSON(w, r, &request); err != nil {
			jsonError(w, "Invalid telephone agent test JSON", http.StatusBadRequest)
			return
		}
		cfg := s.ConfigSnapshot()
		if cfg == nil {
			jsonError(w, "Runtime configuration is unavailable", http.StatusServiceUnavailable)
			return
		}
		voiceCfg := effectiveSIPVoiceConfig(cfg, cfg.SIP.Voice)
		blockers := sipAgentPipelineBlockers(s, voiceCfg)
		if len(blockers) != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusConflict)
			writeSIPJSON(w, map[string]any{"status": "blocked", "blockers": blockers})
			return
		}
		if !request.Live {
			writeSIPJSON(w, map[string]any{"status": "ok", "mode": "preflight"})
			return
		}
		if !request.ConfirmLive {
			jsonError(w, "Live telephone agent test requires explicit confirmation", http.StatusBadRequest)
			return
		}
		if limiter != nil && !limiter.allow(r, 3, time.Hour) {
			w.Header().Set("Retry-After", "3600")
			jsonError(w, "Telephone agent live-test rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		err := runSIPAgentLiveTest(ctx, s, cfg, voiceCfg)
		cancel()
		if err != nil {
			jsonError(w, "Telephone agent live test failed", http.StatusBadGateway)
			return
		}
		writeSIPJSON(w, map[string]any{"status": "ok", "mode": "live"})
	}
}

func decodeSIPAgentJSON(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, sipRequestBodyLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request must contain one JSON object")
	}
	return nil
}

func writeSIPAgent(w http.ResponseWriter, s *Server, sipCfg config.SIPConfig, inherited map[string]bool) {
	var cfg *config.Config
	if s != nil {
		cfg = s.ConfigSnapshot()
	}
	if inherited == nil {
		inherited = map[string]bool{
			"agent_provider_id": strings.TrimSpace(sipCfg.Voice.AgentProviderID) == "",
			"asr_provider_id":   strings.TrimSpace(sipCfg.Voice.Classic.ASRProviderID) == "",
			"asr_mode":          strings.TrimSpace(sipCfg.Voice.Classic.ASRMode) == "",
			"tts_provider":      strings.TrimSpace(sipCfg.Voice.Classic.TTSProvider) == "",
		}
	}
	voiceCfg := effectiveSIPVoiceConfig(cfg, sipCfg.Voice)
	writeSIPJSON(w, map[string]any{
		"config":      sipAgentPayload{InboundRoute: sipCfg.Inbound.Route, AutoAnswerDelayMS: sipCfg.Inbound.AutoAnswerDelayMS, Voice: voiceCfg},
		"sip_enabled": sipCfg.Enabled,
		"blockers":    sipAgentBlockers(s, sipCfg, voiceCfg),
		"inherited":   inherited,
	})
}

func sipAgentBlockers(s *Server, sipCfg config.SIPConfig, voiceCfg config.SIPVoiceConfig) []string {
	blockers := make([]string, 0, 8)
	if !sipCfg.Enabled {
		blockers = append(blockers, "sip_disabled")
	}
	if sipCfg.ReadOnly {
		blockers = append(blockers, "sip_readonly")
	}
	if strings.EqualFold(strings.TrimSpace(sipCfg.Inbound.Route), "agent") && !sipCfg.Permissions.AnswerInbound {
		blockers = append(blockers, "inbound_permission_disabled")
	}
	if !sipCfg.Permissions.OriginateOutbound {
		blockers = append(blockers, "outbound_permission_disabled")
	}
	if s != nil && s.SIPPhone != nil && sipCfg.Enabled && !s.SIPPhone.Status().Registered {
		blockers = append(blockers, "not_registered")
	}
	if s != nil && s.SIPPhone != nil && s.SIPPhone.NetworkConfigurationRequired() {
		blockers = append(blockers, "docker_advertised_host_required")
	}
	return append(blockers, sipAgentPipelineBlockers(s, voiceCfg)...)
}

func sipAgentPipelineBlockers(s *Server, voiceCfg config.SIPVoiceConfig) []string {
	blockers := make([]string, 0, 2)
	var cfg *config.Config
	if s != nil {
		cfg = s.ConfigSnapshot()
	}
	if cfg != nil && voiceCfg.Behavior.UnavailableRequestBehavior == "explain_and_end" && !cfg.SIP.Permissions.AgentHangup {
		blockers = append(blockers, "agent_hangup_required")
	}
	if err := validateSIPAgentReferencesWithServer(context.Background(), s, cfg, voiceCfg, false); err != nil {
		message := strings.ToLower(err.Error())
		switch {
		case strings.Contains(message, "asr"):
			blockers = append(blockers, "asr_unavailable")
		case strings.Contains(message, "tts"):
			blockers = append(blockers, "tts_unavailable")
		case strings.Contains(message, "gemini"):
			blockers = append(blockers, "realtime_unavailable")
		default:
			blockers = append(blockers, "agent_llm_unavailable")
		}
	} else if err := validateSIPAgentToolScope(s, cfg, voiceCfg.AllowedTools); err != nil {
		blockers = append(blockers, "tool_scope_unavailable")
	}
	return blockers
}

func validateSIPAgentToolScope(s *Server, cfg *config.Config, allowed []string) error {
	if s == nil || cfg == nil {
		return fmt.Errorf("telephone tool catalog is unavailable")
	}
	schemas := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, tools.NewManifest(cfg.Directories.ToolsDir), mcpFeatureFlags(s), s.Logger)
	return validateSIPAgentToolScopeWithSchemas(schemas, allowed)
}

func validateSIPAgentToolScopeWithSchemas(schemas []openai.Tool, allowed []string) error {
	available := make(map[string]struct{})
	for _, option := range sipAgentToolOptions(schemas) {
		available[option.ID] = struct{}{}
	}
	for _, name := range allowed {
		if _, ok := available[name]; !ok {
			return fmt.Errorf("unknown or unavailable telephone tool %q", name)
		}
	}
	return nil
}

func sipAgentToolCatalog(s *Server, cfg *config.Config) []sipAgentToolOption {
	if s == nil || cfg == nil {
		return nil
	}
	schemas := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, tools.NewManifest(cfg.Directories.ToolsDir), mcpFeatureFlags(s), s.Logger)
	return sipAgentToolOptions(schemas)
}

func sipAgentToolOptions(schemas []openai.Tool) []sipAgentToolOption {
	options := make([]sipAgentToolOption, 0, len(schemas))
	for _, schema := range schemas {
		if schema.Function == nil || strings.TrimSpace(schema.Function.Name) == "" {
			continue
		}
		if !telephoneAgentToolAllowed(schema.Function.Name) {
			continue
		}
		options = append(options, sipAgentToolOption{ID: schema.Function.Name, Description: schema.Function.Description})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	return options
}

func telephoneAgentToolAllowed(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "discover_tools", "invoke_tool", "execute_skill", "list_agent_skills",
		"activate_agent_skill", "run_agent_skill_script", "run_tool":
		return false
	default:
		return true
	}
}

func runSIPAgentLiveTest(ctx context.Context, s *Server, cfg *config.Config, voiceCfg config.SIPVoiceConfig) error {
	if err := validateSIPAgentReferencesWithServer(ctx, s, cfg, voiceCfg, true); err != nil {
		return err
	}
	switch voiceCfg.Backend {
	case "classic":
		provider, err := requireTelephoneProvider(ctx, s, cfg, voiceCfg.AgentProviderID)
		if err != nil {
			return err
		}
		client := llm.NewClientFromProviderWithConfig(cfg, provider.Type, provider.BaseURL, provider.APIKey, provider.AccountID)
		if s == nil || s.VoiceActionRunner == nil {
			return fmt.Errorf("telephone agent runtime is unavailable")
		}
		backend, err := s.VoiceActionRunner.backendFactory(ctx, voiceCfg)
		if err != nil {
			return err
		}
		classic, ok := backend.(*voice.ClassicBackend)
		if !ok {
			return fmt.Errorf("classic telephone backend is unavailable")
		}
		return runClassicSIPAgentLiveTest(ctx, client, classic, provider.Model, voiceCfg.Language)
	case "gemini_live":
		profile, _ := profileFromConfig(cfg.RealtimeSpeech, voiceCfg.RealtimeProfileID)
		if s == nil || s.VoiceActionRunner == nil {
			return fmt.Errorf("telephone agent runtime is unavailable")
		}
		backend := &voice.GeminiLiveBackend{Profile: profile, Runner: s.VoiceActionRunner, SystemInstruction: telephoneAgentPrompt(voiceCfg), TestNoTools: true}
		bridge := voice.NewBridge(2)
		defer bridge.Close()
		session, err := backend.Start(ctx, voice.CallContext{CallID: "sip-live-test", Direction: "test", AllowedTools: []string{}}, bridge)
		if err != nil {
			return err
		}
		return session.Close()
	default:
		return fmt.Errorf("unsupported telephone backend")
	}
}

func runClassicSIPAgentLiveTest(ctx context.Context, client llm.ChatClient, classic *voice.ClassicBackend, model, language string) error {
	if client == nil || classic == nil || classic.Recognizer == nil || classic.Synthesizer == nil {
		return fmt.Errorf("classic telephone live-test dependencies are unavailable")
	}
	response, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "This is an explicitly confirmed telephone pipeline health check. Reply with only OK."},
			{Role: openai.ChatMessageRoleUser, Content: "health check"},
		},
	})
	if err != nil || len(response.Choices) == 0 {
		return fmt.Errorf("telephone agent LLM probe failed")
	}
	pcm, sampleRate, err := classic.Synthesizer.Synthesize(ctx, response.Choices[0].Message.Content, language)
	if err != nil {
		return err
	}
	wav, err := voice.EncodeWAVPCM16(pcm, sampleRate)
	if err != nil {
		return fmt.Errorf("encode telephone live-test audio: %w", err)
	}
	transcript, err := classic.Recognizer.Recognize(ctx, wav, sampleRate, language)
	if err != nil {
		return fmt.Errorf("telephone ASR probe failed: %w", err)
	}
	if strings.TrimSpace(transcript) == "" {
		return fmt.Errorf("telephone ASR probe returned an empty transcript")
	}
	return nil
}
