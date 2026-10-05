package telegram

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Operator slash commands run only in the owner's private chat.
func TestTelegramChatAllowsOperatorOnlyInPrivateChats(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  *tgbotapi.Message
		want bool
	}{
		{"nil message", nil, false},
		{"nil chat", &tgbotapi.Message{}, false},
		{"private", &tgbotapi.Message{Chat: &tgbotapi.Chat{Type: "private"}}, true},
		{"group", &tgbotapi.Message{Chat: &tgbotapi.Chat{Type: "group"}}, false},
		{"supergroup", &tgbotapi.Message{Chat: &tgbotapi.Chat{Type: "supergroup"}}, false},
		{"channel", &tgbotapi.Message{Chat: &tgbotapi.Chat{Type: "channel"}}, false},
	} {
		if got := telegramChatAllowsOperator(tc.msg); got != tc.want {
			t.Errorf("%s: telegramChatAllowsOperator = %v, want %v", tc.name, got, tc.want)
		}
	}
}
