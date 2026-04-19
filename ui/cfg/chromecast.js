// cfg/chromecast.js — Chromecast device management section module
let ccDevicesCache = [];
let ccDiscoveredCache = [];

function ccRenderDiscoveryState(area, type, message) {
    if (!area) return;
    area.innerHTML = `<div class="cc-discovery-state cc-discovery-state-${type}">${message}</div>`;
}

function ccNormalizeDiscoveryValue(value) {
    return String(value || '').trim().toLowerCase();
}

function ccExistingChromecastIPs() {
    return new Set(
        ccDevicesCache
            .map(d => ccNormalizeDiscoveryValue(d.ip_address))
            .filter(Boolean)
    );
}

function ccBuildDevicePayload({ name, ip, port, description }) {
    return {
        name: String(name || '').trim(),
        type: 'chromecast',
        ip_address: String(ip || '').trim(),
        port: parseInt(port, 10) || 8009,
        description: String(description || '').trim(),
        tags: ['chromecast', 'smart-home'],
        username: '',
        mac_address: '',
        vault_secret_id: ''
    };
}

function ccRenderDiscoveryResults(area, devices) {
    const existingIPs = ccExistingChromecastIPs();

    const listContainer = document.createElement('div');
    listContainer.className = 'cc-discovery-list';

    const title = document.createElement('div');
    title.className = 'cc-discovery-list-title';
    title.textContent = t('config.chromecast.found_devices', { count: devices.length });
    listContainer.appendChild(title);

    const itemsContainer = document.createElement('div');
    itemsContainer.className = 'cc-discovery-items';

    devices.forEach((d, i) => {
        const discoveredIP = ccNormalizeDiscoveryValue(d.addr);
        const alreadyAdded = !!discoveredIP && existingIPs.has(discoveredIP);

        const item = document.createElement('div');
        item.className = 'cc-discovery-item' + (alreadyAdded ? ' is-added' : '');

        const infoDiv = document.createElement('div');

        const nameSpan = document.createElement('span');
        nameSpan.className = 'cc-discovery-name';
        nameSpan.textContent = d.name || t('config.chromecast.unknown');

        const metaSpan = document.createElement('span');
        metaSpan.className = 'cc-discovery-meta';
        metaSpan.textContent = (d.addr || '—') + ':' + (d.port || 8009);

        infoDiv.appendChild(nameSpan);
        infoDiv.appendChild(metaSpan);
        item.appendChild(infoDiv);

        if (alreadyAdded) {
            const addedSpan = document.createElement('span');
            addedSpan.className = 'cc-discovery-added';
            addedSpan.textContent = '✓ ' + t('config.chromecast.already_added');
            item.appendChild(addedSpan);
        } else {
            const btn = document.createElement('button');
            btn.className = 'btn-save cc-btn-compact';
            btn.dataset.ccIdx = String(i);
            btn.textContent = '＋ ' + t('config.chromecast.add_device');
            btn.addEventListener('click', () => ccAddDiscovered(i));
            item.appendChild(btn);
        }

        itemsContainer.appendChild(item);
    });

    listContainer.appendChild(itemsContainer);
    area.innerHTML = '';
    area.appendChild(listContainer);
}

function renderChromecastSection(section) {
    const data = configData['chromecast'] || {};
    const enabledOn = data.enabled === true;
    const ttsPort = data.tts_port || 8090;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // mDNS discovery unavailable banner (Docker bridge network)
    const ccDiscBanner = featureUnavailableBanner('chromecast_discovery');
    if (ccDiscBanner) html += ccDiscBanner;
    const ccDiscBlocked = !!(runtimeData.features && runtimeData.features.chromecast_discovery && !runtimeData.features.chromecast_discovery.available);

    // ── Enabled toggle ──
    const helpEnabled = t('help.chromecast.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.chromecast.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="chromecast.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── TTS Port ──
    const helpPort = t('help.chromecast.tts_port');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.chromecast.tts_port_label') + '</div>';
    if (helpPort) html += '<div class="field-help">' + helpPort + '</div>';
    html += '<input class="field-input cc-tts-port-input" type="number" data-path="chromecast.tts_port" value="' + ttsPort + '">';
    html += '</div>';

    // ── Devices section ──
    html += '<div class="field-group">';
    html += '<div class="field-group-title">📺 ' + t('config.chromecast.devices_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.chromecast.devices_desc') + '</div>';

    // Action buttons row
    html += '<div class="cc-actions-row">';
    html += '<button class="btn-save cc-btn-primary ' + (ccDiscBlocked ? 'cc-btn-disabled' : '') + '" onclick="ccDiscoverDevices()" id="cc-discover-btn">🔍 ' + t('config.chromecast.discover_btn') + '</button>';
    html += '<button class="btn-save cc-btn-secondary" onclick="ccShowAddManual()">＋ ' + t('config.chromecast.add_manual_btn') + '</button>';
    html += '</div>';

    // Discovery results area (hidden initially)
    html += '<div id="cc-discover-area" class="cc-discover-area is-hidden"></div>';

    // Device table
    html += `
    <div id="cc-table-wrap" class="cc-table-wrap is-hidden">
        <table class="cc-table">
            <thead>
                <tr class="cc-table-head">
                    <th>${t('config.chromecast.col_name')}</th>
                    <th>${t('config.chromecast.col_ip')}</th>
                    <th>${t('config.chromecast.col_port')}</th>
                    <th>${t('config.chromecast.col_desc')}</th>
                    <th class="cc-th-actions">${t('config.chromecast.col_actions')}</th>
                </tr>
            </thead>
            <tbody id="cc-tbody"></tbody>
        </table>
    </div>
    <div id="cc-empty" class="cc-empty is-hidden">
        ${t('config.chromecast.no_devices')}
    </div>
    <div id="cc-loading" class="cc-loading">
        ${t('config.chromecast.loading')}
    </div>`;

    // ── Edit modal ──
    html += `
    <div id="cc-modal-overlay" class="cc-modal-overlay is-hidden" onclick="ccCloseModal(event)">
        <div class="cc-modal-card" onclick="event.stopPropagation()">
            <div class="cc-modal-header">
                <span id="cc-modal-title" class="cc-modal-title"></span>
                <button onclick="ccCloseModal()" class="cc-modal-close">✕</button>
            </div>
            <input type="hidden" id="cc-edit-id">
            <div class="field-group cc-field-group-sm">
                <div class="field-label">${t('config.chromecast.field_name')} *</div>
                <div class="field-help">${t('config.chromecast.field_name_help')}</div>
                <input type="text" id="cc-field-name" class="field-input" placeholder="${t('config.chromecast.name_placeholder')}">
            </div>
            <div class="cc-grid-ip-port">
                <div class="field-group">
                    <div class="field-label">${t('config.chromecast.field_ip')}</div>
                    <input type="text" id="cc-field-ip" class="field-input" placeholder="${t('config.chromecast.ip_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.chromecast.port_label')}</div>
                    <input type="number" id="cc-field-port" class="field-input" placeholder="${t('config.chromecast.port_placeholder')}" value="8009">
                </div>
            </div>
            <div class="field-group mt-sm">
                <div class="field-label">${t('config.chromecast.field_desc')}</div>
                <input type="text" id="cc-field-desc" class="field-input" placeholder="${t('config.chromecast.desc_placeholder')}">
            </div>
            <div id="cc-modal-error" class="cc-modal-error is-hidden"></div>
            <div class="cc-modal-actions">
                <button class="btn-save cc-btn-secondary" onclick="ccCloseModal()">${t('config.chromecast.cancel')}</button>
                <button class="btn-save cc-btn-primary" id="cc-modal-save" onclick="ccSaveDevice()">${t('config.chromecast.save')}</button>
            </div>
        </div>
    </div>`;

    html += '</div>';
    html += '</div>';
    document.getElementById('content').innerHTML = html;
    ccLoadDevices();
}

// ── Load chromecast devices from registry ──
async function ccLoadDevices() {
    const empty = document.getElementById('cc-empty');
    const loading = document.getElementById('cc-loading');
    const table = document.getElementById('cc-table-wrap');
    setHidden(loading, false);
    setHidden(table, true);
    setHidden(empty, true);

    try {
        const resp = await fetch('/api/devices');
        if (!resp.ok) throw new Error(await resp.text());
        const allDevices = await resp.json();
        ccDevicesCache = allDevices.filter(d => d.type === 'chromecast');
    } catch (e) {
        loading.textContent = '❌ ' + e.message;
        return;
    }

    setHidden(loading, true);
    if (ccDevicesCache.length === 0) {
        setHidden(empty, false);
        return;
    }
    setHidden(table, false);
    ccRenderRows(ccDevicesCache);
}

function ccRenderRows(devices) {
    const tbody = document.getElementById('cc-tbody');
    tbody.innerHTML = '';
    devices.forEach(d => {
        const tr = document.createElement('tr');
        tr.className = 'cc-device-row';
        tr.dataset.id = d.id;
        tr.innerHTML = `
            <td class="cc-cell cc-cell-name">${escapeHtml(d.name)}</td>
            <td class="cc-cell cc-cell-ip">${escapeHtml(d.ip_address || '—')}</td>
            <td class="cc-cell">${d.port || 8009}</td>
            <td class="cc-cell cc-cell-desc">${escapeHtml(d.description || '—')}</td>
            <td class="cc-cell cc-cell-actions">
                <button onclick="ccShowEditModal('${escapeAttr(d.id)}')" class="cc-icon-btn" title="${t('config.chromecast.edit_tooltip')}">✏️</button>
                <button onclick="ccDeleteDevice('${escapeAttr(d.id)}','${escapeAttr(d.name)}')" class="cc-icon-btn cc-icon-btn-danger" title="${t('config.chromecast.delete_tooltip')}">🗑️</button>
            </td>`;
        tbody.appendChild(tr);
    });
}

// ── mDNS discovery ──
async function ccDiscoverDevices() {
    const btn = document.getElementById('cc-discover-btn');
    const area = document.getElementById('cc-discover-area');
    btn.disabled = true;
    btn.textContent = '⏳ ' + t('config.chromecast.discovering');
    setHidden(area, false);
    ccRenderDiscoveryState(area, 'loading', '⏳ ' + t('config.chromecast.discovering'));

    try {
        const resp = await fetch('/api/chromecast/discover');
        if (!resp.ok) throw new Error(await resp.text());
        const result = await resp.json();

        if (result.status !== 'success') {
            throw new Error(result.message || t('config.chromecast.discovery_failed'));
        }

        const devices = result.devices || [];
        if (devices.length === 0) {
            ccRenderDiscoveryState(area, 'warn', '⚠️ ' + t('config.chromecast.no_devices_found'));
            return;
        }

        ccDiscoveredCache = devices;
        ccRenderDiscoveryResults(area, devices);

    } catch (e) {
        ccRenderDiscoveryState(area, 'error', '❌ ' + escapeHtml(e.message));
    } finally {
        btn.disabled = false;
        btn.textContent = '🔍 ' + t('config.chromecast.discover_btn');
    }
}

// ── Add discovered device directly; fall back to prefilled modal when details are incomplete ──
async function ccAddDiscovered(index) {
    const d = ccDiscoveredCache[index];
    if (!d) return;
    if (!String(d.addr || '').trim()) {
        ccShowEditModal(null, d);
        return;
    }

    const btn = document.querySelector(`button[data-cc-idx="${index}"]`);
    if (btn) {
        btn.disabled = true;
    }

    try {
        const payload = ccBuildDevicePayload({
            name: d.name,
            ip: d.addr,
            port: d.port,
            description: ''
        });
        const resp = await fetch('/api/devices', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Request failed');
        }
        await ccLoadDevices();
        const area = document.getElementById('cc-discover-area');
        if (area && !area.classList.contains('is-hidden') && ccDiscoveredCache.length > 0) {
            ccRenderDiscoveryResults(area, ccDiscoveredCache);
        }
    } catch (e) {
        showToast(e.message || t('config.common.error'), 'error');
        if (btn) {
            btn.disabled = false;
        }
    }
}

// ── Manual add → empty modal ──
function ccShowAddManual() {
    ccShowEditModal(null, null);
}

// ── Show modal (edit existing or add new) ──
function ccShowEditModal(id, prefill) {
    const overlay = document.getElementById('cc-modal-overlay');
    const title = document.getElementById('cc-modal-title');
    setHidden(document.getElementById('cc-modal-error'), true);

    if (id) {
        // Editing existing device
        const d = ccDevicesCache.find(x => x.id === id);
        if (!d) return;
        title.textContent = '✏️ ' + t('config.chromecast.edit_device');
        document.getElementById('cc-edit-id').value = d.id;
        document.getElementById('cc-field-name').value = d.name || '';
        document.getElementById('cc-field-ip').value = d.ip_address || '';
        document.getElementById('cc-field-port').value = d.port || 8009;
        document.getElementById('cc-field-desc').value = d.description || '';
    } else if (prefill) {
        // Adding from discovery
        title.textContent = '＋ ' + t('config.chromecast.add_device');
        document.getElementById('cc-edit-id').value = '';
        document.getElementById('cc-field-name').value = prefill.name || '';
        document.getElementById('cc-field-ip').value = prefill.addr || '';
        document.getElementById('cc-field-port').value = prefill.port || 8009;
        document.getElementById('cc-field-desc').value = '';
    } else {
        // Manual add
        title.textContent = '＋ ' + t('config.chromecast.add_device');
        document.getElementById('cc-edit-id').value = '';
        document.getElementById('cc-field-name').value = '';
        document.getElementById('cc-field-ip').value = '';
        document.getElementById('cc-field-port').value = '8009';
        document.getElementById('cc-field-desc').value = '';
    }
    setHidden(overlay, false);
}

function ccCloseModal(e) {
    if (e && e.target !== e.currentTarget) return;
    setHidden(document.getElementById('cc-modal-overlay'), true);
}

// ── Save device (create/update via device registry API) ──
async function ccSaveDevice() {
    const errBox = document.getElementById('cc-modal-error');
    setHidden(errBox, true);

    const name = document.getElementById('cc-field-name').value.trim();
    if (!name) {
        errBox.textContent = t('config.chromecast.name_required');
        setHidden(errBox, false);
        return;
    }

    const ip = document.getElementById('cc-field-ip').value.trim();
    if (!ip) {
        errBox.textContent = t('config.chromecast.ip_required');
        setHidden(errBox, false);
        return;
    }

    const payload = ccBuildDevicePayload({
        name: name,
        ip: ip,
        port: document.getElementById('cc-field-port').value,
        description: document.getElementById('cc-field-desc').value
    });

    const editId = document.getElementById('cc-edit-id').value;
    const isEdit = !!editId;
    const btn = document.getElementById('cc-modal-save');
    btn.disabled = true;

    try {
        const url = isEdit ? '/api/devices/' + editId : '/api/devices';
        const method = isEdit ? 'PUT' : 'POST';
        const resp = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Request failed');
        }
        ccCloseModal();
        await ccLoadDevices();
        const area = document.getElementById('cc-discover-area');
        if (area && !area.classList.contains('is-hidden') && ccDiscoveredCache.length > 0) {
            ccRenderDiscoveryResults(area, ccDiscoveredCache);
        }
    } catch (e) {
        errBox.textContent = '❌ ' + e.message;
        setHidden(errBox, false);
    } finally {
        btn.disabled = false;
    }
}

// ── Delete device ──
async function ccDeleteDevice(id, name) {
    const msg = t('config.chromecast.delete_confirm', { name: name });
    if (!(await showConfirm(t('config.chromecast.delete_confirm_title'), msg))) return;

    try {
        const resp = await fetch('/api/devices/' + id, { method: 'DELETE' });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Delete failed');
        }
        await ccLoadDevices();
    } catch (e) {
        showToast(e.message || t('config.common.error'), 'error');
    }
}
