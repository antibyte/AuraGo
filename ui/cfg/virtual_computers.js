// cfg/virtual_computers.js - Virtual Computers config section module

let _virtualComputersSection = null;

function vcCfgEnsureData() {
    if (!configData.virtual_computers) configData.virtual_computers = {};
    if (!configData.virtual_computers.control_plane) configData.virtual_computers.control_plane = {};
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.virtual_computers) configData.tools.virtual_computers = {};
    const data = configData.virtual_computers;
    const cp = data.control_plane;
    if (!data.provider) data.provider = 'boring_computers';
    if (!data.default_template) data.default_template = 'python';
    if (!data.default_ttl_seconds) data.default_ttl_seconds = 600;
    if (!data.max_ttl_seconds) data.max_ttl_seconds = 900;
    if (!data.max_running_machines) data.max_running_machines = 3;
    if (!data.max_forks) data.max_forks = 3;
    if (!cp.mode) cp.mode = 'ssh_host';
    if (!cp.ssh_port) cp.ssh_port = 22;
    if (!cp.install_dir) cp.install_dir = '/opt/boring-computers';
    if (!cp.boringd_url) cp.boringd_url = 'http://127.0.0.1:8080';
    return data;
}

function renderVirtualComputersSection(section) {
    if (section) _virtualComputersSection = section; else section = _virtualComputersSection;
    const data = vcCfgEnsureData();
    const cp = data.control_plane || {};
    const enabled = data.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += vcCfgToggleRow('config.virtual_computers.enabled_label', 'help.virtual_computers.enabled', enabled, 'virtual_computers.enabled', 'vcCfgToggleEnabled(this.classList.contains("on"))');

    if (!enabled) {
        html += '<div class="wh-notice"><span>i</span><div>';
        html += '<strong>' + t('config.virtual_computers.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.virtual_computers.disabled_desc') + '</small>';
        html += '</div></div>';
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_computers.private_note') + '</div>';
    html += vcCfgToggleRow('config.virtual_computers.readonly_label', 'help.virtual_computers.readonly', data.readonly === true, 'virtual_computers.readonly');
    html += vcCfgToggleRow('config.virtual_computers.tool_enabled_label', 'help.virtual_computers.tool_enabled', configData.tools.virtual_computers.enabled === true, 'tools.virtual_computers.enabled');
    html += vcCfgToggleRow('config.virtual_computers.auto_setup_label', 'help.virtual_computers.auto_setup', data.auto_setup === true, 'virtual_computers.auto_setup');

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.virtual_computers.control_plane_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.virtual_computers.control_plane_desc') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vcCfgField('config.virtual_computers.provider_label', 'help.virtual_computers.provider',
        '<select class="field-input" data-path="virtual_computers.provider"><option value="boring_computers" selected>boring_computers</option></select>');
    html += vcCfgField('config.virtual_computers.mode_label', 'help.virtual_computers.mode',
        '<select class="field-input" data-path="virtual_computers.control_plane.mode"><option value="ssh_host"' + ((cp.mode || 'ssh_host') === 'ssh_host' ? ' selected' : '') + '>' + t('config.virtual_computers.mode_ssh_host') + '</option></select>');
    html += vcCfgField('config.virtual_computers.host_label', 'help.virtual_computers.host',
        '<input class="field-input" type="text" data-path="virtual_computers.control_plane.host" value="' + escapeAttr(cp.host || '') + '" placeholder="vm-host.local">');
    html += vcCfgField('config.virtual_computers.ssh_port_label', 'help.virtual_computers.ssh_port',
        '<input class="field-input" type="number" min="1" max="65535" data-path="virtual_computers.control_plane.ssh_port" value="' + escapeAttr(cp.ssh_port || 22) + '">');
    html += vcCfgField('config.virtual_computers.credential_id_label', 'help.virtual_computers.credential_id',
        '<input class="field-input" type="text" data-path="virtual_computers.control_plane.credential_id" value="' + escapeAttr(cp.credential_id || '') + '" placeholder="credential-id">');
    html += vcCfgField('config.virtual_computers.install_dir_label', 'help.virtual_computers.install_dir',
        '<input class="field-input" type="text" data-path="virtual_computers.control_plane.install_dir" value="' + escapeAttr(cp.install_dir || '/opt/boring-computers') + '">');
    html += vcCfgField('config.virtual_computers.boringd_url_label', 'help.virtual_computers.boringd_url',
        '<input class="field-input" type="url" data-path="virtual_computers.control_plane.boringd_url" value="' + escapeAttr(cp.boringd_url || 'http://127.0.0.1:8080') + '">');
    html += '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.virtual_computers.limits_title') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vcCfgField('config.virtual_computers.default_template_label', 'help.virtual_computers.default_template',
        '<select class="field-input" data-path="virtual_computers.default_template">' +
        '<option value="python"' + ((data.default_template || 'python') === 'python' ? ' selected' : '') + '>python</option>' +
        '<option value="desktop"' + (data.default_template === 'desktop' ? ' selected' : '') + '>desktop</option>' +
        '</select>');
    html += vcCfgField('config.virtual_computers.default_ttl_label', 'help.virtual_computers.default_ttl_seconds',
        '<input class="field-input" type="number" min="15" max="900" data-path="virtual_computers.default_ttl_seconds" value="' + escapeAttr(data.default_ttl_seconds || 600) + '">');
    html += vcCfgField('config.virtual_computers.max_ttl_label', 'help.virtual_computers.max_ttl_seconds',
        '<input class="field-input" type="number" min="15" max="900" data-path="virtual_computers.max_ttl_seconds" value="' + escapeAttr(data.max_ttl_seconds || 900) + '">');
    html += vcCfgField('config.virtual_computers.max_running_label', 'help.virtual_computers.max_running_machines',
        '<input class="field-input" type="number" min="1" max="50" data-path="virtual_computers.max_running_machines" value="' + escapeAttr(data.max_running_machines || 3) + '">');
    html += vcCfgField('config.virtual_computers.max_forks_label', 'help.virtual_computers.max_forks',
        '<input class="field-input" type="number" min="0" max="50" data-path="virtual_computers.max_forks" value="' + escapeAttr(data.max_forks || 3) + '">');
    html += '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.virtual_computers.gates_title') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vcCfgToggleRow('config.virtual_computers.allow_internet_label', 'help.virtual_computers.allow_internet', data.allow_internet === true, 'virtual_computers.allow_internet');
    html += vcCfgToggleRow('config.virtual_computers.allow_persistent_label', 'help.virtual_computers.allow_persistent', data.allow_persistent === true, 'virtual_computers.allow_persistent');
    html += vcCfgToggleRow('config.virtual_computers.allow_publish_label', 'help.virtual_computers.allow_publish', data.allow_publish === true, 'virtual_computers.allow_publish');
    html += vcCfgToggleRow('config.virtual_computers.allow_volumes_label', 'help.virtual_computers.allow_volumes', data.allow_volumes === true, 'virtual_computers.allow_volumes');
    html += vcCfgToggleRow('config.virtual_computers.allow_agent_tasks_label', 'help.virtual_computers.allow_agent_tasks', data.allow_agent_tasks === true, 'virtual_computers.allow_agent_tasks');
    html += '</div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.virtual_computers.boring_token_label') + '</div>';
    html += '<div class="field-help">' + t('help.virtual_computers.boring_token') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="vc-boring-token-input" value="' + escapeAttr(cfgSecretValue(data.boring_token)) + '" placeholder="' + escapeAttr(cfgSecretPlaceholder(data.boring_token, 'boring-token')) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '<button class="btn-save adg-save-btn" onclick="vcCfgSaveSecret(\'vc-boring-token-input\', \'virtual_computers_boring_token\', \'virtual_computers.boring_token\', \'vc-boring-token-status\')">' + t('config.virtual_computers.save_vault') + '</button>';
    html += '</div><div id="vc-boring-token-status" class="adg-test-result"></div>';
    html += '</div>';

    html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_computers.tailscale_note') + ' <a href="/config#tailscale">' + t('config.virtual_computers.open_tailscale_settings') + '</a></div>';
    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="vcCfgCheckStatus()" id="vc-status-btn">' + t('config.virtual_computers.status_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="vcCfgPreflight()" id="vc-preflight-btn">' + t('config.virtual_computers.preflight_button') + '</button>';
    html += '<button class="btn-save dc-test-btn" onclick="vcCfgInstall()" id="vc-install-btn">' + t('config.virtual_computers.install_button') + '</button>';
    html += '<span id="vc-cfg-result" class="dc-test-result"></span>';
    html += '</div></div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function vcCfgToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || 'toggleBool(this)') + '"></div>';
    html += '</div>';
    return html;
}

function vcCfgField(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function vcCfgToggleEnabled(isOn) {
    const data = vcCfgEnsureData();
    data.enabled = !isOn;
    setNestedValue(configData, 'virtual_computers.enabled', data.enabled);
    setDirty(true);
    renderVirtualComputersSection(null);
}

function vcCfgSetResult(text, state) {
    const result = document.getElementById('vc-cfg-result');
    if (!result) return;
    result.className = 'dc-test-result' + (state ? ' is-' + state : '');
    result.textContent = text || '';
}

async function vcCfgCheckStatus() {
    const btn = document.getElementById('vc-status-btn');
    if (btn) btn.disabled = true;
    vcCfgSetResult(t('config.virtual_computers.status_loading'), '');
    try {
        const resp = await fetch('/api/virtual-computers/setup/status');
        const body = await resp.json();
        if (!resp.ok || body.error) throw new Error(body.error || resp.statusText);
        vcCfgSetResult(body.enabled ? t('config.virtual_computers.status_ok') : t('config.virtual_computers.status_disabled'), body.enabled ? 'success' : '');
    } catch (e) {
        vcCfgSetResult(t('config.virtual_computers.status_error') + ' ' + e.message, 'danger');
    } finally {
        if (btn) btn.disabled = false;
    }
}

async function vcCfgPreflight() {
    await vcCfgPostSetup('/api/virtual-computers/setup/preflight', 'vc-preflight-btn');
}

async function vcCfgInstall() {
    await vcCfgPostSetup('/api/virtual-computers/setup/install', 'vc-install-btn');
}

async function vcCfgPostSetup(url, buttonID) {
    const btn = document.getElementById(buttonID);
    if (btn) btn.disabled = true;
    vcCfgSetResult(t('config.virtual_computers.status_loading'), '');
    try {
        const resp = await fetch(url, { method: 'POST' });
        const body = await resp.json();
        if (!resp.ok || body.error) throw new Error(body.error || resp.statusText);
        vcCfgSetResult(body.message || t('config.virtual_computers.status_ok'), 'success');
    } catch (e) {
        vcCfgSetResult(t('config.virtual_computers.status_error') + ' ' + e.message, 'danger');
    } finally {
        if (btn) btn.disabled = false;
    }
}

function vcCfgSaveSecret(inputID, vaultKey, configPath, statusID) {
    const input = document.getElementById(inputID);
    const statusEl = document.getElementById(statusID);
    const value = input ? input.value.trim() : '';
    if (!value) {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.virtual_computers.token_empty');
        }
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: vaultKey, value })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            if (statusEl) {
                statusEl.className = 'adg-test-result is-success';
                statusEl.textContent = t('config.virtual_computers.token_saved');
            }
            cfgMarkSecretStored(input, configPath);
        } else if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = res.message || t('config.virtual_computers.token_save_failed');
        }
    })
    .catch(() => {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.virtual_computers.token_save_failed');
        }
    });
}
