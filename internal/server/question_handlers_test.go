package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

func TestQuestionStatusReturnsPendingQuestion(t *testing.T) {
	sessionID := "server-question-status"
	tools.RegisterQuestion(sessionID, &tools.PendingQuestion{Question: "Pick", Options: []tools.QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}})
	defer tools.CancelQuestion(sessionID)

	req := httptest.NewRequest(http.MethodGet, "/api/agent/question-status?session="+sessionID, nil)
	rec := httptest.NewRecorder()
	handleQuestionStatus(nil)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	var body struct {
		Status   string                 `json:"status"`
		Question *tools.PendingQuestion `json:"question"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "pending" || body.Question == nil || body.Question.Question != "Pick" {
		t.Fatalf("body = %+v, want pending question", body)
	}
}

func TestQuestionResponseCompletesQuestion(t *testing.T) {
	sessionID := "server-question-response"
	ch := tools.RegisterQuestion(sessionID, &tools.PendingQuestion{Question: "Pick", Options: []tools.QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}})

	body := bytes.NewBufferString(`{"session_id":"` + sessionID + `","selected_value":"a"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agent/question-response", body)
	rec := httptest.NewRecorder()
	handleQuestionResponse(nil)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	select {
	case resp := <-ch:
		if resp.Selected != "a" {
			t.Fatalf("selected = %q, want a", resp.Selected)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion")
	}
}

func TestQuestionResponseNoPendingQuestion(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/agent/question-response", bytes.NewBufferString(`{"session_id":"missing","selected_value":"a"}`))
	rec := httptest.NewRecorder()
	handleQuestionResponse(nil)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "not_found" {
		t.Fatalf("status = %q, want not_found", body["status"])
	}
}

func TestPendingQuestionChatMessageCompletesBeforeAgentRun(t *testing.T) {
	sessionID := "server-question-chat-answer"
	ch := tools.RegisterQuestion(sessionID, &tools.PendingQuestion{
		Question: "Deploy now?",
		Options:  []tools.QuestionOption{{Label: "Yes", Value: "yes"}, {Label: "No", Value: "no"}},
	})
	defer tools.CancelQuestion(sessionID)

	rec := httptest.NewRecorder()
	handled := handlePendingQuestionChatMessage(rec, openai.ChatCompletionRequest{}, sessionID, "2", nil)
	if !handled {
		t.Fatal("expected pending question chat message to be handled")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	select {
	case resp := <-ch:
		if resp.Selected != "no" {
			t.Fatalf("selected = %q, want no", resp.Selected)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending question completion")
	}
	if tools.HasPendingQuestion(sessionID) {
		t.Fatal("pending question was not cleared")
	}
}

func TestPendingQuestionChatMessageBlocksNewTaskWhenAnswerInvalid(t *testing.T) {
	sessionID := "server-question-chat-invalid"
	tools.RegisterQuestion(sessionID, &tools.PendingQuestion{
		Question: "Deploy now?",
		Options:  []tools.QuestionOption{{Label: "Yes", Value: "yes"}, {Label: "No", Value: "no"}},
	})
	defer tools.CancelQuestion(sessionID)

	rec := httptest.NewRecorder()
	handled := handlePendingQuestionChatMessage(rec, openai.ChatCompletionRequest{}, sessionID, "aktualisiere die ki news webseite", nil)
	if !handled {
		t.Fatal("expected invalid chat message to be blocked while a question is pending")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}
	if !tools.HasPendingQuestion(sessionID) {
		t.Fatal("pending question should remain active after invalid answer")
	}
	if !strings.Contains(rec.Body.String(), "Deploy now?") {
		t.Fatalf("expected reminder to include pending question, got: %s", rec.Body.String())
	}
}
