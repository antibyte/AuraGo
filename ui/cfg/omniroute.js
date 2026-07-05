// cfg/omniroute.js - OmniRoute gateway integration section module

let _omniRouteSection = null;

function omniRouteText(key, fallback) {
    if (typeof t === 'function') {
        const value = t(key);
        if (typeof value === 'string' && value.trim() !== '' && value !== key) return value;
    }
    if (typeof fallback === 'string' && fallback.trim() !== '' && fallback !== key) return fallback;
    return '';
}

function omniRouteEnsureData() {
    if (!configData.omniroute) configData.omniroute = {};
    const data = configData.omniroute;
    if (typeof data.auto_start !== 'boolean') data.auto_start = true;
    if (!data.mode) data.mode = 'managed';
    if (!data.url) data.url = 'http://127.0.0.1:20128';
    if (!data.external_base_url) data.external_base_url = 'http://127.0.0.1:20128/v1';
    if (!data.container_name) data.container_name = 'aurago_omniroute';
    if (!data.image) data.image = 'diegosouzapw/omniroute:3.8.39';
    if (!data.host) data.host = '127.0.0.1';
    if (!data.port) data.port = 20128;
    if (!data.host_port) data.host_port = 20128;
    if (!data.network_name) data.network_name = 'aurago_omniroute';
    if (!data.data_volume) data.data_volume = 'aurago_omniroute_data';
    if (!data.health_path) data.health_path = '/api/monitoring/health';
    if (!data.memory_mb) data.memory_mb = 512;
    return data;
}

async function renderOmniRouteSection(section) {
    if (section) _omniRouteSection = section; else section = _omniRouteSection;
    const data = omniRouteEnsureData();
    const enabled = data.enabled === true;
    const managed = data.mode !== 'external';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + omniRouteText('config.section.omniroute.label', section.label) + '</div>';
    html += '<div class="section-desc">' + omniRouteText('config.section.omniroute.desc', section.desc) + '</div>';

    html += omniRouteToggleRow('config.omniroute.enabled_label', 'help.omniroute.enabled', enabled, 'omniroute.enabled', "omniRouteToggleEnabled(this.classList.contains('on'))");

    if (!enabled) {
        html += '<div class="wh-notice"><span>◎</span><div>';
        html += '<strong>' + omniRouteText('config.omniroute.disabled_notice') + '</strong><br>';
        html += '<small>' + omniRouteText('config.omniroute.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">◎ ' + omniRouteText('config.omniroute.sidecar_note') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += omniRouteField('config.omniroute.mode_label', 'help.omniroute.mode',
        "<select class=\"field-select\" data-path=\"omniroute.mode\" onchange=\"setNestedValue(configData,'omniroute.mode',this.value);setDirty(true);renderOmniRouteSection(null)\">" +
        '<option value="managed"' + (managed ? ' selected' : '') + '>' + omniRouteText('config.omniroute.mode_managed') + '</option>' +
        '<option value="external"' + (!managed ? ' selected' : '') + '>' + omniRouteText('config.omniroute.mode_external') + '</option>' +
        '</select>');
    if (managed) {
        html += omniRouteToggleRow('config.omniroute.auto_start_label', 'help.omniroute.auto_start', data.auto_start !== false, 'omniroute.auto_start');
    }
    html += '</div>';

    if (managed) {
        const port = data.port || 20128;
        html += '<div class="field-grid two-cols">';
        html += omniRouteSelectField('config.omniroute.url_label', 'help.omniroute.url', 'omniroute.url', data.url || 'http://127.0.0.1:20128', [
            { value: 'http://omniroute:' + port },
            { value: 'http://127.0.0.1:' + port }
        ]);
        html += omniRouteSelectField('config.omniroute.host_label', 'help.omniroute.host', 'omniroute.host', data.host || '127.0.0.1', [
            { value: '127.0.0.1' },
            { value: '0.0.0.0' },
            { value: 'localhost' }
        ]);
        html += '</div>';
        html += '<div class="field-grid two-cols">';
        html += omniRouteField('config.omniroute.port_label', 'help.omniroute.port',
            '<input class="field-input" type="number" min="1" max="65535" value="' + (data.port || 20128) + '" data-path="omniroute.port">');
        html += omniRouteField('config.omniroute.host_port_label', 'help.omniroute.host_port',
            '<input class="field-input" type="number" min="1" max="65535" value="' + (data.host_port || 20128) + '" data-path="omniroute.host_port">');
        html += '</div>';
        html += '<details class="cfg-advanced-panel omniroute-advanced-panel">';
        html += '<summary class="cfg-advanced-summary">' + omniRouteText('config.omniroute.advanced_label') + '</summary>';
        html += '<div class="cfg-advanced-body">';
        html += omniRouteSelectField('config.omniroute.image_label', 'help.omniroute.image', 'omniroute.image', data.image || 'diegosouzapw/omniroute:3.8.39', [
            { value: 'diegosouzapw/omniroute:3.8.39' }
        ]);
        html += omniRouteField('config.omniroute.container_name_label', 'help.omniroute.container_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.container_name || 'aurago_omniroute') + '" data-path="omniroute.container_name">');
        html += omniRouteField('config.omniroute.network_name_label', 'help.omniroute.network_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.network_name || 'aurago_omniroute') + '" data-path="omniroute.network_name">');
        html += omniRouteField('config.omniroute.data_volume_label', 'help.omniroute.data_volume',
            '<input class="field-input" type="text" value="' + escapeAttr(data.data_volume || 'aurago_omniroute_data') + '" data-path="omniroute.data_volume">');
        html += omniRouteSelectField('config.omniroute.health_path_label', 'help.omniroute.health_path', 'omniroute.health_path', data.health_path || '/api/monitoring/health', [
            { value: '/api/monitoring/health' },
            { value: '/v1/models' }
        ]);
        html += omniRouteField('config.omniroute.memory_mb_label', 'help.omniroute.memory_mb',
            '<input class="field-input" type="number" min="128" max="8192" step="64" value="' + (data.memory_mb || 512) + '" data-path="omniroute.memory_mb">');
        html += '</div></details>';
    } else {
        html += omniRouteSelectField('config.omniroute.external_base_url_label', 'help.omniroute.external_base_url', 'omniroute.external_base_url', data.external_base_url || 'http://127.0.0.1:20128/v1', [
            { value: 'http://127.0.0.1:20128/v1' }
        ]);
    }

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + omniRouteText('config.omniroute.secrets_title') + '</div>';
    html += '<div class="field-group-desc">' + omniRouteText('config.omniroute.secrets_desc') + '</div>';
    html += omniRouteSecretField('config.omniroute.api_key_label', 'help.omniroute.api_key', 'omniroute-api-key-input', 'omniroute.api_key', t('config.providers.key_placeholder_existing'));
    if (managed) {
        html += omniRouteSecretField('config.omniroute.initial_password_label', 'help.omniroute.initial_password', 'omniroute-initial-password-input', 'omniroute.initial_password', t('config.providers.key_placeholder_existing'));
        html += '<details class="cfg-advanced-panel omniroute-secrets-panel">';
        html += '<summary class="cfg-advanced-summary">' + omniRouteText('config.omniroute.advanced_label') + '</summary>';
        html += '<div class="cfg-advanced-body">';
        html += omniRouteSecretField('config.omniroute.jwt_secret_label', 'help.omniroute.jwt_secret', 'omniroute-jwt-secret-input', 'omniroute.jwt_secret', t('config.providers.key_placeholder_existing'));
        html += omniRouteSecretField('config.omniroute.api_key_secret_label', 'help.omniroute.api_key_secret', 'omniroute-api-key-secret-input', 'omniroute.api_key_secret', t('config.providers.key_placeholder_existing'));
        html += omniRouteSecretField('config.omniroute.ws_bridge_secret_label', 'help.omniroute.ws_bridge_secret', 'omniroute-ws-bridge-secret-input', 'omniroute.ws_bridge_secret', t('config.providers.key_placeholder_existing'));
        html += '</div></details>';
    }
    html += '</div>';

    html += '<div class="cfg-actions-row">';
    html += '<button class="btn-save adg-test-btn" onclick="omniRouteTestConnection()" id="omniroute-test-btn">🔌 ' + omniRouteText('config.omniroute.test_button') + '</button>';
    if (managed) {
        html += '<button class="btn-save btn-secondary" onclick="omniRouteStartSidecar()" id="omniroute-start-btn">▶ ' + omniRouteText('config.omniroute.start_button') + '</button>';
        html += '<button class="btn-save btn-secondary" onclick="omniRouteStopSidecar()" id="omniroute-stop-btn">⏹ ' + omniRouteText('config.omniroute.stop_button') + '</button>';
    }
    html += '<span id="omniroute-result" class="adg-test-result"></span>';
    html += '</div>';
    html += '<div id="omniroute-status-panel" class="adg-status-banner"></div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    omniRouteRefreshStatus();
}

function omniRouteToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const helpText = omniRouteText(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + omniRouteText(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || 'toggleBool(this)') + '"></div>';
    html += '</div>';
    return html;
}

function omniRouteField(labelKey, helpKey, inputHtml) {
    const helpText = omniRouteText(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + omniRouteText(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function omniRouteSelectField(labelKey, helpKey, path, value, options) {
    const normalizedValue = String(value ?? '');
    const optionValues = options.map(opt => String(opt.value ?? ''));
    const isCustom = normalizedValue !== '' && !optionValues.includes(normalizedValue);
    const customOption = typeof CFG_OPTION_OTHER_CUSTOM === 'string' ? CFG_OPTION_OTHER_CUSTOM : 'Other / Custom';
    const customLabel = typeof cfgFieldOptionLabel === 'function' ? cfgFieldOptionLabel(customOption) : omniRouteText('config.field.other_custom_option', customOption);
    let html = '<select class="field-select" data-path="' + escapeAttr(path) + '" onchange="omniRouteSelectChanged(this)">';
    options.forEach(opt => {
        const optValue = String(opt.value ?? '');
        const selected = !isCustom && normalizedValue === optValue ? ' selected' : '';
        const label = opt.label !== undefined ? opt.label : optValue;
        html += '<option value="' + escapeAttr(optValue) + '"' + selected + '>' + escapeHtml(label) + '</option>';
    });
    html += '<option value="' + escapeAttr(customOption) + '"' + (isCustom ? ' selected' : '') + '>' + escapeHtml(customLabel) + '</option>';
    html += '</select>';
    html += '<input class="field-input cfg-custom-input' + (isCustom ? '' : ' is-hidden') + '" type="text" data-custom-for="' + escapeAttr(path) + '" value="' + escapeAttr(isCustom ? normalizedValue : '') + '" placeholder="' + escapeAttr(customLabel) + '" oninput="omniRouteCustomChanged(this)">';
    return omniRouteField(labelKey, helpKey, html);
}

function omniRouteSecretField(labelKey, helpKey, id, path, placeholder) {
    return omniRouteField(labelKey, helpKey,
        '<div class="password-wrap"><input class="field-input" type="password" id="' + id + '" value="' + escapeAttr(cfgSecretValue(path.split('.').reduce((o,k)=>o&&o[k], configData))) + '" placeholder="' + escapeAttr(placeholder) + '" data-path="' + path + '" autocomplete="off"><button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>');
}

function omniRouteToggleEnabled(isOn) {
    const data = omniRouteEnsureData();
    data.enabled = !isOn;
    renderOmniRouteSection(null);
    markDirty();
}

function omniRoutePayload() {
    omniRouteEnsureData();
    if (typeof buildConfigPatchFromForm === 'function') {
        const patch = buildConfigPatchFromForm();
        const merged = Object.assign({}, configData.omniroute, patch.omniroute || {});
        return { omniroute: merged };
    }
    return { omniroute: configData.omniroute };
}

function omniRouteSelectChanged(selectEl) {
    if (typeof cfgToggleCustomInput === 'function') cfgToggleCustomInput(selectEl);
    const customOption = typeof CFG_OPTION_OTHER_CUSTOM === 'string' ? CFG_OPTION_OTHER_CUSTOM : 'Other / Custom';
    if (selectEl.value !== customOption) {
        setNestedValue(configData, selectEl.dataset.path, selectEl.value);
    }
    markDirty();
}

function omniRouteCustomChanged(inputEl) {
    const path = inputEl.dataset.customFor;
    if (path) setNestedValue(configData, path, inputEl.value.trim());
    markDirty();
}

function omniRouteStatusState(body) {
    const status = String(body && body.status ? body.status : '').toLowerCase();
    if (body && body.admin_setup_required) return 'warning';
    if (status === 'running' || status === 'ok' || status === 'connected') return 'success';
    if (status === 'error' || status === 'failed') return 'danger';
    if (status === 'stopped' || status === 'stopping' || status === 'starting' || status === 'setup_required') return 'warning';
    return '';
}

async function omniRouteRefreshStatus() {
    const panel = document.getElementById('omniroute-status-panel');
    if (!panel) return;
    try {
        const resp = await fetch('/api/omniroute/status');
        const body = await resp.json();
        panel.className = 'adg-status-banner' + (omniRouteStatusState(body) ? ' is-' + omniRouteStatusState(body) : '');
        panel.innerHTML = omniRouteStatusHTML(body);
    } catch (e) {
        panel.className = 'adg-status-banner is-danger';
        panel.textContent = omniRouteText('config.omniroute.status_error') + ' ' + e.message;
    }
}

function omniRouteStatusHTML(body) {
    const parts = [];
    parts.push('<strong>' + escapeHtml(omniRouteText('config.omniroute.status_prefix')) + '</strong> ' + escapeHtml(body.status || 'unknown'));
    if (body.url) parts.push('<a href="' + escapeAttr(body.url) + '" target="_blank" rel="noopener noreferrer">' + escapeHtml(body.url) + '</a>');
    if (body.provider_base_url) parts.push('<code>' + escapeHtml(body.provider_base_url) + '</code>');
    if (body.admin_setup_required) parts.push('<span>' + escapeHtml(omniRouteText('config.omniroute.admin_setup_required')) + '</span>');
    if (body.message) parts.push('<span>' + escapeHtml(body.message) + '</span>');
    return parts.join('<br>');
}

async function omniRouteTestConnection() {
    await omniRouteAction('/api/omniroute/test', 'omniroute-test-btn', omniRouteText('config.omniroute.testing'));
}

async function omniRouteStartSidecar() {
    await omniRouteAction('/api/omniroute/start', 'omniroute-start-btn', omniRouteText('config.omniroute.starting'));
}

async function omniRouteStopSidecar() {
    await omniRouteAction('/api/omniroute/stop', 'omniroute-stop-btn', omniRouteText('config.omniroute.stopping'));
}

async function omniRouteAction(url, buttonId, loadingText) {
    const result = document.getElementById('omniroute-result');
    const btn = document.getElementById(buttonId);
    if (btn) btn.disabled = true;
    if (result) {
        result.className = 'adg-test-result';
        result.textContent = loadingText;
    }
    try {
        const resp = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: url.endsWith('/test') ? JSON.stringify(omniRoutePayload()) : undefined
        });
        const body = await resp.json();
        if (result) {
            result.className = resp.ok && body.status !== 'error' ? 'adg-test-result is-success' : 'adg-test-result is-danger';
            result.textContent = (body.message || body.status || '').toString();
        }
        const panel = document.getElementById('omniroute-status-panel');
        if (panel) {
            panel.className = 'adg-status-banner' + (omniRouteStatusState(body) ? ' is-' + omniRouteStatusState(body) : '');
            panel.innerHTML = omniRouteStatusHTML(body);
        }
    } catch (e) {
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = omniRouteText('config.omniroute.status_error') + ' ' + e.message;
        }
    } finally {
        if (btn) btn.disabled = false;
    }
}
