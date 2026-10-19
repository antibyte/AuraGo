package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/llm"
	"aurago/internal/memory"
	"aurago/internal/tools"

	"github.com/sashabaranov/go-openai"
)

type bridgeChildApprovalClient struct{ calls int }

func (c *bridgeChildApprovalClient) CreateChatCompletion(context.Context, openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	c.calls++
	message := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: "Waiting for the child action approval."}
	finish := openai.FinishReasonStop
	if c.calls == 1 {
		message.Content = ""
		message.ToolCalls = []openai.ToolCall{{ID: "parent-python", Type: openai.ToolTypeFunction, Function: openai.FunctionCall{Name: "execute_python", Arguments: `{"code":"print('fixture')","enable_tool_bridge":true}`}}}
		finish = openai.FinishReasonToolCalls
	}
	return openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{Message: message, FinishReason: finish}}}, nil
}
func (*bridgeChildApprovalClient) CreateChatCompletionStream(context.Context, openai.ChatCompletionRequest) (llm.CompletionStream, error) {
	return nil, errors.New("streaming is not used by this fixture")
}

func TestToolBridgeChildApprovalDoesNotBorrowRunningParentIdentity(t *testing.T) {
	executed := 0
	s, dc := bridgeSecurityFixture(t, func(_ context.Context, call agent.ToolCall) (string, bool) {
		if call.Action != "docker" {
			return "", false
		}
		executed++
		return `{"status":"success"}`, true
	})
	s.Cfg.Agent.AllowPython = true
	s.Cfg.Sandbox.Enabled = true
	s.Cfg.LLM.Model = "gpt-4o-mini"
	s.Cfg.Agent.ContextWindow = 32768
	s.Cfg.Directories.SkillsDir = t.TempDir()
	s.Cfg.Directories.ToolsDir = t.TempDir()
	stm, err := memory.NewSQLiteMemory(":memory:", s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	history := memory.NewEphemeralHistoryManager()
	defer history.Close()
	profile, err := agent.NewPreparedPromptProfile("bridge-child-fixture", "Use the supplied tool once, then report its result.", agent.ConfiguredToolSchemas(s.Cfg, s.Logger))
	if err != nil {
		t.Fatal(err)
	}
	parentID := ""
	issued := false
	agent.SetToolBridgeGrantIssuer(func(ctx context.Context, parent *agent.DispatchContext) (string, string, func(), error) {
		issued = true
		page, err := stm.SearchAuditEvents(memory.AuditFilter{Source: memory.AuditSourceAgentTool, Type: "agent_action", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range page.Entries {
			if entry.TargetName != "execute_python" {
				continue
			}
			var metadata struct {
				ActionID string `json:"action_id"`
				State    string `json:"action_state"`
			}
			if err := json.Unmarshal([]byte(entry.MetadataJSON), &metadata); err != nil {
				t.Fatal(err)
			}
			if metadata.State != "started" {
				t.Fatalf("parent not started at broker boundary: %s", metadata.State)
			}
			parentID = metadata.ActionID
		}
		if parentID == "" {
			t.Fatal("missing real mainloop parent identity")
		}
		_, token, release, err := s.issueToolBridgeGrant(ctx, parent)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		rr := httptest.NewRecorder()
		authMiddleware(s, handleToolBridgeExecute(s)).ServeHTTP(rr, bridgeSecurityRequest(ctx, token, "docker", `{"parameters":{"operation":"remove","container_id":"fixture-container"}}`))
		pending := agent.PendingActionApprovals(parent.SessionID)
		if rr.Code != http.StatusOK || len(pending) != 1 || pending[0].Action.Tool != "docker" || pending[0].ID == parentID || executed != 0 {
			t.Fatalf("child did not obtain independent approval: status=%d pending=%+v calls=%d body=%s", rr.Code, pending, executed, rr.Body.String())
		}
		// No worker or Docker process is needed: the actual Python dispatch has
		// reached its broker boundary, which is the identity regression under test.
		return "", "", nil, errors.New("fixture stops before isolated worker startup")
	})
	t.Cleanup(func() { agent.SetToolBridgeGrantIssuer(nil) })
	run := agent.RunConfig{Config: s.Cfg, Logger: s.Logger, LLMClient: &bridgeChildApprovalClient{}, ShortTermMem: stm, HistoryManager: history,
		Registry:  tools.NewProcessRegistry(s.Logger),
		SessionID: dc.SessionID, MessageSource: "web_chat", ExecutionHooks: dc.ExecutionHooks, PreparedPrompt: profile,
		SuppressTurnSideEffects: true, Checkpoint: func([]openai.ChatCompletionMessage) error { return nil }}
	_, err = agent.ExecuteAgentLoop(context.Background(), openai.ChatCompletionRequest{Model: s.Cfg.LLM.Model, Messages: []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleUser, Content: "Run the fixture once."}}}, run, false, agent.NoopBroker{})
	if err != nil {
		t.Fatal(err)
	}
	if !issued {
		t.Fatal("mainloop never reached the broker")
	}
	// A child must not append needs_human_approval to the parent's history.
	page, err := stm.SearchAuditEvents(memory.AuditFilter{Source: memory.AuditSourceAgentTool, Type: "agent_action", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range page.Entries {
		var metadata struct {
			ActionID string   `json:"action_id"`
			History  []string `json:"state_history"`
		}
		if err := json.Unmarshal([]byte(entry.MetadataJSON), &metadata); err != nil {
			t.Fatal(err)
		}
		if metadata.ActionID == parentID {
			for _, state := range metadata.History {
				if state == "needs_human_approval" {
					t.Fatal("child rewrote parent lifecycle")
				}
			}
		}
	}
}
