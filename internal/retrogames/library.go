package retrogames

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	maxFavorites   = 500
	maxBroken      = 500
	maxStoredPlays = 200
	maxRecent      = 50
)

// ErrLibraryFull is returned when favorites or broken markers hit their cap.
var ErrLibraryFull = errors.New("retrogames: library list is full")

// LibraryEntry is a favorite, a recent play or a broken marker.
type LibraryEntry struct {
	Game
	At        int64 `json:"at"`
	PlayCount int   `json:"play_count,omitempty"`
	Available bool  `json:"available"`
}

// Library groups the user's lists.
type Library struct {
	Favorites []LibraryEntry `json:"favorites"`
	Recent    []LibraryEntry `json:"recent"`
	Broken    []LibraryEntry `json:"broken"`
}

// SetFavorite adds (with a snapshot of g) or removes a favorite.
func (st *Store) SetFavorite(ctx context.Context, g Game, on bool, now time.Time) error {
	return st.setMarker(ctx, "favorites", "added_at", maxFavorites, g, on, now)
}

// SetBroken adds or removes a "doesn't start" marker.
func (st *Store) SetBroken(ctx context.Context, g Game, on bool, now time.Time) error {
	return st.setMarker(ctx, "broken", "marked_at", maxBroken, g, on, now)
}

func (st *Store) setMarker(ctx context.Context, table, timeColumn string, limit int, g Game, on bool, now time.Time) error {
	if !on {
		if _, err := st.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE identifier = ?`, g.Identifier); err != nil {
			return fmt.Errorf("retrogames: remove from %s: %w", table, err)
		}
		return nil
	}
	var exists int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE identifier = ?`, g.Identifier).Scan(&exists); err != nil {
		return fmt.Errorf("retrogames: read %s: %w", table, err)
	}
	if exists == 0 {
		var n int
		if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&n); err != nil {
			return fmt.Errorf("retrogames: count %s: %w", table, err)
		}
		if n >= limit {
			return ErrLibraryFull
		}
	}
	_, err := st.db.ExecContext(ctx, `INSERT INTO `+table+`(identifier, `+timeColumn+`, title, platform, system, year) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(identifier) DO UPDATE SET title = excluded.title, platform = excluded.platform, system = excluded.system, year = excluded.year`,
		g.Identifier, now.Unix(), g.Title, g.Platform, g.System, g.Year)
	if err != nil {
		return fmt.Errorf("retrogames: write %s: %w", table, err)
	}
	return nil
}

// RecordPlay counts one start of g and keeps the newest maxStoredPlays rows.
func (st *Store) RecordPlay(ctx context.Context, g Game, now time.Time) error {
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("retrogames: begin play: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO plays(identifier, last_played_at, play_count, title, platform, system, year) VALUES (?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(identifier) DO UPDATE SET last_played_at = excluded.last_played_at, play_count = plays.play_count + 1,
			title = excluded.title, platform = excluded.platform, system = excluded.system, year = excluded.year`,
		g.Identifier, now.Unix(), g.Title, g.Platform, g.System, g.Year); err != nil {
		return fmt.Errorf("retrogames: record play: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM plays WHERE identifier NOT IN (SELECT identifier FROM plays ORDER BY last_played_at DESC, identifier LIMIT ?)`, maxStoredPlays); err != nil {
		return fmt.Errorf("retrogames: trim plays: %w", err)
	}
	return tx.Commit()
}

// Library returns favorites, the newest plays and broken markers. Available is
// true when the game is in the index; the Service widens it (Hall of Fame,
// never-synced platforms).
func (st *Store) Library(ctx context.Context) (Library, error) {
	var lib Library
	var err error
	if lib.Favorites, err = st.readEntries(ctx, `SELECT f.identifier, f.title, f.platform, f.system, f.year, f.added_at, 0, g.identifier IS NOT NULL
		FROM favorites f LEFT JOIN games g ON g.identifier = f.identifier ORDER BY f.added_at DESC, f.identifier LIMIT ?`, maxFavorites); err != nil {
		return Library{}, err
	}
	if lib.Recent, err = st.readEntries(ctx, `SELECT p.identifier, p.title, p.platform, p.system, p.year, p.last_played_at, p.play_count, g.identifier IS NOT NULL
		FROM plays p LEFT JOIN games g ON g.identifier = p.identifier ORDER BY p.last_played_at DESC, p.identifier LIMIT ?`, maxRecent); err != nil {
		return Library{}, err
	}
	if lib.Broken, err = st.readEntries(ctx, `SELECT b.identifier, b.title, b.platform, b.system, b.year, b.marked_at, 0, g.identifier IS NOT NULL
		FROM broken b LEFT JOIN games g ON g.identifier = b.identifier ORDER BY b.marked_at DESC, b.identifier LIMIT ?`, maxBroken); err != nil {
		return Library{}, err
	}
	return lib, nil
}

func (st *Store) readEntries(ctx context.Context, query string, limit int) ([]LibraryEntry, error) {
	rows, err := st.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("retrogames: read library: %w", err)
	}
	defer rows.Close()
	out := []LibraryEntry{}
	for rows.Next() {
		var e LibraryEntry
		if err := rows.Scan(&e.Identifier, &e.Title, &e.Platform, &e.System, &e.Year, &e.At, &e.PlayCount, &e.Available); err != nil {
			return nil, fmt.Errorf("retrogames: scan library entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Snapshot returns the stored title/platform/system/year of a game that is
// in any library list, for games that left the index.
func (st *Store) Snapshot(ctx context.Context, id string) (Game, bool, error) {
	var g Game
	err := st.db.QueryRowContext(ctx, `SELECT identifier, title, platform, system, year FROM (
		SELECT identifier, title, platform, system, year FROM favorites WHERE identifier = ?
		UNION ALL SELECT identifier, title, platform, system, year FROM plays WHERE identifier = ?
		UNION ALL SELECT identifier, title, platform, system, year FROM broken WHERE identifier = ?) LIMIT 1`, id, id, id).
		Scan(&g.Identifier, &g.Title, &g.Platform, &g.System, &g.Year)
	if errors.Is(err, sql.ErrNoRows) {
		return Game{}, false, nil
	}
	if err != nil {
		return Game{}, false, fmt.Errorf("retrogames: read snapshot: %w", err)
	}
	return g, true, nil
}
