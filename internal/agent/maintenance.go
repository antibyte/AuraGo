package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/kgextraction"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/planner"
	"aurago/internal/prompts"
	"aurago/internal/security"
	"aurago/internal/services"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

// StartMaintenanceLoop spawns a background goroutine that runs daily at the configured time.
func StartMaintenanceLoop(ctx context.Context, cfg *config.Config, logger *slog.Logger, llmClient llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, contactsDB *sql.DB, plannerDB *sql.DB, cheatsheetDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, guardian *security.LLMGuardian, daemonSupervisor *tools.DaemonSupervisor) {
	startPendingMemoryWriteRetryLoop(ctx, logger, shortTermMem, longTermMem)
	if !cfg.Maintenance.Enabled {
		logger.Info("Daily maintenance is disabled in config")
		return
	}

	hour, minute, err := parseTime(cfg.Maintenance.Time)
	if err != nil {
		logger.Error("Failed to parse maintenance time, defaulting to 04:00", "error", err, "input", cfg.Maintenance.Time)
		hour, minute = 4, 0
	}

	go func() {
		logger.Info("Started System-Level Maintenance Loop", "time", fmt.Sprintf("%02d:%02d", hour, minute))
		for {
			now := time.Now()
			nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
			if now.After(nextRun) || now.Equal(nextRun) {
				nextRun = nextRun.Add(24 * time.Hour)
			}

			sleepDuration := nextRun.Sub(now)
			logger.Debug("Maintenance loop sleeping", "next_run", nextRun, "duration_hours", sleepDuration.Hours())

			select {
			case <-time.After(sleepDuration):
				runMaintenanceTask(ctx, cfg, logger, llmClient, vault, registry, manifest, cronManager, longTermMem, shortTermMem, historyMgr, kg, inventoryDB, contactsDB, plannerDB, cheatsheetDB, missionManagerV2, guardian, daemonSupervisor)
			case <-ctx.Done():
				logger.Info("Maintenance loop shutting down")
				return
			}
		}
	}()
}

func parseTime(t string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(t), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("maintenance time out of range")
	}
	return hour, minute, nil
}

func runMaintenanceTask(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, vault *security.Vault, registry *tools.ProcessRegistry, manifest *tools.Manifest, cronManager *tools.CronManager, longTermMem memory.VectorDB, shortTermMem *memory.SQLiteMemory, historyMgr *memory.HistoryManager, kg *memory.KnowledgeGraph, inventoryDB *sql.DB, contactsDB *sql.DB, plannerDB *sql.DB, cheatsheetDB *sql.DB, missionManagerV2 *tools.MissionManagerV2, guardian *security.LLMGuardian, daemonSupervisor *tools.DaemonSupervisor) {
	startedAt := time.Now()
	ledger := newMaintenanceRunLedger()
	maintenanceBatch := maintenanceSummaryKGResult{}
	ledger.beginPhase("short_term_cleanup")
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil || shortTermMem == nil {
		ledger.addError("maintenance_initialization")
		ledger.markFailed()
		completeMaintenanceRun(cfg, logger, shortTermMem, plannerDB, startedAt, ledger)
		return
	}
	timeout := time.Duration(cfg.CircuitBreaker.MaintenanceTimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	defer func() {
		ledger.recordNamedError("entity_extraction", "batched_kg_persist", maintenanceBatch.KGErr)
		completeMaintenanceRun(cfg, logger, shortTermMem, plannerDB, startedAt, ledger)
	}()

	logger.Info("[Maintenance] Waking up to perform daily tasks")
	retention := resolveMaintenanceRetention(cfg)

	ledger.recordError("stm_retention", RunSTMPRetentionMaintenance(cfg, logger, shortTermMem))

	// Phase A5: Clean up old interaction patterns
	if shortTermMem != nil {
		deleted, err := shortTermMem.CleanOldPatterns(retention.PatternsDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old patterns", "error", err)
			ledger.addError("patterns_cleanup: " + err.Error())
		} else if deleted > 0 {
			logger.Info("[Maintenance] Cleaned old interaction patterns", "deleted", deleted)
		}

		deletedEvents, err := shortTermMem.CleanOldArchiveEvents(retention.ArchiveEventsDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old archive events", "error", err)
			ledger.addError("archive_events_cleanup: " + err.Error())
		} else if deletedEvents > 0 {
			logger.Info("[Maintenance] Cleaned old archive events", "deleted", deletedEvents)
		}

		deletedMoodLog, err := shortTermMem.CleanOldMoodLog(retention.MoodLogDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old mood log entries", "error", err)
			ledger.addError("mood_log_cleanup: " + err.Error())
		} else if deletedMoodLog > 0 {
			logger.Info("[Maintenance] Cleaned old mood log entries", "deleted", deletedMoodLog)
		}

		// Stale error pattern eviction: unresolved errors older than 7 days are
		// likely no longer relevant to current conditions and would otherwise
		// bias the system prompt indefinitely. Resolved patterns are kept.
		deletedErr, err := shortTermMem.CleanOldErrorPatterns(retention.ErrorPatternsDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old error patterns", "error", err)
			ledger.addError("error_patterns_cleanup: " + err.Error())
		} else if deletedErr > 0 {
			logger.Info("[Maintenance] Cleaned stale error patterns", "deleted", deletedErr)
		}

		cleanDays := cfg.Agent.AdaptiveTools.CleanTransitionsAfterDays
		if cleanDays <= 0 {
			cleanDays = 90
		}
		deletedTrans, err := shortTermMem.CleanOldTransitions(cleanDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old tool transitions", "error", err)
			ledger.addError("tool_transitions_cleanup: " + err.Error())
		} else if deletedTrans > 0 {
			logger.Info("[Maintenance] Cleaned stale tool transitions", "deleted", deletedTrans)
		}

		// Stale learned rule eviction: rules that have not been hit and are older
		// than the error-pattern retention are unlikely to remain relevant.
		deletedLR, err := shortTermMem.CleanOldLearnedRules(0.1, retention.ErrorPatternsDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old learned rules", "error", err)
			ledger.addError("learned_rules_cleanup: " + err.Error())
		} else if deletedLR > 0 {
			logger.Info("[Maintenance] Cleaned stale learned rules", "deleted", deletedLR)
		}

		if deleted, err := runMaintenanceCompressedOutputCleanup(taskCtx, cfg, logger, shortTermMem); err != nil {
			ledger.addError("compressed_output_cleanup: " + err.Error())
		} else {
			ledger.phaseResults.CompressedDeleted = int(deleted)
			ledger.addProcessed("short_term_cleanup", int(deleted))
		}
	}
	if maintenanceContextDone(taskCtx, ledger, logger, "short_term_cleanup") {
		return
	}
	ledger.beginPhase("profile_cleanup")

	// Phase D8: Personality Engine maintenance — trait decay + journal
	if cfg.Personality.Engine && shortTermMem != nil {
		ledger.recordError("personality_maintenance", personalityMaintenance(taskCtx, cfg, shortTermMem, logger))
	}

	// User Profile cleanup: remove stale low-confidence entries
	if cfg.Personality.UserProfiling && shortTermMem != nil {
		removed, err := shortTermMem.CleanupStaleProfileEntries(retention.ProfileStaleDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean stale profile entries", "error", err)
			ledger.addError("profile_cleanup: " + err.Error())
		} else if removed > 0 {
			logger.Info("[Maintenance] Cleaned stale user profile entries", "removed", removed)
		}
	}

	if cheatsheetDB != nil {
		expired, err := tools.CheatsheetGetExpiredUnused(cheatsheetDB)
		if err != nil {
			ledger.recordError("cheatsheet_query", err)
			logger.Error("[Maintenance] Failed to find expired cheat sheets", "error", err)
		} else {
			for _, sheet := range expired {
				if err := tools.CheatsheetMarkUnused(cheatsheetDB, sheet.ID); err != nil {
					ledger.recordError("cheatsheet_update", err)
					logger.Error("[Maintenance] Failed to mark cheat sheet unused", "id", sheet.ID, "name", sheet.Name, "error", err)
					continue
				}
				if err := tools.ReindexCheatsheetInVectorDB(cheatsheetDB, longTermMem, sheet.ID); err != nil {
					ledger.recordError("cheatsheet_reindex", err)
					logger.Warn("[Maintenance] Failed to remove inactive cheat sheet from vector DB", "id", sheet.ID, "name", sheet.Name, "error", err)
				}
				if missionManagerV2 != nil {
					if err := tools.InvalidatePreparedMissionsByCheatsheet(missionManagerV2.GetPreparedDB(), missionManagerV2, sheet.ID); err != nil {
						ledger.recordError("cheatsheet_missions", err)
						logger.Warn("[Maintenance] Failed to invalidate prepared missions for expired cheat sheet", "id", sheet.ID, "name", sheet.Name, "error", err)
					}
				}
				logger.Info("[Maintenance] Marked unused agent cheat sheet", "id", sheet.ID, "name", sheet.Name)
			}
		}
	}
	if maintenanceContextDone(taskCtx, ledger, logger, "profile_cleanup") {
		return
	}
	ledger.beginPhase("memory_hygiene")
	if cfg.Consolidation.Enabled && shortTermMem != nil {
		ledger.recordError("memory_baseline", runNightlyMemoryBaselineWithContext(taskCtx, cfg, logger, shortTermMem, longTermMem))
		if maintenanceContextDone(taskCtx, ledger, logger, "memory_hygiene") {
			return
		}
	} else {
		ledger.skipPhase("memory_hygiene")
	}
	ledger.beginPhase("daily_summary")

	today := startedAt.Format("2006-01-02")
	if !cfg.Tools.Journal.Enabled || !cfg.Journal.DailySummary {
		ledger.skipPhase("daily_summary")
	} else {
		if kg != nil && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.AutoExtraction {
			maintenanceBatch = runBatchedMaintenanceSummaryAndKG(taskCtx, cfg, logger, shortTermMem, kg, today)
			ledger.recordError("daily_summary_batch", maintenanceBatch.SummaryErr)
		}
		if !maintenanceBatch.SummaryStored {
			ledger.recordError("daily_summary", generateDailySummary(taskCtx, cfg, logger, client, shortTermMem, today))
		}
	}
	if maintenanceContextDone(taskCtx, ledger, logger, "daily_summary") {
		return
	}
	ledger.beginPhase("activity_rollup")
	if shortTermMem != nil {
		_, err := shortTermMem.GenerateDailyActivityRollup(today)
		ledger.recordError("activity_rollup", err)
	}
	if maintenanceContextDone(taskCtx, ledger, logger, "activity_rollup") {
		return
	}
	ledger.beginPhase("weekly_reflection")

	if ran, err := runWeeklyReflectionJob(taskCtx, cfg, logger, client, shortTermMem, kg, longTermMem, plannerDB); err != nil {
		logger.Warn("[Memory Reflection] Weekly reflection failed during maintenance", "error", err)
		ledger.addError("weekly_reflection: " + err.Error())
	} else if ran {
		logger.Info("[Memory Reflection] Weekly reflection completed during maintenance")
	} else {
		ledger.skipPhase("weekly_reflection")
	}
	if ledger.currentPhase == "weekly_reflection" {
		ledger.finishPhase("weekly_reflection", false)
	}
	ledger.beginPhase("knowledge_graph")

	// Notes: clean up old completed notes (done for >7 days)
	if cfg.Tools.Notes.Enabled && shortTermMem != nil {
		deleted, err := shortTermMem.DeleteOldDoneNotes(retention.DoneNotesDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old done notes", "error", err)
			ledger.addError("notes_cleanup: " + err.Error())
		} else if deleted > 0 {
			logger.Info("[Maintenance] Cleaned old done notes", "deleted", deleted)
		}
	}
	if shortTermMem != nil {
		hygieneStats := runAutomaticMemoryHygiene(cfg, logger, shortTermMem, longTermMem)
		for _, err := range hygieneStats.Errors {
			ledger.recordError("memory_hygiene", err)
		}
		ledger.addDeferred("knowledge_graph", hygieneStats.Deferred)
		ledger.phaseResults.JournalRemoved = hygieneStats.JournalRemoved
		ledger.phaseResults.NotesArchived = hygieneStats.NotesArchived
		ledger.addProcessed("knowledge_graph", hygieneStats.JournalRemoved+hygieneStats.NotesArchived)
	}

	// Knowledge Graph: Garbage collection and semantic reindex
	if kg != nil {
		if _, _, err := kg.CleanupStaleGraphWithOptions(memory.KnowledgeGraphCleanupOptions{
			PendingCoMentionDays: cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays,
			StaleNodeDays:        30,
		}); err != nil {
			logger.Error("[Maintenance] Failed to clean up stale KG elements", "error", err)
			ledger.addError("kg_cleanup: " + err.Error())
		}
		if ran, err := kg.RunSemanticReindexIfDue(); err != nil {
			logger.Warn("[Maintenance] Failed to reindex semantic knowledge graph", "error", err)
			ledger.addError("kg_semantic_reindex: " + err.Error())
		} else if ran {
			if extraPasses, drainErr := kg.DrainSemanticReindexBacklog(2); drainErr != nil {
				logger.Warn("[Maintenance] Failed to drain knowledge graph semantic reindex backlog", "error", drainErr)
				ledger.addError("kg_semantic_reindex: " + drainErr.Error())
			} else if extraPasses > 0 {
				logger.Debug("[Maintenance] Knowledge graph semantic reindex follow-up passes completed", "extra_passes", extraPasses)
			}
			logger.Debug("[Maintenance] Semantic knowledge graph reindex completed")
			if backlog, dirtyNodes, dirtyEdges, backlogErr := kg.HasSemanticReindexBacklog(); backlogErr != nil {
				logger.Warn("[Maintenance] Failed to inspect knowledge graph semantic reindex backlog", "error", backlogErr)
				ledger.addError("kg_semantic_backlog: " + backlogErr.Error())
			} else if backlog {
				logger.Warn("[Maintenance] Knowledge graph semantic reindex backlog remains high", "dirty_nodes", dirtyNodes, "dirty_edges", dirtyEdges)
				recordKnowledgeGraphSemanticReindexBacklogIssue(plannerDB, dirtyNodes, dirtyEdges, logger)
			}
		}
	}

	if kg != nil && inventoryDB != nil {
		if err := kg.SyncExternalSources(inventoryDB, logger); err != nil {
			logger.Warn("[Maintenance] Inventory KG sync failed", "error", err)
			ledger.addError("inventory_kg_sync: " + err.Error())
		}
	}

	// Sync contacts and core memory
	if kg != nil {
		ledger.recordError("contacts_kg_sync", SyncContactsToKnowledgeGraph(taskCtx, contactsDB, kg, logger))
	}
	if kg != nil {
		ledger.recordError("planner_kg_sync", SyncPlannerToKnowledgeGraph(taskCtx, plannerDB, kg, logger))
	}
	if plannerDB != nil {
		if archived, err := planner.ArchiveStaleOperationalIssues(plannerDB, time.Now()); err != nil {
			logger.Warn("[Maintenance] Failed to archive stale operational issues", "error", err)
			ledger.addError("operational_issue_archive: " + err.Error())
		} else if archived > 0 {
			logger.Info("[Maintenance] Archived stale operational issues", "archived", archived)
		}
		if cleaned, err := planner.CleanupOperationalIssues(plannerDB, time.Duration(retention.OperationalIssuesDays)*24*time.Hour); err != nil {
			logger.Warn("[Maintenance] Failed to clean up operational issues", "error", err)
			ledger.addError("operational_issue_cleanup: " + err.Error())
		} else if cleaned > 0 {
			logger.Info("[Maintenance] Cleaned old operational issues", "deleted", cleaned)
		}
	}
	if kg != nil && shortTermMem != nil {
		ledger.recordError("core_kg_sync", SyncCoreMemoryToKnowledgeGraph(taskCtx, shortTermMem, kg, logger))
		recordKnowledgeGraphSparseIssue(plannerDB, shortTermMem, kg, logger)
		recordKnowledgeGraphQualityIssues(plannerDB, kg, logger)
	}
	if maintenanceContextDone(taskCtx, ledger, logger, "knowledge_graph") {
		return
	}
	ledger.beginPhase("entity_extraction")

	// Knowledge Graph: incremental file-based KG sync
	entityCtx, cancelEntity, entityBudgetAvailable := maintenanceContextWithReserve(taskCtx, maintenanceProtectedTailReserve)
	entityEnabled := kg != nil && shortTermMem != nil && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.AutoExtraction
	if !entityEnabled {
		ledger.skipPhase("entity_extraction")
	} else if !entityBudgetAvailable {
		ledger.addDeferred("entity_extraction", 1)
	} else if kg != nil && shortTermMem != nil && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.AutoExtraction {
		syncer := services.NewFileKGSyncer(cfg, logger, client, longTermMem, shortTermMem, kg)
		opts := services.FileKGSyncOptions{
			DryRun:       false,
			Backfill:     false,
			MaxFiles:     50, // conservative nightly limit for first draft
			MinRemaining: 30 * time.Second,
		}
		kgResult := syncer.SyncAllWithContext(entityCtx, opts)
		logFileKGSyncResult(logger, kgResult)
		ledger.phaseResults.KGFilesProcessed = kgResult.FilesProcessed
		ledger.phaseResults.KGNodesExtracted = kgResult.NodesExtracted
		ledger.addProcessed("entity_extraction", kgResult.FilesProcessed)
		ledger.addDeferred("entity_extraction", kgResult.FilesDeferred)
		budgetExpired := entityCtx.Err() != nil && taskCtx.Err() == nil
		for _, syncErr := range kgResult.Errors {
			if budgetExpired && isContextCancellationText(syncErr) {
				continue
			}
			ledger.addError("file_kg_sync: " + syncErr)
		}
		if budgetExpired && kgResult.FilesDeferred == 0 {
			ledger.addDeferred("entity_extraction", 1)
		}
	}

	// Knowledge Graph: nightly batch entity extraction from recent conversations
	if entityBudgetAvailable && entityCtx.Err() == nil && !maintenanceBatch.KGStored && cfg.Tools.KnowledgeGraph.Enabled && cfg.Tools.KnowledgeGraph.AutoExtraction && kg != nil && shortTermMem != nil {
		ledger.recordError("kg_extraction", extractKGEntities(entityCtx, cfg, logger, client, shortTermMem, kg))
	}
	cancelEntity()
	if maintenanceContextDone(taskCtx, ledger, logger, "entity_extraction") {
		return
	}
	ledger.beginPhase("consolidation")

	// STM→LTM Consolidation: extract knowledge from archived messages into VectorDB
	totalStored := 0
	consolidationCtx, cancelConsolidation, consolidationBudgetAvailable := maintenanceContextWithReserve(taskCtx, maintenanceProtectedTailReserve)
	consolidationEnabled := cfg.Consolidation.Enabled && shortTermMem != nil && longTermMem != nil && longTermMem.IsReady() && !longTermMem.IsDisabled()
	if !cfg.Consolidation.Enabled {
		ledger.skipPhase("consolidation")
	} else if !consolidationEnabled {
		ledger.addError("consolidation_unavailable")
	} else if !consolidationBudgetAvailable {
		ledger.addDeferred("consolidation", 1)
		ledger.addPhaseCode("consolidation", "phase_budget_exhausted")
	} else if cfg.Consolidation.Enabled && shortTermMem != nil && longTermMem != nil && longTermMem.IsReady() && !longTermMem.IsDisabled() {
		consolidationResult := consolidateSTMtoLTMWithContext(consolidationCtx, cfg, logger, client, shortTermMem, longTermMem, kg)
		for _, err := range consolidationResult.Errors {
			ledger.recordError("consolidation", err)
		}
		totalStored = consolidationResult.FactsStored
		ledger.phaseResults.ConsolidationFacts = totalStored
		ledger.phaseResults.ConsolidationExcluded = consolidationResult.MessagesExcluded
		ledger.addProcessed("consolidation", consolidationResult.MessagesConsolidated)
		if deferred := consolidationResult.MessagesClaimed - consolidationResult.MessagesConsolidated; deferred > 0 {
			ledger.addDeferred("consolidation", deferred)
		}
		ledger.recordError("episodic_hierarchy", consolidateEpisodicHierarchy(logger, shortTermMem, longTermMem, kg))
	}
	consolidationBudgetExpired := consolidationCtx.Err() != nil && taskCtx.Err() == nil
	cancelConsolidation()
	if cfg.Consolidation.Enabled && shortTermMem != nil {
		if backlog, err := shortTermMem.CountConsolidationCandidates(3); err == nil {
			ledger.phaseResults.ConsolidationBacklog = backlog
			if outstanding := backlog - ledger.phaseDeferred("consolidation"); outstanding > 0 {
				ledger.addDeferred("consolidation", outstanding)
			}
		} else {
			ledger.addError("consolidation_backlog: " + err.Error())
		}
	}
	if consolidationEnabled && consolidationBudgetExpired && ledger.phaseDeferred("consolidation") == 0 {
		ledger.addDeferred("consolidation", 1)
	}
	if consolidationEnabled && consolidationBudgetExpired {
		ledger.addPhaseCode("consolidation", "phase_budget_exhausted")
	}
	if maintenanceContextDone(taskCtx, ledger, logger, "consolidation") {
		return
	}
	ledger.beginPhase("memory_optimization")
	if cfg.Consolidation.Enabled && cfg.Consolidation.AutoOptimize && totalStored > 0 {
		if !maintenanceContextHasAtLeast(taskCtx, maintenanceOptimizationMinimum) {
			ledger.addDeferred("memory_optimization", 1)
		} else {
			memoryMaintenanceResult := runPostConsolidationMemoryOptimizationWithContext(taskCtx, cfg, logger, client, shortTermMem, longTermMem, kg, totalStored)
			for _, err := range memoryMaintenanceResult.Errors {
				ledger.recordError("memory_optimization", err)
			}
			if memoryMaintenanceResult.KGOptimizeErr != nil {
				ledger.addError("kg_optimize: " + memoryMaintenanceResult.KGOptimizeErr.Error())
			}
		}
		if maintenanceContextDone(taskCtx, ledger, logger, "memory_optimization") {
			return
		}
	} else {
		ledger.skipPhase("memory_optimization")
	}
	ledger.beginPhase("skill_quality")

	// Deterministic skill quality review runs before the free-form maintenance
	// agent. It is provenance-gated and never exposes source in the run ledger.
	skillResult := runSkillQualityMaintenance(taskCtx, cfg, logger, client, guardian, daemonSupervisor, cronManager)
	ledger.phaseResults.SkillsReviewed = skillResult.Reviewed
	ledger.phaseResults.SkillsImproved = skillResult.Improved
	ledger.phaseResults.SkillsDeleted = skillResult.Deleted
	ledger.phaseResults.SkillsReviewRequired = skillResult.ReviewRequired
	ledger.phaseResults.SkillActions = skillResult.Actions
	skillOutcome := maintenancePhaseOutcome{Skipped: skillResult.Skipped, Processed: skillResult.Reviewed}
	if skillResult.Deferred {
		skillOutcome.Deferred = 1
	}
	for range skillResult.Errors {
		skillOutcome.ErrorCodes = appendUniqueMaintenanceCode(skillOutcome.ErrorCodes, "skill_quality_failed")
	}
	ledger.applyOutcome("skill_quality", skillOutcome)
	if maintenanceContextDone(taskCtx, ledger, logger, "skill_quality") {
		return
	}

	ledger.beginPhase("prompt_load")
	// 1. Load Maintenance Prompt
	promptPath := filepath.Join(cfg.Directories.PromptsDir, "maintenance.md")
	maintenancePrompt, err := os.ReadFile(promptPath)
	if err != nil {
		logger.Error("[Maintenance] Failed to read maintenance prompt", "error", err)
		ledger.markFailed()
		ledger.addError("maintenance_prompt: " + err.Error())
		return
	}

	ledger.finishPhase("prompt_load", false)
	ledger.beginPhase("agent_loop")
	// 2. Prepare the request
	req := openai.ChatCompletionRequest{
		Model: cfg.LLM.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: string(maintenancePrompt)},
		},
	}

	sessionID := "maintenance"

	// 3. Execute reasoning loop
	if taskCtx.Err() != nil {
		maintenanceContextDone(taskCtx, ledger, logger, "agent_loop")
		return
	}

	// Use NoopBroker for silent background reasoning
	broker := &NoopBroker{}

	runCfg := RunConfig{
		Config:           cfg,
		Logger:           logger,
		LLMClient:        client,
		ShortTermMem:     shortTermMem,
		HistoryManager:   historyMgr,
		LongTermMem:      longTermMem,
		KG:               kg,
		InventoryDB:      inventoryDB,
		CheatsheetDB:     cheatsheetDB,
		Vault:            vault,
		Registry:         registry,
		Manifest:         manifest,
		CronManager:      cronManager,
		MissionManagerV2: missionManagerV2,
		CoAgentRegistry:  nil,
		BudgetTracker:    nil,
		SessionID:        sessionID,
		PlannerDB:        plannerDB,
		IsMaintenance:    true,
		MessageSource:    "maintenance",
	}

	resp, err := ExecuteAgentLoop(taskCtx, req, runCfg, false, broker)
	if err != nil {
		logger.Error("[Maintenance] Agent loop failed", "error", err)
		ledger.addError("agent_loop: " + err.Error())
		return
	}

	if len(resp.Choices) > 0 {
		logger.Info("[Maintenance] Task completed successfully", "response_len", len(resp.Choices[0].Message.Content))
	} else {
		logger.Warn("[Maintenance] Agent returned no choices")
		ledger.addError("agent_loop: no assistant choices returned")
	}
	ledger.finishPhase("agent_loop", false)
}

func maintenanceContextDone(ctx context.Context, ledger *maintenanceRunLedger, logger *slog.Logger, operation string) bool {
	if ctx == nil {
		return false
	}
	if err := ctx.Err(); err != nil {
		if logger != nil {
			logger.Warn("[Maintenance] Stopping after context cancellation", "operation", operation, "error", err)
		}
		if ledger != nil {
			activePhase := ledger.currentPhase
			ledger.addError(operation + ": " + err.Error())
			if activePhase != "" {
				if ledger.phaseDeferred(activePhase) == 0 {
					ledger.addDeferred(activePhase, 1)
				}
				ledger.finishPhase(activePhase, true)
			}
		}
		return true
	}
	if ledger != nil {
		ledger.finishPhase(operation, false)
	}
	return false
}

// personalityMaintenance performs daily trait decay and appends a character journal entry.
func personalityMaintenance(ctx context.Context, cfg *config.Config, stm *memory.SQLiteMemory, logger *slog.Logger) (resultErr error) {
	// 1. Trait decay: nudge all traits toward 0.5, respecting the personality profile's decay rate
	meta := prompts.GetCorePersonalityMeta(cfg.Directories.PromptsDir, cfg.Personality.CorePersonality)
	// Decay amount was previously 0.002 which is practically invisible (250 days to decay from 1.0 to 0.5).
	// Raised to 0.02 so traits meaningfully return toward neutral over ~25 days while still preserving
	// developed personality when interactions are frequent.
	decayAmount := 0.02 * meta.TraitDecayRate
	if err := stm.DecayAllTraitsWeighted(decayAmount, meta); err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Personality] Trait decay failed", "error", err)
	} else {
		logger.Info("[Personality] Daily weighted trait decay applied", "amount", decayAmount, "decay_rate", meta.TraitDecayRate)
	}

	if deleted, err := stm.CleanupAffectEvents(30, 200); err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Affect] Event log cleanup failed", "error", err)
	} else if deleted > 0 {
		logger.Info("[Affect] Event log cleaned up", "deleted", deleted)
	}

	// 2. Emotion history cleanup
	if cfg.Personality.EmotionSynthesizer.Enabled {
		deleted, err := stm.CleanupEmotionHistory(30, cfg.Personality.EmotionSynthesizer.MaxHistoryEntries)
		if err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Error("[EmotionSynthesizer] Emotion history cleanup failed", "error", err)
		} else if deleted > 0 {
			logger.Info("[EmotionSynthesizer] Emotion history cleaned up", "deleted", deleted)
		}
	}

	resultErr = errors.Join(resultErr, runCharacterReflection(ctx, cfg, stm, logger))

	// 3. Character journal: append today's snapshot to data/character_journal.md
	traits, err := stm.GetTraits()
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Personality] Cannot read traits for journal", "error", err)
		return
	}
	mood := stm.GetCurrentMood()
	milestones, milestoneErr := stm.GetMilestones(3)
	resultErr = errors.Join(resultErr, milestoneErr)

	journalPath := filepath.Join(cfg.Directories.DataDir, "character_journal.md")
	f, err := os.OpenFile(journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Personality] Cannot open character journal", "error", err)
		return
	}
	defer func() { resultErr = errors.Join(resultErr, f.Close()) }()

	entry := fmt.Sprintf("\n## %s\n**Mood:** %s\n**Traits:** C:%.2f T:%.2f Cr:%.2f E:%.2f Co:%.2f A:%.2f L:%.2f\n",
		time.Now().Format("2006-01-02"),
		mood,
		traits[memory.TraitCuriosity],
		traits[memory.TraitThoroughness],
		traits[memory.TraitCreativity],
		traits[memory.TraitEmpathy],
		traits[memory.TraitConfidence],
		traits[memory.TraitAffinity],
		traits[memory.TraitLoneliness],
	)
	if len(milestones) > 0 {
		entry += "**Recent Milestones:**\n"
		for _, m := range milestones {
			entry += fmt.Sprintf("- %s\n", m)
		}
	}

	if _, err := f.WriteString(entry); err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Personality] Failed to write journal entry", "error", err)
	} else {
		logger.Info("[Personality] Character journal updated")
	}
	return resultErr
}

// generateDailySummary summarizes the requested journal date.
func generateDailySummary(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, dates ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		logger.Warn("[Journal] Daily summary skipped: maintenance context canceled", "error", err)
		return err
	}
	today := time.Now().Format("2006-01-02")
	if len(dates) > 0 {
		today = dates[0]
	}

	// Check if a summary already exists for today
	if existing, err := stm.GetDailySummary(today); err != nil {
		return err
	} else if existing != nil {
		logger.Debug("[Journal] Daily summary already exists", "date", today)
		return nil
	}

	// Collect today's journal entries
	entries, err := stm.GetJournalEntries(today, today, nil, 50)
	if err != nil || len(entries) == 0 {
		logger.Debug("[Journal] No journal entries today, skipping summary", "date", today)
		if err != nil {
			return fmt.Errorf("daily summary journal entries: %w", err)
		}
		return nil
	}

	journalInput := buildDailySummaryJournalInput(entries)

	prompt := fmt.Sprintf(`Summarize the following activity log for the completed date %s in 2-3 concise sentences.
Focus on: what was accomplished, key decisions, and notable events.
Output ONLY the summary text, no JSON or formatting.

Activity log:
%s`, today, journalInput)

	summaryClient, summaryModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
	if summaryClient == nil || summaryModel == "" {
		logger.Warn("[Journal] Daily summary skipped: no helper/main LLM available")
		return fmt.Errorf("daily summary LLM unavailable")
	}

	resp, err := llm.ExecuteWithRetry(
		ctx,
		summaryClient,
		openai.ChatCompletionRequest{
			Model: summaryModel,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are a concise activity summarizer. Output ONLY 2-3 sentences."},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			MaxTokens: 300,
		},
		logger,
		nil,
	)
	if err != nil || len(resp.Choices) == 0 {
		logger.Warn("[Journal] Failed to generate daily summary via LLM", "error", err, "model", summaryModel)
		if err != nil {
			return fmt.Errorf("daily summary llm: %w", err)
		}
		return fmt.Errorf("daily summary llm returned no choices")
	}

	if resp.Choices[0].FinishReason == openai.FinishReasonLength {
		return fmt.Errorf("daily summary completion truncated")
	}
	_, err = storeDailySummaryText(stm, logger, today, entries, resp.Choices[0].Message.Content)
	return err
}

func uniqueTopics(in []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func buildDailySummaryJournalInput(entries []memory.JournalEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", e.EntryType, e.Title, e.Content))
	}
	return strings.TrimSpace(sb.String())
}

func buildKGActivityInput(turns []memory.ActivityTurn) string {
	var sb strings.Builder
	for _, turn := range turns {
		if strings.TrimSpace(turn.Intent) == "" && strings.TrimSpace(turn.UserRequest) == "" && len(turn.ImportantPoints) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("- intent=%s; request=%s; goal=%s\n", turn.Intent, turn.UserRequest, turn.UserGoal))
		if len(turn.ImportantPoints) > 0 {
			sb.WriteString(fmt.Sprintf("  important: %s\n", strings.Join(turn.ImportantPoints, " | ")))
		}
		if len(turn.Outcomes) > 0 {
			sb.WriteString(fmt.Sprintf("  outcomes: %s\n", strings.Join(turn.Outcomes, " | ")))
		}
		if len(turn.PendingItems) > 0 {
			sb.WriteString(fmt.Sprintf("  pending: %s\n", strings.Join(turn.PendingItems, " | ")))
		}
		if sb.Len() > 3000 {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

func buildKGConversationExcerpt(messages []openai.ChatCompletionMessage) string {
	var sb strings.Builder
	for _, m := range messages {
		if m.Role == "system" || strings.TrimSpace(m.Content) == "" {
			continue
		}
		content := m.Content
		if len(content) > 500 {
			content = truncateUTF8ToLimit(content, 503, "...")
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", m.Role, content))
		if sb.Len() > 8000 {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

func buildKGExtractionInput(messages []openai.ChatCompletionMessage, entries []memory.JournalEntry, turns []memory.ActivityTurn) string {
	sections := make([]string, 0, 3)

	if conversation := buildKGConversationExcerpt(messages); conversation != "" {
		sections = append(sections, "Conversation:\n"+conversation)
	}
	if activity := buildKGActivityInput(turns); activity != "" {
		sections = append(sections, "Activity turns:\n"+activity)
	}
	if journal := buildDailySummaryJournalInput(entries); journal != "" {
		sections = append(sections, "Journal entries:\n"+journal)
	}

	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func storeDailySummaryText(stm *memory.SQLiteMemory, logger *slog.Logger, today string, entries []memory.JournalEntry, summaryText string) (bool, error) {
	summaryText = strings.TrimSpace(summaryText)
	if stm == nil || summaryText == "" {
		return false, fmt.Errorf("daily summary is empty or store unavailable")
	}

	summary := memory.DailySummary{
		Date:      today,
		Summary:   summaryText,
		Sentiment: "neutral",
	}

	toolUsage := make(map[string]int)
	var topics []string
	for _, e := range entries {
		for _, tag := range e.Tags {
			toolUsage[tag]++
		}
		if e.EntryType != "" {
			topics = append(topics, e.EntryType)
		}
	}
	summary.ToolUsage = toolUsage
	summary.KeyTopics = uniqueTopics(topics)

	inserted, err := stm.InsertMaintenanceDailySummary(summary)
	if err != nil {
		return false, err
	}
	if !inserted {
		return true, nil
	}

	logger.Info("[Journal] Daily summary stored", "date", today)
	anchor := summary.Summary
	if idx := strings.Index(anchor, "."); idx > 20 {
		anchor = strings.TrimSpace(anchor[:idx+1])
	}
	if len(anchor) > 220 {
		anchor = strings.TrimSpace(anchor[:220]) + "..."
	}
	if err := stm.UpsertDayAnchor(today, anchor); err != nil {
		logger.Warn("[Journal] Failed to store day anchor", "error", err)
		return true, err
	}
	return true, nil
}

func storeKGExtraction(logger *slog.Logger, kg *memory.KnowledgeGraph, nodes []memory.Node, edges []memory.Edge, contentLength int) error {
	if kg == nil {
		return nil
	}
	if len(nodes) == 0 && len(edges) == 0 {
		logger.Debug("[KG] No entities extracted")
		return nil
	}

	// Compute extraction confidence based on heuristics.
	confidenceScore := kgextraction.ComputeConfidence(kgextraction.ConfidenceInput{
		SourceType:    "auto_extraction",
		ContentLength: contentLength,
		NodeCount:     len(nodes),
		EdgeCount:     len(edges),
	})
	confidenceStr := kgextraction.FormatConfidence(confidenceScore)

	for i := range nodes {
		if nodes[i].Properties == nil {
			nodes[i].Properties = make(map[string]string)
		}
		nodes[i].Properties["source"] = "auto_extraction"
		nodes[i].Properties["extracted_at"] = time.Now().Format("2006-01-02")
		nodes[i].Properties["confidence"] = confidenceStr
	}
	for i := range edges {
		if edges[i].Properties == nil {
			edges[i].Properties = make(map[string]string)
		}
		edges[i].Properties["source"] = "auto_extraction"
		edges[i].Properties["extracted_at"] = time.Now().Format("2006-01-02")
		edges[i].Properties["confidence"] = confidenceStr
	}

	if err := kg.BulkMergeExtractedEntities(nodes, edges); err != nil {
		logger.Error("[KG] Failed to bulk-add extracted entities", "error", err)
		return err
	}

	logger.Info("[KG] Nightly entity extraction complete", "nodes", len(nodes), "edges", len(edges), "confidence", confidenceStr)
	return nil
}

func runBatchedMaintenanceSummaryAndKG(ctx context.Context, cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph, today string) maintenanceSummaryKGResult {
	var outcome maintenanceSummaryKGResult
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		logger.Warn("[HelperLLM] Maintenance summary/KG batch skipped: maintenance context canceled", "error", err)
		outcome.SummaryErr, outcome.KGErr = err, err
		return outcome
	}
	helperManager := newHelperLLMManager(cfg, logger)
	if helperManager == nil || stm == nil || kg == nil {
		return outcome
	}

	if existing, err := stm.GetDailySummary(today); err != nil {
		outcome.SummaryErr = err
		return outcome
	} else if existing != nil {
		return outcome
	}

	entries, err := stm.GetJournalEntries(today, today, nil, 50)
	if err != nil || len(entries) == 0 {
		outcome.SummaryErr = err
		return outcome
	}
	messages, err := stm.GetRecentMessagesAcrossSessions(100)
	if err != nil || len(messages) == 0 {
		outcome.KGErr = err
		return outcome
	}
	turns, err := stm.GetActivityTurnsForDate(today, 20)
	if err != nil {
		outcome.KGErr = err
		return outcome
	}

	journalInput := buildDailySummaryJournalInput(entries)
	conversationInput := buildKGExtractionInput(messages, entries, turns)
	if journalInput == "" || len(conversationInput) < 50 {
		return outcome
	}

	existingNodesString := ""
	if existingNodes, err := kg.GetAllNodes(150); err != nil {
		outcome.KGErr = err
		return outcome
	} else if len(existingNodes) > 0 {
		var contexts []string
		for _, n := range existingNodes {
			contexts = append(contexts, fmt.Sprintf("- ID: %s, Label: %s", n.ID, n.Label))
		}
		existingNodesString = strings.Join(contexts, "\n")
	}

	batchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	result, err := helperManager.AnalyzeMaintenanceSummaryAndKG(batchCtx, today, journalInput, conversationInput, existingNodesString)
	if err != nil {
		helperManager.ObserveFallback("maintenance_summary_kg", err.Error())
		logger.Warn("[HelperLLM] Maintenance summary/KG batch failed, falling back", "error", err)
		outcome.SummaryErr, outcome.KGErr = err, err
		return outcome
	}
	return persistMaintenanceSummaryAndKG(stm, kg, logger, today, entries, result, len(conversationInput))
}

// extractKGEntities performs nightly batch entity extraction from the past 24h of messages.
// Uses an LLM call to extract entities and relationships, then bulk-adds to the knowledge graph.
// This is a conversation-specific adapter around ExtractKGFromText.
func extractKGEntities(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		logger.Warn("[KG] Entity extraction skipped: maintenance context canceled", "error", err)
		return err
	}
	today := time.Now().Format("2006-01-02")

	// Collect recent messages across all sessions.
	messages, err := stm.GetRecentMessagesAcrossSessions(100)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		logger.Debug("[KG] No recent messages for entity extraction")
		return nil
	}
	entries, err := stm.GetJournalEntries(today, today, nil, 30)
	if err != nil {
		return err
	}
	turns, err := stm.GetActivityTurnsForDate(today, 20)
	if err != nil {
		return err
	}

	conversationExcerpt := buildKGExtractionInput(messages, entries, turns)
	if len(conversationExcerpt) < 50 {
		logger.Debug("[KG] Not enough conversation content for entity extraction")
		return nil
	}

	existingNodesString := ""
	if existingNodes, err := kg.GetAllNodes(150); err != nil {
		return fmt.Errorf("read existing KG extraction nodes: %w", err)
	} else if len(existingNodes) > 0 {
		var contexts []string
		for _, n := range existingNodes {
			contexts = append(contexts, fmt.Sprintf("- ID: %s, Label: %s", n.ID, n.Label))
		}
		existingNodesString = "Existing Nodes (reuse IDs if possible):\n" + strings.Join(contexts, "\n") + "\n\n"
	}

	nodes, edges, err := kgextraction.ExtractKGFromTextWithContext(ctx, cfg, logger, client, conversationExcerpt, existingNodesString)
	if err != nil {
		logger.Warn("[KG] Entity extraction failed", "error", err)
		return err
	}

	return storeKGExtraction(logger, kg, nodes, edges, len(conversationExcerpt))
}

const helperConsolidationBatchSize = 2

type consolidationWorkItem struct {
	batchID      string
	messages     []memory.ArchivedMessage
	messageIDs   []int64
	conversation string
}

func buildConsolidationWorkItem(index int, batch []memory.ArchivedMessage) consolidationWorkItem {
	var sb strings.Builder
	messageIDs := make([]int64, 0, len(batch))
	for _, msg := range batch {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", msg.Timestamp, msg.Role, msg.Content))
		messageIDs = append(messageIDs, msg.ID)
	}
	return consolidationWorkItem{
		batchID:      fmt.Sprintf("batch_%d", index+1),
		messages:     batch,
		messageIDs:   messageIDs,
		conversation: strings.TrimSpace(sb.String()),
	}
}

func extractConsolidationFactsWithLLM(ctx context.Context, logger *slog.Logger, client llm.ChatClient, model, conversation string) ([]helperConsolidationFact, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf(`Analyze the following conversation excerpt and extract the most important knowledge.
Return ONLY valid JSON with this exact structure:
{
  "facts": [
    {"concept": "Short topic title", "content": "Detailed factual information extracted"}
  ]
}

Rules:
- Extract concrete facts, decisions, user preferences, technical details, and actionable knowledge
- Each fact should be self-contained and understandable without the original conversation
- Concept should be a brief 2-5 word topic label
- Content should preserve specific details: names, versions, paths, commands, configurations
- Skip generic pleasantries, acknowledgments, and obvious context
- Maximum 10 facts per batch
- If no meaningful facts exist, return {"facts": []}

Conversation:
%s`, conversation)

	extractCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := llm.ExecuteWithRetry(
		extractCtx,
		client,
		openai.ChatCompletionRequest{
			Model: model,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are a knowledge extraction engine. Extract factual knowledge from conversations. Output ONLY valid JSON, no markdown fences."},
				{Role: openai.ChatMessageRoleUser, Content: prompt},
			},
			MaxTokens: 1000,
			ResponseFormat: func() *openai.ChatCompletionResponseFormat {
				if caps, ok := llm.CapabilitiesFromRegistry("", model); ok {
					return llm.JSONResponseFormat(caps.StructuredOutputs)
				}
				return nil
			}(),
		},
		logger,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("llm extraction failed: %w", err)
	}
	raw, err := llm.JSONContentFromResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("json completion failed: %w", err)
	}

	var extracted struct {
		Facts []helperConsolidationFact `json:"facts"`
	}
	if err := json.Unmarshal([]byte(raw), &extracted); err != nil {
		return nil, fmt.Errorf("json parse failed: %w", err)
	}
	return extracted.Facts, nil
}

func countValidConsolidationFacts(facts []helperConsolidationFact) int {
	count := 0
	for _, fact := range facts {
		if strings.TrimSpace(fact.Concept) != "" && strings.TrimSpace(fact.Content) != "" {
			count++
		}
	}
	return count
}

func shouldMarkConsolidationSuccess(stored, skipped, factCount, validFacts int) (bool, string) {
	if factCount == 0 {
		return false, "no_facts_extracted"
	}
	if stored > 0 {
		return true, ""
	}
	if validFacts > 0 && skipped == validFacts {
		return true, "all_duplicates"
	}
	return false, "no_facts_stored"
}

func storeConsolidationFacts(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, facts []helperConsolidationFact) (stored int, skipped int, err error) {
	var createdIDs []string
	var storeErrors []error
	for _, fact := range facts {
		concept, content := strings.TrimSpace(fact.Concept), strings.TrimSpace(fact.Content)
		if concept == "" || content == "" {
			continue
		}
		owned, storeErr := memory.StoreDocumentWithOwnership(ltm, concept, content)
		createdIDs = append(createdIDs, owned.CreatedIDs...)
		if storeErr != nil {
			storeErrors = append(storeErrors, storeErr)
			continue
		}
		if len(owned.CreatedIDs) == 0 && len(owned.UnknownIDs) == 0 {
			skipped++
			continue
		}
		for _, id := range owned.CreatedIDs {
			if err := stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
				ExtractionConfidence: 0.82, VerificationStatus: "unverified",
				SourceType: "consolidation", SourceReliability: 0.82,
			}); err != nil {
				storeErrors = append(storeErrors, err)
			}
		}
		// Legacy backends cannot establish ownership. Only insert missing metadata;
		// never reset an existing record's provenance, verification, or protection.
		for _, id := range owned.UnknownIDs {
			if err := stm.EnsureMemoryMeta(id); err != nil {
				storeErrors = append(storeErrors, err)
			}
		}
		if err := detectMemoryConflictsForDocIDs(logger, stm, ltm, owned.CreatedIDs, content); err != nil {
			storeErrors = append(storeErrors, err)
		}
		stored++
	}
	if len(storeErrors) > 0 {
		rollbackErr := rollbackStoredConsolidationFacts(logger, stm, ltm, createdIDs)
		return 0, skipped, errors.Join(append(storeErrors, rollbackErr)...)
	}
	return stored, skipped, nil
}

func rollbackStoredConsolidationFacts(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, docIDs []string) error {
	var resultErr error
	for _, docID := range docIDs {
		if ltm == nil {
			continue
		}
		if err := ltm.DeleteDocument(docID); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Warn("[Consolidation] Failed to rollback created fact", "doc_id", docID, "error", err)
			continue
		}
		if stm != nil {
			resultErr = errors.Join(resultErr, stm.DeleteDocumentCleanup(docID))
		}
	}
	return resultErr
}

func finalizeConsolidationBatch(
	logger *slog.Logger,
	stm *memory.SQLiteMemory,
	item consolidationWorkItem,
	facts []helperConsolidationFact,
	stored, skipped int,
	storeErr error,
	batchIndex, batchTotal int,
) (success bool, storedCount int, resultErr error) {
	if storeErr != nil {
		return false, 0, storeErr
	}
	validFacts := countValidConsolidationFacts(facts)
	ok, reason := shouldMarkConsolidationSuccess(stored, skipped, len(facts), validFacts)
	if !ok {
		logger.Warn("[Consolidation] Batch not consolidated", "batch", batchIndex, "reason", reason, "stored", stored, "skipped", skipped, "facts", len(facts), "valid_facts", validFacts)
		if err := stm.MarkConsolidationFailure(item.messageIDs, reason); err != nil {
			return false, 0, fmt.Errorf("mark consolidation failure: %w", err)
		}
		return false, 0, nil
	}
	if reason == "all_duplicates" {
		logger.Info("[Consolidation] Batch consolidated with duplicate-only facts", "batch", batchIndex, "skipped", skipped, "facts", len(facts), "valid_facts", validFacts)
	}
	if err := stm.MarkConsolidationSuccess(item.messageIDs); err != nil {
		logger.Error("[Consolidation] Failed to mark batch as consolidated", "batch", batchIndex, "error", err)
		failureErr := stm.MarkConsolidationFailure(item.messageIDs, fmt.Sprintf("mark_success_failed: %v", err))
		return false, 0, errors.Join(fmt.Errorf("mark consolidation success: %w", err), failureErr)
	}
	if err := recordConsolidationBatchEpisode(stm, item.messages, stored, skipped, len(facts), batchIndex, batchTotal); err != nil {
		logger.Warn("[Consolidation] Failed to record batch episode", "batch", batchIndex, "error", err)
		return true, stored, fmt.Errorf("record consolidation batch episode: %w", err)
	}
	return true, stored, nil
}

func recordConsolidationBatchEpisode(stm *memory.SQLiteMemory, batch []memory.ArchivedMessage, stored, skipped, factsCount, batchIndex, batchTotal int) error {
	if stm == nil || len(batch) == 0 {
		return nil
	}
	eventDate := time.Now().Format("2006-01-02")
	if len(batch[0].Timestamp) >= 10 {
		eventDate = batch[0].Timestamp[:10]
	}
	episodeTitle := "Consolidated conversation batch"
	episodeSummary := fmt.Sprintf("%d messages, %d facts extracted, %d stored, %d skipped", len(batch), factsCount, stored, skipped)
	episodeDetails := map[string]string{
		"session_id": batch[0].SessionID,
		"batch":      fmt.Sprintf("%d/%d", batchIndex, batchTotal),
	}
	return stm.InsertEpisodicMemoryWithDetails(eventDate, episodeTitle, episodeSummary, episodeDetails, 2, "consolidation", memory.EpisodicMemoryDetails{
		SessionID:        batch[0].SessionID,
		HierarchyLevel:   1,
		Participants:     []string{"user", "agent"},
		EmotionalValence: 0,
	})
}

func logFileKGSyncResult(logger *slog.Logger, result services.FileKGSyncResult) {
	if logger == nil {
		return
	}
	if len(result.Errors) > 0 {
		logger.Warn("[Maintenance] File KG sync completed with errors",
			"processed", result.FilesProcessed,
			"skipped", result.FilesSkipped,
			"nodes", result.NodesExtracted,
			"edges", result.EdgesExtracted,
			"error_count", len(result.Errors),
			"errors", result.Errors)
		return
	}
	if result.FilesProcessed > 0 || result.NodesExtracted > 0 || result.EdgesExtracted > 0 {
		logger.Info("[Maintenance] File KG sync complete",
			"processed", result.FilesProcessed,
			"skipped", result.FilesSkipped,
			"nodes", result.NodesExtracted,
			"edges", result.EdgesExtracted)
		return
	}
	logger.Debug("[Maintenance] File KG sync: nothing to process")
}

func runMaintenanceCompressedOutputCleanup(ctx context.Context, cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory) (int64, error) {
	if cfg == nil || stm == nil || !cfg.Agent.OutputCompression.Reversible.Enabled {
		return 0, nil
	}
	maxAge := time.Duration(cfg.Agent.OutputCompression.Reversible.MaxAgeHours) * time.Hour
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	deleted, err := stm.CleanupCompressedOutputs(ctx, maxAge)
	if err != nil {
		logger.Error("[Maintenance] Failed to clean compressed tool outputs", "error", err)
		return 0, err
	}
	if deleted > 0 {
		logger.Info("[Maintenance] Cleaned compressed tool outputs", "deleted", deleted)
	}
	return deleted, nil
}

type maintenanceRetentionDays struct {
	PatternsDays          int
	ArchiveEventsDays     int
	MoodLogDays           int
	ErrorPatternsDays     int
	ProfileStaleDays      int
	DoneNotesDays         int
	OperationalIssuesDays int
}

func resolveMaintenanceRetention(cfg *config.Config) maintenanceRetentionDays {
	defaults := maintenanceRetentionDays{
		PatternsDays:          90,
		ArchiveEventsDays:     90,
		MoodLogDays:           30,
		ErrorPatternsDays:     7,
		ProfileStaleDays:      30,
		DoneNotesDays:         7,
		OperationalIssuesDays: 30,
	}
	if cfg == nil {
		return defaults
	}
	retention := cfg.Maintenance.Retention
	if retention.PatternsDays > 0 {
		defaults.PatternsDays = retention.PatternsDays
	}
	if retention.ArchiveEventsDays > 0 {
		defaults.ArchiveEventsDays = retention.ArchiveEventsDays
	}
	if retention.MoodLogDays > 0 {
		defaults.MoodLogDays = retention.MoodLogDays
	}
	if retention.ErrorPatternsDays > 0 {
		defaults.ErrorPatternsDays = retention.ErrorPatternsDays
	}
	if retention.ProfileStaleDays > 0 {
		defaults.ProfileStaleDays = retention.ProfileStaleDays
	}
	if retention.DoneNotesDays > 0 {
		defaults.DoneNotesDays = retention.DoneNotesDays
	}
	if retention.OperationalIssuesDays > 0 {
		defaults.OperationalIssuesDays = retention.OperationalIssuesDays
	}
	return defaults
}

const nightlyMemoryMetaFetchLimit = 50000
const nightlyMemoryConflictScanLimit = 250

const maintenanceProtectedTailReserve = 90 * time.Second
const maintenanceOptimizationMinimum = 30 * time.Second

type nightlyMemoryMaintenanceResult struct {
	Errors        []error
	KGOptimizeErr error
}

func runNightlyMemoryMaintenance(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph, totalStored int) nightlyMemoryMaintenanceResult {
	return runNightlyMemoryMaintenanceWithContext(context.Background(), cfg, logger, client, stm, ltm, kg, totalStored)
}

func runNightlyMemoryMaintenanceWithContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph, totalStored int) (result nightlyMemoryMaintenanceResult) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		logger.Warn("[Maintenance] Nightly memory maintenance skipped: maintenance context canceled", "error", err)
		return result
	}
	if stm == nil {
		return result
	}
	metas, err := loadNightlyMemoryMeta(ctx, cfg, logger, stm, ltm)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	if cfg != nil && cfg.Consolidation.AutoOptimize && totalStored > 0 && ctx.Err() == nil {
		optimizeResult := autoOptimizeMemoryWithContext(ctx, cfg, logger, client, ltm, stm, kg, metas)
		result.KGOptimizeErr = optimizeResult.KGOptimizeErr
		result.Errors = append(result.Errors, optimizeResult.Errors...)
	}
	if err := runNightlyMemoryHygieneWithContext(ctx, cfg, logger, stm, ltm, metas); err != nil {
		result.Errors = append(result.Errors, err)
	}
	return result
}

// runNightlyMemoryBaselineWithContext performs the bounded deterministic memory
// work before LLM-heavy maintenance phases can consume the run deadline.
func runNightlyMemoryBaselineWithContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil || stm == nil {
		return ctx.Err()
	}
	metas, err := loadNightlyMemoryMeta(ctx, cfg, logger, stm, ltm)
	if err != nil {
		return err
	}
	return runNightlyMemoryHygieneWithContext(ctx, cfg, logger, stm, ltm, metas)
}

func loadNightlyMemoryMeta(ctx context.Context, cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB) ([]memory.MemoryMeta, error) {
	var resultErr error
	if stm == nil || ctx.Err() != nil {
		return nil, ctx.Err()
	}
	metas, err := stm.GetAllMemoryMeta(nightlyMemoryMetaFetchLimit, 0)
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Warn("[Maintenance] Failed to fetch memory metadata for nightly memory maintenance", "error", err)
	}
	if cfg != nil && cfg.Consolidation.MemoryMetaBudget > 0 && ltm != nil {
		if evicted, err := stm.ApplyMemoryBudgetEnforcement(cfg.Consolidation.MemoryMetaBudget, ltm); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Warn("[Maintenance] Memory meta budget enforcement failed", "error", err, "budget", cfg.Consolidation.MemoryMetaBudget)
		} else if evicted > 0 {
			logger.Info("[Maintenance] Memory meta budget enforced", "evicted", evicted, "budget", cfg.Consolidation.MemoryMetaBudget)
			InvalidateMemoryMetaCache()
			refreshed, refreshErr := stm.GetAllMemoryMeta(nightlyMemoryMetaFetchLimit, 0)
			if refreshErr != nil {
				resultErr = errors.Join(resultErr, refreshErr)
				logger.Warn("[Maintenance] Failed to refresh memory metadata after budget enforcement", "error", refreshErr)
				metas = nil
			} else {
				metas = refreshed
			}
		}
	}
	return metas, resultErr
}

func runNightlyMemoryHygieneWithContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, metas []memory.MemoryMeta) error {
	if err := ctx.Err(); err != nil {
		logger.Warn("[Maintenance] Stopping nightly memory maintenance: maintenance context canceled", "error", err)
		return err
	}
	curationErr := autoCurateMemory(cfg, logger, stm, metas)
	if err := ctx.Err(); err != nil {
		logger.Warn("[Maintenance] Stopping memory conflict scan: maintenance context canceled", "error", err)
		return err
	}
	return errors.Join(curationErr, detectMemoryConflictsAcrossLTMWithContext(ctx, logger, stm, ltm, metas))
}

func runPostConsolidationMemoryOptimizationWithContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph, totalStored int) (result nightlyMemoryMaintenanceResult) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil || cfg == nil || stm == nil || !cfg.Consolidation.AutoOptimize || totalStored <= 0 {
		return result
	}
	metas, err := stm.GetAllMemoryMeta(nightlyMemoryMetaFetchLimit, 0)
	if err != nil {
		logger.Warn("[Maintenance] Failed to fetch memory metadata for post-consolidation optimization", "error", err)
		result.Errors = append(result.Errors, err)
		return result
	}
	optimizeResult := autoOptimizeMemoryWithContext(ctx, cfg, logger, client, ltm, stm, kg, metas)
	result.KGOptimizeErr = optimizeResult.KGOptimizeErr
	result.Errors = append(result.Errors, optimizeResult.Errors...)
	return result
}

func runPostConsolidationMemoryMaintenance(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph, totalStored int) {
	runNightlyMemoryMaintenance(cfg, logger, client, stm, ltm, kg, totalStored)
}

func cleanConsolidationArchivedMessages(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory) error {
	if cfg == nil || stm == nil || cfg.Consolidation.ArchiveRetainDays <= 0 {
		return nil
	}
	cleaned, err := stm.CleanOldArchivedMessages(cfg.Consolidation.ArchiveRetainDays)
	if err != nil {
		logger.Error("[Consolidation] Failed to clean old archived messages", "error", err)
		return err
	}
	if cleaned > 0 {
		logger.Info("[Consolidation] Cleaned old archived messages", "deleted", cleaned)
	}
	return nil
}

// consolidateSTMtoLTM extracts knowledge from archived STM messages and stores it in the VectorDB.
// This bridges the gap between the sliding-window short-term memory and the persistent long-term memory.
func consolidateSTMtoLTM(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph) (totalStored int, messagesConsolidated int) {
	result := consolidateSTMtoLTMWithContext(context.Background(), cfg, logger, client, stm, ltm, kg)
	return result.FactsStored, result.MessagesConsolidated
}

type consolidationRunBudget struct {
	remaining int
}

type consolidationRunBudgetContextKey struct{}

type consolidationRunResult struct {
	Errors               []error
	FactsStored          int
	MessagesConsolidated int
	MessagesClaimed      int
	MessagesExcluded     int
}

func consolidateSTMtoLTMWithContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph) (result consolidationRunResult) {
	if ctx == nil {
		ctx = context.Background()
	}
	budget, _ := ctx.Value(consolidationRunBudgetContextKey{}).(*consolidationRunBudget)
	rootClaim := budget == nil
	if rootClaim {
		maxMessages := cfg.Consolidation.MaxBatchMessages
		if maxMessages <= 0 {
			maxMessages = 200
		}
		budget = &consolidationRunBudget{remaining: maxMessages}
		ctx = context.WithValue(ctx, consolidationRunBudgetContextKey{}, budget)
		defer func() {
			if err := cleanConsolidationArchivedMessages(cfg, logger, stm); err != nil {
				result.Errors = append(result.Errors, err)
			}
		}()
		excluded, excludeErr := stm.FinalizeIneligibleConsolidationCandidates()
		if excludeErr != nil {
			result.Errors = append(result.Errors, excludeErr)
			logger.Warn("[Consolidation] Failed to exclude internal archive rows", "error", excludeErr)
		} else if excluded > 0 {
			result.MessagesExcluded = int(excluded)
			logger.Info("[Consolidation] Excluded internal archive rows", "count", excluded)
		}
	}
	if err := ctx.Err(); err != nil {
		result.Errors = append(result.Errors, err)
		logger.Warn("[Consolidation] STM->LTM consolidation skipped: maintenance context canceled", "error", err)
		return result
	}
	if !maintenanceContextHasAtLeast(ctx, 75*time.Second) {
		logger.Info("[Consolidation] STM->LTM consolidation deferred: insufficient maintenance time remains")
		return result
	}

	consolidationClient, consolidationModel := resolveHelperBackedLLM(cfg, client, resolveConsolidationModel(cfg))
	if consolidationClient == nil || consolidationModel == "" {
		logger.Warn("[Consolidation] STM->LTM consolidation skipped: no helper/main LLM available")
		return result
	}

	if rootClaim {
		if reclaimed, reclaimErr := stm.ReclaimStaleConsolidationClaims(30 * time.Minute); reclaimErr != nil {
			result.Errors = append(result.Errors, reclaimErr)
			logger.Warn("[Consolidation] Failed to reclaim stale in_progress rows", "error", reclaimErr)
		} else if reclaimed > 0 {
			logger.Info("[Consolidation] Reclaimed stale in_progress rows", "count", reclaimed)
		}
	}

	// Atomically claim rows so concurrent runs cannot process the same messages.
	claimLimit := budget.remaining
	if claimLimit > 40 {
		claimLimit = 40
	}
	if claimLimit <= 0 {
		return result
	}
	archived, err := stm.ClaimConsolidationCandidates(claimLimit, 3)
	if err != nil {
		result.Errors = append(result.Errors, err)
		logger.Error("[Consolidation] Failed to fetch unconsolidated messages", "error", err)
		return result
	}
	if len(archived) == 0 {
		logger.Debug("[Consolidation] No unconsolidated archived messages")
		return result
	}
	budget.remaining -= len(archived)
	result.MessagesClaimed += len(archived)

	logger.Info("[Consolidation] Starting STM→LTM consolidation", "messages", len(archived))

	// Group messages into batches of ~4000 characters for LLM processing
	const maxBatchChars = 4000
	var batches [][]memory.ArchivedMessage
	var currentBatch []memory.ArchivedMessage
	currentLen := 0

	for _, msg := range archived {
		msgLen := len(msg.Content)
		if currentLen+msgLen > maxBatchChars && len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
			currentBatch = nil
			currentLen = 0
		}
		currentBatch = append(currentBatch, msg)
		currentLen += msgLen
	}
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	helperManager := newHelperLLMManager(cfg, logger)
	workItems := make([]consolidationWorkItem, 0, len(batches))
	for i, batch := range batches {
		workItems = append(workItems, buildConsolidationWorkItem(i, batch))
	}

	processWorkItem := func(item consolidationWorkItem, batchIndex int) {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			logger.Warn("[Consolidation] Batch skipped: maintenance context canceled", "batch", batchIndex, "error", err)
			if releaseErr := stm.ReleaseConsolidationClaims(item.messageIDs); releaseErr != nil {
				result.Errors = append(result.Errors, releaseErr)
			}
			return
		}
		facts, err := extractConsolidationFactsWithLLM(ctx, logger, consolidationClient, consolidationModel, item.conversation)
		if err != nil {
			result.Errors = append(result.Errors, err)
			logger.Warn("[Consolidation] LLM extraction failed for batch", "batch", batchIndex, "error", err)
			if markErr := stm.MarkConsolidationFailure(item.messageIDs, err.Error()); markErr != nil {
				result.Errors = append(result.Errors, markErr)
			}
			return
		}
		stored, skipped, storeErr := storeConsolidationFacts(logger, stm, ltm, facts)
		if storeErr != nil {
			result.Errors = append(result.Errors, storeErr)
			logger.Warn("[Consolidation] LTM storage failed for batch", "batch", batchIndex, "error", storeErr)
			if markErr := stm.MarkConsolidationFailure(item.messageIDs, storeErr.Error()); markErr != nil {
				result.Errors = append(result.Errors, markErr)
			}
			return
		}
		ok, storedCount, finalizeErr := finalizeConsolidationBatch(logger, stm, item, facts, stored, skipped, nil, batchIndex, len(workItems))
		if finalizeErr != nil {
			result.Errors = append(result.Errors, finalizeErr)
		}
		if ok {
			result.FactsStored += storedCount
			result.MessagesConsolidated += len(item.messageIDs)
		}
	}

	for i := 0; i < len(workItems); {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			logger.Warn("[Consolidation] Stopping STM->LTM consolidation: maintenance context canceled", "error", err)
			for _, item := range workItems[i:] {
				if releaseErr := stm.ReleaseConsolidationClaims(item.messageIDs); releaseErr != nil {
					result.Errors = append(result.Errors, releaseErr)
				}
			}
			break
		}
		if helperManager == nil {
			processWorkItem(workItems[i], i+1)
			i++
			continue
		}

		end := i + helperConsolidationBatchSize
		if end > len(workItems) {
			end = len(workItems)
		}

		inputs := make([]helperConsolidationBatchInput, 0, end-i)
		group := workItems[i:end]
		for _, item := range group {
			inputs = append(inputs, helperConsolidationBatchInput{
				BatchID:      item.batchID,
				Conversation: item.conversation,
			})
		}

		consolidationCtx, consolidationCancel := context.WithTimeout(ctx, 60*time.Second)
		batchAnalysis, err := helperManager.AnalyzeConsolidationBatches(consolidationCtx, inputs)
		consolidationCancel()
		if err != nil {
			helperManager.ObserveFallback("consolidation_batches", err.Error())
			logger.Warn("[HelperLLM] Consolidation batch failed, splitting into single batches", "start_batch", i+1, "error", err)
			for offset, item := range group {
				if ctx.Err() != nil {
					result.Errors = append(result.Errors, ctx.Err())
					if releaseErr := stm.ReleaseConsolidationClaims(item.messageIDs); releaseErr != nil {
						result.Errors = append(result.Errors, releaseErr)
					}
					continue
				}
				singleCtx, singleCancel := context.WithTimeout(ctx, 45*time.Second)
				singleResult, singleErr := helperManager.AnalyzeConsolidationBatches(singleCtx, []helperConsolidationBatchInput{{
					BatchID: item.batchID, Conversation: item.conversation,
				}})
				singleCancel()
				if singleErr != nil {
					result.Errors = append(result.Errors, singleErr)
					if markErr := stm.MarkConsolidationFailure(item.messageIDs, singleErr.Error()); markErr != nil {
						result.Errors = append(result.Errors, markErr)
					}
					continue
				}
				facts := singleResult.Batches[0].Facts
				stored, skipped, storeErr := storeConsolidationFacts(logger, stm, ltm, facts)
				if storeErr != nil {
					result.Errors = append(result.Errors, storeErr)
					if markErr := stm.MarkConsolidationFailure(item.messageIDs, storeErr.Error()); markErr != nil {
						result.Errors = append(result.Errors, markErr)
					}
					continue
				}
				ok, storedCount, finalizeErr := finalizeConsolidationBatch(logger, stm, item, facts, stored, skipped, nil, i+offset+1, len(workItems))
				if finalizeErr != nil {
					result.Errors = append(result.Errors, finalizeErr)
				}
				if ok {
					result.FactsStored += storedCount
					result.MessagesConsolidated += len(item.messageIDs)
				}
			}
			i = end
			continue
		}

		byID := make(map[string][]helperConsolidationFact, len(batchAnalysis.Batches))
		for _, batchResult := range batchAnalysis.Batches {
			byID[batchResult.BatchID] = batchResult.Facts
		}
		for offset, item := range group {
			facts := byID[item.batchID]
			stored, skipped, storeErr := storeConsolidationFacts(logger, stm, ltm, facts)
			if storeErr != nil {
				result.Errors = append(result.Errors, storeErr)
				logger.Warn("[Consolidation] LTM storage failed for helper batch", "batch_id", item.batchID, "error", storeErr)
				if markErr := stm.MarkConsolidationFailure(item.messageIDs, storeErr.Error()); markErr != nil {
					result.Errors = append(result.Errors, markErr)
				}
				continue
			}
			ok, storedCount, finalizeErr := finalizeConsolidationBatch(logger, stm, item, facts, stored, skipped, nil, i+offset+1, len(workItems))
			if finalizeErr != nil {
				result.Errors = append(result.Errors, finalizeErr)
			}
			if ok {
				result.FactsStored += storedCount
				result.MessagesConsolidated += len(item.messageIDs)
			}
		}
		i = end
	}

	if budget.remaining > 0 && len(archived) == claimLimit && maintenanceContextHasAtLeast(ctx, 75*time.Second) {
		more := consolidateSTMtoLTMWithContext(ctx, cfg, logger, client, stm, ltm, kg)
		result.FactsStored += more.FactsStored
		result.MessagesConsolidated += more.MessagesConsolidated
		result.MessagesClaimed += more.MessagesClaimed
		result.Errors = append(result.Errors, more.Errors...)
	}

	// Create one journal entry for the complete consolidation run.
	if rootClaim && cfg.Tools.Journal.Enabled && result.FactsStored > 0 {
		if _, err := stm.InsertJournalEntry(memory.JournalEntry{
			EntryType: "system",
			Title:     "Nightly STM→LTM Consolidation",
			Content:   fmt.Sprintf("Consolidated %d archived messages into %d LTM facts.", result.MessagesConsolidated, result.FactsStored),
			Tags:      []string{"consolidation", "maintenance", "memory"},
		}); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("store consolidation journal: %w", err))
		}
	}

	if rootClaim {
		logger.Info("[Consolidation] STM→LTM consolidation complete",
			"messages_processed", result.MessagesConsolidated,
			"facts_stored", result.FactsStored,
			"remaining_claim_budget", budget.remaining)
	}
	return result
}

func maintenanceContextHasAtLeast(ctx context.Context, minimum time.Duration) bool {
	if ctx == nil {
		return true
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return true
	}
	return time.Until(deadline) >= minimum
}

func maintenanceContextWithReserve(parent context.Context, reserve time.Duration) (context.Context, context.CancelFunc, bool) {
	if parent == nil {
		parent = context.Background()
	}
	if reserve < 0 {
		reserve = 0
	}
	if err := parent.Err(); err != nil {
		ctx, cancel := context.WithCancel(parent)
		return ctx, cancel, false
	}
	deadline, ok := parent.Deadline()
	if !ok {
		ctx, cancel := context.WithCancel(parent)
		return ctx, cancel, true
	}
	phaseDeadline := deadline.Add(-reserve)
	if !time.Now().Before(phaseDeadline) {
		ctx, cancel := context.WithCancel(parent)
		return ctx, cancel, false
	}
	ctx, cancel := context.WithDeadline(parent, phaseDeadline)
	return ctx, cancel, true
}

func isContextCancellationText(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, context.Canceled.Error()) || strings.Contains(lower, context.DeadlineExceeded.Error())
}

func resolveConsolidationModel(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		return helperCfg.Model
	}
	if model := strings.TrimSpace(cfg.Consolidation.Model); model != "" {
		return model
	}
	return strings.TrimSpace(cfg.LLM.Model)
}

func resolveHelperBackedLLM(cfg *config.Config, fallbackClient llm.ChatClient, fallbackModel string) (llm.ChatClient, string) {
	if helperCfg := llm.ResolveHelperLLM(cfg); helperCfg.Enabled && helperCfg.Model != "" {
		manager := getOrCreateHelperLLMManager(cfg, nil)
		if manager != nil && manager.client != nil {
			return manager.client, helperCfg.Model
		}
		helperClient := llm.NewClientFromProviderWithConfig(cfg, helperCfg.ProviderType, helperCfg.BaseURL, helperCfg.APIKey, helperCfg.AccountID)
		if helperClient != nil {
			return helperClient, helperCfg.Model
		}
	}
	return fallbackClient, strings.TrimSpace(fallbackModel)
}

func consolidateEpisodicHierarchy(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph) (resultErr error) {
	if stm == nil || ltm == nil || ltm.IsDisabled() {
		return
	}
	episodes, err := stm.GetEpisodicMemoriesByHierarchyLevel(1, 40)
	if err != nil {
		return err
	}
	if len(episodes) < 2 {
		return
	}
	groups := make(map[string][]memory.EpisodicMemory)
	for _, episode := range episodes {
		if episode.ActionStatus == "pending" {
			continue
		}
		groupKey := episode.SessionID
		if groupKey == "" {
			groupKey = "global"
		}
		if len(episode.EventDate) >= 7 {
			groupKey += "|" + episode.EventDate[:7]
		}
		groups[groupKey] = append(groups[groupKey], episode)
	}
	for groupKey, group := range groups {
		if len(group) < 2 {
			continue
		}
		summary := buildHierarchicalEpisodeSummary(group)
		if strings.TrimSpace(summary) == "" {
			continue
		}
		concept := "Hierarchical memory synthesis " + groupKey
		owned, err := memory.StoreDocumentWithOwnership(ltm, concept, summary)
		ids := append(append(append([]string{}, owned.CreatedIDs...), owned.ReusedIDs...), owned.UnknownIDs...)
		if err != nil {
			resultErr = errors.Join(resultErr, err, rollbackStoredConsolidationFacts(logger, stm, ltm, owned.CreatedIDs))
			logger.Warn("[Hierarchy] Failed to store episodic synthesis", "group", groupKey, "error", err)
			continue
		}
		var metadataErr error
		for _, id := range owned.CreatedIDs {
			metadataErr = errors.Join(metadataErr, stm.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{
				ExtractionConfidence: 0.88,
				VerificationStatus:   "unverified",
				SourceType:           "hierarchical_consolidation",
				SourceReliability:    0.9,
			}))
		}
		for _, id := range owned.UnknownIDs {
			metadataErr = errors.Join(metadataErr, stm.EnsureMemoryMeta(id))
		}
		if metadataErr != nil {
			resultErr = errors.Join(resultErr, metadataErr, rollbackStoredConsolidationFacts(logger, stm, ltm, owned.CreatedIDs))
			continue
		}
		if kg != nil {
			uniqueParticipants := uniqueHierarchyStrings(nil)
			for _, episode := range group {
				uniqueParticipants = uniqueHierarchyStrings(append(uniqueParticipants, episode.Participants...))
			}
			for _, participant := range uniqueParticipants {
				if participant == "" {
					continue
				}
				if err := kg.AddEdge(participant, concept, "appears_in_memory_synthesis", map[string]string{"group": groupKey}); err != nil {
					resultErr = errors.Join(resultErr, err)
					logger.Warn("[Hierarchy] Failed to sync participant synthesis edge to KG",
						"participant", participant,
						"concept", concept,
						"group", groupKey,
						"error", err)
				}
			}
		}
		related := make([]string, 0, len(ids))
		related = append(related, ids...)
		episodeIDs := make([]int64, 0, len(group))
		for _, episode := range group {
			episodeIDs = append(episodeIDs, episode.ID)
			related = append(related, episode.RelatedDocIDs...)
		}
		episodeErr := stm.InsertEpisodicMemoryWithDetails(group[0].EventDate, "Hierarchical memory synthesis", truncateHierarchySummary(summary, 240), map[string]string{"group": groupKey}, 3, "hierarchical_consolidation", memory.EpisodicMemoryDetails{
			SessionID:      group[0].SessionID,
			HierarchyLevel: 2,
			Participants:   uniqueHierarchyParticipants(group),
			RelatedDocIDs:  uniqueHierarchyStrings(related),
		})
		resultErr = errors.Join(resultErr, episodeErr)
		if episodeErr == nil {
			resultErr = errors.Join(resultErr, stm.MarkEpisodicMemoriesHierarchy(episodeIDs, 2))
		}
	}
	return resultErr
}

func detectMemoryConflictsAcrossLTM(logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, prefetchedMetas []memory.MemoryMeta) {
	detectMemoryConflictsAcrossLTMWithContext(context.Background(), logger, stm, ltm, prefetchedMetas)
}

func detectMemoryConflictsAcrossLTMWithContext(ctx context.Context, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, prefetchedMetas []memory.MemoryMeta) error {
	if stm == nil || ltm == nil || ltm.IsDisabled() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	metas := prefetchedMetas
	if metas == nil {
		var err error
		metas, err = stm.GetAllMemoryMeta(nightlyMemoryConflictScanLimit, 0)
		if err != nil {
			return err
		}
	} else if len(metas) > nightlyMemoryConflictScanLimit {
		metas = metas[:nightlyMemoryConflictScanLimit]
	}
	var resultErr error
	for _, meta := range metas {
		if ctx.Err() != nil {
			return errors.Join(resultErr, ctx.Err())
		}
		resultErr = errors.Join(resultErr, detectMemoryConflictsForDocIDs(logger, stm, ltm, []string{meta.DocID}, ""))
	}
	return resultErr
}

func buildHierarchicalEpisodeSummary(group []memory.EpisodicMemory) string {
	if len(group) == 0 {
		return ""
	}
	parts := make([]string, 0, len(group)+1)
	parts = append(parts, fmt.Sprintf("Memory synthesis for %d related episodes:", len(group)))
	for _, episode := range group {
		parts = append(parts, fmt.Sprintf("- %s: %s", episode.Title, episode.Summary))
	}
	return strings.Join(parts, "\n")
}

func uniqueHierarchyParticipants(group []memory.EpisodicMemory) []string {
	values := make([]string, 0, len(group)*2)
	for _, episode := range group {
		values = append(values, episode.Participants...)
	}
	return uniqueHierarchyStrings(values)
}

func uniqueHierarchyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func truncateHierarchySummary(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

type autoOptimizeMemoryResult struct {
	Errors        []error
	KGOptimizeErr error
}

// autoOptimizeMemory runs priority-based forgetting on VectorDB and Knowledge Graph.
func autoOptimizeMemory(cfg *config.Config, logger *slog.Logger, client llm.ChatClient, ltm memory.VectorDB, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph, prefetchedMetas []memory.MemoryMeta) autoOptimizeMemoryResult {
	return autoOptimizeMemoryWithContext(context.Background(), cfg, logger, client, ltm, stm, kg, prefetchedMetas)
}

func autoOptimizeMemoryWithContext(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, ltm memory.VectorDB, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph, prefetchedMetas []memory.MemoryMeta) (result autoOptimizeMemoryResult) {
	// A partial replacement may have committed metadata even if retirement fails.
	defer InvalidateMemoryMetaCache()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		logger.Warn("[AutoOptimize] Memory optimization skipped: maintenance context canceled", "error", err)
		result.Errors = append(result.Errors, err)
		return result
	}
	threshold := cfg.Consolidation.OptimizeThreshold

	metas := prefetchedMetas
	if metas == nil {
		var err error
		metas, err = stm.GetAllMemoryMeta(nightlyMemoryMetaFetchLimit, 0)
		if err != nil {
			result.Errors = append(result.Errors, err)
			logger.Error("[AutoOptimize] Failed to fetch memory metadata", "error", err)
			return result
		}
	}

	var lowDocs, mediumDocs []string
	for _, meta := range metas {
		if meta.Protected || meta.KeepForever || memory.IsMemoryArchived(meta) {
			continue
		}
		priority := adjustedMemoryPriority(meta, time.Now())
		if priority < threshold {
			lowDocs = append(lowDocs, meta.DocID)
		} else if priority < threshold+2 {
			mediumDocs = append(mediumDocs, meta.DocID)
		}
	}

	// Low priority alone never requires physical destruction. Archive through
	// the policy-aware metadata path and retain the vector for recovery.
	for _, docID := range lowDocs {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			return result
		}
		if err := stm.ApplyMemoryCurationAction(memory.MemoryCurationAction{
			DocID: docID, Action: memory.MemoryCurationActionArchive, Reason: "auto-optimize low priority",
		}, "system", false); err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	// Compress medium-priority documents
	optimizeClient, optimizeModel := resolveHelperBackedLLM(cfg, client, cfg.LLM.Model)
	if optimizeClient == nil || optimizeModel == "" {
		logger.Warn("[AutoOptimize] Compression skipped: no helper/main LLM available")
		if len(mediumDocs) > 0 {
			result.Errors = append(result.Errors, fmt.Errorf("memory compression LLM unavailable"))
		}
		return result
	}
	helperManager := newHelperLLMManager(cfg, logger)
	type compressionWorkItem struct {
		docID    string
		content  string
		concept  string
		memoryID string
		meta     memory.MemoryMeta
	}
	workItems := make([]compressionWorkItem, 0, len(mediumDocs))
	for _, docID := range mediumDocs {
		content, err := ltm.GetByID(docID)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		if len(content) < 300 {
			continue
		}
		meta, err := stm.GetMemoryMeta(docID)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		concept := "Compressed Memory"
		parts := strings.SplitN(content, "\n\n", 2)
		if len(parts) == 2 {
			concept = parts[0]
		}
		workItems = append(workItems, compressionWorkItem{
			docID:    docID,
			meta:     meta,
			content:  content,
			concept:  concept,
			memoryID: fmt.Sprintf("mem_%d", len(workItems)+1),
		})
	}

	compressOne := func(item compressionWorkItem) {
		if err := ctx.Err(); err != nil {
			logger.Warn("[AutoOptimize] Memory compression skipped: maintenance context canceled", "doc_id", item.docID, "error", err)
			result.Errors = append(result.Errors, err)
			return
		}
		compressCtx, compressCancel := context.WithTimeout(ctx, 60*time.Second)
		defer compressCancel()

		resp, err := llm.ExecuteWithRetry(
			compressCtx,
			optimizeClient,
			openai.ChatCompletionRequest{
				Model: optimizeModel,
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: "Compress this memory into a dense bullet-point list of core facts. Output ONLY the compressed text."},
					{Role: openai.ChatMessageRoleUser, Content: item.content},
				},
				MaxTokens: 500,
			},
			logger,
			nil,
		)
		if err != nil || len(resp.Choices) == 0 {
			if err == nil {
				err = fmt.Errorf("compression completion empty")
			}
			result.Errors = append(result.Errors, err)
			return
		}
		compressed := strings.TrimSpace(resp.Choices[0].Message.Content)
		if compressed == "" || resp.Choices[0].FinishReason == openai.FinishReasonLength {
			result.Errors = append(result.Errors, fmt.Errorf("compression completion empty or truncated"))
			return
		}
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			return
		}
		if _, err := stm.ReplaceMemoryDocument(ltm, item.docID, item.concept, item.content, compressed, item.meta, "auto-optimize compressed into replacement memory", "system"); err != nil {
			result.Errors = append(result.Errors, err)
		}

	}

	const helperCompressionBatchSize = 3
	for i := 0; i < len(workItems); {
		if err := ctx.Err(); err != nil {
			logger.Warn("[AutoOptimize] Stopping memory compression: maintenance context canceled", "error", err)
			result.Errors = append(result.Errors, err)
			break
		}
		if helperManager == nil {
			compressOne(workItems[i])
			i++
			continue
		}

		end := i + helperCompressionBatchSize
		if end > len(workItems) {
			end = len(workItems)
		}
		group := workItems[i:end]
		inputs := make([]helperCompressionBatchInput, 0, len(group))
		for _, item := range group {
			inputs = append(inputs, helperCompressionBatchInput{
				MemoryID: item.memoryID,
				Content:  item.content,
			})
		}

		compressionCtx, compressionCancel := context.WithTimeout(ctx, 60*time.Second)
		compressionResult, err := helperManager.CompressMemoryBatches(compressionCtx, inputs)
		compressionCancel()
		if err != nil {
			helperManager.ObserveFallback("compress_memories", err.Error())
			logger.Warn("[HelperLLM] Memory compression batch failed, falling back", "start_memory", i+1, "error", err)
			for _, item := range group {
				compressOne(item)
			}
			i = end
			continue
		}

		byID := make(map[string]string, len(compressionResult.Memories))
		for _, item := range compressionResult.Memories {
			byID[item.MemoryID] = item.Compressed
		}
		for _, item := range group {
			compressed := strings.TrimSpace(byID[item.memoryID])
			if compressed == "" {
				compressOne(item)
				continue
			}
			if err := ctx.Err(); err != nil {
				result.Errors = append(result.Errors, err)
				break
			}
			if _, err := stm.ReplaceMemoryDocument(ltm, item.docID, item.concept, item.content, compressed, item.meta, "auto-optimize compressed into replacement memory", "system"); err != nil {
				result.Errors = append(result.Errors, err)
			}

		}
		i = end
	}

	// Optimize Knowledge Graph
	graphRemoved := 0
	if shouldOptimizeKnowledgeGraph(cfg, kg) {
		if dropped := kg.DroppedAccessHits(); dropped > 0 {
			logger.Warn("[AutoOptimize] Dropped knowledge graph access hits under load", "dropped", dropped)
		}
		removed, err := kg.OptimizeGraph(threshold)
		if err != nil {
			logger.Warn("[AutoOptimize] Knowledge graph optimization failed", "error", err)
			result.KGOptimizeErr = err
		} else {
			graphRemoved = removed
		}
	}

	if len(lowDocs) > 0 || len(mediumDocs) > 0 || graphRemoved > 0 {
		logger.Info("[AutoOptimize] Memory optimization complete",
			"low_removed", len(lowDocs),
			"medium_compressed", len(mediumDocs),
			"graph_nodes_removed", graphRemoved)
	}
	return result
}

func autoCurateMemory(cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, prefetchedMetas []memory.MemoryMeta) (resultErr error) {
	if cfg == nil || stm == nil {
		return
	}
	metas := prefetchedMetas
	if metas == nil {
		var err error
		metas, err = stm.GetAllMemoryMeta(nightlyMemoryMetaFetchLimit, 0)
		if err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Warn("[MemoryCurator] Failed to fetch memory metadata", "error", err)
			return
		}
	}
	usage, err := stm.GetMemoryUsageStats(30, 500)
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Warn("[MemoryCurator] Failed to fetch memory usage stats", "error", err)
		usage = memory.MemoryUsageStats{WindowDays: 30}
	}
	threshold := cfg.MemoryAnalysis.AutoConfirm
	if threshold <= 0 {
		threshold = 0.92
	}
	plan := memory.BuildMemoryCurationPlan(metas, usage, memory.MemoryCurationOptions{
		ConfirmThreshold: threshold,
		MaxActions:       100,
	})
	appliedConfirm := 0
	appliedArchive := 0
	for _, action := range plan.AutoConfirm {
		if err := stm.ApplyMemoryCurationAction(action, "system", false); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Warn("[MemoryCurator] Failed to confirm memory", "doc_id", action.DocID, "error", err)
			continue
		}
		appliedConfirm++
	}
	for _, action := range plan.AutoArchive {
		if err := stm.ApplyMemoryCurationAction(action, "system", false); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Warn("[MemoryCurator] Failed to archive memory", "doc_id", action.DocID, "error", err)
			continue
		}
		appliedArchive++
	}
	if appliedConfirm > 0 || appliedArchive > 0 || plan.ReviewRequiredCount > 0 {
		if appliedConfirm > 0 || appliedArchive > 0 {
			InvalidateMemoryMetaCache()
		}
		logger.Info("[MemoryCurator] Curation run complete",
			"confirmed", appliedConfirm,
			"archived", appliedArchive,
			"review_required", plan.ReviewRequiredCount)
	}
	return resultErr
}

// SyncContactsToKnowledgeGraph synchronizes contacts to the knowledge graph.
func SyncContactsToKnowledgeGraph(ctx context.Context, contactsDB *sql.DB, kg *memory.KnowledgeGraph, logger *slog.Logger) (resultErr error) {
	if contactsDB == nil || kg == nil {
		return
	}

	logger.Info("[Maintenance] Syncing Contacts to Knowledge Graph")

	rows, err := contactsDB.QueryContext(ctx, "SELECT id, name, email, phone, mobile, relationship, birthday FROM contacts")
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Maintenance] Failed to query contacts for KG sync", "error", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		var email, phone, mobile, relationship, birthday sql.NullString
		if err := rows.Scan(&id, &name, &email, &phone, &mobile, &relationship, &birthday); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Error("[Maintenance] Failed to scan contact", "error", err)
			continue
		}

		nodeID := "contact_" + id
		props := map[string]string{
			"type": "person",
		}
		if email.Valid && email.String != "" {
			props["email"] = email.String
		}
		if phone.Valid && phone.String != "" {
			props["phone"] = phone.String
		}
		if mobile.Valid && mobile.String != "" {
			props["mobile"] = mobile.String
		}
		if relationship.Valid && relationship.String != "" {
			props["relationship"] = relationship.String
		}
		if birthday.Valid && birthday.String != "" {
			props["birthday"] = birthday.String
		}

		err := kg.AddNode(nodeID, name, props)
		if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			resultErr = errors.Join(resultErr, err)
			logger.Debug("[Maintenance] AddNode returned error", "nodeID", nodeID, "error", err)
		}

		if relationship.Valid && relationship.String != "" {
			relSlug := strings.ToLower(strings.ReplaceAll(relationship.String, " ", "_"))
			relNodeID := "org_" + relSlug

			if _, err := kg.PruneOutgoingRelationEdges(nodeID, "belongs_to", map[string]struct{}{relNodeID: {}}); err != nil {
				resultErr = errors.Join(resultErr, err)
				logger.Warn("[Maintenance] Failed to prune stale contact relationship edges",
					"contact_node_id", nodeID,
					"relationship_node_id", relNodeID,
					"error", err)
			}

			if err := kg.AddNode(relNodeID, relationship.String, map[string]string{"type": "organization"}); err != nil {
				resultErr = errors.Join(resultErr, err)
				logger.Warn("[Maintenance] Failed to sync relationship org node to KG",
					"contact_node_id", nodeID,
					"relationship_node_id", relNodeID,
					"relationship", relationship.String,
					"error", err)
			} else if err := kg.AddEdge(nodeID, relNodeID, "belongs_to", nil); err != nil {
				resultErr = errors.Join(resultErr, err)
				logger.Warn("[Maintenance] Failed to sync relationship edge to KG",
					"contact_node_id", nodeID,
					"relationship_node_id", relNodeID,
					"relationship", relationship.String,
					"error", err)
			}
		}
	}
	return errors.Join(resultErr, rows.Err(), ctx.Err())
}

// SyncPlannerToKnowledgeGraph synchronizes appointments and todos to the knowledge graph.
func SyncPlannerToKnowledgeGraph(ctx context.Context, plannerDB *sql.DB, kg planner.KnowledgeGraph, logger *slog.Logger) (resultErr error) {
	if plannerDB == nil || kg == nil {
		return
	}

	logger.Info("[Maintenance] Syncing Planner to Knowledge Graph")

	tracker := planner.NewKGSyncTracker()
	if err := planner.EnsurePlannerWorkspaceHub(kg, tracker); err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Debug("[Maintenance] Failed to ensure planner workspace hub", "error", err)
	}

	appointments, err := planner.ListAppointments(plannerDB, "", "")
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Maintenance] Failed to list appointments for KG sync", "error", err)
	} else {
		for _, a := range appointments {
			if a.Status == "cancelled" {
				continue
			}
			contactIDs, contactErr := planner.GetAppointmentContactIDs(plannerDB, a.ID)
			if contactErr != nil {
				resultErr = errors.Join(resultErr, contactErr)
				logger.Debug("[Maintenance] Failed to load appointment contacts for KG sync", "appointment_id", a.ID, "error", contactErr)
				continue
			}
			if err := planner.SyncAppointmentKGRecord(kg, a, contactIDs, tracker); err != nil {
				resultErr = errors.Join(resultErr, err)
				logger.Debug("[Maintenance] Failed to sync appointment to KG", "nodeID", a.KGNodeID, "error", err)
			}
		}
	}

	todos, err := planner.ListTodos(plannerDB, "", "")
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Maintenance] Failed to list todos for KG sync", "error", err)
		return
	}
	for _, t := range todos {
		if err := planner.SyncTodoKGRecord(kg, t, tracker); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Debug("[Maintenance] Failed to sync todo to KG", "nodeID", t.KGNodeID, "error", err)
		}
	}

	if resultErr != nil || ctx.Err() != nil {
		return errors.Join(resultErr, ctx.Err())
	}
	if removed, err := kg.DeleteStalePlannerSyncEdges(tracker.ExpectedEdges, tracker.ActivePlannerNodes); err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Warn("[Maintenance] Failed to clean stale planner KG edges", "error", err)
	} else if removed > 0 {
		logger.Info("[Maintenance] Removed stale planner KG edges", "removed", removed)
	}

	if removed, err := kg.PruneStalePlannerRootNodes(tracker.ActivePlannerNodes); err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Warn("[Maintenance] Failed to prune stale planner KG nodes", "error", err)
	} else if removed > 0 {
		logger.Info("[Maintenance] Removed stale planner KG nodes", "removed", removed)
	}

	if removed, err := kg.PruneStalePlannerItemNodes(tracker.ActivePlannerNodes); err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Warn("[Maintenance] Failed to prune stale planner KG item nodes", "error", err)
	} else if removed > 0 {
		logger.Info("[Maintenance] Removed stale planner KG item nodes", "removed", removed)
	}
	return resultErr
}

// SyncCoreMemoryToKnowledgeGraph synchronizes core memory facts to the knowledge graph.
func SyncCoreMemoryToKnowledgeGraph(ctx context.Context, stm *memory.SQLiteMemory, kg *memory.KnowledgeGraph, logger *slog.Logger) (resultErr error) {
	if stm == nil || kg == nil {
		return
	}

	logger.Info("[Maintenance] Syncing Core Memory to Knowledge Graph")

	facts, err := stm.GetCoreMemoryFacts()
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Error("[Maintenance] Failed to get core memory facts for KG sync", "error", err)
		return
	}

	expected := make(map[string]struct{}, len(facts))
	for _, fact := range facts {
		nodeID := fmt.Sprintf("core_fact_%d", fact.ID)
		expected[nodeID] = struct{}{}
		label := fact.Fact
		if len(label) > 50 {
			label = label[:47] + "..."
		}
		props := map[string]string{
			"type":    "concept",
			"source":  "core_memory",
			"content": fact.Fact,
		}

		err := kg.AddNode(nodeID, label, props)
		if err != nil && !strings.Contains(err.Error(), "UNIQUE constraint failed") {
			resultErr = errors.Join(resultErr, err)
			logger.Debug("[Maintenance] AddNode returned error", "nodeID", nodeID, "error", err)
		}
	}

	if resultErr != nil || ctx.Err() != nil {
		return errors.Join(resultErr, ctx.Err())
	}
	nodes, err := kg.ListNodesByIDPrefix("core_fact_", 10000)
	if err != nil {
		resultErr = errors.Join(resultErr, err)
		logger.Debug("[Maintenance] Failed to list core memory KG nodes", "error", err)
		return
	}
	for _, node := range nodes {
		if _, ok := expected[node.ID]; ok {
			continue
		}
		if err := kg.DeleteNode(node.ID); err != nil {
			resultErr = errors.Join(resultErr, err)
			logger.Debug("[Maintenance] Failed to delete stale core memory KG node", "nodeID", node.ID, "error", err)
		}
	}
	return resultErr
}
