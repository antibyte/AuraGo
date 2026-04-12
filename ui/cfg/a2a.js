function renderA2ASection(section) {
    const data = configData['a2a'] || {};
    const srv = data.server || {};
    const cli = data.client || {};
    const auth = data.auth || {};
    const llmCfg = data.llm || {};
    const bindings = srv.bindings || {};
    const skills = srv.skills || [];
    const remoteAgents = cli.remote_agents || [];

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    html += '<div id="a2a-status-banner" class="a2a-status-banner">' + t('config.a2a.checking') + '</div>';

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
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_description" value="' + escapeAttr(srv.agent_description || '') + '" placeholder="' + escapeAttr(t('config.a2a.agent_description_placeholder')) + '">';
    html += '</div>';

    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_version') + '</div>';
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

    html += '<div class="a2a-toggle-row">';
    html += '<div class="toggle toggle-sm' + (bindings.rest !== false ? ' on' : '') + '" data-path="a2a.server.bindings.rest" onclick="toggleBool(this)"></div>';
    html += '<span class="a2a-toggle-label">' + t('config.a2a.rest_binding') + '</span>';
    html += '</div>';

    html += '<div class="a2a-toggle-row">';
    html += '<div class="toggle toggle-sm' + (bindings.jsonrpc ? ' on' : '') + '" data-path="a2a.server.bindings.jsonrpc" onclick="toggleBool(this)"></div>';
    html += '<span class="a2a-toggle-label">' + t('config.a2a.jsonrpc_binding') + '</span>';
    html += '</div>';

    html += '<div class="a2a-toggle-row">';
    html += '<div class="toggle toggle-sm' + (bindings.grpc ? ' on' : '') + '" data-path="a2a.server.bindings.grpc" onclick="toggleBool(this)"></div>';
    html += '<span class="a2a-toggle-label">' + t('config.a2a.grpc_binding') + '</span>';
    html += '</div>';

    html += '<div class="a2a-port-row">';
    html += '<span class="a2a-toggle-label">' + t('config.a2a.grpc_port') + '</span>';
    html += '<input class="field-input a2a-input-xs" type="number" data-path="a2a.server.bindings.grpc_port" value="' + escapeAttr(String(bindings.grpc_port || 50051)) + '">';
    html += '</div>';
    html += '</div>';

    html += '<div class="a2a-card">';
    html += '<div class="a2a-card-title">⚡ ' + t('config.a2a.capabilities_title') + '</div>';

    html += '<div class="a2a-toggle-row">';
    html += '<div class="toggle toggle-sm' + (srv.streaming ? ' on' : '') + '" data-path="a2a.server.streaming" onclick="toggleBool(this)"></div>';
    html += '<span class="a2a-toggle-label">' + t('config.a2a.streaming') + '</span>';
    html += '</div>';

    html += '<div class="a2a-toggle-row-last">';
    html += '<div class="toggle toggle-sm' + (srv.push_notifications ? ' on' : '') + '" data-path="a2a.server.push_notifications" onclick="toggleBool(this)"></div>';
    html += '<span class="a2a-toggle-label">' + t('config.a2a.push_notifications') + '</span>';
    html += '</div>';
    html += '</div>';

    html += '<div class="a2a-card">';
    html += '<div class="a2a-card-title">🎯 ' + t('config.a2a.skills_title') + '</div>';
    html += '<div class="a2a-desc">' + t('config.a2a.skills_desc') + '</div>';
    html += '<div id="a2a-skills-list">';
    if (skills.length === 0) {
        html += '<div class="a2a-placeholder-sm">' + t('config.a2a.no_skills') + '</div>';
    }
    skills.forEach(function(skill, idx) {
        html += a2aSkillRow(skill, idx);
    });
    html += '</div>';
    html += '<button class="btn-save a2a-add-btn" onclick="a2aAddSkill()">+ ' + t('config.a2a.add_skill') + '</button>';
    html += '</div>';

    html += '<hr class="cfg-section-hr">';
    html += '<div class="a2a-section-title">🔐 ' + t('config.a2a.auth_title') + '</div>';

    html += '<div class="a2a-card">';
    html += '<div class="a2a-toggle-row-mb">';
    html += '<div class="toggle toggle-sm' + (auth.api_key_enabled ? ' on' : '') + '" data-path="a2a.auth.api_key_enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="a2a-toggle-label-bold">' + t('config.a2a.api_key_auth') + '</span>';
    html += '</div>';
    html += '<div class="a2a-field-row">';
    html += '<input class="field-input a2a-input-flex" type="password" id="a2a-api-key" placeholder="' + escapeAttr(t('config.a2a.api_key_placeholder')) + '">';
    html += '<button class="btn-save cfg-save-btn-sm" onclick="a2aSaveVault(\'a2a_api_key\', \'a2a-api-key\')">💾 ' + t('config.a2a.save_vault') + '</button>';
    html += '</div>';
    html += '<span id="a2a-api-key-status" class="a2a-status-text"></span>';
    html += '</div>';

    html += '<div class="a2a-card">';
    html += '<div class="a2a-toggle-row-mb">';
    html += '<div class="toggle toggle-sm' + (auth.bearer_enabled ? ' on' : '') + '" data-path="a2a.auth.bearer_enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="a2a-toggle-label-bold">' + t('config.a2a.bearer_auth') + '</span>';
    html += '</div>';
    html += '<div class="a2a-field-row">';
    html += '<input class="field-input a2a-input-flex" type="password" id="a2a-bearer-secret" placeholder="' + escapeAttr(t('config.a2a.bearer_placeholder')) + '">';
    html += '<button class="btn-save cfg-save-btn-sm" onclick="a2aSaveVault(\'a2a_bearer_secret\', \'a2a-bearer-secret\')">💾 ' + t('config.a2a.save_vault') + '</button>';
    html += '</div>';
    html += '<span id="a2a-bearer-status" class="a2a-status-text"></span>';
    html += '</div>';

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
    html += '<select class="field-input a2a-input-lg" data-path="a2a.llm.provider">';
    html += '<option value="">' + t('config.a2a.llm_use_main') + '</option>';
    var providers = configData.providers || [];
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

    html += '<div id="a2a-remote-agents">';
    if (remoteAgents.length === 0) {
        html += '<div class="a2a-placeholder">' + t('config.a2a.no_remote_agents') + '</div>';
    }
    remoteAgents.forEach(function(ra, idx) {
        html += a2aRemoteAgentCard(ra, idx);
    });
    html += '</div>';
    html += '<button class="btn-save a2a-add-btn-lg" onclick="a2aAddRemoteAgent()">+ ' + t('config.a2a.add_remote_agent') + '</button>';

    if (srv.enabled) {
        html += '<hr class="cfg-section-hr">';
        html += '<div class="field-group">';
        html += '<button class="btn-save a2a-test-btn" onclick="a2aTestServer()" id="a2a-test-btn">🔌 ' + t('config.a2a.test_btn') + '</button>';
        html += '<span id="a2a-test-result" class="a2a-test-result"></span>';
        html += '</div>';
    }

    html += '</div>';
    document.getElementById('content').innerHTML = html;

    a2aCheckStatus();
}

function a2aSkillRow(skill, idx) {
    var h = '<div class="a2a-skill-row" data-skill-idx="' + idx + '">';
    h += '<input class="field-input a2a-input-xs" type="text" data-path="a2a.server.skills.' + idx + '.id" value="' + escapeAttr(skill.id || '') + '" placeholder="ID">';
    h += '<input class="field-input a2a-input-flex" type="text" data-path="a2a.server.skills.' + idx + '.name" value="' + escapeAttr(skill.name || '') + '" placeholder="' + escapeAttr(t('config.a2a.skill_name')) + '">';
    h += '<input class="field-input a2a-input-flex-2" type="text" data-path="a2a.server.skills.' + idx + '.description" value="' + escapeAttr(skill.description || '') + '" placeholder="' + escapeAttr(t('config.a2a.skill_description')) + '">';
    h += '<button class="btn btn-sm btn-danger" onclick="a2aRemoveSkill(' + idx + ')">✕</button>';
    h += '</div>';
    return h;
}

function a2aAddSkill() {
    var skills = configData.a2a && configData.a2a.server && configData.a2a.server.skills ? configData.a2a.server.skills : [];
    skills.push({ id: '', name: '', description: '', tags: [] });
    if (!configData.a2a) configData.a2a = {};
    if (!configData.a2a.server) configData.a2a.server = {};
    configData.a2a.server.skills = skills;
    var container = document.getElementById('a2a-skills-list');
    if (container) {
        var placeholder = container.querySelector('.a2a-placeholder-sm');
        if (placeholder) placeholder.remove();
        container.insertAdjacentHTML('beforeend', a2aSkillRow(skills[skills.length - 1], skills.length - 1));
    }
    markDirty();
}

function a2aRemoveSkill(idx) {
    if (!configData.a2a || !configData.a2a.server || !configData.a2a.server.skills) return;
    configData.a2a.server.skills.splice(idx, 1);
    markDirty();
    var container = document.getElementById('a2a-skills-list');
    if (container) {
        var html = '';
        var skills = configData.a2a.server.skills;
        if (skills.length === 0) {
            html = '<div class="a2a-placeholder-sm">' + t('config.a2a.no_skills') + '</div>';
        }
        skills.forEach(function(skill, i) { html += a2aSkillRow(skill, i); });
        container.innerHTML = html;
    }
}

function a2aRemoteAgentCard(ra, idx) {
    var on = ra.enabled !== false;
    var h = '<div class="a2a-agent-card" data-ra-idx="' + idx + '">';
    h += '<div class="a2a-agent-header">';
    h += '<div class="a2a-agent-title-area">';
    h += '<div class="toggle toggle-sm' + (on ? ' on' : '') + '" data-path="a2a.client.remote_agents.' + idx + '.enabled" onclick="toggleBool(this)"></div>';
    h += '<span class="a2a-agent-name">' + escapeAttr(ra.name || ra.id || t('config.a2a.unnamed_agent')) + '</span>';
    h += '<span id="a2a-ra-status-' + idx + '" class="a2a-status-xs"></span>';
    h += '</div>';
    h += '<div class="a2a-btn-group">';
    h += '<button class="btn-save a2a-sm-btn" onclick="a2aTestRemoteAgent(\'' + escapeAttr(ra.id || '') + '\',' + idx + ')">🔌 ' + t('config.a2a.test_btn_short') + '</button>';
    h += '<button class="btn btn-sm btn-danger" onclick="a2aRemoveRemoteAgent(' + idx + ')">✕</button>';
    h += '</div></div>';

    h += '<div class="a2a-field-row-id">';
    h += '<input class="field-input a2a-input-sm" type="text" data-path="a2a.client.remote_agents.' + idx + '.id" value="' + escapeAttr(ra.id || '') + '" placeholder="ID">';
    h += '<input class="field-input a2a-input-flex" type="text" data-path="a2a.client.remote_agents.' + idx + '.name" value="' + escapeAttr(ra.name || '') + '" placeholder="' + escapeAttr(t('config.a2a.ra_name')) + '">';
    h += '</div>';

    h += '<input class="field-input a2a-input-full-mb" type="text" data-path="a2a.client.remote_agents.' + idx + '.card_url" value="' + escapeAttr(ra.card_url || '') + '" placeholder="https://agent.example.com/.well-known/agent-card.json">';

    h += '<div class="a2a-cred-row">';
    h += '<input class="field-input a2a-input-flex" type="password" id="a2a-ra-apikey-' + idx + '" placeholder="' + escapeAttr(t('config.a2a.ra_api_key')) + '">';
    h += '<button class="btn-save a2a-save-btn-xs" onclick="a2aSaveVault(\'a2a_remote_' + escapeAttr(ra.id || 'new') + '_api_key\', \'a2a-ra-apikey-' + idx + '\')">💾</button>';
    h += '<span id="a2a-ra-apikey-' + idx + '-status" class="a2a-status-xs"></span>';
    h += '</div>';
    h += '<div class="a2a-cred-row-last">';
    h += '<input class="field-input a2a-input-flex" type="password" id="a2a-ra-bearer-' + idx + '" placeholder="' + escapeAttr(t('config.a2a.ra_bearer')) + '">';
    h += '<button class="btn-save a2a-save-btn-xs" onclick="a2aSaveVault(\'a2a_remote_' + escapeAttr(ra.id || 'new') + '_bearer_token\', \'a2a-ra-bearer-' + idx + '\')">💾</button>';
    h += '<span id="a2a-ra-bearer-' + idx + '-status" class="a2a-status-xs"></span>';
    h += '</div>';

    h += '</div>';
    return h;
}

function a2aAddRemoteAgent() {
    if (!configData.a2a) configData.a2a = {};
    if (!configData.a2a.client) configData.a2a.client = {};
    if (!configData.a2a.client.remote_agents) configData.a2a.client.remote_agents = [];
    var agents = configData.a2a.client.remote_agents;
    agents.push({ id: '', name: '', card_url: '', enabled: true });
    var container = document.getElementById('a2a-remote-agents');
    if (container) {
        var placeholder = container.querySelector('.a2a-placeholder');
        if (placeholder) placeholder.remove();
        container.insertAdjacentHTML('beforeend', a2aRemoteAgentCard(agents[agents.length - 1], agents.length - 1));
    }
    markDirty();
}

function a2aRemoveRemoteAgent(idx) {
    if (!configData.a2a || !configData.a2a.client || !configData.a2a.client.remote_agents) return;
    configData.a2a.client.remote_agents.splice(idx, 1);
    markDirty();
    var container = document.getElementById('a2a-remote-agents');
    if (container) {
        var html = '';
        var agents = configData.a2a.client.remote_agents;
        if (agents.length === 0) {
            html = '<div class="a2a-placeholder">' + t('config.a2a.no_remote_agents') + '</div>';
        }
        agents.forEach(function(ra, i) { html += a2aRemoteAgentCard(ra, i); });
        container.innerHTML = html;
    }
}

function a2aSaveVault(vaultKey, inputId) {
    var input = document.getElementById(inputId);
    var statusEl = document.getElementById(inputId + '-status');
    var value = input ? input.value.trim() : '';
    if (!value) {
        if (statusEl) { statusEl.classList.remove('a2a-color-success'); statusEl.classList.add('a2a-color-error'); statusEl.textContent = t('config.a2a.value_empty'); }
        return;
    }
    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: vaultKey, value: value })
    })
    .then(function(r) { return r.json(); })
    .then(function(res) {
        if (res.status === 'ok' || res.success) {
            if (input) input.value = '';
            if (statusEl) { statusEl.classList.remove('a2a-color-error'); statusEl.classList.add('a2a-color-success'); statusEl.textContent = '✓ ' + t('config.a2a.saved'); }
        } else {
            if (statusEl) { statusEl.classList.remove('a2a-color-success'); statusEl.classList.add('a2a-color-error'); statusEl.textContent = '✗ ' + (res.message || t('config.a2a.save_failed')); }
        }
        setTimeout(function() { if (statusEl) { statusEl.classList.remove('a2a-color-success', 'a2a-color-error'); statusEl.textContent = ''; } }, 4000);
    })
    .catch(function() {
        if (statusEl) { statusEl.classList.remove('a2a-color-success'); statusEl.classList.add('a2a-color-error'); statusEl.textContent = '✗ ' + t('config.a2a.save_failed'); }
    });
}

function a2aCheckStatus() {
    var banner = document.getElementById('a2a-status-banner');
    if (!banner) return;
    fetch('/api/a2a/status')
        .then(function(r) { return r.json(); })
        .then(function(res) {
            if (!res.server_enabled && !res.client_enabled) {
                banner.textContent = '⚪ ' + t('config.a2a.status_disabled');
                return;
            }
            var parts = [];
            if (res.server_enabled && res.server) {
                var s = res.server;
                var bindList = [];
                if (s.rest && s.rest.enabled) bindList.push('REST');
                if (s.jsonrpc && s.jsonrpc.enabled) bindList.push('JSON-RPC');
                if (s.grpc && s.grpc.enabled) bindList.push('gRPC');
                parts.push('🟢 ' + t('config.a2a.status_server_active') + ' (' + bindList.join(', ') + ')');
                if (s.active_tasks > 0) parts.push(t('config.a2a.active_tasks', { count: String(s.active_tasks) }));
            }
            if (res.client_enabled && res.remote_agents) {
                var available = res.remote_agents.filter(function(a) { return a.available; }).length;
                parts.push('🟢 ' + t('config.a2a.status_client_active', { available: String(available), total: String(res.remote_agents.length) }));
            }
            banner.classList.add('a2a-color-success');
            banner.innerHTML = parts.join('<br>');
        })
        .catch(function() {
            banner.textContent = '⚪ ' + t('config.a2a.status_unavailable');
        });
}

function a2aTestServer() {
    var btn = document.getElementById('a2a-test-btn');
    var result = document.getElementById('a2a-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.className = 'a2a-test-result a2a-color-secondary'; result.textContent = t('config.a2a.testing'); }

    fetch('/api/a2a/test', { method: 'POST' })
        .then(function(r) { return r.json(); })
        .then(function(res) {
            if (result) {
                if (res.server && res.server.running) {
                    result.className = 'a2a-test-result a2a-color-success';
                    result.textContent = '✓ ' + t('config.a2a.test_ok');
                } else {
                    result.className = 'a2a-test-result a2a-color-error';
                    result.textContent = '✗ ' + t('config.a2a.test_fail');
                }
            }
        })
        .catch(function() {
            if (result) { result.className = 'a2a-test-result a2a-color-error'; result.textContent = '✗ ' + t('config.a2a.test_fail'); }
        })
        .finally(function() { if (btn) btn.disabled = false; });
}

function a2aTestRemoteAgent(agentId, idx) {
    var statusEl = document.getElementById('a2a-ra-status-' + idx);
    if (statusEl) { statusEl.className = 'a2a-status-xs a2a-color-secondary'; statusEl.textContent = '⏳'; }

    fetch('/api/a2a/remote-agents/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agent_id: agentId })
    })
    .then(function(r) { return r.json(); })
    .then(function(res) {
        if (statusEl) {
            if (res.available) {
                statusEl.className = 'a2a-status-xs a2a-color-success';
                statusEl.textContent = '🟢 ' + t('config.a2a.ra_connected');
            } else {
                statusEl.className = 'a2a-status-xs a2a-color-error';
                statusEl.textContent = '🔴 ' + (res.error || t('config.a2a.ra_unavailable'));
            }
        }
    })
    .catch(function() {
        if (statusEl) { statusEl.className = 'a2a-status-xs a2a-color-error'; statusEl.textContent = '🔴 ' + t('config.a2a.ra_unavailable'); }
    });
}
