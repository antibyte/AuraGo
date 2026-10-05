package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aurago/internal/flows"
	"aurago/internal/security"
)

type flowTestBody struct {
	TriggerNode  string         `json:"trigger_node"`
	TriggerData  map[string]any `json:"trigger_data"`
	OnlyNode     string         `json:"only_node"`
	RememberData bool           `json:"remember_data"`
}

// flowRunAction serves the run routes below /api/desktop/flows/{id}/ and reports false for
// unknown actions.
func (s *Server) flowRunAction(w http.ResponseWriter, r *http.Request, id, action string, rest []string) bool {
	ctx := r.Context()
	switch {
	case action == "test" && len(rest) == 0:
		if r.Method != http.MethodPost {
			flowsMethodNotAllowed(w)
			return true
		}
		var body flowTestBody
		if r.ContentLength != 0 && !flowsDecode(w, r, &body, flowsDocBodyLimit) {
			return true
		}
		res, err := s.Flows.StartTestRun(ctx, id, flows.TestRunRequest{TriggerNode: body.TriggerNode,
			TriggerData: body.TriggerData, OnlyNode: body.OnlyNode, RememberData: body.RememberData})
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return true
		}
		flowsJSON(w, http.StatusAccepted, res)
	case action == "run" && len(rest) == 0:
		if r.Method != http.MethodPost {
			flowsMethodNotAllowed(w)
			return true
		}
		res, err := s.Flows.RunNow(ctx, id)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return true
		}
		flowsJSON(w, http.StatusAccepted, res)
	case action == "runs" && len(rest) == 0:
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return true
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		runs, err := s.Flows.Runs(ctx, id, flows.RunFilter{Mode: flows.RunMode(q.Get("mode")),
			Status: flows.RunStatus(q.Get("status")), Limit: limit, Offset: offset})
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return true
		}
		s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"runs": runs})
	case action == "test-data" && len(rest) == 1:
		node := rest[0]
		switch r.Method {
		case http.MethodGet:
			data, err := s.Flows.TriggerSampleData(ctx, id, node)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return true
			}
			flowsJSON(w, http.StatusOK, map[string]any{"data": data})
		case http.MethodPut:
			var body struct {
				Data map[string]any `json:"data"`
			}
			if !flowsDecode(w, r, &body, flowsDocBodyLimit) {
				return true
			}
			if body.Data == nil {
				body.Data = map[string]any{}
			}
			if err := s.Flows.SaveTriggerSample(ctx, id, node, body.Data); err != nil {
				s.flowsErrorFrom(w, r, err)
				return true
			}
			flowsJSON(w, http.StatusOK, map[string]any{"data": body.Data})
		default:
			flowsMethodNotAllowed(w)
		}
	default:
		return false
	}
	return true
}

// flowsRunRoute serves /api/desktop/flows/runs/{run}[/events|/cancel].
func (s *Server) flowsRunRoute(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 || rest[0] == "" {
		flowsError(w, http.StatusNotFound, "FLOW_RUN_NOT_FOUND", "a run id is required")
		return
	}
	runID := rest[0]
	switch {
	case len(rest) == 1:
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		detail, err := s.Flows.Run(r.Context(), runID, r.URL.Query().Get("include") == "doc")
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.flowsJSONScrubbed(w, http.StatusOK, detail)
	case len(rest) == 2 && rest[1] == "events":
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		s.streamFlowRunEvents(w, r, runID)
	case len(rest) == 2 && rest[1] == "cancel":
		if r.Method != http.MethodPost {
			flowsMethodNotAllowed(w)
			return
		}
		if !s.Flows.Cancel(runID) {
			flowsError(w, http.StatusNotFound, "FLOW_RUN_NOT_FOUND", "the run is not active")
			return
		}
		flowsJSON(w, http.StatusAccepted, map[string]bool{"cancelled": true})
	default:
		flowsError(w, http.StatusNotFound, "FLOW_RUN_NOT_FOUND", "unknown run route")
	}
}

// streamFlowRunEvents sends a snapshot, then every bus event after ?after=, then "end".
func (s *Server) streamFlowRunEvents(w http.ResponseWriter, r *http.Request, runID string) {
	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	detail, err := s.Flows.Run(r.Context(), runID, false)
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	backlog, events, unsubscribe, live := s.Flows.Subscribe(runID, after)
	if live && unsubscribe != nil {
		defer unsubscribe()
	}
	if writeFlowSSE(w, flusher, "snapshot", 0, detail) != nil {
		return
	}
	for _, ev := range backlog {
		if writeFlowSSE(w, flusher, "event", ev.Seq, ev) != nil {
			return
		}
	}
	if !live || events == nil {
		_ = writeFlowSSE(w, flusher, "end", 0, map[string]any{})
		return
	}
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ":heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, open := <-events:
			if !open {
				_ = writeFlowSSE(w, flusher, "end", 0, map[string]any{})
				return
			}
			if writeFlowSSE(w, flusher, "event", ev.Seq, ev) != nil {
				return
			}
		}
	}
}

func writeFlowSSE(w http.ResponseWriter, flusher http.Flusher, event string, id int, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var b strings.Builder
	if id > 0 {
		fmt.Fprintf(&b, "id: %d\n", id)
	}
	fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", event, security.Scrub(string(data)))
	if _, err := io.WriteString(w, b.String()); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
