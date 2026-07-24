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
let sipWizardActivationMode = 'registration'; // registration | desktop
let sipWizardTrustedPeers = '';
let sipWizardAllowedCallers = '';
let sipWizardDeniedCallers = '';
let sipPhoneBlockedTargets = '';

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
    state.inbound.route = state.inbound.route || 'agent';
    state.inbound.trusted_peer_cidrs = Array.isArray(state.inbound.trusted_peer_cidrs) ? state.inbound.trusted_peer_cidrs : [];
    state.inbound.allowed_callers = Array.isArray(state.inbound.allowed_callers) ? state.inbound.allowed_callers : [];
    state.inbound.denied_callers = Array.isArray(state.inbound.denied_callers) ? state.inbound.denied_callers : [];
    state.outbound = state.outbound || {};
    state.outbound.allowed_domains = Array.isArray(state.outbound.allowed_domains) ? state.outbound.allowed_domains : [];
    state.outbound.denied_domains = Array.isArray(state.outbound.denied_domains) ? state.outbound.denied_domains : [];
    state.outbound.allowed_users = Array.isArray(state.outbound.allowed_users) ? state.outbound.allowed_users : [];
    state.outbound.denied_users = Array.isArray(state.outbound.denied_users) ? state.outbound.denied_users : [];
    state.outbound.allowed_e164_prefixes = Array.isArray(state.outbound.allowed_e164_prefixes) ? state.outbound.allowed_e164_prefixes : [];
    state.outbound.denied_e164_prefixes = Array.isArray(state.outbound.denied_e164_prefixes) ? state.outbound.denied_e164_prefixes : [];
    state.permissions = state.permissions || {};
    state.voice = state.voice || {};
    state.password = '';
    state.clear_password = false;
    return state;
}

function sipField(path, label, type, value, extra, help) {
    const attrs = extra || '';
    const helpMarkup = help ? `<small>${sipEsc(help)}</small>` : '';
    if (type === 'checkbox') {
        return `<label class="field-group sip-toggle-field"><span><span class="field-label">${sipEsc(label)}</span>${helpMarkup}</span><span class="toggle-wrap"><span class="toggle"><input type="checkbox" data-sip="${path}" ${value ? 'checked' : ''} ${attrs}><span class="slider"></span></span></span></label>`;
    }
    return `<label class="field-group"><span class="field-label">${sipEsc(label)}</span><input class="field-input" type="${type}" data-sip="${path}" value="${sipEsc(value)}" ${attrs}>${helpMarkup}</label>`;
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

function sipOutboundPhoneReady(state) {
    const c = state || sipConfigState;
    if (!c) return false;
    return !c.readonly &&
        !!c.browser_media.enabled &&
        !!c.permissions.originate_outbound &&
        Array.isArray(c.outbound.allowed_domains) && c.outbound.allowed_domains.length > 0 &&
        ((c.outbound.allowed_users && c.outbound.allowed_users.length) ||
            (c.outbound.allowed_e164_prefixes && c.outbound.allowed_e164_prefixes.length));
}

function sipDesktopPhoneReady(state) {
    const c = state || sipConfigState;
    if (!sipOutboundPhoneReady(c)) return false;
    return c.inbound.route === 'manual' &&
        !!c.permissions.answer_inbound &&
        !!c.permissions.send_dtmf &&
        Array.isArray(c.inbound.trusted_peer_cidrs) && c.inbound.trusted_peer_cidrs.length > 0 &&
        Array.isArray(c.inbound.allowed_callers) && c.inbound.allowed_callers.length > 0;
}

function sipWizardConfigured() {
    const provider = sipProvider(sipConfigState.preset_id);
    if (!provider) return '';
    const fullReady = sipDesktopPhoneReady(sipConfigState);
    const outboundReady = sipOutboundPhoneReady(sipConfigState);
    const statusText = fullReady
        ? t('config.sip.wizard.phone_enabled')
        : (outboundReady ? t('config.sip.wizard.phone_enabled') : t('config.sip.wizard.safe_registration'));
    return `<div class="sip-wizard-configured">
        <div class="sip-wizard-configured-header">
            <div>
            <span class="sip-eyebrow">${sipEsc(t('config.sip.wizard.configured'))}</span>
            <h3>${sipEsc(provider.name)}</h3>
            <p>${sipEsc(statusText)}</p>
            </div>
            <button type="button" class="btn btn-secondary" data-sip-wizard="change">${sipEsc(t('config.sip.wizard.change'))}</button>
        </div>
        ${fullReady ? '' : `<div class="sip-phone-activation">
            <div>
                <strong>${sipEsc(t(outboundReady ? 'config.sip.wizard.phone_full_title' : 'config.sip.wizard.phone_full_title'))}</strong>
                <p>${sipEsc(t('config.sip.wizard.phone_full_intro'))}</p>
            </div>
            ${sipDesktopActivationFieldsMarkup()}
            <p class="sip-phone-warning">${sipEsc(t('config.sip.wizard.phone_warning'))}</p>
            <button type="button" class="btn-save" data-sip-wizard="enable-phone">${sipEsc(t(outboundReady ? 'config.sip.wizard.phone_full_enable' : 'config.sip.wizard.phone_full_enable'))}</button>
        </div>`}
    </div>`;
}

function sipDesktopActivationFieldsMarkup() {
    return `<div class="sip-desktop-fields">
        <label class="field-group">
            <span class="field-label">${sipEsc(t('config.sip.wizard.phone_targets'))}</span>
            <input class="field-input" type="text" data-sip-phone-targets value="${sipEsc(sipPhoneTargets)}"
                placeholder="${sipEsc(t('config.sip.wizard.phone_targets_placeholder'))}" autocomplete="off" maxlength="1024">
            <small>${sipEsc(t('config.sip.wizard.phone_targets_hint'))}</small>
        </label>
        <label class="field-group">
            <span class="field-label">${sipEsc(t('config.sip.blocked_targets'))}</span>
            <input class="field-input" type="text" data-sip-phone-blocked-targets value="${sipEsc(sipPhoneBlockedTargets)}"
                placeholder="+49900, premium-*" autocomplete="off" maxlength="1024">
            <small>${sipEsc(t('config.sip.blocked_targets_help'))}</small>
        </label>
        <label class="field-group">
            <span class="field-label">${sipEsc(t('config.sip.wizard.trusted_peers'))}</span>
            <input class="field-input" type="text" data-sip-wizard-trusted-peers value="${sipEsc(sipWizardTrustedPeers)}"
                placeholder="${sipEsc(t('config.sip.wizard.trusted_peers_placeholder'))}" autocomplete="off" maxlength="1024">
            <small>${sipEsc(t('config.sip.wizard.trusted_peers_hint'))}</small>
        </label>
        <label class="field-group">
            <span class="field-label">${sipEsc(t('config.sip.wizard.allowed_callers'))}</span>
            <input class="field-input" type="text" data-sip-wizard-allowed-callers value="${sipEsc(sipWizardAllowedCallers)}"
                placeholder="${sipEsc(t('config.sip.wizard.allowed_callers_placeholder'))}" autocomplete="off" maxlength="1024">
            <small>${sipEsc(t('config.sip.wizard.allowed_callers_hint'))}</small>
        </label>
        <label class="field-group">
            <span class="field-label">${sipEsc(t('config.sip.denied_callers'))}</span>
            <input class="field-input" type="text" data-sip-wizard-denied-callers value="${sipEsc(sipWizardDeniedCallers)}"
                placeholder="+49900*, sip:blocked@*" autocomplete="off" maxlength="1024">
            <small>${sipEsc(t('config.sip.denied_callers_help'))}</small>
        </label>
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
    const desktop = sipWizardActivationMode === 'desktop';
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
        <div class="sip-activation-modes" role="radiogroup" aria-label="${sipEsc(t('config.sip.wizard.mode_title'))}">
            <p class="sip-activation-modes-title">${sipEsc(t('config.sip.wizard.mode_title'))}</p>
            <label class="sip-mode-card ${desktop ? '' : 'is-selected'}">
                <input type="radio" name="sip-activation-mode" data-sip-activation-mode value="registration" ${desktop ? '' : 'checked'}>
                <span>
                    <strong>${sipEsc(t('config.sip.wizard.mode_registration'))}</strong>
                    <small>${sipEsc(t('config.sip.wizard.mode_registration_desc'))}</small>
                </span>
            </label>
            <label class="sip-mode-card ${desktop ? 'is-selected' : ''}">
                <input type="radio" name="sip-activation-mode" data-sip-activation-mode value="desktop" ${desktop ? 'checked' : ''}>
                <span>
                    <strong>${sipEsc(t('config.sip.wizard.mode_desktop'))}</strong>
                    <small>${sipEsc(t('config.sip.wizard.mode_desktop_desc'))}</small>
                </span>
            </label>
        </div>
        ${desktop ? `<div class="sip-desktop-activation-panel">
            <strong>${sipEsc(t('config.sip.wizard.phone_full_title'))}</strong>
            <p>${sipEsc(t('config.sip.wizard.phone_full_intro'))}</p>
            ${sipDesktopActivationFieldsMarkup()}
            <p class="sip-phone-warning">${sipEsc(t('config.sip.wizard.phone_warning'))}</p>
        </div>` : `<div class="sip-security-summary">
            <strong>${sipEsc(t('config.sip.wizard.safe_title'))}</strong>
            <p>${sipEsc(t('config.sip.wizard.safe_registration'))}</p>
        </div>`}
        ${replacing ? `<label class="sip-replace-confirm"><input type="checkbox" data-sip-replace-confirm> <span>${sipEsc(t('config.sip.wizard.replace_confirm'))}</span></label>` : ''}
        <div class="sip-wizard-actions">
            <span></span>
            <button type="button" class="btn-save" data-sip-wizard="apply">${sipEsc(desktop ? t('config.sip.wizard.apply_desktop') : t('config.sip.wizard.apply'))}</button>
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

            <div class="sip-settings-group"><h3>${sipEsc(t('config.sip.routing'))}</h3>
                <p class="sip-settings-hint">${sipEsc(t('config.sip.policy_intro'))}</p>
                <div class="sip-policy-legend">${sipEsc(t('config.sip.policy_precedence'))}</div>
                <div class="sip-settings-grid">
                ${sipSelect('inbound.route', t('config.sip.inbound_route'), c.inbound.route || 'agent', [['agent', t('config.sip.route_agent')], ['manual', t('config.sip.route_manual')], ['reject', t('config.sip.route_reject')]])}
                ${sipField('inbound.auto_answer_delay_ms', t('config.sip.auto_answer_delay'), 'number', c.inbound.auto_answer_delay_ms ?? 1000, 'min="0" max="60000"')}
                ${sipField('inbound.trusted_peer_cidrs', t('config.sip.trusted_peers'), 'text', sipList(c.inbound.trusted_peer_cidrs), 'placeholder="192.168.1.1, 192.168.1.0/24"', t('config.sip.trusted_peers_help'))}
                ${sipField('inbound.allowed_callers', t('config.sip.allowed_callers'), 'text', sipList(c.inbound.allowed_callers), 'placeholder="101, +49*, sip:service-?@pbx.example"', t('config.sip.allowed_callers_help'))}
                ${sipField('inbound.denied_callers', t('config.sip.denied_callers'), 'text', sipList(c.inbound.denied_callers), 'placeholder="+49900*, sip:blocked@*"', t('config.sip.denied_callers_help'))}
                ${sipField('outbound.allowed_domains', t('config.sip.allowed_domains'), 'text', sipList(c.outbound.allowed_domains), 'placeholder="pbx.example, *.example.com"', t('config.sip.allowed_domains_help'))}
                ${sipField('outbound.denied_domains', t('config.sip.denied_domains'), 'text', sipList(c.outbound.denied_domains), 'placeholder="premium.example.com"', t('config.sip.denied_domains_help'))}
                ${sipField('outbound.allowed_users', t('config.sip.allowed_users'), 'text', sipList(c.outbound.allowed_users), 'placeholder="101, sales-*, *"', t('config.sip.allowed_users_help'))}
                ${sipField('outbound.denied_users', t('config.sip.denied_users'), 'text', sipList(c.outbound.denied_users), 'placeholder="0900*, service-??"', t('config.sip.denied_users_help'))}
                ${sipField('outbound.allowed_e164_prefixes', t('config.sip.allowed_e164'), 'text', sipList(c.outbound.allowed_e164_prefixes), 'placeholder="+49, +43"', t('config.sip.allowed_e164_help'))}
                ${sipField('outbound.denied_e164_prefixes', t('config.sip.denied_e164'), 'text', sipList(c.outbound.denied_e164_prefixes), 'placeholder="+49900, +43810"', t('config.sip.denied_e164_help'))}
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

function sipNotifyDirty() {
    sipAdvancedDirty = true;
    sipRefreshTestLock();
    if (typeof setDirty === 'function') setDirty(true);
}

function sipCanonicalDestinations(users, prefixes) {
    const nextUsers = [];
    const nextPrefixes = [];
    for (const value of [...(users || []), ...(prefixes || [])]) {
        const item = String(value || '').trim();
        if (!item) continue;
        if (/^\+[1-9][0-9]{0,14}$/.test(item)) nextPrefixes.push(item);
        else nextUsers.push(item);
    }
    return {
        users: [...new Set(nextUsers)],
        prefixes: [...new Set(nextPrefixes)]
    };
}

function sipNormalizeOutboundPayload(payload) {
    if (!payload || !payload.outbound) return payload;
    const classified = sipCanonicalDestinations(payload.outbound.allowed_users, payload.outbound.allowed_e164_prefixes);
    payload.outbound.allowed_users = classified.users;
    payload.outbound.allowed_e164_prefixes = classified.prefixes;
    const denied = sipCanonicalDestinations(payload.outbound.denied_users, payload.outbound.denied_e164_prefixes);
    payload.outbound.denied_users = denied.users;
    payload.outbound.denied_e164_prefixes = denied.prefixes;
    const domain = String(payload.domain || '').trim().toLowerCase();
    const domains = Array.isArray(payload.outbound.allowed_domains) ? payload.outbound.allowed_domains.slice() : [];
    if (domain && (classified.users.length || classified.prefixes.length) && !domains.map(item => String(item).toLowerCase()).includes(domain)) {
        domains.push(domain);
    }
    payload.outbound.allowed_domains = domains;
    return payload;
}

function sipComparable(value) {
    const copy = JSON.parse(JSON.stringify(value || {}));
    copy.password = '';
    copy.clear_password = false;
    delete copy.password_set;
    // Ignore display-only noise so rendered defaults do not look unsaved.
    sipNormalizeOutboundPayload(copy);
    return JSON.stringify(copy);
}

function sipIsDirty() {
    if (!sipConfigState || !sipSavedState) return false;
    if (!document.querySelector('[data-sip]')) return sipAdvancedDirty;
    try {
        const current = sipRead({ forCompare: true });
        if (sipComparable(current) !== sipSavedState || !!current.password || !!current.clear_password) return true;
    } catch (_) {
        if (sipAdvancedDirty) return true;
    }
    // Wizard phone targets live outside the expert data-sip fields.
    if (document.querySelector('[data-sip-phone-targets]')) {
        const savedTargets = sipList([
            ...(sipConfigState.outbound.allowed_users || []),
            ...(sipConfigState.outbound.allowed_e164_prefixes || [])
        ]);
        if (String(sipPhoneTargets || '').trim() !== String(savedTargets || '').trim()) return true;
        const savedBlockedTargets = sipList([
            ...(sipConfigState.outbound.denied_users || []),
            ...(sipConfigState.outbound.denied_e164_prefixes || [])
        ]);
        if (String(sipPhoneBlockedTargets || '').trim() !== String(savedBlockedTargets || '').trim()) return true;
        const savedPeers = sipList(sipConfigState.inbound.trusted_peer_cidrs || []);
        const savedCallers = sipList(sipConfigState.inbound.allowed_callers || []);
        const savedDeniedCallers = sipList(sipConfigState.inbound.denied_callers || []);
        if (String(sipWizardTrustedPeers || '').trim() !== String(savedPeers || '').trim()) return true;
        if (String(sipWizardAllowedCallers || '').trim() !== String(savedCallers || '').trim()) return true;
        if (String(sipWizardDeniedCallers || '').trim() !== String(savedDeniedCallers || '').trim()) return true;
    }
    return false;
}

function sipHasUnsavedChanges() {
    return sipIsDirty();
}

function sipMarkClean() {
    sipAdvancedDirty = false;
    if (sipConfigState && document.querySelector('[data-sip]')) {
        // Baseline against the rendered form so defaulted display values do not
        // reappear as unsaved changes after a successful save/reload cycle.
        sipSavedState = sipComparable(sipRead({ forCompare: true }));
    } else if (sipConfigState) {
        sipSavedState = sipComparable(sipConfigState);
    } else {
        sipSavedState = '';
    }
    if (typeof setDirty === 'function') setDirty(false);
}

function sipBindEvents() {
    document.querySelectorAll('[data-sip]').forEach(input => {
        const onChange = () => sipNotifyDirty();
        input.addEventListener('input', onChange);
        input.addEventListener('change', onChange);
    });
    document.querySelector('.sip-advanced')?.addEventListener('toggle', event => {
        sipAdvancedOpen = event.currentTarget.open;
    });
    document.querySelector('[data-sip-action="save"]')?.addEventListener('click', () => { sipSave(); });
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
    document.querySelectorAll('[data-sip-activation-mode]').forEach(input => input.addEventListener('change', event => {
        sipWizardActivationMode = event.target.value === 'desktop' ? 'desktop' : 'registration';
        sipWizardMessage = '';
        sipRender();
    }));
    document.querySelector('[data-sip-phone-targets]')?.addEventListener('input', event => {
        sipPhoneTargets = event.target.value;
        sipNotifyDirty();
    });
    document.querySelector('[data-sip-wizard-trusted-peers]')?.addEventListener('input', event => {
        sipWizardTrustedPeers = event.target.value;
        sipNotifyDirty();
    });
    document.querySelector('[data-sip-wizard-allowed-callers]')?.addEventListener('input', event => {
        sipWizardAllowedCallers = event.target.value;
        sipNotifyDirty();
    });
    document.querySelector('[data-sip-wizard-denied-callers]')?.addEventListener('input', event => {
        sipWizardDeniedCallers = event.target.value;
        sipNotifyDirty();
    });
    document.querySelector('[data-sip-phone-blocked-targets]')?.addEventListener('input', event => {
        sipPhoneBlockedTargets = event.target.value;
        sipNotifyDirty();
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

function sipParsePhoneTargets(raw) {
    const users = [];
    const prefixes = [];
    for (const target of sipSplit(raw)) {
        if (/^\+[1-9][0-9]{0,14}$/.test(target)) {
            prefixes.push(target);
        } else if (/^[A-Za-z0-9_.!~*?'()%+\-]+$/.test(target)) {
            users.push(target);
        } else {
            throw new Error(t('config.sip.wizard.phone_invalid', { target }));
        }
    }
    if (!users.length && !prefixes.length) throw new Error(t('config.sip.wizard.phone_required'));
    return { users: [...new Set(users)], prefixes: [...new Set(prefixes)] };
}

function sipParseAllowlist(raw, emptyErrorKey) {
    const values = sipSplit(raw);
    if (!values.length) throw new Error(t(emptyErrorKey));
    return [...new Set(values)];
}

function sipReadDesktopActivationOptions() {
    const targets = sipParsePhoneTargets(sipPhoneTargets);
    const blockedTargets = sipPhoneBlockedTargets.trim() ? sipParsePhoneTargets(sipPhoneBlockedTargets) : { users: [], prefixes: [] };
    const trustedPeers = sipParseAllowlist(sipWizardTrustedPeers, 'config.sip.wizard.trusted_peers_required');
    const allowedCallers = sipParseAllowlist(sipWizardAllowedCallers, 'config.sip.wizard.allowed_callers_required');
    const deniedCallers = sipSplit(sipWizardDeniedCallers);
    return { targets, blockedTargets, trustedPeers, allowedCallers, deniedCallers };
}

function sipPatchOutboundPhoneConfig(base, targets, blockedTargets) {
    const next = JSON.parse(JSON.stringify(base));
    next.readonly = false;
    next.browser_media = next.browser_media || {};
    next.browser_media.enabled = true;
    next.outbound = next.outbound || {};
    next.outbound.allowed_domains = next.domain ? [next.domain] : [];
    next.outbound.allowed_users = targets.users.slice();
    next.outbound.allowed_e164_prefixes = targets.prefixes.slice();
    next.outbound.denied_users = (blockedTargets?.users || []).slice();
    next.outbound.denied_e164_prefixes = (blockedTargets?.prefixes || []).slice();
    next.permissions = next.permissions || {};
    next.permissions.originate_outbound = true;
    next.permissions.send_dtmf = true;
    next.permissions.agent_hangup = true;
    sipNormalizeOutboundPayload(next);
    return next;
}

function sipPatchDesktopPhoneConfig(base, options) {
    const next = sipPatchOutboundPhoneConfig(base, options.targets, options.blockedTargets);
    next.inbound = next.inbound || {};
    next.inbound.route = 'manual';
    next.inbound.trusted_peer_cidrs = options.trustedPeers.slice();
    next.inbound.allowed_callers = options.allowedCallers.slice();
    next.inbound.denied_callers = options.deniedCallers.slice();
    next.permissions.answer_inbound = true;
    return next;
}

async function sipPersistDesktopPhone(base, options) {
    const browserMediaWasEnabled = !!(base && base.browser_media && base.browser_media.enabled);
    const next = sipPatchDesktopPhoneConfig(base, options);
    const result = await sipRequest('/api/sip/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(next)
    });
    sipConfigState = sipNormalize(await sipRequest('/api/sip/config'));
    sipPhoneTargets = sipList([
        ...(sipConfigState.outbound.allowed_users || []),
        ...(sipConfigState.outbound.allowed_e164_prefixes || [])
    ]);
    sipPhoneBlockedTargets = sipList([
        ...(sipConfigState.outbound.denied_users || []),
        ...(sipConfigState.outbound.denied_e164_prefixes || [])
    ]);
    sipWizardTrustedPeers = sipList(sipConfigState.inbound.trusted_peer_cidrs || []);
    sipWizardAllowedCallers = sipList(sipConfigState.inbound.allowed_callers || []);
    sipWizardDeniedCallers = sipList(sipConfigState.inbound.denied_callers || []);
    sipMarkClean();
    const needsRestart = !browserMediaWasEnabled || !!(result && (result.needs_restart || result.status === 'pending'));
    return { result, needsRestart };
}

async function sipOfferRestart(needsRestart) {
    if (!needsRestart) return;
    const title = t('config.sip.restart_modal_title');
    const message = t('config.sip.restart_modal_message');
    let confirmed = false;
    if (typeof showModal === 'function') {
        confirmed = await showModal(title, message, true, {
            confirmText: t('config.sip.restart_modal_confirm'),
            cancelText: t('config.sip.restart_modal_later')
        });
    } else if (typeof showConfirm === 'function') {
        confirmed = await showConfirm(title, message);
    }
    if (!confirmed) return;
    if (typeof restartAuraGo === 'function') {
        await restartAuraGo(true);
    }
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
    let desktopOptions = null;
    if (sipWizardActivationMode === 'desktop') {
        try {
            desktopOptions = sipReadDesktopActivationOptions();
        } catch (error) {
            sipWizardMessage = error.message;
            const status = document.getElementById('sip-wizard-status');
            if (status) status.textContent = sipWizardMessage;
            return;
        }
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
        sipWizardPassword = '';
        sipWizardStep = 0;
        let needsRestart = !!(result && (result.needs_restart || result.status === 'pending'));
        if (desktopOptions) {
            const activated = await sipPersistDesktopPhone(sipConfigState, desktopOptions);
            needsRestart = needsRestart || activated.needsRestart;
            sipWizardMessage = needsRestart
                ? t('config.sip.restart_required')
                : t('config.sip.wizard.phone_enabled');
        } else {
            sipWizardMessage = needsRestart
                ? t('config.sip.restart_required')
                : t('config.sip.wizard.applied');
        }
        sipRender();
        sipMarkClean();
        await sipOfferRestart(needsRestart);
    } catch (error) {
        sipWizardMessage = error.message;
        document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
        button.disabled = false;
    }
}

async function sipEnableBrowserPhone() {
    const button = document.querySelector('[data-sip-wizard="enable-phone"]');
    if (!button) return;
    let options;
    try {
        options = sipReadDesktopActivationOptions();
    } catch (error) {
        sipWizardMessage = error.message;
        document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
        return;
    }
    button.disabled = true;
    sipWizardMessage = t('config.sip.saving');
    document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
    try {
        const activated = await sipPersistDesktopPhone(sipConfigState, options);
        sipWizardMessage = activated.needsRestart
            ? t('config.sip.restart_required')
            : t('config.sip.wizard.phone_enabled');
        sipRender();
        await sipOfferRestart(activated.needsRestart);
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

function sipRead(options) {
    const forCompare = !!(options && options.forCompare);
    const result = JSON.parse(JSON.stringify(sipConfigState));
    document.querySelectorAll('[data-sip]').forEach(input => {
        let value = input.type === 'checkbox' ? input.checked : input.value;
        if (input.type === 'number') value = Number(value);
        if (['media.codecs', 'inbound.trusted_peer_cidrs', 'inbound.allowed_callers', 'inbound.denied_callers', 'outbound.allowed_domains', 'outbound.denied_domains', 'outbound.allowed_users', 'outbound.denied_users', 'outbound.allowed_e164_prefixes', 'outbound.denied_e164_prefixes', 'voice.allowed_tools'].includes(input.dataset.sip)) value = sipSplit(value);
        sipAssign(result, input.dataset.sip, value);
    });
    if (!forCompare && sipAdvancedDirty) result.preset_id = '';
    sipNormalizeOutboundPayload(result);
    return result;
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

function sipActivationPanelOpen() {
    return !!document.querySelector('[data-sip-phone-targets], [data-sip-phone-blocked-targets], [data-sip-wizard-trusted-peers], [data-sip-wizard-allowed-callers], [data-sip-wizard-denied-callers]');
}

function sipActivationPanelFilled() {
    return !!(String(sipPhoneTargets || '').trim() || String(sipPhoneBlockedTargets || '').trim() || String(sipWizardTrustedPeers || '').trim() || String(sipWizardAllowedCallers || '').trim() || String(sipWizardDeniedCallers || '').trim());
}

async function sipSave() {
    const status = document.getElementById('sip-action-status');
    const save = document.querySelector('[data-sip-action="save"]');
    if (!document.querySelector('[data-sip]')) return false;
    if (save) save.disabled = true;
    if (status) status.textContent = t('config.sip.saving');
    try {
        let payload = sipRead();
        // Guided activation panel: full phone when all fields present, otherwise
        // outbound-only when destinations are provided.
        if (sipActivationPanelOpen() && String(sipPhoneTargets || '').trim()) {
            try {
                const hasInbound = String(sipWizardTrustedPeers || '').trim() && String(sipWizardAllowedCallers || '').trim();
                if (hasInbound) {
                    payload = sipPatchDesktopPhoneConfig(payload, sipReadDesktopActivationOptions());
                } else {
                    const blockedTargets = sipPhoneBlockedTargets.trim() ? sipParsePhoneTargets(sipPhoneBlockedTargets) : { users: [], prefixes: [] };
                    payload = sipPatchOutboundPhoneConfig(payload, sipParsePhoneTargets(sipPhoneTargets), blockedTargets);
                }
            } catch (error) {
                if (status) status.textContent = error.message;
                if (save) save.disabled = false;
                return false;
            }
        } else {
            // Destinations in expert mode should still become dialable: classify
            // E.164 values, keep the account domain, and enable outbound calling.
            sipNormalizeOutboundPayload(payload);
            const hasTargets = (payload.outbound.allowed_users && payload.outbound.allowed_users.length) ||
                (payload.outbound.allowed_e164_prefixes && payload.outbound.allowed_e164_prefixes.length);
            if (hasTargets && payload.domain && !payload.readonly) {
                payload.permissions = payload.permissions || {};
                payload.permissions.originate_outbound = true;
                payload.permissions.send_dtmf = true;
                payload.permissions.agent_hangup = true;
                if (!payload.browser_media) payload.browser_media = {};
                payload.browser_media.enabled = true;
            }
        }
        const saved = await sipRequest('/api/sip/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
        sipConfigState = sipNormalize(Object.prototype.hasOwnProperty.call(saved, 'enabled') ? saved : await sipRequest('/api/sip/config'));
        sipPhoneTargets = sipList([
            ...(sipConfigState.outbound.allowed_users || []),
            ...(sipConfigState.outbound.allowed_e164_prefixes || [])
        ]);
        sipPhoneBlockedTargets = sipList([
            ...(sipConfigState.outbound.denied_users || []),
            ...(sipConfigState.outbound.denied_e164_prefixes || [])
        ]);
        sipWizardTrustedPeers = sipList(sipConfigState.inbound.trusted_peer_cidrs || []);
        sipWizardAllowedCallers = sipList(sipConfigState.inbound.allowed_callers || []);
        if (!sipConfigState.preset_id) sipWizardStep = 1;
        const needsRestart = !!(saved && (saved.needs_restart || saved.status === 'pending'));
        sipRender();
        sipMarkClean();
        const nextStatus = document.getElementById('sip-action-status');
        if (nextStatus) {
            nextStatus.textContent = needsRestart
                ? t('config.sip.restart_required')
                : t('config.sip.saved');
        }
        await sipOfferRestart(needsRestart);
        return true;
    } catch (error) {
        if (status) status.textContent = error.message;
        if (save) save.disabled = false;
        return false;
    }
}

async function sipSaveUnsaved() {
    if (!sipHasUnsavedChanges()) return true;
    if (!document.querySelector('[data-sip]')) {
        // Section left without saving; cannot recover field values from the DOM.
        return false;
    }
    return sipSave();
}

function sipDiscardUnsaved() {
    if (sipConfigState) {
        sipPhoneTargets = sipList([
            ...(sipConfigState.outbound.allowed_users || []),
            ...(sipConfigState.outbound.allowed_e164_prefixes || [])
        ]);
        sipPhoneBlockedTargets = sipList([
            ...(sipConfigState.outbound.denied_users || []),
            ...(sipConfigState.outbound.denied_e164_prefixes || [])
        ]);
        sipWizardTrustedPeers = sipList(sipConfigState.inbound.trusted_peer_cidrs || []);
        sipWizardAllowedCallers = sipList(sipConfigState.inbound.allowed_callers || []);
        sipWizardDeniedCallers = sipList(sipConfigState.inbound.denied_callers || []);
    } else {
        sipPhoneTargets = '';
        sipWizardTrustedPeers = '';
        sipWizardAllowedCallers = '';
    }
    sipMarkClean();
}

window.sipHasUnsavedChanges = sipHasUnsavedChanges;
window.sipSaveUnsaved = sipSaveUnsaved;
window.sipDiscardUnsaved = sipDiscardUnsaved;

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
    sipWizardActivationMode = 'registration';
    sipWizardTrustedPeers = '';
    sipWizardAllowedCallers = '';
    sipWizardDeniedCallers = '';
    sipAdvancedDirty = false;
    sipAdvancedOpen = false;
    sipPhoneTargets = '';
    sipPhoneBlockedTargets = '';
    const content = document.getElementById('content');
    content.innerHTML = `<div class="cfg-section active"><div class="cfg-loading-state">${sipEsc(t('config.sip.loading'))}</div></div>`;
    try {
        const [configuration, catalog] = await Promise.all([
            sipRequest('/api/sip/config'),
            sipRequest('/api/sip/providers')
        ]);
        sipConfigState = sipNormalize(configuration);
        sipProviderCatalog = Array.isArray(catalog.providers) ? catalog.providers : [];
        sipPhoneTargets = sipList([
            ...(sipConfigState.outbound.allowed_users || []),
            ...(sipConfigState.outbound.allowed_e164_prefixes || [])
        ]);
        sipWizardTrustedPeers = sipList(sipConfigState.inbound.trusted_peer_cidrs || []);
        sipWizardAllowedCallers = sipList(sipConfigState.inbound.allowed_callers || []);
        sipWizardProviderID = sipConfigState.preset_id || '';
        sipWizardStep = sipConfigState.preset_id && sipProvider(sipConfigState.preset_id) ? 0 : 1;
        sipWizardActivationMode = sipDesktopPhoneReady(sipConfigState) ? 'desktop' : 'registration';
        sipRender();
        sipMarkClean();
    } catch (error) {
        content.innerHTML = `<div class="cfg-section active"><div class="rs-load-error">${sipEsc(error.message)}</div></div>`;
    }
}
