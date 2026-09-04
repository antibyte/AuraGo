let _tsSection = null;
let _tsnetPollTimer = null;
let _tsnetPollOperationID = '';
let _tsnetPollGeneration = 0;
let _tsnetPollingActive = false;
let _tsnetStatusAbort = null;
let _tsnetStatusPromise = null;

async function renderTailscaleSection(section) {
    _tsnetStopPolling();
    if (section) _tsSection = section; else section = _tsSection;
    const cfg = configData.tailscale || {};
    const tsnet = cfg.tsnet || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.tailscale.api_title')}</div>
        <div class="field-group-desc">${t('config.tailscale.api_desc')}</div>
        <div id="ts-api-status-banner" class="adg-status-banner">${t('config.tailscale.checking')}</div>`;

    html += `<div class="field-group">
        <span class="field-label">${t('config.tailscale.enabled_label')}</span>
        <div class="toggle ${cfg.enabled ? 'on' : ''}" data-path="tailscale.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="field-group">
        <span class="field-label">${t('config.tailscale.readonly_label')}</span>
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="tailscale.readonly" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.tailscale.tailnet_label')}</div>
        <input class="field-input" type="text" data-path="tailscale.tailnet" value="${escapeAttr(cfg.tailnet || '')}" placeholder="example.com">
    </div>`;

    html += `<div class="field-group ts-mt">
        <div class="field-label">🔑 ${t('config.tailscale.api_key_label')}</div>
        <div class="field-help">${t('config.tailscale.api_key_hint')}</div>
        <div class="adg-password-row">
            <div class="password-wrap cfg-password-input">
                <input class="field-input adg-password-input" type="password" id="ts-api-key-input" value="${escapeAttr(cfgSecretValue(cfg.api_key))}" placeholder="${escapeAttr(cfgSecretPlaceholder(cfg.api_key, 'tskey-api-••••••••'))}" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
            <button class="btn-save adg-save-btn" onclick="tsSaveApiKey()">💾 ${t('config.tailscale.save_vault')}</button>
        </div>
        <div id="ts-api-key-status" class="adg-test-result"></div>
    </div>`;

    html += `<div class="cfg-actions-row">
        <button class="btn-save adg-test-btn" onclick="tsApiTestConnection()" id="ts-api-test-btn">🔌 ${t('config.tailscale.test_btn')}</button>
        <span id="ts-api-test-result" class="adg-test-result"></span>
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
        html += `<div class="field-grid two-cols">
            <div class="field-group">
                <div class="field-label">${t('config.tailscale.tsnet_hostname_label')}</div>
                <div class="field-help">${t('config.tailscale.tsnet_hostname_hint')}</div>
                <input type="text" class="field-input" data-path="tailscale.tsnet.hostname" value="${escapeAttr(tsnet.hostname || 'aurago')}" placeholder="aurago">
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.tailscale.tsnet_space_agent_hostname_label')}</div>
                <div class="field-help">${t('config.tailscale.tsnet_space_agent_hostname_hint')}</div>
                <input type="text" class="field-input" data-path="tailscale.tsnet.space_agent_hostname" value="${escapeAttr(tsnet.space_agent_hostname || ((tsnet.hostname || 'aurago') + '-space-agent'))}" placeholder="aurago-space-agent">
            </div>
            <div class="field-group">
                <div class="field-label">${t('config.tailscale.tsnet_state_dir_label')}</div>
                <div class="field-help">${t('config.tailscale.tsnet_state_dir_hint')}</div>
                <input type="text" class="field-input" data-path="tailscale.tsnet.state_dir" value="${escapeAttr(tsnet.state_dir || '')}" placeholder="data/tsnet">
            </div>
        </div>`;

        const serveHTTP = tsnet.serve_http === true;
        const exposeHomepage = tsnet.expose_homepage === true;
        const exposeManifest = tsnet.expose_manifest === true;
        const exposeSpaceAgent = tsnet.expose_space_agent === true;
        const funnel = tsnet.funnel === true;
        const allowHTTPFallback = tsnet.allow_http_fallback === true;
        const homepageCfg = configData.homepage || {};
        const manifestCfg = configData.manifest || {};
        const spaceAgentCfg = configData.space_agent || {};
        html += `<div class="ts-exposure-box">
            <div class="ts-exposure-title">${t('config.tailscale.tsnet_exposure_title')}</div>
            <div class="field-group">
                <span class="field-label">${t('config.tailscale.tsnet_serve_http_label')}</span>
                <div class="toggle ${serveHTTP ? 'on' : ''}" data-path="tailscale.tsnet.serve_http" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.serve_http',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block-mb">${t('config.tailscale.tsnet_serve_http_hint')}</small>

            <div class="field-group">
                <span class="field-label">${t('config.tailscale.tsnet_expose_homepage_label')}</span>
                <div class="toggle ${exposeHomepage ? 'on' : ''}" data-path="tailscale.tsnet.expose_homepage" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.expose_homepage',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block">${t('config.tailscale.tsnet_expose_homepage_hint')}</small>
            ${homepageCfg.webserver_enabled ? '' : `<div class="ts-warning-box">${t('config.tailscale.tsnet_homepage_requires_webserver')}</div>`}

            <div class="field-group">
                <span class="field-label">${t('config.tailscale.tsnet_expose_manifest_label')}</span>
                <div class="toggle ${exposeManifest ? 'on' : ''}" data-path="tailscale.tsnet.expose_manifest" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.expose_manifest',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block">${t('config.tailscale.tsnet_expose_manifest_hint')}</small>
            ${manifestCfg.enabled ? '' : `<div class="ts-warning-box">${t('config.tailscale.tsnet_manifest_requires_enabled')}</div>`}

            <div class="field-group">
                <div class="field-label">${t('config.tailscale.tsnet_manifest_port_label')}</div>
                <div class="field-help">${t('config.tailscale.tsnet_manifest_port_hint')}</div>
                <input type="number" min="1" max="65535" class="field-input" data-path="tailscale.tsnet.manifest_port" value="${escapeAttr(tsnet.manifest_port || 443)}">
            </div>

            <div class="field-group">
                <span class="field-label">${t('config.tailscale.tsnet_expose_space_agent_label')}</span>
                <div class="toggle ${exposeSpaceAgent ? 'on' : ''}" data-path="tailscale.tsnet.expose_space_agent" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.expose_space_agent',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block">${t('config.tailscale.tsnet_expose_space_agent_hint')}</small>
            ${spaceAgentCfg.enabled ? '' : `<div class="ts-warning-box">${t('config.tailscale.tsnet_space_agent_requires_enabled')}</div>`}

            <div class="field-group">
                <span class="field-label">${t('config.tailscale.tsnet_funnel_label')}</span>
                <div class="toggle ${funnel ? 'on' : ''}" data-path="tailscale.tsnet.funnel" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.funnel',this.classList.contains('on'));renderTailscaleSection(null)"></div>
            </div>
            <small class="ts-hint-block">${t('config.tailscale.tsnet_funnel_hint')}</small>
            ${serveHTTP ? '' : `<div class="ts-info-box">${t('config.tailscale.tsnet_funnel_requires_web')}</div>`}

            <div class="field-group">
                <span class="field-label">${t('config.tailscale.tsnet_allow_http_fallback_label')}</span>
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
            <div class="field-label">🔑 ${t('config.tailscale.tsnet_shared_key_label')}</div>
            <div class="field-help">${t('config.tailscale.tsnet_shared_key_hint')}</div>
            <div class="adg-password-row">
                <div class="password-wrap cfg-password-input">
                    <input class="field-input adg-password-input" type="password" id="ts-auth-key-input" value="${escapeAttr(cfgSecretValue(tsnet.auth_key))}" placeholder="${escapeAttr(cfgSecretPlaceholder(tsnet.auth_key, 'tskey-auth-••••••••'))}" autocomplete="off">
                    <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
                </div>
                <button class="btn-save adg-save-btn" onclick="tsSaveAuthKey()">💾 ${t('config.tailscale.save_vault')}</button>
            </div>
            <div id="ts-auth-key-status" class="adg-test-result"></div>
            <div id="tsnet-shared-key-warning"></div>
        </div>`;

        html += `<div id="tsnet-status-area" class="ts-status-area">
            <div class="ts-status-title">${t('config.tailscale.tsnet_status_title')}</div>
            <div id="tsnet-node-cards" class="ts-node-grid"></div>
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

    if (cfg.enabled) {
        tsApiCheckStatus();
    } else {
        tsApiSetBanner('neutral', '⚪ ' + t('config.tailscale.status_disabled'));
    }

    if (tsEnabled) {
        _tsnetRefreshStatus();
    }
}

function tsApiSetBanner(state, text) {
    const banner = document.getElementById('ts-api-status-banner');
    if (!banner) return;
    banner.className = 'adg-status-banner';
    if (state) banner.classList.add('is-' + state);
    banner.textContent = text;
}

function tsApiCheckStatus() {
    tsApiSetBanner('neutral', t('config.tailscale.checking'));
    fetch('/api/tailscale/status')
        .then(r => r.json())
        .then(res => {
            if (res.status === 'disabled') {
                tsApiSetBanner('neutral', '⚪ ' + t('config.tailscale.status_disabled'));
                return;
            }
            if (res.status === 'no_credentials') {
                tsApiSetBanner('warning', '🟡 ' + t('config.tailscale.status_no_credentials'));
                return;
            }
            if (res.status === 'ok') {
                let text = '🟢 ' + t('config.tailscale.status_ok');
                if (typeof res.count === 'number') text += ' (' + res.count + ')';
                tsApiSetBanner('success', text);
                return;
            }
            tsApiSetBanner('danger', '🔴 ' + (res.message || t('config.tailscale.connection_failed')));
        })
        .catch(() => tsApiSetBanner('danger', '🔴 ' + t('config.tailscale.connection_failed')));
}

function tsApiTestConnection() {
    const btn = document.getElementById('ts-api-test-btn');
    const result = document.getElementById('ts-api-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.tailscale.loading');
        result.className = 'adg-test-result';
    }

    fetch('/api/tailscale/test', { method: 'POST' })
        .then(r => r.json())
        .then(res => {
            if (btn) btn.disabled = false;
            if (!result) return;
            if (res.status === 'ok') {
                result.className = 'adg-test-result is-success';
                result.textContent = t('config.tailscale.status_success') + ' ' + t('config.tailscale.test_ok');
                tsApiCheckStatus();
            } else {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.tailscale.status_error') + ' ' + (res.message || t('config.tailscale.test_fail'));
            }
        })
        .catch(() => {
            if (btn) btn.disabled = false;
            if (result) {
                result.className = 'adg-test-result is-danger';
                result.textContent = t('config.tailscale.status_error') + ' ' + t('config.tailscale.test_fail');
            }
        });
}

async function _tsnetRefreshStatus() {
    if (_tsnetStatusPromise) return _tsnetStatusPromise;
    const request = _tsnetFetchAndRenderStatus();
    _tsnetStatusPromise = request;
    try {
        return await request;
    } finally {
        if (_tsnetStatusPromise === request) _tsnetStatusPromise = null;
    }
}

async function _tsnetFetchAndRenderStatus() {
    const el = document.getElementById('tsnet-status-info');
    if (!el) {
        _tsnetStopPolling();
        return null;
    }
    if (!el.dataset.loaded) el.textContent = t('config.tailscale.tsnet_status_loading');
    const controller = new AbortController();
    _tsnetStatusAbort = controller;

    try {
        const resp = await fetch('/api/tsnet/status', { signal: controller.signal });
        const data = await resp.json();
        if (!resp.ok) throw new Error(data.message || t('config.tailscale.tsnet_status_error'));
        _tsnetRenderNodeCards(data);

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
                        ⚠️ ${t('config.tailscale.tsnet_http_fallback_notice')}
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
            if (data.manifest_serving && data.manifest_url) {
                info += `<div class="ts-url-row-lg">▦ <strong>${escapeHtml(t('config.tailscale.tsnet_manifest_url_label'))}:</strong> <a href="${escapeAttr(data.manifest_url)}" target="_blank" rel="noopener noreferrer" class="ts-link">${escapeHtml(data.manifest_url)}</a></div>`;
            } else if (data.expose_manifest) {
                info += `<div class="ts-detail-box-lg">▦ ${t('config.tailscale.tsnet_manifest_pending_hint')}</div>`;
            }
            if (data.space_agent_serving && data.space_agent_url) {
                info += `<div class="ts-url-row-lg">🛰️ <strong>${escapeHtml(t('config.tailscale.tsnet_space_agent_url_label'))}:</strong> <a href="${escapeAttr(data.space_agent_url)}" target="_blank" rel="noopener noreferrer" class="ts-link">${escapeHtml(data.space_agent_url)}</a></div>`;
            } else if (data.expose_space_agent) {
                info += `<div class="ts-detail-box-lg">🛰️ ${t('config.tailscale.tsnet_space_agent_pending_hint')}</div>`;
            }
            if (startBtn) startBtn.classList.add('is-hidden');
        } else if (data.starting) {
            info += `<span class="ts-color-warning">⏳ ${t('config.tailscale.tsnet_status_starting')}</span>`;
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
        if (data.operation) {
            const operationLabel = `${t('config.tailscale.tsnet_operation')}: ${escapeHtml(_tsnetOperationLabel(data.operation.type))}`;
            if (data.operation.state === 'running' || data.operation.state === 'pending') {
                info += `<div class="ts-detail-box-lg">⏳ ${operationLabel}</div>`;
                _tsnetStartPolling(data.operation.id || '');
            } else {
                if (_tsnetPollingActive && _tsnetPollOperationID && data.operation.id === _tsnetPollOperationID) {
                    _tsnetStopPolling(false);
                }
                if (data.operation.state === 'failed') {
                    info += `<div class="ts-warning-detail">⚠️ ${operationLabel}: ${escapeHtml(_tsnetErrorLabel(data.operation.error_code))}</div>`;
                }
            }
        }

        el.innerHTML = info;
        el.dataset.loaded = 'true';
        return data;
    } catch (e) {
        if (e && e.name === 'AbortError') return null;
        el.innerHTML = `<span class="ts-color-error">${t('config.tailscale.tsnet_status_error')}</span>`;
        return null;
    } finally {
        if (_tsnetStatusAbort === controller) _tsnetStatusAbort = null;
    }
}

function _tsnetStartPolling(operationID = '') {
    if (operationID) _tsnetPollOperationID = operationID;
    if (_tsnetPollingActive) return;
    _tsnetPollingActive = true;
    const generation = ++_tsnetPollGeneration;

    const poll = async () => {
        if (!_tsnetPollingActive || generation !== _tsnetPollGeneration) return;
        if (!document.getElementById('tsnet-status-area')) {
            _tsnetStopPolling();
            return;
        }
        const data = await _tsnetRefreshStatus();
        if (!_tsnetPollingActive || generation !== _tsnetPollGeneration) return;

        const operation = data && data.operation;
        if (_tsnetPollOperationID) {
            if (operation && operation.id === _tsnetPollOperationID &&
                operation.state !== 'running' && operation.state !== 'pending') {
                _tsnetStopPolling(false);
                return;
            }
        } else if (operation && (operation.state === 'running' || operation.state === 'pending')) {
            _tsnetPollOperationID = operation.id || '';
        } else if (data) {
            _tsnetStopPolling(false);
            return;
        }
        _tsnetPollTimer = setTimeout(poll, 2000);
    };

    _tsnetPollTimer = setTimeout(poll, 0);
}

function _tsnetStopPolling(abortCurrent = true) {
    _tsnetPollingActive = false;
    _tsnetPollOperationID = '';
    _tsnetPollGeneration += 1;
    if (_tsnetPollTimer) {
        clearTimeout(_tsnetPollTimer);
        _tsnetPollTimer = null;
    }
    if (abortCurrent && _tsnetStatusAbort) {
        _tsnetStatusAbort.abort();
        _tsnetStatusAbort = null;
    }
}

function _tsnetActionsBlocked() {
    return typeof hasUnsavedConfigChanges === 'function' && hasUnsavedConfigChanges();
}

function _tsnetRequireSavedConfig() {
    if (!_tsnetActionsBlocked()) return true;
    showToast(t('config.tailscale.tsnet_save_config_first'), 'warning');
    return false;
}

function _tsnetNodeName(node) {
    const key = {
        main: 'config.tailscale.tsnet_node_main',
        manifest: 'config.tailscale.tsnet_node_manifest',
        space_agent: 'config.tailscale.tsnet_node_space_agent'
    }[node];
    return t(key || 'config.tailscale.tsnet_node_main');
}

function _tsnetHealthLabel(health) {
    const normalized = String(health || 'stopped').replace(/[^a-z_]/g, '');
    return t(`config.tailscale.tsnet_health_${normalized}`);
}

function _tsnetKeySourceLabel(source) {
    const normalized = String(source || 'none').replace(/[^a-z_]/g, '');
    return t(`config.tailscale.tsnet_key_source_${normalized}`);
}

function _tsnetErrorLabel(code) {
    const key = {
        TSNET_AUTH_KEY_MISSING: 'config.tailscale.tsnet_error_auth_key_missing',
        TSNET_AUTH_KEY_REJECTED: 'config.tailscale.tsnet_error_auth_key_rejected',
        TSNET_LOGIN_REQUIRED: 'config.tailscale.tsnet_error_login_required',
        TSNET_NODE_KEY_EXPIRED: 'config.tailscale.tsnet_error_node_key_expired',
        TSNET_STATE_CORRUPT: 'config.tailscale.tsnet_error_state_corrupt',
        TSNET_CERT_UNAVAILABLE: 'config.tailscale.tsnet_error_cert_unavailable',
        TSNET_FUNNEL_UNAVAILABLE: 'config.tailscale.tsnet_error_funnel_unavailable',
        TSNET_TIMEOUT: 'config.tailscale.tsnet_error_timeout',
        TSNET_OPERATION_CONFLICT: 'config.tailscale.tsnet_error_operation_conflict',
        TSNET_NODE_NOT_CONFIGURED: 'config.tailscale.tsnet_error_node_not_configured',
        TSNET_BACKEND_UNAVAILABLE: 'config.tailscale.tsnet_error_backend_unavailable'
    }[String(code || '')];
    return t(key || 'config.tailscale.tsnet_status_error');
}

function _tsnetOperationLabel(type) {
    const key = {
        start: 'config.tailscale.tsnet_btn_start',
        stop: 'config.tailscale.tsnet_btn_stop',
        reconfigure: 'config.tailscale.tsnet_reconfigure',
        reauth_normal: 'config.tailscale.tsnet_reauth',
        reauth_recover_state: 'config.tailscale.tsnet_recover_state'
    }[String(type || '')];
    return t(key || 'config.tailscale.tsnet_operation');
}

function _tsnetRenderNodeCards(data) {
    const container = document.getElementById('tsnet-node-cards');
    if (!container) return;
    const nodes = data.nodes || {};
    const operationRunning = !!(data.operation && (data.operation.state === 'running' || data.operation.state === 'pending'));
    const disabled = operationRunning || _tsnetActionsBlocked();
    const nodeOrder = ['main', 'manifest', 'space_agent'];
    container.innerHTML = nodeOrder.map(node => {
        const state = nodes[node] || {};
        const health = state.health || 'stopped';
        const healthClass = health === 'ready' ? 'ts-color-success' : (health === 'stopped' ? 'ts-color-muted' : 'ts-color-warning');
        const recoveryAllowed = state.error_code === 'TSNET_STATE_CORRUPT';
        const lifecycleDisabled = disabled || state.configured !== true;
        const details = [];
        if (state.backend_state) details.push(`<div><strong>${escapeHtml(t('config.tailscale.tsnet_backend'))}:</strong> ${escapeHtml(state.backend_state)}</div>`);
        if (state.dns) details.push(`<div><strong>DNS:</strong> ${escapeHtml(state.dns)}</div>`);
        if (state.key_expiry) details.push(`<div><strong>${escapeHtml(t('config.tailscale.tsnet_key_expiry'))}:</strong> ${escapeHtml(new Date(state.key_expiry).toLocaleString())}</div>`);
        details.push(`<div><strong>${escapeHtml(t('config.tailscale.tsnet_key_source'))}:</strong> ${escapeHtml(_tsnetKeySourceLabel(state.key_source))}</div>`);
        if (state.error_code) details.push(`<div class="ts-color-error"><strong>${escapeHtml(state.error_code)}</strong> ${escapeHtml(_tsnetErrorLabel(state.error_code))}</div>`);
        if (state.credential_changed) details.push(`<div class="ts-color-warning">${escapeHtml(t('config.tailscale.tsnet_credential_saved_pending'))}</div>`);
        if (state.login_url) details.push(`<div><a href="${escapeAttr(state.login_url)}" target="_blank" rel="noopener noreferrer" class="ts-link">${escapeHtml(t('shared.tsnet.login_banner_link'))}</a></div>`);
        return `<article class="ts-node-card" data-tsnet-node="${escapeAttr(node)}">
            <div class="ts-node-card-head">
                <strong>${escapeHtml(_tsnetNodeName(node))}</strong>
                <span class="${healthClass}">● ${escapeHtml(_tsnetHealthLabel(health))}</span>
            </div>
            <div class="ts-node-details">${details.join('')}</div>
            <div class="adg-password-row ts-node-key-row">
                <div class="password-wrap cfg-password-input">
                    <input class="field-input adg-password-input" type="password" id="tsnet-node-key-${escapeAttr(node)}" placeholder="tskey-auth-••••••••" autocomplete="off" ${disabled ? 'disabled' : ''}>
                    <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)" ${disabled ? 'disabled' : ''}>${EYE_OPEN_SVG}</button>
                </div>
                <button class="btn btn-sm btn-secondary" onclick="_tsnetSaveNodeKey('${escapeAttr(node)}')" ${disabled ? 'disabled' : ''}>💾 ${escapeHtml(t('config.tailscale.tsnet_save_node_key'))}</button>
            </div>
            <div id="tsnet-node-result-${escapeAttr(node)}" class="adg-test-result"></div>
            <div class="ts-btn-row">
                <button class="btn btn-sm btn-success" onclick="_tsnetReauth('${escapeAttr(node)}','normal')" ${lifecycleDisabled ? 'disabled' : ''}>🔐 ${escapeHtml(t('config.tailscale.tsnet_reauth'))}</button>
                <button class="btn btn-sm btn-secondary" onclick="_tsnetDeleteNodeKey('${escapeAttr(node)}')" ${disabled ? 'disabled' : ''}>🗑 ${escapeHtml(t('config.tailscale.tsnet_delete_node_key'))}</button>
                ${recoveryAllowed ? `<button class="btn btn-sm btn-danger" onclick="_tsnetReauth('${escapeAttr(node)}','recover_state')" ${lifecycleDisabled ? 'disabled' : ''}>⚠ ${escapeHtml(t('config.tailscale.tsnet_recover_state'))}</button>` : ''}
            </div>
        </article>`;
    }).join('');

    const sharedWarning = document.getElementById('tsnet-shared-key-warning');
    if (sharedWarning) {
        const sharedCount = nodeOrder.filter(node => nodes[node] && nodes[node].configured && nodes[node].key_source === 'shared_vault').length;
        sharedWarning.innerHTML = sharedCount > 1
            ? `<div class="ts-warning-box">${escapeHtml(t('config.tailscale.tsnet_shared_key_warning'))}</div>`
            : '';
    }
}

async function _tsnetSaveNodeKey(node) {
    if (!_tsnetRequireSavedConfig()) return;
    const input = document.getElementById(`tsnet-node-key-${node}`);
    const result = document.getElementById(`tsnet-node-result-${node}`);
    const authKey = input ? input.value.trim() : '';
    if (!authKey) {
        if (result) result.textContent = t('config.tailscale.key_empty');
        return;
    }
    try {
        const resp = await fetch('/api/tsnet/credentials', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ node, auth_key: authKey })
        });
        const data = await resp.json();
        if (!resp.ok) throw new Error(t('config.tailscale.key_save_failed'));
        if (input) input.value = '';
        if (result) {
            result.className = 'adg-test-result is-success';
            result.textContent = t('config.tailscale.tsnet_node_key_saved');
        }
        await _tsnetRefreshStatus();
    } catch (error) {
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = t('config.tailscale.key_save_failed');
        }
    }
}

async function _tsnetDeleteNodeKey(node) {
    if (!_tsnetRequireSavedConfig()) return;
    if (!confirm(t('config.tailscale.tsnet_delete_key_confirm'))) return;
    const resp = await fetch(`/api/tsnet/credentials?node=${encodeURIComponent(node)}`, { method: 'DELETE' });
    const data = await resp.json();
    if (!resp.ok) {
        showToast(t('config.tailscale.key_save_failed'), 'error');
        return;
    }
    showToast(t('config.tailscale.tsnet_node_key_deleted'), 'success');
    await _tsnetRefreshStatus();
}

async function _tsnetReauth(node, mode) {
    if (!_tsnetRequireSavedConfig()) return;
    const recovering = mode === 'recover_state';
    if (recovering && !confirm(t('config.tailscale.tsnet_recover_confirm'))) return;
    try {
        const resp = await fetch('/api/tsnet/reauth', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ node, mode, confirm_new_identity: recovering })
        });
        const data = await resp.json();
        if (!resp.ok) throw new Error(_tsnetErrorLabel(data.error_code));
        showToast(t(recovering ? 'config.tailscale.tsnet_recovery_started' : 'config.tailscale.tsnet_reauth_started'), 'success');
        _tsnetStartPolling(data.operation_id || '');
    } catch (error) {
        showToast(error.message || t('config.tailscale.tsnet_status_error'), 'error');
    }
}

async function _tsnetStop() {
    if (!_tsnetRequireSavedConfig()) return;
    try {
        const resp = await fetch('/api/tsnet/stop', { method: 'POST' });
        const data = await resp.json();
        if (!resp.ok || data.error) {
            showToast(t('config.tailscale.tsnet_status_error'), 'error');
        } else {
            showToast(t('config.tailscale.tsnet_stopped_toast'), 'success');
        }
        _tsnetStartPolling();
        setTimeout(_tsnetRefreshStatus, 500);
    } catch (e) {
        showToast(t('config.tailscale.tsnet_status_error'), 'error');
    }
}

async function _tsnetStart() {
    if (!_tsnetRequireSavedConfig()) return;
    try {
        const resp = await fetch('/api/tsnet/start', { method: 'POST' });
        const data = await resp.json();
        if (!resp.ok || data.error) {
            showToast(t('config.tailscale.tsnet_status_error'), 'error');
        } else {
            showToast(t('config.tailscale.tsnet_starting_toast'), 'success');
            _tsnetStartPolling(data.operation_id || '');
        }
    } catch (e) {
        showToast(t('config.tailscale.tsnet_status_error'), 'error');
    }
}

function tsSaveApiKey() {
    const input = document.getElementById('ts-api-key-input');
    const statusEl = document.getElementById('ts-api-key-status');
    const key = input ? input.value.trim() : '';
    if (!key) {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.tailscale.key_empty');
        }
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
            if (statusEl) {
                statusEl.className = 'adg-test-result is-success';
                statusEl.textContent = t('config.tailscale.key_saved');
            }
            cfgMarkSecretStored(input, 'tailscale.api_key');
        } else if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = res.message || t('config.tailscale.key_save_failed');
        }
        setTimeout(() => { if (statusEl) { statusEl.className = 'adg-test-result'; statusEl.textContent = ''; } }, 4000);
    })
    .catch(() => {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.tailscale.key_save_failed');
        }
    });
}

function tsSaveAuthKey() {
    if (!_tsnetRequireSavedConfig()) return;
    const input = document.getElementById('ts-auth-key-input');
    const statusEl = document.getElementById('ts-auth-key-status');
    const key = input ? input.value.trim() : '';
    if (!key) {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.tailscale.key_empty');
        }
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
            if (statusEl) {
                statusEl.className = 'adg-test-result is-success';
                statusEl.textContent = t('config.tailscale.key_saved');
            }
            cfgMarkSecretStored(input, 'tailscale.tsnet.auth_key');
            _tsnetRefreshStatus();
        } else if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = res.message || t('config.tailscale.key_save_failed');
        }
        setTimeout(() => { if (statusEl) { statusEl.className = 'adg-test-result'; statusEl.textContent = ''; } }, 4000);
    })
    .catch(() => {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.tailscale.key_save_failed');
        }
    });
}

document.addEventListener('aurago:config-saved', () => {
    if (document.getElementById('tsnet-status-area')) _tsnetRefreshStatus();
});

window.addEventListener('cfg:section-leave', () => _tsnetStopPolling());
window.addEventListener('beforeunload', () => _tsnetStopPolling());
