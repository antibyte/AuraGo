package rocketchat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"aurago/internal/budget"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"
)

type Bot struct {
	cancel  context.CancelFunc
	done    chan struct{}
	initial *config.Config
}

func startRuntime(parent context.Context, cfg *config.Config, run func(context.Context)) *Bot {
	ctx, cancel := context.WithCancel(parent)
	b := &Bot{cancel: cancel, done: make(chan struct{}), initial: cfg.Clone()}
	go func() { defer close(b.done); run(ctx) }()
	return b
}

// CancelIfConfigChanged revokes old credentials and grants without waiting on a
// worker. The server drains the cancelled generation after releasing config locks.
func (b *Bot) CancelIfConfigChanged(cfg *config.Config) {
	if b != nil && (cfg == nil || cfg.EggMode.Enabled || !reflect.DeepEqual(b.initial.RocketChat, cfg.RocketChat)) {
		b.cancel()
	}
}

func (b *Bot) Stop() {
	if b != nil {
		b.cancel()
		<-b.done
	}
}

// warnRenameableAllowlistEntries flags allowlist entries matched by username:
// a Rocket.Chat username can be changed by its owner or an admin, the user ID
// cannot, so a freed or renamed username could inherit agent access.
func warnRenameableAllowlistEntries(logger *slog.Logger, cfg *config.Config) {
	if logger == nil || cfg == nil {
		return
	}
	for _, entry := range cfg.RocketChat.AllowedUsers {
		if strings.HasPrefix(entry, "username:") {
			logger.Warn("[RocketChat] allowlist entry matches a renameable username; prefer id:<user-id>", "entry", entry)
		}
	}
}

// StartBot creates one server-owned, serial message consumer.
func StartBot(parent context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, remoteHub *remote.RemoteHub, guardian *security.Guardian, budgetTrackerSnapshot func() *budget.Tracker, snapshot func() (*config.Config, llm.ChatClient)) *Bot {
	if cfg == nil || !cfg.RocketChat.Enabled || cfg.EggMode.Enabled || parent.Err() != nil {
		return nil
	}
	warnRenameableAllowlistEntries(logger, cfg)
	if cfg.RocketChat.URL == "" || cfg.RocketChat.AuthToken == "" || cfg.RocketChat.UserID == "" || cfg.RocketChat.Channel == "" {
		logger.Warn("[RocketChat] Missing connection settings; skipping start")
		return nil
	}
	if err := security.ValidateHTTPBaseURL(cfg.RocketChat.URL); err != nil {
		logger.Warn("[RocketChat] Invalid server URL")
		return nil
	}
	cfg = cfg.Clone()
	security.RegisterSensitive(cfg.RocketChat.AuthToken)
	if snapshot == nil {
		snapshot = func() (*config.Config, llm.ChatClient) { return cfg, client }
	}
	return startRuntime(parent, cfg, func(ctx context.Context) {
		pollLoop(ctx, cfg, logger, snapshot, func(ctx context.Context, current *config.Config, currentClient llm.ChatClient, channel string, msg message) {
			processMessage(ctx, current, logger, currentClient, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, channel, msg, missionManagerV2, remoteHub, guardian, budgetTrackerSnapshot)
		})
	})
}

type messageTimestamp struct {
	Date int64 `json:"$date"`
}

func (t *messageTimestamp) UnmarshalJSON(raw []byte) error {
	if len(raw) > 0 && raw[0] == '"' {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return err
		}
		t.Date = parsed.UnixMilli()
		return nil
	}
	type legacy messageTimestamp
	return json.Unmarshal(raw, (*legacy)(t))
}

// Offset counts already consumed messages sharing the cursor timestamp. This
// keeps a page boundary from discarding additional messages at the same time.
type historyCursor struct {
	since  time.Time
	offset int
}

func (c *historyCursor) advance(msg message) {
	timestamp := time.UnixMilli(msg.Timestamp.Date)
	if timestamp.After(c.since) {
		c.since = timestamp
		c.offset = 0
	}
	if timestamp.Equal(c.since) {
		c.offset++
	}
}

func fetchNewMessages(ctx context.Context, cfg *config.Config, channelID string, cursor historyCursor, latest time.Time) ([]message, error) {
	query := url.Values{"roomId": {channelID}, "oldest": {cursor.since.UTC().Format(time.RFC3339Nano)}, "latest": {latest.UTC().Format(time.RFC3339Nano)}, "count": {"50"}, "offset": {strconv.Itoa(cursor.offset)}, "inclusive": {"true"}, "sort": {`{"ts":1,"_id":1}`}}
	data, code, err := rcRequestContext(ctx, cfg, "GET", "/channels.history?"+query.Encode(), "")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("Rocket.Chat history HTTP %d", code)
	}
	var response struct {
		Messages []message `json:"messages"`
		Success  *bool     `json:"success"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode Rocket.Chat history: %w", err)
	}
	if response.Success != nil && !*response.Success {
		return nil, fmt.Errorf("Rocket.Chat rejected history request")
	}
	if len(response.Messages) > 50 {
		return nil, fmt.Errorf("Rocket.Chat history exceeded the requested page size")
	}
	previous := cursor.since
	for _, msg := range response.Messages {
		ts := time.UnixMilli(msg.Timestamp.Date)
		if msg.ID == "" || ts.Before(previous) || ts.After(latest) {
			return nil, fmt.Errorf("Rocket.Chat history has an invalid cursor or order")
		}
		previous = ts
	}
	return response.Messages, nil
}

func pollLoop(ctx context.Context, cfg *config.Config, logger *slog.Logger, snapshot func() (*config.Config, llm.ChatClient), deliver func(context.Context, *config.Config, llm.ChatClient, string, message)) {
	cursor := historyCursor{since: time.Now().Truncate(time.Millisecond)}
	var channelID string
	for ctx.Err() == nil {
		resolved, err := resolveChannelID(ctx, cfg, cfg.RocketChat.Channel)
		if err == nil {
			channelID = resolved
			break
		}
		logger.Warn("[RocketChat] Channel lookup failed", "error", err)
		timer := time.NewTimer(3 * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
	delay := time.Duration(0)
	for ctx.Err() == nil {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		latest := time.Now()
		failed := false
		// Yield after bounded work; the cursor preserves any remaining backlog.
		for page := 0; page < 100 && ctx.Err() == nil; page++ {
			messages, err := fetchNewMessages(ctx, cfg, channelID, cursor, latest)
			if err != nil {
				if ctx.Err() == nil {
					logger.Warn("[RocketChat] Poll failed", "error", err)
				}
				failed = true
				break
			}
			for _, msg := range messages {
				if ctx.Err() != nil {
					return
				}
				current, client := snapshot()
				if current == nil || !current.RocketChat.Enabled || current.EggMode.Enabled || !reflect.DeepEqual(current.RocketChat, cfg.RocketChat) {
					return
				}
				if msg.User.ID != current.RocketChat.UserID && msg.Msg != "" && isAllowedRocketChatUser(current, msg) {
					deliver(ctx, current, client, channelID, msg)
				}
				cursor.advance(msg)
			}
			if len(messages) < 50 {
				break
			}
		}
		if failed {
			delay = max(3*time.Second, min(delay*2, 30*time.Second))
		} else {
			delay = 3 * time.Second
		}
	}
}
