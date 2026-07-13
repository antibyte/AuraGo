package agent

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/tools"
)

type questionCaptureBroker struct {
	mu           sync.RWMutex
	jsonMessages []string
	events       []string
}

func (b *questionCaptureBroker) Send(event, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event+":"+message)
}
func (b *questionCaptureBroker) SendJSON(jsonStr string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.jsonMessages = append(b.jsonMessages, jsonStr)
}
func (b *questionCaptureBroker) jsonMessagesSnapshot() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]string(nil), b.jsonMessages...)
}
func (b *questionCaptureBroker) eventsSnapshot() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]string(nil), b.events...)
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
	if len(broker.jsonMessagesSnapshot()) == 0 {
		t.Fatal("expected webchat question SSE payload")
	}
}

func TestDispatchQuestionUserUsesInteractiveUIForVirtualDesktop(t *testing.T) {
	sessionID := "dispatch-question-desktop"
	broker := &questionCaptureBroker{}
	done := make(chan string, 1)
	go func() {
		done <- dispatchQuestionUser(ToolCall{Params: map[string]interface{}{
			"question": "Pick",
			"options": []interface{}{
				map[string]interface{}{"label": "A", "value": "a"},
				map[string]interface{}{"label": "B", "value": "b"},
			},
		}}, &DispatchContext{SessionID: sessionID, MessageSource: "virtual_desktop_chat", Broker: broker})
	}()

	deadline := time.After(time.Second)
	var pending *tools.PendingQuestion
	for pending == nil {
		pending = tools.GetPendingQuestion(sessionID)
		select {
		case <-deadline:
			t.Fatal("desktop question was not registered")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if pending.TimeoutSecs != 120 {
		t.Fatalf("desktop question timeout = %d, want 120", pending.TimeoutSecs)
	}
	jsonDeadline := time.After(time.Second)
	var jsonMessages []string
	for len(jsonMessages) == 0 {
		jsonMessages = broker.jsonMessagesSnapshot()
		select {
		case <-jsonDeadline:
			t.Fatalf("expected desktop question to use interactive JSON payload, got %#v", jsonMessages)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	if !strings.Contains(jsonMessages[0], `"type":"question_user"`) {
		t.Fatalf("expected desktop question JSON payload, got %#v", jsonMessages)
	}
	if events := broker.eventsSnapshot(); len(events) != 0 {
		t.Fatalf("did not expect text-channel question event for desktop, got %#v", events)
	}
	tools.CompleteQuestion(sessionID, tools.QuestionResponse{Selected: "a"})
	select {
	case result := <-done:
		if !strings.Contains(result, `"selected":"a"`) {
			t.Fatalf("result = %q, want selected a", result)
		}
	case <-time.After(time.Second):
		t.Fatal("desktop question dispatch did not complete")
	}
}

func TestDispatchCommHandlesQuestionUser(t *testing.T) {
	got, handled := dispatchComm(context.Background(), ToolCall{Action: "question_user"}, &DispatchContext{})
	if !handled || !strings.Contains(got, "question is required") {
		t.Fatalf("dispatchComm handled=%v got=%q", handled, got)
	}
}
