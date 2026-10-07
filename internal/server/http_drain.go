package server

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// drainHTTPConn lets shutdown interrupt a WebSocket read. net/http.Shutdown
// does not own hijacked connections, and canceling a request context does not
// unblock a WebSocket reader waiting for the next frame.
type drainHTTPConn struct {
	net.Conn
	server *Server
	once   sync.Once
}

func (c *drainHTTPConn) Close() (err error) {
	c.once.Do(func() {
		err = c.Conn.Close()
		c.server.httpDrainMu.Lock()
		delete(c.server.httpHijacked, c)
		c.server.httpDrainMu.Unlock()
	})
	return err
}

type drainHTTPWriter struct {
	http.ResponseWriter
	server *Server
}

func (w *drainHTTPWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *drainHTTPWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffered, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err != nil {
		return nil, nil, err
	}
	tracked := &drainHTTPConn{Conn: conn, server: w.server}
	w.server.httpDrainMu.Lock()
	if w.server.httpDraining {
		w.server.httpDrainMu.Unlock()
		_ = tracked.Close()
		return nil, nil, net.ErrClosed
	}
	if w.server.httpHijacked == nil {
		w.server.httpHijacked = make(map[*drainHTTPConn]struct{})
	}
	w.server.httpHijacked[tracked] = struct{}{}
	w.server.httpDrainMu.Unlock()
	return tracked, buffered, nil
}

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
		if websocket.IsWebSocketUpgrade(r) {
			w = &drainHTTPWriter{ResponseWriter: w, server: s}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// setServerLifetimeCancel records the cancel function of the server context
// (serverCtx in Start). beginHTTPDrain calls it, so background lifetimes end
// when shutdown begins on either path: shutdownCh, or Serve ending on its own.
func (s *Server) setServerLifetimeCancel(cancel context.CancelFunc) {
	s.httpDrainMu.Lock()
	s.serverLifetimeCancel = cancel
	s.httpDrainMu.Unlock()
}

func (s *Server) beginHTTPDrain() {
	s.httpDrainMu.Lock()
	s.httpDraining = true
	if s.httpDrainCancel != nil {
		s.httpDrainCancel()
	}
	endLifetime := s.serverLifetimeCancel
	connections := make([]*drainHTTPConn, 0, len(s.httpHijacked))
	for conn := range s.httpHijacked {
		connections = append(connections, conn)
	}
	s.httpDrainMu.Unlock()
	// A handler can wait on the server lifetime instead of its request context
	// (a detached sidecar image pull); end it so httpRequests.Wait returns.
	if endLifetime != nil {
		endLifetime()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
}
