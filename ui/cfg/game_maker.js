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
    const form = window.AuraConfigForm;
    const limit = (name, path, min, max) => form.number({ label: t('config.game_maker.' + name + '_label'), help: tOr('config.game_maker.' + name + '_help', ''), path, value: data[name], min, max });

    const groups = [{ titleKey: 'config.refresh.operation', fields: [
        gmCfgToggleRow('config.game_maker.enabled_label', 'help.game_maker.enabled', enabled, 'game_maker.enabled', "gmCfgToggleEnabled(this)")
    ] }];
    let before = '';
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: t('config.game_maker.disabled_notice'), desc: t('config.game_maker.disabled_desc') });
    } else {
        before = '<div class="cfg-note-banner cfg-note-banner-warning">' + escapeHtml(t('config.game_maker.restart_note')) + '</div>';
        groups.push({ title: t('config.game_maker.permissions_title'), desc: t('config.game_maker.permissions_desc'), fields: [
            gmCfgToggleRow('config.game_maker.readonly_label', 'help.game_maker.readonly', data.readonly === true, 'game_maker.readonly'),
            gmCfgToggleRow('config.game_maker.allow_create_label', 'help.game_maker.allow_create', data.allow_create === true, 'game_maker.allow_create'),
            gmCfgToggleRow('config.game_maker.allow_edit_label', 'help.game_maker.allow_edit', data.allow_edit === true, 'game_maker.allow_edit'),
            gmCfgToggleRow('config.game_maker.allow_delete_label', 'help.game_maker.allow_delete', data.allow_delete === true, 'game_maker.allow_delete'),
            gmCfgToggleRow('config.game_maker.allow_media_generation_label', 'help.game_maker.allow_media_generation', data.allow_media_generation === true, 'game_maker.allow_media_generation')
        ] });
        groups.push({ title: t('config.game_maker.storage_title'),
            fields: [form.field({ label: t('config.game_maker.workspace_path_label'), help: tOr('config.game_maker.workspace_path_help', ''), path: 'game_maker.workspace_path', value: data.workspace_path })],
            content: form.disclosure({ title: t('config.game_maker.limits_title'), content:
                limit('max_projects', 'game_maker.max_projects', 1, 1000) + limit('max_files_per_project', 'game_maker.max_files_per_project', 10, 10000) + limit('max_file_size_kb', 'game_maker.max_file_size_kb', 64, 102400)
                + limit('max_asset_size_mb', 'game_maker.max_asset_size_mb', 1, 1024) + limit('max_project_size_mb', 'game_maker.max_project_size_mb', 1, 10240) + limit('job_timeout_seconds', 'game_maker.job_timeout_seconds', 60, 86400) }) });
    }

    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, beforeHTML: before, groups, afterHTML: after });
    attachChangeListeners();
}

function gmCfgToggleRow(labelKey, helpKey, enabled, path, onclick) {
    return window.AuraConfigForm.toggle({ label: t(labelKey), help: tOr(helpKey, ''), path, value: enabled })
        .replace('onclick="toggleBool(this)"', 'onclick="' + (onclick || "gmCfgToggleValue(this, '" + path + "')") + '"');
}

function gmCfgToggleEnabled(el) {
    const nextEnabled = !(el && el.classList && el.classList.contains('on'));
    // Flip the clicked switch first: state sync still reads it, so a stale one reverts the draft.
    if (el && el.classList) {
        el.classList.toggle('on', nextEnabled);
        el.setAttribute('aria-checked', String(nextEnabled));
    }
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
