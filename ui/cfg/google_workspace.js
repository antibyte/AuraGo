// cfg/google_workspace.js — Google Workspace integration section module

async function renderGoogleWorkspaceSection(section) {
    const data = configData['google_workspace'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.enabled_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabledOn ? ' on' : ''}" data-path="google_workspace.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── ReadOnly toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.readonly_label')}</div>
        <div class="field-hint">${t('config.google_workspace.readonly_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${readonlyOn ? ' on' : ''}" data-path="google_workspace.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── OAuth2 Setup ──
    html += `<div style="margin-top:1.5rem;padding-top:1.2rem;border-top:1px solid var(--border-subtle);">
        <div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:0.5rem;">🔐 ${t('config.google_workspace.oauth_title')}</div>
        <div style="font-size:0.82rem;color:var(--text-secondary);line-height:1.6;margin-bottom:1rem;">
            ${t('config.google_workspace.oauth_desc')}
        </div>`;

    // Client ID
    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.client_id_label')}</div>
        <div class="field-hint">${t('config.google_workspace.client_id_hint')}</div>
        <input class="field-input" type="text" data-path="google_workspace.client_id" value="${escapeAttr(data.client_id || '')}" placeholder="123456789.apps.googleusercontent.com">
    </div>`;

    // Client Secret (vault)
    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.client_secret_label')} <span style="font-size:0.65rem;color:var(--warning);">🔒</span></div>
        <div class="field-hint">${t('config.google_workspace.client_secret_hint')}</div>
        <div style="display:flex;gap:0.5rem;align-items:center;">
            <div class="password-wrap" style="flex:1;">
                <input class="field-input" type="password" id="gw-secret-input" placeholder="GOCSPX-••••••••••" autocomplete="off">
                <button type="button" class="password-toggle" onclick="(function(){var i=document.getElementById('gw-secret-input');i.type=i.type==='password'?'text':'password';})()" title="Toggle visibility">👁</button>
            </div>
            <button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="gwSaveSecret()">💾 ${t('config.google_workspace.save_vault')}</button>
        </div>
        <div id="gw-secret-status" style="margin-top:0.4rem;font-size:0.78rem;"></div>
    </div>`;

    // OAuth Connect / Status
    html += `<div class="field-group">
        <div class="field-label">${t('config.google_workspace.connection_label')}</div>
        <div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">
            <button class="btn-save" id="gw-connect-btn" onclick="gwOAuthConnect()" style="padding:0.45rem 1rem;font-size:0.82rem;">🔗 ${t('config.google_workspace.connect_btn')}</button>
            <button class="btn-save" id="gw-disconnect-btn" onclick="gwOAuthDisconnect()" style="padding:0.45rem 1rem;font-size:0.82rem;background:var(--danger);">🔌 ${t('config.google_workspace.disconnect_btn')}</button>
            <button class="btn-save" id="gw-test-btn" onclick="gwTestConnection()" style="padding:0.45rem 1rem;font-size:0.82rem;">🧪 ${t('config.google_workspace.test_btn')}</button>
        </div>
        <div id="gw-oauth-status" style="margin-top:0.4rem;font-size:0.78rem;"></div>
        <div id="gw-manual-section" style="display:none;margin-top:0.8rem;padding:0.8rem;background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:6px;">
            <div style="font-size:0.82rem;color:var(--warning);font-weight:500;margin-bottom:0.4rem;">⚠️ ${t('config.google_workspace.oauth_manual_title')}</div>
            <div style="font-size:0.78rem;color:var(--text-secondary);line-height:1.5;margin-bottom:0.6rem;">${t('config.google_workspace.oauth_manual_hint')}</div>
            <div style="display:flex;gap:0.5rem;align-items:center;flex-wrap:wrap;">
                <input class="field-input" type="text" id="gw-manual-url" placeholder="http://localhost:8088/api/oauth/callback?code=…&state=…" style="flex:1;min-width:0;font-size:0.78rem;">
                <button class="btn-save" onclick="gwOAuthManual('google_workspace')" style="padding:0.45rem 0.8rem;font-size:0.78rem;white-space:nowrap;">${t('config.google_workspace.oauth_manual_btn')}</button>
            </div>
        </div>
    </div>`;

    html += `</div>`; // close OAuth setup section

    // ── Scope Toggles ──
    html += `<div style="margin-top:1.5rem;padding-top:1.2rem;border-top:1px solid var(--border-subtle);">
        <div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:0.5rem;">📋 ${t('config.google_workspace.scopes_title')}</div>
        <div style="font-size:0.82rem;color:var(--text-secondary);line-height:1.6;margin-bottom:1rem;">
            ${t('config.google_workspace.scopes_desc')}
        </div>`;

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
        html += `<div class="field-group" style="padding:0.3rem 0;">
            <div style="display:flex;align-items:center;gap:0.75rem;">
                <div class="toggle${on ? ' on' : ''}" data-path="google_workspace.${scope.key}" onclick="toggleBool(this)" style="flex-shrink:0;"></div>
                <div>
                    <div style="font-weight:500;font-size:0.88rem;">${scope.label}</div>
                    <div style="font-size:0.75rem;color:var(--text-secondary);">${scope.hint}</div>
                </div>
            </div>
        </div>`;
    }

    html += `</div>`; // close scopes section

    html += `</div>`; // close cfg-section
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Check OAuth status on load
    gwCheckOAuthStatus();
}

// ── Vault Helpers ────────────────────────────────────────────────────────

function gwSaveSecret() {
    const input = document.getElementById('gw-secret-input');
    const statusEl = document.getElementById('gw-secret-status');
    const secret = input ? input.value.trim() : '';

    if (!secret) {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = t('config.google_workspace.secret_empty');
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
            statusEl.style.color = 'var(--success)';
            statusEl.textContent = '✓ ' + t('config.google_workspace.secret_saved');
            if (input) input.value = '';
        } else {
            statusEl.style.color = 'var(--danger)';
            statusEl.textContent = '✗ ' + (res.message || t('config.google_workspace.secret_save_failed'));
        }
        setTimeout(() => { statusEl.textContent = ''; }, 4000);
    })
    .catch(e => {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = '✗ ' + e.message;
    });
}

// ── OAuth Flow ──────────────────────────────────────────────────────────

async function gwOAuthConnect() {
    const btn = document.getElementById('gw-connect-btn');
    const statusEl = document.getElementById('gw-oauth-status');
    btn.disabled = true;

    try {
        const resp = await fetch('/api/oauth/start?provider=google_workspace');
        const data = await resp.json();
        if (data.auth_url) {
            window.open(data.auth_url, '_blank', 'width=600,height=700');
            statusEl.style.color = 'var(--accent)';
            statusEl.textContent = t('config.google_workspace.oauth_waiting');
            const manualSection = document.getElementById('gw-manual-section');
            if (manualSection) manualSection.style.display = 'block';
            // Poll status every 3 seconds for up to 2 minutes
            let attempts = 0;
            const poll = setInterval(async () => {
                attempts++;
                if (attempts > 40) {
                    clearInterval(poll);
                    statusEl.textContent = t('config.google_workspace.oauth_timeout');
                    return;
                }
                try {
                    const sr = await fetch('/api/oauth/status?provider=google_workspace');
                    const sd = await sr.json();
                    if (sd.authorized) {
                        clearInterval(poll);
                        statusEl.style.color = 'var(--success)';
                        statusEl.textContent = '✓ ' + t('config.google_workspace.oauth_connected');
                    }
                } catch (_) {}
            }, 3000);
        } else {
            statusEl.style.color = 'var(--danger)';
            statusEl.textContent = '✗ ' + (data.message || t('config.google_workspace.oauth_failed'));
        }
    } catch (e) {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = '✗ ' + e.message;
    } finally {
        btn.disabled = false;
    }
}

async function gwOAuthManual(provider) {
    const urlInput = document.getElementById('gw-manual-url');
    const statusEl = document.getElementById('gw-oauth-status');
    const pastedURL = urlInput ? urlInput.value.trim() : '';
    if (!pastedURL) {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = '✗ ' + t('config.google_workspace.oauth_manual_failed') + 'URL fehlt.';
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
            statusEl.style.color = 'var(--success)';
            statusEl.textContent = '✓ ' + t('config.google_workspace.oauth_manual_success');
            const manualSection = document.getElementById('gw-manual-section');
            if (manualSection) manualSection.style.display = 'none';
        } else {
            statusEl.style.color = 'var(--danger)';
            statusEl.textContent = '✗ ' + t('config.google_workspace.oauth_manual_failed') + (data.message || '');
        }
    } catch (e) {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = '✗ ' + t('config.google_workspace.oauth_manual_failed') + e.message;
    }
}

async function gwOAuthDisconnect() {
    const statusEl = document.getElementById('gw-oauth-status');
    try {
        const resp = await fetch('/api/oauth/revoke?provider=google_workspace', { method: 'POST' });
        const data = await resp.json();
        if (data.status === 'ok' || data.success) {
            statusEl.style.color = 'var(--success)';
            statusEl.textContent = '✓ ' + t('config.google_workspace.oauth_disconnected');
        } else {
            statusEl.style.color = 'var(--danger)';
            statusEl.textContent = '✗ ' + (data.message || 'Failed');
        }
    } catch (e) {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = '✗ ' + e.message;
    }
}

async function gwCheckOAuthStatus() {
    const statusEl = document.getElementById('gw-oauth-status');
    try {
        const resp = await fetch('/api/oauth/status?provider=google_workspace');
        const data = await resp.json();
        if (data.authorized) {
            statusEl.style.color = 'var(--success)';
            let msg = '✓ ' + t('config.google_workspace.oauth_connected');
            if (data.expired) {
                msg += ' (' + t('config.google_workspace.token_expired') + ')';
                statusEl.style.color = 'var(--warning)';
            }
            if (data.has_refresh_token) {
                msg += ' — ' + t('config.google_workspace.has_refresh');
            }
            statusEl.textContent = msg;
        } else {
            statusEl.style.color = 'var(--text-secondary)';
            statusEl.textContent = t('config.google_workspace.oauth_not_connected');
        }
    } catch (_) {}
}

async function gwTestConnection() {
    const btn = document.getElementById('gw-test-btn');
    const statusEl = document.getElementById('gw-oauth-status');
    btn.disabled = true;
    btn.textContent = '⏳ Testing...';

    try {
        const resp = await fetch('/api/google-workspace/test');
        const data = await resp.json();
        if (data.status === 'ok') {
            statusEl.style.color = 'var(--success)';
            statusEl.textContent = '✓ ' + (data.message || 'Connection successful');
        } else {
            statusEl.style.color = 'var(--danger)';
            statusEl.textContent = '✗ ' + (data.message || 'Connection test failed');
        }
    } catch (e) {
        statusEl.style.color = 'var(--danger)';
        statusEl.textContent = '✗ ' + e.message;
    } finally {
        btn.disabled = false;
        btn.textContent = '🧪 ' + t('config.google_workspace.test_btn');
    }
}
