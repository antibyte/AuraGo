package tools

import (
	"fmt"
	"log/slog"
	"sync"
)

// ── Discord Bridge ──────────────────────────────────────────────────────────
//
// Breaks the import cycle between agent ↔ discord.
// The discord package registers its functions here at startup.
// The agent package calls them through these function pointers.

// DiscordChannelInfo represents a text channel (cycle-safe DTO).
type DiscordChannelInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DiscordMessageInfo represents a fetched message (cycle-safe DTO).
type DiscordMessageInfo struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

var (
	discordMu         sync.RWMutex
	discordSendFunc   func(channelID, content string, logger *slog.Logger) error
	discordFetchFunc  func(channelID string, limit int, logger *slog.Logger) ([]DiscordMessageInfo, error)
	discordListChFunc func(guildID string, logger *slog.Logger) ([]DiscordChannelInfo, error)
)

// RegisterDiscordBridge is called by the discord package at startup.
func RegisterDiscordBridge(
	send func(channelID, content string, logger *slog.Logger) error,
	fetch func(channelID string, limit int, logger *slog.Logger) ([]DiscordMessageInfo, error),
	listCh func(guildID string, logger *slog.Logger) ([]DiscordChannelInfo, error),
) {
	discordMu.Lock()
	defer discordMu.Unlock()
	discordSendFunc = send
	discordFetchFunc = fetch
	discordListChFunc = listCh
}

// DiscordSend sends a message via the registered Discord bridge.
func DiscordSend(channelID, content string, logger *slog.Logger) error {
	discordMu.RLock()
	fn := discordSendFunc
	discordMu.RUnlock()
	if fn == nil {
		return fmt.Errorf("discord bot is not connected")
	}
	return fn(channelID, content, logger)
}

// DiscordFetch retrieves recent messages via the registered Discord bridge.
func DiscordFetch(channelID string, limit int, logger *slog.Logger) ([]DiscordMessageInfo, error) {
	discordMu.RLock()
	fn := discordFetchFunc
	discordMu.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("discord bot is not connected")
	}
	return fn(channelID, limit, logger)
}

// DiscordListChannels lists guild text channels via the registered Discord bridge.
func DiscordListChannels(guildID string, logger *slog.Logger) ([]DiscordChannelInfo, error) {
	discordMu.RLock()
	fn := discordListChFunc
	discordMu.RUnlock()
	if fn == nil {
		return nil, fmt.Errorf("discord bot is not connected")
	}
	return fn(guildID, logger)
}
