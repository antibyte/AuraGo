package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func handleSQLiteImport(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.SQLConnectionPool == nil {
			jsonError(w, "SQL connections unavailable", http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sql-connections/"), "/import")
		if id == "" || strings.Contains(id, "/") {
			jsonError(w, "invalid connection ID", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
		if err := s.SQLConnectionPool.ImportSQLite(ctx, id, r.Body); err != nil {
			jsonError(w, "SQLite import failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "imported"})
	}
}
