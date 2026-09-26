package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
)

type gameMakerPromptUsageMetrics struct {
	Calls              int            `json:"calls"`
	FailedCalls        int            `json:"failed_calls"`
	InputTokens        int            `json:"reported_input_tokens"`
	OutputTokens       int            `json:"reported_output_tokens"`
	CacheReadTokens    int            `json:"reported_cache_read_tokens"`
	CacheWriteTokens   int            `json:"reported_cache_write_tokens"`
	UnknownInputCalls  int            `json:"unknown_input_calls"`
	UnknownOutputCalls int            `json:"unknown_output_calls"`
	UnknownReadCalls   int            `json:"unknown_cache_read_calls"`
	UnknownWriteCalls  int            `json:"unknown_cache_write_calls"`
	PricedCalls        int            `json:"priced_calls"`
	EstimatedCostUSD   *float64       `json:"estimated_total_cost_usd"`
	LocalBuildHits     int            `json:"local_prompt_build_hits"`
	ProviderCacheHits  int            `json:"provider_cache_hits"`
	ContextChanges     map[string]int `json:"context_changes"`
}

// The separate event stream includes failed attempts and source/image calls.
// Never add its token totals to legacy token_usage (which may be estimated).
func summarizeGameMakerPromptUsage(events []gamemaker.Event, complete bool) *gameMakerPromptUsageMetrics {
	if !complete {
		return nil
	}
	result := &gameMakerPromptUsageMetrics{ContextChanges: map[string]int{}}
	cost := 0.0
	for _, event := range events {
		if event.Type != "prompt_usage" {
			continue
		}
		result.Calls++
		if failed, _ := event.Payload["failed"].(bool); failed {
			result.FailedCalls++
		}
		for _, field := range []struct {
			name           string
			total, unknown *int
		}{
			{"input_tokens", &result.InputTokens, &result.UnknownInputCalls},
			{"output_tokens", &result.OutputTokens, &result.UnknownOutputCalls},
			{"cache_read_tokens", &result.CacheReadTokens, &result.UnknownReadCalls},
			{"cache_write_tokens", &result.CacheWriteTokens, &result.UnknownWriteCalls},
		} {
			n, ok := gameMakerEvalTokenInt(event.Payload[field.name])
			if ok {
				*field.total += n
			} else {
				*field.unknown++
			}
		}
		if n, ok := gameMakerEvalTokenInt(event.Payload["cache_read_tokens"]); ok && n > 0 {
			result.ProviderCacheHits++
		}
		if hit, _ := event.Payload["local_prompt_cache_hit"].(bool); hit {
			result.LocalBuildHits++
		}
		if reason, _ := event.Payload["context_change"].(string); reason != "" {
			result.ContextChanges[reason]++
		}
		if n, ok := event.Payload["estimated_cost_usd"].(float64); ok && n >= 0 && !math.IsNaN(n) && !math.IsInf(n, 0) {
			cost += n
			result.PricedCalls++
		}
	}
	if result.Calls == 0 {
		return nil
	}
	if result.Calls == result.PricedCalls {
		result.EstimatedCostUSD = &cost
	}
	return result
}

func TestGameMakerPromptProfiles(t *testing.T) {
	for _, dimension := range []string{"2d", "3d"} {
		planning, err := gameMakerPromptProfile("planning", dimension)
		if err != nil {
			t.Fatal(err)
		}
		building, err := gameMakerPromptProfile("building", dimension)
		if err != nil {
			t.Fatal(err)
		}
		repair, err := gameMakerPromptProfile("repair", dimension)
		if err != nil {
			t.Fatal(err)
		}
		if len(planning.Tools()) != 3 || len(building.Tools()) != 4 {
			t.Fatal("profile tool scope changed")
		}
		if building.Revision() != repair.Revision() || building.SystemPrompt() != repair.SystemPrompt() || !reflect.DeepEqual(building.Tools(), repair.Tools()) {
			t.Fatal("repair changes creation prefix")
		}
		if planning.Revision() == building.Revision() {
			t.Fatal("phase revision missing")
		}
		for _, run := range []gamemaker.JobRun{
			{Stage: "building", Job: gamemaker.Job{ID: "job-one"}, Project: gamemaker.Project{Dimension: dimension}},
			{Stage: "repair", Job: gamemaker.Job{ID: "job-two"}, Project: gamemaker.Project{Dimension: dimension}, Diagnostics: []gamemaker.Diagnostic{{Message: "different failure"}}},
		} {
			profile, _ := gameMakerPromptProfile(run.Stage, run.Project.Dimension)
			if profile.Revision() != building.Revision() {
				t.Fatal("job data changed instructions")
			}
		}
	}
}

func TestGameMakerPromptUsageMetricsUnknownAndSeparate(t *testing.T) {
	events := []gamemaker.Event{
		{Type: "token_usage", Payload: map[string]any{"prompt_tokens": 900}},
		{Type: "prompt_usage", Payload: map[string]any{"input_tokens": 100, "output_tokens": 10, "cache_read_tokens": 60, "cache_write_tokens": 0, "estimated_cost_usd": 0.01, "context_change": "initial"}},
		{Type: "prompt_usage", Payload: map[string]any{"input_tokens": 100, "output_tokens": 10, "local_prompt_cache_hit": true, "failed": true}},
	}
	got := summarizeGameMakerPromptUsage(events, true)
	if got.Calls != 2 || got.InputTokens != 200 || got.CacheReadTokens != 60 || got.UnknownReadCalls != 1 || got.LocalBuildHits != 1 || got.ProviderCacheHits != 1 || got.PricedCalls != 1 || got.EstimatedCostUSD != nil {
		data, _ := json.Marshal(got)
		t.Fatal(string(data))
	}
	if summarizeGameMakerPromptUsage(events, false) != nil {
		t.Fatal("partial report claimed totals")
	}
}

func TestGameMakerPromptUsagePersistsThroughMinimalLoop(t *testing.T) {
	root := t.TempDir()
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true, AllowCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	project, err := service.CreateProject(ctx, gamemaker.CreateProjectRequest{Name: "Usage fixture", Dimension: "2d", Description: "Local usage observation fixture"})
	if err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"model":"step-5-preview","choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":7,"cached_tokens":60}}`)
	}))
	defer provider.Close()
	runner := &gameMakerAgentRunner{service: service}
	observer := runner.gameUsageObserver(gamemaker.JobRun{Project: project, Job: gamemaker.Job{ID: "fixture-job"}})
	profile, _ := agent.NewPreparedPromptProfile("fixture/v1", "Required rules.", nil)
	cfg := &config.Config{}
	cfg.Agent.ContextWindow = 32768
	client := llm.NewClientFromProvider("stepfun", provider.URL+"/v1", "fixture")
	opts := &agent.MinimalLoopOptions{MaxToolRounds: 0, PreparedPrompt: profile, UsageObserver: observer}
	_, history, err := agent.ExecuteMinimalLoop(ctx, client, "step-5-preview", "", "private-source-marker", nil, &agent.DispatchContext{Cfg: cfg}, nil, slog.Default(), opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.PreparedPromptReused = true
	_, _, err = agent.ExecuteMinimalLoop(ctx, client, "step-5-preview", "", "correction", nil, &agent.DispatchContext{Cfg: cfg}, history, slog.Default(), opts)
	if err != nil {
		t.Fatal(err)
	}
	events, err := service.EventsAfter(ctx, project.ID, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	metrics := summarizeGameMakerPromptUsage(events, true)
	if metrics == nil || metrics.Calls != 2 || metrics.ProviderCacheHits != 2 || metrics.LocalBuildHits != 1 || metrics.CacheReadTokens != 120 || metrics.EstimatedCostUSD != nil || metrics.UnknownWriteCalls != 2 || metrics.ContextChanges["initial"] != 1 || len(metrics.ContextChanges) != 1 {
		t.Fatalf("missing or inaccurate persisted usage: %+v", metrics)
	}
	diagnostics := 0
	for _, event := range events {
		data, _ := json.Marshal(event.Payload)
		if strings.Contains(string(data), "private-source-marker") || strings.Contains(string(data), "Required rules.") {
			t.Fatal("request content leaked into usage events")
		}
		if event.Type == "diagnostic" && strings.Contains(fmt.Sprint(event.Payload["message"]), "cache_read=60 cache_write=unknown") {
			diagnostics++
		}
	}
	if diagnostics != 2 {
		t.Fatal("usage summary missing from Studio diagnostics")
	}
}
