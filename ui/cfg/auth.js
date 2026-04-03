// cfg/auth.js — Web Configuration, Login Guard, TOTP & Vault section module

let _totpNewSecret = '';

function authSetStatus(el, level, text) {
    if (!el) return;
    el.classList.remove('is-success', 'is-error');
    if (level === 'success') el.classList.add('is-success');
    if (level === 'error') el.classList.add('is-error');
    el.textContent = text || '';
    setHidden(el, !text);
}

async function renderWebConfigSection(section) {
    const content = document.getElementById('content');
    content.innerHTML = '<div class="cfg-section active"><div class="auth-loading">' + t('config.common.loading') + '</div></div>';

    let authStatus = { enabled: false, password_set: false, totp_enabled: false };
    try {
        const resp = await fetch('/api/auth/status');
        authStatus = await resp.json();
    } catch (e) { /* auth endpoint unavailable */ }

    const webCfg = configData.web_config || {};
    const authCfg = configData.auth || {};
    const isWebEnabled = webCfg.enabled === true;
    const isAuthEnabled = authCfg.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Web Config Toggle ──
    html += `<div class="auth-section-title">
                🛡️ ${t('config.auth.web_config_title')}
            </div>`;
    const wcHelp = (helpTexts['web_config.enabled'] || {})[lang] || '';
    html += `<div class="field-group">
                <div class="field-label">${t('config.auth.config_page_enabled')}</div>
                ${wcHelp ? `<div class="field-help">${wcHelp}</div>` : ''}
                <div class="toggle-wrap">
                    <div class="toggle ${isWebEnabled ? 'on' : ''}" data-path="web_config.enabled" onclick="toggleBool(this)"></div>
                    <span class="toggle-label">${isWebEnabled ? t('config.common.active') : t('config.common.inactive')}</span>
                </div>
            </div>`;

    // ── Login-Schutz ──
    html += `<div class="auth-section-title auth-section-title-spaced">
                🔐 ${t('config.auth.login_guard_title')}
            </div>`;

    // Enable toggle
    const isHttpsEnabled = (configData.server && configData.server.https && configData.server.https.enabled) === true;
    const authHelpText = isHttpsEnabled ? (t('config.auth.https_forces_auth') || 'HTTPS is active. Login cannot be disabled.') : t('config.auth.enable_desc');

    html += `<div class="field-group">
                <div class="field-label">🔐 ${t('config.auth.enable_login_guard')}</div>
                <div class="field-help">${authHelpText}</div>
                <div class="toggle-wrap ${isHttpsEnabled ? 'auth-toggle-wrap-disabled' : ''}">
                    <div class="toggle ${isAuthEnabled ? 'on' : ''}" data-path="auth.enabled" onclick="toggleBool(this)"></div>
                    <span class="toggle-label">${isAuthEnabled ? t('config.common.active') : t('config.common.inactive')}</span>
                </div>
            </div>`;

    // Session / rate limit settings
    html += `<div class="auth-session-grid">
                <div class="field-group auth-field-group-flat">
                    <div class="field-label">${t('config.auth.session_hours')}</div>
                    <div class="field-help">${t('config.auth.session_validity')}</div>
                    <input class="field-input" type="number" min="1" max="8760" data-path="auth.session_timeout_hours" value="${authCfg.session_timeout_hours || 24}">
                </div>
                <div class="field-group auth-field-group-flat">
                    <div class="field-label">${t('config.auth.max_attempts')}</div>
                    <div class="field-help">${t('config.auth.before_lockout')}</div>
                    <input class="field-input" type="number" min="1" max="100" data-path="auth.max_login_attempts" value="${authCfg.max_login_attempts || 5}">
                </div>
                <div class="field-group auth-field-group-flat">
                    <div class="field-label">${t('config.auth.lockout_minutes')}</div>
                    <div class="field-help">${t('config.auth.lockout_duration')}</div>
                    <input class="field-input" type="number" min="1" max="10080" data-path="auth.lockout_minutes" value="${authCfg.lockout_minutes || 15}">
                </div>
            </div>`;

    // ── Password card
    html += `<div class="field-group">
                <div class="field-label">🔑 ${t('config.auth.password_label')}</div>
                <div class="field-help">${authStatus.password_set
            ? t('config.auth.password_is_set')
            : t('config.auth.password_not_set')
        }</div>
                <div class="auth-password-row">
                    <div class="password-wrap auth-password-wrap">
                        <input class="field-input" type="password" id="auth-new-pw"
                            placeholder="${t('config.auth.new_password_placeholder')}"
                            autocomplete="new-password"
                            onkeydown="if(event.key==='Enter')authSetPassword()">
                        <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                    </div>
                    <button onclick="authSetPassword()" class="auth-primary-btn">
                        ${authStatus.password_set ? t('config.auth.update_password') : t('config.auth.set_password')}
                    </button>
                </div>
                <div id="auth-pw-msg" class="auth-inline-msg is-hidden"></div>
            </div>`;

    // ── TOTP card
    html += `<div class="field-group">
                <div class="field-label">📱 ${t('config.auth.totp_title')}</div>
                <div class="field-help">${t('config.auth.totp_desc')}</div>
                <div id="auth-totp-status-area" class="auth-totp-status-area">`;

    if (authStatus.totp_enabled) {
        html += `<div class="auth-totp-active-row">
                    <span class="auth-totp-active-text">✅ ${t('config.auth.totp_active')}</span>
                    <button onclick="authTOTPDisable()" class="auth-totp-disable-btn">
                        ${t('config.auth.totp_disable')}
                    </button>
                </div>`;
    } else {
        html += `<div class="auth-totp-inactive-text">${t('config.auth.totp_not_active')}.</div>
                <button onclick="authTOTPStartSetup()" id="btn-totp-start" class="auth-totp-start-btn">
                    ${t('config.auth.totp_setup')}
                </button>
                <div id="auth-totp-setup" class="auth-totp-setup is-hidden">
                    <div class="auth-totp-instructions">
                        ${t('config.auth.totp_instructions')}
                    </div>
                    <div class="auth-totp-setup-row">
                        <div id="totp-qr" class="auth-totp-qr"></div>
                        <div class="auth-totp-manual-wrap">
                            <div class="auth-totp-manual-label">${t('config.auth.manual_key')}:</div>
                            <div id="totp-secret-display" class="auth-totp-secret"></div>
                        </div>
                    </div>
                    <div>
                        <div class="auth-totp-confirm-label">${t('config.auth.confirmation_code')}</div>
                        <div class="auth-totp-confirm-row">
                            <input id="totp-confirm-code" type="number" class="field-input auth-totp-confirm-input" placeholder="000000"
                                onkeydown="if(event.key==='Enter')authTOTPConfirm()">
                            <button onclick="authTOTPConfirm()" class="auth-primary-btn auth-totp-activate-btn">
                                ${t('config.auth.activate')}
                            </button>
                        </div>
                    </div>
                    <div id="auth-totp-msg" class="auth-inline-msg is-hidden"></div>
                </div>`;
    }

    html += '</div></div>'; // close totp-status-area + field-group

    // ── Security Audit panel (populated async after render) ──
    html += `<div class="auth-section-title auth-section-title-spaced">
                🔎 ${t('config.security.panel_title')}
            </div>`;
    html += '<div id="sec-audit-panel" class="auth-sec-audit-panel">';
    html += `<div class="auth-sec-audit-loading">${t('config.common.loading')}</div>`;
    html += '</div>';

    html += '</div>'; // close cfg-section

    content.innerHTML = html;
    attachChangeListeners();
    loadSecurityAuditPanel();
}

async function authSetPassword() {
    const pw = (document.getElementById('auth-new-pw') || {}).value || '';
    const msgEl = document.getElementById('auth-pw-msg');
    authSetStatus(msgEl, null, '');
    if (!pw || pw.length < 8) {
        authSetStatus(msgEl, 'error', t('config.auth.password_min_length'));
        return;
    }
    try {
        const resp = await fetch('/api/auth/password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ new_password: pw })
        });
        const data = await resp.json();
        if (resp.ok && data.ok) {
            authSetStatus(msgEl, 'success', '✓ ' + (data.message || t('config.common.saved')));
            document.getElementById('auth-new-pw').value = '';
        } else {
            authSetStatus(msgEl, 'error', data.error || t('config.common.error'));
        }
    } catch (e) {
        authSetStatus(msgEl, 'error', t('config.common.network_error'));
    }
}

async function authTOTPStartSetup() {
    try {
        const resp = await fetch('/api/auth/totp/setup');
        if (resp.status === 401) { showToast(t('config.auth.login_first'), 'warn'); return; }
        const data = await resp.json();
        _totpNewSecret = data.secret;
        document.getElementById('totp-secret-display').textContent = data.secret;
        setHidden(document.getElementById('auth-totp-setup'), false);
        setHidden(document.getElementById('btn-totp-start'), true);
        // Render QR code (qrcodejs library)
        const qrEl = document.getElementById('totp-qr');
        qrEl.innerHTML = '';
        if (typeof QRCode !== 'undefined') {
            // Use the real URI for the QR code — the authenticator app needs the actual secret.
            // Never render the raw secret as visible text (it's shown separately as _totpNewSecret).
            new QRCode(qrEl, { text: data.uri || '', width: 180, height: 180, colorDark: '#000000', colorLight: '#ffffff' });
        } else {
            // Fallback: display URI with masked secret as text only
            qrEl.classList.add('auth-totp-qr-fallback');
            qrEl.innerHTML = '<div class="auth-totp-uri-fallback">' + esc((data.uri || '').replace(/secret=[^&]*/, 'secret=***')) + '</div>';
        }
    } catch (e) {
        showToast(t('config.common.error') + ': ' + e.message, 'error');
    }
}

async function authTOTPConfirm() {
    const code = (document.getElementById('totp-confirm-code') || {}).value || '';
    const msgEl = document.getElementById('auth-totp-msg');
    try {
        const resp = await fetch('/api/auth/totp/confirm', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ secret: _totpNewSecret, code })
        });
        const data = await resp.json();
        if (resp.ok && data.ok) {
            authSetStatus(msgEl, 'success', '✓ ' + (data.message || t('config.auth.totp_activated')));
            setTimeout(() => selectSection('web_config'), 1200);
        } else {
            authSetStatus(msgEl, 'error', data.error || t('config.auth.invalid_code'));
        }
    } catch (e) {
        authSetStatus(msgEl, 'error', t('config.common.network_error'));
    }
}

async function authTOTPDisable() {
    if (!(await showConfirm(t('config.auth.totp_disable_confirm_title', {default: t('config.auth.totp_disable_confirm')}), t('config.auth.totp_disable_confirm')))) return;
    try {
        const resp = await fetch('/api/auth/totp', { method: 'DELETE' });
        const data = await resp.json();
        if (resp.ok) {
            selectSection('web_config');
        } else {
            showToast(data.error || t('config.common.error'), 'error');
        }
    } catch (e) {
        showToast(t('config.common.network_error'), 'error');
    }
}

// Vault delete functions are in config.html core (needed by vault modal HTML + renderField)

// ── Security Audit Panel ─────────────────────────────────────────────────────

async function loadSecurityAuditPanel() {
    const panel = document.getElementById('sec-audit-panel');
    if (!panel) return;
    try {
        const resp = await fetch('/api/security/hints');
        if (!resp.ok) {
            panel.innerHTML = `<div class="auth-sec-audit-loading">${t('config.common.error')}</div>`;
            return;
        }
        const data = await resp.json();
        const hints = data.hints || [];
        renderSecurityAuditPanel(panel, hints);
    } catch (_) {
        panel.innerHTML = `<div class="auth-sec-audit-loading">${t('config.common.network_error')}</div>`;
    }
}

function renderSecurityAuditPanel(panel, hints) {
    if (!hints.length) {
        panel.innerHTML = `<div class="auth-sec-ok">
            ✅ ${t('config.security.no_issues')}
        </div>`;
        return;
    }

    const fixable = hints.filter(h => h.auto_fixable);
    const criticals = hints.filter(h => h.severity === 'critical');

    const supportedSeverities = new Set(['critical', 'warning', 'info']);

    let html = '';
    if (fixable.length > 0) {
        const btnLabel = t('config.security.fix_all_auto').replace('{n}', fixable.length);
        html += `<div class="auth-sec-fix-wrap">
            <button id="btn-fix-all" onclick="securityHardenIds(${JSON.stringify(fixable.map(h => h.id))})"
                class="auth-sec-fix-all-btn">
                🔧 ${btnLabel}
            </button>
        </div>`;
    }

    for (const h of hints) {
        const sevKey = supportedSeverities.has(h.severity) ? h.severity : 'info';
        const sevLabel = t('config.security.severity.' + h.severity) || h.severity.toUpperCase();
        const desc = escapeHtml(h.description || '');
        let fixBtn = '';
        if (h.auto_fixable) {
            fixBtn = `<button onclick="securityHardenIds(['${esc(h.id)}'])"
                class="auth-sec-fix-now-btn">
                🔧 ${t('config.security.fix_now')}
            </button>`;
        }
        html += `<div class="auth-sec-hint-card is-${sevKey}">
            <div class="auth-sec-hint-head">
                <span class="auth-sec-hint-badge is-${sevKey}">${sevLabel}</span>
                <span class="auth-sec-hint-title">${escapeHtml(h.title || '')}</span>
            </div>
            <div class="auth-sec-hint-desc">${desc}</div>
            ${fixBtn}
        </div>`;
    }

    panel.innerHTML = html;
}

async function securityHardenIds(ids) {
    const btn = document.getElementById('btn-fix-all');
    if (btn) { btn.disabled = true; btn.textContent = t('config.security.applying') || 'Applying…'; }
    try {
        const resp = await fetch('/api/security/harden', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids })
        });
        const data = await resp.json();
        if (resp.ok) {
            await loadSecurityAuditPanel();
        } else {
            showToast((data && data.error) || t('config.common.error'), 'error');
            if (btn) { btn.disabled = false; btn.textContent = t('config.security.fix_all_auto').replace('{n}', ids.length); }
        }
    } catch (e) {
        showToast(t('config.common.network_error'), 'error');
        if (btn) { btn.disabled = false; }
    }
}
