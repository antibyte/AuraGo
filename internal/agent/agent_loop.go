package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/llm"
	loggerPkg "aurago/internal/logger"
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"
)

// Memory and telemetry helpers moved to agent_memory_helpers.go

const maxConcurrentAgentLoops = 8

var agentLoopLimiter = make(chan struct{}, maxConcurrentAgentLoops)

func acquireAgentLoopSlot(ctx context.Context) (func(), error) {
	select {
	case agentLoopLimiter <- struct{}{}:
		released := false
		return func() {
			if released {
				return
			}
			released = true
			<-agentLoopLimiter
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func withAgentLoopSlot(ctx context.Context, fn func()) error {
	release, err := acquireAgentLoopSlot(ctx)
	if err != nil {
		return err
	}
	defer func() {
		release()
		if rec := recover(); rec != nil {
			panic(rec)
		}
	}()
	fn()
	return nil
}

// agentLoopState holds the mutable state for a single ExecuteAgentLoop invocation.
type agentLoopState struct {
	ctx    context.Context
	stream bool
	broker FeedbackBroker
	runCfg RunConfig
	req    openai.ChatCompletionRequest
	flags  prompts.ContextFlags

	initialUserMsg           string
	dailyTodoReminder        string
	operationalIssueReminder string
	plannerContext           string
	baseAdditionalPrompt     string
	toolGuidesDir            string
	toolingPolicy            ToolingPolicy
	telemetryScope           AgentTelemetryScope

	toolCallCount           int
	rawCodeCount            int
	xmlFallbackCount        int
	missedToolCount         int
	announcementCount       int
	incompleteToolCallCount int
	orphanedBracketTagCount int
	orphanedXMLTagCount     int
	invalidNativeToolCount  int
	sessionTokens           int
	retry422Count           int
	stepsSinceLastFeedback  int
	workflowPlanCount       int

	emptyRetried        bool
	homepageUsedInChain bool
	lastResponseWasTool bool
	coreMemDirty        bool
	personalityEnabled  bool

	isMaintenance                     bool
	currentLogger                     *slog.Logger
	lastTool                          string
	lastActivity                      time.Time
	lastUserMsg                       string
	ragLastUserMsg                    string
	ragToolIterationsSinceLastRefresh int
	sessionTodoList                   string

	sessionUsedTools    map[string]bool
	recentTools         []string
	explicitTools       []string
	pendingTCs          []ToolCall
	pendingSummaryBatch map[string]string
	usedMemoryDocIDs    map[string]int
	turnToolNames       []string
	turnToolSummaries   []string

	coreMemCache       string
	coreMemUpdatedAt   time.Time
	coreMemLoadedAt    time.Time
	tokenCache         *tokenCountCache
	detectedCtxWindow  int
	lastCompressionMsg int

	// Cached compression client/model resolved once per session instead of per loop iteration.
	cachedCompressionClient llm.ChatClient
	cachedCompressionModel  string

	cachedSysPromptKey    string
	cachedSysPrompt       string
	cachedSysPromptTokens int
	cachedSysPromptAt     time.Time

	emotionSynthesizer *memory.EmotionSynthesizer
	meta               memory.PersonalityMeta

	recoveryPolicy  RecoveryPolicy
	recoverySession *RecoverySessionState
	recoveryState   toolRecoveryState

	helperManager *helperLLMManager
	guardian      *security.Guardian
	llmGuardian   *security.LLMGuardian

	useNativeFunctions    bool
	adaptiveFilteredTools []string
}

// makeDispatchContext builds a DispatchContext from the current loop state.
func (s *agentLoopState) makeDispatchContext(currentLogger *slog.Logger) *DispatchContext {
	return &DispatchContext{
		Cfg:                 s.runCfg.Config,
		Logger:              s.currentLogger,
		LLMClient:           s.runCfg.LLMClient,
		Vault:               s.runCfg.Vault,
		Registry:            s.runCfg.Registry,
		Manifest:            s.runCfg.Manifest,
		CronManager:         s.runCfg.CronManager,
		MissionManagerV2:    s.runCfg.MissionManagerV2,
		LongTermMem:         s.runCfg.LongTermMem,
		ShortTermMem:        s.runCfg.ShortTermMem,
		KG:                  s.runCfg.KG,
		InventoryDB:         s.runCfg.InventoryDB,
		InvasionDB:          s.runCfg.InvasionDB,
		CheatsheetDB:        s.runCfg.CheatsheetDB,
		ImageGalleryDB:      s.runCfg.ImageGalleryDB,
		MediaRegistryDB:     s.runCfg.MediaRegistryDB,
		HomepageRegistryDB:  s.runCfg.HomepageRegistryDB,
		ContactsDB:          s.runCfg.ContactsDB,
		PlannerDB:           s.runCfg.PlannerDB,
		SQLConnectionsDB:    s.runCfg.SQLConnectionsDB,
		SQLConnectionPool:   s.runCfg.SQLConnectionPool,
		RemoteHub:           s.runCfg.RemoteHub,
		HistoryMgr:          s.runCfg.HistoryManager,
		IsMaintenance:       tools.IsBusy(),
		SurgeryPlan:         s.runCfg.SurgeryPlan,
		Guardian:            s.guardian,
		LLMGuardian:         s.llmGuardian,
		SessionID:           s.runCfg.SessionID,
		CoAgentRegistry:     s.runCfg.CoAgentRegistry,
		IsCoAgent:           s.runCfg.IsCoAgent,
		CoAgentSpecialist:   s.runCfg.CoAgentSpecialist,
		ParentSessionID:     s.runCfg.ParentSessionID,
		ParentIsMaintenance: s.runCfg.IsMaintenance,
		BudgetTracker:       s.runCfg.BudgetTracker,
		DaemonSupervisor:    s.runCfg.DaemonSupervisor,
		PreparationService:  s.runCfg.PreparationService,
		MessageSource:       s.runCfg.MessageSource,
		Broker:              s.broker,
	}
}

// ExecuteAgentLoop executes the multi-turn reasoning and tool execution loop.
// It supports both synchronous returns and asynchronous streaming via the broker.
func ExecuteAgentLoop(ctx context.Context, req openai.ChatCompletionRequest, runCfg RunConfig, stream bool, broker FeedbackBroker) (openai.ChatCompletionResponse, error) {
	releaseAgentLoopSlot, err := acquireAgentLoopSlot(ctx)
	if err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	defer func() {
		releaseAgentLoopSlot()
		if rec := recover(); rec != nil {
			panic(rec)
		}
	}()

	s := initAgentLoopState(req, runCfg, broker)
	req = s.req

	cfg := s.runCfg.Config
	logger := s.runCfg.Logger
	client := s.runCfg.LLMClient
	shortTermMem := s.runCfg.ShortTermMem
	historyManager := s.runCfg.HistoryManager
	longTermMem := s.runCfg.LongTermMem
	kg := s.runCfg.KG
	registry := s.runCfg.Registry
	manifest := s.runCfg.Manifest
	budgetTracker := s.runCfg.BudgetTracker
	sessionID := s.runCfg.SessionID
	isAutonomousRun := isAutonomousAgentRun(runCfg, sessionID)

	// Mutable state aliases from init
	personalityEnabled := s.personalityEnabled
	emotionSynthesizer := s.emotionSynthesizer
	isMaintenance := s.isMaintenance
	flags := s.flags
	toolingPolicy := s.toolingPolicy
	telemetryScope := s.telemetryScope
	initialUserMsg := s.initialUserMsg
	dailyTodoReminder := s.dailyTodoReminder
	plannerContext := s.plannerContext
	baseAdditionalPrompt := s.baseAdditionalPrompt

	sessionTokens := s.sessionTokens
	recoveryPolicy := s.recoveryPolicy
	recoveryState := s.recoveryState
	emptyRetried := s.emptyRetried
	retry422Count := s.retry422Count
	homepageUsedInChain := s.homepageUsedInChain
	helperManager := s.helperManager
	lastActivity := s.lastActivity
	lastTool := s.lastTool
	recentTools := s.recentTools
	explicitTools := s.explicitTools
	lastResponseWasTool := s.lastResponseWasTool
	ragLastUserMsg := s.ragLastUserMsg
	ragToolIterationsSinceLastRefresh := s.ragToolIterationsSinceLastRefresh
	pendingTCs := s.pendingTCs
	usedMemoryDocIDs := s.usedMemoryDocIDs
	turnToolNames := s.turnToolNames
	turnToolSummaries := s.turnToolSummaries
	lastCompressionMsg := s.lastCompressionMsg
	coreMemCache := s.coreMemCache
	coreMemUpdatedAt := s.coreMemUpdatedAt
	coreMemLoadedAt := s.coreMemLoadedAt
	coreMemDirty := s.coreMemDirty
	tokenCache := s.tokenCache
	detectedCtxWindow := s.detectedCtxWindow
	cachedSysPromptKey := s.cachedSysPromptKey
	cachedSysPrompt := s.cachedSysPrompt
	cachedSysPromptTokens := s.cachedSysPromptTokens
	cachedSysPromptAt := s.cachedSysPromptAt
	sessionTodoList := s.sessionTodoList
	toolGuidesDir := s.toolGuidesDir
	useNativeFunctions := s.useNativeFunctions
	adaptiveFilteredTools := s.adaptiveFilteredTools
	reuseLookup := ReuseLookupResult{}
	lastReuseLookupMsg := ""

	const systemPromptCacheTTL = 30 * time.Second

	loopIterationCount := 0
	for {
		const maxLoopIterations = 100
		loopIterationCount++

		// Safety: prevent infinite loops
		if loopIterationCount > maxLoopIterations {
			s.currentLogger.Error("[Sync] Maximum loop iterations exceeded — aborting to prevent infinite loop", "iterations", loopIterationCount)
			broker.Send("error_recovery", i18n.T(cfg.Server.UILanguage, "backend.stream_error_recovery_loop_exceeded", maxLoopIterations))
			return openai.ChatCompletionResponse{}, fmt.Errorf("agent loop exceeded maximum iterations: %d", maxLoopIterations)
		}

		emotionPolicy := emotionBehaviorPolicy{}
		if !runCfg.IsMission && !isAutonomousRun && personalityEnabled && shortTermMem != nil {
			emotionPolicy = deriveEmotionBehaviorPolicy(shortTermMem, emotionSynthesizer)
		}
		// Per-iteration flag: set when the XML fallback block has already appended an
		// assistant message to req.Messages this turn.  Used below to avoid adding a
		// duplicate assistant entry in the non-native tool-execution branch.
		xmlFallbackHandledThisTurn := false

		// Check for user interrupt
		if checkAndClearInterrupt(sessionID) {
			s.currentLogger.Warn("[Sync] User interrupted the agent — stopping immediately")
			broker.Send("thinking", i18n.T(cfg.Server.UILanguage, "backend.stream_thinking_stopped_by_user"))
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
			s.currentLogger.Warn("[Sync] Lifeboat idle for too long, injecting revive prompt", "minutes", cfg.CircuitBreaker.MaintenanceTimeoutMinutes)
			reviveMsg := "You are idle in the lifeboat. finish your tasks or change back to the supervisor."
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: reviveMsg})
			lastActivity = time.Now() // Reset timer
		}

		// Refresh maintenance status to account for mid-loop handovers
		isMaintenance = isMaintenance || tools.IsBusy()
		flags.IsMaintenanceMode = isMaintenance

		// Caching the logger to avoid opening file on every iteration (leaking FDs)
		if isMaintenance && s.currentLogger == nil {
			logPath := filepath.Join(cfg.Logging.LogDir, "lifeboat.log")
			if l, err := loggerPkg.SetupWithFile(true, logPath, true); err == nil {
				s.currentLogger = l.Logger
			}
		}
		if s.currentLogger == nil {
			s.currentLogger = logger
		}

		s.currentLogger.Debug("[Sync] Agent loop iteration starting", "is_maintenance", isMaintenance, "lock_exists", tools.IsBusy())

		if lastResponseWasTool {
			ragToolIterationsSinceLastRefresh++
		}

		// Tool results are appended as user-role messages; keep loop-scoped context anchored
		// to the original human request instead of the latest tool output.
		lastUserMsg := initialUserMsg
		userEmotionTrigger, userEmotionTriggerDetail, userInactivityHours := detectUserEmotionTrigger(lastUserMsg, shortTermMem, sessionID)

		// Process queued tool calls from multi-tool responses (skip LLM for these)
		if len(pendingTCs) > 0 {
			if processPendingToolCalls(s, ctx, lastUserMsg) {
				pendingTCs = s.pendingTCs
				homepageUsedInChain = s.homepageUsedInChain
				turnToolNames = s.turnToolNames
				turnToolSummaries = s.turnToolSummaries
				lastActivity = s.lastActivity
				sessionTodoList = s.sessionTodoList
				coreMemDirty = s.coreMemDirty
				recentTools = s.recentTools
				lastResponseWasTool = s.lastResponseWasTool
				req = s.req
				continue
			}
		}

		flags.ReuseContext = ""
		if !runCfg.IsMission && !runCfg.IsCoAgent && !isAutonomousRun {
			trimmedReuseQuery := strings.TrimSpace(lastUserMsg)
			if trimmedReuseQuery != "" {
				if trimmedReuseQuery != lastReuseLookupMsg {
					reuseLookup = buildReuseLookup(trimmedReuseQuery, shortTermMem, s.runCfg.CheatsheetDB, s.currentLogger)
					lastReuseLookupMsg = trimmedReuseQuery
				}
				if reuseLookup.Performed {
					flags.ReuseContext = reuseLookup.Prompt
				}
			}
		}

		// Load Personality Meta
		var meta memory.PersonalityMeta
		if personalityEnabled {
			meta = prompts.GetCorePersonalityMeta(cfg.Directories.PromptsDir, flags.CorePersonality)
		}

		// Circuit breaker - berechne Basis-Limit (Tool-spezifische Anpassungen erfolgen später wenn tc bekannt ist)
		effectiveMaxCalls := calculateEffectiveMaxCalls(cfg, ToolCall{}, homepageUsedInChain, personalityEnabled, shortTermMem, s.currentLogger)

		if s.toolCallCount >= effectiveMaxCalls {
			s.currentLogger.Warn("[Sync] Circuit breaker triggered", "count", s.toolCallCount, "limit", effectiveMaxCalls)
			breakerMsg := fmt.Sprintf("CIRCUIT BREAKER: You have reached the maximum of %d consecutive tool calls. You MUST now summarize your progress and respond to the user with a final answer.", effectiveMaxCalls)
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: breakerMsg})
		}

		flags.ActiveProcesses = GetActiveProcessStatus(registry)

		// Load Core Memory (cached, invalidated when manage_memory is called
		// or when the DB timestamp has changed due to external modifications).
		if shortTermMem != nil {
			dbUpdatedAt, err := shortTermMem.GetCoreMemoryUpdatedAt()
			if err == nil && !dbUpdatedAt.IsZero() && !coreMemUpdatedAt.IsZero() && !dbUpdatedAt.Equal(coreMemUpdatedAt) {
				coreMemDirty = true
			}
			if ShouldReloadCoreMemory(coreMemDirty, coreMemLoadedAt, dbUpdatedAt, coreMemUpdatedAt) {
				coreMemCache = shortTermMem.ReadCoreMemory()
				coreMemLoadedAt = time.Now()
				if err == nil {
					coreMemUpdatedAt = dbUpdatedAt
				}
				coreMemDirty = false
			}
		}

		// Extract explicit workflow tools if present (populated from previous iteration's <workflow_plan> tag)
		// explicitTools is persistent across loop iterations

		// Prepare Dynamic Tool Guides
		if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == openai.ChatMessageRoleUser {
			lastUserMsg = messageText(req.Messages[len(req.Messages)-1])
		}

		// Get the mood trigger context from the message history
		triggerValue := getMoodTrigger(req.Messages, lastUserMsg)
		moodTrigger := func() string { return triggerValue }

		// Note: The call to PrepareDynamicGuides will happen after the response is received
		// We initialize flags.PredictedGuides now with empty explicit tools to satisfy builder.go for the first prompt.
		// Skip guide loading in minimal tier — the guides are never injected there (builder.go:443 checks Tier=="full").
		preliminaryTierFlags := prompts.ContextFlags{
			MessageCount:      len(req.Messages),
			IsErrorState:      flags.IsErrorState,
			RequiresCoding:    flags.RequiresCoding,
			RecentlyUsedTools: recentTools,
			PredictedGuides:   explicitTools,
		}
		if preliminaryTier := prompts.DetermineTierAdaptive(&preliminaryTierFlags); preliminaryTier == "full" || len(explicitTools) > 0 {
			// Build skip list: tools that already have native OpenAI function schemas
			// should not also get their guide content (saves tokens, avoids redundancy).
			// Also skip tools that were removed by adaptive filtering — injecting a guide
			// for a tool that no longer has a schema causes model confusion.
			skipTools := make([]string, 0, len(req.Tools)+len(adaptiveFilteredTools))
			for _, t := range req.Tools {
				if t.Function != nil {
					skipTools = append(skipTools, t.Function.Name)
				}
			}
			skipTools = append(skipTools, adaptiveFilteredTools...)
			guideStrategy := toolingPolicy.EffectiveGuideStrategy
			guideStrategy.SkipTools = skipTools
			flags.PredictedGuides = prompts.PrepareDynamicGuidesWithStrategy(
				longTermMem,
				shortTermMem,
				lastUserMsg,
				lastTool,
				toolGuidesDir,
				recentTools,
				explicitTools,
				toolingPolicy.EffectiveMaxToolGuides,
				guideStrategy,
				s.currentLogger,
			)
		} else {
			flags.PredictedGuides = nil
		}
		turnMemoryCandidates := make(map[string]string)
		turnPendingActions := make([]memory.EpisodicMemory, 0, 2)

		// Automatic RAG: retrieve relevant long-term memories for the current user message
		// Phase A3: Over-fetch and re-rank with recency boost from memory_meta
		flags.RetrievedMemories = ""
		flags.PredictedMemories = ""
		retrievalPromptTokens := 0
		var topMemories []string
		if !runCfg.IsMission && !isAutonomousRun && longTermMem != nil && shouldUseRAGForMessage(lastUserMsg) && shouldRefreshRAG(lastUserMsg, ragLastUserMsg, ragToolIterationsSinceLastRefresh, lastResponseWasTool) {
			ragSettings := resolveMemoryAnalysisSettings(cfg, shortTermMem)
			useHelperRAGBatch := helperManager != nil && ragSettings.Enabled && ragSettings.QueryExpansion && ragSettings.LLMReranking
			ragQuery := lastUserMsg
			if useHelperRAGBatch {
				ragQuery = expandQueryForRAG(ctx, cfg, s.currentLogger, lastUserMsg, shortTermMem)
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
			memories, docIDs, err := longTermMem.SearchSimilar(ragQuery, searchLimit, "tool_guides", "documentation")
			RecordRetrievalEventForScope(telemetryScope, "rag_auto_latency:"+retrievalLatencyBucket(time.Since(autoRetrievalStart)))
			if err != nil {
				RecordRetrievalEventForScope(telemetryScope, "rag_auto_error")
			}
			if err == nil {
				ranked := rankMemoryCandidates(memories, docIDs, shortTermMem, usedMemoryDocIDs, time.Now())
				if useHelperRAGBatch {
					batchCtx, batchCancel := context.WithTimeout(ctx, helperRAGBatchTimeout)
					batchResult, batchErr := helperManager.AnalyzeRAG(batchCtx, lastUserMsg, ranked)
					batchCancel()
					if batchErr != nil {
						helperManager.ObserveFallback("rag_batch", batchErr.Error())
						ragQuery = expandQueryForRAG(ctx, cfg, s.currentLogger, lastUserMsg, shortTermMem)
						memories, docIDs, err = longTermMem.SearchSimilar(ragQuery, 6, "tool_guides", "documentation")
						if err == nil {
							ranked = rankMemoryCandidates(memories, docIDs, shortTermMem, usedMemoryDocIDs, time.Now())
							ranked = rerankWithLLM(ctx, cfg, s.currentLogger, ranked, lastUserMsg, shortTermMem)
						} else {
							ranked = nil
						}
					} else {
						if helperQuery := strings.TrimSpace(batchResult.SearchQuery); helperQuery != "" && !strings.EqualFold(helperQuery, strings.TrimSpace(lastUserMsg)) {
							ragQuery = helperQuery
							extraMemories, extraDocIDs, extraErr := longTermMem.SearchSimilar(ragQuery, 4, "tool_guides", "documentation")
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
						ranked = applyHelperRAGScores(s.currentLogger, ranked, batchResult)
					}
				} else {
					// LLM re-ranking: blend LLM relevance scores with policy-ranked scores
					ranked = rerankWithLLM(ctx, cfg, s.currentLogger, ranked, lastUserMsg, shortTermMem)
				}

				// For short queries (<40 chars), apply a softer score filtering to
				// avoid injecting semantically-similar but contextually-irrelevant
				// old memories (e.g. "versuche es erneut" matching old error messages).
				// Use a lower threshold (0.50) to avoid filtering out highly-relevant old memories,
				// and always keep at least the top result if anything was found.
				if len(lastUserMsg) < 40 && len(ranked) > 0 {
					scoreThreshold := 0.50
					var filtered []rankedMemory
					for _, r := range ranked {
						if r.score >= scoreThreshold {
							filtered = append(filtered, r)
						}
					}
					// Preserve at least the top result if it was filtered out — old memories
					// that are semantically highly similar may still be relevant.
					if len(filtered) == 0 && len(ranked) > 0 {
						filtered = ranked[:1]
					}
					if len(filtered) < len(ranked) {
						s.currentLogger.Debug("[RAG] Short-query filter applied", "before", len(ranked), "after", len(filtered))
					}
					ranked = filtered
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
						s.currentLogger.Debug("[RAG] Dropped stale transient memory", "preview", Truncate(r.text, 80))
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
					retrievalPromptTokens += prompts.CountTokensForModel(flags.RetrievedMemories, req.Model)
					RecordRetrievalEventForScope(telemetryScope, "rag_auto_hit")
					RecordRetrievalEventForScope(telemetryScope, "rag_auto_source:ltm")
				} else {
					RecordRetrievalEventForScope(telemetryScope, "rag_auto_filtered_out")
				}
				s.currentLogger.Debug("[Sync] RAG: Retrieved memories (recency-boosted)", "count", len(ranked))
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
					type predictiveFetch struct {
						mem   string
						docID string
						err   error
					}
					fetches := make([]predictiveFetch, len(predictions))

					g, _ := errgroup.WithContext(ctx)
					g.SetLimit(3)
					for i, pred := range predictions {
						i, pred := i, pred
						g.Go(func() error {
							// Use SearchMemoriesOnly: predictive pre-fetch needs only user memories,
							// not tool_guides/documentation — avoids 2 full extra search cycles per request.
							pMem, pIDs, pErr := longTermMem.SearchMemoriesOnly(pred, 1)
							if pErr != nil {
								fetches[i].err = pErr
								return nil
							}
							if len(pMem) > 0 {
								fetches[i].mem = pMem[0]
								if len(pIDs) > 0 {
									fetches[i].docID = pIDs[0]
								}
							}
							return nil
						})
					}
					_ = g.Wait()

					for _, f := range fetches {
						if f.err != nil {
							hadPredictiveError = true
							continue
						}
						if f.mem == "" {
							continue
						}
						if f.docID != "" && usedMemoryDocIDs[f.docID] > 0 {
							continue
						}
						if _, dup := retrievedSet[f.mem]; dup {
							continue
						}
						predictedResults = append(predictedResults, f.mem)
						retrievedSet[f.mem] = struct{}{} // prevent intra-prediction duplicates
						if f.docID != "" && shortTermMem != nil {
							usedMemoryDocIDs[f.docID]++
							_ = shortTermMem.RecordMemoryUsage(f.docID, "ltm_predicted", sessionID, 0, false)
							turnMemoryCandidates[f.docID] = compactMemoryForPrompt(f.mem, 260)
						}
					}
					RecordRetrievalEventForScope(telemetryScope, "rag_predictive_latency:"+retrievalLatencyBucket(time.Since(predictiveStart)))
					if hadPredictiveError {
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_error")
					}
					if len(predictedResults) > 0 {
						flags.PredictedMemories = strings.Join(predictedResults, "\n---\n")
						retrievalPromptTokens += prompts.CountTokensForModel(flags.PredictedMemories, req.Model)
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_hit")
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_source:ltm_predicted")
						s.currentLogger.Debug("[Sync] Predictive RAG: Pre-fetched memories", "count", len(predictedResults), "predictions", predictions, "temporal_predictions", temporalPredictions)
					} else {
						RecordRetrievalEventForScope(telemetryScope, "rag_predictive_miss")
					}
				}
			}

		}

		if len(usedMemoryDocIDs) > 500 {
			usedMemoryDocIDs = make(map[string]int)
		}

		// For capability/availability queries, RAG was intentionally skipped.
		// Inject a live-state policy note so the agent knows not to rely on any
		// stale memory it may have encountered in the conversation history.
		if !runCfg.IsMission && !isAutonomousRun && lastUserMsg != "" && isCapabilityQuery(lastUserMsg) && flags.RetrievedMemories == "" {
			flags.RetrievedMemories = "[Memory Policy] This query concerns agent capabilities or tool/integration availability. " +
				"The authoritative source is the CURRENT TOOL SCHEMA in this context — NOT past memory entries. " +
				"Memory about tool availability is always considered potentially stale. " +
				"If you are unsure whether a tool is present, use discover_tools first. Do not guess, improvise, or attempt hidden tools blindly."
			s.currentLogger.Debug("[RAG] Capability query: injecting live-state policy hint")
		}

		// Inject lightweight recent-day anchors and episodic cards, even when
		// long-term memory retrieval is unavailable/disabled.
		shouldInjectRecentContext := shouldInjectRecentMemoryContext(lastUserMsg)
		if !runCfg.IsMission && !isAutonomousRun && shouldInjectRecentContext && shortTermMem != nil {
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

		if !runCfg.IsMission && !isAutonomousRun && shouldInjectRecentContext && shortTermMem != nil {
			if overview, err := shortTermMem.BuildRecentActivityPromptOverview(3); err == nil {
				flags.RecentActivityOverview = overview
			}
		}

		// Knowledge Graph context injection: search for relevant entities
		if !runCfg.IsMission && !isAutonomousRun && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.PromptInjection && kg != nil && lastUserMsg != "" {
			maxNodes := cfg.Tools.KnowledgeGraph.MaxPromptNodes
			maxChars := cfg.Tools.KnowledgeGraph.MaxPromptChars
			kgContext := kg.SearchForContext(lastUserMsg, maxNodes, maxChars)
			if kgContext != "" {
				flags.KnowledgeContext = kgContext
				s.currentLogger.Debug("[Sync] KG: Injected knowledge context", "chars", len(kgContext))
			}
		}

		// Retrieval Fusion: cross-reference RAG↔KG for bidirectional enrichment.
		// When both RAG and KG produced results, enrich each with the other's findings.
		if !runCfg.IsMission && !isAutonomousRun && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.RetrievalFusion &&
			flags.RetrievedMemories != "" && flags.KnowledgeContext != "" &&
			longTermMem != nil && kg != nil {
			fusionResult := applyRetrievalFusion(topMemories, flags.KnowledgeContext, longTermMem, kg, s.currentLogger)
			if fusionResult.EnrichedMemories != "" {
				flags.RetrievedMemories += "\n---\n" + fusionResult.EnrichedMemories
			}
			if fusionResult.EnrichedKGContext != "" {
				flags.KnowledgeContext += "\n" + fusionResult.EnrichedKGContext
			}
		}

		// Error Pattern Context: inject known errors when in error recovery state.
		// Phase A1/A2: Drop stale entries (>24h old without resolution), tag freshness
		// per entry, and reframe as hypotheses to re-test rather than constraints.
		if flags.IsErrorState && shortTermMem != nil {
			errPatterns, err := shortTermMem.GetRecentErrors(5)
			if err == nil && len(errPatterns) > 0 {
				now := time.Now().UTC()
				filtered := make([]memory.ErrorPattern, 0, len(errPatterns))
				for _, ep := range errPatterns {
					if ep.Resolution != "" {
						filtered = append(filtered, ep)
						continue
					}
					if ts, parseErr := time.Parse(time.RFC3339, ep.LastSeen); parseErr == nil {
						if now.Sub(ts) > 24*time.Hour {
							continue
						}
					}
					filtered = append(filtered, ep)
				}
				if len(filtered) > 0 {
					var epBuf strings.Builder
					epBuf.WriteString("Previously observed tool errors. Treat each as a hypothesis to re-test under current conditions, not as a constraint. They may already be fixed.\n")
					for _, ep := range filtered {
						age := formatErrorAge(ep.LastSeen, now)
						epBuf.WriteString(fmt.Sprintf("- [advisory, verify] Tool: %s | Error: %s | seen %dx", ep.ToolName, ep.ErrorMessage, ep.OccurrenceCount))
						if age != "" {
							epBuf.WriteString(fmt.Sprintf(" | last %s", age))
						}
						if ep.Resolution != "" {
							epBuf.WriteString(fmt.Sprintf(" | Resolution: %s", ep.Resolution))
						}
						epBuf.WriteString("\n")
					}
					flags.ErrorPatternContext = epBuf.String()
				}
			}
		}

		// Phase D: Inject personality line before building system prompt
		if !runCfg.IsMission && !isAutonomousRun && personalityEnabled && shortTermMem != nil {
			if cfg.Personality.EngineV2 {
				// V2 Feature: Narrative Events based on Milestones & Loneliness
				processBehavioralEvents(shortTermMem, &req.Messages, sessionID, meta, s.currentLogger)
			}
			flags.PersonalityLine = shortTermMem.GetPersonalityLineWithMeta(cfg.Personality.EngineV2, meta)

			// Emotion Synthesizer: inject latest emotional description
			if emotionDescription := latestEmotionDescription(shortTermMem, emotionSynthesizer); emotionDescription != "" {
				flags.EmotionDescription = emotionDescription
			}

			// Inner Voice: inject if available and not decayed. During active
			// tool recovery, keep the next prompt strictly procedural.
			if shouldInjectInnerVoiceIntoPrompt(cfg, recoveryState.ConsecutiveErrorCount, runCfg.IsMission, runCfg.IsCoAgent, lastResponseWasTool) {
				if iv, ivCategory := getInnerVoiceForPrompt(sessionID, cfg.Personality.InnerVoice.DecayTurns, cfg.Personality.InnerVoice.DecayMaxAgeSecs); iv != "" {
					flags.InnerVoice = iv
					s.currentLogger.Info("[InnerVoice] Injecting inner voice into system prompt",
						"session_id", sessionID,
						"category", ivCategory,
						"content", iv)
				}
			}
		}

		// User Profiling: inject behavioral instruction + collected profile data
		if !runCfg.IsMission && !isAutonomousRun && cfg.Personality.UserProfiling {
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
		flags.Tier = prompts.DetermineTierAdaptive(&flags)
		if runCfg.IsMission {
			flags.IsMission = true
			flags.Tier = "minimal"
		} else {
			adjustedTier := applyTelemetryAwarePromptTier(toolingPolicy, flags, flags.Tier)
			if adjustedTier != flags.Tier {
				RecordToolPolicyEventForScope(telemetryScope, "prompt_tier_compact")
				s.currentLogger.Info("[ToolingPolicy] Telemetry-aware prompt tier adjustment",
					"provider", telemetryScope.ProviderType,
					"model", telemetryScope.Model,
					"from", flags.Tier,
					"to", adjustedTier,
					"message_count", flags.MessageCount,
					"failure_rate", toolingPolicy.TelemetrySnapshot.FailureRate)
				flags.Tier = adjustedTier
			}
		}
		flags.IsDebugMode = cfg.Agent.DebugMode || GetDebugMode()                                           // re-check each iteration (toggleable at runtime)
		flags.IsVoiceMode = GetVoiceMode() && !isAutonomousAgentRun(runCfg, sessionID) && !runCfg.IsMission // re-check each iteration (toggleable at runtime)

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

		flags.PlannerContext = plannerContext
		flags.DailyTodoReminder = dailyTodoReminder
		flags.OperationalIssueReminder = s.operationalIssueReminder

		// Inject session todo list into system prompt context
		flags.SessionTodoItems = sessionTodoList
		if shortTermMem != nil {
			if planPrompt, err := shortTermMem.BuildSessionPlanPrompt(sessionID); err == nil && strings.TrimSpace(planPrompt) != "" {
				flags.SessionTodoItems = planPrompt
			}
		}
		// When inner voice is active, suppress general emotion guidance while
		// preserving trait-driven curiosity guidance.
		flags.AdditionalPrompt = mergeEmotionBehaviorPrompt(baseAdditionalPrompt, emotionPolicy, flags.InnerVoice != "")
		flags.TokenBudget = calculateEffectivePromptTokenBudget(cfg, ToolCall{}, homepageUsedInChain, s.currentLogger)
		recordRetrievalPromptTelemetry(telemetryScope, retrievalPromptTokens, flags.TokenBudget)
		reconcilePromptToolModeWithRequest(&flags, &toolingPolicy, req.Tools, s.currentLogger)

		// Skip integrations that already have native schemas in the overview
		skipIntegrationTools := make([]string, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Function != nil {
				skipIntegrationTools = append(skipIntegrationTools, t.Function.Name)
			}
		}
		flags.SkipIntegrationTools = skipIntegrationTools

		budgetHint := ""
		if budgetTracker != nil {
			budgetHint = budgetTracker.GetPromptHint()
		}

		keyFlags := flags
		keyFlags.MessageCount = 0 // MessageCount only affects tier selection & metrics, not the prompt content.
		cacheKey, cacheKeyErr := buildSystemPromptCacheKey(cfg.Directories.PromptsDir, &keyFlags, coreMemCache, budgetHint)
		cacheHit := cacheKeyErr == nil &&
			cacheKey != "" &&
			cacheKey == cachedSysPromptKey &&
			cachedSysPrompt != "" &&
			!cachedSysPromptAt.IsZero() &&
			time.Since(cachedSysPromptAt) <= systemPromptCacheTTL

		sysPrompt := ""
		sysPromptTokens := 0
		if cacheHit {
			sysPrompt = cachedSysPrompt
			sysPromptTokens = cachedSysPromptTokens
		} else {
			sysPrompt, sysPromptTokens = prompts.BuildSystemPrompt(cfg.Directories.PromptsDir, &flags, coreMemCache, s.currentLogger)
			if budgetHint != "" {
				sysPrompt += "\n\n" + budgetHint
				sysPromptTokens += prompts.CountTokensForModel(budgetHint, req.Model) + 2
			}

			if cacheKeyErr == nil && cacheKey != "" {
				cachedSysPromptKey = cacheKey
				cachedSysPrompt = sysPrompt
				cachedSysPromptTokens = sysPromptTokens
				cachedSysPromptAt = time.Now()
			} else {
				cachedSysPromptKey = ""
				cachedSysPrompt = ""
				cachedSysPromptTokens = 0
				cachedSysPromptAt = time.Time{}
			}
		}

		s.currentLogger.Debug("[Sync] System prompt ready",
			"cache_hit", cacheHit,
			"length", len(sysPrompt),
			"tier", flags.Tier,
			"tokens", sysPromptTokens,
			"error_state", flags.IsErrorState,
			"coding_mode", flags.RequiresCoding,
			"active_daemons", flags.ActiveProcesses,
		)

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
			if detectedCtxWindow == 0 {
				detectedCtxWindow = llm.DetectContextWindow(cfg.LLM.BaseURL, cfg.LLM.APIKey, req.Model, cfg.LLM.ProviderType, s.currentLogger)
			}
			if detectedCtxWindow > 0 {
				ctxWindow = detectedCtxWindow
			} else {
				ctxWindow = 163840
			}
		}
		completionMargin := 4096
		maxHistoryTokens := ctxWindow - completionMargin
		if maxHistoryTokens < 4096 {
			maxHistoryTokens = 4096
		}
		// Resolve compression client/model once per session, not per iteration.
		if s.cachedCompressionClient == nil && s.cachedCompressionModel == "" {
			s.cachedCompressionClient, s.cachedCompressionModel = resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
		}
		compressionClient, compressionModel := s.cachedCompressionClient, s.cachedCompressionModel
		var compRes CompressHistoryResult
		if compressionClient != nil && compressionModel != "" {
			// Pre-check threshold: use a cheap cached count to show UI feedback
			// before the potentially slow synchronous LLM call inside CompressHistory.
			compressionTokens := 0
			for _, m := range req.Messages {
				compressionTokens += tokenCache.Count(messageText(m), req.Model) + 4
			}
			compressionThreshold := int(float64(maxHistoryTokens) * compressionThresholdPct)
			if compressionTokens > compressionThreshold {
				broker.Send("thinking", "Compressing context...")
			}
			req.Messages, lastCompressionMsg, compRes = CompressHistory(
				ctx, req.Messages, maxHistoryTokens, compressionModel, compressionClient, lastCompressionMsg, s.currentLogger,
			)
			if compRes.Compressed {
				s.currentLogger.Debug("[Compression] History compressed", "dropped", compRes.DroppedCount, "summary_tokens", compRes.SummaryTokens)
			}
		}

		// ── Context window guard ──
		// Use TotalTokens from CompressHistory when available (avoids re-counting
		// all messages). Otherwise count from scratch for the fallback path.
		totalMsgTokens := compRes.TotalTokens
		if totalMsgTokens == 0 && len(req.Messages) > 1 {
			for _, m := range req.Messages {
				totalMsgTokens += prompts.CountTokensForModel(messageText(m), req.Model) + 4
			}
		} else {
			// compRes.TotalTokens excludes system prompt — add it back
			totalMsgTokens += sysPromptTokens + 4
		}
		if len(req.Tools) > 0 {
			toolSchemaJSON, _ := json.Marshal(req.Tools)
			totalMsgTokens += prompts.CountTokensForModel(string(toolSchemaJSON), req.Model)
		}
		if totalMsgTokens > maxHistoryTokens && len(req.Messages) > 2 {
			broker.Send("thinking", "Trimming context window...")
			s.currentLogger.Warn("[ContextGuard] Token limit exceeded before LLM call — trimming history",
				"tokens", totalMsgTokens, "limit", maxHistoryTokens, "messages", len(req.Messages))
			sysMsg := req.Messages[0]
			lastMsg := req.Messages[len(req.Messages)-1]
			var droppedMessages []openai.ChatCompletionMessage
			// Drop messages from index 1 onward (oldest first) until we fit.
			// Always keep system (0), the latest message, and at least 4 recent messages
			// to preserve conversation context continuity.
			const minPreservedMessages = 4
			mid := req.Messages[1 : len(req.Messages)-1]
			preserveFrom := len(mid)
			if preserveFrom > minPreservedMessages {
				preserveFrom = len(mid) - minPreservedMessages
			}
			for totalMsgTokens > maxHistoryTokens && len(mid) > preserveFrom {
				dropped := mid[0]
				droppedMessages = append(droppedMessages, dropped)
				mid = mid[1:]
				totalMsgTokens -= prompts.CountTokensForModel(messageText(dropped), req.Model) + 4
			}
			trimmedMessages := []openai.ChatCompletionMessage{sysMsg}
			remainingRecapBudget := maxHistoryTokens - totalMsgTokens - 4
			if recap := buildTrimmedContextRecap(droppedMessages, remainingRecapBudget); recap != "" {
				trimmedMessages = append(trimmedMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: recap,
				})
				totalMsgTokens += prompts.CountTokensForModel(recap, req.Model) + 4
			}
			req.Messages = append(trimmedMessages, append(mid, lastMsg)...)
			req.Messages = trim422Messages(req.Messages)
			s.currentLogger.Info("[ContextGuard] History trimmed",
				"remaining_messages", len(req.Messages), "estimated_tokens", totalMsgTokens, "dropped_messages", len(droppedMessages))
		}

		// Verbose Logging of LLM Request
		if len(req.Messages) > 0 {
			lastMsg := req.Messages[len(req.Messages)-1]
			// Keep conversation logs in the original logger (stdout) to avoid pollution of technical log
			lastMsgText := messageText(lastMsg)
			logger.Info("[LLM Request]", "role", lastMsg.Role, "content_len", len(lastMsgText), "preview", Truncate(lastMsgText, 200))
			s.currentLogger.Info("[LLM Request Redirected]", "role", lastMsg.Role, "content_len", len(lastMsgText))
			s.currentLogger.Debug("[LLM Full History]", "messages_count", len(req.Messages))
		}

		// Prompt log: append full request JSON to prompts.log when enabled
		if cfg.Logging.EnablePromptLog {
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
			if err := loggerPkg.AppendPromptLogEntry(cfg.Logging.LogDir, entry); err != nil {
				s.currentLogger.Warn("[PromptLog] Failed to write entry", "error", err)
			}
		}

		broker.Send("thinking", "")

		// Pre-send validation: ensure tool-call integrity before sending to the
		// provider. This catches orphaned tool results that slipped through
		// GetForLLM() or were introduced by context compression / trimming.
		if sanitized, dropped := SanitizeToolMessages(req.Messages); dropped > 0 {
			s.currentLogger.Warn("[PreSend] Sanitized orphaned tool messages before LLM call",
				"dropped", dropped, "before", len(req.Messages), "after", len(sanitized))
			req.Messages = sanitized
		}

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
		case "generate_image", "generate_video":
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
			s.currentLogger.Debug("[Temperature] Modulation applied", "base", baseTemp, "personality_delta", tempDelta, "creative_delta", creativeDelta, "effective", effectiveTemp)
		}

		// Budget check: block if daily budget exceeded and enforcement = full
		if budgetTracker != nil && budgetTracker.IsBlocked("chat") {
			broker.Send("budget_blocked", i18n.T(cfg.Server.UILanguage, "backend.stream_budget_blocked"))
			return openai.ChatCompletionResponse{}, fmt.Errorf("budget exceeded (enforcement=full)")
		}

		telemetryScope = refreshTelemetryScope(telemetryScope, client, nil)

		// Configurable timeout for each individual LLM call to prevent infinite hangs
		llmTimeout := time.Duration(cfg.CircuitBreaker.LLMTimeoutSeconds) * time.Second
		llmCtx, cancelResp := context.WithTimeout(ctx, llmTimeout)

		thinkingCB := func(content, state string) {
			broker.SendThinkingBlock("anthropic", content, state)
		}
		llmCtx = llm.WithThinkingCallback(llmCtx, thinkingCB)

		var resp openai.ChatCompletionResponse
		var content string
		var promptTokens, completionTokens, totalTokens int
		var tokenSource string

		if stream {
			chunkIdleTimeout := time.Duration(cfg.CircuitBreaker.LLMStreamChunkTimeoutSeconds) * time.Second
			if chunkIdleTimeout <= 0 {
				chunkIdleTimeout = 30 * time.Second
			}
			if llmTimeout > 0 && chunkIdleTimeout > llmTimeout {
				chunkIdleTimeout = llmTimeout
			}
			result := handleStreamingResponse(llmCtx, req, client, emptyRetried, recoveryPolicy, s.currentLogger, broker, telemetryScope, cancelResp, chunkIdleTimeout)
			if result.recoveryContinue {
				continue
			}
			if result.err != nil {
				return openai.ChatCompletionResponse{}, result.err
			}
			resp = result.resp
			content = result.content
			promptTokens = result.promptTokens
			completionTokens = result.completionTokens
			totalTokens = result.totalTokens
			tokenSource = result.tokenSource
		} else {
			result := handleSyncLLMCall(llmCtx, req, client, emptyRetried, recoveryPolicy, s.currentLogger, broker, telemetryScope, cancelResp, &retry422Count)
			if result.recoveryContinue {
				continue
			}
			if result.err != nil {
				return openai.ChatCompletionResponse{}, result.err
			}
			resp = result.resp
			content = result.content
			telemetryScope = result.telemetryScope
		}

		cancelResp()
		telemetryScope = refreshTelemetryScope(telemetryScope, client, &resp)

		retry422Count = 0 // reset on successful LLM response

		if recoverFromEmptyResponseWithPolicy(recoveryPolicy, resp, content, &req, &emptyRetried, s.currentLogger, broker, telemetryScope) {
			continue
		}
		emptyRetried = false // reset only after confirmed non-empty response

		// Safety Check: Strip "RECAP" hallucinations if the model is still stuck in the old pattern
		content = strings.TrimPrefix(content, "[RECAP OF PREVIOUS DISCUSSIONS]:")
		content = strings.TrimPrefix(content, "[RECAP OF PREVIOUS DISCUSSIONS]:\n")
		content = strings.TrimPrefix(content, "[CONTEXT_RECAP]:")
		content = strings.TrimPrefix(content, "[CONTEXT_RECAP]:\n")
		content = strings.TrimSpace(content)
		content = stripLeakedTodoList(content)

		// Conversation log to stdout
		logger.Info("[LLM Response]", "content_len", len(content), "preview", Truncate(content, 200))
		// Activity log to file
		s.currentLogger.Info("[LLM Response Received]", "content_len", len(content))
		lastActivity = time.Now() // LLM activity

		parsedToolResp := parseToolResponse(resp, s.currentLogger, telemetryScope)
		// Strip the <done/> completion signal from the raw content that gets persisted
		// to history. The streaming layer already filters it from SSE deltas, but the
		// assembled content still contains it and would appear in the chat on reload.
		if parsedToolResp.IsFinished {
			content = strings.TrimSpace(strings.ReplaceAll(content, "<done/>", ""))
		}
		tc := parsedToolResp.ToolCall
		tc = normalizeParsedToolShortcut(tc)
		parsedToolResp.ToolCall = tc
		useNativePath := parsedToolResp.UseNativePath
		nativeAssistantMsg := parsedToolResp.NativeAssistantMsg
		if parsedToolResp.ParseSource == ToolCallParseSourceNative {
			nativeCall := resp.Choices[0].Message.ToolCalls[0]
			s.currentLogger.Info("[Sync] Native tool call detected", "function", tc.Action, "id", nativeCall.ID, "forced", !cfg.LLM.UseNativeFunctions)
		} else if parsedToolResp.ParseSource == ToolCallParseSourceReasoningCleanJSON {
			s.currentLogger.Info("[Sync] Tool call detected after stripping reasoning tags", "function", tc.Action)
			// Validate that the extracted action name is a known builtin or custom tool.
			// Models sometimes include JSON tool calls in their <think> reasoning blocks that
			// reference non-existent tools (e.g. docker_exec, docker_list_containers).
			// Dispatching these wastes a round-trip and causes spurious WARN logs.
			// Only apply this check for reasoning-extracted calls because native and
			// text-mode calls went through the user's explicit instruction flow.
			if tc.IsTool && tc.Action != "" {
				knownActions := knownReasoningExtractedActionSet(req.Tools, manifest)
				if _, known := knownActions[tc.Action]; !known {
					s.currentLogger.Warn("[Sync] Dropping reasoning-extracted tool call: action not in known tool set", "action", tc.Action)
					feedbackMsg := applyEmotionRecoveryNudge(FormatDiscoverToolsFirstFeedback(tc.Action), emotionPolicy)
					msgs := s.recoverySession.PersistRecoveryMessages(PersistRecoveryParams{
						SessionID:            sessionID,
						FeedbackMsg:          feedbackMsg,
						BrokerEventType:      "error_recovery",
						SkipAssistantPersist: true,
					}, shortTermMem, historyManager)
					s.req.Messages = append(s.req.Messages, msgs...)
					s.lastResponseWasTool = false
					req = s.req
					parsedToolResp.ToolCall.IsTool = false
					tc = parsedToolResp.ToolCall
					// Also discard any pending calls from the same reasoning block
					if len(parsedToolResp.PendingToolCalls) > 0 {
						s.currentLogger.Warn("[Sync] Dropping pending tool calls from same reasoning block", "count", len(parsedToolResp.PendingToolCalls))
						parsedToolResp.PendingToolCalls = nil
					}
					continue
				}
			}
		}
		if len(parsedToolResp.PendingToolCalls) > 0 {
			for i := range parsedToolResp.PendingToolCalls {
				parsedToolResp.PendingToolCalls[i] = normalizeParsedToolShortcut(parsedToolResp.PendingToolCalls[i])
			}
			pendingTCs = queuePendingToolCalls(s, pendingTCs, parsedToolResp.PendingToolCalls)
			s.currentLogger.Info("[MultiTool] Queued additional tool calls from response", "count", len(parsedToolResp.PendingToolCalls), "source", parsedToolResp.ParseSource)
		}

		// Unified token accounting: sync path uses provider usage directly.
		// Streaming path has already finalized via streamAcct before this block.
		if !stream {
			promptTokens = resp.Usage.PromptTokens
			completionTokens = resp.Usage.CompletionTokens
			totalTokens = resp.Usage.TotalTokens
			tokenSource = "provider_usage"
		}

		var usedFallbackEstimate bool
		promptTokens, completionTokens, totalTokens, tokenSource, usedFallbackEstimate = applyTokenEstimationFallback(
			promptTokens,
			completionTokens,
			totalTokens,
			tokenSource,
			req,
			content,
		)
		if usedFallbackEstimate {
			SetGlobalTokenEstimated(true)
			s.currentLogger.Warn("[TokenEstimation] Provider returned zero tokens — falling back to estimation which may be inaccurate", "model", req.Model)
		}
		if details := resp.Usage.PromptTokensDetails; details != nil && details.CachedTokens > 0 {
			s.currentLogger.Debug("[PromptCache] Provider reported cached prompt tokens", "cached_tokens", details.CachedTokens, "prompt_tokens", promptTokens, "model", req.Model)
		}

		sessionTokens += totalTokens
		localGlobalTotal := AddGlobalTokenCount(totalTokens)
		localIsEstimated := tokenSource == "fallback_estimate"

		if cfg.CoAgents.CircuitBreaker.MaxTokens > 0 && sessionTokens >= cfg.CoAgents.CircuitBreaker.MaxTokens {
			s.currentLogger.Warn("[Sync] Co-agent token budget exceeded", "used", sessionTokens, "budget", cfg.CoAgents.CircuitBreaker.MaxTokens)
			breakerMsg := fmt.Sprintf("CIRCUIT BREAKER: Token budget of %d reached (used: %d). You MUST now provide your final answer immediately.", cfg.CoAgents.CircuitBreaker.MaxTokens, sessionTokens)
			req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: breakerMsg})
		}

		broker.SendTokenUpdate(promptTokens, completionTokens, totalTokens, sessionTokens, int(localGlobalTotal), localIsEstimated, true, tokenSource)

		// Budget tracking: record cost and send status to UI
		if budgetTracker != nil {
			actualModel := resp.Model
			if actualModel == "" {
				actualModel = req.Model
			}
			budgetCategory := "chat"
			if runCfg.IsCoAgent || isCoAgentSession(sessionID) {
				budgetCategory = "coagent"
			}
			crossedWarning := budgetTracker.RecordForCategory(budgetCategory, actualModel, promptTokens, completionTokens)
			budgetJSON := budgetTracker.GetStatusJSON()
			if budgetJSON != "" {
				broker.SendJSON(budgetJSON)
			}
			if crossedWarning {
				bs := budgetTracker.GetStatus()
				broker.Send("budget_warning", i18n.T(cfg.Server.UILanguage, "backend.stream_budget_warning", fmt.Sprintf("%.0f", bs.Percentage*100), fmt.Sprintf("$%.4f", bs.SpentUSD), fmt.Sprintf("$%.2f", bs.DailyLimit)))
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
					broker.Send("budget_blocked", i18n.T(cfg.Server.UILanguage, "backend.stream_budget_exceeded", fmt.Sprintf("$%.4f", bs.SpentUSD), fmt.Sprintf("$%.2f", bs.DailyLimit), bs.Enforcement))
				} else {
					broker.Send("budget_warning", i18n.T(cfg.Server.UILanguage, "backend.stream_budget_exceeded", fmt.Sprintf("$%.4f", bs.SpentUSD), fmt.Sprintf("$%.2f", bs.DailyLimit), bs.Enforcement))
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

		s.currentLogger.Debug("[Sync] Tool detection", "is_tool", tc.IsTool, "action", tc.Action, "raw_code", tc.RawCodeDetected)

		// CHANGE LOG 2026-04-11: Telemetry overlay for RecoveryClassifier.
		// Classifies the current tool call attempt for observability. This does NOT
		// change behavior — it only logs the category for future migration planning.
		// When the ConsolidatedRecoveryHandler is fully integrated, this overlay will
		// replace the 7+ individual feedback loops below.
		if !tc.IsTool {
			problem := ClassifyToolCallProblem(tc, content, parsedToolResp, useNativeFunctions)
			if problem.Category != RecoveryCategoryNone {
				RecordToolRecoveryEventForScope(telemetryScope, "classifier_"+problem.Category.String()+"_"+problem.SubType)
				s.currentLogger.Debug("[RecoveryClassifier] Problem detected",
					"category", problem.Category.String(),
					"subtype", problem.SubType,
					"retryable", problem.Retryable)
			}
		}

		content, tc, shouldContinue, xmlFallbackHandled := handleAgentLoopRecoveries(s, content, tc, parsedToolResp, useNativePath, emotionPolicy)
		if shouldContinue {
			explicitTools = s.explicitTools
			lastResponseWasTool = s.lastResponseWasTool
			req = s.req
			continue
		}
		if xmlFallbackHandled {
			xmlFallbackHandledThisTurn = true
			req = s.req
		}

		// Berechne effektives Limit neu mit bekanntem tc (für Tool-spezifische Anpassungen)
		effectiveMaxCallsWithTool := calculateEffectiveMaxCalls(cfg, tc, homepageUsedInChain, personalityEnabled, shortTermMem, s.currentLogger)

		if tc.IsTool && s.toolCallCount < effectiveMaxCallsWithTool {
			resp, err, shouldContinue := executeAgentToolTurn(s, ctx, tc, resp, content, useNativePath, nativeAssistantMsg, lastUserMsg, triggerValue, xmlFallbackHandledThisTurn)
			if !shouldContinue {
				return resp, err
			}
			// Sync modified state back to local aliases for the next iteration
			homepageUsedInChain = s.homepageUsedInChain
			lastResponseWasTool = s.lastResponseWasTool
			lastActivity = s.lastActivity
			coreMemDirty = s.coreMemDirty
			recentTools = s.recentTools
			turnToolNames = s.turnToolNames
			turnToolSummaries = s.turnToolSummaries
			req = s.req
			flags = s.flags
			lastTool = s.lastTool
			pendingTCs = s.pendingTCs
			continue
		}

		// Final answer
		if content == "" {
			content = "[Empty Response]"
		}
		s.currentLogger.Debug("[Sync] Final answer", "content_len", len(content), "content_preview", Truncate(content, 200))

		// Don't persist [Empty Response] as a real message — it pollutes future context
		isEmpty := content == "[Empty Response]"
		// Strip <tool_response> hallucinations before persisting — the model fabricated a
		// tool result instead of calling the tool; only keep any human-readable preamble.
		if idx := strings.Index(strings.ToLower(content), "<tool_response"); idx != -1 {
			content = strings.TrimSpace(content[:idx])
			if content == "" {
				isEmpty = true
			}
		}
		if !isEmpty {
			isInternalFinal := runCfg.MessageSource == "heartbeat" || sessionID == "heartbeat"
			id, err := shortTermMem.InsertMessage(sessionID, resp.Choices[0].Message.Role, content, false, isInternalFinal)
			if err != nil {
				s.currentLogger.Error("Failed to persist final-answer message to SQLite", "error", err)
			}
			if sessionID == "default" && !isInternalFinal {
				historyManager.Add(resp.Choices[0].Message.Role, content, id, false, false)
			}
		} else {
			s.currentLogger.Warn("[Sync] Skipping history persistence for empty response")
		}
		// Fire "done" AFTER the message is persisted so that the UI can reliably
		// fall back to /history if the HTTP response was lost (e.g. page refresh
		// during a long-running agent run).
		broker.Send("done", i18n.T(cfg.Server.UILanguage, "backend.stream_done"))

		memAnalysis := resolveMemoryAnalysisSettings(cfg, shortTermMem)
		runTurnSideEffects := shouldRunTurnSideEffects(runCfg, sessionID, flags)
		useBatchedTurnHelper := helperManager != nil && memAnalysis.Enabled && memAnalysis.RealTime && !isEmpty && shortTermMem != nil && runTurnSideEffects
		useBatchedTurnPersonality := useBatchedTurnHelper && personalityEnabled && cfg.Personality.EngineV2

		// Phase D: Final mood + trait update + milestone check at session end
		// Skip personality side-effects for missions, heartbeats, and maintenance —
		// these are background/autonomous runs that should not update mood, traits,
		// or trigger emotion synthesis.
		if personalityEnabled && shortTermMem != nil && !isAutonomousRun && !runCfg.IsMission && !flags.IsMission && !runCfg.IsCoAgent && !runCfg.IsMaintenance && sessionID != "maintenance" {
			if cfg.Personality.EngineV2 {
				if !useBatchedTurnPersonality {
					launchAsyncPersonalityV2Analysis(
						sessionID,
						cfg,
						s.currentLogger,
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
						recoveryState.TotalErrorCount,
						s.toolCallCount-recoveryState.ConsecutiveErrorCount,
						flags.IsMission,
						flags.IsCoAgent,
					)
				}
			} else {
				mood, traitDeltas := memory.DetectMood(lastUserMsg, "", meta)
				currentTraits, _ := shortTermMem.GetTraits()
				// O-08: Apply emotion bias from synthesizer to contextualize V1 detection.
				if emotionSynthesizer != nil {
					mood = memory.ApplyEmotionBias(mood, emotionSynthesizer.GetLastEmotion(), currentTraits)
				}
				_ = shortTermMem.LogMood(mood, moodTrigger())
				for trait, delta := range traitDeltas {
					_ = shortTermMem.UpdateTrait(trait, dampenTraitDelta(currentTraits[trait], delta))
				}
			}
		}

		if memAnalysis.EffectivenessTracking && !isEmpty && runTurnSideEffects && shortTermMem != nil && len(turnMemoryCandidates) > 0 {
			usefulIDs, uselessIDs := assessMemoryEffectiveness(content, turnMemoryCandidates)
			for _, memoryID := range usefulIDs {
				if err := shortTermMem.RecordMemoryEffectiveness(memoryID, true); err != nil {
					s.currentLogger.Debug("Failed to record useful memory effectiveness", "memory_id", memoryID, "error", err)
					continue
				}
				RecordRetrievalEventForScope(telemetryScope, "memory_effectiveness_useful")
			}
			for _, memoryID := range uselessIDs {
				if err := shortTermMem.RecordMemoryEffectiveness(memoryID, false); err != nil {
					s.currentLogger.Debug("Failed to record useless memory effectiveness", "memory_id", memoryID, "error", err)
					continue
				}
				RecordRetrievalEventForScope(telemetryScope, "memory_effectiveness_useless")
			}
		}
		if !isEmpty && runTurnSideEffects && shortTermMem != nil && len(turnPendingActions) > 0 {
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
				batchSuccessCount := s.toolCallCount - recoveryState.ConsecutiveErrorCount
				batchTaskCompleted := recoveryState.ConsecutiveErrorCount == 0 && batchSuccessCount > 0
				batchIVEnabled := shouldGenerateInnerVoice(sessionID, cfg, recoveryState.ConsecutiveErrorCount, recoveryState.TotalErrorCount, batchSuccessCount, batchTaskCompleted, flags.IsMission, flags.IsCoAgent)
				// Fetch inner voice history for narrative continuity
				var batchIVHistory string
				if batchIVEnabled && shortTermMem != nil {
					if ivEntries, ivErr := shortTermMem.GetRecentInnerVoices(3); ivErr == nil && len(ivEntries) > 0 {
						batchIVHistory = memory.FormatInnerVoiceHistory(ivEntries)
					}
				}
				turnPersonalityInput = &helperTurnPersonalityInput{
					RecentHistory:      contextHistory,
					UserOnlyHistory:    userHistory,
					Language:           cfg.Agent.SystemLanguage,
					Traits:             traits,
					PreviousEmotion:    previousEmotion,
					TriggerInfo:        moodTrigger(),
					TriggerType:        userEmotionTrigger,
					TriggerDetail:      userEmotionTriggerDetail,
					InactivityHours:    userInactivityHours,
					ErrorCount:         recoveryState.ConsecutiveErrorCount,
					SuccessCount:       batchSuccessCount,
					InnerVoiceEnabled:  batchIVEnabled,
					InnerVoiceLanguage: cfg.Agent.SystemLanguage,
					InnerVoiceHistory:  batchIVHistory,
				}
			}
			go func(userMsg, aResp, sid string, toolNames, toolSummaries []string, personalityInput *helperTurnPersonalityInput, recentMsgs []openai.ChatCompletionMessage) {
				analysisCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				batchResult, err := helperManager.AnalyzeTurn(analysisCtx, userMsg, aResp, toolNames, toolSummaries, personalityInput)
				if err != nil {
					helperManager.ObserveFallback("analyze_turn", err.Error())
					s.currentLogger.Warn("[HelperLLM] Batched turn analysis failed, falling back", "error", err)
					if useBatchedTurnPersonality {
						launchAsyncPersonalityV2Analysis(
							sid,
							cfg,
							s.currentLogger,
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
							recoveryState.TotalErrorCount,
							s.toolCallCount-recoveryState.ConsecutiveErrorCount,
							flags.IsMission,
							flags.IsCoAgent,
						)
					}
					runMemoryAnalysis(analysisCtx, cfg, s.currentLogger, shortTermMem, kg, longTermMem, userMsg, aResp, sid)
					captureActivityTurn(
						cfg,
						s.currentLogger,
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

				applyMemoryAnalysisResult(cfg, s.currentLogger, shortTermMem, longTermMem, sid, batchResult.MemoryAnalysis)
				if useBatchedTurnPersonality {
					if personalityResult, ok := normalizeHelperTurnPersonalityResult(batchResult.PersonalityAnalysis, meta); ok {
						_, previousEmotion := resolveHelperEmotionBatchState(cfg, emotionSynthesizer)
						v2FailCount.Store(0)
						applyPersonalityV2AnalysisResult(
							sid,
							cfg,
							s.currentLogger,
							shortTermMem,
							emotionSynthesizer,
							previousEmotion,
							moodTrigger(),
							userEmotionTrigger,
							userEmotionTriggerDetail,
							userInactivityHours,
							cfg.Personality.UserProfiling,
							recoveryState.ConsecutiveErrorCount,
							s.toolCallCount-recoveryState.ConsecutiveErrorCount,
							personalityResult,
						)
					} else {
						helperManager.ObserveFallback("analyze_turn_personality", "personality_payload_invalid")
						launchAsyncPersonalityV2Analysis(
							sid,
							cfg,
							s.currentLogger,
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
							recoveryState.TotalErrorCount,
							s.toolCallCount-recoveryState.ConsecutiveErrorCount,
							flags.IsMission,
							flags.IsCoAgent,
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
			if memAnalysis.Enabled && memAnalysis.RealTime && !isEmpty && runTurnSideEffects && shortTermMem != nil {
				go func(userMsg, aResp, sid string) {
					analysisCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
					defer cancel()
					runMemoryAnalysis(analysisCtx, cfg, s.currentLogger, shortTermMem, kg, longTermMem, userMsg, aResp, sid)
				}(lastUserMsg, content, sessionID)
			}

			if !isEmpty && runTurnSideEffects && shortTermMem != nil {
				activityToolNames := append([]string(nil), turnToolNames...)
				activityToolSummaries := append([]string(nil), turnToolSummaries...)
				go captureActivityTurn(
					cfg,
					s.currentLogger,
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
		if runTurnSideEffects {
			JournalAutoTrigger(cfg, shortTermMem, s.currentLogger, sessionID, recentTools, lastUserMsg)
		}

		if !isEmpty && runTurnSideEffects {
			rf := cfg.Agent.ReuseFirst
			if !rf.AutoMaterialize {
				s.currentLogger.Debug("[ReuseFirst] Auto-materialisation disabled by config", "session_id", sessionID)
			} else if reuseFirstSessionAtCap(sessionID, rf.MaxArtifactsPerSession) {
				s.currentLogger.Info("[ReuseFirst] Skipping materialisation: per-session cool-down reached",
					"session_id", sessionID,
					"cap", rf.MaxArtifactsPerSession)
			} else {
				outcome := DeriveRunOutcomeFromSummaries(turnToolSummaries)
				evaluation := evaluateReusabilityWithOutcome(lastUserMsg, content, turnToolNames, turnToolSummaries, reuseLookup, outcome, rf.RequireSuccessSignal)
				if evaluation.Decision == ReusableArtifactNone {
					s.currentLogger.Debug("[ReuseFirst] No materialisation",
						"session_id", sessionID,
						"reason", evaluation.Reason,
						"any_tool_error", outcome.AnyToolError,
						"recovery_loop_hits", outcome.RecoveryLoopHits)
				} else {
					if err := applyReusabilityDecision(runCfg, s.currentLogger, evaluation); err != nil {
						s.currentLogger.Warn("[ReuseFirst] Failed to apply reusability decision", "error", err, "reuse_decision", evaluation.Decision)
					} else {
						reuseFirstSessionRecord(sessionID, evaluation.Decision)
					}
				}
			}
		}

		// Weekly reflection: async trigger if configured and due
		// Guard: only run once per day by checking if a reflection entry already exists today.
		if memAnalysis.Enabled && memAnalysis.WeeklyReflection && runTurnSideEffects && weeklyReflectionDue(cfg, shortTermMem) && shortTermMem != nil {
			today := time.Now().Format("2006-01-02")
			existing, _ := shortTermMem.GetJournalEntries(today, today, []string{"reflection"}, 1)
			if len(existing) == 0 {
				go func() {
					reflCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
					defer cancel()
					_, err := generateMemoryReflection(reflCtx, cfg, s.currentLogger, shortTermMem, kg, longTermMem, client, "recent")
					if err != nil {
						s.currentLogger.Warn("Weekly reflection failed", "error", err)
					}
				}()
			}
		}

		return resp, nil
	}
}

// Helpers moved to agent_loop_helpers.go
