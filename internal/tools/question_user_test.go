package tools

import (
	"testing"
	"time"
)

func TestQuestionLifecycle(t *testing.T) {
	sessionID := "question-lifecycle"
	ch := RegisterQuestion(sessionID, &PendingQuestion{Question: "Pick", Options: []QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}})
	if !HasPendingQuestion(sessionID) {
		t.Fatal("expected pending question")
	}
	if GetPendingQuestion(sessionID) == nil {
		t.Fatal("expected pending question details")
	}
	if !CompleteQuestion(sessionID, QuestionResponse{Selected: "b"}) {
		t.Fatal("expected question completion")
	}
	select {
	case resp := <-ch:
		if resp.Status != "ok" || resp.Selected != "b" {
			t.Fatalf("response = %+v, want ok/b", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for response")
	}
	if HasPendingQuestion(sessionID) {
		t.Fatal("question should be removed after completion")
	}
}

func TestResolveQuestionReply(t *testing.T) {
	sessionID := "question-reply"
	RegisterQuestion(sessionID, &PendingQuestion{
		Question:      "Pick",
		AllowFreeText: true,
		Options:       []QuestionOption{{Label: "Alpha", Value: "a"}, {Label: "Beta", Value: "b"}},
	})
	defer CancelQuestion(sessionID)

	resp, ok := ResolveQuestionReply(sessionID, "2")
	if !ok || resp.Selected != "b" {
		t.Fatalf("numeric response = %+v/%v, want selected b", resp, ok)
	}
	resp, ok = ResolveQuestionReply(sessionID, "custom")
	if !ok || resp.FreeText != "custom" {
		t.Fatalf("free text response = %+v/%v, want custom", resp, ok)
	}
}

func TestConcurrentQuestionsAreIsolated(t *testing.T) {
	chA := RegisterQuestion("question-a", &PendingQuestion{Question: "A", Options: []QuestionOption{{Label: "One", Value: "1"}, {Label: "Two", Value: "2"}}})
	chB := RegisterQuestion("question-b", &PendingQuestion{Question: "B", Options: []QuestionOption{{Label: "Three", Value: "3"}, {Label: "Four", Value: "4"}}})
	defer CancelQuestion("question-a")
	defer CancelQuestion("question-b")

	CompleteQuestion("question-b", QuestionResponse{Selected: "4"})
	select {
	case resp := <-chB:
		if resp.Selected != "4" {
			t.Fatalf("question-b selected = %q, want 4", resp.Selected)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for question-b")
	}
	select {
	case resp := <-chA:
		t.Fatalf("question-a should not receive response, got %+v", resp)
	default:
	}
}
