package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/security"

	"github.com/gorilla/websocket"
)

func TestMCPStreamableHTTPTransportInitializeListAndCall(t *testing.T) {
	var (
		mu           sync.Mutex
		seenAuth     bool
		seenSession  bool
		seenProtocol bool
		seenDelete   bool
		sessionValue = "session-123"
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			mu.Lock()
			seenDelete = r.Header.Get("Mcp-Session-Id") == sessionValue
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") == "Bearer dummy-token-value" {
			mu.Lock()
			seenAuth = true
			mu.Unlock()
		}
		if r.Header.Get("Mcp-Session-Id") == sessionValue {
			mu.Lock()
			seenSession = true
			mu.Unlock()
		}
		if r.Header.Get("MCP-Protocol-Version") == mcpProtocolVersionCurrent {
			mu.Lock()
			seenProtocol = true
			mu.Unlock()
		}
		w.Header().Set("Mcp-Session-Id", sessionValue)

		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if req.ID == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeMCPTestResponse(t, w, req)
	}))
	defer srv.Close()

	conn, err := startMCPServerConnection(context.Background(), MCPServerConfig{
		Name:      "remote-http",
		Enabled:   true,
		Transport: "streamable_http",
		URL:       srv.URL,
		Headers:   map[string]string{"Authorization": "Bearer dummy-token-value"},
	}, testLogger())
	if err != nil {
		t.Fatalf("startMCPServerConnection: %v", err)
	}
	got, err := conn.callTool(context.Background(), "echo", map[string]interface{}{"text": "hello"})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if got != "called echo" {
		t.Fatalf("callTool = %q, want called echo", got)
	}
	conn.close()

	mu.Lock()
	defer mu.Unlock()
	if !seenAuth {
		t.Fatal("Authorization header was not sent")
	}
	if !seenSession {
		t.Fatal("Mcp-Session-Id response header was not preserved for later requests")
	}
	if !seenProtocol {
		t.Fatal("negotiated MCP-Protocol-Version header was not sent")
	}
	if !seenDelete {
		t.Fatal("streamable HTTP session was not terminated with DELETE")
	}
}

func TestMCPWebSocketTransportInitializeListAndCall(t *testing.T) {
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req jsonRPCRequest
			if err := json.Unmarshal(data, &req); err != nil {
				t.Errorf("decode websocket request: %v", err)
				return
			}
			if req.ID == 0 {
				continue
			}
			resp := mcpTestResponseFor(req)
			if err := conn.WriteJSON(resp); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, err := startMCPServerConnection(context.Background(), MCPServerConfig{
		Name:      "remote-ws",
		Enabled:   true,
		Transport: "websocket",
		URL:       wsURL,
	}, testLogger())
	if err != nil {
		t.Fatalf("startMCPServerConnection: %v", err)
	}
	defer conn.close()

	got, err := conn.callTool(context.Background(), "echo", map[string]interface{}{})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if got != "called echo" {
		t.Fatalf("callTool = %q, want called echo", got)
	}
}

func TestMCPSSETransportInitializeListAndCall(t *testing.T) {
	type postedRequest struct {
		req jsonRPCRequest
	}
	events := make(chan jsonRPCResponse, 8)
	posted := make(chan postedRequest, 8)

	var baseURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sse":
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Error("response writer cannot flush")
				return
			}
			fmt.Fprintf(w, "event: endpoint\ndata: %s/messages\n\n", baseURL)
			flusher.Flush()
			for event := range events {
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
				flusher.Flush()
			}
		case r.URL.Path == "/messages":
			var req jsonRPCRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode message request: %v", err)
				return
			}
			posted <- postedRequest{req: req}
			if req.ID != 0 {
				events <- mcpTestResponseFor(req)
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	defer close(events)
	baseURL = srv.URL

	setupCtx, cancelSetup := context.WithCancel(context.Background())
	conn, err := startMCPServerConnection(setupCtx, MCPServerConfig{
		Name:      "remote-sse",
		Enabled:   true,
		Transport: "sse",
		URL:       srv.URL + "/sse",
	}, testLogger())
	if err != nil {
		t.Fatalf("startMCPServerConnection: %v", err)
	}
	defer conn.close()
	cancelSetup()

	got, err := conn.callTool(context.Background(), "echo", map[string]interface{}{})
	if err != nil {
		t.Fatalf("callTool: %v", err)
	}
	if got != "called echo" {
		t.Fatalf("callTool = %q, want called echo", got)
	}

	select {
	case <-posted:
	case <-time.After(time.Second):
		t.Fatal("SSE transport did not POST requests to endpoint event URL")
	}
}

func TestMCPNetworkTransportRequiresURL(t *testing.T) {
	_, err := startMCPServerConnection(context.Background(), MCPServerConfig{
		Name:      "missing-url",
		Enabled:   true,
		Transport: "streamable_http",
	}, testLogger())
	if err == nil || !strings.Contains(err.Error(), "url is required") {
		t.Fatalf("error = %v, want url is required", err)
	}
}

func TestMCPNetworkHeaderSecretTemplates(t *testing.T) {
	endpoint, headers, err := resolveMCPNetworkURLAndHeaders(MCPServerConfig{
		URL:     "https://example.com/{{tenant}}/mcp",
		Headers: map[string]string{"Authorization": "Bearer {{api-token}}"},
		Secrets: map[string]string{
			"tenant":    "acme",
			"api-token": "dummy-token-value",
		},
	})
	if err != nil {
		t.Fatalf("resolveMCPNetworkURLAndHeaders: %v", err)
	}
	if endpoint != "https://example.com/acme/mcp" {
		t.Fatalf("endpoint = %q", endpoint)
	}
	if headers["Authorization"] != "Bearer dummy-token-value" {
		t.Fatalf("Authorization header = %q", headers["Authorization"])
	}
	if redacted := security.Scrub(endpoint + " " + headers["Authorization"]); strings.Contains(redacted, "dummy-token-value") {
		t.Fatalf("resolved network secret was not scrubbed: %q", redacted)
	}

	_, _, err = resolveMCPNetworkURLAndHeaders(MCPServerConfig{
		URL:     "https://example.com/mcp",
		Headers: map[string]string{"Authorization": "Bearer {{missing}}"},
	})
	if err == nil || !strings.Contains(err.Error(), "unresolved placeholder") {
		t.Fatalf("error = %v, want unresolved placeholder", err)
	}
}

func TestHTTPMCPTransportRejectsMissingAndMismatchedResponseIDs(t *testing.T) {
	tests := []struct {
		name string
		id   *int64
		want string
	}{
		{name: "missing", id: nil, want: "did not include request id"},
		{name: "mismatch", id: func() *int64 { id := int64(999); return &id }(), want: "did not match request id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(jsonRPCResponse{JSONRPC: "2.0", ID: tt.id, Result: json.RawMessage(`{}`)})
			}))
			defer srv.Close()
			transport, err := newHTTPMCPTransport(srv.URL, nil)
			if err != nil {
				t.Fatalf("newHTTPMCPTransport: %v", err)
			}
			defer transport.Close()
			if _, err := transport.Send(context.Background(), "ping", nil); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Send error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestHTTPMCPTransportAcceptsStreamableSSEResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\n")
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":%d,\"result\":{}}\n\n", req.ID)
	}))
	defer srv.Close()
	transport, err := newHTTPMCPTransport(srv.URL, nil)
	if err != nil {
		t.Fatalf("newHTTPMCPTransport: %v", err)
	}
	defer transport.Close()
	if _, err := transport.Send(context.Background(), "ping", nil); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestHTTPMCPTransportAllowsConcurrentRequests(t *testing.T) {
	var started atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if started.Add(1) == 2 {
			close(release)
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jsonRPCResponse{JSONRPC: "2.0", ID: &req.ID, Result: json.RawMessage(`{}`)})
	}))
	defer srv.Close()
	transport, err := newHTTPMCPTransport(srv.URL, nil)
	if err != nil {
		t.Fatalf("newHTTPMCPTransport: %v", err)
	}
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, callErr := transport.Send(ctx, "ping", nil)
			errs <- callErr
		}()
	}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent Send: %v", err)
		}
	}
}

func TestHTTPMCPTransportDoesNotForwardSecretsAcrossRedirects(t *testing.T) {
	var redirected atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirected.Store(true)
		if got := r.Header.Get("X-MCP-Secret"); got != "" {
			t.Errorf("redirected secret header = %q", got)
		}
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	transport, err := newHTTPMCPTransport(source.URL, map[string]string{"X-MCP-Secret": "dummy-secret"})
	if err != nil {
		t.Fatalf("newHTTPMCPTransport: %v", err)
	}
	defer transport.Close()
	if _, err := transport.Send(context.Background(), "ping", nil); err == nil || !strings.Contains(err.Error(), "status 307") {
		t.Fatalf("Send error = %v, want redirect status error", err)
	}
	if redirected.Load() {
		t.Fatal("redirect target was contacted")
	}
}

func TestMCPClientRejectsUnsupportedNegotiatedVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		result, _ := json.Marshal(map[string]interface{}{
			"protocolVersion": "2099-01-01",
			"serverInfo":      map[string]string{"name": "future", "version": "1"},
			"capabilities":    map[string]interface{}{},
		})
		_ = json.NewEncoder(w).Encode(jsonRPCResponse{JSONRPC: "2.0", ID: &req.ID, Result: result})
	}))
	defer srv.Close()
	_, err := startMCPServerConnection(context.Background(), MCPServerConfig{
		Name: "future", Enabled: true, Transport: "streamable_http", URL: srv.URL,
	}, testLogger())
	if err == nil || !strings.Contains(err.Error(), "unsupported protocol version") {
		t.Fatalf("startMCPServerConnection error = %v, want unsupported protocol version", err)
	}
}

func writeMCPTestResponse(t *testing.T, w http.ResponseWriter, req jsonRPCRequest) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(mcpTestResponseFor(req)); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func mcpTestResponseFor(req jsonRPCRequest) jsonRPCResponse {
	var result interface{}
	switch req.Method {
	case "initialize":
		result = map[string]interface{}{
			"protocolVersion": mcpProtocolVersionCurrent,
			"serverInfo":      map[string]string{"name": "test-mcp", "version": "1.0.0"},
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
		}
	case "tools/list":
		result = map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "echo",
					"description": "Echo test tool",
					"inputSchema": map[string]interface{}{"type": "object"},
				},
			},
		}
	case "tools/call":
		result = map[string]interface{}{
			"content": []map[string]string{{"type": "text", "text": "called echo"}},
		}
	default:
		return jsonRPCResponse{JSONRPC: "2.0", ID: &req.ID, Error: &jsonRPCError{Code: -32601, Message: "not found"}}
	}
	data, _ := json.Marshal(result)
	return jsonRPCResponse{JSONRPC: "2.0", ID: &req.ID, Result: data}
}
