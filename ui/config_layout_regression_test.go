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
		"cfg/mqtt.js":              {"/api/mqtt/test", "mqttTestConnection"},
		"cfg/netlify.js":           {"/api/netlify/test-connection", "nfTestConnection"},
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

func TestConfigPhase2TTSAndGoogleWorkspaceAvoidInlineStyles(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"cfg/tts.js", "cfg/google_workspace.js"} {
		content := normalizeAssetText(mustReadUIFile(t, file))
		if strings.Contains(content, "style=") {
			t.Fatalf("%s still contains inline style attributes", file)
		}
	}

	ttsJS := normalizeAssetText(mustReadUIFile(t, "cfg/tts.js"))
	for _, marker := range []string{"adg-status-banner", "adg-password-row", "field-select", "tts-provider-section"} {
		if !strings.Contains(ttsJS, marker) {
			t.Fatalf("tts.js missing marker %q", marker)
		}
	}

	gwJS := normalizeAssetText(mustReadUIFile(t, "cfg/google_workspace.js"))
	for _, marker := range []string{"gw-status-line", "adg-password-row", "adg-test-btn", "gw-scope-row"} {
		if !strings.Contains(gwJS, marker) {
			t.Fatalf("google_workspace.js missing marker %q", marker)
		}
	}
}

func TestConfigPhase2cManifestDograhActionRows(t *testing.T) {
	t.Parallel()

	manifestJS := normalizeAssetText(mustReadUIFile(t, "cfg/manifest.js"))
	for _, marker := range []string{
		"adg-status-banner",
		"adg-test-result",
		"cfg-actions-row",
		"CFG_OPTION_OTHER_CUSTOM",
		"cfgFieldOptionLabel",
		"manifestStatusState",
	} {
		if !strings.Contains(manifestJS, marker) {
			t.Fatalf("manifest.js missing marker %q", marker)
		}
	}

	dograhJS := normalizeAssetText(mustReadUIFile(t, "cfg/dograh.js"))
	for _, marker := range []string{
		"adg-status-banner",
		"adg-test-result",
		"cfg-actions-row",
		"dograhSetBanner",
		"dograh-test-result",
		"config.dograh.vault_section_title",
		"config.dograh.mcp_section_title",
	} {
		if !strings.Contains(dograhJS, marker) {
			t.Fatalf("dograh.js missing marker %q", marker)
		}
	}
	if strings.Contains(dograhJS, "field-row") {
		t.Fatal("dograh.js should use field-group layout instead of field-row")
	}
}

func TestConfigPhase3MQTTAndNetlifyActionRows(t *testing.T) {
	t.Parallel()

	mqttJS := normalizeAssetText(mustReadUIFile(t, "cfg/mqtt.js"))
	for _, marker := range []string{
		"adg-status-banner",
		"adg-test-btn",
		"adg-test-result",
		"cfg-actions-row",
		"field-select",
		"field-grid two-cols",
		"adg-password-row",
		"mqttSetBanner",
		"/api/mqtt/test",
	} {
		if !strings.Contains(mqttJS, marker) {
			t.Fatalf("mqtt.js missing marker %q", marker)
		}
	}
	if strings.Contains(mqttJS, "cfg-status-banner") {
		t.Fatal("mqtt.js should use adg-status-banner instead of cfg-status-banner")
	}
	if strings.Contains(mqttJS, `class="field-input" data-path="mqtt.qos"`) {
		t.Fatal("mqtt.js QoS select should use field-select")
	}

	netlifyJS := normalizeAssetText(mustReadUIFile(t, "cfg/netlify.js"))
	for _, marker := range []string{
		"adg-test-btn",
		"adg-test-result",
		"cfg-actions-row",
		"nf-test-btn",
		"/api/netlify/test-connection",
	} {
		if !strings.Contains(netlifyJS, marker) {
			t.Fatalf("netlify.js missing marker %q", marker)
		}
	}
	if strings.Contains(netlifyJS, "nf-test-spinner") || strings.Contains(netlifyJS, "nf-test-msg") {
		t.Fatal("netlify.js should use unified adg-test-result instead of legacy spinner/msg blocks")
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