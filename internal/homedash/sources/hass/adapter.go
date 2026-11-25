package hass

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/security"
	"aurago/internal/tools"
)

const (
	// ReasonWebsocketUnavailable is the status reason while the adapter polls REST.
	ReasonWebsocketUnavailable = "websocket_unavailable"
	// ReasonInvalidURL reports a configured URL the websocket cannot use.
	ReasonInvalidURL = "invalid_url"

	wsFailuresBeforePolling = 3
)

// Catalog errors reach the hub (and possibly clients), so they never carry Home Assistant text.
var (
	errInvalidURL         = errors.New("home assistant: invalid url")
	errCatalogUnavailable = errors.New("home assistant: catalog unavailable")
)

// FactoryOptions are the adapter's dependencies; zero values take the defaults.
type FactoryOptions struct {
	Logger         *slog.Logger
	WS             WSOptions        // websocket timing; zero = the documented defaults
	Now            func() time.Time // default time.Now
	RetryWebsocket time.Duration    // REST polling lasts this long before one websocket attempt (5 min)
	PollOverride   time.Duration    // tests only: replaces poll_seconds
	FetchTimeout   time.Duration    // REST probe and poll timeout (10 s)
}

func (o FactoryOptions) withDefaults() FactoryOptions {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.RetryWebsocket <= 0 {
		o.RetryWebsocket = 5 * time.Minute
	}
	if o.FetchTimeout <= 0 {
		o.FetchTimeout = 10 * time.Second
	}
	return o
}

// Factory builds the adapter from the runtime configuration. It is disabled without a settings
// entry, when switched off (the server already combined the source switch, home_assistant.enabled,
// URL and token), or without a token or URL. It never blocks.
func Factory(o FactoryOptions) homedash.Factory {
	o = o.withDefaults()
	return func(cfg homedash.RuntimeConfig) (homedash.Source, bool) {
		set, ok := cfg.Sources[SourceID]
		if !ok || !set.Enabled {
			return nil, false
		}
		ha := tools.HAConfig{URL: set.Params[ParamURL], AccessToken: set.Params[ParamToken], ReadOnly: set.Params[ParamReadOnly] == "true",
			AllowedServices: slices.Clone(set.Lists[ListAllowedServices]), BlockedServices: slices.Clone(set.Lists[ListBlockedServices])}
		// A missing URL or token means disabled; a configured URL the websocket cannot use still builds
		// the adapter, which then reports unavailable/invalid_url (amendment X2).
		if ha.AccessToken == "" || strings.TrimSpace(ha.URL) == "" {
			return nil, false
		}
		return New(ha, set.PollSeconds, o), true
	}
}

// Adapter is the Home Assistant source (design-backend 8.1, decision D1).
type Adapter struct {
	ha   tools.HAConfig
	poll time.Duration
	o    FactoryOptions
	log  *slog.Logger

	mu          sync.Mutex // guards everything below; held while calling the sink, never across network I/O
	states      map[string]HAState
	reg         Registries
	ix          registryIndex
	complete    bool          // states hold a full list (get_states, a REST poll or a one-shot fetch)
	runStates   bool          // the current run has its own full list (Catalog serves from it)
	sendAll     bool          // the next full list resends every state (first list of a run)
	statusSent  bool          // the current run reported a status ("starting" only comes first)
	wsLive      bool          // the websocket delivered states and has not dropped since
	gen         int           // run generation; a stale detach is ignored
	sink        homedash.Sink // the current run's sink; nil outside Run
	interest    map[string]bool
	metaFP      map[string]string // entity id -> meta fingerprint of the last pushed catalog
	catalogFP   string
	staleMarked bool
}

var (
	_ homedash.Source         = (*Adapter)(nil)
	_ homedash.HistorySource  = (*Adapter)(nil)
	_ homedash.SnapshotSource = (*Adapter)(nil)
)

// New builds an adapter; pollSeconds outside 2..60 becomes 5. The token is registered as sensitive
// here, before any network use (the websocket client and the REST helper register it again).
func New(ha tools.HAConfig, pollSeconds int, o FactoryOptions) *Adapter {
	o = o.withDefaults()
	if ha.AccessToken != "" {
		security.RegisterSensitive(ha.AccessToken)
	}
	if pollSeconds < 2 || pollSeconds > 60 {
		pollSeconds = 5
	}
	poll := time.Duration(pollSeconds) * time.Second
	if o.PollOverride > 0 {
		poll = o.PollOverride
	}
	return &Adapter{ha: ha, poll: poll, o: o, log: o.Logger, states: map[string]HAState{}, ix: indexRegistries(Registries{}),
		interest: map[string]bool{}, metaFP: map[string]string{}}
}

func (a *Adapter) ID() homedash.SourceID { return SourceID }

// SetInterest stores the subscribed natives; REST polling runs only while there are some. While the
// source is stale, newly subscribed entities are marked stale too.
func (a *Adapter) SetInterest(natives []string) {
	set := make(map[string]bool, len(natives))
	for _, n := range natives {
		set[n] = true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	old := a.interest
	a.interest = set
	if a.sink != nil && a.staleMarked {
		var added []string
		for n := range set {
			if !old[n] {
				added = append(added, n)
			}
		}
		a.sendStaleLocked(added)
	}
}

func (a *Adapter) interested() bool { a.mu.Lock(); defer a.mu.Unlock(); return len(a.interest) > 0 }

func (a *Adapter) attach(sink homedash.Sink) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gen++
	a.sink, a.runStates, a.sendAll, a.statusSent, a.wsLive, a.staleMarked, a.catalogFP = sink, false, true, false, false, false, ""
	return a.gen
}

func (a *Adapter) detach(gen int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gen == gen {
		a.sink, a.runStates, a.wsLive = nil, false, false
	}
}

func (a *Adapter) statusLocked(state, reason string) {
	if a.sink != nil {
		a.statusSent = true
		a.sink.Status(homedash.SourceStatus{Source: SourceID, State: state, Reason: reason, At: a.o.Now().UTC()})
	}
}

func (a *Adapter) status(state, reason string) {
	a.mu.Lock()
	a.statusLocked(state, reason)
	a.mu.Unlock()
}

// degradeLocked reports a lost connection and marks the subscribed entities stale (once).
func (a *Adapter) degradeLocked(reason string) {
	a.statusLocked(homedash.StatusDegraded, reason)
	a.markStaleLocked()
}

func (a *Adapter) degrade(reason string) { a.mu.Lock(); a.degradeLocked(reason); a.mu.Unlock() }

func (a *Adapter) setWSLive(v bool) { a.mu.Lock(); a.wsLive = v; a.mu.Unlock() }

// Catalog works whether a run is live or not. A live run serves its own states; otherwise one
// websocket session reads states and registries (REST /api/states without registries when the
// websocket cannot be used; the last known registries then apply). No lock is held during I/O.
func (a *Adapter) Catalog(ctx context.Context) ([]homedash.Entity, error) {
	a.mu.Lock()
	if a.sink != nil && a.runStates {
		list := buildCatalog(a.ha, a.states, a.ix)
		a.mu.Unlock()
		return list, nil
	}
	a.mu.Unlock()
	states, reg, err := a.fetchOnce(ctx)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if errors.Is(err, errInvalidURL) {
			a.log.Debug("homedash: Home Assistant URL is not usable") // never log the URL: it may carry credentials
			return nil, errInvalidURL
		}
		a.log.Debug("homedash: Home Assistant catalog read failed", "error", err)
		return nil, errCatalogUnavailable
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !(a.sink != nil && a.runStates) { // a run that became live meanwhile wins
		next := make(map[string]HAState, len(states))
		for _, st := range states {
			if NativeIDValid(st.EntityID) {
				next[st.EntityID] = st
			}
		}
		a.states, a.complete = next, true
		if reg != nil {
			a.reg, a.ix = *reg, indexRegistries(*reg)
		}
	}
	return buildCatalog(a.ha, a.states, a.ix), nil
}

func (a *Adapter) fetchOnce(ctx context.Context) ([]HAState, *Registries, error) {
	c, err := NewWSClient(a.ha.URL, a.ha.AccessToken, nopHandler{}, a.o.WS)
	if err != nil {
		return nil, nil, errInvalidURL // no network attempt at all
	}
	states, reg, ferr := c.Fetch(ctx)
	if ferr == nil {
		return states, &reg, nil
	}
	if errors.Is(ferr, errAuthInvalid) || ctx.Err() != nil {
		return nil, nil, ferr
	}
	var list []HAState
	if rerr := tools.HARequestJSON(ctx, a.ha, http.MethodGet, "/api/states", nil, &list); rerr != nil {
		return nil, nil, errors.Join(ferr, rerr)
	}
	return list, nil, nil
}

func metaFingerprint(e homedash.Entity) string {
	e.State = homedash.State{}
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// pushCatalogLocked sends the catalog when its meta (not only states) changed since the last push.
func (a *Adapter) pushCatalogLocked() {
	if a.sink == nil || !a.complete {
		return
	}
	list := buildCatalog(a.ha, a.states, a.ix)
	fps := make(map[string]string, len(list))
	h := sha256.New()
	for _, e := range list {
		fp := metaFingerprint(e)
		fps[string(e.ID)] = fp
		h.Write([]byte(string(e.ID) + "=" + fp + ";"))
	}
	sum := hex.EncodeToString(h.Sum(nil))
	if sum == a.catalogFP {
		return
	}
	a.catalogFP, a.metaFP = sum, fps
	a.sink.Catalog(list)
}

func (a *Adapter) metaChangedLocked(st HAState) bool {
	id := string(homedash.MakeEntityID(SourceID, st.EntityID))
	e, ok := entityFor(a.ha, st, a.ix)
	if !ok {
		_, had := a.metaFP[id]
		return had
	}
	return a.metaFP[id] != metaFingerprint(e)
}

func sameHAState(x, y HAState) bool { return x.State == y.State && x.LastUpdated.Equal(y.LastUpdated) }

// relevantLocked reports whether the hub could keep a state of this entity (mapped domain, not
// hidden or disabled in the registry).
func (a *Adapter) relevantLocked(id string) bool {
	d, ok := domainOf(id)
	if !ok {
		return false
	}
	_, mapped := MapDomain(d)
	return mapped && !a.ix.hidden(id)
}

// replaceStatesLocked installs a full state list (get_states or a REST poll): catalog first when its
// meta changed (the hub keeps states only for cataloged entities), then updates for the states that
// changed or were marked stale (a catalog push alone sends no state frames). The first list of a run
// resends everything.
func (a *Adapter) replaceStatesLocked(list []HAState) {
	prev := a.states
	next := make(map[string]HAState, len(list))
	for _, st := range list {
		if NativeIDValid(st.EntityID) {
			next[st.EntityID] = st
		}
	}
	a.states, a.complete = next, true
	if a.sink == nil {
		return
	}
	a.pushCatalogLocked()
	all := a.sendAll || a.staleMarked
	var ups []homedash.StateUpdate
	for id, st := range next {
		if old, ok := prev[id]; ok && !all && sameHAState(old, st) {
			continue
		}
		if !a.relevantLocked(id) {
			continue
		}
		d, _ := domainOf(id)
		ups = append(ups, homedash.StateUpdate{Native: id, State: mapState(d, st)})
	}
	a.sendAll, a.staleMarked = false, false
	if len(ups) > 0 {
		a.sink.Update(ups...)
	}
}

// applyStateLocked installs one state (state_changed or a service answer). An older state is
// ignored; the same state twice (answer, then event) is sent once.
func (a *Adapter) applyStateLocked(st HAState) {
	if !NativeIDValid(st.EntityID) {
		return
	}
	old, had := a.states[st.EntityID]
	if had && !st.LastUpdated.IsZero() && st.LastUpdated.Before(old.LastUpdated) {
		return
	}
	if had && sameHAState(old, st) && !a.staleMarked {
		return
	}
	a.states[st.EntityID] = st
	if a.sink == nil {
		return
	}
	if !had || a.metaChangedLocked(st) {
		a.pushCatalogLocked()
	}
	if !a.relevantLocked(st.EntityID) {
		return
	}
	d, _ := domainOf(st.EntityID)
	a.sink.Update(homedash.StateUpdate{Native: st.EntityID, State: mapState(d, st)})
}

func (a *Adapter) removeStateLocked(id string) {
	if _, ok := a.states[id]; !ok {
		return
	}
	delete(a.states, id)
	a.pushCatalogLocked()
}

// markStaleLocked sends the subscribed entities once with stale:true (design 5: push sources mark
// their entities stale while degraded). The next full list clears it.
func (a *Adapter) markStaleLocked() {
	if a.sink == nil || a.staleMarked {
		return
	}
	a.staleMarked = true
	natives := make([]string, 0, len(a.interest))
	for n := range a.interest {
		natives = append(natives, n)
	}
	a.sendStaleLocked(natives)
}

func (a *Adapter) sendStaleLocked(natives []string) {
	var ups []homedash.StateUpdate
	for _, native := range natives {
		st, ok := a.states[native]
		d, okd := domainOf(native)
		if !ok || !okd {
			continue
		}
		s := mapState(d, st)
		s.Stale = true
		ups = append(ups, homedash.StateUpdate{Native: native, State: s})
	}
	if len(ups) > 0 {
		a.sink.Update(ups...)
	}
}

func (a *Adapter) markStale() { a.mu.Lock(); a.markStaleLocked(); a.mu.Unlock() }

// wsBridge feeds websocket callbacks into the adapter (all on the client's Run goroutine). limit is
// the number of failed handshakes in a row that triggers a REST probe; it is guarded by a.mu.
type wsBridge struct {
	a     *Adapter
	limit int
	probe chan string // carries the failure reason; the Run goroutine answers with a REST poll
}

func (b *wsBridge) OnStates(list []HAState) {
	a := b.a
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runStates = true
	a.replaceStatesLocked(list)
}

func (b *wsBridge) OnStateChanged(sc StateChanged) {
	a := b.a
	a.mu.Lock()
	defer a.mu.Unlock()
	if sc.NewState == nil {
		a.removeStateLocked(sc.EntityID)
		return
	}
	st := *sc.NewState
	if st.EntityID == "" {
		st.EntityID = sc.EntityID
	}
	a.applyStateLocked(st)
}

func (b *wsBridge) OnRegistries(reg Registries) {
	a := b.a
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reg, a.ix = reg, indexRegistries(reg)
	a.pushCatalogLocked()
}

// OnStatus maps client statuses to source statuses (contract section 3). After `limit` failed
// handshakes in a row the status is withheld and a REST probe decides: polling when REST answers,
// degraded when it does not.
func (b *wsBridge) OnStatus(st WSStatus) {
	a := b.a
	a.mu.Lock()
	defer a.mu.Unlock()
	switch st.State {
	case WSLive:
		a.wsLive = true
		b.limit = wsFailuresBeforePolling
		a.statusLocked(homedash.StatusLive, "")
	case WSConnecting:
		if !a.statusSent {
			a.statusLocked(homedash.StatusStarting, "")
		}
	case WSDegraded:
		a.wsLive = false
		if st.Failures >= b.limit {
			select {
			case b.probe <- st.Reason:
			default:
			}
			return
		}
		a.degradeLocked(st.Reason)
	case WSAuthFailed:
		a.wsLive = false
		a.statusLocked(homedash.StatusUnavailable, ReasonAuthFailed)
		a.markStaleLocked()
	}
}

// Run connects while the hub has interest. It uses the websocket first. After
// wsFailuresBeforePolling failed handshakes in a row while GET /api/states still answers (a proxy
// that blocks websockets), it polls REST for RetryWebsocket, then makes one websocket attempt, and
// so on. A failing poll means Home Assistant itself is down: the websocket client's backoff
// (1-60 s, +-20 %) takes over again. Every goroutine Run starts has ended when it returns, and the
// sink is not used afterwards.
func (a *Adapter) Run(ctx context.Context, sink homedash.Sink) error {
	gen := a.attach(sink)
	defer a.detach(gen)
	limit := wsFailuresBeforePolling
	for ctx.Err() == nil {
		if !a.websocketPhase(ctx, limit) {
			return nil
		}
		if a.pollingPhase(ctx) {
			limit = 1
		} else {
			limit = wsFailuresBeforePolling
		}
	}
	return nil
}

// websocketPhase runs the websocket client until ctx ends (false) or the client failed limit
// handshakes in a row while a REST poll succeeds (true: switch to polling). The client's callbacks
// have returned when it returns.
func (a *Adapter) websocketPhase(ctx context.Context, limit int) bool {
	probe := make(chan string, 1)
	client, err := NewWSClient(a.ha.URL, a.ha.AccessToken, &wsBridge{a: a, limit: limit, probe: probe}, a.o.WS)
	if err != nil {
		a.status(homedash.StatusUnavailable, ReasonInvalidURL)
		<-ctx.Done()
		return false
	}
	wctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = client.Run(wctx) }()
	defer func() {
		cancel()
		<-done
		a.setWSLive(false)
	}()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-done:
			return false
		case reason := <-probe:
			applied, err := a.pollOnce(ctx)
			switch {
			case err != nil && ctx.Err() == nil:
				a.degrade(reason) // Home Assistant itself is unreachable
			case err == nil && applied:
				return true
			}
		}
	}
}
