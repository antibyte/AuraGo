package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aurago/internal/tools"
)

// ── Containers API Handlers ─────────────────────────────────────────────────
// Provides REST endpoints for the /containers UI page.
// Wraps existing Docker tool functions with HTTP guards for enabled/read-only.

// containerDockerConfig builds a tools.DockerConfig from current server config.
func containerDockerConfig(s *Server) (tools.DockerConfig, bool, bool) {
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return tools.DockerConfig{Host: s.Cfg.Docker.Host}, s.Cfg.Docker.Enabled, s.Cfg.Docker.ReadOnly
}

// handleContainersList returns all containers (GET /api/containers).
func handleContainersList(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cfg, enabled, _ := containerDockerConfig(s)
		if !enabled {
			containerJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "Docker is not enabled"})
			return
		}
		result := tools.DockerListContainers(cfg, true)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(result))
	}
}

// handleContainerAction routes /api/containers/{id}/{action} requests.
func handleContainerAction(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, enabled, readOnly := containerDockerConfig(s)
		if !enabled {
			containerJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "Docker is not enabled"})
			return
		}

		// Parse path: /api/containers/{id}/{action_or_resource}
		path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 1 || parts[0] == "" {
			containerJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "container ID required"})
			return
		}
		containerID := parts[0]
		action := ""
		if len(parts) == 2 {
			action = parts[1]
		}

		switch action {
		case "start":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			result := tools.DockerContainerAction(cfg, containerID, "start", false)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

		case "stop":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			result := tools.DockerContainerAction(cfg, containerID, "stop", false)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

		case "restart":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			result := tools.DockerContainerAction(cfg, containerID, "restart", false)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

		case "logs":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			tail := 200
			if t := r.URL.Query().Get("tail"); t != "" {
				if v, err := strconv.Atoi(t); err == nil && v > 0 && v <= 5000 {
					tail = v
				}
			}
			result := tools.DockerContainerLogs(cfg, containerID, tail)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

		case "inspect":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			result := tools.DockerInspectContainer(cfg, containerID)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

		case "stats":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			result := tools.DockerStats(cfg, containerID)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

		case "": // DELETE /api/containers/{id} — remove container
			if r.Method != http.MethodDelete {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			force := r.URL.Query().Get("force") == "true"
			result := tools.DockerContainerAction(cfg, containerID, "remove", force)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(result))

		default:
			containerJSON(w, http.StatusNotFound, map[string]string{"status": "error", "message": "unknown action: " + action})
		}
	}
}

// containerJSON is a helper for writing JSON responses with a status code.
func containerJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
