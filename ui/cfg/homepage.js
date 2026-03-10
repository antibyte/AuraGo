// cfg/homepage.js — Homepage tool section module

let hpStatusCache = null;

async function renderHomepageSection(section) {
    // Lazy-load status
    if (hpStatusCache === null) {
        try {
            const resp = await fetch('/api/homepage/status');
            hpStatusCache = resp.ok ? await resp.json() : {};
        } catch (_) { hpStatusCache = {}; }
    }

    const cfg = configData.homepage || {};
    const dockerEnabled = !!(configData.docker && configData.docker.enabled);
    const hpEnabled = cfg.enabled === true;
    const st = hpStatusCache || {};

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

    // ── Status banner ──
    if (dockerEnabled && hpEnabled) {
        const devState = st.dev_container || 'not_found';
        const webState = st.web_container || 'not_found';
        const devIcon = devState === 'running' ? '✅' : devState === 'exited' ? '⏸️' : '⭕';
        const webIcon = webState === 'running' ? '✅' : webState === 'exited' ? '⏸️' : '⭕';
        const anyRunning = devState === 'running' || webState === 'running';
        const borderColor = anyRunning ? 'var(--success)' : 'var(--warning)';
        const bg = anyRunning ? 'rgba(34,197,94,0.06)' : 'rgba(234,179,8,0.06)';

        html += `<div class="wh-notice" style="border-color:${borderColor};background:${bg};">
            <span>${anyRunning ? '🌐' : '⚠️'}</span>
            <div>
                <strong>${anyRunning ? t('config.homepage.status_active') : t('config.homepage.status_inactive')}</strong><br>
                <small>${devIcon} ${t('config.homepage.dev_container')}: ${escapeAttr(devState)} · ${webIcon} ${t('config.homepage.web_container')}: ${escapeAttr(webState)}</small>
            </div>
        </div>`;
    }

    // ── Enabled toggle ──
    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:1rem;padding:0.6rem 1rem;border-radius:8px;background:var(--bg-tertiary);${!dockerEnabled ? 'opacity:0.5;pointer-events:none;' : ''}">
        <span style="font-size:0.85rem;color:var(--text-secondary);">${t('config.homepage.enabled_label')}</span>
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
        html += `<div style="margin-top:1.2rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">🔐 ${t('config.homepage.permissions_title')}</div>`;
        html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.homepage.permissions_desc')}</div>`;

        html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.6rem 1.2rem;">`;

        // Allow Deploy
        html += `<div style="display:flex;align-items:center;gap:0.6rem;padding:0.5rem 0;">
            <div class="toggle ${cfg.allow_deploy ? 'on' : ''}" data-path="homepage.allow_deploy" onclick="toggleBool(this)"></div>
            <span style="font-size:0.82rem;color:var(--text-secondary);">${t('config.homepage.allow_deploy')}</span>
        </div>`;

        // Allow Container Management
        html += `<div style="display:flex;align-items:center;gap:0.6rem;padding:0.5rem 0;">
            <div class="toggle ${cfg.allow_container_management ? 'on' : ''}" data-path="homepage.allow_container_management" onclick="toggleBool(this)"></div>
            <span style="font-size:0.82rem;color:var(--text-secondary);">${t('config.homepage.allow_container')}</span>
        </div>`;

        html += `</div>`;
    }

    // ── Deploy configuration ──
    if (dockerEnabled) {
        html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">🚀 ${t('config.homepage.deploy_title')}</div>`;
        html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.homepage.deploy_desc')}</div>`;

        html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem 1.2rem;">`;

        // Deploy Host
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.deploy_host')}</span>
            <input class="cfg-input" data-path="homepage.deploy_host" value="${escapeAttr(cfg.deploy_host || '')}" placeholder="webserver.example.com" style="width:100%;margin-top:0.2rem;"
                onchange="setNestedValue(configData,'homepage.deploy_host',this.value);setDirty(true)">
        </label>`;

        // Deploy Port
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.deploy_port')}</span>
            <input type="number" class="cfg-input" data-path="homepage.deploy_port" value="${cfg.deploy_port || 22}" min="1" max="65535" style="width:100%;margin-top:0.2rem;"
                onchange="setNestedValue(configData,'homepage.deploy_port',parseInt(this.value)||22);setDirty(true)">
        </label>`;

        // Deploy User
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.deploy_user')}</span>
            <input class="cfg-input" data-path="homepage.deploy_user" value="${escapeAttr(cfg.deploy_user || '')}" placeholder="deploy" style="width:100%;margin-top:0.2rem;"
                onchange="setNestedValue(configData,'homepage.deploy_user',this.value);setDirty(true)">
        </label>`;

        // Deploy Method
        html += `<label style="display:block;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.deploy_method')}</span>
            <select class="cfg-input" data-path="homepage.deploy_method" style="width:100%;margin-top:0.2rem;" onchange="setNestedValue(configData,'homepage.deploy_method',this.value);setDirty(true)">
                <option value="sftp" ${(cfg.deploy_method || 'sftp') === 'sftp' ? 'selected' : ''}>SFTP</option>
                <option value="scp" ${cfg.deploy_method === 'scp' ? 'selected' : ''}>SCP</option>
            </select>
        </label>`;

        // Deploy Path (full width)
        html += `</div>`;
        html += `<div style="margin-top:0.8rem;">
            <label style="display:block;">
                <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.deploy_path')}</span>
                <input class="cfg-input" data-path="homepage.deploy_path" value="${escapeAttr(cfg.deploy_path || '')}" placeholder="/var/www/html" style="width:100%;margin-top:0.2rem;"
                    onchange="setNestedValue(configData,'homepage.deploy_path',this.value);setDirty(true)">
            </label>
        </div>`;

        // ── Deploy credentials (vault-stored) ──
        html += `<div style="margin-top:1.2rem;padding:0.8rem 1rem;border-radius:10px;border:1px dashed var(--border-subtle);background:var(--bg-tertiary);">`;
        html += `<div style="font-size:0.82rem;font-weight:600;color:var(--text-secondary);margin-bottom:0.6rem;">🔑 ${t('config.homepage.credentials_title')}</div>`;
        html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.homepage.credentials_desc')}</div>`;

        html += `<div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem 1.2rem;">`;

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
        html += `<div style="display:flex;gap:0.6rem;align-items:center;margin-top:0.8rem;flex-wrap:wrap;">
            <button class="btn-save" style="padding:0.4rem 1.1rem;font-size:0.78rem;" onclick="hpSaveCredentials()">${t('config.homepage.save_credentials')}</button>
            <span id="hp-cred-status" style="font-size:0.75rem;color:var(--text-tertiary);"></span>
        </div>`;

        html += `</div>`; // end credentials box

        // ── Test Connection ──
        html += `<div id="hp-test-block" style="margin-top:1rem;padding:1rem 1.2rem;border:1px solid var(--border-subtle);border-radius:12px;background:var(--bg-secondary);">
            <div style="font-size:0.8rem;font-weight:600;color:var(--accent);margin-bottom:0.5rem;">🔌 ${t('config.homepage.test_title')}</div>
            <div style="font-size:0.78rem;color:var(--text-secondary);margin-bottom:0.8rem;">${t('config.homepage.test_desc')}</div>
            <div style="display:flex;gap:0.6rem;align-items:center;flex-wrap:wrap;">
                <button class="btn-save" style="padding:0.4rem 1.1rem;font-size:0.78rem;" onclick="hpTestConnection()">${t('config.homepage.test_btn')}</button>
                <span id="hp-test-spinner" style="display:none;font-size:0.75rem;color:var(--text-secondary);">⏳ ${t('config.homepage.connecting')}</span>
            </div>
            <div id="hp-test-result" style="margin-top:0.8rem;display:none;">
                <div id="hp-test-msg" style="font-size:0.82rem;padding:0.45rem 0.7rem;border-radius:7px;"></div>
            </div>
        </div>`;
    }

    // ── Local Webserver (Caddy) ──
    if (dockerEnabled) {
        html += `<div style="margin-top:1.5rem;margin-bottom:0.5rem;font-weight:600;font-size:0.9rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">🖥️ ${t('config.homepage.webserver_title')}</div>`;
        html += `<div class="field-help" style="margin-bottom:0.8rem;">${t('config.homepage.webserver_desc')}</div>`;

        // Webserver Enabled toggle
        html += `<div style="display:flex;align-items:center;gap:0.6rem;padding:0.5rem 0;margin-bottom:0.6rem;">
            <div class="toggle ${cfg.webserver_enabled ? 'on' : ''}" data-path="homepage.webserver_enabled" onclick="toggleBool(this)"></div>
            <span style="font-size:0.82rem;color:var(--text-secondary);">${t('config.homepage.webserver_enabled')}</span>
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
        html += `<label style="display:block;margin-top:0.6rem;">
            <span style="font-size:0.78rem;color:var(--text-secondary);">${t('config.homepage.workspace_path')}</span>
            <input class="cfg-input" data-path="homepage.workspace_path" value="${escapeAttr(cfg.workspace_path || '')}" placeholder="./agent_workspace/homepage" style="width:100%;margin-top:0.2rem;"
                onchange="setNestedValue(configData,'homepage.workspace_path',this.value);setDirty(true)">
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
        if (statusEl) { statusEl.textContent = '⚠️ ' + t('config.homepage.cred_empty'); statusEl.style.color = 'var(--warning)'; }
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

    if (statusEl) { statusEl.textContent = '⏳ ' + t('config.homepage.saving'); statusEl.style.color = 'var(--text-tertiary)'; }

    await saveOne('homepage_deploy_password', pw);
    await saveOne('homepage_deploy_key', key);

    if (errors.length > 0) {
        if (statusEl) { statusEl.textContent = '❌ ' + errors.join('; '); statusEl.style.color = 'var(--danger)'; }
    } else {
        if (statusEl) { statusEl.textContent = '✅ ' + t('config.homepage.cred_saved', { count: saved }); statusEl.style.color = 'var(--success)'; }
        // Clear inputs after successful save
        document.getElementById('hp-deploy-password').value = '';
        document.getElementById('hp-deploy-key').value = '';
    }
}

// ── Test Deploy Connection ──
async function hpTestConnection() {
    const spinner = document.getElementById('hp-test-spinner');
    const resultDiv = document.getElementById('hp-test-result');
    const msgDiv = document.getElementById('hp-test-msg');
    if (!spinner) return;

    spinner.style.display = 'inline';
    resultDiv.style.display = 'none';

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

        resultDiv.style.display = 'block';
        if (json.status === 'ok') {
            msgDiv.style.background = 'rgba(34,197,94,0.12)';
            msgDiv.style.color = 'var(--success, #22c55e)';
            msgDiv.style.border = '1px solid rgba(34,197,94,0.3)';
            msgDiv.textContent = '✅ ' + (json.message || t('config.homepage.test_success'));
        } else {
            msgDiv.style.background = 'rgba(239,68,68,0.10)';
            msgDiv.style.color = 'var(--danger, #ef4444)';
            msgDiv.style.border = '1px solid rgba(239,68,68,0.25)';
            msgDiv.textContent = '❌ ' + (json.message || t('config.homepage.test_failed'));
        }
    } catch (e) {
        resultDiv.style.display = 'block';
        msgDiv.style.background = 'rgba(239,68,68,0.10)';
        msgDiv.style.color = 'var(--danger, #ef4444)';
        msgDiv.style.border = '1px solid rgba(239,68,68,0.25)';
        msgDiv.textContent = '❌ ' + e.message;
    } finally {
        spinner.style.display = 'none';
    }
}
