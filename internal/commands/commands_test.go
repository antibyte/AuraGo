package commands

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	stm := newCommandTestMemory(t)
	active, releaseActive, err := stm.BeginSessionTurn(context.Background(), "stop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseActive()
	queued, releaseQueued, err := stm.RegisterSessionRun(context.Background(), "stop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseQueued()
	other, releaseOther, err := stm.RegisterSessionRun(context.Background(), "other")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOther()
	waited := make(chan error, 1)
	go func() {
		release, err := stm.LockSessionTurn(queued, "stop-test")
		if release != nil {
			release()
		}
		waited <- err
	}()
	if _, err := (&StopCommand{}).Execute(nil, Context{STM: stm, SessionID: "stop-test", Lang: "en"}); err != nil {
		t.Fatal(err)
	}
	for _, run := range []context.Context{active, queued} {
		select {
		case <-run.Done():
		case <-time.After(time.Second):
			t.Fatal("stop did not cancel the session run")
		}
	}
	select {
	case err := <-waited:
		if err != context.Canceled {
			t.Fatalf("queued turn error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued turn did not leave its gate")
	}
	if other.Err() != nil {
		t.Fatal("stop cancelled a different session")
	}
	releaseActive()
	fresh, releaseFresh, err := stm.BeginSessionTurn(context.Background(), "stop-test")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFresh()
	if fresh.Err() != nil {
		t.Fatal("stop fenced a later human turn")
	}
}

func TestIsStopMatchesCommandParsing(t *testing.T) {
	for input, want := range map[string]bool{"/stop": true, "/stop now": true, "/stop\nnow": true, " /stop": false, "/stopping": false, "quoted /stop": false, "": false} {
		if got := IsStop(input); got != want {
			t.Errorf("IsStop(%q) = %v, want %v", input, got, want)
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
	listed := map[string]int{}
	for _, name := range append(append([]string{}, operator...), conversational...) {
		listed[name]++
	}
	for name := range registry {
		if listed[name] != 1 {
			t.Fatalf("/%s must appear in exactly one of the operator or conversational lists (found %d); classify it in operatorCommands and this test", name, listed[name])
		}
	}
	if len(registry) != len(listed) {
		t.Fatalf("registry has %d commands, the lists name %d", len(registry), len(listed))
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
