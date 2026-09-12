package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestAgentSkillStartupDoesNotWaitForGuardian(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer remote.Close()
	defer close(release)
	tmp := t.TempDir()
	cfg := &config.Config{}
	cfg.SQLite.SkillsPath = filepath.Join(tmp, "skills.db")
	cfg.Directories.AgentSkillsDir = filepath.Join(tmp, "agent_skills")
	cfg.Directories.WorkspaceDir = filepath.Join(tmp, "workspace")
	cfg.Tools.SkillManager.ScanWithGuardian = true
	cfg.LLMGuardian.Enabled = true
	cfg.LLMGuardian.BaseURL = remote.URL + "/v1"
	cfg.LLMGuardian.APIKey = "test-only"
	cfg.LLMGuardian.ResolvedModel = "fixture"
	cfg.LLMGuardian.TimeoutSecs = 60
	cfg.LLMGuardian.FailSafe = "allow" // Cancellation must not persist a fail-open verdict.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{Cfg: cfg, Logger: logger, LLMGuardian: security.NewLLMGuardian(cfg, logger)}
	oldSkill, oldAgent := tools.DefaultSkillManager(), tools.DefaultAgentSkillManager()
	defer tools.SetDefaultSkillManager(oldSkill)
	defer tools.SetDefaultAgentSkillManager(oldAgent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initialized := make(chan gamemaker.SkillInstallResult, 1)
	go func() { initialized <- s.initSkillManagers(ctx, tmp) }()
	var installed gamemaker.SkillInstallResult
	select {
	case installed = <-initialized:
	case <-time.After(3 * time.Second):
		cancel()
		<-initialized
		t.Fatal("skill initialization blocked on remote security scanning")
	}
	defer s.SkillsDB.Close()
	select {
	case <-started:
		t.Fatal("Guardian ran on the core startup path")
	default:
	}
	s.GameMaker = &gamemaker.Service{}
	done := make(chan struct{})
	go func() { defer close(done); s.syncAgentSkills(ctx, cfg, installed) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		cancel()
		<-done
		t.Fatal("background security scan did not start")
	}
	if _, ready := s.GameMaker.SkillStatus(); ready {
		t.Fatal("Game Maker became ready before security verification")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("security scan ignored shutdown cancellation")
	}
	var count int
	if err := s.SkillsDB.QueryRow("SELECT count(*) FROM agent_skills_registry").Scan(&count); err != nil || count != 0 {
		t.Fatalf("canceled scan persisted a package: count=%d err=%v", count, err)
	}
	// A subsequent successful pass publishes readiness through the existing service lock.
	cfg.Tools.SkillManager.ScanWithGuardian = false
	s.syncAgentSkills(context.Background(), cfg, installed)
	if _, ready := s.GameMaker.SkillStatus(); !ready {
		t.Fatal("successful verification did not publish Game Maker readiness")
	}
}
