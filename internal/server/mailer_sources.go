package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/mailer"
	"aurago/internal/security"
)

func mailerClassicTransport(a config.EmailAccount) mailer.TransportConfig {
	protocol := strings.ToLower(a.ReceiveProtocol)
	if protocol == "" {
		protocol = "imap"
	}
	address := a.FromAddress
	if address == "" {
		address = a.Username
	}
	return mailer.TransportConfig{ID: a.ID, Source: "email", Name: a.Name, Address: address, ReceiveProtocol: protocol, ReceiveTLS: a.ReceiveTLS,
		IMAPHost: a.IMAPHost, IMAPPort: a.IMAPPort, POP3Host: a.POP3Host, POP3Port: a.POP3Port, Username: a.Username,
		SMTPHost: a.SMTPHost, SMTPPort: a.SMTPPort, SMTPUsername: a.SMTPUsername, SMTPAuthMethod: a.SMTPAuthMethod, SMTPTransport: a.SMTPTransport,
		FromAddress: address, ReadOnly: a.ReadOnly, AgentAccess: a.AllowsAgentAccess(), OAuthSource: a.OAuthSource, OAuthAccountID: a.OAuthAccountID,
		OAuthProvider: a.OAuthProvider, OAuthClientID: a.OAuthClientID, AgentMailInboxID: a.AgentMailInboxID}
}

func mailerGmailCapabilities(read, send, update, create, remove, readonly bool) []string {
	caps := []string{}
	if read {
		caps = append(caps, "read", "search", "folders_read")
	}
	if readonly {
		return caps
	}
	if send {
		caps = append(caps, "send")
	}
	if update {
		caps = append(caps, "read_state", "star", "move", "archive", "trash", "restore", "junk", "labels_write")
	}
	if create {
		caps = append(caps, "folders_create")
	}
	if remove {
		caps = append(caps, "delete")
	}
	return caps
}

func mailerScopedGmailCapabilities(a *mailer.Account, grants config.GogGrants, scopes gog.MailerGmailScopes) {
	a.Capabilities = mailerGmailCapabilities(grants.Read && scopes.Read, grants.Send && scopes.Send,
		grants.Update && scopes.Modify, grants.Create && scopes.CreateLabel, grants.Delete && scopes.PermanentDelete, a.ReadOnly)
	if a.Reason == "" && (grants.Read && !scopes.Read || !a.ReadOnly &&
		(grants.Send && !scopes.Send || grants.Update && !scopes.Modify || grants.Create && !scopes.CreateLabel || grants.Delete && !scopes.PermanentDelete)) {
		a.Reason = "missing_scope"
	}
}

// Source enumeration is passive. It never connects, refreshes tokens, or copies
// credentials into the Mailer store or an HTTP response.
func (s *Server) mailerSourceAccounts(ctx context.Context) ([]mailer.Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, mailer.ErrDisabled
	}
	accounts := []mailer.Account{}
	for _, entry := range cfg.EmailAccounts {
		a := mailer.AccountFromTransportConfig(mailerClassicTransport(entry))
		a.Disabled = entry.Disabled
		a.ReauthorizeSupported = entry.OAuthSource == "gog" && entry.OAuthProvider == "google" || entry.OAuthSource == "mailer" && entry.OAuthProvider == "microsoft"
		if entry.OAuthSource == "gog" {
			parent, exists := cfg.Gog.Account(entry.OAuthAccountID)
			grants := parent.Services["gmail"]
			a.Disabled = a.Disabled || !exists || !cfg.Gog.Enabled || !parent.Enabled || !parent.DesktopAccess || !grants.Read || !cfg.Agent.AllowNetworkRequests
			a.ReadOnly = a.ReadOnly || cfg.Gog.ReadOnly
			scopes := s.gogService().MailerGmailScopeCapabilities(entry.OAuthAccountID)
			mailerScopedGmailCapabilities(&a, grants, gog.MailerGmailScopes{Read: scopes.Protocol, Send: scopes.Protocol, Modify: scopes.Protocol, CreateLabel: scopes.Protocol, PermanentDelete: scopes.Protocol})
		}
		if a.Disabled {
			a.Reason = "account_disabled"
		}
		if a.Name == "" {
			a.Name = a.Address
		}
		accounts = append(accounts, a)
	}
	for _, entry := range cfg.Gog.Accounts {
		grants := entry.Services["gmail"]
		a := mailer.Account{ID: "gog:" + entry.ID, Source: "gog", Protocol: "gmail_api", Name: entry.Name, AgentAccess: entry.AgentAccess, ReadOnly: cfg.Gog.ReadOnly,
			ReauthorizeSupported: true}
		if a.Name == "" {
			a.Name = entry.ID
		}
		switch {
		case !cfg.Gog.Enabled || !entry.Enabled:
			a.Disabled = true
			a.Reason = "account_disabled"
		case !entry.DesktopAccess:
			a.Disabled = true
			a.Reason = "desktop_access_required"
		case !grants.Read:
			a.Disabled = true
			a.Reason = "mail_read_required"
		case !cfg.Agent.AllowNetworkRequests:
			a.Disabled = true
			a.Reason = "network_disabled"
		}
		mailerScopedGmailCapabilities(&a, grants, s.gogService().MailerGmailScopeCapabilities(entry.ID))
		accounts = append(accounts, a)
	}
	gw := cfg.GoogleWorkspace
	if gw.ClientID != "" || gw.Enabled {
		a := mailer.Account{ID: "google_workspace:default", Source: "google_workspace", Protocol: "gmail_api", Name: "Google Workspace", ReadOnly: gw.ReadOnly,
			AgentAccess: gw.Enabled && gw.Gmail, ReauthorizeSupported: true}
		switch {
		case !gw.Enabled:
			a.Disabled = true
			a.Reason = "account_disabled"
		case !gw.Gmail:
			a.Disabled = true
			a.Reason = "mail_read_required"
		case !cfg.Agent.AllowNetworkRequests:
			a.Disabled = true
			a.Reason = "network_disabled"
		}
		mailerScopedGmailCapabilities(&a, config.GogGrants{Read: gw.Gmail, Send: gw.GmailSend, Update: gw.GmailModifyLabels, Create: gw.GmailModifyLabels}, s.googleWorkspaceMailerScopeCapabilities())
		accounts = append(accounts, a)
	}
	am := cfg.AgentMail
	if am.InboxID != "" {
		a := mailer.AccountFromTransportConfig(mailer.TransportConfig{ID: am.InboxID, Source: "agentmail", Name: am.DisplayName, Address: am.InboxID, ReceiveProtocol: "agentmail", ReadOnly: am.ReadOnly, AgentAccess: am.Enabled})
		if a.Name == "" {
			a.Name = "AgentMail"
		}
		a.Disabled = !am.Enabled
		if a.Disabled {
			a.Reason = "account_disabled"
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (s *Server) mailerSource(ctx context.Context, id string) (mailer.Account, error) {
	accounts, err := s.mailerSourceAccounts(ctx)
	if err != nil {
		return mailer.Account{}, err
	}
	for _, a := range accounts {
		if a.ID == id {
			return a, nil
		}
	}
	return mailer.Account{}, mailer.ErrNotFound
}

func (s *Server) mailerSecret(key, fallback string) string {
	value := fallback
	if s.Vault != nil {
		value, _ = s.Vault.ReadSecret(key)
	}
	security.RegisterSensitive(value)
	return value
}

func (s *Server) resolveMailerAdapter(ctx context.Context, id string) (mailer.Adapter, mailer.Account, error) {
	account, err := s.mailerSource(ctx, id)
	if err != nil {
		return nil, account, err
	}
	if account.Disabled {
		return nil, account, mailer.ErrDisabled
	}
	if account.Address == "" && s.Mailer != nil {
		linked, err := s.Mailer.Accounts(ctx)
		if err != nil {
			return nil, account, err
		}
		for _, saved := range linked {
			if saved.ID == account.ID {
				account.Address = saved.Address
				break
			}
		}
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return nil, account, mailer.ErrDisabled
	}
	source, key, ok := strings.Cut(id, ":")
	if !ok || key == "" {
		return nil, account, mailer.ErrNotFound
	}
	transport := mailer.TransportConfig{ID: key, Source: source, Name: account.Name, Address: account.Address, FromAddress: account.Address,
		ReceiveProtocol: account.Protocol, ReadOnly: account.ReadOnly, AgentAccess: account.AgentAccess,
		HTTPClient: security.NewSSRFProtectedHTTPClientSameOrigin(30 * time.Second)}
	switch source {
	case "email":
		entry := cfg.FindEmailAccount(key)
		if entry == nil {
			return nil, account, mailer.ErrNotFound
		}
		transport = mailerClassicTransport(*entry)
		transport.Password = s.mailerSecret("email_"+entry.ID+"_password", entry.Password)
		transport.SMTPPassword = s.mailerSecret("email_"+entry.ID+"_smtp_password", entry.SMTPPassword)
		transport.OAuthClientSecret = s.mailerSecret("email_"+entry.ID+"_oauth_client_secret", entry.OAuthClientSecret)
		a := *entry
		transport.Token = func(ctx context.Context, grant string, _ bool) (string, error) {
			if a.OAuthSource == "gog" {
				return s.gogService().MailProtocolToken(ctx, a.OAuthAccountID, grant)
			}
			return s.mailerOAuthAccessToken(ctx, a, grant)
		}
	case "gog":
		transport.Token = func(ctx context.Context, grant string, _ bool) (string, error) {
			return s.gogService().MailAccessToken(ctx, key, grant)
		}
	case "google_workspace":
		transport.Token = func(ctx context.Context, grant string, _ bool) (string, error) {
			return s.mailerOAuthAccessToken(ctx, config.EmailAccount{OAuthSource: "google_workspace"}, grant)
		}
	case "agentmail":
		transport.AgentMailInboxID = cfg.AgentMail.InboxID
		transport.AgentMailBaseURL = cfg.AgentMail.BaseURL
		transport.AgentMailAPIKey = s.mailerSecret("agentmail_api_key", cfg.AgentMail.APIKey)
	default:
		return nil, account, mailer.ErrNotFound
	}
	adapter, err := mailer.AdapterFor(transport)
	if err != nil {
		return nil, account, fmt.Errorf("mailer adapter unavailable: %w", err)
	}
	return &mailerGuardedAdapter{inner: adapter, server: s, id: id, identity: mailerSourceIdentity(cfg, account)}, account, nil
}
