package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
// outcome of the last startup apply. GET reads the state, DELETE discards the
// pending restore, and POST {"action":"confirm","id":"<pending id>"} confirms
// the security changes of a pending restore so the next start applies it.
func handleBackupRestoreStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodDelete && r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		root, err := agoArchiveBaseDir(filepath.Dir(s.Cfg.ConfigPath))
		if err != nil {
			jsonError(w, "Cannot determine restore root", http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodPost {
			var req struct {
				Action string `json:"action"`
				ID     string `json:"id"`
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 4<<10+1))
			if err != nil || len(body) > 4<<10 || json.Unmarshal(body, &req) != nil || req.Action != "confirm" || req.ID == "" {
				jsonError(w, "Ungültige Anfrage: erwartet wird {\"action\":\"confirm\",\"id\":\"...\"}.", http.StatusBadRequest)
				return
			}
			s.CfgSaveMu.Lock()
			err = restorepending.Confirm(root, req.ID)
			s.CfgSaveMu.Unlock()
			switch {
			case errors.Is(err, restorepending.ErrNoPending):
				jsonError(w, "Es ist keine Wiederherstellung vorgemerkt.", http.StatusNotFound)
			case errors.Is(err, restorepending.ErrIDMismatch):
				jsonError(w, "Die vorgemerkte Wiederherstellung hat sich geändert; bitte Status neu laden.", http.StatusConflict)
			case err != nil:
				s.Logger.Error("[Backup] Failed to confirm pending restore", "error", err)
				jsonError(w, "Die Wiederherstellung konnte nicht bestätigt werden.", http.StatusInternalServerError)
			default:
				s.Logger.Info("[Backup] Pending restore confirmed", "id", req.ID)
				writeJSON(w, map[string]interface{}{"confirmed": true, "id": req.ID, "restart_required": true})
			}
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
			"pending":               st.Pending,
			"pending_created_at":    st.PendingCreatedAt,
			"id":                    st.PendingID,
			"awaiting_confirmation": st.AwaitingConfirmation,
			"restart_required":      st.Pending && !st.AwaitingConfirmation,
			"security_changes":      st.SecurityChanges,
			"last":                  nil,
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
