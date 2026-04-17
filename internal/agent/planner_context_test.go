package agent

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/planner"

	openai "github.com/sashabaranov/go-openai"
)

func TestIsFirstUserMessageInSession(t *testing.T) {
	if !isFirstUserMessageInSession([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
	}) {
		t.Fatal("expected single user message to count as first session message")
	}

	if isFirstUserMessageInSession([]openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "hello"},
		{Role: openai.ChatMessageRoleAssistant, Content: "hi"},
		{Role: openai.ChatMessageRoleUser, Content: "follow-up"},
	}) {
		t.Fatal("did not expect existing conversation history to count as first session message")
	}
}

func TestPlannerPromptContextTextUsesTriggers(t *testing.T) {
	db := newPlannerTestDB(t)
	defer db.Close()

	if _, err := planner.CreateTodo(db, planner.Todo{
		Title:    "Patch planner context",
		Priority: "high",
		Status:   "open",
		DueDate:  "2026-04-18T10:00:00Z",
	}); err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}
	if _, err := planner.CreateAppointment(db, planner.Appointment{
		Title:    "Planner review",
		DateTime: "2026-04-18T09:00:00Z",
		Status:   "upcoming",
	}); err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	runCfg := RunConfig{PlannerDB: db, MessageSource: "web_chat"}
	now := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)

	firstTurn := plannerPromptContextText(runCfg, "hello", now, true, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !strings.Contains(firstTurn, "Patch planner context") {
		t.Fatalf("firstTurn = %q, want todo in planner context", firstTurn)
	}

	keywordTurn := plannerPromptContextText(runCfg, "what is open today?", now, false, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if !strings.Contains(keywordTurn, "Open todos: 1") {
		t.Fatalf("keywordTurn = %q, want planner summary", keywordTurn)
	}

	nonTrigger := plannerPromptContextText(runCfg, "tell me a joke", now, false, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if nonTrigger != "" {
		t.Fatalf("nonTrigger = %q, want no planner context", nonTrigger)
	}
}
