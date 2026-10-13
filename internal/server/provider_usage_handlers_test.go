package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/detective"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
	openai "github.com/sashabaranov/go-openai"
)

func decisionsProviderUsageFixture() []config.ProviderEntry {
	return []config.ProviderEntry{
		{ID: "legacy", Name: "Legacy", Type: "openrouter", Model: "chat-model"},
		{ID: "general", Name: "General", Type: "openai", Model: "chat-model", Usage: config.ProviderUsageGeneral},
		{ID: "decision", Name: "Decision", Type: "openrouter", Model: "", Usage: config.ProviderUsageDecisions},
	}
}

func TestGameMakerCapabilitiesExcludeDecisionsOnlyProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = decisionsProviderUsageFixture()
	s := &Server{Cfg: cfg}
	w := httptest.NewRecorder()
	handleGameMakerCapabilities(s).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/game-maker/capabilities", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d: %s", w.Code, w.Body.String())
	}
	var got gamemaker.Capabilities
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Providers) != 2 {
		t.Fatalf("providers = %+v, want legacy and general only", got.Providers)
	}
	for _, provider := range got.Providers {
		if provider.ID == "decision" || provider.Usage != config.ProviderUsageGeneral {
			t.Fatalf("ineligible provider or missing normalized usage in capability list: %+v", provider)
		}
	}
}

func TestDetectiveCapabilitiesExcludeDecisionsOnlyProviders(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.Providers = decisionsProviderUsageFixture()
	s := &Server{Cfg: cfg}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/detective/capabilities", nil)
	s.handleDetective(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Providers []map[string]string `json:"providers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Providers) != 2 {
		t.Fatalf("providers = %+v, want legacy and general only", got.Providers)
	}
	for _, provider := range got.Providers {
		if provider["id"] == "decision" || provider["usage"] != config.ProviderUsageGeneral {
			t.Fatalf("ineligible provider or missing normalized usage in capability list: %+v", provider)
		}
	}
}

func TestDetectiveRejectsDecisionsOnlyProviderOnCreate(t *testing.T) {
	cfg := &config.Config{}
	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.AllowAgentControl = true
	cfg.Detective.Enabled = true
	cfg.Providers = decisionsProviderUsageFixture()
	svc, err := detective.New(detective.Options{Path: filepath.Join(t.TempDir(), "cases.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s := &Server{Cfg: cfg, Detective: svc}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/desktop/detective/cases", strings.NewReader(`{"topic":"test","effort":"quick","provider_id":"decision"}`))
	s.handleDetective(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("decision-only provider create status = %d: %s", w.Code, w.Body.String())
	}
}

func TestGameMakerRunnerRejectsDecisionsOnlyProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = decisionsProviderUsageFixture()
	s := &Server{Cfg: cfg, LLMClient: llm.WrapOpenAIClient(openai.NewClient("test-key"))}
	runner := &gameMakerAgentRunner{server: s}
	err := runner.RunGameMakerJob(context.Background(), gamemaker.JobRun{Job: gamemaker.Job{ProviderID: "decision"}})
	if err == nil || err.Error() != "selected Game Maker provider is not available for general chat" {
		t.Fatalf("runner error = %v, want general-chat eligibility rejection", err)
	}
}

func TestLooperRejectsDecisionsOnlyProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = decisionsProviderUsageFixture()
	s := &Server{Cfg: cfg}
	if _, _, _, _, _, err := buildLooperRuntime(s, "decision", ""); err == nil {
		t.Fatal("Looper selected a decisions-only provider")
	}
}

func TestAgodeskConfiguredProviderExcludesDecisionsOnlyProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers = decisionsProviderUsageFixture()
	if _, ok := agodeskConfiguredProvider(cfg, "decision"); ok {
		t.Fatal("AgoDesk local agent resolved a decisions-only provider")
	}
	if _, ok := agodeskConfiguredProvider(cfg, "legacy"); !ok {
		t.Fatal("AgoDesk local agent rejected a legacy general provider")
	}
}
