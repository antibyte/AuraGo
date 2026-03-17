// cfg/tailscale.js — Tailscale config section (API integration + tsnet DNS)

let _tsSection = null;

async function renderTailscaleSection(section) {
    if (section) _tsSection = section; else section = _tsSection;
    const cfg = configData.tailscale || {};
    const tsnet = cfg.tsnet || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ═══════════════════════════════════════════════════════════════
    // Section 1: Tailscale API Integration
    // ═══════════════════════════════════════════════════════════════

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.tailscale.api_title')}</div>
        <div class="field-group-desc">${t('config.tailscale.api_desc')}</div>`;

    // Enabled toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.tailscale.enabled_label')}</span>
        <div class="toggle ${cfg.enabled ? 'on' : ''}" data-path="tailscale.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    // Read-only toggle
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.tailscale.readonly_label')}</span>
        <div class="toggle ${cfg.readonly ? 'on' : ''}" data-path="tailscale.readonly" onclick="toggleBool(this)"></div>
    </div>`;

    // Tailnet name
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.tailscale.tailnet_label')}</span>
        <input type="text" class="cfg-input" data-path="tailscale.tailnet" value="${escapeAttr(cfg.tailnet || '')}"
            placeholder="example.com" style="width:100%;margin-top:0.2rem;">
    </label>`;

    // API Key vault hint
    html += `<div class="wh-notice" style="margin-top:0.6rem;">
        <span>🔐</span>
        <div><small>${t('config.tailscale.api_key_hint')}</small></div>
    </div>`;

    html += `</div>`;

    // ═══════════════════════════════════════════════════════════════
    // Section 2: tsnet Embedded Node
    // ═══════════════════════════════════════════════════════════════

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.tailscale.tsnet_title')}</div>
        <div class="field-group-desc">${t('config.tailscale.tsnet_desc')}</div>`;

    // tsnet enabled toggle
    const tsEnabled = tsnet.enabled === true;
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.tailscale.tsnet_enabled_label')}</span>
        <div class="toggle ${tsEnabled ? 'on' : ''}" data-path="tailscale.tsnet.enabled" onclick="toggleBool(this);setNestedValue(configData,'tailscale.tsnet.enabled',this.classList.contains('on'));renderTailscaleSection(null)"></div>
    </div>`;

    if (tsEnabled) {
        // Hostname
        html += `<label style="display:block;margin-bottom:0.6rem;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.tailscale.tsnet_hostname_label')}</span>
            <input type="text" class="cfg-input" data-path="tailscale.tsnet.hostname" value="${escapeAttr(tsnet.hostname || 'aurago')}"
                placeholder="aurago" style="width:100%;margin-top:0.2rem;">
            <small style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.tailscale.tsnet_hostname_hint')}</small>
        </label>`;

        // State directory
        html += `<label style="display:block;margin-bottom:0.6rem;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.tailscale.tsnet_state_dir_label')}</span>
            <input type="text" class="cfg-input" data-path="tailscale.tsnet.state_dir" value="${escapeAttr(tsnet.state_dir || '')}"
                placeholder="data/tsnet" style="width:100%;margin-top:0.2rem;">
            <small style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.tailscale.tsnet_state_dir_hint')}</small>
        </label>`;

        // Auth key hint
        html += `<div class="wh-notice" style="margin-top:0.6rem;">
            <span>🔑</span>
            <div><small>${t('config.tailscale.tsnet_auth_key_hint')}</small></div>
        </div>`;

        // ── Status display ──
        html += `<div id="tsnet-status-area" style="margin-top:1rem;">
            <div style="font-weight:500;font-size:0.85rem;margin-bottom:0.4rem;">${t('config.tailscale.tsnet_status_title')}</div>
            <div id="tsnet-status-info" style="padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);font-size:0.85rem;color:var(--text-secondary);">
                ${t('config.tailscale.tsnet_status_loading')}
            </div>
            <div style="display:flex;gap:0.5rem;margin-top:0.6rem;">
                <button class="btn btn-sm btn-secondary" onclick="_tsnetRefreshStatus()">🔄 ${t('config.tailscale.tsnet_btn_refresh')}</button>
                <button class="btn btn-sm btn-warning" onclick="_tsnetStop()">⏹ ${t('config.tailscale.tsnet_btn_stop')}</button>
            </div>
        </div>`;

        // Funnel (V2 placeholder)
        html += `<div style="margin-top:1rem;opacity:0.5;pointer-events:none;">
            <div style="display:flex;align-items:center;gap:0.8rem;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.tailscale.tsnet_funnel_label')}</span>
                <div class="toggle" data-path="tailscale.tsnet.funnel"></div>
            </div>
            <small style="font-size:0.72rem;color:var(--text-tertiary);">${t('config.tailscale.tsnet_funnel_hint')}</small>
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
        if (data.running) {
            info += `<span style="color:var(--success);">● ${t('config.tailscale.tsnet_status_running')}</span>`;
            if (data.dns) info += `<br><strong>DNS:</strong> <code>${escapeHtml(data.dns)}</code>`;
            if (data.ips && data.ips.length) info += `<br><strong>IPs:</strong> ${escapeHtml(data.ips.join(', '))}`;
            if (data.cert_dns && data.cert_dns.length) info += `<br><strong>Cert:</strong> ${escapeHtml(data.cert_dns.join(', '))}`;
        } else {
            info += `<span style="color:var(--text-muted);">○ ${t('config.tailscale.tsnet_status_stopped')}</span>`;
            if (data.error) info += `<br><small style="color:var(--error);">${escapeHtml(data.error)}</small>`;
        }

        el.innerHTML = info;
    } catch (e) {
        el.innerHTML = `<span style="color:var(--error);">${t('config.tailscale.tsnet_status_error')}</span>`;
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
