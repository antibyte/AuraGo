package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/security"
)

type integrationGuardScanner struct{ seen []string }

func (s *integrationGuardScanner) Evaluate(_ context.Context, text string) (float64, string, bool, error) {
	s.seen = append(s.seen, text)
	return .95, "quarantine", true, nil
}

func TestInjectionGuardPreservesMutationOutcomeAndOriginalUnicode(t *testing.T) {
	scanner := &integrationGuardScanner{}
	defer security.RegisterExternalContentScanner(scanner)()
	var status ToolResultStatus
	ctx := context.WithValue(context.Background(), toolOutcomeKey{}, &status)
	ctx = context.WithValue(ctx, externalContentSourceKey{}, security.ExternalWeb)
	output := formatBrowserAutomationOutput(ctx, `{"status":"success","body":"Original \u200b Unicode"}`)
	if status != ToolResultSuccess || !strings.Contains(output, "QUARANTINE") || strings.Contains(output, "Original") {
		t.Fatal("successful mutation became retryable or leaked content", output)
	}
	if len(scanner.seen) != 1 || !strings.Contains(scanner.seen[0], "\u200b") {
		t.Fatal("classifier saw normalized text")
	}
}

func TestInjectionGuardArchivedWebAndEmailRetainSource(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	scanner := &integrationGuardScanner{}
	defer security.RegisterExternalContentScanner(scanner)()
	for _, tool := range []string{"web_scraper", "fetch_email"} {
		id := t.Name() + tool
		defer clearIsolatedModelBodies(id)
		if err := stm.StoreCompressedOutput(context.Background(), &memory.CompressedToolOutput{SessionID: id, ToolCallID: "call", OutputRef: "ref", ToolName: tool, OriginalContent: strings.Repeat("ordinary text ", 30000), CompressedContent: "compact", FilterUsed: "vault"}); err != nil {
			t.Fatal(err)
		}
		dc := &DispatchContext{Cfg: &config.Config{}, Logger: logger, ShortTermMem: stm, SessionID: id}
		before := len(scanner.seen)
		out := handleReadToolOutput(context.Background(), ToolCall{Params: map[string]interface{}{"ref": "ref", "max_chars": 300}}, dc)
		if !strings.Contains(out, "QUARANTINE") || len(scanner.seen) != before+1 || len(scanner.seen[before]) > 1000 {
			t.Fatal("archive page was not scanned within output budget", out)
		}
		out = handleRetrieveOriginalOutput(context.Background(), ToolCall{Params: map[string]interface{}{"tool_call_id": "call"}}, dc)
		if !strings.Contains(out, "QUARANTINE") || strings.Contains(out, "ordinary text") {
			t.Fatal("legacy archive path bypassed classifier")
		}
	}
}
