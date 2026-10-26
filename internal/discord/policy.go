package discord

import (
	"fmt"
	"strings"

	"aurago/internal/config"

	"github.com/bwmarrin/discordgo"
)

// ChannelAllowed reports whether the agent may read from or write to the given
// Discord channel.
//
// The allowlist is opt-in so that existing installations keep working: an
// empty discord.allowed_channel_ids list permits every channel the bot can
// reach. Once the list is set, only the default channel and the listed
// channels may be used by the agent.
func ChannelAllowed(cfg *config.Config, channelID string) error {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return fmt.Errorf("channel_id is required")
	}
	if cfg == nil {
		return fmt.Errorf("discord configuration is not available")
	}

	allowlist := configuredAllowlist(cfg)
	if len(allowlist) == 0 {
		// No allowlist configured: keep the legacy behaviour.
		return nil
	}

	allowed := allowedChannelIDs(cfg, allowlist)
	for _, id := range allowed {
		if id == channelID {
			return nil
		}
	}

	return fmt.Errorf("discord channel %q is not allowed for the agent; allowed channels: %s; add it to discord.allowed_channel_ids in config.yaml to permit it",
		channelID, strings.Join(allowed, ", "))
}

// filterAllowedChannels keeps only the channels the agent may use. Without a
// configured allowlist every channel stays visible, so the listing is
// unchanged for existing installations.
func filterAllowedChannels(cfg *config.Config, channels []*discordgo.Channel) []*discordgo.Channel {
	allowed := make([]*discordgo.Channel, 0, len(channels))
	for _, ch := range channels {
		if ch == nil {
			continue
		}
		if err := ChannelAllowed(cfg, ch.ID); err != nil {
			continue
		}
		allowed = append(allowed, ch)
	}
	return allowed
}

// configuredAllowlist returns the non-empty entries of discord.allowed_channel_ids.
// The result is empty when no allowlist is configured, which means "no restriction".
func configuredAllowlist(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	var list []string
	for _, id := range cfg.Discord.AllowedChannelIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		list = append(list, id)
	}
	return list
}

// allowedChannelIDs returns the channels the agent may use: the default channel
// plus the configured allowlist, without duplicates and in a stable order.
func allowedChannelIDs(cfg *config.Config, allowlist []string) []string {
	var allowed []string
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		allowed = append(allowed, id)
	}
	if cfg != nil {
		add(cfg.Discord.DefaultChannelID)
	}
	for _, id := range allowlist {
		add(id)
	}
	return allowed
}
