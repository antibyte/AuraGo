package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/memory"
)

const memoryHygieneConfirmToken = "APPLY_MEMORY_HYGIENE"
const kgHygieneConfirmToken = "APPLY_KG_HYGIENE"

type dashboardMemoryHygieneRequest struct {
	Limit     int    `json:"limit"`
	Confirm   string `json:"confirm"`
	IncludeKG bool   `json:"include_kg"`
	KGConfirm string `json:"kg_confirm"`
}

type dashboardHygieneFailure struct {
	Domain string `json:"domain"`
	Target string `json:"target"`
	Action string `json:"action"`
	Error  string `json:"error"`
}

type dashboardMemoryHygienePlan struct {
	Memory    memory.MemoryCurationPlan         `json:"memory"`
	Journal   memory.JournalConsolidationReport `json:"journal"`
	Notes     memory.NotesCurationPlan          `json:"notes"`
	Canonical memory.CanonicalRepairReport      `json:"canonical"`
	KG        dashboardKGHygienePlan            `json:"kg"`
	Totals    map[string]int                    `json:"totals"`
}

type dashboardKGHygienePlan struct {
	Available             bool                                    `json:"available"`
	DryRun                bool                                    `json:"dry_run"`
	OpenConflicts         int                                     `json:"open_conflicts"`
	ConflictSuggestions   []memory.KGConflictResolutionSuggestion `json:"conflict_suggestions,omitempty"`
	DuplicateGroups       int                                     `json:"duplicate_groups"`
	IDDuplicateGroups     int                                     `json:"id_duplicate_groups"`
	StaleEdgesRemoved     int                                     `json:"stale_edges_removed,omitempty"`
	StaleNodesRemoved     int                                     `json:"stale_nodes_removed,omitempty"`
	OptimizedNodesRemoved int                                     `json:"optimized_nodes_removed,omitempty"`
}

func handleDashboardMemoryHygiene(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.ShortTermMem == nil {
			jsonError(w, "Memory subsystem not available", http.StatusServiceUnavailable)
			return
		}

		path := strings.TrimSuffix(r.URL.Path, "/")
		switch {
		case path == "/api/dashboard/memory/hygiene" && r.Method == http.MethodGet:
			handleDashboardMemoryHygieneList(s, w, r)
		case path == "/api/dashboard/memory/hygiene/dry-run" && r.Method == http.MethodPost:
			handleDashboardMemoryHygieneDryRun(s, w, r)
		case path == "/api/dashboard/memory/hygiene/apply" && r.Method == http.MethodPost:
			handleDashboardMemoryHygieneApply(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDashboardMemoryHygieneList(s *Server, w http.ResponseWriter, r *http.Request) {
	limit := parseMemoryCurationLimit(r, 100)
	plan, err := buildDashboardMemoryHygienePlan(s, limit, true)
	if err != nil {
		s.Logger.Error("Failed to build memory hygiene plan", "error", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	events, _ := s.ShortTermMem.ListJournalCurationEvents(20)
	writeMemoryHygieneResponse(w, map[string]interface{}{
		"status":         "ok",
		"plan":           plan,
		"journal_events": events,
	})
}

func handleDashboardMemoryHygieneDryRun(s *Server, w http.ResponseWriter, r *http.Request) {
	req := decodeDashboardMemoryHygieneRequest(r)
	plan, err := buildDashboardMemoryHygienePlan(s, req.Limit, true)
	if err != nil {
		s.Logger.Error("Failed to build memory hygiene dry-run", "error", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	writeMemoryHygieneResponse(w, map[string]interface{}{
		"status": "ok",
		"plan":   plan,
		"totals": plan.Totals,
	})
}

func handleDashboardMemoryHygieneApply(s *Server, w http.ResponseWriter, r *http.Request) {
	req := decodeDashboardMemoryHygieneRequest(r)
	if strings.TrimSpace(req.Confirm) != memoryHygieneConfirmToken {
		jsonError(w, "confirmation token is required", http.StatusBadRequest)
		return
	}
	if req.IncludeKG && strings.TrimSpace(req.KGConfirm) != kgHygieneConfirmToken {
		jsonError(w, "kg confirmation token is required", http.StatusBadRequest)
		return
	}
	plan, err := buildDashboardMemoryHygienePlan(s, req.Limit, true)
	if err != nil {
		s.Logger.Error("Failed to apply memory hygiene plan", "error", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	failedActions := make([]dashboardHygieneFailure, 0)
	memoryApplied := 0
	for _, action := range append(plan.Memory.AutoConfirm, plan.Memory.AutoArchive...) {
		if err := s.ShortTermMem.ApplyMemoryCurationAction(action, "system", false); err != nil {
			s.Logger.Warn("Failed to apply memory curation action during hygiene", "doc_id", action.DocID, "action", action.Action, "error", err)
			failedActions = append(failedActions, dashboardHygieneFailure{
				Domain: "memory",
				Target: action.DocID,
				Action: action.Action,
				Error:  err.Error(),
			})
			continue
		}
		memoryApplied++
	}
	noteApplied := 0
	for _, action := range plan.Notes.AutoArchive {
		if err := s.ShortTermMem.ApplyNoteCurationAction(action, "system", false); err != nil {
			s.Logger.Warn("Failed to apply note curation action during hygiene", "note_id", action.NoteID, "error", err)
			failedActions = append(failedActions, dashboardHygieneFailure{
				Domain: "notes",
				Target: strconv.FormatInt(action.NoteID, 10),
				Action: action.Action,
				Error:  err.Error(),
			})
			continue
		}
		noteApplied++
	}
	journalReport, err := s.ShortTermMem.ConsolidateDuplicateAutoJournalErrors(memory.JournalConsolidationOptions{
		Now:           time.Now().UTC(),
		OlderThan:     24 * time.Hour,
		MinDuplicates: 2,
		Limit:         req.Limit,
		Actor:         "dashboard",
	})
	if err != nil {
		s.Logger.Warn("Failed to consolidate journal duplicates during hygiene", "error", err)
		failedActions = append(failedActions, dashboardHygieneFailure{
			Domain: "journal",
			Action: "consolidate_duplicates",
			Error:  err.Error(),
		})
		plan.Journal = memory.JournalConsolidationReport{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			DryRun:      false,
		}
	} else {
		plan.Journal = journalReport
	}
	canonicalReport, err := s.ShortTermMem.RepairCanonicalMemoryNames(s.LongTermMem, memory.CanonicalRepairOptions{
		Limit: req.Limit,
		Actor: "dashboard",
	})
	if err != nil {
		s.Logger.Warn("Failed to repair canonical names during hygiene", "error", err)
		failedActions = append(failedActions, dashboardHygieneFailure{
			Domain: "canonical",
			Action: "repair_names",
			Error:  err.Error(),
		})
		plan.Canonical = memory.CanonicalRepairReport{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			DryRun:      false,
		}
	} else {
		plan.Canonical = canonicalReport
	}
	kgApplied := 0
	if req.IncludeKG {
		if s.KG == nil {
			failedActions = append(failedActions, dashboardHygieneFailure{
				Domain: "kg",
				Action: "cleanup",
				Error:  "knowledge graph is unavailable",
			})
		} else {
			kgPlan, kgErr := applyDashboardKGHygiene(s, req.Limit)
			if kgErr != nil {
				s.Logger.Warn("Failed to apply KG hygiene", "error", kgErr)
				failedActions = append(failedActions, dashboardHygieneFailure{
					Domain: "kg",
					Action: "cleanup",
					Error:  kgErr.Error(),
				})
			} else {
				plan.KG = kgPlan
				kgApplied = kgPlan.StaleEdgesRemoved + kgPlan.StaleNodesRemoved + kgPlan.OptimizedNodesRemoved
			}
		}
	}
	plan.Totals = dashboardMemoryHygieneTotals(plan)
	if memoryApplied > 0 || plan.Canonical.RepairedCount > 0 {
		agent.InvalidateMemoryMetaCache()
	}
	response := map[string]interface{}{
		"status": "ok",
		"applied": map[string]int{
			"memory":    memoryApplied,
			"journal":   plan.Journal.RemovedEntries,
			"notes":     noteApplied,
			"canonical": plan.Canonical.RepairedCount,
			"kg":        kgApplied,
		},
		"plan":   plan,
		"totals": plan.Totals,
	}
	if len(failedActions) > 0 {
		response["failed_actions"] = failedActions
		errors := make([]string, 0, len(failedActions))
		for _, failure := range failedActions {
			errors = append(errors, failure.Domain+": "+failure.Error)
		}
		response["errors"] = errors
	}
	writeMemoryHygieneResponse(w, response)
}

func buildDashboardMemoryHygienePlan(s *Server, limit int, dryRun bool) (dashboardMemoryHygienePlan, error) {
	if limit <= 0 {
		limit = 100
	}
	memoryPlan, err := buildDashboardMemoryCurationPlan(s, limit)
	if err != nil {
		return dashboardMemoryHygienePlan{}, err
	}
	journalReport, err := s.ShortTermMem.ConsolidateDuplicateAutoJournalErrors(memory.JournalConsolidationOptions{
		Now:           time.Now().UTC(),
		OlderThan:     24 * time.Hour,
		MinDuplicates: 2,
		Limit:         limit,
		DryRun:        dryRun,
		Actor:         "dashboard",
	})
	if err != nil {
		return dashboardMemoryHygienePlan{}, err
	}
	notesPlan, err := s.ShortTermMem.BuildNotesCurationPlan(memory.NotesCurationOptions{
		Now:        time.Now().UTC(),
		MaxActions: limit,
	})
	if err != nil {
		return dashboardMemoryHygienePlan{}, err
	}
	canonicalReport, err := s.ShortTermMem.RepairCanonicalMemoryNames(s.LongTermMem, memory.CanonicalRepairOptions{
		Limit:  limit,
		DryRun: dryRun,
		Actor:  "dashboard",
	})
	if err != nil {
		return dashboardMemoryHygienePlan{}, err
	}
	plan := dashboardMemoryHygienePlan{
		Memory:    memoryPlan,
		Journal:   journalReport,
		Notes:     notesPlan,
		Canonical: canonicalReport,
	}
	plan.KG = buildDashboardKGHygienePlan(s, limit, dryRun)
	plan.Totals = dashboardMemoryHygieneTotals(plan)
	return plan, nil
}

func dashboardMemoryHygieneTotals(plan dashboardMemoryHygienePlan) map[string]int {
	return map[string]int{
		"memory_auto_confirm": plan.Memory.AutoConfirmCount,
		"memory_auto_archive": plan.Memory.AutoArchiveCount,
		"memory_review":       plan.Memory.ReviewRequiredCount,
		"journal_duplicates":  plan.Journal.Groups,
		"journal_removed":     plan.Journal.RemovedEntries,
		"notes_auto_archive":  plan.Notes.AutoArchiveCount,
		"notes_review":        plan.Notes.ReviewRequiredCount,
		"canonical_repairs":   plan.Canonical.RepairedCount,
		"canonical_skipped":   plan.Canonical.SkippedCount,
		"kg_open_conflicts":   plan.KG.OpenConflicts,
		"kg_duplicates":       plan.KG.DuplicateGroups + plan.KG.IDDuplicateGroups,
		"kg_removed":          plan.KG.StaleEdgesRemoved + plan.KG.StaleNodesRemoved + plan.KG.OptimizedNodesRemoved,
	}
}

func buildDashboardKGHygienePlan(s *Server, limit int, dryRun bool) dashboardKGHygienePlan {
	plan := dashboardKGHygienePlan{DryRun: dryRun}
	if s == nil || s.KG == nil {
		return plan
	}
	plan.Available = true
	if conflicts, err := s.KG.GetOpenKGConflicts(limit); err == nil {
		plan.OpenConflicts = len(conflicts)
	}
	if suggestions, err := s.KG.SuggestKGConflictResolutions(limit); err == nil {
		plan.ConflictSuggestions = suggestions
	}
	if quality, err := s.KG.QualityReport(limit); err == nil && quality != nil {
		plan.DuplicateGroups = quality.DuplicateGroups
		plan.IDDuplicateGroups = quality.IDDuplicateGroups
	}
	return plan
}

func applyDashboardKGHygiene(s *Server, limit int) (dashboardKGHygienePlan, error) {
	plan := buildDashboardKGHygienePlan(s, limit, false)
	if s == nil || s.KG == nil {
		return plan, nil
	}
	pendingDays := 7
	if s.Cfg != nil && s.Cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays > 0 {
		pendingDays = s.Cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays
	}
	edgesRemoved, nodesRemoved, err := s.KG.CleanupStaleGraphWithOptions(memory.KnowledgeGraphCleanupOptions{
		PendingCoMentionDays: pendingDays,
		StaleNodeDays:        30,
	})
	if err != nil {
		return plan, err
	}
	plan.StaleEdgesRemoved = edgesRemoved
	plan.StaleNodesRemoved = nodesRemoved
	optimized, err := s.KG.OptimizeGraph(1)
	if err != nil {
		return plan, err
	}
	plan.OptimizedNodesRemoved = optimized
	return plan, nil
}

func decodeDashboardMemoryHygieneRequest(r *http.Request) dashboardMemoryHygieneRequest {
	var req dashboardMemoryHygieneRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Limit <= 0 {
		req.Limit = parseMemoryCurationLimit(r, 100)
	}
	return req
}

func writeMemoryHygieneResponse(w http.ResponseWriter, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}
