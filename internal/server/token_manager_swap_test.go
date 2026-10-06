package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/security"
)

func TestReplaceTokenManagerRetiresPreviousStore(t *testing.T) {
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	oldPath := filepath.Join(dir, "old-tokens.bin")
	oldTM, err := security.NewTokenManager(vault, oldPath)
	if err != nil {
		t.Fatalf("NewTokenManager(old): %v", err)
	}
	if _, _, err := oldTM.Create("old", []string{"webhook"}, nil); err != nil {
		t.Fatalf("Create(old): %v", err)
	}
	before, _ := os.ReadFile(oldPath)
	newTM, err := security.NewTokenManager(vault, filepath.Join(dir, "new-tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager(new): %v", err)
	}

	s := &Server{TokenManager: oldTM}
	s.replaceTokenManager(newTM)

	if s.currentTokenManager() != newTM {
		t.Fatal("currentTokenManager did not return the replacement")
	}
	if _, _, err := oldTM.Create("late", []string{"webhook"}, nil); !errors.Is(err, security.ErrTokenStoreUnavailable) {
		t.Fatalf("retired manager Create error = %v, want ErrTokenStoreUnavailable", err)
	}
	if after, _ := os.ReadFile(oldPath); !bytes.Equal(before, after) {
		t.Fatal("retired manager rewrote its token file")
	}
}

func TestTokenAdminReportsReadOnlyStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.bin")
	owner, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(dir, "vault-e.bin"))
	if err != nil {
		t.Fatalf("NewVault(owner): %v", err)
	}
	tm, err := security.NewTokenManager(owner, path)
	if err != nil {
		t.Fatalf("NewTokenManager(owner): %v", err)
	}
	if _, _, err := tm.Create("existing", []string{"webhook"}, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	wrongKey, err := security.NewVault(strings.Repeat("f", 64), filepath.Join(dir, "vault-f.bin"))
	if err != nil {
		t.Fatalf("NewVault(wrong): %v", err)
	}
	degraded, err := security.NewTokenManager(wrongKey, path)
	if !errors.Is(err, security.ErrTokenStoreUnavailable) {
		t.Fatalf("NewTokenManager(wrong key) error = %v, want ErrTokenStoreUnavailable", err)
	}

	rec := httptest.NewRecorder()
	handleCreateToken(degraded).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(`{"name":"new"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("create on read-only store status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handleDeleteToken(tm).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/tokens/no-such-id", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown token status = %d, want 404", rec.Code)
	}
}

func TestCreateTokenValidatesScopes(t *testing.T) {
	dir := t.TempDir()
	vault, err := security.NewVault(strings.Repeat("e", 64), filepath.Join(dir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault: %v", err)
	}
	tm, err := security.NewTokenManager(vault, filepath.Join(dir, "tokens.bin"))
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}

	rejected := []struct {
		body    string
		wantMsg string
	}{
		{`{"name":"x","scopes":["cyd","admin"]}`, "cyd scope cannot be combined"},
		{`{"name":"x","scopes":["webhook","cyd"]}`, "cyd scope cannot be combined"},
		{`{"name":"x","scopes":["cyd","cyd"]}`, "cyd scope cannot be combined"},
		{`{"name":"x","scopes":["root"]}`, `unknown token scope "root"`},
		{`{"name":"x","scopes":["read","write"]}`, `unknown token scope "read"`},
		{`{"name":"x","scopes":["Admin"]}`, `unknown token scope "Admin"`},
		{`{"name":"x","scopes":[" admin "]}`, `unknown token scope " admin "`},
		{`{"name":"x","scopes":["webhook",""]}`, `unknown token scope ""`},
		{`{"name":"x","scopes":["desktop:remote:device:"]}`, `unknown token scope "desktop:remote:device:"`},
		{`{"name":"x","scopes":["desktop:remote:tag: lab"]}`, `unknown token scope "desktop:remote:tag: lab"`},
	}
	for _, tc := range rejected {
		t.Run("reject "+tc.body, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handleCreateToken(tm).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(tc.body)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			var resp map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if !strings.Contains(resp["error"], tc.wantMsg) {
				t.Fatalf("error = %q, want it to contain %q", resp["error"], tc.wantMsg)
			}
		})
	}
	if n := tm.Count(); n != 0 {
		t.Fatalf("rejected requests stored %d tokens", n)
	}

	accepted := []struct {
		body       string
		wantScopes []string
		wantShort  bool
	}{
		{`{"name":"x","scopes":["webhook"]}`, []string{"webhook"}, false},
		{`{"name":"x"}`, []string{"webhook"}, false},
		{`{"name":"x","scopes":[]}`, []string{"webhook"}, false},
		{`{"name":"x","scopes":["cyd"]}`, []string{"cyd"}, true},
		{`{"name":"x","scopes":["admin","go2rtc.view","desktop:read","desktop:write","desktop:admin","desktop:remote"]}`,
			[]string{"admin", "go2rtc.view", "desktop:read", "desktop:write", "desktop:admin", "desktop:remote"}, false},
		{`{"name":"x","scopes":["desktop:remote:device:dev-1","desktop:remote:tag:Lab"]}`,
			[]string{"desktop:remote:device:dev-1", "desktop:remote:tag:Lab"}, false},
	}
	for _, tc := range accepted {
		t.Run("accept "+tc.body, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handleCreateToken(tm).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(tc.body)))
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
			}
			var resp struct {
				Token   string             `json:"token"`
				Display string             `json:"display"`
				Meta    security.TokenMeta `json:"meta"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if strings.Join(resp.Meta.Scopes, ",") != strings.Join(tc.wantScopes, ",") {
				t.Fatalf("scopes = %q, want %q", resp.Meta.Scopes, tc.wantScopes)
			}
			if short := len(resp.Token) == len("aura_")+security.CYDTokenBodyLen; short != tc.wantShort {
				t.Fatalf("token length %d, short = %v, want %v", len(resp.Token), short, tc.wantShort)
			}
			if tc.wantShort && (resp.Display == "" || resp.Meta.Prefix != "aura_...") {
				t.Fatalf("display = %q, prefix = %q", resp.Display, resp.Meta.Prefix)
			}
			for _, scope := range tc.wantScopes {
				if _, ok := tm.Validate(resp.Token, scope); !ok {
					t.Fatalf("created token does not validate for %q", scope)
				}
			}
		})
	}

	// The allowlist is creation-time only: a stored token whose scope can no
	// longer be created through the API keeps validating.
	legacy, _, err := tm.Create("legacy", []string{"read"}, nil)
	if err != nil {
		t.Fatalf("Create(legacy): %v", err)
	}
	if _, ok := tm.Validate(legacy, "read"); !ok {
		t.Fatal("stored token with a non-allowlisted scope stopped validating")
	}
}
