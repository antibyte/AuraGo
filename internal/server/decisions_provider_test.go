package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/flows"
	"aurago/internal/security"
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
	s, d, _, remote := routedDecisionFixture(t)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	remote.evaluate = func(ctx context.Context) (decisions.Response, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		return decisions.Response{Model: "late"}, nil
	}
	go func() {
		_, err := d.Evaluate(context.Background(), decisions.Request{})
		finished <- err
	}()
	<-started
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != decisions.CodeQueueFull || decisions.IsNonRetryable(err) {
		t.Fatalf("parallel call: %v", err)
	}
	_, err := (flowDecisions{s: s}).Evaluate(context.Background(), flows.DecisionRequest{State: "queue check", Questions: []decisions.Question{{ID: "q", Type: "noul", Instructions: "Is capacity free?"}}})
	var nodeError *flows.NodeError
	if !errors.As(err, &nodeError) || nodeError.Code != "FLOW_NODE_FAILED" || remote.calls.Load() != 1 {
		t.Fatalf("admission rejection must remain retryable without an upstream call: %v", err)
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
}

func TestDecisionProviderDisableCancelsActiveCatalog(t *testing.T) {
	_, d, _, remote := routedDecisionFixture(t)
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	remote.models = func(ctx context.Context) ([]decisions.DecisionModel, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		return []decisions.DecisionModel{{ID: "late"}}, nil
	}
	go func() { _, err := d.Models(context.Background(), "cloud"); finished <- err }()
	<-started
	next := *d.server.ConfigSnapshot()
	next.Decisions.Enabled = false
	cleanup, err := d.prepareConfiguration(&next)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("disabling Decisions did not cancel its active model catalog request")
	}
	close(release)
	if err := <-finished; decisions.ErrorCode(err) != "configuration_changed" {
		t.Fatalf("disabled catalog result code: %s", decisions.ErrorCode(err))
	}
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
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

func TestDecisionProviderPublicCatalogWithoutAPIKey(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	next := *s.Cfg
	next.Providers = append([]config.ProviderEntry(nil), next.Providers...)
	next.Providers[0].APIKey = ""
	cleanup, err := d.prepareConfiguration(&next)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Cfg = &next
	s.cfgSnapshot.Store(&next)
	usedEmptyKey := false
	d.newClient = func(_ string, key string) (decisionProviderClient, error) {
		usedEmptyKey = key == ""
		return remote, nil
	}
	if _, err := d.Models(context.Background(), "cloud"); err != nil {
		t.Fatalf("public catalog unavailable without an API key: %v", err)
	}
	if !usedEmptyKey || remote.catalogCalls.Load() != 1 {
		t.Fatal("public catalog did not use the unauthenticated client")
	}
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); decisions.ErrorCode(err) != "provider_key_missing" || remote.calls.Load() != 0 {
		t.Fatalf("evaluation without credentials must remain blocked: %v", err)
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

func putDecisionOAuthToken(t *testing.T, s *Server, providerID, access, refresh string) {
	t.Helper()
	if s.Vault == nil {
		vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(t.TempDir(), "vault.bin"))
		if err != nil {
			t.Fatal(err)
		}
		s.Vault = vault
	}
	raw, err := json.Marshal(config.OAuthToken{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Vault.WriteSecret("oauth_"+providerID, string(raw)); err != nil {
		t.Fatal(err)
	}
}

func configureDecisionOAuth(t *testing.T, s *Server, d *routedDecisions, access string) {
	t.Helper()
	provider := s.Cfg.FindProvider("cloud")
	if provider == nil {
		t.Fatal("cloud provider missing")
	}
	provider.AuthType = "oauth2"
	provider.APIKey = "stale-hydrated-config-token-must-not-be-used"
	putDecisionOAuthToken(t, s, "cloud", access, "refresh-token-cloud-1")
}

func TestDecisionProviderOAuthCredentialsResolveFromVaultForEvaluateAndModels(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	access := "oauth-access-token-decisions-evaluate-and-models-20261013"
	configureDecisionOAuth(t, s, d, access)
	var keys []string
	d.newOAuthClient = nil // exercise an injected client while retaining the scoped scrubber lease
	d.newClient = func(_, key string) (decisionProviderClient, error) {
		keys = append(keys, key)
		return remote, nil
	}
	d.generation.cancel()
	d.generation = d.newGeneration(s.Cfg)
	if status := d.Status(); !status.Configured || status.Code != "" {
		t.Fatalf("OAuth route was not configured from Vault: %+v", status)
	}
	if _, err := d.Models(context.Background(), "cloud"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0] != access || keys[1] != access {
		t.Fatalf("OAuth credential was not applied to both route and catalog clients (clients=%d)", len(keys))
	}
	if got := security.Scrub(access); got == access {
		t.Fatal("active OAuth credential was not scrubbed")
	}
	if err := d.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := security.Scrub(access); got != access {
		t.Fatal("OAuth credential remained in the process-wide scrubber after shutdown")
	}
}

func TestDecisionProviderOAuthModelsWorkWhileLocalDecisionsAreDisabled(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	access := "oauth-access-token-model-catalog-while-local-disabled-20261013"
	configureDecisionOAuth(t, s, d, access)
	next := *s.Cfg
	next.Decisions.Backend = "local"
	next.Decisions.Enabled = false
	cleanup, err := d.prepareConfiguration(&next)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.Cfg = &next
	s.cfgSnapshot.Store(&next)
	var usedKey string
	d.newClient = func(_, key string) (decisionProviderClient, error) {
		usedKey = key
		return remote, nil
	}
	if _, err := d.Models(context.Background(), "cloud"); err != nil {
		t.Fatal(err)
	}
	if usedKey != access || remote.catalogCalls.Load() != 1 || remote.calls.Load() != 0 {
		t.Fatalf("local/disabled catalog did not use its configured OAuth credential or made an inference (catalog calls=%d evals=%d)", remote.catalogCalls.Load(), remote.calls.Load())
	}
}

func TestDecisionProviderOAuthRotationCancelsOnlyMatchingProviderWork(t *testing.T) {
	s, d, _, first := routedDecisionFixture(t)
	accessV1 := "oauth-access-token-v1-decisions-rotation-20261013"
	accessV2 := "oauth-access-token-v2-decisions-rotation-20261013"
	configureDecisionOAuth(t, s, d, accessV1)
	other := config.ProviderEntry{ID: "other", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: "other-static-key"}
	s.Cfg.Providers = append(s.Cfg.Providers, other)
	otherClient := &fakeDecisionProvider{}
	second := &fakeDecisionProvider{}
	var keyMu sync.Mutex
	var observedKeys []string
	d.newOAuthClient = nil
	d.newClient = func(_, key string) (decisionProviderClient, error) {
		keyMu.Lock()
		observedKeys = append(observedKeys, key)
		keyMu.Unlock()
		switch key {
		case accessV1:
			return first, nil
		case accessV2:
			return second, nil
		case other.APIKey:
			return otherClient, nil
		default:
			return nil, errors.New("unexpected test credential")
		}
	}
	d.generation.cancel()
	d.generation = d.newGeneration(s.Cfg)

	firstEvalStarted, firstEvalCanceled := make(chan struct{}), make(chan struct{})
	firstModelsStarted, firstModelsCanceled := make(chan struct{}), make(chan struct{})
	otherStarted, otherCanceled := make(chan struct{}), make(chan struct{})
	releaseFirst, releaseOther := make(chan struct{}), make(chan struct{})
	var releaseFirstOnce, releaseOtherOnce sync.Once
	defer func() {
		releaseFirstOnce.Do(func() { close(releaseFirst) })
		releaseOtherOnce.Do(func() { close(releaseOther) })
	}()
	first.evaluate = func(ctx context.Context) (decisions.Response, error) {
		close(firstEvalStarted)
		<-ctx.Done()
		close(firstEvalCanceled)
		<-releaseFirst
		return decisions.Response{}, nil
	}
	first.models = func(ctx context.Context) ([]decisions.DecisionModel, error) {
		close(firstModelsStarted)
		<-ctx.Done()
		close(firstModelsCanceled)
		<-releaseFirst
		return []decisions.DecisionModel{{ID: "late-cloud"}}, nil
	}
	otherClient.models = func(ctx context.Context) ([]decisions.DecisionModel, error) {
		close(otherStarted)
		select {
		case <-ctx.Done():
			close(otherCanceled)
			return nil, ctx.Err()
		case <-releaseOther:
			return []decisions.DecisionModel{{ID: "other-model"}}, nil
		}
	}
	finishedEval, finishedCloudModels, finishedOtherModels := make(chan error, 1), make(chan error, 1), make(chan error, 1)
	go func() { _, err := d.Evaluate(context.Background(), decisions.Request{}); finishedEval <- err }()
	go func() { _, err := d.Models(context.Background(), "cloud"); finishedCloudModels <- err }()
	go func() { _, err := d.Models(context.Background(), "other"); finishedOtherModels <- err }()
	<-firstEvalStarted
	<-firstModelsStarted
	<-otherStarted
	if err := s.Vault.WriteSecret("oauth_cloud", `{"access_token":"`+accessV2+`","refresh_token":"refresh-token-cloud-2","token_type":"Bearer"}`); err != nil {
		t.Fatal(err)
	}
	previous := s.ConfigSnapshot()
	next := *previous
	s.reconfigureDecisions(previous, &next)
	for _, cancelled := range []<-chan struct{}{firstEvalCanceled, firstModelsCanceled} {
		select {
		case <-cancelled:
		case <-time.After(time.Second):
			t.Fatal("rotated OAuth credential did not cancel its active call")
		}
	}
	select {
	case <-otherCanceled:
		t.Fatal("rotating cloud credentials cancelled an unrelated provider catalog call")
	case <-time.After(40 * time.Millisecond):
	}
	releaseOtherOnce.Do(func() { close(releaseOther) })
	if err := <-finishedOtherModels; err != nil {
		t.Fatalf("unrelated provider catalog did not complete: %v", err)
	}
	releaseFirstOnce.Do(func() { close(releaseFirst) })
	if err := <-finishedEval; decisions.ErrorCode(err) != "configuration_changed" {
		t.Fatalf("rotated evaluation result: %v", err)
	}
	if err := <-finishedCloudModels; decisions.ErrorCode(err) != "configuration_changed" {
		t.Fatalf("rotated catalog result: %v", err)
	}
	if _, err := d.Evaluate(context.Background(), decisions.Request{}); err != nil {
		t.Fatalf("evaluation with rotated token: %v", err)
	}
	keyMu.Lock()
	defer keyMu.Unlock()
	foundV2 := false
	for _, key := range observedKeys {
		if key == accessV2 {
			foundV2 = true
		}
	}
	if !foundV2 {
		t.Fatalf("new provider generation did not receive the rotated token (clients=%d)", len(observedKeys))
	}
}

func TestDecisionProviderOAuthRevokeInvalidatesRoute(t *testing.T) {
	s, d, _, remote := routedDecisionFixture(t)
	access := "oauth-access-token-for-explicit-revoke-20261013"
	configureDecisionOAuth(t, s, d, access)
	d.newOAuthClient = nil
	d.newClient = func(_, _ string) (decisionProviderClient, error) { return remote, nil }
	d.generation.cancel()
	d.generation = d.newGeneration(s.Cfg)
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	remote.evaluate = func(ctx context.Context) (decisions.Response, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		return decisions.Response{}, nil
	}
	go func() { _, err := d.Evaluate(context.Background(), decisions.Request{}); finished <- err }()
	<-started
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/oauth/revoke?provider=cloud", nil)
	handleOAuthRevoke(s).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke response %d %s", w.Code, w.Body.String())
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("revocation did not cancel the active provider request")
	}
	close(release)
	if err := <-finished; decisions.ErrorCode(err) != "configuration_changed" {
		t.Fatalf("revoked request result: %v", err)
	}
	if status := d.Status(); status.Code != "provider_key_missing" || status.Configured {
		t.Fatalf("revoked provider remained usable: %+v", status)
	}
	if _, err := d.Models(context.Background(), "cloud"); decisions.ErrorCode(err) != "provider_key_missing" {
		t.Fatalf("revoked catalog credential: %v", err)
	}
}

func TestOAuthRevokeReportsVaultDeleteFailure(t *testing.T) {
	s, d, _, _ := routedDecisionFixture(t)
	provider := s.Cfg.FindProvider("cloud")
	provider.AuthType = "oauth2"
	access := "oauth-access-token-revoke-delete-failure-20261013"
	vaultPath := filepath.Join(t.TempDir(), "vault.bin")
	vault, err := security.NewVault(strings.Repeat("b", 64), vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	s.Vault = vault
	putDecisionOAuthToken(t, s, "cloud", access, "refresh-token-cloud")
	d.newOAuthClient = nil
	d.newClient = func(_, _ string) (decisionProviderClient, error) { return &fakeDecisionProvider{}, nil }
	d.generation.cancel()
	d.generation = d.newGeneration(s.Cfg)
	if !d.Status().Configured {
		t.Fatal("test OAuth provider was not configured")
	}
	if err := os.WriteFile(vaultPath, []byte("corrupted vault data"), 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, "/api/oauth/revoke?provider=cloud", nil)
	handleOAuthRevoke(s).ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("revoke reported success after vault deletion failed: status=%d", w.Code)
	}
	if !d.Status().Configured {
		t.Fatal("failed Vault deletion still invalidated the active Decisions credential")
	}
}

type trackingLocalDecisions struct {
	fakeDecisions
	prepareCalls atomic.Int32
	autoStarts   atomic.Int32
}

func (d *trackingLocalDecisions) PrepareReconfigure(config.DecisionsConfig) (func(context.Context) error, error) {
	d.prepareCalls.Add(1)
	return func(context.Context) error { return nil }, nil
}

func (d *trackingLocalDecisions) AutoStart(context.Context) error {
	d.autoStarts.Add(1)
	return nil
}

func TestUnrelatedProviderEditDoesNotReconfigureOrAutostartLocalDecisions(t *testing.T) {
	s, _ := decisionsServerFixture(t)
	s.Cfg.Providers = []config.ProviderEntry{{ID: "one", Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: "static-provider-key"}}
	s.cfgSnapshot.Store(s.Cfg)
	local := &trackingLocalDecisions{fakeDecisions: fakeDecisions{status: decisions.Status{Enabled: true, Installed: true, Ready: false, State: "stopped"}}}
	d := &routedDecisions{server: s, local: local, busy: make(chan struct{}, 1), newClient: func(string, string) (decisionProviderClient, error) {
		return &fakeDecisionProvider{}, nil
	}}
	d.generation = d.newGeneration(s.Cfg)
	s.Decisions = d
	previous := s.ConfigSnapshot()
	next := *previous
	next.Providers = append([]config.ProviderEntry(nil), previous.Providers...)
	next.Providers[0].Name = "renamed provider profile"
	s.reconfigureDecisions(previous, &next)
	if local.prepareCalls.Load() != 0 || local.autoStarts.Load() != 0 {
		t.Fatalf("unrelated provider edit reconfigured local lifecycle: prepare=%d autostart=%d", local.prepareCalls.Load(), local.autoStarts.Load())
	}
}
