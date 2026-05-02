package server

import (
	"encoding/json"
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

func writeChatCompletionErrorResponse(w http.ResponseWriter, sessionID, content string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Aurago-Agent-Error", "true")
	_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{
		ID:      "err-" + sessionID,
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
