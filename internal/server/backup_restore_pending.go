package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aurago/internal/restorepending"
)

// agoSQLiteUserVersion reads PRAGMA user_version from a SQLite file without
// modifying it.
func agoSQLiteUserVersion(path string) (int, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

// agoCheckBackupDBVersions compares the schema version of every staged SQLite
// database with its live counterpart. A backup that is newer than the running
// binary is rejected (second return value); older backups produce warnings
// because migrations run at startup.
func agoCheckBackupDBVersions(entries []agoImportEntry) (older []string, newer []string) {
	for _, entry := range entries {
		if entry.isDir || !agoIsSQLiteDBPath(entry.clean) {
			continue
		}
		info, err := os.Lstat(entry.dest)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		backupV, err := agoSQLiteUserVersion(entry.stage)
		if err != nil {
			continue
		}
		liveV, err := agoSQLiteUserVersion(entry.dest)
		if err != nil {
			continue
		}
		label := fmt.Sprintf("%s (Backup v%d, aktuell v%d)", filepath.ToSlash(entry.clean), backupV, liveV)
		switch {
		case backupV > liveV:
			newer = append(newer, label)
		case backupV < liveV:
			older = append(older, label)
		}
	}
	sort.Strings(older)
	sort.Strings(newer)
	return older, newer
}

func agoJoinLabels(labels []string) string { return strings.Join(labels, ", ") }

// handleBackupRestoreStatus reports a pending (not yet applied) restore and the
// outcome of the last startup apply.
func handleBackupRestoreStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		root, err := agoArchiveBaseDir(filepath.Dir(s.Cfg.ConfigPath))
		if err != nil {
			jsonError(w, "Cannot determine restore root", http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodDelete {
			// Serialize with imports so a half-written pending tree is never removed.
			s.CfgSaveMu.Lock()
			discarded, err := restorepending.Discard(root)
			s.CfgSaveMu.Unlock()
			if err != nil {
				s.Logger.Error("[Backup] Failed to discard pending restore", "error", err)
				jsonError(w, "Die vorgemerkte Wiederherstellung konnte nicht verworfen werden.", http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]bool{"discarded": discarded})
			return
		}
		st := restorepending.ReadState(root)
		resp := map[string]interface{}{
			"pending":            st.Pending,
			"pending_created_at": st.PendingCreatedAt,
			"last":               nil,
		}
		if st.Last != nil {
			resp["last"] = map[string]string{
				"id":         st.Last.ID,
				"applied_at": st.Last.AppliedAt,
				"status":     st.Last.Status,
				"message":    st.Last.Message,
			}
		}
		writeJSON(w, resp)
	}
}
