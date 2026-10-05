package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/remote"

	"github.com/gorilla/websocket"
)

func TestClientStopIsIdempotentUnderConcurrentCalls(t *testing.T) {
	client := &Client{
		done:   make(chan struct{}),
		logger: slog.Default(),
	}

	const goroutines = 64
	start := make(chan struct{})
	panicCh := make(chan interface{}, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				panicCh <- recover()
			}()
			<-start
			client.Stop()
		}()
	}

	close(start)
	wg.Wait()
	close(panicCh)

	for recovered := range panicCh {
		if recovered != nil {
			t.Fatalf("Stop panicked under concurrent calls: %v", recovered)
		}
	}

	select {
	case <-client.done:
	default:
		t.Fatal("Stop did not close done channel")
	}
}

func TestHeartbeatLoopContinuesAfterTransientSendFailure(t *testing.T) {
	client := &Client{
		cfg:      clientConfig{DeviceID: "dev-1"},
		done:     make(chan struct{}),
		logger:   slog.Default(),
		executor: NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB),
	}

	finished := make(chan struct{})
	go func() {
		client.heartbeatLoopWithInterval(10 * time.Millisecond)
		close(finished)
	}()

	select {
	case <-finished:
		t.Fatal("heartbeat loop exited after transient send failure")
	case <-time.After(50 * time.Millisecond):
	}

	client.Stop()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("heartbeat loop did not exit after Stop")
	}
}

// isolateRemoteHome points the user home directory at a temp dir so that a
// regression reaching saveConfig cannot write to the real ~/.aurago-remote.
func isolateRemoteHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func assertNoStoredConfig(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(configPath()); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("auth response must not be persisted, stat %s: %v", configPath(), err)
	}
}

// startFakeSupervisor answers the agent's auth frame with reply and then waits
// for the agent to hang up.
func startFakeSupervisor(t *testing.T, reply *remote.RemoteMessage) string {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
		_ = conn.WriteJSON(reply)
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func newConnectTestClient(t *testing.T, cfg clientConfig) *Client {
	t.Helper()
	client := &Client{
		cfg:      cfg,
		logger:   slog.Default(),
		done:     make(chan struct{}),
		executor: NewExecutor(slog.Default(), remote.DefaultMaxFileSizeMB),
	}
	t.Cleanup(client.Stop)
	return client
}

func TestConnectRejectsUnsignedEnrolledResponse(t *testing.T) {
	isolateRemoteHome(t)
	resp, err := remote.NewMessage(remote.MsgAuthResponse, "dev-attacker", "", 1, remote.AuthResponsePayload{
		Status: "enrolled", DeviceID: "dev-attacker", SharedKey: strings.Repeat("ab", 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectTestClient(t, clientConfig{SupervisorURL: startFakeSupervisor(t, resp)})

	err = client.connect()
	if err == nil || !strings.Contains(err.Error(), "no bootstrap secret") {
		t.Fatalf("expected unsigned enrolled response to be rejected, got %v", err)
	}
	if client.cfg.SharedKey != "" || client.cfg.DeviceID != "" {
		t.Fatalf("attacker key must not be adopted: %+v", client.cfg)
	}
	assertNoStoredConfig(t)
}

func TestConnectAcceptsUnsignedPendingWithoutPersisting(t *testing.T) {
	isolateRemoteHome(t)
	readOnly := false
	resp, err := remote.NewMessage(remote.MsgAuthResponse, "dev-pending", "", 1, remote.AuthResponsePayload{
		Status: "pending", DeviceID: "dev-pending", Message: "awaiting approval in AuraGo UI",
		ReadOnly: &readOnly, AllowedPaths: []string{"/"}, MaxFileSizeMB: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectTestClient(t, clientConfig{SupervisorURL: startFakeSupervisor(t, resp)})
	client.readOnly = true

	err = client.connect()
	if err == nil || !strings.Contains(err.Error(), "pending") {
		t.Fatalf("expected pending approval error, got %v", err)
	}
	if client.cfg.DeviceID != "" || client.cfg.SharedKey != "" {
		t.Fatalf("pending answer must not set device identity: %+v", client.cfg)
	}
	if !client.readOnly || client.allowedPaths != nil {
		t.Fatalf("pending answer must not change bootstrap settings: read_only=%v allowed_paths=%v", client.readOnly, client.allowedPaths)
	}
	if got, want := client.executor.maxFileSizeBytesSnapshot(), int64(remote.DefaultMaxFileSizeMB)*1024*1024; got != want {
		t.Fatalf("pending answer must not change the file size limit: got %d bytes, want %d", got, want)
	}
	assertNoStoredConfig(t)
}

func TestConnectRejectsEnrolledWithInvalidSharedKey(t *testing.T) {
	for name, sharedKey := range map[string]string{
		"short non-hex":   "nothex",
		"64-char non-hex": strings.Repeat("zz", 32),
	} {
		t.Run(name, func(t *testing.T) {
			isolateRemoteHome(t)
			resp, err := remote.NewMessage(remote.MsgAuthResponse, "dev-1", remote.DeriveEnrollmentAuthKey("tok"), 1, remote.AuthResponsePayload{
				Status: "enrolled", DeviceID: "dev-1", SharedKey: sharedKey,
			})
			if err != nil {
				t.Fatal(err)
			}
			client := newConnectTestClient(t, clientConfig{SupervisorURL: startFakeSupervisor(t, resp), EnrollToken: "tok"})

			err = client.connect()
			if err == nil || !strings.Contains(err.Error(), "invalid shared key") {
				t.Fatalf("expected enrolled response with invalid shared key to be rejected, got %v", err)
			}
			if client.cfg.SharedKey != "" || client.cfg.DeviceID != "" || client.cfg.EnrollToken != "tok" {
				t.Fatalf("invalid enrolled response must not change config: %+v", client.cfg)
			}
			assertNoStoredConfig(t)
		})
	}
}

func TestConnectRejectsEnrolledWithoutDeviceID(t *testing.T) {
	isolateRemoteHome(t)
	resp, err := remote.NewMessage(remote.MsgAuthResponse, "", remote.DeriveEnrollmentAuthKey("tok"), 1, remote.AuthResponsePayload{
		Status: "enrolled", SharedKey: strings.Repeat("ab", 32),
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectTestClient(t, clientConfig{SupervisorURL: startFakeSupervisor(t, resp), EnrollToken: "tok"})

	err = client.connect()
	if err == nil || !strings.Contains(err.Error(), "carries no device id") {
		t.Fatalf("expected enrolled response without device id to be rejected, got %v", err)
	}
	if client.cfg.SharedKey != "" || client.cfg.DeviceID != "" || client.cfg.EnrollToken != "tok" {
		t.Fatalf("enrolled response without device id must not change config: %+v", client.cfg)
	}
	assertNoStoredConfig(t)
}

func TestConnectRejectsAuthenticatedWithoutSharedKey(t *testing.T) {
	isolateRemoteHome(t)
	readOnly := false
	resp, err := remote.NewMessage(remote.MsgAuthResponse, "dev-1", remote.DeriveEnrollmentAuthKey("tok"), 1, remote.AuthResponsePayload{
		Status: "authenticated", DeviceID: "dev-1", ReadOnly: &readOnly, AllowedPaths: []string{"/"},
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectTestClient(t, clientConfig{SupervisorURL: startFakeSupervisor(t, resp), EnrollToken: "tok"})
	client.readOnly = true

	err = client.connect()
	if err == nil || !strings.Contains(err.Error(), "without a device shared key") {
		t.Fatalf("expected authenticated response without device key to be rejected, got %v", err)
	}
	if client.cfg.SharedKey != "" || client.cfg.DeviceID != "" || client.cfg.EnrollToken != "tok" {
		t.Fatalf("authenticated response without device key must not change config: %+v", client.cfg)
	}
	if !client.readOnly || client.allowedPaths != nil {
		t.Fatalf("rejected authenticated response must not change bootstrap settings: read_only=%v allowed_paths=%v", client.readOnly, client.allowedPaths)
	}
	assertNoStoredConfig(t)
}

func writeStoredConfig(t *testing.T, cfg clientConfig) {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigIgnoresStoredDeviceIDWithoutSharedKey(t *testing.T) {
	isolateRemoteHome(t)
	stored := clientConfig{SupervisorURL: "ws://supervisor.example/remote", DeviceID: "dev-pending"}
	writeStoredConfig(t, stored)
	before, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatal(err)
	}

	cfg := loadConfig("", "tok", "")
	if cfg.DeviceID != "" || cfg.SharedKey != "" {
		t.Fatalf("stored device id without shared key must be ignored: %+v", cfg)
	}
	if cfg.EnrollToken != "tok" {
		t.Fatalf("configured enrollment token must survive, got %q", cfg.EnrollToken)
	}
	if cfg.SupervisorURL != stored.SupervisorURL {
		t.Fatalf("stored supervisor URL must still be used, got %q", cfg.SupervisorURL)
	}
	after, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("loadConfig must not rewrite the stored config:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestLoadConfigKeepsStoredDeviceWithSharedKey(t *testing.T) {
	isolateRemoteHome(t)
	sharedKey := strings.Repeat("ab", 32)
	writeStoredConfig(t, clientConfig{SupervisorURL: "ws://supervisor.example/remote", DeviceID: "dev-1", SharedKey: sharedKey})

	cfg := loadConfig("", "", "")
	if cfg.DeviceID != "dev-1" || cfg.SharedKey != sharedKey {
		t.Fatalf("enrolled device identity must be restored: %+v", cfg)
	}
}

func TestRejectReplayedFrameChecksDeviceTimestampAndNonce(t *testing.T) {
	client := &Client{cfg: clientConfig{DeviceID: "dev-1", SharedKey: strings.Repeat("ab", 32)}, logger: slog.Default()}
	fresh, err := remote.NewMessage(remote.MsgCommand, "dev-1", client.cfg.SharedKey, 1, map[string]string{"cmd_id": "x"})
	if err != nil {
		t.Fatal(err)
	}

	if reason := client.rejectReplayedFrame(*fresh); reason != "" {
		t.Fatalf("fresh frame must pass, got %q", reason)
	}
	if reason := client.rejectReplayedFrame(*fresh); reason != "nonce missing or replayed" {
		t.Fatalf("identical nonce must be rejected as a replay, got %q", reason)
	}

	foreign := *fresh
	foreign.DeviceID = "dev-2"
	foreign.Nonce = "0123456789abcdef0123456789abcdef"
	if reason := client.rejectReplayedFrame(foreign); reason != "device_id mismatch" {
		t.Fatalf("frame for another device must be rejected, got %q", reason)
	}

	stale := *fresh
	stale.Nonce = "fedcba9876543210fedcba9876543210"
	stale.Timestamp = time.Now().Add(-remote.MaxTimestampDrift - time.Minute).UTC().Format(time.RFC3339)
	if reason := client.rejectReplayedFrame(stale); !strings.HasPrefix(reason, "timestamp drift ") ||
		!strings.HasSuffix(reason, "exceeds maximum "+remote.MaxTimestampDrift.String()) {
		t.Fatalf("stale frame must be rejected for timestamp drift, got %q", reason)
	}
}

// hmacData joins Sequence and Nonce without a delimiter, so a captured frame
// with seq=12 still verifies as seq=1 with nonce "2"+nonce. The fixed nonce
// format is what keeps that shifted copy out of the replay cache.
func TestRejectReplayedFrameRejectsSequenceShiftedNonce(t *testing.T) {
	key := strings.Repeat("ab", 32)
	client := &Client{cfg: clientConfig{DeviceID: "dev-1", SharedKey: key}, logger: slog.Default()}
	frame, err := remote.NewMessage(remote.MsgCommand, "dev-1", key, 12, remote.CommandPayload{CommandID: "cmd-1"})
	if err != nil {
		t.Fatal(err)
	}
	if reason := client.rejectReplayedFrame(*frame); reason != "" {
		t.Fatalf("fresh frame must pass, got %q", reason)
	}

	shifted := *frame
	shifted.Sequence = 1
	shifted.Nonce = "2" + frame.Nonce
	if ok, err := remote.VerifyMessage(shifted, key); err != nil || !ok {
		t.Fatalf("shifted frame is expected to keep a valid HMAC (delimiter-free encoding): ok=%v err=%v", ok, err)
	}
	if reason := client.rejectReplayedFrame(shifted); reason != "invalid nonce format" {
		t.Fatalf("shifted frame must be rejected for its nonce format, got %q", reason)
	}
}

// The agent's lazily created cache must keep a nonce for the whole ±drift
// window the timestamp check accepts, matching remote.NonceReplayTTL.
func TestRejectReplayedFrameCacheCoversFullTimestampWindow(t *testing.T) {
	client := &Client{cfg: clientConfig{DeviceID: "dev-1", SharedKey: strings.Repeat("ab", 32)}, logger: slog.Default()}
	fresh, err := remote.NewMessage(remote.MsgCommand, "dev-1", client.cfg.SharedKey, 1, map[string]string{"cmd_id": "x"})
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	if reason := client.rejectReplayedFrame(*fresh); reason != "" {
		t.Fatalf("fresh frame must pass, got %q", reason)
	}
	if client.replay == nil {
		t.Fatal("rejectReplayedFrame must create the replay cache")
	}
	if !client.replay.Seen("dev-1", fresh.Nonce, t0.Add(remote.MaxTimestampDrift+time.Minute)) {
		t.Fatal("nonce must stay cached past MaxTimestampDrift while the frame can still be fresh")
	}
	if client.replay.Seen("dev-1", fresh.Nonce, t0.Add(remote.NonceReplayTTL+time.Minute)) {
		t.Fatal("nonce should expire after remote.NonceReplayTTL")
	}
}

// dialFrameSupervisor returns an agent-side connection to a fake supervisor
// that writes frames as soon as the agent connects and then closes normally.
func dialFrameSupervisor(t *testing.T, frames ...*remote.RemoteMessage) *websocket.Conn {
	t.Helper()
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for _, frame := range frames {
			if err := conn.WriteJSON(frame); err != nil {
				return
			}
		}
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial fake supervisor: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// readMessagesDispatched runs the client's read loop until the fake supervisor
// hangs up and returns the frames that reached handleMessage. The hook keeps
// the frames from executing, so no command, config update or revoke runs.
func readMessagesDispatched(t *testing.T, client *Client) []remote.RemoteMessage {
	t.Helper()
	var mu sync.Mutex
	var dispatched []remote.RemoteMessage
	prev := handleMessageHook
	handleMessageHook = func(msg remote.RemoteMessage) {
		mu.Lock()
		dispatched = append(dispatched, msg)
		mu.Unlock()
	}

	conn := client.currentConn()
	finished := make(chan struct{})
	go func() {
		client.readMessages()
		close(finished)
	}()
	// Stop the read loop before restoring the hook so a timed-out loop cannot
	// observe the swap.
	t.Cleanup(func() {
		if conn != nil {
			_ = conn.Close()
		}
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("readMessages still running after its connection was closed")
		}
		handleMessageHook = prev
	})
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("readMessages did not return after the supervisor closed the connection")
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]remote.RemoteMessage(nil), dispatched...)
}

func TestReadMessagesFailsClosedWithoutSharedKey(t *testing.T) {
	isolateRemoteHome(t)
	unsigned, err := remote.NewMessage(remote.MsgCommand, "dev-1", "", 1, remote.CommandPayload{CommandID: "cmd-unsigned"})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectTestClient(t, clientConfig{DeviceID: "dev-1"})
	client.conn = dialFrameSupervisor(t, unsigned)

	if dispatched := readMessagesDispatched(t, client); len(dispatched) != 0 {
		t.Fatalf("frames must not be dispatched without a device shared key, got %d", len(dispatched))
	}
}

func TestReadMessagesDispatchesSignedFrameOnceAndDropsReplay(t *testing.T) {
	isolateRemoteHome(t)
	sharedKey := strings.Repeat("ab", 32)
	signed, err := remote.NewMessage(remote.MsgCommand, "dev-1", sharedKey, 1, remote.CommandPayload{CommandID: "cmd-1"})
	if err != nil {
		t.Fatal(err)
	}
	forged, err := remote.NewMessage(remote.MsgCommand, "dev-1", strings.Repeat("cd", 32), 2, remote.CommandPayload{CommandID: "cmd-forged"})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectTestClient(t, clientConfig{DeviceID: "dev-1", SharedKey: sharedKey})
	client.conn = dialFrameSupervisor(t, signed, forged, signed)

	dispatched := readMessagesDispatched(t, client)
	if len(dispatched) != 1 {
		t.Fatalf("expected exactly one dispatched frame (fresh signed), got %d", len(dispatched))
	}
	if dispatched[0].Nonce != signed.Nonce {
		t.Fatalf("dispatched the wrong frame: %+v", dispatched[0])
	}
}

func TestStatusOutput(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *clientConfig
		want    []string
		notWant []string
	}{
		{
			name: "not configured",
			cfg:  nil,
			want: []string{"Not configured."},
		},
		{
			name:    "enrolled",
			cfg:     &clientConfig{SupervisorURL: "ws://sup", DeviceID: "dev-1", SharedKey: strings.Repeat("ab", 32)},
			want:    []string{"Device ID:      dev-1\n", "Status:         Enrolled (shared key present)\n"},
			notWant: []string{"stale pending id"},
		},
		{
			name:    "stale pending id",
			cfg:     &clientConfig{SupervisorURL: "ws://sup", DeviceID: "dev-pending"},
			want:    []string{"Device ID:      \n", "Status:         Not yet enrolled (stale pending id, ignored)\n"},
			notWant: []string{"dev-pending"},
		},
		{
			name:    "never enrolled",
			cfg:     &clientConfig{SupervisorURL: "ws://sup"},
			want:    []string{"Status:         Not yet enrolled\n"},
			notWant: []string{"stale pending id"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			writeStatus(&out, tc.cfg)
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("status output missing %q:\n%s", want, out.String())
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(out.String(), notWant) {
					t.Errorf("status output must not contain %q:\n%s", notWant, out.String())
				}
			}
		})
	}
}
