//go:build !remote_minimal

package remote

import (
	"reflect"
	"testing"
	"time"
)

func TestApplyDevicePolicyPreservesWorkUnlessEffectiveSettingsChange(t *testing.T) {
	for _, tc := range []struct {
		name             string
		before, after    DeviceRecord
		globalReadOnly   bool
		wantDisconnected bool
	}{
		{name: "metadata", before: DeviceRecord{Name: "old"}, after: DeviceRecord{Name: "new", Tags: []string{"desktop"}}},
		{name: "same rights", before: DeviceRecord{AllowedPaths: []string{"/default"}}, after: DeviceRecord{AllowedPaths: []string{"/default"}}},
		{name: "equivalent paths", before: DeviceRecord{AllowedPaths: []string{"/default"}}, after: DeviceRecord{AllowedPaths: []string{" /default ", ""}}},
		{name: "clear inherited paths", before: DeviceRecord{AllowedPaths: []string{"/default"}}},
		{name: "masked read only", after: DeviceRecord{ReadOnly: true}, globalReadOnly: true},
		{name: "restrict read only", after: DeviceRecord{ReadOnly: true}, wantDisconnected: true},
		{name: "relax read only", before: DeviceRecord{ReadOnly: true}, wantDisconnected: true},
		{name: "change paths", after: DeviceRecord{AllowedPaths: []string{"/restricted"}}, wantDisconnected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub, db, _ := newEnrollmentTestHub(t)
			hub.Policy = func() RuntimePolicy {
				return RuntimePolicy{ReadOnly: tc.globalReadOnly, AllowedPaths: []string{"/default"}}
			}
			id, err := CreateDevice(db, DeviceRecord{Status: "approved"})
			if err != nil {
				t.Fatal(err)
			}
			tc.before.ID, tc.after.ID = id, id
			server, _, cleanup := newWebSocketPairForHubTest(t)
			defer cleanup()
			conn := &RemoteConnection{Conn: server, DeviceID: id, ReadOnly: tc.before.ReadOnly, AllowedPaths: tc.before.AllowedPaths,
				configState: "confirmed", configRef: "confirmed-settings", configPolicy: hub.devicePolicySignature(tc.before)}
			hub.Register(id, conn)
			defer hub.Unregister(id)
			hub.pendingMu.Lock()
			hub.pendingOwners["active-command"] = conn
			hub.pendingMu.Unlock()
			// Delivery on another device must not delay invalidation or a harmless save.
			hub.configMu.Lock()
			defer hub.configMu.Unlock()
			done := make(chan error, 1)
			go func() { done <- hub.ApplyDevicePolicy(tc.before, tc.after) }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("policy update waited for an unrelated config writer")
			}
			if connected := hub.GetConnection(id) == conn; connected == tc.wantDisconnected {
				t.Fatalf("connected=%v, want disconnected=%v", connected, tc.wantDisconnected)
			}
			if !tc.wantDisconnected {
				conn.mu.Lock()
				defer conn.mu.Unlock()
				if conn.configState != "confirmed" || conn.configRef != "confirmed-settings" {
					t.Fatal("unchanged effective settings lost their confirmation")
				}
				if conn.ReadOnly != tc.after.ReadOnly || !reflect.DeepEqual(conn.AllowedPaths, append([]string{}, tc.after.AllowedPaths...)) {
					t.Fatal("masked device preferences were not retained for future global changes")
				}
			}
		})
	}
}
