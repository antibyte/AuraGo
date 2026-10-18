package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"aurago/internal/memory"
)

// handleListChatSessions returns the list of recent chat sessions.
// GET /api/chat/sessions
func handleListChatSessions(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		retainLimit := s.Cfg.Consolidation.ChatSessionLimit
		sessions, err := s.ShortTermMem.ListChatSessionsWithLimit(retainLimit)
		if err != nil {
			s.Logger.Error("Failed to list chat sessions", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if sessions == nil {
			sessions = []memory.ChatSession{}
		}
		sessions = sanitizeVisibleChatSessions(sessions)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"sessions": sessions,
		})
	}
}

// handleCreateChatSession creates a new chat session.
// POST /api/chat/sessions
func handleCreateChatSession(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		retainLimit := s.Cfg.Consolidation.ChatSessionLimit
		sess, err := s.ShortTermMem.CreateChatSessionWithLimit(retainLimit)
		if err != nil {
			s.Logger.Error("Failed to create chat session", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"session": sess,
		})
	}
}

// handleGetChatSession returns the messages for a specific chat session.
// GET /api/chat/sessions/{id}
func handleGetChatSession(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
		if sessionID == "" {
			jsonError(w, "Missing session ID", http.StatusBadRequest)
			return
		}

		// Verify session exists
		sess, err := s.ShortTermMem.GetChatSession(sessionID)
		if err != nil {
			s.Logger.Error("Failed to get chat session", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if sess == nil {
			jsonError(w, "Session not found", http.StatusNotFound)
			return
		}

		// Get messages for this session (already filtered: no internal messages)
		messages, err := s.ShortTermMem.GetSessionMessages(sessionID)
		if err != nil {
			s.Logger.Error("Failed to get session messages", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Update last_active_at when user views a session
		_ = s.ShortTermMem.TouchChatSession(sessionID)

		if messages == nil {
			messages = []memory.HistoryMessage{}
		}
		messages = sanitizeVisibleChatHistoryMessages(messages)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"session":  sess,
			"messages": messages,
		})
	}
}

func sanitizeVisibleChatSessions(sessions []memory.ChatSession) []memory.ChatSession {
	if len(sessions) == 0 {
		return sessions
	}
	out := append([]memory.ChatSession(nil), sessions...)
	for i := range out {
		out[i].Preview = stripAgodeskAttachmentBlock(out[i].Preview)
	}
	return out
}

func sanitizeVisibleChatHistoryMessages(messages []memory.HistoryMessage) []memory.HistoryMessage {
	if len(messages) == 0 {
		return messages
	}
	out := append([]memory.HistoryMessage(nil), messages...)
	for i := range out {
		content := stripAgodeskAttachmentBlock(out[i].Content)
		if content == out[i].Content {
			continue
		}
		out[i].Content = content
		out[i].MultiContent = nil
		out[i].ChatCompletionMessage.Content = content
		out[i].ChatCompletionMessage.MultiContent = nil
	}
	return out
}

// handleDeleteChatSession deletes a specific chat session.
// DELETE /api/chat/sessions/{id}
func handleDeleteChatSession(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessionID := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
		if sessionID == "" {
			jsonError(w, "Missing session ID", http.StatusBadRequest)
			return
		}
		deleteMemories := r.URL.Query().Get("delete_memories") == "true"
		job, prepareErr := s.ShortTermMem.PrepareChatPrivacyDeletion(r.Context(), sessionID, s.LongTermMem, s.KG, deleteMemories)
		if job.DeletionJobID == "" {
			status := http.StatusInternalServerError
			code := "deletion_prepare_failed"
			if prepareErr != nil && strings.Contains(strings.ToLower(prepareErr.Error()), "lock") {
				status = http.StatusConflict
				code = "deletion_in_progress"
			}
			if s.Logger != nil {
				s.Logger.Warn("Failed to prepare privacy deletion", "error_code", code)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status": "error", "code": code,
				"deletion": memory.SessionDeletionReport{TranscriptDeleted: false},
			})
			return
		}
		if prepareErr == nil {
			// The bounded worker may encounter an unrelated older job first. The
			// response below is derived from this request's persisted job only.
			_ = s.ShortTermMem.ProcessDuePrivacyDeletionJobs(r.Context(), s.LongTermMem, s.KG, time.Now().UTC(), 1)
		}
		if current, err := s.ShortTermMem.GetPrivacyDeletionJob(r.Context(), job.DeletionJobID); err == nil {
			job = current
		}
		report := memory.SessionDeletionReport{
			TranscriptDeleted: job.TranscriptDeleted,
			DeletedMemories:   job.DeletedCount, RetainedMemories: job.RetainedCount,
		}
		w.Header().Set("Content-Type", "application/json")
		statusText, statusCode := "ok", http.StatusOK
		switch job.DeletionStatus {
		case memory.PrivacyDeletionPreparing, memory.PrivacyDeletionAwaitingContactDelete, memory.PrivacyDeletionPending, memory.PrivacyDeletionProcessing, memory.PrivacyDeletionRetryWait, memory.PrivacyDeletionReconcilePending:
			statusText, statusCode = "partial", http.StatusMultiStatus
		}
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": statusText, "code": func() string {
				if statusText == "partial" {
					return "deletion_incomplete"
				}
				return ""
			}(),
			"deletion": report, "deletion_job_id": job.DeletionJobID, "deletion_status": job.DeletionStatus,
			"pending_count": job.PendingCount, "deleted_count": job.DeletedCount, "retained_count": job.RetainedCount,
		})
	}
}
