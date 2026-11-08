package agent

import (
	"strings"

	"aurago/internal/llm"
)

func helperRAGPrompt(userQuery string, candidates []rankedMemory, divisor int) string {
	userQuery = truncateActivityDigestInput(strings.TrimSpace(userQuery), 700)
	var prompt strings.Builder
	prompt.WriteString("User request:\n")
	prompt.WriteString(helperExternalDataBlock("user_request", helperSourceText(userQuery, 700, divisor), 700))
	prompt.WriteString("\n\nMemory candidates:\n")
	if len(candidates) == 0 {
		prompt.WriteString("none\n")
	}
	for _, candidate := range candidates {
		memoryID := strings.TrimSpace(candidate.docID)
		if memoryID == "" {
			continue
		}
		prompt.WriteString("- memory_id: ")
		prompt.WriteString(memoryID)
		prompt.WriteString("\n  content:\n")
		prompt.WriteString(helperExternalDataBlock("memory_candidate", helperSourceText(strings.TrimSpace(candidate.text), 260, divisor), 260))
		prompt.WriteString("\n")
	}
	return strings.TrimSpace(prompt.String())
}

func validateHelperRAG(raw string, candidates []rankedMemory) error {
	if err := requireHelperFields(raw, "search_query", "search_terms", "candidate_scores"); err != nil {
		return err
	}
	parsed, err := parseHelperRAGBatchResult(raw)
	if err != nil {
		return err
	}
	expected := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if id := strings.TrimSpace(candidate.docID); id != "" {
			expected = append(expected, id)
		}
	}
	actual := make([]string, 0, len(parsed.CandidateScores))
	for _, score := range parsed.CandidateScores {
		actual = append(actual, score.MemoryID)
	}
	return requireHelperIDs(expected, actual)
}

// Cached helper responses precede offloading; reusing one saves no model call.
func (m *helperLLMManager) hasCachedRAG(query string, candidates []rankedMemory) bool {
	if m == nil || m.client == nil || m.model == "" || strings.TrimSpace(query) == "" {
		return false
	}
	key := m.helperCacheKey("rag_batch", m.model, helperRAGPrompt(query, candidates, 1))
	raw, ok := m.getCachedResponse(key)
	if !ok {
		return false
	}
	normalized, err := llm.NormalizeJSONContent(raw)
	if err == nil && validateHelperRAG(normalized, candidates) == nil {
		return true
	}
	m.deleteCachedResponse(key)
	return false
}
