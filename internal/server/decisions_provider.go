package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/security"
)

type decisionProviderClient interface {
	Models(context.Context) ([]decisions.DecisionModel, error)
	Evaluate(context.Context, string, decisions.Request) (decisions.Response, error)
}

type decisionProviderError struct{ error }

func (e decisionProviderError) Unwrap() error        { return e.error }
func (e decisionProviderError) IsNonRetryable() bool { return true }

type decisionProviderGeneration struct {
	id                    uint64
	ctx                   context.Context
	cancel                context.CancelFunc
	active                sync.WaitGroup
	settings              config.DecisionsConfig
	provider              config.ProviderEntry
	credentialFingerprint [32]byte
	client                decisionProviderClient
	releaseCredential     func()
	status                decisions.Status
	modelEpoch            uint64
	modelIdentity         [32]byte
	modelChecked          bool
}

var decisionGenerationSequence atomic.Uint64

type decisionCatalogGeneration struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	active                sync.WaitGroup
	provider              config.ProviderEntry
	credentialFingerprint [32]byte
	client                decisionProviderClient
	releaseCredential     func()
}

// routedDecisions keeps provider work outside the managed local runtime's lifecycle.
type routedDecisions struct {
	local      decisionsService
	server     *Server
	mu         sync.Mutex
	generation *decisionProviderGeneration
	catalogs   map[string]*decisionCatalogGeneration
	// catalogActive covers requests detached by a config change as well as
	// current catalog generations. Shutdown closes admission before waiting.
	catalogActive  sync.WaitGroup
	closed         bool
	busy           chan struct{}
	newClient      func(string, string) (decisionProviderClient, error)
	newOAuthClient func(string, string) (decisionProviderClient, error)
}

func newRoutedDecisions(s *Server, local decisionsService, cfg *config.Config) *routedDecisions {
	d := &routedDecisions{local: local, server: s, busy: make(chan struct{}, decisionProviderSlots), catalogs: make(map[string]*decisionCatalogGeneration), newClient: func(base, key string) (decisionProviderClient, error) {
		return decisions.NewOpenRouterClient(base, key)
	}, newOAuthClient: func(base, key string) (decisionProviderClient, error) {
		return decisions.NewOpenRouterClientScoped(base, key)
	}}
	d.generation = d.newGeneration(cfg)
	return d
}

// decisionProviderSlots keeps one external slot free for interactive work
// while a background consumer holds the other.
const decisionProviderSlots = 2

// providerIdle is a passive read: true when no external evaluation is admitted.
func (d *routedDecisions) providerIdle() bool { return len(d.busy) == 0 }

func decisionsLocalPolicy(c config.DecisionsConfig) config.DecisionsConfig {
	if c.Backend == "provider" {
		c.Enabled = false
	}
	return c
}

func (d *routedDecisions) newGeneration(cfg *config.Config) *decisionProviderGeneration {
	ctx, cancel := context.WithCancel(context.Background())
	g := &decisionProviderGeneration{id: decisionGenerationSequence.Add(1), ctx: ctx, cancel: cancel, settings: cfg.Decisions.Normalize()}
	if g.settings.Backend != "provider" {
		return g
	}
	if p := cfg.FindProvider(g.settings.Provider); p != nil {
		g.provider = decisionProviderProfile(*p)
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
	provider := cfg.FindProvider(g.settings.Provider)
	if provider == nil {
		g.status.State = "needs_setup"
		g.status.Code = "provider_not_configured"
		return g
	}
	key, fingerprint := decisionProviderCredential(d.server, *provider)
	g.credentialFingerprint = fingerprint
	if key == "" {
		g.status.State = "needs_setup"
		g.status.Code = "provider_key_missing"
		return g
	}
	client, releaseCredential, err := d.newProviderClient(g.provider, key)
	if err != nil {
		g.status.State = "needs_setup"
		g.status.Code = decisions.ErrorCode(err)
		return g
	}
	g.client = client
	g.releaseCredential = releaseCredential
	g.status.Configured = true
	return g
}

// decisionProviderCredential keeps OAuth credentials in the Vault-backed
// credential path. Provider APIKey is intentionally ignored for OAuth profiles
// because ApplyOAuthTokens also hydrates general LLM config snapshots.
func decisionProviderCredential(s *Server, provider config.ProviderEntry) (string, [32]byte) {
	var key string
	var fingerprintSource string
	if normalizeProviderAuthType(provider.AuthType) == "oauth2" {
		if s != nil && s.Vault != nil {
			if raw, err := s.Vault.ReadSecret("oauth_" + provider.ID); err == nil {
				fingerprintSource = raw
				var token config.OAuthToken
				if json.Unmarshal([]byte(raw), &token) == nil {
					key = strings.TrimSpace(token.AccessToken)
				}
			}
		}
	} else {
		key = strings.TrimSpace(provider.APIKey)
		if key == "" && s != nil && s.Vault != nil {
			key, _ = s.Vault.ReadSecret("provider_" + provider.ID + "_api_key")
			key = strings.TrimSpace(key)
		}
		fingerprintSource = key
	}
	return key, sha256.Sum256([]byte(fingerprintSource))
}

func (d *routedDecisions) newProviderClient(provider config.ProviderEntry, key string) (decisionProviderClient, func(), error) {
	if normalizeProviderAuthType(provider.AuthType) == "oauth2" {
		var client decisionProviderClient
		var err error
		if d.newOAuthClient != nil {
			client, err = d.newOAuthClient(provider.BaseURL, key)
		} else if d.newClient != nil {
			client, err = d.newClient(provider.BaseURL, key)
		} else {
			err = errors.New("OAuth Decisions client is not configured")
		}
		if err != nil {
			return nil, nil, err
		}
		if closer, ok := client.(interface{ Close() }); ok {
			return client, onceDecisionCredentialRelease(closer.Close), nil
		}
		return client, onceDecisionCredentialRelease(security.RegisterScopedSensitiveExact(key)), nil
	}
	if d.newClient == nil {
		return nil, nil, errors.New("Decisions provider client is not configured")
	}
	client, err := d.newClient(provider.BaseURL, key)
	return client, nil, err
}

func onceDecisionCredentialRelease(release func()) func() {
	var once sync.Once
	return func() {
		once.Do(release)
	}
}

func releaseDecisionCredentialWhenDrained(active *sync.WaitGroup, release func()) {
	if release == nil {
		return
	}
	go func() {
		active.Wait()
		release()
	}()
}

func decisionProviderProfile(provider config.ProviderEntry) config.ProviderEntry {
	provider.APIKey = ""
	return provider
}

func decisionProviderProfilesEqual(a, b config.ProviderEntry) bool {
	return reflect.DeepEqual(decisionProviderProfile(a), decisionProviderProfile(b))
}

// configurationChanged compares the active route with the effective next
// route, including Vault-only credential rotation.
func (d *routedDecisions) configurationChanged(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	settings := cfg.Decisions.Normalize()
	d.mu.Lock()
	current := d.generation
	if current == nil || current.settings != settings {
		d.mu.Unlock()
		return true
	}
	provider := current.provider
	credentialFingerprint := current.credentialFingerprint
	d.mu.Unlock()
	if settings.Backend != "provider" {
		return false
	}
	if !settings.Enabled {
		return false
	}
	next := cfg.FindProvider(settings.Provider)
	if next == nil || !decisionProviderProfilesEqual(provider, *next) {
		return true
	}
	_, nextFingerprint := decisionProviderCredential(d.server, *next)
	return credentialFingerprint != nextFingerprint
}

func (d *routedDecisions) Status() decisions.Status {
	d.mu.Lock()
	if d.generation.settings.Backend == "provider" {
		status := d.generation.status
		status.ModelAvailability = status.ModelAvailability.Snapshot(time.Now())
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
	category := decisionUsageCategory(ctx)
	if d.server.BudgetTracker != nil && d.server.BudgetTracker.IsBlocked(category) {
		return decisions.Response{}, decisions.NewError("budget_exceeded", nil)
	}
	callCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	stop := context.AfterFunc(g.ctx, cancel)
	defer func() { stop(); cancel() }()
	if g.ctx.Err() != nil {
		cancel()
	}
	models, err := g.client.Models(callCtx)
	d.mu.Lock()
	if d.generation != g || d.closed || g.ctx.Err() != nil {
		d.mu.Unlock()
		return decisions.Response{}, decisions.NewError("configuration_changed", context.Canceled)
	}
	if err == nil {
		d.publishModelCatalog(g, models)
		if g.status.ModelAvailability.State == "blocked" {
			err = decisions.NewError(g.status.ModelAvailability.Code, nil)
		}
	}
	if epoch, ok := ctx.Value(decisionModelEpochKey{}).(uint64); ok && epoch != g.modelEpoch {
		err = decisions.NewError("configuration_changed", context.Canceled)
	}
	if err != nil {
		g.status.Ready, g.status.State = false, "error"
		g.status.Code, g.status.LastError = decisions.ErrorCode(err), decisions.ErrorCode(err)
		d.mu.Unlock()
		return decisions.Response{}, err
	}
	model := g.status.ModelAvailability.Model.ResolvedID()
	modelEpoch := g.modelEpoch
	d.mu.Unlock()
	response, err = g.client.Evaluate(callCtx, model, request)
	response.Model = g.settings.Model
	// A paid response still counts when policy revocation prevents its publication.
	if response.Usage.CostUSD != nil && d.server.BudgetTracker != nil {
		d.server.BudgetTracker.RecordCostForCategory(category, *response.Usage.CostUSD)
	}
	// Consumer telemetry needs reported usage even when a paid result is rejected.
	// Never release answers from a failed or revoked generation.
	failedResponse := decisions.Response{}
	if category != "decisions" {
		failedResponse.Usage = response.Usage
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != g || d.closed || g.ctx.Err() != nil || g.modelEpoch != modelEpoch {
		return failedResponse, decisions.NewError("configuration_changed", context.Canceled)
	}
	if err == nil && callCtx.Err() != nil {
		err = decisions.NewError("external_outcome_unknown", callCtx.Err())
	}
	if err != nil {
		g.status.Ready = false
		g.status.State = "error"
		g.status.Code = decisions.ErrorCode(err)
		g.status.LastError = g.status.Code
		return failedResponse, err
	}
	g.status.Ready = true
	g.status.State = "ready"
	g.status.Code = ""
	g.status.LastError = ""
	g.status.LastSuccess = time.Now().UTC().Format(time.RFC3339)
	g.status.ModelRevision = response.ModelRevision
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
	cfg := d.server.ConfigSnapshot()
	p := cfg.FindProvider(providerID)
	if p == nil || !strings.EqualFold(strings.TrimSpace(p.Type), "openrouter") {
		d.mu.Unlock()
		return nil, decisions.NewError("provider_not_configured", nil)
	}
	profile := decisionProviderProfile(*p)
	key, credentialFingerprint := decisionProviderCredential(d.server, *p)
	if key == "" && normalizeProviderAuthType(p.AuthType) == "oauth2" {
		d.mu.Unlock()
		return nil, decisions.NewError("provider_key_missing", nil)
	}
	active := d.generation
	if active.client != nil && active.provider.ID == p.ID && active.credentialFingerprint == credentialFingerprint && decisionProviderProfilesEqual(active.provider, profile) {
		active.active.Add(1)
		d.mu.Unlock()
		defer active.active.Done()
		callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		stop := context.AfterFunc(active.ctx, cancel)
		defer func() { stop(); cancel() }()
		models, err := active.client.Models(callCtx)
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.generation != active || active.ctx.Err() != nil || d.closed {
			return nil, decisions.NewError("configuration_changed", context.Canceled)
		}
		if err == nil {
			d.publishModelCatalog(active, models)
		}
		return models, err
	}
	if d.catalogs == nil {
		d.catalogs = make(map[string]*decisionCatalogGeneration)
	}
	g := d.catalogs[p.ID]
	if g == nil || !decisionProviderProfilesEqual(g.provider, profile) || g.credentialFingerprint != credentialFingerprint {
		client, releaseCredential, err := d.newProviderClient(*p, key)
		if err != nil {
			d.mu.Unlock()
			return nil, err
		}
		if g != nil {
			g.cancel()
			releaseDecisionCredentialWhenDrained(&g.active, g.releaseCredential)
		}
		genCtx, cancel := context.WithCancel(context.Background())
		g = &decisionCatalogGeneration{ctx: genCtx, cancel: cancel, provider: profile, credentialFingerprint: credentialFingerprint, client: client, releaseCredential: releaseCredential}
		d.catalogs[p.ID] = g
	}
	g.active.Add(1)
	d.catalogActive.Add(1)
	d.mu.Unlock()
	defer func() { g.active.Done(); d.catalogActive.Done() }()
	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	stop := context.AfterFunc(g.ctx, cancel)
	defer func() { stop(); cancel() }()
	if g.ctx.Err() != nil {
		cancel()
	}
	models, err := g.client.Models(callCtx)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.catalogs[p.ID] != g || g.ctx.Err() != nil || d.closed {
		return nil, decisions.NewError("configuration_changed", context.Canceled)
	}
	active = d.generation
	if err == nil && active.settings.Backend == "provider" && active.provider.ID == p.ID && active.credentialFingerprint == credentialFingerprint && decisionProviderProfilesEqual(active.provider, profile) {
		d.publishModelCatalog(active, models)
	}
	return models, err
}

func waitDecisionGeneration(ctx context.Context, g *decisionProviderGeneration) error {
	err := waitDecisionGroup(ctx, &g.active)
	if err == nil && g.releaseCredential != nil {
		g.releaseCredential()
	}
	return err
}

func waitDecisionCatalogGeneration(ctx context.Context, g *decisionCatalogGeneration) error {
	err := waitDecisionGroup(ctx, &g.active)
	if err == nil && g.releaseCredential != nil {
		g.releaseCredential()
	}
	return err
}

// prepareCatalogConfiguration revokes only model lookups tied to a provider
// whose saved profile or effective credential changed.
func (d *routedDecisions) prepareCatalogConfiguration(cfg *config.Config) func(context.Context) error {
	d.mu.Lock()
	var retired []*decisionCatalogGeneration
	for id, current := range d.catalogs {
		provider := cfg.FindProvider(id)
		if provider == nil || !strings.EqualFold(strings.TrimSpace(provider.Type), "openrouter") {
			current.cancel()
			releaseDecisionCredentialWhenDrained(&current.active, current.releaseCredential)
			delete(d.catalogs, id)
			retired = append(retired, current)
			continue
		}
		_, fingerprint := decisionProviderCredential(d.server, *provider)
		if !decisionProviderProfilesEqual(current.provider, decisionProviderProfile(*provider)) || current.credentialFingerprint != fingerprint {
			current.cancel()
			releaseDecisionCredentialWhenDrained(&current.active, current.releaseCredential)
			delete(d.catalogs, id)
			retired = append(retired, current)
		}
	}
	d.mu.Unlock()
	if len(retired) == 0 {
		return nil
	}
	return func(ctx context.Context) error {
		return waitDecisionCatalogs(ctx, retired)
	}
}

// invalidateProvider immediately revokes active route/catalog calls after an
// OAuth token is deleted outside normal config snapshot publication.
func (d *routedDecisions) invalidateProvider(providerID string) func(context.Context) error {
	d.mu.Lock()
	var oldRoute *decisionProviderGeneration
	if d.generation.settings.Backend == "provider" && d.generation.settings.Provider == providerID {
		oldRoute = d.generation
		oldRoute.cancel()
		releaseDecisionCredentialWhenDrained(&oldRoute.active, oldRoute.releaseCredential)
		cfg := d.server.ConfigSnapshot()
		d.generation = d.newGeneration(cfg)
		if d.closed {
			d.generation.cancel()
		}
	}
	var oldCatalog *decisionCatalogGeneration
	if d.catalogs != nil {
		oldCatalog = d.catalogs[providerID]
		if oldCatalog != nil {
			oldCatalog.cancel()
			releaseDecisionCredentialWhenDrained(&oldCatalog.active, oldCatalog.releaseCredential)
			delete(d.catalogs, providerID)
		}
	}
	d.mu.Unlock()
	if oldRoute == nil && oldCatalog == nil {
		return nil
	}
	return func(ctx context.Context) error {
		var errs []error
		if oldRoute != nil {
			errs = append(errs, waitDecisionGeneration(ctx, oldRoute))
		}
		if oldCatalog != nil {
			errs = append(errs, waitDecisionCatalogGeneration(ctx, oldCatalog))
		}
		return errors.Join(errs...)
	}
}

func (d *routedDecisions) prepareConfiguration(cfg *config.Config) (func(context.Context) error, error) {
	d.mu.Lock()
	old := d.generation
	policyChanged := old.settings != cfg.Decisions.Normalize()
	var retiredCatalogs []*decisionCatalogGeneration
	if policyChanged {
		for providerID, catalog := range d.catalogs {
			catalog.cancel()
			releaseDecisionCredentialWhenDrained(&catalog.active, catalog.releaseCredential)
			retiredCatalogs = append(retiredCatalogs, catalog)
			delete(d.catalogs, providerID)
		}
	}
	old.cancel()
	releaseDecisionCredentialWhenDrained(&old.active, old.releaseCredential)
	d.generation = d.newGeneration(cfg)
	if d.closed {
		d.generation.cancel()
	}
	cleanup, err := d.local.PrepareReconfigure(decisionsLocalPolicy(cfg.Decisions))
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) error {
		return errors.Join(waitDecisionGeneration(ctx, old), waitDecisionCatalogs(ctx, retiredCatalogs), cleanup(ctx))
	}, nil
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
	releaseDecisionCredentialWhenDrained(&g.active, g.releaseCredential)
	g.status.Ready = false
	g.status.State = "unavailable"
	g.status.Code = decisions.CodeServiceNotReady
	var catalogs []*decisionCatalogGeneration
	for id, catalog := range d.catalogs {
		catalog.cancel()
		releaseDecisionCredentialWhenDrained(&catalog.active, catalog.releaseCredential)
		catalogs = append(catalogs, catalog)
		delete(d.catalogs, id)
	}
	d.mu.Unlock()
	return errors.Join(waitDecisionGeneration(ctx, g), waitDecisionCatalogs(ctx, catalogs), waitDecisionGroup(ctx, &d.catalogActive), d.local.Shutdown(ctx))
}

func waitDecisionCatalogs(ctx context.Context, generations []*decisionCatalogGeneration) error {
	var errs []error
	for _, generation := range generations {
		errs = append(errs, waitDecisionCatalogGeneration(ctx, generation))
	}
	return errors.Join(errs...)
}

func waitDecisionGroup(ctx context.Context, group *sync.WaitGroup) error {
	done := make(chan struct{})
	go func() { group.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
