package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/flows"
)

type fakeDecisionProvider struct {
	calls        atomic.Int32
	catalogCalls atomic.Int32
	evaluate     func(context.Context) (decisions.Response, error)
	models       func(context.Context) ([]decisions.DecisionModel, error)
}

func (f *fakeDecisionProvider) Evaluate(ctx context.Context, _ string, _ decisions.Request) (decisions.Response, error) {
	f.calls.Add(1)
	if f.evaluate != nil {
		return f.evaluate(ctx)
	}
	cost := .025
	noul := .8
	return decisions.Response{Model: "microsoft/microsoft-decision-1", Calibration: "uncalibrated", Execution: &decisions.ExternalExecution{Provider: "upstream"}, Usage: decisions.Usage{CostUSD: &cost}, Answers: map[string]decisions.Answer{"q": {Type: "noul", Decision: "yes", Confidence: .8, Noul: &noul, Probabilities: map[string]float64{"yes": .8, "no": .2}}}}, nil
}
func (f *fakeDecisionProvider) Models(ctx context.Context) ([]decisions.DecisionModel, error) {
	f.catalogCalls.Add(1)
	if f.models != nil {
		return f.models(ctx)
	}
	return []decisions.DecisionModel{{ID: "microsoft/microsoft-decision-1"}}, nil
}

func routedDecisionFixture(t *testing.T) (*Server, *routedDecisions, *fakeDecisions, *fakeDecisionProvider) {
	t.Helper()
	s, local := decisionsServerFixture(t)
	s.Cfg.Decisions.Backend = "provider"
	s.Cfg.Decisions.Provider = "cloud"
	s.Cfg.Decisions.Model = "microsoft/microsoft-decision-1"
	s.Cfg.Providers = []config.ProviderEntry{{ID: "cloud", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: "test-only"}}
	remote := &fakeDecisionProvider{}
	d := &routedDecisions{server: s, local: local, busy: make(chan struct{}, 1), newClient: func(string, string) (decisionProviderClient, error) { return remote, nil }}
	d.generation = d.newGeneration(s.Cfg)
	s.Decisions = d
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = d.Shutdown(ctx)
	})
	return s, d, local, remote
}

func TestDecisionProviderPassiveStatusAndExplicitTest(t *testing.T) {
	s, d, local, remote := routedDecisionFixture(t)
	if status := d.Status(); !status.Configured || status.Ready || status.State != "configured" || status.Backend != "provider" {
		t.Fatalf("passive status: %+v", status)
	}
	if remote.calls.Load() != 0 || local.calls != 0 {
		t.Fatal("construction made an inference")
	}
	for _, action := range []func(context.Context) error{d.Install, d.Start, d.Stop} {
		if decisions.ErrorCode(action(context.Background())) != "provider_lifecycle_unavailable" {
			t.Fatal("provider acquired local lifecycle")
		}
	}
	if err := d.AutoStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/api/decisions/models?provider=cloud", nil))
	if w.Code != 200 || remote.calls.Load() != 0 || remote.catalogCalls.Load() != 1 {
		t.Fatalf("catalog %d %s", w.Code, w.Body.String())
	}
	if _, err := d.Models(context.Background(), "https://attacker.invalid"); decisions.ErrorCode(err) != "provider_not_configured" || remote.catalogCalls.Load() != 1 {
		t.Fatalf("catalog accepted an unsaved target: %v", err)
	}
	r := httptest.NewRequest("POST", "http://localhost/api/decisions/test", nil)
	r.Header.Set("Origin", "http://localhost")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != 200 || remote.calls.Load() != 1 || local.calls != 0 {
		t.Fatalf("test %d %s", w.Code, w.Body.String())
	}
	if status := d.Status(); !status.Ready || status.LastSuccess == "" {
		t.Fatalf("tested status: %+v", status)
	}
	if !strings.Contains(w.Body.String(), `"provider_id":"cloud"`) {
		t.Fatal("configured provenance missing")
	}
}

func TestDecisionProviderCatalogRequiresAdminScope(t *testing.T) {
	s, admin, desktop, read := newBearerSchemeTestServer(t)
	s.Decisions = &fakeDecisions{}
	mux := http.NewServeMux()
	registerDecisionsRoutes(mux, s)
	for _, tc := range []struct {
		token string
		want  int
	}{{admin, http.StatusConflict}, {desktop, http.StatusForbidden}, {read, http.StatusForbidden}} {
		r := httptest.NewRequest("GET", "/api/decisions/models?provider=cloud", nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		authMiddleware(s, mux).ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("catalog scope: %d want %d", w.Code, tc.want)
		}
	}
}

func TestDecisionProviderBudgetAndNoFallback(t *testing.T) {
	s, d, local, remote := routedDecisionFixture(t)
	s.Cfg.Budget.Enabled = true
	s.Cfg.Budget.DailyLimitUSD = .02
	s.Cfg.Budget.Enforcement = "full"
	s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	t.Cleanup(s.BudgetTracker.Flush)
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatal(err)
	}
	if spent := s.BudgetTracker.GetStatus().SpentUSD; spent != .025 {
		t.Fatalf("cost=%g", spent)
	}
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != "budget_exceeded" || !decisions.IsNonRetryable(err) {
		t.Fatalf("budget=%v", err)
	}
	if remote.calls.Load() != 1 || local.calls != 0 {
		t.Fatal("budgeted evaluation escaped routing")
	}
	s.BudgetTracker = nil
	remote.evaluate = func(context.Context) (decisions.Response, error) {
		return decisions.Response{}, decisions.NewError("external_outcome_unknown", context.DeadlineExceeded)
	}
	_, err := flowDecisions{s}.Evaluate(context.Background(), flows.DecisionRequest{State: "test", Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Ready?"}}})
	var ne *flows.NodeError
	if !errors.As(err, &ne) || ne.Code != "FLOW_AI_UNAVAILABLE" || !strings.Contains(ne.Message, "external_outcome_unknown") || local.calls != 0 {
		t.Fatalf("flow retry/fallback error: %v", err)
	}
}

func TestDecisionProviderRevokesActiveEvaluationAndCatalog(t *testing.T) {
	for _, catalog := range []bool{false, true} {
		t.Run(map[bool]string{false: "evaluation", true: "catalog"}[catalog], func(t *testing.T) {
			s, d, _, remote := routedDecisionFixture(t)
			started := make(chan struct{})
			cancelled := make(chan struct{})
			release := make(chan struct{})
			finished := make(chan error, 1)
			wait := func(ctx context.Context) { close(started); <-ctx.Done(); close(cancelled); <-release }
			if catalog {
				remote.models = func(ctx context.Context) ([]decisions.DecisionModel, error) {
					wait(ctx)
					return []decisions.DecisionModel{{ID: "late"}}, nil
				}
			} else {
				remote.evaluate = func(ctx context.Context) (decisions.Response, error) {
					wait(ctx)
					return decisions.Response{Model: "late"}, nil
				}
			}
			go func() {
				var err error
				if catalog {
					_, err = d.Models(context.Background(), "cloud")
				} else {
					_, err = d.Evaluate(context.Background(), decisions.Request{})
				}
				finished <- err
			}()
			<-started
			if !catalog {
				if _, err := d.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != decisions.CodeQueueFull || decisions.IsNonRetryable(err) {
					t.Fatalf("parallel call: %v", err)
				}
				_, err := (flowDecisions{s: s}).Evaluate(context.Background(), flows.DecisionRequest{State: "queue check", Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Is capacity free?"}}})
				var nodeError *flows.NodeError
				if !errors.As(err, &nodeError) || nodeError.Code != "FLOW_NODE_FAILED" || remote.calls.Load() != 1 {
					t.Fatalf("admission rejection must remain retryable without an upstream call: %v", err)
				}
			}
			next := *s.Cfg
			next.Decisions.Enabled = false
			cleanup, err := d.prepareConfiguration(&next)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-cancelled:
			case <-time.After(time.Second):
				t.Fatal("request not cancelled")
			}
			close(release)
			if err = <-finished; decisions.ErrorCode(err) != "configuration_changed" {
				t.Fatalf("late response: %v", err)
			}
			if err = cleanup(context.Background()); err != nil {
				t.Fatal(err)
			}
			if status := d.Status(); status.Ready || status.State != "disabled" {
				t.Fatalf("stale status: %+v", status)
			}
		})
	}
}

func TestDecisionProviderKeyAndShutdown(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	next := *s.Cfg
	next.Providers = append([]config.ProviderEntry(nil), next.Providers...)
	next.Providers[0].APIKey = ""
	cleanup, err := d.prepareConfiguration(&next)
	if err != nil {
		t.Fatal(err)
	}
	if err = cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != "provider_key_missing" || remote.calls.Load() != 0 {
		t.Fatalf("missing key: %v", err)
	}
	if err = d.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Models(context.Background(), "cloud"); decisions.ErrorCode(err) != decisions.CodeServiceNotReady {
		t.Fatalf("shutdown catalog: %v", err)
	}
}

func TestDecisionProviderDoesNotReroutePreviouslyAdmittedInput(t *testing.T) {
	s, d, local, remote := routedDecisionFixture(t)
	before := *s.Cfg
	before.Decisions.Backend = "local"
	cleanup, err := d.prepareConfiguration(&before)
	if err != nil {
		t.Fatal(err)
	}
	_ = cleanup(context.Background())
	ctx, cancel := d.requestContext(context.Background())
	defer cancel()
	cleanup, err = d.prepareConfiguration(s.Cfg)
	if err != nil {
		t.Fatal(err)
	}
	_ = cleanup(context.Background())
	if _, err = d.Evaluate(ctx, decisions.Request{}); decisions.ErrorCode(err) != "configuration_changed" {
		t.Fatalf("admitted input: %v", err)
	}
	if remote.calls.Load() != 0 || local.calls != 0 {
		t.Fatal("input changed destination")
	}
}

func TestDecisionProviderCallerCancellationRemainsCancellation(t *testing.T) {
	s, _, _, remote := routedDecisionFixture(t)
	remote.evaluate = func(ctx context.Context) (decisions.Response, error) {
		return decisions.Response{}, decisions.NewError(decisions.CodeCancelled, ctx.Err())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := flowDecisions{s}.Evaluate(ctx, flows.DecisionRequest{State: "test", Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Ready?"}}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("caller cancellation: %v", err)
	}
	if got := decisionsFailureCode(decisionProviderError{decisions.NewError(decisions.CodeCancelled, context.Canceled)}); got != "request_cancelled" {
		t.Fatalf("HTTP cancellation: %s", got)
	}
	if got := decisionsFailureCode(decisionProviderError{decisions.NewError(decisions.CodeExternalOutcomeUnknown, context.DeadlineExceeded)}); got != decisions.CodeExternalOutcomeUnknown {
		t.Fatalf("lost unknown billed outcome: %s", got)
	}
}
