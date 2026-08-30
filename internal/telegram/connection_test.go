package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckBotTokenAtUsesReadOnlyGetMe(t *testing.T) {
	const token = "123456:telegram-test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/getMe") || !strings.Contains(r.URL.Path, "telegram-test-token") {
			t.Fatalf("path = %q, want bot token getMe path", r.URL.Path)
		}
		fmt.Fprint(w, `{"ok":true,"result":{"id":123,"is_bot":true,"username":"aurago_test"}}`)
	}))
	defer server.Close()

	if err := checkBotTokenAt(context.Background(), token, server.URL); err != nil {
		t.Fatalf("checkBotTokenAt() error = %v", err)
	}
}

func TestCheckBotTokenAtRejectsTelegramError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"ok":false,"description":"Unauthorized"}`)
	}))
	defer server.Close()

	err := checkBotTokenAt(context.Background(), "123456:telegram-test-token", server.URL)
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("checkBotTokenAt() error = %v, want HTTP 401", err)
	}
}
