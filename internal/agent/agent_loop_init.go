package agent

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func reconcileToolPromptModeWithSchemas(flags *prompts.ContextFlags, policy *ToolingPolicy, useNativeFunctions *bool, schemaCount int, logger *slog.Logger) {
	if schemaCount > 0 || useNativeFunctions == nil || !*useNativeFunctions {
		return
	}

	*useNativeFunctions = false
	if policy != nil {
		policy.UseNativeFunctions = false
		policy.AutoEnabledNativeFunctions = false
		policy.StructuredOutputsEnabled = false
		policy.ParallelToolCallsEnabled = false
	}
	if flags != nil {
		flags.NativeToolsEnabled = false
		flags.IsTextModeModel = true
	}
	if logger != nil {
		logger.Warn("[NativeTools] Native function calling disabled for this request because no tool schemas were attached; using text JSON tool mode")
	}
}

func reconcilePromptToolModeWithRequest(flags *prompts.ContextFlags, policy *ToolingPolicy, reqTools []openai.Tool, logger *slog.Logger) {
	if flags == nil || !flags.NativeToolsEnabled || len(reqTools) > 0 {
		return
	}
	useNativeFunctions := true
	reconcileToolPromptModeWithSchemas(flags, policy, &useNativeFunctions, 0, logger)
}

// initAgentLoopState sets up all mutable state before the main agent loop begins.
func initAgentLoopState(req openai.ChatCompletionRequest, runCfg RunConfig, broker FeedbackBroker) *agentLoopState {
	s := &agentLoopState{
		req:    req,
		runCfg: runCfg,
		broker: broker,
	}

	cfg := s.runCfg.Config
	logger := s.runCfg.Logger
	client := s.runCfg.LLMClient
	shortTermMem := s.runCfg.ShortTermMem
	historyManager := s.runCfg.HistoryManager
	longTermMem := s.runCfg.LongTermMem
	kg := s.runCfg.KG
	inventoryDB := s.runCfg.InventoryDB
	invasionDB := s.runCfg.InvasionDB
	cheatsheetDB := s.runCfg.CheatsheetDB
	imageGalleryDB := s.runCfg.ImageGalleryDB
	mediaRegistryDB := s.runCfg.MediaRegistryDB
	homepageRegistryDB := s.runCfg.HomepageRegistryDB
	contactsDB := s.runCfg.ContactsDB
	plannerDB := s.runCfg.PlannerDB
	sqlConnectionsDB := s.runCfg.SQLConnectionsDB
	sqlConnectionPool := s.runCfg.SQLConnectionPool
	remoteHub := s.runCfg.RemoteHub
	vault := s.runCfg.Vault
	registry := s.runCfg.Registry
	manifest := s.runCfg.Manifest
	cronManager := s.runCfg.CronManager
	missionManagerV2 := s.runCfg.MissionManagerV2
	coAgentRegistry := s.runCfg.CoAgentRegistry
	budgetTracker := s.runCfg.BudgetTracker
	sessionID := s.runCfg.SessionID
	isMaintenance := s.runCfg.IsMaintenance
	surgeryPlan := s.runCfg.SurgeryPlan

	if shortTermMem != nil {
		InitializeAgentTelemetryPersistence(shortTermMem)
	}

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

	initialUserMsg := ""
	if len(req.Messages) > 0 {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == openai.ChatMessageRoleUser {
				txt := strings.TrimSpace(messageText(req.Messages[i]))
				if txt == "" {
					continue
				}
				initialUserMsg = txt
				break
			}
		}
	}
	adaptiveUserContext := collectRecentUserIntentText(req.Messages, 4, 800)
	if adaptiveUserContext == "" {
		adaptiveUserContext = initialUserMsg
	}
	isFirstTurn := isFirstUserMessageInSession(req.Messages)
	plannerContext := plannerPromptContextText(runCfg, initialUserMsg, time.Now(), isFirstTurn, logger)
	dailyTodoReminder := dailyTodoReminderText(runCfg, initialUserMsg, time.Now(), logger)
	operationalIssueReminder := operationalIssueReminderText(runCfg, initialUserMsg, isFirstTurn, logger)

	toolingPolicy := buildToolingPolicy(cfg, initialUserMsg)
	telemetryScope := AgentTelemetryScope{
		ProviderType: toolingPolicy.Capabilities.ProviderType,
		Model:        toolingPolicy.Capabilities.Model,
	}
	if toolingPolicy.TelemetryProfile != "default" {
		eventName := "conservative_profile_applied"
		if toolingPolicy.TelemetryProfile == "family_guarded" && toolingPolicy.IntentFamily != "" {
			eventName = "family_guarded_" + toolingPolicy.IntentFamily
		}
		RecordToolPolicyEventForScope(telemetryScope, eventName)
		logger.Info("[ToolingPolicy] Telemetry-aware conservative profile active",
			"provider", telemetryScope.ProviderType,
			"model", telemetryScope.Model,
			"tool_calls", toolingPolicy.TelemetrySnapshot.ToolCalls,
			"failure_rate", toolingPolicy.TelemetrySnapshot.FailureRate,
			"intent_family", toolingPolicy.IntentFamily,
			"family_failure_rate", toolingPolicy.FamilyTelemetry.FailureRate,
			"max_tool_guides", toolingPolicy.EffectiveMaxToolGuides)
	}
	flags := buildPromptContextFlags(runCfg, toolingPolicy, promptContextOptions{
		IsMaintenanceMode:     isMaintenance,
		SurgeryPlan:           surgeryPlan,
		WebhooksDefinitions:   webhooksDef.String(),
		SpecialistsAvailable:  specialistsAvailable(cfg),
		SpecialistsStatus:     buildSpecialistsStatus(cfg),
		SpecialistsSuggestion: buildSpecialistDelegationHint(cfg, initialUserMsg),
	})
	flags.Model = req.Model
	logger.Debug("[Agent] Context flags initialised",
		"token_budget", flags.TokenBudget,
		"session_id", runCfg.SessionID,
	)
	baseAdditionalPrompt := flags.AdditionalPrompt
	toolCallCount := 0
	rawCodeCount := 0
	xmlFallbackCount := 0
	missedToolCount := 0
	announcementCount := 0
	incompleteToolCallCount := 0
	orphanedBracketTagCount := 0 // for [TOOL_CALL] without [/TOOL_CALL] (path 7)
	orphanedXMLTagCount := 0     // for <tool_call> XML in native mode (path 8)
	invalidNativeToolCount := 0
	sessionTokens := 0
	recoveryPolicy := buildRecoveryPolicy(cfg)
	recoverySession := NewRecoverySessionState(logger, broker, cfg)
	emptyRetried := false // Prevents infinite retry on persistent empty responses
	retry422Count := 0    // Counts consecutive 422 retries — capped to prevent infinite loops
	stepsSinceLastFeedback := 0
	homepageUsedInChain := false // Elevated circuit breaker once homepage tool is first used
	// sessionUsedTools tracks every tool called in this conversation so AdaptiveTools
	// always re-includes them next turn (Option 3: context-based alwaysInclude expansion).
	sessionUsedTools := make(map[string]bool)

	// Guardian: prompt injection defense
	guardian := security.NewGuardianWithOptions(logger, security.GuardianOptions{
		MaxScanBytes:  cfg.Guardian.MaxScanBytes,
		ScanEdgeBytes: cfg.Guardian.ScanEdgeBytes,
		Preset:        cfg.Guardian.PromptSec.Preset,
		Spotlight:     cfg.Guardian.PromptSec.Spotlight,
		Canary:        cfg.Guardian.PromptSec.Canary,
	})
	tools.ConfigureTimeouts(cfg.Tools.PythonTimeoutSeconds, cfg.Tools.SkillTimeoutSeconds, cfg.Tools.BackgroundTimeoutSeconds)

	// LLM Guardian: AI-powered pre-execution tool call security
	// Use the shared instance from RunConfig (so metrics are visible to the dashboard),
	// falling back to a fresh instance for callers that don't provide one.
	llmGuardian := runCfg.LLMGuardian
	if llmGuardian == nil {
		llmGuardian = security.NewLLMGuardian(cfg, logger)
	}

	var currentLogger *slog.Logger = logger
	helperManager := newHelperLLMManager(cfg, logger)
	lastActivity := time.Now()
	lastTool := ""
	recoveryState := newToolRecoveryStateWithPolicy(recoveryPolicy)
	recentTools := make([]string, 0, 5) // Track last 5 tools for lazy schema injection
	explicitTools := make([]string, 0)  // Explicit tool guides requested via <workflow_plan> tag
	workflowPlanCount := 0              // Prevent infinite workflow_plan loops
	lastResponseWasTool := false        // True when the previous iteration was a tool call; suppresses announcement detector on completion messages
	ragLastUserMsg := ""
	ragToolIterationsSinceLastRefresh := 0
	pendingTCs := make([]ToolCall, 0) // Queued tool calls from multi-tool responses (processed without a new LLM call)
	pendingSummaryBatch := map[string]string(nil)
	usedMemoryDocIDs := make(map[string]int)
	turnToolNames := make([]string, 0, 8)
	turnToolSummaries := make([]string, 0, 12)

	// Context compression: tracks message count at last compression for cooldown
	lastCompressionMsg := 0

	// Core memory cache: read once, invalidate on manage_memory calls
	// and when the DB updated_at timestamp changes (external modifications).
	// TTL soft-failsafe: if neither dirty nor version-changed, still re-check
	// after coreMemCacheTTL to catch external modifications that didn't update
	// the MAX(updated_at) in a detectable way.
	coreMemCache := ""
	coreMemUpdatedAt := time.Time{}
	coreMemLoadedAt := time.Time{}
	coreMemDirty := true // Force initial load
	tokenCache := newTokenCountCache(4096)
	detectedCtxWindow := 0

	const systemPromptCacheTTL = 30 * time.Second
	cachedSysPromptKey := ""
	cachedSysPrompt := ""
	cachedSysPromptTokens := 0
	cachedSysPromptAt := time.Time{}

	// Session-scoped todo list piggybacked on tool calls
	sessionTodoList := ""

	// Phase D: Personality Engine (opt-in)
	personalityEnabled := cfg.Personality.Engine
	if personalityEnabled && shortTermMem != nil {
		if err := shortTermMem.InitPersonalityTables(); err != nil {
			logger.Error("[Personality] Failed to init tables, disabling", "error", err)
			personalityEnabled = false
		}
	}

	// Emotion Synthesizer (requires Personality Engine V2)
	var emotionSynthesizer *memory.EmotionSynthesizer
	if cfg.Personality.EmotionSynthesizer.Enabled && personalityEnabled && cfg.Personality.EngineV2 {
		// Reuse V2 client setup — prefer resolved provider fields, fall back to legacy inline fields
		esClient := resolvePersonalityAnalyzerClient(cfg, client)
		esModel := resolvePersonalityModel(cfg)
		emotionSynthesizer = memory.NewEmotionSynthesizer(
			esClient,
			esModel,
			cfg.Personality.EmotionSynthesizer.MinIntervalSecs,
			cfg.Personality.EmotionSynthesizer.MaxHistoryEntries,
			cfg.Agent.SystemLanguage,
			currentLogger,
		)
		logger.Info("[EmotionSynthesizer] Initialized", "model", esModel, "interval_secs", cfg.Personality.EmotionSynthesizer.MinIntervalSecs)
		if cfg.Personality.InnerVoice.Enabled {
			logger.Info("[InnerVoice] Enabled", "min_interval_secs", cfg.Personality.InnerVoice.MinIntervalSecs, "max_per_session", cfg.Personality.InnerVoice.MaxPerSession, "decay_turns", cfg.Personality.InnerVoice.DecayTurns)
		}
	}

	// Native function calling: build tool schemas once and attach to request
	toolGuidesDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")

	// Auto-detect DeepSeek and enable native function calling
	useNativeFunctions := toolingPolicy.UseNativeFunctions
	if toolingPolicy.AutoEnabledNativeFunctions {
		logger.Info("[NativeTools] DeepSeek detected, auto-enabling native function calling")
	}

	adaptiveFilteredTools := make([]string, 0)

	if useNativeFunctions {
		ff := buildToolFeatureFlags(runCfg, toolingPolicy)
		ntSchemas := BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, logger)
		allSchemas := make([]openai.Tool, len(ntSchemas))
		copy(allSchemas, ntSchemas)

		// Adaptive tool filtering: remove rarely-used tools to save tokens
		if cfg.Agent.AdaptiveTools.Enabled && shortTermMem != nil {
			halfLife := cfg.Agent.AdaptiveTools.DecayHalfLifeDays
			if halfLife <= 0 {
				halfLife = 7.0
			}
			frequent := prompts.GetFrequentToolsWeighted(0, halfLife, cfg.Agent.AdaptiveTools.WeightSuccessRate) // 0 = all scored tools
			var guideSearcher toolGuideSearcher
			if gs, ok := longTermMem.(toolGuideSearcher); ok {
				guideSearcher = gs
			}
			prioritized := buildAdaptiveToolPriority(ntSchemas, frequent, adaptiveUserContext, guideSearcher, logger)
			maxTools := cfg.Agent.AdaptiveTools.MaxTools

			alwaysInclude := make([]string, len(cfg.Agent.AdaptiveTools.AlwaysInclude))
			copy(alwaysInclude, cfg.Agent.AdaptiveTools.AlwaysInclude)
			if GetVoiceMode() && ff.TTSEnabled {
				alwaysInclude = append(alwaysInclude, "tts", "send_audio")
			}
			if ff.MusicGenerationEnabled {
				alwaysInclude = append(alwaysInclude, "generate_music")
			}
			if ff.VideoGenerationEnabled {
				alwaysInclude = append(alwaysInclude, "generate_video")
			}
			if ff.ImageGenerationEnabled {
				alwaysInclude = append(alwaysInclude, "generate_image")
			}
			alwaysInclude = cacheAwareAdaptiveAlwaysInclude(adaptiveUserContext, alwaysInclude, ntSchemas)
			// Re-include every tool that was actually called in this conversation so the
			// model can continue using tools it already relied on (Option 3: session context).
			for tool := range sessionUsedTools {
				alwaysInclude = append(alwaysInclude, tool)
			}
			// Re-include hidden tools the agent explicitly inspected via discover_tools
			// so the next turn can use native function-calling instead of improvising.
			alwaysInclude = append(alwaysInclude, GetDiscoverRequestedTools(sessionID)...)
			alwaysInclude = expandAdaptiveAlwaysInclude(cfg, alwaysInclude)

			if maxTools > 0 && len(prioritized) > 0 {
				ntSchemas = filterToolSchemas(ntSchemas, prioritized, alwaysInclude, maxTools, logger)
			}
			// Track tools removed by adaptive filtering so their guides are also skipped
			remainingSet := make(map[string]bool, len(ntSchemas))
			for _, t := range ntSchemas {
				if t.Function != nil {
					remainingSet[t.Function.Name] = true
				}
			}
			for _, t := range allSchemas {
				if t.Function != nil && !remainingSet[t.Function.Name] {
					adaptiveFilteredTools = append(adaptiveFilteredTools, t.Function.Name)
				}
			}
		}
		// Update discover_tools state so the agent can browse hidden tools
		SetDiscoverToolsState(sessionID, allSchemas, ntSchemas, cfg.Directories.PromptsDir)

		if len(ntSchemas) == 0 {
			reconcileToolPromptModeWithSchemas(&flags, &toolingPolicy, &useNativeFunctions, len(ntSchemas), logger)
		}

		// Structured Outputs: set Strict=true on every tool definition so the
		// provider uses constrained decoding for tool-call arguments.
		// Only enable this for models that support structured outputs (e.g. GPT-4o,
		// some OpenRouter models). Ollama supports structured outputs via
		// response_format, but does not yet honor Function.Strict=true in the
		// OpenAI-compatible chat completions API, so we skip it to avoid sending
		// an unsupported field.
		if useNativeFunctions && toolingPolicy.StructuredOutputsEnabled {
			for i := range ntSchemas {
				if ntSchemas[i].Function != nil {
					ntSchemas[i].Function.Strict = true
				}
			}
			logger.Info("[NativeTools] Structured outputs enabled (strict mode)")
		} else if useNativeFunctions && toolingPolicy.StructuredOutputsRequested && toolingPolicy.Capabilities.IsOllama {
			logger.Warn("[NativeTools] Strict tool definitions not supported by Ollama, ignoring strict mode")
		}
		if useNativeFunctions {
			req.Tools = ntSchemas
			req.ToolChoice = "auto"
			if toolingPolicy.ParallelToolCallsEnabled {
				req.ParallelToolCalls = true
			}
			logger.Info("[NativeTools] Native function calling enabled", "tool_count", len(ntSchemas), "parallel", toolingPolicy.ParallelToolCallsEnabled)
		}
	}

	// Store mutable state back into struct
	s.req = req
	s.flags = flags
	s.toolingPolicy = toolingPolicy
	s.telemetryScope = telemetryScope
	s.initialUserMsg = initialUserMsg
	s.dailyTodoReminder = dailyTodoReminder
	s.operationalIssueReminder = operationalIssueReminder
	s.plannerContext = plannerContext
	s.baseAdditionalPrompt = baseAdditionalPrompt
	s.toolCallCount = toolCallCount
	s.rawCodeCount = rawCodeCount
	s.xmlFallbackCount = xmlFallbackCount
	s.missedToolCount = missedToolCount
	s.announcementCount = announcementCount
	s.incompleteToolCallCount = incompleteToolCallCount
	s.orphanedBracketTagCount = orphanedBracketTagCount
	s.orphanedXMLTagCount = orphanedXMLTagCount
	s.invalidNativeToolCount = invalidNativeToolCount
	s.sessionTokens = sessionTokens
	s.recoveryPolicy = recoveryPolicy
	s.recoverySession = recoverySession
	s.emptyRetried = emptyRetried
	s.retry422Count = retry422Count
	s.stepsSinceLastFeedback = stepsSinceLastFeedback
	s.homepageUsedInChain = homepageUsedInChain
	s.sessionUsedTools = sessionUsedTools
	s.guardian = guardian
	s.llmGuardian = llmGuardian
	s.currentLogger = currentLogger
	s.helperManager = helperManager
	s.lastActivity = lastActivity
	s.lastTool = lastTool
	s.recoveryState = recoveryState
	s.recentTools = recentTools
	s.explicitTools = explicitTools
	s.workflowPlanCount = workflowPlanCount
	s.lastResponseWasTool = lastResponseWasTool
	s.ragLastUserMsg = ragLastUserMsg
	s.ragToolIterationsSinceLastRefresh = ragToolIterationsSinceLastRefresh
	s.pendingTCs = pendingTCs
	s.pendingSummaryBatch = pendingSummaryBatch
	s.usedMemoryDocIDs = usedMemoryDocIDs
	s.turnToolNames = turnToolNames
	s.turnToolSummaries = turnToolSummaries
	s.lastCompressionMsg = lastCompressionMsg
	s.coreMemCache = coreMemCache
	s.coreMemUpdatedAt = coreMemUpdatedAt
	s.coreMemLoadedAt = coreMemLoadedAt
	s.coreMemDirty = coreMemDirty
	s.tokenCache = tokenCache
	s.detectedCtxWindow = detectedCtxWindow
	s.cachedSysPromptKey = cachedSysPromptKey
	s.cachedSysPrompt = cachedSysPrompt
	s.cachedSysPromptTokens = cachedSysPromptTokens
	s.cachedSysPromptAt = cachedSysPromptAt
	s.sessionTodoList = sessionTodoList
	s.personalityEnabled = personalityEnabled
	s.emotionSynthesizer = emotionSynthesizer
	s.toolGuidesDir = toolGuidesDir
	s.useNativeFunctions = useNativeFunctions
	s.adaptiveFilteredTools = adaptiveFilteredTools
	s.isMaintenance = isMaintenance

	// Suppress unused-variable warnings for values only consumed by makeDispatchContext
	_ = historyManager
	_ = longTermMem
	_ = kg
	_ = inventoryDB
	_ = invasionDB
	_ = cheatsheetDB
	_ = imageGalleryDB
	_ = mediaRegistryDB
	_ = homepageRegistryDB
	_ = contactsDB
	_ = plannerDB
	_ = sqlConnectionsDB
	_ = sqlConnectionPool
	_ = remoteHub
	_ = vault
	_ = registry
	_ = manifest
	_ = cronManager
	_ = missionManagerV2
	_ = coAgentRegistry
	_ = budgetTracker
	_ = sessionID
	_ = surgeryPlan

	return s
}
