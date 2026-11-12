package server

import (
	"net/http"
)

// flowsCredentials serves /api/desktop/flows/credentials/...: the scan and (B.9) the purge. Every
// method needs the desktop admin scope (flowsRequiredScope). No answer carries a credential value.
func (s *Server) flowsCredentials(w http.ResponseWriter, r *http.Request, rest []string) {
	switch {
	case len(rest) == 1 && rest[0] == "scan":
		if r.Method != http.MethodGet {
			flowsMethodNotAllowed(w)
			return
		}
		scan := s.Flows.CredentialScan
		if r.URL.Query().Get("fresh") == "1" {
			scan = s.Flows.CredentialScanFresh
		}
		report, err := scan(r.Context())
		if err != nil {
			s.flowsErrorFrom(w, r, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		flowsJSON(w, http.StatusOK, map[string]any{"report": report})
	case len(rest) == 1 && rest[0] == "purge":
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Cache-Control", "no-store")
			flowsJSON(w, http.StatusOK, map[string]any{"state": s.Flows.CredentialPurgeState()})
		case http.MethodPost:
			var body struct {
				Snapshots []string `json:"snapshots"`
			}
			if !flowsDecode(w, r, &body, flowsSmallBodyLimit, false) {
				return
			}
			state, err := s.Flows.StartCredentialPurge(body.Snapshots)
			if err != nil {
				s.flowsErrorFrom(w, r, err)
				return
			}
			s.recordFlowAudit("flow_credentials_purge", "", "", "Credential cleanup started")
			flowsJSON(w, http.StatusAccepted, map[string]any{"state": state})
		default:
			flowsMethodNotAllowed(w)
		}
	default:
		flowsError(w, http.StatusNotFound, "FLOW_NOT_FOUND", "unknown flow route")
	}
}
