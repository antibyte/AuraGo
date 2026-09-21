package rtlsdr

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

func openStore(directory string) (*sql.DB, State, error) {
	s := State{Version: 1, Tuning: DefaultTuning(), Favorites: []Station{}, Stations: []Station{}, Recordings: []Recording{}, Schedules: []Schedule{}}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, s, err
	}
	db, err := sql.Open("sqlite", filepath.Join(directory, "receiver.db"))
	if err != nil {
		return nil, s, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS receiver_state (id INTEGER PRIMARY KEY CHECK(id=1), data BLOB NOT NULL)`); err != nil {
		db.Close()
		return nil, s, err
	}
	var data []byte
	err = db.QueryRow("SELECT data FROM receiver_state WHERE id=1").Scan(&data)
	if err != nil && err != sql.ErrNoRows {
		db.Close()
		return nil, s, err
	}
	if len(data) > 0 {
		if err = json.Unmarshal(data, &s); err != nil || s.Version != 1 {
			db.Close()
			return nil, s, fmt.Errorf("unsupported receiver state")
		}
	}
	return db, s, nil
}

func (s *Service) saveLocked() error {
	data, err := json.Marshal(s.state)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO receiver_state(id,data) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data", data)
	return err
}

func copyState(state State) State {
	data, _ := json.Marshal(state)
	var result State
	_ = json.Unmarshal(data, &result)
	return result
}
