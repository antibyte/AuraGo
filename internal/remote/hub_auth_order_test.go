//go:build !remote_minimal

package remote

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// A config push that fires while a device authenticates (here from OnConnect,
// which runs as the connection is published) must reach the agent after the
// auth response. That answer is written without the connection's write lock,
// so a push that could see the connection first would write concurrently, and
// gorilla/websocket panics on concurrent writers.
func TestAuthResponsePrecedesWritesToThePublishedConnection(t *testing.T) {
	hub, _, _ := newEnrollmentTestHub(t)
	hub.DefaultAllowedPaths = func() []string { return []string{"/srv"} }
	hub.OnConnect = func(deviceID, _ string) {
		if err := hub.SendConfigUpdate(deviceID, ConfigUpdatePayload{AllowedPaths: []string{}}); err != nil {
			t.Errorf("config push while authenticating: %v", err)
		}
	}
	authenticate := func(msg *RemoteMessage) AuthResponsePayload {
		t.Helper()
		serverConn, agentConn, cleanup := newWebSocketPairForHubTest(t)
		t.Cleanup(cleanup)
		frames := agentFrames(agentConn)
		if err := hub.HandleEnrollment(serverConn, *msg); err != nil {
			t.Fatalf("HandleEnrollment: %v", err)
		}
		first := nextAgentFrame(t, frames, MsgAuthResponse)
		var payload AuthResponsePayload
		if err := json.Unmarshal(first.Payload, &payload); err != nil {
			t.Fatalf("auth response payload: %v", err)
		}
		if got := nextAgentConfigPaths(t, frames); !reflect.DeepEqual(got, []string{"/srv"}) {
			t.Fatalf("config push after authentication carried %q, want [/srv]", got)
		}
		return payload
	}

	const token = "auth-order-token"
	issueTestEnrollment(t, hub, token)
	enrolled := authenticate(enrollmentFrame(t, token, 1))
	if enrolled.Status != "enrolled" {
		t.Fatalf("enrollment = %+v, want enrolled", enrolled)
	}

	reconnect, err := NewMessage(MsgAuth, enrolled.DeviceID, enrolled.SharedKey, 2,
		AuthPayload{DeviceID: enrolled.DeviceID, Hostname: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if got := authenticate(reconnect); got.Status != "authenticated" {
		t.Fatalf("reconnect = %+v, want authenticated", got)
	}
}

// lockedLogBuffer is a log sink that tolerates concurrent writes.
type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// The default list is pushed on a goroutine of its own, where nothing else
// would recover a panic. A transport that panics must cost only its own push:
// the panic is logged as an error and the other devices still get the list.
func TestPushDefaultAllowedPathsSurvivesAPanickingTransport(t *testing.T) {
	var logs lockedLogBuffer
	hub := NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(&logs, nil)))
	// A connection without a socket panics on its first write.
	hub.Register("broken", &RemoteConnection{DeviceID: "broken", SharedKey: strings.Repeat("b", 64)})
	healthy := connectRemoteAgent(t, hub, "healthy", nil)
	hub.DefaultAllowedPaths = func() []string { return []string{"/srv"} }

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PushDefaultAllowedPaths let a transport panic escape: %v", r)
			}
		}()
		hub.PushDefaultAllowedPaths()
	}()

	if got := nextAgentConfigPaths(t, healthy.frames); !reflect.DeepEqual(got, []string{"/srv"}) {
		t.Fatalf("healthy device received %q, want [/srv]", got)
	}
	logged := logs.String()
	if !strings.Contains(logged, "level=ERROR") || !strings.Contains(logged, "device_id=broken") {
		t.Fatalf("the recovered panic must be logged as an error naming the device:\n%s", logged)
	}
}
