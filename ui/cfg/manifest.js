// cfg/manifest.js - Manifest.build gateway integration section module

let _manifestSection = null;

function manifestText(key, fallback) {
    if (typeof t === 'function') {
        const value = t(key);
        if (typeof value === 'string' && value.trim() !== '' && value !== key) return value;
    }
    if (typeof fallback === 'string' && fallback.trim() !== '' && fallback !== key) return fallback;
    return '';
}

function manifestEnsureData() {
    if (!configData.manifest) configData.manifest = {};
    const data = configData.manifest;
    if (!data.routing) data.routing = {};
    if (typeof data.routing.enabled !== 'boolean') data.routing.enabled = false;
    if (!['off', 'fixed', 'auto'].includes(data.routing.specificity_mode)) data.routing.specificity_mode = 'off';
    if (typeof data.routing.specificity !== 'string') data.routing.specificity = '';
    if (!data.routing.headers || typeof data.routing.headers !== 'object' || Array.isArray(data.routing.headers)) data.routing.headers = {};
    if (typeof data.auto_start !== 'boolean') data.auto_start = true;
    if (!data.mode) data.mode = 'managed';
    if (!data.url) data.url = 'http://127.0.0.1:2099';
    if (!data.external_base_url) data.external_base_url = 'https://app.manifest.build/v1';
    if (!data.container_name) data.container_name = 'aurago_manifest';
    if (!data.image) data.image = 'manifestdotbuild/manifest:5';
    if (!data.host) data.host = '127.0.0.1';
    if (!data.port) data.port = 2099;
    if (!data.host_port) data.host_port = 2099;
    if (!data.network_name) data.network_name = 'aurago_manifest';
    if (!data.postgres_container_name) data.postgres_container_name = 'aurago_manifest_postgres';
    if (!data.postgres_image) data.postgres_image = 'postgres:15-alpine';
    if (!data.postgres_user) data.postgres_user = 'manifest';
    if (!data.postgres_database) data.postgres_database = 'manifest';
    if (!data.postgres_volume) data.postgres_volume = 'aurago_manifest_pgdata';
    return data;
}

async function renderManifestSection(section) {
    if (section) _manifestSection = section; else section = _manifestSection;
    const data = manifestEnsureData();
    const enabled = data.enabled === true;
    const managed = data.mode !== 'external';

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + manifestText('config.section.manifest.label', section.label) + '</div>';
    html += '<div class="section-desc">' + manifestText('config.section.manifest.desc', section.desc) + '</div>';

    html += manifestToggleRow('config.manifest.enabled_label', 'help.manifest.enabled', enabled, 'manifest.enabled', "manifestToggleEnabled(this.classList.contains('on'))");

    if (!enabled) {
        html += '<div class="wh-notice"><span>▦</span><div>';
        html += '<strong>' + manifestText('config.manifest.disabled_notice') + '</strong><br>';
        html += '<small>' + manifestText('config.manifest.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">▦ ' + manifestText('config.manifest.sidecar_note') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += manifestField('config.manifest.mode_label', 'help.manifest.mode',
        '<select class="field-select" data-path="manifest.mode" onchange="setNestedValue(configData,\\'manifest.mode\\',this.value);setDirty(true);renderManifestSection(null)">' +
        '<option value="managed"' + (managed ? ' selected' : '') + '>' + manifestText('config.manifest.mode_managed') + '</option>' +
        '<option value="external"' + (!managed ? ' selected' : '') + '>' + manifestText('config.manifest.mode_external') + '</option>' +
        '</select>');
    if (managed) {
        html += manifestToggleRow('config.manifest.auto_start_label', 'help.manifest.auto_start', data.auto_start !== false, 'manifest.auto_start');
    }
    html += '</div>';

    if (managed) {
        const manifestPort = data.port || 2099;
        html += '<div class="field-grid two-cols">';
        html += manifestSelectField('config.manifest.url_label', 'help.manifest.url', 'manifest.url', data.url || 'http://127.0.0.1:2099', [
            { value: 'http://manifest:' + manifestPort },
            { value: 'http://127.0.0.1:' + manifestPort }
        ]);
        html += manifestSelectField('config.manifest.host_label', 'help.manifest.host', 'manifest.host', data.host || '127.0.0.1', [
            { value: '127.0.0.1' },
            { value: '0.0.0.0' },
            { value: 'localhost' }
        ]);
        html += '</div>';
        html += '<div class="field-grid two-cols">';
        html += manifestField('config.manifest.port_label', 'help.manifest.port',
            '<input class="field-input" type="number" min="1" max="65535" value="' + (data.port || 2099) + '" data-path="manifest.port">');
        html += manifestField('config.manifest.host_port_label', 'help.manifest.host_port',
            '<input class="field-input" type="number" min="1" max="65535" value="' + (data.host_port || 2099) + '" data-path="manifest.host_port">');
        html += '</div>';
        html += '<details class="cfg-advanced-panel manifest-advanced-panel">';
        html += '<summary class="cfg-advanced-summary">' + manifestText('config.manifest.advanced_label') + '</summary>';
        html += '<div class="cfg-advanced-body">';
        html += manifestSelectField('config.manifest.image_label', 'help.manifest.image', 'manifest.image', data.image || 'manifestdotbuild/manifest:5', [
            { value: 'manifestdotbuild/manifest:5' }
        ]);
        html += manifestField('config.manifest.container_name_label', 'help.manifest.container_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.container_name || 'aurago_manifest') + '" data-path="manifest.container_name">');
        html += manifestField('config.manifest.network_name_label', 'help.manifest.network_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.network_name || 'aurago_manifest') + '" data-path="manifest.network_name">');
        html += manifestField('config.manifest.postgres_container_name_label', 'help.manifest.postgres_container_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.postgres_container_name || 'aurago_manifest_postgres') + '" data-path="manifest.postgres_container_name">');
        html += manifestSelectField('config.manifest.postgres_image_label', 'help.manifest.postgres_image', 'manifest.postgres_image', data.postgres_image || 'postgres:15-alpine', [
            { value: 'postgres:15-alpine' },
            { value: 'postgres:16-alpine' },
            { value: 'postgres:17-alpine' }
        ]);
        html += manifestField('config.manifest.postgres_volume_label', 'help.manifest.postgres_volume',
            '<input class="field-input" type="text" value="' + escapeAttr(data.postgres_volume || 'aurago_manifest_pgdata') + '" data-path="manifest.postgres_volume">');
        html += manifestSelectField('config.manifest.health_path_label', 'help.manifest.health_path', 'manifest.health_path', data.health_path || '', [
            { value: '', label: 'auto' },
            { value: '/health' },
            { value: '/api/health' }
        ]);
        html += '</div></details>';
    } else {
        html += manifestSelectField('config.manifest.external_base_url_label', 'help.manifest.external_base_url', 'manifest.external_base_url', data.external_base_url || 'https://app.manifest.build/v1', [
            { value: 'https://app.manifest.build/v1' }
        ]);
    }

    html += '<details class="cfg-advanced-panel manifest-routing-panel">';
    html += '<summary class="cfg-advanced-summary">' + manifestText('config.manifest.routing_title') + '</summary>';
    html += '<div class="cfg-advanced-body">';
    html += '<div class="cfg-note-banner cfg-note-banner-info">▦ ' + manifestText('config.manifest.routing_note') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += manifestToggleRow('config.manifest.routing_enabled_label', '', data.routing.enabled === true, 'manifest.routing.enabled');
    html += manifestSelectFieldFixed('config.manifest.routing_specificity_mode_label', '', 'manifest.routing.specificity_mode', data.routing.specificity_mode || 'off', [
        { value: 'off', label: manifestText('config.manifest.routing_mode_off') },
        { value: 'fixed', label: manifestText('config.manifest.routing_mode_fixed') },
        { value: 'auto', label: manifestText('config.manifest.routing_mode_auto') }
    ]);
    html += '</div>';
    html += manifestSelectFieldFixed('config.manifest.routing_specificity_label', '', 'manifest.routing.specificity', data.routing.specificity || '', manifestSpecificityOptions());
    html += manifestRoutingHeaderRows(data.routing.headers);
    html += '</div></details>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + manifestText('config.manifest.secrets_title') + '</div>';
    html += '<div class="field-group-desc">' + manifestText('config.manifest.secrets_desc') + '</div>';
    html += manifestSecretField('config.manifest.api_key_label', 'help.manifest.api_key', 'manifest-api-key-input', 'manifest.api_key', 'mnfst_••••••••');
    if (managed) {
        html += manifestSecretField('config.manifest.postgres_password_label', 'help.manifest.postgres_password', 'manifest-postgres-password-input', 'manifest.postgres_password', t('config.providers.key_placeholder_existing'));
        html += manifestSecretField('config.manifest.better_auth_secret_label', 'help.manifest.better_auth_secret', 'manifest-better-auth-secret-input', 'manifest.better_auth_secret', t('config.providers.key_placeholder_existing'));
    }
    html += '</div>';

    html += '<div class="cfg-actions-row">';
    html += '<button class="btn-save adg-test-btn" onclick="manifestTestConnection()" id="manifest-test-btn">🔌 ' + manifestText('config.manifest.test_button') + '</button>';
    if (managed) {
        html += '<button class="btn-save btn-secondary" onclick="manifestStartSidecars()" id="manifest-start-btn">▶ ' + manifestText('config.manifest.start_button') + '</button>';
        html += '<button class="btn-save btn-secondary" onclick="manifestStopSidecars()" id="manifest-stop-btn">⏹ ' + manifestText('config.manifest.stop_button') + '</button>';
    }
    html += '<span id="manifest-result" class="adg-test-result"></span>';
    html += '</div>';
    html += '<div id="manifest-status-panel" class="adg-status-banner"></div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    manifestRefreshStatus();
}

function manifestToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const helpText = manifestText(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + manifestText(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || 'toggleBool(this)') + '"></div>';
    html += '</div>';
    return html;
}

function manifestField(labelKey, helpKey, inputHtml) {
    const helpText = manifestText(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + manifestText(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function manifestSelectField(labelKey, helpKey, path, value, options) {
    const normalizedValue = String(value ?? '');
    const optionValues = options.map(opt => String(opt.value ?? ''));
    const isCustom = normalizedValue !== '' && !optionValues.includes(normalizedValue);
    const customOption = typeof CFG_OPTION_OTHER_CUSTOM === 'string' ? CFG_OPTION_OTHER_CUSTOM : 'Other / Custom';
    const customLabel = typeof cfgFieldOptionLabel === 'function' ? cfgFieldOptionLabel(customOption) : manifestText('config.field.other_custom_option', customOption);
    let html = '<select class="field-select" data-path="' + escapeAttr(path) + '" onchange="manifestSelectChanged(this)">';
    options.forEach(opt => {
        const optValue = String(opt.value ?? '');
        const selected = !isCustom && normalizedValue === optValue ? ' selected' : '';
        const label = opt.label !== undefined ? opt.label : optValue;
        html += '<option value="' + escapeAttr(optValue) + '"' + selected + '>' + escapeHtml(label) + '</option>';
    });
    html += '<option value="' + escapeAttr(customOption) + '"' + (isCustom ? ' selected' : '') + '>' + escapeHtml(customLabel) + '</option>';
    html += '</select>';
    html += '<input class="field-input cfg-custom-input' + (isCustom ? '' : ' is-hidden') + '" type="text" data-custom-for="' + escapeAttr(path) + '" value="' + escapeAttr(isCustom ? normalizedValue : '') + '" placeholder="' + escapeAttr(customLabel) + '" oninput="manifestCustomChanged(this)">';
    return manifestField(labelKey, helpKey, html);
}

function manifestSelectFieldFixed(labelKey, helpKey, path, value, options) {
    const normalizedValue = String(value ?? '');
    let html = '<select class="field-select" data-path="' + escapeAttr(path) + '" onchange="setNestedValue(configData,this.dataset.path,this.value);markDirty()">';
    options.forEach(opt => {
        const optValue = String(opt.value ?? '');
        const selected = normalizedValue === optValue ? ' selected' : '';
        html += '<option value="' + escapeAttr(optValue) + '"' + selected + '>' + escapeHtml(opt.label || optValue) + '</option>';
    });
    html += '</select>';
    return manifestField(labelKey, helpKey, html);
}

function manifestSpecificityOptions() {
    const categories = ['', 'coding', 'web_browsing', 'data_analysis', 'image_generation', 'video_generation', 'social_media', 'email_management', 'calendar_management', 'trading'];
    return categories.map(value => ({
        value,
        label: value === '' ? manifestText('config.manifest.routing_specificity_none') : value
    }));
}

function manifestRoutingHeaderRows(headers) {
    const entries = Object.entries(headers || {}).filter(([key, value]) => String(key).trim() !== '' && String(value).trim() !== '');
    let html = '<div class="field-group manifest-routing-headers" data-routing-path="manifest.routing.headers">';
    html += '<div class="field-label">' + manifestText('config.manifest.routing_headers_label') + '</div>';
    html += '<input type="hidden" data-path="manifest.routing.headers" data-type="json" data-manifest-routing-headers-store value="' + escapeAttr(JSON.stringify(Object.fromEntries(entries))) + '">';
    entries.forEach(([key, value]) => {
        html += '<div class="field-grid two-cols manifest-routing-header-row">';
        html += '<input class="field-input" type="text" value="' + escapeAttr(key) + '" data-header-key oninput="manifestRoutingCollectHeaders()">';
        html += '<div class="password-wrap">';
        html += '<input class="field-input" type="text" value="' + escapeAttr(value) + '" data-header-value oninput="manifestRoutingCollectHeaders()">';
        html += '<button type="button" class="password-toggle" data-header-remove="' + escapeAttr(key) + '" onclick="manifestRoutingRemoveHeader(this.dataset.headerRemove)" title="' + escapeAttr(manifestText('config.manifest.routing_remove_header')) + '">&times;</button>';
        html += '</div></div>';
    });
    html += '<div class="field-grid two-cols manifest-routing-header-row">';
    html += '<input class="field-input" type="text" value="" data-header-key oninput="manifestRoutingCollectHeaders()">';
    html += '<input class="field-input" type="text" value="" data-header-value oninput="manifestRoutingCollectHeaders()">';
    html += '</div>';
    html += '<button type="button" class="btn-save btn-secondary" onclick="manifestRoutingAddHeader()">' + manifestText('config.manifest.routing_add_header') + '</button>';
    html += '</div>';
    return html;
}

function manifestSecretField(labelKey, helpKey, id, path, placeholder) {
    return manifestField(labelKey, helpKey,
        '<div class="password-wrap"><input class="field-input" type="password" id="' + id + '" value="' + escapeAttr(cfgSecretValue(path.split('.').reduce((o,k)=>o&&o[k], configData))) + '" placeholder="' + escapeAttr(placeholder) + '" data-path="' + path + '" autocomplete="off"><button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>');
}

function manifestToggleEnabled(isOn) {
    const data = manifestEnsureData();
    data.enabled = !isOn;
    setDirty(true);
    renderManifestSection(null);
}

function manifestPayload() {
    manifestEnsureData();
    manifestRoutingCollectHeaders(false);
    if (typeof buildConfigPatchFromForm === 'function') {
        const patch = buildConfigPatchFromForm();
        const merged = Object.assign({}, configData.manifest, patch.manifest || {});
        if (patch.manifest && patch.manifest.routing) {
            merged.routing = Object.assign({}, configData.manifest.routing || {}, patch.manifest.routing);
        }
        return { manifest: merged };
    }
    return { manifest: configData.manifest };
}

function manifestSelectChanged(selectEl) {
    if (typeof cfgToggleCustomInput === 'function') cfgToggleCustomInput(selectEl);
    const customOption = typeof CFG_OPTION_OTHER_CUSTOM === 'string' ? CFG_OPTION_OTHER_CUSTOM : 'Other / Custom';
    if (selectEl.value !== customOption) {
        setNestedValue(configData, selectEl.dataset.path, selectEl.value);
    }
    markDirty();
}

function manifestCustomChanged(inputEl) {
    const path = inputEl.dataset.customFor;
    if (path) setNestedValue(configData, path, inputEl.value.trim());
    markDirty();
}

function manifestRoutingCollectHeaders(mark = true) {
    const data = manifestEnsureData();
    const next = {};
    document.querySelectorAll('.manifest-routing-header-row').forEach(row => {
        const keyEl = row.querySelector('[data-header-key]');
        const valueEl = row.querySelector('[data-header-value]');
        if (!keyEl || !valueEl) return;
        const key = String(keyEl.value || '').trim().toLowerCase();
        const value = String(valueEl.value || '').trim();
        if (key && value) next[key] = value;
    });
    data.routing.headers = next;
    const store = document.querySelector('[data-manifest-routing-headers-store]');
    if (store) store.value = JSON.stringify(next);
    if (mark) markDirty();
}

function manifestRoutingAddHeader() {
    manifestRoutingCollectHeaders();
    renderManifestSection(null);
}

function manifestRoutingRemoveHeader(key) {
    const data = manifestEnsureData();
    delete data.routing.headers[String(key || '').trim().toLowerCase()];
    markDirty();
    renderManifestSection(null);
}

function manifestStatusState(body) {
    const status = String(body && body.status ? body.status : '').toLowerCase();
    if (body && body.admin_setup_required) return 'warning';
    if (status === 'running' || status === 'ok' || status === 'connected') return 'success';
    if (status === 'error' || status === 'failed') return 'danger';
    if (status === 'stopped' || status === 'stopping' || status === 'starting') return 'warning';
    return '';
}

async function manifestRefreshStatus() {
    const panel = document.getElementById('manifest-status-panel');
    if (!panel) return;
    try {
        const resp = await fetch('/api/manifest/status');
        const body = await resp.json();
        panel.className = 'adg-status-banner' + (manifestStatusState(body) ? ' is-' + manifestStatusState(body) : '');
        panel.innerHTML = manifestStatusHTML(body);
    } catch (e) {
        panel.className = 'adg-status-banner is-danger';
        panel.textContent = manifestText('config.manifest.status_error') + ' ' + e.message;
    }
}

function manifestStatusHTML(body) {
    const parts = [];
    parts.push('<strong>' + escapeHtml(manifestText('config.manifest.status_prefix')) + '</strong> ' + escapeHtml(body.status || 'unknown'));
    if (body.url) parts.push('<a href="' + escapeAttr(body.url) + '" target="_blank" rel="noopener noreferrer">' + escapeHtml(body.url) + '</a>');
    if (body.provider_base_url) parts.push('<code>' + escapeHtml(body.provider_base_url) + '</code>');
    if (body.admin_setup_required) parts.push('<span>' + escapeHtml(manifestText('config.manifest.admin_setup_required')) + '</span>');
    if (body.message) parts.push('<span>' + escapeHtml(body.message) + '</span>');
    return parts.join('<br>');
}

async function manifestTestConnection() {
    await manifestAction('/api/manifest/test', 'manifest-test-btn', manifestText('config.manifest.testing'));
}

async function manifestStartSidecars() {
    await manifestAction('/api/manifest/start', 'manifest-start-btn', manifestText('config.manifest.starting'));
}

async function manifestStopSidecars() {
    await manifestAction('/api/manifest/stop', 'manifest-stop-btn', manifestText('config.manifest.stopping'));
}

async function manifestAction(url, buttonId, loadingText) {
    const result = document.getElementById('manifest-result');
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
            body: url.endsWith('/test') ? JSON.stringify(manifestPayload()) : undefined
        });
        const body = await resp.json();
        if (result) {
            result.className = resp.ok && body.status !== 'error' ? 'adg-test-result is-success' : 'adg-test-result is-danger';
            result.textContent = (body.message || body.status || '').toString();
        }
        const panel = document.getElementById('manifest-status-panel');
        if (panel) {
            panel.className = 'adg-status-banner' + (manifestStatusState(body) ? ' is-' + manifestStatusState(body) : '');
            panel.innerHTML = manifestStatusHTML(body);
        }
    } catch (e) {
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = manifestText('config.manifest.status_error') + ' ' + e.message;
        }
    } finally {
        if (btn) btn.disabled = false;
    }
}
