package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/virtualcomputers"
)

func TestVirtualComputersAgentControlGateRequiresVerifiedAssets(t *testing.T) {
	originalLoad := virtualComputersLoadSetupState
	defer func() { virtualComputersLoadSetupState = originalLoad }()

	oldCfg := config.Config{}
	newCfg := config.Config{}
	newCfg.VirtualComputers.Enabled = true
	newCfg.Tools.VirtualComputers.Enabled = true
	newCfg.VirtualComputers.AgentControl.Enabled = true

	virtualComputersLoadSetupState = func() (virtualComputersSetupState, error) {
		return virtualComputersSetupState{}, nil
	}
	if err := virtualComputersEnforceAgentControlGate(nil, oldCfg, newCfg); err == nil {
		t.Fatal("unverified workspace assets unexpectedly passed activation gate")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/workspace/capabilities" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(virtualcomputers.WorkspaceControlPlaneStatus{
			ProtocolVersion:  virtualcomputers.WorkspaceProtocolVersion,
			AssetFingerprint: virtualcomputers.WorkspaceAssetFingerprint(),
		})
	}))
	defer server.Close()
	newCfg.VirtualComputers.ControlPlane.BoringdURL = server.URL
	virtualComputersLoadSetupState = func() (virtualComputersSetupState, error) {
		return virtualComputersSetupState{
			WorkspaceAssetFingerprint: virtualcomputers.WorkspaceAssetFingerprint(),
			WorkspaceVerifiedAt:       time.Now().UTC(),
		}, nil
	}
	if err := virtualComputersEnforceAgentControlGate(nil, oldCfg, newCfg); err != nil {
		t.Fatalf("verified workspace assets rejected: %v", err)
	}
}
