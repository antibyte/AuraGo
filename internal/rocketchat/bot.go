package rocketchat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/budget"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/remote"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// rcHTTPClient is a shared HTTP client for Rocket.Chat REST API calls.
var rcHTTPClient = &http.Client{Timeout: 30 * time.Second, CheckRedirect: security.SameOriginRedirect}

// rcRequest performs a REST API request against the Rocket.Chat server.
func rcRequest(cfg *config.Config, method, endpoint string, body string) ([]byte, int, error) {
	return rcRequestContext(context.Background(), cfg, method, endpoint, body)
}

func rcRequestContext(ctx context.Context, cfg *config.Config, method, endpoint string, body string) ([]byte, int, error) {
	if err := security.ValidateHTTPBaseURL(cfg.RocketChat.URL); err != nil {
		return nil, 0, err
	}
	security.RegisterSensitive(cfg.RocketChat.AuthToken)
	url := strings.TrimRight(cfg.RocketChat.URL, "/") + "/api/v1" + endpoint

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("X-Auth-Token", cfg.RocketChat.AuthToken)
	req.Header.Set("X-User-Id", cfg.RocketChat.UserID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := rcHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %s", security.Scrub(err.Error()))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if len(data) > 2<<20 {
		return nil, resp.StatusCode, fmt.Errorf("Rocket.Chat response exceeds 2 MiB")
	}
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// SendMessage sends a text message to a Rocket.Chat channel.
func SendMessage(cfg *config.Config, channel, text string) error {
	return sendMessageContext(context.Background(), cfg, channel, text)
}

func sendMessageContext(ctx context.Context, cfg *config.Config, channel, text string) error {
	text = security.Scrub(text)
	if channel == "" {
		channel = cfg.RocketChat.Channel
	}
	alias := cfg.RocketChat.Alias
	if alias == "" {
		alias = "AuraGo"
	}

	_, code, err := rcRequestContext(ctx, cfg, "POST", "/chat.sendMessage", fmt.Sprintf(`{"message":{"rid":%q,"msg":%q,"alias":%q}}`, channel, text, alias))
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
	Timestamp messageTimestamp `json:"ts"`
}

func isAllowedRocketChatUser(cfg *config.Config, msg message) bool {
	if len(cfg.RocketChat.AllowedUsers) == 0 {
		return false
	}
	id, username := msg.User.ID, msg.User.Username
	return id != "" && (slices.Contains(cfg.RocketChat.AllowedUsers, "id:"+id) ||
		slices.Contains(cfg.RocketChat.AllowedUsers, id)) ||
		username != "" && slices.Contains(cfg.RocketChat.AllowedUsers, "username:"+username)
}

// resolveChannelID resolves a channel name to its ID.
func resolveChannelID(ctx context.Context, cfg *config.Config, channel string) (string, error) {
	// Try direct channels first
	data, code, err := rcRequestContext(ctx, cfg, "GET", "/channels.info?roomName="+url.QueryEscape(channel), "")
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

// rocketChatSessionID keeps every room/sender pair in its own short-term
// memory, like Discord, instead of the shared "default" web/Telegram session.
func rocketChatSessionID(channelID, userID string) string {
	return "rocketchat:" + channelID + ":" + userID
}

// rocketChatSenderKey names the sender inside the session key: the immutable
// user id, or "username:<name>" when Rocket.Chat omits the id. Empty means the
// sender cannot be told apart from anyone else.
func rocketChatSenderKey(msg message) string {
	if msg.User.ID != "" {
		return msg.User.ID
	}
	if msg.User.Username != "" {
		return "username:" + msg.User.Username
	}
	return ""
}

// processMessage handles a single incoming Rocket.Chat message. The shared
// history manager belongs to the owner's "default" session and is
// deliberately unused: each room/sender pair has its own session.
func processMessage(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, _ *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, channelID string, msg message, missionManagerV2 *tools.MissionManagerV2, remoteHub *remote.RemoteHub, guardian *security.Guardian, budgetTrackerSnapshot func() *budget.Tracker) {
	if ctx.Err() != nil || !cfg.RocketChat.Enabled || !isAllowedRocketChatUser(cfg, msg) {
		return
	}
	inputText := msg.Msg
	sender := rocketChatSenderKey(msg)
	if sender == "" {
		logger.Warn("[RocketChat] Ignoring message without sender id or username")
		return
	}
	sessionID := rocketChatSessionID(channelID, sender)

	// Command interception
	if strings.HasPrefix(inputText, "/") {
		cmdCtx := commands.Context{
			STM:           shortTermMem,
			SessionID:     sessionID,
			Vault:         vault,
			InventoryDB:   inventoryDB,
			Cfg:           cfg,
			PromptsDir:    cfg.Directories.PromptsDir,
			AllowOperator: cfg.RocketChat.AllowOperatorCommands,
		}
		cmdResult, isCmd, err := commands.Handle(inputText, cmdCtx)
		if err != nil {
			logger.Error("[RocketChat] Command execution failed", "error", err)
			_ = sendMessageContext(ctx, cfg, channelID, "⚠️ Fehler beim Ausführen des Befehls.")
			return
		}
		if isCmd {
			_ = sendMessageContext(ctx, cfg, channelID, cmdResult)
			return
		}
	}

	if shouldBlockRocketChatPromptInjection(inputText, guardian, logger, msg.User.Username) {
		_ = sendMessageContext(ctx, cfg, channelID, "Your message was blocked by the security guardian.")
		return
	}
	inputText = security.IsolateExternalData(inputText)

	manifest := tools.NewManifest(cfg.Directories.ToolsDir)

	// Add message to this room/sender session
	if _, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, inputText, false, false); err != nil {
		logger.Error("[RocketChat] Failed to store message", "error", err)
		_ = sendMessageContext(ctx, cfg, channelID, "⚠️ Fehler beim Verarbeiten der Anfrage.")
		return
	}

	// Build RunConfig first so it can be used for prompt flag derivation
	var budgetTracker *budget.Tracker
	if budgetTrackerSnapshot != nil {
		budgetTracker = budgetTrackerSnapshot()
	}
	runCfg := agent.RunConfig{
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
		BudgetTracker:      budgetTracker,
		CronManager:        cronManager,
		MissionManagerV2:   missionManagerV2,
		PreparationService: nil,
		SessionID:          sessionID,
		IsMaintenance:      tools.IsBusy(),
		MessageSource:      "rocketchat",
		VoiceOutputActive:  agent.GetVoiceMode(),
	}

	// Same assembly as Discord: a system placeholder the agent loop fills,
	// then this session's own user/assistant turns from short-term memory.
	finalMessages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem}}
	recent, err := shortTermMem.GetRecentMessages(sessionID, 60)
	if err != nil {
		logger.Error("[RocketChat] Failed to load conversation", "error", err)
		_ = sendMessageContext(ctx, cfg, channelID, "⚠️ Fehler beim Laden des Gesprächsverlaufs.")
		return
	}
	for _, turn := range recent {
		if turn.Role == openai.ChatMessageRoleUser || turn.Role == openai.ChatMessageRoleAssistant {
			finalMessages = append(finalMessages, turn)
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	response, err := agent.ExecuteAgentLoop(ctx, req, runCfg, false, agent.NoopBroker{})
	if err != nil {
		logger.Error("[RocketChat] Agent loop failed", "error", err)
		_ = sendMessageContext(ctx, cfg, channelID, "⚠️ Fehler beim Verarbeiten der Anfrage.")
		return
	}

	if len(response.Choices) > 0 {
		reply := security.StripThinkingTags(response.Choices[0].Message.Content)
		if reply != "" {
			if err := sendMessageContext(ctx, cfg, channelID, reply); err != nil {
				logger.Error("[RocketChat] Failed to send reply", "error", err)
			}
		}
	}
}

func shouldBlockRocketChatPromptInjection(inputText string, guardian *security.Guardian, logger *slog.Logger, username string) bool {
	if guardian == nil {
		return false
	}
	scan := guardian.ScanForInjection(inputText)
	if scan.Level < security.ThreatHigh {
		return false
	}
	if logger != nil {
		logger.Warn("[RocketChat] Prompt injection BLOCKED - message discarded",
			"user", username, "level", scan.Level, "patterns", scan.Patterns)
	}
	return true
}
