package server

import (
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/config"
	"aurago/internal/cyd"
	"aurago/internal/tools"
)

// useActiveGlassSpeech routes CYD alerts through the configured TTS provider.
// sanoTTS stays unused unless it is the provider selected in config.
func (s *Server) useActiveGlassSpeech(h *cyd.Hub) {
	if s == nil || h == nil {
		return
	}
	h.SetSpeechSynth(s.synthesizeGlassSpeech, s.glassSpeechReady)
}

func (s *Server) glassSpeechReady() bool {
	if s == nil {
		return false
	}
	s.CfgMu.RLock()
	cfg := s.Cfg
	s.CfgMu.RUnlock()
	return glassTTSReady(cfg)
}

func (s *Server) synthesizeGlassSpeech(text string) ([]byte, error) {
	if s == nil {
		return nil, os.ErrInvalid
	}
	s.CfgMu.RLock()
	cfg := s.Cfg
	s.CfgMu.RUnlock()
	if !glassTTSReady(cfg) {
		return nil, os.ErrInvalid
	}
	name, err := tools.TTSSynthesize(glassTTSConfig(cfg), text)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(cfg.Directories.DataDir, "tts", filepath.Base(name)))
}

func glassTTSConfig(cfg *config.Config) tools.TTSConfig {
	provider := glassTTSProvider(cfg)
	ttsCfg := tools.TTSConfig{
		Provider:            provider,
		Language:            cfg.TTS.Language,
		DataDir:             cfg.Directories.DataDir,
		CacheRetentionHours: cfg.TTS.CacheRetentionHours,
		CacheMaxFiles:       cfg.TTS.CacheMaxFiles,
		NoProviderFallback:  true,
	}
	if strings.EqualFold(provider, "sanotts") || strings.EqualFold(provider, "phonon") || strings.EqualFold(ttsCfg.Language, "auto") {
		ttsCfg.Language = cfg.SanoTTSLanguage("")
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
	return ttsCfg
}

func glassTTSProvider(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	provider := strings.TrimSpace(cfg.TTS.Provider)
	if provider == "" && cfg.TTS.Piper.Enabled {
		return "piper"
	}
	return provider
}

func glassTTSReady(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	switch strings.ToLower(glassTTSProvider(cfg)) {
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
