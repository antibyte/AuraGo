package retrogames

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	staleAfter       = 7 * 24 * time.Hour
	autoRetryHoldOff = time.Hour
)

var (
	ErrDisabled          = errors.New("retrogames: retro games are disabled")
	ErrSyncRunning       = errors.New("retrogames: a sync is already running")
	ErrUnknownGame       = errors.New("retrogames: unknown game")
	ErrInvalidIdentifier = errors.New("retrogames: invalid identifier")
)

// Settings is the runtime configuration of the Service.
type Settings struct {
	Enabled     bool
	AgentAccess bool
	DataDir     string
}

// Deps are the Service's collaborators; zero values select production defaults.
type Deps struct {
	Logger  *slog.Logger
	Fetcher Fetcher
	Now     func() time.Time
	Sleep   func(context.Context, time.Duration) error
}

// PlatformStatus is the index state of one platform.
type PlatformStatus struct {
	Key           string `json:"key"`
	Label         string `json:"label"`
	Count         int    `json:"count"`
	State         string `json:"state"`
	Page          int    `json:"page,omitempty"`
	Fetched       int    `json:"fetched,omitempty"`
	LastSuccessAt int64  `json:"last_success_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
}

// Status is the overall index state.
type Status struct {
	Enabled    bool             `json:"enabled"`
	Syncing    bool             `json:"syncing"`
	TotalGames int              `json:"total_games"`
	Platforms  []PlatformStatus `json:"platforms"`
	Systems    []CountEntry     `json:"systems"`
}

type syncRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Service owns the store and the background sync.
type Service struct {
	logger  *slog.Logger
	fetcher Fetcher
	now     func() time.Time
	sleep   func(context.Context, time.Duration) error

	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	wg              sync.WaitGroup

	mu              sync.Mutex
	settings        Settings
	store           *Store
	storeDir        string
	run             *syncRun
	progress        map[string]SourceProgress
	lastAutoFailure time.Time
}

// NewService builds a passive service: no disk or network access until used.
func NewService(deps Deps) *Service {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	var fetcher Fetcher = deps.Fetcher
	if fetcher == nil {
		fetcher = NewHTTPFetcher()
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	sleep := deps.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{logger: logger, fetcher: fetcher, now: now, sleep: sleep, lifecycleCtx: ctx, lifecycleCancel: cancel, progress: map[string]SourceProgress{}}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Configure applies new settings. Disabling (or moving the data directory)
// cancels a running sync, waits for it and closes the store.
func (s *Service) Configure(next Settings) {
	next.DataDir = strings.TrimSpace(next.DataDir)
	s.mu.Lock()
	s.settings = next
	var run *syncRun
	var st *Store
	if !next.Enabled || (s.store != nil && s.storeDir != next.DataDir) {
		run = s.run
		st = s.store
		s.store = nil
		s.storeDir = ""
		if !next.Enabled {
			s.progress = map[string]SourceProgress{}
		}
	}
	s.mu.Unlock()
	if run != nil {
		run.cancel()
		<-run.done
	}
	if st != nil {
		if err := st.Close(); err != nil {
			s.logger.Warn("retro games could not close the index", "error", err)
		}
	}
}

// Enabled reports whether the integration is on.
func (s *Service) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.Enabled
}

// AgentAccess reports whether the agent tool may use the service.
func (s *Service) AgentAccess() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.Enabled && s.settings.AgentAccess
}

func (s *Service) openStore() (*Store, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.openStoreLocked()
}

func (s *Service) openStoreLocked() (*Store, error) {
	if !s.settings.Enabled {
		return nil, ErrDisabled
	}
	if s.store != nil {
		return s.store, nil
	}
	if s.settings.DataDir == "" {
		return nil, errors.New("retrogames: no data directory configured")
	}
	st, err := OpenStore(s.settings.DataDir, s.logger)
	if err != nil {
		return nil, err
	}
	s.store = st
	s.storeDir = s.settings.DataDir
	return st, nil
}

// StartSync starts a manual sync of every source. It ignores the automatic
// retry hold-off and accepts a source that shrank (the user asked for it).
func (s *Service) StartSync() error {
	return s.startSync(Sources(), true)
}

func (s *Service) startSync(srcs []Source, manual bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.settings.Enabled {
		return ErrDisabled
	}
	if s.run != nil {
		return ErrSyncRunning
	}
	st, err := s.openStoreLocked()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(s.lifecycleCtx)
	run := &syncRun{cancel: cancel, done: make(chan struct{})}
	s.run = run
	s.progress = make(map[string]SourceProgress, len(srcs))
	for _, src := range srcs {
		s.progress[src.Key] = SourceProgress{Source: src.Key, State: StatePending}
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(run.done)
		defer cancel()
		sy := &syncer{fetcher: s.fetcher, store: st, now: s.now, sleep: s.sleep, logger: s.logger, progress: s.setProgress, acceptShrink: manual}
		err := sy.run(ctx, srcs)
		canceled := ctx.Err() != nil
		s.mu.Lock()
		if s.run == run {
			s.run = nil
		}
		switch {
		case err == nil:
			s.lastAutoFailure = time.Time{}
		case !canceled:
			s.lastAutoFailure = s.now()
		}
		// A run that ended early (cancel, rate limit, store failure) leaves
		// sources pending or running; they are not syncing any more.
		for key, p := range s.progress {
			if p.State == StatePending || p.State == StateRunning {
				p.State = StateIdle
				s.progress[key] = p
			}
		}
		s.mu.Unlock()
		if err != nil && !canceled {
			s.logger.Warn("retro games index sync finished with errors", "error", err)
		}
	}()
	return nil
}

func (s *Service) setProgress(p SourceProgress) {
	s.mu.Lock()
	s.progress[p.Source] = p
	s.mu.Unlock()
}

// ensureFresh starts an automatic sync of the sources that never succeeded or
// are older than staleAfter, unless a sync runs or the last automatic attempt
// failed within the hold-off. Fresh sources are never re-fetched automatically.
func (s *Service) ensureFresh(ctx context.Context) {
	st, err := s.openStore()
	if err != nil {
		return
	}
	s.mu.Lock()
	busy := s.run != nil
	held := !s.lastAutoFailure.IsZero() && s.now().Sub(s.lastAutoFailure) < autoRetryHoldOff
	s.mu.Unlock()
	if busy || held {
		return
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		s.logger.Warn("retro games could not check index freshness", "error", err)
		return
	}
	now := s.now()
	var stale []Source
	for _, src := range Sources() {
		state, ok := states[src.Key]
		if !ok || state.LastSuccessAt.IsZero() || now.Sub(state.LastSuccessAt) > staleAfter {
			stale = append(stale, src)
		}
	}
	if len(stale) == 0 {
		return
	}
	if err := s.startSync(stale, false); err != nil && !errors.Is(err, ErrSyncRunning) {
		s.logger.Warn("retro games could not start the index sync", "error", err)
	}
}

// WaitIdle blocks until no sync runs or ctx ends.
func (s *Service) WaitIdle(ctx context.Context) error {
	for {
		s.mu.Lock()
		run := s.run
		s.mu.Unlock()
		if run == nil {
			return nil
		}
		select {
		case <-run.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Shutdown cancels a running sync and closes the store.
func (s *Service) Shutdown(ctx context.Context) error {
	s.lifecycleCancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	s.mu.Lock()
	st := s.store
	s.store = nil
	s.run = nil
	s.mu.Unlock()
	if st != nil {
		return st.Close()
	}
	return nil
}

// Status reports the index state. It never fails because of a disabled service.
func (s *Service) Status(ctx context.Context) (Status, error) {
	if !s.Enabled() {
		return Status{Platforms: []PlatformStatus{}, Systems: []CountEntry{}}, nil
	}
	s.ensureFresh(ctx)
	st, err := s.openStore()
	if err != nil {
		return Status{}, err
	}
	counts, err := st.PlatformCounts(ctx)
	if err != nil {
		return Status{}, err
	}
	systems, err := st.SystemCounts(ctx, PlatformConsole)
	if err != nil {
		return Status{}, err
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		return Status{}, err
	}
	s.mu.Lock()
	syncing := s.run != nil
	progress := make(map[string]SourceProgress, len(s.progress))
	for k, v := range s.progress {
		progress[k] = v
	}
	s.mu.Unlock()
	out := Status{Enabled: true, Syncing: syncing, Systems: systems, Platforms: []PlatformStatus{}}
	for _, src := range Sources() {
		ps := PlatformStatus{Key: src.Key, Label: src.Label, Count: counts[src.Key], State: StateIdle}
		if p, ok := progress[src.Key]; ok {
			ps.State, ps.Page, ps.Fetched, ps.LastError = p.State, p.Page, p.Fetched, p.Err
		}
		if state, ok := states[src.Key]; ok {
			if !state.LastSuccessAt.IsZero() {
				ps.LastSuccessAt = state.LastSuccessAt.Unix()
			}
			if ps.LastError == "" {
				ps.LastError = state.LastError
			}
		}
		out.TotalGames += ps.Count
		out.Platforms = append(out.Platforms, ps)
	}
	return out, nil
}

// Search validates q, triggers a lazy sync if needed and searches the index.
func (s *Service) Search(ctx context.Context, q Query) (SearchResult, error) {
	q, err := q.Normalize()
	if err != nil {
		return SearchResult{}, err
	}
	s.ensureFresh(ctx)
	st, err := s.openStore()
	if err != nil {
		return SearchResult{}, err
	}
	return st.Search(ctx, q)
}

// HallOfFame returns the curated list, optionally for one platform.
func (s *Service) HallOfFame(ctx context.Context, platform string) ([]Game, error) {
	if !s.Enabled() {
		return nil, ErrDisabled
	}
	if platform != "" && !ValidPlatform(platform) {
		return nil, fmt.Errorf("%w: unknown platform", ErrInvalidQuery)
	}
	s.ensureFresh(ctx)
	return HallOfFame(platform), nil
}

// Game finds a game in the index, then in the Hall of Fame.
func (s *Service) Game(ctx context.Context, id string) (Game, error) {
	if !s.Enabled() {
		return Game{}, ErrDisabled
	}
	id = strings.TrimSpace(id)
	if !ValidIdentifier(id) {
		return Game{}, ErrInvalidIdentifier
	}
	st, err := s.openStore()
	if err != nil {
		return Game{}, err
	}
	g, ok, err := st.Game(ctx, id)
	if err != nil {
		return Game{}, err
	}
	if ok {
		return g, nil
	}
	if g, ok := hallOfFameGame(id); ok {
		return g, nil
	}
	return Game{}, ErrUnknownGame
}

// Library returns favorites, recent plays and broken markers with availability.
func (s *Service) Library(ctx context.Context) (Library, error) {
	st, err := s.openStore()
	if err != nil {
		return Library{}, err
	}
	lib, err := st.Library(ctx)
	if err != nil {
		return Library{}, err
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		return Library{}, err
	}
	mark := func(entries []LibraryEntry) {
		for i := range entries {
			e := &entries[i]
			if e.Available {
				continue
			}
			if _, ok := hallOfFameGame(e.Identifier); ok {
				e.Available = true
				continue
			}
			if state, ok := states[e.Platform]; !ok || state.LastSuccessAt.IsZero() {
				e.Available = true
			}
		}
	}
	mark(lib.Favorites)
	mark(lib.Recent)
	mark(lib.Broken)
	return lib, nil
}

// SetFavorite adds or removes a favorite.
func (s *Service) SetFavorite(ctx context.Context, id string, on bool) error {
	return s.mutateLibrary(ctx, id, on, func(st *Store, g Game) error { return st.SetFavorite(ctx, g, on, s.now()) })
}

// SetBroken adds or removes a "doesn't start" marker.
func (s *Service) SetBroken(ctx context.Context, id string, on bool) error {
	return s.mutateLibrary(ctx, id, on, func(st *Store, g Game) error { return st.SetBroken(ctx, g, on, s.now()) })
}

// RecordPlay counts one start of a game.
func (s *Service) RecordPlay(ctx context.Context, id string) error {
	return s.mutateLibrary(ctx, id, true, func(st *Store, g Game) error { return st.RecordPlay(ctx, g, s.now()) })
}

func (s *Service) mutateLibrary(ctx context.Context, id string, on bool, apply func(*Store, Game) error) error {
	id = strings.TrimSpace(id)
	if !ValidIdentifier(id) {
		return ErrInvalidIdentifier
	}
	st, err := s.openStore()
	if err != nil {
		return err
	}
	g := Game{Identifier: id}
	if on {
		g, err = s.Game(ctx, id)
		if errors.Is(err, ErrUnknownGame) {
			snap, ok, snapErr := st.Snapshot(ctx, id)
			if snapErr != nil {
				return snapErr
			}
			if !ok {
				return ErrUnknownGame
			}
			g, err = snap, nil
		}
		if err != nil {
			return err
		}
	}
	return apply(st, g)
}
