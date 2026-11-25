package server

import (
	"context"
	"net/http"

	"aurago/internal/homedash"
)

type homeDashCommandRequest struct {
	Entity      string         `json:"entity"`
	Command     string         `json:"command"`
	Args        map[string]any `json:"args"`
	RequestID   string         `json:"request_id"`
	DashboardID string         `json:"dashboard_id"`
	WidgetID    string         `json:"widget_id"`
	Confirmed   bool           `json:"confirmed"`
}

// command runs the M0 pipeline (validate, gates, confirmation, rate limits, request-id dedupe,
// in-flight control, Source.Command, audit) for the session principal. A read-only desktop never
// gets here (the route is desktopWrite). 202 means the source accepted; the state follows on the stream.
func (h *hdRequest) command() {
	var req homeDashCommandRequest
	if !h.decode(&req, homeDashSmallBody) {
		return
	}
	switch {
	case req.Entity == "" || req.Command == "":
		h.bad("entity and command are required")
		return
	case req.RequestID == "" || len(req.RequestID) > 64:
		h.bad("request_id must be 1 to 64 characters")
		return
	case req.DashboardID != "" && !homedash.ValidDashboardID(req.DashboardID):
		h.bad("dashboard_id must be a dashboard id")
		return
	case req.WidgetID != "" && !homedash.ValidWidgetID(req.WidgetID):
		h.bad("widget_id must be a widget id")
		return
	}
	if _, _, err := homedash.ParseEntityID(req.Entity); err != nil {
		h.bad("entity must be an entity id")
		return
	}
	if req.Args == nil {
		req.Args = map[string]any{}
	}
	ctx := context.WithValue(h.r.Context(), homeDashAuditKey{}, desktopAuditRequestInfo(h.s, h.r))
	res, err := h.svc.hub.Command(ctx, homedash.Command{
		Entity: homedash.EntityID(req.Entity), Name: homedash.Capability(req.Command), Args: req.Args,
		Principal: homeDashPrincipal(h.cfg, h.r), RequestID: req.RequestID, Confirmed: req.Confirmed,
		DashboardID: req.DashboardID, WidgetID: req.WidgetID,
	})
	if err != nil {
		h.fail(err)
		return
	}
	fields := map[string]any{"accepted": true, "superseded": res.Superseded}
	if res.Expected != nil {
		fields["expected"] = res.Expected
	}
	homeDashWriteOK(h.w, http.StatusAccepted, fields)
}
