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
    html += `<div class="rc-toggle-row rc-toggle-row-main">
        <span class="rc-toggle-label-main">${t('config.remote_control.enabled_label')}</span>
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
    html += `<div class="rc-toggle-row rc-toggle-row-main">
        <span class="rc-toggle-label-main">${t('config.remote_control.read_only_label')}</span>
        <div class="toggle ${readOnly ? 'on' : ''}" data-path="remote_control.read_only" onclick="toggleBool(this)"></div>
    </div>`;

    // ── Network Settings ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.network_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.network_desc')}</div>`;

    // Discovery Port
    const curPort = cfg.discovery_port || 8092;
    html += `<label class="rc-label-block">
        <span class="rc-label-text">${t('config.remote_control.discovery_port_label')}</span>
        <input type="number" class="cfg-input rc-input-spaced" data-path="remote_control.discovery_port" value="${curPort}" min="1024" max="65535">
    </label>`;
    html += `</div>`;

    // ── Security ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.security_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.security_desc')}</div>`;

    // Auto Approve
    const autoApprove = cfg.auto_approve === true;
    html += `<div class="rc-toggle-row">
        <span class="rc-label-text">${t('config.remote_control.auto_approve_label')}</span>
        <div class="toggle ${autoApprove ? 'on' : ''}" data-path="remote_control.auto_approve" onclick="toggleBool(this)"></div>
    </div>`;

    // Audit Log
    const auditLog = cfg.audit_log !== false;
    html += `<div class="rc-toggle-row">
        <span class="rc-label-text">${t('config.remote_control.audit_log_label')}</span>
        <div class="toggle ${auditLog ? 'on' : ''}" data-path="remote_control.audit_log" onclick="toggleBool(this)"></div>
    </div>`;

    // SSH Insecure Host Key
    const sshInsecure = cfg.ssh_insecure_host_key === true;
    html += `<div class="rc-toggle-row rc-toggle-insecure${sshInsecure ? ' is-on' : ''}">
        <span class="rc-label-text">${t('config.remote_control.ssh_insecure_host_key_label')}</span>
        <div class="toggle ${sshInsecure ? 'on' : ''}" data-path="remote_control.ssh_insecure_host_key" onclick="toggleBool(this)"></div>
    </div>`;
    html += `</div>`;

    // ── Limits ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.limits_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.limits_desc')}</div>`;

    // Max File Size MB
    const curMaxFile = cfg.max_file_size_mb || 50;
    html += `<label class="rc-label-block">
        <span class="rc-label-text">${t('config.remote_control.max_file_size_label')}</span>
        <input type="number" class="cfg-input rc-input-spaced" data-path="remote_control.max_file_size_mb" value="${curMaxFile}" min="1" max="500">
    </label>`;

    // Allowed Paths
    const curPaths = (cfg.allowed_paths || []).join('\n');
    html += `<label class="rc-label-block">
        <span class="rc-label-text">${t('config.remote_control.allowed_paths_label')} <small class="rc-label-hint">(${t('config.remote_control.allowed_paths_hint')})</small></span>
        <textarea class="cfg-input rc-input-spaced rc-paths-textarea" data-path="remote_control.allowed_paths" data-type="array-lines" rows="3"
            onchange="setNestedValue(configData,'remote_control.allowed_paths',this.value.split('\\n').filter(l=>l.trim()));setDirty(true)">${escapeAttr(curPaths)}</textarea>
    </label>`;
    html += `</div>`;

    // ── Client Download ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.download_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.download_desc')}</div>
        <div class="rc-download-name-wrap">
            <label class="rc-download-name-label">
                <span class="rc-label-text">${t('config.remote_control.download_name_label')}</span>
                <input type="text" id="rc-device-name" class="cfg-input rc-input-spaced" placeholder="${t('config.remote_control.download_name_placeholder')}">
            </label>
        </div>
        <div id="rc-platform-list" class="rc-platform-list">
            <span class="rc-muted-text">${t('config.remote_control.loading_platforms')}</span>
        </div>
    </div>`;

    // ── Connected Devices (live status) ──
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.remote_control.devices_title')}</div>
        <div class="field-group-desc">${t('config.remote_control.devices_desc')}</div>
        <div id="rc-devices-list" class="rc-devices-list">
            <span class="rc-muted-text">${t('config.remote_control.loading_devices')}</span>
        </div>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    // Load platform list and device list async
    loadRemotePlatforms();
    loadRemoteDevices();
}

async function loadRemotePlatforms() {
    const container = document.getElementById('rc-platform-list');
    if (!container) return;
    try {
        const resp = await fetch('/api/remote/platforms');
        if (!resp.ok) {
            container.innerHTML = `<span class="rc-error-text">${t('config.remote_control.platforms_error')}</span>`;
            return;
        }
        const platforms = await resp.json();
        const available = platforms.filter(p => p.available);
        if (!available.length) {
            container.innerHTML = `<span class="rc-muted-text">${t('config.remote_control.no_platforms')}</span>`;
            return;
        }

        const osIcons = { linux: '🐧', darwin: '🍎', windows: '🪟' };
        let html = '<div class="rc-platform-grid">';
        available.forEach(p => {
            const icon = osIcons[p.os] || '💻';
            const label = `${icon} ${p.os} / ${p.arch}`;
            html += `<button class="btn-save rc-platform-btn"
                onclick="rcDownload('${escapeAttr(p.os)}','${escapeAttr(p.arch)}')">
                ⬇ ${label}
            </button>`;
        });
        html += '</div>';
        container.innerHTML = html;
    } catch (e) {
        container.innerHTML = `<span class="rc-error-text">${t('config.remote_control.error_prefix')} ${e.message}</span>`;
    }
}

function rcDownload(os, arch) {
    const nameEl = document.getElementById('rc-device-name');
    const name = nameEl ? nameEl.value.trim() : '';
    const qs = name ? `?name=${encodeURIComponent(name)}` : '';
    window.location.href = `/api/remote/download/${os}/${arch}${qs}`;
}

async function loadRemoteDevices() {
    const container = document.getElementById('rc-devices-list');
    if (!container) return;
    try {
        const resp = await fetch('/api/remote/devices');
        if (!resp.ok) {
            container.innerHTML = `<span class="rc-muted-text">${t('config.remote_control.no_devices')}</span>`;
            return;
        }
        const devices = await resp.json();
        if (!devices || devices.length === 0) {
            container.innerHTML = `<span class="rc-muted-text">${t('config.remote_control.no_devices')}</span>`;
            return;
        }
        let html = '<div class="rc-device-list">';
        devices.forEach(d => {
            const statusIcon = d.is_connected ? '🟢' : '⚪';
            const statusText = d.is_connected ? t('config.remote_control.status_connected') : t('config.remote_control.status_offline');
            html += `<div class="rc-device-item">
                <span>${statusIcon}</span>
                <div class="rc-device-meta">
                    <div class="rc-device-name">${escapeAttr(d.name || d.hostname || d.id)}</div>
                    <div class="rc-device-subline">${escapeAttr(d.os || '')} ${escapeAttr(d.arch || '')} — ${escapeAttr(d.ip_address || '')}</div>
                </div>
                <span class="rc-device-status ${d.is_connected ? 'is-connected' : ''}">${statusText}</span>
            </div>`;
        });
        html += '</div>';
        container.innerHTML = html;
    } catch (e) {
        container.innerHTML = `<span class="rc-error-text">${t('config.remote_control.error_prefix')} ${e.message}</span>`;
    }
}
