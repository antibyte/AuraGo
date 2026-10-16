package bookshelf

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aurago/internal/fileutil"
)

func (s *Service) Prepare(ctx context.Context, bookID string) (*Lease, error) {
	return s.prepare(ctx, bookID, true)
}

// PrepareReadOnly prepares an original for serving without changing catalog
// metadata or reader history. It may populate/evict disposable cache files.
// Use this when the enclosing application is in global read-only mode.
func (s *Service) PrepareReadOnly(ctx context.Context, bookID string) (*Lease, error) {
	return s.prepare(ctx, bookID, false)
}

func (s *Service) prepare(ctx context.Context, bookID string, recordUnverified bool) (*Lease, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	maxBookBytes := s.maxBookBytes.Load()
	book, err := s.GetBook(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if !book.Available && !book.CachePinned {
		return nil, ErrNotFound
	}
	source, err := s.GetSource(ctx, book.SourceID)
	if err != nil {
		return nil, err
	}
	adapter := s.adapters[source.Provider]
	if adapter == nil {
		return nil, ErrUnsupported
	}
	entry, err := adapter.Stat(ctx, source, book.Path)
	if err != nil {
		if errors.Is(err, ErrSourceRevoked) || errors.Is(err, os.ErrPermission) {
			return nil, err
		}
		if errors.Is(err, ErrSourceUnavailable) {
			if lease, pinnedErr := s.openPinnedSnapshot(ctx, book, source); pinnedErr == nil {
				return lease, nil
			}
		}
		return nil, fmt.Errorf("stat bookshelf source book: %w", err)
	}
	if entry.Size < 0 || entry.Size > maxBookBytes || len(entry.Revision) > 1024 {
		return nil, ErrLimitExceeded
	}
	if entry.Path != "" {
		actualPath, pathErr := cleanRelativePath(entry.Path, false)
		if pathErr != nil || actualPath != book.Path {
			return nil, ErrInvalidPath
		}
	}
	if entry.Size != book.Size || entry.Revision != "" && book.ContentRevision != "" && entry.Revision != book.ContentRevision {
		return nil, ErrStaleRendition
	}
	if entry.Size == 0 {
		return nil, ErrUnsupported
	}
	if entry.Revision == "" {
		if strings.HasPrefix(book.ContentRevision, "unverified:") {
			entry.Revision = book.ContentRevision
		} else {
			entry.Revision = "unverified:pending"
		}
	}
	key := cacheKey(source.ID, source.Generation, book.Path, entry.Revision, entry.Size)
	if !strings.HasPrefix(entry.Revision, "unverified:") {
		if path, size, ok := s.cached(key, entry.Size); ok {
			lease, openErr := s.openLease(key, path, size, entry.Revision)
			if openErr == nil || !errors.Is(openErr, os.ErrNotExist) {
				return lease, openErr
			}
		}
	}
	call, leader, err := s.beginPreparation(key)
	if err != nil {
		return nil, err
	}
	if leader {
		call.key, call.path, call.size, call.rev, call.err = s.prepareFile(ctx, key, source, book, adapter, entry, maxBookBytes)
		s.mu.Lock()
		delete(s.preparing, key)
		close(call.done)
		s.mu.Unlock()
	} else {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
		}
	}
	if call.err != nil {
		return nil, call.err
	}
	if recordUnverified && strings.HasPrefix(call.rev, "unverified:") {
		if err := s.recordUnverifiedSnapshot(ctx, book, call.rev); err != nil {
			return nil, err
		}
	}
	return s.openLease(call.key, call.path, call.size, call.rev)
}

func cacheKey(sourceID string, generation int64, relativePath, revision string, size int64) string {
	keyHash := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s\x00%s\x00%d", sourceID, generation, relativePath, revision, size)))
	return hex.EncodeToString(keyHash[:])
}

// SetMaxBookBytes applies a synchronized source preparation/upload ceiling.
// It is safe to update while requests are active; each preparation snapshots
// one limit and enforces it for its full transfer.
func (s *Service) SetMaxBookBytes(limit int64) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if limit <= 0 || limit > MaxSourceBookBytes {
		return ErrLimitExceeded
	}
	s.maxBookBytes.Store(limit)
	return nil
}

func (s *Service) recordUnverifiedSnapshot(ctx context.Context, book Book, snapshotRevision string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var currentRevision int64
	var currentContentRevision string
	if err := tx.QueryRowContext(ctx, `SELECT revision,content_revision FROM books WHERE id=?`, book.ID).Scan(&currentRevision, &currentContentRevision); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("read bookshelf source fingerprint: %w", err)
	}
	if currentContentRevision == snapshotRevision {
		return nil
	}
	if currentRevision != book.Revision {
		return ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE books SET content_revision=?,reading_state=CASE WHEN reading_state_manual=0 THEN 'unread' ELSE reading_state END,revision=revision+1 WHERE id=? AND revision=?`, snapshotRevision, book.ID, book.Revision); err != nil {
		return fmt.Errorf("record bookshelf source fingerprint: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM progress WHERE book_id=?`, book.ID); err != nil {
		return fmt.Errorf("retire stale bookshelf progress: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM renditions WHERE book_id=?`, book.ID); err != nil {
		return fmt.Errorf("retire stale bookshelf renditions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cache_pins WHERE book_id=?`, book.ID); err != nil {
		return fmt.Errorf("retire stale bookshelf cache pin: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bookshelf source fingerprint: %w", err)
	}
	return nil
}

func (s *Service) beginPreparation(key string) (*prepCall, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, false, ErrClosed
	}
	if existing := s.preparing[key]; existing != nil {
		return existing, false, nil
	}
	call := &prepCall{done: make(chan struct{})}
	s.preparing[key] = call
	return call, true, nil
}

func (s *Service) prepareFile(ctx context.Context, preparationKey string, source Source, book Book, adapter SourceAdapter, entry Entry, maxBookBytes int64) (string, string, int64, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", 0, "", err
	}
	if err := s.reserveCache(entry.Size); err != nil {
		return "", "", 0, "", err
	}
	reserved := true
	defer func() {
		if reserved {
			s.releaseCacheReservation(entry.Size)
		}
	}()
	if err := s.freeSpaceFor(entry.Size); err != nil {
		return "", "", 0, "", err
	}
	reader, err := adapter.Open(ctx, source, book.Path)
	if err != nil {
		return "", "", 0, "", fmt.Errorf("open bookshelf source book: %w", err)
	}
	defer reader.Close()
	tmp, err := os.CreateTemp(s.cacheDir, ".prepare-*.tmp")
	if err != nil {
		return "", "", 0, "", fmt.Errorf("create bookshelf cache staging file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(contextReader{ctx: ctx, r: reader}, maxBookBytes+1))
	if copyErr != nil {
		_ = tmp.Close()
		return "", "", 0, "", fmt.Errorf("copy bookshelf source book: %w", copyErr)
	}
	if written > maxBookBytes {
		_ = tmp.Close()
		return "", "", 0, "", ErrLimitExceeded
	}
	if written != entry.Size {
		_ = tmp.Close()
		return "", "", 0, "", ErrStaleRendition
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", "", 0, "", fmt.Errorf("sync bookshelf cache staging file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", "", 0, "", fmt.Errorf("close bookshelf cache staging file: %w", err)
	}
	after, statErr := adapter.Stat(ctx, source, book.Path)
	if statErr != nil {
		return "", "", 0, "", fmt.Errorf("recheck bookshelf source book: %w", statErr)
	}
	if after.Size != entry.Size || entry.Revision != "" && !strings.HasPrefix(entry.Revision, "unverified:") && after.Revision != entry.Revision {
		return "", "", 0, "", ErrStaleRendition
	}
	if after.Path != "" {
		actualPath, pathErr := cleanRelativePath(after.Path, false)
		if pathErr != nil || actualPath != book.Path {
			return "", "", 0, "", ErrInvalidPath
		}
	}
	revision := entry.Revision
	if strings.HasPrefix(revision, "unverified:") {
		revision = "unverified:" + hex.EncodeToString(hash.Sum(nil))
	}
	// Providers without stable revisions use the downloaded content digest as
	// the durable object key. Never publish changed bytes over a previously
	// pinned snapshot just because both preparations began from the same
	// placeholder revision.
	key := preparationKey
	if strings.HasPrefix(entry.Revision, "unverified:") {
		key = cacheKey(source.ID, source.Generation, book.Path, revision, written)
	}
	destination := filepath.Join(s.cacheDir, key+".book")
	s.cacheMu.Lock()
	s.reservedBytes -= entry.Size
	reserved = false
	if cachedPath, cachedSize, ok := s.cached(key, written); ok && cachedSize == written {
		// The verified content-addressed object already exists. Keep its inode
		// and discard the staging file rather than replacing a pinned/leased
		// snapshot with identical bytes.
		_ = os.Remove(tmpPath)
		destination = cachedPath
	} else {
		renameErr := fileutil.RenameContext(ctx, tmpPath, destination)
		if renameErr != nil {
			s.cacheMu.Unlock()
			return "", "", 0, "", fmt.Errorf("publish bookshelf cached book: %w", renameErr)
		}
	}
	s.cacheMu.Unlock()
	return key, destination, written, revision, nil
}

func (s *Service) cached(key string, expectedSize int64) (string, int64, bool) {
	path := filepath.Join(s.cacheDir, key+".book")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != expectedSize {
		return "", 0, false
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	return path, info.Size(), true
}

func (s *Service) openLease(key, path string, size int64, revision string) (*Lease, error) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open prepared bookshelf book: %w", err)
	}
	s.activeLeases[key]++
	return &Lease{File: file, Path: path, Size: size, Revision: revision, service: s, key: key}, nil
}

func (s *Service) releaseLease(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeLeases[key] <= 1 {
		delete(s.activeLeases, key)
	} else {
		s.activeLeases[key]--
	}
}

func (s *Service) reserveCache(need int64) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if need > s.maxCacheBytes || need > int64(^uint64(0)>>1)-s.reservedBytes {
		return ErrLimitExceeded
	}
	if err := s.makeCacheRoom(need + s.reservedBytes); err != nil {
		return err
	}
	s.reservedBytes += need
	return nil
}

func (s *Service) releaseCacheReservation(bytes int64) {
	s.cacheMu.Lock()
	s.reservedBytes -= bytes
	if s.reservedBytes < 0 {
		s.reservedBytes = 0
	}
	s.cacheMu.Unlock()
}

func (s *Service) makeCacheRoom(need int64) error {
	if need > s.maxCacheBytes {
		return ErrLimitExceeded
	}
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return fmt.Errorf("read bookshelf cache: %w", err)
	}
	type fileEntry struct {
		path     string
		key      string
		size     int64
		modified time.Time
	}
	files := make([]fileEntry, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".book") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".book")
		files = append(files, fileEntry{path: filepath.Join(s.cacheDir, entry.Name()), key: key, size: info.Size(), modified: info.ModTime()})
		total += info.Size()
	}
	pinnedKeys, err := s.pinnedCacheKeys()
	if err != nil {
		return fmt.Errorf("read pinned bookshelf cache keys: %w", err)
	}
	if total+need <= s.maxCacheBytes {
		return nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modified.Before(files[j].modified) })
	for _, file := range files {
		if total+need <= s.maxCacheBytes {
			break
		}
		s.mu.Lock()
		active := s.activeLeases[file.key] > 0
		s.mu.Unlock()
		if active || pinnedKeys[file.key] {
			continue
		}
		if err := os.Remove(file.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
		total -= file.size
	}
	if total+need > s.maxCacheBytes {
		return ErrLimitExceeded
	}
	return nil
}
