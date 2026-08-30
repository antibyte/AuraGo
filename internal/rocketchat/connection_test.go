package rocketchat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestCheckConnectionUsesReadOnlyMeEndpoint(t *testing.T) {
	const token = "rocketchat-test-token"
	const userID = "rocketchat-user"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/me" {
			t.Fatalf("request = %s %s, want GET /api/v1/me", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Auth-Token"); got != token {
			t.Fatalf("X-Auth-Token = %q, want configured token", got)
		}
		if got := r.Header.Get("X-User-Id"); got != userID {
			t.Fatalf("X-User-Id = %q, want configured user ID", got)
		}
		fmt.Fprint(w, `{"success":true,"user":{"_id":"rocketchat-user"}}`)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.RocketChat.URL = server.URL
	cfg.RocketChat.UserID = userID
	cfg.RocketChat.AuthToken = token
	if err := CheckConnection(context.Background(), cfg); err != nil {
		t.Fatalf("CheckConnection() error = %v", err)
	}
}

func TestCheckConnectionRejectsRocketChatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := &config.Config{}
	cfg.RocketChat.URL = server.URL
	cfg.RocketChat.UserID = "rocketchat-user"
	cfg.RocketChat.AuthToken = "rocketchat-test-token"
	if err := CheckConnection(context.Background(), cfg); err == nil {
		t.Fatal("CheckConnection() error = nil, want unauthorized error")
	}
}
