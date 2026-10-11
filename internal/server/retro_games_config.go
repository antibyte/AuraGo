package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/retrogames"
)

func retroGamesSettingsFromConfig(cfg *config.Config) retrogames.Settings {
	if cfg == nil {
		return retrogames.Settings{}
	}
	return retrogames.Settings{
		Enabled:     cfg.RetroGames.Enabled,
		AgentAccess: cfg.RetroGames.AgentAccess,
		DataDir:     filepath.Join(cfg.Directories.DataDir, "retrogames"),
	}
}

func newRetroGamesService(cfg *config.Config, logger *slog.Logger) *retrogames.Service {
	svc := retrogames.NewService(retrogames.Deps{Logger: logger})
	svc.Configure(retroGamesSettingsFromConfig(cfg))
	return svc
}

// syncRetroGamesSettings applies a newly published config to the service.
func (s *Server) syncRetroGamesSettings(cfg *config.Config) {
	if s == nil || s.RetroGames == nil {
		return
	}
	s.RetroGames.Configure(retroGamesSettingsFromConfig(cfg))
}

func (s *Server) retroGamesAvailable() bool {
	if s == nil || s.RetroGames == nil {
		return false
	}
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.RetroGames.Enabled
}

// announceRetroGamesAvailability tells open desktops that the Retro Games app
// appeared or disappeared. Config publications usually hold CfgMu, so the
// broadcast runs on its own goroutine.
func (s *Server) announceRetroGamesAvailability() {
	if s == nil || s.RetroGames == nil {
		return
	}
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("[RetroGames] Recovered from a panic while announcing the Desktop app", "panic", r, "stack", string(debug.Stack()))
			}
		}()
		available := s.retroGamesAvailable()
		s.DesktopMu.Lock()
		hub := s.DesktopHub
		s.DesktopMu.Unlock()
		broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{
			"operation": "app_availability", "app_id": "retro-games", "available": available,
		}, CreatedAt: time.Now().UTC()})
	}()
}

func (s *Server) shutdownRetroGames(ctx context.Context) {
	if s == nil || s.RetroGames == nil {
		return
	}
	if err := s.RetroGames.Shutdown(ctx); err != nil && s.Logger != nil {
		s.Logger.Warn("[RetroGames] Shutdown did not complete", "error", err)
	}
}
