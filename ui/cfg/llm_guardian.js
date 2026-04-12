let _guardianSection = null;

async function renderLLMGuardianSection(section) {
    if (section) _guardianSection = section; else section = _guardianSection;
    const cfg = configData.llm_guardian || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.llm_guardian.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="llm_guardian.enabled" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.enabled',this.classList.contains('on'));renderLLMGuardianSection(null)"></div>
    </div>`;

    if (!enabled) {
        html += `<div class="wh-notice">
            <span>🛡️</span>
            <div>
                <strong>${t('config.llm_guardian.disabled_notice')}</strong><br>
                <small>${t('config.llm_guardian.disabled_desc')}</small>
            </div>
        </div>`;
        html += `</div>`;
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.provider_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.provider_desc')}</div>`;

    const curProvider = cfg.provider || '';
    html += `<label class="lg-field-label">
        <span class="lg-label-text">${t('config.llm_guardian.provider_label')}</span>
        <select class="field-input cfg-input-full" data-path="llm_guardian.provider"
            onchange="setNestedValue(configData,'llm_guardian.provider',this.value);setDirty(true)">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.llm_guardian.select_provider')}</option>`;
    providersCache.forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' — ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += `</select></label>`;

    const curModel = cfg.model || '';
    html += `<label class="lg-field-label">
        <span class="lg-label-text">${t('config.llm_guardian.model_label')} <small class="lg-hint-text">(${t('config.llm_guardian.model_hint')})</small></span>
        <input type="text" class="field-input cfg-input-full" data-path="llm_guardian.model" value="${escapeAttr(curModel)}"
            placeholder="gemini-2.0-flash, gpt-4o-mini...">
    </label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.level_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.level_desc')}</div>`;

    const curLevel = cfg.default_level || 'medium';
    const levels = ['off', 'low', 'medium', 'high'];
    html += `<label class="lg-field-label">
        <span class="lg-label-text">${t('config.llm_guardian.level_label')}</span>
        <select class="field-input cfg-input-full" data-path="llm_guardian.default_level"
            onchange="setNestedValue(configData,'llm_guardian.default_level',this.value);setDirty(true)">`;
    levels.forEach(lv => {
        const sel = (curLevel === lv) ? ' selected' : '';
        html += `<option value="${lv}"${sel}>${t('config.llm_guardian.level_' + lv)}</option>`;
    });
    html += `</select></label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.failsafe_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.failsafe_desc')}</div>`;

    const curFailSafe = cfg.fail_safe || 'quarantine';
    const failSafes = ['block', 'quarantine', 'allow'];
    html += `<label class="lg-field-label">
        <span class="lg-label-text">${t('config.llm_guardian.failsafe_label')}</span>
        <select class="field-input cfg-input-full" data-path="llm_guardian.fail_safe"
            onchange="setNestedValue(configData,'llm_guardian.fail_safe',this.value);setDirty(true)">`;
    failSafes.forEach(fs => {
        const sel = (curFailSafe === fs) ? ' selected' : '';
        html += `<option value="${fs}"${sel}>${t('config.llm_guardian.failsafe_' + fs)}</option>`;
    });
    html += `</select></label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.perf_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.perf_desc')}</div>`;

    const curTTL = cfg.cache_ttl != null ? cfg.cache_ttl : 300;
    html += `<label class="lg-field-label">
        <span class="lg-label-text">${t('config.llm_guardian.cache_ttl_label')}</span>
        <input type="number" class="field-input cfg-input-full" data-path="llm_guardian.cache_ttl" value="${curTTL}"
            min="0" max="3600" step="30">
    </label>`;

    const curRate = cfg.max_checks_per_minute != null ? cfg.max_checks_per_minute : 60;
    html += `<label class="lg-field-label">
        <span class="lg-label-text">${t('config.llm_guardian.rate_limit_label')}</span>
        <input type="number" class="field-input cfg-input-full" data-path="llm_guardian.max_checks_per_minute" value="${curRate}"
            min="1" max="300" step="1">
    </label>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.clarification_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.clarification_desc')}</div>`;

    const clarificationOn = cfg.allow_clarification === true;
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${clarificationOn ? 'on' : ''}" data-path="llm_guardian.allow_clarification" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.allow_clarification',this.classList.contains('on'));setDirty(true)"></div>
        <span class="cfg-toggle-label">${t('config.llm_guardian.clarification_label')}</span>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.scan_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.scan_desc')}</div>`;

    const scanEmails = cfg.scan_emails === true;
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${scanEmails ? 'on' : ''}" data-path="llm_guardian.scan_emails" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.scan_emails',this.classList.contains('on'));setDirty(true)"></div>
        <span class="cfg-toggle-label">${t('config.llm_guardian.scan_emails_label')}</span>
    </div>`;

    const scanDocs = cfg.scan_documents === true;
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${scanDocs ? 'on' : ''}" data-path="llm_guardian.scan_documents" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.scan_documents',this.classList.contains('on'));setDirty(true)"></div>
        <span class="cfg-toggle-label">${t('config.llm_guardian.scan_documents_label')}</span>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.overrides_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.overrides_desc')}</div>`;

    const overrides = cfg.tool_overrides || {};
    const overrideKeys = Object.keys(overrides);

    if (overrideKeys.length > 0) {
        html += `<div class="lg-override-list">`;
        overrideKeys.forEach(toolName => {
            const toolLevel = overrides[toolName] || 'medium';
            const desc = _guardianGetToolDesc(toolName);
            const riskIcon = _guardianHighRiskTools.has(toolName) ? '🔴' : (_guardianRiskyTools.has(toolName) ? '🟡' : '⚪');
            html += `<div class="lg-override-row">
                <span class="lg-override-name" title="${escapeAttr(desc ? toolName + ' — ' + desc : toolName)}">${riskIcon} <strong>${escapeAttr(toolName)}</strong>${desc ? ' <span class="lg-override-desc">— ' + escapeAttr(desc) + '</span>' : ''}</span>
                <select class="field-input lg-select-sm"
                    onchange="guardianSetOverride('${escapeAttr(toolName)}',this.value)">`;
            levels.forEach(lv => {
                const sel = (toolLevel === lv) ? ' selected' : '';
                html += `<option value="${lv}"${sel}>${t('config.llm_guardian.level_' + lv)}</option>`;
            });
            html += `</select>
                <button class="lg-btn-remove"
                    onclick="guardianRemoveOverride('${escapeAttr(toolName)}')">✕</button>
            </div>`;
        });
        html += `</div>`;
    }

    html += `<div class="lg-add-row">
        <input type="text" id="guardian-new-tool" class="field-input lg-add-input" list="guardian-tool-datalist"
            placeholder="${t('config.llm_guardian.overrides_tool_search')}">
        <datalist id="guardian-tool-datalist">`;
    if (_guardianToolList) {
        _guardianToolList.forEach(name => {
            if (!overrides[name]) {
                const desc = _guardianGetToolDesc(name);
                const riskIcon = _guardianHighRiskTools.has(name) ? '🔴' : (_guardianRiskyTools.has(name) ? '🟡' : '⚪');
                const label = desc ? `${riskIcon} ${name} — ${desc}` : `${riskIcon} ${name}`;
                html += `<option value="${escapeAttr(name)}" label="${escapeAttr(label)}">`;
            }
        });
    }
    html += `</datalist>
        <select id="guardian-new-level" class="field-input lg-select-sm">`;
    levels.forEach(lv => {
        const sel = (lv === 'high') ? ' selected' : '';
        html += `<option value="${lv}"${sel}>${t('config.llm_guardian.level_' + lv)}</option>`;
    });
    html += `</select>
        <button class="lg-btn-add"
            onclick="guardianAddOverride()">+</button>
    </div>`;
    html += `</div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();

    if (!_guardianToolList) {
        guardianLoadToolList();
    }
}

let _guardianToolList = null;

const _guardianToolDescriptions = {
    execute_shell: 'Run shell commands',
    execute_sudo: 'Run commands as root',
    execute_python: 'Execute Python code',
    execute_remote_shell: 'SSH remote commands',
    filesystem: 'Read/write files',
    api_request: 'HTTP API calls',
    docker: 'Manage Docker containers',
    proxmox: 'Proxmox VM management',
    home_assistant: 'Smart home control',
    co_agent: 'Spawn sub-agents',
    manage_updates: 'Self-update system',
    set_secret: 'Store vault secrets',
    save_tool: 'Create custom tools',
    netlify: 'Netlify deployments',
    send_email: 'Send emails',
    fetch_email: 'Fetch emails',
    discord: 'Discord messaging',
    manage_memory: 'Long-term memory',
    knowledge_graph: 'Knowledge graph ops',
    manage_notes: 'Manage notes',
    manage_cron: 'Cron job scheduler',
    call_webhook: 'Call outgoing webhooks',
    manage_missions: 'Mission management',
    tailscale: 'Tailscale VPN',
    manage_devices: 'SSH device inventory',
    koofr: 'Koofr cloud storage',
    google_workspace: 'Google Workspace',
    webdav: 'WebDAV file access',
    ollama: 'Ollama local models',
    adguard: 'AdGuard Home DNS',
    chromecast: 'Chromecast control',
    mcp_call: 'MCP server tools',
    cheat_sheet: 'Cheat sheet lookup',
    send_image: 'Send/generate images',
    text_to_speech: 'Text to speech',
    media_registry: 'Media registry',
    homepage_registry: 'Homepage registry',
};

function _guardianGetToolDesc(toolName) {
    const key = 'config.llm_guardian.tool_' + toolName;
    const translated = t(key);
    if (translated && translated !== key) return translated;
    return _guardianToolDescriptions[toolName] || '';
}

const _guardianHighRiskTools = new Set([
    'execute_shell', 'execute_sudo', 'execute_python', 'execute_remote_shell', 'filesystem'
]);

const _guardianRiskyTools = new Set([
    'execute_shell', 'execute_sudo', 'execute_python', 'execute_remote_shell', 'filesystem',
    'api_request', 'docker', 'proxmox', 'set_secret', 'save_tool', 'co_agent',
    'manage_updates', 'netlify', 'home_assistant'
]);

async function guardianLoadToolList() {
    try {
        const resp = await fetch('/api/mcp-server/tools');
        if (!resp.ok) return;
        const names = await resp.json();
        if (Array.isArray(names) && names.length > 0) {
            _guardianToolList = names;
            renderLLMGuardianSection(null);
        }
    } catch (e) {
    }
}

function guardianSetOverride(tool, level) {
    if (!configData.llm_guardian) configData.llm_guardian = {};
    if (!configData.llm_guardian.tool_overrides) configData.llm_guardian.tool_overrides = {};
    configData.llm_guardian.tool_overrides[tool] = level;
    setDirty(true);
    renderLLMGuardianSection(null);
}

function guardianRemoveOverride(tool) {
    if (configData.llm_guardian && configData.llm_guardian.tool_overrides) {
        delete configData.llm_guardian.tool_overrides[tool];
    }
    setDirty(true);
    renderLLMGuardianSection(null);
}

function guardianAddOverride() {
    const toolInput = document.getElementById('guardian-new-tool');
    const levelSelect = document.getElementById('guardian-new-level');
    const tool = (toolInput.value || '').trim();
    if (!tool) return;
    if (!configData.llm_guardian) configData.llm_guardian = {};
    if (!configData.llm_guardian.tool_overrides) configData.llm_guardian.tool_overrides = {};
    configData.llm_guardian.tool_overrides[tool] = levelSelect.value;
    setDirty(true);
    renderLLMGuardianSection(null);
}
