package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestNativeDesktopEnableRequiresParentAndVerifiedImage(t *testing.T) {
	original := virtualComputersLoadSetupState
	defer func() { virtualComputersLoadSetupState = original }()
	virtualComputersLoadSetupState = func() (virtualComputersSetupState, error) { return virtualComputersSetupState{}, nil }
	oldCfg := config.Config{}
	oldCfg.VirtualComputers.Enabled = true
	oldCfg.Tools.VirtualComputers.Enabled = true
	oldCfg.VirtualComputers.AgentControl.Enabled = true
	newCfg := oldCfg
	newCfg.VirtualComputers.AgentControl.NativeAppsEnabled = true
	if virtualComputersEnforceAgentControlGate(nil, oldCfg, newCfg) == nil {
		t.Fatal("native activation bypassed verified-image gate")
	}
	newCfg.VirtualComputers.AgentControl.Enabled = false
	if virtualComputersEnforceAgentControlGate(nil, oldCfg, newCfg) == nil {
		t.Fatal("native activation bypassed parent gate")
	}
}

func TestNativeDesktopStatusRequiresAuthenticatedDesktopAccess(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	s.Cfg.Auth.Enabled = true
	r := httptest.NewRequest(http.MethodGet, "/api/virtual-computers/workspaces/fixture/desktop/status", nil)
	w := httptest.NewRecorder()
	handleVirtualWorkspace(s).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated diagnostic status=%d", w.Code)
	}
}
