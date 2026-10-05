package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

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
	url, _ := startScriptedSupervisor(t, func(remote.RemoteMessage) *remote.RemoteMessage { return reply })
	return url
}

// startScriptedSupervisor answers each auth frame with answer(frame) and then
// waits for the agent to hang up; a nil answer hangs up at once. Every auth
// frame it receives is also sent on the returned channel.
func startScriptedSupervisor(t *testing.T, answer func(auth remote.RemoteMessage) *remote.RemoteMessage) (string, <-chan remote.RemoteMessage) {
	t.Helper()
	frames := make(chan remote.RemoteMessage, 16)
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var auth remote.RemoteMessage
		if err := conn.ReadJSON(&auth); err != nil {
			return
		}
		select {
		case frames <- auth:
		default:
		}
		reply := answer(auth)
		if reply == nil {
			return
		}
		_ = conn.WriteJSON(reply)
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), frames
}

func receiveAuthFrame(t *testing.T, frames <-chan remote.RemoteMessage) remote.RemoteMessage {
	t.Helper()
	select {
	case frame := <-frames:
		return frame
	case <-time.After(5 * time.Second):
		t.Fatal("the agent sent no auth frame")
		return remote.RemoteMessage{}
	}
}

// The enrollment frame travels before anything is authenticated, possibly over
// ws:// or to an impostor. It may carry the lookup hash but never the key that
// signs the supervisor's answer.
func TestConnectSendsLookupHashAndSignsWithAuthKey(t *testing.T) {
	isolateRemoteHome(t)
	const token = "remote_0123456789abcdef0123456789abcdef"
	url, frames := startScriptedSupervisor(t, func(remote.RemoteMessage) *remote.RemoteMessage { return nil })
	client := newConnectTestClient(t, clientConfig{SupervisorURL: url, EnrollToken: token})

	if err := client.connect(); err == nil {
		t.Fatal("connect must fail when the supervisor does not answer")
	}
	frame := receiveAuthFrame(t, frames)
	var auth remote.AuthPayload
	if err := json.Unmarshal(frame.Payload, &auth); err != nil {
		t.Fatal(err)
	}
	if auth.KDF != remote.EnrollmentKDFVersion || auth.TokenHash != remote.DeriveEnrollmentLookupHash(token) || auth.DeviceID != "" {
		t.Fatalf("enrollment payload = %+v", auth)
	}
	if ok, err := remote.VerifyMessage(frame, remote.DeriveEnrollmentAuthKey(token)); err != nil || !ok {
		t.Fatalf("enrollment frame must be signed with the MAC key: ok=%v err=%v", ok, err)
	}
	if ok, _ := remote.VerifyMessage(frame, auth.TokenHash); ok {
		t.Fatal("enrollment frame must not be signed with the lookup hash it carries")
	}
	raw, err := json.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	plain := sha256.Sum256([]byte(token))
	for name, secret := range map[string]string{
		"raw token":  token,
		"plain hash": hex.EncodeToString(plain[:]),
		"MAC key":    remote.DeriveEnrollmentAuthKey(token),
	} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("enrollment frame leaks the %s", name)
		}
	}
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

// The supervisor sends refusals it cannot or must not sign (unknown or used
// token, failed authentication) unsigned, and a tokenless knock has nothing to
// verify with. The agent reports the reason marked unverified, as an error
// errors.Is can tell from a verified refusal, and applies nothing.
func TestConnectReportsUnsignedRefusalAsUnverified(t *testing.T) {
	for name, cfg := range map[string]clientConfig{
		"enrollment token": {EnrollToken: "tok"},
		"device key":       {DeviceID: "dev-1", SharedKey: strings.Repeat("ab", 32)},
		"no key":           {},
	} {
		t.Run(name, func(t *testing.T) {
			isolateRemoteHome(t)
			readOnly := false
			resp, err := remote.NewAuthResponseMessage("", "", remote.AuthResponsePayload{
				Status: "rejected", Message: "enrollment token already used",
				ReadOnly: &readOnly, AllowedPaths: []string{"/"}, MaxFileSizeMB: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			cfg.SupervisorURL = startFakeSupervisor(t, resp)
			want := cfg
			client := newConnectTestClient(t, cfg)
			client.readOnly = true
			client.allowedPaths = []string{"/safe"}

			err = client.connect()
			if err == nil || err.Error() != `supervisor refused (unverified): "enrollment token already used"` || !errors.Is(err, errUnverifiedRefusal) {
				t.Fatalf("expected the unverified refusal reason, got %v", err)
			}
			if client.cfg != want {
				t.Fatalf("an unsigned refusal must not change config: %+v", client.cfg)
			}
			assertRestrictedSettingsKept(t, client)
			if got, want := client.executor.maxFileSizeBytesSnapshot(), int64(remote.DefaultMaxFileSizeMB)*1024*1024; got != want {
				t.Fatalf("an unsigned refusal must not change the file size limit: got %d bytes, want %d", got, want)
			}
			assertNoStoredConfig(t)
		})
	}
}

// An unverified reason comes from whoever answered, so it is capped and
// quoted before it reaches the log.
func TestUnverifiedRefusalReasonIsCappedAndQuoted(t *testing.T) {
	for name, tc := range map[string]struct {
		reason  string
		wantLen int
	}{
		"ascii": {strings.Repeat("x", 1000), maxUnverifiedReasonBytes},
		// 256 is not a multiple of the 3-byte rune, so the cut backs up to 255.
		"multi-byte": {strings.Repeat("€", 100), 255},
		"newlines":   {"line one\nforged log line", len("line one\nforged log line")},
	} {
		reason := tc.reason
		err := unverifiedRefusal(reason)
		quoted := strings.TrimPrefix(err.Error(), "supervisor refused (unverified): ")
		got, uerr := strconv.Unquote(quoted)
		if uerr != nil {
			t.Fatalf("%s: reason must be a quoted string: %q", name, err.Error())
		}
		if len(got) != tc.wantLen || !utf8.ValidString(got) || !strings.HasPrefix(reason, got) {
			t.Fatalf("%s: reason must be a valid %d-byte prefix, got %d bytes", name, tc.wantLen, len(got))
		}
		if strings.Contains(err.Error(), "\n") || !errors.Is(err, errUnverifiedRefusal) {
			t.Fatalf("%s: error = %q", name, err.Error())
		}
	}
}

// Only an unsigned refusal is reported; any other unsigned status to an agent
// holding a key stays an error that carries nothing from the answer.
func TestConnectRefusesOtherUnsignedResponsesWithKey(t *testing.T) {
	for _, status := range []string{"enrolled", "authenticated", "pending"} {
		for name, cfg := range map[string]clientConfig{
			"enrollment token": {EnrollToken: "tok"},
			"device key":       {DeviceID: "dev-1", SharedKey: strings.Repeat("ab", 32)},
		} {
			t.Run(status+"/"+name, func(t *testing.T) {
				isolateRemoteHome(t)
				readOnly := false
				resp, err := remote.NewAuthResponseMessage("dev-attacker", "", remote.AuthResponsePayload{
					Status: status, DeviceID: "dev-attacker", SharedKey: strings.Repeat("cd", 32), Message: "attacker text",
					ReadOnly: &readOnly, AllowedPaths: []string{"/"},
				})
				if err != nil {
					t.Fatal(err)
				}
				cfg.SupervisorURL = startFakeSupervisor(t, resp)
				want := cfg
				client := newConnectTestClient(t, cfg)
				client.readOnly = true
				client.allowedPaths = []string{"/safe"}

				err = client.connect()
				if err == nil || err.Error() != "received unsigned auth response despite bootstrap key" {
					t.Fatalf("expected the unsigned %s answer to be refused, got %v", status, err)
				}
				if client.cfg != want {
					t.Fatalf("an unsigned %s answer must not change config: %+v", status, client.cfg)
				}
				assertRestrictedSettingsKept(t, client)
				assertNoStoredConfig(t)
			})
		}
	}
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

// boundAuthResponse answers an auth frame the way the supervisor does: the
// reply echoes the frame's nonce in RequestNonce and is signed with key.
func boundAuthResponse(t *testing.T, key string, payload remote.AuthResponsePayload) func(remote.RemoteMessage) *remote.RemoteMessage {
	t.Helper()
	return func(auth remote.RemoteMessage) *remote.RemoteMessage {
		reply := payload
		reply.RequestNonce = auth.Nonce
		resp, err := remote.NewAuthResponseMessage(reply.DeviceID, key, reply)
		if err != nil {
			t.Error(err)
			return nil
		}
		return resp
	}
}

// startBoundSupervisor answers every auth frame with a bound, signed reply.
func startBoundSupervisor(t *testing.T, key string, payload remote.AuthResponsePayload) string {
	t.Helper()
	url, _ := startScriptedSupervisor(t, boundAuthResponse(t, key, payload))
	return url
}

func TestConnectAcceptsBoundEnrolledResponse(t *testing.T) {
	isolateRemoteHome(t)
	sharedKey := strings.Repeat("ab", 32)
	url := startBoundSupervisor(t, remote.DeriveEnrollmentAuthKey("tok"), remote.AuthResponsePayload{
		Status: "enrolled", DeviceID: "dev-1", SharedKey: sharedKey,
	})
	client := newConnectTestClient(t, clientConfig{SupervisorURL: url, EnrollToken: "tok"})

	if err := client.connect(); err != nil {
		t.Fatalf("a bound enrolled answer signed with the MAC key must be accepted: %v", err)
	}
	if client.cfg.DeviceID != "dev-1" || client.cfg.SharedKey != sharedKey || client.cfg.EnrollToken != "" {
		t.Fatalf("enrollment must adopt the device identity and drop the token: %+v", client.cfg)
	}
	stored := loadStoredConfig()
	if stored == nil || stored.DeviceID != "dev-1" || stored.SharedKey != sharedKey {
		t.Fatalf("enrollment must be persisted: %+v", stored)
	}
}

func TestConnectRejectsEnrolledWithInvalidSharedKey(t *testing.T) {
	for name, sharedKey := range map[string]string{
		"short non-hex":   "nothex",
		"64-char non-hex": strings.Repeat("zz", 32),
	} {
		t.Run(name, func(t *testing.T) {
			isolateRemoteHome(t)
			url := startBoundSupervisor(t, remote.DeriveEnrollmentAuthKey("tok"), remote.AuthResponsePayload{
				Status: "enrolled", DeviceID: "dev-1", SharedKey: sharedKey,
			})
			client := newConnectTestClient(t, clientConfig{SupervisorURL: url, EnrollToken: "tok"})

			err := client.connect()
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
	url := startBoundSupervisor(t, remote.DeriveEnrollmentAuthKey("tok"), remote.AuthResponsePayload{
		Status: "enrolled", SharedKey: strings.Repeat("ab", 32),
	})
	client := newConnectTestClient(t, clientConfig{SupervisorURL: url, EnrollToken: "tok"})

	err := client.connect()
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
	url := startBoundSupervisor(t, remote.DeriveEnrollmentAuthKey("tok"), remote.AuthResponsePayload{
		Status: "authenticated", DeviceID: "dev-1", ReadOnly: &readOnly, AllowedPaths: []string{"/"},
	})
	client := newConnectTestClient(t, clientConfig{SupervisorURL: url, EnrollToken: "tok"})
	client.readOnly = true

	err := client.connect()
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

// looseAuthenticatedPayload is an "authenticated" answer that would lift
// read-only mode and open every path if the agent applied it.
func looseAuthenticatedPayload() remote.AuthResponsePayload {
	readOnly := false
	return remote.AuthResponsePayload{Status: "authenticated", DeviceID: "dev-1", ReadOnly: &readOnly, AllowedPaths: []string{"/"}}
}

func assertRestrictedSettingsKept(t *testing.T, client *Client) {
	t.Helper()
	client.stateMu.RLock()
	defer client.stateMu.RUnlock()
	if !client.readOnly || len(client.allowedPaths) != 1 || client.allowedPaths[0] != "/safe" {
		t.Fatalf("rejected auth response must not change settings: read_only=%v allowed_paths=%v", client.readOnly, client.allowedPaths)
	}
}

func newRestrictedReconnectClient(t *testing.T, url, key string) *Client {
	t.Helper()
	client := newConnectTestClient(t, clientConfig{SupervisorURL: url, DeviceID: "dev-1", SharedKey: key})
	client.readOnly = true
	client.allowedPaths = []string{"/safe"}
	return client
}

func TestConnectRejectsStaleSignedAuthResponse(t *testing.T) {
	isolateRemoteHome(t)
	key := strings.Repeat("ab", 32)
	bound := boundAuthResponse(t, key, looseAuthenticatedPayload())
	url, _ := startScriptedSupervisor(t, func(auth remote.RemoteMessage) *remote.RemoteMessage {
		resp := bound(auth)
		resp.Timestamp = time.Now().Add(-remote.MaxTimestampDrift - time.Minute).UTC().Format(time.RFC3339)
		if err := remote.SignMessage(resp, key); err != nil {
			t.Error(err)
			return nil
		}
		return resp
	})
	client := newRestrictedReconnectClient(t, url, key)

	err := client.connect()
	if err == nil || err.Error() != "stale or malformed auth response" {
		t.Fatalf("expected the stale auth response to be rejected, got %v", err)
	}
	assertRestrictedSettingsKept(t, client)
}

// A signed answer is only valid for the auth frame it echoes. One that names a
// different request, or none, is refused before anything from it is applied.
func TestConnectRejectsAuthResponseNotBoundToRequest(t *testing.T) {
	key := strings.Repeat("ab", 32)
	for name, requestNonce := range map[string]string{
		"foreign request nonce": "0123456789abcdef0123456789abcdef",
		"no request nonce":      "",
	} {
		t.Run(name, func(t *testing.T) {
			isolateRemoteHome(t)
			payload := looseAuthenticatedPayload()
			payload.RequestNonce = requestNonce
			resp, err := remote.NewAuthResponseMessage("dev-1", key, payload)
			if err != nil {
				t.Fatal(err)
			}
			client := newRestrictedReconnectClient(t, startFakeSupervisor(t, resp), key)

			err = client.connect()
			if err == nil || err.Error() != "auth response not bound to this request" {
				t.Fatalf("expected the unbound auth response to be rejected, got %v", err)
			}
			assertRestrictedSettingsKept(t, client)
		})
	}

	t.Run("enrolled answer for another request", func(t *testing.T) {
		isolateRemoteHome(t)
		payload := remote.AuthResponsePayload{Status: "enrolled", DeviceID: "dev-1", SharedKey: strings.Repeat("cd", 32), RequestNonce: "0123456789abcdef0123456789abcdef"}
		resp, err := remote.NewAuthResponseMessage("dev-1", remote.DeriveEnrollmentAuthKey("tok"), payload)
		if err != nil {
			t.Fatal(err)
		}
		client := newConnectTestClient(t, clientConfig{SupervisorURL: startFakeSupervisor(t, resp), EnrollToken: "tok"})
		err = client.connect()
		if err == nil || err.Error() != "auth response not bound to this request" {
			t.Fatalf("expected the unbound enrolled answer to be rejected, got %v", err)
		}
		if client.cfg.DeviceID != "" || client.cfg.SharedKey != "" || client.cfg.EnrollToken != "tok" {
			t.Fatalf("unbound enrolled answer must not change config: %+v", client.cfg)
		}
		assertNoStoredConfig(t)
	})
}

// A captured, correctly signed answer cannot be replayed on a later connect:
// every connect sends a fresh auth nonce, which the old answer does not echo.
func TestConnectRejectsCapturedAuthResponseOnLaterConnect(t *testing.T) {
	isolateRemoteHome(t)
	key := strings.Repeat("ab", 32)
	bound := boundAuthResponse(t, key, looseAuthenticatedPayload())
	var mu sync.Mutex
	var captured *remote.RemoteMessage
	url, frames := startScriptedSupervisor(t, func(auth remote.RemoteMessage) *remote.RemoteMessage {
		mu.Lock()
		defer mu.Unlock()
		if captured == nil {
			captured = bound(auth)
		}
		return captured
	})
	client := newConnectTestClient(t, clientConfig{SupervisorURL: url, DeviceID: "dev-1", SharedKey: key})
	if err := client.connect(); err != nil {
		t.Fatalf("a bound auth response must be accepted: %v", err)
	}
	first := receiveAuthFrame(t, frames)

	// The admin then restricts the device; a forced reconnect must not let the
	// captured answer undo that.
	client.stateMu.Lock()
	client.readOnly = true
	client.allowedPaths = []string{"/safe"}
	client.stateMu.Unlock()

	err := client.connect()
	if err == nil || err.Error() != "auth response not bound to this request" {
		t.Fatalf("expected the replayed auth response to be rejected, got %v", err)
	}
	if second := receiveAuthFrame(t, frames); second.Nonce == first.Nonce {
		t.Fatal("each connect must send a fresh auth nonce")
	}
	assertRestrictedSettingsKept(t, client)
}

// runEventLog records, in order, the fake supervisor's dials and the Run
// loop's backoff waits.
type runEventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *runEventLog) add(event string) {
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
}

func (l *runEventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.events...)
}

// An on-path attacker can hang up every session right after auth. Frames that
// reach the agent go into its fail-closed replay cache, so Run must back off
// after a short session and reset the backoff only after a stable one.
func TestRunBacksOffAfterShortSessionsAndResetsAfterStableOne(t *testing.T) {
	isolateRemoteHome(t)
	key := strings.Repeat("ab", 32)
	log := &runEventLog{}

	// The fake supervisor authenticates every dial with a fresh signed reply
	// bound to that dial's auth frame and hangs up immediately.
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		log.add("dial")
		var auth remote.RemoteMessage
		if err := conn.ReadJSON(&auth); err != nil {
			return
		}
		reply, err := remote.NewAuthResponseMessage("dev-1", key, remote.AuthResponsePayload{Status: "authenticated", DeviceID: "dev-1", RequestNonce: auth.Nonce})
		if err != nil {
			return
		}
		_ = conn.WriteJSON(reply)
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	t.Cleanup(srv.Close)

	// Scripted session lengths: short, short, stable, short. Run reads the
	// clock once when a session starts and once when it ends.
	sessionLengths := []time.Duration{0, 0, minStableSession + time.Minute, 0}
	clock := time.Unix(1_700_000_000, 0)
	clockCalls := 0
	prevNow, prevAfter := nowFn, afterFn
	nowFn = func() time.Time {
		if clockCalls%2 == 1 && clockCalls/2 < len(sessionLengths) {
			clock = clock.Add(sessionLengths[clockCalls/2])
		}
		clockCalls++
		return clock
	}
	waits := 0
	afterFn = func(d time.Duration) <-chan time.Time {
		log.add("wait " + d.String())
		waits++
		if waits >= 3 {
			return nil // park Run here until Stop
		}
		fired := make(chan time.Time, 1)
		fired <- time.Time{}
		return fired
	}
	t.Cleanup(func() { nowFn, afterFn = prevNow, prevAfter })

	client := &Client{
		cfg:    clientConfig{SupervisorURL: "ws" + strings.TrimPrefix(srv.URL, "http"), DeviceID: "dev-1", SharedKey: key},
		logger: slog.Default(),
		done:   make(chan struct{}),
	}
	finished := make(chan struct{})
	go func() {
		client.Run()
		close(finished)
	}()

	want := []string{"dial", "wait 5s", "dial", "wait 10s", "dial", "dial", "wait 5s"}
	deadline := time.Now().Add(10 * time.Second)
	for len(log.snapshot()) < len(want) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	client.Stop()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after Stop")
	}

	got := log.snapshot()
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Fatalf("reconnect sequence:\n got  %v\n want %v", got, want)
	}
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
	if reason := client.rejectReplayedFrame(*fresh); reason != "nonce replayed or replay cache full" {
		t.Fatalf("identical nonce must be rejected as a replay, got %q", reason)
	}

	foreign := *fresh
	foreign.DeviceID = "dev-2"
	foreign.Nonce = "0123456789abcdef0123456789abcdef"
	if reason := client.rejectReplayedFrame(foreign); reason != "device_id mismatch" {
		t.Fatalf("frame for another device must be rejected, got %q", reason)
	}

	// Every supervisor frame carries the device id, so a frame without one is
	// not bound to this agent.
	unbound := *fresh
	unbound.DeviceID = ""
	unbound.Nonce = "00112233445566778899aabbccddeeff"
	if reason := client.rejectReplayedFrame(unbound); reason != "device_id mismatch" {
		t.Fatalf("frame without a device id must be rejected, got %q", reason)
	}

	stale := *fresh
	stale.Nonce = "fedcba9876543210fedcba9876543210"
	stale.Timestamp = time.Now().Add(-remote.MaxTimestampDrift - time.Minute).UTC().Format(time.RFC3339)
	if reason := client.rejectReplayedFrame(stale); !strings.HasPrefix(reason, "timestamp drift ") ||
		!strings.HasSuffix(reason, "exceeds maximum "+remote.MaxTimestampDrift.String()) {
		t.Fatalf("stale frame must be rejected for timestamp drift, got %q", reason)
	}
}

// Under the old undelimited HMAC form a captured frame with seq=12 still
// verified as seq=1 with nonce "2"+nonce. The canonical form makes that copy
// fail verification; the nonce format check stays as a second line.
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
	if ok, _ := remote.VerifyMessage(shifted, key); ok {
		t.Fatal("a sequence digit shifted into the nonce must not keep a valid HMAC")
	}
	if reason := client.rejectReplayedFrame(shifted); reason != "invalid nonce format" {
		t.Fatalf("shifted frame must be rejected for its nonce format, got %q", reason)
	}
}

// The agent's cache must never evict a live nonce to make room: an attacker who
// can get enough fresh signed frames delivered (for example the supervisor's
// error replies) would otherwise flush it and replay an older captured frame.
func TestRejectReplayedFrameFailsClosedWhenCacheFull(t *testing.T) {
	key := strings.Repeat("ab", 32)
	client := &Client{cfg: clientConfig{DeviceID: "dev-1", SharedKey: key}, logger: slog.Default()}
	captured, err := remote.NewMessage(remote.MsgConfigUpdate, "dev-1", key, 1, remote.ConfigUpdatePayload{})
	if err != nil {
		t.Fatal(err)
	}
	if reason := client.rejectReplayedFrame(*captured); reason != "" {
		t.Fatalf("fresh frame must pass, got %q", reason)
	}

	now := time.Now()
	for i := 1; i < agentReplayCacheEntries; i++ {
		if client.replay.Seen("dev-1", fmt.Sprintf("%032x", i), now) {
			t.Fatalf("filler nonce %d unexpectedly reported as seen", i)
		}
	}

	flood, err := remote.NewMessage(remote.MsgCommand, "dev-1", key, 2, remote.CommandPayload{CommandID: "cmd-flood"})
	if err != nil {
		t.Fatal(err)
	}
	if reason := client.rejectReplayedFrame(*flood); reason != "nonce replayed or replay cache full" {
		t.Fatalf("a new frame must be dropped while the cache is full, got %q", reason)
	}
	if reason := client.rejectReplayedFrame(*captured); reason != "nonce replayed or replay cache full" {
		t.Fatalf("the captured frame must stay rejected after the cache filled up, got %q", reason)
	}
}

// The supervisor answers every injected bad frame with a signed error frame.
// The agent only logs those, so it must not spend replay-cache entries on them,
// but they still have to pass the device, nonce-format and timestamp checks.
func TestRejectReplayedFrameDoesNotCacheErrorFrames(t *testing.T) {
	key := strings.Repeat("ab", 32)
	client := &Client{cfg: clientConfig{DeviceID: "dev-1", SharedKey: key}, logger: slog.Default()}
	client.replay = remote.NewFailClosedNonceReplayCache(remote.NonceReplayTTL, 1)
	errFrame, err := remote.NewMessage(remote.MsgError, "dev-1", key, 1, remote.ErrorPayload{Code: "invalid_hmac"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if reason := client.rejectReplayedFrame(*errFrame); reason != "" {
			t.Fatalf("delivery %d: error frames are not replay-cached, got %q", i+1, reason)
		}
	}
	// The single cache slot is still free for a real frame.
	command, err := remote.NewMessage(remote.MsgCommand, "dev-1", key, 2, remote.CommandPayload{CommandID: "cmd-1"})
	if err != nil {
		t.Fatal(err)
	}
	if reason := client.rejectReplayedFrame(*command); reason != "" {
		t.Fatalf("error frames must not use up cache space, got %q", reason)
	}

	foreign := *errFrame
	foreign.DeviceID = "dev-2"
	if reason := client.rejectReplayedFrame(foreign); reason != "device_id mismatch" {
		t.Fatalf("error frame for another device must be rejected, got %q", reason)
	}
	malformed := *errFrame
	malformed.Nonce = "2" + errFrame.Nonce
	if reason := client.rejectReplayedFrame(malformed); reason != "invalid nonce format" {
		t.Fatalf("error frame with a malformed nonce must be rejected, got %q", reason)
	}
	stale := *errFrame
	stale.Timestamp = time.Now().Add(-remote.MaxTimestampDrift - time.Minute).UTC().Format(time.RFC3339)
	if reason := client.rejectReplayedFrame(stale); !strings.HasPrefix(reason, "timestamp drift ") {
		t.Fatalf("stale error frame must be rejected, got %q", reason)
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

// hmacWithRawKey computes a frame's HMAC the way remote.SignMessage does, but
// with a raw key, so tests can build the empty-key signature SignMessage now
// refuses to produce. It mirrors the version 2 canonical form: every field as
// <decimal length>:<bytes>.
func hmacWithRawKey(frame remote.RemoteMessage, key []byte) string {
	var data strings.Builder
	for _, field := range []string{
		strconv.Itoa(frame.Version), frame.Type, frame.DeviceID, frame.MessageID,
		strconv.FormatUint(frame.Sequence, 10), frame.Nonce, frame.Timestamp, string(frame.Payload),
	} {
		data.WriteString(strconv.Itoa(len(field)) + ":" + field)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data.String()))
	return hex.EncodeToString(mac.Sum(nil))
}

// legacySignedFrame re-signs frame in the pre-version-2 undelimited form, as a
// supervisor built before the canonical HMAC form would.
func legacySignedFrame(t *testing.T, frame *remote.RemoteMessage, keyHex string) *remote.RemoteMessage {
	t.Helper()
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		t.Fatal(err)
	}
	legacy := *frame
	legacy.Version = 0
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(legacy.Type + legacy.DeviceID + legacy.MessageID +
		strconv.FormatUint(legacy.Sequence, 10) + legacy.Nonce + legacy.Timestamp + string(legacy.Payload)))
	legacy.HMAC = hex.EncodeToString(mac.Sum(nil))
	return &legacy
}

// A frame correctly signed in the old undelimited form is not trusted: the
// agent only verifies the current frame version.
func TestReadMessagesDropsUnversionedFrames(t *testing.T) {
	isolateRemoteHome(t)
	sharedKey := strings.Repeat("ab", 32)
	current, err := remote.NewMessage(remote.MsgCommand, "dev-1", sharedKey, 1, remote.CommandPayload{CommandID: "cmd-current"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := remote.NewMessage(remote.MsgCommand, "dev-1", sharedKey, 2, remote.CommandPayload{CommandID: "cmd-old"})
	if err != nil {
		t.Fatal(err)
	}
	client := newConnectTestClient(t, clientConfig{DeviceID: "dev-1", SharedKey: sharedKey})
	client.conn = dialFrameSupervisor(t, legacySignedFrame(t, old, sharedKey), current)

	dispatched := readMessagesDispatched(t, client)
	if len(dispatched) != 1 || dispatched[0].Nonce != current.Nonce {
		t.Fatalf("only the current-version frame may be dispatched, got %d frames", len(dispatched))
	}
}

// An agent talking to a supervisor that predates the canonical form fails to
// connect instead of trusting an answer in the old form.
func TestConnectRejectsUnversionedAuthResponse(t *testing.T) {
	isolateRemoteHome(t)
	key := strings.Repeat("ab", 32)
	bound := boundAuthResponse(t, key, looseAuthenticatedPayload())
	url, _ := startScriptedSupervisor(t, func(auth remote.RemoteMessage) *remote.RemoteMessage {
		return legacySignedFrame(t, bound(auth), key)
	})
	client := newRestrictedReconnectClient(t, url, key)

	err := client.connect()
	if err == nil || !strings.Contains(err.Error(), "unsupported frame version") {
		t.Fatalf("expected the unversioned auth response to be rejected, got %v", err)
	}
	assertRestrictedSettingsKept(t, client)
}

func TestReadMessagesFailsClosedWithoutSharedKey(t *testing.T) {
	isolateRemoteHome(t)
	realKey := strings.Repeat("ab", 32)
	reference, err := remote.NewMessage(remote.MsgCommand, "dev-1", realKey, 1, remote.CommandPayload{CommandID: "cmd-reference"})
	if err != nil {
		t.Fatal(err)
	}
	realKeyBytes, _ := hex.DecodeString(realKey)
	if hmacWithRawKey(*reference, realKeyBytes) != reference.HMAC {
		t.Fatal("hmacWithRawKey no longer matches remote.SignMessage; update it with the canonical HMAC form")
	}

	// An HMAC under the empty key is computable by anyone; it must not let a
	// frame through to an agent that holds no device key.
	emptyKeySigned, err := remote.NewMessage(remote.MsgCommand, "dev-1", "", 1, remote.CommandPayload{CommandID: "cmd-empty-key"})
	if err != nil {
		t.Fatal(err)
	}
	emptyKeySigned.HMAC = hmacWithRawKey(*emptyKeySigned, nil)
	client := newConnectTestClient(t, clientConfig{DeviceID: "dev-1"})
	client.conn = dialFrameSupervisor(t, emptyKeySigned)

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
