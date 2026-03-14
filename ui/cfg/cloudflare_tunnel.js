// cfg/cloudflare_tunnel.js — Cloudflare Tunnel section module

async function renderCloudflareTunnelSection(section) {
    const cfg = configData.cloudflare_tunnel || {};
    const enabled = cfg.enabled === true;
    const readOnly = cfg.readonly === true;
    const autoStart = cfg.auto_start !== false;
    const exposeWebUI = cfg.expose_web_ui !== false;
    const exposeHomepage = cfg.expose_homepage !== false;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // Info banner
    html += `<div class="wh-notice" style="border-color:var(--accent);background:rgba(99,102,241,0.06);">
        <span>🔒</span>
        <div><small>${t('config.cloudflare_tunnel.info')}</small></div>
    </div>`;

    // Enabled toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="cloudflare_tunnel.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🌐</span>
            <div>
                <strong>${t('config.cloudflare_tunnel.disabled_notice')}</strong><br>
                <small>${t('config.cloudflare_tunnel.disabled_desc')}</small>
            </div>
        </div>`;
    }

    // Read-only toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.readonly_label')}</span>
        <div class="toggle ${readOnly ? 'on' : ''}" data-path="cloudflare_tunnel.readonly" onclick="toggleBool(this)"></div>
    </div>`;

    // Auto-start toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.auto_start_label')}</span>
        <div class="toggle ${autoStart ? 'on' : ''}" data-path="cloudflare_tunnel.auto_start" onclick="toggleBool(this)"></div>
    </div>`;

    // Config fields grid
    html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem 1.2rem;margin-top:1rem;">`;

    // Mode
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.mode')}</span>
        <select class="cfg-input" data-path="cloudflare_tunnel.mode" style="width:100%;margin-top:0.2rem;" onchange="setNestedValue(configData,'cloudflare_tunnel.mode',this.value);setDirty(true)">
            <option value="auto" ${cfg.mode === 'auto' || !cfg.mode ? 'selected' : ''}>Auto (Docker → Native)</option>
            <option value="docker" ${cfg.mode === 'docker' ? 'selected' : ''}>Docker</option>
            <option value="native" ${cfg.mode === 'native' ? 'selected' : ''}>Native Binary</option>
        </select>
    </label>`;

    // Auth Method
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.auth_method')}</span>
        <select class="cfg-input" data-path="cloudflare_tunnel.auth_method" style="width:100%;margin-top:0.2rem;" onchange="setNestedValue(configData,'cloudflare_tunnel.auth_method',this.value);setDirty(true)">
            <option value="token" ${cfg.auth_method === 'token' || !cfg.auth_method ? 'selected' : ''}>Connector Token</option>
            <option value="named" ${cfg.auth_method === 'named' ? 'selected' : ''}>Named Tunnel</option>
            <option value="quick" ${cfg.auth_method === 'quick' ? 'selected' : ''}>Quick Tunnel (TryCloudflare)</option>
        </select>
    </label>`;

    // Tunnel Name (for named tunnel)
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.tunnel_name')}</span>
        <input class="cfg-input" data-path="cloudflare_tunnel.tunnel_name" value="${escapeAttr(cfg.tunnel_name || '')}" style="width:100%;margin-top:0.2rem;"
            placeholder="my-tunnel" onchange="setNestedValue(configData,'cloudflare_tunnel.tunnel_name',this.value);setDirty(true)">
    </label>`;

    // Account ID
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.account_id')}</span>
        <input class="cfg-input" data-path="cloudflare_tunnel.account_id" value="${escapeAttr(cfg.account_id || '')}" style="width:100%;margin-top:0.2rem;"
            placeholder="optional" onchange="setNestedValue(configData,'cloudflare_tunnel.account_id',this.value);setDirty(true)">
    </label>`;

    // Metrics Port
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.metrics_port')}</span>
        <input class="cfg-input" data-path="cloudflare_tunnel.metrics_port" type="number" value="${cfg.metrics_port || 0}" style="width:100%;margin-top:0.2rem;"
            placeholder="0 = disabled" onchange="setNestedValue(configData,'cloudflare_tunnel.metrics_port',parseInt(this.value)||0);setDirty(true)">
    </label>`;

    // Log Level
    html += `<label style="display:block;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.log_level')}</span>
        <select class="cfg-input" data-path="cloudflare_tunnel.log_level" style="width:100%;margin-top:0.2rem;" onchange="setNestedValue(configData,'cloudflare_tunnel.log_level',this.value);setDirty(true)">
            <option value="info" ${cfg.log_level === 'info' || !cfg.log_level ? 'selected' : ''}>Info</option>
            <option value="debug" ${cfg.log_level === 'debug' ? 'selected' : ''}>Debug</option>
            <option value="warn" ${cfg.log_level === 'warn' ? 'selected' : ''}>Warn</option>
            <option value="error" ${cfg.log_level === 'error' ? 'selected' : ''}>Error</option>
        </select>
    </label>`;

    html += `</div>`; // close grid

    // Exposure toggles
    html += `<div style="margin-top:1.2rem;">
        <span style="font-size:0.85rem;font-weight:600;color:var(--text-primary);">${t('config.cloudflare_tunnel.exposure_heading')}</span>
    </div>`;

    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin:0.6rem 0;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.expose_web_ui')}</span>
        <div class="toggle ${exposeWebUI ? 'on' : ''}" data-path="cloudflare_tunnel.expose_web_ui" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.cloudflare_tunnel.expose_homepage')}</span>
        <div class="toggle ${exposeHomepage ? 'on' : ''}" data-path="cloudflare_tunnel.expose_homepage" onclick="toggleBool(this)"></div>
    </div>`;

    // Auth hint for token mode
    if ((cfg.auth_method === 'token' || !cfg.auth_method) && enabled) {
        html += `<div class="wh-notice" style="border-color:var(--warning);background:rgba(234,179,8,0.06);">
            <span>🔑</span>
            <div><small>${t('config.cloudflare_tunnel.token_hint')}</small></div>
        </div>`;
    }

    // Auth hint for named tunnel
    if (cfg.auth_method === 'named' && enabled) {
        html += `<div class="wh-notice" style="border-color:var(--warning);background:rgba(234,179,8,0.06);">
            <span>📄</span>
            <div><small>${t('config.cloudflare_tunnel.named_hint')}</small></div>
        </div>`;
    }

    html += `</div>`; // close section
    document.getElementById('content').innerHTML = html;
}
