// cfg/skill_manager.js — Skill Manager configuration section module

let ptbCatalogCache = null;

async function renderSkillManagerSection(section) {
    const data = configData['skill_manager'] || {};
    const enabledOn = data.enabled === true;
    const allowUploadsOn = data.allow_uploads === true;
    const readonlyOn = data.read_only === true;
    const requireScanOn = data.require_scan !== false;
    const requireSandboxOn = data.require_sandbox === true;
    const autoEnableOn = data.auto_enable_clean === true;
    const guardianOn = data.scan_with_guardian === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    const helpEnabled = t('help.skill_manager.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="skill_manager.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Allow Uploads toggle ──
    const helpUploads = t('help.skill_manager.allow_uploads');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.allow_uploads_label') + '</div>';
    if (helpUploads) html += '<div class="field-help">' + helpUploads + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (allowUploadsOn ? ' on' : '') + '" data-path="skill_manager.allow_uploads" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (allowUploadsOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Read-only toggle ──
    const helpReadonly = t('help.skill_manager.read_only');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.read_only_label') + '</div>';
    if (helpReadonly) html += '<div class="field-help">' + helpReadonly + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (readonlyOn ? ' on' : '') + '" data-path="skill_manager.read_only" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Require Scan toggle ──
    const helpScan = t('help.skill_manager.require_scan');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.require_scan_label') + '</div>';
    if (helpScan) html += '<div class="field-help">' + helpScan + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (requireScanOn ? ' on' : '') + '" data-path="skill_manager.require_scan" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (requireScanOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Require Sandbox toggle ──
    const helpSandbox = t('help.skill_manager.require_sandbox');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.require_sandbox_label') + '</div>';
    if (helpSandbox) html += '<div class="field-help">' + helpSandbox + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (requireSandboxOn ? ' on' : '') + '" data-path="skill_manager.require_sandbox" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (requireSandboxOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Auto-enable Clean toggle ──
    const helpAutoEnable = t('help.skill_manager.auto_enable_clean');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.auto_enable_clean_label') + '</div>';
    if (helpAutoEnable) html += '<div class="field-help">' + helpAutoEnable + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (autoEnableOn ? ' on' : '') + '" data-path="skill_manager.auto_enable_clean" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (autoEnableOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Scan with Guardian toggle ──
    const helpGuardian = t('help.skill_manager.scan_with_guardian');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.scan_with_guardian_label') + '</div>';
    if (helpGuardian) html += '<div class="field-help">' + helpGuardian + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (guardianOn ? ' on' : '') + '" data-path="skill_manager.scan_with_guardian" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (guardianOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── Max Upload Size ──
    const helpMaxSize = t('help.skill_manager.max_upload_size_mb');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.max_upload_size_mb_label') + '</div>';
    if (helpMaxSize) html += '<div class="field-help">' + helpMaxSize + '</div>';
    html += '<input class="field-input" type="number" min="1" max="50" data-path="skill_manager.max_upload_size_mb" value="' + escapeAttr(data.max_upload_size_mb || 1) + '">';
    html += '</div>';

    // ── Python Tool Bridge ──────────────────────────────────────────────────
    const bridgeData = (configData['tools'] || {})['python_tool_bridge'] || {};
    const bridgeEnabled = bridgeData.enabled === true;
    const bridgeAllowedTools = Array.isArray(bridgeData.allowed_tools) ? bridgeData.allowed_tools : [];
    const bridgeAllowedSQL = Array.isArray(bridgeData.allowed_sql_connections) ? bridgeData.allowed_sql_connections : [];

    html += '<div class="section-divider"></div>';
    html += '<div class="section-sub-header">🔌 ' + t('config.skill_manager.tool_bridge_header') + '</div>';

    const helpBridgeEnabled = t('help.tools.python_tool_bridge.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.tool_bridge_enabled_label') + '</div>';
    if (helpBridgeEnabled) html += '<div class="field-help">' + helpBridgeEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (bridgeEnabled ? ' on' : '') + '" data-path="tools.python_tool_bridge.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (bridgeEnabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    const helpBridgeTools = t('help.tools.python_tool_bridge.allowed_tools');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.skill_manager.tool_bridge_allowed_tools_label') + '</div>';
    if (helpBridgeTools) html += '<div class="field-help">' + helpBridgeTools + '</div>';
    html += renderPythonToolBridgeAllowedToolsPicker(bridgeAllowedTools);
    html += '</div>';

    const helpBridgeSQL = t('help.tools.python_tool_bridge.allowed_sql_connections');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + (t('config.skill_manager.tool_bridge_allowed_sql_connections_label') || 'Allowed Databases') + '</div>';
    if (helpBridgeSQL) html += '<div class="field-help">' + helpBridgeSQL + '</div>';
    html += renderPythonToolBridgeSQLConnectionsPicker(bridgeAllowedSQL);
    html += '</div>';

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Load catalog async and render checkboxes
    await ptbLoadAndRenderCatalog();
}

function renderPythonToolBridgeAllowedToolsPicker(currentAllowedTools) {
    const allowed = Array.isArray(currentAllowedTools) ? currentAllowedTools : [];
    const allowedJSON = escapeAttr(JSON.stringify(allowed));

    return `
        <div class="ptb-tools-wrap">
            <div class="ptb-tools-toolbar">
                <input id="ptb-tools-search" class="cfg-input ptb-tools-search" placeholder="${escapeAttr(t('config.skill_manager.tool_bridge_search') || 'Search…')}" oninput="ptbFilterCatalog()">
                <span id="ptb-selected-count" class="ptb-selected-count"></span>
                <button class="btn btn-sm" onclick="ptbSelectAll(true)">${t('config.skill_manager.tool_bridge_select_all') || 'Select all'}</button>
                <button class="btn btn-sm" onclick="ptbSelectAll(false)">${t('config.skill_manager.tool_bridge_select_none') || 'Select none'}</button>
            </div>

            <textarea class="cfg-input is-hidden" id="ptb-allowed-tools-state"
                data-path="tools.python_tool_bridge.allowed_tools" data-type="json" style="display:none;">${allowedJSON}</textarea>

            <div id="ptb-tools-status" class="ptb-tools-status">${t('config.skill_manager.tool_bridge_loading') || 'Loading…'}</div>
            <div id="ptb-tools-list" class="mcp-srv-tools-list"></div>
            <div id="ptb-tools-warning" class="ptb-tools-warning" style="display:none;"></div>
        </div>
    `;
}

function renderPythonToolBridgeSQLConnectionsPicker(currentAllowedSQL) {
    const allowed = Array.isArray(currentAllowedSQL) ? currentAllowedSQL : [];
    const allowedJSON = escapeAttr(JSON.stringify(allowed));
    const tmpl = t('config.skill_manager.tool_bridge_selected_dbs') || '{n} selected';
    const countText = String(tmpl).replace('{n}', String(allowed.length));

    return `
        <div class="ptb-sql-wrap">
            <div class="ptb-sql-row">
                <button class="btn btn-sm" onclick="ptbOpenSQLConnectionsModal()">
                    ${t('config.skill_manager.tool_bridge_choose_dbs') || 'Choose…'}
                </button>
                <span id="ptb-sql-count" class="ptb-selected-count">${esc(countText)}</span>
            </div>
            <textarea class="cfg-input is-hidden" id="ptb-allowed-sql-state"
                data-path="tools.python_tool_bridge.allowed_sql_connections" data-type="json" style="display:none;">${allowedJSON}</textarea>
        </div>
    `;
}

async function ptbLoadAndRenderCatalog() {
    const statusEl = document.getElementById('ptb-tools-status');
    const listEl = document.getElementById('ptb-tools-list');
    if (!statusEl || !listEl) return;

    try {
        if (!ptbCatalogCache) {
            const resp = await fetch('/api/python-tool-bridge/tools');
            if (!resp.ok) throw new Error('Failed to load tool catalog');
            ptbCatalogCache = await resp.json();
        }
        ptbRenderCatalog(ptbCatalogCache);
    } catch (e) {
        statusEl.textContent = t('config.skill_manager.tool_bridge_load_error') || 'Error loading tools';
        listEl.innerHTML = '';
    }
}

function ptbRenderCatalog(catalog) {
    const statusEl = document.getElementById('ptb-tools-status');
    const listEl = document.getElementById('ptb-tools-list');
    if (!statusEl || !listEl) return;

    const groups = (catalog && catalog.groups) ? catalog.groups : [];
    if (!groups.length) {
        statusEl.textContent = t('config.skill_manager.tool_bridge_no_tools') || 'No tools available';
        listEl.innerHTML = '';
        ptbUpdateWarning();
        return;
    }

    statusEl.textContent = '';

    const allowed = ptbGetAllowedTools();
    const allowSet = new Set(allowed);

    let html = '';
    for (const g of groups) {
        const key = g.key || '';
        const icon = g.icon || '🔧';
        const toolNames = Array.isArray(g.tools) ? g.tools : [];
        if (!key || !toolNames.length) continue;

        const label = t('config.section.' + key + '.label') || key;
        const desc = t('config.section.' + key + '.desc') || '';

        const toolHint = toolNames.map(n => `<code class="ptb-tool-code">${esc(n)}</code>`).join(' ');

        html += `
            <label class="ptb-tool-item" data-ptb-key="${escapeAttr(key)}" data-ptb-search="${escapeAttr((label + ' ' + key + ' ' + toolNames.join(' ')).toLowerCase())}">
                <input type="checkbox" class="ptb-group-cb" onchange="ptbToggleGroup('${escapeAttr(key)}')">
                <div class="ptb-tool-meta">
                    <div class="ptb-tool-title">${icon} ${esc(label)}</div>
                    ${desc ? `<div class="ptb-tool-desc">${esc(desc)}</div>` : ''}
                    <div class="ptb-tool-hint">${toolHint}</div>
                </div>
                <textarea class="is-hidden ptb-group-tools" style="display:none;">${escapeAttr(JSON.stringify(toolNames))}</textarea>
            </label>
        `;
    }

    listEl.innerHTML = html;
    ptbSyncCheckboxStates();
    ptbFilterCatalog();
    ptbUpdateWarning();
}

function ptbGetAllowedTools() {
    const el = document.getElementById('ptb-allowed-tools-state');
    if (!el) return [];
    try {
        const parsed = JSON.parse(el.value || '[]');
        return Array.isArray(parsed) ? parsed : [];
    } catch (_) {
        return [];
    }
}

function ptbSetAllowedTools(selected) {
    const el = document.getElementById('ptb-allowed-tools-state');
    if (!el) return;
    const uniq = Array.from(new Set((selected || []).map(s => String(s).trim()).filter(Boolean))).sort();
    el.value = JSON.stringify(uniq);
    setNestedValue(configData, 'tools.python_tool_bridge.allowed_tools', uniq);
    setDirty(true);
    ptbSyncCheckboxStates();
    ptbUpdateWarning();
}

function ptbGetAllowedSQLConnections() {
    const el = document.getElementById('ptb-allowed-sql-state');
    if (!el) return [];
    try {
        const parsed = JSON.parse(el.value || '[]');
        return Array.isArray(parsed) ? parsed : [];
    } catch (_) {
        return [];
    }
}

function ptbSetAllowedSQLConnections(selected) {
    const el = document.getElementById('ptb-allowed-sql-state');
    if (!el) return;
    const uniq = Array.from(new Set((selected || []).map(s => String(s).trim()).filter(Boolean))).sort();
    el.value = JSON.stringify(uniq);
    setNestedValue(configData, 'tools.python_tool_bridge.allowed_sql_connections', uniq);
    setDirty(true);

    const countEl = document.getElementById('ptb-sql-count');
    if (countEl) {
        const tmpl = t('config.skill_manager.tool_bridge_selected_dbs') || '{n} selected';
        countEl.textContent = String(tmpl).replace('{n}', String(uniq.length));
    }
}

function ptbToggleGroup(groupKey) {
    const listEl = document.getElementById('ptb-tools-list');
    if (!listEl) return;
    const item = listEl.querySelector(`[data-ptb-key="${escapeAttr(groupKey)}"]`) || listEl.querySelector('[data-ptb-key="' + groupKey + '"]');
    if (!item) return;

    const toolsEl = item.querySelector('.ptb-group-tools');
    if (!toolsEl) return;

    let tools = [];
    try { tools = JSON.parse(toolsEl.value || toolsEl.textContent || '[]'); } catch (_) { tools = []; }
    if (!Array.isArray(tools) || !tools.length) return;

    const cb = item.querySelector('.ptb-group-cb');
    const allowSet = new Set(ptbGetAllowedTools());
    const willEnable = cb && cb.checked === true;

    if (willEnable) {
        tools.forEach(n => allowSet.add(n));
    } else {
        tools.forEach(n => allowSet.delete(n));
    }

    ptbSetAllowedTools(Array.from(allowSet));
}

function ptbSyncCheckboxStates() {
    const listEl = document.getElementById('ptb-tools-list');
    if (!listEl) return;

    const allowed = ptbGetAllowedTools();
    const allowSet = new Set(allowed);

    const items = Array.from(listEl.querySelectorAll('.ptb-tool-item'));
    items.forEach(it => {
        const cb = it.querySelector('.ptb-group-cb');
        const toolsEl = it.querySelector('.ptb-group-tools');
        if (!cb || !toolsEl) return;

        let tools = [];
        try { tools = JSON.parse(toolsEl.value || toolsEl.textContent || '[]'); } catch (_) { tools = []; }
        if (!Array.isArray(tools) || !tools.length) return;

        const selectedCount = tools.filter(n => allowSet.has(n)).length;
        cb.indeterminate = selectedCount > 0 && selectedCount < tools.length;
        cb.checked = selectedCount === tools.length;
    });

    const countEl = document.getElementById('ptb-selected-count');
    if (countEl) {
        const tmpl = t('config.skill_manager.tool_bridge_selected_count') || 'Selected: {n}';
        countEl.textContent = String(tmpl).replace('{n}', String(allowed.length));
    }
}

function ptbSelectAll(enable) {
    const listEl = document.getElementById('ptb-tools-list');
    if (!listEl) return;

    const items = Array.from(listEl.querySelectorAll('.ptb-tool-item'));
    if (!items.length) return;

    if (!enable) {
        items.forEach(it => {
            const cb = it.querySelector('.ptb-group-cb');
            if (cb) cb.checked = false;
        });
        ptbSetAllowedTools([]);
        return;
    }

    const allowSet = new Set();
    items.forEach(it => {
        const cb = it.querySelector('.ptb-group-cb');
        if (cb) cb.checked = true;
        const toolsEl = it.querySelector('.ptb-group-tools');
        if (!toolsEl) return;
        try {
            const tools = JSON.parse(toolsEl.value || toolsEl.textContent || '[]');
            if (Array.isArray(tools)) tools.forEach(n => allowSet.add(n));
        } catch (_) { }
    });
    ptbSetAllowedTools(Array.from(allowSet));
}

function ptbFilterCatalog() {
    const qEl = document.getElementById('ptb-tools-search');
    const listEl = document.getElementById('ptb-tools-list');
    if (!qEl || !listEl) return;

    const q = (qEl.value || '').trim().toLowerCase();
    const items = Array.from(listEl.querySelectorAll('.ptb-tool-item'));
    items.forEach(it => {
        const hay = it.getAttribute('data-ptb-search') || '';
        it.style.display = (!q || hay.includes(q)) ? '' : 'none';
    });
}

function ptbUpdateWarning() {
    const warnEl = document.getElementById('ptb-tools-warning');
    if (!warnEl) return;
    const selected = ptbGetAllowedTools();
    if (!selected.length) {
        warnEl.style.display = '';
        warnEl.textContent = t('config.skill_manager.tool_bridge_none_selected') || 'No tools selected. Python skills will not be able to call any native tools.';
        return;
    }

    // If SQL tool is selected, require explicit allowed SQL connections for the bridge.
    if (selected.includes('sql_query')) {
        const dbs = ptbGetAllowedSQLConnections();
        if (!dbs.length) {
            warnEl.style.display = '';
            warnEl.textContent = t('config.skill_manager.tool_bridge_sql_none_selected') || 'sql_query is selected, but no databases are allowed. SQL bridge calls will be blocked.';
            return;
        }
    }
    warnEl.style.display = 'none';
    warnEl.textContent = '';
}

async function ptbOpenSQLConnectionsModal() {
    const existing = document.getElementById('ptb-sql-modal');
    if (existing) existing.remove();

    const modal = document.createElement('div');
    modal.id = 'ptb-sql-modal';
    modal.className = 'sec-modal-overlay';
    modal.innerHTML = `
        <div class="sec-modal-panel ptb-sql-modal-panel">
            <div class="sec-modal-title">🗄️ ${t('config.skill_manager.tool_bridge_sql_modal_title') || 'Allowed Databases'}</div>
            <div class="sec-modal-desc">${t('config.skill_manager.tool_bridge_sql_modal_desc') || 'Select which SQL connections Python skills may use via the tool bridge.'}</div>
            <input id="ptb-sql-search" class="cfg-input ptb-tools-search" placeholder="${escapeAttr(t('config.skill_manager.tool_bridge_sql_search') || 'Search…')}" oninput="ptbFilterSQLModal()">
            <div id="ptb-sql-list" class="mcp-srv-tools-list ptb-sql-list"></div>
            <div class="sec-modal-actions">
                <button id="ptb-sql-cancel" class="sec-modal-btn sec-modal-btn-skip">${t('common.btn_cancel') || 'Cancel'}</button>
                <button id="ptb-sql-save" class="sec-modal-btn sec-modal-btn-apply">✓ ${t('common.btn_save') || 'Save'}</button>
            </div>
        </div>
    `;

    function close() { modal.remove(); }
    modal.addEventListener('click', e => { if (e.target === modal) close(); });
    document.body.appendChild(modal);

    modal.querySelector('#ptb-sql-cancel').addEventListener('click', close);
    modal.querySelector('#ptb-sql-save').addEventListener('click', () => {
        const checked = Array.from(modal.querySelectorAll('.ptb-sql-cb')).filter(cb => cb.checked).map(cb => cb.value);
        ptbSetAllowedSQLConnections(checked);
        ptbUpdateWarning();
        close();
    });

    await ptbLoadSQLConnectionsIntoModal();
}

async function ptbLoadSQLConnectionsIntoModal() {
    const listEl = document.getElementById('ptb-sql-list');
    if (!listEl) return;

    listEl.innerHTML = `<div class="ptb-tools-status">${t('config.skill_manager.tool_bridge_loading') || 'Loading…'}</div>`;

    try {
        const resp = await fetch('/api/sql-connections');
        if (!resp.ok) throw new Error('Failed to load sql connections');
        const connections = await resp.json();

        const selected = new Set(ptbGetAllowedSQLConnections());
        const rows = Array.isArray(connections) ? connections : [];

        if (!rows.length) {
            listEl.innerHTML = `<div class="ptb-tools-status">${t('config.skill_manager.tool_bridge_sql_no_connections') || 'No SQL connections configured'}</div>`;
            return;
        }

        // Sort by name
        rows.sort((a, b) => String(a.name || '').localeCompare(String(b.name || ''), undefined, { sensitivity: 'base' }));

        let html = '';
        for (const c of rows) {
            const name = c.name || '';
            if (!name) continue;
            const driver = c.driver || '';
            const host = c.host || '';
            const db = c.database_name || '';
            const subtitle = [driver, host, db].filter(Boolean).join(' • ');
            const checked = selected.has(name) ? 'checked' : '';

            html += `
                <label class="ptb-tool-item ptb-sql-item" data-ptb-search="${escapeAttr((name + ' ' + driver + ' ' + host + ' ' + db).toLowerCase())}">
                    <input type="checkbox" class="ptb-sql-cb" value="${escapeAttr(name)}" ${checked}>
                    <div class="ptb-tool-meta">
                        <div class="ptb-tool-title">🗄️ ${esc(name)}</div>
                        ${subtitle ? `<div class="ptb-tool-desc">${esc(subtitle)}</div>` : ''}
                    </div>
                </label>
            `;
        }
        listEl.innerHTML = html;
        ptbFilterSQLModal();
    } catch (_) {
        listEl.innerHTML = `<div class="ptb-tools-status">${t('config.skill_manager.tool_bridge_load_error') || 'Error loading tool list'}</div>`;
    }
}

function ptbFilterSQLModal() {
    const qEl = document.getElementById('ptb-sql-search');
    const listEl = document.getElementById('ptb-sql-list');
    if (!qEl || !listEl) return;

    const q = (qEl.value || '').trim().toLowerCase();
    const items = Array.from(listEl.querySelectorAll('.ptb-sql-item'));
    items.forEach(it => {
        const hay = it.getAttribute('data-ptb-search') || '';
        it.style.display = (!q || hay.includes(q)) ? '' : 'none';
    });
}
