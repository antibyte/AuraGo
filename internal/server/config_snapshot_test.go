package server

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/remote"
)

func TestReplaceConfigStoresNewSnapshotWithoutMutatingOldConfig(t *testing.T) {
	oldCfg := &config.Config{}
	oldCfg.Server.Port = 1111
	s := &Server{Cfg: oldCfg}
	s.initConfigSnapshot()

	newCfg := &config.Config{}
	newCfg.Server.Port = 2222
	s.replaceConfigSnapshot(newCfg)

	if got := s.ConfigSnapshot(); got != newCfg {
		t.Fatalf("ConfigSnapshot returned %p, want new config %p", got, newCfg)
	}
	if s.Cfg != newCfg {
		t.Fatalf("compat Cfg pointer = %p, want %p", s.Cfg, newCfg)
	}
	if oldCfg.Server.Port != 1111 {
		t.Fatalf("old config was mutated: port=%d", oldCfg.Server.Port)
	}
	resolver := s.ConfigSnapshot().AuthorizationSnapshots
	if resolver == nil {
		t.Fatal("published config lost authorization snapshot resolver")
	}
	baseline, current := resolver()
	if baseline != newCfg || current != newCfg {
		t.Fatalf("authorization resolver = (%p, %p), want (%p, %p)", baseline, current, newCfg, newCfg)
	}
}

// A changed remote_control.allowed_paths is pushed to connected agents on a
// background goroutine. A transport that panics there must not take the
// process down; the failure is logged as an error.
func TestReplaceConfigSnapshotSurvivesAPanickingAllowedPathsPush(t *testing.T) {
	var logs syncBuffer
	oldCfg := &config.Config{}
	oldCfg.RemoteControl.AllowedPaths = []string{"/old"}
	s := &Server{Cfg: oldCfg, RemoteHub: remote.NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(&logs, nil)))}
	s.initConfigSnapshot()
	// A connection without a socket panics on its first write.
	s.RemoteHub.Register("broken", &remote.RemoteConnection{DeviceID: "broken", SharedKey: strings.Repeat("b", 64)})

	newCfg := &config.Config{}
	newCfg.RemoteControl.AllowedPaths = []string{"/srv"}
	s.replaceConfigSnapshot(newCfg)

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "level=ERROR") {
		if time.Now().After(deadline) {
			t.Fatalf("the failed push was not logged as an error:\n%s", logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(logs.String(), "device_id=broken") {
		t.Fatalf("the error must name the device:\n%s", logs.String())
	}
}
