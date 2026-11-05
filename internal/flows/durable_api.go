package flows

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrDurableDisabled           = errors.New("durable processing is disabled")
	ErrDurableUnavailable        = errors.New("durable processing is unavailable")
	ErrDurableCapacity           = errors.New("durable processing capacity is exhausted")
	ErrDurableInvalidGraph       = errors.New("the graph cannot run durably")
	ErrDurableGenerationConflict = errors.New("the durable generation changed")
	ErrDurableMissionCancel      = errors.New("durable runs require an individual generation-fenced cancellation")
	ErrDurableApprovalAuthUnavailable = errors.New("durable approvals require a configured owner login")
	ErrDurableApprovalConflict        = errors.New("the durable approval changed elsewhere")
	ErrDurableApprovalNotFound        = errors.New("the durable approval was not found")
	ErrDurableApprovalUnavailable     = errors.New("the durable approval is unavailable")
)

// DurableAuthority must inspect current gates without calling back into a flow
// lock. Nil always denies durable execution.
type DurableAuthority interface {
	DurableEnabled(context.Context) (bool, error)
	DurableAllowed(context.Context) error
	AuthorizeDurable(context.Context, string, string, int) error
}

// DurableApprovalAuthority is checked outside flow, runner and SQLite locks.
// Missing approval authority fails closed for every approval-capable flow.
type DurableApprovalAuthority interface {
	DurableApprovalAllowed(context.Context) error
}

// DurableHistoryProjector rereads committed authoritative state; it must not
// invoke mission completion, dependent triggers or notifications.
type DurableHistoryProjector interface {
	ProjectDurableRun(context.Context, string) error
}

type DurableStatus struct {
	Configured        bool   `json:"configured"`
	Effective         bool   `json:"effective"`
	RecoveryAvailable bool   `json:"recovery_available"`
	Reason            string `json:"reason,omitempty"`
	WaitingCount      int    `json:"waiting_count"`
	ClaimedCount      int    `json:"claimed_count"`
	BlockedCount      int    `json:"blocked_count"`
}

type DurableRunStatus struct {
	RunID           string     `json:"run_id"`
	State           string     `json:"state"`
	BlockedCode     string     `json:"blocked_code,omitempty"`
	WakeAt          *time.Time `json:"wake_at,omitempty"`
	Deadline        *time.Time `json:"deadline,omitempty"`
	Generation      int64      `json:"generation"`
	CancelAvailable bool       `json:"cancel_available"`
	ApprovalGateID  string     `json:"approval_gate_id,omitempty"`
}

const (
	DurableApprovalPending   = "pending"
	DurableApprovalApproved  = "approved"
	DurableApprovalRejected  = "rejected"
	DurableApprovalExpired   = "expired"
	DurableApprovalCancelled = "cancelled"
	DurableApprovalStale     = "stale"
	DurableApprovalUnknown   = "unknown"

	DurableApprovalBlockAuthUnavailable = "approval_auth_unavailable"
	DurableApprovalBlockSessionRequired = "session_required"
	DurableApprovalBlockReadOnly        = "readonly"
	DurableApprovalBlockDurableDisabled = "durable_disabled"
	DurableApprovalBlockStale           = "stale"
	DurableApprovalBlockKeyUnavailable  = "key_unavailable"
	DurableApprovalBlockCheckpoint      = "checkpoint_unavailable"
	DurableApprovalBlockDecided         = "decision_complete"
	DurableApprovalBlockDeadline        = "deadline_elapsed"

	DurableApprovalDecisionApprove = "approve"
	DurableApprovalDecisionReject  = "reject"
)

// DurableApprovalItem contains bounded gate metadata only. Review payloads are
// returned separately and only while the encrypted checkpoint is retained.
type DurableApprovalItem struct {
	GateID          string     `json:"gate_id"`
	RunID           string     `json:"run_id"`
	FlowID          string     `json:"flow_id"`
	FlowName        string     `json:"flow_name"`
	Revision        int        `json:"revision"`
	NodeID          string     `json:"node_id"`
	NodeKey         string     `json:"node_key"`
	NodeLabel       string     `json:"node_label,omitempty"`
	AdmissionOwner  string     `json:"admission_owner"`
	GateGeneration  int64      `json:"gate_generation"`
	ContentDigest   string     `json:"content_digest"`
	ExpiresAt       time.Time  `json:"expires_at"`
	State           string     `json:"state"`
	CreatedAt       time.Time  `json:"created_at"`
	DecidedAt       *time.Time `json:"decided_at,omitempty"`
	Decision        string     `json:"decision,omitempty"`
	Actor           string     `json:"actor,omitempty"`
	ReviewAvailable bool       `json:"review_available"`
	CanDecide       bool       `json:"can_decide"`
	BlockedReason   string     `json:"blocked_reason"`
}

type DurableApprovalPage struct {
	Items      []DurableApprovalItem `json:"items"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

type DurableApprovalReview struct {
	Summary    string          `json:"summary"`
	ReviewData json.RawMessage `json:"review_data"`
}

type DurableApprovalDetail struct {
	DurableApprovalItem
	Review *DurableApprovalReview `json:"review,omitempty"`
}

type DurableApprovalResult struct {
	DurableApprovalItem
	RunState         RunStatus `json:"run_state"`
	ResumeGeneration *int64    `json:"resume_generation,omitempty"`
	Idempotent       bool      `json:"idempotent"`
}

type DurableProjection struct {
	RunID      string     `json:"run_id"`
	FlowID     string     `json:"flow_id"`
	MissionID  string     `json:"mission_id"`
	Revision   int        `json:"revision"`
	Status     RunStatus  `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	ErrorCode  string     `json:"error_code,omitempty"`
}

type DurableProjectionPage struct {
	Items      []DurableProjection `json:"items"`
	NextCursor string              `json:"next_cursor,omitempty"`
}
