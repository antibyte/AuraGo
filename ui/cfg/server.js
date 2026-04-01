// cfg/server.js — Server config section with HTTPS certificate modes

let _srvSection = null;

async function renderServerSection(section) {
    if (section) _srvSection = section; else section = _srvSection;
    const cfg = configData.server || {};
    const https = cfg.https || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ═══════════════════════════════════════════════════════════════
    // Section 1: General Server Settings
    // ═══════════════════════════════════════════════════════════════

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.server.general_title')}</div>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.host_label')}</span>
        <input type="text" class="cfg-input" data-path="server.host" value="${escapeAttr(cfg.host || '')}"
            placeholder="0.0.0.0" style="width:100%;margin-top:0.2rem;">
    </label>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.port_label')}</span>
        <input type="number" class="cfg-input" data-path="server.port" value="${cfg.port || 3000}" min="1" max="65535"
            style="width:100%;margin-top:0.2rem;">
    </label>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.bridge_address_label')}</span>
        <input type="text" class="cfg-input" data-path="server.bridge_address" value="${escapeAttr(cfg.bridge_address || '')}"
            placeholder="" style="width:100%;margin-top:0.2rem;">
        <small style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.server.bridge_address_hint')}</small>
    </label>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.ui_language_label')}</span>
        <select class="cfg-input" data-path="server.ui_language" style="width:100%;margin-top:0.2rem;">
            ${['de','en','es','fr','it','pt','nl','pl','zh','ja','hi','da','sv','no','cs','el'].map(l =>
                `<option value="${l}" ${(cfg.ui_language||'de')===l?'selected':''}>${l.toUpperCase()}</option>`
            ).join('')}
        </select>
    </label>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.oauth_redirect_label')}</span>
        <input type="text" class="cfg-input" data-path="server.oauth_redirect_base_url" value="${escapeAttr(cfg.oauth_redirect_base_url || '')}"
            placeholder="https://aurago.example.com" style="width:100%;margin-top:0.2rem;">
        <small style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.server.oauth_redirect_hint')}</small>
    </label>`;

    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.max_body_bytes_label')}</span>
        <input type="number" class="cfg-input" data-path="server.max_body_bytes" value="${cfg.max_body_bytes || 0}" min="0"
            style="width:100%;margin-top:0.2rem;">
        <small style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.server.max_body_bytes_hint')}</small>
    </label>`;

    html += `</div>`;

    // ═══════════════════════════════════════════════════════════════
    // Section 2: HTTPS / TLS Configuration
    // ═══════════════════════════════════════════════════════════════

    html += `<div class="field-group">
        <div class="field-group-title">🔒 ${t('config.server.https_title')}</div>
        <div class="field-group-desc">${t('config.server.https_desc')}</div>`;

    // HTTPS enabled toggle
    const httpsEnabled = https.enabled === true;
    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.server.https_enabled_label')}</span>
        <div class="toggle ${httpsEnabled ? 'on' : ''}" data-path="server.https.enabled" onclick="_srvToggleHttps(this)"></div>
    </div>`;

    if (httpsEnabled) {
        // Certificate mode selector
        const certMode = https.cert_mode || 'auto';
        html += `<label style="display:block;margin-bottom:0.8rem;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.cert_mode_label')}</span>
            <select class="cfg-input" id="cert-mode-select" data-path="server.https.cert_mode" style="width:100%;margin-top:0.2rem;"
                onchange="_srvChangeCertMode(this)">
                <option value="auto" ${certMode==='auto'?'selected':''}>${t('config.server.cert_mode_auto')}</option>
                <option value="custom" ${certMode==='custom'?'selected':''}>${t('config.server.cert_mode_custom')}</option>
                <option value="selfsigned" ${certMode==='selfsigned'?'selected':''}>${t('config.server.cert_mode_selfsigned')}</option>
            </select>
        </label>`;

        // Ports (always visible)
        html += `<div style="display:flex;gap:1rem;margin-bottom:0.8rem;">
            <label style="flex:1;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.https_port_label')}</span>
                <input type="number" class="cfg-input" data-path="server.https.https_port" value="${https.https_port || 443}" min="1" max="65535"
                    style="width:100%;margin-top:0.2rem;">
            </label>
            <label style="flex:1;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.http_port_label')}</span>
                <input type="number" class="cfg-input" data-path="server.https.http_port" value="${https.http_port || 80}" min="1" max="65535"
                    style="width:100%;margin-top:0.2rem;">
            </label>
        </div>`;

        // Behind proxy toggle
        html += `<div class="cfg-toggle-row">
            <span class="cfg-toggle-label">${t('config.server.behind_proxy_label')}</span>
            <div class="toggle ${https.behind_proxy ? 'on' : ''}" data-path="server.https.behind_proxy" onclick="toggleBool(this)"></div>
        </div>`;

        // ── Mode-specific fields ──
        if (certMode === 'auto') {
            // Let's Encrypt: domain + email
            html += `<div class="wh-notice" style="margin-bottom:0.8rem;">
                <span>🌐</span>
                <div><small>${t('config.server.cert_auto_notice')}</small></div>
            </div>`;

            html += `<label style="display:block;margin-bottom:0.6rem;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.domain_label')}</span>
                <input type="text" class="cfg-input" data-path="server.https.domain" value="${escapeAttr(https.domain || '')}"
                    placeholder="aurago.example.com" style="width:100%;margin-top:0.2rem;">
            </label>`;

            html += `<label style="display:block;margin-bottom:0.6rem;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.email_label')}</span>
                <input type="email" class="cfg-input" data-path="server.https.email" value="${escapeAttr(https.email || '')}"
                    placeholder="admin@example.com" style="width:100%;margin-top:0.2rem;">
            </label>`;

        } else if (certMode === 'custom') {
            // Custom cert: file paths + upload
            html += `<div class="wh-notice" style="margin-bottom:0.8rem;">
                <span>📄</span>
                <div><small>${t('config.server.cert_custom_notice')}</small></div>
            </div>`;

            html += `<label style="display:block;margin-bottom:0.6rem;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.cert_file_label')}</span>
                <input type="text" class="cfg-input" data-path="server.https.cert_file" value="${escapeAttr(https.cert_file || '')}"
                    placeholder="data/certs/custom.crt" style="width:100%;margin-top:0.2rem;">
            </label>`;

            html += `<label style="display:block;margin-bottom:0.6rem;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.key_file_label')}</span>
                <input type="text" class="cfg-input" data-path="server.https.key_file" value="${escapeAttr(https.key_file || '')}"
                    placeholder="data/certs/custom.key" style="width:100%;margin-top:0.2rem;">
            </label>`;

            // Upload form
            html += `<div style="margin-top:0.6rem;padding:0.8rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
                <div style="font-weight:500;font-size:0.85rem;margin-bottom:0.5rem;">${t('config.server.cert_upload_title')}</div>
                <div style="display:flex;gap:0.5rem;flex-direction:column;">
                    <label style="display:block;">
                        <span style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.server.cert_upload_cert')}</span>
                        <input type="file" id="cert-upload-cert" accept=".pem,.crt,.cer" style="width:100%;margin-top:0.2rem;font-size:0.8rem;">
                    </label>
                    <label style="display:block;">
                        <span style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.server.cert_upload_key')}</span>
                        <input type="file" id="cert-upload-key" accept=".pem,.key" style="width:100%;margin-top:0.2rem;font-size:0.8rem;">
                    </label>
                    <button class="btn btn-sm btn-primary" onclick="_srvUploadCert()" style="margin-top:0.3rem;">📤 ${t('config.server.cert_upload_btn')}</button>
                </div>
            </div>`;

        } else if (certMode === 'selfsigned') {
            // Self-signed: domain + regenerate button
            html += `<div class="wh-notice" style="margin-bottom:0.8rem;">
                <span>🔐</span>
                <div><small>${t('config.server.cert_selfsigned_notice')}</small></div>
            </div>`;
            // Warn when port 443 is selected — requires root/admin on Linux
            const httpsPort = parseInt(https.https_port) || 443;
            if (httpsPort < 1024) {
                html += `<div class="wh-notice" style="margin-bottom:0.8rem;background:var(--warning-bg,#3d2e00);border-color:var(--warning,#f9a825);">
                    <span>⚠️</span>
                    <div><small>${t('config.server.selfsigned_port_warning') || 'Port ' + httpsPort + ' requires root/admin privileges on Linux. For local use without root, consider port 8443.'}</small></div>
                </div>`;
            }

            html += `<label style="display:block;margin-bottom:0.6rem;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.server.domain_label')}</span>
                <input type="text" class="cfg-input" data-path="server.https.domain" value="${escapeAttr(https.domain || '')}"
                    placeholder="aurago.local" style="width:100%;margin-top:0.2rem;">
                <small style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.server.cert_selfsigned_domain_hint')}</small>
            </label>`;

            html += `<button class="btn btn-sm btn-warning" onclick="_srvRegenCert()" style="margin-top:0.3rem;">🔄 ${t('config.server.cert_regen_btn')}</button>`;
        }

        // ── Certificate Status (all modes) ──
        html += `<div id="cert-status-area" style="margin-top:1rem;">
            <div style="font-weight:500;font-size:0.85rem;margin-bottom:0.4rem;">${t('config.server.cert_status_title')}</div>
            <div id="cert-status-info" style="padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);font-size:0.85rem;color:var(--text-secondary);">
                ${t('config.server.cert_status_loading')}
            </div>
            <button class="btn btn-sm btn-secondary" onclick="_srvRefreshCertStatus()" style="margin-top:0.4rem;">🔄 ${t('config.server.cert_status_refresh')}</button>
        </div>`;

    } else {
        html += `<div class="wh-notice">
            <span>🔓</span>
            <div>
                <strong>${t('config.server.https_disabled_notice')}</strong><br>
                <small>${t('config.server.https_disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `</div>`;
    html += `</div>`;

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    if (httpsEnabled) {
        _srvRefreshCertStatus();
    }
}

function _srvSyncFormState() {
    const content = document.getElementById('content');
    if (!content) return;
    content.querySelectorAll('[data-path]').forEach(el => {
        const path = el.dataset.path;
        if (!path || path.startsWith('server.https.enabled')) return;
        let value;
        if (el.tagName === 'SELECT') {
            value = el.value;
        } else if (el.type === 'number') {
            value = el.value === '' ? '' : Number(el.value);
        } else {
            value = el.value;
        }
        setNestedValue(configData, path, value);
    });
}

function _srvToggleHttps(toggle) {
    _srvSyncFormState();
    toggleBool(toggle);
    setNestedValue(configData, 'server.https.enabled', toggle.classList.contains('on'));
    markDirty();
    renderServerSection(null);
}

function _srvChangeCertMode(select) {
    _srvSyncFormState();
    setNestedValue(configData, 'server.https.cert_mode', select.value);
    markDirty();
    renderServerSection(null);
}

async function _srvRefreshCertStatus() {
    const el = document.getElementById('cert-status-info');
    if (!el) return;
    el.textContent = t('config.server.cert_status_loading');

    try {
        const resp = await fetch('/api/cert/status');
        const data = await resp.json();

        let info = '';
        info += `<strong>${t('config.server.cert_status_mode')}:</strong> ${escapeHtml(data.mode || '---')}`;
        if (data.domain) info += `<br><strong>${t('config.server.cert_status_domain')}:</strong> ${escapeHtml(data.domain)}`;

        if (data.cert_info) {
            const ci = data.cert_info;
            info += `<br><strong>${t('config.server.cert_status_subject')}:</strong> ${escapeHtml(ci.subject || '---')}`;
            info += `<br><strong>${t('config.server.cert_status_issuer')}:</strong> ${escapeHtml(ci.issuer || '---')}`;
            info += `<br><strong>${t('config.server.cert_status_expires')}:</strong> ${escapeHtml(ci.not_after || '---')}`;
            if (ci.dns_names && ci.dns_names.length) {
                info += `<br><strong>SANs:</strong> ${escapeHtml(ci.dns_names.join(', '))}`;
            }
            if (ci.expired) {
                info += `<br><span style="color:var(--error);font-weight:bold;">⚠ ${t('config.server.cert_status_expired')}</span>`;
            }
        } else {
            info += `<br><span style="color:var(--text-muted);">${t('config.server.cert_status_none')}</span>`;
        }

        el.innerHTML = info;
    } catch (e) {
        el.innerHTML = `<span style="color:var(--error);">${t('config.server.cert_status_error')}</span>`;
    }
}

async function _srvUploadCert() {
    const certFile = document.getElementById('cert-upload-cert')?.files?.[0];
    const keyFile = document.getElementById('cert-upload-key')?.files?.[0];

    if (!certFile || !keyFile) {
        showToast(t('config.server.cert_upload_missing'), 'error');
        return;
    }

    const form = new FormData();
    form.append('cert', certFile);
    form.append('key', keyFile);

    try {
        const resp = await fetch('/api/cert/upload', { method: 'POST', body: form });
        const data = await resp.json();
        if (data.error) {
            showToast(data.error, 'error');
        } else {
            showToast(data.message || t('config.server.cert_upload_success'), 'success');
            setTimeout(_srvRefreshCertStatus, 500);
        }
    } catch (e) {
        showToast(e.message, 'error');
    }
}

async function _srvRegenCert() {
    try {
        const resp = await fetch('/api/cert/regenerate', { method: 'POST' });
        const data = await resp.json();
        if (data.error) {
            showToast(data.error, 'error');
        } else {
            showToast(data.message || t('config.server.cert_regen_success'), 'success');
            setTimeout(_srvRefreshCertStatus, 500);
        }
    } catch (e) {
        showToast(e.message, 'error');
    }
}
