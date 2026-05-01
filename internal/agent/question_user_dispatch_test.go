package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"
)

type questionCaptureBroker struct {
	jsonMessages []string
	events       []string
}

func (b *questionCaptureBroker) Send(event, message string) {
	b.events = append(b.events, event+":"+message)
}
func (b *questionCaptureBroker) SendJSON(jsonStr string) {
	b.jsonMessages = append(b.jsonMessages, jsonStr)
}
func (b *questionCaptureBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}
func (b *questionCaptureBroker) SendLLMStreamDone(finishReason string) {}
func (b *questionCaptureBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
}
func (b *questionCaptureBroker) SendThinkingBlock(provider, content, state string) {}

func TestDispatchQuestionUserValidation(t *testing.T) {
	got := dispatchQuestionUser(ToolCall{Params: map[string]interface{}{}}, &DispatchContext{})
	if !strings.Contains(got, "question is required") {
		t.Fatalf("got %q, want missing question error", got)
	}
}

func TestDispatchQuestionUserCompletes(t *testing.T) {
	sessionID := "dispatch-question"
	broker := &questionCaptureBroker{}
	done := make(chan string, 1)
	go func() {
		done <- dispatchQuestionUser(ToolCall{Params: map[string]interface{}{
			"question":        "Pick",
			"timeout_seconds": float64(2),
			"options": []interface{}{
				map[string]interface{}{"label": "A", "value": "a"},
				map[string]interface{}{"label": "B", "value": "b"},
			},
		}}, &DispatchContext{SessionID: sessionID, MessageSource: "web_chat", Broker: broker})
	}()

	deadline := time.After(time.Second)
	for !tools.HasPendingQuestion(sessionID) {
		select {
		case <-deadline:
			t.Fatal("question was not registered")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	tools.CompleteQuestion(sessionID, tools.QuestionResponse{Selected: "b"})
	select {
	case result := <-done:
		if !strings.Contains(result, `"selected":"b"`) {
			t.Fatalf("result = %q, want selected b", result)
		}
	case <-time.After(time.Second):
		t.Fatal("dispatch did not complete")
	}
	if len(broker.jsonMessages) == 0 {
		t.Fatal("expected webchat question SSE payload")
	}
}

func TestDispatchCommHandlesQuestionUser(t *testing.T) {
	got, handled := dispatchComm(context.Background(), ToolCall{Action: "question_user"}, &DispatchContext{})
	if !handled || !strings.Contains(got, "question is required") {
		t.Fatalf("dispatchComm handled=%v got=%q", handled, got)
	}
}
