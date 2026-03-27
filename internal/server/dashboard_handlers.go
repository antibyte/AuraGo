package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/invasion"
	"aurago/internal/inventory"
	"aurago/internal/memory"
	"aurago/internal/mqtt"
	"aurago/internal/prompts"
	"aurago/internal/tools"
)

// ── Dashboard API Handlers ──────────────────────────────────────────────────
// Provides data for the /dashboard metrics page.
// All endpoints are guarded by WebConfig.Enabled in server.go route registration.

// handleDashboardSystem returns system metrics (CPU, RAM, Disk, Network, SSE clients, uptime).
func handleDashboardSystem(s *Server, sse *SSEBroadcaster, startedAt time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the JSON from GetSystemMetrics
		raw := tools.GetSystemMetrics("")
		var metricsResult struct {
			Status string              `json:"status"`
			Data   tools.SystemMetrics `json:"data"`
		}
		if err := json.Unmarshal([]byte(raw), &metricsResult); err != nil {
			s.Logger.Error("Failed to parse system metrics", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"cpu":            metricsResult.Data.CPU,
			"memory":         metricsResult.Data.Memory,
			"disk":           metricsResult.Data.Disk,
			"network":        metricsResult.Data.Network,
			"sse_clients":    sse.ClientCount(),
			"uptime_seconds": int(time.Since(startedAt).Seconds()),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleDashboardMoodHistory returns mood log entries for a given time range.
func handleDashboardMoodHistory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if entries == nil {
			entries = []memory.EmotionHistoryEntry{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

// handleDashboardMemory returns memory statistics (core memory, messages, vectordb, graph, milestones).
func handleDashboardMemory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		coreCount, _ := s.ShortTermMem.GetCoreMemoryCount()
		msgCount, _ := s.ShortTermMem.GetMessageCount()

		vectorCount := 0
		vectorDisabled := false
		if s.LongTermMem != nil {
			vectorCount = s.LongTermMem.Count()
			vectorDisabled = s.LongTermMem.IsDisabled()
		}

		graphNodes, graphEdges := 0, 0
		if s.KG != nil {
			graphNodes, graphEdges = s.KG.Stats()
		}

		milestones, _ := s.ShortTermMem.GetMilestoneEntries(10)
		if milestones == nil {
			milestones = []memory.MilestoneEntry{}
		}

		// Extended memory stats: journal, notes, error patterns
		journalCount := 0
		if stats, err := s.ShortTermMem.GetJournalStats("", ""); err == nil {
			for _, c := range stats {
				journalCount += c
			}
		}
		notesCount, _ := s.ShortTermMem.GetNotesCount()
		errorPatternsCount, _ := s.ShortTermMem.GetErrorPatternsCount()
		episodicStats, _ := s.ShortTermMem.GetEpisodicMemoryStats(72, 4)
		usageStats, _ := s.ShortTermMem.GetMemoryUsageStats(14, 5)
		memoryHealth := memory.MemoryHealthReport{
			Usage: usageStats,
		}
		if metas, err := s.ShortTermMem.GetAllMemoryMeta(); err == nil {
			memoryHealth = memory.BuildMemoryHealthReport(metas, usageStats)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"core_memory_facts": coreCount,
			"chat_messages":     msgCount,
			"vectordb_entries":  vectorCount,
			"vectordb_disabled": vectorDisabled,
			"knowledge_graph": map[string]int{
				"nodes": graphNodes,
				"edges": graphEdges,
			},
			"journal_entries": journalCount,
			"notes_count":     notesCount,
			"error_patterns":  errorPatternsCount,
			"milestones":      milestones,
			"episodic":        episodicStats,
			"memory_health":   memoryHealth,
		})
	}
}

// handleDashboardCoreMemory returns all core memory facts as a JSON array.
func handleDashboardCoreMemory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rows, err := s.ShortTermMem.GetCoreMemoryFacts()
		if err != nil {
			s.Logger.Error("Failed to get core memory facts", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"facts": rows,
			"count": len(rows),
		})
	}
}

// handleDashboardProfile returns all user profile entries grouped by category.
func handleDashboardProfile(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		entries, err := s.ShortTermMem.GetProfileEntries("")
		if err != nil {
			s.Logger.Error("Failed to get profile entries", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		categories := make(map[string][]map[string]interface{})
		for _, e := range entries {
			categories[e.Category] = append(categories[e.Category], map[string]interface{}{
				"key":        e.Key,
				"value":      e.Value,
				"confidence": e.Confidence,
				"source":     e.Source,
				"updated_at": e.UpdatedAt,
				"first_seen": e.FirstSeen,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"categories":    categories,
			"total_entries": len(entries),
		})
	}
}

// handleDashboardProfileEntry handles DELETE and PUT operations on individual
// user profile entries.
//
//	DELETE /api/dashboard/profile/entry?category=X&key=Y  – removes the entry
//	PUT    /api/dashboard/profile/entry  { category, key, value }  – updates the value
func handleDashboardProfileEntry(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			category := r.URL.Query().Get("category")
			key := r.URL.Query().Get("key")
			if category == "" || key == "" {
				http.Error(w, "category and key are required", http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.DeleteProfileEntry(category, key); err != nil {
				s.Logger.Error("Failed to delete profile entry", "error", err)
				http.Error(w, "Not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodPut:
			var body struct {
				Category string `json:"category"`
				Key      string `json:"key"`
				Value    string `json:"value"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Category == "" || body.Key == "" || body.Value == "" {
				http.Error(w, "category, key, and value are required", http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.UpsertProfileEntry(body.Category, body.Key, body.Value, "manual"); err != nil {
				s.Logger.Error("Failed to update profile entry", "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDashboardActivity returns cron jobs, processes, webhooks, co-agents, and background tasks.
func handleDashboardActivity(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Cron Jobs
		var cronJobs interface{} = []struct{}{}
		if s.CronManager != nil {
			cronJobs = s.CronManager.GetJobs()
		}

		// Process Registry
		var processes interface{} = []struct{}{}
		if s.Registry != nil {
			processes = s.Registry.List()
		}

		// Webhooks summary
		webhookInfo := map[string]interface{}{"count": 0, "recent_events": 0}
		if s.WebhookManager != nil {
			hooks := s.WebhookManager.List()
			webhookInfo["count"] = len(hooks)
			if whLog := s.WebhookManager.GetLog(); whLog != nil {
				webhookInfo["recent_events"] = len(whLog.Recent(20))
			}
		}

		// Co-Agents
		var coagents interface{} = []struct{}{}
		if s.CoAgentRegistry != nil {
			coagents = s.CoAgentRegistry.List()
		}

		backgroundSummary := map[string]int{
			"queued":    0,
			"waiting":   0,
			"running":   0,
			"completed": 0,
			"failed":    0,
			"canceled":  0,
			"total":     0,
		}
		var backgroundTasks interface{} = []struct{}{}
		if s.BackgroundTasks != nil {
			summary := s.BackgroundTasks.Summary()
			backgroundSummary = map[string]int{
				"queued":    summary.Queued,
				"waiting":   summary.Waiting,
				"running":   summary.Running,
				"completed": summary.Completed,
				"failed":    summary.Failed,
				"canceled":  summary.Canceled,
				"total":     summary.Total,
			}
			backgroundTasks = s.BackgroundTasks.ListTasks(12)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cron_jobs":               cronJobs,
			"processes":               processes,
			"webhooks":                webhookInfo,
			"coagents":                coagents,
			"background_tasks":        backgroundTasks,
			"background_task_summary": backgroundSummary,
		})
	}
}

// handleCronAPI handles DELETE and PUT operations on cron jobs from the dashboard.
//
//	DELETE /api/cron?id=xxx                    – removes the cron job
//	PUT    /api/cron  {id, cron_expr, task_prompt} – updates (remove + re-add)
func handleCronAPI(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.CronManager == nil {
			http.Error(w, "Cron scheduler not available", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if id == "" {
				http.Error(w, "id required", http.StatusBadRequest)
				return
			}
			result, err := s.CronManager.ManageSchedule("remove", id, "", "")
			if err != nil {
				s.Logger.Error("Failed to remove cron job", "id", id, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result)) //nolint:errcheck

		case http.MethodPut:
			var body struct {
				ID         string `json:"id"`
				CronExpr   string `json:"cron_expr"`
				TaskPrompt string `json:"task_prompt"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" || body.CronExpr == "" || body.TaskPrompt == "" {
				http.Error(w, "id, cron_expr, and task_prompt required", http.StatusBadRequest)
				return
			}
			// Remove old job first (ignore if not found)
			if _, err := s.CronManager.ManageSchedule("remove", body.ID, "", ""); err != nil {
				s.Logger.Warn("Failed to remove old cron job before update", "id", body.ID, "error", err)
			}
			// Re-add with same ID and updated parameters
			result, err := s.CronManager.ManageSchedule("add", body.ID, body.CronExpr, body.TaskPrompt)
			if err != nil {
				s.Logger.Error("Failed to add updated cron job", "id", body.ID, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result)) //nolint:errcheck

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDashboardPromptStats returns aggregated prompt builder metrics.
func handleDashboardPromptStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prompts.GetAggregatedStats())
	}
}

// handleDashboardToolStats returns aggregated tool usage statistics.
func handleDashboardToolStats(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		stats := prompts.GetToolUsageStats()

		type response struct {
			prompts.ToolUsageAggregated
			AdaptiveEnabled bool                         `json:"adaptive_enabled"`
			AdaptiveScores  []prompts.ToolDecayScore     `json:"adaptive_scores,omitempty"`
			MaxTools        int                          `json:"max_tools,omitempty"`
			AgentTelemetry  agent.AgentTelemetrySnapshot `json:"agent_telemetry"`
		}
		resp := response{
			ToolUsageAggregated: stats,
			AdaptiveEnabled:     cfg.Agent.AdaptiveTools.Enabled,
			AgentTelemetry:      agent.GetAgentTelemetrySnapshot(),
		}
		if cfg.Agent.AdaptiveTools.Enabled {
			resp.AdaptiveScores = prompts.GetAdaptiveToolScores(cfg.Agent.AdaptiveTools.DecayHalfLifeDays, cfg.Agent.AdaptiveTools.WeightSuccessRate)
			resp.MaxTools = cfg.Agent.AdaptiveTools.MaxTools
		}
		json.NewEncoder(w).Encode(resp)
	}
}

// handleDashboardLogs returns the last N lines from the supervisor log file.
// Query param: ?lines=100 (default 100, max 500)
func handleDashboardLogs(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		maxLines := 100
		if n, err := strconv.Atoi(r.URL.Query().Get("lines")); err == nil && n > 0 {
			maxLines = n
		}
		if maxLines > 500 {
			maxLines = 500
		}

		logDir := s.Cfg.Logging.LogDir
		if logDir == "" {
			logDir = "./log"
		}

		logPath := filepath.Join(logDir, "aurago.log")
		lines, err := tailFile(logPath, maxLines)
		if err != nil {
			// Try lifeboat.log as fallback
			logPath = filepath.Join(logDir, "lifeboat.log")
			lines, err = tailFile(logPath, maxLines)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"lines":    []string{},
					"error":    "Log file not available",
					"log_file": logPath,
				})
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"lines":    lines,
			"log_file": filepath.Base(logPath),
			"count":    len(lines),
		})
	}
}

// handleDashboardGitHubRepos returns GitHub repos for the dashboard widget.
// It first lists locally tracked projects, then fetches live repos from the GitHub API.
// Only repos in cfg.GitHub.AllowedRepos are returned (or all repos if the list is empty).
func handleDashboardGitHubRepos(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if !s.Cfg.GitHub.Enabled || s.Cfg.GitHub.Owner == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": false,
				"repos":   []struct{}{},
			})
			return
		}

		// Read token from vault
		token := ""
		if s.Vault != nil {
			t, err := s.Vault.ReadSecret("github_token")
			if err == nil {
				token = t
			}
		}

		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": true,
				"error":   "GitHub token not found in vault",
				"repos":   []struct{}{},
			})
			return
		}

		ghCfg := tools.GitHubConfig{
			Token:   token,
			Owner:   s.Cfg.GitHub.Owner,
			BaseURL: s.Cfg.GitHub.BaseURL,
		}

		raw := tools.GitHubListRepos(ghCfg, "")
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			http.Error(w, `{"error":"failed to parse GitHub response"}`, http.StatusInternalServerError)
			return
		}

		if result["status"] != "ok" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": true,
				"error":   result["message"],
				"repos":   []struct{}{},
			})
			return
		}

		// Build allowed repos filter
		allowedMap := map[string]bool{}
		hasAllowedList := len(s.Cfg.GitHub.AllowedRepos) > 0
		for _, r := range s.Cfg.GitHub.AllowedRepos {
			allowedMap[r] = true
		}

		// Load tracked projects for cross-reference
		tracked := map[string]bool{}
		trackedRaw := tools.GitHubListProjects(s.Cfg.Directories.WorkspaceDir)
		var trackedResult map[string]interface{}
		if err := json.Unmarshal([]byte(trackedRaw), &trackedResult); err == nil {
			if projects, ok := trackedResult["projects"].([]interface{}); ok {
				for _, p := range projects {
					if pm, ok := p.(map[string]interface{}); ok {
						if name, ok := pm["name"].(string); ok {
							tracked[name] = true
						}
					}
				}
			}
		}

		// Enrich repos with tracked status and filter by allowed list
		repos := result["repos"]
		var filteredRepos []interface{}
		if repoList, ok := repos.([]interface{}); ok {
			for _, r := range repoList {
				if rm, ok := r.(map[string]interface{}); ok {
					name, _ := rm["name"].(string)
					rm["tracked"] = tracked[name]
					// Include repo if: repo is explicitly allowed, OR it's tracked (agent-created).
					// When AllowedRepos is empty, only tracked repos are shown (consistent with agent enforcement).
					if tracked[name] || (hasAllowedList && allowedMap[name]) {
						filteredRepos = append(filteredRepos, rm)
					}
				}
			}
		}
		if filteredRepos == nil {
			filteredRepos = []interface{}{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": true,
			"owner":   s.Cfg.GitHub.Owner,
			"repos":   filteredRepos,
			"count":   len(filteredRepos),
		})
	}
}

// handleGitHubReposForUI returns all GitHub repos with an `allowed` flag for the config UI.
// Used by the config section to let the user pick which repos the agent may access.
func handleGitHubReposForUI(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if !s.Cfg.GitHub.Enabled {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "GitHub integration is not enabled",
			})
			return
		}

		token := ""
		if s.Vault != nil {
			t, err := s.Vault.ReadSecret("github_token")
			if err == nil {
				token = t
			}
		}
		if token == "" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "GitHub token not found in vault",
			})
			return
		}

		ghCfg := tools.GitHubConfig{
			Token:   token,
			Owner:   s.Cfg.GitHub.Owner,
			BaseURL: s.Cfg.GitHub.BaseURL,
		}

		raw := tools.GitHubListRepos(ghCfg, "")
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "failed to parse GitHub response",
			})
			return
		}

		if result["status"] != "ok" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": result["message"],
			})
			return
		}

		// Build allowed set
		allowedMap := map[string]bool{}
		for _, r := range s.Cfg.GitHub.AllowedRepos {
			allowedMap[r] = true
		}

		// Load tracked (agent-created) projects
		tracked := map[string]bool{}
		trackedRaw := tools.GitHubListProjects(s.Cfg.Directories.WorkspaceDir)
		var trackedResult map[string]interface{}
		if err := json.Unmarshal([]byte(trackedRaw), &trackedResult); err == nil {
			if projects, ok := trackedResult["projects"].([]interface{}); ok {
				for _, p := range projects {
					if pm, ok := p.(map[string]interface{}); ok {
						if name, ok := pm["name"].(string); ok {
							tracked[name] = true
						}
					}
				}
			}
		}

		// Annotate repos
		repos := result["repos"]
		if repoList, ok := repos.([]interface{}); ok {
			for _, r := range repoList {
				if rm, ok := r.(map[string]interface{}); ok {
					name, _ := rm["name"].(string)
					rm["allowed"] = allowedMap[name]
					rm["agent_created"] = tracked[name]
				}
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"repos":  repos,
			"count":  result["count"],
		})
	}
}

// handleDashboardCoreMemoryMutate handles POST (add), PUT (update), and DELETE (remove) for core memory facts.
func handleDashboardCoreMemoryMutate(s *Server, sse *SSEBroadcaster) http.HandlerFunc {
	// pushMemoryStats collects current memory stats and broadcasts them to all SSE clients.
	pushMemoryStats := func() {
		coreCount, _ := s.ShortTermMem.GetCoreMemoryCount()
		msgCount, _ := s.ShortTermMem.GetMessageCount()
		vectorCount, vectorDisabled := 0, false
		if s.LongTermMem != nil {
			vectorCount = s.LongTermMem.Count()
			vectorDisabled = s.LongTermMem.IsDisabled()
		}
		graphNodes, graphEdges := 0, 0
		if s.KG != nil {
			graphNodes, graphEdges = s.KG.Stats()
		}
		sse.BroadcastType(EventMemoryUpdate, map[string]interface{}{
			"core_memory_facts": coreCount,
			"chat_messages":     msgCount,
			"vectordb_entries":  vectorCount,
			"vectordb_disabled": vectorDisabled,
			"knowledge_graph": map[string]int{
				"nodes": graphNodes,
				"edges": graphEdges,
			},
		})
	}

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req struct {
				Fact string `json:"fact"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Fact == "" {
				http.Error(w, `{"error":"fact is required"}`, http.StatusBadRequest)
				return
			}
			id, err := s.ShortTermMem.AddCoreMemoryFact(req.Fact)
			if err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
				return
			}
			go pushMemoryStats()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "id": id})

		case http.MethodPut:
			var req struct {
				ID   int64  `json:"id"`
				Fact string `json:"fact"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 || req.Fact == "" {
				http.Error(w, `{"error":"id and fact are required"}`, http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.UpdateCoreMemoryFact(req.ID, req.Fact); err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
				return
			}
			go pushMemoryStats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodDelete:
			var req struct {
				ID int64 `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
				http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.DeleteCoreMemoryFact(req.ID); err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
				return
			}
			go pushMemoryStats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDashboardOverview returns a composite snapshot of agent status, integrations, missions, invasion, indexer, devices, MQTT, notes, security, context, and last activity.
func handleDashboardOverview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		// ── Agent Info ─────────────────────────────────────────
		agentInfo := map[string]interface{}{
			"model":          cfg.LLM.Model,
			"provider":       cfg.LLM.ProviderType,
			"personality":    cfg.Personality.CorePersonality,
			"context_window": cfg.Agent.ContextWindow,
			"busy":           tools.IsBusy(),
			"debug":          agent.GetDebugMode(),
			"maintenance":    cfg.Maintenance.Enabled,
		}

		// ── Integrations (all Enabled flags) ──────────────────
		integrations := map[string]bool{
			"telegram":          cfg.Telegram.BotToken != "",
			"discord":           cfg.Discord.Enabled,
			"email":             cfg.Email.Enabled,
			"home_assistant":    cfg.HomeAssistant.Enabled,
			"docker":            cfg.Docker.Enabled,
			"co_agents":         cfg.CoAgents.Enabled,
			"webhooks":          cfg.Webhooks.Enabled,
			"webdav":            cfg.WebDAV.Enabled,
			"koofr":             cfg.Koofr.Enabled,
			"paperless_ngx":     cfg.PaperlessNGX.Enabled,
			"chromecast":        cfg.Chromecast.Enabled,
			"proxmox":           cfg.Proxmox.Enabled,
			"ollama":            cfg.Ollama.Enabled,
			"ollama_managed":    cfg.Ollama.ManagedInstance.Enabled,
			"rocketchat":        cfg.RocketChat.Enabled,
			"tailscale":         cfg.Tailscale.Enabled,
			"cloudflare_tunnel": cfg.CloudflareTunnel.Enabled,
			"ansible":           cfg.Ansible.Enabled,
			"invasion":          cfg.InvasionControl.Enabled,
			"github":            cfg.GitHub.Enabled,
			"mqtt":              cfg.MQTT.Enabled,
			"budget":            cfg.Budget.Enabled,
			"indexing":          cfg.Indexing.Enabled,
			"auth":              cfg.Auth.Enabled,
			"fallback_llm":      cfg.FallbackLLM.Enabled,
			"personality_v2":    cfg.Personality.EngineV2,
			"user_profiling":    cfg.Personality.UserProfiling,
			"tts":               cfg.TTS.Provider != "" || cfg.TTS.Piper.Enabled,
			"piper_tts":         cfg.TTS.Piper.Enabled,
			// extended integrations
			"n8n":              cfg.N8n.Enabled,
			"fritzbox":         cfg.FritzBox.Enabled,
			"meshcentral":      cfg.MeshCentral.Enabled,
			"a2a":              cfg.A2A.Server.Enabled,
			"adguard":          cfg.AdGuard.Enabled,
			"s3":               cfg.S3.Enabled,
			"mcp":              cfg.MCP.Enabled,
			"mcp_server":       cfg.MCPServer.Enabled,
			"memory_analysis":  cfg.MemoryAnalysis.Enabled,
			"llm_guardian":     cfg.LLMGuardian.Enabled,
			"security_proxy":   cfg.SecurityProxy.Enabled,
			"sandbox":          cfg.Sandbox.Enabled,
			"ai_gateway":       cfg.AIGateway.Enabled,
			"image_generation": cfg.ImageGeneration.Enabled,
			"google_workspace": cfg.GoogleWorkspace.Enabled,
			"netlify":          cfg.Netlify.Enabled,
			"homepage":         cfg.Homepage.Enabled,
			"virustotal":       cfg.VirusTotal.Enabled,
			"brave_search":     cfg.BraveSearch.Enabled,
			"firewall":         cfg.Firewall.Enabled,
			"remote_control":   cfg.RemoteControl.Enabled,
			"web_scraper":      cfg.Tools.WebScraper.Enabled,
			"skill_manager":    cfg.Tools.SkillManager.Enabled,
		}

		// ── Missions Summary ──────────────────────────────────
		missionsSummary := map[string]interface{}{
			"total": 0, "enabled": 0, "running": 0, "queued": 0,
		}
		if s.MissionManagerV2 != nil {
			missions := s.MissionManagerV2.List()
			enabledCount := 0
			runningCount := 0
			for _, m := range missions {
				if m.Enabled {
					enabledCount++
				}
				if m.Status == "running" {
					runningCount++
				}
			}
			queue, runningID := s.MissionManagerV2.GetQueue()
			queueLen := 0
			if queue != nil {
				queueLen = len(queue.List())
			}
			if runningID != "" {
				runningCount = 1
			}
			missionsSummary = map[string]interface{}{
				"total":   len(missions),
				"enabled": enabledCount,
				"running": runningCount,
				"queued":  queueLen,
			}
		}

		// ── Invasion Summary ──────────────────────────────────
		invasionSummary := map[string]interface{}{
			"nests": 0, "eggs": 0, "connected_eggs": 0, "connected_nests": []string{},
		}
		if s.InvasionDB != nil {
			nests, _ := invasion.ListNests(s.InvasionDB)
			eggs, _ := invasion.ListEggs(s.InvasionDB)
			invasionSummary["nests"] = len(nests)
			invasionSummary["eggs"] = len(eggs)
		}
		if s.EggHub != nil {
			invasionSummary["connected_eggs"] = s.EggHub.ConnectionCount()
			invasionSummary["connected_nests"] = s.EggHub.ConnectedNests()
		}

		// ── Indexer Status ────────────────────────────────────
		indexerStatus := map[string]interface{}{
			"enabled": false,
		}
		if s.FileIndexer != nil {
			status := s.FileIndexer.Status()
			indexerStatus = map[string]interface{}{
				"enabled":       true,
				"running":       status.Running,
				"total_files":   status.TotalFiles,
				"indexed_files": status.IndexedFiles,
				"last_scan_at":  status.LastScanAt,
			}
		}

		// ── Devices Count ─────────────────────────────────────
		deviceCount := 0
		if s.InventoryDB != nil {
			devices, err := inventory.ListAllDevices(s.InventoryDB)
			if err == nil {
				deviceCount = len(devices)
			}
		}

		// ── MQTT Status ───────────────────────────────────────
		mqttStatus := map[string]interface{}{
			"enabled":   cfg.MQTT.Enabled,
			"connected": false,
			"buffer":    0,
		}
		if cfg.MQTT.Enabled {
			mqttStatus["connected"] = mqtt.IsConnected()
			mqttStatus["buffer"] = mqtt.BufferLen()
		}

		// ── Notes Summary ─────────────────────────────────────
		notesSummary := map[string]interface{}{
			"total": 0, "open": 0, "done": 0,
		}
		if s.ShortTermMem != nil {
			allNotes, err := s.ShortTermMem.ListNotes("", -1)
			if err == nil {
				open := 0
				done := 0
				for _, n := range allNotes {
					if n.Done {
						done++
					} else {
						open++
					}
				}
				notesSummary = map[string]interface{}{
					"total": len(allNotes),
					"open":  open,
					"done":  done,
				}
			}
		}

		// ── Security Summary ──────────────────────────────────
		securitySummary := map[string]interface{}{
			"vault_keys": 0, "tokens": 0,
		}
		if s.Vault != nil {
			keys, err := s.Vault.ListKeys()
			if err == nil {
				securitySummary["vault_keys"] = len(keys)
			}
		}
		if s.TokenManager != nil {
			securitySummary["tokens"] = s.TokenManager.Count()
		}

		// ── Context Summary ───────────────────────────────────
		contextSummary := map[string]interface{}{
			"total_chars": 0, "has_summary": false,
		}
		if s.HistoryManager != nil {
			contextSummary["total_chars"] = s.HistoryManager.TotalChars()
			contextSummary["has_summary"] = s.HistoryManager.GetSummary() != ""
		}

		// ── Last Activity ─────────────────────────────────────
		lastActivityHours := float64(-1)
		if s.ShortTermMem != nil {
			h, err := s.ShortTermMem.GetHoursSinceLastUserMessage("")
			if err == nil {
				lastActivityHours = h
			}
		}

		// ── Cheat Sheets Summary ──────────────────────────────
		cheatsheetsSummary := map[string]interface{}{
			"total": 0, "active": 0,
		}
		if s.CheatsheetDB != nil {
			total, active, _ := tools.CheatsheetCount(s.CheatsheetDB)
			cheatsheetsSummary["total"] = total
			cheatsheetsSummary["active"] = active
		}

		// ── Tunnel Status ─────────────────────────────────────
		tunnelInfo := map[string]interface{}{
			"running": tools.IsTunnelRunning(),
		}
		if url := tools.GetTunnelURL(); url != "" {
			tunnelInfo["url"] = url
		}

		// ── Skills Summary ────────────────────────────────────
		skillsSummary := map[string]interface{}{
			"total": 0, "agent": 0, "user": 0, "pending": 0,
		}
		if s.SkillManager != nil {
			total, agentN, userN, pending, err := s.SkillManager.GetStats()
			if err == nil {
				skillsSummary = map[string]interface{}{
					"total": total, "agent": agentN, "user": userN, "pending": pending,
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"agent":               agentInfo,
			"integrations":        integrations,
			"missions":            missionsSummary,
			"invasion":            invasionSummary,
			"indexer":             indexerStatus,
			"devices":             deviceCount,
			"mqtt":                mqttStatus,
			"notes":               notesSummary,
			"security":            securitySummary,
			"context":             contextSummary,
			"last_activity_hours": lastActivityHours,
			"cheatsheets":         cheatsheetsSummary,
			"tunnel":              tunnelInfo,
			"skills":              skillsSummary,
		})
	}
}

// handleDashboardNotes returns all notes as a JSON array, with optional filtering.
// Query params: ?category=xxx&done=0|1|-1 (default: all)
func handleDashboardNotes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		category := r.URL.Query().Get("category")
		doneFilter := -1
		if d := r.URL.Query().Get("done"); d != "" {
			if parsed, err := strconv.Atoi(d); err == nil {
				doneFilter = parsed
			}
		}

		notes, err := s.ShortTermMem.ListNotes(category, doneFilter)
		if err != nil {
			s.Logger.Error("Failed to list notes", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if notes == nil {
			notes = []memory.Note{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"notes": notes,
			"count": len(notes),
		})
	}
}

// tailFile reads the last N lines from a file efficiently.
func tailFile(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read all lines (aurago.log is truncated on restart, so it's bounded)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024) // 256KB max line
	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return last N lines
	if len(allLines) <= n {
		return allLines, nil
	}
	return allLines[len(allLines)-n:], nil
}

// handleDashboardGuardian returns LLM Guardian metrics for the dashboard card.
func handleDashboardGuardian(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		enabled := s.Cfg.LLMGuardian.Enabled
		level := s.Cfg.LLMGuardian.DefaultLevel
		failSafe := s.Cfg.LLMGuardian.FailSafe
		s.CfgMu.RUnlock()

		response := map[string]interface{}{
			"enabled":   enabled,
			"level":     level,
			"fail_safe": failSafe,
		}

		if s.LLMGuardian != nil && s.LLMGuardian.Metrics != nil {
			snap := s.LLMGuardian.Metrics.Snapshot()
			response["metrics"] = snap
		} else {
			response["metrics"] = nil
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleDashboardJournal returns recent journal entries as JSON.
// Query params: ?from=YYYY-MM-DD&to=YYYY-MM-DD&type=xxx&limit=20
func handleDashboardJournal(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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

// handleDashboardJournalStats returns journal statistics.
// Query params: ?from=YYYY-MM-DD&to=YYYY-MM-DD
func handleDashboardJournalStats(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")

		stats, err := s.ShortTermMem.GetJournalStats(from, to)
		if err != nil {
			s.Logger.Error("Failed to get journal stats", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
