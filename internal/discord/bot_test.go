package discord

import (
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
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

func TestShouldHandleDiscordMessageAllowsDefaultChannelWhenGuildIDDiffers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID = "user-1"
	cfg.Discord.GuildID = "configured-guild"
	cfg.Discord.DefaultChannelID = "channel-1"

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:    &discordgo.User{ID: "user-1", Username: "Andi"},
		GuildID:   "actual-guild",
		ChannelID: "channel-1",
		Content:   "status bitte",
	}}

	decision := shouldHandleDiscordMessage("bot-1", msg, cfg)
	if !decision.Accepted {
		t.Fatalf("decision = %+v, want default channel message accepted despite guild mismatch", decision)
	}
	if decision.Reason != "default_channel" {
		t.Fatalf("reason = %q, want default_channel", decision.Reason)
	}
}

func TestShouldHandleDiscordMessageRejectsWrongGuildOutsideDefaultChannel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID = "user-1"
	cfg.Discord.GuildID = "configured-guild"
	cfg.Discord.DefaultChannelID = "channel-1"

	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:    &discordgo.User{ID: "user-1", Username: "Andi"},
		GuildID:   "actual-guild",
		ChannelID: "channel-2",
		Content:   "<@bot-1> status bitte",
		Mentions:  []*discordgo.User{{ID: "bot-1"}},
	}}

	decision := shouldHandleDiscordMessage("bot-1", msg, cfg)
	if decision.Accepted {
		t.Fatalf("decision = %+v, want wrong guild message rejected", decision)
	}
	if decision.Reason != "wrong_guild" {
		t.Fatalf("reason = %q, want wrong_guild", decision.Reason)
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

func TestBuildDiscordAgentMessagesKeepsRecapAfterSystemPlaceholder(t *testing.T) {
	historyManager := memory.NewEphemeralHistoryManager()
	t.Cleanup(historyManager.Close)
	if err := historyManager.SetSummary("short recap"); err != nil {
		t.Fatalf("set summary: %v", err)
	}
	if err := historyManager.Add(openai.ChatMessageRoleUser, "status bitte", 1, false, false); err != nil {
		t.Fatalf("add history: %v", err)
	}

	messages := buildDiscordAgentMessages(historyManager)

	if len(messages) != 3 {
		t.Fatalf("message count = %d, want 3: %#v", len(messages), messages)
	}
	if messages[0].Role != openai.ChatMessageRoleSystem || messages[0].Content != "" {
		t.Fatalf("first message = (%q, %q), want empty system placeholder", messages[0].Role, messages[0].Content)
	}
	if messages[1].Role != openai.ChatMessageRoleSystem || !strings.Contains(messages[1].Content, "short recap") {
		t.Fatalf("second message = (%q, %q), want recap system message", messages[1].Role, messages[1].Content)
	}
	if messages[2].Role != openai.ChatMessageRoleUser || messages[2].Content != "status bitte" {
		t.Fatalf("third message = (%q, %q), want preserved history user message", messages[2].Role, messages[2].Content)
	}
}
