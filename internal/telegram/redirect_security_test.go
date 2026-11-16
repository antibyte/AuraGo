package telegram

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTelegramClientsNeverFollowRedirects(t *testing.T) {
	hits := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		io.WriteString(w, `{"ok":true,"result":{"id":1}}`)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer source.Close()
	for _, client := range []*http.Client{newTelegramHTTPClient(), telegramConnectionHTTPClient} {
		resp, err := client.Get(source.URL + "/botfixture-token/getMe")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			t.Errorf("status=%d", resp.StatusCode)
		}
	}
	if err := checkBotTokenAt(context.Background(), "fixture-secret-token", source.URL); err == nil {
		t.Error("redirect treated as valid token")
	}
	if _, err := downloadFile(context.Background(), source.URL+"/file/botfixture-token/media", slog.Default()); err == nil {
		t.Error("redirect treated as media")
	}
	if hits != 0 {
		t.Fatalf("redirect target received %d requests", hits)
	}
}
