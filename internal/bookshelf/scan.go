package bookshelf

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type scanDirectory struct {
	path  string
	depth int
}

type scannedEntry struct {
	path     string
	title    string
	format   string
	size     int64
	revision string
	preserve bool
}

// Scan lists one source. Entry paths and directory arguments are relative to
// Source.Path. A partial listing adds or refreshes observed books but never
// marks unseen entries unavailable.
func (s *Service) Scan(ctx context.Context, sourceID string) (ScanResult, error) {
	if err := s.checkOpen(); err != nil {
		return ScanResult{}, err
	}
	lock := s.sourceLock(sourceID)
	lock.Lock()
	defer lock.Unlock()
	source, err := s.GetSource(ctx, sourceID)
	if err != nil {
		return ScanResult{}, err
	}
	adapter := s.adapters[source.Provider]
	if adapter == nil {
		return ScanResult{}, ErrUnsupported
	}
	result := ScanResult{SourceID: sourceID, Complete: true}
	maxBookBytes := s.maxBookBytes.Load()
	queue := []scanDirectory{{}}
	seen := make(map[string]scannedEntry)
	processed := 0
	limited := false
	for len(queue) > 0 && !limited {
		if err := ctx.Err(); err != nil {
			return ScanResult{}, err
		}
		dir := queue[0]
		queue = queue[1:]
		entries, listErr := adapter.List(ctx, source, dir.path)
		if listErr != nil {
			if errors.Is(listErr, context.Canceled) || errors.Is(listErr, context.DeadlineExceeded) || ctx.Err() != nil {
				return ScanResult{}, ctx.Err()
			}
			result.Complete = false
			result.Issues = append(result.Issues, ScanIssue{Path: dir.path, Code: "source_unavailable"})
			continue
		}
		for _, entry := range entries {
			processed++
			if processed > s.maxFiles*2 {
				result.Complete = false
				result.Issues = append(result.Issues, ScanIssue{Code: "scan_limit"})
				limited = true
				break
			}
			if err := ctx.Err(); err != nil {
				return ScanResult{}, err
			}
			entryPath, cleanErr := cleanRelativePath(entry.Path, false)
			if cleanErr != nil || len(entryPath) > 4096 {
				result.Complete = false
				result.Issues = append(result.Issues, ScanIssue{Code: "invalid_entry"})
				continue
			}
			expectedParent := dir.path
			if expectedParent == "" {
				expectedParent = "."
			}
			if path.Dir(entryPath) != expectedParent {
				result.Complete = false
				result.Issues = append(result.Issues, ScanIssue{Code: "invalid_entry"})
				continue
			}
			if entry.IsDir {
				if source.Recursive {
					if dir.depth >= 64 {
						result.Complete = false
						result.Issues = append(result.Issues, ScanIssue{Path: entryPath, Code: "depth_limit"})
					} else {
						queue = append(queue, scanDirectory{path: entryPath, depth: dir.depth + 1})
					}
				}
				continue
			}
			format := normalizeFormat(entry.Format, entryPath)
			if !validFormat(format) {
				continue
			}
			result.Seen++
			if result.Seen > s.maxFiles {
				result.Complete = false
				result.Issues = append(result.Issues, ScanIssue{Code: "scan_limit"})
				limited = true
				break
			}
			if entry.Size < 0 || len(entry.Revision) > 1024 {
				result.Complete = false
				result.Issues = append(result.Issues, ScanIssue{Path: entryPath, Code: "invalid_entry"})
				continue
			}
			if entry.Size > maxBookBytes {
				result.Issues = append(result.Issues, ScanIssue{Path: entryPath, Code: "size_limit"})
				seen[entryPath] = scannedEntry{path: entryPath, preserve: true}
				continue
			}
			title := strings.TrimSuffix(path.Base(entryPath), filepath.Ext(entryPath))
			if strings.EqualFold(filepath.Ext(entryPath), ".zip") && strings.HasSuffix(strings.ToLower(entryPath), ".fb2.zip") {
				title = strings.TrimSuffix(path.Base(entryPath), filepath.Ext(entryPath))
				title = strings.TrimSuffix(title, filepath.Ext(title))
			}
			if entry.Name != "" && validateText(entry.Name, 2048) {
				title = strings.TrimSpace(entry.Name)
			}
			seen[entryPath] = scannedEntry{path: entryPath, title: title, format: format, size: entry.Size, revision: entry.Revision}
		}
	}
	if err := ctx.Err(); err != nil {
		return ScanResult{}, err
	}
	if err := s.applyScan(ctx, source, seen, result.Complete, &result); err != nil {
		return ScanResult{}, err
	}
	return result, nil
}

func (s *Service) applyScan(ctx context.Context, source Source, seen map[string]scannedEntry, complete bool, result *ScanResult) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bookshelf scan update: %w", err)
	}
	defer tx.Rollback()
	var generation int64
	if err := tx.QueryRowContext(ctx, `SELECT generation FROM sources WHERE id=?`, source.ID).Scan(&generation); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("check bookshelf source generation: %w", err)
	}
	if generation != source.Generation {
		return ErrConflict
	}
	token, err := newID()
	if err != nil {
		return err
	}
	for _, entry := range seen {
		id := bookID(source.ID, entry.path)
		if entry.preserve {
			// The source entry was observed but exceeds the configured transfer
			// limit. Keep an existing catalog record visible and unchanged; do not
			// import a new oversized entry or retire its history as "missing".
			if _, err := tx.ExecContext(ctx, `UPDATE books SET scan_token=?,last_scanned_at=? WHERE id=? AND source_id=? AND available=1`, token, nowText(), id, source.ID); err != nil {
				return fmt.Errorf("preserve oversized bookshelf entry: %w", err)
			}
			continue
		}
		var oldRevision int64
		var oldContentRevision string
		var oldSize int64
		var oldFormat string
		var oldAvailable int
		getErr := tx.QueryRowContext(ctx, `SELECT revision,content_revision,size,format,available FROM books WHERE id=?`, id).Scan(
			&oldRevision, &oldContentRevision, &oldSize, &oldFormat, &oldAvailable)
		isNew := errors.Is(getErr, sql.ErrNoRows)
		if getErr != nil && !isNew {
			return fmt.Errorf("read existing bookshelf book during scan: %w", getErr)
		}
		revision := int64(1)
		changed := true
		contentChanged := false
		if !isNew {
			revision = oldRevision
			contentChanged = oldSize != entry.size || oldFormat != entry.format || entry.revision != "" && oldContentRevision != entry.revision
			changed = contentChanged || oldAvailable == 0
			if changed {
				revision++
			}
		}
		contentRevision := entry.revision
		if !isNew && contentRevision == "" && !contentChanged {
			contentRevision = oldContentRevision
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO books(id,source_id,path,content_revision,size,format,title,available,revision,scan_token,last_scanned_at)
			VALUES(?,?,?,?,?,?,?,1,?,?,?) ON CONFLICT(id) DO UPDATE SET content_revision=excluded.content_revision,size=excluded.size,format=excluded.format,
				title=CASE WHEN books.title='' THEN excluded.title ELSE books.title END,available=1,revision=excluded.revision,scan_token=excluded.scan_token,last_scanned_at=excluded.last_scanned_at`,
			id, source.ID, entry.path, contentRevision, entry.size, entry.format, entry.title, revision, token, nowText())
		if err != nil {
			return fmt.Errorf("write bookshelf scan result: %w", err)
		}
		if contentChanged {
			if _, err := tx.ExecContext(ctx, `UPDATE books SET reading_state='unread' WHERE id=? AND reading_state_manual=0`, id); err != nil {
				return fmt.Errorf("reset derived bookshelf reading state: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM progress WHERE book_id=?`, id); err != nil {
				return fmt.Errorf("retire stale bookshelf progress: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM renditions WHERE book_id=?`, id); err != nil {
				return fmt.Errorf("retire stale bookshelf renditions: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM cache_pins WHERE book_id=?`, id); err != nil {
				return fmt.Errorf("retire stale bookshelf cache pin: %w", err)
			}
		}
		if isNew {
			result.Added++
		} else if changed {
			result.Updated++
		}
	}
	if complete {
		res, err := tx.ExecContext(ctx, `UPDATE books SET available=0,revision=revision+1 WHERE source_id=? AND available=1 AND scan_token<>?`, source.ID, token)
		if err != nil {
			return fmt.Errorf("mark missing bookshelf books: %w", err)
		}
		if removed, _ := res.RowsAffected(); removed > 0 {
			result.Removed = int(removed)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bookshelf scan: %w", err)
	}
	return nil
}

func normalizeFormat(format, filename string) string {
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	if format != "" {
		return format
	}
	name := strings.ToLower(filename)
	if strings.HasSuffix(name, ".fb2.zip") {
		return "fb2.zip"
	}
	return strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
}
