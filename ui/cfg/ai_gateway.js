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
    cfgActionResult.show('ai-gw-test-result', '', '');
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
    html += cfgStatusBanner.html('ai-gw-status-banner', 'checking');
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
    html += '<button type="button" class="btn-save adg-test-btn" id="ai-gw-test-btn" data-config-action-draft="true" onclick="aiGatewayTestConnection()">' + t('config.ai_gateway.test_btn') + '</button>';
    html += '<button type="button" class="btn-save btn-secondary" id="ai-gw-model-test-btn" data-config-action-draft="true" onclick="aiGatewayTestConnection(\'generation\')">' + t('config.ai_gateway.model_test') + '</button>';
    html += cfgActionResult.html('ai-gw-test-result') + '</div></div>';
    const content = document.getElementById('content');
    content.innerHTML = html;
    attachChangeListeners();
    const select = document.getElementById('ai-gw-provider');
    select.addEventListener('change', () => { _aiGatewayProviderID = select.value; aiGatewayCheckStatus(); });
    // The tests send the draft, so an edit only makes the last test result outdated.
    const invalidateDraft = event => {
        if (!event.target.closest('[data-path],.ai-gateway-metadata-row,.toggle')) return;
        cfgActionResult.show('ai-gw-test-result', '', '');
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
    cfgActionResult.show('ai-gw-test-result', '', '');
}

function aiGatewaySetBanner(state, detail) {
    cfgStatusBanner('ai-gw-status-banner', state, { detail: detail, retry: state === 'error' || state === 'unknown' ? () => aiGatewayCheckStatus() : null });
}

function aiGatewaySetButtons(disabled) {
    for (const id of ['ai-gw-test-btn', 'ai-gw-model-test-btn']) { const button = document.getElementById(id); if (button) button.disabled = disabled; }
}

// Maps a probe answer to a banner state and the visible message.
function aiGatewayView(response, res, kind) {
    const successful = response.ok && res.live_status === 'ok';
    let key = 'config.ai_gateway.route_skipped';
    let state = 'degraded';
    if (successful) { key = kind === 'generation' ? 'config.ai_gateway.model_test_success' : 'config.ai_gateway.connection_metadata'; state = 'connected'; }
    else if (res.status === 'configured') key = res.live_status === 'skipped' ? 'config.ai_gateway.skipped' : 'config.ai_gateway.untested';
    else if (res.status === 'disabled') { key = 'config.ai_gateway.status_disabled'; state = 'off'; }
    else if (res.status === 'setup_required') key = 'config.ai_gateway.setup_required';
    else if (res.status === 'no_credentials') { key = 'config.ai_gateway.status_no_credentials'; state = 'unconfigured'; }
    else if (res.status === 'invalid_config' || !response.ok || res.status === 'error') { key = 'config.ai_gateway.connection_failed'; state = 'error'; }
    let message = t(key);
    // A rejected draft (HTTP 400) names its reason, for example a changed target.
    if (state === 'error' && !response.ok && !res.code && res.status === 'error' && typeof res.message === 'string' && res.message.trim()) message = res.message.trim();
    if (res.code && !successful) message += ' (' + res.code + ')';
    if (res.request_timeout_ms > 0) message += ' · ' + res.request_timeout_effective_ms + ' ms';
    return { state, message, successful };
}

async function aiGatewayRequest(test, kind = 'metadata') {
    aiGatewayInvalidate();
    const select = document.getElementById('ai-gw-provider');
    const epoch = _aiGatewayEpoch;
    if (!select || !select.value) {
        aiGatewaySetBanner('unconfigured', t('config.ai_gateway.no_provider'));
        aiGatewaySetButtons(true); return;
    }
    const controller = new AbortController(); _aiGatewayController = controller;
    const current = () => epoch === _aiGatewayEpoch && document.getElementById('ai-gw-provider') === select && select.value === _aiGatewayProviderID;
    const applyView = (response, res) => {
        const view = aiGatewayView(response, res, kind);
        aiGatewaySetBanner(view.state, view.message);
        aiGatewaySetButtons(!res.route_supported);
        return view;
    };
    if (test) {
        const button = document.getElementById(kind === 'generation' ? 'ai-gw-model-test-btn' : 'ai-gw-test-btn');
        // The draft travels with the test: the gateway section as typed, the provider as saved.
        return cfgActionResult(button, async () => {
            aiGatewaySetBanner('checking', t('config.ai_gateway.loading'));
            try {
                const response = await fetch('/api/ai-gateway/test', {
                    method: 'POST', headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ provider_id: select.value, probe_kind: kind, draft: cfgDraftPatch(['ai_gateway']) }), signal: controller.signal
                });
                const res = await response.json().catch(() => ({}));
                if (!current()) return new DOMException('stale', 'AbortError');
                const view = applyView(response, res);
                return { status: view.successful ? 'ok' : (view.state === 'error' ? 'error' : 'warning'), message: view.message };
            } catch (err) {
                if (err.name !== 'AbortError' && current()) aiGatewaySetBanner('error', t('config.ai_gateway.connection_failed'));
                throw err;
            }
        }, {
            target: 'ai-gw-test-result',
            pending: t('config.ai_gateway.loading'),
            success: o => o.body.message,
            failure: o => o.body && o.body.message ? o.body.message : t('config.ai_gateway.connection_failed'),
            interpret: () => ({ reason: '' })
        });
    }
    aiGatewaySetBanner('checking', t('config.ai_gateway.loading'));
    try {
        const response = await fetch('/api/ai-gateway/status?provider_id=' + encodeURIComponent(select.value), { signal: controller.signal });
        const res = await response.json();
        if (!current()) return;
        applyView(response, res);
    } catch (err) {
        if (err.name !== 'AbortError' && current()) { aiGatewaySetBanner('error', t('config.ai_gateway.connection_failed')); aiGatewaySetButtons(false); }
    }
}

function aiGatewayCheckStatus() { return aiGatewayRequest(false); }
function aiGatewayTestConnection(kind = 'metadata') { return aiGatewayRequest(true, kind); }
