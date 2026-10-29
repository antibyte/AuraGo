package tools

import (
	"strings"

	"aurago/internal/config"
	"aurago/internal/realtimespeech"
	"aurago/internal/speechlab"
)

const (
	// TTSRouteChat is automatic web-chat speech. Active Speech Lab chat output
	// replaces the configured provider for this route only.
	TTSRouteChat = "chat"
	// TTSRouteTool is the tts tool plus Bluetooth and Chromecast playback.
	TTSRouteTool = "tool"
	// TTSRouteGlass is a CYD alert. It never replaces a missing Phonon voice.
	TTSRouteGlass = "glass"
)

// BuildTTSConfig copies the effective speech settings for one route.
func BuildTTSConfig(cfg *config.Config, route, language string, clients ...*speechlab.Client) TTSConfig {
	if cfg == nil {
		return TTSConfig{}
	}
	provider := strings.TrimSpace(cfg.TTS.Provider)
	switch route {
	case TTSRouteChat:
		if speechLabProvider(provider) && !(cfg.SpeechLab.Active() && cfg.SpeechLab.ChatOutputEnabled) {
			provider = ""
		}
		if provider == "" && cfg.TTS.Piper.Enabled {
			provider = "piper"
		}
		if cfg.SpeechLab.Active() && cfg.SpeechLab.ChatOutputEnabled {
			provider = "speech_lab"
		}
	default:
		if provider == "" && cfg.TTS.Piper.Enabled {
			provider = "piper"
		}
	}

	ttsCfg := TTSConfig{
		Provider:            provider,
		DataDir:             cfg.Directories.DataDir,
		CacheRetentionHours: cfg.TTS.CacheRetentionHours,
		CacheMaxFiles:       cfg.TTS.CacheMaxFiles,
	}
	if route == TTSRouteGlass {
		ttsCfg.NoProviderFallback = true
		ttsCfg.Language = cfg.TTS.Language
	} else {
		ttsCfg.Language = language
		if ttsCfg.Language == "" {
			ttsCfg.Language = cfg.TTS.Language
		}
		if route == TTSRouteChat && ttsCfg.Language == "" {
			ttsCfg.Language = cfg.SpeechLab.Language
		}
	}
	if strings.EqualFold(provider, "sanotts") || strings.EqualFold(provider, "phonon") || strings.EqualFold(ttsCfg.Language, "auto") {
		requested := language
		if route == TTSRouteGlass {
			requested = ""
		}
		ttsCfg.Language = cfg.SanoTTSLanguage(requested)
	}
	ttsCfg.Phonon.VoiceID = cfg.TTS.Phonon.VoiceID
	ttsCfg.Phonon.Persona = cfg.Personality.CorePersonality
	ttsCfg.ElevenLabs.APIKey = cfg.TTS.ElevenLabs.APIKey
	ttsCfg.ElevenLabs.VoiceID = cfg.TTS.ElevenLabs.VoiceID
	ttsCfg.ElevenLabs.ModelID = cfg.TTS.ElevenLabs.ModelID
	ttsCfg.MiniMax.APIKey = cfg.TTS.MiniMax.APIKey
	ttsCfg.MiniMax.VoiceID = cfg.TTS.MiniMax.VoiceID
	ttsCfg.MiniMax.ModelID = cfg.TTS.MiniMax.ModelID
	ttsCfg.MiniMax.Speed = cfg.TTS.MiniMax.Speed
	ttsCfg.Mistral.APIKey = cfg.TTS.Mistral.APIKey
	ttsCfg.Mistral.VoiceID = cfg.TTS.Mistral.VoiceID
	ttsCfg.Mistral.ModelID = cfg.TTS.Mistral.ModelID
	ttsCfg.XAI = cfg.TTS.XAI
	ttsCfg.OpenAI = cfg.TTS.OpenAI
	ttsCfg.StepFun = cfg.TTS.StepFun
	ttsCfg.Resemble = cfg.TTS.Resemble
	ttsCfg.Piper.Port = cfg.TTS.Piper.ContainerPort
	ttsCfg.Piper.Voice = cfg.TTS.Piper.Voice
	ttsCfg.Piper.SpeakerID = cfg.TTS.Piper.SpeakerID
	ttsCfg.Supertonic.URL = cfg.TTS.Supertonic.URL
	ttsCfg.Supertonic.Model = cfg.TTS.Supertonic.Model
	ttsCfg.Supertonic.Voice = cfg.TTS.Supertonic.Voice
	ttsCfg.Supertonic.Speed = cfg.TTS.Supertonic.Speed
	ttsCfg.Supertonic.Steps = cfg.TTS.Supertonic.Steps
	ttsCfg.Supertonic.ResponseFormat = cfg.TTS.Supertonic.ResponseFormat
	ttsCfg.SpeechLab.BaseURL = cfg.SpeechLab.BaseURL
	ttsCfg.SpeechLab.Language = cfg.SpeechLab.Language
	ttsCfg.SpeechLab.TimeoutSeconds = cfg.SpeechLab.TimeoutSeconds
	if len(clients) > 0 {
		ttsCfg.SpeechLab.Client = clients[0]
	}
	return ttsCfg
}

// TTSRouteStatus is one effective speech path. It carries no URL, key or voice.
type TTSRouteStatus struct {
	Code     string `json:"code"`
	Provider string `json:"provider"`
	Ready    bool   `json:"ready"`
}

// EffectiveTTSRoutes reports web chat, the shared tool path, glass alerts and
// the selected Live Speech profile.
func EffectiveTTSRoutes(cfg *config.Config) []TTSRouteStatus {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return []TTSRouteStatus{
		ttsRouteStatus("web_chat", TTSRouteChat, cfg),
		ttsRouteStatus("tool", TTSRouteTool, cfg),
		ttsRouteStatus("glass", TTSRouteGlass, cfg),
		liveSpeechRouteStatus(cfg),
	}
}

func ttsRouteStatus(code, route string, cfg *config.Config) TTSRouteStatus {
	return TTSRouteStatus{
		Code:     code,
		Provider: strings.TrimSpace(BuildTTSConfig(cfg, route, "").Provider),
		Ready:    TTSReady(cfg, route),
	}
}

func liveSpeechRouteStatus(cfg *config.Config) TTSRouteStatus {
	status := TTSRouteStatus{Code: "live_speech"}
	if cfg == nil || !cfg.RealtimeSpeech.Enabled {
		return status
	}
	selected := strings.TrimSpace(cfg.RealtimeSpeech.DefaultProfile)
	for _, profile := range cfg.RealtimeSpeech.Profiles {
		if profile.ID != selected || !profile.Enabled {
			continue
		}
		status.Provider = strings.TrimSpace(profile.Provider)
		status.Ready = liveSpeechProfileReady(status.Provider, profile.APIKey)
		return status
	}
	return status
}

func liveSpeechProfileReady(provider, apiKey string) bool {
	catalog, ok := realtimespeech.Provider(provider)
	if !ok {
		return false
	}
	if catalog.Capabilities.RequiresAPIKey && strings.TrimSpace(apiKey) == "" {
		return false
	}
	return true
}

// TTSReady reports whether the route has a usable speech backend.
func TTSReady(cfg *config.Config, route string) bool {
	if cfg == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(BuildTTSConfig(cfg, route, "").Provider)) {
	case "google", "sanotts", "phonon":
		return true
	case "elevenlabs":
		return strings.TrimSpace(cfg.TTS.ElevenLabs.APIKey) != ""
	case "minimax":
		return strings.TrimSpace(cfg.TTS.MiniMax.APIKey) != ""
	case "mistral":
		return strings.TrimSpace(cfg.TTS.Mistral.APIKey) != "" && strings.TrimSpace(cfg.TTS.Mistral.VoiceID) != ""
	case "xai":
		return strings.TrimSpace(cfg.TTS.XAI.APIKey) != ""
	case "openai":
		return strings.TrimSpace(cfg.TTS.OpenAI.APIKey) != ""
	case "stepfun":
		return strings.TrimSpace(cfg.TTS.StepFun.APIKey) != ""
	case "resemble":
		return strings.TrimSpace(cfg.TTS.Resemble.APIKey) != "" && strings.TrimSpace(cfg.TTS.Resemble.VoiceID) != ""
	case "piper":
		return cfg.TTS.Piper.Enabled
	case "supertonic":
		return strings.TrimSpace(cfg.TTS.Supertonic.URL) != ""
	case "s2s", "speech_lab", "s2s-lab":
		return cfg.SpeechLab.Active()
	default:
		return false
	}
}

func speechLabProvider(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "s2s", "speech_lab", "s2s-lab":
		return true
	default:
		return false
	}
}

// ensureSpeechLabSnapshot requires the current /ready voice before synthesis.
// A caller that already stored the TTS id and voice is left unchanged.
func ensureSpeechLabSnapshot(cfg *TTSConfig) error {
	if cfg == nil || !speechLabProvider(cfg.Provider) {
		return nil
	}
	// Voice and TTS id are the snapshot. The HTTP client can be created later.
	if strings.TrimSpace(cfg.SpeechLab.ExpectedTTSID) != "" && strings.TrimSpace(cfg.SpeechLab.Voice) != "" {
		return nil
	}
	client := cfg.SpeechLab.Client
	if client == nil {
		lab := config.SpeechLabConfig{
			Enabled:        true,
			BaseURL:        cfg.SpeechLab.BaseURL,
			Language:       speechLabFirstNonEmpty(cfg.Language, cfg.SpeechLab.Language),
			TimeoutSeconds: cfg.SpeechLab.TimeoutSeconds,
		}
		created, err := speechlab.NewClient(lab)
		if err != nil {
			return err
		}
		client = created
		cfg.SpeechLab.Client = client
	}
	ready, err := client.Require(ttsRequestContext(*cfg), false, true)
	if err != nil {
		return err
	}
	cfg.SpeechLab.ExpectedTTSID = ready.TTSID
	cfg.SpeechLab.Voice = ready.Voice
	return nil
}
