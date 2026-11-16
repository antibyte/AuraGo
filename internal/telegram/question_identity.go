package telegram

import (
	"fmt"

	"aurago/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func telegramConversationIdentity(msg *tgbotapi.Message, cfg *config.Config) (string, bool) {
	owner := cfg != nil && cfg.Telegram.UserID != 0 && msg.From.ID == cfg.Telegram.UserID && msg.Chat.IsPrivate()
	if owner {
		return "default", true
	}
	return fmt.Sprintf("telegram:%d:%d", msg.Chat.ID, msg.From.ID), false
}
