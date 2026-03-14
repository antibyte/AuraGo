// cfg/mcp_server.js — Built-in MCP Server section module

async function renderMCPServerSection(section) {
    const cfg = configData.mcp_server || {};
    const enabled = cfg.enabled === true;
    const requireAuth = cfg.require_auth === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // Info banner
    html += `<div class="wh-notice" style="border-color:var(--accent);background:rgba(99,102,241,0.06);">
        <span>🔗</span>
        <div><small>${t('config.mcp_server.info')}</small></div>
    </div>`;

    // Enabled toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.mcp_server.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="mcp_server.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🔌</span>
            <div>
                <strong>${t('config.mcp_server.disabled_notice')}</strong><br>
                <small>${t('config.mcp_server.disabled_desc')}</small>
            </div>
        </div>`;
    }

    // Require Auth toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.mcp_server.require_auth')}</span>
        <div class="toggle ${requireAuth ? 'on' : ''}" data-path="mcp_server.require_auth" onclick="toggleBool(this)"></div>
    </div>`;

    // Auth warning when disabled
    if (!requireAuth) {
        html += `<div class="wh-notice" style="border-color:var(--warning);background:rgba(234,179,8,0.06);">
            <span>⚠️</span>
            <div><small>${t('config.mcp_server.auth_warning')}</small></div>
        </div>`;
    }

    // Endpoint URL (read-only display)
    if (enabled) {
        const proto = location.protocol;
        const host = location.host;
        const endpointUrl = proto + '//' + host + '/mcp';
        html += `<div style="margin:1rem 0;">
            <label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.mcp_server.endpoint_url')}</span>
                <div style="display:flex;gap:0.5rem;margin-top:0.2rem;">
                    <input class="cfg-input" value="${escapeAttr(endpointUrl)}" readonly style="flex:1;opacity:0.8;cursor:default;" id="mcp-endpoint-url">
                    <button class="btn" onclick="navigator.clipboard.writeText(document.getElementById('mcp-endpoint-url').value).then(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copied'))}';setTimeout(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copy_url'))}'},1500)})">${t('config.mcp_server.copy_url')}</button>
                </div>
            </label>
        </div>`;
    }

    // Token section (only when auth enabled)
    if (requireAuth && enabled) {
        html += `<div style="margin:1rem 0;" id="mcp-token-section">
            <label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.mcp_server.token')}</span>
                <div style="display:flex;gap:0.5rem;margin-top:0.2rem;">
                    <input class="cfg-input" id="mcp-token-value" value="" readonly style="flex:1;font-family:monospace;font-size:0.8rem;" placeholder="••••••••">
                    <button class="btn" id="mcp-gen-token" onclick="mcpGenerateToken()">${t('config.mcp_server.generate_token')}</button>
                    <button class="btn" id="mcp-copy-token" onclick="navigator.clipboard.writeText(document.getElementById('mcp-token-value').value).then(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copied'))}';setTimeout(()=>{this.textContent='${escapeAttr(t('config.mcp_server.copy_token'))}'},1500)})">${t('config.mcp_server.copy_token')}</button>
                </div>
            </label>
        </div>`;
        // Load existing token
        mcpLoadToken();
    }

    // Allowed tools
    html += `<div style="margin-top:1.2rem;">
        <span style="font-size:0.85rem;font-weight:600;color:var(--text-primary);">${t('config.mcp_server.allowed_tools')}</span>
        <div style="font-size:0.75rem;color:var(--text-secondary);margin-bottom:0.6rem;">${t('config.mcp_server.allowed_tools_desc')}</div>
        <div id="mcp-tools-list" style="display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:0.4rem;"></div>
    </div>`;

    html += `</div>`; // close section
    document.getElementById('content').innerHTML = html;

    // Fetch available tools from backend and render checkboxes
    mcpLoadToolList(cfg.allowed_tools || []);
}

async function mcpLoadToolList(allowed) {
    const container = document.getElementById('mcp-tools-list');
    if (!container) return;
    try {
        const resp = await fetch('/api/mcp-server/tools');
        if (!resp.ok) throw new Error('Failed to load tools');
        const toolNames = await resp.json();
        if (!toolNames || toolNames.length === 0) {
            container.innerHTML = `<div class="wh-notice"><span>⚠️</span><div><small>${t('config.mcp_server.no_tools_warning')}</small></div></div>`;
            return;
        }
        const allowSet = new Set(allowed);
        let listHtml = '';
        for (const name of toolNames) {
            const checked = allowSet.size === 0 || allowSet.has(name) ? 'checked' : '';
            listHtml += `<label style="display:flex;align-items:center;gap:0.4rem;font-size:0.8rem;padding:0.3rem 0.5rem;border-radius:6px;background:var(--bg-tertiary);cursor:pointer;">
                <input type="checkbox" class="mcp-tool-cb" value="${escapeAttr(name)}" ${checked} onchange="mcpUpdateAllowedTools()">
                <code style="font-size:0.75rem;">${escapeAttr(name)}</code>
            </label>`;
        }
        container.innerHTML = listHtml;
    } catch (e) {
        container.innerHTML = '<span style="color:var(--danger);font-size:0.8rem;">Error loading tools</span>';
    }
}

function mcpUpdateAllowedTools() {
    const checkboxes = document.querySelectorAll('.mcp-tool-cb');
    const allChecked = Array.from(checkboxes).every(cb => cb.checked);
    const selected = allChecked ? [] : Array.from(checkboxes).filter(cb => cb.checked).map(cb => cb.value);
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
