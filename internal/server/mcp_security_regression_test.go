package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/agent"
	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestMCPEndpointRejectsRebindingHostEvenWithMatchingOrigin(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCPServer.Enabled = true
	cfg.Server.HTTPS.Domain = "aurago.example"
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, tc := range []struct {
		host, origin string
		want         int
	}{
		{"attacker.example", "http://attacker.example", 403},
		{"attacker.example", "", 403},
		{"localhost.attacker.example", "http://localhost.attacker.example", 403},
		{"0.0.0.0", "", 403},
		{"aurago.example", "http://attacker.example", 403},
		{"aurago.example", "http://aurago.example", 200},
		{"127.0.0.1:8088", "", 200},
		{"[::1]:8088", "", 200},
	} {
		t.Run(tc.host+"/"+tc.origin, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			handleMCPEndpoint(s)(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d", w.Code, tc.want)
			}
		})
	}
}

func TestMCPSessionsSeparateClientsAndBindPrincipal(t *testing.T) {
	var signer mcpSessionSigner
	request := func(credential string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", nil)
		r.Header.Set("Authorization", credential)
		return r
	}
	clientA := request("Bearer client-a-fixture")
	idA, err := signer.session(clientA, true)
	if err != nil {
		t.Fatal(err)
	}
	idB, err := signer.session(clientA, true)
	if err != nil || idA == idB {
		t.Fatal("two clients shared a session")
	}
	clientA.Header.Set("Mcp-Session-Id", idA)
	if id, err := signer.session(clientA, false); err != nil || id != idA {
		t.Fatal("client cannot resume its own session")
	}
	clientB := request("Bearer client-b-fixture")
	clientB.Header.Set("Mcp-Session-Id", idA)
	if _, err := signer.session(clientB, false); err == nil {
		t.Fatal("another credential reused the session")
	}
	clientA.Header.Set("Mcp-Session-Id", idA+"tampered")
	if _, err := signer.session(clientA, false); err == nil {
		t.Fatal("tampered session accepted")
	}
	expired := strings.Repeat("a", 64) + ".1"
	clientA.Header.Set("Mcp-Session-Id", expired+"."+signer.sign(expired, mcpSessionPrincipal(clientA)))
	if _, err := signer.session(clientA, false); err == nil {
		t.Fatal("expired session accepted")
	}
	ctxA := context.WithValue(context.Background(), mcpSessionContextKey{}, idA)
	ctxB := context.WithValue(context.Background(), mcpSessionContextKey{}, idB)
	if mcpDispatchSessionID(ctxA) == mcpDispatchSessionID(ctxB) || mcpDispatchSessionID(context.Background()) == mcpDispatchSessionID(context.Background()) {
		t.Fatal("independent clients share agent history")
	}
}

func TestMCPRuntimeTestTargetsOnlySelectedServerAndBindsAliases(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowMCP, cfg.MCP.Enabled = true, true
	cfg.Dograh.Enabled, cfg.Dograh.MCPClientEnabled = true, true
	cfg.Dograh.APIURL, cfg.Dograh.APIKey = "http://127.0.0.1:8000", "fixture"
	saved := config.MCPServer{Name: "selected", Transport: "streamable_http", URL: "https://approved.example/mcp", Headers: map[string]string{"Authorization": "Bearer {{approved}}"}, Enabled: true}
	cfg.MCP.Servers = []config.MCPServer{saved}
	s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	old := testExternalMCPServer
	t.Cleanup(func() { testExternalMCPServer = old })
	calls := 0
	testExternalMCPServer = func(ctx context.Context, got tools.MCPServerConfig, _ *slog.Logger) (tools.MCPConnectionTestResult, error) {
		calls++
		if got.Name != saved.Name || got.URL != saved.URL {
			t.Fatal("test used unselected server")
		}
		if got.ExecutionPermissions == nil || !got.MCPEnabled {
			t.Fatal("test lost runtime grants")
		}
		return tools.MCPConnectionTestResult{Status: "ok"}, nil
	}
	check := func(candidate config.MCPServer, want int) {
		t.Helper()
		body, _ := json.Marshal(candidate)
		w := httptest.NewRecorder()
		handleMCPRuntimeTestConnection(s)(w, httptest.NewRequest(http.MethodPost, "/api/mcp-runtime/test-connection", strings.NewReader(string(body))))
		if w.Code != want {
			t.Fatalf("status=%d want=%d body=%s", w.Code, want, w.Body.String())
		}
	}
	check(saved, 200)
	changed := saved
	changed.URL = "https://attacker.example/mcp"
	check(changed, 400)
	changed = saved
	changed.Transport, changed.Command = "stdio", "attacker-command"
	check(changed, 400)
	cfg.Agent.AllowMCP = false
	check(saved, 403)
	if calls != 1 {
		t.Fatalf("unexpected test launch count %d", calls)
	}
}

func TestMCPManagedDograhPrivateGrantIsOriginBound(t *testing.T) {
	for _, tc := range []struct {
		mode, target string
		docker, want bool
	}{
		{"managed", "http://127.0.0.1:8000", false, true},
		{"managed", "http://dograh-api:8000", true, true},
		{"external", "http://127.0.0.1:8000", false, false},
		{"managed", "http://127.0.0.1:8088", false, false},
		{"managed", "http://10.0.0.5:8000", false, false},
		{"managed", "http://127.0.0.1:8000/other", false, false},
		{"managed", "http://127.0.0.1.attacker.example:8000", false, false},
	} {
		cfg := &config.Config{}
		cfg.Dograh.Mode, cfg.Dograh.APIURL = tc.mode, tc.target
		cfg.Runtime.IsDocker = tc.docker
		if got := managedDograhMCPOrigin(cfg); got != tc.want {
			t.Fatalf("mode=%s target=%s: got=%v want=%v", tc.mode, tc.target, got, tc.want)
		}
	}
}

func TestMCPDebugPresetCannotWidenExplicitAllowlist(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCPServer.VSCodeDebugBridge = true
	cfg.MCPServer.AllowedTools = []string{"ask_aurago"}
	if got := mcpEffectiveAllowedTools(cfg); len(got) != 1 || got[0] != "ask_aurago" {
		t.Fatalf("preset widened scope: %v", got)
	}
}

func TestMCPScopedRunRejectsRevokedServerAndAllowlist(t *testing.T) {
	for _, revoke := range []string{"server", "tool"} {
		initial := &config.Config{}
		initial.MCPServer.Enabled = true
		initial.MCPServer.AllowedTools = []string{"query_memory"}
		s := &Server{Cfg: initial}
		run := mcpScopedConfig(s, initial)
		current := *initial
		if revoke == "server" {
			current.MCPServer.Enabled = false
		} else {
			current.MCPServer.AllowedTools = []string{}
		}
		s.Cfg = &current
		result := agent.DispatchToolCallResult(context.Background(), &agent.ToolCall{Action: "query_memory"}, &agent.DispatchContext{Cfg: run}, "")
		if !result.IsError || !strings.Contains(result.Output, "authorization_changed") {
			t.Fatalf("revocation=%s result=%+v", revoke, result)
		}
	}
}
