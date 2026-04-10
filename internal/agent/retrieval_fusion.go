package agent

import (
	"log/slog"
	"regexp"
	"strings"

	"aurago/internal/memory"
)

// retrievalFusionResult holds enriched context from cross-referencing RAG (VectorDB)
// and KG (Knowledge Graph) retrieval results.
type retrievalFusionResult struct {
	// EnrichedMemories contains additional LTM memories discovered via KG entity labels.
	EnrichedMemories string
	// EnrichedKGContext contains additional KG entities discovered via RAG memory text.
	EnrichedKGContext string
}

// Fusion budget constants — strict limits to prevent context explosion.
const (
	// fusionMaxEntities limits how many entity labels are used as LTM queries (KG→RAG).
	fusionMaxEntities = 3
	// fusionMaxRAGQueries limits how many RAG memories are used as KG queries (RAG→KG).
	fusionMaxRAGQueries = 2
	// fusionCharBudget is the strict character budget per direction.
	fusionCharBudget = 400
)

// entityLinePattern matches KG context lines like "- [entity_id] label | prop: val".
var entityLinePattern = regexp.MustCompile(`^-\s+\[([^\]]+)\]\s*(.*)`)

// applyRetrievalFusion enriches RAG and KG context by cross-referencing results from
// both subsystems. When strong RAG hits exist, related KG entities are loaded (RAG→KG).
// When strong KG hits exist, related VectorDB memories are searched (KG→RAG).
//
// The fusion is budget-limited and only activates when both subsystems produced results.
// This creates a bidirectional enrichment that improves context quality without exploding
// the prompt size.
func applyRetrievalFusion(
	topMemories []string,
	kgContext string,
	longTermMem memory.VectorDB,
	kg *memory.KnowledgeGraph,
	logger *slog.Logger,
) retrievalFusionResult {
	result := retrievalFusionResult{}
	hasRAG := len(topMemories) > 0
	hasKG := kgContext != ""

	if !hasRAG && !hasKG {
		return result
	}

	// Direction 1: KG→RAG — extract entity labels from KG context, search LTM.
	// This finds long-term memories related to the knowledge graph entities.
	if hasKG && longTermMem != nil && !longTermMem.IsDisabled() {
		labels := extractKGEntityLabels(kgContext, fusionMaxEntities)
		if len(labels) > 0 {
			var extraMemories []string
			for _, label := range labels {
				mems, _, err := longTermMem.SearchMemoriesOnly(label, 1)
				if err != nil || len(mems) == 0 {
					continue
				}
				// Deduplicate against existing top memories and already-found extras.
				if containsString(topMemories, mems[0]) || containsString(extraMemories, mems[0]) {
					continue
				}
				compacted := compactMemoryForPrompt(mems[0], 200)
				if compacted == "" {
					continue
				}
				extraMemories = append(extraMemories, compacted)
				if len(extraMemories) >= fusionMaxEntities {
					break
				}
			}
			if len(extraMemories) > 0 {
				fusionText := "[Related Memories via Knowledge Graph]\n" + strings.Join(extraMemories, "\n---\n")
				if len(fusionText) > fusionCharBudget {
					fusionText = truncateUTF8SafeAgent(fusionText, fusionCharBudget)
				}
				result.EnrichedMemories = fusionText
			}
		}
	}

	// Direction 2: RAG→KG — use top RAG memories as queries for additional KG context.
	// This discovers knowledge graph entities that are semantically related to retrieved memories.
	if hasRAG && kg != nil {
		var extraKGParts []string
		limit := len(topMemories)
		if limit > fusionMaxRAGQueries {
			limit = fusionMaxRAGQueries
		}
		for i := 0; i < limit; i++ {
			// Use first 120 chars of memory as KG query to keep it focused.
			query := topMemories[i]
			if len(query) > 120 {
				query = query[:120]
			}
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}
			kgResult := kg.SearchForContext(query, 2, 200)
			if kgResult == "" {
				continue
			}
			// Skip if this result is already contained in the existing KG context.
			if strings.Contains(kgContext, kgResult) {
				continue
			}
			// Skip if we already added this exact result.
			if containsString(extraKGParts, kgResult) {
				continue
			}
			extraKGParts = append(extraKGParts, kgResult)
		}
		if len(extraKGParts) > 0 {
			fusionText := strings.Join(extraKGParts, "\n")
			if len(fusionText) > fusionCharBudget {
				fusionText = truncateUTF8SafeAgent(fusionText, fusionCharBudget)
			}
			result.EnrichedKGContext = fusionText
		}
	}

	if logger != nil && (result.EnrichedMemories != "" || result.EnrichedKGContext != "") {
		logger.Debug("[RetrievalFusion] Enriched context",
			"ltm_via_kg", result.EnrichedMemories != "",
			"kg_via_ltm", result.EnrichedKGContext != "",
			"mem_chars", len(result.EnrichedMemories),
			"kg_chars", len(result.EnrichedKGContext),
		)
	}

	return result
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
