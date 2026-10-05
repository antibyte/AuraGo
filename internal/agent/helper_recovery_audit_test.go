package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/llm"
	"aurago/internal/memory"
	"github.com/sashabaranov/go-openai"
)

type helperRecoveryClient struct {
	responses []openai.ChatCompletionResponse
	requests  []openai.ChatCompletionRequest
}

func (c *helperRecoveryClient) CreateChatCompletion(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.requests = append(c.requests, req)
	return c.responses[min(len(c.requests)-1, len(c.responses)-1)], nil
}

func (*helperRecoveryClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, fmt.Errorf("unexpected stream")
}

func helperRecoveryResponse(text string, reason openai.FinishReason) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Content: text}, FinishReason: reason}}}
}

func TestHelperRecoveryRebuildsEveryOperationWithSmallerSources(t *testing.T) {
	source := strings.Repeat("Überprüfung 🔧 Quelle. ", 300)
	for _, tc := range []struct {
		name, response, preserved string
		run                       func(*helperLLMManager) error
	}{
		{"abstract", `{"abstract":"A useful summary"}`, "sheet-identifier", func(m *helperLLMManager) error {
			_, err := m.GenerateCheatsheetAbstract(context.Background(), "sheet-identifier", source)
			return err
		}},
		{"turn", `{"memory_analysis":{},"activity_digest":{},"personality_analysis":{}}`, "query_memory", func(m *helperLLMManager) error {
			_, err := m.AnalyzeTurn(context.Background(), source, source, []string{"query_memory"}, []string{source}, &helperTurnPersonalityInput{CurrentUserMessage: source, RecentHistory: source})
			return err
		}},
		{"maintenance", `{"daily_summary":"A useful summary","kg_extraction":{"nodes":[],"edges":[]}}`, "node-identifier", func(m *helperLLMManager) error {
			_, err := m.AnalyzeMaintenanceSummaryAndKG(context.Background(), "2026-10-05", source, source, "node-identifier")
			return err
		}},
		{"consolidation", `{"batches":[{"batch_id":"batch-identifier","facts":[]}]}`, "batch-identifier", func(m *helperLLMManager) error {
			_, err := m.AnalyzeConsolidationBatches(context.Background(), []helperConsolidationBatchInput{{BatchID: "batch-identifier", Conversation: source}})
			return err
		}},
		{"compression", `{"memories":[{"memory_id":"memory-identifier","compressed":"A useful summary"}]}`, "memory-identifier", func(m *helperLLMManager) error {
			_, err := m.CompressMemoryBatches(context.Background(), []helperCompressionBatchInput{{MemoryID: "memory-identifier", Content: source}})
			return err
		}},
		{"summaries", `{"summaries":[{"batch_id":"batch-identifier","summary":"A useful summary"}]}`, "batch-identifier", func(m *helperLLMManager) error {
			_, err := m.SummarizeContentBatches(context.Background(), []helperContentSummaryBatchInput{{BatchID: "batch-identifier", SourceName: "web", SearchQuery: "query", Content: source}})
			return err
		}},
		{"rag", `{"search_query":"query","search_terms":[],"candidate_scores":[{"memory_id":"memory-identifier","score":8}]}`, "memory-identifier", func(m *helperLLMManager) error {
			_, err := m.AnalyzeRAG(context.Background(), source, []rankedMemory{{docID: "memory-identifier", text: source}})
			return err
		}},
		{"character", `{"notes":[]}`, "required-existing-note", func(m *helperLLMManager) error {
			_, err := m.ProposeCharacterNotes(context.Background(), memory.CharacterReflectionInput{CorePersonality: "neutral", Milestones: []string{source}, ExistingNotes: []string{"required-existing-note"}})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := &helperRecoveryClient{responses: []openai.ChatCompletionResponse{helperRecoveryResponse(`{"truncated":`, openai.FinishReasonLength), helperRecoveryResponse(tc.response, openai.FinishReasonStop)}}
			manager := &helperLLMManager{client: client, model: "helper-model", responseCache: make(map[string]string)}
			if err := tc.run(manager); err != nil {
				t.Fatal(err)
			}
			if len(client.requests) != 2 {
				t.Fatalf("calls = %d", len(client.requests))
			}
			first, second := client.requests[0], client.requests[1]
			if len(second.Messages[1].Content) >= len(first.Messages[1].Content) {
				t.Fatal("retry did not reduce source budget")
			}
			if first.Messages[0].Content != second.Messages[0].Content || !strings.Contains(second.Messages[1].Content, tc.preserved) {
				t.Fatal("retry lost required structure")
			}
			if !utf8.ValidString(second.Messages[1].Content) || second.MaxTokens <= 0 {
				t.Fatal("invalid retry request")
			}
			if tc.name == "turn" {
				for _, msg := range []string{first.Messages[1].Content, second.Messages[1].Content} {
					if strings.Count(msg, "<external_data ") != strings.Count(msg, "</external_data>") {
						t.Fatal("retry cut isolation wrapper")
					}
				}
			}
			if err := tc.run(manager); err != nil {
				t.Fatal(err)
			}
			if len(client.requests) != 2 {
				t.Fatal("valid retry was not cached under original request")
			}
		})
	}
}

func TestHelperMissingIDsHaveOneRetryAndNeverPopulateCache(t *testing.T) {
	client := &helperRecoveryClient{responses: []openai.ChatCompletionResponse{helperRecoveryResponse(`{"summaries":[{"batch_id":"first","summary":"partial"}]}`, openai.FinishReasonStop)}}
	manager := &helperLLMManager{client: client, model: "helper-model", responseCache: make(map[string]string)}
	items := []helperContentSummaryBatchInput{{BatchID: "first", SourceName: "web", SearchQuery: "query", Content: strings.Repeat("source ", 100)}, {BatchID: "second", SourceName: "web", SearchQuery: "query", Content: strings.Repeat("other ", 100)}}
	if result, err := manager.SummarizeContentBatches(context.Background(), items); err == nil || len(result.Summaries) != 0 {
		t.Fatalf("accepted partial result: %#v, %v", result, err)
	}
	if len(client.requests) != 2 || len(manager.responseCache) != 0 {
		t.Fatalf("calls/cache = %d/%d", len(client.requests), len(manager.responseCache))
	}
	for _, req := range client.requests {
		if !strings.Contains(req.Messages[1].Content, "=== second ===") {
			t.Fatal("retry omitted expected ID")
		}
	}
}

func TestHelperRejectsInvalidStructureBeforeCacheAndStopsUnshrinkableRetry(t *testing.T) {
	client := &helperRecoveryClient{responses: []openai.ChatCompletionResponse{helperRecoveryResponse(`{}`, openai.FinishReasonStop)}}
	manager := &helperLLMManager{client: client, model: "helper-model", responseCache: make(map[string]string)}
	_, err := manager.GenerateCheatsheetAbstract(context.Background(), "sheet-identifier", "")
	if err == nil || len(client.requests) != 1 || len(manager.responseCache) != 0 {
		t.Fatalf("err/calls/cache = %v/%d/%d", err, len(client.requests), len(manager.responseCache))
	}
}

func TestHelperCharacterCacheTracksReflectionInput(t *testing.T) {
	client := &helperRecoveryClient{responses: []openai.ChatCompletionResponse{helperRecoveryResponse(`{"notes":[]}`, openai.FinishReasonStop)}}
	manager := &helperLLMManager{client: client, model: "helper-model", responseCache: make(map[string]string)}
	input := memory.CharacterReflectionInput{CorePersonality: "neutral", Milestones: []string{"First milestone"}}
	for i := 0; i < 2; i++ {
		if _, err := manager.ProposeCharacterNotes(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	input.Milestones = []string{"Second milestone"}
	if _, err := manager.ProposeCharacterNotes(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("calls = %d; identical input should hit cache, changed input should miss", len(client.requests))
	}
}
