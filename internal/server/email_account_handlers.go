package server

import (
	"aurago/internal/config"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// emailAccountJSON is the API representation of an email account.
type emailAccountJSON struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IMAPHost      string `json:"imap_host"`
	IMAPPort      int    `json:"imap_port"`
	SMTPHost      string `json:"smtp_host"`
	SMTPPort      int    `json:"smtp_port"`
	Username      string `json:"username"`
	Password      string `json:"password,omitempty"`
	FromAddress   string `json:"from_address"`
	Enabled       bool   `json:"enabled"`
	AllowSending  bool   `json:"allow_sending"`
	WatchEnabled  bool   `json:"watch_enabled"`
	WatchInterval int    `json:"watch_interval_seconds"`
	WatchFolder   string `json:"watch_folder"`
}

// handleEmailAccounts dispatches GET / PUT for /api/email-accounts.
func handleEmailAccounts(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetEmailAccounts(s, w, r)
		case http.MethodPut:
			handlePutEmailAccounts(s, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleGetEmailAccounts returns the email accounts list with passwords masked.
func handleGetEmailAccounts(s *Server, w http.ResponseWriter, _ *http.Request) {
	s.CfgMu.RLock()
	accounts := s.Cfg.EmailAccounts
	s.CfgMu.RUnlock()

	out := make([]emailAccountJSON, len(accounts))
	for i, a := range accounts {
		pw := a.Password
		if pw != "" {
			pw = maskedKey
		}
		out[i] = emailAccountJSON{
			ID:            a.ID,
			Name:          a.Name,
			IMAPHost:      a.IMAPHost,
			IMAPPort:      a.IMAPPort,
			SMTPHost:      a.SMTPHost,
			SMTPPort:      a.SMTPPort,
			Username:      a.Username,
			Password:      pw,
			FromAddress:   a.FromAddress,
			Enabled:       !a.Disabled,
			AllowSending:  !a.ReadOnly,
			WatchEnabled:  a.WatchEnabled,
			WatchInterval: a.WatchInterval,
			WatchFolder:   a.WatchFolder,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handlePutEmailAccounts saves a new email accounts array to config.yaml and hot-reloads.
func handlePutEmailAccounts(s *Server, w http.ResponseWriter, r *http.Request) {
	var incoming []emailAccountJSON
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Build id → old password map so masked values are preserved
	s.CfgMu.RLock()
	oldPwMap := make(map[string]string, len(s.Cfg.EmailAccounts))
	for _, a := range s.Cfg.EmailAccounts {
		oldPwMap[a.ID] = a.Password
	}
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()

	if configPath == "" {
		http.Error(w, "Config path not set", http.StatusInternalServerError)
		return
	}

	// Convert to EmailAccount slice; write passwords to vault, not YAML
	entries := make([]config.EmailAccount, len(incoming))
	for i, a := range incoming {
		a.ID = strings.TrimSpace(a.ID)
		if a.ID == "" {
			http.Error(w, "Account ID must not be empty", http.StatusBadRequest)
			return
		}

		// ── Password → vault ──
		pw := a.Password
		if pw == maskedKey {
			if old, ok := oldPwMap[a.ID]; ok {
				pw = old
			}
		}
		if pw != "" && pw != maskedKey && s.Vault != nil {
			if err := s.Vault.WriteSecret("email_"+a.ID+"_password", pw); err != nil {
				s.Logger.Error("[EmailAccounts] Failed to write password to vault", "id", a.ID, "error", err)
			}
		}

		entries[i] = config.EmailAccount{
			ID:            a.ID,
			Name:          a.Name,
			IMAPHost:      a.IMAPHost,
			IMAPPort:      a.IMAPPort,
			SMTPHost:      a.SMTPHost,
			SMTPPort:      a.SMTPPort,
			Username:      a.Username,
			Password:      pw,
			FromAddress:   a.FromAddress,
			Disabled:      !a.Enabled,
			ReadOnly:      !a.AllowSending,
			WatchEnabled:  a.WatchEnabled,
			WatchInterval: a.WatchInterval,
			WatchFolder:   a.WatchFolder,
		}
	}

	// Read raw YAML, update email_accounts key, write back
	data, err := os.ReadFile(configPath)
	if err != nil {
		s.Logger.Error("Failed to read config for email-accounts update", "error", err)
		http.Error(w, "Failed to read config", http.StatusInternalServerError)
		return
	}

	var rawCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &rawCfg); err != nil {
		s.Logger.Error("Failed to parse config for email-accounts update", "error", err)
		http.Error(w, "Failed to parse config", http.StatusInternalServerError)
		return
	}

	// Build email_accounts as []interface{} for YAML marshal (passwords excluded)
	acctList := make([]interface{}, len(entries))
	for i, e := range entries {
		m := map[string]interface{}{
			"id":                     e.ID,
			"name":                   e.Name,
			"imap_host":              e.IMAPHost,
			"imap_port":              e.IMAPPort,
			"smtp_host":              e.SMTPHost,
			"smtp_port":              e.SMTPPort,
			"username":               e.Username,
			"from_address":           e.FromAddress,
			"disabled":               e.Disabled,
			"readonly":               e.ReadOnly,
			"watch_enabled":          e.WatchEnabled,
			"watch_interval_seconds": e.WatchInterval,
			"watch_folder":           e.WatchFolder,
		}
		// Password is NOT written to YAML — it lives in the vault.
		acctList[i] = m
	}
	rawCfg["email_accounts"] = acctList

	out, err := yaml.Marshal(rawCfg)
	if err != nil {
		s.Logger.Error("Failed to marshal config after email-accounts update", "error", err)
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	if err := config.WriteFileAtomic(configPath, out, 0o600); err != nil {
		s.Logger.Error("Failed to write config after email-accounts update", "error", err)
		http.Error(w, "Failed to write config", http.StatusInternalServerError)
		return
	}

	// Hot-reload
	s.CfgMu.Lock()
	newCfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		s.CfgMu.Unlock()
		s.Logger.Error("[EmailAccounts] Hot-reload failed", "error", loadErr)
		http.Error(w, "Saved but reload failed", http.StatusInternalServerError)
		return
	}
	savedPath := s.Cfg.ConfigPath
	*s.Cfg = *newCfg
	s.Cfg.ConfigPath = savedPath
	// Apply vault secrets and re-resolve after hot-reload
	s.Cfg.ApplyVaultSecrets(s.Vault)
	s.Cfg.ResolveProviders()
	s.Cfg.ApplyOAuthTokens(s.Vault)
	s.CfgMu.Unlock()

	s.Logger.Info("[EmailAccounts] Updated", "count", len(entries))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(entries),
	})
}
