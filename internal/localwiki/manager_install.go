package localwiki

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"aurago/internal/fileutil"
)

// cancelWait is how long Cancel waits for the download to stop.
const cancelWait = 10 * time.Second

// installPlan is everything a download needs after the synchronous preflight.
type installPlan struct {
	dir            string
	target         Edition
	urls           []string
	resumeBytes    int64
	staleTarget    string
	deleteOldFirst bool
	required       int64
}

// Install checks the preconditions synchronously (enabled, idle, storage
// directory, catalog and .meta4, free space) and then downloads, verifies and
// publishes the selected edition in the background. A matching interrupted
// download is resumed. The background work is bound to the manager's
// lifetime, not to ctx.
func (m *Manager) Install(ctx context.Context, req InstallRequest) error {
	// Checked before loadIfStale: the request must not wait for the first
	// load, which may hang on an unreachable storage directory.
	if m.firstLoadPending() {
		if !m.Settings().Enabled {
			return ErrDisabled
		}
		return ErrBusy
	}
	// A storage directory changed a moment ago is loaded here, unless a load
	// already runs: the request never waits for one.
	m.tryLoadIfStale()
	m.mu.Lock()
	settings := m.settings
	var installed *Edition
	if m.state != nil && m.state.Edition != nil {
		edition := *m.state.Edition
		installed = &edition
	}
	busy := !m.started || m.shuttingDown || m.op != nil || m.deleting || m.ioToken != 0 || settings.DataDir != m.activeDir
	m.mu.Unlock()
	if !settings.Enabled {
		return ErrDisabled
	}
	// The lexical check comes first: an invalid directory is reported even
	// while it cannot be loaded.
	if err := checkDataDirShape(settings.DataDir, m.sensitive); err != nil {
		return err
	}
	if busy {
		return ErrBusy
	}
	if err := prepareDataDir(settings.DataDir, m.sensitive); err != nil {
		return err
	}
	plan, err := m.planInstall(ctx, settings, installed)
	if err != nil {
		return err
	}
	if err := m.checkInstallSpace(&plan, req, installed); err != nil {
		return err
	}
	return m.startOperation(plan)
}

func (m *Manager) planInstall(ctx context.Context, settings Settings, installed *Edition) (installPlan, error) {
	plan := installPlan{dir: settings.DataDir}
	// Before the space check: a leftover restart file would count as used.
	m.removeStaleRestartFiles(plan.dir)
	pending, err := readDownload(settings.DataDir, m.catalogBase)
	if err != nil {
		m.logger.Warn("[LocalWikipedia] download.json is unreadable; starting a new download", "error", err)
		pending = nil
	}
	if pending != nil && pending.Target.Language == settings.Language && pending.Target.Variant == settings.Variant {
		plan.target = pending.Target
		plan.urls = preferURL(pending.URLs, pending.LastURL)
	} else {
		if pending != nil {
			plan.staleTarget = pending.Target.FileName
		}
		target, urls, err := m.resolveTarget(ctx, settings, installed)
		if err != nil {
			if errors.Is(err, ErrAlreadyInstalled) {
				m.discardPending(plan.dir, pending)
			}
			return installPlan{}, err
		}
		plan.target, plan.urls = target, urls
	}
	if installed != nil && installed.Name == plan.target.Name {
		m.discardPending(plan.dir, pending)
		return installPlan{}, ErrAlreadyInstalled
	}
	if plan.staleTarget == "" {
		if info, err := os.Stat(filepath.Join(plan.dir, plan.target.FileName+".part")); err == nil && info.Size() <= plan.target.Size {
			plan.resumeBytes = info.Size()
		}
	}
	return plan, nil
}

// removeStaleRestartFiles deletes the restart files a killed download left in
// dir (see removeRestartFiles). They are never resumed and only occupy space
// the install's space check would count as used. Like discardPending it acts
// only while the manager is idle and marks the cleanup with beginStorageIO, so
// no download that could be writing a restart file starts meanwhile; mu is not
// held during the removals.
func (m *Manager) removeStaleRestartFiles(dir string) {
	release, ok := m.beginStorageIO(dir)
	if !ok {
		return
	}
	defer release()
	m.cleanRestartFiles(dir)
}

// discardPending drops an interrupted download (its download.json and partial
// file) that the installed edition made pointless, and clears the interrupted
// marker so the status no longer offers to resume it. pending is nil when
// there is no readable download.json. Nothing is touched unless the manager is
// idle (beginStorageIO): the cleanup is marked as storage I/O, so no operation can
// start (and begin to write the files) meanwhile, no running one loses its
// files, and no load replaces the loaded state. mu is not held during the
// removals, which may hang on an unreachable share.
func (m *Manager) discardPending(dir string, pending *downloadFile) {
	release, ok := m.beginStorageIO(dir)
	if !ok {
		return
	}
	defer release()
	if pending != nil {
		name := pending.Target.FileName
		// The installed edition cannot change while the storage I/O is marked.
		m.mu.Lock()
		installed := m.installedFileLocked(name)
		m.mu.Unlock()
		if m.beforeStorageIO != nil {
			m.beforeStorageIO()
		}
		if err := removeDownload(dir); err != nil {
			m.logger.Warn("[LocalWikipedia] download.json could not be removed", "error", err)
		}
		if err := removePartialDownload(dir, name); err != nil {
			m.logger.Warn("[LocalWikipedia] A stale partial download could not be removed", "file", name, "error", err)
		}
		if !installed {
			if err := removeRegularFile(filepath.Join(dir, name)); err != nil {
				m.logger.Warn("[LocalWikipedia] A retired edition could not be removed", "file", name, "error", err)
			}
		}
	}
	m.mu.Lock()
	m.interrupted = false
	m.errCode = ""
	m.errRequired = 0
	m.mu.Unlock()
}

// resolveTarget finds the newest edition for the selection and reads its
// exact size, SHA-256 and mirrors from the .meta4. The Kiwix load balancer URL
// (the .meta4 URL without ".meta4") is the last mirror. An edition that is
// already installed is reported before the .meta4 is fetched.
func (m *Manager) resolveTarget(ctx context.Context, settings Settings, installed *Edition) (Edition, []string, error) {
	info, err := m.catalog(ctx, settings.Language, false)
	if err != nil {
		return Edition{}, nil, err
	}
	entry, ok := info.Variants[settings.Variant]
	if !ok {
		return Edition{}, nil, fmt.Errorf("%w: no %s edition for %s", ErrCatalogUnreachable, settings.Variant, settings.Language)
	}
	if installed != nil && installed.Name == entry.Name {
		return Edition{}, nil, ErrAlreadyInstalled
	}
	meta4URL, err := requireHTTPS(entry.Meta4URL)
	if err != nil || !meta4HostAllowed(meta4URL, m.catalogBase) {
		return Edition{}, nil, fmt.Errorf("%w: unexpected metadata URL %q", ErrCatalogUnreachable, entry.Meta4URL)
	}
	data, err := m.fetchSmall(ctx, entry.Meta4URL, meta4BodyLimit)
	if err != nil {
		return Edition{}, nil, fmt.Errorf("%w: %v", ErrCatalogUnreachable, err)
	}
	meta, err := parseMeta4(data, entry.Name+".zim", m.catalogBase)
	if err != nil {
		return Edition{}, nil, fmt.Errorf("%w: %v", ErrCatalogUnreachable, err)
	}
	target := Edition{
		Language: settings.Language, Variant: settings.Variant, Date: entry.Date, Name: entry.Name,
		FileName: meta.FileName, Size: meta.Size, SHA256: meta.SHA256, ArticleCount: entry.ArticleCount,
	}
	return target, mirrorList(meta.URLs, strings.TrimSuffix(entry.Meta4URL, ".meta4")), nil
}

// mirrorList puts the load-balancer URL last without exceeding maxMirrors,
// which download.json refuses to read back.
func mirrorList(mirrors []string, fallback string) []string {
	out := make([]string, 0, min(len(mirrors)+1, maxMirrors))
	for _, mirror := range mirrors {
		if mirror != fallback && len(out) < maxMirrors-1 {
			out = append(out, mirror)
		}
	}
	return append(out, fallback)
}

func preferURL(urls []string, first string) []string {
	if first == "" {
		return append([]string(nil), urls...)
	}
	out := []string{first}
	for _, candidate := range urls {
		if candidate != first {
			out = append(out, candidate)
		}
	}
	return out
}

// checkInstallSpace requires free >= missing bytes + max(1 GiB, 1 %). Unknown
// free space needs ConfirmUnknownSpace; with ReplaceDeleteOldFirst the
// installed edition's size counts as free because it is deleted first.
// CanDeleteOld tells the UI whether the download would fit once the installed
// edition is gone.
func (m *Manager) checkInstallSpace(plan *installPlan, req InstallRequest, installed *Edition) error {
	plan.required = requiredBytes(plan.target.Size, plan.resumeBytes)
	hasOld := installed != nil && installed.FileName != plan.target.FileName
	plan.deleteOldFirst = req.ReplaceMode == ReplaceDeleteOldFirst && hasOld
	free, err := m.freeDisk(plan.dir)
	if err != nil {
		if !req.ConfirmUnknownSpace {
			return ErrUnknownFreeSpace
		}
		m.logger.Warn("[LocalWikipedia] Free space is unknown; the administrator confirmed the download", "dir", plan.dir, "error", err)
		return nil
	}
	fitsWithoutOld := free >= plan.required
	fitsAfterDelete := hasOld && (fitsWithoutOld || installed.Size >= plan.required-free)
	if fitsWithoutOld || (plan.deleteOldFirst && fitsAfterDelete) {
		return nil
	}
	return &InsufficientSpaceError{Required: plan.required, Available: free, CanDeleteOld: fitsAfterDelete}
}

func (m *Manager) startOperation(plan installPlan) error {
	m.mu.Lock()
	if m.op != nil || m.deleting || m.ioToken != 0 || m.shuttingDown || plan.dir != m.activeDir {
		m.mu.Unlock()
		return ErrBusy
	}
	ctx, cancel := context.WithCancel(m.lifecycleCtx)
	op := &operation{
		cancel: cancel, done: make(chan struct{}), phase: StateDownloading,
		target: plan.target, bytesDone: plan.resumeBytes, required: plan.required,
	}
	m.op = op
	m.interrupted = false
	m.errCode = ""
	m.errRequired = 0
	// A retired edition of the same name still waiting for its deletion must
	// not be deleted once the download is published under that name.
	unlisted := false
	if m.state != nil && slices.Contains(m.state.PendingDelete, plan.target.FileName) {
		m.state.PendingDelete = slices.DeleteFunc(m.state.PendingDelete, func(name string) bool { return name == plan.target.FileName })
		unlisted = true
	}
	m.wg.Add(1)
	m.mu.Unlock()
	if unlisted {
		if err := m.saveState(plan.dir); err != nil {
			m.logger.Warn("[LocalWikipedia] state.json could not be updated", "error", err)
		}
	}
	go m.runOperation(ctx, op, plan)
	return nil
}

func (m *Manager) runOperation(ctx context.Context, op *operation, plan installPlan) {
	defer m.wg.Done()
	defer op.cancel()
	err := m.install(ctx, op, plan)
	m.finishOperation(op, plan, err)
}

func (m *Manager) install(ctx context.Context, op *operation, plan installPlan) error {
	if plan.staleTarget != "" && plan.staleTarget != plan.target.FileName {
		if err := removePartialDownload(plan.dir, plan.staleTarget); err != nil {
			m.logger.Warn("[LocalWikipedia] A stale partial download could not be removed", "file", plan.staleTarget, "error", err)
		}
		// startOperation unlisted a retired edition of that name when the
		// stale download began; nothing would delete it any more.
		if err := m.removeUnpublished(plan.dir, plan.staleTarget); err != nil {
			m.logger.Warn("[LocalWikipedia] A retired edition could not be removed", "file", plan.staleTarget, "error", err)
		}
	}
	if plan.deleteOldFirst {
		if err := m.detachInstalled(plan.dir); err != nil {
			m.logger.Warn("[LocalWikipedia] state.json could not be updated", "error", err)
		}
	}
	record := &downloadFile{Target: plan.target, URLs: plan.urls, StartedAt: m.now().UTC()}
	if err := writeDownload(plan.dir, record); err != nil {
		return fmt.Errorf("write download.json: %w", err)
	}
	partPath := filepath.Join(plan.dir, plan.target.FileName+".part")
	job := &downloadJob{
		client:       m.client,
		logger:       m.logger,
		partPath:     partPath,
		dir:          plan.dir,
		size:         plan.target.Size,
		sha256:       plan.target.SHA256,
		urls:         plan.urls,
		freeDisk:     m.freeDisk,
		checkEvery:   m.diskInterval,
		stallTimeout: m.stallTimeout,
		onPhase:      func(phase string) { m.setPhase(op, phase) },
		onProgress:   func(done, _ int64) { m.setProgress(op, done) },
		onURL: func(finalURL string) {
			// download.json is read back after a restart: it must only hold
			// URLs readDownload accepts.
			if record.LastURL == finalURL || checkMirrorURL(finalURL, plan.target.FileName, []*url.URL{m.catalogBase}) != nil {
				return
			}
			record.LastURL = finalURL
			if err := writeDownload(plan.dir, record); err != nil {
				m.logger.Warn("[LocalWikipedia] download.json could not be updated", "error", err)
			}
		},
	}
	if err := job.run(ctx); err != nil {
		return err
	}
	m.setPhase(op, StateVerifying)
	return m.publish(ctx, plan, partPath)
}

// publish verifies the part file, renames it into place, records state.json
// and swaps the library. The previous edition is closed and deleted when its
// last reader releases it.
func (m *Manager) publish(ctx context.Context, plan installPlan, partPath string) error {
	probe, err := OpenLibrary(partPath, plan.target)
	if err != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("%w: %v", errZIMUnreadable, err)
	}
	edition := plan.target
	edition.UUID = probe.uuid()
	edition.ArticleCount = probe.archive.ArticleCount()
	_ = probe.Close()

	finalPath := filepath.Join(plan.dir, edition.FileName)
	if err := fileutil.RenameContext(ctx, partPath, finalPath); err != nil {
		return fmt.Errorf("publish edition: %w", err)
	}
	edition.InstalledAt = m.now().UTC()
	lib, err := OpenLibrary(finalPath, edition)
	if err != nil {
		_ = os.Remove(finalPath)
		return fmt.Errorf("%w: %v", errZIMUnreadable, err)
	}

	m.stateMu.Lock()
	m.mu.Lock()
	next := &stateFile{Edition: &edition, LastUpdateCheck: edition.InstalledAt}
	if m.state != nil {
		for _, name := range m.state.PendingDelete {
			if name != edition.FileName { // never schedule the file just published
				next.PendingDelete = append(next.PendingDelete, name)
			}
		}
		if m.state.Edition != nil && m.state.Edition.FileName != edition.FileName && !slices.Contains(next.PendingDelete, m.state.Edition.FileName) {
			next.PendingDelete = append(next.PendingDelete, m.state.Edition.FileName)
		}
	}
	m.mu.Unlock()
	if err := writeState(plan.dir, cloneState(next)); err != nil {
		m.stateMu.Unlock()
		_ = lib.Close()
		// Keep the verified download for a retry instead of fetching it again.
		if renameErr := os.Rename(finalPath, partPath); renameErr != nil {
			m.logger.Warn("[LocalWikipedia] The verified download could not be set aside for a retry", "file", edition.FileName, "error", renameErr)
		}
		return fmt.Errorf("write state.json: %w", err)
	}
	m.mu.Lock()
	previous := m.lib
	m.lib = newLibraryRef(lib)
	m.state = next
	m.loadCode = "" // a replaced edition that could not be read no longer matters
	m.mu.Unlock()
	m.stateMu.Unlock()

	if err := removeDownload(plan.dir); err != nil {
		m.logger.Warn("[LocalWikipedia] download.json could not be removed", "error", err)
	}
	if lib.fulltextNote != "" {
		m.logger.Warn("[LocalWikipedia] Full-text search unavailable; title search only", "reason", lib.fulltextNote)
	}
	if previous != nil {
		previous.retire(func() { m.processPendingDeletes(plan.dir) })
	} else {
		m.processPendingDeletes(plan.dir)
	}
	m.logger.Info("[LocalWikipedia] Edition installed", "edition", edition.Name, "fulltext", lib.Fulltext())
	return nil
}

func (m *Manager) finishOperation(op *operation, plan installPlan, err error) {
	var space *spaceError
	code := ""
	interrupted := false
	var required int64
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		interrupted = true
	case errors.As(err, &space):
		interrupted, code, required = true, CodeInsufficientDiskSpace, space.required
	case errors.Is(err, errChecksumMismatch):
		code = CodeChecksumMismatch
	case errors.Is(err, errZIMUnreadable):
		code = CodeZIMUnreadable
	default:
		interrupted, code = true, CodeDownloadFailed
	}
	if code == CodeChecksumMismatch || code == CodeZIMUnreadable {
		if removeErr := removeDownload(plan.dir); removeErr != nil {
			m.logger.Warn("[LocalWikipedia] download.json could not be removed", "error", removeErr)
		}
	}
	m.mu.Lock()
	m.op = nil
	m.interrupted = interrupted
	m.errCode = code
	m.errRequired = required
	if space != nil {
		// The download's own measurement is the newest one.
		m.diskSeq++
		m.recordFreeLocked(plan.dir, space.available, m.diskSeq)
	}
	m.mu.Unlock()
	close(op.done)
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		m.logger.Info("[LocalWikipedia] Download stopped; it can be resumed", "edition", plan.target.Name)
	default:
		m.logger.Warn("[LocalWikipedia] Download did not complete", "edition", plan.target.Name, "code", code, "error", err)
	}
	// A storage-directory change made while the operation ran was held back
	// by loadIfStale; load it now. The download changed the free space.
	m.signalReload()
	m.signalProbe()
}

func (m *Manager) setPhase(op *operation, phase string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	op.phase = phase
	op.meter.reset()
	op.rate, op.eta = 0, 0
}

func (m *Manager) setProgress(op *operation, done int64) {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	op.bytesDone = done
	if op.phase != StateDownloading {
		return
	}
	op.rate = op.meter.observe(now, done)
	op.eta = 0
	if op.rate > 0 {
		op.eta = (op.target.Size - done + op.rate - 1) / op.rate
	}
}

// Cancel stops the running download and keeps its part file, so Install
// resumes it later. It waits up to ten seconds for the download to stop.
func (m *Manager) Cancel() error {
	m.mu.Lock()
	op := m.op
	m.mu.Unlock()
	if op == nil {
		return ErrNoOperation
	}
	op.cancel()
	select {
	case <-op.done:
	case <-time.After(cancelWait):
	}
	return nil
}
