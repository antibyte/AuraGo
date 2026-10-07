package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestMCPBridgeCarriesScopeIntoAgentLoopAndSeparatesHistory(t *testing.T) {
	run, client, cleanup := newPromptPipelineTestRunConfig(t, "mcp-client-a", "")
	defer cleanup()
	run.Config.Agent.AllowShell = true
	run.Config.Agent.AllowUnsafeHostExecution = true
	run.Config.Tools.Memory.Enabled = true
	if _, err := run.ShortTermMem.InsertMessage("mcp-client-b", openai.ChatMessageRoleUser, "private-client-b-message", false, false); err != nil {
		t.Fatal(err)
	}
	run.AllowedTools = bridgeAllowedTools(&DispatchContext{ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet([]string{"ask_aurago", "query_memory"})})
	if _, err := AskAuraGoBridge(context.Background(), run, "inspect memory"); err != nil {
		t.Fatal(err)
	}
	for _, schema := range client.lastReq.Tools {
		if schema.Function != nil && schema.Function.Name != "query_memory" {
			t.Fatalf("bridge offered ungranted tool %s", schema.Function.Name)
		}
	}
	for _, msg := range client.lastReq.Messages {
		if strings.Contains(msg.Content, "private-client-b-message") {
			t.Fatal("another client leaked into bridge history")
		}
	}
	dc := &DispatchContext{Cfg: run.Config, Logger: run.Logger, SessionID: run.SessionID, ToolScopeRestricted: true, AllowedTools: normalizedAllowedToolSet(run.AllowedTools)}
	got := DispatchToolCallResult(context.Background(), &ToolCall{Action: "execute_shell", Command: "must-not-execute"}, dc, "")
	if !got.IsError || !strings.Contains(got.Output, "tool_scope_denied") {
		t.Fatalf("model bypassed inherited scope: %+v", got)
	}
	run.AllowedTools = nil
	if _, err := AskAuraGoBridge(context.Background(), run, "unscoped"); err == nil {
		t.Fatal("unscoped bridge run accepted")
	}
}
