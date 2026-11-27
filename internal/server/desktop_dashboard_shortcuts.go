package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/homedash"
)

// dashboardShortcutTimeout bounds every HomeDash lookup made on behalf of the desktop.
const dashboardShortcutTimeout = 2 * time.Second

// liveDesktopService returns the running desktop service and hub without ever creating them.
// HomeDash hooks use it so a server without a desktop stays without one.
func (s *Server) liveDesktopService() (*desktop.Service, *desktop.Hub) {
	s.DesktopMu.Lock()
	defer s.DesktopMu.Unlock()
	if s.desktopClosed || s.DesktopService == nil {
		return nil, nil
	}
	return s.DesktopService, s.DesktopHub
}

// projectDashboardShortcuts fits the dashboard shortcuts of a bootstrap to the caller: live name,
// gone when the dashboard is gone, HomeDash is off or the token is not admin. It always builds a
// new slice because Bootstrap may hand out the cached one.
func (s *Server) projectDashboardShortcuts(r *http.Request, p *desktop.BootstrapPayload) {
	var ids []string
	for _, sc := range p.Shortcuts {
		if sc.TargetType == desktop.ShortcutTargetDashboard {
			ids = append(ids, sc.TargetID)
		}
	}
	if len(ids) == 0 {
		if p.Shortcuts == nil {
			p.Shortcuts = []desktop.Shortcut{}
		}
		return
	}
	var names map[string]string
	var storeErr error
	keep := desktopRequestIsAdmin(s, r) && homeDashEnabled(s.ConfigSnapshot())
	if keep {
		svc := s.homeDash.Load()
		if svc == nil {
			keep = false
		} else {
			ctx, cancel := context.WithTimeout(r.Context(), dashboardShortcutTimeout)
			names, storeErr = svc.store.DashboardNames(ctx, ids)
			cancel()
		}
	}
	out := make([]desktop.Shortcut, 0, len(p.Shortcuts))
	for _, sc := range p.Shortcuts {
		if sc.TargetType == desktop.ShortcutTargetDashboard {
			if !keep {
				continue
			}
			if storeErr == nil {
				name, ok := names[sc.TargetID]
				if !ok {
					continue
				}
				sc.Name = name
			}
		}
		out = append(out, sc)
	}
	p.Shortcuts = out
}

// syncDashboardShortcutName copies a dashboard name onto its desktop shortcut after a save.
func (s *Server) syncDashboardShortcutName(ctx context.Context, id, name string) {
	svc, hub := s.liveDesktopService()
	if svc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dashboardShortcutTimeout)
	defer cancel()
	changed, err := svc.RenameTargetShortcuts(ctx, desktop.ShortcutTargetDashboard, id, name, desktop.SourceSystem)
	if err != nil {
		s.logDashboardShortcutError("HomeDash: desktop shortcut rename failed", id, err)
		return
	}
	if changed {
		s.broadcastShortcutEvent(hub, "update_shortcut", id)
	}
}

// dropDashboardShortcut removes the desktop shortcut of a deleted dashboard.
func (s *Server) dropDashboardShortcut(ctx context.Context, id string) {
	svc, hub := s.liveDesktopService()
	if svc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dashboardShortcutTimeout)
	defer cancel()
	changed, err := svc.RemoveTargetShortcuts(ctx, desktop.ShortcutTargetDashboard, id, desktop.SourceSystem)
	if err != nil {
		s.logDashboardShortcutError("HomeDash: desktop shortcut removal failed", id, err)
		return
	}
	if changed {
		s.broadcastShortcutEvent(hub, "remove_shortcut", id)
	}
}

func (s *Server) broadcastShortcutEvent(hub *desktop.Hub, op, dashboardID string) {
	broadcastDesktopEvent(s, hub, desktop.Event{
		Type:      "desktop_changed",
		Payload:   map[string]interface{}{"operation": op, "shortcut_id": desktop.DashboardShortcutID(dashboardID)},
		CreatedAt: time.Now().UTC(),
	})
}

func (s *Server) logDashboardShortcutError(msg, id string, err error) {
	if s.Logger != nil {
		s.Logger.Warn(msg, "dashboard", id, "error", err)
	}
}

// addDashboardShortcut validates a dashboard and pins it. It returns the HTTP status for a failure.
func (s *Server) addDashboardShortcut(r *http.Request, svc *desktop.Service, id string) (desktop.Shortcut, int, error) {
	if !desktopRequestIsAdmin(s, r) {
		return desktop.Shortcut{}, http.StatusForbidden, errDesktopScopeRequired
	}
	if !homedash.ValidDashboardID(id) {
		return desktop.Shortcut{}, http.StatusBadRequest, errors.New("invalid dashboard id")
	}
	if !homeDashEnabled(s.ConfigSnapshot()) {
		return desktop.Shortcut{}, http.StatusServiceUnavailable, errors.New("HomeDash is disabled")
	}
	hd := s.homeDash.Load()
	if hd == nil {
		return desktop.Shortcut{}, http.StatusServiceUnavailable, errors.New("HomeDash storage is unavailable")
	}
	ctx, cancel := context.WithTimeout(r.Context(), dashboardShortcutTimeout)
	defer cancel()
	names, err := hd.store.DashboardNames(ctx, []string{id})
	if err != nil {
		return desktop.Shortcut{}, http.StatusInternalServerError, errors.New("could not read the dashboard")
	}
	name, ok := names[id]
	if !ok {
		return desktop.Shortcut{}, http.StatusNotFound, errors.New("dashboard not found")
	}
	sc, err := svc.AddDesktopDashboardShortcut(ctx, id, name, desktop.SourceUser)
	if err != nil {
		return desktop.Shortcut{}, http.StatusInternalServerError, err
	}
	// The dashboard may have been deleted meanwhile; its delete hook ran before the row existed.
	if again, err := hd.store.DashboardNames(ctx, []string{id}); err == nil {
		if _, still := again[id]; !still {
			_, _ = svc.RemoveTargetShortcuts(ctx, desktop.ShortcutTargetDashboard, id, desktop.SourceSystem)
			return desktop.Shortcut{}, http.StatusNotFound, errors.New("dashboard not found")
		}
	}
	return sc, http.StatusOK, nil
}

var errDesktopScopeRequired = errors.New("desktop_scope_required")

func writeDesktopScopeRequired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"desktop_scope_required","message":"Desktop API scope is required for this endpoint."}`))
}

// handleAddDashboardShortcut answers POST /api/desktop/shortcuts with a dashboard_id.
func (s *Server) handleAddDashboardShortcut(w http.ResponseWriter, r *http.Request, svc *desktop.Service, hub *desktop.Hub, id string) {
	sc, status, err := s.addDashboardShortcut(r, svc, id)
	if err != nil {
		if errors.Is(err, errDesktopScopeRequired) {
			writeDesktopScopeRequired(w)
			return
		}
		jsonError(w, err.Error(), status)
		return
	}
	s.broadcastShortcutEvent(hub, "add_shortcut", id)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "shortcut": sc})
}
