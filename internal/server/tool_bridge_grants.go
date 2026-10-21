package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/agent"
	"aurago/internal/security"
)

const bridgeGrantLifetime = 5 * time.Minute
const bridgeGrantCallLimit = 32

type toolBridgeGrant struct {
	ctx     context.Context
	cancel  context.CancelFunc
	dc      agent.DispatchContext
	tools   map[string]struct{}
	sql     map[string]struct{}
	expires time.Time
	calls   int
}

type toolBridgeGrantRegistry struct {
	sync.Mutex
	grants map[[32]byte]*toolBridgeGrant
}

func cloneBridgeScope(scope map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(scope))
	for key := range scope {
		result[key] = struct{}{}
	}
	return result
}

func (s *Server) issueToolBridgeGrant(ctx context.Context, dc *agent.DispatchContext) (string, string, func(), error) {
	if dc == nil || dc.Cfg == nil || dc.SessionID == "" || ctx.Err() != nil {
		return "", "", nil, fmt.Errorf("tool bridge requires an active originating run")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.Tools.PythonToolBridge.Enabled || !dc.Cfg.Tools.PythonToolBridge.Enabled {
		return "", "", nil, fmt.Errorf("tool bridge is disabled")
	}
	allowed := make(map[string]struct{})
	current := make(map[string]bool)
	for _, name := range cfg.Tools.PythonToolBridge.AllowedTools {
		current[name] = true
	}
	for _, name := range dc.Cfg.Tools.PythonToolBridge.AllowedTools {
		if !current[name] {
			continue
		}
		if dc.ToolScopeRestricted {
			if _, ok := dc.AllowedTools[name]; !ok {
				continue
			}
		}
		// Broker capabilities cannot recursively mint execution capabilities.
		switch name {
		case "execute_python", "execute_shell", "run_tool", "run_skill", "execute_skill", "execute_agent_skill", "run_agent_skill_script", "invoke_tool":
			continue
		}
		allowed[name] = struct{}{}
	}
	if len(allowed) == 0 {
		return "", "", nil, fmt.Errorf("tool bridge has no allowed actions in this run")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", nil, fmt.Errorf("generate bridge capability: %w", err)
	}
	token := hex.EncodeToString(raw[:])
	key := sha256.Sum256([]byte(token))
	expires := time.Now().Add(bridgeGrantLifetime)
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(expires) {
		expires = deadline
	}
	leaseCtx, cancel := context.WithDeadline(ctx, expires)
	releaseScrub := security.RegisterScopedSensitiveExact(token)
	closeLease := func() { cancel(); releaseScrub() }
	local := agent.ChildToolDispatchContext(dc)
	local.AllowedTools = cloneBridgeScope(allowed)
	local.ToolScopeRestricted = true
	local.AllowedAgentSkills = cloneBridgeScope(dc.AllowedAgentSkills)
	local.SkillScopeRestricted = true
	grant := &toolBridgeGrant{ctx: leaseCtx, cancel: closeLease, dc: local, tools: allowed, sql: make(map[string]struct{}), expires: expires}
	for _, name := range dc.Cfg.Tools.PythonToolBridge.AllowedSQLConnections {
		grant.sql[name] = struct{}{}
	}
	registry := &s.toolBridgeGrants
	registry.Lock()
	if registry.grants == nil {
		registry.grants = make(map[[32]byte]*toolBridgeGrant)
	}
	for hash, old := range registry.grants {
		if old.ctx.Err() != nil || time.Now().After(old.expires) {
			old.cancel()
			delete(registry.grants, hash)
		}
	}
	if len(registry.grants) >= 256 {
		registry.Unlock()
		closeLease()
		return "", "", nil, fmt.Errorf("tool bridge run capacity reached")
	}
	registry.grants[key] = grant
	registry.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() { closeLease(); registry.Lock(); delete(registry.grants, key); registry.Unlock() })
	}
	return agent.ToolBridgeEndpoint(cfg), token, release, nil
}

func (s *Server) bridgeGrant(r *http.Request, consume bool) (*toolBridgeGrant, bool) {
	if r == nil || r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/api/internal/tool-bridge/") {
		return nil, false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil, false
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, false
	}
	token := r.Header.Get("X-Internal-Token")
	if len(token) != 64 {
		return nil, false
	}
	key := sha256.Sum256([]byte(token))
	registry := &s.toolBridgeGrants
	registry.Lock()
	defer registry.Unlock()
	grant := registry.grants[key]
	if grant == nil {
		return nil, false
	}
	if grant.ctx.Err() != nil || !time.Now().Before(grant.expires) {
		grant.cancel()
		delete(registry.grants, key)
		return nil, false
	}
	if grant.calls >= bridgeGrantCallLimit {
		return nil, false
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/internal/tool-bridge/")
	if _, ok := grant.tools[name]; !ok {
		return nil, false
	}
	if consume {
		grant.calls++
	}
	return grant, true
}
