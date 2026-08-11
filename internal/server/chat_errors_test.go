package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/i18n"
	"aurago/ui"

	"github.com/sashabaranov/go-openai"
)

type streamedErrorCaptureBroker struct {
	typedEvent string
	typed      AgentErrorPayload
	done       bool
}

func (b *streamedErrorCaptureBroker) Send(event, _ string) { b.done = event == "done" }
func (*streamedErrorCaptureBroker) SendJSON(string)        {}
func (*streamedErrorCaptureBroker) SendLLMStreamDelta(string, string, string, int, string) {
}
func (*streamedErrorCaptureBroker) SendLLMStreamDone(string) {}
func (*streamedErrorCaptureBroker) SendTokenUpdate(int, int, int, int, int, bool, bool, string) {
}
func (*streamedErrorCaptureBroker) SendThinkingBlock(string, string, string) {}
func (b *streamedErrorCaptureBroker) SendTyped(event string, payload interface{}) bool {
	b.typedEvent = event
	b.typed, _ = payload.(AgentErrorPayload)
	return true
}

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

func TestEmitStreamedContextBudgetErrorUsesErrorFrameAndTypedEvent(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	rec := httptest.NewRecorder()
	broker := &streamedErrorCaptureBroker{}
	err := &agent.ContextBudgetExceededError{
		Model: "secret-model-name", ContextWindow: 1024, RequiredInput: 1500,
	}

	emitStreamedAgentError(rec, rec, broker, "session-1", "de", err)
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"context_budget_exceeded"`) || !strings.Contains(body, `"status":413`) {
		t.Fatalf("missing structured context error: %s", body)
	}
	if !strings.Contains(body, `"type":"invalid_request_error"`) {
		t.Fatalf("413 frame has wrong error type: %s", body)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("stream does not end with DONE: %q", body)
	}
	if strings.Contains(body, "secret-model-name") || strings.Contains(body, `"choices"`) || strings.Contains(body, `"delta"`) {
		t.Fatalf("stream leaked diagnostics or assistant delta: %s", body)
	}
	if broker.typedEvent != string(EventAgentError) || broker.typed.Code != "context_budget_exceeded" || broker.typed.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("typed event = %q %+v", broker.typedEvent, broker.typed)
	}
	if !broker.done {
		t.Fatal("legacy done event was not emitted")
	}
	frames := strings.Split(strings.TrimSpace(body), "\n\n")
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(frames[0], "data: ")), &envelope); err != nil {
		t.Fatalf("invalid SSE JSON frame: %v", err)
	}
}

func TestEmitStreamedServerErrorUsesServerErrorFrameAndTypedEvent(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())
	rec := httptest.NewRecorder()
	broker := &streamedErrorCaptureBroker{}

	emitStreamedAgentError(rec, rec, broker, "session-2", "en", errors.New("SECRET_PROMPT_CONTENT"))
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"agent_execution_failed"`) ||
		!strings.Contains(body, `"status":500`) ||
		!strings.Contains(body, `"type":"server_error"`) {
		t.Fatalf("missing structured server error: %s", body)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("stream does not end with DONE: %q", body)
	}
	if strings.Contains(body, "SECRET_PROMPT_CONTENT") || strings.Contains(body, `"choices"`) || strings.Contains(body, `"delta"`) {
		t.Fatalf("stream leaked diagnostics or assistant delta: %s", body)
	}
	if broker.typedEvent != string(EventAgentError) || broker.typed.Code != "agent_execution_failed" || broker.typed.Status != http.StatusInternalServerError {
		t.Fatalf("typed event = %q %+v", broker.typedEvent, broker.typed)
	}
	if !broker.done {
		t.Fatal("legacy done event was not emitted")
	}
}
