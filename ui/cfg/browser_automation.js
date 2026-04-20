// cfg/browser_automation.js — Browser Automation config section module

let _baSection = null;

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

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.integration_enabled') + '</div>';
    html += '<div class="toggle ' + (integrationEnabled ? 'on' : '') + '" onclick="baToggleIntegration(this.classList.contains(\'on\'))"></div>';
    html += '</div>';
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
    html += baToggleRow('config.browser_automation.readonly_label', data.readonly === true, 'browser_automation.readonly');
    html += baToggleRow('config.browser_automation.headless_label', data.headless !== false, 'browser_automation.headless');
    html += baToggleRow('config.browser_automation.allow_uploads_label', data.allow_file_uploads !== false, 'browser_automation.allow_file_uploads');
    html += baToggleRow('config.browser_automation.allow_downloads_label', data.allow_file_downloads !== false, 'browser_automation.allow_file_downloads');

    html += '<details class="cfg-advanced-panel ba-advanced-panel">';
    html += '<summary class="cfg-advanced-summary">⚙️ ' + t('config.browser_automation.advanced_label') + '</summary>';
    html += '<div class="cfg-advanced-body">';
    html += '<div class="cfg-advanced-help">' + t('config.browser_automation.advanced_desc') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.mode_label') + '</div>';
    html += '<select class="field-input" data-path="browser_automation.mode">';
    html += '<option value="sidecar"' + ((data.mode || 'sidecar') === 'sidecar' ? ' selected' : '') + '>sidecar</option>';
    html += '</select>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.url_label') + '</div>';
    html += '<input class="field-input" type="text" value="' + escapeAttr(data.url || 'http://browser-automation:7331') + '" data-path="browser_automation.url">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.container_name_label') + '</div>';
    html += '<input class="field-input" type="text" value="' + escapeAttr(data.container_name || 'aurago_browser_automation') + '" data-path="browser_automation.container_name">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.image_label') + '</div>';
    html += '<input class="field-input" type="text" value="' + escapeAttr(data.image || 'aurago-browser-automation:latest') + '" data-path="browser_automation.image">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.dockerfile_dir_label') + '</div>';
    html += '<input class="field-input" type="text" value="' + escapeAttr(data.dockerfile_dir || '.') + '" data-path="browser_automation.dockerfile_dir">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.allowed_download_dir_label') + '</div>';
    html += '<input class="field-input" type="text" value="' + escapeAttr(data.allowed_download_dir || 'browser_downloads') + '" data-path="browser_automation.allowed_download_dir">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.browser_automation.screenshots_dir_label') + '</div>';
    html += '<input class="field-input" type="text" value="' + escapeAttr(data.screenshots_dir || 'browser_screenshots') + '" data-path="browser_automation.screenshots_dir">';
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.session_ttl_label') + '</div><input class="field-input" type="number" min="1" max="720" value="' + (data.session_ttl_minutes || 30) + '" data-path="browser_automation.session_ttl_minutes"></div>';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.max_sessions_label') + '</div><input class="field-input" type="number" min="1" max="20" value="' + (data.max_sessions || 3) + '" data-path="browser_automation.max_sessions"></div>';
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.viewport_width_label') + '</div><input class="field-input" type="number" min="320" max="3840" value="' + (viewport.width || 1280) + '" data-path="browser_automation.viewport.width"></div>';
    html += '<div class="field-group"><div class="field-label">' + t('config.browser_automation.viewport_height_label') + '</div><input class="field-input" type="number" min="240" max="2160" value="' + (viewport.height || 720) + '" data-path="browser_automation.viewport.height"></div>';
    html += '</div>';

    html += baToggleRow('config.browser_automation.auto_build_label', data.auto_build !== false, 'browser_automation.auto_build');
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

function baToggleRow(labelKey, enabled, path) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div><div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="toggleBool(this)"></div></div>';
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
        const toolEnabled = browserAutomation.enabled === true;
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
