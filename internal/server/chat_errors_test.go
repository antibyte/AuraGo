package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/i18n"
	"aurago/ui"

	"github.com/sashabaranov/go-openai"
)

func TestChatCompletionErrorMessageQuotaExceeded(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())

	err := &openai.APIError{
		HTTPStatusCode: http.StatusTooManyRequests,
		Message:        `geminiException - {"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details. Quota exceeded for metric: generativelanguage.googleapis.com/generate_content_paid_tier_3_input_token_count, limit: 16000, model: gemma-4-31b","status":"RESOURCE_EXHAUSTED"}}`,
	}

	msg := chatCompletionErrorMessage("de", err)
	if msg == "" || strings.Contains(msg, "backend.") {
		t.Fatalf("chatCompletionErrorMessage returned untranslated message: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "kontingent") && !strings.Contains(strings.ToLower(msg), "rate-limit") {
		t.Fatalf("quota error message should mention quota/rate limit, got %q", msg)
	}
}

func TestChatCompletionErrorMessageProviderConfig(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())

	err := &openai.APIError{
		HTTPStatusCode: http.StatusBadRequest,
		Message:        "GenerateContentRequest.tools[0].function_declarations[15].name: Invalid function name",
	}

	msg := chatCompletionErrorMessage("en", err)
	if msg == "" || strings.Contains(msg, "backend.") {
		t.Fatalf("chatCompletionErrorMessage returned untranslated message: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "provider") || !strings.Contains(strings.ToLower(msg), "configuration") {
		t.Fatalf("config error message should mention provider configuration, got %q", msg)
	}
}

func TestWriteChatCompletionErrorResponseMarksAgentError(t *testing.T) {
	rec := httptest.NewRecorder()

	writeChatCompletionErrorResponse(rec, "session-1", "provider error")

	if got := rec.Header().Get("X-Aurago-Agent-Error"); got != "true" {
		t.Fatalf("X-Aurago-Agent-Error = %q, want true", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
