package agent

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

type consolidationDeadlineVector struct {
	maintenanceOwnedVector
	contextCalls int
}

func (v *consolidationDeadlineVector) StoreDocumentOwnedContext(ctx context.Context, _, _ string, _ memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	v.contextCalls++
	<-ctx.Done()
	return memory.VectorStoreResult{}, ctx.Err()
}

func TestConsolidationStorageDeadlineReleasesClaims(t *testing.T) {
	for _, path := range []string{"direct", "helper", "helper fallback"} {
		t.Run(path, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ResetGlobalHelperLLMManager()
				defer ResetGlobalHelperLLMManager()
				stm, logger := maintenanceRegressionStores(t)
				for range 4 {
					if _, err := stm.InsertMessage("direct", "user", strings.Repeat("Remember the backup target. ", 100), false, false); err != nil {
						t.Fatal(err)
					}
				}
				if err := stm.DeleteOldMessages("direct", 1); err != nil {
					t.Fatal(err)
				}
				cfg := &config.Config{}
				cfg.Tools.Memory.Enabled = true
				cfg.Tools.KnowledgeGraph.Enabled = true
				cfg.LLM.Model = "test-model"
				vdb := &consolidationDeadlineVector{}
				const facts = `"facts":[{"concept":"Backup","content":"The backup target is the NAS."}]`
				client := maintenanceCompletionTestClient{complete: func(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
					response := `{` + facts + `}`
					if path != "direct" {
						paired := strings.Contains(req.Messages[len(req.Messages)-1].Content, "=== batch_2 ===")
						response = `{"batches":[{"batch_id":"batch_1",` + facts + `}]}`
						if paired && path == "helper fallback" {
							response = `{"batches":[]}`
						} else if paired {
							response = `{"batches":[{"batch_id":"batch_1",` + facts + `},{"batch_id":"batch_2",` + facts + `}]}`
						}
					}
					return skillQualityTestClient{response: response}.CreateChatCompletion(ctx, req)
				}}
				if path != "direct" {
					cfg.LLM.HelperEnabled = true
					cfg.LLM.HelperProviderType = "openai"
					cfg.LLM.HelperBaseURL = "http://127.0.0.1:1/v1"
					cfg.LLM.HelperResolvedModel = "test-model"
					manager := newHelperLLMManager(cfg, logger)
					if manager == nil {
						t.Fatal("helper manager unavailable")
					}
					manager.client = client
				}
				for range 4 {
					ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
					started := time.Now()
					result := consolidateSTMtoLTMWithContext(ctx, cfg, logger, client, stm, vdb, nil)
					cancel()
					if time.Since(started) != 10*time.Second || result.MessagesConsolidated != 0 || len(result.Errors) == 0 {
						t.Fatalf("elapsed=%v result=%+v", time.Since(started), result)
					}
					pending, err := stm.GetConsolidationCandidates(10, 3)
					if err != nil || len(pending) != 3 {
						t.Fatalf("pending=%+v error=%v", pending, err)
					}
					for _, message := range pending {
						if message.ConsolidationStatus != "pending" || message.ConsolidationRetries != 0 {
							t.Fatalf("deadline consumed retry: %+v", message)
						}
					}
				}
				if vdb.contextCalls != 4 {
					t.Fatalf("context-aware writes=%d, want 4", vdb.contextCalls)
				}
			})
		})
	}
}
