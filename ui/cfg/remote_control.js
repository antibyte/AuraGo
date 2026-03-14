// cfg/remote_control.js — Remote Control config section module

let _rcSection = null;

async function renderRemoteControlSection(section) {
    if (section) _rcSection = section; else section = _rcSection;
    const cfg = configData.remote_control || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.remote_control.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="remote_control.enabled" onclick="toggleBool(this);setNestedValue(configData,'remote_control.enabled',this.classList.contains('on'));renderRemoteControlSection(null)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>📡</span>
            <div>
                <strong>${t('config.remote_control.disabled_notice')}</strong><br>
                <small>${t('config.remote_control.disabled_desc')}</small>
            </div>
        </div>`;
        html += `</div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    // ── Read-Only toggle ──
    const readOnly = cfg.read_only === true;
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.remote_control.read_only_label')}</span>
        <div class="toggle ${readOnly ? 'on' : ''}" data-path="remote_control.read_only" onclick="toggleBool(this)"></div>
    </div>`;

    // ── Network Settings ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.network_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.network_desc')}</div>`;

    // Discovery Port
    const curPort = cfg.discovery_port || 8092;
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.remote_control.discovery_port_label')}</span>
        <input type="number" class="cfg-input" data-path="remote_control.discovery_port" value="${curPort}" min="1024" max="65535"
            style="width:100%;margin-top:0.2rem;">
    </label>`;
    html += `</div>`;

    // ── Security ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.security_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.security_desc')}</div>`;

    // Auto Approve
    const autoApprove = cfg.auto_approve === true;
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.remote_control.auto_approve_label')}</span>
        <div class="toggle ${autoApprove ? 'on' : ''}" data-path="remote_control.auto_approve" onclick="toggleBool(this)"></div>
    </div>`;

    // Audit Log
    const auditLog = cfg.audit_log !== false;
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.remote_control.audit_log_label')}</span>
        <div class="toggle ${auditLog ? 'on' : ''}" data-path="remote_control.audit_log" onclick="toggleBool(this)"></div>
    </div>`;
    html += `</div>`;

    // ── Limits ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.limits_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.limits_desc')}</div>`;

    // Max File Size MB
    const curMaxFile = cfg.max_file_size_mb || 50;
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.remote_control.max_file_size_label')}</span>
        <input type="number" class="cfg-input" data-path="remote_control.max_file_size_mb" value="${curMaxFile}" min="1" max="500"
            style="width:100%;margin-top:0.2rem;">
    </label>`;

    // Allowed Paths
    const curPaths = (cfg.allowed_paths || []).join('\n');
    html += `<label style="display:block;margin-bottom:0.6rem;">
        <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.remote_control.allowed_paths_label')} <small style="color:var(--text-tertiary);">(${t('config.remote_control.allowed_paths_hint')})</small></span>
        <textarea class="cfg-input" data-path="remote_control.allowed_paths" rows="3"
            style="width:100%;margin-top:0.2rem;font-family:monospace;font-size:0.8rem;"
            onchange="setNestedValue(configData,'remote_control.allowed_paths',this.value.split('\\n').filter(l=>l.trim()));setDirty(true)">${escapeAttr(curPaths)}</textarea>
    </label>`;
    html += `</div>`;

    // ── Connected Devices (live status) ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.devices_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.devices_desc')}</div>
        <div id="rc-devices-list" style="margin-top:0.5rem;">
            <span style="font-size:0.8rem;color:var(--text-tertiary);">${t('config.remote_control.loading_devices')}</span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Load device list async
    loadRemoteDevices();
}

async function loadRemoteDevices() {
    const container = document.getElementById('rc-devices-list');
    if (!container) return;
    try {
        const resp = await fetch('/api/remote/devices');
        if (!resp.ok) {
            container.innerHTML = `<span style="color:var(--text-tertiary);font-size:0.8rem;">${t('config.remote_control.no_devices')}</span>`;
            return;
        }
        const devices = await resp.json();
        if (!devices || devices.length === 0) {
            container.innerHTML = `<span style="color:var(--text-tertiary);font-size:0.8rem;">${t('config.remote_control.no_devices')}</span>`;
            return;
        }
        let html = '<div style="display:flex;flex-direction:column;gap:0.5rem;">';
        devices.forEach(d => {
            const statusColor = d.is_connected ? 'var(--success)' : 'var(--text-tertiary)';
            const statusIcon = d.is_connected ? '🟢' : '⚪';
            const statusText = d.is_connected ? t('config.remote_control.status_connected') : t('config.remote_control.status_offline');
            html += `<div style="display:flex;align-items:center;gap:0.8rem;padding:0.5rem 0.8rem;border-radius:6px;background:var(--bg-tertiary);">
                <span>${statusIcon}</span>
                <div style="flex:1;">
                    <div style="font-size:0.85rem;font-weight:500;">${escapeAttr(d.name || d.hostname || d.id)}</div>
                    <div style="font-size:0.72rem;color:var(--text-tertiary);">${escapeAttr(d.os || '')} ${escapeAttr(d.arch || '')} — ${escapeAttr(d.ip_address || '')}</div>
                </div>
                <span style="font-size:0.75rem;color:${statusColor};">${statusText}</span>
            </div>`;
        });
        html += '</div>';
        container.innerHTML = html;
    } catch (e) {
        container.innerHTML = `<span style="color:var(--danger);font-size:0.8rem;">Error: ${e.message}</span>`;
    }
}
