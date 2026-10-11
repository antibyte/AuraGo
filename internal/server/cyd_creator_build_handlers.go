package server

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"aurago/internal/cydcreator"
)

const (
	cydSSEPage = 500
	// cydSSEHeartbeat is the gap between ": ping" comments; tests shorten it.
	cydSSEHeartbeatDefault = 15 * time.Second
)

var cydSSEHeartbeat = cydSSEHeartbeatDefault

func (s *Server) cydStartBuild(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct{}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	build, err := s.CYDCreator.StartBuild(r.Context(), params["id"], cydcreator.BuildOptions{Origin: cydcreator.OriginUser})
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusAccepted, map[string]any{"build": build})
}

func (s *Server) cydListBuilds(w http.ResponseWriter, r *http.Request, params map[string]string) {
	builds, err := s.CYDCreator.ListBuilds(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"builds": builds})
}

func (s *Server) cydGetBuild(w http.ResponseWriter, r *http.Request, params map[string]string) {
	build, err := s.CYDCreator.GetBuild(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"build": build})
}

func (s *Server) cydCancelBuild(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct{}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	if err := s.CYDCreator.CancelBuild(r.Context(), params["id"]); err != nil {
		s.cydError(w, r, err)
		return
	}
	build, err := s.CYDCreator.GetBuild(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusAccepted, map[string]any{"build": build})
}

func (s *Server) cydBuildLog(w http.ResponseWriter, r *http.Request, params map[string]string) {
	text, err := s.CYDCreator.BuildLog(r.Context(), params["id"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, text)
	}
}

var cydArtifactTypes = map[string]string{
	cydcreator.ArtifactMergedBin:   "application/octet-stream",
	cydcreator.ArtifactFirmwareBin: "application/octet-stream",
	cydcreator.ArtifactResultJSON:  "application/json",
	cydcreator.ArtifactBuildLog:    "text/plain; charset=utf-8",
}

func (s *Server) cydArtifact(w http.ResponseWriter, r *http.Request, params map[string]string) {
	file, info, err := s.CYDCreator.OpenArtifact(r.Context(), params["id"], params["name"])
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	contentType := cydArtifactTypes[info.Name]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": params["id"] + "-" + info.Name}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-CYD-SHA256", info.SHA256)
	cydServeReader(w, r, info.Name, file)
}

func (s *Server) cydDecodeBacktrace(w http.ResponseWriter, r *http.Request, params map[string]string) {
	var body struct {
		Addresses []string `json:"addresses"`
	}
	if !cydDecodeJSON(w, r, cydSmallBodyLimit, &body) {
		return
	}
	frames, err := s.CYDCreator.DecodeBacktrace(r.Context(), params["id"], body.Addresses)
	if err != nil {
		s.cydError(w, r, err)
		return
	}
	cydWriteJSON(w, http.StatusOK, map[string]any{"frames": frames})
}

func writeCYDSSE(w io.Writer, ev cydcreator.Event) {
	if ev.Seq > 0 {
		_, _ = fmt.Fprintf(w, "id: %d\n", ev.Seq)
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, ev.Data)
}

// cydEventsPruned reports whether the project's persisted events after
// cursor (a seq the client already has) may be incomplete. Pruning deletes a
// project's oldest events, so when the oldest kept event is newer than
// cursor, the event at cursor is gone and events after it may be gone too
// (seq is global, so a gap alone proves nothing). Callers check after reading
// a page, which also catches a prune that ran while the page was read.
func cydEventsPruned(ctx context.Context, svc *cydcreator.Service, projectID string, cursor int64) (bool, error) {
	oldest, err := svc.EventsAfter(ctx, projectID, 0, 1)
	if err != nil {
		return false, err
	}
	return len(oldest) == 1 && oldest[0].Seq > cursor, nil
}

// cydEvents streams project events: the persisted backlog after
// max(?after=, Last-Event-ID) in pages of 500, then stream_ready (no id),
// then live events; ": ping" every 15 s. The subscription is taken before the
// backlog is read, so an event emitted in between reaches both and the live
// loop drops it by seq (ephemeral events, seq 0, are never dropped). When
// events after the resume point were pruned, stream_ready carries
// {"reload":true}: the client reloads the project state. A subscriber that
// falls behind is dropped by the service and a closed subscription (project
// deleted, service closed) ends the stream; the client reconnects with
// Last-Event-ID. The handler starts no goroutine and returns when the client
// goes away.
func (s *Server) cydEvents(w http.ResponseWriter, r *http.Request, params map[string]string) {
	svc, projectID := s.CYDCreator, params["id"]
	if _, err := svc.GetProject(r.Context(), projectID); err != nil {
		s.cydError(w, r, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		cydWriteError(w, http.StatusInternalServerError, cydcreator.CodeInternal, "streaming is not supported", nil)
		return
	}
	var after int64
	for _, raw := range []string{r.URL.Query().Get("after"), r.Header.Get("Last-Event-ID")} {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil && v > after {
			after = v
		}
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	events, unsubscribe := svc.Subscribe(projectID)
	defer unsubscribe()
	w.WriteHeader(http.StatusOK)
	reload := false
	for {
		cursor := after
		page, err := svc.EventsAfter(r.Context(), projectID, after, cydSSEPage)
		if err != nil {
			return
		}
		if cursor > 0 && !reload {
			if reload, err = cydEventsPruned(r.Context(), svc, projectID, cursor); err != nil {
				return
			}
		}
		for _, ev := range page {
			writeCYDSSE(w, ev)
			after = ev.Seq
		}
		if len(page) < cydSSEPage {
			break
		}
	}
	ready := "{}"
	if reload {
		ready = `{"reload":true}`
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", cydcreator.EventStreamReady, ready)
	flusher.Flush()
	ping := time.NewTicker(cydSSEHeartbeat)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, open := <-events:
			if !open {
				return
			}
			if ev.Seq > 0 && ev.Seq <= after {
				continue // already sent with the backlog
			}
			writeCYDSSE(w, ev)
			if ev.Seq > 0 {
				after = ev.Seq
			}
			flusher.Flush()
		case <-ping.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
