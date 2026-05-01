package tools

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// QuestionOption is a selectable answer shown to the user.
type QuestionOption struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// QuestionResponse is returned to the agent after the user answers or the wait times out.
type QuestionResponse struct {
	Status   string `json:"status"`
	Selected string `json:"selected"`
	FreeText string `json:"free_text,omitempty"`
}

// PendingQuestion tracks a blocking user question for a single session.
type PendingQuestion struct {
	Question      string           `json:"question"`
	Options       []QuestionOption `json:"options"`
	AllowFreeText bool             `json:"allow_free_text"`
	SessionID     string           `json:"session_id"`
	Timeout       time.Duration    `json:"-"`
	TimeoutSecs   int              `json:"timeout_seconds"`
	CreatedAt     time.Time        `json:"created_at"`
	responseCh    chan QuestionResponse
}

var pendingQuestions sync.Map

// RegisterQuestion registers a pending question and returns its response channel.
func RegisterQuestion(sessionID string, q *PendingQuestion) <-chan QuestionResponse {
	if q == nil {
		return nil
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	q.SessionID = sessionID
	if q.CreatedAt.IsZero() {
		q.CreatedAt = time.Now()
	}
	if q.TimeoutSecs <= 0 && q.Timeout > 0 {
		q.TimeoutSecs = int(q.Timeout.Seconds())
	}
	q.responseCh = make(chan QuestionResponse, 1)
	pendingQuestions.Store(sessionID, q)
	return q.responseCh
}

// CompleteQuestion writes the user's response to a pending question.
func CompleteQuestion(sessionID string, response QuestionResponse) bool {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	raw, ok := pendingQuestions.LoadAndDelete(sessionID)
	if !ok {
		return false
	}
	q, ok := raw.(*PendingQuestion)
	if !ok || q.responseCh == nil {
		return false
	}
	if strings.TrimSpace(response.Status) == "" {
		response.Status = "ok"
	}
	select {
	case q.responseCh <- response:
	default:
	}
	return true
}

// HasPendingQuestion returns true if the session has an active question.
func HasPendingQuestion(sessionID string) bool {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	_, ok := pendingQuestions.Load(sessionID)
	return ok
}

// GetPendingQuestion returns the pending question for a session.
func GetPendingQuestion(sessionID string) *PendingQuestion {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	raw, ok := pendingQuestions.Load(sessionID)
	if !ok {
		return nil
	}
	q, _ := raw.(*PendingQuestion)
	return q
}

// CancelQuestion removes a pending question without sending a response.
func CancelQuestion(sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	pendingQuestions.Delete(sessionID)
}

// ResolveQuestionReply converts a text-channel reply into a structured response.
func ResolveQuestionReply(sessionID, reply string) (QuestionResponse, bool) {
	q := GetPendingQuestion(sessionID)
	if q == nil {
		return QuestionResponse{}, false
	}
	trimmed := strings.TrimSpace(reply)
	if trimmed == "" {
		return QuestionResponse{}, false
	}
	if n, err := strconv.Atoi(trimmed); err == nil && n >= 1 && n <= len(q.Options) {
		return QuestionResponse{Status: "ok", Selected: q.Options[n-1].Value}, true
	}
	for _, opt := range q.Options {
		if strings.EqualFold(trimmed, opt.Value) || strings.EqualFold(trimmed, opt.Label) {
			return QuestionResponse{Status: "ok", Selected: opt.Value}, true
		}
	}
	if q.AllowFreeText {
		return QuestionResponse{Status: "ok", FreeText: trimmed}, true
	}
	return QuestionResponse{}, false
}
