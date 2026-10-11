package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/retrogames"
	"aurago/internal/retrogames/retrogamestest"
)

func TestRetroGamesSettingsFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join("data", "dir")
	cfg.RetroGames.Enabled = true
	cfg.RetroGames.AgentAccess = false
	got := retroGamesSettingsFromConfig(cfg)
	want := retrogames.Settings{Enabled: true, AgentAccess: false, DataDir: filepath.Join("data", "dir", "retrogames")}
	if got != want {
		t.Fatalf("settings = %+v, want %+v", got, want)
	}
	if got := retroGamesSettingsFromConfig(nil); got.Enabled {
		t.Fatalf("nil config must disable: %+v", got)
	}
}

func TestSyncRetroGamesSettingsFollowsConfig(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Directories.DataDir = t.TempDir()
	s.Cfg.RetroGames.Enabled = true
	s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	s.syncRetroGamesSettings(s.Cfg)
	if !s.RetroGames.Enabled() {
		t.Fatal("service should be enabled")
	}
	off := *s.Cfg
	off.RetroGames.Enabled = false
	s.syncRetroGamesSettings(&off)
	if s.RetroGames.Enabled() {
		t.Fatal("service should follow the switch off")
	}
}

func TestRetroGamesCapability(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	caps := desktopCapabilities{s: s}
	if caps.HasCapability("retro_games") {
		t.Fatal("no service: capability must be false")
	}
	s.RetroGames = retrogamestest.NewService(t, &retrogamestest.Fetcher{})
	s.Cfg.RetroGames.Enabled = true
	if !caps.HasCapability("retro_games") {
		t.Fatal("enabled: capability must be true")
	}
	s.Cfg.RetroGames.Enabled = false
	if caps.HasCapability("retro_games") {
		t.Fatal("disabled: capability must be false")
	}
}

func TestShutdownRetroGamesIsNilSafe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	(&Server{}).shutdownRetroGames(ctx)
}
