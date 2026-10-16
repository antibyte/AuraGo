package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/fileutil"
	"aurago/internal/gog"
	"aurago/internal/mailer"
	"aurago/internal/security"
	"gopkg.in/yaml.v3"
)

func registerMailerAccountRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/mailer/accounts/create", s.handleMailerCreateAccount)
	mux.HandleFunc("/api/desktop/mailer/oauth/start", s.handleMailerOAuthStart)
	mux.HandleFunc("/api/desktop/mailer/oauth/callback", s.handleMailerOAuthCallback)
}

type mailerCreateAccountRequest struct {
	Type              string `json:"type"`
	ID                string `json:"id"`
	Name              string `json:"name"`
	Address           string `json:"address"`
	ReceiveProtocol   string `json:"receive_protocol"`
	ReceiveTLS        string `json:"receive_tls"`
	IMAPHost          string `json:"imap_host"`
	IMAPPort          int    `json:"imap_port"`
	POP3Host          string `json:"pop3_host"`
	POP3Port          int    `json:"pop3_port"`
	SMTPHost          string `json:"smtp_host"`
	SMTPPort          int    `json:"smtp_port"`
	SMTPTransport     string `json:"smtp_tls"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	SMTPUsername      string `json:"smtp_username"`
	SMTPPassword      string `json:"smtp_password"`
	SMTPAuthMethod    string `json:"smtp_auth_method"`
	OAuthClientID     string `json:"oauth_client_id"`
	OAuthClientSecret string `json:"oauth_client_secret"`
	GogAccountID      string `json:"gog_account_id"`
}

func (s *Server) mailerSetupAllowed(w http.ResponseWriter, r *http.Request) bool {
	if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
		return false
	}
	if r.Method != http.MethodPost {
		mailerJSON(w, 405, map[string]string{"error": "method_not_allowed"})
		return false
	}
	if !mailerOriginOK(r) {
		mailerJSON(w, 403, map[string]string{"error": "same_origin_required"})
		return false
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		mailerHTTPError(w, mailer.ErrDisabled)
		return false
	}
	if cfg.VirtualDesktop.ReadOnly {
		mailerHTTPError(w, mailer.ErrReadOnly)
		return false
	}
	if s.Mailer == nil {
		mailerJSON(w, 503, map[string]string{"error": "mailer_unavailable"})
		return false
	}
	return true
}

func (s *Server) handleMailerCreateAccount(w http.ResponseWriter, r *http.Request) {
	if !s.mailerSetupAllowed(w, r) {
		return
	}
	var body mailerCreateAccountRequest
	if decodeMailerJSON(w, r, &body, 32<<10) != nil {
		mailerHTTPError(w, mailer.ErrInvalid)
		return
	}
	if body.GogAccountID != "" && (body.ReceiveProtocol == "" || body.ReceiveProtocol == "gmail_api") {
		if !strings.HasPrefix(body.GogAccountID, "gog:") && body.GogAccountID != "google_workspace:default" {
			mailerHTTPError(w, mailer.ErrInvalid)
			return
		}
		err := s.Mailer.Connect(r.Context(), body.GogAccountID)
		if err != nil {
			mailerHTTPError(w, err)
			return
		}
		mailerJSON(w, 201, map[string]any{"account_id": body.GogAccountID, "connected": true})
		return
	}
	accountID, oauth, err := s.createMailerAccount(r.Context(), body)
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	response := map[string]any{"account_id": accountID, "oauth_required": oauth, "connected": false}
	if !oauth {
		connectErr := s.Mailer.Connect(r.Context(), accountID)
		if connectErr == nil {
			response["connected"] = true
		} else {
			response["connection_error"] = "connection_required"
		}
	}
	mailerJSON(w, 201, response)
}

func validMailerHost(host string) bool {
	return host != "" && len(host) <= 253 && !strings.ContainsAny(host, " /\\\r\n\t@?#")
}

func (s *Server) createMailerAccount(ctx context.Context, b mailerCreateAccountRequest) (string, bool, error) {
	b.Address = strings.TrimSpace(b.Address)
	address, err := mail.ParseAddress(b.Address)
	if err != nil || address.Address != b.Address || len(b.Address) > 254 || len(b.Name) > 120 || strings.ContainsAny(b.Name, "\r\n") {
		return "", false, mailer.ErrInvalid
	}
	if b.Type != "email" && b.Type != "google" && b.Type != "microsoft" {
		return "", false, mailer.ErrInvalid
	}
	for _, secret := range []string{b.Password, b.SMTPPassword, b.OAuthClientSecret} {
		if len(secret) > 4096 {
			return "", false, mailer.ErrInvalid
		}
		security.RegisterSensitive(secret)
	}
	if s.Vault == nil {
		return "", false, fmt.Errorf("mailer vault unavailable")
	}
	id := strings.ToLower(strings.TrimSpace(b.ID))
	if !config.ValidGogAccountID(id) {
		var entropy [8]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			return "", false, err
		}
		id = "mail-" + hex.EncodeToString(entropy[:])
	}
	if b.Name == "" {
		b.Name = b.Address
	}
	oauth := b.Type != "email"
	googleProtocol := b.Type == "google" && (b.ReceiveProtocol == "imap" || b.ReceiveProtocol == "pop3")
	if oauth && !(googleProtocol && b.GogAccountID != "") && (b.OAuthClientID == "" || len(b.OAuthClientID) > 256 || b.OAuthClientSecret == "") {
		return "", false, mailer.ErrInvalid
	}
	sourceID := "email:" + id
	if b.Type == "google" && !googleProtocol {
		sourceID = "gog:" + id
	}
	secretChanges := make([]mailerVaultChange, 0, 4)
	if b.Type == "google" && b.GogAccountID == "" {
		secretChanges = append(secretChanges, mailerVaultChange{key: gog.ClientKey(id), value: b.OAuthClientSecret})
	} else if b.OAuthClientSecret != "" && !googleProtocol {
		secretChanges = append(secretChanges, mailerVaultChange{key: "email_" + id + "_oauth_client_secret", value: b.OAuthClientSecret})
	}
	if b.Password != "" {
		secretChanges = append(secretChanges, mailerVaultChange{key: "email_" + id + "_password", value: b.Password})
	}
	if b.SMTPPassword != "" {
		secretChanges = append(secretChanges, mailerVaultChange{key: "email_" + id + "_smtp_password", value: b.SMTPPassword})
	}
	err = s.persistMailerAccountsWithSecrets(ctx, func(cfg *config.Config) error {
		if cfg.FindEmailAccount(id) != nil {
			return mailer.ErrConflict
		}
		if _, exists := cfg.Gog.Account(id); exists {
			return mailer.ErrConflict
		}
		if b.Type == "google" {
			if b.GogAccountID == "" {
				if len(cfg.Gog.Accounts) >= 20 {
					return mailer.ErrInvalid
				}
				// Enabling a first account cannot activate another user's existing grants.
				if len(cfg.Gog.Accounts) == 0 && !cfg.Gog.Enabled {
					cfg.Gog.Enabled = true
					cfg.Gog.ReadOnly = false
				}
				cfg.Gog.Accounts = append(cfg.Gog.Accounts, config.GogAccount{ID: id, Name: b.Name, ClientID: b.OAuthClientID, Enabled: true, DesktopAccess: true,
					Services: map[string]config.GogGrants{"gmail": {Read: true, Create: true, Update: true, Send: true}}})
				b.GogAccountID = "gog:" + id
			} else {
				if !strings.HasPrefix(b.GogAccountID, "gog:") {
					return mailer.ErrInvalid
				}
				if _, ok := cfg.Gog.Account(strings.TrimPrefix(b.GogAccountID, "gog:")); !ok {
					return mailer.ErrNotFound
				}
			}
			if !googleProtocol {
				return nil
			}
		}
		if len(cfg.EmailAccounts) >= 100 {
			return mailer.ErrInvalid
		}
		if b.Username == "" {
			b.Username = b.Address
		}
		if len(b.Username) > 320 || len(b.SMTPUsername) > 320 || strings.ContainsAny(b.Username+b.SMTPUsername, "\r\n") {
			return mailer.ErrInvalid
		}
		if b.ReceiveProtocol == "" {
			b.ReceiveProtocol = "imap"
		}
		if b.ReceiveProtocol != "imap" && b.ReceiveProtocol != "pop3" {
			return mailer.ErrInvalid
		}
		if b.ReceiveTLS == "" {
			b.ReceiveTLS = "tls"
		}
		if b.ReceiveTLS != "tls" && b.ReceiveTLS != "starttls" {
			return mailer.ErrInvalid
		}
		if b.Type == "microsoft" {
			if b.IMAPHost == "" {
				b.IMAPHost = "outlook.office365.com"
			}
			if b.POP3Host == "" {
				b.POP3Host = "outlook.office365.com"
			}
			if b.SMTPHost == "" {
				b.SMTPHost = "smtp.office365.com"
			}
			b.SMTPAuthMethod = "xoauth2"
		}
		if googleProtocol {
			if b.IMAPHost == "" {
				b.IMAPHost = "imap.gmail.com"
			}
			if b.POP3Host == "" {
				b.POP3Host = "pop.gmail.com"
			}
			if b.SMTPHost == "" {
				b.SMTPHost = "smtp.gmail.com"
			}
			if b.SMTPPort == 0 {
				b.SMTPPort = 465
				b.SMTPTransport = "tls"
			}
			b.SMTPAuthMethod = "xoauth2"
		}
		if b.IMAPPort == 0 {
			b.IMAPPort = 993
			if b.ReceiveTLS == "starttls" {
				b.IMAPPort = 143
			}
		}
		if b.POP3Port == 0 {
			b.POP3Port = 995
			if b.ReceiveTLS == "starttls" {
				b.POP3Port = 110
			}
		}
		if b.SMTPPort == 0 {
			b.SMTPPort = 587
		}
		if b.SMTPTransport == "" {
			b.SMTPTransport = "starttls"
			if b.SMTPPort == 465 {
				b.SMTPTransport = "tls"
			}
		}
		if b.SMTPTransport != "tls" && b.SMTPTransport != "starttls" {
			return mailer.ErrInvalid
		}
		if !validMailerHost(b.SMTPHost) || b.SMTPPort < 1 || b.SMTPPort > 65535 {
			return mailer.ErrInvalid
		}
		if b.ReceiveProtocol == "imap" && (!validMailerHost(b.IMAPHost) || b.IMAPPort < 1 || b.IMAPPort > 65535) {
			return mailer.ErrInvalid
		}
		if b.ReceiveProtocol == "pop3" && (!validMailerHost(b.POP3Host) || b.POP3Port < 1 || b.POP3Port > 65535) {
			return mailer.ErrInvalid
		}
		if b.SMTPAuthMethod == "" {
			b.SMTPAuthMethod = "plain"
		}
		if b.SMTPAuthMethod != "plain" && b.SMTPAuthMethod != "login" && !(oauth && b.SMTPAuthMethod == "xoauth2") {
			return mailer.ErrInvalid
		}
		if !oauth && b.Password == "" {
			return mailer.ErrInvalid
		}
		denied := false
		a := config.EmailAccount{ID: id, Name: b.Name, FromAddress: b.Address, Username: b.Username, Password: b.Password, ReceiveProtocol: b.ReceiveProtocol, ReceiveTLS: b.ReceiveTLS,
			IMAPHost: b.IMAPHost, IMAPPort: b.IMAPPort, POP3Host: b.POP3Host, POP3Port: b.POP3Port, SMTPHost: b.SMTPHost, SMTPPort: b.SMTPPort, SMTPTransport: b.SMTPTransport,
			SMTPUsername: b.SMTPUsername, SMTPPassword: b.SMTPPassword, SMTPAuthMethod: b.SMTPAuthMethod, AgentAccess: &denied}
		if oauth {
			a.OAuthSource = "mailer"
			a.OAuthProvider = b.Type
			a.OAuthClientID = b.OAuthClientID
			a.OAuthClientSecret = b.OAuthClientSecret
		}
		if googleProtocol {
			a.OAuthSource, a.OAuthAccountID, a.OAuthProvider = "gog", strings.TrimPrefix(b.GogAccountID, "gog:"), "google"
			a.OAuthClientID, a.OAuthClientSecret = "", ""
			b.OAuthClientSecret = ""
		}
		cfg.EmailAccounts = append(cfg.EmailAccounts, a)
		return nil
	}, secretChanges)
	return sourceID, oauth, err
}

type mailerVaultChange struct {
	key   string
	value string
}

type mailerVaultSnapshot struct {
	key     string
	value   string
	exists  bool
	written string
}

// Serialize account writes with all other configuration editors, retaining
// unrelated YAML and publishing the new permissions only after durable storage.
func (s *Server) persistMailerAccounts(ctx context.Context, mutate func(*config.Config) error) error {
	return s.persistMailerAccountsWithSecrets(ctx, mutate, nil)
}

func (s *Server) persistMailerAccountsWithSecrets(ctx context.Context, mutate func(*config.Config) error, secrets []mailerVaultChange) error {
	return s.persistMailerAccountsWithSecretsAndWriter(ctx, mutate, secrets, fileutil.WriteFileContext)
}

func (s *Server) persistMailerAccountsWithSecretsAndWriter(
	ctx context.Context,
	mutate func(*config.Config) error,
	secrets []mailerVaultChange,
	writeConfig func(context.Context, string, []byte, os.FileMode) error,
) error {
	return s.persistAccountConfigWithSecretsAndWriter(ctx, mutate, secrets, writeConfig, true)
}

func (s *Server) persistConfiguredEmailAccountsWithSecrets(ctx context.Context, mutate func(*config.Config) error, secrets []mailerVaultChange) error {
	return s.persistAccountConfigWithSecretsAndWriter(ctx, mutate, secrets, fileutil.WriteFileContext, false)
}

func (s *Server) persistAccountConfigWithSecretsAndWriter(
	ctx context.Context,
	mutate func(*config.Config) error,
	secrets []mailerVaultChange,
	writeConfig func(context.Context, string, []byte, os.FileMode) error,
	requireDesktopWrite bool,
) error {
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	snapshot := s.ConfigSnapshot()
	if snapshot == nil {
		return fmt.Errorf("mailer configuration unavailable")
	}
	cfg := snapshot.Clone()
	if requireDesktopWrite && !cfg.VirtualDesktop.Enabled {
		return mailer.ErrDisabled
	}
	if requireDesktopWrite && cfg.VirtualDesktop.ReadOnly {
		return mailer.ErrReadOnly
	}
	if cfg.ConfigPath == "" {
		return fmt.Errorf("mailer configuration path unavailable")
	}
	data, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw == nil {
		raw = map[string]any{}
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	raw["email_accounts"], raw["gog"] = cfg.EmailAccounts, cfg.Gog
	encoded, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if writeConfig == nil {
		writeConfig = fileutil.WriteFileContext
	}
	if len(secrets) > 0 && s.Vault == nil {
		return fmt.Errorf("mailer vault unavailable")
	}
	previous := make([]mailerVaultSnapshot, 0, len(secrets))
	seenSecrets := make(map[string]struct{}, len(secrets))
	for _, change := range secrets {
		if strings.TrimSpace(change.key) == "" {
			return fmt.Errorf("invalid mailer credential key")
		}
		if _, duplicate := seenSecrets[change.key]; duplicate {
			return fmt.Errorf("duplicate mailer credential key")
		}
		seenSecrets[change.key] = struct{}{}
		snapshot := mailerVaultSnapshot{key: change.key, written: change.value}
		old, readErr := s.Vault.ReadSecret(change.key)
		switch {
		case readErr == nil:
			snapshot.value, snapshot.exists = old, true
		case errors.Is(readErr, security.ErrSecretNotFound):
		default:
			return fmt.Errorf("read prior mailer credential: %w", readErr)
		}
		previous = append(previous, snapshot)
	}
	rollback := func(cause error, applied []mailerVaultSnapshot) error {
		if restoreErr := s.restoreMailerVaultChanges(applied); restoreErr != nil {
			return errors.Join(cause, fmt.Errorf("restore mailer credentials: %w", restoreErr))
		}
		return cause
	}
	applied := make([]mailerVaultSnapshot, 0, len(previous))
	for _, snapshot := range previous {
		if err := ctx.Err(); err != nil {
			return rollback(err, applied)
		}
		var writeErr error
		if snapshot.exists {
			writeErr = s.Vault.CompareAndSwapSecret(snapshot.key, snapshot.value, snapshot.written)
		} else {
			writeErr = s.Vault.WriteSecretIfAbsent(snapshot.key, snapshot.written)
		}
		if writeErr != nil {
			return rollback(fmt.Errorf("store mailer credential: %w", writeErr), applied)
		}
		applied = append(applied, snapshot)
	}
	if err := ctx.Err(); err != nil {
		return rollback(err, applied)
	}
	if err := writeConfig(ctx, cfg.ConfigPath, encoded, 0600); err != nil {
		return rollback(err, applied)
	}
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg)
	s.CfgMu.Unlock()
	return nil
}

func (s *Server) restoreMailerVaultChanges(changes []mailerVaultSnapshot) error {
	var restoreErrors []error
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		if change.exists {
			if err := s.Vault.CompareAndSwapSecret(change.key, change.written, change.value); err != nil && !errors.Is(err, security.ErrSecretChanged) {
				restoreErrors = append(restoreErrors, err)
			}
			continue
		}
		if err := s.Vault.CompareAndDeleteSecret(change.key, change.written); err != nil && !errors.Is(err, security.ErrSecretChanged) {
			restoreErrors = append(restoreErrors, err)
		}
	}
	return errors.Join(restoreErrors...)
}

func (s *Server) updateMailerAgentAccess(ctx context.Context, id string, enabled bool) error {
	return s.persistMailerAccounts(ctx, func(cfg *config.Config) error {
		source, key, ok := strings.Cut(id, ":")
		if !ok {
			return mailer.ErrInvalid
		}
		switch source {
		case "email":
			for i := range cfg.EmailAccounts {
				if cfg.EmailAccounts[i].ID == key {
					a := &cfg.EmailAccounts[i]
					if enabled && (a.ReceiveProtocol == "pop3" || a.OAuthSource != "") {
						return mailer.ErrCapability
					}
					a.AgentAccess = &enabled
					return nil
				}
			}
		case "gog":
			for i := range cfg.Gog.Accounts {
				if cfg.Gog.Accounts[i].ID == key {
					cfg.Gog.Accounts[i].AgentAccess = enabled
					return nil
				}
			}
		default:
			return mailer.ErrCapability
		}
		return mailer.ErrNotFound
	})
}

func (s *Server) mailerTestConnection(w http.ResponseWriter, r *http.Request, id string, receive, send bool) {
	if !receive && !send {
		mailerHTTPError(w, mailer.ErrInvalid)
		return
	}
	adapter, _, err := s.resolveMailerAdapter(r.Context(), id)
	if err != nil {
		mailerHTTPError(w, err)
		return
	}
	result := map[string]string{"receive": "skipped", "send": "skipped"}
	probe := func(kind string, check func(context.Context) error) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if check(ctx) != nil {
			result[kind] = "error"
			result[kind+"_error"] = "connection_failed"
		} else {
			result[kind] = "ok"
		}
	}
	if receive {
		probe("receive", adapter.Test)
	}
	if send {
		if tester, ok := adapter.(mailer.SendTester); ok {
			probe("send", tester.TestSend)
		} else {
			result["send"] = "error"
			result["send_error"] = "capability_unavailable"
		}
	}
	mailerJSON(w, 200, result)
}
