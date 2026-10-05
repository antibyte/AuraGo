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
	if err == nil || !strings.Contains(err.Error(), "unsigned") {
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
		ReadOnly: &readOnly, AllowedPaths: []string{"/"},
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
	assertNoStoredConfig(t)
}

func TestConnectRejectsEnrolledWithInvalidSharedKey(t *testing.T) {
	isolateRemoteHome(t)
	resp, err := remote.NewMessage(remote.MsgAuthResponse, "dev-1", remote.DeriveEnrollmentAuthKey("tok"), 1, remote.AuthResponsePayload{
		Status: "enrolled", DeviceID: "dev-1", SharedKey: "nothex",
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
