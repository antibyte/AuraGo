package virtualcomputers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type handshakeTestTransport struct {
	capabilities WorkspaceCapabilities
	err          error
}

func (t handshakeTestTransport) Call(_ context.Context, machineID, method string, params interface{}, result interface{}) error {
	if t.err != nil {
		return t.err
	}
	if method != "system.capabilities" {
		return WorkspaceRPCError{Code: "unexpected_method", Message: method}
	}
	request, ok := params.(WorkspaceHandshakeRequest)
	if !ok || request.ProtocolVersion != WorkspaceProtocolVersion || request.MachineID != machineID {
		return WorkspaceRPCError{Code: "invalid_handshake", Message: "invalid handshake request"}
	}
	*result.(*WorkspaceCapabilities) = t.capabilities
	return nil
}

func TestWorkspaceHandshakeValidation(t *testing.T) {
	tests := []struct {
		name string
		caps WorkspaceCapabilities
		code string
	}{
		{name: "valid", caps: WorkspaceCapabilities{ProtocolVersion: WorkspaceProtocolVersion, MachineID: "vm-1", InstanceNonce: "nonce-1"}},
		{name: "version", caps: WorkspaceCapabilities{ProtocolVersion: "aurago.workspace.v0", MachineID: "vm-1", InstanceNonce: "nonce-1"}, code: "workspace_protocol_mismatch"},
		{name: "machine", caps: WorkspaceCapabilities{ProtocolVersion: WorkspaceProtocolVersion, MachineID: "vm-2", InstanceNonce: "nonce-1"}, code: "workspace_machine_mismatch"},
		{name: "nonce", caps: WorkspaceCapabilities{ProtocolVersion: WorkspaceProtocolVersion, MachineID: "vm-1"}, code: "workspace_invalid_handshake"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := WorkspaceHandshake(context.Background(), handshakeTestTransport{capabilities: test.caps}, "vm-1", "old-nonce")
			if test.code == "" {
				if err != nil {
					t.Fatalf("WorkspaceHandshake: %v", err)
				}
				return
			}
			rpcErr, ok := err.(WorkspaceRPCError)
			if !ok || rpcErr.Code != test.code {
				t.Fatalf("WorkspaceHandshake error = %#v, want code %q", err, test.code)
			}
		})
	}
}

func TestWebSocketWorkspaceTransportUsesBearerDeadlinesAndMonotonicSequences(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var sequences []uint64
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/machines/vm-1/workspace" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer host-only-token" || r.URL.Query().Get("token") != "" {
			http.Error(w, "invalid workspace authentication", http.StatusUnauthorized)
			return
		}
		connection, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer connection.Close()
		var request workspaceRPCRequest
		if err := connection.ReadJSON(&request); err != nil {
			t.Errorf("read workspace request: %v", err)
			return
		}
		if request.JSONRPC != "2.0" || request.Protocol != WorkspaceProtocolVersion || request.ID == "" || request.Deadline == "" {
			t.Errorf("invalid workspace request envelope: %+v", request)
		}
		mu.Lock()
		sequences = append(sequences, request.Sequence)
		mu.Unlock()
		_ = connection.WriteJSON(workspaceRPCResponse{
			JSONRPC: "2.0", Protocol: WorkspaceProtocolVersion, ID: "unrelated", Sequence: request.Sequence,
			Result: json.RawMessage(`{"ignored":true}`),
		})
		_ = connection.WriteJSON(workspaceRPCResponse{
			JSONRPC: "2.0", Protocol: WorkspaceProtocolVersion, ID: request.ID, Sequence: request.Sequence,
			Result: json.RawMessage(`{"ok":true}`),
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{BaseURL: server.URL, Token: "host-only-token", Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	transport := NewWebSocketWorkspaceTransport(client)
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var result map[string]bool
		err := transport.Call(ctx, "vm-1", "system.health", map[string]bool{"probe": true}, &result)
		cancel()
		if err != nil || !result["ok"] {
			t.Fatalf("transport call %d = result=%v err=%v", i, result, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(sequences) != 2 || sequences[0] == 0 || sequences[1] != sequences[0]+1 {
		t.Fatalf("workspace sequences = %v, want consecutive non-zero values", sequences)
	}
}
