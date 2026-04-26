package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"aurago/internal/security"
	"aurago/internal/telnyx"
	"aurago/internal/tools"
)

func dispatchMessagingCases(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger
	guardian := dc.Guardian
	mediaRegistryDB := dc.MediaRegistryDB

	switch tc.Action {
	case "send_telegram":
		req := decodeSendTelegramArgs(tc)
		logger.Info("LLM requested telegram message", "title", req.Title)
		return "Tool Output: " + tools.SendNotification(cfg, logger, "telegram", req.Title, req.Message, req.Priority, nil, nil), true

	case "send_notification", "notification_center", "send_push_notification", "web_push":
		req := decodeNotificationArgs(tc)
		logger.Info("LLM requested notification", "channel", req.Channel, "title", req.Title)
		var discordSend tools.DiscordSendFunc
		if cfg.Discord.Enabled {
			discordSend = func(channelID, content string) error {
				return tools.DiscordSend(channelID, content, logger)
			}
		}
		var telnyxSend tools.TelnyxSendFunc
		if cfg.Telnyx.Enabled && cfg.Telnyx.PhoneNumber != "" {
			telnyxSend = func(to, message string) error {
				client := telnyx.NewClient(cfg.Telnyx.APIKey, logger)
				_, err := client.SendSMS(ctx, cfg.Telnyx.PhoneNumber, to, message, cfg.Telnyx.MessagingProfileID)
				return err
			}
		}
		return "Tool Output: " + tools.SendNotification(cfg, logger, req.Channel, req.Title, req.Message, req.Priority, discordSend, telnyxSend), true

	case "send_image":
		req := decodeSendMediaArgs(tc)
		logger.Info("LLM requested image send", "path", req.Path, "caption", req.Caption)
		return handleSendImage(req, cfg, logger), true

	case "send_audio":
		req := decodeSendMediaArgs(tc)
		logger.Info("LLM requested audio send", "path", req.Path, "title", req.Title)
		return handleSendAudio(req, cfg, logger, mediaRegistryDB), true

	case "send_video":
		req := decodeSendMediaArgs(tc)
		logger.Info("LLM requested video send", "path", req.Path, "title", req.Title)
		return handleSendVideo(req, cfg, logger, mediaRegistryDB), true

	case "send_youtube_video":
		if !cfg.Tools.SendYouTubeVideo.Enabled {
			return `Tool Output: {"status":"error","message":"send_youtube_video is disabled. Set tools.send_youtube_video.enabled=true in config.yaml."}`, true
		}
		req := decodeYouTubeVideoArgs(tc)
		logger.Info("LLM requested YouTube video send", "url", req.URL, "title", req.Title)
		return handleSendYouTubeVideo(req, logger), true

	case "send_document":
		req := decodeSendMediaArgs(tc)
		logger.Info("LLM requested document send", "path", req.Path, "title", req.Title)
		return handleSendDocument(req, cfg, logger, mediaRegistryDB), true

	case "send_discord":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled. Configure the discord section in config.yaml."}`, true
		}
		if cfg.Discord.ReadOnly {
			return `Tool Output: {"status":"error","message":"Discord is in read-only mode. Disable discord.read_only to allow changes."}`, true
		}
		req := decodeDiscordMessageArgs(tc)
		channelID := req.ChannelID
		if channelID == "" {
			channelID = cfg.Discord.DefaultChannelID
		}
		if channelID == "" {
			return `Tool Output: {"status": "error", "message": "'channel_id' is required (or set default_channel_id in config)"}`, true
		}
		message := req.Message
		if message == "" {
			return `Tool Output: {"status": "error", "message": "'message' (or 'content') is required"}`, true
		}
		logger.Info("LLM requested Discord send", "channel", channelID)
		if err := tools.DiscordSend(channelID, message, logger); err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Discord send failed: %v"}`, err), true
		}
		return fmt.Sprintf(`Tool Output: {"status": "success", "message": "Message sent to Discord channel %s"}`, channelID), true

	case "fetch_discord":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled. Configure the discord section in config.yaml."}`, true
		}
		req := decodeDiscordMessageArgs(tc)
		channelID := req.ChannelID
		if channelID == "" {
			channelID = cfg.Discord.DefaultChannelID
		}
		if channelID == "" {
			return `Tool Output: {"status": "error", "message": "'channel_id' is required (or set default_channel_id in config)"}`, true
		}
		limit := req.Limit
		if limit <= 0 {
			limit = 10
		}
		logger.Info("LLM requested Discord message fetch", "channel", channelID, "limit", limit)
		msgs, err := tools.DiscordFetch(channelID, limit, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Discord fetch failed: %v"}`, err), true
		}
		if guardian != nil {
			for i := range msgs {
				scanRes := guardian.ScanForInjection(msgs[i].Author + " " + msgs[i].Content)
				if scanRes.Level >= security.ThreatHigh {
					logger.Warn("[Discord] Guardian HIGH threat in message", "author", msgs[i].Author, "threat", scanRes.Level.String())
					msgs[i].Content = security.RedactedText("guardian blocked content after injection detection")
				} else {
					msgs[i].Content = guardian.SanitizeToolOutput("discord", msgs[i].Content)
				}
			}
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(msgs),
			"data":   msgs,
		})
		return "Tool Output: " + string(data), true

	case "list_discord_channels":
		if !cfg.Discord.Enabled {
			return `Tool Output: {"status": "error", "message": "Discord is not enabled."}`, true
		}
		guildID := cfg.Discord.GuildID
		if guildID == "" {
			return `Tool Output: {"status": "error", "message": "'guild_id' must be set in config.yaml"}`, true
		}
		logger.Info("LLM requested Discord channel list", "guild", guildID)
		channels, err := tools.DiscordListChannels(guildID, logger)
		if err != nil {
			return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Channel list failed: %v"}`, err), true
		}
		data, _ := json.Marshal(map[string]interface{}{
			"status": "success",
			"count":  len(channels),
			"data":   channels,
		})
		return "Tool Output: " + string(data), true

	case "telnyx_sms":
		req := decodeTelnyxSMSArgs(tc)
		if !cfg.Telnyx.Enabled {
			return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`, true
		}
		if cfg.Telnyx.ReadOnly {
			return `Tool Output: {"status":"error","message":"Telnyx is in read-only mode"}`, true
		}
		if cfg.Telnyx.PhoneNumber == "" {
			return `Tool Output: {"status":"error","message":"Telnyx phone number not configured"}`, true
		}
		return "Tool Output: " + telnyx.DispatchSMS(ctx, req.Operation, req.To, req.Message, req.MessageID, req.MediaURLs, cfg, logger), true

	case "telnyx_call":
		req := decodeTelnyxCallArgs(tc)
		if !cfg.Telnyx.Enabled {
			return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`, true
		}
		if cfg.Telnyx.ReadOnly && req.Operation != "list_active" {
			return `Tool Output: {"status":"error","message":"Telnyx is in read-only mode"}`, true
		}
		return "Tool Output: " + telnyx.DispatchCall(ctx, req.Operation, req.To, req.CallControlID, req.Text, req.AudioURL, req.MaxDigits, req.TimeoutSecs, cfg, logger), true

	case "telnyx_manage":
		req := decodeTelnyxManageArgs(tc)
		if !cfg.Telnyx.Enabled {
			return `Tool Output: {"status":"error","message":"Telnyx integration is disabled"}`, true
		}
		return "Tool Output: " + telnyx.DispatchManage(ctx, req.Operation, req.Limit, req.Port, cfg, logger), true
	}
	return "", false
}
