package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/security"
	"github.com/sashabaranov/go-openai"
)

func enabledMemoryTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.Journal.Enabled = true
	cfg.Tools.Notes.Enabled = true
	return cfg
}

func TestRuntimeMemoryPolicyAndPersonalFacts(t *testing.T) {
	cfg := enabledMemoryTestConfig()
	if !runtimeMemoryWriteAllowed(cfg, "memory", "human") || runtimeMemoryWriteAllowed(cfg, "memory", "a2a-task-1") {
		t.Fatal("role policy")
	}
	cfg.Tools.Memory.ReadOnly = true
	if runtimeMemoryWriteAllowed(cfg, "memory", "human") {
		t.Fatal("readonly bypass")
	}
	for _, text := range []string{"User is disabled and uses a wheelchair.", "User prefers API error handling tutorials.", "User's birthday is 1990-04-17.", "User prefers toolbox debugging guides."} {
		if !shouldStoreExtractedMemory(text, "personal") {
			t.Fatalf("legitimate personal fact filtered: %s", text)
		}
	}
	for _, text := range []string{"The tool is disabled.", "The endpoint has a bug.", "api_key=synthetic-secret-123456"} {
		if shouldStoreExtractedMemory(text, "operational") {
			t.Fatalf("unsafe fact allowed: %s", text)
		}
	}
}

func TestConsolidationSerializesCompleteSourcesWithRepeatedProvenance(t *testing.T) {
	content := strings.Repeat("生日🎈", 1500) + " final fact"
	parts := serializeConsolidationSources([]memory.ArchivedMessage{{ID: 42, SessionID: "source-session", Role: "user", Timestamp: "2026-10-18", Content: content}})
	var restored strings.Builder
	for _, part := range parts {
		if len(part) > 4000 || !utf8.ValidString(part) || !strings.Contains(part, fmt.Sprintf("role=%q", "user")) || !strings.Contains(part, fmt.Sprintf("session=%q", "source-session")) {
			t.Fatal("invalid or missing source header")
		}
		_, body, _ := strings.Cut(part, "\n")
		restored.WriteString(strings.TrimSuffix(body, "\n[end source part]\n"))
	}
	if restored.String() != content || len(parts) < 2 {
		t.Fatal("source lost or truncated")
	}
}

func TestHelperConsolidationNeverTruncatesSourceOnRetry(t *testing.T) {
	var sources []string
	client := maintenanceCompletionTestClient{complete: func(_ context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
		sources = append(sources, req.Messages[1].Content)
		content := `{broken`
		if len(sources) > 1 {
			content = `{"batches":[{"batch_id":"batch_1","facts":[]}]}`
		}
		return skillQualityTestClient{response: content}.CreateChatCompletion(context.Background(), req)
	}}
	manager := &helperLLMManager{client: client, model: "test-model"}
	full := strings.Repeat("preserve source\n", 400) + "FINAL_SENTINEL"
	if _, err := manager.AnalyzeConsolidationBatches(context.Background(), []helperConsolidationBatchInput{{BatchID: "batch_1", Conversation: full}}); err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 || sources[0] != sources[1] || !strings.Contains(sources[0], "FINAL_SENTINEL") {
		t.Fatal("helper silently changed extraction source")
	}
}

func TestConsolidationOnlyFinalizesFullyProcessedGiantMessages(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "provider_outage"}[fail], func(t *testing.T) {
			stm, logger := maintenanceRegressionStores(t)
			body := strings.Repeat("specific complete source detail ", 400) + "FINAL_SOURCE_SENTINEL"
			for _, text := range []string{body, "tail protected"} {
				if _, err := stm.InsertMessage("giant-session", "user", text, false, false); err != nil {
					t.Fatal(err)
				}
			}
			if err := stm.DeleteOldMessages("giant-session", 1); err != nil {
				t.Fatal(err)
			}
			cfg := enabledMemoryTestConfig()
			cfg.LLM.Model = "test-model"
			var source strings.Builder
			calls := 0
			client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
				calls++
				source.WriteString(req.Messages[len(req.Messages)-1].Content)
				if fail && calls > 1 {
					return openai.ChatCompletionResponse{}, context.DeadlineExceeded
				}
				return skillQualityTestClient{response: `{"facts":[]}`}.CreateChatCompletion(ctx, req)
			}}
			result := consolidateSTMtoLTMWithContext(context.Background(), cfg, logger, client, stm, &fakeVectorDB{}, nil)
			remaining, err := stm.CountConsolidationCandidates(1)
			if err != nil {
				t.Fatal(err)
			}
			if fail {
				if result.MessagesConsolidated != 0 || remaining != 1 {
					t.Fatalf("partial source marked complete or retry consumed: %+v remaining=%d", result, remaining)
				}
			} else if result.MessagesConsolidated != 1 || remaining != 0 || !strings.Contains(source.String(), "FINAL_SOURCE_SENTINEL") {
				t.Fatalf("source incomplete: %+v remaining=%d", result, remaining)
			}
		})
	}
}

func TestHelperCompressionRetainsCompleteSource(t *testing.T) {
	client := &mockChatClient{response: `{"memories":[{"memory_id":"m1","compressed":"complete fact"}]}`}
	manager := &helperLLMManager{client: client, model: "test-model"}
	if _, err := manager.CompressMemoryBatches(context.Background(), []helperCompressionBatchInput{{MemoryID: "m1", Content: strings.Repeat("durable detail ", 400) + "FINAL_COMPRESSION_SENTINEL"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(client.lastReq.Messages[1].Content, "FINAL_COMPRESSION_SENTINEL") {
		t.Fatal("compression archived a source it never saw completely")
	}
}

func TestConsolidationFilteredFactKeepsHealthyFact(t *testing.T) {
	stm, logger := maintenanceRegressionStores(t)
	vdb := &fakeVectorDB{}
	stored, skipped, err := storeConsolidationFacts(logger, stm, vdb, []helperConsolidationFact{{Concept: "credential", Content: "api_key=synthetic-secret-123456"}, {Concept: "profile", Content: "User prefers clear German explanations."}})
	if err != nil || stored != 1 || skipped != 1 {
		t.Fatalf("healthy fact rolled back: stored=%d skipped=%d err=%v", stored, skipped, err)
	}
}

func TestConsolidationEncodedBudgetPreservesLongProvenanceAndEscapedSource(t *testing.T) {
	session := strings.Repeat("long<&生日-session\"", 600)
	content := strings.Repeat("<&🎈", 2000) + " final whitespace \n\t"
	message := memory.ArchivedMessage{ID: 43, SessionID: session, Role: "user", Timestamp: "2026-10-18", Content: content}
	parts := serializeConsolidationSources([]memory.ArchivedMessage{message})
	var provenance, restored strings.Builder
	for _, part := range parts {
		encoded := security.IsolateExternalData(part)
		if len(encoded) > 4000 || !utf8.ValidString(part) {
			t.Fatalf("encoded source budget exceeded: %d", len(encoded))
		}
		for _, record := range strings.Split(part, "\n[end source part]\n") {
			if record == "" {
				continue
			}
			header, body, ok := strings.Cut(record, "\n")
			if !ok {
				t.Fatal("missing record header")
			}
			if strings.Contains(header, "provenance_part=") {
				provenance.WriteString(body)
			} else {
				restored.WriteString(body)
			}
		}
		client := &mockChatClient{response: `{"batches":[{"batch_id":"batch_1","facts":[]}]}`}
		manager := &helperLLMManager{client: client, model: "test-model"}
		if _, err := manager.AnalyzeConsolidationBatches(context.Background(), []helperConsolidationBatchInput{{BatchID: "batch_1", Conversation: part}}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(client.lastReq.Messages[1].Content, security.IsolateExternalData(strings.TrimSpace(part))) {
			t.Fatal("final helper request changed encoded source")
		}
	}
	wantHeader := fmt.Sprintf("[%s] session=%q role=%q message_id=%d", message.Timestamp, session, message.Role, message.ID)
	if provenance.String() != wantHeader || restored.String() != content {
		t.Fatal("provenance or escaped source lost")
	}
}
