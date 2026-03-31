// cfg/mission_preparation.js — Mission Preparation configuration section module

function renderMissionPreparationSection(section) {
    const data = configData['mission_preparation'] || {};
    const enabledOn = data.enabled === true;
    const autoScheduledOn = data.auto_prepare_scheduled !== false;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.mission_preparation.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.mission_preparation.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="mission_preparation.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Provider override ──
    const helpProvider = t('help.mission_preparation.provider');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.mission_preparation.provider_label') + '</div>';
    if (helpProvider) html += '<div class="field-help">' + helpProvider + '</div>';
    html += '<input class="field-input" type="text" data-path="mission_preparation.provider" value="' + escapeAttr(data.provider || '') + '" placeholder="' + t('config.mission_preparation.provider_placeholder') + '">';
    html += '</div>';

    // ── Timeout ──
    const helpTimeout = t('help.mission_preparation.timeout_seconds');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.mission_preparation.timeout_seconds_label') + '</div>';
    if (helpTimeout) html += '<div class="field-help">' + helpTimeout + '</div>';
    html += '<input class="field-input" type="number" min="30" max="600" data-path="mission_preparation.timeout_seconds" value="' + escapeAttr(data.timeout_seconds || 120) + '">';
    html += '</div>';

    // ── Max Essential Tools ──
    const helpTools = t('help.mission_preparation.max_essential_tools');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.mission_preparation.max_essential_tools_label') + '</div>';
    if (helpTools) html += '<div class="field-help">' + helpTools + '</div>';
    html += '<input class="field-input" type="number" min="1" max="20" data-path="mission_preparation.max_essential_tools" value="' + escapeAttr(data.max_essential_tools || 5) + '">';
    html += '</div>';

    // ── Cache Expiry Hours ──
    const helpCache = t('help.mission_preparation.cache_expiry_hours');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.mission_preparation.cache_expiry_hours_label') + '</div>';
    if (helpCache) html += '<div class="field-help">' + helpCache + '</div>';
    html += '<input class="field-input" type="number" min="1" max="720" data-path="mission_preparation.cache_expiry_hours" value="' + escapeAttr(data.cache_expiry_hours || 24) + '">';
    html += '</div>';

    // ── Min Confidence ──
    const helpConf = t('help.mission_preparation.min_confidence');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.mission_preparation.min_confidence_label') + '</div>';
    if (helpConf) html += '<div class="field-help">' + helpConf + '</div>';
    html += '<input class="field-input" type="number" min="0" max="1" step="0.1" data-path="mission_preparation.min_confidence" value="' + escapeAttr(data.min_confidence || 0.5) + '">';
    html += '</div>';

    // ── Auto-Prepare Scheduled toggle ──
    const helpAuto = t('help.mission_preparation.auto_prepare_scheduled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.mission_preparation.auto_prepare_scheduled_label') + '</div>';
    if (helpAuto) html += '<div class="field-help">' + helpAuto + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (autoScheduledOn ? ' on' : '') + '" data-path="mission_preparation.auto_prepare_scheduled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (autoScheduledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
}
