package server

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCodeStudioPreviewTokenSignVerifyExpiryTamper(t *testing.T) {
	key, err := codeStudioPreviewKey("secret-a")
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	now := time.Unix(1_800_000_000, 0)
	token, expires, err := issueCodeStudioPreviewToken(key, "local", now)
	if err != nil || !expires.Equal(now.Add(12*time.Hour)) {
		t.Fatalf("issue = %q, %v, %v; want a token expiring in 12 h", token, expires, err)
	}
	claims, ok := parseCodeStudioPreviewToken(key, token, now.Add(time.Hour))
	if !ok || claims.Workspace != "local" || claims.Version != 1 {
		t.Fatalf("parse = %+v, %v; want local v1", claims, ok)
	}
	if _, ok := parseCodeStudioPreviewToken(key, token, now.Add(12*time.Hour)); ok {
		t.Fatal("token is still valid at its expiry time")
	}
	other, _ := codeStudioPreviewKey("secret-b")
	if _, ok := parseCodeStudioPreviewToken(other, token, now); ok {
		t.Fatal("token verified with a different session secret")
	}
	payload, signature, _ := strings.Cut(token, ".")
	forged, _ := json.Marshal(codeStudioPreviewClaims{Version: 1, Workspace: "ssh-other", Expires: now.Add(24 * time.Hour).Unix()})
	if _, ok := parseCodeStudioPreviewToken(key, base64.RawURLEncoding.EncodeToString(forged)+"."+signature, now); ok {
		t.Fatal("a payload with a foreign workspace kept the old signature and verified")
	}
	for _, bad := range []string{"", ".", payload, payload + ".", token + ".x", strings.Repeat("a", 600)} {
		if _, ok := parseCodeStudioPreviewToken(key, bad, now); ok {
			t.Fatalf("malformed token %q verified", bad)
		}
	}
	if _, _, err := issueCodeStudioPreviewToken(key, "../evil", now); err == nil {
		t.Fatal("a workspace id outside [A-Za-z0-9_-] was accepted")
	}
	// The key is derived with a purpose label: a desktop embed token signed with the session secret never verifies.
	embed, err := issueDesktopEmbedTokenForPath("secret-a", "Apps/x/index.html", now)
	if err != nil {
		t.Fatalf("embed token: %v", err)
	}
	if _, ok := parseCodeStudioPreviewToken(key, embed, now); ok {
		t.Fatal("a desktop embed token was accepted as a preview token")
	}
}

func TestCodeStudioPreviewKeyWithoutSessionSecret(t *testing.T) {
	first, err := codeStudioPreviewKey("")
	if err != nil || len(first) == 0 {
		t.Fatalf("process key = %x, %v; want a random key", first, err)
	}
	second, _ := codeStudioPreviewKey("  ")
	if string(first) != string(second) {
		t.Fatal("the per-process key changed between calls")
	}
	derived, _ := codeStudioPreviewKey("x")
	if string(derived) == string(first) || string(derived) == "x" {
		t.Fatal("the session-secret key must be derived and differ from the process key")
	}
}
