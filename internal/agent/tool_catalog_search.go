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
	matches, err := searcher.SearchToolGuideMatchesContext(bounded, query, 5)
	if err != nil {
		return lexical
	}
	seen := map[string]bool{}
	for _, entry := range lexical {
		seen[entry.Name] = true
	}
	for _, match := range matches {
		manual := strings.TrimSuffix(filepath.Base(match.Path), ".md")
		for _, entry := range c.Entries() {
			if !seen[entry.Name] && prompts.ToolManualID(entry.Name) == manual {
				lexical = append(lexical, entry)
				seen[entry.Name] = true
			}
		}
	}
	return lexical
}
