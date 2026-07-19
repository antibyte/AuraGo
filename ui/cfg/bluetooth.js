// cfg/bluetooth.js — Linux BlueZ device and per-stream audio management.
let btDevices = [];

function renderBluetoothSection(section) {
    const data = configData.bluetooth || {};
    const enabled = data.enabled !== false;
    const readonly = data.readonly !== false;
    const allowPlayback = data.allow_playback === true;
    const backend = data.audio_backend || 'auto';
    const timeout = Number(data.scan_timeout_seconds || 10);

    const html = `
    <div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>
        <div id="bt-runtime-banner" class="adg-status-banner" role="status" aria-live="polite">${escapeHtml(t('config.bluetooth.loading'))}</div>

        <div class="field-group">
            <div class="field-label">${escapeHtml(t('config.bluetooth.enabled'))}</div>
            <div class="field-help">${escapeHtml(t('help.bluetooth.enabled'))}</div>
            <div class="toggle-wrap">
                <div class="toggle${enabled ? ' on' : ''}" data-path="bluetooth.enabled" onclick="toggleBool(this)"></div>
                <span class="toggle-label">${enabled ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
            </div>
        </div>

        <div class="field-group">
            <div class="field-label">${escapeHtml(t('config.bluetooth.readonly'))}</div>
            <div class="field-help">${escapeHtml(t('help.bluetooth.readonly'))}</div>
            <div class="toggle-wrap">
                <div class="toggle${readonly ? ' on' : ''}" data-path="bluetooth.readonly" onclick="toggleBool(this)"></div>
                <span class="toggle-label">${readonly ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
            </div>
        </div>

        <div class="field-group">
            <div class="field-label">${escapeHtml(t('config.bluetooth.allow_playback'))}</div>
            <div class="field-help">${escapeHtml(t('help.bluetooth.allow_playback'))}</div>
            <div class="toggle-wrap">
                <div class="toggle${allowPlayback ? ' on' : ''}" data-path="bluetooth.allow_playback" onclick="toggleBool(this)"></div>
                <span class="toggle-label">${allowPlayback ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
            </div>
        </div>

        <div class="field-group">
            <div class="field-label">${escapeHtml(t('config.bluetooth.audio_backend'))}</div>
            <div class="field-help">${escapeHtml(t('help.bluetooth.audio_backend'))}</div>
            <select class="field-select" data-path="bluetooth.audio_backend">
                <option value="auto"${backend === 'auto' ? ' selected' : ''}>${escapeHtml(t('config.bluetooth.backend_auto'))}</option>
                <option value="pipewire"${backend === 'pipewire' ? ' selected' : ''}>PipeWire</option>
                <option value="pulse"${backend === 'pulse' ? ' selected' : ''}>PulseAudio</option>
            </select>
        </div>

        <div class="field-group">
            <div class="field-label">${escapeHtml(t('config.bluetooth.scan_timeout'))}</div>
            <div class="field-help">${escapeHtml(t('help.bluetooth.scan_timeout'))}</div>
            <input class="field-input" type="number" min="1" max="60" data-path="bluetooth.scan_timeout_seconds" value="${timeout}">
        </div>

        <div class="field-group">
            <div class="field-label">${escapeHtml(t('config.bluetooth.default_device'))}</div>
            <div class="field-help">${escapeHtml(t('help.bluetooth.default_device'))}</div>
            <select id="bt-default-device" class="field-select" data-path="bluetooth.default_device">
                <option value="">${escapeHtml(t('config.bluetooth.default_automatic'))}</option>
            </select>
        </div>

        <div class="field-group">
            <div class="field-group-title">${escapeHtml(t('config.bluetooth.devices'))}</div>
            <div class="field-group-desc">${escapeHtml(t('config.bluetooth.devices_desc'))}</div>
            <div class="cc-actions-row">
                <button id="bt-reprobe-btn" class="btn-save cc-btn-secondary" onclick="btReprobe()">↻ ${escapeHtml(t('config.bluetooth.reprobe'))}</button>
                <button id="bt-discover-btn" class="btn-save cc-btn-primary" onclick="btDiscover()">⌕ ${escapeHtml(t('config.bluetooth.discover'))}</button>
                <button id="bt-test-btn" class="btn-save cc-btn-secondary" onclick="btTestAudio()">♪ ${escapeHtml(t('config.bluetooth.test_audio'))}</button>
                <button id="bt-stop-btn" class="btn-save cc-btn-secondary" onclick="btStopAudio()">■ ${escapeHtml(t('config.bluetooth.stop_audio'))}</button>
            </div>
            <div id="bt-action-result" class="adg-test-result" role="status" aria-live="polite"></div>
            <div id="bt-devices" class="cc-discover-area"></div>
        </div>

        <div id="bt-pin-modal" class="cc-modal-overlay is-hidden" onclick="btClosePINModal(event)">
            <div class="cc-modal-card" role="dialog" aria-modal="true" aria-labelledby="bt-pin-title" onclick="event.stopPropagation()">
                <div class="cc-modal-header">
                    <span id="bt-pin-title" class="cc-modal-title">${escapeHtml(t('config.bluetooth.pin_title'))}</span>
                    <button class="cc-modal-close" onclick="btClosePINModal()" aria-label="${escapeAttr(t('config.bluetooth.cancel'))}">✕</button>
                </div>
                <input id="bt-pin-address" type="hidden">
                <div class="field-group">
                    <div class="field-label">${escapeHtml(t('config.bluetooth.pin'))}</div>
                    <div class="field-help">${escapeHtml(t('config.bluetooth.pin_help'))}</div>
                    <input id="bt-pin-value" class="field-input" type="password" inputmode="numeric" autocomplete="one-time-code" maxlength="16">
                </div>
                <div class="cc-modal-actions">
                    <button class="btn-save cc-btn-secondary" onclick="btClosePINModal()">${escapeHtml(t('config.bluetooth.cancel'))}</button>
                    <button class="btn-save cc-btn-primary" onclick="btConfirmPair()">${escapeHtml(t('config.bluetooth.pair'))}</button>
                </div>
            </div>
        </div>
    </div>`;

    document.getElementById('content').innerHTML = html;
    btLoadStatus();
}

function btSavedConfigRequired() {
    if (typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges()) {
        showToast(t('config.bluetooth.save_first'), 'warn');
        return false;
    }
    return true;
}

async function btRequest(path, options) {
    const response = await fetch(path, options);
    const data = await response.json().catch(() => ({}));
    if (!response.ok || data.status === 'error') {
        throw new Error(data.message || t('config.bluetooth.request_failed'));
    }
    return data;
}

async function btLoadStatus() {
    const banner = document.getElementById('bt-runtime-banner');
    try {
        const data = await btRequest('/api/bluetooth/status');
        const status = data.status || {};
        btDevices = Array.isArray(data.devices) ? data.devices : [];
        btRenderStatus(status, data.playback || {});
        btRenderDevices();
        btPopulateDefaultDevice();
    } catch (error) {
        if (banner) {
            banner.className = 'adg-status-banner is-danger';
            banner.textContent = error.message || t('config.bluetooth.status_failed');
        }
        btDevices = [];
        btRenderDevices();
    }
}

function btRenderStatus(status, playback) {
    const banner = document.getElementById('bt-runtime-banner');
    if (!banner) return;
    const adapter = status.adapter || {};
    const audio = status.audio || {};
    if (status.usable) {
        const backend = audio.usable ? (audio.backend || '—') : t('config.bluetooth.audio_unavailable');
        banner.className = 'adg-status-banner is-success';
        banner.textContent = t('config.bluetooth.status_ready', {
            adapter: adapter.name || adapter.address || 'BlueZ',
            backend
        });
    } else {
        banner.className = 'adg-status-banner is-warning';
        banner.textContent = status.reason || t('config.bluetooth.status_unavailable');
    }
    const result = document.getElementById('bt-action-result');
    if (result && playback && playback.state && playback.state !== 'idle') {
        result.className = playback.state === 'error' ? 'adg-test-result is-danger' : 'adg-test-result is-success';
        result.textContent = t('config.bluetooth.playback_state', { state: playback.state });
    }
}

function btDeviceName(device) {
    return String(device.alias || device.name || device.address || t('config.bluetooth.unknown'));
}

function btRenderDevices() {
    const area = document.getElementById('bt-devices');
    if (!area) return;
    if (!btDevices.length) {
        area.innerHTML = `<div class="cc-empty">${escapeHtml(t('config.bluetooth.no_devices'))}</div>`;
        return;
    }
    const readonly = (configData.bluetooth || {}).readonly !== false;
    area.innerHTML = `<div class="cc-table-wrap">
        <table class="cc-table">
            <thead><tr class="cc-table-head">
                <th>${escapeHtml(t('config.bluetooth.col_device'))}</th>
                <th>${escapeHtml(t('config.bluetooth.col_address'))}</th>
                <th>${escapeHtml(t('config.bluetooth.col_state'))}</th>
                <th class="cc-th-actions">${escapeHtml(t('config.bluetooth.col_actions'))}</th>
            </tr></thead>
            <tbody>${btDevices.map(device => {
                const state = [
                    device.paired ? t('config.bluetooth.state_paired') : t('config.bluetooth.state_unpaired'),
                    device.connected ? t('config.bluetooth.state_connected') : t('config.bluetooth.state_disconnected'),
                    device.audio ? t('config.bluetooth.state_audio') : ''
                ].filter(Boolean).join(' · ');
                const address = escapeAttr(device.address || '');
                let actions = '';
                if (!readonly) {
                    if (!device.paired) {
                        actions += `<button class="btn-save cc-btn-compact" onclick="btOpenPINModal('${address}')">${escapeHtml(t('config.bluetooth.pair'))}</button>`;
                    }
                    if (device.paired && !device.connected) {
                        actions += `<button class="btn-save cc-btn-compact" onclick="btDeviceAction('connect','${address}')">${escapeHtml(t('config.bluetooth.connect'))}</button>`;
                    }
                    if (device.connected) {
                        actions += `<button class="btn-save cc-btn-compact" onclick="btDeviceAction('disconnect','${address}')">${escapeHtml(t('config.bluetooth.disconnect'))}</button>`;
                    }
                }
                if (!actions) actions = '—';
                return `<tr class="cc-device-row">
                    <td class="cc-cell cc-cell-name">${escapeHtml(btDeviceName(device))}</td>
                    <td class="cc-cell">${escapeHtml(device.address || '—')}</td>
                    <td class="cc-cell">${escapeHtml(state)}</td>
                    <td class="cc-cell cc-cell-actions">${actions}</td>
                </tr>`;
            }).join('')}</tbody>
        </table>
    </div>`;
}

function btPopulateDefaultDevice() {
    const select = document.getElementById('bt-default-device');
    if (!select) return;
    const current = String((configData.bluetooth || {}).default_device || '');
    select.innerHTML = `<option value="">${escapeHtml(t('config.bluetooth.default_automatic'))}</option>` +
        btDevices.map(device => `<option value="${escapeAttr(device.address)}"${device.address === current ? ' selected' : ''}>${escapeHtml(btDeviceName(device))} (${escapeHtml(device.address)})</option>`).join('');
    if (current && !btDevices.some(device => device.address === current)) {
        const option = document.createElement('option');
        option.value = current;
        option.selected = true;
        option.textContent = current;
        select.appendChild(option);
    }
}

async function btReprobe() {
    if (!btSavedConfigRequired()) return;
    await btRunAction('bt-reprobe-btn', async () => {
        await btRequest('/api/bluetooth/reprobe', { method: 'POST' });
        await btLoadStatus();
        showToast(t('config.bluetooth.reprobe_ok'), 'success');
    });
}

async function btDiscover() {
    if (!btSavedConfigRequired()) return;
    const timeout = Number((configData.bluetooth || {}).scan_timeout_seconds || 10);
    await btRunAction('bt-discover-btn', async () => {
        const data = await btRequest('/api/bluetooth/discover', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ timeout_seconds: timeout })
        });
        btDevices = Array.isArray(data.devices) ? data.devices : [];
        btRenderDevices();
        btPopulateDefaultDevice();
        showToast(t('config.bluetooth.discover_ok', { count: btDevices.length }), 'success');
    });
}

function btOpenPINModal(address) {
    if (!btSavedConfigRequired()) return;
    const modal = document.getElementById('bt-pin-modal');
    document.getElementById('bt-pin-address').value = address;
    document.getElementById('bt-pin-value').value = '';
    setHidden(modal, false);
    document.getElementById('bt-pin-value').focus();
}

function btClosePINModal(event) {
    if (event && event.target !== event.currentTarget) return;
    const pin = document.getElementById('bt-pin-value');
    const address = document.getElementById('bt-pin-address');
    if (pin) pin.value = '';
    if (address) address.value = '';
    setHidden(document.getElementById('bt-pin-modal'), true);
}

async function btConfirmPair() {
    const address = document.getElementById('bt-pin-address').value;
    const pin = document.getElementById('bt-pin-value').value.trim();
    btClosePINModal();
    await btDeviceAction('pair', address, pin);
}

async function btDeviceAction(operation, address, pin) {
    if (!btSavedConfigRequired()) return;
    await btRunAction('', async () => {
        const data = await btRequest('/api/bluetooth/devices/action', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ operation, address, pin: pin || '' })
        });
        btDevices = Array.isArray(data.devices) ? data.devices : btDevices;
        btRenderDevices();
        btPopulateDefaultDevice();
        showToast(t('config.bluetooth.action_ok'), 'success');
    });
}

async function btTestAudio() {
    if (!btSavedConfigRequired()) return;
    const device = document.getElementById('bt-default-device').value;
    await btRunAction('bt-test-btn', async () => {
        await btRequest('/api/bluetooth/audio/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device })
        });
        showToast(t('config.bluetooth.test_ok'), 'success');
        await btLoadStatus();
    });
}

async function btStopAudio() {
    if (!btSavedConfigRequired()) return;
    await btRunAction('bt-stop-btn', async () => {
        await btRequest('/api/bluetooth/audio/stop', { method: 'POST' });
        showToast(t('config.bluetooth.stop_ok'), 'success');
        await btLoadStatus();
    });
}

async function btRunAction(buttonID, action) {
    const button = buttonID ? document.getElementById(buttonID) : null;
    const result = document.getElementById('bt-action-result');
    if (button) button.disabled = true;
    if (result) {
        result.className = 'adg-test-result';
        result.textContent = t('config.bluetooth.working');
    }
    try {
        await action();
        if (result) {
            result.className = 'adg-test-result is-success';
            result.textContent = t('config.bluetooth.action_ok');
        }
    } catch (error) {
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = error.message || t('config.bluetooth.request_failed');
        }
        showToast(error.message || t('config.bluetooth.request_failed'), 'error');
    } finally {
        if (button) button.disabled = false;
    }
}
