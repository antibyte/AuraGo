package commands

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/i18n"
	"aurago/internal/memory"
	"aurago/ui"
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

func TestHelpCommandUsesContextLanguage(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())

	out, handled, err := Handle("/help", Context{Lang: "en"})
	if err != nil {
		t.Fatalf("Handle(/help): %v", err)
	}
	if !handled {
		t.Fatal("/help was not handled")
	}
	if !strings.Contains(out, "Available Commands") {
		t.Fatalf("English help header missing: %s", out)
	}
	if !strings.Contains(out, "Deletes the current chat history") {
		t.Fatalf("English reset help missing: %s", out)
	}
	if strings.Contains(out, "Loescht") || strings.Contains(out, "Löscht") {
		t.Fatalf("help output still contains German reset help: %s", out)
	}
}

func TestHelpCommandDefaultsToGerman(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())

	out, handled, err := Handle("/help", Context{})
	if err != nil {
		t.Fatalf("Handle(/help): %v", err)
	}
	if !handled {
		t.Fatal("/help was not handled")
	}
	if !strings.Contains(out, "Verfügbare Befehle") {
		t.Fatalf("German fallback help header missing: %s", out)
	}
}

// The gate must refuse operator commands before they execute: /restart
// schedules os.Exit, so this test never runs it with AllowOperator set.
func TestOperatorCommandsRequireOperatorContext(t *testing.T) {
	i18n.Load(ui.Content, slog.Default())

	out, isCmd, err := Handle("/restart", Context{Lang: "en"})
	if err != nil || !isCmd {
		t.Fatalf("restart must be intercepted as a command: isCmd=%v err=%v", isCmd, err)
	}
	if !strings.Contains(out, "private") || !strings.Contains(out, "/restart") {
		t.Fatalf("operator command without AllowOperator must be refused with the private-only message, got %q", out)
	}

	operator := []string{"sudopwd", "addssh", "restart", "personality", "debug", "voice"}
	conversational := []string{"help", "reset", "stop", "budget", "credits", "warnings"}
	for _, name := range operator {
		if !operatorCommands[name] {
			t.Fatalf("%s must be classified as an operator command", name)
		}
		out, isCmd, err := Handle("/"+name+" on", Context{Lang: "en"})
		if err != nil || !isCmd || !strings.Contains(out, "private") {
			t.Fatalf("/%s without AllowOperator must be refused: out=%q isCmd=%v err=%v", name, out, isCmd, err)
		}
	}
	for _, name := range conversational {
		if operatorCommands[name] {
			t.Fatalf("%s must stay a conversational command", name)
		}
	}
	// Every registered command needs an explicit decision, so a new command
	// cannot silently land on the conversational side.
	classified := len(operator) + len(conversational)
	if len(registry) != classified {
		t.Fatalf("registry has %d commands, %d are classified; classify the new command in operatorCommands or this test", len(registry), classified)
	}

	out, _, _ = Handle("/help", Context{Lang: "en"})
	if strings.Contains(out, "private") {
		t.Fatalf("help must work without AllowOperator, got %q", out)
	}

	promptsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(promptsDir, "personalities"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "personalities", "operatorcheck.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err = Handle("/personality", Context{Lang: "en", Cfg: &config.Config{}, PromptsDir: promptsDir, AllowOperator: true})
	if err != nil || !strings.Contains(out, "operatorcheck") {
		t.Fatalf("AllowOperator must let /personality run, got %q err=%v", out, err)
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
