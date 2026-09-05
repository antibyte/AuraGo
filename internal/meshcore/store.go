package meshcore

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"time"
)

type store struct {
	db   *sql.DB
	salt []byte
}

func openStore(dir string) (*store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "meshcore.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &store{db: db}
	ok := false
	defer func() {
		if !ok {
			db.Close()
		}
	}()
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
CREATE TABLE IF NOT EXISTS meshcore_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS meshcore_messages (id TEXT PRIMARY KEY, received INTEGER NOT NULL, state TEXT NOT NULL, data BLOB NOT NULL, binding TEXT NOT NULL DEFAULT '', agent_notified INTEGER NOT NULL DEFAULT 0);
CREATE INDEX IF NOT EXISTS meshcore_received ON meshcore_messages(received);
CREATE TABLE IF NOT EXISTS meshcore_executions (id TEXT PRIMARY KEY, expires INTEGER NOT NULL);
INSERT OR IGNORE INTO meshcore_meta(key,value) VALUES('version','1');`)
	if err != nil {
		return nil, fmt.Errorf("initialize meshcore store: %w", err)
	}
	var version string
	if err = db.QueryRow("SELECT value FROM meshcore_meta WHERE key='version'").Scan(&version); err != nil || version != "1" {
		return nil, fmt.Errorf("unsupported meshcore schema")
	}
	var salt string
	err = db.QueryRow("SELECT value FROM meshcore_meta WHERE key='binding_salt'").Scan(&salt)
	if err == sql.ErrNoRows {
		b := make([]byte, 32)
		if _, err = rand.Read(b); err != nil {
			return nil, err
		}
		salt = hex.EncodeToString(b)
		_, err = db.Exec("INSERT INTO meshcore_meta(key,value) VALUES('binding_salt',?)", salt)
	}
	if err != nil {
		return nil, err
	}
	s.salt, err = hex.DecodeString(salt)
	if err != nil || len(s.salt) != 32 {
		return nil, fmt.Errorf("invalid binding salt")
	}
	_, err = db.Exec("UPDATE meshcore_messages SET state='outcome_unknown' WHERE state IN ('processing','sending'); UPDATE meshcore_messages SET state='received' WHERE state='pending'")
	if err != nil {
		return nil, err
	}
	ok = true
	return s, nil
}
func (s *store) insert(m Message) (bool, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return false, err
	}
	r, err := s.db.Exec("INSERT OR IGNORE INTO meshcore_messages(id,received,state,data,binding) VALUES(?,?,?,?,?)", m.ID, m.ReceivedAt, m.State, b, m.Binding)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}
func (s *store) save(m Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("UPDATE meshcore_messages SET state=?,data=? WHERE id=?", m.State, b, m.ID)
	return err
}
func (s *store) claim(id string) (bool, error) {
	r, err := s.db.Exec("UPDATE meshcore_messages SET state='processing' WHERE id=? AND state='pending'", id)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}
func (s *store) list(limit, offset int) ([]Message, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.Query("SELECT data,state,binding FROM meshcore_messages ORDER BY received DESC,id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Message{}
	for rows.Next() {
		var b []byte
		var state, binding string
		if err := rows.Scan(&b, &state, &binding); err != nil {
			return nil, err
		}
		var m Message
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		m.State = state
		m.Binding = binding
		if state == "outcome_unknown" {
			m.SendState = "outcome_unknown"
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (s *store) get(id string) (Message, error) {
	var b []byte
	var m Message
	var state, binding string
	err := s.db.QueryRow("SELECT data,state,binding FROM meshcore_messages WHERE id=?", id).Scan(&b, &state, &binding)
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(b, &m)
	m.State = state
	m.Binding = binding
	return m, err
}
func (s *store) prune(c Config) error {
	_, err := s.db.Exec("DELETE FROM meshcore_messages WHERE state NOT IN ('pending','processing','sending') AND received<?", time.Now().Add(-time.Duration(c.RetentionDays)*24*time.Hour).Unix())
	if err != nil {
		return err
	}
	_, err = s.db.Exec("DELETE FROM meshcore_messages WHERE id IN (SELECT id FROM meshcore_messages WHERE state NOT IN ('pending','processing','sending') ORDER BY received,id LIMIT MAX(0,(SELECT COUNT(*) FROM meshcore_messages)-?))", c.MaxMessages)
	return err
}

// Keep execution tombstones beyond the maximum configurable command age, even
// if inbox retention evicts the body. A full ledger blocks new automatic work.
func (s *store) reserveExecution(id string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM meshcore_executions WHERE expires<?", time.Now().Unix()); err != nil {
		return false, err
	}
	result, err := tx.Exec("INSERT OR IGNORE INTO meshcore_executions(id,expires) SELECT ?,? WHERE (SELECT COUNT(*) FROM meshcore_executions)<65536", id, time.Now().Add(48*time.Hour).Unix())
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}
