package server

import (
	"net/http"

	"aurago/internal/flows"
)

func (s *Server) flowScenarios(w http.ResponseWriter, r *http.Request, flowID string, rest []string) {
	ctx := r.Context()
	keys := flowDataKeys{s}
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			list, err := s.Flows.ListScenarios(ctx, flowID, keys)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"scenarios": list})
		case http.MethodPost:
			var body struct {
				BaseRevision    int            `json:"base_revision"`
				ExpectedVersion int            `json:"expected_version"`
				Scenario        flows.Scenario `json:"scenario"`
			}
			if !flowsDecode(w, r, &body, flowsTestDataBodyLimit, false) {
				return
			}
			creating := body.Scenario.ID == ""
			item, err := s.Flows.SaveScenario(ctx, flowID, body.BaseRevision, body.ExpectedVersion, body.Scenario, keys)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			status := http.StatusOK
			if creating {
				status = http.StatusCreated
			}
			s.flowsJSONScrubbed(w, status, map[string]any{"scenario": item})
		default:
			flowsMethodNotAllowed(w)
		}
		return
	}
	id := rest[0]
	switch r.Method {
	case http.MethodGet:
		item, err := s.Flows.GetScenario(ctx, flowID, id, keys)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"scenario": item})
	case http.MethodPatch:
		var body struct {
			Name            string `json:"name"`
			ExpectedVersion int    `json:"expected_version"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
			return
		}
		item, err := s.Flows.RenameScenario(ctx, flowID, id, body.ExpectedVersion, body.Name, keys)
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"scenario": item})
	case http.MethodDelete:
		var body struct {
			ExpectedVersion int `json:"expected_version"`
		}
		if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
			return
		}
		if err := s.Flows.DeleteScenario(ctx, flowID, id, body.ExpectedVersion, keys); err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		flowsJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		flowsMethodNotAllowed(w)
	}
}
