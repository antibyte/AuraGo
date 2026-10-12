package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/remote"
)

func TestAgodeskDeviceSaveOnlyDisconnectsForEffectivePolicyChanges(t *testing.T) {
	for _, tc := range []struct {
		name, body       string
		globalReadOnly   bool
		wantDisconnected bool
	}{
		{name: "empty", body: `{}`},
		{name: "metadata", body: `{"name":"renamed","tags":["agodesk","desktop"]}`},
		{name: "same rights", body: `{"read_only":false,"allowed_paths":[]}`},
		{name: "local paths", body: `{"allowed_paths":["/ignored-by-agodesk"]}`},
		{name: "global restriction masks device change", globalReadOnly: true, body: `{"read_only":true}`},
		{name: "restrict rights", body: `{"read_only":true}`, wantDisconnected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newAgodeskPairingTestServer(t)
			s.RemoteHub.Policy = func() remote.RuntimePolicy {
				rc := s.ConfigSnapshot().RemoteControl
				return remote.RuntimePolicy{ReadOnly: rc.ReadOnly, AllowedPaths: rc.AllowedPaths, MaxFileSizeMB: rc.MaxFileSizeMB}
			}
			if tc.globalReadOnly {
				cfg := *s.ConfigSnapshot()
				cfg.RemoteControl.ReadOnly = true
				s.replaceConfigSnapshot(&cfg)
			}
			_, cleanup, accepted := pairAgodeskTestClient(t, s, "synthetic-device-save-token", nil)
			defer cleanup()
			broker := currentAgodeskDesktopBroker(s)
			session := broker.session(accepted.DeviceID)
			if session == nil {
				t.Fatal("missing paired session")
			}
			runCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			registerAgodeskActiveRun(session.state, "synthetic-running-chat", "test-conversation", cancel)
			recorder := httptest.NewRecorder()
			handleRemoteDevice(s).ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/api/remote/devices/"+accepted.DeviceID, strings.NewReader(tc.body)))
			if recorder.Code != http.StatusOK {
				t.Fatalf("save status %d", recorder.Code)
			}
			disconnected := broker.session(accepted.DeviceID) != session
			if disconnected != tc.wantDisconnected || (runCtx.Err() != nil) != tc.wantDisconnected {
				t.Fatalf("session disconnected=%v, chat cancelled=%v, want both %v", disconnected, runCtx.Err() != nil, tc.wantDisconnected)
			}
			if tc.globalReadOnly {
				device, err := remote.GetDevice(s.RemoteHub.DB(), accepted.DeviceID)
				if err != nil || !device.ReadOnly {
					t.Fatal("masked device preference was not saved")
				}
			}
		})
	}
}
