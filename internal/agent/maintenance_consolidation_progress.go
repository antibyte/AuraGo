package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
)

const consolidationProgressPromptVersion = "source-bound-v2"
const consolidationMaxClaimPage = 1000
const consolidationMaxPublishedFactsPerRun = 100
const consolidationStaleClaimAge = 30 * time.Minute

type checkpointedConsolidationItem struct {
	message           memory.ArchivedMessage
	key               memory.ConsolidationCheckpointKey
	parts             []string
	results           map[int][]helperConsolidationFact
	failed            error
	blocked           bool
	finished          bool
	readyMarked       bool
	failureRecorded   bool
	publicationLoaded bool
	facts             []helperConsolidationFact
	publishCursor     int
	storedFacts       int
	skippedFacts      int
	factCount         int
}

func consolidateSTMtoLTMWithDurableProgress(ctx context.Context, cfg *config.Config, logger *slog.Logger, client llm.ChatClient, stm *memory.SQLiteMemory, ltm memory.VectorDB, kg *memory.KnowledgeGraph) (result consolidationRunResult) {
	if cfg == nil || stm == nil || ltm == nil || !runtimeMemoryWriteAllowed(cfg, "memory", "") {
		return result
	}
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
			if ctx.Err() != nil || cfg == nil || cfg.Consolidation.ArchiveRetainDays <= 0 {
				return
			}
			if _, err := stm.CleanOldArchivedMessagesContext(ctx, cfg.Consolidation.ArchiveRetainDays, 250); err != nil {
				if logger != nil {
					logger.Error("[Consolidation] Failed to clean old archived messages", "error", err)
				}
				result.Errors = append(result.Errors, err)
			}
		}()
		excluded, err := stm.FinalizeIneligibleConsolidationCandidatesContext(ctx, 250)
		if err != nil {
			result.Errors = append(result.Errors, err)
		} else {
			result.MessagesExcluded = int(excluded)
		}
	}
	if err := ctx.Err(); err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	if reclaimed, err := stm.ReclaimStaleConsolidationClaimsContext(ctx, consolidationStaleClaimAge, 100); err != nil {
		result.Errors = append(result.Errors, err)
	} else if reclaimed > 0 && logger != nil {
		logger.Info("[Consolidation] Reclaimed stale claims", "count", reclaimed)
	}
	if ctx.Err() != nil {
		result.Errors = append(result.Errors, ctx.Err())
		return result
	}
	consolidationClient, model := resolveHelperBackedLLM(cfg, client, resolveConsolidationModel(cfg))
	if consolidationClient == nil || model == "" {
		return result
	}
	claimLimit := min(budget.remaining, consolidationMaxClaimPage)
	if claimLimit <= 0 {
		return result
	}
	archived, err := stm.ClaimConsolidationCandidatesContext(ctx, claimLimit, 3)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	if len(archived) == 0 {
		return result
	}
	budget.remaining -= len(archived)
	result.MessagesClaimed = len(archived)
	ids := make([]int64, 0, len(archived))
	for _, message := range archived {
		ids = append(ids, message.ID)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := stm.ReleaseConsolidationClaimsContext(cleanupCtx, ids); err != nil {
			result.Errors = append(result.Errors, err)
		}
	}()

	items := make([]*checkpointedConsolidationItem, 0, len(archived))
	for _, message := range archived {
		if err := ctx.Err(); err != nil {
			result.Errors = append(result.Errors, err)
			return result
		}
		parts := serializeConsolidationSources([]memory.ArchivedMessage{message})
		key := memory.ConsolidationCheckpointKey{
			MessageID: message.ID, SourceHash: memory.ConsolidationSourceHash(message),
			PromptVersion: consolidationProgressPromptVersion, ModelID: model,
		}
		checkpoint, err := stm.EnsureConsolidationCheckpoint(ctx, key, len(parts))
		if err != nil {
			if ctx.Err() != nil {
				result.Errors = append(result.Errors, ctx.Err())
				return result
			}
			result.Errors = append(result.Errors, err)
			if markErr := stm.DeferConsolidationClaimsContext(ctx, []int64{message.ID}); markErr != nil {
				result.Errors = append(result.Errors, markErr)
			}
			continue
		}
		_ = checkpoint
		item := &checkpointedConsolidationItem{message: message, key: key, parts: parts, results: make(map[int][]helperConsolidationFact)}
		persisted, err := stm.ListConsolidationPartResults(ctx, key)
		if err != nil {
			result.Errors = append(result.Errors, err)
			item.failed = err
		} else {
			for _, part := range persisted {
				var facts []helperConsolidationFact
				if err := json.Unmarshal([]byte(part.FactsJSON), &facts); err != nil || facts == nil || countValidConsolidationFacts(facts) != len(facts) || len(facts) > 10 {
					item.failed = errors.New("checkpoint_invalid")
					break
				}
				item.results[part.PartIndex] = facts
			}
		}
		items = append(items, item)
	}

	// Publish complete messages between fair extraction rounds. Each publish
	// pass advances at most one fact per message, with a total run cap, so a
	// large ready checkpoint cannot starve smaller completed messages.
	publishedFacts := 0
	publishReadyItems := func() bool {
		for _, item := range items {
			if item.failed != nil || item.blocked || item.finished || len(item.results) != len(item.parts) || item.publicationLoaded {
				continue
			}
			if err := ctx.Err(); err != nil {
				result.Errors = append(result.Errors, err)
				return false
			}
			if !item.readyMarked {
				if err := stm.MarkConsolidationCheckpointReady(ctx, item.key); err != nil {
					if ctx.Err() != nil {
						result.Errors = append(result.Errors, ctx.Err())
						return false
					}
					result.Errors = append(result.Errors, err)
					_ = stm.DeferConsolidationClaimsContext(ctx, []int64{item.message.ID})
					item.blocked = true
					continue
				}
				item.readyMarked = true
			}
			checkpoint, err := stm.EnsureConsolidationCheckpoint(ctx, item.key, len(item.parts))
			if err != nil {
				result.Errors = append(result.Errors, err)
				_ = stm.DeferConsolidationClaimsContext(ctx, []int64{item.message.ID})
				item.blocked = true
				continue
			}
			persisted, err := stm.ListConsolidationPartResults(ctx, item.key)
			if err != nil {
				if ctx.Err() != nil {
					result.Errors = append(result.Errors, ctx.Err())
					return false
				}
				result.Errors = append(result.Errors, err)
				_ = stm.DeferConsolidationClaimsContext(ctx, []int64{item.message.ID})
				item.blocked = true
				continue
			}
			item.facts = make([]helperConsolidationFact, 0)
			if len(persisted) != len(item.parts) {
				item.failed = errors.New("checkpoint_incomplete")
			}
			for index, part := range persisted {
				if part.PartIndex != index {
					item.failed = errors.New("checkpoint_invalid")
					break
				}
				var partFacts []helperConsolidationFact
				if err := json.Unmarshal([]byte(part.FactsJSON), &partFacts); err != nil || partFacts == nil || countValidConsolidationFacts(partFacts) != len(partFacts) || len(partFacts) > 10 {
					item.failed = errors.New("checkpoint_invalid")
					break
				}
				item.facts = append(item.facts, partFacts...)
			}
			if item.failed != nil {
				result.Errors = append(result.Errors, item.failed)
				reason := "checkpoint_invalid"
				if item.failed.Error() == "checkpoint_incomplete" {
					reason = "checkpoint_incomplete"
				}
				if err := stm.RecordConsolidationFailureContext(ctx, []int64{item.message.ID}, reason); err != nil {
					result.Errors = append(result.Errors, err)
				}
				item.failureRecorded = true
				item.blocked = true
				continue
			}
			item.factCount = len(item.facts)
			item.publishCursor = checkpoint.PublishCursor
			if item.publishCursor < 0 || item.publishCursor > len(item.facts) {
				item.failed = errors.New("checkpoint_invalid")
				item.failureRecorded = true
				item.blocked = true
				result.Errors = append(result.Errors, item.failed)
				if err := stm.RecordConsolidationFailureContext(ctx, []int64{item.message.ID}, "checkpoint_invalid"); err != nil {
					result.Errors = append(result.Errors, err)
				}
				continue
			}
			item.publicationLoaded = true
		}

		for publishedFacts < consolidationMaxPublishedFactsPerRun {
			progress := false
			for _, item := range items {
				if item.failed != nil || item.blocked || item.finished || !item.publicationLoaded || item.publishCursor >= len(item.facts) {
					continue
				}
				if err := ctx.Err(); err != nil {
					result.Errors = append(result.Errors, err)
					return false
				}
				if !runtimeMemoryWriteAllowed(cfg, "memory", item.message.SessionID) {
					item.blocked = true
					continue
				}
				cursor := item.publishCursor
				stored, ignored, err := storeConsolidationFactsWithContext(ctx, logger, stm, ltm, item.facts[cursor:cursor+1], memory.MemoryMetaUpdate{SourceSessionID: item.message.SessionID, SourceRole: item.message.Role})
				if err != nil {
					if ctx.Err() != nil {
						result.Errors = append(result.Errors, ctx.Err())
						return false
					}
					result.Errors = append(result.Errors, err)
					if isTransientHelperError(err) {
						_ = stm.DeferConsolidationClaimsContext(ctx, []int64{item.message.ID})
					} else if markErr := stm.RecordConsolidationFailureContext(ctx, []int64{item.message.ID}, "publication_failed"); markErr != nil {
						result.Errors = append(result.Errors, markErr)
					}
					item.blocked = true
					continue
				}
				// Count a fact as soon as the idempotent vector write succeeds. A
				// later cancellation or finalize error must not erase this run's work.
				result.FactsStored += stored
				item.storedFacts += stored
				item.skippedFacts += ignored
				advanced, err := stm.AdvanceConsolidationPublishCursor(ctx, item.key, cursor)
				if err != nil || !advanced {
					if err == nil {
						err = errors.New("consolidation_publish_cursor_conflict")
					}
					result.Errors = append(result.Errors, err)
					if ctx.Err() == nil {
						_ = stm.DeferConsolidationClaimsContext(ctx, []int64{item.message.ID})
					}
					item.blocked = true
					continue
				}
				item.publishCursor++
				publishedFacts++
				progress = true
				if publishedFacts >= consolidationMaxPublishedFactsPerRun {
					break
				}
			}
			for batchIndex, item := range items {
				if item.failed != nil || item.blocked || item.finished || !item.publicationLoaded || item.publishCursor < len(item.facts) {
					continue
				}
				finalizeErr := stm.FinalizeConsolidationCheckpoint(ctx, item.key)
				if finalizeErr != nil && ctx.Err() == nil {
					// The first commit may have been durable even when the driver
					// reported an ambiguous failure. The exact-source retry is safe.
					if retryErr := stm.FinalizeConsolidationCheckpoint(ctx, item.key); retryErr == nil {
						finalizeErr = nil
					} else {
						finalizeErr = errors.Join(finalizeErr, retryErr)
					}
				}
				if finalizeErr != nil {
					if ctx.Err() != nil {
						result.Errors = append(result.Errors, ctx.Err())
						return false
					}
					result.Errors = append(result.Errors, finalizeErr)
					_ = stm.RecordConsolidationFailureContext(ctx, []int64{item.message.ID}, "finalize_failed")
					item.blocked = true
					continue
				}
				item.finished = true
				result.MessagesConsolidated++
				if err := recordConsolidationBatchEpisode(stm, []memory.ArchivedMessage{item.message}, item.storedFacts, item.skippedFacts, item.factCount, batchIndex+1, len(items)); err != nil {
					result.Errors = append(result.Errors, err)
				}
			}
			if !progress {
				break
			}
		}
		return true
	}

	// Resume any already-complete checkpoints before spending another helper
	// request, then handle at most one part per source message in each round.
	if !publishReadyItems() {
		return result
	}
	maxParts := 0
	for _, item := range items {
		if len(item.parts) > maxParts {
			maxParts = len(item.parts)
		}
	}
	for partIndex := 0; partIndex < maxParts; partIndex++ {
		pending := make([]*checkpointedConsolidationItem, 0, len(items))
		for _, item := range items {
			if ctx.Err() != nil {
				result.Errors = append(result.Errors, ctx.Err())
				return result
			}
			if item.failed != nil || item.blocked || item.finished || partIndex >= len(item.parts) {
				continue
			}
			if _, saved := item.results[partIndex]; saved {
				continue
			}
			pending = append(pending, item)
		}
		batchSize := consolidationDirectBatchSize
		if llm.ResolveHelperLLM(cfg).Enabled {
			batchSize = helperConsolidationBatchSize
		}
		for offset := 0; offset < len(pending); offset += batchSize {
			end := min(offset+batchSize, len(pending))
			group := pending[offset:end]
			factsByMessage, extractionErr := extractConsolidationPartGroup(ctx, cfg, logger, consolidationClient, model, group, partIndex)
			if extractionErr != nil && ctx.Err() != nil {
				result.Errors = append(result.Errors, ctx.Err())
				return result
			}
			if extractionErr != nil && errors.Is(extractionErr, context.DeadlineExceeded) {
				// A request-level deadline is unspent work; leave it pending and
				// let the deferred claim release retry it on a later maintenance run.
				result.Errors = append(result.Errors, extractionErr)
				for _, item := range group {
					item.blocked = true
				}
				continue
			}
			if extractionErr != nil {
				// A malformed or unavailable grouped response does not poison the
				// stable per-message checkpoints. Retry each missing part alone.
				factsByMessage = make(map[int64][]helperConsolidationFact, len(group))
				for _, item := range group {
					conversation := fmt.Sprintf("source message_id=%d part=%d\n%s", item.message.ID, partIndex, item.parts[partIndex])
					facts, err := extractConsolidationFactsWithLLM(ctx, cfg, logger, consolidationClient, model, conversation)
					if err != nil {
						if errors.Is(err, context.DeadlineExceeded) {
							item.blocked = true
							result.Errors = append(result.Errors, err)
							continue
						}
						item.failed = err
						result.Errors = append(result.Errors, err)
						continue
					}
					factsByMessage[item.message.ID] = facts
				}
			}
			for _, item := range group {
				facts, ok := factsByMessage[item.message.ID]
				if item.failed != nil || item.blocked {
					continue
				}
				if !ok || facts == nil || len(facts) > 10 || countValidConsolidationFacts(facts) != len(facts) {
					item.failed = errors.New("extraction_invalid")
					result.Errors = append(result.Errors, item.failed)
					continue
				}
				encoded, err := json.Marshal(facts)
				if err == nil {
					err = stm.SaveConsolidationPartResult(ctx, item.key, partIndex, string(encoded))
				}
				if err != nil {
					if ctx.Err() != nil {
						result.Errors = append(result.Errors, ctx.Err())
						return result
					}
					item.failed = err
					result.Errors = append(result.Errors, err)
					continue
				}
				item.results[partIndex] = facts
			}
			// Persist the complete validated group before publishing so ready
			// messages share the same bounded, round-robin publication pass.
			if !publishReadyItems() {
				return result
			}
		}
		if !publishReadyItems() {
			return result
		}
	}

	for _, item := range items {
		if item.blocked || item.finished {
			continue
		}
		if item.failed != nil {
			if item.failureRecorded {
				continue
			}
			if ctx.Err() != nil {
				result.Errors = append(result.Errors, ctx.Err())
				return result
			}
			reason := "extraction_invalid"
			if isTransientHelperError(item.failed) {
				reason = "infrastructure_unavailable"
				if err := stm.DeferConsolidationClaimsContext(ctx, []int64{item.message.ID}); err != nil {
					result.Errors = append(result.Errors, err)
				}
			} else if err := stm.RecordConsolidationFailureContext(ctx, []int64{item.message.ID}, reason); err != nil {
				result.Errors = append(result.Errors, err)
			}
			continue
		}
		if len(item.results) != len(item.parts) {
			if err := stm.RecordConsolidationFailureContext(ctx, []int64{item.message.ID}, "checkpoint_incomplete"); err != nil {
				result.Errors = append(result.Errors, err)
			}
			continue
		}
	}

	if budget.remaining > 0 && len(archived) == claimLimit && ctx.Err() == nil {
		more := consolidateSTMtoLTMWithContext(ctx, cfg, logger, client, stm, ltm, kg)
		result.FactsStored += more.FactsStored
		result.MessagesConsolidated += more.MessagesConsolidated
		result.MessagesClaimed += more.MessagesClaimed
		result.MessagesExcluded += more.MessagesExcluded
		result.Errors = append(result.Errors, more.Errors...)
	}
	if rootClaim && cfg.Tools.Journal.Enabled && result.FactsStored > 0 {
		if _, err := stm.InsertJournalEntry(memory.JournalEntry{EntryType: "system", Title: "Nightly STM→LTM Consolidation", Content: fmt.Sprintf("Consolidated %d archived messages into %d LTM facts.", result.MessagesConsolidated, result.FactsStored), Tags: []string{"consolidation", "maintenance", "memory"}}); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("store consolidation journal: %w", err))
		}
	}
	return result
}
