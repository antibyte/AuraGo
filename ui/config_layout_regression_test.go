package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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
		if !hasModuleMarker(css, marker) {
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
		if !hasModuleMarker(heartbeatJS, marker) {
			t.Fatalf("heartbeat.js missing help marker %q", marker)
		}
	}

	webhooksJS := normalizeAssetText(mustReadUIFile(t, "cfg/webhooks.js"))
	for _, marker := range []string{
		"t('help.webhooks.enabled')",
		"t('help.webhooks.max_payload_size')",
		"t('help.webhooks.rate_limit')",
	} {
		if !hasModuleMarker(webhooksJS, marker) {
			t.Fatalf("webhooks.js missing help marker %q", marker)
		}
	}

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	utilsJS := normalizeAssetText(mustReadUIFile(t, "js/config/utils.js"))
	configJS := mainJS + "\n" + utilsJS
	for _, marker := range []string{
		"cfgFieldOptionLabel",
		"config.field.disabled_option",
		"config.field.other_custom_option",
	} {
		if !hasModuleMarker(configJS, marker) {
			t.Fatalf("config main.js / utils.js missing marker %q", marker)
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
		if !hasModuleMarker(content, spec.marker) {
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
		"cfg/here_now.js":          {"/api/here-now/test-connection", "hereNowTestConnection"},
		"cfg/vercel.js":            {"/api/vercel/test-connection", "vercelTestConnection"},
	}
	for file, markers := range checks {
		content := normalizeAssetText(mustReadUIFile(t, file))
		for _, marker := range markers {
			if !hasModuleMarker(content, marker) {
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
		if !hasModuleMarker(emailJS, marker) {
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
		if !hasModuleMarker(secretsJS, marker) {
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
	for _, marker := range []string{"adg-password-row", "field-select", "tts-provider-section"} {
		if !hasModuleMarker(ttsJS, marker) {
			t.Fatalf("tts.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "tts.js", ttsJS, nil, []string{"piper-status-banner", "supertonic-status-banner"})
	requireDockSecret(t, "tts.js", ttsJS, "tts.elevenlabs.api_key", "tts.minimax.api_key", "tts.mistral.api_key")

	gwJS := normalizeAssetText(mustReadUIFile(t, "cfg/google_workspace.js"))
	for _, marker := range []string{"adg-password-row", "adg-test-btn", "AuraConfigForm.toggle("} {
		if !hasModuleMarker(gwJS, marker) {
			t.Fatalf("google_workspace.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "google_workspace.js", gwJS, []string{"gw-oauth-status"}, nil)
	if strings.Contains(gwJS, "gw-status-line") {
		t.Fatal("google_workspace.js must not use the legacy gw-status-line class")
	}
}

func TestConfigPhase2cManifestDograhActionRows(t *testing.T) {
	t.Parallel()

	manifestJS := normalizeAssetText(mustReadUIFile(t, "cfg/manifest.js"))
	for _, marker := range []string{
		"cfg-actions-row",
		"CFG_OPTION_OTHER_CUSTOM",
		"cfgFieldOptionLabel",
		"manifestStatusState",
	} {
		if !hasModuleMarker(manifestJS, marker) {
			t.Fatalf("manifest.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "manifest.js", manifestJS, []string{"manifest-result"}, []string{"manifest-status-panel"})

	dograhJS := normalizeAssetText(mustReadUIFile(t, "cfg/dograh.js"))
	for _, marker := range []string{
		"cfg-actions-row",
		"config.dograh.mcp_section_title",
	} {
		if !hasModuleMarker(dograhJS, marker) {
			t.Fatalf("dograh.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "dograh.js", dograhJS, []string{"dograh-test-result", "dograh-mcp-result", "dograh-webhook-result"}, []string{"dograh-status-box"})
	if strings.Contains(dograhJS, "field-row") {
		t.Fatal("dograh.js should use field-group layout instead of field-row")
	}
}

func TestConfigPhase3MQTTAndNetlifyActionRows(t *testing.T) {
	t.Parallel()

	mqttJS := normalizeAssetText(mustReadUIFile(t, "cfg/mqtt.js"))
	for _, marker := range []string{
		"adg-test-btn",
		"cfg-actions-row",
		"field-select",
		"field-grid two-cols",
		"adg-password-row",
		"/api/mqtt/test",
	} {
		if !hasModuleMarker(mqttJS, marker) {
			t.Fatalf("mqtt.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "mqtt.js", mqttJS, []string{"mqtt-test-result"}, []string{"mqtt-status-banner"})
	requireDockSecret(t, "mqtt.js", mqttJS, "mqtt.password")
	if strings.Contains(mqttJS, "mqttSetBanner") {
		t.Fatal("mqtt.js must use cfgStatusBanner instead of its own banner setter")
	}
	if strings.Contains(mqttJS, `class="field-input" data-path="mqtt.qos"`) {
		t.Fatal("mqtt.js QoS select should use field-select")
	}

	netlifyJS := normalizeAssetText(mustReadUIFile(t, "cfg/netlify.js"))
	for _, marker := range []string{
		"adg-test-btn",
		"cfg-actions-row",
		"nf-test-btn",
		"/api/netlify/test-connection",
		"field-grid two-cols",
	} {
		if !hasModuleMarker(netlifyJS, marker) {
			t.Fatalf("netlify.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "netlify.js", netlifyJS, []string{"nf-test-result"}, []string{"nf-status-banner"})
	requireDockSecret(t, "netlify.js", netlifyJS, "netlify.token")
	if strings.Contains(netlifyJS, "nf-test-spinner") || strings.Contains(netlifyJS, "nf-test-msg") {
		t.Fatal("netlify.js should use the cfgActionResult line instead of legacy spinner/msg blocks")
	}
	if strings.Contains(netlifyJS, "cfg-input cfg-input-full") {
		t.Fatal("netlify.js site fields should use field-input inside field-grid")
	}
	if strings.Contains(netlifyJS, "nf-grid-2col") {
		t.Fatal("netlify.js permission toggles should use field-grid two-cols instead of nf-grid-2col")
	}
}

func TestNetlifyDeployToggleAndTemplateDefaultAreConservative(t *testing.T) {
	t.Parallel()

	netlifyJS := normalizeAssetText(mustReadUIFile(t, "cfg/netlify.js"))
	if !strings.Contains(netlifyJS, "cfg.allow_deploy === true") {
		t.Fatal("netlify.js deploy toggle should only render active when allow_deploy is explicitly true")
	}
	if strings.Contains(netlifyJS, "cfg.allow_deploy !== false") {
		t.Fatal("netlify.js must not treat a missing allow_deploy value as enabled")
	}

	template, err := os.ReadFile("../config_template.yaml")
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}
	text := normalizeAssetText(template)
	start := strings.Index(text, "netlify:")
	end := strings.Index(text, "vercel:")
	if start < 0 || end <= start {
		t.Fatalf("could not locate netlify block in config_template.yaml")
	}
	netlifyBlock := text[start:end]
	if !strings.Contains(netlifyBlock, "allow_deploy: false") {
		t.Fatalf("netlify allow_deploy should default to false for new installs; block:\n%s", netlifyBlock)
	}
}

func TestConfigPhase3VercelActionRows(t *testing.T) {
	t.Parallel()

	vercelJS := normalizeAssetText(mustReadUIFile(t, "cfg/vercel.js"))
	for _, marker := range []string{
		"adg-test-btn",
		"cfg-actions-row",
		"vercel-test-btn",
		"/api/vercel/test-connection",
		"field-grid two-cols",
	} {
		if !hasModuleMarker(vercelJS, marker) {
			t.Fatalf("vercel.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "vercel.js", vercelJS, []string{"vercel-test-result"}, []string{"vercel-status-banner"})
	requireDockSecret(t, "vercel.js", vercelJS, "vercel.token")
	if strings.Contains(vercelJS, "adg-save-btn") {
		t.Fatal("vercel.js must not render a vault save button")
	}
	if strings.Contains(vercelJS, "vercel-test-spinner") || strings.Contains(vercelJS, "vercel-test-msg") {
		t.Fatal("vercel.js should use the cfgActionResult line instead of legacy spinner/msg blocks")
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
		"fbSetBanner",
		"adg-password-row",
		"cfg-actions-row",
		"adg-test-btn",
		"attachChangeListeners",
		"status_not_configured",
		"/api/fritzbox/status?check=1",
		"/api/fritzbox/test",
		"fritzbox.web_port",
		"fritzbox.insecure_skip_verify",
	} {
		if !hasModuleMarker(fritzboxJS, marker) {
			t.Fatalf("fritzbox.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "fritzbox.js", fritzboxJS, []string{"fb-test-result"}, []string{"fb-status-banner"})
	requireDockSecret(t, "fritzbox.js", fritzboxJS, "fritzbox.password")
	// The test sends the typed values as a draft and reports on its own result line.
	for _, marker := range []string{"cfgDraftPatch(['fritzbox'])", "target: 'fb-test-result'"} {
		if !strings.Contains(fritzboxJS, marker) {
			t.Fatalf("fritzbox.js test must send the draft and target its result line: missing %q", marker)
		}
	}
	if strings.Contains(fritzboxJS, "adg-save-btn") {
		t.Fatal("fritzbox.js must not render a vault save button")
	}
	successStart := strings.Index(fritzboxJS, "if (res.status === 'ok') {")
	if successStart < 0 {
		t.Fatal("fritzbox.js missing test success branch")
	}
	successBranch := fritzboxJS[successStart:]
	successEnd := strings.Index(successBranch, "} else {")
	if successEnd < 0 {
		t.Fatal("fritzbox.js missing test failure branch after success branch")
	}
	successBranch = successBranch[:successEnd]
	if strings.Contains(successBranch, "fbCheckStatus()") {
		t.Fatal("fritzbox.js should not replace a successful form-value test with saved-config status")
	}

	githubJS := normalizeAssetText(mustReadUIFile(t, "cfg/github.js"))
	for _, marker := range []string{
		"adg-password-row",
		"adg-test-btn",
		"cfg-actions-row",
		"github-fetch-status",
		"data-repo-full-name",
		"dataset.repoFullName",
		"btn-save btn-secondary",
	} {
		if !hasModuleMarker(githubJS, marker) {
			t.Fatalf("github.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "github.js", githubJS, []string{"github-test-result", "github-fetch-status"}, nil)
	requireDockSecret(t, "github.js", githubJS, "github.token")
	if strings.Contains(githubJS, "cfg-save-btn-sm") || strings.Contains(githubJS, "cfg-status-banner") || strings.Contains(githubJS, "github-token-status") || strings.Contains(githubJS, "adg-save-btn") {
		t.Fatal("github.js should use unified adg-* and btn-secondary patterns without a vault save button")
	}
}

func TestConfigPhase3TailscaleA2AVaultRows(t *testing.T) {
	t.Parallel()

	tailscaleJS := normalizeAssetText(mustReadUIFile(t, "cfg/tailscale.js"))
	for _, marker := range []string{
		"adg-password-row",
		"tsnet-action-result",
	} {
		if !hasModuleMarker(tailscaleJS, marker) {
			t.Fatalf("tailscale.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "tailscale.js", tailscaleJS, []string{"ts-api-test-result", "tsnet-action-result"}, []string{"ts-api-status-banner"})
	requireDockSecret(t, "tailscale.js", tailscaleJS, "tailscale.api_key", "tailscale.tsnet.auth_key")
	for _, removed := range []string{"cfg-save-btn-sm", "ts-key-status", "ts-api-key-status", "ts-auth-key-status", "adg-save-btn"} {
		if strings.Contains(tailscaleJS, removed) {
			t.Fatalf("tailscale.js still carries the removed vault row marker %q", removed)
		}
	}

	a2aJS := normalizeAssetText(mustReadUIFile(t, "cfg/a2a.js"))
	for _, marker := range []string{
		"adg-password-row",
	} {
		if !hasModuleMarker(a2aJS, marker) {
			t.Fatalf("a2a.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "a2a.js", a2aJS, []string{"a2a-test-result"}, []string{"a2a-status-banner"})
	requireDockSecret(t, "a2a.js", a2aJS, "a2a.auth.api_key", "a2a.auth.bearer_secret")
	for _, removed := range []string{"cfg-save-btn-sm", "a2a-status-text", "a2a-bearer-secret-status", "adg-save-btn", "a2aSaveVault"} {
		if strings.Contains(a2aJS, removed) {
			t.Fatalf("a2a.js still carries the removed vault row marker %q", removed)
		}
	}
}

func TestConfigPhase3MediaGenerationModules(t *testing.T) {
	t.Parallel()

	specs := []struct {
		file     string
		testAPI  string
		testFn   string
		resultID string
		wantGrid bool
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
			spec.resultID,
			"field-select",
			spec.testAPI,
			spec.testFn,
		}
		if spec.wantGrid {
			markers = append(markers, "field-grid two-cols", "field-input")
		}
		for _, marker := range markers {
			if !hasModuleMarker(content, marker) {
				t.Fatalf("%s missing marker %q", spec.file, marker)
			}
		}
		if strings.Contains(content, "cfg-save-btn-sm") || strings.Contains(content, "ig-test-status") || strings.Contains(content, "ig-flex-row") {
			t.Fatalf("%s should use unified cfg-actions-row/cfgActionResult patterns", spec.file)
		}
		requireFeedbackContract(t, spec.file, content, []string{spec.resultID}, nil)
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
		if !hasModuleMarker(mcpJS, marker) {
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
		"cfg-actions-row",
		"adg-test-btn",
		"adg-password-row",
		"field-grid two-cols",
		"/api/composio/test",
	} {
		if !hasModuleMarker(composioJS, marker) {
			t.Fatalf("composio.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "composio.js", composioJS, []string{"composio-test-result", "composio-connect-status", "composio-modal-result"}, []string{"composio-status-banner"})
	requireDockSecret(t, "composio.js", composioJS, "composio.api_key")
	if strings.Contains(composioJS, "adg-save-btn") {
		t.Fatal("composio.js must not render a vault save button")
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
	} {
		if !hasModuleMarker(emailJS, marker) {
			t.Fatalf("email.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "email.js", emailJS, []string{"ea-test-result"}, nil)
	if strings.Contains(emailJS, "cfg-save-btn-sm") {
		t.Fatal("email.js should not use cfg-save-btn-sm")
	}

	serverJS := normalizeAssetText(mustReadUIFile(t, "cfg/server.js"))
	for _, marker := range []string{
		"field-grid two-cols",
		"field-input",
		"field-select",
	} {
		if !hasModuleMarker(serverJS, marker) {
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
		if !hasModuleMarker(securityProxyJS, marker) {
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
		"adg-test-btn",
		"cfg-actions-row",
		"/api/homepage/test-connection",
		"hpTestConnection",
	} {
		if !hasModuleMarker(homepageJS, marker) {
			t.Fatalf("homepage.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "homepage.js", homepageJS, []string{"hp-test-result"}, nil)
	requireDockSecret(t, "homepage.js", homepageJS, "homepage.deploy_password", "homepage.deploy_key")
	if strings.Contains(homepageJS, "cfg-input") || strings.Contains(homepageJS, "hp-test-spinner") {
		t.Fatal("homepage.js should use unified field-* patterns and the cfgActionResult line")
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
		"/api/remote/enroll",
		"rcCreateEnrollmentToken",
	} {
		if !hasModuleMarker(remoteControlJS, marker) {
			t.Fatalf("remote_control.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "remote_control.js", remoteControlJS, []string{"rc-token-status", "rc-token-copy-state"}, nil)
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
		"cfg.max_checks_per_min != null",
	} {
		if !hasModuleMarker(llmGuardianJS, marker) {
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
		"field-grid two-cols",
		"field-input",
		"cfg-actions-row",
		"adg-test-btn",
		"/api/ai-gateway/test",
		"aiGatewayTestConnection",
		"ai_gateway.log_mode",
		"ai_gateway.request_timeout_ms",
		"ai_gateway.huggingface_provider_slug",
		"provider_id",
		"probe_kind",
		"AbortController",
		"aiGatewayRenderMetadataRows",
		"ai_gateway.metadata",
	} {
		if !hasModuleMarker(aiGatewayJS, marker) {
			t.Fatalf("ai_gateway.js missing marker %q", marker)
		}
	}
	requireFeedbackContract(t, "ai_gateway.js", aiGatewayJS, []string{"ai-gw-test-result"}, []string{"ai-gw-status-banner"})
	requireDockSecret(t, "ai_gateway.js", aiGatewayJS, "ai_gateway.token")
	if strings.Contains(aiGatewayJS, "cfg-input") || strings.Contains(aiGatewayJS, "ai-gw-grid") {
		t.Fatal("ai_gateway.js should use field-grid and field-input instead of legacy ai-gw patterns")
	}
	if strings.Contains(aiGatewayJS, "is-neutral") {
		t.Fatal("ai_gateway.js should not use nonexistent is-neutral banner state")
	}
}

func TestConfigAIGatewayI18NComplete(t *testing.T) {
	t.Parallel()

	configKeys := []string{
		"config.ai_gateway.provider_select",
		"config.ai_gateway.model_test",
		"config.ai_gateway.untested",
		"config.ai_gateway.skipped",
		"config.ai_gateway.save_first",
		"config.ai_gateway.hf_slug",
		"config.ai_gateway.hf_help",
		"config.ai_gateway.rest_help",
		"config.ai_gateway.retry_info",
		"config.ai_gateway.no_provider",
		"config.ai_gateway.model_test_success",
		"config.ai_gateway.connection_metadata",
		"config.ai_gateway.route_skipped",
		"config.ai_gateway.metadata_invalid",
		"config.ai_gateway.log_mode",
		"config.ai_gateway.log_mode_metadata_only",
		"config.ai_gateway.log_mode_off",
		"config.ai_gateway.log_mode_full",
		"config.ai_gateway.request_timeout_ms",
		"config.ai_gateway.metadata",
		"config.ai_gateway.metadata_key",
		"config.ai_gateway.metadata_value",
		"config.ai_gateway.metadata_add",
		"config.ai_gateway.status_configured",
		"config.ai_gateway.status_unsupported",
		"config.ai_gateway.status_live_ok",
		"config.ai_gateway.status_live_failed",
	}
	helpKeys := []string{
		"help.ai_gateway.log_mode",
		"help.ai_gateway.request_timeout_ms",
		"help.ai_gateway.metadata",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			configData, err := os.ReadFile("lang/config/ai_gateway/" + lang + ".json")
			if err != nil {
				t.Fatalf("read config ai_gateway %s: %v", lang, err)
			}
			var configValues map[string]string
			if err := json.Unmarshal(configData, &configValues); err != nil {
				t.Fatalf("parse config ai_gateway %s: %v", lang, err)
			}
			for _, key := range configKeys {
				if strings.TrimSpace(configValues[key]) == "" || configValues[key] == key {
					t.Fatalf("lang/config/ai_gateway/%s.json missing %q", lang, key)
				}
			}

			helpData, err := os.ReadFile("lang/help/" + lang + ".json")
			if err != nil {
				t.Fatalf("read help %s: %v", lang, err)
			}
			var helpValues map[string]string
			if err := json.Unmarshal(helpData, &helpValues); err != nil {
				t.Fatalf("parse help %s: %v", lang, err)
			}
			for _, key := range helpKeys {
				if strings.TrimSpace(helpValues[key]) == "" || helpValues[key] == key {
					t.Fatalf("lang/help/%s.json missing %q", lang, key)
				}
			}
		})
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
			if !hasModuleMarker(content, marker) {
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
		if !hasModuleMarker(ttsJS, marker) {
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
		if !hasModuleMarker(telnyxJS, marker) {
			t.Fatalf("telnyx.js missing voice language dropdown marker %q", marker)
		}
	}
	if strings.Contains(telnyxJS, `<input class="field-input" type="text" data-path="telnyx.voice_language"`) {
		t.Fatal("telnyx.voice_language should be a dropdown with custom fallback, not a free text field")
	}
}

func TestCloudflareTunnelRestartTreatsNonOKAsError(t *testing.T) {
	t.Parallel()

	content := normalizeAssetText(mustReadUIFile(t, "cfg/cloudflare_tunnel.js"))
	// The action runs through cfgActionResult; a non-ok body turns into an error line with the server reason.
	requireFeedbackContract(t, "cloudflare_tunnel.js", content, []string{"cf-action-status"}, []string{"cf-tunnel-status-banner"})
	requireDockSecret(t, "cloudflare_tunnel.js", content, "cloudflare_tunnel.token")
	for _, marker := range []string{
		"interpret: o =>",
		"state: 'error', reason:",
		"target: 'cf-action-status'",
	} {
		if !hasModuleMarker(content, marker) {
			t.Fatalf("cloudflare tunnel restart UI missing error contract marker %q", marker)
		}
	}
}

func TestConfigManifestDograhAvoidEmbeddedFallbackTables(t *testing.T) {
	t.Parallel()

	for _, spec := range []struct {
		file      string
		forbidden string
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

	// Legacy result/banner classes (ids with the same names are fine): a class attribute, classList
	// call, class argument of cfgActionResult.html or a selector that names one of them.
	legacyFeedbackClass := `(?:adg-test-result|adg-status-banner|dc-test-result|em-test-result|a2a-test-result|a2a-status-banner|ldap-test-result|[a-z0-9]+-test-result)`
	legacyFeedbackClassUse := []*regexp.Regexp{
		regexp.MustCompile(`class(?:Name)?\s*=\s*["'][^"']*` + legacyFeedbackClass),
		regexp.MustCompile(`classList\.\w+\([^)]*` + legacyFeedbackClass),
		regexp.MustCompile(`\.html\([^)]*,\s*['"][^'"]*` + legacyFeedbackClass),
		regexp.MustCompile(`(?:querySelector(?:All)?|closest|matches)\(\s*['"][^'"]*\.` + legacyFeedbackClass),
	}
	nativeDialog := regexp.MustCompile(`(?:^|[^\w.$])(?:window\.)?(?:confirm|alert)\(`)

	// Modules the wave 1 plan left out on purpose (rich status panels, already good, not yet planned).
	// They may keep their own result/banner classes and native dialogs until their own pass.
	notMigratedInWave1 := map[string]bool{
		"meshcore.js":        true, // already good, not in wave 1
		"ruview.js":          true, // already good, not in wave 1
		"s3.js":              true, // already good, not in wave 1
		"speech_lab.js":      true, // progress UI follows later
		"telephone_agent.js": true, // rich status panel
		"security_proxy.js":  true, // rich status panel
		"server.js":          true, // rich status panel
		"sandbox.js":         true, // rich status panel
		"daemon_skills.js":   true, // rich status panel
		"updates.js":         true, // own update flow
		"secrets.js":         true, // vault manager
		"skill_manager.js":   true, // own list UI
		"local_wikipedia.js": true, // rich status panel
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
		if !notMigratedInWave1[entry.Name()] {
			for _, re := range legacyFeedbackClassUse {
				if loc := re.FindStringIndex(content); loc != nil {
					t.Errorf("%s uses a legacy result/banner class (%q); use cfgActionResult/cfgStatusBanner (ids are fine)", path, content[loc[0]:loc[1]])
				}
			}
			if loc := nativeDialog.FindStringIndex(content); loc != nil {
				t.Errorf("%s uses a native confirm()/alert(); use showConfirm/showAlert (%q)", path, strings.TrimSpace(content[loc[0]:loc[1]]))
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
	utilsJS := normalizeAssetText(mustReadUIFile(t, "js/config/utils.js"))
	configJS := mainJS + "\n" + utilsJS
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
		if !hasModuleMarker(configJS, marker) {
			t.Fatalf("config main.js / utils.js missing dirty guard marker %q", marker)
		}
	}
	if strings.Contains(mainJS, "setTimeout(() => { initialSnapshot = collectSnapshot(); setDirty(false); }, 100);") {
		t.Fatal("config main.js should not use a timed initial snapshot reset; it races async section rendering")
	}
	for _, marker := range []string{
		"function isTrustedDiscreteControlChange(event)",
		"if (isTrustedDiscreteControlChange(event)) return true;",
		"if (window.AuraConfigState && window.AuraConfigState.isDirty()) return;",
		"Never hide the save bar while the draft still has changes",
		"el.dataset.cfgDirtyBound === 'true'",
		".field-textarea, .cfg-input, [data-path]",
	} {
		if !hasModuleMarker(mainJS, marker) {
			t.Fatalf("config main.js missing dropdown dirty-tracking marker %q", marker)
		}
	}
}

func TestConfigDataPathSelectsUseFieldSelect(t *testing.T) {
	t.Parallel()

	cfgDir := filepath.Join("cfg")
	entries, err := os.ReadDir(cfgDir)
	if err != nil {
		t.Fatalf("read cfg dir: %v", err)
	}
	re := regexp.MustCompile(`<select\s+[^>]*data-path=`)
	badClass := regexp.MustCompile(`class="[^"]*field-input[^"]*"`)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		path := filepath.Join(cfgDir, entry.Name())
		raw := string(mustReadUIFile(t, filepath.ToSlash(path)))
		// Skip modal-only selects without data-path; scan line-ish chunks around data-path selects.
		for _, match := range re.FindAllString(raw, -1) {
			if badClass.MatchString(match) && !strings.Contains(match, "field-select") {
				t.Fatalf("%s: data-path select should use field-select, got %q", entry.Name(), match)
			}
		}
	}
}

func TestConfigMaintenanceHelpTextComplete(t *testing.T) {
	t.Parallel()

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"help.maintenance.enabled",
		"help.maintenance.time",
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
		if !hasModuleMarker(html, marker) {
			t.Fatalf("config.html missing save bar a11y marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config-workspace.css"))
	for _, marker := range []string{
		"position: relative;",
		"flex: 0 0 auto;",
		"min-height: 64px;",
		"env(safe-area-inset-bottom",
		"z-index: 10;",
	} {
		if !hasModuleMarker(css, marker) {
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
		if !hasModuleMarker(mainJS, marker) {
			t.Fatalf("config main.js missing sidebar semantic marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/config.css"))
	for _, marker := range []string{
		".cfg-visually-hidden",
		".sidebar-item:disabled",
	} {
		if !hasModuleMarker(css, marker) {
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
		if !hasModuleMarker(mainJS, marker) {
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
		if !hasModuleMarker(html, marker) {
			t.Fatalf("setup.html missing success deep link marker %q", marker)
		}
	}

	css := normalizeAssetText(mustReadUIFile(t, "css/setup.css"))
	for _, marker := range []string{
		".success-config-links",
		".success-config-link-grid",
	} {
		if !hasModuleMarker(css, marker) {
			t.Fatalf("setup.css missing success deep link marker %q", marker)
		}
	}
}

// hasModuleMarker reports whether content carries the literal marker. All config modules
// are on the shared feedback helpers now, so there is no alternative spelling to accept.
func hasModuleMarker(content, marker string) bool {
	return strings.Contains(content, marker)
}

// requireFeedbackContract checks the shared feedback contract of a migrated module: its result
// lines come from cfgActionResult (helper call plus the id the module renders) and its banners
// from cfgStatusBanner.
func requireFeedbackContract(t *testing.T, file, content string, resultIDs []string, bannerIDs []string) {
	t.Helper()
	if len(resultIDs) > 0 && !strings.Contains(content, "cfgActionResult(") {
		t.Fatalf("%s must run its actions through cfgActionResult(", file)
	}
	for _, id := range resultIDs {
		if !strings.Contains(content, "cfgActionResult.html('"+id+"'") {
			t.Fatalf("%s must render the result line %q with cfgActionResult.html", file, id)
		}
	}
	for _, id := range bannerIDs {
		if !strings.Contains(content, "cfgStatusBanner.html('"+id+"'") {
			t.Fatalf("%s must render the banner %q with cfgStatusBanner.html", file, id)
		}
	}
}

// requireDockSecret checks that secrets are bound to the save dock through data-path and that the
// module has no own vault POST any more.
func requireDockSecret(t *testing.T, file, content string, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if !strings.Contains(content, `data-path="`+path+`"`) && !strings.Contains(content, "'"+path+"'") && !strings.Contains(content, `"`+path+`"`) {
			t.Fatalf("%s must bind the secret %q to the save dock through data-path", file, path)
		}
	}
	if strings.Contains(content, "/api/vault/secrets") {
		t.Fatalf("%s still posts secrets to /api/vault/secrets; the save dock writes them", file)
	}
}
