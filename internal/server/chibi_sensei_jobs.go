package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"aurago/internal/chibisensei"
)

// chibiEventHeartbeat is the interval of the SSE comment ping.
const chibiEventHeartbeat = 15 * time.Second

// chibiEventWriteTimeout is how long one write of the stream may take before the stream ends (a variable so a
// test can shorten it).
var chibiEventWriteTimeout = 30 * time.Second

func chibiJobNotFound(w http.ResponseWriter) {
	writeChibiError(w, http.StatusNotFound, "job_not_found", "Job not found.", nil)
}

func handleChibiJob(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	job, err := acc.rt.store.GetJob(r.Context(), jobID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiJobNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleChibiProjectJobs lists the newest jobs of a project; ?active=1 only the unfinished ones.
func handleChibiProjectJobs(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	jobs, err := acc.rt.store.ListJobs(r.Context(), id, r.URL.Query().Get("active") == "1")
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

// handleChibiJobCancel is a stop operation: it works while the desktop is read-only.
func handleChibiJobCancel(s *Server, w http.ResponseWriter, r *http.Request, jobID string) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopStop, false)
	if !ok {
		return
	}
	job, err := acc.rt.jobs.Cancel(r.Context(), jobID)
	if errors.Is(err, chibisensei.ErrNotFound) {
		chibiJobNotFound(w)
		return
	}
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	writeChibiJSON(w, http.StatusOK, map[string]any{"job": job})
}

// handleChibiEvents streams the events of a project as server-sent events: on connect one "job" event
// per unfinished job (oldest first), the current "rows" revision and, when the project has a non-terminal run, that
// run as a "run" event (RunSummary); then every hub event (job, rows, checks, run, metric, stage), with a comment ping every 15 s. The stream ends when the client leaves, the
// runtime is retired (shutdown, data directory change) or the server stops.
func handleChibiEvents(s *Server, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeChibiMethodError(w, http.MethodGet)
		return
	}
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeRead, desktopRead, false)
	if !ok {
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		writeChibiError(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming is not supported.", nil)
		return
	}
	ctx := r.Context()
	events, unsubscribe, err := acc.rt.hub.Subscribe(id)
	if err != nil {
		writeChibiError(w, http.StatusTooManyRequests, "too_many_streams", "Too many open event streams.", nil)
		return
	}
	defer unsubscribe()
	project, err := acc.rt.store.GetProject(ctx, id)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	active, err := acc.rt.store.ListJobs(ctx, id, true)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	// A stream outlives the server's usual window, but every write gets its own deadline: a dead connection
	// ends the stream at the next write and frees its slot.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	arm := func() { _ = rc.SetWriteDeadline(time.Now().Add(chibiEventWriteTimeout)) }
	flush := func() bool { return rc.Flush() == nil }
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-store")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")

	send := func(name string, data any) bool {
		raw, err := json.Marshal(data)
		if err != nil {
			return true
		}
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, raw)
		return err == nil
	}
	arm()
	if _, err := io.WriteString(w, ": chibi-sensei events\n\n"); err != nil {
		return
	}
	for i := len(active) - 1; i >= 0; i-- { // ListJobs is newest first
		if !send("job", active[i]) {
			return
		}
	}
	if !send("rows", map[string]any{"data_rev": project.DataRev}) {
		return
	}
	if runs, err := acc.rt.store.ActiveRuns(ctx); err == nil {
		for _, run := range runs {
			if run.ProjectID == id && !send("run", run.Summary()) {
				return
			}
		}
	}
	if !flush() {
		return
	}

	var serverDone <-chan struct{}
	if s.integrationCtx != nil {
		serverDone = s.integrationCtx.Done()
	}
	heartbeat := time.NewTicker(chibiEventHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-acc.rt.retiring:
			return
		case <-serverDone:
			return
		case ev, open := <-events:
			if !open {
				return
			}
			arm()
			if !send(ev.Name, ev.Data) || !flush() {
				return
			}
		case <-heartbeat.C:
			arm()
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil || !flush() {
				return
			}
		}
	}
}
