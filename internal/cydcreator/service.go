package cydcreator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Write origins (overview "Locking rule"). Every write method refuses with
// CYD_PROJECT_LOCKED while cc_projects.locked_by_job is set, unless the origin
// is JobOrigin of that job.
const (
	OriginUser      = "user"
	OriginRestore   = "restore"
	OriginImport    = "import"
	originJobPrefix = "job:"
)

// JobOrigin is the origin of writes made by agent job jobID.
func JobOrigin(jobID string) string { return originJobPrefix + jobID }

func validOrigin(origin string) bool {
	switch origin {
	case OriginUser, OriginRestore, OriginImport:
		return true
	}
	id, ok := strings.CutPrefix(origin, originJobPrefix)
	return ok && validElementID(id)
}

// ServiceLimits are the configurable limits (config section cyd_creator).
// UpdateLimits replaces them at runtime; zero durations and counts fall back
// to DefaultServiceLimits.
type ServiceLimits struct {
	BuildTimeout         time.Duration // warm cache (build_timeout_seconds)
	ColdBuildTimeout     time.Duration // first build of a (project, board) (cold_build_timeout_seconds)
	KeepBuildsPerProject int           // finished builds kept per project
	MaxRepairAttempts    int           // plan 3
	JobTimeout           time.Duration // plan 3
	Model                string        // plan 3: provider ID, "" = main provider
	OTA                  OTALimits     // plan 5 and Capabilities
}

// OTALimits mirrors cyd_creator.ota.
type OTALimits struct {
	Enabled          bool
	HeartbeatSeconds int
	PublicURL        string
}

// DefaultServiceLimits returns the spec §15 defaults.
func DefaultServiceLimits() ServiceLimits {
	return ServiceLimits{
		BuildTimeout:         300 * time.Second,
		ColdBuildTimeout:     600 * time.Second,
		KeepBuildsPerProject: 5,
		MaxRepairAttempts:    3,
		JobTimeout:           20 * time.Minute,
		OTA:                  OTALimits{Enabled: true, HeartbeatSeconds: 60},
	}
}

// withDefaults fills zero durations and counts; an all-zero value (no
// configuration at all) becomes DefaultServiceLimits.
func (l ServiceLimits) withDefaults() ServiceLimits {
	d := DefaultServiceLimits()
	if l == (ServiceLimits{}) {
		return d
	}
	if l.BuildTimeout <= 0 {
		l.BuildTimeout = d.BuildTimeout
	}
	if l.ColdBuildTimeout <= 0 {
		l.ColdBuildTimeout = d.ColdBuildTimeout
	}
	if l.ColdBuildTimeout < l.BuildTimeout {
		l.ColdBuildTimeout = l.BuildTimeout
	}
	if l.KeepBuildsPerProject <= 0 {
		l.KeepBuildsPerProject = d.KeepBuildsPerProject
	}
	if l.MaxRepairAttempts < 0 {
		l.MaxRepairAttempts = 0
	}
	if l.JobTimeout <= 0 {
		l.JobTimeout = d.JobTimeout
	}
	if l.OTA.HeartbeatSeconds <= 0 {
		l.OTA.HeartbeatSeconds = d.OTA.HeartbeatSeconds
	}
	return l
}

// ServiceConfig wires a Service. WorkspaceDir is the virtual desktop
// workspace (projects live in <WorkspaceDir>/CYD/<slug>/), DataDir the AuraGo
// data directory (artefacts in <DataDir>/cyd_creator/artifacts/), DBPath the
// SQLite file (sqlite.cyd_creator_path).
type ServiceConfig struct {
	WorkspaceDir, DataDir, DBPath string
	Builder                       Builder
	Generate                      func(GenerateInput) (GenerateOutput, error) // nil → Generate
	IDs                           IDSource                                    // nil → RandomIDs{}
	Now                           func() time.Time
	Limits                        ServiceLimits
	Logger                        *slog.Logger
	// Scrub redacts registered secrets from agent text shown in the job chat
	// (the server passes security.Scrub; this package cannot import it,
	// because config's tests import cydcreator). nil leaves text unchanged.
	Scrub func(string) string
}

// Service is the CYD Creator core: projects, files, assets, events and the
// build queue. Plans 3 and 5 add jobs, revisions and devices to it.
type Service struct {
	db           *sql.DB
	builder      Builder
	generate     func(GenerateInput) (GenerateOutput, error)
	now          func() time.Time
	logger       *slog.Logger
	scrub        func(string) string // ServiceConfig.Scrub; nil = unchanged
	ids          IDSource
	workspaceDir string
	dataDir      string
	projectsDir  string
	artifactsDir string

	limitsMu sync.RWMutex
	limits   ServiceLimits

	createMu     sync.Mutex // slug allocation
	locksMu      sync.Mutex
	projectLocks map[string]*sync.Mutex

	fenceMu sync.Mutex        // leaf lock
	fences  map[string]string // project id → receipt id of an unresolved publication (staging_publish.go)

	eventsMu    sync.Mutex     // serializes persisted event order and fan-out
	emitCounts  map[string]int // persisted emits per project since its last prune (eventsMu)
	subsMu      sync.Mutex
	subscribers map[string]map[*eventSubscriber]struct{}
	subsClosed  bool // set by Close: Subscribe then hands out a closed channel

	jobs jobRuntime // agent jobs (plan 3)

	buildsMu sync.Mutex
	queue    []*queuedBuild
	running  *runningBuild
	waiters  map[string]chan struct{}
	wake     chan struct{}
	pulling  atomic.Bool

	deviceLimits deviceLimiter // device endpoint rate limits (plan 5, ota.go)

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

// queuedBuild is a build waiting for the worker with its generated tree.
type queuedBuild struct {
	build    Build
	tree     map[string][]byte
	env      string
	cacheKey string
}

// runningBuild is the build the worker executes.
type runningBuild struct {
	queued *queuedBuild
	ctx    context.Context
	cancel context.CancelCauseFunc
	log    *buildLog
}

// lockedIDs makes any IDSource safe for concurrent use (SeqIDs is not).
type lockedIDs struct {
	mu  sync.Mutex
	ids IDSource
}

func (l *lockedIDs) NewID(prefix string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ids.NewID(prefix)
}

// NewService opens the database (migrating it), marks builds and jobs of a
// previous process as interrupted, discards their partial artefacts, recovers
// interrupted publications, drafts and working copies (recoverJobs; a project
// it cannot recover is fenced, the start goes on) and starts the build worker.
func NewService(cfg ServiceConfig) (*Service, error) {
	if strings.TrimSpace(cfg.WorkspaceDir) == "" || strings.TrimSpace(cfg.DataDir) == "" || strings.TrimSpace(cfg.DBPath) == "" {
		return nil, errors.New("cyd creator workspace, data directory and database path are required")
	}
	ids := cfg.IDs
	if ids == nil {
		ids = RandomIDs{}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	generate := cfg.Generate
	if generate == nil {
		generate = Generate
	}
	s := &Service{
		builder:      cfg.Builder,
		generate:     generate,
		now:          now,
		logger:       logger,
		scrub:        cfg.Scrub,
		ids:          &lockedIDs{ids: ids},
		workspaceDir: cfg.WorkspaceDir,
		dataDir:      cfg.DataDir,
		projectsDir:  filepath.Join(cfg.WorkspaceDir, "CYD"),
		artifactsDir: filepath.Join(cfg.DataDir, "cyd_creator", "artifacts"),
		limits:       cfg.Limits.withDefaults(),
		projectLocks: map[string]*sync.Mutex{},
		emitCounts:   map[string]int{},
		subscribers:  map[string]map[*eventSubscriber]struct{}{},
		waiters:      map[string]chan struct{}{},
		wake:         make(chan struct{}, 1),
	}
	for _, dir := range []string{s.projectsDir, s.artifactsDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create cyd creator directory: %w", err)
		}
	}
	db, err := openStore(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	s.db = db
	interruptedJobs, err := unfinishedJobIDs(context.Background(), db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	recovered, err := recoverStore(context.Background(), db, formatTime(now()))
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.discardPartialArtifacts(recovered)
	for _, b := range recovered {
		build, err := s.loadBuild(context.Background(), b.ID)
		if err != nil {
			logger.Warn("CYD Creator interrupted build could not be read; no build_status event sent",
				"build_id", b.ID, "project_id", b.ProjectID, "error", err)
			continue
		}
		s.Emit(b.ProjectID, EventBuildStatus, b.JobID, b.ID, map[string]any{"build": build})
	}
	s.recoverJobs(context.Background(), interruptedJobs)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.wg.Add(1)
	go s.buildWorker()
	return s, nil
}

// serviceCloseWait bounds how long Close waits for the build worker.
var serviceCloseWait = 30 * time.Second

// Close stops the build worker (a running build ends as interrupted), waits
// for it at most serviceCloseWait and closes the database. The Builder is
// owned and closed by the caller.
func (s *Service) Close() error {
	s.closeOnce.Do(func() {
		s.jobs.mu.Lock()
		s.jobs.closing = true // no new job goroutine joins s.wg after this point
		s.jobs.mu.Unlock()
		s.cancel()
		done := make(chan struct{})
		go func() { s.wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(serviceCloseWait):
			s.logger.Warn("CYD Creator build worker did not stop in time")
		}
		s.dropAllSubscribers()
		s.closeErr = s.db.Close()
	})
	return s.closeErr
}

// UpdateLimits replaces the limits (live config reload).
func (s *Service) UpdateLimits(l ServiceLimits) {
	s.limitsMu.Lock()
	s.limits = l.withDefaults()
	s.limitsMu.Unlock()
}

// Limits returns the current limits.
func (s *Service) Limits() ServiceLimits {
	s.limitsMu.RLock()
	defer s.limitsMu.RUnlock()
	return s.limits
}

var defaultService atomic.Pointer[Service]

// SetDefaultService publishes the service for the agent dispatcher (pattern
// gamemaker.SetDefaultService); nil withdraws it.
func SetDefaultService(s *Service) { defaultService.Store(s) }

// DefaultService returns the published service or nil.
func DefaultService() *Service { return defaultService.Load() }

// lockProject serializes writes (and consistent reads) of one project.
func (s *Service) lockProject(id string) func() {
	s.locksMu.Lock()
	m := s.projectLocks[id]
	if m == nil {
		m = &sync.Mutex{}
		s.projectLocks[id] = m
	}
	s.locksMu.Unlock()
	m.Lock()
	return m.Unlock
}

// checkLock applies the locking rule to a write with origin. A project with
// an unresolved publication (row.PublishReview) refuses every write.
func checkLock(row projectRow, origin string) error {
	if !validOrigin(origin) {
		return NewError(CodeBadRequest, fmt.Sprintf("invalid write origin %q", origin), nil)
	}
	if row.PublishReview != "" {
		return publicationReviewError()
	}
	if row.LockedByJob == "" || origin == JobOrigin(row.LockedByJob) {
		return nil
	}
	return NewError(CodeProjectLocked, "an agent job is working on this project", map[string]any{"job_id": row.LockedByJob})
}

// loadProjectRow returns the row (with the publication fence in
// PublishReview) or *Error CYD_NOT_FOUND.
func (s *Service) loadProjectRow(ctx context.Context, id string) (projectRow, error) {
	row, err := scanProjectRow(s.db.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM cc_projects WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return projectRow{}, NewError(CodeNotFound, "project not found", map[string]any{"project_id": id})
	}
	if err != nil {
		return projectRow{}, fmt.Errorf("read cyd creator project: %w", err)
	}
	row.PublishReview = s.publicationFence(row.ID)
	return row, nil
}

// loadBuild returns the build or *Error CYD_NOT_FOUND.
func (s *Service) loadBuild(ctx context.Context, id string) (Build, error) {
	b, err := scanBuild(s.db.QueryRowContext(ctx, `SELECT `+buildColumns+` FROM cc_builds WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Build{}, NewError(CodeNotFound, "build not found", map[string]any{"build_id": id})
	}
	if err != nil {
		return Build{}, fmt.Errorf("read cyd creator build: %w", err)
	}
	return b, nil
}

func (s *Service) projectDir(slug string) string { return filepath.Join(s.projectsDir, slug) }

// openRoot opens the project directory (dir == "") or another tree such as a
// job's staging copy as an os.Root, so every file access stays inside it.
func (s *Service) openRoot(row projectRow, dir string) (*os.Root, error) {
	if dir == "" {
		dir = s.projectDir(row.Slug)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open cyd creator project directory: %w", err)
	}
	return root, nil
}

// internalError hides the cause of an unexpected failure behind CYD_INTERNAL
// and logs it.
func (s *Service) internalError(op string, err error) error {
	if err == nil {
		return nil
	}
	var typed *Error
	if errors.As(err, &typed) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	s.logger.Warn("CYD Creator operation failed", "op", op, "error", err)
	return NewError(CodeInternal, op+" failed", nil)
}
