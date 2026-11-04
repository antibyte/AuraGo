package server

import (
	"context"
	"net/http"
)

func (s *Server) flowRepairMission(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		flowsMethodNotAllowed(w)
		return
	}
	var body struct {
		BaseRevision     *int `json:"base_revision"`
		BaseLiveRevision *int `json:"base_live_revision"`
	}
	if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
		return
	}
	if body.BaseRevision == nil || *body.BaseRevision <= 0 || body.BaseLiveRevision == nil || *body.BaseLiveRevision < 0 {
		flowsError(w, http.StatusBadRequest, "FLOW_BAD_REQUEST", "draft and live revisions are required")
		return
	}
	if err := s.Flows.RepairMission(r.Context(), id, *body.BaseRevision, *body.BaseLiveRevision); err != nil {
		s.flowsErrorFrom(w, r, err)
		return
	}
	name := id
	if rec, err := s.Flows.GetFlow(context.WithoutCancel(r.Context()), id); err == nil {
		name = rec.Name
	}
	s.recordFlowAudit("flow_mission_repair", id, name, "Flow "+name+" Mission Control connection repaired")
	s.broadcastFlowsChanged(id, "repaired")
	flowsJSON(w, http.StatusOK, map[string]any{"status": "repaired"})
}
