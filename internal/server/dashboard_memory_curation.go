package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/agent"
	"aurago/internal/memory"
)

const memoryCurationConfirmToken = "APPLY_MEMORY_CURATION"

type dashboardMemoryCurationRequest struct {
	Limit   int    `json:"limit"`
	Confirm string `json:"confirm"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
}

func handleDashboardMemoryCuration(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.ShortTermMem == nil {
			jsonError(w, "Memory subsystem not available", http.StatusServiceUnavailable)
			return
		}

		path := strings.TrimSuffix(r.URL.Path, "/")
		switch {
		case path == "/api/dashboard/memory/curation" && r.Method == http.MethodGet:
			handleDashboardMemoryCurationList(s, w, r)
		case path == "/api/dashboard/memory/curation/dry-run" && r.Method == http.MethodPost:
			handleDashboardMemoryCurationDryRun(s, w, r)
		case path == "/api/dashboard/memory/curation/apply" && r.Method == http.MethodPost:
			handleDashboardMemoryCurationApply(s, w, r)
		case strings.HasPrefix(path, "/api/dashboard/memory/curation/") && r.Method == http.MethodPost:
			docID := strings.TrimPrefix(path, "/api/dashboard/memory/curation/")
			if docID == "" || docID == "dry-run" || docID == "apply" {
				jsonError(w, "doc_id is required", http.StatusBadRequest)
				return
			}
			handleDashboardMemoryCurationDocAction(s, w, r, docID)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleDashboardMemoryCurationList(s *Server, w http.ResponseWriter, r *http.Request) {
	limit := parseMemoryCurationLimit(r, 100)
	plan, err := buildDashboardMemoryCurationPlan(s, limit)
	if err != nil {
		s.Logger.Error("Failed to build memory curation plan", "error", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	events, _ := s.ShortTermMem.ListMemoryCurationEvents(20)
	archived, _ := s.ShortTermMem.ListArchivedMemoryMeta(50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"plan":     plan,
		"events":   events,
		"archived": archived,
	})
}

func handleDashboardMemoryCurationDryRun(s *Server, w http.ResponseWriter, r *http.Request) {
	req := decodeDashboardMemoryCurationRequest(r)
	plan, err := buildDashboardMemoryCurationPlan(s, req.Limit)
	if err != nil {
		s.Logger.Error("Failed to build memory curation dry-run", "error", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                "ok",
		"plan":                  plan,
		"auto_confirm":          plan.AutoConfirm,
		"auto_archive":          plan.AutoArchive,
		"review_required":       plan.ReviewRequired,
		"auto_confirm_count":    plan.AutoConfirmCount,
		"auto_archive_count":    plan.AutoArchiveCount,
		"review_required_count": plan.ReviewRequiredCount,
	})
}

func handleDashboardMemoryCurationApply(s *Server, w http.ResponseWriter, r *http.Request) {
	req := decodeDashboardMemoryCurationRequest(r)
	if strings.TrimSpace(req.Confirm) != memoryCurationConfirmToken {
		jsonError(w, "confirmation token is required", http.StatusBadRequest)
		return
	}
	plan, err := buildDashboardMemoryCurationPlan(s, req.Limit)
	if err != nil {
		s.Logger.Error("Failed to build memory curation apply plan", "error", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	applied := 0
	for _, action := range append(plan.AutoConfirm, plan.AutoArchive...) {
		if err := s.ShortTermMem.ApplyMemoryCurationAction(action, "system", false); err != nil {
			s.Logger.Warn("Failed to apply memory curation action", "doc_id", action.DocID, "action", action.Action, "error", err)
			continue
		}
		applied++
	}
	if applied > 0 {
		agent.InvalidateMemoryMetaCache()
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "ok",
		"applied": applied,
		"plan":    plan,
	})
}

func handleDashboardMemoryCurationDocAction(s *Server, w http.ResponseWriter, r *http.Request, docID string) {
	req := decodeDashboardMemoryCurationRequest(r)
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		jsonError(w, "action is required", http.StatusBadRequest)
		return
	}
	curationAction := memory.MemoryCurationAction{
		DocID:  docID,
		Action: action,
		Reason: req.Reason,
	}
	if err := s.ShortTermMem.ApplyMemoryCurationAction(curationAction, "admin", false); err != nil {
		s.Logger.Warn("Failed to apply manual memory curation action", "doc_id", docID, "action", action, "error", err)
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	agent.InvalidateMemoryMetaCache()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func buildDashboardMemoryCurationPlan(s *Server, limit int) (memory.MemoryCurationPlan, error) {
	if limit <= 0 {
		limit = 100
	}
	metas, err := s.ShortTermMem.GetAllMemoryMeta(50000, 0)
	if err != nil {
		return memory.MemoryCurationPlan{}, err
	}
	usage, err := s.ShortTermMem.GetMemoryUsageStats(30, 500)
	if err != nil {
		usage = memory.MemoryUsageStats{WindowDays: 30}
	}
	threshold := 0.92
	if s.Cfg != nil && s.Cfg.MemoryAnalysis.AutoConfirm > 0 {
		threshold = s.Cfg.MemoryAnalysis.AutoConfirm
	}
	return memory.BuildMemoryCurationPlan(metas, usage, memory.MemoryCurationOptions{
		ConfirmThreshold: threshold,
		MaxActions:       limit,
	}), nil
}

func decodeDashboardMemoryCurationRequest(r *http.Request) dashboardMemoryCurationRequest {
	var req dashboardMemoryCurationRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Limit <= 0 {
		req.Limit = parseMemoryCurationLimit(r, 100)
	}
	return req
}

func parseMemoryCurationLimit(r *http.Request, fallback int) int {
	limit := fallback
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit <= 0 {
		limit = fallback
	}
	if limit > 500 {
		limit = 500
	}
	return limit
}
