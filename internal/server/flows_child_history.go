package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"aurago/internal/flows"
)

// flowChildHistory reads one retained trace or its run-pinned document. The
// service verifies ownership and document hashes in one database snapshot.
func (s *Server) flowChildHistory(w http.ResponseWriter, r *http.Request, runID, rawSeq, invocationID string, document bool) {
	if r.Method != http.MethodGet {
		flowsMethodNotAllowed(w)
		return
	}
	seq, err := strconv.Atoi(rawSeq)
	if err != nil || !flows.ValidChildParentSeq(seq) || strconv.Itoa(seq) != rawSeq ||
		(document && !flows.ValidChildInvocationID(invocationID)) {
		flowsError(w, http.StatusNotFound, "FLOW_CHILD_HISTORY_NOT_FOUND", "child history not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var value any
	if !document {
		value, err = s.Flows.ChildTrace(ctx, runID, seq)
	} else {
		value, err = s.Flows.ChildTraceDocument(ctx, runID, seq, invocationID)
	}
	if errors.Is(err, flows.ErrChildHistoryNotFound) {
		flowsError(w, http.StatusNotFound, "FLOW_CHILD_HISTORY_NOT_FOUND", "child history not found")
		return
	}
	if err != nil {
		flowsError(w, http.StatusServiceUnavailable, "FLOW_CHILD_HISTORY_UNAVAILABLE", "child history is unavailable")
		return
	}
	data, err := flows.EncodeChildHistoryResponse(value, flowStoreScrubber())
	if err != nil || (!document && len(data) > flows.MaxChildTraceBytes) {
		flowsError(w, http.StatusServiceUnavailable, "FLOW_CHILD_HISTORY_UNAVAILABLE", "child history is unavailable")
		return
	}
	flowsChildWriteBody(w, data)
}

// The typed encoder protects structural identities; do not scrub its JSON again.
func (s *Server) flowsChildSimulationJSON(w http.ResponseWriter, value any) {
	data, err := flows.EncodeChildHistoryResponse(value, flowStoreScrubber())
	if err != nil {
		flowsWriteEncodeFailure(w)
		return
	}
	flowsChildWriteBody(w, data)
}

// Writing the encoder bytes unchanged keeps the 256 KiB trace limit exact.
func flowsChildWriteBody(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
