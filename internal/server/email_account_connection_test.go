package server

import (
	"testing"

	"aurago/internal/config"
)

func TestResolveEmailConnectionTestUsesSavedTransportAndSeparateSMTPCredentials(t *testing.T) {
	s := newMailerAccountTestServer(t)
	s.Cfg.EmailAccounts = []config.EmailAccount{{
		ID: "saved", IMAPHost: "imap.example.test", IMAPPort: 143, ReceiveTLS: "starttls",
		SMTPHost: "smtp.example.test", SMTPPort: 465, SMTPTransport: "tls",
		Username: "imap-user", SMTPUsername: "smtp-user",
	}}
	if err := s.Vault.WriteSecret("email_saved_password", "saved-imap-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.Vault.WriteSecret("email_saved_smtp_password", "saved-smtp-secret"); err != nil {
		t.Fatal(err)
	}
	maskedSMTP := maskedKey
	got := resolveEmailConnectionTest(s, emailAccountJSON{ID: "saved", Password: maskedKey, SMTPPassword: &maskedSMTP})
	if got.imapHost != "imap.example.test" || got.imapPort != 143 || got.receiveTLS != "starttls" || got.imapUsername != "imap-user" || got.imapPassword != "saved-imap-secret" {
		t.Fatalf("saved IMAP settings were not inherited: %#v", got)
	}
	if got.smtpHost != "smtp.example.test" || got.smtpPort != 465 || got.smtpTransport != "tls" || got.smtpUsername != "smtp-user" || got.smtpPassword != "saved-smtp-secret" {
		t.Fatalf("saved SMTP settings and separate credentials were not inherited: %#v", got)
	}
}

func TestResolveEmailConnectionTestDoesNotSendSavedSecretsToOverrides(t *testing.T) {
	s := newMailerAccountTestServer(t)
	s.Cfg.EmailAccounts = []config.EmailAccount{{
		ID: "saved", IMAPHost: "imap.example.test", IMAPPort: 993, ReceiveTLS: "tls",
		SMTPHost: "smtp.example.test", SMTPPort: 587, SMTPTransport: "starttls",
		Username: "imap-user", SMTPUsername: "smtp-user",
	}}
	if err := s.Vault.WriteSecret("email_saved_password", "saved-imap-secret"); err != nil {
		t.Fatal(err)
	}
	if err := s.Vault.WriteSecret("email_saved_smtp_password", "saved-smtp-secret"); err != nil {
		t.Fatal(err)
	}
	maskedSMTP := maskedKey
	got := resolveEmailConnectionTest(s, emailAccountJSON{
		ID: "saved", IMAPHost: "override-imap.example.test", IMAPPort: 993,
		SMTPHost: "override-smtp.example.test", SMTPPort: 587,
		Username: "imap-user", SMTPUsername: emailStringPointer("smtp-user"),
		Password: maskedKey, SMTPPassword: &maskedSMTP,
	})
	if got.imapPassword != "" || got.smtpPassword != "" {
		t.Fatalf("saved credentials escaped to overridden hosts: IMAP=%q SMTP=%q", got.imapPassword, got.smtpPassword)
	}
	if got.receiveTLS != "tls" || got.smtpTransport != "starttls" {
		t.Fatalf("omitted transport fields did not inherit saved modes: %#v", got)
	}

	got = resolveEmailConnectionTest(s, emailAccountJSON{
		ID: "saved", IMAPHost: "override-imap.example.test", SMTPHost: "override-smtp.example.test",
		Username: "override-imap-user", SMTPUsername: emailStringPointer("override-smtp-user"),
		Password: "explicit-imap-secret", SMTPPassword: emailStringPointer("explicit-smtp-secret"),
	})
	if got.imapPassword != "explicit-imap-secret" || got.smtpPassword != "explicit-smtp-secret" || got.smtpUsername != "override-smtp-user" {
		t.Fatalf("explicit override credentials were not retained: %#v", got)
	}
}
