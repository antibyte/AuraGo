// cfg/virtual_desktop.js - Virtual Desktop config section module

let _virtualDesktopSection = null;

function vdCfgEnsureData() {
    if (!configData.virtual_desktop) configData.virtual_desktop = {};
    if (!configData.tools) configData.tools = {};
    if (!configData.tools.virtual_desktop) configData.tools.virtual_desktop = {};
    if (!configData.tools.office_document) configData.tools.office_document = {};
    if (!configData.tools.office_workbook) configData.tools.office_workbook = {};
    const data = configData.virtual_desktop;
    if (!data.workspace_dir) data.workspace_dir = 'agent_workspace/virtual_desktop';
    if (!data.max_file_size_mb) data.max_file_size_mb = 50;
    if (!data.max_ws_clients) data.max_ws_clients = 8;
    if (!data.remote_max_session_minutes) data.remote_max_session_minutes = 60;
    if (!data.remote_idle_timeout_minutes) data.remote_idle_timeout_minutes = 5;
    if (typeof data.allow_generated_apps !== 'boolean') data.allow_generated_apps = true;
    if (typeof data.allow_agent_control === 'boolean') {
        configData.tools.virtual_desktop.enabled = data.allow_agent_control === true;
    }
    return data;
}
function renderVirtualDesktopSection(section) {
    if (section) _virtualDesktopSection = section; else section = _virtualDesktopSection;
    const data = vdCfgEnsureData();
    const enabled = data.enabled === true;
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += vdCfgToggleRow('config.virtual_desktop.enabled_label', 'help.virtual_desktop.enabled', enabled, 'virtual_desktop.enabled', "vdCfgToggleEnabled(this)");
    html += vdCfgHiddenToggle('tools.virtual_desktop.enabled', data.allow_agent_control === true);

    if (!enabled) {
        html += '<div class="wh-notice"><span>▣</span><div>';
        html += '<strong>' + t('config.virtual_desktop.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.virtual_desktop.disabled_desc') + '</small>';
        html += '</div></div>';
        html += '</div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-info">▣ ' + t('config.virtual_desktop.first_party_note') + '</div>';
    html += vdCfgToggleRow('config.virtual_desktop.readonly_label', 'help.virtual_desktop.readonly', data.readonly === true, 'virtual_desktop.readonly');
    html += vdCfgToggleRow('config.virtual_desktop.agent_control_label', 'help.virtual_desktop.allow_agent_control', data.allow_agent_control === true, 'virtual_desktop.allow_agent_control', "vdCfgToggleAgentControl(this)");
    html += vdCfgToggleRow('config.virtual_desktop.generated_apps_label', 'help.virtual_desktop.allow_generated_apps', data.allow_generated_apps !== false, 'virtual_desktop.allow_generated_apps');
    html += vdCfgToggleRow('config.virtual_desktop.python_jobs_label', 'help.virtual_desktop.allow_python_jobs', data.allow_python_jobs === true, 'virtual_desktop.allow_python_jobs');
    html += vdCfgToggleRow('config.virtual_desktop.serial_browser_label', 'help.virtual_desktop.serial_browser_enabled', data.serial_browser_enabled === true, 'virtual_desktop.serial_browser_enabled');
    html += vdCfgToggleRow('config.virtual_desktop.serial_host_label', 'help.virtual_desktop.serial_host_enabled', data.serial_host_enabled === true, 'virtual_desktop.serial_host_enabled');
    html += vdCfgToggleRow('config.virtual_desktop.retronet_label', 'help.virtual_desktop.retronet_enabled', data.retronet_enabled === true, 'virtual_desktop.retronet_enabled');
    html += '<div class="cfg-note-banner cfg-note-banner-info">' + t('config.virtual_desktop.office_tools_note') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += vdCfgToggleRow('config.virtual_desktop.office_document_label', 'help.virtual_desktop.office_document', configData.tools.office_document.enabled === true, 'tools.office_document.enabled');
    html += vdCfgToggleRow('config.virtual_desktop.office_document_readonly_label', 'help.virtual_desktop.office_document_readonly', configData.tools.office_document.readonly === true, 'tools.office_document.readonly');
    html += vdCfgToggleRow('config.virtual_desktop.office_workbook_label', 'help.virtual_desktop.office_workbook', configData.tools.office_workbook.enabled === true, 'tools.office_workbook.enabled');
    html += vdCfgToggleRow('config.virtual_desktop.office_workbook_readonly_label', 'help.virtual_desktop.office_workbook_readonly', configData.tools.office_workbook.readonly === true, 'tools.office_workbook.readonly');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += vdCfgField('config.virtual_desktop.workspace_dir_label', 'help.virtual_desktop.workspace_dir',
        '<input class="field-input" type="text" value="' + escapeAttr(data.workspace_dir || 'agent_workspace/virtual_desktop') + '" data-path="virtual_desktop.workspace_dir">');
    html += vdCfgField('config.virtual_desktop.max_file_size_label', 'help.virtual_desktop.max_file_size_mb',
        '<input class="field-input" type="number" min="1" max="1024" value="' + (data.max_file_size_mb || 50) + '" data-path="virtual_desktop.max_file_size_mb">');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += vdCfgField('config.virtual_desktop.max_ws_clients_label', 'help.virtual_desktop.max_ws_clients',
        '<input class="field-input" type="number" min="1" max="64" value="' + (data.max_ws_clients || 8) + '" data-path="virtual_desktop.max_ws_clients">');
    html += '</div>';

    html += '<div class="field-grid two-cols">';
    html += vdCfgField('config.virtual_desktop.remote_max_session_label', 'help.virtual_desktop.remote_max_session_minutes',
        '<input class="field-input" type="number" min="1" max="1440" value="' + (data.remote_max_session_minutes || 60) + '" data-path="virtual_desktop.remote_max_session_minutes">');
    html += vdCfgField('config.virtual_desktop.remote_idle_timeout_label', 'help.virtual_desktop.remote_idle_timeout_minutes',
        '<input class="field-input" type="number" min="1" max="120" value="' + (data.remote_idle_timeout_minutes || 5) + '" data-path="virtual_desktop.remote_idle_timeout_minutes">');
    html += '</div>';

    html += vdCfgVideoStudio();
    html += vdCfgRetroGames();
    const layerling = data.layerling || {};
    html += '<h3>Layerling</h3>';
    html += vdCfgToggleRow('config.layerling.enabled', 'config.layerling.help', layerling.enabled === true, 'virtual_desktop.layerling.enabled');
    const layerlingAccessLabels = { off: t('config.layerling.off'), read: t('config.layerling.read'), write: t('config.layerling.write') };
    html += vdCfgField('config.layerling.access', 'config.layerling.access_help', '<select class="field-select" data-path="virtual_desktop.layerling.agent_access">' + Object.keys(layerlingAccessLabels).map(value => '<option value="' + value + '"' + ((layerling.agent_access || 'off') === value ? ' selected' : '') + '>' + layerlingAccessLabels[value] + '</option>').join('') + '</select>');
    html += '<div class="field-group">';
    html += '<button class="btn-save dc-test-btn" onclick="vdCfgTestDesktop()" id="vd-cfg-test-btn">▣ ' + t('config.virtual_desktop.test_button') + '</button>';
    html += '<a class="btn-save dc-test-btn" href="/desktop">' + t('config.virtual_desktop.open_button') + '</a>';
    html += '<span id="vd-cfg-result" class="dc-test-result"></span>';
    html += '</div>';
    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function vdCfgHiddenToggle(path, enabled) {
    return '<div class="toggle ' + (enabled ? 'on' : '') + ' pw-u-hidden" data-path="' + path + '" aria-hidden="true"></div>';
}

function vdCfgToggleRow(labelKey, helpKey, enabled, path, onclick) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || 'toggleBool(this)') + '"></div>';
    html += '</div>';
    return html;
}

function vdCfgField(labelKey, helpKey, inputHtml) {
    const helpText = t(helpKey);
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    if (helpText) html += '<div class="field-help">' + helpText + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

// Flip the clicked switch first: state sync still reads it, so a stale one reverts the draft.
function vdCfgFlipToggle(el) {
    const on = !el.classList.contains('on');
    el.classList.toggle('on', on);
    el.setAttribute('aria-checked', String(on));
    return on;
}

function vdCfgToggleEnabled(el) {
    const data = vdCfgEnsureData();
    data.enabled = vdCfgFlipToggle(el);
    setDirty(true);
    renderVirtualDesktopSection(null);
}

function vdCfgToggleAgentControl(el) {
    const data = vdCfgEnsureData();
    data.allow_agent_control = vdCfgFlipToggle(el);
    configData.tools.virtual_desktop.enabled = data.allow_agent_control === true;
    setDirty(true);
    renderVirtualDesktopSection(null);
}

async function vdCfgTestDesktop() {
    const result = document.getElementById('vd-cfg-result');
    const btn = document.getElementById('vd-cfg-test-btn');
    btn.disabled = true;
    result.textContent = t('config.virtual_desktop.loading');
    result.className = 'dc-test-result';
    try {
        const resp = await fetch('/api/desktop/bootstrap');
        const body = await resp.json();
        result.className = resp.ok && body.enabled ? 'dc-test-result is-success' : 'dc-test-result';
        result.textContent = body.enabled ? t('config.virtual_desktop.status_ok') : t('config.virtual_desktop.status_disabled');
    } catch (e) {
        result.className = 'dc-test-result is-danger';
        result.textContent = t('config.virtual_desktop.status_error') + ' ' + e.message;
    } finally {
        btn.disabled = false;
    }
}

function vdCfgVideoStudio() {
    const data = configData.video_studio || {};
    const prefix = 'config.video_studio.';
    let html = '<h3>' + t(prefix + 'title') + '</h3><p class="field-help">' + t(prefix + 'help') + '</p>';
    html += '<div class="field-grid two-cols">';
    html += vdCfgToggleRow(prefix + 'enabled', '', data.enabled === true, 'video_studio.enabled');
    html += vdCfgToggleRow(prefix + 'readonly', '', data.readonly === true, 'video_studio.readonly');
    html += vdCfgField(prefix + 'ffmpeg', '', '<input class="field-input" type="text" value="' + escapeAttr(data.ffmpeg_path || '') + '" data-path="video_studio.ffmpeg_path">');
    // The server raises an export limit below 512 MB to 512 MB and never lets it exceed the project limit, so the
    // smallest useful value is min(512, project limit).
    const exportMin = Math.min(512, Number(data.max_project_size_mb) || 16384);
    for (const [name, label, fallback, min, max, help] of [
        ['max_asset_size_mb', 'asset_limit', 4096, 1, 32768],
        ['max_project_size_mb', 'project_limit', 16384, 1, 65536],
        ['max_export_size_mb', 'export_limit', 8192, exportMin, 65536, 'export_limit_help'],
        ['render_timeout_seconds', 'timeout', 7200, 30, 14400]
    ]) {
        html += vdCfgField(prefix + label, help ? prefix + help : '', '<input class="field-input" type="number" min="' + min + '" max="' + max + '" value="' + escapeAttr(String(data[name] || fallback)) + '" data-path="video_studio.' + name + '">');
    }
    const hwModes = { auto: t(prefix + 'hw_auto'), off: t(prefix + 'hw_off'), vaapi: 'VAAPI', qsv: 'Intel Quick Sync', nvenc: 'NVIDIA NVENC', amf: 'AMD AMF' };
    html += vdCfgField(prefix + 'hardware_encoding', prefix + 'hardware_encoding_help', '<select class="field-select" data-path="video_studio.hardware_encoding">' + Object.keys(hwModes).map(value => '<option value="' + value + '"' + ((data.hardware_encoding || 'auto') === value ? ' selected' : '') + '>' + escapeAttr(hwModes[value]) + '</option>').join('') + '</select>');
    html += vdCfgField(prefix + 'hardware_device', '', '<input class="field-input" type="text" placeholder="/dev/dri/renderD128" value="' + escapeAttr(data.hardware_device || '') + '" data-path="video_studio.hardware_device">');
    html += '</div><div class="field-group"><button type="button" class="btn-save dc-test-btn" onclick="vdCfgTestVideoStudio(this)">' + t(prefix + 'check') + '</button><span id="vd-video-studio-result" class="dc-test-result" role="status"></span></div>';
    return html;
}

async function vdCfgTestVideoStudio(button) {
    const result = document.getElementById('vd-video-studio-result');
    const prefix = 'config.video_studio.';
    if (hasUnsavedConfigChanges()) { result.textContent = t(prefix + 'save_first'); return; }
    button.disabled = true;
    result.textContent = t(prefix + 'checking');
    try {
        // The first check after a configuration change tests up to three hardware encoders (15 s each).
        const response = await fetch('/api/desktop/video-studio/status', { signal: AbortSignal.timeout(60000) });
        const status = await response.json();
        if (!result.isConnected) return;
        const ready = response.ok && status.enabled && status.desktop_enabled && status.ffmpeg_ready;
        let text = t(prefix + (ready ? 'ready' : 'unavailable'));
        if (ready && status.encoder && status.encoder.name) text += ' ' + t(prefix + 'encoder').replace('{{name}}', status.encoder.name + (status.encoder.device ? ' (' + status.encoder.device + ')' : ''));
        result.textContent = text;
        result.className = 'dc-test-result ' + (ready ? 'is-success' : 'is-danger');
    } catch (_) {
        if (result.isConnected) result.textContent = t(prefix + 'unavailable');
    } finally { button.disabled = false; }
}

function vdCfgRetroGames() {
    const data = configData.retro_games || (configData.retro_games = {});
    if (typeof data.enabled !== 'boolean') data.enabled = true;
    if (typeof data.agent_access !== 'boolean') data.agent_access = true;
    const prefix = 'config.retro_games.';
    let html = '<h3>' + t(prefix + 'title') + '</h3><p class="field-help">' + t(prefix + 'help') + '</p>';
    html += '<div class="field-grid two-cols">';
    html += vdCfgToggleRow(prefix + 'enabled', '', data.enabled === true, 'retro_games.enabled');
    html += vdCfgToggleRow(prefix + 'agent_access', '', data.agent_access === true, 'retro_games.agent_access');
    html += '</div><div class="field-group"><button type="button" class="btn-save dc-test-btn" onclick="vdCfgTestRetroGames(this)">' + t(prefix + 'check') + '</button><span id="vd-retro-games-result" class="dc-test-result" role="status"></span></div>';
    return html;
}

async function vdCfgTestRetroGames(button) {
    const result = document.getElementById('vd-retro-games-result');
    const prefix = 'config.retro_games.';
    if (hasUnsavedConfigChanges()) { result.textContent = t(prefix + 'save_first'); return; }
    button.disabled = true;
    result.className = 'dc-test-result';
    result.textContent = t(prefix + 'checking');
    try {
        // passive=1: a plain status request would start the first index download.
        const response = await fetch('/api/desktop/retro-games/status?passive=1', { signal: AbortSignal.timeout(15000) });
        const status = await response.json();
        if (!result.isConnected) return;
        if (!response.ok || !status.enabled) {
            result.textContent = t(prefix + 'unavailable');
            result.className = 'dc-test-result is-danger';
            return;
        }
        const total = Number(status.total_games) || 0;
        if (total > 0) result.textContent = t(prefix + 'status_ready').replace('{count}', total.toLocaleString(document.documentElement.lang || undefined));
        else result.textContent = t(prefix + (status.syncing ? 'status_building' : 'status_empty'));
        result.className = 'dc-test-result is-success';
    } catch (_) {
        if (result.isConnected) {
            result.textContent = t(prefix + 'unavailable');
            result.className = 'dc-test-result is-danger';
        }
    } finally {
        button.disabled = false;
    }
}
