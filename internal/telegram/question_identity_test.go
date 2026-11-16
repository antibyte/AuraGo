package telegram

import (
	"aurago/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"testing"
)

func TestTelegramConversationIdentityIsolatesGroupsAndOtherSenders(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telegram.UserID = 42
	for _, tc := range []struct {
		user, chat int64
		kind, want string
		owner      bool
	}{
		{42, 42, "private", "default", true},
		{43, 43, "private", "telegram:43:43", false},
		{42, -100, "group", "telegram:-100:42", false},
		{42, -101, "supergroup", "telegram:-101:42", false},
	} {
		msg := &tgbotapi.Message{From: &tgbotapi.User{ID: tc.user}, Chat: &tgbotapi.Chat{ID: tc.chat, Type: tc.kind}}
		if id, owner := telegramConversationIdentity(msg, cfg); id != tc.want || owner != tc.owner {
			t.Fatalf("identity = %s/%v", id, owner)
		}
	}
}
