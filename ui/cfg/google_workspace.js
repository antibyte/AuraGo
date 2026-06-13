// cfg/google_workspace.js — Google Workspace integration section module

let _gwPollInterval = null;

function _gwClearPoll() {
    if (_gwPollInterval) { clearInterval(_gwPollInterval); _gwPollInterval = null; }
}

window.addEventListener('cfg:section-leave', _gwClearPoll);

function gwSetStatus(el, state, text) {
    if (!el) return;
    el.className = 'gw-status-line';
    if (state) el.classList.add('is-' + state);
    el.textContent = text;
}

async function renderGoogleWorkspaceSection(section) {
    const data = configData['google_workspace'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.enabled_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabledOn ? ' on' : ''}" data-path="google_workspace.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.readonly_label')}</div>
        <div class="field-help">${t('config.google_workspace.readonly_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${readonlyOn ? ' on' : ''}" data-path="google_workspace.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">🔐 ${t('config.google_workspace.oauth_title')}</div>
        <div class="field-group-desc">${t('config.google_workspace.oauth_desc')}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.client_id_label')}</div>
        <div class="field-help">${t('config.google_workspace.client_id_hint')}</div>
        <input class="field-input" type="text" data-path="google_workspace.client_id" value="${escapeAttr(data.client_id || '')}" placeholder="123456789.apps.googleusercontent.com">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.client_secret_label')} <span class="gw-vault-badge">🔒</span></div>
        <div class="field-help">${t('config.google_workspace.client_secret_hint')}</div>
        <div class="adg-password-row">
            <div class="password-wrap adg-password-input">
                <input class="field-input adg-password-input" type="password" id="gw-secret-input" value="${escapeAttr(cfgSecretValue(data.client_secret))}" placeholder="${escapeAttr(cfgSecretPlaceholder(data.client_secret, 'GOCSPX-••••••••••'))}" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
            <button class="btn-save adg-save-btn" onclick="gwSaveSecret()">💾 ${t('config.google_workspace.save_vault')}</button>
        </div>
        <div id="gw-secret-status" class="gw-status-line"></div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.connection_label')}</div>
        <div class="cfg-actions-row gw-actions-row">
            <button class="btn-save adg-save-btn" id="gw-connect-btn" onclick="gwOAuthConnect()">🔗 ${t('config.google_workspace.connect_btn')}</button>
            <button class="btn-save adg-save-btn gw-btn-disconnect" id="gw-disconnect-btn" onclick="gwOAuthDisconnect()">🔌 ${t('config.google_workspace.disconnect_btn')}</button>
            <button class="btn-save adg-test-btn" id="gw-test-btn" onclick="gwTestConnection()">🧪 ${t('config.google_workspace.test_btn')}</button>
        </div>
        <div id="gw-oauth-status" class="gw-status-line"></div>
        <div id="gw-manual-section" class="gw-manual-panel">
            <div class="gw-manual-title">⚠️ ${t('config.google_workspace.oauth_manual_title')}</div>
            <div class="gw-manual-hint">${t('config.google_workspace.oauth_manual_hint')}</div>
            <div class="gw-manual-row">
                <input class="field-input" type="text" id="gw-manual-url" placeholder="http://localhost:8088/api/oauth/callback?code=…&state=…">
                <button class="btn-save adg-save-btn" onclick="gwOAuthManual('google_workspace')">${t('config.google_workspace.oauth_manual_btn')}</button>
            </div>
        </div>
    </div>`;

    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">📋 ${t('config.google_workspace.scopes_title')}</div>
        <div class="field-group-desc">${t('config.google_workspace.scopes_desc')}</div>`;

    const scopes = [
        { key: 'gmail',          label: t('config.google_workspace.scope_gmail'),           hint: t('config.google_workspace.scope_gmail_hint') },
        { key: 'gmail_send',     label: t('config.google_workspace.scope_gmail_send'),      hint: t('config.google_workspace.scope_gmail_send_hint') },
        { key: 'calendar',       label: t('config.google_workspace.scope_calendar'),        hint: t('config.google_workspace.scope_calendar_hint') },
        { key: 'calendar_write', label: t('config.google_workspace.scope_calendar_write'),  hint: t('config.google_workspace.scope_calendar_write_hint') },
        { key: 'drive',          label: t('config.google_workspace.scope_drive'),            hint: t('config.google_workspace.scope_drive_hint') },
        { key: 'docs',           label: t('config.google_workspace.scope_docs'),             hint: t('config.google_workspace.scope_docs_hint') },
        { key: 'docs_write',     label: t('config.google_workspace.scope_docs_write'),       hint: t('config.google_workspace.scope_docs_write_hint') },
        { key: 'sheets',         label: t('config.google_workspace.scope_sheets'),           hint: t('config.google_workspace.scope_sheets_hint') },
        { key: 'sheets_write',   label: t('config.google_workspace.scope_sheets_write'),     hint: t('config.google_workspace.scope_sheets_write_hint') },
    ];

    for (const scope of scopes) {
        const on = data[scope.key] === true;
        html += `<div class="field-group gw-scope-group">
            <div class="gw-scope-row">
                <div class="toggle${on ? ' on' : ''}" data-path="google_workspace.${scope.key}" onclick="toggleBool(this)"></div>
                <div>
                    <div class="gw-scope-label">${scope.label}</div>
                    <div class="gw-scope-hint">${scope.hint}</div>
                </div>
            </div>
        </div>`;
    }

    html += `</div></div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    gwCheckOAuthStatus();
}

function gwSaveSecret() {
    const input = document.getElementById('gw-secret-input');
    const statusEl = document.getElementById('gw-secret-status');
    const secret = input ? input.value.trim() : '';

    if (!secret) {
        gwSetStatus(statusEl, 'danger', t('config.google_workspace.secret_empty'));
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'google_workspace_client_secret', value: secret })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            gwSetStatus(statusEl, 'success', '✓ ' + t('config.google_workspace.secret_saved'));
            cfgMarkSecretStored(input, 'google_workspace.client_secret');
        } else {
            gwSetStatus(statusEl, 'danger', '✗ ' + (res.message || t('config.google_workspace.secret_save_failed')));
        }
        setTimeout(() => { if (statusEl) statusEl.textContent = ''; }, 4000);
    })
    .catch(e => gwSetStatus(statusEl, 'danger', '✗ ' + e.message));
}

async function gwOAuthConnect() {
    const btn = document.getElementById('gw-connect-btn');
    const statusEl = document.getElementById('gw-oauth-status');
    btn.disabled = true;

    try {
        const resp = await fetch('/api/oauth/start?provider=google_workspace');
        const data = await resp.json();
        if (data.auth_url) {
            window.open(data.auth_url, '_blank', 'width=600,height=700,noopener,noreferrer');
            gwSetStatus(statusEl, 'accent', t('config.google_workspace.oauth_waiting'));
            const manualSection = document.getElementById('gw-manual-section');
            if (manualSection) manualSection.classList.add('is-visible');
            _gwClearPoll();
            let attempts = 0;
            _gwPollInterval = setInterval(async () => {
                attempts++;
                if (attempts > 40) {
                    _gwClearPoll();
                    gwSetStatus(statusEl, 'warning', t('config.google_workspace.oauth_timeout'));
                    return;
                }
                try {
                    const sr = await fetch('/api/oauth/status?provider=google_workspace');
                    const sd = await sr.json();
                    if (sd.authorized) {
                        _gwClearPoll();
                        gwSetStatus(statusEl, 'success', '✓ ' + t('config.google_workspace.oauth_connected'));
                    }
                } catch (_) {}
            }, 3000);
        } else {
            gwSetStatus(statusEl, 'danger', '✗ ' + (data.message || t('config.google_workspace.oauth_failed')));
        }
    } catch (e) {
        gwSetStatus(statusEl, 'danger', '✗ ' + e.message);
    } finally {
        btn.disabled = false;
    }
}

async function gwOAuthManual(provider) {
    const urlInput = document.getElementById('gw-manual-url');
    const statusEl = document.getElementById('gw-oauth-status');
    const pastedURL = urlInput ? urlInput.value.trim() : '';
    if (!pastedURL) {
        gwSetStatus(statusEl, 'danger', '✗ ' + t('config.google_workspace.oauth_manual_failed'));
        return;
    }
    try {
        const resp = await fetch('/api/oauth/manual', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: pastedURL, provider: provider })
        });
        const data = await resp.json();
        if (data.success) {
            gwSetStatus(statusEl, 'success', '✓ ' + t('config.google_workspace.oauth_manual_success'));
            const manualSection = document.getElementById('gw-manual-section');
            if (manualSection) manualSection.classList.remove('is-visible');
        } else {
            gwSetStatus(statusEl, 'danger', '✗ ' + t('config.google_workspace.oauth_manual_failed') + (data.message || ''));
        }
    } catch (e) {
        gwSetStatus(statusEl, 'danger', '✗ ' + t('config.google_workspace.oauth_manual_failed') + e.message);
    }
}

async function gwOAuthDisconnect() {
    const statusEl = document.getElementById('gw-oauth-status');
    try {
        const resp = await fetch('/api/oauth/revoke?provider=google_workspace', { method: 'POST' });
        const data = await resp.json();
        if (data.status === 'ok' || data.success) {
            gwSetStatus(statusEl, 'success', '✓ ' + t('config.google_workspace.oauth_disconnected'));
        } else {
            gwSetStatus(statusEl, 'danger', '✗ ' + (data.message || 'Failed'));
        }
    } catch (e) {
        gwSetStatus(statusEl, 'danger', '✗ ' + e.message);
    }
}

async function gwCheckOAuthStatus() {
    const statusEl = document.getElementById('gw-oauth-status');
    try {
        const resp = await fetch('/api/oauth/status?provider=google_workspace');
        const data = await resp.json();
        if (data.authorized) {
            let msg = '✓ ' + t('config.google_workspace.oauth_connected');
            let state = 'success';
            if (data.expired) {
                msg += ' (' + t('config.google_workspace.token_expired') + ')';
                state = 'warning';
            }
            if (data.has_refresh_token) {
                msg += ' — ' + t('config.google_workspace.has_refresh');
            }
            gwSetStatus(statusEl, state, msg);
        } else {
            gwSetStatus(statusEl, 'muted', t('config.google_workspace.oauth_not_connected'));
        }
    } catch (_) {}
}

async function gwTestConnection() {
    const btn = document.getElementById('gw-test-btn');
    const statusEl = document.getElementById('gw-oauth-status');
    btn.disabled = true;
    btn.textContent = t('config.google_workspace.testing');

    try {
        const resp = await fetch('/api/google-workspace/test');
        const data = await resp.json();
        if (data.status === 'ok') {
            const okMsg = data.message || t('config.google_workspace.test_success');
            gwSetStatus(statusEl, 'success', '✓ ' + okMsg);
        } else {
            const failMsg = data.message || t('config.google_workspace.test_failed');
            gwSetStatus(statusEl, 'danger', '✗ ' + failMsg);
        }
    } catch (e) {
        gwSetStatus(statusEl, 'danger', '✗ ' + e.message);
    } finally {
        btn.disabled = false;
        btn.textContent = '🧪 ' + t('config.google_workspace.test_btn');
    }
}