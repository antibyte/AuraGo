// cfg/manifest.js - Manifest.build gateway integration section module

let _manifestSection = null;

function manifestEnsureData() {
    if (!configData.manifest) configData.manifest = {};
    const data = configData.manifest;
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
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += manifestToggleRow('config.manifest.enabled_label', 'help.manifest.enabled', enabled, 'manifest.enabled', "manifestToggleEnabled(this.classList.contains('on'))");

    if (!enabled) {
        html += '<div class="wh-notice"><span>▦</span><div>';
        html += '<strong>' + t('config.manifest.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.manifest.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">▦ ' + t('config.manifest.sidecar_note') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += manifestField('config.manifest.mode_label', 'help.manifest.mode',
        '<select class="field-input" data-path="manifest.mode" onchange="setNestedValue(configData,\\'manifest.mode\\',this.value);setDirty(true);renderManifestSection(null)">' +
        '<option value="managed"' + (managed ? ' selected' : '') + '>' + t('config.manifest.mode_managed') + '</option>' +
        '<option value="external"' + (!managed ? ' selected' : '') + '>' + t('config.manifest.mode_external') + '</option>' +
        '</select>');
    html += manifestToggleRow('config.manifest.auto_start_label', 'help.manifest.auto_start', data.auto_start !== false, 'manifest.auto_start');
    html += '</div>';

    if (managed) {
        html += '<div class="field-grid two-cols">';
        html += manifestField('config.manifest.url_label', 'help.manifest.url',
            '<input class="field-input" type="url" value="' + escapeAttr(data.url || 'http://127.0.0.1:2099') + '" data-path="manifest.url">');
        html += manifestField('config.manifest.host_label', 'help.manifest.host',
            '<input class="field-input" type="text" value="' + escapeAttr(data.host || '127.0.0.1') + '" data-path="manifest.host">');
        html += '</div>';
        html += '<div class="field-grid two-cols">';
        html += manifestField('config.manifest.port_label', 'help.manifest.port',
            '<input class="field-input" type="number" min="1" max="65535" value="' + (data.port || 2099) + '" data-path="manifest.port">');
        html += manifestField('config.manifest.host_port_label', 'help.manifest.host_port',
            '<input class="field-input" type="number" min="1" max="65535" value="' + (data.host_port || 2099) + '" data-path="manifest.host_port">');
        html += '</div>';
        html += '<details class="cfg-advanced-panel manifest-advanced-panel">';
        html += '<summary class="cfg-advanced-summary">' + t('config.manifest.advanced_label') + '</summary>';
        html += '<div class="cfg-advanced-body">';
        html += manifestField('config.manifest.image_label', 'help.manifest.image',
            '<input class="field-input" type="text" value="' + escapeAttr(data.image || 'manifestdotbuild/manifest:5') + '" data-path="manifest.image">');
        html += manifestField('config.manifest.container_name_label', 'help.manifest.container_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.container_name || 'aurago_manifest') + '" data-path="manifest.container_name">');
        html += manifestField('config.manifest.network_name_label', 'help.manifest.network_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.network_name || 'aurago_manifest') + '" data-path="manifest.network_name">');
        html += manifestField('config.manifest.postgres_container_name_label', 'help.manifest.postgres_container_name',
            '<input class="field-input" type="text" value="' + escapeAttr(data.postgres_container_name || 'aurago_manifest_postgres') + '" data-path="manifest.postgres_container_name">');
        html += manifestField('config.manifest.postgres_image_label', 'help.manifest.postgres_image',
            '<input class="field-input" type="text" value="' + escapeAttr(data.postgres_image || 'postgres:15-alpine') + '" data-path="manifest.postgres_image">');
        html += manifestField('config.manifest.postgres_volume_label', 'help.manifest.postgres_volume',
            '<input class="field-input" type="text" value="' + escapeAttr(data.postgres_volume || 'aurago_manifest_pgdata') + '" data-path="manifest.postgres_volume">');
        html += manifestField('config.manifest.health_path_label', 'help.manifest.health_path',
            '<input class="field-input" type="text" value="' + escapeAttr(data.health_path || '') + '" data-path="manifest.health_path" placeholder="/health">');
        html += '</div></details>';
    } else {
        html += manifestField('config.manifest.external_base_url_label', 'help.manifest.external_base_url',
            '<input class="field-input" type="url" value="' + escapeAttr(data.external_base_url || 'https://app.manifest.build/v1') + '" data-path="manifest.external_base_url">');
    }

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.manifest.secrets_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.manifest.secrets_desc') + '</div>';
    html += manifestSecretField('config.manifest.api_key_label', 'help.manifest.api_key', 'manifest-api-key-input', 'manifest.api_key', 'mnfst_••••••••');
    if (managed) {
        html += manifestSecretField('config.manifest.postgres_password_label', 'help.manifest.postgres_password', 'manifest-postgres-password-input', 'manifest.postgres_password', t('config.providers.key_placeholder_existing'));
        html += manifestSecretField('config.manifest.better_auth_secret_label', 'help.manifest.better_auth_secret', 'manifest-better-auth-secret-input', 'manifest.better_auth_secret', t('config.providers.key_placeholder_existing'));
    }
    html += '</div>';

    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="manifestTestConnection()" id="manifest-test-btn">' + t('config.manifest.test_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="manifestStartSidecars()" id="manifest-start-btn">' + t('config.manifest.start_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="manifestStopSidecars()" id="manifest-stop-btn">' + t('config.manifest.stop_button') + '</button>';
    html += '<span id="manifest-result" class="dc-test-result"></span>';
    html += '</div>';
    html += '<div id="manifest-status-panel" class="cfg-note-banner cfg-note-banner-info"></div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    manifestRefreshStatus();
}

function manifestToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || 'toggleBool(this)') + '"></div>';
    html += '</div>';
    return html;
}

function manifestField(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
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
    return { manifest: configData.manifest };
}

async function manifestRefreshStatus() {
    const panel = document.getElementById('manifest-status-panel');
    if (!panel) return;
    try {
        const resp = await fetch('/api/manifest/status');
        const body = await resp.json();
        panel.innerHTML = manifestStatusHTML(body);
    } catch (e) {
        panel.textContent = t('config.manifest.status_error') + ' ' + e.message;
    }
}

function manifestStatusHTML(body) {
    const parts = [];
    parts.push('<strong>' + escapeHtml(t('config.manifest.status_prefix')) + '</strong> ' + escapeHtml(body.status || 'unknown'));
    if (body.url) parts.push('<a href="' + escapeAttr(body.url) + '" target="_blank" rel="noopener noreferrer">' + escapeHtml(body.url) + '</a>');
    if (body.provider_base_url) parts.push('<code>' + escapeHtml(body.provider_base_url) + '</code>');
    if (body.admin_setup_required) parts.push('<span>' + escapeHtml(t('config.manifest.admin_setup_required')) + '</span>');
    if (body.message) parts.push('<span>' + escapeHtml(body.message) + '</span>');
    return parts.join('<br>');
}

async function manifestTestConnection() {
    await manifestAction('/api/manifest/test', 'manifest-test-btn', t('config.manifest.testing'));
}

async function manifestStartSidecars() {
    await manifestAction('/api/manifest/start', 'manifest-start-btn', t('config.manifest.starting'));
}

async function manifestStopSidecars() {
    await manifestAction('/api/manifest/stop', 'manifest-stop-btn', t('config.manifest.stopping'));
}

async function manifestAction(url, buttonId, loadingText) {
    const result = document.getElementById('manifest-result');
    const btn = document.getElementById(buttonId);
    if (btn) btn.disabled = true;
    if (result) {
        result.className = 'dc-test-result';
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
            result.className = resp.ok && body.status !== 'error' ? 'dc-test-result is-success' : 'dc-test-result is-danger';
            result.textContent = (body.message || body.status || '').toString();
        }
        const panel = document.getElementById('manifest-status-panel');
        if (panel) panel.innerHTML = manifestStatusHTML(body);
    } catch (e) {
        if (result) {
            result.className = 'dc-test-result is-danger';
            result.textContent = t('config.manifest.status_error') + ' ' + e.message;
        }
    } finally {
        if (btn) btn.disabled = false;
    }
}
