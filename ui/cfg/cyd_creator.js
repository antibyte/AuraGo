// cfg/cyd_creator.js — CYD Creator configuration section: the switch, the build container, the agent
// (its LLM provider through the shared provider dropdown, lang/meta.json help.cyd_creator.model) and
// updates over Wi-Fi. Defaults follow the spec (§15); the server validates the ranges.

let _cydCreatorSection = null;

const CYD_CREATOR_DEFAULTS = Object.freeze({
    enabled: false,
    builder_image: 'ghcr.io/antibyte/aurago-cyd-builder:1',
    build_timeout_seconds: 300,
    cold_build_timeout_seconds: 600,
    memory_mb: 2048,
    cpu_cores: 2,
    idle_stop_minutes: 20,
    max_repair_attempts: 3,
    job_timeout_minutes: 20,
    model: '',
    keep_builds_per_project: 5
});

const CYD_CREATOR_OTA_DEFAULTS = Object.freeze({ enabled: true, heartbeat_seconds: 60, public_url: '' });

function ccCfgFill(target, defaults) {
    Object.keys(defaults).forEach(key => {
        const def = defaults[key];
        if (typeof def === 'boolean' && typeof target[key] !== 'boolean') target[key] = def;
        else if (typeof def === 'number' && (typeof target[key] !== 'number' || !Number.isFinite(target[key]))) target[key] = def;
        else if (typeof def === 'string' && typeof target[key] !== 'string') target[key] = def;
    });
}

function ccCfgEnsureData() {
    if (!configData.cyd_creator || typeof configData.cyd_creator !== 'object') configData.cyd_creator = {};
    const data = configData.cyd_creator;
    ccCfgFill(data, CYD_CREATOR_DEFAULTS);
    if (!data.ota || typeof data.ota !== 'object') data.ota = {};
    ccCfgFill(data.ota, CYD_CREATOR_OTA_DEFAULTS);
    return data;
}

function ccCfgToggleRow(labelKey, helpKey, on, path, handler) {
    return window.AuraConfigForm.toggle({ label: t(labelKey), help: helpKey ? tOr(helpKey, '') : '', path, value: on, onclick: handler || "ccCfgToggleValue(this, '" + path + "')" });
}

function ccCfgNumber(labelKey, helpKey, path, value, min, max, step) {
    return window.AuraConfigForm.number({ label: t(labelKey), help: helpKey ? tOr(helpKey, '') : '', path, value, min, max, step });
}

function ccCfgText(labelKey, helpKey, path, value, placeholder, type) {
    return window.AuraConfigForm.field({ type: type || 'text', label: t(labelKey), help: helpKey ? tOr(helpKey, '') : '', path, value: value || '', placeholder: placeholder || '' });
}

function renderCYDCreatorSection(section) {
    if (section) _cydCreatorSection = section;
    else section = _cydCreatorSection;
    const data = ccCfgEnsureData();
    const enabled = data.enabled === true;
    const form = window.AuraConfigForm;

    const groups = [{ titleKey: 'config.refresh.operation', fields: [
        ccCfgToggleRow('config.cyd_creator.enabled_label', 'help.cyd_creator.enabled', enabled, 'cyd_creator.enabled', 'ccCfgToggleEnabled(this)')
    ] }];
    let before = '';
    let after = '';

    if (!enabled) {
        after = form.emptyState({ title: t('config.cyd_creator.disabled_notice'), desc: t('config.cyd_creator.disabled_desc') });
    } else {
        before = '<div class="cfg-note-banner cfg-note-banner-warning">' + escapeHtml(t('config.cyd_creator.restart_note')) + '</div>';
        groups.push({ title: t('config.cyd_creator.build_title'), fields: [
            ccCfgText('config.cyd_creator.builder_image_label', 'help.cyd_creator.builder_image', 'cyd_creator.builder_image', data.builder_image, CYD_CREATOR_DEFAULTS.builder_image),
            ccCfgNumber('config.cyd_creator.memory_mb_label', 'config.cyd_creator.memory_mb_help', 'cyd_creator.memory_mb', data.memory_mb, 512, 16384, 256),
            ccCfgNumber('config.cyd_creator.cpu_cores_label', 'config.cyd_creator.cpu_cores_help', 'cyd_creator.cpu_cores', data.cpu_cores, 0.5, 16, 0.5),
            ccCfgNumber('config.cyd_creator.build_timeout_seconds_label', 'config.cyd_creator.build_timeout_seconds_help', 'cyd_creator.build_timeout_seconds', data.build_timeout_seconds, 60, 1800),
            ccCfgNumber('config.cyd_creator.cold_build_timeout_seconds_label', 'help.cyd_creator.cold_build_timeout_seconds', 'cyd_creator.cold_build_timeout_seconds', data.cold_build_timeout_seconds, 60, 3600),
            ccCfgNumber('config.cyd_creator.idle_stop_minutes_label', 'config.cyd_creator.idle_stop_minutes_help', 'cyd_creator.idle_stop_minutes', data.idle_stop_minutes, 1, 1440),
            ccCfgNumber('config.cyd_creator.keep_builds_per_project_label', 'config.cyd_creator.keep_builds_per_project_help', 'cyd_creator.keep_builds_per_project', data.keep_builds_per_project, 1, 50)
        ] });
        groups.push({ title: t('config.cyd_creator.agent_title'), fields: [
            '<div class="field-group"><div class="field-label">' + escapeHtml(t('config.cyd_creator.model_label')) + '</div>'
            + '<div class="field-help">' + escapeHtml(t('config.cyd_creator.model_help')) + '</div>'
            + '<select class="field-select" data-path="cyd_creator.model" data-config-choice="providers" aria-label="' + escapeAttr(t('config.cyd_creator.model_label')) + '">'
            + cfgChoiceOptionsHTML('providers', helpTexts['cyd_creator.model'] || {}, data.model || '') + '</select>' + cfgChoiceHintHTML('providers') + '</div>',
            ccCfgNumber('config.cyd_creator.job_timeout_minutes_label', 'config.cyd_creator.job_timeout_minutes_help', 'cyd_creator.job_timeout_minutes', data.job_timeout_minutes, 5, 120),
            ccCfgNumber('config.cyd_creator.max_repair_attempts_label', 'help.cyd_creator.max_repair_attempts', 'cyd_creator.max_repair_attempts', data.max_repair_attempts, 0, 10)
        ] });
        groups.push({ title: t('config.cyd_creator.ota_title'), fields: [
            ccCfgToggleRow('config.cyd_creator.ota_enabled_label', 'help.cyd_creator.ota_enabled', data.ota.enabled === true, 'cyd_creator.ota.enabled'),
            ccCfgNumber('config.cyd_creator.ota_heartbeat_seconds_label', 'config.cyd_creator.ota_heartbeat_seconds_help', 'cyd_creator.ota.heartbeat_seconds', data.ota.heartbeat_seconds, 30, 600),
            ccCfgText('config.cyd_creator.ota_public_url_label', 'help.cyd_creator.ota_public_url', 'cyd_creator.ota.public_url', data.ota.public_url, 'https://aurago.example', 'url')
        ] });
        after = '<div class="field-group"><a class="btn-save dc-test-btn" href="/desktop?app=cyd-creator">' + escapeHtml(t('config.cyd_creator.open_app')) + '</a></div>';
    }

    document.getElementById('content').innerHTML = form.renderSpec({ label: section.label, desc: section.desc, beforeHTML: before, groups, afterHTML: after });
    attachChangeListeners();
}

function ccCfgToggleEnabled(el) {
    const next = !(el && el.classList && el.classList.contains('on'));
    // The re-render below detaches this switch, but the click still bubbles on to the
    // AuraConfigState document listener, which reads the clicked element. Give the old switch the
    // new value first, so that listener cannot write the previous value back into the draft.
    if (el && el.classList) {
        el.classList.toggle('on', next);
        el.setAttribute('aria-checked', next ? 'true' : 'false');
    }
    const data = ccCfgEnsureData();
    data.enabled = next;
    setNestedValue(configData, 'cyd_creator.enabled', next);
    if (window.AuraConfigState) window.AuraConfigState.set('cyd_creator.enabled', next);
    renderCYDCreatorSection(null);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}

function ccCfgToggleValue(el, path) {
    toggleBool(el);
    const value = Boolean(el && el.classList && el.classList.contains('on'));
    if (el) el.setAttribute('aria-checked', value ? 'true' : 'false');
    setNestedValue(configData, path, value);
    if (window.AuraConfigState) window.AuraConfigState.set(path, value);
    setDirty(window.AuraConfigState ? window.AuraConfigState.isDirty() : true);
}
