package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/memory"
)

// handleDashboardMoodHistory returns mood log entries for a given time range.
func handleDashboardMoodHistory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		hours := 24
		if h := r.URL.Query().Get("hours"); h != "" {
			if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 {
				hours = parsed
			}
		}

		entries, err := s.ShortTermMem.GetMoodHistory(hours)
		if err != nil {
			s.Logger.Error("Failed to get mood history", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if entries == nil {
			entries = []memory.MoodLogEntry{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

// handleDashboardEmotionHistory returns synthesized emotion history entries.
func handleDashboardEmotionHistory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		hours := 24
		if h := r.URL.Query().Get("hours"); h != "" {
			if parsed, err := strconv.Atoi(h); err == nil && parsed > 0 {
				hours = parsed
			}
		}

		entries, err := s.ShortTermMem.GetEmotionHistory(hours)
		if err != nil {
			s.Logger.Error("Failed to get emotion history", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if entries == nil {
			entries = []memory.EmotionHistoryEntry{}
		}
		for i := range entries {
			entries[i].Description = sanitizeEmotionPreview(entries[i].Description, 280)
			entries[i].Cause = sanitizeEmotionPreview(entries[i].Cause, 140)
			entries[i].TriggerSummary = sanitizeEmotionPreview(entries[i].TriggerSummary, 80)
		}

		w.Header().Set("Content-Type", "application/json")
		type emotionSummary struct {
			Count          int            `json:"count"`
			LatestCause    string         `json:"latest_cause"`
			LatestStyle    string         `json:"latest_style"`
			LatestSource   string         `json:"latest_source"`
			AverageValence float64        `json:"average_valence"`
			AverageArousal float64        `json:"average_arousal"`
			TriggerCounts  map[string]int `json:"trigger_counts"`
		}

		summary := emotionSummary{
			Count:         len(entries),
			TriggerCounts: map[string]int{},
		}
		if len(entries) > 0 {
			summary.LatestCause = entries[0].Cause
			summary.LatestStyle = entries[0].RecommendedResponseStyle
			summary.LatestSource = entries[0].Source
		}
		for _, e := range entries {
			summary.AverageValence += e.Valence
			summary.AverageArousal += e.Arousal
			trigger := strings.TrimSpace(e.TriggerSummary)
			if trigger == "" {
				trigger = "conversation"
			}
			if len(trigger) > 64 {
				trigger = trigger[:64]
			}
			summary.TriggerCounts[trigger]++
		}
		if len(entries) > 0 {
			summary.AverageValence /= float64(len(entries))
			summary.AverageArousal /= float64(len(entries))
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": entries,
			"summary": summary,
			"hours":   hours,
			"count":   len(entries),
		})
	}
}

// handleDashboardJournal returns recent journal entries as JSON.
// Query params: ?from=YYYY-MM-DD&to=YYYY-MM-DD&type=xxx&limit=20
func handleDashboardJournal(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		var types []string
		if t := r.URL.Query().Get("type"); t != "" {
			types = []string{t}
		}

		entries, err := s.ShortTermMem.GetJournalEntries(from, to, types, limit)
		if err != nil {
			s.Logger.Error("Failed to list journal entries", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if entries == nil {
			entries = []memory.JournalEntry{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"entries": entries,
			"count":   len(entries),
		})
	}
}

// handleDashboardJournalSummary returns recent daily summaries.
// Query params: ?days=7
func handleDashboardJournalSummary(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		days := 7
		if d := r.URL.Query().Get("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
				days = parsed
			}
		}

		summaries, err := s.ShortTermMem.GetRecentDailySummaries(days)
		if err != nil {
			s.Logger.Error("Failed to list daily summaries", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if summaries == nil {
			summaries = []memory.DailySummary{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summaries": summaries,
			"count":     len(summaries),
		})
	}
}

// handleActivityOverview returns a recent multi-day activity overview.
// Query params: ?days=7&include_entries=true
func handleActivityOverview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.ShortTermMem == nil {
			jsonError(w, "Memory unavailable", http.StatusServiceUnavailable)
			return
		}

		days := 7
		if d := r.URL.Query().Get("days"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
				days = parsed
			}
		}
		includeEntries := strings.EqualFold(r.URL.Query().Get("include_entries"), "true")

		overview, err := s.ShortTermMem.BuildRecentActivityOverview(days, includeEntries)
		if err != nil {
			s.Logger.Error("Failed to build activity overview", "error", err, "days", days)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if overview == nil {
			overview = &memory.ActivityOverviewResponse{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(overview)
	}
}

// handleDashboardJournalStats returns journal statistics.
// Query params: ?from=YYYY-MM-DD&to=YYYY-MM-DD
func handleDashboardJournalStats(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")

		stats, err := s.ShortTermMem.GetJournalStats(from, to)
		if err != nil {
			s.Logger.Error("Failed to get journal stats", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

// handleDashboardErrors returns frequent and recent error patterns from the error learning system.
func handleDashboardErrors(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		frequent, err := s.ShortTermMem.GetFrequentErrors("", 10)
		if err != nil || frequent == nil {
			frequent = []memory.ErrorPattern{}
		}

		recent, err := s.ShortTermMem.GetRecentErrors(10)
		if err != nil || recent == nil {
			recent = []memory.ErrorPattern{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"frequent": frequent,
			"recent":   recent,
		})
	}
}

