// cfg/devices.js — Device Registry section module
let devicesCache = [];

function devicesSetHidden(element, hidden) {
    if (!element) return;
    element.classList.toggle('is-hidden', !!hidden);
}

function renderDevicesSection(section) {
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += `
    <div class="devices-toolbar">
        <div class="devices-toolbar-left">
            <input type="text" id="devices-filter" class="field-input devices-filter-input" placeholder="${t('config.devices.filter_placeholder')}" oninput="devicesApplyFilter()">
        </div>
        <button class="btn-save devices-add-btn" onclick="devicesShowModal()">
            ＋ ${t('config.devices.add_device')}
        </button>
    </div>
    <div id="devices-table-wrap" class="devices-table-wrap is-hidden">
        <table class="devices-table">
            <thead>
                <tr class="devices-table-head-row">
                    <th class="devices-cell">${t('config.devices.col_name')}</th>
                    <th class="devices-cell">${t('config.devices.col_type')}</th>
                    <th class="devices-cell">${t('config.devices.col_ip')}</th>
                    <th class="devices-cell">${t('config.devices.col_port')}</th>
                    <th class="devices-cell">${t('config.devices.col_user')}</th>
                    <th class="devices-cell">${t('config.devices.col_mac')}</th>
                    <th class="devices-cell">${t('config.devices.col_tags')}</th>
                    <th class="devices-cell devices-cell-actions">${t('config.devices.col_actions')}</th>
                </tr>
            </thead>
            <tbody id="devices-tbody"></tbody>
        </table>
    </div>
    <div id="devices-empty" class="devices-empty is-hidden">
        ${t('config.devices.empty')}
    </div>
    <div id="devices-loading" class="devices-loading">
        ${t('config.devices.loading')}
    </div>`;

    // Modal overlay
    html += `
    <div id="device-modal-overlay" class="devices-modal-overlay is-hidden" onclick="devicesCloseModal(event)">
        <div class="devices-modal-card" onclick="event.stopPropagation()">
            <div class="devices-modal-header">
                <span id="device-modal-title" class="devices-modal-title"></span>
                <button onclick="devicesCloseModal()" class="devices-modal-close">✕</button>
            </div>
            <input type="hidden" id="device-edit-id">
            <div class="field-group devices-field-group-bottom">
                <div class="field-label">Name *</div>
                <input type="text" id="device-field-name" class="field-input" placeholder="${t('config.devices.name_placeholder')}">
            </div>
            <div class="devices-grid devices-grid-two">
                <div class="field-group">
                    <div class="field-label">${t('config.devices.type_label')}</div>
                    <select id="device-field-type" class="field-input devices-select-compact">
                        <option value="server">Server</option>
                        <option value="router">Router</option>
                        <option value="switch">Switch</option>
                        <option value="printer">Printer</option>
                        <option value="nas">NAS</option>
                        <option value="camera">Camera</option>
                        <option value="iot">IoT</option>
                        <option value="vm">VM</option>
                        <option value="container">Container</option>
                        <option value="generic">Generic</option>
                    </select>
                </div>
                <div class="field-group">
                    <div class="field-label">Port</div>
                    <input type="number" id="device-field-port" class="field-input" placeholder="22" value="22">
                </div>
            </div>
            <div class="devices-grid devices-grid-ip-user">
                <div class="field-group">
                    <div class="field-label">${t('config.devices.ip_address')}</div>
                    <input type="text" id="device-field-ip" class="field-input" placeholder="192.168.1.100">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.devices.username')}</div>
                    <input type="text" id="device-field-username" class="field-input" placeholder="root">
                </div>
            </div>
            <div class="field-group devices-field-group-top">
                <div class="field-label">${t('config.devices.description')}</div>
                <input type="text" id="device-field-desc" class="field-input" placeholder="${t('config.devices.desc_placeholder')}">
            </div>
            <div class="field-group devices-field-group-top">
                <div class="field-label">
                    ${t('config.devices.mac_address')}
                    <span class="devices-field-hint">(${t('config.devices.mac_hint')})</span>
                </div>
                <div class="devices-mac-row">
                    <input type="text" id="device-field-mac" class="field-input devices-mac-input" placeholder="AA:BB:CC:DD:EE:FF">
                    <button id="device-find-mac-btn" class="btn-secondary devices-find-mac-btn" onclick="devicesFindMAC()" title="${t('config.devices.find_mac_tooltip')}">
                        🔍 ${t('config.devices.find_mac')}
                    </button>
                </div>
                <div id="device-mac-status" class="devices-mac-status is-hidden"></div>
            </div>
            <div class="field-group devices-field-group-top">
                <div class="field-label">Tags <span class="devices-field-hint">(${t('config.devices.tags_hint')})</span></div>
                <input type="text" id="device-field-tags" class="field-input" placeholder="${t('config.devices.tags_placeholder')}">
            </div>
            <div class="field-group devices-field-group-top">
                <div class="field-label">Vault Secret ID <span class="devices-field-hint">(${t('config.devices.vault_read_only')})</span></div>
                <input type="text" id="device-field-vault" class="field-input devices-vault-input" disabled>
            </div>
            <div id="device-modal-error" class="devices-modal-error is-hidden"></div>
            <div class="devices-modal-actions">
                <button class="btn-save devices-modal-cancel" onclick="devicesCloseModal()">${t('config.devices.cancel')}</button>
                <button class="btn-save devices-modal-save" id="device-modal-save" onclick="devicesSave()">${t('config.devices.save')}</button>
            </div>
        </div>
    </div>`;

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    devicesLoad();
}

async function devicesLoad() {
    const tbody = document.getElementById('devices-tbody');
    const empty = document.getElementById('devices-empty');
    const loading = document.getElementById('devices-loading');
    const table = document.getElementById('devices-table-wrap');
    devicesSetHidden(loading, false);
    devicesSetHidden(table, true);
    devicesSetHidden(empty, true);

    try {
        const resp = await fetch('/api/devices');
        if (!resp.ok) throw new Error(await resp.text());
        devicesCache = await resp.json();
    } catch (e) {
        loading.textContent = '❌ ' + e.message;
        return;
    }

    devicesSetHidden(loading, true);
    if (devicesCache.length === 0) {
        devicesSetHidden(empty, false);
        return;
    }
    devicesSetHidden(table, false);
    devicesRenderRows(devicesCache);
}

function devicesRenderRows(devices) {
    const tbody = document.getElementById('devices-tbody');
    tbody.innerHTML = '';
    devices.forEach(d => {
        const tags = (d.tags || []).map(tag => `<span class="devices-tag">${escapeHtml(tag)}</span>`).join('');
        const tr = document.createElement('tr');
        tr.className = 'devices-row';
        tr.dataset.id = d.id;
        tr.innerHTML = `
            <td class="devices-cell devices-cell-name">${escapeHtml(d.name)}</td>
            <td class="devices-cell"><span class="devices-type-pill">${escapeHtml(d.type)}</span></td>
            <td class="devices-cell devices-cell-mono devices-cell-ip">${escapeHtml(d.ip_address || '—')}</td>
            <td class="devices-cell">${d.port || '—'}</td>
            <td class="devices-cell">${escapeHtml(d.username || '—')}</td>
            <td class="devices-cell devices-cell-mono devices-cell-mac">${escapeHtml(d.mac_address || '—')}</td>
            <td class="devices-cell">${tags || '—'}</td>
            <td class="devices-cell devices-cell-actions">
                <button onclick="devicesShowModal('${escapeAttr(d.id)}')" class="devices-action-btn" title="${t('config.devices.edit_tooltip')}">✏️</button>
                <button onclick="devicesDelete('${escapeAttr(d.id)}','${escapeAttr(d.name)}')" class="devices-action-btn devices-action-btn-delete" title="${t('config.devices.delete_tooltip')}">🗑️</button>
            </td>`;
        tbody.appendChild(tr);
    });
}

function devicesApplyFilter() {
    const q = (document.getElementById('devices-filter').value || '').toLowerCase();
    const filtered = devicesCache.filter(d => {
        const hay = [d.name, d.type, d.ip_address, d.username, d.description, d.mac_address, ...(d.tags || [])].join(' ').toLowerCase();
        return hay.includes(q);
    });
    devicesRenderRows(filtered);
    devicesSetHidden(document.getElementById('devices-empty'), filtered.length !== 0);
    devicesSetHidden(document.getElementById('devices-table-wrap'), filtered.length === 0);
}

function devicesShowModal(id) {
    const overlay = document.getElementById('device-modal-overlay');
    const title = document.getElementById('device-modal-title');
    devicesSetHidden(document.getElementById('device-modal-error'), true);
    devicesSetHidden(document.getElementById('device-mac-status'), true);

    if (id) {
        const d = devicesCache.find(x => x.id === id);
        if (!d) return;
        title.textContent = t('config.devices.edit_device');
        document.getElementById('device-edit-id').value = d.id;
        document.getElementById('device-field-name').value = d.name || '';
        document.getElementById('device-field-type').value = d.type || 'generic';
        document.getElementById('device-field-ip').value = d.ip_address || '';
        document.getElementById('device-field-port').value = d.port || 22;
        document.getElementById('device-field-username').value = d.username || '';
        document.getElementById('device-field-desc').value = d.description || '';
        document.getElementById('device-field-mac').value = d.mac_address || '';
        document.getElementById('device-field-tags').value = (d.tags || []).join(', ');
        document.getElementById('device-field-vault').value = d.vault_secret_id || '';
    } else {
        title.textContent = t('config.devices.new_device');
        document.getElementById('device-edit-id').value = '';
        document.getElementById('device-field-name').value = '';
        document.getElementById('device-field-type').value = 'server';
        document.getElementById('device-field-ip').value = '';
        document.getElementById('device-field-port').value = '22';
        document.getElementById('device-field-username').value = '';
        document.getElementById('device-field-desc').value = '';
        document.getElementById('device-field-mac').value = '';
        document.getElementById('device-field-tags').value = '';
        document.getElementById('device-field-vault').value = '';
    }
    devicesSetHidden(overlay, false);
}

function devicesCloseModal(e) {
    if (e && e.target !== e.currentTarget) return;
    devicesSetHidden(document.getElementById('device-modal-overlay'), true);
}

// devicesFindMAC queries the ARP table via the backend API and fills in the MAC field.
async function devicesFindMAC() {
    const ip = (document.getElementById('device-field-ip').value || '').trim();
    const statusEl = document.getElementById('device-mac-status');
    const btn = document.getElementById('device-find-mac-btn');

    if (!ip) {
        statusEl.textContent = '⚠️ ' + t('config.devices.find_mac_no_ip');
        statusEl.className = 'devices-mac-status devices-mac-warn';
        devicesSetHidden(statusEl, false);
        return;
    }

    btn.disabled = true;
    statusEl.textContent = t('config.devices.find_mac_searching');
    statusEl.className = 'devices-mac-status devices-mac-info';
    devicesSetHidden(statusEl, false);

    try {
        const resp = await fetch('/api/tools/mac_lookup', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ip })
        });
        const data = await resp.json();

        if (data.status === 'success' && data.mac_address) {
            document.getElementById('device-field-mac').value = data.mac_address;
            statusEl.textContent = '✅ ' + t('config.devices.find_mac_found', { mac: data.mac_address });
            statusEl.className = 'devices-mac-status devices-mac-ok';
        } else if (data.status === 'not_found') {
            statusEl.textContent = '🔍 ' + t('config.devices.find_mac_not_found');
            statusEl.className = 'devices-mac-status devices-mac-warn';
        } else {
            statusEl.textContent = '❌ ' + (data.message || t('config.devices.find_mac_error'));
            statusEl.className = 'devices-mac-status devices-mac-error';
        }
    } catch (e) {
        statusEl.textContent = '❌ ' + e.message;
        statusEl.className = 'devices-mac-status devices-mac-error';
    } finally {
        btn.disabled = false;
        devicesSetHidden(statusEl, false);
    }
}

async function devicesSave() {
    const errBox = document.getElementById('device-modal-error');
    devicesSetHidden(errBox, true);

    const name = document.getElementById('device-field-name').value.trim();
    if (!name) {
        errBox.textContent = t('config.devices.name_required');
        devicesSetHidden(errBox, false);
        return;
    }

    const tagsRaw = document.getElementById('device-field-tags').value.trim();
    const tags = tagsRaw ? tagsRaw.split(',').map(s => s.trim()).filter(Boolean) : [];

    const payload = {
        name: name,
        type: document.getElementById('device-field-type').value,
        ip_address: document.getElementById('device-field-ip').value.trim(),
        port: parseInt(document.getElementById('device-field-port').value) || 22,
        username: document.getElementById('device-field-username').value.trim(),
        description: document.getElementById('device-field-desc').value.trim(),
        mac_address: document.getElementById('device-field-mac').value.trim(),
        tags: tags,
        vault_secret_id: document.getElementById('device-field-vault').value || ''
    };

    const editId = document.getElementById('device-edit-id').value;
    const isEdit = !!editId;
    const btn = document.getElementById('device-modal-save');
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
        devicesCloseModal();
        await devicesLoad();
    } catch (e) {
        errBox.textContent = '❌ ' + e.message;
        devicesSetHidden(errBox, false);
    } finally {
        btn.disabled = false;
    }
}

async function devicesDelete(id, name) {
    const msg = t('config.devices.delete_confirm', {name: name});
    if (!confirm(msg)) return;

    try {
        const resp = await fetch('/api/devices/' + id, { method: 'DELETE' });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Delete failed');
        }
        await devicesLoad();
    } catch (e) {
        showToast(e.message || t('config.common.error'), 'error');
    }
}
