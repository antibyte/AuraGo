package server

import (
	"log/slog"
	"testing"

	"aurago/internal/config"
)

func TestNewServerFromOptionsWiresCoreDependencies(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Directories.DataDir = t.TempDir()
	shutdownCh := make(chan struct{})
	logger := slog.Default()

	srv := newServerFromOptions(StartOptions{
		Cfg:          cfg,
		Logger:       logger,
		AccessLogger: logger,
		IsFirstStart: true,
		ShutdownCh:   shutdownCh,
	})

	if srv.Cfg != cfg {
		t.Fatalf("Cfg not wired")
	}
	if srv.Logger != logger || srv.AccessLogger != logger {
		t.Fatalf("logger wiring mismatch")
	}
	if srv.ShutdownCh != shutdownCh {
		t.Fatalf("shutdown channel not wired")
	}
	if !srv.IsFirstStart {
		t.Fatalf("expected IsFirstStart to be true")
	}
	if srv.MissionManagerV2 == nil || srv.Guardian == nil || srv.EggHub == nil {
		t.Fatalf("expected core services to be initialized")
	}
	if srv.StartedAt.IsZero() {
		t.Fatalf("expected StartedAt to be initialized")
	}
}
