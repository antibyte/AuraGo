// cfg/devices.js — Device Registry section module
let devicesCache = [];
let deviceCredentialsCache = [];
let deviceCredentialsLoaded = false;
let deviceCredentialsError = '';

function renderDevicesSection(section) {
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += `
    <div class="devices-toolbar">
        <div class="devices-toolbar-left">
            <div class="field-group devices-filter-group">
                <label class="field-label" for="devices-filter">${t('config.devices.filter_label')}</label>
                <input type="text" id="devices-filter" class="field-input devices-filter-input" placeholder="${escapeAttr(t('config.devices.filter_placeholder'))}" oninput="devicesApplyFilter()">
                <div class="field-help">${t('config.devices.filter_help')}</div>
            </div>
        </div>
        <button class="btn-save devices-add-btn" onclick="devicesShowModal()">
            ${cfgIcon('plus')}<span>${t('config.devices.add_device')}</span>
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
    ${cfgActionResult.html('devices-list-result')}
    <div id="devices-state"></div>`;

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    devicesLoad();
}

const DEVICE_DIALOG = 'device-dialog';

// Dialog body; every control has a label tied to it.
function devicesDialogBody() {
    const typeOptions = ['server', 'router', 'switch', 'printer', 'nas', 'camera', 'iot', 'vm', 'container', 'generic']
        .map(type => `<option value="${type}">${t('config.devices.type_' + type)}</option>`).join('');
    return `
        <input type="hidden" id="device-edit-id">
        <div class="field-group devices-field-group-bottom">
            <label class="field-label" for="device-field-name">${t('config.devices.field_name_label')}</label>
            <input type="text" id="device-field-name" class="field-input" placeholder="${escapeAttr(t('config.devices.name_placeholder'))}">
        </div>
        <div class="devices-grid devices-grid-three">
            <div class="field-group">
                <label class="field-label" for="device-field-type">${t('config.devices.type_label')}</label>
                <select id="device-field-type" class="field-input devices-select-compact">${typeOptions}</select>
            </div>
            <div class="field-group">
                <label class="field-label" for="device-field-protocol">${t('config.devices.protocol_label')}</label>
                <select id="device-field-protocol" class="field-input devices-select-compact">
                    <option value="none">${t('config.devices.protocol_none')}</option>
                    <option value="ssh">${t('config.devices.protocol_ssh')}</option>
                    <option value="vnc">${t('config.devices.protocol_vnc')}</option>
                </select>
            </div>
            <div class="field-group">
                <label class="field-label" for="device-field-port">${t('config.devices.field_port_label')}</label>
                <input type="number" id="device-field-port" class="field-input" placeholder="${escapeAttr(t('config.devices.port_placeholder'))}" value="22">
            </div>
        </div>
        <div class="devices-grid devices-grid-ip-user">
            <div class="field-group">
                <label class="field-label" for="device-field-ip">${t('config.devices.ip_address')}</label>
                <input type="text" id="device-field-ip" class="field-input" placeholder="${escapeAttr(t('config.devices.ip_placeholder'))}">
            </div>
            <div class="field-group">
                <label class="field-label" for="device-field-username">${t('config.devices.username')}</label>
                <input type="text" id="device-field-username" class="field-input" placeholder="${escapeAttr(t('config.devices.username_placeholder'))}">
            </div>
        </div>
        <div class="field-group devices-field-group-top">
            <label class="field-label" for="device-field-credential">${t('config.devices.credential_label')}</label>
            <select id="device-field-credential" class="field-input devices-select-compact" onchange="devicesUpdateCredentialMode()"></select>
        </div>
        <div class="field-group devices-field-group-top">
            <label class="field-label" for="device-field-desc">${t('config.devices.description')}</label>
            <input type="text" id="device-field-desc" class="field-input" placeholder="${escapeAttr(t('config.devices.desc_placeholder'))}">
        </div>
        <div class="field-group devices-field-group-top">
            <label class="field-label" for="device-field-mac">
                ${t('config.devices.mac_address')}
                <span class="devices-field-hint">(${t('config.devices.mac_hint')})</span>
            </label>
            <div class="devices-mac-row">
                <input type="text" id="device-field-mac" class="field-input devices-mac-input" placeholder="${escapeAttr(t('config.devices.mac_placeholder'))}">
                <button type="button" id="device-find-mac-btn" class="btn-secondary devices-find-mac-btn" onclick="devicesFindMAC()" title="${escapeAttr(t('config.devices.find_mac_tooltip'))}">
                    ${cfgIcon('search')}<span>${t('config.devices.find_mac')}</span>
                </button>
            </div>
            ${cfgActionResult.html('device-mac-status', 'devices-mac-status')}
        </div>
        <div class="field-group devices-field-group-top">
            <label class="field-label" for="device-field-tags">${t('config.devices.field_tags_label')} <span class="devices-field-hint">(${t('config.devices.tags_hint')})</span></label>
            <input type="text" id="device-field-tags" class="field-input" placeholder="${escapeAttr(t('config.devices.tags_placeholder'))}">
        </div>
        <div class="field-group devices-field-group-top">
            <label class="field-label" for="device-field-vault">${t('config.devices.field_vault_secret_id_label')} <span class="devices-field-hint">(${t('config.devices.vault_read_only')})</span></label>
            <input type="text" id="device-field-vault" class="field-input devices-vault-input" disabled>
        </div>
        ${cfgActionResult.html('device-form-error')}`;
}

async function devicesLoad() {
    const table = document.getElementById('devices-table-wrap');
    if (!table) return;
    setHidden(table, true);
    cfgListState('devices-state', { state: 'loading', title: t('config.devices.loading') });

    try {
        const resp = await fetch('/api/devices');
        if (!resp.ok) throw new Error(await resp.text());
        devicesCache = await resp.json();
    } catch (e) {
        cfgListState('devices-state', { state: 'error', text: e.message, retry: devicesLoad });
        return;
    }

    if (devicesCache.length === 0) {
        cfgListState('devices-state', { state: 'empty', title: t('config.devices.empty') });
        return;
    }
    const state = document.getElementById('devices-state');
    if (state) state.innerHTML = '';
    setHidden(table, false);
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
            <td class="devices-cell">${cfgBadge('idle', d.type)}</td>
            <td class="devices-cell devices-cell-mono devices-cell-ip">${escapeHtml(d.ip_address || '—')}</td>
            <td class="devices-cell">${d.port || '—'}</td>
            <td class="devices-cell">${escapeHtml(d.username || '—')}</td>
            <td class="devices-cell devices-cell-mono devices-cell-mac">${escapeHtml(d.mac_address || '—')}</td>
            <td class="devices-cell">${tags || '—'}</td>
            <td class="devices-cell devices-cell-actions">
                <button type="button" data-device-edit="${escapeAttr(d.id)}" class="btn-secondary" aria-label="${escapeAttr(t('config.devices.edit_tooltip'))}" title="${escapeAttr(t('config.devices.edit_tooltip'))}">${cfgIcon('edit')}</button>
                ${cfgDeleteButton({ iconOnly: true, label: t('config.common.delete_item', { name: d.name }), attrs: { 'data-device-delete': d.id } })}
            </td>`;
        tr.querySelector('[data-device-edit]').addEventListener('click', () => devicesShowModal(d.id));
        tr.querySelector('[data-device-delete]').addEventListener('click', () => devicesDelete(d.id, d.name));
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
    if (filtered.length === 0) cfgListState('devices-state', { state: 'empty', title: t('config.devices.empty') });
    else document.getElementById('devices-state').innerHTML = '';
    setHidden(document.getElementById('devices-table-wrap'), filtered.length === 0);
}

// The dialog is built fresh on every open and removed again on close.
async function devicesShowModal(id) {
    await devicesEnsureCredentialsLoaded();
    const d = id ? devicesCache.find(x => x.id === id) : null;
    if (id && !d) return;

    const old = document.getElementById(DEVICE_DIALOG);
    if (old) old.remove();
    document.body.insertAdjacentHTML('beforeend', cfgModal({
        id: DEVICE_DIALOG,
        title: t(d ? 'config.devices.edit_device' : 'config.devices.new_device'),
        size: 'lg',
        body: devicesDialogBody(),
        actions: [
            `<button type="button" class="btn-secondary" onclick="closeModal('${DEVICE_DIALOG}')">${t('config.devices.cancel')}</button>`,
            `<button type="button" class="btn-save" id="device-save-btn" onclick="devicesSave()">${t('config.devices.save')}</button>`
        ]
    }));
    const dialog = document.getElementById(DEVICE_DIALOG);

    if (d) {
        document.getElementById('device-edit-id').value = d.id;
        document.getElementById('device-field-name').value = d.name || '';
        document.getElementById('device-field-type').value = d.type || 'generic';
        document.getElementById('device-field-protocol').value = d.protocol || 'ssh';
        document.getElementById('device-field-ip').value = d.ip_address || '';
        document.getElementById('device-field-port').value = d.port || 22;
        document.getElementById('device-field-username').value = d.username || '';
        document.getElementById('device-field-desc').value = d.description || '';
        document.getElementById('device-field-mac').value = d.mac_address || '';
        document.getElementById('device-field-tags').value = (d.tags || []).join(', ');
        document.getElementById('device-field-vault').value = d.vault_secret_id || '';
        devicesPopulateCredentialSelect(d.credential_id || '');
    } else {
        document.getElementById('device-edit-id').value = '';
        document.getElementById('device-field-type').value = 'server';
        document.getElementById('device-field-protocol').value = 'ssh';
        devicesPopulateCredentialSelect('');
    }
    devicesUpdateCredentialMode();
    openModal(DEVICE_DIALOG, { initialFocus: '#device-field-name', onClose: () => dialog.remove() });
}

function devicesCloseModal() {
    closeModal(DEVICE_DIALOG);
}

// devicesFindMAC queries the ARP table via the backend API and fills in the MAC field.
function devicesFindMAC() {
    const ip = (document.getElementById('device-field-ip').value || '').trim();

    if (!ip) {
        cfgActionResult.show('device-mac-status', 'warning', t('config.devices.find_mac_no_ip'));
        return Promise.resolve({ skipped: 'no-ip' });
    }

    return cfgActionResult(document.getElementById('device-find-mac-btn'), () => fetch('/api/tools/mac_lookup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ip })
    }), {
        target: 'device-mac-status',
        pending: t('config.devices.find_mac_searching'),
        success: outcome => t('config.devices.find_mac_found', { mac: outcome.body.mac_address }),
        failure: t('config.devices.find_mac_error'),
        interpret: outcome => {
            const data = outcome.body || {};
            if (data.status === 'success' && data.mac_address) return { state: 'success' };
            if (data.status === 'not_found') return { state: 'warning', reason: t('config.devices.find_mac_not_found') };
            return { state: 'error', reason: data.message || outcome.reason || '' };
        },
        onSuccess: outcome => {
            document.getElementById('device-field-mac').value = outcome.body.mac_address;
        }
    });
}

function devicesSave() {
    const name = document.getElementById('device-field-name').value.trim();
    cfgActionResult.show('device-form-error', '', '');
    if (!name) {
        cfgActionResult.show('device-form-error', 'error', t('config.devices.name_required'));
        return Promise.resolve({ skipped: 'invalid' });
    }

    const tagsRaw = document.getElementById('device-field-tags').value.trim();
    const tags = tagsRaw ? tagsRaw.split(',').map(s => s.trim()).filter(Boolean) : [];
    const credentialId = (document.getElementById('device-field-credential').value || '').trim();

    const payload = {
        name: name,
        type: document.getElementById('device-field-type').value,
        protocol: document.getElementById('device-field-protocol').value || 'ssh',
        ip_address: document.getElementById('device-field-ip').value.trim(),
        port: parseInt(document.getElementById('device-field-port').value) || 22,
        username: credentialId ? '' : document.getElementById('device-field-username').value.trim(),
        description: document.getElementById('device-field-desc').value.trim(),
        mac_address: document.getElementById('device-field-mac').value.trim(),
        tags: tags,
        credential_id: credentialId,
        vault_secret_id: credentialId ? '' : (document.getElementById('device-field-vault').value || '')
    };

    const editId = document.getElementById('device-edit-id').value;
    const isEdit = !!editId;
    return cfgActionResult(document.getElementById('device-save-btn'), () => fetch(isEdit ? '/api/devices/' + editId : '/api/devices', {
        method: isEdit ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
    }), {
        target: 'device-form-error',
        success: t('config.common.saved'),
        failure: t('config.common.error'),
        onSuccess: async () => {
            devicesCloseModal();
            await devicesLoad();
            cfgActionResult.show('devices-list-result', 'success', t('config.common.saved'));
        }
    });
}

async function devicesEnsureCredentialsLoaded() {
    if (deviceCredentialsLoaded) return;
    try {
        const resp = await fetch('/api/credentials');
        if (!resp.ok) throw new Error(await resp.text());
        deviceCredentialsCache = await resp.json();
        deviceCredentialsError = '';
    } catch (e) {
        deviceCredentialsCache = [];
        deviceCredentialsError = e.message || t('config.devices.credential_unavailable');
    } finally {
        deviceCredentialsLoaded = true;
    }
}

function devicesPopulateCredentialSelect(selectedId) {
    const select = document.getElementById('device-field-credential');
    if (!select) return;
    const selected = selectedId || '';
    select.innerHTML = `<option value="">${escapeHtml(t('config.devices.credential_none'))}</option>`;
    let found = selected === '';
    deviceCredentialsCache.forEach(cred => {
        const option = document.createElement('option');
        option.value = cred.id || '';
        option.textContent = devicesCredentialLabel(cred);
        if (option.value === selected) found = true;
        select.appendChild(option);
    });
    if (selected && !found) {
        const option = document.createElement('option');
        option.value = selected;
        option.textContent = selected;
        select.appendChild(option);
    }
    if (deviceCredentialsError) {
        const option = document.createElement('option');
        option.value = '';
        option.disabled = true;
        option.textContent = t('config.devices.credential_unavailable');
        select.appendChild(option);
    }
    select.value = selected;
}

function devicesCredentialLabel(cred) {
    const name = cred.name || cred.id || t('config.devices.credential_none');
    const userHost = [cred.username, cred.host].filter(Boolean).join('@');
    const suffix = [cred.type, userHost].filter(Boolean).join(' · ');
    return suffix ? `${name} (${suffix})` : name;
}

function devicesUpdateCredentialMode() {
    const select = document.getElementById('device-field-credential');
    const username = document.getElementById('device-field-username');
    if (!select || !username) return;
    const hasCredential = !!select.value;
    username.disabled = hasCredential;
    if (hasCredential) {
        username.value = '';
    }
}

async function devicesDelete(id, name) {
    const answer = await showConfirmDialog({
        title: t('config.common.delete_item', { name: name }),
        message: t('config.devices.delete_confirm', { name: name }),
        danger: true,
        confirmText: t('config.common.delete')
    });
    if (!answer.confirmed) return { skipped: 'cancelled' };

    const btn = document.querySelector('#devices-tbody tr[data-id="' + id + '"] .cfg-delete-btn');
    return cfgActionResult(btn, () => fetch('/api/devices/' + encodeURIComponent(id), { method: 'DELETE' }), {
        target: 'devices-list-result',
        success: name + ': ' + t('config.feedback.done'),
        failure: name + ': ' + t('config.common.error'),
        onSuccess: () => devicesLoad()
    });
}
