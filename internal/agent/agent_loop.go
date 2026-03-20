package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/llm"
	loggerPkg "aurago/internal/logger"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// ExecuteAgentLoop executes the multi-turn reasoning and tool execution loop.
// It supports both synchronous returns and asynchronous streaming via the broker.
func ExecuteAgentLoop(ctx context.Context, req openai.ChatCompletionRequest, runCfg RunConfig, stream bool, broker FeedbackBroker) (openai.ChatCompletionResponse, error) {
	cfg := runCfg.Config
	logger := runCfg.Logger
	client := runCfg.LLMClient
	shortTermMem := runCfg.ShortTermMem
	historyManager := runCfg.HistoryManager
	longTermMem := runCfg.LongTermMem
	kg := runCfg.KG
	inventoryDB := runCfg.InventoryDB
	invasionDB := runCfg.InvasionDB
	cheatsheetDB := runCfg.CheatsheetDB
	imageGalleryDB := runCfg.ImageGalleryDB
	mediaRegistryDB := runCfg.MediaRegistryDB
	homepageRegistryDB := runCfg.HomepageRegistryDB
	remoteHub := runCfg.RemoteHub
	vault := runCfg.Vault
	registry := runCfg.Registry
	manifest := runCfg.Manifest
	cronManager := runCfg.CronManager
	missionManagerV2 := runCfg.MissionManagerV2
	coAgentRegistry := runCfg.CoAgentRegistry
	budgetTracker := runCfg.BudgetTracker
	sessionID := runCfg.SessionID
	isMaintenance := runCfg.IsMaintenance
	surgeryPlan := runCfg.SurgeryPlan

	// Load persistent adaptive tool usage data on first run
	if cfg.Agent.AdaptiveTools.Enabled && shortTermMem != nil {
		if entries, err := shortTermMem.LoadToolUsageAdaptive(); err == nil && len(entries) > 0 {
			converted := make([]prompts.ToolUsageEntry, len(entries))
			for i, e := range entries {
				converted[i] = prompts.ToolUsageEntry{ToolName: e.ToolName, TotalCount: e.TotalCount, SuccessCount: e.SuccessCount, LastUsed: e.LastUsed}
			}
			prompts.LoadAdaptiveToolState(converted)
			logger.Info("[AdaptiveTools] Loaded persistent tool usage", "tools_tracked", len(entries))
		}
	}

	var webhooksDef strings.Builder
	if cfg.Webhooks.Enabled && len(cfg.Webhooks.Outgoing) > 0 {
		for _, w := range cfg.Webhooks.Outgoing {
			webhooksDef.WriteString(fmt.Sprintf("- **%s**: %s\n", w.Name, w.Description))
			if len(w.Parameters) > 0 {
				webhooksDef.WriteString("  Parameters:\n")
				for _, p := range w.Parameters {
					reqStr := ""
					if p.Required {
						reqStr = " (required)"
					}
					webhooksDef.WriteString(fmt.Sprintf("    - `%s` [%s]%s: %s\n", p.Name, p.Type, reqStr, p.Description))
				}
			}
		}
	}

	flags := prompts.ContextFlags{
		IsErrorState:      false,
		RequiresCoding:    false,
		SystemLanguage:    cfg.Agent.SystemLanguage,
		LifeboatEnabled:   cfg.Maintenance.LifeboatEnabled,
		IsMaintenanceMode: isMaintenance,
		SurgeryPlan:       surgeryPlan,
		CorePersonality:   cfg.Agent.CorePersonality,
		TokenBudget:       cfg.Agent.SystemPromptTokenBudget,
		IsDebugMode:       cfg.Agent.DebugMode || GetDebugMode(),
		IsCoAgent:         strings.HasPrefix(sessionID, "coagent-"),
		DiscordEnabled:    cfg.Discord.Enabled,
		EmailEnabled:      cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
		// Docker-socket-dependent tools: only gate when actually inside Docker
		// without the socket mounted. On bare-metal / LXC the socket simply isn't
		// at the probed path, which is normal — do not disable user-configured tools.
		DockerEnabled:            cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
		WebDAVEnabled:            cfg.WebDAV.Enabled,
		KoofrEnabled:             cfg.Koofr.Enabled,
		PaperlessNGXEnabled:      cfg.PaperlessNGX.Enabled,
		ChromecastEnabled:        cfg.Chromecast.Enabled,
		CoAgentEnabled:           cfg.CoAgents.Enabled,
		GoogleWorkspaceEnabled:   cfg.GoogleWorkspace.Enabled,
		OneDriveEnabled:          cfg.OneDrive.Enabled,
		ProxmoxEnabled:           cfg.Proxmox.Enabled,
		OllamaEnabled:            cfg.Ollama.Enabled,
		TailscaleEnabled:         cfg.Tailscale.Enabled,
		CloudflareTunnelEnabled:  cfg.CloudflareTunnel.Enabled,
		AnsibleEnabled:           cfg.Ansible.Enabled,
		InvasionControlEnabled:   cfg.InvasionControl.Enabled && invasionDB != nil,
		GitHubEnabled:            cfg.GitHub.Enabled,
		MQTTEnabled:              cfg.MQTT.Enabled,
		AdGuardEnabled:           cfg.AdGuard.Enabled,
		MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
		SandboxEnabled:           cfg.Sandbox.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
		MeshCentralEnabled:       cfg.MeshCentral.Enabled,
		HomepageEnabled:          cfg.Homepage.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK || cfg.Homepage.AllowLocalServer),
		HomepageAllowLocalServer: cfg.Homepage.AllowLocalServer,
		NetlifyEnabled:           cfg.Netlify.Enabled,
		WebhooksEnabled:          cfg.Webhooks.Enabled,
		WebhooksDefinitions:      webhooksDef.String(),
		VirusTotalEnabled:        cfg.VirusTotal.Enabled,
		BraveSearchEnabled:       cfg.BraveSearch.Enabled,
		ImageGenerationEnabled:   cfg.ImageGeneration.Enabled,
		RemoteControlEnabled:     cfg.RemoteControl.Enabled && remoteHub != nil,
		MemoryEnabled:            cfg.Tools.Memory.Enabled,
		KnowledgeGraphEnabled:    cfg.Tools.KnowledgeGraph.Enabled,
		SecretsVaultEnabled:      cfg.Tools.SecretsVault.Enabled,
		SchedulerEnabled:         cfg.Tools.Scheduler.Enabled,
		NotesEnabled:             cfg.Tools.Notes.Enabled,
		JournalEnabled:           cfg.Tools.Journal.Enabled,
		MissionsEnabled:          cfg.Tools.Missions.Enabled,
		StopProcessEnabled:       cfg.Tools.StopProcess.Enabled,
		InventoryEnabled:         cfg.Tools.Inventory.Enabled,
		MemoryMaintenanceEnabled: cfg.Tools.MemoryMaintenance.Enabled,
		WOLEnabled:               cfg.Tools.WOL.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.BroadcastOK),
		MediaRegistryEnabled:     cfg.MediaRegistry.Enabled && mediaRegistryDB != nil,
		HomepageRegistryEnabled:  cfg.Homepage.Enabled && homepageRegistryDB != nil,
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
		WebCaptureEnabled:        cfg.Tools.WebCapture.Enabled,
		NetworkPingEnabled:       cfg.Tools.NetworkPing.Enabled,
		WebScraperEnabled:        cfg.Tools.WebScraper.Enabled,
		S3Enabled:                cfg.S3.Enabled,
		NetworkScanEnabled:       cfg.Tools.NetworkScan.Enabled,
		FormAutomationEnabled:    cfg.Tools.FormAutomation.Enabled,
		UPnPScanEnabled:          cfg.Tools.UPnPScan.Enabled,
		FritzBoxSystemEnabled:    cfg.FritzBox.Enabled && cfg.FritzBox.System.Enabled,
		FritzBoxNetworkEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Network.Enabled,
		FritzBoxTelephonyEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled,
		FritzBoxSmartHomeEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.SmartHome.Enabled,
		FritzBoxStorageEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Storage.Enabled,
		FritzBoxTVEnabled:        cfg.FritzBox.Enabled && cfg.FritzBox.TV.Enabled,
		A2AEnabled:               cfg.A2A.Server.Enabled || cfg.A2A.Client.Enabled,
		AdditionalPrompt:         cfg.Agent.AdditionalPrompt,
	}
	logger.Debug("[Agent] Context flags initialised",
		"token_budget", flags.TokenBudget,
		"session_id", runCfg.SessionID,
	)
	toolCallCount := 0
	rawCodeCount := 0
	missedToolCount := 0
	announcementCount := 0
	sessionTokens := 0
	emptyRetried := false // Prevents infinite retry on persistent empty responses
	stepsSinceLastFeedback := 0
	lastToolError := ""          // Tracks the last tool error string for consecutive-error detection
	consecutiveErrorCount := 0   // Incremented each time the same tool error repeats back-to-back
	homepageUsedInChain := false // Elevated circuit breaker once homepage tool is first used

	// Guardian: prompt injection defense
	guardian := security.NewGuardian(logger)

	// LLM Guardian: AI-powered pre-execution tool call security
	// Use the shared instance from RunConfig (so metrics are visible to the dashboard),
	// falling back to a fresh instance for callers that don't provide one.
	llmGuardian := runCfg.LLMGuardian
	if llmGuardian == nil {
		llmGuardian = security.NewLLMGuardian(cfg, logger)
	}

	var currentLogger *slog.Logger = logger
	lastActivity := time.Now()
	lastTool := ""
	recentTools := make([]string, 0, 5) // Track last 5 tools for lazy schema injection
	explicitTools := make([]string, 0)  // Explicit tool guides requested via <workflow_plan> tag
	workflowPlanCount := 0              // Prevent infinite workflow_plan loops
	lastResponseWasTool := false        // True when the previous iteration was a tool call; suppresses announcement detector on completion messages
	pendingTCs := make([]ToolCall, 0)   // Queued tool calls from multi-tool responses (processed without a new LLM call)

	// Context compression: tracks message count at last compression for cooldown
	lastCompressionMsg := 0

	// Core memory cache: read once, invalidate on manage_memory calls
	coreMemCache := ""
	coreMemDirty := true // Force initial load

	// Session-scoped todo list piggybacked on tool calls
	sessionTodoList := ""

	// Phase D: Personality Engine (opt-in)
	personalityEnabled := cfg.Agent.PersonalityEngine
	if personalityEnabled && shortTermMem != nil {
		if err := shortTermMem.InitPersonalityTables(); err != nil {
			logger.Error("[Personality] Failed to init tables, disabling", "error", err)
			personalityEnabled = false
		}
	}

	// Emotion Synthesizer (requires Personality Engine V2)
	var emotionSynthesizer *memory.EmotionSynthesizer
	if cfg.Agent.EmotionSynthesizer.Enabled && personalityEnabled && cfg.Agent.PersonalityEngineV2 {
		// Reuse V2 client setup
		var esClient memory.PersonalityAnalyzerClient = client
		if cfg.Agent.PersonalityV2URL != "" {
			key := cfg.Agent.PersonalityV2APIKey
			if key == "" {
				key = "dummy"
			}
			v2Cfg := openai.DefaultConfig(key)
			v2Cfg.BaseURL = cfg.Agent.PersonalityV2URL
			esClient = openai.NewClientWithConfig(v2Cfg)
		}
		esModel := cfg.Agent.PersonalityV2Model
		if esModel == "" {
			esModel = cfg.LLM.Model
		}
		emotionSynthesizer = memory.NewEmotionSynthesizer(
			esClient,
			esModel,
			cfg.Agent.EmotionSynthesizer.MinIntervalSecs,
			cfg.Agent.EmotionSynthesizer.MaxHistoryEntries,
			cfg.Agent.SystemLanguage,
			currentLogger,
		)
		logger.Info("[EmotionSynthesizer] Initialized", "model", esModel, "interval_secs", cfg.Agent.EmotionSynthesizer.MinIntervalSecs)
	}

	// Native function calling: build tool schemas once and attach to request
	toolGuidesDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")

	// Auto-detect DeepSeek and enable native function calling
	useNativeFunctions := cfg.LLM.UseNativeFunctions
	if strings.Contains(strings.ToLower(cfg.LLM.Model), "deepseek") && !useNativeFunctions {
		useNativeFunctions = true
		logger.Info("[NativeTools] DeepSeek detected, auto-enabling native function calling")
	}

	if useNativeFunctions {
		ff := ToolFeatureFlags{
			HomeAssistantEnabled:     cfg.HomeAssistant.Enabled,
			DockerEnabled:            cfg.Docker.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
			CoAgentEnabled:           cfg.CoAgents.Enabled,
			SudoEnabled:              cfg.Agent.SudoEnabled && !cfg.Runtime.IsDocker,
			WebhooksEnabled:          cfg.Webhooks.Enabled,
			ProxmoxEnabled:           cfg.Proxmox.Enabled,
			OllamaEnabled:            cfg.Ollama.Enabled,
			TailscaleEnabled:         cfg.Tailscale.Enabled,
			AnsibleEnabled:           cfg.Ansible.Enabled,
			InvasionControlEnabled:   cfg.InvasionControl.Enabled && invasionDB != nil,
			GitHubEnabled:            cfg.GitHub.Enabled,
			MQTTEnabled:              cfg.MQTT.Enabled,
			AdGuardEnabled:           cfg.AdGuard.Enabled,
			MCPEnabled:               cfg.MCP.Enabled && cfg.Agent.AllowMCP,
			SandboxEnabled:           cfg.Sandbox.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK),
			MeshCentralEnabled:       cfg.MeshCentral.Enabled,
			HomepageEnabled:          cfg.Homepage.Enabled && (!cfg.Runtime.IsDocker || cfg.Runtime.DockerSocketOK || cfg.Homepage.AllowLocalServer),
			HomepageAllowLocalServer: cfg.Homepage.AllowLocalServer,
			NetlifyEnabled:           cfg.Netlify.Enabled,
			FirewallEnabled:          cfg.Firewall.Enabled && cfg.Runtime.FirewallAccessOK,
			EmailEnabled:             cfg.Email.Enabled || len(cfg.EmailAccounts) > 0,
			CloudflareTunnelEnabled:  cfg.CloudflareTunnel.Enabled,
			GoogleWorkspaceEnabled:   cfg.GoogleWorkspace.Enabled,
			OneDriveEnabled:          cfg.OneDrive.Enabled,
			ImageGenerationEnabled:   cfg.ImageGeneration.Enabled,
			RemoteControlEnabled:     cfg.RemoteControl.Enabled && remoteHub != nil,
			MemoryEnabled:            cfg.Tools.Memory.Enabled,
			KnowledgeGraphEnabled:    cfg.Tools.KnowledgeGraph.Enabled,
			SecretsVaultEnabled:      cfg.Tools.SecretsVault.Enabled,
			SchedulerEnabled:         cfg.Tools.Scheduler.Enabled,
			NotesEnabled:             cfg.Tools.Notes.Enabled,
			JournalEnabled:           cfg.Tools.Journal.Enabled,
			MissionsEnabled:          cfg.Tools.Missions.Enabled,
			StopProcessEnabled:       cfg.Tools.StopProcess.Enabled,
			InventoryEnabled:         cfg.Tools.Inventory.Enabled,
			MemoryMaintenanceEnabled: cfg.Tools.MemoryMaintenance.Enabled,
			WOLEnabled:               cfg.Tools.WOL.Enabled && cfg.Runtime.BroadcastOK,
			MediaRegistryEnabled:     cfg.MediaRegistry.Enabled && mediaRegistryDB != nil,
			HomepageRegistryEnabled:  cfg.Homepage.Enabled && homepageRegistryDB != nil,
			MemoryAnalysisEnabled:    cfg.MemoryAnalysis.Enabled,
			DocumentCreatorEnabled:   cfg.Tools.DocumentCreator.Enabled,
			WebCaptureEnabled:        cfg.Tools.WebCapture.Enabled,
			NetworkPingEnabled:       cfg.Tools.NetworkPing.Enabled,
			WebScraperEnabled:        cfg.Tools.WebScraper.Enabled,
			S3Enabled:                cfg.S3.Enabled,
			NetworkScanEnabled:       cfg.Tools.NetworkScan.Enabled,
			FormAutomationEnabled:    cfg.Tools.FormAutomation.Enabled,
			UPnPScanEnabled:          cfg.Tools.UPnPScan.Enabled,
			FritzBoxSystemEnabled:    cfg.FritzBox.Enabled && cfg.FritzBox.System.Enabled,
			FritzBoxNetworkEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Network.Enabled,
			FritzBoxTelephonyEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.Telephony.Enabled,
			FritzBoxSmartHomeEnabled: cfg.FritzBox.Enabled && cfg.FritzBox.SmartHome.Enabled,
			FritzBoxStorageEnabled:   cfg.FritzBox.Enabled && cfg.FritzBox.Storage.Enabled,
			FritzBoxTVEnabled:        cfg.FritzBox.Enabled && cfg.FritzBox.TV.Enabled,
			// Danger Zone capability gates
			AllowShell:           cfg.Agent.AllowShell,
			AllowPython:          cfg.Agent.AllowPython,
			AllowFilesystemWrite: cfg.Agent.AllowFilesystemWrite,
			AllowNetworkRequests: cfg.Agent.AllowNetworkRequests,
			AllowRemoteShell:     cfg.Agent.AllowRemoteShell,
			AllowSelfUpdate:      cfg.Agent.AllowSelfUpdate,
		}
		ntSchemas := BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, logger)

		// Adaptive tool filtering: remove rarely-used tools to save tokens
		if cfg.Agent.AdaptiveTools.Enabled && shortTermMem != nil {
			halfLife := cfg.Agent.AdaptiveTools.DecayHalfLifeDays
			if halfLife <= 0 {
				halfLife = 7.0
			}
			frequent := prompts.GetFrequentToolsWeighted(0, halfLife, cfg.Agent.AdaptiveTools.WeightSuccessRate) // 0 = all scored tools
			maxTools := cfg.Agent.AdaptiveTools.MaxTools
			alwaysInclude := cfg.Agent.AdaptiveTools.AlwaysInclude
			if maxTools > 0 && len(frequent) > 0 {
				ntSchemas = filterToolSchemas(ntSchemas, frequent, alwaysInclude, maxTools, logger)
			}
		}
		// Structured Outputs: set Strict=true on every tool definition so the
		// provider uses constrained decoding for tool-call arguments.
		// Only enable this for models that support structured outputs (e.g. GPT-4o,
		// some OpenRouter models). Ollama does not support strict mode.
		isOllama := strings.EqualFold(cfg.LLM.ProviderType, "ollama")
		if cfg.LLM.StructuredOutputs && !isOllama {
			for i := range ntSchemas {
				if ntSchemas[i].Function != nil {
					ntSchemas[i].Function.Strict = true
				}
			}
			logger.Info("[NativeTools] Structured outputs enabled (strict mode)")
		} else if cfg.LLM.StructuredOutputs && isOllama {
			logger.Warn("[NativeTools] Structured outputs not supported by Ollama, ignoring")
		}
		req.Tools = ntSchemas
		req.ToolChoice = "auto"
		// Ollama does not support parallel_tool_calls — only set for compatible providers
		if !isOllama {
			req.ParallelToolCalls = true
		}
		logger.Info("[NativeTools] Native function calling enabled", "tool_count", len(ntSchemas), "parallel", !isOllama)
	}

	for {
		// Check for user interrupt
		if checkAndClearInterrupt(sessionID) {
			currentLogger.Warn("[Sync] User interrupted the agent — stopping immediately")
			broker.Send("thinking", "Stopped by user.")
			stopContent := "⏹ Stopped."
			// Persist the stop event so the agent remembers it was stopped
			if shortTermMem != nil {
				msgID, _ := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, stopContent, false, false)
				if sessionID == "default" && historyManager != nil {
					historyManager.Add(openai.ChatMessageRoleAssistant, stopContent, msgID, false, false)
				}
			}
			return openai.ChatCompletionResponse{
				ID:      "stop-" + sessionID,
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []openai.ChatCompletionChoice{{
					Index: 0,
					Message: openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleAssistant,
						Content: stopContent,
					},
					FinishReason: openai.FinishReasonStop,
				}},
			}, nil
		}

		// Revive logic: If idle in lifeboat for too long, poke the agent
		if isMaintenance && time.Since(lastActivity) > time.Duration(cfg.CircuitBreaker.MaintenanceTimeoutMinutes)*time.Minute {
			currentLogger.Warn("[Sync] Lifeboat idle for too long, injecting revive prompt", "minutes", cfg.CircuitBreaker.MaintenanceTimeoutMinutes)
			reviveMsg := "You are idle in the lifeboat. finish your tasks or change back to the supervisor."
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: reviveMsg})
			lastActivity = time.Now() // Reset timer
		}

		// Refresh maintenance status to account for mid-loop handovers
		isMaintenance = isMaintenance || tools.IsBusy()
		flags.IsMaintenanceMode = isMaintenance

		// Caching the logger to avoid opening file on every iteration (leaking FDs)
		if isMaintenance && currentLogger == nil {
			logPath := filepath.Join(cfg.Logging.LogDir, "lifeboat.log")
			if l, err := loggerPkg.SetupWithFile(true, logPath, true); err == nil {
				currentLogger = l.Logger
			}
		}
		if currentLogger == nil {
			currentLogger = logger
		}

		currentLogger.Debug("[Sync] Agent loop iteration starting", "is_maintenance", isMaintenance, "lock_exists", tools.IsBusy())

		// Extract the last user message early — needed for Guardian context in pendingTCs path
		lastUserMsg := ""
		if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == openai.ChatMessageRoleUser {
			lastUserMsg = req.Messages[len(req.Messages)-1].Content
		}

		// Process queued tool calls from multi-tool responses (skip LLM for these)
		if len(pendingTCs) > 0 {
			ptc := pendingTCs[0]
			pendingTCs = pendingTCs[1:]
			toolCallCount++
			if ptc.Action == "homepage" || ptc.Action == "homepage_tool" {
				homepageUsedInChain = true
			}
			broker.Send("thinking", fmt.Sprintf("[%d] Running %s...", toolCallCount, ptc.Action))
			ptcJSON := ptc.RawJSON
			if ptcJSON == "" {
				ptcJSON = fmt.Sprintf(`{"action":"%s"}`, ptc.Action)
			}
			id, idErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, ptcJSON, false, true)
			if idErr != nil {
				currentLogger.Error("Failed to persist queued tool-call message", "error", idErr)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, ptcJSON, id, false, true)
			}
			broker.Send("tool_call", ptcJSON)
			broker.Send("tool_start", ptc.Action)
			pResultContent := DispatchToolCall(ctx, ptc, cfg, currentLogger, client, vault, registry, manifest, cronManager, missionManagerV2, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyManager, tools.IsBusy(), surgeryPlan, guardian, llmGuardian, sessionID, coAgentRegistry, budgetTracker, lastUserMsg)
			pResultContent = truncateToolOutput(pResultContent, cfg.Agent.ToolOutputLimit)
			prompts.RecordToolUsage(ptc.Action, ptc.Operation, !isToolError(pResultContent))
			prompts.RecordAdaptiveToolUsage(ptc.Action, !isToolError(pResultContent))
			if shortTermMem != nil {
				_ = shortTermMem.UpsertToolUsage(ptc.Action, !isToolError(pResultContent))
			}
			broker.Send("tool_output", pResultContent)
			if ptc.Action == "send_image" {
				var imgRes struct {
					Status  string `json:"status"`
					WebPath string `json:"web_path"`
					Caption string `json:"caption"`
				}
				raw := strings.TrimPrefix(pResultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &imgRes) == nil && imgRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{"path": imgRes.WebPath, "caption": imgRes.Caption})
					broker.Send("image", string(evtPayload))
				}
			}
			if ptc.Action == "send_audio" {
				var audioRes struct {
					Status   string `json:"status"`
					WebPath  string `json:"web_path"`
					Title    string `json:"title"`
					MimeType string `json:"mime_type"`
					Filename string `json:"filename"`
				}
				raw := strings.TrimPrefix(pResultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &audioRes) == nil && audioRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{
						"path":      audioRes.WebPath,
						"title":     audioRes.Title,
						"mime_type": audioRes.MimeType,
						"filename":  audioRes.Filename,
					})
					broker.Send("audio", string(evtPayload))
				}
			}
			if ptc.Action == "send_document" {
				var docRes struct {
					Status     string `json:"status"`
					WebPath    string `json:"web_path"`
					PreviewURL string `json:"preview_url"`
					Title      string `json:"title"`
					MimeType   string `json:"mime_type"`
					Filename   string `json:"filename"`
					Format     string `json:"format"`
				}
				raw := strings.TrimPrefix(pResultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &docRes) == nil && docRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{
						"path":        docRes.WebPath,
						"preview_url": docRes.PreviewURL,
						"title":       docRes.Title,
						"mime_type":   docRes.MimeType,
						"filename":    docRes.Filename,
						"format":      docRes.Format,
					})
					broker.Send("document", string(evtPayload))
				}
			}
			broker.Send("tool_end", ptc.Action)
			lastActivity = time.Now()
			// Update session todo from piggybacked _todo field
			if ptc.Todo != "" {
				sessionTodoList = ptc.Todo
				broker.Send("todo_update", sessionTodoList)
			}
			if ptc.Action == "manage_memory" || ptc.Action == "core_memory" {
				coreMemDirty = true
			}
			// Track recent tools for journal auto-trigger (keep last 5, dedup)
			{
				found := false
				for _, rt := range recentTools {
					if rt == ptc.Action {
						found = true
						break
					}
				}
				if !found {
					recentTools = append(recentTools, ptc.Action)
					if len(recentTools) > 5 {
						recentTools = recentTools[len(recentTools)-5:]
					}
				}
			}
			id, idErr = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, pResultContent, false, true)
			if idErr != nil {
				currentLogger.Error("Failed to persist queued tool-result message", "error", idErr)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, pResultContent, id, false, true)
			}
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: ptcJSON})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: pResultContent})
			lastResponseWasTool = true
			continue
		}

		// Load Personality Meta
		var meta memory.PersonalityMeta
		if personalityEnabled {
			meta = prompts.GetCorePersonalityMeta(cfg.Directories.PromptsDir, flags.CorePersonality)
		}

		// Circuit breaker - berechne Basis-Limit (Tool-spezifische Anpassungen erfolgen später wenn tc bekannt ist)
		effectiveMaxCalls := calculateEffectiveMaxCalls(cfg, ToolCall{}, homepageUsedInChain, personalityEnabled, shortTermMem, currentLogger)

		if toolCallCount >= effectiveMaxCalls {
			currentLogger.Warn("[Sync] Circuit breaker triggered", "count", toolCallCount, "limit", effectiveMaxCalls)
			breakerMsg := fmt.Sprintf("CIRCUIT BREAKER: You have reached the maximum of %d consecutive tool calls. You MUST now summarize your progress and respond to the user with a final answer.", effectiveMaxCalls)
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: breakerMsg})
		}

		flags.ActiveProcesses = GetActiveProcessStatus(registry)

		// Load Core Memory (cached, invalidated when manage_memory is called)
		if coreMemDirty {
			if shortTermMem != nil {
				coreMemCache = shortTermMem.ReadCoreMemory()
			}
			coreMemDirty = false
		}

		// Extract explicit workflow tools if present (populated from previous iteration's <workflow_plan> tag)
		// explicitTools is persistent across loop iterations

		// Prepare Dynamic Tool Guides
		if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == openai.ChatMessageRoleUser {
			lastUserMsg = req.Messages[len(req.Messages)-1].Content
		}

		// Get the mood trigger context from the message history
		triggerValue := getMoodTrigger(req.Messages, lastUserMsg)
		moodTrigger := func() string { return triggerValue }

		// Note: The call to PrepareDynamicGuides will happen after the response is received
		// We initialize flags.PredictedGuides now with empty explicit tools to satisfy builder.go for the first prompt
		flags.PredictedGuides = prompts.PrepareDynamicGuides(longTermMem, shortTermMem, lastUserMsg, lastTool, toolGuidesDir, recentTools, explicitTools, cfg.Agent.MaxToolGuides, currentLogger)

		// Automatic RAG: retrieve relevant long-term memories for the current user message
		// Phase A3: Over-fetch and re-rank with recency boost from memory_meta
		flags.RetrievedMemories = ""
		flags.PredictedMemories = ""
		var topMemories []string
		if lastUserMsg != "" && longTermMem != nil {
			// Query expansion: enrich user message with LLM-generated keywords for better RAG
			ragQuery := expandQueryForRAG(ctx, cfg, currentLogger, lastUserMsg)

			// Over-fetch 6 candidates, then re-rank to keep best 3
			memories, docIDs, err := longTermMem.SearchSimilar(ragQuery, 6, "tool_guides")
			if err == nil && len(memories) > 0 {
				ranked := rerankWithRecency(memories, docIDs, shortTermMem, currentLogger)

				// LLM re-ranking: blend LLM relevance scores with recency-boosted scores
				ranked = rerankWithLLM(ctx, cfg, currentLogger, ranked, lastUserMsg)

				for _, r := range ranked {
					_ = shortTermMem.UpdateMemoryAccess(r.docID)
				}
				if len(ranked) > 3 {
					ranked = ranked[:3]
				}
				for _, r := range ranked {
					topMemories = append(topMemories, r.text)
				}
				flags.RetrievedMemories = strings.Join(topMemories, "\n---\n")
				currentLogger.Debug("[Sync] RAG: Retrieved memories (recency-boosted)", "count", len(ranked))
			}

			// Phase A4: Record interaction pattern for temporal learning
			if shortTermMem != nil {
				topic := lastUserMsg
				if len(topic) > 80 {
					topic = topic[:80]
				}
				_ = shortTermMem.RecordInteraction(topic)
			}

			// Phase B: Predictive pre-fetch based on temporal patterns + tool transitions
			// Deduplicate against already-retrieved memories to avoid wasting tokens
			if shortTermMem != nil {
				now := time.Now()
				predictions, err := shortTermMem.PredictNextQuery(lastTool, now.Hour(), int(now.Weekday()), 2)
				if err == nil && len(predictions) > 0 {
					// Build set of already-retrieved memory texts for dedup
					retrievedSet := make(map[string]struct{})
					for _, r := range topMemories {
						retrievedSet[r] = struct{}{}
					}

					var predictedResults []string
					for _, pred := range predictions {
						// Use SearchMemoriesOnly: predictive pre-fetch needs only user memories,
						// not tool_guides/documentation — avoids 2 full extra search cycles per request.
						pMem, _, pErr := longTermMem.SearchMemoriesOnly(pred, 1)
						if pErr == nil && len(pMem) > 0 {
							if _, dup := retrievedSet[pMem[0]]; !dup {
								predictedResults = append(predictedResults, pMem[0])
							}
						}
					}
					if len(predictedResults) > 0 {
						flags.PredictedMemories = strings.Join(predictedResults, "\n---\n")
						currentLogger.Debug("[Sync] Predictive RAG: Pre-fetched memories", "count", len(predictedResults), "predictions", predictions)
					}
				}
			}
		}

		// Knowledge Graph context injection: search for relevant entities
		if cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.PromptInjection && kg != nil && lastUserMsg != "" {
			maxNodes := cfg.Tools.KnowledgeGraph.MaxPromptNodes
			maxChars := cfg.Tools.KnowledgeGraph.MaxPromptChars
			kgContext := kg.SearchForContext(lastUserMsg, maxNodes, maxChars)
			if kgContext != "" {
				flags.KnowledgeContext = kgContext
				currentLogger.Debug("[Sync] KG: Injected knowledge context", "chars", len(kgContext))
			}
		}

		// Error Pattern Context: inject known errors when in error recovery state
		if flags.IsErrorState && shortTermMem != nil {
			errPatterns, err := shortTermMem.GetRecentErrors(5)
			if err == nil && len(errPatterns) > 0 {
				var epBuf strings.Builder
				epBuf.WriteString("Previous tool errors (learn from these to avoid repeating):\n")
				for _, ep := range errPatterns {
					epBuf.WriteString(fmt.Sprintf("- Tool: %s | Error: %s (occurred %d times)", ep.ToolName, ep.ErrorMessage, ep.OccurrenceCount))
					if ep.Resolution != "" {
						epBuf.WriteString(fmt.Sprintf(" | Resolution: %s", ep.Resolution))
					}
					epBuf.WriteString("\n")
				}
				flags.ErrorPatternContext = epBuf.String()
			}
		}

		// Phase D: Inject personality line before building system prompt
		if personalityEnabled && shortTermMem != nil {
			if cfg.Agent.PersonalityEngineV2 {
				// V2 Feature: Narrative Events based on Milestones & Loneliness
				processBehavioralEvents(shortTermMem, &req.Messages, sessionID, meta, currentLogger)
			}
			flags.PersonalityLine = shortTermMem.GetPersonalityLine(cfg.Agent.PersonalityEngineV2)

			// Emotion Synthesizer: inject LLM-generated emotional description
			if emotionSynthesizer != nil {
				if es := emotionSynthesizer.GetLastEmotion(); es != nil {
					flags.EmotionDescription = es.Description
				}
			}
		}

		// User Profiling: inject behavioral instruction + collected profile data
		if cfg.Agent.UserProfiling {
			flags.UserProfilingEnabled = true
			if cfg.Agent.PersonalityEngineV2 && shortTermMem != nil {
				flags.UserProfileSummary = shortTermMem.GetUserProfileSummary(cfg.Agent.UserProfilingThreshold)
				logger.Debug("User profiling enabled", "summaryLength", len(flags.UserProfileSummary), "threshold", cfg.Agent.UserProfilingThreshold)
			} else {
				logger.Debug("User profiling enabled (without profile summary - V2 engine disabled or no memory)")
			}
		}

		// Adaptive tier: adjust prompt complexity based on conversation length and context signals
		flags.MessageCount = len(req.Messages)
		flags.RecentlyUsedTools = recentTools
		flags.Tier = prompts.DetermineTierAdaptive(flags)
		flags.IsDebugMode = cfg.Agent.DebugMode || GetDebugMode() // re-check each iteration (toggleable at runtime)

		// Inject high-priority open notes as reminders
		if cfg.Tools.Notes.Enabled && shortTermMem != nil {
			if hpNotes, err := shortTermMem.GetHighPriorityOpenNotes(5); err == nil && len(hpNotes) > 0 {
				var sb strings.Builder
				for _, n := range hpNotes {
					sb.WriteString(fmt.Sprintf("- [%s] %s", n.Category, n.Title))
					if n.DueDate != "" {
						sb.WriteString(fmt.Sprintf(" (due: %s)", n.DueDate))
					}
					sb.WriteString("\n")
				}
				flags.HighPriorityNotes = sb.String()
			}
		}

		// Inject session todo list into system prompt context
		flags.SessionTodoItems = sessionTodoList

		sysPrompt := prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, flags, coreMemCache, currentLogger)

		// Inject budget hint into system prompt when threshold is crossed
		if budgetTracker != nil {
			if hint := budgetTracker.GetPromptHint(); hint != "" {
				sysPrompt += "\n\n" + hint
			}
		}

		currentLogger.Debug("[Sync] System prompt rebuilt", "length", len(sysPrompt), "tier", flags.Tier, "tokens", prompts.CountTokens(sysPrompt), "error_state", flags.IsErrorState, "coding_mode", flags.RequiresCoding, "active_daemons", flags.ActiveProcesses)

		if len(req.Messages) > 0 && req.Messages[0].Role == openai.ChatMessageRoleSystem {
			req.Messages[0].Content = sysPrompt
		} else {
			req.Messages = append([]openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
			}, req.Messages...)
		}

		// ── Context compression ──
		// Before the hard-trim guard, try to compress older messages into a summary
		// to preserve knowledge while freeing token budget.
		ctxWindow := cfg.Agent.ContextWindow
		if ctxWindow <= 0 {
			ctxWindow = 163840
		}
		completionMargin := 4096
		maxHistoryTokens := ctxWindow - completionMargin
		if maxHistoryTokens < 4096 {
			maxHistoryTokens = 4096
		}
		req.Messages, lastCompressionMsg, _ = CompressHistory(
			ctx, req.Messages, maxHistoryTokens, cfg.LLM.Model, client, lastCompressionMsg, currentLogger,
		)

		// ── Context window guard ──
		// Count total tokens across all messages and trim old history if we would
		// exceed the model's context window. We keep the system prompt (index 0) and
		// always preserve the final user message so the model has something to answer.
		// A 4096-token margin is reserved for the model's completion output.
		totalMsgTokens := 0
		for _, m := range req.Messages {
			totalMsgTokens += prompts.CountTokens(m.Content) + 4 // ~4 tokens overhead per message
		}
		if totalMsgTokens > maxHistoryTokens && len(req.Messages) > 2 {
			currentLogger.Warn("[ContextGuard] Token limit exceeded before LLM call — trimming history",
				"tokens", totalMsgTokens, "limit", maxHistoryTokens, "messages", len(req.Messages))
			sysMsg := req.Messages[0]
			lastMsg := req.Messages[len(req.Messages)-1]
			// Drop messages from index 1 onward (oldest first) until we fit.
			// Always keep system (0) and the latest message.
			mid := req.Messages[1 : len(req.Messages)-1]
			for totalMsgTokens > maxHistoryTokens && len(mid) > 0 {
				dropped := mid[0]
				mid = mid[1:]
				totalMsgTokens -= prompts.CountTokens(dropped.Content) + 4
			}
			req.Messages = append([]openai.ChatCompletionMessage{sysMsg}, append(mid, lastMsg)...)
			currentLogger.Info("[ContextGuard] History trimmed",
				"remaining_messages", len(req.Messages), "estimated_tokens", totalMsgTokens)
		}

		// Verbose Logging of LLM Request
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			// Keep conversation logs in the original logger (stdout) to avoid pollution of technical log
			logger.Info("[LLM Request]", "role", lastMsg.Role, "content_len", len(lastMsg.Content), "preview", Truncate(lastMsg.Content, 200))
			currentLogger.Info("[LLM Request Redirected]", "role", lastMsg.Role, "content_len", len(lastMsg.Content))
			currentLogger.Debug("[LLM Full History]", "messages_count", len(req.Messages))
		}

		broker.Send("thinking", "")

		// ── Temperature: base from config + personality modulation ──
		baseTemp := cfg.LLM.Temperature
		if baseTemp <= 0 {
			baseTemp = 0.7
		}
		tempDelta := 0.0
		if personalityEnabled && shortTermMem != nil {
			tempDelta = shortTermMem.GetTemperatureDelta()
		}
		// Creative tool boost: raise temperature for homepage design and image generation
		creativeDelta := 0.0
		switch lastTool {
		case "homepage", "homepage_tool":
			creativeDelta = 0.2
		case "generate_image":
			creativeDelta = 0.3
		}
		effectiveTemp := baseTemp + tempDelta + creativeDelta
		// Clamp to safe range [0.05, 1.5] — never fully deterministic, never too wild
		if effectiveTemp < 0.05 {
			effectiveTemp = 0.05
		}
		if effectiveTemp > 1.5 {
			effectiveTemp = 1.5
		}
		req.Temperature = float32(effectiveTemp)
		if tempDelta != 0 || creativeDelta != 0 {
			currentLogger.Debug("[Temperature] Modulation applied", "base", baseTemp, "personality_delta", tempDelta, "creative_delta", creativeDelta, "effective", effectiveTemp)
		}

		// Budget check: block if daily budget exceeded and enforcement = full
		if budgetTracker != nil && budgetTracker.IsBlocked("chat") {
			broker.Send("budget_blocked", "Daily budget exceeded. All LLM calls blocked until reset.")
			return openai.ChatCompletionResponse{}, fmt.Errorf("budget exceeded (enforcement=full)")
		}

		// Configurable timeout for each individual LLM call to prevent infinite hangs
		llmCtx, cancelResp := context.WithTimeout(ctx, time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds)*time.Second)

		var resp openai.ChatCompletionResponse
		var content string
		var err error
		var promptTokens, completionTokens, totalTokens int

		if stream {
			stm, streamErr := llm.ExecuteStreamWithRetry(llmCtx, client, req, currentLogger, broker)
			if streamErr != nil {
				cancelResp()
				// Same 422 recovery as the sync path: trim malformed history and retry.
				if strings.Contains(streamErr.Error(), "422") || strings.Contains(strings.ToLower(streamErr.Error()), "unprocessable") {
					currentLogger.Warn("[Stream] 422 Unprocessable from provider — trimming malformed history", "error", streamErr)
					broker.Send("thinking", "Context error recovered — retrying...")
					var trimmed []openai.ChatCompletionMessage
					for _, m := range req.Messages {
						if m.Role != openai.ChatMessageRoleTool {
							trimmed = append(trimmed, m)
						}
					}
					if len(trimmed) > 7 {
						trimmed = append(trimmed[:1], trimmed[len(trimmed)-6:]...)
					}
					trimmed = append(trimmed, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleSystem,
						Content: "SYSTEM: The previous tool call history was trimmed due to a provider error. Summarise the situation for the user and explain what you were doing and what went wrong.",
					})
					req.Messages = trimmed
					currentLogger.Info("[Stream] Context trimmed after 422, retrying", "new_messages_count", len(req.Messages))
					continue
				}
				return openai.ChatCompletionResponse{}, streamErr
			}

			var assembledResponse strings.Builder
			// Collect streamed tool calls (native function calling via streaming).
			// The API sends partial tool call data across multiple chunks that must
			// be reassembled: each chunk carries an Index identifying the call and
			// incremental Function.Name / Function.Arguments fragments.
			streamToolCalls := map[int]*openai.ToolCall{}
			for {
				chunk, rErr := stm.Recv()
				if rErr != nil {
					if rErr.Error() != "EOF" {
						currentLogger.Error("Stream error", "error", rErr)
					}
					break
				}
				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta
					if delta.Content != "" {
						assembledResponse.WriteString(delta.Content)
						// Proxy the JSON chunk to the broker if it supports dynamic passthrough (SSE)
						// We'll marshal it so we can push it cleanly
						if chunkData, mErr := json.Marshal(chunk); mErr == nil {
							broker.SendJSON(fmt.Sprintf("data: %s\n\n", string(chunkData)))
						}
					}
					// Accumulate streamed tool call fragments
					for _, tc := range delta.ToolCalls {
						idx := 0
						if tc.Index != nil {
							idx = *tc.Index
						}
						existing, ok := streamToolCalls[idx]
						if !ok {
							clone := openai.ToolCall{
								Index: tc.Index,
								ID:    tc.ID,
								Type:  tc.Type,
								Function: openai.FunctionCall{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							}
							streamToolCalls[idx] = &clone
						} else {
							if tc.ID != "" {
								existing.ID = tc.ID
							}
							if tc.Function.Name != "" {
								existing.Function.Name += tc.Function.Name
							}
							existing.Function.Arguments += tc.Function.Arguments
						}
					}
				}
			}
			stm.Close()
			content = assembledResponse.String()

			// Build sorted slice of assembled tool calls
			var assembledToolCalls []openai.ToolCall
			if len(streamToolCalls) > 0 {
				assembledToolCalls = make([]openai.ToolCall, 0, len(streamToolCalls))
				for i := 0; i < len(streamToolCalls); i++ {
					if tc, ok := streamToolCalls[i]; ok {
						assembledToolCalls = append(assembledToolCalls, *tc)
					}
				}
				currentLogger.Info("[Stream] Assembled streamed tool calls", "count", len(assembledToolCalls))
			}

			// Estimate streaming tokens
			completionTokens = estimateTokensForModel(content, req.Model)
			for _, m := range req.Messages {
				promptTokens += estimateTokensForModel(m.Content, req.Model)
			}
			totalTokens = promptTokens + completionTokens

			// Mock a response object for remaining loop logic
			resp = openai.ChatCompletionResponse{
				Choices: []openai.ChatCompletionChoice{
					{Message: openai.ChatCompletionMessage{
						Role:      openai.ChatMessageRoleAssistant,
						Content:   content,
						ToolCalls: assembledToolCalls,
					}},
				},
				Usage: openai.Usage{
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
				},
			}
		} else {
			resp, err = llm.ExecuteWithRetry(llmCtx, client, req, currentLogger, broker)
			if err != nil {
				cancelResp()
				// 422 Unprocessable Entity: the provider rejected the message sequence —
				// this happens when repeated identical tool responses have grown the history
				// into an invalid state (e.g. tool messages without matching tool_calls).
				// Instead of killing the session, strip role=tool messages, trim history,
				// inject an explanatory system note, and retry.
				if strings.Contains(err.Error(), "422") || strings.Contains(strings.ToLower(err.Error()), "unprocessable") {
					currentLogger.Warn("[Sync] 422 Unprocessable from provider — trimming malformed history", "error", err)
					broker.Send("thinking", "Context error recovered — retrying...")
					// Remove all role=tool messages (they need matching tool_call_ids which may
					// have been invalidated by trimming), keep system + user/assistant only.
					var trimmed []openai.ChatCompletionMessage
					for _, m := range req.Messages {
						if m.Role != openai.ChatMessageRoleTool {
							trimmed = append(trimmed, m)
						}
					}
					// Keep system prompt + last 6 messages to avoid re-triggering 422.
					if len(trimmed) > 7 {
						trimmed = append(trimmed[:1], trimmed[len(trimmed)-6:]...)
					}
					trimmed = append(trimmed, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleSystem,
						Content: "SYSTEM: The previous tool call history was trimmed due to a provider error. Summarise the situation for the user and explain what you were doing and what went wrong.",
					})
					req.Messages = trimmed
					currentLogger.Info("[Sync] Context trimmed after 422, retrying", "new_messages_count", len(req.Messages))
					continue
				}
				return openai.ChatCompletionResponse{}, err
			}
			if len(resp.Choices) == 0 {
				cancelResp()
				return openai.ChatCompletionResponse{}, fmt.Errorf("no choices returned from LLM")
			}
			content = resp.Choices[0].Message.Content
		}

		cancelResp()

		// Empty response recovery: if the LLM returns nothing, trim history and retry once.
		// This typically happens when the total context exceeds the model's window.
		if strings.TrimSpace(content) == "" && len(resp.Choices[0].Message.ToolCalls) == 0 && len(req.Messages) > 4 && !emptyRetried {
			emptyRetried = true
			currentLogger.Warn("[Sync] Empty LLM response detected, trimming history and retrying", "messages_count", len(req.Messages))
			broker.Send("thinking", "Context too large, retrimming...")
			// Keep system prompt (index 0) + optional summary (index 1 if system) + last 4 messages
			var trimmed []openai.ChatCompletionMessage
			trimmed = append(trimmed, req.Messages[0]) // system prompt
			// Keep second message if it's a system summary
			startIdx := 1
			if len(req.Messages) > 1 && req.Messages[1].Role == openai.ChatMessageRoleSystem {
				trimmed = append(trimmed, req.Messages[1])
				startIdx = 2
			}
			// Keep last 4 messages from history
			historyMsgs := req.Messages[startIdx:]
			if len(historyMsgs) > 4 {
				historyMsgs = historyMsgs[len(historyMsgs)-4:]
			}
			trimmed = append(trimmed, historyMsgs...)
			req.Messages = trimmed
			currentLogger.Info("[Sync] Retrying with trimmed context", "new_messages_count", len(req.Messages))
			continue
		}

		// Safety Check: Strip "RECAP" hallucinations if the model is still stuck in the old pattern
		content = strings.TrimPrefix(content, "[RECAP OF PREVIOUS DISCUSSIONS]:")
		content = strings.TrimPrefix(content, "[RECAP OF PREVIOUS DISCUSSIONS]:\n")
		content = strings.TrimPrefix(content, "[CONTEXT_RECAP]:")
		content = strings.TrimPrefix(content, "[CONTEXT_RECAP]:\n")
		content = strings.TrimSpace(content)

		// Conversation log to stdout
		logger.Info("[LLM Response]", "content_len", len(content), "preview", Truncate(content, 200))
		// Activity log to file
		currentLogger.Info("[LLM Response Received]", "content_len", len(content))
		lastActivity = time.Now() // LLM activity

		// Detect tool call: native API-level ToolCalls (use_native_functions=true) or text-based JSON
		var tc ToolCall
		useNativePath := false
		nativeAssistantMsg := resp.Choices[0].Message // snapshot for role=tool continuation

		if len(resp.Choices[0].Message.ToolCalls) > 0 {
			nativeCall := resp.Choices[0].Message.ToolCalls[0]
			// Primary native path: parse directly from API-level ToolCall object
			// We now take this path if UseNativeFunctions is true OR if the model sent them anyway
			tc = NativeToolCallToToolCall(nativeCall, currentLogger)
			useNativePath = true
			currentLogger.Info("[Sync] Native tool call detected", "function", tc.Action, "id", nativeCall.ID, "forced", !cfg.LLM.UseNativeFunctions)

			// Queue additional native tool calls for batch execution.
			// The OpenAI API requires a role=tool response for each tool_call in the
			// assistant message, so these are processed inline (not in the regular pendingTCs loop).
			if len(resp.Choices[0].Message.ToolCalls) > 1 {
				for _, extra := range resp.Choices[0].Message.ToolCalls[1:] {
					extraTC := NativeToolCallToToolCall(extra, currentLogger)
					pendingTCs = append(pendingTCs, extraTC)
				}
				currentLogger.Info("[MultiTool] Queued additional native tool calls from response", "count", len(resp.Choices[0].Message.ToolCalls)-1)
			}
		}

		// Text-based fallback: parse JSON from content string if native path not taken
		if !useNativePath {
			tc = ParseToolCall(content)
			// If the response contains multiple tool calls (e.g. two manage_memory adds),
			// queue the extras so they execute in subsequent iterations without a new LLM call.
			if tc.IsTool {
				extras := extractExtraToolCalls(content, tc.RawJSON)
				if len(extras) > 0 {
					currentLogger.Info("[MultiTool] Queued additional tool calls from response", "count", len(extras))
					pendingTCs = append(pendingTCs, extras...)
				}
			}
		}

		// Obsolete: we now send it later when histContent is fully assembled.
		if !stream {
			promptTokens = resp.Usage.PromptTokens
			completionTokens = resp.Usage.CompletionTokens
			totalTokens = resp.Usage.TotalTokens
		}

		if totalTokens == 0 {
			// Estimate tokens if usage is missing
			muTokens.Lock()
			GlobalTokenEstimated = true
			muTokens.Unlock()

			// Estimate prompt tokens from all messages in request
			for _, m := range req.Messages {
				promptTokens += estimateTokensForModel(m.Content, req.Model)
			}
			// Estimate completion tokens from response content
			completionTokens = estimateTokensForModel(content, req.Model)
			totalTokens = promptTokens + completionTokens
		}

		sessionTokens += totalTokens
		muTokens.Lock()
		GlobalTokenCount += totalTokens
		localGlobalTotal := GlobalTokenCount
		localIsEstimated := GlobalTokenEstimated
		muTokens.Unlock()

		broker.SendJSON(fmt.Sprintf(`{"event":"tokens","prompt":%d,"completion":%d,"total":%d,"session_total":%d,"global_total":%d,"is_estimated":%t}`,
			promptTokens, completionTokens, totalTokens, sessionTokens, localGlobalTotal, localIsEstimated))

		// Budget tracking: record cost and send status to UI
		if budgetTracker != nil {
			actualModel := resp.Model
			if actualModel == "" {
				actualModel = req.Model
			}
			crossedWarning := budgetTracker.Record(actualModel, promptTokens, completionTokens)
			budgetJSON := budgetTracker.GetStatusJSON()
			if budgetJSON != "" {
				broker.SendJSON(budgetJSON)
			}
			if crossedWarning {
				bs := budgetTracker.GetStatus()
				warnMsg := fmt.Sprintf("\u26a0\ufe0f Budget warning: %.0f%% used ($%.4f / $%.2f)", bs.Percentage*100, bs.SpentUSD, bs.DailyLimit)
				broker.Send("budget_warning", warnMsg)
				// Journal: record budget threshold event once per session
				if shortTermMem != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries && sessionID == "default" {
					_, _ = shortTermMem.InsertJournalEntry(memory.JournalEntry{
						EntryType:     "budget_exceeded",
						Title:         fmt.Sprintf("Budget threshold reached: %.0f%% used", bs.Percentage*100),
						Content:       fmt.Sprintf("$%.4f of $%.2f daily budget consumed", bs.SpentUSD, bs.DailyLimit),
						Importance:    3,
						SessionID:     sessionID,
						AutoGenerated: true,
					})
				}
			}
			if budgetTracker.IsExceeded() {
				bs := budgetTracker.GetStatus()
				if bs.IsBlocked {
					exMsg := fmt.Sprintf("\u26d4 Budget exceeded! $%.4f / $%.2f (enforcement: %s)", bs.SpentUSD, bs.DailyLimit, bs.Enforcement)
					broker.Send("budget_blocked", exMsg)
				} else {
					wMsg := fmt.Sprintf("\u26a0\ufe0f Budget exceeded: $%.4f / $%.2f (enforcement: %s)", bs.SpentUSD, bs.DailyLimit, bs.Enforcement)
					broker.Send("budget_warning", wMsg)
				}
			}
		}

		currentLogger.Debug("[Sync] Tool detection", "is_tool", tc.IsTool, "action", tc.Action, "raw_code", tc.RawCodeDetected)

		// Clear explicit tools after they've been consumed (they were injected this iteration)
		if len(explicitTools) > 0 {
			explicitTools = explicitTools[:0]
		}

		// Detect <workflow_plan>["tool1","tool2"]</workflow_plan> in the response
		if workflowPlanCount < 3 {
			if parsed, stripped := parseWorkflowPlan(content); len(parsed) > 0 {
				workflowPlanCount++
				explicitTools = parsed
				currentLogger.Info("[Sync] Workflow plan detected, loading tool guides", "tools", parsed, "attempt", workflowPlanCount)
				broker.Send("workflow_plan", strings.Join(parsed, ", "))

				// Store the stripped content as assistant message
				strippedContent := strings.TrimSpace(stripped)
				if strippedContent != "" {
					id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, strippedContent, false, false)
					if err != nil {
						currentLogger.Error("Failed to persist workflow plan message", "error", err)
					}
					if sessionID == "default" {
						historyManager.Add(openai.ChatMessageRoleAssistant, strippedContent, id, false, false)
					}
					req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: strippedContent})
				}

				// Inject a system nudge so the agent knows the guides are available
				nudge := fmt.Sprintf("Tool manuals loaded for: %s. Proceed with your plan.", strings.Join(parsed, ", "))
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: nudge})
				continue
			}
		}

		if tc.RawCodeDetected && rawCodeCount < 2 {
			rawCodeCount++
			currentLogger.Warn("[Sync] Raw code detected, sending corrective feedback", "attempt", rawCodeCount)
			broker.Send("error_recovery", "Raw code detected, requesting JSON format...")

			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, content, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist assistant message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, content, id, false, true)
			}

			feedbackMsg := "ERROR: You sent raw Python code instead of a JSON tool call. My supervisor only understands JSON tool calls. Please wrap your code in a valid JSON object: {\"action\": \"save_tool\", \"name\": \"script.py\", \"description\": \"...\", \"code\": \"<your python code with \\n escaped>\"}."
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, false)
			}

			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			continue
		}

		// Recovery: model sent an announcement/preamble instead of a tool call
		// Triggered when: no tool, short response, contains action-intent phrases
		announcementPhrases := []string{
			"lass mich", "ich starte", "ich werde", "ich führe", "ich teste",
			"let me", "i will", "i'll", "i am going to", "i'm going to",
			"let's start", "starting", "launching", "i'll start", "i'll run",
			"alles klar", "okay, let", "sure, let", "sure, i",
			"ich suche nach", "ich schaue nach", "ich prüfe", "ich überprüfe",
			"ich sehe mir", "lass mich sehen", "ich werde nachschauen",
			"i'll check", "let me check", "checking", "searching", "looking",
			"i am looking", "i will look", "i'll search", "i will search",
			"ich frage ab", "ich lade", "i'll load", "i am loading",
		}
		isAnnouncement := func() bool {
			if tc.IsTool || useNativePath || tc.RawCodeDetected || len(content) > 1000 {
				return false
			}
			// A response ending with '?' is a conversational reply, not an action announcement
			if strings.HasSuffix(strings.TrimRight(strings.TrimSpace(content), "\"'"), "?") {
				return false
			}
			// If the LLM just completed a tool call, a text response is a completion confirmation, not an announcement
			if lastResponseWasTool {
				return false
			}
			lc := strings.ToLower(content)
			for _, phrase := range announcementPhrases {
				if strings.Contains(lc, phrase) {
					return true
				}
			}
			return false
		}()
		if isAnnouncement && announcementCount < 2 {
			announcementCount++
			currentLogger.Warn("[Sync] Announcement-only response detected, requesting immediate tool call", "attempt", announcementCount, "content_preview", Truncate(content, 120))
			broker.Send("error_recovery", "Announcement without action detected, requesting tool call...")

			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, content, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist assistant message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, content, id, false, true)
			}

			feedbackMsg := "ERROR: You announced what you were going to do but did not output a tool call. When executing a task, your ENTIRE response must be ONLY the raw JSON tool call — no explanation before it. Output the JSON tool call NOW."
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, false)
			}

			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			continue
		}

		// Recovery: model wrapped tool call in markdown fence instead of bare JSON
		if !tc.IsTool && !tc.RawCodeDetected && missedToolCount < 2 &&
			(strings.Contains(content, "```") || strings.Contains(content, "{")) &&
			(strings.Contains(content, `"action"`) || strings.Contains(content, `'action'`)) {
			missedToolCount++
			currentLogger.Warn("[Sync] Missed tool call in fence, sending corrective feedback", "attempt", missedToolCount, "content_preview", Truncate(content, 150))
			broker.Send("error_recovery", "Tool call wrapped in fence, requesting raw JSON...")

			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, content, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist assistant message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, content, id, false, true)
			}

			feedbackMsg := "ERROR: Your response contained explanation text and/or markdown fences (```json). Tool calls MUST be a raw JSON object ONLY - no explanation before or after, no markdown, no fences. Output ONLY the JSON object, starting with { and ending with }. Example: {\"action\": \"co_agent\", \"operation\": \"spawn\", \"task\": \"...\"}"
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, false)
			}

			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			continue
		}

		// Berechne effektives Limit neu mit bekanntem tc (für Tool-spezifische Anpassungen)
		effectiveMaxCallsWithTool := calculateEffectiveMaxCalls(cfg, tc, homepageUsedInChain, personalityEnabled, shortTermMem, currentLogger)

		if tc.IsTool && toolCallCount < effectiveMaxCallsWithTool {
			toolCallCount++
			if tc.Action == "homepage" || tc.Action == "homepage_tool" {
				homepageUsedInChain = true
			}
			broker.Send("thinking", fmt.Sprintf("[%d] Running %s...", toolCallCount, tc.Action))

			// Persist tool call to history: native path synthesizes a text representation
			histContent := content

			// Decide if this message should be hidden from the UI history endpoint.
			// Hide it if it's purely a synthetic JSON string (e.g. no text, only tool call),
			// but show it if the LLM provided conversational text.
			isMsgInternal := true
			if strings.TrimSpace(content) != "" && !strings.HasPrefix(strings.TrimSpace(content), "{") {
				isMsgInternal = false
			}

			if useNativePath && histContent == "" && len(nativeAssistantMsg.ToolCalls) > 0 {
				nc := nativeAssistantMsg.ToolCalls[0]
				histContent = fmt.Sprintf("{\"action\": \"%s\"}", nc.Function.Name)
				if nc.Function.Arguments != "" && len(nc.Function.Arguments) > 2 {
					args := strings.TrimSpace(nc.Function.Arguments)
					if strings.HasPrefix(args, "{") && strings.HasSuffix(args, "}") {
						inner := args[1 : len(args)-1]
						if inner != "" {
							histContent = fmt.Sprintf("{\"action\": \"%s\", %s}", nc.Function.Name, inner)
						}
					}
				}
			}
			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, histContent, false, isMsgInternal)
			if err != nil {
				currentLogger.Error("Failed to persist tool-call message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleAssistant, histContent, id, false, isMsgInternal)
			}

			// For SSE: send only the JSON representation of the tool call.
			// In the non-native (legacy JSON) path the LLM response may include
			// conversational preamble text before the JSON object. Sending that
			// preamble would cause the UI to render it as a spurious assistant
			// message. We therefore always send the raw JSON (or a minimal
			// synthetic fallback), never the preamble text.
			// In the native function-calling path histContent is already the
			// synthetic JSON we built above, so no change is needed there.
			sseToolContent := histContent
			if !useNativePath {
				if tc.RawJSON != "" {
					sseToolContent = tc.RawJSON
				} else {
					sseToolContent = fmt.Sprintf(`{"action":"%s"}`, tc.Action)
				}
			}
			broker.Send("tool_call", sseToolContent)
			broker.Send("tool_start", tc.Action)

			if tc.Action == "execute_python" {
				flags.RequiresCoding = true
				broker.Send("coding", "Executing Python script...")
			}

			// Co-agent spawn: send a dedicated status event with a task preview
			if (tc.Action == "co_agent" || tc.Action == "co_agents") &&
				(tc.Operation == "spawn" || tc.Operation == "start" || tc.Operation == "create") {
				taskPreview := tc.Task
				if taskPreview == "" {
					taskPreview = tc.Content
				}
				if len(taskPreview) > 80 {
					taskPreview = taskPreview[:80] + "…"
				}
				broker.Send("co_agent_spawn", taskPreview)
			}

			resultContent := DispatchToolCall(ctx, tc, cfg, currentLogger, client, vault, registry, manifest, cronManager, missionManagerV2, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyManager, tools.IsBusy(), surgeryPlan, guardian, llmGuardian, sessionID, coAgentRegistry, budgetTracker, lastUserMsg)
			resultContent = truncateToolOutput(resultContent, cfg.Agent.ToolOutputLimit)
			toolFailed := isToolError(resultContent)
			prompts.RecordToolUsage(tc.Action, tc.Operation, !toolFailed)
			prompts.RecordAdaptiveToolUsage(tc.Action, !toolFailed)
			if shortTermMem != nil {
				_ = shortTermMem.UpsertToolUsage(tc.Action, !toolFailed)
			}

			// Record error patterns for learning
			if toolFailed && shortTermMem != nil {
				errMsg := extractErrorMessage(resultContent)
				if errMsg != "" {
					_ = shortTermMem.RecordError(tc.Action, errMsg)
					// Journal: log new error pattern (only on first occurrence within a chain)
					if consecutiveErrorCount == 0 && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries {
						_, _ = shortTermMem.InsertJournalEntry(memory.JournalEntry{
							EntryType:     "error_learned",
							Title:         fmt.Sprintf("Error in %s", tc.Action),
							Content:       errMsg,
							Tags:          []string{tc.Action},
							Importance:    2,
							SessionID:     sessionID,
							AutoGenerated: true,
						})
					}
				}
			}

			// Journal: record LLM Guardian block events
			if strings.Contains(resultContent, "[TOOL BLOCKED]") && shortTermMem != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries {
				reason := resultContent
				if len(reason) > 150 {
					reason = reason[:150] + "…"
				}
				_, _ = shortTermMem.InsertJournalEntry(memory.JournalEntry{
					EntryType:     "security_event",
					Title:         fmt.Sprintf("Guardian blocked: %s", tc.Action),
					Content:       reason,
					Tags:          []string{tc.Action, "security"},
					Importance:    4,
					SessionID:     sessionID,
					AutoGenerated: true,
				})
			}

			// Record resolution when a tool succeeds after previous errors
			if !toolFailed && consecutiveErrorCount > 0 && shortTermMem != nil && lastToolError != "" {
				_ = shortTermMem.RecordResolution(tc.Action, lastToolError, "Succeeded with adjusted parameters")
			}

			broker.Send("tool_output", resultContent)

			// Emit SSE image event so the Web UI shows the image immediately (before LLM responds)
			if tc.Action == "send_image" {
				var imgRes struct {
					Status  string `json:"status"`
					WebPath string `json:"web_path"`
					Caption string `json:"caption"`
				}
				raw := strings.TrimPrefix(resultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &imgRes) == nil && imgRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{
						"path":    imgRes.WebPath,
						"caption": imgRes.Caption,
					})
					broker.Send("image", string(evtPayload))
				}
			}
			if tc.Action == "send_audio" {
				var audioRes struct {
					Status   string `json:"status"`
					WebPath  string `json:"web_path"`
					Title    string `json:"title"`
					MimeType string `json:"mime_type"`
					Filename string `json:"filename"`
				}
				raw := strings.TrimPrefix(resultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &audioRes) == nil && audioRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{
						"path":      audioRes.WebPath,
						"title":     audioRes.Title,
						"mime_type": audioRes.MimeType,
						"filename":  audioRes.Filename,
					})
					broker.Send("audio", string(evtPayload))
				}
			}
			if tc.Action == "send_document" {
				var docRes struct {
					Status     string `json:"status"`
					WebPath    string `json:"web_path"`
					PreviewURL string `json:"preview_url"`
					Title      string `json:"title"`
					MimeType   string `json:"mime_type"`
					Filename   string `json:"filename"`
					Format     string `json:"format"`
				}
				raw := strings.TrimPrefix(resultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &docRes) == nil && docRes.Status == "success" {
					evtPayload, _ := json.Marshal(map[string]string{
						"path":        docRes.WebPath,
						"preview_url": docRes.PreviewURL,
						"title":       docRes.Title,
						"mime_type":   docRes.MimeType,
						"filename":    docRes.Filename,
						"format":      docRes.Format,
					})
					broker.Send("document", string(evtPayload))
				}
			}

			broker.Send("tool_end", tc.Action)
			lastActivity = time.Now() // Tool activity

			// Update session todo from piggybacked _todo field
			if tc.Todo != "" {
				sessionTodoList = tc.Todo
				broker.Send("todo_update", sessionTodoList)
			}

			// Invalidate core memory cache when it was modified
			if tc.Action == "manage_memory" {
				coreMemDirty = true
			}

			// Record transition
			if lastTool != "" {
				_ = shortTermMem.RecordToolTransition(lastTool, tc.Action)
			}
			lastTool = tc.Action
			// Track recent tools for lazy schema injection (keep last 5, dedup)
			found := false
			for _, rt := range recentTools {
				if rt == tc.Action {
					found = true
					break
				}
			}
			if !found {
				recentTools = append(recentTools, tc.Action)
				if len(recentTools) > 5 {
					recentTools = recentTools[len(recentTools)-5:]
				}
			}

			// Proactive Workflow Feedback (Phase: Keep the user engaged during long chains)
			if cfg.Agent.WorkflowFeedback && !flags.IsCoAgent && sessionID == "default" {
				stepsSinceLastFeedback++
				if stepsSinceLastFeedback >= 4 {
					stepsSinceLastFeedback = 0
					feedbackPhrases := []string{
						"Ich brauche noch einen Moment, bin aber dran...",
						"Die Analyse läuft noch, einen Augenblick bitte...",
						"Ich suche noch nach weiteren Informationen...",
						"Bin gleich fertig mit der Bearbeitung...",
						"Das dauert einen Moment länger als erwartet, bleib dran...",
						"Ich verarbeite die Daten noch...",
					}
					// Simple pseudo-random selection based on time
					phrase := feedbackPhrases[time.Now().Unix()%int64(len(feedbackPhrases))]
					broker.Send("progress", phrase)
				}
			}

			// Phase D: Mood detection after each tool call
			if personalityEnabled && shortTermMem != nil {
				triggerInfo := moodTrigger()
				if strings.Contains(resultContent, "ERROR") || strings.Contains(resultContent, "error") {
					triggerInfo = moodTrigger() + " [tool error]"
				}

				if cfg.Agent.PersonalityEngineV2 {
					// ── V2: Asynchronous LLM-Based Mood Analysis ──
					// Extract recent context (e.g. last 5 messages) for the analyzer
					recentMsgs := req.Messages
					if len(recentMsgs) > 5 {
						recentMsgs = recentMsgs[len(recentMsgs)-5:]
					}
					var historyBuilder strings.Builder
					var userHistoryBuilder strings.Builder
					for _, m := range recentMsgs {
						// Skip system messages — they contain the full agent prompt (tool guides, identity,
						// rules, etc.) and must not be fed to the mood/profile analyzer. Including them
						// causes the LLM to attribute every mentioned technology to the user's profile
						// even when the user never mentioned it.
						if m.Role == openai.ChatMessageRoleSystem {
							continue
						}
						historyBuilder.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
						// Build a user-only history to avoid attributing agent/tool content to the user profile
						if m.Role == openai.ChatMessageRoleUser {
							userHistoryBuilder.WriteString(fmt.Sprintf("user: %s\n", m.Content))
						}
					}
					historyBuilder.WriteString(fmt.Sprintf("Tool Result: %s\n", resultContent))
					// Note: Tool Results are intentionally excluded from userHistory

					var v2Client memory.PersonalityAnalyzerClient = client
					if cfg.Agent.PersonalityV2URL != "" {
						key := cfg.Agent.PersonalityV2APIKey
						if key == "" {
							key = "dummy" // Ollama sometimes requires a non-empty string
						}
						v2Cfg := openai.DefaultConfig(key)
						v2Cfg.BaseURL = cfg.Agent.PersonalityV2URL
						v2Client = openai.NewClientWithConfig(v2Cfg)
					}

					go func(contextHistory string, userHistory string, tInfo string, modelName string, analyzerClient memory.PersonalityAnalyzerClient, m memory.PersonalityMeta, profilingEnabled bool) {
						v2Ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Agent.PersonalityV2TimeoutSecs)*time.Second)
						defer cancel()

						mood, affDelta, traitDeltas, profileUpdates, err := shortTermMem.AnalyzeMoodV2(v2Ctx, analyzerClient, modelName, contextHistory, userHistory, m, profilingEnabled)
						if err != nil {
							v2URL := cfg.Agent.PersonalityV2URL
							if v2URL == "" {
								v2URL = "(main LLM endpoint)"
							}
							currentLogger.Warn("[Personality V2] Failed to analyze mood",
								"error", err,
								"model", modelName,
								"url", v2URL,
								"timeout_secs", cfg.Agent.PersonalityV2TimeoutSecs,
								"hint", "increase agent.personality_v2_timeout_secs in config if this is a timeout")
						}

						_ = shortTermMem.LogMood(mood, tInfo)
						for trait, delta := range traitDeltas {
							_ = shortTermMem.UpdateTrait(trait, delta)
						}
						_ = shortTermMem.UpdateTrait(memory.TraitAffinity, affDelta)

						// User Profiling: persist observed profile attributes
						if profilingEnabled && len(profileUpdates) > 0 {
							validCategories := map[string]bool{"tech": true, "prefs": true, "interests": true, "context": true, "comm": true}
							count := 0
							for _, pu := range profileUpdates {
								if count >= 1 {
									break // Hard limit (matches prompt)
								}
								trimVal := strings.TrimSpace(pu.Value)
								if validCategories[pu.Category] && pu.Key != "" && pu.Value != "" &&
									!strings.EqualFold(pu.Key, trimVal) && len(trimVal) >= 2 &&
									!strings.ContainsAny(pu.Key, " \t") && pu.Key == strings.ToLower(pu.Key) {
									if err := shortTermMem.UpsertProfileEntry(pu.Category, pu.Key, pu.Value, "v2"); err != nil {
										currentLogger.Warn("[User Profiling] Failed to upsert profile entry", "key", pu.Key, "error", err)
									}
									count++
								}
							}
							_ = shortTermMem.EnforceProfileSizeLimit(50)
							if err := shortTermMem.DeduplicateProfileEntries(); err != nil {
								currentLogger.Warn("[User Profiling] Deduplication failed", "error", err)
							}
							if del, down, err := shortTermMem.PruneStaleProfileEntries(); err == nil && (del > 0 || down > 0) {
								currentLogger.Debug("[User Profiling] Pruned stale entries", "deleted", del, "downgraded", down)
							}
							currentLogger.Debug("[User Profiling] Profile updates applied", "count", count)
						}

						currentLogger.Debug("[Personality V2] Asynchronous mood analysis complete", "mood", mood, "affinity_delta", affDelta)

						// Emotion Synthesizer: generate emotional description after V2 analysis
						if emotionSynthesizer != nil {
							prevMood := ""
							if prev := emotionSynthesizer.GetLastEmotion(); prev != nil {
								prevMood = string(prev.PrimaryMood)
							}
							moodChanged := prevMood != string(mood)
							shouldSynthesize := cfg.Agent.EmotionSynthesizer.TriggerAlways ||
								(cfg.Agent.EmotionSynthesizer.TriggerOnMoodChange && moodChanged) ||
								emotionSynthesizer.GetLastEmotion() == nil

							if shouldSynthesize {
								traits, _ := shortTermMem.GetTraits()
								esInput := memory.EmotionInput{
									UserMessage:  tInfo,
									CurrentMood:  mood,
									Traits:       traits,
									LastEmotion:  emotionSynthesizer.GetLastEmotion(),
									ErrorCount:   consecutiveErrorCount,
									SuccessCount: toolCallCount - consecutiveErrorCount,
									TimeOfDay:    memory.TimeOfDay(),
								}
								esCtx, esCancel := context.WithTimeout(context.Background(), 15*time.Second)
								_, _ = emotionSynthesizer.SynthesizeEmotion(esCtx, shortTermMem, esInput)
								esCancel()
							}
						}
					}(historyBuilder.String(), userHistoryBuilder.String(), triggerInfo, cfg.Agent.PersonalityV2Model, v2Client, meta, cfg.Agent.UserProfiling)

				} else {
					// ── V1: Synchronous Heuristic-Based Mood Analysis ──
					mood, traitDeltas := memory.DetectMood(lastUserMsg, resultContent, meta)
					_ = shortTermMem.LogMood(mood, triggerInfo)
					for trait, delta := range traitDeltas {
						_ = shortTermMem.UpdateTrait(trait, delta)
					}
				}
				flags.PersonalityLine = shortTermMem.GetPersonalityLine(cfg.Agent.PersonalityEngineV2)

				// Emotion Synthesizer: update flags with latest emotion if available
				if emotionSynthesizer != nil {
					if es := emotionSynthesizer.GetLastEmotion(); es != nil {
						flags.EmotionDescription = es.Description
					}
				}
			}

			if tc.NotifyOnCompletion {
				resultContent = fmt.Sprintf(
					"[TOOL COMPLETION NOTIFICATION]\nAction: %s\nStatus: Completed\nTimestamp: %s\nOutput:\n%s",
					tc.Action,
					time.Now().Format(time.RFC3339),
					resultContent,
				)
			}
			// Make sure errors from execute_python trigger recovery mode
			if tc.Action == "execute_python" {
				if strings.Contains(resultContent, "[EXECUTION ERROR]") || strings.Contains(resultContent, "TIMEOUT") {
					flags.IsErrorState = true
					broker.Send("error_recovery", "Script error detected, retrying...")
				} else {
					flags.IsErrorState = false
				}
			}
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleSystem, resultContent, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist tool-result message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleSystem, resultContent, id, false, true)
			}

			// Phase 72: Broadcast the supervisor's result to the UI (shown only in debug mode)
			broker.Send("tool_output", resultContent)

			// Phase 1: Lifecycle Handover check
			if strings.Contains(resultContent, "Maintenance Mode activated") {
				currentLogger.Info("Handover sentinel detected, Sidecar taking over...")
				// We return the response so the user sees the handover message,
				// and the loop terminates. The process stays alive in "busy" mode
				// until the sidecar triggers a reload.
				id, err := shortTermMem.InsertMessage(sessionID, resp.Choices[0].Message.Role, content, false, false)
				if err != nil {
					currentLogger.Error("Failed to persist handover message to SQLite", "error", err)
				}
				if sessionID == "default" {
					historyManager.Add(resp.Choices[0].Message.Role, content, id, false, false)
				}
				return resp, nil
			}

			if useNativePath {
				// Native path: use proper role=tool format so the LLM gets structured multi-turn context
				req.Messages = append(req.Messages, nativeAssistantMsg)
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    resultContent,
					ToolCallID: tc.NativeCallID,
				})

				// Execute batched native tool calls inline.
				// The OpenAI API requires ALL role=tool responses to be present before
				// the next API call when the assistant message contains multiple tool_calls.
				for len(pendingTCs) > 0 && pendingTCs[0].NativeCallID != "" {
					btc := pendingTCs[0]
					pendingTCs = pendingTCs[1:]
					toolCallCount++
					if btc.Action == "homepage" || btc.Action == "homepage_tool" {
						homepageUsedInChain = true
					}
					broker.Send("thinking", fmt.Sprintf("[%d] Running %s (batched)...", toolCallCount, btc.Action))
					broker.Send("tool_start", btc.Action)

					bResult := DispatchToolCall(ctx, btc, cfg, currentLogger, client, vault, registry, manifest, cronManager, missionManagerV2, longTermMem, shortTermMem, kg, inventoryDB, invasionDB, cheatsheetDB, imageGalleryDB, mediaRegistryDB, homepageRegistryDB, remoteHub, historyManager, tools.IsBusy(), surgeryPlan, guardian, llmGuardian, sessionID, coAgentRegistry, budgetTracker, lastUserMsg)
					bResult = truncateToolOutput(bResult, cfg.Agent.ToolOutputLimit)
					prompts.RecordToolUsage(btc.Action, btc.Operation, !isToolError(bResult))
					prompts.RecordAdaptiveToolUsage(btc.Action, !isToolError(bResult))
					if shortTermMem != nil {
						_ = shortTermMem.UpsertToolUsage(btc.Action, !isToolError(bResult))
					}
					broker.Send("tool_output", bResult)
					broker.Send("tool_end", btc.Action)
					lastActivity = time.Now()

					if btc.Action == "manage_memory" || btc.Action == "core_memory" {
						coreMemDirty = true
					}
					// Track recent tools for journal auto-trigger (keep last 5, dedup)
					{
						found := false
						for _, rt := range recentTools {
							if rt == btc.Action {
								found = true
								break
							}
						}
						if !found {
							recentTools = append(recentTools, btc.Action)
							if len(recentTools) > 5 {
								recentTools = recentTools[len(recentTools)-5:]
							}
						}
					}

					// Persist batched call to history
					bHistContent := fmt.Sprintf(`{"action": "%s"}`, btc.Action)
					bID, bErr := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleAssistant, bHistContent, false, true)
					if bErr != nil {
						currentLogger.Error("Failed to persist batched tool-call message", "error", bErr)
					}
					if sessionID == "default" {
						historyManager.Add(openai.ChatMessageRoleAssistant, bHistContent, bID, false, true)
					}
					bID, bErr = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleSystem, bResult, false, true)
					if bErr != nil {
						currentLogger.Error("Failed to persist batched tool-result message", "error", bErr)
					}
					if sessionID == "default" {
						historyManager.Add(openai.ChatMessageRoleSystem, bResult, bID, false, true)
					}

					req.Messages = append(req.Messages, openai.ChatCompletionMessage{
						Role:       openai.ChatMessageRoleTool,
						Content:    bResult,
						ToolCallID: btc.NativeCallID,
					})
				}
			} else {
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
				req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: resultContent})
			}

			// Support early exit for Lifeboat
			if strings.Contains(resultContent, "[LIFEBOAT_EXIT_SIGNAL]") {
				currentLogger.Info("[Sync] Early exit signal received, stopping loop.")
				return resp, nil
			}

			// Consecutive identical error circuit breaker:
			// If the agent keeps retrying the exact same failing tool call, stop it before
			// it exhausts MaxToolCalls and wastes the entire budget on pointless retries.
			// Also catches sandbox/shell failures reported as a non-zero exit_code.
			hasSandboxFailure := strings.Contains(resultContent, `"exit_code":`) &&
				!strings.Contains(resultContent, `"exit_code": 0`) &&
				!strings.Contains(resultContent, `"exit_code":0`)
			isToolError := strings.Contains(resultContent, `"status": "error"`) ||
				strings.Contains(resultContent, `"status":"error"`) ||
				strings.Contains(resultContent, `[EXECUTION ERROR]`) ||
				hasSandboxFailure
			if isToolError {
				if resultContent == lastToolError {
					consecutiveErrorCount++
					if consecutiveErrorCount >= 3 {
						currentLogger.Warn("[Sync] Consecutive identical error — circuit breaker triggered",
							"action", tc.Action, "count", consecutiveErrorCount)
						abortMsg := fmt.Sprintf(
							"CIRCUIT BREAKER: The tool '%s' returned the same error %d times in a row. "+
								"You MUST stop retrying it — calling it again will produce the exact same result. "+
								"Do NOT call '%s' again this session. "+
								"Instead: inform the user about the error, explain what likely needs to be fixed "+
								"(e.g. wrong URL, missing credentials, service unavailable), and wait for their input.",
							tc.Action, consecutiveErrorCount, tc.Action)
						req.Messages = append(req.Messages,
							openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: abortMsg})
						consecutiveErrorCount = 0
						lastToolError = ""
					}
				} else {
					consecutiveErrorCount = 1
				}
				lastToolError = resultContent
			} else {
				consecutiveErrorCount = 0
				lastToolError = ""
			}

			// 429 Mitigation: Add a delay between turns to respect rate limits (controlled by config)
			select {
			case <-time.After(time.Duration(cfg.Agent.StepDelaySeconds) * time.Second):
				// Continue to next turn
			case <-ctx.Done():
				return resp, ctx.Err()
			}
			lastResponseWasTool = true
			continue
		}

		// Final answer
		if content == "" {
			content = "[Empty Response]"
		}
		currentLogger.Debug("[Sync] Final answer", "content_len", len(content), "content_preview", Truncate(content, 200))
		broker.Send("done", "Response complete.")

		// Don't persist [Empty Response] as a real message — it pollutes future context
		isEmpty := content == "[Empty Response]"
		if !isEmpty {
			id, err := shortTermMem.InsertMessage(sessionID, resp.Choices[0].Message.Role, content, false, false)
			if err != nil {
				currentLogger.Error("Failed to persist final-answer message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(resp.Choices[0].Message.Role, content, id, false, false)
			}
		} else {
			currentLogger.Warn("[Sync] Skipping history persistence for empty response")
		}

		// Phase D: Final mood + trait update + milestone check at session end
		if personalityEnabled && shortTermMem != nil {
			mood, traitDeltas := memory.DetectMood(lastUserMsg, "", meta)
			_ = shortTermMem.LogMood(mood, moodTrigger())
			for trait, delta := range traitDeltas {
				_ = shortTermMem.UpdateTrait(trait, delta)
			}
			// Milestone check
			traits, tErr := shortTermMem.GetTraits()
			if tErr == nil {
				for _, m := range memory.CheckMilestones(traits) {
					has, err := shortTermMem.HasMilestone(m.Label)
					if err != nil {
						continue // skip on DB error
					}
					if !has {
						trigger := shortTermMem.GetLastMoodTrigger()
						details := fmt.Sprintf("%s %s %.2f", m.Trait, m.Direction, m.Threshold)
						if trigger != "" {
							details = fmt.Sprintf("%s (Trigger: %q)", details, trigger)
						}
						_ = shortTermMem.AddMilestone(m.Label, details)
					}
				}
			}
		}

		// Real-time memory analysis: async post-response extraction of memory-worthy content
		if cfg.MemoryAnalysis.Enabled && cfg.MemoryAnalysis.RealTime && !isEmpty && shortTermMem != nil {
			go func(userMsg, aResp, sid string) {
				analysisCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				runMemoryAnalysis(analysisCtx, cfg, currentLogger, shortTermMem, kg, longTermMem, userMsg, aResp, sid)
			}(lastUserMsg, content, sessionID)
		}

		// Journal auto-trigger: create entries for significant tool chains
		JournalAutoTrigger(cfg, shortTermMem, currentLogger, sessionID, recentTools, lastUserMsg)

		// Weekly reflection: async trigger if configured and due
		// Guard: only run once per day by checking if a reflection entry already exists today.
		if cfg.MemoryAnalysis.Enabled && cfg.MemoryAnalysis.WeeklyReflection && weeklyReflectionDue(cfg) && shortTermMem != nil {
			today := time.Now().Format("2006-01-02")
			existing, _ := shortTermMem.GetJournalEntries(today, today, []string{"reflection"}, 1)
			if len(existing) == 0 {
				go func() {
					reflCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
					defer cancel()
					_, err := generateMemoryReflection(reflCtx, cfg, currentLogger, shortTermMem, kg, longTermMem, client, "recent")
					if err != nil {
						currentLogger.Warn("Weekly reflection failed", "error", err)
					}
				}()
			}
		}

		return resp, nil
	}
}
