package server

import (
	"aurago/internal/config"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPEmptyCatalogWireArray(t *testing.T) {
	for _, bridge := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.MCPServer.Enabled = true
		cfg.MCPServer.VSCodeDebugBridge = bridge
		s := &Server{Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
		// Check the wire representation without enabling bridge authentication.
		cfg.MCPServer.VSCodeDebugBridge = false
		w := httptest.NewRecorder()
		handleMCPEndpoint(s)(w, httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)))
		if !strings.Contains(w.Body.String(), `"tools":[]`) {
			t.Fatalf("unexpected response: %s", w.Body.String())
		}
		cfg.MCPServer.VSCodeDebugBridge = bridge
		if got := mcpBuildToolList(s); got == nil || len(got) != 0 {
			t.Fatalf("empty scope: %#v", got)
		}
	}
}
