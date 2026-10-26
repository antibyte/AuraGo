package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

const actionApprovalTTL = 10 * time.Minute

const (
	// actionApprovalSnapshotMaxBytes is the normal one-shot snapshot cap.
	// An allow rule can still authorize a larger high-risk call.
	actionApprovalSnapshotMaxBytes = 32 << 10
	// copiedSpanSnapshotMaxBytes bounds a copied-span approval. An allow rule
	// cannot exempt that copy, so the exact call has to fit in memory.
	copiedSpanSnapshotMaxBytes = 256 << 10
	// actionApprovalPreviewMaxBytes bounds the text shown in the dialog.
	// The stored call stays exact.
	actionApprovalPreviewMaxBytes = 8 << 10
)

const copiedSpanApprovalMessage = "This high-risk action copies isolated tool text and needs a one-shot approval. No side effect was executed. Do not repeat the call. An allow rule does not exempt the copy."

var ErrActionApprovalUnavailable = errors.New("action approval is unavailable, expired or already decided")

type approvedActionKey struct{}
type actionPreviewKey struct{}
type approvedActionGrant struct {
	SessionID, ArgumentsHash string
	ExpiresAt                time.Time
	now                      func() time.Time
}

func (g approvedActionGrant) expired() bool {
	if g.ExpiresAt.IsZero() {
		return false
	}
	now := time.Now
	if g.now != nil {
		now = g.now
	}
	return !now().Before(g.ExpiresAt)
}

// ActionApproval never exposes the executable payload. Preview text is scrubbed;
// exact call data exists only in bounded process memory until decision/expiry.
type ActionApproval struct {
	ID               string           `json:"id"`
	SessionID        string           `json:"session_id"`
	Action           ActionDescriptor `json:"action"`
	ArgumentsPreview string           `json:"arguments_preview"`
	CreatedAt        time.Time        `json:"created_at"`
	ExpiresAt        time.Time        `json:"expires_at"`
	State            string           `json:"state"`
}

type pendingActionApproval struct {
	view   ActionApproval
	call   ToolCall
	dc     DispatchContext
	ledger *agentActionLedger
	event  AgentActionEvent
	cancel context.CancelFunc
}

type actionApprovalManager struct {
	mu      sync.Mutex
	entries map[string]*pendingActionApproval
	now     func() time.Time
}

func newActionApprovalManager() *actionApprovalManager {
	return &actionApprovalManager{entries: make(map[string]*pendingActionApproval), now: time.Now}
}

var actionApprovals = newActionApprovalManager()

func (m *actionApprovalManager) expireLocked() {
	for id, item := range m.entries {
		if item.view.State == "pending" && !m.now().Before(item.view.ExpiresAt) {
			_, _ = item.ledger.transition(item.event, AgentActionStateCancelled, "Approval expired; no action was executed", "", "", ToolCall{}, 0)
			delete(m.entries, id)
		}
	}
}

func requestActionApproval(tc ToolCall, dc *DispatchContext, action ActionDescriptor) *ToolDispatchResult {
	return actionApprovals.request(tc, dc, action, false)
}

// requestCopiedSpanApproval holds a high-risk call that copies isolated tool
// text. An allow rule cannot authorize it. Calls up to 256 KiB stay approvable;
// only the dialog preview is shortened.
func requestCopiedSpanApproval(tc ToolCall, dc *DispatchContext, action ActionDescriptor) *ToolDispatchResult {
	return actionApprovals.request(tc, dc, action, true)
}

func (m *actionApprovalManager) request(tc ToolCall, dc *DispatchContext, action ActionDescriptor, copiedSpan bool) *ToolDispatchResult {
	payload, err := json.Marshal(tc)
	limit := actionApprovalSnapshotMaxBytes
	if copiedSpan {
		limit = copiedSpanSnapshotMaxBytes
	}
	if err != nil || len(payload) > limit {
		if copiedSpan {
			return actionPolicyRefusal("copied_span_too_large", "This high-risk action copies isolated tool text and is too large to hold for one-shot approval. It was not executed. Shorten the copied text. An allow rule does not exempt it.", ToolResultDenied)
		}
		return actionPolicyRefusal("action_rule_required", "This action is too large for a safe approval preview; configure an exact administrator action rule.", ToolResultDenied)
	}
	type snapshotToolCall ToolCall
	var snapshot snapshotToolCall
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return actionPolicyRefusal("invalid_arguments", "Cannot snapshot action arguments.", ToolResultDenied)
	}
	call := ToolCall(snapshot)
	call.NativeCallID, call.TransportAction, call.nativeCall = tc.NativeCallID, tc.TransportAction, tc.nativeCall
	local := *dc
	local.AllowedTools = cloneActionToolSet(dc.AllowedTools)
	local.AllowedAgentSkills = cloneActionToolSet(dc.AllowedAgentSkills)
	preview := make(map[string]interface{})
	_ = json.Unmarshal(payload, &preview)
	for key, value := range preview {
		if value == nil || value == "" || value == false || value == float64(0) {
			delete(preview, key)
		}
	}
	previewBytes, _ := json.MarshalIndent(preview, "", "  ")
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	count := 0
	for _, existing := range m.entries {
		if existing.view.SessionID == dc.SessionID {
			count++
			if existing.view.State == "pending" && existing.view.Action.ArgumentsHash == action.ArgumentsHash {
				if copiedSpan {
					return approvalRequiredMessage(existing.view, copiedSpanApprovalMessage)
				}
				return approvalRequiredResult(existing.view)
			}
		}
	}
	if len(m.entries) >= 256 || count >= 16 {
		return actionPolicyRefusal("approval_capacity", "Resolve existing approvals before proposing another high-risk action.", ToolResultDenied)
	}
	ledger := dc.actionLedger
	var event AgentActionEvent
	if ledger != nil && dc.actionEvent != nil {
		event = *dc.actionEvent
	} else {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return actionPolicyRefusal("approval_unavailable", "Cannot create an approval identity.", ToolResultDenied)
		}
		ledger = newAgentActionLedger(dc.ShortTermMem, dc.Logger, dc.Broker, dc.SessionID, dc.MessageSource)
		event, err = ledger.ProposeTool("approval:"+hex.EncodeToString(nonce[:]), tc)
		if err != nil {
			return actionPolicyRefusal("approval_unavailable", "Cannot persist the action audit.", ToolResultDenied)
		}
	}
	event, err = ledger.WaitForHuman(event)
	if err != nil {
		return actionPolicyRefusal("approval_unavailable", "Cannot record the approval state.", ToolResultDenied)
	}
	if dc.actionEvent != nil {
		*dc.actionEvent = event
	}
	previewText := security.RedactSensitiveInfo(security.Scrub(string(previewBytes)))
	if copiedSpan {
		previewText = boundApprovalPreview(previewText)
	}
	view := ActionApproval{ID: event.ID, SessionID: dc.SessionID, Action: action, ArgumentsPreview: previewText, CreatedAt: m.now().UTC(), ExpiresAt: m.now().Add(actionApprovalTTL).UTC(), State: "pending"}
	view.Action.Target = safeActionTarget(view.Action.Target)
	m.entries[view.ID] = &pendingActionApproval{view: view, call: call, dc: local, ledger: ledger, event: event}
	if dc.Broker != nil {
		encoded, _ := json.Marshal(view)
		dc.Broker.Send("action_approval", string(encoded))
	}
	if copiedSpan {
		return approvalRequiredMessage(view, copiedSpanApprovalMessage)
	}
	return approvalRequiredResult(view)
}

func boundApprovalPreview(preview string) string {
	note := "\n… preview shortened; approval still runs the exact call."
	if len(preview) <= actionApprovalPreviewMaxBytes {
		return preview
	}
	budget := actionApprovalPreviewMaxBytes - len(note)
	if budget < 0 {
		budget = 0
	}
	cut := preview
	if len(cut) > budget {
		cut = cut[:budget]
	}
	for cut != "" && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut + note
}

func approvalRequiredResult(view ActionApproval) *ToolDispatchResult {
	return approvalRequiredMessage(view, "This high-risk action requires an administrator decision. No side effect was executed. Do not repeat the call; configure an action rule for unattended use.")
}

func approvalRequiredMessage(view ActionApproval, message string) *ToolDispatchResult {
	encoded, _ := json.Marshal(map[string]interface{}{"status": "deferred", "code": "approval_required", "action_id": view.ID, "expires_at": view.ExpiresAt, "message": message})
	return &ToolDispatchResult{Output: "Tool Output: " + string(encoded), Status: ToolResultDeferred}
}

func cloneActionToolSet(source map[string]struct{}) map[string]struct{} {
	if source == nil {
		return nil
	}
	copy := make(map[string]struct{}, len(source))
	for key := range source {
		copy[key] = struct{}{}
	}
	return copy
}

// PendingActionApprovals returns only the requested conversation's approvals.
func PendingActionApprovals(sessionID string) []ActionApproval {
	return pendingActionApprovals(sessionID, false)
}

// AdministrativePendingActionApprovals is mounted only behind requireAdmin.
func AdministrativePendingActionApprovals() []ActionApproval { return pendingActionApprovals("", true) }

func pendingActionApprovals(sessionID string, all bool) []ActionApproval {
	m := actionApprovals
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	result := make([]ActionApproval, 0)
	for _, item := range m.entries {
		if item.view.State == "pending" && (all || item.view.SessionID == sessionID) {
			result = append(result, item.view)
		}
	}
	return result
}

// DecideActionApproval leases one exact call once. The ordinary dispatcher still
// revalidates current capabilities, readonly, scopes and explicit deny rules.
func DecideActionApproval(ctx context.Context, sessionID, id, decision string) (ToolDispatchResult, error) {
	if decision != "approve" && decision != "reject" {
		return ToolDispatchResult{}, fmt.Errorf("decision must be approve or reject")
	}
	m := actionApprovals
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	m.mu.Lock()
	m.expireLocked()
	item := m.entries[id]
	if item == nil || item.view.SessionID != sessionID || item.view.State != "pending" {
		m.mu.Unlock()
		return ToolDispatchResult{}, ErrActionApprovalUnavailable
	}
	if decision == "reject" {
		delete(m.entries, id)
		m.mu.Unlock()
		_, _ = item.ledger.transition(item.event, AgentActionStateCancelled, "Administrator rejected the action", "", "", ToolCall{}, 0)
		return *actionPolicyRefusal("approval_rejected", "The administrator rejected this action.", ToolResultDenied), nil
	}
	item.view.State, item.cancel = "running", cancel
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.entries, id); m.mu.Unlock() }()
	// Expiry bounds waiting for the turn gate only. Stop and deletion retain
	// the same registered context after the timer is stopped at admission.
	expired := make(chan struct{})
	gateTimer := time.AfterFunc(item.view.ExpiresAt.Sub(m.now()), func() { cancel(); close(expired) })
	defer gateTimer.Stop()
	cancelBeforeStart := func(err error) (ToolDispatchResult, error) {
		_, _ = item.ledger.transition(item.event, AgentActionStateCancelled, "Approval cancelled before execution", "", "", ToolCall{}, 0)
		if !m.now().Before(item.view.ExpiresAt) {
			err = ErrActionApprovalUnavailable
		}
		return ToolDispatchResult{}, err
	}
	if item.dc.ShortTermMem != nil {
		registered, release, err := item.dc.ShortTermMem.BeginSessionTurn(runCtx, sessionID)
		if err != nil {
			return cancelBeforeStart(err)
		}
		defer release()
		runCtx = registered
	}
	if !gateTimer.Stop() {
		<-expired // A racing expiry always wins; do not detach cancellation.
		return cancelBeforeStart(ErrActionApprovalUnavailable)
	}
	if !m.now().Before(item.view.ExpiresAt) {
		return cancelBeforeStart(ErrActionApprovalUnavailable)
	}
	if err := runCtx.Err(); err != nil {
		return cancelBeforeStart(err)
	}
	accepted, err := item.ledger.Accept(item.event, "Administrator approved the exact stored action")
	if err != nil {
		return ToolDispatchResult{}, err
	}
	local := item.dc
	local.actionLedger, local.actionEvent = item.ledger, &accepted
	grant := approvedActionGrant{SessionID: sessionID, ArgumentsHash: item.view.Action.ArgumentsHash, ExpiresAt: item.view.ExpiresAt, now: m.now}
	runCtx = context.WithValue(runCtx, approvedActionKey{}, grant)
	result := DispatchToolCallResult(runCtx, &item.call, &local, local.UserContext)
	state := AgentActionStateSucceeded
	if runCtx.Err() != nil {
		state = AgentActionStateCancelled
	} else if result.Status == ToolResultUnknown {
		state = AgentActionStateUnknown
	} else if result.IsError {
		state = AgentActionStateBlocked
	}
	_, _ = item.ledger.transition(accepted, state, "Approved action finished", "", "", ToolCall{}, local.ExecutionTimeMs)
	return result, nil
}

// CancelActionApprovals revokes pending calls and cancels an already leased call.
// It does not delete the conversation or create a memory deletion fence.
func CancelActionApprovals(sessionID string) {
	m := actionApprovals
	m.mu.Lock()
	var cancelled []*pendingActionApproval
	for id, item := range m.entries {
		if item.view.SessionID != sessionID {
			continue
		}
		cancelled = append(cancelled, item)
		delete(m.entries, id)
	}
	m.mu.Unlock()
	for _, item := range cancelled {
		if item.cancel != nil {
			item.cancel()
		}
	}
	for _, item := range cancelled {
		if item.view.State == "running" {
			continue
		}
		_, _ = item.ledger.transition(item.event, AgentActionStateCancelled, "Action cancelled by Stop", "", "", ToolCall{}, 0)
	}
}

// PreviewActionPolicy is side-effect free: it never creates an approval request.
func PreviewActionPolicy(tc ToolCall, dc *DispatchContext) (ActionDescriptor, string) {
	tc = prepareToolCall(tc, dc)
	a := DescribeAction(tc, dc)
	if tc.PreparationError != "" {
		return a, "denied"
	}
	ctx := context.WithValue(context.Background(), actionPreviewKey{}, true)
	if result := checkDispatchActionPolicy(ctx, tc, dc); result != nil {
		if strings.Contains(result.Output, "approval_required") || strings.Contains(result.Output, "action_rule_required") {
			return a, "rule_required"
		}
		return a, "denied"
	}
	return a, "allowed"
}
