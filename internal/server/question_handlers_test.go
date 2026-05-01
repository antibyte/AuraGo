package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/tools"
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
