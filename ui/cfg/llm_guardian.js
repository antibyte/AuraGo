let _llmGuardianSection = null;

async function renderLLMGuardianSection(section) {
    if (section) _llmGuardianSection = section; else section = _llmGuardianSection;
    const cfg = configData.llm_guardian || {};
    const enabled = cfg.enabled === true;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.llm_guardian.enabled_label')}</span>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="llm_guardian.enabled" role="switch" aria-checked="${enabled}" aria-label="${escapeAttr(t('config.llm_guardian.enabled_label'))}" aria-describedby="lg-enabled-help" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.enabled',this.classList.contains('on'));renderLLMGuardianSection(null)"></div>
    </div>
    <div class="field-help" id="lg-enabled-help">${t('config.llm_guardian.enabled_help')}</div>`;

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
    html += `<div class="field-grid two-cols">`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.llm_guardian.provider_label')}</div>
        <div class="field-help">${t('config.llm_guardian.provider_help')}</div>
        <select class="field-select" data-path="llm_guardian.provider">
            <option value=""${!curProvider ? ' selected' : ''}>${t('config.llm_guardian.select_provider')}</option>`;
    configGeneralProviders().forEach(p => {
        const sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        const name = p.name || p.id;
        const badge = p.type ? (' [' + p.type + ']') : '';
        const model = p.model ? (' — ' + p.model) : '';
        html += `<option value="${escapeAttr(p.id)}"${sel}>${escapeAttr(name + badge + model)}</option>`;
    });
    html += configPreservedProviderOptionHTML(curProvider, configGeneralProviders());
    html += `</select></div>`;

    const curModel = cfg.model || '';
    html += `<div class="field-group">
        <div class="field-label">${t('config.llm_guardian.model_label')}</div>
        <div class="field-help">${t('config.llm_guardian.model_hint')}</div>
        <input type="text" class="field-input" data-path="llm_guardian.model" value="${escapeAttr(curModel)}"
            placeholder="gemini-2.0-flash, gpt-4o-mini...">
    </div></div>`;
    html += `</div>`;

    const curLevel = cfg.default_level || 'medium';
    const levels = ['off', 'low', 'medium', 'high'];
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.level_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.level_desc')}</div>
        <div class="field-group">
            <div class="field-label">${t('config.llm_guardian.level_label')}</div>
            <div class="field-help">${t('config.llm_guardian.level_help')}</div>
            <select class="field-select" data-path="llm_guardian.default_level">`;
    levels.forEach(lv => {
        const sel = (curLevel === lv) ? ' selected' : '';
        html += `<option value="${lv}"${sel}>${t('config.llm_guardian.level_' + lv)}</option>`;
    });
    html += `</select></div></div>`;

    const curFailSafe = cfg.fail_safe || 'block';
    const failSafes = ['block', 'quarantine', 'allow'];
    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.failsafe_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.failsafe_desc')}</div>
        <div class="field-group">
            <div class="field-label">${t('config.llm_guardian.failsafe_label')}</div>
            <div class="field-help">${t('config.llm_guardian.failsafe_help')}</div>
            <select class="field-select" data-path="llm_guardian.fail_safe">`;
    failSafes.forEach(fs => {
        const sel = (curFailSafe === fs) ? ' selected' : '';
        html += `<option value="${fs}"${sel}>${t('config.llm_guardian.failsafe_' + fs)}</option>`;
    });
    html += `</select></div></div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.perf_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.perf_desc')}</div>`;

    const curTTL = cfg.cache_ttl != null ? cfg.cache_ttl : 300;
    const curRate = cfg.max_checks_per_minute != null ? cfg.max_checks_per_minute : (cfg.max_checks_per_min != null ? cfg.max_checks_per_min : 60);
    html += `<div class="field-grid two-cols">
        <div class="field-group">
            <div class="field-label">${t('config.llm_guardian.cache_ttl_label')}</div>
            <div class="field-help">${t('config.llm_guardian.cache_ttl_help')}</div>
            <input type="number" class="field-input" data-path="llm_guardian.cache_ttl" value="${curTTL}" min="0" max="3600" step="30">
        </div>
        <div class="field-group">
            <div class="field-label">${t('config.llm_guardian.rate_limit_label')}</div>
            <div class="field-help">${t('config.llm_guardian.rate_limit_help')}</div>
            <input type="number" class="field-input" data-path="llm_guardian.max_checks_per_minute" value="${curRate}" min="1" max="300" step="1">
        </div>
    </div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.clarification_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.clarification_desc')}</div>`;

    const clarificationOn = cfg.allow_clarification === true;
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${clarificationOn ? 'on' : ''}" data-path="llm_guardian.allow_clarification" role="switch" aria-checked="${clarificationOn}" aria-label="${escapeAttr(t('config.llm_guardian.clarification_label'))}" aria-describedby="lg-allow_clarification-help" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.allow_clarification',this.classList.contains('on'));setDirty(true)"></div>
        <span class="cfg-toggle-label">${t('config.llm_guardian.clarification_label')}</span>
    </div>
    <div class="field-help" id="lg-allow_clarification-help">${t('config.llm_guardian.clarification_help')}</div>`;
    html += `</div>`;

    html += `<div class="field-group">
        <div class="field-group-title">${t('config.llm_guardian.scan_title')}</div>
        <div class="field-group-desc">${t('config.llm_guardian.scan_desc')}</div>`;

    const scanEmails = cfg.scan_emails === true;
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${scanEmails ? 'on' : ''}" data-path="llm_guardian.scan_emails" role="switch" aria-checked="${scanEmails}" aria-label="${escapeAttr(t('config.llm_guardian.scan_emails_label'))}" aria-describedby="lg-scan_emails-help" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.scan_emails',this.classList.contains('on'));setDirty(true)"></div>
        <span class="cfg-toggle-label">${t('config.llm_guardian.scan_emails_label')}</span>
    </div>
    <div class="field-help" id="lg-scan_emails-help">${t('config.llm_guardian.scan_emails_help')}</div>`;

    const scanDocs = cfg.scan_documents === true;
    html += `<div class="cfg-toggle-row-compact">
        <div class="toggle ${scanDocs ? 'on' : ''}" data-path="llm_guardian.scan_documents" role="switch" aria-checked="${scanDocs}" aria-label="${escapeAttr(t('config.llm_guardian.scan_documents_label'))}" aria-describedby="lg-scan_documents-help" onclick="toggleBool(this);setNestedValue(configData,'llm_guardian.scan_documents',this.classList.contains('on'));setDirty(true)"></div>
        <span class="cfg-toggle-label">${t('config.llm_guardian.scan_documents_label')}</span>
    </div>
    <div class="field-help" id="lg-scan_documents-help">${t('config.llm_guardian.scan_documents_help')}</div>`;
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
                <select class="field-select lg-select-sm" aria-label="${escapeAttr(t('config.llm_guardian.override_level_for', { tool: toolName }))}"
                    onchange="guardianSetOverride('${escapeAttr(toolName)}',this.value)">`;
            levels.forEach(lv => {
                const sel = (toolLevel === lv) ? ' selected' : '';
                html += `<option value="${lv}"${sel}>${t('config.llm_guardian.level_' + lv)}</option>`;
            });
            html += `</select>
                <button class="lg-btn-remove" title="${escapeAttr(t('config.llm_guardian.override_remove'))}" aria-label="${escapeAttr(t('config.llm_guardian.override_remove') + ': ' + toolName)}"
                    onclick="guardianRemoveOverride('${escapeAttr(toolName)}')">✕</button>
            </div>`;
        });
        html += `</div>`;
    }

    html += `<div class="field-help" id="guardian-new-help">${t('config.llm_guardian.override_add_help')}</div>`;
    html += `<div class="lg-add-row">
        <input type="text" id="guardian-new-tool" class="field-input lg-add-input" list="guardian-tool-datalist" aria-label="${escapeAttr(t('config.llm_guardian.override_tool_label'))}" aria-describedby="guardian-new-help"
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
        <select id="guardian-new-level" class="field-select lg-select-sm" aria-label="${escapeAttr(t('config.llm_guardian.override_level_label'))}" aria-describedby="guardian-new-help">`;
    levels.forEach(lv => {
        const sel = (lv === 'high') ? ' selected' : '';
        html += `<option value="${lv}"${sel}>${t('config.llm_guardian.level_' + lv)}</option>`;
    });
    html += `</select>
        <button class="lg-btn-add" title="${escapeAttr(t('config.llm_guardian.override_add'))}" aria-label="${escapeAttr(t('config.llm_guardian.override_add'))}"
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

function _guardianGetToolDesc(toolName) {
    const key = 'config.llm_guardian.tool_' + toolName;
    return tOr(key, '');
}

const _guardianHighRiskTools = new Set([
    'execute_shell', 'execute_sudo', 'execute_python', 'execute_remote_shell', 'filesystem'
]);

const _guardianRiskyTools = new Set([
    'execute_shell', 'execute_sudo', 'execute_python', 'execute_remote_shell', 'filesystem',
    'api_request', 'docker', 'proxmox', 'set_secret', 'save_tool', 'co_agent',
    'manage_updates', 'netlify', 'here_now_site', 'treg_call', 'home_assistant'
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
