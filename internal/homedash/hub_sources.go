package homedash

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"sync"
	"time"
)

type sourceRT struct {
	id        SourceID
	src       Source
	fp        string
	active    bool
	natives   map[string]int // subscribed native -> subscriber count
	warmUntil time.Time
	inflight  int
	running   bool
	runGen    int
	cancel    context.CancelFunc
	done      chan struct{}
	idle      *time.Timer
	idleGen   int
	status    SourceStatus
	catalog   map[EntityID]Entity
	catalogAt time.Time

	// guarded by Hub.mu
	lastDone   chan struct{} // done channel of the newest run goroutine, kept after stop so the next run can wait for it
	catSeq     int           // catalog fetch numbers issued
	catApplied int           // highest catalog number applied
	recheck    *time.Timer   // single catalog recheck timer
	recheckGen int
	failedFrom SourceStatus          // status before a catalog failure overwrote it
	dropped    map[EntityID]struct{} // ids of the catalog a rebuild cleared; removal base for the next catalog

	seq       int        // interest notes issued; guarded by Hub.mu
	dmu       sync.Mutex // serializes SetInterest delivery; never held together with Hub.mu
	delivered int        // highest note seq delivered; guarded by dmu
}

type interestNote struct {
	rt      *sourceRT
	src     Source
	natives []string
	seq     int
}

type catalogJob struct {
	id  SourceID
	src Source
	seq int
}

// catalogJobLocked numbers a catalog fetch; results older than the last applied one are dropped.
func (h *Hub) catalogJobLocked(rt *sourceRT) catalogJob {
	rt.catSeq++
	return catalogJob{id: rt.id, src: rt.src, seq: rt.catSeq}
}

// armRecheckLocked keeps one catalog recheck timer per runtime and pushes it out.
func (h *Hub) armRecheckLocked(rt *sourceRT) {
	d := h.opts.CatalogTTL + 10*time.Millisecond
	if rt.recheck != nil {
		rt.recheck.Reset(d)
		return
	}
	rt.recheckGen++
	gen, id := rt.recheckGen, rt.id
	rt.recheck = time.AfterFunc(d, func() { h.recheck(id, gen) })
}

func stopRecheckLocked(rt *sourceRT) {
	if rt.recheck != nil {
		rt.recheck.Stop()
		rt.recheck = nil
		rt.recheckGen++
	}
}

// dropSourceStatesLocked forgets every cached state of one source.
func (h *Hub) dropSourceStatesLocked(id SourceID) {
	for eid := range h.states {
		if src, _, err := ParseEntityID(string(eid)); err == nil && src == id {
			delete(h.states, eid)
		}
	}
}

func (rt *sourceRT) interested(now time.Time) bool {
	return rt.active && (len(rt.natives) > 0 || rt.inflight > 0 || now.Before(rt.warmUntil))
}

// deliver hands interest hints to their sources. It runs without Hub.mu. Notes of one source are
// delivered in sequence order: a note older than one already delivered is dropped.
func (h *Hub) deliver(notes []*interestNote) {
	for _, n := range notes {
		if n == nil {
			continue
		}
		n.rt.dmu.Lock()
		if n.seq > n.rt.delivered {
			n.rt.delivered = n.seq
			h.setInterestGuarded(n.src, n.natives)
		}
		n.rt.dmu.Unlock()
	}
}

func (h *Hub) setInterestGuarded(src Source, natives []string) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Warn("homedash: SetInterest panicked", "source", src.ID(), "panic", fmt.Sprint(r))
		}
	}()
	src.SetInterest(natives)
}

// callFactory runs a factory under Hub.mu; a panic must not leave the lock held.
func callFactory(f Factory, cfg RuntimeConfig) (src Source, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			src, ok = nil, false
		}
	}()
	return f(cfg)
}

// reconcileLocked starts a wanted source, arms the generation-guarded idle stop of an unwanted one,
// and returns the interest note to deliver after unlocking.
func (h *Hub) reconcileLocked(rt *sourceRT) *interestNote {
	if h.closed || rt.src == nil || !rt.active {
		return nil
	}
	if rt.interested(h.opts.Now()) {
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
			rt.idleGen++
		}
		if !rt.running {
			h.startLocked(rt)
		}
	} else if rt.running && rt.idle == nil {
		rt.idleGen++
		gen, id := rt.idleGen, rt.id
		rt.idle = time.AfterFunc(h.opts.IdleStop, func() { h.idleStop(id, gen) })
	}
	natives := make([]string, 0, len(rt.natives))
	for n := range rt.natives {
		natives = append(natives, n)
	}
	sort.Strings(natives)
	rt.seq++
	return &interestNote{rt: rt, src: rt.src, natives: natives, seq: rt.seq}
}

func (h *Hub) recheck(id SourceID, gen int) {
	h.mu.Lock()
	var note *interestNote
	if rt := h.rts[id]; rt != nil && rt.recheck != nil && rt.recheckGen == gen {
		note = h.reconcileLocked(rt)
	}
	h.mu.Unlock()
	h.deliver([]*interestNote{note})
}

func (h *Hub) startLocked(rt *sourceRT) {
	ctx, cancel := context.WithCancel(h.ctx)
	rt.runGen++
	prev := rt.lastDone
	rt.running, rt.cancel, rt.done = true, cancel, make(chan struct{})
	rt.lastDone = rt.done
	h.setStatusLocked(rt, StatusStarting, "")
	sink := &hubSink{h: h, id: rt.id, src: rt.src, gen: rt.runGen}
	h.wg.Add(1)
	go h.runLoop(ctx, rt.id, rt.src, sink, rt.done, prev)
}

// stopLocked cancels a running source; the returned channel closes when its goroutine ended.
func (h *Hub) stopLocked(rt *sourceRT) chan struct{} {
	if !rt.running {
		return nil
	}
	rt.cancel()
	done := rt.done
	rt.running, rt.cancel, rt.done = false, nil, nil
	return done
}

func (h *Hub) runLoop(ctx context.Context, id SourceID, src Source, sink *hubSink, done, prev chan struct{}) {
	defer h.wg.Done()
	defer close(done)
	// Runs of one source never overlap. The wait is not cut short by ctx: done must only close once
	// no earlier run of this runtime is left, otherwise a later run could overtake a hung one.
	if prev != nil {
		<-prev
	}
	if ctx.Err() != nil {
		return
	}
	attempt := 0
	for {
		started := time.Now()
		err := runGuarded(ctx, src, sink)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			attempt = 0
		}
		delay := h.opts.Backoff[min(attempt, len(h.opts.Backoff)-1)]
		attempt++
		reason := ReasonStopped
		if err != nil {
			reason = ReasonCrashed
		}
		h.log.Warn("homedash: source ended, restarting", "source", id, "error", err, "retry_in", delay)
		h.mu.Lock()
		if rt, ok := sink.live(); ok {
			h.setStatusLocked(rt, StatusDegraded, reason)
		}
		h.mu.Unlock()
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

func runGuarded(ctx context.Context, src Source, sink Sink) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("source panic: %v", r)
		}
	}()
	return src.Run(ctx, sink)
}

func catalogGuarded(ctx context.Context, src Source) (list []Entity, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("catalog panic: %v", r)
		}
	}()
	return src.Catalog(ctx)
}

func (h *Hub) idleStop(id SourceID, gen int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rt := h.rts[id]
	if rt == nil || rt.idle == nil || rt.idleGen != gen || rt.interested(h.opts.Now()) {
		return
	}
	rt.idle = nil
	h.stopLocked(rt)
	h.setStatusLocked(rt, StatusIdle, "")
}

func (h *Hub) setStatusLocked(rt *sourceRT, state, reason string) {
	if rt.status.State == state && rt.status.Reason == reason {
		return
	}
	rt.status = SourceStatus{Source: rt.id, State: state, Reason: reason, At: h.opts.Now()}
	for _, s := range h.subs {
		if s.srcCount[rt.id] > 0 {
			s.pushSource(rt.status)
		}
	}
}

// fetchCatalogs runs Catalog for each job concurrently (CatalogTimeout each, outside the lock) and
// applies results whose source is still current. A failure marks a stopped source (or one without any
// catalog) unavailable. It returns after the timeout even when a source ignores its context; the late
// result still passes the generation and sequence guards.
func (h *Hub) fetchCatalogs(ctx context.Context, jobs []catalogJob) {
	if len(jobs) == 0 {
		return
	}
	fin := make(chan int, len(jobs))
	for i, j := range jobs {
		go func(i int, j catalogJob) {
			defer func() { fin <- i }()
			h.fetchOne(ctx, j)
		}(i, j)
	}
	timer := time.NewTimer(h.opts.CatalogTimeout + catalogSlack)
	defer timer.Stop()
	finished := make([]bool, len(jobs))
	for left := len(jobs); left > 0; {
		select {
		case i := <-fin:
			finished[i] = true
			left--
		case <-timer.C:
			h.mu.Lock()
			for i, j := range jobs {
				if finished[i] {
					continue
				}
				h.log.Warn("homedash: catalog timed out", "source", j.id)
				if rt := h.rts[j.id]; !h.closed && rt != nil && rt.src == j.src {
					h.catalogFailedLocked(rt)
				}
			}
			h.mu.Unlock()
			return
		}
	}
}

// catalogSlack is added to CatalogTimeout before fetchCatalogs stops waiting for a source that
// ignores its context.
const catalogSlack = 100 * time.Millisecond

func (h *Hub) fetchOne(ctx context.Context, j catalogJob) {
	cctx, cancel := context.WithTimeout(ctx, h.opts.CatalogTimeout)
	defer cancel()
	list, err := catalogGuarded(cctx, j.src)
	h.mu.Lock()
	defer h.mu.Unlock()
	rt := h.rts[j.id]
	if h.closed || rt == nil || rt.src != j.src {
		return
	}
	if err != nil {
		if ctx.Err() != nil {
			// The caller gave up (for example an HTTP client disconnected); the source is not at fault.
			h.log.Debug("homedash: catalog fetch abandoned by caller", "source", j.id, "error", err)
			return
		}
		h.log.Warn("homedash: catalog failed", "source", j.id, "error", err)
		h.catalogFailedLocked(rt)
		return
	}
	if !h.applyCatalogLocked(rt, list, j.seq) {
		return
	}
	if rt.status.State == StatusUnavailable && rt.status.Reason == ReasonCatalogFailed {
		if rt.running && rt.failedFrom.State != "" {
			h.setStatusLocked(rt, rt.failedFrom.State, rt.failedFrom.Reason)
		} else if !rt.running {
			h.setStatusLocked(rt, StatusIdle, "")
		}
	}
}

// catalogFailedLocked marks a source whose catalog could not be fetched; a running source that
// already has a catalog, or that reported its own terminal unavailable reason (auth_failed,
// invalid_url), keeps its own status.
func (h *Hub) catalogFailedLocked(rt *sourceRT) {
	if rt.running && (rt.catalog != nil || rt.status.State == StatusUnavailable) {
		return
	}
	if rt.status.State == StatusUnavailable && rt.status.Reason == ReasonCatalogFailed {
		return
	}
	rt.failedFrom = rt.status
	h.setStatusLocked(rt, StatusUnavailable, ReasonCatalogFailed)
}

// applyCatalogLocked replaces a source's catalog and pushes entity/removed frames for subscribed ids.
// seq orders results: one not newer than the last applied is dropped (false).
func (h *Hub) applyCatalogLocked(rt *sourceRT, list []Entity, seq int) bool {
	if seq <= rt.catApplied {
		return false
	}
	rt.catApplied = seq
	if len(list) > MaxCatalogEntities {
		list = list[:MaxCatalogEntities]
	}
	now := h.opts.Now()
	next := make(map[EntityID]Entity, len(list))
	for _, raw := range list {
		e, ok := sanitizeEntity(rt.id, raw, now)
		if !ok {
			continue
		}
		next[e.ID] = e
		if old, has := h.states[e.ID]; !has || e.State.UpdatedAt.After(old.UpdatedAt) {
			h.states[e.ID] = e.State
		}
	}
	for eid := range h.states {
		src, native, err := ParseEntityID(string(eid))
		if err != nil || src != rt.id {
			continue
		}
		if _, in := next[eid]; !in && rt.natives[native] == 0 {
			delete(h.states, eid)
		}
	}
	for _, s := range h.subs {
		for id := range s.set {
			src, _, _ := ParseEntityID(string(id))
			if src != rt.id {
				continue
			}
			e, inNext := next[id]
			old, inOld := rt.catalog[id]
			if _, was := rt.dropped[id]; was && !inOld {
				inOld = true // known before the rebuild: report it as removed when it is gone
			}
			switch {
			case inNext && (!inOld || !sameMeta(old, e)):
				e.State = h.states[id]
				s.pushEntity(h.viewLocked(e))
			case !inNext && inOld:
				s.pushRemoved(id)
			}
		}
	}
	rt.catalog, rt.catalogAt, rt.dropped = next, now, nil
	return true
}

func sameMeta(a, b Entity) bool {
	a.State, b.State = State{}, State{}
	return reflect.DeepEqual(a, b)
}

type hubSink struct {
	h   *Hub
	id  SourceID
	src Source
	gen int
}

// live returns the runtime when this sink still belongs to its current run (call with Hub.mu held).
func (s *hubSink) live() (*sourceRT, bool) {
	rt := s.h.rts[s.id]
	return rt, rt != nil && !s.h.closed && rt.src == s.src && rt.runGen == s.gen && rt.running
}

func (s *hubSink) Catalog(entities []Entity) {
	s.h.mu.Lock()
	defer s.h.mu.Unlock()
	if rt, ok := s.live(); ok {
		rt.catSeq++
		s.h.applyCatalogLocked(rt, entities, rt.catSeq)
	}
}

func (s *hubSink) Status(st SourceStatus) {
	s.h.mu.Lock()
	defer s.h.mu.Unlock()
	if rt, ok := s.live(); ok && slices.Contains([]string{StatusStarting, StatusLive, StatusPolling, StatusDegraded, StatusUnavailable}, st.State) {
		s.h.setStatusLocked(rt, st.State, CleanText(st.Reason, 64))
	}
}

func (s *hubSink) Update(updates ...StateUpdate) {
	h := s.h
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := s.live()
	if !ok {
		return
	}
	now := h.opts.Now()
	for _, u := range updates {
		id := MakeEntityID(s.id, u.Native)
		if _, _, err := ParseEntityID(string(id)); err != nil {
			continue
		}
		_, inCatalog := rt.catalog[id]
		if rt.natives[u.Native] == 0 && !inCatalog {
			continue // bounded cache: only cataloged or subscribed entities keep a state
		}
		if old, has := h.states[id]; has && !u.State.UpdatedAt.IsZero() && !old.UpdatedAt.IsZero() && u.State.UpdatedAt.Before(old.UpdatedAt) {
			continue
		}
		st := sanitizeState(u.State, now)
		h.states[id] = st
		if rt.natives[u.Native] == 0 {
			continue
		}
		if f, isNum := st.Value.(float64); isNum && h.rec != nil {
			h.rec.observe(id, f, st.UpdatedAt)
		}
		for _, sub := range h.subs {
			if _, held := sub.set[id]; held {
				sub.pushState(id, st)
			}
		}
	}
}

// sanitizeState bounds an adapter state: scalar value, short text, allowlisted attributes.
func sanitizeState(st State, now time.Time) State {
	switch v := st.Value.(type) {
	case nil, bool, float64:
	case string:
		st.Value = CleanText(v, MaxStateTextRunes)
	case float32:
		st.Value = float64(v)
	case int:
		st.Value = float64(v)
	case int8:
		st.Value = float64(v)
	case int16:
		st.Value = float64(v)
	case int32:
		st.Value = float64(v)
	case int64:
		st.Value = float64(v)
	case uint:
		st.Value = float64(v)
	case uint8:
		st.Value = float64(v)
	case uint16:
		st.Value = float64(v)
	case uint32:
		st.Value = float64(v)
	case uint64:
		st.Value = float64(v)
	default:
		st.Value = nil
	}
	st.Text = CleanText(st.Text, MaxStateTextRunes)
	st.Attributes = BoundAttributes(st.Attributes)
	if st.UpdatedAt.IsZero() {
		st.UpdatedAt = now
	}
	return st
}

// sanitizeEntity bounds an adapter entity. It reports false when the id does not belong to src.
func sanitizeEntity(src SourceID, e Entity, now time.Time) (Entity, bool) {
	idSrc, native, err := ParseEntityID(string(e.ID))
	if err != nil || idSrc != src {
		return Entity{}, false
	}
	e.Source = src
	e.Name = CleanText(e.Name, MaxNameRunes)
	if e.Name == "" {
		e.Name = CleanText(native, MaxNameRunes)
	}
	e.Area = CleanText(e.Area, 80)
	e.AreaID = CleanText(e.AreaID, 64)
	e.DeviceClass = CleanText(e.DeviceClass, 64)
	e.Unit = CleanText(e.Unit, 16)
	if !ValidIcon(e.IconHint) {
		e.IconHint = ""
	}
	if !slices.Contains(AllKinds(), e.Kind) {
		e.Kind = KindGeneric
	}
	caps := make([]CapabilitySpec, 0, len(e.Capabilities))
	readOnly := true
	for _, c := range e.Capabilities {
		info, ok := LookupCapability(c.Name)
		if !ok {
			continue
		}
		if !info.Command {
			c.Allowed = true
		}
		c.BlockedBy = CleanText(c.BlockedBy, 64)
		if len(c.Options) > 0 {
			opts := make([]string, 0, min(len(c.Options), 64))
			for _, o := range c.Options {
				if len(opts) >= 64 {
					break
				}
				opts = append(opts, CleanText(o, 64))
			}
			c.Options = opts
		}
		if info.Command && c.Allowed {
			readOnly = false
		}
		caps = append(caps, c)
	}
	e.Capabilities = caps
	e.ReadOnly = readOnly
	e.ReadOnlyReason = CleanText(e.ReadOnlyReason, 64)
	if e.Media != nil {
		m := *e.Media
		if m.Live != "go2rtc_viewer" && m.Live != "mjpeg" {
			m.Live = ""
		}
		if len(m.ViewerPath) < 5 || m.ViewerPath[:5] != "/api/" {
			m.ViewerPath = ""
		}
		e.Media = &m
	}
	e.State = sanitizeState(e.State, now)
	return e, true
}
