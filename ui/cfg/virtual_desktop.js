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
    if (!data.control_level) data.control_level = 'confirm_destructive';
    if (!data.max_ws_clients) data.max_ws_clients = 8;
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
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += vdCfgToggleRow('config.virtual_desktop.enabled_label', 'help.virtual_desktop.enabled', enabled, 'virtual_desktop.enabled', "vdCfgToggleEnabled(this.classList.contains('on'))");
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
    html += vdCfgToggleRow('config.virtual_desktop.agent_control_label', 'help.virtual_desktop.allow_agent_control', data.allow_agent_control === true, 'virtual_desktop.allow_agent_control', "vdCfgToggleAgentControl(this.classList.contains('on'))");
    html += vdCfgToggleRow('config.virtual_desktop.generated_apps_label', 'help.virtual_desktop.allow_generated_apps', data.allow_generated_apps !== false, 'virtual_desktop.allow_generated_apps');
    html += vdCfgToggleRow('config.virtual_desktop.python_jobs_label', 'help.virtual_desktop.allow_python_jobs', data.allow_python_jobs === true, 'virtual_desktop.allow_python_jobs');
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
    html += vdCfgField('config.virtual_desktop.control_level_label', 'help.virtual_desktop.control_level',
        '<select class="field-input" data-path="virtual_desktop.control_level">' +
        '<option value="confirm_destructive"' + ((data.control_level || 'confirm_destructive') === 'confirm_destructive' ? ' selected' : '') + '>' + t('config.virtual_desktop.control_confirm') + '</option>' +
        '<option value="trusted"' + (data.control_level === 'trusted' ? ' selected' : '') + '>' + t('config.virtual_desktop.control_trusted') + '</option>' +
        '</select>');
    html += vdCfgField('config.virtual_desktop.max_ws_clients_label', 'help.virtual_desktop.max_ws_clients',
        '<input class="field-input" type="number" min="1" max="64" value="' + (data.max_ws_clients || 8) + '" data-path="virtual_desktop.max_ws_clients">');
    html += '</div>';

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
    return '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" style="display:none" aria-hidden="true"></div>';
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

function vdCfgToggleEnabled(isOn) {
    const data = vdCfgEnsureData();
    data.enabled = !isOn;
    setDirty(true);
    renderVirtualDesktopSection(null);
}

function vdCfgToggleAgentControl(isOn) {
    const data = vdCfgEnsureData();
    data.allow_agent_control = !isOn;
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
