package server

import (
	"encoding/json"
	"net/http"
	"strings"

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
		sessions, err := s.ShortTermMem.ListChatSessions()
		if err != nil {
			s.Logger.Error("Failed to list chat sessions", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if sessions == nil {
			sessions = []memory.ChatSession{}
		}
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
		sess, err := s.ShortTermMem.CreateChatSession()
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"session":  sess,
			"messages": messages,
		})
	}
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
		if err := s.ShortTermMem.DeleteChatSession(sessionID); err != nil {
			s.Logger.Error("Failed to delete chat session", "error", err)
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}
}
