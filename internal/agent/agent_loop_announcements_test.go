package agent

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
)

func TestAnnouncementDetectorIgnoresPhraseOnlyPromises(t *testing.T) {
	tc := ToolCall{}
	cases := []string{
		"I will update the file now.",
		"Ich werde die Datei jetzt aktualisieren.",
	}
	for _, content := range cases {
		if isAnnouncementOnlyResponse(content, tc, false, false, "continue") {
			t.Fatalf("phrase-only response must not trigger structural announcement recovery: %q", content)
		}
	}
}

func TestAnnouncementDetectorCatchesStructuredPlanWithoutToolCall(t *testing.T) {
	tc := ToolCall{}
	content := "1. Build production bundle\n2. Deploy to Netlify\n3. Verify homepage"
	if !isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("expected numbered plan without tool call to trigger structural announcement recovery")
	}
}

func TestAnnouncementDetectorCatchesPathPlanWithoutToolCall(t *testing.T) {
	tc := ToolCall{}
	content := "Plan:\n- Update src/App.tsx\n- Run npm test\n- Open http://localhost:3000"
	if !isAnnouncementOnlyResponse(content, tc, false, false, "continue") {
		t.Fatal("expected structured path/url plan without tool call to trigger announcement recovery")
	}
}

func TestAnnouncementDetectorIgnoresCompletionSummaryWithURLAndMetrics(t *testing.T) {
	tc := ToolCall{}
	content := "Completed successfully.\n- 3 files changed\n- HTTP 200\n- Preview: http://localhost:8080/app/"
	if isAnnouncementOnlyResponse(content, tc, false, true, "continue") {
		t.Fatal("completion summary with URL/status/metrics must not trigger announcement recovery")
	}
}

func TestAnnouncementDetectorIgnoresPlainPathReference(t *testing.T) {
	tc := ToolCall{}
	content := "The config.yaml file looks correct to me."
	if isAnnouncementOnlyResponse(content, tc, false, false, "check config") {
		t.Fatal("plain path reference should not trigger announcement recovery")
	}
}

func TestAsksUserForInputUsesOnlyVisibleQuestionMark(t *testing.T) {
	if !asksUserForInput("Should I deploy now?") {
		t.Fatal("visible question mark should be treated as user input")
	}
	if asksUserForInput("Continue after the build finishes.") {
		t.Fatal("question detection must require a visible question mark")
	}
}

func TestAnnouncementDetectorPureThinkBlockDoesNotTrigger(t *testing.T) {
	tc := ToolCall{}
	if isAnnouncementOnlyResponse("", tc, true, false, "absolute stille") {
		t.Fatal("empty sanitized content (pure think-block) must not trigger announcement recovery")
	}
	if isAnnouncementOnlyResponse("", tc, false, true, "weiter") {
		t.Fatal("empty sanitized content (pure think-block) post-tool must not trigger announcement recovery")
	}
}

func TestDesktopAnnouncementRecoveryRejectsDoneWithoutToolAfterPromise(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AnnouncementDetector.Enabled = true
	cfg.Agent.AnnouncementDetector.MaxRetries = 2
	logger := slog.New(slog.NewTextHandler(testDiscardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	s := &agentLoopState{
		ctx:                context.Background(),
		broker:             NoopBroker{},
		currentLogger:      logger,
		useNativeFunctions: true,
		announcementCount:  1,
		lastUserMsg:        "ERROR: Your last response was text-only - use the native function-calling mechanism NOW.",
		recoverySession:    NewRecoverySessionState(logger, NoopBroker{}, cfg),
		runCfg: RunConfig{
			Config:        cfg,
			SessionID:     "virtual-desktop",
			MessageSource: "virtual_desktop_chat",
			ShortTermMem:  stm,
		},
	}

	content := "I need the task again before I can continue. <done/>"
	parsed := ParsedToolResponse{
		Content:          content,
		SanitizedContent: strings.ReplaceAll(content, " <done/>", ""),
		IsFinished:       true,
	}

	_, _, shouldContinue, _ := handleAgentLoopRecoveries(s, content, ToolCall{}, parsed, true, emotionBehaviorPolicy{})
	if !shouldContinue {
		t.Fatal("expected desktop recovery to reject <done/> when no desktop tool ran after an action promise")
	}
	if s.announcementCount != 2 {
		t.Fatalf("announcementCount = %d, want 2", s.announcementCount)
	}
	if len(s.req.Messages) == 0 || !strings.Contains(s.req.Messages[len(s.req.Messages)-1].Content, "virtual_desktop") {
		t.Fatalf("expected recovery feedback to require a desktop tool call, got %#v", s.req.Messages)
	}
}

func TestDesktopEmptyResponseAfterToolRequiresRecovery(t *testing.T) {
	runCfg := RunConfig{MessageSource: "virtual_desktop_chat"}

	if !shouldAbortDesktopEmptyAfterTool(runCfg, "", true) {
		t.Fatal("expected empty desktop response after a tool call to require recovery")
	}
	if !shouldAbortDesktopEmptyAfterTool(runCfg, "<think>still thinking", true) {
		t.Fatal("expected reasoning-only desktop response after a tool call to require recovery")
	}
	if shouldAbortDesktopEmptyAfterTool(runCfg, "done", true) {
		t.Fatal("did not expect visible content to require empty-response recovery")
	}
	if shouldAbortDesktopEmptyAfterTool(runCfg, "", false) {
		t.Fatal("did not expect pre-tool empty response to use the desktop post-tool guard")
	}
	if shouldAbortDesktopEmptyAfterTool(RunConfig{MessageSource: "web_chat"}, "", true) {
		t.Fatal("did not expect non-desktop chat to use the desktop post-tool guard")
	}
}

type testDiscardWriter struct{}

func (testDiscardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
