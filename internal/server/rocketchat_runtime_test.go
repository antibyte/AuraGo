package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestRocketChatConfigPublicationRevokesActiveRequestAndShutdownPreventsRestart(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(cancelled) }))
	defer remote.Close()
	cfg := &config.Config{}
	cfg.RocketChat.Enabled = true
	cfg.RocketChat.URL = remote.URL
	cfg.RocketChat.AuthToken = "test-fixture-secret"
	cfg.RocketChat.UserID = "bot"
	cfg.RocketChat.Channel = "room"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), integrationCtx: ctx}
	s.initConfigSnapshot()
	s.configureRocketChatBot()
	defer s.stopRocketChatBot()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request not started")
	}
	changed := cfg.Clone()
	changed.RocketChat.Enabled = false
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(changed)
	s.CfgMu.Unlock()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("config publication did not revoke request")
	}
	s.stopRocketChatBot()
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	s.configureRocketChatBot()
	if s.rocketChatBot.Load() != nil {
		t.Fatal("shutdown allowed a new bot generation")
	}
}
