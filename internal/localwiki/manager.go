package localwiki

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
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
	firstLoad       chan struct{} // closed once the loop has loaded the storage directory for the first time
	probeNow        chan struct{}
	diskProbeEvery  time.Duration
	diskProbeLimit  time.Duration
	loadMu          sync.Mutex // serializes loads; request paths only TryLock it
	beforeLoadLock  func()     // test seam: runs right before a load waits for loadMu
	beforeStorageIO func()     // test seam: runs when a load or cleanup starts its storage I/O (storage I/O marked)
	stateMu         sync.Mutex // orders state.json writes; never taken while holding mu

	// mu guards the fields below. It is never held across file system or
	// network I/O: Configure, Status, Acquire and Shutdown take it, and a hung
	// storage directory (an unreachable network share) must not stall them.
	mu           sync.Mutex
	settings     Settings
	started      bool
	shuttingDown bool
	deleting     bool // Delete is running; no operation may start meanwhile
	// ioToken is non-zero while a load or a cleanup reads or changes the
	// storage directory without holding mu (see holdStorageIOLocked). No
	// operation, Delete, load or other cleanup starts meanwhile.
	ioToken      uint64
	ioSeq        uint64     // last token handed out
	activeDir    string     // storage directory the loaded state belongs to
	state        *stateFile // nil when nothing is installed or pending deletion
	lib          *libraryRef
	op           *operation
	interrupted  bool   // a download.json describes a download that can be resumed
	loadCode     string // why the load failed: zim_unreadable (edition) or state_unreadable (state.json); set by loadLocked
	errCode      string // why the last operation stopped (set when it ends)
	errRequired  int64  // bytes the paused download needed (errCode insufficient_disk_space)
	catalogCache map[string]catalogCacheEntry
	disk         diskReading          // last free-space measurement (see probeDisk)
	diskSeq      uint64               // orders free-space measurements
	diskInFlight map[string]time.Time // storage directories being measured, with the start time
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
		firstLoad:       make(chan struct{}),
		probeNow:        make(chan struct{}, 1),
		diskProbeEvery:  diskProbeInterval,
		diskProbeLimit:  diskProbeTimeout,
		diskInFlight:    map[string]time.Time{},
		settings:        Settings{Variant: VariantNoPic},
		catalogCache:    map[string]catalogCacheEntry{},
	}
}

// Configure publishes new settings. It only stores them and signals the
// background loops: it never waits for a load or for storage I/O and never
// starts a download. A changed storage directory is loaded by the background
// loop once no download runs, and its free space is measured in the
// background.
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
	newProbeDir := m.started && s.DataDir != m.settings.DataDir
	m.settings = s
	m.mu.Unlock()
	if changedDir {
		m.signalReload()
	}
	if newProbeDir {
		m.signalProbe()
	}
}

// Settings returns the settings the manager currently acts on.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// Start starts the background loop, which first loads the installed edition
// and then follows storage directory changes and runs the daily update check.
// Start itself does no I/O: a slow or hung storage directory (a hard-mounted
// network share, a large retired edition being deleted) never delays the
// caller. Until the first load has finished, Status reports Loading, Acquire
// returns ok=false and Install and Delete refuse with ErrBusy. It never
// resumes a download.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started || m.shuttingDown {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()
	if ctx != nil {
		context.AfterFunc(ctx, m.lifecycleCancel)
	}
	m.wg.Add(2)
	go m.loop()
	go m.probeLoop()
}

// firstLoadPending reports whether Start ran and the background loop has not
// finished its first load yet.
func (m *Manager) firstLoadPending() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.firstLoadPendingLocked()
}

// firstLoadPendingLocked is firstLoadPending for a caller that holds mu.
func (m *Manager) firstLoadPendingLocked() bool {
	if !m.started {
		return false
	}
	select {
	case <-m.firstLoad:
		return false
	default:
		return true
	}
}

// Shutdown cancels a running download (its part file is kept for "Resume"),
// stops the background loops and closes the library once its readers are done.
//
// It honours ctx even when a load or download is stuck in storage I/O that
// cannot be interrupted (an unreachable network share): it then returns
// ctx.Err(), and the stuck goroutine finishes on its own; the library is
// closed after it. A free-space measurement that hangs is never waited for.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.shuttingDown = true
	op := m.op
	m.mu.Unlock()
	m.lifecycleCancel()
	if op != nil {
		op.cancel()
	}
	finished := make(chan struct{})
	go func() {
		m.wg.Wait()
		m.closeLibrary()
		close(finished)
	}()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// closeLibrary takes the library out of service; it is closed once its readers
// released it.
func (m *Manager) closeLibrary() {
	m.mu.Lock()
	ref := m.lib
	m.lib = nil
	m.mu.Unlock()
	if ref != nil {
		ref.retire(nil)
	}
}

func (m *Manager) loop() {
	defer m.wg.Done()
	m.loadIfStale() // skipped once Shutdown began
	close(m.firstLoad)
	timer := time.NewTimer(m.firstCheck)
	defer timer.Stop()
	for {
		select {
		case <-m.lifecycleCtx.Done():
			return
		case <-m.reload:
			m.loadIfStale()
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

// loadIfStale loads the configured storage directory when it differs from the
// loaded one and nothing else uses the storage directory (no operation, Delete
// or cleanup). The conditions are checked after taking loadMu: a load that
// finished while this call waited must not be repeated, and a download that
// started meanwhile must not be reset. A load held back by a cleanup is
// retried when the cleanup ends (holdStorageIOLocked's release). A call that
// finds nothing to load does not take loadMu at all (loadPending): Delete only
// TryLocks it, and a wake-up after every operation or cleanup must not make it
// answer ErrBusy while it looks.
func (m *Manager) loadIfStale() {
	if !m.loadPending() {
		return
	}
	m.lockLoad()
	defer m.loadMu.Unlock()
	m.loadIfStaleLocked()
}

// tryLoadIfStale is loadIfStale for request paths: it never waits for a load
// that is already running, which may hang on an unreachable share.
func (m *Manager) tryLoadIfStale() {
	if !m.loadPending() {
		return
	}
	if !m.loadMu.TryLock() {
		return
	}
	defer m.loadMu.Unlock()
	m.loadIfStaleLocked()
}

// loadPending reports whether a load looks needed right now. It is only a
// shortcut for the callers above; they check again under loadMu.
func (m *Manager) loadPending() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.staleLocked()
}

// staleLocked reports whether the configured storage directory is not the
// loaded one and nothing else uses the storage directory. The caller holds mu.
func (m *Manager) staleLocked() bool {
	return m.started && !m.shuttingDown && m.op == nil && !m.deleting && m.ioToken == 0 && m.settings.DataDir != m.activeDir
}

// loadIfStaleLocked is loadIfStale for a caller that holds loadMu.
func (m *Manager) loadIfStaleLocked() {
	m.mu.Lock()
	dir := m.settings.DataDir
	stale := m.staleLocked()
	var release func()
	if stale {
		release = m.holdStorageIOLocked()
	}
	m.mu.Unlock()
	if !stale {
		return
	}
	// Released by a defer: a load can run in a request goroutine (Install),
	// whose panic net/http recovers, and a mark left behind would refuse every
	// later load, Install and Delete until a restart.
	defer release()
	m.loadLocked(dir)
}

// holdStorageIOLocked marks the storage directory as used by a load or
// cleanup that runs without holding mu and returns the function that ends the
// mark and lets the loop load a storage directory change it held back. The
// function is meant for a defer, so a panic cannot leave the mark behind; it
// may be called more than once and never ends a later holder's mark. The
// caller holds mu and has checked that ioToken is zero.
func (m *Manager) holdStorageIOLocked() func() {
	m.ioSeq++
	token := m.ioSeq
	m.ioToken = token
	return func() {
		m.mu.Lock()
		released := m.ioToken == token
		if released {
			m.ioToken = 0
		}
		m.mu.Unlock()
		if released {
			m.signalReload()
		}
	}
}

// beginStorageIO marks a cleanup of dir that runs without holding mu and
// returns the release for a defer. It fails, and the caller skips the
// cleanup, unless dir is the loaded storage directory and nothing else uses
// it: no operation, Delete, load or other cleanup. While it is marked,
// operations, Delete and loads refuse to start.
func (m *Manager) beginStorageIO(dir string) (func(), bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.op != nil || m.deleting || m.ioToken != 0 || m.shuttingDown || m.activeDir != dir {
		return nil, false
	}
	return m.holdStorageIOLocked(), true
}

// cleanRestartFiles removes the restart files of dir (see removeRestartFiles);
// they are never resumed. The caller has marked the storage I/O.
func (m *Manager) cleanRestartFiles(dir string) {
	if m.beforeStorageIO != nil {
		m.beforeStorageIO()
	}
	if err := removeRestartFiles(dir); err != nil {
		m.logger.Warn("[LocalWikipedia] A stale restart file could not be removed", "dir", dir, "error", err)
	}
}

func (m *Manager) lockLoad() {
	if m.beforeLoadLock != nil {
		m.beforeLoadLock()
	}
	m.loadMu.Lock()
}

// loadLocked reads dir's state.json and download.json, opens the installed
// edition and replaces the current one. It never touches the network; the only
// repairs it makes on disk are reconcileDownload's and the removal of stale
// restart files. The caller holds loadMu and has marked the storage I/O
// (loadIfStaleLocked releases the mark afterwards); mu is only taken to
// publish the result.
func (m *Manager) loadLocked(dir string) {
	var (
		st          *stateFile
		ref         *libraryRef
		code        string
		interrupted bool
	)
	if filepath.IsAbs(dir) {
		// Restart files are never resumed; a process killed during a
		// restart leaves one behind.
		m.cleanRestartFiles(dir)
		var stateErr error
		if st, stateErr = readState(dir); stateErr != nil {
			// Nothing says which edition is installed: no edition is
			// reported, and Install or Delete replace the file.
			m.logger.Warn("[LocalWikipedia] state.json is unreadable", "dir", dir, "error", stateErr)
			code = CodeStateUnreadable
		}
		// The download is reconciled before the edition is opened: the repair
		// may put the installed edition's file back in place.
		pending, err := readDownload(dir, m.catalogBase)
		switch {
		case err != nil:
			m.logger.Warn("[LocalWikipedia] download.json is unreadable; the interrupted download cannot be resumed", "dir", dir, "error", err)
		case pending != nil && stateErr != nil:
			// Without a readable state nothing says which files are the
			// installed edition's, so the directory is left as it is.
			interrupted = true
		case pending != nil:
			interrupted = m.reconcileDownload(dir, st, pending)
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
	}
	m.mu.Lock()
	if m.op != nil || m.shuttingDown {
		// The manager closed while the directory was read (no operation can
		// start while the storage I/O is marked; the check is defensive).
		m.mu.Unlock()
		if ref != nil {
			ref.retire(nil)
		}
		return
	}
	previous := m.lib
	m.activeDir = dir
	m.state = st
	m.lib = ref
	m.interrupted = interrupted
	m.loadCode = code
	m.errCode = ""
	m.errRequired = 0
	m.mu.Unlock()
	if previous != nil {
		previous.retire(nil)
	}
	m.processPendingDeletes(dir)
}

// reconcileDownload repairs what a crash during publication leaves behind and
// reports whether download.json still describes a download that can be
// resumed. st is the successfully read state.json (nil when there is none);
// it must not be called when state.json could not be read. Publication renames
// <edition>.zim.part to <edition>.zim, writes state.json and then removes
// download.json:
//   - a crash after the state was written leaves a download.json for the
//     installed edition: it and any partial file are dropped (see
//     dropInstalledDownload);
//   - a crash between the rename and the state write leaves the finished
//     <edition>.zim unnamed by state.json: it becomes the partial file again,
//     so Resume only re-hashes it instead of downloading everything.
func (m *Manager) reconcileDownload(dir string, st *stateFile, pending *downloadFile) bool {
	target := pending.Target.FileName
	finalPath := filepath.Join(dir, target)
	partPath := finalPath + ".part"
	if st != nil && st.Edition != nil && st.Edition.FileName == target {
		m.dropInstalledDownload(dir, st.Edition)
		return false
	}
	if st != nil && slices.Contains(st.PendingDelete, target) {
		return true // the file is a retired edition, not a finished download
	}
	if info, err := os.Lstat(finalPath); err == nil && info.Mode().IsRegular() {
		if _, err := os.Lstat(partPath); errors.Is(err, fs.ErrNotExist) {
			if err := fileutil.Rename(finalPath, partPath); err != nil {
				m.logger.Warn("[LocalWikipedia] A finished download could not be set aside for verification", "file", target, "error", err)
			} else {
				m.logger.Info("[LocalWikipedia] Recovered a download that was not published; resuming only re-verifies it", "file", target)
			}
		}
	}
	return true
}

// dropInstalledDownload removes the download.json of the installed edition
// (the crash happened after state.json was written) and the partial files.
// If the installed edition's file is missing but a partial file of exactly its
// size exists, that file is put back: an earlier version of this code, run
// while state.json was unreadable, set the installed file aside as a partial
// download. loadLocked opens the edition afterwards and so verifies it.
func (m *Manager) dropInstalledDownload(dir string, installed *Edition) {
	finalPath := filepath.Join(dir, installed.FileName)
	partPath := finalPath + ".part"
	m.logger.Info("[LocalWikipedia] Dropping the download.json of the installed edition", "edition", installed.FileName)
	if _, err := os.Lstat(finalPath); errors.Is(err, fs.ErrNotExist) {
		if info, err := os.Lstat(partPath); err == nil && info.Mode().IsRegular() && info.Size() == installed.Size {
			if err := fileutil.Rename(partPath, finalPath); err != nil {
				m.logger.Warn("[LocalWikipedia] The installed edition could not be put back in place", "file", installed.FileName, "error", err)
			} else {
				m.logger.Info("[LocalWikipedia] Put the installed edition back in place", "file", installed.FileName)
			}
		}
	}
	if err := removeDownload(dir); err != nil {
		m.logger.Warn("[LocalWikipedia] download.json could not be removed", "error", err)
	}
	if err := removePartialDownload(dir, installed.FileName); err != nil {
		m.logger.Warn("[LocalWikipedia] A stale partial download could not be removed", "file", installed.FileName, "error", err)
	}
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
	var failed []string
	for _, name := range pending {
		if name == current || !zimFileNamePattern.MatchString(name) {
			continue
		}
		if err := m.deleteListed(dir, name); err != nil {
			m.logger.Warn("[LocalWikipedia] The previous edition could not be deleted yet", "file", name, "error", err)
			failed = append(failed, name)
		}
	}
	m.mu.Lock()
	if m.state == nil || m.activeDir != dir {
		m.mu.Unlock()
		return
	}
	m.state.PendingDelete = mergePendingDeletes(m.state.PendingDelete, pending, failed)
	if m.state.Edition == nil && len(m.state.PendingDelete) == 0 {
		m.state = nil
	}
	m.mu.Unlock()
	if err := m.saveState(dir); err != nil {
		m.logger.Warn("[LocalWikipedia] state.json could not be updated", "error", err)
	}
}

// deleteListed deletes a retired edition file, but only while it is still
// listed: a download that was started for the same file name unlists it.
func (m *Manager) deleteListed(dir, name string) error {
	m.mu.Lock()
	listed := m.state != nil && m.activeDir == dir && slices.Contains(m.state.PendingDelete, name)
	m.mu.Unlock()
	if !listed {
		return nil
	}
	return removeIfExists(filepath.Join(dir, name))
}

// mergePendingDeletes returns what remains listed after a pass: the names in
// current (the list as it is now) that the pass did not handle or could not
// delete. Names that were unlisted while the pass ran stay unlisted, and names
// added meanwhile are kept.
func mergePendingDeletes(current, snapshot, failed []string) []string {
	var next []string
	for _, name := range current {
		if !slices.Contains(snapshot, name) || slices.Contains(failed, name) {
			next = append(next, name)
		}
	}
	return next
}

// Status reports the installed edition, a running download and the selection.
// It never touches the storage directory: the free space comes from the
// background probe (probeDisk) and the directory check is lexical.
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
		FreeBytes:           m.freeBytesLocked(settings.DataDir),
	}
	// An operation's code outranks the load error of the installed edition,
	// which is not shown while an operation runs (its progress is).
	fromOperation := m.errCode != ""
	if !fromOperation && m.op == nil {
		status.ErrorCode = m.loadCode
	}
	status.Readable = m.lib != nil
	// A changed storage directory that is being loaded (it may hang on an
	// unreachable share) is reported like the first load; the previous
	// directory's edition stays served meanwhile.
	status.Loading = m.firstLoadPendingLocked() || (m.ioToken != 0 && settings.DataDir != m.activeDir)
	if m.state != nil && m.state.Edition != nil {
		edition := *m.state.Edition
		status.Edition = &edition
		status.SelectionMatchesInstalled = edition.Language == settings.Language && edition.Variant == settings.Variant
		if update := m.state.Update; update != nil {
			status.UpdateAvailable = &UpdateInfo{Date: update.Date, Size: update.Size}
		}
	}
	if m.lib != nil {
		status.Fulltext = m.lib.lib.Fulltext()
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

	if status.ErrorCode == "" && status.Loading {
		status.ErrorCode = CodeBusy
	}
	if status.ErrorCode == "" && status.State == StateReady && !status.Fulltext {
		status.ErrorCode = CodeFulltextUnsupported
	}
	if status.ErrorCode == "" && !status.OperationInProgress {
		if err := checkDataDirShape(settings.DataDir, m.sensitive); err != nil {
			status.ErrorCode = CodeDataDirInvalid
		}
	}
	status.Recommendation = Recommendation(status.ErrorCode)
	if fromOperation {
		status.Recommendation = operationRecommendation(status.ErrorCode)
	}
	status.Languages = Languages()
	return status
}

// stateLocked derives the reported state. An edition that is being served is
// never reported as an error: clients read "error" as "nothing readable", and
// a failed update leaves the installed edition online. The code of the failed
// operation is still reported in error_code.
func (m *Manager) stateLocked() string {
	switch {
	case m.op != nil:
		return m.op.phase
	case m.interrupted:
		return StateInterrupted
	case m.lib != nil:
		return StateReady
	case m.errCode != "" || m.loadCode != "":
		return StateError
	default:
		return StateNotInstalled
	}
}

// Acquire returns the open edition for one request. release must be called
// exactly once (extra calls are ignored); ok is false when the integration is
// disabled, no readable edition is installed or the first load after Start
// has not finished yet.
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
