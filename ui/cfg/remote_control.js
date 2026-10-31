// cfg/remote_control.js — Remote Control config section module

let _rcSection = null;
let _rcDevicesOwner = null;
let _rcPollTimer = null;
let _rcFetch = null;

async function renderRemoteControlSection(section) {
    if (section) _rcSection = section; else section = _rcSection;
    clearTimeout(_rcPollTimer);
    if (_rcFetch) _rcFetch.abort();
    _rcFetch = null;
    _rcDevicesOwner = null;
    const cfg = configData.remote_control || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div class="cfg-group-title">${t('config.refresh.operation')}</div>`;
    html += `<div class="field-group">
        <span class="field-label">${t('config.remote_control.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="remote_control.enabled" onclick="toggleBool(this);setNestedValue(configData,'remote_control.enabled',this.classList.contains('on'));renderRemoteControlSection(null)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>📡</span>
            <div>
                <strong>${t('config.remote_control.disabled_notice')}</strong><br>
                <small>${t('config.remote_control.disabled_desc')}</small>
            </div>
        </div>`;
        html += `</div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    // ── Read-Only toggle ──
    const readOnly = cfg.read_only === true;
    html += `<div class="field-group">
        <span class="field-label">${t('config.remote_control.read_only_label')}</span>
        <div class="toggle ${readOnly ? 'on' : ''}" data-path="remote_control.read_only" onclick="toggleBool(this)"></div>
    </div>`;

    // ── Network Settings ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.network_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.network_desc')}</div>`;

    const connectionMode = cfg.connection_mode || 'auto';
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.remote_control.connection_mode_label')}</div>
        <div class="field-help">${t('config.remote_control.connection_mode_hint')}</div>
        <select class="field-select" data-path="remote_control.connection_mode"
            onchange="setNestedValue(configData,'remote_control.connection_mode',this.value);setDirty(true);renderRemoteControlSection(null)">
            <option value="auto" ${connectionMode === 'auto' ? 'selected' : ''}>${t('config.remote_control.connection_mode_auto')}</option>
            <option value="tailscale" ${connectionMode === 'tailscale' ? 'selected' : ''}>${t('config.remote_control.connection_mode_tailscale')}</option>
            <option value="manual" ${connectionMode === 'manual' ? 'selected' : ''}>${t('config.remote_control.connection_mode_manual')}</option>
        </select>
    </div>`;

    if (connectionMode === 'tailscale') {
        html += `<div class="field-group">
            <div class="field-label">${t('config.remote_control.tailscale_address_label')}</div>
            <div class="field-help">${t('config.remote_control.tailscale_address_hint')}</div>
            <input type="text" class="field-input" data-path="remote_control.tailscale_address"
                value="${escapeAttr(cfg.tailscale_address || '')}" placeholder="aurago.tailnet.ts.net">
        </div>`;
    }

    if (connectionMode === 'manual') {
        html += `<div class="field-group">
            <div class="field-label">${t('config.remote_control.supervisor_url_label')}</div>
            <div class="field-help">${t('config.remote_control.supervisor_url_hint')}</div>
            <input type="text" class="field-input" data-path="remote_control.supervisor_url"
                value="${escapeAttr(cfg.supervisor_url || '')}" placeholder="wss://aurago.example.com/api/remote/ws">
        </div>`;
    }

    html += `</div></div>`;

    // ── Security ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.security_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.security_desc')}</div>`;

    html += `<div class="field-help">${t('config.remote_control.pairing_token_desc')}</div>`;

    html += `<p class="field-help">${t('config.remote_control.security_hint')}</p>`;

    // Audit Log
    const auditLog = cfg.audit_log !== false;
    html += `<div class="field-group">
        <span class="field-label">${t('config.remote_control.audit_log_label')}</span>
        <div class="toggle ${auditLog ? 'on' : ''}" data-path="remote_control.audit_log" onclick="toggleBool(this)"></div>
    </div>`;

    // SSH Insecure Host Key
    const sshInsecure = cfg.ssh_insecure_host_key === true;
    html += `<div class="field-group rc-toggle-insecure${sshInsecure ? ' is-on' : ''}">
        <span class="field-label">${t('config.remote_control.ssh_insecure_host_key_label')}</span>
        <div class="toggle ${sshInsecure ? 'on' : ''}" data-path="remote_control.ssh_insecure_host_key" onclick="toggleBool(this)"></div>
    </div>`;
    html += `</div>`;

    // ── Limits ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.limits_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.limits_desc')}</div>`;

    // Max File Size MB
    const curMaxFile = cfg.max_file_size_mb || 50;
    const curPaths = (cfg.allowed_paths || []).join('\n');
    html += `<div class="field-grid two-cols">
        <div class="field-group">
            <div class="field-label">${t('config.remote_control.max_file_size_label')}</div>
            <input type="number" class="field-input" data-path="remote_control.max_file_size_mb" value="${curMaxFile}" min="1" max="500">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.remote_control.allowed_paths_label')}</div>
            <div class="field-help">${t('config.remote_control.allowed_paths_hint')}</div>
            <textarea class="field-input rc-paths-textarea" data-path="remote_control.allowed_paths" data-type="array-lines" rows="3">${escapeAttr(curPaths)}</textarea>
        </div>
    </div>`;
    html += `</div>`;

    // ── Pairing Token ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.pairing_token_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.pairing_token_desc')}</div>
        <div class="field-group">
            <div class="field-label">${t('config.remote_control.download_name_label')}</div>
            <input type="text" id="rc-enrollment-device-name" class="field-input" placeholder="${t('config.remote_control.download_name_placeholder')}">
        </div>
        <div class="cfg-actions-row">
            <button id="rc-token-generate-btn" class="btn-save" onclick="rcCreateEnrollmentToken()">
                ${t('config.remote_control.pairing_token_generate')}
            </button>
            ${cfgActionResult.html('rc-token-status')}
        </div>
        <div id="rc-token-result" class="rc-token-result"></div>
        <small class="cfg-help">${t('config.remote_control.pairing_token_hint')}</small>
    </div>`;

    // ── Client Download ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.download_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.download_desc')}</div>
        <div class="field-group">
            <div class="field-label">${t('config.remote_control.download_name_label')}</div>
            <input type="text" id="rc-device-name" class="field-input" placeholder="${t('config.remote_control.download_name_placeholder')}">
        </div>
        <div id="rc-platform-list" class="rc-platform-list">
            <span class="rc-muted-text">${t('config.remote_control.loading_platforms')}</span>
        </div>
    </div>`;

    // ── Connected Devices (live status) ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.devices_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.devices_desc')}</div>
        <button type="button" class="btn-save btn-secondary" onclick="loadRemoteDevices()">${t('config.remote_control.refresh')}</button>
        <div id="rc-devices-error" class="rc-error-text" role="status"></div>
        <div id="rc-devices-list" class="rc-devices-list">
            <span class="rc-muted-text">${t('config.remote_control.loading_devices')}</span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Load platform list and device list async
    loadRemotePlatforms();
    _rcDevicesOwner = document.getElementById('rc-devices-list');
    loadRemoteDevices();
}

async function loadRemotePlatforms() {
    const container = document.getElementById('rc-platform-list');
    if (!container) return;
    try {
        const resp = await fetch('/api/remote/platforms');
        if (!resp.ok) {
            container.innerHTML = `<span class="rc-error-text">${t('config.remote_control.platforms_error')}</span>`;
            return;
        }
        const platforms = await resp.json();
        const available = platforms.filter(p => p.available);
        if (!available.length) {
            container.innerHTML = `<span class="rc-muted-text">${t('config.remote_control.no_platforms')}</span>`;
            return;
        }

        const osIcons = { linux: '🐧', darwin: '🍎', windows: '🪟' };
        let html = '<div class="rc-platform-grid">';
        available.forEach(p => {
            const icon = osIcons[p.os] || '💻';
            const label = `${icon} ${p.os} / ${p.arch}`;
            html += `<button class="btn-save rc-platform-btn"
                onclick="rcDownload('${escapeAttr(p.os)}','${escapeAttr(p.arch)}')">
                ⬇ ${label}
            </button>`;
        });
        html += '</div>';
        container.innerHTML = html;
    } catch (e) {
        container.innerHTML = `<span class="rc-error-text">${escapeAttr(t('config.remote_control.error_prefix'))} ${escapeAttr(e && e.message ? e.message : String(e))}</span>`;
    }
}

function rcDownload(os, arch) {
    const nameEl = document.getElementById('rc-device-name');
    const name = nameEl ? nameEl.value.trim() : '';
    const qs = name ? `?name=${encodeURIComponent(name)}` : '';
    window.location.href = `/api/remote/download/${os}/${arch}${qs}`;
}

// The token itself stays in #rc-token-result (it is shown once); #rc-token-status carries the outcome.
async function rcCreateEnrollmentToken(pendingID) {
    const btn = document.getElementById('rc-token-generate-btn');
    const result = document.getElementById('rc-token-result');
    const nameEl = document.getElementById('rc-enrollment-device-name');
    if (!btn || !result) return;

    return cfgActionResult(btn, async () => {
        result.innerHTML = '';
        return fetch(pendingID ? `/api/remote/devices/${encodeURIComponent(pendingID)}/approve` : '/api/remote/enroll', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_name: nameEl ? nameEl.value.trim() : '' })
        });
    }, {
        target: 'rc-token-status',
        pending: t('config.remote_control.pairing_token_generating'),
        success: t('config.remote_control.pairing_token_ready'),
        failure: t('config.remote_control.pairing_token_failed'),
        // The approve/enroll answer is {status:"token_required", token}: a success. The token is never part of the result line.
        interpret: outcome => outcome.httpStatus < 400 && outcome.body && outcome.body.token ? { state: 'success', reason: '' } : { state: 'error', reason: outcome.httpStatus >= 400 ? outcome.reason : '' },
        onSuccess: async outcome => {
            const data = outcome.body;
            const expires = data.expires_at ? new Date(data.expires_at).toLocaleString() : '';
            result.innerHTML = `<div class="rc-token-card">
                <code id="rc-token-value" class="rc-token-value">${escapeAttr(data.token)}</code>
                <div class="cfg-actions-row">
                    <button id="rc-token-copy-btn" class="btn-save btn-secondary" onclick="rcCopyEnrollmentToken()">${t('config.remote_control.pairing_token_copy')}</button>
                    ${cfgActionResult.html('rc-token-copy-state')}
                </div>
                <div class="field-help">${t('config.remote_control.pairing_token_expires')} ${escapeAttr(expires)}</div>
            </div>`;
            if (pendingID) await loadRemoteDevices();
        }
    });
}

function rcCopyEnrollmentToken() {
    const tokenEl = document.getElementById('rc-token-value');
    const token = tokenEl ? tokenEl.textContent.trim() : '';
    if (!token) return;
    return cfgActionResult(document.getElementById('rc-token-copy-btn'), () => navigator.clipboard.writeText(token), {
        target: 'rc-token-copy-state',
        success: t('config.remote_control.pairing_token_copied'),
        failure: t('config.remote_control.pairing_token_failed'),
        clearAfter: 3000
    });
}

function rcText(key) { return t('config.remote_control.' + key); }

async function loadRemoteDevices() {
    const container = document.getElementById('rc-devices-list');
    if (!container || container !== _rcDevicesOwner || _rcFetch) return;
    clearTimeout(_rcPollTimer);
    const request = new AbortController();
    _rcFetch = request;
    const timeout = setTimeout(() => request.abort(), 8000);
    try {
        const resp = await fetch('/api/remote/devices', { signal: request.signal });
        if (!resp.ok) throw new Error(rcText('devices_error'));
        const devices = await resp.json();
        if (!Array.isArray(devices)) throw new Error(rcText('devices_error'));
        if (!container.isConnected || container !== _rcDevicesOwner) return;
        document.getElementById('rc-devices-error').textContent = '';
        if (!devices.length) {
            container.innerHTML = `<span class="rc-muted-text">${rcText('no_devices')}</span>`;
            return;
        }
        container.querySelector(':scope > .rc-muted-text')?.remove();
        const ids = new Set(devices.map(d => d.id));
        for (const card of container.querySelectorAll('[data-device-id]')) {
            if (!ids.has(card.dataset.deviceId)) card.remove();
        }
        for (const d of devices) {
            let card = [...container.children].find(el => el.dataset.deviceId === d.id);
            if (!card) {
                card = document.createElement('article');
                card.className = 'rc-managed-device';
                card.dataset.deviceId = d.id;
                card.innerHTML = '<div class="rc-device-summary"></div><div class="rc-device-editor"></div>';
                container.appendChild(card);
            }
            const state = ['confirmed', 'pending', 'error', 'server_enforced'].includes(d.config_state) ? d.config_state : 'pending';
            const status = d.status === 'revoked' ? 'revoked' : d.status === 'upgrade_required' ? 'upgrade_required' : d.is_connected ? 'status_connected' : 'status_offline';
            card.querySelector('.rc-device-summary').innerHTML = `
                <div class="rc-device-name">${escapeAttr(d.name || d.hostname || d.id)}</div>
                <div class="rc-device-subline">${escapeAttr(d.os || '')} ${escapeAttr(d.arch || '')} · ${rcText('version')}: ${escapeAttr(d.version || rcText('unknown'))}</div>
                <div class="rc-device-status ${d.is_connected ? 'is-connected' : ''}">${rcText(status)} · ${rcText(state)}</div>
                <div class="field-help">${rcText('effective')}: ${rcText(d.effective_read_only ? 'device_read_only' : 'write_enabled')}</div>
                <div class="field-help rc-wrap">${rcText('allowed_paths_label')}: ${escapeAttr((d.effective_allowed_paths || []).join(', ') || '—')}</div>
                <div class="field-help rc-wrap">${rcText('capabilities')}: ${escapeAttr((d.capabilities || []).join(', ') || rcText('unknown'))}</div>`;
            const editor = card.querySelector('.rc-device-editor');
            if (editor.querySelector('details[open]') || editor.contains(document.activeElement) || editor.dataset.busy === 'true') continue;
            const agodesk = (d.tags || []).includes('agodesk');
            editor.innerHTML = `<details><summary>${rcText('manage')}</summary>
                <label class="field-label rc-device-option"><input type="checkbox" data-rc-readonly ${d.read_only ? 'checked' : ''}> ${rcText('device_read_only')}</label>
                ${agodesk ? `<p class="field-help">${rcText('server_enforced')}</p>` : `<label class="field-label">${rcText('allowed_paths_label')}<textarea class="field-input rc-paths-textarea" data-rc-paths rows="3">${escapeAttr((d.allowed_paths || []).join('\n'))}</textarea></label><p class="field-help">${rcText('paths_inherit')}</p>`}
                <div class="cfg-actions-row rc-device-actions">
                    <button type="button" class="btn-save" data-rc-action="save">${rcText('save_device')}</button>
                    ${d.status === 'pending' ? `<button type="button" class="btn-save btn-secondary" data-approve-device data-rc-action="approve">${rcText('pairing_token_generate')}</button>` : ''}
                    ${d.status !== 'revoked' ? `<button type="button" class="btn-save btn-secondary" data-rc-action="revoke">${rcText('revoke')}</button>` : ''}
                    <button type="button" class="btn-save btn-secondary" data-rc-action="delete">${rcText('delete')}</button>
                </div><div class="field-help" data-rc-feedback role="status"></div>
            </details>`;
            editor.querySelectorAll('[data-rc-action]').forEach(button => button.addEventListener('click', () => rcDeviceAction(card, d, button.dataset.rcAction)));
        }
    } catch (error) {
        if (container.isConnected && container === _rcDevicesOwner) {
            document.getElementById('rc-devices-error').textContent = rcText('devices_error');
        }
    } finally {
        clearTimeout(timeout);
        if (_rcFetch === request) _rcFetch = null;
        if (container.isConnected && container === _rcDevicesOwner) rcScheduleRefresh(container);
    }
}

function rcScheduleRefresh(container) {
    clearTimeout(_rcPollTimer);
    _rcPollTimer = setTimeout(() => {
        if (!container.isConnected || container !== _rcDevicesOwner) return;
        if (document.visibilityState === 'visible' && container.getClientRects().length) loadRemoteDevices();
        else rcScheduleRefresh(container);
    }, 10000);
}

async function rcDeviceAction(card, device, action) {
    const editor = card.querySelector('.rc-device-editor');
    if (editor.dataset.busy === 'true') return;
    if (action === 'approve') { await rcCreateEnrollmentToken(device.id); return; }
    if (action !== 'save' && !await showConfirm(rcText(action), rcText('confirm_removal'))) return;
    if (!card.isConnected) return;
    editor.dataset.busy = 'true';
    const buttons = [...editor.querySelectorAll('button')];
    buttons.forEach(button => { button.disabled = true; });
    const feedback = editor.querySelector('[data-rc-feedback]');
    feedback.textContent = rcText('pending');
    const payload = { read_only: editor.querySelector('[data-rc-readonly]').checked };
    const paths = editor.querySelector('[data-rc-paths]');
    if (paths) payload.allowed_paths = paths.value.split('\n').map(p => p.trim()).filter(Boolean);
    const request = new AbortController();
    const timeout = setTimeout(() => request.abort(), 8000);
    try {
        const resp = await fetch(`/api/remote/devices/${encodeURIComponent(device.id)}${action === 'revoke' ? '/revoke' : ''}`, {
            signal: request.signal,
            method: action === 'save' ? 'PUT' : action === 'delete' ? 'DELETE' : 'POST',
            headers: { 'Content-Type': 'application/json' },
            ...(action === 'save' ? { body: JSON.stringify(payload) } : {})
        });
        const data = await resp.json();
        if (!resp.ok) throw new Error(data.error || rcText('error'));
        if (!card.isConnected) return;
        feedback.textContent = rcText('saved');
        if (action === 'delete') card.remove();
        await loadRemoteDevices();
    } catch (error) {
        if (card.isConnected) feedback.textContent = rcText('error') + ': ' + error.message;
    } finally {
        clearTimeout(timeout);
        editor.dataset.busy = 'false';
        buttons.forEach(button => { button.disabled = false; });
    }
}
