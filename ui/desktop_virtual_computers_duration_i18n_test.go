package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopVirtualComputersDurationI18n(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/virtual-computers.js")
	for _, want := range []string{
		"function formatDuration(seconds, context)",
		"function formatExpiryCountdown(expiresAt, nowMs, context)",
		"context.t(key, { count })",
		"desktop.virtual_computers_duration_seconds",
		"desktop.virtual_computers_duration_minutes",
		"desktop.virtual_computers_duration_hours",
		"desktop.virtual_computers_duration_days",
		"desktop.virtual_computers_expiry_days",
		"formatDuration(machine.ttl_seconds, c)",
		"formatDuration(300, c)",
		"formatExpiryCountdown(expiresAt, undefined, c)",
		"formatExpiryCountdown(node.dataset.expiresAt, now, state.context)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("virtual computers duration i18n missing marker %q", want)
		}
	}

	workspaces := readDesktopAssetText(t, "js/desktop/apps/virtual-computers-workspaces.js")
	if !strings.Contains(workspaces, "formatExpiryCountdown(workspace.lease_expires_at, undefined, c)") {
		t.Fatal("workspace expiry countdown must pass context into formatExpiryCountdown")
	}

	for _, forbidden := range []string{
		"`${value} s`",
		"` min`",
		"`${days}d ${clock}`",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("virtual computers still hardcodes %q", forbidden)
		}
	}

	enHours := "{{count}} h"
	enExpiry := "{{days}}d {{clock}}"
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"desktop.virtual_computers_duration_seconds",
			"desktop.virtual_computers_duration_minutes",
			"desktop.virtual_computers_duration_hours",
			"desktop.virtual_computers_duration_days",
		} {
			got := values[key]
			if strings.TrimSpace(got) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
			if !strings.Contains(got, "{{count}}") {
				t.Fatalf("%s %s must keep {{count}}", path, key)
			}
		}
		expiry := values["desktop.virtual_computers_expiry_days"]
		if strings.TrimSpace(expiry) == "" {
			t.Fatalf("%s missing non-empty desktop.virtual_computers_expiry_days", path)
		}
		if !strings.Contains(expiry, "{{days}}") || !strings.Contains(expiry, "{{clock}}") {
			t.Fatalf("%s expiry days must keep {{days}} and {{clock}}", path)
		}
		if lang == "de" {
			if values["desktop.virtual_computers_duration_hours"] == enHours {
				t.Fatalf("%s must not copy the English duration hours string", path)
			}
			if expiry == enExpiry {
				t.Fatalf("%s must not copy the English expiry days string", path)
			}
		}
	}
}
