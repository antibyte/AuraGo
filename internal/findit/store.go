package findit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aurago/internal/dbutil"
	"aurago/internal/fileutil"
	"aurago/internal/uid"

	_ "modernc.org/sqlite"
)

// Open opens the standalone FindIt database and applies its schema migrations.
func Open(dbPath string) (*Store, error) {
	db, err := dbutil.Open(dbPath, dbutil.WithImmediateTransactions(), dbutil.WithBusyTimeout(5000))
	if err != nil {
		return nil, wrapError("unavailable", "open FindIt database", err)
	}
	success := false
	defer func() {
		if !success {
			_ = db.Close()
		}
	}()
	if err := migrate(dbPath, db); err != nil {
		return nil, err
	}
	if err := seedBuiltInTemplates(db); err != nil {
		return nil, wrapError("unavailable", "initialize FindIt catalog", err)
	}
	success = true
	return &Store{db: db}, nil
}

// Close releases the database handle. A nil store is safe to close.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func migrate(dbPath string, db *sql.DB) error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return wrapError("unavailable", "read FindIt schema version", err)
	}
	if version > SchemaVersion {
		return domainError("unavailable", "FindIt database schema is newer than this version")
	}
	if version == SchemaVersion {
		return nil
	}
	var exists int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&exists); err != nil {
		return wrapError("unavailable", "inspect FindIt database schema", err)
	}
	if version == 0 && exists > 0 {
		return domainError("unavailable", "FindIt database has an unknown schema")
	}
	if dbPath != ":memory:" && version > 0 {
		info, err := os.Stat(dbPath)
		if err == nil && info.Size() > 0 {
			if err := backupDatabase(db); err != nil {
				return wrapError("unavailable", "back up FindIt database before migration", err)
			}
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return wrapError("unavailable", "inspect FindIt database before migration", err)
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return wrapError("unavailable", "begin FindIt schema migration", err)
	}
	defer tx.Rollback()
	for _, statement := range finditSchema {
		if _, err := tx.Exec(statement); err != nil {
			return wrapError("unavailable", "create FindIt schema", err)
		}
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion)); err != nil {
		return wrapError("unavailable", "set FindIt schema version", err)
	}
	if err := tx.Commit(); err != nil {
		return wrapError("unavailable", "commit FindIt schema migration", err)
	}
	return nil
}

func backupDatabase(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA database_list`)
	if err != nil {
		return err
	}
	var path string
	for rows.Next() {
		var sequence int
		var name, file string
		if err := rows.Scan(&sequence, &name, &file); err != nil {
			_ = rows.Close()
			return err
		}
		if name == "main" {
			path = file
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	backup := fmt.Sprintf("%s.findit-v%d-%s.bak", path, SchemaVersion, uid.New())
	if _, err := db.Exec(`VACUUM main INTO ?`, backup); err != nil {
		return err
	}
	if err := fileutil.RestrictToOwner(backup); err != nil {
		return err
	}
	pruneBackups(path)
	return nil
}

func pruneBackups(path string) {
	matches, err := filepath.Glob(path + ".findit-v*.bak")
	if err != nil || len(matches) <= 3 {
		return
	}
	entries := make([]os.FileInfo, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err == nil {
			entries = append(entries, info)
		}
	}
	if len(entries) <= 3 {
		return
	}
	// Backup names contain monotonic UUID entropy rather than timestamps; order by file mtime.
	for len(matches) > 3 {
		oldestPath := ""
		oldest := time.Time{}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err == nil && (oldestPath == "" || info.ModTime().Before(oldest)) {
				oldestPath, oldest = match, info.ModTime()
			}
		}
		if oldestPath == "" || os.Remove(oldestPath) != nil {
			return
		}
		filtered := matches[:0]
		for _, match := range matches {
			if match != oldestPath {
				filtered = append(filtered, match)
			}
		}
		matches = filtered
	}
}

var finditSchema = []string{
	`CREATE TABLE IF NOT EXISTS templates (
		id TEXT PRIMARY KEY, version INTEGER NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL, built_in INTEGER NOT NULL DEFAULT 0, fields_json TEXT NOT NULL,
		archived INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS items (
		id TEXT PRIMARY KEY, version INTEGER NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		category TEXT NOT NULL, template_id TEXT NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
		manufacturer TEXT NOT NULL DEFAULT '', part_number TEXT NOT NULL DEFAULT '', unit TEXT NOT NULL,
		minimum_stock_micro INTEGER NOT NULL DEFAULT 0 CHECK(minimum_stock_micro >= 0),
		package_family TEXT NOT NULL DEFAULT '', pin_count INTEGER NOT NULL DEFAULT 0 CHECK(pin_count >= 0),
		pitch_mm REAL, length_mm REAL, width_mm REAL, height_mm REAL, package_size TEXT NOT NULL DEFAULT '',
		size_system TEXT NOT NULL DEFAULT '', tags_json TEXT NOT NULL, datasheet_urls_json TEXT NOT NULL,
		image_id TEXT NOT NULL DEFAULT '', icon_id TEXT NOT NULL DEFAULT '', track_serials INTEGER NOT NULL DEFAULT 0,
		properties_json TEXT NOT NULL, archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS item_properties (
		item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE, field_key TEXT NOT NULL, value_type TEXT NOT NULL,
		text_value TEXT, number_value REAL, bool_value INTEGER, PRIMARY KEY(item_id, field_key)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_findit_item_property_number ON item_properties(field_key, number_value) WHERE number_value IS NOT NULL`,
	`CREATE INDEX IF NOT EXISTS idx_findit_item_property_text ON item_properties(field_key, text_value) WHERE text_value IS NOT NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_findit_item_part_number ON items(lower(trim(manufacturer)), lower(trim(part_number))) WHERE archived=0 AND trim(part_number)<>''`,
	`CREATE INDEX IF NOT EXISTS idx_findit_items_category ON items(category, archived, name COLLATE NOCASE)`,
	`CREATE INDEX IF NOT EXISTS idx_findit_items_template ON items(template_id, archived)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS findit_items_fts USING fts5(item_id UNINDEXED, name, description, manufacturer, part_number, category, package_family, tags, properties, tokenize='unicode61 remove_diacritics 2', prefix='2 3')`,
	`CREATE TABLE IF NOT EXISTS locations (
		id TEXT PRIMARY KEY, version INTEGER NOT NULL, parent_id TEXT REFERENCES locations(id) ON DELETE RESTRICT,
		name TEXT NOT NULL, kind TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_findit_location_sibling_name ON locations(ifnull(parent_id,''), lower(trim(name))) WHERE archived=0`,
	`CREATE INDEX IF NOT EXISTS idx_findit_location_parent ON locations(parent_id, archived, name COLLATE NOCASE)`,
	`CREATE TABLE IF NOT EXISTS stocks (
		id TEXT PRIMARY KEY, item_id TEXT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
		location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
		quantity_micro INTEGER NOT NULL CHECK(quantity_micro >= 0), lot TEXT NOT NULL DEFAULT '',
		condition TEXT NOT NULL DEFAULT 'new', serial TEXT NOT NULL DEFAULT '', serial_norm TEXT NOT NULL DEFAULT '',
		note TEXT NOT NULL DEFAULT '', version INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_findit_stock_serial ON stocks(item_id, serial_norm) WHERE serial_norm<>''`,
	`CREATE INDEX IF NOT EXISTS idx_findit_stock_item_location ON stocks(item_id, location_id, quantity_micro)`,
	`CREATE INDEX IF NOT EXISTS idx_findit_stock_location ON stocks(location_id, item_id)`,
	`CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY, version INTEGER NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK(status IN ('draft','released','cancelled','completed')),
		bom_revision INTEGER NOT NULL DEFAULT 1, released_revision INTEGER NOT NULL DEFAULT 0,
		archived INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS project_lines (
		id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
		item_id TEXT NOT NULL REFERENCES items(id) ON DELETE RESTRICT, quantity_micro INTEGER NOT NULL CHECK(quantity_micro > 0),
		note TEXT NOT NULL DEFAULT '', active INTEGER NOT NULL DEFAULT 1
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS idx_findit_project_line_active ON project_lines(project_id,item_id) WHERE active=1`,
	`CREATE TABLE IF NOT EXISTS project_releases (
		project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT, revision INTEGER NOT NULL,
		lines_json TEXT NOT NULL, released_at TEXT NOT NULL, PRIMARY KEY(project_id, revision)
	)`,
	`CREATE TABLE IF NOT EXISTS reservations (
		id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
		line_id TEXT NOT NULL REFERENCES project_lines(id) ON DELETE RESTRICT,
		stock_id TEXT NOT NULL REFERENCES stocks(id) ON DELETE RESTRICT,
		quantity_micro INTEGER NOT NULL CHECK(quantity_micro > 0), consumed_micro INTEGER NOT NULL DEFAULT 0 CHECK(consumed_micro >= 0),
		released_micro INTEGER NOT NULL DEFAULT 0 CHECK(released_micro >= 0), created_at TEXT NOT NULL,
		CHECK(consumed_micro + released_micro <= quantity_micro)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_findit_reservation_active ON reservations(stock_id, quantity_micro, consumed_micro, released_micro)`,
	`CREATE INDEX IF NOT EXISTS idx_findit_reservation_project ON reservations(project_id, line_id)`,
	`CREATE TABLE IF NOT EXISTS stock_movements (
		id TEXT PRIMARY KEY, request_id TEXT NOT NULL, kind TEXT NOT NULL, item_id TEXT NOT NULL REFERENCES items(id) ON DELETE RESTRICT,
		stock_id TEXT NOT NULL REFERENCES stocks(id) ON DELETE RESTRICT, source_location_id TEXT REFERENCES locations(id) ON DELETE RESTRICT,
		target_location_id TEXT REFERENCES locations(id) ON DELETE RESTRICT, quantity_micro INTEGER NOT NULL,
		project_id TEXT REFERENCES projects(id) ON DELETE RESTRICT, reservation_id TEXT REFERENCES reservations(id) ON DELETE RESTRICT,
		serial TEXT NOT NULL DEFAULT '', lot TEXT NOT NULL DEFAULT '', condition TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_findit_movements_item ON stock_movements(item_id, created_at DESC)`,
	`CREATE TABLE IF NOT EXISTS command_replays (
		request_id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL, result_json TEXT NOT NULL, created_at TEXT NOT NULL
	)`,
}

func seedBuiltInTemplates(db *sql.DB) error {
	now := nowString()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range Catalog().Templates {
		fields, err := json.Marshal(item.Fields)
		if err != nil {
			return err
		}
		var existingFields, existingCategory string
		var builtIn int
		lookupErr := tx.QueryRow(`SELECT built_in,category,fields_json FROM templates WHERE id=?`, item.ID).
			Scan(&builtIn, &existingCategory, &existingFields)
		if lookupErr != nil && !errors.Is(lookupErr, sql.ErrNoRows) {
			return fmt.Errorf("read existing FindIt template %q: %w", item.ID, lookupErr)
		}
		if lookupErr == nil && builtIn != 0 {
			var used int
			if err := tx.QueryRow(`SELECT count(*) FROM items WHERE template_id=?`, item.ID).Scan(&used); err != nil {
				return fmt.Errorf("check existing FindIt template %q use: %w", item.ID, err)
			}
			if used > 0 {
				var previous []PropertyDefinition
				if err := json.Unmarshal([]byte(existingFields), &previous); err != nil {
					return fmt.Errorf("decode existing FindIt template %q fields: %w", item.ID, err)
				}
				if !compatibleTemplateChange(previous, item.Fields) {
					fields = []byte(existingFields)
				}
				if item.Category != existingCategory {
					item.Category = existingCategory
				}
			}
		}
		if _, err := tx.Exec(`INSERT INTO templates(id,version,name,description,category,built_in,fields_json,created_at,updated_at)
			VALUES(?,?,?,?,?,1,?,?,?) ON CONFLICT(id) DO UPDATE SET
			version=templates.version+1,name=excluded.name,description=excluded.description,category=excluded.category,fields_json=excluded.fields_json,updated_at=excluded.updated_at
			WHERE templates.built_in=1 AND (templates.name<>excluded.name OR templates.description<>excluded.description OR templates.category<>excluded.category OR templates.fields_json<>excluded.fields_json)`,
			item.ID, item.Version, item.Name, item.Description, item.Category, fields, now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nowString() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func commit(ctx context.Context, tx *sql.Tx) error {
	return fileutil.PublishContext(ctx, tx.Commit)
}
