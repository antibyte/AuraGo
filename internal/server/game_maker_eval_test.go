package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"
	"gopkg.in/yaml.v3"
)

// Explicit opt-in: runs the real isolated agent against selected configured
// providers on their host. Credentials never enter reports or the browser.
// Serve the loopback parent through an SSH tunnel and keep it open in Chrome.
func TestGameMakerLiveEvaluation(t *testing.T) {
	configPath := os.Getenv("GAMEMAKER_EVAL_CONFIG")
	if configPath == "" {
		t.Skip("set GAMEMAKER_EVAL_CONFIG and GAMEMAKER_EVAL_ENV_FILE on the provider host")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var original config.Config
	if err = yaml.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	master := os.Getenv("AURAGO_MASTER_KEY")
	if master == "" {
		env, err := os.Open(os.Getenv("GAMEMAKER_EVAL_ENV_FILE"))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(env)
		for scanner.Scan() {
			key, value, ok := strings.Cut(scanner.Text(), "=")
			if ok && strings.TrimSpace(key) == "AURAGO_MASTER_KEY" {
				master = strings.Trim(strings.TrimSpace(value), "\"'")
			}
		}
		env.Close()
		if err = scanner.Err(); err != nil {
			t.Fatal(err)
		}
	}
	dataDir := original.Directories.DataDir
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(filepath.Dir(configPath), dataDir)
	}
	vault, err := security.NewVault(master, filepath.Join(dataDir, "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	master = ""
	root := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	jobTimeout := time.Duration(original.GameMaker.JobTimeoutSeconds) * time.Second
	if jobTimeout <= 0 {
		jobTimeout = 30 * time.Minute // Same default as the Game Maker service.
	}
	service, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "games.db"), WorkspacePath: filepath.Join(root, "games"), Enabled: true, AllowCreate: true, AllowEdit: true, JobTimeout: jobTimeout, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.SetSkillStatus(nil, true)
	previous := gamemaker.DefaultService()
	gamemaker.SetDefaultService(service)
	defer gamemaker.SetDefaultService(previous)
	cfg := &config.Config{}
	// Preserve the configured global cap; provider/model limits resolve normally.
	cfg.Agent.ContextWindow = original.Agent.ContextWindow
	cfg.CircuitBreaker = original.CircuitBreaker
	// Match config.Load defaults when these fields are omitted from YAML.
	if cfg.CircuitBreaker.LLMTimeoutSeconds <= 0 {
		cfg.CircuitBreaker.LLMTimeoutSeconds = 600
	}
	if cfg.CircuitBreaker.MaxToolCalls <= 0 {
		cfg.CircuitBreaker.MaxToolCalls = 10
	}
	cfg.GameMaker.Enabled = true
	cfg.GameMaker.AllowCreate = true
	cfg.GameMaker.AllowEdit = true
	cfg.Directories.ToolsDir = filepath.Join(root, "tools")
	cfg.Directories.WorkspaceDir = root
	requiredModels := map[string]string{"agnesai": "agnes-2.5-flash", "stepfun": "step-3.7-flash"}
	for _, id := range []string{"agnesai", "stepfun"} {
		p := original.FindProvider(id)
		if p == nil {
			t.Fatalf("missing selected provider %s", id)
		}
		expectedModel := requiredModels[id]
		if !strings.Contains(strings.ToLower(p.Model), expectedModel) {
			t.Fatalf("provider %s model %q does not contain required model ID %q", id, p.Model, expectedModel)
		}
		selected := *p
		selected.APIKey, err = vault.ReadSecret("provider_" + id + "_api_key")
		if err != nil {
			t.Fatalf("configured credential unavailable for %s", id)
		}
		security.RegisterSensitive(selected.APIKey)
		cfg.Providers = append(cfg.Providers, selected)
	}
	server := &Server{Cfg: cfg, GameMaker: service, Logger: logger, HistoryManager: memory.NewEphemeralHistoryManager()}
	server.Registry = tools.NewProcessRegistry(logger)
	server.ShortTermMem, err = memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer server.ShortTermMem.Close()
	first := cfg.Providers[0]
	server.LLMClient = llm.NewClientFromProviderWithConfig(cfg, first.Type, first.BaseURL, first.APIKey, first.AccountID)
	service.SetRunner(&gameMakerAgentRunner{server: server, service: service})
	var mu sync.RWMutex
	var projectID, jobID string
	parent, err := os.ReadFile("../gamemaker/testdata/studio-parent.html")
	if err != nil {
		t.Fatal(err)
	}
	// Keep polling across successive evaluation jobs; this is a local test parent.
	html := strings.Replace(string(parent), "if(polling||window.done)return", "if(polling)return", 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		pid, jid := projectID, jobID
		mu.RUnlock()
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, html)
			return
		case "/state":
			job, _ := service.GetJob(r.Context(), jid)
			grant, _ := service.CreatePreviewGrant(pid)
			json.NewEncoder(w).Encode(map[string]any{"job": job, "grant": grant})
			return
		case "/report":
			var report gamemaker.PreviewReport
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1600000)).Decode(&report) != nil {
				w.WriteHeader(400)
				return
			}
			if err := service.ReportPreview(pid, report); err != nil {
				http.Error(w, err.Error(), 400)
			}
			return
		}
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/game-maker/preview/"), "/", 2)
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		data, kind, err := service.PreviewFile(parts[0], parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Security-Policy", gameMakerPreviewCSP)
		w.Header().Set("Content-Type", kind)
		w.Write(data)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:8896")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go httpServer.Serve(listener)
	defer httpServer.Close()
	t.Log("Evaluation parent ready at http://127.0.0.1:8896/ (SSH tunnel, no service replacement)")
	reports := filepath.Join(filepath.Dir(configPath), "reports", "game-maker-evaluation")
	if dir := os.Getenv("GAMEMAKER_EVAL_REPORT_DIR"); dir != "" {
		reports = dir
	}
	if err = os.MkdirAll(reports, 0700); err != nil {
		t.Fatal(err)
	}
	for _, provider := range cfg.Providers {
		for _, task := range gameMakerEvaluationTasks(os.Getenv("GAMEMAKER_EVAL_BUILDER") == "1") {
			if os.Getenv("GAMEMAKER_EVAL_PRESENTATION") == "1" {
				if task.dimension == "3d" {
					task.prompt += " Add the built-in forest-rain atmosphere, blood-spray and blood-pool on actual target hits, muzzle-flash on shots, grass footsteps, rifle shots and flesh impact sounds. Use presentation in set_design and the existing guided lifecycle."
				} else {
					task.prompt += " Add the built-in coast atmosphere, pickup-glow on scoring, stone-debris on brick hits, impact-stone hit sound and victory/defeat cues. Use presentation in set_design and the existing guided lifecycle."
				}
			}
			if only := os.Getenv("GAMEMAKER_EVAL_PROVIDER"); only != "" && only != provider.ID {
				continue
			}
			if only := os.Getenv("GAMEMAKER_EVAL_TASK"); only != "" && only != task.name {
				continue
			}
			p, err := service.CreateProject(context.Background(), gamemaker.CreateProjectRequest{Name: task.name, Dimension: task.dimension, Description: task.prompt, ProviderID: provider.ID, Model: provider.Model})
			if err != nil {
				t.Fatal(err)
			}
			job, err := service.StartJob(context.Background(), p.ID, gamemaker.StartJobRequest{Prompt: task.prompt})
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			projectID = p.ID
			jobID = job.ID
			mu.Unlock()
			started := time.Now()
			for time.Since(started) < jobTimeout+time.Minute {
				time.Sleep(250 * time.Millisecond)
				job, _ = service.GetJob(context.Background(), job.ID)
				if job.Status == "ready" || job.Status == "failed" || job.Status == "cancelled" {
					break
				}
			}
			events, eventsComplete, eventsErr := readGameMakerEvalEvents(context.Background(), service, p.ID)
			if eventsErr != nil {
				t.Logf("event paging incomplete for %s/%s: %v", provider.ID, task.name, eventsErr)
			}
			reportEvents := events
			if len(reportEvents) > gameMakerEvalReportEvents {
				reportEvents = reportEvents[:gameMakerEvalReportEvents]
			}
			plan, planErr := service.GetPlan(context.Background(), job.ID)
			if job.Status == "ready" {
				// Publication removes staging; the revision owns the accepted plan.
				var data []byte
				data, planErr = os.ReadFile(filepath.Join(root, "games", filepath.FromSlash(p.ProjectKey), ".aurago", "game-plan.json"))
				if planErr == nil {
					plan = &gamemaker.GamePlan{}
					planErr = json.Unmarshal(data, plan)
				}
			}
			if (os.Getenv("GAMEMAKER_EVAL_PRESENTATION") == "1" || os.Getenv("GAMEMAKER_EVAL_BUILDER") == "1") && job.Status != "ready" {
				t.Errorf("presentation game did not become ready: %s: %s", job.Status, job.Error)
			}
			if os.Getenv("GAMEMAKER_EVAL_PRESENTATION") == "1" && (planErr != nil || plan == nil || plan.Presentation == nil || plan.Presentation.Environment == "" || len(plan.Presentation.Sounds) < 2 || len(plan.Presentation.Effects) < 2) {
				t.Errorf("%s/%s: missing requested presentation in accepted plan", provider.ID, task.name)
			}
			if (os.Getenv("GAMEMAKER_EVAL_PRESENTATION") == "1" || os.Getenv("GAMEMAKER_EVAL_BUILDER") == "1") && job.Status == "ready" {
				file, err := os.Create(filepath.Join(reports, provider.ID+"-"+task.name+".zip"))
				if err != nil {
					t.Fatal(err)
				}
				_, exportErr := service.WriteExport(context.Background(), p.ID, file)
				closeErr := file.Close()
				if exportErr != nil || closeErr != nil {
					t.Fatalf("retain playable evaluation: export=%v close=%v", exportErr, closeErr)
				}
			}
			result := map[string]any{"model": provider.Model, "provider": provider.ID, "task": task.name, "seconds": time.Since(started).Seconds(), "job": job, "plan": plan, "events": reportEvents, "events_total": len(events), "events_complete": eventsComplete && eventsErr == nil, "event_read_error": eventsErr != nil, "event_metrics": summarizeGameMakerEvalEvents(events, eventsComplete && eventsErr == nil), "brief_requirements": task.requirements, "context_cap": cfg.Agent.ContextWindow, "tool_limit": cfg.CircuitBreaker.MaxToolCalls, "llm_timeout_seconds": cfg.CircuitBreaker.LLMTimeoutSeconds, "job_timeout_seconds": jobTimeout.Seconds()}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			if err = os.WriteFile(filepath.Join(reports, provider.ID+"-"+task.name+".json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			t.Logf("model=%s task=%s status=%s seconds=%.1f error=%s", provider.Model, task.name, job.Status, time.Since(started).Seconds(), job.Error)
		}
	}
}

type gameMakerEvalTokenUsage struct {
	PromptTokens           int `json:"prompt_tokens"`
	CompletionTokens       int `json:"completion_tokens"`
	TotalTokens            int `json:"total_tokens"`
	Rounds                 int `json:"rounds"`
	ProviderUsageRounds    int `json:"provider_usage_rounds"`
	FallbackEstimateRounds int `json:"fallback_estimate_rounds"`
	EstimatedRounds        int `json:"estimated_rounds"`
	UnknownSourceRounds    int `json:"unknown_source_rounds,omitempty"`
}

type gameMakerEvalEventMetrics struct {
	EventCount         int                      `json:"event_count"`
	Complete           bool                     `json:"complete"`
	TypeCounts         map[string]int           `json:"type_counts"`
	ToolCalls          *int                     `json:"tool_calls"`
	ToolCallsReason    string                   `json:"tool_calls_reason"`
	SkillActivations   int                      `json:"skill_activations"`
	RepairRounds       *int                     `json:"repair_rounds"`
	RepairRoundsReason string                   `json:"repair_rounds_reason"`
	ValidationResults  int                      `json:"validation_results"`
	ValidationFailures int                      `json:"validation_failures"`
	TokenUsage         *gameMakerEvalTokenUsage `json:"token_usage"`
	TokenUsageReason   string                   `json:"token_usage_reason"`
}

// gameMakerEvalTokenInt decodes only bounded integral numeric values from the
// persisted event payload. Event JSON is untrusted input even though the
// broker writes this particular event.
func gameMakerEvalTokenInt(value any) (int, bool) {
	const maxValue = 10_000_000
	var n int64
	switch value := value.(type) {
	case int:
		n = int64(value)
	case int32:
		n = int64(value)
	case int64:
		n = value
	case uint:
		if uint64(value) > maxValue {
			return 0, false
		}
		n = int64(value)
	case uint32:
		n = int64(value)
	case uint64:
		if value > maxValue {
			return 0, false
		}
		n = int64(value)
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > maxValue || value != math.Trunc(value) {
			return 0, false
		}
		n = int64(value)
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0, false
		}
		n = parsed
	default:
		return 0, false
	}
	if n < 0 || n > maxValue {
		return 0, false
	}
	return int(n), true
}

func addGameMakerEvalTokenValue(current, value int) int {
	const maxAggregate = 1_000_000_000
	if value <= 0 || current >= maxAggregate-value {
		return maxAggregate
	}
	return current + value
}

const (
	gameMakerEvalEventPageSize = 500
	gameMakerEvalMaxEvents     = 20000
	gameMakerEvalReportEvents  = 500
)

func readGameMakerEvalEvents(ctx context.Context, service *gamemaker.Service, projectID string) ([]gamemaker.Event, bool, error) {
	events := make([]gamemaker.Event, 0, gameMakerEvalEventPageSize)
	var afterID int64
	for len(events) < gameMakerEvalMaxEvents {
		limit := gameMakerEvalEventPageSize
		if remaining := gameMakerEvalMaxEvents - len(events); remaining < limit {
			limit = remaining
		}
		page, err := service.EventsAfter(ctx, projectID, afterID, limit)
		if err != nil {
			return events, false, err
		}
		if len(page) == 0 {
			return events, true, nil
		}
		nextID := afterID
		for _, event := range page {
			if event.ID <= nextID {
				return events, false, fmt.Errorf("event IDs did not advance")
			}
			nextID = event.ID
		}
		events = append(events, page...)
		afterID = nextID
		if len(page) < limit {
			return events, true, nil
		}
	}
	return events, false, nil
}

// summarizeGameMakerEvalEvents only derives counters from persisted, bounded
// Game Maker events. Token usage is emitted once per actual LLM round by the
// evaluation broker and is retained without prompt bodies or credentials.
func summarizeGameMakerEvalEvents(events []gamemaker.Event, complete bool) gameMakerEvalEventMetrics {
	metrics := gameMakerEvalEventMetrics{
		EventCount:         len(events),
		Complete:           complete,
		TypeCounts:         map[string]int{},
		ToolCallsReason:    "persisted Game Maker events contain no tool_call records; skill_activations is the only tool-related event counter",
		RepairRoundsReason: "phase events do not expose a repair label; repeated building boundaries are counted after the initial build",
		TokenUsageReason:   "no complete token_usage event rounds were persisted",
	}
	buildingPhases := 0
	toolCallEvents := 0
	var tokenUsage gameMakerEvalTokenUsage
	tokenUsageEvents := 0
	invalidTokenUsageEvents := 0
	for _, event := range events {
		metrics.TypeCounts[event.Type]++
		switch event.Type {
		case "tool_call":
			toolCallEvents++
		case "skill_activation":
			metrics.SkillActivations++
		case "phase":
			if phase, _ := event.Payload["phase"].(string); phase == "building" {
				buildingPhases++
			}
		case "validation_result":
			metrics.ValidationResults++
			if validationResultStatus(event.Payload) == "failed" {
				metrics.ValidationFailures++
			}
		case "token_usage":
			prompt, promptOK := gameMakerEvalTokenInt(event.Payload["prompt_tokens"])
			completion, completionOK := gameMakerEvalTokenInt(event.Payload["completion_tokens"])
			total, totalOK := gameMakerEvalTokenInt(event.Payload["total_tokens"])
			if !promptOK || !completionOK || !totalOK {
				invalidTokenUsageEvents++
				continue
			}
			tokenUsage.PromptTokens = addGameMakerEvalTokenValue(tokenUsage.PromptTokens, prompt)
			tokenUsage.CompletionTokens = addGameMakerEvalTokenValue(tokenUsage.CompletionTokens, completion)
			tokenUsage.TotalTokens = addGameMakerEvalTokenValue(tokenUsage.TotalTokens, total)
			tokenUsage.Rounds++
			tokenUsageEvents++
			if estimated, _ := event.Payload["estimated"].(bool); estimated {
				tokenUsage.EstimatedRounds++
			}
			source, _ := event.Payload["token_source"].(string)
			switch source {
			case "provider_usage":
				tokenUsage.ProviderUsageRounds++
			case "fallback_estimate":
				tokenUsage.FallbackEstimateRounds++
			default:
				tokenUsage.UnknownSourceRounds++
			}
		}
	}
	if !complete {
		metrics.ToolCalls = nil
		metrics.ToolCallsReason = "event stream reached the 500-record read limit; tool-call count is incomplete"
		metrics.RepairRounds = nil
		metrics.RepairRoundsReason = "event stream reached the 500-record read limit; repair count is incomplete"
		metrics.TokenUsage = nil
		metrics.TokenUsageReason = "event stream reached the 500-record read limit; token usage is incomplete"
	} else {
		if toolCallEvents > 0 {
			count := toolCallEvents
			metrics.ToolCalls = &count
			metrics.ToolCallsReason = "counted persisted tool_call events"
		}
		if buildingPhases > 0 {
			repairs := buildingPhases - 1
			if repairs < 0 {
				repairs = 0
			}
			metrics.RepairRounds = &repairs
			metrics.RepairRoundsReason = "counted phase=building events after the initial build; validation failure details remain separate"
		} else {
			metrics.RepairRounds = nil
			metrics.RepairRoundsReason = "no phase=building event was persisted"
		}
		if tokenUsageEvents > 0 {
			metrics.TokenUsage = &tokenUsage
			if invalidTokenUsageEvents > 0 {
				metrics.TokenUsageReason = fmt.Sprintf("aggregated %d valid token_usage rounds; %d malformed rounds ignored", tokenUsageEvents, invalidTokenUsageEvents)
			} else {
				metrics.TokenUsageReason = "aggregated persisted token_usage rounds; no prompt bodies or credentials are retained"
			}
		} else if invalidTokenUsageEvents > 0 {
			metrics.TokenUsageReason = fmt.Sprintf("%d malformed token_usage events were ignored", invalidTokenUsageEvents)
		}
	}
	return metrics
}

func validationResultStatus(payload map[string]any) string {
	result, _ := payload["result"].(map[string]any)
	status, _ := result["gameplay_status"].(string)
	if status == "" {
		if ok, exists := result["ok"].(bool); exists {
			if ok {
				return "passed"
			}
			return "failed"
		}
	}
	return status
}

func TestGameMakerEvalEventMetrics(t *testing.T) {
	failed := map[string]any{"gameplay_status": "failed"}
	events := []gamemaker.Event{
		{Type: "phase", Payload: map[string]any{"phase": "planning"}},
		{Type: "phase", Payload: map[string]any{"phase": "building"}},
		{Type: "validation_result", Payload: map[string]any{"result": failed}},
		{Type: "phase", Payload: map[string]any{"phase": "building"}},
		{Type: "validation_result", Payload: map[string]any{"result": map[string]any{"ok": true}}},
		{Type: "skill_activation", Payload: map[string]any{"tool_id": "unused-in-this-test"}},
		{Type: "token_usage", Payload: map[string]any{"prompt_tokens": 120, "completion_tokens": 80, "total_tokens": 200, "estimated": false, "token_source": "provider_usage"}},
		{Type: "token_usage", Payload: map[string]any{"prompt_tokens": 50, "completion_tokens": 30, "total_tokens": 80, "estimated": true, "token_source": "fallback_estimate"}},
	}
	metrics := summarizeGameMakerEvalEvents(events, true)
	if metrics.ToolCalls != nil {
		t.Fatalf("tool calls = %v, want unavailable when tool_call events are absent", *metrics.ToolCalls)
	}
	if metrics.RepairRounds == nil || *metrics.RepairRounds != 1 {
		t.Fatalf("repair rounds = %v, want 1", metrics.RepairRounds)
	}
	if metrics.ValidationResults != 2 || metrics.ValidationFailures != 1 || metrics.SkillActivations != 1 {
		t.Fatalf("event metrics = %+v", metrics)
	}
	if metrics.TokenUsage == nil || metrics.TokenUsage.Rounds != 2 || metrics.TokenUsage.PromptTokens != 170 || metrics.TokenUsage.CompletionTokens != 110 || metrics.TokenUsage.TotalTokens != 280 || metrics.TokenUsage.ProviderUsageRounds != 1 || metrics.TokenUsage.FallbackEstimateRounds != 1 || metrics.TokenUsage.EstimatedRounds != 1 {
		t.Fatalf("token metrics = %+v", metrics.TokenUsage)
	}
	truncated := summarizeGameMakerEvalEvents(events, false)
	if truncated.RepairRounds != nil || truncated.ToolCalls != nil {
		t.Fatal("truncated event stream must not claim complete counters")
	}
}
