let sandboxStatusCache = null;
let shellSandboxStatusCache = null;

async function renderSandboxSection(section) {
    if (sandboxStatusCache === null) {
        try {
            const resp = await fetch('/api/sandbox/status');
            sandboxStatusCache = resp.ok ? await resp.json() : {};
        } catch (_) { sandboxStatusCache = {}; }
    }
    const sbCfg = configData.sandbox || {};
    const sbEnabled = sbCfg.enabled === true;
    const st = sandboxStatusCache || {};

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    const sbBanner = featureUnavailableBanner('sandbox');
    if (sbBanner) html += sbBanner;
    const sbBlocked = (typeof runtimeStatusKnown === 'function' && !runtimeStatusKnown())
        || !!(runtimeData.features && runtimeData.features.sandbox && !runtimeData.features.sandbox.available);

    if (sbBlocked) html += '<div class="feature-unavailable-fields">';

    // ── General Card ──
    html += `<div class="sb-card">
        <div class="sb-card-header">
            <span class="sb-card-icon">⚙️</span>
            <span class="sb-card-title">${t('config.sandbox.card_general')}</span>
        </div>
        <div class="sb-card-body">`;

    if (sbEnabled && st.ready) {
        const langList = (st.languages || []).join(', ') || 'python';
        html += `<div class="wh-notice sb-notice-success">
            <span>✅</span>
            <div>
                <strong>${t('config.sandbox.active')}</strong><br>
                <small>${t('config.sandbox.backend_label')}: ${escapeAttr(st.backend || 'docker')} · ${t('config.sandbox.languages')}: ${escapeAttr(langList)}</small>
            </div>
        </div>`;
    } else if (sbEnabled && !st.ready) {
        html += `<div class="wh-notice sb-notice-warning">
            <span>⚠️</span>
            <div>
                <strong>${t('config.sandbox.not_ready')}</strong><br>
                <small>${escapeAttr(st.error || t('config.sandbox.unknown_error'))}</small>
            </div>
        </div>`;
    }

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.sandbox.enabled_label')}</span>
        <div class="toggle ${sbEnabled ? 'on' : ''}" data-path="sandbox.enabled" aria-describedby="sb-enabled-help" onclick="toggleBool(this)"></div>
    </div>
    <div class="field-help" id="sb-enabled-help">${t('help.sandbox.enabled')}</div>`;

    if (!sbEnabled) {
        html += `<div class="wh-notice">
            <span>📦</span>
            <div>
                <strong>${t('config.sandbox.disabled_notice')}</strong><br>
                <small>${t('config.sandbox.disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `</div></div>`; // close sb-card-body & sb-card (General)

    // ── Backend Configuration Card ──
    html += `<div class="sb-card">
        <div class="sb-card-header">
            <span class="sb-card-icon">🐳</span>
            <span class="sb-card-title">${t('config.sandbox.card_backend')}</span>
        </div>
        <div class="sb-card-body">`;

    html += `<div class="sb-grid-2col">`;

    html += `<label class="sb-field-label">
        <span class="sb-field-caption">${t('config.sandbox.backend_label')}</span>
        <select class="field-select sb-field-input" data-path="sandbox.backend" aria-describedby="sb-help-sandbox-backend" onchange="setNestedValue(configData,'sandbox.backend',this.value);setDirty(true)">
            <option value="docker" ${sbCfg.backend === 'docker' || !sbCfg.backend ? 'selected' : ''}>${t('config.sandbox.backend_docker')}</option>
            <option value="podman" ${sbCfg.backend === 'podman' ? 'selected' : ''}>${t('config.sandbox.backend_podman')}</option>
        </select>
        <small class="field-help sb-field-hint" id="sb-help-sandbox-backend">${t('help.sandbox.backend')}</small>
    </label>`;

    html += `<label class="sb-field-label">
        <span class="sb-field-caption">${t('config.sandbox.image_label')}</span>
        <input class="field-input sb-field-input" data-path="sandbox.image" aria-describedby="sb-help-sandbox-image" value="${escapeAttr(sbCfg.image || 'python:3.11-slim')}"
            onchange="setNestedValue(configData,'sandbox.image',this.value);setDirty(true)">
        <small class="field-help sb-field-hint" id="sb-help-sandbox-image">${t('help.sandbox.image')}</small>
    </label>`;

    html += `<label class="sb-field-label">
        <span class="sb-field-caption">${t('config.sandbox.docker_host_label')} <small class="sb-field-hint">(${t('config.sandbox.empty_auto')})</small></span>
        <input class="field-input sb-field-input" data-path="sandbox.docker_host" aria-describedby="sb-help-sandbox-docker-host" value="${escapeAttr(sbCfg.docker_host || '')}" placeholder="unix:///var/run/docker.sock"
            onchange="setNestedValue(configData,'sandbox.docker_host',this.value);setDirty(true)">
        <small class="field-help sb-field-hint" id="sb-help-sandbox-docker-host">${t('help.sandbox.docker_host')}</small>
    </label>`;

    html += `<label class="sb-field-label">
        <span class="sb-field-caption">${t('config.sandbox.timeout_label_full')}</span>
        <input type="number" class="field-input sb-field-input" data-path="sandbox.timeout_seconds" aria-describedby="sb-help-sandbox-timeout-seconds" value="${sbCfg.timeout_seconds || 30}" min="5" max="300"
            onchange="setNestedValue(configData,'sandbox.timeout_seconds',parseInt(this.value)||30);setDirty(true)">
        <small class="field-help sb-field-hint" id="sb-help-sandbox-timeout-seconds">${t('help.sandbox.timeout_seconds')}</small>
    </label>`;

    html += `<label class="sb-field-label">
        <span class="sb-field-caption">${t('config.sandbox.pool_size_label')} <small class="sb-field-hint">${t('config.sandbox.pool_size_unavailable')}</small></span>
        <input type="number" class="field-input sb-field-input" data-path="sandbox.pool_size" aria-describedby="sb-help-sandbox-pool-size" value="${sbCfg.pool_size || 0}" min="0" max="20" disabled
            onchange="setNestedValue(configData,'sandbox.pool_size',parseInt(this.value)||0);setDirty(true)">
        <small class="field-help sb-field-hint" id="sb-help-sandbox-pool-size">${t('help.sandbox.pool_size')}</small>
    </label>`;

    html += `</div>`;

    html += `</div></div>`; // close sb-card-body & sb-card (Backend)

    // ── Options Card ──
    html += `<div class="sb-card">
        <div class="sb-card-header">
            <span class="sb-card-icon">🔧</span>
            <span class="sb-card-title">${t('config.sandbox.card_options')}</span>
        </div>
        <div class="sb-card-body">`;

    html += `<div class="sb-grid-toggle">`;

    html += `<div class="field-group"><div class="cfg-toggle-row-compact">
        <div class="toggle ${sbCfg.auto_install === true || sbCfg.auto_install === undefined ? 'on' : ''}" data-path="sandbox.auto_install" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.sandbox.auto_install_label')}</span>
    </div>
    <div class="field-help">${t('help.sandbox.auto_install')}</div></div>`;

    html += `<div class="field-group"><div class="cfg-toggle-row-compact">
        <div class="toggle ${sbCfg.network_enabled ? 'on' : ''}" data-path="sandbox.network_enabled" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.sandbox.network_label')}</span>
    </div>
    <div class="field-help">${t('help.sandbox.network_enabled')}</div></div>`;

    html += `<div class="field-group"><div class="cfg-toggle-row-compact">
        <div class="toggle ${sbCfg.allow_local_python_fallback ? 'on' : ''}" data-path="sandbox.allow_local_python_fallback" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.sandbox.local_python_fallback_label')}</span>
    </div>
    <div class="field-help">${t('config.sandbox.local_python_fallback_help')}</div></div>`;

    html += `<div class="field-group"><div class="cfg-toggle-row-compact">
        <div class="toggle ${sbCfg.keep_alive ? 'on' : ''}" data-path="sandbox.keep_alive" onclick="toggleBool(this)"></div>
        <span class="cfg-toggle-label">${t('config.sandbox.keep_alive_label')}</span>
    </div>
    <div class="field-help">${t('help.sandbox.keep_alive')}</div></div>`;

    html += `</div>`;

    if (sbCfg.allow_local_python_fallback) {
        html += `<div class="wh-notice sb-notice-warning">
            <span>⚠️</span>
            <div>
                <strong>${t('config.sandbox.local_python_fallback_label')}</strong><br>
                <small>${t('config.sandbox.local_python_fallback_warning')}</small>
            </div>
        </div>`;
    }

    html += `</div></div>`; // close sb-card-body & sb-card (Options)

    // ── System Status Card ──
    if (st.docker_available !== undefined) {
        html += `<div class="sb-card">
            <div class="sb-card-header">
                <span class="sb-card-icon">📊</span>
                <span class="sb-card-title">${t('config.sandbox.system_status')}</span>
            </div>
            <div class="sb-card-body">
                <div class="sb-status-grid">
                    <div>${st.docker_available ? '✅' : '❌'} Docker</div>
                    <div>${st.python_available ? '✅' : '❌'} Python</div>
                    <div>${st.package_installed ? '✅' : '❌'} llm-sandbox</div>
                    <div>${st.ready ? '✅' : '❌'} ${t('config.sandbox.ready')}</div>
                </div>
            </div>
        </div>`;
    }

    if (sbBlocked) html += '</div>';
    html += `</div>`;

    if (shellSandboxStatusCache === null) {
        try {
            const resp = await fetch('/api/sandbox/shell-status');
            shellSandboxStatusCache = resp.ok ? await resp.json() : {};
        } catch (_) { shellSandboxStatusCache = {}; }
    }
    const shCfg = configData.shell_sandbox || {};
    const shEnabled = shCfg.enabled === true;
    const shSt = shellSandboxStatusCache || {};

    html += `<div class="cfg-section active sb-section-mt">
        <div class="section-header">${t('config.shell_sandbox.title')}</div>
        <div class="section-desc">${t('config.shell_sandbox.desc')}</div>`;

    // ── Shell Sandbox General Card ──
    html += `<div class="sb-card">
        <div class="sb-card-header">
            <span class="sb-card-icon">🛡️</span>
            <span class="sb-card-title">${t('config.sandbox.card_general')}</span>
        </div>
        <div class="sb-card-body">`;

    if (shSt.available && shEnabled) {
        html += `<div class="wh-notice sb-notice-success">
            <span>✅</span>
            <div>
                <strong>${t('config.shell_sandbox.active')}</strong><br>
                <small>${t('config.sandbox.backend_label')}: ${escapeAttr(shSt.backend || 'landlock')} · Landlock ABI v${shSt.landlock_abi || 0} · Kernel ${escapeAttr(shSt.kernel || 'N/A')}</small>
            </div>
        </div>`;
    } else if (shEnabled && !shSt.available) {
        let reason = '';
        if (shSt.in_docker) reason = t('config.shell_sandbox.unavail_docker');
        else if (!shSt.landlock_abi) reason = t('config.shell_sandbox.unavail_kernel');
        else reason = t('config.shell_sandbox.unavail_platform');
        html += `<div class="wh-notice sb-notice-warning">
            <span>⚠️</span>
            <div>
                <strong>${t('config.shell_sandbox.not_available')}</strong><br>
                <small>${reason}</small>
            </div>
        </div>`;
    }

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.shell_sandbox.enabled_label')}</span>
        <div class="toggle ${shEnabled ? 'on' : ''}" data-path="shell_sandbox.enabled" aria-describedby="sb-shell-enabled-help" onclick="toggleBool(this)"></div>
    </div>
    <div class="field-help" id="sb-shell-enabled-help">${t('config.shell_sandbox.enabled_help')}</div>`;

    if (!shEnabled) {
        html += `<div class="wh-notice">
            <span>🛡️</span>
            <div>
                <strong>${t('config.shell_sandbox.disabled_notice')}</strong><br>
                <small>${t('config.shell_sandbox.disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `</div></div>`; // close sb-card-body & sb-card (Shell General)

    // ── Shell Sandbox Resource Limits Card ──
    if (shEnabled) {
        html += `<div class="sb-card">
            <div class="sb-card-header">
                <span class="sb-card-icon">📏</span>
                <span class="sb-card-title">${t('config.sandbox.card_resources')}</span>
            </div>
            <div class="sb-card-body">`;

        html += `<div class="sb-grid-2col">`;

        html += `<label class="sb-field-label">
            <span class="sb-field-caption">${t('config.shell_sandbox.max_memory')}</span>
            <input type="number" class="field-input sb-field-input" data-path="shell_sandbox.max_memory_mb" aria-describedby="sb-help-shell-sandbox-max-memory-mb" value="${shCfg.max_memory_mb || 1024}" min="64" max="8192"
                onchange="setNestedValue(configData,'shell_sandbox.max_memory_mb',parseInt(this.value)||1024);setDirty(true)">
        <small class="field-help sb-field-hint" id="sb-help-shell-sandbox-max-memory-mb">${t('config.shell_sandbox.max_memory_help')}</small>
    </label>`;

        html += `<label class="sb-field-label">
            <span class="sb-field-caption">${t('config.shell_sandbox.max_cpu')}</span>
            <input type="number" class="field-input sb-field-input" data-path="shell_sandbox.max_cpu_seconds" aria-describedby="sb-help-shell-sandbox-max-cpu-seconds" value="${shCfg.max_cpu_seconds || 30}" min="5" max="600"
                onchange="setNestedValue(configData,'shell_sandbox.max_cpu_seconds',parseInt(this.value)||30);setDirty(true)">
        <small class="field-help sb-field-hint" id="sb-help-shell-sandbox-max-cpu-seconds">${t('config.shell_sandbox.max_cpu_help')}</small>
    </label>`;

        html += `<label class="sb-field-label">
            <span class="sb-field-caption">${t('config.shell_sandbox.max_procs')}</span>
            <input type="number" class="field-input sb-field-input" data-path="shell_sandbox.max_processes" aria-describedby="sb-help-shell-sandbox-max-processes" value="${shCfg.max_processes || 50}" min="5" max="500"
                onchange="setNestedValue(configData,'shell_sandbox.max_processes',parseInt(this.value)||50);setDirty(true)">
            <small class="field-help sb-field-hint" id="sb-help-shell-sandbox-max-processes">${t('config.shell_sandbox.max_procs_note')}</small>
        </label>`;

        html += `<label class="sb-field-label">
            <span class="sb-field-caption">${t('config.shell_sandbox.max_fsize')}</span>
            <input type="number" class="field-input sb-field-input" data-path="shell_sandbox.max_file_size_mb" aria-describedby="sb-help-shell-sandbox-max-file-size-mb" value="${shCfg.max_file_size_mb || 100}" min="1" max="4096"
                onchange="setNestedValue(configData,'shell_sandbox.max_file_size_mb',parseInt(this.value)||100);setDirty(true)">
        <small class="field-help sb-field-hint" id="sb-help-shell-sandbox-max-file-size-mb">${t('config.shell_sandbox.max_fsize_help')}</small>
    </label>`;

        html += `</div>`;

        html += `</div></div>`; // close sb-card-body & sb-card (Resources)
    }

    // ── Shell Sandbox System Capabilities Card ──
    if (shSt.kernel || shSt.landlock_abi !== undefined) {
        html += `<div class="sb-card">
            <div class="sb-card-header">
                <span class="sb-card-icon">📊</span>
                <span class="sb-card-title">${t('config.shell_sandbox.system_caps')}</span>
            </div>
            <div class="sb-card-body">
                <div class="sb-status-grid">
                    <div>${shSt.landlock_abi ? '✅' : '❌'} Landlock ABI v${shSt.landlock_abi || 0}</div>
                    <div>${shSt.in_docker ? '⚠️' : '✅'} ${shSt.in_docker ? t('config.shell_sandbox.in_docker') : t('config.shell_sandbox.native')}</div>
                    <div>🐧 Kernel ${escapeAttr(shSt.kernel || 'N/A')}</div>
                    <div>${shSt.available ? '✅' : '❌'} ${t('config.shell_sandbox.ready')}</div>
                </div>
            </div>
        </div>`;
    }

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}
