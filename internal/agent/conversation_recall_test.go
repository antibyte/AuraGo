package agent

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"
	"aurago/internal/security"
)

func TestConversationRecallIsBoundedScrubbedAndSessionBound(t *testing.T) {
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	secret := "conversation-recall-secret-7b2d"
	security.RegisterSensitive(secret)
	var firstID int64
	for i := 0; i < 5; i++ {
		id, err := stm.InsertActivityTurn(memory.ActivityTurn{
			Date: "2026-09-03", SessionID: "session-a", Intent: "cache repair task",
			Outcomes: []string{fmt.Sprintf("result %d %s", i, secret)},
		})
		if err != nil {
			t.Fatal(err)
		}
		if firstID == 0 {
			firstID = id
		}
	}
	if _, err := stm.InsertActivityTurn(memory.ActivityTurn{Date: "2026-09-03", SessionID: "session-b", Intent: "cache repair task", Outcomes: []string{"foreign"}}); err != nil {
		t.Fatal(err)
	}
	message, tokens := buildConversationRecallMessage(stm, "session-a", "cache repair task", "agnes-2.5-flash")
	if tokens <= 0 || tokens > conversationRecallMaxTokens {
		t.Fatalf("recall tokens=%d", tokens)
	}
	if strings.Count(message.Content, "[conversation:") > conversationRecallMaxResults || strings.Contains(message.Content, secret) || strings.Contains(message.Content, "foreign") {
		t.Fatalf("recall was not bounded, scrubbed, or session scoped: %q", message.Content)
	}
	queryResult, err := executeQueryMemory(ToolCall{Content: "cache repair task", Sources: []string{"conversation"}}, "session-a", stm, nil, nil, nil, nil)
	if err != nil || strings.Contains(queryResult, secret) || strings.Contains(queryResult, "session-b") {
		t.Fatalf("conversation query leaked data: %q err=%v", queryResult, err)
	}
	conversationID := "conversation:" + registerConversationReference("session-a", fmt.Sprintf("activity:%d", firstID))
	result, err := executeRecallMemory(ToolCall{ID: conversationID}, "session-b", stm, nil)
	if err != nil || !strings.Contains(result, fmt.Sprintf(`"missing":[%q]`, conversationID)) {
		t.Fatalf("cross-session recall result=%q err=%v", result, err)
	}
	result, err = executeRecallMemory(ToolCall{ID: conversationID}, "session-a", stm, nil)
	if err != nil || !strings.Contains(result, "cache repair task") {
		t.Fatalf("same-session recall result=%q err=%v", result, err)
	}
}

func TestConversationReferenceExpires(t *testing.T) {
	token := registerConversationReference("session-a", "message:1")
	conversationReferences.Lock()
	entry := conversationReferences.entries[token]
	entry.expiresAt = time.Now().Add(-time.Second)
	conversationReferences.entries[token] = entry
	conversationReferences.Unlock()
	if _, ok := resolveConversationReference("session-a", token); ok {
		t.Fatal("expired conversation reference resolved")
	}
}
