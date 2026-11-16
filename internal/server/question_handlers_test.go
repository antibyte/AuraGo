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

	body := bytes.NewBufferString(`{"question_id":"` + tools.GetPendingQuestion(sessionID).QuestionID + `","session_id":"` + sessionID + `","selected_value":"a"}`)
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
		if resp.Source != tools.QuestionSourceWeb {
			t.Fatalf("source = %q, want web", resp.Source)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion")
	}
}

// A loopback call with valid internal headers on the question-response route
// is labelled like the same call on /v1/chat/completions.
func TestQuestionResponseRejectsInternalLoopbackAnswers(t *testing.T) {
	s := newOperatorCommandTestServer(t)
	const sessionID = "server-question-response-internal"
	ch := tools.RegisterQuestion(sessionID, &tools.PendingQuestion{Question: "Pick", AllowFreeText: true, Options: []tools.QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}})
	defer tools.CancelQuestion(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/api/agent/question-response", bytes.NewBufferString(`{"question_id":"`+tools.GetPendingQuestion(sessionID).QuestionID+`","session_id":"`+sessionID+`","free_text":"blue"}`))
	req.RemoteAddr = "127.0.0.1:43210"
	req.Header.Set("X-Internal-FollowUp", "true")
	req.Header.Set("X-Internal-Token", s.internalToken)
	handleQuestionResponse(s)(httptest.NewRecorder(), req)
	select {
	case resp := <-ch:
		t.Fatalf("internal loopback answered owner question: %+v", resp)
	default:
		if !tools.HasPendingQuestion(sessionID) {
			t.Fatal("internal answer removed question")
		}
	}
}

// The real /v1/chat/completions call site labels an owner reply as web and a
// loopback follow-up or mission turn as internal.
func TestChatCompletionsLabelsQuestionAnswerSource(t *testing.T) {
	s := newOperatorCommandTestServer(t)
	followUp := map[string]string{"X-Internal-FollowUp": "true", "X-Internal-Token": s.internalToken}
	mission := map[string]string{"X-Internal-FollowUp": "true", "X-Internal-Token": s.internalToken, "X-Mission-ID": "m1", "X-Session-ID": "default"}
	for _, tc := range []struct {
		name    string
		headers map[string]string
		want    tools.QuestionSource
	}{
		{"follow-up", followUp, tools.QuestionSourceInternal},
		{"mission", mission, tools.QuestionSourceInternal},
		{"owner", nil, tools.QuestionSourceWeb},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := tools.RegisterQuestion("default", &tools.PendingQuestion{Question: "Pick", AllowFreeText: true, Options: []tools.QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}})
			defer tools.CancelQuestion("default")
			postChatCommand(t, s, "blue", tc.headers)
			if tc.want == tools.QuestionSourceInternal {
				if !tools.HasPendingQuestion("default") {
					t.Fatal("internal turn removed question")
				}
				return
			}
			select {
			case resp := <-ch:
				if resp.FreeText != "blue" || resp.Source != tc.want {
					t.Fatalf("response = %+v, want free text blue from %q", resp, tc.want)
				}
			case <-time.After(time.Second):
				t.Fatal("pending question was not completed")
			}
		})
	}
}

func TestQuestionResponseLabelsDesktopChatAnswers(t *testing.T) {
	ch := tools.RegisterQuestion(desktopChatSessionID, &tools.PendingQuestion{Question: "Pick", AllowFreeText: true, Options: []tools.QuestionOption{{Label: "A", Value: "a"}, {Label: "B", Value: "b"}}})
	defer tools.CancelQuestion(desktopChatSessionID)

	body := bytes.NewBufferString(`{"question_id":"` + tools.GetPendingQuestion(desktopChatSessionID).QuestionID + `","session_id":"` + desktopChatSessionID + `","free_text":"blue"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agent/question-response", body)
	rec := httptest.NewRecorder()
	handleQuestionResponse(nil)(rec, req)
	select {
	case resp := <-ch:
		if resp.FreeText != "blue" || resp.Source != tools.QuestionSourceDesktop {
			t.Fatalf("response = %+v, want free text from desktop", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion")
	}
}

func TestChatCompletionQuestionAnswerSource(t *testing.T) {
	for _, tc := range []struct {
		followUp  bool
		missionID string
		want      tools.QuestionSource
	}{
		{false, "", tools.QuestionSourceWeb},
		{true, "", tools.QuestionSourceInternal},
		{true, "mission-1", tools.QuestionSourceInternal},
		{false, "mission-1", tools.QuestionSourceInternal},
	} {
		if got := chatCompletionQuestionAnswerSource(tc.followUp, tc.missionID); got != tc.want {
			t.Errorf("chatCompletionQuestionAnswerSource(%v, %q) = %q, want %q", tc.followUp, tc.missionID, got, tc.want)
		}
	}
}

func TestQuestionResponseNoPendingQuestion(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/agent/question-response", bytes.NewBufferString(`{"session_id":"missing","question_id":"stale","selected_value":"a"}`))
	rec := httptest.NewRecorder()
	handleQuestionResponse(nil)(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestQuestionResponseRejectsMissingStaleAndUnlistedAnswers(t *testing.T) {
	const session = "http-question-validation"
	q := &tools.PendingQuestion{Question: "Continue?", Options: []tools.QuestionOption{{Value: "yes"}}}
	tools.RegisterQuestion(session, q)
	defer tools.CancelQuestion(session)
	for _, tc := range []struct{ id, selected, freeText string }{
		{"", "yes", ""}, {"stale", "yes", ""}, {q.QuestionID, "arbitrary", ""}, {q.QuestionID, "yes", "forbidden free text"},
	} {
		body, err := json.Marshal(questionResponseRequest{SessionID: session, QuestionID: tc.id, SelectedValue: tc.selected, FreeText: tc.freeText})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		handleQuestionResponse(nil)(rec, httptest.NewRequest(http.MethodPost, "/api/agent/question-response", bytes.NewReader(body)))
		if rec.Code != http.StatusConflict || tools.GetPendingQuestion(session) != q {
			t.Fatal("invalid answer consumed pending question")
		}
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
	handled := handlePendingQuestionChatMessage(rec, openai.ChatCompletionRequest{}, sessionID, "2", tools.QuestionSourceWeb, nil)
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
		if resp.Source != tools.QuestionSourceWeb {
			t.Fatalf("source = %q, want web", resp.Source)
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
	handled := handlePendingQuestionChatMessage(rec, openai.ChatCompletionRequest{}, sessionID, "aktualisiere die ki news webseite", tools.QuestionSourceWeb, nil)
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
