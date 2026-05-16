package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

type questionResponseRequest struct {
	SessionID     string `json:"session_id"`
	SelectedValue string `json:"selected_value"`
	FreeText      string `json:"free_text"`
}

func handleQuestionStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sessionID := strings.TrimSpace(r.URL.Query().Get("session"))
		if sessionID == "" {
			sessionID = "default"
		}
		q := tools.GetPendingQuestion(sessionID)
		if q == nil {
			writeJSON(w, map[string]interface{}{"status": "none"})
			return
		}
		writeJSON(w, map[string]interface{}{"status": "pending", "question": q})
	}
}

func handleQuestionResponse(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req questionResponseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.SessionID) == "" {
			req.SessionID = "default"
		}
		resp := tools.QuestionResponse{Status: "ok", Selected: strings.TrimSpace(req.SelectedValue), FreeText: strings.TrimSpace(req.FreeText)}
		if !tools.CompleteQuestion(req.SessionID, resp) {
			writeJSON(w, map[string]interface{}{"status": "not_found"})
			return
		}
		writeJSON(w, map[string]interface{}{"status": "ok"})
	}
}

func handlePendingQuestionChatMessage(w http.ResponseWriter, req openai.ChatCompletionRequest, sessionID, message string, logger *slog.Logger) bool {
	if !tools.HasPendingQuestion(sessionID) {
		return false
	}
	if response, ok := tools.ResolveQuestionReply(sessionID, message); ok {
		tools.CompleteQuestion(sessionID, response)
		if logger != nil {
			logger.Info("[QuestionUser] Completed pending question from chat message", "session_id", sessionID)
		}
		writeChatCompletionTextResponse(w, req, sessionID, "Danke, ich mache mit deiner Antwort weiter.")
		return true
	}
	q := tools.GetPendingQuestion(sessionID)
	if logger != nil {
		logger.Info("[QuestionUser] Chat message blocked because a pending question is waiting for a valid answer", "session_id", sessionID)
	}
	writeChatCompletionTextResponse(w, req, sessionID, formatPendingQuestionReminder(q))
	return true
}

func formatPendingQuestionReminder(q *tools.PendingQuestion) string {
	if q == nil {
		return "Ich warte noch auf deine Antwort auf die offene Frage."
	}
	var b strings.Builder
	b.WriteString("Ich warte noch auf deine Antwort auf die offene Frage:\n\n")
	b.WriteString(strings.TrimSpace(q.Question))
	if len(q.Options) > 0 {
		b.WriteString("\n\nBitte antworte mit einer der Optionen:")
		for i, opt := range q.Options {
			label := strings.TrimSpace(opt.Label)
			if label == "" {
				label = strings.TrimSpace(opt.Value)
			}
			if label == "" {
				label = fmt.Sprintf("Option %d", i+1)
			}
			b.WriteString(fmt.Sprintf("\n%d. %s", i+1, label))
		}
	}
	if q.AllowFreeText {
		b.WriteString("\n\nFreitext ist ebenfalls möglich.")
	}
	return b.String()
}
