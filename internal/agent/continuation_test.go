package agent

import (
	"reflect"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestPrivateContinuationKeepsReasoningWithinBudget(t *testing.T) {
	for _, preserve := range []bool{false, true} {
		req := openai.ChatCompletionRequest{Model: "agnes-2.0-flash", Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "Create a runner"},
			{Role: "assistant", Content: "Planned", ReasoningContent: "The runner needs obstacles"},
			{Role: "user", Content: "Continue"},
		}}
		if _, err := finalizePromptRequestForSend(&req, promptRequestFinalizationTestBudget(), nil, "openai", nil, preserve); err != nil {
			t.Fatal(err)
		}
		if (req.Messages[1].ReasoningContent != "") != preserve {
			t.Fatal("reasoning retention is not isolated to opted-in runs")
		}
	}
	req := openai.ChatCompletionRequest{Messages: []openai.ChatCompletionMessage{{Role: "assistant", Content: "plan", ReasoningContent: strings.Repeat("obstacle ", 20000)}}}
	if _, err := finalizePromptRequestForSend(&req, promptRequestFinalizationTestBudget(), nil, "openai", nil, true); err == nil {
		t.Fatal("retained reasoning bypassed token budget")
	}
}

func TestPrivateContinuationEmptyRetryDoesNotEraseContext(t *testing.T) {
	messages := []openai.ChatCompletionMessage{{Role: "user", Content: "Original runner request"}}
	for i := 0; i < 10; i++ {
		messages = append(messages, openai.ChatCompletionMessage{Role: "assistant", Content: "work", ReasoningContent: "working reasoning"})
	}
	req := openai.ChatCompletionRequest{Messages: append([]openai.ChatCompletionMessage(nil), messages...)}
	resp := openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: openai.ChatCompletionMessage{Role: "assistant"}}}}
	retried := false
	if !recoverFromEmptyResponseWithPolicy(defaultRecoveryPolicy(), resp, "", &req, &retried, nil, nil, AgentTelemetryScope{}, true) {
		t.Fatal("missing single retry")
	}
	if !reflect.DeepEqual(req.Messages, messages) {
		t.Fatal("empty response discarded working context")
	}
	if recoverFromEmptyResponseWithPolicy(defaultRecoveryPolicy(), resp, "", &req, &retried, nil, nil, AgentTelemetryScope{}, true) {
		t.Fatal("retry budget expanded")
	}
}
