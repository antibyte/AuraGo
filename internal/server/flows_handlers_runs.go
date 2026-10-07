package server

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
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

// flowsTestDataBodyLimit bounds the body of PUT {id}/test-data/{node}: the flows cap on
// remembered sample data (flows.MaxStoredOutputBytes of its JSON encoding, else
// flows.ErrTestDataTooLarge) plus room for the {"data": …} wrapper and formatting. Data near
// the cap gets the flow service's answer; a far larger body is refused before it is decoded.
const flowsTestDataBodyLimit = flows.MaxStoredOutputBytes + 32<<10

// flowRunIDPattern accepts what can be a run id (flows.NewRunID gives "run_" and twelve
// characters); anything else is FLOW_RUN_NOT_FOUND before the store or the bus is asked.
var flowRunIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// flowRunFilter reads ?mode=, ?status=, ?limit= and ?offset= of GET {id}/runs. A mode or
// status that is not one of the flows package's (RunMode.Valid, RunStatus.Valid), or a
// limit or offset that is not a non-negative whole number, gives the message of a
// FLOW_BAD_REQUEST answer (it does not echo the value); an absent or empty one is left
// unset. The limit is not clamped here: Store.ListRuns applies its default (50) and its cap
// (200).
func flowRunFilter(q url.Values) (flows.RunFilter, string) {
	f := flows.RunFilter{Mode: flows.RunMode(q.Get("mode")), Status: flows.RunStatus(q.Get("status"))}
	if f.Mode != "" && !f.Mode.Valid() {
		return f, "mode must be test, live, agent or call"
	}
	if f.Status != "" && !f.Status.Valid() {
		return f, "status must be queued, running, waiting, success, error or cancelled"
	}
	for _, p := range []struct {
		name string
		dst  *int
	}{{"limit", &f.Limit}, {"offset", &f.Offset}} {
		text := q.Get(p.name)
		if text == "" {
			continue
		}
		n, err := strconv.Atoi(text)
		if err != nil || n < 0 {
			return f, p.name + " must be a non-negative whole number"
		}
		*p.dst = n
	}
	return f, ""
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
		if !flowsDecode(w, r, &body, flowsDocBodyLimit, true) {
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
		filter, problem := flowRunFilter(r.URL.Query())
		if problem != "" {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", problem)
			return true
		}
		// The run list of a flow that does not exist is FLOW_NOT_FOUND, not an empty list
		// (GetFlow is a lock-free read).
		if _, err := s.Flows.GetFlow(ctx, id); err != nil {
			s.flowsErrorFrom(w, r, err)
			return true
		}
		runs, err := s.Flows.Runs(ctx, id, filter)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return true
		}
		s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"runs": runs})
	case action == "test-data" && len(rest) == 1:
		// Sample data can hold anything the user pasted, a secret included: both answers
		// are scrubbed by value. Only an enabled trigger of the draft has sample data (the
		// service answers FLOW_NO_TRIGGER for any other node); an id that cannot be a node
		// id is refused before the flow is read.
		node := rest[0]
		if !flows.ValidNodeID(node) {
			flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "the node id is not valid")
			return true
		}
		switch r.Method {
		case http.MethodGet:
			data, err := s.Flows.TriggerSampleData(ctx, id, node)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return true
			}
			s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"data": data})
		case http.MethodPut:
			var body struct {
				Data map[string]any `json:"data"`
			}
			if !flowsDecode(w, r, &body, flowsTestDataBodyLimit, false) {
				return true
			}
			if body.Data == nil {
				body.Data = map[string]any{}
			}
			if err := s.Flows.SaveTriggerSample(ctx, id, node, body.Data); err != nil {
				s.flowsErrorFrom(w, r, err)
				return true
			}
			s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"data": body.Data})
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
	if !flowRunIDPattern.MatchString(runID) {
		flowsError(w, http.StatusNotFound, "FLOW_RUN_NOT_FOUND", "run not found")
		return
	}
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
		s.cancelFlowRun(w, r, runID)
	default:
		flowsError(w, http.StatusNotFound, "FLOW_RUN_NOT_FOUND", "unknown run route")
	}
}

// cancelFlowRun answers POST runs/{run}/cancel: 202 {"cancelled": true} when the run is
// active (also again while a cancelled run winds down: the cancel is idempotent), 404
// FLOW_RUN_NOT_FOUND for a run the store does not know, and 409 FLOW_RUN_FINISHED for a
// stored run that is no longer active (finished, or ending at this moment). The runner
// forgets a run once it ended, so the run's header is read from the store first
// (Service.RunHeader: no steps, no flow lock). The first cancel of a live run goes to the
// audit timeline (flow_run_cancel); test runs belong to the editor and are not audited.
func (s *Server) cancelFlowRun(w http.ResponseWriter, r *http.Request, runID string) {
	ctx := r.Context()
	run, err := s.Flows.RunHeader(ctx, runID)
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	known, first := false, false
	if !run.Status.Terminal() {
		known, first = s.Flows.CancelRun(runID)
	}
	if !known {
		flowsError(w, http.StatusConflict, "FLOW_RUN_FINISHED", "the run has already finished")
		return
	}
	if first && run.Mode == flows.ModeLive {
		// The run is cancelled; a client that went away must not leave the entry without
		// the flow's name.
		name, label := "", run.FlowID
		if rec, err := s.Flows.GetFlow(context.WithoutCancel(ctx), run.FlowID); err == nil {
			name, label = rec.Name, rec.Name
		}
		s.recordFlowAudit("flow_run_cancel", run.FlowID, name, "Flow "+label+": run "+runID+" cancelled")
	}
	flowsJSON(w, http.StatusAccepted, map[string]bool{"cancelled": true})
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
	// flowStreamHeartbeat is the interval of the ": heartbeat" comment on an open stream;
	// Server.flowStreamBeat overrides it when set (tests shorten it).
	flowStreamHeartbeat = 15 * time.Second
)

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
//   - ": heartbeat" comments every flowStreamHeartbeat (writeSSEComment).
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
// connection), or when its sweep forgot a leaked log. "end" is sent when the stream
// delivered run_finished, when the snapshot (read after subscribing) shows the run
// finished, when the bus does not know the run (forgotten after its retention, or a run
// of an earlier process), or when the channel closed before it delivered a single event:
// a dropped subscriber has 256 buffered events to read first, so such a close is a run
// that already ended (or a forgotten leaked log). The bus closes the channel only after
// the runner stored the run's result, so a client that reloads the run on "end" finds it
// final. Any other close sends "resync": the run goes on, and a new stream with ?after=
// continues from the log without a gap. A finished run's stream (a reconnect included, also
// one after the last event or for a run whose stored status is stale because FinishRun
// failed) ends at once with snapshot and "end", so a client never loops on "resync".
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
		// A drained or departed client stops the backlog too (each event is scrubbed).
		if ctx.Err() != nil || send(ev) != nil {
			return
		}
	}
	if !live || events == nil {
		_ = writeFlowSSE(w, flusher, "end", 0, map[string]any{})
		return
	}
	beat := flowStreamHeartbeat
	if s.flowStreamBeat > 0 {
		beat = s.flowStreamBeat
	}
	heartbeat := time.NewTicker(beat)
	defer heartbeat.Stop()
	received := 0 // events read from the channel
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if writeSSEComment(w, flusher, "heartbeat") != nil {
				return
			}
		case ev, open := <-events:
			if open {
				received++
				if send(ev) != nil {
					return
				}
				continue
			}
			if finished || received == 0 || (detail.Run != nil && detail.Run.Status.Terminal()) {
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
	prefix := "event: " + event + "\ndata: "
	if id > 0 {
		prefix = "id: " + strconv.Itoa(id) + "\n" + prefix
	}
	if _, err := io.WriteString(w, prefix); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n\n"); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
