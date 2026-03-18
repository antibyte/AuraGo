package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

// memoryAnalysisResult represents extracted memory-worthy content from a conversation turn.
type memoryAnalysisResult struct {
	Facts       []extractedFact `json:"facts,omitempty"`
	Preferences []extractedFact `json:"preferences,omitempty"`
	Corrections []extractedFact `json:"corrections,omitempty"`
}

type extractedFact struct {
	Content    string  `json:"content"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

const memoryAnalysisPrompt = `You are a memory extraction assistant. Analyze the following conversation exchange and extract any information worth remembering.

Extract:
1. **Facts**: Concrete facts about the user, their environment, preferences, or projects (e.g., "User runs Proxmox on a Dell R730", "User's name is Alex")
2. **Preferences**: User preferences, habits, or workflows (e.g., "User prefers Go over Python", "User likes minimal logging")
3. **Corrections**: Corrections to previously known information (e.g., "User moved from Berlin to Munich", "User switched from Docker to Podman")

For each extracted item, provide:
- content: The factual statement to remember
- category: A short category label (e.g., "infrastructure", "personal", "workflow", "preference")
- confidence: How confident you are this is worth storing (0.0 to 1.0)

Rules:
- Only extract genuinely useful, long-term information. Skip transient requests like "show me the logs".
- Do NOT extract information that is just part of the current task context.
- Do NOT extract emotions, moods, or temporary states.
- If there is nothing worth remembering, return empty arrays.

Respond ONLY with valid JSON in this exact format:
{"facts":[],"preferences":[],"corrections":[]}

User message:
%s

Assistant response:
%s`

// runMemoryAnalysis performs async post-response memory extraction using the configured analysis provider.
func runMemoryAnalysis(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	stm *memory.SQLiteMemory,
	kg *memory.KnowledgeGraph,
	ltm memory.VectorDB,
	userMsg string,
	assistantResp string,
	sessionID string,
) {
	if userMsg == "" || len(userMsg) < 10 {
		return // too short to analyze
	}

	// Create analysis client (uses dedicated provider or falls back to main LLM)
	analysisClient := llm.NewClientFromProvider(
		cfg.MemoryAnalysis.ProviderType,
		cfg.MemoryAnalysis.BaseURL,
		cfg.MemoryAnalysis.APIKey,
	)

	// Truncate for analysis (no need to send huge responses)
	truncUser := userMsg
	if len(truncUser) > 2000 {
		truncUser = truncUser[:2000] + "..."
	}
	truncResp := assistantResp
	if len(truncResp) > 2000 {
		truncResp = truncResp[:2000] + "..."
	}

	prompt := fmt.Sprintf(memoryAnalysisPrompt, truncUser, truncResp)

	req := openai.ChatCompletionRequest{
		Model: cfg.MemoryAnalysis.ResolvedModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   800,
	}

	resp, err := analysisClient.CreateChatCompletion(ctx, req)
	if err != nil {
		logger.Warn("[Memory Analysis] LLM call failed", "error", err)
		return
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return
	}

	raw := resp.Choices[0].Message.Content
	// Strip markdown code fences if present
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw[3:], "\n"); idx >= 0 {
			raw = raw[3+idx+1:]
		}
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
		}
		raw = strings.TrimSpace(raw)
	}

	var result memoryAnalysisResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		logger.Warn("[Memory Analysis] Failed to parse response", "error", err, "raw", Truncate(raw, 200))
		return
	}

	threshold := cfg.MemoryAnalysis.AutoConfirm
	stored := 0

	// Process facts
	for _, f := range result.Facts {
		if f.Confidence >= threshold && f.Content != "" {
			if ltm != nil {
				concept := fmt.Sprintf("[%s] %s", f.Category, f.Content)
				if _, err := ltm.StoreDocument(concept, "source:memory_analysis session:"+sessionID); err != nil {
					logger.Warn("[Memory Analysis] Failed to store fact in LTM", "error", err)
				} else {
					stored++
				}
			}
		}
	}

	// Process preferences
	for _, p := range result.Preferences {
		if p.Confidence >= threshold && p.Content != "" {
			if ltm != nil {
				concept := fmt.Sprintf("[preference:%s] %s", p.Category, p.Content)
				if _, err := ltm.StoreDocument(concept, "source:memory_analysis session:"+sessionID); err != nil {
					logger.Warn("[Memory Analysis] Failed to store preference in LTM", "error", err)
				} else {
					stored++
				}
			}
		}
	}

	// Process corrections — these update core memory
	for _, c := range result.Corrections {
		if c.Confidence >= threshold && c.Content != "" {
			if ltm != nil {
				concept := fmt.Sprintf("[correction:%s] %s", c.Category, c.Content)
				if _, err := ltm.StoreDocument(concept, "source:memory_analysis session:"+sessionID); err != nil {
					logger.Warn("[Memory Analysis] Failed to store correction in LTM", "error", err)
				} else {
					stored++
				}
			}
		}
	}

	if stored > 0 {
		logger.Info("[Memory Analysis] Stored extracted memories",
			"facts", len(result.Facts),
			"preferences", len(result.Preferences),
			"corrections", len(result.Corrections),
			"stored", stored,
			"session", sessionID,
		)
	}
}

// expandQueryForRAG uses the MemoryAnalysis LLM to generate optimized search keywords
// from the user's message for better RAG retrieval. Returns the expanded query string
// or the original message on failure/timeout.
func expandQueryForRAG(ctx context.Context, cfg *config.Config, logger *slog.Logger, userMsg string) string {
	if !cfg.MemoryAnalysis.Enabled || !cfg.MemoryAnalysis.QueryExpansion || len(userMsg) <= 20 {
		return userMsg
	}

	expandCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	client := llm.NewClientFromProvider(
		cfg.MemoryAnalysis.ProviderType,
		cfg.MemoryAnalysis.BaseURL,
		cfg.MemoryAnalysis.APIKey,
	)

	model := cfg.MemoryAnalysis.ResolvedModel
	if model == "" {
		model = cfg.LLM.Model
	}

	truncMsg := userMsg
	if len(truncMsg) > 500 {
		truncMsg = truncMsg[:500]
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Extract 2-3 concise search keywords from this message. Output ONLY the keywords separated by spaces, nothing else.\n\nMessage: " + truncMsg,
			},
		},
		Temperature: 0.0,
		MaxTokens:   50,
	}

	resp, err := client.CreateChatCompletion(expandCtx, req)
	if err != nil {
		logger.Debug("[RAG Query Expansion] LLM call failed, using original query", "error", err)
		return userMsg
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return userMsg
	}

	expanded := strings.TrimSpace(resp.Choices[0].Message.Content)
	if expanded == "" || len(expanded) > 200 {
		return userMsg
	}

	// Combine original query with expanded keywords for better embedding coverage
	combined := userMsg + " " + expanded
	logger.Debug("[RAG Query Expansion] Expanded query", "original_len", len(userMsg), "keywords", expanded)
	return combined
}

// rerankWithLLM uses the MemoryAnalysis LLM to score the relevance of RAG candidates
// against the user query. Returns re-ranked results or falls back to the input order on failure.
func rerankWithLLM(ctx context.Context, cfg *config.Config, logger *slog.Logger, candidates []rankedMemory, userQuery string) []rankedMemory {
	if !cfg.MemoryAnalysis.Enabled || !cfg.MemoryAnalysis.LLMReranking || len(candidates) == 0 {
		return candidates
	}

	rerankCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	client := llm.NewClientFromProvider(
		cfg.MemoryAnalysis.ProviderType,
		cfg.MemoryAnalysis.BaseURL,
		cfg.MemoryAnalysis.APIKey,
	)

	model := cfg.MemoryAnalysis.ResolvedModel
	if model == "" {
		model = cfg.LLM.Model
	}

	// Build candidate list for the prompt
	var sb strings.Builder
	for i, c := range candidates {
		text := c.text
		if len(text) > 300 {
			text = text[:300]
		}
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i, text))
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("Rate how relevant each memory is to the query. Output ONLY a JSON array of scores (0-10), one per memory, in order.\n\nQuery: %s\n\nMemories:\n%s",
					userQuery, sb.String()),
			},
		},
		Temperature: 0.0,
		MaxTokens:   100,
	}

	resp, err := client.CreateChatCompletion(rerankCtx, req)
	if err != nil {
		logger.Debug("[RAG LLM Rerank] LLM call failed, keeping original order", "error", err)
		return candidates
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return candidates
	}

	raw := strings.TrimSpace(resp.Choices[0].Message.Content)
	// Strip markdown code fences if present
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw[3:], "\n"); idx >= 0 {
			raw = raw[3+idx+1:]
		}
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
		}
		raw = strings.TrimSpace(raw)
	}

	var scores []float64
	if err := json.Unmarshal([]byte(raw), &scores); err != nil {
		logger.Debug("[RAG LLM Rerank] Failed to parse scores", "error", err, "raw", raw)
		return candidates
	}

	if len(scores) != len(candidates) {
		logger.Debug("[RAG LLM Rerank] Score count mismatch", "expected", len(candidates), "got", len(scores))
		return candidates
	}

	// Apply LLM scores: blend with existing similarity score (70% LLM, 30% original)
	for i := range candidates {
		llmScore := scores[i]
		if llmScore < 0 {
			llmScore = 0
		}
		if llmScore > 10 {
			llmScore = 10
		}
		normalizedLLM := llmScore / 10.0
		candidates[i].score = normalizedLLM*0.7 + candidates[i].score*0.3
	}

	// Sort by new blended score descending
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	logger.Debug("[RAG LLM Rerank] Re-ranked candidates", "count", len(candidates), "scores", scores)
	return candidates
}

// weeklyReflectionDue checks if the weekly reflection should run today.
func weeklyReflectionDue(cfg *config.Config) bool {
	if !cfg.MemoryAnalysis.Enabled || !cfg.MemoryAnalysis.WeeklyReflection {
		return false
	}
	today := strings.ToLower(time.Now().Weekday().String())
	return today == strings.ToLower(cfg.MemoryAnalysis.ReflectionDay)
}

const reflectionPrompt = `You are a memory analyst. Review the following memory data and produce a structured reflection.

Analyze:
1. **Patterns**: Recurring themes, topics, or behaviors across memories
2. **Contradictions**: Facts that conflict with each other (e.g., two different locations stored)
3. **Knowledge Gaps**: Areas where the user has mentioned topics but key details are missing
4. **Suggestions**: Specific recommendations for memory maintenance (what to consolidate, what to verify, what to remove)

Memory data (%s scope):

=== Recent Journal Entries ===
%s

=== Knowledge Graph Sample ===
%s

=== Core Memory Facts ===
%s

Respond in this JSON format:
{"patterns":["pattern1","pattern2"],"contradictions":["contradiction1"],"gaps":["gap1","gap2"],"suggestions":["suggestion1","suggestion2"],"summary":"Brief 2-3 sentence overall assessment"}`

// generateMemoryReflection produces a LLM-driven analysis of memory health and patterns.
func generateMemoryReflection(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
	stm *memory.SQLiteMemory,
	kg *memory.KnowledgeGraph,
	ltm memory.VectorDB,
	mainClient llm.ChatClient,
	scope string,
) (interface{}, error) {
	// Gather data from each source
	var journalData, kgData, coreData string

	// Journal entries
	if stm != nil {
		limit := 10
		if scope == "monthly" {
			limit = 30
		} else if scope == "full" {
			limit = 50
		}
		entries, err := stm.SearchJournalEntries("", limit)
		if err == nil && len(entries) > 0 {
			b, _ := json.Marshal(entries)
			journalData = string(b)
		}
		if journalData == "" {
			journalData = "(no journal entries)"
		}
	} else {
		journalData = "(unavailable)"
	}

	// Knowledge Graph
	if kg != nil {
		kgData = kg.SearchForContext("*", 20, 2000)
		if kgData == "" {
			kgData = "(no knowledge graph data)"
		}
	} else {
		kgData = "(unavailable)"
	}

	// Core Memory
	if stm != nil {
		facts, err := stm.GetCoreMemoryFacts()
		if err == nil && len(facts) > 0 {
			b, _ := json.Marshal(facts)
			coreData = string(b)
		}
		if coreData == "" {
			coreData = "(no core memory facts)"
		}
	} else {
		coreData = "(unavailable)"
	}

	prompt := fmt.Sprintf(reflectionPrompt, scope, journalData, kgData, coreData)

	// Use dedicated analysis provider if configured, otherwise main client
	analysisClient := mainClient
	if cfg.MemoryAnalysis.APIKey != "" {
		analysisClient = llm.NewClientFromProvider(
			cfg.MemoryAnalysis.ProviderType,
			cfg.MemoryAnalysis.BaseURL,
			cfg.MemoryAnalysis.APIKey,
		)
	}

	model := cfg.MemoryAnalysis.ResolvedModel
	if model == "" {
		model = cfg.LLM.Model
	}

	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   2000, // reasoning models need budget for thinking + JSON response
	}

	resp, err := analysisClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("reflection LLM call: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("empty reflection response")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw[3:], "\n"); idx >= 0 {
			raw = raw[3+idx+1:]
		}
		if strings.HasSuffix(raw, "```") {
			raw = strings.TrimSuffix(raw, "```")
		}
		raw = strings.TrimSpace(raw)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		// If JSON parse fails, return as plain text
		return map[string]interface{}{
			"summary": raw,
		}, nil
	}

	// Store reflection as a journal entry for future reference
	// Map internal scope names to human-readable labels
	if stm != nil {
		summary, _ := result["summary"].(string)
		if summary != "" {
			scopeLabel := scope
			switch scope {
			case "recent":
				scopeLabel = "weekly"
			case "monthly":
				scopeLabel = "monthly"
			case "full":
				scopeLabel = "full"
			}
			_, _ = stm.InsertJournalEntry(memory.JournalEntry{
				EntryType:     "reflection",
				Title:         fmt.Sprintf("Memory Reflection (%s)", scopeLabel),
				Content:       summary,
				Importance:    3,
				AutoGenerated: true,
			})
		}
	}

	return result, nil
}
