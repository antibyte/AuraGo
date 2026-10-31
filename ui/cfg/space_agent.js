// cfg/space_agent.js — Space Agent config section module

let _spaceAgentSection = null;
const SPACE_AGENT_DEFAULT_GIT_REF = '10f4ffdaf50a8136cf8450d17c11286178fd58e6';

function spaceAgentEnsureData() {
    if (!configData.space_agent) configData.space_agent = {};
    const data = configData.space_agent;
    if (!data.repo_url) data.repo_url = 'https://github.com/agent0ai/space-agent';
    if (!data.git_ref) data.git_ref = SPACE_AGENT_DEFAULT_GIT_REF;
    if (!data.container_name) data.container_name = 'aurago_space_agent';
    if (!data.image) data.image = 'aurago-space-agent:main';
    if (!data.host) data.host = '0.0.0.0';
    const legacyDefaultUrl = !data.public_url || data.public_url === 'http://127.0.0.1:3000';
    if (!data.port || (data.port === 3000 && legacyDefaultUrl)) data.port = 3100;
    if (typeof data.https_enabled !== 'boolean') data.https_enabled = true;
    if (!data.https_port) data.https_port = 3101;
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
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div class="cfg-group-title">' + t('config.refresh.operation') + '</div>';

    html += spaceAgentToggleRow('config.space_agent.enabled_label', 'help.space_agent.enabled', enabled, 'space_agent.enabled', "spaceAgentToggleEnabled(this)");

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

    html += '<div class="cfg-group-title">' + t('config.refresh.connection') + '</div>';
    html += '<div class="field-grid">';
    html += spaceAgentField('config.space_agent.public_url_label', 'help.space_agent.public_url',
        '<input class="field-input" type="url" placeholder="https://aurago-server.local:3101" value="' + escapeAttr(data.public_url || '') + '" data-path="space_agent.public_url">');
    html += spaceAgentField('config.space_agent.port_label', 'help.space_agent.port',
        '<input class="field-input" type="number" min="1" max="65535" value="' + (data.port || 3100) + '" data-path="space_agent.port">');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += spaceAgentField('config.space_agent.https_port_label', 'help.space_agent.https_port',
        '<input class="field-input" type="number" min="1" max="65535" value="' + (data.https_port || 3101) + '" data-path="space_agent.https_port">');
    html += spaceAgentToggleRow('config.space_agent.https_enabled_label', 'help.space_agent.https_enabled', data.https_enabled !== false, 'space_agent.https_enabled');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += spaceAgentField('config.space_agent.host_label', 'help.space_agent.host',
        '<input class="field-input" type="text" value="' + escapeAttr(data.host || '0.0.0.0') + '" data-path="space_agent.host">');
    html += '</div><div class="cfg-group-title">' + t('config.refresh.credentials') + '</div><div class="field-grid">';
    html += spaceAgentField('config.space_agent.admin_user_label', 'help.space_agent.admin_user',
        '<input class="field-input" type="text" value="' + escapeAttr(data.admin_user || 'admin') + '" data-path="space_agent.admin_user">');
    html += '</div>';

    html += spaceAgentField('config.space_agent.admin_password_label', 'help.space_agent.admin_password',
        '<div class="password-wrap"><input class="field-input" type="password" value="' + escapeAttr(data.admin_password || '') + '" data-path="space_agent.admin_password" autocomplete="new-password"><button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>');

    html += '<div class="cfg-group-title">' + t('config.refresh.runtime') + '</div>';
    html += '<details class="cfg-advanced-panel space-agent-advanced-panel">';
    html += '<summary class="cfg-advanced-summary">⚙️ ' + t('config.space_agent.advanced_label') + '</summary>';
    html += '<div class="cfg-advanced-body">';
    html += '<div class="cfg-advanced-help">' + t('config.space_agent.advanced_desc') + '</div>';
    html += spaceAgentField('config.space_agent.repo_url_label', 'help.space_agent.repo_url',
        '<input class="field-input" type="url" value="' + escapeAttr(data.repo_url || 'https://github.com/agent0ai/space-agent') + '" data-path="space_agent.repo_url">');
    html += spaceAgentField('config.space_agent.git_ref_label', 'help.space_agent.git_ref',
        '<input class="field-input" type="text" value="' + escapeAttr(data.git_ref || SPACE_AGENT_DEFAULT_GIT_REF) + '" data-path="space_agent.git_ref">');
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
    html += '<button class="btn-save dc-test-btn" onclick="spaceAgentRecreate()" id="space-agent-recreate-btn" data-config-action-draft="false">🔄 ' + t('config.space_agent.recreate_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="spaceAgentStatus()" id="space-agent-status-btn" data-config-action-draft="false">🔌 ' + t('config.space_agent.status_button') + '</button>';
    html += cfgActionResult.html('space-agent-result');
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

function spaceAgentToggleEnabled(el) {
    const enabled = !el.classList.contains('on');
    // Flip the clicked switch first: state sync still reads it, so a stale one reverts the draft.
    el.classList.toggle('on', enabled);
    el.setAttribute('aria-checked', String(enabled));
    const data = spaceAgentEnsureData();
    data.enabled = enabled;
    setDirty(true);
    renderSpaceAgentSection(null);
}

// Status and recreate act on the running container, so unsaved changes block them
// with the reason in the result line (draft="false" on the buttons).
function spaceAgentRuntimeLocked() {
    if (!window.AuraConfigState || !AuraConfigState.isDirty()) return false;
    cfgActionResult.show('space-agent-result', 'warning', t('config.feedback.runtime_only'));
    return true;
}

async function spaceAgentStatus() {
    if (spaceAgentRuntimeLocked()) return { skipped: 'locked' };
    return cfgActionResult(document.getElementById('space-agent-status-btn'), () => fetch('/api/space-agent/status'), {
        target: 'space-agent-result',
        pending: t('config.space_agent.loading'),
        failure: t('config.space_agent.status_error'),
        interpret: outcome => {
            if (!outcome.ok || !outcome.body) return null;
            const status = outcome.body.status || 'unknown';
            // Only a running container is a success; every other state is shown as a warning.
            return { state: status === 'running' ? 'success' : 'warning', reason: t('config.space_agent.status_prefix') + ' ' + status };
        },
        success: outcome => outcome.reason
    });
}

async function spaceAgentRecreate() {
    if (spaceAgentRuntimeLocked()) return { skipped: 'locked' };
    if (!(await showConfirm(t('config.space_agent.confirm_recreate_title'), t('config.space_agent.confirm_recreate_message')))) return;
    return cfgActionResult(document.getElementById('space-agent-recreate-btn'), () => fetch('/api/space-agent/recreate', { method: 'POST' }), {
        target: 'space-agent-result',
        pending: t('config.space_agent.recreate_starting'),
        failure: t('config.space_agent.status_error'),
        success: outcome => outcome.reason || t('config.feedback.done'),
        lock: '#space-agent-status-btn'
    });
}
