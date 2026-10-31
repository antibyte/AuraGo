let sqlConnCache = [];
let sqlConnModalRevision = 0;

const SQLCONN_DIALOG = 'sqlconn-dialog';

window.addEventListener('cfg:section-leave', function () {
    const dialog = document.getElementById(SQLCONN_DIALOG);
    if (dialog) {
        closeModal(SQLCONN_DIALOG);
        dialog.remove();
    }
});

function renderSQLConnectionsSection(section) {
    // A dialog left over from an earlier render of this section would carry stale ids.
    const stale = document.getElementById(SQLCONN_DIALOG);
    if (stale) {
        closeModal(SQLCONN_DIALOG);
        stale.remove();
    }

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    const sqlCfg = configData['sql_connections'] || {};
    const sqlEnabled = !!sqlCfg.enabled;
    const sqlReadonly = !!sqlCfg.readonly;
    const sqlAllowManagement = !!sqlCfg.allow_management;

    html += `<div class="sb-card">
        <div class="sb-card-header">
            <span class="sb-card-icon">${cfgIcon('settings')}</span>
            <span class="sb-card-title">${t('config.sql_connections.settings_title')}</span>
        </div>
        <div class="sb-card-body">
            <div class="field-group">
                <div class="field-label">${t('config.sql_connections.enabled_label')}</div>
                <div class="toggle-wrap">
                    <div class="toggle${sqlEnabled ? ' on' : ''}" data-path="sql_connections.enabled" onclick="toggleBool(this)"></div>
                    <span class="toggle-label">${sqlEnabled ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
                </div>
                <div class="field-help">${t('config.sql_connections.enabled_help')}</div>
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.sql_connections.readonly_label')}</div>
                <div class="toggle-wrap">
                    <div class="toggle${sqlReadonly ? ' on' : ''}" data-path="sql_connections.readonly" onclick="toggleBool(this)"></div>
                    <span class="toggle-label">${sqlReadonly ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
                </div>
                <div class="field-help">${t('config.sql_connections.readonly_help')}</div>
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.sql_connections.allow_management_label')}</div>
                <div class="toggle-wrap">
                    <div class="toggle${sqlAllowManagement ? ' on' : ''}" data-path="sql_connections.allow_management" onclick="toggleBool(this)"></div>
                    <span class="toggle-label">${sqlAllowManagement ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
                </div>
                <div class="field-help">${t('config.sql_connections.allow_management_help')}</div>
            </div>
        </div>
    </div>`;

    html += `
    <div class="sqlconn-toolbar">
        <div class="sqlconn-toolbar-left">
            <input type="text" id="sqlconn-filter" class="field-input sqlconn-filter-input" aria-label="${escapeAttr(t('config.sql_connections.filter_label'))}" aria-describedby="sqlconn-filter-help" placeholder="${t('config.sql_connections.filter_placeholder')}" oninput="sqlConnApplyFilter()">
            <div class="field-help" id="sqlconn-filter-help">${t('config.sql_connections.filter_help')}</div>
        </div>
        <button class="btn-save sqlconn-add-btn" onclick="sqlConnShowModal()">
            ${cfgIcon('plus')} ${t('config.sql_connections.add_connection')}
        </button>
    </div>
    ${cfgActionResult.html('sqlconn-list-result')}
    <div id="sqlconn-table-wrap" class="sql-table-wrap">
        <table class="sql-table">
            <thead>
                <tr class="sql-table-head-row">
                    <th class="sql-table-th">${t('config.sql_connections.col_name')}</th>
                    <th class="sql-table-th">${t('config.sql_connections.col_driver')}</th>
                    <th class="sql-table-th">${t('config.sql_connections.col_host')}</th>
                    <th class="sql-table-th">${t('config.sql_connections.col_database')}</th>
                    <th class="sql-table-th">${t('config.sql_connections.col_description')}</th>
                    <th class="sql-table-th">${t('config.sql_connections.col_permissions')}</th>
                    <th class="sql-table-th sql-table-th-actions">${t('config.sql_connections.col_actions')}</th>
                </tr>
            </thead>
            <tbody id="sqlconn-tbody"></tbody>
        </table>
    </div>
    <div id="sqlconn-state"></div>`;

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    sqlConnLoad();
}

// One permission switch of the dialog. Dangerous ones (data-cfg-confirm-enable) ask before they turn on;
// the dialog is not part of the config draft, so nothing is marked dirty.
function sqlConnPermToggle(el) {
    const turningOn = !el.classList.contains('on');
    const apply = () => {
        el.classList.toggle('on', turningOn);
        syncToggleA11y(el);
    };
    const category = el.getAttribute('data-cfg-confirm-enable');
    if (!turningOn || !category) { apply(); return; }
    const label = (document.getElementById(el.id + '-label') || {}).textContent || '';
    showConfirmDialog({
        title: String(t('config.confirm_enable.title')).replace('{label}', () => label),
        message: String(t('config.confirm_enable.' + category)).replace('{label}', () => label),
        danger: true
    }).then(answer => { if (answer.confirmed) apply(); });
}

function sqlConnPermSet(id, on) {
    const el = document.getElementById(id);
    if (!el) return;
    el.classList.toggle('on', !!on);
    syncToggleA11y(el);
}

function sqlConnPermOn(id) {
    const el = document.getElementById(id);
    return !!el && el.classList.contains('on');
}

function sqlConnPermItem(id, labelKey, on, extra) {
    return `<div class="sqlconn-permission-item">
        <span class="sqlconn-permission-label" id="${id}-label">${t(labelKey)}</span>
        <div class="toggle${on ? ' on' : ''}" id="${id}"${extra || ''} role="switch" aria-labelledby="${id}-label" onclick="sqlConnPermToggle(this)"></div>
    </div>`;
}

// The connection dialog is built once per open section and reused for every add/edit.
function sqlConnEnsureDialog() {
    if (document.getElementById(SQLCONN_DIALOG)) return;
    const body = `
            <input type="hidden" id="sqlconn-edit-id">

            <div class="field-group sql-field-group">
                <div class="field-label">${t('config.sql_connections.name_label')} *</div>
                <div class="field-help">${t('config.sql_connections.name_help')}</div>
                <input type="text" id="sqlconn-field-name" class="field-input" placeholder="${t('config.sql_connections.name_placeholder')}">
            </div>

            <div class="sqlconn-grid-two">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.driver_label')} *</div>
                    <div class="field-help">${t('config.sql_connections.driver_help')}</div>
                    <select id="sqlconn-field-driver" class="field-input sql-select-compact" onchange="sqlConnDriverChanged()">
                        <option value="postgres">${t('config.sql_connections.driver_postgres')}</option>
                        <option value="mysql">${t('config.sql_connections.driver_mysql')}</option>
                        <option value="sqlite">${t('config.sql_connections.driver_sqlite')}</option>
                    </select>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.port_label')}</div>
                    <div class="field-help">${t('config.sql_connections.port_help')}</div>
                    <input type="number" id="sqlconn-field-port" class="field-input" placeholder="${t('config.sql_connections.port_postgres')}">
                </div>
            </div>

            <div class="sqlconn-grid-host" id="sqlconn-host-row">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.host_label')}</div>
                    <div class="field-help">${t('config.sql_connections.host_help')}</div>
                    <input type="text" id="sqlconn-field-host" class="field-input" placeholder="${t('config.sql_connections.host_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.ssl_mode_label')}</div>
                    <div class="field-help">${t('config.sql_connections.ssl_mode_help')}</div>
                    <select id="sqlconn-field-ssl" class="field-input sql-select-compact">
                        <option value="disable">${t('config.sql_connections.ssl_disable')}</option>
                        <option value="require">${t('config.sql_connections.ssl_require')}</option>
                        <option value="verify-ca">${t('config.sql_connections.ssl_verify_ca')}</option>
                        <option value="verify-full">${t('config.sql_connections.ssl_verify_full')}</option>
                    </select>
                </div>
            </div>

            <div class="field-group sql-field-group-mt">
                <div class="field-label">${t('config.sql_connections.database_label')} *</div>
                <div class="field-help">${t('config.sql_connections.database_help')}</div>
                <input type="text" id="sqlconn-field-database" class="field-input" placeholder="${t('config.sql_connections.database_placeholder')}">
            </div>

            <div id="sqlconn-import-row" class="field-group is-hidden">
                <label for="sqlconn-import-file">${t('config.sql_connections.sqlite_import')}</label>
                <input type="file" id="sqlconn-import-file" class="field-input" accept=".db,.sqlite,.sqlite3,application/vnd.sqlite3">
                <div class="field-help">${t('config.sql_connections.sqlite_import_help')}</div>
            </div>

            <div class="field-group sql-field-group-mt">
                <div class="field-label">${t('config.sql_connections.description_label')}</div>
                <div class="field-help">${t('config.sql_connections.description_help')}</div>
                <input type="text" id="sqlconn-field-desc" class="field-input" placeholder="${t('config.sql_connections.description_placeholder')}">
            </div>

            <div class="sqlconn-grid-two sqlconn-creds-row" id="sqlconn-creds-row">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.username_label')}</div>
                    <div class="field-help">${t('config.sql_connections.username_help')}</div>
                    <input type="text" id="sqlconn-field-username" class="field-input" placeholder="${t('config.sql_connections.username_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.password_label')}</div>
                    <div class="field-help">${t('config.sql_connections.password_help')}</div>
                    <input type="password" id="sqlconn-field-password" class="field-input" placeholder="${t('config.sql_connections.password_placeholder')}">
                </div>
            </div>

            <div class="sqlconn-permissions-card">
                <div class="field-label sql-perm-title">${t('config.sql_connections.permissions_title')}</div>
                <div class="sqlconn-permissions-grid">
                    ${sqlConnPermItem('sqlconn-perm-read', 'config.sql_connections.perm_read', true)}
                    ${sqlConnPermItem('sqlconn-perm-write', 'config.sql_connections.perm_write', false)}
                    ${sqlConnPermItem('sqlconn-perm-change', 'config.sql_connections.perm_change', false)}
                    ${sqlConnPermItem('sqlconn-perm-delete', 'config.sql_connections.perm_delete', false, ' data-cfg-confirm-enable="delete"')}
                </div>
                <div class="field-help sql-perm-hint">${t('config.sql_connections.permissions_hint')}</div>
            </div>

            ${cfgActionResult.html('sqlconn-test-result')}
            ${cfgActionResult.html('sqlconn-save-result')}`;
    const actions = [
        `<button type="button" class="btn-save sqlconn-test-btn" id="sqlconn-test-btn" data-config-action-draft="true" onclick="sqlConnTest()">${cfgIcon('plug')} ${t('config.sql_connections.test_connection')}</button>`,
        `<button type="button" class="btn-save sqlconn-secondary-btn" onclick="sqlConnCloseModal()">${t('config.sql_connections.cancel')}</button>`,
        `<button type="button" class="btn-save sqlconn-primary-btn" id="sqlconn-save-btn" onclick="sqlConnSave()">${t('config.sql_connections.save')}</button>`
    ];
    document.body.insertAdjacentHTML('beforeend', cfgModal({ id: SQLCONN_DIALOG, title: '', size: 'lg', body, actions }));
    const dialog = document.getElementById(SQLCONN_DIALOG);
    if (typeof enhanceConfigControls === 'function') enhanceConfigControls(dialog);
}

async function sqlConnLoad() {
    const tbody = document.getElementById('sqlconn-tbody');
    const table = document.getElementById('sqlconn-table-wrap');
    if (!tbody || !table) return;
    table.classList.add('is-hidden');
    cfgListState('sqlconn-state', { state: 'loading', title: t('config.sql_connections.loading') });

    try {
        const resp = await fetch('/api/sql-connections');
        if (!resp.ok) throw new Error(await resp.text());
        sqlConnCache = await resp.json();
        if (!Array.isArray(sqlConnCache)) sqlConnCache = [];
    } catch (e) {
        cfgListState('sqlconn-state', { state: 'error', title: t('config.common.load_failed'), text: e.message, retry: sqlConnLoad });
        return;
    }

    sqlConnApplyFilter();
}

function sqlConnRenderRows(connections) {
    const tbody = document.getElementById('sqlconn-tbody');
    tbody.innerHTML = '';
    const permStatus = { R: 'ok', W: 'warn', C: 'warn', D: 'error' };
    connections.forEach(c => {
        const perms = [];
        if (c.allow_read) perms.push('R');
        if (c.allow_write) perms.push('W');
        if (c.allow_change) perms.push('C');
        if (c.allow_delete) perms.push('D');
        const permBadges = perms.map(p => {
            const labels = { R: t('config.sql_connections.badge_read'), W: t('config.sql_connections.badge_write'), C: t('config.sql_connections.badge_change'), D: t('config.sql_connections.badge_delete') };
            return cfgBadge(permStatus[p], labels[p]);
        }).join('');

        const hostDisplay = c.driver === 'sqlite' ? '—' : (c.host || 'localhost') + (c.port ? ':' + c.port : '');
        const tr = document.createElement('tr');
        tr.classList.add('sql-table-row');
        tr.dataset.id = c.id;
        tr.innerHTML = `
            <td class="sql-td-name">${escapeHtml(c.name)}</td>
            <td class="sql-td">${cfgBadge('idle', c.driver)}</td>
            <td class="sql-td-host">${escapeHtml(hostDisplay)}</td>
            <td class="sql-td">${escapeHtml(c.driver === 'sqlite' ? t(c.state === 'sqlite_import_required' ? 'config.sql_connections.sqlite_import_required' : 'config.sql_connections.driver_sqlite') : (c.database_name || '—'))}</td>
            <td class="sql-td-desc" title="${escapeHtml(c.description || '')}">${escapeHtml(c.description || '—')}</td>
            <td class="sql-td">${permBadges || '—'}</td>
            <td class="sql-td-actions">
                <button type="button" onclick="sqlConnTestExisting(this.dataset.id)" data-id="${escapeAttr(c.id)}" class="sql-action-btn" title="${escapeAttr(t('config.sql_connections.test_tooltip'))}" aria-label="${escapeAttr(t('config.sql_connections.test_tooltip') + ': ' + c.name)}">${cfgIcon('plug')}</button>
                <button type="button" onclick="sqlConnShowModal(this.dataset.id)" data-id="${escapeAttr(c.id)}" class="sql-action-btn" title="${escapeAttr(t('config.sql_connections.edit_tooltip'))}" aria-label="${escapeAttr(t('config.sql_connections.edit_tooltip') + ': ' + c.name)}">${cfgIcon('edit')}</button>
                ${cfgDeleteButton({ iconOnly: true, label: t('config.common.delete_item', { name: c.name }), attrs: { 'data-id': c.id, 'data-name': c.name, onclick: 'sqlConnDelete(this.dataset.id, this.dataset.name)' } })}
            </td>`;
        tbody.appendChild(tr);
    });
}

function sqlConnApplyFilter() {
    const filterInput = document.getElementById('sqlconn-filter');
    const table = document.getElementById('sqlconn-table-wrap');
    if (!filterInput || !table) return;
    const q = (filterInput.value || '').toLowerCase();
    const filtered = sqlConnCache.filter(c => {
        const hay = [c.name, c.driver, c.host, c.database_name, c.description].join(' ').toLowerCase();
        return hay.includes(q);
    });
    sqlConnRenderRows(filtered);
    table.classList.toggle('is-hidden', filtered.length === 0);
    if (filtered.length === 0) cfgListState('sqlconn-state', { state: 'empty', title: t('config.sql_connections.empty') });
    else document.getElementById('sqlconn-state').replaceChildren();
}

function sqlConnDriverChanged() {
    const driver = document.getElementById('sqlconn-field-driver').value;
    const isSQLite = driver === 'sqlite';
    document.getElementById('sqlconn-host-row').classList.toggle('is-hidden', isSQLite);
    document.getElementById('sqlconn-creds-row').classList.toggle('is-hidden', isSQLite);
    const portInput = document.getElementById('sqlconn-field-port');
    portInput.closest('.field-group').classList.toggle('is-hidden', isSQLite);
    if (driver === 'postgres') portInput.placeholder = t('config.sql_connections.port_postgres');
    else if (driver === 'mysql') portInput.placeholder = t('config.sql_connections.port_mysql');
    else portInput.placeholder = '';
    const dbInput = document.getElementById('sqlconn-field-database');
    dbInput.readOnly = isSQLite;
    dbInput.closest('.field-group').classList.toggle('is-hidden', isSQLite);
    dbInput.placeholder = isSQLite ? t('config.sql_connections.sqlite_import_required') : t('config.sql_connections.database_placeholder');
    document.getElementById('sqlconn-import-row').classList.toggle('is-hidden', !isSQLite);
}

function sqlConnShowModal(id) {
    sqlConnEnsureDialog();
    sqlConnModalRevision++;
    document.getElementById('sqlconn-save-btn').disabled = false;
    document.getElementById('sqlconn-import-file').value = '';
    const title = document.getElementById(SQLCONN_DIALOG + '-title');
    cfgActionResult.show('sqlconn-test-result', '', '');
    cfgActionResult.show('sqlconn-save-result', '', '');

    if (id) {
        const c = sqlConnCache.find(x => x.id === id);
        if (!c) return;
        title.textContent = t('config.sql_connections.edit_connection');
        document.getElementById('sqlconn-edit-id').value = c.id;
        document.getElementById('sqlconn-field-name').value = c.name || '';
        document.getElementById('sqlconn-field-driver').value = c.driver || 'postgres';
        document.getElementById('sqlconn-field-host').value = c.host || '';
        document.getElementById('sqlconn-field-port').value = (c.port !== undefined && c.port !== null) ? c.port : '';
        document.getElementById('sqlconn-field-database').value = c.database_name || '';
        document.getElementById('sqlconn-field-desc').value = c.description || '';
        // An empty stored mode connects as "disable" (pool default); show that equivalent.
        document.getElementById('sqlconn-field-ssl').value = c.ssl_mode || 'disable';
        document.getElementById('sqlconn-field-username').value = '';
        document.getElementById('sqlconn-field-password').value = '';
        document.getElementById('sqlconn-field-password').placeholder = c.vault_secret_id ? t('config.providers.key_placeholder_existing') : t('config.sql_connections.password_placeholder');
        sqlConnPermSet('sqlconn-perm-read', c.allow_read);
        sqlConnPermSet('sqlconn-perm-write', c.allow_write);
        sqlConnPermSet('sqlconn-perm-change', c.allow_change);
        sqlConnPermSet('sqlconn-perm-delete', c.allow_delete);
    } else {
        title.textContent = t('config.sql_connections.new_connection');
        document.getElementById('sqlconn-edit-id').value = '';
        document.getElementById('sqlconn-field-name').value = '';
        document.getElementById('sqlconn-field-driver').value = 'postgres';
        document.getElementById('sqlconn-field-host').value = '';
        document.getElementById('sqlconn-field-port').value = '';
        document.getElementById('sqlconn-field-database').value = '';
        document.getElementById('sqlconn-field-desc').value = '';
        document.getElementById('sqlconn-field-ssl').value = 'require';
        document.getElementById('sqlconn-field-username').value = '';
        document.getElementById('sqlconn-field-password').value = '';
        document.getElementById('sqlconn-field-password').placeholder = t('config.sql_connections.password_placeholder');
        sqlConnPermSet('sqlconn-perm-read', true);
        sqlConnPermSet('sqlconn-perm-write', false);
        sqlConnPermSet('sqlconn-perm-change', false);
        sqlConnPermSet('sqlconn-perm-delete', false);
    }
    sqlConnDriverChanged();
    openModal(SQLCONN_DIALOG, { initialFocus: '#sqlconn-field-name', onClose: () => { sqlConnModalRevision++; } });
}

function sqlConnCloseModal() {
    closeModal(SQLCONN_DIALOG);
}

// Edited record fields as the save and test requests send them.
function sqlConnPayload() {
    const isSQLite = document.getElementById('sqlconn-field-driver').value === 'sqlite';
    return {
        name: document.getElementById('sqlconn-field-name').value.trim(),
        driver: document.getElementById('sqlconn-field-driver').value,
        host: document.getElementById('sqlconn-field-host').value.trim(),
        port: parseInt(document.getElementById('sqlconn-field-port').value) || 0,
        database_name: document.getElementById('sqlconn-field-database').value.trim(),
        description: document.getElementById('sqlconn-field-desc').value.trim(),
        // SQLite hides the TLS field; leave it to the server default.
        ssl_mode: isSQLite ? '' : document.getElementById('sqlconn-field-ssl').value,
        username: document.getElementById('sqlconn-field-username').value.trim(),
        password: document.getElementById('sqlconn-field-password').value,
        allow_read: sqlConnPermOn('sqlconn-perm-read'),
        allow_write: sqlConnPermOn('sqlconn-perm-write'),
        allow_change: sqlConnPermOn('sqlconn-perm-change'),
        allow_delete: sqlConnPermOn('sqlconn-perm-delete')
    };
}

async function sqlConnSave() {
    const revision = sqlConnModalRevision;
    const saveLine = 'sqlconn-save-result';
    cfgActionResult.show(saveLine, '', '');
    cfgActionResult.show('sqlconn-test-result', '', '');

    const payload = sqlConnPayload();
    if (!payload.name) {
        cfgActionResult.show(saveLine, 'error', t('config.sql_connections.name_required'));
        return { skipped: 'invalid' };
    }
    const isSQLite = payload.driver === 'sqlite';
    const importFile = document.getElementById('sqlconn-import-file').files[0];
    if (!payload.database_name && !isSQLite) {
        cfgActionResult.show(saveLine, 'error', t('config.sql_connections.database_required'));
        return { skipped: 'invalid' };
    }

    const editId = document.getElementById('sqlconn-edit-id').value;
    const isEdit = !!editId;
    const btn = document.getElementById('sqlconn-save-btn');

    return cfgActionResult(btn, async () => {
        const url = isEdit ? '/api/sql-connections/' + editId : '/api/sql-connections';
        const resp = await fetch(url, {
            method: isEdit ? 'PUT' : 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) return resp;
        const saved = await resp.json().catch(() => ({}));
        const savedId = editId || saved.id;
        if (revision === sqlConnModalRevision) document.getElementById('sqlconn-edit-id').value = savedId;
        if (isSQLite && importFile) {
            const imported = await fetch('/api/sql-connections/' + encodeURIComponent(savedId) + '/import', {
                method: 'POST', headers: { 'Content-Type': 'application/octet-stream' }, body: importFile
            });
            if (!imported.ok) return imported;
        }
        return { ok: true };
    }, {
        target: saveLine,
        success: t('config.common.saved'),
        failure: t('config.feedback.failed'),
        onSuccess: async () => {
            if (revision !== sqlConnModalRevision) return;
            sqlConnCloseModal();
            await sqlConnLoad();
            cfgActionResult.show('sqlconn-list-result', 'success', t('config.common.saved'));
        }
    });
}

// Tests the values typed into the dialog (draft); a connection that is not saved yet is tested as typed.
function sqlConnTest() {
    cfgActionResult.show('sqlconn-save-result', '', '');
    const editId = document.getElementById('sqlconn-edit-id').value;
    const btn = document.getElementById('sqlconn-test-btn');
    return cfgActionResult(btn, () => fetch('/api/sql-connections/' + encodeURIComponent(editId || 'new') + '/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(sqlConnPayload())
    }), {
        target: 'sqlconn-test-result',
        pending: t('config.sql_connections.testing'),
        success: t('config.sql_connections.test_success'),
        failure: t('config.feedback.failed')
    });
}

function sqlConnTestExisting(id) {
    const connection = sqlConnCache.find(x => x.id === id) || {};
    const btn = document.querySelector('#sqlconn-tbody tr[data-id="' + id + '"] .sql-action-btn');
    return cfgActionResult(btn, () => fetch('/api/sql-connections/' + encodeURIComponent(id) + '/test', { method: 'POST' }), {
        target: 'sqlconn-list-result',
        pending: (connection.name ? connection.name + ': ' : '') + t('config.sql_connections.testing'),
        success: (connection.name ? connection.name + ': ' : '') + t('config.sql_connections.test_success'),
        failure: (connection.name ? connection.name + ': ' : '') + t('config.feedback.failed')
    });
}

async function sqlConnDelete(id, name) {
    const answer = await showConfirmDialog({
        title: t('config.common.delete_item', { name: name }),
        message: t('config.sql_connections.delete_confirm', { name: name }),
        danger: true,
        confirmText: t('config.common.delete')
    });
    if (!answer.confirmed) return { skipped: 'cancelled' };

    const btn = document.querySelector('#sqlconn-tbody tr[data-id="' + id + '"] .cfg-delete-btn');
    return cfgActionResult(btn, () => fetch('/api/sql-connections/' + encodeURIComponent(id), { method: 'DELETE' }), {
        target: 'sqlconn-list-result',
        success: name + ': ' + t('config.feedback.done'),
        failure: name + ': ' + t('config.feedback.failed'),
        onSuccess: () => sqlConnLoad()
    });
}
