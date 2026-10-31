let _spSection = null;

async function renderSecurityProxySection(section) {
    if (section) _spSection = section; else section = _spSection;
    const cfg = configData.security_proxy || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.security_proxy.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="security_proxy.enabled" role="switch" aria-checked="${enabled}" aria-label="${escapeAttr(t('config.security_proxy.enabled_label'))}" aria-describedby="sp-enabled-help" onclick="toggleBool(this);setNestedValue(configData,'security_proxy.enabled',this.classList.contains('on'));renderSecurityProxySection(null)"></div>
    </div>
    <div class="field-help" id="sp-enabled-help">${t('config.security_proxy.enabled_help')}</div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>${cfgIcon('shield')}</span>
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
        <div id="proxy-status-info" class="sp-status-info" role="status" aria-live="polite">
            ${t('config.security_proxy.status_loading')}
        </div>
        <div class="cfg-actions-row">
            <button type="button" id="proxy-start-btn" class="btn btn-sm btn-primary" onclick="_proxyAction('start', this)" title="${t('config.security_proxy.btn_start')}">${cfgIcon('play')} ${t('config.security_proxy.btn_start')}</button>
            <button type="button" id="proxy-stop-btn" class="btn btn-sm btn-warning" onclick="_proxyAction('stop', this)" title="${t('config.security_proxy.btn_stop')}">${cfgIcon('stop')} ${t('config.security_proxy.btn_stop')}</button>
            <button type="button" id="proxy-reload-btn" class="btn btn-sm btn-secondary" onclick="_proxyAction('reload', this)" title="${t('config.security_proxy.btn_reload')}">${cfgIcon('refresh')} ${t('config.security_proxy.btn_reload')}</button>
            ${cfgDeleteButton({ label: t('config.security_proxy.btn_destroy'), cls: 'btn-sm', attrs: { id: 'proxy-destroy-btn', title: t('config.security_proxy.btn_destroy'), onclick: "_proxyAction('destroy', this)" } })}
            <button type="button" class="btn btn-sm btn-secondary" onclick="_proxyShowLogs()" title="${t('config.security_proxy.btn_logs')}">${cfgIcon('list')} ${t('config.security_proxy.btn_logs')}</button>
            ${cfgActionResult.html('proxy-action-result')}
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.tls_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.tls_desc')}</div>`;

    html += `<div class="field-grid two-cols">
        <div class="field-group">
            <div class="field-label">${t('config.security_proxy.domain_label')}</div>
            <div class="field-help">${t('config.security_proxy.domain_help')}</div>
            <input type="text" class="field-input" data-path="security_proxy.domain" value="${escapeAttr(cfg.domain || '')}" placeholder="aurago.example.com">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.security_proxy.email_label')}</div>
            <div class="field-help">${t('config.security_proxy.email_help')}</div>
            <input type="email" class="field-input" data-path="security_proxy.email" value="${escapeAttr(cfg.email || '')}" placeholder="admin@example.com">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.security_proxy.https_port_label')}</div>
            <div class="field-help">${t('config.security_proxy.https_port_help')}</div>
            <input type="number" class="field-input" data-path="security_proxy.https_port" value="${cfg.https_port || 443}" min="1" max="65535">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.security_proxy.http_port_label')}</div>
            <div class="field-help">${t('config.security_proxy.http_port_help')}</div>
            <input type="number" class="field-input" data-path="security_proxy.http_port" value="${cfg.http_port || 80}" min="1" max="65535">
        </div>
    </div>`;
    html += `</div>`;

    const rl = cfg.rate_limiting || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.rate_limit_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.rate_limit_desc')}</div>`;

    html += `<div class="sp-toggle-row">
        <span class="cfg-label">${t('config.security_proxy.rate_limit_enabled_label')}</span>
        <div class="toggle ${rl.enabled ? 'on' : ''}" data-path="security_proxy.rate_limiting.enabled" role="switch" aria-describedby="sp-rate-limit-hint" aria-checked="${!!rl.enabled}" aria-label="${escapeAttr(t('config.security_proxy.rate_limit_enabled_label'))}" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="field-help" id="sp-rate-limit-hint">${t('config.security_proxy.rate_limit_build_hint')}</div>`;

    html += `<div class="field-grid two-cols">
        <div class="field-group">
            <div class="field-label">${t('config.security_proxy.rps_label')}</div>
            <div class="field-help">${t('config.security_proxy.rps_help')}</div>
            <input type="number" class="field-input" data-path="security_proxy.rate_limiting.requests_per_second" value="${rl.requests_per_second || 10}" min="1" max="10000">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.security_proxy.burst_label')}</div>
            <div class="field-help">${t('config.security_proxy.burst_help')}</div>
            <input type="number" class="field-input" data-path="security_proxy.rate_limiting.burst" value="${rl.burst || 50}" min="1" max="50000">
        </div>
    </div>`;
    html += `</div>`;

    const ipf = cfg.ip_filter || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.ip_filter_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.ip_filter_desc')}</div>`;

    html += `<div class="sp-toggle-row">
        <span class="cfg-label">${t('config.security_proxy.ip_filter_enabled_label')}</span>
        <div class="toggle ${ipf.enabled ? 'on' : ''}" data-path="security_proxy.ip_filter.enabled" role="switch" aria-describedby="sp-ip-filter-help" aria-checked="${!!ipf.enabled}" aria-label="${escapeAttr(t('config.security_proxy.ip_filter_enabled_label'))}" onclick="toggleBool(this)"></div>
    </div>
    <div class="field-help" id="sp-ip-filter-help">${t('config.security_proxy.ip_filter_enabled_help')}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.security_proxy.ip_filter_mode_label')}</div>
        <div class="field-help">${t('config.security_proxy.ip_filter_mode_help')}</div>
        <select class="field-select" data-path="security_proxy.ip_filter.mode">
            <option value="blocklist" ${(ipf.mode||'blocklist')==='blocklist'?'selected':''}>${t('config.security_proxy.ip_filter_mode_blocklist')}</option>
            <option value="allowlist" ${ipf.mode==='allowlist'?'selected':''}>${t('config.security_proxy.ip_filter_mode_allowlist')}</option>
        </select>
    </div>`;

    const curAddresses = (ipf.addresses || []).join('\n');
    html += `<div class="field-group">
        <div class="field-label">${t('config.security_proxy.ip_filter_addresses_label')}</div>
        <div class="field-help">${t('config.security_proxy.ip_filter_addresses_hint')}</div>
        <textarea class="field-input sp-addresses-input" data-path="security_proxy.ip_filter.addresses" data-type="array-lines" rows="3">${escapeHtml(curAddresses)}</textarea>
    </div>`;
    html += `</div>`;

    const ba = cfg.basic_auth || {};
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.basic_auth_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.basic_auth_desc')}</div>`;

    html += `<div class="sp-toggle-row">
        <span class="cfg-label">${t('config.security_proxy.basic_auth_enabled_label')}</span>
        <div class="toggle ${ba.enabled ? 'on' : ''}" data-path="security_proxy.basic_auth.enabled" role="switch" aria-checked="${!!ba.enabled}" aria-label="${escapeAttr(t('config.security_proxy.basic_auth_enabled_label'))}" onclick="toggleBool(this)"></div>
    </div>
    <div class="field-help">${t('config.security_proxy.basic_auth_enabled_help')}</div>`;

    html += `<div class="wh-notice">
        <span>${cfgIcon('lock')}</span>
        <div><small>${t('config.security_proxy.basic_auth_vault_hint')}</small></div>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group sp-placeholder">
        <div class="field-group-title">${t('config.security_proxy.geo_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.geo_desc')}</div>
        <div class="wh-notice">
            <span>${cfgIcon('globe')}</span>
            <div><small>${t('config.security_proxy.geo_placeholder')}</small></div>
        </div>
    </div>`;

    const routes = cfg.additional_routes || [];
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.routes_title')}</div>
        <div class="field-group-desc">${t('config.security_proxy.routes_desc')}</div>`;

    html += `<div class="field-help">${t('config.security_proxy.routes_help')}</div>`;
    html += `<div id="proxy-routes-list">`;
    routes.forEach((route, i) => {
        html += _renderRouteRow(route, i);
    });
    html += `</div>`;

    html += `<button class="btn btn-sm btn-secondary" onclick="_addProxyRoute()">+ ${t('config.security_proxy.routes_add')}</button>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.security_proxy.docker_title')}</div>
        <div class="field-group">
            <div class="field-label">${t('config.security_proxy.docker_host_label')}</div>
            <div class="field-help">${t('config.security_proxy.docker_host_hint')}</div>
            <input type="text" class="field-input" data-path="security_proxy.docker_host" value="${escapeAttr(cfg.docker_host || '')}" placeholder="unix:///var/run/docker.sock">
        </div>
    </div>`;

    html += `<div id="proxy-logs-area" class="is-hidden">
        <div class="field-group">
            <div class="field-group-title">${cfgIcon('list')} ${t('config.security_proxy.logs_title')}</div>
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
        <input type="text" class="field-input sp-route-input-1" aria-label="${escapeAttr(t('config.security_proxy.route_name_label'))}" placeholder="${t('config.security_proxy.route_name')}" value="${escapeAttr(route.name || '')}"
            onchange="_updateProxyRoute(${index},'name',this.value)">
        <input type="text" class="field-input sp-route-input-2" aria-label="${escapeAttr(t('config.security_proxy.route_domain_label'))}" placeholder="${t('config.security_proxy.route_domain')}" value="${escapeAttr(route.domain || '')}"
            onchange="_updateProxyRoute(${index},'domain',this.value)">
        <input type="text" class="field-input sp-route-input-2" aria-label="${escapeAttr(t('config.security_proxy.route_upstream_label'))}" placeholder="${t('config.security_proxy.route_upstream')}" value="${escapeAttr(route.upstream || '')}"
            onchange="_updateProxyRoute(${index},'upstream',this.value)">
        ${cfgDeleteButton({ iconOnly: true, cls: 'btn-sm', label: t('config.security_proxy.route_remove'), attrs: { onclick: '_removeProxyRoute(' + index + ')' } })}
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

async function _proxyAction(action, button) {
    if (action === 'destroy') {
        const answer = await showConfirmDialog({
            title: t('config.security_proxy.confirm_destroy_title'),
            message: t('config.security_proxy.confirm_destroy_message'),
            danger: true,
            confirmText: t('config.security_proxy.btn_destroy')
        });
        if (!answer.confirmed) return;
    }
    // Start and reload can build the rate-limit image first; the other actions stay
    // locked and the line next to them says so until the request finishes.
    return cfgActionResult(button || ('#proxy-' + action + '-btn'), () => fetch(`/api/proxy/${action}`, { method: 'POST' }), {
        target: 'proxy-action-result',
        lock: '#proxy-status-area .cfg-actions-row button',
        pending: (action === 'start' || action === 'reload') ? t('config.security_proxy.action_pending') : undefined,
        success: outcome => (outcome.body && outcome.body.message) || t('config.security_proxy.action_success', { action: t('config.security_proxy.btn_' + action) }),
        failure: t('config.security_proxy.action_failed'),
        onSuccess: () => setTimeout(_proxyFetchStatus, 1500),
        onError: () => _proxyFetchStatus()
    });
}

async function _proxyFetchStatus() {
    try {
        const res = await fetch('/api/proxy/status');
        const data = await res.json();
        const el = document.getElementById('proxy-status-info');
        if (!el) return;
        if (data.status === 'ok' && data.proxy) {
            const p = data.proxy;
            // The badge escapes its text; the image name from the API goes in as a text node.
            const badge = document.createElement('span');
            badge.innerHTML = cfgBadge(p.running ? 'ok' : 'idle', p.state || t('config.security_proxy.state_unknown'));
            const nodes = [badge.firstElementChild];
            if (p.image) {
                nodes.push(document.createTextNode(' - ' + p.image));
            }
            el.replaceChildren(...nodes);
        } else {
            el.textContent = data.message || t('config.security_proxy.status_unknown');
        }
    } catch(e) {
        const el = document.getElementById('proxy-status-info');
        if (el) el.textContent = t('config.security_proxy.error_prefix') + ' ' + e.message;
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
            content.textContent = data.logs || t('config.security_proxy.logs_empty');
        } else {
            content.textContent = data.message || t('config.security_proxy.logs_error');
        }
    } catch(e) {
        content.textContent = t('config.security_proxy.error_with_message', { message: e.message });
    }
}
