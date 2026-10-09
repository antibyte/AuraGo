package server

import (
	"context"
	"net/http"
	"time"

	"aurago/internal/desktop"
)

// Serial and Retro-Net capabilities are server-owned so changing a grant never
// recreates the desktop database/service or grants a generated app access to
// host devices or outbound dialing.
type desktopLivePolicy struct {
	Enabled                  bool `json:"enabled"`
	ReadOnly                 bool `json:"readonly"`
	SerialBrowserEnabled     bool `json:"serial_browser_enabled"`
	SerialHostEnabled        bool `json:"serial_host_enabled"`
	RetroNetEnabled          bool `json:"retronet_enabled"`
	RemoteMaxSessionMinutes  int  `json:"remote_max_session_minutes"`
	RemoteIdleTimeoutMinutes int  `json:"remote_idle_timeout_minutes"`
}

func (s *Server) desktopSerialPolicy(r *http.Request) desktopLivePolicy {
	policy := desktopLivePolicy{RemoteMaxSessionMinutes: 60, RemoteIdleTimeoutMinutes: 5}
	if s == nil {
		return policy
	}
	s.CfgMu.RLock()
	if s.Cfg != nil {
		cfg := s.Cfg.VirtualDesktop
		policy.Enabled, policy.ReadOnly = cfg.Enabled, cfg.ReadOnly
		policy.SerialBrowserEnabled = cfg.Enabled && !cfg.ReadOnly && cfg.SerialBrowserEnabled
		policy.SerialHostEnabled = cfg.Enabled && !cfg.ReadOnly && cfg.SerialHostEnabled
		policy.RetroNetEnabled = cfg.Enabled && !cfg.ReadOnly && cfg.RetroNetEnabled
		if cfg.RemoteMaxSessionMinutes > 0 {
			policy.RemoteMaxSessionMinutes = cfg.RemoteMaxSessionMinutes
		}
		if cfg.RemoteIdleTimeoutMinutes > 0 {
			policy.RemoteIdleTimeoutMinutes = cfg.RemoteIdleTimeoutMinutes
		}
	}
	s.CfgMu.RUnlock()
	if r != nil && !desktopRequestIsAdmin(s, r) {
		policy.SerialBrowserEnabled, policy.SerialHostEnabled = false, false
		if token, _ := bearerCredential(r.Header.Get("Authorization")); !desktopTokenHasScope(s, token, desktopScopeWrite) {
			policy.ReadOnly = true
			policy.RetroNetEnabled = false
		}
	}
	return policy
}

func registerDesktopSerialRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/serial/ports", withDesktopSerialGuard(s, false, desktop.HandleSerialPorts()))
	mux.HandleFunc("/api/desktop/serial/connect", withDesktopSerialGuard(s, true, func(w http.ResponseWriter, r *http.Request) {
		policy := s.desktopSerialPolicy(r)
		options := desktop.RemoteProxyOptions{
			MaxSessionDuration: time.Duration(policy.RemoteMaxSessionMinutes) * time.Minute,
			IdleTimeout:        time.Duration(policy.RemoteIdleTimeoutMinutes) * time.Minute,
		}
		desktop.HandleSerialProxy(options, func(request *http.Request) bool {
			return request.Context().Err() == nil && s.desktopSerialPolicy(request).SerialHostEnabled && desktopWSAuthorizationValid(s, request, desktopScopeAdmin)
		})(w, r)
	}))
}

func withDesktopSerialGuard(s *Server, connect bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		operation := desktopRead
		if connect {
			operation = desktopExecute
		}
		if !requireDesktopOperation(s, w, r, desktopScopeAdmin, operation) {
			return
		}
		s.CfgMu.RLock()
		enabled := s.Cfg != nil && s.Cfg.VirtualDesktop.Enabled && s.Cfg.VirtualDesktop.SerialHostEnabled
		s.CfgMu.RUnlock()
		if !enabled {
			writeDesktopPolicyError(w, "serial_disabled", "Host serial connections are disabled.")
			return
		}
		if !connect {
			next(w, r)
			return
		}
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		if s.integrationCtx != nil {
			stop := context.AfterFunc(s.integrationCtx, cancel)
			defer stop()
		}
		r = r.WithContext(ctx)
		// A disconnected device can leave a read blocked. Rechecking every frame
		// is not enough: revoke the owning context even when no bytes arrive.
		go func() {
			tick := time.NewTicker(time.Second)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					if !s.desktopSerialPolicy(r).SerialHostEnabled || !desktopWSAuthorizationValid(s, r, desktopScopeAdmin) {
						cancel()
						return
					}
				}
			}
		}()
		next(w, r)
	}
}
