package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"aurago/internal/agent"
	"aurago/internal/memory"
)

// handleDashboardMemory returns memory statistics (core memory, messages, vectordb, graph, milestones).
func handleDashboardMemory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.ShortTermMem == nil {
			jsonError(w, "Memory subsystem not available", http.StatusServiceUnavailable)
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
			graphNodes, graphEdges, _ = s.KG.Stats()
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
		pendingActions, _ := s.ShortTermMem.GetPendingEpisodicActionsForQuery("", 5)
		memoryConflicts, _ := s.ShortTermMem.GetOpenMemoryConflicts(5)
		memoryHealth := memory.MemoryHealthReport{
			Usage: usageStats,
		}
		if metas, err := s.ShortTermMem.GetAllMemoryMeta(1000, 0); err == nil {
			memoryHealth = memory.BuildMemoryHealthReport(metas, usageStats)
		}
		memoryStrategy := agent.BuildMemoryAnalysisDashboardState(s.Cfg, s.ShortTermMem)

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
			"journal_entries":  journalCount,
			"notes_count":      notesCount,
			"error_patterns":   errorPatternsCount,
			"milestones":       milestones,
			"episodic":         episodicStats,
			"pending_actions":  pendingActions,
			"memory_conflicts": memoryConflicts,
			"memory_health": map[string]interface{}{
				"usage":         memoryHealth.Usage,
				"confidence":    memoryHealth.Confidence,
				"effectiveness": memoryHealth.Effectiveness,
				"curator":       memoryHealth.Curator,
				"strategy":      memoryStrategy,
			},
		})
	}
}

// handleDashboardCoreMemory returns all core memory facts as a JSON array.
func handleDashboardCoreMemory(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rows, err := s.ShortTermMem.GetCoreMemoryFacts()
		if err != nil {
			s.Logger.Error("Failed to get core memory facts", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		entries, err := s.ShortTermMem.GetProfileEntries("")
		if err != nil {
			s.Logger.Error("Failed to get profile entries", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
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
				jsonError(w, "category and key are required", http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.DeleteProfileEntry(category, key); err != nil {
				s.Logger.Error("Failed to delete profile entry", "error", err)
				jsonError(w, "Not found", http.StatusNotFound)
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
				jsonError(w, "category, key, and value are required", http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.UpsertProfileEntry(body.Category, body.Key, body.Value, "manual"); err != nil {
				s.Logger.Error("Failed to update profile entry", "error", err)
				jsonError(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
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
			graphNodes, graphEdges, _ = s.KG.Stats()
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
				jsonError(w, `{"error":"fact is required"}`, http.StatusBadRequest)
				return
			}
			id, err := s.ShortTermMem.AddCoreMemoryFact(req.Fact)
			if err != nil {
				jsonError(w, `{"error":"Failed to add core memory fact"}`, http.StatusInternalServerError)
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
				jsonError(w, `{"error":"id and fact are required"}`, http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.UpdateCoreMemoryFact(req.ID, req.Fact); err != nil {
				jsonError(w, `{"error":"Failed to update core memory fact"}`, http.StatusInternalServerError)
				return
			}
			go pushMemoryStats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		case http.MethodDelete:
			var req struct {
				ID      int64  `json:"id"`
				All     bool   `json:"all"`
				Confirm string `json:"confirm"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				jsonError(w, `{"error":"invalid request"}`, http.StatusBadRequest)
				return
			}
			if req.All {
				if req.Confirm != "DELETE_ALL_CORE_MEMORY" {
					jsonError(w, `{"error":"confirmation token is required"}`, http.StatusBadRequest)
					return
				}
				deleted, err := s.ShortTermMem.DeleteAllCoreMemoryFacts()
				if err != nil {
					jsonError(w, `{"error":"Failed to delete all core memory facts"}`, http.StatusInternalServerError)
					return
				}
				go pushMemoryStats()
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "deleted": deleted})
				return
			}
			if req.ID == 0 {
				jsonError(w, `{"error":"id is required"}`, http.StatusBadRequest)
				return
			}
			if err := s.ShortTermMem.DeleteCoreMemoryFact(req.ID); err != nil {
				jsonError(w, `{"error":"Failed to delete core memory fact"}`, http.StatusInternalServerError)
				return
			}
			go pushMemoryStats()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDashboardNotes returns all notes as a JSON array, with optional filtering.
// Query params: ?category=xxx&done=0|1|-1 (default: all)
func handleDashboardNotes(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, "Internal server error", http.StatusInternalServerError)
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
