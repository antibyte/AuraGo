package homedash

import (
	"context"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// Options tunes a Hub. Zero values take the documented defaults.
type Options struct {
	Now              func() time.Time
	Logger           *slog.Logger
	IdleStop         time.Duration   // 60 s
	CatalogTTL       time.Duration   // 30 s
	CatalogTimeout   time.Duration   // 8 s
	CoalesceInterval time.Duration   // 100 ms, read by the stream writer
	CommandTimeout   time.Duration   // 15 s
	Backoff          []time.Duration // 1,2,4,8,16,30,60 s
	Audit            func(ctx context.Context, cmd Command, outcome string)
}

func (o Options) withDefaults() Options {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&o.IdleStop, 60*time.Second)
	def(&o.CatalogTTL, 30*time.Second)
	def(&o.CatalogTimeout, 8*time.Second)
	def(&o.CoalesceInterval, 100*time.Millisecond)
	def(&o.CommandTimeout, 15*time.Second)
	if len(o.Backoff) == 0 {
		o.Backoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, time.Minute}
	}
	return o
}

// Filter narrows an Entities listing.
type Filter struct {
	Source        SourceID
	Kind          Kind
	Area, Query   string
	Limit, Offset int
}

// DashboardEvent tells streams that a dashboard changed.
type DashboardEvent struct {
	ID               string `json:"id"`
	Revision         int64  `json:"revision"`
	ApprovedRevision int64  `json:"approved_revision"`
}

// Close reasons a stream ends with.
const (
	CloseShutdown      = "shutdown"
	CloseDisabled      = "disabled"
	CloseKioskDisabled = "kiosk_disabled"
	CloseDeviceRevoked = "device_revoked"
	CloseAuthLost      = "auth_lost"
	CloseClosed        = "closed"
)

// stateRecorder is the history ring (historyStore in history.go).
type stateRecorder interface {
	touch(id EntityID, now time.Time)
	observe(id EntityID, v float64, at time.Time)
	resize(points, entities int, bucket time.Duration)
	series(id EntityID, from, to time.Time, points int) []Sample
}

// Hub is the demand-driven entity hub: sources run only while someone is interested.
//
// Locking contract: Hub.mu guards configuration, source runtimes, the state map, the subscriber
// registry and every subscriber's set/srcCount. Subscriber.mu guards its pending maps. Lock order is
// Hub.mu then Subscriber.mu; Subscriber.mu holders never take Hub.mu. No source method is called
// while Hub.mu is held except factories (which must not block); SetInterest is delivered after
// unlocking, serialized per source by sourceRT.dmu with a sequence number so a stale hint never
// overwrites a newer one. Sinks never block.
type Hub struct {
	opts      Options
	log       *slog.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startedAt time.Time
	reconf    chan struct{}

	mu        sync.Mutex
	closed    bool
	cfg       RuntimeConfig
	pending   *RuntimeConfig
	factories map[SourceID]Factory
	order     []SourceID
	rts       map[SourceID]*sourceRT
	subs      map[string]*Subscriber
	states    map[EntityID]State
	approved  map[string]int64
	rec       stateRecorder // *historyStore; has its own lock (order Hub.mu then its mu)
	cmds      *commandState // command limiter, dedupe, latest-wins slots, per-source semaphores; own locks
}

// NewHub builds a hub whose goroutines end when parent is cancelled or Close is called.
func NewHub(parent context.Context, opts Options) *Hub {
	opts = opts.withDefaults()
	ctx, cancel := context.WithCancel(parent)
	h := &Hub{opts: opts, log: opts.Logger, ctx: ctx, cancel: cancel, startedAt: opts.Now(), reconf: make(chan struct{}, 1),
		factories: map[SourceID]Factory{}, rts: map[SourceID]*sourceRT{}, subs: map[string]*Subscriber{},
		states: map[EntityID]State{}, approved: map[string]int64{},
		cmds: newCommandState(), rec: newHistoryStore(360, 500, time.Minute)}
	h.wg.Add(1)
	go h.reconfigureLoop()
	return h
}

// Register adds a source factory; call it before the first Reconfigure.
func (h *Hub) Register(id SourceID, f Factory) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || f == nil || !ValidSourceID(string(id)) {
		return
	}
	if _, dup := h.factories[id]; !dup {
		h.order = append(h.order, id)
		h.rts[id] = &sourceRT{id: id, natives: map[string]int{}, status: SourceStatus{Source: id, State: StatusDisabled, At: h.opts.Now()}}
	}
	h.factories[id] = f
}

// Reconfigure stores the snapshot and wakes the hub goroutine; it never blocks (safe under CfgMu).
func (h *Hub) Reconfigure(cfg RuntimeConfig) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	c := cfg.withDefaults()
	h.pending = &c
	h.mu.Unlock()
	select {
	case h.reconf <- struct{}{}:
	default:
	}
}

// Close cancels every source, ends every stream with "shutdown" and waits up to 10 s. Idempotent.
func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	for _, rt := range h.rts {
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
		}
		stopRecheckLocked(rt)
		h.stopLocked(rt)
	}
	for _, s := range h.subs {
		s.closeWith(CloseShutdown)
	}
	h.subs = map[string]*Subscriber{}
	h.mu.Unlock()
	h.cancel()
	done := make(chan struct{})
	go func() { h.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		h.log.Warn("homedash: sources did not stop within 10 s")
	}
}

// Sources returns the status of every registered source in registration order.
func (h *Hub) Sources() []SourceStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sourcesLocked()
}

func (h *Hub) sourcesLocked() []SourceStatus {
	out := make([]SourceStatus, 0, len(h.order))
	for _, id := range h.order {
		out = append(out, h.rts[id].status)
	}
	return out
}

// DashboardChanged tells desktop streams about every change and kiosk streams of that dashboard
// only when its approved revision moved.
func (h *Hub) DashboardChanged(id string, revision, approved int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	prev, seen := h.approved[id]
	h.approved[id] = approved
	ev := DashboardEvent{ID: id, Revision: revision, ApprovedRevision: approved}
	for _, s := range h.subs {
		if s.p.Kind == PrincipalKiosk && (s.p.DashboardID != id || (seen && prev == approved)) {
			continue
		}
		s.pushDashboard(ev)
	}
}

// RevokeDevice ends the streams of one kiosk device.
func (h *Hub) RevokeDevice(deviceID, reason string) {
	h.mu.Lock()
	var notes []*interestNote
	for _, s := range h.subs {
		if s.p.DeviceID != "" && s.p.DeviceID == deviceID {
			notes = append(notes, h.detachLocked(s, reason)...)
		}
	}
	h.mu.Unlock()
	h.deliver(notes)
}

// controlBlock returns the blocked_by value when the source's control switch refuses commands,
// or "" when commands may pass. It fails closed: a source without a settings entry, or with
// Control false (whatever ConfigKey holds), is blocked.
func controlBlock(src SourceID, set SourceSettings, ok bool) string {
	switch {
	case ok && set.Control:
		return ""
	case set.ConfigKey != "":
		return "homedash.sources." + set.ConfigKey + ".control"
	default:
		return "homedash.sources." + string(src)
	}
}

// viewLocked overlays the hub gates on command capabilities of an entity leaving the hub.
func (h *Hub) viewLocked(e Entity) Entity {
	set, found := h.cfg.Sources[e.Source]
	block := ""
	switch {
	case h.cfg.ReadOnly:
		block = "homedash.readonly"
	case h.cfg.DesktopReadOnly:
		block = "virtual_desktop.readonly"
	default:
		block = controlBlock(e.Source, set, found)
	}
	caps := slices.Clone(e.Capabilities)
	readOnly, reason := true, ""
	for i := range caps {
		info, ok := LookupCapability(caps[i].Name)
		if !ok || !info.Command {
			caps[i].Allowed = true
			continue
		}
		if caps[i].Allowed && block != "" {
			caps[i].Allowed, caps[i].BlockedBy = false, block
		}
		if caps[i].Allowed {
			readOnly = false
		} else if reason == "" {
			reason = caps[i].BlockedBy
		}
	}
	if caps == nil {
		caps = []CapabilitySpec{}
	}
	e.Capabilities, e.ReadOnly, e.ReadOnlyReason = caps, readOnly, ""
	if readOnly {
		e.ReadOnlyReason = reason
	}
	return e
}

const (
	defaultListLimit = 100
	maxListLimit     = 500
)

// catalogStale reports whether a runtime's catalog must be fetched again.
func (h *Hub) catalogStale(rt *sourceRT, now time.Time) bool {
	return rt.catalog == nil || now.Sub(rt.catalogAt) > h.opts.CatalogTTL
}

// Entities lists the catalogs of the active sources matching the filter. It keeps the matching
// sources warm for CatalogTTL and fetches stale catalogs concurrently outside the lock.
func (h *Hub) Entities(ctx context.Context, f Filter) (items []Entity, total int, sources []SourceStatus) {
	now := h.opts.Now()
	h.mu.Lock()
	if h.closed {
		sources = h.sourcesLocked()
		h.mu.Unlock()
		return []Entity{}, 0, sources
	}
	var notes []*interestNote
	var jobs []catalogJob
	for _, id := range h.order {
		rt := h.rts[id]
		if !rt.active || rt.src == nil || (f.Source != "" && f.Source != id) {
			continue
		}
		rt.warmUntil = now.Add(h.opts.CatalogTTL)
		notes = append(notes, h.reconcileLocked(rt))
		h.armRecheckLocked(rt)
		if h.catalogStale(rt, now) {
			jobs = append(jobs, h.catalogJobLocked(rt))
		}
	}
	h.mu.Unlock()
	h.deliver(notes)
	h.fetchCatalogs(ctx, jobs)

	query := strings.ToLower(strings.TrimSpace(f.Query))
	area := strings.ToLower(strings.TrimSpace(f.Area))
	h.mu.Lock()
	items = []Entity{}
	for _, id := range h.order {
		rt := h.rts[id]
		if !rt.active || (f.Source != "" && f.Source != id) {
			continue
		}
		for _, e := range rt.catalog {
			if f.Kind != "" && e.Kind != f.Kind {
				continue
			}
			if area != "" && strings.ToLower(e.Area) != area && strings.ToLower(e.AreaID) != area {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(e.Name), query) &&
				!strings.Contains(strings.ToLower(string(e.ID)), query) &&
				!strings.Contains(strings.ToLower(e.Area), query) {
				continue
			}
			if st, ok := h.states[e.ID]; ok {
				e.State = st
			}
			items = append(items, h.viewLocked(e))
		}
	}
	sources = h.sourcesLocked()
	h.mu.Unlock()

	sort.Slice(items, func(i, j int) bool {
		a, b := strings.ToLower(items[i].Name), strings.ToLower(items[j].Name)
		if a != b {
			return a < b
		}
		return items[i].ID < items[j].ID
	})
	total = len(items)
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	return items[offset:end:end], total, sources
}

// Entity returns one entity of an active source, fetching that source's catalog when stale. It does
// not warm the source.
func (h *Hub) Entity(ctx context.Context, id EntityID) (Entity, error) {
	src, _, err := ParseEntityID(string(id))
	if err != nil {
		return Entity{}, ErrEntityNotFound
	}
	now := h.opts.Now()
	h.mu.Lock()
	rt := h.rts[src]
	if h.closed || rt == nil || !rt.active || rt.src == nil {
		h.mu.Unlock()
		return Entity{}, ErrEntityNotFound
	}
	var jobs []catalogJob
	if h.catalogStale(rt, now) {
		jobs = append(jobs, h.catalogJobLocked(rt))
	}
	h.mu.Unlock()
	h.fetchCatalogs(ctx, jobs)

	h.mu.Lock()
	defer h.mu.Unlock()
	rt = h.rts[src]
	if h.closed || rt == nil || !rt.active {
		return Entity{}, ErrEntityNotFound
	}
	e, ok := rt.catalog[id]
	if !ok {
		return Entity{}, ErrEntityNotFound
	}
	if st, ok := h.states[id]; ok {
		e.State = st
	}
	return h.viewLocked(e), nil
}
