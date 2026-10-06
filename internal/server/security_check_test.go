package server

import (
	"aurago/internal/config"
	"aurago/internal/sandbox"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestCheckSecurityAddsPhase2Hints(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Agent.AllowPython = true
	cfg.Agent.AllowMCP = true
	cfg.MCP.Enabled = true
	cfg.Agent.AllowShell = true
	cfg.Agent.AllowFilesystemWrite = true
	cfg.Auth.Enabled = false
	cfg.CloudflareTunnel.Enabled = true
	cfg.CloudflareTunnel.ExposeWebUI = true

	hints := CheckSecurity(cfg)
	if !hasSecurityHint(hints, "python_no_sandbox") {
		t.Fatalf("expected python_no_sandbox hint, got %#v", hints)
	}
	if !hasSecurityHint(hints, "mcp_public") {
		t.Fatalf("expected mcp_public hint, got %#v", hints)
	}
	if !hasSecurityHint(hints, "critical_public_exposure") {
		t.Fatalf("expected critical_public_exposure hint, got %#v", hints)
	}
}

func TestCheckSecuritySkipsPythonNoSandboxWhenSandboxReady(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Sandbox.Enabled = true
	cfg.Runtime.IsDocker = false

	hints := CheckSecurity(cfg)
	if hasSecurityHint(hints, "python_no_sandbox") {
		t.Fatalf("did not expect python_no_sandbox hint, got %#v", hints)
	}
}

func TestCheckSecurityWarnsWhenShellEnabledWithoutSandbox(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Agent.AllowShell = true

	hints := CheckSecurity(cfg)
	hint := findSecurityHint(hints, "shell_no_sandbox")
	if hint == nil {
		t.Fatalf("expected shell_no_sandbox hint, got %#v", hints)
	}
	if hint.Severity != SevWarning {
		t.Fatalf("severity = %q, want %q", hint.Severity, SevWarning)
	}
	if !strings.Contains(hint.Description, "can bypass Desktop Notes protection") {
		t.Fatalf("missing explanation of unisolated execution: %s", hint.Description)
	}
}

func TestCheckSecurityWarnsWhenNetworkShareWritesAreEnabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.NetworkShares.Enabled = true
	cfg.NetworkShares.ReadOnly = false
	cfg.NetworkShares.AllowCreate = true
	cfg.NetworkShares.AllowDelete = true

	hints := CheckSecurity(cfg)
	hint := findSecurityHint(hints, "network_shares_write_enabled")
	if hint == nil {
		t.Fatalf("expected network_shares_write_enabled hint, got %#v", hints)
	}
	if hint.Severity != SevWarning {
		t.Fatalf("severity = %q, want %q", hint.Severity, SevWarning)
	}
	if patch := hint.FixPatch["network_shares"]; patch == nil {
		t.Fatalf("expected network_shares read-only auto-fix, got %#v", hint.FixPatch)
	}
}

func TestCheckSecurityShellNoSandboxCriticalWhenPublic(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Server.Host = "0.0.0.0"
	cfg.CloudflareTunnel.Enabled = true
	cfg.CloudflareTunnel.ExposeWebUI = true

	hints := CheckSecurity(cfg)
	hint := findSecurityHint(hints, "shell_no_sandbox")
	if hint == nil {
		t.Fatalf("expected shell_no_sandbox hint, got %#v", hints)
	}
	if hint.Severity != SevCritical {
		t.Fatalf("severity = %q, want %q", hint.Severity, SevCritical)
	}
}

func TestCheckSecuritySkipsShellNoSandboxWhenShellDisabledOrSandboxReady(t *testing.T) {
	disabled := &config.Config{}
	if hasSecurityHint(CheckSecurity(disabled), "shell_no_sandbox") {
		t.Fatal("did not expect shell_no_sandbox when shell is disabled")
	}

	ready := &config.Config{}
	ready.Agent.AllowShell = true
	ready.ShellSandbox.Enabled = true
	ready.Runtime.IsDocker = false
	if runtime.GOOS == "linux" {
		restore := sandbox.SetForTest(testReadyShellSandbox{})
		t.Cleanup(restore)
	}
	if runtime.GOOS == "linux" && hasSecurityHint(CheckSecurity(ready), "shell_no_sandbox") {
		t.Fatal("did not expect shell_no_sandbox when shell sandbox backend is ready")
	}
	if runtime.GOOS != "linux" && !hasSecurityHint(CheckSecurity(ready), "shell_no_sandbox") {
		t.Fatal("expected shell_no_sandbox on non-Linux even when shell sandbox is configured")
	}
}

func TestCheckSecurityWarnsWhenShellSandboxFallsBackUnsandboxed(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.ShellSandbox.Enabled = true
	cfg.Runtime.IsDocker = false

	oldGOOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = oldGOOS })
	restore := sandbox.SetForTest(&sandbox.FallbackSandbox{})
	t.Cleanup(restore)

	hints := CheckSecurity(cfg)
	if !hasSecurityHint(hints, "shell_no_sandbox") {
		t.Fatalf("expected shell_no_sandbox when shell sandbox falls back to unsandboxed backend, got %#v", hints)
	}
}

func TestCheckSecurityReportsLegacyUnsandboxedHostShell(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.Agent.LegacyUnsandboxedShell = true
	cfg.ShellSandbox.Enabled = false

	oldGOOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = oldGOOS })
	t.Cleanup(sandbox.SetForTest(&sandbox.FallbackSandbox{}))

	hint := findSecurityHint(CheckSecurity(cfg), "shell_unsafe_host_legacy")
	if hint == nil {
		t.Fatal("expected shell_unsafe_host_legacy when the Linux host shell runs under the legacy default")
	}
	if hint.Severity != SevWarning {
		t.Fatalf("severity = %q, want %q", hint.Severity, SevWarning)
	}
	if !strings.Contains(hint.Description, "agent.allow_unsandboxed_shell: true") {
		t.Fatalf("legacy hint must name the explicit allow_unsandboxed_shell option: %s", hint.Description)
	}
	const hostShellRule = "Without agent.allow_unsandboxed_shell or agent.allow_unsafe_host_execution the Linux/macOS host shell is refused."
	shellHint := findSecurityHint(CheckSecurity(cfg), "shell_no_sandbox")
	if shellHint == nil || !strings.Contains(shellHint.Description, hostShellRule) {
		t.Fatalf("shell_no_sandbox must state the host shell rule, got %#v", shellHint)
	}

	cfg.Agent.LegacyUnsandboxedShell = false
	if hasSecurityHint(CheckSecurity(cfg), "shell_unsafe_host_legacy") {
		t.Fatal("did not expect shell_unsafe_host_legacy without the legacy grandfather")
	}
	cfg.Agent.LegacyUnsandboxedShell = true

	cfg.ShellSandbox.Enabled = true
	restoreBlocked := sandbox.SetForTest(&sandbox.BlockingSandbox{})
	if hasSecurityHint(CheckSecurity(cfg), "shell_unsafe_host_legacy") {
		t.Fatal("did not expect shell_unsafe_host_legacy while the blocked sandbox refuses the shell")
	}
	restoreBlocked()
	cfg.ShellSandbox.Enabled = false

	cfg.Agent.AllowUnsafeHostExecution = true
	if hasSecurityHint(CheckSecurity(cfg), "shell_unsafe_host_legacy") {
		t.Fatal("did not expect shell_unsafe_host_legacy once allow_unsafe_host_execution is explicit")
	}

	cfg.Agent.AllowUnsafeHostExecution = false
	cfg.Agent.AllowUnsandboxedShell = true
	if hasSecurityHint(CheckSecurity(cfg), "shell_unsafe_host_legacy") {
		t.Fatal("did not expect shell_unsafe_host_legacy once allow_unsandboxed_shell is explicit")
	}

	cfg.Agent.AllowUnsandboxedShell = false
	runtimeGOOS = "windows"
	windowsHints := CheckSecurity(cfg)
	if hasSecurityHint(windowsHints, "shell_unsafe_host_legacy") {
		t.Fatal("did not expect shell_unsafe_host_legacy on Windows, which has its own shell gate")
	}
	if shellHint := findSecurityHint(windowsHints, "shell_no_sandbox"); shellHint == nil || strings.Contains(shellHint.Description, hostShellRule) {
		t.Fatalf("shell_no_sandbox on Windows must omit the Linux/macOS host shell rule, got %#v", shellHint)
	}
}

func TestCheckSecuritySkipsShellNoSandboxWhenEffectiveSandboxActive(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.AllowShell = true
	cfg.ShellSandbox.Enabled = true
	cfg.Runtime.IsDocker = false

	oldGOOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = oldGOOS })
	restore := sandbox.SetForTest(testReadyShellSandbox{})
	t.Cleanup(restore)

	if hasSecurityHint(CheckSecurity(cfg), "shell_no_sandbox") {
		t.Fatal("did not expect shell_no_sandbox when effective shell sandbox backend is active")
	}
}

func TestCheckSecurityNoPasswordTextSaysLocked(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Auth.Enabled = true

	hints := CheckSecurity(cfg)
	hint := findSecurityHint(hints, "no_password")
	if hint == nil {
		t.Fatalf("expected no_password hint, got %#v", hints)
	}
	if strings.Contains(strings.ToLower(hint.Description), "bypassed") {
		t.Fatalf("no_password hint should not claim auth is bypassed: %q", hint.Description)
	}
	if !strings.Contains(strings.ToLower(hint.Description), "locked") {
		t.Fatalf("no_password hint should explain access is locked: %q", hint.Description)
	}
}

func TestCheckSecurityWarnsWhenLLMGuardianProviderMissing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.LLMGuardian.Enabled = true
	cfg.LLMGuardian.Provider = "missing-guardian"
	cfg.Providers = []config.ProviderEntry{{ID: "main", Type: "openai"}}

	hints := CheckSecurity(cfg)
	hint := findSecurityHint(hints, "llm_guardian_provider_missing")
	if hint == nil {
		t.Fatalf("expected llm_guardian_provider_missing hint, got %#v", hints)
	}
	if hint.Severity != SevWarning {
		t.Fatalf("severity = %q, want %q", hint.Severity, SevWarning)
	}
	if !strings.Contains(strings.ToLower(hint.Description), "fallback") {
		t.Fatalf("description should mention fallback behavior: %q", hint.Description)
	}
}

func TestCheckSecurityDoesNotTreatLANHTTPSTailnetAsInternetFacing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.CertMode = "selfsigned"
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ServeHTTP = true
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "hash"
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = false
	cfg.Agent.AllowSelfUpdate = true

	if isInternetFacing(cfg) {
		t.Fatal("LAN HTTPS plus tailnet serve should not count as public internet exposure")
	}
	if !isNetworkFacing(cfg) {
		t.Fatal("LAN HTTPS plus tailnet serve should still count as network-facing")
	}

	hints := CheckSecurity(cfg)
	for _, id := range []string{"self_update_public", "totp_disabled"} {
		if hasSecurityHint(hints, id) {
			t.Fatalf("did not expect %s for LAN/tailnet-only instance, got %#v", id, hints)
		}
	}
	if !hasSecurityHint(hints, "docker_enabled_no_readonly") {
		t.Fatalf("expected Docker write-access warning for network-facing LAN/tailnet instance, got %#v", hints)
	}
}

func TestIsNetworkFacingIncludesTailscaleManifestExposure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ExposeManifest = true

	if !isNetworkFacing(cfg) {
		t.Fatal("Tailscale Manifest exposure should count as network-facing")
	}
}

func TestCheckSecurityTreatsTailscaleFunnelAsInternetFacing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.HTTPS.Enabled = true
	cfg.Server.HTTPS.CertMode = "selfsigned"
	cfg.Tailscale.TsNet.Enabled = true
	cfg.Tailscale.TsNet.ServeHTTP = true
	cfg.Tailscale.TsNet.Funnel = true
	cfg.Auth.Enabled = true
	cfg.Auth.PasswordHash = "hash"
	cfg.Docker.Enabled = true
	cfg.Docker.ReadOnly = false
	cfg.Agent.AllowSelfUpdate = true

	if !isInternetFacing(cfg) {
		t.Fatal("Tailscale Funnel should count as public internet exposure")
	}

	hints := CheckSecurity(cfg)
	for _, id := range []string{"docker_enabled_no_readonly", "self_update_public"} {
		if !hasSecurityHint(hints, id) {
			t.Fatalf("expected %s for Tailscale Funnel exposure, got %#v", id, hints)
		}
	}
}

func TestPublicHostnameLikelyTreatsTailnetAndPrivateHostsAsNonPublic(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"aurago", "aurago.local", "aurago.tailnet-name.ts.net", "192.168.6.238", "100.100.100.100"} {
		if publicHostnameLikely(host) {
			t.Fatalf("host %q should not look public", host)
		}
	}
	for _, host := range []string{"aurago.my-domain.com", "8.8.8.8"} {
		if !publicHostnameLikely(host) {
			t.Fatalf("host %q should look public", host)
		}
	}
}

// sipAutoAnswerConfig returns an answering SIP endpoint that trips both SIP hints.
func sipAutoAnswerConfig() *config.Config {
	cfg := &config.Config{}
	cfg.SIP.Enabled = true
	cfg.SIP.ReadOnly = false
	cfg.SIP.Permissions.AnswerInbound = true
	cfg.SIP.Inbound.Route = "agent"
	cfg.SIP.BindHost = "0.0.0.0"
	cfg.SIP.Inbound.AllowedCallers = []string{"*"}
	cfg.SIP.Inbound.TrustedPeerCIDRs = []string{"192.168.178.0/24"}
	return cfg
}

var sipHintIDs = []string{"sip_auto_answer_all_interfaces", "sip_wildcard_callers_cidr_peers"}

func TestCheckSecurityWarnsOnSIPAutoAnswerAndWildcardCallers(t *testing.T) {
	t.Parallel()

	hints := CheckSecurity(sipAutoAnswerConfig())
	for _, id := range sipHintIDs {
		hint := findSecurityHint(hints, id)
		if hint == nil {
			t.Fatalf("expected %s hint, got %#v", id, hints)
		}
		if hint.Severity != SevWarning {
			t.Fatalf("%s severity = %q, want %q", id, hint.Severity, SevWarning)
		}
		if hint.AutoFixable || len(hint.FixPatch) != 0 {
			t.Fatalf("%s must stay a manual hint, got %#v", id, hint)
		}
	}

	precise := sipAutoAnswerConfig()
	precise.SIP.Inbound.Route = "manual"
	precise.SIP.Inbound.TrustedPeerCIDRs = []string{"192.168.178.1"}
	hints = CheckSecurity(precise)
	for _, id := range sipHintIDs {
		if hasSecurityHint(hints, id) {
			t.Fatalf("did not expect %s for a manual route with an exact peer, got %#v", id, hints)
		}
	}
}

func TestCheckSecuritySIPAutoAnswerDependsOnBindHostAndRoute(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		bind  string
		route string
		want  bool
	}{
		{bind: "0.0.0.0", route: "agent", want: true},
		{bind: "::", route: "agent", want: true},
		{bind: "", route: "agent", want: true},
		{bind: " 0.0.0.0 ", route: " Agent ", want: true},
		{bind: "192.168.178.5", route: "agent", want: false},
		{bind: "127.0.0.1", route: "agent", want: false},
		{bind: "0.0.0.0", route: "manual", want: false},
		{bind: "0.0.0.0", route: "reject", want: false},
	} {
		cfg := sipAutoAnswerConfig()
		cfg.SIP.BindHost = tc.bind
		cfg.SIP.Inbound.Route = tc.route
		if got := hasSecurityHint(CheckSecurity(cfg), "sip_auto_answer_all_interfaces"); got != tc.want {
			t.Fatalf("bind %q route %q: sip_auto_answer_all_interfaces = %v, want %v", tc.bind, tc.route, got, tc.want)
		}
	}
}

func TestCheckSecuritySIPWildcardCallersNeedSubnetPeer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		peers   []string
		callers []string
		route   string
		want    bool
	}{
		{name: "ipv4 subnet", peers: []string{"192.168.178.0/24"}, callers: []string{"*"}, route: "agent", want: true},
		{name: "ipv6 subnet", peers: []string{"fd00::/64"}, callers: []string{"*"}, route: "agent", want: true},
		{name: "subnet among exact peers", peers: []string{"192.168.178.1", "10.0.0.0/8"}, callers: []string{"*"}, route: "manual", want: true},
		{name: "ipv4 host in cidr form", peers: []string{"192.168.178.1/32"}, callers: []string{"*"}, route: "agent", want: false},
		{name: "ipv6 host in cidr form", peers: []string{"fd00::1/128"}, callers: []string{"*"}, route: "agent", want: false},
		{name: "exact peer", peers: []string{"192.168.178.1"}, callers: []string{"*"}, route: "agent", want: false},
		{name: "named callers", peers: []string{"192.168.178.0/24"}, callers: []string{"alice", "**610"}, route: "agent", want: false},
		{name: "reject route", peers: []string{"192.168.178.0/24"}, callers: []string{"*"}, route: "reject", want: false},
	} {
		cfg := sipAutoAnswerConfig()
		cfg.SIP.Inbound.TrustedPeerCIDRs = tc.peers
		cfg.SIP.Inbound.AllowedCallers = tc.callers
		cfg.SIP.Inbound.Route = tc.route
		if got := hasSecurityHint(CheckSecurity(cfg), "sip_wildcard_callers_cidr_peers"); got != tc.want {
			t.Fatalf("%s: sip_wildcard_callers_cidr_peers = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestCheckSecuritySIPHintsRequireAnAnsweringEndpoint(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(*config.Config){
		"readonly":           func(cfg *config.Config) { cfg.SIP.ReadOnly = true },
		"answer inbound off": func(cfg *config.Config) { cfg.SIP.Permissions.AnswerInbound = false },
		"sip disabled":       func(cfg *config.Config) { cfg.SIP.Enabled = false },
	} {
		cfg := sipAutoAnswerConfig()
		mutate(cfg)
		hints := CheckSecurity(cfg)
		for _, id := range sipHintIDs {
			if hasSecurityHint(hints, id) {
				t.Fatalf("%s: did not expect %s, got %#v", name, id, hints)
			}
		}
	}
}

// fritzBoxReadOnlyConfig returns an enabled Fritz!Box over HTTP whose enabled
// feature groups are all read-only.
func fritzBoxReadOnlyConfig() *config.Config {
	cfg := &config.Config{}
	cfg.FritzBox.Enabled = true
	cfg.FritzBox.HTTPS = false
	cfg.FritzBox.System.Enabled = true
	cfg.FritzBox.System.ReadOnly = true
	cfg.FritzBox.Network.Enabled = true
	cfg.FritzBox.Network.ReadOnly = true
	cfg.FritzBox.Telephony.Enabled = true
	cfg.FritzBox.Telephony.ReadOnly = true
	cfg.FritzBox.SmartHome.Enabled = true
	cfg.FritzBox.SmartHome.ReadOnly = true
	cfg.FritzBox.Storage.Enabled = true
	cfg.FritzBox.Storage.ReadOnly = true
	cfg.FritzBox.TV.Enabled = true
	cfg.FritzBox.TV.ReadOnly = true
	return cfg
}

func TestCheckSecurityWarnsOnWritableFritzBoxGroupsOverHTTP(t *testing.T) {
	t.Parallel()

	for name, mutate := range map[string]func(*config.Config){
		"system":     func(cfg *config.Config) { cfg.FritzBox.System.ReadOnly = false },
		"network":    func(cfg *config.Config) { cfg.FritzBox.Network.ReadOnly = false },
		"telephony":  func(cfg *config.Config) { cfg.FritzBox.Telephony.ReadOnly = false },
		"smart home": func(cfg *config.Config) { cfg.FritzBox.SmartHome.ReadOnly = false },
		"storage":    func(cfg *config.Config) { cfg.FritzBox.Storage.ReadOnly = false },
	} {
		cfg := fritzBoxReadOnlyConfig()
		mutate(cfg)
		hint := findSecurityHint(CheckSecurity(cfg), "fritzbox_plaintext_sessions")
		if hint == nil {
			t.Fatalf("%s writable over HTTP: expected fritzbox_plaintext_sessions", name)
		}
		if hint.Severity != SevWarning {
			t.Fatalf("severity = %q, want %q", hint.Severity, SevWarning)
		}
		if hint.AutoFixable || len(hint.FixPatch) != 0 {
			t.Fatalf("fritzbox_plaintext_sessions must stay a manual hint, got %#v", hint)
		}
		for _, want := range []string{"fritzbox.https: true", "49443", "insecure_skip_verify"} {
			if !strings.Contains(hint.Description, want) {
				t.Fatalf("description must mention %q: %s", want, hint.Description)
			}
		}

		cfg.FritzBox.HTTPS = true
		if hasSecurityHint(CheckSecurity(cfg), "fritzbox_plaintext_sessions") {
			t.Fatalf("%s writable over HTTPS: did not expect fritzbox_plaintext_sessions", name)
		}
	}
}

func TestCheckSecuritySkipsFritzBoxPlaintextHintWithoutWritableGroup(t *testing.T) {
	t.Parallel()

	if hasSecurityHint(CheckSecurity(fritzBoxReadOnlyConfig()), "fritzbox_plaintext_sessions") {
		t.Fatal("did not expect fritzbox_plaintext_sessions when every group is read-only")
	}

	disabledGroup := fritzBoxReadOnlyConfig()
	disabledGroup.FritzBox.Network.Enabled = false
	disabledGroup.FritzBox.Network.ReadOnly = false
	if hasSecurityHint(CheckSecurity(disabledGroup), "fritzbox_plaintext_sessions") {
		t.Fatal("did not expect fritzbox_plaintext_sessions for a disabled group without readonly")
	}

	tvOnly := fritzBoxReadOnlyConfig()
	tvOnly.FritzBox.TV.ReadOnly = false
	if hasSecurityHint(CheckSecurity(tvOnly), "fritzbox_plaintext_sessions") {
		t.Fatal("did not expect fritzbox_plaintext_sessions for the TV group, which has no write actions")
	}

	integrationOff := fritzBoxReadOnlyConfig()
	integrationOff.FritzBox.System.ReadOnly = false
	integrationOff.FritzBox.Enabled = false
	if hasSecurityHint(CheckSecurity(integrationOff), "fritzbox_plaintext_sessions") {
		t.Fatal("did not expect fritzbox_plaintext_sessions while the integration is disabled")
	}
}

func hasSecurityHint(hints []SecurityHint, id string) bool {
	return findSecurityHint(hints, id) != nil
}

func findSecurityHint(hints []SecurityHint, id string) *SecurityHint {
	for _, hint := range hints {
		if hint.ID == id {
			return &hint
		}
	}
	return nil
}

type testReadyShellSandbox struct{}

func (testReadyShellSandbox) Available() bool { return true }
func (testReadyShellSandbox) Name() string    { return "landlock" }
func (testReadyShellSandbox) PrepareCommand(command, workDir string) *exec.Cmd {
	cmd := exec.Command("echo", "unused")
	cmd.Dir = workDir
	return cmd
}
func (testReadyShellSandbox) PrepareExecCommand(binary string, args []string, workDir string) *exec.Cmd {
	cmd := exec.Command("echo", "unused")
	cmd.Dir = workDir
	return cmd
}
