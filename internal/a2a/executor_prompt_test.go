package a2a

import (
	"testing"

	"aurago/internal/prompts"

	"github.com/sashabaranov/go-openai"
)

func TestBuildA2APromptRequestUsesUserMessageAndTrustedAddendum(t *testing.T) {
	req, addenda := buildA2APromptRequest("test-model", "<external_data>task</external_data>", "A2A runtime contract")
	if len(req.Messages) != 1 || req.Messages[0].Role != openai.ChatMessageRoleUser {
		t.Fatalf("A2A messages = %#v, want one user message", req.Messages)
	}
	if len(addenda) != 1 || addenda[0].ID != prompts.PromptAddendumA2A || addenda[0].Text != "A2A runtime contract" {
		t.Fatalf("A2A addenda = %#v", addenda)
	}
}
