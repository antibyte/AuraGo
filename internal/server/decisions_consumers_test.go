package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

// lifecycleDecisions counts every call a passive snapshot must never make.
type lifecycleDecisions struct {
	*fakeDecisions
	lifecycle int
}

func (l *lifecycleDecisions) Install(context.Context) error   { l.lifecycle++; return nil }
func (l *lifecycleDecisions) Start(context.Context) error     { l.lifecycle++; return nil }
func (l *lifecycleDecisions) AutoStart(context.Context) error { l.lifecycle++; return nil }

func hubLocal(t *testing.T) (*Server, *fakeDecisions) {
	t.Helper()
	s, fake := decisionsServerFixture(t)
	fake.status.ProjectorSHA256, fake.status.ProjectorRevision = "sha", "rev"
	fake.status.Runtime.ProjectorSHA256, fake.status.Runtime.ProjectorRevision = "sha", "rev"
	return s, fake
}

func enable(s *Server, sw config.DecisionConsumerSwitch, ids ...string) {
	for _, id := range ids {
		switch id {
		case "person_wake":
			s.Cfg.DecisionConsumers.PersonWake = sw
		case "mail_triage":
			s.Cfg.DecisionConsumers.MailTriage = sw
		}
	}
}

func TestCapabilitiesLocalProjector(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*decisions.Status)
		images consumers.ImageSupport
		cpu    bool
	}{
		{"supported", func(st *decisions.Status) { st.CPUFallbackActive = true }, consumers.ImagesSupported, true},
		{"no projector", func(st *decisions.Status) { st.ProjectorSHA256 = "" }, consumers.ImagesUnsupported, false},
		{"not ready", func(st *decisions.Status) { st.Ready = false }, consumers.ImagesUnknown, false},
		{"unattested", func(st *decisions.Status) { st.Runtime.ProjectorRevision = "other" }, consumers.ImagesUnknown, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, fake := hubLocal(t)
			tc.mutate(&fake.status)
			caps := s.decisionHub().capabilities(s.Cfg.Decisions.Normalize(), fake.Status(), 0, 0)
			if caps.Images != tc.images || caps.CPUOnly != tc.cpu || caps.External {
				t.Fatalf("caps = %+v", caps)
			}
		})
	}
}

func TestCapabilitiesProviderStaleIsUnknown(t *testing.T) {
	s, d, _, _ := routedDecisionFixture(t)
	set := func(a decisions.ModelAvailability) {
		d.mu.Lock()
		d.generation.status.ModelAvailability = a
		d.mu.Unlock()
	}
	caps := func() consumers.Capabilities {
		return s.decisionHub().capabilities(s.Cfg.Decisions.Normalize(), d.Status(), 0, 0)
	}
	set(decisions.ModelAvailability{State: "usable", CheckedAt: time.Now().Add(-48 * time.Hour), Model: &decisions.DecisionModel{SupportsImages: true}})
	if c := caps(); c.Images != consumers.ImagesUnknown || !c.External {
		t.Fatalf("stale catalog: %+v", c)
	}
	set(decisions.ModelAvailability{State: "usable", CheckedAt: time.Now(), Model: &decisions.DecisionModel{SupportsImages: true}})
	if c := caps(); c.Images != consumers.ImagesSupported {
		t.Fatalf("fresh with images: %+v", c)
	}
	set(decisions.ModelAvailability{State: "usable", CheckedAt: time.Now(), Model: &decisions.DecisionModel{}})
	if c := caps(); c.Images != consumers.ImagesUnsupported {
		t.Fatalf("fresh without images: %+v", c)
	}
}

func TestImageRequiredStaysOff(t *testing.T) {
	snap := func(s *Server) consumers.Snapshot { return s.decisionHub().Snapshot(s.Cfg, consumers.PersonWake) }
	check := func(t *testing.T, got consumers.Snapshot, code string, available, allowed bool) {
		t.Helper()
		if got.Code != code || got.Available != available || got.ImagesAllowed != allowed || (got.Evaluate != nil) != available {
			t.Fatalf("snapshot code=%q available=%v allowed=%v evaluate=%v", got.Code, got.Available, got.ImagesAllowed, got.Evaluate != nil)
		}
	}
	t.Run("local unsupported", func(t *testing.T) {
		s, fake := hubLocal(t)
		enable(s, config.DecisionConsumerSwitch{Enabled: true}, "person_wake")
		fake.status.ProjectorSHA256 = ""
		check(t, snap(s), decisions.CodeImagesUnsupported, false, false)
	})
	t.Run("local unknown", func(t *testing.T) {
		s, fake := hubLocal(t)
		enable(s, config.DecisionConsumerSwitch{Enabled: true}, "person_wake")
		fake.status.Runtime.ProjectorRevision = "other"
		check(t, snap(s), consumers.CodeImagesUnknown, false, false)
	})
	t.Run("local supported", func(t *testing.T) {
		s, _ := hubLocal(t)
		enable(s, config.DecisionConsumerSwitch{Enabled: true}, "person_wake")
		check(t, snap(s), consumers.CodeAvailable, true, true)
	})
	external := func(t *testing.T, allow bool) *Server {
		s, d, _, _ := routedDecisionFixture(t)
		enable(s, config.DecisionConsumerSwitch{Enabled: true, AllowExternalImages: allow}, "person_wake")
		d.mu.Lock()
		d.generation.status.ModelAvailability = decisions.ModelAvailability{State: "usable", CheckedAt: time.Now(), Model: &decisions.DecisionModel{SupportsImages: true}}
		d.mu.Unlock()
		return s
	}
	t.Run("external not permitted", func(t *testing.T) {
		check(t, snap(external(t, false)), consumers.CodeImagesExternalNotPermitted, false, false)
	})
	t.Run("external permitted", func(t *testing.T) {
		check(t, snap(external(t, true)), consumers.CodeAvailable, true, true)
	})
}

func TestConsumerDisabled(t *testing.T) {
	s, _ := hubLocal(t)
	hub := s.decisionHub()
	if got := hub.Snapshot(s.Cfg, consumers.MailTriage); got.Code != consumers.CodeConsumerDisabled || got.Available {
		t.Fatalf("off by default: %+v", got)
	}
	if got := hub.Snapshot(s.Cfg, consumers.ID("nope")); got.Code != consumers.CodeConsumerDisabled || got.Available {
		t.Fatalf("unknown id: %+v", got)
	}
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
	if got := hub.Snapshot(s.Cfg, consumers.MailTriage); !got.Available || got.Code != consumers.CodeAvailable {
		t.Fatalf("enabled: %+v", got)
	}
}

// TestRoutingIdentityStableAcrossConsumerToggle also covers the provider
// generation: a consumer switch must never rotate it.
func TestConsumerToggleDoesNotRotateProviderGeneration(t *testing.T) {
	s, d, _, _ := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	generation := d.generation
	before := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	mail := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
	next := *s.Cfg
	next.DecisionConsumers.MailTriage.Enabled = true
	s.decisionHub().configChanged(s.Cfg, &next)
	s.Cfg = &next
	s.cfgSnapshot.Store(&next)
	if d.generation != generation || !before.Current() {
		t.Fatal("consumer toggle rotated the provider generation or revoked the router")
	}
	if after := (taskDecisionService{s}).RoutingSnapshot(s.Cfg); after.Identity != before.Identity || !after.Available {
		t.Fatal("router identity moved with a consumer toggle")
	}
	if mail.Available {
		t.Fatal("disabled consumer was available")
	}
	enabled := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
	off := next
	off.DecisionConsumers.MailTriage.Enabled = false
	s.decisionHub().configChanged(&next, &off)
	s.Cfg = &off
	s.cfgSnapshot.Store(&off)
	if enabled.Current() || !before.Current() {
		t.Fatal("toggle must revoke only the toggled consumer")
	}
}

func TestBudgetBlockedFallsBack(t *testing.T) {
	s, _ := hubLocal(t)
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
	s.Cfg.Budget.Enabled = true
	s.Cfg.Budget.DailyLimitUSD = .01
	s.Cfg.Budget.Enforcement = "full"
	s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	t.Cleanup(s.BudgetTracker.Flush)
	if !s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage).Available {
		t.Fatal("blocked before any spend")
	}
	s.BudgetTracker.RecordCost(1)
	got := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
	if got.Available || got.Code != consumers.CodeBudgetBlocked || got.Evaluate != nil {
		t.Fatalf("budget: %+v", got)
	}
}

func TestTextOnlyDropsImages(t *testing.T) {
	s, fake := hubLocal(t)
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
	snapshot := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
	if _, err := snapshot.Evaluate(context.Background(), decisions.Request{Images: []decisions.Image{{MIME: "image/png"}}}); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 || len(fake.request.Images) != 0 {
		t.Fatalf("calls=%d images=%d", fake.calls, len(fake.request.Images))
	}
	if st := s.decisionHub().Stats()[consumers.MailTriage]; st.ImagesSkipped != 1 || st.Attempts != 1 || st.Successes != 1 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestImagesUnsupportedPinsEpoch(t *testing.T) {
	s, d, local, remote := routedDecisionFixture(t)
	enable(s, config.DecisionConsumerSwitch{Enabled: true, AllowExternalImages: true}, "person_wake")
	fresh := func() {
		d.mu.Lock()
		d.generation.status.ModelAvailability = decisions.ModelAvailability{State: "usable", CheckedAt: time.Now(), Model: &decisions.DecisionModel{SupportsImages: true}}
		d.mu.Unlock()
	}
	fresh()
	first := s.decisionHub().Snapshot(s.Cfg, consumers.PersonWake)
	if !first.Available {
		t.Fatalf("setup: %+v", first)
	}
	remote.evaluate = func(context.Context) (decisions.Response, error) {
		return decisions.Response{}, decisions.NewError(decisions.CodeImagesUnsupported, nil)
	}
	if _, err := first.Evaluate(context.Background(), decisions.Request{Images: []decisions.Image{{MIME: "image/png"}}}); err == nil {
		t.Fatal("expected the image error")
	}
	if got := s.decisionHub().Snapshot(s.Cfg, consumers.PersonWake); got.Available || got.Code != decisions.CodeImagesUnsupported {
		t.Fatalf("after pin: %+v", got)
	}
	if st := s.decisionHub().Stats()[consumers.PersonWake]; st.Failures != 1 || st.LastCode != decisions.CodeImagesUnsupported {
		t.Fatalf("stats: %+v", st)
	}
	// A new generation (model switch) starts unpinned.
	d.mu.Lock()
	d.generation = d.newGeneration(s.Cfg)
	d.mu.Unlock()
	fresh()
	if got := s.decisionHub().Snapshot(s.Cfg, consumers.PersonWake); !got.Available {
		t.Fatalf("pin leaked into the next generation: %+v", got)
	}
	// So does the local backend, even at the same epoch number.
	local.status.ProjectorSHA256, local.status.ProjectorRevision = "sha", "rev"
	local.status.Runtime.ProjectorSHA256, local.status.Runtime.ProjectorRevision = "sha", "rev"
	cfg := s.Cfg.Decisions.Normalize()
	cfg.Backend = "local"
	d.mu.Lock()
	id := d.generation.id
	d.mu.Unlock()
	if caps := s.decisionHub().capabilities(cfg, local.Status(), id, 0); caps.Images != consumers.ImagesSupported {
		t.Fatalf("pin leaked into the local backend: %+v", caps)
	}
}

func TestStatusConsumersPassive(t *testing.T) {
	s, fake := hubLocal(t)
	wrapped := &lifecycleDecisions{fakeDecisions: fake}
	s.Decisions = wrapped
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	enable(s, config.DecisionConsumerSwitch{Enabled: true, AllowExternalImages: true}, "person_wake", "mail_triage")
	for id := range consumers.Registry {
		s.decisionHub().Snapshot(s.Cfg, id)
	}
	if fake.calls != 0 || wrapped.lifecycle != 0 {
		t.Fatalf("snapshots acted: evaluate=%d lifecycle=%d", fake.calls, wrapped.lifecycle)
	}
}

// gatedDecisions reports a configurable manager load and can block Evaluate
// until its context ends or release is closed.
type gatedDecisions struct {
	*fakeDecisions
	active, waiting int
	block           bool
	started         chan struct{}
	release         chan struct{}
	evaluated       atomic.Int32
}

func newGated(f *fakeDecisions) *gatedDecisions {
	return &gatedDecisions{fakeDecisions: f, started: make(chan struct{}, 4), release: make(chan struct{})}
}

func (g *gatedDecisions) Load() (int, int) { return g.active, g.waiting }
func (g *gatedDecisions) Evaluate(ctx context.Context, r decisions.Request) (decisions.Response, error) {
	g.evaluated.Add(1)
	if g.block {
		g.block = false
		g.started <- struct{}{}
		select {
		case <-ctx.Done():
			return decisions.Response{}, decisions.NewError(decisions.CodeCancelled, ctx.Err())
		case <-g.release:
		}
	}
	return decisions.Response{}, nil
}

func routerOn(s *Server) {
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
}

func TestBackgroundDefersWhenManagerBusy(t *testing.T) {
	s, fake := hubLocal(t)
	g := newGated(fake)
	g.active = 1
	s.Decisions = g
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
	snap := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
	_, err := snap.Evaluate(context.Background(), decisions.Request{})
	if decisions.ErrorCode(err) != consumers.CodeAdmissionDeferred || g.evaluated.Load() != 0 {
		t.Fatalf("err=%v evaluated=%d", err, g.evaluated.Load())
	}
	if st := s.decisionHub().Stats()[consumers.MailTriage]; st.Deferred != 1 {
		t.Fatalf("stats %+v", st)
	}
	g.active = 0
	if _, err := snap.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatal(err)
	}
}

func TestBackgroundDefersWhenInteractiveInFlight(t *testing.T) {
	s, fake := hubLocal(t)
	g := newGated(fake)
	g.block = true
	s.Decisions = g
	routerOn(s)
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
	router := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	done := make(chan error, 1)
	go func() { _, err := router.Evaluate(context.Background(), decisions.Request{}); done <- err }()
	<-g.started
	mail := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
	if _, err := mail.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != consumers.CodeAdmissionDeferred {
		t.Fatalf("err=%v", err)
	}
	close(g.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := mail.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatalf("after interactive: %v", err)
	}
}

func TestBackgroundMinInterval(t *testing.T) {
	s, fake := hubLocal(t)
	s.Decisions = newGated(fake)
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "person_wake")
	hub := s.decisionHub()
	clock := time.Now()
	hub.now = func() time.Time { return clock }
	snap := hub.Snapshot(s.Cfg, consumers.PersonWake)
	if _, err := snap.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(time.Second)
	if _, err := snap.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != consumers.CodeAdmissionDeferred {
		t.Fatalf("second call within 2s: %v", err)
	}
	clock = clock.Add(2 * time.Second)
	if _, err := snap.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatalf("after interval: %v", err)
	}
}

func TestInteractivePreemptsLocalBackground(t *testing.T) {
	s, fake := hubLocal(t)
	g := newGated(fake)
	g.block = true
	s.Decisions = g
	routerOn(s)
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
	mail := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
	done := make(chan error, 1)
	go func() { _, err := mail.Evaluate(context.Background(), decisions.Request{}); done <- err }()
	<-g.started
	router := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	if _, err := router.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatalf("router: %v", err)
	}
	select {
	case err := <-done:
		if decisions.ErrorCode(err) != consumers.CodePreempted {
			t.Fatalf("background err=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("background was not preempted")
	}
	if st := s.decisionHub().Stats()[consumers.MailTriage]; st.Preempted != 1 {
		t.Fatalf("stats %+v", st)
	}
}

func TestExternalReservedInteractiveSlot(t *testing.T) {
	setup := func(t *testing.T) (*Server, *fakeDecisionProvider, chan struct{}, chan struct{}) {
		s, d, _, remote := routedDecisionFixture(t)
		d.reserved = make(chan struct{}, 1)
		routerOn(s)
		enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
		started, release := make(chan struct{}, 2), make(chan struct{})
		var first atomic.Bool
		remote.evaluate = func(ctx context.Context) (decisions.Response, error) {
			if first.CompareAndSwap(false, true) {
				started <- struct{}{}
				select {
				case <-ctx.Done():
				case <-release:
				}
			}
			return decisions.Response{}, nil
		}
		return s, remote, started, release
	}
	t.Run("API call holds busy", func(t *testing.T) {
		s, _, started, release := setup(t)
		done := make(chan error, 1)
		call := func() error { _, err := s.Decisions.Evaluate(context.Background(), decisions.Request{}); return err }
		go func() { done <- call() }()
		<-started
		if err := call(); decisions.ErrorCode(err) != decisions.CodeQueueFull {
			t.Fatalf("second API call: %v", err)
		}
		router := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
		if _, err := router.Evaluate(context.Background(), decisions.Request{}); err != nil {
			t.Fatalf("router: %v", err)
		}
		close(release)
		<-done
	})
	t.Run("background holds busy", func(t *testing.T) {
		s, d, started, release := setup(t)
		_ = d
		mail := s.decisionHub().Snapshot(s.Cfg, consumers.MailTriage)
		if !mail.Available {
			t.Fatalf("setup: %+v", mail)
		}
		done := make(chan error, 1)
		go func() { _, err := mail.Evaluate(context.Background(), decisions.Request{}); done <- err }()
		<-started
		if _, err := mail.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != consumers.CodeAdmissionDeferred {
			t.Fatalf("second background: %v", err)
		}
		router := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
		if _, err := router.Evaluate(context.Background(), decisions.Request{}); err != nil {
			t.Fatalf("router: %v", err)
		}
		close(release)
		if err := <-done; decisions.ErrorCode(err) == consumers.CodePreempted {
			t.Fatalf("paid external call was cut: %v", err)
		}
	})
}

func TestDecisionsStatusListsConsumersPassively(t *testing.T) {
	s, fake := hubLocal(t)
	wrapped := &lifecycleDecisions{fakeDecisions: fake}
	s.Decisions = wrapped
	enable(s, config.DecisionConsumerSwitch{Enabled: true}, "mail_triage")
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/decisions/status", nil))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		State        string           `json:"state"`
		Capabilities map[string]any   `json:"capabilities"`
		Consumers    []map[string]any `json:"consumers"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "ready" || got.Capabilities["images"] != "supported" || len(got.Consumers) != 9 {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if got.Consumers[0]["id"] != "router" || got.Consumers[4]["id"] != "mail_triage" || got.Consumers[4]["enabled"] != true || got.Consumers[4]["available"] != true {
		t.Fatalf("consumers: %v", got.Consumers)
	}
	if got.Consumers[5]["enabled"] != false || got.Consumers[5]["code"] != consumers.CodeConsumerDisabled {
		t.Fatalf("webhook row: %v", got.Consumers[5])
	}
	if fake.calls != 0 || wrapped.lifecycle != 0 {
		t.Fatalf("status acted: evaluate=%d lifecycle=%d", fake.calls, wrapped.lifecycle)
	}
}
