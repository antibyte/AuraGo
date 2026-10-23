package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/desktopstore"
	"aurago/internal/tools"

	"github.com/gorilla/websocket"
)

// Requires an isolated, empty reviewed image. Creates test tokens, never printers.
func TestPrintGuardLiveCPU(t *testing.T) {
	origin := os.Getenv("AURAGO_PRINTGUARD_LIVE_URL")
	if origin == "" {
		t.Skip("requires an isolated PrintGuard test container")
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.Path != "" || u.RawQuery != "" {
		t.Fatal("live fixture must be an explicit loopback HTTP origin")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		t.Fatal("invalid fixture port")
	}
	s, store := printGuardServerFixture(t, port)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	code, state, err := s.printGuardState(ctx, "")
	var snapshot struct{ Printers, Cameras []json.RawMessage }
	if err != nil || code != http.StatusOK || json.Unmarshal(state, &snapshot) != nil || len(snapshot.Printers) != 0 || len(snapshot.Cameras) != 0 {
		t.Fatal("live test requires an empty hub without API tokens")
	}
	wsURL := "ws://" + net.JoinHostPort(u.Hostname(), u.Port()) + "/api/ws"
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, http.Header{"Origin": {origin}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	mint := func(scope string) string {
		t.Helper()
		if err := ws.WriteJSON(map[string]string{"cmd": "token.create", "name": "AuraGo isolated test", "scope": scope}); err != nil {
			t.Fatal(err)
		}
		for {
			var event struct{ Event, Token string }
			if err := ws.ReadJSON(&event); err != nil {
				t.Fatal(err)
			}
			if event.Event == "token_created" && strings.HasPrefix(event.Token, "pg_") {
				return event.Token
			}
			if event.Event == "error" {
				t.Fatal("fixture token creation refused")
			}
		}
	}
	previous, configured := tools.CurrentRuntimePermissionsForTest()
	tools.ConfigureRuntimePermissions(tools.RuntimePermissionsFromConfig(s.Cfg))
	t.Cleanup(func() {
		if configured {
			tools.ConfigureRuntimePermissions(previous)
		} else {
			tools.ClearRuntimePermissionsForTest()
		}
	})
	for _, scope := range []string{"read", "control"} {
		token := mint(scope)
		configurePrintGuardFixture(t, store, scope, token)
		if err := s.verifyPrintGuardConnection(ctx, store); err != nil {
			t.Fatalf("%s handshake: %v", scope, err)
		}
		runtime, _, err := printGuardBindingConfig(s.Cfg, s.Vault, true)
		if err != nil {
			t.Fatal(err)
		}
		mgr := tools.InitMCPManager([]tools.MCPServerConfig{*runtime}, s.Logger)
		for _, tool := range mgr.ListTools(desktopstore.PrintGuardMCPName) {
			if !slices.Contains(runtime.AllowedTools, tool.Name) {
				t.Fatalf("extra tool discovered: %s", tool.Name)
			}
		}
		if _, err := mgr.CallTool(ctx, desktopstore.PrintGuardMCPName, "get_state", nil); err != nil {
			t.Fatal(err)
		}
		if err := tools.SetManagedMCPServer(desktopstore.PrintGuardMCPName, nil); err != nil {
			t.Fatal(err)
		}
		mgr.Close()
	}
	t.Log("reviewed live CPU hub: anonymous API rejection, read/control discovery, allowlist and state read passed; no printer commands sent")
}
