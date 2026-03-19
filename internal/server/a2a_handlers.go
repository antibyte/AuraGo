package server

import (
	"encoding/json"
	"net/http"

	a2apkg "aurago/internal/a2a"
)

// handleA2AStatus returns the current A2A server and client status.
func handleA2AStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := map[string]interface{}{
			"server_enabled": s.Cfg.A2A.Server.Enabled,
			"client_enabled": s.Cfg.A2A.Client.Enabled,
		}

		if s.A2AServer != nil {
			status["server"] = s.A2AServer.Status()
		}
		if s.A2AClientMgr != nil {
			status["remote_agents"] = s.A2AClientMgr.ListRemoteAgents()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

// handleA2ARemoteAgents returns the list of configured remote A2A agents and their status.
func handleA2ARemoteAgents(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var agents []a2apkg.RemoteAgentStatus
		if s.A2AClientMgr != nil {
			agents = s.A2AClientMgr.ListRemoteAgents()
		}
		if agents == nil {
			agents = []a2apkg.RemoteAgentStatus{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
	}
}

// handleA2ACard returns the current agent card.
func handleA2ACard(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.A2AServer == nil {
			http.Error(w, "A2A server not enabled", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.A2AServer.Card())
	}
}

// handleA2ATest tests the A2A server by returning basic connectivity info.
func handleA2ATest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		result := map[string]interface{}{
			"status": "ok",
		}

		if s.A2AServer != nil {
			result["server"] = s.A2AServer.Status()
		} else {
			result["server"] = map[string]interface{}{"enabled": false}
		}

		if s.A2AClientMgr != nil {
			result["remote_agents"] = s.A2AClientMgr.ListRemoteAgents()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// handleA2ARemoteAgentTest tests connectivity to a specific remote A2A agent.
func handleA2ARemoteAgentTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if s.A2AClientMgr == nil {
			http.Error(w, "A2A client not enabled", http.StatusNotFound)
			return
		}

		status, ok := s.A2AClientMgr.GetRemoteAgentStatus(req.AgentID)
		if !ok {
			http.Error(w, "Remote agent not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}
