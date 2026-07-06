package server

import (
	"net/http"
	"strings"

	"aurago/internal/agent"
)

func handleClearChat(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		authEnabled := s.Cfg.Auth.Enabled
		sessionSecret := s.Cfg.Auth.SessionSecret
		s.CfgMu.RUnlock()
		if authEnabled && !IsAuthenticated(r, sessionSecret) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","redirect":"/auth/login"}`))
			return
		}

		sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
		if sessionID != "" && sessionID != "default" {
			if err := s.ShortTermMem.ClearSession(sessionID); err != nil {
				s.Logger.Error("Failed to clear session", "session_id", sessionID, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			agent.ClearDiscoverToolsState(sessionID)
			agent.ResetInnerVoiceSession(sessionID)
			w.WriteHeader(http.StatusOK)
			return
		}

		if err := s.HistoryManager.Clear(); err != nil {
			s.Logger.Error("Failed to clear chat history", "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		agent.ResetInnerVoiceSession("default")
		w.WriteHeader(http.StatusOK)
	}
}
