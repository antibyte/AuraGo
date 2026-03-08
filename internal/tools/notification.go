package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
)

// NotificationChannel identifies a notification target.
type NotificationChannel string

const (
	ChannelNtfy     NotificationChannel = "ntfy"
	ChannelPushover NotificationChannel = "pushover"
	ChannelTelegram NotificationChannel = "telegram"
	ChannelDiscord  NotificationChannel = "discord"
	ChannelAll      NotificationChannel = "all"
)

// notifyHTTPClient is shared across notification calls with a bounded timeout.
var notifyHTTPClient = &http.Client{Timeout: 15 * time.Second}

// DiscordSendFunc is injected by the agent dispatch layer to avoid an import cycle.
// It sends a message to the given Discord channel.
type DiscordSendFunc func(channelID, content string) error

// SendNotification dispatches a notification to the specified channel(s).
// channel may be "ntfy", "pushover", "telegram", "discord", or "all".
// discordSend may be nil if Discord is not available.
func SendNotification(cfg *config.Config, logger *slog.Logger, channel, title, message, priority string, discordSend DiscordSendFunc) string {
	encode := func(r map[string]interface{}) string {
		b, _ := json.Marshal(r)
		return string(b)
	}

	if message == "" {
		return encode(map[string]interface{}{"status": "error", "message": "message is required"})
	}
	if title == "" {
		title = "AuraGo"
	}
	if priority == "" {
		priority = "normal"
	}

	ch := NotificationChannel(strings.ToLower(strings.TrimSpace(channel)))

	type result struct {
		Channel string `json:"channel"`
		Status  string `json:"status"`
		Detail  string `json:"detail,omitempty"`
	}

	var results []result

	send := func(c NotificationChannel) {
		var err error
		switch c {
		case ChannelNtfy:
			err = sendNtfy(cfg, title, message, priority)
		case ChannelPushover:
			err = sendPushover(cfg, title, message, priority)
		case ChannelTelegram:
			err = sendTelegramNotification(cfg, title, message)
		case ChannelDiscord:
			err = sendDiscordNotification(cfg, discordSend, title, message)
		default:
			results = append(results, result{Channel: string(c), Status: "error", Detail: "unknown channel"})
			return
		}
		if err != nil {
			logger.Warn("Notification failed", "channel", c, "error", err)
			results = append(results, result{Channel: string(c), Status: "error", Detail: err.Error()})
		} else {
			logger.Info("Notification sent", "channel", c, "title", title)
			results = append(results, result{Channel: string(c), Status: "sent"})
		}
	}

	if ch == ChannelAll {
		// Send to all enabled channels
		if cfg.Notifications.Ntfy.Enabled {
			send(ChannelNtfy)
		}
		if cfg.Notifications.Pushover.Enabled {
			send(ChannelPushover)
		}
		if cfg.Telegram.BotToken != "" && cfg.Telegram.UserID != 0 {
			send(ChannelTelegram)
		}
		if cfg.Discord.Enabled && cfg.Discord.DefaultChannelID != "" && discordSend != nil {
			send(ChannelDiscord)
		}
		if len(results) == 0 {
			return encode(map[string]interface{}{"status": "error", "message": "no notification channels are enabled"})
		}
	} else {
		send(ch)
	}

	return encode(map[string]interface{}{
		"status":  "success",
		"results": results,
	})
}

// ── ntfy.sh ─────────────────────────────────────────────────────────────────

func sendNtfy(cfg *config.Config, title, message, priority string) error {
	if !cfg.Notifications.Ntfy.Enabled {
		return fmt.Errorf("ntfy is not enabled in config")
	}

	baseURL := cfg.Notifications.Ntfy.URL
	if baseURL == "" {
		baseURL = "https://ntfy.sh"
	}
	topic := cfg.Notifications.Ntfy.Topic
	if topic == "" {
		return fmt.Errorf("ntfy topic is not configured")
	}

	url := strings.TrimRight(baseURL, "/") + "/" + topic

	req, err := http.NewRequest("POST", url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("failed to create ntfy request: %w", err)
	}
	req.Header.Set("Title", title)
	req.Header.Set("Tags", "robot")

	// Map priority to ntfy levels (1=min, 3=default, 5=max)
	switch strings.ToLower(priority) {
	case "low":
		req.Header.Set("Priority", "2")
	case "high":
		req.Header.Set("Priority", "4")
	case "critical":
		req.Header.Set("Priority", "5")
	default:
		req.Header.Set("Priority", "3")
	}

	// Optional auth token
	if cfg.Notifications.Ntfy.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Notifications.Ntfy.Token)
	}

	resp, err := notifyHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("ntfy request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ntfy returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ── Pushover ────────────────────────────────────────────────────────────────

func sendPushover(cfg *config.Config, title, message, priority string) error {
	if !cfg.Notifications.Pushover.Enabled {
		return fmt.Errorf("pushover is not enabled in config")
	}

	userKey := cfg.Notifications.Pushover.UserKey
	appToken := cfg.Notifications.Pushover.AppToken
	if userKey == "" || appToken == "" {
		return fmt.Errorf("pushover user_key and app_token must be configured")
	}

	// Map priority to Pushover levels (-2=lowest, 0=normal, 1=high, 2=emergency)
	pushPrio := "0"
	switch strings.ToLower(priority) {
	case "low":
		pushPrio = "-1"
	case "high":
		pushPrio = "1"
	case "critical":
		pushPrio = "1" // Emergency (2) requires extra params so we use High
	}

	payload, _ := json.Marshal(map[string]string{
		"token":    appToken,
		"user":     userKey,
		"title":    title,
		"message":  message,
		"priority": pushPrio,
	})

	resp, err := notifyHTTPClient.Post("https://api.pushover.net/1/messages.json", "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pushover request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pushover returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ── Telegram (standalone HTTP, no dependency on telegram package state) ──────

func sendTelegramNotification(cfg *config.Config, title, message string) error {
	botToken := cfg.Telegram.BotToken
	chatID := cfg.Telegram.UserID
	if botToken == "" || chatID == 0 {
		return fmt.Errorf("telegram bot_token and telegram_user_id must be configured")
	}

	text := fmt.Sprintf("*%s*\n%s", escapeMarkdownV2(title), escapeMarkdownV2(message))

	payload, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "MarkdownV2",
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	resp, err := notifyHTTPClient.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram returned HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// escapeMarkdownV2 escapes special characters for Telegram MarkdownV2 format.
func escapeMarkdownV2(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_", "*", "\\*", "[", "\\[", "]", "\\]",
		"(", "\\(", ")", "\\)", "~", "\\~", "`", "\\`",
		">", "\\>", "#", "\\#", "+", "\\+", "-", "\\-",
		"=", "\\=", "|", "\\|", "{", "\\{", "}", "\\}",
		".", "\\.", "!", "\\!",
	)
	return replacer.Replace(s)
}

// ── Discord (via injected function to avoid import cycles) ──────────────────

func sendDiscordNotification(cfg *config.Config, discordSend DiscordSendFunc, title, message string) error {
	if !cfg.Discord.Enabled || cfg.Discord.DefaultChannelID == "" {
		return fmt.Errorf("discord is not enabled or default_channel_id is not configured")
	}
	if discordSend == nil {
		return fmt.Errorf("discord send function not available in this context")
	}

	formatted := fmt.Sprintf("**%s**\n%s", title, message)
	return discordSend(cfg.Discord.DefaultChannelID, formatted)
}
