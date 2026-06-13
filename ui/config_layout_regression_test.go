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
		"cfg/vercel.js":            {"/api/vercel/test-connection", "vercelTestConnection"},
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
		"field-grid two-cols",
	} {
		if !strings.Contains(netlifyJS, marker) {
			t.Fatalf("netlify.js missing marker %q", marker)
		}
	}
	if strings.Contains(netlifyJS, "nf-test-spinner") || strings.Contains(netlifyJS, "nf-test-msg") {
		t.Fatal("netlify.js should use unified adg-test-result instead of legacy spinner/msg blocks")
	}
	if strings.Contains(netlifyJS, "cfg-input cfg-input-full") {
		t.Fatal("netlify.js site fields should use field-input inside field-grid")
	}
}

func TestConfigPhase3VercelActionRows(t *testing.T) {
	t.Parallel()

	vercelJS := normalizeAssetText(mustReadUIFile(t, "cfg/vercel.js"))
	for _, marker := range []string{
		"adg-test-btn",
		"adg-test-result",
		"cfg-actions-row",
		"vercel-test-btn",
		"/api/vercel/test-connection",
		"adg-save-btn",
		"field-grid two-cols",
	} {
		if !strings.Contains(vercelJS, marker) {
			t.Fatalf("vercel.js missing marker %q", marker)
		}
	}
	if strings.Contains(vercelJS, "vercel-test-spinner") || strings.Contains(vercelJS, "vercel-test-msg") {
		t.Fatal("vercel.js should use unified adg-test-result instead of legacy spinner/msg blocks")
	}
	if strings.Contains(vercelJS, "cfg-input cfg-input-full") {
		t.Fatal("vercel.js project fields should use field-input inside field-grid")
	}
}

func TestConfigPhase3FritzboxGitHub(t *testing.T) {
	t.Parallel()

	fritzboxJS := normalizeAssetText(mustReadUIFile(t, "cfg/fritzbox.js"))
	for _, marker := range []string{
		"adg-status-banner",
		"fbSetBanner",
		"adg-password-row",
		"adg-save-btn",
		"cfg-actions-row",
		"adg-test-btn",
		"adg-test-result",
		"attachChangeListeners",
		"status_not_configured",
		"/api/fritzbox/test",
	} {
		if !strings.Contains(fritzboxJS, marker) {
			t.Fatalf("fritzbox.js missing marker %q", marker)
		}
	}
	if strings.Contains(fritzboxJS, "cfg-status-banner") {
		t.Fatal("fritzbox.js should use adg-status-banner instead of cfg-status-banner")
	}

	githubJS := normalizeAssetText(mustReadUIFile(t, "cfg/github.js"))
	for _, marker := range []string{
		"adg-password-row",
		"adg-save-btn",
		"adg-test-btn",
		"adg-test-result",
		"cfg-actions-row",
		"github-token-status",
		"github-fetch-status",
		"btn-save btn-secondary",
	} {
		if !strings.Contains(githubJS, marker) {
			t.Fatalf("github.js missing marker %q", marker)
		}
	}
	if strings.Contains(githubJS, "cfg-save-btn-sm") || strings.Contains(githubJS, "cfg-status-banner") {
		t.Fatal("github.js should use unified adg-* and btn-secondary patterns")
	}
}

func TestConfigPhase3TailscaleA2AVaultRows(t *testing.T) {
	t.Parallel()

	tailscaleJS := normalizeAssetText(mustReadUIFile(t, "cfg/tailscale.js"))
	for _, marker := range []string{
		"adg-password-row",
		"adg-save-btn",
		"adg-test-result",
		"ts-api-key-status",
		"ts-auth-key-status",
	} {
		if !strings.Contains(tailscaleJS, marker) {
			t.Fatalf("tailscale.js missing marker %q", marker)
		}
	}
	if strings.Contains(tailscaleJS, "cfg-save-btn-sm") || strings.Contains(tailscaleJS, "ts-key-status") {
		t.Fatal("tailscale.js vault rows should use adg-password-row and adg-test-result")
	}

	a2aJS := normalizeAssetText(mustReadUIFile(t, "cfg/a2a.js"))
	for _, marker := range []string{
		"adg-password-row",
		"adg-save-btn",
		"adg-test-result",
		"a2a-bearer-secret-status",
	} {
		if !strings.Contains(a2aJS, marker) {
			t.Fatalf("a2a.js missing marker %q", marker)
		}
	}
	if strings.Contains(a2aJS, "cfg-save-btn-sm") || strings.Contains(a2aJS, "a2a-status-text") {
		t.Fatal("a2a.js auth vault rows should use unified adg-* patterns")
	}
}

func TestConfigPhase3MediaGenerationModules(t *testing.T) {
	t.Parallel()

	specs := []struct {
		file       string
		testAPI    string
		testFn     string
		resultID   string
		wantGrid   bool
	}{
		{"cfg/image_generation.js", "/api/image-generation/test", "imggenTestConnection", "imggen-test-result", true},
		{"cfg/music_generation.js", "/api/music-generation/test", "musicTestConnection", "music-test-result", true},
		{"cfg/video_generation.js", "/api/video-generation/test", "videoTestConnection", "video-test-result", true},
		{"cfg/yepapi.js", "/api/yepapi/test", "yepapiTestConnection", "yepapi-test-result", false},
	}
	for _, spec := range specs {
		content := normalizeAssetText(mustReadUIFile(t, spec.file))
		markers := []string{
			"cfg-actions-row",
			"adg-test-btn",
			"adg-test-result",
			spec.resultID,
			"field-select",
			spec.testAPI,
			spec.testFn,
		}
		if spec.wantGrid {
			markers = append(markers, "field-grid two-cols", "field-input")
		}
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Fatalf("%s missing marker %q", spec.file, marker)
			}
		}
		if strings.Contains(content, "cfg-save-btn-sm") || strings.Contains(content, "ig-test-status") || strings.Contains(content, "ig-flex-row") {
			t.Fatalf("%s should use unified cfg-actions-row/adg-test-* patterns", spec.file)
		}
		if spec.wantGrid && strings.Contains(content, "cfg-input cfg-input-full") {
			t.Fatalf("%s should use field-input/field-select instead of cfg-input", spec.file)
		}
	}
}

func TestConfigPhase3MCPActionRows(t *testing.T) {
	t.Parallel()

	mcpJS := normalizeAssetText(mustReadUIFile(t, "cfg/mcp.js"))
	for _, marker := range []string{
		"cfg-actions-row",
		"btn-save btn-secondary",
		"mcpServerAdd",
		"mcpSecretAdd",
	} {
		if !strings.Contains(mcpJS, marker) {
			t.Fatalf("mcp.js missing marker %q", marker)
		}
	}
	if strings.Contains(mcpJS, "cfg-save-btn-sm") {
		t.Fatal("mcp.js add-server/secret buttons should use btn-secondary instead of cfg-save-btn-sm")
	}
}

func TestConfigPhase3ComposioEmailServer(t *testing.T) {
	t.Parallel()

	composioJS := normalizeAssetText(mustReadUIFile(t, "cfg/composio.js"))
	for _, marker := range []string{
		"adg-status-banner",
		"composioSetBanner",
		"cfg-actions-row",
		"adg-test-btn",
		"adg-test-result",
		"adg-password-row",
		"adg-save-btn",
		"field-grid two-cols",
		"/api/composio/test",
	} {
		if !strings.Contains(composioJS, marker) {
			t.Fatalf("composio.js missing marker %q", marker)
		}
	}
	if strings.Contains(composioJS, "cfg-save-btn-sm") || strings.Contains(composioJS, "cmp-status-line") {
		t.Fatal("composio.js should use unified adg-* patterns")
	}

	emailJS := normalizeAssetText(mustReadUIFile(t, "cfg/email.js"))
	for _, marker := range []string{
		"cfg-actions-row",
		"btn-save btn-secondary",
		"field-select",
		"attachChangeListeners",
		"adg-test-btn",
		"adg-test-result",
	} {
		if !strings.Contains(emailJS, marker) {
			t.Fatalf("email.js missing marker %q", marker)
		}
	}
	if strings.Contains(emailJS, "cfg-save-btn-sm") {
		t.Fatal("email.js should not use cfg-save-btn-sm")
	}

	serverJS := normalizeAssetText(mustReadUIFile(t, "cfg/server.js"))
	for _, marker := range []string{
		"field-grid two-cols",
		"field-input",
		"field-select",
	} {
		if !strings.Contains(serverJS, marker) {
			t.Fatalf("server.js missing marker %q", marker)
		}
	}
	if strings.Contains(serverJS, "cfg-input cfg-input-full") {
		t.Fatal("server.js should use field-input/field-select instead of cfg-input")
	}
}

func TestConfigPhase3SecurityProxyHomepage(t *testing.T) {
	t.Parallel()

	securityProxyJS := normalizeAssetText(mustReadUIFile(t, "cfg/security_proxy.js"))
	for _, marker := range []string{
		"cfg-actions-row",
		"field-grid two-cols",
		"field-input",
		"field-select",
		"/api/proxy/status",
	} {
		if !strings.Contains(securityProxyJS, marker) {
			t.Fatalf("security_proxy.js missing marker %q", marker)
		}
	}
	if strings.Contains(securityProxyJS, "cfg-input") {
		t.Fatal("security_proxy.js should use field-input/field-select instead of cfg-input")
	}

	homepageJS := normalizeAssetText(mustReadUIFile(t, "cfg/homepage.js"))
	for _, marker := range []string{
		"field-grid two-cols",
		"field-input",
		"field-select",
		"adg-save-btn",
		"adg-test-btn",
		"adg-test-result",
		"cfg-actions-row",
		"/api/homepage/test-connection",
		"hpTestConnection",
	} {
		if !strings.Contains(homepageJS, marker) {
			t.Fatalf("homepage.js missing marker %q", marker)
		}
	}
	if strings.Contains(homepageJS, "cfg-input") || strings.Contains(homepageJS, "hp-test-spinner") {
		t.Fatal("homepage.js should use unified field-* and adg-test-* patterns")
	}
}

func TestConfigPhase3RemoteControlLLMGuardian(t *testing.T) {
	t.Parallel()

	remoteControlJS := normalizeAssetText(mustReadUIFile(t, "cfg/remote_control.js"))
	for _, marker := range []string{
		"field-grid two-cols",
		"field-input",
		"field-select",
		"cfg-actions-row",
		"adg-test-result",
		"/api/remote/enroll",
		"rcCreateEnrollmentToken",
	} {
		if !strings.Contains(remoteControlJS, marker) {
			t.Fatalf("remote_control.js missing marker %q", marker)
		}
	}
	if strings.Contains(remoteControlJS, "cfg-input") {
		t.Fatal("remote_control.js should use field-input/field-select instead of cfg-input")
	}

	llmGuardianJS := normalizeAssetText(mustReadUIFile(t, "cfg/llm_guardian.js"))
	for _, marker := range []string{
		"field-grid two-cols",
		"field-input",
		"field-select",
		"llm_guardian.provider",
		"llm_guardian.default_level",
		"llm_guardian.fail_safe",
	} {
		if !strings.Contains(llmGuardianJS, marker) {
			t.Fatalf("llm_guardian.js missing marker %q", marker)
		}
	}
	if strings.Contains(llmGuardianJS, "cfg-input-full") {
		t.Fatal("llm_guardian.js should use field-input/field-select without cfg-input-full")
	}
}

func TestConfigPhase3AIGateway(t *testing.T) {
	t.Parallel()

	aiGatewayJS := normalizeAssetText(mustReadUIFile(t, "cfg/ai_gateway.js"))
	for _, marker := range []string{
		"adg-status-banner",
		"aiGatewaySetBanner",
		"field-grid two-cols",
		"field-input",
		"adg-password-input",
		"cfg-actions-row",
		"adg-test-btn",
		"adg-test-result",
		"/api/ai-gateway/test",
		"aiGatewayTestConnection",
	} {
		if !strings.Contains(aiGatewayJS, marker) {
			t.Fatalf("ai_gateway.js missing marker %q", marker)
		}
	}
	if strings.Contains(aiGatewayJS, "cfg-input") || strings.Contains(aiGatewayJS, "ai-gw-grid") {
		t.Fatal("ai_gateway.js should use field-grid and field-input instead of legacy ai-gw patterns")
	}
	if strings.Contains(aiGatewayJS, "is-neutral") {
		t.Fatal("ai_gateway.js should not use nonexistent is-neutral banner state")
	}
}

func TestConfigManifestDograhAvoidEmbeddedFallbackTables(t *testing.T) {
	t.Parallel()

	for _, spec := range []struct {
		file       string
		forbidden  string
	}{
		{"cfg/manifest.js", "manifestFallbackText"},
		{"cfg/dograh.js", "dograhFallbackText"},
	} {
		content := normalizeAssetText(mustReadUIFile(t, spec.file))
		if strings.Contains(content, spec.forbidden) {
			t.Fatalf("%s still embeds %s; use ui/lang translations via t()", spec.file, spec.forbidden)
		}
		if !strings.Contains(content, "function manifestText") && !strings.Contains(content, "function dograhText") {
			t.Fatalf("%s should keep section text helper", spec.file)
		}
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