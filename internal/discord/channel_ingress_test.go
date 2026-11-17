package discord

import (
	"testing"

	"aurago/internal/config"
	"github.com/bwmarrin/discordgo"
)

func TestShouldHandleDiscordMessageHonorsChannelAllowlist(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID = "owner"
	cfg.Discord.DefaultChannelID = "default"
	cfg.Discord.AllowedChannelIDs = []string{"allowed"}
	for _, content := range []string{"hello", "/stop", "answer to a pending question"} {
		for _, tc := range []struct {
			channel, guild string
			allow          bool
		}{
			{"forbidden", "guild", false}, {"allowed", "guild", true},
			{"default", "guild", true}, {"owner-dm", "", true},
		} {
			msg := &discordgo.MessageCreate{Message: &discordgo.Message{
				Author: &discordgo.User{ID: "owner"}, ChannelID: tc.channel, GuildID: tc.guild,
				Content: content, Mentions: []*discordgo.User{{ID: "bot"}},
			}}
			got := shouldHandleDiscordMessage("bot", msg, cfg)
			if got.Accepted != tc.allow || !tc.allow && got.Reason != "channel_not_allowed" {
				t.Fatalf("%s/%s: %+v", tc.channel, content, got)
			}
		}
	}
	cfg.Discord.AllowedChannelIDs = nil
	msg := &discordgo.MessageCreate{Message: &discordgo.Message{Author: &discordgo.User{ID: "owner"}, ChannelID: "legacy", GuildID: "guild", Mentions: []*discordgo.User{{ID: "bot"}}}}
	if got := shouldHandleDiscordMessage("bot", msg, cfg); !got.Accepted {
		t.Fatalf("empty allowlist changed legacy behavior: %+v", got)
	}
}
