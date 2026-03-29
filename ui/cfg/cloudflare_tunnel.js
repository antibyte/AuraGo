// cfg/cloudflare_tunnel.js — Cloudflare Tunnel section module

async function renderCloudflareTunnelSection(section) {
    window._cfSectionMeta = section;
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
    html += `<div class="wh-notice cft-notice-info">
        <span>🔒</span>
        <div><small>${t('config.cloudflare_tunnel.info')}</small></div>
    </div>`;

    // Enabled toggle
    html += `<div class="cft-toggle-row">
        <span class="cft-toggle-label">${t('config.cloudflare_tunnel.enabled_label')}</span>
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
    html += `<div class="cft-toggle-row">
        <span class="cft-toggle-label">${t('config.cloudflare_tunnel.readonly_label')}</span>
        <div class="toggle ${readOnly ? 'on' : ''}" data-path="cloudflare_tunnel.readonly" onclick="toggleBool(this)"></div>
    </div>`;

    // Auto-start toggle
    html += `<div class="cft-toggle-row">
        <span class="cft-toggle-label">${t('config.cloudflare_tunnel.auto_start_label')}</span>
        <div class="toggle ${autoStart ? 'on' : ''}" data-path="cloudflare_tunnel.auto_start" onclick="toggleBool(this)"></div>
    </div>`;

    // Config fields grid
    html += `<div class="cft-grid">`;

    // Mode
    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.mode')}</span>
        <select class="cfg-input cft-field-input" data-path="cloudflare_tunnel.mode" onchange="setNestedValue(configData,'cloudflare_tunnel.mode',this.value);setDirty(true)">
            <option value="auto" ${cfg.mode === 'auto' || !cfg.mode ? 'selected' : ''}>Auto (Docker → Native)</option>
            <option value="docker" ${cfg.mode === 'docker' ? 'selected' : ''}>Docker</option>
            <option value="native" ${cfg.mode === 'native' ? 'selected' : ''}>Native Binary</option>
        </select>
    </label>`;

    // Auth Method
    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.auth_method')}</span>
        <select class="cfg-input cft-field-input" data-path="cloudflare_tunnel.auth_method" onchange="setNestedValue(configData,'cloudflare_tunnel.auth_method',this.value);setDirty(true)">
            <option value="token" ${cfg.auth_method === 'token' || !cfg.auth_method ? 'selected' : ''}>Connector Token</option>
            <option value="named" ${cfg.auth_method === 'named' ? 'selected' : ''}>Named Tunnel</option>
            <option value="quick" ${cfg.auth_method === 'quick' ? 'selected' : ''}>Quick Tunnel (TryCloudflare)</option>
        </select>
    </label>`;

    // Tunnel Name (for named tunnel)
    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.tunnel_name')}</span>
        <input class="cfg-input cft-field-input" data-path="cloudflare_tunnel.tunnel_name" value="${escapeAttr(cfg.tunnel_name || '')}"
            placeholder="my-tunnel" onchange="setNestedValue(configData,'cloudflare_tunnel.tunnel_name',this.value);setDirty(true)">
    </label>`;

    // Account ID
    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.account_id')}</span>
        <input class="cfg-input cft-field-input" data-path="cloudflare_tunnel.account_id" value="${escapeAttr(cfg.account_id || '')}"
            placeholder="optional" onchange="setNestedValue(configData,'cloudflare_tunnel.account_id',this.value);setDirty(true)">
    </label>`;

    // Metrics Port
    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.metrics_port')}</span>
        <input class="cfg-input cft-field-input" data-path="cloudflare_tunnel.metrics_port" type="number" value="${cfg.metrics_port || 0}"
            placeholder="0 = disabled" onchange="setNestedValue(configData,'cloudflare_tunnel.metrics_port',parseInt(this.value)||0);setDirty(true)">
    </label>`;

    // Log Level
    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.log_level')}</span>
        <select class="cfg-input cft-field-input" data-path="cloudflare_tunnel.log_level" onchange="setNestedValue(configData,'cloudflare_tunnel.log_level',this.value);setDirty(true)">
            <option value="info" ${cfg.log_level === 'info' || !cfg.log_level ? 'selected' : ''}>Info</option>
            <option value="debug" ${cfg.log_level === 'debug' ? 'selected' : ''}>Debug</option>
            <option value="warn" ${cfg.log_level === 'warn' ? 'selected' : ''}>Warn</option>
            <option value="error" ${cfg.log_level === 'error' ? 'selected' : ''}>Error</option>
        </select>
    </label>`;

    html += `</div>`; // close grid

    // Loopback HTTP port toggle — only relevant when HTTPS is active
    const httpsEnabled = (configData.server?.https?.enabled === true);
    if (httpsEnabled) {
        const loopbackEnabled = (cfg.loopback_port !== undefined && cfg.loopback_port > 0);
        const loopbackPortVal = loopbackEnabled ? cfg.loopback_port : 18080;
        html += `<div class="cft-toggle-row" style="margin-top:0.6rem;">
            <span class="cft-toggle-label">${t('config.cloudflare_tunnel.loopback_label')}</span>
            <div class="toggle ${loopbackEnabled ? 'on' : ''}" onclick="cloudflareTunnelToggleLoopback(this)"></div>
        </div>`;
        if (loopbackEnabled) {
            html += `<div style="font-size:0.78rem;color:var(--text-muted);margin:0.1rem 0 0.3rem 0.2rem;display:flex;align-items:center;gap:0.4rem;">
                → http://127.0.0.1:<input type="number" min="1024" max="65535" value="${loopbackPortVal}"
                    data-path="cloudflare_tunnel.loopback_port"
                    style="width:5.5rem;font-size:0.78rem;padding:1px 4px;border:1px solid var(--border);border-radius:3px;background:var(--input-bg);color:var(--text);"
                    onchange="cloudflareTunnelChangeLoopbackPort(this.value)">
            </div>`;
        } else {
            // Hidden input so buildConfigPatchFromForm() always sends loopback_port=0 when disabled.
            html += `<input type="hidden" data-path="cloudflare_tunnel.loopback_port" value="0">`;
        }
        html += `<div class="wh-notice cft-notice-info" style="margin-top:0.3rem;">
            <span>🔄</span>
            <div><small>${t('config.cloudflare_tunnel.loopback_hint')}</small></div>
        </div>`;
    }
    html += `<div class="cft-exposure-heading-wrap">
        <span class="cft-exposure-heading">${t('config.cloudflare_tunnel.exposure_heading')}</span>
    </div>`;

    const isNamed = cfg.auth_method === 'named';
    if (isNamed) {
        // Named tunnel: multiple ingress rules supported — both services can be exposed independently
        html += `<div class="cft-toggle-row cft-toggle-row-exposure">
            <span class="cft-toggle-label">${t('config.cloudflare_tunnel.expose_web_ui')}</span>
            <div class="toggle ${exposeWebUI ? 'on' : ''}" data-path="cloudflare_tunnel.expose_web_ui" onclick="toggleBool(this)"></div>
        </div>`;
        html += `<div class="cft-toggle-row">
            <span class="cft-toggle-label">${t('config.cloudflare_tunnel.expose_homepage')}</span>
            <div class="toggle ${exposeHomepage ? 'on' : ''}" data-path="cloudflare_tunnel.expose_homepage" onclick="toggleBool(this)"></div>
        </div>`;
        html += `<div class="wh-notice cft-notice-info" style="margin-top:0.5rem;">
            <span>ℹ️</span>
            <div><small>${t('config.cloudflare_tunnel.expose_named_hint')}</small></div>
        </div>`;
    } else {
        // Token / Quick tunnel: single origin URL (--url) — only one target allowed
        const exposeTarget = (exposeHomepage && !exposeWebUI) ? 'homepage' : 'web_ui';
        html += `<label class="cft-field-label" style="max-width:420px;margin-top:0.4rem;">
            <span class="cft-field-caption">${t('config.cloudflare_tunnel.expose_target_label')}</span>
            <select class="cfg-input cft-field-input" onchange="cloudflareTunnelSetExposeTarget(this.value)">
                <option value="web_ui" ${exposeTarget === 'web_ui' ? 'selected' : ''}>${t('config.cloudflare_tunnel.expose_web_ui')}</option>
                <option value="homepage" ${exposeTarget === 'homepage' ? 'selected' : ''}>${t('config.cloudflare_tunnel.expose_homepage')}</option>
            </select>
        </label>
        <input type="hidden" id="cf-expose-web-ui-val" data-path="cloudflare_tunnel.expose_web_ui" data-type="json" value="${exposeTarget === 'web_ui' ? 'true' : 'false'}">
        <input type="hidden" id="cf-expose-homepage-val" data-path="cloudflare_tunnel.expose_homepage" data-type="json" value="${exposeTarget === 'homepage' ? 'true' : 'false'}">`;
        html += `<div class="wh-notice cft-notice-warn" style="margin-top:0.5rem;">
            <span>⚠️</span>
            <div><small>${t('config.cloudflare_tunnel.expose_single_hint')}</small></div>
        </div>`;
    }

    // Token input for token auth mode
    if ((cfg.auth_method === 'token' || !cfg.auth_method) && enabled) {
        html += `<div class="field-group" style="margin-top:0.8rem;">
            <div class="field-label">${t('config.cloudflare_tunnel.token_label')}</div>
            <div class="adg-password-row">
                <input class="field-input adg-password-input" type="password" id="cloudflare-tunnel-token" placeholder="${t('config.cloudflare_tunnel.token_placeholder')}">
                <button class="btn-save adg-save-btn" onclick="cloudflareTunnelSaveToken()">💾 ${t('config.cloudflare_tunnel.save_vault')}</button>
                <button class="btn-save adg-save-btn" style="background:var(--accent);margin-left:0.5rem;" onclick="cloudflareTunnelRestart()">🔄 ${t('config.cloudflare_tunnel.start_tunnel')}</button>
            </div>
            <span id="cloudflare-tunnel-token-status" style="font-size:0.78rem;display:block;margin-top:0.3rem;"></span>
        </div>`;
    }

    // Auth hint for named tunnel
    if (cfg.auth_method === 'named' && enabled) {
        html += `<div class="wh-notice cft-notice-warn">
            <span>📄</span>
            <div><small>${t('config.cloudflare_tunnel.named_hint')}</small></div>
        </div>`;
    }

    html += `</div>`; // close section
    document.getElementById('content').innerHTML = html;
}

function cloudflareTunnelToggleLoopback(el) {
    const isOn = el.classList.toggle('on');
    // Use a fixed default port (18080) to avoid ordering issues; 0 = disabled
    const currentPort = configData?.cloudflare_tunnel?.loopback_port;
    const portToUse = (isOn && currentPort > 0) ? currentPort : 18080;
    setNestedValue(configData, 'cloudflare_tunnel.loopback_port', isOn ? portToUse : 0);
    setDirty(true);
    renderCloudflareTunnelSection(window._cfSectionMeta || {});
}

function cloudflareTunnelChangeLoopbackPort(value) {
    const port = parseInt(value, 10);
    if (!port || port < 1024 || port > 65535) return;
    setNestedValue(configData, 'cloudflare_tunnel.loopback_port', port);
    setDirty(true);
}

function cloudflareTunnelSetExposeTarget(value) {
    setNestedValue(configData, 'cloudflare_tunnel.expose_web_ui', value === 'web_ui');
    setNestedValue(configData, 'cloudflare_tunnel.expose_homepage', value === 'homepage');
    // Keep hidden data-path inputs in sync so buildConfigPatchFromForm() captures the values
    const webUiInput = document.getElementById('cf-expose-web-ui-val');
    const hpInput = document.getElementById('cf-expose-homepage-val');
    if (webUiInput) webUiInput.value = value === 'web_ui' ? 'true' : 'false';
    if (hpInput) hpInput.value = value === 'homepage' ? 'true' : 'false';
    setDirty(true);
}

function cloudflareTunnelSaveToken() {
    const token = document.getElementById('cloudflare-tunnel-token')?.value;
    const status = document.getElementById('cloudflare-tunnel-token-status');
    if (!token) {
        if (status) { status.textContent = t('config.cloudflare_tunnel.token_empty'); status.style.color = 'var(--error)'; }
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'cloudflared_token', value: token })
    })
    .then(r => r.json())
    .then(data => {
        if (data.error) {
            if (status) { status.textContent = data.error; status.style.color = 'var(--error)'; }
        } else {
            if (status) { status.textContent = t('config.cloudflare_tunnel.token_saved'); status.style.color = 'var(--success)'; }
            document.getElementById('cloudflare-tunnel-token').value = '';
        }
    })
    .catch(err => {
        if (status) { status.textContent = 'Error: ' + err; status.style.color = 'var(--error)'; }
    });
}

function cloudflareTunnelRestart() {
    const status = document.getElementById('cloudflare-tunnel-token-status');
    if (status) { status.textContent = 'Restarting tunnel...'; status.style.color = 'var(--accent)'; }
    fetch('/api/cloudflare-tunnel/restart', { method: 'POST' })
    .then(r => r.json())
    .then(data => {
        if (data.error) {
            if (status) { status.textContent = data.error; status.style.color = 'var(--error)'; }
        } else {
            if (status) { status.textContent = 'Tunnel restarted successfully'; status.style.color = 'var(--success)'; }
        }
    })
    .catch(err => {
        if (status) { status.textContent = 'Error: ' + err; status.style.color = 'var(--error)'; }
    });
}
