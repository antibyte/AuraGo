package memory

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
)

// MemoryOrigin is a compact trust label. Source session identifiers stay private.
type MemoryOrigin struct {
	Actor  string `json:"actor,omitempty"`
	Origin string `json:"origin,omitempty"`
	Trust  string `json:"trust"`
}

func (s *SQLiteMemory) initRememberOrigins() error {
	var exists int
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='memory_record_origins'`).Scan(&exists); err != nil {
		return err
	}
	if exists != 0 {
		return nil
	}
	populated := 0
	for _, table := range []string{"core_memory", "notes", "journal_entries"} {
		var exists int
		if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		var count int
		if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil {
			return err
		}
		populated += count
	}

	if populated > 0 {
		if _, err := backupMainDatabase(s.db, "pre-remember-origins"); err != nil {
			return err
		}
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS memory_record_origins (
 store TEXT NOT NULL,record_id INTEGER NOT NULL,actor TEXT NOT NULL,origin TEXT NOT NULL,
 source_ref TEXT NOT NULL,content_hash TEXT NOT NULL,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(store,record_id));`)
	return err
}

func insertRememberOrigin(tx *sql.Tx, store string, id int64, content, sessionID string) error {
	_, err := tx.Exec(`INSERT INTO memory_record_origins(store,record_id,actor,origin,source_ref,content_hash) VALUES(?,?,'agent','model_remember',?,?) ON CONFLICT DO NOTHING`, store, id, fmt.Sprintf("%x", sha256.Sum256([]byte(sessionID))), fmt.Sprintf("%x", sha256.Sum256([]byte(content))))
	return err
}

// MemoryOriginForRecord validates provenance against the current record content.
// Reused or edited legacy/user records never inherit a model-written label.
func (s *SQLiteMemory) MemoryOriginForRecord(store string, id int64, content string) (MemoryOrigin, error) {
	origin := MemoryOrigin{Trust: "legacy-unknown"}
	var hash string
	err := s.db.QueryRow(`SELECT actor,origin,content_hash FROM memory_record_origins WHERE store=? AND record_id=?`, store, id).Scan(&origin.Actor, &origin.Origin, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return origin, nil
	}
	if err != nil {
		return MemoryOrigin{Trust: "unavailable"}, err
	}
	if hash != fmt.Sprintf("%x", sha256.Sum256([]byte(content))) {
		return MemoryOrigin{Trust: "legacy-unknown"}, nil
	}
	if origin.Actor == "agent" && origin.Origin == "model_remember" {
		origin.Trust = "agent-recorded"
	}
	return origin, nil
}

// AddModelCoreFact stores a new model record and provenance atomically. Existing
// normalized facts retain their original provenance and do not consume capacity.
func (s *SQLiteMemory) AddModelCoreFact(fact, sessionID string, maxEntries int, hardCap bool) (int64, error) {
	if err := ValidateCoreMemoryFact(fact); err != nil {
		return 0, err
	}
	if len(fact) > maxCoreMemoryFactLen {
		fact = truncateUTF8Bytes(fact, maxCoreMemoryFactLen)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	normalized := normalizeCoreMemoryFactForDedupe(fact)
	var id int64
	err = tx.QueryRow(`SELECT id FROM core_memory WHERE normalized_fact=?`, normalized).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if hardCap && maxEntries > 0 {
		var count int
		if err := tx.QueryRow(`SELECT count(*) FROM core_memory`).Scan(&count); err != nil {
			return 0, err
		}
		if count >= maxEntries {
			return 0, fmt.Errorf("core memory entry limit reached")
		}
	}
	result, err := tx.Exec(`INSERT INTO core_memory(fact,normalized_fact) VALUES(?,?)`, fact, normalized)
	if err != nil {
		return 0, err
	}
	id, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err = insertRememberOrigin(tx, "core", id, fact, sessionID); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *SQLiteMemory) AddModelNote(category, title, content string, priority int, dueDate, sessionID string) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	id, err := s.addNote(tx, category, title, content, priority, dueDate)
	if err != nil {
		return 0, err
	}
	// addNote applies the shared content length limit before persistence.
	var stored string
	if err := tx.QueryRow(`SELECT content FROM notes WHERE id=?`, id).Scan(&stored); err != nil {
		return 0, err
	}
	if err := insertRememberOrigin(tx, "notes", id, stored, sessionID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *SQLiteMemory) InsertModelJournalEntry(entry JournalEntry) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	id, err := s.insertJournalEntry(tx, entry)
	if err != nil {
		return 0, err
	}
	if err := insertRememberOrigin(tx, "journal", id, entry.Content, entry.SessionID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

type rememberSQLExecutor interface {
	Exec(string, ...any) (sql.Result, error)
}
