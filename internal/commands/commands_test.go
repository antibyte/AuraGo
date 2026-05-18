package commands

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"aurago/internal/memory"
)

func TestResetCommandDefaultsToDefaultSession(t *testing.T) {
	t.Parallel()

	stm := newCommandTestMemory(t)
	if _, err := stm.InsertMessage("default", "user", "remove me", false, false); err != nil {
		t.Fatalf("InsertMessage default: %v", err)
	}
	if _, err := stm.InsertMessage("virtual-desktop", "user", "keep me", false, false); err != nil {
		t.Fatalf("InsertMessage desktop: %v", err)
	}

	if _, err := (&ResetCommand{}).Execute(nil, Context{STM: stm, Lang: "en"}); err != nil {
		t.Fatalf("ResetCommand.Execute: %v", err)
	}

	defaultMsgs, err := stm.GetSessionMessages("default")
	if err != nil {
		t.Fatalf("GetSessionMessages(default): %v", err)
	}
	if len(defaultMsgs) != 0 {
		t.Fatalf("default session has %d messages after reset, want 0", len(defaultMsgs))
	}
	desktopMsgs, err := stm.GetSessionMessages("virtual-desktop")
	if err != nil {
		t.Fatalf("GetSessionMessages(virtual-desktop): %v", err)
	}
	if len(desktopMsgs) != 1 {
		t.Fatalf("desktop session has %d messages after default reset, want 1", len(desktopMsgs))
	}
}

func TestResetCommandUsesRequestedSession(t *testing.T) {
	t.Parallel()

	stm := newCommandTestMemory(t)
	if _, err := stm.InsertMessage("default", "user", "keep me", false, false); err != nil {
		t.Fatalf("InsertMessage default: %v", err)
	}
	if _, err := stm.InsertMessage("virtual-desktop", "user", "remove me", false, false); err != nil {
		t.Fatalf("InsertMessage desktop: %v", err)
	}

	if _, err := (&ResetCommand{}).Execute(nil, Context{STM: stm, SessionID: "virtual-desktop", Lang: "en"}); err != nil {
		t.Fatalf("ResetCommand.Execute: %v", err)
	}

	defaultMsgs, err := stm.GetSessionMessages("default")
	if err != nil {
		t.Fatalf("GetSessionMessages(default): %v", err)
	}
	if len(defaultMsgs) != 1 {
		t.Fatalf("default session has %d messages after desktop reset, want 1", len(defaultMsgs))
	}
	desktopMsgs, err := stm.GetSessionMessages("virtual-desktop")
	if err != nil {
		t.Fatalf("GetSessionMessages(virtual-desktop): %v", err)
	}
	if len(desktopMsgs) != 0 {
		t.Fatalf("desktop session has %d messages after desktop reset, want 0", len(desktopMsgs))
	}
}

func TestStopCommandInterruptsRequestedSession(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("commands.go")
	if err != nil {
		t.Fatalf("read commands source: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"SessionID        string",
		"commandSessionID(ctx)",
		"agent.InterruptSession(commandSessionID(ctx))",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("commands source missing marker %q", marker)
		}
	}
}

func newCommandTestMemory(t *testing.T) *memory.SQLiteMemory {
	t.Helper()
	stm, err := memory.NewSQLiteMemory(":memory:", slog.Default())
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	return stm
}
