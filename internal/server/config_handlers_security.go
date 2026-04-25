package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// handleVaultStatus returns whether the vault is available.
// The vault is available when s.Vault != nil (i.e. AURAGO_MASTER_KEY was
// provided at startup). The vault.bin file is created lazily on the first
// write, so checking for file existence would yield a false negative on a
// fresh installation where no secrets have been stored yet.
func handleVaultStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"exists": s.Vault != nil})
	}
}

// handleVaultDelete deletes vault.bin (and its lockfile).
func handleVaultDelete(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vaultPath := filepath.Join(s.Cfg.Directories.DataDir, "vault.bin")
		lockPath := vaultPath + ".lock"

		// Delete vault files
		if err := os.Remove(vaultPath); err != nil && !os.IsNotExist(err) {
			s.Logger.Error("[Vault] Failed to delete vault file", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"message": "Vault-Datei konnte nicht gelöscht werden."})
			return
		}
		os.Remove(lockPath) // best-effort

		// Update in-memory config
		s.CfgMu.Lock()
		s.Cfg.Server.MasterKey = ""
		s.Cfg.Auth.PasswordHash = ""
		s.Cfg.Auth.SessionSecret = ""
		s.Cfg.Auth.TOTPSecret = ""
		s.CfgMu.Unlock()
		s.Vault = nil

		s.Logger.Info("[Vault] Vault deleted via Web UI")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Vault gelöscht."})
	}
}

// handleSecurityHints returns the current list of security hints for the running config.
// Requires an active session (auth-gated via server_routes.go WebConfig.Enabled block).
func handleSecurityHints(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.CfgMu.RLock()
		hints := CheckSecurity(s.Cfg)
		internetFacing := isInternetFacing(s.Cfg)
		networkFacing := isNetworkFacing(s.Cfg)
		s.CfgMu.RUnlock()

		// Build a serialisable view (strip FixPatch — applied server-side only)
		type hintView struct {
			ID          string `json:"id"`
			Severity    string `json:"severity"`
			Title       string `json:"title"`
			Description string `json:"description"`
			AutoFixable bool   `json:"auto_fixable"`
		}
		views := make([]hintView, len(hints))
		for i, h := range hints {
			views[i] = hintView{
				ID:          h.ID,
				Severity:    h.Severity,
				Title:       h.Title,
				Description: h.Description,
				AutoFixable: h.AutoFixable,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"hints":           views,
			"internet_facing": internetFacing,
			"network_facing":  networkFacing,
		})
	}
}

// handleSecurityHarden applies auto-fixable hardening patches selected by the user.
// Expects JSON body: {"ids": ["auth_disabled", "n8n_no_token", ...]}.
func handleSecurityHarden(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		applied, err := ApplyHardening(s, req.IDs)
		if err != nil {
			s.Logger.Error("[Security] Hardening failed", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Failed to apply hardening"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"applied": applied,
			"message": fmt.Sprintf("%d hardening measure(s) applied.", len(applied)),
		})
	}
}
