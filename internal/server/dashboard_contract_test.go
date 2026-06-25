package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

	for _, key := range []string{"core_memory_facts", "chat_messages", "vectordb_entries", "knowledge_graph", "journal_entries", "notes_count", "error_patterns", "milestones", "episodic", "pending_actions", "memory_conflicts", "memory_health"} {
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
	for _, key := range []string{"usage", "confidence", "curator", "strategy"} {
		if _, ok := health[key]; !ok {
			t.Fatalf("memory_health missing key %q", key)
		}
	}
	if _, ok := health["effectiveness"]; !ok {
		t.Fatal("memory_health missing key \"effectiveness\"")
	}
}

func TestHandleDashboardMemoryKnowledgeGraphHealthFields(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	if err := kg.AddNode("router", "Router", map[string]string{"type": "device"}); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	s := &Server{ShortTermMem: stm, KG: kg}
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/memory", nil)
	rec := httptest.NewRecorder()
	handleDashboardMemory(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	kgPayload, ok := body["knowledge_graph"].(map[string]interface{})
	if !ok {
		t.Fatalf("knowledge_graph has unexpected type %T", body["knowledge_graph"])
	}
	for _, key := range []string{"nodes", "edges", "dirty_nodes", "semantic_enabled"} {
		if _, ok := kgPayload[key]; !ok {
			t.Fatalf("knowledge_graph missing key %q", key)
		}
	}
	if nodes, _ := kgPayload["nodes"].(float64); nodes != 1 {
		t.Fatalf("nodes = %v, want 1", kgPayload["nodes"])
	}
	if dirty, _ := kgPayload["dirty_nodes"].(float64); dirty != 1 {
		t.Fatalf("dirty_nodes = %v, want 1", kgPayload["dirty_nodes"])
	}
}

func TestHandleDashboardCoreMemoryMutateDeleteAllRequiresConfirmation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { stm.Close() })

	if _, err := stm.AddCoreMemoryFact("first"); err != nil {
		t.Fatalf("AddCoreMemoryFact first: %v", err)
	}
	if _, err := stm.AddCoreMemoryFact("second"); err != nil {
		t.Fatalf("AddCoreMemoryFact second: %v", err)
	}

	s := &Server{ShortTermMem: stm, Logger: logger}
	handler := handleDashboardCoreMemoryMutate(s, NewSSEBroadcaster())

	missingConfirm := httptest.NewRequest(http.MethodDelete, "/api/dashboard/core-memory/mutate", bytes.NewReader([]byte(`{"all":true}`)))
	missingConfirm.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, missingConfirm)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete all without confirmation status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/dashboard/core-memory/mutate", bytes.NewReader([]byte(`{"all":true,"confirm":"DELETE_ALL_CORE_MEMORY"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete all status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode delete all response: %v", err)
	}
	if got := int(body["deleted"].(float64)); got != 2 {
		t.Fatalf("deleted = %d, want 2", got)
	}
	count, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("core memory count after delete all = %d, want 0", count)
	}
}

func TestHandleDashboardCoreMemoryMutateRejectsWrites(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	s := &Server{ShortTermMem: stm, Logger: logger}
	handler := handleDashboardCoreMemoryMutate(s, NewSSEBroadcaster())

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/dashboard/core-memory/mutate",
		bytes.NewReader([]byte(`{"fact":"User prefers concise German answers"}`)),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	count, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("core memory count = %d, want 0", count)
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
	cfg.Agent.AdaptiveTools.MaxTotalTools = 32
	cfg.Agent.AdaptiveTools.ProviderProfilesEnabled = true
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

	for _, key := range []string{
		"total_calls",
		"by_tool",
		"top_tools",
		"recent",
		"adaptive_enabled",
		"adaptive_scores",
		"max_tools",
		"max_total_tools",
		"provider_tool_profile",
		"last_tool_filter_report",
		"agent_telemetry",
	} {
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
	if got := int(body["max_total_tools"].(float64)); got != 32 {
		t.Fatalf("max_total_tools = %d, want 32", got)
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
	t.Cleanup(func() { _ = mgr.Close() })
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

func TestHandleDashboardCronjobsContractUpdateAndDelete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronMgr := tools.NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = cronMgr.Close() })

	if _, err := cronMgr.ManageScheduleWithSource("add", "mission_daily", "0 9 * * *", "Run mission", "en", "mission"); err != nil {
		t.Fatalf("add cron job: %v", err)
	}
	if _, err := cronMgr.ManageSchedule("disable", "mission_daily", "", "", "en"); err != nil {
		t.Fatalf("disable cron job: %v", err)
	}

	s := &Server{CronManager: cronMgr, Logger: logger}
	handler := handleDashboardCronjobs(s)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/cronjobs?q=mission&source=mission&status=disabled", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d; body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cronjobs response: %v", err)
	}
	for _, key := range []string{"jobs", "total", "enabled", "disabled"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("cronjobs payload missing key %q", key)
		}
	}
	if got := int(body["total"].(float64)); got != 1 {
		t.Fatalf("total = %d, want 1", got)
	}
	jobs := body["jobs"].([]interface{})
	job := jobs[0].(map[string]interface{})
	if job["source"] != "mission" || job["disabled"] != true {
		t.Fatalf("job source/disabled = %#v/%#v, want mission/true", job["source"], job["disabled"])
	}

	update := bytes.NewReader([]byte(`{"id":"mission_daily","cron_expr":"0 10 * * *","task_prompt":"Run updated mission","disabled":false}`))
	req = httptest.NewRequest(http.MethodPut, "/api/dashboard/cronjobs", update)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	updatedJobs := cronMgr.GetJobs()
	if len(updatedJobs) != 1 {
		t.Fatalf("cron job count after update = %d, want 1", len(updatedJobs))
	}
	updated := updatedJobs[0]
	if updated.Source != "mission" {
		t.Fatalf("source after update = %q, want mission", updated.Source)
	}
	if updated.Disabled {
		t.Fatal("updated cron job should be enabled")
	}
	if updated.CronExpr != "0 10 * * *" || updated.TaskPrompt != "Run updated mission" {
		t.Fatalf("updated job = %+v", updated)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/dashboard/cronjobs/mission_daily", nil)
	rec = httptest.NewRecorder()
	handleDashboardCronjobByID(s).ServeHTTP(rec, deleteReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := len(cronMgr.GetJobs()); got != 0 {
		t.Fatalf("cron job count after delete = %d, want 0", got)
	}
}

func TestHandleDashboardCronjobsReportsRuntimeErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)

	dir := t.TempDir()
	legacyJobs := []byte(`[
		{"id":"good-job","cron_expr":"0 * * * *","task_prompt":"run good job"},
		{"id":"bad-job","cron_expr":"not a cron expression","task_prompt":"run bad job"}
	]`)
	if err := os.WriteFile(filepath.Join(dir, "crontab.json"), legacyJobs, 0o600); err != nil {
		t.Fatalf("write legacy cron json: %v", err)
	}

	cronMgr := tools.NewCronManager(dir)
	t.Cleanup(func() { _ = cronMgr.Close() })
	if err := cronMgr.Start(func(string) {}); err == nil {
		t.Fatal("Start returned nil, want invalid persisted cron expression")
	}

	s := &Server{CronManager: cronMgr, Logger: logger}
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/cronjobs?status=error", nil)
	rec := httptest.NewRecorder()
	handleDashboardCronjobs(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d; body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode cronjobs response: %v", err)
	}
	if got := int(body["total"].(float64)); got != 1 {
		t.Fatalf("total = %d, want 1", got)
	}
	if got := int(body["errors"].(float64)); got != 1 {
		t.Fatalf("errors = %d, want 1", got)
	}
	jobs := body["jobs"].([]interface{})
	job := jobs[0].(map[string]interface{})
	if job["id"] != "bad-job" || job["status"] != "error" || job["registered"] != false {
		t.Fatalf("bad job payload = %#v, want error and unregistered", job)
	}
	if lastErr, _ := job["last_error"].(string); !strings.Contains(lastErr, "not a cron expression") {
		t.Fatalf("last_error = %q, want parse error", lastErr)
	}
}

func TestHandleCronAPILegacyContract(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronMgr := tools.NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = cronMgr.Close() })

	s := &Server{CronManager: cronMgr, Logger: logger}
	handler := handleCronAPI(s)

	postReq := httptest.NewRequest(http.MethodPost, "/api/cron", bytes.NewReader([]byte(`{"id":"legacy-job","cron_expr":"0 8 * * *","task_prompt":"run legacy"}`)))
	postReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, postReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("post status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/cron", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, getReq)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"legacy-job"`) {
		t.Fatalf("get status/body = %d/%s, want listed legacy job", rec.Code, rec.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/cron", bytes.NewReader([]byte(`{"id":"legacy-job","cron_expr":"0 9 * * *","task_prompt":"run updated"}`)))
	putReq.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, putReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	jobs := cronMgr.GetJobs()
	if len(jobs) != 1 || jobs[0].CronExpr != "0 9 * * *" || jobs[0].TaskPrompt != "run updated" {
		t.Fatalf("jobs after put = %+v, want single updated job", jobs)
	}

	badDeleteReq := httptest.NewRequest(http.MethodDelete, "/api/cron?id=missing", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, badDeleteReq)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/cron?id=legacy-job", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, deleteReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := len(cronMgr.GetJobs()); got != 0 {
		t.Fatalf("cron job count after delete = %d, want 0", got)
	}
}

func TestHandleCronAPIPutInvalidExpressionPreservesExistingJob(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronMgr := tools.NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = cronMgr.Close() })

	s := &Server{CronManager: cronMgr, Logger: logger}
	handler := handleCronAPI(s)

	postReq := httptest.NewRequest(http.MethodPost, "/api/cron", bytes.NewReader([]byte(`{"id":"legacy-job","cron_expr":"0 8 * * *","task_prompt":"run legacy"}`)))
	postReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, postReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("post status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/cron", bytes.NewReader([]byte(`{"id":"legacy-job","cron_expr":"not a cron expression","task_prompt":"bad update"}`)))
	putReq.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, putReq)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid put status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	jobs := cronMgr.GetJobs()
	if len(jobs) != 1 || jobs[0].ID != "legacy-job" || jobs[0].CronExpr != "0 8 * * *" || jobs[0].TaskPrompt != "run legacy" {
		t.Fatalf("jobs after invalid put = %+v, want original job preserved", jobs)
	}
}

func TestHandleCronAPIDisabledMutationsWriteSingleJSONResponse(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	cronMgr := tools.NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = cronMgr.Close() })

	s := &Server{CronManager: cronMgr, Logger: logger}
	handler := handleCronAPI(s)

	postReq := httptest.NewRequest(http.MethodPost, "/api/cron", bytes.NewReader([]byte(`{"id":"legacy-disabled","cron_expr":"0 7 * * *","task_prompt":"run disabled","disabled":true}`)))
	postReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, postReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("disabled post status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("disabled post returned invalid JSON body: %s", rec.Body.String())
	}
	jobs := cronMgr.GetJobs()
	if len(jobs) != 1 || !jobs[0].Disabled {
		t.Fatalf("jobs after disabled post = %+v, want one disabled job", jobs)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/cron", bytes.NewReader([]byte(`{"id":"legacy-disabled","cron_expr":"0 8 * * *","task_prompt":"still disabled","disabled":true}`)))
	putReq.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, putReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("disabled put status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("disabled put returned invalid JSON body: %s", rec.Body.String())
	}
	jobs = cronMgr.GetJobs()
	if len(jobs) != 1 || !jobs[0].Disabled || jobs[0].CronExpr != "0 8 * * *" || jobs[0].TaskPrompt != "still disabled" {
		t.Fatalf("jobs after disabled put = %+v, want one updated disabled job", jobs)
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

func TestHandleDashboardHelperLLMContract(t *testing.T) {
	agent.ResetHelperLLMRuntimeStats()
	t.Cleanup(agent.ResetHelperLLMRuntimeStats)

	cfg := &config.Config{}
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperProviderType = "openrouter"
	cfg.LLM.HelperResolvedModel = "google/gemini-2.0-flash-lite"

	agent.MergeHelperLLMRuntimeStats("content_summaries", agent.HelperLLMOperationStats{
		Requests:     3,
		CacheHits:    1,
		LLMCalls:     2,
		Fallbacks:    1,
		BatchedItems: 5,
		SavedCalls:   2,
		LastDetail:   "cache_hit",
	})

	s := &Server{Cfg: cfg}

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/helper-llm", nil)
	rec := httptest.NewRecorder()

	handleDashboardHelperLLM(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	for _, key := range []string{"enabled", "updated_at", "totals", "operations"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("helper llm payload missing key %q", key)
		}
	}

	totals, ok := body["totals"].(map[string]interface{})
	if !ok {
		t.Fatalf("totals has unexpected type %T", body["totals"])
	}
	for _, key := range []string{"requests", "cache_hits", "llm_calls", "fallbacks", "batched_items", "saved_calls"} {
		if _, ok := totals[key]; !ok {
			t.Fatalf("helper llm totals missing key %q", key)
		}
	}

	operations, ok := body["operations"].(map[string]interface{})
	if !ok {
		t.Fatalf("operations has unexpected type %T", body["operations"])
	}
	if _, ok := operations["content_summaries"]; !ok {
		t.Fatalf("operations missing content_summaries entry: %#v", operations)
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
	cfg.LLM.HelperEnabled = true
	cfg.LLM.HelperProvider = "helper"
	cfg.LLM.HelperProviderType = "openrouter"
	cfg.LLM.HelperResolvedModel = "google/gemini-2.0-flash-lite"
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

	for _, key := range []string{"agent", "integrations", "missions", "context", "skills", "planner"} {
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
	for _, key := range []string{"docker", "helper_llm", "memory_analysis", "llm_guardian", "homepage", "netlify", "skill_manager"} {
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

func TestHandleDashboardAuditContractAndDelete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	if _, err := stm.RecordAuditEvent(memory.AuditEvent{
		Source:     memory.AuditSourceAgentTool,
		EventType:  "tool_call",
		TargetName: "execute_shell",
		Status:     memory.AuditStatusSuccess,
		Summary:    "execute_shell completed",
		Detail:     "exit code 0",
		DurationMS: 42,
	}); err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}
	if _, err := stm.RecordAuditEvent(memory.AuditEvent{
		Source:     memory.AuditSourceHeartbeat,
		EventType:  "heartbeat_run",
		TargetName: "scheduler",
		Status:     memory.AuditStatusError,
		Summary:    "Heartbeat failed",
		Detail:     "model unavailable",
	}); err != nil {
		t.Fatalf("RecordAuditEvent heartbeat: %v", err)
	}

	s := &Server{ShortTermMem: stm, Logger: logger}
	handler := handleDashboardAudit(s, NewSSEBroadcaster())

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/audit?source=agent_tool&status=success&q=shell&limit=10", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d; body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	for _, key := range []string{"entries", "total", "limit", "offset"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("audit payload missing key %q", key)
		}
	}
	if got := int(body["total"].(float64)); got != 1 {
		t.Fatalf("total = %d, want 1", got)
	}

	badBulk := httptest.NewRequest(http.MethodDelete, "/api/dashboard/audit", bytes.NewReader([]byte(`{"all":true}`)))
	badBulk.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, badBulk)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bulk delete without confirmation status = %d, want 400", rec.Code)
	}

	goodBulk := httptest.NewRequest(http.MethodDelete, "/api/dashboard/audit", bytes.NewReader([]byte(`{"source":"heartbeat","confirm":"DELETE_AUDIT_EVENTS"}`)))
	goodBulk.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, goodBulk)
	if rec.Code != http.StatusOK {
		t.Fatalf("bulk delete status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}
