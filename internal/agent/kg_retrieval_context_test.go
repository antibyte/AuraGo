package agent

import (
	"aurago/internal/memory"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	chromem "github.com/philippgille/chromem-go"
	"github.com/sashabaranov/go-openai"
)

func TestAgentLoopCancellationStopsKnowledgeGraphEmbeddingWait(t *testing.T) {
	runCfg, client, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
	defer cleanup()
	started := make(chan struct{}, 1)
	kg := newBlockingQueryKnowledgeGraph(t, started)
	defer kg.Close()
	runCfg.KG = kg
	runCfg.Config.Tools.KnowledgeGraph.Enabled = true
	runCfg.Config.Tools.KnowledgeGraph.PromptInjection = true
	runCfg.SuppressTurnSideEffects = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := ExecuteAgentLoop(ctx, openai.ChatCompletionRequest{
			Model:    runCfg.Config.LLM.Model,
			Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "How does the NAS backup schedule work?"}},
		}, runCfg, false, NoopBroker{})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		cancel()
		t.Fatal("agent loop did not start KG query embedding")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("agent loop error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("agent loop did not stop after turn cancellation")
	}
	if client.calls != 0 {
		t.Fatalf("LLM calls after canceled KG lookup = %d, want 0", client.calls)
	}
}

func TestRetrievalFusionCancellationStopsKnowledgeGraphEmbeddingWait(t *testing.T) {
	started := make(chan struct{}, 1)
	kg := newBlockingQueryKnowledgeGraph(t, started)
	defer kg.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan retrievalFusionResult, 1)
	go func() {
		done <- applyRetrievalFusion(ctx, []rankedMemory{{text: "NAS backup schedule and retention policy"}}, "- [existing] Existing", nil, nil, kg, slog.Default())
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("retrieval fusion did not start KG query embedding")
	}
	cancel()
	select {
	case result := <-done:
		if len(result.EnrichedKGContexts) != 0 {
			t.Fatalf("fusion result after cancellation = %+v, want no KG enrichment", result.EnrichedKGContexts)
		}
	case <-time.After(time.Second):
		t.Fatal("retrieval fusion did not stop after cancellation")
	}
}

func TestQueryMemoryForLocalAgentContextStopsKnowledgeGraphEmbeddingWait(t *testing.T) {
	started := make(chan struct{}, 1)
	kg := newBlockingQueryKnowledgeGraph(t, started)
	defer kg.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := QueryMemoryForLocalAgentContext(ctx, "NAS backup schedule and retention", 5, nil, nil, kg, nil, nil)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("local agent query did not start KG query embedding")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("local agent query error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("local agent query did not stop after cancellation")
	}
}

func newBlockingQueryKnowledgeGraph(t *testing.T, started chan<- struct{}) *memory.KnowledgeGraph {
	t.Helper()
	kg, err := memory.NewKnowledgeGraph(":memory:", "", slog.Default())
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	err = kg.EnableSemanticSearchShared(chromem.NewDB(), func(ctx context.Context, query string) ([]float32, error) {
		if query == "knowledge graph semantic validation" {
			return []float32{1, 0}, nil
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if err != nil {
		_ = kg.Close()
		t.Fatalf("EnableSemanticSearchShared: %v", err)
	}
	return kg
}
