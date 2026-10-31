function renderA2ASection(section) {
    const skills = a2aList('a2a.server.skills');
    const remoteAgents = a2aList('a2a.client.remote_agents');
    const data = configData['a2a'] || {};
    const srv = data.server || {};
    const cli = data.client || {};
    const auth = data.auth || {};
    const llmCfg = data.llm || {};
    const bindings = srv.bindings || {};
    const authAPIKeyPlaceholder = cfgSecretPlaceholder(auth.api_key, t('config.a2a.api_key_placeholder'));
    const authBearerPlaceholder = cfgSecretPlaceholder(auth.bearer_secret, t('config.a2a.bearer_placeholder'));

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += cfgStatusBanner.html('a2a-status-banner', (srv.enabled || cli.enabled) ? 'checking' : 'off', { detail: (srv.enabled || cli.enabled) ? '' : t('config.a2a.status_disabled') });

    html += '<hr class="cfg-section-hr">';
    html += '<div class="a2a-section-title">🖥️ ' + t('config.a2a.server_title') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.server_enabled') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.server_enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (srv.enabled ? ' on' : '') + '" data-path="a2a.server.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (srv.enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_name') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.agent_name') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_name" value="' + escapeAttr(srv.agent_name || t('config.a2a.agent_name_placeholder')) + '" placeholder="' + escapeAttr(t('config.a2a.agent_name_placeholder')) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_description') + '</div>';
    html += '<div class="field-help">' + t('config.a2a.help_agent_description') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_description" value="' + escapeAttr(srv.agent_description || '') + '" placeholder="' + escapeAttr(t('config.a2a.agent_description_placeholder')) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_version') + '</div>';
    html += '<div class="field-help">' + t('config.a2a.help_agent_version') + '</div>';
    html += '<input class="field-input a2a-input-sm" type="text" data-path="a2a.server.agent_version" value="' + escapeAttr(srv.agent_version || t('config.a2a.agent_version_placeholder')) + '" placeholder="' + escapeAttr(t('config.a2a.agent_version_placeholder')) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_url') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.agent_url') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_url" value="' + escapeAttr(srv.agent_url || '') + '" placeholder="' + escapeAttr(t('config.a2a.agent_url_placeholder')) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.port') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.port') + '</div>';
    html += '<input class="field-input a2a-input-sm" type="number" data-path="a2a.server.port" value="' + escapeAttr(String(srv.port || 0)) + '" placeholder="0">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.base_path') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.base_path') + '</div>';
    html += '<input class="field-input a2a-input-md" type="text" data-path="a2a.server.base_path" value="' + escapeAttr(srv.base_path || '/a2a') + '" placeholder="/a2a">';
    html += '</div>';

    html += '<div class="a2a-card">';
    html += '<div class="a2a-card-title">🔌 ' + t('config.a2a.bindings_title') + '</div>';

    html += AuraConfigForm.toggle({ path: 'a2a.server.bindings.rest', helpKey: 'config.a2a.help_rest_binding', value: bindings.rest !== false, labelKey: 'config.a2a.rest_binding' });
    html += AuraConfigForm.toggle({ path: 'a2a.server.bindings.jsonrpc', helpKey: 'config.a2a.help_jsonrpc', value: !!bindings.jsonrpc, labelKey: 'config.a2a.jsonrpc_binding' });
    html += AuraConfigForm.toggle({ path: 'a2a.server.bindings.grpc', helpKey: 'config.a2a.help_grpc', value: !!bindings.grpc, labelKey: 'config.a2a.grpc_binding' });
    html += AuraConfigForm.number({ path: 'a2a.server.bindings.grpc_port', helpKey: 'config.a2a.help_grpc_port', value: bindings.grpc_port || 50051, labelKey: 'config.a2a.grpc_port' });
    html += '</div>';

    html += '<div class="a2a-card">';
    html += '<div class="a2a-card-title">⚡ ' + t('config.a2a.capabilities_title') + '</div>';

    html += AuraConfigForm.toggle({ path: 'a2a.server.streaming', helpKey: 'config.a2a.help_streaming', value: !!srv.streaming, labelKey: 'config.a2a.streaming' });
    html += AuraConfigForm.toggle({ path: 'a2a.server.push_notifications', helpKey: 'config.a2a.help_push', value: !!srv.push_notifications, labelKey: 'config.a2a.push_notifications' });
    html += '</div>';

    html += '<div class="a2a-card">';
    html += '<div class="a2a-card-title">🎯 ' + t('config.a2a.skills_title') + '</div>';
    html += '<div class="a2a-desc">' + t('config.a2a.skills_desc') + '</div>';
    html += '<div class="field-help" id="a2a-skills-help">' + t('config.a2a.skills_fields_help') + '</div>';
    html += '<div id="a2a-skills-list">' + a2aSkillRows(skills) + '</div>';
    html += '<button class="btn-save a2a-add-btn" onclick="a2aAddSkill()">+ ' + t('config.a2a.add_skill') + '</button>';
    html += '</div>';

    html += '<hr class="cfg-section-hr">';
    html += '<div class="a2a-section-title">🔐 ' + t('config.a2a.auth_title') + '</div>';

    html += '<div class="a2a-card">';
    html += AuraConfigForm.toggle({ path: 'a2a.auth.api_key_enabled', helpKey: 'config.a2a.help_api_key_enabled', value: !!auth.api_key_enabled, labelKey: 'config.a2a.api_key_auth' });
    html += '<div class="field-group"><div class="field-label">' + t('config.a2a.api_key_label') + '</div>';
    html += '<div class="field-help">' + t('config.a2a.help_api_key') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="a2a-api-key" data-path="a2a.auth.api_key" value="' + escapeAttr(cfgSecretValue(auth.api_key)) + '" placeholder="' + escapeAttr(authAPIKeyPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '</div>';
    html += '</div></div>';

    html += '<div class="a2a-card">';
    html += AuraConfigForm.toggle({ path: 'a2a.auth.bearer_enabled', helpKey: 'config.a2a.help_bearer_enabled', value: !!auth.bearer_enabled, labelKey: 'config.a2a.bearer_auth' });
    html += '<div class="field-group"><div class="field-label">' + t('config.a2a.bearer_secret_label') + '</div>';
    html += '<div class="field-help">' + t('config.a2a.help_bearer_secret') + '</div>';
    html += '<div class="adg-password-row">';
    html += '<div class="password-wrap cfg-password-input">';
    html += '<input class="field-input adg-password-input" type="password" id="a2a-bearer-secret" data-path="a2a.auth.bearer_secret" value="' + escapeAttr(cfgSecretValue(auth.bearer_secret)) + '" placeholder="' + escapeAttr(authBearerPlaceholder) + '" autocomplete="off">';
    html += '<button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">' + EYE_OPEN_SVG + '</button>';
    html += '</div>';
    html += '</div>';
    html += '</div></div>';

    if (!auth.api_key_enabled && !auth.bearer_enabled) {
        html += '<div class="wh-notice a2a-warning-notice">';
        html += '<span>⚠️</span><div><small>' + t('config.a2a.no_auth_warning') + '</small></div>';
        html += '</div>';
    }

    html += '<hr class="cfg-section-hr">';
    html += '<div class="a2a-section-title">🧠 ' + t('config.a2a.llm_title') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.llm_provider') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.llm_provider') + '</div>';
    html += '<select class="field-select a2a-input-lg" data-path="a2a.llm.provider">';
    html += '<option value="">' + t('config.a2a.llm_use_main') + '</option>';
    var providers = configGeneralProviders(configData.providers || []);
    var preservedProvider = configPreservedProviderOptionHTML(llmCfg.provider, providers);
    if (preservedProvider) preservedProvider = preservedProvider.replace('<option ', '<option disabled ');
    html += preservedProvider;
    providers.forEach(function(p) {
        var sel = (llmCfg.provider === p.id) ? ' selected' : '';
        html += '<option value="' + escapeAttr(p.id) + '"' + sel + '>' + escapeAttr(p.name || p.id) + '</option>';
    });
    html += '</select>';
    html += '</div>';

    html += '<hr class="cfg-section-hr">';
    html += '<div class="a2a-section-title">🌍 ' + t('config.a2a.client_title') + '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.client_enabled') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.client_enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (cli.enabled ? ' on' : '') + '" data-path="a2a.client.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (cli.enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    html += '<div class="field-help" id="a2a-remote-help">' + t('config.a2a.remote_agents_help') + '</div>';
    html += '<div id="a2a-remote-agents">' + a2aRemoteAgentCards(remoteAgents) + '</div>';
    html += '<button class="btn-save a2a-add-btn-lg" onclick="a2aAddRemoteAgent()">+ ' + t('config.a2a.add_remote_agent') + '</button>';

    if (srv.enabled) {
        html += '<hr class="cfg-section-hr">';
        html += '<div class="field-group">';
        html += '<button class="btn-save a2a-test-btn" onclick="a2aTestServer()" id="a2a-test-btn" data-config-action-draft="false">🔌 ' + t('config.a2a.test_btn') + '</button>';
        html += cfgActionResult.html('a2a-test-result');
        html += '</div>';
    }

    html += '</div>';
    document.getElementById('content').innerHTML = html;

    a2aCheckStatus();
}

// The server replaces a2a.server.skills and a2a.client.remote_agents wholesale,
// so the draft treats each list as one value. Rows edit the draft; adopt it
// before rendering or changing a list so typed edits and fields the rows do not
// show (skill tags, the masked agent credentials) survive.
function a2aList(path) {
    var keys = path.split('.');
    if (!configData.a2a) configData.a2a = {};
    if (!configData.a2a[keys[1]]) configData.a2a[keys[1]] = {};
    var owner = configData.a2a[keys[1]];
    var state = window.AuraConfigState;
    if (state && typeof state.markAtomic === 'function') {
        state.markAtomic(path);
        var draft = state.get(path);
        if (Array.isArray(draft)) owner[keys[2]] = JSON.parse(JSON.stringify(draft));
    }
    if (!Array.isArray(owner[keys[2]])) owner[keys[2]] = [];
    return owner[keys[2]];
}

// Only the list container re-renders, before any listener reads the controls
// back into the draft; fields outside it keep what was typed.
function a2aSetList(path, list, render) {
    if (window.AuraConfigState) window.AuraConfigState.replace(path, list, render);
    else render();
    setDirty(true);
}

function a2aSkillRows(skills) {
    if (skills.length === 0) return '<div class="a2a-placeholder-sm">' + t('config.a2a.no_skills') + '</div>';
    return skills.map(function(skill, idx) { return a2aSkillRow(skill, idx); }).join('');
}

function a2aRenderSkills() {
    var container = document.getElementById('a2a-skills-list');
    if (!container) return;
    container.innerHTML = a2aSkillRows(a2aList('a2a.server.skills'));
    attachChangeListeners();
}

function a2aSkillRow(skill, idx) {
    var h = '<div class="a2a-skill-row" data-skill-idx="' + idx + '">';
    h += '<input class="field-input a2a-input-xs" type="text" data-path="a2a.server.skills.' + idx + '.id" value="' + escapeAttr(skill.id || '') + '" placeholder="ID" aria-label="' + escapeAttr(t('config.a2a.skill_id')) + '" aria-describedby="a2a-skills-help">';
    h += '<input class="field-input a2a-input-flex" type="text" data-path="a2a.server.skills.' + idx + '.name" value="' + escapeAttr(skill.name || '') + '" placeholder="' + escapeAttr(t('config.a2a.skill_name')) + '" aria-label="' + escapeAttr(t('config.a2a.skill_name')) + '" aria-describedby="a2a-skills-help">';
    h += '<input class="field-input a2a-input-flex-2" type="text" data-path="a2a.server.skills.' + idx + '.description" value="' + escapeAttr(skill.description || '') + '" placeholder="' + escapeAttr(t('config.a2a.skill_description')) + '" aria-label="' + escapeAttr(t('config.a2a.skill_description')) + '" aria-describedby="a2a-skills-help">';
    h += '<button class="btn btn-sm btn-danger" onclick="a2aRemoveSkill(' + idx + ')">✕</button>';
    h += '</div>';
    return h;
}

function a2aAddSkill() {
    var skills = a2aList('a2a.server.skills');
    skills.push({ id: '', name: '', description: '', tags: [] });
    a2aSetList('a2a.server.skills', skills, a2aRenderSkills);
}

async function a2aRemoveSkill(idx) {
    var skills = a2aList('a2a.server.skills');
    var skillDoomed = skills[idx] || {};
    if (!(await showConfirm(t('config.a2a.confirm_remove_skill_title'), t('config.a2a.confirm_remove_skill_message', { name: skillDoomed.name || skillDoomed.id || String(idx + 1) })))) return;
    skills.splice(idx, 1);
    a2aSetList('a2a.server.skills', skills, a2aRenderSkills);
}

function a2aRemoteAgentCards(agents) {
    if (agents.length === 0) return '<div class="a2a-placeholder">' + t('config.a2a.no_remote_agents') + '</div>';
    return agents.map(function(ra, idx) { return a2aRemoteAgentCard(ra, idx); }).join('');
}

function a2aRenderRemoteAgents() {
    var container = document.getElementById('a2a-remote-agents');
    if (!container) return;
    container.innerHTML = a2aRemoteAgentCards(a2aList('a2a.client.remote_agents'));
    attachChangeListeners();
}

function a2aRemoteAgentCard(ra, idx) {
    var on = ra.enabled !== false;
    var apiKeyPlaceholder = cfgSecretPlaceholder(ra.api_key, t('config.a2a.ra_api_key'));
    var bearerPlaceholder = cfgSecretPlaceholder(ra.bearer_token, t('config.a2a.ra_bearer'));
    var h = '<div class="a2a-agent-card" data-ra-idx="' + idx + '">';
    h += '<div class="a2a-agent-header">';
    h += '<div class="a2a-agent-title-area">';
    h += '<div class="toggle toggle-sm' + (on ? ' on' : '') + '" data-path="a2a.client.remote_agents.' + idx + '.enabled" onclick="toggleBool(this)"></div>';
    h += '<span class="a2a-agent-name">' + escapeAttr(ra.name || ra.id || t('config.a2a.unnamed_agent')) + '</span>';
    h += cfgActionResult.html('a2a-ra-status-' + idx, 'a2a-status-xs');
    h += '</div>';
    h += '<div class="a2a-btn-group">';
    h += '<button id="a2a-ra-test-' + idx + '" class="btn-save a2a-sm-btn" data-config-action-draft="false" onclick="a2aTestRemoteAgent(\'' + escapeAttr(ra.id || '') + '\',' + idx + ')">🔌 ' + t('config.a2a.test_btn_short') + '</button>';
    h += '<button class="btn btn-sm btn-danger" onclick="a2aRemoveRemoteAgent(' + idx + ')">✕</button>';
    h += '</div></div>';

    h += '<div class="a2a-field-row-id">';
    h += '<input class="field-input a2a-input-sm" type="text" data-path="a2a.client.remote_agents.' + idx + '.id" value="' + escapeAttr(ra.id || '') + '" placeholder="ID" aria-label="' + escapeAttr(t('config.a2a.ra_id')) + '" aria-describedby="a2a-remote-help">';
    h += '<input class="field-input a2a-input-flex" type="text" data-path="a2a.client.remote_agents.' + idx + '.name" value="' + escapeAttr(ra.name || '') + '" placeholder="' + escapeAttr(t('config.a2a.ra_name')) + '" aria-label="' + escapeAttr(t('config.a2a.ra_name')) + '" aria-describedby="a2a-remote-help">';
    h += '</div>';

    h += '<input class="field-input a2a-input-full-mb" type="text" data-path="a2a.client.remote_agents.' + idx + '.card_url" value="' + escapeAttr(ra.card_url || '') + '" placeholder="https://agent.example.com/.well-known/agent-card.json" aria-label="' + escapeAttr(t('config.a2a.ra_card_url')) + '" aria-describedby="a2a-remote-help">';

    h += '<div class="a2a-cred-row">';
    h += '<input class="field-input a2a-input-flex" type="password" id="a2a-ra-apikey-' + idx + '" data-path="a2a.client.remote_agents.' + idx + '.api_key" value="' + escapeAttr(cfgSecretValue(ra.api_key)) + '" placeholder="' + escapeAttr(apiKeyPlaceholder) + '" aria-label="' + escapeAttr(t('config.a2a.ra_api_key_label')) + '" aria-describedby="a2a-remote-help">';
    h += '</div>';
    h += '<div class="a2a-cred-row-last">';
    h += '<input class="field-input a2a-input-flex" type="password" id="a2a-ra-bearer-' + idx + '" data-path="a2a.client.remote_agents.' + idx + '.bearer_token" value="' + escapeAttr(cfgSecretValue(ra.bearer_token)) + '" placeholder="' + escapeAttr(bearerPlaceholder) + '" aria-label="' + escapeAttr(t('config.a2a.ra_bearer_label')) + '" aria-describedby="a2a-remote-help">';
    h += '</div>';

    h += '</div>';
    return h;
}

function a2aAddRemoteAgent() {
    var agents = a2aList('a2a.client.remote_agents');
    agents.push({ id: '', name: '', card_url: '', enabled: true });
    a2aSetList('a2a.client.remote_agents', agents, a2aRenderRemoteAgents);
}

async function a2aRemoveRemoteAgent(idx) {
    var agents = a2aList('a2a.client.remote_agents');
    var agentDoomed = agents[idx] || {};
    if (!(await showConfirm(t('config.a2a.confirm_remove_agent_title'), t('config.a2a.confirm_remove_agent_message', { name: agentDoomed.name || agentDoomed.id || String(idx + 1) })))) return;
    agents.splice(idx, 1);
    a2aSetList('a2a.client.remote_agents', agents, a2aRenderRemoteAgents);
}

function a2aCheckStatus() {
    var banner = document.getElementById('a2a-status-banner');
    if (!banner) return null;
    return cfgStatusBanner.check(banner, function() { return fetch('/api/a2a/status'); }, function(res) {
        if (!res) return null;
        if (!res.server_enabled && !res.client_enabled) return { state: 'off', detail: t('config.a2a.status_disabled') };
        var parts = [];
        var degraded = false;
        if (res.server_enabled && res.server) {
            var s = res.server;
            var bindList = [];
            if (s.rest && s.rest.enabled) bindList.push('REST');
            if (s.jsonrpc && s.jsonrpc.enabled) bindList.push('JSON-RPC');
            if (s.grpc && s.grpc.enabled) bindList.push('gRPC');
            parts.push(t('config.a2a.status_server_active') + ' (' + bindList.join(', ') + ')');
            if (s.active_tasks > 0) parts.push(t('config.a2a.active_tasks', { count: String(s.active_tasks) }));
        }
        if (res.client_enabled && res.remote_agents) {
            var available = res.remote_agents.filter(function(a) { return a.available; }).length;
            parts.push(t('config.a2a.status_client_active', { available: String(available), total: String(res.remote_agents.length) }));
            degraded = res.remote_agents.length > 0 && available < res.remote_agents.length;
        }
        if (!parts.length) return { state: 'unknown', detail: t('config.a2a.status_unavailable') };
        return { state: degraded ? 'degraded' : 'connected', detail: parts.join(' · ') };
    });
}

// Runtime check of the running server: it needs the saved configuration.
function a2aTestServer() {
    var btn = document.getElementById('a2a-test-btn');
    return cfgActionResult(btn, function() { return fetch('/api/a2a/test', { method: 'POST' }); }, {
        target: 'a2a-test-result',
        pending: t('config.a2a.testing'),
        success: t('config.a2a.test_ok'),
        failure: t('config.a2a.test_fail'),
        interpret: function(outcome) {
            var body = outcome.body || {};
            return body.server && body.server.running ? {} : { state: 'error' };
        }
    });
}

function a2aTestRemoteAgent(agentId, idx) {
    var btn = document.getElementById('a2a-ra-test-' + idx);
    return cfgActionResult(btn, function() {
        return fetch('/api/a2a/remote-agents/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ agent_id: agentId })
        });
    }, {
        target: 'a2a-ra-status-' + idx,
        pending: t('config.a2a.testing'),
        success: t('config.a2a.ra_connected'),
        failure: t('config.a2a.ra_unavailable'),
        interpret: function(outcome) {
            var body = outcome.body || {};
            return body.available === false ? { state: 'error', reason: body.error || '' } : {};
        }
    });
}
