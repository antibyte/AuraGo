package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/decisions"
)

func TestLLMRouterDecisionsPassiveAndAccounting(t *testing.T) {
	s, d, local, remote := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	s.Cfg.Budget.Enabled = true
	s.Cfg.Budget.DailyLimitUSD = 1
	s.Cfg.Budget.Enforcement = "full"
	s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	t.Cleanup(s.BudgetTracker.Flush)
	snapshot := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	if !snapshot.Available || remote.calls.Load() != 0 || remote.catalogCalls.Load() != 0 || local.calls != 0 {
		t.Fatal("passive availability probed or misreported provider")
	}
	if _, err := snapshot.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatal(err)
	}
	if s.BudgetTracker.CategorySpendUSD("routing") != .025 || s.BudgetTracker.CategorySpendUSD("decisions") != 0 || s.BudgetTracker.GetStatus().SpentUSD != .025 {
		t.Fatal("routing cost booked incorrectly")
	}
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatal(err)
	}
	if s.BudgetTracker.CategorySpendUSD("routing") != .025 || s.BudgetTracker.CategorySpendUSD("decisions") != .025 {
		t.Fatal("ordinary Decisions accounting changed")
	}
	encoded, _ := json.Marshal(snapshot)
	if strings.Contains(string(encoded), "test-only") || strings.Contains(string(encoded), "Identity") || strings.Contains(string(encoded), "Evaluate") {
		t.Fatal("private snapshot data exposed")
	}
	// Replacing credentials invalidates captured work even when the model is unchanged.
	next := *s.Cfg
	next.Providers = append([]config.ProviderEntry(nil), s.Cfg.Providers...)
	next.Providers[0].APIKey = "rotated-test-only"
	cleanup, err := d.prepareConfiguration(&next)
	if err != nil {
		t.Fatal(err)
	}
	s.Cfg = &next
	s.cfgSnapshot.Store(&next)
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if snapshot.Current() {
		t.Fatal("credential generation remained valid")
	}
	if _, err := snapshot.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != "configuration_changed" {
		t.Fatal("stale capability evaluated")
	}
	fresh := (taskDecisionService{s}).RoutingSnapshot(&next)
	if fresh.Identity == snapshot.Identity {
		t.Fatal("cache identity survived rotation")
	}
	cleanup = d.invalidateProvider("cloud")
	if cleanup != nil {
		if err := cleanup(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if fresh.Current() {
		t.Fatal("revoked capability remained valid")
	}
}

func TestLLMRouterDecisionsUnavailableDoesNotStartLocal(t *testing.T) {
	s, local := decisionsServerFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	local.status.Ready = false
	snapshot := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	if snapshot.Available || snapshot.Evaluate != nil || local.calls != 0 {
		t.Fatal("stopped local runtime was activated")
	}
	manager := decisions.NewManager(s.Cfg.Decisions, t.TempDir())
	s.Decisions = manager
	before := manager.CacheGeneration()
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if manager.CacheGeneration() == before {
		t.Fatal("stop failed to invalidate local generation")
	}
	if (taskDecisionService{s}).RoutingSnapshot(s.Cfg).Available {
		t.Fatal("stopped manager reported available")
	}
}

func TestLLMRouterDecisionsConfigurationPublicationBoundary(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	old := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	next := *s.Cfg
	next.Decisions.Model = "experimental/jeff-2b"
	cleanup, err := d.prepareConfiguration(&next)
	if err != nil {
		t.Fatal(err)
	}
	// The service policy is published before the server's immutable config.
	inBetween := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	if inBetween.Available || old.Current() || remote.calls.Load() != 0 {
		t.Fatal("snapshot admitted work across configuration publication")
	}
	s.Cfg = &next
	s.cfgSnapshot.Store(&next)
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	fresh := (taskDecisionService{s}).RoutingSnapshot(&next)
	if fresh.Identity == old.Identity {
		t.Fatal("backend/model generation reused a cache identity")
	}
}

func TestLLMRouterDecisionsRejectedResponseRetainsOnlyUsage(t *testing.T) {
	s, _, _, remote := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	cost := .025
	remote.evaluate = func(context.Context) (decisions.Response, error) {
		return decisions.Response{Usage: decisions.Usage{CostUSD: &cost, PromptTokens: 42}, Answers: map[string]decisions.Answer{"task_area": {Decision: "coding"}}}, decisions.NewError("external_outcome_unknown", nil)
	}
	response, err := (taskDecisionService{s}).RoutingSnapshot(s.Cfg).Evaluate(context.Background(), decisions.Request{})
	if err == nil || response.Usage.CostUSD == nil || *response.Usage.CostUSD != cost || response.Usage.PromptTokens != 42 || len(response.Answers) != 0 {
		t.Fatal("rejected inference leaked answers or lost reported usage")
	}
}

func TestLLMRouterDecisionsDisableReenableRevokesSnapshot(t *testing.T) {
	for _, toggle := range []string{"router", "decisions"} {
		t.Run(toggle, func(t *testing.T) {
			s, _, local, remote := routedDecisionFixture(t)
			s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
			s.Cfg.LLMRouter.Enabled = true
			original := *s.Cfg
			old := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
			disabled := original
			if toggle == "router" {
				disabled.LLMRouter.Enabled = false
			} else {
				disabled.LLMRouter.DecisionsFallback = false
			}
			s.replaceConfigSnapshot(&disabled)
			s.replaceConfigSnapshot(&original)
			fresh := (taskDecisionService{s}).RoutingSnapshot(s.ConfigSnapshot())
			if old.Current() || !fresh.Available || old.Identity == fresh.Identity || remote.calls.Load() != 0 || local.calls != 0 {
				t.Fatal("switching off and on restored a stale classification or started the service")
			}
		})
	}
}

func TestLLMRouterDecisionsRevocationDiscardsLateResult(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
	s.Cfg.LLMRouter.Enabled = true
	started := make(chan struct{})
	release := make(chan struct{})
	remote.evaluate = func(ctx context.Context) (decisions.Response, error) {
		close(started)
		<-ctx.Done()
		<-release
		return decisions.Response{}, nil
	}
	snapshot := (taskDecisionService{s}).RoutingSnapshot(s.Cfg)
	finished := make(chan error, 1)
	go func() { _, err := snapshot.Evaluate(context.Background(), decisions.Request{}); finished <- err }()
	<-started
	next := *s.Cfg
	next.Decisions.Enabled = false
	cleanup, err := d.prepareConfiguration(&next)
	if err != nil {
		t.Fatal(err)
	}
	s.Cfg = &next
	s.cfgSnapshot.Store(&next)
	close(release)
	select {
	case err := <-finished:
		if decisions.ErrorCode(err) != "configuration_changed" {
			t.Fatalf("late result: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("revocation failed to cancel classification")
	}
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLLMRouterDecisionsPreviewContract(t *testing.T) {
	s := routerServerFixture(t)
	s.Cfg.Decisions.Enabled = true
	fake := &fakeDecisions{status: decisions.Status{Enabled: true, Ready: true, Installed: true}}
	s.Decisions = fake
	for _, tc := range []struct {
		body        string
		code, calls int
	}{
		{`{"text":"Bitte mach daraus etwas Passendes"}`, 200, 0},
		{`{"text":"Bitte mach daraus etwas Passendes","classifier":"local"}`, 200, 0},
		{`{"text":"Bitte mach daraus etwas Passendes","helper":true,"classifier":"decisions"}`, 400, 0},
		{`{"text":"Bitte mach daraus etwas Passendes","classifier":"other"}`, 400, 0},
		{`{"text":"Bitte mach daraus etwas Passendes","classifier":"decisions"}`, 200, 1},
	} {
		w := httptest.NewRecorder()
		before := fake.calls
		handleLLMRouterPreview(s)(w, httptest.NewRequest(http.MethodPost, "/api/llm-router/preview", strings.NewReader(tc.body)))
		if w.Code != tc.code || fake.calls-before != tc.calls {
			t.Fatalf("preview %s: code=%d calls=%d body=%s", tc.body, w.Code, fake.calls-before, w.Body.String())
		}
	}
	before := fake.calls
	w := httptest.NewRecorder()
	handleLLMRouterStatus(s)(w, httptest.NewRequest(http.MethodGet, "/api/llm-router/status", nil))
	if fake.calls != before || !strings.Contains(w.Body.String(), `"decisions"`) {
		t.Fatal("status not passive or missing Decisions")
	}
}

// TestRoutingSnapshotGoldenCodes pins every code RoutingSnapshot can return,
// together with the identifying fields and the Current() revocation rule.
func TestRoutingSnapshotGoldenCodes(t *testing.T) {
	type fixture struct {
		s   *Server
		cfg *config.Config
	}
	routerOn := func(s *Server) {
		s.Cfg.LLMRouter = config.DefaultLLMRouterConfig()
		s.Cfg.LLMRouter.Enabled = true
	}
	local := func(t *testing.T) (fixture, *fakeDecisions) {
		s, fake := decisionsServerFixture(t)
		routerOn(s)
		return fixture{s, s.Cfg}, fake
	}
	provider := func(t *testing.T) (fixture, *routedDecisions) {
		s, d, _, _ := routedDecisionFixture(t)
		routerOn(s)
		return fixture{s, s.Cfg}, d
	}
	for _, tc := range []struct {
		name         string
		build        func(t *testing.T) fixture
		code         string
		available    bool
		backend      string
		providerID   string
		model        string
		hasCurrent   bool
		hasEvaluate  bool
		revokeOnFlag bool
	}{
		{"service disabled", func(t *testing.T) fixture {
			f, _ := local(t)
			f.s.Cfg.Decisions.Enabled = false
			return f
		}, "service_disabled", false, "local", "", "", false, false, false},
		{"router disabled", func(t *testing.T) fixture {
			f, _ := local(t)
			f.s.Cfg.LLMRouter.Enabled = false
			return f
		}, "router_disabled", false, "local", "", "", false, false, false},
		{"fallback disabled", func(t *testing.T) fixture {
			f, _ := local(t)
			f.s.Cfg.LLMRouter.DecisionsFallback = false
			return f
		}, "router_disabled", false, "local", "", "", false, false, false},
		{"configuration changed", func(t *testing.T) fixture {
			f, _ := local(t)
			other := *f.s.Cfg
			other.Decisions.Deployment = "docker"
			f.cfg = &other
			return f
		}, "configuration_changed", false, "local", "", "", true, false, false},
		{"provider not configured", func(t *testing.T) fixture {
			f, d := provider(t)
			d.mu.Lock()
			d.generation.status.Configured = false
			d.mu.Unlock()
			return f
		}, "provider_not_configured", false, "provider", "cloud", "microsoft/microsoft-decision-1", true, false, false},
		{"blocked availability", func(t *testing.T) fixture {
			f, d := provider(t)
			d.mu.Lock()
			d.generation.status.Configured = true
			d.generation.status.ModelAvailability = decisions.ModelAvailability{State: "blocked", Code: "model_not_available", CheckedAt: time.Now()}
			d.mu.Unlock()
			return f
		}, "model_not_available", false, "provider", "cloud", "microsoft/microsoft-decision-1", true, false, false},
		{"local not ready", func(t *testing.T) fixture {
			f, fake := local(t)
			fake.status.Ready = false
			return f
		}, "service_not_ready", false, "local", "", "", true, false, false},
		{"local available", func(t *testing.T) fixture {
			f, _ := local(t)
			return f
		}, "available", true, "local", "", "", true, true, true},
		{"provider available", func(t *testing.T) fixture {
			f, _ := provider(t)
			return f
		}, "available", true, "provider", "cloud", "microsoft/microsoft-decision-1", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.build(t)
			snapshot := (taskDecisionService{f.s}).RoutingSnapshot(f.cfg)
			if snapshot.Code != tc.code || snapshot.Available != tc.available || snapshot.Backend != tc.backend || snapshot.Provider != tc.providerID || snapshot.Model != tc.model {
				t.Fatalf("snapshot = code %q available %v backend %q provider %q model %q", snapshot.Code, snapshot.Available, snapshot.Backend, snapshot.Provider, snapshot.Model)
			}
			if (snapshot.Current != nil) != tc.hasCurrent || (snapshot.Evaluate != nil) != tc.hasEvaluate {
				t.Fatalf("closures: current=%v evaluate=%v", snapshot.Current != nil, snapshot.Evaluate != nil)
			}
			if tc.revokeOnFlag {
				if !snapshot.Current() {
					t.Fatal("fresh snapshot is not current")
				}
				next := *f.s.Cfg
				next.LLMRouter.DecisionsFallback = false
				f.s.Cfg = &next
				f.s.cfgSnapshot.Store(&next)
				if snapshot.Current() {
					t.Fatal("snapshot stayed current after the router fallback was switched off")
				}
			}
		})
	}
}
