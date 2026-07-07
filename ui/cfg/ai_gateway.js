// cfg/ai_gateway.js — AI Gateway section module

let _aiGatewaySection = null;

function aiGatewayText(key, fallback) {
    if (typeof t === 'function') {
        const value = t(key);
        if (typeof value === 'string' && value.trim() !== '' && value !== key) return value;
    }
    return fallback || '';
}

function aiGatewayEnsureData() {
    if (!configData.ai_gateway) configData.ai_gateway = {};
    const gw = configData.ai_gateway;
    if (!gw.mode) gw.mode = 'auto';
    if (!gw.log_mode) gw.log_mode = 'metadata_only';
    if (!gw.metadata || typeof gw.metadata !== 'object' || Array.isArray(gw.metadata)) gw.metadata = {};
    if (typeof gw.request_timeout_ms !== 'number') gw.request_timeout_ms = 0;
    if (typeof gw.max_attempts !== 'number') gw.max_attempts = 0;
    if (typeof gw.retry_delay_ms !== 'number') gw.retry_delay_ms = 0;
    if (typeof gw.backoff !== 'string') gw.backoff = '';
    return gw;
}

function renderAIGatewaySection(section) {
    if (section) {
        _aiGatewaySection = section;
    } else {
        section = _aiGatewaySection || { label: aiGatewayText('config.ai_gateway.title', 'AI Gateway'), desc: '' };
    }
    const gw = aiGatewayEnsureData();
    const enabled = gw.enabled === true;
    const tokenPlaceholder = cfgSecretPlaceholder(gw.token, t('config.ai_gateway.token_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
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

    html += aiGatewaySelectField('config.ai_gateway.mode', 'help.ai_gateway.mode', 'ai_gateway.mode', gw.mode || 'auto', [
        { value: 'auto', label: t('config.ai_gateway.mode_auto') },
        { value: 'openai_compatible', label: t('config.ai_gateway.mode_openai_compatible') },
        { value: 'provider_native', label: t('config.ai_gateway.mode_provider_native') }
    ]);

    html += aiGatewaySelectField('config.ai_gateway.log_mode', 'help.ai_gateway.log_mode', 'ai_gateway.log_mode', gw.log_mode || 'metadata_only', [
        { value: 'metadata_only', label: t('config.ai_gateway.log_mode_metadata_only') },
        { value: 'off', label: t('config.ai_gateway.log_mode_off') },
        { value: 'full', label: t('config.ai_gateway.log_mode_full') }
    ]);

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

    html += '<div class="field-grid two-cols">';
    html += aiGatewayNumberField('config.ai_gateway.request_timeout_ms', 'help.ai_gateway.request_timeout_ms', 'ai_gateway.request_timeout_ms', gw.request_timeout_ms || 0, 0, 300000);
    html += aiGatewayNumberField('config.ai_gateway.max_attempts', 'help.ai_gateway.max_attempts', 'ai_gateway.max_attempts', gw.max_attempts || 0, 0, 5);
    html += aiGatewayNumberField('config.ai_gateway.retry_delay_ms', 'help.ai_gateway.retry_delay_ms', 'ai_gateway.retry_delay_ms', gw.retry_delay_ms || 0, 0, 5000);
    html += aiGatewaySelectField('config.ai_gateway.backoff', 'help.ai_gateway.backoff', 'ai_gateway.backoff', gw.backoff || '', [
        { value: '', label: aiGatewayText('config.common.default', 'Default') },
        { value: 'constant', label: t('config.ai_gateway.backoff_constant') },
        { value: 'linear', label: t('config.ai_gateway.backoff_linear') },
        { value: 'exponential', label: t('config.ai_gateway.backoff_exponential') }
    ]);
    html += '</div>';

    html += aiGatewayRenderMetadataRows(gw.metadata);
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

function aiGatewayField(labelKey, helpKey, inputHtml) {
    let html = '<div class="field-group">';
    html += '<div class="field-label">' + aiGatewayText(labelKey) + '</div>';
    const help = aiGatewayText(helpKey);
    if (help) html += '<div class="field-help">' + help + '</div>';
    html += inputHtml;
    html += '</div>';
    return html;
}

function aiGatewaySelectField(labelKey, helpKey, path, value, options) {
    let html = '<select class="field-select" data-path="' + escapeAttr(path) + '">';
    const normalized = String(value ?? '');
    options.forEach(opt => {
        const optValue = String(opt.value ?? '');
        html += '<option value="' + escapeAttr(optValue) + '"' + (normalized === optValue ? ' selected' : '') + '>' + escapeHtml(opt.label || optValue) + '</option>';
    });
    html += '</select>';
    return aiGatewayField(labelKey, helpKey, html);
}

function aiGatewayNumberField(labelKey, helpKey, path, value, min, max) {
    return aiGatewayField(labelKey, helpKey,
        '<input class="field-input" type="number" min="' + min + '" max="' + max + '" step="1" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(String(value || 0)) + '">');
}

function aiGatewayRenderMetadataRows(metadata) {
    const entries = Object.entries(metadata || {})
        .filter(([key, value]) => String(key).trim() !== '' && String(value).trim() !== '')
        .slice(0, 5);
    let html = '<div class="field-group ai-gateway-metadata" data-ai-gateway-metadata>';
    html += '<div class="field-label">' + t('config.ai_gateway.metadata') + '</div>';
    html += '<div class="field-help">' + t('help.ai_gateway.metadata') + '</div>';
    html += '<input type="hidden" data-path="ai_gateway.metadata" data-type="json" data-ai-gateway-metadata-store value="' + escapeAttr(JSON.stringify(Object.fromEntries(entries))) + '">';
    entries.forEach(([key, value]) => {
        html += '<div class="field-grid two-cols ai-gateway-metadata-row">';
        html += '<input class="field-input" type="text" value="' + escapeAttr(key) + '" placeholder="' + escapeAttr(t('config.ai_gateway.metadata_key')) + '" data-ai-gateway-metadata-key oninput="aiGatewayCollectMetadata()">';
        html += '<div class="password-wrap">';
        html += '<input class="field-input" type="text" value="' + escapeAttr(value) + '" placeholder="' + escapeAttr(t('config.ai_gateway.metadata_value')) + '" data-ai-gateway-metadata-value oninput="aiGatewayCollectMetadata()">';
        html += '<button type="button" class="password-toggle" data-ai-gateway-metadata-remove="' + escapeAttr(key) + '" onclick="aiGatewayRemoveMetadata(this.dataset.aiGatewayMetadataRemove)">&times;</button>';
        html += '</div></div>';
    });
    html += '<div class="field-grid two-cols ai-gateway-metadata-row">';
    html += '<input class="field-input" type="text" value="" placeholder="' + escapeAttr(t('config.ai_gateway.metadata_key')) + '" data-ai-gateway-metadata-key oninput="aiGatewayCollectMetadata()">';
    html += '<input class="field-input" type="text" value="" placeholder="' + escapeAttr(t('config.ai_gateway.metadata_value')) + '" data-ai-gateway-metadata-value oninput="aiGatewayCollectMetadata()">';
    html += '</div>';
    html += '<button type="button" class="btn-save btn-secondary" onclick="aiGatewayAddMetadataRow()">' + t('config.ai_gateway.metadata_add') + '</button>';
    html += '</div>';
    return html;
}

function aiGatewayCollectMetadata(mark = true) {
    const gw = aiGatewayEnsureData();
    const next = {};
    document.querySelectorAll('.ai-gateway-metadata-row').forEach(row => {
        if (Object.keys(next).length >= 5) return;
        const keyEl = row.querySelector('[data-ai-gateway-metadata-key]');
        const valueEl = row.querySelector('[data-ai-gateway-metadata-value]');
        if (!keyEl || !valueEl) return;
        const key = String(keyEl.value || '').trim();
        const value = String(valueEl.value || '').trim();
        if (key && value) next[key] = value;
    });
    gw.metadata = next;
    const store = document.querySelector('[data-ai-gateway-metadata-store]');
    if (store) store.value = JSON.stringify(next);
    if (mark) markDirty();
}

function aiGatewayAddMetadataRow() {
    aiGatewayCollectMetadata();
    renderAIGatewaySection(null);
}

function aiGatewayRemoveMetadata(key) {
    const gw = aiGatewayEnsureData();
    delete gw.metadata[String(key || '').trim()];
    markDirty();
    renderAIGatewaySection(null);
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
            if (res.status === 'unsupported_provider') {
                aiGatewaySetBanner('warning', '🟡 ' + t('config.ai_gateway.status_unsupported'));
                return;
            }
            if (res.live_status === 'ok') {
                aiGatewaySetBanner('success', '🟢 ' + t('config.ai_gateway.status_live_ok'));
                return;
            }
            if (res.live_status === 'failed') {
                aiGatewaySetBanner('danger', '🔴 ' + t('config.ai_gateway.status_live_failed'));
                return;
            }
            if (res.status === 'configured') {
                aiGatewaySetBanner('success', '🟢 ' + t('config.ai_gateway.status_configured'));
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

    fetch('/api/ai-gateway/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({})
    })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.ai_gateway.status_success') + ' ' + t('config.ai_gateway.status_live_ok');
                aiGatewayCheckStatus();
            } else if (res.live_status === 'skipped' && res.status === 'configured') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.ai_gateway.status_success') + ' ' + t('config.ai_gateway.status_configured');
                aiGatewayCheckStatus();
            } else if (res.status === 'unsupported_provider') {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.ai_gateway.status_error') + ' ' + t('config.ai_gateway.status_unsupported');
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
