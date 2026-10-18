package server

import (
	"math"
	"sort"
	"strings"
	"time"
)

// Severities of dashboard attention items, most urgent first.
const (
	attentionCritical = "critical"
	attentionWarning  = "warning"
	attentionInfo     = "info"
)

const (
	// dashboardAttentionLimit caps the "Needs attention" list; the rest is
	// reported as attention_more.
	dashboardAttentionLimit = 8

	attentionDiskWarnPercent     = 90.0
	attentionDiskCriticalPercent = 97.0
	attentionMemoryWarnPercent   = 95.0
)

// attentionItem is one row of the overview's "Needs attention" list. The text
// is never sent pre-translated: the client renders dashboard.attention.<code>
// with params. At is RFC3339 or empty.
type attentionItem struct {
	ID       string                 `json:"id"`
	Severity string                 `json:"severity"`
	Code     string                 `json:"code"`
	Params   map[string]interface{} `json:"params"`
	Section  string                 `json:"section"`
	Count    int                    `json:"count"`
	At       string                 `json:"at"`
}

// attentionOperationalIssue is the part of an active operational issue the
// rules may show. PublicID is planner.OperationalIssuePublicID of the
// fingerprint; the fingerprint itself never enters the rules. Title is the
// planner's sanitized title.
type attentionOperationalIssue struct {
	PublicID    string
	Severity    string
	Title       string
	Kind        string
	Source      string
	Occurrences int
	LastSeen    string
}

// attentionInputs is everything buildDashboardAttention reads. The summary
// fills it from the server's stores; zero values mean "nothing to report"
// (source not configured, unavailable or healthy).
type attentionInputs struct {
	OperationalIssues      []attentionOperationalIssue // active issues, planner order, at most 20
	OperationalIssuesTotal int                         // all active issues (problems badge)

	CronErrorCount   int
	CronFirstErrorID string

	MissionsFailed24h    int
	MissionsLastFailedAt time.Time

	DaemonsFailed         int
	DaemonFirstFailedName string

	BudgetEnabled    bool
	BudgetWarning    bool
	BudgetBlocked    bool
	BudgetPercentage float64 // 0..1, as budget.BudgetStatus.Percentage

	HasHealth bool
	Health    systemMetricsSnapshot

	IndexerPresent    bool
	IndexerErrorCount int

	MemoryConflicts int

	LocalWikiUpdateAvailable bool
	LocalWikiUpdateDate      string
}

// dashboardSummaryBadges are the sidebar counters of the problems and
// automations sections.
type dashboardSummaryBadges struct {
	Problems    int `json:"problems"`
	Automations int `json:"automations"`
}

// dashboardAttentionRules are the rules of spec §8.2; each returns zero or more items.
var dashboardAttentionRules = []func(attentionInputs) []attentionItem{
	attentionOperationalIssues,
	attentionCronErrors,
	attentionMissionsFailed,
	attentionDaemonFailed,
	attentionBudget,
	attentionDiskHigh,
	attentionMemoryHigh,
	attentionIndexerError,
	attentionMemoryConflicts,
	attentionLocalWikiUpdate,
}

// buildDashboardAttention evaluates every rule, orders the items by severity,
// then at (newest first, empty last), then code, and keeps the first
// dashboardAttentionLimit. more is the number of items cut off.
func buildDashboardAttention(in attentionInputs) (items []attentionItem, more int) {
	all := make([]attentionItem, 0, len(in.OperationalIssues)+len(dashboardAttentionRules))
	for _, rule := range dashboardAttentionRules {
		all = append(all, rule(in)...)
	}
	sortAttentionItems(all)
	if len(all) > dashboardAttentionLimit {
		return all[:dashboardAttentionLimit], len(all) - dashboardAttentionLimit
	}
	return all, 0
}

// dashboardOverallState is critical if any item is critical, warning if any is
// a warning, else ok; info items never change ok. The list is sorted by
// severity, so the kept items decide the same way as the full list.
func dashboardOverallState(items []attentionItem) string {
	overall := "ok"
	for _, item := range items {
		switch item.Severity {
		case attentionCritical:
			return attentionCritical
		case attentionWarning:
			overall = attentionWarning
		}
	}
	return overall
}

// dashboardAttentionBadges: problems = all active operational issues;
// automations = cron errors + failed missions (24 h) + failed daemons.
func dashboardAttentionBadges(in attentionInputs) dashboardSummaryBadges {
	return dashboardSummaryBadges{
		Problems:    in.OperationalIssuesTotal,
		Automations: in.CronErrorCount + in.MissionsFailed24h + in.DaemonsFailed,
	}
}

// sortAttentionItems orders by severity (critical, warning, info), then at
// descending with empty or unparsable at last, then code. The sort is stable,
// so full ties keep rule order (operational issues keep planner order).
func sortAttentionItems(items []attentionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if ra, rb := attentionSeverityRank(a.Severity), attentionSeverityRank(b.Severity); ra != rb {
			return ra < rb
		}
		ta, okA := parseAttentionTime(a.At)
		tb, okB := parseAttentionTime(b.At)
		if okA != okB {
			return okA
		}
		if okA && !ta.Equal(tb) {
			return ta.After(tb)
		}
		return a.Code < b.Code
	})
}

func attentionSeverityRank(severity string) int {
	switch severity {
	case attentionCritical:
		return 0
	case attentionWarning:
		return 1
	default:
		return 2
	}
}

func parseAttentionTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func formatAttentionTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func attentionPercent(value float64) int {
	return int(math.Round(value))
}

// attentionOperationalIssueSeverity maps the planner's stored severities.
func attentionOperationalIssueSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "error", "high":
		return attentionCritical
	case "warning", "warn", "medium":
		return attentionWarning
	default:
		return attentionInfo
	}
}

// attentionOperationalIssues: one item per active issue. The sanitized title
// is the only text shown untranslated (spec §8.1).
func attentionOperationalIssues(in attentionInputs) []attentionItem {
	items := make([]attentionItem, 0, len(in.OperationalIssues))
	for _, issue := range in.OperationalIssues {
		items = append(items, attentionItem{
			ID:       "operational_issue:" + issue.PublicID,
			Severity: attentionOperationalIssueSeverity(issue.Severity),
			Code:     "operational_issue",
			Params: map[string]interface{}{
				"title":       issue.Title,
				"kind":        issue.Kind,
				"source":      issue.Source,
				"occurrences": issue.Occurrences,
			},
			Section: "problems",
			Count:   1,
			At:      issue.LastSeen,
		})
	}
	return items
}

func attentionCronErrors(in attentionInputs) []attentionItem {
	if in.CronErrorCount <= 0 {
		return nil
	}
	return []attentionItem{{
		ID:       "cron_errors",
		Severity: attentionWarning,
		Code:     "cron_errors",
		Params:   map[string]interface{}{"first_id": in.CronFirstErrorID},
		Section:  "automations",
		Count:    in.CronErrorCount,
	}}
}

func attentionMissionsFailed(in attentionInputs) []attentionItem {
	if in.MissionsFailed24h <= 0 {
		return nil
	}
	return []attentionItem{{
		ID:       "missions_failed",
		Severity: attentionWarning,
		Code:     "missions_failed",
		Params:   map[string]interface{}{},
		Section:  "automations",
		Count:    in.MissionsFailed24h,
		At:       formatAttentionTime(in.MissionsLastFailedAt),
	}}
}

func attentionDaemonFailed(in attentionInputs) []attentionItem {
	if in.DaemonsFailed <= 0 {
		return nil
	}
	return []attentionItem{{
		ID:       "daemon_failed",
		Severity: attentionWarning,
		Code:     "daemon_failed",
		Params:   map[string]interface{}{"first_name": in.DaemonFirstFailedName},
		Section:  "automations",
		Count:    in.DaemonsFailed,
	}}
}

func attentionBudget(in attentionInputs) []attentionItem {
	if !in.BudgetEnabled {
		return nil
	}
	item := attentionItem{
		Params:  map[string]interface{}{"percent": attentionPercent(in.BudgetPercentage * 100)},
		Section: "llm",
		Count:   1,
	}
	switch {
	case in.BudgetBlocked:
		item.ID, item.Code, item.Severity = "budget_blocked", "budget_blocked", attentionCritical
	case in.BudgetWarning:
		item.ID, item.Code, item.Severity = "budget_warning", "budget_warning", attentionWarning
	default:
		return nil
	}
	return []attentionItem{item}
}

func attentionDiskHigh(in attentionInputs) []attentionItem {
	if !in.HasHealth || !in.Health.DiskAvailable || in.Health.DiskPercent < attentionDiskWarnPercent {
		return nil
	}
	severity := attentionWarning
	if in.Health.DiskPercent >= attentionDiskCriticalPercent {
		severity = attentionCritical
	}
	return []attentionItem{{
		ID:       "disk_high",
		Severity: severity,
		Code:     "disk_high",
		Params:   map[string]interface{}{"percent": attentionPercent(in.Health.DiskPercent)},
		Section:  "system",
		Count:    1,
		At:       formatAttentionTime(in.Health.SampledAt),
	}}
}

func attentionMemoryHigh(in attentionInputs) []attentionItem {
	if !in.HasHealth || !in.Health.MemoryAvailable || in.Health.MemoryPercent < attentionMemoryWarnPercent {
		return nil
	}
	return []attentionItem{{
		ID:       "memory_high",
		Severity: attentionWarning,
		Code:     "memory_high",
		Params:   map[string]interface{}{"percent": attentionPercent(in.Health.MemoryPercent)},
		Section:  "system",
		Count:    1,
		At:       formatAttentionTime(in.Health.SampledAt),
	}}
}

// attentionIndexerError counts the indexer's error list, which includes
// "embedding pipeline unavailable" (indexing on but unable to work). The texts
// may hold paths and stay server-side.
func attentionIndexerError(in attentionInputs) []attentionItem {
	if !in.IndexerPresent || in.IndexerErrorCount <= 0 {
		return nil
	}
	return []attentionItem{{
		ID:       "indexer_error",
		Severity: attentionWarning,
		Code:     "indexer_error",
		Params:   map[string]interface{}{},
		Section:  "filesync",
		Count:    in.IndexerErrorCount,
	}}
}

func attentionMemoryConflicts(in attentionInputs) []attentionItem {
	if in.MemoryConflicts <= 0 {
		return nil
	}
	return []attentionItem{{
		ID:       "memory_conflicts",
		Severity: attentionInfo,
		Code:     "memory_conflicts",
		Params:   map[string]interface{}{},
		Section:  "memory",
		Count:    in.MemoryConflicts,
	}}
}

func attentionLocalWikiUpdate(in attentionInputs) []attentionItem {
	if !in.LocalWikiUpdateAvailable {
		return nil
	}
	return []attentionItem{{
		ID:       "localwiki_update",
		Severity: attentionInfo,
		Code:     "localwiki_update",
		Params:   map[string]interface{}{"date": in.LocalWikiUpdateDate},
		Section:  "system",
		Count:    1,
	}}
}
