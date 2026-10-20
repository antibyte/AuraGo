package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/planner"

	"github.com/sashabaranov/go-openai"
)

type consolidationProgressTestClient struct {
	calls       []string
	requests    int
	cancelAfter int
	cancel      context.CancelFunc
}

var consolidationProgressMarker = regexp.MustCompile(`source message_id=(\d+)\s+part=(\d+)`)

func (c *consolidationProgressTestClient) CreateChatCompletion(_ context.Context, request openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	prompt := ""
	if len(request.Messages) > 1 {
		prompt = request.Messages[1].Content
	}
	matches := consolidationProgressMarker.FindAllStringSubmatch(prompt, -1)
	if len(matches) == 0 {
		c.calls = append(c.calls, "unknown")
	} else {
		for _, match := range matches {
			c.calls = append(c.calls, match[1]+"/"+match[2])
		}
	}
	c.requests++
	content := maintenanceConsolidationResponseForRequest(request, `"facts":[{"concept":"stable fact","content":"a durable fact"}]`)
	response := openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{
		Message:      openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content},
		FinishReason: openai.FinishReasonStop,
	}}}
	if c.cancelAfter > 0 && c.requests == c.cancelAfter && c.cancel != nil {
		c.cancel()
	}
	return response, nil
}

func (*consolidationProgressTestClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("streaming is not used by consolidation")
}

func TestDurableConsolidationProcessesPartsFairlyAcrossMessages(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	messages := insertPendingConsolidationSources(t, stm, 2, strings.Repeat("source ", 1200))
	if len(serializeConsolidationSources(messages[:1])) < 2 {
		t.Fatal("test source did not split into multiple parts")
	}
	cfg := consolidationProgressTestConfig()
	client := &consolidationProgressTestClient{}
	vector := &dedupConsolidationVectorDB{}
	result := consolidateSTMtoLTMWithContext(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), client, stm, vector, nil)
	if len(result.Errors) != 0 || result.MessagesConsolidated != 2 {
		t.Fatalf("consolidation result=%+v", result)
	}
	if len(client.calls) < 4 || client.calls[0] != fmt.Sprintf("%d/0", messages[0].ID) || client.calls[1] != fmt.Sprintf("%d/0", messages[1].ID) || client.calls[2] != fmt.Sprintf("%d/1", messages[0].ID) || client.calls[3] != fmt.Sprintf("%d/1", messages[1].ID) {
		t.Fatalf("extraction calls were not one part per message per round: %v", client.calls)
	}
	remaining, err := stm.GetConsolidationCandidates(10, 3)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("consolidation candidates after completion=%d err=%v", len(remaining), err)
	}
}

func TestDurableConsolidationResumesSavedPartAfterCancellationAndRestart(t *testing.T) {
	path := t.TempDir() + "/maintenance.db"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	messages := insertPendingConsolidationSources(t, stm, 1, strings.Repeat("source ", 1200))
	message := messages[0]
	parts := serializeConsolidationSources([]memory.ArchivedMessage{message})
	if len(parts) < 2 {
		t.Fatal("test source did not split into multiple parts")
	}
	cfg := consolidationProgressTestConfig()
	client := &consolidationProgressTestClient{}
	vector := &dedupConsolidationVectorDB{}
	ctx, cancel := context.WithCancel(context.Background())
	client.cancelAfter, client.cancel = 2, cancel
	first := consolidateSTMtoLTMWithContext(ctx, cfg, logger, client, stm, vector, nil)
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) || first.MessagesConsolidated != 0 {
		t.Fatalf("canceled consolidation result=%+v ctx=%v", first, ctx.Err())
	}
	claimed, err := stm.ClaimConsolidationCandidatesContext(context.Background(), 1, 3)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim checkpoint source for inspection: %v, %v", claimed, err)
	}
	key := memory.ConsolidationCheckpointKey{MessageID: claimed[0].ID, SourceHash: memory.ConsolidationSourceHash(claimed[0]), PromptVersion: consolidationProgressPromptVersion, ModelID: "gpt-4o"}
	savedParts, err := stm.ListConsolidationPartResults(context.Background(), key)
	if err != nil || len(savedParts) != 1 {
		t.Fatalf("saved parts=%d err=%v; want one durable part", len(savedParts), err)
	}
	if err := stm.ReleaseConsolidationClaimsContext(context.Background(), []int64{claimed[0].ID}); err != nil {
		t.Fatalf("release inspected source claim: %v", err)
	}
	if err := stm.Close(); err != nil {
		t.Fatalf("close before restart: %v", err)
	}
	stm, err = memory.NewSQLiteMemory(path, logger)
	if err != nil {
		t.Fatalf("reopen memory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	client = &consolidationProgressTestClient{}
	resumed := consolidateSTMtoLTMWithContext(context.Background(), cfg, logger, client, stm, vector, nil)
	if len(resumed.Errors) != 0 || resumed.MessagesConsolidated != 1 {
		t.Fatalf("resumed consolidation result=%+v", resumed)
	}
	if len(client.calls) != len(parts)-1 {
		t.Fatalf("resume calls=%d, want %d remaining parts; calls=%v", len(client.calls), len(parts)-1, client.calls)
	}
	if len(client.calls) > 0 && client.calls[0] == fmt.Sprintf("%d/0", message.ID) {
		t.Fatalf("resume repeated already saved first part: %v", client.calls)
	}
}

func TestDurableConsolidationPublishesReadySmallMessageBeforeContinuingLargeSource(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	const sessionID = "maintenance-fair-publication"
	for _, content := range []string{"small ready source", strings.Repeat("large source ", 1200), "retained newest source"} {
		if _, err := stm.InsertMessage(sessionID, "user", content, false, false); err != nil {
			t.Fatalf("insert mixed-size source: %v", err)
		}
	}
	if err := stm.EnforceSTMPRetentionForSessionContext(context.Background(), sessionID, 1, 100); err != nil {
		t.Fatalf("archive old sources: %v", err)
	}
	messages, err := stm.GetConsolidationCandidates(10, 3)
	if err != nil || len(messages) != 2 {
		t.Fatalf("archived candidates=%d err=%v; want small and large sources", len(messages), err)
	}
	smallParts := serializeConsolidationSources([]memory.ArchivedMessage{messages[0]})
	largeParts := serializeConsolidationSources([]memory.ArchivedMessage{messages[1]})
	if len(smallParts) != 1 || len(largeParts) < 3 {
		t.Fatalf("source part counts small=%d large=%d; want 1 and at least 3", len(smallParts), len(largeParts))
	}
	cfg := consolidationProgressTestConfig()
	cfg.Consolidation.MaxBatchMessages = 2
	ctx, cancel := context.WithCancel(context.Background())
	client := &consolidationProgressTestClient{cancelAfter: 2, cancel: cancel}
	result := consolidateSTMtoLTMWithContext(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), client, stm, &dedupConsolidationVectorDB{}, nil)
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) || result.MessagesConsolidated != 1 || result.FactsStored != 1 {
		t.Fatalf("first interrupted run=%+v ctx=%v; small source should finish before large extraction continues", result, ctx.Err())
	}
	if len(client.calls) < 3 || client.calls[0] != fmt.Sprintf("%d/0", messages[0].ID) || client.calls[1] != fmt.Sprintf("%d/0", messages[1].ID) || client.calls[2] != fmt.Sprintf("%d/1", messages[1].ID) {
		t.Fatalf("fair extraction call order=%v", client.calls)
	}
	remaining, err := stm.GetConsolidationCandidates(10, 3)
	if err != nil || len(remaining) != 1 || remaining[0].ID != messages[1].ID {
		t.Fatalf("interrupted remaining candidates=%+v err=%v; want only the incomplete large source", remaining, err)
	}
}

func TestTerminalConsolidationFailureStaysInLedgerBriefingAndIssueUntilRecovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	memoryPath := filepath.Join(t.TempDir(), "memory.db")
	stm, err := memory.NewSQLiteMemory(memoryPath, logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	if err := stm.InitJournalTables(); err != nil {
		t.Fatalf("InitJournalTables: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	messages := insertPendingConsolidationSources(t, stm, 1, "A terminal consolidation source.")
	if err := stm.Close(); err != nil {
		t.Fatalf("close before setting terminal fixture: %v", err)
	}
	fixtureDB, err := sql.Open("sqlite", memoryPath)
	if err != nil {
		t.Fatalf("open terminal fixture DB: %v", err)
	}
	if _, err := fixtureDB.Exec(`UPDATE archived_messages SET consolidation_status='failed', consolidation_retries=3 WHERE id=?`, messages[0].ID); err != nil {
		t.Fatalf("mark terminal consolidation failure: %v", err)
	}
	if err := fixtureDB.Close(); err != nil {
		t.Fatalf("close terminal fixture DB: %v", err)
	}
	stm, err = memory.NewSQLiteMemory(memoryPath, logger)
	if err != nil {
		t.Fatalf("reopen memory with terminal fixture: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	terminal, err := stm.CountTerminalConsolidationFailures(context.Background(), 3)
	if err != nil || terminal != 1 {
		t.Fatalf("terminal failures=%d err=%v", terminal, err)
	}

	db, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB: %v", err)
	}
	defer db.Close()
	ledger := newMaintenanceRunLedger()
	ledger.beginPhase("consolidation")
	addTerminalConsolidationFailuresToLedger(ledger, terminal)
	ledger.finishPhase("consolidation", false)
	phase := ledger.results().Phases[0]
	if phase.Status != "partial" || !containsMaintenanceCode(phase.ErrorCodes, "terminal_failures_require_recovery") {
		t.Fatalf("terminal failure missing from maintenance ledger: %+v", phase)
	}
	reconcileMaintenancePhaseIssues(db, []memory.MaintenancePhaseResult{phase}, time.Now(), logger)
	var status string
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint='maintenance|phase|consolidation'`).Scan(&status); err != nil || status != "open" {
		t.Fatalf("terminal consolidation operational issue status=%q err=%v", status, err)
	}
	briefing := formatMorningBriefing(&config.Config{}, time.Now(), ledger.status(), ledger.results(), 1)
	if !strings.Contains(briefing, "terminal_failures_require_recovery") || !strings.Contains(briefing, "Open operational issues: 1") {
		t.Fatalf("briefing omitted terminal maintenance state: %s", briefing)
	}

	if recovered, err := stm.RecoverTerminalConsolidationFailures(context.Background(), []int64{messages[0].ID}); err != nil || recovered != 1 {
		t.Fatalf("explicit terminal recovery=%d err=%v", recovered, err)
	}
	terminal, err = stm.CountTerminalConsolidationFailures(context.Background(), 3)
	if err != nil || terminal != 0 {
		t.Fatalf("terminal failures after recovery=%d err=%v", terminal, err)
	}
	ledger = newMaintenanceRunLedger()
	ledger.beginPhase("consolidation")
	ledger.finishPhase("consolidation", false)
	reconcileMaintenancePhaseIssues(db, ledger.results().Phases, time.Now(), logger)
	if err := db.QueryRow(`SELECT status FROM operational_issues WHERE fingerprint='maintenance|phase|consolidation'`).Scan(&status); err != nil || status != "done" {
		t.Fatalf("successful post-recovery phase did not resolve the issue: status=%q err=%v", status, err)
	}
}

func containsMaintenanceCode(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

func consolidationProgressTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.LLM.Model = "gpt-4o"
	cfg.Agent.ContextWindow = 128000
	cfg.Tools.Memory.Enabled = true
	cfg.Consolidation.MaxBatchMessages = 10
	return cfg
}

func insertPendingConsolidationSources(t *testing.T, stm *memory.SQLiteMemory, count int, content string) []memory.ArchivedMessage {
	t.Helper()
	sessionID := "maintenance-progress-session"
	for index := 0; index < count*2; index++ {
		if _, err := stm.InsertMessage(sessionID, "user", content, false, false); err != nil {
			t.Fatalf("insert STM source %d: %v", index, err)
		}
	}
	if err := stm.EnforceSTMPRetentionForSessionContext(context.Background(), sessionID, count, count*2); err != nil {
		t.Fatalf("archive test STM sources: %v", err)
	}
	messages, err := stm.GetConsolidationCandidates(count+1, 3)
	if err != nil {
		t.Fatalf("list archived test sources: %v", err)
	}
	if len(messages) != count {
		t.Fatalf("archived test sources=%d want %d", len(messages), count)
	}
	return messages
}
