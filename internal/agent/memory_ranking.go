package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"aurago/internal/memory"
)

// rankMemoryCandidates centralizes the retrieval score calculation for vector memories.
// It combines semantic similarity, recency, confidence/provenance signals, and
// session-local reuse penalties into one consistent score pipeline.
func rankMemoryCandidates(memories []string, docIDs []string, stm *memory.SQLiteMemory, usedDocIDs map[string]int, now time.Time) ([]rankedMemory, error) {
	return rankMemoryCandidatesWithScores(memories, docIDs, nil, stm, usedDocIDs, now)
}

func rankMemoryCandidatesWithScores(memories []string, docIDs []string, similarities []float64, stm *memory.SQLiteMemory, usedDocIDs map[string]int, now time.Time) ([]rankedMemory, error) {
	if len(memories) == 0 {
		return nil, nil
	}
	if len(docIDs) != len(memories) {
		return nil, fmt.Errorf("memory results are missing document IDs")
	}
	metaMap, err := loadMemoryMetaMap(stm, docIDs)
	if err != nil {
		return nil, err
	}
	results := make([]rankedMemory, 0, len(memories))

	for i, mem := range memories {
		docID := ""
		if i < len(docIDs) {
			docID = docIDs[i]
		}
		sim := 0.0
		if i < len(similarities) {
			sim = similarities[i]
		}
		if sim <= 0 {
			sim = memory.ExtractSimilarityScore(mem)
		}

		meta := memory.MemoryMeta{}
		if docID != "" {
			if storedMeta, hasMeta := metaMap[docID]; hasMeta {
				if memory.IsMemoryArchived(storedMeta) {
					continue
				}
				meta = storedMeta
			}
		}
		finalScore := calculateMemoryRankingScore(sim, meta, usedDocIDs[docID], now)
		results = append(results, rankedMemory{text: mem, docID: docID, score: finalScore})
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	return results, nil
}

func searchSimilarWithScores(ctx context.Context, vdb memory.VectorDB, query string, topK int, excludeCollections ...string) ([]string, []string, []float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if scored, ok := vdb.(memory.ContextScoredVectorDB); ok {
		results, err := scored.SearchSimilarScoredContext(ctx, query, topK, excludeCollections...)
		if err != nil {
			return nil, nil, nil, err
		}
		return splitScoredMemoryResults(results)
	}
	if scored, ok := vdb.(memory.ScoredVectorDB); ok {
		results, err := scored.SearchSimilarScored(query, topK, excludeCollections...)
		if err != nil {
			return nil, nil, nil, err
		}
		return splitScoredMemoryResults(results)
	}
	if ctxVdb, ok := vdb.(memory.ContextVectorDB); ok {
		memories, docIDs, err := ctxVdb.SearchSimilarContext(ctx, query, topK, excludeCollections...)
		return memories, docIDs, nil, err
	}
	memories, docIDs, err := vdb.SearchSimilar(query, topK, excludeCollections...)
	return memories, docIDs, nil, err
}

// searchRankedMemoriesOnly searches aurago_memories and applies the shared ranking
// pipeline, including archived-memory filtering via memory_meta.
func searchRankedMemoriesOnly(
	ctx context.Context,
	vdb memory.VectorDB,
	stm *memory.SQLiteMemory,
	query string,
	topK int,
	usedDocIDs map[string]int,
	now time.Time,
) ([]rankedMemory, error) {
	if vdb == nil || !vdb.IsReady() || vdb.IsDisabled() {
		return nil, nil
	}
	searchLimit := topK
	if searchLimit > 0 {
		searchLimit *= 3
	}
	memories, docIDs, similarities, err := searchMemoriesOnlyWithScores(ctx, vdb, query, searchLimit)
	if err != nil {
		return nil, err
	}
	if len(memories) == 0 {
		return nil, nil
	}
	ranked, err := rankMemoryCandidatesWithScores(memories, docIDs, similarities, stm, usedDocIDs, now)
	if err != nil {
		return nil, err
	}
	if topK > 0 && len(ranked) > topK {
		ranked = ranked[:topK]
	}
	return ranked, nil
}

func searchMemoriesOnlyWithScores(ctx context.Context, vdb memory.VectorDB, query string, topK int) ([]string, []string, []float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if scored, ok := vdb.(memory.ContextScoredVectorDB); ok {
		results, err := scored.SearchMemoriesOnlyScoredContext(ctx, query, topK)
		if err != nil {
			return nil, nil, nil, err
		}
		return splitScoredMemoryResults(results)
	}
	if scored, ok := vdb.(memory.ScoredVectorDB); ok {
		results, err := scored.SearchMemoriesOnlyScored(query, topK)
		if err != nil {
			return nil, nil, nil, err
		}
		return splitScoredMemoryResults(results)
	}
	if ctxVdb, ok := vdb.(memory.ContextVectorDB); ok {
		memories, docIDs, err := ctxVdb.SearchMemoriesOnlyContext(ctx, query, topK)
		return memories, docIDs, nil, err
	}
	memories, docIDs, err := vdb.SearchMemoriesOnly(query, topK)
	return memories, docIDs, nil, err
}

func splitScoredMemoryResults(results []memory.SearchResult) ([]string, []string, []float64, error) {
	memories := make([]string, 0, len(results))
	docIDs := make([]string, 0, len(results))
	similarities := make([]float64, 0, len(results))
	for _, result := range results {
		memories = append(memories, result.Text)
		docIDs = append(docIDs, result.DocID)
		similarities = append(similarities, result.Similarity)
	}
	return memories, docIDs, similarities, nil
}

func loadMemoryMetaMap(stm *memory.SQLiteMemory, docIDs []string) (metaMap map[string]memory.MemoryMeta, resultErr error) {
	defer func() {
		if resultErr != nil {
			slog.Warn("[RAG] Memory metadata enrichment skipped", "error", resultErr)
		}
	}()
	metaMap = make(map[string]memory.MemoryMeta)
	if stm == nil {
		return nil, fmt.Errorf("memory metadata store is unavailable")
	}
	seen := make(map[string]bool, len(docIDs))
	for _, id := range docIDs {
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("memory result has an empty document ID")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		meta, err := stm.GetMemoryMeta(id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("load memory candidate metadata: %w", err)
		}
		metaMap[id] = meta
	}
	return metaMap, nil
}

func calculateMemoryRankingScore(similarity float64, meta memory.MemoryMeta, reuseCount int, now time.Time) float64 {
	return similarity *
		(1.0 + memoryRecencyBonus(meta, now)) *
		memoryConfidenceMultiplier(meta) *
		memoryReusePenaltyMultiplier(reuseCount)
}

func memoryRecencyBonus(meta memory.MemoryMeta, now time.Time) float64 {
	recencyBonus := 0.0

	if eventTime, err := time.Parse("2006-01-02 15:04:05", meta.LastEventAt); err == nil {
		daysSince := now.Sub(eventTime).Hours() / 24
		if daysSince < 30 {
			recencyBonus += 0.35 * (1.0 - daysSince/30.0)
		}
	}
	if lastAccessed, err := time.Parse("2006-01-02 15:04:05", meta.LastAccessed); err == nil {
		daysSince := now.Sub(lastAccessed).Hours() / 24
		if daysSince < 30 {
			recencyBonus += 0.15 * (1.0 - daysSince/30.0)
		}
	}

	return recencyBonus
}

func memoryConfidenceMultiplier(meta memory.MemoryMeta) float64 {
	extractionConfidence := meta.ExtractionConfidence
	if extractionConfidence <= 0 {
		extractionConfidence = 0.75
	}
	sourceReliability := meta.SourceReliability
	if sourceReliability <= 0 {
		sourceReliability = 0.70
	}

	multiplier := 1.0
	multiplier *= 0.90 + extractionConfidence*0.20
	multiplier *= 0.92 + sourceReliability*0.16

	switch strings.ToLower(strings.TrimSpace(meta.VerificationStatus)) {
	case "confirmed":
		multiplier *= 1.12
	case "contradicted":
		multiplier *= 0.35
	}

	return multiplier
}

func memoryReusePenaltyMultiplier(reuseCount int) float64 {
	if reuseCount <= 0 {
		return 1.0
	}
	penalty := 0.18 * float64(reuseCount)
	if penalty > 0.54 {
		penalty = 0.54
	}
	return 1.0 - penalty
}
