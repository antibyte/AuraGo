package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/security"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// photoSendTimeout bounds one SendPhoto call: the getMe handshake of the short-lived
	// client and the upload.
	photoSendTimeout = 30 * time.Second
	// photoCaptionRunes is the Bot API limit of a photo caption.
	photoCaptionRunes = 1024
	// photoMaxBytes is the Bot API limit of an uploaded photo.
	photoMaxBytes = 10 << 20
)

// photoAPIEndpoint is the Bot API endpoint format of SendPhoto; tests point it to a local server.
var photoAPIEndpoint = tgbotapi.APIEndpoint

// SendPhoto sends jpeg as a native Telegram photo with a plain-text caption to the configured
// operator chat (telegram.user_id). Like SendTestMessage it builds a short-lived bot client
// from the configuration, so it works whether or not long polling runs, and it sends
// through sendTelegramPhotoFile, the sender of the agent's photos. The caption is cut to the
// Bot API's 1024 runes. Returned errors never carry the bot token.
func SendPhoto(ctx context.Context, cfg *config.Config, name string, jpeg []byte, caption string) error {
	if cfg == nil || strings.TrimSpace(cfg.Telegram.BotToken) == "" || cfg.Telegram.UserID == 0 {
		return errors.New("telegram is not configured")
	}
	if len(jpeg) == 0 || len(jpeg) > photoMaxBytes {
		return fmt.Errorf("a photo must have 1 byte to %d MB", photoMaxBytes>>20)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, photoSendTimeout)
	defer cancel()
	token := cfg.Telegram.BotToken
	security.RegisterSensitive(token)
	client := contextBoundHTTPClient{parent: ctx, client: &http.Client{Timeout: photoSendTimeout}}
	bot, err := tgbotapi.NewBotAPIWithClient(token, photoAPIEndpoint, client)
	if err != nil {
		return fmt.Errorf("telegram client: %s", redactBotToken(err.Error(), token))
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "photo.jpg"
	}
	if err := sendTelegramPhotoFile(bot, cfg.Telegram.UserID, tgbotapi.FileBytes{Name: name, Bytes: jpeg}, boundPhotoCaption(caption)); err != nil {
		return fmt.Errorf("telegram photo: %s", redactBotToken(err.Error(), token))
	}
	return nil
}

// boundPhotoCaption cuts caption to photoCaptionRunes runes, the ellipsis included.
func boundPhotoCaption(caption string) string {
	caption = strings.TrimSpace(caption)
	if utf8.RuneCountInString(caption) <= photoCaptionRunes {
		return caption
	}
	return string([]rune(caption)[:photoCaptionRunes-1]) + "…"
}

// redactBotToken removes the token from text; request URLs carry it in their path.
func redactBotToken(text, token string) string {
	if token == "" {
		return text
	}
	text = strings.ReplaceAll(text, token, "[redacted]")
	return strings.ReplaceAll(text, url.PathEscape(token), "[redacted]")
}
