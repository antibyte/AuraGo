package agent

import (
	"context"
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
