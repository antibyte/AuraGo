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
let sipWizardTestCode = '';
let sipWizardFieldErrors = {};
let sipWizardPasswordVisible = false;
let sipAdvancedDirty = false;
let sipAdvancedOpen = false;
let sipPhoneTargets = '';
let sipWizardOutboundScope = 'all';
let sipWizardInboundEnabled = false;
let sipWizardInboundScope = 'all';
let sipWizardCustomCallers = '';
let sipRuntimeStatus = null;
let sipConnectionVerified = false;
let sipNeedsRestart = false;
let sipDeleteConfirm = false;
let sipDeleteHistory = false;

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
        ${[
            ['config.sip.wizard.choose', 1],
            ['config.sip.account', 2],
            ['config.sip.wizard.calling', 3],
            ['config.sip.wizard.verify', 4]
        ].map(([key, step]) => `<li class="${active === step ? 'is-current' : (active > step ? 'is-complete' : '')}" ${active === step ? 'aria-current="step"' : ''}><span>${step}</span>${sipEsc(t(key))}</li>`).join('')}
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
    const provider = sipProvider(sipConfigState.preset_id) || {
        id: sipConfigState.preset_id || '',
        name: sipConfigState.display_name || sipConfigState.registrar || t('config.sip.profile.custom_provider'),
        fields: []
    };
    const fullReady = sipDesktopPhoneReady(sipConfigState);
    const outboundReady = sipOutboundPhoneReady(sipConfigState);
    const registered = sipConnectionVerified || !!(sipRuntimeStatus && sipRuntimeStatus.registered);
    const browserReady = !!sipConfigState.browser_media.enabled && !sipNeedsRestart;
    const headline = registered && outboundReady
        ? t('config.sip.profile.ready')
        : (registered ? t('config.sip.profile.connected') : t('config.sip.profile.attention'));
    const readiness = [
        [registered, 'config.sip.account', registered ? 'config.sip.profile.account_ready' : 'config.sip.profile.account_not_ready'],
        [outboundReady, 'config.sip.originate_outbound', outboundReady ? 'config.sip.profile.outbound_ready' : 'config.sip.profile.outbound_off'],
        [fullReady, 'config.sip.answer_inbound', fullReady ? 'config.sip.profile.inbound_ready' : 'config.sip.profile.inbound_off'],
        [browserReady, 'config.sip.browser_media', browserReady ? 'config.sip.profile.audio_ready' : (sipNeedsRestart ? 'config.sip.profile.audio_restart' : 'config.sip.profile.audio_off')]
    ];
    return `<article class="sip-profile-card">
        <header class="sip-profile-header">
            <div><span class="sip-eyebrow">${sipEsc(t('config.sip.wizard.configured'))}</span><h3>${sipEsc(provider.name)}</h3><p>${sipEsc(headline)}</p></div>
            <span class="sip-profile-state ${registered && outboundReady ? 'is-ready' : 'is-attention'}">${sipEsc(registered && outboundReady ? t('config.sip.profile.badge_ready') : t('config.sip.profile.badge_check'))}</span>
        </header>
        <dl class="sip-readiness-list">${readiness.map(([ready, label, value]) => `<div><dt>${sipEsc(t(label))}</dt><dd class="${ready ? 'is-ready' : 'is-muted'}"><span class="sip-readiness-dot" aria-hidden="true"></span>${sipEsc(t(value))}</dd></div>`).join('')}</dl>
        ${sipWizardTestCode ? `<div class="sip-test-diagnostic sip-profile-diagnostic" role="alert"><strong>${sipEsc(sipTestErrorMessage(sipWizardTestCode))}</strong><div><button type="button" class="btn btn-secondary" data-sip-profile="credentials">${sipEsc(t('config.sip.diagnostic.check_credentials'))}</button><button type="button" class="sip-text-action" data-sip-wizard="change">${sipEsc(t('config.sip.diagnostic.check_provider'))}</button></div></div>` : ''}
        ${sipNeedsRestart ? `<div class="sip-restart-banner" role="status"><div><strong>${sipEsc(t('config.sip.profile.restart_title'))}</strong><p>${sipEsc(t('config.sip.profile.restart_text'))}</p></div><button type="button" class="btn-save" data-sip-profile="restart">${sipEsc(t('config.sip.restart_modal_confirm'))}</button></div>` : ''}
        <div class="sip-profile-actions">
            <button type="button" class="btn-save" data-sip-profile="test">${sipEsc(t('config.sip.test'))}</button>
            <button type="button" class="btn btn-secondary" data-sip-profile="permissions">${sipEsc(t('config.sip.profile.permissions'))}</button>
            <button type="button" class="btn btn-secondary" data-sip-profile="credentials">${sipEsc(t('config.sip.profile.credentials'))}</button>
            <button type="button" class="sip-text-action" data-sip-wizard="change">${sipEsc(t('config.sip.wizard.change'))}</button>
            <button type="button" class="sip-text-action is-danger" data-sip-profile="delete">${sipEsc(t('config.sip.profile.delete'))}</button>
        </div>
        ${sipDeleteConfirm ? sipDeleteConfirmation() : ''}
    </article>`;
}

function sipDeleteConfirmation() {
    return `<section class="sip-delete-confirm" role="alertdialog" tabindex="-1" aria-labelledby="sip-delete-title" aria-describedby="sip-delete-text">
        <div><strong id="sip-delete-title">${sipEsc(t('config.sip.delete.title'))}</strong><p id="sip-delete-text">${sipEsc(t('config.sip.delete.text'))}</p></div>
        <label><input type="checkbox" data-sip-delete-history ${sipDeleteHistory ? 'checked' : ''}> <span>${sipEsc(t('config.sip.delete.history'))}</span></label>
        <div><button type="button" class="btn btn-secondary" data-sip-profile="delete-cancel">${sipEsc(t('config.sip.delete.cancel'))}</button><button type="button" class="btn-save sip-danger-button" data-sip-profile="delete-confirm">${sipEsc(t('config.sip.delete.confirm'))}</button></div>
    </section>`;
}

function sipProviderGroupsMarkup() {
    const query = sipWizardQuery.trim().toLocaleLowerCase();
    const priority = ['fritzbox', 'sipgate-de', 'easybell-voip', 'telekom-de', 'generic-pbx'];
    const filtered = sipProviderCatalog.filter(provider => {
        if (!query) return true;
        return `${provider.name} ${provider.region} ${sipProviderCategory(provider.category)}`.toLocaleLowerCase().includes(query);
    }).sort((left, right) => {
        const leftPriority = priority.indexOf(left.id);
        const rightPriority = priority.indexOf(right.id);
        if (leftPriority >= 0 || rightPriority >= 0) {
            if (leftPriority < 0) return 1;
            if (rightPriority < 0) return -1;
            return leftPriority - rightPriority;
        }
        return left.name.localeCompare(right.name);
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
            const fieldID = `sip-wizard-${field.key.replaceAll('_', '-')}`;
            const error = sipWizardFieldErrors[field.key] || '';
            return `<label class="field-group">
                <span class="field-label">${sipEsc(t(field.label_key))}${field.required ? ' *' : ''}</span>
                <span class="${field.secret ? 'sip-password-field' : ''}"><input id="${sipEsc(fieldID)}" class="field-input" type="${field.secret && !sipWizardPasswordVisible ? 'password' : 'text'}" data-sip-wizard-field="${sipEsc(field.key)}"
                    value="${sipEsc(value)}" placeholder="${sipEsc(placeholder)}" maxlength="${field.secret ? '1024' : '512'}"
                    ${field.secret ? 'autocomplete="new-password"' : 'autocomplete="off"'} ${error ? `aria-invalid="true" aria-describedby="${sipEsc(fieldID)}-error"` : ''}>
                    ${field.secret ? `<button type="button" class="sip-password-toggle" data-sip-wizard="password-toggle" aria-controls="${sipEsc(fieldID)}">${sipEsc(t(sipWizardPasswordVisible ? 'config.sip.password_hide' : 'config.sip.password_show'))}</button>` : ''}</span>
                    ${error ? `<small id="${sipEsc(fieldID)}-error" class="sip-field-error">${sipEsc(error)}</small>` : ''}
            </label>`;
        }).join('')}</div>
        <div class="sip-wizard-actions">
            <a class="sip-doc-link" href="${sipEsc(provider.documentation_url)}" target="_blank" rel="noopener noreferrer">${sipEsc(t('config.sip.wizard.documentation'))} ↗</a>
            <button type="button" class="btn-save" data-sip-wizard="calling">${sipEsc(t('config.sip.wizard.continue'))}</button>
        </div>
    </div>`;
}

function sipWizardCalling(provider) {
    const domesticAvailable = !!provider.domestic_region;
    const outboundOptions = [
        ['all', 'config.sip.wizard.scope_all', 'config.sip.wizard.scope_all_desc'],
        ...(domesticAvailable ? [['domestic', 'config.sip.wizard.scope_domestic', 'config.sip.wizard.scope_domestic_desc']] : []),
        ['custom', 'config.sip.wizard.scope_custom', 'config.sip.wizard.scope_custom_desc']
    ];
    return `<div class="sip-wizard-panel">
        <div class="sip-wizard-provider-heading">
            <button type="button" class="sip-back-button" data-sip-wizard="back" aria-label="${sipEsc(t('config.sip.wizard.back'))}">←</button>
            <div><span class="sip-eyebrow">${sipEsc(t('config.sip.wizard.calling'))}</span><h3>${sipEsc(t('config.sip.wizard.calling_title'))}</h3></div>
        </div>
        <fieldset class="sip-choice-group">
            <legend>${sipEsc(t('config.sip.wizard.outbound_title'))}</legend>
            <p>${sipEsc(t('config.sip.wizard.outbound_intro'))}</p>
            ${outboundOptions.map(([value, title, description]) => sipChoiceCard('sip-outbound-scope', value, sipWizardOutboundScope, title, description)).join('')}
            ${sipWizardOutboundScope === 'custom' ? `<label class="field-group sip-guided-values"><span class="field-label">${sipEsc(t('config.sip.wizard.custom_targets'))}</span><input class="field-input" type="text" data-sip-guided-targets value="${sipEsc(sipPhoneTargets)}" placeholder="${sipEsc(t('config.sip.wizard.custom_targets_placeholder'))}" autocomplete="off"><small>${sipEsc(t('config.sip.wizard.custom_targets_hint'))}</small></label>` : ''}
        </fieldset>
        <fieldset class="sip-choice-group sip-inbound-choice">
            <legend>${sipEsc(t('config.sip.wizard.inbound_title'))}</legend>
            <label class="sip-inbound-toggle"><span><strong>${sipEsc(t('config.sip.wizard.inbound_enable'))}</strong><small>${sipEsc(t('config.sip.wizard.inbound_enable_desc'))}</small></span><span class="toggle-wrap"><span class="toggle"><input type="checkbox" data-sip-guided-inbound ${sipWizardInboundEnabled ? 'checked' : ''}><span class="slider"></span></span></span></label>
            ${sipWizardInboundEnabled ? `<div class="sip-inbound-options">
                ${sipChoiceCard('sip-inbound-scope', 'all', sipWizardInboundScope, 'config.sip.wizard.callers_all', 'config.sip.wizard.callers_all_desc')}
                ${sipChoiceCard('sip-inbound-scope', 'custom', sipWizardInboundScope, 'config.sip.wizard.callers_custom', 'config.sip.wizard.callers_custom_desc')}
                ${sipWizardInboundScope === 'custom' ? `<label class="field-group sip-guided-values"><span class="field-label">${sipEsc(t('config.sip.wizard.custom_callers'))}</span><input class="field-input" type="text" data-sip-guided-callers value="${sipEsc(sipWizardCustomCallers)}" placeholder="${sipEsc(t('config.sip.wizard.custom_callers_placeholder'))}" autocomplete="off"></label>` : ''}
            </div>` : ''}
        </fieldset>
        <p class="sip-emergency-note">${sipEsc(t('config.sip.wizard.phone_warning'))}</p>
        <div class="sip-wizard-actions"><span></span><button type="button" class="btn-save" data-sip-wizard="review">${sipEsc(t('config.sip.wizard.continue'))}</button></div>
    </div>`;
}

function sipChoiceCard(name, value, selected, titleKey, descriptionKey) {
    return `<label class="sip-mode-card ${selected === value ? 'is-selected' : ''}">
        <input type="radio" name="${sipEsc(name)}" data-sip-guided-scope="${sipEsc(name)}" value="${sipEsc(value)}" ${selected === value ? 'checked' : ''}>
        <span><strong>${sipEsc(t(titleKey))}</strong><small>${sipEsc(t(descriptionKey))}</small></span>
    </label>`;
}

function sipWizardReview(provider) {
    const replacing = !!(sipConfigState.registrar && sipConfigState.preset_id !== provider.id);
    const outboundSummary = t(`config.sip.wizard.scope_${sipWizardOutboundScope}`);
    const inboundSummary = sipWizardInboundEnabled
        ? t(sipWizardInboundScope === 'custom' ? 'config.sip.wizard.callers_custom' : 'config.sip.wizard.callers_all')
        : t('config.sip.wizard.inbound_off');
    return `<div class="sip-wizard-panel">
        <div class="sip-wizard-provider-heading">
            <button type="button" class="sip-back-button" data-sip-wizard="back" aria-label="${sipEsc(t('config.sip.wizard.back'))}">←</button>
            <div><span class="sip-eyebrow">${sipEsc(t('config.sip.wizard.verify'))}</span><h3>${sipEsc(t('config.sip.wizard.verify_title'))}</h3></div>
        </div>
        <div class="sip-review-grid">
            <div><span>${sipEsc(t('config.sip.registrar'))}</span><strong>${sipEsc(provider.fields.some(field => field.key === 'server') ? (sipWizardValues.server || '') : t('config.sip.wizard.automatic'))}</strong></div>
            <div><span>${sipEsc(t('config.sip.username'))}</span><strong>${sipEsc(sipWizardValues.username || sipWizardValues.phone_number || '')}</strong></div>
            <div><span>${sipEsc(t('config.sip.originate_outbound'))}</span><strong>${sipEsc(outboundSummary)}</strong></div>
            <div><span>${sipEsc(t('config.sip.answer_inbound'))}</span><strong>${sipEsc(inboundSummary)}</strong></div>
        </div>
        <div class="sip-security-summary"><strong>${sipEsc(t('config.sip.wizard.verify_check_title'))}</strong><p>${sipEsc(t('config.sip.wizard.verify_check_text'))}</p></div>
        ${sipWizardTestCode ? `<div class="sip-test-diagnostic" role="alert"><strong>${sipEsc(sipTestErrorMessage(sipWizardTestCode))}</strong><div><button type="button" class="btn btn-secondary" data-sip-wizard="diagnostic-credentials">${sipEsc(t('config.sip.diagnostic.check_credentials'))}</button><button type="button" class="btn btn-secondary" data-sip-wizard="diagnostic-provider">${sipEsc(t('config.sip.diagnostic.check_provider'))}</button></div></div>` : ''}
        ${replacing ? `<label class="sip-replace-confirm"><input type="checkbox" data-sip-replace-confirm> <span>${sipEsc(t('config.sip.wizard.replace_confirm'))}</span></label>` : ''}
        <div class="sip-wizard-actions">
            <span></span>
            <button type="button" class="btn-save" data-sip-wizard="apply">${sipEsc(t('config.sip.wizard.save_test'))}</button>
        </div>
    </div>`;
}

function sipWizardMarkup() {
    if (sipWizardStep === 0 && sipConfigState.registrar) return sipWizardConfigured();
    const provider = sipProvider(sipWizardProviderID);
    let body = sipWizardProviderCards();
    if (sipWizardStep === 2 && provider) body = sipWizardCredentials(provider);
    if (sipWizardStep === 3 && provider) body = sipWizardCalling(provider);
    if (sipWizardStep === 4 && provider) body = sipWizardReview(provider);
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
            <div class="rs-save-row sip-expert-actions"><span id="sip-action-status" role="status" aria-live="polite"></span><button type="button" class="btn-save" data-sip-action="save">${sipEsc(t('config.sip.save'))}</button></div>
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
    </section>`;
    sipBindEvents();
    sipLoadStatus();
}

function sipNotifyDirty() {
    sipAdvancedDirty = true;
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
        delete sipWizardFieldErrors[input.dataset.sipWizardField];
        input.removeAttribute('aria-invalid');
    }));
    document.querySelector('[data-sip-wizard="password-toggle"]')?.addEventListener('click', () => {
        sipWizardPasswordVisible = !sipWizardPasswordVisible;
        sipRender();
        document.querySelector('[data-sip-wizard-field="password"]')?.focus();
    });
    document.querySelector('[data-sip-wizard="back"]')?.addEventListener('click', () => {
        sipWizardStep = sipWizardStep === 4 ? 3 : (sipWizardStep === 3 ? 2 : 1);
        sipWizardMessage = '';
        sipRender();
        sipFocusWizardHeading();
    });
    document.querySelector('[data-sip-wizard="calling"]')?.addEventListener('click', sipReviewProvider);
    document.querySelector('[data-sip-wizard="review"]')?.addEventListener('click', sipReviewCalling);
    document.querySelector('[data-sip-wizard="apply"]')?.addEventListener('click', sipApplyProvider);
    document.querySelector('[data-sip-wizard="diagnostic-credentials"]')?.addEventListener('click', () => {
        sipWizardStep = 2;
        sipWizardMessage = '';
        sipRender();
        const field = document.querySelector('[data-sip-wizard-field="password"], [data-sip-wizard-field]');
        if (field) field.focus();
        else sipFocusWizardHeading();
    });
    document.querySelector('[data-sip-wizard="diagnostic-provider"]')?.addEventListener('click', () => {
        sipWizardStep = 1;
        sipWizardMessage = '';
        sipRender();
        sipFocusWizardHeading();
    });
    document.querySelectorAll('[data-sip-guided-scope]').forEach(input => input.addEventListener('change', event => {
        if (event.target.dataset.sipGuidedScope === 'sip-outbound-scope') sipWizardOutboundScope = event.target.value;
        if (event.target.dataset.sipGuidedScope === 'sip-inbound-scope') sipWizardInboundScope = event.target.value;
        sipWizardMessage = '';
        sipRender();
    }));
    document.querySelector('[data-sip-guided-targets]')?.addEventListener('input', event => {
        sipPhoneTargets = event.target.value;
    });
    document.querySelector('[data-sip-guided-callers]')?.addEventListener('input', event => {
        sipWizardCustomCallers = event.target.value;
    });
    document.querySelector('[data-sip-guided-inbound]')?.addEventListener('change', event => {
        sipWizardInboundEnabled = event.target.checked;
        sipWizardMessage = '';
        sipRender();
    });
    sipBindProfileEvents();
}

function sipBindProfileEvents() {
    document.querySelectorAll('[data-sip-wizard="change"]').forEach(button => button.addEventListener('click', () => {
        sipWizardStep = 1;
        sipWizardProviderID = '';
        sipWizardMessage = '';
        sipWizardFieldErrors = {};
        sipRender();
        sipFocusWizardHeading();
    }));
    document.querySelector('[data-sip-profile="test"]')?.addEventListener('click', sipTestProfile);
    document.querySelector('[data-sip-profile="permissions"]')?.addEventListener('click', () => {
        const provider = sipProvider(sipConfigState.preset_id);
        if (!provider) {
            document.querySelector('.sip-advanced')?.setAttribute('open', '');
            sipAdvancedOpen = true;
            document.querySelector('.sip-advanced')?.scrollIntoView({ block: 'start', behavior: 'smooth' });
            return;
        }
        sipWizardProviderID = provider.id;
        sipPrefillWizardValues(provider);
        sipWizardStep = 3;
        sipWizardMessage = '';
        sipRender();
        sipFocusWizardHeading();
    });
    document.querySelector('[data-sip-profile="credentials"]')?.addEventListener('click', () => {
        const provider = sipProvider(sipConfigState.preset_id);
        if (!provider) {
            document.querySelector('.sip-advanced')?.setAttribute('open', '');
            sipAdvancedOpen = true;
            document.querySelector('.sip-advanced')?.scrollIntoView({ block: 'start', behavior: 'smooth' });
            return;
        }
        sipWizardProviderID = provider.id;
        sipPrefillWizardValues(provider);
        sipWizardStep = 2;
        sipWizardMessage = '';
        sipRender();
        sipFocusWizardHeading();
    });
    document.querySelector('[data-sip-profile="delete"]')?.addEventListener('click', () => {
        sipDeleteConfirm = true;
        sipRender();
        document.querySelector('.sip-delete-confirm')?.focus();
    });
    document.querySelector('[data-sip-profile="delete-cancel"]')?.addEventListener('click', () => {
        sipDeleteConfirm = false;
        sipDeleteHistory = false;
        sipRender();
        document.querySelector('[data-sip-profile="delete"]')?.focus();
    });
    document.querySelector('[data-sip-delete-history]')?.addEventListener('change', event => {
        sipDeleteHistory = event.target.checked;
    });
    document.querySelector('[data-sip-profile="delete-confirm"]')?.addEventListener('click', sipDeleteProfile);
    document.querySelector('[data-sip-profile="restart"]')?.addEventListener('click', async () => {
        if (typeof restartAuraGo === 'function') await restartAuraGo(true);
    });
}

function sipFocusWizardHeading() {
    const heading = document.querySelector('.sip-wizard-provider-heading h3, .sip-wizard-title h2');
    if (!heading) return;
    heading.setAttribute('tabindex', '-1');
    heading.focus();
}

function sipPrefillWizardValues(provider) {
    const source = sipConfigState || {};
    const values = {};
    provider.fields.forEach(field => {
        if (field.secret) return;
        if (field.key === 'server') values[field.key] = source.registrar || field.default || '';
        else if (field.key === 'phone_number') values[field.key] = source.username || field.default || '';
        else values[field.key] = source[field.key] || field.default || '';
    });
    sipWizardValues = values;
    sipWizardPassword = '';
    sipWizardFieldErrors = {};
}

function sipBindProviderCards() {
    document.querySelectorAll('[data-provider-id]').forEach(button => button.addEventListener('click', () => {
        sipWizardProviderID = button.dataset.providerId;
        sipWizardValues = {};
        sipWizardPassword = '';
        sipWizardPasswordVisible = false;
        sipWizardFieldErrors = {};
        sipWizardMessage = '';
        sipWizardStep = 2;
        sipRender();
        sipFocusWizardHeading();
    }));
}

function sipReviewProvider() {
    const provider = sipProvider(sipWizardProviderID);
    if (!provider) return;
    sipWizardFieldErrors = {};
    const missing = provider.fields.find(field => {
        if (!field.required) return false;
        if (field.secret) return !sipWizardPassword && !(sipConfigState.password_set && sipConfigState.preset_id === provider.id);
        return !String(sipWizardValues[field.key] ?? field.default ?? '').trim();
    });
    if (missing) {
        sipWizardMessage = t('config.sip.wizard.required', { field: t(missing.label_key) });
        sipWizardFieldErrors[missing.key] = sipWizardMessage;
        sipRender();
        document.querySelector(`[data-sip-wizard-field="${CSS.escape(missing.key)}"]`)?.focus();
        return;
    }
    provider.fields.forEach(field => {
        if (!field.secret && sipWizardValues[field.key] == null && field.default) sipWizardValues[field.key] = field.default;
    });
    sipWizardMessage = '';
    sipWizardStep = 3;
    sipRender();
    sipFocusWizardHeading();
}

function sipReviewCalling() {
    try {
        if (sipWizardOutboundScope === 'custom') sipParseGuidedValues(sipPhoneTargets, 'config.sip.wizard.custom_targets_required');
        if (sipWizardInboundEnabled && sipWizardInboundScope === 'custom') {
            sipParseGuidedValues(sipWizardCustomCallers, 'config.sip.wizard.custom_callers_required');
        }
    } catch (error) {
        sipWizardMessage = error.message;
        const status = document.getElementById('sip-wizard-status');
        if (status) status.textContent = sipWizardMessage;
        return;
    }
    sipWizardMessage = '';
    sipWizardStep = 4;
    sipRender();
    sipFocusWizardHeading();
}

function sipParseGuidedValues(raw, emptyErrorKey) {
    const values = sipSplit(raw);
    if (!values.length) throw new Error(t(emptyErrorKey));
    for (const value of values) {
        if (/[*?\r\n\x00]/.test(value) || !/^[A-Za-z0-9_.!~'()%+\-]+$/.test(value)) {
            throw new Error(t('config.sip.wizard.custom_value_invalid', { value }));
        }
    }
    return [...new Set(values)];
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
    let outboundValues = [];
    let inboundValues = [];
    try {
        if (sipWizardOutboundScope === 'custom') outboundValues = sipParseGuidedValues(sipPhoneTargets, 'config.sip.wizard.custom_targets_required');
        if (sipWizardInboundEnabled && sipWizardInboundScope === 'custom') {
            inboundValues = sipParseGuidedValues(sipWizardCustomCallers, 'config.sip.wizard.custom_callers_required');
        }
    } catch (error) {
        sipWizardMessage = error.message;
        const status = document.getElementById('sip-wizard-status');
        if (status) status.textContent = sipWizardMessage;
        return;
    }
    button.disabled = true;
    sipWizardTestCode = '';
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
                confirm_replace: replacing,
                activation: {
                    outbound_scope: sipWizardOutboundScope,
                    outbound_values: outboundValues,
                    inbound_scope: sipWizardInboundEnabled ? sipWizardInboundScope : 'disabled',
                    inbound_values: inboundValues
                }
            })
        });
        sipConfigState = sipNormalize(Object.prototype.hasOwnProperty.call(result, 'enabled') ? result : await sipRequest('/api/sip/config'));
        sipWizardPassword = '';
        sipNeedsRestart = !!(result && (result.needs_restart || result.status === 'pending'));
        sipWizardMessage = t('config.sip.testing');
        document.getElementById('sip-wizard-status').textContent = sipWizardMessage;
        sipConnectionVerified = false;
        await sipRequest('/api/sip/test', { method: 'POST' });
        sipConnectionVerified = true;
        sipRuntimeStatus = await sipRequest('/api/sip/status').catch(() => ({ registered: false, state: 'unknown' }));
        sipWizardStep = 0;
        sipWizardMessage = sipNeedsRestart ? t('config.sip.restart_required') : t('config.sip.wizard.phone_enabled');
        sipRender();
        sipMarkClean();
    } catch (error) {
        const field = error.body && error.body.field;
        if (field && sipProvider(sipWizardProviderID)?.fields.some(item => item.key === field)) {
            sipWizardFieldErrors[field] = error.message;
            sipWizardMessage = error.message;
            sipWizardStep = 2;
            sipRender();
            document.querySelector(`[data-sip-wizard-field="${CSS.escape(field)}"]`)?.focus();
            return;
        }
        sipWizardTestCode = error.code || '';
        sipWizardMessage = sipWizardTestCode ? sipTestErrorMessage(sipWizardTestCode) : error.message;
        if (sipWizardTestCode === 'authentication_failed') {
            sipWizardFieldErrors.password = sipWizardMessage;
        }
        sipWizardStep = 4;
        sipRender();
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
    sipNormalizeOutboundPayload(result);
    return result;
}

async function sipRequest(path, options) {
    const response = await fetch(path, Object.assign({ credentials: 'same-origin', cache: 'no-store' }, options || {}));
    const body = await response.json().catch(() => ({}));
    if (!response.ok) {
        const error = new Error(body.error || body.message || ('HTTP ' + response.status));
        error.code = body.code || '';
        error.retryable = !!body.retryable;
        error.body = body;
        throw error;
    }
    return body;
}

async function sipSave() {
    const status = document.getElementById('sip-action-status');
    const save = document.querySelector('[data-sip-action="save"]');
    if (!document.querySelector('[data-sip]')) return false;
    if (save) save.disabled = true;
    if (status) status.textContent = t('config.sip.saving');
    try {
        const payload = sipRead();
        // Expert destinations still become dialable when the administrator
        // explicitly leaves read-only mode.
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
        const saved = await sipRequest('/api/sip/config', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
        sipConfigState = sipNormalize(Object.prototype.hasOwnProperty.call(saved, 'enabled') ? saved : await sipRequest('/api/sip/config'));
        sipPhoneTargets = sipList([
            ...(sipConfigState.outbound.allowed_users || []),
            ...(sipConfigState.outbound.allowed_e164_prefixes || [])
        ]);
        if (sipConfigState.registrar) sipWizardStep = 0;
        const needsRestart = !!(saved && (saved.needs_restart || saved.status === 'pending'));
        sipNeedsRestart = sipNeedsRestart || needsRestart;
        sipRender();
        sipMarkClean();
        const nextStatus = document.getElementById('sip-action-status');
        if (nextStatus) {
            nextStatus.textContent = needsRestart
                ? t('config.sip.restart_required')
                : t('config.sip.saved');
        }
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
    } else {
        sipPhoneTargets = '';
    }
    sipMarkClean();
}

window.sipHasUnsavedChanges = sipHasUnsavedChanges;
window.sipSaveUnsaved = sipSaveUnsaved;
window.sipDiscardUnsaved = sipDiscardUnsaved;

function sipTestErrorMessage(code) {
    const known = ['dns_failed', 'unreachable', 'authentication_failed', 'rejected', 'timeout', 'failed'];
    return t(`config.sip.diagnostic.${known.includes(code) ? code : 'failed'}`);
}

async function sipTestProfile() {
    const button = document.querySelector('[data-sip-profile="test"]');
    if (button) button.disabled = true;
    sipWizardMessage = t('config.sip.testing');
    const message = document.getElementById('sip-wizard-status');
    if (message) message.textContent = sipWizardMessage;
    try {
        await sipRequest('/api/sip/test', { method: 'POST' });
        sipConnectionVerified = true;
        sipWizardTestCode = '';
        sipWizardMessage = t('config.sip.test_ok');
        await sipLoadStatus();
    } catch (error) {
        sipConnectionVerified = false;
        sipWizardTestCode = error.code || 'failed';
        sipWizardMessage = sipTestErrorMessage(sipWizardTestCode);
        sipRender();
    } finally {
        const currentButton = document.querySelector('[data-sip-profile="test"]');
        if (currentButton) currentButton.disabled = false;
    }
}

async function sipDeleteProfile() {
    const button = document.querySelector('[data-sip-profile="delete-confirm"]');
    if (button) button.disabled = true;
    sipWizardMessage = t('config.sip.delete.deleting');
    const status = document.getElementById('sip-wizard-status');
    if (status) status.textContent = sipWizardMessage;
    try {
        const result = await sipRequest(`/api/sip/config?purge_history=${sipDeleteHistory ? 'true' : 'false'}`, { method: 'DELETE' });
        sipConfigState = sipNormalize(await sipRequest('/api/sip/config'));
        sipRuntimeStatus = { registered: false, state: 'disabled' };
        sipConnectionVerified = false;
        sipNeedsRestart = !!(result && result.needs_restart);
        sipDeleteConfirm = false;
        sipDeleteHistory = false;
        sipWizardProviderID = '';
        sipWizardValues = {};
        sipWizardPassword = '';
        sipWizardStep = 1;
        sipWizardMessage = t('config.sip.delete.deleted');
        sipRender();
        sipMarkClean();
        sipFocusWizardHeading();
    } catch (error) {
        sipWizardMessage = error.message;
        const current = document.getElementById('sip-wizard-status');
        if (current) current.textContent = sipWizardMessage;
        if (button) button.disabled = false;
    }
}

function sipStatusLabel(state) {
    const known = ['disabled', 'registering', 'registered', 'failed', 'connecting', 'ringing', 'active', 'ending', 'ended'];
    return t(`config.sip.status.${known.includes(state) ? state : 'unknown'}`);
}

async function sipLoadStatus() {
    const banner = document.getElementById('sip-status');
    if (!banner) return;
    try {
        const status = await sipRequest('/api/sip/status');
        sipRuntimeStatus = status;
        if (status.registered) sipConnectionVerified = true;
        if (status.state === 'failed' || status.state === 'disabled') sipConnectionVerified = false;
        banner.className = `adg-status-banner ${status.registered ? 'is-success' : (status.state === 'failed' ? 'is-danger' : 'is-warning')}`;
        banner.textContent = t('config.sip.status_summary', { state: sipStatusLabel(status.state) });
        const profile = document.querySelector('.sip-profile-card');
        if (profile && sipWizardStep === 0) {
            profile.outerHTML = sipWizardConfigured();
            sipBindProfileEvents();
        }
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
    sipWizardTestCode = '';
    sipWizardFieldErrors = {};
    sipWizardPasswordVisible = false;
    sipWizardOutboundScope = 'all';
    sipWizardInboundEnabled = false;
    sipWizardInboundScope = 'all';
    sipWizardCustomCallers = '';
    sipAdvancedDirty = false;
    sipAdvancedOpen = false;
    sipPhoneTargets = '';
    const content = document.getElementById('content');
    content.innerHTML = `<div class="cfg-section active"><div class="cfg-loading-state">${sipEsc(t('config.sip.loading'))}</div></div>`;
    try {
        const [configuration, catalog, runtimeStatus, appState] = await Promise.all([
            sipRequest('/api/sip/config'),
            sipRequest('/api/sip/providers'),
            sipRequest('/api/sip/status').catch(() => null),
            sipRequest('/api/sip/app/state').catch(() => null)
        ]);
        sipConfigState = sipNormalize(configuration);
        sipRuntimeStatus = runtimeStatus;
        sipConnectionVerified = !!(runtimeStatus && runtimeStatus.registered);
        sipNeedsRestart = !!(appState && Array.isArray(appState.blockers) && appState.blockers.includes('browser_media_restart_required'));
        sipProviderCatalog = Array.isArray(catalog.providers) ? catalog.providers : [];
        sipPhoneTargets = sipList([
            ...(sipConfigState.outbound.allowed_users || []),
            ...(sipConfigState.outbound.allowed_e164_prefixes || [])
        ]);
        sipWizardProviderID = sipConfigState.preset_id || '';
        sipWizardStep = sipConfigState.registrar ? 0 : 1;
        sipWizardOutboundScope = (sipConfigState.outbound.allowed_users || []).includes('*') ? 'all' : 'custom';
        sipWizardInboundEnabled = sipConfigState.inbound.route === 'manual' && !!sipConfigState.permissions.answer_inbound;
        sipWizardInboundScope = (sipConfigState.inbound.allowed_callers || []).includes('*') ? 'all' : 'custom';
        sipWizardCustomCallers = sipList(sipConfigState.inbound.allowed_callers || []);
        sipRender();
        sipMarkClean();
    } catch (error) {
        content.innerHTML = `<div class="cfg-section active"><div class="rs-load-error">${sipEsc(error.message)}</div></div>`;
    }
}
