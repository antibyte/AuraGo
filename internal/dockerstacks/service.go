package dockerstacks

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// Hosts is satisfied by *dockerhost.Registry.
type Hosts interface {
	Host(ctx context.Context, id string) (dockerhost.Host, error)
	Client(ctx context.Context, id string) (*dockerutil.Client, error)
}

// Options configure a Service. Zero values take the defaults.
type Options struct {
	Logger       *slog.Logger
	Now          func() time.Time // default time.Now
	HealthWait   time.Duration    // default 120 s
	HealthPoll   time.Duration    // default 2 s
	MaxDuration  time.Duration    // default 60 min
	ListCacheTTL time.Duration    // default 30 s
	LocalHostID  string           // default dockerhost.LocalHostID; source for copying the runner image
}

func (o Options) withDefaults() Options {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.HealthWait <= 0 {
		o.HealthWait = 120 * time.Second
	}
	if o.HealthPoll <= 0 {
		o.HealthPoll = 2 * time.Second
	}
	if o.MaxDuration <= 0 {
		o.MaxDuration = 60 * time.Minute
	}
	if o.ListCacheTTL <= 0 {
		o.ListCacheTTL = 30 * time.Second
	}
	if o.LocalHostID == "" {
		o.LocalHostID = dockerhost.LocalHostID
	}
	return o
}

// lockKey identifies what one operation or one file write owns: a stack on a
// host, or the host itself (stack "") for prepare.
type lockKey struct{ host, stack string }

// Service is the stacks service of the Docker Manager. It is safe for
// concurrent use.
type Service struct {
	db      *sql.DB
	journal *journal
	hosts   Hosts
	opts    Options
	log     *slog.Logger

	// baseCtx bounds every operation goroutine; Close cancels it. An operation
	// stopped that way stays in the journal as it is and is reattached by the
	// next Open.
	baseCtx context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	mu        sync.Mutex
	closed    bool
	runs      map[string]*opRun       // active operations by id
	locks     map[lockKey]string      // (host, stack) -> id of the active operation
	scans     map[string]*scanCache   // folder scan per host
	ready     map[string]readyHost    // hosts last seen as prepared
	hostLocks map[string]*sync.Mutex  // serialize preparing a host
	fileLocks map[lockKey]*sync.Mutex // serialize read-compare-write of a stack's files
	swept     map[string]bool         // hosts whose leftover helper containers were cleaned up
	parked    map[string]*opRun       // adopted operations waiting for their host to answer, by id

	// rawLine, when set by a test, sees every raw line of a runner's output.
	rawLine func(operationID, line string)
}

// readyHost remembers that a host passed the helper check, and with which image.
type readyHost struct {
	at    time.Time
	image string
}

// scanFolder is what the lister found in one folder of the stacks directory.
type scanFolder struct {
	composeFile string // the first of ComposeFileNames present; "" when the folder has only a .env
	hasEnv      bool
}

// scanCache is the result of one lister run.
type scanCache struct {
	at      time.Time
	folders map[string]scanFolder // by folder name; valid stack names only
	ignored []IgnoredFolder
}

// cachedScan returns the host's folder scan when it is younger than ListCacheTTL.
func (s *Service) cachedScan(hostID string) (*scanCache, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.scans[hostID]
	if !ok || s.opts.Now().Sub(c.at) >= s.opts.ListCacheTTL {
		return nil, false
	}
	return c, true
}

func (s *Service) storeScan(hostID string, c *scanCache) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scans[hostID] = c
}

// invalidateScan drops the cached folder scan of a host (our own writes and
// operations change the folders).
func (s *Service) invalidateScan(hostID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.scans, hostID)
}

// readyImage returns the helper image of a host that passed the helper check
// less than ListCacheTTL ago.
func (s *Service) readyImage(hostID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.ready[hostID]
	if !ok || s.opts.Now().Sub(r.at) >= s.opts.ListCacheTTL {
		return "", false
	}
	return r.image, true
}

func (s *Service) markReady(hostID, image string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready[hostID] = readyHost{at: s.opts.Now(), image: image}
}

// Open creates the journal table in db (the shared docker_manager.db handle,
// see dockerhost.Store.DB) and returns a service.
func Open(db *sql.DB, hosts Hosts, opts Options) (*Service, error) {
	if db == nil {
		return nil, errors.New("dockerstacks: a database is required")
	}
	if hosts == nil {
		return nil, errors.New("dockerstacks: a host registry is required")
	}
	opts = opts.withDefaults()
	j, err := newJournal(db)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{
		db: db, journal: j, hosts: hosts, opts: opts, log: opts.Logger,
		baseCtx: ctx, cancel: cancel,
		runs:      map[string]*opRun{},
		locks:     map[lockKey]string{},
		scans:     map[string]*scanCache{},
		ready:     map[string]readyHost{},
		hostLocks: map[string]*sync.Mutex{},
		fileLocks: map[lockKey]*sync.Mutex{},
		swept:     map[string]bool{},
		parked:    map[string]*opRun{},
	}
	return s, nil
}

// Close stops the followers. Runner containers keep going; the next Open
// reattaches to them. Close returns when the service's goroutines have ended.
func (s *Service) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.cancel()
	s.wg.Wait()
}

func (s *Service) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// target resolves a host and its Engine client.
func (s *Service) target(ctx context.Context, hostID string) (dockerhost.Host, engine, error) {
	h, err := s.hosts.Host(ctx, hostID)
	if err != nil {
		return dockerhost.Host{}, engine{}, err
	}
	c, err := s.hosts.Client(ctx, hostID)
	if err != nil {
		return dockerhost.Host{}, engine{}, err
	}
	return h, newEngine(c), nil
}

// newOperationID returns "op_" plus 16 lowercase hex characters.
func newOperationID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate operation id: %w", err)
	}
	return "op_" + hex.EncodeToString(b[:]), nil
}

// newShortID returns 8 lowercase hex characters (lister container names).
func newShortID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// hostLock returns the mutex that serializes preparing one host.
func (s *Service) hostLock(hostID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.hostLocks[hostID]
	if !ok {
		m = &sync.Mutex{}
		s.hostLocks[hostID] = m
	}
	return m
}

// fileLock returns the mutex that serializes the file writes of one stack.
func (s *Service) fileLock(hostID, stack string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := lockKey{hostID, stack}
	m, ok := s.fileLocks[key]
	if !ok {
		m = &sync.Mutex{}
		s.fileLocks[key] = m
	}
	return m
}

// activeRun returns the operation running on a stack, if any.
func (s *Service) activeRun(hostID, stack string) *opRun {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.locks[lockKey{hostID, stack}]; ok {
		return s.runs[id]
	}
	return nil
}

// refuseWhileDeleting returns a BusyError when a delete is running on the stack.
func (s *Service) refuseWhileDeleting(hostID, stack string) error {
	if run := s.activeRun(hostID, stack); run != nil {
		if op := run.snapshot(); op.Kind == KindDelete {
			return &BusyError{OperationID: op.ID, Kind: op.Kind}
		}
	}
	return nil
}
