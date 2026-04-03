let _tsSection = null;

async function renderTailscaleSection(section) {
    if (section) _tsSection = section; else section = _tsSection;
    const cfg = configData.tailscale || {};
    const tsnet = cfg.tsnet || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.tailscale.api_title')}</div>
        <div class="field-group-desc">${t('config.tailscale.api_desc')}</div>`;

    html += `<div class="ts-toggle-row">
        <span class="ts-toggle-label">${t('config.tailscale.enabled_label')}</span>
        <div class="toggle ${cfg.enabled ? 'on' : ''}" data-path="tailscale.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="ts-toggle-row">
        <span class="ts-toggle-label">${t('config.tailscale.readonly_label')}</span>
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="tailscale.readonly" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<label class="ts-label-block">
        <span class="ts-toggle-label">${t('config.tailscale.tailnet_label')}</span>
        <input type="text" class="cfg-input cfg-input-full" data-path="tailscale.tailnet" value="${escapeAttr(cfg.tailnet || '')}"
            placeholder="example.com">
    </label>`;

    html += `<div class="field-group ts-mt">
        <div class="ts-key-label">🔑 ${t('config.tailscale.api_key_label')}</div>
        <div class="cfg-secret-row">
            <div class="password-wrap ts-pw-wrap">
                <input class="field-input cfg-input" type="password" id="ts-api-key-input" placeholder="tskey-api-••••••••" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
            <button class="btn-save cfg-save-btn-sm" onclick="tsSaveApiKey()">💾 ${t('config.tailscale.save_vault')}</button>
        </div>
        <div id="ts-api-key-status" class="ts-key-status"></div>
        <div class="ts-key-hint">${t('config.tailscale.api_key_hint')}</div>
    </div>`;

    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.tailscale.tsnet_title')}</div>
        <div class="field-group-desc">${t('config.tailscale.tsnet_desc')}</div>`;

    const tsEnabled = tsnet.enabled === true;
    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.tailscale.tsnet_enabled_label')}</span>
        <div class="toggle ${tsEnabled ? 'on' : ''}" data-path="tailscale.tsnet.enabled" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.enabled',this.classList.contains('on'));renderTailscaleSection(null)"></div>
    </div>`;

    if (tsEnabled) {
        html += `<label class="ts-label-block">
            <span class="ts-toggle-label">${t('config.tailscale.tsnet_hostname_label')}</span>
            <input type="text" class="cfg-input cfg-input-full" data-path="tailscale.tsnet.hostname" value="${escapeAttr(tsnet.hostname || 'aurago')}"
                placeholder="aurago">
            <small class="ts-hint">${t('config.tailscale.tsnet_hostname_hint')}</small>
        </label>`;

        html += `<label class="ts-label-block">
            <span class="ts-toggle-label">${t('config.tailscale.tsnet_state_dir_label')}</span>
            <input type="text" class="cfg-input cfg-input-full" data-path="tailscale.tsnet.state_dir" value="${escapeAttr(tsnet.state_dir || '')}"
                placeholder="data/tsnet">
            <small class="ts-hint">${t('config.tailscale.tsnet_state_dir_hint')}</small>
        </label>`;

        const serveHTTP = tsnet.serve_http === true;
        const exposeHomepage = tsnet.expose_homepage === true;
        const funnel = tsnet.funnel === true;
        const allowHTTPFallback = tsnet.allow_http_fallback === true;
        const homepageCfg = configData.homepage || {};
        html += `<div class="ts-exposure-box">
            <div class="ts-exposure-title">${t('config.tailscale.tsnet_exposure_title')}</div>
            <div class="ts-exposure-row">
                <span class="ts-exposure-label">${t('config.tailscale.tsnet_serve_http_label')}</span>
                <div class="toggle ${serveHTTP ? 'on' : ''}" data-path="tailscale.tsnet.serve_http" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.serve_http',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block-mb">${t('config.tailscale.tsnet_serve_http_hint')}</small>

            <div class="ts-exposure-row">
                <span class="ts-exposure-label">${t('config.tailscale.tsnet_expose_homepage_label')}</span>
                <div class="toggle ${exposeHomepage ? 'on' : ''}" data-path="tailscale.tsnet.expose_homepage" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.expose_homepage',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block">${t('config.tailscale.tsnet_expose_homepage_hint')}</small>
            ${homepageCfg.webserver_enabled ? '' : `<div class="ts-warning-box">${t('config.tailscale.tsnet_homepage_requires_webserver')}</div>`}

            <div class="ts-exposure-row-mt">
                <span class="ts-exposure-label">${t('config.tailscale.tsnet_funnel_label')}</span>
                <div class="toggle ${funnel ? 'on' : ''}" data-path="tailscale.tsnet.funnel" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.funnel',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block">${t('config.tailscale.tsnet_funnel_hint')}</small>
            ${serveHTTP ? '' : `<div class="ts-info-box">${t('config.tailscale.tsnet_funnel_requires_web')}</div>`}

            <div class="ts-exposure-row-mt">
                <span class="ts-exposure-label">${t('config.tailscale.tsnet_allow_http_fallback_label')}</span>
                <div class="toggle ${allowHTTPFallback ? 'on' : ''}" data-path="tailscale.tsnet.allow_http_fallback" onclick="toggleBool(this)"></div>
            </div>
            <small class="ts-hint-block">${t('config.tailscale.tsnet_allow_http_fallback_hint')}</small>
        </div>`;

        html += `<div class="wh-notice ts-mt">
            <span>ℹ️</span>
            <div>
                <strong>${t('config.tailscale.tsnet_requirements_title')}</strong><br>
                <small>${t('config.tailscale.tsnet_https_requirements')}</small><br>
                <small>${t('config.tailscale.tsnet_funnel_requirements')}</small>
            </div>
        </div>`;

        html += `<div class="field-group ts-mt">
            <div class="ts-key-label">🔑 ${t('config.tailscale.tsnet_auth_key_label')}</div>
            <div class="cfg-secret-row">
                <div class="password-wrap ts-pw-wrap">
                    <input class="field-input cfg-input" type="password" id="ts-auth-key-input" placeholder="tskey-auth-••••••••" autocomplete="off">
                    <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                </div>
                <button class="btn-save cfg-save-btn-sm" onclick="tsSaveAuthKey()">💾 ${t('config.tailscale.save_vault')}</button>
            </div>
            <div id="ts-auth-key-status" class="ts-key-status"></div>
            <div class="ts-key-hint">${t('config.tailscale.tsnet_auth_key_hint')}</div>
        </div>`;

        html += `<div id="tsnet-status-area" class="ts-status-area">
            <div class="ts-status-title">${t('config.tailscale.tsnet_status_title')}</div>
            <div id="tsnet-status-info" class="ts-status-info">
                ${t('config.tailscale.tsnet_status_loading')}
            </div>
            <div class="ts-btn-row">
                <button class="btn btn-sm btn-secondary" onclick="_tsnetRefreshStatus()">🔄 ${t('config.tailscale.tsnet_btn_refresh')}</button>
                <button id="tsnet-btn-start" class="btn btn-sm btn-success is-hidden" onclick="_tsnetStart()">▶ ${t('config.tailscale.tsnet_btn_start')}</button>
                <button class="btn btn-sm btn-warning" onclick="_tsnetStop()">⏹ ${t('config.tailscale.tsnet_btn_stop')}</button>
            </div>
        </div>`;

    } else {
        html += `<div class="wh-notice">
            <span>📡</span>
            <div>
                <strong>${t('config.tailscale.tsnet_disabled_notice')}</strong><br>
                <small>${t('config.tailscale.tsnet_disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `</div>`;
    html += `</div>`;

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    if (tsEnabled) {
        _tsnetRefreshStatus();
    }
}

async function _tsnetRefreshStatus() {
    const el = document.getElementById('tsnet-status-info');
    if (!el) return;
    el.textContent = t('config.tailscale.tsnet_status_loading');

    try {
        const resp = await fetch('/api/tsnet/status');
        const data = await resp.json();

        let info = '';
        const startBtn = document.getElementById('tsnet-btn-start');
        if (data.running) {
            if (data.serving_http) {
                info += `<span class="ts-color-success">● ${t('config.tailscale.tsnet_status_running')}</span>`;
                if (data.funnel_active && data.public_url) {
                    info += `<div class="ts-detail-box">🌍 <strong>${escapeHtml(t('config.tailscale.tsnet_public_url_label'))}:</strong> <a href="${escapeAttr(data.public_url)}" target="_blank" rel="noopener noreferrer" class="ts-link">${escapeHtml(data.public_url)}</a></div>`;
                }
                if (data.http_fallback) {
                    info += `<div class="ts-warning-detail">
                        ⚠️ ${t('config.tailscale.tsnet_http_fallback_notice') || 'Running in HTTP mode (port 80) — enable HTTPS in the Tailscale admin panel for encrypted access.'}
                        <a href="https://tailscale.com/s/https" target="_blank" rel="noopener noreferrer" class="ts-link-ml">Enable HTTPS →</a>
                    </div>`;
                    const httpBase = data.dns ? data.dns.replace(/\.$/, '') : (data.ips && data.ips.length ? data.ips[0] : null);
                    if (httpBase) {
                        const httpUrl = `http://${httpBase}`;
                        info += `<div class="ts-url-row">🌐 <strong>URL:</strong> <a href="${escapeAttr(httpUrl)}" target="_blank" rel="noopener noreferrer" class="ts-link">${escapeHtml(httpUrl)}</a></div>`;
                    }
                } else {
                    if (data.web_ui_url) {
                        info += `<div class="ts-url-row">🌐 <strong>URL:</strong> <a href="${escapeAttr(data.web_ui_url)}" target="_blank" rel="noopener noreferrer" class="ts-link">${escapeHtml(data.web_ui_url)}</a></div>`;
                    }
                }
                if (data.dns) info += `<br><strong>DNS:</strong> <code>${escapeHtml(data.dns)}</code>`;
                if (data.ips && data.ips.length) info += `<br><strong>IPs:</strong> ${escapeHtml(data.ips.join(', '))}`;
                if (!data.http_fallback && data.cert_dns && data.cert_dns.length) info += `<br><strong>Cert:</strong> ${escapeHtml(data.cert_dns.join(', '))}`;
            } else {
                info += `<span class="ts-color-success">● ${t('config.tailscale.tsnet_status_running')}</span> <span class="ts-network-muted">(${t('config.tailscale.tsnet_network_only')})</span>`;
                if (data.dns) info += `<br><strong>DNS:</strong> <code>${escapeHtml(data.dns)}</code>`;
                if (data.ips && data.ips.length) info += `<br><strong>IPs:</strong> ${escapeHtml(data.ips.join(', '))}`;
                info += `<div class="ts-detail-box-lg">💡 ${t('config.tailscale.tsnet_network_only_hint')}</div>`;
            }
            if (data.homepage_serving && data.homepage_url) {
                info += `<div class="ts-url-row-lg">🏠 <strong>${escapeHtml(t('config.tailscale.tsnet_homepage_url_label'))}:</strong> <a href="${escapeAttr(data.homepage_url)}" target="_blank" rel="noopener noreferrer" class="ts-link">${escapeHtml(data.homepage_url)}</a></div>`;
            } else if (data.expose_homepage) {
                info += `<div class="ts-detail-box-lg">🏠 ${t('config.tailscale.tsnet_homepage_pending_hint')}</div>`;
            }
            if (startBtn) startBtn.classList.add('is-hidden');
        } else if (data.starting) {
            info += `<span class="ts-color-warning">⏳ ${t('config.tailscale.tsnet_status_starting') || 'Waiting for authentication…'}</span>`;
            if (startBtn) startBtn.classList.add('is-hidden');
        } else {
            info += `<span class="ts-color-muted">○ ${t('config.tailscale.tsnet_status_stopped')}</span>`;
            if (data.error) info += `<br><small class="ts-color-error">${escapeHtml(data.error)}</small>`;
            if (startBtn) startBtn.classList.remove('is-hidden');
        }

        if (data.login_url) {
            info += `<div class="ts-login-banner">
                <div class="ts-login-title">🔐 ${t('config.tailscale.tsnet_needs_login')}</div>
                <a href="${escapeAttr(data.login_url)}" target="_blank" rel="noopener noreferrer" class="ts-login-link">${escapeHtml(data.login_url)}</a>
            </div>`;
        }

        el.innerHTML = info;
    } catch (e) {
        el.innerHTML = `<span class="ts-color-error">${t('config.tailscale.tsnet_status_error')}</span>`;
    }
}

async function _tsnetStop() {
    try {
        const resp = await fetch('/api/tsnet/stop', { method: 'POST' });
        const data = await resp.json();
        if (data.error) {
            showToast(data.error, 'error');
        } else {
            showToast(t('config.tailscale.tsnet_stopped_toast'), 'success');
        }
        setTimeout(_tsnetRefreshStatus, 500);
    } catch (e) {
        showToast(e.message, 'error');
    }
}

async function _tsnetStart() {
    try {
        const resp = await fetch('/api/tsnet/start', { method: 'POST' });
        const data = await resp.json();
        if (data.error) {
            showToast(data.error, 'error');
        } else {
            showToast(t('config.tailscale.tsnet_starting_toast') || 'Starting…', 'success');
            let attempts = 0;
            const poll = setInterval(async () => {
                await _tsnetRefreshStatus();
                attempts++;
                if (attempts > 120) clearInterval(poll);
            }, 3000);
        }
    } catch (e) {
        showToast(e.message, 'error');
    }
}

function tsSaveApiKey() {
    const input = document.getElementById('ts-api-key-input');
    const statusEl = document.getElementById('ts-api-key-status');
    const key = input ? input.value.trim() : '';
    if (!key) {
        if (statusEl) { statusEl.className = 'ts-key-status ts-color-error'; statusEl.textContent = t('config.tailscale.key_empty'); }
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'tailscale_api_key', value: key })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            if (statusEl) { statusEl.className = 'ts-key-status ts-color-success'; statusEl.textContent = '✓ ' + t('config.tailscale.key_saved'); }
            if (input) input.value = '';
        } else {
            if (statusEl) { statusEl.className = 'ts-key-status ts-color-error'; statusEl.textContent = '✗ ' + (res.message || t('config.tailscale.key_save_failed')); }
        }
        setTimeout(() => { if (statusEl) { statusEl.className = 'ts-key-status'; statusEl.textContent = ''; } }, 4000);
    })
    .catch(() => {
        if (statusEl) { statusEl.className = 'ts-key-status ts-color-error'; statusEl.textContent = '✗ ' + t('config.tailscale.key_save_failed'); }
    });
}

function tsSaveAuthKey() {
    const input = document.getElementById('ts-auth-key-input');
    const statusEl = document.getElementById('ts-auth-key-status');
    const key = input ? input.value.trim() : '';
    if (!key) {
        if (statusEl) { statusEl.className = 'ts-key-status ts-color-error'; statusEl.textContent = t('config.tailscale.key_empty'); }
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'tailscale_tsnet_authkey', value: key })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            if (statusEl) { statusEl.className = 'ts-key-status ts-color-success'; statusEl.textContent = '✓ ' + t('config.tailscale.key_saved'); }
            if (input) input.value = '';
        } else {
            if (statusEl) { statusEl.className = 'ts-key-status ts-color-error'; statusEl.textContent = '✗ ' + (res.message || t('config.tailscale.key_save_failed')); }
        }
        setTimeout(() => { if (statusEl) { statusEl.className = 'ts-key-status'; statusEl.textContent = ''; } }, 4000);
    })
    .catch(() => {
        if (statusEl) { statusEl.className = 'ts-key-status ts-color-error'; statusEl.textContent = '✗ ' + t('config.tailscale.key_save_failed'); }
    });
}
