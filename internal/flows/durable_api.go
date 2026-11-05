package flows

import (
	"context"
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
)

// DurableAuthority must inspect current gates without calling back into a flow
// lock. Nil always denies durable execution.
type DurableAuthority interface {
	DurableEnabled(context.Context) (bool, error)
	DurableAllowed(context.Context) error
	AuthorizeDurable(context.Context, string, string, int) error
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
