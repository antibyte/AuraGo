package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestHandleDashboardPromptStatsContract(t *testing.T) {
	prompts.RecordBuild(prompts.PromptBuildRecord{
		Timestamp:     time.Now(),
		Tier:          "full",
		RawLen:        1000,
		OptimizedLen:  700,
		FormatSavings: 50,
		ShedSavings:   100,
		FilterSavings: 150,
		Tokens:        120,
		ModulesLoaded: 8,
		ModulesUsed:   6,
		GuidesCount:   2,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/prompt-stats", nil)
	rec := httptest.NewRecorder()

	handleDashboardPromptStats().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"total_builds", "avg_raw_len", "avg_optimized_len", "avg_saved_chars", "tier_distribution", "recent"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("prompt stats missing key %q", key)
		}
	}
	if got := int(body["total_builds"].(float64)); got < 1 {
		t.Fatalf("total_builds = %d, want >= 1", got)
	}
}

func TestHandleDashboardMemoryContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.RecordMemoryUsage("doc-1", "ltm_retrieved", "sess-1", 0.9, false); err != nil {
		t.Fatalf("RecordMemoryUsage retrieved: %v", err)
	}
	if err := stm.RecordMemoryUsage("doc-2", "ltm_predicted", "sess-1", 0.4, false); err != nil {
		t.Fatalf("RecordMemoryUsage predicted: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-1", memory.MemoryMetaUpdate{
		ExtractionConfidence: 0.92,
		VerificationStatus:   "confirmed",
		SourceType:           "memory_analysis",
		SourceReliability:    0.88,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails doc-1: %v", err)
	}
	if err := stm.UpsertMemoryMetaWithDetails("doc-2", memory.MemoryMetaUpdate{
		ExtractionConfidence: 0.40,
		VerificationStatus:   "unverified",
		SourceType:           "memory_analysis",
		SourceReliability:    0.45,
	}); err != nil {
		t.Fatalf("UpsertMemoryMetaWithDetails doc-2: %v", err)
	}
	if err := stm.InsertEpisodicMemoryWithDetails("2026-03-27", "Deploy", "Homepage rollout finished", map[string]string{"scope": "homepage"}, 3, "memory_analysis", memory.EpisodicMemoryDetails{
		SessionID:        "sess-1",
		Participants:     []string{"user", "agent"},
		RelatedDocIDs:    []string{"doc-1"},
		EmotionalValence: 0.3,
	}); err != nil {
		t.Fatalf("InsertEpisodicMemoryWithDetails: %v", err)
	}

	s := &Server{ShortTermMem: stm}
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/memory", nil)
	rec := httptest.NewRecorder()

	handleDashboardMemory(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"core_memory_facts", "chat_messages", "vectordb_entries", "knowledge_graph", "journal_entries", "notes_count", "error_patterns", "milestones", "episodic", "memory_health"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("memory payload missing key %q", key)
		}
	}

	episodic, ok := body["episodic"].(map[string]interface{})
	if !ok {
		t.Fatalf("episodic has unexpected type %T", body["episodic"])
	}
	for _, key := range []string{"total_count", "recent_count", "by_source", "recent_cards"} {
		if _, ok := episodic[key]; !ok {
			t.Fatalf("episodic payload missing key %q", key)
		}
	}

	health, ok := body["memory_health"].(map[string]interface{})
	if !ok {
		t.Fatalf("memory_health has unexpected type %T", body["memory_health"])
	}
	for _, key := range []string{"usage", "confidence", "curator"} {
		if _, ok := health[key]; !ok {
			t.Fatalf("memory_health missing key %q", key)
		}
	}
}

func TestHandleDashboardEmotionHistoryContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })
	if err := stm.InsertEmotionStateHistory(memory.EmotionState{
		Description:              "I feel calmer after the successful fix.",
		PrimaryMood:              memory.MoodFocused,
		SecondaryMood:            "relieved",
		Valence:                  0.35,
		Arousal:                  0.42,
		Confidence:               0.81,
		Cause:                    "the deployment issue was resolved",
		Source:                   "llm_structured",
		RecommendedResponseStyle: "calm_and_precise",
	}, "plan_completed"); err != nil {
		t.Fatalf("InsertEmotionStateHistory: %v", err)
	}

	s := &Server{ShortTermMem: stm}
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/emotion-history?hours=24", nil)
	rec := httptest.NewRecorder()

	handleDashboardEmotionHistory(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"entries", "summary", "hours", "count"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("emotion history payload missing key %q", key)
		}
	}

	entries, ok := body["entries"].([]interface{})
	if !ok || len(entries) != 1 {
		t.Fatalf("entries = %#v, want single entry", body["entries"])
	}

	summary, ok := body["summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("summary has unexpected type %T", body["summary"])
	}
	for _, key := range []string{"count", "latest_cause", "latest_style", "latest_source", "average_valence", "average_arousal", "trigger_counts"} {
		if _, ok := summary[key]; !ok {
			t.Fatalf("summary missing key %q", key)
		}
	}
}

func TestHandleDashboardToolStatsContract(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AdaptiveTools.Enabled = true
	cfg.Agent.AdaptiveTools.MaxTools = 17
	cfg.Agent.AdaptiveTools.DecayHalfLifeDays = 7
	cfg.Agent.AdaptiveTools.WeightSuccessRate = true

	prompts.RecordToolUsage("shell", "execute", true)
	prompts.RecordToolUsage("docker", "list", false)
	prompts.RecordAdaptiveToolUsage("shell", true)
	prompts.RecordAdaptiveToolUsage("docker", false)
	agent.RecordToolParseSource(agent.ToolCallParseSourceNative)
	agent.RecordToolRecoveryEvent("provider_422_recovered")
	agent.RecordToolPolicyEvent("conservative_profile_applied")
	scope := agent.AgentTelemetryScope{ProviderType: "openrouter", Model: "deepseek-chat"}
	agent.RecordToolParseSourceForScope(scope, agent.ToolCallParseSourceNative)
	agent.RecordScopedToolResultForTool(scope, "homepage", false)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/tool-stats", nil)
	rec := httptest.NewRecorder()

	handleDashboardToolStats(cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"total_calls", "by_tool", "top_tools", "recent", "adaptive_enabled", "adaptive_scores", "max_tools", "agent_telemetry"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("tool stats missing key %q", key)
		}
	}
	if got := int(body["total_calls"].(float64)); got < 2 {
		t.Fatalf("total_calls = %d, want >= 2", got)
	}
	if enabled, ok := body["adaptive_enabled"].(bool); !ok || !enabled {
		t.Fatalf("adaptive_enabled = %#v, want true", body["adaptive_enabled"])
	}
	if got := int(body["max_tools"].(float64)); got != 17 {
		t.Fatalf("max_tools = %d, want 17", got)
	}
	telemetry, ok := body["agent_telemetry"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent_telemetry has unexpected type %T", body["agent_telemetry"])
	}
	for _, key := range []string{"parse_sources", "recovery_events", "policy_events", "scopes"} {
		if _, ok := telemetry[key]; !ok {
			t.Fatalf("agent_telemetry missing key %q", key)
		}
	}
	scopes, ok := telemetry["scopes"].([]interface{})
	if !ok || len(scopes) == 0 {
		t.Fatalf("agent_telemetry scopes = %#v, want non-empty scope list", telemetry["scopes"])
	}
	foundFamilyData := false
	for _, raw := range scopes {
		scopeMap, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := scopeMap["tool_families"]; ok {
			foundFamilyData = true
			break
		}
	}
	if !foundFamilyData {
		t.Fatal("expected at least one telemetry scope with tool_families")
	}
}

func TestHandleDashboardActivityContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := tools.NewBackgroundTaskManager(t.TempDir(), logger)
	if _, err := mgr.ScheduleFollowUp("background diagnostic", tools.BackgroundTaskScheduleOptions{
		Source:      "follow_up",
		Description: "Autonomous follow-up",
	}); err != nil {
		t.Fatalf("ScheduleFollowUp: %v", err)
	}

	s := &Server{BackgroundTasks: mgr}
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/activity", nil)
	rec := httptest.NewRecorder()

	handleDashboardActivity(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"cron_jobs", "processes", "webhooks", "coagents", "background_tasks", "background_task_summary"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("activity payload missing key %q", key)
		}
	}
	tasks, ok := body["background_tasks"].([]interface{})
	if !ok || len(tasks) == 0 {
		t.Fatalf("background_tasks = %#v, want non-empty task list", body["background_tasks"])
	}
	summary, ok := body["background_task_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("background_task_summary has unexpected type %T", body["background_task_summary"])
	}
	if got := int(summary["total"].(float64)); got < 1 {
		t.Fatalf("background task total = %d, want >= 1", got)
	}
}

func TestHandleDashboardGuardianContract(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLMGuardian.Enabled = true
	cfg.LLMGuardian.DefaultLevel = "medium"
	cfg.LLMGuardian.FailSafe = "quarantine"

	guardian := &security.LLMGuardian{Metrics: &security.GuardianMetrics{}}
	guardian.Metrics.Record(security.GuardianResult{
		Decision:   security.DecisionAllow,
		TokensUsed: 42,
	})

	s := &Server{
		Cfg:         cfg,
		LLMGuardian: guardian,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/guardian", nil)
	rec := httptest.NewRecorder()

	handleDashboardGuardian(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"enabled", "level", "fail_safe", "metrics"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("guardian payload missing key %q", key)
		}
	}
	metrics, ok := body["metrics"].(map[string]interface{})
	if !ok {
		t.Fatalf("metrics has unexpected type %T", body["metrics"])
	}
	for _, key := range []string{"total_checks", "allows", "total_tokens", "cache_hit_rate"} {
		if _, ok := metrics[key]; !ok {
			t.Fatalf("guardian metrics missing key %q", key)
		}
	}
}

func TestHandleDashboardOverviewContract(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.LLM.ProviderType = "openrouter"
	cfg.Personality.CorePersonality = "helper"
	cfg.Agent.ContextWindow = 32000
	cfg.Maintenance.Enabled = true
	cfg.Docker.Enabled = true
	cfg.MemoryAnalysis.Enabled = true
	cfg.LLMGuardian.Enabled = true
	cfg.Homepage.Enabled = true
	cfg.Netlify.Enabled = true
	cfg.Tools.SkillManager.Enabled = true

	s := &Server{Cfg: cfg}

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/overview", nil)
	rec := httptest.NewRecorder()

	handleDashboardOverview(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"agent", "integrations", "missions", "context", "skills"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("overview payload missing key %q", key)
		}
	}

	agentInfo, ok := body["agent"].(map[string]interface{})
	if !ok {
		t.Fatalf("agent info has unexpected type %T", body["agent"])
	}
	if agentInfo["model"] != "test-model" {
		t.Fatalf("agent model = %#v, want test-model", agentInfo["model"])
	}
	if agentInfo["provider"] != "openrouter" {
		t.Fatalf("agent provider = %#v, want openrouter", agentInfo["provider"])
	}

	integrations, ok := body["integrations"].(map[string]interface{})
	if !ok {
		t.Fatalf("integrations has unexpected type %T", body["integrations"])
	}
	for _, key := range []string{"docker", "memory_analysis", "llm_guardian", "homepage", "netlify", "skill_manager"} {
		if _, ok := integrations[key]; !ok {
			t.Fatalf("integrations missing key %q", key)
		}
	}
}

func TestHandleActivityOverviewContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	if err := stm.InitNotesTables(); err != nil {
		t.Fatalf("InitNotesTables: %v", err)
	}
	if _, err := stm.AddNote("todo", "Validate activity endpoint", "", 3, ""); err != nil {
		t.Fatalf("AddNote: %v", err)
	}
	if _, err := stm.InsertActivityTurn(memory.ActivityTurn{
		Date:            time.Now().Format("2006-01-02"),
		SessionID:       "default",
		Channel:         "web_chat",
		UserRelevant:    true,
		Intent:          "Inspect the new activity endpoint",
		UserRequest:     "Show the recent activity overview",
		UserGoal:        "Inspect the new activity endpoint",
		ActionsTaken:    []string{"query_memory"},
		Outcomes:        []string{"Built the activity endpoint response"},
		ImportantPoints: []string{"Endpoint returns a recent overview"},
		ToolNames:       []string{"query_memory"},
	}); err != nil {
		t.Fatalf("InsertActivityTurn: %v", err)
	}

	s := &Server{ShortTermMem: stm, Logger: logger}
	req := httptest.NewRequest(http.MethodGet, "/api/memory/activity-overview?days=7&include_entries=true", nil)
	rec := httptest.NewRecorder()

	handleActivityOverview(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	for _, key := range []string{"overview_summary", "days", "highlights", "important_points", "pending_items", "top_goals", "entries"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("activity overview missing key %q", key)
		}
	}
}
