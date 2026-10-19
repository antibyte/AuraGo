package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/tools"
)

// newDaemonToolBridge owns credentials for one process generation. A renewed
// lease keeps the original run snapshot; exhausted leases wait for their expiry.
func (s *Server) newDaemonToolBridge(parent context.Context, sessionID string, allowed []string) (*tools.DaemonBridgeRun, error) {
	if parent == nil || parent.Err() != nil || !strings.HasPrefix(sessionID, "daemon:") {
		return nil, fmt.Errorf("daemon owner is unavailable")
	}
	cfg := s.ConfigSnapshot().Clone()
	if cfg == nil {
		return nil, fmt.Errorf("daemon configuration is unavailable")
	}
	s.bindConfigAuthorization(cfg)
	ctx, cancel := context.WithCancel(parent)
	stopShutdown := func() bool { return false }
	if s.integrationCtx != nil {
		stopShutdown = context.AfterFunc(s.integrationCtx, cancel)
	}
	releaseRun := func() {}
	if s.ShortTermMem != nil {
		var err error
		ctx, releaseRun, err = s.ShortTermMem.RegisterSessionRun(ctx, sessionID)
		if err != nil {
			cancel()
			stopShutdown()
			return nil, err
		}
	}
	scope := make(map[string]struct{}, len(allowed))
	for _, name := range allowed {
		scope[name] = struct{}{}
	}
	dc := &agent.DispatchContext{
		Cfg: cfg, Logger: flowDispatchLogger(s.Logger), LLMClient: s.LLMClient, Vault: s.Vault, Registry: s.Registry,
		Manifest: tools.NewManifest(cfg.Directories.ToolsDir), CronManager: s.CronManager, MissionManagerV2: s.MissionManagerV2, WebhookManager: s.WebhookManager,
		LongTermMem: s.LongTermMem, ShortTermMem: s.ShortTermMem, KG: s.KG, InventoryDB: s.InventoryDB, InvasionDB: s.InvasionDB,
		CheatsheetDB: s.CheatsheetDB, ImageGalleryDB: s.ImageGalleryDB, MediaRegistryDB: s.MediaRegistryDB, HomepageRegistryDB: s.HomepageRegistryDB,
		ContactsDB: s.ContactsDB, PlannerDB: s.PlannerDB, SQLConnectionsDB: s.SQLConnectionsDB, SQLConnectionPool: s.SQLConnectionPool,
		RemoteHub: s.RemoteHub, HistoryMgr: s.HistoryManager, Guardian: s.Guardian, LLMGuardian: s.LLMGuardian,
		SessionID: sessionID, MessageSource: "background", CoAgentRegistry: s.CoAgentRegistry, BudgetTracker: s.BudgetTracker,
		DaemonSupervisor: s.DaemonSupervisor, PreparationService: s.PreparationService, WorkspaceSearch: s.WorkspaceSearch,
		ToolScopeRestricted: true, AllowedTools: scope, SkillScopeRestricted: true, AllowedAgentSkills: map[string]struct{}{},
	}
	return s.daemonToolBridgeRun(ctx, cancel, stopShutdown, releaseRun, dc), nil
}

func (s *Server) daemonToolBridgeRun(ctx context.Context, cancel context.CancelFunc, stopShutdown func() bool, releaseRun func(), dc *agent.DispatchContext, clocks ...func() time.Time) *tools.DaemonBridgeRun {
	now := time.Now
	if len(clocks) > 0 {
		now = clocks[0]
	}
	var mu sync.Mutex
	var token string
	var expires time.Time
	var releaseLease func()
	gate := make(chan struct{}, 1)
	var once sync.Once
	closeRun := func() {
		once.Do(func() {
			cancel()
			stopShutdown()
			releaseRun()
			mu.Lock()
			defer mu.Unlock()
			if releaseLease != nil {
				releaseLease()
				releaseLease = nil
			}
		})
	}
	call := func(owner context.Context, request tools.DaemonToolRequest) tools.DaemonToolResponse {
		response := tools.DaemonToolResponse{ID: request.ID, Status: "error", Result: "Daemon tool capability unavailable, expired or rate limited"}
		select {
		case <-ctx.Done():
			return response
		case <-owner.Done():
			return response
		case gate <- struct{}{}:
		}
		defer func() { <-gate }()
		if ctx.Err() != nil || owner.Err() != nil || !toolBridgeNameRegex.MatchString(request.Tool) {
			return response
		}
		mu.Lock()
		if !now().Before(expires) {
			if releaseLease != nil {
				releaseLease()
				releaseLease = nil
			}
			_, issued, release, err := s.issueToolBridgeGrant(ctx, dc)
			if err != nil {
				mu.Unlock()
				return response
			}
			token, releaseLease, expires = issued, release, now().Add(bridgeGrantLifetime)
		}
		capability := token
		mu.Unlock()
		body, err := json.Marshal(toolBridgeRequest{Parameters: request.Parameters, Timeout: min(60, max(1, request.Timeout))})
		if err != nil || len(body) > 64<<10 {
			return response
		}
		r, err := http.NewRequestWithContext(owner, http.MethodPost, "http://127.0.0.1/api/internal/tool-bridge/"+request.Tool, bytes.NewReader(body))
		if err != nil {
			return response
		}
		r.RemoteAddr = "127.0.0.1:0"
		r.Header.Set("X-Internal-Token", capability)
		grant, ok := s.bridgeGrant(r, true)
		if !ok {
			return response
		} // Never refill an exhausted lease early.
		output := &daemonBridgeResponseWriter{header: make(http.Header)}
		s.serveToolBridgeGrant(output, r, grant)
		var result toolBridgeResponse
		if output.overflow || json.Unmarshal(output.body.Bytes(), &result) != nil {
			response.Result = "Daemon tool response exceeded its limit or was invalid"
			return response
		}
		response.Status, response.Result = result.Status, result.Result
		return response
	}
	return &tools.DaemonBridgeRun{Owner: ctx, Call: call, Release: closeRun}
}

type daemonBridgeResponseWriter struct {
	header   http.Header
	body     bytes.Buffer
	overflow bool
}

func (w *daemonBridgeResponseWriter) Header() http.Header { return w.header }
func (w *daemonBridgeResponseWriter) WriteHeader(int)     {}
func (w *daemonBridgeResponseWriter) Write(data []byte) (int, error) {
	if len(data)+w.body.Len() > 1<<20 {
		w.overflow = true
		return 0, fmt.Errorf("daemon response exceeds limit")
	}
	return w.body.Write(data)
}
