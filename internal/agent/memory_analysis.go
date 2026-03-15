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
		MaxTokens:   500,
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
			raw = raw[:len(raw)-3]
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
		MaxTokens:   1000,
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
			raw = raw[:len(raw)-3]
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
	if stm != nil {
		summary, _ := result["summary"].(string)
		if summary != "" {
			_, _ = stm.InsertJournalEntry(memory.JournalEntry{
				EntryType:     "reflection",
				Title:         fmt.Sprintf("Memory Reflection (%s)", scope),
				Content:       summary,
				Importance:    3,
				AutoGenerated: true,
			})
		}
	}

	return result, nil
}
