package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/retrogames"
)

func retroGamesSettingsFromConfig(cfg *config.Config) retrogames.Settings {
	if cfg == nil {
		return retrogames.Settings{}
	}
	settings := retrogames.Settings{
		Enabled:     cfg.RetroGames.Enabled,
		AgentAccess: cfg.RetroGames.AgentAccess,
	}
	// Without a data directory the service refuses to open its index; joining
	// onto "" would put it in ./retrogames, relative to the working directory.
	if dataDir := strings.TrimSpace(cfg.Directories.DataDir); dataDir != "" {
		settings.DataDir = filepath.Join(dataDir, config.RetroGamesDirName)
	}
	return settings
}

func newRetroGamesService(cfg *config.Config, logger *slog.Logger) *retrogames.Service {
	svc := retrogames.NewService(retrogames.Deps{Logger: logger})
	svc.Configure(retroGamesSettingsFromConfig(cfg))
	return svc
}

// syncRetroGamesSettings configures the service from the current config
// snapshot. replaceConfigSnapshot calls it after every publication. The
// snapshot is read and applied under retroGamesSyncMu: a call that read an
// older snapshot finishes applying it before a later call reads the newer one,
// so publications that overlap can never leave the service on older settings.
func (s *Server) syncRetroGamesSettings() {
	if s == nil || s.RetroGames == nil {
		return
	}
	s.retroGamesSyncMu.Lock()
	defer s.retroGamesSyncMu.Unlock()
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	settings := retroGamesSettingsFromConfig(cfg)
	if s.retroGamesBeforeConfigure != nil {
		s.retroGamesBeforeConfigure()
	}
	s.RetroGames.Configure(settings)
}

func (s *Server) retroGamesAvailable() bool {
	if s == nil || s.RetroGames == nil {
		return false
	}
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.RetroGames.Enabled
}

// publishRetroGamesAvailability tells open desktops that the Retro Games app
// appeared or disappeared, so start menus refresh at once.
func (s *Server) publishRetroGamesAvailability(available bool) {
	if s == nil {
		return
	}
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	broadcastDesktopEvent(s, hub, desktop.Event{Type: "desktop_changed", Payload: map[string]interface{}{
		"operation": "app_availability", "app_id": "retro-games", "available": available,
	}, CreatedAt: time.Now().UTC()})
}

// announceRetroGamesAvailability tells open desktops that the Retro Games app
// appeared or disappeared. Config publications usually hold CfgMu, so the
// broadcast runs on its own goroutine and announces what the snapshot current
// by then grants.
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
		s.publishRetroGamesAvailability(s.retroGamesAvailable())
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
