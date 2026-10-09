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
	if err := removePartFiles(dir); err != nil {
		return fmt.Errorf("remove partial download: %w", err)
	}
	if err := removeDownload(dir); err != nil {
		return fmt.Errorf("remove download.json: %w", err)
	}
	if err := m.detachInstalled(dir); err != nil {
		return fmt.Errorf("update state.json: %w", err)
	}
	return nil
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
	}
	m.update = nil
	m.interrupted = false
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

// removePartFiles deletes every partial edition download (<edition>.zim.part)
// in dir; other files are never touched.
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
		name, isPart := strings.CutSuffix(entry.Name(), ".part")
		if !isPart || !entry.Type().IsRegular() || !zimFileNamePattern.MatchString(name) {
			continue
		}
		if err := removeIfExists(filepath.Join(dir, entry.Name())); err != nil && first == nil {
			first = err
		}
	}
	return first
}
