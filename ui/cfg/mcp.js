let mcpServersCache = null;
let mcpPreferencesCache = null;
let mcpRuntimeToolsCache = {};
let mcpSecretsCache = null;

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

    html += `<div class="field-group" style="margin-top:1.5rem;">
        <div class="field-group-title">🔌 ${t('config.mcp.server_title')}</div>
        <div class="field-group-desc">${t('config.mcp.server_desc')}</div>
        <div class="mcp-action-row">
            <button class="btn-save cfg-save-btn-sm" onclick="mcpServerAdd()">
                ＋ ${t('config.mcp.new_server')}
            </button>
        </div>
        <div id="mcp-servers-list"></div>
        <div id="mcp-servers-empty" class="mcp-empty-state is-hidden">
            ${t('config.mcp.empty')}
        </div>
    </div>
    <div class="field-group" style="margin-top:1.5rem;">
        <div class="field-group-title">🔐 ${t('config.mcp.secrets_title')}</div>
        <div class="field-group-desc">${t('config.mcp.secrets_desc')}</div>
        <div class="mcp-action-row">
            <button class="btn-save cfg-save-btn-sm" onclick="mcpSecretAdd()">
                ＋ ${t('config.mcp.new_secret')}
            </button>
        </div>
        <div id="mcp-secrets-list"></div>
        <div id="mcp-secrets-empty" class="mcp-empty-state is-hidden">
            ${t('config.mcp.secrets_empty')}
        </div>
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
    mcpSecretRenderCards();
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
    if (mcpSecretsCache === null) {
        try {
            const resp = await fetch('/api/mcp-secrets');
            const data = resp.ok ? await resp.json() : { secrets: [] };
            mcpSecretsCache = Array.isArray(data.secrets) ? data.secrets : [];
        } catch (_) {
            mcpSecretsCache = [];
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
        const runtimeLabel = s.runtime === 'docker' ? t('config.mcp.runtime_docker') : t('config.mcp.runtime_local');
        const workdirLabel = s.host_workdir || '—';

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
                <div><span class="mcp-grid-label">${t('config.mcp.card_runtime')}</span> ${escapeAttr(runtimeLabel)}</div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_env_vars')}</span> ${envCount}</div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_workdir')}</span> <code>${escapeAttr(workdirLabel)}</code></div>
            </div>
        </div>`;
    });
    wrap.innerHTML = html;
}

function mcpSecretRenderCards() {
    const wrap = document.getElementById('mcp-secrets-list');
    const empty = document.getElementById('mcp-secrets-empty');
    if (!wrap) return;
    if (!mcpSecretsCache || mcpSecretsCache.length === 0) {
        wrap.innerHTML = '';
        if (empty) empty.classList.remove('is-hidden');
        return;
    }
    if (empty) empty.classList.add('is-hidden');

    let html = '';
    mcpSecretsCache.forEach(secret => {
        const statusBadge = secret.has_value
            ? `<span class="mcp-badge mcp-badge-active">✅ ${t('config.mcp.secret_status_set')}</span>`
            : `<span class="mcp-badge mcp-badge-inactive">⚠️ ${t('config.mcp.secret_status_missing')}</span>`;
        html += `
        <div class="mcp-card" style="margin-top:1rem;">
            <div class="mcp-card-header">
                <div class="mcp-card-title">
                    🔑 ${escapeAttr(secret.label || secret.alias || '—')}${statusBadge}
                </div>
                <div class="mcp-card-actions">
                    <button onclick="mcpSecretEdit('${escapeAttr(secret.alias)}')" class="mcp-card-btn mcp-card-btn-edit" title="${t('config.mcp.card_edit_tooltip')}">✏️</button>
                    <button onclick="mcpSecretDelete('${escapeAttr(secret.alias)}')" class="mcp-card-btn mcp-card-btn-delete" title="${t('config.mcp.card_delete_tooltip')}">🗑️</button>
                </div>
            </div>
            <div class="mcp-card-grid">
                <div><span class="mcp-grid-label">${t('config.mcp.secret_alias')}</span> <code>${escapeAttr(secret.alias || '—')}</code></div>
                <div><span class="mcp-grid-label">${t('config.mcp.secret_usage')}</span> <code>{{${escapeAttr(secret.alias || '')}}}</code></div>
            </div>
            ${secret.description ? `<div class="field-help" style="margin-top:.35rem;">${escapeAttr(secret.description)}</div>` : ''}
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
                if (selectedTool) {
                    toolHelp = `<div class="field-help" style="margin-top:.35rem;">${escapeAttr(t('config.mcp.mapping_tool_selected', { capability: t(capability.titleKey) }))}</div>`;
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
    mcpServerShowModal({
        name: '',
        command: '',
        args: [],
        env: {},
        enabled: true,
        runtime: 'local',
        docker_image: '',
        docker_command: '',
        allow_local_fallback: false,
        host_workdir: '',
        container_workdir: '/workspace'
    }, -1);
}

function mcpServerEdit(idx) {
    mcpServerShowModal({ ...mcpServersCache[idx] }, idx);
}

async function mcpServerDelete(idx) {
    const s = mcpServersCache[idx];
    if (!await showConfirm(t('config.mcp.delete_confirm', { name: s.name }))) return;
    mcpServersCache.splice(idx, 1);
    await mcpServerSave();
    mcpServerRenderCards();
    mcpRuntimeToolsCache = {};
    await mcpRenderRoutingCards();
}

function mcpServerShowModal(data, idx) {
    const isEdit = idx >= 0;
    const argsStr = (data.args || []).join('\n');
    const envStr = data.env ? Object.entries(data.env).map(([k, v]) => k + '=' + v).join('\n') : '';
    const runtime = data.runtime || 'local';

    const overlay = document.createElement('div');
    overlay.className = 'mcp-modal-overlay';
    overlay.innerHTML = `
    <div class="mcp-modal-box">
        <div class="mcp-modal-title">${isEdit ? t('config.mcp.edit_server') : t('config.mcp.new_server')}</div>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_name')}</span>
            <input id="mcp-m-name" class="field-input cfg-input-full" value="${escapeAttr(data.name || '')}" placeholder="my-server">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_command')}</span>
            <input id="mcp-m-command" class="field-input cfg-input-full" value="${escapeAttr(data.command || '')}" placeholder="npx">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_runtime')}</span>
            <select id="mcp-m-runtime" class="field-input cfg-input-full">
                <option value="local" ${runtime === 'local' ? 'selected' : ''}>${t('config.mcp.runtime_local')}</option>
                <option value="docker" ${runtime === 'docker' ? 'selected' : ''}>${t('config.mcp.runtime_docker')}</option>
            </select>
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_args_label')} <small class="mcp-modal-hint">(${t('config.mcp.args_hint')})</small></span>
            <textarea id="mcp-m-args" class="field-input mcp-modal-textarea" rows="3" placeholder="-y\n@my/mcp-server">${escapeAttr(argsStr)}</textarea>
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_environment')} <small class="mcp-modal-hint">(KEY=VALUE, ${t('config.mcp.env_hint')}; {{alias}}, {{workdir}})</small></span>
            <textarea id="mcp-m-env" class="field-input mcp-modal-textarea" rows="4" placeholder="API_KEY={{api-token}}\nBASE_PATH={{workdir}}">${escapeAttr(envStr)}</textarea>
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_docker_image')}</span>
            <input id="mcp-m-docker-image" class="field-input cfg-input-full" value="${escapeAttr(data.docker_image || '')}" placeholder="ghcr.io/astral-sh/uv:latest">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_docker_command')}</span>
            <input id="mcp-m-docker-command" class="field-input cfg-input-full" value="${escapeAttr(data.docker_command || '')}" placeholder="uvx">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_host_workdir')}</span>
            <input id="mcp-m-host-workdir" class="field-input cfg-input-full" value="${escapeAttr(data.host_workdir || '')}" placeholder="agent_workspace/mcp/${escapeAttr(data.name || 'server')}">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.field_container_workdir')}</span>
            <input id="mcp-m-container-workdir" class="field-input cfg-input-full" value="${escapeAttr(data.container_workdir || '/workspace')}" placeholder="/workspace">
        </label>
        <label class="mcp-modal-check-row">
            <input id="mcp-m-enabled" type="checkbox" ${data.enabled ? 'checked' : ''}>
            <span class="mcp-modal-check-text">${t('config.mcp.enabled_checkbox')}</span>
        </label>
        <label class="mcp-modal-check-row">
            <input id="mcp-m-local-fallback" type="checkbox" ${data.allow_local_fallback ? 'checked' : ''}>
            <span class="mcp-modal-check-text">${t('config.mcp.field_allow_local_fallback')}</span>
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
            runtime: document.getElementById('mcp-m-runtime').value,
            args: document.getElementById('mcp-m-args').value.split('\n').map(l => l.trim()).filter(Boolean),
            env: {},
            enabled: document.getElementById('mcp-m-enabled').checked,
            docker_image: document.getElementById('mcp-m-docker-image').value.trim(),
            docker_command: document.getElementById('mcp-m-docker-command').value.trim(),
            allow_local_fallback: document.getElementById('mcp-m-local-fallback').checked,
            host_workdir: document.getElementById('mcp-m-host-workdir').value.trim(),
            container_workdir: document.getElementById('mcp-m-container-workdir').value.trim()
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

function mcpSecretAdd() {
    mcpSecretShowModal({ alias: '', label: '', description: '', has_value: false }, '');
}

function mcpSecretEdit(alias) {
    const secret = (mcpSecretsCache || []).find(item => item.alias === alias);
    if (!secret) return;
    mcpSecretShowModal(secret, alias);
}

async function mcpSecretDelete(alias) {
    const secret = (mcpSecretsCache || []).find(item => item.alias === alias);
    if (!secret) return;
    if (!await showConfirm(t('config.mcp.secret_delete_confirm', { name: secret.label || secret.alias }))) return;
    mcpSecretsCache = (mcpSecretsCache || []).filter(item => item.alias !== alias);
    await mcpSaveSecrets({}, { [alias]: true });
    mcpSecretRenderCards();
}

function mcpSecretShowModal(data, originalAlias) {
    const overlay = document.createElement('div');
    overlay.className = 'mcp-modal-overlay';
    overlay.innerHTML = `
    <div class="mcp-modal-box">
        <div class="mcp-modal-title">${originalAlias ? t('config.mcp.edit_secret') : t('config.mcp.new_secret')}</div>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.secret_alias')}</span>
            <input id="mcp-s-alias" class="field-input cfg-input-full" value="${escapeAttr(data.alias || '')}" placeholder="api-token">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.secret_label')}</span>
            <input id="mcp-s-label" class="field-input cfg-input-full" value="${escapeAttr(data.label || '')}" placeholder="MiniMax API Token">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.secret_description')}</span>
            <input id="mcp-s-description" class="field-input cfg-input-full" value="${escapeAttr(data.description || '')}" placeholder="${t('config.mcp.secret_description_placeholder')}">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">${t('config.mcp.secret_value')}</span>
            <input id="mcp-s-value" type="password" class="field-input cfg-input-full" value="" placeholder="${data.has_value ? t('config.mcp.secret_value_placeholder_set') : t('config.mcp.secret_value_placeholder_empty')}">
        </label>
        <label class="mcp-modal-check-row">
            <input id="mcp-s-clear" type="checkbox">
            <span class="mcp-modal-check-text">${t('config.mcp.secret_clear_value')}</span>
        </label>
        <div class="mcp-modal-footer">
            <button class="btn-save mcp-btn-cancel" onclick="this.closest('.mcp-modal-overlay').remove()">${t('config.mcp.cancel')}</button>
            <button class="btn-save mcp-btn-save" id="mcp-s-save">${t('config.mcp.save')}</button>
        </div>
    </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });

    document.getElementById('mcp-s-save').addEventListener('click', async () => {
        const alias = document.getElementById('mcp-s-alias').value.trim().toLowerCase();
        const label = document.getElementById('mcp-s-label').value.trim();
        const description = document.getElementById('mcp-s-description').value.trim();
        const value = document.getElementById('mcp-s-value').value;
        const clearValue = document.getElementById('mcp-s-clear').checked;
        if (!alias) {
            showToast(t('config.mcp.secret_alias_required'), 'warn');
            return;
        }
        mcpSecretsCache = (mcpSecretsCache || []).filter(item => item.alias !== originalAlias && item.alias !== alias);
        mcpSecretsCache.push({
            alias,
            label,
            description,
            has_value: clearValue ? false : !!(value || data.has_value)
        });
        mcpSecretsCache.sort((a, b) => a.alias.localeCompare(b.alias));

        const deleteFlags = {};
        if (originalAlias && originalAlias !== alias) {
            deleteFlags[originalAlias] = true;
        }
        if (clearValue) {
            deleteFlags[alias] = true;
        }
        await mcpSaveSecrets(value ? { [alias]: value } : {}, deleteFlags);
        overlay.remove();
        mcpSecretRenderCards();
    });
}

async function mcpServerSave() {
    try {
        const resp = await fetch('/api/mcp-servers', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
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

async function mcpSaveSecrets(valuesByAlias = {}, deleteFlags = {}) {
    try {
        const resp = await fetch('/api/mcp-secrets', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                secrets: (mcpSecretsCache || []).map(secret => ({
                    alias: secret.alias,
                    label: secret.label || '',
                    description: secret.description || '',
                    value: valuesByAlias[secret.alias] || '',
                    delete_value: !!deleteFlags[secret.alias]
                }))
            })
        });
        if (!resp.ok) throw new Error(await resp.text());
        const data = await resp.json();
        mcpSecretsCache = Array.isArray(data.secrets) ? data.secrets : [];
        showToast(t('config.mcp.secret_saved'), 'success');
    } catch (e) {
        showToast(t('config.mcp.secret_save_failed') + e.message, 'error');
    }
}

async function mcpSavePreferences() {
    try {
        const resp = await fetch('/api/mcp-preferences', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(configData.mcp.preferred_capabilities || {})
        });
        if (!resp.ok) throw new Error(await resp.text());
        showToast(t('config.mcp.mapping_saved'), 'success');
    } catch (e) {
        showToast(t('config.mcp.mapping_save_failed') + e.message, 'error');
    }
}
