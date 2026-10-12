package persondetect

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/dbutil"

	"modernc.org/sqlite" // also registers the "sqlite" driver that dbutil.Open uses
)

const (
	eventStoreSchemaVersion = 1
	detectionsParentDir     = "go2rtc"
	detectionsDBFile        = "detections.db"
	detectionsDirName       = "detections"
	eventPageDefault        = 50
	eventPageMax            = 100
)

var eventStoreSchema = []string{
	`CREATE TABLE IF NOT EXISTS rev_seq (id INTEGER PRIMARY KEY CHECK (id = 1), value INTEGER NOT NULL)`,
	`INSERT OR IGNORE INTO rev_seq(id, value) VALUES (1, 0)`,
	`CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		camera_id TEXT NOT NULL,
		camera_name TEXT NOT NULL,
		started_at INTEGER NOT NULL,
		ended_at INTEGER,
		max_score REAL NOT NULL DEFAULT 0,
		max_persons INTEGER NOT NULL DEFAULT 0,
		zones TEXT NOT NULL DEFAULT '[]',
		armed INTEGER NOT NULL DEFAULT 0,
		cover_frame_id INTEGER,
		cover_score REAL NOT NULL DEFAULT 0,
		frame_count INTEGER NOT NULL DEFAULT 0,
		bytes INTEGER NOT NULL DEFAULT 0,
		alerts TEXT NOT NULL DEFAULT '{}',
		rev INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS events_camera ON events(camera_id, id)`,
	`CREATE INDEX IF NOT EXISTS events_started ON events(started_at)`,
	`CREATE INDEX IF NOT EXISTS events_ended ON events(ended_at)`,
	`CREATE INDEX IF NOT EXISTS events_rev ON events(rev)`,
	`CREATE TABLE IF NOT EXISTS frames (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		seq INTEGER NOT NULL,
		path TEXT NOT NULL,
		thumb_path TEXT NOT NULL,
		size INTEGER NOT NULL,
		captured_at INTEGER NOT NULL,
		width INTEGER NOT NULL,
		height INTEGER NOT NULL,
		boxes TEXT NOT NULL,
		score REAL NOT NULL,
		rev INTEGER NOT NULL,
		UNIQUE(event_id, seq)
	)`,
	`CREATE TABLE IF NOT EXISTS arm_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		at INTEGER NOT NULL,
		mode TEXT NOT NULL,
		armed INTEGER NOT NULL,
		actor TEXT NOT NULL,
		rev INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS arm_log_rev ON arm_log(rev)`,
	`CREATE INDEX IF NOT EXISTS arm_log_at ON arm_log(at)`,
	`CREATE TABLE IF NOT EXISTS state (key TEXT PRIMARY KEY, value TEXT NOT NULL, rev INTEGER NOT NULL)`,
}

const (
	eventSummaryColumns = `id, camera_id, camera_name, started_at, ended_at, max_score, max_persons, zones, armed, cover_frame_id, frame_count, alerts`
	armLogColumns       = `id, at, mode, armed, actor`
)

// EventStore persists events, frames, the arm log and the arm state in
// <data>/go2rtc/detections.db. Every write runs in one transaction stamped with
// the next value of a single revision sequence; the change feed reads the rows
// above a cursor.
//
// Every list it returns is non-nil (events, frames, boxes, zones and the alerts
// map), and every timestamp is UTC truncated to milliseconds, which is how the
// store keeps them. AUTOINCREMENT ids are never reused, so image URLs that name a
// frame id can be cached as immutable.
type EventStore struct {
	db      *sql.DB
	root    string
	writeMu sync.Mutex
	rev     atomic.Int64
}

// NewEvent is the initial record of an event.
type NewEvent struct {
	CameraID   string
	CameraName string
	StartedAt  time.Time
	Armed      bool
	Stats      EventStats
	Alerts     map[string]AlertResult
}

// StoredFrame is one frame and its thumbnail on disk. Boxes are the boxes that
// count (see EventFrame); the store persists what it is given.
type StoredFrame struct {
	Seq        int
	Path       string // slash separated, relative to the detections root
	ThumbPath  string
	Size       int64 // bytes of the frame plus its thumbnail
	CapturedAt time.Time
	Width      int
	Height     int
	Boxes      []Box
	Score      float64
}

type armRecord struct {
	Mode      ArmMode
	Armed     bool
	ChangedAt time.Time
	ChangedBy string
}

type lastEventRef struct {
	id int64
	at time.Time
}

type eventRowScanner interface {
	Scan(dest ...any) error
}

// DetectionsPaths returns the database file and the image root below a data directory.
func DetectionsPaths(dataDir string) (dbPath, root string) {
	base := filepath.Join(dataDir, detectionsParentDir)
	return filepath.Join(base, detectionsDBFile), filepath.Join(base, detectionsDirName)
}

// OpenEventStore opens or creates <dataDir>/go2rtc/detections.db and the image
// root. Only a database that is really corrupt is moved aside (to
// .corrupt-<timestamp>) and replaced by a fresh one; any other open error, and a
// database from a newer schema, is returned with the file left untouched.
func OpenEventStore(dataDir string, logger *slog.Logger) (*EventStore, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, errors.New("persondetect: empty data directory")
	}
	if logger == nil {
		logger = slog.Default()
	}
	dbPath, root := DetectionsPaths(dataDir)
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("persondetect: create detections directory: %w", err)
	}
	db, err := openDetectionsDB(dbPath)
	if err != nil && isCorruptDatabase(err) {
		logger.Error("Person detection database is corrupt; starting a fresh one", "path", dbPath, "error", err)
		if moveErr := moveCorruptDatabaseAside(dbPath, time.Now().UTC()); moveErr != nil {
			return nil, fmt.Errorf("persondetect: move corrupt detections database aside: %w", moveErr)
		}
		db, err = openDetectionsDB(dbPath)
	}
	if err != nil {
		return nil, fmt.Errorf("persondetect: open detections database: %w", err)
	}
	version, err := dbutil.GetUserVersion(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("persondetect: read schema version: %w", err)
	}
	if version > eventStoreSchemaVersion {
		_ = db.Close()
		return nil, fmt.Errorf("persondetect: detections database was created by a newer AuraGo (schema %d)", version)
	}
	for _, stmt := range eventStoreSchema {
		if _, err := db.Exec(stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("persondetect: create schema: %w", err)
		}
	}
	if version < eventStoreSchemaVersion {
		if err := dbutil.SetUserVersion(db, eventStoreSchemaVersion); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("persondetect: set schema version: %w", err)
		}
	}
	st := &EventStore{db: db, root: root}
	var rev int64
	if err := db.QueryRow(`SELECT value FROM rev_seq WHERE id = 1`).Scan(&rev); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("persondetect: read revision: %w", err)
	}
	st.rev.Store(rev)
	return st, nil
}

// openDetectionsDB creates the database file with mode 0640 when it is missing
// and opens it. SQLite gives its WAL and SHM files the database file's mode, and
// <data>/go2rtc is bind-mounted (read-only) into the go2rtc container, so none of
// the three may be readable by other users.
func openDetectionsDB(dbPath string) (*sql.DB, error) {
	file, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o640)
	if err != nil {
		return nil, err
	}
	_ = file.Close()
	return dbutil.Open(dbPath)
}

// isCorruptDatabase reports whether an open error means the file is no usable
// SQLite database (as internal/retrogames does). dbutil's own corruption
// recovery is not used: it reacts to any open error, including I/O and
// permission failures, and renames the file to a fixed .bak name.
func isCorruptDatabase(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() & 0xff {
		case 11, 26: // SQLITE_CORRUPT, SQLITE_NOTADB
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "database integrity check failed") || strings.Contains(msg, "file is not a database")
}

// moveCorruptDatabaseAside renames the database and its WAL/SHM companions that
// exist to <name>.corrupt-<timestamp>.
func moveCorruptDatabaseAside(path string, now time.Time) error {
	stamp := now.Format("20060102-150405")
	for _, suffix := range []string{"", "-wal", "-shm"} {
		src := path + suffix
		if _, err := os.Stat(src); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if err := os.Rename(src, src+".corrupt-"+stamp); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the database.
func (st *EventStore) Close() error { return st.db.Close() }

// Root is the absolute image root.
func (st *EventStore) Root() string { return st.root }

// Cursor is the revision of the last committed write.
func (st *EventStore) Cursor() int64 { return st.rev.Load() }

// write runs fn in one transaction that owns the next revision.
func (st *EventStore) write(ctx context.Context, fn func(tx *sql.Tx, rev int64) error) error {
	st.writeMu.Lock()
	defer st.writeMu.Unlock()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE rev_seq SET value = value + 1 WHERE id = 1`); err != nil {
		return err
	}
	var rev int64
	if err := tx.QueryRowContext(ctx, `SELECT value FROM rev_seq WHERE id = 1`).Scan(&rev); err != nil {
		return err
	}
	if err := fn(tx, rev); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	st.rev.Store(rev)
	return nil
}

func unixMillis(t time.Time) int64 { return t.UnixMilli() }

// fromUnixMillis is the only way the store turns a stored time into a time.Time:
// UTC, millisecond precision.
func fromUnixMillis(v int64) time.Time { return time.UnixMilli(v).UTC() }

func sqlBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func nonNilBoxes(v []Box) []Box {
	if v == nil {
		return []Box{}
	}
	return v
}

func unknownEventError(id int64) error {
	return fmt.Errorf("%w: %d", ErrUnknownEvent, id)
}

func requireEventRow(res sql.Result, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return unknownEventError(id)
	}
	return nil
}

func checkRowLimit(limit int) error {
	if limit < 1 {
		return fmt.Errorf("persondetect: row limit %d is below 1", limit)
	}
	return nil
}

// CreateEvent records the start of an event and returns its id.
func (st *EventStore) CreateEvent(ctx context.Context, ev NewEvent) (int64, error) {
	zones, err := json.Marshal(nonNilStrings(ev.Stats.Zones))
	if err != nil {
		return 0, err
	}
	alerts := ev.Alerts
	if alerts == nil {
		alerts = map[string]AlertResult{}
	}
	encodedAlerts, err := json.Marshal(alerts)
	if err != nil {
		return 0, err
	}
	var id int64
	err = st.write(ctx, func(tx *sql.Tx, rev int64) error {
		res, err := tx.ExecContext(ctx, `INSERT INTO events(camera_id, camera_name, started_at, max_score, max_persons, zones, armed, alerts, rev) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ev.CameraID, ev.CameraName, unixMillis(ev.StartedAt), ev.Stats.MaxScore, ev.Stats.MaxPersons, string(zones), sqlBool(ev.Armed), string(encodedAlerts), rev)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("persondetect: create event: %w", err)
	}
	return id, nil
}

// AddFrame records a stored frame, adds its bytes to the event and makes it the
// cover when its score beats the current cover. An unknown event is
// ErrUnknownEvent.
func (st *EventStore) AddFrame(ctx context.Context, eventID int64, f StoredFrame) (int64, error) {
	boxes, err := json.Marshal(nonNilBoxes(f.Boxes))
	if err != nil {
		return 0, err
	}
	var frameID int64
	err = st.write(ctx, func(tx *sql.Tx, rev int64) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM events WHERE id = ?`, eventID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return unknownEventError(eventID)
			}
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO frames(event_id, seq, path, thumb_path, size, captured_at, width, height, boxes, score, rev) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			eventID, f.Seq, f.Path, f.ThumbPath, f.Size, unixMillis(f.CapturedAt), f.Width, f.Height, string(boxes), f.Score, rev)
		if err != nil {
			return err
		}
		if frameID, err = res.LastInsertId(); err != nil {
			return err
		}
		upd, err := tx.ExecContext(ctx, `UPDATE events SET
				frame_count = frame_count + 1,
				bytes = bytes + ?,
				cover_frame_id = CASE WHEN cover_frame_id IS NULL OR ? > cover_score THEN ? ELSE cover_frame_id END,
				cover_score = CASE WHEN cover_frame_id IS NULL OR ? > cover_score THEN ? ELSE cover_score END,
				rev = ?
			WHERE id = ?`, f.Size, f.Score, frameID, f.Score, f.Score, rev, eventID)
		if err != nil {
			return err
		}
		return requireEventRow(upd, eventID)
	})
	if err != nil {
		return 0, fmt.Errorf("persondetect: add frame: %w", err)
	}
	return frameID, nil
}

// UpdateEventStats raises the event's maximum score and person count and stores its zones.
func (st *EventStore) UpdateEventStats(ctx context.Context, id int64, stats EventStats) error {
	zones, err := json.Marshal(nonNilStrings(stats.Zones))
	if err != nil {
		return err
	}
	return st.write(ctx, func(tx *sql.Tx, rev int64) error {
		res, err := tx.ExecContext(ctx, `UPDATE events SET max_score = MAX(max_score, ?), max_persons = MAX(max_persons, ?), zones = ?, rev = ? WHERE id = ?`,
			stats.MaxScore, stats.MaxPersons, string(zones), rev, id)
		if err != nil {
			return err
		}
		return requireEventRow(res, id)
	})
}

// EndEvent sets the end time of an open event; an ended or unknown event is left alone.
func (st *EventStore) EndEvent(ctx context.Context, id int64, endedAt time.Time) error {
	return st.write(ctx, func(tx *sql.Tx, rev int64) error {
		_, err := tx.ExecContext(ctx, `UPDATE events SET ended_at = ?, rev = ? WHERE id = ? AND ended_at IS NULL`, unixMillis(endedAt), rev, id)
		return err
	})
}

// SetAlertResult records the outcome of one action of an event.
func (st *EventStore) SetAlertResult(ctx context.Context, id int64, action string, result AlertResult) error {
	return st.write(ctx, func(tx *sql.Tx, rev int64) error {
		var raw string
		if err := tx.QueryRowContext(ctx, `SELECT alerts FROM events WHERE id = ?`, id).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return unknownEventError(id)
			}
			return err
		}
		alerts := map[string]AlertResult{}
		_ = json.Unmarshal([]byte(raw), &alerts)
		if alerts == nil {
			alerts = map[string]AlertResult{}
		}
		alerts[action] = result
		encoded, err := json.Marshal(alerts)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE events SET alerts = ?, rev = ? WHERE id = ?`, string(encoded), rev, id)
		return err
	})
}

// CloseOpenEvents ends events a crash left open at their last stored frame (or
// their start) and returns how many it closed. All of them share one revision.
func (st *EventStore) CloseOpenEvents(ctx context.Context) (int64, error) {
	var open int64
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE ended_at IS NULL`).Scan(&open); err != nil {
		return 0, err
	}
	if open == 0 {
		return 0, nil
	}
	var closed int64
	err := st.write(ctx, func(tx *sql.Tx, rev int64) error {
		res, err := tx.ExecContext(ctx, `UPDATE events SET
				ended_at = COALESCE((SELECT MAX(captured_at) FROM frames WHERE frames.event_id = events.id), started_at),
				rev = ?
			WHERE ended_at IS NULL`, rev)
		if err != nil {
			return err
		}
		closed, err = res.RowsAffected()
		return err
	})
	return closed, err
}

func scanEventSummary(r eventRowScanner, extra ...any) (EventSummary, error) {
	var (
		s       EventSummary
		started int64
		ended   sql.NullInt64
		cover   sql.NullInt64
		zones   string
		alerts  string
		armed   int
	)
	dest := append([]any{&s.ID, &s.CameraID, &s.CameraName, &started, &ended, &s.MaxScore, &s.MaxPersons, &zones, &armed, &cover, &s.FrameCount, &alerts}, extra...)
	if err := r.Scan(dest...); err != nil {
		return EventSummary{}, err
	}
	s.StartedAt = fromUnixMillis(started)
	if ended.Valid {
		at := fromUnixMillis(ended.Int64)
		s.EndedAt = &at
	}
	if cover.Valid {
		id := cover.Int64
		s.CoverFrameID = &id
	}
	s.Armed = armed != 0
	if err := json.Unmarshal([]byte(zones), &s.Zones); err != nil || s.Zones == nil {
		s.Zones = []string{}
	}
	if err := json.Unmarshal([]byte(alerts), &s.Alerts); err != nil || s.Alerts == nil {
		s.Alerts = map[string]AlertResult{}
	}
	return s, nil
}

// readEventRevisions reads rows of eventSummaryColumns followed by rev and
// returns them with the revision of the last row (0 without rows).
func readEventRevisions(rows *sql.Rows) ([]EventSummary, int64, error) {
	defer rows.Close()
	out := []EventSummary{}
	var last int64
	for rows.Next() {
		var rev int64
		ev, err := scanEventSummary(rows, &rev)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, ev)
		last = rev
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, last, nil
}

// readArmLogRevisions reads rows of armLogColumns followed by rev and returns
// them with the revision of the last row (0 without rows).
func readArmLogRevisions(rows *sql.Rows) ([]ArmLogEntry, int64, error) {
	defer rows.Close()
	out := []ArmLogEntry{}
	var last int64
	for rows.Next() {
		var e ArmLogEntry
		var at, rev int64
		var mode string
		var armed int
		if err := rows.Scan(&e.ID, &at, &mode, &armed, &e.Actor, &rev); err != nil {
			return nil, 0, err
		}
		e.At, e.Mode, e.Armed = fromUnixMillis(at), ArmMode(mode), armed != 0
		out = append(out, e)
		last = rev
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, last, nil
}

// Events returns one page of events, newest first.
func (st *EventStore) Events(ctx context.Context, q EventQuery) (EventPage, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = eventPageDefault
	}
	if limit > eventPageMax {
		limit = eventPageMax
	}
	var where []string
	var args []any
	if q.CameraID != "" {
		where = append(where, "camera_id = ?")
		args = append(args, q.CameraID)
	}
	if !q.From.IsZero() {
		where = append(where, "started_at >= ?")
		args = append(args, unixMillis(q.From))
	}
	if !q.To.IsZero() {
		where = append(where, "started_at < ?")
		args = append(args, unixMillis(q.To))
	}
	if q.Before > 0 {
		where = append(where, "id < ?")
		args = append(args, q.Before)
	}
	query := `SELECT ` + eventSummaryColumns + ` FROM events`
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, " AND ")
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := st.db.QueryContext(ctx, query, args...)
	if err != nil {
		return EventPage{}, fmt.Errorf("persondetect: list events: %w", err)
	}
	defer rows.Close()
	page := EventPage{Events: []EventSummary{}}
	for rows.Next() {
		ev, err := scanEventSummary(rows)
		if err != nil {
			return EventPage{}, err
		}
		if len(page.Events) == limit {
			next := page.Events[limit-1].ID
			page.NextBefore = &next
			break
		}
		page.Events = append(page.Events, ev)
	}
	if err := rows.Err(); err != nil {
		return EventPage{}, err
	}
	return page, nil
}

// Event returns one event with its frames.
func (st *EventStore) Event(ctx context.Context, id int64) (EventDetail, error) {
	summary, err := scanEventSummary(st.db.QueryRowContext(ctx, `SELECT `+eventSummaryColumns+` FROM events WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return EventDetail{}, unknownEventError(id)
	}
	if err != nil {
		return EventDetail{}, err
	}
	rows, err := st.db.QueryContext(ctx, `SELECT id, seq, captured_at, score, width, height, boxes FROM frames WHERE event_id = ? ORDER BY seq`, id)
	if err != nil {
		return EventDetail{}, err
	}
	defer rows.Close()
	detail := EventDetail{EventSummary: summary, Frames: []EventFrame{}}
	for rows.Next() {
		var f EventFrame
		var captured int64
		var boxes string
		if err := rows.Scan(&f.ID, &f.Seq, &captured, &f.Score, &f.Width, &f.Height, &boxes); err != nil {
			return EventDetail{}, err
		}
		f.CapturedAt = fromUnixMillis(captured)
		if err := json.Unmarshal([]byte(boxes), &f.Boxes); err != nil || f.Boxes == nil {
			f.Boxes = []Box{}
		}
		detail.Frames = append(detail.Frames, f)
	}
	if err := rows.Err(); err != nil {
		return EventDetail{}, err
	}
	return detail, nil
}

// revisionPageFilter selects the rows of table whose revision lies in
// (after, upTo], up to and including the revision of the limit-th such row. A
// page therefore never splits a revision: one write can stamp several rows
// (CloseOpenEvents), and a caller continues after the page's last revision, so
// rows of that revision left out of the page would never be delivered. Its
// arguments are after, after, upTo, limit.
func revisionPageFilter(table string) string {
	return `rev > ? AND rev <= (SELECT COALESCE(MAX(rev), 0) FROM (SELECT rev FROM ` + table + ` WHERE rev > ? AND rev <= ? ORDER BY rev LIMIT ?))`
}

// EventsBetween returns the events whose revision lies in (after, upTo], oldest
// revision first. It returns limit rows, or more when the rows of the last
// revision do not fit (a page never splits a revision), or fewer when no more
// exist; last is the revision of the last row (0 without rows) and full reports
// that the limit was reached. limit must be at least 1.
func (st *EventStore) EventsBetween(ctx context.Context, after, upTo int64, limit int) ([]EventSummary, int64, bool, error) {
	if err := checkRowLimit(limit); err != nil {
		return nil, 0, false, err
	}
	rows, err := st.db.QueryContext(ctx, `SELECT `+eventSummaryColumns+`, rev FROM events WHERE `+revisionPageFilter("events")+` ORDER BY rev, id`,
		after, after, upTo, limit)
	if err != nil {
		return nil, 0, false, fmt.Errorf("persondetect: read changed events: %w", err)
	}
	out, last, err := readEventRevisions(rows)
	if err != nil {
		return nil, 0, false, fmt.Errorf("persondetect: read changed events: %w", err)
	}
	return out, last, len(out) >= limit, nil
}

// ArmLogBetween is EventsBetween for the arm log.
func (st *EventStore) ArmLogBetween(ctx context.Context, after, upTo int64, limit int) ([]ArmLogEntry, int64, bool, error) {
	if err := checkRowLimit(limit); err != nil {
		return nil, 0, false, err
	}
	rows, err := st.db.QueryContext(ctx, `SELECT `+armLogColumns+`, rev FROM arm_log WHERE `+revisionPageFilter("arm_log")+` ORDER BY rev, id`,
		after, after, upTo, limit)
	if err != nil {
		return nil, 0, false, fmt.Errorf("persondetect: read arm log: %w", err)
	}
	out, last, err := readArmLogRevisions(rows)
	if err != nil {
		return nil, 0, false, fmt.Errorf("persondetect: read arm log: %w", err)
	}
	return out, last, len(out) >= limit, nil
}

// EventsLatest returns the newest events by revision whose revision is at most
// upTo, at most limit rows, oldest revision first like EventsBetween. It backs
// the Changes(0) snapshot, which must not replay old rows as new. limit must be
// at least 1.
func (st *EventStore) EventsLatest(ctx context.Context, upTo int64, limit int) ([]EventSummary, error) {
	if err := checkRowLimit(limit); err != nil {
		return nil, err
	}
	rows, err := st.db.QueryContext(ctx, `SELECT `+eventSummaryColumns+`, rev FROM (
			SELECT `+eventSummaryColumns+`, rev FROM events WHERE rev <= ? ORDER BY rev DESC, id DESC LIMIT ?
		) ORDER BY rev, id`, upTo, limit)
	if err != nil {
		return nil, fmt.Errorf("persondetect: read latest events: %w", err)
	}
	out, _, err := readEventRevisions(rows)
	if err != nil {
		return nil, fmt.Errorf("persondetect: read latest events: %w", err)
	}
	return out, nil
}

// ArmLogLatest is EventsLatest for the arm log.
func (st *EventStore) ArmLogLatest(ctx context.Context, upTo int64, limit int) ([]ArmLogEntry, error) {
	if err := checkRowLimit(limit); err != nil {
		return nil, err
	}
	rows, err := st.db.QueryContext(ctx, `SELECT `+armLogColumns+`, rev FROM (
			SELECT `+armLogColumns+`, rev FROM arm_log WHERE rev <= ? ORDER BY rev DESC, id DESC LIMIT ?
		) ORDER BY rev, id`, upTo, limit)
	if err != nil {
		return nil, fmt.Errorf("persondetect: read latest arm log: %w", err)
	}
	out, _, err := readArmLogRevisions(rows)
	if err != nil {
		return nil, fmt.Errorf("persondetect: read latest arm log: %w", err)
	}
	return out, nil
}

// Frame returns the relative frame and thumbnail paths of a frame id.
func (st *EventStore) Frame(ctx context.Context, frameID int64) (string, string, error) {
	var path, thumb string
	err := st.db.QueryRowContext(ctx, `SELECT path, thumb_path FROM frames WHERE id = ?`, frameID).Scan(&path, &thumb)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("%w: frame %d", ErrUnknownEvent, frameID)
	}
	return path, thumb, err
}

// RemoveEvent deletes an event with its frames and image files and returns the freed bytes.
func (st *EventStore) RemoveEvent(ctx context.Context, id int64) (int64, error) {
	var files []string
	var freed int64
	err := st.write(ctx, func(tx *sql.Tx, rev int64) error {
		if err := tx.QueryRowContext(ctx, `SELECT bytes FROM events WHERE id = ?`, id).Scan(&freed); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return unknownEventError(id)
			}
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT path, thumb_path FROM frames WHERE event_id = ?`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var path, thumb string
			if err := rows.Scan(&path, &thumb); err != nil {
				_ = rows.Close()
				return err
			}
			files = append(files, path, thumb)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return 0, err
	}
	for _, rel := range files {
		st.removeFile(rel)
	}
	return freed, nil
}

// removeFile deletes one image below the root and then its empty parent directories.
func (st *EventStore) removeFile(rel string) {
	target := filepath.Join(st.root, filepath.FromSlash(rel))
	if !pathInside(st.root, target) {
		return
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
	for dir := filepath.Dir(target); pathInside(st.root, dir); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break
		}
	}
}

// TotalBytes is the sum of all stored frame and thumbnail bytes.
func (st *EventStore) TotalBytes(ctx context.Context) (int64, error) {
	var total int64
	err := st.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(bytes), 0) FROM events`).Scan(&total)
	return total, err
}

// LastEvents returns the newest event per camera.
func (st *EventStore) LastEvents(ctx context.Context) (map[string]lastEventRef, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT e.camera_id, e.id, e.started_at FROM events e
		JOIN (SELECT camera_id, MAX(id) AS id FROM events GROUP BY camera_id) newest ON newest.id = e.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]lastEventRef{}
	for rows.Next() {
		var camera string
		var id, started int64
		if err := rows.Scan(&camera, &id, &started); err != nil {
			return nil, err
		}
		out[camera] = lastEventRef{id: id, at: fromUnixMillis(started)}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadArm returns the persisted arm state; a fresh database is off.
func (st *EventStore) LoadArm(ctx context.Context) (armRecord, error) {
	rec := armRecord{Mode: ArmOff}
	rows, err := st.db.QueryContext(ctx, `SELECT key, value FROM state WHERE key IN ('arm_mode', 'arm_armed', 'arm_changed_at', 'arm_changed_by')`)
	if err != nil {
		return rec, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return armRecord{Mode: ArmOff}, err
		}
		switch key {
		case "arm_mode":
			if ValidArmMode(value) {
				rec.Mode = ArmMode(value)
			}
		case "arm_armed":
			rec.Armed = value == "1"
		case "arm_changed_at":
			if ms, err := strconv.ParseInt(value, 10, 64); err == nil && ms > 0 {
				rec.ChangedAt = fromUnixMillis(ms)
			}
		case "arm_changed_by":
			rec.ChangedBy = value
		}
	}
	if err := rows.Err(); err != nil {
		return armRecord{Mode: ArmOff}, err
	}
	return rec, nil
}

// RecordArm stores the arm state and appends the change to the arm log in one transaction.
func (st *EventStore) RecordArm(ctx context.Context, rec armRecord) error {
	return st.write(ctx, func(tx *sql.Tx, rev int64) error {
		for _, kv := range [][2]string{
			{"arm_mode", string(rec.Mode)},
			{"arm_armed", strconv.Itoa(sqlBool(rec.Armed))},
			{"arm_changed_at", strconv.FormatInt(unixMillis(rec.ChangedAt), 10)},
			{"arm_changed_by", rec.ChangedBy},
		} {
			if _, err := tx.ExecContext(ctx, `INSERT INTO state(key, value, rev) VALUES (?, ?, ?)
				ON CONFLICT(key) DO UPDATE SET value = excluded.value, rev = excluded.rev`, kv[0], kv[1], rev); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO arm_log(at, mode, armed, actor, rev) VALUES (?, ?, ?, ?, ?)`,
			unixMillis(rec.ChangedAt), string(rec.Mode), sqlBool(rec.Armed), rec.ChangedBy, rev)
		return err
	})
}

func (st *EventStore) queryIDs(ctx context.Context, query string, args ...any) ([]int64, error) {
	rows, err := st.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (st *EventStore) expiredEventIDs(ctx context.Context, cutoff time.Time, limit int) ([]int64, error) {
	return st.queryIDs(ctx, `SELECT id FROM events WHERE ended_at IS NOT NULL AND ended_at < ? ORDER BY ended_at, id LIMIT ?`, unixMillis(cutoff), limit)
}

func (st *EventStore) oldestEndedEventIDs(ctx context.Context, limit int) ([]int64, error) {
	return st.queryIDs(ctx, `SELECT id FROM events WHERE ended_at IS NOT NULL ORDER BY ended_at, id LIMIT ?`, limit)
}

func (st *EventStore) pruneArmLog(ctx context.Context, before time.Time) (int64, error) {
	var deleted int64
	err := st.write(ctx, func(tx *sql.Tx, rev int64) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM arm_log WHERE at < ?`, unixMillis(before))
		if err != nil {
			return err
		}
		deleted, err = res.RowsAffected()
		return err
	})
	return deleted, err
}

// pathInside reports whether target lies strictly below root (lexically).
func pathInside(root, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
