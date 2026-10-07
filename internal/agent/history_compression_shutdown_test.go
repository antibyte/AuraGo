package agent

import (
	"aurago/internal/llm"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/planner"

	openai "github.com/sashabaranov/go-openai"
)

// ctxBlockingSummaryClient blocks every summary request until its context ends.
type ctxBlockingSummaryClient struct {
	startOnce  sync.Once
	returnOnce sync.Once
	started    chan struct{}
	returned   chan struct{}
}

func newCtxBlockingSummaryClient() *ctxBlockingSummaryClient {
	return &ctxBlockingSummaryClient{started: make(chan struct{}), returned: make(chan struct{})}
}

func (c *ctxBlockingSummaryClient) CreateChatCompletion(ctx context.Context, _ openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.startOnce.Do(func() { close(c.started) })
	<-ctx.Done()
	c.returnOnce.Do(func() { close(c.returned) })
	return openai.ChatCompletionResponse{}, ctx.Err()
}

func (c *ctxBlockingSummaryClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("streaming is not used by history compression")
}

func seedPersistentCompressionHistory(t *testing.T, sessionID string) (*memory.SQLiteMemory, *memory.HistoryManager) {
	t.Helper()
	dir := t.TempDir()
	stm, err := memory.NewSQLiteMemory(filepath.Join(dir, "memory.db"), testLogger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	history := memory.NewHistoryManager(filepath.Join(dir, "history.json"))
	t.Cleanup(history.Close)
	for turn := 0; turn < 10; turn++ {
		for _, role := range []string{openai.ChatMessageRoleUser, openai.ChatMessageRoleAssistant} {
			content := role + " turn " + strings.Repeat(string(rune('a'+turn)), 120)
			id, err := stm.InsertMessage(sessionID, role, content, false, false)
			if err != nil {
				t.Fatalf("InsertMessage: %v", err)
			}
			if err := history.Add(role, content, id, false, false); err != nil {
				t.Fatalf("HistoryManager.Add: %v", err)
			}
		}
	}
	return stm, history
}

func resetDefaultSideEffectsForTest(t *testing.T) {
	t.Helper()
	defaultSideEffects.mu.Lock()
	previous := defaultSideEffects.group
	defaultSideEffects.group = NewAsyncTaskGroup(context.Background())
	defaultSideEffects.mu.Unlock()
	t.Cleanup(func() {
		defaultSideEffects.mu.Lock()
		defaultSideEffects.group = previous
		defaultSideEffects.mu.Unlock()
	})
}

func TestProactiveHistoryCompressionDrainsWithSideEffectGroup(t *testing.T) {
	previousCoordinator := historyCompressionCoordinator
	historyCompressionCoordinator = &sessionCompressionCoordinator{sessions: make(map[string]*sessionCompressionState)}
	t.Cleanup(func() { historyCompressionCoordinator = previousCoordinator })

	stm, history := seedPersistentCompressionHistory(t, "default")
	before, err := stm.GetSessionMessages("default")
	if err != nil {
		t.Fatalf("GetSessionMessages before: %v", err)
	}
	cfg := &config.Config{}
	cfg.LLM.Model = "test"
	cfg.Agent.MemoryCompressionCharLimit = 100
	client := newCtxBlockingSummaryClient()
	group := NewAsyncTaskGroup(context.Background())
	runCfg := RunConfig{
		Config: cfg, Logger: testLogger, LLMClient: client,
		ShortTermMem: stm, HistoryManager: history, SessionID: "default", SideEffects: group,
	}

	if !ScheduleProactiveHistoryCompression(runCfg) {
		t.Fatal("expected proactive compression to start")
	}
	select {
	case <-client.started:
	case <-time.After(2 * time.Second):
		t.Fatal("proactive compression never requested a summary")
	}
	if err := group.Shutdown(2 * time.Second); err != nil {
		t.Fatalf("side-effect group shutdown: %v", err)
	}
	select {
	case <-client.returned:
	default:
		t.Fatal("proactive compression is still running after the side-effect group drained")
	}
	after, err := stm.GetSessionMessages("default")
	if err != nil {
		t.Fatalf("GetSessionMessages after: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("cancelled compression changed SQLite history: %d -> %d messages", len(before), len(after))
	}
}

func TestPersistentCompressionCancellationRecordsNoFailure(t *testing.T) {
	stm, history := seedPersistentCompressionHistory(t, t.Name())
	plannerDB, err := planner.InitDB(filepath.Join(t.TempDir(), "planner.db"))
	if err != nil {
		t.Fatalf("planner.InitDB: %v", err)
	}
	t.Cleanup(func() { _ = plannerDB.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runCfg := RunConfig{
		Config: &config.Config{}, Logger: testLogger,
		ShortTermMem: stm, HistoryManager: history, SessionID: t.Name(), PlannerDB: plannerDB,
	}
	client := &mockChatClient{err: context.Canceled}

	for attempt := 0; attempt < 2; attempt++ {
		if result := compressPersistentHistory(ctx, runCfg, 0, 1, true, "test", client, testLogger); result.Compressed {
			t.Fatal("cancelled compression must not compress history")
		}
	}
	historyCompressionFailures.Lock()
	failures := historyCompressionFailures.counts[t.Name()]
	historyCompressionFailures.Unlock()
	if failures != 0 {
		t.Fatalf("cancelled compression recorded %d failures, want 0", failures)
	}
}

func TestShutdownSideEffectsDrainsAndRejectsLaterTasks(t *testing.T) {
	resetDefaultSideEffectsForTest(t)
	started := make(chan struct{})
	finished := make(chan struct{})
	if !sideEffectsFromRunConfig(RunConfig{}).Go(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(finished)
	}) {
		t.Fatal("default side-effect group rejected a task before shutdown")
	}
	<-started

	if err := ShutdownSideEffects(time.Second); err != nil {
		t.Fatalf("ShutdownSideEffects: %v", err)
	}
	select {
	case <-finished:
	default:
		t.Fatal("ShutdownSideEffects returned before the task finished")
	}
	if sideEffectsFromRunConfig(RunConfig{}).Go(func(context.Context) {}) {
		t.Fatal("a side effect started after process shutdown")
	}
}
