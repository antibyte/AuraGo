package rocketchat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// rcHTTPClient is a shared HTTP client for Rocket.Chat REST API calls.
var rcHTTPClient = &http.Client{Timeout: 30 * time.Second}

// StartBot initializes the Rocket.Chat bot and begins polling for new messages.
func StartBot(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, guardian *security.Guardian) {
	if !cfg.RocketChat.Enabled {
		return
	}
	if cfg.RocketChat.URL == "" || cfg.RocketChat.AuthToken == "" || cfg.RocketChat.UserID == "" {
		logger.Warn("[RocketChat] Missing URL, user_id, or auth_token — skipping start")
		return
	}

	logger.Info("[RocketChat] Bot starting", "url", cfg.RocketChat.URL, "channel", cfg.RocketChat.Channel)

	go pollLoop(cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, missionManagerV2, guardian)
}

// rcRequest performs a REST API request against the Rocket.Chat server.
func rcRequest(cfg *config.Config, method, endpoint string, body string) ([]byte, int, error) {
	url := strings.TrimRight(cfg.RocketChat.URL, "/") + "/api/v1" + endpoint

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Auth-Token", cfg.RocketChat.AuthToken)
	req.Header.Set("X-User-Id", cfg.RocketChat.UserID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rcHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// SendMessage sends a text message to a Rocket.Chat channel.
func SendMessage(cfg *config.Config, channel, text string) error {
	text = security.Scrub(text)
	if channel == "" {
		channel = cfg.RocketChat.Channel
	}
	alias := cfg.RocketChat.Alias
	if alias == "" {
		alias = "AuraGo"
	}

	_, code, err := rcRequest(cfg, "POST", "/chat.sendMessage", fmt.Sprintf(`{"message":{"rid":%q,"msg":%q,"alias":%q}}`, channel, text, alias))
	if err != nil {
		return fmt.Errorf("send message failed: %w", err)
	}
	if code != 200 {
		return fmt.Errorf("send message returned HTTP %d", code)
	}
	return nil
}

// message represents a Rocket.Chat message from the API.
type message struct {
	ID   string `json:"_id"`
	RID  string `json:"rid"`
	Msg  string `json:"msg"`
	User struct {
		ID       string `json:"_id"`
		Username string `json:"username"`
	} `json:"u"`
	Timestamp struct {
		Date int64 `json:"$date"`
	} `json:"ts"`
}

// pollLoop continuously polls for new messages in the configured channel.
func pollLoop(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, guardian *security.Guardian) {
	channel := cfg.RocketChat.Channel
	if channel == "" {
		logger.Error("[RocketChat] No channel configured")
		return
	}

	// Resolve channel ID from channel name
	channelID, err := resolveChannelID(cfg, channel)
	if err != nil {
		logger.Error("[RocketChat] Failed to resolve channel", "channel", channel, "error", err)
		return
	}
	logger.Info("[RocketChat] Resolved channel", "name", channel, "id", channelID)

	lastTS := time.Now()
	basePollInterval := 3 * time.Second
	pollInterval := basePollInterval
	maxPollInterval := 30 * time.Second

	for {
		time.Sleep(pollInterval)

		messages, err := fetchNewMessages(cfg, channelID, lastTS)
		if err != nil {
			logger.Error("[RocketChat] Poll failed", "error", err)
			pollInterval *= 2
			if pollInterval > maxPollInterval {
				pollInterval = maxPollInterval
			}
			continue
		}
		pollInterval = basePollInterval

		for _, msg := range messages {
			// Skip bot's own messages
			if msg.User.ID == cfg.RocketChat.UserID {
				continue
			}
			// Skip empty
			if strings.TrimSpace(msg.Msg) == "" {
				continue
			}

			msgTime := time.UnixMilli(msg.Timestamp.Date)
			if msgTime.After(lastTS) {
				lastTS = msgTime
			}

			if !isAllowedRocketChatUser(cfg, msg) {
				logger.Warn("[RocketChat] Blocked unauthorized message", "user_id", msg.User.ID, "username", msg.User.Username)
				continue
			}

			logger.Info("[RocketChat] Received message", "user", msg.User.Username, "text_len", len(msg.Msg))

				go processMessage(cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, channelID, msg, missionManagerV2, guardian)
		}
	}
}

func isAllowedRocketChatUser(cfg *config.Config, msg message) bool {
	if len(cfg.RocketChat.AllowedUsers) == 0 {
		return false
	}
	for _, candidate := range []string{msg.User.ID, msg.User.Username} {
		if candidate != "" && slices.Contains(cfg.RocketChat.AllowedUsers, candidate) {
			return true
		}
	}
	return false
}

// resolveChannelID resolves a channel name to its ID.
func resolveChannelID(cfg *config.Config, channel string) (string, error) {
	// Try direct channels first
	data, code, err := rcRequest(cfg, "GET", "/channels.info?roomName="+channel, "")
	if err != nil {
		return "", err
	}
	if code == 200 {
		var resp struct {
			Channel struct {
				ID string `json:"_id"`
			} `json:"channel"`
		}
		if json.Unmarshal(data, &resp) == nil && resp.Channel.ID != "" {
			return resp.Channel.ID, nil
		}
	}
	// Maybe it's already an ID
	return channel, nil
}

// fetchNewMessages fetches messages newer than the given timestamp.
func fetchNewMessages(cfg *config.Config, channelID string, since time.Time) ([]message, error) {
	ts := since.UTC().Format("2006-01-02T15:04:05.000Z")
	endpoint := fmt.Sprintf("/channels.history?roomId=%s&oldest=%s&count=50", channelID, ts)
	data, code, err := rcRequest(cfg, "GET", endpoint, "")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", code, string(data))
	}
	var resp struct {
		Messages []message `json:"messages"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("JSON parse error: %w", err)
	}
	return resp.Messages, nil
}

// processMessage handles a single incoming Rocket.Chat message.
func processMessage(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, channelID string, msg message, missionManagerV2 *tools.MissionManagerV2, guardian *security.Guardian) {
	inputText := msg.Msg

	// Command interception
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
			logger.Error("[RocketChat] Command execution failed", "error", err)
			_ = SendMessage(cfg, channelID, "⚠️ Fehler beim Ausführen des Befehls.")
			return
		}
		if isCmd {
			_ = SendMessage(cfg, channelID, cmdResult)
			return
		}
	}

	if guardian != nil {
		if scan := guardian.ScanForInjection(inputText); scan.Level >= security.ThreatHigh {
			logger.Warn("[RocketChat] Prompt injection detected in message",
				"user", msg.User.Username, "level", scan.Level, "patterns", scan.Patterns)
		}
	}
	inputText = security.IsolateExternalData(inputText)

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
		SessionID:         sessionID,
		IsMaintenance:     tools.IsBusy(),
		MessageSource:     "rocketchat",
		VoiceOutputActive: agent.GetVoiceMode(),
	}
	finalMessages := historyManager.Get()
	if currentSummary := historyManager.GetSummary(); currentSummary != "" {
		finalMessages = append([]openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleSystem,
			Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + currentSummary,
		}}, finalMessages...)
	}

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	response, err := agent.ExecuteAgentLoop(ctx, req, runCfg, false, agent.NoopBroker{})
	if err != nil {
		logger.Error("[RocketChat] Agent loop failed", "error", err)
		_ = SendMessage(cfg, channelID, "⚠️ Fehler beim Verarbeiten der Anfrage.")
		return
	}

	if len(response.Choices) > 0 {
		reply := security.StripThinkingTags(response.Choices[0].Message.Content)
		if reply != "" {
			if err := SendMessage(cfg, channelID, reply); err != nil {
				logger.Error("[RocketChat] Failed to send reply", "error", err)
			}
		}
	}
}
