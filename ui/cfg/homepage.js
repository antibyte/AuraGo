// cfg/homepage.js — Homepage tool section module

function hpSetHidden(el, hidden) {
    if (!el) return;
    el.classList.toggle('is-hidden', !!hidden);
}

function hpSetStatusState(el, state, text) {
    if (!el) return;
    el.classList.remove('is-success', 'is-error', 'is-warning', 'is-muted');
    if (state === 'success') el.classList.add('is-success');
    if (state === 'error') el.classList.add('is-error');
    if (state === 'warning') el.classList.add('is-warning');
    if (state === 'muted') el.classList.add('is-muted');
    el.textContent = text || '';
}

async function renderHomepageSection(section) {
    // Always fetch fresh status — stale cache would persist docker_available:false
    // across the session even after Docker becomes accessible again.
    let st = {};
    try {
        const resp = await fetch('/api/homepage/status');
        st = resp.ok ? await resp.json() : {};
    } catch (_) {}

    const cfg = configData.homepage || {};
    const dockerEnabled = !!(configData.docker && configData.docker.enabled);
    const hpEnabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Docker dependency notice ──
    if (!dockerEnabled) {
        html += `<div class="wh-notice" style="border-color:var(--danger);background:rgba(239,68,68,0.06);">
            <span>🐳</span>
            <div>
                <strong>${t('config.homepage.docker_required')}</strong><br>
                <small>${t('config.homepage.docker_required_desc')}</small>
            </div>
        </div>`;
    }

    // ── Docker socket unavailable banner (runtime) ──
    const hpBanner = featureUnavailableBanner('homepage_docker');
    if (hpBanner) html += hpBanner;

    // ── Status banner ──
    // Only show container status when Docker is configured AND containers exist.
    // SSH/SFTP deploy works without Docker — no warning needed just because containers aren't running.
    if (dockerEnabled && hpEnabled) {
        if (st.docker_available === false) {
            // Docker configured but not reachable — only show banner if container management is enabled
            if (cfg.allow_container_management) {
                html += `<div class="wh-notice" style="border-color:var(--warning);background:rgba(234,179,8,0.06);">
                    <span>⚠️</span>
                    <div>
                        <strong>${t('config.homepage.status_inactive')}</strong><br>
                        <small>Docker engine not reachable. Container management unavailable. Deploy via SSH/SFTP still works.</small>
                    </div>
                </div>`;
            }
        } else {
            // dev_container / web_container are objects: {exists, running, status}
            const devCtr = st.dev_container;
            const webCtr = st.web_container;
            const devExists = devCtr && devCtr.exists;
            const webExists = webCtr && webCtr.exists;
            // Only show container status when at least one container has been created.
            // "not_created" is normal initial state — deploy via SSH/SFTP works without containers.
            if (devExists || webExists) {
                const devState = devCtr ? (devCtr.status || (devExists ? 'stopped' : 'not_created')) : 'not_found';
                const webState = webCtr ? (webCtr.status || (webExists ? 'stopped' : 'not_created')) : 'not_found';
                const devIcon = devState === 'running' ? '✅' : devState === 'exited' ? '⏸️' : '⭕';
                const webIcon = webState === 'running' ? '✅' : webState === 'exited' ? '⏸️' : '⭕';
                const devRunning = devState === 'running';
                const borderColor = devRunning ? 'var(--success)' : 'var(--warning)';
                const bg = devRunning ? 'rgba(34,197,94,0.06)' : 'rgba(234,179,8,0.06)';
                html += `<div class="wh-notice" style="border-color:${borderColor};background:${bg};">
                    <span>${devRunning ? '🌐' : '⚠️'}</span>
                    <div>
                        <strong>${devRunning ? t('config.homepage.status_active') : t('config.homepage.status_inactive')}</strong><br>
                        <small>${devIcon} ${t('config.homepage.dev_container')}: ${escapeAttr(devState)} · ${webIcon} ${t('config.homepage.web_container')}: ${escapeAttr(webState)}</small>
                    </div>
                </div>`;
            }
        }
    }

    // ── Enabled toggle ──
    html += `<div class="cfg-toggle-row-highlight" ${!dockerEnabled ? 'style="opacity:0.5;pointer-events:none;"' : ''}>
        <span class="cfg-toggle-label">${t('config.homepage.enabled_label')}</span>
        <div class="toggle ${hpEnabled ? 'on' : ''}" data-path="homepage.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!hpEnabled && dockerEnabled) {
        html += `<div class="wh-notice">
            <span>🌐</span>
            <div>
                <strong>${t('config.homepage.disabled_notice')}</strong><br>
                <small>${t('config.homepage.disabled_desc')}</small>
            </div>
        </div>`;
    }

    // ── Permission toggles ──
    if (dockerEnabled) {
        html += `<div class="hp-section-title hp-section-title-sm-top">🔐 ${t('config.homepage.permissions_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.permissions_desc')}</div>`;

        html += `<div class="hp-grid-two hp-grid-tight">`;

        // Allow Deploy
        html += `<div class="hp-toggle-row">
            <div class="toggle ${cfg.allow_deploy ? 'on' : ''}" data-path="homepage.allow_deploy" onclick="toggleBool(this)"></div>
            <span class="hp-toggle-label">${t('config.homepage.allow_deploy')}</span>
        </div>`;

        // Allow Container Management
        html += `<div class="hp-toggle-row">
            <div class="toggle ${cfg.allow_container_management ? 'on' : ''}" data-path="homepage.allow_container_management" onclick="toggleBool(this)"></div>
            <span class="hp-toggle-label">${t('config.homepage.allow_container')}</span>
        </div>`;

        html += `</div>`;
    }

    // ── Deploy configuration ──
    if (dockerEnabled) {
        html += `<div class="hp-section-title hp-section-title-lg-top">🚀 ${t('config.homepage.deploy_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.deploy_desc')}</div>`;

        html += `<div class="hp-grid-two hp-grid-wide">`;

        // Deploy Host
        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_host')}</span>
            <input class="cfg-input hp-input-top" data-path="homepage.deploy_host" value="${escapeAttr(cfg.deploy_host || '')}" placeholder="webserver.example.com"
                onchange="setNestedValue(configData,'homepage.deploy_host',this.value);setDirty(true)">
        </label>`;

        // Deploy Port
        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_port')}</span>
            <input type="number" class="cfg-input hp-input-top" data-path="homepage.deploy_port" value="${cfg.deploy_port || 22}" min="1" max="65535"
                onchange="setNestedValue(configData,'homepage.deploy_port',parseInt(this.value)||22);setDirty(true)">
        </label>`;

        // Deploy User
        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_user')}</span>
            <input class="cfg-input hp-input-top" data-path="homepage.deploy_user" value="${escapeAttr(cfg.deploy_user || '')}" placeholder="deploy"
                onchange="setNestedValue(configData,'homepage.deploy_user',this.value);setDirty(true)">
        </label>`;

        // Deploy Method
        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_method')}</span>
            <select class="cfg-input hp-input-top" data-path="homepage.deploy_method" onchange="setNestedValue(configData,'homepage.deploy_method',this.value);setDirty(true)">
                <option value="sftp" ${(cfg.deploy_method || 'sftp') === 'sftp' ? 'selected' : ''}>SFTP</option>
                <option value="scp" ${cfg.deploy_method === 'scp' ? 'selected' : ''}>SCP</option>
            </select>
        </label>`;

        // Deploy Path (full width)
        html += `</div>`;
        html += `<div class="hp-block-top">
            <label class="hp-label-block">
                <span class="hp-input-label">${t('config.homepage.deploy_path')}</span>
                <input class="cfg-input hp-input-top" data-path="homepage.deploy_path" value="${escapeAttr(cfg.deploy_path || '')}" placeholder="/var/www/html"
                    onchange="setNestedValue(configData,'homepage.deploy_path',this.value);setDirty(true)">
            </label>
        </div>`;

        // ── Deploy credentials (vault-stored) ──
        html += `<div class="hp-credentials-box">`;
        html += `<div class="hp-credentials-title">🔑 ${t('config.homepage.credentials_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.credentials_desc')}</div>`;

        html += `<div class="hp-grid-two hp-grid-wide">`;

        // Password field (vault)
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.deploy_password')}  <small style="color:var(--text-tertiary);">🔐 vault</small></span>
            <div class="password-wrap" style="margin-top:0.2rem;">
                <input class="field-input" type="password" id="hp-deploy-password" value="" placeholder="••••••••" autocomplete="off" style="width:100%;">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
        </label>`;

        // SSH Key field (vault)
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.deploy_key')}  <small style="color:var(--text-tertiary);">🔐 vault</small></span>
            <div class="password-wrap" style="margin-top:0.2rem;">
                <input class="field-input" type="password" id="hp-deploy-key" value="" placeholder="••••••••" autocomplete="off" style="width:100%;">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
        </label>`;

        html += `</div>`;

        // Save credentials button
        html += `<div class="hp-credentials-actions">
            <button class="btn-save hp-btn-small" onclick="hpSaveCredentials()">${t('config.homepage.save_credentials')}</button>
            <span id="hp-cred-status" class="hp-cred-status hp-cred-status-inline is-muted"></span>
        </div>`;

        html += `</div>`; // end credentials box

        // ── Test Connection ──
        html += `<div id="hp-test-block" style="margin-top:1rem;padding:1rem 1.2rem;border:1px solid var(--border-subtle);border-radius:12px;background:var(--bg-secondary);">
            <div style="font-size:0.8rem;font-weight:600;color:var(--accent);margin-bottom:0.5rem;">🔌 ${t('config.homepage.test_title')}</div>
            <div style="font-size:0.78rem;color:var(--text-secondary);margin-bottom:0.8rem;">${t('config.homepage.test_desc')}</div>
            <div style="display:flex;gap:0.6rem;align-items:center;flex-wrap:wrap;">
                <button class="btn-save" style="padding:0.4rem 1.1rem;font-size:0.78rem;" onclick="hpTestConnection()">${t('config.homepage.test_btn')}</button>
                <span id="hp-test-spinner" class="hp-test-spinner is-hidden">⏳ ${t('config.homepage.connecting')}</span>
            </div>
            <div id="hp-test-result" class="hp-test-result is-hidden">
                <div id="hp-test-msg" class="hp-test-msg"></div>
            </div>
        </div>`;
    }

    // ── Local Webserver (Caddy) ──
    if (dockerEnabled) {
        html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">🖥️ ${t('config.homepage.webserver_title')}</div>`;
        html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.homepage.webserver_desc')}</div>`;

        // Webserver Enabled toggle
        html += `<div class="cfg-toggle-row-compact">
            <div class="toggle ${cfg.webserver_enabled ? 'on' : ''}" data-path="homepage.webserver_enabled" onclick="toggleBool(this)"></div>
            <span class="cfg-toggle-label">${t('config.homepage.webserver_enabled')}</span>
        </div>`;

        // Webserver Internal Only toggle
        html += `<div class="cfg-toggle-row-compact">
            <div class="toggle ${cfg.webserver_internal_only ? 'on' : ''}" data-path="homepage.webserver_internal_only" onclick="toggleBool(this)"></div>
            <span class="cfg-toggle-label">${t('config.homepage.webserver_internal_only')}</span>
        </div>`;

        html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem 1.2rem;">`;

        // Webserver Port
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.webserver_port')}</span>
            <input type="number" class="cfg-input" data-path="homepage.webserver_port" value="${cfg.webserver_port || 8080}" min="1" max="65535" style="width:100%;margin-top:0.2rem;"
                onchange="setNestedValue(configData,'homepage.webserver_port',parseInt(this.value)||8080);setDirty(true)">
        </label>`;

        // Webserver Domain
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.webserver_domain')} <small style="color:var(--text-tertiary);">(${t('config.homepage.optional')})</small></span>
            <input class="cfg-input" data-path="homepage.webserver_domain" value="${escapeAttr(cfg.webserver_domain || '')}" placeholder="mysite.example.com" style="width:100%;margin-top:0.2rem;"
                onchange="setNestedValue(configData,'homepage.webserver_domain',this.value);setDirty(true)">
        </label>`;

        html += `</div>`;
    }

    // ── Workspace Path ──
    if (dockerEnabled) {
        html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">📁 ${t('config.homepage.workspace_title')}</div>`;

        // Warning: webserver enabled but no workspace_path
        if (cfg.webserver_enabled && !cfg.workspace_path) {
            html += `<div style="background:rgba(245,158,11,0.12);border:1px solid rgba(245,158,11,0.4);border-radius:8px;padding:0.7rem 0.9rem;margin-bottom:0.8rem;display:flex;align-items:flex-start;gap:0.6rem;overflow:hidden;">
                <span style="font-size:1.1rem;flex-shrink:0;">⚠️</span>
                <span style="font-size:0.82rem;color:var(--text-primary);line-height:1.5;overflow-wrap:anywhere;min-width:0;">${t('config.homepage.workspace_path_missing_warning')}</span>
            </div>`;
        }

        html += `<label style="display:block;margin-top:0.6rem;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.workspace_path')}</span>
            <div style="display:flex;gap:0.5rem;align-items:center;margin-top:0.2rem;">
                <input id="hp-workspace-path-input" class="cfg-input" data-path="homepage.workspace_path" value="${escapeAttr(cfg.workspace_path || '')}" placeholder="/home/aurago/aurago/agent_workspace/homepage" style="flex:1;"
                    onchange="setNestedValue(configData,'homepage.workspace_path',this.value);setDirty(true)">
                <button class="btn btn-secondary btn-sm" onclick="hpAutoDetectWorkspace()" title="${t('config.homepage.workspace_autodetect_title')}" style="white-space:nowrap;flex-shrink:0;">
                    🔍 ${t('config.homepage.workspace_autodetect_btn')}
                </button>
            </div>
        </label>`;
    }

    // ── Circuit Breaker ──
    if (dockerEnabled) {
        html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">⚡ ${t('config.homepage.circuit_breaker_title')}</div>`;
        html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.homepage.circuit_breaker_desc')}</div>`;

        html += `<label style="display:block;max-width:200px;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.circuit_breaker_max_calls')}</span>
            <input type="number" class="cfg-input" data-path="homepage.circuit_breaker_max_calls" value="${cfg.circuit_breaker_max_calls || 35}" min="1" max="100" style="width:100%;margin-top:0.2rem;"
                onchange="setNestedValue(configData,'homepage.circuit_breaker_max_calls',parseInt(this.value)||35);setDirty(true)">
        </label>`;
    }

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

// ── Save deploy credentials to vault ──
async function hpSaveCredentials() {
    const statusEl = document.getElementById('hp-cred-status');
    const pw = document.getElementById('hp-deploy-password').value;
    const key = document.getElementById('hp-deploy-key').value;

    if (!pw && !key) {
        hpSetStatusState(statusEl, 'warning', '⚠️ ' + t('config.homepage.cred_empty'));
        return;
    }

    let saved = 0;
    let errors = [];

    async function saveOne(vaultKey, value) {
        if (!value) return;
        try {
            const resp = await fetch('/api/vault/secrets', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ key: vaultKey, value: value })
            });
            if (!resp.ok) {
                const txt = await resp.text();
                errors.push(vaultKey + ': ' + txt);
            } else {
                saved++;
            }
        } catch (e) {
            errors.push(vaultKey + ': ' + e.message);
        }
    }

    hpSetStatusState(statusEl, 'muted', '⏳ ' + t('config.homepage.saving'));

    await saveOne('homepage_deploy_password', pw);
    await saveOne('homepage_deploy_key', key);

    if (errors.length > 0) {
        hpSetStatusState(statusEl, 'error', '❌ ' + errors.join('; '));
    } else {
        hpSetStatusState(statusEl, 'success', '✅ ' + t('config.homepage.cred_saved', { count: saved }));
        // Clear inputs after successful save
        document.getElementById('hp-deploy-password').value = '';
        document.getElementById('hp-deploy-key').value = '';
    }
}

// ── Auto-detect workspace path from running homepage container ──
async function hpAutoDetectWorkspace() {
    const input = document.getElementById('hp-workspace-path-input');
    if (!input) return;
    const origText = input.value;
    input.disabled = true;
    input.value = t('config.homepage.workspace_autodetect_loading');
    try {
        const resp = await fetch('/api/homepage/detect-workspace');
        const data = await resp.json();
        if (data.status === 'ok' && data.path) {
            input.value = data.path;
            setNestedValue(configData, 'homepage.workspace_path', data.path);
            setDirty(true);
            // Re-render to clear the warning banner now that a path is set
            renderSection();
        } else {
            input.value = origText;
            alert('⚠️ ' + (data.message || t('config.homepage.workspace_autodetect_failed')));
        }
    } catch (e) {
        input.value = origText;
        alert('⚠️ ' + t('config.homepage.workspace_autodetect_failed'));
    } finally {
        input.disabled = false;
    }
}

// ── Test Deploy Connection ──
async function hpTestConnection() {
    const spinner = document.getElementById('hp-test-spinner');
    const resultDiv = document.getElementById('hp-test-result');
    const msgDiv = document.getElementById('hp-test-msg');
    if (!spinner) return;

    hpSetHidden(spinner, false);
    hpSetHidden(resultDiv, true);

    const getField = (path) => {
        const el = document.querySelector('[data-path="' + path + '"]');
        if (!el) return '';
        return el.value.trim();
    };

    const body = {
        host: getField('homepage.deploy_host'),
        port: parseInt(getField('homepage.deploy_port')) || 0,
        user: getField('homepage.deploy_user'),
        password: document.getElementById('hp-deploy-password') ? document.getElementById('hp-deploy-password').value : '',
        path: getField('homepage.deploy_path')
    };

    try {
        const resp = await fetch('/api/homepage/test-connection', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        const json = await resp.json();

        hpSetHidden(resultDiv, false);
        if (json.status === 'ok') {
            msgDiv.classList.remove('is-error');
            msgDiv.classList.add('is-success');
            msgDiv.textContent = '✅ ' + (json.message || t('config.homepage.test_success'));
        } else {
            msgDiv.classList.remove('is-success');
            msgDiv.classList.add('is-error');
            msgDiv.textContent = '❌ ' + (json.message || t('config.homepage.test_failed'));
        }
    } catch (e) {
        hpSetHidden(resultDiv, false);
        msgDiv.classList.remove('is-success');
        msgDiv.classList.add('is-error');
        msgDiv.textContent = '❌ ' + e.message;
    } finally {
        hpSetHidden(spinner, true);
    }
}
