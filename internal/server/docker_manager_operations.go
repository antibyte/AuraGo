package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"aurago/internal/dockerstacks"
)

// ── Docker Manager operations API ───────────────────────────────────────────
// /api/docker-manager/operations: the journal of stack operations and the live
// stream of one. Reading is allowed in docker.readonly mode.

const (
	// dmSSEWriteTimeout bounds each write of an event stream, so a client that
	// stops reading ends the stream instead of pinning the handler; heartbeats
	// re-arm it. The API listeners' httpstream wrapper renews its own budget per
	// write too. The deadline is never cleared (a stuck client must time out).
	dmSSEWriteTimeout = 30 * time.Second
)

// dmSSEHeartbeat is how long an idle operation stream stays silent before it
// writes a comment line. A variable so that a test can shorten it.
var dmSSEHeartbeat = 15 * time.Second

// operationRoute serves the operation routes. It returns false when the path is
// none of them.
func (dm *dmRequest) operationRoute(segs []string) bool {
	switch {
	case len(segs) == 1 && segs[0] == "operations":
		dm.operationList()
	case len(segs) == 2 && segs[0] == "operations":
		dm.operationGet(segs[1])
	case len(segs) == 3 && segs[0] == "operations" && segs[2] == "stream":
		dm.operationStream(segs[1])
	default:
		return false
	}
	return true
}

// operationList serves GET /operations[?host=&stack=&active=1&limit=]: newest
// first, without their output (the operation itself has it).
func (dm *dmRequest) operationList() {
	if !dm.allow(http.MethodGet) {
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	query := dm.r.URL.Query()
	filter := dockerstacks.OperationFilter{
		HostID: query.Get("host"),
		Stack:  query.Get("stack"),
		Active: dmTruthy(query.Get("active")),
	}
	if limit, err := strconv.Atoi(query.Get("limit")); err == nil && limit > 0 {
		filter.Limit = limit // the service clamps it to 100
	}
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	ops, err := svc.Operations(ctx, filter)
	if err != nil {
		dm.operationFailure(err)
		return
	}
	for i := range ops {
		ops[i].Output = ""
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"operations": dmSlice(ops)})
}

// operationGet serves GET /operations/{id}: the operation with its output tail.
func (dm *dmRequest) operationGet(id string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	op, err := svc.Operation(ctx, id)
	if err != nil {
		dm.operationFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"operation": op})
}

// dmEventStream writes server-sent events, one write deadline per write.
type dmEventStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	rc      *http.ResponseController
}

// arm sets the write deadline for the next frame. Writers without deadline
// support (httptest recorders) own their transport.
func (st *dmEventStream) arm() error {
	if err := st.rc.SetWriteDeadline(time.Now().Add(dmSSEWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

// send writes one event whose data is the compact JSON of data (one "data:"
// line) and flushes it.
func (st *dmEventStream) send(event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := st.arm(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(st.w, "event: %s\ndata: %s\n\n", event, raw); err != nil {
		return err
	}
	st.flusher.Flush()
	return nil
}

// comment writes an SSE comment line, which keeps proxies and the write
// deadline alive on a quiet stream.
func (st *dmEventStream) comment(text string) error {
	if err := st.arm(); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(st.w, ": %s\n\n", text); err != nil {
		return err
	}
	st.flusher.Flush()
	return nil
}

// operationStream serves GET /operations/{id}/stream as text/event-stream:
//
//   - "event: snapshot": the Operation with the output so far;
//   - "event: output", "progress", "state": the service's Event as JSON;
//   - "event: done": the finished Operation, after which the handler returns;
//   - ": heartbeat" comments while nothing happens.
//
// A finished operation answers snapshot and done at once. When the service
// closes the channel without a done event (the subscriber fell 256 events
// behind, or AuraGo is shutting down) the handler looks the operation up once
// more: a finished one still gets its done event, any other stream just ends and
// the client reconnects for a fresh snapshot. The handler starts no goroutine and
// returns when the client goes away; the subscription is always cancelled.
func (dm *dmRequest) operationStream(id string) {
	if !dm.allow(http.MethodGet) {
		return
	}
	svc, ok := dm.stackService()
	if !ok {
		return
	}
	flusher, ok := dm.w.(http.Flusher)
	if !ok {
		dmWriteError(dm.w, http.StatusInternalServerError, dmCodeInternal, "streaming is not supported", nil)
		return
	}
	snapshot, events, cancel, err := svc.Subscribe(id)
	if err != nil {
		dm.operationFailure(err)
		return
	}
	defer cancel()

	header := dm.w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	dm.w.WriteHeader(http.StatusOK)
	stream := &dmEventStream{w: dm.w, flusher: flusher, rc: http.NewResponseController(dm.w)}

	if stream.send("snapshot", snapshot) != nil {
		return
	}
	if snapshot.Done() {
		_ = stream.send(dockerstacks.EventDone, snapshot)
		return
	}
	beat := time.NewTicker(dmSSEHeartbeat)
	defer beat.Stop()
	ctx := dm.r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			if stream.comment("heartbeat") != nil {
				return
			}
		case ev, open := <-events:
			switch {
			case !open:
				dm.finishClosedStream(ctx, svc, stream, id)
				return
			case ev.Type == dockerstacks.EventDone:
				if ev.Operation != nil {
					_ = stream.send(dockerstacks.EventDone, *ev.Operation)
				} else {
					dm.finishClosedStream(ctx, svc, stream, id)
				}
				return
			default:
				if stream.send(ev.Type, ev) != nil {
					return
				}
			}
		}
	}
}

// finishClosedStream handles a channel that closed without a done event: if the
// operation has finished in the meantime the client gets its done event now;
// otherwise nothing more is sent and the client reconnects.
func (dm *dmRequest) finishClosedStream(ctx context.Context, svc dmStackService, stream *dmEventStream, id string) {
	ctx, cancel := context.WithTimeout(ctx, dmRegistryTimeout)
	defer cancel()
	if op, err := svc.Operation(ctx, id); err == nil && op.Done() {
		_ = stream.send(dockerstacks.EventDone, op)
	}
}
