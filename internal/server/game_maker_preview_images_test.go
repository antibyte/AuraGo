package server

import (
	"context"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"github.com/sashabaranov/go-openai"
)

func TestGameMakerPreviewRetainsPixelsUntilResponse(t *testing.T) {
	runner := &gameMakerAgentRunner{}
	run := agent.RunConfig{}
	runner.attachGamePreviewImages(&run, &config.Config{}, "job")
	req := openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{
		Role: openai.ChatMessageRoleUser, Name: "game_maker_preview",
		MultiContent: []openai.ChatMessagePart{{Type: openai.ChatMessagePartTypeImageURL, ImageURL: &openai.ChatMessageImageURL{URL: "data:image/jpeg;base64,fixture"}}},
	}}}
	for range 2 {
		if err := run.ExecutionHooks.BeforeContext(context.Background(), &req); err != nil {
			t.Fatal(err)
		}
		if len(req.Messages[0].MultiContent) != 1 {
			t.Fatal("request retry lost the requested screenshot")
		}
	}
	req.Messages = append(req.Messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Observed"})
	if err := run.ExecutionHooks.BeforeContext(context.Background(), &req); err != nil {
		t.Fatal(err)
	}
	if req.Messages[0].MultiContent != nil || req.Messages[0].Content == "" {
		t.Fatal("acknowledged screenshot did not become a compact reference")
	}
}
