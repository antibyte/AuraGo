// cfg/game_maker.js — Game Maker Studio configuration section

let _gameMakerSection = null;

function gmCfgEnsureData() {
    if (!configData.game_maker) configData.game_maker = {};
    const data = configData.game_maker;
    if (typeof data.enabled !== 'boolean') data.enabled = false;
    if (typeof data.readonly !== 'boolean') data.readonly = true;
    if (typeof data.allow_create !== 'boolean') data.allow_create = false;
    if (typeof data.allow_edit !== 'boolean') data.allow_edit = false;
    if (typeof data.allow_delete !== 'boolean') data.allow_delete = false;
    if (typeof data.allow_media_generation !== 'boolean') data.allow_media_generation = false;
    if (!data.workspace_path) data.workspace_path = 'agent_workspace/virtual_desktop';
    if (!data.max_projects) data.max_projects = 25;
    if (!data.max_files_per_project) data.max_files_per_project = 250;
    if (!data.max_file_size_kb) data.max_file_size_kb = 2048;
    if (!data.max_asset_size_mb) data.max_asset_size_mb = 32;
    if (!data.max_project_size_mb) data.max_project_size_mb = 100;
    if (!data.job_timeout_seconds) data.job_timeout_seconds = 1800;
    return data;
}

function renderGameMakerSection(section) {
    if (section) _gameMakerSection = section;
    else section = _gameMakerSection;
    const data = gmCfgEnsureData();
    const enabled = data.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += gmCfgToggleRow(
        'config.game_maker.enabled_label',
        'help.game_maker.enabled',
        enabled,
        'game_maker.enabled',
        "gmCfgToggleEnabled(this)"
    );

    if (!enabled) {
        html += '<div class="wh-notice"><span>🎮</span><div>';
        html += '<strong>' + t('config.game_maker.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.game_maker.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-warning">↻ ' + t('config.game_maker.restart_note') + '</div>';
    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.game_maker.permissions_title') + '</div>';
    html += '<div class="section-desc">' + t('config.game_maker.permissions_desc') + '</div>';
    html += gmCfgToggleRow('config.game_maker.readonly_label', 'help.game_maker.readonly', data.readonly === true, 'game_maker.readonly');
    html += '<div class="field-grid two-cols">';
    html += gmCfgToggleRow('config.game_maker.allow_create_label', 'help.game_maker.allow_create', data.allow_create === true, 'game_maker.allow_create');
    html += gmCfgToggleRow('config.game_maker.allow_edit_label', 'help.game_maker.allow_edit', data.allow_edit === true, 'game_maker.allow_edit');
    html += gmCfgToggleRow('config.game_maker.allow_delete_label', 'help.game_maker.allow_delete', data.allow_delete === true, 'game_maker.allow_delete');
    html += gmCfgToggleRow('config.game_maker.allow_media_generation_label', 'help.game_maker.allow_media_generation', data.allow_media_generation === true, 'game_maker.allow_media_generation');
    html += '</div>';

    html += '<div class="cfg-group-title cfg-group-title-top">' + t('config.game_maker.storage_title') + '</div>';
    html += gmCfgField('config.game_maker.workspace_path_label',
        '<input class="field-input" type="text" value="' + escapeAttr(data.workspace_path) + '" data-path="game_maker.workspace_path">');

    html += '<details class="cfg-advanced-panel">';
    html += '<summary class="cfg-advanced-summary">⚙️ ' + t('config.game_maker.limits_title') + '</summary>';
    html += '<div class="cfg-advanced-body"><div class="field-grid two-cols">';
    html += gmCfgField('config.game_maker.max_projects_label',
        '<input class="field-input" type="number" min="1" max="1000" value="' + data.max_projects + '" data-path="game_maker.max_projects">');
    html += gmCfgField('config.game_maker.max_files_per_project_label',
        '<input class="field-input" type="number" min="10" max="10000" value="' + data.max_files_per_project + '" data-path="game_maker.max_files_per_project">');
    html += gmCfgField('config.game_maker.max_file_size_kb_label',
        '<input class="field-input" type="number" min="64" max="102400" value="' + data.max_file_size_kb + '" data-path="game_maker.max_file_size_kb">');
    html += gmCfgField('config.game_maker.max_asset_size_mb_label',
        '<input class="field-input" type="number" min="1" max="1024" value="' + data.max_asset_size_mb + '" data-path="game_maker.max_asset_size_mb">');
    html += gmCfgField('config.game_maker.max_project_size_mb_label',
        '<input class="field-input" type="number" min="1" max="10240" value="' + data.max_project_size_mb + '" data-path="game_maker.max_project_size_mb">');
    html += gmCfgField('config.game_maker.job_timeout_seconds_label',
        '<input class="field-input" type="number" min="60" max="86400" value="' + data.job_timeout_seconds + '" data-path="game_maker.job_timeout_seconds">');
    html += '</div></div></details>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function gmCfgToggleRow(labelKey, helpKey, enabled, path, onclick) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + t(labelKey) + '</div>';
    const helpText = t(helpKey);
    if (helpText && helpText !== helpKey) html += '<div class="field-help">' + helpText + '</div>';
    html += '<div class="toggle ' + (enabled ? 'on' : '') + '" data-path="' + path + '" onclick="' + (onclick || "gmCfgToggleValue(this, '" + path + "')") + '"></div>';
    html += '</div>';
    return html;
}

function gmCfgField(labelKey, inputHtml) {
    return '<div class="field-group"><div class="field-label">' + t(labelKey) + '</div>' + inputHtml + '</div>';
}

function gmCfgToggleEnabled(el) {
    const nextEnabled = !(el && el.classList && el.classList.contains('on'));
    const data = gmCfgEnsureData();
    data.enabled = nextEnabled;
    setNestedValue(configData, 'game_maker.enabled', nextEnabled);
    if (window.AuraConfigState) window.AuraConfigState.set('game_maker.enabled', nextEnabled);
    renderGameMakerSection(null);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}

function gmCfgToggleValue(el, path) {
    toggleBool(el);
    const value = Boolean(el && el.classList && el.classList.contains('on'));
    setNestedValue(configData, path, value);
    if (window.AuraConfigState) window.AuraConfigState.set(path, value);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}
