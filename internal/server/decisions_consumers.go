package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/decisions"
	"aurago/internal/decisions/consumers"
)

// decisionHub serves every Decisions consumer from one place. Snapshots are
// passive: they only read Status() and never install, start or probe anything.
type decisionHub struct {
	s *Server

	mu           sync.Mutex
	stats        map[consumers.ID]*consumers.Stats
	imagesPinned map[uint64]bool // modelEpoch -> unsupported after images_unsupported
	epochs       map[consumers.ID]*atomic.Uint64

	// Admission state (all under mu). One global background slot; interactive
	// work is counted so background consumers can step aside.
	now                func() time.Time // test seam; nil means time.Now
	interactive        int
	background         *backgroundCall
	lastBackground     map[consumers.ID]time.Time
	lastCatalogRefresh time.Time
}

// backgroundCall is the hub-owned handle of the single admitted background call.
type backgroundCall struct {
	cancel    context.CancelFunc
	external  bool
	preempted bool // guarded by decisionHub.mu
}

// decisionCatalogTTL mirrors the model catalog cache lifetime.
const decisionCatalogTTL = 5 * time.Minute

func (h *decisionHub) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

// localDecisionLoad reads the local manager's load without side effects. Calls
// that bypass the hub (EasyDrag, API) show up here.
func (h *decisionHub) localDecisionLoad() (active, waiting int, ok bool) {
	var local decisionsService = h.s.Decisions
	if router, isRouter := local.(*routedDecisions); isRouter {
		local = router.local
	}
	if loader, has := local.(interface{ Load() (int, int) }); has {
		active, waiting = loader.Load()
		return active, waiting, true
	}
	return 0, 0, false
}

func (h *decisionHub) providerIdle() bool {
	if router, ok := h.s.Decisions.(*routedDecisions); ok {
		return router.providerIdle()
	}
	return true
}

// admit applies the scheduling rules. Interactive work always passes and
// cancels a LOCAL background call; paid external calls are never cut. Background
// work is try-only: any non-empty code means "deferred", nothing was started.
func (h *decisionHub) admit(ctx context.Context, spec consumers.Spec, external bool) (context.Context, func(), *backgroundCall, string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if spec.Class == consumers.Interactive {
		h.interactive++
		if h.background != nil && !h.background.external {
			h.background.preempted = true
			h.background.cancel()
		}
		return ctx, func() { h.mu.Lock(); h.interactive--; h.mu.Unlock() }, nil, ""
	}
	if h.interactive > 0 || h.background != nil {
		return nil, nil, nil, consumers.CodeAdmissionDeferred
	}
	now := h.clock()
	if last, ok := h.lastBackground[spec.ID]; ok && spec.MinInterval > 0 && now.Sub(last) < spec.MinInterval {
		return nil, nil, nil, consumers.CodeAdmissionDeferred
	}
	if external {
		if !h.providerIdle() {
			return nil, nil, nil, consumers.CodeAdmissionDeferred
		}
	} else if active, waiting, ok := h.localDecisionLoad(); ok && (active > 0 || waiting > 0) {
		return nil, nil, nil, consumers.CodeAdmissionDeferred
	}
	ctx, cancel := context.WithCancel(ctx)
	call := &backgroundCall{cancel: cancel, external: external}
	h.background = call
	h.lastBackground[spec.ID] = now
	return ctx, func() {
		cancel()
		h.mu.Lock()
		if h.background == call {
			h.background = nil
		}
		h.mu.Unlock()
	}, call, ""
}

func (h *decisionHub) wasPreempted(call *backgroundCall) bool {
	if call == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return call.preempted
}

func (s *Server) decisionHub() *decisionHub {
	s.decisionHubOnce.Do(func() {
		s.decisionHubInst = &decisionHub{
			s:              s,
			stats:          make(map[consumers.ID]*consumers.Stats),
			imagesPinned:   make(map[uint64]bool),
			epochs:         make(map[consumers.ID]*atomic.Uint64),
			lastBackground: make(map[consumers.ID]time.Time),
		}
	})
	return s.decisionHubInst
}

// epoch returns the consumer's own revocation counter. The router keeps using
// the server-wide routing epoch.
func (h *decisionHub) epoch(id consumers.ID) *atomic.Uint64 {
	if id == consumers.Router {
		return &h.s.routingDecisionEpoch
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	counter := h.epochs[id]
	if counter == nil {
		counter = new(atomic.Uint64)
		h.epochs[id] = counter
	}
	return counter
}

// configChanged revokes only the consumers whose own switch changed.
func (h *decisionHub) configChanged(previous, next *config.Config) {
	for id := range consumers.Registry {
		if id == consumers.Router {
			continue
		}
		if previous.DecisionConsumers.Switch(string(id)) != next.DecisionConsumers.Switch(string(id)) {
			h.epoch(id).Add(1)
		}
	}
}

// Stats returns detached copies.
func (h *decisionHub) Stats() map[consumers.ID]consumers.Stats {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[consumers.ID]consumers.Stats, len(h.stats))
	for id, st := range h.stats {
		out[id] = *st
	}
	return out
}

func (h *decisionHub) record(id consumers.ID, update func(*consumers.Stats)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	st := h.stats[id]
	if st == nil {
		st = new(consumers.Stats)
		h.stats[id] = st
	}
	update(st)
}

func (h *decisionHub) imagesAreUnsupported(modelEpoch uint64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.imagesPinned[modelEpoch]
}

// consumerPolicy is the part of the configuration a consumer's identity
// covers besides the Decisions settings. For the router it is the two router
// switches; for everything else it is the consumer's own entry.
type consumerPolicy struct {
	RouterEnabled, RouterFallback bool
	Switch                        config.DecisionConsumerSwitch
}

func policyFor(cfg *config.Config, id consumers.ID) consumerPolicy {
	if id == consumers.Router {
		return consumerPolicy{RouterEnabled: cfg.LLMRouter.Enabled, RouterFallback: cfg.LLMRouter.DecisionsFallback}
	}
	return consumerPolicy{Switch: cfg.DecisionConsumers.Switch(string(id))}
}

func (p consumerPolicy) on(id consumers.ID) bool {
	if id == consumers.Router {
		return p.RouterEnabled && p.RouterFallback
	}
	return p.Switch.Enabled
}

func (h *decisionHub) Snapshot(cfg *config.Config, id consumers.ID) consumers.Snapshot {
	result := consumers.Snapshot{Consumer: id, Code: "service_not_ready"}
	s := h.s
	spec, known := consumers.Registry[id]
	if !known {
		result.Code = consumers.CodeConsumerDisabled
		return result
	}
	if s == nil || cfg == nil || s.Decisions == nil {
		return result
	}
	settings := cfg.Decisions.Normalize()
	result.Backend, result.Provider, result.Model = settings.Backend, settings.Provider, settings.Model
	if !settings.Enabled {
		result.Code = "service_disabled"
		return result
	}
	policy := policyFor(cfg, id)
	if !policy.on(id) {
		if id == consumers.Router {
			result.Code = "router_disabled"
		} else {
			result.Code = consumers.CodeConsumerDisabled
		}
		return result
	}
	status := s.Decisions.Status()
	if status.Model != "" {
		result.Model = status.Model
	}
	var generation *decisionProviderGeneration
	var generationID uint64
	var modelEpoch uint64
	var modelIdentity [32]byte
	local := s.Decisions
	if router, ok := s.Decisions.(*routedDecisions); ok {
		router.mu.Lock()
		generation = router.generation
		generationID = generation.id
		modelEpoch, modelIdentity = generation.modelEpoch, generation.modelIdentity
		closed := router.closed || generation.ctx.Err() != nil
		router.mu.Unlock()
		if closed {
			return result
		}
		local = router.local
	}
	localGeneration := func() uint64 {
		if settings.Backend != "provider" {
			if manager, ok := local.(interface{ CacheGeneration() uint64 }); ok {
				return manager.CacheGeneration()
			}
		}
		return 0
	}
	version := localGeneration()
	counter := h.epoch(id)
	epoch := counter.Load()
	identity, _ := json.Marshal([]any{id, settings, policy, generationID, modelIdentity, status.ModelAvailability.State, version, epoch, status.Enabled, status.Installed, status.Ready && settings.Backend != "provider"})
	result.Identity = sha256.Sum256(identity)
	result.Current = func() bool {
		current := s.ConfigSnapshot()
		if current == nil || counter.Load() != epoch || current.Decisions.Normalize() != settings || policyFor(current, id) != policy || !policy.on(id) {
			return false
		}
		if router, ok := s.Decisions.(*routedDecisions); ok {
			router.mu.Lock()
			valid := !router.closed && router.generation == generation && generation.settings == settings && generation.ctx.Err() == nil && generation.modelEpoch == modelEpoch
			router.mu.Unlock()
			if !valid {
				return false
			}
		}
		return localGeneration() == version
	}
	if !result.Current() {
		result.Code = consumers.CodeConfigurationChanged
		return result
	}
	switch {
	case !status.Enabled:
		result.Code = "service_disabled"
	case settings.Backend == "provider" && !status.Configured:
		result.Code = "provider_not_configured"
	case settings.Backend == "provider" && status.ModelAvailability.State == "blocked":
		result.Code = status.ModelAvailability.Code
	case settings.Backend != "provider" && !status.Ready:
		result.Code = "service_not_ready"
	default:
		result.Available, result.Code = true, consumers.CodeAvailable
	}
	if !result.Available {
		return result
	}
	result.Caps = h.capabilities(settings, status, modelEpoch)
	if spec.Modality == consumers.ImageRequired {
		switch {
		case result.Caps.Images == consumers.ImagesUnsupported:
			result.Available, result.Code = false, decisions.CodeImagesUnsupported
		case result.Caps.Images != consumers.ImagesSupported:
			result.Available, result.Code = false, consumers.CodeImagesUnknown
		case result.Caps.External && !policy.Switch.AllowExternalImages:
			result.Available, result.Code = false, consumers.CodeImagesExternalNotPermitted
		default:
			result.ImagesAllowed = true
		}
	}
	// The agent checks the router's budget itself, so only other consumers do.
	if result.Available && id != consumers.Router && s.BudgetTracker != nil && s.BudgetTracker.IsBlocked(spec.BudgetCategory) {
		result.Available, result.Code = false, consumers.CodeBudgetBlocked
	}
	if !result.Available {
		return result
	}
	external := result.Caps.External
	result.Evaluate = func(ctx context.Context, request decisions.Request) (decisions.Response, error) {
		if !result.Current() {
			return decisions.Response{}, decisions.NewError(consumers.CodeConfigurationChanged, context.Canceled)
		}
		if spec.Modality == consumers.TextOnly && len(request.Images) > 0 {
			request.Images = nil
			h.record(id, func(st *consumers.Stats) { st.ImagesSkipped++ })
		}
		admitted, release, call, deferred := h.admit(ctx, spec, external)
		if deferred != "" {
			h.record(id, func(st *consumers.Stats) { st.Deferred++; st.LastCode = deferred })
			return decisions.Response{}, decisions.NewError(deferred, nil)
		}
		defer release()
		ctx = withDecisionUsageCategory(admitted, spec.BudgetCategory)
		if router, ok := s.Decisions.(*routedDecisions); ok {
			ctx = context.WithValue(ctx, router, generation)
			ctx = context.WithValue(ctx, decisionModelEpochKey{}, modelEpoch)
		}
		response, err := s.Decisions.Evaluate(ctx, request)
		if err != nil && decisions.ErrorCode(err) == decisions.CodeImagesUnsupported {
			h.mu.Lock()
			h.imagesPinned[modelEpoch] = true
			h.mu.Unlock()
		}
		if err != nil && h.wasPreempted(call) {
			h.record(id, func(st *consumers.Stats) { st.Preempted++; st.LastCode = consumers.CodePreempted })
			return decisions.Response{}, decisions.NewError(consumers.CodePreempted, err)
		}
		h.recordOutcome(id, response, err, external)
		if !result.Current() {
			return response, decisions.NewError(consumers.CodeConfigurationChanged, context.Canceled)
		}
		return response, err
	}
	return result
}

func (h *decisionHub) capabilities(settings config.DecisionsConfig, status decisions.Status, modelEpoch uint64) consumers.Capabilities {
	caps := consumers.Capabilities{Images: consumers.ImagesUnknown}
	if settings.Backend == "provider" {
		caps.External = true
		availability := status.ModelAvailability.Snapshot(time.Now())
		if availability.State == "usable" && availability.Model != nil {
			if availability.Model.SupportsImages {
				caps.Images = consumers.ImagesSupported
			} else {
				caps.Images = consumers.ImagesUnsupported
			}
		}
	} else {
		caps.CPUOnly = status.CPUFallbackActive
		switch {
		case !status.Ready:
		case status.ProjectorSHA256 == "":
			caps.Images = consumers.ImagesUnsupported
		case status.Runtime.ProjectorRevision == status.ProjectorRevision && status.Runtime.ProjectorSHA256 == status.ProjectorSHA256:
			caps.Images = consumers.ImagesSupported
		}
	}
	if h.imagesAreUnsupported(modelEpoch) {
		caps.Images = consumers.ImagesUnsupported
	}
	return caps
}

func (h *decisionHub) recordOutcome(id consumers.ID, response decisions.Response, err error, external bool) {
	h.record(id, func(st *consumers.Stats) {
		st.Attempts++
		if err == nil {
			st.Successes++
			st.LastCode = consumers.CodeAvailable
		} else {
			st.Failures++
			st.LastCode = decisions.ErrorCode(err)
			if errors.Is(err, context.DeadlineExceeded) || st.LastCode == decisions.CodeRuntimeScoreTimeout {
				st.Timeouts++
			}
		}
		st.InputTokens += response.Usage.PromptTokens
		st.OutputTokens += response.Usage.CompletionTokens
		if cost := response.Usage.CostUSD; cost != nil && !math.IsNaN(*cost) && !math.IsInf(*cost, 0) {
			st.CostUSD += *cost
		} else if external {
			st.CostMissing++
		}
	})
}

// RefreshImageCapability is for background consumers whose first Snapshot says
// images_unknown on the provider backend because the model catalog is stale. It
// reads the catalog (no inference, no cost) at most once per catalog TTL, shared
// across all consumers, bounded by the consumer's timeout, and returns a fresh
// snapshot. Snapshot itself never refreshes anything.
func (h *decisionHub) RefreshImageCapability(ctx context.Context, cfg *config.Config, id consumers.ID) consumers.Snapshot {
	snapshot := h.Snapshot(cfg, id)
	spec, known := consumers.Registry[id]
	router, routed := h.s.Decisions.(*routedDecisions)
	if !known || spec.Modality != consumers.ImageRequired || snapshot.Code != consumers.CodeImagesUnknown || snapshot.Backend != "provider" || !routed || snapshot.Provider == "" {
		return snapshot
	}
	h.mu.Lock()
	now := h.clock()
	if !h.lastCatalogRefresh.IsZero() && now.Sub(h.lastCatalogRefresh) < decisionCatalogTTL {
		h.mu.Unlock()
		return snapshot
	}
	h.lastCatalogRefresh = now
	h.mu.Unlock()
	callCtx, cancel := context.WithTimeout(ctx, spec.Timeout)
	defer cancel()
	if _, err := router.Models(callCtx, snapshot.Provider); err != nil {
		return snapshot
	}
	return h.Snapshot(cfg, id)
}
