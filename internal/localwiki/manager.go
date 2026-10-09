package localwiki

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
)

// operation is the one install, resume or update that may run at a time.
type operation struct {
	cancel    context.CancelFunc
	done      chan struct{}
	phase     string
	target    Edition
	bytesDone int64
	required  int64
	meter     rateMeter
	rate      int64
	eta       int64
}

type catalogCacheEntry struct {
	info    CatalogInfo
	fetched time.Time
}

// Manager owns the installed Local Wikipedia edition: catalog lookups, the one
// download that may run, verification, publication, the daily update check and
// the refcounted Library used by the agent tool and the desktop app.
type Manager struct {
	logger       *slog.Logger
	client       *http.Client
	freeDisk     func(string) (int64, error)
	now          func() time.Time
	catalogBase  *url.URL
	sensitive    func(string) bool
	diskInterval int64
	stallTimeout time.Duration
	firstCheck   time.Duration

	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	wg              sync.WaitGroup
	reload          chan struct{}
	loadMu          sync.Mutex // serializes load
	stateMu         sync.Mutex // orders state.json writes; never taken while holding mu

	mu           sync.Mutex
	settings     Settings
	started      bool
	shuttingDown bool
	deleting     bool       // Delete is running; no operation may start meanwhile
	activeDir    string     // storage directory the loaded state belongs to
	state        *stateFile // nil when nothing is installed or pending deletion
	lib          *libraryRef
	op           *operation
	interrupted  bool
	errCode      string
	errRequired  int64
	update       *UpdateInfo
	catalogCache map[string]catalogCacheEntry
}

// NewManager creates a passive manager: no I/O and no goroutine until Start.
//
// The catalog URL is the one trusted host of the default HTTP client and of
// the metalink and download.json mirror checks: it may be a local address
// (a LAN Kiwix mirror, the fake Kiwix of the tests), every other host may not.
func NewManager(deps Deps) *Manager {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	freeDisk := deps.FreeDiskBytes
	if freeDisk == nil {
		freeDisk = fileutil.FreeDiskBytes
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	base := strings.TrimSpace(deps.CatalogBaseURL)
	if base == "" {
		base = defaultCatalogBaseURL
	}
	catalogBase, err := requireHTTPS(base)
	if err != nil {
		logger.Warn("[LocalWikipedia] Invalid catalog URL; using the Kiwix default", "error", err)
		catalogBase, _ = url.Parse(defaultCatalogBaseURL)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		logger:          logger,
		client:          newHTTPClient(deps.HTTPClient, catalogBase),
		freeDisk:        freeDisk,
		now:             now,
		catalogBase:     catalogBase,
		sensitive:       deps.IsSensitivePath,
		diskInterval:    defaultDiskCheckInterval,
		stallTimeout:    defaultStallTimeout,
		firstCheck:      2 * time.Minute,
		lifecycleCtx:    ctx,
		lifecycleCancel: cancel,
		reload:          make(chan struct{}, 1),
		settings:        Settings{Variant: VariantNoPic},
		catalogCache:    map[string]catalogCacheEntry{},
	}
}

// Configure publishes new settings. It never starts a download; a changed
// storage directory is loaded by the background loop once no download runs.
func (m *Manager) Configure(s Settings) {
	if s.Variant != VariantMaxi {
		s.Variant = VariantNoPic
	}
	if dir := strings.TrimSpace(s.DataDir); dir != "" {
		s.DataDir = filepath.Clean(dir)
	} else {
		s.DataDir = ""
	}
	m.mu.Lock()
	changedDir := m.started && s.DataDir != m.activeDir
	m.settings = s
	m.mu.Unlock()
	if changedDir {
		m.signalReload()
	}
}

// Settings returns the settings the manager currently acts on.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// Start loads the installed edition and starts the background loop (storage
// directory changes, daily update check). It never resumes a download.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started || m.shuttingDown {
		m.mu.Unlock()
		return
	}
	m.started = true
	dir := m.settings.DataDir
	m.mu.Unlock()
	if ctx != nil {
		context.AfterFunc(ctx, m.lifecycleCancel)
	}
	m.load(dir)
	m.wg.Add(1)
	go m.loop()
}

// Shutdown cancels a running download (its part file is kept for "Resume"),
// stops the background loop and closes the library once its readers are done.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.shuttingDown = true
	op := m.op
	m.mu.Unlock()
	m.lifecycleCancel()
	if op != nil {
		op.cancel()
	}
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	m.mu.Lock()
	ref := m.lib
	m.lib = nil
	m.mu.Unlock()
	if ref != nil {
		ref.retire(nil)
	}
	return nil
}

func (m *Manager) loop() {
	defer m.wg.Done()
	timer := time.NewTimer(m.firstCheck)
	defer timer.Stop()
	for {
		select {
		case <-m.lifecycleCtx.Done():
			return
		case <-m.reload:
			m.reloadIfIdle()
		case <-timer.C:
			m.retryPendingDeletes()
			m.maybeCheckUpdate(m.lifecycleCtx)
			timer.Reset(loopTick)
		}
	}
}

// retryPendingDeletes deletes retired edition files that could not be removed
// earlier (on Windows a file that something still holds open).
func (m *Manager) retryPendingDeletes() {
	m.mu.Lock()
	dir := m.activeDir
	m.mu.Unlock()
	m.processPendingDeletes(dir)
}

func (m *Manager) signalReload() {
	select {
	case m.reload <- struct{}{}:
	default:
	}
}

func (m *Manager) reloadIfIdle() {
	m.mu.Lock()
	dir := m.settings.DataDir
	idle := m.op == nil && !m.shuttingDown && dir != m.activeDir
	m.mu.Unlock()
	if idle {
		m.load(dir)
	}
}

// load reads dir's state.json and download.json, opens the installed edition
// and replaces the current one. It never touches the network.
func (m *Manager) load(dir string) {
	m.loadMu.Lock()
	defer m.loadMu.Unlock()
	var (
		st          *stateFile
		ref         *libraryRef
		code        string
		interrupted bool
	)
	if filepath.IsAbs(dir) {
		var err error
		if st, err = readState(dir); err != nil {
			m.logger.Warn("[LocalWikipedia] state.json is unreadable", "dir", dir, "error", err)
			code = CodeZIMUnreadable
		}
		if st != nil && st.Edition != nil {
			lib, err := OpenLibrary(filepath.Join(dir, st.Edition.FileName), *st.Edition)
			if err != nil {
				m.logger.Warn("[LocalWikipedia] The installed edition cannot be opened", "file", st.Edition.FileName, "error", err)
				code = CodeZIMUnreadable
			} else {
				ref = newLibraryRef(lib)
				if lib.fulltextNote != "" {
					m.logger.Warn("[LocalWikipedia] Full-text search unavailable; title search only", "reason", lib.fulltextNote)
				}
			}
		}
		pending, err := readDownload(dir, m.catalogBase)
		switch {
		case err != nil:
			m.logger.Warn("[LocalWikipedia] download.json is unreadable; the interrupted download cannot be resumed", "dir", dir, "error", err)
		case pending != nil:
			interrupted = true
		}
	}
	m.mu.Lock()
	previous := m.lib
	m.activeDir = dir
	m.state = st
	m.lib = ref
	m.interrupted = interrupted
	m.errCode = code
	m.errRequired = 0
	m.update = nil
	m.mu.Unlock()
	if previous != nil {
		previous.retire(nil)
	}
	m.processPendingDeletes(dir)
}

// saveState writes the in-memory state of dir; state.json is removed when it
// would be empty.
func (m *Manager) saveState(dir string) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.mu.Lock()
	if m.activeDir != dir {
		m.mu.Unlock()
		return nil
	}
	snapshot := cloneState(m.state)
	m.mu.Unlock()
	if snapshot == nil || (snapshot.Edition == nil && len(snapshot.PendingDelete) == 0) {
		return removeIfExists(filepath.Join(dir, stateFileName))
	}
	return writeState(dir, snapshot)
}

// processPendingDeletes removes retired edition files listed in state.json.
// A file that cannot be deleted yet stays listed and is retried later.
func (m *Manager) processPendingDeletes(dir string) {
	m.mu.Lock()
	if m.state == nil || m.activeDir != dir || len(m.state.PendingDelete) == 0 {
		m.mu.Unlock()
		return
	}
	pending := append([]string(nil), m.state.PendingDelete...)
	current := ""
	if m.state.Edition != nil {
		current = m.state.Edition.FileName
	}
	m.mu.Unlock()
	kept := make([]string, 0)
	for _, name := range pending {
		if name == current || !zimFileNamePattern.MatchString(name) {
			continue
		}
		if err := removeIfExists(filepath.Join(dir, name)); err != nil {
			m.logger.Warn("[LocalWikipedia] The previous edition could not be deleted yet", "file", name, "error", err)
			kept = append(kept, name)
		}
	}
	m.mu.Lock()
	if m.state == nil || m.activeDir != dir {
		m.mu.Unlock()
		return
	}
	for _, name := range m.state.PendingDelete {
		if !slices.Contains(pending, name) {
			kept = append(kept, name)
		}
	}
	m.state.PendingDelete = kept
	if m.state.Edition == nil && len(kept) == 0 {
		m.state = nil
	}
	m.mu.Unlock()
	if err := m.saveState(dir); err != nil {
		m.logger.Warn("[LocalWikipedia] state.json could not be updated", "error", err)
	}
}

// Status reports the installed edition, a running download and the selection.
func (m *Manager) Status() Status {
	m.mu.Lock()
	settings := m.settings
	status := Status{
		Selection:           Selection{Language: settings.Language, Variant: settings.Variant},
		DataDir:             settings.DataDir,
		DataDirLocked:       settings.DataDirLocked,
		SystemLanguage:      settings.SystemLanguage,
		OperationInProgress: m.op != nil,
		ErrorCode:           m.errCode,
		FreeBytes:           -1,
	}
	if m.state != nil && m.state.Edition != nil {
		edition := *m.state.Edition
		status.Edition = &edition
		status.SelectionMatchesInstalled = edition.Language == settings.Language && edition.Variant == settings.Variant
	}
	if m.lib != nil {
		status.Fulltext = m.lib.lib.Fulltext()
	}
	if m.update != nil {
		update := *m.update
		status.UpdateAvailable = &update
	}
	if op := m.op; op != nil {
		status.BytesDone = op.bytesDone
		status.BytesTotal = op.target.Size
		status.RateBytesPerSec = op.rate
		status.ETASeconds = op.eta
		status.RequiredBytes = op.required
		if op.target.Size > 0 {
			status.Progress = float64(op.bytesDone) / float64(op.target.Size)
		}
	} else if m.errCode == CodeInsufficientDiskSpace {
		status.RequiredBytes = m.errRequired
	}
	status.State = m.stateLocked()
	m.mu.Unlock()

	if status.ErrorCode == "" && status.State == StateReady && !status.Fulltext {
		status.ErrorCode = CodeFulltextUnsupported
	}
	if status.ErrorCode == "" && !status.OperationInProgress {
		if err := checkDataDirShape(settings.DataDir, m.sensitive); err != nil {
			status.ErrorCode = CodeDataDirInvalid
		}
	}
	status.Recommendation = Recommendation(status.ErrorCode)
	if free, err := m.freeDisk(settings.DataDir); err == nil {
		status.FreeBytes = free
	}
	status.Languages = Languages()
	return status
}

func (m *Manager) stateLocked() string {
	switch {
	case m.op != nil:
		return m.op.phase
	case m.interrupted:
		return StateInterrupted
	case m.errCode != "":
		return StateError
	case m.lib != nil:
		return StateReady
	default:
		return StateNotInstalled
	}
}

// Acquire returns the open edition for one request. release must be called
// exactly once (extra calls are ignored); ok is false when the integration is
// disabled or no readable edition is installed.
func (m *Manager) Acquire() (*Library, func(), bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ref := m.lib
	if !m.settings.Enabled || ref == nil || !ref.acquire() {
		return nil, func() {}, false
	}
	var once sync.Once
	return ref.lib, func() { once.Do(ref.release) }, true
}
