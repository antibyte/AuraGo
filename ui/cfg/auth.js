// cfg/auth.js — Web Configuration, Login Guard, TOTP & Vault section module

let _totpNewSecret = '';

async function renderWebConfigSection(section) {
    const content = document.getElementById('content');
    content.innerHTML = '<div class="cfg-section active"><div style="text-align:center;padding:3rem;color:var(--text-secondary);">' + t('config.common.loading') + '</div></div>';

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
    html += `<div style="margin-bottom:0.5rem;font-weight:600;font-size:0.85rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.3rem;">
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
    html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.85rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.3rem;">
                🔐 ${t('config.auth.login_guard_title')}
            </div>`;

    // Enable toggle
    const isHttpsEnabled = (configData.server && configData.server.https && configData.server.https.enabled) === true;
    const authToggleStyle = isHttpsEnabled ? ' opacity: 0.6; pointer-events: none;' : '';
    const authHelpText = isHttpsEnabled ? (t('config.auth.https_forces_auth') || 'HTTPS is active. Login cannot be disabled.') : t('config.auth.enable_desc');

    html += `<div class="field-group">
                <div class="field-label">🔐 ${t('config.auth.enable_login_guard')}</div>
                <div class="field-help">${authHelpText}</div>
                <div class="toggle-wrap" style="${authToggleStyle}">
                    <div class="toggle ${isAuthEnabled ? 'on' : ''}" data-path="auth.enabled" onclick="toggleBool(this)"></div>
                    <span class="toggle-label">${isAuthEnabled ? t('config.common.active') : t('config.common.inactive')}</span>
                </div>
            </div>`;

    // Session / rate limit settings
    html += `<div style="display:grid;grid-template-columns:1fr 1fr 1fr;gap:0.75rem;margin-bottom:0.75rem;">
                <div class="field-group" style="margin-bottom:0;">
                    <div class="field-label">${t('config.auth.session_hours')}</div>
                    <div class="field-help">${t('config.auth.session_validity')}</div>
                    <input class="field-input" type="number" min="1" max="8760" data-path="auth.session_timeout_hours" value="${authCfg.session_timeout_hours || 24}">
                </div>
                <div class="field-group" style="margin-bottom:0;">
                    <div class="field-label">${t('config.auth.max_attempts')}</div>
                    <div class="field-help">${t('config.auth.before_lockout')}</div>
                    <input class="field-input" type="number" min="1" max="100" data-path="auth.max_login_attempts" value="${authCfg.max_login_attempts || 5}">
                </div>
                <div class="field-group" style="margin-bottom:0;">
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
                <div style="display:flex;gap:0.5rem;align-items:center;margin-top:0.4rem;">
                    <div class="password-wrap" style="flex:1;margin-bottom:0;">
                        <input class="field-input" type="password" id="auth-new-pw"
                            placeholder="${t('config.auth.new_password_placeholder')}"
                            autocomplete="new-password"
                            onkeydown="if(event.key==='Enter')authSetPassword()">
                        <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                    </div>
                    <button onclick="authSetPassword()" style="padding:0.5rem 1rem;background:linear-gradient(135deg,#2dd4bf,#0d9488);color:#fff;border:none;border-radius:8px;font-size:0.82rem;font-weight:600;cursor:pointer;white-space:nowrap;">
                        ${authStatus.password_set ? t('config.auth.update_password') : t('config.auth.set_password')}
                    </button>
                </div>
                <div id="auth-pw-msg" style="margin-top:0.5rem;font-size:0.8rem;display:none;"></div>
            </div>`;

    // ── TOTP card
    html += `<div class="field-group">
                <div class="field-label">📱 ${t('config.auth.totp_title')}</div>
                <div class="field-help">${t('config.auth.totp_desc')}</div>
                <div id="auth-totp-status-area" style="margin-top:0.5rem;">`;

    if (authStatus.totp_enabled) {
        html += `<div style="display:flex;align-items:center;gap:0.75rem;">
                    <span style="color:var(--success);font-weight:600;">✅ ${t('config.auth.totp_active')}</span>
                    <button onclick="authTOTPDisable()" style="padding:0.35rem 0.75rem;background:rgba(239,68,68,0.1);color:#f87171;border:1px solid rgba(239,68,68,0.3);border-radius:8px;font-size:0.78rem;font-weight:600;cursor:pointer;">
                        ${t('config.auth.totp_disable')}
                    </button>
                </div>`;
    } else {
        html += `<div style="margin-bottom:0.5rem;font-size:0.82rem;color:var(--text-secondary);">${t('config.auth.totp_not_active')}.</div>
                <button onclick="authTOTPStartSetup()" id="btn-totp-start" style="padding:0.45rem 1rem;background:var(--bg-glass);color:var(--text-primary);border:1px solid var(--border-accent);border-radius:8px;font-size:0.82rem;font-weight:600;cursor:pointer;">
                    ${t('config.auth.totp_setup')}
                </button>
                <div id="auth-totp-setup" style="display:none;margin-top:1.25rem;">
                    <div style="font-size:0.82rem;color:var(--text-secondary);margin-bottom:0.85rem;">
                        ${t('config.auth.totp_instructions')}
                    </div>
                    <div style="display:flex;gap:1.5rem;align-items:flex-start;flex-wrap:wrap;margin-bottom:1rem;">
                        <div id="totp-qr" style="background:#fff;padding:8px;border-radius:10px;flex-shrink:0;"></div>
                        <div style="flex:1;min-width:180px;">
                            <div style="font-size:0.72rem;font-weight:600;color:var(--text-secondary);text-transform:uppercase;letter-spacing:0.06em;margin-bottom:0.35rem;">${t('config.auth.manual_key')}:</div>
                            <div id="totp-secret-display" style="font-family:monospace;font-size:0.85rem;word-break:break-all;padding:0.6rem 0.75rem;background:var(--bg-glass);border:1px solid var(--border-subtle);border-radius:8px;letter-spacing:0.08em;"></div>
                        </div>
                    </div>
                    <div>
                        <div style="font-size:0.78rem;color:var(--text-secondary);margin-bottom:0.35rem;">${t('config.auth.confirmation_code')}</div>
                        <div style="display:flex;gap:0.5rem;align-items:center;">
                            <input id="totp-confirm-code" type="number" class="field-input" placeholder="000000"
                                style="font-family:monospace;letter-spacing:0.3em;text-align:center;font-size:1.1rem;max-width:160px;"
                                onkeydown="if(event.key==='Enter')authTOTPConfirm()">
                            <button onclick="authTOTPConfirm()" style="padding:0.5rem 1rem;background:linear-gradient(135deg,#2dd4bf,#0d9488);color:#fff;border:none;border-radius:8px;font-size:0.82rem;font-weight:600;cursor:pointer;">
                                ${t('config.auth.activate')}
                            </button>
                        </div>
                    </div>
                    <div id="auth-totp-msg" style="margin-top:0.5rem;font-size:0.8rem;display:none;"></div>
                </div>`;
    }

    html += '</div></div>'; // close totp-status-area + field-group

    // ── Security Audit panel (populated async after render) ──
    html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.85rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.3rem;">
                🔎 ${t('config.security.panel_title')}
            </div>`;
    html += '<div id="sec-audit-panel" style="margin-bottom:1rem;">';
    html += `<div style="color:var(--text-secondary);font-size:0.82rem;">${t('config.common.loading')}</div>`;
    html += '</div>';

    html += '</div>'; // close cfg-section

    content.innerHTML = html;
    attachChangeListeners();
    loadSecurityAuditPanel();
}

async function authSetPassword() {
    const pw = (document.getElementById('auth-new-pw') || {}).value || '';
    const msgEl = document.getElementById('auth-pw-msg');
    msgEl.style.display = 'none';
    if (!pw || pw.length < 8) {
        msgEl.style.display = '';
        msgEl.style.color = 'var(--danger)';
        msgEl.textContent = t('config.auth.password_min_length');
        return;
    }
    try {
        const resp = await fetch('/api/auth/password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ new_password: pw })
        });
        const data = await resp.json();
        msgEl.style.display = '';
        if (resp.ok && data.ok) {
            msgEl.style.color = 'var(--success)';
            msgEl.textContent = '✓ ' + (data.message || t('config.common.saved'));
            document.getElementById('auth-new-pw').value = '';
        } else {
            msgEl.style.color = 'var(--danger)';
            msgEl.textContent = data.error || t('config.common.error');
        }
    } catch (e) {
        msgEl.style.display = '';
        msgEl.style.color = 'var(--danger)';
        msgEl.textContent = t('config.common.network_error');
    }
}

async function authTOTPStartSetup() {
    try {
        const resp = await fetch('/api/auth/totp/setup');
        if (resp.status === 401) { alert(t('config.auth.login_first')); return; }
        const data = await resp.json();
        _totpNewSecret = data.secret;
        document.getElementById('totp-secret-display').textContent = data.secret;
        document.getElementById('auth-totp-setup').style.display = '';
        document.getElementById('btn-totp-start').style.display = 'none';
        // Render QR code (qrcodejs library)
        const qrEl = document.getElementById('totp-qr');
        qrEl.innerHTML = '';
        if (typeof QRCode !== 'undefined') {
            new QRCode(qrEl, { text: data.uri, width: 180, height: 180, colorDark: '#000000', colorLight: '#ffffff' });
        } else {
            // Fallback: display URI as text
            qrEl.style.cssText = 'background:none;padding:0;';
            qrEl.innerHTML = '<div style="font-size:0.65rem;word-break:break-all;max-width:220px;color:var(--text-primary);">' + esc(data.uri) + '</div>';
        }
    } catch (e) {
        alert(t('config.common.error') + ': ' + e.message);
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
        msgEl.style.display = '';
        if (resp.ok && data.ok) {
            msgEl.style.color = 'var(--success)';
            msgEl.textContent = '✓ ' + (data.message || t('config.auth.totp_activated'));
            setTimeout(() => selectSection('web_config'), 1200);
        } else {
            msgEl.style.color = 'var(--danger)';
            msgEl.textContent = data.error || t('config.auth.invalid_code');
        }
    } catch (e) {
        msgEl.style.display = '';
        msgEl.style.color = 'var(--danger)';
        msgEl.textContent = t('config.common.network_error');
    }
}

async function authTOTPDisable() {
    if (!confirm(t('config.auth.totp_disable_confirm'))) return;
    try {
        const resp = await fetch('/api/auth/totp', { method: 'DELETE' });
        const data = await resp.json();
        if (resp.ok) {
            selectSection('web_config');
        } else {
            alert(data.error || t('config.common.error'));
        }
    } catch (e) {
        alert(t('config.common.network_error'));
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
            panel.innerHTML = `<div style="color:var(--text-secondary);font-size:0.82rem;">${t('config.common.error')}</div>`;
            return;
        }
        const data = await resp.json();
        const hints = data.hints || [];
        renderSecurityAuditPanel(panel, hints);
    } catch (_) {
        panel.innerHTML = `<div style="color:var(--text-secondary);font-size:0.82rem;">${t('config.common.network_error')}</div>`;
    }
}

function renderSecurityAuditPanel(panel, hints) {
    if (!hints.length) {
        panel.innerHTML = `<div style="display:flex;align-items:center;gap:0.5rem;padding:0.6rem 0.8rem;background:rgba(45,212,191,0.08);border:1px solid rgba(45,212,191,0.25);border-radius:8px;font-size:0.83rem;color:var(--success);">
            ✅ ${t('config.security.no_issues')}
        </div>`;
        return;
    }

    const fixable = hints.filter(h => h.auto_fixable);
    const criticals = hints.filter(h => h.severity === 'critical');

    const sevColors = {
        critical: { bg: 'rgba(239,68,68,0.08)', border: 'rgba(239,68,68,0.3)', badge: '#f87171' },
        warning:  { bg: 'rgba(251,191,36,0.08)', border: 'rgba(251,191,36,0.3)', badge: '#fbbf24' },
        info:     { bg: 'rgba(99,179,237,0.08)',  border: 'rgba(99,179,237,0.3)',  badge: '#60a5fa' },
    };

    let html = '';
    if (fixable.length > 0) {
        const btnLabel = t('config.security.fix_all_auto').replace('{n}', fixable.length);
        html += `<div style="margin-bottom:0.75rem;">
            <button id="btn-fix-all" onclick="securityHardenIds(${JSON.stringify(fixable.map(h => h.id))})"
                style="padding:0.45rem 1rem;background:linear-gradient(135deg,#f97316,#dc2626);color:#fff;border:none;border-radius:8px;font-size:0.82rem;font-weight:600;cursor:pointer;">
                🔧 ${btnLabel}
            </button>
        </div>`;
    }

    for (const h of hints) {
        const c = sevColors[h.severity] || sevColors.info;
        const sevLabel = t('config.security.severity.' + h.severity) || h.severity.toUpperCase();
        const desc = esc(h.description);
        let fixBtn = '';
        if (h.auto_fixable) {
            fixBtn = `<button onclick="securityHardenIds(['${h.id}'])"
                style="margin-top:0.5rem;padding:0.3rem 0.7rem;background:rgba(255,255,255,0.08);color:var(--text-primary);border:1px solid var(--border-accent);border-radius:6px;font-size:0.77rem;font-weight:600;cursor:pointer;">
                🔧 ${t('config.security.fix_now')}
            </button>`;
        }
        html += `<div style="margin-bottom:0.5rem;padding:0.65rem 0.85rem;background:${c.bg};border:1px solid ${c.border};border-radius:8px;">
            <div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.3rem;">
                <span style="font-size:0.68rem;font-weight:700;padding:0.1rem 0.45rem;border-radius:4px;background:${c.badge};color:#0f172a;">${sevLabel}</span>
                <span style="font-size:0.83rem;font-weight:600;color:var(--text-primary);">${esc(h.title)}</span>
            </div>
            <div style="font-size:0.78rem;color:var(--text-secondary);">${desc}</div>
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
            alert((data && data.error) || t('config.common.error'));
            if (btn) { btn.disabled = false; btn.textContent = t('config.security.fix_all_auto').replace('{n}', ids.length); }
        }
    } catch (e) {
        alert(t('config.common.network_error'));
        if (btn) { btn.disabled = false; }
    }
}
