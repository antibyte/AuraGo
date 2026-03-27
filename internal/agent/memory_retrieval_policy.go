package agent

import "sort"

// applySessionMemoryReusePenalty gently downranks memories that have already
// been injected earlier in the same agent loop session. This keeps novel but
// still relevant memories visible without hard-blocking repeats when the
// result set is small or the same memory remains the best match.
func applySessionMemoryReusePenalty(candidates []rankedMemory, usedDocIDs map[string]int) []rankedMemory {
	if len(candidates) == 0 || len(usedDocIDs) == 0 {
		return candidates
	}

	ranked := make([]rankedMemory, len(candidates))
	copy(ranked, candidates)

	for i := range ranked {
		reuseCount := usedDocIDs[ranked[i].docID]
		if reuseCount <= 0 {
			continue
		}
		ranked[i].score = ranked[i].score * memoryReusePenaltyMultiplier(reuseCount)
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	return ranked
}

func markMemoryDocIDsUsed(usedDocIDs map[string]int, candidates []rankedMemory) {
	if len(usedDocIDs) == 0 && len(candidates) == 0 {
		return
	}
	for _, candidate := range candidates {
		if candidate.docID == "" {
			continue
		}
		usedDocIDs[candidate.docID]++
	}
}
