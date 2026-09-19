package server

import (
	"context"
	"encoding/json"
	"time"

	"aurago/internal/planner"
	"aurago/internal/systemworld"
	"aurago/internal/tools"
)

// collectWorldFacts runs once per minute in the existing shared metrics worker.
// Only explicit counters and public identifiers cross the persistence boundary.
func (s *Server) collectWorldFacts(ctx context.Context, now int64) {
	w := s.worldRuntime()
	if now-w.lastFacts < 60000 {
		return
	}
	w.lastFacts = now
	cfg := s.ConfigSnapshot()
	entities := []systemworld.Entity{}
	metrics := map[string]float64{}
	var currentIssues map[string]bool
	district := func(id, state string, values map[string]float64) {
		entities = append(entities, systemworld.Entity{ID: id, Kind: "district", District: id, State: state, At: now, Source: "system-world/" + id, Values: values})
	}
	if s.ShortTermMem != nil {
		values := map[string]float64{}
		if n, err := s.ShortTermMem.GetCoreMemoryCount(); err == nil {
			values["core_facts"] = float64(n)
		}
		if n, err := s.ShortTermMem.GetMessageCount(); err == nil {
			values["chat_messages"] = float64(n)
		}
		if n, err := s.ShortTermMem.GetNotesCount(); err == nil {
			values["notes"] = float64(n)
		}
		if s.LongTermMem != nil && !s.LongTermMem.IsDisabled() {
			values["vectors"] = float64(s.LongTermMem.Count())
		}
		state := "unknown"
		if len(values) > 0 {
			state = "idle"
		}
		district("memory", state, values)
	}
	if s.KG != nil {
		if n, e, err := s.KG.Stats(); err == nil {
			district("graph", "idle", map[string]float64{"nodes": float64(n), "relations": float64(e)})
		}
	}
	if s.MissionManagerV2 != nil {
		missions := s.MissionManagerV2.List()
		running := 0
		for _, m := range missions {
			if m.Status == "running" {
				running++
			}
		}
		queue, _ := s.MissionManagerV2.GetQueue()
		queued := 0
		if queue != nil {
			queued = len(queue.List())
		}
		state := "idle"
		if running > 0 {
			state = "running"
		}
		district("missions", state, map[string]float64{"total": float64(len(missions)), "running": float64(running), "queued": float64(queued)})
	}
	configured := 0
	for id, enabled := range dashboardIntegrationFlags(cfg) {
		state := "disabled"
		if enabled {
			state = "configured"
			configured++
		}
		entities = append(entities, systemworld.Entity{ID: "integration:" + id, Kind: "integration", District: "integrations", Label: id, State: state, At: now, Source: "configuration"})
	}
	district("integrations", "idle", map[string]float64{"enabled": float64(configured)})
	if s.PlannerDB != nil {
		if page, err := planner.ListOperationalIssues(s.PlannerDB, planner.OperationalIssueListFilter{Status: "open", Limit: 100}); err == nil {
			currentIssues = map[string]bool{}
			state := "idle"
			for _, v := range page.Items {
				currentIssues["issue:"+planner.OperationalIssuePublicID(v.Fingerprint)] = true
				severity := "waiting"
				if v.Severity == "error" || v.Severity == "critical" || v.Severity == "high" {
					state = "error"
					severity = "error"
				}
				// Kind/severity are sufficient for history. Do not persist issue detail or reference.
				entities = append(entities, systemworld.Entity{ID: "issue:" + planner.OperationalIssuePublicID(v.Fingerprint), Kind: "issue", District: "operations", Label: worldText(v.Kind), State: severity, At: now, Source: "operational-issues", Values: map[string]float64{"occurrences": float64(v.Occurrences)}})
			}
			district("operations", state, map[string]float64{"issues": float64(page.Total)})
		}
	}
	if s.BudgetTracker != nil {
		b := s.BudgetTracker.GetStatus()
		metrics["cost"] = b.SpentUSD
		for _, m := range b.Models {
			metrics["tokens_input"] += float64(m.InputTokens)
			metrics["tokens_output"] += float64(m.OutputTokens)
		}
	}
	if ctx.Err() != nil {
		return
	}
	var sensors struct {
		Data []tools.TempSensor `json:"data"`
	}
	if json.Unmarshal([]byte(tools.GetSystemMetrics("sensors")), &sensors) == nil {
		for _, v := range sensors.Data {
			if v.Temperature > 0 && v.Temperature < 150 {
				if v.Temperature > metrics["temperature"] {
					metrics["temperature"] = v.Temperature
				}
			}
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, key := range []string{"temperature", "cost", "tokens_input", "tokens_output"} {
		delete(w.metrics, key)
	}
	for key, v := range metrics {
		w.metrics[key] = v
	}
	for _, e := range entities {
		w.set(e)
	}
	if currentIssues != nil {
		for id, e := range w.entities {
			if e.Kind == "issue" && !currentIssues[id] {
				e.State = "removed"
				e.At = now
				w.set(e)
				delete(w.entities, id)
			}
		}
	}
	e := w.entities["agent"]
	e.Model = worldText(cfg.LLM.Model)
	e.Provider = worldText(cfg.LLM.ProviderType)
	e.At = now
	e.Source = "agent/runtime"
	w.set(e)
	// Short-lived tool indicators cannot remain busy indefinitely after a restart/error.
	for id, e := range w.entities {
		if e.Kind == "tool" && now-e.At > int64(5*time.Minute/time.Millisecond) {
			e.State = "removed"
			e.At = now
			w.set(e)
			delete(w.entities, id)
			delete(w.toolStates, e.Label)
		}
	}
}
