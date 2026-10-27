// cfg/chromecast.js — Chromecast device management section module
let ccDevicesCache = [];
let ccDiscoveredCache = [];

function ccRenderDiscoveryState(area, type, message) {
    if (!area) return;
    area.innerHTML = `<div class="cfgdev-discovery-state cfgdev-discovery-state-${type}">${message}</div>`;
}

function ccNormalizeDiscoveryValue(value) {
    return String(value || '').trim().toLowerCase();
}

function ccDiscoveryDisplayName(device) {
    return String((device && (device.friendly_name || device.name)) || '').trim();
}

function ccDiscoveryDescription(device) {
    const friendlyName = ccDiscoveryDisplayName(device);
    const serviceName = String((device && device.name) || '').trim();
    if (serviceName && serviceName !== friendlyName) {
        return serviceName;
    }
    return '';
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
    listContainer.className = 'cfgdev-discovery-list';

    const title = document.createElement('div');
    title.className = 'cfgdev-discovery-list-title';
    title.textContent = t('config.chromecast.found_devices', { count: devices.length });
    listContainer.appendChild(title);

    const itemsContainer = document.createElement('div');
    itemsContainer.className = 'cfgdev-discovery-items';

    devices.forEach((d, i) => {
        const discoveredIP = ccNormalizeDiscoveryValue(d.addr);
        const alreadyAdded = !!discoveredIP && existingIPs.has(discoveredIP);

        const item = document.createElement('div');
        item.className = 'cfgdev-discovery-item' + (alreadyAdded ? ' is-added' : '');

        const infoDiv = document.createElement('div');

        const nameSpan = document.createElement('span');
        nameSpan.className = 'cfgdev-discovery-name';
        nameSpan.textContent = ccDiscoveryDisplayName(d) || t('config.chromecast.unknown');

        const metaSpan = document.createElement('span');
        metaSpan.className = 'cfgdev-discovery-meta';
        metaSpan.textContent = (d.addr || '—') + ':' + (d.port || 8009);

        infoDiv.appendChild(nameSpan);
        infoDiv.appendChild(metaSpan);
        if (d.group) {
            const groupSpan = document.createElement('span');
            groupSpan.className = 'cfgdev-discovery-added';
            groupSpan.textContent = t('config.chromecast.group_label');
            infoDiv.appendChild(groupSpan);
        }
        if (d.audio_only) {
            const audioSpan = document.createElement('span');
            audioSpan.className = 'cfgdev-discovery-meta';
            audioSpan.textContent = t('config.chromecast.audio_only_label');
            infoDiv.appendChild(audioSpan);
        }
        item.appendChild(infoDiv);

        if (alreadyAdded) {
            const addedSpan = document.createElement('span');
            addedSpan.className = 'cfgdev-discovery-added';
            addedSpan.textContent = '✓ ' + t('config.chromecast.already_added');
            item.appendChild(addedSpan);
        } else {
            const btn = document.createElement('button');
            btn.className = 'btn-save cfgdev-btn-compact';
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
    const mediaAllowlist = Array.isArray(data.media_host_allowlist)
        ? data.media_host_allowlist.join(', ')
        : String(data.media_host_allowlist || '');

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
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
    html += '<input class="field-input cfgdev-tts-port-input" type="number" data-path="chromecast.tts_port" value="' + ttsPort + '">';
    html += '</div>';

    // ── Media host allowlist ──
    const helpMediaAllowlist = t('help.chromecast.media_host_allowlist');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.chromecast.media_host_allowlist_label') + '</div>';
    if (helpMediaAllowlist) html += '<div class="field-help">' + helpMediaAllowlist + '</div>';
    html += '<input class="field-input" type="text" data-path="chromecast.media_host_allowlist" data-type="array" value="' + escapeAttr(mediaAllowlist) + '" placeholder="192.168.1.10, media.lan:8096, 192.168.1.0/24">';
    html += '</div>';

    const helpAdvertise = t('help.chromecast.advertise_host');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.chromecast.advertise_host_label') + '</div>';
    if (helpAdvertise) html += '<div class="field-help">' + helpAdvertise + '</div>';
    html += '<input class="field-input" type="text" data-path="chromecast.advertise_host" value="' + escapeAttr(data.advertise_host || '') + '" placeholder="192.168.1.20">';
    html += '</div>';

    const readOnlyOn = data.read_only === true;
    const helpReadOnly = t('help.chromecast.read_only');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.chromecast.read_only_label') + '</div>';
    if (helpReadOnly) html += '<div class="field-help">' + helpReadOnly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readOnlyOn ? ' on' : '') + '" data-path="chromecast.read_only" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readOnlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Devices section ──
    html += '<div class="field-group">';
    html += '<div class="field-group-title">📺 ' + t('config.chromecast.devices_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.chromecast.devices_desc') + '</div>';

    // Action buttons row
    html += '<div class="cfgdev-actions-row">';
    html += '<button class="btn-save cfgdev-btn-primary ' + (ccDiscBlocked ? 'cfgdev-btn-disabled' : '') + '" onclick="ccDiscoverDevices()" id="cfgdev-discover-btn">🔍 ' + t('config.chromecast.discover_btn') + '</button>';
    html += '<button class="btn-save cfgdev-btn-secondary" onclick="ccShowAddManual()">＋ ' + t('config.chromecast.add_manual_btn') + '</button>';
    html += '</div>';

    // Discovery results area (hidden initially)
    html += '<div id="cfgdev-discover-area" class="cfgdev-discover-area is-hidden"></div>';
    html += '<div id="cfgdev-test-status" class="field-help"></div>';

    // Device table
    html += `
    <div id="cfgdev-table-wrap" class="cfgdev-table-wrap is-hidden">
        <table class="cfgdev-table">
            <thead>
                <tr class="cfgdev-table-head">
                    <th>${t('config.chromecast.col_name')}</th>
                    <th>${t('config.chromecast.col_ip')}</th>
                    <th>${t('config.chromecast.col_port')}</th>
                    <th>${t('config.chromecast.col_desc')}</th>
                    <th class="cfgdev-th-actions">${t('config.chromecast.col_actions')}</th>
                </tr>
            </thead>
            <tbody id="cfgdev-tbody"></tbody>
        </table>
    </div>
    <div id="cfgdev-empty" class="cfgdev-empty is-hidden">
        ${t('config.chromecast.no_devices')}
    </div>
    <div id="cfgdev-loading" class="cfgdev-loading">
        ${t('config.chromecast.loading')}
    </div>`;

    // ── Edit modal ──
    html += `
    <div id="cfgdev-modal-overlay" class="cfgdev-modal-overlay is-hidden" onclick="ccCloseModal(event)">
        <div class="cfgdev-modal-card" onclick="event.stopPropagation()">
            <div class="cfgdev-modal-header">
                <span id="cfgdev-modal-title" class="cfgdev-modal-title"></span>
                <button onclick="ccCloseModal()" class="cfgdev-modal-close">✕</button>
            </div>
            <input type="hidden" id="cfgdev-edit-id">
            <div class="field-group cfgdev-field-group-sm">
                <div class="field-label">${t('config.chromecast.field_name')} *</div>
                <div class="field-help">${t('config.chromecast.field_name_help')}</div>
                <input type="text" id="cfgdev-field-name" class="field-input" placeholder="${t('config.chromecast.name_placeholder')}">
            </div>
            <div class="cfgdev-grid-ip-port">
                <div class="field-group">
                    <div class="field-label">${t('config.chromecast.field_ip')}</div>
                    <input type="text" id="cfgdev-field-ip" class="field-input" placeholder="${t('config.chromecast.ip_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.chromecast.port_label')}</div>
                    <input type="number" id="cfgdev-field-port" class="field-input" placeholder="${t('config.chromecast.port_placeholder')}" value="8009">
                </div>
            </div>
            <div class="field-group mt-sm">
                <div class="field-label">${t('config.chromecast.field_desc')}</div>
                <input type="text" id="cfgdev-field-desc" class="field-input" placeholder="${t('config.chromecast.desc_placeholder')}">
            </div>
            <div id="cfgdev-modal-error" class="cfgdev-modal-error is-hidden"></div>
            <div class="cfgdev-modal-actions">
                <button class="btn-save cfgdev-btn-secondary" onclick="ccCloseModal()">${t('config.chromecast.cancel')}</button>
                <button class="btn-save cfgdev-btn-primary" id="cfgdev-modal-save" onclick="ccSaveDevice()">${t('config.chromecast.save')}</button>
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
    const empty = document.getElementById('cfgdev-empty');
    const loading = document.getElementById('cfgdev-loading');
    const table = document.getElementById('cfgdev-table-wrap');
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

function ccChromecastMutable() {
    const data = configData.chromecast || {};
    return data.enabled === true && data.read_only !== true;
}

function ccSetTestStatus(message) {
    const status = document.getElementById('cfgdev-test-status');
    if (status) status.textContent = message || '';
}

async function ccTestDevice(id) {
    if (!ccChromecastMutable()) {
        ccSetTestStatus(t('config.chromecast.test_disabled'));
        return;
    }
    ccSetTestStatus(t('config.chromecast.test_working'));
    try {
        const resp = await fetch('/api/chromecast/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ device_id: id })
        });
        if (!resp.ok) {
            ccSetTestStatus(t('config.chromecast.test_failed'));
            return;
        }
        ccSetTestStatus(t('config.chromecast.test_ok'));
    } catch (e) {
        ccSetTestStatus(t('config.chromecast.test_failed'));
    }
}

function ccRenderRows(devices) {
    const tbody = document.getElementById('cfgdev-tbody');
    tbody.innerHTML = '';
    devices.forEach(d => {
        const tr = document.createElement('tr');
        tr.className = 'cfgdev-device-row';
        tr.dataset.id = d.id;
        tr.innerHTML = `
            <td class="cfgdev-cell cfgdev-cell-name">${escapeHtml(d.name)}</td>
            <td class="cfgdev-cell cfgdev-cell-ip">${escapeHtml(d.ip_address || '—')}</td>
            <td class="cfgdev-cell">${d.port || 8009}</td>
            <td class="cfgdev-cell cfgdev-cell-desc">${escapeHtml(d.description || '—')}</td>
            <td class="cfgdev-cell cfgdev-cell-actions">
                <button onclick="ccTestDevice('${escapeAttr(d.id)}')" class="cfgdev-icon-btn" ${ccChromecastMutable() ? '' : 'disabled'} title="${escapeAttr(t('config.chromecast.test_button'))}">${escapeHtml(t('config.chromecast.test_button'))}</button>
                <button onclick="ccShowEditModal('${escapeAttr(d.id)}')" class="cfgdev-icon-btn" title="${t('config.chromecast.edit_tooltip')}">✏️</button>
                <button onclick="ccDeleteDevice('${escapeAttr(d.id)}','${escapeAttr(d.name)}')" class="cfgdev-icon-btn cfgdev-icon-btn-danger" title="${t('config.chromecast.delete_tooltip')}">🗑️</button>
            </td>`;
        tbody.appendChild(tr);
    });
}

// ── mDNS discovery ──
async function ccDiscoverDevices() {
    const btn = document.getElementById('cfgdev-discover-btn');
    const area = document.getElementById('cfgdev-discover-area');
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
            name: ccDiscoveryDisplayName(d),
            ip: d.addr,
            port: d.port,
            description: ccDiscoveryDescription(d)
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
        const area = document.getElementById('cfgdev-discover-area');
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
    const overlay = document.getElementById('cfgdev-modal-overlay');
    const title = document.getElementById('cfgdev-modal-title');
    setHidden(document.getElementById('cfgdev-modal-error'), true);

    if (id) {
        // Editing existing device
        const d = ccDevicesCache.find(x => x.id === id);
        if (!d) return;
        title.textContent = '✏️ ' + t('config.chromecast.edit_device');
        document.getElementById('cfgdev-edit-id').value = d.id;
        document.getElementById('cfgdev-field-name').value = d.name || '';
        document.getElementById('cfgdev-field-ip').value = d.ip_address || '';
        document.getElementById('cfgdev-field-port').value = d.port || 8009;
        document.getElementById('cfgdev-field-desc').value = d.description || '';
    } else if (prefill) {
        // Adding from discovery
        title.textContent = '＋ ' + t('config.chromecast.add_device');
        document.getElementById('cfgdev-edit-id').value = '';
        document.getElementById('cfgdev-field-name').value = ccDiscoveryDisplayName(prefill);
        document.getElementById('cfgdev-field-ip').value = prefill.addr || '';
        document.getElementById('cfgdev-field-port').value = prefill.port || 8009;
        document.getElementById('cfgdev-field-desc').value = ccDiscoveryDescription(prefill);
    } else {
        // Manual add
        title.textContent = '＋ ' + t('config.chromecast.add_device');
        document.getElementById('cfgdev-edit-id').value = '';
        document.getElementById('cfgdev-field-name').value = '';
        document.getElementById('cfgdev-field-ip').value = '';
        document.getElementById('cfgdev-field-port').value = '8009';
        document.getElementById('cfgdev-field-desc').value = '';
    }
    setHidden(overlay, false);
}

function ccCloseModal(e) {
    if (e && e.target !== e.currentTarget) return;
    setHidden(document.getElementById('cfgdev-modal-overlay'), true);
}

// ── Save device (create/update via device registry API) ──
async function ccSaveDevice() {
    const errBox = document.getElementById('cfgdev-modal-error');
    setHidden(errBox, true);

    const name = document.getElementById('cfgdev-field-name').value.trim();
    if (!name) {
        errBox.textContent = t('config.chromecast.name_required');
        setHidden(errBox, false);
        return;
    }

    const ip = document.getElementById('cfgdev-field-ip').value.trim();
    if (!ip) {
        errBox.textContent = t('config.chromecast.ip_required');
        setHidden(errBox, false);
        return;
    }

    const payload = ccBuildDevicePayload({
        name: name,
        ip: ip,
        port: document.getElementById('cfgdev-field-port').value,
        description: document.getElementById('cfgdev-field-desc').value
    });

    const editId = document.getElementById('cfgdev-edit-id').value;
    const isEdit = !!editId;
    const btn = document.getElementById('cfgdev-modal-save');
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
        const area = document.getElementById('cfgdev-discover-area');
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
