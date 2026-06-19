package ui

import (
	"encoding/json"
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
	if strings.Contains(netlifyJS, "nf-grid-2col") {
		t.Fatal("netlify.js permission toggles should use field-grid two-cols instead of nf-grid-2col")
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
	if strings.Contains(vercelJS, "nf-grid-2col") {
		t.Fatal("vercel.js permission toggles should use field-grid two-cols instead of nf-grid-2col")
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
	if strings.Contains(mcpJS, "cfg-save-btn-sm") || strings.Contains(mcpJS, "cfg-input-full") {
		t.Fatal("mcp.js should use btn-secondary and field-input/field-select without cfg-input-full")
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
	if strings.Contains(homepageJS, "hp-grid-two") || strings.Contains(homepageJS, "hp-toggle-row") {
		t.Fatal("homepage.js permission toggles should use field-grid two-cols and cfg-toggle-row-compact")
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

func TestConfigFieldDescriptionsAndDropdownUXCoverage(t *testing.T) {
	t.Parallel()

	checks := map[string][]string{
		"cfg/cloudflare_tunnel.js": {
			"config.cloudflare_tunnel.enabled_help",
			"config.cloudflare_tunnel.readonly_help",
			"config.cloudflare_tunnel.auto_start_help",
			"config.cloudflare_tunnel.mode_help",
			"config.cloudflare_tunnel.auth_method_help",
			"config.cloudflare_tunnel.tunnel_name_help",
			"config.cloudflare_tunnel.account_id_help",
			"config.cloudflare_tunnel.metrics_port_help",
			"config.cloudflare_tunnel.log_level_help",
			"config.cloudflare_tunnel.expose_target_help",
			"config.cloudflare_tunnel.token_hint",
		},
		"cfg/mcp_server.js": {
			"config.mcp_server.enabled_help",
			"config.mcp_server.vscode_bridge_desc",
			"config.mcp_server.require_auth_help",
			"config.mcp_server.allowed_tools_desc",
		},
		"cfg/netlify.js": {
			"config.netlify.enabled_help",
			"config.netlify.readonly_help",
			"config.netlify.allow_deploy_help",
			"config.netlify.allow_site_management_help",
			"config.netlify.allow_env_management_help",
			"config.netlify.default_site_id_help",
			"config.netlify.team_slug_help",
		},
		"cfg/onedrive.js": {
			"config.onedrive.enabled_help",
			"config.onedrive.readonly_hint",
			"config.onedrive.client_id_hint",
			"config.onedrive.tenant_id_hint",
		},
		"cfg/vercel.js": {
			"config.vercel.enabled_help",
			"config.vercel.readonly_help",
			"config.vercel.allow_deploy_help",
			"config.vercel.allow_project_management_help",
			"config.vercel.allow_env_management_help",
			"config.vercel.allow_domain_management_help",
			"config.vercel.default_project_id_help",
			"config.vercel.team_id_help",
			"config.vercel.team_slug_help",
		},
	}

	for file, markers := range checks {
		content := normalizeAssetText(mustReadUIFile(t, file))
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Fatalf("%s missing field description marker %q", file, marker)
			}
		}
	}

	ttsJS := normalizeAssetText(mustReadUIFile(t, "cfg/tts.js"))
	for _, marker := range []string{
		"ttsLanguageSelect('tts.language'",
		`<select class="field-select" data-path="`,
		"config.tts.language_custom_help",
		`data-custom-for="`,
	} {
		if !strings.Contains(ttsJS, marker) {
			t.Fatalf("tts.js missing language dropdown marker %q", marker)
		}
	}
	if strings.Contains(ttsJS, `<input class="field-input" type="text" data-path="tts.language"`) {
		t.Fatal("tts.language should be a dropdown with custom fallback, not a free text field")
	}

	telnyxJS := normalizeAssetText(mustReadUIFile(t, "cfg/telnyx.js"))
	for _, marker := range []string{
		`<select class="field-select" data-path="telnyx.voice_language"`,
		"help.telnyx.voice_language",
		`data-custom-for="telnyx.voice_language"`,
	} {
		if !strings.Contains(telnyxJS, marker) {
			t.Fatalf("telnyx.js missing voice language dropdown marker %q", marker)
		}
	}
	if strings.Contains(telnyxJS, `<input class="field-input" type="text" data-path="telnyx.voice_language"`) {
		t.Fatal("telnyx.voice_language should be a dropdown with custom fallback, not a free text field")
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

func TestConfigLegacyPatternAudit(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir("cfg")
	if err != nil {
		t.Fatalf("read cfg dir: %v", err)
	}

	forbidden := []string{
		"cfg-input-full",
		"cfg-save-btn-sm",
		"cfg-status-banner",
		"ig-test-status",
		"ig-flex-row",
		"cmp-status-line",
		"ai-gw-grid",
		"hp-grid-two",
		"nf-grid-2col",
	}
	// Match cfg-input on form controls only — not layout helpers like cfg-input-row.
	cfgInputMarkers := []string{
		`class="cfg-input"`,
		`class="cfg-input `,
		`class='cfg-input'`,
		`class='cfg-input `,
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		path := "cfg/" + entry.Name()
		content := normalizeAssetText(mustReadUIFile(t, path))
		for _, pattern := range forbidden {
			if strings.Contains(content, pattern) {
				t.Fatalf("%s still contains forbidden legacy pattern %q", path, pattern)
			}
		}
		for _, marker := range cfgInputMarkers {
			if strings.Contains(content, marker) {
				t.Fatalf("%s still uses cfg-input on form controls; use field-input/field-select", path)
			}
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

func TestConfigDirtyGuardAndHashNavigationMarkers(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"function hasUnsavedConfigChanges()",
		"let suppressDirtyTracking = false",
		"function resetDirtySnapshot()",
		"function normalizeSectionKey(key)",
		"async function confirmDiscardUnsavedChanges()",
		"async function navigateToConfigSection(key, options = {})",
		"function handleConfigBeforeUnload(event)",
		"function handleConfigHashChange()",
		"window.addEventListener('beforeunload', handleConfigBeforeUnload)",
		"window.addEventListener('hashchange', handleConfigHashChange)",
		"await selectSection(activeSection, { scrollBehavior: 'auto' });",
		"resetDirtySnapshot();",
		"suppressDirtyTracking = false;",
		"const dirty = collectSnapshot() !== initialSnapshot;",
		"setDirty(dirty);",
		"let userEditedSinceSnapshot = false",
		"const CONFIG_EDIT_INTENT_WINDOW_MS",
		"function installConfigEditIntentTracking()",
		"function scheduleDirtyBaselineRefresh",
		"markDirty(event)",
		"oninput=\"markDirty(event)\"",
		"autocomplete=\"new-password\"",
		"data-lpignore=\"true\"",
		"collectSnapshot() !== initialSnapshot",
		"t('config.unsaved_changes.title')",
		"navigateToConfigSection(s.key);",
		"navigateToConfigSection(item.dataset.section);",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing dirty guard marker %q", marker)
		}
	}
	if strings.Contains(mainJS, "setTimeout(() => { initialSnapshot = collectSnapshot(); setDirty(false); }, 100);") {
		t.Fatal("config main.js should not use a timed initial snapshot reset; it races async section rendering")
	}
}

func TestConfigMaintenanceHelpTextComplete(t *testing.T) {
	t.Parallel()

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"help.maintenance.enabled",
		"help.maintenance.time",
		"help.maintenance.lifeboat_enabled",
		"help.maintenance.lifeboat_port",
		"help.maintenance.retention.patterns_days",
		"help.maintenance.retention.archive_events_days",
		"help.maintenance.retention.mood_log_days",
		"help.maintenance.retention.error_patterns_days",
		"help.maintenance.retention.profile_stale_days",
		"help.maintenance.retention.done_notes_days",
		"help.maintenance.retention.operational_issues_days",
	}

	for _, lang := range langs {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("lang/help/" + lang + ".json")
			if err != nil {
				t.Fatalf("read help %s: %v", lang, err)
			}
			var help map[string]string
			if err := json.Unmarshal(data, &help); err != nil {
				t.Fatalf("parse help %s: %v", lang, err)
			}
			for _, key := range keys {
				if strings.TrimSpace(help[key]) == "" || help[key] == key {
					t.Fatalf("help/%s.json missing maintenance help %q", lang, key)
				}
			}
		})
	}
}

func TestConfigSaveBarStickyAndLiveStatusMarkers(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "config.html"))
	for _, marker := range []string{
		`id="saveStatus"`,
		`role="status"`,
		`aria-live="polite"`,
		`aria-atomic="true"`,
		`type="button"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("config.html missing save bar a11y marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	for _, marker := range []string{
		"--cfg-save-bar-height: 72px;",
		"--cfg-save-bar-height: 96px;",
		"position: fixed;",
		"bottom: 0;",
		"z-index: 30;",
		"scroll-padding-bottom: calc(var(--cfg-save-bar-height) + env(safe-area-inset-bottom, 0px));",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config.css missing sticky save bar marker %q", marker)
		}
	}
}

func TestConfigSidebarSemanticControlsMarkers(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"document.createElement('button')",
		"header.type = 'button';",
		"header.setAttribute('aria-expanded'",
		"item.type = 'button';",
		"el.setAttribute('aria-current', 'page')",
		"el.removeAttribute('aria-current')",
		`class="cfg-visually-hidden"`,
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing sidebar semantic marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	for _, marker := range []string{
		".cfg-visually-hidden",
		".sidebar-item:disabled",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("config.css missing sidebar semantic marker %q", marker)
		}
	}
}

func TestConfigToggleSwitchA11yMarkers(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		"function syncToggleA11y(toggle)",
		"function enhanceConfigControls(root = document)",
		"toggle.setAttribute('role', 'switch')",
		"toggle.setAttribute('aria-checked'",
		"event.key !== ' ' && event.key !== 'Enter'",
		"toggle.click();",
		"syncToggleA11y(el);",
		"enhanceConfigControls();",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing toggle a11y marker %q", marker)
		}
	}
}

func TestSetupSuccessConfigDeepLinksMarkers(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "setup.html"))
	for _, marker := range []string{
		`href="/config#providers"`,
		`href="/config#web_config"`,
		`href="/config#server"`,
		`href="/config#backup_restore"`,
		"setup.success_config_links_title",
		"setup.success_config_backup",
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("setup.html missing success deep link marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/setup.css"))
	for _, marker := range []string{
		".success-config-links",
		".success-config-link-grid",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("setup.css missing success deep link marker %q", marker)
		}
	}
}

