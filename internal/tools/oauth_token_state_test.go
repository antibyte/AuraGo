package tools

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

type oauthFixtureTransport func(*http.Request) (*http.Response, error)

func (f oauthFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOAuthRotationSurvivesStaleConfigAndConcurrentClients(t *testing.T) {
	for _, provider := range []string{"onedrive", "google_workspace"} {
		t.Run(provider, func(t *testing.T) {
			v, err := security.NewVault(strings.Repeat("02", 32), filepath.Join(t.TempDir(), "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			key := "oauth_" + provider
			initial := `{"access_token":"fixture-old","refresh_token":"fixture-refresh-old","token_expiry":"2000-01-01T00:00:00Z"}`
			if err := v.WriteSecret(key, initial); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			client := &http.Client{Transport: oauthFixtureTransport(func(r *http.Request) (*http.Response, error) {
				calls.Add(1)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"fixture-new","refresh_token":"fixture-refresh-new","expires_in":3600}`)), Header: make(http.Header)}, nil
			})}
			oldOD, oldGW := odHTTPClient, gwHTTPClient
			defer func() { odHTTPClient, gwHTTPClient = oldOD, oldGW }()
			odHTTPClient, gwHTTPClient = client, client
			newRefresh := func() func() error {
				cfg := config.Config{}
				if provider == "onedrive" {
					c, err := NewOneDriveClient(cfg, v)
					if err != nil {
						t.Fatal(err)
					}
					return c.refreshIfNeeded
				}
				c, err := NewGWorkspaceClient(cfg, v)
				if err != nil {
					t.Fatal(err)
				}
				return c.refreshIfNeeded
			}
			refreshes := make([]func() error, 12)
			for i := range refreshes {
				refreshes[i] = newRefresh()
			}
			var wg sync.WaitGroup
			for _, refresh := range refreshes {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := refresh(); err != nil {
						t.Error(err)
					}
				}()
			}
			wg.Wait()
			if calls.Load() != 1 {
				t.Fatalf("refreshes=%d; wanted one", calls.Load())
			}
			if err := newRefresh()(); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 1 {
				t.Fatal("fresh client reused stale config")
			}
			raw, err := v.ReadSecret(key)
			if err != nil {
				t.Fatal(err)
			}
			var tok config.OAuthToken
			if err := json.Unmarshal([]byte(raw), &tok); err != nil {
				t.Fatal(err)
			}
			if tok.RefreshToken != "fixture-refresh-new" || strings.Contains(raw, "token_expiry") || tok.Expiry == "" {
				t.Fatal("rotation not atomically persisted in canonical format")
			}
		})
	}
}

func TestOAuthRotationCannotReviveRevokedCredentials(t *testing.T) {
	for _, provider := range []string{"onedrive", "google_workspace"} {
		t.Run(provider, func(t *testing.T) {
			v, err := security.NewVault(strings.Repeat("03", 32), filepath.Join(t.TempDir(), "vault.bin"))
			if err != nil {
				t.Fatal(err)
			}
			key := "oauth_" + provider
			tok := config.OAuthToken{AccessToken: "fixture-old", RefreshToken: "fixture-refresh", Expiry: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
			raw, _ := json.Marshal(tok)
			if err := v.WriteSecret(key, string(raw)); err != nil {
				t.Fatal(err)
			}
			client := &http.Client{Transport: oauthFixtureTransport(func(r *http.Request) (*http.Response, error) {
				if err := v.DeleteSecret(key); err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"fixture-rotated","refresh_token":"fixture-rotated-refresh","expires_in":3600}`))}, nil
			})}
			oldOD, oldGW := odHTTPClient, gwHTTPClient
			defer func() { odHTTPClient, gwHTTPClient = oldOD, oldGW }()
			odHTTPClient, gwHTTPClient = client, client
			var refresh func() error
			var current func() string
			if provider == "onedrive" {
				c, e := NewOneDriveClient(config.Config{}, v)
				if e != nil {
					t.Fatal(e)
				}
				refresh, current = c.refreshIfNeeded, c.tokenValue
			} else {
				c, e := NewGWorkspaceClient(config.Config{}, v)
				if e != nil {
					t.Fatal(e)
				}
				refresh, current = c.refreshIfNeeded, c.tokenValue
			}
			if err := refresh(); !errors.Is(err, security.ErrSecretChanged) {
				t.Fatalf("revoked rotation accepted: %v", err)
			}
			if current() != "fixture-old" {
				t.Fatal("failed persistence published replacement")
			}
			if _, err := v.ReadSecret(key); err == nil {
				t.Fatal("revoked token restored")
			}
			if err := refresh(); err == nil {
				t.Fatal("stale in-memory credentials used after revocation")
			}
		})
	}
}
