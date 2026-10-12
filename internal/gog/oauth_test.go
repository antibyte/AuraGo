package gog

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
