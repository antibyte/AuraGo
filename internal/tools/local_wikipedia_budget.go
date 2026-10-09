package tools

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"aurago/internal/localwiki"
	"aurago/internal/security"
)

// local_wikipedia answers are sized to the agent's inline budget: a result
// above agent.output_compression.reversible.max_inline_chars (measured in
// bytes) is archived in the output vault and the model sees only its start.
const (
	// localWikipediaDefaultBudget is the budget in bytes when the caller passes
	// none: the agent's default inline limit.
	localWikipediaDefaultBudget = 6000
	// localWikipediaMinBudget keeps answers useful under a tiny inline limit;
	// the agent then archives them like any other large tool output.
	localWikipediaMinBudget = 2000
	// localWikipediaBudgetReserve stays free for what the dispatch may add
	// after the tool: a Guardian warning, redaction markers, the text-mode
	// "[Tool Output]" prefix.
	localWikipediaBudgetReserve = 400

	localWikipediaLeadRunes       = 2000 // the library's lead cap
	localWikipediaMinLeadRunes    = 300
	localWikipediaSnippetRunes    = 200 // the library's snippet cap
	localWikipediaMinSnippetRunes = 60
	localWikipediaPageRunes       = 8000 // the library's page cap
	localWikipediaMinPageRunes    = 100  // the library's smallest page
	// localWikipediaReadAttempts bounds the re-reads with a smaller page.
	localWikipediaReadAttempts = 6
)

// localWikipediaBudget turns the caller's inline limit (bytes; 0 = default)
// into the budget an answer must fit.
func localWikipediaBudget(limit int) int {
	if limit <= 0 {
		limit = localWikipediaDefaultBudget
	}
	return max(limit, localWikipediaMinBudget) - localWikipediaBudgetReserve
}

// localWikipediaModelBytes is the size of an answer as the agent compares it
// with its inline limit (len in bytes, agent.maybeStorePrimaryToolOutputVault)
// after the dispatch pipeline (agent.DispatchToolCallResult): StripThinkingTags
// unwraps the per-field isolation of an answer that is not isolated as a
// whole, then the Guardian isolates the whole answer once more and
// HTML-escapes it (a JSON quote becomes &#34;, an escaped &amp; becomes
// &amp;amp;). Multi-byte scripts count 2-4 bytes per character.
func localWikipediaModelBytes(out string) int {
	return len(security.IsolateExternalData(security.StripThinkingTags(out)))
}

// localWikipediaLargest returns the largest n in [lo, hi] for which fits is
// true (fits grows false with n); ok is false when none is.
func localWikipediaLargest(lo, hi int, fits func(int) bool) (int, bool) {
	best, ok := 0, false
	for lo <= hi {
		mid := lo + (hi-lo+1)/2
		if fits(mid) {
			best, ok = mid, true
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best, ok
}

// localWikipediaClip shortens text to at most n runes (n >= 2), cutting after
// the last space in the second half when there is one, and marks the cut
// with "…".
func localWikipediaClip(text string, n int) string {
	if n <= 0 || utf8.RuneCountInString(text) <= n {
		return text
	}
	runes := []rune(text)[:max(n-1, 1)]
	cut := len(runes)
	for i := len(runes) - 1; i >= len(runes)/2; i-- {
		if unicode.IsSpace(runes[i]) {
			cut = i
			break
		}
	}
	return strings.TrimRightFunc(string(runes[:cut]), unicode.IsSpace) + "…"
}

// localWikipediaSearchShape says how much of a search result an answer shows.
type localWikipediaSearchShape struct {
	hits, leads, leadRunes, snippetRunes int
}

func localWikipediaSearchAnswer(base localWikipediaSearchOutput, hits []localwiki.SearchHit, shape localWikipediaSearchShape) string {
	out := base
	out.Results = make([]localWikipediaHit, 0, shape.hits)
	for i, hit := range hits[:shape.hits] {
		lead := ""
		if i < shape.leads {
			lead = localWikipediaClip(hit.Lead, shape.leadRunes)
		}
		out.Results = append(out.Results, localWikipediaHit{
			Title:   security.IsolateExternalData(hit.Title),
			Path:    security.IsolateExternalData(hit.Path),
			Snippet: security.IsolateExternalData(localWikipediaClip(hit.Snippet, shape.snippetRunes)),
			Lead:    security.IsolateExternalData(lead),
		})
	}
	return localWikipediaJSON(out)
}

// localWikipediaFitSearch builds the largest search answer that fits budget:
// first shorter leads, then fewer leads, then shorter snippets, then fewer
// results (the first result always stays).
func localWikipediaFitSearch(base localWikipediaSearchOutput, hits []localwiki.SearchHit, budget int) string {
	shape := localWikipediaSearchShape{
		hits:         len(hits),
		leads:        min(len(hits), localWikipediaLeads),
		leadRunes:    localWikipediaLeadRunes,
		snippetRunes: localWikipediaSnippetRunes,
	}
	answer := func(s localWikipediaSearchShape) string { return localWikipediaSearchAnswer(base, hits, s) }
	fits := func(s localWikipediaSearchShape) bool { return localWikipediaModelBytes(answer(s)) <= budget }
	if fits(shape) {
		return answer(shape)
	}
	for ; shape.leads > 0; shape.leads-- {
		if n, ok := localWikipediaLargest(localWikipediaMinLeadRunes, localWikipediaLeadRunes, func(n int) bool {
			s := shape
			s.leadRunes = n
			return fits(s)
		}); ok {
			shape.leadRunes = n
			return answer(shape)
		}
	}
	if fits(shape) || len(hits) == 0 {
		return answer(shape)
	}
	if n, ok := localWikipediaLargest(localWikipediaMinSnippetRunes, localWikipediaSnippetRunes, func(n int) bool {
		s := shape
		s.snippetRunes = n
		return fits(s)
	}); ok {
		shape.snippetRunes = n
		return answer(shape)
	}
	shape.snippetRunes = localWikipediaMinSnippetRunes
	n, _ := localWikipediaLargest(1, shape.hits, func(n int) bool {
		s := shape
		s.hits = n
		return fits(s)
	})
	shape.hits = max(n, 1)
	return answer(shape)
}

// localWikipediaFitSections returns how many sections (at most shown) an
// answer can list so that answer(n) stays within limit; at least 0.
func localWikipediaFitSections(shown int, limit int, answer func(n int) string) int {
	if localWikipediaModelBytes(answer(shown)) <= limit {
		return shown
	}
	n, _ := localWikipediaLargest(0, shown-1, func(n int) bool {
		return localWikipediaModelBytes(answer(n)) <= limit
	})
	return n
}
