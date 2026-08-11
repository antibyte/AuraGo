package prompts

import (
	"context"
	"strings"
	"unicode/utf8"
)

func hardTruncateToBudget(prompt string, budget int, model string) string {
	result, _ := hardTruncateToBudgetContext(context.Background(), prompt, budget, model)
	return result
}

func hardTruncateToBudgetContext(ctx context.Context, prompt string, budget int, model string) (string, error) {
	ctx = normalizePromptContext(ctx)
	if err := promptContextErr(ctx); err != nil {
		return "", err
	}
	if budget <= 0 || prompt == "" {
		return "", nil
	}
	if countTokensWithModelContext(ctx, prompt, model) <= budget {
		return prompt, nil
	}
	result, _, err := hardTruncateKnownOverflowContext(ctx, prompt, budget, model)
	return result, err
}

// hardTruncateKnownOverflowContext trims a document whose exact full token
// count is already known to exceed budget. It excludes the full input from the
// binary search, preserving the prompt builder's two-full-tokenization bound.
func hardTruncateKnownOverflowContext(ctx context.Context, prompt string, budget int, model string) (string, int, error) {
	ctx = normalizePromptContext(ctx)
	if err := promptContextErr(ctx); err != nil {
		return "", 0, err
	}
	if budget <= 0 || prompt == "" {
		return "", 0, nil
	}

	// Tokenizers operate on UTF-8 bytes, but provider requests must remain valid
	// UTF-8. The binary search below therefore considers only rune boundaries
	// and still validates the exact token count for every candidate.
	bytes := []byte(prompt)
	marker := "\n\n[BUDGET TRUNCATED]"

	bestWithMarker, bestTokens, ok, err := longestBytePrefixWithinBudgetDetailedContext(ctx, bytes, marker, budget, model, false)
	if err != nil {
		return "", 0, err
	}
	if ok {
		return bestWithMarker, bestTokens, nil
	}

	bestWithoutMarker, bestTokens, ok, err := longestBytePrefixWithinBudgetDetailedContext(ctx, bytes, "", budget, model, false)
	if err != nil {
		return "", 0, err
	}
	if ok {
		return bestWithoutMarker, bestTokens, nil
	}

	markerTokens := countTokensWithModelContext(ctx, marker, model)
	if markerTokens <= budget {
		return marker, markerTokens, nil
	}
	return "", 0, nil
}

// longestBytePrefixWithinBudget performs a binary search over valid UTF-8 rune
// boundaries, appending suffix, and returns the longest fitting candidate.
func longestBytePrefixWithinBudget(data []byte, suffix string, budget int, model string) (string, bool) {
	result, ok, _ := longestBytePrefixWithinBudgetContext(context.Background(), data, suffix, budget, model)
	return result, ok
}

func longestBytePrefixWithinBudgetContext(ctx context.Context, data []byte, suffix string, budget int, model string) (string, bool, error) {
	result, _, ok, err := longestBytePrefixWithinBudgetDetailedContext(ctx, data, suffix, budget, model, true)
	return result, ok, err
}

func longestBytePrefixWithinBudgetDetailedContext(ctx context.Context, data []byte, suffix string, budget int, model string, includeFull bool) (string, int, bool, error) {
	ctx = normalizePromptContext(ctx)
	if err := promptContextErr(ctx); err != nil {
		return "", 0, false, err
	}
	valid := strings.ToValidUTF8(string(data), "�")
	boundaries := make([]int, 1, utf8.RuneCountInString(valid)+1)
	boundaries[0] = 0
	for offset := 0; offset < len(valid); {
		_, size := utf8.DecodeRuneInString(valid[offset:])
		if size <= 0 {
			break
		}
		offset += size
		boundaries = append(boundaries, offset)
	}
	lo, hi := 0, len(boundaries)-1
	if !includeFull && hi > 0 {
		hi--
	}
	best := ""
	bestTokens := 0
	found := false

	for lo <= hi {
		if err := promptContextErr(ctx); err != nil {
			return "", 0, false, err
		}
		mid := (lo + hi) / 2
		candidate := valid[:boundaries[mid]] + suffix
		candidateTokens := countTokensWithModelContext(ctx, candidate, model)
		if candidateTokens <= budget {
			best = candidate
			bestTokens = candidateTokens
			found = true
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}

	return best, bestTokens, found, nil
}
