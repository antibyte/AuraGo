package server

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

func TestVirtualComputersAutoSetupSkipsDisabledConfiguration(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	var runs atomic.Int32
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		runs.Add(1)
		return nil
	}

	if virtualComputersTriggerAutoSetup(&Server{}, virtualcomputers.ToolConfig{AutoSetup: true}) {
		t.Fatal("disabled integration must not start auto setup")
	}
	if virtualComputersTriggerAutoSetup(&Server{}, virtualcomputers.ToolConfig{Enabled: true}) {
		t.Fatal("auto_setup=false must not start auto setup")
	}
	if runs.Load() != 0 {
		t.Fatalf("auto setup runs = %d", runs.Load())
	}
}

func TestVirtualComputersAutoSetupRunsSingleFlight(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var runs atomic.Int32
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		runs.Add(1)
		started <- struct{}{}
		<-release
		return nil
	}
	cfg := virtualcomputers.ToolConfig{Enabled: true, AutoSetup: true}

	if !virtualComputersTriggerAutoSetup(&Server{}, cfg) {
		t.Fatal("first auto setup was not started")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("auto setup runner did not start")
	}
	if virtualComputersTriggerAutoSetup(&Server{}, cfg) {
		t.Fatal("second concurrent auto setup must be coalesced")
	}
	close(release)
	waitForVirtualComputersAutoSetupIdle(t)
	if runs.Load() != 1 {
		t.Fatalf("auto setup runs = %d", runs.Load())
	}
}

func TestRegisterVirtualComputersRoutesTriggersAutoSetup(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{}, 1)
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		started <- struct{}{}
		return nil
	}
	cfg := &config.Config{}
	cfg.VirtualComputers.Enabled = true
	cfg.VirtualComputers.AutoSetup = true

	registerVirtualComputersRoutes(http.NewServeMux(), &Server{Cfg: cfg})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("route registration did not trigger automatic provisioning")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func TestVirtualComputersConfigChangeTriggersAutoSetupWhenEnabled(t *testing.T) {
	restore := setVirtualComputersAutoSetupTestHooks(t)
	defer restore()
	started := make(chan struct{}, 1)
	virtualComputersAutoSetupNeeded = func(*Server, virtualcomputers.ToolConfig) bool { return true }
	virtualComputersAutoSetupRunner = func(context.Context, *Server, virtualcomputers.ToolConfig) error {
		started <- struct{}{}
		return nil
	}
	oldCfg := config.Config{}
	newCfg := config.Config{}
	newCfg.VirtualComputers.Enabled = true
	newCfg.VirtualComputers.AutoSetup = true

	virtualComputersAutoSetupAfterConfigChange(&Server{}, oldCfg, newCfg)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("enabling integration did not trigger automatic provisioning")
	}
	waitForVirtualComputersAutoSetupIdle(t)
}

func setVirtualComputersAutoSetupTestHooks(t *testing.T) func() {
	t.Helper()
	oldNeeded := virtualComputersAutoSetupNeeded
	oldRunner := virtualComputersAutoSetupRunner
	resetVirtualComputersAutoSetupState()
	return func() {
		waitForVirtualComputersAutoSetupIdle(t)
		virtualComputersAutoSetupNeeded = oldNeeded
		virtualComputersAutoSetupRunner = oldRunner
		resetVirtualComputersAutoSetupState()
	}
}

func waitForVirtualComputersAutoSetupIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		virtualComputersAutoSetupState.Lock()
		running := virtualComputersAutoSetupState.running
		virtualComputersAutoSetupState.Unlock()
		if !running {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("auto setup did not become idle")
}
