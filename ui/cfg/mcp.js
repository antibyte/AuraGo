let mcpServersCache = null;
let mcpPreferencesCache = null;
let mcpRuntimeToolsCache = {};

const MCP_CAPABILITY_DEFS = [
    {
        key: 'web_search',
        icon: '🔎',
        titleKey: 'config.mcp.mapping_web_search',
        descKey: 'config.mcp.mapping_web_search_desc'
    },
    {
        key: 'vision',
        icon: '🖼️',
        titleKey: 'config.mcp.mapping_vision',
        descKey: 'config.mcp.mapping_vision_desc'
    }
];

async function renderMCPSection(section) {
    await mcpEnsureCaches();
    const mcpEnabled = configData.mcp && configData.mcp.enabled;
    const allowMcp = configData.agent && configData.agent.allow_mcp;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    if (!allowMcp) {
        html += `<div class="wh-notice mcp-notice-danger">
            <span>🔒</span>
            <div>
                <strong>${t('config.mcp.locked_notice')}</strong><br>
                <small>${t('config.mcp.locked_desc')}</small>
            </div>
        </div></div>`;
        document.getElementById('content').innerHTML = html;
        return;
    }

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.mcp.enabled_label')}</span>
        <div class="toggle ${mcpEnabled ? 'on' : ''}" data-path="mcp.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!mcpEnabled) {
        html += `<div class="wh-notice">
            <span>⚠️</span>
            <div>
                <strong>${t('config.mcp.disabled_notice')}</strong><br>
                <small>${t('config.mcp.disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `<div class="mcp-action-row">
        <button class="btn-save cfg-save-btn-sm" onclick="mcpServerAdd()">
            ＋ ${t('config.mcp.new_server')}
        </button>
    </div>
    <div id="mcp-servers-list"></div>
    <div id="mcp-servers-empty" class="mcp-empty-state is-hidden">
        ${t('config.mcp.empty')}
    </div>
    <div class="field-group" style="margin-top:1.5rem;">
        <div class="field-group-title">🧭 ${t('config.mcp.routing_title')}</div>
        <div class="field-group-desc">${t('config.mcp.routing_desc')}</div>
        <div id="mcp-routing-list"></div>
    </div>
    </div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    mcpServerRenderCards();
    await mcpRenderRoutingCards();
}

async function mcpEnsureCaches() {
    if (mcpServersCache === null) {
        try {
            const resp = await fetch('/api/mcp-servers');
            mcpServersCache = resp.ok ? await resp.json() : [];
        } catch (_) {
            mcpServersCache = [];
        }
    }
    if (mcpPreferencesCache === null) {
        try {
            const resp = await fetch('/api/mcp-preferences');
            mcpPreferencesCache = resp.ok ? await resp.json() : {};
        } catch (_) {
            mcpPreferencesCache = {};
        }
    }
    configData.mcp = configData.mcp || {};
    configData.mcp.preferred_capabilities = mcpPreferencesCache || {};
}

function mcpServerRenderCards() {
    const wrap = document.getElementById('mcp-servers-list');
    const empty = document.getElementById('mcp-servers-empty');
    if (!wrap) return;
    if (mcpServersCache.length === 0) {
        wrap.innerHTML = '';
        if (empty) empty.classList.remove('is-hidden');
        return;
    }
    if (empty) empty.classList.add('is-hidden');

    let html = '';
    mcpServersCache.forEach((s, idx) => {
        const enabledBadge = s.enabled
            ? `<span class="mcp-badge mcp-badge-active">✅ ${t('config.mcp.active_badge')}</span>`
            : `<span class="mcp-badge mcp-badge-inactive">⏸ ${t('config.mcp.inactive_badge')}</span>`;
        const argsStr = (s.args || []).join(' ');
        const envCount = s.env ? Object.keys(s.env).length : 0;

        html += `
        <div class="mcp-card" data-idx="${idx}">
            <div class="mcp-card-header">
                <div class="mcp-card-title">
                    🔌 ${escapeAttr(s.name || '—')}${enabledBadge}
                </div>
                <div class="mcp-card-actions">
                    <button onclick="mcpServerEdit(${idx})" class="mcp-card-btn mcp-card-btn-edit" title="${t('config.mcp.card_edit_tooltip')}">✏️</button>
                    <button onclick="mcpServerDelete(${idx})" class="mcp-card-btn mcp-card-btn-delete" title="${t('config.mcp.card_delete_tooltip')}">🗑️</button>
                </div>
            </div>
            <div class="mcp-card-grid">
                <div><span class="mcp-grid-label">${t('config.mcp.card_command')}</span> <code>${escapeAttr(s.command || '—')}</code></div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_args')}</span> ${argsStr ? '<code>' + escapeAttr(argsStr) + '</code>' : '—'}</div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_env_vars')}</span> ${envCount}</div>
            </div>
        </div>`;
    });
    wrap.innerHTML = html;
}

async function mcpRenderRoutingCards() {
    const container = document.getElementById('mcp-routing-list');
    if (!container) return;

    const enabledServers = (mcpServersCache || []).filter(server => server && server.enabled);
    let html = '';

    for (const capability of MCP_CAPABILITY_DEFS) {
        const pref = mcpGetPreference(capability.key);
        const serverOptions = enabledServers.map(server =>
            `<option value="${escapeAttr(server.name)}" ${server.name === pref.server ? 'selected' : ''}>${escapeAttr(server.name)}</option>`
        ).join('');

        let toolOptions = `<option value="">${t('config.mcp.mapping_select_tool')}</option>`;
        let toolHelp = '';
        if (pref.server) {
            const tools = await mcpGetRuntimeTools(pref.server);
            if (tools.length === 0) {
                toolOptions = `<option value="">${t('config.mcp.mapping_no_tools')}</option>`;
            } else {
                toolOptions += tools.map(tool =>
                    `<option value="${escapeAttr(tool.name)}" ${tool.name === pref.tool ? 'selected' : ''}>${escapeAttr(tool.name)}</option>`
                ).join('');
                const selectedTool = tools.find(tool => tool.name === pref.tool);
                if (selectedTool && selectedTool.description) {
                    toolHelp = `<div class="field-help" style="margin-top:.35rem;">${escapeAttr(selectedTool.description)}</div>`;
                }
            }
        }

        html += `
        <div class="mcp-card" style="margin-top:1rem;">
            <div class="mcp-card-header">
                <div class="mcp-card-title">${capability.icon} ${t(capability.titleKey)}</div>
            </div>
            <div class="field-group-desc" style="margin-bottom:.9rem;">${t(capability.descKey)}</div>
            <div class="mcp-card-grid">
                <label>
                    <span class="mcp-grid-label">${t('config.mcp.mapping_server')}</span>
                    <select class="field-input cfg-input-full" onchange="mcpPreferenceServerChanged('${capability.key}', this.value)">
                        <option value="">${t('config.mcp.mapping_builtin_option')}</option>
                        ${serverOptions}
                    </select>
                </label>
                <label>
                    <span class="mcp-grid-label">${t('config.mcp.mapping_tool')}</span>
                    <select class="field-input cfg-input-full" ${pref.server ? '' : 'disabled'} onchange="mcpPreferenceToolChanged('${capability.key}', this.value)">
                        ${toolOptions}
                    </select>
                </label>
            </div>
            ${toolHelp}
        </div>`;
    }

    if (!html) {
        html = `<div class="wh-notice"><span>ℹ️</span><div><small>${t('config.mcp.routing_empty')}</small></div></div>`;
    }
    container.innerHTML = html;
}

function mcpGetPreference(key) {
    const prefs = (configData.mcp && configData.mcp.preferred_capabilities) || {};
    const pref = prefs[key] || {};
    return {
        server: pref.server || '',
        tool: pref.tool || ''
    };
}

async function mcpGetRuntimeTools(serverName) {
    const key = String(serverName || '').trim();
    if (!key) return [];
    if (mcpRuntimeToolsCache[key]) return mcpRuntimeToolsCache[key];
    try {
        const resp = await fetch('/api/mcp-runtime/tools?server=' + encodeURIComponent(key));
        const data = resp.ok ? await resp.json() : { tools: [] };
        mcpRuntimeToolsCache[key] = Array.isArray(data.tools) ? data.tools : [];
    } catch (_) {
        mcpRuntimeToolsCache[key] = [];
    }
    return mcpRuntimeToolsCache[key];
}

async function mcpPreferenceServerChanged(capabilityKey, serverName) {
    const prefs = configData.mcp.preferred_capabilities || {};
    prefs[capabilityKey] = { server: serverName || '', tool: '' };
    configData.mcp.preferred_capabilities = prefs;
    mcpPreferencesCache = prefs;
    if (serverName) {
        await mcpGetRuntimeTools(serverName);
    }
    await mcpSavePreferences();
    await mcpRenderRoutingCards();
}

async function mcpPreferenceToolChanged(capabilityKey, toolName) {
    const prefs = configData.mcp.preferred_capabilities || {};
    const current = prefs[capabilityKey] || {};
    prefs[capabilityKey] = {
        server: current.server || '',
        tool: toolName || ''
    };
    configData.mcp.preferred_capabilities = prefs;
    mcpPreferencesCache = prefs;
    await mcpSavePreferences();
    await mcpRenderRoutingCards();
}

function mcpServerAdd() {
    mcpServerShowModal({name:'', command:'', args:[], env:{}, enabled:true}, -1);
}

function mcpServerEdit(idx) {
    mcpServerShowModal({...mcpServersCache[idx]}, idx);
}

async function mcpServerDelete(idx) {
    const s = mcpServersCache[idx];
    if (!await showConfirm(t('config.mcp.delete_confirm', {name: s.name}))) return;
    mcpServersCache.splice(idx, 1);
    await mcpServerSave();
    mcpServerRenderCards();
    mcpRuntimeToolsCache = {};
    await mcpRenderRoutingCards();
}

function mcpServerShowModal(data, idx) {
    const isEdit = idx >= 0;
    const argsStr = (data.args || []).join('\n');
    const envStr = data.env ? Object.entries(data.env).map(([k,v]) => k + '=' + v).join('\n') : '';

    const overlay = document.createElement('div');
    overlay.className = 'mcp-modal-overlay';
    overlay.innerHTML = `
    <div class="mcp-modal-box">
        <div class="mcp-modal-title">${isEdit ? t('config.mcp.edit_server') : t('config.mcp.new_server')}</div>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_name')}</span>
            <input id="mcp-m-name" class="field-input cfg-input-full" value="${escapeAttr(data.name)}" placeholder="my-server">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_command')}</span>
            <input id="mcp-m-command" class="field-input cfg-input-full" value="${escapeAttr(data.command)}" placeholder="npx">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_args_label') || 'Args'} <small class="mcp-modal-hint">(${t('config.mcp.args_hint')})</small></span>
            <textarea id="mcp-m-args" class="field-input mcp-modal-textarea" rows="3" placeholder="-y\n@my/mcp-server">${escapeAttr(argsStr)}</textarea>
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_environment')} <small class="mcp-modal-hint">(KEY=VALUE, ${t('config.mcp.env_hint')})</small></span>
            <textarea id="mcp-m-env" class="field-input mcp-modal-textarea" rows="3" placeholder="API_KEY=xxx\nDEBUG=1">${escapeAttr(envStr)}</textarea>
        </label>
        <label class="mcp-modal-check-row">
            <input id="mcp-m-enabled" type="checkbox" ${data.enabled ? 'checked' : ''}>
            <span class="mcp-modal-check-text">${t('config.mcp.enabled_checkbox')}</span>
        </label>
        <div class="mcp-modal-footer">
            <button class="btn-save mcp-btn-cancel" onclick="this.closest('.mcp-modal-overlay').remove()">${t('config.mcp.cancel')}</button>
            <button class="btn-save mcp-btn-save" id="mcp-m-save">${t('config.mcp.save')}</button>
        </div>
    </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });

    document.getElementById('mcp-m-save').addEventListener('click', async () => {
        const entry = {
            name: document.getElementById('mcp-m-name').value.trim(),
            command: document.getElementById('mcp-m-command').value.trim(),
            args: document.getElementById('mcp-m-args').value.split('\n').map(l => l.trim()).filter(Boolean),
            env: {},
            enabled: document.getElementById('mcp-m-enabled').checked
        };
        document.getElementById('mcp-m-env').value.split('\n').forEach(line => {
            const eq = line.indexOf('=');
            if (eq > 0) entry.env[line.substring(0, eq).trim()] = line.substring(eq + 1).trim();
        });
        if (!entry.name || !entry.command) {
            showToast(t('config.mcp.name_command_required'), 'warn');
            return;
        }
        if (isEdit) {
            mcpServersCache[idx] = entry;
        } else {
            if (mcpServersCache.some(s => s.name === entry.name)) {
                showToast(t('config.mcp.name_exists'), 'warn');
                return;
            }
            mcpServersCache.push(entry);
        }
        await mcpServerSave();
        overlay.remove();
        mcpServerRenderCards();
        mcpRuntimeToolsCache = {};
        await mcpRenderRoutingCards();
    });
}

async function mcpServerSave() {
    try {
        const resp = await fetch('/api/mcp-servers', {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({
                enabled: !!(configData.mcp && configData.mcp.enabled),
                servers: mcpServersCache
            })
        });
        if (!resp.ok) throw new Error(await resp.text());
        const reload = await fetch('/api/mcp-servers');
        if (reload.ok) mcpServersCache = await reload.json();
    } catch (e) {
        showToast(t('config.common.error') + ': ' + e.message, 'error');
    }
}

async function mcpSavePreferences() {
    try {
        const resp = await fetch('/api/mcp-preferences', {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(configData.mcp.preferred_capabilities || {})
        });
        if (!resp.ok) throw new Error(await resp.text());
        showToast(t('config.mcp.mapping_saved'), 'success');
    } catch (e) {
        showToast(t('config.mcp.mapping_save_failed') + e.message, 'error');
    }
}
