package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

func newDashboardSummaryTestMemory(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}

func TestDashboardSummaryWithoutSourcesReportsZeros(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 41, 0, 0, time.UTC)
	got := buildDashboardSummary(&Server{Cfg: &config.Config{}}, now)

	if got.GeneratedAt != "2026-10-17T09:41:00Z" {
		t.Fatalf("generated_at = %q", got.GeneratedAt)
	}
	if got.Overall != "ok" || got.Health != nil || got.AttentionMore != 0 {
		t.Fatalf("overall = %q, health = %+v, attention_more = %d; want ok, nil, 0", got.Overall, got.Health, got.AttentionMore)
	}
	if !reflect.DeepEqual(got.Unavailable, []string{"health"}) {
		t.Fatalf("unavailable = %v, want [health] (no metrics sample yet; nothing else is configured)", got.Unavailable)
	}
	if got.Attention == nil || len(got.Attention) != 0 || got.RecentActivity == nil || len(got.RecentActivity) != 0 {
		t.Fatalf("attention = %#v, recent_activity = %#v; want empty, non-nil lists", got.Attention, got.RecentActivity)
	}
	if got.LastActivityHours != -1 {
		t.Fatalf("last_activity_hours = %v, want -1", got.LastActivityHours)
	}
	if want := (dashboardSummaryMaintenance{LastStatus: "never"}); got.Maintenance != want {
		t.Fatalf("maintenance = %+v, want %+v", got.Maintenance, want)
	}
	if want := (dashboardSummaryKPIs{Budget: dashboardSummaryBudget{State: "ok"}}); got.KPIs != want {
		t.Fatalf("kpis = %+v, want %+v", got.KPIs, want)
	}
	if got.Badges != (dashboardSummaryBadges{}) || got.Agent.Mood != "" || got.Context.TotalChars != 0 {
		t.Fatalf("badges = %+v, mood = %q, context = %+v; want zeros", got.Badges, got.Agent.Mood, got.Context)
	}
}

func TestDashboardSummaryHealthComesFromSnapshot(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 41, 0, 0, time.UTC)
	s := &Server{Cfg: &config.Config{}, StartedAt: now.Add(-90 * time.Second)}
	s.storeSystemMetricsSnapshot(tools.SystemMetrics{
		Available: map[string]bool{"cpu": true, "memory": true, "disk": true},
		CPU:       tools.CPUMetrics{UsagePercent: 12},
		Memory:    tools.MemoryMetrics{UsedPercent: 50},
		Disk:      tools.DiskMetrics{UsedPercent: 98},
	}, now.Add(-5*time.Second))

	got := buildDashboardSummary(s, now)
	want := dashboardSummaryHealth{CPUPercent: 12, MemoryPercent: 50, DiskPercent: 98, UptimeSeconds: 90, SampledAt: "2026-10-17T09:40:55Z"}
	if got.Health == nil || *got.Health != want {
		t.Fatalf("health = %+v, want %+v", got.Health, want)
	}
	if len(got.Unavailable) != 0 {
		t.Fatalf("unavailable = %v, want none", got.Unavailable)
	}
	if got.Overall != attentionCritical || len(got.Attention) != 1 || got.Attention[0].Code != "disk_high" {
		t.Fatalf("overall = %q, attention = %+v; want one critical disk_high", got.Overall, got.Attention)
	}
}

func TestDashboardSummaryCountsFailedMissionsInLocal24Hours(t *testing.T) {
	db, err := tools.InitMissionHistoryDB(filepath.Join(t.TempDir(), "mission_history.db"))
	if err != nil {
		t.Fatalf("InitMissionHistoryDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Runs are stored in the server's local time and From is a string compare.
	// A fixed +02:00 zone makes the test independent of the machine's zone and
	// catches a UTC-formatted cutoff (it would also count the 25 h old run).
	zone := time.FixedZone("UTC+2", 2*60*60)
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, zone)
	for _, run := range []struct {
		id, status string
		age        time.Duration
	}{
		{"run_recent", "error", time.Hour},
		{"run_edge", "error", 23 * time.Hour},
		{"run_old", "error", 25 * time.Hour},
		{"run_ok", "success", 2 * time.Hour},
	} {
		if _, err := db.Exec(`INSERT INTO mission_history (id, mission_id, mission_name, trigger_type, status, error_msg, started_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			run.id, "mission-1", "Nightly report", "cron", run.status,
			"secret-mission-error in /srv/aurago/data", now.Add(-run.age).Format(time.RFC3339)); err != nil {
			t.Fatalf("insert %s: %v", run.id, err)
		}
	}

	got := buildDashboardSummary(&Server{Cfg: &config.Config{}, MissionHistoryDB: db}, now)
	if got.KPIs.Automations.MissionsFailed24h != 2 || got.Badges.Automations != 2 {
		t.Fatalf("missions_failed_24h = %d, badges.automations = %d; want 2 and 2",
			got.KPIs.Automations.MissionsFailed24h, got.Badges.Automations)
	}
	item, ok := findAttentionItem(got.Attention, "missions_failed")
	if !ok || item.Count != 2 || item.At != now.Add(-time.Hour).Format(time.RFC3339) {
		t.Fatalf("missions_failed item = %+v (found %v), want count 2 at the newest failed run", item, ok)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if strings.Contains(string(raw), "secret-mission-error") || strings.Contains(string(raw), "/srv/aurago") {
		t.Fatalf("summary leaks a mission error text: %s", raw)
	}
}

func TestDashboardSummaryMoodFollowsPersonalityEngine(t *testing.T) {
	stm := newDashboardSummaryTestMemory(t)
	s := &Server{Cfg: &config.Config{}, ShortTermMem: stm, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if got := buildDashboardSummary(s, time.Now()).Agent.Mood; got != "" {
		t.Fatalf("mood with the personality engine off = %q, want empty", got)
	}
	s.Cfg.Personality.Engine = true
	want, _ := s.buildPersonalityStatePayload()["mood"].(string)
	if got := buildDashboardSummary(s, time.Now()).Agent.Mood; got != want {
		t.Fatalf("mood = %q, want %q (the rule of buildPersonalityStatePayload)", got, want)
	}
}

func TestDashboardSummaryRecentActivityKeepsLatestEightWithoutFreeText(t *testing.T) {
	stm := newDashboardSummaryTestMemory(t)
	base := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		source := memory.AuditSourceAgentTool
		if i%2 == 0 {
			source = memory.AuditSourceMission
		}
		if _, err := stm.RecordAuditEvent(memory.AuditEvent{
			Timestamp:  base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			Source:     source,
			EventType:  "run",
			TargetName: fmt.Sprintf("target-%d", i),
			Status:     memory.AuditStatusSuccess,
			Summary:    fmt.Sprintf("summary-secret-%d", i),
			Detail:     fmt.Sprintf("detail-secret-%d", i),
			DurationMS: int64(i),
		}); err != nil {
			t.Fatalf("RecordAuditEvent(%d): %v", i, err)
		}
	}

	got := buildDashboardSummary(&Server{Cfg: &config.Config{}, ShortTermMem: stm}, time.Now())
	if len(got.RecentActivity) != 8 {
		t.Fatalf("recent_activity has %d rows, want 8", len(got.RecentActivity))
	}
	first, second, last := got.RecentActivity[0], got.RecentActivity[1], got.RecentActivity[7]
	if first.TargetName != "target-9" || first.Source != memory.AuditSourceAgentTool || first.Section != "audit" || first.DurationMS != 9 || first.ID == 0 {
		t.Fatalf("first row = %+v, want the newest agent_tool event in section audit", first)
	}
	if second.TargetName != "target-8" || second.Section != "automations" {
		t.Fatalf("second row = %+v, want the mission event in section automations", second)
	}
	if last.TargetName != "target-2" {
		t.Fatalf("last row = %+v, want target-2", last)
	}
	raw, err := json.Marshal(got.RecentActivity)
	if err != nil {
		t.Fatalf("marshal recent_activity: %v", err)
	}
	if strings.Contains(string(raw), "secret") {
		t.Fatalf("recent_activity leaks summary/detail: %s", raw)
	}
	for _, source := range got.Unavailable {
		if source == "audit" || source == "memory" {
			t.Fatalf("unavailable = %v, want audit and memory available", got.Unavailable)
		}
	}
}

func TestDashboardSummaryKnowledgeAndConflictCounts(t *testing.T) {
	kg, err := memory.NewKnowledgeGraph(":memory:", "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	for _, id := range []string{"router", "nas"} {
		if err := kg.AddNode(id, strings.ToUpper(id), map[string]string{"type": "device"}); err != nil {
			t.Fatalf("AddNode(%s): %v", id, err)
		}
	}
	stm := newDashboardSummaryTestMemory(t)
	for _, fact := range []string{"User prefers metric units", "Home server runs Debian"} {
		if _, err := stm.AddCoreMemoryFact(fact); err != nil {
			t.Fatalf("AddCoreMemoryFact: %v", err)
		}
	}
	for i, key := range []string{"home_city", "timezone"} {
		if err := stm.RegisterMemoryConflict(fmt.Sprintf("doc-a-%d", i), fmt.Sprintf("doc-b-%d", i), key, "left", "right", "contradiction"); err != nil {
			t.Fatalf("RegisterMemoryConflict(%s): %v", key, err)
		}
	}
	stats, err := kg.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	facts, err := stm.GetCoreMemoryCount()
	if err != nil {
		t.Fatalf("GetCoreMemoryCount: %v", err)
	}

	got := buildDashboardSummary(&Server{Cfg: &config.Config{}, KG: kg, ShortTermMem: stm}, time.Now())
	want := dashboardSummaryKnowledge{KGNodes: stats.TotalNodes, KGEdges: stats.TotalEdges, CoreFacts: facts}
	if got.KPIs.Knowledge != want || want.KGNodes < 2 || want.CoreFacts < 2 {
		t.Fatalf("knowledge = %+v, want %+v with at least 2 nodes and 2 core facts", got.KPIs.Knowledge, want)
	}
	item, ok := findAttentionItem(got.Attention, "memory_conflicts")
	if !ok || item.Count != 2 || item.Severity != attentionInfo || item.Section != "memory" {
		t.Fatalf("memory_conflicts item = %+v (found %v), want info count 2 in memory", item, ok)
	}
	if got.Overall != "ok" {
		t.Fatalf("overall = %q, want ok (info items do not change ok)", got.Overall)
	}
}

func TestDashboardSummaryBudgetKPIState(t *testing.T) {
	for _, tc := range []struct {
		status budget.BudgetStatus
		want   string
	}{
		{budget.BudgetStatus{Enabled: true, SpentUSD: 1, DailyLimit: 5, Percentage: 0.2}, "ok"},
		{budget.BudgetStatus{Enabled: true, SpentUSD: 4.25, DailyLimit: 5, Percentage: 0.85, IsWarning: true}, "warning"},
		{budget.BudgetStatus{Enabled: true, SpentUSD: 6, DailyLimit: 5, Percentage: 1, IsWarning: true, IsExceeded: true, IsBlocked: true}, "blocked"},
	} {
		got := dashboardSummaryBudgetKPI(tc.status)
		want := dashboardSummaryBudget{
			Enabled:       tc.status.Enabled,
			SpentUSD:      tc.status.SpentUSD,
			DailyLimitUSD: tc.status.DailyLimit,
			Percentage:    tc.status.Percentage,
			State:         tc.want,
		}
		if got != want {
			t.Fatalf("dashboardSummaryBudgetKPI(%+v) = %+v, want %+v", tc.status, got, want)
		}
	}
}
