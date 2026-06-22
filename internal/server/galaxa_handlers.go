package server

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/dbutil"

	_ "modernc.org/sqlite"
)

type galaxaHighscore struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
	Stage int    `json:"stage"`
	Date  string `json:"date"`
}

var (
	galaxaDBOnce sync.Once
	galaxaDBInst *sql.DB
	galaxaDBErr  error
	galaxaDBMu   sync.Mutex
)

func getGalaxaDB(dataDir string) (*sql.DB, error) {
	galaxaDBMu.Lock()
	defer galaxaDBMu.Unlock()
	galaxaDBOnce.Do(func() {
		dbPath := filepath.Join(dataDir, "galaxa.db")
		galaxaDBInst, galaxaDBErr = dbutil.Open(dbPath)
		if galaxaDBErr != nil {
			return
		}
		galaxaDBErr = galaxaDBInst.Ping()
		if galaxaDBErr != nil {
			galaxaDBInst.Close()
			galaxaDBInst = nil
			return
		}
		_, galaxaDBErr = galaxaDBInst.Exec(`CREATE TABLE IF NOT EXISTS galaxa_highscores (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			score INTEGER NOT NULL,
			stage INTEGER NOT NULL DEFAULT 1,
			date TEXT NOT NULL
		)`)
		if galaxaDBErr != nil {
			galaxaDBInst.Close()
			galaxaDBInst = nil
		}
	})
	return galaxaDBInst, galaxaDBErr
}

func closeGalaxaDB(logger *slog.Logger) {
	galaxaDBMu.Lock()
	defer galaxaDBMu.Unlock()
	if galaxaDBInst == nil {
		return
	}
	if err := galaxaDBInst.Close(); err != nil && logger != nil {
		logger.Warn("Failed to close galaxa database", "error", err)
	}
	galaxaDBInst = nil
	galaxaDBErr = nil
	galaxaDBOnce = sync.Once{}
}

func handleGalaxaHighscoreGet(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		db, err := getGalaxaDB(dataDir)
		if err != nil || db == nil {
			json.NewEncoder(w).Encode([]galaxaHighscore{})
			return
		}

		rows, err := db.Query(`SELECT name, score, stage, date FROM galaxa_highscores ORDER BY score DESC LIMIT 10`)
		if err != nil {
			json.NewEncoder(w).Encode([]galaxaHighscore{})
			return
		}
		defer rows.Close()

		var scores []galaxaHighscore
		for rows.Next() {
			var hs galaxaHighscore
			if err := rows.Scan(&hs.Name, &hs.Score, &hs.Stage, &hs.Date); err != nil {
				continue
			}
			scores = append(scores, hs)
		}
		if scores == nil {
			scores = []galaxaHighscore{}
		}
		json.NewEncoder(w).Encode(scores)
	}
}

func handleGalaxaHighscorePost(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Name  string `json:"name"`
			Score int    `json:"score"`
			Stage int    `json:"stage"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || len(req.Name) > 3 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "name must be 1-3 characters"})
			return
		}
		if req.Score <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "score must be positive"})
			return
		}
		if req.Stage <= 0 {
			req.Stage = 1
		}

		s.CfgMu.RLock()
		dataDir := s.Cfg.Directories.DataDir
		s.CfgMu.RUnlock()

		db, err := getGalaxaDB(dataDir)
		if err != nil || db == nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "database not available"})
			return
		}

		_, err = db.Exec(`INSERT INTO galaxa_highscores (name, score, stage, date) VALUES (?, ?, ?, ?)`,
			strings.ToUpper(req.Name), req.Score, req.Stage, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to save"})
			return
		}

		rows, err := db.Query(`SELECT name, score, stage, date FROM galaxa_highscores ORDER BY score DESC LIMIT 10`)
		if err != nil {
			json.NewEncoder(w).Encode([]galaxaHighscore{})
			return
		}
		defer rows.Close()

		var scores []galaxaHighscore
		for rows.Next() {
			var hs galaxaHighscore
			if err := rows.Scan(&hs.Name, &hs.Score, &hs.Stage, &hs.Date); err != nil {
				continue
			}
			scores = append(scores, hs)
		}
		if scores == nil {
			scores = []galaxaHighscore{}
		}
		json.NewEncoder(w).Encode(scores)
	}
}
