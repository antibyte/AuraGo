package flows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// MissionBridge connects flows with Mission Control. Plan 1c implements it on top of
// MissionManagerV2, the mission history, the planner and notifications.
//
// The rule for implementations: no method of the bridge may synchronously call a
// Service method that takes a flow lock, for any flow. Today those are Publish,
// SetEnabled, DeleteFlow and DeleteFlowForMission. The Service calls the bridge while
// it holds a flow lock, and FlowRunFinished can run inside such an operation (deleting a
// flow ends its queued runs through Runner.CancelFlow on the caller's goroutine), so
// such a call could wait for itself, or for another operation that waits for it. Work that needs one of these methods goes through a goroutine, as
// FlowHooks.FlowMissionDeleted does. Lock-free reads (GetFlow, ListFlows, and the timer
// queries the run part of the Service provides) may be called synchronously; they must
// stay lock-free.
type MissionBridge interface {
	// CreateFlowMission creates the (disabled) mission that represents a flow.
	CreateFlowMission(flowID, name string) (string, error)
	// SyncFlowMission stores the published name and trigger bindings of a flow mission.
	SyncFlowMission(missionID, name string, bindings []TriggerBinding) error
	// SetFlowMissionEnabled turns the flow's Mission Control triggers on or off.
	SetFlowMissionEnabled(missionID string, enabled bool) error
	// FlowMissionEnabled reports the mission's enabled switch.
	FlowMissionEnabled(missionID string) bool
	// DeleteFlowMission removes the mission; it must not call back into Service.DeleteFlow.
	// A mission that is already gone is not an error, so a failed delete can be retried.
	DeleteFlowMission(missionID string) error
	// FlowRunStarted records a live run in the mission history and returns the history id.
	FlowRunStarted(missionID string, rec RunRecord) string
	// FlowRunFinished records the result, fires dependent missions and sends failure notifications.
	FlowRunFinished(info RunFinishedInfo)
}

// RunFinishedInfo describes a finished live run for Mission Control. Every live run is
// reported, also one whose flow was deleted while the run was queued or active; then
// the fields hold what is still known.
type RunFinishedInfo struct {
	// MissionID is the mission the run was recorded in at its start, else the flow's
	// mission. It is empty when the flow has no mission, or when the flow is gone and
	// the run never started.
	MissionID string
	// HistoryID is the mission history entry FlowRunStarted returned. It is empty when
	// FlowRunStarted was not called: the run never started (cancelled while queued, or
	// shut down), its flow could not be read at the start, or the flow has no mission.
	// The bridge then records the run without an entry to complete, or skips it.
	HistoryID string
	// FlowName is the flow's current name, or the name in the run's document when the
	// flow is gone; empty when neither can be read.
	FlowName string
	// NotifyOnError is the setting of the revision the run executed; empty when no
	// document of the flow can be read.
	NotifyOnError string
	Record        RunRecord
	Result        RunResult
	// Outputs holds the outputs of the final nodes (nodes without successors) of the
	// revision the run executed, by key. It is empty, never nil, when the run produced
	// none (it never started) or no document of the flow can be read.
	Outputs map[string]any
}

// ServiceConfig tunes the service; zero values use the defaults.
type ServiceConfig struct {
	MaxParallelRuns   int
	MaxParallelNodes  int
	MaxQueuedPerFlow  int
	RunRetentionDays  int
	MaxRunsPerFlow    int
	RetentionInterval time.Duration
}

// ValidationError carries the issues that block saving or publishing.
type ValidationError struct {
	Issues []Issue
}

func (e *ValidationError) Error() string {
	n := 0
	for _, is := range e.Issues {
		if is.Severity == SeverityError {
			n++
		}
	}
	return fmt.Sprintf("the flow has %d problem(s)", n)
}

var (
	ErrNotPublished = errors.New("the flow has not been published yet")
	ErrNoTrigger    = errors.New("the flow has no matching active trigger")
)

// Service is the entry point for the HTTP API, Mission Control and timers.
type Service struct {
	store    *Store
	reg      *Registry
	services *Services
	bridge   MissionBridge
	cfg      ServiceConfig
	logger   *slog.Logger
	engine   *Engine
	runner   *Runner
	timers   *TimerService
	locks    flowLocks

	// startMu serializes Start. It is not taken by Shutdown, which only needs mu.
	startMu sync.Mutex
	// mu guards the fields below. It is held only briefly and never across a call into
	// the bridge, the runner or the store.
	mu       sync.Mutex
	history  map[string]runHistory // live runs that started, by run id
	started  bool
	closed   bool
	bootTime time.Time
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// NewService wires engine, runner and timers. Call Start, and Shutdown when done.
func NewService(store *Store, reg *Registry, services *Services, bridge MissionBridge, cfg ServiceConfig, logger *slog.Logger) *Service {
	if services == nil {
		services = &Services{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.RunRetentionDays <= 0 {
		cfg.RunRetentionDays = 30
	}
	if cfg.MaxRunsPerFlow <= 0 {
		cfg.MaxRunsPerFlow = 200
	}
	if cfg.RetentionInterval <= 0 {
		cfg.RetentionInterval = time.Hour
	}
	s := &Service{store: store, reg: reg, services: services, bridge: bridge, cfg: cfg, logger: logger,
		history: map[string]runHistory{}, stop: make(chan struct{}), done: make(chan struct{})}
	s.bootTime = services.Now()
	s.engine = NewEngine(reg, services, logger, cfg.MaxParallelNodes)
	s.runner = NewRunner(s.engine, store, RunnerHooks{OnRunStarted: s.onRunStarted, OnRunFinished: s.onRunFinished},
		RunnerConfig{MaxParallelRuns: cfg.MaxParallelRuns, MaxQueuedPerFlow: cfg.MaxQueuedPerFlow}, logger)
	s.timers = NewTimerService(store, services.clock(), s.onTimerFired, s.onTimerMissed, logger)
	// Yearly timers recur at their local time of day, in the zone the triggers are bound in.
	s.timers.SetLocation(services.Loc())
	return s
}

// Registry returns the node registry.
func (s *Service) Registry() *Registry { return s.reg }

// Store returns the flow store.
func (s *Service) Store() *Store { return s.store }

// Runner returns the run scheduler.
func (s *Service) Runner() *Runner { return s.runner }

// Start marks runs interrupted by a restart, arms the timers and starts the retention loop.
// Runs started after NewService (for example by Mission Control during startup) are kept,
// so Start may be called after MissionManagerV2.Start.
//
// Start is idempotent: once it succeeded, further calls return nil and do nothing, so
// there is never a second retention loop. Concurrent calls wait for each other. After a
// failed Start nothing runs and Start may be called again. After Shutdown, Start returns
// ErrRunnerClosed.
//
// Known limits: Start repairs nothing that a crash cut short. A stop between the store
// publish and the mission sync (Publish), or between the mission switch and the timers
// (SetEnabled), leaves the mission or the timers behind the store until the next
// successful Publish or SetEnabled of that flow. Shutdown does not wait for flow
// operations in flight; stop the API before closing the store (see Shutdown).
func (s *Service) Start(ctx context.Context) error {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.mu.Lock()
	closed, started := s.closed, s.started
	s.mu.Unlock()
	if closed {
		return ErrRunnerClosed
	}
	if started {
		return nil
	}
	// A failed earlier attempt may have marked the interrupted runs already; doing it
	// again only touches runs from before the boot time, so it is harmless.
	n, err := s.store.MarkInterruptedRuns(ctx, s.bootTime, s.now())
	if err != nil {
		return err
	}
	if n > 0 {
		s.logger.Warn("flow runs were interrupted by a restart", "count", n)
	}
	if err := s.timers.Start(ctx); err != nil {
		if s.isClosed() { // Shutdown stopped the timers while they started
			return ErrRunnerClosed
		}
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		// Shutdown ran meanwhile and stopped the timers; it does not wait for a loop
		// that was not started, so none may start now.
		return ErrRunnerClosed
	}
	s.started = true
	go s.retentionLoop()
	return nil
}

// Shutdown stops the timers, cancels the runs and waits for them and for the retention
// loop, or until ctx ends (then it returns ctx's error and the rest ends in the
// background). It may be called more than once, and without Start.
//
// Known limit: Shutdown does not wait for flow operations in flight (CreateFlow,
// SaveDraft, Publish, SetEnabled, the deletes), which may still use the store and the
// bridge. The caller stops the API and Mission Control's calls into the Service first,
// and closes the store only after Shutdown.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	started := s.started
	s.mu.Unlock()
	s.stopOnce.Do(func() { close(s.stop) })
	s.timers.Stop()
	err := s.runner.Shutdown(ctx)
	if started {
		select {
		case <-s.done:
		case <-ctx.Done():
			if err == nil {
				err = ctx.Err()
			}
		}
	}
	return err
}

func (s *Service) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Service) retentionLoop() {
	defer close(s.done)
	ticker := time.NewTicker(s.cfg.RetentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			if _, err := s.PruneNow(context.Background()); err != nil {
				s.logger.Warn("flow run retention failed", "error", err)
			}
		}
	}
}

// PruneNow applies run retention and forgets old event logs.
func (s *Service) PruneNow(ctx context.Context) (int64, error) {
	n, err := s.store.PruneRuns(ctx, s.cfg.RunRetentionDays, s.cfg.MaxRunsPerFlow, s.now())
	s.runner.Bus().Sweep()
	return n, err
}

func (s *Service) now() time.Time { return s.services.Now() }

func (s *Service) vc(mode ValidationMode) ValidateContext {
	return ValidateContext{Mode: mode, Now: s.now(), Location: s.services.Loc()}
}

// CreateRequest creates a flow: empty, from a template, or from an imported document.
type CreateRequest struct {
	Name      string
	Template  string
	Import    *Flow
	Translate func(string) string
}

// CreateFlow creates the flow and its mission.
func (s *Service) CreateFlow(ctx context.Context, req CreateRequest) (*FlowRecord, error) {
	var f *Flow
	switch {
	case req.Import != nil:
		clone, err := req.Import.Clone()
		if err != nil {
			return nil, err
		}
		f = clone
	case req.Template != "":
		tpl, err := TemplateFlow(req.Template, req.Translate)
		if err != nil {
			return nil, err
		}
		f = tpl
	default:
		f = &Flow{Schema: SchemaVersion}
	}
	f.ID = NewFlowID()
	f.Kind = KindFlow
	if name := strings.TrimSpace(req.Name); name != "" {
		f.Name = name
	}
	f.Normalize()
	if issues := Validate(f, s.reg, s.vc(ModeDraft)); HasErrors(issues) {
		return nil, &ValidationError{Issues: issues}
	}
	missionID, err := s.bridge.CreateFlowMission(f.ID, f.Name)
	if err != nil {
		return nil, fmt.Errorf("create flow mission: %w", err)
	}
	rec, err := s.store.CreateFlow(ctx, f, missionID, s.now())
	if err != nil {
		_ = s.bridge.DeleteFlowMission(missionID)
		return nil, err
	}
	return rec, nil
}

// GetFlow returns a flow. It takes no flow lock, so a MissionBridge may call it
// synchronously (see MissionBridge); keep it that way.
func (s *Service) GetFlow(ctx context.Context, id string) (*FlowRecord, error) {
	return s.store.GetFlow(ctx, id)
}

// Validate checks a document without saving it.
func (s *Service) Validate(doc *Flow, mode ValidationMode) []Issue {
	return Validate(doc, s.reg, s.vc(mode))
}

// SaveDraft validates (draft rules) and stores a new draft revision. The returned issues
// include warnings for the editor's hint panel.
func (s *Service) SaveDraft(ctx context.Context, id string, doc *Flow, baseRevision int) (int, []Issue, error) {
	if doc == nil {
		return 0, nil, errors.New("a flow document is required")
	}
	f, err := doc.Clone()
	if err != nil {
		return 0, nil, err
	}
	f.ID = id
	f.Kind = KindFlow
	f.Normalize()
	issues := Validate(f, s.reg, s.vc(ModeDraft))
	if HasErrors(issues) {
		return 0, issues, &ValidationError{Issues: issues}
	}
	rev, err := s.store.SaveDraft(ctx, id, f, baseRevision, s.now())
	return rev, issues, err
}

// FlowSummary is one card on the editor's start page.
type FlowSummary struct {
	ID                    string        `json:"id"`
	Name                  string        `json:"name"`
	Description           string        `json:"description,omitempty"`
	MissionID             string        `json:"mission_id"`
	Enabled               bool          `json:"enabled"`
	Published             bool          `json:"published"`
	HasUnpublishedChanges bool          `json:"has_unpublished_changes"`
	DraftRevision         int           `json:"draft_revision"`
	LiveRevision          int           `json:"live_revision"`
	UpdatedAt             time.Time     `json:"updated_at"`
	LastRun               *RunRecord    `json:"last_run,omitempty"`
	Triggers              []string      `json:"triggers"`
	Preview               []PreviewNode `json:"preview"`
}

// PreviewNode positions a node in the start page's mini preview.
type PreviewNode struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Category string  `json:"category"`
}

// ListFlows returns the start page cards. It takes no flow lock, so a MissionBridge may
// call it synchronously (see MissionBridge); keep it that way.
func (s *Service) ListFlows(ctx context.Context) ([]FlowSummary, error) {
	records, err := s.store.ListFlows(ctx, KindFlow)
	if err != nil {
		return nil, err
	}
	last, err := s.store.LastLiveRuns(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]FlowSummary, 0, len(records))
	for _, rec := range records {
		sum := FlowSummary{
			ID: rec.ID, Name: rec.Name, Description: rec.Description, MissionID: rec.MissionID,
			Published: rec.Published(), HasUnpublishedChanges: rec.HasUnpublishedChanges(),
			DraftRevision: rec.DraftRevision, LiveRevision: rec.LiveRevision, UpdatedAt: rec.UpdatedAt,
			Triggers: []string{}, Preview: []PreviewNode{},
		}
		if run, ok := last[rec.ID]; ok {
			r := run
			sum.LastRun = &r
		}
		for _, n := range rec.Draft.Nodes {
			category := "unknown"
			if def, ok := s.reg.Lookup(n.Type); ok {
				category = def.Category
				if def.Trigger && !n.Settings.Disabled {
					sum.Triggers = append(sum.Triggers, n.Type)
				}
			}
			sum.Preview = append(sum.Preview, PreviewNode{X: n.Position.X, Y: n.Position.Y, Category: category})
		}
		sum.Enabled = rec.MissionID != "" && s.bridge.FlowMissionEnabled(rec.MissionID)
		out = append(out, sum)
	}
	return out, nil
}

// DiffSummary counts what changed between the published revision and the draft.
type DiffSummary struct {
	FirstPublish bool `json:"first_publish"`
	AddedNodes   int  `json:"added_nodes"`
	RemovedNodes int  `json:"removed_nodes"`
	ChangedNodes int  `json:"changed_nodes"`
	ChangedEdges int  `json:"changed_edges"`
}

// PublishPreview is shown in the publish dialog.
type PublishPreview struct {
	Issues     []Issue         `json:"issues"`
	Effects    []EffectSummary `json:"effects"`
	Diff       DiffSummary     `json:"diff"`
	CanPublish bool            `json:"can_publish"`
}

// PublishPreview validates the draft with publish rules and summarises effects and changes.
func (s *Service) PublishPreview(ctx context.Context, id string) (*PublishPreview, error) {
	rec, err := s.store.GetFlow(ctx, id)
	if err != nil {
		return nil, err
	}
	issues := append(Validate(rec.Draft, s.reg, s.vc(ModePublish)), selfTriggerIssues(rec.Draft, rec.MissionID)...)
	if issues == nil {
		issues = []Issue{}
	}
	effects := CollectEffects(rec.Draft, s.reg)
	if effects == nil {
		effects = []EffectSummary{}
	}
	return &PublishPreview{Issues: issues, Effects: effects, Diff: DiffFlows(rec.Live, rec.Draft), CanPublish: !HasErrors(issues)}, nil
}

// DiffFlows compares the live revision with the draft. Moving a node is not a change.
//
// A nil live revision is a first publish; a nil draft counts as every live node and
// edge removed. Nodes are matched by id. Edges are compared as a set of their source
// and target ports, without their ids: edges with the same ports collapse into one (the
// validator rejects such duplicates anyway), and an edge whose ends changed counts
// twice, once for the old and once for the new connection. The counts do not depend on
// map order, so the result is deterministic.
func DiffFlows(live, draft *Flow) DiffSummary {
	if draft == nil {
		draft = &Flow{}
	}
	if live == nil {
		return DiffSummary{FirstPublish: true, AddedNodes: len(draft.Nodes)}
	}
	signature := func(n Node) string {
		n.Position = Point{}
		data, _ := json.Marshal(n)
		return string(data)
	}
	before := map[string]string{}
	for _, n := range live.Nodes {
		before[n.ID] = signature(n)
	}
	var d DiffSummary
	seen := map[string]bool{}
	for _, n := range draft.Nodes {
		seen[n.ID] = true
		old, ok := before[n.ID]
		switch {
		case !ok:
			d.AddedNodes++
		case old != signature(n):
			d.ChangedNodes++
		}
	}
	for id := range before {
		if !seen[id] {
			d.RemovedNodes++
		}
	}
	edgeKey := func(e Edge) string {
		return e.Source.Node + "|" + e.Source.Port + "|" + e.Target.Node + "|" + e.Target.Port
	}
	liveEdges, draftEdges := map[string]bool{}, map[string]bool{}
	for _, e := range live.Edges {
		liveEdges[edgeKey(e)] = true
	}
	for _, e := range draft.Edges {
		draftEdges[edgeKey(e)] = true
	}
	for k := range draftEdges {
		if !liveEdges[k] {
			d.ChangedEdges++
		}
	}
	for k := range liveEdges {
		if !draftEdges[k] {
			d.ChangedEdges++
		}
	}
	return d
}
