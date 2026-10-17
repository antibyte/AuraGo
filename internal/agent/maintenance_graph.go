package agent

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"
)

// Planner helpers keep their shared interface while maintenance bounds embeddings.
type maintenancePlannerGraph struct {
	*memory.KnowledgeGraph
	ctx context.Context
}

func (kg maintenancePlannerGraph) AddNode(id, label string, properties map[string]string) error {
	return kg.AddNodeIndexed(kg.ctx, id, label, properties)
}

func (kg maintenancePlannerGraph) AddEdge(source, target, relation string, properties map[string]string) error {
	return kg.AddEdgeContext(kg.ctx, source, target, relation, properties)
}

func runKnowledgeGraphMaintenance(graphCtx context.Context, cfg *config.Config, logger *slog.Logger, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph, inventoryDB, contactsDB, plannerDB *sql.DB, ledger *maintenanceRunLedger) {
	retention := resolveMaintenanceRetention(cfg)
	// Notes: clean up old completed notes (done for >7 days)
	if graphCtx.Err() != nil {
		return
	}
	if cfg.Tools.Notes.Enabled && stm != nil {
		deleted, err := stm.DeleteOldDoneNotes(retention.DoneNotesDays)
		if err != nil {
			logger.Error("[Maintenance] Failed to clean old done notes", "error", err)
			ledger.addError("notes_cleanup: " + err.Error())
		} else if deleted > 0 {
			logger.Info("[Maintenance] Cleaned old done notes", "deleted", deleted)
		}
	}
	if graphCtx.Err() != nil {
		return
	}
	if stm != nil {
		hygieneStats := runAutomaticMemoryHygieneWithContext(graphCtx, cfg, logger, stm, ltm)
		for _, err := range hygieneStats.Errors {
			ledger.recordError("memory_hygiene", err)
		}
		ledger.addDeferred("knowledge_graph", hygieneStats.Deferred)
		ledger.phaseResults.JournalRemoved = hygieneStats.JournalRemoved
		ledger.phaseResults.NotesArchived = hygieneStats.NotesArchived
		ledger.addProcessed("knowledge_graph", hygieneStats.JournalRemoved+hygieneStats.NotesArchived)
	}

	// Knowledge Graph: Garbage collection and semantic reindex
	if graphCtx.Err() != nil {
		return
	}
	if kg != nil {
		if _, _, err := kg.CleanupStaleGraphWithOptions(memory.KnowledgeGraphCleanupOptions{
			PendingCoMentionDays: cfg.Tools.KnowledgeGraph.PendingCoMentionTTLDays,
			StaleNodeDays:        30,
		}); err != nil {
			logger.Error("[Maintenance] Failed to clean up stale KG elements", "error", err)
			ledger.addError("kg_cleanup: " + err.Error())
		}
		if graphCtx.Err() != nil {
			return
		}
		if ran, err := kg.RunSemanticReindexIfDueContext(graphCtx); err != nil {
			logger.Warn("[Maintenance] Failed to reindex semantic knowledge graph", "error", err)
			if graphCtx.Err() == nil {
				ledger.addError("kg_semantic_reindex: " + err.Error())
			}
		} else if ran {
			if extraPasses, drainErr := kg.DrainSemanticReindexBacklogContext(graphCtx, 2); drainErr != nil {
				logger.Warn("[Maintenance] Failed to drain knowledge graph semantic reindex backlog", "error", drainErr)
				if graphCtx.Err() == nil {
					ledger.addError("kg_semantic_reindex: " + drainErr.Error())
				}
			} else if extraPasses > 0 {
				logger.Debug("[Maintenance] Knowledge graph semantic reindex follow-up passes completed", "extra_passes", extraPasses)
			}
			if graphCtx.Err() != nil {
				return
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

	if graphCtx.Err() != nil {
		return
	}
	if kg != nil && inventoryDB != nil {
		if err := kg.SyncExternalSourcesContext(graphCtx, inventoryDB, logger); err != nil {
			logger.Warn("[Maintenance] Inventory KG sync failed", "error", err)
			ledger.addError("inventory_kg_sync: " + err.Error())
		}
	}

	// Sync contacts and core memory
	if graphCtx.Err() != nil {
		return
	}
	if kg != nil {
		ledger.recordError("contacts_kg_sync", SyncContactsToKnowledgeGraph(graphCtx, contactsDB, kg, logger))
	}
	if graphCtx.Err() != nil {
		return
	}
	if kg != nil {
		ledger.recordError("planner_kg_sync", SyncPlannerToKnowledgeGraph(graphCtx, plannerDB, kg, logger))
	}
	if graphCtx.Err() != nil {
		return
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
	if graphCtx.Err() != nil {
		return
	}
	if kg != nil && stm != nil {
		ledger.recordError("core_kg_sync", SyncCoreMemoryToKnowledgeGraph(graphCtx, stm, kg, logger))
		if graphCtx.Err() != nil {
			return
		}
		recordKnowledgeGraphSparseIssue(plannerDB, stm, kg, logger)
		recordKnowledgeGraphQualityIssues(plannerDB, kg, logger)
	}
}
