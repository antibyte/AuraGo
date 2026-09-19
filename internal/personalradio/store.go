package personalradio

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/dbutil"
	_ "modernc.org/sqlite"
)

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func openStore(dir string) (*sql.DB, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create radio store: %w", err)
	}
	db, err := dbutil.Open(filepath.Join(dir, "radio.db"))
	if err != nil {
		return nil, err
	}
	// Radio migrations only touch the app's isolated database.
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_meta(version INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS stations(id TEXT PRIMARY KEY, body TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS assets(id TEXT PRIMARY KEY, hash TEXT UNIQUE NOT NULL, body TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS station_tracks(station TEXT NOT NULL REFERENCES stations(id) ON DELETE CASCADE, asset TEXT NOT NULL REFERENCES assets(id), favorite INTEGER NOT NULL DEFAULT 0, blocked INTEGER NOT NULL DEFAULT 0, weight INTEGER NOT NULL DEFAULT 100, PRIMARY KEY(station,asset));
CREATE TABLE IF NOT EXISTS plays(segment TEXT PRIMARY KEY, station TEXT NOT NULL, asset TEXT NOT NULL, started TEXT NOT NULL, ended TEXT, outcome TEXT NOT NULL DEFAULT 'started');
CREATE INDEX IF NOT EXISTS plays_station ON plays(station,started);
CREATE INDEX IF NOT EXISTS plays_asset ON plays(station,asset,started);
CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY, station TEXT NOT NULL, kind TEXT NOT NULL, day TEXT NOT NULL, amount INTEGER NOT NULL, status TEXT NOT NULL, result TEXT NOT NULL DEFAULT '');
CREATE INDEX IF NOT EXISTS jobs_day ON jobs(station,day,kind);
CREATE TABLE IF NOT EXISTS news(id TEXT PRIMARY KEY, station TEXT NOT NULL, body TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS editorial(station TEXT NOT NULL, created TEXT NOT NULL, body TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS registry_ignored(station TEXT NOT NULL REFERENCES stations(id) ON DELETE CASCADE, media_id INTEGER NOT NULL, PRIMARY KEY(station,media_id));
INSERT INTO schema_meta(version) SELECT 1 WHERE NOT EXISTS(SELECT 1 FROM schema_meta);
UPDATE jobs SET status='interrupted' WHERE status='running';`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate radio store: %w", err)
	}
	var version int
	if err = db.QueryRow("SELECT version FROM schema_meta").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version < 2 {
		tx, e := db.Begin()
		if e != nil {
			db.Close()
			return nil, e
		}
		// Correct only the old factory reserve once; retain customized profiles.
		_, e = tx.Exec(`ALTER TABLE station_tracks ADD COLUMN genre TEXT NOT NULL DEFAULT '';
UPDATE stations SET body=json_set(body,'$.reserve_minutes',0,'$.min_tracks',2,'$.revision',COALESCE(json_extract(body,'$.revision'),0)+1)
WHERE json_extract(body,'$.reserve_minutes')=30 AND json_extract(body,'$.min_tracks')=8;
UPDATE schema_meta SET version=2;`)
		if e == nil {
			e = tx.Commit()
		} else {
			tx.Rollback()
		}
		if e != nil {
			db.Close()
			return nil, fmt.Errorf("migrate radio startup: %w", e)
		}
	}
	// Playback never resumes automatically after restart. Reap only app-owned
	// temporary files; durable library assets and provider originals survive.
	if entries, e := os.ReadDir(dir); e == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) == 36 && filepath.Ext(name) == ".wav" {
				id := name[:32]
				if _, e = hex.DecodeString(id); e != nil {
					continue
				}
				var found int
				if db.QueryRow("SELECT 1 FROM assets WHERE id=?", id).Scan(&found) == sql.ErrNoRows {
					_ = os.Remove(filepath.Join(dir, name))
				}
			}
		}
	}
	return db, nil
}

func (s *Service) Stations() ([]Station, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query("SELECT body FROM stations ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Station{}
	for rows.Next() {
		var b string
		var v Station
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) station(id string) (Station, error) {
	var b string
	var v Station
	err := s.db.QueryRow("SELECT body FROM stations WHERE id=?", id).Scan(&b)
	if err == sql.ErrNoRows {
		return v, ErrNotFound
	}
	if err != nil {
		return v, err
	}
	err = json.Unmarshal([]byte(b), &v)
	return v, err
}
func (s *Service) SaveStation(v Station) (Station, error) {
	if err := v.Validate(); err != nil {
		return v, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.ID == "" {
		var count int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM stations").Scan(&count); err != nil {
			return v, err
		}
		if count >= 32 {
			return v, ErrLimit
		}
		v.ID = newID()
		v.Revision = 1
	} else {
		old, err := s.station(v.ID)
		if err != nil {
			return v, err
		}
		if old.Revision != v.Revision {
			return v, ErrConflict
		}
		if s.state.StationID == v.ID && s.state.Status != "stopped" {
			return v, ErrConflict
		}
		v.Revision++
	}
	b, _ := json.Marshal(v)
	_, err := s.db.Exec("INSERT INTO stations(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body", v.ID, string(b))
	return v, err
}
func (s *Service) DeleteStation(id string, revision int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.station(id)
	if err != nil {
		return err
	}
	if v.Revision != revision || (s.state.StationID == id && s.state.Status != "stopped") {
		return ErrConflict
	}
	_, err = s.db.Exec("DELETE FROM stations WHERE id=?", id)
	return err
}
func (s *Service) tracks(id string) ([]Track, error) {
	rows, err := s.db.Query(`SELECT a.body,t.favorite,t.blocked,t.weight,t.genre,
(SELECT COUNT(*) FROM plays p WHERE p.station=t.station AND p.asset=t.asset),
COALESCE((SELECT MAX(started) FROM plays p WHERE p.station=t.station AND p.asset=t.asset),'')
FROM station_tracks t JOIN assets a ON a.id=t.asset WHERE t.station=? ORDER BY a.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Track{}
	for rows.Next() {
		var b, last, genre string
		var t Track
		var favorite, blocked, weight, plays int
		if err = rows.Scan(&b, &favorite, &blocked, &weight, &genre, &plays, &last); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(b), &t); err != nil {
			return nil, err
		}
		t.Favorite = favorite != 0
		if genre != "" {
			t.Genre = genre
		}
		t.Blocked = blocked != 0
		t.Weight = weight
		t.Plays = plays
		t.LastPlayed, _ = time.Parse(time.RFC3339Nano, last)
		out = append(out, t)
	}
	return out, rows.Err()
}
func (s *Service) Tracks(id string) ([]Track, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.station(id); err != nil {
		return nil, err
	}
	return s.tracks(id)
}
func (s *Service) UpdateTrack(station, id string, favorite, blocked bool, weight int) error {
	if weight < 1 || weight > 200 {
		return ErrLimit
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.db.Exec("UPDATE station_tracks SET favorite=?,blocked=?,weight=? WHERE station=? AND asset=?", favorite, blocked, weight, station, id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	// Preserve the already committed transition; the client stops a blocked next
	// track before starting it when it refreshes the queue.
	if s.state.StationID == station && blocked {
		s.removeTrackLocked(id)
	}
	return nil
}
func (s *Service) RemoveTrack(station, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO registry_ignored(station,media_id)
SELECT t.station,json_extract(a.body,'$.media_id') FROM station_tracks t JOIN assets a ON a.id=t.asset
WHERE t.station=? AND t.asset=? AND json_extract(a.body,'$.media_id')>0`, station, id); err != nil {
		return err
	}
	_, err := s.db.Exec("DELETE FROM station_tracks WHERE station=? AND asset=?", station, id)
	if s.state.StationID == station {
		s.removeTrackLocked(id)
	}
	return err
}

// DeleteUnused removes only normalized radio copies without station references.
// Shared media originals and local source files are never touched.
func (s *Service) DeleteUnused() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Status != "stopped" || s.musicActive {
		return 0, ErrConflict
	}
	rows, err := s.db.Query("SELECT id FROM assets WHERE id NOT IN (SELECT asset FROM station_tracks)")
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err = os.Remove(filepath.Join(s.dir, id+".wav")); err != nil && !os.IsNotExist(err) {
			return n, err
		}
		if _, err = s.db.Exec("DELETE FROM assets WHERE id=?", id); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (s *Service) News(id string) []Segment { s.mu.Lock(); defer s.mu.Unlock(); return s.loadNews(id) }
func (s *Service) removeTrackLocked(id string) {
	q := s.state.Queue[:0]
	for _, x := range s.state.Queue {
		if x.TrackID != id || x.ID == s.state.Current {
			q = append(q, x)
		}
	}
	s.state.Queue = q
}

func (s *Service) reserve(station, kind string, amount, limit int) (string, error) {
	day := s.now().UTC().Format("2006-01-02")
	var used int
	if err := s.db.QueryRow("SELECT COALESCE(SUM(amount),0) FROM jobs WHERE station=? AND kind=? AND day=?", station, kind, day).Scan(&used); err != nil {
		return "", err
	}
	// Failed and interrupted requests conservatively retain their reservation:
	// providers can have charged even when their response was lost.
	if amount <= 0 || used+amount > limit {
		return "", ErrLimit
	}
	id := newID()
	_, err := s.db.Exec("INSERT INTO jobs(id,station,kind,day,amount,status) VALUES(?,?,?,?,?,'running')", id, station, kind, day, amount)
	return id, err
}
func (s *Service) finishJob(id string, err error, result string) {
	status := "done"
	if err != nil {
		status = "failed"
	}
	if _, e := s.db.Exec("UPDATE jobs SET status=?,result=? WHERE id=?", status, result, id); e != nil {
		s.state.Code = "radio_storage_error"
	}
}

func (s *Service) recent(id string) []string {
	rows, err := s.db.Query("SELECT body FROM editorial WHERE station=? ORDER BY created DESC LIMIT 8", id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var b string
		if rows.Scan(&b) == nil {
			out = append(out, b)
		}
	}
	return out
}
