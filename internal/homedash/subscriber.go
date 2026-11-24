package homedash

import (
	"slices"
	"sort"
	"sync"
)

// Reasons a Set call refuses an id.
const (
	RejectInvalidID      = "invalid_id"
	RejectUnknownSource  = "unknown_source"
	RejectSourceDisabled = "source_disabled"
	RejectForbidden      = "forbidden"
	RejectLimit          = "limit"
)

// Rejection names one id a Set call refused and why.
type Rejection struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// Batch is what a stream writer sends: Snapshot for ids newly added by Set, Entities for meta or
// capability changes, States coalesced latest-wins.
type Batch struct {
	Snapshot  []Entity
	Entities  []Entity
	States    map[EntityID]State
	Removed   []EntityID
	Sources   []SourceStatus
	Dashboard *DashboardEvent
	Closed    string
}

// Empty reports whether the batch carries nothing to send.
func (b Batch) Empty() bool {
	return len(b.Snapshot) == 0 && len(b.Entities) == 0 && len(b.States) == 0 && len(b.Removed) == 0 && len(b.Sources) == 0 && b.Dashboard == nil && b.Closed == ""
}

// Subscriber is one open stream. Its pending maps hold only the latest value per key.
type Subscriber struct {
	h  *Hub
	id string
	p  Principal
	// guarded by Hub.mu
	set      map[EntityID]struct{}
	srcCount map[SourceID]int
	// guarded by mu
	mu      sync.Mutex
	snap    map[EntityID]Entity
	ents    map[EntityID]Entity
	states  map[EntityID]State
	removed map[EntityID]struct{}
	sources map[SourceID]SourceStatus
	dash    *DashboardEvent
	closed  string
	ready   chan struct{}
}

// Attach opens a stream for the principal.
func (h *Hub) Attach(p Principal) (*Subscriber, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case h.closed:
		return nil, ErrHubClosed
	case !h.cfg.active():
		return nil, newError(CodeHomeDashDisabled, "HomeDash is disabled")
	case p.Kind == PrincipalAgent:
		return nil, &Error{Code: CodeForbiddenEntity, Message: "agents have no stream", BlockedBy: "agent"}
	case p.Kind == PrincipalKiosk && !h.cfg.KioskEnabled:
		return nil, newError(CodeKioskDisabled, "the kiosk is disabled")
	}
	limit, n := h.cfg.Limits.MaxStreams, 0
	if p.Kind == PrincipalKiosk {
		limit = h.cfg.Limits.MaxKioskStreams
	}
	for _, s := range h.subs {
		if (s.p.Kind == PrincipalKiosk) == (p.Kind == PrincipalKiosk) {
			n++
		}
	}
	if n >= limit {
		return nil, ErrTooManyStreams
	}
	s := &Subscriber{h: h, id: randomHex(16), p: p, set: map[EntityID]struct{}{}, srcCount: map[SourceID]int{},
		snap: map[EntityID]Entity{}, ents: map[EntityID]Entity{}, states: map[EntityID]State{},
		removed: map[EntityID]struct{}{}, sources: map[SourceID]SourceStatus{}, ready: make(chan struct{}, 1)}
	h.subs[s.id] = s
	return s, nil
}

// Subscriber finds an open stream; the principal must match the one that attached it.
func (h *Hub) Subscriber(id string, p Principal) (*Subscriber, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.subs[id]
	if s == nil || s.p.Kind != p.Kind || s.p.Key != p.Key || s.p.DeviceID != p.DeviceID {
		return nil, false
	}
	return s, true
}

// ID returns the stream id.
func (s *Subscriber) ID() string { return s.id }

// Ready receives a signal whenever something is pending; it never blocks the hub.
func (s *Subscriber) Ready() <-chan struct{} { return s.ready }

func (s *Subscriber) signal() {
	select {
	case s.ready <- struct{}{}:
	default:
	}
}

func decNative(rt *sourceRT, native string) {
	if rt.natives[native]--; rt.natives[native] <= 0 {
		delete(rt.natives, native)
	}
}

// Set replaces the subscription. Valid ids of enabled sources not yet in the catalog are accepted
// and reported unavailable in the snapshot.
func (s *Subscriber) Set(ids []EntityID) ([]EntityID, []Rejection, error) {
	h := s.h
	h.mu.Lock()
	if h.closed || h.subs[s.id] != s {
		h.mu.Unlock()
		return nil, nil, newError(CodeStreamNotFound, "the stream is closed")
	}
	var accepted []EntityID
	var rejected []Rejection
	want := map[EntityID]struct{}{}
	for _, id := range ids {
		if _, dup := want[id]; dup {
			continue
		}
		src, _, err := ParseEntityID(string(id))
		rt := h.rts[src]
		reason := ""
		switch {
		case err != nil:
			reason = RejectInvalidID
		case rt == nil:
			reason = RejectUnknownSource
		case !rt.active:
			reason = RejectSourceDisabled
		case s.p.Kind == PrincipalKiosk && !s.p.Allowed.Has(id):
			reason = RejectForbidden
		case len(want) >= h.cfg.Limits.MaxSubscriptions:
			reason = RejectLimit
		}
		if reason != "" {
			rejected = append(rejected, Rejection{ID: string(id), Reason: reason})
			continue
		}
		want[id] = struct{}{}
		accepted = append(accepted, id)
	}
	touched := map[SourceID]*sourceRT{}
	for id := range s.set {
		if _, keep := want[id]; keep {
			continue
		}
		src, native, _ := ParseEntityID(string(id))
		delete(s.set, id)
		if s.srcCount[src]--; s.srcCount[src] <= 0 {
			delete(s.srcCount, src)
		}
		if rt := h.rts[src]; rt != nil {
			decNative(rt, native)
			touched[src] = rt
		}
		s.forget(id)
	}
	now := h.opts.Now()
	for _, id := range accepted {
		if _, had := s.set[id]; had {
			continue
		}
		src, native, _ := ParseEntityID(string(id))
		rt := h.rts[src]
		s.set[id] = struct{}{}
		s.srcCount[src]++
		rt.natives[native]++
		touched[src] = rt
		e, ok := rt.catalog[id]
		if !ok {
			e = Entity{ID: id, Source: src, Kind: KindGeneric, Name: CleanText(native, MaxNameRunes),
				Capabilities: []CapabilitySpec{}, ReadOnly: true, State: State{Available: false, UpdatedAt: now}}
		}
		if st, ok := h.states[id]; ok {
			e.State = st
		}
		s.pushSnapshot(h.viewLocked(e))
		s.pushSource(rt.status)
		if h.rec != nil {
			h.rec.touch(id, now)
		}
	}
	var notes []*interestNote
	for _, rt := range touched {
		notes = append(notes, h.reconcileLocked(rt))
	}
	h.mu.Unlock()
	h.deliver(notes)
	return accepted, rejected, nil
}

// Close ends the stream and releases its interest.
func (s *Subscriber) Close() {
	s.h.mu.Lock()
	notes := s.h.detachLocked(s, CloseClosed)
	s.h.mu.Unlock()
	s.h.deliver(notes)
}

func (h *Hub) detachLocked(s *Subscriber, reason string) []*interestNote {
	if h.subs[s.id] != s {
		return nil
	}
	delete(h.subs, s.id)
	touched := map[SourceID]*sourceRT{}
	for id := range s.set {
		src, native, _ := ParseEntityID(string(id))
		if rt := h.rts[src]; rt != nil {
			decNative(rt, native)
			touched[src] = rt
		}
	}
	s.set, s.srcCount = map[EntityID]struct{}{}, map[SourceID]int{}
	s.closeWith(reason)
	var notes []*interestNote
	for _, rt := range touched {
		notes = append(notes, h.reconcileLocked(rt))
	}
	return notes
}

func (s *Subscriber) closeWith(reason string) {
	s.mu.Lock()
	if s.closed == "" {
		s.closed = reason
	}
	s.mu.Unlock()
	s.signal()
}

// push helpers run with Hub.mu held; they never block and keep only the latest value per key.
func (s *Subscriber) pushState(id EntityID, st State) {
	s.mu.Lock()
	if s.closed == "" {
		s.states[id] = st
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushEntity(e Entity) {
	s.mu.Lock()
	if s.closed == "" {
		if _, inSnap := s.snap[e.ID]; inSnap {
			s.snap[e.ID] = e
		} else {
			s.ents[e.ID] = e
		}
		delete(s.removed, e.ID)
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushSnapshot(e Entity) {
	s.mu.Lock()
	if s.closed == "" {
		s.snap[e.ID] = e
		delete(s.ents, e.ID)
		delete(s.states, e.ID)
		delete(s.removed, e.ID)
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushRemoved(id EntityID) {
	s.mu.Lock()
	if s.closed == "" {
		s.forgetLocked(id)
		s.removed[id] = struct{}{}
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushSource(st SourceStatus) {
	s.mu.Lock()
	if s.closed == "" {
		s.sources[st.Source] = st
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) pushDashboard(ev DashboardEvent) {
	s.mu.Lock()
	if s.closed == "" {
		s.dash = &ev
	}
	s.mu.Unlock()
	s.signal()
}

func (s *Subscriber) forget(id EntityID) { s.mu.Lock(); s.forgetLocked(id); s.mu.Unlock() }

func (s *Subscriber) forgetLocked(id EntityID) {
	delete(s.snap, id)
	delete(s.ents, id)
	delete(s.states, id)
	delete(s.removed, id)
}

// Drain swaps out everything pending. Memory per subscriber is bounded by its subscription size.
func (s *Subscriber) Drain() Batch {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := Batch{Closed: s.closed, Dashboard: s.dash}
	s.dash = nil
	b.Snapshot, b.Entities = sortedEntities(s.snap), sortedEntities(s.ents)
	s.snap, s.ents = map[EntityID]Entity{}, map[EntityID]Entity{}
	if len(s.states) > 0 {
		b.States, s.states = s.states, map[EntityID]State{}
	}
	for id := range s.removed {
		b.Removed = append(b.Removed, id)
	}
	slices.Sort(b.Removed)
	s.removed = map[EntityID]struct{}{}
	for _, st := range s.sources {
		b.Sources = append(b.Sources, st)
	}
	sort.Slice(b.Sources, func(i, j int) bool { return b.Sources[i].Source < b.Sources[j].Source })
	s.sources = map[SourceID]SourceStatus{}
	return b
}

func sortedEntities(m map[EntityID]Entity) []Entity {
	if len(m) == 0 {
		return nil
	}
	out := make([]Entity, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
