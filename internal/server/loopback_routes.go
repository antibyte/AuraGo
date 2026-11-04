package server

import "net/http"

func (s *Server) loopbackTargetHandler(homepage, webUI http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.CfgMu.RLock()
		homepageOnly := s.Cfg.CloudflareTunnel.ExposeHomepage && !s.Cfg.CloudflareTunnel.ExposeWebUI
		s.CfgMu.RUnlock()
		if homepageOnly && !isValidInternalLoopbackToken(r, s.internalToken) {
			homepage.ServeHTTP(w, r)
			return
		}
		webUI.ServeHTTP(w, r)
	})
}
