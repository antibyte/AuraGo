package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestMCPUnsentWaiterPreservesActiveCall(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	defer inputReader.Close()
	defer outputWriter.Close()
	tr := newStdioMCPTransport(nil, inputWriter, outputReader, nil)
	conn := &mcpConn{ready: true, name: "s", transport: tr, toolsRefreshed: time.Now()}
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{"s": conn}, configs: map[string]MCPServerConfig{"s": {Name: "s", Enabled: true, Command: "fixture"}}}
	defer mgr.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	accepted := make(chan jsonRPCRequest, 1)
	peerDone := make(chan error, 1)
	reply := make(chan struct{})
	go func() {
		line, err := bufio.NewReader(inputReader).ReadBytes('\n')
		var request jsonRPCRequest
		if err == nil {
			err = json.Unmarshal(line, &request)
		}
		if err != nil {
			peerDone <- err
			return
		}
		accepted <- request
		select {
		case <-reply:
		case <-ctx.Done():
			peerDone <- ctx.Err()
			return
		}
		_, err = fmt.Fprintf(outputWriter, `{"jsonrpc":"2.0","id":%d,"result":{"content":[{"type":"text","text":"completed"}]}}`+"\n", request.ID)
		peerDone <- err
	}()
	active := make(chan error, 1)
	go func() {
		result, err := mgr.CallTool(ctx, "s", "read", nil)
		if err == nil && !strings.Contains(result, "completed") {
			err = fmt.Errorf("unexpected result: %s", result)
		}
		active <- err
	}()
	select {
	case <-accepted:
	case err := <-peerDone:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	waiter, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stop()
	_, err := mgr.CallTool(waiter, "s", "read", nil)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, errMCPRequestNotSent) {
		t.Fatalf("unsent waiter: %v", err)
	}
	if tr.closing.Load() || mgr.conns["s"] != conn {
		t.Fatal("unsent waiter invalidated active connection")
	}
	close(reply)
	if err := <-active; err != nil {
		t.Fatal(err)
	}
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
}

func TestMCPDiscoveryWaiterPreservesConnection(t *testing.T) {
	for _, list := range []bool{false, true} {
		t.Run(fmt.Sprint(list), func(t *testing.T) {
			conn := &mcpConn{ready: true, name: "s", transport: &disclosureMCPTransport{}}
			mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{"s": conn}, configs: map[string]MCPServerConfig{"s": {Name: "s", Enabled: true, Command: "fixture"}}}
			defer mgr.Close()
			if err := conn.discoveryMu.Lock(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer conn.discoveryMu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			var err error
			if list {
				_, err = mgr.ListToolsWithStatus(ctx, "s")
			} else {
				_, err = mgr.CallTool(ctx, "s", "read", nil)
			}
			if !errors.Is(err, errMCPRequestNotSent) || mgr.conns["s"] != conn {
				t.Fatalf("discovery waiter retired connection: %v", err)
			}
		})
	}
}

func TestMCPWebSocketUnsentWaiterPreservesConnection(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var request jsonRPCRequest
		if conn.ReadJSON(&request) != nil {
			return
		}
		_ = conn.WriteJSON(map[string]interface{}{"id": request.ID, "result": map[string]interface{}{"content": []map[string]string{{"type": "text", "text": "completed"}}}})
		<-release
	}))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tr, err := newWebSocketMCPTransport(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	conn := &mcpConn{ready: true, name: "s", transport: tr, toolsRefreshed: time.Now()}
	mgr := &MCPManager{logger: testLogger(), conns: map[string]*mcpConn{"s": conn}, configs: map[string]MCPServerConfig{"s": {Name: "s", Enabled: true, Command: "fixture"}}}
	defer mgr.Close()
	if err := tr.writeMu.Lock(ctx); err != nil {
		t.Fatal(err)
	}
	waiter, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	defer stop()
	_, err = mgr.CallTool(waiter, "s", "read", nil)
	tr.writeMu.Unlock()
	if !errors.Is(err, errMCPRequestNotSent) || mgr.conns["s"] != conn {
		t.Fatalf("unsent websocket request retired connection: %v", err)
	}
	result, err := mgr.CallTool(ctx, "s", "read", nil)
	if err != nil || !strings.Contains(result, "completed") {
		t.Fatalf("connection unusable after cancelled waiter: %v", err)
	}
}

func TestMCPVisionLocalSchemaReferences(t *testing.T) {
	file := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(file, []byte("\x89PNG\r\n\x1a\nfixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, defs := range []string{"$defs", "definitions"} {
		for _, key := range []string{"image_path", "image_url", "image_base64"} {
			for _, wrapped := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/wrapped=%v", defs, key, wrapped), func(t *testing.T) {
					pointer := "#/" + defs + "/Image"
					schema := map[string]interface{}{
						"type": "object", "required": []string{key, "unrelated"},
						defs:         map[string]interface{}{"Image": map[string]interface{}{"type": "string", "minLength": 1}},
						"properties": map[string]interface{}{key: map[string]interface{}{"$ref": pointer}},
					}
					if wrapped {
						props := schema["properties"].(map[string]interface{})
						props[key] = map[string]interface{}{"$ref": "#/inputSchema/" + defs + "/Image"}
						schema = map[string]interface{}{"inputSchema": schema}
					}
					runtime := "streamable_http"
					if key == "image_path" {
						runtime = "local"
					}
					tool := &MCPToolInfo{Name: "vision", InputSchema: schema, connection: &mcpConn{runtime: runtime}}
					args, cleanup, err := buildPreferredMCPVisionArgs(context.Background(), tool, file, "")
					defer cleanup()
					if err != nil || args[key] == "" || args[key] == nil {
						t.Fatalf("valid schema reference rejected: %v", err)
					}
					if _, mutated := schema["$ref"]; mutated {
						t.Fatal("input schema mutated")
					}
				})
			}
		}
	}
}

func TestMCPVisionSchemaReferencesPreserveRootAndRestrictions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		schema   map[string]interface{}
		accepted bool
	}{
		{"root object reference", map[string]interface{}{"type": "object", "properties": map[string]interface{}{"image_url": map[string]interface{}{"$ref": "#"}}}, false},
		{"HTTP-only local reference", map[string]interface{}{"$defs": map[string]interface{}{"Image": map[string]interface{}{"type": "string", "pattern": "^https://"}}, "properties": map[string]interface{}{"image_url": map[string]interface{}{"$ref": "#/$defs/Image"}}}, false},
		{"external reference", map[string]interface{}{"properties": map[string]interface{}{"image_url": map[string]interface{}{"$ref": "https://example.invalid/image.json"}}}, false},
		{"local reference with document ID", map[string]interface{}{"$id": "https://example.invalid/schema.json", "$defs": map[string]interface{}{"Image": map[string]interface{}{"type": "string"}}, "properties": map[string]interface{}{"image_url": map[string]interface{}{"$ref": "#/$defs/Image"}}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mcpImageInputAccepts(tc.schema, "image_url", "data:image/png;base64,fixture"); got != tc.accepted {
				t.Fatalf("accepted=%v want=%v", got, tc.accepted)
			}
		})
	}
}
