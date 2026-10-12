package cydcreator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Event types (overview "SSE"). Plan 2 emits project_changed, build_status,
// build_log and stream_ready; plans 3 and 5 emit the others.
const (
	EventProjectChanged = "project_changed"
	EventLockChanged    = "lock_changed"
	EventBuildStatus    = "build_status"
	EventBuildLog       = "build_log"
	EventJobStatus      = "job_status"
	EventJobStep        = "job_step"
	EventJobMessage     = "job_message"
	EventDeviceChanged  = "device_changed"
	EventStreamReady    = "stream_ready"
)

// Event is one project event. Seq is 0 for ephemeral (not persisted) events.
type Event struct {
	Seq       int64           `json:"seq"`
	ProjectID string          `json:"project_id"`
	Type      string          `json:"type"`
	JobID     string          `json:"job_id,omitempty"`
	BuildID   string          `json:"build_id,omitempty"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

const (
	// subscriberBuffer is the channel size of one subscriber. A subscriber
	// that falls this far behind is dropped: its channel is closed and the SSE
	// client reconnects with Last-Event-ID.
	subscriberBuffer = 256
	maxEventsPage    = 500
)

// maxEventsPerProject persisted events are kept per project; older ones are
// pruned after every eventPruneEvery emits of that project. Variables so tests
// can shrink them.
var (
	maxEventsPerProject = 5000
	eventPruneEvery     = 200
)

type eventSubscriber struct {
	ch     chan Event
	closed bool
}

// marshalEventData encodes event data as compact (single-line) JSON, so an
// SSE frame always carries it on one "data:" line. json.Marshal output is
// compact already; a caller's json.RawMessage is compacted.
func marshalEventData(data any) json.RawMessage {
	if data == nil {
		return json.RawMessage(`{}`)
	}
	if raw, ok := data.(json.RawMessage); ok && json.Valid(raw) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw); err != nil {
			return json.RawMessage(`{}`)
		}
		return compact.Bytes()
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

// Emit persists an event and fans it out to the project's subscribers.
// Failures are logged; events never fail the operation that caused them.
//
// Never call Emit or EmitEphemeral inside a database transaction or while
// iterating rows: Emit holds eventsMu (which EmitEphemeral also takes) while
// it writes through its own pool connection (dbutil default: 4 connections,
// busy_timeout 5 s). Inside an open write transaction the INSERT waits for the
// busy timeout and the event is lost while the transaction cannot commit; open
// rows hold connections, and an exhausted pool blocks every emitter. Commit or
// close the rows first, then emit.
func (s *Service) Emit(projectID, typ, jobID, buildID string, data any) {
	raw := marshalEventData(data)
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	created := s.now().UTC()
	res, err := s.db.Exec(`INSERT INTO cc_events(project_id,type,job_id,build_id,data,created_at) VALUES(?,?,?,?,?,?)`,
		projectID, typ, jobID, buildID, string(raw), formatTime(created))
	if err != nil {
		s.logger.Warn("CYD Creator event could not be stored", "type", typ, "error", err)
		return
	}
	// A persisted event must never go out live with seq 0 (the ephemeral
	// marker); without its seq it is only delivered by EventsAfter.
	if seq, err := res.LastInsertId(); err != nil || seq <= 0 {
		s.logger.Warn("CYD Creator event was stored without a known seq; not sent live", "type", typ, "error", err)
	} else {
		s.fanOut(Event{Seq: seq, ProjectID: projectID, Type: typ, JobID: jobID, BuildID: buildID, Data: raw, CreatedAt: created})
	}
	s.emitCounts[projectID]++
	if s.emitCounts[projectID] >= eventPruneEvery {
		delete(s.emitCounts, projectID) // a missing entry counts as 0, so the map only holds projects between prunes
		if _, err := s.db.Exec(`DELETE FROM cc_events WHERE project_id=? AND seq < (
			SELECT seq FROM cc_events WHERE project_id=? ORDER BY seq DESC LIMIT 1 OFFSET ?)`,
			projectID, projectID, maxEventsPerProject-1); err != nil {
			s.logger.Warn("CYD Creator events could not be pruned", "error", err)
		}
	}
}

// EmitEphemeral fans an event out without storing it (Seq = 0).
func (s *Service) EmitEphemeral(projectID, typ, jobID, buildID string, data any) {
	raw := marshalEventData(data)
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()
	s.fanOut(Event{ProjectID: projectID, Type: typ, JobID: jobID, BuildID: buildID, Data: raw, CreatedAt: s.now().UTC()})
}

// fanOut delivers ev to every subscriber of its project without blocking.
// Callers hold eventsMu, so subscribers see persisted events in seq order.
func (s *Service) fanOut(ev Event) {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for sub := range s.subscribers[ev.ProjectID] {
		select {
		case sub.ch <- ev:
		default:
			s.dropSubscriberLocked(ev.ProjectID, sub)
		}
	}
}

// Subscribe returns a channel of the project's live events and a cancel
// function (idempotent). The channel is closed when the subscriber falls
// subscriberBuffer events behind, when the project is deleted and on Close;
// after Close it is returned already closed.
func (s *Service) Subscribe(projectID string) (<-chan Event, func()) {
	sub := &eventSubscriber{ch: make(chan Event, subscriberBuffer)}
	s.subsMu.Lock()
	if s.subsClosed {
		sub.closed = true
		close(sub.ch)
		s.subsMu.Unlock()
		return sub.ch, func() {}
	}
	if s.subscribers[projectID] == nil {
		s.subscribers[projectID] = map[*eventSubscriber]struct{}{}
	}
	s.subscribers[projectID][sub] = struct{}{}
	s.subsMu.Unlock()
	return sub.ch, func() {
		s.subsMu.Lock()
		s.dropSubscriberLocked(projectID, sub)
		s.subsMu.Unlock()
	}
}

func (s *Service) dropSubscriberLocked(projectID string, sub *eventSubscriber) {
	if sub.closed {
		return
	}
	sub.closed = true
	close(sub.ch)
	if subs := s.subscribers[projectID]; subs != nil {
		delete(subs, sub)
		if len(subs) == 0 {
			delete(s.subscribers, projectID)
		}
	}
}

// dropProjectSubscribers ends the subscriptions of a deleted project and
// forgets its prune counter.
func (s *Service) dropProjectSubscribers(projectID string) {
	s.eventsMu.Lock()
	delete(s.emitCounts, projectID)
	s.eventsMu.Unlock()
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	for sub := range s.subscribers[projectID] {
		s.dropSubscriberLocked(projectID, sub)
	}
}

// dropAllSubscribers closes every subscription and refuses new ones (Close).
func (s *Service) dropAllSubscribers() {
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	s.subsClosed = true
	for projectID, subs := range s.subscribers {
		for sub := range subs {
			s.dropSubscriberLocked(projectID, sub)
		}
	}
}

// EventsAfter returns up to limit (1..500, default 500) persisted events of
// the project with Seq > after, oldest first.
func (s *Service) EventsAfter(ctx context.Context, projectID string, after int64, limit int) ([]Event, error) {
	if limit <= 0 || limit > maxEventsPage {
		limit = maxEventsPage
	}
	rows, err := s.db.QueryContext(ctx, `SELECT seq,project_id,type,job_id,build_id,data,created_at FROM cc_events
		WHERE project_id=? AND seq>? ORDER BY seq LIMIT ?`, projectID, after, limit)
	if err != nil {
		return nil, s.internalError("list events", fmt.Errorf("query events: %w", err))
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var ev Event
		var data, created string
		if err := rows.Scan(&ev.Seq, &ev.ProjectID, &ev.Type, &ev.JobID, &ev.BuildID, &data, &created); err != nil {
			return nil, s.internalError("list events", err)
		}
		ev.Data = json.RawMessage(data)
		ev.CreatedAt, _ = time.Parse(timeLayout, created)
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, s.internalError("list events", err)
	}
	return out, nil
}

// LatestEventSeq returns the seq of the project's newest persisted event, 0
// when it has none. An SSE resume point above it cannot come from this
// database (forged, or the database was restored or reset), so the stream
// asks the client to reload.
func (s *Service) LatestEventSeq(ctx context.Context, projectID string) (int64, error) {
	var seq int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0) FROM cc_events WHERE project_id=?`, projectID).Scan(&seq); err != nil {
		return 0, s.internalError("read events", fmt.Errorf("query newest event: %w", err))
	}
	return seq, nil
}
