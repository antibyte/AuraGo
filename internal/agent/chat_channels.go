package agent

import (
	"fmt"
	"sort"
	"strings"

	"aurago/internal/remote"
)

func buildReachableChatChannelsContext(runCfg RunConfig) string {
	cfg := runCfg.Config
	if cfg == nil {
		return ""
	}

	var lines []string
	if cfg.Telegram.BotToken != "" && cfg.Telegram.UserID != 0 {
		lines = append(lines, "- Telegram: reachable via `send_telegram` to the configured default Telegram chat.")
	}
	if cfg.Discord.Enabled {
		target := "configured Discord server"
		if cfg.Discord.DefaultChannelID != "" {
			target = fmt.Sprintf("default channel `%s`", safePromptMetadataText(cfg.Discord.DefaultChannelID, 80))
		}
		if cfg.Discord.ReadOnly {
			lines = append(lines, "- Discord: reachable read-only via `fetch_discord` and `list_discord_channels`; outbound sends are blocked by config.")
		} else {
			lines = append(lines, "- Discord: reachable via `send_discord` to "+target+"; use `list_discord_channels` when a different channel is needed.")
		}
	}
	if cfg.Telnyx.Enabled {
		switch {
		case cfg.Telnyx.ReadOnly:
			lines = append(lines, "- SMS: Telnyx is reachable for inbound messages only; outbound SMS is blocked by config.")
		case cfg.Telnyx.PhoneNumber != "" && len(cfg.Telnyx.AllowedNumbers) > 0:
			lines = append(lines, "- SMS: reachable via `send_notification` with channel `telnyx` for the primary allowed SMS target, or `telnyx_sms` for an explicitly allowed number.")
		case cfg.Telnyx.PhoneNumber != "":
			lines = append(lines, "- SMS: Telnyx is configured but has no allowed outbound numbers; do not send SMS until an allowed number is configured.")
		}
	}
	if cfg.Notifications.Ntfy.Enabled {
		lines = append(lines, "- Ntfy: reachable via `send_notification` with channel `ntfy`.")
	}
	if cfg.Notifications.Pushover.Enabled {
		lines = append(lines, "- Pushover: reachable via `send_notification` with channel `pushover`.")
	}
	if cfg.RocketChat.Enabled && cfg.RocketChat.Channel != "" {
		lines = append(lines, "- Rocket.Chat: incoming messages identify as Rocket.Chat; direct proactive send tools are not exposed, so reply in-session when contacted there.")
	}

	for _, device := range connectedAgoDeskChatDevices(runCfg.RemoteHub) {
		label := safePromptMetadataText(device.Name, 80)
		if label == "" {
			label = safePromptMetadataText(device.Hostname, 80)
		}
		if label == "" {
			label = "AgoDesk"
		}
		deviceID := safePromptMetadataText(device.ID, 80)
		lines = append(lines, fmt.Sprintf("- AgoChat: `%s` (%s) is connected; send proactive text with `send_agodesk_chat` using device_id `%s`.", label, deviceID, deviceID))
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func hasConnectedAgoDeskChatDevice(hub *remote.RemoteHub) bool {
	return len(connectedAgoDeskChatDevices(hub)) > 0
}

func connectedAgoDeskChatDevices(hub *remote.RemoteHub) []remote.DeviceRecord {
	if hub == nil || hub.DB() == nil {
		return nil
	}
	devices, err := remote.ListDevices(hub.DB())
	if err != nil {
		return nil
	}
	var out []remote.DeviceRecord
	for _, device := range devices {
		if !isAgoDeskDevice(device) || !hub.IsConnected(device.ID) {
			continue
		}
		out = append(out, device)
	}
	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(out[i].Name + out[i].ID))
		right := strings.ToLower(strings.TrimSpace(out[j].Name + out[j].ID))
		return left < right
	})
	return out
}

func isAgoDeskDevice(device remote.DeviceRecord) bool {
	for _, tag := range device.Tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "agodesk", "desktop-client":
			return true
		}
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(device.Name)), "agodesk")
}
