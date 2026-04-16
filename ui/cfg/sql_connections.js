let sqlConnCache = [];

window.addEventListener('cfg:section-leave', function () {
    const overlay = document.getElementById('sqlconn-modal-overlay');
    if (overlay && !overlay.classList.contains('is-hidden')) {
        sqlConnCloseModal();
    }
});

function renderSQLConnectionsSection(section) {
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    const sqlCfg = configData['sql_connections'] || {};
    const sqlEnabled = !!sqlCfg.enabled;
    const sqlReadonly = !!sqlCfg.readonly;
    const sqlAllowManagement = !!sqlCfg.allow_management;

    html += `<div class="cfg-group-title cfg-group-title-top">${t('config.sql_connections.settings_title')}</div>`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.sql_connections.enabled_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${sqlEnabled ? ' on' : ''}" data-path="sql_connections.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${sqlEnabled ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
        <div class="field-help">${t('config.sql_connections.enabled_help')}</div>
    </div>`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.sql_connections.readonly_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${sqlReadonly ? ' on' : ''}" data-path="sql_connections.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${sqlReadonly ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
        <div class="field-help">${t('config.sql_connections.readonly_help')}</div>
    </div>`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.sql_connections.allow_management_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${sqlAllowManagement ? ' on' : ''}" data-path="sql_connections.allow_management" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${sqlAllowManagement ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
        <div class="field-help">${t('config.sql_connections.allow_management_help')}</div>
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
                        <option value="">${t('config.sql_connections.ssl_default')}</option>
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
                        <input type="checkbox" id="sqlconn-perm-delete"> ${t('config.sql_connections.perm_delete')}
                    </label>
                </div>
                <div class="sql-perm-hint">${t('config.sql_connections.permissions_hint')}</div>
            </div>

            <div id="sqlconn-modal-error" class="sql-modal-error is-hidden"></div>
            <div id="sqlconn-modal-success" class="sql-modal-success is-hidden"></div>

            <div class="sqlconn-modal-actions">
                <button class="btn-save sqlconn-test-btn" id="sqlconn-test-btn" onclick="sqlConnTest()">
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
        const hostDisplay = c.driver === 'sqlite' ? (c.database_name || '—') : (c.host || 'localhost') + (c.port ? ':' + c.port : '');
        const tr = document.createElement('tr');
        tr.classList.add('sql-table-row');
        tr.dataset.id = c.id;
        tr.innerHTML = `
            <td class="sql-td-name">${escapeHtml(c.name)}</td>
            <td class="sql-td"><span class="sql-driver-badge">${driverIcon} ${escapeHtml(c.driver)}</span></td>
            <td class="sql-td-host">${escapeHtml(hostDisplay)}</td>
            <td class="sql-td">${escapeHtml(c.database_name || '—')}</td>
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
    if (driver === 'postgres') portInput.placeholder = t('config.sql_connections.port_postgres');
    else if (driver === 'mysql') portInput.placeholder = t('config.sql_connections.port_mysql');
    else portInput.placeholder = '';
    const dbInput = document.getElementById('sqlconn-field-database');
    dbInput.placeholder = isSQLite ? t('config.sql_connections.sqlite_path_placeholder') : t('config.sql_connections.database_placeholder');
}

function sqlConnShowModal(id) {
    const overlay = document.getElementById('sqlconn-modal-overlay');
    const title = document.getElementById('sqlconn-modal-title');
    document.getElementById('sqlconn-modal-error').classList.add('is-hidden');
    document.getElementById('sqlconn-modal-success').classList.add('is-hidden');

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
        document.getElementById('sqlconn-field-ssl').value = c.ssl_mode || '';
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
        document.getElementById('sqlconn-field-ssl').value = '';
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
    document.getElementById('sqlconn-modal-overlay').classList.add('is-hidden');
}

async function sqlConnSave() {
    const errBox = document.getElementById('sqlconn-modal-error');
    const successBox = document.getElementById('sqlconn-modal-success');
    errBox.classList.add('is-hidden');
    successBox.classList.add('is-hidden');

    const name = document.getElementById('sqlconn-field-name').value.trim();
    const database = document.getElementById('sqlconn-field-database').value.trim();
    if (!name) {
        errBox.textContent = t('config.sql_connections.name_required');
        errBox.classList.remove('is-hidden');
        return;
    }
    if (!database) {
        errBox.textContent = t('config.sql_connections.database_required');
        errBox.classList.remove('is-hidden');
        return;
    }

    const payload = {
        name: name,
        driver: document.getElementById('sqlconn-field-driver').value,
        host: document.getElementById('sqlconn-field-host').value.trim(),
        port: parseInt(document.getElementById('sqlconn-field-port').value) || 0,
        database_name: database,
        description: document.getElementById('sqlconn-field-desc').value.trim(),
        ssl_mode: document.getElementById('sqlconn-field-ssl').value,
        username: document.getElementById('sqlconn-field-username').value.trim(),
        password: document.getElementById('sqlconn-field-password').value,
        allow_read: document.getElementById('sqlconn-perm-read').checked,
        allow_write: document.getElementById('sqlconn-perm-write').checked,
        allow_change: document.getElementById('sqlconn-perm-change').checked,
        allow_delete: document.getElementById('sqlconn-perm-delete').checked
    };

    const editId = document.getElementById('sqlconn-edit-id').value;
    const isEdit = !!editId;
    const btn = document.getElementById('sqlconn-modal-save');
    btn.disabled = true;

    try {
        const url = isEdit ? '/api/sql-connections/' + editId : '/api/sql-connections';
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
        sqlConnCloseModal();
        await sqlConnLoad();
    } catch (e) {
        errBox.textContent = '❌ ' + e.message;
        errBox.classList.remove('is-hidden');
    } finally {
        btn.disabled = false;
    }
}

async function sqlConnTest() {
    const errBox = document.getElementById('sqlconn-modal-error');
    const successBox = document.getElementById('sqlconn-modal-success');
    errBox.classList.add('is-hidden');
    successBox.classList.add('is-hidden');

    const btn = document.getElementById('sqlconn-test-btn');
    btn.disabled = true;
    btn.textContent = '⏳ ' + t('config.sql_connections.testing');

    const editId = document.getElementById('sqlconn-edit-id').value;

    try {
        if (editId) {
            const resp = await fetch('/api/sql-connections/' + editId + '/test', { method: 'POST' });
            const data = await resp.json().catch(() => ({}));
            // Check both HTTP status and response status field
            if (!resp.ok || data.status === 'error') {
                throw new Error(data.message || (resp.ok ? 'Test failed' : 'Test failed with status ' + resp.status));
            }
            successBox.textContent = '✅ ' + (data.message || t('config.sql_connections.test_success'));
        } else {
            errBox.textContent = t('config.sql_connections.save_before_test');
            errBox.classList.remove('is-hidden');
            return;
        }
        successBox.classList.remove('is-hidden');
    } catch (e) {
        errBox.textContent = '❌ ' + e.message;
        errBox.classList.remove('is-hidden');
    } finally {
        btn.disabled = false;
        btn.textContent = '🔌 ' + t('config.sql_connections.test_connection');
    }
}

async function sqlConnTestExisting(id) {
    try {
        const resp = await fetch('/api/sql-connections/' + id + '/test', { method: 'POST' });
        const data = await resp.json().catch(() => ({}));
        // Check both HTTP status and response status field
        if (!resp.ok || data.status === 'error') {
            throw new Error(data.message || (resp.ok ? 'Test failed' : 'Test failed with status ' + resp.status));
        }
        showToast(data.message || t('config.sql_connections.test_success'), 'success');
    } catch (e) {
        showToast(e.message || t('config.common.error'), 'error');
    }
}

async function sqlConnDelete(id, name) {
    const msg = t('config.sql_connections.delete_confirm', { name: name });
    if (!await showConfirm(msg)) return;

    try {
        const resp = await fetch('/api/sql-connections/' + id, { method: 'DELETE' });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Delete failed');
        }
        await sqlConnLoad();
    } catch (e) {
        showToast(e.message || t('config.common.error'), 'error');
    }
}
