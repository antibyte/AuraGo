// cfg/chromecast.js — Chromecast device management section module
let ccDevicesCache = [];
let ccDiscoveredCache = [];

function ccRenderDiscoveryState(area, type, message) {
    if (!area) return;
    area.innerHTML = `<div class="cfgdev-discovery-state cfgdev-discovery-state-${type}" role="${type === 'error' ? 'alert' : 'status'}">${message}</div>`;
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
            addedSpan.innerHTML = cfgIcon('check') + '<span>' + escapeHtml(t('config.chromecast.already_added')) + '</span>';
            item.appendChild(addedSpan);
        } else {
            const btn = document.createElement('button');
            btn.className = 'btn-save cfgdev-btn-compact';
            btn.dataset.ccIdx = String(i);
            btn.innerHTML = cfgIcon('plus') + '<span>' + escapeHtml(t('config.chromecast.add_device')) + '</span>';
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
    html += '<div class="field-group-title">' + cfgIcon('monitor') + '<span>' + t('config.chromecast.devices_title') + '</span></div>';
    html += '<div class="field-group-desc">' + t('config.chromecast.devices_desc') + '</div>';

    // Action buttons row
    html += '<div class="cfgdev-actions-row">';
    html += '<button class="btn-save cfgdev-btn-primary ' + (ccDiscBlocked ? 'cfgdev-btn-disabled' : '') + '" onclick="ccDiscoverDevices()" id="cfgdev-discover-btn">' + ccDiscoverLabel() + '</button>';
    html += '<button class="btn-save cfgdev-btn-secondary" onclick="ccShowAddManual()">' + cfgIcon('plus') + '<span>' + t('config.chromecast.add_manual_btn') + '</span></button>';
    html += '</div>';

    // Discovery results area (hidden initially)
    html += '<div id="cfgdev-discover-area" class="cfgdev-discover-area is-hidden"></div>';
    html += cfgActionResult.html('cfgdev-test-status');

    // Device table; loading, empty and error states share one container.
    html += `
    <div id="cfgdev-state"></div>
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
    </div>`;

    html += '</div>';
    html += '</div>';
    document.getElementById('content').innerHTML = html;
    ccLoadDevices();
}

function ccDiscoverLabel(busy) {
    return cfgIcon(busy ? 'loader' : 'search') + '<span>' + escapeHtml(t(busy ? 'config.chromecast.discovering' : 'config.chromecast.discover_btn')) + '</span>';
}

// ── Load chromecast devices from registry ──
async function ccLoadDevices() {
    const table = document.getElementById('cfgdev-table-wrap');
    if (!table) return;
    setHidden(table, true);
    cfgListState('cfgdev-state', { state: 'loading', title: t('config.chromecast.loading') });

    try {
        const resp = await fetch('/api/devices');
        if (!resp.ok) throw new Error(await resp.text());
        const allDevices = await resp.json();
        ccDevicesCache = allDevices.filter(d => d.type === 'chromecast');
    } catch (e) {
        cfgListState('cfgdev-state', { state: 'error', text: e.message, retry: ccLoadDevices });
        return;
    }

    if (ccDevicesCache.length === 0) {
        cfgListState('cfgdev-state', { state: 'empty', title: t('config.chromecast.no_devices') });
        return;
    }
    const state = document.getElementById('cfgdev-state');
    if (state) state.innerHTML = '';
    setHidden(table, false);
    ccRenderRows(ccDevicesCache);
}

function ccChromecastMutable() {
    const data = configData.chromecast || {};
    return data.enabled === true && data.read_only !== true;
}

// Plays a test sound on the cast device through the running service, so unsaved
// settings lock the row buttons (data-config-action-draft="false").
function ccTestDevice(id) {
    if (!ccChromecastMutable()) {
        cfgActionResult.show('cfgdev-test-status', 'warning', t('config.chromecast.test_disabled'));
        return;
    }
    return cfgActionResult(document.getElementById('cfgdev-test-btn-' + id), () => fetch('/api/chromecast/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ device_id: id })
    }), {
        target: 'cfgdev-test-status',
        pending: t('config.chromecast.test_working'),
        success: t('config.chromecast.test_ok'),
        failure: t('config.chromecast.test_failed')
    });
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
                <button id="cfgdev-test-btn-${escapeAttr(d.id)}" data-config-action-draft="false" onclick="ccTestDevice(${cfgJsArg(d.id)})" class="btn-secondary" ${ccChromecastMutable() ? '' : 'disabled'} title="${escapeAttr(t('config.chromecast.test_button'))}">${escapeHtml(t('config.chromecast.test_button'))}</button>
                <button type="button" onclick="ccShowEditModal(${cfgJsArg(d.id)})" class="btn-secondary" aria-label="${escapeAttr(t('config.chromecast.edit_tooltip'))}" title="${escapeAttr(t('config.chromecast.edit_tooltip'))}">${cfgIcon('edit')}</button>
                ${cfgDeleteButton({ iconOnly: true, label: t('config.common.delete_item', { name: d.name }), attrs: { 'data-cc-delete': d.id } })}
            </td>`;
        tr.querySelector('[data-cc-delete]').addEventListener('click', () => ccDeleteDevice(d.id, d.name));
        tbody.appendChild(tr);
    });
}

// ── mDNS discovery ──
async function ccDiscoverDevices() {
    const btn = document.getElementById('cfgdev-discover-btn');
    const area = document.getElementById('cfgdev-discover-area');
    AuraConfigFeedback.busy(btn, true);
    btn.innerHTML = ccDiscoverLabel(true);
    setHidden(area, false);
    ccRenderDiscoveryState(area, 'loading', escapeHtml(t('config.chromecast.discovering')));

    try {
        const resp = await fetch('/api/chromecast/discover');
        if (!resp.ok) throw new Error(await resp.text());
        const result = await resp.json();

        if (result.status !== 'success') {
            throw new Error(result.message || t('config.chromecast.discovery_failed'));
        }

        const devices = result.devices || [];
        if (devices.length === 0) {
            ccRenderDiscoveryState(area, 'warn', escapeHtml(t('config.chromecast.no_devices_found')));
            return;
        }

        ccDiscoveredCache = devices;
        ccRenderDiscoveryResults(area, devices);

    } catch (e) {
        ccRenderDiscoveryState(area, 'error', escapeHtml(e.message));
    } finally {
        AuraConfigFeedback.busy(btn, false);
        btn.innerHTML = ccDiscoverLabel(false);
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
            throw new Error(data.error || t('config.common.error'));
        }
        await ccLoadDevices();
        const area = document.getElementById('cfgdev-discover-area');
        if (area && !area.classList.contains('is-hidden') && ccDiscoveredCache.length > 0) {
            ccRenderDiscoveryResults(area, ccDiscoveredCache);
        }
    } catch (e) {
        cfgActionResult.show('cfgdev-test-status', 'error', e.message || t('config.common.error'));
        if (btn) {
            btn.disabled = false;
        }
    }
}

// ── Manual add: empty dialog ──
function ccShowAddManual() {
    ccShowEditModal(null, null);
}

const CC_DEVICE_DIALOG = 'cfgdev-device-dialog';

function ccDialogBody() {
    return `
        <input type="hidden" id="cfgdev-edit-id">
        <div class="field-group cfgdev-field-group-sm">
            <label class="field-label" for="cfgdev-field-name">${t('config.chromecast.field_name')} *</label>
            <div class="field-help">${t('config.chromecast.field_name_help')}</div>
            <input type="text" id="cfgdev-field-name" class="field-input" placeholder="${escapeAttr(t('config.chromecast.name_placeholder'))}">
        </div>
        <div class="cfgdev-grid-ip-port">
            <div class="field-group">
                <label class="field-label" for="cfgdev-field-ip">${t('config.chromecast.field_ip')}</label>
                <input type="text" id="cfgdev-field-ip" class="field-input" placeholder="${escapeAttr(t('config.chromecast.ip_placeholder'))}">
            </div>
            <div class="field-group">
                <label class="field-label" for="cfgdev-field-port">${t('config.chromecast.port_label')}</label>
                <input type="number" id="cfgdev-field-port" class="field-input" placeholder="${escapeAttr(t('config.chromecast.port_placeholder'))}" value="8009">
            </div>
        </div>
        <div class="field-group mt-sm">
            <label class="field-label" for="cfgdev-field-desc">${t('config.chromecast.field_desc')}</label>
            <input type="text" id="cfgdev-field-desc" class="field-input" placeholder="${escapeAttr(t('config.chromecast.desc_placeholder'))}">
        </div>
        <div id="cfgdev-form-error" class="pw-field-error is-hidden" role="alert"></div>`;
}

// ── Show dialog (edit existing or add new); built fresh on every open and removed on close ──
function ccShowEditModal(id, prefill) {
    let titleText = t('config.chromecast.add_device_title');
    let device = null;
    if (id) {
        device = ccDevicesCache.find(x => x.id === id);
        if (!device) return;
        titleText = t('config.chromecast.edit_device');
    }
    const old = document.getElementById(CC_DEVICE_DIALOG);
    if (old) old.remove();
    document.body.insertAdjacentHTML('beforeend', cfgModal({
        id: CC_DEVICE_DIALOG,
        title: titleText,
        size: 'sm',
        body: ccDialogBody(),
        actions: [
            `<button type="button" class="btn-secondary" onclick="closeModal('${CC_DEVICE_DIALOG}')">${t('config.chromecast.cancel')}</button>`,
            `<button type="button" class="btn-save" id="cfgdev-save-btn" onclick="ccSaveDevice()">${t('config.chromecast.save')}</button>`
        ]
    }));
    const dialog = document.getElementById(CC_DEVICE_DIALOG);

    const values = device
        ? { id: device.id, name: device.name || '', ip: device.ip_address || '', port: device.port || 8009, desc: device.description || '' }
        : prefill
            ? { id: '', name: ccDiscoveryDisplayName(prefill), ip: prefill.addr || '', port: prefill.port || 8009, desc: ccDiscoveryDescription(prefill) }
            : { id: '', name: '', ip: '', port: '8009', desc: '' };
    document.getElementById('cfgdev-edit-id').value = values.id;
    document.getElementById('cfgdev-field-name').value = values.name;
    document.getElementById('cfgdev-field-ip').value = values.ip;
    document.getElementById('cfgdev-field-port').value = values.port;
    document.getElementById('cfgdev-field-desc').value = values.desc;
    openModal(CC_DEVICE_DIALOG, { initialFocus: '#cfgdev-field-name', onClose: () => dialog.remove() });
}

function ccCloseModal() {
    closeModal(CC_DEVICE_DIALOG);
}

// ── Save device (create/update via device registry API) ──
async function ccSaveDevice() {
    const errBox = document.getElementById('cfgdev-form-error');
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
    const btn = document.getElementById('cfgdev-save-btn');
    AuraConfigFeedback.busy(btn, true);

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
            throw new Error(data.error || t('config.common.error'));
        }
        ccCloseModal();
        await ccLoadDevices();
        const area = document.getElementById('cfgdev-discover-area');
        if (area && !area.classList.contains('is-hidden') && ccDiscoveredCache.length > 0) {
            ccRenderDiscoveryResults(area, ccDiscoveredCache);
        }
    } catch (e) {
        errBox.textContent = e.message;
        setHidden(errBox, false);
    } finally {
        AuraConfigFeedback.busy(btn, false);
    }
}

// ── Delete device ──
async function ccDeleteDevice(id, name) {
    const answer = await showConfirmDialog({
        title: t('config.common.delete_item', { name: name }),
        message: t('config.chromecast.delete_confirm', { name: name }),
        danger: true,
        confirmText: t('config.common.delete')
    });
    if (!answer.confirmed) return;

    try {
        const resp = await fetch('/api/devices/' + id, { method: 'DELETE' });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || t('config.common.error'));
        }
        await ccLoadDevices();
    } catch (e) {
        cfgActionResult.show('cfgdev-test-status', 'error', e.message || t('config.common.error'));
    }
}
