package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/fileutil"
	"modernc.org/sqlite"
)

// MemorySnapshotCoordinator excludes snapshots from complete cross-store
// publications. SQLite reservations also cover callers writing directly to SQL.
// Neither acquiring nor holding a freeze requires an application store mutex.
type MemorySnapshotCoordinator struct {
	mu      sync.Mutex
	changed chan struct{}
	writers int
	frozen  bool
}

var memorySnapshotCoordinators sync.Map

func snapshotCoordinator(key string) *MemorySnapshotCoordinator {
	c, _ := memorySnapshotCoordinators.LoadOrStore(key, &MemorySnapshotCoordinator{changed: make(chan struct{})})
	return c.(*MemorySnapshotCoordinator)
}

func snapshotPathKey(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

func snapshotSQLiteURI(path, query string) string {
	abs, _ := filepath.Abs(path)
	value := filepath.ToSlash(abs)
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return (&url.URL{Scheme: "file", Path: value, RawQuery: query}).String()
}

func (s *SQLiteMemory) snapshotCoordinator(contexts ...context.Context) (*MemorySnapshotCoordinator, error) {
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	var path string
	if err := s.db.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&path); err != nil {
		return nil, fmt.Errorf("locate memory snapshot coordinator: %w", err)
	}
	if path == "" {
		return snapshotCoordinator(fmt.Sprintf("memory:%p", s.db)), nil
	}
	return snapshotCoordinator(snapshotPathKey(path)), nil
}

func (c *MemorySnapshotCoordinator) signalLocked() { close(c.changed); c.changed = make(chan struct{}) }

func (c *MemorySnapshotCoordinator) acquire(ctx context.Context) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		if !c.frozen {
			c.writers++
			c.mu.Unlock()
			var once sync.Once
			return func() { once.Do(func() { c.mu.Lock(); c.writers--; c.signalLocked(); c.mu.Unlock() }) }, nil
		}
		changed := c.changed
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

func (c *MemorySnapshotCoordinator) freeze(ctx context.Context) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.mu.Lock()
		if !c.frozen {
			c.frozen = true
			c.mu.Unlock()
			break
		}
		changed := c.changed
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
	var once sync.Once
	release := func() { once.Do(func() { c.mu.Lock(); c.frozen = false; c.signalLocked(); c.mu.Unlock() }) }
	for {
		c.mu.Lock()
		active, changed := c.writers, c.changed
		c.mu.Unlock()
		if err := ctx.Err(); err != nil {
			release()
			return nil, err
		}
		if active == 0 {
			return release, nil
		}
		select {
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

type memoryLeaseKey struct{}
type memorySnapshotLease struct {
	coordinator *MemorySnapshotCoordinator
	active      atomic.Bool
}

// BeginMemoryPublication must precede the first local SQL/index lock. Nested
// calls carry the returned context, never reacquiring a lease behind a freeze.
func (s *SQLiteMemory) BeginMemoryPublication(ctx context.Context) (context.Context, func(), error) {
	if ctx == nil {
		ctx = context.Background()
	}
	c, err := s.snapshotCoordinator(ctx)
	if err != nil {
		return ctx, nil, err
	}
	if lease, ok := ctx.Value(memoryLeaseKey{}).(*memorySnapshotLease); ok && lease.coordinator == c && lease.active.Load() {
		return ctx, func() {}, ctx.Err()
	}
	release, err := c.acquire(ctx)
	if err != nil {
		return ctx, nil, err
	}
	lease := &memorySnapshotLease{coordinator: c}
	lease.active.Store(true)
	return context.WithValue(ctx, memoryLeaseKey{}, lease), func() {
		if lease.active.Swap(false) {
			release()
		}
	}, nil
}

// FreezeMemorySnapshots reserves every source before pinning the read snapshots.
// This also fences standalone SQL mutations without wrapping every sql.DB call.
// Callers release after pinning, before copying, hashing or archive encoding.
func FreezeMemorySnapshots(ctx context.Context, cfg *config.Config) (func(), error) {
	if cfg == nil {
		return nil, fmt.Errorf("memory snapshot configuration is required")
	}
	c := snapshotCoordinator(snapshotPathKey(cfg.SQLite.ShortTermPath))
	unfreeze, err := c.freeze(ctx)
	if err != nil {
		return nil, err
	}
	type reservation struct {
		db   *sql.DB
		conn *sql.Conn
	}
	var reservations []reservation
	var once sync.Once
	release := func() {
		once.Do(func() {
			for i := len(reservations) - 1; i >= 0; i-- {
				r := reservations[i]
				_, _ = r.conn.ExecContext(context.Background(), `ROLLBACK`)
				_ = r.conn.Close()
				_ = r.db.Close()
			}
			unfreeze()
		})
	}
	paths := []string{cfg.SQLite.ShortTermPath, cfg.SQLite.KnowledgeGraphPath, config.DocumentCorpusPath(cfg)}
	sort.Strings(paths)
	seen := map[string]bool{}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			release()
			return nil, fmt.Errorf("all three memory databases are required")
		}
		key := snapshotPathKey(path)
		if seen[key] {
			release()
			return nil, fmt.Errorf("memory snapshot databases must have distinct paths")
		}
		seen[key] = true
		if err := rejectSnapshotSymlinks(path); err != nil {
			release()
			return nil, err
		}
		db, err := sql.Open("sqlite", snapshotSQLiteURI(path, "mode=rw&_pragma=busy_timeout(100)"))
		if err != nil {
			release()
			return nil, err
		}
		conn, err := db.Conn(ctx)
		if err != nil {
			db.Close()
			release()
			return nil, err
		}
		for {
			_, err = conn.ExecContext(ctx, `BEGIN IMMEDIATE`)
			var sqliteErr *sqlite.Error
			if !errors.As(err, &sqliteErr) || sqliteErr.Code()&255 != 5 {
				break
			}
			select {
			case <-ctx.Done():
				err = ctx.Err()
			case <-time.After(10 * time.Millisecond):
				continue
			}
			break
		}
		if err != nil {
			conn.Close()
			db.Close()
			release()
			return nil, fmt.Errorf("reserve memory snapshot source: %w", err)
		}
		reservations = append(reservations, reservation{db, conn})
	}
	// Only explicit publication journals are checked; legacy metadata gaps are
	// not guessed at or repaired by the backup path.
	for _, r := range reservations {
		var hasLedger bool
		if err := r.conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='memory_maintenance_meta')`).Scan(&hasLedger); err != nil {
			release()
			return nil, err
		}
		if hasLedger {
			var pending int
			if err := r.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_maintenance_meta WHERE key LIKE 'snapshot.publication.%' OR key LIKE 'file_index.replacement.%'`).Scan(&pending); err != nil {
				release()
				return nil, err
			}
			if pending != 0 {
				release()
				return nil, fmt.Errorf("memory snapshot blocked by %d unfinished publications", pending)
			}
		}
	}
	return release, nil
}

const memorySnapshotConsistency = "coordinated-three-db-read-snapshot-v1"

func createPrivateSnapshotFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
}

// CaptureMemorySnapshots pins all three WAL read transactions at one quiescent
// point. Writers resume BEFORE copying: even a large backup cannot exhaust their
// SQLite busy timeout. The online backup API reads these pinned connections.
func CaptureMemorySnapshots(ctx context.Context, cfg *config.Config, destinations map[string]string) (outErr error) {
	if len(destinations) != 3 {
		return fmt.Errorf("three memory snapshot destinations are required")
	}
	release, err := FreezeMemorySnapshots(ctx, cfg)
	if err != nil {
		return err
	}
	defer release()
	type pinnedSource struct {
		name string
		db   *sql.DB
		conn *sql.Conn
	}
	var pinned []pinnedSource
	defer func() {
		for _, source := range pinned {
			_, _ = source.conn.ExecContext(context.Background(), `ROLLBACK`)
			_ = source.conn.Close()
			_ = source.db.Close()
		}
	}()
	sources := map[string]string{"short_term.sqlite": cfg.SQLite.ShortTermPath, "knowledge_graph.sqlite": cfg.SQLite.KnowledgeGraphPath, "documents.sqlite": config.DocumentCorpusPath(cfg)}
	for _, name := range []string{"short_term.sqlite", "knowledge_graph.sqlite", "documents.sqlite"} {
		if destinations[name] == "" {
			return fmt.Errorf("missing memory snapshot destination %s", name)
		}
		db, err := sql.Open("sqlite", snapshotSQLiteURI(sources[name], "mode=ro"))
		if err != nil {
			return err
		}
		conn, err := db.Conn(ctx)
		if err != nil {
			db.Close()
			return err
		}
		pinned = append(pinned, pinnedSource{name, db, conn})
		if _, err := conn.ExecContext(ctx, `BEGIN`); err != nil {
			return err
		}
		var count int
		// BEGIN alone is deferred: the first read actually establishes the WAL snapshot.
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master`).Scan(&count); err != nil {
			return err
		}
	}
	release()
	for _, source := range pinned {
		if err := ctx.Err(); err != nil {
			return err
		}
		dst := destinations[source.name]
		if err := rejectSnapshotSymlinks(dst); err != nil {
			return err
		}
		// Never replace an earlier snapshot. Restrict before the first page write.
		file, err := createPrivateSnapshotFile(dst)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := fileutil.RestrictToOwner(dst); err != nil {
			return err
		}
		err = source.conn.Raw(func(raw any) (err error) {
			backuper, ok := raw.(interface {
				NewBackup(string) (*sqlite.Backup, error)
			})
			if !ok {
				return fmt.Errorf("SQLite online backup is unavailable")
			}
			backup, err := backuper.NewBackup(snapshotSQLiteURI(dst, "mode=rw"))
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, backup.Finish()) }()
			for {
				if err := ctx.Err(); err != nil {
					return err
				}
				more, err := backup.Step(256)
				if err != nil || !more {
					return err
				}
			}
		})
		if err != nil {
			return fmt.Errorf("copy pinned memory snapshot %s: %w", source.name, err)
		}
		check, err := sql.Open("sqlite", snapshotSQLiteURI(dst, "mode=ro&immutable=1"))
		if err != nil {
			return err
		}
		var integrity string
		err = check.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&integrity)
		check.Close()
		if err != nil || integrity != "ok" {
			return fmt.Errorf("check memory snapshot %s: %s (%v)", source.name, integrity, err)
		}
	}
	return nil
}
