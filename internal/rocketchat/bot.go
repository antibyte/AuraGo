package rocketchat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"aurago/internal/agent"
	"aurago/internal/commands"
	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// rcHTTPClient is a shared HTTP client for Rocket.Chat REST API calls.
var rcHTTPClient = &http.Client{Timeout: 30 * time.Second}

// StartBot initializes the Rocket.Chat bot and begins polling for new messages.
func StartBot(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB) {
	if !cfg.RocketChat.Enabled {
		return
	}
	if cfg.RocketChat.URL == "" || cfg.RocketChat.AuthToken == "" || cfg.RocketChat.UserID == "" {
		logger.Warn("[RocketChat] Missing URL, user_id, or auth_token — skipping start")
		return
	}

	logger.Info("[RocketChat] Bot starting", "url", cfg.RocketChat.URL, "channel", cfg.RocketChat.Channel)

	go pollLoop(cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB)
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
func pollLoop(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB) {
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
	pollInterval := 3 * time.Second

	for {
		time.Sleep(pollInterval)

		messages, err := fetchNewMessages(cfg, channelID, lastTS)
		if err != nil {
			logger.Error("[RocketChat] Poll failed", "error", err)
			continue
		}

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

			logger.Info("[RocketChat] Received message", "user", msg.User.Username, "text_len", len(msg.Msg))

			go processMessage(cfg, logger, client, shortTermMem, longTermMem, vault, registry, cronManager, historyManager, kg, inventoryDB, channelID, msg)
		}
	}
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
func processMessage(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, shortTermMem *memory.SQLiteMemory, longTermMem memory.VectorDB, vault *security.Vault, registry *tools.ProcessRegistry, cronManager *tools.CronManager, historyManager *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, channelID string, msg message) {
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

	manifest := tools.NewManifest(cfg.Directories.ToolsDir)
	sessionID := "default"

	// Add message to history
	mid, _ := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, inputText, false, false)
	if sessionID == "default" {
		historyManager.Add(openai.ChatMessageRoleUser, inputText, mid, false, false)
	}

	// Build context flags
	flags := prompts.ContextFlags{
		ActiveProcesses:          agent.GetActiveProcessStatus(registry),
		IsMaintenanceMode:        tools.IsBusy(),
		LifeboatEnabled:          cfg.Maintenance.LifeboatEnabled,
		SystemLanguage:           cfg.Agent.SystemLanguage,
		CorePersonality:          cfg.Agent.CorePersonality,
		TokenBudget:              cfg.Agent.SystemPromptTokenBudget,
		IsDebugMode:              cfg.Agent.DebugMode,
		DiscordEnabled:           cfg.Discord.Enabled,
		EmailEnabled:             cfg.Email.Enabled,
		DockerEnabled:            cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
		WebDAVEnabled:            cfg.WebDAV.Enabled,
		KoofrEnabled:             cfg.Koofr.Enabled,
		ChromecastEnabled:        cfg.Chromecast.Enabled,
		CoAgentEnabled:           cfg.CoAgents.Enabled,
		GoogleWorkspaceEnabled:   cfg.GoogleWorkspace.Enabled,
		ProxmoxEnabled:           cfg.Proxmox.Enabled,
		OllamaEnabled:            cfg.Ollama.Enabled,
		TailscaleEnabled:         cfg.Tailscale.Enabled,
		CloudflareTunnelEnabled:  cfg.CloudflareTunnel.Enabled,
		AnsibleEnabled:           cfg.Ansible.Enabled,
		GitHubEnabled:            cfg.GitHub.Enabled,
		MQTTEnabled:              cfg.MQTT.Enabled,
		AdGuardEnabled:           cfg.AdGuard.Enabled,
		MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:           cfg.Sandbox.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		MeshCentralEnabled:       cfg.MeshCentral.Enabled,
		HomepageEnabled:          cfg.Homepage.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK || cfg.Homepage.AllowLocalServer),
		HomepageAllowLocalServer: cfg.Homepage.AllowLocalServer,
		NetlifyEnabled:           cfg.Netlify.Enabled,
		VirusTotalEnabled:        cfg.VirusTotal.Enabled,
		BraveSearchEnabled:       cfg.BraveSearch.Enabled,
		MemoryEnabled:            cfg.Tools.Memory.Enabled,
		KnowledgeGraphEnabled:    cfg.Tools.KnowledgeGraph.Enabled,
		SecretsVaultEnabled:      cfg.Tools.SecretsVault.Enabled,
		SchedulerEnabled:         cfg.Tools.Scheduler.Enabled,
		NotesEnabled:             cfg.Tools.Notes.Enabled,
		MissionsEnabled:          cfg.Tools.Missions.Enabled,
		StopProcessEnabled:       cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:         cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled: cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:               cfg.Tools.WOL.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.BroadcastOK),
		AllowShell:               cfg.Agent.AllowShell,
		AllowPython:              cfg.Agent.AllowPython,
		AllowFilesystemWrite:     cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:     cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:         cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:          cfg.Agent.AllowSelfUpdate,
		IsEgg:                    cfg.EggMode.Enabled,
		InternetExposed:          cfg.Server.HTTPS.Enabled,
		IsDocker:                 cfg.Runtime.IsDocker,
		AdditionalPrompt:         cfg.Agent.AdditionalPrompt,
	}

	coreMem := shortTermMem.ReadCoreMemory()
	sysPrompt := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, flags, coreMem, logger)

	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
	}

	currentSummary := historyManager.GetSummary()
	if currentSummary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "[CONTEXT_RECAP]: " + currentSummary,
		})
	}
	finalMessages = append(finalMessages, historyManager.Get()...)

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	if cfg.LLM.UseNativeFunctions {
		ff := agent.ToolFeatureFlags{
			HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
			DockerEnabled:            cfg.Docker.Enabled && cfg.Runtime.DockerSocketOK,
			CoAgentEnabled:           cfg.CoAgents.Enabled,
			SudoEnabled:              cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker,
			WebhooksEnabled:          cfg.Webhooks.Enabled,
			ProxmoxEnabled:           cfg.Proxmox.Enabled,
			OllamaEnabled:            cfg.Ollama.Enabled,
			HomepageEnabled:          cfg.Homepage.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK || cfg.Homepage.AllowLocalServer),
			HomepageAllowLocalServer: cfg.Homepage.AllowLocalServer,
			NetlifyEnabled:           cfg.Netlify.Enabled,
			AdGuardEnabled:           cfg.AdGuard.Enabled,
			GoogleWorkspaceEnabled:   cfg.GoogleWorkspace.Enabled,
		}
		ntSchemas := agent.BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, logger)
		req.Tools = ntSchemas
		req.ToolChoice = "auto"
	}

	// Send to LLM
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)
	defer cancel()

	response, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		logger.Error("[RocketChat] LLM call failed", "error", err)
		_ = SendMessage(cfg, channelID, "⚠️ Fehler beim Verarbeiten der Anfrage.")
		return
	}

	if len(response.Choices) > 0 {
		reply := response.Choices[0].Message.Content
		if reply != "" {
			// Store assistant reply
			replyID, _ := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, reply, false, false)
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, reply, replyID, false, false)
			}

			// Send reply to RocketChat
			if err := SendMessage(cfg, channelID, reply); err != nil {
				logger.Error("[RocketChat] Failed to send reply", "error", err)
			}
		}
	}
}
