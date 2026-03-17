// cfg/security_proxy.js — Security Proxy (Caddy) config section module

let _spSection = null;
let _proxyStatus = null;

async function renderSecurityProxySection(section) {
    if (section) _spSection = section; else section = _spSection;
    const cfg = configData.security_proxy || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.security_proxy.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="security_proxy.enabled" onclick="toggleBool(this);setNestedValue(configData,'security_proxy.enabled',this.classList.contains('on'));renderSecurityProxySection(null)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🛡️</span>
            <div>
                <strong>${t('config.security_proxy.disabled_notice')}</strong><br>
                <small>${t('config.security_proxy.disabled_desc')}</small>
            </div>
        </div></div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    // ── Status & Controls ──
    html += `<div class="field-group" id="proxy-status-area">
        <div class="field-group-title">${t('config.security_proxy.status_title')}</div>
        <div id="proxy-status-info" style="margin-bottom:0.8rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);font-size:0.85rem;color:var(--text-secondary);">
            ${t('config.security_proxy.status_loading')}
        </div>
        <div style="display:flex;gap:0.5rem;flex-wrap:wrap;">
            <button class="btn btn-sm btn-primary" onclick="_proxyAction('start')" title="${t('config.security_proxy.btn_start')}">▶ ${t('config.security_proxy.btn_start')}</button>
            <button class="btn btn-sm btn-warning" onclick="_proxyAction('stop')" title="${t('config.security_proxy.btn_stop')}">⏹ ${t('config.security_proxy.btn_stop')}</button>
            <button class="btn btn-sm btn-secondary" onclick="_proxyAction('reload')" title="${t('config.security_proxy.btn_reload')}">🔄 ${t('config.security_proxy.btn_reload')}</button>
            <button class="btn btn-sm btn-danger" onclick="_proxyAction('destroy')" title="${t('config.security_proxy.btn_destroy')}">🗑️ ${t('config.security_proxy.btn_destroy')}</button>
            <button class="btn btn-sm btn-secondary" onclick="_proxyShowLogs()" title="${t('config.security_proxy.btn_logs')}">📋 ${t('config.security_proxy.btn_logs')}</button>
        </div>
    </div>`;

    // ── Domain & TLS ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.tls_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.tls_desc')}</div>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.domain_label')}</span>
        <input type="text" class="cfg-input" data-path="security_proxy.domain" value="${cfg.domain || ''}"
            placeholder="aurago.example.com" style="width:100%;margin-top:0.2rem;">
    </label>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.email_label')}</span>
        <input type="email" class="cfg-input" data-path="security_proxy.email" value="${cfg.email || ''}"
            placeholder="admin@example.com" style="width:100%;margin-top:0.2rem;">
    </label>`;

    // Ports
    html += `<div style="display:flex;gap:1rem;">
        <label style="flex:1;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.https_port_label')}</span>
            <input type="number" class="cfg-input" data-path="security_proxy.https_port" value="${cfg.https_port || 443}" min="1" max="65535"
                style="width:100%;margin-top:0.2rem;">
        </label>
        <label style="flex:1;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.http_port_label')}</span>
            <input type="number" class="cfg-input" data-path="security_proxy.http_port" value="${cfg.http_port || 80}" min="1" max="65535"
                style="width:100%;margin-top:0.2rem;">
        </label>
    </div>`;
    html += `</div>`;

    // ── Rate Limiting ──
    const rl = cfg.rate_limiting || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.rate_limit_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.rate_limit_desc')}</div>`;

    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.rate_limit_enabled_label')}</span>
        <div class="toggle ${rl.enabled ? 'on' : ''}" data-path="security_proxy.rate_limiting.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div style="display:flex;gap:1rem;">
        <label style="flex:1;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.rps_label')}</span>
            <input type="number" class="cfg-input" data-path="security_proxy.rate_limiting.requests_per_second" value="${rl.requests_per_second || 10}" min="1" max="10000"
                style="width:100%;margin-top:0.2rem;">
        </label>
        <label style="flex:1;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.burst_label')}</span>
            <input type="number" class="cfg-input" data-path="security_proxy.rate_limiting.burst" value="${rl.burst || 50}" min="1" max="50000"
                style="width:100%;margin-top:0.2rem;">
        </label>
    </div>`;
    html += `</div>`;

    // ── IP Filter ──
    const ipf = cfg.ip_filter || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.ip_filter_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.ip_filter_desc')}</div>`;

    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.ip_filter_enabled_label')}</span>
        <div class="toggle ${ipf.enabled ? 'on' : ''}" data-path="security_proxy.ip_filter.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.ip_filter_mode_label')}</span>
        <select class="cfg-input" data-path="security_proxy.ip_filter.mode" style="width:100%;margin-top:0.2rem;">
            <option value="blocklist" ${(ipf.mode||'blocklist')==='blocklist'?'selected':''}>${t('config.security_proxy.ip_filter_mode_blocklist')}</option>
            <option value="allowlist" ${ipf.mode==='allowlist'?'selected':''}>${t('config.security_proxy.ip_filter_mode_allowlist')}</option>
        </select>
    </label>`;

    const curAddresses = (ipf.addresses || []).join('\n');
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.ip_filter_addresses_label')} <small style="color:var(--text-tertiary);">(${t('config.security_proxy.ip_filter_addresses_hint')})</small></span>
        <textarea class="cfg-input" data-path="security_proxy.ip_filter.addresses" data-type="array-lines" rows="3"
            style="width:100%;margin-top:0.2rem;font-family:monospace;font-size:0.8rem;">${curAddresses}</textarea>
    </label>`;
    html += `</div>`;

    // ── Basic Auth ──
    const ba = cfg.basic_auth || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.basic_auth_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.basic_auth_desc')}</div>`;

    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.basic_auth_enabled_label')}</span>
        <div class="toggle ${ba.enabled ? 'on' : ''}" data-path="security_proxy.basic_auth.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="wh-notice" style="margin-bottom:0.6rem;">
        <span>🔐</span>
        <div><small>${t('config.security_proxy.basic_auth_vault_hint')}</small></div>
    </div>`;
    html += `</div>`;

    // ── Geo-Blocking (V2 — placeholder) ──
    html += `<div class="field-group" style="opacity:0.5;">
        <div class="field-group-title">${t('config.security_proxy.geo_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.geo_desc')}</div>
        <div class="wh-notice">
            <span>🌍</span>
            <div><small>${t('config.security_proxy.geo_placeholder')}</small></div>
        </div>
    </div>`;

    // ── Additional Routes ──
    const routes = cfg.additional_routes || [];
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.routes_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.routes_desc')}</div>`;

    html += `<div id="proxy-routes-list">`;
    routes.forEach((route, i) => {
        html += _renderRouteRow(route, i);
    });
    html += `</div>`;

    html += `<button class="btn btn-sm btn-secondary" onclick="_addProxyRoute()" style="margin-top:0.5rem;">+ ${t('config.security_proxy.routes_add')}</button>`;
    html += `</div>`;

    // ── Docker Host override ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.docker_title')}</div>
        <label style="display:block;margin-bottom:0.6rem;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.security_proxy.docker_host_label')} <small style="color:var(--text-tertiary);">(${t('config.security_proxy.docker_host_hint')})</small></span>
            <input type="text" class="cfg-input" data-path="security_proxy.docker_host" value="${cfg.docker_host || ''}"
                placeholder="unix:///var/run/docker.sock" style="width:100%;margin-top:0.2rem;">
        </label>
    </div>`;

    // Logs area (hidden initially)
    html += `<div id="proxy-logs-area" style="display:none;">
        <div class="field-group">
            <div class="field-group-title">📋 ${t('config.security_proxy.logs_title')}</div>
            <pre id="proxy-logs-content" style="max-height:300px;overflow:auto;padding:0.8rem;background:var(--bg-primary);border-radius:8px;font-size:0.75rem;white-space:pre-wrap;word-break:break-all;color:var(--text-secondary);"></pre>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Fetch live status
    _proxyFetchStatus();
}

function _renderRouteRow(route, index) {
    return `<div class="proxy-route-row" style="display:flex;gap:0.5rem;margin-bottom:0.5rem;align-items:center;" data-index="${index}">
        <input type="text" class="cfg-input" placeholder="${t('config.security_proxy.route_name')}" value="${route.name || ''}"
            onchange="_updateProxyRoute(${index},'name',this.value)" style="flex:1;">
        <input type="text" class="cfg-input" placeholder="${t('config.security_proxy.route_domain')}" value="${route.domain || ''}"
            onchange="_updateProxyRoute(${index},'domain',this.value)" style="flex:2;">
        <input type="text" class="cfg-input" placeholder="${t('config.security_proxy.route_upstream')}" value="${route.upstream || ''}"
            onchange="_updateProxyRoute(${index},'upstream',this.value)" style="flex:2;">
        <button class="btn btn-sm btn-danger" onclick="_removeProxyRoute(${index})" title="Remove">✕</button>
    </div>`;
}

function _addProxyRoute() {
    if (!configData.security_proxy) configData.security_proxy = {};
    if (!configData.security_proxy.additional_routes) configData.security_proxy.additional_routes = [];
    configData.security_proxy.additional_routes.push({ name: '', domain: '', upstream: '' });
    renderSecurityProxySection(null);
    markDirty();
}

function _removeProxyRoute(index) {
    if (!configData.security_proxy?.additional_routes) return;
    configData.security_proxy.additional_routes.splice(index, 1);
    renderSecurityProxySection(null);
    markDirty();
}

function _updateProxyRoute(index, field, value) {
    if (!configData.security_proxy?.additional_routes?.[index]) return;
    configData.security_proxy.additional_routes[index][field] = value;
    markDirty();
}

async function _proxyAction(action) {
    try {
        const res = await fetch(`/api/proxy/${action}`, { method: 'POST' });
        const data = await res.json();
        if (data.status === 'ok') {
            showToast(data.message || action + ' successful', 'success');
        } else {
            showToast(data.message || 'Action failed', 'error');
        }
        setTimeout(_proxyFetchStatus, 1500);
    } catch(e) {
        showToast('Error: ' + e.message, 'error');
    }
}

async function _proxyFetchStatus() {
    try {
        const res = await fetch('/api/proxy/status');
        const data = await res.json();
        const el = document.getElementById('proxy-status-info');
        if (!el) return;
        if (data.status === 'ok' && data.proxy) {
            const p = data.proxy;
            const color = p.running ? 'var(--success)' : 'var(--text-tertiary)';
            const icon = p.running ? '🟢' : '⚪';
            el.innerHTML = `${icon} <strong style="color:${color}">${p.state || 'unknown'}</strong>` +
                (p.image ? ` &mdash; ${p.image}` : '');
        } else {
            el.textContent = data.message || t('config.security_proxy.status_unknown');
        }
    } catch(e) {
        const el = document.getElementById('proxy-status-info');
        if (el) el.textContent = 'Error: ' + e.message;
    }
}

async function _proxyShowLogs() {
    const area = document.getElementById('proxy-logs-area');
    const content = document.getElementById('proxy-logs-content');
    if (!area || !content) return;
    area.style.display = 'block';
    content.textContent = t('config.security_proxy.status_loading');
    try {
        const res = await fetch('/api/proxy/logs?tail=200');
        const data = await res.json();
        if (data.status === 'ok') {
            content.textContent = data.logs || '(empty)';
        } else {
            content.textContent = data.message || 'Error fetching logs';
        }
    } catch(e) {
        content.textContent = 'Error: ' + e.message;
    }
}
