package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

type fakeActivityDigestClient struct {
	response string
	err      error
}

func (f fakeActivityDigestClient) CreateChatCompletion(_ context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	if f.err != nil {
		return openai.ChatCompletionResponse{}, f.err
	}
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{Message: openai.ChatCompletionMessage{Content: f.response}},
		},
	}, nil
}

func (f fakeActivityDigestClient) CreateChatCompletionStream(_ context.Context, _ openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return nil, nil
}

func TestParseActivityDigestResponseStripsCodeFences(t *testing.T) {
	raw := "```json\n{\"intent\":\"Deploy update\",\"user_goal\":\"Deploy update\",\"actions_taken\":[\"Ran docker deploy\"],\"outcomes\":[\"Deployment succeeded\"],\"important_points\":[\"No downtime\"],\"pending_items\":[\"Verify logs\"],\"importance\":3,\"entities\":[\"docker\"]}\n```"
	got, err := parseActivityDigestResponse(raw)
	if err != nil {
		t.Fatalf("parseActivityDigestResponse: %v", err)
	}
	if got.Intent != "Deploy update" {
		t.Fatalf("intent = %q", got.Intent)
	}
	if got.Importance != 3 {
		t.Fatalf("importance = %d", got.Importance)
	}
	if len(got.PendingItems) != 1 || got.PendingItems[0] != "Verify logs" {
		t.Fatalf("pending = %#v", got.PendingItems)
	}
}

func TestBuildActivityDigestWithLLMUsesStructuredResponse(t *testing.T) {
	client := fakeActivityDigestClient{
		response: `{"intent":"Investigate backups","user_goal":"Investigate backups","actions_taken":["Checked notes","Queried recent activity"],"outcomes":["Found the retention issue"],"important_points":["Retention policy was misconfigured"],"pending_items":["Apply the retention fix"],"importance":3,"entities":["backup","notes"]}`,
	}
	got, err := buildActivityDigestWithLLM(context.Background(), client, "test-model", "What happened with backups?", "I found the retention issue.", []string{"query_memory"}, []string{"query_memory: completed - found retention issue"})
	if err != nil {
		t.Fatalf("buildActivityDigestWithLLM: %v", err)
	}
	want := memory.ActivityDigest{
		Intent:          "Investigate backups",
		UserGoal:        "Investigate backups",
		ActionsTaken:    []string{"Checked notes", "Queried recent activity"},
		Outcomes:        []string{"Found the retention issue"},
		ImportantPoints: []string{"Retention policy was misconfigured"},
		PendingItems:    []string{"Apply the retention fix"},
		Importance:      3,
		Entities:        []string{"backup", "notes"},
	}
	if got.Intent != want.Intent || got.UserGoal != want.UserGoal || got.Importance != want.Importance {
		t.Fatalf("got digest = %#v", got)
	}
	if len(got.ActionsTaken) != len(want.ActionsTaken) || got.ActionsTaken[0] != want.ActionsTaken[0] {
		t.Fatalf("actions = %#v", got.ActionsTaken)
	}
	if len(got.PendingItems) != 1 || got.PendingItems[0] != "Apply the retention fix" {
		t.Fatalf("pending = %#v", got.PendingItems)
	}
}

func TestCaptureActivityTurnWithDigestSyncsEntitiesToKnowledgeGraph(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	captureActivityTurnWithDigest(
		stm,
		kg,
		"session-1",
		"web_chat",
		"Please check the backup host",
		[]string{"query_memory"},
		false,
		true,
		memory.ActivityDigest{
			Intent:   "Check backup host",
			UserGoal: "Verify backup health",
			Entities: []string{"Backup Host", "Proxmox VE"},
		},
		"runtime_helper_batch",
	)

	nodes, err := kg.GetAllNodes(20)
	if err != nil {
		t.Fatalf("GetAllNodes: %v", err)
	}
	edges, err := kg.GetAllEdges(20)
	if err != nil {
		t.Fatalf("GetAllEdges: %v", err)
	}

	foundEntity := false
	for _, node := range nodes {
		if node.ID == "backup_host" || node.ID == "proxmox_ve" {
			foundEntity = true
		}
	}
	if !foundEntity {
		t.Fatal("expected activity entity nodes in knowledge graph")
	}

	foundCoOccurrence := false
	for _, edge := range edges {
		if edge.Relation == "co_mentioned_with" {
			foundCoOccurrence = true
			break
		}
	}
	if !foundCoOccurrence {
		t.Fatal("expected co_mentioned_with edge between entities in knowledge graph")
	}

	forbiddenTurn := false
	for _, node := range nodes {
		if strings.HasPrefix(node.ID, "activity_turn_") {
			forbiddenTurn = true
			break
		}
	}
	if forbiddenTurn {
		t.Fatal("activity_turn_* nodes should no longer be created")
	}
}
