package agent

import "slices"

// toolSearchLeaderMargin is how far ahead of the next lexical hit the lexical
// leader must be for the lexical ranking to keep the first place against the
// semantic one. It equals toolSearchPhraseBoost, the largest single multiplier
// in the lexical scorer: a lead larger than one phrase boost is a lexical
// verdict, not a tie. Reciprocal rank fusion sees ranks only, never this
// distance, which is what the guard adds back.
const toolSearchLeaderMargin = toolSearchPhraseBoost

// holdClearLexicalLeader gives a clear lexical leader the first place after the
// pinned names. fused is the result of fuseToolRanking, pinnedCount the number
// of exact-tier names at its head and hits the ranking rankedSearch returned.
//
// The leader is the first lexical-tier hit in that order. It is clear when its
// score is at least toolSearchLeaderMargin times the highest score among all
// other lexical-tier hits, whatever their status class, or when no other
// lexical-tier hit exists. A clear leader that the fusion placed later moves to
// fused[pinnedCount]; every other tool keeps its relative order. A leader that
// is not clear, one that already holds the place and one the fusion dropped
// leave fused as it is, so the semantic ranking still breaks near-ties.
//
// The input is never changed; a moved leader comes back in a new slice.
func holdClearLexicalLeader(fused []string, pinnedCount int, hits []toolSearchHit) []string {
	if pinnedCount < 0 || pinnedCount >= len(fused) {
		return fused
	}
	var leader *toolSearchHit
	runnerUp := 0.0
	for i := range hits {
		hit := &hits[i]
		if hit.Tier != toolSearchTierLexical {
			continue
		}
		if leader == nil {
			leader = hit
			continue
		}
		runnerUp = max(runnerUp, hit.Score)
	}
	if leader == nil || leader.Entry == nil || leader.Score < toolSearchLeaderMargin*runnerUp {
		return fused
	}

	at := slices.Index(fused[pinnedCount:], leader.Entry.Name)
	if at <= 0 { // missing, or already first after the pinned names
		return fused
	}
	at += pinnedCount
	out := make([]string, len(fused))
	copy(out, fused[:pinnedCount])
	out[pinnedCount] = fused[at]
	copy(out[pinnedCount+1:], fused[pinnedCount:at])
	copy(out[at+1:], fused[at+1:])
	return out
}
