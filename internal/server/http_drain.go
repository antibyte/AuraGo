package server

import (
	"context"
	"net/http"
)

// trackHTTP cancels active handlers on shutdown and prevents additions once
// draining begins. Dependencies stay open until every accepted handler returns.
func (s *Server) trackHTTP(next http.Handler) http.Handler {
	if next == nil {
		next = http.DefaultServeMux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.httpDrainMu.Lock()
		if s.httpDraining {
			s.httpDrainMu.Unlock()
			http.Error(w, "shutting down", 503)
			return
		}
		if s.httpDrainCtx == nil {
			s.httpDrainCtx, s.httpDrainCancel = context.WithCancel(context.Background())
		}
		ctx, cancel := context.WithCancel(r.Context())
		stop := context.AfterFunc(s.httpDrainCtx, cancel)
		s.httpRequests.Add(1)
		s.httpDrainMu.Unlock()
		defer s.httpRequests.Done()
		defer cancel()
		defer stop()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) beginHTTPDrain() {
	s.httpDrainMu.Lock()
	defer s.httpDrainMu.Unlock()
	s.httpDraining = true
	if s.httpDrainCancel != nil {
		s.httpDrainCancel()
	}
}
