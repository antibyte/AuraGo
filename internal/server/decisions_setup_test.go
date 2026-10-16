package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"gopkg.in/yaml.v3"
)

type setupDecisionsFake struct {
	*fakeDecisions
	entered  chan struct{}
	finished chan struct{}
}

func (f *setupDecisionsFake) Setup(ctx context.Context) error {
	close(f.entered)
	<-ctx.Done()
	close(f.finished)
	return ctx.Err()
}

func TestDecisionsSetupSavesOnlyOwnSettingsAndSurvivesRequest(t *testing.T) {
	s, fake := decisionsServerFixture(t)
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("agent:\n  name: kept\ndecisions:\n  enabled: false\n  provider: kept-provider\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s.Cfg.ConfigPath = file
	s.Cfg.Decisions = config.DecisionsConfig{Provider: "kept-provider"}
	service := &setupDecisionsFake{fakeDecisions: fake, entered: make(chan struct{}), finished: make(chan struct{})}
	s.Decisions = service
	r := httptest.NewRequest("POST", "/api/decisions/setup", strings.NewReader(`{"deployment":"auto","cpu_fallback_allowed":false}`))
	ctx, cancel := context.WithCancel(r.Context())
	w := httptest.NewRecorder()
	s.handleDecisionsSetup(w, r.WithContext(ctx))
	if w.Code != 202 {
		t.Fatalf("setup = %d: %s", w.Code, w.Body.String())
	}
	cancel()
	select {
	case <-service.entered:
	case <-time.After(time.Second):
		t.Fatal("setup not started")
	}
	select {
	case <-service.finished:
		t.Fatal("request cancellation stopped accepted setup")
	default:
	}
	var response struct {
		SetupID   string                 `json:"setup_id"`
		Decisions config.DecisionsConfig `json:"decisions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.SetupID == "" || !response.Decisions.Enabled || response.Decisions.Backend != "local" || response.Decisions.CPUFallbackAllowed {
		t.Fatalf("response = %+v", response)
	}
	raw, _ := os.ReadFile(file)
	var saved struct {
		Agent     struct{ Name string }
		Decisions config.DecisionsConfig
	}
	if err := yaml.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Agent.Name != "kept" || saved.Decisions.Provider != "kept-provider" {
		t.Fatalf("unrelated fields changed: %+v", saved)
	}
	duplicate := httptest.NewRecorder()
	s.handleDecisionsSetup(duplicate, httptest.NewRequest("POST", "/api/decisions/setup", strings.NewReader(`{"cpu_fallback_allowed":false}`)))
	if duplicate.Code != 409 {
		t.Fatalf("duplicate = %d", duplicate.Code)
	}
	s.cancelDecisionsSetup()
	select {
	case <-service.finished:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel setup")
	}
}
