package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/agent"
	"aurago/internal/config"
)

func bridgeSecurityFixture(t *testing.T, handle func(context.Context, agent.ToolCall) (string, bool)) (*Server, *agent.DispatchContext) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Auth.Enabled = true
	cfg.Auth.SessionSecret = "fixture-session-signing-secret-0123456789"
	cfg.Auth.PasswordHash = "configured-fixture-password-hash"
	cfg.Docker.Enabled = true
	cfg.Agent.ActionPolicy.Enabled = true
	cfg.Tools.PythonToolBridge.Enabled = true
	cfg.Tools.PythonToolBridge.AllowedTools = []string{"docker"}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := &Server{Cfg: cfg, Logger: logger}
	s.bindConfigAuthorization(cfg)
	dc := &agent.DispatchContext{Cfg: cfg, Logger: logger, SessionID: t.Name(), MessageSource: "web_chat", ExecutionHooks: &agent.ExecutionHooks{HandleTool: handle}}
	t.Cleanup(func() { agent.CancelActionApprovals(dc.SessionID) })
	return s, dc
}

func bridgeSecurityRequest(ctx context.Context, token, tool, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/internal/tool-bridge/"+tool, strings.NewReader(body)).WithContext(ctx)
	r.RemoteAddr = "127.0.0.1:42000"
	r.Header.Set("X-Internal-Token", token)
	return r
}

func issueBridgeSecurityToken(t *testing.T, ctx context.Context, s *Server, dc *agent.DispatchContext) string {
	t.Helper()
	_, token, release, err := s.issueToolBridgeGrant(ctx, dc)
	if err != nil {
		t.Fatalf("issue run capability: %v", err)
	}
	t.Cleanup(release)
	return token
}

func TestToolBridgeScopedTokenHasNoAdminOrOtherToolAuthority(t *testing.T) {
	calls := 0
	s, dc := bridgeSecurityFixture(t, func(context.Context, agent.ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	s.internalToken = "fixture-process-token-distinct-from-run-capability"
	s.Cfg.Tools.PythonToolBridge.AllowedTools = []string{"docker", "proxmox", "execute_python", "execute_shell", "execute_skill", "run_agent_skill_script", "run_tool", "invoke_tool"}
	dc.ToolScopeRestricted = true
	dc.AllowedTools = map[string]struct{}{"docker": {}, "execute_python": {}, "execute_shell": {}, "execute_skill": {}, "run_agent_skill_script": {}, "run_tool": {}, "invoke_tool": {}}
	token := issueBridgeSecurityToken(t, context.Background(), s, dc)
	bridge := authMiddleware(s, handleToolBridgeExecute(s))
	allowed := httptest.NewRecorder()
	bridge.ServeHTTP(allowed, bridgeSecurityRequest(context.Background(), token, "docker", `{"parameters":{"operation":"list"}}`))
	if allowed.Code != http.StatusOK || calls != 1 {
		t.Fatalf("allowed action failed: status=%d calls=%d body=%s", allowed.Code, calls, allowed.Body.String())
	}
	for _, tool := range []string{"proxmox", "execute_python", "execute_shell", "execute_skill", "run_agent_skill_script", "run_tool", "invoke_tool"} {
		rr := httptest.NewRecorder()
		bridge.ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, tool, `{"parameters":{"operation":"list"}}`))
		if rr.Code != http.StatusUnauthorized || calls != 1 {
			t.Errorf("out-of-scope %s authenticated: status=%d calls=%d", tool, rr.Code, calls)
		}
	}
	adminReached := false
	admin := authMiddleware(s, requireAdmin(s, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { adminReached = true })))
	for _, test := range []struct {
		followUp   string
		wantStatus int
		wantError  string
	}{
		{"", http.StatusUnauthorized, "unauthorized"},
		// Forged chat-control headers are rejected before ordinary web auth.
		{"true", http.StatusForbidden, "invalid_internal_chat_headers"},
	} {
		r := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(`{}`))
		r.RemoteAddr = "127.0.0.1:42000"
		r.Header.Set("X-Internal-Token", token)
		r.Header.Set("X-Internal-FollowUp", test.followUp)
		rr := httptest.NewRecorder()
		admin.ServeHTTP(rr, r)
		var response struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if rr.Code != test.wantStatus || response.Error != test.wantError || adminReached {
			t.Fatalf("broker token admin refusal: followup=%q status=%d error=%q handler_reached=%v", test.followUp, rr.Code, response.Error, adminReached)
		}
	}
	r := bridgeSecurityRequest(context.Background(), token, "docker", `{"parameters":{"operation":"list"}}`)
	r.RemoteAddr = "203.0.113.7:42000"
	rr := httptest.NewRecorder()
	bridge.ServeHTTP(rr, r)
	if rr.Code != http.StatusUnauthorized || calls != 1 {
		t.Fatal("run capability worked from a non-loopback address")
	}
}

func TestToolBridgeCannotChooseItsOriginatingSession(t *testing.T) {
	calls := 0
	s, dc := bridgeSecurityFixture(t, func(context.Context, agent.ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	token := issueBridgeSecurityToken(t, context.Background(), s, dc)
	handler := authMiddleware(s, handleToolBridgeExecute(s))
	for _, body := range []string{
		`{"session_id":"forged-origin","parameters":{"operation":"remove"}}`,
		`{"parameters":{"operation":"list"}} {"parameters":{"operation":"remove"}}`,
	} {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, "docker", body))
		if rr.Code != http.StatusBadRequest || calls != 0 {
			t.Fatalf("untrusted request shape accepted: status=%d calls=%d", rr.Code, calls)
		}
	}
	// An argument/header with a session-shaped name is ordinary tool data. It
	// cannot move a proposal into another session or turn a mission into a chat.
	r := bridgeSecurityRequest(context.Background(), token, "docker", `{"parameters":{"operation":"remove","container_id":"fixture-container","session_id":"forged-origin"}}`)
	r.Header.Set("X-Session-ID", "forged-origin")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)
	var response toolBridgeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	pending := agent.PendingActionApprovals(dc.SessionID)
	if rr.Code != http.StatusOK || response.Status != "error" || calls != 0 || len(pending) != 1 || pending[0].SessionID != dc.SessionID || len(agent.PendingActionApprovals("forged-origin")) != 0 {
		t.Fatalf("bridge lost trusted origin or executed unapproved action: status=%d calls=%d pending=%d", rr.Code, calls, len(pending))
	}
}

func TestToolBridgePublishedRevocationStopsExistingGrant(t *testing.T) {
	for _, mode := range []string{"bridge disabled", "tool removed", "integration disabled", "readonly", "explicit deny"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s, dc := bridgeSecurityFixture(t, func(context.Context, agent.ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
			// A policy allow cannot resurrect any of the revoked capabilities.
			s.Cfg.Agent.ActionPolicy.Allow = []config.ActionPolicyRule{{Tool: "docker", Operation: "remove", Target: "*"}}
			token := issueBridgeSecurityToken(t, context.Background(), s, dc)
			current := *s.Cfg
			switch mode {
			case "bridge disabled":
				current.Tools.PythonToolBridge.Enabled = false
			case "tool removed":
				current.Tools.PythonToolBridge.AllowedTools = nil
			case "integration disabled":
				current.Docker.Enabled = false
			case "readonly":
				current.Docker.ReadOnly = true
			case "explicit deny":
				current.Agent.ActionPolicy.Deny = []config.ActionPolicyRule{{Tool: "docker", Operation: "remove", Target: "*"}}
			}
			s.cfgSnapshot.Store(&current)
			rr := httptest.NewRecorder()
			authMiddleware(s, handleToolBridgeExecute(s)).ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, "docker", `{"parameters":{"operation":"remove","container_id":"fixture-container"}}`))
			var response toolBridgeResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Status != "error" || calls != 0 {
				t.Fatalf("revoked run executed: status=%d response=%+v calls=%d", rr.Code, response, calls)
			}
		})
	}
}

func TestToolBridgeQuotaAndReleaseInvalidateRunCapability(t *testing.T) {
	calls := 0
	s, dc := bridgeSecurityFixture(t, func(context.Context, agent.ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	_, token, release, err := s.issueToolBridgeGrant(context.Background(), dc)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	handler := authMiddleware(s, handleToolBridgeExecute(s))
	for i := 0; i < bridgeGrantCallLimit; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, "docker", `{"parameters":{"operation":"list"}}`))
		if rr.Code != http.StatusOK {
			t.Fatalf("call %d unexpectedly refused: %d", i+1, rr.Code)
		}
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, "docker", `{"parameters":{"operation":"list"}}`))
	if rr.Code != http.StatusUnauthorized || calls != bridgeGrantCallLimit {
		t.Fatalf("quota bypass: status=%d calls=%d", rr.Code, calls)
	}
	// A separate, unexhausted capability is revoked immediately and idempotently.
	_, token, release2, err := s.issueToolBridgeGrant(context.Background(), dc)
	if err != nil {
		t.Fatal(err)
	}
	release2()
	release2()
	if _, ok := s.bridgeGrant(bridgeSecurityRequest(context.Background(), token, "docker", `{}`), false); ok {
		t.Fatal("released capability remained usable")
	}
}

func TestToolBridgeParentAndRequestCancellationReachRunningDispatch(t *testing.T) {
	for _, mode := range []string{"parent", "request"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			var calls atomic.Int32
			s, dc := bridgeSecurityFixture(t, func(ctx context.Context, _ agent.ToolCall) (string, bool) {
				calls.Add(1)
				close(started)
				<-ctx.Done()
				return `{"status":"error","message":"cancelled fixture"}`, true
			})
			parentCtx, cancelParent := context.WithCancel(context.Background())
			defer cancelParent()
			reqCtx, cancelRequest := context.WithCancel(context.Background())
			defer cancelRequest()
			token := issueBridgeSecurityToken(t, parentCtx, s, dc)
			rr := httptest.NewRecorder()
			done := make(chan struct{})
			go func() {
				authMiddleware(s, handleToolBridgeExecute(s)).ServeHTTP(rr, bridgeSecurityRequest(reqCtx, token, "docker", `{"parameters":{"operation":"list"}}`))
				close(done)
			}()
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("bridge hook did not start")
			}
			if mode == "parent" {
				cancelParent()
			} else {
				cancelRequest()
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("bridge dispatch ignored cancellation")
			}
			var response toolBridgeResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Status != "error" || calls.Load() != 1 {
				t.Fatalf("cancelled dispatch reported success: %+v calls=%d", response, calls.Load())
			}
			if mode == "parent" {
				if _, ok := s.bridgeGrant(bridgeSecurityRequest(context.Background(), token, "docker", `{}`), false); ok {
					t.Fatal("parent cancellation left capability usable")
				}
			}
		})
	}
}

func TestToolBridgeSQLConnectionScopeCannotExpandAfterIssue(t *testing.T) {
	calls := 0
	s, dc := bridgeSecurityFixture(t, func(context.Context, agent.ToolCall) (string, bool) { calls++; return `{"status":"success"}`, true })
	s.Cfg.Tools.PythonToolBridge.AllowedTools = []string{"sql_query"}
	s.Cfg.Tools.PythonToolBridge.AllowedSQLConnections = []string{"fixture-original"}
	token := issueBridgeSecurityToken(t, context.Background(), s, dc)
	current := *s.Cfg
	current.Tools.PythonToolBridge.AllowedSQLConnections = []string{"fixture-original", "fixture-new"}
	s.cfgSnapshot.Store(&current)
	rr := httptest.NewRecorder()
	handleToolBridgeExecute(s).ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, "sql_query", `{"parameters":{"connection_name":"fixture-new","query":"SELECT 1"}}`))
	if rr.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("new SQL scope leaked into old run: status=%d calls=%d", rr.Code, calls)
	}
	current.Tools.PythonToolBridge.AllowedSQLConnections = nil
	rr = httptest.NewRecorder()
	handleToolBridgeExecute(s).ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, "sql_query", `{"parameters":{"connection_name":"fixture-original","query":"SELECT 1"}}`))
	if rr.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("revoked SQL scope retained by old run: status=%d calls=%d", rr.Code, calls)
	}
}

func TestToolBridgeNeverReportsUncertainOutcomeAsSuccess(t *testing.T) {
	for _, mode := range []string{"unknown", "panic"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			s, dc := bridgeSecurityFixture(t, func(context.Context, agent.ToolCall) (string, bool) {
				calls++
				if mode == "panic" {
					panic("fixture panic must not become success")
				}
				return `{"status":"outcome_unknown","code":"outcome_unknown"}`, true
			})
			token := issueBridgeSecurityToken(t, context.Background(), s, dc)
			rr := httptest.NewRecorder()
			handleToolBridgeExecute(s).ServeHTTP(rr, bridgeSecurityRequest(context.Background(), token, "docker", `{"parameters":{"operation":"list"}}`))
			var response toolBridgeResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Status != "error" || calls != 1 {
				t.Fatalf("uncertain bridge call was successful or replayed: status=%s calls=%d", response.Status, calls)
			}
		})
	}
}
