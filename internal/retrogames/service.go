package retrogames

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
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

	// openMu serializes store opens so the slow OpenStore runs outside mu
	// (Enabled, Status and the sync bookkeeping never wait for the disk).
	openMu sync.Mutex

	mu       sync.Mutex
	settings Settings
	closed   bool
	store    *Store
	storeDir string
	run      *syncRun
	progress map[string]SourceProgress
	// teardowns holds the pending background teardowns started by Configure.
	teardowns       map[chan struct{}]struct{}
	lastAutoFailure time.Time
	// freshUntil is the first instant a source can become stale, learned from
	// the last freshness check; until then ensureFresh skips the database read.
	freshUntil time.Time
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
	return &Service{logger: logger, fetcher: fetcher, now: now, sleep: sleep, lifecycleCtx: ctx, lifecycleCancel: cancel, progress: map[string]SourceProgress{}, teardowns: map[chan struct{}]struct{}{}}
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

// Configure applies new settings and never waits: the callers publish config
// snapshots under locks of their own. Disabling (or moving the data directory)
// detaches the store at once, so every later call is refused or reopens the
// index elsewhere, and hands the old store and a running sync to a background
// teardown that cancels the sync, waits for it and then closes the store.
// Shutdown and WaitIdle wait for that teardown. After Shutdown Configure only
// records the settings: a shut-down service never opens anything again.
func (s *Service) Configure(next Settings) {
	next.DataDir = strings.TrimSpace(next.DataDir)
	var run *syncRun
	var st *Store
	var tracked chan struct{}
	s.mu.Lock()
	s.settings = next
	s.freshUntil = time.Time{}
	if !next.Enabled || (s.store != nil && s.storeDir != next.DataDir) {
		// s.run stays set: the sync's own goroutine clears it when it ends.
		run = s.run
		st = s.store
		s.store = nil
		s.storeDir = ""
		if !next.Enabled {
			s.progress = map[string]SourceProgress{}
		}
		// wg.Add runs under s.mu while the service is open: Shutdown sets
		// closed under s.mu before it waits, so Add never races Wait.
		if (run != nil || st != nil) && !s.closed {
			s.wg.Add(1)
			tracked = make(chan struct{})
			s.teardowns[tracked] = struct{}{}
		}
	}
	s.mu.Unlock()
	if run == nil && st == nil {
		return
	}
	if tracked == nil {
		// Shut down already: nobody waits for a goroutine, so finish here.
		s.teardown(run, st)
		return
	}
	go func() {
		defer s.wg.Done()
		defer s.finishTeardown(tracked)
		s.teardown(run, st)
	}()
}

// teardown stops a detached sync and, once it ended, closes the store it used.
func (s *Service) teardown(run *syncRun, st *Store) {
	if run != nil {
		run.cancel()
		<-run.done
	}
	if st != nil {
		s.closeStore(st)
	}
}

// finishTeardown marks one background teardown as done.
func (s *Service) finishTeardown(tracked chan struct{}) {
	s.mu.Lock()
	delete(s.teardowns, tracked)
	s.mu.Unlock()
	close(tracked)
}

// Enabled reports whether the integration is on. A shut-down service is off.
func (s *Service) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.Enabled && !s.closed
}

// AgentAccess reports whether the agent tool may use the service.
func (s *Service) AgentAccess() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.Enabled && !s.closed && s.settings.AgentAccess
}

// disabledLocked returns the error that refuses work, or nil while the
// service is on. s.mu must be held.
func (s *Service) disabledLocked() error {
	if s.closed {
		return fmt.Errorf("%w: service is shut down", ErrDisabled)
	}
	if !s.settings.Enabled {
		return ErrDisabled
	}
	return nil
}

// storeErr turns a store failure into ErrDisabled when the service was
// disabled or shut down while the request was running (the store handle is
// then closed and the raw error would only say "database is closed").
func (s *Service) storeErr(err error) error {
	if err == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if derr := s.disabledLocked(); derr != nil {
		return derr
	}
	return err
}

// openedStoreLocked returns the open store, or the directory to open. s.mu
// must be held.
func (s *Service) openedStoreLocked() (*Store, string, error) {
	if err := s.disabledLocked(); err != nil {
		return nil, "", err
	}
	if s.store != nil {
		return s.store, "", nil
	}
	if s.settings.DataDir == "" {
		return nil, "", fmt.Errorf("%w: no data directory configured", ErrDisabled)
	}
	return nil, s.settings.DataDir, nil
}

// openStore returns the index, opening it on first use. The database is
// opened without holding s.mu; openMu keeps concurrent callers from opening
// it twice.
func (s *Service) openStore() (*Store, error) {
	s.mu.Lock()
	st, dir, err := s.openedStoreLocked()
	s.mu.Unlock()
	if st != nil || err != nil {
		return st, err
	}

	s.openMu.Lock()
	defer s.openMu.Unlock()
	// Another caller may have opened it while this one waited.
	s.mu.Lock()
	st, dir, err = s.openedStoreLocked()
	s.mu.Unlock()
	if st != nil || err != nil {
		return st, err
	}

	opened, err := OpenStore(dir, s.logger)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	if derr := s.disabledLocked(); derr != nil {
		s.mu.Unlock()
		s.closeStore(opened)
		return nil, derr
	}
	if s.settings.DataDir != dir {
		s.mu.Unlock()
		s.closeStore(opened)
		return nil, fmt.Errorf("%w: data directory changed while opening the index", ErrDisabled)
	}
	if s.store != nil {
		existing := s.store
		s.mu.Unlock()
		s.closeStore(opened)
		return existing, nil
	}
	s.store = opened
	s.storeDir = dir
	s.mu.Unlock()
	return opened, nil
}

func (s *Service) closeStore(st *Store) {
	if err := st.Close(); err != nil {
		s.logger.Warn("retro games could not close the index", "error", err)
	}
}

// StartSync starts a manual sync of every source. It ignores the automatic
// retry hold-off and accepts a source that shrank (the user asked for it).
func (s *Service) StartSync() error {
	return s.startSync(Sources(), true)
}

func (s *Service) startSync(srcs []Source, manual bool) error {
	// The store is opened before the single-flight section so the slow open
	// never runs under s.mu.
	st, err := s.openStore()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Re-check: the service may have been disabled, shut down or moved to
	// another data directory while the store was opening. Shutdown sets
	// closed under s.mu before it waits, so wg.Add below never races Wait.
	if err := s.disabledLocked(); err != nil {
		return err
	}
	if s.store != st {
		return fmt.Errorf("%w: index changed while starting the sync", ErrDisabled)
	}
	if s.run != nil {
		return ErrSyncRunning
	}
	s.freshUntil = time.Time{}
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
	now := s.now()
	s.mu.Lock()
	busy := s.run != nil
	held := !s.lastAutoFailure.IsZero() && now.Sub(s.lastAutoFailure) < autoRetryHoldOff
	fresh := !s.freshUntil.IsZero() && now.Before(s.freshUntil)
	s.mu.Unlock()
	if busy || held || fresh {
		return
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		// A canceled request or a service that was switched off meanwhile is
		// not worth a warning.
		if ctx.Err() == nil && !errors.Is(s.storeErr(err), ErrDisabled) {
			s.logger.Warn("retro games could not check index freshness", "error", err)
		}
		return
	}
	var stale []Source
	var oldest time.Time
	for _, src := range Sources() {
		state, ok := states[src.Key]
		if !ok || state.LastSuccessAt.IsZero() || now.Sub(state.LastSuccessAt) > staleAfter {
			stale = append(stale, src)
			continue
		}
		if oldest.IsZero() || state.LastSuccessAt.Before(oldest) {
			oldest = state.LastSuccessAt
		}
	}
	if len(stale) == 0 {
		// Nothing can turn stale before the oldest source ages out.
		s.mu.Lock()
		if s.store == st {
			s.freshUntil = oldest.Add(staleAfter)
		}
		s.mu.Unlock()
		return
	}
	if err := s.startSync(stale, false); err != nil && !errors.Is(err, ErrSyncRunning) && !errors.Is(err, ErrDisabled) {
		s.logger.Warn("retro games could not start the index sync", "error", err)
	}
}

// WaitIdle blocks until no sync runs and no teardown started by Configure is
// pending, or until ctx ends.
func (s *Service) WaitIdle(ctx context.Context) error {
	for {
		s.mu.Lock()
		var pending chan struct{}
		if s.run != nil {
			pending = s.run.done
		} else {
			for tracked := range s.teardowns {
				pending = tracked
				break
			}
		}
		s.mu.Unlock()
		if pending == nil {
			return nil
		}
		select {
		case <-pending:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Shutdown cancels a running sync and closes the store. The service is closed
// for good: every later call is refused with ErrDisabled and nothing is opened.
func (s *Service) Shutdown(ctx context.Context) error {
	// closed is set before waiting, so no sync can start (and wg.Add) once
	// wg.Wait is running.
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
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
	disabled := Status{Platforms: []PlatformStatus{}, Systems: []CountEntry{}}
	st, err := s.openStore()
	if errors.Is(err, ErrDisabled) {
		return disabled, nil
	}
	if err != nil {
		return Status{}, err
	}
	// Switched off while the request ran: report the disabled state, as above.
	fail := func(err error) (Status, error) {
		if err = s.storeErr(err); errors.Is(err, ErrDisabled) {
			return disabled, nil
		}
		return Status{}, err
	}
	counts, err := st.PlatformCounts(ctx)
	if err != nil {
		return fail(err)
	}
	systems, err := st.SystemCounts(ctx, PlatformConsole)
	if err != nil {
		return fail(err)
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		return fail(err)
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
	res, err := st.Search(ctx, q)
	if err != nil {
		return SearchResult{}, s.storeErr(err)
	}
	return res, nil
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
		return Game{}, s.storeErr(err)
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
		return Library{}, s.storeErr(err)
	}
	states, err := st.SyncStates(ctx)
	if err != nil {
		return Library{}, s.storeErr(err)
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

// LibraryKind selects the library list a change touches.
type LibraryKind int

// The library lists PrepareLibraryChange can change.
const (
	LibraryFavorite LibraryKind = iota
	LibraryBroken
	LibraryPlay
)

// SetFavorite adds or removes a favorite.
func (s *Service) SetFavorite(ctx context.Context, id string, on bool) error {
	return s.changeLibrary(ctx, LibraryFavorite, id, on)
}

// SetBroken adds or removes a "doesn't start" marker.
func (s *Service) SetBroken(ctx context.Context, id string, on bool) error {
	return s.changeLibrary(ctx, LibraryBroken, id, on)
}

// RecordPlay counts one start of a game.
func (s *Service) RecordPlay(ctx context.Context, id string) error {
	return s.changeLibrary(ctx, LibraryPlay, id, true)
}

func (s *Service) changeLibrary(ctx context.Context, kind LibraryKind, id string, on bool) error {
	commit, release, err := s.PrepareLibraryChange(ctx, kind, id, on)
	if err != nil {
		return err
	}
	defer release()
	return commit(ctx)
}

// errLibraryChangeDone refuses a second commit, or a commit after release.
var errLibraryChangeDone = errors.New("retrogames: library change already committed or released")

// PrepareLibraryChange does everything for a library change except the write:
// it validates the identifier, opens the index, resolves the game (index, Hall
// of Fame, then the library snapshot of a game that left the index) and then
// takes the store's write lock, waiting for a running sync transaction if
// needed. A ctx that ends while it waits returns ctx.Err(). Nothing is written
// until commit runs.
//
// On success the caller holds the write lock and must call release (defer it):
// release is idempotent and frees the lock unless commit already did. commit
// performs the single store write while holding the lock, so it never waits
// for a sync and a caller can keep a short critical section around it; it
// frees the lock when it returns and works only once. On error commit and
// release are nil and no lock is held. on is ignored for LibraryPlay (a play
// always counts).
func (s *Service) PrepareLibraryChange(ctx context.Context, kind LibraryKind, id string, on bool) (commit func(context.Context) error, release func(), err error) {
	switch kind {
	case LibraryFavorite, LibraryBroken:
	case LibraryPlay:
		on = true
	default:
		return nil, nil, fmt.Errorf("retrogames: unknown library kind %d", kind)
	}
	id = strings.TrimSpace(id)
	if !ValidIdentifier(id) {
		return nil, nil, ErrInvalidIdentifier
	}
	st, err := s.openStore()
	if err != nil {
		return nil, nil, err
	}
	g := Game{Identifier: id}
	if on {
		g, err = s.Game(ctx, id)
		if errors.Is(err, ErrUnknownGame) {
			snap, ok, snapErr := st.Snapshot(ctx, id)
			if snapErr != nil {
				return nil, nil, s.storeErr(snapErr)
			}
			if !ok {
				return nil, nil, ErrUnknownGame
			}
			g, err = snap, nil
		}
		if err != nil {
			return nil, nil, err
		}
	}
	unlock, err := st.lockWrites(ctx)
	if err != nil {
		return nil, nil, err
	}
	// The service may have been switched off, shut down or moved to another
	// data directory while this waited; the detached store is (being) closed.
	s.mu.Lock()
	derr := s.disabledLocked()
	if derr == nil && s.store != st {
		derr = fmt.Errorf("%w: index changed while preparing the change", ErrDisabled)
	}
	s.mu.Unlock()
	if derr != nil {
		unlock()
		return nil, nil, derr
	}
	var done atomic.Bool
	release = func() {
		if done.CompareAndSwap(false, true) {
			unlock()
		}
	}
	commit = func(ctx context.Context) error {
		if !done.CompareAndSwap(false, true) {
			return errLibraryChangeDone
		}
		defer unlock()
		var err error
		switch kind {
		case LibraryFavorite:
			err = st.setFavoriteLocked(ctx, g, on, s.now())
		case LibraryBroken:
			err = st.setBrokenLocked(ctx, g, on, s.now())
		default:
			err = st.recordPlayLocked(ctx, g, s.now())
		}
		return s.storeErr(err)
	}
	return commit, release, nil
}
