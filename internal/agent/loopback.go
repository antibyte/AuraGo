package agent

import (
	"context"
	"time"

	"aurago/internal/prompts"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// Loopback injects an external message into the agent loop synchronously.
// Used by webhook-based integrations (e.g. Telnyx SMS) to relay incoming
// messages through the full agent pipeline including tool execution.
// The caller should invoke this in a goroutine for non-blocking operation.
func Loopback(runCfg RunConfig, message string, broker FeedbackBroker) {
	cfg := runCfg.Config
	logger := runCfg.Logger
	shortTermMem := runCfg.ShortTermMem
	historyManager := runCfg.HistoryManager

	if shortTermMem == nil || historyManager == nil || cfg == nil {
		if logger != nil {
			logger.Error("[Loopback] Missing required dependencies")
		}
		return
	}

	sessionID := runCfg.SessionID
	if sessionID == "" {
		sessionID = "default"
	}

	// Create manifest for tool resolution
	if runCfg.Manifest == nil {
		runCfg.Manifest = tools.NewManifest(cfg.Directories.ToolsDir)
	}

	// Insert external message into short-term memory
	mid, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, message, false, false)
	if err != nil {
		logger.Error("[Loopback] Failed to insert message", "error", err)
		return
	}
	historyManager.Add(openai.ChatMessageRoleUser, message, mid, false, false)

	// Build context flags
	flags := prompts.ContextFlags{
		ActiveProcesses:          GetActiveProcessStatus(runCfg.Registry),
		IsMaintenanceMode:        tools.IsBusy(),
		LifeboatEnabled:          cfg.Maintenance.LifeboatEnabled,
		SystemLanguage:           cfg.Agent.SystemLanguage,
		CorePersonality:          cfg.Agent.CorePersonality,
		TokenBudget:              cfg.Agent.SystemPromptTokenBudget,
		IsDebugMode:              cfg.Agent.DebugMode || GetDebugMode(),
		DiscordEnabled:           cfg.Discord.Enabled,
		EmailEnabled:             cfg.Email.Enabled,
		DockerEnabled:            cfg.Docker.Enabled,
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
		MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:           cfg.Sandbox.Enabled,
		MeshCentralEnabled:       cfg.MeshCentral.Enabled,
		HomepageEnabled:          cfg.Homepage.Enabled && cfg.Docker.Enabled,
		NetlifyEnabled:           cfg.Netlify.Enabled,
		ImageGenerationEnabled:   cfg.ImageGeneration.Enabled,
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
		WOLEnabled:               cfg.Tools.WOL.Enabled,
		AllowShell:               cfg.Agent.AllowShell,
		AllowPython:              cfg.Agent.AllowPython,
		AllowFilesystemWrite:     cfg.Agent.AllowFilesystemWrite,
		AllowNetworkRequests:     cfg.Agent.AllowNetworkRequests,
		AllowRemoteShell:         cfg.Agent.AllowRemoteShell,
		AllowSelfUpdate:          cfg.Agent.AllowSelfUpdate,
		IsEgg:                    cfg.EggMode.Enabled,
		InternetExposed:          cfg.Server.HTTPS.Enabled,
		IsDocker:                 cfg.Runtime.IsDocker,
		DocumentCreatorEnabled:   cfg.Tools.DocumentCreator.Enabled,
		FritzBoxSystemEnabled:    cfg.FritzBox.Enabled && cfg.FritzBox.System.Enabled,
		FritzBoxNetworkEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Network.Enabled,
		FritzBoxTelephonyEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled,
		FritzBoxSmartHomeEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.SmartHome.Enabled,
		FritzBoxStorageEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Storage.Enabled,
		FritzBoxTVEnabled:        cfg.FritzBox.Enabled && cfg.FritzBox.TV.Enabled,
		A2AEnabled:               cfg.A2A.Server.Enabled || cfg.A2A.Client.Enabled,
		TelnyxEnabled:            cfg.Telnyx.Enabled,
		AdditionalPrompt:         cfg.Agent.AdditionalPrompt,
	}
	coreMem := shortTermMem.ReadCoreMemory()
	sysPrompt := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, flags, coreMem, logger)

	// Assemble messages
	finalMessages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
	}
	if summary := historyManager.GetSummary(); summary != "" {
		finalMessages = append(finalMessages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "[CONTEXT_RECAP]: The following is a summary of previous relevant discussions for context. DO NOT echo or repeat this recap in your response:\n" + summary,
		})
	}
	finalMessages = append(finalMessages, historyManager.Get()...)

	req := openai.ChatCompletionRequest{
		Model:    cfg.LLM.Model,
		Messages: finalMessages,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resp, err := ExecuteAgentLoop(ctx, req, runCfg, false, broker)
	if err != nil {
		logger.Error("[Loopback] Agent loop failed", "error", err)
		broker.Send("error_recovery", "Sorry, I encountered an error processing your message.")
		return
	}

	// Send final response via broker
	if len(resp.Choices) > 0 {
		answer := resp.Choices[0].Message.Content
		if answer != "" {
			broker.Send("final_response", answer)
		}
	}

	logger.Info("[Loopback] Completed", "session", sessionID)
}
