// cfg/network_shares.js — local SMB/NFS server-share configuration and administration.
let nsShares = [];
let nsRuntime = null;
let nsPermissions = null;
let nsEditingID = '';

function renderNetworkSharesSection(section) {
    const data = configData.network_shares || {};
    const smb = data.smb || {};
    const nfs = data.nfs || {};
    const html = `
    <div class="cfg-section active">
        <div class="section-header">${escapeHtml(section.label)}</div>
        <div class="section-desc">${escapeHtml(section.desc)}</div>
        <div id="ns-runtime-banner" class="adg-status-banner" role="status" aria-live="polite">${escapeHtml(t('config.network_shares.loading'))}</div>

        ${nsToggle('network_shares.enabled', data.enabled === true, 'config.network_shares.enabled', 'help.network_shares.enabled')}
        ${nsToggle('network_shares.readonly', data.readonly !== false, 'config.network_shares.readonly', 'help.network_shares.readonly')}

        <div class="cfg-card">
            <div class="cfg-card-title">${escapeHtml(t('config.network_shares.permissions_title'))}</div>
            ${nsToggle('network_shares.allow_create', data.allow_create === true, 'config.network_shares.allow_create', 'help.network_shares.allow_create')}
            ${nsToggle('network_shares.allow_update', data.allow_update === true, 'config.network_shares.allow_update', 'help.network_shares.allow_update')}
            ${nsToggle('network_shares.allow_delete', data.allow_delete === true, 'config.network_shares.allow_delete', 'help.network_shares.allow_delete')}
        </div>

        ${nsListEditor('network_shares.allowed_roots', data.allowed_roots, 'config.network_shares.roots', 'help.network_shares.roots', 'config.network_shares.root_placeholder')}

        <div class="cfg-card">
            <div class="cfg-card-title">${escapeHtml(t('config.network_shares.smb_title'))}</div>
            ${nsToggle('network_shares.smb.enabled', smb.enabled !== false, 'config.network_shares.protocol_enabled', 'help.network_shares.smb_enabled')}
            ${nsToggle('network_shares.smb.allow_guest', smb.allow_guest === true, 'config.network_shares.allow_guest', 'help.network_shares.allow_guest')}
            ${nsListEditor('network_shares.smb.allowed_principals', smb.allowed_principals, 'config.network_shares.principals', 'help.network_shares.principals', 'config.network_shares.principal_placeholder')}
        </div>

        <div class="cfg-card">
            <div class="cfg-card-title">${escapeHtml(t('config.network_shares.nfs_title'))}</div>
            ${nsToggle('network_shares.nfs.enabled', nfs.enabled !== false, 'config.network_shares.protocol_enabled', 'help.network_shares.nfs_enabled')}
            ${nsListEditor('network_shares.nfs.allowed_clients', nfs.allowed_clients, 'config.network_shares.clients', 'help.network_shares.clients', 'config.network_shares.client_placeholder')}
        </div>

        <div class="field-group">
            <div class="field-group-title">${escapeHtml(t('config.network_shares.runtime_title'))}</div>
            <div class="cc-actions-row">
                <button id="ns-reprobe-btn" type="button" class="btn-save cc-btn-secondary" onclick="nsReprobe()">↻ ${escapeHtml(t('config.network_shares.reprobe'))}</button>
                <button id="ns-create-btn" type="button" class="btn-save cc-btn-primary" onclick="nsOpenShareModal('')" disabled>＋ ${escapeHtml(t('config.network_shares.create'))}</button>
            </div>
            <div id="ns-action-result" class="adg-test-result" role="status" aria-live="polite"></div>
            <div id="ns-protocol-status"></div>
            <div id="ns-share-table"></div>
        </div>

        <div id="ns-share-modal" class="cc-modal-overlay is-hidden" onclick="nsCloseShareModal(event)">
            <div class="cc-modal-card" role="dialog" aria-modal="true" aria-labelledby="ns-modal-title" onclick="event.stopPropagation()">
                <div class="cc-modal-header">
                    <span id="ns-modal-title" class="cc-modal-title"></span>
                    <button type="button" class="cc-modal-close" onclick="nsCloseShareModal()" aria-label="${escapeAttr(t('config.network_shares.cancel'))}">✕</button>
                </div>
                <div id="ns-modal-error" class="cc-modal-error is-hidden" role="alert"></div>
                <div id="ns-modal-body"></div>
                <div class="cc-modal-actions">
                    <button type="button" class="btn-save cc-btn-secondary" onclick="nsCloseShareModal()">${escapeHtml(t('config.network_shares.cancel'))}</button>
                    <button id="ns-modal-save" type="button" class="btn-save cc-btn-primary" onclick="nsSaveShare()">${escapeHtml(t('config.network_shares.save_share'))}</button>
                </div>
            </div>
        </div>

        <div id="ns-delete-modal" class="cc-modal-overlay is-hidden" onclick="nsCloseDeleteModal(event)">
            <div class="cc-modal-card" role="dialog" aria-modal="true" aria-labelledby="ns-delete-title" onclick="event.stopPropagation()">
                <div class="cc-modal-header">
                    <span id="ns-delete-title" class="cc-modal-title">${escapeHtml(t('config.network_shares.delete_title'))}</span>
                    <button type="button" class="cc-modal-close" onclick="nsCloseDeleteModal()" aria-label="${escapeAttr(t('config.network_shares.cancel'))}">✕</button>
                </div>
                <div id="ns-delete-message" class="field-help"></div>
                <div class="cc-modal-actions">
                    <button type="button" class="btn-save cc-btn-secondary" onclick="nsCloseDeleteModal()">${escapeHtml(t('config.network_shares.cancel'))}</button>
                    <button id="ns-delete-confirm" type="button" class="btn-save btn-danger" onclick="nsDeleteShare()">${escapeHtml(t('config.network_shares.delete'))}</button>
                </div>
            </div>
        </div>
    </div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    nsLoadStatus();
}

function nsToggle(path, enabled, labelKey, helpKey) {
    return `<div class="field-group">
        <div class="field-label">${escapeHtml(t(labelKey))}</div>
        <div class="field-help">${escapeHtml(t(helpKey))}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabled ? ' on' : ''}" data-path="${escapeAttr(path)}" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${escapeHtml(enabled ? t('config.toggle.active') : t('config.toggle.inactive'))}</span>
        </div>
    </div>`;
}

function nsListEditor(path, rawValues, labelKey, helpKey, placeholderKey) {
    const values = Array.isArray(rawValues) ? rawValues : [];
    const token = path.replace(/[^a-z0-9]/gi, '-');
    return `<div class="field-group">
        <div class="field-label">${escapeHtml(t(labelKey))}</div>
        <div class="field-help">${escapeHtml(t(helpKey))}</div>
        <input id="ns-store-${token}" type="hidden" data-path="${escapeAttr(path)}" data-type="array" value="${escapeAttr(values.join(', '))}">
        <div class="three-d-printer-list">
            ${values.map((value, index) => `<div class="adg-password-row">
                <input class="field-input" type="text" value="${escapeAttr(value)}" data-ns-list="${escapeAttr(path)}" data-ns-index="${index}" onchange="nsChangeListValue(this)">
                <button type="button" class="btn-save cc-btn-secondary" data-ns-list="${escapeAttr(path)}" data-ns-index="${index}" onclick="nsRemoveListValue(this)">${escapeHtml(t('config.network_shares.remove'))}</button>
            </div>`).join('')}
        </div>
        <div class="adg-password-row">
            <input id="ns-new-${token}" class="field-input" type="text" placeholder="${escapeAttr(t(placeholderKey))}">
            <button type="button" class="btn-save cc-btn-secondary" data-ns-list="${escapeAttr(path)}" data-ns-input="ns-new-${token}" onclick="nsAddListValue(this)">＋ ${escapeHtml(t('config.network_shares.add'))}</button>
        </div>
    </div>`;
}

function nsConfigArray(path) {
    const value = getNestedValue(configData, path);
    return Array.isArray(value) ? value.slice() : [];
}

function nsSetConfigArray(path, values) {
    const normalized = values.map(value => String(value || '').trim()).filter(Boolean);
    setNestedValue(configData, path, normalized);
    if (window.AuraConfigState) window.AuraConfigState.set(path, normalized);
    setDirty(true);
    renderNetworkSharesSection(nsCurrentSection());
}

function nsCurrentSection() {
    for (const group of SECTIONS) {
        const section = group.items.find(item => item.key === 'network_shares');
        if (section) return section;
    }
    return { label: t('config.section.network_shares.label'), desc: t('config.section.network_shares.desc') };
}

function nsChangeListValue(input) {
    const values = nsConfigArray(input.dataset.nsList);
    values[Number(input.dataset.nsIndex)] = input.value;
    nsSetConfigArray(input.dataset.nsList, values);
}

function nsRemoveListValue(button) {
    const values = nsConfigArray(button.dataset.nsList);
    values.splice(Number(button.dataset.nsIndex), 1);
    nsSetConfigArray(button.dataset.nsList, values);
}

function nsAddListValue(button) {
    const input = document.getElementById(button.dataset.nsInput);
    const value = String((input || {}).value || '').trim();
    if (!value) {
        showToast(t('config.network_shares.value_required'), 'warn');
        return;
    }
    nsSetConfigArray(button.dataset.nsList, nsConfigArray(button.dataset.nsList).concat(value));
}

function nsSavedConfigRequired() {
    if (typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges()) {
        showToast(t('config.network_shares.save_first'), 'warn');
        return false;
    }
    return true;
}

async function nsRequest(path, options) {
    const response = await fetch(path, options);
    const data = response.status === 204 ? {} : await response.json().catch(() => ({}));
    if (!response.ok || data.status === 'error') {
        throw new Error(data.message || t('config.network_shares.request_failed'));
    }
    return data;
}

async function nsLoadStatus() {
    const banner = document.getElementById('ns-runtime-banner');
    try {
        const data = await nsRequest('/api/network-shares/status');
        nsRuntime = data.status || {};
        nsPermissions = data.permissions || {};
        if (nsRuntime.usable) {
            const list = await nsRequest('/api/network-shares');
            nsShares = Array.isArray(list.shares) ? list.shares : [];
        } else {
            nsShares = [];
        }
        nsRenderRuntime();
        nsRenderShares();
    } catch (error) {
        if (banner) {
            banner.className = 'adg-status-banner is-danger';
            banner.textContent = error.message || t('config.network_shares.status_failed');
        }
        nsShares = [];
        nsRenderShares();
    }
}

function nsRenderRuntime() {
    const banner = document.getElementById('ns-runtime-banner');
    if (banner) {
        banner.className = nsRuntime && nsRuntime.usable ? 'adg-status-banner is-success' : 'adg-status-banner is-warning';
        banner.textContent = nsRuntime && nsRuntime.usable
            ? t('config.network_shares.status_ready')
            : ((nsRuntime || {}).reason || t('config.network_shares.status_unavailable'));
    }
    const area = document.getElementById('ns-protocol-status');
    if (!area) return;
    const protocols = [['SMB', (nsRuntime || {}).smb || {}], ['NFS', (nsRuntime || {}).nfs || {}]];
    area.innerHTML = `<div class="cc-table-wrap"><table class="cc-table">
        <thead><tr class="cc-table-head">
            <th>${escapeHtml(t('config.network_shares.protocol'))}</th>
            <th>${escapeHtml(t('config.network_shares.installed'))}</th>
            <th>${escapeHtml(t('config.network_shares.service'))}</th>
            <th>${escapeHtml(t('config.network_shares.configured'))}</th>
            <th>${escapeHtml(t('config.network_shares.access'))}</th>
        </tr></thead>
        <tbody>${protocols.map(([name, status]) => {
            const access = status.writable
                ? escapeHtml(t('config.network_shares.write'))
                : (status.readable
                    ? `${escapeHtml(t('config.network_shares.read'))}${status.reason ? `<div class="field-help">${escapeHtml(status.reason)}</div>` : ''}`
                    : escapeHtml(status.reason || t('config.network_shares.unavailable')));
            return `<tr class="cc-device-row">
            <td class="cc-cell cc-cell-name">${name}</td>
            <td class="cc-cell">${escapeHtml(nsYesNo(status.installed))}</td>
            <td class="cc-cell">${escapeHtml(nsYesNo(status.service_active))}</td>
            <td class="cc-cell">${escapeHtml(nsYesNo(status.configured))}</td>
            <td class="cc-cell">${access}</td>
        </tr>`;
        }).join('')}</tbody>
    </table></div>`;
    const createButton = document.getElementById('ns-create-btn');
    if (createButton) {
        createButton.disabled = !(nsPermissions && !nsPermissions.readonly && nsPermissions.allow_create &&
            ((nsRuntime.smb || {}).writable || (nsRuntime.nfs || {}).writable));
    }
}

function nsYesNo(value) {
    return value ? t('config.network_shares.yes') : t('config.network_shares.no');
}

function nsRenderShares() {
    const area = document.getElementById('ns-share-table');
    if (!area) return;
    if (!nsShares.length) {
        area.innerHTML = `<div class="cc-empty">${escapeHtml(t('config.network_shares.no_shares'))}</div>`;
        return;
    }
    area.innerHTML = `<div class="cc-table-wrap"><table class="cc-table">
        <thead><tr class="cc-table-head">
            <th>${escapeHtml(t('config.network_shares.name'))}</th>
            <th>${escapeHtml(t('config.network_shares.protocol'))}</th>
            <th>${escapeHtml(t('config.network_shares.path'))}</th>
            <th>${escapeHtml(t('config.network_shares.state'))}</th>
            <th class="cc-th-actions">${escapeHtml(t('config.network_shares.actions'))}</th>
        </tr></thead>
        <tbody>${nsShares.map(share => {
            const state = share.drift ? t('config.network_shares.drifted') :
                (share.managed ? t('config.network_shares.managed') : t('config.network_shares.external'));
            const canUpdate = share.mutable && nsPermissions && nsPermissions.allow_update;
            const canDelete = share.mutable && nsPermissions && nsPermissions.allow_delete;
            return `<tr class="cc-device-row">
                <td class="cc-cell cc-cell-name">${escapeHtml(share.name || '')}</td>
                <td class="cc-cell">${escapeHtml(String(share.protocol || '').toUpperCase())}</td>
                <td class="cc-cell">${escapeHtml(share.path || '')}</td>
                <td class="cc-cell">${escapeHtml(state)}</td>
                <td class="cc-cell cc-cell-actions">
                    ${canUpdate ? `<button type="button" class="btn-save cc-btn-compact" data-share-id="${escapeAttr(share.id)}" onclick="nsOpenShareModal(this.dataset.shareId)">${escapeHtml(t('config.network_shares.edit'))}</button>` : ''}
                    ${canDelete ? `<button type="button" class="btn-save cc-btn-compact" data-share-id="${escapeAttr(share.id)}" onclick="nsOpenDeleteModal(this.dataset.shareId)">${escapeHtml(t('config.network_shares.delete'))}</button>` : '—'}
                </td>
            </tr>`;
        }).join('')}</tbody>
    </table></div>`;
}

async function nsReprobe() {
    if (!nsSavedConfigRequired()) return;
    await nsRunAction('ns-reprobe-btn', async () => {
        await nsRequest('/api/network-shares/reprobe', { method: 'POST' });
        await nsLoadStatus();
        showToast(t('config.network_shares.reprobe_ok'), 'success');
    });
}

function nsOpenShareModal(id) {
    if (!nsSavedConfigRequired()) return;
    const share = id ? nsShares.find(item => item.id === id) : null;
    if (id && !share) return;
    nsEditingID = id || '';
    const permissions = nsPermissions || {
        smb_enabled: true,
        nfs_enabled: true,
        smb_allow_guest: false,
        allowed_principals: [],
        allowed_clients: []
    };
    document.getElementById('ns-modal-title').textContent = share
        ? t('config.network_shares.edit_title')
        : t('config.network_shares.create_title');
    const protocol = share ? share.protocol : (permissions.smb_enabled ? 'smb' : 'nfs');
    const body = document.getElementById('ns-modal-body');
    body.innerHTML = `
        <div class="field-group">
            <div class="field-label">${escapeHtml(t('config.network_shares.protocol'))}</div>
            <select id="ns-form-protocol" class="field-select"${share ? ' disabled' : ''} onchange="nsRenderAccessFields()">
                ${permissions.smb_enabled ? `<option value="smb"${protocol === 'smb' ? ' selected' : ''}>SMB</option>` : ''}
                ${permissions.nfs_enabled ? `<option value="nfs"${protocol === 'nfs' ? ' selected' : ''}>NFS</option>` : ''}
            </select>
        </div>
        ${nsTextField('ns-form-name', 'config.network_shares.name', share ? share.name : '', !!share)}
        ${nsTextField('ns-form-path', 'config.network_shares.path', share ? share.path : '', !!share)}
        ${nsTextField('ns-form-comment', 'config.network_shares.comment', share ? share.comment : '', false)}
        <label class="toggle-wrap"><input id="ns-form-readonly" type="checkbox"${!share || share.read_only ? ' checked' : ''}> ${escapeHtml(t('config.network_shares.share_readonly'))}</label>
        <div id="ns-form-access"></div>`;
    nsRenderAccessFields(share);
    const error = document.getElementById('ns-modal-error');
    error.textContent = '';
    setHidden(error, true);
    setHidden(document.getElementById('ns-share-modal'), false);
}

function nsTextField(id, labelKey, value, disabled) {
    return `<div class="field-group">
        <div class="field-label">${escapeHtml(t(labelKey))}</div>
        <input id="${id}" class="field-input" type="text" value="${escapeAttr(value || '')}"${disabled ? ' disabled' : ''}>
    </div>`;
}

function nsRenderAccessFields(existing) {
    const area = document.getElementById('ns-form-access');
    if (!area) return;
    const permissions = nsPermissions || {
        smb_allow_guest: false,
        allowed_principals: [],
        allowed_clients: []
    };
    const protocol = document.getElementById('ns-form-protocol').value;
    const share = existing || (nsEditingID ? nsShares.find(item => item.id === nsEditingID) : null);
    const access = (share || {}).access || {};
    if (protocol === 'smb') {
        const levels = new Map((access.acl || []).map(entry => [entry.principal, entry.level]));
        area.innerHTML = `
            ${permissions.smb_allow_guest ? `<label class="toggle-wrap"><input id="ns-form-guest" type="checkbox"${access.guest ? ' checked' : ''}> ${escapeHtml(t('config.network_shares.guest'))}</label>` : ''}
            <div class="field-label">${escapeHtml(t('config.network_shares.acl'))}</div>
            ${(permissions.allowed_principals || []).map(principal => `<div class="adg-password-row">
                <span class="field-help">${escapeHtml(principal)}</span>
                <select class="field-select ns-acl-level" data-principal="${escapeAttr(principal)}">
                    ${['none', 'read', 'change', 'full', 'deny'].map(level => `<option value="${level}"${levels.get(principal) === level ? ' selected' : ''}>${escapeHtml(t('config.network_shares.level_' + level))}</option>`).join('')}
                </select>
            </div>`).join('')}
        `;
    } else {
        const selected = new Set(access.clients || []);
        area.innerHTML = `<div class="field-label">${escapeHtml(t('config.network_shares.clients'))}</div>
            ${(permissions.allowed_clients || []).map(client => `<label class="toggle-wrap">
                <input class="ns-client" type="checkbox" value="${escapeAttr(client)}"${selected.has(client) ? ' checked' : ''}> ${escapeHtml(client)}
            </label>`).join('')}`;
    }
}

function nsCollectShare() {
    const protocol = document.getElementById('ns-form-protocol').value;
    const share = {
        protocol,
        name: document.getElementById('ns-form-name').value.trim(),
        path: document.getElementById('ns-form-path').value.trim(),
        comment: document.getElementById('ns-form-comment').value.trim(),
        read_only: document.getElementById('ns-form-readonly').checked,
        access: {}
    };
    if (protocol === 'smb') {
        share.access.guest = !!(document.getElementById('ns-form-guest') || {}).checked;
        share.access.acl = Array.from(document.querySelectorAll('.ns-acl-level'))
            .filter(select => select.value !== 'none')
            .map(select => ({ principal: select.dataset.principal, level: select.value }));
    } else {
        share.access.clients = Array.from(document.querySelectorAll('.ns-client:checked')).map(input => input.value);
    }
    return share;
}

async function nsSaveShare() {
    const button = document.getElementById('ns-modal-save');
    const error = document.getElementById('ns-modal-error');
    button.disabled = true;
    try {
        const share = nsCollectShare();
        await nsRequest('/api/network-shares/validate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ operation: nsEditingID ? 'update' : 'create', share })
        });
        if (nsEditingID) {
            await nsRequest('/api/network-shares/' + encodeURIComponent(nsEditingID), {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ comment: share.comment, read_only: share.read_only, access: share.access })
            });
        } else {
            await nsRequest('/api/network-shares', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(share)
            });
        }
        nsCloseShareModal();
        await nsLoadStatus();
        showToast(t('config.network_shares.saved'), 'success');
    } catch (requestError) {
        error.textContent = requestError.message || t('config.network_shares.request_failed');
        setHidden(error, false);
    } finally {
        button.disabled = false;
    }
}

function nsCloseShareModal(event) {
    if (event && event.target !== event.currentTarget) return;
    nsEditingID = '';
    setHidden(document.getElementById('ns-share-modal'), true);
}

function nsOpenDeleteModal(id) {
    if (!nsSavedConfigRequired()) return;
    const share = nsShares.find(item => item.id === id);
    if (!share) return;
    nsEditingID = id;
    document.getElementById('ns-delete-message').textContent =
        t('config.network_shares.delete_message', { name: share.name });
    setHidden(document.getElementById('ns-delete-modal'), false);
}

function nsCloseDeleteModal(event) {
    if (event && event.target !== event.currentTarget) return;
    nsEditingID = '';
    setHidden(document.getElementById('ns-delete-modal'), true);
}

async function nsDeleteShare() {
    const id = nsEditingID;
    const button = document.getElementById('ns-delete-confirm');
    button.disabled = true;
    try {
        await nsRequest('/api/network-shares/' + encodeURIComponent(id), { method: 'DELETE' });
        nsCloseDeleteModal();
        await nsLoadStatus();
        showToast(t('config.network_shares.deleted'), 'success');
    } catch (error) {
        showToast(error.message || t('config.network_shares.request_failed'), 'error');
    } finally {
        button.disabled = false;
    }
}

async function nsRunAction(buttonID, action) {
    const button = document.getElementById(buttonID);
    const result = document.getElementById('ns-action-result');
    if (button) button.disabled = true;
    if (result) {
        result.className = 'adg-test-result';
        result.textContent = t('config.network_shares.working');
    }
    try {
        await action();
        if (result) {
            result.className = 'adg-test-result is-success';
            result.textContent = t('config.network_shares.action_ok');
        }
    } catch (error) {
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = error.message || t('config.network_shares.request_failed');
        }
        showToast(error.message || t('config.network_shares.request_failed'), 'error');
    } finally {
        if (button) button.disabled = false;
    }
}
