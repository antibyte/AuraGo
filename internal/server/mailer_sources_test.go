package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/gog"
	"aurago/internal/mailer"
)

func TestMailerSourcesArePassiveAndPreserveExistingGrants(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	no := false
	s.Cfg.EmailAccounts = []config.EmailAccount{
		{ID: "old", Username: "same@example.test", Password: "fixture-password-not-real", SMTPPassword: "fixture-smtp-not-real", OAuthClientSecret: "fixture-client-not-real"},
		{ID: "new", Username: "same@example.test", ReceiveProtocol: "pop3", AgentAccess: &no},
	}
	s.Cfg.Gog.Enabled = true
	s.Cfg.Agent.AllowNetworkRequests = true
	s.Cfg.Gog.Accounts = []config.GogAccount{{ID: "google", Enabled: true, DesktopAccess: true, AgentAccess: false, Services: map[string]config.GogGrants{"gmail": {Read: true, Send: true}}}}
	s.Cfg.AgentMail.Enabled = true
	s.Cfg.AgentMail.InboxID = "configured@example.test"
	s.Cfg.AgentMail.APIKey = "fixture-agentmail-not-real"
	accounts, err := s.mailerSourceAccounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 4 {
		t.Fatalf("lost duplicate address/source: %d", len(accounts))
	}
	if !accounts[0].AgentAccess || accounts[1].AgentAccess || accounts[1].Protocol != "pop3" || accounts[1].ID != "email:new" {
		t.Fatalf("grant or identity drift: %+v", accounts[:2])
	}
	data, err := json.Marshal(accounts)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"fixture-password-not-real", "fixture-smtp-not-real", "fixture-client-not-real", "fixture-agentmail-not-real"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("source enumeration exposed a credential")
		}
	}
	google := accounts[2]
	for _, forbidden := range []string{"delete", "labels_write", "folders_create"} {
		for _, cap := range google.Capabilities {
			if cap == forbidden {
				t.Fatalf("GoG permissions widened: %s", cap)
			}
		}
	}
	if google.AgentAccess || google.Disabled {
		t.Fatal("desktop access incorrectly depends on agent access or shell")
	}
}

func TestMailerSourcesIntersectProviderScopesWithoutRefreshing(t *testing.T) {
	s := newMailerOAuthTestServer(t)
	s.Cfg.Gog.Enabled = true
	s.Cfg.Gog.Accounts = []config.GogAccount{{ID: "google", ClientID: "fixture-client", Enabled: true, DesktopAccess: true,
		Services: map[string]config.GogGrants{"gmail": {Read: true, Send: true, Update: true, Create: true, Delete: true}}}}
	for _, tc := range []struct {
		scope                  string
		wantDelete, wantCreate bool
		wantReason             string
	}{
		{"https://www.googleapis.com/auth/gmail.readonly", false, false, "missing_scope"},
		{"https://www.googleapis.com/auth/gmail.modify", false, true, "missing_scope"},
		{"https://mail.google.com/", true, true, ""},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			raw, err := json.Marshal(gog.Token{AccessToken: "fixture-access-not-real", ClientID: "fixture-client", Scopes: []string{tc.scope}})
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Vault.WriteUserSecret(gog.TokenKey("google"), string(raw), true); err != nil {
				t.Fatal(err)
			}
			a, err := s.mailerSource(context.Background(), "gog:google")
			if err != nil {
				t.Fatal(err)
			}
			has := func(capability string) bool {
				for _, cap := range a.Capabilities {
					if cap == capability {
						return true
					}
				}
				return false
			}
			if has("delete") != tc.wantDelete || has("folders_create") != tc.wantCreate || a.Reason != tc.wantReason || !a.ReauthorizeSupported || !has("read") {
				t.Fatalf("provider scope projection: %+v", a)
			}
		})
	}
}

type mailerBlockingAdapter struct {
	mailerHTTPAdapter
	started chan struct{}
}

func (a *mailerBlockingAdapter) Read(ctx context.Context, _, _ string) (mailer.Message, error) {
	close(a.started)
	<-ctx.Done()
	return mailer.Message{}, ctx.Err()
}

func TestMailerSourceRevocationCancelsActiveProtocolRead(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	s.Cfg.EmailAccounts = []config.EmailAccount{{ID: "main", Username: "mail@example.test"}}
	s.initConfigSnapshot()
	previous := s.ConfigSnapshot()
	account, err := s.mailerSource(context.Background(), "email:main")
	if err != nil {
		t.Fatal(err)
	}
	inner := &mailerBlockingAdapter{started: make(chan struct{})}
	adapter := &mailerGuardedAdapter{server: s, id: account.ID, identity: mailerSourceIdentity(s.Cfg, account), inner: inner}
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() { _, err := adapter.Read(ctx, "INBOX", "1"); done <- err }()
	<-inner.started
	s.CfgMu.Lock()
	next := s.Cfg.Clone()
	next.EmailAccounts[0].Disabled = true
	s.Cfg = next
	s.replaceConfigSnapshot(next)
	s.CfgMu.Unlock()
	if previous.EmailAccounts[0].Disabled || s.ConfigSnapshot() != next {
		t.Fatal("revocation must publish a detached config snapshot")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("revoked read: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("revocation left protocol operation running")
	}
}

func TestMailerGuardRejectsWriteWithoutSourceCapability(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	s.Cfg.Gog.Enabled = true
	s.Cfg.Agent.AllowNetworkRequests = true
	s.Cfg.Gog.Accounts = []config.GogAccount{{ID: "read", Enabled: true, DesktopAccess: true, Services: map[string]config.GogGrants{"gmail": {Read: true}}}}
	account, err := s.mailerSource(context.Background(), "gog:read")
	if err != nil {
		t.Fatal(err)
	}
	inner := &mailerHTTPAdapter{}
	adapter := &mailerGuardedAdapter{server: s, id: account.ID, identity: mailerSourceIdentity(s.Cfg, account), inner: inner}
	if err := adapter.Mutate(context.Background(), "INBOX", "1", mailer.Mutation{Kind: mailer.MutationTrash}); !errors.Is(err, mailer.ErrCapability) {
		t.Fatalf("permission bypass: %v", err)
	}
	if len(inner.mutations) != 0 {
		t.Fatal("denied mutation reached provider")
	}
}
