package discord

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/media"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/telegram"
	"aurago/internal/tools"

	"github.com/bwmarrin/discordgo"
	"github.com/sashabaranov/go-openai"
)

// session holds the active Discord session so tools can send messages.
var (
	session   *discordgo.Session
	sessionMu sync.RWMutex
)

func setSession(s *discordgo.Session) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	session = s
}

// StartBot initializes the Discord bot and begins listening for messages.
func StartBot(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, guardian *security.Guardian) {
	if !cfg.Discord.Enabled || cfg.Discord.BotToken == "" {
		if cfg.Discord.Enabled {
			logger.Warn("[Discord] Bot token is missing, skipping Discord start.")
		}
		return
	}

	dg, err := discordgo.New("Bot " + cfg.Discord.BotToken)
	if err != nil {
		logger.Error("[Discord] Failed to create session", "error", err)
		return
	}

	// Only subscribe to message events for efficiency
	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsDirectMessages | discordgo.IntentsMessageContent

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		handleMessage(s, m, cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, missionManagerV2, guardian)
	})

	if err := dg.Open(); err != nil {
		logger.Error("[Discord] Failed to open websocket connection", "error", err)
		return
	}

	setSession(dg)
	logger.Info("[Discord] Bot connected", "user", dg.State.User.Username+"#"+dg.State.User.Discriminator)

	// Register bridge functions so agent can call Discord without import cycles
	tools.RegisterDiscordBridge(
		SendMessage,
		func(channelID string, limit int, logger *slog.Logger) ([]tools.DiscordMessageInfo, error) {
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
			var result []tools.DiscordChannelInfo
			for _, ch := range chs {
				result = append(result, tools.DiscordChannelInfo{
					ID:   ch.ID,
					Name: ch.Name,
				})
			}
			return result, nil
		},
	)
}

// GetSession returns the active Discord session (for sending messages from tools).
func GetSession() *discordgo.Session {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	return session
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
		if len(chunk) > 1990 {
			// Try to cut at a newline
			cutAt := strings.LastIndex(chunk[:1990], "\n")
			if cutAt < 500 {
				cutAt = 1990
			}
			chunk = content[:cutAt]
			content = content[cutAt:]
		} else {
			content = ""
		}

		if _, err := s.ChannelMessageSend(channelID, chunk); err != nil {
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

func handleMessage(s *discordgo.Session, m *discordgo.MessageCreate, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, guardian *security.Guardian) {
	// Ignore own messages
	if m.Author.ID == s.State.User.ID {
		return
	}

	allowedUserID := strings.TrimSpace(cfg.Discord.AllowedUserID)
	if allowedUserID == "" {
		logger.Warn("[Discord] Blocking message because allowed_user_id is not configured", "user_id", m.Author.ID, "username", m.Author.Username)
		return
	}
	if allowedUserID == "0" {
		logger.Warn("[Discord] Discovery mode: blocked message from user", "user_id", m.Author.ID, "username", m.Author.Username)
		return
	}
	if m.Author.ID != allowedUserID {
		logger.Warn("[Discord] Blocked unauthorized Discord message", "user_id", m.Author.ID)
		return
	}

	// [Guild filter] — if guild_id is set, only accept messages from that guild
	if cfg.Discord.GuildID != "" && m.GuildID != cfg.Discord.GuildID {
		// Allow DMs (GuildID == "") through
		if m.GuildID != "" {
			return
		}
	}

	inputText := m.Content
	hasAttachments := len(m.Attachments) > 0
	if inputText == "" && !hasAttachments {
		return
	}

	// Bot must be mentioned or DMed to respond (avoid responding to every message)
	isDM := m.GuildID == ""
	isMentioned := false
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			isMentioned = true
			break
		}
	}

	if !isDM && !isMentioned {
		return // Only respond when mentioned or in DMs
	}

	// Strip the bot mention from the message text
	if isMentioned {
		inputText = strings.ReplaceAll(inputText, "<@"+s.State.User.ID+">", "")
		inputText = strings.ReplaceAll(inputText, "<@!"+s.State.User.ID+">", "")
		inputText = strings.TrimSpace(inputText)
	}

	// ── Voice & Image Attachments ───────────────────────────────────────
	// Discord sends voice messages and images as attachments on the message.
	for _, att := range m.Attachments {
		inputText = processDiscordAttachment(att, inputText, cfg, logger)
	}

	if inputText == "" {
		return
	}

	logger.Info("[Discord] Processing message", "user", m.Author.Username, "channel", m.ChannelID, "isDM", isDM)

	// Slash command interception
	if strings.HasPrefix(inputText, "/") {
		cmdCtx := commands.Context{
			STM:         shortTermMem,
			HM:          historyManager,
			Vault:       vault,
			InventoryDB: inventoryDB,
			Cfg:         cfg,
			PromptsDir:  cfg.Directories.PromptsDir,
		}
		cmdResult, isCmd, err := commands.Handle(inputText, cmdCtx)
		if err != nil {
			logger.Error("[Discord] Command execution failed", "error", err)
			s.ChannelMessageSend(m.ChannelID, "⚠️ Command execution failed.")
			return
		}
		if isCmd {
			SendMessage(m.ChannelID, cmdResult, logger)
			return
		}
	}

	if guardian != nil {
		if scan := guardian.ScanForInjection(inputText); scan.Level >= security.ThreatHigh {
			logger.Warn("[Discord] Prompt injection detected in message",
				"user", m.Author.Username, "level", scan.Level, "patterns", scan.Patterns)
		}
	}
	inputText = security.IsolateExternalData(inputText)

	// Show typing indicator
	s.ChannelTyping(m.ChannelID)

	// Set up a goroutine to keep typing indicator active
	typingCtx, stopTyping := context.WithCancel(context.Background())
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
	processDiscordMessage(s, m, inputText, cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, missionManagerV2)
	stopTyping()
}

// processDiscordAttachment handles a single Discord attachment with proper cleanup.
// Returns the updated inputText.
func processDiscordAttachment(att *discordgo.MessageAttachment, inputText string, cfg *config.Config, logger *slog.Logger) string {
	const maxAttachmentBytes = 50 * 1024 * 1024 // 50 MB
	if att.Size > maxAttachmentBytes {
		logger.Warn("[Discord] Attachment too large, skipping", "filename", att.Filename, "size_bytes", att.Size, "limit_bytes", maxAttachmentBytes)
		return inputText
	}

	// Voice / Audio attachment
	if media.IsAudioContentType(att.ContentType) || media.IsAudioFilename(att.Filename) {
		logger.Info("[Discord] Audio attachment detected", "filename", att.Filename, "content_type", att.ContentType, "size", att.Size)

		audioPath, err := media.DownloadFile(att.URL, "discord_voice")
		if err != nil {
			logger.Error("[Discord] Failed to download audio", "error", err)
			return inputText
		}
		defer os.Remove(audioPath)

		mp3Path := audioPath + ".mp3"
		if err := telegram.ConvertOggToMp3(audioPath, mp3Path); err != nil {
			logger.Error("[Discord] Failed to convert audio to mp3", "error", err)
			return inputText
		}
		defer os.Remove(mp3Path)

		text, err := telegram.TranscribeMultimodal(mp3Path, cfg)
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

		imgPath, err := media.DownloadFile(att.URL, "discord_img")
		if err != nil {
			logger.Error("[Discord] Failed to download image", "error", err)
			return inputText
		}
		defer os.Remove(imgPath)

		analysis, err := telegram.AnalyzeImage(imgPath, cfg)
		if err != nil {
			logger.Error("[Discord] Image analysis failed", "error", err)
			analysis = "[Error analyzing image]"
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
	savedPath, err := media.SaveAttachment(att.URL, att.Filename, attachDir)
	if err != nil {
		logger.Error("[Discord] Failed to save attachment", "error", err)
		return inputText
	}
	agentPath := "agent_workspace/workdir/attachments/" + filepath.Base(savedPath)
	fileNote := "[DATEI ANGEHÄNGT]: " + agentPath
	if att.ContentType != "" {
		fileNote += " (" + att.ContentType + ")"
	}
	if inputText != "" {
		return inputText + "\n\n" + fileNote
	}
	return fileNote
}

func processDiscordMessage(s *discordgo.Session, m *discordgo.MessageCreate, inputText string, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2) {
	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	sessionID := "default"

	// Add message to history
	mid, _ := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, inputText, false, false)
	if sessionID == "default" {
		historyManager.Add(openai.ChatMessageRoleUser, inputText, mid, false, false)
	}

	// Build RunConfig first so it can be used for prompt flag derivation
	runCfg := agent.RunConfig{
		Config:            cfg,
		Logger:            logger,
		LLMClient:         client,
		ShortTermMem:      shortTermMem,
		HistoryManager:    historyManager,
		LongTermMem:       longTermMem,
		KG:                kg,
		InventoryDB:       inventoryDB,
		Vault:             vault,
		Registry:          registry,
		Manifest:          manifest,
		CronManager:       cronManager,
		MissionManagerV2:  missionManagerV2,
		CoAgentRegistry:    nil,
		BudgetTracker:      nil,
		PreparationService: nil,
		SessionID:          sessionID,
		IsMaintenance:     tools.IsBusy(),
		MessageSource:     "discord",
		VoiceOutputActive: agent.GetVoiceMode(),
	}

	// Build context flags via central factory
	toolingPolicy := agent.BuildToolingPolicy(cfg, inputText)
	flags := agent.BuildPromptContextFlags(runCfg, toolingPolicy, agent.PromptContextOptions{
		IsMaintenanceMode:     tools.IsBusy(),
		ActiveProcesses:       agent.GetActiveProcessStatus(registry),
		SpecialistsAvailable:  agent.BuildSpecialistsAvailable(cfg),
		SpecialistsStatus:     agent.BuildSpecialistsStatus(cfg),
		SpecialistsSuggestion: agent.BuildSpecialistDelegationHint(cfg, inputText),
	})
	coreMem := shortTermMem.ReadCoreMemory()

	sysPrompt := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, flags, coreMem, logger)

	// Assemble final messages for LLM
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
	}

	currentSummary := historyManager.GetSummary()
	if currentSummary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + currentSummary,
		})
	}
	finalMessages = append(finalMessages, historyManager.Get()...)

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	// Run the agent loop
	ctx := context.Background()
	broker := &DiscordBroker{
		session:   s,
		channelID: m.ChannelID,
		logger:    logger,
	}

	resp, err := agent.ExecuteAgentLoop(ctx, req, runCfg, false, broker)

	if err != nil {
		logger.Error("[Discord] Agent loop failed", "error", err)
		s.ChannelMessageSend(m.ChannelID, "⚠️ Sorry, I encountered an error processing your request.")
		return
	}

	// Send captured audio files as native Discord audio messages
	for _, af := range broker.AudioFiles {
		if err := SendDiscordAudio(m.ChannelID, af.FilePath, af.Title, logger); err != nil {
			logger.Warn("[Discord] Failed to send audio", "path", af.FilePath, "error", err)
		}
	}

	// Send result back to Discord
	if len(resp.Choices) > 0 {
		answer := security.StripThinkingTags(resp.Choices[0].Message.Content)
		if answer != "" {
			// Extract markdown images and send as native Discord attachments
			cleanText, images := media.ExtractMarkdownImages(answer)
			for _, img := range images {
				var localPath string
				if strings.HasPrefix(img.URL, "/files/") {
					// Local workspace file
					localPath = filepath.Join(cfg.Directories.WorkspaceDir, strings.TrimPrefix(img.URL, "/files/"))
				} else if strings.HasPrefix(img.URL, "http://") || strings.HasPrefix(img.URL, "https://") {
					// Remote URL: download and sanitize before sending
					imagesDir := filepath.Join(cfg.Directories.WorkspaceDir, "images")
					sanitized, err := media.DownloadAndSanitizeImage(img.URL, imagesDir)
					if err != nil {
						logger.Warn("[Discord] Failed to download/sanitize image URL", "url", img.URL, "error", err)
						continue
					}
					localPath = sanitized
				} else {
					continue
				}
				if err := SendDiscordImage(m.ChannelID, localPath, img.Caption, logger); err != nil {
					logger.Warn("[Discord] Failed to send image", "path", localPath, "error", err)
				}
			}
			if cleanText != "" {
				if err := SendMessage(m.ChannelID, cleanText, logger); err != nil {
					logger.Error("[Discord] Failed to send response", "error", err)
				}
			}
		}
	}
}

// ── Discord Broker (implements agent.FeedbackBroker) ────────────────────────

// DiscordBroker implements agent.FeedbackBroker for real-time Discord feedback.
type DiscordBroker struct {
	session    *discordgo.Session
	channelID  string
	logger     *slog.Logger
	AudioFiles []telegram.CapturedAudio
}

func (b *DiscordBroker) Send(event, message string) {
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
	// Only send high-level status events to avoid spamming
	if event == "tool_start" || event == "error_recovery" || event == "api_retry" {
		b.logger.Info("[Discord Status]", "event", event, "message", message)
		text := fmt.Sprintf("⚙️ **%s**: %s", strings.ToUpper(event), message)
		b.session.ChannelMessageSend(b.channelID, text)
	}
	if event == "budget_warning" {
		b.session.ChannelMessageSend(b.channelID, "⚠️ "+message)
	}
	if event == "budget_blocked" {
		b.session.ChannelMessageSend(b.channelID, "🚫 "+message)
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
