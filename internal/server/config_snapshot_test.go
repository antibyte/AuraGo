package server

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/proxy"
	"aurago/internal/remote"
)

func TestReplaceConfigSnapshotUpdatesSecurityProxyManager(t *testing.T) {
	startup := &config.Config{}
	startup.SecurityProxy.Domain = "old.example.com"
	s := &Server{Cfg: startup, ProxyManager: proxy.NewManager(startup, slog.New(slog.NewTextHandler(io.Discard, nil)))}
	s.initConfigSnapshot()

	saved := &config.Config{}
	saved.SecurityProxy.Domain = "new.example.com"
	saved.SecurityProxy.HTTPSPort = 8443
	s.replaceConfigSnapshot(saved)

	if got := s.ProxyManager.Config(); got != saved {
		t.Fatalf("security proxy manager config = %p, want the published snapshot %p", got, saved)
	}
}

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
// background goroutine. A device transport that panics is caught by the hub's
// per-device recover (pushDefaultAllowedPathsTo), which logs the device and
// the stack; the other devices still get their push. This covers the hub's
// recover; TestReplaceConfigSnapshotRecoversAPanicOutsideTheHubWrapper covers
// the server goroutine's own recover.
func TestReplaceConfigSnapshotSurvivesAPanickingDevicePush(t *testing.T) {
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

// A panic in the push itself, outside the hub's per-device wrapper, reaches
// the recover on the server goroutine that replaceConfigSnapshot starts. It
// must be logged with its stack instead of ending the process.
func TestReplaceConfigSnapshotRecoversAPanicOutsideTheHubWrapper(t *testing.T) {
	var logs syncBuffer
	oldCfg := &config.Config{}
	oldCfg.RemoteControl.AllowedPaths = []string{"/old"}
	s := &Server{
		Cfg:       oldCfg,
		Logger:    slog.New(slog.NewTextHandler(&logs, nil)),
		RemoteHub: remote.NewRemoteHub(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil))),
	}
	s.pushRemoteAllowedPaths = func(*remote.RemoteHub) { panic("push exploded outside the per-device wrapper") }
	s.initConfigSnapshot()

	newCfg := &config.Config{}
	newCfg.RemoteControl.AllowedPaths = []string{"/srv"}
	s.replaceConfigSnapshot(newCfg)

	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), "pushing remote allowed paths") {
		if time.Now().After(deadline) {
			t.Fatalf("the server goroutine did not log the recovered panic:\n%s", logs.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	out := logs.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "push exploded") {
		t.Fatalf("recovered panic not logged as an error with its value:\n%s", out)
	}
	if !strings.Contains(out, "stack=") || !strings.Contains(out, "goroutine") {
		t.Fatalf("recovered panic logged without its stack:\n%s", out)
	}
}
