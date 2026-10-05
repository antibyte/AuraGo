package tools

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

type telegramUpload struct {
	path, chatID, caption, filename, content string
}

func fakeTelegramAPI(t *testing.T, status int) (*httptest.Server, <-chan telegramUpload) {
	t.Helper()
	uploads := make(chan telegramUpload, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		up := telegramUpload{path: r.URL.Path, chatID: r.FormValue("chat_id"), caption: r.FormValue("caption")}
		if f, hdr, err := r.FormFile("document"); err == nil {
			data, _ := io.ReadAll(f)
			up.filename, up.content = hdr.Filename, string(data)
		}
		uploads <- up
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	old := telegramAPIBase
	telegramAPIBase = srv.URL
	t.Cleanup(func() {
		telegramAPIBase = old
		srv.Close()
	})
	return srv, uploads
}

func telegramTestConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	workspace := t.TempDir()
	path := filepath.Join(workspace, "bericht.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4 test"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.Directories.WorkspaceDir = workspace
	cfg.Telegram.BotToken = "123:secret"
	cfg.Telegram.UserID = 4711
	return cfg, path
}

func TestSendTelegramDocumentUploadsMultipart(t *testing.T) {
	_, uploads := fakeTelegramAPI(t, http.StatusOK)
	cfg, path := telegramTestConfig(t)
	if err := SendTelegramDocument(context.Background(), cfg, path, "Dein Bericht"); err != nil {
		t.Fatalf("SendTelegramDocument: %v", err)
	}
	up := <-uploads
	if up.path != "/bot123:secret/sendDocument" || up.chatID != "4711" || up.caption != "Dein Bericht" ||
		up.filename != "bericht.pdf" || up.content != "%PDF-1.4 test" {
		t.Fatalf("upload = %+v", up)
	}
}

func TestSendTelegramDocumentErrorsHideTheToken(t *testing.T) {
	fakeTelegramAPI(t, http.StatusBadRequest)
	cfg, path := telegramTestConfig(t)
	err := SendTelegramDocument(context.Background(), cfg, path, "")
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "400") {
		t.Fatalf("error = %v", err)
	}
	cfg.Telegram.BotToken = ""
	if err := SendTelegramDocument(context.Background(), cfg, path, ""); err == nil {
		t.Fatal("a missing bot token must fail")
	}
}

func TestSendTelegramFileResolvesPathAndSplitsLongText(t *testing.T) {
	_, uploads := fakeTelegramAPI(t, http.StatusOK)
	cfg, _ := telegramTestConfig(t)
	out := SendTelegramFile(context.Background(), cfg, slog.Default(), "bericht.pdf", "Titel", "Kurzer Text")
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil || res["status"] != "success" || res["document"] != "bericht.pdf" {
		t.Fatalf("result = %s (%v)", out, err)
	}
	if up := <-uploads; up.caption != "Titel\nKurzer Text" {
		t.Fatalf("caption = %q", up.caption)
	}
	out = SendTelegramFile(context.Background(), cfg, slog.Default(), "../outside.pdf", "", "x")
	if !strings.Contains(out, `"status":"error"`) {
		t.Fatalf("an outside path must fail: %s", out)
	}
}
