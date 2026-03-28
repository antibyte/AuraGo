// cfg/mcp_server.js — Built-in MCP Server section module

const MCP_VSCODE_BRIDGE_PRESET = [
    'ask_aurago',
    'filesystem',
    'smart_file_read',
    'execute_shell',
    'api_request',
    'query_memory',
    'context_memory',
    'homepage',
    'netlify',
    'web_capture'
];

async function renderMCPServerSection(section) {
    const cfg = configData.mcp_server || {};
    const enabled = cfg.enabled === true;
    const vscodeBridge = cfg.vscode_debug_bridge === true;
    const requireAuth = cfg.require_auth === true;
    const effectiveEnabled = enabled || vscodeBridge;
    const effectiveRequireAuth = requireAuth || vscodeBridge;
    const allowedTools = Array.isArray(cfg.allowed_tools) ? cfg.allowed_tools : [];
    const effectiveAllowedTools = vscodeBridge
        ? Array.from(new Set([...allowedTools, ...MCP_VSCODE_BRIDGE_PRESET]))
        : allowedTools;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // Info banner
    html += `<div class="wh-notice mcp-srv-notice-info">
        <span>🔗</span>
        <div><small>${t('config.mcp_server.info')}</small></div>
    </div>`;

    // Enabled toggle
    html += `<div class="mcp-srv-toggle-row">
        <span class="mcp-srv-toggle-label">${t('config.mcp_server.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="mcp_server.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="mcp-srv-toggle-row">
        <span class="mcp-srv-toggle-label">${t('config.mcp_server.vscode_bridge')}</span>
        <div class="toggle ${vscodeBridge ? 'on' : ''}" onclick="mcpToggleVSCodeBridge(this)"></div>
    </div>
    <div class="mcp-srv-tools-desc">${t('config.mcp_server.vscode_bridge_desc')}</div>`;

    if (vscodeBridge) {
        html += `<div class="wh-notice mcp-srv-notice-info">
            <span>🧪</span>
            <div>
                <small>${t('config.mcp_server.vscode_bridge_notice')}</small>
            </div>
        </div>`;
    }

    if (!effectiveEnabled) {
        html += `<div class="wh-notice">
            <span>🔌</span>
            <div>
                <strong>${t('config.mcp_server.disabled_notice')}</strong><br>
                <small>${t('config.mcp_server.disabled_desc')}</small>
            </div>
        </div>`;
    }

    // Require Auth toggle
    html += `<div class="mcp-srv-toggle-row">
        <span class="mcp-srv-toggle-label">${t('config.mcp_server.require_auth')}</span>
        <div class="toggle ${effectiveRequireAuth ? 'on' : ''}" data-path="mcp_server.require_auth" onclick="toggleBool(this)"></div>
    </div>`;

    // Auth warning when disabled
    if (!effectiveRequireAuth) {
        html += `<div class="wh-notice mcp-srv-notice-warn">
            <span>⚠️</span>
            <div><small>${t('config.mcp_server.auth_warning')}</small></div>
        </div>`;
    }

    // Endpoint URL (read-only display)
    if (effectiveEnabled) {
        const proto = location.protocol;
        const host = location.host;
        const endpointUrl = proto + '//' + host + '/mcp';
        html += `<div class="mcp-srv-block">
            <label class="mcp-srv-field-label">
                <span class="mcp-srv-caption">${t('config.mcp_server.endpoint_url')}</span>
                <div class="mcp-srv-action-row">
                    <input class="cfg-input mcp-srv-endpoint-input" value="${escapeAttr(endpointUrl)}" readonly id="mcp-endpoint-url">
                    <button class="btn" onclick="navigator.clipboard.writeText(document.getElementById('mcp-endpoint-url').value).then(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copied'))}';setTimeout(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copy_url'))}'},1500)})">${t('config.mcp_server.copy_url')}</button>
                </div>
            </label>
        </div>`;
    }

    // Token section (only when auth enabled)
    if (effectiveRequireAuth && effectiveEnabled) {
        html += `<div class="mcp-srv-block" id="mcp-token-section">
            <label class="mcp-srv-field-label">
                <span class="mcp-srv-caption">${t('config.mcp_server.token')}</span>
                <div class="mcp-srv-action-row">
                    <input class="cfg-input mcp-srv-token-input" id="mcp-token-value" value="" readonly placeholder="••••••••">
                    <button class="btn" id="mcp-gen-token" onclick="mcpGenerateToken()">${t('config.mcp_server.generate_token')}</button>
                    <button class="btn" id="mcp-copy-token" onclick="navigator.clipboard.writeText(document.getElementById('mcp-token-value').value).then(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copied'))}';setTimeout(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copy_token'))}'},1500)})">${t('config.mcp_server.copy_token')}</button>
                </div>
            </label>
        </div>`;
        // Load existing token
        mcpLoadToken();
    }

    if (vscodeBridge) {
        html += `<div class="mcp-srv-block">
            <label class="mcp-srv-field-label">
                <span class="mcp-srv-caption">${t('config.mcp_server.vscode_config')}</span>
                <div class="mcp-srv-tools-desc">${t('config.mcp_server.vscode_config_desc')}</div>
                <div id="mcp-vscode-bridge-config" class="mcp-srv-tools-list"></div>
            </label>
        </div>`;
    }

    // Allowed tools
    html += `<div class="mcp-srv-tools-wrap">
        <span class="mcp-srv-tools-title">${t('config.mcp_server.allowed_tools')}</span>
        <div class="mcp-srv-tools-desc">${t('config.mcp_server.allowed_tools_desc')}</div>
        <div id="mcp-tools-list" class="mcp-srv-tools-list"></div>
    </div>`;

    html += `</div>`; // close section
    document.getElementById('content').innerHTML = html;

    // Fetch available tools from backend and render checkboxes
    mcpLoadToolList(effectiveAllowedTools);
    if (vscodeBridge) {
        mcpLoadVSCodeBridgeInfo();
    }
}

async function mcpLoadToolList(allowed) {
    const container = document.getElementById('mcp-tools-list');
    if (!container) return;
    try {
        const resp = await fetch('/api/mcp-server/tools');
        if (!resp.ok) throw new Error('Failed to load tools');
        const toolNames = await resp.json();
        const bridgeEnabled = configData?.mcp_server?.vscode_debug_bridge === true;
        const allToolNames = bridgeEnabled
            ? Array.from(new Set([...(toolNames || []), ...MCP_VSCODE_BRIDGE_PRESET]))
            : (toolNames || []);
        if (!allToolNames || allToolNames.length === 0) {
            container.innerHTML = `<div class="wh-notice"><span>⚠️</span><div><small>${t('config.mcp_server.no_tools_warning')}</small></div></div>`;
            return;
        }
        const allowSet = new Set(allowed);
        let listHtml = '';
        for (const name of allToolNames) {
            const checked = allowSet.size === 0 || allowSet.has(name) ? 'checked' : '';
            listHtml += `<label class="mcp-srv-tool-item">
                <input type="checkbox" class="mcp-tool-cb" value="${escapeAttr(name)}" ${checked} onchange="mcpUpdateAllowedTools()">
                <code class="mcp-srv-tool-code">${escapeAttr(name)}</code>
            </label>`;
        }
        container.innerHTML = listHtml;
    } catch (e) {
        container.innerHTML = '<span class="mcp-srv-error">Error loading tools</span>';
    }
}

function mcpUpdateAllowedTools() {
    const checkboxes = document.querySelectorAll('.mcp-tool-cb');
    const allChecked = Array.from(checkboxes).every(cb => cb.checked);
    const bridgeEnabled = configData?.mcp_server?.vscode_debug_bridge === true;
    const selected = (allChecked && !bridgeEnabled)
        ? []
        : Array.from(checkboxes).filter(cb => cb.checked).map(cb => cb.value);
    setNestedValue(configData, 'mcp_server.allowed_tools', selected);
    setDirty(true);
}

async function mcpGenerateToken() {
    try {
        const resp = await fetch('/api/mcp-server/token', { method: 'POST' });
        if (!resp.ok) throw new Error('Failed to generate token');
        const data = await resp.json();
        const input = document.getElementById('mcp-token-value');
        if (input && data.token) {
            input.value = data.token;
        }
    } catch (e) {
        console.error('Token generation failed:', e);
    }
}

async function mcpLoadToken() {
    try {
        const resp = await fetch('/api/mcp-server/token');
        if (!resp.ok) return;
        const data = await resp.json();
        const input = document.getElementById('mcp-token-value');
        if (input && data.token) {
            input.value = data.token;
        }
    } catch (_) {}
}

function mcpToggleVSCodeBridge(toggleEl) {
    const willEnable = !toggleEl.classList.contains('on');
    setNestedValue(configData, 'mcp_server.vscode_debug_bridge', willEnable);

    if (willEnable) {
        setNestedValue(configData, 'mcp_server.enabled', true);
        setNestedValue(configData, 'mcp_server.require_auth', true);
        const current = Array.isArray(configData?.mcp_server?.allowed_tools) ? configData.mcp_server.allowed_tools : [];
        setNestedValue(configData, 'mcp_server.allowed_tools', Array.from(new Set([...current, ...MCP_VSCODE_BRIDGE_PRESET])));
        const tokenInput = document.getElementById('mcp-token-value');
        if (!tokenInput || !tokenInput.value) {
            mcpGenerateToken().catch(() => {});
        }
    }

    setDirty(true);
    selectSection('mcp_server', { scrollBehavior: 'auto' });
}

async function mcpLoadVSCodeBridgeInfo() {
    const container = document.getElementById('mcp-vscode-bridge-config');
    if (!container) return;
    try {
        const resp = await fetch('/api/mcp-server/vscode-bridge');
        if (!resp.ok) throw new Error('Failed to load VS Code bridge info');
        const data = await resp.json();
        const tools = Array.isArray(data.recommended_tools) ? data.recommended_tools : [];
        const clients = data.clients || {};
        const clientOptions = [
            { key: 'vscode', label: clients.vscode?.label || 'VS Code' },
            { key: 'cursor', label: clients.cursor?.label || 'Cursor' },
            { key: 'claude_desktop', label: clients.claude_desktop?.label || 'Claude Desktop' }
        ];
        const toolBadges = tools.map(name => `<code class="mcp-srv-tool-code">${esc(name)}</code>`).join(' ');
        const tokenHint = data.token_present
            ? ''
            : `<div class="wh-notice mcp-srv-notice-warn"><span>🔑</span><div><small>${t('config.mcp_server.vscode_token_hint')}</small></div></div>`;
        container.innerHTML = `
            ${tokenHint}
            <div class="mcp-srv-tools-desc">${t('config.mcp_server.vscode_bridge_tools')}</div>
            <div class="mcp-srv-tools-desc" style="display:flex; gap:.4rem; flex-wrap:wrap; margin:.5rem 0 1rem 0;">${toolBadges}</div>
            <div class="mcp-srv-tools-desc" style="margin-bottom:.5rem;">${t('config.mcp_server.client_selector')}</div>
            <div class="mcp-srv-action-row" style="margin-bottom:.75rem; flex-wrap:wrap;">
                ${clientOptions.map(client => `<button class="btn mcp-client-btn" data-client="${client.key}" onclick="mcpSelectClientConfig('${client.key}', this)">${esc(client.label)}</button>`).join('')}
            </div>
            <textarea class="cfg-input mcp-srv-endpoint-input" id="mcp-vscode-config" readonly style="min-height: 220px; font-family: var(--font-mono, monospace); white-space: pre;"></textarea>
            <div class="mcp-srv-action-row" style="margin-top:.75rem;">
                <button class="btn" onclick="mcpCopyVSCodeConfig(this)">${t('config.mcp_server.copy_vscode_config')}</button>
                <button class="btn" id="mcp-client-link-btn" style="display:none;" onclick="mcpOpenClientInstallLink()">${t('config.mcp_server.open_client_link')}</button>
            </div>`;
        window.__auragoMcpClientConfigs = clients;
        const firstBtn = container.querySelector('.mcp-client-btn');
        mcpSelectClientConfig('vscode', firstBtn);
    } catch (e) {
        container.innerHTML = `<span class="mcp-srv-error">${esc(e.message || 'Failed to load VS Code bridge info')}</span>`;
    }
}

function mcpCopyVSCodeConfig(btn) {
    const input = document.getElementById('mcp-vscode-config');
    if (!input) return;
    navigator.clipboard.writeText(input.value).then(() => {
        const original = btn.textContent;
        btn.textContent = t('config.mcp_server.copied');
        setTimeout(() => { btn.textContent = original; }, 1500);
    });
}

function mcpSelectClientConfig(clientKey, btn) {
    const clients = window.__auragoMcpClientConfigs || {};
    const client = clients[clientKey] || {};
    const input = document.getElementById('mcp-vscode-config');
    if (input) {
        input.value = client.config || '';
    }
    document.querySelectorAll('.mcp-client-btn').forEach(el => el.classList.toggle('primary', el === btn || el.dataset.client === clientKey));
    const linkBtn = document.getElementById('mcp-client-link-btn');
    if (linkBtn) {
        const hasLink = !!client.install_link;
        linkBtn.style.display = hasLink ? '' : 'none';
        linkBtn.dataset.href = client.install_link || '';
    }
}

function mcpOpenClientInstallLink() {
    const linkBtn = document.getElementById('mcp-client-link-btn');
    const href = linkBtn?.dataset?.href;
    if (href) {
        window.open(href, '_blank');
    }
}
