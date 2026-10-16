package bookshelf

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"aurago/internal/dbutil"
	"aurago/internal/fileutil"
	_ "modernc.org/sqlite"
)

const schemaVersion = 6

type prepCall struct {
	done chan struct{}
	err  error
	key  string
	path string
	size int64
	rev  string
}

type Service struct {
	db            *sql.DB
	root          string
	cacheDir      string
	coversDir     string
	renditionsDir string
	adapters      map[string]SourceAdapter
	maxFiles      int
	maxBookBytes  atomic.Int64
	maxCacheBytes int64
	minFreeBytes  int64
	mu            sync.Mutex
	cacheMu       sync.Mutex
	coverMu       sync.Mutex
	renditionMu   sync.Mutex
	reservedBytes int64
	closed        bool
	activeLeases  map[string]int
	preparing     map[string]*prepCall
	sourceLocks   map[string]*sync.Mutex
}

func New(ctx context.Context, opts Options) (*Service, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.Root) == "" {
		return nil, errors.New("bookshelf: data root is required")
	}
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolve bookshelf data root: %w", err)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create bookshelf data root: %w", err)
	}
	for _, dir := range []string{"cache", "covers", "renditions"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			return nil, fmt.Errorf("create bookshelf %s directory: %w", dir, err)
		}
	}
	if entries, readErr := os.ReadDir(filepath.Join(root, "cache")); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), ".prepare-") && strings.HasSuffix(entry.Name(), ".tmp") {
				_ = os.Remove(filepath.Join(root, "cache", entry.Name()))
			}
		}
	}
	if entries, readErr := os.ReadDir(filepath.Join(root, "renditions")); readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), ".stage-") && strings.HasSuffix(entry.Name(), ".tmp") {
				_ = os.Remove(filepath.Join(root, "renditions", entry.Name()))
			}
		}
	}
	dbPath := filepath.Join(root, "catalog.sqlite")
	db, err := dbutil.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open bookshelf catalog: %w", err)
	}
	s := &Service{
		db: db, root: root, cacheDir: filepath.Join(root, "cache"), coversDir: filepath.Join(root, "covers"), renditionsDir: filepath.Join(root, "renditions"),
		adapters: cloneAdapters(opts.Adapters), maxFiles: opts.MaxFilesPerScan,
		maxCacheBytes: opts.MaxCacheBytes, minFreeBytes: opts.MinFreeBytes,
		activeLeases: make(map[string]int), preparing: make(map[string]*prepCall), sourceLocks: make(map[string]*sync.Mutex),
	}
	if s.maxFiles <= 0 || s.maxFiles > 100_000 {
		s.maxFiles = 100_000
	}
	maxBookBytes := opts.MaxBookBytes
	if maxBookBytes <= 0 || maxBookBytes > MaxSourceBookBytes {
		maxBookBytes = MaxSourceBookBytes
	}
	s.maxBookBytes.Store(maxBookBytes)
	if s.maxCacheBytes <= 0 {
		s.maxCacheBytes = 1 << 30
	}
	if s.minFreeBytes <= 0 {
		s.minFreeBytes = 64 << 20
	}
	if err := s.migrate(ctx, dbPath); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func cloneAdapters(adapters map[string]SourceAdapter) map[string]SourceAdapter {
	copy := make(map[string]SourceAdapter, len(adapters))
	for key, adapter := range adapters {
		copy[strings.ToLower(strings.TrimSpace(key))] = adapter
	}
	return copy
}

func (s *Service) migrate(ctx context.Context, dbPath string) error {
	version, err := dbutil.GetUserVersion(s.db)
	if err != nil {
		return err
	}
	if version > schemaVersion {
		return fmt.Errorf("bookshelf catalog schema %d is newer than supported version %d", version, schemaVersion)
	}
	if version == schemaVersion {
		return nil
	}
	if version > 0 && dbPath != ":memory:" {
		backup := fmt.Sprintf("%s.v%d.%s.bak", dbPath, version, time.Now().UTC().Format("20060102T150405.000000000"))
		if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, backup); err != nil {
			return fmt.Errorf("back up bookshelf catalog before migration: %w", err)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bookshelf migration: %w", err)
	}
	defer tx.Rollback()
	if version == 0 {
		for _, statement := range []string{
			`CREATE TABLE sources (id TEXT PRIMARY KEY, name TEXT NOT NULL, provider TEXT NOT NULL, mount_id TEXT NOT NULL DEFAULT '', path TEXT NOT NULL DEFAULT '', recursive INTEGER NOT NULL DEFAULT 0, host TEXT NOT NULL DEFAULT '', share TEXT NOT NULL DEFAULT '', username TEXT NOT NULL DEFAULT '', domain TEXT NOT NULL DEFAULT '', generation INTEGER NOT NULL DEFAULT 1, refresh_minutes INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL)`,
			`CREATE TABLE books (id TEXT PRIMARY KEY, source_id TEXT NOT NULL, path TEXT NOT NULL, content_revision TEXT NOT NULL DEFAULT '', size INTEGER NOT NULL DEFAULT 0, format TEXT NOT NULL, title TEXT NOT NULL, author TEXT NOT NULL DEFAULT '', series TEXT NOT NULL DEFAULT '', description TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '[]', cover_file TEXT NOT NULL DEFAULT '', available INTEGER NOT NULL DEFAULT 1, revision INTEGER NOT NULL DEFAULT 1, scan_token TEXT NOT NULL DEFAULT '', last_scanned_at TEXT NOT NULL, reading_state TEXT NOT NULL DEFAULT 'unread', reading_state_manual INTEGER NOT NULL DEFAULT 0, UNIQUE(source_id,path))`,
			`CREATE TABLE metadata_overrides (book_id TEXT PRIMARY KEY, title TEXT, author TEXT, series TEXT, description TEXT, tags TEXT, FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE)`,
			`CREATE TABLE collections (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE COLLATE NOCASE, position INTEGER NOT NULL DEFAULT 0, revision INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL)`,
			`CREATE TABLE book_collections (book_id TEXT NOT NULL, collection_id TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(book_id,collection_id), FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE, FOREIGN KEY(collection_id) REFERENCES collections(id) ON DELETE CASCADE)`,
			`CREATE TABLE progress (book_id TEXT PRIMARY KEY, revision INTEGER NOT NULL DEFAULT 1, rendition_revision TEXT NOT NULL DEFAULT '', locator TEXT NOT NULL DEFAULT 'null', fraction REAL NOT NULL DEFAULT 0, updated_at TEXT NOT NULL, FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE)`,
			`CREATE TABLE annotations (id TEXT PRIMARY KEY, book_id TEXT NOT NULL, kind TEXT NOT NULL, locator TEXT NOT NULL, text TEXT NOT NULL DEFAULT '', color TEXT NOT NULL DEFAULT '', rendition_revision TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE)`,
			`CREATE TABLE cache_pins (book_id TEXT PRIMARY KEY, cache_key TEXT NOT NULL, content_revision TEXT NOT NULL, source_generation INTEGER NOT NULL, size INTEGER NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE)`,
			`CREATE INDEX books_source_available ON books(source_id,available)`,
			`CREATE INDEX books_title ON books(title COLLATE NOCASE)`,
			`CREATE INDEX book_collections_order ON book_collections(collection_id,position)`,
			`CREATE INDEX annotations_book ON annotations(book_id,created_at)`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("create bookshelf catalog schema: %w", err)
			}
		}
	} else if version > 0 && version < 3 {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE books ADD COLUMN reading_state TEXT NOT NULL DEFAULT 'unread'`); err != nil {
			return fmt.Errorf("add bookshelf reading state: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `ALTER TABLE annotations ADD COLUMN rendition_revision TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("bind bookshelf annotations to rendition: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS cache_pins (book_id TEXT PRIMARY KEY, cache_key TEXT NOT NULL, content_revision TEXT NOT NULL, source_generation INTEGER NOT NULL, size INTEGER NOT NULL, updated_at TEXT NOT NULL, FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE)`); err != nil {
			return fmt.Errorf("create bookshelf cache pins: %w", err)
		}
	}
	if version > 0 && version < 4 {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE books ADD COLUMN reading_state_manual INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add bookshelf manual reading state: %w", err)
		}
	}
	if version > 0 && version < 5 {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE sources ADD COLUMN refresh_minutes INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add bookshelf source refresh interval: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS renditions (book_id TEXT NOT NULL, format TEXT NOT NULL, base_revision TEXT NOT NULL, revision TEXT NOT NULL, size INTEGER NOT NULL, path TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(book_id,format,base_revision), FOREIGN KEY(book_id) REFERENCES books(id) ON DELETE CASCADE)`); err != nil {
		return fmt.Errorf("create bookshelf rendition schema: %w", err)
	}
	if hasColumn, err := tableHasColumn(ctx, tx, "renditions", "converter_version"); err != nil {
		return fmt.Errorf("inspect bookshelf rendition schema: %w", err)
	} else if !hasColumn {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE renditions ADD COLUMN converter_version TEXT NOT NULL DEFAULT 'legacy'`); err != nil {
			return fmt.Errorf("add bookshelf rendition converter version: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS renditions_book ON renditions(book_id,format)`); err != nil {
		return fmt.Errorf("index bookshelf renditions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bookshelf migration: %w", err)
	}
	return nil
}

func tableHasColumn(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	if table != "renditions" && table != "books" && table != "sources" && table != "annotations" {
		return false, fmt.Errorf("unsupported bookshelf schema table")
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, typeName string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	return s.db.Close()
}

func (s *Service) checkOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	return nil
}

func (s *Service) sourceLock(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.sourceLocks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		s.sourceLocks[id] = lock
	}
	return lock
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate bookshelf id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *Service) freeSpaceFor(bytes int64) error {
	free, err := fileutil.FreeDiskBytes(s.root)
	if err != nil {
		return fmt.Errorf("check bookshelf cache free space: %w", err)
	}
	if free-bytes < s.minFreeBytes {
		return ErrInsufficientDisk
	}
	return nil
}

func nowText() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func parseTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}

func cleanRelativePath(value string, allowEmpty bool) (string, error) {
	windowsDrivePath := len(value) >= 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':'
	if strings.ContainsAny(value, "\x00\\") || strings.HasPrefix(value, "/") || filepath.VolumeName(value) != "" || windowsDrivePath {
		return "", ErrInvalidPath
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." && allowEmpty {
		return "", nil
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(clean) {
		return "", ErrInvalidPath
	}
	return clean, nil
}

func validateText(value string, max int) bool {
	return len(value) <= max && !strings.ContainsRune(value, '\x00') && utf8.ValidString(value)
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
