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

// remoteAgent is a device registered on a live socket pair. Nothing answers on
// the agent side, so a command the hub lets through reaches the agent and
// ends as "timeout".
type remoteAgent struct {
	hub    *RemoteHub
	id     string
	frames <-chan RemoteMessage
}

func connectRemoteAgent(t *testing.T, hub *RemoteHub, deviceID string, ownPaths []string) *remoteAgent {
	t.Helper()
	serverConn, agentConn, cleanup := newWebSocketPairForHubTest(t)
	t.Cleanup(cleanup)
	hub.Register(deviceID, &RemoteConnection{
		Conn: serverConn, DeviceID: deviceID, SharedKey: strings.Repeat("b", 64), AllowedPaths: ownPaths,
	})
	return &remoteAgent{hub: hub, id: deviceID, frames: agentFrames(agentConn)}
}

func newRemoteAgentFixture(t *testing.T, ownPaths []string) *remoteAgent {
	t.Helper()
	return connectRemoteAgent(t, NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))), "dev-1", ownPaths)
}

func (a *remoteAgent) send(t *testing.T, op string) ResultPayload {
	t.Helper()
	result, err := a.hub.SendCommand(a.id, CommandPayload{
		Operation: op,
		Args:      map[string]interface{}{"command": "id", "path": "/etc/hostname"},
	}, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("SendCommand(%s): %v", op, err)
	}
	return result
}

func (a *remoteAgent) expectRefused(t *testing.T, stage string) {
	t.Helper()
	for _, op := range allShellOperations {
		result := a.send(t, op)
		if result.Status != "denied" || result.ErrorCode != ShellRequiresAllowedPathsCode ||
			!strings.Contains(result.Error, "allowed_paths") {
			t.Fatalf("%s: %s result = %+v, want allowed_paths denial", stage, op, result)
		}
	}
}

// expectDispatched sends op and checks it is the next frame the agent gets;
// frames arrive in order, so nothing else went out before it.
func (a *remoteAgent) expectDispatched(t *testing.T, op string) {
	t.Helper()
	if result := a.send(t, op); result.Status != "timeout" {
		t.Fatalf("%s result = %+v, want dispatch to the agent", op, result)
	}
	msg := nextAgentFrame(t, a.frames, MsgCommand)
	var cmd CommandPayload
	if err := json.Unmarshal(msg.Payload, &cmd); err != nil || cmd.Operation != op {
		t.Fatalf("agent received command %s (%v), want %s", msg.Payload, err, op)
	}
}

func (a *remoteAgent) ownPaths() []string {
	conn := a.hub.GetConnection(a.id)
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.AllowedPaths
}

func TestSendCommandRefusesShellWithoutAllowedPaths(t *testing.T) {
	a := newRemoteAgentFixture(t, nil)

	a.expectRefused(t, "no allowed paths")
	a.expectDispatched(t, OpFileRead)

	if err := a.hub.SendConfigUpdate(a.id, ConfigUpdatePayload{AllowedPaths: []string{"/srv/data"}}); err != nil {
		t.Fatalf("SendConfigUpdate grant: %v", err)
	}
	if got := nextAgentConfigPaths(t, a.frames); !reflect.DeepEqual(got, []string{"/srv/data"}) {
		t.Fatalf("granted allowed_paths = %q", got)
	}
	a.expectDispatched(t, OpShellExec)

	// With no global default, an empty allowed_paths list in a full snapshot
	// reaches the agent as an explicit revocation.
	if err := a.hub.SendConfigUpdate(a.id, ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
		t.Fatalf("SendConfigUpdate revoke: %v", err)
	}
	if got := nextAgentConfigPaths(t, a.frames); len(got) != 0 {
		t.Fatalf("revoked allowed_paths = %q, want an empty list", got)
	}
	a.expectRefused(t, "after revocation")
	a.expectDispatched(t, OpSysinfo)
}

func TestSendCommandShellUsesDefaultAllowedPaths(t *testing.T) {
	a := newRemoteAgentFixture(t, nil)

	// A default that holds no usable path counts as empty.
	a.hub.DefaultAllowedPaths = func() []string { return []string{"", "  "} }
	a.expectRefused(t, "blank default")

	a.hub.DefaultAllowedPaths = func() []string { return []string{"/srv"} }
	a.expectDispatched(t, OpShellExec)

	// A full snapshot that clears the device list sends the default, while the
	// connection keeps the device's own (empty) list.
	if err := a.hub.SendConfigUpdate(a.id, ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
		t.Fatalf("SendConfigUpdate: %v", err)
	}
	if got := nextAgentConfigPaths(t, a.frames); !reflect.DeepEqual(got, []string{"/srv"}) {
		t.Fatalf("cleared device list sent allowed_paths = %q, want the default [/srv]", got)
	}
	if got := a.ownPaths(); len(got) != 0 {
		t.Fatalf("connection list = %q, want the device's own empty list", got)
	}
	a.expectDispatched(t, OpShellSessionStart)

	// The default is evaluated live: once it is empty, shell is refused at
	// once, before any push reaches the agent.
	a.hub.DefaultAllowedPaths = func() []string { return nil }
	a.expectRefused(t, "default cleared")

	if err := a.hub.SendConfigUpdate(a.id, ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
		t.Fatalf("SendConfigUpdate: %v", err)
	}
	if got := nextAgentConfigPaths(t, a.frames); len(got) != 0 {
		t.Fatalf("allowed_paths with both lists empty = %q, want an empty list", got)
	}
	a.expectDispatched(t, OpSysinfo)

	// A device's own list does not depend on the default.
	own := newRemoteAgentFixture(t, []string{"/data"})
	own.expectDispatched(t, OpShellExec)
}

func TestPushDefaultAllowedPathsUpdatesDevicesWithoutOwnList(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	bareID, err := CreateDevice(db, DeviceRecord{Name: "bare", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	ownID, err := CreateDevice(db, DeviceRecord{Name: "own", Status: "approved", AllowedPaths: []string{"/data"}})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewRemoteHub(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	bare := connectRemoteAgent(t, hub, bareID, nil)
	own := connectRemoteAgent(t, hub, ownID, []string{"/data"})

	hub.DefaultAllowedPaths = func() []string { return []string{"/srv"} }
	hub.PushDefaultAllowedPaths()

	if got := nextAgentConfigPaths(t, bare.frames); !reflect.DeepEqual(got, []string{"/srv"}) {
		t.Fatalf("device without its own list received %q, want the default [/srv]", got)
	}
	if got := bare.ownPaths(); len(got) != 0 {
		t.Fatalf("connection list after push = %q, want the device's own empty list", got)
	}
	bare.expectDispatched(t, OpShellExec)
	// The next frame on the other socket is this command, so no config update
	// went to the device with its own list.
	own.expectDispatched(t, OpSysinfo)
}

// The list an agent receives at authentication is the device's own list, or
// the global default when that is empty; the connection keeps the own list.
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

	connectionPaths := func(deviceID string) []string {
		t.Helper()
		conn := hub.GetConnection(deviceID)
		if conn == nil {
			t.Fatalf("device %s has no connection", deviceID)
		}
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return conn.AllowedPaths
	}

	enrolled := exchange(enrollmentFrame(t, token, 1))
	if enrolled.Status != "enrolled" || !reflect.DeepEqual(enrolled.AllowedPaths, []string{"/srv"}) {
		t.Fatalf("enrollment response = %+v, want allowed_paths [/srv]", enrolled)
	}
	if got := connectionPaths(enrolled.DeviceID); len(got) != 0 {
		t.Fatalf("enrolled connection list = %q, want the device's own empty list", got)
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
	if got := connectionPaths(enrolled.DeviceID); !reflect.DeepEqual(got, []string{"/data"}) {
		t.Fatalf("connection list = %q, want [/data]", got)
	}

	setDevicePaths(nil)
	defaultPaths.Store(&[]string{})
	if got := reconnect().AllowedPaths; got == nil || len(got) != 0 {
		t.Fatalf("reconnect with both lists empty sent %#v, want an empty list", got)
	}
	if blocked := hub.commandBlockedByMissingAllowedPaths(hub.GetConnection(enrolled.DeviceID), OpShellExec); !blocked {
		t.Fatal("shell admitted with both lists empty")
	}
}

func TestEffectiveAllowedPathsForDevice(t *testing.T) {
	db, err := InitDB(filepath.Join(t.TempDir(), "remote.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	offlineID, err := CreateDevice(db, DeviceRecord{Name: "offline", Status: "offline"})
	if err != nil {
		t.Fatal(err)
	}
	ownID, err := CreateDevice(db, DeviceRecord{Name: "own", Status: "offline", AllowedPaths: []string{" /data "}})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewRemoteHub(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	connected := connectRemoteAgent(t, hub, "live-1", []string{"/live"})

	if got := hub.EffectiveAllowedPaths(offlineID); got == nil || len(got) != 0 {
		t.Fatalf("no lists: %#v, want an empty list", got)
	}
	hub.DefaultAllowedPaths = func() []string { return []string{"/srv"} }
	for _, tc := range []struct {
		deviceID string
		want     []string
	}{
		{offlineID, []string{"/srv"}},
		{ownID, []string{"/data"}},
		{connected.id, []string{"/live"}},
	} {
		if got := hub.EffectiveAllowedPaths(tc.deviceID); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("EffectiveAllowedPaths(%s) = %q, want %q", tc.deviceID, got, tc.want)
		}
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
	offlineAgodeskID, err := CreateDevice(db, DeviceRecord{Name: "agodesk-off", Status: "offline", Tags: []string{"agodesk", "desktop-client"}})
	if err != nil {
		t.Fatalf("CreateDevice offline agodesk: %v", err)
	}
	hub := NewRemoteHub(db, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	transport := &recordingCommandTransport{connected: map[string]bool{agodeskID: true}, result: ResultPayload{Status: "ok"}}
	hub.RegisterCommandTransport("agodesk", transport)

	// Without a connection the allowed-paths check stays out of the way and
	// dispatch reports the missing connection.
	for _, deviceID := range []string{offlineID, offlineAgodeskID} {
		result, err := hub.SendCommand(deviceID, CommandPayload{Operation: OpShellExec}, time.Second)
		if err == nil || !strings.Contains(err.Error(), "no active connection") || result.Status == "denied" {
			t.Fatalf("offline device %s: result = %+v, err = %v; want only the missing connection", deviceID, result, err)
		}
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
