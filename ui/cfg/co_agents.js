// cfg/co_agents.js — Co-Agents & Specialists config section module

let _coAgentsSection = null;

async function renderCoAgentsSection(section) {
    if (section) _coAgentsSection = section; else section = _coAgentsSection;
    const cfg = configData['co_agents'] || {};
    const enabledOn = cfg.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // ── Enabled toggle ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.co_agents.enabled_label') + '</div>';
    html += '<div class="field-help">' + t('config.co_agents.enabled_desc') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="co_agents.enabled" onclick="toggleBool(this);setNestedValue(configData,\'co_agents.enabled\',this.classList.contains(\'on\'));renderCoAgentsSection(null)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    if (!enabledOn) {
        html += '<div class="wh-notice"><span>\u{1F916}</span><div>';
        html += '<strong>' + t('config.co_agents.disabled_notice') + '</strong><br>';
        html += '<small>' + t('config.co_agents.disabled_desc') + '</small>';
        html += '</div></div></div>';
        document.getElementById('content').innerHTML = html;
        attachChangeListeners();
        return;
    }

    // ── Max Concurrent ──
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.co_agents.max_concurrent_label') + '</div>';
    html += '<div class="field-help">' + t('config.co_agents.max_concurrent_desc') + '</div>';
    html += '<input class="field-input" type="number" min="1" max="10" data-path="co_agents.max_concurrent" value="' + (cfg.max_concurrent || 3) + '" style="width:80px;">';
    html += '</div>';

    // ── LLM Provider ──
    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.co_agents.llm_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.co_agents.llm_desc') + '</div>';

    const coLLM = cfg.llm || {};
    const curProvider = coLLM.provider || '';
    html += '<label style="display:block;margin-bottom:0.6rem;">';
    html += '<span style="font-size:0.78rem;color:var(--text-secondary);">' + t('config.co_agents.provider_label') + '</span>';
    html += '<select class="cfg-input" data-path="co_agents.llm.provider" style="width:100%;margin-top:0.2rem;" onchange="setNestedValue(configData,\'co_agents.llm.provider\',this.value);setDirty(true)">';
    html += '<option value=""' + (!curProvider ? ' selected' : '') + '>' + t('config.co_agents.select_provider') + '</option>';
    providersCache.forEach(function(p) {
        var sel = (String(curProvider) === String(p.id)) ? ' selected' : '';
        var name = p.name || p.id;
        var badge = p.type ? (' [' + p.type + ']') : '';
        var model = p.model ? (' \u2014 ' + p.model) : '';
        html += '<option value="' + escapeAttr(p.id) + '"' + sel + '>' + escapeAttr(name + badge + model) + '</option>';
    });
    html += '</select></label>';
    html += '</div>';

    // ── Circuit Breaker ──
    var cb = cfg.circuit_breaker || {};
    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.co_agents.cb_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.co_agents.cb_desc') + '</div>';
    html += _coAgentNumberField('co_agents.circuit_breaker.max_tool_calls', t('config.co_agents.cb_max_tool_calls'), cb.max_tool_calls || 10);
    html += _coAgentNumberField('co_agents.circuit_breaker.timeout_seconds', t('config.co_agents.cb_timeout'), cb.timeout_seconds || 300);
    html += _coAgentNumberField('co_agents.circuit_breaker.max_tokens', t('config.co_agents.cb_max_tokens'), cb.max_tokens || 0);
    html += '</div>';

    // ══════════════════════════════════════════════
    // SPECIALISTS
    // ══════════════════════════════════════════════
    html += '<div style="margin-top:2rem;margin-bottom:1rem;font-size:1.05rem;font-weight:700;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.5rem;">';
    html += '\u{1F9E0} ' + t('config.co_agents.specialists_title');
    html += '</div>';
    html += '<div style="font-size:0.83rem;color:var(--text-secondary);margin-bottom:1.2rem;">' + t('config.co_agents.specialists_desc') + '</div>';

    var roles = [
        { key: 'researcher', icon: '\uD83D\uDD0D', color: '#4fc3f7' },
        { key: 'coder',      icon: '\uD83D\uDCBB', color: '#81c784' },
        { key: 'designer',   icon: '\uD83C\uDFA8', color: '#f48fb1' },
        { key: 'security',   icon: '\uD83D\uDEE1\uFE0F', color: '#ffb74d' },
        { key: 'writer',     icon: '\u270D\uFE0F',  color: '#ce93d8' }
    ];

    var specialists = cfg.specialists || {};
    for (var i = 0; i < roles.length; i++) {
        html += _renderSpecialistCard(roles[i], specialists[roles[i].key] || {});
    }

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

// ── Specialist card ──
function _renderSpecialistCard(role, spec) {
    var specEnabled = spec.enabled === true;
    var basePath = 'co_agents.specialists.' + role.key;

    var html = '<div style="border:1px solid ' + (specEnabled ? role.color + '55' : 'var(--border-subtle)') + ';border-radius:12px;padding:1rem 1.2rem;margin-bottom:1rem;background:' + (specEnabled ? role.color + '08' : 'var(--bg-secondary)') + ';transition:all 0.2s;">';

    // Header row: icon + name + toggle
    html += '<div style="display:flex;align-items:center;gap:0.8rem;">';
    html += '<span style="font-size:1.4rem;">' + role.icon + '</span>';
    html += '<div style="flex:1;">';
    html += '<div style="font-weight:600;font-size:0.92rem;color:' + (specEnabled ? role.color : 'var(--text-primary)') + ';">' + t('config.co_agents.spec_' + role.key + '_name') + '</div>';
    html += '<div style="font-size:0.78rem;color:var(--text-secondary);margin-top:0.15rem;">' + t('config.co_agents.spec_' + role.key + '_desc') + '</div>';
    html += '</div>';
    html += '<div class="toggle' + (specEnabled ? ' on' : '') + '" data-path="' + basePath + '.enabled" onclick="toggleBool(this);_coAgentSetSpec(\'' + basePath + '.enabled\',this.classList.contains(\'on\'))"></div>';
    html += '</div>';

    if (!specEnabled) {
        html += '</div>';
        return html;
    }

    // Expanded content
    html += '<div style="margin-top:1rem;padding-top:0.8rem;border-top:1px solid var(--border-subtle);">';

    // Provider dropdown
    var specLLM = spec.llm || {};
    var specProvider = specLLM.provider || '';
    html += '<label style="display:block;margin-bottom:0.7rem;">';
    html += '<span style="font-size:0.78rem;color:var(--text-secondary);">' + t('config.co_agents.spec_provider_label') + '</span>';
    html += '<select class="cfg-input" data-path="' + basePath + '.llm.provider" style="width:100%;margin-top:0.2rem;" onchange="setNestedValue(configData,\'' + basePath + '.llm.provider\',this.value);setDirty(true)">';
    html += '<option value=""' + (!specProvider ? ' selected' : '') + '>' + t('config.co_agents.spec_inherit_provider') + '</option>';
    providersCache.forEach(function(p) {
        var sel = (String(specProvider) === String(p.id)) ? ' selected' : '';
        var name = p.name || p.id;
        var badge = p.type ? (' [' + p.type + ']') : '';
        var model = p.model ? (' \u2014 ' + p.model) : '';
        html += '<option value="' + escapeAttr(p.id) + '"' + sel + '>' + escapeAttr(name + badge + model) + '</option>';
    });
    html += '</select></label>';

    // Circuit breaker overrides
    html += '<div style="font-size:0.78rem;font-weight:600;color:var(--text-secondary);margin-bottom:0.5rem;">' + t('config.co_agents.spec_cb_overrides') + '</div>';
    var specCB = spec.circuit_breaker || {};
    html += '<div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:0.5rem;">';
    html += _coAgentSmallNumber(basePath + '.circuit_breaker.max_tool_calls', t('config.co_agents.cb_max_tool_calls'), specCB.max_tool_calls || 0);
    html += _coAgentSmallNumber(basePath + '.circuit_breaker.timeout_seconds', t('config.co_agents.cb_timeout'), specCB.timeout_seconds || 0);
    html += _coAgentSmallNumber(basePath + '.circuit_breaker.max_tokens', t('config.co_agents.cb_max_tokens'), specCB.max_tokens || 0);
    html += '</div>';
    html += '<div style="font-size:0.72rem;color:var(--text-tertiary);margin-top:0.3rem;">' + t('config.co_agents.spec_cb_hint') + '</div>';

    html += '</div>'; // expanded
    html += '</div>'; // card
    return html;
}

function _coAgentSetSpec(path, value) {
    setNestedValue(configData, path, value);
    setDirty(true);
    renderCoAgentsSection(null);
}

function _coAgentNumberField(path, label, value) {
    return '<label style="display:flex;align-items:center;gap:0.6rem;margin-bottom:0.5rem;">' +
        '<span style="font-size:0.8rem;color:var(--text-secondary);min-width:140px;">' + label + '</span>' +
        '<input class="cfg-input" type="number" min="0" data-path="' + path + '" value="' + value + '" style="width:90px;">' +
        '</label>';
}

function _coAgentSmallNumber(path, label, value) {
    return '<label style="display:block;">' +
        '<span style="font-size:0.72rem;color:var(--text-secondary);display:block;margin-bottom:0.15rem;">' + label + '</span>' +
        '<input class="cfg-input" type="number" min="0" data-path="' + path + '" value="' + value + '" style="width:100%;">' +
        '</label>';
}
