package server

import (
	"aurago/internal/fileutil"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Desktop operations are independent of token scopes: an administrator still
// observes readonly, while authenticated cleanup remains available after revocation.
type desktopOperation uint8

const (
	desktopRead desktopOperation = iota
	desktopWrite
	desktopExecute
	desktopStop
)

func desktopMethodOperation(method string) desktopOperation {
	if method == http.MethodGet || method == http.MethodHead {
		return desktopRead
	}
	return desktopWrite
}

func requireDesktopOperation(s *Server, w http.ResponseWriter, r *http.Request, scope string, operation desktopOperation) bool {
	if !authenticateDesktopPermission(s, w, r, scope) {
		return false
	}
	return checkDesktopOperation(s, w, r, operation)
}

func desktopRequestOperation(r *http.Request) desktopOperation {
	path := r.URL.Path
	if r.Method == http.MethodPost {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if (len(parts) == 5 && parts[0] == "api" && parts[1] == "game-maker" && parts[2] == "jobs" && parts[4] == "cancel") ||
			(len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "video-studio" && parts[3] == "jobs" && parts[5] == "cancel") ||
			(len(parts) == 6 && parts[0] == "api" && parts[1] == "desktop" && parts[2] == "personal-radio" && parts[3] == "stations" && (parts[5] == "stop" || parts[5] == "pause")) ||
			path == "/api/desktop/rtl-sdr/stop" {
			return desktopStop
		}
	}
	if r.Method == http.MethodDelete && path == "/api/desktop/rtl-sdr/scan" {
		return desktopStop
	}
	if r.Method == http.MethodPost && strings.HasPrefix(path, "/api/desktop/store/apps/") && strings.HasSuffix(path, "/stop") {
		return desktopStop
	}
	if r.Method == http.MethodGet && (strings.HasSuffix(path, "/terminal") || strings.HasSuffix(path, "/vnc") || strings.HasSuffix(path, "/console") || path == "/api/desktop/ssh" || path == "/api/desktop/retronet/connect") {
		return desktopExecute
	}
	if r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/virtual-computers/tasks/") {
		return desktopStop
	}
	switch r.URL.Path {
	case "/api/desktop/looper/stop", "/api/desktop/looper/pause":
		return desktopStop
	case "/api/desktop/ssh", "/api/desktop/vnc":
		return desktopExecute
	case "/api/desktop/embed-token":
		return desktopRead
	}
	return desktopMethodOperation(r.Method)
}

func checkDesktopOperation(s *Server, w http.ResponseWriter, r *http.Request, operation desktopOperation) bool {
	if operation == desktopRead || operation == desktopStop {
		return true
	}
	if grant, _ := r.Context().Value(desktopRunContextKey{}).(*desktopRunGrant); grant != nil && grant.server == s {
		if r.Context().Err() == nil {
			return true
		}
		writeDesktopPolicyError(w, "desktop_readonly", "The desktop action was revoked.")
		return false
	}
	ctx, _, err := s.beginDesktopRun(r.Context())
	if err != nil {
		writeDesktopPolicyError(w, err.Error(), "The desktop does not allow this action.")
		return false
	}
	*r = *r.WithContext(ctx)
	return true
}

type desktopRunContextKey struct{}
type desktopRunGrant struct {
	server *Server
	epoch  uint64
}
type desktopRunRegistry struct {
	mu    sync.Mutex
	epoch uint64
	next  uint64
	runs  map[uint64]context.CancelFunc
}

// Admission and config revocation share CfgMu -> registry.mu lock ordering.
// Request contexts end with the HTTP request. Background owners must call done.
func (s *Server) beginDesktopRun(parent context.Context) (context.Context, context.CancelFunc, error) {
	if s == nil {
		return nil, nil, errors.New("desktop_disabled")
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	if s.Cfg == nil {
		return nil, nil, errors.New("desktop_disabled")
	}
	if s.Cfg.VirtualDesktop.ReadOnly {
		return nil, nil, errors.New("desktop_readonly")
	}
	if err := parent.Err(); err != nil {
		return nil, nil, err
	}
	r := &s.desktopRuns
	r.mu.Lock()
	if r.runs == nil {
		r.runs = make(map[uint64]context.CancelFunc)
	}
	r.next++
	id := r.next
	ctx, cancel := context.WithCancel(parent)
	ctx = context.WithValue(ctx, desktopRunContextKey{}, &desktopRunGrant{server: s, epoch: r.epoch})
	grantContext := ctx
	ctx = fileutil.WithPublicationGate(ctx, func(commit func() error) error {
		return publishDesktopResult(grantContext, commit)
	})
	r.runs[id] = cancel
	r.mu.Unlock()
	context.AfterFunc(ctx, func() { r.mu.Lock(); delete(r.runs, id); r.mu.Unlock() })
	return ctx, cancel, nil
}

// Called while publishing a new configuration, without waiting for network I/O.
func (s *Server) revokeDesktopRuns() {
	r := &s.desktopRuns
	r.mu.Lock()
	r.epoch++
	runs := r.runs
	r.runs = make(map[uint64]context.CancelFunc)
	r.mu.Unlock()
	for _, cancel := range runs {
		cancel()
	}
}

func (s *Server) beginDesktopBackgroundRun(timeout time.Duration) (context.Context, context.CancelFunc, error) {
	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, timeoutCancel := context.WithTimeout(parent, timeout)
	ctx, runCancel, err := s.beginDesktopRun(ctx)
	if err != nil {
		timeoutCancel()
		return nil, nil, err
	}
	return ctx, func() { runCancel(); timeoutCancel() }, nil
}

// Use this only for bounded local publication, never network/provider work.
// Revocation waits for a publication already in progress and rejects late ones.
func publishDesktopResult(ctx context.Context, publish func() error) error {
	grant, _ := ctx.Value(desktopRunContextKey{}).(*desktopRunGrant)
	if grant == nil {
		if err := ctx.Err(); err != nil {
			return err
		}
		return publish()
	}
	r := &grant.server.desktopRuns
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.epoch != grant.epoch {
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return publish()
}

func writeDesktopPolicyError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "code": code, "message": message})
}
