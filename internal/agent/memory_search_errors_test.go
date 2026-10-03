package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

type partialSearchVectorDB struct {
	archiveFilterVectorDB
	results []memory.SearchResult
	err     error
}

func (v *partialSearchVectorDB) SearchSimilarScored(string, int, ...string) ([]memory.SearchResult, error) {
	return v.results, v.err
}
func (v *partialSearchVectorDB) SearchMemoriesOnlyScored(string, int) ([]memory.SearchResult, error) {
	return v.results, v.err
}
func (v *partialSearchVectorDB) SearchSimilar(query string, limit int, excludes ...string) ([]string, []string, error) {
	texts, ids, _, _ := splitScoredMemoryResults(v.results)
	return texts, ids, v.err
}

func TestMemorySearchConsumersKeepCheckedPartialResults(t *testing.T) {
	run, client, cleanup := newPromptPipelineTestRunConfig(t, t.Name(), "web_chat")
	defer cleanup()
	active := "Daily NAS backups run at 21:00."
	archived := "Obsolete NAS backups run at 02:00."
	searchErr := errors.New("controlled collection failure")
	vdb := &partialSearchVectorDB{results: []memory.SearchResult{
		{Text: active, DocID: "healthy", Similarity: 1},
		{Text: archived, DocID: "archived", Similarity: 1},
	}, err: searchErr}
	for _, id := range []string{"healthy", "archived"} {
		if err := run.ShortTermMem.UpsertMemoryMetaWithDetails(id, memory.MemoryMetaUpdate{ExtractionConfidence: .99, VerificationStatus: "confirmed", SourceReliability: .99}); err != nil {
			t.Fatal(err)
		}
	}
	if err := run.ShortTermMem.ApplyMemoryCurationAction(memory.MemoryCurationAction{DocID: "archived", Action: memory.MemoryCurationActionArchive}, "admin", false); err != nil {
		t.Fatal(err)
	}
	ranked, err := searchRankedMemoriesOnly(context.Background(), vdb, run.ShortTermMem, "NAS backups", 3, nil, time.Now())
	if !errors.Is(err, searchErr) || len(ranked) != 1 || ranked[0].docID != "healthy" {
		t.Fatalf("ranked partial search = %v, %v", ranked, err)
	}
	bundle := gatherMemorySourceResults("NAS backups", ToolCall{}, run.ShortTermMem, vdb, nil, nil, nil, 5, map[string]bool{"ltm": true}, nil, false, memory.TemporalQueryRange{})
	if len(bundle.Results) != 1 || bundle.Results[0].Count != 1 || len(bundle.Errors) != 1 {
		t.Fatalf("explicit partial search = %+v", bundle)
	}
	snapshot := buildContextSnapshot(context.Background(), CoAgentRequest{Task: "Explain NAS backups"}, vdb, run.ShortTermMem)
	if !strings.Contains(snapshot, active) || strings.Contains(snapshot, archived) {
		t.Fatalf("co-agent snapshot = %s", snapshot)
	}
	fusion := applyRetrievalFusion(nil, "- [nas] NAS", vdb, run.ShortTermMem, nil, run.Logger)
	if !strings.Contains(fusion.EnrichedMemories, active) || strings.Contains(fusion.EnrichedMemories, archived) {
		t.Fatalf("fusion = %+v", fusion)
	}
	run.LongTermMem = vdb
	if _, err := ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{
		Model:    run.Config.LLM.Model,
		Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Describe the current daily NAS backup schedule."}},
	}, run, false, NoopBroker{}); err != nil {
		t.Fatal(err)
	}
	var prompt strings.Builder
	for _, msg := range client.lastReq.Messages {
		prompt.WriteString(msg.Content)
	}
	if !strings.Contains(prompt.String(), active) || strings.Contains(prompt.String(), archived) {
		t.Fatal("actual agent request must include the healthy memory and exclude the archive")
	}
}
