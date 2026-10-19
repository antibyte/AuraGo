package agent

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"aurago/internal/memory"
	"aurago/internal/memory/kgquery"
)

// retrievalFusionResult holds enriched context from cross-referencing RAG (VectorDB)
// and KG (Knowledge Graph) retrieval results.
type retrievalFusionResult struct {
	// EnrichedMemories contains additional LTM memories discovered via KG entity labels.
	EnrichedMemories []rankedMemory
	// EnrichedKGContext contains additional KG entities discovered via RAG memory text.
	EnrichedKGContexts []memory.KnowledgeGraphContextResult
}

// Fusion budget constants — strict limits to prevent context explosion.
const (
	// fusionMaxEntities limits how many entity labels are used as LTM queries (KG→RAG).
	fusionMaxEntities = 3
	// fusionMaxRAGQueries limits how many RAG memories are used as KG queries (RAG→KG).
	fusionMaxRAGQueries = 2
	// fusionCharBudget is the strict character budget per direction.
	fusionCharBudget = 400
	// fusionOnDemandCharBudget preserves the smaller historical on-demand fusion cap.
	fusionOnDemandCharBudget = 260
)

// entityLinePattern matches KG context lines like "- [entity_id] label | prop: val".
var entityLinePattern = regexp.MustCompile(`^-\s+\[([^\]]+)\]\s*(.*)`)
var knowledgeEdgeLinePattern = regexp.MustCompile(`^\s+- \[([^\]]+)\] .* -\[([^\]]+)\]-> \[([^\]]+)\]`)
var memorySimilarityPrefixPattern = regexp.MustCompile(`(?i)^\s*\[similarity:\s*[0-9.]+\]\s*`)
var memoryMetadataPrefixPattern = regexp.MustCompile(`(?i)^\s*\[(domain:\s*[^\]]+|tool_guides|documentation|file_index|trust:\s*(agent-extracted|agent-recorded|user-recorded|user-confirmed|system-recorded|legacy-unknown))\]\s*`)

// applyRetrievalFusion enriches RAG and KG context by cross-referencing results from
// both subsystems. When strong RAG hits exist, related KG entities are loaded (RAG→KG).
// When strong KG hits exist, related VectorDB memories are searched (KG→RAG).
//
// The fusion is budget-limited and only activates when both subsystems produced results.
// This creates a bidirectional enrichment that improves context quality without exploding
// the prompt size. The LTM searches run under the caller's turn context, so a
// cancelled turn stops searching instead of finishing every entity query.
func applyRetrievalFusion(
	ctx context.Context,
	topMemories []rankedMemory,
	kgContext string,
	longTermMem memory.VectorDB,
	stm *memory.SQLiteMemory,
	kg *memory.KnowledgeGraph,
	logger *slog.Logger,
) retrievalFusionResult {
	return applyRetrievalFusionWithBudget(ctx, topMemories, kgContext, longTermMem, stm, kg, logger, fusionCharBudget)
}

func applyRetrievalFusionWithBudget(
	ctx context.Context,
	topMemories []rankedMemory,
	kgContext string,
	longTermMem memory.VectorDB,
	stm *memory.SQLiteMemory,
	kg *memory.KnowledgeGraph,
	logger *slog.Logger,
	maxChars int,
) retrievalFusionResult {
	result := retrievalFusionResult{}
	if maxChars <= 0 {
		maxChars = fusionCharBudget
	}
	hasRAG := len(topMemories) > 0
	hasKG := strings.TrimSpace(kgContext) != ""

	if !hasRAG && !hasKG {
		return result
	}

	// Direction 1: KG→RAG — extract entity labels from KG context, search LTM.
	// This finds long-term memories related to the knowledge graph entities.
	if hasKG && longTermMem != nil && !longTermMem.IsDisabled() {
		labels := extractKGEntityLabels(kgContext, fusionMaxEntities)
		if len(labels) > 0 {
			var extraMemories []rankedMemory
			for _, label := range labels {
				ranked, err := searchRankedMemoriesOnly(ctx, longTermMem, stm, label, 1, nil, time.Now())
				if ctx.Err() != nil {
					break
				}
				if err != nil && logger != nil {
					logger.Warn("[RAG] Related memory search incomplete", "error", err)
				}
				if len(ranked) == 0 {
					continue
				}
				if strings.TrimSpace(ranked[0].docID) == "" {
					continue
				}
				// Deduplicate against existing top memories and already-found extras.
				if containsEquivalentRankedMemory(topMemories, ranked[0]) || containsEquivalentRankedMemory(extraMemories, ranked[0]) {
					continue
				}
				if strings.TrimSpace(ranked[0].text) == "" {
					continue
				}
				extraMemories = append(extraMemories, ranked[0])
				if len(extraMemories) >= fusionMaxEntities {
					break
				}
			}
			result.EnrichedMemories = extraMemories
		}
	}

	// Direction 2: RAG→KG — use top RAG memories as queries for additional KG context.
	// This discovers knowledge graph entities that are semantically related to retrieved memories.
	if hasRAG && kg != nil {
		var extraKGResults []memory.KnowledgeGraphContextResult
		limit := len(topMemories)
		if limit > fusionMaxRAGQueries {
			limit = fusionMaxRAGQueries
		}
		for i := 0; i < limit; i++ {
			// Search with the complete retrieved memory. Do not byte-slice a fact or
			// turn it into an arbitrary substring that can change its meaning.
			query := strings.TrimSpace(sanitizeMemoryForPrompt(topMemories[i].text))
			if query == "" {
				continue
			}
			kgResult := kg.SearchForContextStructured(query, 2, maxChars)
			if ctx.Err() != nil {
				break
			}
			if len(kgResult.Nodes) == 0 {
				continue
			}
			extraKGResults = append(extraKGResults, kgResult)
		}
		result.EnrichedKGContexts = dedupeKGContextResults(kgContext, extraKGResults)
	}

	if logger != nil && (len(result.EnrichedMemories) > 0 || len(result.EnrichedKGContexts) > 0) {
		logger.Debug("[RetrievalFusion] Enriched context",
			"ltm_via_kg", len(result.EnrichedMemories) > 0,
			"kg_via_ltm", len(result.EnrichedKGContexts) > 0,
			"memories", len(result.EnrichedMemories),
			"kg_results", len(result.EnrichedKGContexts),
		)
	}

	return result
}

func dedupeKGContextResults(baseContext string, results []memory.KnowledgeGraphContextResult) []memory.KnowledgeGraphContextResult {
	seenNodes := knowledgeContextIDs(baseContext)
	seenEdges := knowledgeContextEdgeIDs(baseContext)
	var deduped []memory.KnowledgeGraphContextResult
	for _, result := range results {
		for index, node := range result.Nodes {
			if strings.TrimSpace(node.ID) == "" {
				continue
			}
			if _, exists := seenNodes[node.ID]; exists {
				continue
			}
			single := result.LimitNodes(index, 1)
			filteredEdges := make([]memory.Edge, 0, len(result.EdgesByNodeID[node.ID]))
			for _, edge := range result.EdgesByNodeID[node.ID] {
				key := knowledgeEdgeIdentity(edge)
				if _, exists := seenEdges[key]; exists {
					continue
				}
				seenEdges[key] = struct{}{}
				filteredEdges = append(filteredEdges, edge)
			}
			single.EdgesByNodeID = map[string][]memory.Edge{node.ID: filteredEdges}
			deduped = append(deduped, single)
			seenNodes[node.ID] = struct{}{}
		}
	}
	return deduped
}

func knowledgeContextEdgeIDs(context string) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, line := range strings.Split(context, "\n") {
		matches := knowledgeEdgeLinePattern.FindStringSubmatch(line)
		if len(matches) == 4 {
			ids[matches[1]+"\x00"+matches[2]+"\x00"+matches[3]] = struct{}{}
		}
	}
	return ids
}

func knowledgeEdgeIdentity(edge memory.Edge) string {
	return edge.Source + "\x00" + edge.Relation + "\x00" + edge.Target
}

func formatFusionMemoriesForPrompt(memories []rankedMemory, maxBytes int) string {
	entries := make([]string, 0, len(memories))
	unreferenceableOmitted := false
	for _, item := range memories {
		text := sanitizeMemoryForPrompt(item.text)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if len(text) > maxBytes {
			if strings.TrimSpace(item.docID) == "" {
				unreferenceableOmitted = true
				continue
			}
			text = "[memory:" + item.docID + "] (content omitted; use recall_memory)"
		}
		entries = append(entries, text)
	}
	if unreferenceableOmitted {
		entries = append(entries, "[memory omitted: no stable recall ID]")
	}
	return kgquery.FitEntries(entries, "\n---\n", maxBytes)
}

func formatFusionKGContextsForPrompt(results []memory.KnowledgeGraphContextResult, maxChars int) string {
	entries := make([]string, 0, len(results))
	for _, result := range results {
		if formatted := result.FormatContext(maxChars); formatted != "" {
			entries = append(entries, formatted)
		}
	}
	return kgquery.FitEntries(entries, "\n", maxChars)
}

func formatFusionKGIndexes(results []memory.KnowledgeGraphContextResult, maxChars int) string {
	var entries []string
	for _, result := range results {
		index := result.AvailableIndex(maxChars)
		if strings.TrimSpace(index) != "" {
			entries = append(entries, strings.Split(index, "\n")...)
		}
	}
	return kgquery.FitEntries(entries, "\n", maxChars)
}

func knowledgeContextIDs(context string) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, line := range strings.Split(context, "\n") {
		matches := entityLinePattern.FindStringSubmatch(line)
		if len(matches) == 3 && !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "\t") {
			ids[matches[1]] = struct{}{}
		}
	}
	return ids
}

func containsEquivalentRankedMemory(memories []rankedMemory, candidate rankedMemory) bool {
	for _, item := range memories {
		if candidate.docID != "" && item.docID == candidate.docID {
			return true
		}
		if retrievalMemoryDedupKey(candidate.text) != "" && retrievalMemoryDedupKey(candidate.text) == retrievalMemoryDedupKey(item.text) {
			return true
		}
	}
	return false
}

// extractKGEntityLabels parses entity labels from the formatted KG context string.
// The format produced by KnowledgeGraph.SearchForContext is:
//
//   - [entity_id] label | prop1: val1 | prop2: val2
//   - [src] -[relation]-> [tgt]
func extractKGEntityLabels(kgContext string, maxLabels int) []string {
	var labels []string
	lines := strings.Split(kgContext, "\n")
	for _, line := range lines {
		matches := entityLinePattern.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		// Skip indented lines (edge lines like "  - [src] -[rel]-> [tgt]").
		if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
			continue
		}
		label := strings.TrimSpace(matches[2])
		if label == "" {
			continue
		}
		// Label is everything before the first " | ".
		if idx := strings.Index(label, " | "); idx > 0 {
			label = label[:idx]
		}
		label = strings.TrimSpace(label)
		if label != "" && label != "Unknown" && len(label) >= 2 {
			labels = append(labels, label)
		}
		if len(labels) >= maxLabels {
			break
		}
	}
	return labels
}

// containsString checks if a string exists in a slice.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func containsEquivalentMemory(slice []string, s string) bool {
	key := retrievalMemoryDedupKey(s)
	if key == "" {
		return containsString(slice, s)
	}
	for _, item := range slice {
		if retrievalMemoryDedupKey(item) == key {
			return true
		}
	}
	return false
}

func retrievalMemoryDedupKey(text string) string {
	text = sanitizeMemoryForPrompt(text)
	for {
		trimmed := strings.TrimSpace(text)
		trimmed = memorySimilarityPrefixPattern.ReplaceAllString(trimmed, "")
		next := memoryMetadataPrefixPattern.ReplaceAllString(trimmed, "")
		if next == text {
			text = next
			break
		}
		text = next
	}
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.TrimSuffix(text, "...")
	text = strings.TrimSuffix(text, "…")
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	key := strings.Join(fields, " ")
	runes := []rune(key)
	if len(runes) > 180 {
		key = string(runes[:180])
	}
	return key
}

// truncateUTF8SafeAgent truncates a string to at most maxLen runes, breaking at
// the last newline boundary if possible.
func truncateUTF8SafeAgent(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	truncated := string(runes[:maxLen])
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated
}
