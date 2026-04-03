let _spSection = null;

async function renderSecurityProxySection(section) {
    if (section) _spSection = section; else section = _spSection;
    const cfg = configData.security_proxy || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.security_proxy.enabled_label')}</span>
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

    html += `<div class="field-group" id="proxy-status-area">
        <div class="field-group-title">${t('config.security_proxy.status_title')}</div>
        <div id="proxy-status-info" class="sp-status-info">
            ${t('config.security_proxy.status_loading')}
        </div>
        <div class="cfg-field-row">
            <button class="btn btn-sm btn-primary" onclick="_proxyAction('start')" title="${t('config.security_proxy.btn_start')}">▶ ${t('config.security_proxy.btn_start')}</button>
            <button class="btn btn-sm btn-warning" onclick="_proxyAction('stop')" title="${t('config.security_proxy.btn_stop')}">⏹ ${t('config.security_proxy.btn_stop')}</button>
            <button class="btn btn-sm btn-secondary" onclick="_proxyAction('reload')" title="${t('config.security_proxy.btn_reload')}">🔄 ${t('config.security_proxy.btn_reload')}</button>
            <button class="btn btn-sm btn-danger" onclick="_proxyAction('destroy')" title="${t('config.security_proxy.btn_destroy')}">🗑️ ${t('config.security_proxy.btn_destroy')}</button>
            <button class="btn btn-sm btn-secondary" onclick="_proxyShowLogs()" title="${t('config.security_proxy.btn_logs')}">📋 ${t('config.security_proxy.btn_logs')}</button>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.tls_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.tls_desc')}</div>`;

    html += `<label class="sp-label">
        <span class="cfg-label">${t('config.security_proxy.domain_label')}</span>
        <input type="text" class="cfg-input cfg-input-full" data-path="security_proxy.domain" value="${escapeAttr(cfg.domain || '')}"
            placeholder="aurago.example.com">
    </label>`;

    html += `<label class="sp-label">
        <span class="cfg-label">${t('config.security_proxy.email_label')}</span>
        <input type="email" class="cfg-input cfg-input-full" data-path="security_proxy.email" value="${escapeAttr(cfg.email || '')}"
            placeholder="admin@example.com">
    </label>`;

    html += `<div class="sp-port-row">
        <label>
            <span class="cfg-label">${t('config.security_proxy.https_port_label')}</span>
            <input type="number" class="cfg-input cfg-input-full" data-path="security_proxy.https_port" value="${cfg.https_port || 443}" min="1" max="65535">
        </label>
        <label>
            <span class="cfg-label">${t('config.security_proxy.http_port_label')}</span>
            <input type="number" class="cfg-input cfg-input-full" data-path="security_proxy.http_port" value="${cfg.http_port || 80}" min="1" max="65535">
        </label>
    </div>`;
    html += `</div>`;

    const rl = cfg.rate_limiting || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.rate_limit_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.rate_limit_desc')}</div>`;

    html += `<div class="sp-toggle-row">
        <span class="cfg-label">${t('config.security_proxy.rate_limit_enabled_label')}</span>
        <div class="toggle ${rl.enabled ? 'on' : ''}" data-path="security_proxy.rate_limiting.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="sp-port-row">
        <label>
            <span class="cfg-label">${t('config.security_proxy.rps_label')}</span>
            <input type="number" class="cfg-input cfg-input-full" data-path="security_proxy.rate_limiting.requests_per_second" value="${rl.requests_per_second || 10}" min="1" max="10000">
        </label>
        <label>
            <span class="cfg-label">${t('config.security_proxy.burst_label')}</span>
            <input type="number" class="cfg-input cfg-input-full" data-path="security_proxy.rate_limiting.burst" value="${rl.burst || 50}" min="1" max="50000">
        </label>
    </div>`;
    html += `</div>`;

    const ipf = cfg.ip_filter || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.ip_filter_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.ip_filter_desc')}</div>`;

    html += `<div class="sp-toggle-row">
        <span class="cfg-label">${t('config.security_proxy.ip_filter_enabled_label')}</span>
        <div class="toggle ${ipf.enabled ? 'on' : ''}" data-path="security_proxy.ip_filter.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<label class="sp-label">
        <span class="cfg-label">${t('config.security_proxy.ip_filter_mode_label')}</span>
        <select class="cfg-input cfg-input-full" data-path="security_proxy.ip_filter.mode">
            <option value="blocklist" ${(ipf.mode||'blocklist')==='blocklist'?'selected':''}>${t('config.security_proxy.ip_filter_mode_blocklist')}</option>
            <option value="allowlist" ${ipf.mode==='allowlist'?'selected':''}>${t('config.security_proxy.ip_filter_mode_allowlist')}</option>
        </select>
    </label>`;

    const curAddresses = (ipf.addresses || []).join('\n');
    html += `<label class="sp-label">
        <span class="cfg-label">${t('config.security_proxy.ip_filter_addresses_label')} <small style="color:var(--text-tertiary);">(${t('config.security_proxy.ip_filter_addresses_hint')})</small></span>
        <textarea class="cfg-input cfg-input-full" data-path="security_proxy.ip_filter.addresses" data-type="array-lines" rows="3"
            style="font-family:monospace;font-size:0.8rem;">${escapeHtml(curAddresses)}</textarea>
    </label>`;
    html += `</div>`;

    const ba = cfg.basic_auth || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.basic_auth_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.basic_auth_desc')}</div>`;

    html += `<div class="sp-toggle-row">
        <span class="cfg-label">${t('config.security_proxy.basic_auth_enabled_label')}</span>
        <div class="toggle ${ba.enabled ? 'on' : ''}" data-path="security_proxy.basic_auth.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="wh-notice">
        <span>🔐</span>
        <div><small>${t('config.security_proxy.basic_auth_vault_hint')}</small></div>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group sp-placeholder">
        <div class="field-group-title">${t('config.security_proxy.geo_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.geo_desc')}</div>
        <div class="wh-notice">
            <span>🌍</span>
            <div><small>${t('config.security_proxy.geo_placeholder')}</small></div>
        </div>
    </div>`;

    const routes = cfg.additional_routes || [];
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.routes_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.routes_desc')}</div>`;

    html += `<div id="proxy-routes-list">`;
    routes.forEach((route, i) => {
        html += _renderRouteRow(route, i);
    });
    html += `</div>`;

    html += `<button class="btn btn-sm btn-secondary" onclick="_addProxyRoute()">+ ${t('config.security_proxy.routes_add')}</button>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.docker_title')}</div>
        <label class="sp-label">
            <span class="cfg-label">${t('config.security_proxy.docker_host_label')} <small style="color:var(--text-tertiary);">(${t('config.security_proxy.docker_host_hint')})</small></span>
            <input type="text" class="cfg-input cfg-input-full" data-path="security_proxy.docker_host" value="${escapeAttr(cfg.docker_host || '')}"
                placeholder="unix:///var/run/docker.sock">
        </label>
    </div>`;

    html += `<div id="proxy-logs-area" class="is-hidden">
        <div class="field-group">
            <div class="field-group-title">📋 ${t('config.security_proxy.logs_title')}</div>
            <pre id="proxy-logs-content" class="sp-logs-pre"></pre>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    _proxyFetchStatus();
}

function _renderRouteRow(route, index) {
    return `<div class="sp-route-row" data-index="${index}">
        <input type="text" class="cfg-input sp-route-input-1" placeholder="${t('config.security_proxy.route_name')}" value="${escapeAttr(route.name || '')}"
            onchange="_updateProxyRoute(${index},'name',this.value)">
        <input type="text" class="cfg-input sp-route-input-2" placeholder="${t('config.security_proxy.route_domain')}" value="${escapeAttr(route.domain || '')}"
            onchange="_updateProxyRoute(${index},'domain',this.value)">
        <input type="text" class="cfg-input sp-route-input-2" placeholder="${t('config.security_proxy.route_upstream')}" value="${escapeAttr(route.upstream || '')}"
            onchange="_updateProxyRoute(${index},'upstream',this.value)">
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
    area.classList.remove('is-hidden');
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
