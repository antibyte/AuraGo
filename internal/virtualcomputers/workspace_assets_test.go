package virtualcomputers

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestWorkspaceAssetsArePinnedAndComplete(t *testing.T) {
	fingerprint := WorkspaceAssetFingerprint()
	if len(fingerprint) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(fingerprint))
	}
	if _, err := hex.DecodeString(fingerprint); err != nil {
		t.Fatalf("fingerprint is not hexadecimal: %v", err)
	}

	patchScript := workspacePatchInstallSnippet()
	for _, required := range []string{
		workspaceServerSHA256,
		workspaceTemplateSHA256,
		workspaceMachineVolumeSHA256,
		workspaceMachineSHA256,
		"0001-workspace-vsock-proxy.patch",
		"0002-python-template-vsock.patch",
		"0003-workspace-v2-volumes.patch",
		"0004-per-workspace-network-policy.patch",
		"0005-workspace-capability-status.patch",
	} {
		if !strings.Contains(patchScript, required) {
			t.Fatalf("patch install script is missing %q", required)
		}
	}
	proxyPatch, err := workspaceAssets.ReadFile("patches/0001-workspace-vsock-proxy.patch")
	if err != nil || !strings.Contains(string(proxyPatch), "query token is not accepted") {
		t.Fatalf("workspace proxy patch does not enforce header-only authentication: %v", err)
	}
	networkPatch, err := workspaceAssets.ReadFile("patches/0004-per-workspace-network-policy.patch")
	if err != nil || !strings.Contains(string(networkPatch), `"--dport", "25"`) || !strings.Contains(string(networkPatch), `"--hashlimit-name", rateName`) {
		t.Fatalf("workspace LAN policy does not retain SMTP and connection-rate protections: %v", err)
	}

	guestScript := workspaceGuestInstallSnippet()
	for _, required := range []string{"go.mod", "go.sum", "aurago-workspace-agent", "/workspace", "/run/aurago", "Remove the upstream unmanaged Chromium launch", "boring-init.aurago"} {
		if !strings.Contains(guestScript, required) {
			t.Fatalf("guest install script is missing %q", required)
		}
	}
	guestSource, err := workspaceAssets.ReadFile("guest_workspace_agent/main.go")
	if err != nil || !strings.Contains(string(guestSource), WorkspaceProtocolVersion) {
		t.Fatalf("embedded guest source does not declare protocol %q: %v", WorkspaceProtocolVersion, err)
	}
}

func TestValidateWorkspaceNetworkCIDRs(t *testing.T) {
	for _, valid := range [][]string{{}, {"10.10.0.0/16"}, {"172.31.255.0/24", "192.168.40.7/32"}} {
		if err := ValidateWorkspaceNetworkCIDRs(valid); err != nil {
			t.Fatalf("ValidateWorkspaceNetworkCIDRs(%v): %v", valid, err)
		}
	}
	for _, invalid := range [][]string{
		{""}, {"not-a-cidr"}, {"8.8.8.0/24"}, {"10.0.0.0/7"}, {"127.0.0.0/8"}, {"169.254.0.0/16"}, {"fc00::/7"},
	} {
		if err := ValidateWorkspaceNetworkCIDRs(invalid); err == nil {
			t.Fatalf("ValidateWorkspaceNetworkCIDRs(%v) unexpectedly succeeded", invalid)
		}
	}
}

func TestLocalProtectedCIDRsAreIPv4Hosts(t *testing.T) {
	protected := localProtectedCIDRs()
	if len(protected) == 0 {
		t.Fatal("local protected CIDRs are empty")
	}
	for _, value := range protected {
		if !strings.HasSuffix(value, "/32") {
			t.Fatalf("protected CIDR %q is not an IPv4 host rule", value)
		}
	}
}
