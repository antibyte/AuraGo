// cfg/google_workspace.js — Google Workspace integration section module

let _gwPollInterval = null;

function _gwClearPoll() {
    if (_gwPollInterval) { clearInterval(_gwPollInterval); _gwPollInterval = null; }
}

window.addEventListener('cfg:section-leave', _gwClearPoll);

// OAuth progress and outcomes go to the result line next to the connection buttons.
function gwSetStatus(el, state, text) {
    if (!el) return;
    cfgActionResult.show(el, state === 'danger' ? 'error' : (state === 'success' || state === 'warning' ? state : 'pending'), text);
}

async function renderGoogleWorkspaceSection(section) {
    const data = configData['google_workspace'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += AuraConfigForm.statusBanner({ id: 'gw-status-banner', state: enabledOn ? 'checking' : 'off' });

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
                <input class="field-input adg-password-input" type="password" id="gw-secret-input" data-path="google_workspace.client_secret" value="${escapeAttr(cfgSecretValue(data.client_secret))}" placeholder="${escapeAttr(cfgSecretPlaceholder(data.client_secret, 'GOCSPX-••••••••••'))}" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.connection_label')}</div>
        <div class="cfg-actions-row gw-actions-row">
            <button class="btn-save adg-save-btn" id="gw-connect-btn" onclick="gwOAuthConnect()">🔗 ${t('config.google_workspace.connect_btn')}</button>
            <button class="btn-save adg-save-btn gw-btn-disconnect" id="gw-disconnect-btn" onclick="gwOAuthDisconnect()">🔌 ${t('config.google_workspace.disconnect_btn')}</button>
            <button class="btn-save adg-test-btn" id="gw-test-btn" data-config-action-draft="true" onclick="gwTestConnection()">🧪 ${t('config.google_workspace.test_btn')}</button>
            ${cfgActionResult.html('gw-test-result')}
        </div>
        ${cfgActionResult.html('gw-oauth-status', 'gw-status-line')}
        <div id="gw-manual-section" class="gw-manual-panel">
            <div class="gw-manual-title">⚠️ ${t('config.google_workspace.oauth_manual_title')}</div>
            <div class="gw-manual-hint">${t('config.google_workspace.oauth_manual_hint')}</div>
            <div class="gw-manual-row">
                <input class="field-input" type="text" id="gw-manual-url" placeholder="http://localhost:8088/api/oauth/callback?code=…&state=…">
                <button class="btn-save adg-save-btn" id="gw-manual-btn" onclick="gwOAuthManual('google_workspace')">${t('config.google_workspace.oauth_manual_btn')}</button>
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
        { key: 'gmail_modify_labels', label: t('config.google_workspace.scope_gmail_modify_labels'), hint: t('config.google_workspace.scope_gmail_modify_labels_hint') },
        { key: 'calendar',       label: t('config.google_workspace.scope_calendar'),        hint: t('config.google_workspace.scope_calendar_hint') },
        { key: 'calendar_write', label: t('config.google_workspace.scope_calendar_write'),  hint: t('config.google_workspace.scope_calendar_write_hint') },
        { key: 'drive',          label: t('config.google_workspace.scope_drive'),            hint: t('config.google_workspace.scope_drive_hint') },
        { key: 'docs',           label: t('config.google_workspace.scope_docs'),             hint: t('config.google_workspace.scope_docs_hint') },
        { key: 'docs_write',     label: t('config.google_workspace.scope_docs_write'),       hint: t('config.google_workspace.scope_docs_write_hint') },
        { key: 'sheets',         label: t('config.google_workspace.scope_sheets'),           hint: t('config.google_workspace.scope_sheets_hint') },
        { key: 'sheets_write',   label: t('config.google_workspace.scope_sheets_write'),     hint: t('config.google_workspace.scope_sheets_write_hint') },
    ];

    for (const scope of scopes) {
        html += AuraConfigForm.toggle({ path: 'google_workspace.' + scope.key, value: data[scope.key], label: scope.label, help: scope.hint });
    }

    html += `</div></div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    gwCheckOAuthStatus();
}

function gwOAuthConnect() {
    const btn = document.getElementById('gw-connect-btn');
    return cfgActionResult(btn, () => fetch('/api/oauth/start?provider=google_workspace'), {
        target: 'gw-oauth-status',
        success: t('config.google_workspace.oauth_waiting'),
        failure: t('config.google_workspace.oauth_failed'),
        interpret: outcome => (outcome.body && outcome.body.auth_url ? null : { state: 'error', reason: (outcome.body && outcome.body.message) || outcome.reason || '' }),
        onSuccess: outcome => {
            const statusEl = document.getElementById('gw-oauth-status');
            window.open(outcome.body.auth_url, '_blank', 'width=600,height=700,noopener,noreferrer');
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
                        gwSetStatus(statusEl, 'success', t('config.google_workspace.oauth_connected'));
                        gwCheckOAuthStatus();
                    }
                } catch (_) {}
            }, 3000);
        }
    });
}

function gwOAuthManual(provider) {
    const urlInput = document.getElementById('gw-manual-url');
    const pastedURL = urlInput ? urlInput.value.trim() : '';
    const btn = document.getElementById('gw-manual-btn');
    if (!pastedURL) {
        gwSetStatus(document.getElementById('gw-oauth-status'), 'danger', t('config.google_workspace.oauth_manual_failed'));
        return Promise.resolve({ skipped: 'empty' });
    }
    return cfgActionResult(btn, () => fetch('/api/oauth/manual', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: pastedURL, provider: provider })
    }), {
        target: 'gw-oauth-status',
        success: t('config.google_workspace.oauth_manual_success'),
        failure: t('config.google_workspace.oauth_manual_failed'),
        interpret: outcome => (outcome.body && outcome.body.success ? null : { state: 'error', reason: (outcome.body && outcome.body.message) || outcome.reason || '' }),
        onSuccess: () => {
            const manualSection = document.getElementById('gw-manual-section');
            if (manualSection) manualSection.classList.remove('is-visible');
            gwCheckOAuthStatus();
        }
    });
}

async function gwOAuthDisconnect() {
    if (!(await showConfirm(t('config.google_workspace.confirm_disconnect_title'), t('config.google_workspace.confirm_disconnect_message')))) return { skipped: 'cancelled' };
    const btn = document.getElementById('gw-disconnect-btn');
    return cfgActionResult(btn, () => fetch('/api/oauth/revoke?provider=google_workspace', { method: 'DELETE' }), {
        target: 'gw-oauth-status',
        pending: t('config.google_workspace.disconnecting'),
        success: t('config.google_workspace.oauth_disconnected'),
        failure: t('config.feedback.failed'),
        onSuccess: () => gwCheckOAuthStatus()
    });
}

// Connection state banner; the result line below it only carries the outcome of the last action.
function gwCheckOAuthStatus() {
    const banner = document.getElementById('gw-status-banner');
    if (!banner) return;
    if (!(configData.google_workspace || {}).enabled) {
        cfgStatusBanner(banner, 'off');
        return;
    }
    return cfgStatusBanner.check(banner, () => fetch('/api/oauth/status?provider=google_workspace'), body => {
        if (!body) return null;
        if (!body.authorized) return { state: 'unconfigured', detail: t('config.google_workspace.oauth_not_connected') };
        const parts = [];
        if (body.has_refresh_token) parts.push(t('config.google_workspace.has_refresh'));
        if (body.expired) {
            parts.unshift(t('config.google_workspace.token_expired'));
            return { state: 'degraded', detail: parts.join(' — ') };
        }
        return { state: 'connected', detail: parts.join(' — ') };
    });
}

function gwTestConnection() {
    const btn = document.getElementById('gw-test-btn');
    return cfgActionResult(btn, () => fetch('/api/google-workspace/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['google_workspace']) })
    }), {
        target: 'gw-test-result',
        pending: t('config.google_workspace.testing'),
        success: t('config.google_workspace.test_success'),
        failure: t('config.google_workspace.test_failed')
    });
}
