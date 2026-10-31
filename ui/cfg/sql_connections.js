let sqlConnCache = [];
let sqlConnModalRevision = 0;

window.addEventListener('cfg:section-leave', function () {
    const overlay = document.getElementById('sqlconn-modal-overlay');
    if (overlay && !overlay.classList.contains('is-hidden')) {
        sqlConnCloseModal();
    }
});

function renderSQLConnectionsSection(section) {
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    const sqlCfg = configData['sql_connections'] || {};
    const sqlEnabled = !!sqlCfg.enabled;
    const sqlReadonly = !!sqlCfg.readonly;
    const sqlAllowManagement = !!sqlCfg.allow_management;

    html += `<div class="sb-card">
        <div class="sb-card-header">
            <span class="sb-card-icon">⚙️</span>
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
            <input type="text" id="sqlconn-filter" class="field-input sqlconn-filter-input" placeholder="${t('config.sql_connections.filter_placeholder')}" oninput="sqlConnApplyFilter()">
        </div>
        <button class="btn-save sqlconn-add-btn" onclick="sqlConnShowModal()">
            ＋ ${t('config.sql_connections.add_connection')}
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
    <div id="sqlconn-empty" class="sql-empty-state is-hidden">
        ${t('config.sql_connections.empty')}
    </div>
    <div id="sqlconn-loading" class="sql-loading-state">
        ${t('config.sql_connections.loading')}
    </div>`;

    html += `
    <div id="sqlconn-modal-overlay" class="sql-modal-overlay is-hidden" onclick="sqlConnCloseModal(event)">
        <div class="sqlconn-modal" onclick="event.stopPropagation()">
            <div class="sqlconn-modal-header">
                <span id="sqlconn-modal-title" class="sql-modal-title"></span>
                <button onclick="sqlConnCloseModal()" class="sql-modal-close">✕</button>
            </div>
            <input type="hidden" id="sqlconn-edit-id">

            <div class="field-group sql-field-group">
                <div class="field-label">${t('config.sql_connections.name_label')} *</div>
                <input type="text" id="sqlconn-field-name" class="field-input" placeholder="${t('config.sql_connections.name_placeholder')}">
            </div>

            <div class="sqlconn-grid-two">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.driver_label')} *</div>
                    <select id="sqlconn-field-driver" class="field-input sql-select-compact" onchange="sqlConnDriverChanged()">
                        <option value="postgres">${t('config.sql_connections.driver_postgres')}</option>
                        <option value="mysql">${t('config.sql_connections.driver_mysql')}</option>
                        <option value="sqlite">${t('config.sql_connections.driver_sqlite')}</option>
                    </select>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.port_label')}</div>
                    <input type="number" id="sqlconn-field-port" class="field-input" placeholder="${t('config.sql_connections.port_postgres')}">
                </div>
            </div>

            <div class="sqlconn-grid-host" id="sqlconn-host-row">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.host_label')}</div>
                    <input type="text" id="sqlconn-field-host" class="field-input" placeholder="${t('config.sql_connections.host_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.ssl_mode_label')}</div>
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
                <input type="text" id="sqlconn-field-database" class="field-input" placeholder="${t('config.sql_connections.database_placeholder')}">
            </div>

            <div id="sqlconn-import-row" class="field-group is-hidden">
                <label for="sqlconn-import-file">${t('config.sql_connections.sqlite_import')}</label>
                <input type="file" id="sqlconn-import-file" class="field-input" accept=".db,.sqlite,.sqlite3,application/vnd.sqlite3">
                <div class="field-help">${t('config.sql_connections.sqlite_import_help')}</div>
            </div>

            <div class="field-group sql-field-group-mt">
                <div class="field-label">${t('config.sql_connections.description_label')}</div>
                <input type="text" id="sqlconn-field-desc" class="field-input" placeholder="${t('config.sql_connections.description_placeholder')}">
            </div>

            <div class="sqlconn-grid-two sqlconn-creds-row" id="sqlconn-creds-row">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.username_label')}</div>
                    <input type="text" id="sqlconn-field-username" class="field-input" placeholder="${t('config.sql_connections.username_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.password_label')}</div>
                    <input type="password" id="sqlconn-field-password" class="field-input" placeholder="${t('config.sql_connections.password_placeholder')}">
                </div>
            </div>

            <div class="sqlconn-permissions-card">
                <div class="field-label sql-perm-title">${t('config.sql_connections.permissions_title')}</div>
                <div class="sqlconn-permissions-grid">
                    <label class="sqlconn-permission-item">
                        <input type="checkbox" id="sqlconn-perm-read" checked> ${t('config.sql_connections.perm_read')}
                    </label>
                    <label class="sqlconn-permission-item">
                        <input type="checkbox" id="sqlconn-perm-write"> ${t('config.sql_connections.perm_write')}
                    </label>
                    <label class="sqlconn-permission-item">
                        <input type="checkbox" id="sqlconn-perm-change"> ${t('config.sql_connections.perm_change')}
                    </label>
                    <label class="sqlconn-permission-item">
                        <input type="checkbox" id="sqlconn-perm-delete" data-cfg-confirm-enable="delete"> ${t('config.sql_connections.perm_delete')}
                    </label>
                </div>
                <div class="sql-perm-hint">${t('config.sql_connections.permissions_hint')}</div>
            </div>

            ${cfgActionResult.html('sqlconn-test-result')}
            ${cfgActionResult.html('sqlconn-save-result')}

            <div class="sqlconn-modal-actions">
                <button class="btn-save sqlconn-test-btn" id="sqlconn-test-btn" data-config-action-draft="true" onclick="sqlConnTest()">
                    🔌 ${t('config.sql_connections.test_connection')}
                </button>
                <div class="sqlconn-modal-actions-right">
                    <button class="btn-save sqlconn-secondary-btn" onclick="sqlConnCloseModal()">${t('config.sql_connections.cancel')}</button>
                    <button class="btn-save sqlconn-primary-btn" id="sqlconn-modal-save" onclick="sqlConnSave()">${t('config.sql_connections.save')}</button>
                </div>
            </div>
        </div>
    </div>`;

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    sqlConnLoad();
}

async function sqlConnLoad() {
    const tbody = document.getElementById('sqlconn-tbody');
    const empty = document.getElementById('sqlconn-empty');
    const loading = document.getElementById('sqlconn-loading');
    const table = document.getElementById('sqlconn-table-wrap');
    loading.classList.remove('is-hidden');
    table.classList.add('is-hidden');
    empty.classList.add('is-hidden');

    try {
        const resp = await fetch('/api/sql-connections');
        if (!resp.ok) throw new Error(await resp.text());
        sqlConnCache = await resp.json();
        if (!Array.isArray(sqlConnCache)) sqlConnCache = [];
    } catch (e) {
        loading.textContent = '❌ ' + e.message;
        return;
    }

    loading.classList.add('is-hidden');
    if (sqlConnCache.length === 0) {
        empty.classList.remove('is-hidden');
        return;
    }
    table.classList.remove('is-hidden');
    sqlConnRenderRows(sqlConnCache);
    sqlConnApplyFilter();
}

function sqlConnRenderRows(connections) {
    const tbody = document.getElementById('sqlconn-tbody');
    tbody.innerHTML = '';
    const permClasses = { R: 'sql-perm-read', W: 'sql-perm-write', C: 'sql-perm-change', D: 'sql-perm-delete' };
    connections.forEach(c => {
        const perms = [];
        if (c.allow_read) perms.push('R');
        if (c.allow_write) perms.push('W');
        if (c.allow_change) perms.push('C');
        if (c.allow_delete) perms.push('D');
        const permBadges = perms.map(p => {
            const labels = { R: t('config.sql_connections.badge_read'), W: t('config.sql_connections.badge_write'), C: t('config.sql_connections.badge_change'), D: t('config.sql_connections.badge_delete') };
            return '<span class="sql-perm-badge ' + permClasses[p] + '">' + labels[p] + '</span>';
        }).join('');

        const driverIcon = { postgres: '🐘', mysql: '🐬', sqlite: '📄' }[c.driver] || '🗄️';
        const hostDisplay = c.driver === 'sqlite' ? '—' : (c.host || 'localhost') + (c.port ? ':' + c.port : '');
        const tr = document.createElement('tr');
        tr.classList.add('sql-table-row');
        tr.dataset.id = c.id;
        tr.innerHTML = `
            <td class="sql-td-name">${escapeHtml(c.name)}</td>
            <td class="sql-td"><span class="sql-driver-badge">${driverIcon} ${escapeHtml(c.driver)}</span></td>
            <td class="sql-td-host">${escapeHtml(hostDisplay)}</td>
            <td class="sql-td">${escapeHtml(c.driver === 'sqlite' ? t(c.state === 'sqlite_import_required' ? 'config.sql_connections.sqlite_import_required' : 'config.sql_connections.driver_sqlite') : (c.database_name || '—'))}</td>
            <td class="sql-td-desc" title="${escapeHtml(c.description || '')}">${escapeHtml(c.description || '—')}</td>
            <td class="sql-td">${permBadges || '—'}</td>
            <td class="sql-td-actions">
                <button onclick="sqlConnTestExisting('${c.id}')" class="sql-action-btn" title="${t('config.sql_connections.test_tooltip')}">🔌</button>
                <button onclick="sqlConnShowModal('${c.id}')" class="sql-action-btn" title="${t('config.sql_connections.edit_tooltip')}">✏️</button>
                <button onclick="sqlConnDelete('${c.id}','${escapeHtml(c.name).replace(/'/g, "\\'")}')" class="sql-action-btn sql-action-btn-delete" title="${t('config.sql_connections.delete_tooltip')}">🗑️</button>
            </td>`;
        tbody.appendChild(tr);
    });
}

function sqlConnApplyFilter() {
    const q = (document.getElementById('sqlconn-filter').value || '').toLowerCase();
    const filtered = sqlConnCache.filter(c => {
        const hay = [c.name, c.driver, c.host, c.database_name, c.description].join(' ').toLowerCase();
        return hay.includes(q);
    });
    sqlConnRenderRows(filtered);
    document.getElementById('sqlconn-empty').classList.toggle('is-hidden', filtered.length > 0);
    document.getElementById('sqlconn-table-wrap').classList.toggle('is-hidden', filtered.length === 0);
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
    sqlConnModalRevision++;
    document.getElementById('sqlconn-modal-save').disabled = false;
    document.getElementById('sqlconn-import-file').value = '';
    const overlay = document.getElementById('sqlconn-modal-overlay');
    const title = document.getElementById('sqlconn-modal-title');
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
        document.getElementById('sqlconn-perm-read').checked = !!c.allow_read;
        document.getElementById('sqlconn-perm-write').checked = !!c.allow_write;
        document.getElementById('sqlconn-perm-change').checked = !!c.allow_change;
        document.getElementById('sqlconn-perm-delete').checked = !!c.allow_delete;
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
        document.getElementById('sqlconn-perm-read').checked = true;
        document.getElementById('sqlconn-perm-write').checked = false;
        document.getElementById('sqlconn-perm-change').checked = false;
        document.getElementById('sqlconn-perm-delete').checked = false;
    }
    sqlConnDriverChanged();
    overlay.classList.remove('is-hidden');
}

function sqlConnCloseModal(e) {
    if (e && e.target !== e.currentTarget) return;
    sqlConnModalRevision++;
    document.getElementById('sqlconn-modal-overlay').classList.add('is-hidden');
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
        allow_read: document.getElementById('sqlconn-perm-read').checked,
        allow_write: document.getElementById('sqlconn-perm-write').checked,
        allow_change: document.getElementById('sqlconn-perm-change').checked,
        allow_delete: document.getElementById('sqlconn-perm-delete').checked
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
    const btn = document.getElementById('sqlconn-modal-save');

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

// Tests the values typed into the modal (draft); a connection that is not saved yet is tested as typed.
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
    const msg = t('config.sql_connections.delete_confirm', { name: name });
    if (!await showConfirm(msg)) return { skipped: 'cancelled' };

    const btn = document.querySelector('#sqlconn-tbody tr[data-id="' + id + '"] .sql-action-btn-delete');
    return cfgActionResult(btn, () => fetch('/api/sql-connections/' + encodeURIComponent(id), { method: 'DELETE' }), {
        target: 'sqlconn-list-result',
        success: name + ': ' + t('config.feedback.done'),
        failure: name + ': ' + t('config.feedback.failed'),
        onSuccess: () => sqlConnLoad()
    });
}
