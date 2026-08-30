package discord

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckBotTokenAtUsesCurrentUserRESTEndpoint(t *testing.T) {
	const token = "discord-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/users/@me" {
			t.Fatalf("request = %s %s, want GET /users/@me", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bot "+token {
			t.Fatalf("Authorization = %q, want bot token", got)
		}
		fmt.Fprint(w, `{"id":"123","username":"aurago"}`)
	}))
	defer server.Close()

	if err := checkBotTokenAt(context.Background(), token, server.URL); err != nil {
		t.Fatalf("checkBotTokenAt() error = %v", err)
	}
}

func TestCheckBotTokenAtRejectsUnauthorizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	err := checkBotTokenAt(context.Background(), "discord-test-token", server.URL)
	if err == nil || err.Error() != "Discord API returned HTTP 401" {
		t.Fatalf("checkBotTokenAt() error = %v, want HTTP 401", err)
	}
}
