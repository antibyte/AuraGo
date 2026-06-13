package ui

import (
	"os"
	"strings"
	"testing"
)

func TestConfigLayoutCSSDefinesGridAndActionRows(t *testing.T) {
	t.Parallel()

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	for _, marker := range []string{
		".field-grid",
		"display:grid",
		".field-grid.two-cols",
		"grid-template-columns:repeat(2, minmax(0, 1fr))",
		".cfg-actions-row",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config.css missing %q", marker)
		}
	}
}

func TestConfigPhase1HelpMarkersPresent(t *testing.T) {
	t.Parallel()

	heartbeatJS := normalizeAssetText(mustReadUIFile(t, "cfg/heartbeat.js"))
	for _, marker := range []string{
		"t('help.heartbeat.enabled')",
		"t('help.heartbeat.check_tasks')",
		"t('help.heartbeat.day_time_window')",
		"t('help.heartbeat.night_time_window')",
	} {
		if !strings.Contains(heartbeatJS, marker) {
			t.Fatalf("heartbeat.js missing help marker %q", marker)
		}
	}

	webhooksJS := normalizeAssetText(mustReadUIFile(t, "cfg/webhooks.js"))
	for _, marker := range []string{
		"t('help.webhooks.enabled')",
		"t('help.webhooks.max_payload_size')",
		"t('help.webhooks.rate_limit')",
	} {
		if !strings.Contains(webhooksJS, marker) {
			t.Fatalf("webhooks.js missing help marker %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"cfgFieldOptionLabel",
		"config.field.disabled_option",
		"config.field.other_custom_option",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing marker %q", marker)
		}
	}

	videoJS := normalizeAssetText(mustReadUIFile(t, "cfg/video_download.js"))
	if strings.Contains(videoJS, "field-grid two-col\"") || strings.Contains(videoJS, "field-grid two-col'") {
		t.Fatal("video_download.js still uses two-col typo")
	}
	if !strings.Contains(videoJS, "field-grid two-cols") {
		t.Fatal("video_download.js should use field-grid two-cols")
	}
}

func TestConfigPhase2AuraConfigFormRollout(t *testing.T) {
	t.Parallel()

	for _, spec := range []struct {
		file   string
		marker string
	}{
		{"cfg/adguard.js", "form.renderSpec"},
		{"cfg/koofr.js", "form.renderSpec"},
		{"cfg/webdav.js", "form.renderSpec"},
		{"cfg/agentmail.js", "form.renderSpec"},
	} {
		content := normalizeAssetText(mustReadUIFile(t, spec.file))
		if !strings.Contains(content, spec.marker) {
			t.Fatalf("%s missing %q", spec.file, spec.marker)
		}
	}

	builder := normalizeAssetText(mustReadUIFile(t, "cfg/form-builder.js"))
	if !strings.Contains(builder, "field-select") {
		t.Fatal("form-builder.js select must use field-select")
	}
}

func TestConfigPhase2TestConnectionMarkers(t *testing.T) {
	t.Parallel()

	checks := map[string][]string{
		"cfg/koofr.js":             {"/api/koofr/test", "koofrTestConnection"},
		"cfg/webdav.js":            {"/api/webdav/test", "webdavTestConnection"},
		"cfg/github.js":            {"githubTestConnection", "config.github.test_btn"},
		"cfg/cloudflare_tunnel.js": {"cloudflareTunnelCheckStatus", "/api/cloudflare-tunnel/status"},
		"cfg/telnyx.js":            {"/api/telnyx/test", "telnyxTestConnection"},
		"cfg/tailscale.js":         {"/api/tailscale/test", "tsApiTestConnection"},
		"cfg/ai_gateway.js":        {"/api/ai-gateway/test", "aiGatewayTestConnection"},
		"cfg/email.js":             {"/api/email-accounts/test", "emailAccountTestFromModal"},
	}
	for file, markers := range checks {
		content := normalizeAssetText(mustReadUIFile(t, file))
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Fatalf("%s missing marker %q", file, marker)
			}
		}
	}
}

func TestConfigPhase2EmailModalUsesSharedOverlay(t *testing.T) {
	t.Parallel()

	emailJS := normalizeAssetText(mustReadUIFile(t, "cfg/email.js"))
	for _, marker := range []string{
		"modal-overlay open active",
		"modal-card email-modal-card",
		"modal-actions",
		"btn btn-secondary",
	} {
		if !strings.Contains(emailJS, marker) {
			t.Fatalf("email.js missing modal marker %q", marker)
		}
	}
	if strings.Contains(emailJS, "em-modal-overlay") {
		t.Fatal("email.js should not use em-modal-overlay")
	}
}

func TestConfigPhase2SecretsModalUsesSharedOverlay(t *testing.T) {
	t.Parallel()

	secretsJS := normalizeAssetText(mustReadUIFile(t, "cfg/secrets.js"))
	for _, marker := range []string{
		"modal-overlay open active",
		"modal-card secrets-modal-card",
		"modal-actions",
		"btn btn-secondary",
	} {
		if !strings.Contains(secretsJS, marker) {
			t.Fatalf("secrets.js missing modal marker %q", marker)
		}
	}
	if strings.Contains(secretsJS, "overlay.style.cssText") {
		t.Fatal("secrets.js should not use inline overlay styles")
	}
}

func TestConfigVirtualDesktopSectionLabelsInSectionsBundle(t *testing.T) {
	t.Parallel()

	langs := []string{"en", "de", "fr", "ja", "zh"}
	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("lang/config/sections/" + lang + ".json")
			if err != nil {
				t.Fatalf("read sections %s: %v", lang, err)
			}
			content := string(data)
			for _, key := range []string{
				"config.section.virtual_desktop.label",
				"config.section.virtual_desktop.desc",
			} {
				if !strings.Contains(content, key) {
					t.Fatalf("sections/%s.json missing %q", lang, key)
				}
			}
		})
	}
}