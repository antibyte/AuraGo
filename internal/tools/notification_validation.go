package tools

import (
	"fmt"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/cyd"
	"aurago/internal/push"
)

// Validate locally before reserving a receipt; helpers retain their own guards
// for callers outside SendNotificationContext and for runtime availability races.
func validateNotification(cfg *config.Config, channel NotificationChannel, title, message string, discordSend DiscordSendFunc, telnyxSend ...TelnyxSendFunc) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	switch channel {
	case ChannelNtfy:
		if !cfg.Notifications.Ntfy.Enabled || cfg.Notifications.Ntfy.Topic == "" {
			return fmt.Errorf("ntfy must be enabled with a configured topic")
		}
	case ChannelPushover:
		if !cfg.Notifications.Pushover.Enabled || cfg.Notifications.Pushover.UserKey == "" || cfg.Notifications.Pushover.AppToken == "" {
			return fmt.Errorf("pushover must be enabled with user_key and app_token")
		}
	case ChannelTelegram:
		if cfg.Telegram.ReadOnly {
			return fmt.Errorf("telegram is in read-only mode")
		}
		if _, _, err := telegramDocumentTarget(cfg); err != nil {
			return err
		}
		if n := utf16Units(title) + 1 + utf16Units(message); n > telegramMaxMessageUnits {
			return fmt.Errorf("telegram message is too long: %d UTF-16 units, the limit is %d", n, telegramMaxMessageUnits)
		}
	case ChannelWhatsApp:
		if !cfg.WhatsApp.Enabled {
			return errWhatsAppDisabled
		}
		if cfg.WhatsApp.ReadOnly {
			return errWhatsAppReadOnly
		}
		sender := currentWhatsAppOwnerSender()
		if sender == nil || !sender.Ready() {
			return errWhatsAppNotRunning
		}
		if n := utf8.RuneCountInString(whatsAppNotificationTitle(title)) + 1 + utf8.RuneCountInString(message); n > WhatsAppMaxMessageRunes {
			return fmt.Errorf("whatsapp message is too long: %d characters, the limit is %d - shorten the message", n, WhatsAppMaxMessageRunes)
		}
	case ChannelDiscord:
		if cfg.Discord.ReadOnly {
			return fmt.Errorf("discord is in read-only mode")
		}
		if !cfg.Discord.Enabled || cfg.Discord.DefaultChannelID == "" || discordSend == nil {
			return fmt.Errorf("discord must be enabled with a default channel and available sender")
		}
	case ChannelTelnyx:
		if cfg.Telnyx.ReadOnly {
			return fmt.Errorf("telnyx is in read-only mode")
		}
		if !cfg.Telnyx.Enabled || len(cfg.Telnyx.AllowedNumbers) == 0 || len(telnyxSend) == 0 || telnyxSend[0] == nil {
			return fmt.Errorf("telnyx must be enabled with allowed numbers and an available sender")
		}
	case ChannelPush:
		if push.GlobalManager == nil {
			return fmt.Errorf("web push manager is not initialized")
		}
		if push.GlobalManager.CountSubscriptions() == 0 {
			return fmt.Errorf("no active web push subscriptions found")
		}
	case ChannelCYD:
		if !cfg.Cyd.Enabled {
			return fmt.Errorf("cyd is not enabled in config")
		}
		if cyd.Global() == nil {
			return fmt.Errorf("cyd hub is not started")
		}
	default:
		return fmt.Errorf("unknown channel")
	}
	return nil
}
