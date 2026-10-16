package bookshelf

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetCachePinned keeps the exact prepared source snapshot on this server.
// Pinning first prepares and validates the source; it never writes to a source.
func (s *Service) SetCachePinned(ctx context.Context, bookID string, expectedBookRevision int64, pinned bool) (CacheInfo, error) {
	if err := s.checkOpen(); err != nil {
		return CacheInfo{}, err
	}
	book, err := s.GetBook(ctx, bookID)
	if err != nil {
		return CacheInfo{}, err
	}
	if book.Revision != expectedBookRevision {
		return CacheInfo{}, ErrConflict
	}
	var lease *Lease
	var source Source
	if pinned {
		lease, err = s.Prepare(ctx, bookID)
		if err != nil {
			return CacheInfo{}, err
		}
		defer lease.Close()
		source, err = s.GetSource(ctx, book.SourceID)
		if err != nil {
			return CacheInfo{}, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CacheInfo{}, err
	}
	defer tx.Rollback()
	var currentRevision int64
	var contentRevision string
	if err := tx.QueryRowContext(ctx, `SELECT revision,content_revision FROM books WHERE id=?`, bookID).Scan(&currentRevision, &contentRevision); errors.Is(err, sql.ErrNoRows) {
		return CacheInfo{}, ErrNotFound
	} else if err != nil {
		return CacheInfo{}, err
	}
	if currentRevision != expectedBookRevision {
		return CacheInfo{}, ErrConflict
	}
	if pinned {
		var generation int64
		if err := tx.QueryRowContext(ctx, `SELECT generation FROM sources WHERE id=?`, source.ID).Scan(&generation); errors.Is(err, sql.ErrNoRows) {
			return CacheInfo{}, ErrNotFound
		} else if err != nil {
			return CacheInfo{}, err
		}
		if generation != source.Generation || contentRevision != lease.Revision {
			return CacheInfo{}, ErrStaleRendition
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO cache_pins(book_id,cache_key,content_revision,source_generation,size,updated_at) VALUES(?,?,?,?,?,?)
			ON CONFLICT(book_id) DO UPDATE SET cache_key=excluded.cache_key,content_revision=excluded.content_revision,source_generation=excluded.source_generation,size=excluded.size,updated_at=excluded.updated_at`,
			bookID, lease.key, lease.Revision, source.Generation, lease.Size, nowText()); err != nil {
			return CacheInfo{}, fmt.Errorf("pin bookshelf cache snapshot: %w", err)
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM cache_pins WHERE book_id=?`, bookID); err != nil {
			return CacheInfo{}, fmt.Errorf("unpin bookshelf cache snapshot: %w", err)
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE books SET revision=revision+1 WHERE id=? AND revision=?`, bookID, expectedBookRevision)
	if err != nil {
		return CacheInfo{}, fmt.Errorf("update bookshelf cache pin revision: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return CacheInfo{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return CacheInfo{}, fmt.Errorf("commit bookshelf cache pin: %w", err)
	}
	return s.CacheInfo(ctx)
}

// CacheInfo reports private cache usage and pinned usage without exposing paths.
func (s *Service) CacheInfo(ctx context.Context) (CacheInfo, error) {
	if err := s.checkOpen(); err != nil {
		return CacheInfo{}, err
	}
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		return CacheInfo{}, fmt.Errorf("read bookshelf cache: %w", err)
	}
	info := CacheInfo{LimitBytes: s.maxCacheBytes}
	used := make(map[string]int64)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".book") {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".book")
		if !validCacheKey(key) {
			continue
		}
		fileInfo, err := entry.Info()
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Size() < 0 {
			continue
		}
		used[key] = fileInfo.Size()
		info.UsedBytes += fileInfo.Size()
		info.Books++
	}
	rows, err := s.db.QueryContext(ctx, `SELECT cache_key FROM cache_pins`)
	if err != nil {
		return CacheInfo{}, fmt.Errorf("read bookshelf cache pins: %w", err)
	}
	defer rows.Close()
	seen := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return CacheInfo{}, err
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if size, ok := used[key]; ok {
			info.PinnedBytes += size
			info.PinnedBooks++
		}
	}
	if err := rows.Err(); err != nil {
		return CacheInfo{}, err
	}
	return info, nil
}

func (s *Service) openPinnedSnapshot(ctx context.Context, book Book, source Source) (*Lease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var key, revision string
	var generation, size int64
	err := s.db.QueryRowContext(ctx, `SELECT cache_key,content_revision,source_generation,size FROM cache_pins WHERE book_id=?`, book.ID).
		Scan(&key, &revision, &generation, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if revision != book.ContentRevision || revision == "" || generation != source.Generation || size < 1 || !validCacheKey(key) {
		return nil, ErrStaleRendition
	}
	path := filepath.Join(s.cacheDir, key+".book")
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return nil, ErrStaleRendition
	}
	return s.openLease(key, path, size, revision)
}

func (s *Service) pinnedCacheKeys() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT cache_key FROM cache_pins`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make(map[string]bool)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		if validCacheKey(key) {
			keys[key] = true
		}
	}
	return keys, rows.Err()
}

func validCacheKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(key)
	return err == nil && len(decoded) == 32
}
