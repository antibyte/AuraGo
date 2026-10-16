package bookshelf

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

func (s *Service) AddSource(ctx context.Context, source Source) (Source, error) {
	if err := s.checkOpen(); err != nil {
		return Source{}, err
	}
	if source.ID == "" {
		id, err := newID()
		if err != nil {
			return Source{}, err
		}
		source.ID = id
	}
	source.Provider = strings.ToLower(strings.TrimSpace(source.Provider))
	source.Name = strings.TrimSpace(source.Name)
	var err error
	source.Path, err = cleanRelativePath(source.Path, true)
	if err != nil {
		return Source{}, err
	}
	if !validID(source.ID) || source.Name == "" || !validateText(source.Name, 200) || len(source.Path) > 4096 ||
		!validateText(source.Provider, 20) || !validateText(source.MountID, 256) ||
		!validateText(source.Host, 253) || !validateText(source.Share, 256) ||
		!validateText(source.Username, 256) || !validateText(source.Domain, 256) || !validRefreshMinutes(source.RefreshMinutes) {
		return Source{}, fmt.Errorf("invalid bookshelf source")
	}
	if _, ok := s.adapters[source.Provider]; !ok {
		return Source{}, ErrUnsupported
	}
	if source.Provider == ProviderSMB && (strings.TrimSpace(source.Host) == "" || strings.TrimSpace(source.Share) == "") {
		return Source{}, fmt.Errorf("invalid SMB source coordinates")
	}
	if source.Provider != ProviderSMB && (source.Host != "" || source.Share != "" || source.Username != "" || source.Domain != "") {
		return Source{}, fmt.Errorf("SMB coordinates are only valid for SMB sources")
	}
	source.Generation = 1
	_, err = s.db.ExecContext(ctx, `INSERT INTO sources(id,name,provider,mount_id,path,recursive,host,share,username,domain,generation,refresh_minutes,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, source.ID, source.Name, source.Provider, source.MountID, source.Path,
		boolInt(source.Recursive), source.Host, source.Share, source.Username, source.Domain, source.Generation, source.RefreshMinutes, nowText())
	if err != nil {
		return Source{}, fmt.Errorf("add bookshelf source: %w", err)
	}
	return source, nil
}

func (s *Service) UpdateSource(ctx context.Context, source Source) (Source, error) {
	if err := s.checkOpen(); err != nil {
		return Source{}, err
	}
	lock := s.sourceLock(source.ID)
	lock.Lock()
	defer lock.Unlock()
	current, err := s.GetSource(ctx, source.ID)
	if err != nil {
		return Source{}, err
	}
	if source.Generation != current.Generation {
		return Source{}, ErrConflict
	}
	source.Provider = strings.ToLower(strings.TrimSpace(source.Provider))
	source.Name = strings.TrimSpace(source.Name)
	source.Path, err = cleanRelativePath(source.Path, true)
	if err != nil {
		return Source{}, err
	}
	if source.Name == "" || !validateText(source.Name, 200) || !validID(source.ID) || len(source.Path) > 4096 {
		return Source{}, fmt.Errorf("invalid bookshelf source")
	}
	if _, ok := s.adapters[source.Provider]; !ok {
		return Source{}, ErrUnsupported
	}
	if source.Provider == ProviderSMB && (strings.TrimSpace(source.Host) == "" || strings.TrimSpace(source.Share) == "") {
		return Source{}, fmt.Errorf("invalid SMB source coordinates")
	}
	if !validateText(source.MountID, 256) || !validateText(source.Host, 253) || !validateText(source.Share, 256) || !validateText(source.Username, 256) || !validateText(source.Domain, 256) || !validRefreshMinutes(source.RefreshMinutes) {
		return Source{}, fmt.Errorf("invalid bookshelf source coordinates")
	}
	if source.Provider != ProviderSMB && (source.Host != "" || source.Share != "" || source.Username != "" || source.Domain != "") {
		return Source{}, fmt.Errorf("SMB coordinates are only valid for SMB sources")
	}
	identityChanged := source.Provider != current.Provider || source.MountID != current.MountID || source.Path != current.Path ||
		source.Host != current.Host || source.Share != current.Share || source.Username != current.Username || source.Domain != current.Domain
	source.Generation = current.Generation + 1
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Source{}, fmt.Errorf("begin bookshelf source update: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE sources SET name=?,provider=?,mount_id=?,path=?,recursive=?,host=?,share=?,username=?,domain=?,generation=?,refresh_minutes=? WHERE id=? AND generation=?`,
		source.Name, source.Provider, source.MountID, source.Path, boolInt(source.Recursive), source.Host,
		source.Share, source.Username, source.Domain, source.Generation, source.RefreshMinutes, source.ID, current.Generation)
	if err != nil {
		return Source{}, fmt.Errorf("update bookshelf source: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Source{}, checkSourceGeneration(ctx, tx, source.ID, current.Generation)
	}
	if identityChanged {
		// The same relative path can now name different content. Make old rows
		// unavailable until a fresh scan confirms them, clear revision-bound
		// reading state and renditions, and retain annotations as historical
		// records tied to their previous rendition revisions.
		if _, err := tx.ExecContext(ctx, `UPDATE books SET content_revision='',available=0,revision=revision+1,
			reading_state=CASE WHEN reading_state_manual=0 THEN 'unread' ELSE reading_state END WHERE source_id=?`, source.ID); err != nil {
			return Source{}, fmt.Errorf("invalidate books after bookshelf source change: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM progress WHERE book_id IN (SELECT id FROM books WHERE source_id=?)`, source.ID); err != nil {
			return Source{}, fmt.Errorf("retire progress after bookshelf source change: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM renditions WHERE book_id IN (SELECT id FROM books WHERE source_id=?)`, source.ID); err != nil {
			return Source{}, fmt.Errorf("retire renditions after bookshelf source change: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cache_pins WHERE book_id IN (SELECT id FROM books WHERE source_id=?)`, source.ID); err != nil {
		return Source{}, fmt.Errorf("retire bookshelf pins after source change: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Source{}, fmt.Errorf("commit bookshelf source update: %w", err)
	}
	return source, nil
}

func checkSourceGeneration(ctx context.Context, tx *sql.Tx, id string, expected int64) error {
	var generation int64
	if err := tx.QueryRowContext(ctx, `SELECT generation FROM sources WHERE id=?`, id).Scan(&generation); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("read bookshelf source generation: %w", err)
	}
	if generation != expected {
		return ErrConflict
	}
	return ErrNotFound
}

// ResolveLocal materializes a selected path using the source's read-only Stat
// adapter. It never reads or changes the source file contents.
func (s *Service) ResolveLocal(ctx context.Context, sourceID, relativePath string) (Book, error) {
	lock := s.sourceLock(sourceID)
	lock.Lock()
	defer lock.Unlock()
	source, err := s.GetSource(ctx, sourceID)
	if err != nil {
		return Book{}, err
	}
	if source.Provider != ProviderLocal {
		return Book{}, ErrUnsupported
	}
	clean, err := cleanRelativePath(relativePath, false)
	if err != nil {
		return Book{}, err
	}
	adapter := s.adapters[source.Provider]
	if adapter == nil {
		return Book{}, ErrUnsupported
	}
	entry, err := adapter.Stat(ctx, source, clean)
	if err != nil {
		return Book{}, fmt.Errorf("stat selected bookshelf file: %w", err)
	}
	if entry.IsDir {
		return Book{}, ErrUnsupported
	}
	entry.Path = clean
	return s.AddBook(ctx, sourceID, entry, Metadata{})
}

// FindLocal performs a read-only exact catalog lookup for a local source path.
func (s *Service) FindLocal(ctx context.Context, sourceID, relativePath string) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	source, err := s.GetSource(ctx, sourceID)
	if err != nil {
		return Book{}, err
	}
	if source.Provider != ProviderLocal {
		return Book{}, ErrUnsupported
	}
	clean, err := cleanRelativePath(relativePath, false)
	if err != nil {
		return Book{}, err
	}
	return s.GetBook(ctx, bookID(source.ID, clean))
}

// AddBook adds a selected source entry after a read-only metadata lookup.
func (s *Service) AddBook(ctx context.Context, sourceID string, entry Entry, metadata Metadata) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	source, err := s.GetSource(ctx, sourceID)
	if err != nil {
		return Book{}, err
	}
	entry.Path, err = cleanRelativePath(entry.Path, false)
	if err != nil {
		return Book{}, err
	}
	maxBookBytes := s.maxBookBytes.Load()
	if entry.Size > maxBookBytes {
		return Book{}, ErrLimitExceeded
	}
	if entry.IsDir || entry.Size < 0 || len(entry.Revision) > 1024 {
		return Book{}, fmt.Errorf("invalid bookshelf entry")
	}
	format := normalizeFormat(entry.Format, entry.Path)
	if !validFormat(format) {
		return Book{}, ErrUnsupported
	}
	if metadata.Title == "" {
		metadata.Title = strings.TrimSuffix(path.Base(entry.Path), filepath.Ext(entry.Path))
	}
	if metadata.Format == "" {
		metadata.Format = format
	}
	if len(metadata.Tags) > 128 {
		return Book{}, fmt.Errorf("invalid bookshelf metadata")
	}
	for _, tag := range metadata.Tags {
		if tag == "" || !validateText(tag, 128) {
			return Book{}, fmt.Errorf("invalid bookshelf metadata")
		}
	}
	metadata = normalizeMetadata(metadata)
	if err := validateMetadata(metadata); err != nil {
		return Book{}, err
	}
	id := bookID(source.ID, entry.Path)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback()
	var oldRevision int64
	var oldContentRevision string
	var oldSize int64
	var oldFormat string
	getErr := tx.QueryRowContext(ctx, `SELECT revision,content_revision,size,format FROM books WHERE id=?`, id).Scan(&oldRevision, &oldContentRevision, &oldSize, &oldFormat)
	newBook := errors.Is(getErr, sql.ErrNoRows)
	if getErr != nil && !newBook {
		return Book{}, fmt.Errorf("read existing bookshelf book: %w", getErr)
	}
	revision := int64(1)
	contentChanged := false
	if !newBook {
		revision = oldRevision
		contentChanged = oldSize != entry.Size || oldFormat != format || entry.Revision != "" && oldContentRevision != entry.Revision
		if contentChanged {
			revision++
		}
	}
	contentRevision := entry.Revision
	if !newBook && contentRevision == "" && !contentChanged {
		contentRevision = oldContentRevision
	}
	tags, _ := json.Marshal(metadata.Tags)
	_, err = tx.ExecContext(ctx, `INSERT INTO books(id,source_id,path,content_revision,size,format,title,author,series,description,tags,available,revision,last_scanned_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,1,?,?) ON CONFLICT(id) DO UPDATE SET content_revision=excluded.content_revision,size=excluded.size,format=excluded.format,
		title=CASE WHEN books.title='' THEN excluded.title ELSE books.title END,available=1,revision=excluded.revision,last_scanned_at=excluded.last_scanned_at`,
		id, source.ID, entry.Path, contentRevision, entry.Size, format, metadata.Title, metadata.Author, metadata.Series, metadata.Description, string(tags), revision, nowText())
	if err != nil {
		return Book{}, fmt.Errorf("add bookshelf book: %w", err)
	}
	if contentChanged {
		if _, err := tx.ExecContext(ctx, `UPDATE books SET reading_state='unread' WHERE id=? AND reading_state_manual=0`, id); err != nil {
			return Book{}, fmt.Errorf("reset derived bookshelf reading state: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM progress WHERE book_id=?`, id); err != nil {
			return Book{}, fmt.Errorf("retire stale bookshelf progress: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM renditions WHERE book_id=?`, id); err != nil {
			return Book{}, fmt.Errorf("retire stale bookshelf renditions: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM cache_pins WHERE book_id=?`, id); err != nil {
			return Book{}, fmt.Errorf("retire stale bookshelf cache pin: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Book{}, fmt.Errorf("commit bookshelf book: %w", err)
	}
	return s.GetBook(ctx, id)
}

func (s *Service) GetSource(ctx context.Context, id string) (Source, error) {
	if err := s.checkOpen(); err != nil {
		return Source{}, err
	}
	var source Source
	var recursive int
	err := s.db.QueryRowContext(ctx, `SELECT id,name,provider,mount_id,path,recursive,host,share,username,domain,generation,refresh_minutes FROM sources WHERE id=?`, id).Scan(
		&source.ID, &source.Name, &source.Provider, &source.MountID, &source.Path, &recursive,
		&source.Host, &source.Share, &source.Username, &source.Domain, &source.Generation, &source.RefreshMinutes)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, ErrNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("get bookshelf source: %w", err)
	}
	source.Recursive = recursive != 0
	return source, nil
}

func (s *Service) ListSources(ctx context.Context) ([]Source, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,provider,mount_id,path,recursive,host,share,username,domain,generation,refresh_minutes FROM sources ORDER BY name COLLATE NOCASE,id`)
	if err != nil {
		return nil, fmt.Errorf("list bookshelf sources: %w", err)
	}
	defer rows.Close()
	var sources []Source
	for rows.Next() {
		var source Source
		var recursive int
		if err := rows.Scan(&source.ID, &source.Name, &source.Provider, &source.MountID, &source.Path,
			&recursive, &source.Host, &source.Share, &source.Username, &source.Domain, &source.Generation, &source.RefreshMinutes); err != nil {
			return nil, fmt.Errorf("scan bookshelf source: %w", err)
		}
		source.Recursive = recursive != 0
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Service) RemoveSource(ctx context.Context, id string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	lock := s.sourceLock(id)
	lock.Lock()
	defer lock.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bookshelf source removal: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE books SET available=0,revision=revision+1 WHERE source_id=? AND available=1`, id); err != nil {
		return fmt.Errorf("mark bookshelf books unavailable: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM cache_pins WHERE book_id IN (SELECT id FROM books WHERE source_id=?)`, id); err != nil {
		return fmt.Errorf("clear bookshelf pins for removed source: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM sources WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("remove bookshelf source: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// RemoveBook deletes only Bookshelf catalog data. It never calls a source write method.
func (s *Service) RemoveBook(ctx context.Context, id string, expectedRevision int64) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM books WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return fmt.Errorf("remove bookshelf book: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return checkRevision(ctx, s.db, id)
	}
	return nil
}

// SetReadingState changes the user's explicit catalog status using the same
// book revision used by metadata and collection editors.
func (s *Service) SetReadingState(ctx context.Context, bookID string, expectedBookRevision int64, state string) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if !validID(bookID) || !validReadingState(state) {
		return Book{}, fmt.Errorf("invalid bookshelf reading state")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE books SET reading_state=?,reading_state_manual=1,revision=revision+1 WHERE id=? AND revision=?`, state, bookID, expectedBookRevision)
	if err != nil {
		return Book{}, fmt.Errorf("set bookshelf reading state: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Book{}, checkRevision(ctx, s.db, bookID)
	}
	return s.GetBook(ctx, bookID)
}

func validReadingState(state string) bool {
	return state == "unread" || state == "reading" || state == "finished"
}

func validRefreshMinutes(minutes int) bool {
	return minutes == 0 || minutes >= 15 && minutes <= 7*24*60
}

func (s *Service) Query(ctx context.Context, query Query) (Page, error) {
	if err := s.checkOpen(); err != nil {
		return Page{}, err
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	if query.SourceID != "" && !validID(query.SourceID) || query.CollectionID != "" && !validID(query.CollectionID) {
		return Page{}, fmt.Errorf("invalid bookshelf query filter")
	}
	query.Sort = strings.ToLower(strings.TrimSpace(query.Sort))
	if query.Sort == "" {
		if query.CollectionID != "" {
			query.Sort = "collection"
		} else {
			query.Sort = "title"
		}
	}
	sortSQL := map[string]string{
		"title":    `COALESCE(o.title,b.title) COLLATE NOCASE,b.id`,
		"author":   `COALESCE(o.author,b.author) COLLATE NOCASE,COALESCE(o.title,b.title) COLLATE NOCASE,b.id`,
		"recent":   `COALESCE(p.updated_at,'') DESC,COALESCE(o.title,b.title) COLLATE NOCASE,b.id`,
		"progress": `COALESCE(p.fraction,0) DESC,COALESCE(p.updated_at,'') DESC,COALESCE(o.title,b.title) COLLATE NOCASE,b.id`,
	}
	orderBy, ok := sortSQL[query.Sort]
	collectionSort := query.Sort == "collection"
	if collectionSort && query.CollectionID != "" {
		orderBy, ok = `(SELECT position FROM book_collections WHERE collection_id=? AND book_id=b.id),b.id`, true
	}
	if !ok {
		return Page{}, fmt.Errorf("invalid bookshelf sort")
	}
	if collectionSort && query.CollectionID == "" {
		return Page{}, fmt.Errorf("collection sort requires a collection filter")
	}
	query.Status = strings.ToLower(strings.TrimSpace(query.Status))
	if query.Status == "all" {
		query.Status = ""
	}
	if query.Status != "" && !validReadingState(query.Status) {
		return Page{}, fmt.Errorf("invalid bookshelf status")
	}
	if query.Limit <= 0 {
		query.Limit = 50
	}
	if query.Limit > 200 {
		query.Limit = 200
	}
	query.Q = strings.TrimSpace(query.Q)
	if len(query.Q) > 256 {
		query.Q = query.Q[:256]
	}
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query.Q) + "%"
	filter := `(?='' OR b.source_id=?) AND (?='' OR EXISTS (SELECT 1 FROM book_collections bc WHERE bc.book_id=b.id AND bc.collection_id=?)) AND (?='' OR b.reading_state=?) AND (?='' OR lower(COALESCE(o.title,b.title)||' '||COALESCE(o.author,b.author)||' '||COALESCE(o.series,b.series)||' '||COALESCE(o.tags,b.tags)) LIKE lower(?) ESCAPE '\')`
	args := []any{query.SourceID, query.SourceID, query.CollectionID, query.CollectionID, query.Status, query.Status, query.Q, pattern}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM books b LEFT JOIN metadata_overrides o ON o.book_id=b.id WHERE `+filter, args...).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count bookshelf books: %w", err)
	}
	queryArgs := append([]any(nil), args...)
	if collectionSort {
		queryArgs = append(queryArgs, query.CollectionID)
	}
	queryArgs = append(queryArgs, query.Limit, query.Offset)
	rows, err := s.db.QueryContext(ctx, `SELECT `+bookColumns+` FROM books b
		LEFT JOIN metadata_overrides o ON o.book_id=b.id LEFT JOIN progress p ON p.book_id=b.id
		WHERE `+filter+` ORDER BY `+orderBy+` LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return Page{}, fmt.Errorf("query bookshelf books: %w", err)
	}
	defer rows.Close()
	page := Page{Books: make([]Book, 0, query.Limit), Total: total}
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return Page{}, fmt.Errorf("scan bookshelf book: %w", err)
		}
		if err := s.loadBookCollections(ctx, &book); err != nil {
			return Page{}, err
		}
		if err := s.loadBookRenditions(ctx, &book); err != nil {
			return Page{}, err
		}
		page.Books = append(page.Books, book)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	return page, nil
}

func (s *Service) GetBook(ctx context.Context, id string) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	book, err := scanBook(s.db.QueryRowContext(ctx, `SELECT `+bookColumns+` FROM books b LEFT JOIN metadata_overrides o ON o.book_id=b.id LEFT JOIN progress p ON p.book_id=b.id WHERE b.id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	}
	if err != nil {
		return Book{}, fmt.Errorf("get bookshelf book: %w", err)
	}
	if err := s.loadBookCollections(ctx, &book); err != nil {
		return Book{}, err
	}
	if err := s.loadBookRenditions(ctx, &book); err != nil {
		return Book{}, err
	}
	return book, nil
}

const bookColumns = `b.id,b.source_id,b.path,b.content_revision,b.size,b.format,b.title,b.author,b.series,b.description,b.tags,b.cover_file,b.available,b.revision,b.last_scanned_at,
	b.reading_state,b.reading_state_manual,EXISTS(SELECT 1 FROM cache_pins cp JOIN sources ps ON ps.id=b.source_id WHERE cp.book_id=b.id AND cp.content_revision=b.content_revision AND cp.source_generation=ps.generation),o.title,o.author,o.series,o.description,o.tags,p.revision,p.rendition_revision,p.locator,p.fraction,p.updated_at`

type rowScanner interface{ Scan(...any) error }

func scanBook(row rowScanner) (Book, error) {
	var book Book
	var available int
	var readingStateManual int
	var cachePinned int
	var tags string
	var coverFile string
	var scanned string
	var overrideTitle, overrideAuthor, overrideSeries, overrideDescription, overrideTags sql.NullString
	var progressRevision sql.NullInt64
	var renditionRevision, locator, updated sql.NullString
	var fraction sql.NullFloat64
	err := row.Scan(&book.ID, &book.SourceID, &book.Path, &book.ContentRevision, &book.Size, &book.Format,
		&book.Title, &book.Author, &book.Series, &book.Description, &tags, &coverFile, &available, &book.Revision, &scanned,
		&book.ReadingState, &readingStateManual, &cachePinned,
		&overrideTitle, &overrideAuthor, &overrideSeries, &overrideDescription, &overrideTags,
		&progressRevision, &renditionRevision, &locator, &fraction, &updated)
	if err != nil {
		return Book{}, err
	}
	book.Available = available != 0
	book.ReadingStateManual = readingStateManual != 0
	book.CachePinned = cachePinned != 0
	book.LastScannedAt = parseTime(scanned)
	_ = json.Unmarshal([]byte(tags), &book.Tags)
	if overrideTitle.Valid {
		book.Overrides.Title = &overrideTitle.String
		book.Title = overrideTitle.String
	}
	if overrideAuthor.Valid {
		book.Overrides.Author = &overrideAuthor.String
		book.Author = overrideAuthor.String
	}
	if overrideSeries.Valid {
		book.Overrides.Series = &overrideSeries.String
		book.Series = overrideSeries.String
	}
	if overrideDescription.Valid {
		book.Overrides.Description = &overrideDescription.String
		book.Description = overrideDescription.String
	}
	if overrideTags.Valid {
		book.Overrides.Tags = new([]string)
		_ = json.Unmarshal([]byte(overrideTags.String), book.Overrides.Tags)
		book.Tags = *book.Overrides.Tags
	}
	if coverFile != "" {
		book.CoverURL = "/api/desktop/bookshelf/books/" + book.ID + "/cover"
	}
	if progressRevision.Valid {
		book.Progress = &Progress{Revision: progressRevision.Int64, Fraction: fraction.Float64}
		book.Progress.RenditionRevision = renditionRevision.String
		if locator.Valid {
			book.Progress.Locator = json.RawMessage(locator.String)
		}
		book.Progress.UpdatedAt = parseTime(updated.String)
	}
	book.CoverFallback = FallbackCover(book)
	return book, nil
}

func (s *Service) loadBookCollections(ctx context.Context, book *Book) error {
	rows, err := s.db.QueryContext(ctx, `SELECT collection_id FROM book_collections WHERE book_id=? ORDER BY position,collection_id`, book.ID)
	if err != nil {
		return fmt.Errorf("load bookshelf book collections: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		book.Collections = append(book.Collections, id)
	}
	return rows.Err()
}

func (s *Service) UpdateMetadata(ctx context.Context, id string, expectedRevision int64, metadata Metadata) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	if len(metadata.Tags) > 128 {
		return Book{}, fmt.Errorf("invalid bookshelf metadata")
	}
	for _, tag := range metadata.Tags {
		if tag == "" || !validateText(tag, 128) {
			return Book{}, fmt.Errorf("invalid bookshelf metadata")
		}
	}
	metadata = normalizeMetadata(metadata)
	if err := validateMetadata(metadata); err != nil {
		return Book{}, err
	}
	tags, _ := json.Marshal(metadata.Tags)
	res, err := s.db.ExecContext(ctx, `UPDATE books SET title=?,author=?,series=?,description=?,format=?,tags=?,revision=revision+1 WHERE id=? AND revision=?`,
		metadata.Title, metadata.Author, metadata.Series, metadata.Description, metadata.Format, string(tags), id, expectedRevision)
	if err != nil {
		return Book{}, fmt.Errorf("update bookshelf metadata: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Book{}, checkRevision(ctx, s.db, id)
	}
	return s.GetBook(ctx, id)
}

func (s *Service) UpdateMetadataOverride(ctx context.Context, id string, expectedRevision int64, override MetadataOverride) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	if err := validateOverride(override); err != nil {
		return Book{}, err
	}
	tags := sql.NullString{}
	if override.Tags != nil {
		encoded, _ := json.Marshal(*override.Tags)
		tags = sql.NullString{String: string(encoded), Valid: true}
	}
	var title, author, series, description sql.NullString
	if override.Title != nil {
		title = sql.NullString{String: *override.Title, Valid: true}
	}
	if override.Author != nil {
		author = sql.NullString{String: *override.Author, Valid: true}
	}
	if override.Series != nil {
		series = sql.NullString{String: *override.Series, Valid: true}
	}
	if override.Description != nil {
		description = sql.NullString{String: *override.Description, Valid: true}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE books SET revision=revision+1 WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return Book{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Book{}, checkRevision(ctx, tx, id)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO metadata_overrides(book_id,title,author,series,description,tags) VALUES(?,?,?,?,?,?)
		ON CONFLICT(book_id) DO UPDATE SET title=COALESCE(excluded.title,metadata_overrides.title),author=COALESCE(excluded.author,metadata_overrides.author),
			series=COALESCE(excluded.series,metadata_overrides.series),description=COALESCE(excluded.description,metadata_overrides.description),tags=COALESCE(excluded.tags,metadata_overrides.tags)`,
		id, title, author, series, description, tags); err != nil {
		return Book{}, fmt.Errorf("save bookshelf metadata override: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Book{}, err
	}
	return s.GetBook(ctx, id)
}

func (s *Service) ResetMetadataOverrides(ctx context.Context, id string, expectedRevision int64) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE books SET revision=revision+1 WHERE id=? AND revision=?`, id, expectedRevision)
	if err != nil {
		return Book{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Book{}, checkRevision(ctx, tx, id)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM metadata_overrides WHERE book_id=?`, id); err != nil {
		return Book{}, err
	}
	if err := tx.Commit(); err != nil {
		return Book{}, err
	}
	return s.GetBook(ctx, id)
}

func checkRevision(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) error {
	var revision int64
	err := db.QueryRowContext(ctx, `SELECT revision FROM books WHERE id=?`, id).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrConflict
}

func normalizeMetadata(metadata Metadata) Metadata {
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.Author = strings.TrimSpace(metadata.Author)
	metadata.Series = strings.TrimSpace(metadata.Series)
	metadata.Description = strings.TrimSpace(metadata.Description)
	metadata.Format = strings.ToLower(strings.TrimSpace(metadata.Format))
	metadata.Tags = normalizeTags(metadata.Tags)
	return metadata
}

func validateMetadata(metadata Metadata) error {
	if metadata.Title == "" || !validateText(metadata.Title, 2048) || !validateText(metadata.Author, 512) ||
		!validateText(metadata.Series, 512) || !validateText(metadata.Description, 32_768) || !validFormat(metadata.Format) || len(metadata.Tags) > 128 {
		return fmt.Errorf("invalid bookshelf metadata")
	}
	for _, tag := range metadata.Tags {
		if tag == "" || !validateText(tag, 128) {
			return fmt.Errorf("invalid bookshelf metadata")
		}
	}
	return nil
}

func validateOverride(override MetadataOverride) error {
	for _, field := range []*string{override.Title, override.Author, override.Series, override.Description} {
		if field != nil && !validateText(*field, 32_768) {
			return fmt.Errorf("invalid bookshelf metadata override")
		}
	}
	if override.Title != nil && len(*override.Title) > 2048 || override.Author != nil && len(*override.Author) > 512 || override.Series != nil && len(*override.Series) > 512 {
		return fmt.Errorf("invalid bookshelf metadata override")
	}
	if override.Tags != nil && len(*override.Tags) > 128 {
		return fmt.Errorf("invalid bookshelf metadata override")
	}
	if override.Tags != nil {
		for _, tag := range *override.Tags {
			if tag == "" || !validateText(tag, 128) {
				return fmt.Errorf("invalid bookshelf metadata override")
			}
		}
	}
	return nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || !validateText(tag, 128) {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func validID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func validFormat(format string) bool {
	switch format {
	case "epub", "pdf", "mobi", "azw", "azw3", "kfx", "fb2", "fb2.zip", "cbz", "cbr", "txt", "html", "htm", "rtf", "docx", "odt", "djvu", "acsm":
		return true
	default:
		return false
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func bookID(sourceID, path string) string {
	sum := sha256.Sum256([]byte(sourceID + "\x00" + filepath.ToSlash(path)))
	return hex.EncodeToString(sum[:16])
}
