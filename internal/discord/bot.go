package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/media"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/telegram"
	"aurago/internal/tools"
	"aurago/internal/webhooks"

	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
)

func buildDiscordAgentMessages(historyManager *memory.HistoryManager) []openai.ChatCompletionMessage {
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem},
	}
	if historyManager == nil {
		return finalMessages
	}
	currentSummary := historyManager.GetSummary()
	if currentSummary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: agent.FormatContextRecapForPrompt(currentSummary),
		})
	}
	return append(finalMessages, historyManager.Get()...)
}

func discordConversationID(m *discordgo.MessageCreate) string {
	guild := m.GuildID
	if guild == "" {
		guild = "dm"
	}
	return "discord:" + guild + ":" + m.ChannelID + ":" + m.Author.ID
}

// discordBotUserID returns the ID of the bot user behind the session, or "" when
// the session is not connected yet.
func discordBotUserID(s *discordgo.Session) string {
	if s == nil || s.State == nil || s.State.User == nil {
		return ""
	}
	return s.State.User.ID
}

// session holds the active Discord session so tools can send messages.
var (
	session   *discordgo.Session
	sessionMu sync.RWMutex
	status    BotStatus
	statusMu  sync.RWMutex
)

// BotStatus describes the runtime state of the Discord bot without exposing secrets.
type BotStatus struct {
	Enabled                  bool   `json:"enabled"`
	Connected                bool   `json:"connected"`
	Status                   string `json:"status"`
	Message                  string `json:"message,omitempty"`
	LastError                string `json:"last_error,omitempty"`
	User                     string `json:"user,omitempty"`
	TokenPresent             bool   `json:"token_present"`
	ReadOnly                 bool   `json:"read_only"`
	GuildConfigured          bool   `json:"guild_configured"`
	DefaultChannelConfigured bool   `json:"default_channel_configured"`
	AllowedUserConfigured    bool   `json:"allowed_user_configured"`
	UpdatedAt                string `json:"updated_at,omitempty"`
	DeliveryFailures         int    `json:"delivery_failures"`
	LastDeliveryErrorAt      string `json:"last_delivery_error_at,omitempty"`
}

// deliveryState counts messages that could not be delivered. It is kept apart
// from the bot status, because a status update replaces that struct and must
// not reset the counter.
var deliveryState = struct {
	sync.Mutex
	failures    int
	lastFailure string
}{}

// markDeliveryFailure counts a message that could not be delivered. A lost
// answer is invisible to the user, so the counter makes it visible in the
// health endpoint.
func markDeliveryFailure(now time.Time) {
	deliveryState.Lock()
	defer deliveryState.Unlock()
	deliveryState.failures++
	deliveryState.lastFailure = now.UTC().Format(time.RFC3339)
}

func setSession(s *discordgo.Session) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	session = s
}

func setStatus(st BotStatus) {
	if st.UpdatedAt == "" {
		st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	statusMu.Lock()
	status = st
	statusMu.Unlock()
}

func discordErrorMessage(errText string) string {
	msg := strings.TrimSpace(errText)
	if msg == "" {
		return ""
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "4014") || strings.Contains(lower, "disallowed intent") {
		return msg + " — enable the Message Content Intent for the bot in the Discord Developer Portal, or remove that privileged intent from the bot configuration."
	}
	if strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "improper token") {
		return msg + " — check whether the Discord bot token in the AuraGo vault is valid and has not been reset."
	}
	return msg
}

func statusFromConfig(cfg *config.Config, st BotStatus) BotStatus {
	if cfg != nil {
		st.Enabled = cfg.Discord.Enabled
		st.TokenPresent = strings.TrimSpace(cfg.Discord.BotToken) != ""
		st.ReadOnly = cfg.Discord.ReadOnly
		st.GuildConfigured = strings.TrimSpace(cfg.Discord.GuildID) != ""
		st.DefaultChannelConfigured = strings.TrimSpace(cfg.Discord.DefaultChannelID) != ""
		st.AllowedUserConfigured = strings.TrimSpace(cfg.Discord.AllowedUserID) != "" && strings.TrimSpace(cfg.Discord.AllowedUserID) != "0"
	}
	if !st.Enabled {
		st.Connected = false
		st.Status = "disabled"
		st.Message = "Discord integration is disabled."
		return st
	}
	if !st.TokenPresent {
		st.Connected = false
		st.Status = "missing_token"
		st.Message = "Discord is enabled, but no bot token is available in the vault."
		return st
	}
	if st.Status == "" {
		st.Status = "disconnected"
		st.Message = "Discord bot is not connected."
	}
	if st.LastError != "" {
		st.Message = discordErrorMessage(st.LastError)
	}
	return st
}

// Status returns the current Discord bot status with config-derived diagnostics.
func Status(cfg *config.Config) BotStatus {
	statusMu.RLock()
	st := status
	statusMu.RUnlock()
	deliveryState.Lock()
	st.DeliveryFailures = deliveryState.failures
	st.LastDeliveryErrorAt = deliveryState.lastFailure
	deliveryState.Unlock()
	return statusFromConfig(cfg, st)
}

func resetStatusForTest() {
	statusMu.Lock()
	status = BotStatus{}
	statusMu.Unlock()
	setSession(nil)
}

func setErrorForTest(errText string) {
	setStatus(BotStatus{Status: "error", LastError: errText, Connected: false})
}

// ── Worker limit ────────────────────────────────────────────────────────────

// discordWorkerWaitTimeout bounds how long an inbound message waits for a free
// worker slot before it is rejected. It is a variable so tests can shorten it.
var discordWorkerWaitTimeout = 60 * time.Second

// discordBusyMessage is the short reply sent when no worker slot became free.
const discordBusyMessage = "⏳ I'm busy right now — please try again in a moment."

// discordMessageSender is the part of the Discord session the inbound path uses
// to answer a message.
type discordMessageSender interface {
	ChannelMessageSend(channelID string, content string, options ...discordgo.RequestOption) (*discordgo.Message, error)
}

// newDiscordWorkerSlots creates the semaphore that bounds how many inbound
// messages run an agent loop at the same time. A limit of zero or less means no
// limit, signalled by a nil channel.
func newDiscordWorkerSlots(limit int) chan struct{} {
	if limit <= 0 {
		return nil
	}
	return make(chan struct{}, limit)
}

// acquireDiscordWorker reserves one Discord worker slot, mirroring the Telegram
// worker semaphore. It returns false when the context is cancelled or the wait
// budget runs out before a slot becomes free.
func acquireDiscordWorker(ctx context.Context, slots chan struct{}) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case slots <- struct{}{}:
		if ctx.Err() != nil {
			<-slots
			return false
		}
		return true
	case <-ctx.Done():
		return false
	}
}

// runDiscordMessage runs the agent pipeline for one accepted message under the
// worker limit. It returns false when the message was rejected because no
// worker slot became free in time; then no agent run happened.
func runDiscordMessage(ctx context.Context, slots chan struct{}, limit int, sender discordMessageSender, channelID string, logger *slog.Logger, run func()) bool {
	if limit <= 0 || slots == nil {
		run()
		return true
	}
	waitCtx, cancel := context.WithTimeout(ctx, discordWorkerWaitTimeout)
	defer cancel()
	if !acquireDiscordWorker(waitCtx, slots) {
		if logger != nil {
			logger.Warn("[Discord] Worker pool exhausted, message rejected", "channel", channelID, "limit", limit)
		}
		if sender != nil {
			sender.ChannelMessageSend(channelID, discordBusyMessage)
		}
		return false
	}
	defer func() { <-slots }()
	run()
	return true
}

// StartBot initializes the Discord bot and begins listening for messages.
func StartBot(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, remoteHub *remote.RemoteHub, guardian *security.Guardian, budgetTrackerSnapshot func() *budget.Tracker, webhookManager *webhooks.Manager, snapshot func() (*config.Config, llm.ChatClient), routingDecisions agent.TaskDecisionService) {
	if cfg == nil || !cfg.Discord.Enabled {
		setStatus(statusFromConfig(cfg, BotStatus{Status: "disabled", Connected: false}))
		return
	}
	if strings.TrimSpace(cfg.Discord.BotToken) == "" {
		if cfg.Discord.Enabled {
			logger.Warn("[Discord] Bot token is missing, skipping Discord start.")
		}
		setStatus(statusFromConfig(cfg, BotStatus{Status: "missing_token", Connected: false}))
		return
	}

	if existing := GetSession(); existing != nil {
		if err := existing.Close(); err != nil && logger != nil {
			logger.Warn("[Discord] Error closing previous session before reconnect", "error", err)
		}
		setSession(nil)
	}

	setStatus(statusFromConfig(cfg, BotStatus{Status: "connecting", Connected: false, Message: "Connecting to Discord gateway..."}))

	dg, err := discordgo.New("Bot " + cfg.Discord.BotToken)
	if err != nil {
		logger.Error("[Discord] Failed to create session", "error", err)
		setStatus(statusFromConfig(cfg, BotStatus{Status: "error", Connected: false, LastError: err.Error()}))
		return
	}

	// Only subscribe to message events for efficiency
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	// The worker limit bounds how many inbound messages run an agent loop at the
	// same time. The Discord bot is restarted whenever the discord config
	// section changes, so the startup value stays current.
	workerLimit := cfg.Discord.MaxConcurrentWorkers
	workerSlots := newDiscordWorkerSlots(workerLimit)

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		current, currentClient := cfg, client
		if snapshot != nil {
			current, currentClient = snapshot()
		}
		if current == nil || !current.Discord.Enabled || current.EggMode.Enabled || current.Discord.BotToken != cfg.Discord.BotToken {
			return
		}
		// Authorization first: a message that is not addressed to the bot
		// neither occupies a worker slot nor triggers a busy reply.
		decision := shouldHandleDiscordMessage(discordBotUserID(s), m, current)
		if !decision.Accepted {
			logBlockedDiscordMessage(logger, m, current, decision)
			return
		}
		runDiscordMessage(context.Background(), workerSlots, workerLimit, s, m.ChannelID, logger, func() {
			handleMessage(s, m, current, logger, currentClient, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, missionManagerV2, remoteHub, guardian, budgetTrackerSnapshot, webhookManager, routingDecisions)
		})
	})

	if err := dg.Open(); err != nil {
		logger.Error("[Discord] Failed to open websocket connection", "error", err)
		setStatus(statusFromConfig(cfg, BotStatus{Status: "error", Connected: false, LastError: err.Error()}))
		return
	}

	setSession(dg)
	user := ""
	if dg.State != nil && dg.State.User != nil {
		user = dg.State.User.Username + "#" + dg.State.User.Discriminator
	}
	logger.Info("[Discord] Bot connected", "user", user)
	setStatus(statusFromConfig(cfg, BotStatus{Status: "connected", Connected: true, User: user, Message: "Discord bot is connected."}))

	// The channel policy is evaluated against the live configuration, so a
	// changed allowlist applies without restarting the bot.
	policyConfig := func() *config.Config {
		if snapshot == nil {
			return cfg
		}
		current, _ := snapshot()
		if current == nil {
			return cfg
		}
		return current
	}

	if len(configuredAllowlist(cfg)) == 0 {
		logger.Warn("[Discord] No channel allowlist configured — the agent may use any channel the bot can reach; restrict this with discord.allowed_channel_ids in config.yaml")
	}

	// Register bridge functions so agent can call Discord without import cycles
	tools.RegisterDiscordBridge(
		func(channelID, content string, logger *slog.Logger) error {
			if err := ChannelAllowed(policyConfig(), channelID); err != nil {
				return err
			}
			return SendMessage(channelID, content, logger)
		},
		func(channelID string, limit int, logger *slog.Logger) ([]tools.DiscordMessageInfo, error) {
			if err := ChannelAllowed(policyConfig(), channelID); err != nil {
				return nil, err
			}
			msgs, err := FetchMessages(channelID, limit, logger)
			if err != nil {
				return nil, err
			}
			var result []tools.DiscordMessageInfo
			for _, m := range msgs {
				result = append(result, tools.DiscordMessageInfo{
					ID:        m.ID,
					Author:    m.Author.Username,
					Content:   m.Content,
					Timestamp: m.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
				})
			}
			return result, nil
		},
		func(guildID string, logger *slog.Logger) ([]tools.DiscordChannelInfo, error) {
			chs, err := ListGuildChannels(guildID, logger)
			if err != nil {
				return nil, err
			}
			policyCfg := policyConfig()
			var result []tools.DiscordChannelInfo
			for _, ch := range filterAllowedChannels(policyCfg, chs) {
				result = append(result, tools.DiscordChannelInfo{
					ID:   ch.ID,
					Name: ch.Name,
				})
			}
			return result, nil
		},
	)
}

// StopBot gracefully closes the Discord websocket connection and clears the session.
// Safe to call multiple times.
func StopBot(logger *slog.Logger) {
	s := GetSession()
	if s == nil {
		setStatus(BotStatus{Status: "disconnected", Connected: false, Message: "Discord bot is not connected."})
		return
	}
	if err := s.Close(); err != nil {
		logger.Warn("[Discord] Error closing session", "error", err)
	}
	setSession(nil)
	setStatus(BotStatus{Status: "disconnected", Connected: false, Message: "Discord bot disconnected."})
	logger.Info("[Discord] Bot disconnected")
}

// GetSession returns the active Discord session (for sending messages from tools).
func GetSession() *discordgo.Session {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return session
}

// agentTextMessage wraps text the agent produced (answers, command results,
// status events, the send_discord tool) with embeds suppressed. Discord fetches
// the URL of an embed itself, so an answer that ends in an attacker's URL with
// conversation data in its query would leak without any tool call. Every such
// text has to be built here; a plain ChannelMessageSend would embed links.
func agentTextMessage(content string) *discordgo.MessageSend {
	return &discordgo.MessageSend{Content: content, Flags: discordgo.MessageFlagsSuppressEmbeds}
}

// SendMessage sends a message to a Discord channel. Used by the agent's send_discord tool.
func SendMessage(channelID, content string, logger *slog.Logger) error {
	content = security.Scrub(content)
	s := GetSession()
	if s == nil {
		return fmt.Errorf("discord bot is not connected")
	}

	totalLen := len(content)

	// Discord limit: 2000 chars per message
	for len(content) > 0 {
		chunk := content
		if utf8.RuneCountInString(chunk) > 1990 {
			// Try to cut at a newline first, respecting UTF-8 boundaries
			runes := []rune(chunk)
			cutAtRunes := 1990
			cutAtBytes := len([]byte(string(runes[:cutAtRunes])))

			// Look for a newline within the last 500 runes to avoid cutting mid-paragraph
			newlineIdx := -1
			searchStart := cutAtRunes - 500
			if searchStart < 0 {
				searchStart = 0
			}
			for i := cutAtRunes - 1; i >= searchStart; i-- {
				if runes[i] == '\n' {
					newlineIdx = i
					break
				}
			}
			if newlineIdx >= 0 {
				cutAtRunes = newlineIdx + 1 // include the newline in the first chunk
				cutAtBytes = len([]byte(string(runes[:cutAtRunes])))
			}

			chunk = content[:cutAtBytes]
			content = content[cutAtBytes:]
		} else {
			content = ""
		}

		if _, err := s.ChannelMessageSendComplex(channelID, agentTextMessage(chunk)); err != nil {
			return fmt.Errorf("failed to send Discord message: %w", err)
		}
	}

	logger.Info("[Discord] Message sent", "channel", channelID, "len", totalLen)
	return nil
}

// SendDiscordImage sends a local image file as a Discord file attachment with an optional caption.
func SendDiscordImage(channelID, localPath, caption string, logger *slog.Logger) error {
	s := GetSession()
	if s == nil {
		return fmt.Errorf("discord bot is not connected")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open image file: %w", err)
	}
	defer f.Close()

	msg := &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:   filepath.Base(localPath),
				Reader: f,
			},
		},
	}
	if caption != "" {
		msg.Content = caption
	}
	_, err = s.ChannelMessageSendComplex(channelID, msg)
	if err != nil {
		return fmt.Errorf("failed to send Discord image: %w", err)
	}
	logger.Info("[Discord] Image sent", "channel", channelID, "file", filepath.Base(localPath))
	return nil
}

// SendDiscordAudio sends a local audio file as a Discord file attachment with an optional title.
func SendDiscordAudio(channelID, localPath, title string, logger *slog.Logger) error {
	s := GetSession()
	if s == nil {
		return fmt.Errorf("discord bot is not connected")
	}
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}
	defer f.Close()

	msg := &discordgo.MessageSend{
		Files: []*discordgo.File{
			{
				Name:   filepath.Base(localPath),
				Reader: f,
			},
		},
	}
	if title != "" {
		msg.Content = title
	}
	_, err = s.ChannelMessageSendComplex(channelID, msg)
	if err != nil {
		return fmt.Errorf("failed to send Discord audio: %w", err)
	}
	logger.Info("[Discord] Audio sent", "channel", channelID, "file", filepath.Base(localPath))
	return nil
}

// FetchMessages retrieves the last N messages from a Discord channel.
func FetchMessages(channelID string, limit int, logger *slog.Logger) ([]*discordgo.Message, error) {
	s := GetSession()
	if s == nil {
		return nil, fmt.Errorf("discord bot is not connected")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // Discord API max
	}

	messages, err := s.ChannelMessages(channelID, limit, "", "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Discord messages: %w", err)
	}

	logger.Info("[Discord] Messages fetched", "channel", channelID, "count", len(messages))
	return messages, nil
}

// ListGuildChannels returns text channels the bot can see in a guild.
func ListGuildChannels(guildID string, logger *slog.Logger) ([]*discordgo.Channel, error) {
	s := GetSession()
	if s == nil {
		return nil, fmt.Errorf("discord bot is not connected")
	}

	channels, err := s.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}

	var textChannels []*discordgo.Channel
	for _, ch := range channels {
		if ch.Type == discordgo.ChannelTypeGuildText {
			textChannels = append(textChannels, ch)
		}
	}

	logger.Info("[Discord] Channels listed", "guild", guildID, "count", len(textChannels))
	return textChannels, nil
}

// ── Message Handler ─────────────────────────────────────────────────────────

type messageDecision struct {
	Accepted    bool
	Reason      string
	IsDM        bool
	IsMentioned bool
}

func shouldHandleDiscordMessage(botUserID string, m *discordgo.MessageCreate, cfg *config.Config) messageDecision {
	if m == nil || m.Author == nil {
		return messageDecision{Reason: "invalid_message"}
	}
	if m.Author.ID == botUserID {
		return messageDecision{Reason: "self"}
	}

	allowedUserID := strings.TrimSpace(cfg.Discord.AllowedUserID)
	if allowedUserID == "" {
		return messageDecision{Reason: "missing_allowed_user"}
	}
	if allowedUserID == "0" {
		return messageDecision{Reason: "discovery_mode"}
	}
	if m.Author.ID != allowedUserID {
		return messageDecision{Reason: "unauthorized_user"}
	}

	isDM := m.GuildID == ""
	isMentioned := false
	for _, mention := range m.Mentions {
		if mention.ID == botUserID {
			isMentioned = true
			break
		}
	}
	if isDM {
		return messageDecision{Accepted: true, Reason: "dm", IsDM: true, IsMentioned: isMentioned}
	}
	if defaultChannelID := strings.TrimSpace(cfg.Discord.DefaultChannelID); defaultChannelID != "" && m.ChannelID == defaultChannelID {
		return messageDecision{Accepted: true, Reason: "default_channel", IsMentioned: isMentioned}
	}
	if guildID := strings.TrimSpace(cfg.Discord.GuildID); guildID != "" && m.GuildID != "" && m.GuildID != guildID {
		return messageDecision{Reason: "wrong_guild"}
	}
	if isMentioned {
		return messageDecision{Accepted: true, Reason: "mention", IsMentioned: true}
	}
	return messageDecision{Reason: "not_mentioned"}
}

// logBlockedDiscordMessage records why an inbound message was dropped. In
// discovery mode (discord.allowed_user_id == "0") it reports the ID and name of
// the discovered user, so the operator can authorize that user.
func logBlockedDiscordMessage(logger *slog.Logger, m *discordgo.MessageCreate, cfg *config.Config, decision messageDecision) {
	if logger == nil || m == nil {
		return
	}
	switch decision.Reason {
	case "missing_allowed_user":
		logger.Warn("[Discord] Blocking message because allowed_user_id is not configured", "user_id", m.Author.ID, "username", m.Author.Username)
	case "discovery_mode":
		logger.Warn("[Discord] Unauthorized Discord ID discovered — set discord.allowed_user_id to authorize this user", "id", m.Author.ID, "username", m.Author.Username)
	case "unauthorized_user":
		logger.Warn("[Discord] Blocked unauthorized Discord message", "user_id", m.Author.ID)
	case "wrong_guild":
		logger.Warn("[Discord] Ignoring message from unexpected guild", "configured_guild", strings.TrimSpace(cfg.Discord.GuildID), "message_guild", m.GuildID, "channel", m.ChannelID)
	case "not_mentioned":
		logger.Debug("[Discord] Ignoring channel message without bot mention outside default channel", "channel", m.ChannelID)
	}
}

// sendDiscordNotice sends a short notice to the channel and records a failed
// delivery. A notice that does not arrive is invisible to the user, so it is
// counted in the health endpoint.
func sendDiscordNotice(s *discordgo.Session, channelID, text string, logger *slog.Logger) {
	if _, err := s.ChannelMessageSend(channelID, text); err != nil {
		markDeliveryFailure(time.Now())
		if logger != nil {
			logger.Error("[Discord] Notice could not be delivered", "error", err)
		}
	}
}

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, remoteHub *remote.RemoteHub, guardian *security.Guardian, budgetTrackerSnapshot func() *budget.Tracker, webhookManager *webhooks.Manager, routingDecisions ...agent.TaskDecisionService) {
	botUserID := discordBotUserID(s)
	decision := shouldHandleDiscordMessage(botUserID, m, cfg)
	if !decision.Accepted {
		logBlockedDiscordMessage(logger, m, cfg, decision)
		return
	}

	var budgetTracker *budget.Tracker
	if budgetTrackerSnapshot != nil {
		budgetTracker = budgetTrackerSnapshot()
	}
	preprocessingCtx := budget.WithTracking(context.Background(), budgetTracker, "chat")
	inputText := m.Content
	sessionID, err := shortTermMem.ResolveUserSession(context.Background(), discordConversationID(m))
	if err != nil {
		logger.Error("[Discord] Resolve conversation", "error", err)
		return
	}
	hasAttachments := len(m.Attachments) > 0
	if inputText == "" && !hasAttachments {
		return
	}
	ctx, releaseRun, err := shortTermMem.RegisterSessionRun(preprocessingCtx, sessionID)
	if err != nil {
		return
	}
	defer releaseRun()

	// Strip the bot mention from the message text
	if decision.IsMentioned {
		inputText = strings.ReplaceAll(inputText, "<@"+botUserID+">", "")
		inputText = strings.ReplaceAll(inputText, "<@!"+botUserID+">", "")
		inputText = strings.TrimSpace(inputText)
	}

	// ── Voice & Image Attachments ───────────────────────────────────────
	// Discord sends voice messages and images as attachments on the message.
	for _, att := range m.Attachments {
		if ctx.Err() != nil {
			return
		}
		inputText = processDiscordAttachmentContext(ctx, att, inputText, cfg, logger)
	}

	if ctx.Err() != nil || inputText == "" {
		return
	}

	if tools.HasPendingQuestion(sessionID) {
		if response, ok := tools.ResolveQuestionReply(sessionID, inputText); ok {
			response.Source = tools.QuestionSourceDiscord
			tools.CompleteQuestion(sessionID, response)
			return
		}
		sendDiscordNotice(s, m.ChannelID, "Please reply with one of the listed numbers.", logger)
		return
	}

	logger.Info("[Discord] Processing message", "user", m.Author.Username, "channel", m.ChannelID, "isDM", decision.IsDM, "trigger", decision.Reason)

	// Slash command interception
	if strings.HasPrefix(inputText, "/") {
		cmdCtx := commands.Context{
			STM:           shortTermMem,
			SessionID:     sessionID,
			Vault:         vault,
			InventoryDB:   inventoryDB,
			Cfg:           cfg,
			PromptsDir:    cfg.Directories.PromptsDir,
			AllowOperator: decision.IsDM,
		}
		cmdResult, isCmd, err := commands.Handle(inputText, cmdCtx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			logger.Error("[Discord] Command execution failed", "error", err)
			sendDiscordNotice(s, m.ChannelID, "⚠️ Command execution failed.", logger)
			return
		}
		if isCmd {
			SendMessage(m.ChannelID, cmdResult, logger)
			return
		}
	}

	// A message that forwards or replies to someone else's message gets a fixed
	// marker inside the isolated block; the scan below reads the author name too.
	if marker := discordReferenceMarker(m.Message, botUserID); marker != "" {
		inputText = marker + "\n" + inputText
	}

	// Security: Block prompt injection attempts (not just log them)
	if guardian != nil {
		if scan := guardian.ScanForInjection(inputText); scan.Level >= security.ThreatHigh {
			logger.Warn("[Discord] Prompt injection BLOCKED — message discarded",
				"user", m.Author.Username, "level", scan.Level, "patterns", scan.Patterns)
			sendDiscordNotice(s, m.ChannelID, "⚠️ Your message was blocked by the security guardian.", logger)
			return
		}
	}
	inputText = security.IsolateExternalData(inputText)
	if ctx.Err() != nil {
		return
	}

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	// Set up a goroutine to keep typing indicator active
	typingCtx, stopTyping := context.WithCancel(ctx)
	defer stopTyping()
	go func() {
		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.ChannelTyping(m.ChannelID)
			case <-typingCtx.Done():
				return
			}
		}
	}()

	// Process through the agent
	processDiscordMessage(ctx, sessionID, s, m, inputText, cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, kg, inventoryDB, missionManagerV2, remoteHub, budgetTracker, webhookManager, routingDecisions...)
}

// processDiscordAttachment handles a single Discord attachment with proper cleanup.
// Returns the updated inputText.
func processDiscordAttachment(att *discordgo.MessageAttachment, inputText string, cfg *config.Config, logger *slog.Logger) string {
	return processDiscordAttachmentContext(context.Background(), att, inputText, cfg, logger)
}

func processDiscordAttachmentContext(ctx context.Context, att *discordgo.MessageAttachment, inputText string, cfg *config.Config, logger *slog.Logger) string {
	if ctx.Err() != nil {
		return inputText
	}
	const maxAttachmentBytes = 50 * 1024 * 1024 // 50 MB
	if att.Size > maxAttachmentBytes {
		logger.Warn("[Discord] Attachment too large, skipping", "filename", att.Filename, "size_bytes", att.Size, "limit_bytes", maxAttachmentBytes)
		return inputText
	}

	// Voice / Audio attachment
	if media.IsAudioContentType(att.ContentType) || media.IsAudioFilename(att.Filename) {
		logger.Info("[Discord] Audio attachment detected", "filename", att.Filename, "content_type", att.ContentType, "size", att.Size)

		audioPath, err := media.DownloadFileContext(ctx, att.URL, "discord_voice")
		if err != nil {
			logger.Error("[Discord] Failed to download audio", "error", err)
			return inputText
		}
		defer os.Remove(audioPath)

		mp3Path := audioPath + ".mp3"
		if err := telegram.ConvertOggToMp3Context(ctx, audioPath, mp3Path); err != nil {
			logger.Error("[Discord] Failed to convert audio to mp3", "error", err)
			return inputText
		}
		defer os.Remove(mp3Path)
		if ctx.Err() != nil {
			return inputText
		}

		audioData, err := os.ReadFile(mp3Path)
		if err != nil {
			logger.Error("[Discord] Failed to read converted audio", "error", err)
			return inputText
		}
		text, _, err := tools.TranscribeAudio(ctx, filepath.Base(mp3Path), audioData, cfg)
		if err != nil {
			logger.Error("[Discord] Voice transcription failed", "error", err)
			return inputText
		}

		logger.Info("[Discord] Voice transcription successful", "text_len", len(text))
		if inputText != "" {
			return text + "\n" + inputText
		}
		return text
	}

	// Image attachment
	if media.IsImageContentType(att.ContentType) || media.IsImageFilename(att.Filename) {
		logger.Info("[Discord] Image attachment detected", "filename", att.Filename, "content_type", att.ContentType)

		imgPath, err := media.DownloadFileContext(ctx, att.URL, "discord_img")
		if err != nil {
			logger.Error("[Discord] Failed to download image", "error", err)
			return inputText
		}
		defer os.Remove(imgPath)

		analysis, err := telegram.AnalyzeImageContext(ctx, imgPath, cfg)
		if err != nil {
			logger.Error("[Discord] Image analysis failed", "error", err)
			if errors.Is(err, tools.ErrVisionPublicURLRequired) {
				analysis = "[Image analysis unavailable: " + tools.VisionPublicURLRequiredMessage + "]"
			} else {
				analysis = "[Error analyzing image]"
			}
		}

		logger.Info("[Discord] Image analysis successful", "length", len(analysis))
		if inputText != "" {
			return "[USER SENT AN IMAGE]\n" + analysis + "\n\n[USER TEXT]:\n" + inputText
		}
		return "[USER SENT AN IMAGE]\n" + analysis
	}

	// Generic file / document attachment
	logger.Info("[Discord] File attachment detected", "filename", att.Filename, "content_type", att.ContentType)
	attachDir := filepath.Join(cfg.Directories.WorkspaceDir, "attachments")
	savedPath, err := media.SaveAttachmentContext(ctx, att.URL, att.Filename, attachDir)
	if err != nil {
		logger.Error("[Discord] Failed to save attachment", "error", err)
		return inputText
	}
	agentPath := "agent_workspace/workdir/attachments/" + filepath.Base(savedPath)
	fileNote := "[FILE ATTACHED]: " + agentPath
	if att.ContentType != "" {
		fileNote += " (" + att.ContentType + ")"
	}
	if inputText != "" {
		return inputText + "\n\n" + fileNote
	}
	return fileNote
}

func processDiscordMessage(ctx context.Context, sessionID string, s *discordgo.Session, m *discordgo.MessageCreate, inputText string, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, remoteHub *remote.RemoteHub, budgetTracker *budget.Tracker, webhookManager *webhooks.Manager, routingDecisions ...agent.TaskDecisionService) {
	humanInput := memory.NewHumanInputEvent(m.Content, sessionID, "discord:"+m.ID, m.Timestamp)
	if !m.Timestamp.IsZero() {
		humanInput.OccurredAt = m.Timestamp.UTC()
	}
	releaseTurn, err := shortTermMem.LockSessionTurn(ctx, sessionID)
	if err != nil {
		return
	}
	defer releaseTurn()
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)

	// Add message to history
	_, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, inputText, false, false)
	if err != nil {
		logger.Error("[Discord] Failed to store message", "error", err)
		return
	}

	// Build RunConfig first so it can be used for prompt flag derivation
	routingText := m.Content
	if s != nil && s.State != nil && s.State.User != nil {
		routingText = strings.ReplaceAll(routingText, "<@"+s.State.User.ID+">", "")
		routingText = strings.ReplaceAll(routingText, "<@!"+s.State.User.ID+">", "")
	}
	var decisionService agent.TaskDecisionService
	if len(routingDecisions) > 0 {
		decisionService = routingDecisions[0]
	}
	runCfg := agent.RunConfig{
		RoutingDecisions:   decisionService,
		TaskRoutingText:    &routingText,
		HumanInput:         humanInput,
		Config:             cfg,
		Logger:             logger,
		LLMClient:          client,
		ShortTermMem:       shortTermMem,
		HistoryManager:     nil,
		LongTermMem:        longTermMem,
		KG:                 kg,
		InventoryDB:        inventoryDB,
		RemoteHub:          remoteHub,
		Vault:              vault,
		Registry:           registry,
		Manifest:           manifest,
		CronManager:        cronManager,
		MissionManagerV2:   missionManagerV2,
		WebhookManager:     webhookManager,
		CoAgentRegistry:    nil,
		BudgetTracker:      budgetTracker,
		PreparationService: nil,
		SessionID:          sessionID,
		MessageSource:      "discord",
		VoiceOutputActive:  agent.GetVoiceMode(),
	}

	// Assemble final messages for LLM
	finalMessages := buildDiscordAgentMessages(nil)
	recent, err := shortTermMem.GetRecentMessages(sessionID, 60)
	if err != nil {
		logger.Error("[Discord] Failed to load conversation", "error", err)
		return
	}
	for _, msg := range recent {
		if msg.Role == openai.ChatMessageRoleUser || msg.Role == openai.ChatMessageRoleAssistant {
			finalMessages = append(finalMessages, msg)
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	// Run the agent loop
	broker := &DiscordBroker{
		session:   s,
		channelID: m.ChannelID,
		logger:    logger,
	}

	resp, err := agent.ExecuteAgentLoop(ctx, req, runCfg, false, broker)
	if ctx.Err() != nil {
		return
	}

	if err != nil {
		logger.Error("[Discord] Agent loop failed", "error", err)
		sendDiscordNotice(s, m.ChannelID, "⚠️ Sorry, I encountered an error processing your request.", logger)
		return
	}

	// Send captured audio files as native Discord audio messages
	for _, af := range broker.AudioFiles {
		if ctx.Err() != nil {
			return
		}
		if err := SendDiscordAudio(m.ChannelID, af.FilePath, af.Title, logger); err != nil {
			logger.Warn("[Discord] Failed to send audio", "path", af.FilePath, "error", err)
		}
	}

	// Send result back to Discord
	if len(resp.Choices) > 0 {
		answer := security.StripThinkingTags(resp.Choices[0].Message.Content)
		// Extract markdown images and send as native Discord attachments
		cleanText, images := media.ExtractMarkdownImages(answer)
		for _, img := range images {
			if ctx.Err() != nil {
				return
			}
			var localPath string
			staged := false
			if strings.HasPrefix(img.URL, "/files/") {
				// Local workspace file
				var err error
				localPath, err = media.StageWorkspaceImage(cfg.Directories.WorkspaceDir, img.URL)
				if err != nil {
					logger.Warn("Workspace image rejected", "error", err)
					continue
				}
				staged = true
			} else if strings.HasPrefix(img.URL, "http://") || strings.HasPrefix(img.URL, "https://") {
				// Remote URL: download and sanitize before sending
				imagesDir := filepath.Join(cfg.Directories.WorkspaceDir, "images")
				sanitized, err := media.DownloadAndSanitizeImageContext(ctx, img.URL, imagesDir)
				if err != nil {
					logger.Warn("[Discord] Failed to download/sanitize image URL", "url", img.URL, "error", err)
					continue
				}
				localPath = sanitized
			} else {
				continue
			}
			if ctx.Err() != nil {
				if staged {
					_ = os.Remove(localPath)
				}
				return
			}
			if err := SendDiscordImage(m.ChannelID, localPath, img.Caption, logger); err != nil {
				logger.Warn("[Discord] Failed to send image", "path", localPath, "error", err)
			}
			if staged {
				_ = os.Remove(localPath)
			}
		}
		cleanText = telegram.AppendMissingYouTubeLinks(cleanText, broker.YouTubeVideos)
		if cleanText != "" && ctx.Err() == nil {
			if err := SendMessage(m.ChannelID, cleanText, logger); err != nil {
				markDeliveryFailure(time.Now())
				logger.Error("[Discord] Failed to send response", "error", err)
			}
		}
	}
}

// ── Discord Broker (implements agent.FeedbackBroker) ────────────────────────

// DiscordBroker implements agent.FeedbackBroker for real-time Discord feedback.
type DiscordBroker struct {
	session       *discordgo.Session
	channelID     string
	logger        *slog.Logger
	AudioFiles    []telegram.CapturedAudio
	YouTubeVideos []telegram.CapturedYouTubeVideo
}

func (b *DiscordBroker) Send(event, message string) {
	if event == "question_user" {
		b.deliver(message, "question")
		return
	}
	// Capture audio events for native sending after the loop
	if event == "audio" {
		var audio struct {
			FilePath string `json:"file_path"`
			Title    string `json:"title"`
			MimeType string `json:"mime_type"`
			Filename string `json:"filename"`
		}
		if json.Unmarshal([]byte(message), &audio) == nil && audio.FilePath != "" {
			b.AudioFiles = append(b.AudioFiles, telegram.CapturedAudio{
				FilePath: audio.FilePath,
				Title:    audio.Title,
				MimeType: audio.MimeType,
				Filename: audio.Filename,
			})
		}
		return
	}
	if event == "youtube_video" {
		var video struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			VideoID string `json:"video_id"`
		}
		if json.Unmarshal([]byte(message), &video) == nil && video.URL != "" {
			b.YouTubeVideos = append(b.YouTubeVideos, telegram.CapturedYouTubeVideo{
				URL:     video.URL,
				Title:   video.Title,
				VideoID: video.VideoID,
			})
		}
		return
	}
	// Only send high-level status events to avoid spamming
	if event == "tool_start" || event == "error_recovery" || event == "api_retry" {
		b.logger.Info("[Discord Status]", "event", event, "message", message)
		text := fmt.Sprintf("⚙️ **%s**: %s", strings.ToUpper(event), message)
		b.deliver(text, "status event")
	}
	if event == "budget_warning" {
		b.deliver("⚠️ "+message, "budget warning")
	}
	if event == "budget_blocked" {
		b.deliver("🚫 "+message, "budget blocked message")
	}
}

// deliver sends a message to the channel and records a failed delivery. A lost
// message is invisible to the user, so it is logged and counted in the health
// endpoint.
func (b *DiscordBroker) deliver(text, what string) {
	if _, err := b.session.ChannelMessageSendComplex(b.channelID, agentTextMessage(text)); err != nil {
		markDeliveryFailure(time.Now())
		if b.logger != nil {
			b.logger.Warn("[Discord] Failed to send "+what, "error", err)
		}
	}
}

func (b *DiscordBroker) SendJSON(jsonStr string) {
	// Token usage etc. — skip for Discord
}

func (b *DiscordBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}

func (b *DiscordBroker) SendLLMStreamDone(finishReason string) {}

func (b *DiscordBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
}

func (b *DiscordBroker) SendThinkingBlock(provider, content, state string) {
}
