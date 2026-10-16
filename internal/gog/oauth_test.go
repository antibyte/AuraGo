package gog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
)

type tokenRoundTrip func(*http.Request) (*http.Response, error)

func (f tokenRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGogAccountTokenCannotOverwriteAnotherClientSecret(t *testing.T) {
	s, _ := fixtureService(t)
	const secret = "fixture-isolated-client-secret"
	if err := s.Vault.WriteSecret(ClientKey("primary"), secret); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginLogin("client_primary", "fixture-state", "other-client"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Vault.ReadSecret(ClientKey("primary")); err != nil || got != secret {
		t.Fatal("another account's login overwrote the client secret")
	}
	if err := s.Disconnect("client_primary"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Vault.ReadSecret(ClientKey("primary")); err != nil || got != secret {
		t.Fatal("another account's disconnect removed the client secret")
	}
}

func TestGogRefreshSerializesAndDisconnectWins(t *testing.T) {
	for _, disconnect := range []bool{false, true} {
		t.Run(map[bool]string{true: "disconnect", false: "serialized"}[disconnect], func(t *testing.T) {
			s, c := fixtureService(t)
			tok, _, _ := s.token("primary")
			tok.Expiry = time.Now().Add(-time.Minute)
			tok.RefreshToken = "fixture-refresh"
			raw, _ := json.Marshal(tok)
			if e := s.Vault.WriteSecret(TokenKey("primary"), string(raw)); e != nil {
				t.Fatal(e)
			}
			if e := s.Vault.WriteSecret(ClientKey("primary"), "fixture-client-secret"); e != nil {
				t.Fatal(e)
			}
			var calls atomic.Int32
			entered := make(chan struct{})
			release := make(chan struct{})
			s.tokenClient = &http.Client{Transport: tokenRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != TokenURL || r.Method != "POST" {
					t.Error("wrong token endpoint")
				}
				calls.Add(1)
				close(entered)
				<-release
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"fixture-new-access","expires_in":3600}`)), Header: http.Header{}}, nil
			})}
			var wg sync.WaitGroup
			errs := make(chan error, 8)
			for range 8 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, e := s.accessToken(context.Background(), c.Gog.Accounts[0], Operation{Service: "gmail", Grant: "read"})
					errs <- e
				}()
			}
			<-entered
			if disconnect {
				if e := s.Disconnect("primary"); e != nil {
					t.Fatal(e)
				}
			}
			close(release)
			wg.Wait()
			close(errs)
			if calls.Load() != 1 {
				t.Fatalf("refresh calls=%d", calls.Load())
			}
			for e := range errs {
				if (e != nil) != disconnect {
					t.Fatalf("disconnect=%v error=%v", disconnect, e)
				}
			}
			if s.Connected("primary") == disconnect {
				t.Fatal("wrong final token state")
			}
		})
	}
}

func TestGogScopesForEveryService(t *testing.T) {
	names := map[string]string{"gmail": "gmail", "drive": "drive", "docs": "documents", "sheets": "spreadsheets", "slides": "presentations", "calendar": "calendar", "tasks": "tasks", "contacts": "contacts"}
	for service, name := range names {
		tok := Token{Scopes: []string{"https://www.googleapis.com/auth/" + name + ".readonly"}}
		if !tokenHasScope(tok, Operation{Service: service, Grant: "read"}) || tokenHasScope(tok, Operation{Service: service, Grant: "create"}) {
			t.Fatalf("read scope grants write: %s", service)
		}
		if service == "gmail" {
			name = "gmail.modify"
		}
		tok.Scopes = []string{"https://www.googleapis.com/auth/" + name}
		if !tokenHasScope(tok, Operation{Service: service, Grant: "read"}) || !tokenHasScope(tok, Operation{Service: service, Grant: "update"}) {
			t.Fatalf("full scope rejected: %s", service)
		}
	}
}

func TestGmailMailAccessTokenUsesOperationGrantAndActualScope(t *testing.T) {
	tests := []struct {
		name, grant, wantErr string
		grants               config.GogGrants
		scopes               []string
	}{
		{name: "modify maps to update", grant: "modify", grants: config.GogGrants{Update: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		{name: "modify scope sends", grant: "send", grants: config.GogGrants{Send: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		{name: "compose scope sends", grant: "send", grants: config.GogGrants{Send: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.compose"}},
		{name: "labels scope creates label", grant: "create", grants: config.GogGrants{Create: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.labels"}},
		{name: "modify scope creates label", grant: "create", grants: config.GogGrants{Create: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		{name: "compose cannot create label", grant: "create", wantErr: "missing_scope", grants: config.GogGrants{Create: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.compose"}},
		{name: "existing scope cannot bypass create grant", grant: "create", wantErr: "permission_denied", grants: config.GogGrants{Update: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		{name: "delete grant still needs full mail for Mailer permanent delete", grant: "delete", wantErr: "missing_scope", grants: config.GogGrants{Delete: true}, scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}},
		{name: "full mail authorizes Mailer permanent delete", grant: "delete", grants: config.GogGrants{Delete: true}, scopes: []string{"https://mail.google.com/"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, cfg := fixtureService(t)
			cfg.Gog.Accounts[0].Services["gmail"] = tc.grants
			tok, _, err := s.token("primary")
			if err != nil {
				t.Fatal(err)
			}
			tok.Scopes = append([]string(nil), tc.scopes...)
			raw, err := json.Marshal(tok)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Vault.WriteSecret(TokenKey("primary"), string(raw)); err != nil {
				t.Fatal(err)
			}
			got, err := s.MailAccessToken(context.Background(), "primary", tc.grant)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("error=%v, want %q", err, tc.wantErr)
				}
				if got != "" {
					t.Fatal("denied operation returned an access token")
				}
				return
			}
			if err != nil || got != tok.AccessToken {
				t.Fatalf("token=%q err=%v", got, err)
			}
		})
	}
}

func TestGmailCreateScopeRequestsOnlyGrantedLabelCapability(t *testing.T) {
	scopes := Scopes(config.GogAccount{Services: map[string]config.GogGrants{"gmail": {Create: true, Send: true}}}, false)
	joined := strings.Join(scopes, " ")
	for _, required := range []string{"https://www.googleapis.com/auth/gmail.compose", "https://www.googleapis.com/auth/gmail.labels"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("requested scopes %q omit %q", joined, required)
		}
	}
	for _, forbidden := range []string{"gmail.modify", "gmail.send", "mail.google.com"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("create/send-only scopes unexpectedly include broader %q: %q", forbidden, joined)
		}
	}
}

func TestNativeGmailDraftAndDeleteKeepExistingScopes(t *testing.T) {
	compose := Token{Scopes: []string{"https://www.googleapis.com/auth/gmail.compose"}}
	if !tokenHasScope(compose, Operation{Service: "gmail", Grant: "create"}) {
		t.Fatal("native create_draft no longer accepts gmail.compose")
	}
	modify := Token{Scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}}
	if !tokenHasScope(modify, Operation{Service: "gmail", Grant: "delete"}) {
		t.Fatal("native draft/label/trash delete no longer accepts gmail.modify")
	}
}

func TestMailerGmailScopeCapabilitiesAreReadOnlyAndPrecise(t *testing.T) {
	tests := []struct {
		name                                      string
		scopes                                    []string
		read, send, modify, label, hard, protocol bool
	}{
		{name: "modify", scopes: []string{"https://www.googleapis.com/auth/gmail.modify"}, read: true, send: true, modify: true, label: true},
		{name: "compose", scopes: []string{"https://www.googleapis.com/auth/gmail.compose"}, send: true},
		{name: "labels", scopes: []string{"https://www.googleapis.com/auth/gmail.labels"}, label: true},
		{name: "full mail", scopes: []string{"https://mail.google.com/"}, read: true, send: true, modify: true, label: true, hard: true, protocol: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := fixtureService(t)
			token, _, err := s.token("primary")
			if err != nil {
				t.Fatal(err)
			}
			token.Scopes = tc.scopes
			raw, _ := json.Marshal(token)
			if err := s.Vault.WriteSecret(TokenKey("primary"), string(raw)); err != nil {
				t.Fatal(err)
			}
			got := s.MailerGmailScopeCapabilities("primary")
			if got.Read != tc.read || got.Send != tc.send || got.Modify != tc.modify || got.CreateLabel != tc.label || got.PermanentDelete != tc.hard || got.Protocol != tc.protocol {
				t.Fatalf("scope capabilities=%+v, want read=%v send=%v modify=%v label=%v permanent-delete=%v protocol=%v", got, tc.read, tc.send, tc.modify, tc.label, tc.hard, tc.protocol)
			}
		})
	}
}

func TestGogFirstLoginUsesCASMarkerAndDisconnectWins(t *testing.T) {
	s, cfg := fixtureService(t)
	if err := s.Vault.DeleteSecret(TokenKey("primary")); err != nil {
		t.Fatal(err)
	}
	if err := s.Vault.WriteSecret(ClientKey("primary"), "fixture-client-secret"); err != nil {
		t.Fatal(err)
	}
	previous, err := s.BeginLogin("primary", strings.Repeat("a", 64), "fixture-client")
	if err != nil || previous == "" {
		t.Fatalf("first login did not reserve a marker: previous=%q err=%v", previous, err)
	}
	var marker Token
	if json.Unmarshal([]byte(previous), &marker) != nil || marker.PendingLogin == "" || marker.AccessToken != "" {
		t.Fatalf("first-login marker contains an unexpected token: %#v", marker)
	}
	requested := Scopes(cfg.Gog.Accounts[0], cfg.Gog.ReadOnly)
	s.tokenClient = &http.Client{Transport: tokenRoundTrip(func(*http.Request) (*http.Response, error) {
		body, _ := json.Marshal(map[string]any{"access_token": "fixture-first-access", "refresh_token": "fixture-first-refresh", "expires_in": 3600, "scope": strings.Join(requested, " ")})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})}
	// A revocation while the exchange is in flight removes the exact marker.
	if err := s.Disconnect("primary"); err != nil {
		t.Fatal(err)
	}
	err = s.Exchange(context.Background(), cfg.Gog.Accounts[0], url.Values{"grant_type": {"authorization_code"}, "code": {"fixture-code"}}, previous, requested)
	if err == nil || err.Error() != "secret changed during operation" {
		t.Fatalf("late first-login completion survived disconnect: %v", err)
	}
	if _, err := s.Vault.ReadSecret(TokenKey("primary")); err == nil {
		t.Fatal("revoked first login recreated the token")
	}
}

func TestGogParallelFirstLoginCompletionHasSingleCASWinner(t *testing.T) {
	s, cfg := fixtureService(t)
	if err := s.Vault.DeleteSecret(TokenKey("primary")); err != nil {
		t.Fatal(err)
	}
	if err := s.Vault.WriteSecret(ClientKey("primary"), "fixture-client-secret"); err != nil {
		t.Fatal(err)
	}
	previous, err := s.BeginLogin("primary", strings.Repeat("b", 64), "fixture-client")
	if err != nil {
		t.Fatal(err)
	}
	requested := Scopes(cfg.Gog.Accounts[0], cfg.Gog.ReadOnly)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	s.tokenClient = &http.Client{Transport: tokenRoundTrip(func(*http.Request) (*http.Response, error) {
		call := calls.Add(1)
		entered <- struct{}{}
		<-release
		body, _ := json.Marshal(map[string]any{"access_token": "fixture-parallel-access-" + string(rune('0'+call)), "refresh_token": "fixture-parallel-refresh", "expires_in": 3600, "scope": strings.Join(requested, " ")})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})}
	results := make(chan error, 2)
	for range 2 {
		go func() {
			results <- s.Exchange(context.Background(), cfg.Gog.Accounts[0], url.Values{"grant_type": {"authorization_code"}, "code": {"fixture-code"}}, previous, requested)
		}()
	}
	<-entered
	<-entered
	close(release)
	errA, errB := <-results, <-results
	if (errA == nil) == (errB == nil) {
		t.Fatalf("expected one CAS winner and one loser, errors=%v / %v", errA, errB)
	}
	if calls.Load() != 2 {
		t.Fatalf("token endpoint calls=%d, want 2 concurrent completions", calls.Load())
	}
	stored, _, err := s.token("primary")
	if err != nil || !strings.HasPrefix(stored.AccessToken, "fixture-parallel-access-") {
		t.Fatalf("winning first-login token not stored: %#v err=%v", stored, err)
	}
}

func TestMailerProtocolTokenRequiresActualFullMailScopeAndCurrentGrant(t *testing.T) {
	s, cfg := fixtureService(t)
	if _, err := s.MailProtocolToken(context.Background(), "primary", "imap"); err == nil || err.Error() != "missing_scope" {
		t.Fatalf("Gmail API scopes authorized IMAP: %v", err)
	}
	tok, _, err := s.token("primary")
	if err != nil {
		t.Fatal(err)
	}
	tok.Scopes = append(tok.Scopes, "https://mail.google.com/")
	raw, _ := json.Marshal(tok)
	if err := s.Vault.WriteSecret(TokenKey("primary"), string(raw)); err != nil {
		t.Fatal(err)
	}
	if got, err := s.MailProtocolToken(context.Background(), "primary", "imap"); err != nil || got != tok.AccessToken {
		t.Fatalf("full protocol scope was rejected: token=%q err=%v", got, err)
	}
	cfg.Gog.Accounts[0].Services["gmail"] = config.GogGrants{Read: true, Send: false}
	if _, err := s.MailProtocolToken(context.Background(), "primary", "smtp"); err == nil || err.Error() != "permission_denied" {
		t.Fatalf("SMTP ignored the current GoG send grant: %v", err)
	}
}

func TestGogEnvironmentAndAccountIsolation(t *testing.T) {
	s, _ := fixtureService(t)
	called := false
	s.RunProcess = func(_ context.Context, _ string, args []string, dir string, env []string) (string, int, bool, error) {
		called = true
		if len(env) != 2 || !strings.HasPrefix(env[0], "GOG_ACCESS_TOKEN=") || !strings.HasPrefix(env[1], "GOG_HOME="+dir) {
			t.Fatalf("unexpected environment keys")
		}
		for _, a := range args {
			if strings.Contains(a, "fixture-gog-access") {
				t.Error("token in argv")
			}
		}
		return `{"text":"fixture-gog-access"}`, 0, true, nil
	}
	r := s.Execute(context.Background(), "agent", Request{AccountID: "primary", Service: "gmail", Operation: "labels"}, nil)
	if !called || r.Status != "ok" || strings.Contains(string(r.Data), "fixture-gog-access") {
		t.Fatal("credential isolation failed")
	}
	called = false
	s.Execute(context.Background(), "agent", Request{AccountID: "missing", Service: "gmail", Operation: "labels"}, nil)
	if called {
		t.Fatal("cross-account execution")
	}
}
