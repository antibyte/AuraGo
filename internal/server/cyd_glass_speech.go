package server

import (
	"os"
	"path/filepath"

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
	return tools.BuildTTSConfig(cfg, tools.TTSRouteGlass, "")
}

func glassTTSReady(cfg *config.Config) bool {
	return tools.TTSReady(cfg, tools.TTSRouteGlass)
}
