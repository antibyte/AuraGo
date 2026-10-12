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
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + escapeHtml(t(labelKey)) + '</div>';
    const help = helpKey ? t(helpKey) : '';
    if (help && help !== helpKey) html += '<div class="field-help">' + escapeHtml(help) + '</div>';
    html += '<div class="toggle' + (on ? ' on' : '') + '" role="switch" tabindex="0" aria-checked="' + (on ? 'true' : 'false') + '" aria-label="' + escapeAttr(t(labelKey)) + '" data-path="' + path + '" onclick="' + (handler || "ccCfgToggleValue(this, '" + path + "')") + '"></div>';
    html += '</div>';
    return html;
}

function ccCfgNumber(labelKey, helpKey, path, value, min, max, step) {
    let html = '<div class="field-group"><div class="field-label">' + escapeHtml(t(labelKey)) + '</div>';
    const help = helpKey ? t(helpKey) : '';
    if (help && help !== helpKey) html += '<div class="field-help">' + escapeHtml(help) + '</div>';
    html += '<input class="field-input" type="number" min="' + min + '" max="' + max + '"' + (step ? ' step="' + step + '"' : '') + ' value="' + escapeAttr(String(value)) + '" data-path="' + path + '" aria-label="' + escapeAttr(t(labelKey)) + '">';
    return html + '</div>';
}

function ccCfgText(labelKey, helpKey, path, value, placeholder) {
    let html = '<div class="field-group"><div class="field-label">' + escapeHtml(t(labelKey)) + '</div>';
    const help = helpKey ? t(helpKey) : '';
    if (help && help !== helpKey) html += '<div class="field-help">' + escapeHtml(help) + '</div>';
    html += '<input class="field-input" type="text" value="' + escapeAttr(value || '') + '" placeholder="' + escapeAttr(placeholder || '') + '" data-path="' + path + '" aria-label="' + escapeAttr(t(labelKey)) + '">';
    return html + '</div>';
}

function renderCYDCreatorSection(section) {
    if (section) _cydCreatorSection = section;
    else section = _cydCreatorSection;
    const data = ccCfgEnsureData();
    const enabled = data.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + escapeHtml(section.label) + '</div>';
    html += '<div class="section-desc">' + escapeHtml(section.desc) + '</div>';
    html += ccCfgToggleRow('config.cyd_creator.enabled_label', 'help.cyd_creator.enabled', enabled, 'cyd_creator.enabled', 'ccCfgToggleEnabled(this)');

    if (!enabled) {
        html += '<div class="wh-notice"><span>i</span><div>';
        html += '<strong>' + escapeHtml(t('config.cyd_creator.disabled_notice')) + '</strong><br>';
        html += '<small>' + escapeHtml(t('config.cyd_creator.disabled_desc')) + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += '<div class="cfg-note-banner cfg-note-banner-warning">' + escapeHtml(t('config.cyd_creator.restart_note')) + '</div>';

    html += '<div class="cfg-group-title cfg-group-title-top">' + escapeHtml(t('config.cyd_creator.build_title')) + '</div>';
    html += ccCfgText('config.cyd_creator.builder_image_label', 'help.cyd_creator.builder_image', 'cyd_creator.builder_image', data.builder_image, CYD_CREATOR_DEFAULTS.builder_image);
    html += '<div class="field-grid two-cols">';
    html += ccCfgNumber('config.cyd_creator.memory_mb_label', '', 'cyd_creator.memory_mb', data.memory_mb, 512, 16384, 256);
    html += ccCfgNumber('config.cyd_creator.cpu_cores_label', '', 'cyd_creator.cpu_cores', data.cpu_cores, 0.5, 16, 0.5);
    html += ccCfgNumber('config.cyd_creator.build_timeout_seconds_label', '', 'cyd_creator.build_timeout_seconds', data.build_timeout_seconds, 60, 1800);
    html += ccCfgNumber('config.cyd_creator.cold_build_timeout_seconds_label', 'help.cyd_creator.cold_build_timeout_seconds', 'cyd_creator.cold_build_timeout_seconds', data.cold_build_timeout_seconds, 60, 3600);
    html += ccCfgNumber('config.cyd_creator.idle_stop_minutes_label', '', 'cyd_creator.idle_stop_minutes', data.idle_stop_minutes, 1, 1440);
    html += ccCfgNumber('config.cyd_creator.keep_builds_per_project_label', '', 'cyd_creator.keep_builds_per_project', data.keep_builds_per_project, 1, 50);
    html += '</div>';

    html += '<div class="cfg-group-title cfg-group-title-top">' + escapeHtml(t('config.cyd_creator.agent_title')) + '</div>';
    html += '<div class="field-group"><div class="field-label">' + escapeHtml(t('config.cyd_creator.model_label')) + '</div>'
        + '<div class="field-help">' + escapeHtml(t('config.cyd_creator.model_help')) + '</div>'
        + '<select class="field-select" data-path="cyd_creator.model" data-config-choice="providers" aria-label="' + escapeAttr(t('config.cyd_creator.model_label')) + '">'
        + cfgChoiceOptionsHTML('providers', helpTexts['cyd_creator.model'] || {}, data.model || '') + '</select>' + cfgChoiceHintHTML('providers') + '</div>';
    html += '<div class="field-grid two-cols">';
    html += ccCfgNumber('config.cyd_creator.job_timeout_minutes_label', '', 'cyd_creator.job_timeout_minutes', data.job_timeout_minutes, 5, 120);
    html += ccCfgNumber('config.cyd_creator.max_repair_attempts_label', 'help.cyd_creator.max_repair_attempts', 'cyd_creator.max_repair_attempts', data.max_repair_attempts, 0, 10);
    html += '</div>';

    html += '<div class="cfg-group-title cfg-group-title-top">' + escapeHtml(t('config.cyd_creator.ota_title')) + '</div>';
    html += ccCfgToggleRow('config.cyd_creator.ota_enabled_label', 'help.cyd_creator.ota_enabled', data.ota.enabled === true, 'cyd_creator.ota.enabled');
    html += '<div class="field-grid two-cols">';
    html += ccCfgNumber('config.cyd_creator.ota_heartbeat_seconds_label', '', 'cyd_creator.ota.heartbeat_seconds', data.ota.heartbeat_seconds, 30, 600);
    html += ccCfgText('config.cyd_creator.ota_public_url_label', 'help.cyd_creator.ota_public_url', 'cyd_creator.ota.public_url', data.ota.public_url, 'https://aurago.example');
    html += '</div>';

    html += '<div class="field-group"><a class="btn-save dc-test-btn" href="/desktop?app=cyd-creator">' + escapeHtml(t('config.cyd_creator.open_app')) + '</a></div>';
    html += '</div>';

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function ccCfgToggleEnabled(el) {
    const next = !(el && el.classList && el.classList.contains('on'));
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
