package ui

import (
	"os"
	"strings"
	"testing"
)

func readWave0File(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// A status banner must never stay on "checking..." when the integration is disabled
// or not configured: the render function has to set a terminal state itself.
func TestConfigStatusBannersLeaveCheckingState(t *testing.T) {
	t.Parallel()
	cases := []struct{ file, disabledCall, disabledKey string }{
		{"cfg/adguard.js", "adgSetBannerState(document.getElementById('adg-status-banner')", "config.adguard.status_disabled"},
		{"cfg/fritzbox.js", "fbSetBanner('neutral'", "config.fritzbox.status_disabled"},
		{"cfg/truenas.js", "truenasSetBannerState(document.getElementById('truenas-status-banner')", "config.truenas.status_disabled"},
		{"cfg/jellyfin.js", "jellyfinSetBannerState(document.getElementById('jellyfin-status-banner')", "config.jellyfin.status_disabled"},
		{"cfg/mqtt.js", "mqttSetBanner('neutral'", "config.mqtt.status_disabled"},
		{"cfg/obsidian.js", "obsidianSetBannerState(document.getElementById('obsidian-status-banner')", "config.obsidian.status_disabled"},
	}
	for _, c := range cases {
		src := readWave0File(t, c.file)
		// A module on the shared banner (cfgStatusBanner) sets every state through it.
		if strings.Contains(src, "cfgStatusBanner(") {
			continue
		}
		if !strings.Contains(src, c.disabledCall) || !strings.Contains(src, c.disabledKey) {
			t.Errorf("%s does not set a disabled banner state", c.file)
		}
	}
	for _, m := range []string{"adguard", "truenas", "jellyfin", "obsidian"} {
		src := readWave0File(t, "cfg/"+m+".js")
		if !strings.Contains(src, "config."+m+".status_unknown") && !strings.Contains(src, "cfgStatusBanner(") {
			t.Errorf("cfg/%s.js has no unknown-status fallback", m)
		}
	}
}

func TestConfigAuthStatusFailureIsNotPasswordNotSet(t *testing.T) {
	t.Parallel()
	src := readWave0File(t, "cfg/auth.js")
	for _, want := range []string{"config.auth.status_load_failed", "auth-status-retry", "aria-disabled=\"true\" data-disabled=\"true\""} {
		// The shared banner brings its own retry button.
		if want == "auth-status-retry" && strings.Contains(src, "cfgStatusBanner(") {
			continue
		}
		if !strings.Contains(src, want) {
			t.Errorf("cfg/auth.js missing %q", want)
		}
	}
	if strings.Index(src, "statusError") > strings.Index(src, "config.auth.password_not_set") {
		t.Error("auth status error branch must come before the password cards")
	}
}

func TestConfigWave0MisleadingStatesFixed(t *testing.T) {
	t.Parallel()
	if src := readWave0File(t, "cfg/ai_gateway.js"); strings.Contains(src, "'setup_required') key = 'config.ai_gateway.hf_help'") {
		t.Error("ai_gateway setup_required must not show the HF slug help")
	}
	if src := readWave0File(t, "cfg/mcp_server.js"); !strings.Contains(src, "forced_by_bridge") || !strings.Contains(src, "mcp-token-error") {
		t.Error("mcp_server.js must show the forced-on note and inline token errors")
	}
	if src := readWave0File(t, "cfg/updates.js"); strings.Contains(src, "(${remaining}s)`") || !strings.Contains(src, "'{seconds}'") {
		t.Error("updates.js countdown must use the {seconds} placeholder")
	}
}
