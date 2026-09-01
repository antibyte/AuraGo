package virtualcomputers

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestWorkspaceAssetsArePinnedAndComplete(t *testing.T) {
	wantSourceHashes := map[string]string{
		"server.go":        "1119d90cc3932a7beba47592ccdba336b519b91e87a594e01e1f34a0b8badf14",
		"templates.go":     "b48d9702ec3ea5b05db5b241af37b07a3e2a2a2d641234cfbdcf90bf69b38d92",
		"machinevolume.go": "6a6a9e58cd5cebb8def27a25722245ed546530a3eb3770bed81bcf8048551e15",
		"machine.go":       "eb294a8e665cfe7d9e1855afb5edc4b829ca6ab7cbc149ca38f33d915fab22f9",
	}
	gotSourceHashes := map[string]string{
		"server.go":        workspaceServerSHA256,
		"templates.go":     workspaceTemplateSHA256,
		"machinevolume.go": workspaceMachineVolumeSHA256,
		"machine.go":       workspaceMachineSHA256,
	}
	for name, want := range wantSourceHashes {
		if got := gotSourceHashes[name]; got != want {
			t.Fatalf("pinned LF source hash for %s = %q, want %q", name, got, want)
		}
	}

	fingerprint := WorkspaceAssetFingerprint()
	if len(fingerprint) != 64 {
		t.Fatalf("fingerprint length = %d, want 64", len(fingerprint))
	}
	if _, err := hex.DecodeString(fingerprint); err != nil {
		t.Fatalf("fingerprint is not hexadecimal: %v", err)
	}

	patchScript := workspacePatchInstallSnippet()
	for _, required := range []string{
		`git -C "${REPO_DIR}" reset --hard "${BORING_REVISION}"`,
		`"${REPO_DIR}/boringd/workspace.go"`,
		`"${REPO_DIR}/boringd/workspace_network.go"`,
	} {
		if !strings.Contains(patchScript, required) {
			t.Fatalf("patch install script is missing deterministic reset marker %q", required)
		}
	}
	if strings.Contains(patchScript, "git clean") || strings.Contains(patchScript, "apply --reverse --check") {
		t.Fatal("patch install script must reset only its known files and apply the ordered series afresh")
	}
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
