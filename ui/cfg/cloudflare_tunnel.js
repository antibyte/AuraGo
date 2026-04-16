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

    html += `<div class="wh-notice cft-notice-info">
        <span>🔒</span>
        <div><small>${t('config.cloudflare_tunnel.info')}</small></div>
    </div>`;

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

    html += `<div class="cft-toggle-row">
        <span class="cft-toggle-label">${t('config.cloudflare_tunnel.readonly_label')}</span>
        <div class="toggle ${readOnly ? 'on' : ''}" data-path="cloudflare_tunnel.readonly" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="cft-toggle-row">
        <span class="cft-toggle-label">${t('config.cloudflare_tunnel.auto_start_label')}</span>
        <div class="toggle ${autoStart ? 'on' : ''}" data-path="cloudflare_tunnel.auto_start" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="cft-grid">`;

    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.mode')}</span>
        <select class="field-input cft-field-input" data-path="cloudflare_tunnel.mode" onchange="setNestedValue(configData,'cloudflare_tunnel.mode',this.value);setDirty(true)">
            <option value="auto" ${cfg.mode === 'auto' || !cfg.mode ? 'selected' : ''}>${t('config.cloudflare_tunnel.mode_auto')}</option>
            <option value="docker" ${cfg.mode === 'docker' ? 'selected' : ''}>${t('config.cloudflare_tunnel.mode_docker')}</option>
            <option value="native" ${cfg.mode === 'native' ? 'selected' : ''}>${t('config.cloudflare_tunnel.mode_native')}</option>
        </select>
    </label>`;

    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.auth_method')}</span>
        <select class="field-input cft-field-input" data-path="cloudflare_tunnel.auth_method" onchange="setNestedValue(configData,'cloudflare_tunnel.auth_method',this.value);setDirty(true)">
            <option value="token" ${cfg.auth_method === 'token' || !cfg.auth_method ? 'selected' : ''}>${t('config.cloudflare_tunnel.auth_connector_token')}</option>
            <option value="named" ${cfg.auth_method === 'named' ? 'selected' : ''}>${t('config.cloudflare_tunnel.auth_named_tunnel')}</option>
            <option value="quick" ${cfg.auth_method === 'quick' ? 'selected' : ''}>${t('config.cloudflare_tunnel.auth_quick_tunnel')}</option>
        </select>
    </label>`;

    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.tunnel_name')}</span>
        <input class="field-input cft-field-input" data-path="cloudflare_tunnel.tunnel_name" value="${escapeAttr(cfg.tunnel_name || '')}"
            placeholder="my-tunnel" onchange="setNestedValue(configData,'cloudflare_tunnel.tunnel_name',this.value);setDirty(true)">
    </label>`;

    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.account_id')}</span>
        <input class="field-input cft-field-input" data-path="cloudflare_tunnel.account_id" value="${escapeAttr(cfg.account_id || '')}"
            placeholder="optional" onchange="setNestedValue(configData,'cloudflare_tunnel.account_id',this.value);setDirty(true)">
    </label>`;

    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.metrics_port')}</span>
        <input class="field-input cft-field-input" data-path="cloudflare_tunnel.metrics_port" type="number" value="${cfg.metrics_port || 0}"
            placeholder="0 = disabled" onchange="setNestedValue(configData,'cloudflare_tunnel.metrics_port',parseInt(this.value)||0);setDirty(true)">
    </label>`;

    html += `<label class="cft-field-label">
        <span class="cft-field-caption">${t('config.cloudflare_tunnel.log_level')}</span>
        <select class="field-input cft-field-input" data-path="cloudflare_tunnel.log_level" onchange="setNestedValue(configData,'cloudflare_tunnel.log_level',this.value);setDirty(true)">
            <option value="info" ${cfg.log_level === 'info' || !cfg.log_level ? 'selected' : ''}>${t('config.cloudflare_tunnel.log_info')}</option>
            <option value="debug" ${cfg.log_level === 'debug' ? 'selected' : ''}>${t('config.cloudflare_tunnel.log_debug')}</option>
            <option value="warn" ${cfg.log_level === 'warn' ? 'selected' : ''}>${t('config.cloudflare_tunnel.log_warn')}</option>
            <option value="error" ${cfg.log_level === 'error' ? 'selected' : ''}>${t('config.cloudflare_tunnel.log_error')}</option>
        </select>
    </label>`;

    html += `</div>`;

    const httpsEnabled = (configData.server?.https?.enabled === true);
    if (httpsEnabled) {
        const loopbackEnabled = (cfg.loopback_port !== undefined && cfg.loopback_port > 0);
        const loopbackPortVal = loopbackEnabled ? cfg.loopback_port : 18080;
        html += `<div class="cft-toggle-row cf-loopback-toggle">
            <span class="cft-toggle-label">${t('config.cloudflare_tunnel.loopback_label')}</span>
            <div class="toggle ${loopbackEnabled ? 'on' : ''}" onclick="cloudflareTunnelToggleLoopback(this)"></div>
        </div>`;
        html += `<div id="cf-loopback-port-row" class="cf-loopback-hint${loopbackEnabled ? '' : ' is-hidden'}">
            → http://127.0.0.1:<input type="number" min="1024" max="65535" value="${loopbackPortVal}"
                data-path="cloudflare_tunnel.loopback_port"
                class="cf-loopback-port-input"
                onchange="cloudflareTunnelChangeLoopbackPort(this.value)">
        </div>`;
        html += `<div class="wh-notice cft-notice-info cf-notice-loopback">
            <span>🔄</span>
            <div><small>${t('config.cloudflare_tunnel.loopback_hint')}</small></div>
        </div>`;
    }
    html += `<div class="cft-exposure-heading-wrap">
        <span class="cft-exposure-heading">${t('config.cloudflare_tunnel.exposure_heading')}</span>
    </div>`;

    const isNamed = cfg.auth_method === 'named';
    if (isNamed) {
        html += `<div class="cft-toggle-row cft-toggle-row-exposure">
            <span class="cft-toggle-label">${t('config.cloudflare_tunnel.expose_web_ui')}</span>
            <div class="toggle ${exposeWebUI ? 'on' : ''}" data-path="cloudflare_tunnel.expose_web_ui" onclick="toggleBool(this)"></div>
        </div>`;
        html += `<div class="cft-toggle-row">
            <span class="cft-toggle-label">${t('config.cloudflare_tunnel.expose_homepage')}</span>
            <div class="toggle ${exposeHomepage ? 'on' : ''}" data-path="cloudflare_tunnel.expose_homepage" onclick="toggleBool(this)"></div>
        </div>`;
        html += `<div class="wh-notice cft-notice-info cf-notice-mt-sm">
            <span>ℹ️</span>
            <div><small>${t('config.cloudflare_tunnel.expose_named_hint')}</small></div>
        </div>`;
    } else {
        const exposeTarget = (exposeHomepage && !exposeWebUI) ? 'homepage' : 'web_ui';
        html += `<label class="cft-field-label cf-expose-narrow">
            <span class="cft-field-caption">${t('config.cloudflare_tunnel.expose_target_label')}</span>
            <select class="field-input cft-field-input" onchange="cloudflareTunnelSetExposeTarget(this.value)">
                <option value="web_ui" ${exposeTarget === 'web_ui' ? 'selected' : ''}>${t('config.cloudflare_tunnel.expose_web_ui')}</option>
                <option value="homepage" ${exposeTarget === 'homepage' ? 'selected' : ''}>${t('config.cloudflare_tunnel.expose_homepage')}</option>
            </select>
        </label>
        <input type="hidden" id="cf-expose-web-ui-val" data-path="cloudflare_tunnel.expose_web_ui" data-type="json" value="${exposeTarget === 'web_ui' ? 'true' : 'false'}">
        <input type="hidden" id="cf-expose-homepage-val" data-path="cloudflare_tunnel.expose_homepage" data-type="json" value="${exposeTarget === 'homepage' ? 'true' : 'false'}">`;
        html += `<div class="wh-notice cft-notice-warn cf-notice-mt-sm">
            <span>⚠️</span>
            <div><small>${t('config.cloudflare_tunnel.expose_single_hint')}</small></div>
        </div>`;
    }

    if ((cfg.auth_method === 'token' || !cfg.auth_method) && enabled) {
        html += `<div class="field-group cf-token-group">
            <div class="field-label">${t('config.cloudflare_tunnel.token_label')}</div>
            <div class="cfg-password-row">
                <input class="field-input cfg-password-input" type="password" id="cloudflare-tunnel-token" value="${escapeAttr(cfgSecretValue(cfg.token))}" placeholder="${escapeAttr(cfgSecretPlaceholder(cfg.token, t('config.cloudflare_tunnel.token_placeholder')))}">
                <button class="btn-save adg-save-btn" onclick="cloudflareTunnelSaveToken()">💾 ${t('config.cloudflare_tunnel.save_vault')}</button>
                <button class="btn-save adg-save-btn cf-restart-btn" onclick="cloudflareTunnelRestart()">🔄 ${t('config.cloudflare_tunnel.start_tunnel')}</button>
            </div>
            <span id="cloudflare-tunnel-token-status" class="cf-token-status"></span>
        </div>`;
    }

    if (cfg.auth_method === 'named' && enabled) {
        html += `<div class="wh-notice cft-notice-warn">
            <span>📄</span>
            <div><small>${t('config.cloudflare_tunnel.named_hint')}</small></div>
        </div>`;
    }

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
}

function cloudflareTunnelToggleLoopback(el) {
    const isOn = el.classList.toggle('on');
    const currentPort = configData?.cloudflare_tunnel?.loopback_port;
    const portToUse = (isOn && currentPort > 0) ? currentPort : 18080;
    setNestedValue(configData, 'cloudflare_tunnel.loopback_port', isOn ? portToUse : 0);
    setDirty(true);
    const portRow = document.getElementById('cf-loopback-port-row');
    if (portRow) {
        portRow.classList.toggle('is-hidden', !isOn);
        const inp = portRow.querySelector('input[type="number"]');
        if (inp) inp.value = isOn ? portToUse : 0;
    }
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
        if (status) { status.textContent = t('config.cloudflare_tunnel.token_empty'); status.className = 'cf-token-status is-error'; }
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
            if (status) { status.textContent = data.error; status.className = 'cf-token-status is-error'; }
        } else {
            if (status) { status.textContent = t('config.cloudflare_tunnel.token_saved'); status.className = 'cf-token-status is-success'; }
            cfgMarkSecretStored(document.getElementById('cloudflare-tunnel-token'), 'cloudflare_tunnel.token');
        }
    })
    .catch(err => {
        if (status) { status.textContent = 'Error: ' + err; status.className = 'cf-token-status is-error'; }
    });
}

function cloudflareTunnelRestart() {
    const status = document.getElementById('cloudflare-tunnel-token-status');
    if (status) { status.textContent = t('config.cloudflare_tunnel.restarting'); status.className = 'cf-token-status is-pending'; }
    fetch('/api/cloudflare-tunnel/restart', { method: 'POST' })
    .then(r => r.json())
    .then(data => {
        if (data.error) {
            if (status) { status.textContent = data.error; status.className = 'cf-token-status is-error'; }
        } else {
            if (status) { status.textContent = t('config.cloudflare_tunnel.restart_success'); status.className = 'cf-token-status is-success'; }
        }
    })
    .catch(err => {
        if (status) { status.textContent = 'Error: ' + err; status.className = 'cf-token-status is-error'; }
    });
}
