// cfg/homepage.js — Homepage tool section module

function hpSetStatusState(el, state, text) {
    if (!el) return;
    el.classList.remove('is-success', 'is-error', 'is-warning', 'is-muted');
    if (state === 'success') el.classList.add('is-success');
    if (state === 'error') el.classList.add('is-error');
    if (state === 'warning') el.classList.add('is-warning');
    if (state === 'muted') el.classList.add('is-muted');
    el.textContent = text || '';
}

function hpCountAdaptivePromptTools() {
    const tools = (configData && configData.tools) || {};
    let count = 0;
    const flags = [
        tools.memory && tools.memory.enabled,
        tools.knowledge_graph && tools.knowledge_graph.enabled,
        tools.secrets_vault && tools.secrets_vault.enabled,
        tools.scheduler && tools.scheduler.enabled,
        tools.notes && tools.notes.enabled,
        tools.missions && tools.missions.enabled,
        tools.stop_process && tools.stop_process.enabled,
        tools.inventory && tools.inventory.enabled,
        tools.memory_maintenance && tools.memory_maintenance.enabled,
        tools.journal && tools.journal.enabled,
        tools.wol && tools.wol.enabled,
        tools.web_scraper && tools.web_scraper.enabled,
        tools.pdf_extractor && tools.pdf_extractor.enabled,
        tools.document_creator && tools.document_creator.enabled,
        tools.web_capture && tools.web_capture.enabled,
        tools.network_ping && tools.network_ping.enabled,
        tools.network_scan && tools.network_scan.enabled,
        tools.form_automation && tools.form_automation.enabled,
        tools.upnp_scan && tools.upnp_scan.enabled,
        tools.contacts && tools.contacts.enabled
    ];
    flags.forEach(v => { if (v) count += 1; });
    return count;
}

function hpCountAdaptivePromptIntegrations() {
    let count = 0;
    const flags = [
        configData.discord && configData.discord.enabled,
        (configData.email && configData.email.enabled) || (Array.isArray(configData.email_accounts) && configData.email_accounts.length > 0),
        configData.home_assistant && configData.home_assistant.enabled,
        configData.fritzbox && configData.fritzbox.enabled,
        configData.telnyx && configData.telnyx.enabled,
        configData.meshcentral && configData.meshcentral.enabled,
        configData.docker && configData.docker.enabled,
        configData.webdav && configData.webdav.enabled,
        configData.koofr && configData.koofr.enabled,
        configData.s3 && configData.s3.enabled,
        configData.paperless_ngx && configData.paperless_ngx.enabled,
        configData.proxmox && configData.proxmox.enabled,
        configData.tailscale && configData.tailscale.enabled,
        configData.cloudflare_tunnel && configData.cloudflare_tunnel.enabled,
        configData.ansible && configData.ansible.enabled,
        configData.github && configData.github.enabled,
        configData.netlify && configData.netlify.enabled,
        configData.adguard && configData.adguard.enabled,
        configData.mqtt && configData.mqtt.enabled,
        configData.google_workspace && configData.google_workspace.enabled,
        configData.onedrive && configData.onedrive.enabled,
        configData.jellyfin && configData.jellyfin.enabled,
        configData.remote_control && configData.remote_control.enabled,
        configData.invasion_control && configData.invasion_control.enabled,
        configData.sql_connections && configData.sql_connections.enabled,
        configData.webhooks && configData.webhooks.enabled,
        configData.n8n && configData.n8n.enabled,
        (configData.mcp && configData.mcp.enabled) && (configData.agent && configData.agent.allow_mcp),
        configData.homepage && configData.homepage.enabled,
        configData.co_agents && configData.co_agents.enabled,
        configData.image_generation && configData.image_generation.enabled,
        configData.embeddings && configData.embeddings.provider && configData.embeddings.provider !== 'disabled',
        configData.vision && configData.vision.provider,
        configData.whisper && (configData.whisper.provider || configData.whisper.mode),
        configData.tts && (configData.tts.provider || (configData.tts.piper && configData.tts.piper.enabled))
    ];
    flags.forEach(v => { if (v) count += 1; });
    return count;
}

function hpCalculateAdaptivePromptBudget(baseBudget) {
    if (!(configData.agent && configData.agent.adaptive_system_prompt_token_budget) || baseBudget <= 0) {
        return baseBudget;
    }
    const extra = (hpCountAdaptivePromptTools() * 48) + (hpCountAdaptivePromptIntegrations() * 160);
    const maxExtra = Math.min(4096, Math.floor(baseBudget / 2));
    return baseBudget + Math.min(extra, Math.max(0, maxExtra));
}

async function renderHomepageSection(section) {
    let st = {};
    try {
        const resp = await fetch('/api/homepage/status');
        st = resp.ok ? await resp.json() : {};
    } catch (_) {}

    const cfg = configData.homepage || {};
    const dockerEnabled = !!(configData.docker && configData.docker.enabled);
    const hpEnabled = cfg.enabled === true;
    const basePromptBudget = hpCalculateAdaptivePromptBudget(Math.max(0, parseInt(configData.agent && configData.agent.system_prompt_token_budget, 10) || 0));
    const baseToolCalls = Math.max(1, parseInt(configData.circuit_breaker && configData.circuit_breaker.max_tool_calls, 10) || 10);
    const homepageToolCalls = Math.max(baseToolCalls, parseInt(cfg.circuit_breaker_max_calls, 10) || 35);
    const homepagePromptBudget = basePromptBudget > 0
        ? Math.round(basePromptBudget * (homepageToolCalls / baseToolCalls))
        : 0;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    if (!dockerEnabled) {
        html += `<div class="wh-notice hp-notice-danger">
            <span>🐳</span>
            <div>
                <strong>${t('config.homepage.docker_required')}</strong><br>
                <small>${t('config.homepage.docker_required_desc')}</small>
            </div>
        </div>`;
    }

    const hpBanner = featureUnavailableBanner('homepage_docker');
    if (hpBanner) html += hpBanner;

    if (dockerEnabled && hpEnabled) {
        if (st.docker_available === false) {
            if (cfg.allow_container_management) {
                html += `<div class="wh-notice hp-notice-warning">
                    <span>⚠️</span>
                    <div>
                        <strong>${t('config.homepage.status_inactive')}</strong><br>
                        <small>${t('config.homepage.docker_engine_unavailable')}</small>
                    </div>
                </div>`;
            }
        } else {
            const devCtr = st.dev_container;
            const webCtr = st.web_container;
            const devExists = devCtr && devCtr.exists;
            const webExists = webCtr && webCtr.exists;
            if (devExists || webExists) {
                const devState = devCtr ? (devCtr.status || (devExists ? 'stopped' : 'not_created')) : 'not_found';
                const webState = webCtr ? (webCtr.status || (webExists ? 'stopped' : 'not_created')) : 'not_found';
                const devIcon = devState === 'running' ? '✅' : devState === 'exited' ? '⏸️' : '⭕';
                const webIcon = webState === 'running' ? '✅' : webState === 'exited' ? '⏸️' : '⭕';
                const devRunning = devState === 'running';
                const noticeStateClass = devRunning ? 'hp-notice-success' : 'hp-notice-warning';
                html += `<div class="wh-notice ${noticeStateClass}">
                    <span>${devRunning ? '🌐' : '⚠️'}</span>
                    <div>
                        <strong>${devRunning ? t('config.homepage.status_active') : t('config.homepage.status_inactive')}</strong><br>
                        <small>${devIcon} ${t('config.homepage.dev_container')}: ${escapeAttr(devState)} · ${webIcon} ${t('config.homepage.web_container')}: ${escapeAttr(webState)}</small>
                    </div>
                </div>`;
            }
        }
    }

    html += `<div class="cfg-toggle-row-highlight${!dockerEnabled ? ' hp-disabled' : ''}">
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

    if (dockerEnabled) {
        html += `<div class="hp-section-title hp-section-title-sm-top">🔐 ${t('config.homepage.permissions_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.permissions_desc')}</div>`;

        html += `<div class="hp-grid-two hp-grid-tight">`;

        html += `<div class="hp-toggle-row">
            <div class="toggle ${cfg.allow_deploy ? 'on' : ''}" data-path="homepage.allow_deploy" onclick="toggleBool(this)"></div>
            <span class="hp-toggle-label">${t('config.homepage.allow_deploy')}</span>
        </div>`;

        html += `<div class="hp-toggle-row">
            <div class="toggle ${cfg.allow_container_management ? 'on' : ''}" data-path="homepage.allow_container_management" onclick="toggleBool(this)"></div>
            <span class="hp-toggle-label">${t('config.homepage.allow_container')}</span>
        </div>`;

        html += `</div>`;
    }

    if (dockerEnabled) {
        html += `<div class="hp-section-title hp-section-title-lg-top">🚀 ${t('config.homepage.deploy_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.deploy_desc')}</div>`;

        html += `<div class="hp-grid-two hp-grid-wide">`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_host')}</span>
            <input class="cfg-input hp-input-top" data-path="homepage.deploy_host" value="${escapeAttr(cfg.deploy_host || '')}" placeholder="webserver.example.com"
                onchange="setNestedValue(configData,'homepage.deploy_host',this.value);setDirty(true)">
        </label>`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_port')}</span>
            <input type="number" class="cfg-input hp-input-top" data-path="homepage.deploy_port" value="${cfg.deploy_port || 22}" min="1" max="65535"
                onchange="setNestedValue(configData,'homepage.deploy_port',parseInt(this.value)||22);setDirty(true)">
        </label>`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_user')}</span>
            <input class="cfg-input hp-input-top" data-path="homepage.deploy_user" value="${escapeAttr(cfg.deploy_user || '')}" placeholder="deploy"
                onchange="setNestedValue(configData,'homepage.deploy_user',this.value);setDirty(true)">
        </label>`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_method')}</span>
            <select class="cfg-input hp-input-top" data-path="homepage.deploy_method" onchange="setNestedValue(configData,'homepage.deploy_method',this.value);setDirty(true)">
                <option value="sftp" ${(cfg.deploy_method || 'sftp') === 'sftp' ? 'selected' : ''}>${t('config.homepage.deploy_method_sftp')}</option>
                <option value="scp" ${cfg.deploy_method === 'scp' ? 'selected' : ''}>${t('config.homepage.deploy_method_scp')}</option>
            </select>
        </label>`;

        html += `</div>`;
        html += `<div class="hp-block-top">
            <label class="hp-label-block">
                <span class="hp-input-label">${t('config.homepage.deploy_path')}</span>
                <input class="cfg-input hp-input-top" data-path="homepage.deploy_path" value="${escapeAttr(cfg.deploy_path || '')}" placeholder="/var/www/html"
                    onchange="setNestedValue(configData,'homepage.deploy_path',this.value);setDirty(true)">
            </label>
        </div>`;

        html += `<div class="hp-credentials-box">`;
        html += `<div class="hp-credentials-title">🔑 ${t('config.homepage.credentials_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.credentials_desc')}</div>`;

        html += `<div class="hp-grid-two hp-grid-wide">`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_password')}  <small class="hp-lock-icon">🔐 vault</small></span>
            <div class="password-wrap hp-pw-wrap-mt">
                <input class="field-input hp-w-full" type="password" id="hp-deploy-password" value="${escapeAttr(cfgSecretValue(cfg.deploy_password))}" placeholder="${escapeAttr(cfgSecretPlaceholder(cfg.deploy_password))}" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
        </label>`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.deploy_key')}  <small class="hp-lock-icon">🔐 vault</small></span>
            <div class="password-wrap hp-pw-wrap-mt">
                <input class="field-input hp-w-full" type="password" id="hp-deploy-key" value="${escapeAttr(cfgSecretValue(cfg.deploy_key))}" placeholder="${escapeAttr(cfgSecretPlaceholder(cfg.deploy_key))}" autocomplete="off">
                <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
        </label>`;

        html += `</div>`;

        html += `<div class="hp-credentials-actions">
            <button class="btn-save hp-btn-small" onclick="hpSaveCredentials()">${t('config.homepage.save_credentials')}</button>
            <span id="hp-cred-status" class="hp-cred-status hp-cred-status-inline is-muted"></span>
        </div>`;

        html += `</div>`;

        html += `<div id="hp-test-block" class="hp-test-block">
            <div class="hp-test-title">🔌 ${t('config.homepage.test_title')}</div>
            <div class="hp-test-desc">${t('config.homepage.test_desc')}</div>
            <div class="hp-test-row">
                <button class="btn-save hp-btn-small" onclick="hpTestConnection()">${t('config.homepage.test_btn')}</button>
                <span id="hp-test-spinner" class="hp-test-spinner is-hidden">⏳ ${t('config.homepage.connecting')}</span>
            </div>
            <div id="hp-test-result" class="hp-test-result is-hidden">
                <div id="hp-test-msg" class="hp-test-msg"></div>
            </div>
        </div>`;
    }

    if (dockerEnabled) {
        html += `<div class="hp-section-title hp-section-title-lg-top">🖥️ ${t('config.homepage.webserver_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.webserver_desc')}</div>`;

        html += `<div class="cfg-toggle-row-compact">
            <span class="cfg-toggle-label">${t('config.homepage.webserver_enabled')}</span>
            <div class="toggle ${cfg.webserver_enabled ? 'on' : ''}" data-path="homepage.webserver_enabled" onclick="toggleBool(this)"></div>
        </div>`;

        html += `<div class="cfg-toggle-row-compact">
            <span class="cfg-toggle-label">${t('config.homepage.webserver_internal_only')}</span>
            <div class="toggle ${cfg.webserver_internal_only ? 'on' : ''}" data-path="homepage.webserver_internal_only" onclick="toggleBool(this)"></div>
        </div>`;

        html += `<div class="hp-grid-2col">`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.webserver_port')}</span>
            <input type="number" class="cfg-input hp-input-top" data-path="homepage.webserver_port" value="${cfg.webserver_port || 8080}" min="1" max="65535"
                onchange="setNestedValue(configData,'homepage.webserver_port',parseInt(this.value)||8080);setDirty(true)">
        </label>`;

        html += `<label class="hp-label-block">
            <span class="hp-input-label">${t('config.homepage.webserver_domain')} <small class="hp-text-tertiary">(${t('config.homepage.optional')})</small></span>
            <input class="cfg-input hp-input-top" data-path="homepage.webserver_domain" value="${escapeAttr(cfg.webserver_domain || '')}" placeholder="mysite.example.com"
                onchange="setNestedValue(configData,'homepage.webserver_domain',this.value);setDirty(true)">
        </label>`;

        html += `</div>`;
    }

    if (dockerEnabled) {
        html += `<div class="hp-section-title hp-section-title-lg-top">📁 ${t('config.homepage.workspace_title')}</div>`;
        html += `<div class="field-help hp-help-mb-sm">${t('config.homepage.workspace_desc')}</div>`;

        if (cfg.webserver_enabled && !cfg.workspace_path) {
            html += `<div class="hp-warning-box">
                <span class="hp-warning-icon">⚠️</span>
                <span class="hp-warning-text">${t('config.homepage.workspace_path_missing_warning')}</span>
            </div>`;
        }

        html += `<label class="hp-label-mt">
            <span class="hp-input-label">${t('config.homepage.workspace_path')}</span>
            <div class="hp-flex-row">
                <input id="hp-workspace-path-input" class="cfg-input hp-input-flex" data-path="homepage.workspace_path" value="${escapeAttr(cfg.workspace_path || '')}" placeholder="/home/aurago/aurago/agent_workspace/homepage"
                    onchange="setNestedValue(configData,'homepage.workspace_path',this.value);setDirty(true)">
                <button class="btn btn-secondary btn-sm hp-btn-nowrap" onclick="hpAutoDetectWorkspace()" title="${t('config.homepage.workspace_autodetect_title')}">
                    🔍 ${t('config.homepage.workspace_autodetect_btn')}
                </button>
            </div>
        </label>`;
        html += `<div class="field-help hp-help-mt-sm">${t('config.homepage.workspace_relative_hint')}</div>`;
    }

    if (dockerEnabled) {
        html += `<div class="hp-section-title hp-section-title-lg-top">⚡ ${t('config.homepage.circuit_breaker_title')}</div>`;
        html += `<div class="field-help hp-help-spaced">${t('config.homepage.circuit_breaker_desc')}</div>`;

        html += `<label class="hp-label-sm">
            <span class="hp-input-label">${t('config.homepage.circuit_breaker_max_calls')}</span>
            <input type="number" class="cfg-input hp-input-top" data-path="homepage.circuit_breaker_max_calls" value="${cfg.circuit_breaker_max_calls || 35}" min="1" max="100"
                onchange="setNestedValue(configData,'homepage.circuit_breaker_max_calls',parseInt(this.value)||35);setDirty(true)">
        </label>`;

        html += `<div class="cfg-toggle-row-compact hp-toggle-mt">
            <span class="cfg-toggle-label">${t('config.homepage.allow_temporary_token_budget_overflow')}</span>
            <div class="toggle ${cfg.allow_temporary_token_budget_overflow ? 'on' : ''}" data-path="homepage.allow_temporary_token_budget_overflow" onclick="toggleBool(this)"></div>
        </div>`;
        html += `<div class="field-help hp-help-mt-xs">${t('config.homepage.allow_temporary_token_budget_overflow_desc')}</div>`;
        if (cfg.allow_temporary_token_budget_overflow && basePromptBudget > 0) {
            html += `<div class="field-help hp-help-mt-xxs">${t('config.homepage.token_budget_preview', {
                base: basePromptBudget,
                effective: homepagePromptBudget,
                base_calls: baseToolCalls,
                homepage_calls: homepageToolCalls
            })}</div>`;
        }
    }

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

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
        if (pw) cfgMarkSecretStored(document.getElementById('hp-deploy-password'), 'homepage.deploy_password');
        if (key) cfgMarkSecretStored(document.getElementById('hp-deploy-key'), 'homepage.deploy_key');
    }
}

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
            if (window.showToast) {
                window.showToast(data.message || t('config.homepage.workspace_autodetect_success'), 'success');
            }
            renderSection();
        } else {
            input.value = origText;
            if (window.showToast) {
                window.showToast(data.message || t('config.homepage.workspace_autodetect_failed'), 'warning', 5000);
            }
        }
    } catch (e) {
        input.value = origText;
        if (window.showToast) {
            window.showToast(t('config.homepage.workspace_autodetect_failed'), 'warning', 5000);
        }
    } finally {
        input.disabled = false;
    }
}

async function hpTestConnection() {
    const spinner = document.getElementById('hp-test-spinner');
    const resultDiv = document.getElementById('hp-test-result');
    const msgDiv = document.getElementById('hp-test-msg');
    if (!spinner) return;

    setHidden(spinner, false);
    setHidden(resultDiv, true);

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

        setHidden(resultDiv, false);
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
        setHidden(resultDiv, false);
        msgDiv.classList.remove('is-success');
        msgDiv.classList.add('is-error');
        msgDiv.textContent = '❌ ' + e.message;
    } finally {
        setHidden(spinner, true);
    }
}
