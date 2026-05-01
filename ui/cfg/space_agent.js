// cfg/space_agent.js — Space Agent config section module

let _spaceAgentSection = null;

function spaceAgentEnsureData() {
    if (!configData.space_agent) configData.space_agent = {};
    const data = configData.space_agent;
    if (!data.repo_url) data.repo_url = 'https://github.com/agent0ai/space-agent';
    if (!data.git_ref) data.git_ref = 'main';
    if (!data.container_name) data.container_name = 'aurago_space_agent';
    if (!data.image) data.image = 'aurago-space-agent:main';
    if (!data.host) data.host = '0.0.0.0';
    const legacyDefaultUrl = !data.public_url || data.public_url === 'http://127.0.0.1:3000';
    if (!data.port || (data.port === 3000 && legacyDefaultUrl)) data.port = 3100;
    if (!data.customware_path) data.customware_path = 'data/sidecars/space-agent/customware';
    if (!data.data_path) data.data_path = 'data/sidecars/space-agent/data';
    if (!data.admin_user) data.admin_user = 'admin';
    if (!data.public_url || data.public_url === 'http://127.0.0.1:3000') data.public_url = '';
    if (typeof data.auto_start !== 'boolean') data.auto_start = true;
    return data;
}

async function renderSpaceAgentSection(section) {
    if (section) _spaceAgentSection = section; else section = _spaceAgentSection;
    const data = spaceAgentEnsureData();
    const enabled = data.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += spaceAgentToggleRow('config.space_agent.enabled_label', 'help.space_agent.enabled', enabled, 'space_agent.enabled', "spaceAgentToggleEnabled(this.classList.contains('on'))");

    if (!enabled) {
        html += '<div class="wh-notice"><span>🛰️</span><div>';
        html += '<strong>' + t('config.space_agent.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.space_agent.disabled_desc') + '</small>';
        html += '</div></div>';
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">🛰️ ' + t('config.space_agent.sidecar_note') + '</div>';
    html += spaceAgentToggleRow('config.space_agent.auto_start_label', 'help.space_agent.auto_start', data.auto_start !== false, 'space_agent.auto_start');

    html += '<div class="field-grid two-cols">';
    html += spaceAgentField('config.space_agent.public_url_label', 'help.space_agent.public_url',
        '<input class="field-input" type="url" placeholder="http://aurago-server.local:3100" value="' + escapeAttr(data.public_url || '') + '" data-path="space_agent.public_url">');
    html += spaceAgentField('config.space_agent.port_label', 'help.space_agent.port',
        '<input class="field-input" type="number" min="1" max="65535" value="' + (data.port || 3100) + '" data-path="space_agent.port">');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += spaceAgentField('config.space_agent.host_label', 'help.space_agent.host',
        '<input class="field-input" type="text" value="' + escapeAttr(data.host || '0.0.0.0') + '" data-path="space_agent.host">');
    html += spaceAgentField('config.space_agent.admin_user_label', 'help.space_agent.admin_user',
        '<input class="field-input" type="text" value="' + escapeAttr(data.admin_user || 'admin') + '" data-path="space_agent.admin_user">');
    html += '</div>';

    html += spaceAgentField('config.space_agent.admin_password_label', 'help.space_agent.admin_password',
        '<div class="password-wrap"><input class="field-input" type="password" value="' + escapeAttr(data.admin_password || '') + '" data-path="space_agent.admin_password" autocomplete="new-password"><button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>');

    html += '<details class="cfg-advanced-panel space-agent-advanced-panel">';
    html += '<summary class="cfg-advanced-summary">⚙️ ' + t('config.space_agent.advanced_label') + '</summary>';
    html += '<div class="cfg-advanced-body">';
    html += '<div class="cfg-advanced-help">' + t('config.space_agent.advanced_desc') + '</div>';
    html += spaceAgentField('config.space_agent.repo_url_label', 'help.space_agent.repo_url',
        '<input class="field-input" type="url" value="' + escapeAttr(data.repo_url || 'https://github.com/agent0ai/space-agent') + '" data-path="space_agent.repo_url">');
    html += spaceAgentField('config.space_agent.git_ref_label', 'help.space_agent.git_ref',
        '<input class="field-input" type="text" value="' + escapeAttr(data.git_ref || 'main') + '" data-path="space_agent.git_ref">');
    html += spaceAgentField('config.space_agent.container_name_label', 'help.space_agent.container_name',
        '<input class="field-input" type="text" value="' + escapeAttr(data.container_name || 'aurago_space_agent') + '" data-path="space_agent.container_name">');
    html += spaceAgentField('config.space_agent.image_label', 'help.space_agent.image',
        '<input class="field-input" type="text" value="' + escapeAttr(data.image || 'aurago-space-agent:main') + '" data-path="space_agent.image">');
    html += spaceAgentField('config.space_agent.data_path_label', 'help.space_agent.data_path',
        '<input class="field-input" type="text" value="' + escapeAttr(data.data_path || 'data/sidecars/space-agent/data') + '" data-path="space_agent.data_path">');
    html += spaceAgentField('config.space_agent.customware_path_label', 'help.space_agent.customware_path',
        '<input class="field-input" type="text" value="' + escapeAttr(data.customware_path || 'data/sidecars/space-agent/customware') + '" data-path="space_agent.customware_path">');
    html += '</div></details>';

    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="spaceAgentRecreate()" id="space-agent-recreate-btn">🔄 ' + t('config.space_agent.recreate_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="spaceAgentStatus()" id="space-agent-status-btn">🔌 ' + t('config.space_agent.status_button') + '</button>';
    html += '<span id="space-agent-result" class="dc-test-result"></span>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function spaceAgentToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || 'toggleBool(this)') + '"></div>';
    html += '</div>';
    return html;
}

function spaceAgentField(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function spaceAgentToggleEnabled(isOn) {
    const data = spaceAgentEnsureData();
    data.enabled = !isOn;
    setDirty(true);
    renderSpaceAgentSection(null);
}

async function spaceAgentStatus() {
    const result = document.getElementById('space-agent-result');
    const btn = document.getElementById('space-agent-status-btn');
    btn.disabled = true;
    result.textContent = t('config.space_agent.loading');
    result.className = 'dc-test-result';
    try {
        const resp = await fetch('/api/space-agent/status');
        const body = await resp.json();
        result.className = body.status === 'running' ? 'dc-test-result is-success' : 'dc-test-result';
        result.textContent = t('config.space_agent.status_prefix') + ' ' + (body.status || 'unknown');
    } catch (e) {
        result.className = 'dc-test-result is-danger';
        result.textContent = t('config.space_agent.status_error') + ' ' + e.message;
    } finally {
        btn.disabled = false;
    }
}

async function spaceAgentRecreate() {
    const result = document.getElementById('space-agent-result');
    const btn = document.getElementById('space-agent-recreate-btn');
    btn.disabled = true;
    result.textContent = t('config.space_agent.recreate_starting');
    result.className = 'dc-test-result';
    try {
        const resp = await fetch('/api/space-agent/recreate', { method: 'POST' });
        const body = await resp.json();
        result.className = resp.ok ? 'dc-test-result is-success' : 'dc-test-result is-danger';
        result.textContent = body.message || (body.status || '');
    } catch (e) {
        result.className = 'dc-test-result is-danger';
        result.textContent = t('config.space_agent.status_error') + ' ' + e.message;
    } finally {
        btn.disabled = false;
    }
}
