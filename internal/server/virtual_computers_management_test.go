package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/virtualcomputers"
)

func TestVirtualComputersManagementLocalAccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != virtualcomputers.ManagementBasePath+"/" {
			t.Fatalf("health path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	restore := setVirtualComputersManagementTestHooks(t, upstream.URL)
	defer restore()
	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneLocalHost}}

	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("ensure local management access: %v", err)
	}
	if !virtualComputersManagementHealthy(&Server{}, cfg) {
		t.Fatal("local management service should be healthy")
	}
}

func TestVirtualComputersManagementRemoteTunnelReuseAndReplacement(t *testing.T) {
	restore := setVirtualComputersManagementTestHooks(t, "http://127.0.0.1:18081")
	defer restore()

	started := false
	starts := 0
	closes := 0
	virtualComputersManagementHealthProbe = func(string) bool { return started }
	virtualComputersManagementSSHExecutor = func(_ *Server, cfg virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
		return virtualComputersSSHExecutor{Host: cfg.ControlPlane.Host, Port: 22}, nil
	}
	virtualComputersManagementTunnelStarter = func(_ virtualComputersSSHExecutor, localAddr, remoteAddr string, _ *Server) (func(), error) {
		starts++
		started = true
		if localAddr != "127.0.0.1:18081" || remoteAddr != virtualcomputers.ManagementListenAddr {
			t.Fatalf("tunnel = %q -> %q", localAddr, remoteAddr)
		}
		return func() { closes++; started = false }, nil
	}

	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneSSHHost, Host: "first.example"}}
	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("first access: %v", err)
	}
	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("reused access: %v", err)
	}
	if starts != 1 || closes != 0 {
		t.Fatalf("reuse starts=%d closes=%d", starts, closes)
	}

	cfg.ControlPlane.Host = "second.example"
	if err := virtualComputersEnsureManagementAccess(&Server{}, cfg); err != nil {
		t.Fatalf("replacement access: %v", err)
	}
	if starts != 2 || closes != 1 {
		t.Fatalf("replacement starts=%d closes=%d", starts, closes)
	}
}

func TestVirtualComputersManagementFailedHealthClosesNewTunnel(t *testing.T) {
	restore := setVirtualComputersManagementTestHooks(t, "http://127.0.0.1:18081")
	defer restore()

	closes := 0
	virtualComputersManagementHealthProbe = func(string) bool { return false }
	virtualComputersManagementSSHExecutor = func(_ *Server, _ virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
		return virtualComputersSSHExecutor{Host: "remote.example", Port: 22}, nil
	}
	virtualComputersManagementTunnelStarter = func(_ virtualComputersSSHExecutor, _, _ string, _ *Server) (func(), error) {
		return func() { closes++ }, nil
	}
	cfg := virtualcomputers.ToolConfig{ControlPlane: virtualcomputers.ControlPlaneConfig{Mode: virtualcomputers.ControlPlaneSSHHost, Host: "remote.example"}}

	err := virtualComputersEnsureManagementAccess(&Server{}, cfg)
	if err == nil || err.Error() != "Boring Computers management service is unavailable; run setup install or repair" {
		t.Fatalf("error = %v", err)
	}
	if closes != 1 {
		t.Fatalf("new tunnel close count = %d", closes)
	}
	virtualComputersManagementTunnel.Lock()
	defer virtualComputersManagementTunnel.Unlock()
	if virtualComputersManagementTunnel.close != nil || virtualComputersManagementTunnel.key != "" {
		t.Fatalf("failed tunnel retained: key=%q close=%v", virtualComputersManagementTunnel.key, virtualComputersManagementTunnel.close != nil)
	}
}

func setVirtualComputersManagementTestHooks(t *testing.T, managementURL string) func() {
	t.Helper()
	oldURL := virtualComputersManagementURL
	oldProbe := virtualComputersManagementHealthProbe
	oldExecutor := virtualComputersManagementSSHExecutor
	oldStarter := virtualComputersManagementTunnelStarter
	virtualComputersManagementURL = managementURL
	virtualComputersManagementHealthProbe = func(rawURL string) bool {
		resp, err := http.Get(virtualcomputers.ManagementHealthURL(rawURL))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	}
	virtualComputersManagementSSHExecutor = func(_ *Server, _ virtualcomputers.ToolConfig) (virtualComputersSSHExecutor, error) {
		return virtualComputersSSHExecutor{}, fmt.Errorf("unexpected SSH executor request")
	}
	virtualComputersManagementTunnelStarter = func(_ virtualComputersSSHExecutor, _, _ string, _ *Server) (func(), error) {
		return nil, fmt.Errorf("unexpected tunnel start")
	}
	resetVirtualComputersManagementTunnel()
	return func() {
		resetVirtualComputersManagementTunnel()
		virtualComputersManagementURL = oldURL
		virtualComputersManagementHealthProbe = oldProbe
		virtualComputersManagementSSHExecutor = oldExecutor
		virtualComputersManagementTunnelStarter = oldStarter
	}
}
