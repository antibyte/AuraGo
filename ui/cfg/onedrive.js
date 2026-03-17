// cfg/onedrive.js — OneDrive integration section module

async function renderOneDriveSection(section) {
    const data = configData['onedrive'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.enabled_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabledOn ? ' on' : ''}" data-path="onedrive.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── Read-Only toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.readonly_label')}</div>
        <div class="field-hint">${t('config.onedrive.readonly_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${readonlyOn ? ' on' : ''}" data-path="onedrive.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── Authentication section ──
    html += `<div style="margin-top:1.5rem;padding-top:1.2rem;border-top:1px solid var(--border-subtle);">
        <div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:0.6rem;">🔐 ${t('config.onedrive.auth_title')}</div>
        <div style="font-size:0.82rem;color:var(--text-secondary);line-height:1.6;margin-bottom:1.2rem;">
            ${t('config.onedrive.auth_desc')}
        </div>`;

    // Client ID
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.client_id_label')}</div>
        <div class="field-hint">${t('config.onedrive.client_id_hint')}</div>
        <input class="field-input" type="text" data-path="onedrive.client_id" value="${escapeAttr(data.client_id || '')}" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx">
    </div>`;

    // Tenant ID
    html += `<div class="field-group">
        <div class="field-label">${t('config.onedrive.tenant_id_label')}</div>
        <div class="field-hint">${t('config.onedrive.tenant_id_hint')}</div>
        <input class="field-input" type="text" data-path="onedrive.tenant_id" value="${escapeAttr(data.tenant_id || '')}" placeholder="common">
    </div>`;

    html += `</div>`; // close auth section

    // ── Connection ──
    html += `<div style="margin-top:1.5rem;padding-top:1.2rem;border-top:1px solid var(--border-subtle);">
        <div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:0.6rem;">🔗 ${t('config.onedrive.connection_label')}</div>
        <div style="font-size:0.82rem;color:var(--text-secondary);line-height:1.5;margin-bottom:1rem;">
            ${t('config.onedrive.auth_instructions')}
        </div>
        <div id="od-status-bar" style="margin-bottom:1rem;"></div>
        <div style="display:flex;gap:0.6rem;flex-wrap:wrap;align-items:center;">
            <button class="btn-save" id="od-connect-btn" onclick="odConnect()">${t('config.onedrive.connect_btn')}</button>
            <button class="btn-secondary" id="od-disconnect-btn" onclick="odDisconnect()" style="display:none;">${t('config.onedrive.disconnect_btn')}</button>
            <button class="btn-secondary" onclick="odTestConnection()">${t('config.onedrive.test_btn')}</button>
        </div>
        <!-- Device code display (shown after pressing Connect) -->
        <div id="od-device-code-box" style="display:none;margin-top:1.2rem;padding:1rem 1.2rem;border-radius:10px;background:var(--bg-tertiary);border:1px solid var(--border);">
            <div style="font-size:0.85rem;font-weight:600;color:var(--text-primary);margin-bottom:0.5rem;">📋 ${t('config.onedrive.device_code_title')}</div>
            <div id="od-device-code-value" style="font-size:2rem;font-weight:700;letter-spacing:0.25em;color:var(--accent);font-family:monospace;margin:0.5rem 0;"></div>
            <a id="od-device-code-link" href="#" target="_blank" rel="noopener" class="btn-save" style="display:inline-block;margin-top:0.4rem;text-decoration:none;font-size:0.82rem;">🌐 ${t('config.onedrive.device_code_link')}</a>
            <div style="margin-top:0.8rem;font-size:0.8rem;color:var(--text-secondary);">${t('config.onedrive.auth_waiting')}</div>
        </div>
        <div id="od-auth-result" style="margin-top:0.8rem;font-size:0.82rem;"></div>
    </div>`;

    html += `</div>`; // close cfg-section
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Load current connection status
    odRefreshStatus();
}

// ── OneDrive connection helpers ────────────────────────────────────────────

let _odPollTimer = null;

function odSetStatus(msg, color) {
    const bar = document.getElementById('od-status-bar');
    if (!bar) return;
    bar.innerHTML = `<div style="padding:0.5rem 0.9rem;border-radius:8px;background:var(--bg-tertiary);font-size:0.82rem;color:${color || 'var(--text-secondary)'};">${msg}</div>`;
}

async function odRefreshStatus() {
    try {
        const resp = await fetch('/api/onedrive/auth/status');
        const data = await resp.json();
        const disconnectBtn = document.getElementById('od-disconnect-btn');
        const connectBtn = document.getElementById('od-connect-btn');
        if (data.connected) {
            let statusText = t('config.onedrive.auth_connected');
            if (data.token_expired && data.has_refresh) statusText += ` (${t('config.onedrive.token_expired')}, ${t('config.onedrive.has_refresh')})`;
            else if (data.token_expired) statusText += ` (${t('config.onedrive.token_expired')})`;
            odSetStatus('✅ ' + statusText, 'var(--success)');
            if (disconnectBtn) disconnectBtn.style.display = '';
            if (connectBtn) connectBtn.style.display = 'none';
        } else {
            odSetStatus('⚪ ' + t('config.onedrive.auth_not_connected'), 'var(--text-secondary)');
            if (disconnectBtn) disconnectBtn.style.display = 'none';
            if (connectBtn) connectBtn.style.display = '';
        }
    } catch (_) {}
}

async function odConnect() {
    // First save config so client_id / tenant_id are available server-side
    await saveConfig();

    const resultDiv = document.getElementById('od-auth-result');
    const codeBox = document.getElementById('od-device-code-box');
    if (resultDiv) resultDiv.innerHTML = '';
    if (codeBox) codeBox.style.display = 'none';

    try {
        const resp = await fetch('/api/onedrive/auth/start', { method: 'POST' });
        const data = await resp.json();
        if (data.error) {
            if (resultDiv) resultDiv.innerHTML = `<span style="color:var(--danger);">❌ ${esc(data.error)}</span>`;
            return;
        }
        // Show device code
        const codeEl = document.getElementById('od-device-code-value');
        const linkEl = document.getElementById('od-device-code-link');
        if (codeEl) codeEl.textContent = data.user_code || '';
        if (linkEl) linkEl.href = data.verification_uri || 'https://microsoft.com/devicelogin';
        if (codeBox) codeBox.style.display = '';
        odSetStatus('⏳ ' + t('config.onedrive.auth_waiting'), 'var(--warning)');

        // Start polling
        if (_odPollTimer) clearInterval(_odPollTimer);
        const expiresAt = Date.now() + (data.expires_in || 900) * 1000;
        _odPollTimer = setInterval(() => odPoll(expiresAt), (data.interval || 5) * 1000);
    } catch (e) {
        if (resultDiv) resultDiv.innerHTML = `<span style="color:var(--danger);">❌ ${esc(String(e))}</span>`;
    }
}

async function odPoll(expiresAt) {
    if (Date.now() > expiresAt) {
        clearInterval(_odPollTimer);
        const codeBox = document.getElementById('od-device-code-box');
        if (codeBox) codeBox.style.display = 'none';
        odSetStatus('⏰ ' + t('config.onedrive.auth_timeout'), 'var(--danger)');
        return;
    }
    try {
        const resp = await fetch('/api/onedrive/auth/poll', { method: 'POST' });
        const data = await resp.json();
        switch (data.status) {
            case 'authorized':
                clearInterval(_odPollTimer);
                const codeBox = document.getElementById('od-device-code-box');
                if (codeBox) codeBox.style.display = 'none';
                odSetStatus('✅ ' + t('config.onedrive.auth_connected'), 'var(--success)');
                const disconnectBtn = document.getElementById('od-disconnect-btn');
                const connectBtn = document.getElementById('od-connect-btn');
                if (disconnectBtn) disconnectBtn.style.display = '';
                if (connectBtn) connectBtn.style.display = 'none';
                break;
            case 'declined':
                clearInterval(_odPollTimer);
                odSetStatus('🚫 ' + t('config.onedrive.auth_declined'), 'var(--danger)');
                break;
            case 'expired':
                clearInterval(_odPollTimer);
                odSetStatus('⏰ ' + t('config.onedrive.auth_timeout'), 'var(--danger)');
                break;
            case 'error':
                clearInterval(_odPollTimer);
                odSetStatus('❌ ' + t('config.onedrive.auth_failed') + (data.error ? ': ' + esc(data.error) : ''), 'var(--danger)');
                break;
            // pending / slow_down: keep polling
        }
    } catch (_) {}
}

async function odDisconnect() {
    if (!confirm(t('config.onedrive.disconnect_btn') + '?')) return;
    try {
        await fetch('/api/onedrive/auth/revoke', { method: 'DELETE' });
        odSetStatus('⚪ ' + t('config.onedrive.auth_disconnected'), 'var(--text-secondary)');
        const disconnectBtn = document.getElementById('od-disconnect-btn');
        const connectBtn = document.getElementById('od-connect-btn');
        if (disconnectBtn) disconnectBtn.style.display = 'none';
        if (connectBtn) connectBtn.style.display = '';
    } catch (_) {}
}

async function odTestConnection() {
    odSetStatus('🔄 Testing…', 'var(--text-secondary)');
    try {
        const resp = await fetch('/api/onedrive/test');
        const data = await resp.json();
        if (data.ok) {
            odSetStatus('✅ ' + t('config.onedrive.test_success') + (data.owner ? ` — ${esc(data.owner)}` : ''), 'var(--success)');
        } else {
            odSetStatus('❌ ' + t('config.onedrive.test_failed') + (data.error ? ': ' + esc(data.error) : ''), 'var(--danger)');
        }
    } catch (e) {
        odSetStatus('❌ ' + esc(String(e)), 'var(--danger)');
    }
}
