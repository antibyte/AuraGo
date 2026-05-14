package discord

import (
	"strings"
	"testing"

	"aurago/internal/config"

	"github.com/bwmarrin/discordgo"
)

func TestStatusReportsDisabledAndMissingToken(t *testing.T) {
	resetStatusForTest()

	disabled := Status(&config.Config{})
	if disabled.Status != "disabled" || disabled.Connected {
		t.Fatalf("disabled status = %+v, want disabled and disconnected", disabled)
	}

	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	missing := Status(cfg)
	if missing.Status != "missing_token" || missing.Connected || missing.TokenPresent {
		t.Fatalf("missing-token status = %+v, want missing_token and disconnected", missing)
	}
}

func TestStatusAnnotatesDisallowedIntentErrors(t *testing.T) {
	resetStatusForTest()
	setErrorForTest("websocket closed: 4014: Disallowed intent(s)")

	cfg := &config.Config{}
	cfg.Discord.Enabled = true
	cfg.Discord.BotToken = "token"

	st := Status(cfg)
	if st.Status != "error" {
		t.Fatalf("status = %q, want error", st.Status)
	}
	if !strings.Contains(strings.ToLower(st.Message), "message content intent") {
		t.Fatalf("message = %q, want message content intent hint", st.Message)
	}
}

func TestShouldHandleDiscordMessageAllowsDefaultChannelWithoutMention(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID = "user-1"
	cfg.Discord.DefaultChannelID = "channel-1"

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:    &discordgo.User{ID: "user-1", Username: "Andi"},
		GuildID:   "guild-1",
		ChannelID: "channel-1",
		Content:   "status bitte",
	}}

	decision := shouldHandleDiscordMessage("bot-1", msg, cfg)
	if !decision.Accepted {
		t.Fatalf("decision = %+v, want default channel message accepted", decision)
	}
	if decision.Reason != "default_channel" {
		t.Fatalf("reason = %q, want default_channel", decision.Reason)
	}
}

func TestShouldHandleDiscordMessageIgnoresOtherChannelWithoutMention(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID = "user-1"
	cfg.Discord.DefaultChannelID = "channel-1"

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:    &discordgo.User{ID: "user-1", Username: "Andi"},
		GuildID:   "guild-1",
		ChannelID: "channel-2",
		Content:   "status bitte",
	}}

	decision := shouldHandleDiscordMessage("bot-1", msg, cfg)
	if decision.Accepted {
		t.Fatalf("decision = %+v, want non-default channel without mention ignored", decision)
	}
	if decision.Reason != "not_mentioned" {
		t.Fatalf("reason = %q, want not_mentioned", decision.Reason)
	}
}
