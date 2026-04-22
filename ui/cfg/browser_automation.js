// cfg/browser_automation.js — Browser Automation config section module

let _baSection = null;

function baDefaultSidecarURL() {
    return (typeof isDockerRuntime === 'function' && isDockerRuntime())
        ? 'http://browser-automation:7331'
        : 'http://127.0.0.1:7331';
}

function baEnsureData() {
    if (!configData.browser_automation) configData.browser_automation = {};
    if (!configData.browser_automation.viewport) configData.browser_automation.viewport = {};
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.browser_automation) configData.tools.browser_automation = {};
    configData.tools.browser_automation.enabled = configData.browser_automation.enabled === true;
    return {
        integration: configData.browser_automation,
        tool: configData.tools.browser_automation
    };
}

async function renderBrowserAutomationSection(section) {
    if (section) _baSection = section; else section = _baSection;
    const state = baEnsureData();
    const data = state.integration;
    const integrationEnabled = data.enabled === true;
    const viewport = data.viewport || {};

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Integration enabled toggle ──
    const helpEnabled = t('help.browser_automation.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.integration_enabled') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle ' + (integrationEnabled ? 'on' : '') + '" onclick="baToggleIntegration(this.classList.contains(\'on\'))"></div>';
    html += '</div>';
    html += '<div class="toggle ' + (integrationEnabled ? 'on' : '') + '" data-path="browser_automation.enabled" style="display:none" aria-hidden="true"></div>';
    html += '<div class="toggle ' + (integrationEnabled ? 'on' : '') + '" data-path="tools.browser_automation.enabled" style="display:none" aria-hidden="true"></div>';

    if (!integrationEnabled) {
        html += '<div class="wh-notice"><span>🌐</span><div>';
        html += '<strong>' + t('config.browser_automation.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.browser_automation.disabled_desc') + '</small>';
        html += '</div></div>';
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">🧭 ' + t('config.browser_automation.sidecar_note') + '</div>';
    html += baToggleRow('config.browser_automation.readonly_label', 'help.browser_automation.readonly', data.readonly === true, 'browser_automation.readonly');
    html += baToggleRow('config.browser_automation.headless_label', 'help.browser_automation.headless', data.headless !== false, 'browser_automation.headless');
    html += baToggleRow('config.browser_automation.allow_uploads_label', 'help.browser_automation.allow_file_uploads', data.allow_file_uploads !== false, 'browser_automation.allow_file_uploads');
    html += baToggleRow('config.browser_automation.allow_downloads_label', 'help.browser_automation.allow_file_downloads', data.allow_file_downloads !== false, 'browser_automation.allow_file_downloads');

    html += '<details class="cfg-advanced-panel ba-advanced-panel">';
    html += '<summary class="cfg-advanced-summary">⚙️ ' + t('config.browser_automation.advanced_label') + '</summary>';
    html += '<div class="cfg-advanced-body">';
    html += '<div class="cfg-advanced-help">' + t('config.browser_automation.advanced_desc') + '</div>';

    html += baFieldWithHelp('config.browser_automation.mode_label', 'help.browser_automation.mode',
        '<select class="field-input" data-path="browser_automation.mode">' +
        '<option value="sidecar"' + ((data.mode || 'sidecar') === 'sidecar' ? ' selected' : '') + '>sidecar</option>' +
        '</select>');

    html += baFieldWithHelp('config.browser_automation.url_label', 'help.browser_automation.url',
        '<input class="field-input" type="text" value="' + escapeAttr(data.url || baDefaultSidecarURL()) + '" data-path="browser_automation.url">');

    html += baFieldWithHelp('config.browser_automation.container_name_label', 'help.browser_automation.container_name',
        '<input class="field-input" type="text" value="' + escapeAttr(data.container_name || 'aurago_browser_automation') + '" data-path="browser_automation.container_name">');

    html += baFieldWithHelp('config.browser_automation.image_label', 'help.browser_automation.image',
        '<input class="field-input" type="text" value="' + escapeAttr(data.image || 'aurago-browser-automation:latest') + '" data-path="browser_automation.image">');

    html += baFieldWithHelp('config.browser_automation.dockerfile_dir_label', 'help.browser_automation.dockerfile_dir',
        '<input class="field-input" type="text" value="' + escapeAttr(data.dockerfile_dir || '.') + '" data-path="browser_automation.dockerfile_dir">');

    html += baFieldWithHelp('config.browser_automation.allowed_download_dir_label', 'help.browser_automation.allowed_download_dir',
        '<input class="field-input" type="text" value="' + escapeAttr(data.allowed_download_dir || 'browser_downloads') + '" data-path="browser_automation.allowed_download_dir">');

    html += baFieldWithHelp('config.browser_automation.screenshots_dir_label', 'help.browser_automation.screenshots_dir',
        '<input class="field-input" type="text" value="' + escapeAttr(data.screenshots_dir || 'browser_screenshots') + '" data-path="browser_automation.screenshots_dir">');

    // Session TTL + Max Sessions (grid)
    const helpTTL = t('help.browser_automation.session_ttl_minutes');
    const helpMaxSessions = t('help.browser_automation.max_sessions');
    html += '<div class="field-grid two-cols">';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.session_ttl_label') + '</div>';
    if (helpTTL) html += '<div class="field-help">' + helpTTL + '</div>';
    html += '<input class="field-input" type="number" min="1" max="720" value="' + (data.session_ttl_minutes || 30) + '" data-path="browser_automation.session_ttl_minutes"></div>';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.max_sessions_label') + '</div>';
    if (helpMaxSessions) html += '<div class="field-help">' + helpMaxSessions + '</div>';
    html += '<input class="field-input" type="number" min="1" max="20" value="' + (data.max_sessions || 3) + '" data-path="browser_automation.max_sessions"></div>';
    html += '</div>';

    // Viewport (grid)
    const helpVPW = t('help.browser_automation.viewport.width');
    const helpVPH = t('help.browser_automation.viewport.height');
    html += '<div class="field-grid two-cols">';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.viewport_width_label') + '</div>';
    if (helpVPW) html += '<div class="field-help">' + helpVPW + '</div>';
    html += '<input class="field-input" type="number" min="320" max="3840" value="' + (viewport.width || 1280) + '" data-path="browser_automation.viewport.width"></div>';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.viewport_height_label') + '</div>';
    if (helpVPH) html += '<div class="field-help">' + helpVPH + '</div>';
    html += '<input class="field-input" type="number" min="240" max="2160" value="' + (viewport.height || 720) + '" data-path="browser_automation.viewport.height"></div>';
    html += '</div>';

    html += baToggleRow('config.browser_automation.auto_build_label', 'help.browser_automation.auto_build', data.auto_build !== false, 'browser_automation.auto_build');
    html += '</div>';
    html += '</details>';

    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="baTestConnection()" id="ba-test-btn">🔌 ' + t('config.browser_automation.test_button') + '</button>';
    html += '<span id="ba-test-result" class="dc-test-result"></span>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function baToggleRow(labelKey, helpKey, enabled, path) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div>';
    html += '</div>';
    return html;
}

function baFieldWithHelp(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function baToggleIntegration(isOn) {
    const state = baEnsureData();
    const next = !isOn;
    state.integration.enabled = next;
    state.tool.enabled = next;
    setDirty(true);
    renderBrowserAutomationSection(null);
}

async function baTestConnection() {
    const btn = document.getElementById('ba-test-btn');
    const result = document.getElementById('ba-test-result');
    btn.disabled = true;
    result.textContent = t('config.browser_automation.loading');
    result.className = 'dc-test-result';
    try {
        const patch = buildConfigPatchFromForm();
        const browserAutomation = patch.browser_automation || {};
        const toolEnabled = patch.tools?.browser_automation?.enabled === true || browserAutomation.enabled === true;
        const resp = await fetch('/api/browser-automation/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                browser_automation: browserAutomation,
                tool_enabled: toolEnabled
            })
        });
        const body = await resp.json();
        if (resp.ok && body.status === 'success') {
            result.className = 'dc-test-result is-success';
            result.textContent = t('config.browser_automation.status_ok');
        } else {
            result.className = 'dc-test-result is-danger';
            result.textContent = t('config.browser_automation.status_error') + ' ' + (body.message || ('HTTP ' + resp.status));
        }
    } catch (e) {
        result.className = 'dc-test-result is-danger';
        result.textContent = t('config.browser_automation.status_error') + ' ' + e.message;
    } finally {
        btn.disabled = false;
    }
}
