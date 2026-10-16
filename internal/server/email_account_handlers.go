package server

import (
	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// emailAccountJSON is the API representation of an email account.
type emailAccountJSON struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	IMAPHost        string  `json:"imap_host"`
	IMAPPort        int     `json:"imap_port"`
	ReceiveProtocol *string `json:"receive_protocol,omitempty"`
	ReceiveTLS      *string `json:"receive_tls,omitempty"`
	POP3Host        *string `json:"pop3_host,omitempty"`
	POP3Port        *int    `json:"pop3_port,omitempty"`
	SMTPHost        string  `json:"smtp_host"`
	SMTPPort        int     `json:"smtp_port"`
	SMTPTransport   *string `json:"smtp_tls,omitempty"`
	Username        string  `json:"username"`
	Password        string  `json:"password,omitempty"`
	SMTPUsername    *string `json:"smtp_username,omitempty"`
	SMTPPassword    *string `json:"smtp_password,omitempty"`
	SMTPAuthMethod  *string `json:"smtp_auth_method,omitempty"`
	SMTPAuthSource  *string `json:"smtp_auth_source,omitempty"`
	FromAddress     string  `json:"from_address"`
	OAuthSource     *string `json:"oauth_source,omitempty"`
	OAuthAccountID  *string `json:"oauth_account_id,omitempty"`
	OAuthProvider   *string `json:"oauth_provider,omitempty"`
	OAuthClientID   *string `json:"oauth_client_id,omitempty"`
	AgentAccess     *bool   `json:"agent_access,omitempty"`
	Enabled         bool    `json:"enabled"`
	AllowSending    bool    `json:"allow_sending"`
	WatchEnabled    bool    `json:"watch_enabled"`
	WatchInterval   int     `json:"watch_interval_seconds"`
	WatchFolder     string  `json:"watch_folder"`
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleEmailAccountsTest probes IMAP and/or SMTP credentials from the config UI.
func handleEmailAccountsTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req emailAccountJSON
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		settings := resolveEmailConnectionTest(s, req)
		if (settings.imapHost != "" && (settings.imapUsername == "" || settings.imapPassword == "")) ||
			(settings.smtpHost != "" && (settings.smtpUsername == "" || settings.smtpPassword == "")) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Username and password are required",
			})
			return
		}

		imapHost := settings.imapHost
		smtpHost := settings.smtpHost
		if imapHost == "" && smtpHost == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "At least one of IMAP or SMTP host is required",
			})
			return
		}

		result := map[string]interface{}{
			"status": "ok",
		}
		var errors []string

		if imapHost != "" {
			if err := tools.TestIMAPConnectionWithTLS(r.Context(), imapHost, settings.imapPort, settings.imapUsername, settings.imapPassword, settings.receiveTLS); err != nil {
				result["imap"] = "error"
				errors = append(errors, "IMAP: "+err.Error())
			} else {
				result["imap"] = "ok"
			}
		} else {
			result["imap"] = "skipped"
		}

		if smtpHost != "" {
			if err := tools.TestSMTPAuthWithTransport(r.Context(), smtpHost, settings.smtpPort, settings.smtpUsername, settings.smtpPassword, settings.smtpTransport); err != nil {
				result["smtp"] = "error"
				errors = append(errors, "SMTP: "+err.Error())
			} else {
				result["smtp"] = "ok"
			}
		} else {
			result["smtp"] = "skipped"
		}

		imapStatus, _ := result["imap"].(string)
		smtpStatus, _ := result["smtp"].(string)
		switch {
		case len(errors) == 0:
			result["status"] = "ok"
			result["message"] = "Email connection successful"
		case (imapStatus == "ok" && smtpStatus == "error") || (smtpStatus == "ok" && imapStatus == "error"):
			result["status"] = "partial"
			result["message"] = strings.Join(errors, "; ")
		default:
			result["status"] = "error"
			result["message"] = strings.Join(errors, "; ")
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		if imapStatus == "ok" && smtpStatus == "ok" {
			result["message"] = fmt.Sprintf("IMAP and SMTP authentication successful")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

type emailConnectionTestSettings struct {
	imapHost, imapUsername, imapPassword, receiveTLS    string
	imapPort                                            int
	smtpHost, smtpUsername, smtpPassword, smtpTransport string
	smtpPort                                            int
}

// resolveEmailConnectionTest inherits saved settings for an account ID only
// while credentials remain bound to the saved protocol endpoint and username.
func resolveEmailConnectionTest(s *Server, req emailAccountJSON) emailConnectionTestSettings {
	var saved config.EmailAccount
	hasSaved := false
	if id := strings.TrimSpace(req.ID); id != "" {
		s.CfgMu.RLock()
		for i := range s.Cfg.EmailAccounts {
			if s.Cfg.EmailAccounts[i].ID == id {
				saved = s.Cfg.EmailAccounts[i]
				hasSaved = true
				break
			}
		}
		s.CfgMu.RUnlock()
	}
	value := func(provided string, fallback string) string {
		if strings.TrimSpace(provided) != "" {
			return strings.TrimSpace(provided)
		}
		return strings.TrimSpace(fallback)
	}
	settings := emailConnectionTestSettings{
		imapHost:     value(req.IMAPHost, saved.IMAPHost),
		imapUsername: value(req.Username, saved.Username),
		receiveTLS:   emailMode(req.ReceiveTLS, saved.ReceiveTLS),
		smtpHost:     value(req.SMTPHost, saved.SMTPHost),
	}
	settings.imapPort = requestOrSavedPort(req.IMAPPort, saved.IMAPPort)
	if settings.receiveTLS == "" {
		settings.receiveTLS = "tls"
	}
	if settings.imapPort <= 0 {
		settings.imapPort = 993
		if strings.EqualFold(settings.receiveTLS, "starttls") {
			settings.imapPort = 143
		}
	}
	settings.smtpPort = requestOrSavedPort(req.SMTPPort, saved.SMTPPort)
	settings.smtpTransport = emailMode(req.SMTPTransport, saved.SMTPTransport)
	if settings.smtpTransport == "" {
		settings.smtpTransport = "starttls"
		if settings.smtpPort == 465 {
			settings.smtpTransport = "tls"
		}
	}
	if settings.smtpPort <= 0 {
		settings.smtpPort = 587
		if strings.EqualFold(settings.smtpTransport, "tls") {
			settings.smtpPort = 465
		}
	}
	settings.smtpUsername = settings.imapUsername
	if strings.TrimSpace(saved.SMTPUsername) != "" {
		settings.smtpUsername = strings.TrimSpace(saved.SMTPUsername)
	}
	if req.SMTPUsername != nil {
		settings.smtpUsername = strings.TrimSpace(*req.SMTPUsername)
		if settings.smtpUsername == "" {
			settings.smtpUsername = settings.imapUsername
		}
	} else if strings.TrimSpace(req.Username) != "" {
		// A user explicitly editing the shared username also edits SMTP's
		// inherited username when no separate SMTP username was supplied.
		if !hasSaved || strings.TrimSpace(req.Username) != strings.TrimSpace(saved.Username) {
			settings.smtpUsername = settings.imapUsername
		}
	}
	requestPassword := strings.TrimSpace(req.Password)
	if requestPassword == maskedKey {
		requestPassword = ""
	}
	settings.imapPassword = requestPassword
	if settings.imapPassword == "" && hasSaved && sameEmailTestTarget(settings.imapHost, settings.imapPort, settings.imapUsername,
		saved.IMAPHost, emailEffectivePort(saved.IMAPPort, saved.ReceiveTLS, true), saved.Username) {
		settings.imapPassword = saved.Password
		if settings.imapPassword == "" {
			settings.imapPassword = readEmailAccountSecret(s, saved.ID, "password")
		}
	}
	if req.SMTPPassword != nil {
		settings.smtpPassword = strings.TrimSpace(*req.SMTPPassword)
		if settings.smtpPassword == maskedKey {
			settings.smtpPassword = ""
		}
	}
	if settings.smtpPassword == "" && requestPassword != "" {
		settings.smtpPassword = requestPassword
	}
	if settings.smtpPassword == "" && hasSaved && sameEmailTestTarget(settings.smtpHost, settings.smtpPort, settings.smtpUsername,
		saved.SMTPHost, emailEffectivePort(saved.SMTPPort, saved.SMTPTransport, false), savedEffectiveSMTPUsername(saved)) {
		settings.smtpPassword = saved.SMTPPassword
		if settings.smtpPassword == "" {
			settings.smtpPassword = readEmailAccountSecret(s, saved.ID, "smtp_password")
		}
		if settings.smtpPassword == "" && strings.EqualFold(settings.smtpUsername, saved.Username) {
			settings.smtpPassword = saved.Password
			if settings.smtpPassword == "" {
				settings.smtpPassword = readEmailAccountSecret(s, saved.ID, "password")
			}
		}
	}
	security.RegisterSensitive(settings.imapPassword)
	security.RegisterSensitive(settings.smtpPassword)
	return settings
}

func sameEmailTestTarget(host string, port int, username, savedHost string, savedPort int, savedUsername string) bool {
	return strings.EqualFold(strings.TrimSpace(host), strings.TrimSpace(savedHost)) && port == savedPort && strings.TrimSpace(username) == strings.TrimSpace(savedUsername)
}

func emailEffectivePort(port int, mode string, imap bool) int {
	if port > 0 {
		return port
	}
	if imap {
		if strings.EqualFold(strings.TrimSpace(mode), "starttls") {
			return 143
		}
		return 993
	}
	if strings.EqualFold(strings.TrimSpace(mode), "tls") {
		return 465
	}
	return 587
}

func requestOrSavedPort(requested, saved int) int {
	if requested > 0 {
		return requested
	}
	return saved
}

func savedEffectiveSMTPUsername(a config.EmailAccount) string {
	if strings.TrimSpace(a.SMTPUsername) != "" {
		return a.SMTPUsername
	}
	return a.Username
}
func readEmailAccountSecret(s *Server, accountID, suffix string) string {
	if s.Vault == nil || strings.TrimSpace(accountID) == "" {
		return ""
	}
	secret, _ := s.Vault.ReadSecret("email_" + accountID + "_" + suffix)
	return secret
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
		smtpPassword := a.SMTPPassword
		if smtpPassword != "" {
			smtpPassword = maskedKey
		}
		out[i] = emailAccountJSON{
			ID:              a.ID,
			Name:            a.Name,
			IMAPHost:        a.IMAPHost,
			IMAPPort:        a.IMAPPort,
			ReceiveProtocol: emailStringPointer(a.ReceiveProtocol),
			ReceiveTLS:      emailStringPointer(a.ReceiveTLS),
			POP3Host:        emailStringPointer(a.POP3Host),
			POP3Port:        emailIntPointer(a.POP3Port),
			SMTPHost:        a.SMTPHost,
			SMTPPort:        a.SMTPPort,
			SMTPTransport:   emailStringPointer(a.SMTPTransport),
			Username:        a.Username,
			Password:        pw,
			SMTPUsername:    emailStringPointer(a.SMTPUsername),
			SMTPPassword:    emailStringPointer(smtpPassword),
			SMTPAuthMethod:  emailStringPointer(normalizeEmailSMTPAuthMethod(a.SMTPAuthMethod)),
			SMTPAuthSource:  emailStringPointer(a.SMTPAuthSource),
			FromAddress:     a.FromAddress,
			OAuthSource:     emailStringPointer(a.OAuthSource),
			OAuthAccountID:  emailStringPointer(a.OAuthAccountID),
			OAuthProvider:   emailStringPointer(a.OAuthProvider),
			OAuthClientID:   emailStringPointer(a.OAuthClientID),
			AgentAccess:     a.AgentAccess,
			Enabled:         !a.Disabled,
			AllowSending:    !a.ReadOnly,
			WatchEnabled:    a.WatchEnabled,
			WatchInterval:   a.WatchInterval,
			WatchFolder:     a.WatchFolder,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// handlePutEmailAccounts saves a new email accounts array to config.yaml and hot-reloads.
func handlePutEmailAccounts(s *Server, w http.ResponseWriter, r *http.Request) {
	var incoming []emailAccountJSON
	if err := decodeMailerJSON(w, r, &incoming, 128<<10); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if len(incoming) > 100 {
		jsonError(w, "Too many email accounts", http.StatusBadRequest)
		return
	}
	secretChanges := make([]mailerVaultChange, 0, len(incoming)*2)
	for _, item := range incoming {
		id := strings.TrimSpace(item.ID)
		if password := item.Password; password != "" && password != maskedKey {
			secretChanges = append(secretChanges, mailerVaultChange{key: "email_" + id + "_password", value: password})
		}
		if item.SMTPPassword != nil {
			password := *item.SMTPPassword
			if password != "" && password != maskedKey {
				secretChanges = append(secretChanges, mailerVaultChange{key: "email_" + id + "_smtp_password", value: password})
			}
		}
	}
	err := s.persistConfiguredEmailAccountsWithSecrets(r.Context(), func(cfg *config.Config) error {
		old := make(map[string]config.EmailAccount, len(cfg.EmailAccounts))
		for _, account := range cfg.EmailAccounts {
			old[account.ID] = account
		}
		entries := make([]config.EmailAccount, 0, len(incoming))
		seen := make(map[string]struct{}, len(incoming))
		for _, item := range incoming {
			item.ID = strings.TrimSpace(item.ID)
			if !config.ValidGogAccountID(item.ID) {
				return fmt.Errorf("invalid email account ID")
			}
			if _, exists := seen[item.ID]; exists {
				return fmt.Errorf("duplicate email account ID")
			}
			seen[item.ID] = struct{}{}
			entry := old[item.ID]
			entry.ID, entry.Name = item.ID, item.Name
			entry.IMAPHost, entry.IMAPPort = item.IMAPHost, item.IMAPPort
			entry.SMTPHost, entry.SMTPPort = item.SMTPHost, item.SMTPPort
			entry.Username, entry.FromAddress = item.Username, item.FromAddress
			entry.Disabled, entry.ReadOnly = !item.Enabled, !item.AllowSending
			entry.WatchEnabled, entry.WatchInterval, entry.WatchFolder = item.WatchEnabled, item.WatchInterval, item.WatchFolder

			pw := item.Password
			setPassword := pw != "" && pw != maskedKey
			if !setPassword {
				pw = entry.Password
				if pw == "" && s.Vault != nil {
					pw, _ = s.Vault.ReadSecret("email_" + item.ID + "_password")
				}
			}
			smtpPassword := ""
			if item.SMTPPassword != nil {
				smtpPassword = *item.SMTPPassword
			}
			setSMTPPassword := smtpPassword != "" && smtpPassword != maskedKey
			if !setSMTPPassword {
				smtpPassword = entry.SMTPPassword
				if smtpPassword == "" && s.Vault != nil {
					smtpPassword, _ = s.Vault.ReadSecret("email_" + item.ID + "_smtp_password")
				}
			}
			for _, secret := range []string{pw, smtpPassword} {
				if len(secret) > 4096 || strings.ContainsAny(secret, "\r\n\x00") {
					return fmt.Errorf("invalid email credential")
				}
				security.RegisterSensitive(secret)
			}
			entry.Password, entry.SMTPPassword = pw, smtpPassword
			applyEmailString(&entry.ReceiveProtocol, item.ReceiveProtocol)
			applyEmailString(&entry.ReceiveTLS, item.ReceiveTLS)
			applyEmailString(&entry.POP3Host, item.POP3Host)
			applyEmailInt(&entry.POP3Port, item.POP3Port)
			applyEmailString(&entry.SMTPTransport, item.SMTPTransport)
			applyEmailString(&entry.SMTPUsername, item.SMTPUsername)
			applyEmailString(&entry.SMTPAuthMethod, item.SMTPAuthMethod)
			applyEmailString(&entry.SMTPAuthSource, item.SMTPAuthSource)
			applyEmailString(&entry.OAuthSource, item.OAuthSource)
			applyEmailString(&entry.OAuthAccountID, item.OAuthAccountID)
			applyEmailString(&entry.OAuthProvider, item.OAuthProvider)
			applyEmailString(&entry.OAuthClientID, item.OAuthClientID)
			if item.AgentAccess != nil {
				value := *item.AgentAccess
				entry.AgentAccess = &value
			} else if _, exists := old[item.ID]; !exists {
				denied := false
				entry.AgentAccess = &denied
			}
			if entry.ReceiveProtocol == "" {
				entry.ReceiveProtocol = "imap"
			}
			if entry.ReceiveTLS == "" {
				entry.ReceiveTLS = "tls"
			}
			if entry.SMTPTransport == "" {
				if entry.SMTPPort == 465 {
					entry.SMTPTransport = "tls"
				} else {
					entry.SMTPTransport = "starttls"
				}
			}
			if entry.SMTPUsername == "" {
				entry.SMTPUsername = entry.Username
			}
			if entry.SMTPAuthMethod == "" {
				entry.SMTPAuthMethod = "plain"
			}
			entry.SMTPAuthMethod = normalizeEmailSMTPAuthMethod(entry.SMTPAuthMethod)
			if entry.ReceiveProtocol != "imap" && entry.ReceiveProtocol != "pop3" {
				return fmt.Errorf("unsupported receive protocol")
			}
			if entry.ReceiveTLS != "tls" && entry.ReceiveTLS != "starttls" || entry.SMTPTransport != "tls" && entry.SMTPTransport != "starttls" {
				return fmt.Errorf("email transports must use TLS")
			}
			if entry.SMTPAuthMethod != "plain" && entry.SMTPAuthMethod != "login" && entry.SMTPAuthMethod != "xoauth2" {
				return fmt.Errorf("unsupported SMTP authentication method")
			}
			if entry.OAuthSource != "" && entry.OAuthSource != "gog" && entry.OAuthSource != "mailer" {
				return fmt.Errorf("unsupported OAuth source")
			}
			if entry.OAuthSource == "gog" {
				parent, ok := cfg.Gog.Account(entry.OAuthAccountID)
				if !ok || entry.OAuthProvider != "google" || entry.SMTPAuthMethod != "xoauth2" || !parent.Enabled {
					return fmt.Errorf("Google OAuth account reference is invalid")
				}
			} else if entry.OAuthSource == "mailer" {
				if entry.OAuthProvider != "microsoft" || entry.OAuthClientID == "" || entry.SMTPAuthMethod != "xoauth2" {
					return fmt.Errorf("Microsoft OAuth account setup is incomplete")
				}
				if entry.OAuthClientSecret == "" && s.Vault != nil {
					entry.OAuthClientSecret, _ = s.Vault.ReadSecret("email_" + item.ID + "_oauth_client_secret")
				}
				if entry.OAuthClientSecret == "" {
					return fmt.Errorf("Microsoft OAuth client secret is unavailable")
				}
			}
			entries = append(entries, entry)
		}
		cfg.EmailAccounts = entries
		return nil
	}, secretChanges)
	if err != nil {
		s.Logger.Error("[EmailAccounts] Update failed", "error", err)
		jsonError(w, "Failed to save email accounts", http.StatusBadRequest)
		return
	}
	s.Logger.Info("[EmailAccounts] Updated", "count", len(incoming))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"count":  len(incoming),
	})
}

func emailStringPointer(value string) *string { return &value }
func emailIntPointer(value int) *int          { return &value }

func normalizeEmailSMTPAuthMethod(method string) string {
	if method == "password" {
		return "plain"
	}
	return method
}

func applyEmailString(dst *string, value *string) {
	if value != nil {
		*dst = *value
	}
}

func emailMode(value *string, fallback string) string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return fallback
	}
	return strings.TrimSpace(*value)
}

func applyEmailInt(dst *int, value *int) {
	if value != nil {
		*dst = *value
	}
}
