package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGenericVaultMutationsCannotChangeAuthenticationSecrets(t *testing.T) {
	for _, key := range []string{"auth_password_hash", "auth.password_hash", "auth.session_secret", "auth.totp_secret", "AUTH_SESSION_SECRET", "session_cookie_key", "master_key", "internal_token"} {
		t.Run(key, func(t *testing.T) {
			// A nil Vault deliberately catches any access beyond the security gate.
			s := &Server{}
			post := httptest.NewRecorder()
			handleSetVaultSecret(s, post, httptest.NewRequest(http.MethodPost, "/api/vault/secrets", strings.NewReader(`{"key":"`+key+`","value":"fixture"}`)))
			if post.Code != http.StatusForbidden {
				t.Fatalf("set reserved key: status=%d", post.Code)
			}
			remove := httptest.NewRecorder()
			handleDeleteVaultSecret(s, remove, httptest.NewRequest(http.MethodDelete, "/api/vault/secrets?key="+url.QueryEscape(key), nil))
			if remove.Code != http.StatusForbidden {
				t.Fatalf("delete reserved key: status=%d", remove.Code)
			}
		})
	}
}
