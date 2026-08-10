package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"

	openai "github.com/sashabaranov/go-openai"
)

func TestSessionCompressionCoordinatorSerializesAndAppliesCooldown(t *testing.T) {
	coordinator := &sessionCompressionCoordinator{sessions: make(map[string]*sessionCompressionState)}
	first, ok := coordinator.acquire(context.Background(), "session-1", false, false)
	if !ok {
		t.Fatal("first compression lease was not acquired")
	}
	if _, ok := coordinator.acquire(context.Background(), "session-1", false, false); ok {
		t.Fatal("parallel compression lease was acquired for the same session")
	}
	first.release(true)
	if _, ok := coordinator.acquire(context.Background(), "session-1", false, true); ok {
		t.Fatal("proactive compression ignored per-session cooldown")
	}
	necessary, ok := coordinator.acquire(context.Background(), "session-1", false, false)
	if !ok {
		t.Fatal("required compression should ignore proactive cooldown")
	}
	necessary.release(false)
}

func TestSessionCompressionCoordinatorBoundsInactiveSessions(t *testing.T) {
	coordinator := &sessionCompressionCoordinator{sessions: make(map[string]*sessionCompressionState)}
	for i := 0; i < sessionCompressionMaxEntries+200; i++ {
		lease, ok := coordinator.acquire(context.Background(), fmt.Sprintf("session-%d", i), false, false)
		if !ok {
			t.Fatalf("lease %d was not acquired", i)
		}
		lease.release(false)
	}
	coordinator.mu.Lock()
	count := len(coordinator.sessions)
	coordinator.mu.Unlock()
	if count > sessionCompressionMaxEntries {
		t.Fatalf("compression coordinator sessions = %d, limit = %d", count, sessionCompressionMaxEntries)
	}
}

func TestSessionCompressionCoordinatorEvictsIdleButProtectsActiveSession(t *testing.T) {
	coordinator := &sessionCompressionCoordinator{sessions: make(map[string]*sessionCompressionState)}
	active, ok := coordinator.acquire(context.Background(), "active", false, false)
	if !ok {
		t.Fatal("active lease was not acquired")
	}
	coordinator.mu.Lock()
	coordinator.sessions["active"].lastUsed = time.Now().Add(-2 * sessionCompressionIdleTTL)
	coordinator.sessions["idle"] = &sessionCompressionState{
		gate: make(chan struct{}, 1), lastUsed: time.Now().Add(-2 * sessionCompressionIdleTTL),
	}
	coordinator.cleanupLocked(time.Now(), "")
	_, activePresent := coordinator.sessions["active"]
	_, idlePresent := coordinator.sessions["idle"]
	coordinator.mu.Unlock()
	if !activePresent {
		t.Fatal("active compression state was evicted")
	}
	if idlePresent {
		t.Fatal("idle compression state was not evicted")
	}
	active.release(false)
}

func TestPersistentCompressionUsesStableIDsAndReusesSummaryInRequest(t *testing.T) {
	dir := t.TempDir()
	stm, err := memory.NewSQLiteMemory(filepath.Join(dir, "memory.db"), testLogger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	history := memory.NewHistoryManager(filepath.Join(dir, "history.json"))
	t.Cleanup(history.Close)

	requestMessages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: "generated system prompt"}}
	for turn := 0; turn < 10; turn++ {
		for _, role := range []string{openai.ChatMessageRoleUser, openai.ChatMessageRoleAssistant} {
			content := role + " turn " + strings.Repeat(string(rune('a'+turn)), 120)
			pinned := turn == 0
			id, insertErr := stm.InsertMessage("default", role, content, pinned, false)
			if insertErr != nil {
				t.Fatalf("InsertMessage: %v", insertErr)
			}
			if addErr := history.Add(role, content, id, pinned, false); addErr != nil {
				t.Fatalf("HistoryManager.Add: %v", addErr)
			}
			requestMessages = append(requestMessages, openai.ChatCompletionMessage{Role: role, Content: content})
		}
	}

	client := &mockChatClient{response: "stable persistent summary"}
	runCfg := RunConfig{
		Config: &config.Config{}, Logger: testLogger, LLMClient: client,
		ShortTermMem: stm, HistoryManager: history, SessionID: "default",
	}
	result := compressPersistentHistory(context.Background(), runCfg, 0, 1, true, "test", client, testLogger)
	if !result.Compressed || len(result.Dropped) == 0 {
		t.Fatalf("persistent result = %+v, want compression", result)
	}
	if history.GetSummary() != "stable persistent summary" {
		t.Fatalf("persistent summary = %q", history.GetSummary())
	}
	for _, message := range result.Dropped {
		if message.ID <= 0 {
			t.Fatalf("compressed message lacks stable ID: %+v", message)
		}
		if message.Pinned {
			t.Fatalf("pinned message was compressed: %+v", message)
		}
	}
	remainingSQLite, err := stm.GetSessionMessages("default")
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	droppedIDs := make(map[int64]struct{}, len(result.Dropped))
	for _, message := range result.Dropped {
		droppedIDs[message.ID] = struct{}{}
	}
	for _, message := range remainingSQLite {
		if _, dropped := droppedIDs[message.ID]; dropped {
			t.Fatalf("compressed ID %d remained in SQLite", message.ID)
		}
	}

	updatedRequest := applyPersistentCompressionToRequest(requestMessages, result)
	recapCount := 0
	for _, message := range updatedRequest {
		if strings.HasPrefix(strings.TrimSpace(message.Content), "[CONTEXT_RECAP]:") {
			recapCount++
			if !strings.Contains(message.Content, "stable persistent summary") {
				t.Fatalf("request recap does not use persisted summary: %q", message.Content)
			}
		}
		for _, dropped := range result.Dropped {
			if message.Role == dropped.Role && message.Content == dropped.Content {
				t.Fatalf("compressed message remained in request: %q", message.Content)
			}
		}
	}
	if recapCount != 1 {
		t.Fatalf("request recap count = %d, want 1", recapCount)
	}
}

func TestPersistentCompressionDoesNotPublishSummaryWhenSQLiteUpdateFails(t *testing.T) {
	dir := t.TempDir()
	stm, err := memory.NewSQLiteMemory(filepath.Join(dir, "memory.db"), testLogger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	history := memory.NewHistoryManager(filepath.Join(dir, "history.json"))
	t.Cleanup(history.Close)
	for turn := 0; turn < 8; turn++ {
		for _, role := range []string{openai.ChatMessageRoleUser, openai.ChatMessageRoleAssistant} {
			content := role + " turn " + strings.Repeat("x", 100)
			id, insertErr := stm.InsertMessage("default", role, content, false, false)
			if insertErr != nil {
				t.Fatalf("InsertMessage: %v", insertErr)
			}
			if addErr := history.Add(role, content, id, false, false); addErr != nil {
				t.Fatalf("HistoryManager.Add: %v", addErr)
			}
		}
	}
	before := len(history.GetAll())
	if err := stm.Close(); err != nil {
		t.Fatalf("Close SQLite: %v", err)
	}
	client := &mockChatClient{response: "must not be published"}
	result := compressPersistentHistory(context.Background(), RunConfig{
		Config: &config.Config{}, Logger: testLogger, LLMClient: client,
		ShortTermMem: stm, HistoryManager: history, SessionID: "default",
	}, 0, 1, true, "test", client, testLogger)
	if result.Compressed {
		t.Fatal("compression was published despite SQLite failure")
	}
	if history.GetSummary() != "" || len(history.GetAll()) != before {
		t.Fatalf("history changed after SQLite failure: summary=%q messages=%d/%d", history.GetSummary(), len(history.GetAll()), before)
	}
}

func TestPersistentCompressionBoundsHelperInputAtAtomicGroups(t *testing.T) {
	dir := t.TempDir()
	stm, err := memory.NewSQLiteMemory(filepath.Join(dir, "memory.db"), testLogger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	history := memory.NewHistoryManager(filepath.Join(dir, "history.json"))
	t.Cleanup(history.Close)

	for turn := 0; turn < 180; turn++ {
		for _, role := range []string{openai.ChatMessageRoleUser, openai.ChatMessageRoleAssistant} {
			content := fmt.Sprintf("%s turn %d %s", role, turn, strings.Repeat("x", 80))
			id, insertErr := stm.InsertMessage("default", role, content, false, false)
			if insertErr != nil {
				t.Fatalf("InsertMessage: %v", insertErr)
			}
			if addErr := history.Add(role, content, id, false, false); addErr != nil {
				t.Fatalf("HistoryManager.Add: %v", addErr)
			}
		}
	}

	client := &mockChatClient{response: "bounded summary"}
	result := compressPersistentHistory(context.Background(), RunConfig{
		Config: &config.Config{}, Logger: testLogger, LLMClient: client,
		ShortTermMem: stm, HistoryManager: history, SessionID: "default",
	}, 0, 1, true, "test", client, testLogger)
	if !result.Compressed {
		t.Fatal("expected bounded persistent compression")
	}
	if len(result.Dropped) > persistentCompressionMaxMessages {
		t.Fatalf("dropped messages = %d, limit = %d", len(result.Dropped), persistentCompressionMaxMessages)
	}
	if len(result.Dropped)%2 != 0 {
		t.Fatalf("dropped messages = %d, want complete user/assistant groups", len(result.Dropped))
	}
	if len(client.lastReq.Messages) != 1 || len(client.lastReq.Messages[0].Content) > persistentCompressionMaxTranscriptChars+4096 {
		t.Fatalf("summary helper prompt unexpectedly large: %d chars", len(client.lastReq.Messages[0].Content))
	}
}
