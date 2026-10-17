package server

import (
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/prompts"
	"aurago/internal/tools"
)

const (
	dashboardSummaryIssueLimit    = 20 // active operational issues fed to the rules
	dashboardSummaryConflictLimit = 50 // open memory conflicts counted
	dashboardSummaryActivityLimit = 8  // audit events in recent_activity
)

// dashboardSummaryResponse is the body of GET /api/dashboard/summary (spec
// §8.1). agent, context, last_activity_hours and maintenance use the keys of
// /api/dashboard/overview that the agent banner reads.
type dashboardSummaryResponse struct {
	GeneratedAt       string                      `json:"generated_at"`
	Overall           string                      `json:"overall"`
	Agent             dashboardSummaryAgent       `json:"agent"`
	Context           dashboardSummaryContext     `json:"context"`
	LastActivityHours float64                     `json:"last_activity_hours"`
	Maintenance       dashboardSummaryMaintenance `json:"maintenance"`
	Health            *dashboardSummaryHealth     `json:"health"`
	Attention         []attentionItem             `json:"attention"`
	AttentionMore     int                         `json:"attention_more"`
	Badges            dashboardSummaryBadges      `json:"badges"`
	KPIs              dashboardSummaryKPIs        `json:"kpis"`
	RecentActivity    []dashboardSummaryActivity  `json:"recent_activity"`
	Unavailable       []string                    `json:"unavailable"`
}

type dashboardSummaryAgent struct {
	Model         string `json:"model"`
	Provider      string `json:"provider"`
	Personality   string `json:"personality"`
	ContextWindow int    `json:"context_window"`
	Busy          bool   `json:"busy"`
	Debug         bool   `json:"debug"`
	Maintenance   bool   `json:"maintenance"`
	Mood          string `json:"mood"`
}

type dashboardSummaryContext struct {
	TotalChars int `json:"total_chars"`
}

// dashboardSummaryMaintenance keeps only the four banner keys; the overview's
// phase_results (error texts, integration check details) stay server-side.
type dashboardSummaryMaintenance struct {
	Enabled    bool   `json:"enabled"`
	LastRun    string `json:"last_run"`
	LastStatus string `json:"last_status"`
	NextRun    string `json:"next_run"`
}

type dashboardSummaryHealth struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskPercent   float64 `json:"disk_percent"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	SampledAt     string  `json:"sampled_at"`
}

type dashboardSummaryKPIs struct {
	Budget      dashboardSummaryBudget      `json:"budget"`
	Automations dashboardSummaryAutomations `json:"automations"`
	Knowledge   dashboardSummaryKnowledge   `json:"knowledge"`
}

// dashboardSummaryBudget amounts are USD (the tracker has no currency field);
// Percentage is the tracker's 0..1 fraction.
type dashboardSummaryBudget struct {
	Enabled       bool    `json:"enabled"`
	SpentUSD      float64 `json:"spent_usd"`
	DailyLimitUSD float64 `json:"daily_limit_usd"`
	Percentage    float64 `json:"percentage"`
	State         string  `json:"state"`
}

type dashboardSummaryAutomations struct {
	MissionsRunning   int `json:"missions_running"`
	MissionsQueued    int `json:"missions_queued"`
	MissionsFailed24h int `json:"missions_failed_24h"`
	CronErrors        int `json:"cron_errors"`
	DaemonsFailed     int `json:"daemons_failed"`
}

type dashboardSummaryKnowledge struct {
	KGNodes   int `json:"kg_nodes"`
	KGEdges   int `json:"kg_edges"`
	CoreFacts int `json:"core_facts"`
}

// dashboardSummaryActivity is one audit event without its free-text summary
// and detail.
type dashboardSummaryActivity struct {
	ID         int64  `json:"id"`
	Timestamp  string `json:"timestamp"`
	Source     string `json:"source"`
	EventType  string `json:"event_type"`
	TargetName string `json:"target_name"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Section    string `json:"section"`
}

// dashboardSummaryCollector reads the sources one after another and records
// which ones failed. Errors go to the server log only; the response carries
// just the fixed source name.
type dashboardSummaryCollector struct {
	s           *Server
	unavailable []string
}

func (c *dashboardSummaryCollector) markUnavailable(source string) {
	for _, existing := range c.unavailable {
		if existing == source {
			return
		}
	}
	c.unavailable = append(c.unavailable, source)
}

func (c *dashboardSummaryCollector) sourceFailed(source string, err error) {
	if c.s != nil && c.s.Logger != nil {
		c.s.Logger.Warn("[Dashboard] Summary source unavailable", "source", source, "error", err)
	}
	c.markUnavailable(source)
}

// buildDashboardSummary gathers the summary from the server's local stores. A
// source that is not configured reports zeros; a source that fails is skipped
// and listed in Unavailable. It never fails as a whole.
func buildDashboardSummary(s *Server, now time.Time) dashboardSummaryResponse {
	if s == nil {
		s = &Server{}
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		cfg = &config.Config{}
	}
	c := &dashboardSummaryCollector{s: s, unavailable: []string{}}
	var in attentionInputs

	resp := dashboardSummaryResponse{
		GeneratedAt: now.Format(time.RFC3339),
		Agent:       dashboardSummaryAgentBlock(s, cfg, now),
		Maintenance: dashboardSummaryMaintenanceFrom(buildMaintenanceStatusSummary(s, cfg)),
	}
	if s.HistoryManager != nil {
		resp.Context.TotalChars = s.HistoryManager.TotalChars()
	}

	resp.Health = c.collectHealth(&in, now)
	c.collectOperationalIssues(&in)
	resp.KPIs.Automations = c.collectAutomations(&in, now)

	// GetStatus is nil-safe and has no error path, so budget never becomes unavailable.
	budgetStatus := s.BudgetTracker.GetStatus()
	in.BudgetEnabled = budgetStatus.Enabled
	in.BudgetWarning = budgetStatus.IsWarning
	in.BudgetBlocked = budgetStatus.IsBlocked
	in.BudgetPercentage = budgetStatus.Percentage
	resp.KPIs.Budget = dashboardSummaryBudgetKPI(budgetStatus)

	if s.FileIndexer != nil {
		in.IndexerPresent = true
		in.IndexerErrorCount = len(s.FileIndexer.Status().Errors)
	}
	resp.KPIs.Knowledge = c.collectKnowledge(&in)
	resp.LastActivityHours = c.collectLastActivity()
	resp.RecentActivity = c.collectRecentActivity()
	in.LocalWikiUpdateDate, in.LocalWikiUpdateAvailable = dashboardSummaryLocalWikiUpdate(s, cfg)

	resp.Attention, resp.AttentionMore = buildDashboardAttention(in)
	resp.Overall = dashboardOverallState(resp.Attention)
	resp.Badges = dashboardAttentionBadges(in)
	resp.Unavailable = c.unavailable
	return resp
}

func dashboardSummaryAgentBlock(s *Server, cfg *config.Config, now time.Time) dashboardSummaryAgent {
	personality, _ := prompts.ResolvePersonalityID(cfg.Personality.CorePersonality)
	return dashboardSummaryAgent{
		Model:         cfg.LLM.Model,
		Provider:      cfg.LLM.ProviderType,
		Personality:   personality,
		ContextWindow: cfg.Agent.ContextWindow,
		Busy:          tools.IsBusy(),
		Debug:         agent.GetDebugMode(),
		Maintenance:   cfg.Maintenance.Enabled,
		Mood:          dashboardSummaryMood(s, cfg, now),
	}
}

// dashboardSummaryMood follows buildPersonalityStatePayload: empty unless the
// personality engine is on; the current mood, replaced by the affect mood of
// the personality snapshot when that read succeeds.
func dashboardSummaryMood(s *Server, cfg *config.Config, now time.Time) string {
	if cfg == nil || !cfg.Personality.Engine || s == nil || s.ShortTermMem == nil {
		return ""
	}
	mood := string(s.ShortTermMem.GetCurrentMood())
	if snapshot, err := s.ShortTermMem.GetPersonalitySnapshotAt(now); err == nil {
		mood = string(snapshot.Affect.Mood)
	}
	return mood
}

func dashboardSummaryMaintenanceFrom(summary map[string]interface{}) dashboardSummaryMaintenance {
	out := dashboardSummaryMaintenance{LastStatus: "never"}
	out.Enabled, _ = summary["enabled"].(bool)
	out.LastRun, _ = summary["last_run"].(string)
	out.NextRun, _ = summary["next_run"].(string)
	if status, ok := summary["last_status"].(string); ok && status != "" {
		out.LastStatus = status
	}
	return out
}

// dashboardSummaryBudgetKPI: blocked if is_blocked, else warning if
// is_warning, else ok (also when tracking is off; enabled tells the client).
func dashboardSummaryBudgetKPI(status budget.BudgetStatus) dashboardSummaryBudget {
	state := "ok"
	switch {
	case status.IsBlocked:
		state = "blocked"
	case status.IsWarning:
		state = "warning"
	}
	return dashboardSummaryBudget{
		Enabled:       status.Enabled,
		SpentUSD:      status.SpentUSD,
		DailyLimitUSD: status.DailyLimit,
		Percentage:    status.Percentage,
		State:         state,
	}
}

func dashboardSummaryLocalWikiUpdate(s *Server, cfg *config.Config) (string, bool) {
	summary := dashboardLocalWikipediaSummary(cfg, s.dashboardLocalWikipediaStatusSource())
	update, ok := summary["update_available"].(map[string]interface{})
	if !ok {
		return "", false
	}
	date, _ := update["date"].(string)
	return date, true
}

// collectHealth reads the collector's snapshot; before the first tick health
// is null and listed as unavailable (that is expected, so nothing is logged).
func (c *dashboardSummaryCollector) collectHealth(in *attentionInputs, now time.Time) *dashboardSummaryHealth {
	snap, ok := c.s.loadSystemMetricsSnapshot()
	if !ok {
		c.markUnavailable("health")
		return nil
	}
	in.Health, in.HasHealth = snap, true
	var uptime int64
	if !c.s.StartedAt.IsZero() {
		uptime = int64(now.Sub(c.s.StartedAt).Seconds())
	}
	return &dashboardSummaryHealth{
		CPUPercent:    snap.CPUPercent,
		MemoryPercent: snap.MemoryPercent,
		DiskPercent:   snap.DiskPercent,
		UptimeSeconds: uptime,
		SampledAt:     snap.SampledAt.Format(time.RFC3339),
	}
}

// collectOperationalIssues maps the planner's sanitized list items. The
// fingerprint only feeds OperationalIssuePublicID; detail stays server-side.
func (c *dashboardSummaryCollector) collectOperationalIssues(in *attentionInputs) {
	if c.s.PlannerDB == nil {
		return
	}
	page, err := planner.ListOperationalIssues(c.s.PlannerDB, planner.OperationalIssueListFilter{
		Status: "active",
		Limit:  dashboardSummaryIssueLimit,
	})
	if err != nil {
		c.sourceFailed("operational_issues", err)
		return
	}
	in.OperationalIssuesTotal = page.Total
	in.OperationalIssues = make([]attentionOperationalIssue, 0, len(page.Items))
	for _, item := range page.Items {
		in.OperationalIssues = append(in.OperationalIssues, attentionOperationalIssue{
			PublicID:    planner.OperationalIssuePublicID(item.Fingerprint),
			Severity:    item.Severity,
			Title:       item.Title,
			Kind:        item.Kind,
			Source:      item.Source,
			Occurrences: item.Occurrences,
			LastSeen:    item.LastSeen,
		})
	}
}

// collectAutomations fills the automations KPIs and the cron, mission and
// daemon rule inputs. Only counts, the first cron job id and the first daemon
// display name are kept; last_error texts stay server-side.
func (c *dashboardSummaryCollector) collectAutomations(in *attentionInputs, now time.Time) dashboardSummaryAutomations {
	var out dashboardSummaryAutomations
	if mm := c.s.MissionManagerV2; mm != nil {
		for _, m := range mm.List() {
			if m.Status == "running" {
				out.MissionsRunning++
			}
		}
		queue, runningID := mm.GetQueue()
		if queue != nil {
			out.MissionsQueued = len(queue.List())
		}
		if runningID != "" {
			out.MissionsRunning = 1
		}
	}
	if db := c.s.MissionHistoryDB; db != nil {
		// Runs are stored with time.Now().Format(time.RFC3339) (local time) and
		// From is a string comparison, so the cutoff keeps now's location.
		cutoff := now.Add(-24 * time.Hour).Format(time.RFC3339)
		page, err := tools.QueryMissionHistory(db, tools.MissionHistoryFilter{Result: "error", From: cutoff, Limit: 1})
		if err != nil {
			c.sourceFailed("missions", err)
		} else {
			in.MissionsFailed24h = page.Total
			if len(page.Entries) > 0 && page.Entries[0] != nil {
				in.MissionsLastFailedAt = page.Entries[0].StartedAt
			}
		}
	}
	if cm := c.s.CronManager; cm != nil {
		for _, job := range dashboardCronjobsFromTools(cm.GetJobsWithRuntimeStatus()) {
			if job.Status != "error" {
				continue
			}
			if in.CronErrorCount == 0 {
				in.CronFirstErrorID = job.ID
			}
			in.CronErrorCount++
		}
	}
	if ds := c.s.DaemonSupervisor; ds != nil {
		for _, st := range ds.ListDaemons() {
			if !dashboardDaemonFailed(st) {
				continue
			}
			if in.DaemonsFailed == 0 {
				in.DaemonFirstFailedName = st.SkillName
			}
			in.DaemonsFailed++
		}
	}
	out.MissionsFailed24h = in.MissionsFailed24h
	out.CronErrors = in.CronErrorCount
	out.DaemonsFailed = in.DaemonsFailed
	return out
}

// dashboardDaemonFailed: crashed, or disabled by the supervisor itself.
func dashboardDaemonFailed(st tools.DaemonState) bool {
	return st.Status == tools.DaemonCrashed || (st.Status == tools.DaemonDisabled && st.AutoDisabled)
}

func (c *dashboardSummaryCollector) collectKnowledge(in *attentionInputs) dashboardSummaryKnowledge {
	var out dashboardSummaryKnowledge
	if c.s.KG != nil {
		stats, err := c.s.KG.GetStats()
		if err != nil {
			c.sourceFailed("knowledge", err)
		} else if stats != nil {
			out.KGNodes, out.KGEdges = stats.TotalNodes, stats.TotalEdges
		}
	}
	if stm := c.s.ShortTermMem; stm != nil {
		if count, err := stm.GetCoreMemoryCount(); err != nil {
			c.sourceFailed("memory", err)
		} else {
			out.CoreFacts = count
		}
		if conflicts, err := stm.GetOpenMemoryConflicts(dashboardSummaryConflictLimit); err != nil {
			c.sourceFailed("memory", err)
		} else {
			in.MemoryConflicts = len(conflicts)
		}
	}
	return out
}

func (c *dashboardSummaryCollector) collectLastActivity() float64 {
	if c.s.ShortTermMem == nil {
		return -1
	}
	hours, err := c.s.ShortTermMem.GetHoursSinceLastUserMessage("")
	if err != nil {
		c.sourceFailed("memory", err)
		return -1
	}
	return hours
}

// collectRecentActivity lists the latest audit events; summary and detail are
// free text and are not copied.
func (c *dashboardSummaryCollector) collectRecentActivity() []dashboardSummaryActivity {
	out := []dashboardSummaryActivity{}
	if c.s.ShortTermMem == nil {
		return out
	}
	page, err := c.s.ShortTermMem.SearchAuditEvents(memory.AuditFilter{Limit: dashboardSummaryActivityLimit})
	if err != nil {
		c.sourceFailed("audit", err)
		return out
	}
	if page == nil {
		return out
	}
	for _, event := range page.Entries {
		section := "audit"
		if event.Source == memory.AuditSourceMission {
			section = "automations"
		}
		out = append(out, dashboardSummaryActivity{
			ID:         event.ID,
			Timestamp:  event.Timestamp,
			Source:     event.Source,
			EventType:  event.EventType,
			TargetName: event.TargetName,
			Status:     event.Status,
			DurationMS: event.DurationMS,
			Section:    section,
		})
	}
	return out
}
