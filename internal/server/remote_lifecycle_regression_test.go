package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"aurago/internal/agodesk"
	"aurago/internal/remote"

	"github.com/gorilla/websocket"
)

func TestAgodeskConcurrentPairingConsumesTokenOnce(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	const token = "synthetic-concurrent-pairing-token"
	if _, err := s.RemoteHub.IssueEnrollmentToken(token, "concurrent", time.Now().Add(time.Hour).UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	const clients = 12
	start := make(chan struct{})
	accepted := make(chan bool, clients)
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			r, code, _ := acceptAgodeskSessionStart(s, httptest.NewRequest(http.MethodGet, "https://localhost/api/agodesk/ws", nil), fmt.Sprintf("pair-%d", i), agodesk.SessionStartPayload{PairingToken: token})
			accepted <- code == "" && r.Approved
		}(i)
	}
	close(start)
	wg.Wait()
	close(accepted)
	n := 0
	for ok := range accepted {
		if ok {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("single-use token accepted %d times", n)
	}
	devices, err := remote.ListDevices(s.RemoteHub.DB())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("losing enrollments left %d device records", len(devices))
	}
}

func TestAgodeskRevokeInvalidatesExistingSession(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	_, cleanup, accepted := pairAgodeskTestClient(t, s, "synthetic-revoke-regression-token", nil)
	defer cleanup()
	session := currentAgodeskDesktopBroker(s).session(accepted.DeviceID)
	if session == nil {
		t.Fatal("missing paired session")
	}
	recorder := httptest.NewRecorder()
	handleRemoteDeviceRevoke(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/remote/devices/"+accepted.DeviceID+"/revoke", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("revoke status=%d", recorder.Code)
	}
	session.state.mu.RLock()
	paired := session.state.paired
	session.state.mu.RUnlock()
	if paired || s.RemoteHub.IsConnected(accepted.DeviceID) {
		t.Fatal("revoked session remains authorized")
	}
	device, err := remote.GetDevice(s.RemoteHub.DB(), accepted.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	if device.Status != "revoked" {
		t.Fatalf("disconnect replaced revoked status with %q", device.Status)
	}
}

func TestAgodeskReadOnlyPolicyChangesDisconnectSession(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	cfg := *s.ConfigSnapshot()
	cfg.RemoteControl.ReadOnly = true
	s.replaceConfigSnapshot(&cfg)
	_, cleanup, accepted := pairAgodeskTestClient(t, s, "synthetic-readonly-policy-token", nil)
	defer cleanup()
	if currentAgodeskDesktopBroker(s).session(accepted.DeviceID) == nil {
		t.Fatal("missing readonly session")
	}
	next := cfg
	next.RemoteControl.ReadOnly = false
	s.replaceConfigSnapshot(&next)
	if currentAgodeskDesktopBroker(s).session(accepted.DeviceID) != nil {
		t.Fatal("relaxed policy left stale permissions and capabilities in the session")
	}
}

func remoteTLSTestDialer(srv *httptest.Server) *websocket.Dialer {
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return &websocket.Dialer{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
}

func TestAgodeskLateResultAndCancellationDoNotAffectReplacement(t *testing.T) {
	old := &agodeskConnectionState{paired: true}
	current := &agodeskConnectionState{paired: true}
	result := make(chan agodeskDesktopCommandResult, 1)
	session := &agodeskDesktopSession{deviceID: "device", state: current, pending: map[string]chan agodeskDesktopCommandResult{"same-command": result}}
	broker := &agodeskDesktopBroker{sessions: map[string]*agodeskDesktopSession{"device": session}}
	broker.disconnectSession("device", old)
	if broker.session("device") != session {
		t.Fatal("old cancellation closed replacement")
	}
	payload := agodesk.DesktopResultPayload{CommandID: "same-command"}
	if broker.HandleResult("device", old, payload) {
		t.Fatal("old connection supplied replacement result")
	}
	if !broker.HandleResult("device", current, payload) {
		t.Fatal("current connection result rejected")
	}
}

func TestAgodeskStaleCleanupPreservesReplacementStatus(t *testing.T) {
	s := newAgodeskPairingTestServer(t)
	id, err := remote.CreateDevice(s.RemoteHub.DB(), remote.DeviceRecord{Status: "connected", Tags: []string{"agodesk"}})
	if err != nil {
		t.Fatal(err)
	}
	old, current := &agodeskConnectionState{paired: true}, &agodeskConnectionState{paired: true}
	broker := &agodeskDesktopBroker{hub: s.RemoteHub, sessions: map[string]*agodeskDesktopSession{id: {deviceID: id, state: current}}}
	if broker.UnregisterSession(id, old) {
		t.Fatal("old cleanup removed replacement")
	}
	device, err := remote.GetDevice(s.RemoteHub.DB(), id)
	if err != nil || device.Status != "connected" {
		t.Fatal("old cleanup changed replacement status")
	}
	if !broker.UnregisterSession(id, current) {
		t.Fatal("current cleanup failed")
	}
	device, err = remote.GetDevice(s.RemoteHub.DB(), id)
	if err != nil || device.Status != "offline" {
		t.Fatal("current cleanup did not persist offline status")
	}
}
