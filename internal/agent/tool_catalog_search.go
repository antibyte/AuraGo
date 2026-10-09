package agent

import (
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"context"
	"path/filepath"
	"strings"
	"time"
)

type discoveryGuideSearcher interface {
	SearchToolGuideMatchesContext(context.Context, string, int) ([]memory.ToolGuideMatch, error)
}

// SearchContext retains exact lexical precedence and augments incomplete task
// matches with the existing manual index. Unavailable embeddings fail closed to
// deterministic lexical discovery without performing a new indexing operation.
func (c *ToolCatalog) SearchContext(ctx context.Context, query string, source memory.VectorDB) []*ToolCatalogEntry {
	lexical := c.Search(query)
	searcher, ok := source.(discoveryGuideSearcher)
	if !ok || len(lexical) >= 5 || ctx.Err() != nil {
		return lexical
	}
	bounded, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()
	// A non-displacing tool that is not enabled loses its manual before the
	// cut to the top five, so it never costs another tool its slot.
	const semanticTopK = 5
	matches, err := searcher.SearchToolGuideMatchesContext(bounded, query, semanticTopK+len(prompts.NonDisplacingTools))
	if err != nil {
		return lexical
	}
	seen := map[string]bool{}
	for _, entry := range lexical {
		seen[entry.Name] = true
	}
	considered := 0
	for _, match := range matches {
		manual := strings.TrimSuffix(filepath.Base(match.Path), ".md")
		if prompts.IsNonDisplacingManual(manual) && !c.enabledManual(manual) {
			continue
		}
		if considered == semanticTopK {
			break
		}
		considered++
		for _, entry := range c.Entries() {
			if !seen[entry.Name] && prompts.ToolManualID(entry.Name) == manual {
				lexical = append(lexical, entry)
				seen[entry.Name] = true
			}
		}
	}
	return lexical
}

// enabledManual reports whether an enabled catalog entry uses the manual.
func (c *ToolCatalog) enabledManual(manual string) bool {
	for _, entry := range c.Entries() {
		if entry.Enabled && prompts.ToolManualID(entry.Name) == manual {
			return true
		}
	}
	return false
}
