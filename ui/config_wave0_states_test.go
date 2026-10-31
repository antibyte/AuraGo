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
// or not configured, or when the status check hangs: every module renders its banner through
// cfgStatusBanner, which has the terminal states off/unconfigured/unknown (with retry) built in.
func TestConfigStatusBannersLeaveCheckingState(t *testing.T) {
	t.Parallel()
	cases := []struct{ file, bannerID string }{
		{"cfg/adguard.js", "adg-status-banner"},
		{"cfg/fritzbox.js", "fb-status-banner"},
		{"cfg/truenas.js", "truenas-status-banner"},
		{"cfg/jellyfin.js", "jellyfin-status-banner"},
		{"cfg/mqtt.js", "mqtt-status-banner"},
		{"cfg/obsidian.js", "obsidian-status-banner"},
	}
	for _, c := range cases {
		src := readWave0File(t, c.file)
		if !strings.Contains(src, "cfgStatusBanner.html('"+c.bannerID+"'") && !strings.Contains(src, "id: '"+c.bannerID+"'") {
			t.Errorf("%s does not render %s through the shared banner", c.file, c.bannerID)
		}
		// The initial state of a disabled module is "off", never "checking".
		if !strings.Contains(src, "'off'") {
			t.Errorf("%s does not set a disabled (off) banner state", c.file)
		}
		// The status call runs through cfgStatusBanner.check (timeout -> unknown with retry).
		if !strings.Contains(src, "cfgStatusBanner.check(") {
			t.Errorf("%s status check does not use cfgStatusBanner.check, so it can hang on checking", c.file)
		}
		for _, legacy := range []string{"SetBannerState", "setBannerState", "status_unknown'"} {
			if strings.Contains(src, legacy) {
				t.Errorf("%s still carries the legacy banner setter %q", c.file, legacy)
			}
		}
	}
}

func TestConfigAuthStatusFailureIsNotPasswordNotSet(t *testing.T) {
	t.Parallel()
	src := readWave0File(t, "cfg/auth.js")
	for _, want := range []string{"config.auth.status_load_failed", "auth-status-retry", "aria-disabled=\"true\" data-disabled=\"true\""} {
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
