package agent

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"aurago/internal/llm"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

type publicationRecoveryVector struct {
	*dedupConsolidationVectorDB
	cancel          context.CancelFunc
	cancelOnAttempt int
	storeAttempts   int
}

func (v *publicationRecoveryVector) StoreDocumentOwnedContext(ctx context.Context, concept, content string, mode memory.VectorStoreMode) (memory.VectorStoreResult, error) {
	v.storeAttempts++
	if err := ctx.Err(); err != nil {
		return memory.VectorStoreResult{}, err
	}
	if v.cancelOnAttempt > 0 && v.storeAttempts == v.cancelOnAttempt {
		v.cancel()
		return memory.VectorStoreResult{}, context.Canceled
	}
	return v.dedupConsolidationVectorDB.StoreDocumentOwned(concept, content, mode)
}

type publicationRecoveryClient struct {
	requests int
}

func (c *publicationRecoveryClient) CreateChatCompletion(ctx context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if err := ctx.Err(); err != nil {
		return openai.ChatCompletionResponse{}, err
	}
	c.requests++
	content := maintenanceConsolidationResponseForRequest(request, `"facts":[{"concept":"Fact One","content":"first durable fact"},{"concept":"Fact Two","content":"second durable fact"}]`)
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content},
		FinishReason: openai.FinishReasonStop,
	}}}, nil
}

func (*publicationRecoveryClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("streaming is not used by consolidation")
}

func TestDurableConsolidationResumesPublicationAfterCancellationAndRestart(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbPath := filepath.Join(t.TempDir(), "maintenance.db")
	stm, err := memory.NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	messages := insertPendingConsolidationSources(t, stm, 1, "Remember both durable publication facts.")
	message := messages[0]
	key := memory.ConsolidationCheckpointKey{
		MessageID: message.ID, SourceHash: memory.ConsolidationSourceHash(message),
		PromptVersion: consolidationProgressPromptVersion, ModelID: "gpt-4o",
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	vector := &publicationRecoveryVector{
		dedupConsolidationVectorDB: &dedupConsolidationVectorDB{},
		cancel:                     cancel,
		cancelOnAttempt:            2,
	}
	client := &publicationRecoveryClient{}
	cfg := consolidationProgressTestConfig()
	first := consolidateSTMtoLTMWithContext(cancelCtx, cfg, logger, client, stm, vector, nil)
	cancel()
	if !errors.Is(cancelCtx.Err(), context.Canceled) {
		t.Fatalf("publication context error = %v, want canceled", cancelCtx.Err())
	}
	if !errors.Is(errors.Join(first.Errors...), context.Canceled) || first.FactsStored != 1 || first.MessagesConsolidated != 0 {
		t.Fatalf("interrupted publication result = %+v; want one stored fact, no completed source, and cancellation", first)
	}
	if vector.storeAttempts != 2 || len(vector.documents) != 1 {
		t.Fatalf("first-run vector attempts=%d durable documents=%d; want two attempts and one durable fact", vector.storeAttempts, len(vector.documents))
	}
	if _, ok := vector.documents["Fact One"]; !ok {
		t.Fatalf("first durable fact missing from vector store: %v", vector.documents)
	}

	inspector, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open checkpoint inspector: %v", err)
	}
	var cursor int
	var checkpointState string
	if err := inspector.QueryRow(`SELECT publish_cursor,state FROM memory_consolidation_checkpoints WHERE message_id=? AND source_hash=? AND prompt_version=? AND model_id=?`, key.MessageID, key.SourceHash, key.PromptVersion, key.ModelID).Scan(&cursor, &checkpointState); err != nil {
		t.Fatalf("read durable publication checkpoint: %v", err)
	}
	if cursor != 1 || checkpointState != "publishing" {
		t.Fatalf("interrupted checkpoint cursor/state = %d/%q; want 1/\"publishing\"", cursor, checkpointState)
	}
	var metadataCount int
	if err := inspector.QueryRow(`SELECT COUNT(*) FROM memory_meta WHERE doc_id IN ('Fact One','Fact Two')`).Scan(&metadataCount); err != nil {
		t.Fatalf("count first-run durable fact metadata: %v", err)
	}
	if metadataCount != 1 {
		t.Fatalf("first-run durable fact metadata count = %d, want 1", metadataCount)
	}
	var consolidated bool
	var status string
	if err := inspector.QueryRow(`SELECT consolidated,consolidation_status FROM archived_messages WHERE id=?`, message.ID).Scan(&consolidated, &status); err != nil {
		t.Fatalf("read interrupted source state: %v", err)
	}
	if consolidated || status == "done" {
		t.Fatalf("source completed with only %d of 2 facts published: consolidated=%v status=%q", cursor, consolidated, status)
	}
	if err := inspector.Close(); err != nil {
		t.Fatalf("close checkpoint inspector: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("close before restart: %v", err)
	}

	stm, err = memory.NewSQLiteMemory(dbPath, logger)
	if err != nil {
		t.Fatalf("reopen memory: %v", err)
	}
	defer stm.Close()
	vector.cancelOnAttempt = 0
	resumedClient := &publicationRecoveryClient{}
	resumed := consolidateSTMtoLTMWithContext(context.Background(), cfg, logger, resumedClient, stm, vector, nil)
	if len(resumed.Errors) != 0 || resumed.FactsStored != 1 || resumed.MessagesConsolidated != 1 {
		t.Fatalf("resumed publication result = %+v; want one remaining fact and one completed source", resumed)
	}
	if resumedClient.requests != 0 {
		t.Fatalf("resume made %d extraction requests; saved complete facts should be reused", resumedClient.requests)
	}
	if vector.storeAttempts != 3 || len(vector.documents) != 2 {
		t.Fatalf("total vector attempts=%d durable documents=%d; want three attempts and exactly two facts", vector.storeAttempts, len(vector.documents))
	}
	for _, id := range []string{"Fact One", "Fact Two"} {
		if _, ok := vector.documents[id]; !ok {
			t.Fatalf("durable fact %q missing after resume: %v", id, vector.documents)
		}
	}
	inspector, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen checkpoint inspector: %v", err)
	}
	defer inspector.Close()
	metadataCount = 0
	if err := inspector.QueryRow(`SELECT COUNT(*) FROM memory_meta WHERE doc_id IN ('Fact One','Fact Two')`).Scan(&metadataCount); err != nil {
		t.Fatalf("count durable fact metadata: %v", err)
	}
	if metadataCount != 2 {
		t.Fatalf("durable fact metadata count = %d, want 2", metadataCount)
	}
	if err := inspector.QueryRow(`SELECT consolidated,consolidation_status FROM archived_messages WHERE id=?`, message.ID).Scan(&consolidated, &status); err != nil {
		t.Fatalf("read resumed source state: %v", err)
	}
	if !consolidated || status != "done" {
		t.Fatalf("source not completed after all facts: consolidated=%v status=%q", consolidated, status)
	}
	var checkpoints int
	if err := inspector.QueryRow(`SELECT COUNT(*) FROM memory_consolidation_checkpoints WHERE message_id=?`, message.ID).Scan(&checkpoints); err != nil {
		t.Fatalf("count completed checkpoints: %v", err)
	}
	if checkpoints != 0 {
		t.Fatalf("completed checkpoint rows = %d, want 0", checkpoints)
	}
}
