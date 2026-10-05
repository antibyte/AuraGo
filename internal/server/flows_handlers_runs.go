package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/flows"
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

// Run event streams (GET /api/desktop/flows/runs/{run}/events, streamFlowRunEvents).
const (
	// flowStreamsPerRun and flowStreamsTotal bound the open event streams of one run and of
	// all runs together. Each stream holds a bus subscription (a buffer of 256 events) and a
	// request goroutine until its run ends; a stream beyond a limit is refused with 429
	// FLOW_RUN_LIMIT.
	flowStreamsPerRun = 16
	flowStreamsTotal  = 64
	// flowStreamMaxSeq bounds the start point a client gives (?after=, Last-Event-ID). A run
	// has about 2*flows.MaxNodes+2 events, so a larger value is garbage and ignored.
	flowStreamMaxSeq = 1 << 20
)

// flowStreamHeartbeat is the interval of the ":heartbeat" comment on an open stream. It is
// a variable so that tests can shorten it.
var flowStreamHeartbeat = 15 * time.Second

// flowStreamLimiter counts the open run event streams, per run and in total. The zero value
// is ready to use; Server.flowStreams holds the one of the API.
type flowStreamLimiter struct {
	mu    sync.Mutex
	total int
	runs  map[string]int
}

// acquire reserves a stream of runID and reports false when a limit is reached; a reserved
// stream must be given back with release.
func (l *flowStreamLimiter) acquire(runID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= flowStreamsTotal || l.runs[runID] >= flowStreamsPerRun {
		return false
	}
	if l.runs == nil {
		l.runs = map[string]int{}
	}
	l.runs[runID]++
	l.total++
	return true
}

// release gives back a stream that acquire reserved.
func (l *flowStreamLimiter) release(runID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.total--
	if n := l.runs[runID] - 1; n > 0 {
		l.runs[runID] = n
	} else {
		delete(l.runs, runID)
	}
}

// flowStreamAfter returns the sequence number a stream starts after: the larger of ?after=
// and the Last-Event-ID header. EventSource sends Last-Event-ID (the id of the last event it
// received) when it reconnects on its own, with the URL it was created with, so ?after= is
// then the older of the two; a client that creates a new EventSource sends ?after= only. A
// value that is not a decimal number from 0 to flowStreamMaxSeq is ignored; without a valid
// one the stream starts at the beginning of the log (the events are idempotent).
func flowStreamAfter(r *http.Request) int {
	after := 0
	for _, text := range []string{r.URL.Query().Get("after"), r.Header.Get("Last-Event-ID")} {
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err == nil && n > after && n <= flowStreamMaxSeq {
			after = n
		}
	}
	return after
}

// streamFlowRunEvents serves the live events of a run as text/event-stream:
//
//   - "event: snapshot": the RunDetail from the store, without an id;
//   - "id: <seq>" and "event: event": every bus event after the start point (flowStreamAfter),
//     data a RunEvent that carries its "seq";
//   - "event: end", data {}: the run is over; the stream closes;
//   - "event: resync", data {"after": <seq>}: the stream closes before the run ended, see
//     below; the client reconnects with ?after=<the last seq it applied>;
//   - ":heartbeat" comments every flowStreamHeartbeat.
//
// Order: it subscribes first and reads the snapshot second. The bus keeps the whole log of
// a run (not a ring) until its retention after the run ended, so the backlog holds every
// event after the start point, and the snapshot is never older than the subscription.
// Events the snapshot already reflects come again; a client applies the snapshot, then the
// events, and a step_finished replaces the step of its node, so that is harmless. Read the
// other way round, an event published between the two reads would be lost when the bus
// forgot the run in between (its retention ran out).
//
// End versus resync: the bus closes a subscriber's channel when the run finished, when the
// subscriber fell more than 256 events behind (this handler blocks on a slow client's
// connection), or when its sweep forgot a leaked log. "end" is sent only when the stream
// delivered run_finished, when the snapshot (read after subscribing) shows the run
// finished, or when the bus does not know the run (forgotten after its retention, or a run
// of an earlier process). The bus closes the channel only after the runner stored the
// run's result, so a client that reloads the run on "end" finds it final. Any other close
// sends "resync": the run goes on, and a new stream with ?after= continues from the log
// without a gap. A finished run's stream (a reconnect included) ends at once with snapshot
// and "end", and a reconnect after the last event cannot loop on "resync".
//
// The stream also ends when the client goes away and when the server drains for a
// shutdown: trackHTTP cancels the request context on beginHTTPDrain, so an open stream does
// not hold up httpRequests.Wait. Write deadline: the API listeners wrap their handlers in
// httpstream.WithWriteTimeout, which renews the deadline before every write of a
// text/event-stream response, so the heartbeat keeps a long run's stream open within the
// listener's WriteTimeout; the deadline is deliberately not cleared (a stuck client must
// still time out). The gzip middleware leaves text/event-stream uncompressed. Streams are
// limited per run and in total (flowStreamsPerRun, flowStreamsTotal).
func (s *Server) streamFlowRunEvents(w http.ResponseWriter, r *http.Request, runID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		flowsError(w, http.StatusInternalServerError, "FLOW_INTERNAL", "streaming is not supported")
		return
	}
	if !s.flowStreams.acquire(runID) {
		flowsError(w, http.StatusTooManyRequests, "FLOW_RUN_LIMIT", "too many open run event streams; close other run views and try again")
		return
	}
	defer s.flowStreams.release(runID)
	ctx := r.Context()
	after := flowStreamAfter(r)
	backlog, events, unsubscribe, live := s.Flows.Subscribe(runID, after)
	if unsubscribe != nil {
		defer unsubscribe()
	}
	detail, err := s.Flows.Run(ctx, runID, false)
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if writeFlowSSE(w, flusher, "snapshot", 0, detail) != nil {
		return
	}
	last, finished := after, false
	send := func(ev flows.RunEvent) error {
		if ev.Seq > last {
			last = ev.Seq
		}
		if ev.Type == flows.EventRunFinished {
			finished = true
		}
		return writeFlowSSE(w, flusher, "event", ev.Seq, ev)
	}
	for _, ev := range backlog {
		if send(ev) != nil {
			return
		}
	}
	if !live || events == nil {
		_ = writeFlowSSE(w, flusher, "end", 0, map[string]any{})
		return
	}
	heartbeat := time.NewTicker(flowStreamHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ":heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, open := <-events:
			if open {
				if send(ev) != nil {
					return
				}
				continue
			}
			if finished || (detail.Run != nil && detail.Run.Status.Terminal()) {
				_ = writeFlowSSE(w, flusher, "end", 0, map[string]any{})
			} else {
				_ = writeFlowSSE(w, flusher, "resync", 0, map[string]int{"after": last})
			}
			return
		}
	}
}

// writeFlowSSE writes one event, flushed at once: an "id:" line when id > 0, the event name
// and the payload as one line of JSON with its values scrubbed (flowScrubbedJSON).
func writeFlowSSE(w http.ResponseWriter, flusher http.Flusher, event string, id int, payload any) error {
	data, err := flowScrubbedJSON(payload)
	if err != nil {
		return err
	}
	var b strings.Builder
	if id > 0 {
		fmt.Fprintf(&b, "id: %d\n", id)
	}
	fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", event, data)
	if _, err := io.WriteString(w, b.String()); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// flowScrubbedJSON encodes value with the registered secrets redacted in its values, as
// flowsJSONScrubbed does: value is encoded and decoded once into plain JSON values, which
// scrubFlowValue copies with every string, map key and number scrubbed. Scrubbing the
// encoded text instead would miss a secret holding a quote, a backslash or a control
// character (JSON escapes them) and could break the JSON. The result holds no line break
// (json.Marshal escapes them), so it fits one "data:" line.
func flowScrubbedJSON(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var plain any
	if err := json.Unmarshal(data, &plain); err != nil {
		return nil, err
	}
	return json.Marshal(scrubFlowValue(plain))
}
