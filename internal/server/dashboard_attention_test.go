package server

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func findAttentionItem(items []attentionItem, code string) (attentionItem, bool) {
	for _, item := range items {
		if item.Code == code {
			return item, true
		}
	}
	return attentionItem{}, false
}

func attentionIDs(items []attentionItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestDashboardAttentionOperationalIssueSeverityMapping(t *testing.T) {
	for _, tc := range []struct {
		severity string
		want     string
	}{
		{"critical", attentionCritical},
		{"error", attentionCritical},
		{"high", attentionCritical},
		{" HIGH ", attentionCritical},
		{"warning", attentionWarning},
		{"warn", attentionWarning},
		{"medium", attentionWarning},
		{"info", attentionInfo},
		{"low", attentionInfo},
		{"", attentionInfo},
	} {
		items, more := buildDashboardAttention(attentionInputs{OperationalIssues: []attentionOperationalIssue{{
			PublicID:    "abc123",
			Severity:    tc.severity,
			Title:       "Tool failed",
			Kind:        "tool_failure",
			Source:      "heartbeat",
			Occurrences: 3,
			LastSeen:    "2026-10-17T09:00:00Z",
		}}})
		want := []attentionItem{{
			ID:       "operational_issue:abc123",
			Severity: tc.want,
			Code:     "operational_issue",
			Params: map[string]interface{}{
				"title":       "Tool failed",
				"kind":        "tool_failure",
				"source":      "heartbeat",
				"occurrences": 3,
			},
			Section: "problems",
			Count:   1,
			At:      "2026-10-17T09:00:00Z",
		}}
		if more != 0 || !reflect.DeepEqual(items, want) {
			t.Fatalf("severity %q: items = %#v, more = %d; want %#v", tc.severity, items, more, want)
		}
	}
}

func TestDashboardAttentionRules(t *testing.T) {
	sampled := time.Date(2026, 10, 17, 9, 41, 0, 0, time.UTC)
	failedAt := time.Date(2026, 10, 17, 8, 15, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	health := func(mem, disk float64, memOK, diskOK bool) systemMetricsSnapshot {
		return systemMetricsSnapshot{
			CPUPercent:      10,
			MemoryPercent:   mem,
			DiskPercent:     disk,
			CPUAvailable:    true,
			MemoryAvailable: memOK,
			DiskAvailable:   diskOK,
			SampledAt:       sampled,
		}
	}
	none := []attentionItem{}
	for _, tc := range []struct {
		name string
		in   attentionInputs
		want []attentionItem
	}{
		{name: "nothing to report", in: attentionInputs{}, want: none},
		{
			name: "cron errors are aggregated",
			in:   attentionInputs{CronErrorCount: 2, CronFirstErrorID: "nightly-report"},
			want: []attentionItem{{ID: "cron_errors", Severity: attentionWarning, Code: "cron_errors",
				Params: map[string]interface{}{"first_id": "nightly-report"}, Section: "automations", Count: 2}},
		},
		{
			name: "failed missions are aggregated",
			in:   attentionInputs{MissionsFailed24h: 3, MissionsLastFailedAt: failedAt},
			want: []attentionItem{{ID: "missions_failed", Severity: attentionWarning, Code: "missions_failed",
				Params: map[string]interface{}{}, Section: "automations", Count: 3, At: "2026-10-17T08:15:00+02:00"}},
		},
		{
			name: "failed daemons are aggregated",
			in:   attentionInputs{DaemonsFailed: 1, DaemonFirstFailedName: "Mail watcher"},
			want: []attentionItem{{ID: "daemon_failed", Severity: attentionWarning, Code: "daemon_failed",
				Params: map[string]interface{}{"first_name": "Mail watcher"}, Section: "automations", Count: 1}},
		},
		{
			name: "budget warning",
			in:   attentionInputs{BudgetEnabled: true, BudgetWarning: true, BudgetPercentage: 0.834},
			want: []attentionItem{{ID: "budget_warning", Severity: attentionWarning, Code: "budget_warning",
				Params: map[string]interface{}{"percent": 83}, Section: "llm", Count: 1}},
		},
		{
			name: "budget blocked wins over warning",
			in:   attentionInputs{BudgetEnabled: true, BudgetWarning: true, BudgetBlocked: true, BudgetPercentage: 1},
			want: []attentionItem{{ID: "budget_blocked", Severity: attentionCritical, Code: "budget_blocked",
				Params: map[string]interface{}{"percent": 100}, Section: "llm", Count: 1}},
		},
		{name: "budget flags ignored when tracking is off", in: attentionInputs{BudgetWarning: true, BudgetBlocked: true}, want: none},
		{name: "disk below 90 percent", in: attentionInputs{HasHealth: true, Health: health(50, 89.9, true, true)}, want: none},
		{
			name: "disk at 90 percent warns",
			in:   attentionInputs{HasHealth: true, Health: health(50, 90, true, true)},
			want: []attentionItem{{ID: "disk_high", Severity: attentionWarning, Code: "disk_high",
				Params: map[string]interface{}{"percent": 90}, Section: "system", Count: 1, At: "2026-10-17T09:41:00Z"}},
		},
		{
			name: "disk at 97 percent is critical",
			in:   attentionInputs{HasHealth: true, Health: health(50, 97.2, true, true)},
			want: []attentionItem{{ID: "disk_high", Severity: attentionCritical, Code: "disk_high",
				Params: map[string]interface{}{"percent": 97}, Section: "system", Count: 1, At: "2026-10-17T09:41:00Z"}},
		},
		{name: "disk ignored when not sampled", in: attentionInputs{HasHealth: true, Health: health(50, 99, true, false)}, want: none},
		{name: "no health snapshot yet", in: attentionInputs{Health: health(99, 99, true, true)}, want: none},
		{name: "memory below 95 percent", in: attentionInputs{HasHealth: true, Health: health(94.9, 10, true, true)}, want: none},
		{
			name: "memory at 95 percent warns",
			in:   attentionInputs{HasHealth: true, Health: health(95.4, 10, true, true)},
			want: []attentionItem{{ID: "memory_high", Severity: attentionWarning, Code: "memory_high",
				Params: map[string]interface{}{"percent": 95}, Section: "system", Count: 1, At: "2026-10-17T09:41:00Z"}},
		},
		{name: "memory ignored when not sampled", in: attentionInputs{HasHealth: true, Health: health(99, 10, false, true)}, want: none},
		{
			name: "indexer errors",
			in:   attentionInputs{IndexerPresent: true, IndexerErrorCount: 2},
			want: []attentionItem{{ID: "indexer_error", Severity: attentionWarning, Code: "indexer_error",
				Params: map[string]interface{}{}, Section: "filesync", Count: 2}},
		},
		{name: "indexer without errors", in: attentionInputs{IndexerPresent: true}, want: none},
		{name: "indexer absent", in: attentionInputs{IndexerErrorCount: 2}, want: none},
		{
			name: "open memory conflicts",
			in:   attentionInputs{MemoryConflicts: 4},
			want: []attentionItem{{ID: "memory_conflicts", Severity: attentionInfo, Code: "memory_conflicts",
				Params: map[string]interface{}{}, Section: "memory", Count: 4}},
		},
		{
			name: "local wikipedia update",
			in:   attentionInputs{LocalWikiUpdateAvailable: true, LocalWikiUpdateDate: "2026-10"},
			want: []attentionItem{{ID: "localwiki_update", Severity: attentionInfo, Code: "localwiki_update",
				Params: map[string]interface{}{"date": "2026-10"}, Section: "system", Count: 1}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, more := buildDashboardAttention(tc.in)
			if more != 0 || !reflect.DeepEqual(items, tc.want) {
				t.Fatalf("items = %#v, more = %d; want %#v", items, more, tc.want)
			}
		})
	}
}

func TestDashboardAttentionOrdering(t *testing.T) {
	in := attentionInputs{
		OperationalIssues: []attentionOperationalIssue{
			{PublicID: "a", Severity: "error", LastSeen: "2026-10-17T08:00:00Z"},
			{PublicID: "b", Severity: "critical", LastSeen: "2026-10-17T09:00:00Z"},
			{PublicID: "c", Severity: "low", LastSeen: "2026-10-17T06:00:00Z"},
		},
		BudgetEnabled:        true,
		BudgetBlocked:        true,
		BudgetPercentage:     1,
		CronErrorCount:       1,
		CronFirstErrorID:     "nightly-report",
		MissionsFailed24h:    1,
		MissionsLastFailedAt: time.Date(2026, 10, 17, 7, 0, 0, 0, time.UTC),
		HasHealth:            true,
		Health: systemMetricsSnapshot{
			DiskPercent:     92,
			DiskAvailable:   true,
			MemoryAvailable: true,
			SampledAt:       time.Date(2026, 10, 17, 9, 30, 0, 0, time.UTC),
		},
		MemoryConflicts:          1,
		LocalWikiUpdateAvailable: true,
		LocalWikiUpdateDate:      "2026-10",
	}
	items, more := buildDashboardAttention(in)
	want := []string{
		"operational_issue:b", "operational_issue:a", "budget_blocked", // critical: newest first, empty at last
		"disk_high", "missions_failed", "cron_errors", // warning
		"operational_issue:c", "localwiki_update", // info: timed first, then by code
	}
	if got := attentionIDs(items); !reflect.DeepEqual(got, want) || more != 1 {
		t.Fatalf("order = %v, more = %d; want %v, 1", got, more, want)
	}
	if _, ok := findAttentionItem(items, "memory_conflicts"); ok {
		t.Fatal("memory_conflicts is the 9th item and must be cut off")
	}
}

func TestDashboardAttentionCapsAtEight(t *testing.T) {
	issues := func(n int) []attentionOperationalIssue {
		out := make([]attentionOperationalIssue, n)
		for i := range out {
			out[i] = attentionOperationalIssue{
				PublicID: fmt.Sprintf("issue-%02d", i),
				Severity: "warning",
				LastSeen: time.Date(2026, 10, 17, 9, i, 0, 0, time.UTC).Format(time.RFC3339),
			}
		}
		return out
	}
	items, more := buildDashboardAttention(attentionInputs{OperationalIssues: issues(8)})
	if len(items) != 8 || more != 0 {
		t.Fatalf("8 issues: len = %d, more = %d; want 8, 0", len(items), more)
	}
	items, more = buildDashboardAttention(attentionInputs{OperationalIssues: issues(10)})
	if len(items) != 8 || more != 2 {
		t.Fatalf("10 issues: len = %d, more = %d; want 8, 2", len(items), more)
	}
	if items[0].ID != "operational_issue:issue-09" || items[7].ID != "operational_issue:issue-02" {
		t.Fatalf("kept %v, want the 8 newest (issue-09 down to issue-02)", attentionIDs(items))
	}
}

func TestDashboardAttentionOverall(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []attentionItem
		want  string
	}{
		{"empty", nil, "ok"},
		{"info only", []attentionItem{{Severity: attentionInfo}}, "ok"},
		{"warning", []attentionItem{{Severity: attentionInfo}, {Severity: attentionWarning}}, "warning"},
		{"critical", []attentionItem{{Severity: attentionWarning}, {Severity: attentionCritical}}, "critical"},
	} {
		if got := dashboardOverallState(tc.items); got != tc.want {
			t.Fatalf("%s: overall = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDashboardAttentionBadges(t *testing.T) {
	got := dashboardAttentionBadges(attentionInputs{
		OperationalIssues:      make([]attentionOperationalIssue, 20),
		OperationalIssuesTotal: 27,
		CronErrorCount:         2,
		MissionsFailed24h:      3,
		DaemonsFailed:          1,
	})
	if want := (dashboardSummaryBadges{Problems: 27, Automations: 6}); got != want {
		t.Fatalf("badges = %+v, want %+v", got, want)
	}
}
