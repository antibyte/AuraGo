package server

import (
	"aurago/internal/config"
	"aurago/internal/localwiki"
)

// dashboardLocalWikipediaStatusSource returns the manager's status reader, or nil when the
// server has no Local Wikipedia manager.
func (s *Server) dashboardLocalWikipediaStatusSource() func() localwiki.Status {
	if s == nil || s.LocalWiki == nil {
		return nil
	}
	return s.LocalWiki.Status
}

// dashboardLocalWikipediaSummary feeds the dashboard badge and its update hint. It reads only
// the manager's in-memory status and never contacts the Kiwix catalog.
//
// readable and loading are the manager's own flags (an edition is open and served; the first
// background load is still running): clients must not derive them from state. error_code is
// the stable code only; the English recommendation stays out so the UI can localize the hint.
func dashboardLocalWikipediaSummary(cfg *config.Config, status func() localwiki.Status) map[string]interface{} {
	enabled := cfg != nil && cfg.LocalWikipedia.Enabled
	summary := map[string]interface{}{"enabled": enabled}
	if !enabled || status == nil {
		return summary
	}
	st := status()
	summary["state"] = st.State
	summary["readable"] = st.Readable
	summary["loading"] = st.Loading
	if st.ErrorCode != "" {
		summary["error_code"] = st.ErrorCode
	}
	if st.Edition != nil {
		summary["edition"] = map[string]interface{}{
			"language": st.Edition.Language,
			"variant":  string(st.Edition.Variant),
			"date":     st.Edition.Date,
		}
	}
	if st.UpdateAvailable != nil {
		summary["update_available"] = map[string]interface{}{
			"date": st.UpdateAvailable.Date,
			"size": st.UpdateAvailable.Size,
		}
	}
	return summary
}
