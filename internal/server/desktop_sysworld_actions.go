package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"
)

type worldActionResult struct {
	Entity string `json:"entity"`
	Action string `json:"action"`
	Status string `json:"status"`
	At     int64  `json:"at"`
	code   int
}

// Small in-process response adapter keeps the existing service handlers and
// their enabled/read-only guards authoritative. It never makes a loopback request.
type worldActionWriter struct {
	header http.Header
	body   bytes.Buffer
	code   int
}

func (w *worldActionWriter) Header() http.Header { return w.header }
func (w *worldActionWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}
func (w *worldActionWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = 200
	}
	if w.body.Len()+len(p) <= 64*1024 {
		return w.body.Write(p)
	}
	return len(p), nil
}

func handleSystemWorldAction(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		cfg := s.ConfigSnapshot()
		if !cfg.VirtualDesktop.Enabled || cfg.VirtualDesktop.ReadOnly {
			jsonError(w, "System World actions unavailable", 403)
			return
		}
		var req struct {
			Entity    string `json:"entity"`
			Action    string `json:"action"`
			RequestID string `json:"request_id"`
			Confirmed bool   `json:"confirmed"`
		}
		if decodeDesktopJSON(w, r, &req, 2048) != nil || !worldID.MatchString(req.Entity) || !worldID.MatchString(req.RequestID) || len(req.RequestID) < 16 {
			jsonError(w, "Invalid System World action", 400)
			return
		}
		if !slices.Contains([]string{"start", "stop", "restart", "cancel"}, req.Action) {
			jsonError(w, "Action is not supported", 400)
			return
		}
		if req.Action != "start" && !req.Confirmed {
			jsonError(w, "Confirm the selected target", 400)
			return
		}
		runtime := s.worldRuntime()
		if !runtime.actionMu.TryLock() {
			jsonError(w, "A System World action is already pending", 409)
			return
		}
		defer runtime.actionMu.Unlock()
		now := time.Now().UnixMilli()
		for id, v := range runtime.requests {
			if now-v.At > 300000 {
				delete(runtime.requests, id)
			}
		}
		respond := func(v worldActionResult) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(v.code)
			_ = json.NewEncoder(w).Encode(v)
		}
		if old, ok := runtime.requests[req.RequestID]; ok {
			if old.Entity != req.Entity || old.Action != req.Action {
				jsonError(w, "Request ID already used", 409)
				return
			}
			respond(old)
			return
		}
		if len(runtime.requests) >= 1024 {
			jsonError(w, "Too many recent actions", 429)
			return
		}
		runtime.mu.Lock()
		entity, ok := runtime.entities[req.Entity]
		runtime.mu.Unlock()
		if !ok || !slices.Contains(s.worldCapabilities(entity), req.Action) {
			jsonError(w, "Action unavailable for this target", 409)
			return
		}
		id := strings.TrimPrefix(entity.ID, entity.Kind+":")
		clone := r.Clone(r.Context())
		u := *r.URL
		clone.URL = &u
		capture := &worldActionWriter{header: http.Header{}}
		switch entity.Kind {
		case "container":
			clone.URL.Path = "/api/containers/" + id + "/" + req.Action
			handleContainerAction(s)(capture, clone)
		case "daemon":
			clone.URL.Path = "/api/daemons/" + id + "/" + req.Action
			handleDaemonAction(s)(capture, clone)
		case "mission":
			if req.Action == "start" {
				handleMissionRunV2(s, capture, clone, id)
			} else {
				handleMissionCancelV2(s, capture, clone, id)
			}
		default:
			jsonError(w, "Unsupported target", 400)
			return
		}
		result := worldActionResult{Entity: req.Entity, Action: req.Action, Status: "accepted", At: now, code: 202}
		var response struct {
			Status string `json:"status"`
		}
		if capture.code < 200 || capture.code >= 300 || json.Unmarshal(capture.body.Bytes(), &response) != nil || !slices.Contains([]string{"ok", "running", "queued", "cancelling", "started"}, response.Status) {
			result.Status = "failed"
			result.code = 409
		} else if entity.Kind == "container" && response.Status == "ok" {
			// Docker's synchronous action endpoint has acknowledged completion.
			result.Status = "completed"
			result.code = 200
		}
		// Acceptance is not a claim that a stopped process or cancelled mission has finished.
		runtime.requests[req.RequestID] = result
		respond(result)
	}
}
