package server

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
)

type decisionProviderClient interface {
	Models(context.Context) ([]decisions.DecisionModel, error)
	Evaluate(context.Context, string, decisions.Request) (decisions.Response, error)
}

type decisionProviderError struct{ error }

func (e decisionProviderError) Unwrap() error        { return e.error }
func (e decisionProviderError) IsNonRetryable() bool { return true }

type decisionProviderGeneration struct {
	ctx      context.Context
	cancel   context.CancelFunc
	active   sync.WaitGroup
	settings config.DecisionsConfig
	provider config.ProviderEntry
	client   decisionProviderClient
	status   decisions.Status
}

// routedDecisions keeps provider work outside the managed local runtime's lifecycle.
type routedDecisions struct {
	local      decisionsService
	server     *Server
	mu         sync.Mutex
	generation *decisionProviderGeneration
	closed     bool
	busy       chan struct{}
	newClient  func(string, string) (decisionProviderClient, error)
}

func newRoutedDecisions(s *Server, local decisionsService, cfg *config.Config) *routedDecisions {
	d := &routedDecisions{local: local, server: s, busy: make(chan struct{}, 1), newClient: func(base, key string) (decisionProviderClient, error) {
		return decisions.NewOpenRouterClient(base, key)
	}}
	d.generation = d.newGeneration(cfg)
	return d
}

func decisionsLocalPolicy(c config.DecisionsConfig) config.DecisionsConfig {
	if c.Backend == "provider" {
		c.Enabled = false
	}
	return c
}

func (d *routedDecisions) newGeneration(cfg *config.Config) *decisionProviderGeneration {
	ctx, cancel := context.WithCancel(context.Background())
	g := &decisionProviderGeneration{ctx: ctx, cancel: cancel, settings: cfg.Decisions.Normalize()}
	if g.settings.Backend != "provider" {
		return g
	}
	if p := cfg.FindProvider(g.settings.Provider); p != nil {
		g.provider = *p
	}
	g.status = decisions.Status{Enabled: g.settings.Enabled, Backend: "provider", Deployment: "provider", Provider: g.settings.Provider, Model: g.settings.Model, State: "configured"}
	if !g.settings.Enabled {
		g.status.State = "disabled"
		g.status.Code = decisions.CodeServiceDisabled
		return g
	}
	if err := config.ValidateDecisionsProvider(cfg); err != nil {
		g.status.State = "needs_setup"
		g.status.Code = "provider_not_configured"
		return g
	}
	key := g.provider.APIKey
	if key == "" && d.server.Vault != nil {
		key, _ = d.server.Vault.ReadSecret("provider_" + g.provider.ID + "_api_key")
	}
	if key == "" {
		g.status.State = "needs_setup"
		g.status.Code = "provider_key_missing"
		return g
	}
	client, err := d.newClient(g.provider.BaseURL, key)
	if err != nil {
		g.status.State = "needs_setup"
		g.status.Code = decisions.ErrorCode(err)
		return g
	}
	g.client = client
	g.status.Configured = true
	return g
}

func (d *routedDecisions) Status() decisions.Status {
	d.mu.Lock()
	if d.generation.settings.Backend == "provider" {
		status := d.generation.status
		d.mu.Unlock()
		return status
	}
	d.mu.Unlock()
	return d.local.Status()
}

// requestContext pins routing before the server resolves media. A change from
// local to external must not export input admitted under the old policy.
func (d *routedDecisions) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	d.mu.Lock()
	g := d.generation
	closed := d.closed
	d.mu.Unlock()
	ctx, cancel := context.WithCancel(context.WithValue(ctx, d, g))
	stop := context.AfterFunc(g.ctx, cancel)
	if closed || g.ctx.Err() != nil {
		cancel()
	}
	return ctx, func() { stop(); cancel() }
}

func (d *routedDecisions) Evaluate(ctx context.Context, request decisions.Request) (response decisions.Response, err error) {
	d.mu.Lock()
	g := d.generation
	if admitted, ok := ctx.Value(d).(*decisionProviderGeneration); ok && admitted != g {
		d.mu.Unlock()
		return decisions.Response{}, decisionProviderError{decisions.NewError("configuration_changed", context.Canceled)}
	}
	if d.closed {
		d.mu.Unlock()
		return decisions.Response{}, decisions.NewError(decisions.CodeServiceNotReady, nil)
	}
	if g.settings.Backend != "provider" {
		d.mu.Unlock()
		return d.local.Evaluate(ctx, request)
	}
	if !g.status.Configured {
		code := g.status.Code
		d.mu.Unlock()
		return decisions.Response{}, decisions.NewError(code, nil)
	}
	select {
	case d.busy <- struct{}{}:
	default:
		d.mu.Unlock()
		return decisions.Response{}, decisions.NewError(decisions.CodeQueueFull, nil)
	}
	// Admission rejection is safe to retry because it cannot reach the provider.
	// Once admitted, preserve terminal errors to prevent repeating a paid request.
	defer func() {
		if err != nil {
			err = decisionProviderError{err}
		}
	}()
	g.active.Add(1)
	d.mu.Unlock()
	defer func() { <-d.busy; g.active.Done() }()
	if d.server.BudgetTracker != nil && d.server.BudgetTracker.IsBlocked("decisions") {
		return decisions.Response{}, decisions.NewError("budget_exceeded", nil)
	}
	callCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	stop := context.AfterFunc(g.ctx, cancel)
	defer func() { stop(); cancel() }()
	if g.ctx.Err() != nil {
		cancel()
	}
	response, err = g.client.Evaluate(callCtx, g.settings.Model, request)
	// A paid response still counts when policy revocation prevents its publication.
	if response.Usage.CostUSD != nil && d.server.BudgetTracker != nil {
		d.server.BudgetTracker.RecordCostForCategory("decisions", *response.Usage.CostUSD)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != g || d.closed || g.ctx.Err() != nil {
		return decisions.Response{}, decisions.NewError("configuration_changed", context.Canceled)
	}
	if err == nil && callCtx.Err() != nil {
		err = decisions.NewError("external_outcome_unknown", callCtx.Err())
	}
	if err != nil {
		g.status.Ready = false
		g.status.State = "error"
		g.status.Code = decisions.ErrorCode(err)
		g.status.LastError = g.status.Code
		return decisions.Response{}, err
	}
	g.status.Ready = true
	g.status.State = "ready"
	g.status.Code = ""
	g.status.LastError = ""
	g.status.LastSuccess = time.Now().UTC().Format(time.RFC3339)
	if response.Execution != nil {
		response.Execution.ProviderID = g.provider.ID
	}
	return response, nil
}

func (d *routedDecisions) Models(ctx context.Context, providerID string) ([]decisions.DecisionModel, error) {
	// Catalog reads may use a saved connection before Decisions itself is enabled.
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, decisions.NewError(decisions.CodeServiceNotReady, nil)
	}
	g := d.generation
	cfg := d.server.ConfigSnapshot()
	p := cfg.FindProvider(providerID)
	if p == nil || !strings.EqualFold(strings.TrimSpace(p.Type), "openrouter") {
		d.mu.Unlock()
		return nil, decisions.NewError("provider_not_configured", nil)
	}
	key := p.APIKey
	if key == "" && d.server.Vault != nil {
		key, _ = d.server.Vault.ReadSecret("provider_" + p.ID + "_api_key")
	}
	client, err := d.newClient(p.BaseURL, key)
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}
	g.active.Add(1)
	d.mu.Unlock()
	defer g.active.Done()
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	stop := context.AfterFunc(g.ctx, cancel)
	defer func() { stop(); cancel() }()
	if g.ctx.Err() != nil {
		cancel()
	}
	models, err := client.Models(callCtx)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != g || g.ctx.Err() != nil || d.closed {
		return nil, decisions.NewError("configuration_changed", context.Canceled)
	}
	return models, err
}

func waitDecisionGeneration(ctx context.Context, g *decisionProviderGeneration) error {
	done := make(chan struct{})
	go func() { g.active.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *routedDecisions) prepareConfiguration(cfg *config.Config) (func(context.Context) error, error) {
	d.mu.Lock()
	old := d.generation
	old.cancel()
	d.generation = d.newGeneration(cfg)
	if d.closed {
		d.generation.cancel()
	}
	cleanup, err := d.local.PrepareReconfigure(decisionsLocalPolicy(cfg.Decisions))
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error { return errors.Join(waitDecisionGeneration(ctx, old), cleanup(ctx)) }, nil
}

func (d *routedDecisions) PrepareReconfigure(c config.DecisionsConfig) (func(context.Context) error, error) {
	cfg := *d.server.ConfigSnapshot()
	cfg.Decisions = c
	return d.prepareConfiguration(&cfg)
}

func (d *routedDecisions) localAction(ctx context.Context, action func(context.Context) error) error {
	d.mu.Lock()
	provider := d.generation.settings.Backend == "provider"
	closed := d.closed
	d.mu.Unlock()
	if closed {
		return decisions.NewError(decisions.CodeServiceNotReady, nil)
	}
	if provider {
		return decisions.NewError("provider_lifecycle_unavailable", nil)
	}
	return action(ctx)
}
func (d *routedDecisions) AutoStart(ctx context.Context) error {
	d.mu.Lock()
	provider := d.generation.settings.Backend == "provider"
	d.mu.Unlock()
	if provider {
		return nil
	}
	return d.localAction(ctx, d.local.AutoStart)
}
func (d *routedDecisions) Install(ctx context.Context) error {
	return d.localAction(ctx, d.local.Install)
}
func (d *routedDecisions) Start(ctx context.Context) error { return d.localAction(ctx, d.local.Start) }
func (d *routedDecisions) Stop(ctx context.Context) error  { return d.localAction(ctx, d.local.Stop) }
func (d *routedDecisions) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	d.closed = true
	g := d.generation
	g.cancel()
	g.status.Ready = false
	g.status.State = "unavailable"
	g.status.Code = decisions.CodeServiceNotReady
	d.mu.Unlock()
	return errors.Join(waitDecisionGeneration(ctx, g), d.local.Shutdown(ctx))
}
