//go:build !remote_minimal

package remote

import (
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var allShellOperations = []string{
	OpShellExec, OpShellExecStream,
	OpShellSessionStart, OpShellSessionRead, OpShellSessionInput, OpShellSessionStop, OpShellSessionList,
}

// agentCommandOps forwards the operation of every command frame that reaches
// the agent side of the socket. It stops when the socket closes.
func agentCommandOps(agent *websocket.Conn) <-chan string {
	ops := make(chan string, 32)
	go func() {
		defer close(ops)
		for {
			var msg RemoteMessage
			if err := agent.ReadJSON(&msg); err != nil {
				return
			}
			if msg.Type != MsgCommand {
				continue
			}
			var cmd CommandPayload
			if json.Unmarshal(msg.Payload, &cmd) == nil {
				ops <- cmd.Operation
			}
		}
	}()
	return ops
}

func nextAgentCommandOp(t *testing.T, ops <-chan string) string {
	t.Helper()
	select {
	case op, ok := <-ops:
		if !ok {
			t.Fatal("agent socket closed before a command frame arrived")
		}
		return op
	case <-time.After(5 * time.Second):
		t.Fatal("no command frame reached the agent")
	}
	return ""
}

func TestSendCommandRefusesShellWithoutAllowedPaths(t *testing.T) {
	hub := NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	serverConn, agentConn, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	hub.Register("dev-1", &RemoteConnection{Conn: serverConn, DeviceID: "dev-1", SharedKey: strings.Repeat("b", 64)})
	ops := agentCommandOps(agentConn)

	// Nothing answers on the agent side, so a command the hub lets through
	// reaches the agent and ends as "timeout".
	send := func(op string) ResultPayload {
		t.Helper()
		result, err := hub.SendCommand("dev-1", CommandPayload{
			Operation: op,
			Args:      map[string]interface{}{"command": "id", "path": "/etc/hostname"},
		}, 20*time.Millisecond)
		if err != nil {
			t.Fatalf("SendCommand(%s): %v", op, err)
		}
		return result
	}
	expectRefused := func(stage string) {
		t.Helper()
		for _, op := range allShellOperations {
			result := send(op)
			if result.Status != "denied" || result.ErrorCode != "REMOTE_ALLOWED_PATHS_REQUIRED" ||
				!strings.Contains(result.Error, "allowed_paths") {
				t.Fatalf("%s: %s result = %+v, want allowed_paths denial", stage, op, result)
			}
		}
	}
	expectDispatched := func(op string) {
		t.Helper()
		if result := send(op); result.Status != "timeout" {
			t.Fatalf("%s result = %+v, want dispatch to the agent", op, result)
		}
		// Frames arrive in order, so this also proves no refused shell
		// command went out before it.
		if got := nextAgentCommandOp(t, ops); got != op {
			t.Fatalf("agent received %q, want %q", got, op)
		}
	}

	expectRefused("no allowed paths")
	expectDispatched(OpFileRead)

	if err := hub.SendConfigUpdate("dev-1", ConfigUpdatePayload{AllowedPaths: []string{"/srv/data"}}); err != nil {
		t.Fatalf("SendConfigUpdate grant: %v", err)
	}
	expectDispatched(OpShellExec)

	// An empty allowed_paths list in a full snapshot is an explicit revocation.
	if err := hub.SendConfigUpdate("dev-1", ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
		t.Fatalf("SendConfigUpdate revoke: %v", err)
	}
	expectRefused("after revocation")
	expectDispatched(OpSysinfo)
}

func TestSendCommandShellAllowedPathsWithoutConnection(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	offlineID, err := CreateDevice(db, DeviceRecord{Name: "nas", Status: "offline"})
	if err != nil {
		t.Fatalf("CreateDevice offline: %v", err)
	}
	agodeskID, err := CreateDevice(db, DeviceRecord{Name: "agodesk", Status: "approved", Tags: []string{"agodesk", "desktop-client"}})
	if err != nil {
		t.Fatalf("CreateDevice agodesk: %v", err)
	}
	hub := NewRemoteHub(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	transport := &recordingCommandTransport{connected: map[string]bool{agodeskID: true}, result: ResultPayload{Status: "ok"}}
	hub.RegisterCommandTransport("agodesk", transport)

	// Without a connection the stored device record decides.
	result, err := hub.SendCommand(offlineID, CommandPayload{Operation: OpShellExec}, time.Second)
	if err != nil || result.Status != "denied" || result.ErrorCode != "REMOTE_ALLOWED_PATHS_REQUIRED" {
		t.Fatalf("offline device shell result = %+v, err = %v; want allowed_paths denial", result, err)
	}

	// AgoDesk never receives allowed_paths; its shell access is its own
	// advertised capability and locally configured working directories.
	for _, op := range allShellOperations {
		result, err := hub.SendCommand(agodeskID, CommandPayload{Operation: op}, time.Second)
		if err != nil || result.Status != "ok" {
			t.Fatalf("agodesk %s result = %+v, err = %v; want dispatch to the transport", op, result, err)
		}
	}
	if len(transport.calls) != len(allShellOperations) {
		t.Fatalf("transport calls = %d, want %d", len(transport.calls), len(allShellOperations))
	}
}
