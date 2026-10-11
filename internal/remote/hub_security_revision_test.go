//go:build !remote_minimal

package remote

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSecurityRevisionRefusalPreservesEnrollmentToken(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	const token = "synthetic-security-revision-token"
	id := issueTestEnrollment(t, hub, token)
	for _, revision := range []int{0, ClientSecurityRevision} {
		msg, err := NewMessage(MsgAuth, "", DeriveEnrollmentAuthKey(token), 1, AuthPayload{SecurityRevision: revision, KDF: EnrollmentKDFVersion, TokenHash: DeriveEnrollmentLookupHash(token), Hostname: "old-client"})
		if err != nil {
			t.Fatal(err)
		}
		response := enrollmentTestSocket(t, hub)(msg)
		if response.ErrorCode != ClientUpgradeRequired || response.Status != "rejected" {
			t.Fatalf("client without required revision/capabilities accepted: %+v", response)
		}
	}
	assertEnrollmentUnused(t, db, token, id)
	assertNoDevices(t, db)
}

func TestRemoteWriterDeadlineAndCloseDoNotWaitForWriterLock(t *testing.T) {
	server, _, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	conn := &RemoteConnection{Conn: server}
	conn.writeMu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- conn.SendContext(ctx, &RemoteMessage{}) }()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("blocked writer succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("write lock exceeded command deadline")
	}
	closed := make(chan struct{})
	go func() { conn.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close waits for writer")
	}
	conn.writeMu.Unlock()
}

func TestRemoteSettingsAckBindsMessageAndConnection(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	id, err := CreateDevice(db, DeviceRecord{Name: "ack-test", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	server, peer, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	conn := &RemoteConnection{Conn: server, DeviceID: id, SharedKey: strings.Repeat("ab", 32), Capabilities: ClassicCapabilities()}
	hub.Register(id, conn)
	defer hub.Unregister(id)
	go hub.HandleMessages(conn)
	if err := hub.SendConfigUpdate(id, ConfigUpdatePayload{}); err != nil {
		t.Fatal(err)
	}
	var update RemoteMessage
	if err := peer.ReadJSON(&update); err != nil {
		t.Fatal(err)
	}
	wrong, _ := NewMessage(MsgAck, id, conn.SharedKey, 1, AckPayload{RefID: "another-message", Success: true})
	if err := peer.WriteJSON(wrong); err != nil {
		t.Fatal(err)
	}
	// A heartbeat provides an ordered barrier through the same reader.
	heartbeat, _ := NewMessage(MsgHeartbeat, id, conn.SharedKey, 2, HeartbeatPayload{Hostname: "barrier"})
	if err := peer.WriteJSON(heartbeat); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for conn.TelemetrySnapshot().Hostname != "barrier" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hub.DeviceRuntime(DeviceRecord{ID: id}).ConfigState != "pending" {
		t.Fatal("unbound ACK applied settings")
	}
	ack, _ := NewMessage(MsgAck, id, conn.SharedKey, 3, AckPayload{RefID: update.MessageID, Success: true})
	if err := peer.WriteJSON(ack); err != nil {
		t.Fatal(err)
	}
	for hub.DeviceRuntime(DeviceRecord{ID: id}).ConfigState != "confirmed" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hub.DeviceRuntime(DeviceRecord{ID: id}).ConfigState != "confirmed" {
		t.Fatal("bound ACK did not confirm settings")
	}
	server2, _, cleanup2 := newWebSocketPairForHubTest(t)
	defer cleanup2()
	next := &RemoteConnection{Conn: server2, DeviceID: id, SharedKey: conn.SharedKey}
	hub.Register(id, next)
	hub.unregisterConnection(id, conn)
	if hub.GetConnection(id) != next {
		t.Fatal("old disconnect removed new connection")
	}
}

func TestRemotePolicyChangeCancelsActiveCommand(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	id, err := CreateDevice(db, DeviceRecord{Name: "policy", Status: "approved", AllowedPaths: []string{"/srv"}})
	if err != nil {
		t.Fatal(err)
	}
	server, peer, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	conn := &RemoteConnection{Conn: server, DeviceID: id, SharedKey: strings.Repeat("ab", 32), AllowedPaths: []string{"/srv"}, Capabilities: ClassicCapabilities()}
	hub.Register(id, conn)
	finished := make(chan struct{})
	go func() {
		_, _ = hub.SendCommand(id, CommandPayload{Operation: OpShellExec, Args: map[string]interface{}{"command": "work"}}, time.Minute)
		close(finished)
	}()
	var command RemoteMessage
	if err := peer.ReadJSON(&command); err != nil {
		t.Fatal(err)
	}
	ro := true
	if err := hub.SendConfigUpdate(id, ConfigUpdatePayload{ReadOnly: &ro}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("policy change left command active")
	}
	if hub.IsConnected(id) {
		t.Fatal("old authorization retained live transport")
	}
}

func TestRevocationSurvivesStaleDeviceUpdate(t *testing.T) {
	_, db, _ := newEnrollmentTestHub(t)
	id, err := CreateDevice(db, DeviceRecord{Status: "connected", Name: "before"})
	if err != nil {
		t.Fatal(err)
	}
	stale, err := GetDevice(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateDeviceStatus(db, id, "revoked"); err != nil {
		t.Fatal(err)
	}
	stale.Name = "after"
	if err := UpdateDevice(db, stale); err != nil {
		t.Fatal(err)
	}
	current, err := GetDevice(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != "revoked" {
		t.Fatal("stale device update revived revoked identity")
	}
}

func TestReconnectMetadataCannotRestoreStalePermissions(t *testing.T) {
	_, db, _ := newEnrollmentTestHub(t)
	id, err := CreateDevice(db, DeviceRecord{Status: "connected", AllowedPaths: []string{"/old"}})
	if err != nil {
		t.Fatal(err)
	}
	stale, err := GetDevice(db, id)
	if err != nil {
		t.Fatal(err)
	}
	updated := stale
	updated.ReadOnly, updated.AllowedPaths = true, []string{"/restricted"}
	if err := UpdateDevice(db, updated); err != nil {
		t.Fatal(err)
	}
	stale.Hostname = "current-host"
	if err := UpdateDeviceMetadata(db, stale); err != nil {
		t.Fatal(err)
	}
	current, err := GetDevice(db, id)
	if err != nil || !current.ReadOnly || len(current.AllowedPaths) != 1 || current.AllowedPaths[0] != "/restricted" || current.Hostname != "current-host" {
		t.Fatal("reconnect metadata replaced current permissions")
	}
	if err := UpdateDeviceStatus(db, id, "revoked"); err != nil {
		t.Fatal(err)
	}
	if err := UpdateDeviceMetadata(db, stale); err == nil {
		t.Fatal("revoked reconnect was accepted")
	}
}

func TestGlobalReadOnlyDoesNotOverwriteDevicePreference(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	hub.Policy = func() RuntimePolicy { return RuntimePolicy{ReadOnly: true} }
	const token = "synthetic-readonly-enrollment-token"
	issueTestEnrollment(t, hub, token)
	response := enrollmentTestSocket(t, hub)(enrollmentFrame(t, token, 1))
	if response.Status != "enrolled" || response.ReadOnly == nil || !*response.ReadOnly {
		t.Fatal("global restriction was not applied at authentication")
	}
	device, err := GetDevice(db, response.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	if device.ReadOnly {
		t.Fatal("global restriction overwrote the device preference")
	}
	hub.Policy = func() RuntimePolicy { return RuntimePolicy{} }
	if hub.DeviceRuntime(device).EffectiveReadOnly {
		t.Fatal("removed global restriction remains effective")
	}
	device.ReadOnly = true
	if !hub.DeviceRuntime(device).EffectiveReadOnly {
		t.Fatal("device-specific restriction was lost")
	}
}

func TestRemoteUnconfirmedSettingsDisconnectAfterTenSeconds(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	id, err := CreateDevice(db, DeviceRecord{Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	server, peer, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	conn := &RemoteConnection{Conn: server, DeviceID: id, SharedKey: strings.Repeat("ab", 32)}
	hub.Register(id, conn)
	if err := hub.SendConfigUpdate(id, ConfigUpdatePayload{}); err != nil {
		t.Fatal(err)
	}
	var frame RemoteMessage
	if err := peer.ReadJSON(&frame); err != nil {
		t.Fatal(err)
	}
	select {
	case <-conn.done():
	case <-time.After(11 * time.Second):
		t.Fatal("missing ACK did not close connection")
	}
	if hub.DeviceRuntime(DeviceRecord{ID: id}).ConfigState != "error" {
		t.Fatal("ACK failure is not visible after disconnect")
	}
}

func TestDevicePolicyInvalidationDoesNotWaitForOtherConfigWrites(t *testing.T) {
	hub, db, _ := newEnrollmentTestHub(t)
	id, err := CreateDevice(db, DeviceRecord{Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	server, _, cleanup := newWebSocketPairForHubTest(t)
	defer cleanup()
	conn := &RemoteConnection{Conn: server, DeviceID: id}
	hub.Register(id, conn)
	hub.pendingMu.Lock()
	hub.pendingOwners["active-command"] = conn
	hub.pendingMu.Unlock()
	hub.configMu.Lock()
	defer hub.configMu.Unlock()
	done := make(chan struct{})
	go func() { hub.InvalidateDevicePolicy(id); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rights invalidation waited for unrelated config delivery")
	}
	if hub.IsConnected(id) {
		t.Fatal("active execution retained its transport")
	}
}
