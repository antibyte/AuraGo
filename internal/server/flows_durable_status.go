package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"aurago/internal/flows"
)

const flowDurableStatusTimeout = 2 * time.Second

// Nil counts mean that no authoritative aggregate is currently available.
type flowDurableCapability struct {
	Configured        bool   `json:"configured"`
	Effective         bool   `json:"effective"`
	RecoveryAvailable bool   `json:"recovery_available"`
	Reason            string `json:"reason,omitempty"`
	WaitingCount      *int   `json:"waiting_count"`
	ClaimedCount      *int   `json:"claimed_count"`
	BlockedCount      *int   `json:"blocked_count"`
}

func (s *Server) flowDurableCapability(ctx context.Context) flowDurableCapability {
	var result flowDurableCapability
	if s == nil {
		result.Reason = "service_unavailable"
		return result
	}
	if cfg := s.ConfigSnapshot(); cfg != nil {
		result.Configured = cfg.Flows.DurableEnabled
	}
	if s.Flows == nil {
		result.Reason = "service_unavailable"
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, flowDurableStatusTimeout)
	defer cancel()
	status, err := s.Flows.DurableStatus(ctx)
	if err != nil {
		result.Reason = "storage_unavailable"
		return result
	}
	result.Configured = status.Configured
	result.Effective = status.Effective
	result.RecoveryAvailable = status.RecoveryAvailable
	result.Reason = status.Reason
	result.WaitingCount = &status.WaitingCount
	result.ClaimedCount = &status.ClaimedCount
	result.BlockedCount = &status.BlockedCount
	return result
}

// handleFlowRuntime is a bounded read-only status; it never opens a disabled
// service or creates a checkpoint key.
func (s *Server) handleFlowRuntime(w http.ResponseWriter, r *http.Request) {
	if !authenticateDesktopPermission(s, w, r, desktopScopeRead) {
		return
	}
	if !flowsOriginOK(r) {
		flowsError(w, http.StatusForbidden, "FLOW_PERMISSION_DENIED", "same-origin request required")
		return
	}
	if r.Method != http.MethodGet {
		flowsMethodNotAllowed(w)
		return
	}
	if !checkDesktopOperation(s, w, r, desktopRead) {
		return
	}
	flowsJSON(w, http.StatusOK, s.flowDurableCapability(r.Context()))
}

func (s *Server) cancelDurableFlowRun(w http.ResponseWriter, r *http.Request, runID string) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var request struct {
		Generation *int64 `json:"generation"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&request)
	if err == nil {
		if extra := decoder.Decode(new(any)); extra != io.EOF {
			err = extra
			if err == nil {
				err = errors.New("trailing JSON")
			}
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		flowsError(w, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE", "the request body is too large")
		return
	}
	if err != nil || request.Generation == nil || *request.Generation < 0 {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "a non-negative generation is required")
		return
	}
	accepted, err := s.Flows.CancelDurableRun(r.Context(), runID, *request.Generation)
	if errors.Is(err, flows.ErrDurableGenerationConflict) {
		flowsError(w, http.StatusConflict, "FLOW_DURABLE_GENERATION_CONFLICT", "the durable generation changed")
		return
	}
	if err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	flowsJSON(w, http.StatusOK, map[string]bool{"accepted": accepted})
}
