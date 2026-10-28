package dockerwatch

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"
)

// Hub owns the live data of all Docker hosts and the open stream clients.
//
// Locking: Hub.mu guards the clients, their subscriptions and every host
// runtime (watcher state, rings, callbacks). Messages are delivered into the
// clients' buffered channels while it is held, which never blocks, so the order
// of messages a client sees is the order they were produced in. Order of locks:
// Hub.mu, then Client.mu; budgetMu, dfMu and a df entry's gate are leaves and
// never held while taking Hub.mu.
type Hub struct {
	hosts Hosts
	opts  Options
	log   *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	closed  bool
	clients map[string]*Client
	rts     map[string]*hostRT
	nextCB  int

	budgetMu sync.Mutex
	streams  map[string]int
	projects map[string]int // project and stack log follows per host

	dfMu sync.Mutex
	df   map[string]*dfEntry
}

// hostRT is what the hub runs for one host: the events watcher, the callbacks
// of WatchHost and the stats state. It exists while any of them does.
type hostRT struct {
	id        string
	status    HostStatus // last status of the running watcher; State "" without one
	watcher   *watcher
	idle      *time.Timer // pending stop of the watcher
	idleGen   int
	callbacks map[int]func(Event)
	st        *statsState
}

// New creates a hub. Nothing runs until a client subscribes or WatchHost is called.
func New(hosts Hosts, opts Options) *Hub {
	opts = opts.withDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		hosts:    hosts,
		opts:     opts,
		log:      opts.Logger,
		ctx:      ctx,
		cancel:   cancel,
		clients:  map[string]*Client{},
		rts:      map[string]*hostRT{},
		streams:  map[string]int{},
		projects: map[string]int{},
		df:       map[string]*dfEntry{},
	}
}

// Close stops every watcher and sampler, closes all clients and waits for the
// hub's goroutines. It is safe to call twice.
func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	clients := make([]*Client, 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	for _, rt := range h.rts {
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
		}
	}
	h.mu.Unlock()
	h.cancel()
	for _, c := range clients {
		c.shut()
	}
	h.wg.Wait()
}

// Publish sends msg to the clients subscribed to host's events. It never
// blocks and starts nothing; the server's operation bridge uses it.
func (h *Hub) Publish(host string, msg Message) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.publishLocked(host, msg)
}

func (h *Hub) publishLocked(host string, msg Message) {
	for _, c := range h.clients {
		if slices.Contains(c.sub.Hosts, host) {
			c.push(msg)
		}
	}
}

// WatchHost registers fn for the normalised events of host, including the
// watcher's resync as Event{Type: "resync", Action: "reconnect"|"poll"}, and
// keeps the host's events watcher running until the returned cancel is called
// (the watcher then stops after IdleStop unless a client still needs it). fn
// runs on the watcher's goroutine and must not block or call back into the hub.
func (h *Hub) WatchHost(host string, fn func(Event)) (cancel func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || fn == nil || host == "" {
		return func() {}
	}
	rt := h.rtLocked(host)
	h.nextCB++
	id := h.nextCB
	rt.callbacks[id] = fn
	h.reconcileLocked(host)
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if rt := h.rts[host]; rt != nil {
				delete(rt.callbacks, id)
			}
			h.reconcileLocked(host)
		})
	}
}

// rtLocked returns the runtime of a host, creating it.
func (h *Hub) rtLocked(host string) *hostRT {
	rt := h.rts[host]
	if rt == nil {
		rt = &hostRT{id: host, callbacks: map[int]func(Event){}, st: newStatsState()}
		h.rts[host] = rt
	}
	return rt
}

// interestLocked tells who needs host: the number of event users (clients with
// the host in Hosts plus WatchHost callbacks), the clients that want host
// gauges, and the union of the container ids clients want stats for.
func (h *Hub) interestLocked(host string) (events, gauges int, ids []string) {
	set := map[string]struct{}{}
	for _, c := range h.clients {
		if c.isClosed() {
			continue
		}
		if slices.Contains(c.sub.Hosts, host) {
			events++
		}
		if slices.Contains(c.sub.HostStats, host) {
			gauges++
		}
		for _, id := range c.sub.Stats[host] {
			set[id] = struct{}{}
		}
	}
	if rt := h.rts[host]; rt != nil {
		events += len(rt.callbacks)
	}
	ids = make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return events, gauges, ids
}

// reconcileLocked starts and stops what a host needs after its interest changed.
func (h *Hub) reconcileLocked(host string) {
	if h.closed {
		return
	}
	events, gauges, ids := h.interestLocked(host)
	rt := h.rts[host]
	if rt == nil {
		if events == 0 && gauges == 0 && len(ids) == 0 {
			return
		}
		rt = h.rtLocked(host)
	}
	h.reconcileEventsLocked(rt, events)
	h.reconcileStatsLocked(rt, len(ids) > 0, gauges > 0)
	h.gcLocked(rt)
}

func (h *Hub) reconcileEventsLocked(rt *hostRT, users int) {
	if users > 0 {
		if rt.idle != nil {
			rt.idle.Stop()
			rt.idle = nil
			rt.idleGen++
		}
		if rt.watcher == nil {
			h.startWatcherLocked(rt)
		}
		return
	}
	if rt.watcher != nil && rt.idle == nil {
		rt.idleGen++
		gen, host := rt.idleGen, rt.id
		rt.idle = time.AfterFunc(h.opts.IdleStop, func() { h.idleStop(host, gen) })
	}
}

// idleStop ends the watcher of a host that has had no user for IdleStop.
func (h *Hub) idleStop(host string, gen int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rt := h.rts[host]
	if rt == nil || rt.idle == nil || rt.idleGen != gen || rt.watcher == nil {
		return
	}
	rt.idle = nil
	rt.watcher.stop()
	rt.watcher = nil
	rt.status = HostStatus{}
	h.gcLocked(rt)
}

// gcLocked forgets a runtime that has nothing left to run.
func (h *Hub) gcLocked(rt *hostRT) {
	if rt.watcher == nil && rt.idle == nil && len(rt.callbacks) == 0 && rt.st.idle() {
		delete(h.rts, rt.id)
	}
}

// setStatusLocked records a watcher's status and tells the host's clients; the
// same state and reason twice in a row is not repeated.
func (h *Hub) setStatusLocked(rt *hostRT, state, reason string) {
	if rt.status.State == state && rt.status.Reason == reason {
		return
	}
	rt.status = HostStatus{Host: rt.id, State: state, Reason: reason, T: h.opts.Now().UnixMilli()}
	h.publishLocked(rt.id, Message{Event: MsgHostStatus, Data: rt.status})
}

// status returns the watcher status of a host (for tests and diagnostics).
func (h *Hub) status(host string) HostStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	if rt := h.rts[host]; rt != nil {
		return rt.status
	}
	return HostStatus{}
}
