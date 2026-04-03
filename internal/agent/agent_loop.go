package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
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

func retrievalLatencyBucket(elapsed time.Duration) string {
	switch {
	case elapsed < 50*time.Millisecond:
		return "lt_50ms"
	case elapsed < 150*time.Millisecond:
		return "50_149ms"
	case elapsed < 500*time.Millisecond:
		return "150_499ms"
	case elapsed < time.Second:
		return "500_999ms"
	default:
		return "ge_1000ms"
	}
}

func retrievalPromptTokenBucket(tokens int) string {
	switch {
	case tokens <= 0:
		return "0"
	case tokens <= 128:
		return "1_128"
	case tokens <= 384:
		return "129_384"
	case tokens <= 768:
		return "385_768"
	default:
		return "ge_769"
	}
}

func retrievalPromptShareBucket(tokens, budget int) string {
	if tokens <= 0 || budget <= 0 {
		return "0_pct"
	}
	share := (float64(tokens) / float64(budget)) * 100
	switch {
	case share <= 10:
		return "1_10_pct"
	case share <= 25:
		return "11_25_pct"
	case share <= 40:
		return "26_40_pct"
	default:
		return "gt_40_pct"
	}
}

func recordRetrievalPromptTelemetry(scope AgentTelemetryScope, retrievalTokens, tokenBudget int) {
	RecordRetrievalEventForScope(scope, "memory_prompt_tokens:"+retrievalPromptTokenBucket(retrievalTokens))
	RecordRetrievalEventForScope(scope, "memory_prompt_share:"+retrievalPromptShareBucket(retrievalTokens, tokenBudget))
	if retrievalTokens > 0 {
		share := 0
		if tokenBudget > 0 {
			share = int(math.Round((float64(retrievalTokens) / float64(tokenBudget)) * 100))
		}
		RecordRetrievalEventForScope(scope, fmt.Sprintf("memory_prompt_share_value:%d", share))
	}
}

const ragRefreshAfterToolIterations = 2
const maxConcurrentAgentLoops = 8

var agentLoopLimiter = make(chan struct{}, maxConcurrentAgentLoops)

func shouldRefreshRAG(query, lastQuery string, toolIterations int) bool {
	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery == "" {
		return false
	}
	if trimmedQuery != strings.TrimSpace(lastQuery) {
		return true
	}
	return toolIterations >= ragRefreshAfterToolIterations
}

func buildTrimmedContextRecap(messages []openai.ChatCompletionMessage, tokenBudget int) string {
	if len(messages) == 0 || tokenBudget <= 0 {
		return ""
	}
	start := 0
	if len(messages) > 6 {
		start = len(messages) - 6
	}
	var builder strings.Builder
	builder.WriteString("[TRIMMED_CONTEXT_RECAP]: Older conversation content was condensed to stay within the model context window. Use this only as supporting context and do not quote it verbatim.\n")
	if start > 0 {
		builder.WriteString(fmt.Sprintf("Earlier omitted messages before this recap: %d\n", start))
	}
	for _, msg := range messages[start:] {
		content := strings.Join(strings.Fields(msg.Content), " ")
		if content == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(msg.Role)
		builder.WriteString(": ")
		builder.WriteString(Truncate(content, 220))
		builder.WriteString("\n")
	}
	recap := strings.TrimSpace(builder.String())
	for recap != "" && prompts.CountTokens(recap) > tokenBudget {
		if len(recap) <= 160 {
			return ""
		}
		recap = strings.TrimSpace(Truncate(recap, len(recap)-(len(recap)/4)))
	}
	return recap
}

// ExecuteAgentLoop executes the multi-turn reasoning and tool execution loop.
// It supports both synchronous returns and asynchronous streaming via the broker.
func ExecuteAgentLoop(ctx context.Context, req openai.ChatCompletionRequest, runCfg RunConfig, stream bool, broker FeedbackBroker) (openai.ChatCompletionResponse, error) {
	select {
	case agentLoopLimiter <- struct{}{}:
		defer func() { <-agentLoopLimiter }()
	case <-ctx.Done():
		return openai.ChatCompletionResponse{}, ctx.Err()
	}

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
	contactsDB := runCfg.ContactsDB
	sqlConnectionsDB := runCfg.SQLConnectionsDB
	sqlConnectionPool := runCfg.SQLConnectionPool
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
			if req.Messages[i].Role == openai.ChatMessageRoleUser && strings.TrimSpace(req.Messages[i].Content) != "" {
				initialUserMsg = req.Messages[i].Content
				break
			}
		}
	}

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
	logger.Debug("[Agent] Context flags initialised",
		"token_budget", flags.TokenBudget,
		"session_id", runCfg.SessionID,
	)
	baseAdditionalPrompt := flags.AdditionalPrompt
	toolCallCount := 0
	rawCodeCount := 0
	missedToolCount := 0
	announcementCount := 0
	invalidNativeToolCount := 0
	sessionTokens := 0
	recoveryPolicy := buildRecoveryPolicy(cfg)
	emptyRetried := false // Prevents infinite retry on persistent empty responses
	retry422Count := 0    // Counts consecutive 422 retries — capped to prevent infinite loops
	stepsSinceLastFeedback := 0
	homepageUsedInChain := false // Elevated circuit breaker once homepage tool is first used

	// Guardian: prompt injection defense
	guardian := security.NewGuardianWithOptions(logger, security.GuardianOptions{
		MaxScanBytes:  cfg.Guardian.MaxScanBytes,
		ScanEdgeBytes: cfg.Guardian.ScanEdgeBytes,
	})
	tools.ConfigureTimeouts(cfg.Tools.PythonTimeoutSeconds, cfg.Tools.SkillTimeoutSeconds, cfg.Tools.BackgroundTimeoutSeconds)

	// LLM Guardian: AI-powered pre-execution tool call security
	// Use the shared instance from RunConfig (so metrics are visible to the dashboard),
	// falling back to a fresh instance for callers that don't provide one.
	llmGuardian := runCfg.LLMGuardian
	if llmGuardian == nil {
		llmGuardian = security.NewLLMGuardian(cfg, logger)
	}

	makeDispatchContext := func(currentLogger *slog.Logger) *DispatchContext {
		return &DispatchContext{
			Cfg: cfg, Logger: currentLogger, LLMClient: client, Vault: vault,
			Registry: registry, Manifest: manifest, CronManager: cronManager,
			MissionManagerV2: missionManagerV2, LongTermMem: longTermMem,
			ShortTermMem: shortTermMem, KG: kg, InventoryDB: inventoryDB,
			InvasionDB: invasionDB, CheatsheetDB: cheatsheetDB,
			ImageGalleryDB: imageGalleryDB, MediaRegistryDB: mediaRegistryDB,
			HomepageRegistryDB: homepageRegistryDB, ContactsDB: contactsDB,
			SQLConnectionsDB: sqlConnectionsDB, SQLConnectionPool: sqlConnectionPool,
			RemoteHub: remoteHub, HistoryMgr: historyManager,
			IsMaintenance: tools.IsBusy(), SurgeryPlan: surgeryPlan,
			Guardian: guardian, LLMGuardian: llmGuardian,
			SessionID: sessionID, CoAgentRegistry: coAgentRegistry,
			BudgetTracker: budgetTracker,
		}
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
	ragToolIterationsSinceLastRefresh := ragRefreshAfterToolIterations
	pendingTCs := make([]ToolCall, 0) // Queued tool calls from multi-tool responses (processed without a new LLM call)
	pendingSummaryBatch := map[string]string(nil)
	usedMemoryDocIDs := make(map[string]int)
	turnToolNames := make([]string, 0, 8)
	turnToolSummaries := make([]string, 0, 12)

	// Context compression: tracks message count at last compression for cooldown
	lastCompressionMsg := 0

	// Core memory cache: read once, invalidate on manage_memory calls
	coreMemCache := ""
	coreMemDirty := true // Force initial load

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
	}

	// Native function calling: build tool schemas once and attach to request
	toolGuidesDir := filepath.Join(cfg.Directories.PromptsDir, "tools_manuals")

	// Auto-detect DeepSeek and enable native function calling
	useNativeFunctions := toolingPolicy.UseNativeFunctions
	if toolingPolicy.AutoEnabledNativeFunctions {
		logger.Info("[NativeTools] DeepSeek detected, auto-enabling native function calling")
	}

	if useNativeFunctions {
		ff := buildToolFeatureFlags(runCfg, toolingPolicy)
		ntSchemas := BuildNativeToolSchemas(cfg.Directories.SkillsDir, manifest, ff, logger)

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
			prioritized := buildAdaptiveToolPriority(ntSchemas, frequent, initialUserMsg, guideSearcher, logger)
			maxTools := cfg.Agent.AdaptiveTools.MaxTools
			
			alwaysInclude := make([]string, len(cfg.Agent.AdaptiveTools.AlwaysInclude))
			copy(alwaysInclude, cfg.Agent.AdaptiveTools.AlwaysInclude)
			if GetVoiceMode() {
				alwaysInclude = append(alwaysInclude, "tts", "send_audio")
			}

			if maxTools > 0 && len(prioritized) > 0 {
				ntSchemas = filterToolSchemas(ntSchemas, prioritized, alwaysInclude, maxTools, logger)
			}
		}
		// Structured Outputs: set Strict=true on every tool definition so the
		// provider uses constrained decoding for tool-call arguments.
		// Only enable this for models that support structured outputs (e.g. GPT-4o,
		// some OpenRouter models). Ollama does not support strict mode.
		if toolingPolicy.StructuredOutputsEnabled {
			for i := range ntSchemas {
				if ntSchemas[i].Function != nil {
					ntSchemas[i].Function.Strict = true
				}
			}
			logger.Info("[NativeTools] Structured outputs enabled (strict mode)")
		} else if toolingPolicy.StructuredOutputsRequested && toolingPolicy.Capabilities.IsOllama {
			logger.Warn("[NativeTools] Structured outputs not supported by Ollama, ignoring")
		}
		req.Tools = ntSchemas
		req.ToolChoice = "auto"
		if toolingPolicy.ParallelToolCallsEnabled {
			req.ParallelToolCalls = true
		}
		logger.Info("[NativeTools] Native function calling enabled", "tool_count", len(ntSchemas), "parallel", toolingPolicy.ParallelToolCallsEnabled)
	}

	for {
		emotionPolicy := emotionBehaviorPolicy{}
		if !runCfg.IsMission && personalityEnabled && shortTermMem != nil {
			emotionPolicy = deriveEmotionBehaviorPolicy(shortTermMem, emotionSynthesizer)
		}

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

		if lastResponseWasTool {
			ragToolIterationsSinceLastRefresh++
		}

		// Tool results are appended as user-role messages; keep loop-scoped context anchored
		// to the original human request instead of the latest tool output.
		lastUserMsg := initialUserMsg
		userEmotionTrigger, userEmotionTriggerDetail, userInactivityHours := detectUserEmotionTrigger(lastUserMsg, shortTermMem, sessionID)

		// Process queued tool calls from multi-tool responses (skip LLM for these)
		if len(pendingTCs) > 0 {
			dispatchCtx := makeDispatchContext(currentLogger)
			if helperManager != nil && len(pendingSummaryBatch) == 0 {
				pendingSummaryBatch = maybeBuildPendingSummaryBatch(ctx, pendingTCs, dispatchCtx, helperManager, lastUserMsg)
			}

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
			pResultContent := ""
			if precomputed, ok := pendingSummaryBatch[pendingSummaryBatchKey(ptc)]; ok {
				pResultContent = precomputed
				delete(pendingSummaryBatch, pendingSummaryBatchKey(ptc))
				if len(pendingSummaryBatch) == 0 {
					pendingSummaryBatch = nil
				}
			} else if recoveryState.handleDuplicateToolCall(ptc, &req, currentLogger, telemetryScope) {
				pResultContent = blockedToolOutputFromRequest(&req)
			} else {
				pResultContent = DispatchToolCall(ctx, ptc, dispatchCtx, lastUserMsg)
			}
			policyResult := finalizeToolExecution(ptc, pResultContent, cfg, shortTermMem, sessionID, &recoveryState, &req, currentLogger, telemetryScope)
			pResultContent = policyResult.Content
			trackActivityTool(&turnToolNames, &turnToolSummaries, ptc.Action, pResultContent)
			recordPlanToolProgress(shortTermMem, sessionID, ptc, pResultContent, currentLogger)
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
			if ptc.Action == "tts" {
				var ttsRes struct {
					Status string `json:"status"`
					File   string `json:"file"`
				}
				raw := strings.TrimPrefix(pResultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &ttsRes) == nil && ttsRes.Status == "success" {
					mimeType := "audio/mpeg"
					if strings.HasSuffix(ttsRes.File, ".wav") {
						mimeType = "audio/wav"
					}
					evtPayload, _ := json.Marshal(map[string]string{
						"path":      "/tts/" + ttsRes.File,
						"title":     "TTS Audio",
						"mime_type": mimeType,
						"filename":  ttsRes.File,
						"file_path": filepath.Join(cfg.Directories.DataDir, "tts", ttsRes.File),
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
				sessionTodoList = string(ptc.Todo)
				broker.Send("todo_update", sessionTodoList)
			}
			if ptc.Action == "manage_plan" {
				emitSessionPlanUpdate(broker, shortTermMem, sessionID, currentLogger)
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
		flags.PredictedGuides = prompts.PrepareDynamicGuidesWithStrategy(
			longTermMem,
			shortTermMem,
			lastUserMsg,
			lastTool,
			toolGuidesDir,
			recentTools,
			explicitTools,
			toolingPolicy.EffectiveMaxToolGuides,
			toolingPolicy.EffectiveGuideStrategy,
			currentLogger,
		)
		turnMemoryCandidates := make(map[string]string)
		turnPendingActions := make([]memory.EpisodicMemory, 0, 2)

		// Automatic RAG: retrieve relevant long-term memories for the current user message
		// Phase A3: Over-fetch and re-rank with recency boost from memory_meta
		flags.RetrievedMemories = ""
		flags.PredictedMemories = ""
		retrievalPromptTokens := 0
		var topMemories []string
		if !runCfg.IsMission && longTermMem != nil && shouldUseRAGForMessage(lastUserMsg) && shouldRefreshRAG(lastUserMsg, ragLastUserMsg, ragToolIterationsSinceLastRefresh) {
			ragSettings := resolveMemoryAnalysisSettings(cfg, shortTermMem)
			useHelperRAGBatch := helperManager != nil && ragSettings.Enabled && ragSettings.QueryExpansion && ragSettings.LLMReranking
			ragQuery := lastUserMsg
			if !useHelperRAGBatch {
				ragQuery = expandQueryForRAG(ctx, cfg, currentLogger, lastUserMsg, shortTermMem)
			}
			ragLastUserMsg = lastUserMsg
			ragToolIterationsSinceLastRefresh = 0

			// Over-fetch 6 candidates, then re-rank to keep best 3
			RecordRetrievalEventForScope(telemetryScope, "rag_auto_attempt")
			autoRetrievalStart := time.Now()
			searchLimit := 6
			if useHelperRAGBatch {
				searchLimit = 8
			}
			memories, docIDs, err := longTermMem.SearchMemoriesOnly(ragQuery, searchLimit)
			RecordRetrievalEventForScope(telemetryScope, "rag_auto_latency:"+retrievalLatencyBucket(time.Since(autoRetrievalStart)))
			if err != nil {
				RecordRetrievalEventForScope(telemetryScope, "rag_auto_error")
			}
			if err == nil {
				ranked := rankMemoryCandidates(memories, docIDs, shortTermMem, usedMemoryDocIDs, time.Now())
				if useHelperRAGBatch {
					batchCtx, batchCancel := context.WithTimeout(ctx, 1800*time.Millisecond)
					batchResult, batchErr := helperManager.AnalyzeRAG(batchCtx, lastUserMsg, ranked)
					batchCancel()
					if batchErr != nil {
						helperManager.ObserveFallback("rag_batch", batchErr.Error())
						ragQuery = expandQueryForRAG(ctx, cfg, currentLogger, lastUserMsg, shortTermMem)
						memories, docIDs, err = longTermMem.SearchMemoriesOnly(ragQuery, 6)
						if err == nil {
							ranked = rankMemoryCandidates(memories, docIDs, shortTermMem, usedMemoryDocIDs, time.Now())
							ranked = rerankWithLLM(ctx, cfg, currentLogger, ranked, lastUserMsg, shortTermMem)
						} else {
							ranked = nil
						}
					} else {
						if helperQuery := strings.TrimSpace(batchResult.SearchQuery); helperQuery != "" && !strings.EqualFold(helperQuery, strings.TrimSpace(lastUserMsg)) {
							ragQuery = helperQuery
							extraMemories, extraDocIDs, extraErr := longTermMem.SearchMemoriesOnly(ragQuery, 4)
							if extraErr == nil && len(extraMemories) > 0 {
								extraRanked := rankMemoryCandidates(extraMemories, extraDocIDs, shortTermMem, usedMemoryDocIDs, time.Now())
								existing := make(map[string]struct{}, len(ranked))
								for _, item := range ranked {
									existing[item.docID] = struct{}{}
								}
								for _, item := range extraRanked {
									if _, ok := existing[item.docID]; ok {
										continue
									}
									existing[item.docID] = struct{}{}
									ranked = append(ranked, item)
								}
							}
						}
						ranked = applyHelperRAGScores(currentLogger, ranked, batchResult)
					}
				} else {
					// LLM re-ranking: blend LLM relevance scores with policy-ranked scores
					ranked = rerankWithLLM(ctx, cfg, currentLogger, ranked, lastUserMsg, shortTermMem)
				}

				// For short queries (<40 chars), apply stricter score filtering to
				// avoid injecting semantically-similar but contextually-irrelevant
				// old memories (e.g. "versuche es erneut" matching old error messages).
				if len(lastUserMsg) < 40 {
					var filtered []rankedMemory
					for _, r := range ranked {
						if r.score >= 0.65 {
							filtered = append(filtered, r)
						}
					}
					ranked = filtered
					if len(ranked) > 0 {
						currentLogger.Debug("[RAG] Short-query filter applied", "before", len(memories), "after", len(ranked))
					}
				}

				if len(ranked) > 3 {
					ranked = ranked[:3]
				}
				for _, r := range ranked {
					_ = shortTermMem.UpdateMemoryAccess(r.docID)
					_ = shortTermMem.RecordMemoryUsage(r.docID, "ltm_retrieved", sessionID, r.score, false)
				}
				markMemoryDocIDsUsed(usedMemoryDocIDs, ranked)
				wantsDeepDetails := wantsDetailedMemory(lastUserMsg)
				for _, r := range ranked {
					if !shouldServeRAGMemory(r.text) {
						currentLogger.Debug("[RAG] Dropped stale transient memory", "preview", Truncate(r.text, 80))
						continue
					}
					compactText := compactMemoryForPrompt(r.text, 260)
					topMemories = append(topMemories, compactText)
					turnMemoryCandidates[r.docID] = compactText
				}
				if wantsDeepDetails {
					for i, r := range ranked {
						if i >= 2 {
							break
						}
						full, ferr := longTermMem.GetByID(r.docID)
						if ferr == nil && full != "" && shouldServeRAGMemory(full) {
							detailed := compactMemoryForPrompt(full, 700)
							topMemories = append(topMemories, "[Detailed Memory]\n"+detailed)
							turnMemoryCandidates[r.docID] = detailed
						}
					}
				}
				flags.RetrievedMemories = strings.Join(topMemories, "\n---\n")
				if flags.RetrievedMemories != "" {
					retrievalPromptTokens += prompts.CountTokens(flags.RetrievedMemories)
					RecordRetrievalEventForScope(telemetryScope, "rag_auto_hit")
					RecordRetrievalEventForScope(telemetryScope, "rag_auto_source:ltm")
				} else {
					RecordRetrievalEventForScope(telemetryScope, "rag_auto_filtered_out")
				}
				currentLogger.Debug("[Sync] RAG: Retrieved memories (recency-boosted)", "count", len(ranked))
			} else {
				RecordRetrievalEventForScope(telemetryScope, "rag_auto_miss")
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
				temporalPredictions, err := shortTermMem.PredictNextQuery(lastTool, now.Hour(), int(now.Weekday()), 2)
				if err == nil && len(temporalPredictions) > 0 {
					predictions := buildPredictiveMemoryQueries(lastUserMsg, lastTool, temporalPredictions, 3)
					if len(predictions) == 0 {
						predictions = temporalPredictions
					}
					// Build set of already-retrieved memory texts for dedup
					retrievedSet := make(map[string]struct{})
					for _, r := range topMemories {
						retrievedSet[r] = struct{}{}
					}

					var predictedResults []string
					RecordRetrievalEventForScope(telemetryScope, "rag_predictive_attempt")
					predictiveStart := time.Now()
					hadPredictiveError := false
					for _, pred := range predictions {
						// Use SearchMemoriesOnly: predictive pre-fetch needs only user memories,
						// not tool_guides/documentation — avoids 2 full extra search cycles per request.
						pMem, pIDs, pErr := longTermMem.SearchMemoriesOnly(pred, 1)
						if pErr != nil {
							hadPredictiveError = true
						}
						if pErr == nil && len(pMem) > 0 {
							if len(pIDs) > 0 && usedMemoryDocIDs[pIDs[0]] > 0 {
								continue
							}
							if _, dup := retrievedSet[pMem[0]]; !dup {
								predictedResults = append(predictedResults, pMem[0])
								retrievedSet[pMem[0]] = struct{}{} // prevent intra-prediction duplicates
								if len(pIDs) > 0 && pIDs[0] != "" {
									usedMemoryDocIDs[pIDs[0]]++
									_ = shortTermMem.RecordMemoryUsage(pIDs[0], "ltm_predicted", sessionID, 0, false)
									turnMemoryCandidates[pIDs[0]] = compactMemoryForPrompt(pMem[0], 260)
								}
							}
						}
					}
					RecordRetrievalEventForScope(telemetryScope, "rag_predictive_latency:"+retrievalLatencyBucket(time.Since(predictiveStart)))
					if hadPredictiveError {
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_error")
					}
					if len(predictedResults) > 0 {
						flags.PredictedMemories = strings.Join(predictedResults, "\n---\n")
						retrievalPromptTokens += prompts.CountTokens(flags.PredictedMemories)
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_hit")
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_source:ltm_predicted")
						currentLogger.Debug("[Sync] Predictive RAG: Pre-fetched memories", "count", len(predictedResults), "predictions", predictions, "temporal_predictions", temporalPredictions)
					} else {
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_miss")
					}
				}
			}

		}

		// For capability/availability queries, RAG was intentionally skipped.
		// Inject a live-state policy note so the agent knows not to rely on any
		// stale memory it may have encountered in the conversation history.
		if !runCfg.IsMission && lastUserMsg != "" && isCapabilityQuery(lastUserMsg) && flags.RetrievedMemories == "" {
			flags.RetrievedMemories = "[Memory Policy] This query concerns agent capabilities or tool/integration availability. " +
				"The authoritative source is the CURRENT TOOL SCHEMA in this context — NOT past memory entries. " +
				"Memory about tool availability is always considered potentially stale. " +
				"If you are unsure whether a tool is present, inspect the tool list directly or attempt to use the tool."
			currentLogger.Debug("[RAG] Capability query: injecting live-state policy hint")
		}

		// Inject lightweight recent-day anchors and episodic cards, even when
		// long-term memory retrieval is unavailable/disabled.
		if !runCfg.IsMission && lastUserMsg != "" && shortTermMem != nil {
			pendingActions, pErr := shortTermMem.GetPendingEpisodicActionsForQuery(lastUserMsg, 2)
			if pErr == nil && len(pendingActions) > 0 {
				turnPendingActions = append(turnPendingActions, pendingActions...)
				lines := make([]string, 0, len(pendingActions))
				for _, action := range pendingActions {
					line := action.EventDate + " | " + action.Title + " — " + action.Summary
					if trigger := strings.TrimSpace(action.TriggerQuery); trigger != "" {
						line += " | trigger: " + trigger
					}
					lines = append(lines, line)
				}
				prefix := "[Pending Follow-Ups]\n- " + strings.Join(lines, "\n- ")
				if flags.RetrievedMemories == "" {
					flags.RetrievedMemories = prefix
				} else {
					flags.RetrievedMemories += "\n---\n" + prefix
				}
			}
			anchors, aErr := shortTermMem.GetRecentDayAnchors(2)
			if aErr == nil && len(anchors) > 0 {
				prefix := "[Recent Day Anchors]\n- " + strings.Join(anchors, "\n- ")
				if flags.RetrievedMemories == "" {
					flags.RetrievedMemories = prefix
				} else {
					flags.RetrievedMemories += "\n---\n" + prefix
				}
			}
			episodic, eErr := shortTermMem.GetRecentEpisodicMemories(72, 2)
			if eErr == nil && len(episodic) > 0 {
				prefix := "[Last 72h Episodes]\n- " + strings.Join(episodic, "\n- ")
				if flags.RetrievedMemories == "" {
					flags.RetrievedMemories = prefix
				} else {
					flags.RetrievedMemories += "\n---\n" + prefix
				}
			}
		}

		if !runCfg.IsMission && shortTermMem != nil {
			if overview, err := shortTermMem.BuildRecentActivityPromptOverview(7); err == nil {
				flags.RecentActivityOverview = overview
			}
		}

		// Knowledge Graph context injection: search for relevant entities
		if !runCfg.IsMission && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.PromptInjection && kg != nil && lastUserMsg != "" {
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
		if !runCfg.IsMission && personalityEnabled && shortTermMem != nil {
			if cfg.Personality.EngineV2 {
				// V2 Feature: Narrative Events based on Milestones & Loneliness
				processBehavioralEvents(shortTermMem, &req.Messages, sessionID, meta, currentLogger)
			}
			flags.PersonalityLine = shortTermMem.GetPersonalityLineWithMeta(cfg.Personality.EngineV2, meta)

			// Emotion Synthesizer: inject latest emotional description
			if emotionDescription := latestEmotionDescription(shortTermMem, emotionSynthesizer); emotionDescription != "" {
				flags.EmotionDescription = emotionDescription
			}
		}

		// User Profiling: inject behavioral instruction + collected profile data
		if !runCfg.IsMission && cfg.Personality.UserProfiling {
			flags.UserProfilingEnabled = true
			if cfg.Personality.EngineV2 && shortTermMem != nil {
				flags.UserProfileSummary = shortTermMem.GetUserProfileSummary(cfg.Personality.UserProfilingThreshold)
				logger.Debug("User profiling enabled", "summaryLength", len(flags.UserProfileSummary), "threshold", cfg.Personality.UserProfilingThreshold)
			} else {
				logger.Debug("User profiling enabled (without profile summary - V2 engine disabled or no memory)")
			}
		}

		// Adaptive tier: adjust prompt complexity based on conversation length and context signals
		flags.MessageCount = len(req.Messages)
		flags.RecentlyUsedTools = recentTools
		flags.Tier = prompts.DetermineTierAdaptive(flags)
		if runCfg.IsMission {
			flags.IsMission = true
			flags.Tier = "minimal"
		} else {
			adjustedTier := applyTelemetryAwarePromptTier(toolingPolicy, flags, flags.Tier)
			if adjustedTier != flags.Tier {
				RecordToolPolicyEventForScope(telemetryScope, "prompt_tier_compact")
				currentLogger.Info("[ToolingPolicy] Telemetry-aware prompt tier adjustment",
					"provider", telemetryScope.ProviderType,
					"model", telemetryScope.Model,
					"from", flags.Tier,
					"to", adjustedTier,
					"message_count", flags.MessageCount,
					"failure_rate", toolingPolicy.TelemetrySnapshot.FailureRate)
				flags.Tier = adjustedTier
			}
		}
		flags.IsDebugMode = cfg.Agent.DebugMode || GetDebugMode() // re-check each iteration (toggleable at runtime)
		flags.IsVoiceMode = GetVoiceMode()                        // re-check each iteration (toggleable at runtime)

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
		if shortTermMem != nil {
			if planPrompt, err := shortTermMem.BuildSessionPlanPrompt(sessionID); err == nil && strings.TrimSpace(planPrompt) != "" {
				flags.SessionTodoItems = planPrompt
			}
		}
		flags.AdditionalPrompt = mergeAdditionalPrompt(baseAdditionalPrompt, emotionPolicy.PromptHint)
		flags.TokenBudget = calculateEffectivePromptTokenBudget(cfg, ToolCall{}, homepageUsedInChain, currentLogger)
		recordRetrievalPromptTelemetry(telemetryScope, retrievalPromptTokens, flags.TokenBudget)

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
		compressionClient, compressionModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
		if compressionClient != nil && compressionModel != "" {
			req.Messages, lastCompressionMsg, _ = CompressHistory(
				ctx, req.Messages, maxHistoryTokens, compressionModel, compressionClient, lastCompressionMsg, currentLogger,
			)
		}

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
			var droppedMessages []openai.ChatCompletionMessage
			// Drop messages from index 1 onward (oldest first) until we fit.
			// Always keep system (0) and the latest message.
			mid := req.Messages[1 : len(req.Messages)-1]
			for totalMsgTokens > maxHistoryTokens && len(mid) > 0 {
				dropped := mid[0]
				droppedMessages = append(droppedMessages, dropped)
				mid = mid[1:]
				totalMsgTokens -= prompts.CountTokens(dropped.Content) + 4
			}
			trimmedMessages := []openai.ChatCompletionMessage{sysMsg}
			remainingRecapBudget := maxHistoryTokens - totalMsgTokens - 4
			if recap := buildTrimmedContextRecap(droppedMessages, remainingRecapBudget); recap != "" {
				trimmedMessages = append(trimmedMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: recap,
				})
				totalMsgTokens += prompts.CountTokens(recap) + 4
			}
			req.Messages = append(trimmedMessages, append(mid, lastMsg)...)
			currentLogger.Info("[ContextGuard] History trimmed",
				"remaining_messages", len(req.Messages), "estimated_tokens", totalMsgTokens, "dropped_messages", len(droppedMessages))
		}

		// Verbose Logging of LLM Request
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			// Keep conversation logs in the original logger (stdout) to avoid pollution of technical log
			logger.Info("[LLM Request]", "role", lastMsg.Role, "content_len", len(lastMsg.Content), "preview", Truncate(lastMsg.Content, 200))
			currentLogger.Info("[LLM Request Redirected]", "role", lastMsg.Role, "content_len", len(lastMsg.Content))
			currentLogger.Debug("[LLM Full History]", "messages_count", len(req.Messages))
		}

		// Prompt log: append full request JSON to prompts.log when enabled
		if cfg.Logging.EnablePromptLog && cfg.Logging.LogDir != "" {
			if f, ferr := os.OpenFile(
				filepath.Join(cfg.Logging.LogDir, "prompts.log"),
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644,
			); ferr == nil {
				type promptLogEntry struct {
					Time       string                         `json:"time"`
					Model      string                         `json:"model"`
					ToolsCount int                            `json:"tools_count"`
					Messages   []openai.ChatCompletionMessage `json:"messages"`
				}
				entry := promptLogEntry{
					Time:       time.Now().UTC().Format(time.RFC3339),
					Model:      req.Model,
					ToolsCount: len(req.Tools),
					Messages:   req.Messages,
				}
				_ = json.NewEncoder(f).Encode(entry)
				_ = f.Close()
			}
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

		telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)

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
				telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
				if recovered, recErr := recoverFrom422WithPolicy(recoveryPolicy, streamErr, &retry422Count, &req, currentLogger, broker, "Stream", telemetryScope); recovered {
					continue
				} else if recErr != nil {
					return openai.ChatCompletionResponse{}, recErr
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
						// Proxy content to SSE client for real-time display.
						// Skip content that looks like a JSON tool call payload —
						// it will be parsed and dispatched as a tool call below.
						// Streaming raw JSON to the UI causes it to appear as text
						// in the chat before the tool can execute (especially with
						// providers that don't use the tool_calls field properly).
						//
						// Suppress JSON tool-call chunks so they never render as chat text.
						// {"tool_call":...} / {"tool_name":...} are always suppressed (MiniMax format).
						// Broader JSON heuristics (action/command/...) only suppressed when MiniMaxFix=true.
						trimmed := strings.TrimLeft(delta.Content, " \t\r\n")
						isToolCallJSON := len(trimmed) > 0 && trimmed[0] == '{' &&
							(strings.Contains(trimmed, `"tool_call"`) || strings.Contains(trimmed, `"tool_name"`))
						isLikelyToolCallJSON := len(trimmed) > 0 && trimmed[0] == '{' &&
							(strings.Contains(trimmed, `"action"`) || strings.Contains(trimmed, `"command"`) ||
								strings.Contains(trimmed, `"operation"`) || strings.Contains(trimmed, `"tool_call"`) ||
								strings.Contains(trimmed, `"tool"`) || strings.Contains(trimmed, `"name"`) ||
								strings.Contains(trimmed, `"arguments"`))
						suppressForMiniMax := isToolCallJSON || (cfg.LLM.MiniMaxFix && isLikelyToolCallJSON)
						if !suppressForMiniMax {
							if chunkData, mErr := json.Marshal(chunk); mErr == nil {
								broker.SendJSON(fmt.Sprintf("data: %s\n\n", string(chunkData)))
							}
						}
					}
					// Accumulate streamed tool call fragments
					for _, tc := range delta.ToolCalls {
						mergeStreamToolCallChunk(streamToolCalls, tc)
					}
				}
			}
			stm.Close()
			content = assembledResponse.String()

			// Build sorted slice of assembled tool calls
			assembledToolCalls := assembleSortedStreamToolCalls(streamToolCalls)
			if len(assembledToolCalls) > 0 {
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
				telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)
				if recovered, recErr := recoverFrom422WithPolicy(recoveryPolicy, err, &retry422Count, &req, currentLogger, broker, "Sync", telemetryScope); recovered {
					continue
				} else if recErr != nil {
					return openai.ChatCompletionResponse{}, recErr
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
		telemetryScope = refreshTelemetryScope(telemetryScope, client, &resp)

		retry422Count = 0 // reset on successful LLM response

		if recoverFromEmptyResponseWithPolicy(recoveryPolicy, resp, content, &req, &emptyRetried, currentLogger, broker, telemetryScope) {
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

		parsedToolResp := parseToolResponse(resp, currentLogger, telemetryScope)
		tc := parsedToolResp.ToolCall
		useNativePath := parsedToolResp.UseNativePath
		nativeAssistantMsg := parsedToolResp.NativeAssistantMsg
		if parsedToolResp.ParseSource == ToolCallParseSourceNative {
			nativeCall := resp.Choices[0].Message.ToolCalls[0]
			currentLogger.Info("[Sync] Native tool call detected", "function", tc.Action, "id", nativeCall.ID, "forced", !cfg.LLM.UseNativeFunctions)
		} else if parsedToolResp.ParseSource == ToolCallParseSourceReasoningCleanJSON {
			currentLogger.Info("[Sync] Tool call detected after stripping reasoning tags", "function", tc.Action)
		}
		if len(parsedToolResp.PendingToolCalls) > 0 {
			pendingTCs = append(pendingTCs, parsedToolResp.PendingToolCalls...)
			currentLogger.Info("[MultiTool] Queued additional tool calls from response", "count", len(parsedToolResp.PendingToolCalls), "source", parsedToolResp.ParseSource)
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
			budgetCategory := "chat"
			if strings.HasPrefix(sessionID, "coagent-") || strings.HasPrefix(sessionID, "specialist-") {
				budgetCategory = "coagent"
			}
			crossedWarning := budgetTracker.RecordForCategory(budgetCategory, actualModel, promptTokens, completionTokens)
			budgetJSON := budgetTracker.GetStatusJSON()
			if budgetJSON != "" {
				broker.SendJSON(budgetJSON)
			}
			if crossedWarning {
				bs := budgetTracker.GetStatus()
				warnMsg := fmt.Sprintf("\u26a0\ufe0f Budget warning: %.0f%% used ($%.4f / $%.2f)", bs.Percentage*100, bs.SpentUSD, bs.DailyLimit)
				broker.Send("budget_warning", warnMsg)
				// Journal: record budget warning event once per session
				if shortTermMem != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries && sessionID == "default" {
					_, _ = shortTermMem.InsertJournalEntry(memory.JournalEntry{
						EntryType:     "budget_warning",
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
				// Journal: record budget exceeded event once per session
				if shortTermMem != nil && cfg.Tools.Journal.Enabled && cfg.Journal.AutoEntries && sessionID == "default" {
					bs2 := budgetTracker.GetStatus()
					_, _ = shortTermMem.InsertJournalEntry(memory.JournalEntry{
						EntryType:     "budget_exceeded",
						Title:         fmt.Sprintf("Budget exceeded: %.0f%% used", bs2.Percentage*100),
						Content:       fmt.Sprintf("$%.4f of $%.2f daily budget consumed (enforcement: %s)", bs2.SpentUSD, bs2.DailyLimit, bs2.Enforcement),
						Importance:    5,
						SessionID:     sessionID,
						AutoGenerated: true,
					})
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
			feedbackMsg = applyEmotionRecoveryNudge(feedbackMsg, emotionPolicy)
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, true)
			}

			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			continue
		}

		if useNativePath && tc.NativeArgsMalformed && invalidNativeToolCount < 2 {
			invalidNativeToolCount++
			currentLogger.Warn("[Sync] Invalid native tool call detected, requesting corrected function call",
				"attempt", invalidNativeToolCount,
				"action", tc.Action,
				"error", tc.NativeArgsError)
			broker.Send("error_recovery", "Invalid native tool call detected, requesting corrected function call...")

			recoveryTool := tc.Action
			if strings.TrimSpace(recoveryTool) == "" {
				recoveryTool = "the requested tool"
			} else {
				recoveryTool = Truncate(strings.ReplaceAll(strings.ReplaceAll(recoveryTool, "\n", " "), "\r", " "), 80)
			}
			feedbackMsg := fmt.Sprintf(
				"ERROR: Your last native function call for %q had invalid function arguments JSON and was discarded. Emit the function call again with valid JSON arguments only. Do not include source code, XML/HTML, or prose inside the function name or outside the JSON arguments.",
				recoveryTool,
			)
			feedbackMsg = applyEmotionRecoveryNudge(feedbackMsg, emotionPolicy)
			id, err := shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist invalid-native-tool feedback message", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, true)
			}
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: feedbackMsg})
			lastResponseWasTool = false
			continue
		}

		// Recovery: model announced a next action but did not emit the tool call yet.
		// Use sanitized content (think-tags stripped) to avoid false positives from
		// reasoning-only language inside <think> blocks triggering the forward-cue detector.
		announcementContent := parsedToolResp.SanitizedContent
		if announcementContent == "" {
			announcementContent = content
		}
		isAnnouncement := isAnnouncementOnlyResponse(announcementContent, tc, useNativePath, lastResponseWasTool, lastUserMsg)
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

			var feedbackMsg string
			if useNativeFunctions {
				feedbackMsg = "ERROR: You announced what you were going to do but did not call a tool. You MUST use the native function-calling mechanism to invoke a tool — do not include any text before or after the function call."
			} else {
				feedbackMsg = "ERROR: You announced what you were going to do but did not output a tool call. When executing a task, your ENTIRE response must be ONLY the raw JSON tool call — no explanation before it. Output the JSON tool call NOW."
			}
			feedbackMsg = applyEmotionRecoveryNudge(feedbackMsg, emotionPolicy)
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, true)
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
			feedbackMsg = applyEmotionRecoveryNudge(feedbackMsg, emotionPolicy)
			id, err = shortTermMem.InsertMessage(sessionID, openai.ChatMessageRoleUser, feedbackMsg, false, true)
			if err != nil {
				currentLogger.Error("Failed to persist feedback message to SQLite", "error", err)
			}
			if sessionID == "default" {
				historyManager.Add(openai.ChatMessageRoleUser, feedbackMsg, id, false, true)
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

			// Strip <think> reasoning blocks — never store/display them in history.
			histContent = security.StripThinkingTags(histContent)

			// When the LLM mixes conversational text with a trailing JSON tool call
			// (e.g. "Build erfolgreich!\n\n{"tool_call":"deploy",...}"), keep only the
			// text portion so the raw JSON never appears as a chat message.
			if !useNativePath {
				if jsonIdx := strings.Index(histContent, "{"); jsonIdx > 0 {
					textPart := strings.TrimSpace(histContent[:jsonIdx])
					if textPart != "" {
						histContent = textPart
					}
				}
			}

			// Tool-call turn messages are operational scaffolding, not user-facing chat history.
			// They are shown live via SSE/debug UI and should not reappear as normal chat bubbles
			// after a reload, even when the model added prose before the tool call.
			isMsgInternal := true

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

			if recoveryState.handleDuplicateToolCall(tc, &req, currentLogger, telemetryScope) {
				lastResponseWasTool = false
				continue
			}

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

			dispatchCtx := makeDispatchContext(currentLogger)
			resultContent := DispatchToolCall(ctx, tc, dispatchCtx, lastUserMsg)
			policyResult := finalizeToolExecution(tc, resultContent, cfg, shortTermMem, sessionID, &recoveryState, &req, currentLogger, telemetryScope)
			resultContent = policyResult.Content
			trackActivityTool(&turnToolNames, &turnToolSummaries, tc.Action, resultContent)
			recordPlanToolProgress(shortTermMem, sessionID, tc, resultContent, currentLogger)

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
			if tc.Action == "tts" {
				var ttsRes struct {
					Status string `json:"status"`
					File   string `json:"file"`
				}
				raw := strings.TrimPrefix(resultContent, "[Tool Output]\n")
				raw = strings.TrimPrefix(raw, "Tool Output: ")
				if json.Unmarshal([]byte(raw), &ttsRes) == nil && ttsRes.Status == "success" {
					mimeType := "audio/mpeg"
					if strings.HasSuffix(ttsRes.File, ".wav") {
						mimeType = "audio/wav"
					}
					evtPayload, _ := json.Marshal(map[string]string{
						"path":      "/tts/" + ttsRes.File,
						"title":     "TTS Audio",
						"mime_type": mimeType,
						"filename":  ttsRes.File,
						"file_path": filepath.Join(cfg.Directories.DataDir, "tts", ttsRes.File),
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
				sessionTodoList = string(tc.Todo)
				broker.Send("todo_update", sessionTodoList)
			}
			if tc.Action == "manage_plan" {
				emitSessionPlanUpdate(broker, shortTermMem, sessionID, currentLogger)
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

				if cfg.Personality.EngineV2 {
					// ── V2: Asynchronous LLM-Based Mood Analysis ──
					recentMsgs := req.Messages
					toolEmotionTrigger, toolEmotionDetail := detectToolEmotionTrigger(tc, recoveryState.ConsecutiveErrorCount, toolCallCount-recoveryState.ConsecutiveErrorCount)
					launchAsyncPersonalityV2Analysis(
						cfg,
						currentLogger,
						client,
						shortTermMem,
						emotionSynthesizer,
						recentMsgs,
						triggerInfo,
						toolEmotionTrigger,
						toolEmotionDetail,
						0,
						"Tool Result",
						resultContent,
						meta,
						cfg.Personality.UserProfiling,
						recoveryState.ConsecutiveErrorCount,
						toolCallCount-recoveryState.ConsecutiveErrorCount,
					)

				} else {
					// ── V1: Synchronous Heuristic-Based Mood Analysis ──
					mood, traitDeltas := memory.DetectMood(lastUserMsg, resultContent, meta)
					// O-08: Apply emotion bias from synthesizer to contextualize V1 detection.
					if emotionSynthesizer != nil {
						traits, _ := shortTermMem.GetTraits()
						mood = memory.ApplyEmotionBias(mood, emotionSynthesizer.GetLastEmotion(), traits)
					}
					_ = shortTermMem.LogMood(mood, triggerInfo)
					for trait, delta := range traitDeltas {
						_ = shortTermMem.UpdateTrait(trait, delta)
					}
				}
				flags.PersonalityLine = shortTermMem.GetPersonalityLineWithMeta(cfg.Personality.EngineV2, meta)

				// Emotion Synthesizer: update flags with latest emotion if available
				if emotionDescription := latestEmotionDescription(shortTermMem, emotionSynthesizer); emotionDescription != "" {
					flags.EmotionDescription = emotionDescription
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
				var nativePendingSummaryBatch map[string]string
				nativeDispatchCtx := makeDispatchContext(currentLogger)
				for len(pendingTCs) > 0 && pendingTCs[0].NativeCallID != "" {
					if helperManager != nil && len(nativePendingSummaryBatch) == 0 {
						nativePendingSummaryBatch = maybeBuildPendingSummaryBatch(ctx, pendingTCs, nativeDispatchCtx, helperManager, lastUserMsg)
					}

					btc := pendingTCs[0]
					pendingTCs = pendingTCs[1:]
					toolCallCount++
					if btc.Action == "homepage" || btc.Action == "homepage_tool" {
						homepageUsedInChain = true
					}
					broker.Send("thinking", fmt.Sprintf("[%d] Running %s (batched)...", toolCallCount, btc.Action))
					broker.Send("tool_start", btc.Action)

					bResult := ""
					if precomputed, ok := nativePendingSummaryBatch[pendingSummaryBatchKey(btc)]; ok {
						bResult = precomputed
						delete(nativePendingSummaryBatch, pendingSummaryBatchKey(btc))
						if len(nativePendingSummaryBatch) == 0 {
							nativePendingSummaryBatch = nil
						}
					} else if recoveryState.handleDuplicateToolCall(btc, &req, currentLogger, telemetryScope) {
						bResult = blockedToolOutputFromRequest(&req)
					} else {
						bResult = DispatchToolCall(ctx, btc, nativeDispatchCtx, lastUserMsg)
					}
					policyResult := finalizeToolExecution(btc, bResult, cfg, shortTermMem, sessionID, &recoveryState, &req, currentLogger, telemetryScope)
					bResult = policyResult.Content
					trackActivityTool(&turnToolNames, &turnToolSummaries, btc.Action, bResult)
					recordPlanToolProgress(shortTermMem, sessionID, btc, bResult, currentLogger)
					broker.Send("tool_output", bResult)
					broker.Send("tool_end", btc.Action)
					if btc.Action == "manage_plan" {
						emitSessionPlanUpdate(broker, shortTermMem, sessionID, currentLogger)
					}
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

		memAnalysis := resolveMemoryAnalysisSettings(cfg, shortTermMem)
		useBatchedTurnHelper := helperManager != nil && memAnalysis.Enabled && memAnalysis.RealTime && !isEmpty && shortTermMem != nil && !flags.IsCoAgent
		useBatchedTurnPersonality := useBatchedTurnHelper && personalityEnabled && cfg.Personality.EngineV2

		// Phase D: Final mood + trait update + milestone check at session end
		if personalityEnabled && shortTermMem != nil {
			if cfg.Personality.EngineV2 {
				if !useBatchedTurnPersonality {
					launchAsyncPersonalityV2Analysis(
						cfg,
						currentLogger,
						client,
						shortTermMem,
						emotionSynthesizer,
						req.Messages,
						moodTrigger(),
						userEmotionTrigger,
						userEmotionTriggerDetail,
						userInactivityHours,
						"Assistant Response",
						content,
						meta,
						cfg.Personality.UserProfiling,
						recoveryState.ConsecutiveErrorCount,
						toolCallCount-recoveryState.ConsecutiveErrorCount,
					)
				}
			} else {
				mood, traitDeltas := memory.DetectMood(lastUserMsg, "", meta)
				// O-08: Apply emotion bias from synthesizer to contextualize V1 detection.
				if emotionSynthesizer != nil {
					traits, _ := shortTermMem.GetTraits()
					mood = memory.ApplyEmotionBias(mood, emotionSynthesizer.GetLastEmotion(), traits)
				}
				_ = shortTermMem.LogMood(mood, moodTrigger())
				for trait, delta := range traitDeltas {
					_ = shortTermMem.UpdateTrait(trait, delta)
				}
			}
		}

		if memAnalysis.EffectivenessTracking && !isEmpty && shortTermMem != nil && len(turnMemoryCandidates) > 0 {
			usefulIDs, uselessIDs := assessMemoryEffectiveness(content, turnMemoryCandidates)
			for _, memoryID := range usefulIDs {
				if err := shortTermMem.RecordMemoryEffectiveness(memoryID, true); err != nil {
					currentLogger.Debug("Failed to record useful memory effectiveness", "memory_id", memoryID, "error", err)
					continue
				}
				RecordRetrievalEventForScope(telemetryScope, "memory_effectiveness_useful")
			}
			for _, memoryID := range uselessIDs {
				if err := shortTermMem.RecordMemoryEffectiveness(memoryID, false); err != nil {
					currentLogger.Debug("Failed to record useless memory effectiveness", "memory_id", memoryID, "error", err)
					continue
				}
				RecordRetrievalEventForScope(telemetryScope, "memory_effectiveness_useless")
			}
		}
		if !isEmpty && shortTermMem != nil && len(turnPendingActions) > 0 {
			resolveCompletedPendingActions(shortTermMem, lastUserMsg, content, turnPendingActions)
		}

		if useBatchedTurnHelper {
			activityToolNames := append([]string(nil), turnToolNames...)
			activityToolSummaries := append([]string(nil), turnToolSummaries...)
			var turnPersonalityInput *helperTurnPersonalityInput
			if useBatchedTurnPersonality {
				contextHistory, userHistory := buildPersonalityHistories(req.Messages, "Assistant Response", content)
				_, previousEmotion := resolveHelperEmotionBatchState(cfg, emotionSynthesizer)
				traits, _ := shortTermMem.GetTraits()
				turnPersonalityInput = &helperTurnPersonalityInput{
					RecentHistory:   contextHistory,
					UserOnlyHistory: userHistory,
					Language:        cfg.Agent.SystemLanguage,
					Traits:          traits,
					PreviousEmotion: previousEmotion,
					TriggerInfo:     moodTrigger(),
					TriggerType:     userEmotionTrigger,
					TriggerDetail:   userEmotionTriggerDetail,
					InactivityHours: userInactivityHours,
					ErrorCount:      recoveryState.ConsecutiveErrorCount,
					SuccessCount:    toolCallCount - recoveryState.ConsecutiveErrorCount,
				}
			}
			go func(userMsg, aResp, sid string, toolNames, toolSummaries []string, personalityInput *helperTurnPersonalityInput, recentMsgs []openai.ChatCompletionMessage) {
				analysisCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				batchResult, err := helperManager.AnalyzeTurn(analysisCtx, userMsg, aResp, toolNames, toolSummaries, personalityInput)
				if err != nil {
					helperManager.ObserveFallback("analyze_turn", err.Error())
					currentLogger.Warn("[HelperLLM] Batched turn analysis failed, falling back", "error", err)
					if useBatchedTurnPersonality {
						launchAsyncPersonalityV2Analysis(
							cfg,
							currentLogger,
							client,
							shortTermMem,
							emotionSynthesizer,
							recentMsgs,
							moodTrigger(),
							userEmotionTrigger,
							userEmotionTriggerDetail,
							userInactivityHours,
							"Assistant Response",
							aResp,
							meta,
							cfg.Personality.UserProfiling,
							recoveryState.ConsecutiveErrorCount,
							toolCallCount-recoveryState.ConsecutiveErrorCount,
						)
					}
					runMemoryAnalysis(analysisCtx, cfg, currentLogger, shortTermMem, kg, longTermMem, userMsg, aResp, sid)
					captureActivityTurn(
						cfg,
						currentLogger,
						shortTermMem,
						kg,
						sid,
						runCfg.MessageSource,
						userMsg,
						aResp,
						toolNames,
						toolSummaries,
						runCfg.IsMission || runCfg.MessageSource == "mission" || sid == "maintenance",
						!isMaintenance && sid != "maintenance",
					)
					return
				}

				applyMemoryAnalysisResult(cfg, currentLogger, shortTermMem, longTermMem, sid, batchResult.MemoryAnalysis)
				if useBatchedTurnPersonality {
					if personalityResult, ok := normalizeHelperTurnPersonalityResult(batchResult.PersonalityAnalysis, meta); ok {
						_, previousEmotion := resolveHelperEmotionBatchState(cfg, emotionSynthesizer)
						v2FailCount.Store(0)
						applyPersonalityV2AnalysisResult(
							cfg,
							currentLogger,
							shortTermMem,
							emotionSynthesizer,
							previousEmotion,
							moodTrigger(),
							userEmotionTrigger,
							userEmotionTriggerDetail,
							userInactivityHours,
							cfg.Personality.UserProfiling,
							recoveryState.ConsecutiveErrorCount,
							toolCallCount-recoveryState.ConsecutiveErrorCount,
							personalityResult,
						)
					} else {
						helperManager.ObserveFallback("analyze_turn_personality", "personality_payload_invalid")
						launchAsyncPersonalityV2Analysis(
							cfg,
							currentLogger,
							client,
							shortTermMem,
							emotionSynthesizer,
							recentMsgs,
							moodTrigger(),
							userEmotionTrigger,
							userEmotionTriggerDetail,
							userInactivityHours,
							"Assistant Response",
							aResp,
							meta,
							cfg.Personality.UserProfiling,
							recoveryState.ConsecutiveErrorCount,
							toolCallCount-recoveryState.ConsecutiveErrorCount,
						)
					}
				}
				captureActivityTurnWithDigest(
					shortTermMem,
					kg,
					sid,
					runCfg.MessageSource,
					userMsg,
					toolNames,
					runCfg.IsMission || runCfg.MessageSource == "mission" || sid == "maintenance",
					!isMaintenance && sid != "maintenance",
					batchResult.ActivityDigest,
					"runtime_helper_batch",
				)
			}(lastUserMsg, content, sessionID, activityToolNames, activityToolSummaries, turnPersonalityInput, append([]openai.ChatCompletionMessage(nil), req.Messages...))
		} else {
			// Real-time memory analysis: async post-response extraction of memory-worthy content
			if memAnalysis.Enabled && memAnalysis.RealTime && !isEmpty && shortTermMem != nil {
				go func(userMsg, aResp, sid string) {
					analysisCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()
					runMemoryAnalysis(analysisCtx, cfg, currentLogger, shortTermMem, kg, longTermMem, userMsg, aResp, sid)
				}(lastUserMsg, content, sessionID)
			}

			if !isEmpty && shortTermMem != nil && !flags.IsCoAgent {
				activityToolNames := append([]string(nil), turnToolNames...)
				activityToolSummaries := append([]string(nil), turnToolSummaries...)
				go captureActivityTurn(
					cfg,
					currentLogger,
					shortTermMem,
					kg,
					sessionID,
					runCfg.MessageSource,
					lastUserMsg,
					content,
					activityToolNames,
					activityToolSummaries,
					runCfg.IsMission || runCfg.MessageSource == "mission" || sessionID == "maintenance",
					!isMaintenance && sessionID != "maintenance",
				)
			}
		}

		// Journal auto-trigger: create entries for significant tool chains
		JournalAutoTrigger(cfg, shortTermMem, currentLogger, sessionID, recentTools, lastUserMsg)

		// Weekly reflection: async trigger if configured and due
		// Guard: only run once per day by checking if a reflection entry already exists today.
		if memAnalysis.Enabled && memAnalysis.WeeklyReflection && weeklyReflectionDue(cfg, shortTermMem) && shortTermMem != nil {
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

func compactMemoryForPrompt(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	return strings.TrimSpace(text[:maxLen]) + "…"
}

func wantsDetailedMemory(query string) bool {
	q := strings.ToLower(query)
	cues := []string{
		"detail", "details", "exact", "specific", "precise",
		"genau", "details", "welche", "wann", "konkret",
	}
	for _, cue := range cues {
		if strings.Contains(q, cue) {
			return true
		}
	}
	return false
}

// trim422Messages produces a structurally valid message sequence after a 422 rejection.
// It strips all tool-call / tool-response pairs, removes orphaned assistant ToolCalls,
// keeps the system prompt + the most recent user/assistant exchanges, and appends
// a recovery user message so the model can reply cleanly.
func trim422Messages(msgs []openai.ChatCompletionMessage) []openai.ChatCompletionMessage {
	// Collect IDs of tool responses so we can match assistant->tool pairs.
	toolResponseIDs := make(map[string]bool, len(msgs))
	for _, m := range msgs {
		if m.Role == openai.ChatMessageRoleTool && m.ToolCallID != "" {
			toolResponseIDs[m.ToolCallID] = true
		}
	}

	// Pass 1: keep only system/user/assistant messages. For assistant messages
	// with ToolCalls, only keep them if ALL their tool call IDs have matching
	// tool responses AND we also keep those responses. Otherwise drop the
	// entire tool-call round (assistant + tool responses) to avoid dangling refs.
	var clean []openai.ChatCompletionMessage
	for _, m := range msgs {
		switch m.Role {
		case openai.ChatMessageRoleTool:
			continue // always drop — we'll re-add complete pairs below if needed
		case openai.ChatMessageRoleAssistant:
			if len(m.ToolCalls) > 0 {
				// Drop assistant messages that requested tool calls — their tool
				// responses have been dropped, leaving an orphaned tool_call.
				continue
			}
			// Plain assistant text — keep it.
			clean = append(clean, m)
		default:
			// system, user — keep as-is
			clean = append(clean, m)
		}
	}

	// Pass 2: keep system prompt (first system messages) + last few user/assistant messages.
	// Find the boundary between leading system messages and the conversation.
	sysEnd := 0
	for sysEnd < len(clean) && clean[sysEnd].Role == openai.ChatMessageRoleSystem {
		sysEnd++
	}
	conversation := clean[sysEnd:]
	if len(conversation) > 4 {
		conversation = conversation[len(conversation)-4:]
	}
	// Ensure the conversation starts with a user message (providers reject leading assistant).
	for len(conversation) > 0 && conversation[0].Role != openai.ChatMessageRoleUser {
		conversation = conversation[1:]
	}

	trimmed := make([]openai.ChatCompletionMessage, 0, sysEnd+len(conversation)+1)
	trimmed = append(trimmed, clean[:sysEnd]...)
	trimmed = append(trimmed, conversation...)
	// Append a user message (not system) so the model can respond naturally.
	trimmed = append(trimmed, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "[The previous tool call history was trimmed due to a provider error. Please summarise what you were doing and continue.]",
	})
	return trimmed
}
