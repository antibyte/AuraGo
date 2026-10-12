package persondetect

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var storeT0 = time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)

func pdTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestEventStore(t *testing.T) *EventStore {
	t.Helper()
	st, err := OpenEventStore(t.TempDir(), pdTestLogger())
	if err != nil {
		t.Fatalf("OpenEventStore: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func modeOf(info os.FileInfo) os.FileMode {
	if info == nil {
		return 0
	}
	return info.Mode().Perm()
}

// requireModeWithin fails when path has permission bits beyond limit. A umask
// can only remove bits, so this holds under 022 and 077 alike.
func requireModeWithin(t *testing.T, path string, limit os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm()&^limit != 0 {
		t.Fatalf("%s mode = %v, %v; want no bits beyond %v", filepath.Base(path), modeOf(info), err, limit)
	}
}

func TestOpenEventStoreLayoutAndPermissions(t *testing.T) {
	dir := t.TempDir()
	dbPath, root := DetectionsPaths(dir)
	// An empty image root from before is reused, not moved aside.
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	before := time.Now().UnixMilli()
	st, err := OpenEventStore(dir, pdTestLogger())
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("OpenEventStore: %v", err)
	}
	defer st.Close()
	if dbPath != filepath.Join(dir, "go2rtc", "detections.db") || root != filepath.Join(dir, "go2rtc", "detections") || st.Root() != root {
		t.Fatalf("paths = %s, %s, root %s", dbPath, root, st.Root())
	}
	var version int
	if err := st.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != eventStoreSchemaVersion {
		t.Fatalf("user_version = %d, %v; want %d", version, err, eventStoreSchemaVersion)
	}
	// A fresh database starts its ids and revisions at the creation time in ms,
	// so they never repeat ids or revisions of a database that was reset.
	seed := st.Cursor()
	if seed < before || seed > after {
		t.Fatalf("fresh cursor = %d, want the creation time in ms (%d..%d)", seed, before, after)
	}
	ctx := context.Background()
	// A write makes SQLite create its WAL and SHM files, which take the
	// database file's mode.
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	frameID, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_t.jpg", Size: 1, CapturedAt: storeT0, Score: 0.9})
	if err != nil {
		t.Fatalf("AddFrame: %v", err)
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmOn, Armed: true, ChangedAt: storeT0, ChangedBy: ActorUser}); err != nil {
		t.Fatalf("RecordArm: %v", err)
	}
	entries, err := st.ArmLogLatest(ctx, st.Cursor(), 1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ArmLogLatest = %+v, %v", entries, err)
	}
	for name, v := range map[string]int64{"event id": id, "frame id": frameID, "arm log id": entries[0].ID, "cursor": st.Cursor()} {
		if v <= seed || v >= 1<<53 {
			t.Fatalf("%s = %d, want above the seed %d and below 2^53", name, v, seed)
		}
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
			requireModeWithin(t, path, 0o640)
		}
		requireModeWithin(t, root, 0o750)
	}
	if orphaned, _ := filepath.Glob(root + ".orphaned-*"); len(orphaned) != 0 {
		t.Fatalf("an empty image root was moved aside: %v", orphaned)
	}
	if _, err := OpenEventStore("  ", pdTestLogger()); err == nil {
		t.Fatal("empty data directory accepted")
	}
}

func TestOpenEventStoreRecoversOnlyCorruptDatabases(t *testing.T) {
	dir := t.TempDir()
	dbPath, _ := DetectionsPaths(dir)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, bytes.Repeat([]byte("not a database "), 200), 0o640); err != nil {
		t.Fatal(err)
	}
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatalf("OpenEventStore on a corrupt file: %v", err)
	}
	moved, _ := filepath.Glob(dbPath + ".corrupt-*")
	if len(moved) != 1 {
		t.Fatalf("corrupt database moved aside as %v, want one .corrupt-<time> file", moved)
	}
	if runtime.GOOS != "windows" {
		requireModeWithin(t, dbPath, 0o640)
	}
	if _, err := st.CreateEvent(context.Background(), NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); err != nil {
		t.Fatalf("CreateEvent on the fresh database: %v", err)
	}
	// A database written by a newer schema is refused and left where it is.
	if _, err := st.db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenEventStore(dir, pdTestLogger()); err == nil {
		t.Fatal("a database from a newer schema was opened")
	}
	if moved, _ := filepath.Glob(dbPath + ".corrupt-*"); len(moved) != 1 {
		t.Fatalf("a newer database must not be moved aside: %v", moved)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("the newer database must stay: %v", err)
	}
}

// A database that exists but cannot be used is an error and stays where it is:
// only real corruption moves a database aside.
func TestOpenEventStoreLeavesUnusableDatabasesInPlace(t *testing.T) {
	requireInPlace := func(t *testing.T, dbPath string) {
		t.Helper()
		if moved, _ := filepath.Glob(dbPath + "*.corrupt-*"); len(moved) != 0 {
			t.Fatalf("moved aside: %v", moved)
		}
		if _, err := os.Lstat(dbPath); err != nil {
			t.Fatalf("the database must stay: %v", err)
		}
	}
	newStoreWithEvent := func(t *testing.T) (string, string) {
		t.Helper()
		dir := t.TempDir()
		st, err := OpenEventStore(dir, pdTestLogger())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateEvent(context.Background(), NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); err != nil {
			t.Fatal(err)
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
		dbPath, _ := DetectionsPaths(dir)
		return dir, dbPath
	}

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		dbPath, _ := DetectionsPaths(dir)
		if err := os.MkdirAll(dbPath, 0o750); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenEventStore(dir, pdTestLogger()); err == nil {
			t.Fatal("a directory in place of the database was accepted")
		}
		requireInPlace(t, dbPath)
		if info, err := os.Stat(dbPath); err != nil || !info.IsDir() {
			t.Fatalf("the directory must stay: %v", err)
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows has no POSIX read permission to take away")
		}
		if os.Geteuid() == 0 {
			t.Skip("root reads every file")
		}
		dir, dbPath := newStoreWithEvent(t)
		if err := os.Chmod(dbPath, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dbPath, 0o640) })
		if _, err := OpenEventStore(dir, pdTestLogger()); err == nil {
			t.Fatal("an unreadable database was opened")
		}
		requireInPlace(t, dbPath)
		if info, err := os.Stat(dbPath); err != nil || info.Mode().Perm() != 0 {
			t.Fatalf("the unreadable database's mode changed: %v, %v", modeOf(info), err)
		}
	})

	t.Run("locked", func(t *testing.T) {
		previous := eventStoreBusyTimeoutMS
		eventStoreBusyTimeoutMS = 200
		t.Cleanup(func() { eventStoreBusyTimeoutMS = previous })
		dir, dbPath := newStoreWithEvent(t)
		ctx := context.Background()
		holder, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(100)")
		if err != nil {
			t.Fatal(err)
		}
		defer holder.Close()
		holder.SetMaxOpenConns(1)
		conn, err := holder.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		for _, stmt := range []string{`PRAGMA locking_mode=EXCLUSIVE`, `BEGIN EXCLUSIVE`} {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}
		start := time.Now()
		if st, err := OpenEventStore(dir, pdTestLogger()); err == nil {
			_ = st.Close()
			t.Fatal("a database locked by another connection was opened")
		}
		if took := time.Since(start); took > 3*time.Second {
			t.Fatalf("open of a locked database took %v", took)
		}
		requireInPlace(t, dbPath)
		if _, err := conn.ExecContext(ctx, `ROLLBACK`); err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
		_ = holder.Close()
		st, err := OpenEventStore(dir, pdTestLogger())
		if err != nil {
			t.Fatalf("open after the lock was released: %v", err)
		}
		defer st.Close()
		if page, err := st.Events(ctx, EventQuery{}); err != nil || len(page.Events) != 1 {
			t.Fatalf("events after the lock = %+v, %v", page.Events, err)
		}
	})
}

// Existing files and directories with wider modes are tightened on open (never
// widened): <data>/go2rtc is mounted into the go2rtc container, so a restored
// 0644 database would be readable there.
func TestOpenEventStoreTightensExistingModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	dir := t.TempDir()
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	dbPath, root := DetectionsPaths(dir)
	// SQLite treats empty WAL and SHM files as no WAL and an uninitialized index.
	for _, path := range []string{dbPath + "-wal", dbPath + "-shm"} {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, mode := range map[string]os.FileMode{dbPath: 0o644, dbPath + "-wal": 0o666, dbPath + "-shm": 0o644, root: 0o755} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	st, err = OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateEvent(context.Background(), NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		requireModeWithin(t, path, 0o640)
		if info, _ := os.Stat(path); info.Mode().Perm()&0o600 != 0o600 {
			t.Fatalf("%s lost its owner bits: %v", filepath.Base(path), info.Mode().Perm())
		}
	}
	requireModeWithin(t, root, 0o750)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	// A tighter mode stays as it is.
	if err := os.Chmod(dbPath, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	st, err = OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if info, err := os.Stat(dbPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("a 0600 database became %v, %v", modeOf(info), err)
	}
	if info, err := os.Stat(root); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("a 0700 image root became %v, %v", modeOf(info), err)
	}
}

// fillStoreForReset stores events with frames, image files and arm-log rows and
// returns the highest event and frame ids, the cursor and the image paths.
func fillStoreForReset(t *testing.T, st *EventStore) (lastEvent, lastFrame, cursor int64, images []string) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		id, err := st.CreateEvent(ctx, NewEvent{CameraID: "front-door", CameraName: "Front", StartedAt: storeT0})
		if err != nil {
			t.Fatal(err)
		}
		rel := fmt.Sprintf("2026/10/11/front-door/%d_1.jpg", id)
		thumb := fmt.Sprintf("2026/10/11/front-door/%d_1_thumb.jpg", id)
		for _, p := range []string{rel, thumb} {
			full := filepath.Join(st.Root(), filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte("old image"), 0o640); err != nil {
				t.Fatal(err)
			}
		}
		frameID, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: rel, ThumbPath: thumb, Size: 18, CapturedAt: storeT0, Score: 0.9})
		if err != nil {
			t.Fatal(err)
		}
		if err := st.RecordArm(ctx, armRecord{Mode: ArmOn, Armed: true, ChangedAt: storeT0, ChangedBy: ActorUser}); err != nil {
			t.Fatal(err)
		}
		lastEvent, lastFrame = id, frameID
		images = append(images, rel, thumb)
	}
	return lastEvent, lastFrame, st.Cursor(), images
}

// waitPastMillis waits until the wall clock in ms is beyond v. A database reset
// seeds ids and revisions from the clock; in practice far more time passes
// between a database's creation and its reset than it has writes.
func waitPastMillis(t *testing.T, v int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().UnixMilli() <= v {
		if time.Now().After(deadline) {
			t.Fatalf("the clock did not pass %d", v)
		}
		time.Sleep(time.Millisecond)
	}
}

// requireResetAfter checks a store opened after a database reset: ids and the
// cursor continue above the old ones, and the old images were moved aside to
// one detections.orphaned-<time> directory, untouched, behind a fresh empty root.
func requireResetAfter(t *testing.T, st *EventStore, lastEvent, lastFrame, cursor int64, images []string, logs string) {
	t.Helper()
	ctx := context.Background()
	if st.Cursor() <= cursor {
		t.Fatalf("cursor after the reset = %d, want above %d", st.Cursor(), cursor)
	}
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "front-door", CameraName: "Front", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	rel := fmt.Sprintf("2026/10/11/front-door/%d_1.jpg", id)
	frameID, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: rel, ThumbPath: rel + "_t", Size: 5, CapturedAt: storeT0, Score: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if id <= lastEvent || frameID <= lastFrame {
		t.Fatalf("ids after the reset: event %d (before %d), frame %d (before %d)", id, lastEvent, frameID, lastFrame)
	}
	for _, old := range images {
		if old == rel {
			t.Fatalf("the new frame path %s collides with an old image", rel)
		}
	}
	root := st.Root()
	orphaned, _ := filepath.Glob(root + ".orphaned-*")
	if len(orphaned) != 1 {
		t.Fatalf("orphaned image roots = %v, want one", orphaned)
	}
	for _, p := range images {
		data, err := os.ReadFile(filepath.Join(orphaned[0], filepath.FromSlash(p)))
		if err != nil || string(data) != "old image" {
			t.Fatalf("old image %s in %s: %q, %v", p, orphaned[0], data, err)
		}
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("the new image root holds %v, %v; want it empty", entries, err)
	}
	if runtime.GOOS != "windows" {
		requireModeWithin(t, root, 0o750)
	}
	if total, err := st.TotalBytes(ctx); err != nil || total != 5 {
		t.Fatalf("TotalBytes = %d, %v; want only the new frame", total, err)
	}
	if !strings.Contains(logs, "level=WARN") || !strings.Contains(logs, filepath.Base(orphaned[0])) {
		t.Fatalf("the move of the old images must be logged as a warning with its path:\n%s", logs)
	}
}

func TestEventStoreIDsKeepIncreasingAfterCorruptionRecovery(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	lastEvent, lastFrame, cursor, images := fillStoreForReset(t, st)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	dbPath, _ := DetectionsPaths(dir)
	if err := os.WriteFile(dbPath, bytes.Repeat([]byte("garbage!"), 1000), 0o640); err != nil {
		t.Fatal(err)
	}
	waitPastMillis(t, cursor)
	var logs bytes.Buffer
	st, err = OpenEventStore(dir, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	corrupt, _ := filepath.Glob(dbPath + ".corrupt-*")
	if len(corrupt) != 1 {
		t.Fatalf("corrupt files = %v", corrupt)
	}
	if !strings.Contains(logs.String(), filepath.Base(corrupt[0])) {
		t.Fatalf("the corruption log must name the moved file %s:\n%s", filepath.Base(corrupt[0]), logs.String())
	}
	requireResetAfter(t, st, lastEvent, lastFrame, cursor, images, logs.String())
}

func TestEventStoreDeletedDatabaseMovesOrphanedImagesAside(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	lastEvent, lastFrame, cursor, images := fillStoreForReset(t, st)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	dbPath, _ := DetectionsPaths(dir)
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	waitPastMillis(t, cursor)
	var logs bytes.Buffer
	st, err = OpenEventStore(dir, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if corrupt, _ := filepath.Glob(dbPath + "*.corrupt-*"); len(corrupt) != 0 {
		t.Fatalf("nothing was corrupt, yet %v", corrupt)
	}
	requireResetAfter(t, st, lastEvent, lastFrame, cursor, images, logs.String())
}

func TestEventStoreReopenKeepsIDsAndImages(t *testing.T) {
	dir := t.TempDir()
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	lastEvent, lastFrame, cursor, images := fillStoreForReset(t, st)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	st, err = OpenEventStore(dir, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.Cursor() != cursor {
		t.Fatalf("cursor after reopen = %d, want %d", st.Cursor(), cursor)
	}
	ctx := context.Background()
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	frameID, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "n.jpg", ThumbPath: "n_t.jpg", Size: 1, CapturedAt: storeT0, Score: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if id != lastEvent+1 || frameID != lastFrame+1 || st.Cursor() != cursor+2 {
		t.Fatalf("after reopen: event %d (want %d), frame %d (want %d), cursor %d (want %d)", id, lastEvent+1, frameID, lastFrame+1, st.Cursor(), cursor+2)
	}
	for _, p := range images {
		if _, err := os.Stat(filepath.Join(st.Root(), filepath.FromSlash(p))); err != nil {
			t.Fatalf("image %s after reopen: %v", p, err)
		}
	}
	if orphaned, _ := filepath.Glob(st.Root() + ".orphaned-*"); len(orphaned) != 0 {
		t.Fatalf("a reopen moved images aside: %v", orphaned)
	}
	if logs.Len() != 0 {
		t.Fatalf("a normal reopen logged:\n%s", logs.String())
	}
}

// Schema steps run only from a database's own version on, in one transaction
// together with the user_version update.
func TestEventStoreMigrationsAreGatedAndAtomic(t *testing.T) {
	production := eventStoreMigrations
	t.Cleanup(func() { eventStoreMigrations = production })
	now := time.Now()
	openRaw := func(t *testing.T) *sql.DB {
		t.Helper()
		dbPath, _ := DetectionsPaths(t.TempDir())
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
			t.Fatal(err)
		}
		db, err := openDetectionsDB(dbPath, pdTestLogger())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		return db
	}
	userVersion := func(t *testing.T, db *sql.DB) int {
		t.Helper()
		var v int
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	tableExists := func(t *testing.T, db *sql.DB, name string) bool {
		t.Helper()
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n == 1
	}

	// A failing later step rolls back the earlier steps and the version.
	db := openRaw(t)
	eventStoreMigrations = []func(*sql.Tx, time.Time) error{
		migrateEventStoreV1,
		func(*sql.Tx, time.Time) error { return errors.New("step 2 failed") },
	}
	if err := migrateEventStore(db, 0, now); err == nil || !strings.Contains(err.Error(), "step 2 failed") {
		t.Fatalf("migration with a failing step = %v", err)
	}
	if v := userVersion(t, db); v != 0 || tableExists(t, db, "events") {
		t.Fatalf("after a failed migration: user_version %d, events table %v", v, tableExists(t, db, "events"))
	}

	// A database at version 1 runs only the steps after it.
	eventStoreMigrations = production
	db = openRaw(t)
	if err := migrateEventStore(db, 0, now); err != nil || userVersion(t, db) != 1 {
		t.Fatalf("migration to version 1 = %v, user_version %d", err, userVersion(t, db))
	}
	ranV2 := false
	eventStoreMigrations = []func(*sql.Tx, time.Time) error{
		func(*sql.Tx, time.Time) error { return errors.New("step 1 ran again") },
		func(tx *sql.Tx, _ time.Time) error {
			ranV2 = true
			_, err := tx.Exec(`CREATE TABLE v2_marker (x INTEGER)`)
			return err
		},
	}
	if err := migrateEventStore(db, 1, now); err != nil || !ranV2 || userVersion(t, db) != 2 || !tableExists(t, db, "v2_marker") {
		t.Fatalf("migration from version 1 = %v, step 2 ran %v, user_version %d", err, ranV2, userVersion(t, db))
	}
}

// A write waits for the write lock only as long as its context allows: a delete
// whose client went away must not queue behind a long retention pass.
func TestEventStoreWritesGiveUpWhenTheContextEnds(t *testing.T) {
	st := newTestEventStore(t)
	bg := context.Background()
	id, err := st.CreateEvent(bg, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := st.lockWrites(bg)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(bg)
	cancel()
	if _, err := st.RemoveEvent(cancelled, id); !errors.Is(err, context.Canceled) {
		t.Fatalf("RemoveEvent with a cancelled context = %v", err)
	}
	short, cancelShort := context.WithTimeout(bg, 50*time.Millisecond)
	defer cancelShort()
	start := time.Now()
	if _, err := st.CreateEvent(short, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CreateEvent behind the lock = %v", err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("CreateEvent waited %v behind the lock", took)
	}
	// A context that is already done is refused even when the lock is free.
	unlock()
	unlock() // idempotent
	if err := st.EndEvent(cancelled, id, storeT0); !errors.Is(err, context.Canceled) {
		t.Fatalf("EndEvent with a cancelled context = %v", err)
	}
	if _, err := st.RemoveEvent(bg, id); err != nil {
		t.Fatalf("RemoveEvent after the lock was released: %v", err)
	}
}

// Event reads the summary and the frames in one snapshot, so frame_count always
// matches the frames returned, also while frames are being added.
func TestEventStoreEventReadsOneSnapshot(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for seq := 1; seq <= 300; seq++ {
			if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: seq, Path: fmt.Sprintf("%d.jpg", seq), ThumbPath: "t.jpg", Size: 1, CapturedAt: storeT0, Score: 0.5}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	reads, mismatches := 0, 0
	var writeErr error
	for running := true; running; {
		select {
		case writeErr = <-done:
			running = false
		default:
		}
		d, err := st.Event(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		reads++
		if d.FrameCount != len(d.Frames) {
			mismatches++
		}
	}
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if mismatches > 0 {
		t.Fatalf("%d of %d reads returned frame_count != len(frames)", mismatches, reads)
	}
}

// storeFeedClient emulates Service.Changes with a cursor (Task 15) at a small
// row limit and keeps the newest version of every row it was sent.
type storeFeedClient struct {
	limit  int
	cursor int64
	events map[int64]EventSummary
	arm    map[int64]ArmLogEntry
}

func (c *storeFeedClient) poll(ctx context.Context, st *EventStore) error {
	status := st.Cursor()
	if c.cursor >= status {
		return nil
	}
	events, eventsLast, eventsFull, err := st.EventsBetween(ctx, c.cursor, status, c.limit)
	if err != nil {
		return err
	}
	armLog, armLast, armFull, err := st.ArmLogBetween(ctx, c.cursor, status, c.limit)
	if err != nil {
		return err
	}
	next := status
	if eventsFull && eventsLast < next {
		next = eventsLast
	}
	if armFull && armLast < next {
		next = armLast
	}
	if next <= c.cursor {
		return fmt.Errorf("cursor did not advance: %d -> %d (status %d)", c.cursor, next, status)
	}
	for _, ev := range events {
		c.events[ev.ID] = ev
	}
	for _, e := range armLog {
		c.arm[e.ID] = e
	}
	c.cursor = next
	return nil
}

// Concurrent pipeline, retention and arming writes (CloseOpenEvents stamps
// several rows with one revision) while clients poll at small limits: once the
// writers stop and the clients drain, every client holds the final version of
// every event and arm-log row.
func TestEventStoreFeedDeliversEveryRow(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	fail := func(where string, err error) {
		select {
		case errs <- fmt.Errorf("%s: %w", where, err):
		default:
		}
	}
	for cam := 0; cam < 4; cam++ {
		wg.Add(1)
		go func(cam int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(cam) + 1))
			for {
				select {
				case <-stop:
					return
				default:
				}
				id, err := st.CreateEvent(ctx, NewEvent{CameraID: fmt.Sprint("cam", cam), CameraName: "Cam", StartedAt: storeT0.Add(time.Duration(rng.Intn(100000)) * time.Millisecond)})
				if err != nil {
					fail("create", err)
					return
				}
				frames := 1 + rng.Intn(4)
				for seq := 1; seq <= frames; seq++ {
					if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: seq, Path: fmt.Sprintf("x/%d_%d.jpg", id, seq), ThumbPath: "t", Size: int64(10 + rng.Intn(100)), CapturedAt: storeT0, Score: rng.Float64()}); err != nil && !errors.Is(err, ErrUnknownEvent) {
						fail("frame", err)
					}
					if err := st.UpdateEventStats(ctx, id, EventStats{MaxScore: rng.Float64(), MaxPersons: rng.Intn(5), Zones: []string{fmt.Sprint("z", rng.Intn(3))}}); err != nil && !errors.Is(err, ErrUnknownEvent) {
						fail("stats", err)
					}
					if rng.Intn(3) == 0 {
						if err := st.SetAlertResult(ctx, id, ActionAgent, AlertResult{Status: AlertSent, Detail: fmt.Sprint(rng.Intn(1000))}); err != nil && !errors.Is(err, ErrUnknownEvent) {
							fail("alert", err)
						}
					}
				}
				if rng.Intn(2) == 0 { // the other half is left for CloseOpenEvents
					if err := st.EndEvent(ctx, id, storeT0.Add(time.Hour)); err != nil {
						fail("end", err)
					}
				}
			}
		}(cam)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		rng := rand.New(rand.NewSource(99))
		modes := []ArmMode{ArmOn, ArmOff, ArmSchedule}
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			switch rng.Intn(3) {
			case 0:
				if _, err := st.CloseOpenEvents(ctx); err != nil {
					fail("close open events", err)
				}
			case 1:
				if err := st.RecordArm(ctx, armRecord{Mode: modes[rng.Intn(3)], Armed: rng.Intn(2) == 0, ChangedAt: storeT0.Add(time.Duration(i) * time.Second), ChangedBy: ActorUser}); err != nil {
					fail("arm", err)
				}
			case 2:
				ids, err := st.oldestEndedEventIDs(ctx, 2)
				if err != nil {
					fail("oldest", err)
					break
				}
				for _, id := range ids {
					if _, err := st.RemoveEvent(ctx, id); err != nil && !errors.Is(err, ErrUnknownEvent) {
						fail("remove", err)
					}
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()
	clients := []*storeFeedClient{{limit: 1}, {limit: 2}, {limit: 7}}
	for _, c := range clients {
		c.events, c.arm = map[int64]EventSummary{}, map[int64]ArmLogEntry{}
		wg.Add(1)
		go func(c *storeFeedClient) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if err := c.poll(ctx, st); err != nil {
					fail(fmt.Sprintf("poll at limit %d", c.limit), err)
					return
				}
			}
		}(c)
	}
	time.Sleep(1500 * time.Millisecond)
	close(stop)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		return
	}
	final := st.Cursor()
	for _, c := range clients {
		for polls := 0; c.cursor < final; polls++ {
			if polls > 100000 {
				t.Fatalf("client at limit %d did not drain", c.limit)
			}
			if err := c.poll(ctx, st); err != nil {
				t.Fatal(err)
			}
		}
	}
	truthEvents, err := st.EventsLatest(ctx, final, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	truthArm, err := st.ArmLogLatest(ctx, final, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	if len(truthEvents) == 0 || len(truthArm) == 0 {
		t.Fatalf("the run wrote %d events and %d arm-log rows", len(truthEvents), len(truthArm))
	}
	for _, c := range clients {
		missing, stale := 0, 0
		var sample []string
		for _, ev := range truthEvents {
			got, ok := c.events[ev.ID]
			switch {
			case !ok:
				missing++
			case !reflect.DeepEqual(got, ev):
				stale++
			default:
				continue
			}
			if len(sample) < 3 {
				sample = append(sample, fmt.Sprintf("event %d: client %+v, store %+v", ev.ID, got, ev))
			}
		}
		for _, e := range truthArm {
			if got, ok := c.arm[e.ID]; !ok {
				missing++
			} else if got != e {
				stale++
			}
		}
		if missing+stale > 0 {
			t.Fatalf("client at limit %d: %d missing, %d stale rows of %d events and %d arm-log rows; %v", c.limit, missing, stale, len(truthEvents), len(truthArm), sample)
		}
	}
}

func TestEventStoreEventLifecycleAndCover(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	id, err := st.CreateEvent(ctx, NewEvent{
		CameraID: "front-door", CameraName: "Front door", StartedAt: storeT0, Armed: true,
		Stats:  EventStats{MaxScore: 0.6, MaxPersons: 1, Zones: []string{"Driveway"}},
		Alerts: map[string]AlertResult{ActionAgent: {Status: AlertSkipped, Detail: "disarmed"}},
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	f1, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_thumb.jpg", Size: 100, CapturedAt: storeT0, Width: 640, Height: 360, Boxes: []Box{{X1: 0.1, Y1: 0.2, X2: 0.3, Y2: 0.9, Score: 0.6}}, Score: 0.6})
	if err != nil {
		t.Fatalf("AddFrame 1: %v", err)
	}
	f2, err := st.AddFrame(ctx, id, StoredFrame{Seq: 2, Path: "b.jpg", ThumbPath: "b_thumb.jpg", Size: 50, CapturedAt: storeT0.Add(5 * time.Second), Width: 640, Height: 360, Score: 0.9})
	if err != nil {
		t.Fatalf("AddFrame 2: %v", err)
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 3, Path: "c.jpg", ThumbPath: "c_thumb.jpg", Size: 100, CapturedAt: storeT0.Add(10 * time.Second), Width: 640, Height: 360, Score: 0.7}); err != nil {
		t.Fatalf("AddFrame 3: %v", err)
	}
	if err := st.UpdateEventStats(ctx, id, EventStats{MaxScore: 0.9, MaxPersons: 2, Zones: []string{"Driveway", "Porch"}}); err != nil {
		t.Fatalf("UpdateEventStats: %v", err)
	}
	if err := st.SetAlertResult(ctx, id, ActionMission, AlertResult{Status: AlertSent}); err != nil {
		t.Fatalf("SetAlertResult: %v", err)
	}
	if err := st.EndEvent(ctx, id, storeT0.Add(12*time.Second)); err != nil {
		t.Fatalf("EndEvent: %v", err)
	}
	d, err := st.Event(ctx, id)
	if err != nil {
		t.Fatalf("Event: %v", err)
	}
	if d.CoverFrameID == nil || *d.CoverFrameID != f2 || d.FrameCount != 3 || d.MaxScore != 0.9 || d.MaxPersons != 2 || !d.Armed {
		t.Fatalf("detail = %+v, want cover %d", d.EventSummary, f2)
	}
	if !reflect.DeepEqual(d.Zones, []string{"Driveway", "Porch"}) || d.EndedAt == nil || !d.EndedAt.Equal(storeT0.Add(12*time.Second)) {
		t.Fatalf("zones %v ended %v", d.Zones, d.EndedAt)
	}
	if d.Alerts[ActionAgent] != (AlertResult{Status: AlertSkipped, Detail: "disarmed"}) || d.Alerts[ActionMission] != (AlertResult{Status: AlertSent}) {
		t.Fatalf("alerts = %+v", d.Alerts)
	}
	if len(d.Frames) != 3 || d.Frames[0].ID != f1 || d.Frames[0].Boxes[0].Score != 0.6 || d.Frames[0].Width != 640 || d.Frames[1].Boxes == nil {
		t.Fatalf("frames = %+v", d.Frames)
	}
	if total, err := st.TotalBytes(ctx); err != nil || total != 250 {
		t.Fatalf("TotalBytes = %d, %v", total, err)
	}
	if path, thumb, err := st.Frame(ctx, f1); err != nil || path != "a.jpg" || thumb != "a_thumb.jpg" {
		t.Fatalf("Frame = %q %q %v", path, thumb, err)
	}
	if _, _, err := st.Frame(ctx, 999); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("unknown frame err = %v", err)
	}
	if _, err := st.Event(ctx, 999); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("unknown event err = %v", err)
	}
	if err := st.UpdateEventStats(ctx, 999, EventStats{}); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("stats of unknown event err = %v", err)
	}
	if err := st.SetAlertResult(ctx, 999, ActionAgent, AlertResult{Status: AlertSent}); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("alert of unknown event err = %v", err)
	}
	if _, err := st.AddFrame(ctx, 999, StoredFrame{Seq: 1, Path: "z.jpg", ThumbPath: "z_t.jpg", Size: 1, CapturedAt: storeT0, Score: 0.9}); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("frame of unknown event err = %v", err)
	}
}

func TestEventStoreRevisionsDriveChanges(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	c0 := st.Cursor()
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	c1 := st.Cursor()
	events, last, full, err := st.EventsBetween(ctx, c0, c1, 10)
	if err != nil || len(events) != 1 || events[0].ID != id || last != c1 || full || c1 <= c0 {
		t.Fatalf("EventsBetween(c0,c1) = %d events, last %d, full %v, err %v", len(events), last, full, err)
	}
	if events, _, _, _ := st.EventsBetween(ctx, c1, c1, 10); len(events) != 0 {
		t.Fatal("no change after the cursor expected")
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_t.jpg", Size: 10, CapturedAt: storeT0, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	c2 := st.Cursor()
	if events, _, _, _ := st.EventsBetween(ctx, c1, c2, 10); len(events) != 1 || events[0].FrameCount != 1 {
		t.Fatalf("a frame insert must bump its event: %+v", events)
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmOn, Armed: true, ChangedAt: storeT0, ChangedBy: ActorUser}); err != nil {
		t.Fatal(err)
	}
	c3 := st.Cursor()
	entries, armLast, _, err := st.ArmLogBetween(ctx, c2, c3, 10)
	if err != nil || len(entries) != 1 || armLast != c3 || entries[0].Mode != ArmOn || !entries[0].Armed || entries[0].Actor != ActorUser || !entries[0].At.Equal(storeT0) {
		t.Fatalf("arm log = %+v, %v", entries, err)
	}
	for i := 0; i < 3; i++ {
		if _, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0}); err != nil {
			t.Fatal(err)
		}
	}
	events, last, full, err = st.EventsBetween(ctx, c3, st.Cursor(), 2)
	if err != nil || len(events) != 2 || !full || last != c3+2 {
		t.Fatalf("capped = %d events, last %d (want %d), full %v", len(events), last, c3+2, full)
	}
	if _, _, _, err := st.EventsBetween(ctx, c0, st.Cursor(), 0); err == nil {
		t.Fatal("EventsBetween accepted a row limit of 0")
	}
	if _, _, _, err := st.ArmLogBetween(ctx, c0, st.Cursor(), 0); err == nil {
		t.Fatal("ArmLogBetween accepted a row limit of 0")
	}
}

// One write can stamp several rows (CloseOpenEvents). A capped page must not
// split that revision: the next call continues after the page's last revision,
// so rows of that revision left out of the page would never be delivered.
func TestEventStoreBetweenKeepsRevisionsWhole(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	var ids []int64
	for i := 0; i < 3; i++ {
		id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Duration(i) * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if n, err := st.CloseOpenEvents(ctx); err != nil || n != 3 {
		t.Fatalf("CloseOpenEvents = %d, %v", n, err)
	}
	closedRev := st.Cursor()
	newest, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	upTo := st.Cursor()

	events, last, full, err := st.EventsBetween(ctx, 0, upTo, 2)
	if err != nil || len(events) != 3 || last != closedRev || !full {
		t.Fatalf("first page = %d events, last %d (want %d), full %v, err %v", len(events), last, closedRev, full, err)
	}
	for i, ev := range events {
		if ev.ID != ids[i] || ev.EndedAt == nil {
			t.Fatalf("first page row %d = %+v, want closed event %d", i, ev, ids[i])
		}
	}
	events, last, full, err = st.EventsBetween(ctx, last, upTo, 2)
	if err != nil || len(events) != 1 || events[0].ID != newest || last != upTo || full {
		t.Fatalf("second page = %+v, last %d, full %v, err %v", events, last, full, err)
	}
}

// EventsLatest and ArmLogLatest back the Changes(0) snapshot: the newest rows
// by revision up to a cursor, returned oldest revision first.
func TestEventStoreLatestRows(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	if events, err := st.EventsLatest(ctx, st.Cursor(), 10); err != nil || events == nil || len(events) != 0 {
		t.Fatalf("EventsLatest on an empty store = %#v, %v", events, err)
	}
	if entries, err := st.ArmLogLatest(ctx, st.Cursor(), 10); err != nil || entries == nil || len(entries) != 0 {
		t.Fatalf("ArmLogLatest on an empty store = %#v, %v", entries, err)
	}

	var ids, eventRevs []int64
	for i := 0; i < 5; i++ {
		id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Duration(i) * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		ids, eventRevs = append(ids, id), append(eventRevs, st.Cursor())
	}
	// Changing the oldest event gives it the newest revision.
	if err := st.SetAlertResult(ctx, ids[0], ActionAgent, AlertResult{Status: AlertSent}); err != nil {
		t.Fatal(err)
	}
	snapshot := st.Cursor()
	// A row written after the snapshot cursor stays out.
	if _, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	eventIDs := func(events []EventSummary) []int64 {
		out := []int64{}
		for _, ev := range events {
			out = append(out, ev.ID)
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		upTo  int64
		limit int
		want  []int64
	}{
		{"more rows than the limit", snapshot, 3, []int64{ids[3], ids[4], ids[0]}},
		{"fewer rows than the limit", snapshot, 100, []int64{ids[1], ids[2], ids[3], ids[4], ids[0]}},
		{"cursor below the newest revision", eventRevs[2], 2, []int64{ids[1], ids[2]}},
		{"cursor before every row", 0, 10, []int64{}},
	} {
		events, err := st.EventsLatest(ctx, tc.upTo, tc.limit)
		if err != nil || !reflect.DeepEqual(eventIDs(events), tc.want) {
			t.Fatalf("EventsLatest %s = %v, %v; want %v", tc.name, eventIDs(events), err, tc.want)
		}
		if events == nil {
			t.Fatalf("EventsLatest %s returned a nil list", tc.name)
		}
	}
	if events, err := st.EventsLatest(ctx, snapshot, 1); err != nil || len(events) != 1 || events[0].Alerts[ActionAgent].Status != AlertSent {
		t.Fatalf("EventsLatest must return the changed row in full: %+v, %v", events, err)
	}

	var armRevs []int64
	modes := []ArmMode{ArmOn, ArmOff, ArmSchedule, ArmOn, ArmOff}
	for i, mode := range modes {
		if err := st.RecordArm(ctx, armRecord{Mode: mode, Armed: mode == ArmOn, ChangedAt: storeT0.Add(time.Duration(i) * time.Minute), ChangedBy: ActorUser}); err != nil {
			t.Fatal(err)
		}
		armRevs = append(armRevs, st.Cursor())
	}
	armModes := func(entries []ArmLogEntry) []ArmMode {
		out := []ArmMode{}
		for _, e := range entries {
			out = append(out, e.Mode)
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		upTo  int64
		limit int
		want  []ArmMode
	}{
		{"more rows than the limit", armRevs[4], 2, []ArmMode{ArmOn, ArmOff}},
		{"fewer rows than the limit", armRevs[4], 50, modes},
		{"cursor below the newest revision", armRevs[2], 2, []ArmMode{ArmOff, ArmSchedule}},
		{"cursor before every row", snapshot, 10, []ArmMode{}},
	} {
		entries, err := st.ArmLogLatest(ctx, tc.upTo, tc.limit)
		if err != nil || !reflect.DeepEqual(armModes(entries), tc.want) {
			t.Fatalf("ArmLogLatest %s = %v, %v; want %v", tc.name, armModes(entries), err, tc.want)
		}
		if entries == nil {
			t.Fatalf("ArmLogLatest %s returned a nil list", tc.name)
		}
	}
	if entries, _ := st.ArmLogLatest(ctx, armRevs[4], 2); !entries[0].At.Equal(storeT0.Add(3*time.Minute)) || entries[1].ID <= entries[0].ID {
		t.Fatalf("ArmLogLatest rows = %+v", entries)
	}
	if _, err := st.EventsLatest(ctx, snapshot, 0); err == nil {
		t.Fatal("EventsLatest accepted a row limit of 0")
	}
	if _, err := st.ArmLogLatest(ctx, snapshot, -1); err == nil {
		t.Fatal("ArmLogLatest accepted a negative row limit")
	}
}

func TestEventStorePagingAndFilters(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	var created []int64
	for i := 0; i < 5; i++ {
		camera := "a"
		if i%2 == 1 {
			camera = "b"
		}
		id, err := st.CreateEvent(ctx, NewEvent{CameraID: camera, CameraName: camera, StartedAt: storeT0.Add(time.Duration(i) * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, id)
	}
	ids := func(page EventPage) []int64 {
		out := []int64{}
		for _, e := range page.Events {
			out = append(out, e.ID)
		}
		return out
	}
	page, err := st.Events(ctx, EventQuery{Limit: 2})
	if err != nil || !reflect.DeepEqual(ids(page), []int64{created[4], created[3]}) || page.NextBefore == nil || *page.NextBefore != created[3] {
		t.Fatalf("page 1 = %v next %v err %v", ids(page), page.NextBefore, err)
	}
	page, _ = st.Events(ctx, EventQuery{Limit: 2, Before: created[3]})
	if !reflect.DeepEqual(ids(page), []int64{created[2], created[1]}) || page.NextBefore == nil || *page.NextBefore != created[1] {
		t.Fatalf("page 2 = %v next %v", ids(page), page.NextBefore)
	}
	page, _ = st.Events(ctx, EventQuery{Limit: 2, Before: created[1]})
	if !reflect.DeepEqual(ids(page), []int64{created[0]}) || page.NextBefore != nil {
		t.Fatalf("page 3 = %v next %v", ids(page), page.NextBefore)
	}
	if page, _ := st.Events(ctx, EventQuery{CameraID: "a"}); !reflect.DeepEqual(ids(page), []int64{created[4], created[2], created[0]}) {
		t.Fatalf("camera filter = %v", ids(page))
	}
	if page, _ := st.Events(ctx, EventQuery{From: storeT0.Add(time.Minute), To: storeT0.Add(3 * time.Minute)}); !reflect.DeepEqual(ids(page), []int64{created[2], created[1]}) {
		t.Fatalf("time filter = %v", ids(page))
	}
	if page, _ := st.Events(ctx, EventQuery{Limit: 500}); len(page.Events) != 5 {
		t.Fatalf("large limit = %d events", len(page.Events))
	}
	if page, _ := st.Events(ctx, EventQuery{CameraID: "none"}); page.Events == nil || len(page.Events) != 0 {
		t.Fatalf("empty page must be an empty list: %#v", page.Events)
	}
}

// Every list the store returns is a JSON array (and the alerts map an object),
// never null: the API hands these values to the browser as they are.
func TestEventStoreListsAreNeverNil(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	requireArray := func(name string, obj map[string]interface{}, key string) {
		t.Helper()
		if list, ok := obj[key].([]interface{}); !ok || len(list) != 0 {
			t.Fatalf("%s: %q = %#v, want []", name, key, obj[key])
		}
	}
	requireObject := func(name string, obj map[string]interface{}, key string) {
		t.Helper()
		if m, ok := obj[key].(map[string]interface{}); !ok || len(m) != 0 {
			t.Fatalf("%s: %q = %#v, want {}", name, key, obj[key])
		}
	}

	// Nothing stored yet.
	page, err := st.Events(ctx, EventQuery{})
	if err != nil {
		t.Fatal(err)
	}
	requireArray("empty page", jsonObjectOf(t, page), "events")
	if events, _, _, err := st.EventsBetween(ctx, 0, st.Cursor()+1, 10); err != nil || events == nil || string(mustMarshal(t, events)) != "[]" {
		t.Fatalf("EventsBetween on an empty store = %#v, %v", events, err)
	}
	if entries, _, _, err := st.ArmLogBetween(ctx, 0, st.Cursor()+1, 10); err != nil || entries == nil || string(mustMarshal(t, entries)) != "[]" {
		t.Fatalf("ArmLogBetween on an empty store = %#v, %v", entries, err)
	}
	if last, err := st.LastEvents(ctx); err != nil || last == nil {
		t.Fatalf("LastEvents on an empty store = %#v, %v", last, err)
	}

	// An event created without zones or alerts, before any frame.
	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if err != nil {
		t.Fatal(err)
	}
	d, err := st.Event(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	obj := jsonObjectOf(t, d)
	requireArray("event without frames", obj, "zones")
	requireArray("event without frames", obj, "frames")
	requireObject("event without frames", obj, "alerts")
	events, _, _, err := st.EventsBetween(ctx, 0, st.Cursor(), 10)
	if err != nil || len(events) != 1 {
		t.Fatalf("EventsBetween = %+v, %v", events, err)
	}
	requireArray("changed event", jsonObjectOf(t, events[0]), "zones")
	requireObject("changed event", jsonObjectOf(t, events[0]), "alerts")
	latest, err := st.EventsLatest(ctx, st.Cursor(), 10)
	if err != nil || len(latest) != 1 {
		t.Fatalf("EventsLatest = %+v, %v", latest, err)
	}
	requireArray("latest event", jsonObjectOf(t, latest[0]), "zones")
	requireObject("latest event", jsonObjectOf(t, latest[0]), "alerts")

	// A frame stored without boxes, and stats without zones.
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_t.jpg", Size: 1, CapturedAt: storeT0, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateEventStats(ctx, id, EventStats{MaxScore: 0.9, MaxPersons: 1}); err != nil {
		t.Fatal(err)
	}
	d, err = st.Event(ctx, id)
	if err != nil || len(d.Frames) != 1 {
		t.Fatalf("Event = %+v, %v", d, err)
	}
	requireArray("event with stats without zones", jsonObjectOf(t, d), "zones")
	requireArray("frame without boxes", jsonObjectOf(t, d.Frames[0]), "boxes")

	// Rows whose JSON columns hold null (or damage) still decode to empty lists.
	if _, err := st.db.Exec(`UPDATE events SET zones = 'null', alerts = 'null' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE frames SET boxes = 'not json' WHERE event_id = ?`, id); err != nil {
		t.Fatal(err)
	}
	d, err = st.Event(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	obj = jsonObjectOf(t, d)
	requireArray("stored null zones", obj, "zones")
	requireObject("stored null alerts", obj, "alerts")
	requireArray("damaged boxes", jsonObjectOf(t, d.Frames[0]), "boxes")
}

// Timestamps come back in UTC, truncated to milliseconds, whatever location and
// precision they were written with.
func TestEventStoreReturnsMillisecondUTC(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	zone := time.FixedZone("UTC+2", 2*60*60)
	started := time.Date(2026, 10, 11, 14, 0, 0, 123456789, zone)
	captured := started.Add(time.Second + 999999*time.Nanosecond)
	ended := started.Add(2*time.Second + time.Nanosecond)
	armedAt := started.Add(3*time.Second + 555555*time.Nanosecond)
	requireMS := func(name string, got, written time.Time) {
		t.Helper()
		want := written.Truncate(time.Millisecond)
		if !got.Equal(want) || got.Location() != time.UTC || got.Nanosecond()%int(time.Millisecond) != 0 {
			t.Fatalf("%s = %s (%s), want %s in UTC", name, got.Format(time.RFC3339Nano), got.Location(), want.UTC().Format(time.RFC3339Nano))
		}
	}

	id, err := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: started})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: "a.jpg", ThumbPath: "a_t.jpg", Size: 1, CapturedAt: captured, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	if err := st.EndEvent(ctx, id, ended); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmOn, Armed: true, ChangedAt: armedAt, ChangedBy: ActorUser}); err != nil {
		t.Fatal(err)
	}

	d, err := st.Event(ctx, id)
	if err != nil || d.EndedAt == nil || len(d.Frames) != 1 {
		t.Fatalf("Event = %+v, %v", d, err)
	}
	requireMS("Event.StartedAt", d.StartedAt, started)
	requireMS("Event.EndedAt", *d.EndedAt, ended)
	requireMS("Event.Frames[0].CapturedAt", d.Frames[0].CapturedAt, captured)
	if !reflect.DeepEqual(d.StartedAt, time.Date(2026, 10, 11, 12, 0, 0, 123000000, time.UTC)) {
		t.Fatalf("StartedAt = %#v, want the plain UTC value", d.StartedAt)
	}
	if got := string(mustMarshal(t, d.StartedAt)); got != `"2026-10-11T12:00:00.123Z"` {
		t.Fatalf("StartedAt JSON = %s", got)
	}

	page, err := st.Events(ctx, EventQuery{})
	if err != nil || len(page.Events) != 1 {
		t.Fatalf("Events = %+v, %v", page, err)
	}
	requireMS("Events.StartedAt", page.Events[0].StartedAt, started)
	requireMS("Events.EndedAt", *page.Events[0].EndedAt, ended)
	between, _, _, err := st.EventsBetween(ctx, 0, st.Cursor(), 10)
	if err != nil || len(between) != 1 {
		t.Fatalf("EventsBetween = %+v, %v", between, err)
	}
	requireMS("EventsBetween.StartedAt", between[0].StartedAt, started)
	latest, err := st.EventsLatest(ctx, st.Cursor(), 10)
	if err != nil || len(latest) != 1 {
		t.Fatalf("EventsLatest = %+v, %v", latest, err)
	}
	requireMS("EventsLatest.EndedAt", *latest[0].EndedAt, ended)

	entries, _, _, err := st.ArmLogBetween(ctx, 0, st.Cursor(), 10)
	if err != nil || len(entries) != 1 {
		t.Fatalf("ArmLogBetween = %+v, %v", entries, err)
	}
	requireMS("ArmLogBetween.At", entries[0].At, armedAt)
	if entries, err = st.ArmLogLatest(ctx, st.Cursor(), 10); err != nil || len(entries) != 1 {
		t.Fatalf("ArmLogLatest = %+v, %v", entries, err)
	}
	requireMS("ArmLogLatest.At", entries[0].At, armedAt)
	rec, err := st.LoadArm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireMS("LoadArm.ChangedAt", rec.ChangedAt, armedAt)
	last, err := st.LastEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	requireMS("LastEvents.at", last["cam"].at, started)
}

func TestEventStoreArmStateDefaultsOffAndPersists(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	st, err := OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := st.LoadArm(ctx)
	if err != nil || rec.Mode != ArmOff || rec.Armed || !rec.ChangedAt.IsZero() || rec.ChangedBy != "" {
		t.Fatalf("fresh arm state = %+v, %v", rec, err)
	}
	if err := st.RecordArm(ctx, armRecord{Mode: ArmSchedule, Armed: true, ChangedAt: storeT0, ChangedBy: ActorAgent}); err != nil {
		t.Fatal(err)
	}
	cursor := st.Cursor()
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = OpenEventStore(dir, pdTestLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	rec, err = st.LoadArm(ctx)
	if err != nil || rec.Mode != ArmSchedule || !rec.Armed || !rec.ChangedAt.Equal(storeT0) || rec.ChangedBy != ActorAgent {
		t.Fatalf("reloaded arm state = %+v, %v", rec, err)
	}
	if st.Cursor() != cursor {
		t.Fatalf("the cursor must survive a reopen: %d, want %d", st.Cursor(), cursor)
	}
}

func TestEventStoreCloseOpenEventsAndRemove(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	withFrame, _ := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0})
	if _, err := st.AddFrame(ctx, withFrame, StoredFrame{Seq: 1, Path: "x.jpg", ThumbPath: "x_t.jpg", Size: 1, CapturedAt: storeT0.Add(7 * time.Second), Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	noFrames, _ := st.CreateEvent(ctx, NewEvent{CameraID: "cam", CameraName: "Cam", StartedAt: storeT0.Add(time.Minute)})
	if n, err := st.CloseOpenEvents(ctx); err != nil || n != 2 {
		t.Fatalf("CloseOpenEvents = %d, %v", n, err)
	}
	cursor := st.Cursor()
	if n, err := st.CloseOpenEvents(ctx); err != nil || n != 0 || st.Cursor() != cursor {
		t.Fatalf("second CloseOpenEvents = %d, %v; cursor moved %d -> %d", n, err, cursor, st.Cursor())
	}
	if d, _ := st.Event(ctx, withFrame); d.EndedAt == nil || !d.EndedAt.Equal(storeT0.Add(7*time.Second)) {
		t.Fatalf("event with frames ended at %v", d.EndedAt)
	}
	if d, _ := st.Event(ctx, noFrames); d.EndedAt == nil || !d.EndedAt.Equal(storeT0.Add(time.Minute)) {
		t.Fatalf("event without frames ended at %v", d.EndedAt)
	}

	id, _ := st.CreateEvent(ctx, NewEvent{CameraID: "front-door", CameraName: "Front", StartedAt: storeT0})
	rel, thumb := "2026/10/11/front-door/9_1.jpg", "2026/10/11/front-door/9_1_thumb.jpg"
	for _, p := range []string{rel, thumb} {
		full := filepath.Join(st.Root(), filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("jpeg"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	frameID, err := st.AddFrame(ctx, id, StoredFrame{Seq: 1, Path: rel, ThumbPath: thumb, Size: 8, CapturedAt: storeT0, Score: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	freed, err := st.RemoveEvent(ctx, id)
	if err != nil || freed != 8 {
		t.Fatalf("RemoveEvent = %d, %v", freed, err)
	}
	// The frames went with their event (ON DELETE CASCADE).
	var frames int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM frames WHERE event_id = ?`, id).Scan(&frames); err != nil || frames != 0 {
		t.Fatalf("frames of the removed event = %d, %v", frames, err)
	}
	if _, _, err := st.Frame(ctx, frameID); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("Frame of a removed event err = %v", err)
	}
	if _, err := st.Event(ctx, withFrame); err != nil {
		t.Fatalf("the other events must stay: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.Root(), "2026")); !os.IsNotExist(err) {
		t.Fatalf("empty date directories must be removed: %v", err)
	}
	if _, err := os.Stat(st.Root()); err != nil {
		t.Fatalf("the image root itself must stay: %v", err)
	}
	if _, err := st.RemoveEvent(ctx, id); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("second RemoveEvent err = %v", err)
	}
}

func TestEventStoreLastEventsPerCamera(t *testing.T) {
	st := newTestEventStore(t)
	ctx := context.Background()
	_, _ = st.CreateEvent(ctx, NewEvent{CameraID: "a", CameraName: "A", StartedAt: storeT0})
	lastA, _ := st.CreateEvent(ctx, NewEvent{CameraID: "a", CameraName: "A", StartedAt: storeT0.Add(time.Minute)})
	lastB, _ := st.CreateEvent(ctx, NewEvent{CameraID: "b", CameraName: "B", StartedAt: storeT0.Add(2 * time.Minute)})
	last, err := st.LastEvents(ctx)
	if err != nil || len(last) != 2 || last["a"].id != lastA || !last["a"].at.Equal(storeT0.Add(time.Minute)) || last["b"].id != lastB {
		t.Fatalf("LastEvents = %+v, %v", last, err)
	}
}
