let mcpServersCache = null;
let mcpPreferencesCache = null;
let mcpRuntimeToolsCache = {};
let mcpSecretsCache = null;

const MCP_CAPABILITY_DEFS = [
    {
        key: 'web_search',
        icon: 'search',
        titleKey: 'config.mcp.mapping_web_search',
        descKey: 'config.mcp.mapping_web_search_desc'
    },
    {
        key: 'vision',
        icon: 'image',
        titleKey: 'config.mcp.mapping_vision',
        descKey: 'config.mcp.mapping_vision_desc'
    }
];

async function renderMCPSection(section) {
    mcpLastSection = section;
    await mcpEnsureCaches();
    const mcpEnabled = configData.mcp && configData.mcp.enabled;
    const allowMcp = configData.agent && configData.agent.allow_mcp;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    if (!allowMcp) {
        html += `<div class="wh-notice mcp-notice-danger">
            <span>${cfgIcon('lock')}</span>
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
        <div class="toggle ${mcpEnabled ? 'on' : ''}" data-path="mcp.enabled" aria-describedby="mcp-enabled-help" onclick="toggleBool(this)"></div>
    </div>
    <div class="field-help" id="mcp-enabled-help">${t('help.mcp.enabled')}</div>`;

    if (!mcpEnabled) {
        html += `<div class="wh-notice">
            <span>${cfgIcon('alert')}</span>
            <div>
                <strong>${t('config.mcp.disabled_notice')}</strong><br>
                <small>${t('config.mcp.disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `<div class="field-group pw-u-mt-150">
        <div class="field-group-title">${cfgIcon('plug')} ${t('config.mcp.server_title')}</div>
        <div class="field-group-desc">${t('config.mcp.server_desc')}</div>
        <div class="mcp-action-row cfg-actions-row">
            <button class="btn-save btn-secondary" onclick="mcpServerAdd()"${mcpDisabledAttr('servers')}>
                ${cfgIcon('plus')} ${t('config.mcp.new_server')}
            </button>
        </div>
        <div id="mcp-servers-list"></div>
    </div>
    <div class="field-group pw-u-mt-150">
        <div class="field-group-title">${cfgIcon('lock')} ${t('config.mcp.secrets_title')}</div>
        <div class="field-group-desc">${t('config.mcp.secrets_desc')}</div>
        <div class="mcp-action-row cfg-actions-row">
            <button class="btn-save btn-secondary" onclick="mcpSecretAdd()"${mcpDisabledAttr('secrets')}>
                ${cfgIcon('plus')} ${t('config.mcp.new_secret')}
            </button>
        </div>
        <div id="mcp-secrets-list"></div>
    </div>
    <div class="field-group pw-u-mt-150">
        <div class="field-group-title">${cfgIcon('compass')} ${t('config.mcp.routing_title')}</div>
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

const mcpLoadErrors = { servers: null, prefs: null, secrets: null };
let mcpLastSection = null;

async function mcpFetchJSON(url) {
    const resp = await fetch(url);
    if (!resp.ok) {
        const txt = (await resp.text().catch(() => '')).trim();
        throw new Error(txt || resp.statusText || ('HTTP ' + resp.status));
    }
    return resp.json();
}

async function mcpEnsureCaches() {
    if (mcpServersCache === null) {
        try {
            const data = await mcpFetchJSON('/api/mcp-servers');
            mcpServersCache = Array.isArray(data) ? data : [];
            mcpLoadErrors.servers = null;
        } catch (e) {
            mcpLoadErrors.servers = e.message || String(e);
        }
    }
    if (mcpPreferencesCache === null) {
        try {
            mcpPreferencesCache = (await mcpFetchJSON('/api/mcp-preferences')) || {};
            mcpLoadErrors.prefs = null;
        } catch (e) {
            mcpLoadErrors.prefs = e.message || String(e);
        }
    }
    if (mcpSecretsCache === null) {
        try {
            const data = await mcpFetchJSON('/api/mcp-secrets');
            mcpSecretsCache = Array.isArray(data.secrets) ? data.secrets : [];
            mcpLoadErrors.secrets = null;
        } catch (e) {
            mcpLoadErrors.secrets = e.message || String(e);
        }
    }
    configData.mcp = configData.mcp || {};
    if (mcpPreferencesCache !== null) {
        configData.mcp.preferred_capabilities = mcpPreferencesCache;
    }
}

function mcpRetryLoad() {
    if (mcpLoadErrors.servers) mcpServersCache = null;
    if (mcpLoadErrors.prefs) mcpPreferencesCache = null;
    if (mcpLoadErrors.secrets) mcpSecretsCache = null;
    if (mcpLastSection) renderMCPSection(mcpLastSection);
}

function mcpShowLoadError(target, what, reason) {
    cfgListState(target, { state: 'error', title: t('config.mcp.load_failed', { what }), text: reason || '', retry: mcpRetryLoad });
}

function mcpDisabledAttr(errKey) {
    return mcpLoadErrors[errKey] ? ` disabled title="${escapeAttr(t('config.mcp.load_failed_disabled'))}"` : '';
}

function mcpServerRenderCards() {
    const wrap = document.getElementById('mcp-servers-list');
    if (!wrap) return;
    if (mcpServersCache === null) {
        mcpShowLoadError(wrap, t('config.mcp.server_title'), mcpLoadErrors.servers);
        return;
    }
    if (mcpServersCache.length === 0) {
        cfgListState(wrap, { state: 'empty', title: t('config.mcp.empty') });
        return;
    }

    let html = '';
    mcpServersCache.forEach((s, idx) => {
        const enabledBadge = s.enabled
            ? cfgBadge('ok', t('config.mcp.active_badge'))
            : cfgBadge('idle', t('config.mcp.inactive_badge'));
        const argsStr = (s.args || []).join(' ');
        const envCount = s.env ? Object.keys(s.env).length : 0;
        const headersCount = s.headers ? Object.keys(s.headers).length : 0;
        const allowedToolsCount = Array.isArray(s.allowed_tools) ? s.allowed_tools.length : 0;
        const allowedToolsLabel = allowedToolsCount > 0 ? String(allowedToolsCount) : t('config.mcp.card_allowed_tools_all');
        const transport = s.transport || 'stdio';
        const transportLabel = mcpTransportLabel(transport);
        const runtimeLabel = s.runtime === 'docker' ? t('config.mcp.runtime_docker') : t('config.mcp.runtime_local');
        const workdirLabel = s.host_workdir || '—';
        const endpointLabel = transport === 'stdio' ? (s.command || '—') : (s.url || '—');
        const endpointKey = transport === 'stdio' ? 'config.mcp.card_command' : 'config.mcp.card_url';

        html += `
        <div class="mcp-card" data-idx="${idx}">
            <div class="mcp-card-header">
                <div class="mcp-card-title">
                    ${cfgIcon('plug')} ${escapeAttr(s.name || '—')} ${enabledBadge}
                </div>
                <div class="mcp-card-actions">
                    <button type="button" onclick="mcpServerEdit(${idx})" class="mcp-card-btn mcp-card-btn-edit" title="${t('config.mcp.card_edit_tooltip')}" aria-label="${escapeAttr(t('config.mcp.card_edit_tooltip') + ': ' + (s.name || ''))}">${cfgIcon('edit')}</button>
                    ${cfgDeleteButton({ iconOnly: true, label: t('config.common.delete_item', { name: s.name || '' }), attrs: { onclick: 'mcpServerDelete(' + idx + ')' } })}
                </div>
            </div>
            <div class="mcp-card-grid">
                <div><span class="mcp-grid-label">${t('config.mcp.card_transport')}</span> ${escapeAttr(transportLabel)}</div>
                <div><span class="mcp-grid-label">${t(endpointKey)}</span> <code>${escapeAttr(endpointLabel)}</code></div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_args')}</span> ${argsStr ? '<code>' + escapeAttr(argsStr) + '</code>' : '—'}</div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_runtime')}</span> ${escapeAttr(runtimeLabel)}</div>
                <div><span class="mcp-grid-label">${t(transport === 'stdio' ? 'config.mcp.card_env_vars' : 'config.mcp.card_headers')}</span> ${transport === 'stdio' ? envCount : headersCount}</div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_allowed_tools')}</span> ${escapeAttr(allowedToolsLabel)}</div>
                <div><span class="mcp-grid-label">${t('config.mcp.card_workdir')}</span> <code>${escapeAttr(workdirLabel)}</code></div>
            </div>
        </div>`;
    });
    wrap.innerHTML = html;
}

function mcpSecretRenderCards() {
    const wrap = document.getElementById('mcp-secrets-list');
    if (!wrap) return;
    if (mcpSecretsCache === null) {
        mcpShowLoadError(wrap, t('config.mcp.secrets_title'), mcpLoadErrors.secrets);
        return;
    }
    if (mcpSecretsCache.length === 0) {
        cfgListState(wrap, { state: 'empty', title: t('config.mcp.secrets_empty') });
        return;
    }

    let html = '';
    mcpSecretsCache.forEach(secret => {
        const statusBadge = secret.has_value
            ? cfgBadge('ok', t('config.mcp.secret_status_set'))
            : cfgBadge('warn', t('config.mcp.secret_status_missing'));
        html += `
        <div class="mcp-card pw-u-mt-100">
            <div class="mcp-card-header">
                <div class="mcp-card-title">
                    ${cfgIcon('key')} ${escapeAttr(secret.label || secret.alias || '—')} ${statusBadge}
                </div>
                <div class="mcp-card-actions">
                    <button type="button" onclick="mcpSecretEdit(${escapeAttr(JSON.stringify(String(secret.alias)))})" class="mcp-card-btn mcp-card-btn-edit" title="${t('config.mcp.card_edit_tooltip')}" aria-label="${escapeAttr(t('config.mcp.card_edit_tooltip') + ': ' + (secret.label || secret.alias || ''))}">${cfgIcon('edit')}</button>
                    ${cfgDeleteButton({ iconOnly: true, label: t('config.common.delete_item', { name: secret.label || secret.alias || '' }), attrs: { onclick: 'mcpSecretDelete(' + JSON.stringify(String(secret.alias)) + ')' } })}
                </div>
            </div>
            <div class="mcp-card-grid">
                <div><span class="mcp-grid-label">${t('config.mcp.secret_alias')}</span> <code>${escapeAttr(secret.alias || '—')}</code></div>
                <div><span class="mcp-grid-label">${t('config.mcp.secret_usage')}</span> <code>{{${escapeAttr(secret.alias || '')}}}</code></div>
            </div>
            ${secret.description ? `<div class="field-help pw-u-mt-035">${escapeAttr(secret.description)}</div>` : ''}
        </div>`;
    });
    wrap.innerHTML = html;
}

async function mcpRenderRoutingCards() {
    const container = document.getElementById('mcp-routing-list');
    if (!container) return;
    if (mcpPreferencesCache === null) {
        mcpShowLoadError(container, t('config.mcp.routing_title'), mcpLoadErrors.prefs);
        return;
    }

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
                    toolHelp = `<div class="field-help pw-u-mt-035">${escapeAttr(t('config.mcp.mapping_tool_selected', { capability: t(capability.titleKey) }))}</div>`;
                }
            }
        }

        html += `
        <div class="mcp-card pw-u-mt-100">
            <div class="mcp-card-header">
                <div class="mcp-card-title">${cfgIcon(capability.icon)} ${t(capability.titleKey)}</div>
            </div>
            <div class="field-group-desc pw-u-mb-090">${t(capability.descKey)}</div>
            <div class="mcp-card-grid">
                <label>
                    <span class="mcp-grid-label">${t('config.mcp.mapping_server')}</span>
                    <select class="field-select" aria-describedby="mcp-map-${capability.key}-server-help" onchange="mcpPreferenceServerChanged('${capability.key}', this.value)">
                        <option value="">${t('config.mcp.mapping_builtin_option')}</option>
                        ${serverOptions}
                    </select>
                    <div class="field-help" id="mcp-map-${capability.key}-server-help">${t('config.mcp.mapping_server_help')}</div>
                </label>
                <label>
                    <span class="mcp-grid-label">${t('config.mcp.mapping_tool')}</span>
                    <select class="field-select" ${pref.server ? '' : 'disabled'} aria-describedby="mcp-map-${capability.key}-tool-help" onchange="mcpPreferenceToolChanged('${capability.key}', this.value)">
                        ${toolOptions}
                    </select>
                    <div class="field-help" id="mcp-map-${capability.key}-tool-help">${t('config.mcp.mapping_tool_help')}</div>
                </label>
            </div>
            ${toolHelp}
        </div>`;
    }

    if (!html) {
        cfgListState(container, { state: 'empty', title: t('config.mcp.routing_empty') });
        return;
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
    if (mcpPreferencesCache === null) return;
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
    if (mcpPreferencesCache === null) return;
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
    if (mcpServersCache === null) { showToast(t('config.mcp.load_failed_disabled'), 'warn'); return; }
    mcpServerShowModal({
        name: '',
        transport: 'stdio',
        url: '',
        headers: {},
        command: '',
        args: [],
        env: {},
        enabled: true,
        runtime: 'local',
        docker_image: '',
        docker_command: '',
        allow_local_fallback: false,
        allow_private_network: false,
        host_workdir: '',
        container_workdir: '/workspace'
    }, -1);
}

function mcpServerEdit(idx) {
    if (mcpServersCache === null) return;
    mcpServerShowModal({ ...mcpServersCache[idx] }, idx);
}

async function mcpServerDelete(idx) {
    if (mcpServersCache === null) return;
    const s = mcpServersCache[idx];
    const answer = await showConfirmDialog({
        title: t('config.common.delete_item', { name: s.name }),
        message: t('config.mcp.delete_confirm', { name: s.name }),
        danger: true,
        confirmText: t('config.common.delete')
    });
    if (!answer.confirmed) return;
    const next = mcpServersCache.slice();
    next.splice(idx, 1);
    const res = await mcpServerSave(next);
    if (!res.ok) {
        showToast(t('config.common.error') + ': ' + res.error, 'error');
        return;
    }
    mcpServerRenderCards();
    mcpRuntimeToolsCache = {};
    await mcpRenderRoutingCards();
}

const MCP_SERVER_MODAL = 'mcp-server-dialog';
const MCP_SECRET_MODAL = 'mcp-secret-dialog';

// One dialog per kind: rebuilt with the entry's values on every open, removed again on close.
function mcpOpenModal(id, title, body, actions, focusSelector) {
    const old = document.getElementById(id);
    if (old) old.remove();
    document.body.insertAdjacentHTML('beforeend', cfgModal({ id, title, size: 'lg', body, actions }));
    const modal = document.getElementById(id);
    if (typeof enhanceConfigControls === 'function') enhanceConfigControls(modal);
    openModal(id, { initialFocus: focusSelector, onClose: () => modal.remove() });
    return modal;
}

function mcpModalField(id, labelHTML, controlHTML, helpHTML, cls) {
    return `<div class="field-group${cls ? ' ' + cls : ''}">
        <label class="field-label" for="${id}">${labelHTML}</label>
        ${controlHTML}
        <div class="field-help" id="${id}-help">${helpHTML}</div>
    </div>`;
}

function mcpModalHint(text) {
    return ` <small>(${text})</small>`;
}

// On/off rows in the dialogs are switches that only change the dialog; they never mark the page dirty.
function mcpModalToggleRow(id, labelHTML, helpHTML, on, cls, extra) {
    return `<div class="cfg-toggle-row${cls ? ' ' + cls : ''}">
        <div>
            <span class="cfg-toggle-label" id="${id}-label">${labelHTML}</span>
            <div class="field-help" id="${id}-help">${helpHTML}</div>
        </div>
        <div class="toggle${on ? ' on' : ''}" id="${id}" role="switch" tabindex="0" aria-checked="${on ? 'true' : 'false'}" aria-labelledby="${id}-label" aria-describedby="${id}-help"${extra || ''} onclick="mcpModalToggle(this)"></div>
    </div>`;
}

function mcpModalToggle(el) {
    const turningOn = !el.classList.contains('on');
    const apply = () => {
        el.classList.toggle('on', turningOn);
        el.setAttribute('aria-checked', turningOn ? 'true' : 'false');
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

function mcpModalShowError(boxId, message, fieldId) {
    const box = document.getElementById(boxId);
    if (box) { box.textContent = message; box.classList.remove('is-hidden'); }
    const field = fieldId ? document.getElementById(fieldId) : null;
    if (field) {
        field.setAttribute('aria-invalid', 'true');
        field.setAttribute('aria-describedby', fieldId + '-help ' + boxId);
        field.addEventListener('input', () => {
            field.removeAttribute('aria-invalid');
            field.setAttribute('aria-describedby', fieldId + '-help');
        }, { once: true });
        field.focus();
    }
}

function mcpModalClearError(boxId, modalId) {
    const box = document.getElementById(boxId);
    if (box) box.classList.add('is-hidden');
    const modal = document.getElementById(modalId);
    if (modal) modal.querySelectorAll('[aria-invalid]').forEach(el => el.removeAttribute('aria-invalid'));
}

function mcpServerShowModal(data, idx) {
    const isEdit = idx >= 0;
    const argsStr = (data.args || []).join('\n');
    const envStr = data.env ? Object.entries(data.env).map(([k, v]) => k + '=' + v).join('\n') : '';
    const headersStr = data.headers ? Object.entries(data.headers).map(([k, v]) => k + '=' + v).join('\n') : '';
    const allowedToolsStr = Array.isArray(data.allowed_tools) ? data.allowed_tools.join('\n') : '';
    const transport = data.transport || 'stdio';
    const runtime = data.runtime || 'local';
    const describe = id => ` aria-describedby="${id}-help"`;

    const body = `
        ${mcpModalField('mcp-m-name', t('config.mcp.field_name'),
            `<input id="mcp-m-name" class="field-input" value="${escapeAttr(data.name || '')}" placeholder="my-server"${describe('mcp-m-name')}>`,
            t('config.mcp.field_name_help'))}
        ${mcpModalField('mcp-m-transport', t('config.mcp.field_transport'),
            `<select id="mcp-m-transport" class="field-select"${describe('mcp-m-transport')}>
                <option value="stdio" ${transport === 'stdio' ? 'selected' : ''}>${t('config.mcp.transport_stdio')}</option>
                <option value="streamable_http" ${transport === 'streamable_http' ? 'selected' : ''}>${t('config.mcp.transport_streamable_http')}</option>
                <option value="sse" ${transport === 'sse' ? 'selected' : ''}>${t('config.mcp.transport_sse')}</option>
                <option value="websocket" ${transport === 'websocket' ? 'selected' : ''}>${t('config.mcp.transport_websocket')}</option>
            </select>`,
            t('config.mcp.field_transport_help'))}
        ${mcpModalField('mcp-m-command', t('config.mcp.field_command'),
            `<input id="mcp-m-command" class="field-input" value="${escapeAttr(data.command || '')}" placeholder="npx"${describe('mcp-m-command')}>`,
            t('config.mcp.field_command_help'), 'mcp-stdio-fields')}
        ${mcpModalField('mcp-m-url', t('config.mcp.field_url'),
            `<input id="mcp-m-url" class="field-input" value="${escapeAttr(data.url || '')}" placeholder="https://example.com/mcp"${describe('mcp-m-url')}>`,
            t('config.mcp.field_url_help'), 'mcp-network-fields')}
        ${mcpModalField('mcp-m-headers', t('config.mcp.field_headers') + mcpModalHint('KEY=VALUE, {{alias}}'),
            `<textarea id="mcp-m-headers" class="field-input" rows="4" placeholder="Authorization=Bearer {{remote-token}}"${describe('mcp-m-headers')}>${escapeAttr(headersStr)}</textarea>`,
            t('config.mcp.headers_hint'), 'mcp-network-fields')}
        ${mcpModalField('mcp-m-runtime', t('config.mcp.field_runtime'),
            `<select id="mcp-m-runtime" class="field-select"${describe('mcp-m-runtime')}>
                <option value="local" ${runtime === 'local' ? 'selected' : ''}>${t('config.mcp.runtime_local')}</option>
                <option value="docker" ${runtime === 'docker' ? 'selected' : ''}>${t('config.mcp.runtime_docker')}</option>
            </select>`,
            t('config.mcp.field_runtime_help'), 'mcp-stdio-fields')}
        ${mcpModalField('mcp-m-args', t('config.mcp.field_args_label') + mcpModalHint(t('config.mcp.args_hint')),
            `<textarea id="mcp-m-args" class="field-input" rows="3" placeholder="-y\n@my/mcp-server"${describe('mcp-m-args')}>${escapeAttr(argsStr)}</textarea>`,
            t('config.mcp.field_args_help'), 'mcp-stdio-fields')}
        ${mcpModalField('mcp-m-env', t('config.mcp.field_environment') + mcpModalHint('KEY=VALUE, ' + t('config.mcp.env_hint') + '; {{alias}}, {{workdir}}'),
            `<textarea id="mcp-m-env" class="field-input" rows="4" placeholder="API_KEY={{api-token}}\nBASE_PATH={{workdir}}"${describe('mcp-m-env')}>${escapeAttr(envStr)}</textarea>`,
            t('config.mcp.field_environment_secret_hint'), 'mcp-stdio-fields')}
        ${mcpModalField('mcp-m-allowed-tools', t('config.mcp.field_allowed_tools') + mcpModalHint(t('config.mcp.args_hint')),
            `<textarea id="mcp-m-allowed-tools" class="field-input" rows="3" placeholder="understand_image\ntext_to_audio"${describe('mcp-m-allowed-tools')}>${escapeAttr(allowedToolsStr)}</textarea>`,
            t('config.mcp.field_allowed_tools_hint'))}
        ${mcpModalField('mcp-m-docker-image', t('config.mcp.field_docker_image'),
            `<input id="mcp-m-docker-image" class="field-input" value="${escapeAttr(data.docker_image || '')}" placeholder="ghcr.io/astral-sh/uv:latest"${describe('mcp-m-docker-image')}>`,
            t('config.mcp.field_docker_image_help'), 'mcp-stdio-fields')}
        ${mcpModalField('mcp-m-docker-command', t('config.mcp.field_docker_command'),
            `<input id="mcp-m-docker-command" class="field-input" value="${escapeAttr(data.docker_command || '')}" placeholder="uvx"${describe('mcp-m-docker-command')}>`,
            t('config.mcp.field_docker_command_help'), 'mcp-stdio-fields')}
        ${mcpModalField('mcp-m-host-workdir', t('config.mcp.field_host_workdir'),
            `<input id="mcp-m-host-workdir" class="field-input" value="${escapeAttr(data.host_workdir || '')}" placeholder="agent_workspace/mcp/${escapeAttr(data.name || 'server')}"${describe('mcp-m-host-workdir')}>`,
            t('config.mcp.field_host_workdir_help'), 'mcp-stdio-fields')}
        ${mcpModalField('mcp-m-container-workdir', t('config.mcp.field_container_workdir'),
            `<input id="mcp-m-container-workdir" class="field-input" value="${escapeAttr(data.container_workdir || '/workspace')}" placeholder="/workspace"${describe('mcp-m-container-workdir')}>`,
            t('config.mcp.field_container_workdir_help'), 'mcp-stdio-fields')}
        ${mcpModalToggleRow('mcp-m-enabled', t('config.mcp.enabled_checkbox'), t('config.mcp.enabled_checkbox_help'), !!data.enabled)}
        ${mcpModalToggleRow('mcp-m-local-fallback', t('config.mcp.field_allow_local_fallback'), t('config.mcp.field_allow_local_fallback_help'), !!data.allow_local_fallback, 'mcp-stdio-fields')}
        ${mcpModalToggleRow('mcp-m-private-network', t('config.mcp.field_allow_private_network'), t('config.mcp.field_allow_private_network_help'), !!data.allow_private_network, 'mcp-network-fields')}
        ${mcpModalToggleRow('mcp-m-allow-destructive', t('config.mcp.field_allow_destructive'), t('config.mcp.field_allow_destructive_help'), !!data.allow_destructive, '', ' data-cfg-confirm-enable="destroy"')}
        ${cfgActionResult.html('mcp-m-test-result')}
        <div id="mcp-m-error" class="wh-notice mcp-notice-danger is-hidden" role="alert"></div>`;
    const actions = [
        `<button type="button" class="btn-secondary" onclick="closeModal('${MCP_SERVER_MODAL}')">${t('config.mcp.cancel')}</button>`,
        `<button type="button" class="btn-secondary" id="mcp-m-test">${t('config.mcp.test_connection')}</button>`,
        `<button type="button" class="btn-save" id="mcp-m-save">${t('config.mcp.save')}</button>`
    ];
    mcpOpenModal(MCP_SERVER_MODAL, isEdit ? t('config.mcp.edit_server') : t('config.mcp.new_server'), body, actions, '#mcp-m-name');
    document.getElementById('mcp-m-transport').addEventListener('change', mcpUpdateTransportFields);
    mcpUpdateTransportFields();

    document.getElementById('mcp-m-save').addEventListener('click', async () => {
        mcpModalClearError('mcp-m-error', MCP_SERVER_MODAL);
        const entry = mcpCollectServerModalEntry();
        if (!mcpValidateServerEntry(entry)) {
            return;
        }
        const saveBtn = document.getElementById('mcp-m-save');
        const showErr = msg => mcpModalShowError('mcp-m-error', msg);
        if (mcpServersCache === null) { showErr(t('config.mcp.load_failed_disabled')); return; }
        const next = mcpServersCache.slice();
        if (isEdit) {
            next[idx] = entry;
        } else {
            if (next.some(s => s.name === entry.name)) {
                mcpModalShowError('mcp-m-error', t('config.mcp.name_exists'), 'mcp-m-name');
                return;
            }
            next.push(entry);
        }
        saveBtn.disabled = true;
        const res = await mcpServerSave(next);
        saveBtn.disabled = false;
        if (!res.ok) {
            showErr(t('config.common.error') + ': ' + res.error);
            return;
        }
        closeModal(MCP_SERVER_MODAL);
        mcpServerRenderCards();
        mcpRuntimeToolsCache = {};
        await mcpRenderRoutingCards();
    });
    document.getElementById('mcp-m-test').addEventListener('click', async () => {
        mcpModalClearError('mcp-m-error', MCP_SERVER_MODAL);
        const entry = mcpCollectServerModalEntry();
        if (!mcpValidateServerEntry(entry)) {
            return;
        }
        await mcpTestServerConnection(entry, document.getElementById('mcp-m-test'));
    });
}

function mcpTransportLabel(transport) {
    const key = {
        stdio: 'config.mcp.transport_stdio',
        streamable_http: 'config.mcp.transport_streamable_http',
        sse: 'config.mcp.transport_sse',
        websocket: 'config.mcp.transport_websocket'
    }[transport || 'stdio'];
    return t(key || 'config.mcp.transport_stdio');
}

function mcpUpdateTransportFields() {
    const transportEl = document.getElementById('mcp-m-transport');
    if (!transportEl) return;
    const isStdio = transportEl.value === 'stdio';
    document.querySelectorAll('.mcp-stdio-fields').forEach(el => {
        el.classList.toggle('is-hidden', !isStdio);
    });
    document.querySelectorAll('.mcp-network-fields').forEach(el => {
        el.classList.toggle('is-hidden', isStdio);
    });
}

// KEY=VALUE per line. Blank lines are skipped; any other line without a key is reported, never dropped silently.
function mcpParseKeyValueLines(text) {
    const values = {};
    const invalid = [];
    String(text || '').split('\n').forEach((line, index) => {
        if (!line.trim()) return;
        const eq = line.indexOf('=');
        const key = eq > 0 ? line.substring(0, eq).trim() : '';
        if (!key) { invalid.push(index + 1); return; }
        values[key] = line.substring(eq + 1).trim();
    });
    return { values, invalid };
}

function mcpParseKeyValueTextarea(id) {
    const el = document.getElementById(id);
    return el ? mcpParseKeyValueLines(el.value) : { values: {}, invalid: [] };
}

function mcpCollectServerModalEntry() {
    const transport = document.getElementById('mcp-m-transport').value || 'stdio';
    return {
        name: document.getElementById('mcp-m-name').value.trim(),
        transport,
        url: document.getElementById('mcp-m-url').value.trim(),
        headers: mcpParseKeyValueTextarea('mcp-m-headers').values,
        command: document.getElementById('mcp-m-command').value.trim(),
        runtime: document.getElementById('mcp-m-runtime').value,
        args: document.getElementById('mcp-m-args').value.split('\n').map(l => l.trim()).filter(Boolean),
        env: mcpParseKeyValueTextarea('mcp-m-env').values,
        enabled: document.getElementById('mcp-m-enabled').classList.contains('on'),
        docker_image: document.getElementById('mcp-m-docker-image').value.trim(),
        docker_command: document.getElementById('mcp-m-docker-command').value.trim(),
        allow_local_fallback: document.getElementById('mcp-m-local-fallback').classList.contains('on'),
        allow_private_network: document.getElementById('mcp-m-private-network').classList.contains('on'),
        allowed_tools: document.getElementById('mcp-m-allowed-tools').value.split('\n').map(l => l.trim()).filter(Boolean),
        allow_destructive: document.getElementById('mcp-m-allow-destructive').classList.contains('on'),
        host_workdir: document.getElementById('mcp-m-host-workdir').value.trim(),
        container_workdir: document.getElementById('mcp-m-container-workdir').value.trim()
    };
}

// Shows the first problem inside the dialog (role=alert, aria-invalid on the field); true when the entry is usable.
function mcpValidateServerEntry(entry) {
    const hasCommand = entry.command || (entry.runtime === 'docker' && entry.docker_command);
    const stdio = entry.transport === 'stdio';
    if (!entry.name) {
        mcpModalShowError('mcp-m-error', t('config.mcp.name_command_or_url_required'), 'mcp-m-name');
        return false;
    }
    if (stdio && !hasCommand) {
        mcpModalShowError('mcp-m-error', t('config.mcp.name_command_or_url_required'), 'mcp-m-command');
        return false;
    }
    if (!stdio && !entry.url) {
        mcpModalShowError('mcp-m-error', t('config.mcp.name_command_or_url_required'), 'mcp-m-url');
        return false;
    }
    // Only the list that belongs to the chosen transport is checked; the other one is hidden.
    const listId = stdio ? 'mcp-m-env' : 'mcp-m-headers';
    const invalid = mcpParseKeyValueTextarea(listId).invalid;
    if (invalid.length) {
        const field = stdio ? t('config.mcp.field_environment') : t('config.mcp.field_headers');
        mcpModalShowError('mcp-m-error', t('config.mcp.kv_invalid', { field, lines: invalid.join(', ') }), listId);
        return false;
    }
    return true;
}

async function mcpTestServerConnection(entry, button) {
    return cfgActionResult(button, () => fetch('/api/mcp-runtime/test-connection', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(entry)
    }), {
        target: 'mcp-m-test-result',
        success: outcome => t('config.mcp.test_connection_success', { count: (outcome.body && outcome.body.tool_count) || 0 }),
        failure: t('config.mcp.test_connection_failed')
    });
}

function mcpSecretAdd() {
    if (mcpSecretsCache === null) { showToast(t('config.mcp.load_failed_disabled'), 'warn'); return; }
    mcpSecretShowModal({ alias: '', label: '', description: '', has_value: false }, '');
}

function mcpSecretEdit(alias) {
    if (mcpSecretsCache === null) return;
    const secret = (mcpSecretsCache || []).find(item => item.alias === alias);
    if (!secret) return;
    mcpSecretShowModal(secret, alias);
}

async function mcpSecretDelete(alias) {
    const secret = (mcpSecretsCache || []).find(item => item.alias === alias);
    if (!secret) return;
    const name = secret.label || secret.alias;
    const answer = await showConfirmDialog({
        title: t('config.common.delete_item', { name }),
        message: t('config.mcp.secret_delete_confirm', { name }),
        danger: true,
        confirmText: t('config.common.delete')
    });
    if (!answer.confirmed) return;
    if (mcpSecretsCache === null) return;
    const next = mcpSecretsCache.filter(item => item.alias !== alias);
    const res = await mcpSaveSecrets(next, {}, { [alias]: true });
    if (!res.ok) {
        showToast(t('config.mcp.secret_save_failed') + res.error, 'error');
        return;
    }
    showToast(t('config.mcp.secret_saved'), 'success');
    mcpSecretRenderCards();
}

function mcpSecretShowModal(data, originalAlias) {
    const describe = id => ` aria-describedby="${id}-help"`;
    const body = `
        ${mcpModalField('mcp-s-alias', t('config.mcp.secret_alias'),
            `<input id="mcp-s-alias" class="field-input" value="${escapeAttr(data.alias || '')}" placeholder="api-token"${describe('mcp-s-alias')}>`,
            t('config.mcp.secret_alias_help'))}
        ${mcpModalField('mcp-s-label', t('config.mcp.secret_label'),
            `<input id="mcp-s-label" class="field-input" value="${escapeAttr(data.label || '')}" placeholder="MiniMax API Token"${describe('mcp-s-label')}>`,
            t('config.mcp.secret_label_help'))}
        ${mcpModalField('mcp-s-description', t('config.mcp.secret_description'),
            `<input id="mcp-s-description" class="field-input" value="${escapeAttr(data.description || '')}" placeholder="${escapeAttr(t('config.mcp.secret_description_placeholder'))}"${describe('mcp-s-description')}>`,
            t('config.mcp.secret_description_help'))}
        ${mcpModalField('mcp-s-value', t('config.mcp.secret_value'),
            `<input id="mcp-s-value" type="password" class="field-input" value="" autocomplete="off" placeholder="${escapeAttr(data.has_value ? t('config.mcp.secret_value_placeholder_set') : t('config.mcp.secret_value_placeholder_empty'))}"${describe('mcp-s-value')}>`,
            t('config.mcp.secret_value_help'))}
        ${mcpModalToggleRow('mcp-s-clear', t('config.mcp.secret_clear_value'), t('config.mcp.secret_clear_value_help'), false)}
        <div id="mcp-s-error" class="wh-notice mcp-notice-danger is-hidden" role="alert"></div>`;
    const actions = [
        `<button type="button" class="btn-secondary" onclick="closeModal('${MCP_SECRET_MODAL}')">${t('config.mcp.cancel')}</button>`,
        `<button type="button" class="btn-save" id="mcp-s-save">${t('config.mcp.save')}</button>`
    ];
    mcpOpenModal(MCP_SECRET_MODAL, originalAlias ? t('config.mcp.edit_secret') : t('config.mcp.new_secret'), body, actions, '#mcp-s-alias');

    document.getElementById('mcp-s-save').addEventListener('click', async () => {
        const alias = document.getElementById('mcp-s-alias').value.trim().toLowerCase();
        const label = document.getElementById('mcp-s-label').value.trim();
        const description = document.getElementById('mcp-s-description').value.trim();
        const value = document.getElementById('mcp-s-value').value;
        const clearValue = document.getElementById('mcp-s-clear').classList.contains('on');
        const saveBtn = document.getElementById('mcp-s-save');
        const showErr = (msg, fieldId) => mcpModalShowError('mcp-s-error', msg, fieldId);
        mcpModalClearError('mcp-s-error', MCP_SECRET_MODAL);
        if (!alias) {
            showErr(t('config.mcp.secret_alias_required'), 'mcp-s-alias');
            return;
        }
        if (mcpSecretsCache === null) { showErr(t('config.mcp.load_failed_disabled')); return; }
        const next = mcpSecretsCache.filter(item => item.alias !== originalAlias && item.alias !== alias);
        next.push({
            alias,
            label,
            description,
            has_value: clearValue ? false : !!(value || data.has_value)
        });
        next.sort((a, b) => a.alias.localeCompare(b.alias));

        const deleteFlags = {};
        if (originalAlias && originalAlias !== alias) {
            deleteFlags[originalAlias] = true;
        }
        if (clearValue) {
            deleteFlags[alias] = true;
        }
        saveBtn.disabled = true;
        const res = await mcpSaveSecrets(next, value ? { [alias]: value } : {}, deleteFlags);
        saveBtn.disabled = false;
        if (!res.ok) {
            showErr(t('config.mcp.secret_save_failed') + res.error);
            return;
        }
        showToast(t('config.mcp.secret_saved'), 'success');
        closeModal(MCP_SECRET_MODAL);
        mcpSecretRenderCards();
    });
}

async function mcpServerSave(nextServers) {
    if (mcpServersCache === null || !Array.isArray(nextServers)) {
        return { ok: false, error: t('config.mcp.load_failed_disabled') };
    }
    try {
        const resp = await fetch('/api/mcp-servers', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                enabled: !!(configData.mcp && configData.mcp.enabled),
                servers: nextServers
            })
        });
        if (!resp.ok) throw new Error((await resp.text()).trim() || resp.statusText);
        mcpServersCache = nextServers;
        try {
            const reload = await fetch('/api/mcp-servers');
            if (reload.ok) mcpServersCache = await reload.json();
        } catch (_) { /* keep saved list */ }
        return { ok: true };
    } catch (e) {
        return { ok: false, error: e.message || String(e) };
    }
}

async function mcpSaveSecrets(nextSecrets, valuesByAlias = {}, deleteFlags = {}) {
    if (mcpSecretsCache === null || !Array.isArray(nextSecrets)) {
        return { ok: false, error: t('config.mcp.load_failed_disabled') };
    }
    try {
        const resp = await fetch('/api/mcp-secrets', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                secrets: nextSecrets.map(secret => ({
                    alias: secret.alias,
                    label: secret.label || '',
                    description: secret.description || '',
                    value: valuesByAlias[secret.alias] || '',
                    delete_value: !!deleteFlags[secret.alias]
                }))
            })
        });
        if (!resp.ok) throw new Error((await resp.text()).trim() || resp.statusText);
        const data = await resp.json();
        mcpSecretsCache = Array.isArray(data.secrets) ? data.secrets : nextSecrets;
        return { ok: true };
    } catch (e) {
        return { ok: false, error: e.message || String(e) };
    }
}

async function mcpSavePreferences() {
    if (mcpPreferencesCache === null) return;
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
