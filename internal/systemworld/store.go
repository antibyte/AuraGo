// Package systemworld persists only typed, bounded telemetry, never source payloads.
package systemworld

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Entity struct {
	ID       string             `json:"id"`
	Kind     string             `json:"kind"`
	District string             `json:"district"`
	Label    string             `json:"label"`
	State    string             `json:"state"`
	At       int64              `json:"at"`
	Actions  []string           `json:"actions,omitempty"`
	Source   string             `json:"source,omitempty"`
	Model    string             `json:"model,omitempty"`
	Provider string             `json:"provider,omitempty"`
	Values   map[string]float64 `json:"values,omitempty"`
}
type Snapshot struct {
	At       int64              `json:"at"`
	Metrics  map[string]float64 `json:"metrics"`
	Entities []Entity           `json:"entities"`
	// A bounded producer dropped deltas. Preserve the sample but mark the preceding replay gap.
	IncompleteBefore bool `json:"-"`
}
type Event struct {
	ID     int64  `json:"id"`
	At     int64  `json:"at"`
	Entity Entity `json:"entity"`
}
type Sample struct {
	At      int64   `json:"at"`
	Key     string  `json:"key"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}
type Store struct{ db *sql.DB }

func NewStore(ctx context.Context, db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("system world database unavailable")
	}
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS system_world_metrics (minute INTEGER,key TEXT,min REAL,max REAL,total REAL,count INTEGER,PRIMARY KEY(minute,key));
CREATE TABLE IF NOT EXISTS system_world_checkpoints (at INTEGER PRIMARY KEY,data TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS system_world_events (id INTEGER PRIMARY KEY AUTOINCREMENT,at INTEGER NOT NULL,entity TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS system_world_events_at ON system_world_events(at);`)
	if err != nil {
		return nil, fmt.Errorf("initialize system world: %w", err)
	}
	return &Store{db: db}, nil
}

// Save atomically publishes a complete checkpoint and its aggregates. Five-minute
// checkpoints plus state deltas keep large installations bounded within 24 hours.
func (s *Store) Save(ctx context.Context, snap Snapshot, changes []Entity) error {
	if len(snap.Entities) > 1000 || len(snap.Metrics) > 64 || len(changes) > 1000 {
		return fmt.Errorf("system world sample exceeds limit")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	minute := snap.At / 60000 * 60000
	cutoff := snap.At - int64(24*time.Hour/time.Millisecond)
	for key, value := range snap.Metrics {
		if len(key) > 64 || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO system_world_metrics VALUES(?,?,?,?,?,1) ON CONFLICT(minute,key) DO UPDATE SET min=MIN(min,excluded.min),max=MAX(max,excluded.max),total=total+excluded.total,count=count+1`, minute, key, value, value, value)
		if err != nil {
			return fmt.Errorf("record system world metric: %w", err)
		}
	}
	for _, e := range changes {
		e.Actions = nil
		data, _ := json.Marshal(e)
		if _, err = tx.ExecContext(ctx, `INSERT INTO system_world_events(at,entity) VALUES(?,?)`, e.At, string(data)); err != nil {
			return err
		}
	}
	var last sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT MAX(at) FROM system_world_checkpoints`).Scan(&last); err != nil {
		return err
	}
	if snap.IncompleteBefore {
		if _, err = tx.ExecContext(ctx, `DELETE FROM system_world_checkpoints WHERE at=?`, last.Int64); err != nil {
			return err
		}
	}
	if snap.IncompleteBefore || !last.Valid || snap.At-last.Int64 >= 300000 {
		for i := range snap.Entities {
			snap.Entities[i].Actions = nil
		}
		data, _ := json.Marshal(snap)
		if len(data) > 768*1024 {
			return fmt.Errorf("system world checkpoint exceeds limit")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO system_world_checkpoints(at,data) VALUES(?,?)`, snap.At, string(data)); err != nil {
			return err
		}
	}
	for _, q := range []string{`DELETE FROM system_world_metrics WHERE minute < ?`, `DELETE FROM system_world_events WHERE at < ?`, `DELETE FROM system_world_checkpoints WHERE at < ?`} {
		if _, err = tx.ExecContext(ctx, q, cutoff); err != nil {
			return err
		}
	}
	// If a storm truncates deltas, invalidate checkpoints that relied on those
	// deltas. Their metric history remains visible, with an explicit replay gap.
	var droppedThrough sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT MAX(at) FROM system_world_events WHERE id <= COALESCE((SELECT id FROM system_world_events ORDER BY id DESC LIMIT 1 OFFSET 50000),0)`).Scan(&droppedThrough); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM system_world_events WHERE id <= COALESCE((SELECT id FROM system_world_events ORDER BY id DESC LIMIT 1 OFFSET 50000),0)`); err != nil {
		return err
	}
	if droppedThrough.Valid {
		if _, err = tx.ExecContext(ctx, `DELETE FROM system_world_checkpoints WHERE at<?`, droppedThrough.Int64); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) History(ctx context.Context, since int64) ([]Sample, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT minute,key,min,max,total/count,count FROM system_world_metrics WHERE minute>=? ORDER BY minute,key LIMIT 92160`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Sample{}
	for rows.Next() {
		var v Sample
		if err = rows.Scan(&v.At, &v.Key, &v.Min, &v.Max, &v.Average, &v.Count); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Events(ctx context.Context, since, after int64, limit int) ([]Event, error) {
	limit = max(1, min(limit, 200))
	rows, err := s.db.QueryContext(ctx, `SELECT id,at,entity FROM system_world_events WHERE at>=? AND id>? ORDER BY id LIMIT ?`, since, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var v Event
		var data string
		if err = rows.Scan(&v.ID, &v.At, &data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(data), &v.Entity); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) At(ctx context.Context, at int64) (Snapshot, error) {
	var data string
	var snap Snapshot
	err := s.db.QueryRowContext(ctx, `SELECT data FROM system_world_checkpoints WHERE at<=? ORDER BY at DESC LIMIT 1`, at).Scan(&data)
	if err != nil {
		return snap, err
	}
	if err = json.Unmarshal([]byte(data), &snap); err != nil {
		return snap, err
	}
	// Checkpoints arrive every five minutes. Never extend an older checkpoint into a missing interval.
	if at-snap.At >= 300000 {
		return Snapshot{}, sql.ErrNoRows
	}
	rows, err := s.db.QueryContext(ctx, `SELECT entity FROM system_world_events WHERE at>? AND at<=? ORDER BY id LIMIT 50000`, snap.At, at)
	if err != nil {
		return snap, err
	}
	defer rows.Close()
	entities := map[string]Entity{}
	for _, e := range snap.Entities {
		entities[e.ID] = e
	}
	for rows.Next() {
		var encoded string
		var e Entity
		if err = rows.Scan(&encoded); err != nil {
			return snap, err
		}
		if err = json.Unmarshal([]byte(encoded), &e); err != nil {
			return snap, err
		}
		if e.State == "removed" {
			delete(entities, e.ID)
		} else {
			entities[e.ID] = e
		}
	}
	if err = rows.Err(); err != nil {
		return snap, err
	}
	rows.Close()
	snap.Entities = []Entity{}
	for _, e := range entities {
		e.Actions = nil
		snap.Entities = append(snap.Entities, e)
	}
	sort.Slice(snap.Entities, func(i, j int) bool { return snap.Entities[i].ID < snap.Entities[j].ID })
	// Return actual minute sample time so the client can show gaps/age.
	var stamp sql.NullInt64
	if err = s.db.QueryRowContext(ctx, `SELECT MAX(minute) FROM system_world_metrics WHERE minute<=? AND minute>=?`, at, at-90000).Scan(&stamp); err != nil {
		return snap, err
	}
	if !stamp.Valid {
		return Snapshot{}, sql.ErrNoRows
	}
	metricRows, err := s.db.QueryContext(ctx, `SELECT key,CASE WHEN key LIKE 'observed:%' THEN max ELSE total/count END FROM system_world_metrics WHERE minute=?`, stamp.Int64)
	if err != nil {
		return snap, err
	}
	defer metricRows.Close()
	snap.Metrics = map[string]float64{}
	for metricRows.Next() {
		var k string
		var v float64
		if err = metricRows.Scan(&k, &v); err != nil {
			return snap, err
		}
		snap.Metrics[k] = v
	}
	for i := range snap.Entities {
		e := &snap.Entities[i]
		if e.Kind != "tool" {
			if observed := int64(snap.Metrics["observed:"+e.Source]); observed > e.At {
				e.At = min(observed, at)
			}
		}
	}
	// Freshness receipts are internal replay metadata, not displayable metrics.
	for key := range snap.Metrics {
		if strings.HasPrefix(key, "observed:") {
			delete(snap.Metrics, key)
		}
	}
	snap.At = stamp.Int64
	return snap, metricRows.Err()
}
