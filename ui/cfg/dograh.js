// cfg/dograh.js - Dograh integration section module

let _dograhSection = null;

const dograhEndpoints = {
    status: '/api/dograh/status',
    test: '/api/dograh/test',
    start: '/api/dograh/start',
    stop: '/api/dograh/stop',
    recreate: '/api/dograh/recreate',
    provisionWebhook: '/api/dograh/provision-webhook',
    registerMCP: '/api/dograh/register-aurago-mcp-tool'
};

function dograhText(key, fallback) {
    if (typeof t === 'function') {
        const value = t(key);
        if (typeof value === 'string' && value.trim() !== '' && value !== key) return value;
    }
    if (typeof fallback === 'string' && fallback.trim() !== '' && fallback !== key) return fallback;
    return '';
}

function dograhEnsureData() {
    if (!configData.dograh) configData.dograh = {};
    const data = configData.dograh;
    if (typeof data.auto_start !== 'boolean') data.auto_start = true;
    if (typeof data.readonly !== 'boolean') data.readonly = true;
    if (typeof data.mcp_client_enabled !== 'boolean') data.mcp_client_enabled = true;
    if (typeof data.mcp_server_tool_enabled !== 'boolean') data.mcp_server_tool_enabled = true;
    if (!data.mode) data.mode = 'managed';
    if (!data.api_url) data.api_url = 'http://127.0.0.1:8000';
    if (!data.ui_url) data.ui_url = 'http://127.0.0.1:3010';
    if (!data.host) data.host = '127.0.0.1';
    if (!data.api_host_port) data.api_host_port = 8000;
    if (!data.ui_host_port) data.ui_host_port = 3010;
    if (!data.api_image || data.api_image === 'dograhai/dograh-api:latest') data.api_image = 'ghcr.io/dograh-hq/dograh-api:latest';
    if (!data.ui_image || data.ui_image === 'dograhai/dograh-ui:latest') data.ui_image = 'ghcr.io/dograh-hq/dograh-ui:latest';
    if (!data.postgres_image) data.postgres_image = 'pgvector/pgvector:pg17';
    if (!data.redis_image) data.redis_image = 'redis:7';
    if (!data.minio_image) data.minio_image = 'minio/minio:latest';
    if (!data.callback_webhook_slug) data.callback_webhook_slug = 'dograh-callback';
    if (!data.aurago_mcp_tool_name) data.aurago_mcp_tool_name = 'AuraGo';
    return data;
}

async function renderDograhSection(section) {
    if (section) _dograhSection = section; else section = _dograhSection;
    const data = dograhEnsureData();
    const enabled = data.enabled === true;
    const managed = data.mode !== 'external';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + dograhText('config.section.dograh.label', section.label) + '</div>';
    html += '<div class="section-desc">' + dograhText('config.section.dograh.desc', section.desc) + '</div>';
    html += dograhToggleRow('config.dograh.enabled_label', 'help.dograh.enabled', enabled, 'dograh.enabled', "dograhToggleEnabled(this)");

    if (!enabled) {
        html += '<div class="wh-notice"><span>▧</span><div><strong>' + dograhText('config.dograh.disabled_notice') + '</strong><br>';
        html += '<small>' + dograhText('config.dograh.disabled_desc') + '</small></div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">▧ ' + dograhText('config.dograh.sidecar_note') + '</div>';
    html += cfgStatusBanner.html('dograh-status-box', 'checking', { detail: dograhText('config.dograh.status_prefix') + ' ...' });
    html += '<div class="cfg-actions-row">';
    html += dograhActionButton('dograh-test-btn', 'config.dograh.test_button', 'dograhAction(\'test\')', 'adg-test-btn', '🔌 ');
    html += dograhActionButton('dograh-start-btn', 'config.dograh.start_button', 'dograhAction(\'start\')', 'btn-secondary', '▶ ');
    html += dograhActionButton('dograh-stop-btn', 'config.dograh.stop_button', 'dograhAction(\'stop\')', 'btn-secondary', '⏹ ');
    html += dograhActionButton('dograh-recreate-btn', 'config.dograh.recreate_button', 'dograhAction(\'recreate\')', 'btn-secondary', '🔄 ');
    html += cfgActionResult.html('dograh-test-result');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += dograhField('config.dograh.mode_label', 'help.dograh.mode',
        '<select class="field-select" data-path="dograh.mode" onchange="setNestedValue(configData,\'dograh.mode\',this.value);setDirty(true);renderDograhSection(null)">' +
        '<option value="managed"' + (managed ? ' selected' : '') + '>' + dograhText('config.dograh.mode_managed') + '</option>' +
        '<option value="external"' + (!managed ? ' selected' : '') + '>' + dograhText('config.dograh.mode_external') + '</option>' +
        '</select>');
    html += dograhToggleRow('config.dograh.readonly_label', 'help.dograh.readonly', data.readonly !== false, 'dograh.readonly');
    if (managed) html += dograhToggleRow('config.dograh.auto_start_label', 'help.dograh.auto_start', data.auto_start !== false, 'dograh.auto_start');
    html += dograhToggleRow('config.dograh.telemetry_label', 'help.dograh.telemetry_enabled', data.telemetry_enabled === true, 'dograh.telemetry_enabled');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += dograhInput('config.dograh.api_url_label', 'help.dograh.api_url', 'dograh.api_url', data.api_url || 'http://127.0.0.1:8000');
    html += dograhInput('config.dograh.ui_url_label', 'help.dograh.ui_url', 'dograh.ui_url', data.ui_url || 'http://127.0.0.1:3010');
    html += '</div>';

    if (managed) {
        html += '<div class="field-grid two-cols">';
        html += dograhInput('config.dograh.host_label', 'help.dograh.host', 'dograh.host', data.host || '127.0.0.1');
        html += dograhNumber('config.dograh.api_host_port_label', 'help.dograh.api_host_port', 'dograh.api_host_port', data.api_host_port || 8000);
        html += dograhNumber('config.dograh.ui_host_port_label', 'help.dograh.ui_host_port', 'dograh.ui_host_port', data.ui_host_port || 3010);
        html += '</div>';
    }

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + escapeHtml(dograhText('config.dograh.vault_section_title')) + '</div>';
    html += dograhSecretField('config.dograh.api_key_label', 'help.dograh.api_key', 'dograh-api-key-input', 'dograh.api_key', 'dg_••••••••');
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + escapeHtml(dograhText('config.dograh.mcp_section_title')) + '</div>';
    html += '<div class="field-grid two-cols">';
    html += dograhToggleRow('config.dograh.mcp_client_label', 'help.dograh.mcp_client_enabled', data.mcp_client_enabled !== false, 'dograh.mcp_client_enabled');
    html += dograhToggleRow('config.dograh.mcp_server_tool_label', 'help.dograh.mcp_server_tool_enabled', data.mcp_server_tool_enabled !== false, 'dograh.mcp_server_tool_enabled');
    html += dograhInput('config.dograh.credential_uuid_label', 'help.dograh.credential_uuid', 'dograh.aurago_mcp_credential_uuid', data.aurago_mcp_credential_uuid || '');
    html += dograhInput('config.dograh.allowed_tools_label', 'help.dograh.allowed_tools', 'dograh.aurago_mcp_allowed_tools', Array.isArray(data.aurago_mcp_allowed_tools) ? data.aurago_mcp_allowed_tools.join(', ') : '');
    html += '</div><div class="cfg-actions-row">';
    html += dograhButton('dograh-mcp-btn', 'config.dograh.mcp_register_button', 'dograhAction(\'registerMCP\')');
    html += cfgActionResult.html('dograh-mcp-result');
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + escapeHtml(dograhText('config.dograh.webhook_section_title')) + '</div>';
    html += dograhInput('config.dograh.webhook_slug_label', 'help.dograh.webhook_slug', 'dograh.callback_webhook_slug', data.callback_webhook_slug || 'dograh-callback');
    html += '<div class="cfg-actions-row">' + dograhButton('dograh-webhook-btn', 'config.dograh.webhook_button', 'dograhAction(\'provisionWebhook\')') + cfgActionResult.html('dograh-webhook-result') + '</div>';
    html += '<div id="dograh-webhook-token" class="cfg-note-banner cfg-note-banner-info" role="status" hidden></div>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    dograhRefreshStatus();
}

function dograhField(labelKey, helpKey, controlHTML) {
    const helpText = dograhText(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + escapeHtml(dograhText(labelKey)) + '</div>';
    if (helpText) html += '<div class="field-help">' + escapeHtml(helpText) + '</div>';
    html += controlHTML;
    return html + '</div>';
}

function dograhInput(labelKey, helpKey, path, value) {
    const arrayType = path === 'dograh.aurago_mcp_allowed_tools' ? ' data-type="array"' : '';
    return dograhField(labelKey, helpKey, '<input class="field-input" type="text" value="' + escapeAttr(value || '') + '" data-path="' + escapeAttr(path) + '"' + arrayType + '>');
}

function dograhNumber(labelKey, helpKey, path, value) {
    return dograhField(labelKey, helpKey, '<input class="field-input" type="number" min="1" max="65535" value="' + escapeAttr(value || '') + '" data-path="' + escapeAttr(path) + '">');
}

function dograhSecretField(labelKey, helpKey, id, path, placeholder) {
    const value = cfgSecretValue(path.split('.').reduce((o, k) => o && o[k], configData));
    return dograhField(labelKey, helpKey,
        '<div class="password-wrap"><input class="field-input" type="password" id="' + id + '" value="' + escapeAttr(value) + '" placeholder="' + escapeAttr(placeholder) + '" data-path="' + escapeAttr(path) + '" autocomplete="off"><button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>');
}

function dograhToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const handler = onclick || 'toggleBool(this)';
    return dograhField(labelKey, helpKey,
        '<div class="toggle-wrap">' +
        '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + escapeAttr(path) + '" onclick="' + handler + '"></div>' +
        '<span class="toggle-label">' + escapeHtml(enabled ? dograhText('config.toggle.active', 'Active') : dograhText('config.toggle.inactive', 'Inactive')) + '</span>' +
        '</div>');
}

function dograhButton(id, labelKey, onclick) {
    return dograhActionButton(id, labelKey, onclick, 'btn-secondary', '');
}

function dograhActionButton(id, labelKey, onclick, btnClass, prefix) {
    // Start, stop, recreate, test and registration act on the running service.
    return '<button type="button" id="' + id + '" class="btn-save ' + btnClass + '" data-config-action-draft="false" onclick="' + onclick + '">' + prefix + escapeHtml(dograhText(labelKey)) + '</button>';
}

function dograhSetBanner(state, detail, opts) {
    cfgStatusBanner('dograh-status-box', state, Object.assign({ detail: detail, retry: state === 'error' || state === 'unknown' ? dograhRefreshStatus : null }, opts));
}

// The webhook token is shown once, in its own box (never inside the status banner).
function dograhShowWebhookToken(token) {
    const box = document.getElementById('dograh-webhook-token');
    if (!box) return;
    const hint = document.createElement('span');
    hint.textContent = dograhText('config.dograh.webhook_token_hint') + ' ';
    const code = document.createElement('code');
    code.textContent = token;
    box.replaceChildren(hint, code);
    box.hidden = false;
}

function dograhActionLoadingText(action) {
    const map = {
        test: 'config.dograh.testing',
        start: 'config.dograh.starting',
        stop: 'config.dograh.stopping',
        recreate: 'config.dograh.recreating'
    };
    return dograhText(map[action] || 'config.dograh.status_prefix') || action;
}

function dograhToggleEnabled(el) {
    const enabled = !el.classList.contains('on');
    // Flip the clicked switch first: state sync still reads it, so a stale one reverts the draft.
    el.classList.toggle('on', enabled);
    el.setAttribute('aria-checked', String(enabled));
    setNestedValue(configData, 'dograh.enabled', enabled);
    setDirty(true);
    renderDograhSection(null);
}

async function dograhRefreshStatus() {
    if (!document.getElementById('dograh-status-box')) return;
    dograhSetBanner('checking', dograhText('config.dograh.status_prefix') + ' ...');
    try {
        const res = await fetch(dograhEndpoints.status, { credentials: 'same-origin' });
        const body = await res.json();
        if (!res.ok) throw new Error(body.error || body.message || res.statusText);
        dograhRenderStatus(body);
    } catch (err) {
        dograhSetBanner('error', dograhText('config.dograh.status_error') + ' ' + err.message);
    }
}

function dograhStatusState(body) {
    const status = String(body && body.status ? body.status : '').toLowerCase();
    if (body && body.admin_setup_required) return 'degraded';
    if (body && body.setup_required) return 'degraded';
    if (status === 'running' || status === 'ok' || status === 'connected') return 'connected';
    if (status === 'error' || status === 'failed') return 'error';
    if (status === 'disabled') return 'off';
    if (status === 'stopped' || status === 'stopping' || status === 'starting') return 'degraded';
    return 'unknown';
}

function dograhRenderStatus(body) {
    const parts = [dograhText('config.dograh.status_prefix') + ' ' + ((body && body.status) || 'unknown')];
    if (body && body.admin_setup_required) parts.push(dograhText('config.dograh.admin_setup_required'));
    if (body && body.setup_required && body.message) parts.push(body.message);
    const links = [];
    if (body && body.api_url) links.push({ href: body.api_url, label: 'API' });
    if (body && body.ui_url) links.push({ href: body.ui_url, label: 'UI' });
    dograhSetBanner(dograhStatusState(body), parts.join(' · '), { links: links });
}

const dograhActionButtons = { registerMCP: 'dograh-mcp-btn', provisionWebhook: 'dograh-webhook-btn' };
const dograhActionResults = { registerMCP: 'dograh-mcp-result', provisionWebhook: 'dograh-webhook-result' };

async function dograhAction(action) {
    if (action === 'recreate' && !(await showConfirm(t('config.dograh.confirm_recreate_title'), t('config.dograh.confirm_recreate_message')))) return;
    const btn = document.getElementById(dograhActionButtons[action] || 'dograh-' + action + '-btn');
    const endpoint = dograhEndpoints[action];
    if (!btn || !endpoint) return;
    const loadingText = dograhActionLoadingText(action);
    const outcome = await cfgActionResult(btn, () => {
        if (action !== 'test') dograhSetBanner('checking', dograhText('config.dograh.status_prefix') + ' ' + loadingText);
        return fetch(endpoint, {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ dograh: configData.dograh || {} })
        });
    }, {
        target: dograhActionResults[action] || 'dograh-test-result',
        pending: loadingText,
        success: o => o.reason || t('config.feedback.done'),
        failure: dograhText('config.dograh.status_error'),
        lock: '.cfg-actions-row button'
    });
    if (outcome.skipped) return;
    const body = outcome.body;
    if (outcome.httpStatus >= 400 || !body) {
        if (action !== 'test') dograhSetBanner('error', dograhText('config.dograh.status_error') + ' ' + (outcome.reason || ''));
        return;
    }
    if (body.token) {
        dograhShowWebhookToken(body.token);
        dograhRefreshStatus();
        return;
    }
    dograhRenderStatus(body);
    setTimeout(dograhRefreshStatus, 900);
}
