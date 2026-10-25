package agent

import (
	"aurago/internal/memory"
	"aurago/internal/prompts"
	"context"
	"sort"
	"strings"
	"time"
)

type discoveryGuideSearcher interface {
	SearchToolGuideMatchesContext(context.Context, string, int) ([]memory.ToolGuideMatch, error)
}

// discoverySemanticTimeout bounds the manual search that augments a search.
const discoverySemanticTimeout = 400 * time.Millisecond

// SearchContext ranks like Search and fuses the result with the existing
// manual index (fuseToolRanking), so a query that shares no word with a tool
// still finds the tool its manual describes. Exact name, alias and operation
// matches stay first. Unavailable embeddings fail closed to the deterministic
// lexical result without performing a new indexing operation.
func (c *ToolCatalog) SearchContext(ctx context.Context, query string, source memory.VectorDB) []*ToolCatalogEntry {
	hits := c.rankedSearch(query)
	searcher, ok := source.(discoveryGuideSearcher)
	if !ok || c == nil || strings.TrimSpace(query) == "" || ctx.Err() != nil {
		return hitEntries(hits)
	}
	bounded, cancel := context.WithTimeout(ctx, discoverySemanticTimeout)
	defer cancel()
	cfg := discoveryToolFusion
	// A non-displacing tool that is not enabled loses its manual before the
	// cut to the window, so it never costs another tool its place.
	matches, err := searcher.SearchToolGuideMatchesContext(bounded, query, cfg.ManualWindow+len(prompts.NonDisplacingTools))
	if err != nil {
		return hitEntries(hits)
	}

	index := c.searchIndex()
	var pinned, lexical, manuals []string
	for _, hit := range hits {
		if hit.Tier > toolSearchTierLexical {
			pinned = append(pinned, hit.Entry.Name)
		} else {
			lexical = append(lexical, hit.Entry.Name)
		}
	}
	members := make(map[string][]string, len(matches))
	for _, match := range matches {
		manual := manualIDFromPath(match.Path)
		manuals = append(manuals, manual)
		if _, done := members[manual]; !done {
			for _, entry := range index.members[manual] {
				members[manual] = append(members[manual], entry.Name)
			}
		}
	}
	skip := func(manual string) bool { return prompts.IsNonDisplacingManual(manual) && !c.enabledManual(manual) }
	fused := fuseToolRanking(pinned, lexical, manuals, members, skip, cfg)

	out := make([]*ToolCatalogEntry, 0, len(fused))
	for _, name := range fused {
		if entry := c.entries[name]; entry != nil {
			out = append(out, entry)
		}
	}
	// The fusion mixes the status classes; a tool that can be called now
	// still comes before one that needs setup or is disabled.
	rest := out[len(pinned):]
	sort.SliceStable(rest, func(i, j int) bool { return toolSearchStatusClass(rest[i]) < toolSearchStatusClass(rest[j]) })
	return out
}

// enabledManual reports whether an enabled catalog entry uses the manual.
func (c *ToolCatalog) enabledManual(manual string) bool {
	for _, entry := range c.searchIndex().members[manual] {
		if entry.Enabled {
			return true
		}
	}
	return false
}
