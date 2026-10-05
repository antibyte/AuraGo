package discord

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"

	"github.com/bwmarrin/discordgo"
)

// handleMessage feeds decision.IsDM into commands.Context.AllowOperator. The
// test observes /personality's effect on the config, so no Discord session is
// needed (the reply goes nowhere without one).
func TestDiscordOperatorCommandsRunOnlyInDirectMessages(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		name    string
		guildID string
		want    string
	}{
		{"guild default channel", "guild-1", "old"},
		{"direct message", "", "operatorcheck"},
	} {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "personalities"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "personalities", "operatorcheck.md"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(root, "config.yaml")
		if err := os.WriteFile(configPath, []byte("personality:\n  core_personality: old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := &config.Config{ConfigPath: configPath}
		cfg.Personality.CorePersonality = "old"
		cfg.Directories.PromptsDir = root
		cfg.Discord.AllowedUserID = "user-1"
		cfg.Discord.DefaultChannelID = "channel-1"

		msg := &discordgo.MessageCreate{Message: &discordgo.Message{
			Author:    &discordgo.User{ID: "user-1", Username: "Andi"},
			GuildID:   tc.guildID,
			ChannelID: "channel-1",
			Content:   "/personality operatorcheck",
		}}
		handleMessage(nil, msg, cfg, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

		if cfg.Personality.CorePersonality != tc.want {
			t.Fatalf("%s: core personality = %q, want %q", tc.name, cfg.Personality.CorePersonality, tc.want)
		}
	}
}
