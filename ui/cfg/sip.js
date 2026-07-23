// cfg/sip.js — guided provider setup plus the full expert configuration.
let sipConfigState = null;
let sipSavedState = '';
let sipProviderCatalog = [];
let sipWizardStep = 1;
let sipWizardProviderID = '';
let sipWizardValues = {};
let sipWizardPassword = '';
let sipWizardQuery = '';
let sipWizardMessage = '';
let sipAdvancedDirty = false;
let sipAdvancedOpen = false;
let sipPhoneTargets = '';

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
    state.browser_media = state.browser_media || {};
    if (!Number.isFinite(Number(state.browser_media.udp_port)) || Number(state.browser_media.udp_port) === 0) state.browser_media.udp_port = 30100;
    state.inbound = state.inbound || {};
    state.outbound = state.outbound || {};
    state.outbound.allowed_domains = Array.isArray(state.outbound.allowed_domains) ? state.outbound.allowed_domains : [];
    state.outbound.allowed_users = Array.isArray(state.outbound.allowed_users) ? state.outbound.allowed_users : [];
    state.outbound.allowed_e164_prefixes = Array.isArray(state.outbound.allowed_e164_prefixes) ? state.outbound.allowed_e164_prefixes : [];
    state.permissions = state.permissions || {};
    state.voice = state.voice || {};
    state.password = '';
    state.clear_password = false;
    return state;
}

function sipField(path, label, type, value, extra) {
    const attrs = extra || '';
    if (type === 'checkbox') {
        return `<label class="field-group sip-toggle-field"><span class="field-label">${sipEsc(label)}</span><span class="toggle-wrap"><span class="toggle"><input type="checkbox" data-sip="${path}" ${value ? 'checked' : ''} ${attrs}><span class="slider"></span></span></span></label>`;
    }
    return `<label class="field-group"><span class="field-label">${sipEsc(label)}</span><input class="field-input" type="${type}" data-sip="${path}" value="${sipEsc(value)}" ${attrs}></label>`;
}

function sipSelect(path, label, value, options) {
    return `<label class="field-group"><span class="field-label">${sipEsc(label)}</span><select class="field-input" data-sip="${path}">${options.map(option =>
        `<option value="${sipEsc(option[0])}" ${option[0] === value ? 'selected' : ''}>${sipEsc(option[1])}</option>`
    ).join('')}</select></label>`;
}

function sipProvider(id) {
    return sipProviderCatalog.find(provider => provider.id === id) || null;
}

function sipProviderCategory(category) {
    return t(`config.sip.wizard.category_${category}`);
}

function sipWizardProgress() {
    const active = Math.max(1, sipWizardStep);
    return `<ol class="sip-wizard-progress" aria-label="${sipEsc(t('config.sip.wizard.progress'))}">
        <li class="${active >= 1 ? 'is-active' : ''}"><span>1</span>${sipEsc(t('config.sip.wizard.choose'))}</li>
        <li class="${active >= 2 ? 'is-active' : ''}"><span>2</span>${sipEsc(t('config.sip.account'))}</li>
        <li class="${active >= 3 ? 'is-active' : ''}"><span>3</span>${sipEsc(t('config.sip.wizard.review'))}</li>
    </ol>`;
}

function sipWizardConfigured() {
    const provider = sipProvider(sipConfigState.preset_id);
    if (!provider) return '';
    const phoneReady = !sipConfigState.readonly &&
        sipConfigState.browser_media.enabled &&
        sipConfigState.permissions.originate_outbound &&
        sipConfigState.outbound.allowed_domains.includes(sipConfigState.domain) &&
        (sipConfigState.outbound.allowed_users.length || sipConfigState.outbound.allowed_e164_prefixes.length);
    return `<div class="sip-wizard-configured">
        <div class="sip-wizard-configured-header">
            <div>
            <span class="sip-eyebrow">${sipEsc(t('config.sip.wizard.configured'))}</span>
            <h3>${sipEsc(provider.name)}</h3>
            <p>${sipEsc(phoneReady ? t('config.sip.wizard.phone_enabled') : t('config.sip.wizard.safe_registration'))}</p>
            </div>
            <button type="button" class="btn btn-secondary" data-sip-wizard="change">${sipEsc(t('config.sip.wizard.change'))}</button>
        </div>
        ${phoneReady ? '' : `<div class="sip-phone-activation">
            <div>
                <strong>${sipEsc(t('config.sip.wizard.phone_title'))}</strong>
                <p>${sipEsc(t('config.sip.wizard.phone_intro'))}</p>
            </div>
            <label class="field-group">
                <span class="field-label">${sipEsc(t('config.sip.wizard.phone_targets'))}</span>
                <input class="field-input" type="text" data-sip-phone-targets value="${sipEsc(sipPhoneTargets)}"
                    placeholder="${sipEsc(t('config.sip.wizard.phone_targets_placeholder'))}" autocomplete="off" maxlength="1024">
                <small>${sipEsc(t('config.sip.wizard.phone_targets_hint'))}</small>
            </label>
            <p class="sip-phone-warning">${sipEsc(t('config.sip.wizard.phone_warning'))}</p>
            <button type="button" class="btn-save" data-sip-wizard="enable-phone">${sipEsc(t('config.sip.wizard.phone_enable'))}</button>
        </div>`}
    </div>`;
}

function sipProviderGroupsMarkup() {
    const query = sipWizardQuery.trim().toLocaleLowerCase();
    const filtered = sipProviderCatalog.filter(provider => {
        if (!query) return true;
        return `${provider.name} ${provider.region} ${sipProviderCategory(provider.category)}`.toLocaleLowerCase().includes(query);
    });
    const groups = new Map();
    filtered.forEach(provider => {
        if (!groups.has(provider.category)) groups.set(provider.category, []);
        groups.get(provider.category).push(provider);
    });
    const order = ['local', 'germany', 'europe', 'north_america', 'global', 'pbx'];
    const content = order.filter(category => groups.has(category)).map(category => `
        <section class="sip-provider-group">
            <h4>${sipEsc(sipProviderCategory(category))}</h4>
            <div class="sip-provider-grid">${groups.get(category).map(provider => `
                <button type="button" class="sip-provider-card" data-provider-id="${sipEsc(provider.id)}">
                    <span class="sip-provider-monogram" aria-hidden="true">${sipEsc(provider.name.slice(0, 2).toUpperCase())}</span>
                    <span class="sip-provider-copy"><strong>${sipEsc(provider.name)}</strong><small>${sipEsc(provider.region)}</small></span>
                    <span class="sip-provider-arrow" aria-hidden="true">→</span>
                </button>`).join('')}</div>
        </section>`).join('');
    return content || `<p class="sip-empty-state">${sipEsc(t('config.sip.wizard.no_results'))}</p>`;
}

function sipWizardProviderCards() {
    return `<label class="sip-provider-search">
        <span>${sipEsc(t('config.sip.wizard.search'))}</span>
        <input class="field-input" type="search" data-sip-provider-search value="${sipEsc(sipWizardQuery)}" autocomplete="off">
    </label>
    <div class="sip-provider-results">${sipProviderGroupsMarkup()}</div>`;
}

function sipWizardCredentials(provider) {
    const canReusePassword = sipConfigState.password_set && sipConfigState.preset_id === provider.id;
    const passwordHint = canReusePassword ? t('config.sip.password_stored') : t('config.sip.password_missing');
    return `<div class="sip-wizard-panel">
        <div class="sip-wizard-provider-heading">
            <button type="button" class="sip-back-button" data-sip-wizard="back" aria-label="${sipEsc(t('config.sip.wizard.back'))}">←</button>
            <div><span class="sip-eyebrow">${sipEsc(sipProviderCategory(provider.category))} · ${sipEsc(provider.region)}</span><h3>${sipEsc(provider.name)}</h3></div>
        </div>
        ${provider.notice ? `<div class="sip-wizard-notice">${sipEsc(t(`config.sip.wizard.notice_${provider.notice}`))}</div>` : ''}
        <div class="sip-wizard-fields">${provider.fields.map(field => {
            const value = field.secret ? '' : (sipWizardValues[field.key] ?? field.default ?? '');
            const placeholder = field.secret ? passwordHint : (field.placeholder || '');
            return `<label class="field-group">
                <span class="field-label">${sipEsc(t(field.label_key))}${field.required ? ' *' : ''}</span>
                <input class="field-input" type="${field.secret ? 'password' : 'text'}" data-sip-wizard-field="${sipEsc(field.key)}"
                    value="${sipEsc(value)}" placeholder="${sipEsc(placeholder)}" maxlength="${field.secret ? '1024' : '512'}"
                    ${field.secret ? 'autocomplete="new-password"' : 'autocomplete="off"'}>
            </label>`;
        }).join('')}</div>
        <div class="sip-wizard-actions">
            <a class="sip-doc-link" href="${sipEsc(provider.documentation_url)}" target="_blank" rel="noopener noreferrer">${sipEsc(t('config.sip.wizard.documentation'))} ↗</a>
            <button type="button" class="btn-save" data-sip-wizard="review">${sipEsc(t('config.sip.wizard.continue'))}</button>
        </div>
    </div>`;
}

function sipWizardReview(provider) {
    const replacing = !!(sipConfigState.registrar && sipConfigState.preset_id !== provider.id);
    return `<div class="sip-wizard-panel">
        <div class="sip-wizard-provider-heading">
            <button type="button" class="sip-back-button" data-sip-wizard="back" aria-label="${sipEsc(t('config.sip.wizard.back'))}">←</button>
            <div><span class="sip-eyebrow">${sipEsc(t('config.sip.wizard.review'))}</span><h3>${sipEsc(provider.name)}</h3></div>
        </div>
        <div class="sip-review-grid">
            <div><span>${sipEsc(t('config.sip.registrar'))}</span><strong>${sipEsc(provider.fields.some(field => field.key === 'server') ? (sipWizardValues.server || '') : t('config.sip.wizard.automatic'))}</strong></div>
            <div><span>${sipEsc(t('config.sip.username'))}</span><strong>${sipEsc(sipWizardValues.username || sipWizardValues.phone_number || '')}</strong></div>
            <div><span>${sipEsc(t('config.sip.transport'))}</span><strong>UDP · PCMA / PCMU</strong></div>
            <div><span>${sipEsc(t('config.sip.bind_host'))}</span><strong>0.0.0.0:5060</strong></div>
        </div>
        <div class="sip-security-summary">
            <strong>${sipEsc(t('config.sip.wizard.safe_title'))}</strong>
            <p>${sipEsc(t('config.sip.wizard.safe_registration'))}</p>
        </div>
        ${replacing ? `<label class="sip-replace-confirm"><input type="checkbox" data-sip-replace-confirm> <span>${sipEsc(t('config.sip.wizard.replace_confirm'))}</span></label>` : ''}
        <div class="sip-wizard-actions">
            <span></span>
            <button type="button" class="btn-save" data-sip-wizard="apply">${sipEsc(t('config.sip.wizard.apply'))}</button>
        </div>
    </div>`;
}

function sipWizardMarkup() {
    if (sipWizardStep === 0 && sipConfigState.preset_id) return sipWizardConfigured();
    const provider = sipProvider(sipWizardProviderID);
    let body = sipWizardProviderCards();
    if (sipWizardStep === 2 && provider) body = sipWizardCredentials(provider);
    if (sipWizardStep === 3 && provider) body = sipWizardReview(provider);
    return `${sipWizardProgress()}${body}`;
}

function sipAdvancedMarkup(c) {
    const passwordHint = c.password_set ? t('config.sip.password_stored') : t('config.sip.password_missing');
    return `<details class="sip-advanced" ${sipAdvancedOpen ? 'open' : ''}>
        <summary>${sipEsc(t('config.sip.wizard.advanced'))}<span>${sipEsc(t('config.sip.wizard.advanced_hint'))}</span></summary>
        <div class="sip-advanced-content">
            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.activation'))}</h3><div class="sip-settings-grid">
                ${sipField('enabled', t('config.sip.enabled'), 'checkbox', c.enabled)}
                ${sipField('readonly', t('config.sip.readonly'), 'checkbox', c.readonly)}
            </div></div>

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.account'))}</h3><div class="sip-settings-grid">
                ${sipField('registrar', t('config.sip.registrar'), 'text', c.registrar || '', 'maxlength="255"')}
                ${sipField('domain', t('config.sip.domain'), 'text', c.domain || '', 'maxlength="255"')}
                ${sipField('username', t('config.sip.username'), 'text', c.username || '', 'maxlength="255"')}
                ${sipField('auth_username', t('config.sip.auth_username'), 'text', c.auth_username || '', 'maxlength="255"')}
                ${sipField('display_name', t('config.sip.display_name'), 'text', c.display_name || '', 'maxlength="100"')}
                ${sipField('outbound_proxy', t('config.sip.outbound_proxy'), 'text', c.outbound_proxy || '', 'maxlength="255"')}
                <label class="field-group"><span class="field-label">${sipEsc(t('config.sip.password'))}</span><input class="field-input" type="password" data-sip="password" value="" autocomplete="new-password" placeholder="${sipEsc(passwordHint)}"><small>${sipEsc(passwordHint)}</small></label>
                ${sipField('clear_password', t('config.sip.clear_password'), 'checkbox', c.clear_password)}
            </div></div>

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.signaling'))}</h3><div class="sip-settings-grid">
                ${sipField('bind_host', t('config.sip.bind_host'), 'text', c.bind_host || '127.0.0.1')}
                ${sipField('bind_port', t('config.sip.bind_port'), 'number', c.bind_port || 5060, 'min="1" max="65535"')}
                ${sipSelect('transport', t('config.sip.transport'), c.transport || 'udp', [['udp', 'UDP'], ['tcp', 'TCP'], ['tls', 'TLS']])}
                ${sipField('prefer_srv', t('config.sip.wizard.prefer_srv'), 'checkbox', c.prefer_srv)}
                ${sipField('register_expires_seconds', t('config.sip.register_expires'), 'number', c.register_expires_seconds || 300, 'min="60" max="3600"')}
                ${sipField('advertised_signaling_host', t('config.sip.advertised_signaling_host'), 'text', c.advertised_signaling_host || '')}
                ${sipField('tls.server_name', t('config.sip.tls_server_name'), 'text', c.tls.server_name || '')}
                ${sipField('tls.cert_file', t('config.sip.tls_cert_file'), 'text', c.tls.cert_file || '')}
                ${sipField('tls.key_file', t('config.sip.tls_key_file'), 'text', c.tls.key_file || '')}
            </div></div>

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.media'))}</h3><div class="sip-settings-grid">
                ${sipField('media.rtp_port_start', t('config.sip.rtp_port_start'), 'number', c.media.rtp_port_start || 30000, 'min="1024" max="65534" step="2"')}
                ${sipField('media.rtp_port_end', t('config.sip.rtp_port_end'), 'number', c.media.rtp_port_end || 30099, 'min="1025" max="65535"')}
                ${sipField('media.advertised_host', t('config.sip.advertised_media_host'), 'text', c.media.advertised_host || '')}
                ${sipField('media.symmetric_rtp', t('config.sip.symmetric_rtp'), 'checkbox', c.media.symmetric_rtp)}
                ${sipField('media.jitter_buffer_ms', t('config.sip.jitter_buffer'), 'number', c.media.jitter_buffer_ms || 60, 'min="20" max="200" step="20"')}
                ${sipField('media.codecs', t('config.sip.codecs'), 'text', sipList(c.media.codecs || ['pcma', 'pcmu']), 'readonly')}
            </div></div>

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.browser_media'))}</h3>
                <p class="sip-settings-hint">${sipEsc(t('config.sip.browser_media_note'))}</p>
                <div class="sip-settings-grid">
                    ${sipField('browser_media.enabled', t('config.sip.browser_media_enabled'), 'checkbox', c.browser_media.enabled)}
                    ${sipField('browser_media.bind_host', t('config.sip.browser_media_bind_host'), 'text', c.browser_media.bind_host || '', 'placeholder="127.0.0.1"')}
                    ${sipField('browser_media.udp_port', t('config.sip.browser_media_udp_port'), 'number', c.browser_media.udp_port || 30100, 'min="1024" max="65535"')}
                    ${sipField('browser_media.advertised_ip', t('config.sip.browser_media_advertised_ip'), 'text', c.browser_media.advertised_ip || '')}
                </div>
            </div>

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.routing'))}</h3><div class="sip-settings-grid">
                ${sipSelect('inbound.route', t('config.sip.inbound_route'), c.inbound.route || 'agent', [['agent', t('config.sip.route_agent')], ['manual', t('config.sip.route_manual')], ['reject', t('config.sip.route_reject')]])}
                ${sipField('inbound.auto_answer_delay_ms', t('config.sip.auto_answer_delay'), 'number', c.inbound.auto_answer_delay_ms ?? 1000, 'min="0" max="60000"')}
                ${sipField('inbound.trusted_peer_cidrs', t('config.sip.trusted_peers'), 'text', sipList(c.inbound.trusted_peer_cidrs))}
                ${sipField('inbound.allowed_callers', t('config.sip.allowed_callers'), 'text', sipList(c.inbound.allowed_callers))}
                ${sipField('outbound.allowed_domains', t('config.sip.allowed_domains'), 'text', sipList(c.outbound.allowed_domains))}
                ${sipField('outbound.allowed_users', t('config.sip.allowed_users'), 'text', sipList(c.outbound.allowed_users))}
                ${sipField('outbound.allowed_e164_prefixes', t('config.sip.allowed_e164'), 'text', sipList(c.outbound.allowed_e164_prefixes))}
            </div></div>

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.permissions'))}</h3><div class="sip-settings-grid">
                ${sipField('permissions.answer_inbound', t('config.sip.answer_inbound'), 'checkbox', c.permissions.answer_inbound)}
                ${sipField('permissions.originate_outbound', t('config.sip.originate_outbound'), 'checkbox', c.permissions.originate_outbound)}
                ${sipField('permissions.send_dtmf', t('config.sip.send_dtmf'), 'checkbox', c.permissions.send_dtmf)}
                ${sipField('permissions.agent_hangup', t('config.sip.agent_hangup'), 'checkbox', c.permissions.agent_hangup)}
            </div></div>

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.voice'))}</h3><div class="sip-settings-grid">
                ${sipSelect('voice.backend', t('config.sip.voice_backend'), c.voice.backend || 'classic', [['classic', t('config.sip.backend_classic')], ['gemini_live', 'Gemini Live']])}
                ${sipField('voice.realtime_profile_id', t('config.sip.realtime_profile'), 'text', c.voice.realtime_profile_id || '')}
                ${sipField('voice.language', t('config.sip.language'), 'text', c.voice.language || 'auto')}
                ${sipField('voice.allowed_tools', t('config.sip.allowed_tools'), 'text', sipList(c.voice.allowed_tools))}
                ${sipField('voice.persist_transcripts', t('config.sip.persist_transcripts'), 'checkbox', c.voice.persist_transcripts)}
                ${sipField('voice.max_call_duration_seconds', t('config.sip.max_duration'), 'number', c.voice.max_call_duration_seconds || 3600, 'min="30" max="86400"')}
                ${sipField('history_retention_days', t('config.sip.history_retention'), 'number', c.history_retention_days || 90, 'min="1" max="3650"')}
            </div></div>
        </div>
    </details>`;
}

function sipRender() {
    const c = sipConfigState;
    document.getElementById('content').innerHTML = `<section class="cfg-section active sip-section">
        <div class="section-header">${sipEsc(t('config.sip.title'))}</div>
        <div class="section-desc">${sipEsc(t('config.sip.wizard.intro'))}</div>
        <div id="sip-status" class="adg-status-banner" role="status" aria-live="polite">${sipEsc(t('config.sip.loading_status'))}</div>
        <div class="sip-wizard-shell">
            <div class="sip-wizard-title"><span class="sip-eyebrow">${sipEsc(t('config.sip.wizard.eyebrow'))}</span><h2>${sipEsc(t('config.sip.wizard.title'))}</h2></div>
            ${sipWizardMarkup()}
            <div id="sip-wizard-status" class="sip-wizard-status" role="status" aria-live="polite">${sipEsc(sipWizardMessage)}</div>
        </div>
        ${sipAdvancedMarkup(c)}
        <div class="rs-security-note"><strong>${sipEsc(t('config.sip.security_title'))}</strong><p>${sipEsc(t('config.sip.security_note'))}</p></div>
        <div class="rs-save-row"><span id="sip-action-status" role="status" aria-live="polite"></span><button type="button" class="btn btn-secondary" data-sip-action="test">${sipEsc(t('config.sip.test'))}</button><button type="button" class="btn-save" data-sip-action="save">${sipEsc(t('config.sip.save'))}</button></div>
    </section>`;
    sipBindEvents();
    sipRefreshTestLock();
    sipLoadStatus();
}

function sipBindEvents() {
    document.querySelectorAll('[data-sip]').forEach(input => input.addEventListener('input', () => {
        sipAdvancedDirty = true;
        sipRefreshTestLock();
    }));
    document.querySelector('.sip-advanced')?.addEventListener('toggle', event => {
        sipAdvancedOpen = event.currentTarget.open;
    });
    document.querySelector('[data-sip-action="save"]')?.addEventListener('click', sipSave);
    document.querySelector('[data-sip-action="test"]')?.addEventListener('click', sipTest);
    document.querySelector('[data-sip-provider-search]')?.addEventListener('input', event => {
        sipWizardQuery = event.target.value;
        const results = document.querySelector('.sip-provider-results');
        if (results) results.innerHTML = sipProviderGroupsMarkup();
        sipBindProviderCards();
    });
    sipBindProviderCards();
    document.querySelectorAll('[data-sip-wizard-field]').forEach(input => input.addEventListener('input', () => {
        if (input.dataset.sipWizardField === 'password') sipWizardPassword = input.value;
        else sipWizardValues[input.dataset.sipWizardField] = input.value;
    }));
    document.querySelector('[data-sip-wizard="change"]')?.addEventListener('click', () => {
        sipWizardStep = 1;
        sipWizardProviderID = '';
        sipWizardMessage = '';
        sipRender();
    });
    document.querySelector('[data-sip-wizard="back"]')?.addEventListener('click', () => {
        sipWizardStep = sipWizardStep === 3 ? 2 : 1;
        sipWizardMessage = '';
        sipRender();
    });
    document.querySelector('[data-sip-wizard="review"]')?.addEventListener('click', sipReviewProvider);
    document.querySelector('[data-sip-wizard="apply"]')?.addEventListener('click', sipApplyProvider);
    document.querySelector('[data-sip-phone-targets]')?.addEventListener('input', event => {
        sipPhoneTargets = event.target.value;
    });
    document.querySelector('[data-sip-wizard="enable-phone"]')?.addEventListener('click', sipEnableBrowserPhone);
}

function sipBindProviderCards() {
    document.querySelectorAll('[data-provider-id]').forEach(button => button.addEventListener('click', () => {
        sipWizardProviderID = button.dataset.providerId;
        sipWizardValues = {};
        sipWizardPassword = '';
        sipWizardMessage = '';
        sipWizardStep = 2;
        sipRender();
    }));
}

function sipReviewProvider() {
    const provider = sipProvider(sipWizardProviderID);
    if (!provider) return;
    const missing = provider.fields.find(field => {
        if (!field.required) return false;
        if (field.secret) return !sipWizardPassword && !(sipConfigState.password_set && sipConfigState.preset_id === provider.id);
        return !String(sipWizardValues[field.key] ?? field.default ?? '').trim();
    });
    if (missing) {
        sipWizardMessage = t('config.sip.wizard.required', { field: t(missing.label_key) });
        const status = document.getElementById('sip-wizard-status');
        if (status) status.textContent = sipWizardMessage;
        return;
    }
    provider.fields.forEach(field => {
        if (!field.secret && sipWizardValues[field.key] == null && field.default) sipWizardValues[field.key] = field.default;
    });
    sipWizardMessage = '';
    sipWizardStep = 3;
    sipRender();
}

async function sipApplyProvider() {
    const provider = sipProvider(sipWizardProviderID);
    const button = document.querySelector('[data-sip-wizard="apply"]');
    if (!provider || !button) return;
    const replacing = !!(sipConfigState.registrar && sipConfigState.preset_id !== provider.id);
    const confirmation = document.querySelector('[data-sip-replace-confirm]');
    if (replacing && !confirmation?.checked) {
        sipWizardMessage = t('config.sip.wizard.replace_required');
        const status = document.getElementById('sip-wizard-status');
        if (status) status.textContent = sipWizardMessage;
        return;
    }
    button.disabled = true;
    sipWizardMessage = t('config.sip.saving');
    document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
    try {
        const result = await sipRequest('/api/sip/setup', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                provider_id: provider.id,
                values: sipWizardValues,
                password: sipWizardPassword,
                confirm_replace: replacing
            })
        });
        sipConfigState = sipNormalize(Object.prototype.hasOwnProperty.call(result, 'enabled') ? result : await sipRequest('/api/sip/config'));
        sipSavedState = sipComparable(sipConfigState);
        sipAdvancedDirty = false;
        sipWizardPassword = '';
        sipWizardStep = 0;
        sipWizardMessage = result && result.needs_restart ? t('config.sip.restart_required') : t('config.sip.wizard.applied');
        sipRender();
    } catch (error) {
        sipWizardMessage = error.message;
        document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
        button.disabled = false;
    }
}

function sipParsePhoneTargets(raw) {
    const users = [];
    const prefixes = [];
    for (const target of sipSplit(raw)) {
        if (/^\+[1-9][0-9]{0,14}$/.test(target)) {
            prefixes.push(target);
        } else if (/^[A-Za-z0-9_.!~*'()%+\-]+$/.test(target)) {
            users.push(target);
        } else {
            throw new Error(t('config.sip.wizard.phone_invalid', { target }));
        }
    }
    if (!users.length && !prefixes.length) throw new Error(t('config.sip.wizard.phone_required'));
    return { users: [...new Set(users)], prefixes: [...new Set(prefixes)] };
}

async function sipEnableBrowserPhone() {
    const button = document.querySelector('[data-sip-wizard="enable-phone"]');
    if (!button) return;
    let targets;
    try {
        targets = sipParsePhoneTargets(sipPhoneTargets);
    } catch (error) {
        sipWizardMessage = error.message;
        document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
        return;
    }
    button.disabled = true;
    sipWizardMessage = t('config.sip.saving');
    document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
    try {
        const browserMediaWasEnabled = !!sipConfigState.browser_media.enabled;
        const next = JSON.parse(JSON.stringify(sipConfigState));
        next.readonly = false;
        next.browser_media.enabled = true;
        next.outbound.allowed_domains = [next.domain];
        next.outbound.allowed_users = targets.users;
        next.outbound.allowed_e164_prefixes = targets.prefixes;
        next.permissions.originate_outbound = true;
        next.permissions.send_dtmf = true;
        next.permissions.agent_hangup = true;
        const result = await sipRequest('/api/sip/config', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(next)
        });
        sipConfigState = sipNormalize(await sipRequest('/api/sip/config'));
        sipSavedState = sipComparable(sipConfigState);
        sipAdvancedDirty = false;
        sipWizardMessage = !browserMediaWasEnabled || (result && result.needs_restart)
            ? t('config.sip.restart_required')
            : t('config.sip.wizard.phone_enabled');
        sipRender();
    } catch (error) {
        sipWizardMessage = error.message;
        document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
        button.disabled = false;
    }
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
    if (sipAdvancedDirty) result.preset_id = '';
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
        sipConfigState = sipNormalize(Object.prototype.hasOwnProperty.call(saved, 'enabled') ? saved : await sipRequest('/api/sip/config'));
        sipSavedState = sipComparable(sipConfigState);
        sipAdvancedDirty = false;
        if (!sipConfigState.preset_id) sipWizardStep = 1;
        sipRender();
        document.getElementById('sip-action-status').textContent = saved && saved.needs_restart
            ? t('config.sip.restart_required')
            : t('config.sip.saved');
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
    sipWizardStep = 1;
    sipWizardProviderID = '';
    sipWizardValues = {};
    sipWizardPassword = '';
    sipWizardQuery = '';
    sipWizardMessage = '';
    sipAdvancedDirty = false;
    sipAdvancedOpen = false;
    sipPhoneTargets = '';
    const content = document.getElementById('content');
    content.innerHTML = `<div class="cfg-section active"><div class="cfg-loading-state">${sipEsc(t('config.sip.loading'))}</div></div>`;
    try {
        const [configuration, catalog] = await Promise.all([
            sipRequest('/api/sip/config'),
            sipRequest('/api/sip/providers')
        ]);
        sipConfigState = sipNormalize(configuration);
        sipProviderCatalog = Array.isArray(catalog.providers) ? catalog.providers : [];
        sipSavedState = sipComparable(sipConfigState);
        sipPhoneTargets = sipList([
            ...(sipConfigState.outbound.allowed_users || []),
            ...(sipConfigState.outbound.allowed_e164_prefixes || [])
        ]);
        sipWizardProviderID = sipConfigState.preset_id || '';
        sipWizardStep = sipConfigState.preset_id && sipProvider(sipConfigState.preset_id) ? 0 : 1;
        sipAdvancedDirty = false;
        sipRender();
    } catch (error) {
        content.innerHTML = `<div class="cfg-section active"><div class="rs-load-error">${sipEsc(error.message)}</div></div>`;
    }
}
