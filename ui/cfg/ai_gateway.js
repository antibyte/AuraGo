// AI Gateway configuration and saved-provider diagnostics.
let _aiGatewaySection = null;
let _aiGatewayProviderID = '';
let _aiGatewayEpoch = 0;
let _aiGatewayController = null;

function aiGatewayText(key, fallback = '') {
    const value = typeof t === 'function' ? t(key) : key;
    return typeof value === 'string' && value !== key ? value : fallback;
}

function aiGatewayEnsureData() {
    if (!configData.ai_gateway) configData.ai_gateway = {};
    const gw = configData.ai_gateway;
    if (!gw.log_mode) gw.log_mode = 'metadata_only';
    if (!gw.metadata || typeof gw.metadata !== 'object') gw.metadata = {};
    return gw;
}

function aiGatewayInvalidate() {
    ++_aiGatewayEpoch;
    _aiGatewayController?.abort();
    _aiGatewayController = null;
    const result = document.getElementById('ai-gw-test-result');
    if (result) result.textContent = '';
}

function aiGatewayHasDraft() {
    return typeof hasUnsavedConfigChanges === 'function' ? hasUnsavedConfigChanges() : Boolean(window.AuraConfigState?.isDirty());
}

function renderAIGatewaySection(section) {
    aiGatewayInvalidate();
    if (section) _aiGatewaySection = section;
    section = _aiGatewaySection || { label: t('config.ai_gateway.title'), desc: '' };
    const gw = aiGatewayEnsureData();
    const providers = Array.isArray(configData.providers) ? configData.providers : [];
    if (!providers.some(p => p.id === _aiGatewayProviderID)) _aiGatewayProviderID = providers.some(p => p.id === configData.llm?.provider) ? configData.llm.provider : (providers[0]?.id || '');
    let html = '<div class="cfg-section active"><div class="section-header">' + escapeHtml(section.label) + '</div>';
    html += '<div class="section-desc">' + escapeHtml(section.desc || '') + '</div>';
    html += '<div id="ai-gw-status-banner" class="adg-status-banner" role="status" aria-live="polite"></div>';
    html += '<div class="wh-notice"><small>' + t('config.ai_gateway.info') + '</small></div>';
    html += '<div class="field-group"><div class="field-label">' + t('config.ai_gateway.enabled_label') + '</div><div class="toggle-wrap">';
    html += '<div class="toggle' + (gw.enabled === true ? ' on' : '') + '" data-path="ai_gateway.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + t(gw.enabled === true ? 'config.toggle.active' : 'config.toggle.inactive') + '</span></div></div>';
    html += '<div class="field-grid two-cols">';
    html += aiGatewayField('config.ai_gateway.account_id', '', '<input class="field-input" data-path="ai_gateway.account_id" value="' + escapeAttr(gw.account_id || '') + '">');
    html += aiGatewayField('config.ai_gateway.gateway_id', '', '<input class="field-input" data-path="ai_gateway.gateway_id" value="' + escapeAttr(gw.gateway_id || '') + '">');
    const tokenPlaceholder = cfgSecretPlaceholder(gw.token, t('config.ai_gateway.token_placeholder'));
    html += aiGatewayField('config.ai_gateway.token', 'help.ai_gateway.token', '<div class="password-wrap cfg-password-input"><input class="field-input" type="password" autocomplete="off" data-path="ai_gateway.token" value="' + escapeAttr(cfgSecretValue(gw.token)) + '" placeholder="' + escapeAttr(tokenPlaceholder) + '"><button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button></div>');
    html += aiGatewayField('config.ai_gateway.hf_slug', 'config.ai_gateway.hf_help', '<input class="field-input" data-path="ai_gateway.huggingface_provider_slug" value="' + escapeAttr(gw.huggingface_provider_slug || '') + '">');
    html += aiGatewaySelectField('config.ai_gateway.log_mode', 'help.ai_gateway.log_mode', 'ai_gateway.log_mode', gw.log_mode, [
        { value: 'metadata_only', label: t('config.ai_gateway.log_mode_metadata_only') },
        { value: 'off', label: t('config.ai_gateway.log_mode_off') },
        { value: 'full', label: t('config.ai_gateway.log_mode_full') }
    ]);
    html += aiGatewayNumberField('config.ai_gateway.request_timeout_ms', 'help.ai_gateway.request_timeout_ms', 'ai_gateway.request_timeout_ms', gw.request_timeout_ms || 0, 0, 300000);
    html += '</div><div class="wh-notice"><small>' + t('config.ai_gateway.retry_info') + '</small></div>';
    html += aiGatewayRenderMetadataRows(gw.metadata);
    html += '<div class="field-group"><label class="field-label" for="ai-gw-provider">' + t('config.ai_gateway.provider_select') + '</label><select class="field-select" id="ai-gw-provider">';
    if (!providers.length) html += '<option value="">' + t('config.ai_gateway.no_provider') + '</option>';
    providers.forEach(p => { html += '<option value="' + escapeAttr(p.id) + '"' + (p.id === _aiGatewayProviderID ? ' selected' : '') + '>' + escapeHtml(p.name || p.id) + '</option>'; });
    html += '</select></div><div class="cfg-actions-row">';
    html += '<button type="button" class="btn-save adg-test-btn" id="ai-gw-test-btn" onclick="aiGatewayTestConnection()">' + t('config.ai_gateway.test_btn') + '</button>';
    html += '<button type="button" class="btn-save btn-secondary" id="ai-gw-model-test-btn" onclick="aiGatewayTestConnection(\'generation\')">' + t('config.ai_gateway.model_test') + '</button>';
    html += '<span id="ai-gw-test-result" class="adg-test-result" role="status" aria-live="polite"></span></div></div>';
    const content = document.getElementById('content');
    content.innerHTML = html;
    attachChangeListeners();
    const select = document.getElementById('ai-gw-provider');
    select.addEventListener('change', () => { _aiGatewayProviderID = select.value; aiGatewayCheckStatus(); });
    const invalidateDraft = event => {
        if (!event.target.closest('[data-path],.ai-gateway-metadata-row,.toggle')) return;
        aiGatewayInvalidate();
        aiGatewaySetBanner('warning', t('config.ai_gateway.save_first'));
        aiGatewaySetButtons(true);
    };
    // Section replacement disposes these listeners without accumulating root handlers.
    content.firstElementChild.addEventListener('input', invalidateDraft);
    content.firstElementChild.addEventListener('click', event => { if (event.target.closest('.toggle')) invalidateDraft(event); });
    aiGatewayCheckStatus();
}

function aiGatewayField(labelKey, helpKey, inputHtml) {
    return '<div class="field-group"><div class="field-label">' + aiGatewayText(labelKey) + '</div>' + (helpKey ? '<div class="field-help">' + aiGatewayText(helpKey) + '</div>' : '') + inputHtml + '</div>';
}

function aiGatewaySelectField(labelKey, helpKey, path, value, options) {
    return aiGatewayField(labelKey, helpKey, '<select class="field-select" data-path="' + escapeAttr(path) + '">' + options.map(o => '<option value="' + escapeAttr(o.value) + '"' + (value === o.value ? ' selected' : '') + '>' + escapeHtml(o.label) + '</option>').join('') + '</select>');
}

function aiGatewayNumberField(labelKey, helpKey, path, value, min, max) {
    return aiGatewayField(labelKey, helpKey, '<input class="field-input" type="number" step="1" min="' + min + '" max="' + max + '" data-path="' + escapeAttr(path) + '" value="' + escapeAttr(String(value)) + '">');
}

function aiGatewayRenderMetadataRows(metadata) {
    let html = '<div class="field-group ai-gateway-metadata" data-ai-gateway-metadata><div class="field-label">' + t('config.ai_gateway.metadata') + '</div><div class="field-help">' + t('help.ai_gateway.metadata') + '</div>';
    html += '<input type="hidden" data-path="ai_gateway.metadata" data-type="json" data-ai-gateway-metadata-store value="' + escapeAttr(JSON.stringify(metadata)) + '">';
    const rows = Array.isArray(metadata) ? metadata : Object.entries(metadata);
    [...rows.filter(([key, value]) => key || value), ['', '']].forEach(([key, value]) => {
        html += '<div class="field-grid two-cols ai-gateway-metadata-row"><input class="field-input" value="' + escapeAttr(key) + '" placeholder="' + escapeAttr(t('config.ai_gateway.metadata_key')) + '" data-ai-gateway-metadata-key oninput="aiGatewayCollectMetadata()"><div class="password-wrap"><input class="field-input" value="' + escapeAttr(value) + '" placeholder="' + escapeAttr(t('config.ai_gateway.metadata_value')) + '" data-ai-gateway-metadata-value oninput="aiGatewayCollectMetadata()">';
        if (key || value) html += '<button type="button" class="password-toggle" data-ai-gateway-metadata-remove onclick="aiGatewayRemoveMetadata(this)">&times;</button>';
        html += '</div></div>';
    });
    return html + '<small id="ai-gw-meta-error" role="status">' + (Array.isArray(metadata) ? escapeHtml(t('config.ai_gateway.metadata_invalid')) : '') + '</small><button type="button" class="btn-save btn-secondary" onclick="aiGatewayAddMetadataRow()">' + t('config.ai_gateway.metadata_add') + '</button></div>';
}

function aiGatewayCollectMetadata(mark = true) {
    const next = Object.create(null);
    let valid = true;
    const rows = [...document.querySelectorAll('.ai-gateway-metadata-row')].map(row => [
        row.querySelector('[data-ai-gateway-metadata-key]').value,
        row.querySelector('[data-ai-gateway-metadata-value]').value
    ]);
    rows.forEach(([rawKey, rawValue]) => {
        const key = rawKey.trim();
        const value = rawValue.trim();
        if (!key && !value) return;
        if (!key || !value || Object.hasOwn(next, key)) valid = false;
        next[key] = value;
    });
    valid = valid && Object.keys(next).length <= 5;
    const gw = aiGatewayEnsureData();
    // Invalid rows are editable draft data, never a valid payload for saving.
    gw.metadata = valid ? next : rows;
    const store = document.querySelector('[data-ai-gateway-metadata-store]');
    if (store) store.value = JSON.stringify(gw.metadata);
    const error = document.getElementById('ai-gw-meta-error');
    if (error) error.textContent = valid ? '' : t('config.ai_gateway.metadata_invalid');
    if (mark) markDirty();
    return valid;
}

function aiGatewayAddMetadataRow() {
    if (aiGatewayCollectMetadata()) renderAIGatewaySection(null);
}

function aiGatewayRemoveMetadata(button) {
    button.closest('.ai-gateway-metadata-row')?.remove();
    aiGatewayCollectMetadata();
    aiGatewayInvalidate();
    aiGatewaySetBanner('warning', t('config.ai_gateway.save_first'));
    aiGatewaySetButtons(true);
}

function aiGatewaySetBanner(state, text) {
    const banner = document.getElementById('ai-gw-status-banner');
    if (banner) { banner.className = 'adg-status-banner' + (state ? ' is-' + state : ''); banner.textContent = text; }
}

function aiGatewaySetButtons(disabled) {
    for (const id of ['ai-gw-test-btn', 'ai-gw-model-test-btn']) { const button = document.getElementById(id); if (button) button.disabled = disabled; }
}

async function aiGatewayRequest(test, kind = 'metadata') {
    aiGatewayInvalidate();
    const select = document.getElementById('ai-gw-provider');
    const epoch = _aiGatewayEpoch;
    if (!select || !select.value || aiGatewayHasDraft()) {
        aiGatewaySetBanner('warning', t(aiGatewayHasDraft() ? 'config.ai_gateway.save_first' : 'config.ai_gateway.no_provider'));
        aiGatewaySetButtons(true); return;
    }
    const controller = new AbortController(); _aiGatewayController = controller;
    const current = () => epoch === _aiGatewayEpoch && document.getElementById('ai-gw-provider') === select && select.value === _aiGatewayProviderID;
    aiGatewaySetButtons(true);
    aiGatewaySetBanner('', t('config.ai_gateway.loading'));
    try {
        const response = await fetch(test ? '/api/ai-gateway/test' : '/api/ai-gateway/status?provider_id=' + encodeURIComponent(select.value), {
            ...(test ? { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ provider_id: select.value, probe_kind: kind }) } : {}), signal: controller.signal
        });
        const res = await response.json();
        if (!current()) return;
        const successful = response.ok && res.live_status === 'ok';
        let key = 'config.ai_gateway.route_skipped';
        if (successful) key = kind === 'generation' ? 'config.ai_gateway.model_test_success' : 'config.ai_gateway.connection_metadata';
        else if (res.status === 'configured') key = res.live_status === 'skipped' ? 'config.ai_gateway.skipped' : 'config.ai_gateway.untested';
        else if (res.status === 'disabled') key = 'config.ai_gateway.status_disabled';
        else if (res.status === 'setup_required') key = 'config.ai_gateway.setup_required';
        else if (res.status === 'no_credentials') key = 'config.ai_gateway.status_no_credentials';
        else if (res.status === 'invalid_config' || !response.ok || res.status === 'error') key = 'config.ai_gateway.connection_failed';
        let message = t(key);
        if (res.code && !successful) message += ' (' + res.code + ')';
        if (res.request_timeout_ms > 0) message += ' · ' + res.request_timeout_effective_ms + ' ms';
        aiGatewaySetBanner(successful ? 'success' : (res.status === 'error' || !response.ok ? 'danger' : 'warning'), message);
        const result = document.getElementById('ai-gw-test-result');
        if (test && result) { result.textContent = message; result.className = 'adg-test-result' + (successful ? ' is-success' : ''); }
        aiGatewaySetButtons(!res.route_supported || aiGatewayHasDraft());
    } catch (err) {
        if (err.name !== 'AbortError' && current()) { aiGatewaySetBanner('danger', t('config.ai_gateway.connection_failed')); aiGatewaySetButtons(aiGatewayHasDraft()); }
    }
}

function aiGatewayCheckStatus() { return aiGatewayRequest(false); }
function aiGatewayTestConnection(kind = 'metadata') { return aiGatewayRequest(true, kind); }
