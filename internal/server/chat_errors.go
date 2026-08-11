package server

import (
	"aurago/internal/agent"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aurago/internal/i18n"
	"aurago/internal/llm"

	"github.com/sashabaranov/go-openai"
)

func chatCompletionErrorMessage(lang string, err error) string {
	switch {
	case err == nil:
		return i18n.T(lang, "backend.handler_sync_error")
	case agent.IsContextBudgetExceeded(err):
		return i18n.T(lang, "backend.handler_context_budget_error")
	case llm.IsImageNotSupportedError(err):
		return i18n.T(lang, "backend.handler_image_not_supported")
	case llm.IsQuotaExceeded(err):
		return i18n.T(lang, "backend.handler_llm_quota_error")
	case llm.IsAuthError(err):
		return i18n.T(lang, "backend.handler_llm_auth_error")
	case llm.ClassifyError(err) == llm.ErrCategoryNonRetryableConfig:
		return i18n.T(lang, "backend.handler_llm_config_error")
	case strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "context canceled"):
		return i18n.T(lang, "backend.handler_timeout_error")
	default:
		return i18n.T(lang, "backend.handler_sync_error")
	}
}

func writeChatCompletionStreamError(w http.ResponseWriter, code, message string, status int) {
	errorType := "invalid_request_error"
	if status >= http.StatusInternalServerError {
		errorType = "server_error"
	}
	payload := map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    errorType,
			"code":    code,
			"status":  status,
		},
	}
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}

func emitStreamedAgentError(w http.ResponseWriter, flusher http.Flusher, broker agent.FeedbackBroker, sessionID, lang string, err error) {
	message := chatCompletionErrorMessage(lang, err)
	code := "agent_execution_failed"
	status := http.StatusInternalServerError
	if agent.IsContextBudgetExceeded(err) {
		code = "context_budget_exceeded"
		status = http.StatusRequestEntityTooLarge
	}
	if typed, ok := broker.(agent.TypedFeedbackBroker); ok {
		_ = typed.SendTyped(string(EventAgentError), AgentErrorPayload{
			SessionID: sessionID,
			Code:      code,
			Message:   message,
			Status:    status,
		})
	}
	writeChatCompletionStreamError(w, code, message, status)
	flusher.Flush()
	broker.Send("done", i18n.T(lang, "backend.stream_done"))
}

func writeChatCompletionErrorResponse(w http.ResponseWriter, sessionID, content string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Aurago-Agent-Error", "true")
	writeChatCompletionJSONResponse(w, "err-"+sessionID, content)
}

func writeChatCompletionTextResponse(w http.ResponseWriter, req openai.ChatCompletionRequest, sessionID, content string) {
	if req.Stream {
		writeChatCompletionStreamTextResponse(w, sessionID, content)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeChatCompletionJSONResponse(w, "msg-"+sessionID, content)
}

func writeChatCompletionJSONResponse(w http.ResponseWriter, id, content string) {
	_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
		ID:      id,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "aurago",
		Choices: []openai.ChatCompletionChoice{{
			Index: 0,
			Message: openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: content,
			},
			FinishReason: openai.FinishReasonStop,
		}},
	})
}

func writeChatCompletionStreamTextResponse(w http.ResponseWriter, sessionID, content string) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	id := "msg-" + sessionID
	created := time.Now().Unix()
	writeChunk := func(payload map[string]interface{}) {
		data, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	}
	writeChunk(map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   "aurago",
		"choices": []map[string]interface{}{{
			"index": 0,
			"delta": map[string]interface{}{
				"role":    openai.ChatMessageRoleAssistant,
				"content": content,
			},
			"finish_reason": nil,
		}},
	})
	writeChunk(map[string]interface{}{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   "aurago",
		"choices": []map[string]interface{}{{
			"index":         0,
			"delta":         map[string]interface{}{},
			"finish_reason": openai.FinishReasonStop,
		}},
	})
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
}
