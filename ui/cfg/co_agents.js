let _coAgentsSection = null;

async function renderCoAgentsSection(section) {
    if (section) _coAgentsSection = section; else section = _coAgentsSection;
    const cfg = configData['co_agents'] || {};
    const enabledOn = cfg.enabled === true;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

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

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.co_agents.max_concurrent_label') + '</div>';
    html += '<div class="field-help">' + t('config.co_agents.max_concurrent_desc') + '</div>';
    html += '<input class="field-input ca-concurrent-input" type="number" min="1" max="10" data-path="co_agents.max_concurrent" value="' + (cfg.max_concurrent || 3) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.co_agents.llm_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.co_agents.llm_desc') + '</div>';

    const coLLM = cfg.llm || {};
    const curProvider = coLLM.provider || '';
    html += '<label class="ca-provider-label">';
    html += '<span class="ca-provider-caption">' + t('config.co_agents.provider_label') + '</span>';
    html += '<select class="field-input ca-provider-select" data-path="co_agents.llm.provider" onchange="setNestedValue(configData,\'co_agents.llm.provider\',this.value);setDirty(true)">';
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

    var cb = cfg.circuit_breaker || {};
    html += '<div class="field-group">';
    html += '<div class="field-group-title">' + t('config.co_agents.cb_title') + '</div>';
    html += '<div class="field-group-desc">' + t('config.co_agents.cb_desc') + '</div>';
    html += _coAgentNumberField('co_agents.circuit_breaker.max_tool_calls', t('config.co_agents.cb_max_tool_calls'), cb.max_tool_calls || 10);
    html += _coAgentNumberField('co_agents.circuit_breaker.timeout_seconds', t('config.co_agents.cb_timeout'), cb.timeout_seconds || 300);
    html += _coAgentNumberField('co_agents.circuit_breaker.max_tokens', t('config.co_agents.cb_max_tokens'), cb.max_tokens || 0);
    html += '</div>';

    html += '<div class="ca-specialists-heading">';
    html += '\u{1F9E0} ' + t('config.co_agents.specialists_title');
    html += '</div>';
    html += '<div class="ca-specialists-desc">' + t('config.co_agents.specialists_desc') + '</div>';

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

function _renderSpecialistCard(role, spec) {
    var specEnabled = spec.enabled === true;
    var basePath = 'co_agents.specialists.' + role.key;

    var html = '<div class="ca-card' + (specEnabled ? '' : ' is-disabled') + '" style="--ca-role-color:' + role.color + '">';

    html += '<div class="ca-card-header">';
    html += '<span class="ca-card-icon">' + role.icon + '</span>';
    html += '<div class="ca-card-info">';
    html += '<div class="ca-card-name">' + t('config.co_agents.spec_' + role.key + '_name') + '</div>';
    html += '<div class="ca-card-desc">' + t('config.co_agents.spec_' + role.key + '_desc') + '</div>';
    html += '</div>';
    html += '<div class="toggle' + (specEnabled ? ' on' : '') + '" data-path="' + basePath + '.enabled" onclick="toggleBool(this);_coAgentSetSpec(\'' + basePath + '.enabled\',this.classList.contains(\'on\'))"></div>';
    html += '</div>';

    if (!specEnabled) {
        html += '</div>';
        return html;
    }

    html += '<div class="ca-card-body">';

    var specLLM = spec.llm || {};
    var specProvider = specLLM.provider || '';
    html += '<label class="ca-spec-label">';
    html += '<span class="ca-spec-caption">' + t('config.co_agents.spec_provider_label') + '</span>';
    html += '<select class="field-input ca-spec-select" data-path="' + basePath + '.llm.provider" onchange="setNestedValue(configData,\'' + basePath + '.llm.provider\',this.value);setDirty(true)">';
    html += '<option value=""' + (!specProvider ? ' selected' : '') + '>' + t('config.co_agents.spec_inherit_provider') + '</option>';
    providersCache.forEach(function(p) {
        var sel = (String(specProvider) === String(p.id)) ? ' selected' : '';
        var name = p.name || p.id;
        var badge = p.type ? (' [' + p.type + ']') : '';
        var model = p.model ? (' \u2014 ' + p.model) : '';
        html += '<option value="' + escapeAttr(p.id) + '"' + sel + '>' + escapeAttr(name + badge + model) + '</option>';
    });
    html += '</select></label>';

    html += '<div class="ca-cb-title">' + t('config.co_agents.spec_cb_overrides') + '</div>';
    var specCB = spec.circuit_breaker || {};
    html += '<div class="ca-cb-grid">';
    html += _coAgentSmallNumber(basePath + '.circuit_breaker.max_tool_calls', t('config.co_agents.cb_max_tool_calls'), specCB.max_tool_calls || 0);
    html += _coAgentSmallNumber(basePath + '.circuit_breaker.timeout_seconds', t('config.co_agents.cb_timeout'), specCB.timeout_seconds || 0);
    html += _coAgentSmallNumber(basePath + '.circuit_breaker.max_tokens', t('config.co_agents.cb_max_tokens'), specCB.max_tokens || 0);
    html += '</div>';
    html += '<div class="ca-cb-hint">' + t('config.co_agents.spec_cb_hint') + '</div>';

    html += '<div class="ca-section-divider"></div>';
    html += '<div class="ca-cb-title">' + t('config.co_agents.spec_extra_title') + '</div>';

    html += '<label class="ca-spec-label">';
    html += '<span class="ca-spec-caption">' + t('config.co_agents.spec_additional_prompt_label') + '</span>';
    html += '<textarea class="field-input ca-spec-textarea" rows="3" data-path="' + basePath + '.additional_prompt" placeholder="' + escapeAttr(t('config.co_agents.spec_additional_prompt_placeholder')) + '" oninput="setNestedValue(configData,\'' + basePath + '.additional_prompt\',this.value);setDirty(true)">' + escapeHtml(spec.additional_prompt || '') + '</textarea>';
    html += '</label>';

    html += '<div class="ca-cheatsheet-field" id="ca-cs-' + role.key + '">';
    html += '<span class="ca-spec-caption">' + t('config.co_agents.spec_cheatsheet_label') + '</span>';
    html += '<div class="ca-cheatsheet-row">';
    if (spec.cheatsheet_id) {
        html += '<span class="ca-cheatsheet-name" id="ca-cs-name-' + role.key + '">' + escapeHtml(spec._cheatsheet_name || spec.cheatsheet_id) + '</span>';
        html += '<button class="btn btn-sm btn-secondary" onclick="_coAgentRemoveCheatsheet(\'' + role.key + '\')">' + t('config.co_agents.spec_cheatsheet_remove') + '</button>';
    } else {
        html += '<span class="ca-cheatsheet-none" id="ca-cs-name-' + role.key + '">' + t('config.co_agents.spec_cheatsheet_none') + '</span>';
        html += '<button class="btn btn-sm btn-secondary" onclick="_coAgentPickCheatsheet(\'' + role.key + '\')">' + t('config.co_agents.spec_cheatsheet_pick') + '</button>';
    }
    html += '</div></div>';

    html += '</div>';
    html += '</div>';
    return html;
}

function _coAgentSetSpec(path, value) {
    setNestedValue(configData, path, value);
    setDirty(true);
    renderCoAgentsSection(null);
}

function _coAgentNumberField(path, label, value) {
    return '<label class="ca-num-field">' +
        '<span class="ca-num-label">' + label + '</span>' +
        '<input class="field-input ca-num-input" type="number" min="0" data-path="' + path + '" value="' + value + '">' +
        '</label>';
}

function _coAgentSmallNumber(path, label, value) {
    return '<label class="ca-small-num-wrap">' +
        '<span class="ca-small-num-label">' + label + '</span>' +
        '<input class="field-input ca-small-num-input" type="number" min="0" data-path="' + path + '" value="' + value + '">' +
        '</label>';
}

var _coAgentCheatsheetCache = null;

function _coAgentPickCheatsheet(roleKey) {
    var basePath = 'co_agents.specialists.' + roleKey;
    var modalId = 'ca-cs-modal';
    var existingModal = document.getElementById(modalId);
    if (existingModal) existingModal.remove();

    var modal = document.createElement('div');
    modal.className = 'modal-overlay active';
    modal.id = modalId;
    modal.innerHTML = '<div class="modal"><div class="modal-header"><h2>' + escapeHtml(t('config.co_agents.spec_cheatsheet_modal_title')) + '</h2><button class="modal-close" onclick="document.getElementById(\'ca-cs-modal\').remove()">&times;</button></div><div class="ca-cs-list" id="ca-cs-list"><div class="ca-cs-loading">' + escapeHtml(t('config.co_agents.spec_cheatsheet_loading')) + '</div></div></div>';
    document.body.appendChild(modal);

    fetch('/api/cheatsheets?active=true').then(function(r) { return r.json(); }).then(function(sheets) {
        _coAgentCheatsheetCache = sheets || [];
        var listEl = document.getElementById('ca-cs-list');
        if (!_coAgentCheatsheetCache.length) {
            listEl.innerHTML = '<div class="ca-cs-empty">' + escapeHtml(t('config.co_agents.spec_cheatsheet_empty')) + '</div>';
            return;
        }
        listEl.innerHTML = _coAgentCheatsheetCache.map(function(s) {
            return '<div class="ca-cs-item" onclick="_coAgentSelectCheatsheet(\'' + roleKey + '\',\'' + escapeAttr(s.id) + '\',\'' + escapeAttr(s.name || '') + '\')">' +
                '<span class="ca-cs-item-name">' + escapeHtml(s.name) + '</span>' +
                '<span class="ca-cs-item-preview">' + escapeHtml((s.content || '').substring(0, 80)) + '</span></div>';
        }).join('');
    }).catch(function() {
        document.getElementById('ca-cs-list').innerHTML = '<div class="ca-cs-empty">' + escapeHtml(t('config.co_agents.spec_cheatsheet_error')) + '</div>';
    });
}

function _coAgentSelectCheatsheet(roleKey, id, name) {
    var basePath = 'co_agents.specialists.' + roleKey;
    setNestedValue(configData, basePath + '.cheatsheet_id', id);
    setNestedValue(configData, basePath + '._cheatsheet_name', name);
    setDirty(true);
    var modal = document.getElementById('ca-cs-modal');
    if (modal) modal.remove();
    renderCoAgentsSection(null);
}

function _coAgentRemoveCheatsheet(roleKey) {
    var basePath = 'co_agents.specialists.' + roleKey;
    setNestedValue(configData, basePath + '.cheatsheet_id', '');
    setNestedValue(configData, basePath + '._cheatsheet_name', '');
    setDirty(true);
    renderCoAgentsSection(null);
}
