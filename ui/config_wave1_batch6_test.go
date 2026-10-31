package ui

import (
	"strings"
	"testing"
)

// Config UX wave 1, batch 6: google_workspace, onedrive, sql_connections, auth, auth_webauthn,
// backup, indexing, mcp_server, prompts, rules and devices report through the shared result
// line (busy button, server reason), and the connection tests send the edited values.
func TestConfigWave1Batch6StaticContracts(t *testing.T) {
	t.Parallel()

	read := func(module string) string { return string(mustReadUIFile(t, "cfg/"+module+".js")) }
	must := func(module, src string, wants ...string) {
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Errorf("%s.js missing %q", module, want)
			}
		}
	}
	banned := func(module, src string, bans ...string) {
		for _, ban := range bans {
			if strings.Contains(src, ban) {
				t.Errorf("%s.js still contains %q", module, ban)
			}
		}
	}

	gw := read("google_workspace")
	must("google_workspace", gw, "cfgActionResult(", "cfgStatusBanner.check(", "cfgDraftPatch(['google_workspace'])",
		`data-config-action-draft="true"`, "/api/google-workspace/test", `data-path="google_workspace.client_secret"`, "gw-test-result")
	banned("google_workspace", gw, "/api/vault/secrets", "gwSaveSecret", "gw-secret-status", "save_vault")

	od := read("onedrive")
	must("onedrive", od, "cfgActionResult(", "cfgStatusBanner.check(", "cfgDraftPatch(['onedrive'])",
		`data-config-action-draft="true"`, "/api/onedrive/test", "od-test-result")
	banned("onedrive", od, "odSetStatus", "⏳", "✅", "❌", "⚪")

	sql := read("sql_connections")
	must("sql_connections", sql, "cfgActionResult(", `id="sqlconn-test-btn" data-config-action-draft="true"`,
		"sqlConnPayload()", "'new'", "sqlconn-test-result", "sqlconn-save-result", "sqlconn-list-result", `data-cfg-confirm-enable="delete"`)
	banned("sql_connections", sql, "save_before_test", "sqlconn-modal-error", "sqlconn-modal-success", "showToast(")

	for module, wants := range map[string][]string{
		"auth":          {"cfgActionResult(", "auth-set-pw-btn", "auth-totp-confirm-btn", "sec-harden-result"},
		"auth_webauthn": {"cfg-action-result"},
		"backup":        {"cfgActionResult(", "backup-create-status", "backup-import-status", "showConfirm("},
		"indexing":      {"cfgActionResult(", "idx-rescan-msg", "idx-dir-msg", "idx-add-btn"},
		"mcp_server":    {"cfgActionResult(", "mcp-preset-status", "mcp-token-error"},
		"prompts":       {"cfgActionResult(", "pers-editor-status", "pers-save-btn"},
		"rules":         {"cfgActionResult(", "rules-status", "rules-save-btn"},
		"devices":       {"cfgActionResult(", "device-mac-status", "device-modal-error", "devices-list-result"},
	} {
		must(module, read(module), wants...)
	}
	banned("backup", read("backup"), "btn.disabled = true", "btn.disabled = false")
	banned("indexing", read("indexing"), "setAttribute('aria-busy', 'true')")
	banned("devices", read("devices"), "devices-mac-ok", "devices-mac-warn", "showToast(")
	banned("rules", read("rules"), "showToast(rulesText('config.rules.save_failed'")
	banned("auth", read("auth"), "btn.textContent = t('config.security.applying')", "showToast(t('config.common.network_error')")
}

func TestConfigWave1Batch6ModulesBrowser(t *testing.T) {
	requirePrecisionBrowserSmoke(t)
	browser := newSmokeBrowser(t)
	page := browser.MustPage(configRefreshOrigin(t) + "/config#overview")
	defer func() {
		page.MustEval(`() => window.removeEventListener('beforeunload', handleConfigBeforeUnload)`)
		page.MustClose()
	}()
	waitForJSBool(t, page, `() => !!document.querySelector('.pw-overview-card')`)
	page.MustSetViewport(1440, 900, 1, false)

	// Stub every request of the batch's endpoints; unknown URLs go to the fixture server.
	page.MustEval(`() => {
		const original = window.fetch;
		window.b6Calls = [];
		window.b6Responses = {};
		window.fetch = (url, init) => {
			const key = String(url);
			const stub = window.b6Responses[key];
			if (stub) {
				window.b6Calls.push({url: key, method: (init && init.method) || 'GET', body: init && init.body});
				return Promise.resolve(new Response(stub.body, {status: stub.status || 200, headers: {'Content-Type': stub.type || 'application/json'}}));
			}
			return original(url, init);
		};
		window.b6Result = async (button, id, fn) => {
			const el = document.getElementById(id);
			if (button) document.getElementById(button).dataset.configActionLocked = 'false';
			const outcome = await fn();
			return {state: el.dataset.state, text: el.textContent, busy: button ? document.getElementById(button).dataset.configActionBusy : '', skipped: outcome && outcome.skipped};
		};
	}`)
	stub := func(url string, status int, body string) {
		page.MustEval(`(url, status, body) => { window.b6Responses[url] = {status, body}; window.b6Calls = []; }`, url, status, body)
	}
	type outcome struct{ State, Text, Busy string }
	run := func(js string, args ...interface{}) outcome {
		m := page.MustEval(js, args...).Map()
		return outcome{m["state"].Str(), m["text"].Str(), m["busy"].Str()}
	}

	t.Run("google_workspace", func(t *testing.T) {
		page.MustEval(`async () => { await selectSection('google_workspace', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
		state := page.MustEval(`() => ({
			banner: document.getElementById('gw-status-banner').classList.contains('cfg-state-banner'),
			secret: !!document.querySelector('input[type="password"][data-path="google_workspace.client_secret"]'),
			vault: document.querySelectorAll('#content [onclick*="gwSaveSecret"]').length,
			draft: document.getElementById('gw-test-btn').dataset.configActionDraft === 'true',
			result: document.getElementById('gw-test-result').classList.contains('cfg-action-result')
		})`).Map()
		if !state["banner"].Bool() || !state["secret"].Bool() || state["vault"].Int() != 0 || !state["draft"].Bool() || !state["result"].Bool() {
			t.Fatalf("google_workspace markup: %v", state)
		}
		stub("/api/google-workspace/test", 502, `{"status":"error","message":"Fixture says no"}`)
		got := run(`() => b6Result('gw-test-btn', 'gw-test-result', () => gwTestConnection())`)
		if got.State != "error" || !strings.Contains(got.Text, "Fixture says no") || got.Busy == "true" {
			t.Errorf("google_workspace test result = %+v", got)
		}
		sent := page.MustEval(`() => ({method: b6Calls[0].method, draft: JSON.parse(b6Calls[0].body).draft})`).Map()
		if sent["method"].Str() != "POST" || sent["draft"].Nil() {
			t.Errorf("google_workspace test did not post the draft: %v", sent)
		}
	})

	t.Run("sql_connections", func(t *testing.T) {
		page.MustEval(`async () => { await selectSection('sql_connections', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
		stub("/api/sql-connections", 200, `[]`)
		stub("/api/sql-connections/new/test", 400, `{"status":"error","message":"Connection test failed. Check host."}`)
		page.MustEval(`() => { sqlConnShowModal(); document.getElementById('sqlconn-field-name').value = 'Typed'; document.getElementById('sqlconn-field-host').value = 'db.example'; document.getElementById('sqlconn-field-database').value = 'app'; document.getElementById('sqlconn-field-password').value = 'typed-secret'; }`)
		got := run(`() => b6Result('sqlconn-test-btn', 'sqlconn-test-result', () => sqlConnTest())`)
		if got.State != "error" || !strings.Contains(got.Text, "Check host") || got.Busy == "true" {
			t.Errorf("sql test result = %+v", got)
		}
		body := page.MustEval(`() => JSON.parse(b6Calls[0].body)`).Map()
		if body["host"].Str() != "db.example" || body["password"].Str() != "typed-secret" || body["database_name"].Str() != "app" {
			t.Errorf("sql test did not send the typed record: %v", body)
		}
	})

	t.Run("backup", func(t *testing.T) {
		page.MustEval(`async () => { await selectSection('backup_restore', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
		stub("/api/backup/create", 500, `disk full`)
		got := run(`() => b6Result('backup-create-btn', 'backup-create-status', () => backupCreate())`)
		if got.State != "error" || !strings.Contains(got.Text, "disk full") || got.Busy == "true" {
			t.Errorf("backup create result = %+v", got)
		}
	})

	t.Run("indexing", func(t *testing.T) {
		stub("/api/indexing/status", 200, `{"enabled":true,"status":{"running":true,"total_files":1,"indexed_files":1}}`)
		stub("/api/indexing/directories", 200, `{"directories":[]}`)
		page.MustEval(`async () => { await selectSection('indexing', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
		stub("/api/indexing/rescan", 409, `{"error":"scan already running"}`)
		got := run(`() => b6Result('idx-rescan-btn', 'idx-rescan-msg', () => idxRescan())`)
		if got.State != "error" || !strings.Contains(got.Text, "scan already running") || got.Busy == "true" {
			t.Errorf("indexing rescan result = %+v", got)
		}
	})

	t.Run("mcp_server", func(t *testing.T) {
		page.MustEval(`async () => { configData.mcp_server = Object.assign({}, configData.mcp_server, {enabled: true, vscode_debug_bridge: true}); await selectSection('mcp_server', {scrollBehavior:'auto'}); }`)
		stub("/api/mcp-server/tools", 200, `["ask_aurago"]`)
		stub("/api/mcp-server/vscode-bridge", 200, `{"recommended_tools":["ask_aurago"],"clients":{}}`)
		got := run(`() => b6Result('mcp-apply-preset', 'mcp-preset-status', () => mcpApplyBridgePreset(document.getElementById('mcp-apply-preset')))`)
		if got.State != "success" || got.Text == "" || got.Busy == "true" {
			t.Errorf("mcp preset result = %+v", got)
		}
		page.MustEval(`() => { resetDirtySnapshot(); }`)
	})

	t.Run("web_config", func(t *testing.T) {
		stub("/api/auth/status", 200, `{"enabled":true,"password_set":true,"totp_enabled":false,"authenticated":true}`)
		stub("/api/security/hints", 200, `{"hints":[]}`)
		page.MustEval(`async () => { await selectSection('web_config', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
		stub("/api/auth/password", 403, `{"error":"current password is wrong"}`)
		got := run(`() => { document.getElementById('auth-new-pw').value = 'longenough1'; document.getElementById('auth-current-pw').value = 'x'; return b6Result('auth-set-pw-btn', 'auth-pw-msg', () => authSetPassword()); }`)
		if got.State != "error" || !strings.Contains(got.Text, "current password is wrong") || got.Busy == "true" {
			t.Errorf("auth password result = %+v", got)
		}
	})

	t.Run("rules", func(t *testing.T) {
		stub("/api/config/rules", 200, `{"enabled":true,"rules":[{"id":"r1","title":"R1","enabled":true}],"candidates":{}}`)
		stub("/api/config/rules/r1", 200, `{"rule":{"id":"r1","title":"R1","enabled":true,"priority":1,"body":"x"},"design":""}`)
		page.MustEval(`async () => { await selectSection('rules', {scrollBehavior:'auto'}); resetDirtySnapshot(); }`)
		stub("/api/config/rules/r1", 500, `{"error":"rules dir is read-only"}`)
		got := run(`() => b6Result('rules-save-btn', 'rules-status', () => rulesSave())`)
		if got.State != "error" || !strings.Contains(got.Text, "rules dir is read-only") || got.Busy == "true" {
			t.Errorf("rules save result = %+v", got)
		}
	})

	// onedrive and devices are not listed in the section menu; render them directly.
	t.Run("onedrive", func(t *testing.T) {
		page.MustEval(`async () => {
			await new Promise((resolve, reject) => { const s = document.createElement('script'); s.src = '/cfg/onedrive.js'; s.onload = resolve; s.onerror = reject; document.head.appendChild(s); });
			configData.onedrive = {enabled: true};
			await renderOneDriveSection({label: 'OneDrive', desc: ''});
		}`)
		state := page.MustEval(`() => ({
			banner: document.getElementById('od-status-banner').classList.contains('cfg-state-banner'),
			draft: document.getElementById('od-test-btn').dataset.configActionDraft === 'true',
			result: document.getElementById('od-test-result').classList.contains('cfg-action-result')
		})`).Map()
		if !state["banner"].Bool() || !state["draft"].Bool() || !state["result"].Bool() {
			t.Fatalf("onedrive markup: %v", state)
		}
		stub("/api/onedrive/test", 200, `{"ok":false,"error":"Not connected"}`)
		got := run(`() => b6Result('od-test-btn', 'od-test-result', () => odTestConnection())`)
		if got.State != "error" || !strings.Contains(got.Text, "Not connected") || got.Busy == "true" {
			t.Errorf("onedrive test result = %+v", got)
		}
	})

	t.Run("devices", func(t *testing.T) {
		stub("/api/devices", 200, `[]`)
		stub("/api/credentials", 200, `[]`)
		page.MustEval(`async () => {
			await new Promise((resolve, reject) => { const s = document.createElement('script'); s.src = '/cfg/devices.js'; s.onload = resolve; s.onerror = reject; document.head.appendChild(s); });
			await renderDevicesSection({label: 'Devices', desc: ''});
			await devicesShowModal();
			document.getElementById('device-field-ip').value = '10.0.0.5';
		}`)
		stub("/api/tools/mac_lookup", 200, `{"status":"not_found"}`)
		got := run(`() => b6Result('device-find-mac-btn', 'device-mac-status', () => devicesFindMAC())`)
		if got.State != "warning" || got.Text == "" || got.Busy == "true" {
			t.Errorf("devices mac lookup result = %+v", got)
		}
	})
}
