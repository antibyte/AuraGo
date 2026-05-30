package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/memory"
	"aurago/internal/security"
)

const (
	agentActionAuditEventType      = "agent_action"
	agentActionSSEType             = "agent_action"
	defaultAgentActionStallTimeout = 5 * time.Minute
)

type AgentActionState string

const (
	AgentActionStateProposed           AgentActionState = "proposed"
	AgentActionStateAccepted           AgentActionState = "accepted"
	AgentActionStateStarted            AgentActionState = "started"
	AgentActionStateSucceeded          AgentActionState = "succeeded"
	AgentActionStateFailed             AgentActionState = "failed"
	AgentActionStateBlocked            AgentActionState = "blocked"
	AgentActionStateCancelled          AgentActionState = "cancelled"
	AgentActionStateNeedsHumanApproval AgentActionState = "needs_human_approval"
	AgentActionStateSanitized          AgentActionState = "sanitized"
)

// AgentActionEvent is the public wire shape for action lifecycle updates.
type AgentActionEvent struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	TurnID        string `json:"turn_id,omitempty"`
	ToolName      string `json:"tool_name"`
	State         string `json:"state"`
	Status        string `json:"status"`
	Summary       string `json:"summary"`
	Result        string `json:"result,omitempty"`
	Error         string `json:"error,omitempty"`
	CorrelationID string `json:"correlation_id"`
	ArgsHash      string `json:"args_hash,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`

	stateHistory  []string
	messageSource string
	toolCall      ToolCall
}

type agentActionAuditMetadata struct {
	ActionID      string   `json:"action_id"`
	TurnID        string   `json:"turn_id,omitempty"`
	ActionState   string   `json:"action_state"`
	StateHistory  []string `json:"state_history,omitempty"`
	ArgsHash      string   `json:"args_hash,omitempty"`
	MessageSource string   `json:"message_source,omitempty"`
	NativeCallID  string   `json:"native_call_id,omitempty"`
	Operation     string   `json:"operation,omitempty"`
	HasCommand    bool     `json:"has_command,omitempty"`
	HasPath       bool     `json:"has_path,omitempty"`
	Result        string   `json:"result,omitempty"`
	Error         string   `json:"error,omitempty"`
}

type agentActionLedger struct {
	stm           *memory.SQLiteMemory
	logger        *slog.Logger
	broker        FeedbackBroker
	sessionID     string
	messageSource string
}

func newAgentActionLedger(stm *memory.SQLiteMemory, logger *slog.Logger, broker FeedbackBroker, sessionID, messageSource string) *agentActionLedger {
	if broker == nil {
		broker = NoopBroker{}
	}
	return &agentActionLedger{
		stm:           stm,
		logger:        logger,
		broker:        broker,
		sessionID:     strings.TrimSpace(sessionID),
		messageSource: strings.TrimSpace(messageSource),
	}
}

func (l *agentActionLedger) ProposeTool(turnID string, tc ToolCall) (AgentActionEvent, error) {
	trackingTC := toolCallForExecutionTracking(tc)
	toolName := strings.TrimSpace(trackingTC.Action)
	if toolName == "" {
		toolName = "tool"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	argsHash := hashToolCallArgs(trackingTC)
	id := newAgentActionID(l.sessionID, turnID, toolName, trackingTC.NativeCallID, argsHash)
	action := AgentActionEvent{
		ID:            id,
		SessionID:     l.sessionID,
		TurnID:        strings.TrimSpace(turnID),
		ToolName:      toolName,
		State:         string(AgentActionStateProposed),
		Status:        memory.AuditStatusRunning,
		Summary:       fmt.Sprintf("%s proposed", toolName),
		CorrelationID: "agent_action:" + id,
		ArgsHash:      argsHash,
		CreatedAt:     now,
		UpdatedAt:     now,
		stateHistory:  []string{string(AgentActionStateProposed)},
		messageSource: l.messageSource,
		toolCall:      trackingTC,
	}
	return l.persist(action, trackingTC, 0)
}

func (l *agentActionLedger) Accept(action AgentActionEvent, summary string) (AgentActionEvent, error) {
	if strings.TrimSpace(summary) == "" {
		summary = fmt.Sprintf("%s accepted", action.ToolName)
	}
	return l.transition(action, AgentActionStateAccepted, summary, "", "", ToolCall{}, 0)
}

func (l *agentActionLedger) Start(action AgentActionEvent) (AgentActionEvent, error) {
	return l.transition(action, AgentActionStateStarted, fmt.Sprintf("%s started", action.ToolName), "", "", ToolCall{}, 0)
}

func (l *agentActionLedger) WaitForHuman(action AgentActionEvent) (AgentActionEvent, error) {
	return l.transition(action, AgentActionStateNeedsHumanApproval, fmt.Sprintf("%s waiting for user approval", action.ToolName), "", "", ToolCall{}, 0)
}

func (l *agentActionLedger) CompleteTool(action AgentActionEvent, result toolExecutionResult, durationMS int64) (AgentActionEvent, error) {
	state := AgentActionStateSucceeded
	errText := ""
	switch auditStatusForToolOutcome(result.Outcome, result.Failed) {
	case memory.AuditStatusBlocked:
		state = AgentActionStateBlocked
		errText = result.Content
	case memory.AuditStatusSanitized:
		state = AgentActionStateSanitized
	case memory.AuditStatusError:
		state = AgentActionStateFailed
		errText = result.Content
	}
	summary := fmt.Sprintf("%s %s", action.ToolName, state)
	return l.transition(action, state, summary, result.Content, errText, ToolCall{}, durationMS)
}

func (l *agentActionLedger) Fail(action AgentActionEvent, errText string) (AgentActionEvent, error) {
	return l.transition(action, AgentActionStateFailed, fmt.Sprintf("%s failed", action.ToolName), "", errText, ToolCall{}, 0)
}

func (l *agentActionLedger) Block(action AgentActionEvent, result string) (AgentActionEvent, error) {
	return l.transition(action, AgentActionStateBlocked, fmt.Sprintf("%s blocked", action.ToolName), result, result, ToolCall{}, 0)
}

func (l *agentActionLedger) MarkStalledActions(timeout time.Duration) error {
	if l == nil || l.stm == nil || timeout <= 0 {
		return nil
	}
	page, err := l.stm.SearchAuditEvents(memory.AuditFilter{
		Source: memory.AuditSourceAgentTool,
		Status: memory.AuditStatusRunning,
		Type:   agentActionAuditEventType,
		Limit:  200,
	})
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, event := range page.Entries {
		if l.sessionID != "" && event.SessionID != l.sessionID {
			continue
		}
		var metadata agentActionAuditMetadata
		if err := json.Unmarshal([]byte(event.MetadataJSON), &metadata); err != nil {
			continue
		}
		state := AgentActionState(metadata.ActionState)
		if state != AgentActionStateAccepted && state != AgentActionStateStarted {
			continue
		}
		ts, err := parseAgentActionTimestamp(event.Timestamp)
		if err != nil || now.Sub(ts) < timeout {
			continue
		}
		action := AgentActionEvent{
			ID:            firstNonEmpty(metadata.ActionID, strings.TrimPrefix(event.CorrelationID, "agent_action:")),
			SessionID:     event.SessionID,
			TurnID:        metadata.TurnID,
			ToolName:      firstNonEmpty(event.TargetName, event.TargetID, "tool"),
			State:         string(state),
			Status:        event.Status,
			Summary:       event.Summary,
			CorrelationID: event.CorrelationID,
			ArgsHash:      metadata.ArgsHash,
			CreatedAt:     event.Timestamp,
			UpdatedAt:     event.Timestamp,
			stateHistory:  append([]string(nil), metadata.StateHistory...),
			messageSource: firstNonEmpty(metadata.MessageSource, l.messageSource),
		}
		if action.CorrelationID == "" && action.ID != "" {
			action.CorrelationID = "agent_action:" + action.ID
		}
		if _, err := l.Fail(action, fmt.Sprintf("agent action stalled after %s", timeout)); err != nil && l.logger != nil {
			l.logger.Warn("Failed to mark stalled agent action", "tool", action.ToolName, "error", err)
		}
	}
	return nil
}

func (l *agentActionLedger) transition(action AgentActionEvent, state AgentActionState, summary, result, errText string, tc ToolCall, durationMS int64) (AgentActionEvent, error) {
	if isTerminalAgentActionState(AgentActionState(action.State)) {
		return action, fmt.Errorf("agent action %s is already terminal: %s", action.ID, action.State)
	}
	if err := validateAgentActionTransition(AgentActionState(action.State), state); err != nil {
		return action, err
	}
	action.State = string(state)
	action.Status = auditStatusForAgentActionState(state)
	action.Summary = strings.TrimSpace(summary)
	action.Result = truncateActionWireText(result)
	action.Error = truncateActionWireText(errText)
	action.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	action.messageSource = l.messageSource
	action.stateHistory = appendActionState(action.stateHistory, string(state))
	if strings.TrimSpace(action.toolCall.Action) == "" {
		action.toolCall = tc
	}
	return l.persist(action, tc, durationMS)
}

func (l *agentActionLedger) persist(action AgentActionEvent, tc ToolCall, durationMS int64) (AgentActionEvent, error) {
	if strings.TrimSpace(tc.Action) == "" {
		tc = action.toolCall
	}
	if action.Status == "" {
		action.Status = auditStatusForAgentActionState(AgentActionState(action.State))
	}
	if len(action.stateHistory) == 0 {
		action.stateHistory = []string{action.State}
	}
	metadata, _ := json.Marshal(agentActionAuditMetadata{
		ActionID:      action.ID,
		TurnID:        action.TurnID,
		ActionState:   action.State,
		StateHistory:  append([]string(nil), action.stateHistory...),
		ArgsHash:      action.ArgsHash,
		MessageSource: action.messageSource,
		NativeCallID:  strings.TrimSpace(tc.NativeCallID),
		Operation:     strings.TrimSpace(firstNonEmpty(tc.Operation, tc.SubOperation)),
		HasCommand:    strings.TrimSpace(tc.Command) != "",
		HasPath:       strings.TrimSpace(firstNonEmpty(tc.Path, tc.FilePath, tc.LocalPath, tc.RemotePath)) != "",
		Result:        action.Result,
		Error:         action.Error,
	})
	auditEvent := memory.AuditEvent{
		Source:        memory.AuditSourceAgentTool,
		EventType:     agentActionAuditEventType,
		Actor:         "agent",
		SessionID:     action.SessionID,
		TargetID:      action.ToolName,
		TargetName:    action.ToolName,
		Status:        action.Status,
		Summary:       action.Summary,
		Detail:        action.Result,
		DurationMS:    durationMS,
		CorrelationID: action.CorrelationID,
		MetadataJSON:  string(metadata),
	}
	if l.stm != nil {
		if err := l.stm.UpsertAuditEventByCorrelation(auditEvent); err != nil {
			if l.logger != nil {
				l.logger.Warn("Failed to record agent action event", "tool", action.ToolName, "state", action.State, "error", err)
			}
			return action, err
		}
	}
	l.emit(action)
	return action, nil
}

func (l *agentActionLedger) emit(action AgentActionEvent) {
	if l.broker == nil {
		return
	}
	payload, err := json.Marshal(struct {
		Type    string           `json:"type"`
		Payload AgentActionEvent `json:"payload"`
	}{
		Type:    agentActionSSEType,
		Payload: action,
	})
	if err != nil {
		return
	}
	l.broker.SendJSON(string(payload))
}

func auditStatusForAgentActionState(state AgentActionState) string {
	switch state {
	case AgentActionStateSucceeded:
		return memory.AuditStatusSuccess
	case AgentActionStateFailed:
		return memory.AuditStatusError
	case AgentActionStateBlocked:
		return memory.AuditStatusBlocked
	case AgentActionStateSanitized:
		return memory.AuditStatusSanitized
	case AgentActionStateCancelled:
		return memory.AuditStatusWarning
	default:
		return memory.AuditStatusRunning
	}
}

func isTerminalAgentActionState(state AgentActionState) bool {
	switch state {
	case AgentActionStateSucceeded, AgentActionStateFailed, AgentActionStateBlocked, AgentActionStateCancelled, AgentActionStateSanitized:
		return true
	default:
		return false
	}
}

func validateAgentActionTransition(from, to AgentActionState) error {
	if from == "" {
		return nil
	}
	if from == to {
		return nil
	}
	if isTerminalAgentActionState(from) {
		return fmt.Errorf("agent action is already terminal: %s", from)
	}
	switch from {
	case AgentActionStateProposed:
		switch to {
		case AgentActionStateAccepted, AgentActionStateFailed, AgentActionStateCancelled, AgentActionStateNeedsHumanApproval:
			return nil
		}
	case AgentActionStateAccepted:
		switch to {
		case AgentActionStateStarted, AgentActionStateFailed, AgentActionStateBlocked, AgentActionStateCancelled, AgentActionStateNeedsHumanApproval:
			return nil
		}
	case AgentActionStateStarted:
		if isTerminalAgentActionState(to) {
			return nil
		}
	case AgentActionStateNeedsHumanApproval:
		if isTerminalAgentActionState(to) {
			return nil
		}
		switch to {
		case AgentActionStateAccepted, AgentActionStateFailed, AgentActionStateCancelled:
			return nil
		}
	}
	return fmt.Errorf("invalid agent action transition %s -> %s", from, to)
}

func appendActionState(history []string, state string) []string {
	if state == "" {
		return history
	}
	if len(history) > 0 && history[len(history)-1] == state {
		return history
	}
	return append(history, state)
}

func hashToolCallArgs(tc ToolCall) string {
	normalized, err := json.Marshal(tc)
	if err != nil {
		normalized = []byte(tc.Action)
	}
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

func newAgentActionID(sessionID, turnID, toolName, nativeCallID, argsHash string) string {
	seed := strings.Join([]string{
		strings.TrimSpace(sessionID),
		strings.TrimSpace(turnID),
		strings.TrimSpace(toolName),
		strings.TrimSpace(nativeCallID),
		strings.TrimSpace(argsHash),
		time.Now().UTC().Format(time.RFC3339Nano),
	}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "act_" + hex.EncodeToString(sum[:8])
}

func truncateActionWireText(text string) string {
	text = strings.TrimSpace(security.Scrub(text))
	if len([]rune(text)) <= 300 {
		return text
	}
	return strings.TrimSpace(string([]rune(text)[:297])) + "..."
}

func parseAgentActionTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	if ts, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return ts.UTC(), nil
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return ts.UTC(), nil
}

func beginAgentToolAction(s *agentLoopState, tc ToolCall, turnID string) (*agentActionLedger, AgentActionEvent) {
	if s == nil {
		return nil, AgentActionEvent{}
	}
	ledger := newAgentActionLedger(s.runCfg.ShortTermMem, s.currentLogger, s.broker, s.runCfg.SessionID, s.runCfg.MessageSource)
	action, err := ledger.ProposeTool(turnID, tc)
	if err != nil {
		if s.currentLogger != nil {
			s.currentLogger.Warn("Failed to propose agent action", "tool", tc.Action, "error", err)
		}
		return ledger, AgentActionEvent{}
	}
	action, err = ledger.Accept(action, "validated")
	if err != nil && s.currentLogger != nil {
		s.currentLogger.Warn("Failed to accept agent action", "tool", tc.Action, "error", err)
	}
	return ledger, action
}

func startAgentToolAction(logger *slog.Logger, ledger *agentActionLedger, action AgentActionEvent) AgentActionEvent {
	if ledger == nil || action.ID == "" {
		return action
	}
	var (
		next AgentActionEvent
		err  error
	)
	if action.ToolName == "question_user" {
		next, err = ledger.WaitForHuman(action)
	} else {
		next, err = ledger.Start(action)
	}
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to start agent action", "tool", action.ToolName, "error", err)
		}
		return action
	}
	return next
}

func completeAgentToolAction(logger *slog.Logger, ledger *agentActionLedger, action AgentActionEvent, result toolExecutionResult, durationMS int64) AgentActionEvent {
	if ledger == nil || action.ID == "" {
		return action
	}
	next, err := ledger.CompleteTool(action, result, durationMS)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to complete agent action", "tool", action.ToolName, "error", err)
		}
		return action
	}
	return next
}

func blockAgentToolAction(logger *slog.Logger, ledger *agentActionLedger, action AgentActionEvent, result string) AgentActionEvent {
	if ledger == nil || action.ID == "" {
		return action
	}
	next, err := ledger.Block(action, result)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to block agent action", "tool", action.ToolName, "error", err)
		}
		return action
	}
	return next
}

func failAgentToolAction(logger *slog.Logger, ledger *agentActionLedger, action AgentActionEvent, errText string) AgentActionEvent {
	if ledger == nil || action.ID == "" {
		return action
	}
	next, err := ledger.Fail(action, errText)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to fail agent action", "tool", action.ToolName, "error", err)
		}
		return action
	}
	return next
}

func agentActionTurnID(sessionID string, reqMessageCount, toolCallCount int) string {
	return fmt.Sprintf("%s:%d:%d", strings.TrimSpace(sessionID), reqMessageCount, toolCallCount)
}
