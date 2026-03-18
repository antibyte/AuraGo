package server

import (
	"aurago/internal/sandbox"
	"aurago/internal/tools"
	"encoding/json"
	"net/http"
)

// handleSandboxStatus returns the sandbox readiness status as JSON.
func handleSandboxStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		status := tools.SandboxGetStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

// handleShellSandboxStatus returns the shell sandbox capabilities and status.
func handleShellSandboxStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		caps := sandbox.GetCapabilities()
		sb := sandbox.Get()

		resp := map[string]interface{}{
			"enabled":      s.Cfg.ShellSandbox.Enabled,
			"available":    sb.Available(),
			"backend":      sb.Name(),
			"landlock_abi": caps.LandlockABI,
			"in_docker":    caps.InDocker,
			"kernel":       caps.KernelVersion,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
