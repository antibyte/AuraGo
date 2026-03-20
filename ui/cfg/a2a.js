// cfg/a2a.js — A2A (Agent-to-Agent) protocol config panel

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

    // ── Status banner ──
    html += '<div id="a2a-status-banner" style="margin-bottom:1rem;padding:0.8rem 1rem;border-radius:10px;font-size:0.84rem;background:var(--bg-tertiary);color:var(--text-secondary);">' + t('config.a2a.checking') + '</div>';

    // ══════════════════════════════════════════════════════════════════════
    // SERVER SECTION
    // ══════════════════════════════════════════════════════════════════════
    html += '<hr style="border:none;border-top:1px solid var(--border-subtle);margin:1.5rem 0;">';
    html += '<div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:1rem;">🖥️ ' + t('config.a2a.server_title') + '</div>';

    // Server enabled
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.server_enabled') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.server_enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (srv.enabled ? ' on' : '') + '" data-path="a2a.server.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (srv.enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // Agent Name
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_name') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.agent_name') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_name" value="' + escapeAttr(srv.agent_name || 'AuraGo') + '" placeholder="AuraGo">';
    html += '</div>';

    // Agent Description
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_description') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_description" value="' + escapeAttr(srv.agent_description || '') + '" placeholder="' + escapeAttr(t('config.a2a.agent_description_placeholder')) + '">';
    html += '</div>';

    // Agent Version
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_version') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_version" value="' + escapeAttr(srv.agent_version || '1.0.0') + '" placeholder="1.0.0" style="width:120px;">';
    html += '</div>';

    // Agent URL (override)
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.agent_url') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.agent_url') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.agent_url" value="' + escapeAttr(srv.agent_url || '') + '" placeholder="https://my-agent.example.com">';
    html += '</div>';

    // Port
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.port') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.port') + '</div>';
    html += '<input class="field-input" type="number" data-path="a2a.server.port" value="' + escapeAttr(String(srv.port || 0)) + '" placeholder="0" style="width:120px;">';
    html += '</div>';

    // Base Path
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.base_path') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.base_path') + '</div>';
    html += '<input class="field-input" type="text" data-path="a2a.server.base_path" value="' + escapeAttr(srv.base_path || '/a2a') + '" placeholder="/a2a" style="width:200px;">';
    html += '</div>';

    // ── Bindings ──
    html += '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.8rem;">';
    html += '<div style="font-weight:600;font-size:0.88rem;color:var(--text-primary);margin-bottom:0.7rem;">🔌 ' + t('config.a2a.bindings_title') + '</div>';

    // REST
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.5rem;">';
    html += '<div class="toggle toggle-sm' + (bindings.rest !== false ? ' on' : '') + '" data-path="a2a.server.bindings.rest" onclick="toggleBool(this)"></div>';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">REST (HTTP)</span>';
    html += '</div>';

    // JSON-RPC
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.5rem;">';
    html += '<div class="toggle toggle-sm' + (bindings.jsonrpc ? ' on' : '') + '" data-path="a2a.server.bindings.jsonrpc" onclick="toggleBool(this)"></div>';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">JSON-RPC 2.0</span>';
    html += '</div>';

    // gRPC
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.5rem;">';
    html += '<div class="toggle toggle-sm' + (bindings.grpc ? ' on' : '') + '" data-path="a2a.server.bindings.grpc" onclick="toggleBool(this)"></div>';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">gRPC</span>';
    html += '</div>';

    // gRPC Port (only relevant when gRPC enabled)
    html += '<div style="display:flex;align-items:center;gap:0.6rem;margin-top:0.3rem;">';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.a2a.grpc_port') + '</span>';
    html += '<input class="field-input" type="number" data-path="a2a.server.bindings.grpc_port" value="' + escapeAttr(String(bindings.grpc_port || 50051)) + '" style="width:100px;">';
    html += '</div>';
    html += '</div>'; // end bindings

    // ── Capabilities ──
    html += '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.8rem;">';
    html += '<div style="font-weight:600;font-size:0.88rem;color:var(--text-primary);margin-bottom:0.7rem;">⚡ ' + t('config.a2a.capabilities_title') + '</div>';

    // Streaming
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.5rem;">';
    html += '<div class="toggle toggle-sm' + (srv.streaming ? ' on' : '') + '" data-path="a2a.server.streaming" onclick="toggleBool(this)"></div>';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.a2a.streaming') + '</span>';
    html += '</div>';

    // Push Notifications
    html += '<div style="display:flex;align-items:center;gap:0.5rem;">';
    html += '<div class="toggle toggle-sm' + (srv.push_notifications ? ' on' : '') + '" data-path="a2a.server.push_notifications" onclick="toggleBool(this)"></div>';
    html += '<span style="font-size:0.82rem;color:var(--text-secondary);">' + t('config.a2a.push_notifications') + '</span>';
    html += '</div>';
    html += '</div>'; // end capabilities

    // ── Skills ──
    html += '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.8rem;">';
    html += '<div style="font-weight:600;font-size:0.88rem;color:var(--text-primary);margin-bottom:0.7rem;">🎯 ' + t('config.a2a.skills_title') + '</div>';
    html += '<div style="font-size:0.78rem;color:var(--text-secondary);margin-bottom:0.6rem;">' + t('config.a2a.skills_desc') + '</div>';
    html += '<div id="a2a-skills-list">';
    if (skills.length === 0) {
        html += '<div style="font-size:0.8rem;color:var(--text-tertiary);padding:0.4rem 0;">' + t('config.a2a.no_skills') + '</div>';
    }
    skills.forEach(function(skill, idx) {
        html += a2aSkillRow(skill, idx);
    });
    html += '</div>';
    html += '<button class="btn-save" style="padding:0.4rem 0.8rem;font-size:0.8rem;margin-top:0.5rem;" onclick="a2aAddSkill()">+ ' + t('config.a2a.add_skill') + '</button>';
    html += '</div>'; // end skills

    // ══════════════════════════════════════════════════════════════════════
    // AUTHENTICATION SECTION
    // ══════════════════════════════════════════════════════════════════════
    html += '<hr style="border:none;border-top:1px solid var(--border-subtle);margin:1.5rem 0;">';
    html += '<div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:1rem;">🔐 ' + t('config.a2a.auth_title') + '</div>';

    // API Key auth
    html += '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.8rem;">';
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.6rem;">';
    html += '<div class="toggle toggle-sm' + (auth.api_key_enabled ? ' on' : '') + '" data-path="a2a.auth.api_key_enabled" onclick="toggleBool(this)"></div>';
    html += '<span style="font-weight:600;font-size:0.85rem;color:var(--text-primary);">' + t('config.a2a.api_key_auth') + '</span>';
    html += '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    html += '<input class="field-input" type="password" id="a2a-api-key" placeholder="' + escapeAttr(t('config.a2a.api_key_placeholder')) + '" style="flex:1;">';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;" onclick="a2aSaveVault(\'a2a_api_key\', \'a2a-api-key\')">💾 ' + t('config.a2a.save_vault') + '</button>';
    html += '</div>';
    html += '<span id="a2a-api-key-status" style="font-size:0.78rem;margin-top:0.3rem;display:block;"></span>';
    html += '</div>';

    // Bearer token auth
    html += '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.8rem;">';
    html += '<div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.6rem;">';
    html += '<div class="toggle toggle-sm' + (auth.bearer_enabled ? ' on' : '') + '" data-path="a2a.auth.bearer_enabled" onclick="toggleBool(this)"></div>';
    html += '<span style="font-weight:600;font-size:0.85rem;color:var(--text-primary);">' + t('config.a2a.bearer_auth') + '</span>';
    html += '</div>';
    html += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    html += '<input class="field-input" type="password" id="a2a-bearer-secret" placeholder="' + escapeAttr(t('config.a2a.bearer_placeholder')) + '" style="flex:1;">';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;" onclick="a2aSaveVault(\'a2a_bearer_secret\', \'a2a-bearer-secret\')">💾 ' + t('config.a2a.save_vault') + '</button>';
    html += '</div>';
    html += '<span id="a2a-bearer-status" style="font-size:0.78rem;margin-top:0.3rem;display:block;"></span>';
    html += '</div>';

    // Auth warning when both disabled
    if (!auth.api_key_enabled && !auth.bearer_enabled) {
        html += '<div class="wh-notice" style="border-color:var(--warning);background:rgba(234,179,8,0.06);">';
        html += '<span>⚠️</span><div><small>' + t('config.a2a.no_auth_warning') + '</small></div>';
        html += '</div>';
    }

    // ══════════════════════════════════════════════════════════════════════
    // LLM SECTION
    // ══════════════════════════════════════════════════════════════════════
    html += '<hr style="border:none;border-top:1px solid var(--border-subtle);margin:1.5rem 0;">';
    html += '<div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:1rem;">🧠 ' + t('config.a2a.llm_title') + '</div>';

    // Provider picker
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.llm_provider') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.llm_provider') + '</div>';
    html += '<select class="field-input" data-path="a2a.llm.provider" style="width:240px;">';
    html += '<option value="">' + t('config.a2a.llm_use_main') + '</option>';
    var providers = configData.providers || [];
    providers.forEach(function(p) {
        var sel = (llmCfg.provider === p.id) ? ' selected' : '';
        html += '<option value="' + escapeAttr(p.id) + '"' + sel + '>' + escapeAttr(p.name || p.id) + '</option>';
    });
    html += '</select>';
    html += '</div>';

    // ══════════════════════════════════════════════════════════════════════
    // CLIENT SECTION — Remote Agents
    // ══════════════════════════════════════════════════════════════════════
    html += '<hr style="border:none;border-top:1px solid var(--border-subtle);margin:1.5rem 0;">';
    html += '<div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:1rem;">🌍 ' + t('config.a2a.client_title') + '</div>';

    // Client enabled
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.a2a.client_enabled') + '</div>';
    html += '<div class="field-help">' + t('help.a2a.client_enabled') + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (cli.enabled ? ' on' : '') + '" data-path="a2a.client.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (cli.enabled ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // Remote agents list
    html += '<div id="a2a-remote-agents">';
    if (remoteAgents.length === 0) {
        html += '<div style="font-size:0.82rem;color:var(--text-tertiary);padding:0.5rem 0;">' + t('config.a2a.no_remote_agents') + '</div>';
    }
    remoteAgents.forEach(function(ra, idx) {
        html += a2aRemoteAgentCard(ra, idx);
    });
    html += '</div>';
    html += '<button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;margin-top:0.5rem;" onclick="a2aAddRemoteAgent()">+ ' + t('config.a2a.add_remote_agent') + '</button>';

    // ── Test connection ──
    if (srv.enabled) {
        html += '<hr style="border:none;border-top:1px solid var(--border-subtle);margin:1.5rem 0;">';
        html += '<div class="field-group">';
        html += '<button class="btn-save" style="padding:0.5rem 1.2rem;font-size:0.85rem;" onclick="a2aTestServer()" id="a2a-test-btn">🔌 ' + t('config.a2a.test_btn') + '</button>';
        html += '<span id="a2a-test-result" style="margin-left:0.8rem;font-size:0.83rem;"></span>';
        html += '</div>';
    }

    html += '</div>'; // close section
    document.getElementById('content').innerHTML = html;

    // Auto-check status
    a2aCheckStatus();
}

// ── Skill row helper ──
function a2aSkillRow(skill, idx) {
    var h = '<div style="display:flex;gap:0.5rem;align-items:center;margin-bottom:0.4rem;" data-skill-idx="' + idx + '">';
    h += '<input class="field-input" type="text" data-path="a2a.server.skills.' + idx + '.id" value="' + escapeAttr(skill.id || '') + '" placeholder="ID" style="width:100px;">';
    h += '<input class="field-input" type="text" data-path="a2a.server.skills.' + idx + '.name" value="' + escapeAttr(skill.name || '') + '" placeholder="' + escapeAttr(t('config.a2a.skill_name')) + '" style="flex:1;">';
    h += '<input class="field-input" type="text" data-path="a2a.server.skills.' + idx + '.description" value="' + escapeAttr(skill.description || '') + '" placeholder="' + escapeAttr(t('config.a2a.skill_description')) + '" style="flex:2;">';
    h += '<button class="btn-save" style="padding:0.3rem 0.6rem;font-size:0.78rem;background:var(--danger);" onclick="a2aRemoveSkill(' + idx + ')">✕</button>';
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
        // Remove "no skills" placeholder
        var placeholder = container.querySelector('[style*="text-tertiary"]');
        if (placeholder) placeholder.remove();
        container.insertAdjacentHTML('beforeend', a2aSkillRow(skills[skills.length - 1], skills.length - 1));
    }
    markDirty();
}

function a2aRemoveSkill(idx) {
    if (!configData.a2a || !configData.a2a.server || !configData.a2a.server.skills) return;
    configData.a2a.server.skills.splice(idx, 1);
    markDirty();
    // Re-render skills list
    var container = document.getElementById('a2a-skills-list');
    if (container) {
        var html = '';
        var skills = configData.a2a.server.skills;
        if (skills.length === 0) {
            html = '<div style="font-size:0.8rem;color:var(--text-tertiary);padding:0.4rem 0;">' + t('config.a2a.no_skills') + '</div>';
        }
        skills.forEach(function(skill, i) { html += a2aSkillRow(skill, i); });
        container.innerHTML = html;
    }
}

// ── Remote agent card helper ──
function a2aRemoteAgentCard(ra, idx) {
    var on = ra.enabled !== false;
    var h = '<div style="background:var(--bg-tertiary);border-radius:10px;padding:1rem;margin-bottom:0.6rem;" data-ra-idx="' + idx + '">';
    h += '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.6rem;">';
    h += '<div style="display:flex;align-items:center;gap:0.6rem;">';
    h += '<div class="toggle toggle-sm' + (on ? ' on' : '') + '" data-path="a2a.client.remote_agents.' + idx + '.enabled" onclick="toggleBool(this)"></div>';
    h += '<span style="font-weight:600;font-size:0.85rem;color:var(--text-primary);">' + escapeAttr(ra.name || ra.id || t('config.a2a.unnamed_agent')) + '</span>';
    h += '<span id="a2a-ra-status-' + idx + '" style="font-size:0.75rem;"></span>';
    h += '</div>';
    h += '<div style="display:flex;gap:0.4rem;">';
    h += '<button class="btn-save" style="padding:0.3rem 0.6rem;font-size:0.75rem;" onclick="a2aTestRemoteAgent(\'' + escapeAttr(ra.id || '') + '\',' + idx + ')">🔌 ' + t('config.a2a.test_btn_short') + '</button>';
    h += '<button class="btn-save" style="padding:0.3rem 0.6rem;font-size:0.75rem;background:var(--danger);" onclick="a2aRemoveRemoteAgent(' + idx + ')">✕</button>';
    h += '</div></div>';

    // ID
    h += '<div style="display:flex;gap:0.5rem;margin-bottom:0.4rem;">';
    h += '<input class="field-input" type="text" data-path="a2a.client.remote_agents.' + idx + '.id" value="' + escapeAttr(ra.id || '') + '" placeholder="ID" style="width:120px;">';
    h += '<input class="field-input" type="text" data-path="a2a.client.remote_agents.' + idx + '.name" value="' + escapeAttr(ra.name || '') + '" placeholder="' + escapeAttr(t('config.a2a.ra_name')) + '" style="flex:1;">';
    h += '</div>';

    // Card URL
    h += '<input class="field-input" type="text" data-path="a2a.client.remote_agents.' + idx + '.card_url" value="' + escapeAttr(ra.card_url || '') + '" placeholder="https://agent.example.com/.well-known/agent-card.json" style="width:100%;margin-bottom:0.4rem;">';

    // Auth for this remote agent
    h += '<div style="display:flex;gap:0.5rem;align-items:center;margin-bottom:0.3rem;">';
    h += '<input class="field-input" type="password" id="a2a-ra-apikey-' + idx + '" placeholder="' + escapeAttr(t('config.a2a.ra_api_key')) + '" style="flex:1;">';
    h += '<button class="btn-save" style="padding:0.35rem 0.7rem;font-size:0.78rem;" onclick="a2aSaveVault(\'a2a_remote_' + escapeAttr(ra.id || 'new') + '_api_key\', \'a2a-ra-apikey-' + idx + '\')">💾</button>';
    h += '</div>';
    h += '<div style="display:flex;gap:0.5rem;align-items:center;">';
    h += '<input class="field-input" type="password" id="a2a-ra-bearer-' + idx + '" placeholder="' + escapeAttr(t('config.a2a.ra_bearer')) + '" style="flex:1;">';
    h += '<button class="btn-save" style="padding:0.35rem 0.7rem;font-size:0.78rem;" onclick="a2aSaveVault(\'a2a_remote_' + escapeAttr(ra.id || 'new') + '_bearer_token\', \'a2a-ra-bearer-' + idx + '\')">💾</button>';
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
        var placeholder = container.querySelector('[style*="text-tertiary"]');
        if (placeholder) placeholder.remove();
        container.insertAdjacentHTML('beforeend', a2aRemoteAgentCard(agents[agents.length - 1], agents.length - 1));
    }
    markDirty();
}

function a2aRemoveRemoteAgent(idx) {
    if (!configData.a2a || !configData.a2a.client || !configData.a2a.client.remote_agents) return;
    configData.a2a.client.remote_agents.splice(idx, 1);
    markDirty();
    // Re-render
    var container = document.getElementById('a2a-remote-agents');
    if (container) {
        var html = '';
        var agents = configData.a2a.client.remote_agents;
        if (agents.length === 0) {
            html = '<div style="font-size:0.82rem;color:var(--text-tertiary);padding:0.5rem 0;">' + t('config.a2a.no_remote_agents') + '</div>';
        }
        agents.forEach(function(ra, i) { html += a2aRemoteAgentCard(ra, i); });
        container.innerHTML = html;
    }
}

// ── Vault save helper ──
function a2aSaveVault(vaultKey, inputId) {
    var input = document.getElementById(inputId);
    var statusEl = document.getElementById(inputId + '-status');
    var value = input ? input.value.trim() : '';
    if (!value) {
        if (statusEl) { statusEl.style.color = 'var(--danger)'; statusEl.textContent = t('config.a2a.value_empty'); }
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
            if (statusEl) { statusEl.style.color = 'var(--success)'; statusEl.textContent = '✓ ' + t('config.a2a.saved'); }
        } else {
            if (statusEl) { statusEl.style.color = 'var(--danger)'; statusEl.textContent = '✗ ' + (res.message || t('config.a2a.save_failed')); }
        }
        setTimeout(function() { if (statusEl) statusEl.textContent = ''; }, 4000);
    })
    .catch(function() {
        if (statusEl) { statusEl.style.color = 'var(--danger)'; statusEl.textContent = '✗ ' + t('config.a2a.save_failed'); }
    });
}

// ── Status check ──
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
            banner.style.color = 'var(--success)';
            banner.innerHTML = parts.join('<br>');
        })
        .catch(function() {
            banner.textContent = '⚪ ' + t('config.a2a.status_unavailable');
        });
}

// ── Test server ──
function a2aTestServer() {
    var btn = document.getElementById('a2a-test-btn');
    var result = document.getElementById('a2a-test-result');
    if (btn) btn.disabled = true;
    if (result) { result.style.color = 'var(--text-secondary)'; result.textContent = t('config.a2a.testing'); }

    fetch('/api/a2a/test', { method: 'POST' })
        .then(function(r) { return r.json(); })
        .then(function(res) {
            if (result) {
                if (res.server && res.server.running) {
                    result.style.color = 'var(--success)';
                    result.textContent = '✓ ' + t('config.a2a.test_ok');
                } else {
                    result.style.color = 'var(--danger)';
                    result.textContent = '✗ ' + t('config.a2a.test_fail');
                }
            }
        })
        .catch(function() {
            if (result) { result.style.color = 'var(--danger)'; result.textContent = '✗ ' + t('config.a2a.test_fail'); }
        })
        .finally(function() { if (btn) btn.disabled = false; });
}

// ── Test remote agent ──
function a2aTestRemoteAgent(agentId, idx) {
    var statusEl = document.getElementById('a2a-ra-status-' + idx);
    if (statusEl) { statusEl.style.color = 'var(--text-secondary)'; statusEl.textContent = '⏳'; }

    fetch('/api/a2a/remote-agents/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agent_id: agentId })
    })
    .then(function(r) { return r.json(); })
    .then(function(res) {
        if (statusEl) {
            if (res.available) {
                statusEl.style.color = 'var(--success)';
                statusEl.textContent = '🟢 ' + t('config.a2a.ra_connected');
            } else {
                statusEl.style.color = 'var(--danger)';
                statusEl.textContent = '🔴 ' + (res.error || t('config.a2a.ra_unavailable'));
            }
        }
    })
    .catch(function() {
        if (statusEl) { statusEl.style.color = 'var(--danger)'; statusEl.textContent = '🔴 ' + t('config.a2a.ra_unavailable'); }
    });
}
