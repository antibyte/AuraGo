// cfg/onedrive.js — OneDrive integration section module

async function renderOneDriveSection(section) {
    const data = configData['onedrive'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.enabled_label')}</div>
        <div class="field-help">${t('config.onedrive.enabled_help')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabledOn ? ' on' : ''}" data-path="onedrive.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── Read-Only toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.readonly_label')}</div>
        <div class="field-help">${t('config.onedrive.readonly_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${readonlyOn ? ' on' : ''}" data-path="onedrive.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── Authentication section ──
    html += `<div class="od-section-block">
        <div class="od-section-title">🔐 ${t('config.onedrive.auth_title')}</div>
        <div class="od-section-desc od-section-desc-spacious">
            ${t('config.onedrive.auth_desc')}
        </div>`;

    // Client ID
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.client_id_label')}</div>
        <div class="field-help">${t('config.onedrive.client_id_hint')}</div>
        <input class="field-input" type="text" data-path="onedrive.client_id" value="${escapeAttr(data.client_id || '')}" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx">
    </div>`;

    // Tenant ID
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.tenant_id_label')}</div>
        <div class="field-help">${t('config.onedrive.tenant_id_hint')}</div>
        <input class="field-input" type="text" data-path="onedrive.tenant_id" value="${escapeAttr(data.tenant_id || '')}" placeholder="common">
    </div>`;

    html += `</div>`; // close auth section

    // ── Connection ──
    html += `<div class="od-section-block">
        <div class="od-section-title">🔗 ${t('config.onedrive.connection_label')}</div>
        <div class="od-section-desc">
            ${t('config.onedrive.auth_instructions')}
        </div>
        ${AuraConfigForm.statusBanner({ id: 'od-status-banner', state: enabledOn ? 'checking' : 'off' })}
        <div class="od-btn-row">
            <button class="btn-save" id="od-connect-btn" onclick="odConnect()">${t('config.onedrive.connect_btn')}</button>
            <button class="btn-secondary is-hidden" id="od-disconnect-btn" onclick="odDisconnect()">${t('config.onedrive.disconnect_btn')}</button>
            <button class="btn-secondary" id="od-test-btn" data-config-action-draft="true" onclick="odTestConnection()">${t('config.onedrive.test_btn')}</button>
            ${cfgActionResult.html('od-test-result')}
        </div>
        <!-- Device code display (shown after pressing Connect) -->
        <div id="od-device-code-box" class="od-device-code-box is-hidden">
            <div class="od-device-code-title">📋 ${t('config.onedrive.device_code_title')}</div>
            <div id="od-device-code-value" class="od-device-code-value"></div>
            <a id="od-device-code-link" href="#" target="_blank" rel="noopener" class="btn-save od-device-code-link">🌐 ${t('config.onedrive.device_code_link')}</a>
            <div class="od-device-code-hint">${t('config.onedrive.auth_waiting')}</div>
        </div>
        ${cfgActionResult.html('od-auth-result', 'od-auth-result')}
    </div>`;

    html += `</div>`; // close cfg-section
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Load current connection status
    odRefreshStatus();
}

// ── OneDrive connection helpers ────────────────────────────────────────────

let _odPollTimer = null;

function _odClearPoll() {
    if (_odPollTimer) { clearInterval(_odPollTimer); _odPollTimer = null; }
}

window.addEventListener('cfg:section-leave', _odClearPoll);

function odShowConnected(connected) {
    setHidden(document.getElementById('od-disconnect-btn'), !connected);
    setHidden(document.getElementById('od-connect-btn'), connected);
}

function odRefreshStatus() {
    const banner = document.getElementById('od-status-banner');
    if (!banner) return;
    if (!(configData.onedrive || {}).enabled) {
        cfgStatusBanner(banner, 'off');
        return;
    }
    return cfgStatusBanner.check(banner, () => fetch('/api/onedrive/auth/status'), body => {
        if (!body) return null;
        odShowConnected(!!body.connected);
        if (!body.connected) return { state: 'unconfigured', detail: t('config.onedrive.auth_not_connected') };
        if (body.token_expired) {
            const note = body.has_refresh ? `${t('config.onedrive.token_expired')}, ${t('config.onedrive.has_refresh')}` : t('config.onedrive.token_expired');
            return { state: 'degraded', detail: `${t('config.onedrive.auth_connected')} (${note})` };
        }
        return { state: 'connected', detail: t('config.onedrive.auth_connected') };
    });
}

async function odConnect() {
    // First save config so client_id / tenant_id are available server-side
    await saveConfig();

    const codeBox = document.getElementById('od-device-code-box');
    setHidden(codeBox, true);
    const banner = document.getElementById('od-status-banner');

    return cfgActionResult(document.getElementById('od-connect-btn'), () => fetch('/api/onedrive/auth/start', { method: 'POST' }), {
        target: 'od-auth-result',
        success: t('config.onedrive.auth_waiting'),
        failure: t('config.onedrive.auth_failed'),
        onSuccess: outcome => {
            const data = outcome.body || {};
            const codeEl = document.getElementById('od-device-code-value');
            const linkEl = document.getElementById('od-device-code-link');
            if (codeEl) codeEl.textContent = data.user_code || '';
            if (linkEl) linkEl.href = data.verification_uri || 'https://microsoft.com/devicelogin';
            setHidden(codeBox, false);
            cfgStatusBanner(banner, 'checking', { detail: t('config.onedrive.auth_waiting') });

            if (_odPollTimer) clearInterval(_odPollTimer);
            const expiresAt = Date.now() + (data.expires_in || 900) * 1000;
            _odPollTimer = setInterval(() => odPoll(expiresAt), (data.interval || 5) * 1000);
        }
    });
}

async function odPoll(expiresAt) {
    const banner = document.getElementById('od-status-banner');
    const resultLine = 'od-auth-result';
    if (Date.now() > expiresAt) {
        _odClearPoll();
        setHidden(document.getElementById('od-device-code-box'), true);
        cfgStatusBanner(banner, 'error', { detail: t('config.onedrive.auth_timeout'), retry: odConnect });
        return;
    }
    try {
        const resp = await fetch('/api/onedrive/auth/poll', { method: 'POST' });
        const data = await resp.json();
        switch (data.status) {
            case 'authorized':
                _odClearPoll();
                setHidden(document.getElementById('od-device-code-box'), true);
                cfgStatusBanner(banner, 'connected', { detail: t('config.onedrive.auth_connected') });
                cfgActionResult.show(resultLine, 'success', t('config.onedrive.auth_connected'));
                odShowConnected(true);
                break;
            case 'declined':
                _odClearPoll();
                cfgStatusBanner(banner, 'error', { detail: t('config.onedrive.auth_declined'), retry: odConnect });
                break;
            case 'expired':
                _odClearPoll();
                cfgStatusBanner(banner, 'error', { detail: t('config.onedrive.auth_timeout'), retry: odConnect });
                break;
            case 'error':
                _odClearPoll();
                cfgStatusBanner(banner, 'error', { detail: t('config.onedrive.auth_failed') + (data.error ? ': ' + data.error : ''), retry: odConnect });
                break;
            // pending / slow_down: keep polling
        }
    } catch (_) {}
}

async function odDisconnect() {
    if (!(await showConfirm(t('config.onedrive.disconnect_confirm_title'), t('config.onedrive.disconnect_btn') + '?'))) return { skipped: 'cancelled' };
    return cfgActionResult(document.getElementById('od-disconnect-btn'), () => fetch('/api/onedrive/auth/revoke', { method: 'DELETE' }), {
        target: 'od-auth-result',
        success: t('config.onedrive.auth_disconnected'),
        failure: t('config.feedback.failed'),
        interpret: outcome => (outcome.httpStatus > 0 && outcome.httpStatus < 400 ? { state: 'success' } : null),
        onSuccess: () => {
            cfgStatusBanner(document.getElementById('od-status-banner'), 'unconfigured', { detail: t('config.onedrive.auth_not_connected') });
            odShowConnected(false);
        }
    });
}

function odTestConnection() {
    const btn = document.getElementById('od-test-btn');
    return cfgActionResult(btn, () => fetch('/api/onedrive/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['onedrive']) })
    }), {
        target: 'od-test-result',
        success: outcome => t('config.onedrive.test_success') + (outcome.body && outcome.body.owner ? ' — ' + outcome.body.owner : ''),
        failure: t('config.onedrive.test_failed')
    });
}
