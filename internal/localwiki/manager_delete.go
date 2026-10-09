package localwiki

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Delete removes the installed edition, partial downloads, download.json and
// state.json. An edition that is still being read is closed and deleted when
// its last reader releases it. It refuses to run while a download is active,
// and no download can start while it runs.
func (m *Manager) Delete() error {
	// Like Install, Delete does not wait for the first load after Start.
	if m.firstLoadPending() {
		return ErrBusy
	}
	// A storage-directory reload must not interleave with the deletion.
	m.loadMu.Lock()
	defer m.loadMu.Unlock()
	m.mu.Lock()
	if m.op != nil || m.deleting {
		m.mu.Unlock()
		return ErrBusy
	}
	m.deleting = true
	dir := m.activeDir
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.deleting = false
		m.mu.Unlock()
		m.signalReload() // apply a storage-directory change made meanwhile
	}()
	if !filepath.IsAbs(dir) {
		return nil
	}
	// download.json goes first: without it the leftovers below are never
	// mistaken for a resumable download if Delete stops half way.
	pending, err := readDownload(dir, m.catalogBase)
	if err != nil {
		m.logger.Warn("[LocalWikipedia] download.json is unreadable; only partial files are removed", "error", err)
		pending = nil
	}
	if err := removeDownload(dir); err != nil {
		return fmt.Errorf("remove download.json: %w", err)
	}
	if err := removePartFiles(dir); err != nil {
		return fmt.Errorf("remove partial download: %w", err)
	}
	if pending != nil {
		if err := m.removeUnpublished(dir, pending.Target.FileName); err != nil {
			return fmt.Errorf("remove unpublished download: %w", err)
		}
	}
	if err := m.detachInstalled(dir); err != nil {
		return fmt.Errorf("update state.json: %w", err)
	}
	return nil
}

// removeUnpublished deletes a finished download that a crash left unnamed by
// state.json (see reconcileDownload), or a retired edition of the same name
// that a download took over. The installed edition is not touched here:
// detachInstalled retires it, so readers can finish.
func (m *Manager) removeUnpublished(dir, fileName string) error {
	m.mu.Lock()
	installed := m.installedFileLocked(fileName)
	m.mu.Unlock()
	if installed {
		return nil
	}
	return removeRegularFile(filepath.Join(dir, fileName))
}

// installedFileLocked reports whether state.json names fileName as the
// installed edition. The caller holds mu.
func (m *Manager) installedFileLocked(fileName string) bool {
	return m.state != nil && m.state.Edition != nil && m.state.Edition.FileName == fileName
}

// removeRegularFile deletes path if it is a regular file (never a link or a
// directory).
func removeRegularFile(path string) error {
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	return removeIfExists(path)
}

// detachInstalled takes the installed edition out of service and schedules its
// file for deletion once no reader uses it. Install uses it for
// ReplaceDeleteOldFirst; Wikipedia is then offline until the new edition is
// published. It also clears the interrupted and error markers: whatever they
// described belongs to the removed edition or download.
func (m *Manager) detachInstalled(dir string) error {
	m.mu.Lock()
	ref := m.lib
	m.lib = nil
	if m.state != nil && m.state.Edition != nil {
		m.state.PendingDelete = append(m.state.PendingDelete, m.state.Edition.FileName)
		m.state.Edition = nil
		m.state.Update = nil
	}
	m.interrupted = false
	m.loadCode = ""
	m.errCode = ""
	m.errRequired = 0
	m.mu.Unlock()
	err := m.saveState(dir)
	if ref != nil {
		ref.retire(func() { m.processPendingDeletes(dir) })
	} else {
		m.processPendingDeletes(dir)
	}
	return err
}

// removePartFiles deletes every partial edition download (<edition>.zim.part
// and an unfinished restart, <edition>.zim.part.restart) in dir; other files
// are never touched.
func removePartFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var first error
	for _, entry := range entries {
		name, isPart := strings.CutSuffix(strings.TrimSuffix(entry.Name(), restartSuffix), ".part")
		if !isPart || !entry.Type().IsRegular() || !zimFileNamePattern.MatchString(name) {
			continue
		}
		if err := removeIfExists(filepath.Join(dir, entry.Name())); err != nil && first == nil {
			first = err
		}
	}
	return first
}
