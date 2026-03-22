// cfg/sql_connections.js — SQL Connections section module
let sqlConnCache = [];

function renderSQLConnectionsSection(section) {
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += `
    <div style="display:flex;justify-content:space-between;align-items:center;margin-top:1.5rem;margin-bottom:1rem;">
        <div style="display:flex;gap:0.5rem;align-items:center;">
            <input type="text" id="sqlconn-filter" class="field-input" placeholder="${t('config.sql_connections.filter_placeholder')}" style="width:220px;padding:0.4rem 0.7rem;font-size:0.82rem;" oninput="sqlConnApplyFilter()">
        </div>
        <button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;" onclick="sqlConnShowModal()">
            ＋ ${t('config.sql_connections.add_connection')}
        </button>
    </div>
    <div id="sqlconn-table-wrap" style="overflow-x:auto;">
        <table style="width:100%;border-collapse:collapse;font-size:0.82rem;">
            <thead>
                <tr style="border-bottom:2px solid var(--border-subtle);text-align:left;">
                    <th style="padding:0.5rem 0.6rem;">${t('config.sql_connections.col_name')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.sql_connections.col_driver')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.sql_connections.col_host')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.sql_connections.col_database')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.sql_connections.col_description')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.sql_connections.col_permissions')}</th>
                    <th style="padding:0.5rem 0.6rem;text-align:right;">${t('config.sql_connections.col_actions')}</th>
                </tr>
            </thead>
            <tbody id="sqlconn-tbody"></tbody>
        </table>
    </div>
    <div id="sqlconn-empty" style="display:none;text-align:center;padding:2rem;color:var(--text-tertiary);font-size:0.85rem;">
        ${t('config.sql_connections.empty')}
    </div>
    <div id="sqlconn-loading" style="text-align:center;padding:2rem;color:var(--text-secondary);font-size:0.85rem;">
        ${t('config.sql_connections.loading')}
    </div>`;

    // Modal overlay
    html += `
    <div id="sqlconn-modal-overlay" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,0.55);z-index:1000;backdrop-filter:blur(4px);" onclick="sqlConnCloseModal(event)">
        <div style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);background:var(--bg-primary);border:1px solid var(--border-subtle);border-radius:14px;padding:1.5rem;width:min(560px,90vw);max-height:85vh;overflow-y:auto;" onclick="event.stopPropagation()">
            <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1.2rem;">
                <span id="sqlconn-modal-title" style="font-size:1rem;font-weight:600;"></span>
                <button onclick="sqlConnCloseModal()" style="background:none;border:none;color:var(--text-secondary);font-size:1.2rem;cursor:pointer;">✕</button>
            </div>
            <input type="hidden" id="sqlconn-edit-id">

            <div class="field-group" style="margin-bottom:0.8rem;">
                <div class="field-label">${t('config.sql_connections.name_label')} *</div>
                <input type="text" id="sqlconn-field-name" class="field-input" placeholder="${t('config.sql_connections.name_placeholder')}">
            </div>

            <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem;">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.driver_label')} *</div>
                    <select id="sqlconn-field-driver" class="field-input" style="padding:0.45rem 0.6rem;" onchange="sqlConnDriverChanged()">
                        <option value="postgres">PostgreSQL</option>
                        <option value="mysql">MySQL / MariaDB</option>
                        <option value="sqlite">SQLite</option>
                    </select>
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.port_label')}</div>
                    <input type="number" id="sqlconn-field-port" class="field-input" placeholder="5432">
                </div>
            </div>

            <div style="display:grid;grid-template-columns:2fr 1fr;gap:0.8rem;margin-top:0.8rem;" id="sqlconn-host-row">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.host_label')}</div>
                    <input type="text" id="sqlconn-field-host" class="field-input" placeholder="localhost">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.ssl_mode_label')}</div>
                    <select id="sqlconn-field-ssl" class="field-input" style="padding:0.45rem 0.6rem;">
                        <option value="">Default</option>
                        <option value="disable">Disable</option>
                        <option value="require">Require</option>
                        <option value="verify-ca">Verify CA</option>
                        <option value="verify-full">Verify Full</option>
                    </select>
                </div>
            </div>

            <div class="field-group" style="margin-top:0.8rem;">
                <div class="field-label">${t('config.sql_connections.database_label')} *</div>
                <input type="text" id="sqlconn-field-database" class="field-input" placeholder="${t('config.sql_connections.database_placeholder')}">
            </div>

            <div class="field-group" style="margin-top:0.8rem;">
                <div class="field-label">${t('config.sql_connections.description_label')}</div>
                <input type="text" id="sqlconn-field-desc" class="field-input" placeholder="${t('config.sql_connections.description_placeholder')}">
            </div>

            <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem;margin-top:0.8rem;" id="sqlconn-creds-row">
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.username_label')}</div>
                    <input type="text" id="sqlconn-field-username" class="field-input" placeholder="${t('config.sql_connections.username_placeholder')}">
                </div>
                <div class="field-group">
                    <div class="field-label">${t('config.sql_connections.password_label')}</div>
                    <input type="password" id="sqlconn-field-password" class="field-input" placeholder="${t('config.sql_connections.password_placeholder')}">
                </div>
            </div>

            <div style="margin-top:1rem;padding:0.8rem;background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:10px;">
                <div class="field-label" style="margin-bottom:0.5rem;font-weight:600;">${t('config.sql_connections.permissions_title')}</div>
                <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.5rem;">
                    <label style="display:flex;align-items:center;gap:0.4rem;font-size:0.82rem;cursor:pointer;">
                        <input type="checkbox" id="sqlconn-perm-read" checked> ${t('config.sql_connections.perm_read')}
                    </label>
                    <label style="display:flex;align-items:center;gap:0.4rem;font-size:0.82rem;cursor:pointer;">
                        <input type="checkbox" id="sqlconn-perm-write"> ${t('config.sql_connections.perm_write')}
                    </label>
                    <label style="display:flex;align-items:center;gap:0.4rem;font-size:0.82rem;cursor:pointer;">
                        <input type="checkbox" id="sqlconn-perm-change"> ${t('config.sql_connections.perm_change')}
                    </label>
                    <label style="display:flex;align-items:center;gap:0.4rem;font-size:0.82rem;cursor:pointer;">
                        <input type="checkbox" id="sqlconn-perm-delete"> ${t('config.sql_connections.perm_delete')}
                    </label>
                </div>
                <div style="margin-top:0.4rem;font-size:0.75rem;color:var(--text-tertiary);">${t('config.sql_connections.permissions_hint')}</div>
            </div>

            <div id="sqlconn-modal-error" style="display:none;margin-top:0.7rem;padding:0.5rem 0.8rem;background:rgba(239,68,68,0.1);border:1px solid rgba(239,68,68,0.3);border-radius:8px;font-size:0.8rem;color:var(--danger);"></div>
            <div id="sqlconn-modal-success" style="display:none;margin-top:0.7rem;padding:0.5rem 0.8rem;background:rgba(34,197,94,0.1);border:1px solid rgba(34,197,94,0.3);border-radius:8px;font-size:0.8rem;color:var(--success);"></div>

            <div style="display:flex;justify-content:space-between;align-items:center;margin-top:1.2rem;">
                <button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;background:var(--bg-secondary);color:var(--accent);border:1px solid var(--accent);box-shadow:none;" id="sqlconn-test-btn" onclick="sqlConnTest()">
                    🔌 ${t('config.sql_connections.test_connection')}
                </button>
                <div style="display:flex;gap:0.6rem;">
                    <button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;background:var(--bg-secondary);color:var(--text-primary);box-shadow:none;" onclick="sqlConnCloseModal()">${t('config.sql_connections.cancel')}</button>
                    <button class="btn-save" style="padding:0.45rem 1.2rem;font-size:0.82rem;" id="sqlconn-modal-save" onclick="sqlConnSave()">${t('config.sql_connections.save')}</button>
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
    loading.style.display = 'block';
    table.style.display = 'none';
    empty.style.display = 'none';

    try {
        const resp = await fetch('/api/sql-connections');
        if (!resp.ok) throw new Error(await resp.text());
        sqlConnCache = await resp.json();
        if (!Array.isArray(sqlConnCache)) sqlConnCache = [];
    } catch (e) {
        loading.textContent = '❌ ' + e.message;
        return;
    }

    loading.style.display = 'none';
    if (sqlConnCache.length === 0) {
        empty.style.display = 'block';
        return;
    }
    table.style.display = 'block';
    sqlConnRenderRows(sqlConnCache);
}

function sqlConnRenderRows(connections) {
    const tbody = document.getElementById('sqlconn-tbody');
    tbody.innerHTML = '';
    connections.forEach(c => {
        const perms = [];
        if (c.allow_read) perms.push('R');
        if (c.allow_write) perms.push('W');
        if (c.allow_change) perms.push('C');
        if (c.allow_delete) perms.push('D');
        const permBadges = perms.map(p => {
            const colors = { R: '#3b82f6', W: '#22c55e', C: '#f59e0b', D: '#ef4444' };
            const labels = { R: t('config.sql_connections.badge_read'), W: t('config.sql_connections.badge_write'), C: t('config.sql_connections.badge_change'), D: t('config.sql_connections.badge_delete') };
            return '<span style="display:inline-block;background:' + colors[p] + '22;color:' + colors[p] + ';border:1px solid ' + colors[p] + '44;border-radius:6px;padding:0.1rem 0.4rem;font-size:0.7rem;font-weight:600;margin-right:0.2rem;">' + labels[p] + '</span>';
        }).join('');

        const driverIcon = { postgres: '🐘', mysql: '🐬', sqlite: '📄' }[c.driver] || '🗄️';
        const hostDisplay = c.driver === 'sqlite' ? (c.database_name || '—') : (c.host || 'localhost') + (c.port ? ':' + c.port : '');
        const tr = document.createElement('tr');
        tr.style.borderBottom = '1px solid var(--border-subtle)';
        tr.dataset.id = c.id;
        tr.innerHTML = `
            <td style="padding:0.5rem 0.6rem;font-weight:500;">${escapeHtml(c.name)}</td>
            <td style="padding:0.5rem 0.6rem;"><span style="background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:6px;padding:0.15rem 0.5rem;font-size:0.75rem;">${driverIcon} ${escapeHtml(c.driver)}</span></td>
            <td style="padding:0.5rem 0.6rem;font-family:monospace;font-size:0.8rem;">${escapeHtml(hostDisplay)}</td>
            <td style="padding:0.5rem 0.6rem;">${escapeHtml(c.database_name || '—')}</td>
            <td style="padding:0.5rem 0.6rem;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${escapeHtml(c.description || '')}">${escapeHtml(c.description || '—')}</td>
            <td style="padding:0.5rem 0.6rem;">${permBadges || '—'}</td>
            <td style="padding:0.5rem 0.6rem;text-align:right;white-space:nowrap;">
                <button onclick="sqlConnTestExisting(${c.id})" style="background:none;border:none;cursor:pointer;font-size:0.9rem;" title="${t('config.sql_connections.test_tooltip')}">🔌</button>
                <button onclick="sqlConnShowModal(${c.id})" style="background:none;border:none;cursor:pointer;font-size:0.9rem;" title="${t('config.sql_connections.edit_tooltip')}">✏️</button>
                <button onclick="sqlConnDelete(${c.id},'${escapeHtml(c.name).replace(/'/g, "\\'")}')" style="background:none;border:none;cursor:pointer;font-size:0.9rem;margin-left:0.3rem;" title="${t('config.sql_connections.delete_tooltip')}">🗑️</button>
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
    document.getElementById('sqlconn-empty').style.display = filtered.length === 0 ? 'block' : 'none';
    document.getElementById('sqlconn-table-wrap').style.display = filtered.length > 0 ? 'block' : 'none';
}

function sqlConnDriverChanged() {
    const driver = document.getElementById('sqlconn-field-driver').value;
    const isSQLite = driver === 'sqlite';
    document.getElementById('sqlconn-host-row').style.display = isSQLite ? 'none' : '';
    document.getElementById('sqlconn-creds-row').style.display = isSQLite ? 'none' : '';
    const portInput = document.getElementById('sqlconn-field-port');
    if (driver === 'postgres') portInput.placeholder = '5432';
    else if (driver === 'mysql') portInput.placeholder = '3306';
    else portInput.placeholder = '';
    const dbInput = document.getElementById('sqlconn-field-database');
    dbInput.placeholder = isSQLite ? t('config.sql_connections.sqlite_path_placeholder') : t('config.sql_connections.database_placeholder');
}

function sqlConnShowModal(id) {
    const overlay = document.getElementById('sqlconn-modal-overlay');
    const title = document.getElementById('sqlconn-modal-title');
    document.getElementById('sqlconn-modal-error').style.display = 'none';
    document.getElementById('sqlconn-modal-success').style.display = 'none';

    if (id) {
        const c = sqlConnCache.find(x => x.id === id);
        if (!c) return;
        title.textContent = t('config.sql_connections.edit_connection');
        document.getElementById('sqlconn-edit-id').value = c.id;
        document.getElementById('sqlconn-field-name').value = c.name || '';
        document.getElementById('sqlconn-field-driver').value = c.driver || 'postgres';
        document.getElementById('sqlconn-field-host').value = c.host || '';
        document.getElementById('sqlconn-field-port').value = c.port || '';
        document.getElementById('sqlconn-field-database').value = c.database_name || '';
        document.getElementById('sqlconn-field-desc').value = c.description || '';
        document.getElementById('sqlconn-field-ssl').value = c.ssl_mode || '';
        document.getElementById('sqlconn-field-username').value = '';
        document.getElementById('sqlconn-field-password').value = '';
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
        document.getElementById('sqlconn-perm-read').checked = true;
        document.getElementById('sqlconn-perm-write').checked = false;
        document.getElementById('sqlconn-perm-change').checked = false;
        document.getElementById('sqlconn-perm-delete').checked = false;
    }
    sqlConnDriverChanged();
    overlay.style.display = 'block';
}

function sqlConnCloseModal(e) {
    if (e && e.target !== e.currentTarget) return;
    document.getElementById('sqlconn-modal-overlay').style.display = 'none';
}

async function sqlConnSave() {
    const errBox = document.getElementById('sqlconn-modal-error');
    const successBox = document.getElementById('sqlconn-modal-success');
    errBox.style.display = 'none';
    successBox.style.display = 'none';

    const name = document.getElementById('sqlconn-field-name').value.trim();
    const database = document.getElementById('sqlconn-field-database').value.trim();
    if (!name) {
        errBox.textContent = t('config.sql_connections.name_required');
        errBox.style.display = 'block';
        return;
    }
    if (!database) {
        errBox.textContent = t('config.sql_connections.database_required');
        errBox.style.display = 'block';
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
        errBox.style.display = 'block';
    } finally {
        btn.disabled = false;
    }
}

async function sqlConnTest() {
    const errBox = document.getElementById('sqlconn-modal-error');
    const successBox = document.getElementById('sqlconn-modal-success');
    errBox.style.display = 'none';
    successBox.style.display = 'none';

    const btn = document.getElementById('sqlconn-test-btn');
    btn.disabled = true;
    btn.textContent = '⏳ ' + t('config.sql_connections.testing');

    const editId = document.getElementById('sqlconn-edit-id').value;

    try {
        if (editId) {
            // Test existing connection via ID
            const resp = await fetch('/api/sql-connections/' + editId + '/test', { method: 'POST' });
            const data = await resp.json().catch(() => ({}));
            if (!resp.ok) throw new Error(data.error || 'Test failed');
            successBox.textContent = '✅ ' + (data.message || t('config.sql_connections.test_success'));
        } else {
            // For new connections, save first then test
            errBox.textContent = t('config.sql_connections.save_before_test');
            errBox.style.display = 'block';
            return;
        }
        successBox.style.display = 'block';
    } catch (e) {
        errBox.textContent = '❌ ' + e.message;
        errBox.style.display = 'block';
    } finally {
        btn.disabled = false;
        btn.textContent = '🔌 ' + t('config.sql_connections.test_connection');
    }
}

async function sqlConnTestExisting(id) {
    try {
        const resp = await fetch('/api/sql-connections/' + id + '/test', { method: 'POST' });
        const data = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(data.error || 'Test failed');

        const toast = document.createElement('div');
        toast.style.cssText = 'position:fixed;top:1rem;right:1rem;z-index:9999;background:var(--surface-elevated);border:1px solid rgba(34,197,94,0.35);border-radius:10px;padding:0.75rem 1.25rem;font-size:0.85rem;color:#4ade80;box-shadow:0 8px 30px rgba(0,0,0,0.3);max-width:420px;';
        toast.textContent = '✅ ' + (data.message || t('config.sql_connections.test_success'));
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 4000);
    } catch (e) {
        const toast = document.createElement('div');
        toast.style.cssText = 'position:fixed;top:1rem;right:1rem;z-index:9999;background:var(--surface-elevated);border:1px solid rgba(239,68,68,0.35);border-radius:10px;padding:0.75rem 1.25rem;font-size:0.85rem;color:#fca5a5;box-shadow:0 8px 30px rgba(0,0,0,0.3);max-width:420px;';
        toast.textContent = '❌ ' + e.message;
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 4000);
    }
}

async function sqlConnDelete(id, name) {
    const msg = t('config.sql_connections.delete_confirm', { name: name });
    if (!confirm(msg)) return;

    try {
        const resp = await fetch('/api/sql-connections/' + id, { method: 'DELETE' });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Delete failed');
        }
        await sqlConnLoad();
    } catch (e) {
        alert('❌ ' + e.message);
    }
}
