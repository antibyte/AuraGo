package bookshelf

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"aurago/internal/fileutil"
)

// AttachRendition stores a generated EPUB as private Bookshelf data. The body
// is bounded and tied to the exact source snapshot used for conversion.
func (s *Service) AttachRendition(ctx context.Context, bookID string, expectedBookRevision int64, baseRevision, format string, body io.Reader) (Rendition, error) {
	return s.AttachRenditionVersioned(ctx, bookID, expectedBookRevision, baseRevision, format, "legacy", body)
}

// AttachRenditionVersioned includes the converter fingerprint in the managed
// rendition identity so outputs from changed converter builds are not reused.
func (s *Service) AttachRenditionVersioned(ctx context.Context, bookID string, expectedBookRevision int64, baseRevision, format, converterVersion string, body io.Reader) (Rendition, error) {
	if err := s.checkOpen(); err != nil {
		return Rendition{}, err
	}
	if !validID(bookID) || !validateText(baseRevision, 1024) || baseRevision == "" || !validateText(converterVersion, 1024) || converterVersion == "" || body == nil {
		return Rendition{}, fmt.Errorf("invalid bookshelf rendition")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "epub" {
		return Rendition{}, ErrUnsupported
	}
	book, err := s.GetBook(ctx, bookID)
	if err != nil {
		return Rendition{}, err
	}
	if book.Revision != expectedBookRevision {
		return Rendition{}, ErrConflict
	}
	if book.ContentRevision != "" && book.ContentRevision != baseRevision || book.ContentRevision == "" && !strings.HasPrefix(baseRevision, "unverified:") {
		return Rendition{}, ErrStaleRendition
	}
	file, err := os.CreateTemp(s.renditionsDir, ".stage-*.tmp")
	if err != nil {
		return Rendition{}, fmt.Errorf("create bookshelf rendition staging file: %w", err)
	}
	tmpPath := file.Name()
	defer os.Remove(tmpPath)
	hash := sha256.New()
	writer := io.MultiWriter(spaceCheckedWriter{service: s, writer: file}, hash)
	written, copyErr := io.CopyBuffer(writer, io.LimitReader(contextReader{ctx: ctx, r: body}, MaxRenditionBytes+1), make([]byte, 1<<20))
	if copyErr != nil {
		_ = file.Close()
		return Rendition{}, fmt.Errorf("write bookshelf rendition: %w", copyErr)
	}
	if written == 0 || written > MaxRenditionBytes {
		_ = file.Close()
		if written > MaxRenditionBytes {
			return Rendition{}, ErrLimitExceeded
		}
		return Rendition{}, fmt.Errorf("empty bookshelf rendition")
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return Rendition{}, fmt.Errorf("sync bookshelf rendition: %w", err)
	}
	if err := file.Close(); err != nil {
		return Rendition{}, fmt.Errorf("close bookshelf rendition: %w", err)
	}
	if err := validateEPUB(tmpPath, written); err != nil {
		return Rendition{}, err
	}

	digest := hex.EncodeToString(hash.Sum(nil))
	converterDigest := sha256.Sum256([]byte(converterVersion))
	filename := bookID + "-" + hex.EncodeToString(converterDigest[:8]) + "-" + digest + ".epub"
	destination := filepath.Join(s.renditionsDir, filename)
	s.renditionMu.Lock()
	defer s.renditionMu.Unlock()
	if err := s.makeRenditionRoom(ctx, bookID, format, baseRevision, filename, written); err != nil {
		return Rendition{}, err
	}
	if _, statErr := os.Stat(destination); errors.Is(statErr, os.ErrNotExist) {
		if err := fileutil.RenameContext(ctx, tmpPath, destination); err != nil {
			return Rendition{}, fmt.Errorf("publish bookshelf rendition: %w", err)
		}
	} else if statErr != nil {
		return Rendition{}, fmt.Errorf("check bookshelf rendition: %w", statErr)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Rendition{}, err
	}
	defer tx.Rollback()
	var currentRevision int64
	var currentBase string
	if err := tx.QueryRowContext(ctx, `SELECT revision,content_revision FROM books WHERE id=?`, bookID).Scan(&currentRevision, &currentBase); errors.Is(err, sql.ErrNoRows) {
		return Rendition{}, ErrNotFound
	} else if err != nil {
		return Rendition{}, fmt.Errorf("recheck bookshelf rendition source: %w", err)
	}
	if currentRevision != expectedBookRevision {
		return Rendition{}, ErrConflict
	}
	if currentBase != "" && currentBase != baseRevision || currentBase == "" && !strings.HasPrefix(baseRevision, "unverified:") {
		return Rendition{}, ErrStaleRendition
	}
	updated := nowText()
	if _, err := tx.ExecContext(ctx, `INSERT INTO renditions(book_id,format,base_revision,revision,converter_version,size,path,updated_at) VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(book_id,format,base_revision) DO UPDATE SET revision=excluded.revision,converter_version=excluded.converter_version,size=excluded.size,path=excluded.path,updated_at=excluded.updated_at`,
		bookID, format, baseRevision, digest, converterVersion, written, filename, updated); err != nil {
		return Rendition{}, fmt.Errorf("save bookshelf rendition: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Rendition{}, fmt.Errorf("commit bookshelf rendition: %w", err)
	}
	return Rendition{Format: format, BaseRevision: baseRevision, Revision: digest, ConverterVersion: converterVersion, Size: written, Available: true, UpdatedAt: parseTime(updated)}, nil
}

type spaceCheckedWriter struct {
	service *Service
	writer  io.Writer
}

func (w spaceCheckedWriter) Write(data []byte) (int, error) {
	if err := w.service.freeSpaceFor(int64(len(data))); err != nil {
		return 0, err
	}
	return w.writer.Write(data)
}

// OpenRendition opens only the exact derived output for the requested source
// snapshot. The returned lease must be closed after the response is finished.
func (s *Service) OpenRendition(ctx context.Context, bookID, baseRevision, format string) (*Lease, error) {
	return s.OpenRenditionVersion(ctx, bookID, baseRevision, format, "")
}

// OpenRenditionVersion opens a stored rendition only when its converter
// fingerprint matches. An empty fingerprint is reserved for compatibility
// callers that intentionally accept any stored converter version.
func (s *Service) OpenRenditionVersion(ctx context.Context, bookID, baseRevision, format, converterVersion string) (*Lease, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if !validID(bookID) || baseRevision == "" || !validateText(baseRevision, 1024) || !validateText(converterVersion, 1024) {
		return nil, ErrNotFound
	}
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "epub" {
		return nil, ErrUnsupported
	}
	// Validate the current source grant and snapshot before serving any stored
	// derivative. A source revocation must not be bypassed by a local rendition.
	// Opening a stored derivative is a read operation. In particular, do not
	// persist an unverified source fingerprint here: that would retire reader
	// history, renditions, and pins while the caller may be in Desktop readonly.
	original, err := s.PrepareReadOnly(ctx, bookID)
	if err != nil {
		return nil, err
	}
	currentSnapshot := original.Revision
	_ = original.Close()
	if currentSnapshot != baseRevision {
		return nil, ErrStaleRendition
	}
	book, err := s.GetBook(ctx, bookID)
	if err != nil {
		return nil, err
	}
	if book.ContentRevision != "" && book.ContentRevision != baseRevision {
		return nil, ErrStaleRendition
	}
	var rendition Rendition
	var filename, updated string
	query := `SELECT format,base_revision,revision,converter_version,size,path,updated_at FROM renditions WHERE book_id=? AND format=? AND base_revision=?`
	args := []any{bookID, format, baseRevision}
	if converterVersion != "" {
		query += ` AND converter_version=?`
		args = append(args, converterVersion)
	}
	err = s.db.QueryRowContext(ctx, query, args...).Scan(&rendition.Format, &rendition.BaseRevision, &rendition.Revision, &rendition.ConverterVersion, &rendition.Size, &filename, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read bookshelf rendition: %w", err)
	}
	if filepath.Base(filename) != filename || !strings.HasPrefix(filename, bookID+"-") || !strings.HasSuffix(filename, ".epub") || rendition.Size < 1 || rendition.Size > MaxRenditionBytes {
		return nil, ErrNotFound
	}
	path := filepath.Join(s.renditionsDir, filename)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("stat bookshelf rendition: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() != rendition.Size {
		return nil, ErrStaleRendition
	}
	key := "rendition:" + filename
	lease, err := s.openLease(key, path, rendition.Size, rendition.Revision)
	if err != nil {
		return nil, err
	}
	return lease, nil
}

func (s *Service) loadBookRenditions(ctx context.Context, book *Book) error {
	rows, err := s.db.QueryContext(ctx, `SELECT format,base_revision,revision,converter_version,size,path,updated_at FROM renditions WHERE book_id=? ORDER BY updated_at DESC,format`, book.ID)
	if err != nil {
		return fmt.Errorf("list bookshelf renditions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rendition Rendition
		var filename, updated string
		if err := rows.Scan(&rendition.Format, &rendition.BaseRevision, &rendition.Revision, &rendition.ConverterVersion, &rendition.Size, &filename, &updated); err != nil {
			return err
		}
		rendition.UpdatedAt = parseTime(updated)
		if filepath.Base(filename) == filename && strings.HasPrefix(filename, book.ID+"-") && strings.HasSuffix(filename, ".epub") {
			if info, err := os.Stat(filepath.Join(s.renditionsDir, filename)); err == nil && info.Mode().IsRegular() && info.Size() == rendition.Size {
				rendition.Available = true
			}
		}
		book.Renditions = append(book.Renditions, rendition)
	}
	return rows.Err()
}

// makeRenditionRoom first removes abandoned outputs, then enforces a quota on
// referenced rendition bytes. A replacement excludes its current row.
func (s *Service) makeRenditionRoom(ctx context.Context, bookID, format, baseRevision, filename string, need int64) error {
	entries, err := os.ReadDir(s.renditionsDir)
	if err != nil {
		return fmt.Errorf("read bookshelf rendition storage: %w", err)
	}
	referenced := make(map[string]int64)
	rows, err := s.db.QueryContext(ctx, `SELECT path,MAX(size) FROM renditions GROUP BY path`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var path string
		var size int64
		if err := rows.Scan(&path, &size); err != nil {
			rows.Close()
			return err
		}
		referenced[path] = size
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".epub") {
			continue
		}
		if _, ok := referenced[entry.Name()]; !ok {
			s.mu.Lock()
			active := s.activeLeases["rendition:"+entry.Name()] > 0
			s.mu.Unlock()
			if active {
				if info, err := entry.Info(); err == nil {
					referenced[entry.Name()] = info.Size()
				}
				continue
			}
			if err := os.Remove(filepath.Join(s.renditionsDir, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
	}
	var oldPath string
	err = s.db.QueryRowContext(ctx, `SELECT path FROM renditions WHERE book_id=? AND format=? AND base_revision=?`, bookID, format, baseRevision).Scan(&oldPath)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find replaced bookshelf rendition: %w", err)
	}
	var projected int64
	for path, size := range referenced {
		if path == oldPath && oldPath != filename {
			var otherRefs int
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM renditions WHERE path=? AND NOT (book_id=? AND format=? AND base_revision=?)`, path, bookID, format, baseRevision).Scan(&otherRefs); err != nil {
				return err
			}
			if otherRefs == 0 {
				continue
			}
		}
		if path == filename {
			if need > size {
				projected += need
			} else {
				projected += size
			}
			continue
		}
		projected += size
	}
	if _, alreadyReferenced := referenced[filename]; !alreadyReferenced {
		projected += need
	}
	if need > MaxRenditionBytes || projected > MaxRenditionBytes {
		return ErrLimitExceeded
	}
	return nil
}

func validateEPUB(filename string, size int64) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open bookshelf EPUB rendition: %w", err)
	}
	defer file.Close()
	archive, err := boundedZipReader(file, size)
	if err != nil {
		return fmt.Errorf("invalid bookshelf EPUB archive: %w", err)
	}
	if len(archive.File) == 0 || len(archive.File) > 10_000 || archive.File[0].Name != "mimetype" || archive.File[0].Method != zip.Store {
		return fmt.Errorf("invalid bookshelf EPUB structure")
	}
	const maxEntryBytes = 32 << 20
	const maxExpandedBytes = 256 << 20
	var expanded int64
	validMimetype := false
	containerValid := false
	var containerTargets []string
	paths := make(map[string]struct{}, len(archive.File))
	for _, entry := range archive.File {
		if err := validateArchivePath(entry.Name); err != nil {
			return fmt.Errorf("invalid bookshelf EPUB path: %w", err)
		}
		if entry.UncompressedSize64 > maxEntryBytes || entry.UncompressedSize64 > uint64(maxExpandedBytes-expanded) {
			return ErrLimitExceeded
		}
		compressed := entry.CompressedSize64
		if compressed == 0 && entry.UncompressedSize64 > 0 || compressed > 0 && entry.UncompressedSize64 > compressed*200 {
			return ErrLimitExceeded
		}
		reader, err := entry.Open()
		if err != nil {
			return fmt.Errorf("read bookshelf EPUB entry: %w", err)
		}
		content, readErr := io.ReadAll(io.LimitReader(reader, maxEntryBytes+1))
		closeErr := reader.Close()
		if readErr != nil {
			return fmt.Errorf("read bookshelf EPUB entry: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close bookshelf EPUB entry: %w", closeErr)
		}
		if int64(len(content)) != int64(entry.UncompressedSize64) {
			return fmt.Errorf("truncated bookshelf EPUB entry")
		}
		expanded += int64(len(content))
		if entry.Name == "mimetype" {
			validMimetype = string(content) == "application/epub+zip"
		}
		paths[entry.Name] = struct{}{}
		if entry.Name == "META-INF/container.xml" {
			container, err := parseXMLTree(content, maxMetadataXMLBytes)
			if err != nil {
				return fmt.Errorf("invalid bookshelf EPUB container: %w", err)
			}
			for _, rootfile := range findNodes(container, "rootfile") {
				fullPath := attrLocal(rootfile, "full-path")
				cleanRootfile := path.Clean(fullPath)
				if attrLocal(rootfile, "media-type") == "application/oebps-package+xml" && cleanRootfile == fullPath && !strings.HasPrefix(cleanRootfile, "/") && cleanRootfile != ".." && !strings.HasPrefix(cleanRootfile, "../") {
					containerTargets = append(containerTargets, cleanRootfile)
				}
			}
		}
	}
	for _, target := range containerTargets {
		if _, ok := paths[target]; ok && strings.HasSuffix(strings.ToLower(target), ".opf") {
			containerValid = true
		}
	}
	if !validMimetype || !containerValid || expanded > maxExpandedBytes {
		return fmt.Errorf("invalid bookshelf EPUB mimetype or expanded size")
	}
	return nil
}
