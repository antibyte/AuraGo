package ruview

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/config"
)

// maxStorageEntries caps one walk. A data directory with more entries than
// this is treated as unreadable (storage_unreadable) instead of walked for
// minutes: the budget check must stay cheap enough to run on every recording
// start and every few seconds in the background.
const maxStorageEntries = 200_000

var (
	errStorageTooLarge   = errors.New("RuView data directory has too many entries")
	errStorageUnreadable = errors.New("RuView storage could not be checked")
)

func dataDirectory(c *config.Config) string {
	return filepath.Join(c.Directories.DataDir, "ruview", "data")
}

// walkAndStore measures the managed data directory and caches the outcome for
// Status. The walk runs without any lock; only the cache is written under mu.
// A walk ended by its context says nothing about the storage and is not cached.
func (m *Manager) walkAndStore(ctx context.Context, c *config.Config) (int64, error) {
	m.mu.Lock()
	walk := m.walkStorage
	m.mu.Unlock()
	if walk == nil {
		walk = directoryBytes
	}
	used, err := walk(ctx, dataDirectory(c))
	if err != nil && ctx.Err() != nil {
		return 0, ctx.Err()
	}
	m.mu.Lock()
	m.storageAt = time.Now()
	if err != nil {
		m.storageUsed, m.storageKnown, m.storageErr = 0, false, true
	} else {
		m.storageUsed, m.storageKnown, m.storageErr = used, true, false
	}
	m.mu.Unlock()
	return used, err
}

// sampleStorage walks the managed data directory at most every m.storageEvery
// (M28). sampled is false when the cached result is still young.
func (m *Manager) sampleStorage(ctx context.Context, c *config.Config) (used int64, sampled bool, err error) {
	m.mu.Lock()
	young := !m.storageAt.IsZero() && time.Since(m.storageAt) < m.storageEvery
	if !young {
		// Claim the interval now so a slow walk is not started twice.
		m.storageAt = time.Now()
	}
	m.mu.Unlock()
	if young {
		return 0, false, nil
	}
	used, err = m.walkAndStore(ctx, c)
	return used, true, err
}

// storageCheck answers the budget question for a recording start. A sample
// that is at most storageEvery old is used as it is (a burst of starts must
// not walk the directory each time; the watch loop enforces the budget at the
// same cadence); otherwise it walks and refreshes the cache. An unreadable
// directory is an error, not "full": the caller must not guess.
func (m *Manager) storageCheck(ctx context.Context, c *config.Config) (limited bool, err error) {
	budget := c.RuView.StorageMB << 20
	m.mu.Lock()
	if !m.storageAt.IsZero() && time.Since(m.storageAt) < m.storageEvery && (m.storageKnown || m.storageErr) {
		used, failed := m.storageUsed, m.storageErr
		m.mu.Unlock()
		if failed {
			return false, errStorageUnreadable
		}
		return used >= budget, nil
	}
	m.mu.Unlock()
	used, err := m.walkAndStore(ctx, c)
	if err != nil {
		return false, err
	}
	return used >= budget, nil
}

// directoryBytes sums the regular files below root. A missing root is empty
// (nothing recorded yet); links and special files are errors, and so is a
// directory with more than maxStorageEntries entries. The walk stops with ctx.
func directoryBytes(ctx context.Context, root string) (int64, error) {
	return walkBytes(ctx, root, maxStorageEntries)
}

func walkBytes(ctx context.Context, root string, limit int) (int64, error) {
	var size int64
	entries := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == root && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		entries++
		if entries > limit {
			return errStorageTooLarge
		}
		if entries%256 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("RuView data directory contains a link")
		}
		if !d.IsDir() {
			i, e := d.Info()
			if e != nil {
				return e
			}
			if !i.Mode().IsRegular() {
				return fmt.Errorf("RuView data contains a special file")
			}
			size += i.Size()
		}
		return nil
	})
	if err == nil {
		err = ctx.Err()
	}
	return size, err
}
