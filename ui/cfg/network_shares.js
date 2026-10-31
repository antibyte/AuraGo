// cfg/network_shares.js — local SMB/NFS server-share configuration and administration.
let nsShares = [];
let nsRuntime = null;
let nsPermissions = null;
let nsEditingID = '';

const nsReasonTranslationKeys = {
    disabled: 'config.network_shares.reason_disabled',
    protocol_disabled: 'config.network_shares.reason_disabled',
    unsupported_os: 'config.network_shares.reason_unavailable',
    unavailable: 'config.network_shares.reason_unavailable',
    probe_failed: 'config.network_shares.reason_probe_failed',
    no_readable_backend: 'config.network_shares.reason_unavailable',
    no_available_root: 'config.network_shares.reason_no_roots',
    not_installed: 'config.network_shares.reason_not_installed',
    not_readable: 'config.network_shares.reason_unavailable',
    not_configured: 'config.network_shares.reason_not_configured',
    registry_shares_disabled: 'config.network_shares.reason_not_configured',
    service_inactive: 'config.network_shares.reason_service_inactive',
    backend_unwritable: 'config.network_shares.reason_privileges',
    readonly: 'config.network_shares.reason_readonly',
    permission_disabled: 'config.network_shares.reason_permissions',
    docker_restricted: 'config.network_shares.reason_docker',
    no_new_privileges: 'config.network_shares.reason_hardening',
    protect_system_strict: 'config.network_shares.reason_hardening',
    privilege_required: 'config.network_shares.reason_privileges',
    root_not_absolute: 'config.network_shares.reason_root_unavailable',
    root_unavailable: 'config.network_shares.reason_root_unavailable',
    root_not_directory: 'config.network_shares.reason_root_unavailable',
    root_unresolvable: 'config.network_shares.reason_root_unavailable'
};

const nsErrorTranslationKeys = {
    NETWORK_SHARES_DISABLED: 'config.network_shares.error_unavailable',
    SHARE_PROTOCOL_UNAVAILABLE: 'config.network_shares.error_unavailable',
    SHARE_OUTSIDE_ALLOWED_ROOT: 'config.network_shares.error_invalid',
    SHARE_READ_ONLY: 'config.network_shares.error_forbidden',
    SHARE_PERMISSION_DENIED: 'config.network_shares.error_forbidden',
    SHARE_NOT_MANAGED: 'config.network_shares.error_not_managed',
    SHARE_CONFLICT: 'config.network_shares.error_conflict',
    SHARE_DRIFT: 'config.network_shares.error_drift',
    SHARE_APPLY_FAILED: 'config.network_shares.error_apply',
    SHARE_INVALID_ARGUMENT: 'config.network_shares.error_invalid',
    SHARE_NOT_FOUND: 'config.network_shares.error_not_found'
};

const nsDriftTranslationKeys = {
    name_changed: 'config.network_shares.drift_identity',
    path_changed: 'config.network_shares.drift_identity',
    marker_missing: 'config.network_shares.drift_identity',
    ownership_mismatch: 'config.network_shares.drift_identity',
    inactive: 'config.network_shares.drift_inactive',
    missing: 'config.network_shares.drift_inactive',
    read_only_changed: 'config.network_shares.drift_configuration',
    comment_changed: 'config.network_shares.drift_configuration',
    access_changed: 'config.network_shares.drift_configuration',
    orphaned_marker: 'config.network_shares.drift_orphan',
    unsafe_admin_users: 'config.network_shares.drift_unsafe',
    rollback_failed: 'config.network_shares.drift_rollback',
    outside_allowed_roots: 'config.network_shares.drift_outside'
};

function nsTranslatedKey(key, fallbackKey) {
    if (!key) return t(fallbackKey);
    const translated = t(key);
    return translated && translated !== key ? translated : t(fallbackKey);
}

function nsLocalizedReason(code) {
    return nsTranslatedKey(nsReasonTranslationKeys[String(code || '')], 'config.network_shares.reason_unavailable');
}

function nsLocalizedError(code) {
    return nsTranslatedKey(nsErrorTranslationKeys[String(code || '')], 'config.network_shares.request_failed');
}

function nsLocalizedDrift(code) {
    return nsTranslatedKey(nsDriftTranslationKeys[String(code || '')], 'config.network_shares.drift_configuration');
}

function renderNetworkSharesSection(section) {
    const data = configData.network_shares || {};
    const smb = data.smb || {};
    const nfs = data.nfs || {};
    const html = `
    <div class="cfg-section active">
        <div class="section-header">${escapeHtml(section.label)}</div>
        <div class="section-desc">${escapeHtml(section.desc)}</div>
        ${cfgStatusBanner.html('ns-runtime-banner', 'checking', { detail: t('config.network_shares.loading') })}

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
            <div class="cfgdev-actions-row">
                <button id="ns-reprobe-btn" type="button" class="btn-save cfgdev-btn-secondary" onclick="nsReprobe()">${cfgIcon('refresh')}<span>${escapeHtml(t('config.network_shares.reprobe'))}</span></button>
                <button id="ns-create-btn" type="button" class="btn-save cfgdev-btn-primary" onclick="nsOpenShareModal('')" disabled>${cfgIcon('plus')}<span>${escapeHtml(t('config.network_shares.create'))}</span></button>
            </div>
            ${cfgActionResult.html('ns-action-result')}
            <div id="ns-protocol-status"></div>
            <div id="ns-share-table"></div>
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
            ${values.map((value, index) => `<div class="cfg-secret-row">
                <input class="field-input" type="text" value="${escapeAttr(value)}" data-ns-list="${escapeAttr(path)}" data-ns-index="${index}" onchange="nsChangeListValue(this)">
                ${cfgDeleteButton({ label: t('config.network_shares.remove'), attrs: { 'data-ns-list': path, 'data-ns-index': index, onclick: 'nsRemoveListValue(this)' } })}
            </div>`).join('')}
        </div>
        <div class="cfg-secret-row">
            <input id="ns-new-${token}" class="field-input" type="text" placeholder="${escapeAttr(t(placeholderKey))}">
            <button type="button" class="btn-secondary" data-ns-list="${escapeAttr(path)}" data-ns-input="ns-new-${token}" onclick="nsAddListValue(this)">${cfgIcon('plus')}<span>${escapeHtml(t('config.network_shares.add'))}</span></button>
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
        cfgActionResult.show('ns-action-result', 'warning', t('config.network_shares.value_required'));
        return;
    }
    nsSetConfigArray(button.dataset.nsList, nsConfigArray(button.dataset.nsList).concat(value));
}

function nsSavedConfigRequired() {
    if (typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges()) {
        cfgActionResult.show('ns-action-result', 'warning', t('config.network_shares.save_first'));
        return false;
    }
    return true;
}

async function nsRequest(path, options) {
    let response;
    try {
        response = await fetch(path, options);
    } catch (_) {
        throw new Error(t('config.network_shares.request_failed'));
    }
    const data = response.status === 204 ? {} : await response.json().catch(() => ({}));
    if (!response.ok || data.status === 'error') {
        const error = new Error(nsLocalizedError(data.code));
        error.code = data.code || '';
        throw error;
    }
    return data;
}

async function nsLoadStatus() {
    cfgStatusBanner('ns-runtime-banner', 'checking', { detail: t('config.network_shares.loading') });
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
        cfgStatusBanner('ns-runtime-banner', 'error', { detail: error.message || t('config.network_shares.status_failed'), retry: nsLoadStatus });
        nsShares = [];
        nsRenderShares();
    }
}

function nsRenderRuntime() {
    if (document.getElementById('ns-runtime-banner')) {
        const reasonCode = String((nsRuntime || {}).reason_code || '');
        const state = nsRuntime && nsRuntime.usable ? 'connected'
            : (['disabled', 'protocol_disabled'].includes(reasonCode) ? 'off' : (['not_configured', 'registry_shares_disabled'].includes(reasonCode) ? 'unconfigured' : 'degraded'));
        cfgStatusBanner('ns-runtime-banner', state, {
            detail: nsRuntime && nsRuntime.usable ? t('config.network_shares.status_ready') : nsLocalizedReason(reasonCode),
            retry: state === 'degraded' ? nsLoadStatus : null
        });
    }
    const area = document.getElementById('ns-protocol-status');
    if (!area) return;
    const protocols = [['SMB', (nsRuntime || {}).smb || {}], ['NFS', (nsRuntime || {}).nfs || {}]];
    area.innerHTML = `<div class="cfgdev-table-wrap"><table class="cfgdev-table">
        <thead><tr class="cfgdev-table-head">
            <th>${escapeHtml(t('config.network_shares.protocol'))}</th>
            <th>${escapeHtml(t('config.network_shares.installed'))}</th>
            <th>${escapeHtml(t('config.network_shares.service'))}</th>
            <th>${escapeHtml(t('config.network_shares.configured'))}</th>
            <th>${escapeHtml(t('config.network_shares.access'))}</th>
        </tr></thead>
        <tbody>${protocols.map(([name, status]) => {
            const reason = status.reason_code ? nsLocalizedReason(status.reason_code) : '';
            const access = status.writable
                ? escapeHtml(t('config.network_shares.write'))
                : (status.readable
                    ? `${escapeHtml(t('config.network_shares.read'))}${reason ? `<div class="field-help">${escapeHtml(reason)}</div>` : ''}`
                    : escapeHtml(reason || t('config.network_shares.unavailable')));
            return `<tr class="cfgdev-device-row">
            <td class="cfgdev-cell cfgdev-cell-name">${name}</td>
            <td class="cfgdev-cell">${escapeHtml(nsYesNo(status.installed))}</td>
            <td class="cfgdev-cell">${escapeHtml(nsYesNo(status.service_active))}</td>
            <td class="cfgdev-cell">${escapeHtml(nsYesNo(status.configured))}</td>
            <td class="cfgdev-cell">${access}</td>
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
        cfgListState(area, { state: 'empty', title: t('config.network_shares.no_shares') });
        return;
    }
    area.innerHTML = `<div class="cfgdev-table-wrap"><table class="cfgdev-table">
        <thead><tr class="cfgdev-table-head">
            <th>${escapeHtml(t('config.network_shares.name'))}</th>
            <th>${escapeHtml(t('config.network_shares.protocol'))}</th>
            <th>${escapeHtml(t('config.network_shares.path'))}</th>
            <th>${escapeHtml(t('config.network_shares.state'))}</th>
            <th class="cfgdev-th-actions">${escapeHtml(t('config.network_shares.actions'))}</th>
        </tr></thead>
        <tbody>${nsShares.map(share => {
            const state = share.drift ? t('config.network_shares.drifted') :
                (share.managed ? t('config.network_shares.managed') : t('config.network_shares.external'));
            const driftDetail = share.drift
                ? `<div class="field-help">${escapeHtml(nsLocalizedDrift(share.drift))}</div>`
                : '';
            const canUpdate = share.mutable && nsPermissions && nsPermissions.allow_update;
            const canDelete = share.mutable && nsPermissions && nsPermissions.allow_delete;
            return `<tr class="cfgdev-device-row">
                <td class="cfgdev-cell cfgdev-cell-name">${escapeHtml(share.name || '')}</td>
                <td class="cfgdev-cell">${escapeHtml(String(share.protocol || '').toUpperCase())}</td>
                <td class="cfgdev-cell">${escapeHtml(share.path || '')}</td>
                <td class="cfgdev-cell">${escapeHtml(state)}${driftDetail}</td>
                <td class="cfgdev-cell cfgdev-cell-actions">
                    ${canUpdate ? `<button type="button" class="btn-secondary cfgdev-btn-compact" data-share-id="${escapeAttr(share.id)}" onclick="nsOpenShareModal(this.dataset.shareId)">${cfgIcon('edit')}<span>${escapeHtml(t('config.network_shares.edit'))}</span></button>` : ''}
                    ${canDelete ? cfgDeleteButton({ label: t('config.network_shares.delete'), attrs: { 'data-share-id': share.id, onclick: 'nsDeleteShare(this)' } }) : '—'}
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
        return { status: 'ok', message: t('config.network_shares.reprobe_ok') };
    });
}

const NS_SHARE_DIALOG = 'ns-share-dialog';

// The share dialog is built fresh on every open and removed again on close.
function nsOpenShareModal(id) {
    if (!nsSavedConfigRequired()) return;
    const share = id ? nsShares.find(item => item.id === id) : null;
    if (id && !share) return;
    const protocols = share ? [share.protocol] : nsWritableProtocols();
    if (!protocols.length) {
        cfgActionResult.show('ns-action-result', 'warning', t('config.network_shares.reason_unavailable'));
        return;
    }
    nsEditingID = id || '';
    const protocol = protocols[0];
    const old = document.getElementById(NS_SHARE_DIALOG);
    if (old) old.remove();
    document.body.insertAdjacentHTML('beforeend', cfgModal({
        id: NS_SHARE_DIALOG,
        title: share ? t('config.network_shares.edit_title') : t('config.network_shares.create_title'),
        size: 'lg',
        body: `
        ${cfgActionResult.html('ns-form-error')}
        <div class="field-group">
            <label class="field-label" for="ns-form-protocol">${escapeHtml(t('config.network_shares.protocol'))}</label>
            <select id="ns-form-protocol" class="field-select"${share ? ' disabled' : ''} onchange="nsRenderAccessFields()">
                ${protocols.map(value => `<option value="${value}"${protocol === value ? ' selected' : ''}>${value.toUpperCase()}</option>`).join('')}
            </select>
        </div>
        ${nsTextField('ns-form-name', 'config.network_shares.name', share ? share.name : '', !!share)}
        ${nsTextField('ns-form-path', 'config.network_shares.path', share ? share.path : '', !!share)}
        ${nsTextField('ns-form-comment', 'config.network_shares.comment', share ? share.comment : '', false)}
        ${nsDialogToggle('ns-form-readonly', !share || share.read_only, 'config.network_shares.share_readonly')}
        <div id="ns-form-access"></div>`,
        actions: [
            `<button type="button" class="btn-secondary" onclick="closeModal('${NS_SHARE_DIALOG}')">${escapeHtml(t('config.network_shares.cancel'))}</button>`,
            `<button id="ns-save-btn" type="button" class="btn-save" onclick="nsSaveShare()">${escapeHtml(t('config.network_shares.save_share'))}</button>`
        ]
    }));
    const dialog = document.getElementById(NS_SHARE_DIALOG);
    if (typeof enhanceConfigControls === 'function') enhanceConfigControls(dialog);
    nsRenderAccessFields(share);
    openModal(NS_SHARE_DIALOG, { initialFocus: share ? '#ns-form-comment' : '#ns-form-name', onClose: () => { nsEditingID = ''; dialog.remove(); } });
}

// On/off switch inside a dialog: not part of the config draft, so it has its own handler instead of toggleBool.
function nsDialogToggle(id, on, labelKey) {
    return `<div class="field-group cfg-toggle-row">
        <span class="cfg-toggle-label" id="${id}-label">${escapeHtml(t(labelKey))}</span>
        <div class="toggle${on ? ' on' : ''}" id="${id}" role="switch" tabindex="0" aria-checked="${on ? 'true' : 'false'}" aria-labelledby="${id}-label" onclick="nsDialogToggleFlip(this)"></div>
    </div>`;
}

function nsDialogToggleFlip(el) {
    el.classList.toggle('on');
    syncToggleA11y(el);
}

function nsWritableProtocols() {
    const permissions = nsPermissions || {};
    const runtime = nsRuntime || {};
    const protocols = [];
    if (permissions.smb_enabled && (runtime.smb || {}).writable) protocols.push('smb');
    if (permissions.nfs_enabled && (runtime.nfs || {}).writable) protocols.push('nfs');
    return protocols;
}

function nsTextField(id, labelKey, value, disabled) {
    return `<div class="field-group">
        <label class="field-label" for="${id}">${escapeHtml(t(labelKey))}</label>
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
        const availableLevels = (nsRuntime && (nsRuntime.smb || {}).backend === 'samba')
            ? ['none', 'read', 'change', 'deny']
            : ['none', 'read', 'change', 'full', 'deny'];
        area.innerHTML = `
            ${permissions.smb_allow_guest ? nsDialogToggle('ns-form-guest', !!access.guest, 'config.network_shares.guest') : ''}
            <div class="field-label">${escapeHtml(t('config.network_shares.acl'))}</div>
            ${(permissions.allowed_principals || []).map(principal => `<div class="cfg-secret-row">
                <span class="field-help">${escapeHtml(principal)}</span>
                <select class="field-select ns-acl-level" aria-label="${escapeAttr(principal)}" data-principal="${escapeAttr(principal)}">
                    ${availableLevels.map(level => `<option value="${level}"${levels.get(principal) === level ? ' selected' : ''}>${escapeHtml(t('config.network_shares.level_' + level))}</option>`).join('')}
                </select>
            </div>`).join('')}
        `;
    } else {
        const selected = new Set(access.clients || []);
        area.innerHTML = `<div class="field-label">${escapeHtml(t('config.network_shares.clients'))}</div>
            ${(permissions.allowed_clients || []).map(client => `<label class="cfg-choice-item">
                <input class="ns-client" type="checkbox" data-cfg-choice value="${escapeAttr(client)}"${selected.has(client) ? ' checked' : ''}> ${escapeHtml(client)}
            </label>`).join('')}`;
    }
    if (typeof enhanceConfigControls === 'function') enhanceConfigControls(area);
}

function nsCollectShare() {
    const protocol = document.getElementById('ns-form-protocol').value;
    const share = {
        protocol,
        name: document.getElementById('ns-form-name').value.trim(),
        path: document.getElementById('ns-form-path').value.trim(),
        comment: document.getElementById('ns-form-comment').value.trim(),
        read_only: document.getElementById('ns-form-readonly').classList.contains('on'),
        access: {}
    };
    if (protocol === 'smb') {
        share.access.guest = !!(document.getElementById('ns-form-guest') && document.getElementById('ns-form-guest').classList.contains('on'));
        share.access.acl = Array.from(document.querySelectorAll('.ns-acl-level'))
            .filter(select => select.value !== 'none')
            .map(select => ({ principal: select.dataset.principal, level: select.value }));
    } else {
        share.access.clients = Array.from(document.querySelectorAll('.ns-client:checked')).map(input => input.value);
    }
    return share;
}

async function nsSaveShare() {
    const button = document.getElementById('ns-save-btn');
    await cfgActionResult(button, async () => {
        const share = nsCollectShare();
        await nsRequest('/api/network-shares/validate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                operation: nsEditingID ? 'update' : 'create',
                id: nsEditingID || undefined,
                share
            })
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
        return { status: 'ok' };
    }, {
        target: 'ns-form-error',
        pending: t('config.network_shares.working'),
        success: t('config.network_shares.saved'),
        reasonText: nsReasonText,
        onSuccess: async () => {
            closeModal(NS_SHARE_DIALOG);
            await nsLoadStatus();
            cfgActionResult.show('ns-action-result', 'success', t('config.network_shares.saved'));
        }
    });
}

// Asks first (permanent), then deletes behind the pressed row button; the outcome shows in the toolbar result line.
async function nsDeleteShare(button) {
    if (!nsSavedConfigRequired()) return;
    const id = button && button.dataset ? button.dataset.shareId : '';
    const share = nsShares.find(item => item.id === id);
    if (!share) return;
    const answer = await showConfirmDialog({
        title: t('config.common.delete_item', { name: share.name }),
        message: t('config.network_shares.delete_message', { name: share.name }),
        danger: true,
        confirmText: t('config.network_shares.delete')
    });
    if (!answer.confirmed) return;
    await cfgActionResult(button, async () => {
        await nsRequest('/api/network-shares/' + encodeURIComponent(id), { method: 'DELETE' });
        return { status: 'ok' };
    }, {
        target: 'ns-action-result',
        pending: t('config.network_shares.working'),
        success: t('config.network_shares.deleted'),
        reasonText: nsReasonText,
        onSuccess: () => nsLoadStatus()
    });
}

// Failures show the localized text for the error code, never the raw backend message.
function nsReasonText(body, ctx) {
    if (body && typeof body === 'object' && body.code) return nsLocalizedError(body.code);
    return '';
}

// Runs a toolbar action behind its busy button; the reason (a localized error from nsRequest) stays next to it.
async function nsRunAction(buttonID, action) {
    return cfgActionResult(document.getElementById(buttonID), action, {
        target: 'ns-action-result',
        pending: t('config.network_shares.working'),
        success: o => (o.body && o.body.message) || t('config.network_shares.action_ok'),
        reasonText: nsReasonText,
        lock: '.cfgdev-actions-row button'
    });
}
