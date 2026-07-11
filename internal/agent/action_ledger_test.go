package agent

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"aurago/internal/memory"
)

type actionLedgerCaptureBroker struct {
	jsonMessages []string
}

func (b *actionLedgerCaptureBroker) Send(event, message string) {}
func (b *actionLedgerCaptureBroker) SendJSON(jsonStr string) {
	b.jsonMessages = append(b.jsonMessages, jsonStr)
}
func (b *actionLedgerCaptureBroker) SendLLMStreamDelta(content, toolName, toolID string, index int, finishReason string) {
}
func (b *actionLedgerCaptureBroker) SendLLMStreamDone(finishReason string) {}
func (b *actionLedgerCaptureBroker) SendTokenUpdate(prompt, completion, total, sessionTotal, globalTotal int, isEstimated, isFinal bool, source string) {
}
func (b *actionLedgerCaptureBroker) SendThinkingBlock(provider, content, state string) {}

type typedActionLedgerCaptureBroker struct {
	actionLedgerCaptureBroker
	typedEvents []typedActionLedgerEvent
}

type typedActionLedgerEvent struct {
	eventType string
	payload   interface{}
}

func (b *typedActionLedgerCaptureBroker) SendTyped(eventType string, payload interface{}) bool {
	b.typedEvents = append(b.typedEvents, typedActionLedgerEvent{eventType: eventType, payload: payload})
	return true
}

func TestAgentActionLedgerRecordsLifecycleInAudit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	broker := &actionLedgerCaptureBroker{}
	ledger := newAgentActionLedger(stm, logger, broker, "sess-1", "web_chat")

	action, err := ledger.ProposeTool("turn-1", ToolCall{
		Action:    "execute_shell",
		Operation: "run",
		Command:   "echo redacted-probe",
	})
	if err != nil {
		t.Fatalf("ProposeTool: %v", err)
	}
	if action.State != string(AgentActionStateProposed) {
		t.Fatalf("state = %q, want proposed", action.State)
	}
	if action.ArgsHash == "" {
		t.Fatal("expected args hash")
	}

	if action, err = ledger.Accept(action, "validated"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if action, err = ledger.Start(action); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if action, err = ledger.CompleteTool(action, toolExecutionResult{
		Content: "hello",
		Failed:  false,
		Outcome: ExecutionOutcomeSuccess,
	}, 37); err != nil {
		t.Fatalf("CompleteTool: %v", err)
	}
	if action.State != string(AgentActionStateSucceeded) {
		t.Fatalf("final state = %q, want succeeded", action.State)
	}
	if action.Operation != "run" {
		t.Fatalf("operation = %q, want run", action.Operation)
	}
	if action.Outcome != ExecutionOutcomeSuccess.String() {
		t.Fatalf("outcome = %q, want success", action.Outcome)
	}
	if action.DurationMS != 37 {
		t.Fatalf("duration_ms = %d, want 37", action.DurationMS)
	}
	if action.MessageSource != "web_chat" {
		t.Fatalf("message_source = %q, want web_chat", action.MessageSource)
	}
	if strings.Join(action.StateHistory, ",") != "proposed,accepted,started,succeeded" {
		t.Fatalf("public state history = %#v", action.StateHistory)
	}

	page, err := stm.SearchAuditEvents(memory.AuditFilter{
		Source: memory.AuditSourceAgentTool,
		Type:   agentActionAuditEventType,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("SearchAuditEvents: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("audit total = %d, want 1", page.Total)
	}
	entry := page.Entries[0]
	if entry.CorrelationID != action.CorrelationID {
		t.Fatalf("correlation id = %q, want %q", entry.CorrelationID, action.CorrelationID)
	}
	if entry.Status != memory.AuditStatusSuccess {
		t.Fatalf("status = %q, want success", entry.Status)
	}
	if strings.Contains(entry.MetadataJSON, "redacted-probe") {
		t.Fatalf("metadata leaked tool arguments: %s", entry.MetadataJSON)
	}

	var metadata agentActionAuditMetadata
	if err := json.Unmarshal([]byte(entry.MetadataJSON), &metadata); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if metadata.ActionState != string(AgentActionStateSucceeded) {
		t.Fatalf("metadata action state = %q, want succeeded", metadata.ActionState)
	}
	if metadata.Outcome != ExecutionOutcomeSuccess.String() {
		t.Fatalf("metadata outcome = %q, want success", metadata.Outcome)
	}
	if metadata.DurationMS != 37 {
		t.Fatalf("metadata duration_ms = %d, want 37", metadata.DurationMS)
	}
	wantHistory := []string{"proposed", "accepted", "started", "succeeded"}
	if strings.Join(metadata.StateHistory, ",") != strings.Join(wantHistory, ",") {
		t.Fatalf("state history = %#v, want %#v", metadata.StateHistory, wantHistory)
	}

	if len(broker.jsonMessages) != 4 {
		t.Fatalf("typed events = %d, want 4", len(broker.jsonMessages))
	}
	var wire struct {
		Type    string           `json:"type"`
		Payload AgentActionEvent `json:"payload"`
	}
	if err := json.Unmarshal([]byte(broker.jsonMessages[len(broker.jsonMessages)-1]), &wire); err != nil {
		t.Fatalf("unmarshal typed event: %v", err)
	}
	if wire.Type != agentActionSSEType {
		t.Fatalf("typed event = %q, want %q", wire.Type, agentActionSSEType)
	}
	if wire.Payload.State != string(AgentActionStateSucceeded) {
		t.Fatalf("payload state = %q, want succeeded", wire.Payload.State)
	}
	if strings.Contains(broker.jsonMessages[len(broker.jsonMessages)-1], "redacted-probe") {
		t.Fatalf("typed event leaked tool arguments: %s", broker.jsonMessages[len(broker.jsonMessages)-1])
	}
}

func TestAgentActionSubjectOnlyLabelsSkills(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ledger := newAgentActionLedger(nil, logger, NoopBroker{}, "sess-1", "web_chat")

	skillAction, err := ledger.ProposeTool("turn-skill", ToolCall{
		Action: "activate_agent_skill",
		Skill:  "review-helper",
	})
	if err != nil {
		t.Fatalf("ProposeTool skill: %v", err)
	}
	if skillAction.Subject != "review-helper" {
		t.Fatalf("skill subject = %q, want review-helper", skillAction.Subject)
	}

	fileAction, err := ledger.ProposeTool("turn-file", ToolCall{
		Action: "remote_control_files",
		Path:   `C:\Users\Andi\Documents\private.txt`,
		Params: map[string]interface{}{
			"path": `C:\Users\Andi\Documents\private.txt`,
		},
	})
	if err != nil {
		t.Fatalf("ProposeTool file: %v", err)
	}
	if fileAction.Subject != "" {
		t.Fatalf("non-skill subject = %q, want empty", fileAction.Subject)
	}
}

func TestAgentActionLedgerUsesTypedBrokerWhenAvailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	broker := &typedActionLedgerCaptureBroker{}
	ledger := newAgentActionLedger(stm, logger, broker, "sess-typed", "web_chat")
	if _, err := ledger.ProposeTool("turn-1", ToolCall{Action: "execute_shell"}); err != nil {
		t.Fatalf("ProposeTool: %v", err)
	}
	if len(broker.typedEvents) != 1 {
		t.Fatalf("typed events = %d, want 1", len(broker.typedEvents))
	}
	if len(broker.jsonMessages) != 0 {
		t.Fatalf("raw json fallback events = %d, want 0", len(broker.jsonMessages))
	}
	if broker.typedEvents[0].eventType != agentActionSSEType {
		t.Fatalf("typed event = %q, want %q", broker.typedEvents[0].eventType, agentActionSSEType)
	}
	payload, ok := broker.typedEvents[0].payload.(AgentActionEvent)
	if !ok {
		t.Fatalf("typed payload = %T, want AgentActionEvent", broker.typedEvents[0].payload)
	}
	if payload.SessionID != "sess-typed" || payload.MessageSource != "web_chat" {
		t.Fatalf("typed payload attribution = %#v", payload)
	}
}

func TestAgentActionLedgerRejectsTransitionsAfterTerminalState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	ledger := newAgentActionLedger(stm, logger, NoopBroker{}, "sess-1", "web_chat")
	action, err := ledger.ProposeTool("turn-1", ToolCall{Action: "execute_shell"})
	if err != nil {
		t.Fatalf("ProposeTool: %v", err)
	}
	action, err = ledger.Fail(action, "schema error")
	if err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if _, err := ledger.Start(action); err == nil {
		t.Fatal("expected transition after terminal state to fail")
	}
}

func TestAgentActionLedgerMarksStalledStartedActions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	metadata, _ := json.Marshal(agentActionAuditMetadata{
		ActionID:     "act_stalled",
		ActionState:  string(AgentActionStateStarted),
		StateHistory: []string{"proposed", "accepted", "started"},
		ArgsHash:     "hash",
	})
	_, err = stm.RecordAuditEvent(memory.AuditEvent{
		Timestamp:     time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
		Source:        memory.AuditSourceAgentTool,
		EventType:     agentActionAuditEventType,
		Actor:         "agent",
		SessionID:     "sess-1",
		TargetID:      "execute_shell",
		TargetName:    "execute_shell",
		Status:        memory.AuditStatusRunning,
		Summary:       "execute_shell started",
		CorrelationID: "agent_action:act_stalled",
		MetadataJSON:  string(metadata),
	})
	if err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}

	broker := &actionLedgerCaptureBroker{}
	ledger := newAgentActionLedger(stm, logger, broker, "sess-1", "web_chat")
	if err := ledger.MarkStalledActions(5 * time.Minute); err != nil {
		t.Fatalf("MarkStalledActions: %v", err)
	}

	page, err := stm.SearchAuditEvents(memory.AuditFilter{
		Source: memory.AuditSourceAgentTool,
		Type:   agentActionAuditEventType,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("SearchAuditEvents: %v", err)
	}
	if page.Entries[0].Status != memory.AuditStatusError {
		t.Fatalf("status = %q, want error", page.Entries[0].Status)
	}
	var got agentActionAuditMetadata
	if err := json.Unmarshal([]byte(page.Entries[0].MetadataJSON), &got); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if got.ActionState != string(AgentActionStateFailed) {
		t.Fatalf("action state = %q, want failed", got.ActionState)
	}
	if !strings.Contains(got.Error, "stalled") {
		t.Fatalf("error = %q, want stalled marker", got.Error)
	}
	if len(broker.jsonMessages) != 1 {
		t.Fatalf("typed events = %d, want 1", len(broker.jsonMessages))
	}
}

func TestMarkAllStalledAgentActionsAcrossSessionsIgnoresHumanApproval(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	startedMeta, _ := json.Marshal(agentActionAuditMetadata{
		ActionID:     "act_old_started",
		ActionState:  string(AgentActionStateStarted),
		StateHistory: []string{"proposed", "accepted", "started"},
	})
	approvalMeta, _ := json.Marshal(agentActionAuditMetadata{
		ActionID:     "act_waiting",
		ActionState:  string(AgentActionStateNeedsHumanApproval),
		StateHistory: []string{"proposed", "accepted", "needs_human_approval"},
	})
	for _, event := range []memory.AuditEvent{
		{
			Timestamp:     time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
			Source:        memory.AuditSourceAgentTool,
			EventType:     agentActionAuditEventType,
			Actor:         "agent",
			SessionID:     "sess-a",
			TargetID:      "execute_shell",
			TargetName:    "execute_shell",
			Status:        memory.AuditStatusRunning,
			Summary:       "execute_shell started",
			CorrelationID: "agent_action:act_old_started",
			MetadataJSON:  string(startedMeta),
		},
		{
			Timestamp:     time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339),
			Source:        memory.AuditSourceAgentTool,
			EventType:     agentActionAuditEventType,
			Actor:         "agent",
			SessionID:     "sess-b",
			TargetID:      "question_user",
			TargetName:    "question_user",
			Status:        memory.AuditStatusRunning,
			Summary:       "question_user waiting for user approval",
			CorrelationID: "agent_action:act_waiting",
			MetadataJSON:  string(approvalMeta),
		},
	} {
		if _, err := stm.RecordAuditEvent(event); err != nil {
			t.Fatalf("RecordAuditEvent: %v", err)
		}
	}

	if err := MarkAllStalledAgentActions(stm, logger, NoopBroker{}, 5*time.Minute); err != nil {
		t.Fatalf("MarkAllStalledAgentActions: %v", err)
	}

	failed, err := stm.SearchAuditEvents(memory.AuditFilter{Status: memory.AuditStatusError, Type: agentActionAuditEventType, Limit: 10})
	if err != nil {
		t.Fatalf("SearchAuditEvents failed: %v", err)
	}
	if failed.Total != 1 || failed.Entries[0].SessionID != "sess-a" {
		t.Fatalf("failed actions = %#v, want only sess-a", failed.Entries)
	}
	waiting, err := stm.SearchAuditEvents(memory.AuditFilter{Status: memory.AuditStatusRunning, Type: agentActionAuditEventType, Limit: 10})
	if err != nil {
		t.Fatalf("SearchAuditEvents running: %v", err)
	}
	if waiting.Total != 1 || waiting.Entries[0].SessionID != "sess-b" {
		t.Fatalf("running actions = %#v, want only human approval", waiting.Entries)
	}
}

func TestAgentActionLedgerCompletesAfterHumanApprovalWait(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })

	ledger := newAgentActionLedger(stm, logger, NoopBroker{}, "sess-1", "web_chat")
	action, err := ledger.ProposeTool("turn-1", ToolCall{Action: "question_user"})
	if err != nil {
		t.Fatalf("ProposeTool: %v", err)
	}
	action, err = ledger.Accept(action, "validated")
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	action, err = ledger.WaitForHuman(action)
	if err != nil {
		t.Fatalf("WaitForHuman: %v", err)
	}
	if action.State != string(AgentActionStateNeedsHumanApproval) {
		t.Fatalf("state = %q, want needs_human_approval", action.State)
	}
	action, err = ledger.CompleteTool(action, toolExecutionResult{
		Content: `Tool Output: {"status":"answered"}`,
		Outcome: ExecutionOutcomeSuccess,
	}, 1200)
	if err != nil {
		t.Fatalf("CompleteTool: %v", err)
	}
	if action.State != string(AgentActionStateSucceeded) {
		t.Fatalf("state = %q, want succeeded", action.State)
	}
}

func TestAgentToolLoopDoesNotWriteLegacyToolCallAuditRows(t *testing.T) {
	src, err := os.ReadFile("agent_loop_tools.go")
	if err != nil {
		t.Fatalf("read agent_loop_tools.go: %v", err)
	}
	if strings.Contains(string(src), "recordToolAuditEvent(") {
		t.Fatal("agent tool loop must use agent_action ledger audit rows instead of legacy tool_call audit rows")
	}
}

func TestCoAgentRunConfigSetsMessageSource(t *testing.T) {
	src, err := os.ReadFile("coagent.go")
	if err != nil {
		t.Fatalf("read coagent.go: %v", err)
	}
	if !strings.Contains(string(src), `MessageSource:`) || !strings.Contains(string(src), `"co_agent"`) {
		t.Fatal("co-agent RunConfig must set MessageSource to co_agent for ledger attribution")
	}
}
