package agent

import (
	"log/slog"
	"slices"
	"time"

	"aurago/internal/prompts"
	openai "github.com/sashabaranov/go-openai"
)

// Weights of the signals that join the lexical and semantic ranking in the
// adaptive first selection (fuseToolRanking, K 20). Lexical hits weigh 1 and
// manual matches 0.8 (adaptiveToolFusion). The tools a request names outright
// weigh a little less than a manual match; family seeds and the user's own
// usage only decide among tools that the text of the request does not
// separate, so the seeds sit below the intent matches and usage below both. A
// seed at the first position scores like a lexical hit at position 22, a
// usage-only tool like one at position 50.
const (
	adaptiveIntentWeight = 0.7
	adaptiveSeedWeight   = 0.5
	adaptiveUsageWeight  = 0.3
	// adaptiveUsageMaxAdded bounds the tools that usage alone puts in the
	// fused head; the rest of the usage list only fills slots that stay free.
	adaptiveUsageMaxAdded = 4
	// adaptiveLeaderPlaces is how far down the signals may push the best
	// lexical hit: it stays among the first this many tools behind the exact
	// matches. A tool that only one signal ranks never passes it (every signal
	// weighs less than a lexical hit), but tools that several signals and the
	// text rank together can; this keeps the ranking from burying the best
	// match of the text under them.
	adaptiveLeaderPlaces = 3
)

// adaptivePriorityInput is everything the adaptive ranking looks at.
type adaptivePriorityInput struct {
	// Schemas are the tools the selection can offer.
	Schemas []openai.Tool
	// Usage lists the tools the user relies on, most relied-on first.
	Usage []string
	// Query is the user message.
	Query  string
	Guides toolGuideSearcher
	Logger *slog.Logger
	// Held are tools the selection includes anyway (hard and soft ones). They
	// still rank, but a family seed does not spend one of its places on them.
	Held []string
}

// buildAdaptiveToolPriority ranks the offered tools for a request, best first.
// See rankAdaptiveTools.
func buildAdaptiveToolPriority(schemas []openai.Tool, weightedUsage []string, userQuery string, guideSearcher toolGuideSearcher, logger *slog.Logger) []string {
	return rankAdaptiveTools(adaptivePriorityInput{Schemas: schemas, Usage: weightedUsage, Query: userQuery, Guides: guideSearcher, Logger: logger})
}

// rankAdaptiveTools orders the offered tools for a request:
//
//   - exact matches of a tool name, alias or operation come first;
//   - then one fused ranking (fuseToolRanking) of the lexical search, the
//     semantic manual search and three weighted signals: the tools whose name
//     the request spells out, the seeds of its family and cues, and the
//     tools the user relies on (at most adaptiveUsageMaxAdded of them that no
//     other signal found);
//   - then the curated neighbours of everything ranked so far;
//   - then the rest of the usage list, which fills the slots that stay free.
//
// The adaptiveIntentOnlyTools take no part in any of it: a signal drops them
// before it counts positions, so enabling one never moves another tool.
func rankAdaptiveTools(in adaptivePriorityInput) []string {
	// The whole user message arrives here, up to a megabyte; the ranking, the
	// intent matchers and the semantic search all work on its head and tail.
	userQuery := boundToolSearchQuery(in.Query)
	available := make([]string, 0, len(in.Schemas))
	availableSet := make(map[string]bool, len(in.Schemas))
	for _, schema := range in.Schemas {
		if schema.Function == nil || schema.Function.Name == "" {
			continue
		}
		name := schema.Function.Name
		if !availableSet[name] {
			available = append(available, name)
			availableSet[name] = true
		}
	}
	// rankable are the tools a signal may rank.
	rankable := make([]string, 0, len(available))
	rankableSet := make(map[string]bool, len(available))
	for _, name := range available {
		if !slices.Contains(adaptiveIntentOnlyTools, name) {
			rankable = append(rankable, name)
			rankableSet[name] = true
		}
	}

	prioritized := make([]string, 0, len(in.Usage)+8)
	seen := make(map[string]bool, len(rankableSet))
	add := func(name string) {
		if name == "" || seen[name] || !rankableSet[name] {
			return
		}
		seen[name] = true
		prioritized = append(prioritized, name)
	}

	// Explicit discovery and adaptive selection share names, aliases and
	// operations. The selection ranks only what it can offer, so the catalog
	// leaves out the disabled placeholders. Exact matches come first, then the
	// lexical ranking and the semantic manual ranking are fused with the
	// signals (fuseToolRanking). The adaptiveIntentOnlyTools take no part in
	// the ranking and their manuals use no semantic slot.
	catalog := newToolCatalog(in.Schemas, in.Schemas, "", false)
	var pinned, lexical []string
	for _, hit := range catalog.rankedSearch(userQuery) {
		if slices.Contains(adaptiveIntentOnlyTools, hit.Entry.Name) {
			continue
		}
		if hit.Tier > toolSearchTierLexical {
			pinned = append(pinned, hit.Entry.Name)
		} else {
			lexical = append(lexical, hit.Entry.Name)
		}
	}
	var manuals []string
	members := map[string][]string{}
	if in.Guides != nil && discoverySemanticQueryAllowed(userQuery) {
		// A manual can document several tools (email, mqtt, yepapi_*), so its
		// name maps to every available tool of that family; a manual named like
		// an available tool documents that tool.
		byManual := make(map[string][]string, len(available))
		for _, tool := range available {
			manual := prompts.ToolManualID(tool)
			byManual[manual] = append(byManual[manual], tool)
		}
		cfg := adaptiveToolFusion
		for _, path := range searchToolGuidesWithTimeout(in.Guides, userQuery, cfg.ManualWindow+len(prompts.NonDisplacingTools), 400*time.Millisecond, in.Logger) {
			manual := manualIDFromPath(path)
			manuals = append(manuals, manual)
			if family := byManual[manual]; len(family) > 0 {
				members[manual] = family
			} else if availableSet[manual] {
				members[manual] = []string{manual}
			}
		}
	}

	held := stringSet(in.Held)
	signals := []toolRankSignal{
		{Names: extractIntentMatchedTools(userQuery, rankable), Weight: adaptiveIntentWeight},
		{Names: adaptiveSeedsForQuery(userQuery, func(name string) bool { return rankableSet[name] && !held[name] }), Weight: adaptiveSeedWeight},
		{Names: rankableNames(in.Usage, rankableSet), Weight: adaptiveUsageWeight, MaxAdded: adaptiveUsageMaxAdded},
	}
	fused := fuseToolRanking(pinned, lexical, manuals, members, prompts.IsNonDisplacingManual, adaptiveToolFusion, signals...)
	if len(lexical) > 0 {
		fused = keepLexicalLeader(fused, len(pinned), lexical[0], adaptiveLeaderPlaces)
	}
	for _, name := range fused {
		add(name)
	}

	headCount := len(prioritized)
	for i := 0; i < headCount; i++ {
		for _, neighbor := range adaptiveToolNeighbors[prioritized[i]] {
			add(neighbor)
		}
	}

	for _, tool := range in.Usage {
		add(tool)
	}

	return prioritized
}

// keepLexicalLeader moves the best lexical hit up when the fused order put it
// further down than the first places tools behind the exact matches.
func keepLexicalLeader(order []string, exact int, leader string, places int) []string {
	at := slices.Index(order, leader)
	limit := exact + places - 1
	if at <= limit || limit >= len(order) {
		return order
	}
	out := slices.Delete(slices.Clone(order), at, at+1)
	return slices.Insert(out, limit, leader)
}

// rankableNames keeps the names a signal may rank, in order and without
// duplicates.
func rankableNames(names []string, rankable map[string]bool) []string {
	out := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if rankable[name] && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}
