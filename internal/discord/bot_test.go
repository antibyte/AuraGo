package discord

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

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

// IsDM feeds commands.Context.AllowOperator: only direct messages may run
// operator slash commands, guild channels never.
func TestShouldHandleDiscordMessageMarksOnlyDirectMessagesAsDM(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.AllowedUserID = "user-1"
	cfg.Discord.DefaultChannelID = "channel-1"
	author := &discordgo.User{ID: "user-1", Username: "Andi"}

	for _, tc := range []struct {
		name   string
		msg    *discordgo.Message
		wantDM bool
	}{
		{"direct message", &discordgo.Message{Author: author, ChannelID: "dm-1", Content: "/restart"}, true},
		{"default guild channel", &discordgo.Message{Author: author, GuildID: "guild-1", ChannelID: "channel-1", Content: "/restart"}, false},
		{"mention in guild channel", &discordgo.Message{Author: author, GuildID: "guild-1", ChannelID: "channel-2", Content: "<@bot-1> /restart", Mentions: []*discordgo.User{{ID: "bot-1"}}}, false},
	} {
		decision := shouldHandleDiscordMessage("bot-1", &discordgo.MessageCreate{Message: tc.msg}, cfg)
		if !decision.Accepted || decision.IsDM != tc.wantDM {
			t.Fatalf("%s: decision = %+v, want accepted with IsDM=%v", tc.name, decision, tc.wantDM)
		}
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

func TestDiscordConversationIDScopesGuildChannelAndUser(t *testing.T) {
	message := &discordgo.MessageCreate{Message: &discordgo.Message{
		GuildID: "guild-1", ChannelID: "channel-1", Author: &discordgo.User{ID: "user-1"},
	}}
	want := "discord:guild-1:channel-1:user-1"
	if got := discordConversationID(message); got != want {
		t.Fatalf("conversation = %q, want %q", got, want)
	}
	for _, changed := range []*discordgo.MessageCreate{
		{Message: &discordgo.Message{GuildID: "guild-2", ChannelID: "channel-1", Author: &discordgo.User{ID: "user-1"}}},
		{Message: &discordgo.Message{GuildID: "guild-1", ChannelID: "channel-2", Author: &discordgo.User{ID: "user-1"}}},
		{Message: &discordgo.Message{GuildID: "guild-1", ChannelID: "channel-1", Author: &discordgo.User{ID: "user-2"}}},
	} {
		if got := discordConversationID(changed); got == want {
			t.Fatalf("conversation collision after dimension changed: %q", got)
		}
	}
}

// ── Channel allowlist (WP-1) ────────────────────────────────────────────────

// Without an allowlist every channel stays usable: existing installations must
// keep their behaviour.
func TestChannelAllowedWithoutAllowlistPermitsEveryChannel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.DefaultChannelID = "channel-default"

	for _, channelID := range []string{"channel-default", "channel-other", "dm-1"} {
		if err := ChannelAllowed(cfg, channelID); err != nil {
			t.Fatalf("channel %q = %v, want no error without an allowlist", channelID, err)
		}
	}
}

func TestChannelAllowedWithAllowlistPermitsDefaultAndListedChannels(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.DefaultChannelID = "channel-default"
	cfg.Discord.AllowedChannelIDs = []string{"channel-allowed"}

	for _, channelID := range []string{"channel-default", "channel-allowed"} {
		if err := ChannelAllowed(cfg, channelID); err != nil {
			t.Fatalf("channel %q = %v, want allowed", channelID, err)
		}
	}
}

func TestChannelAllowedRejectsChannelOutsideAllowlist(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.DefaultChannelID = "channel-default"
	cfg.Discord.AllowedChannelIDs = []string{"channel-allowed"}

	err := ChannelAllowed(cfg, "channel-other")
	if err == nil {
		t.Fatal("ChannelAllowed = nil, want an error for a channel outside the allowlist")
	}
	msg := err.Error()
	if !strings.Contains(msg, "discord.allowed_channel_ids") {
		t.Fatalf("error = %q, want it to name the config key", msg)
	}
	if !strings.Contains(msg, "channel-default") || !strings.Contains(msg, "channel-allowed") {
		t.Fatalf("error = %q, want it to name the allowed channels", msg)
	}
}

func TestChannelAllowedRejectsEmptyChannel(t *testing.T) {
	cfg := &config.Config{}
	cfg.Discord.AllowedChannelIDs = []string{"channel-allowed"}

	if err := ChannelAllowed(cfg, "   "); err == nil {
		t.Fatal("ChannelAllowed = nil, want an error for an empty channel")
	}
}

func TestFilterAllowedChannelsHidesUnreleasedChannels(t *testing.T) {
	channels := []*discordgo.Channel{
		{ID: "channel-default", Name: "default"},
		{ID: "channel-allowed", Name: "allowed"},
		{ID: "channel-other", Name: "other"},
	}

	cfg := &config.Config{}
	cfg.Discord.DefaultChannelID = "channel-default"
	cfg.Discord.AllowedChannelIDs = []string{"channel-allowed"}

	filtered := filterAllowedChannels(cfg, channels)
	if len(filtered) != 2 {
		t.Fatalf("filtered channels = %d, want 2: %+v", len(filtered), filtered)
	}
	if filtered[0].ID != "channel-default" || filtered[1].ID != "channel-allowed" {
		t.Fatalf("filtered channels = %+v, want the default and the listed channel", filtered)
	}

	legacy := filterAllowedChannels(&config.Config{}, channels)
	if len(legacy) != len(channels) {
		t.Fatalf("filtered channels without allowlist = %d, want %d", len(legacy), len(channels))
	}
}

// ── Worker limit (WP-2) ─────────────────────────────────────────────────────

// fakeDiscordSender records what was sent to a channel.
type fakeDiscordSender struct {
	channelID string
	content   string
	sends     int
}

func (f *fakeDiscordSender) ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	f.channelID = channelID
	f.content = content
	f.sends++
	return &discordgo.Message{ChannelID: channelID, Content: content}, nil
}

func TestAcquireDiscordWorkerReleasesSlot(t *testing.T) {
	slots := make(chan struct{}, 1)
	ctx := context.Background()

	if !acquireDiscordWorker(ctx, slots) {
		t.Fatal("acquireDiscordWorker = false, want a slot")
	}
	shortCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if acquireDiscordWorker(shortCtx, slots) {
		t.Fatal("acquireDiscordWorker = true, want no slot from an exhausted pool")
	}
	<-slots
	if !acquireDiscordWorker(ctx, slots) {
		t.Fatal("acquireDiscordWorker = false, want a slot after the release")
	}
	<-slots
}

func TestRunDiscordMessageReleasesSlotAfterTheRun(t *testing.T) {
	slots := newDiscordWorkerSlots(1)
	if slots == nil {
		t.Fatal("newDiscordWorkerSlots(1) = nil, want a semaphore")
	}
	sender := &fakeDiscordSender{}
	runs := 0

	if !runDiscordMessage(context.Background(), slots, 1, sender, "channel-1", slog.Default(), func() { runs++ }) {
		t.Fatal("runDiscordMessage rejected a message while the pool was free")
	}
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
	if len(slots) != 0 {
		t.Fatalf("slots in use = %d, want 0 after the run", len(slots))
	}
	if sender.sends != 0 {
		t.Fatalf("sends = %d, want no busy reply for an accepted message", sender.sends)
	}
}

func TestRunDiscordMessageRejectsWhenPoolExhausted(t *testing.T) {
	original := discordWorkerWaitTimeout
	discordWorkerWaitTimeout = 50 * time.Millisecond
	t.Cleanup(func() { discordWorkerWaitTimeout = original })

	slots := newDiscordWorkerSlots(1)
	slots <- struct{}{} // another run occupies the only slot
	defer func() { <-slots }()

	sender := &fakeDiscordSender{}
	runs := 0
	accepted := runDiscordMessage(context.Background(), slots, 1, sender, "channel-1", slog.Default(), func() { runs++ })
	if accepted {
		t.Fatal("runDiscordMessage accepted a message while the pool was exhausted")
	}
	if runs != 0 {
		t.Fatalf("runs = %d, want 0 — no agent run may start", runs)
	}
	if sender.sends != 1 {
		t.Fatalf("sends = %d, want exactly one busy reply", sender.sends)
	}
	if sender.content != discordBusyMessage {
		t.Fatalf("busy reply = %q, want %q", sender.content, discordBusyMessage)
	}
	if sender.channelID != "channel-1" {
		t.Fatalf("busy reply channel = %q, want channel-1", sender.channelID)
	}
}

func TestRunDiscordMessageWithoutLimitRunsUnbounded(t *testing.T) {
	sender := &fakeDiscordSender{}
	runs := 0

	if !runDiscordMessage(context.Background(), newDiscordWorkerSlots(0), 0, sender, "channel-1", slog.Default(), func() { runs++ }) {
		t.Fatal("runDiscordMessage rejected a message without a configured limit")
	}
	if runs != 1 {
		t.Fatalf("runs = %d, want 1", runs)
	}
}

// ── Discovery mode (WP-10) ──────────────────────────────────────────────────

func TestDiscoveryModeLogsDiscoveredUser(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	cfg := &config.Config{}
	cfg.Discord.AllowedUserID = "0"
	msg := &discordgo.MessageCreate{Message: &discordgo.Message{
		Author:  &discordgo.User{ID: "user-42", Username: "Andi"},
		GuildID: "guild-1", ChannelID: "channel-1", Content: "hello",
	}}

	decision := shouldHandleDiscordMessage("bot-1", msg, cfg)
	if decision.Accepted || decision.Reason != "discovery_mode" {
		t.Fatalf("decision = %+v, want a blocked discovery_mode decision", decision)
	}
	logBlockedDiscordMessage(logger, msg, cfg, decision)

	out := buf.String()
	if !strings.Contains(out, "user-42") {
		t.Fatalf("log = %q, want the discovered user ID", out)
	}
	if !strings.Contains(out, "Andi") {
		t.Fatalf("log = %q, want the discovered username", out)
	}
}
