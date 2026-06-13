// cfg/ai_gateway.js — AI Gateway section module

function renderAIGatewaySection(section) {
    const gw = configData.ai_gateway || {};
    const enabled = gw.enabled === true;
    const tokenPlaceholder = cfgSecretPlaceholder(gw.token, t('config.ai_gateway.token_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div id="ai-gw-status-banner" class="adg-status-banner">' + t('config.ai_gateway.checking') + '</div>';

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

    html += '<div class="field-grid two-cols">';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ai_gateway.account_id') + '</div>';
    html += '<div class="field-help">' + t('help.ai_gateway.account_id') + '</div>';
    html += '<input class="field-input" data-path="ai_gateway.account_id" value="' + escapeAttr(gw.account_id || '') + '" placeholder="' + escapeAttr(t('config.ai_gateway.account_id_placeholder')) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ai_gateway.gateway_id') + '</div>';
    html += '<div class="field-help">' + t('help.ai_gateway.gateway_id') + '</div>';
    html += '<input class="field-input" data-path="ai_gateway.gateway_id" value="' + escapeAttr(gw.gateway_id || '') + '" placeholder="' + escapeAttr(t('config.ai_gateway.gateway_id_placeholder')) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.ai_gateway.token') + '</div>';
    html += '<div class="field-help">' + t('help.ai_gateway.token') + '</div>';
    html += '<div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" data-path="ai_gateway.token" value="' + escapeAttr(cfgSecretValue(gw.token)) + '" placeholder="' + escapeAttr(tokenPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '</div>';

    html += '</div>';
    html += '</div>';

    html += '<div class="cfg-actions-row">';
    html += '<button class="btn-save adg-test-btn" onclick="aiGatewayTestConnection()" id="ai-gw-test-btn">🔌 ' + t('config.ai_gateway.test_btn') + '</button>';
    html += '<span id="ai-gw-test-result" class="adg-test-result"></span>';
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    if (enabled) {
        aiGatewayCheckStatus();
    } else {
        aiGatewaySetBanner('', '⚪ ' + t('config.ai_gateway.status_disabled'));
    }
}

function aiGatewaySetBanner(state, text) {
    const banner = document.getElementById('ai-gw-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner' + (state ? ' is-' + state : '');
    banner.textContent = text;
}

function aiGatewayCheckStatus() {
    aiGatewaySetBanner('', t('config.ai_gateway.checking'));
    fetch('/api/ai-gateway/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                aiGatewaySetBanner('', '⚪ ' + t('config.ai_gateway.status_disabled'));
                return;
            }
            if (res.status === 'no_credentials') {
                aiGatewaySetBanner('warning', '🟡 ' + t('config.ai_gateway.status_no_credentials'));
                return;
            }
            if (res.status === 'ok') {
                aiGatewaySetBanner('success', '🟢 ' + t('config.ai_gateway.status_ok'));
                return;
            }
            aiGatewaySetBanner('danger', '🔴 ' + (res.message || t('config.ai_gateway.connection_failed')));
        })
        .catch(() => aiGatewaySetBanner('danger', '🔴 ' + t('config.ai_gateway.connection_failed')));
}

function aiGatewayTestConnection() {
    const btn = document.getElementById('ai-gw-test-btn');
    const result = document.getElementById('ai-gw-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.ai_gateway.loading');
        result.className = 'adg-test-result';
    }

    fetch('/api/ai-gateway/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.ai_gateway.status_success') + ' ' + t('config.ai_gateway.test_ok');
                aiGatewayCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.ai_gateway.status_error') + ' ' + (res.message || t('config.ai_gateway.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.ai_gateway.status_error') + ' ' + t('config.ai_gateway.test_fail');
            }
        });
}
