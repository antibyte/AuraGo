//go:build !remote_minimal

package remote

import (
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var allShellOperations = []string{
	OpShellExec, OpShellExecStream,
	OpShellSessionStart, OpShellSessionRead, OpShellSessionInput, OpShellSessionStop, OpShellSessionList,
}

// agentFrames forwards every frame that reaches the agent side of the socket,
// in order. It stops when the socket closes.
func agentFrames(agent *websocket.Conn) <-chan RemoteMessage {
	frames := make(chan RemoteMessage, 32)
	go func() {
		defer close(frames)
		for {
			var msg RemoteMessage
			if err := agent.ReadJSON(&msg); err != nil {
				return
			}
			frames <- msg
		}
	}()
	return frames
}

func nextAgentFrame(t *testing.T, frames <-chan RemoteMessage, wantType string) RemoteMessage {
	t.Helper()
	select {
	case msg, ok := <-frames:
		if !ok {
			t.Fatalf("agent socket closed before a %s frame arrived", wantType)
		}
		if msg.Type != wantType {
			t.Fatalf("agent received a %s frame (%s), want %s", msg.Type, msg.Payload, wantType)
		}
		return msg
	case <-time.After(5 * time.Second):
		t.Fatalf("no %s frame reached the agent", wantType)
	}
	return RemoteMessage{}
}

// nextAgentConfigPaths reads the next frame, which must be a config update
// carrying allowed_paths, and returns that list.
func nextAgentConfigPaths(t *testing.T, frames <-chan RemoteMessage) []string {
	t.Helper()
	msg := nextAgentFrame(t, frames, MsgConfigUpdate)
	var update struct {
		AllowedPaths *[]string `json:"allowed_paths"`
	}
	if err := json.Unmarshal(msg.Payload, &update); err != nil || update.AllowedPaths == nil {
		t.Fatalf("config update %s lacks allowed_paths (%v)", msg.Payload, err)
	}
	return *update.AllowedPaths
}

// remoteAgentFixture registers dev-1 on a live socket pair. Nothing answers on
// the agent side, so a command the hub lets through reaches the agent and
// ends as "timeout".
type remoteAgentFixture struct {
	hub    *RemoteHub
	frames <-chan RemoteMessage
}

func newRemoteAgentFixture(t *testing.T, allowedPaths []string) *remoteAgentFixture {
	t.Helper()
	hub := NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	serverConn, agentConn, cleanup := newWebSocketPairForHubTest(t)
	t.Cleanup(cleanup)
	hub.Register("dev-1", &RemoteConnection{
		Conn: serverConn, DeviceID: "dev-1", SharedKey: strings.Repeat("b", 64), AllowedPaths: allowedPaths,
	})
	return &remoteAgentFixture{hub: hub, frames: agentFrames(agentConn)}
}

func (f *remoteAgentFixture) send(t *testing.T, op string) ResultPayload {
	t.Helper()
	result, err := f.hub.SendCommand("dev-1", CommandPayload{
		Operation: op,
		Args:      map[string]interface{}{"command": "id", "path": "/etc/hostname"},
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("SendCommand(%s): %v", op, err)
	}
	return result
}

func (f *remoteAgentFixture) expectRefused(t *testing.T, stage string) {
	t.Helper()
	for _, op := range allShellOperations {
		result := f.send(t, op)
		if result.Status != "denied" || result.ErrorCode != "REMOTE_ALLOWED_PATHS_REQUIRED" ||
			!strings.Contains(result.Error, "allowed_paths") {
			t.Fatalf("%s: %s result = %+v, want allowed_paths denial", stage, op, result)
		}
	}
}

// expectDispatched sends op and checks it is the next frame the agent gets;
// frames arrive in order, so no refused command went out before it.
func (f *remoteAgentFixture) expectDispatched(t *testing.T, op string) {
	t.Helper()
	if result := f.send(t, op); result.Status != "timeout" {
		t.Fatalf("%s result = %+v, want dispatch to the agent", op, result)
	}
	msg := nextAgentFrame(t, f.frames, MsgCommand)
	var cmd CommandPayload
	if err := json.Unmarshal(msg.Payload, &cmd); err != nil || cmd.Operation != op {
		t.Fatalf("agent received command %s (%v), want %s", msg.Payload, err, op)
	}
}

func TestSendCommandRefusesShellWithoutAllowedPaths(t *testing.T) {
	f := newRemoteAgentFixture(t, nil)

	f.expectRefused(t, "no allowed paths")
	f.expectDispatched(t, OpFileRead)

	if err := f.hub.SendConfigUpdate("dev-1", ConfigUpdatePayload{AllowedPaths: []string{"/srv/data"}}); err != nil {
		t.Fatalf("SendConfigUpdate grant: %v", err)
	}
	if got := nextAgentConfigPaths(t, f.frames); !reflect.DeepEqual(got, []string{"/srv/data"}) {
		t.Fatalf("granted allowed_paths = %q", got)
	}
	f.expectDispatched(t, OpShellExec)

	// With no global default, an empty allowed_paths list in a full snapshot
	// reaches the agent as an explicit revocation.
	if err := f.hub.SendConfigUpdate("dev-1", ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
		t.Fatalf("SendConfigUpdate revoke: %v", err)
	}
	if got := nextAgentConfigPaths(t, f.frames); len(got) != 0 {
		t.Fatalf("revoked allowed_paths = %q, want an empty list", got)
	}
	f.expectRefused(t, "after revocation")
	f.expectDispatched(t, OpSysinfo)
}

func TestSendCommandShellUsesDefaultAllowedPaths(t *testing.T) {
	f := newRemoteAgentFixture(t, nil)

	// A default that holds no usable path counts as empty.
	f.hub.DefaultAllowedPaths = func() []string { return []string{"", "  "} }
	f.expectRefused(t, "blank default")

	f.hub.DefaultAllowedPaths = func() []string { return []string{"/srv"} }
	f.expectDispatched(t, OpShellExec)

	// A full snapshot that clears the device list falls back to the default.
	if err := f.hub.SendConfigUpdate("dev-1", ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
		t.Fatalf("SendConfigUpdate: %v", err)
	}
	if got := nextAgentConfigPaths(t, f.frames); !reflect.DeepEqual(got, []string{"/srv"}) {
		t.Fatalf("cleared device list sent allowed_paths = %q, want the default [/srv]", got)
	}
	f.expectDispatched(t, OpShellSessionStart)

	// Clearing the default as well leaves no paths.
	f.hub.DefaultAllowedPaths = func() []string { return nil }
	if err := f.hub.SendConfigUpdate("dev-1", ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
		t.Fatalf("SendConfigUpdate: %v", err)
	}
	if got := nextAgentConfigPaths(t, f.frames); len(got) != 0 {
		t.Fatalf("allowed_paths with both lists empty = %q, want an empty list", got)
	}
	f.expectRefused(t, "both lists empty")
	f.expectDispatched(t, OpSysinfo)
}

// The list an agent receives at authentication is the device's own list, or
// the global default when that is empty.
func TestAuthResponseCarriesEffectiveAllowedPaths(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	// HandleEnrollment runs on the socket's goroutine, so the default is
	// swapped atomically.
	var defaultPaths atomic.Pointer[[]string]
	defaultPaths.Store(&[]string{" /srv ", ""})
	hub.DefaultAllowedPaths = func() []string { return *defaultPaths.Load() }
	exchange := enrollmentTestSocket(t, hub)
	token := "allowed-paths-token"
	issueTestEnrollment(t, hub, token)

	enrolled := exchange(enrollmentFrame(t, token, 1))
	if enrolled.Status != "enrolled" || !reflect.DeepEqual(enrolled.AllowedPaths, []string{"/srv"}) {
		t.Fatalf("enrollment response = %+v, want allowed_paths [/srv]", enrolled)
	}
	if conn := hub.GetConnection(enrolled.DeviceID); conn == nil || !reflect.DeepEqual(conn.AllowedPaths, []string{"/srv"}) {
		t.Fatalf("enrolled connection = %+v, want allowed_paths [/srv]", conn)
	}

	seq := uint64(2)
	reconnect := func() AuthResponsePayload {
		t.Helper()
		msg, err := NewMessage(MsgAuth, enrolled.DeviceID, enrolled.SharedKey, seq, AuthPayload{DeviceID: enrolled.DeviceID, Hostname: "test"})
		if err != nil {
			t.Fatal(err)
		}
		seq++
		got := exchange(msg)
		if got.Status != "authenticated" {
			t.Fatalf("reconnect = %+v", got)
		}
		return got
	}
	setDevicePaths := func(paths []string) {
		t.Helper()
		device, err := GetDevice(db, enrolled.DeviceID)
		if err != nil {
			t.Fatal(err)
		}
		device.AllowedPaths = paths
		if err := UpdateDevice(db, device); err != nil {
			t.Fatal(err)
		}
	}

	if got := reconnect().AllowedPaths; !reflect.DeepEqual(got, []string{"/srv"}) {
		t.Fatalf("reconnect without a device list sent %q, want the default [/srv]", got)
	}

	setDevicePaths([]string{"/data"})
	if got := reconnect().AllowedPaths; !reflect.DeepEqual(got, []string{"/data"}) {
		t.Fatalf("reconnect with a device list sent %q, want the device list [/data]", got)
	}

	setDevicePaths(nil)
	defaultPaths.Store(&[]string{})
	if got := reconnect().AllowedPaths; got == nil || len(got) != 0 {
		t.Fatalf("reconnect with both lists empty sent %#v, want an empty list", got)
	}
	if blocked := hub.commandBlockedByMissingAllowedPaths(enrolled.DeviceID, hub.GetConnection(enrolled.DeviceID), OpShellExec); !blocked {
		t.Fatal("shell admitted with both lists empty")
	}
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
	// ... falling back to the global default when its own list is empty.
	hub.DefaultAllowedPaths = func() []string { return []string{"/srv"} }
	result, err = hub.SendCommand(offlineID, CommandPayload{Operation: OpShellExec}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "no active connection") {
		t.Fatalf("offline device with a default: result = %+v, err = %v; want only the missing connection", result, err)
	}
	hub.DefaultAllowedPaths = nil

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
