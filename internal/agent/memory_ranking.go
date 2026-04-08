package agent

import (
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/memory"
)

var memoryMetaCacheTTL = 1 * time.Minute
var memoryMetaCacheNow = time.Now

var memoryMetaCache = struct {
	mu       sync.RWMutex
	stm      *memory.SQLiteMemory
	loadedAt time.Time
	data     map[string]memory.MemoryMeta
}{}

// rankMemoryCandidates centralizes the retrieval score calculation for vector memories.
// It combines semantic similarity, recency, confidence/provenance signals, and
// session-local reuse penalties into one consistent score pipeline.
func rankMemoryCandidates(memories []string, docIDs []string, stm *memory.SQLiteMemory, usedDocIDs map[string]int, now time.Time) []rankedMemory {
	metaMap := loadMemoryMetaMap(stm)
	results := make([]rankedMemory, 0, len(memories))

	for i, mem := range memories {
		if i >= len(docIDs) {
			break
		}
		docID := docIDs[i]
		sim := memory.ExtractSimilarityScore(mem)
		if sim == 0 {
			sim = 0.5
		}

		meta, _ := metaMap[docID]
		finalScore := calculateMemoryRankingScore(sim, meta, usedDocIDs[docID], now)
		results = append(results, rankedMemory{text: mem, docID: docID, score: finalScore})
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	return results
}

func loadMemoryMetaMap(stm *memory.SQLiteMemory) map[string]memory.MemoryMeta {
	metaMap := make(map[string]memory.MemoryMeta)
	if stm == nil {
		return metaMap
	}

	now := memoryMetaCacheNow()
	memoryMetaCache.mu.RLock()
	if memoryMetaCache.stm == stm && memoryMetaCache.data != nil && now.Sub(memoryMetaCache.loadedAt) < memoryMetaCacheTTL {
		cached := memoryMetaCache.data
		memoryMetaCache.mu.RUnlock()
		return cached
	}
	memoryMetaCache.mu.RUnlock()

	memoryMetaCache.mu.Lock()
	defer memoryMetaCache.mu.Unlock()
	if memoryMetaCache.stm == stm && memoryMetaCache.data != nil && now.Sub(memoryMetaCache.loadedAt) < memoryMetaCacheTTL {
		return memoryMetaCache.data
	}

	metas, err := stm.GetAllMemoryMeta(50000, 0)
	if err != nil {
		return metaMap
	}
	for _, meta := range metas {
		metaMap[meta.DocID] = meta
	}
	memoryMetaCache.stm = stm
	memoryMetaCache.loadedAt = now
	memoryMetaCache.data = metaMap
	return metaMap
}

func resetMemoryMetaCacheForTests() {
	memoryMetaCache.mu.Lock()
	defer memoryMetaCache.mu.Unlock()
	memoryMetaCache.stm = nil
	memoryMetaCache.loadedAt = time.Time{}
	memoryMetaCache.data = nil
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
