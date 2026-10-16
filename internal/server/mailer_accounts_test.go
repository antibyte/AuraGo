package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/mailer"
	"aurago/internal/security"
)

func newMailerAccountTestServer(t *testing.T) *Server {
	t.Helper()
	s := newDesktopOfficeTestServer(t)
	s.Cfg.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(s.Cfg.ConfigPath, []byte("unrelated_setting: preserved\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var err error
	s.Vault, err = security.NewVault(strings.Repeat("ab", 32), filepath.Join(t.TempDir(), "vault.bin"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMailerCreateAccountPreservesSharedGrantsAndKeepsSecretsInVault(t *testing.T) {
	s := newMailerAccountTestServer(t)
	s.Cfg.EmailAccounts = []config.EmailAccount{{ID: "legacy", Username: "legacy@example.test", WatchEnabled: true}}
	body := mailerCreateAccountRequest{Type: "email", ID: "new-mail", Name: "New", Address: "owner@example.test", Password: "mailer-fixture-password", SMTPPassword: "mailer-fixture-smtp",
		ReceiveProtocol: "pop3", POP3Host: "pop.example.test", SMTPHost: "smtp.example.test"}
	id, oauth, err := s.createMailerAccount(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	if id != "email:new-mail" || oauth {
		t.Fatalf("unexpected identity %q %v", id, oauth)
	}
	cfg := s.ConfigSnapshot()
	account := cfg.FindEmailAccount("new-mail")
	if account == nil || account.AgentAccess == nil || *account.AgentAccess || account.WatchEnabled || account.ReadOnly || account.POP3Port != 995 {
		t.Fatalf("unsafe account defaults: %#v", account)
	}
	if !cfg.EmailAccounts[0].AllowsAgentAccess() || !cfg.EmailAccounts[0].WatchEnabled {
		t.Fatal("legacy grants changed")
	}
	data, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), body.Password) || strings.Contains(string(data), body.SMTPPassword) || !strings.Contains(string(data), "unrelated_setting: preserved") {
		t.Fatal("secret leaked or unrelated YAML lost")
	}
	for suffix, expected := range map[string]string{"password": body.Password, "smtp_password": body.SMTPPassword} {
		actual, err := s.Vault.ReadSecret("email_new-mail_" + suffix)
		if err != nil || actual != expected {
			t.Fatal("credential not stored in its canonical Vault entry")
		}
	}
	if _, _, err := s.createMailerAccount(context.Background(), body); !errors.Is(err, mailer.ErrConflict) {
		t.Fatalf("duplicate account accepted: %v", err)
	}
	if err := s.updateMailerAgentAccess(context.Background(), id, true); !errors.Is(err, mailer.ErrCapability) {
		t.Fatal("unsupported POP3 agent access granted")
	}
}

func TestMailerCreateGoogleUsesCanonicalGoGWithoutExpandingExistingPermissions(t *testing.T) {
	s := newMailerAccountTestServer(t)
	s.Cfg.Gog = config.GogConfig{Enabled: false, ReadOnly: true, Accounts: []config.GogAccount{{ID: "old", Enabled: true, AgentAccess: true}}}
	body := mailerCreateAccountRequest{Type: "google", ID: "new-google", Address: "owner@example.test", OAuthClientID: "fixture-client", OAuthClientSecret: "fixture-client-secret"}
	id, oauth, err := s.createMailerAccount(context.Background(), body)
	if err != nil {
		t.Fatal(err)
	}
	cfg := s.ConfigSnapshot()
	a, found := cfg.Gog.Account("new-google")
	if !found || id != "gog:new-google" || !oauth || cfg.Gog.Enabled || !cfg.Gog.ReadOnly || a.AgentAccess || !a.DesktopAccess || !a.Services["gmail"].Send {
		t.Fatal("new Google account expanded existing integration permissions")
	}
	secret, err := s.Vault.ReadSecret(gog.ClientKey(a.ID))
	if err != nil || secret != body.OAuthClientSecret {
		t.Fatal("GoG canonical credential missing")
	}
	if len(cfg.EmailAccounts) != 0 {
		t.Fatal("duplicated Google account as a classical account")
	}
}

func TestMailerAccountSetupRejectsReadOnlyAndPlaintextTransport(t *testing.T) {
	s := newMailerAccountTestServer(t)
	b := mailerCreateAccountRequest{Type: "email", ID: "fixture", Address: "owner@example.test", Password: "fixture-password", IMAPHost: "imap.example.test", SMTPHost: "smtp.example.test", ReceiveTLS: "none"}
	if _, _, err := s.createMailerAccount(context.Background(), b); !errors.Is(err, mailer.ErrInvalid) {
		t.Fatalf("plaintext accepted: %v", err)
	}
	b.ReceiveTLS = "tls"
	s.Cfg.VirtualDesktop.ReadOnly = true
	if _, _, err := s.createMailerAccount(context.Background(), b); !errors.Is(err, mailer.ErrReadOnly) {
		t.Fatalf("read-only setup accepted: %v", err)
	}
}

func TestMailerGoogleProtocolReferencesExistingGoGCredentials(t *testing.T) {
	s := newMailerAccountTestServer(t)
	s.Cfg.Gog = config.GogConfig{Enabled: true, Accounts: []config.GogAccount{{ID: "parent", Enabled: true, DesktopAccess: true, Services: map[string]config.GogGrants{"gmail": {Read: true, Send: true, Update: true}}}}}
	id, oauth, err := s.createMailerAccount(context.Background(), mailerCreateAccountRequest{Type: "google", ID: "protocol", Address: "owner@example.test", ReceiveProtocol: "pop3", GogAccountID: "gog:parent"})
	if err != nil {
		t.Fatal(err)
	}
	account := s.ConfigSnapshot().FindEmailAccount("protocol")
	if id != "email:protocol" || !oauth || account == nil || account.OAuthSource != "gog" || account.OAuthAccountID != "parent" || account.OAuthClientSecret != "" || account.POP3Host != "pop.gmail.com" {
		t.Fatal("Google protocol account did not retain canonical OAuth reference")
	}
	if _, err := s.Vault.ReadSecret("email_protocol_oauth_client_secret"); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("duplicated OAuth secret: %v", err)
	}
}

func TestMailerCreateAccountValidatesBeforeWritingCredentials(t *testing.T) {
	s := newMailerAccountTestServer(t)
	body := mailerCreateAccountRequest{
		Type: "google", ID: "invalid-google", Address: "owner@example.test", ReceiveProtocol: "imap",
		OAuthClientID: "fixture-client", OAuthClientSecret: "fixture-client-secret", SMTPHost: "bad/host",
	}
	if _, _, err := s.createMailerAccount(context.Background(), body); !errors.Is(err, mailer.ErrInvalid) {
		t.Fatalf("invalid account accepted: %v", err)
	}
	if _, err := s.Vault.ReadSecret(gog.ClientKey(body.ID)); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("validation failure left a GoG credential behind: %v", err)
	}
	if _, exists := s.ConfigSnapshot().Gog.Account(body.ID); exists {
		t.Fatal("validation failure published a GoG account")
	}
}

func TestMailerAccountConfigFailureRestoresPreviousCredential(t *testing.T) {
	s := newMailerAccountTestServer(t)
	const key = "email_existing_password"
	if err := s.Vault.WriteSecret(key, "previous-password"); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("simulated config write failure")
	err := s.persistMailerAccountsWithSecretsAndWriter(
		context.Background(), func(*config.Config) error { return nil },
		[]mailerVaultChange{{key: key, value: "new-password"}},
		func(context.Context, string, []byte, os.FileMode) error { return wantErr },
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("persistence error = %v, want %v", err, wantErr)
	}
	if got, readErr := s.Vault.ReadSecret(key); readErr != nil || got != "previous-password" {
		t.Fatalf("previous credential was not restored: got %q, err %v", got, readErr)
	}
}

func TestMailerAccountRollbackDoesNotOverwriteConcurrentVaultRotation(t *testing.T) {
	s := newMailerAccountTestServer(t)
	const key = "email_existing_oauth_client_secret"
	if err := s.Vault.WriteSecret(key, "old-client-secret"); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("simulated config write failure")
	err := s.persistMailerAccountsWithSecretsAndWriter(
		context.Background(), func(*config.Config) error { return nil },
		[]mailerVaultChange{{key: key, value: "new-client-secret"}},
		func(context.Context, string, []byte, os.FileMode) error {
			if err := s.Vault.WriteSecret(key, "rotated-client-secret"); err != nil {
				return err
			}
			return wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("persistence error = %v, want %v", err, wantErr)
	}
	if got, readErr := s.Vault.ReadSecret(key); readErr != nil || got != "rotated-client-secret" {
		t.Fatalf("rollback overwrote the newer credential: got %q, err %v", got, readErr)
	}
}

func TestMailerAccountCancellationRemovesNewCredential(t *testing.T) {
	s := newMailerAccountTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	const key = "email_cancelled_password"
	err := s.persistMailerAccountsWithSecretsAndWriter(
		ctx, func(*config.Config) error { return nil },
		[]mailerVaultChange{{key: key, value: "temporary-password"}},
		func(ctx context.Context, _ string, _ []byte, _ os.FileMode) error {
			cancel()
			return ctx.Err()
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("persistence error = %v, want cancellation", err)
	}
	if _, readErr := s.Vault.ReadSecret(key); !errors.Is(readErr, security.ErrSecretNotFound) {
		t.Fatalf("cancelled account left a credential behind: %v", readErr)
	}
}

func TestEmailAccountValidationFailureDoesNotOrphanCredentials(t *testing.T) {
	s := newMailerAccountTestServer(t)
	incoming := []emailAccountJSON{
		{ID: "first-valid", Name: "First", IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587,
			Username: "first@example.test", Password: "first-account-password", FromAddress: "first@example.test", Enabled: true, AllowSending: true},
		{ID: "second-invalid", Name: "Second", IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587,
			ReceiveProtocol: emailStringPointer("invalid"), Username: "second@example.test", Password: "second-account-password", FromAddress: "second@example.test", Enabled: true, AllowSending: true},
	}
	body, err := json.Marshal(incoming)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/email-accounts", strings.NewReader(string(body)))
	response := httptest.NewRecorder()
	handleEmailAccounts(s).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid update status = %d, want %d; body %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	for _, key := range []string{"email_first-valid_password", "email_second-invalid_password"} {
		if _, err := s.Vault.ReadSecret(key); !errors.Is(err, security.ErrSecretNotFound) {
			t.Fatalf("failed update left credential %q in Vault: %v", key, err)
		}
	}
	if len(s.ConfigSnapshot().EmailAccounts) != 0 {
		t.Fatal("failed update published a partial account list")
	}
}

func TestEmailAccountPutPersistsCredentialsInVaultAndPreservesMaskedRotation(t *testing.T) {
	s := newMailerAccountTestServer(t)
	smtpPassword := "explicit-smtp-password"
	incoming := []emailAccountJSON{{
		ID: "vault-roundtrip", Name: "Vault Roundtrip", IMAPHost: "imap.example.test", IMAPPort: 993,
		SMTPHost: "smtp.example.test", SMTPPort: 587, Username: "mail@example.test", FromAddress: "mail@example.test",
		Password: "explicit-imap-password", SMTPPassword: &smtpPassword, Enabled: true, AllowSending: true,
	}}
	body, err := json.Marshal(incoming)
	if err != nil {
		t.Fatal(err)
	}
	put := func(body []byte) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodPut, "/api/email-accounts", strings.NewReader(string(body)))
		response := httptest.NewRecorder()
		handleEmailAccounts(s).ServeHTTP(response, request)
		return response
	}
	response := put(body)
	if response.Code != http.StatusOK {
		t.Fatalf("credential update status = %d, want %d; body %s", response.Code, http.StatusOK, response.Body.String())
	}

	passwords := map[string]string{
		"email_vault-roundtrip_password":      incoming[0].Password,
		"email_vault-roundtrip_smtp_password": smtpPassword,
	}
	for key, expected := range passwords {
		if got, err := s.Vault.ReadSecret(key); err != nil || got != expected {
			t.Fatalf("Vault[%q] = %q, %v; want explicit credential", key, got, err)
		}
	}
	configPath := s.ConfigSnapshot().ConfigPath
	configYAML, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range passwords {
		if strings.Contains(string(configYAML), secret) {
			t.Fatal("email credential was written to config.yaml")
		}
	}
	if !strings.Contains(string(configYAML), "unrelated_setting: preserved") {
		t.Fatal("account update lost unrelated YAML")
	}

	rotated := map[string]string{
		"email_vault-roundtrip_password":      "newer-rotated-imap-password",
		"email_vault-roundtrip_smtp_password": "newer-rotated-smtp-password",
	}
	for key, secret := range rotated {
		if err := s.Vault.WriteSecret(key, secret); err != nil {
			t.Fatal(err)
		}
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/api/email-accounts", nil)
	getResponse := httptest.NewRecorder()
	handleEmailAccounts(s).ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResponse.Code, http.StatusOK)
	}
	var masked []emailAccountJSON
	if err := json.Unmarshal(getResponse.Body.Bytes(), &masked); err != nil {
		t.Fatal(err)
	}
	if len(masked) != 1 || masked[0].Password != maskedKey || masked[0].SMTPPassword == nil || *masked[0].SMTPPassword != maskedKey {
		t.Fatalf("GET did not mask both stored credentials: %#v", masked)
	}
	maskedBody, err := json.Marshal(masked)
	if err != nil {
		t.Fatal(err)
	}
	response = put(maskedBody)
	if response.Code != http.StatusOK {
		t.Fatalf("masked round-trip status = %d, want %d; body %s", response.Code, http.StatusOK, response.Body.String())
	}
	for key, expected := range rotated {
		if got, err := s.Vault.ReadSecret(key); err != nil || got != expected {
			t.Fatalf("masked update changed newer Vault value %q: got %q, %v", key, got, err)
		}
	}
}

func TestEmailAccountsKeepIntegrationPolicyAndDefaultSMTPTransport(t *testing.T) {
	s := newMailerAccountTestServer(t)
	s.Cfg.VirtualDesktop.Enabled = false
	s.Cfg.VirtualDesktop.ReadOnly = true
	incoming := []emailAccountJSON{
		{ID: "implicit-tls", Name: "TLS", IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 465,
			Username: "tls@example.test", FromAddress: "tls@example.test", Enabled: true, AllowSending: true},
		{ID: "implicit-starttls", Name: "STARTTLS", IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587,
			Username: "starttls@example.test", FromAddress: "starttls@example.test", Enabled: true, AllowSending: true},
	}
	body, err := json.Marshal(incoming)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/email-accounts", strings.NewReader(string(body)))
	response := httptest.NewRecorder()
	handleEmailAccounts(s).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("integration update status = %d, want %d; body %s", response.Code, http.StatusOK, response.Body.String())
	}
	for id, want := range map[string]string{"implicit-tls": "tls", "implicit-starttls": "starttls"} {
		account := s.ConfigSnapshot().FindEmailAccount(id)
		if account == nil || account.SMTPTransport != want {
			t.Fatalf("account %q SMTP transport = %#v, want %q", id, account, want)
		}
	}
}

func TestEmailAccountLegacyPasswordAuthRoundTripsAsPlainAndKeepsAgentGrant(t *testing.T) {
	s := newMailerAccountTestServer(t)
	allowed := true
	s.Cfg.EmailAccounts = []config.EmailAccount{{ID: "legacy-auth", Name: "Legacy", FromAddress: "legacy@example.test", Username: "legacy@example.test",
		IMAPHost: "imap.example.test", IMAPPort: 993, SMTPHost: "smtp.example.test", SMTPPort: 587, SMTPTransport: "starttls",
		SMTPAuthMethod: "password", AgentAccess: &allowed}}
	getRequest := httptest.NewRequest(http.MethodGet, "/api/email-accounts", nil)
	getResponse := httptest.NewRecorder()
	handleEmailAccounts(s).ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("GET status = %d", getResponse.Code)
	}
	var accounts []emailAccountJSON
	if err := json.Unmarshal(getResponse.Body.Bytes(), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].SMTPAuthMethod == nil || *accounts[0].SMTPAuthMethod != "plain" {
		t.Fatalf("legacy auth projection = %#v, want plain", accounts)
	}
	accounts[0].SMTPAuthMethod = emailStringPointer("password")
	accounts[0].AgentAccess = nil // Old clients omit the field; preserve the saved grant.
	body, err := json.Marshal(accounts)
	if err != nil {
		t.Fatal(err)
	}
	putRequest := httptest.NewRequest(http.MethodPut, "/api/email-accounts", strings.NewReader(string(body)))
	putResponse := httptest.NewRecorder()
	handleEmailAccounts(s).ServeHTTP(putResponse, putRequest)
	if putResponse.Code != http.StatusOK {
		t.Fatalf("PUT legacy auth status = %d; body %s", putResponse.Code, putResponse.Body.String())
	}
	account := s.ConfigSnapshot().FindEmailAccount("legacy-auth")
	if account == nil || account.SMTPAuthMethod != "plain" || !account.AllowsAgentAccess() {
		t.Fatalf("legacy account changed unexpectedly: %#v", account)
	}
}
