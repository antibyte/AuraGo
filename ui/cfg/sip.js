// cfg/sip.js — native single-account SIP endpoint configuration.
let sipConfigState = null;
let sipSavedState = '';

function sipEsc(value) {
    return String(value == null ? '' : value)
        .replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;').replaceAll("'", '&#39;');
}

function sipList(value) {
    return Array.isArray(value) ? value.join(', ') : '';
}

function sipSplit(value) {
    return String(value || '').split(',').map(item => item.trim()).filter(Boolean);
}

function sipNormalize(data) {
    const state = data || {};
    state.tls = state.tls || {};
    state.media = state.media || {};
    state.inbound = state.inbound || {};
    state.outbound = state.outbound || {};
    state.permissions = state.permissions || {};
    state.voice = state.voice || {};
    state.password = '';
    state.clear_password = false;
    return state;
}

function sipField(path, label, type, value, extra) {
    const attrs = extra || '';
    if (type === 'checkbox') {
        return `<label class="field-group"><span class="field-label">${sipEsc(label)}</span><div class="toggle-wrap"><input type="checkbox" data-sip="${path}" ${value ? 'checked' : ''} ${attrs}></div></label>`;
    }
    return `<label class="field-group"><span class="field-label">${sipEsc(label)}</span><input class="field-input" type="${type}" data-sip="${path}" value="${sipEsc(value)}" ${attrs}></label>`;
}

function sipSelect(path, label, value, options) {
    return `<label class="field-group"><span class="field-label">${sipEsc(label)}</span><select class="field-input" data-sip="${path}">${options.map(option =>
        `<option value="${sipEsc(option[0])}" ${option[0] === value ? 'selected' : ''}>${sipEsc(option[1])}</option>`
    ).join('')}</select></label>`;
}

function sipRender() {
    const c = sipConfigState;
    const passwordHint = c.password_set ? t('config.sip.password_stored') : t('config.sip.password_missing');
    document.getElementById('content').innerHTML = `<section class="cfg-section active sip-section">
        <div class="section-header">${sipEsc(t('config.sip.title'))}</div>
        <div class="section-desc">${sipEsc(t('config.sip.description'))}</div>
        <div id="sip-status" class="adg-status-banner" role="status" aria-live="polite">${sipEsc(t('config.sip.loading_status'))}</div>

        <div class="settings-group"><h3>${sipEsc(t('config.sip.activation'))}</h3><div class="settings-grid">
            ${sipField('enabled', t('config.sip.enabled'), 'checkbox', c.enabled)}
            ${sipField('readonly', t('config.sip.readonly'), 'checkbox', c.readonly)}
        </div></div>

        <div class="settings-group"><h3>${sipEsc(t('config.sip.account'))}</h3><div class="settings-grid">
            ${sipField('registrar', t('config.sip.registrar'), 'text', c.registrar || '', 'maxlength="255"')}
            ${sipField('domain', t('config.sip.domain'), 'text', c.domain || '', 'maxlength="255"')}
            ${sipField('username', t('config.sip.username'), 'text', c.username || '', 'maxlength="255"')}
            ${sipField('auth_username', t('config.sip.auth_username'), 'text', c.auth_username || '', 'maxlength="255"')}
            ${sipField('display_name', t('config.sip.display_name'), 'text', c.display_name || '', 'maxlength="100"')}
            ${sipField('outbound_proxy', t('config.sip.outbound_proxy'), 'text', c.outbound_proxy || '', 'maxlength="255"')}
            <label class="field-group"><span class="field-label">${sipEsc(t('config.sip.password'))}</span><input class="field-input" type="password" data-sip="password" value="" autocomplete="new-password" placeholder="${sipEsc(passwordHint)}"><small>${sipEsc(passwordHint)}</small></label>
            ${sipField('clear_password', t('config.sip.clear_password'), 'checkbox', c.clear_password)}
        </div></div>

        <div class="settings-group"><h3>${sipEsc(t('config.sip.signaling'))}</h3><div class="settings-grid">
            ${sipField('bind_host', t('config.sip.bind_host'), 'text', c.bind_host || '127.0.0.1')}
            ${sipField('bind_port', t('config.sip.bind_port'), 'number', c.bind_port || 5060, 'min="1" max="65535"')}
            ${sipSelect('transport', t('config.sip.transport'), c.transport || 'udp', [['udp', 'UDP'], ['tcp', 'TCP'], ['tls', 'TLS']])}
            ${sipField('register_expires_seconds', t('config.sip.register_expires'), 'number', c.register_expires_seconds || 300, 'min="60" max="3600"')}
            ${sipField('advertised_signaling_host', t('config.sip.advertised_signaling_host'), 'text', c.advertised_signaling_host || '')}
            ${sipField('tls.server_name', t('config.sip.tls_server_name'), 'text', c.tls.server_name || '')}
            ${sipField('tls.cert_file', t('config.sip.tls_cert_file'), 'text', c.tls.cert_file || '')}
            ${sipField('tls.key_file', t('config.sip.tls_key_file'), 'text', c.tls.key_file || '')}
        </div></div>

        <div class="settings-group"><h3>${sipEsc(t('config.sip.media'))}</h3><div class="settings-grid">
            ${sipField('media.rtp_port_start', t('config.sip.rtp_port_start'), 'number', c.media.rtp_port_start || 30000, 'min="1024" max="65534" step="2"')}
            ${sipField('media.rtp_port_end', t('config.sip.rtp_port_end'), 'number', c.media.rtp_port_end || 30099, 'min="1025" max="65535"')}
            ${sipField('media.advertised_host', t('config.sip.advertised_media_host'), 'text', c.media.advertised_host || '')}
            ${sipField('media.symmetric_rtp', t('config.sip.symmetric_rtp'), 'checkbox', c.media.symmetric_rtp)}
            ${sipField('media.jitter_buffer_ms', t('config.sip.jitter_buffer'), 'number', c.media.jitter_buffer_ms || 60, 'min="20" max="200" step="20"')}
            ${sipField('media.codecs', t('config.sip.codecs'), 'text', sipList(c.media.codecs || ['pcma', 'pcmu']), 'readonly')}
        </div></div>

        <div class="settings-group"><h3>${sipEsc(t('config.sip.routing'))}</h3><div class="settings-grid">
            ${sipSelect('inbound.route', t('config.sip.inbound_route'), c.inbound.route || 'agent', [['agent', t('config.sip.route_agent')], ['manual', t('config.sip.route_manual')], ['reject', t('config.sip.route_reject')]])}
            ${sipField('inbound.auto_answer_delay_ms', t('config.sip.auto_answer_delay'), 'number', c.inbound.auto_answer_delay_ms || 1000, 'min="0" max="60000"')}
            ${sipField('inbound.trusted_peer_cidrs', t('config.sip.trusted_peers'), 'text', sipList(c.inbound.trusted_peer_cidrs))}
            ${sipField('inbound.allowed_callers', t('config.sip.allowed_callers'), 'text', sipList(c.inbound.allowed_callers))}
            ${sipField('outbound.allowed_domains', t('config.sip.allowed_domains'), 'text', sipList(c.outbound.allowed_domains))}
            ${sipField('outbound.allowed_users', t('config.sip.allowed_users'), 'text', sipList(c.outbound.allowed_users))}
            ${sipField('outbound.allowed_e164_prefixes', t('config.sip.allowed_e164'), 'text', sipList(c.outbound.allowed_e164_prefixes))}
        </div></div>

        <div class="settings-group"><h3>${sipEsc(t('config.sip.permissions'))}</h3><div class="settings-grid">
            ${sipField('permissions.answer_inbound', t('config.sip.answer_inbound'), 'checkbox', c.permissions.answer_inbound)}
            ${sipField('permissions.originate_outbound', t('config.sip.originate_outbound'), 'checkbox', c.permissions.originate_outbound)}
            ${sipField('permissions.send_dtmf', t('config.sip.send_dtmf'), 'checkbox', c.permissions.send_dtmf)}
            ${sipField('permissions.agent_hangup', t('config.sip.agent_hangup'), 'checkbox', c.permissions.agent_hangup)}
        </div></div>

        <div class="settings-group"><h3>${sipEsc(t('config.sip.voice'))}</h3><div class="settings-grid">
            ${sipSelect('voice.backend', t('config.sip.voice_backend'), c.voice.backend || 'classic', [['classic', t('config.sip.backend_classic')], ['gemini_live', 'Gemini Live']])}
            ${sipField('voice.realtime_profile_id', t('config.sip.realtime_profile'), 'text', c.voice.realtime_profile_id || '')}
            ${sipField('voice.language', t('config.sip.language'), 'text', c.voice.language || 'auto')}
            ${sipField('voice.allowed_tools', t('config.sip.allowed_tools'), 'text', sipList(c.voice.allowed_tools))}
            ${sipField('voice.persist_transcripts', t('config.sip.persist_transcripts'), 'checkbox', c.voice.persist_transcripts)}
            ${sipField('voice.max_call_duration_seconds', t('config.sip.max_duration'), 'number', c.voice.max_call_duration_seconds || 3600, 'min="30" max="86400"')}
            ${sipField('history_retention_days', t('config.sip.history_retention'), 'number', c.history_retention_days || 90, 'min="1" max="3650"')}
        </div></div>

        <div class="rs-security-note"><strong>${sipEsc(t('config.sip.security_title'))}</strong><p>${sipEsc(t('config.sip.security_note'))}</p></div>
        <div class="rs-save-row"><span id="sip-action-status" role="status" aria-live="polite"></span><button type="button" class="btn btn-secondary" data-sip-action="test">${sipEsc(t('config.sip.test'))}</button><button type="button" class="btn-save" data-sip-action="save">${sipEsc(t('config.sip.save'))}</button></div>
    </section>`;
    document.querySelectorAll('[data-sip]').forEach(input => input.addEventListener('input', sipRefreshTestLock));
    document.querySelector('[data-sip-action="save"]')?.addEventListener('click', sipSave);
    document.querySelector('[data-sip-action="test"]')?.addEventListener('click', sipTest);
    sipRefreshTestLock();
    sipLoadStatus();
}

function sipAssign(target, path, value) {
    const parts = path.split('.');
    let cursor = target;
    for (let index = 0; index < parts.length - 1; index += 1) cursor = cursor[parts[index]];
    cursor[parts.at(-1)] = value;
}

function sipRead() {
    const result = JSON.parse(JSON.stringify(sipConfigState));
    document.querySelectorAll('[data-sip]').forEach(input => {
        let value = input.type === 'checkbox' ? input.checked : input.value;
        if (input.type === 'number') value = Number(value);
        if (['media.codecs', 'inbound.trusted_peer_cidrs', 'inbound.allowed_callers', 'outbound.allowed_domains', 'outbound.allowed_users', 'outbound.allowed_e164_prefixes', 'voice.allowed_tools'].includes(input.dataset.sip)) value = sipSplit(value);
        sipAssign(result, input.dataset.sip, value);
    });
    return result;
}

function sipComparable(value) {
    const copy = JSON.parse(JSON.stringify(value));
    copy.password = '';
    copy.clear_password = false;
    return JSON.stringify(copy);
}

function sipRefreshTestLock() {
    const button = document.querySelector('[data-sip-action="test"]');
    if (!button) return;
    const current = sipRead();
    const dirty = sipComparable(current) !== sipSavedState || current.password || current.clear_password;
    let reason = '';
    if (dirty) reason = t('config.sip.save_first');
    else if (!current.enabled) reason = t('config.sip.enable_first');
    else if (!current.password_set) reason = t('config.sip.password_required');
    button.disabled = !!reason;
    button.title = reason;
}

async function sipRequest(path, options) {
    const response = await fetch(path, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(body.error || body.message || ('HTTP ' + response.status));
    return body;
}

async function sipSave() {
    const status = document.getElementById('sip-action-status');
    const save = document.querySelector('[data-sip-action="save"]');
    save.disabled = true;
    status.textContent = t('config.sip.saving');
    try {
        const saved = await sipRequest('/api/sip/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sipRead()) });
        // A saved configuration may need a process restart when runtime
        // reconciliation returns 202. Reload the authoritative masked config
        // instead of treating the pending-status envelope as configuration.
        sipConfigState = sipNormalize(Object.prototype.hasOwnProperty.call(saved, 'enabled') ? saved : await sipRequest('/api/sip/config'));
        sipSavedState = sipComparable(sipConfigState);
        sipRender();
        document.getElementById('sip-action-status').textContent = t('config.sip.saved');
    } catch (error) {
        status.textContent = error.message;
        save.disabled = false;
    }
}

async function sipTest() {
    const status = document.getElementById('sip-action-status');
    status.textContent = t('config.sip.testing');
    try {
        await sipRequest('/api/sip/test', { method: 'POST' });
        status.textContent = t('config.sip.test_ok');
        sipLoadStatus();
    } catch (error) {
        status.textContent = error.message;
    }
}

async function sipLoadStatus() {
    const banner = document.getElementById('sip-status');
    if (!banner) return;
    try {
        const status = await sipRequest('/api/sip/status');
        banner.className = `adg-status-banner ${status.registered ? 'is-success' : (status.state === 'failed' ? 'is-danger' : 'is-warning')}`;
        banner.textContent = t('config.sip.status_value', { state: status.state, address: status.bind_address });
    } catch (error) {
        banner.className = 'adg-status-banner is-danger';
        banner.textContent = error.message;
    }
}

async function renderSIPSection() {
    const content = document.getElementById('content');
    content.innerHTML = `<div class="cfg-section active"><div class="cfg-loading-state">${sipEsc(t('config.sip.loading'))}</div></div>`;
    try {
        sipConfigState = sipNormalize(await sipRequest('/api/sip/config'));
        sipSavedState = sipComparable(sipConfigState);
        sipRender();
    } catch (error) {
        content.innerHTML = `<div class="cfg-section active"><div class="rs-load-error">${sipEsc(error.message)}</div></div>`;
    }
}
