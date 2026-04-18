// cfg/ai_gateway.js — AI Gateway section module

function renderAIGatewaySection(section) {
    const gw = configData.ai_gateway || {};
    const enabled = gw.enabled === true;
    const tokenPlaceholder = cfgSecretPlaceholder(gw.token, t('config.ai_gateway.token_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div class="wh-notice ai-gw-info-notice">';
    html += '<span>🌩️</span>';
    html += '<div><small>' + t('config.ai_gateway.info') + '</small></div>';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ai_gateway.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('help.ai_gateway.enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabled ? ' on' : '') + '" data-path="ai_gateway.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    if (!enabled) {
        html += '<div class="wh-notice">';
        html += '<span>☁️</span>';
        html += '<div>';
        html += '<strong>' + t('config.ai_gateway.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.ai_gateway.disabled_desc') + '</small>';
        html += '</div></div>';
    }

    html += '<div class="field-group">';
    html += '<div class="field-group-title">⚙️ ' + t('config.ai_gateway.settings_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.ai_gateway.settings_desc') + '</div>';

    html += '<div class="ai-gw-grid">';

    html += '<label class="ai-gw-label">';
    html += '<span class="ai-gw-label-text">' + t('config.ai_gateway.account_id') + '</span>';
    html += '<span class="field-help">' + t('help.ai_gateway.account_id') + '</span>';
    html += '<input class="cfg-input ai-gw-input-spaced" data-path="ai_gateway.account_id" value="' + escapeAttr(gw.account_id || '') + '" placeholder="' + escapeAttr(t('config.ai_gateway.account_id_placeholder')) + '">';
    html += '</label>';

    html += '<label class="ai-gw-label">';
    html += '<span class="ai-gw-label-text">' + t('config.ai_gateway.gateway_id') + '</span>';
    html += '<span class="field-help">' + t('help.ai_gateway.gateway_id') + '</span>';
    html += '<input class="cfg-input ai-gw-input-spaced" data-path="ai_gateway.gateway_id" value="' + escapeAttr(gw.gateway_id || '') + '" placeholder="' + escapeAttr(t('config.ai_gateway.gateway_id_placeholder')) + '">';
    html += '</label>';

    html += '<label class="ai-gw-label">';
    html += '<span class="ai-gw-label-text">' + t('config.ai_gateway.token') + '</span>';
    html += '<span class="field-help">' + t('help.ai_gateway.token') + '</span>';
    html += '<div class="password-wrap">';
    html += '<input class="cfg-input ai-gw-input-spaced" type="password" data-path="ai_gateway.token" value="' + escapeAttr(cfgSecretValue(gw.token)) + '" placeholder="' + escapeAttr(tokenPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '</label>';

    html += '</div>';
    html += '</div>';
    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}
