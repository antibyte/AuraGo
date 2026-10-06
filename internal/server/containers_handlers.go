package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// handleContainersList returns all containers (GET /api/containers) with the
// protection flags of adminContainerListJSON. A Docker failure answers 502.
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
		writeContainerToolResult(w, adminContainerListJSON(r.Context(), s, cfg))
	}
}

// handleContainerAction routes /api/containers/{id}/{action} requests.
// Terminal, update and remove pass containerActionAllowed first. Start, stop,
// restart, pause, unpause, logs, inspect and stats never consult container
// protection: System World calls this handler for start/stop/restart, and
// restarting the AuraGo container itself works because dockerd performs the
// restart. Tool errors answer 502 with their unchanged JSON body.
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
			writeContainerToolResult(w, tools.DockerContainerAction(cfg, containerID, "start", false))

		case "stop":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			writeContainerToolResult(w, tools.DockerContainerAction(cfg, containerID, "stop", false))

		case "restart":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			writeContainerToolResult(w, tools.DockerContainerAction(cfg, containerID, "restart", false))

		case "pause", "unpause":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			writeContainerToolResult(w, tools.DockerContainerAction(cfg, containerID, action, false))

		case "update":
			if r.Method != http.MethodPost {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			if !containerActionAllowed(s, cfg, containerID, "update", w, r) {
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
			defer cancel()
			writeContainerToolResult(w, tools.DockerUpdateContainerImage(ctx, cfg, containerID, s.Logger))

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
			writeContainerToolResult(w, tools.DockerContainerLogs(cfg, containerID, tail))

		case "inspect":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			writeContainerToolResult(w, tools.DockerInspectContainer(cfg, containerID))

		case "stats":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			writeContainerToolResult(w, tools.DockerStats(cfg, containerID))

		case "terminal":
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			// Refuse a cross-origin handshake before any Docker request or DNS
			// lookup; handleContainerTerminal checks it again.
			if !sameOriginOrNoOrigin(r) {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "forbidden websocket origin"})
				return
			}
			// Likewise a request that is not a WebSocket upgrade: the protection
			// lookup inspects the target and resolves the Docker endpoint, and a
			// plain GET has no use for either.
			if rejectNonWebSocketTerminalRequest(w, r) {
				return
			}
			if !containerActionAllowed(s, cfg, containerID, "terminal", w, r) {
				return
			}
			handleContainerTerminal(s, cfg, containerID, w, r)

		case "protection":
			// Read-only report of the terminal/update/remove rules for this
			// container. Browsers hide the HTTP answer of a refused WebSocket
			// handshake; the Containers page asks here and then offers the
			// confirmation. It never starts a shell.
			if r.Method != http.MethodGet {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			p := containerProtectionFor(r.Context(), s, cfg, containerID)
			report := map[string]interface{}{
				"status":             "ok",
				"container_id":       containerID,
				"owner":              p.label(),
				"protected":          p.protected(),
				"update_unsupported": p.updateCannotComplete(),
				"read_only":          readOnly,
			}
			if p.protected() {
				report["message"] = p.confirmationMessage()
			}
			containerJSON(w, http.StatusOK, report)

		case "": // DELETE /api/containers/{id} — remove container
			if r.Method != http.MethodDelete {
				jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if readOnly {
				containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
				return
			}
			if !containerActionAllowed(s, cfg, containerID, "remove", w, r) {
				return
			}
			force := r.URL.Query().Get("force") == "true"
			writeContainerToolResult(w, tools.DockerContainerAction(cfg, containerID, "remove", force))

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

// writeContainerToolResult writes a tools.Docker* JSON result. An error result
// keeps its body and gets 502 Bad Gateway: Docker or the tool layer refused the
// request. Never 401: the shared fetch wrapper treats 401 as an expired login.
func writeContainerToolResult(w http.ResponseWriter, result string) {
	status := http.StatusOK
	var envelope struct {
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(result), &envelope) == nil && envelope.Status == "error" {
		status = http.StatusBadGateway
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(result))
}
