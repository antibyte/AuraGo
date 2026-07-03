/* AuraGo – Skills Manager page JS */
/* global I18N, t, applyI18n, esc */
'use strict';

let allSkills = [];
let allAgentSkills = [];
let allTemplates = [];
let daemonStates = {};  // skill_id -> daemon state object
let daemonSystemEnabled = false;  // true when daemon_skills.enabled in config
let currentTypeFilter = 'all';
let currentSecFilter = 'all';
let currentSkillMode = 'python';
let currentDetailId = '';
let currentAgentSkillId = '';
let deleteTargetId = '';
let skillDeleteInFlight = false;
let selectedFile = null;
let selectedImportFile = null;
let selectedAgentImportFile = null;
let codeEditorView = null;
let codeEditorSkillId = '';
let codeEditorDraft = null;
let vaultKeyTargetId = '';
let allVaultSecrets = [];
let credentialMap = {}; // id -> name
let detailVersions = [];
let detailAudit = [];
let searchDebounceHandle = null;
let codeMirrorModulePromise = null;
let agentResourcePathDialogResolve = null;
let agentFileDeleteInFlight = false;
let agentFileDeleteDialogResolve = null;
let agentFileDeleteBusy = false;

// ── Initialization ──────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', async () => {
    await loadCredentialMap();
    loadSkills();
    loadAgentSkills();
    loadTemplates();
    initDropzone();
    applyPlaceholders();
    initDaemonSSE();
});

function applyPlaceholders() {
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        const translated = t(key);
        if (translated && translated !== key) {
            el.placeholder = translated;
        }
    });
}

async function loadCredentialMap() {
    try {
        const resp = await fetch('/api/credentials');
        if (!resp.ok) return;
        const data = await resp.json();
        const list = Array.isArray(data) ? data : (data.items || data.credentials || []);
        credentialMap = {};
        list.forEach(c => { credentialMap[String(c.id)] = c.name || c.id; });
    } catch (_) { }
}

// ── Data fetching ───────────────────────────────────────────────────────────

async function loadSkills() {
    try {
        const resp = await fetch('/api/skills');
        if (resp.status === 503) {
            showDisabledState();
            return;
        }
        const data = await resp.json();
        if (data.status !== 'ok') {
            showDisabledState();
            return;
        }
        allSkills = data.skills || [];
        updateStats(data.stats);
        // Track whether the daemon system is enabled so we can hide daemon
        // action buttons when the supervisor is not running.
        daemonSystemEnabled = !!data.daemon_system_enabled;
        if (data.daemon_states && typeof data.daemon_states === 'object') {
            daemonStates = data.daemon_states;
        }
        renderSkills();
    } catch (e) {
        console.error('Failed to load skills:', e);
    }
}

async function loadAgentSkills() {
    try {
        const resp = await fetch('/api/agent-skills');
        if (resp.status === 503) {
            allAgentSkills = [];
            if (currentSkillMode === 'agent') showDisabledState();
            return;
        }
        const data = await resp.json();
        if (data.status !== 'ok') {
            allAgentSkills = [];
            if (currentSkillMode === 'agent') showDisabledState();
            return;
        }
        allAgentSkills = data.skills || [];
        if (currentSkillMode === 'agent') {
            updateAgentSkillStats();
            renderAgentSkills();
        }
    } catch (e) {
        console.error('Failed to load Agent Skills:', e);
    }
}

async function loadTemplates() {
    try {
        const resp = await fetch('/api/skills/templates');
        if (!resp.ok) return;
        const data = await resp.json();
        if (data.status === 'ok') {
            allTemplates = data.templates || [];
            populateTemplateSelect();
            populateGenerateTemplateSelect();
        }
    } catch (e) {
        console.error('Failed to load templates:', e);
    }
}

function showDisabledState() {
    document.getElementById('sk-grid').style.display = 'none';
    document.getElementById('sk-empty').style.display = 'none';
    document.getElementById('sk-disabled').style.display = '';
    document.getElementById('sk-status-bar').style.display = 'none';
    document.getElementById('sk-toolbar-actions').style.display = 'none';
    document.getElementById('agent-toolbar-actions').style.display = 'none';
    document.getElementById('sk-security-filters').style.display = 'none';
}

// ── Daemon Data ─────────────────────────────────────────────────────────────

    function initDaemonSSE() {
        if (window.AuraSSE) {
            // Reload full skills list on daemon state changes so badges reflect
            // the new status without a separate /api/daemons round-trip.
            window.AuraSSE.on('daemon_update', () => { loadSkills(); });
        }
    }

    function getDaemonState(skillId) {
        return daemonStates[skillId] || null;
    }

    function renderDaemonBadge(skill) {
        const isDaemon = skill.IsDaemon || skill.is_daemon;
        if (!isDaemon) return '';
        const id = skill.name || skill.Name || skill.id || skill.ID || '';
        const ds = getDaemonState(id);
        if (!ds) {
            return `<span class="sk-daemon-badge sk-daemon-stopped" title="${t('skills.daemon_stopped')}">⏹ ${t('skills.daemon')}</span>`;
        }
        const status = (ds.status || ds.Status || 'stopped').toLowerCase();
        const statusMap = {
            running: { cls: 'sk-daemon-running', icon: '🟢', label: t('skills.daemon_running') },
            stopped: { cls: 'sk-daemon-stopped', icon: '⏹', label: t('skills.daemon_stopped') },
            error: { cls: 'sk-daemon-error', icon: '🔴', label: t('skills.daemon_error') },
            disabled: { cls: 'sk-daemon-disabled', icon: '⛔', label: t('skills.daemon_disabled') },
            starting: { cls: 'sk-daemon-starting', icon: '🟡', label: t('skills.daemon_starting') },
        };
        const info = statusMap[status] || statusMap.stopped;
        return `<span class="sk-daemon-badge ${info.cls}" title="${info.label}">${info.icon} ${t('skills.daemon')}</span>`;
    }

    function renderDaemonActions(skill) {
        const isDaemon = skill.IsDaemon || skill.is_daemon;
        if (!isDaemon) return '';
        if (!daemonSystemEnabled) {
            return `<div class="sk-daemon-actions"><span class="sk-daemon-badge sk-daemon-disabled" title="${t('skills.daemon_disabled_hint')}">⛔ ${t('skills.daemon')}</span></div>`;
        }
        const id = skill.name || skill.Name || skill.id || skill.ID || '';
        const ds = getDaemonState(id);
        const status = ds ? (ds.status || ds.Status || 'stopped').toLowerCase() : 'stopped';
        const autoDisabled = ds && (ds.auto_disabled || ds.AutoDisabled);

        let btns = '';
        if (status === 'running') {
            btns = `<button class="btn btn-sm btn-warning" onclick="daemonAction('${id}','stop')">${t('skills.daemon_stop')}</button>`;
        } else if (autoDisabled || status === 'disabled') {
            btns = `<button class="btn btn-sm btn-primary" onclick="daemonAction('${id}','reenable')">${t('skills.daemon_reenable')}</button>`;
        } else {
            btns = `<button class="btn btn-sm btn-primary" onclick="daemonAction('${id}','start')">${t('skills.daemon_start')}</button>`;
        }
        return `<div class="sk-daemon-actions">${btns}</div>`;
    }

    // eslint-disable-next-line no-unused-vars
    async function daemonAction(skillId, action) {
        try {
            const resp = await fetch(`/api/daemons/${encodeURIComponent(skillId)}/${action}`, { method: 'POST' });
            let data;
            try {
                data = await resp.json();
            } catch (parseErr) {
                console.error('Daemon action failed: non-JSON response', resp.status, parseErr);
                showToast(`${t('skills.daemon_action_failed')}: HTTP ${resp.status}`, 'error');
                return;
            }
            if (data.status === 'ok' && resp.ok) {
                showToast(t('skills.daemon_action_ok'), 'success');
                await loadSkills();
            } else {
                const msg = data.message || data.error || 'Unknown error';
                console.error('Daemon action error:', msg);
                showToast(`${t('skills.daemon_action_failed')}: ${msg}`, 'error');
            }
        } catch (e) {
            console.error('Daemon action network error:', e);
            showToast(`${t('skills.daemon_action_failed')}: ${e.message || 'Network error'}`, 'error');
        }
    }

    let daemonSettingsCache = { missions: [], cheatsheets: [] };

    async function loadDaemonSettingsOptions() {
        try {
            const [mResp, csResp] = await Promise.all([
                fetch('/api/missions/v2'),
                fetch('/api/cheatsheets')
            ]);
            if (mResp.ok) {
                const mData = await mResp.json();
                daemonSettingsCache.missions = (mData.missions || []).filter(m => m.enabled);
            }
            if (csResp.ok) {
                const csData = await csResp.json();
                const csList = Array.isArray(csData) ? csData : (csData.cheatsheets || csData.items || []);
                daemonSettingsCache.cheatsheets = csList.filter(c => c.active !== false);
            }
        } catch (_) { }
    }

    function renderDaemonSettings(skill, daemon) {
        const isDaemon = skill.IsDaemon || skill.is_daemon;
        if (!isDaemon) return '';
        daemon = daemon || {};
        const wakeAgent = daemon.wake_agent !== false;
        const missionId = daemon.trigger_mission_id || '';
        const missionName = daemon.trigger_mission_name || '';
        const csId = daemon.cheatsheet_id || '';
        const csName = daemon.cheatsheet_name || '';

        const missionOpts = daemonSettingsCache.missions.map(m =>
            `<option value="${esc(m.id)}" ${m.id === missionId ? 'selected' : ''}>${esc(m.name)}</option>`
        ).join('');
        const csOpts = daemonSettingsCache.cheatsheets.map(c =>
            `<option value="${esc(c.id)}" ${c.id === csId ? 'selected' : ''}>${esc(c.name)}</option>`
        ).join('');

        return `
        <div class="sk-daemon-settings-section">
            <h4 class="sk-daemon-settings-title">👹 ${t('skills.daemon_settings_title')}</h4>
            <div class="sk-detail-grid">
                <div class="sk-detail-row">
                    <span class="sk-detail-label">${t('skills.daemon_wake_agent')}:</span>
                    <label class="toggle-wrap compact">
                        <div class="toggle${wakeAgent ? ' on' : ''}" id="daemon-wake-toggle" onclick="this.classList.toggle('on')"></div>
                        <span class="toggle-label">${wakeAgent ? (t('config.toggle.active')) : (t('config.toggle.inactive'))}</span>
                    </label>
                </div>
                <div class="sk-detail-row">
                    <span class="sk-detail-label">${t('skills.daemon_trigger_mission')}:</span>
                    <select id="daemon-mission-select" class="field-input sk-daemon-select">
                        <option value="">— ${t('skills.daemon_no_mission')} —</option>
                        ${missionOpts}
                    </select>
                </div>
                <div class="sk-detail-row">
                    <span class="sk-detail-label">${t('skills.daemon_cheatsheet')}:</span>
                    <select id="daemon-cheatsheet-select" class="field-input sk-daemon-select">
                        <option value="">— ${t('skills.daemon_no_cheatsheet')} —</option>
                        ${csOpts}
                    </select>
                </div>
            </div>
            <div class="sk-daemon-settings-help">${t('skills.daemon_settings_help')}</div>
            <div class="sk-daemon-settings-actions">
                <button class="btn btn-sm btn-primary" onclick="saveDaemonSettings('${esc(skill.ID || skill.id || '')}')">${t('common.btn_save')}</button>
            </div>
        </div>`;
    }

    // eslint-disable-next-line no-unused-vars
    async function saveDaemonSettings(skillId) {
        const wakeToggle = document.getElementById('daemon-wake-toggle');
        const wakeAgent = wakeToggle && wakeToggle.classList.contains('on');
        const missionId = (document.getElementById('daemon-mission-select') || {}).value || '';
        const cheatsheetId = (document.getElementById('daemon-cheatsheet-select') || {}).value || '';

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(skillId)}/daemon`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    wake_agent: wakeAgent,
                    trigger_mission_id: missionId,
                    cheatsheet_id: cheatsheetId
                })
            });
            const data = await resp.json();
            if (data.status === 'ok') {
                showToast(t('skills.daemon_settings_saved'), 'success');
                showDetail(currentDetailId);
                loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // ── Stats ───────────────────────────────────────────────────────────────────

    function updateStats(stats) {
        if (!stats) return;
        document.getElementById('sk-total').textContent = stats.total || 0;
        document.getElementById('sk-agent').textContent = stats.agent || 0;
        document.getElementById('sk-user').textContent = stats.user || 0;
        document.getElementById('sk-pending').textContent = stats.pending || 0;
    }

    function updateAgentSkillStats() {
        const total = allAgentSkills.length;
        const clean = allAgentSkills.filter(s => (s.security_status || '').toLowerCase() === 'clean').length;
        const warning = allAgentSkills.filter(s => (s.security_status || '').toLowerCase() === 'warning').length;
        const pending = allAgentSkills.filter(s => ['pending', 'error'].includes((s.security_status || '').toLowerCase())).length;
        document.getElementById('sk-total').textContent = total;
        document.getElementById('sk-agent').textContent = clean;
        document.getElementById('sk-user').textContent = warning;
        document.getElementById('sk-pending').textContent = pending;
    }

    // ── Rendering ───────────────────────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    function switchSkillMode(mode) {
        currentSkillMode = mode === 'agent' ? 'agent' : 'python';
        document.getElementById('sk-tab-python').classList.toggle('active', currentSkillMode === 'python');
        document.getElementById('sk-tab-agent').classList.toggle('active', currentSkillMode === 'agent');
        document.getElementById('sk-tab-python').setAttribute('aria-selected', currentSkillMode === 'python' ? 'true' : 'false');
        document.getElementById('sk-tab-agent').setAttribute('aria-selected', currentSkillMode === 'agent' ? 'true' : 'false');
        document.getElementById('sk-toolbar-actions').style.display = currentSkillMode === 'python' ? '' : 'none';
        document.getElementById('agent-toolbar-actions').style.display = currentSkillMode === 'agent' ? '' : 'none';
        const typeFilter = document.querySelector('.sk-filter-group');
        if (typeFilter) typeFilter.style.display = currentSkillMode === 'python' ? '' : 'none';
        document.getElementById('sk-disabled').style.display = 'none';
        document.getElementById('sk-status-bar').style.display = '';
        document.getElementById('sk-security-filters').style.display = '';
        if (currentSkillMode === 'agent') {
            updateAgentSkillStats();
            renderAgentSkills();
        } else {
            loadSkills();
        }
    }

    function renderSkills() {
        if (currentSkillMode === 'agent') {
            renderAgentSkills();
            return;
        }
        const grid = document.getElementById('sk-grid');
        const empty = document.getElementById('sk-empty');
        const disabled = document.getElementById('sk-disabled');
        disabled.style.display = 'none';

        const filtered = getFilteredSkills();

        if (filtered.length === 0) {
            grid.style.display = 'none';
            empty.style.display = '';
            return;
        }
        empty.style.display = 'none';
        grid.style.display = '';

        grid.innerHTML = filtered.map(s => renderCard(s)).join('');
        if (typeof applyI18n === 'function') applyI18n();
    }

    function renderCard(skill) {
        const name = esc(skill.Name || skill.name || 'Unknown');
        const desc = esc(skill.Description || skill.description || '');
        const type = (skill.Type || skill.type || 'user').toLowerCase();
        const secStatus = (skill.SecurityStatus || skill.security_status || 'pending').toLowerCase();
        const enabled = skill.Enabled !== undefined ? skill.Enabled : skill.enabled;
        const id = esc(skill.ID || skill.id || '');
        const deps = skill.Dependencies || skill.dependencies || [];
        const tags = skill.Tags || skill.tags || [];
        const category = skill.Category || skill.category || '';
        const vaultKeys = skill.VaultKeys || skill.vault_keys || [];
        const internalTools = skill.InternalTools || skill.internal_tools || [];

        const typeLabel = type.charAt(0).toUpperCase() + type.slice(1);
        const secBadge = renderSecurityBadge(secStatus);
        const enabledClass = enabled ? 'sk-enabled' : 'sk-disabled-card';
        const toggleLabel = enabled ? (t('skills.btn_disable')) : (t('skills.btn_enable'));
        const toggleClass = enabled ? 'btn-secondary' : 'btn-primary';

        let depTags = '';
        if (deps.length > 0) {
            depTags = `<div class="sk-card-deps">${deps.slice(0, 5).map(d => `<span class="sk-dep-tag">${esc(d)}</span>`).join('')}${deps.length > 5 ? `<span class="sk-dep-tag">+${deps.length - 5}</span>` : ''}</div>`;
        }
        const metaTags = [];
        if (category) metaTags.push(`<span class="sk-dep-tag">${esc(category)}</span>`);
        tags.slice(0, 4).forEach(tag => metaTags.push(`<span class="sk-dep-tag">${esc(tag)}</span>`));
        const metaRow = metaTags.length > 0 ? `<div class="sk-card-deps">${metaTags.join('')}</div>` : '';

        let vaultRow = '';
        {
            const keyTags = vaultKeys.length > 0
                ? vaultKeys.map(k => {
                    if (k.startsWith('cred:')) {
                        const cname = credentialMap[k.slice(5)] || k.slice(5);
                        return `<code class="sk-vault-key-tag sk-vault-key-cred" title="${esc(k)}">🔑 ${esc(cname)}</code>`;
                    }
                    return `<code class="sk-vault-key-tag" title="${esc(k)}">${esc(k)}</code>`;
                }).join('')
                : '';
            vaultRow = `<div class="sk-card-vault">
            <button class="btn btn-xs btn-secondary sk-vault-edit-btn" onclick="openVaultKeyModal('${id}')" title="${t('skills.btn_edit_secrets')}">🔑 ${t('skills.btn_edit_secrets')}</button>
            <button class="btn btn-xs btn-secondary sk-vault-edit-btn" onclick="openInternalToolsModal('${id}')" title="${t('skills.btn_edit_internal_tools')}">⚙️ ${t('skills.btn_edit_internal_tools')}</button>
            ${keyTags ? `<span class="sk-vault-keys">${keyTags}</span>` : ''}
        </div>`;
        }

        const internalToolsRow = internalTools.length > 0
            ? `<div class="sk-card-internal-tools"><span class="sk-internal-tools-label">⚙️ ${t('skills.internal_tools_label')}:</span> ${internalTools.map(tool => `<code class="sk-dep-tag sk-internal-tool-tag" title="${esc(tool)}">${esc(tool)}</code>`).join('')}</div>`
            : '';

        return `
    <div class="sk-card ${enabledClass}" data-id="${id}" data-type="${type}" data-sec="${secStatus}">
        <div class="sk-card-header">
            <div class="sk-card-name" title="${name}">${name}</div>
            <div class="sk-card-badges">
                <span class="sk-type-badge sk-type-${type}">${typeLabel}</span>
                ${secBadge}
                ${renderDaemonBadge(skill)}
            </div>
        </div>
        <div class="sk-card-desc">${desc || '<em>' + (t('skills.no_description')) + '</em>'}</div>
        ${metaRow}
        ${depTags}
        ${vaultRow}
        ${internalToolsRow}
        ${renderDaemonActions(skill)}
        <div class="sk-card-actions">
            <button class="btn btn-sm btn-secondary" onclick="showDetail('${id}')" data-i18n="skills.btn_details">Details</button>
            <button class="btn btn-sm btn-secondary" onclick="viewCode('${id}')" data-i18n="skills.btn_view_code">Code</button>
            <button class="btn btn-sm ${toggleClass}" onclick="toggleSkill('${id}', ${!enabled})">${toggleLabel}</button>
            <button class="btn btn-sm btn-danger" onclick="showDeleteSkillModal('${id}', '${name}')" data-i18n="skills.btn_delete">Delete</button>
        </div>
    </div>`;
    }

    function renderAgentSkills() {
        const grid = document.getElementById('sk-grid');
        const empty = document.getElementById('sk-empty');
        const disabled = document.getElementById('sk-disabled');
        disabled.style.display = 'none';
        const filtered = getFilteredAgentSkills();
        if (filtered.length === 0) {
            grid.style.display = 'none';
            empty.style.display = '';
            return;
        }
        empty.style.display = 'none';
        grid.style.display = '';
        grid.innerHTML = filtered.map(s => renderAgentSkillCard(s)).join('');
        if (typeof applyI18n === 'function') applyI18n();
    }

    function renderAgentSkillCard(skill) {
        const id = esc(skill.id || '');
        const name = esc(skill.name || 'Unknown');
        const desc = esc(skill.description || '');
        const secStatus = (skill.security_status || 'pending').toLowerCase();
        const enabled = !!skill.enabled;
        const warningApproved = !!skill.warning_approved;
        const scripts = skill.scripts || [];
        const canEnable = secStatus === 'clean' || (secStatus === 'warning' && warningApproved);
        const toggleDisabled = (!enabled && !canEnable) ? 'disabled' : '';
        const toggleTitle = !enabled && !canEnable ? t('skills.agent_enable_blocked') : '';
        const toggleLabel = enabled ? t('skills.btn_disable') : t('skills.btn_enable');
        const scriptTags = scripts.length > 0
            ? `<div class="sk-card-deps">${scripts.map(s => `<span class="sk-dep-tag">${esc(s.path || s.Path || '')}</span>`).join('')}</div>`
            : '';
        const approval = secStatus === 'warning' && !warningApproved
            ? `<button class="btn btn-sm btn-warning" onclick="approveAgentSkillWarning('${id}')" data-i18n="skills.agent_btn_approve">${t('skills.agent_btn_approve')}</button>`
            : '';
        return `
    <div class="sk-card ${enabled ? 'sk-enabled' : 'sk-disabled-card'}" data-id="${id}" data-sec="${secStatus}">
        <div class="sk-card-header">
            <div class="sk-card-name" title="${name}">${name}</div>
            <div class="sk-card-badges">
                <span class="sk-type-badge sk-type-agent" data-i18n="skills.tab_agent">Agent Skills</span>
                ${renderSecurityBadge(secStatus)}
            </div>
        </div>
        <div class="sk-card-desc">${desc || '<em>' + t('skills.no_description') + '</em>'}</div>
        ${scriptTags}
        <div class="sk-card-actions">
            <button class="btn btn-sm btn-secondary" onclick="showAgentSkillDetail('${id}')" data-i18n="skills.btn_details">${t('skills.btn_details')}</button>
            <button class="btn btn-sm btn-secondary" onclick="editAgentSkill('${id}')" data-i18n="skills.agent_btn_edit">${t('skills.agent_btn_edit')}</button>
            <button class="btn btn-sm btn-secondary" onclick="verifyAgentSkill('${id}')" data-i18n="skills.btn_verify">${t('skills.btn_verify')}</button>
            ${approval}
            <button class="btn btn-sm ${enabled ? 'btn-secondary' : 'btn-primary'}" title="${esc(toggleTitle)}" onclick="toggleAgentSkill('${id}', ${!enabled})" ${toggleDisabled}>${toggleLabel}</button>
            <button class="btn btn-sm btn-secondary" onclick="showAgentSkillTestModal('${id}')" data-i18n="skills.btn_test">${t('skills.btn_test')}</button>
            <button class="btn btn-sm btn-danger" onclick="deleteAgentSkill('${id}', '${name}')" data-i18n="skills.btn_delete">${t('skills.btn_delete')}</button>
        </div>
    </div>`;
    }

    function renderSecurityBadge(status) {
        const labels = {
            clean: t('skills.sec_clean'),
            warning: t('skills.sec_warning'),
            dangerous: t('skills.sec_dangerous'),
            pending: t('skills.sec_pending'),
            error: t('skills.sec_error')
        };
        const label = labels[status] || status;
        return `<span class="sk-sec-badge sk-sec-${status}">${label}</span>`;
    }

    // ── Filtering ───────────────────────────────────────────────────────────────

    function getFilteredSkills() {
        const search = (document.getElementById('sk-search').value || '').toLowerCase();
        return allSkills.filter(s => {
            const type = (s.Type || s.type || '').toLowerCase();
            const sec = (s.SecurityStatus || s.security_status || 'pending').toLowerCase();
            const name = (s.Name || s.name || '').toLowerCase();
            const desc = (s.Description || s.description || '').toLowerCase();
            const isDaemon = s.IsDaemon || s.is_daemon;

            if (currentTypeFilter === 'daemon' && !isDaemon) return false;
            if (currentTypeFilter !== 'all' && currentTypeFilter !== 'daemon' && type !== currentTypeFilter) return false;
            if (currentSecFilter !== 'all' && sec !== currentSecFilter) return false;
            if (search && !name.includes(search) && !desc.includes(search)) return false;
            return true;
        });
    }

    function getFilteredAgentSkills() {
        const search = (document.getElementById('sk-search').value || '').toLowerCase();
        return allAgentSkills.filter(s => {
            const sec = (s.security_status || 'pending').toLowerCase();
            const name = (s.name || '').toLowerCase();
            const desc = (s.description || '').toLowerCase();
            if (currentSecFilter !== 'all' && sec !== currentSecFilter) return false;
            if (search && !name.includes(search) && !desc.includes(search)) return false;
            return true;
        });
    }

    // eslint-disable-next-line no-unused-vars
    function filterSkills() {
        window.clearTimeout(searchDebounceHandle);
        searchDebounceHandle = window.setTimeout(() => currentSkillMode === 'agent' ? renderAgentSkills() : renderSkills(), 120);
    }

    // eslint-disable-next-line no-unused-vars
    function setSkillFilter(filter) {
        currentTypeFilter = filter;
        document.querySelectorAll('.sk-filter-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.filter === filter);
        });
        if (currentSkillMode === 'agent') renderAgentSkills(); else renderSkills();
    }

    // eslint-disable-next-line no-unused-vars
    function setSecurityFilter(filter) {
        currentSecFilter = filter;
        document.querySelectorAll('.sk-pill').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.sec === filter);
        });
        if (currentSkillMode === 'agent') renderAgentSkills(); else renderSkills();
    }

    // ── Skill Actions ───────────────────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    async function toggleAgentSkill(id, enabled) {
        try {
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(id)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled })
            });
            const data = await resp.json();
            if (data.status === 'ok') {
                showToast(t('skills.toggle_success'), 'success');
                await loadAgentSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function verifyAgentSkill(id) {
        try {
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(id)}/verify`, { method: 'POST' });
            const data = await resp.json();
            if (data.status === 'scanned') {
                showToast(t('skills.scan_complete'), 'success');
                await loadAgentSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function approveAgentSkillWarning(id) {
        try {
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(id)}/approve-warning`, { method: 'POST' });
            const data = await resp.json();
            if (data.status === 'approved') {
                showToast(t('skills.agent_warning_approved'), 'success');
                await loadAgentSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function showAgentSkillDetail(id) {
        try {
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(id)}?content=true`);
            const data = await resp.json();
            if (data.status !== 'ok') {
                showToast(data.message || t('common.error'), 'error');
                return;
            }
            const s = data.skill;
            currentAgentSkillId = id;
            currentDetailId = '';
            const scripts = s.scripts || [];
            const resources = s.resources || [];
            document.getElementById('detail-modal-title').textContent = s.name || t('skills.tab_agent');
            const actions = document.querySelector('#detail-modal .modal-actions');
            if (actions) actions.style.display = 'none';
            document.getElementById('detail-modal-body').innerHTML = `
                <div class="sk-detail-grid">
                    <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.agent_field_name')}:</span><code>${esc(s.name || '')}</code></div>
                    <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.upload_description')}:</span><span>${esc(s.description || '')}</span></div>
                    <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_security')}:</span>${renderSecurityBadge((s.security_status || 'pending').toLowerCase())}</div>
                    <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.agent_warning_approved_label')}:</span><span>${s.warning_approved ? t('common.yes') : t('common.no')}</span></div>
                </div>
                <h4>${t('skills.agent_skill_md')}</h4>
                <pre class="sk-code-preview">${esc(data.content || '')}</pre>
                <h4>${t('skills.agent_resources')}</h4>
                <div class="sk-card-deps">${resources.map(r => {
                    const p = r.path || r.Path || '';
                    const kind = r.kind || r.Kind || guessResourceKind(p);
                    const icon = resourceKindIcon(kind);
                    const ext = p.split('.').pop().toLowerCase();
                    const binaryExts = ['png','jpg','jpeg','gif','svg','pdf','zip','tar','gz','bin','exe','dll','so','dylib'];
                    if (binaryExts.includes(ext)) {
                        return `<span class="sk-dep-tag sk-dep-tag-clickable" onclick="downloadDetailResource('${esc(id)}','${esc(p)}')" title="${t('skills.agent_btn_download_file')}">${icon} ${esc(p)}</span>`;
                    }
                    return `<span class="sk-dep-tag sk-dep-tag-clickable" onclick="previewDetailResource('${esc(id)}','${esc(p)}')" title="Preview">${icon} ${esc(p)}</span>`;
                }).join('') || '<em>' + t('skills.agent_no_resources') + '</em>'}</div>
                <h4>${t('skills.agent_scripts')}</h4>
                <div class="sk-card-deps">${scripts.map(r => `<span class="sk-dep-tag">${esc(r.path || r.Path || '')}</span>`).join('') || '<em>' + t('skills.agent_no_scripts') + '</em>'}</div>
            `;
            document.getElementById('detail-modal').classList.add('active');
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function editAgentSkill(id) {
        currentAgentSkillId = id;
        const resp = await fetch(`/api/agent-skills/${encodeURIComponent(id)}?content=true`);
        const data = await resp.json();
        if (data.status !== 'ok') {
            showToast(data.message || t('common.error'), 'error');
            return;
        }
        document.getElementById('agent-skill-modal-title').textContent = data.skill.name || t('skills.agent_edit_title');
        document.getElementById('agent-create-fields').style.display = 'none';
        document.getElementById('agent-skill-content').value = data.content || '';
        document.getElementById('agent-resource-browser').style.display = '';
        renderAgentResourceList(data.skill.resources || []);
        document.getElementById('agent-skill-modal').classList.add('active');
    }

    function renderAgentResourceList(resources) {
        const list = document.getElementById('agent-resource-list');
        if (!list) return;
        if (!resources || resources.length === 0) {
            list.innerHTML = `<div class="sk-resource-empty">
                <p>${esc(t('skills.agent_no_resources'))}</p>
                <small>${esc(t('skills.agent_resource_empty_hint'))}</small>
                <button class="btn btn-sm btn-secondary" type="button" onclick="newAgentSkillFile()">${esc(t('skills.agent_btn_new_file'))}</button>
            </div>`;
            return;
        }
        const currentPath = (document.getElementById('agent-resource-path')?.value || '').trim();
        const sorted = [...resources].sort((a, b) => (a.path || '').localeCompare(b.path || ''));
        list.innerHTML = sorted.map(r => {
            const p = r.path || r.Path || '';
            const kind = r.kind || r.Kind || guessResourceKind(p);
            const icon = resourceKindIcon(kind);
            const ext = p.split('.').pop().toLowerCase();
            const isBinary = ['png','jpg','jpeg','gif','svg','pdf','zip','tar','gz','bin','exe','dll','so','dylib'].includes(ext);
            const chipClass = ['sk-resource-chip', isBinary ? 'sk-resource-binary' : '', p === currentPath ? 'is-selected' : ''].filter(Boolean).join(' ');
            return `<button type="button" class="${chipClass}" data-resource-path="${esc(p)}" onclick="loadAgentSkillResource(this.dataset.resourcePath)">
                <span class="sk-resource-icon">${icon}</span>
                <span class="sk-resource-name">${esc(p)}</span>
            </button>`;
        }).join('');
    }

    function guessResourceKind(path) {
        if (path.startsWith('scripts/')) return 'script';
        if (path.startsWith('references/')) return 'reference';
        if (path.startsWith('assets/')) return 'asset';
        if (path.startsWith('agents/')) return 'agent';
        return 'file';
    }

    function resourceKindIcon(kind) {
        switch (kind) {
            case 'script': return '⚙️';
            case 'reference': return '📄';
            case 'asset': return '📦';
            case 'agent': return '🤖';
            default: return '📁';
        }
    }

    function setAgentResourceSelection(path) {
        document.querySelectorAll('#agent-resource-list .sk-resource-chip').forEach(chip => {
            chip.classList.toggle('is-selected', chip.dataset.resourcePath === path);
        });
    }

    function validateAgentResourcePath(path) {
        const value = (path || '').trim();
        const invalidSegment = value.split('/').some(part => part === '..');
        const isAbsolute = value.startsWith('/') || /^[a-zA-Z]:/.test(value);
        const hasBackslash = value.includes('\\');
        if (!value) {
            return { ok: false, value, message: t('skills.agent_resource_path_required') };
        }
        if (invalidSegment || isAbsolute || hasBackslash) {
            return { ok: false, value, message: t('skills.agent_resource_path_invalid') };
        }
        return { ok: true, value, message: '' };
    }

    function setAgentResourcePathError(message) {
        const errorEl = document.getElementById('agent-resource-path-error');
        if (!errorEl) return;
        errorEl.textContent = message || '';
        errorEl.style.display = message ? '' : 'none';
    }

    // eslint-disable-next-line no-unused-vars
    function onAgentResourcePathInput() {
        const input = document.getElementById('agent-resource-path-input');
        const confirmBtn = document.getElementById('agent-resource-path-confirm-btn');
        const validation = validateAgentResourcePath(input ? input.value : '');
        if (confirmBtn) confirmBtn.disabled = !validation.ok;
        setAgentResourcePathError(input && input.value.trim() ? validation.message : '');
    }

    // eslint-disable-next-line no-unused-vars
    function onAgentResourcePathKeydown(event) {
        if (event.key === 'Escape') {
            event.preventDefault();
            cancelAgentResourcePathDialog();
        }
        if (event.key === 'Enter') {
            event.preventDefault();
            confirmAgentResourcePathDialog();
        }
    }

    function closeAgentResourcePathDialog(result) {
        const overlay = document.getElementById('agent-resource-path-modal');
        if (overlay) {
            overlay.classList.remove('active');
            overlay.style.display = 'none';
            overlay.onclick = null;
        }
        const resolve = agentResourcePathDialogResolve;
        agentResourcePathDialogResolve = null;
        if (resolve) resolve(result);
    }

    // eslint-disable-next-line no-unused-vars
    function cancelAgentResourcePathDialog() {
        closeAgentResourcePathDialog(null);
    }

    // eslint-disable-next-line no-unused-vars
    function confirmAgentResourcePathDialog() {
        const input = document.getElementById('agent-resource-path-input');
        const validation = validateAgentResourcePath(input ? input.value : '');
        if (!validation.ok) {
            setAgentResourcePathError(validation.message);
            if (input) input.focus();
            return;
        }
        closeAgentResourcePathDialog(validation.value);
    }

    function showAgentResourcePathDialog(options) {
        const overlay = document.getElementById('agent-resource-path-modal');
        const titleEl = document.getElementById('agent-resource-path-title');
        const descEl = document.getElementById('agent-resource-path-description');
        const input = document.getElementById('agent-resource-path-input');
        const confirmBtn = document.getElementById('agent-resource-path-confirm-btn');
        if (!overlay || !input || !confirmBtn) return Promise.resolve(null);

        if (titleEl) titleEl.textContent = t(options.titleKey);
        if (descEl) descEl.textContent = t(options.descriptionKey || 'skills.agent_resource_path_help');
        input.value = options.value || '';
        confirmBtn.textContent = t(options.confirmKey);
        setAgentResourcePathError('');
        overlay.style.display = 'flex';
        overlay.classList.add('active');
        overlay.onclick = event => {
            if (event.target === overlay) cancelAgentResourcePathDialog();
        };
        window.setTimeout(() => {
            input.focus();
            if (input.value) input.select();
        }, 0);
        onAgentResourcePathInput();
        return new Promise(resolve => {
            agentResourcePathDialogResolve = resolve;
        });
    }

    // eslint-disable-next-line no-unused-vars
    async function loadAgentSkillResource(path) {
        if (!currentAgentSkillId) return;
        document.getElementById('agent-file-editor').style.display = '';
        document.getElementById('agent-resource-path').value = path;
        setAgentResourceSelection(path);
        const ext = path.split('.').pop().toLowerCase();
        const binaryExts = ['png','jpg','jpeg','gif','svg','pdf','zip','tar','gz','bin','exe','dll','so','dylib'];
        if (binaryExts.includes(ext)) {
            document.getElementById('agent-resource-content').style.display = 'none';
            document.getElementById('agent-binary-download').style.display = '';
        } else {
            document.getElementById('agent-resource-content').style.display = '';
            document.getElementById('agent-binary-download').style.display = 'none';
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/files?path=${encodeURIComponent(path)}`);
            const data = await resp.json();
            if (data.status === 'ok') {
                document.getElementById('agent-resource-content').value = data.content || '';
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function saveAgentSkillResource() {
        if (!currentAgentSkillId) return;
        const path = document.getElementById('agent-resource-path').value.trim();
        const content = document.getElementById('agent-resource-content').value;
        if (!path) {
            showToast(t('skills.agent_resource_path_required'), 'error');
            return;
        }
        const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/files?path=${encodeURIComponent(path)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ content })
        });
        const data = await resp.json();
        if (data.status === 'saved') {
            showToast(t('skills.code_saved'), 'success');
            renderAgentResourceList((data.skill && data.skill.resources) || []);
            await loadAgentSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function newAgentSkillFile() {
        if (!currentAgentSkillId) return;
        const path = await showAgentResourcePathDialog({
            titleKey: 'skills.agent_resource_new_title',
            confirmKey: 'skills.agent_resource_confirm_create',
            value: ''
        });
        if (!path) return;
        const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/files`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path, content: '', binary: false })
        });
        const data = await resp.json();
        if (data.status === 'saved') {
            showToast(t('skills.code_saved'), 'success');
            renderAgentResourceList((data.skill && data.skill.resources) || []);
            loadAgentSkillResource(path);
            await loadAgentSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    }

    async function showAgentFileDeleteConfirm(message) {
        const overlay = document.getElementById('agent-file-delete-modal');
        const messageEl = document.getElementById('agent-file-delete-message');
        if (!overlay) return Promise.resolve(false);
        if (messageEl) messageEl.textContent = message;
        setAgentFileDeleteBusy(false);
        overlay.style.display = 'flex';
        overlay.classList.add('active');
        overlay.onclick = event => {
            if (event.target === overlay) cancelAgentFileDeleteDialog();
        };
        return new Promise(resolve => {
            agentFileDeleteDialogResolve = resolve;
        });
    }

    function closeAgentFileDeleteDialog(result) {
        const overlay = document.getElementById('agent-file-delete-modal');
        if (overlay) {
            overlay.classList.remove('active');
            overlay.style.display = 'none';
            overlay.onclick = null;
        }
        setAgentFileDeleteBusy(false);
        const resolve = agentFileDeleteDialogResolve;
        agentFileDeleteDialogResolve = null;
        if (resolve) resolve(result);
    }

    function setAgentFileDeleteBusy(busy) {
        agentFileDeleteBusy = busy;
        const confirmBtn = document.getElementById('agent-file-delete-confirm-btn');
        const cancelBtn = document.getElementById('agent-file-delete-cancel-btn');
        if (confirmBtn) {
            confirmBtn.disabled = busy;
        }
        if (cancelBtn) {
            cancelBtn.disabled = busy;
        }
    }

    // eslint-disable-next-line no-unused-vars
    function confirmAgentFileDeleteDialog() {
        if (agentFileDeleteBusy) return;
        setAgentFileDeleteBusy(true);
        const resolve = agentFileDeleteDialogResolve;
        agentFileDeleteDialogResolve = null;
        if (resolve) resolve(true);
    }

    // eslint-disable-next-line no-unused-vars
    function cancelAgentFileDeleteDialog() {
        if (agentFileDeleteBusy) return;
        closeAgentFileDeleteDialog(false);
    }

    // eslint-disable-next-line no-unused-vars
    function onAgentFileDeleteKeydown(event) {
        if (event.key === 'Escape') {
            event.preventDefault();
            cancelAgentFileDeleteDialog();
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function deleteAgentSkillFile() {
        if (!currentAgentSkillId) return;
        if (agentFileDeleteInFlight) return;
        const path = document.getElementById('agent-resource-path').value.trim();
        if (!path) return;
        const msg = t('skills.agent_delete_file_text').replace('{path}', path);
        agentFileDeleteInFlight = true;
        try {
            const confirmed = await showAgentFileDeleteConfirm(msg);
            if (!confirmed) return;
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/files?path=${encodeURIComponent(path)}`, {
                method: 'DELETE'
            });
            const data = await resp.json();
            if (data.status === 'deleted') {
                showToast(t('skills.delete_success'), 'success');
                document.getElementById('agent-file-editor').style.display = 'none';
                renderAgentResourceList((data.skill && data.skill.resources) || []);
                await loadAgentSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } finally {
            closeAgentFileDeleteDialog(null);
            agentFileDeleteInFlight = false;
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function renameAgentSkillFile() {
        if (!currentAgentSkillId) return;
        const oldPath = document.getElementById('agent-resource-path').value.trim();
        if (!oldPath) return;
        const newPath = await showAgentResourcePathDialog({
            titleKey: 'skills.agent_resource_rename_title',
            confirmKey: 'skills.agent_resource_confirm_rename',
            value: oldPath
        });
        if (!newPath || newPath === oldPath) return;
        const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/files/rename`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ from: oldPath, to: newPath })
        });
        const data = await resp.json();
        if (data.status === 'renamed') {
            showToast(t('skills.toggle_success'), 'success');
            renderAgentResourceList((data.skill && data.skill.resources) || []);
            loadAgentSkillResource(newPath);
            await loadAgentSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    function uploadAgentSkillFile() {
        document.getElementById('agent-file-upload-input').click();
    }

    // eslint-disable-next-line no-unused-vars
    async function handleAgentFileUpload(event) {
        if (!currentAgentSkillId) return;
        const file = event.target.files && event.target.files[0];
        if (!file) return;
        const path = await showAgentResourcePathDialog({
            titleKey: 'skills.agent_resource_upload_title',
            confirmKey: 'skills.agent_resource_confirm_upload',
            value: file.name
        });
        if (!path) {
            event.target.value = '';
            return;
        }
        const form = new FormData();
        form.append('file', file);
        form.append('path', path);
        const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/files/upload`, {
            method: 'POST',
            body: form
        });
        const data = await resp.json();
        if (data.status === 'uploaded') {
            showToast(t('skills.upload_success'), 'success');
            renderAgentResourceList((data.skill && data.skill.resources) || []);
            loadAgentSkillResource(path);
            await loadAgentSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
        event.target.value = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function downloadAgentSkillFile() {
        if (!currentAgentSkillId) return;
        const path = document.getElementById('agent-resource-path').value.trim();
        if (!path) return;
        const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/files/raw?path=${encodeURIComponent(path)}`);
        if (!resp.ok) {
            showToast(t('common.error'), 'error');
            return;
        }
        const blob = await resp.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = path.split('/').pop();
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }

    // eslint-disable-next-line no-unused-vars
    function showAgentSkillCreateModal() {
        currentAgentSkillId = '';
        document.getElementById('agent-skill-modal-title').textContent = t('skills.agent_create_title');
        document.getElementById('agent-create-fields').style.display = '';
        document.getElementById('agent-resource-browser').style.display = 'none';
        document.getElementById('agent-skill-name').value = '';
        document.getElementById('agent-skill-description').value = '';
        document.getElementById('agent-skill-content').value = '# Instructions\n\nUse this skill when the task matches the package description.\n';
        document.getElementById('agent-skill-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeAgentSkillModal() {
        document.getElementById('agent-skill-modal').classList.remove('active');
        currentAgentSkillId = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function saveAgentSkill() {
        const content = document.getElementById('agent-skill-content').value;
        try {
            let resp;
            if (currentAgentSkillId) {
                resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ content })
                });
            } else {
                resp = await fetch('/api/agent-skills', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        name: document.getElementById('agent-skill-name').value.trim(),
                        description: document.getElementById('agent-skill-description').value.trim(),
                        body: content
                    })
                });
            }
            const data = await resp.json();
            if (data.status === 'ok' || data.status === 'created') {
                showToast(t('skills.create_success'), 'success');
                closeAgentSkillModal();
                await loadAgentSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    function showAgentSkillImportModal() {
        selectedAgentImportFile = null;
        document.getElementById('agent-import-file').value = '';
        document.getElementById('agent-import-path').value = '';
        document.getElementById('agent-import-file-name').textContent = t('skills.agent_import_zip_hint');
        document.getElementById('agent-import-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeAgentSkillImportModal() {
        document.getElementById('agent-import-modal').classList.remove('active');
        selectedAgentImportFile = null;
    }

    // eslint-disable-next-line no-unused-vars
    function handleAgentImportFileSelect(event) {
        selectedAgentImportFile = event.target.files && event.target.files[0] ? event.target.files[0] : null;
        document.getElementById('agent-import-file-name').textContent = selectedAgentImportFile ? selectedAgentImportFile.name : t('skills.agent_import_zip_hint');
    }

    // eslint-disable-next-line no-unused-vars
    async function submitAgentSkillImport() {
        const sourcePath = document.getElementById('agent-import-path').value.trim();
        try {
            let resp;
            if (selectedAgentImportFile) {
                const form = new FormData();
                form.append('file', selectedAgentImportFile);
                resp = await fetch('/api/agent-skills/import', { method: 'POST', body: form });
            } else {
                resp = await fetch('/api/agent-skills/import', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ source_path: sourcePath })
                });
            }
            const data = await resp.json();
            if (data.status === 'imported') {
                showToast(t('skills.import_success'), 'success');
                closeAgentSkillImportModal();
                await loadAgentSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(e.message || t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function showAgentSkillTestModal(id) {
        currentAgentSkillId = id;
        const skill = allAgentSkills.find(s => String(s.id) === String(id));
        const scripts = (skill && skill.scripts) || [];
        const select = document.getElementById('agent-test-script');
        select.innerHTML = scripts.map(s => {
            const p = s.path || s.Path || '';
            return `<option value="${esc(p)}">${esc(p)}</option>`;
        }).join('');
        document.getElementById('agent-test-args').value = '{}';
        document.getElementById('agent-test-output').textContent = '';
        onAgentTestScriptChange();
        document.getElementById('agent-test-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function onAgentTestScriptChange() {
        const sel = document.getElementById('agent-test-script');
        const warn = document.getElementById('agent-nonpython-warning');
        if (!sel || !warn) return;
        const val = sel.value || '';
        const ext = val.split('.').pop().toLowerCase();
        warn.style.display = (ext === 'sh' || ext === 'js') ? '' : 'none';
    }

    // eslint-disable-next-line no-unused-vars
    function closeAgentSkillTestModal() {
        document.getElementById('agent-test-modal').classList.remove('active');
        currentAgentSkillId = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function runAgentSkillTest() {
        if (!currentAgentSkillId) return;
        let args = {};
        try {
            args = JSON.parse(document.getElementById('agent-test-args').value || '{}');
        } catch (e) {
            showToast(t('skills.test_invalid_json'), 'error');
            return;
        }
        const resp = await fetch(`/api/agent-skills/${encodeURIComponent(currentAgentSkillId)}/test`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ script: document.getElementById('agent-test-script').value, args })
        });
        const data = await resp.json();
        document.getElementById('agent-test-output').textContent = data.output || data.message || '';
        if (data.status === 'ok') {
            showToast(t('skills.test_success'), 'success');
        } else {
            showToast(data.message || t('skills.test_failed'), 'error');
        }
    }

    async function previewDetailResource(skillId, path) {
        try {
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(skillId)}/files?path=${encodeURIComponent(path)}`);
            const data = await resp.json();
            if (data.status === 'ok') {
                const w = window.open('', '_blank', 'noopener,noreferrer');
                if (!w) return;
                w.document.title = path;
                const pre = w.document.createElement('pre');
                pre.textContent = data.content || '';
                w.document.body.appendChild(pre);
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (_) {
            showToast(t('common.error'), 'error');
        }
    }

    async function downloadDetailResource(skillId, path) {
        try {
            const resp = await fetch(`/api/agent-skills/${encodeURIComponent(skillId)}/files/raw?path=${encodeURIComponent(path)}`);
            if (!resp.ok) { showToast(t('common.error'), 'error'); return; }
            const blob = await resp.blob();
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = path.split('/').pop();
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
        } catch (_) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function toggleSkill(id, enabled) {
        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(id)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled })
            });
            const data = await resp.json();
            if (data.status === 'ok') {
                showToast(t('skills.toggle_success'), 'success');
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // ── Detail Modal ────────────────────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    async function showDetail(id) {
        currentDetailId = id;
        currentAgentSkillId = '';
        const actions = document.querySelector('#detail-modal .modal-actions');
        if (actions) actions.style.display = '';
        const body = document.getElementById('detail-modal-body');
        body.innerHTML = `<p>${t('common.loading')}</p>`;
        document.getElementById('detail-modal').classList.add('active');

        try {
            const [skillResp, versionsResp, auditResp] = await Promise.all([
                fetch(`/api/skills/${encodeURIComponent(id)}`),
                fetch(`/api/skills/${encodeURIComponent(id)}/versions`),
                fetch(`/api/skills/${encodeURIComponent(id)}/audit?limit=20`)
            ]);
            const data = await skillResp.json();
            if (data.status !== 'ok') {
                body.innerHTML = `<p class="sk-error">${esc(data.message || 'Not found')}</p>`;
                return;
            }
            const s = data.skill;
            const isDaemon = s.IsDaemon || s.is_daemon;
            if (isDaemon) await loadDaemonSettingsOptions();
            detailVersions = versionsResp.ok ? ((await versionsResp.json()).versions || []) : [];
            detailAudit = auditResp.ok ? ((await auditResp.json()).audit || []) : [];
            const sec = s.SecurityReport || s.security_report;
            const deps = (s.Dependencies || s.dependencies || []);
            const vaultKeys = (s.VaultKeys || s.vault_keys || []);
            const internalTools = (s.InternalTools || s.internal_tools || []);
            const tags = (s.Tags || s.tags || []);
            const category = (s.Category || s.category || '');

            let secHTML = '';
            if (sec) {
                const findings = sec.StaticAnalysis || sec.static_analysis || [];
                if (findings.length > 0) {
                    secHTML = `<div class="sk-findings">
                    <h4 data-i18n="skills.findings_title">${t('skills.findings_title')}</h4>
                    <ul>${findings.map(f => `<li class="sk-finding sk-finding-${(f.Severity || f.severity || 'info').toLowerCase()}">
                        <strong>${esc(f.Category || f.category || '')}</strong>: ${esc(f.Message || f.message || '')}
                        ${f.Line || f.line ? ` <span class="sk-finding-line">(${t('skills.finding_line')} ${f.Line || f.line})</span>` : ''}
                    </li>`).join('')}</ul>
                </div>`;
                }
                if (sec.GuardianDecision || sec.guardian_decision) {
                    secHTML += `<div class="sk-guardian-result">
                    <h4>${t('skills.guardian_title')}</h4>
                    <p><strong>${t('skills.guardian_decision')}:</strong> ${esc(sec.GuardianDecision || sec.guardian_decision)}</p>
                    ${sec.GuardianReason || sec.guardian_reason ? `<p>${esc(sec.GuardianReason || sec.guardian_reason)}</p>` : ''}
                </div>`;
                }
            }

            body.innerHTML = `
            <div class="sk-detail-grid">
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_name')}:</span> <span>${esc(s.Name || s.name)}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_type')}:</span> <span class="sk-type-badge sk-type-${(s.Type || s.type || '').toLowerCase()}">${esc(s.Type || s.type)}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_status')}:</span> ${renderSecurityBadge((s.SecurityStatus || s.security_status || 'pending').toLowerCase())}</div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_enabled')}:</span> <span>${s.Enabled || s.enabled ? '✅' : '❌'}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_created')}:</span> <span>${esc(s.CreatedAt || s.created_at || '-')}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_description')}:</span> <span>${esc(s.Description || s.description || '-')}</span></div>
                ${category ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.field_category')}:</span> <span>${esc(category)}</span></div>` : ''}
                ${tags.length > 0 ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.field_tags')}:</span> <span class="sk-meta-list">${tags.map(tag => `<span class="sk-dep-tag">${esc(tag)}</span>`).join('')}</span></div>` : ''}
                ${deps.length > 0 ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_deps')}:</span> <span>${deps.map(d => `<span class="sk-dep-tag">${esc(d)}</span>`).join(' ')}</span></div>` : ''}
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.vault_keys_label')}:</span> <span>${vaultKeys.length > 0 ? vaultKeys.map(k => {
                if (k.startsWith('cred:')) {
                    const cname = credentialMap[k.slice(5)] || k.slice(5);
                    return `<code class="sk-vault-key-tag sk-vault-key-cred" title="${esc(k)}">🔑 ${esc(cname)}</code>`;
                }
                return `<code class="sk-vault-key-tag">${esc(k)}</code>`;
            }).join(' ') : `<span class="sk-vault-none">${t('skills.vault_none')}</span>`}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">⚙️ ${t('skills.internal_tools_label')}:</span> <span>${internalTools.length > 0 ? internalTools.map(tool => `<code class="sk-dep-tag sk-internal-tool-tag">${esc(tool)}</code>`).join(' ') : `<span class="sk-vault-none">${t('skills.internal_tools_none')}</span>`}</span></div>
            </div>
            ${renderDaemonSettings(s, data.daemon)}
            ${renderSkillDocumentation(s)}
            ${renderSkillHistory(detailVersions)}
            ${renderSkillAudit(detailAudit)}
            ${secHTML}`;
            if (typeof applyI18n === 'function') applyI18n();
            // Documentation section is loaded lazily so the panel opens fast.
            const hasDoc = s.HasDocumentation || s.has_documentation;
            if (hasDoc) {
                loadAndRenderSkillDocumentation(s.ID || s.id);
            }
        } catch (e) {
            body.innerHTML = `<p class="sk-error">${t('common.error')}</p>`;
        }
    }

    // eslint-disable-next-line no-unused-vars
    function closeDetailModal() {
        document.getElementById('detail-modal').classList.remove('active');
        currentDetailId = '';
        currentAgentSkillId = '';
        const actions = document.querySelector('#detail-modal .modal-actions');
        if (actions) actions.style.display = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function verifyCurrentSkill() {
        if (!currentDetailId) return;
        const btn = document.getElementById('detail-verify-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading');

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(currentDetailId)}/verify`, { method: 'POST' });
            const data = await resp.json();
            if (data.status === 'scanned') {
                showToast(t('skills.scan_complete'), 'success');
                showDetail(currentDetailId);
                loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_verify');
        }
    }

    // ── Code Editor Modal (CodeMirror 6) ────────────────────────────────────────

    async function loadCodeMirror() {
        if (!codeMirrorModulePromise) {
            codeMirrorModulePromise = import('/js/vendor/codemirror-bundle.esm.js');
        }
        return codeMirrorModulePromise;
    }

    function createEditorState(cm, code, readOnly) {
        if (!cm || !cm.EditorState || !cm.EditorView) {
            throw new Error('CodeMirror unavailable');
        }
        const extensions = [
            cm.lineNumbers && cm.lineNumbers(),
            cm.highlightActiveLineGutter && cm.highlightActiveLineGutter(),
            cm.highlightSpecialChars && cm.highlightSpecialChars(),
            cm.history && cm.history(),
            cm.drawSelection && cm.drawSelection(),
            cm.dropCursor && cm.dropCursor(),
            cm.syntaxHighlighting && cm.defaultHighlightStyle && cm.syntaxHighlighting(cm.defaultHighlightStyle, { fallback: true }),
            cm.closeBrackets && cm.closeBrackets(),
            cm.autocompletion && cm.autocompletion(),
            cm.rectangularSelection && cm.rectangularSelection(),
            cm.crosshairCursor && cm.crosshairCursor(),
            cm.highlightActiveLine && cm.highlightActiveLine(),
            cm.highlightSelectionMatches && cm.highlightSelectionMatches(),
            cm.EditorView.lineWrapping,
            cm.python && cm.python(),
            cm.oneDark,
            cm.keymap && cm.keymap.of([
                ...(cm.closeBracketsKeymap || []),
                ...(cm.defaultKeymap || []),
                ...(cm.searchKeymap || []),
                ...(cm.historyKeymap || []),
                ...(cm.completionKeymap || []),
                cm.indentWithTab,
            ].filter(Boolean)),
        ].filter(Boolean);
        if (readOnly) {
            extensions.push(cm.EditorState.readOnly.of(true));
        }
        return cm.EditorState.create({ doc: code, extensions });
    }

    function getCurrentEditorCode() {
        if (codeEditorView && codeEditorView.state && codeEditorView.state.doc) {
            return codeEditorView.state.doc.toString();
        }
        const fallback = document.getElementById('code-editor-fallback');
        return fallback ? fallback.value : '';
    }

    function setDraftMeta(meta, readOnly) {
        codeEditorDraft = meta || null;
        const metaWrap = document.getElementById('code-draft-meta');
        const saveBtn = document.getElementById('code-save-btn');
        if (!meta) {
            metaWrap.style.display = 'none';
            saveBtn.textContent = t('skills.btn_save_code');
            return;
        }
        metaWrap.style.display = '';
        document.getElementById('code-draft-name').value = meta.name || '';
        document.getElementById('code-draft-description').value = meta.description || '';
        document.getElementById('code-draft-category').value = meta.category || '';
        document.getElementById('code-draft-tags').value = (meta.tags || []).join(', ');
        const docEl = document.getElementById('code-draft-documentation');
        if (docEl) docEl.value = meta.documentation || '';
        saveBtn.textContent = readOnly ? (t('common.btn_close')) : (t('skills.btn_create_skill'));
    }

    // eslint-disable-next-line no-unused-vars
    async function openCodeEditor(id, code, readOnly, draftMeta = null) {
        codeEditorSkillId = id;
        const container = document.getElementById('code-editor-container');
        const loadingLabel = t('common.loading');
        container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${loadingLabel && loadingLabel !== 'common.loading' ? loadingLabel : 'Loading...'}</p>`;

        if (codeEditorView) {
            codeEditorView.destroy();
            codeEditorView = null;
        }

        try {
            const cm = await loadCodeMirror();
            container.innerHTML = '';
            const state = createEditorState(cm, code || '', readOnly);
            codeEditorView = new cm.EditorView({
                state,
                parent: container,
            });
        } catch (_) {
            container.innerHTML = `<textarea id="code-editor-fallback" class="sk-editor-fallback" ${readOnly ? 'readonly' : ''}></textarea>`;
            document.getElementById('code-editor-fallback').value = code || '';
            showToast(t('skills.editor_fallback'), 'info');
        }

        const saveBtn = document.getElementById('code-save-btn');
        saveBtn.style.display = readOnly ? 'none' : '';
        setDraftMeta(draftMeta, readOnly);

        document.getElementById('code-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    async function viewCode(id) {
        document.getElementById('code-modal').classList.add('active');
        const container = document.getElementById('code-editor-container');
        container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('common.loading')}</p>`;

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(id)}?code=true`);
            const data = await resp.json();
            if (data.status === 'ok' && data.code) {
                container.innerHTML = '';
                await openCodeEditor(id, data.code, false, null);
            } else {
                container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('skills.no_code')}</p>`;
            }
        } catch (e) {
            container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('skills.failed_to_load_code')}</p>`;
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function saveSkillCode() {
        const code = getCurrentEditorCode();
        if (!code) return;
        const btn = document.getElementById('code-save-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading');

        try {
            let resp;
            if (codeEditorSkillId) {
                resp = await fetch(`/api/skills/${encodeURIComponent(codeEditorSkillId)}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ code })
                });
            } else {
                const name = document.getElementById('code-draft-name').value.trim();
                const description = document.getElementById('code-draft-description').value.trim();
                const category = document.getElementById('code-draft-category').value.trim();
                const tags = parseCSV(document.getElementById('code-draft-tags').value);
                const docEl = document.getElementById('code-draft-documentation');
                const documentation = docEl ? docEl.value : '';
                resp = await fetch('/api/skills', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, description, category, tags, code, documentation })
                });
            }
            const data = await resp.json();
            if (data.status === 'ok' || data.status === 'created') {
                showToast(codeEditorSkillId ? (t('skills.code_saved')) : (t('skills.create_success')), 'success');
                await loadSkills();
                closeCodeModal();
                if (!codeEditorSkillId && data.skill && data.skill.id) {
                    currentDetailId = data.skill.id;
                }
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_save_code');
        }
    }

    // eslint-disable-next-line no-unused-vars
    function closeCodeModal() {
        const modal = document.getElementById('code-modal');
        modal.classList.remove('active');
        modal.classList.remove('sk-code-overlay-fullscreen');
        if (codeEditorView) {
            codeEditorView.destroy();
            codeEditorView = null;
        }
        codeEditorSkillId = '';
        codeEditorDraft = null;
        document.getElementById('code-draft-meta').style.display = 'none';
    }

    // eslint-disable-next-line no-unused-vars
    function toggleCodeFullscreen() {
        document.getElementById('code-modal').classList.toggle('sk-code-overlay-fullscreen');
    }

    // ── Upload Modal ────────────────────────────────────────────────────────────

    function initDropzone() {
        const dz = document.getElementById('sk-dropzone');
        if (!dz) return;

        dz.addEventListener('click', () => document.getElementById('upload-file').click());
        dz.addEventListener('dragover', (e) => { e.preventDefault(); dz.classList.add('sk-dropzone-active'); });
        dz.addEventListener('dragleave', () => dz.classList.remove('sk-dropzone-active'));
        dz.addEventListener('drop', (e) => {
            e.preventDefault();
            dz.classList.remove('sk-dropzone-active');
            if (e.dataTransfer.files.length > 0) {
                selectFile(e.dataTransfer.files[0]);
            }
        });
    }

    // eslint-disable-next-line no-unused-vars
    function handleFileSelect(event) {
        if (event.target.files.length > 0) {
            selectFile(event.target.files[0]);
        }
    }

    function selectFile(file) {
        if (!file.name.endsWith('.py')) {
            showToast(t('skills.upload_py_only'), 'error');
            return;
        }
        selectedFile = file;
        document.getElementById('sk-dropzone').style.display = 'none';
        document.getElementById('sk-selected-file').style.display = '';
        document.getElementById('sk-selected-name').textContent = file.name;
        document.getElementById('upload-submit-btn').disabled = false;
    }

    // eslint-disable-next-line no-unused-vars
    function clearFileSelection() {
        selectedFile = null;
        document.getElementById('sk-dropzone').style.display = '';
        document.getElementById('sk-selected-file').style.display = 'none';
        document.getElementById('upload-submit-btn').disabled = true;
        document.getElementById('upload-file').value = '';
    }

    // eslint-disable-next-line no-unused-vars
    function showUploadModal() {
        clearFileSelection();
        document.getElementById('upload-name').value = '';
        document.getElementById('upload-description').value = '';
        document.getElementById('upload-category').value = '';
        document.getElementById('upload-tags').value = '';
        const docEl = document.getElementById('upload-documentation');
        if (docEl) docEl.value = '';
        document.getElementById('upload-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeUploadModal() {
        document.getElementById('upload-modal').classList.remove('active');
    }

    // eslint-disable-next-line no-unused-vars
    async function submitUpload() {
        if (!selectedFile) return;
        const btn = document.getElementById('upload-submit-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading');

        const fd = new FormData();
        fd.append('file', selectedFile);
        fd.append('name', document.getElementById('upload-name').value.trim());
        fd.append('description', document.getElementById('upload-description').value.trim());
        fd.append('category', document.getElementById('upload-category').value.trim());
        fd.append('tags', document.getElementById('upload-tags').value.trim());
        const uploadDocEl = document.getElementById('upload-documentation');
        if (uploadDocEl && uploadDocEl.value.trim()) {
            fd.append('documentation', uploadDocEl.value);
        }

        try {
            const resp = await fetch('/api/skills/upload', { method: 'POST', body: fd });
            const data = await resp.json();

            if (data.status === 'uploaded' || data.status === 'created') {
                showToast(t('skills.upload_success'), 'success');
                closeUploadModal();
                await loadSkills();
            } else if (data.status === 'rejected') {
                const msgs = (data.validation && data.validation.Errors) ? data.validation.Errors.join(', ') : (data.message || 'Validation failed');
                showToast(msgs, 'error');
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_upload');
        }
    }

    // ── Template Modal ──────────────────────────────────────────────────────────

    function populateTemplateSelect() {
        const sel = document.getElementById('template-select');
        if (!sel) return;
        // Keep the first "choose" option
        while (sel.options.length > 1) sel.remove(1);
        allTemplates.forEach(tmpl => {
            const opt = document.createElement('option');
            opt.value = tmpl.Name || tmpl.name || '';
            opt.textContent = opt.value;
            sel.appendChild(opt);
        });
    }

    function populateGenerateTemplateSelect() {
        const sel = document.getElementById('generate-template-select');
        if (!sel) return;
        while (sel.options.length > 1) sel.remove(1);
        allTemplates.forEach(tmpl => {
            const opt = document.createElement('option');
            opt.value = tmpl.Name || tmpl.name || '';
            opt.textContent = opt.value;
            sel.appendChild(opt);
        });
    }

    // eslint-disable-next-line no-unused-vars
    function onTemplateSelect() {
        const sel = document.getElementById('template-select');
        const descDiv = document.getElementById('template-description');
        const btn = document.getElementById('template-submit-btn');
        const tmpl = allTemplates.find(t => (t.Name || t.name) === sel.value);

        if (tmpl) {
            descDiv.textContent = tmpl.Description || tmpl.description || '';
            descDiv.style.display = '';
            btn.disabled = false;
        } else {
            descDiv.style.display = 'none';
            btn.disabled = true;
        }
    }

    // eslint-disable-next-line no-unused-vars
    function showTemplateModal() {
        document.getElementById('template-skill-name').value = '';
        document.getElementById('template-description-input').value = '';
        document.getElementById('template-category-input').value = '';
        document.getElementById('template-tags-input').value = '';
        document.getElementById('template-base-url-input').value = '';
        document.getElementById('template-dependencies-input').value = '';
        document.getElementById('template-select').selectedIndex = 0;
        document.getElementById('template-description').style.display = 'none';
        document.getElementById('template-submit-btn').disabled = true;
        document.getElementById('template-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeTemplateModal() {
        document.getElementById('template-modal').classList.remove('active');
    }

    // eslint-disable-next-line no-unused-vars
    async function submitTemplate() {
        const templateName = document.getElementById('template-select').value;
        const skillName = document.getElementById('template-skill-name').value.trim();
        const description = document.getElementById('template-description-input').value.trim();
        const category = document.getElementById('template-category-input').value.trim();
        const tags = parseCSV(document.getElementById('template-tags-input').value);
        const baseURL = document.getElementById('template-base-url-input').value.trim();
        const dependencies = parseCSV(document.getElementById('template-dependencies-input').value);
        const tplDocEl = document.getElementById('template-documentation');
        const documentation = tplDocEl ? tplDocEl.value : '';

        if (!templateName || !skillName) {
            showToast(t('skills.template_required'), 'error');
            return;
        }

        const btn = document.getElementById('template-submit-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading');

        try {
            const resp = await fetch('/api/skills/templates', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    template_name: templateName,
                    skill_name: skillName,
                    description: description,
                    category: category,
                    tags: tags,
                    base_url: baseURL,
                    dependencies: dependencies,
                    documentation: documentation
                })
            });
            const data = await resp.json();

            if (data.status === 'created') {
                showToast(t('skills.template_success'), 'success');
                closeTemplateModal();
                await loadSkills();
                // Open the code editor for the newly created skill
                if (data.skill_id) {
                    viewCode(data.skill_id);
                }
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_create');
        }
    }

    // ── Delete Modal ────────────────────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    function showDeleteSkillModal(id, name) {
        deleteTargetId = id;
        skillDeleteInFlight = false;
        document.getElementById('delete-skill-name').textContent = name;
        document.getElementById('delete-files-checkbox').checked = true;
        setSkillDeleteBusy(false);
        document.getElementById('delete-skill-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function deleteAgentSkill(id, name) {
        deleteTargetId = `agent:${id}`;
        skillDeleteInFlight = false;
        document.getElementById('delete-skill-name').textContent = name;
        document.getElementById('delete-files-checkbox').checked = true;
        setSkillDeleteBusy(false);
        document.getElementById('delete-skill-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeDeleteSkillModal() {
        document.getElementById('delete-skill-modal').classList.remove('active');
        deleteTargetId = '';
        skillDeleteInFlight = false;
        setSkillDeleteBusy(false);
    }

    // eslint-disable-next-line no-unused-vars
    async function confirmDeleteSkill() {
        if (!deleteTargetId || skillDeleteInFlight) return;
        skillDeleteInFlight = true;
        setSkillDeleteBusy(true);
        const deleteFiles = document.getElementById('delete-files-checkbox').checked;

        try {
            const isAgentSkillDelete = deleteTargetId.startsWith('agent:');
            const rawID = isAgentSkillDelete ? deleteTargetId.slice(6) : deleteTargetId;
            const apiPath = isAgentSkillDelete ? '/api/agent-skills/' : '/api/skills/';
            const resp = await fetch(`${apiPath}${encodeURIComponent(rawID)}?delete_files=${deleteFiles}`, { method: 'DELETE' });
            const data = await resp.json();
            if (data.status === 'deleted') {
                showToast(t('skills.delete_success'), 'success');
                closeDeleteSkillModal();
                closeDetailModal();
                if (isAgentSkillDelete) await loadAgentSkills(); else await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        } finally {
            if (deleteTargetId) {
                skillDeleteInFlight = false;
                setSkillDeleteBusy(false);
            }
        }
    }

    function setSkillDeleteBusy(busy) {
        const confirmBtn = document.getElementById('skill-delete-confirm-btn');
        if (confirmBtn) {
            confirmBtn.disabled = busy;
        }
    }

    function renderSkillDocumentation(s) {
        const has = s.HasDocumentation || s.has_documentation;
        const hint = t('skills.documentation_hint');
        const editLabel = has ? (t('skills.documentation_edit')) : (t('skills.documentation_add'));
        const deleteBtn = has ? `<button class="btn btn-sm btn-danger" onclick="deleteSkillDocumentation()" data-i18n="skills.documentation_delete">${t('skills.documentation_delete')}</button>` : '';
        const placeholder = has ? `<p class="sk-loading">${t('common.loading')}</p>` : `<p class="sk-vault-none" data-i18n="skills.documentation_empty">${t('skills.documentation_empty')}</p>`;
        return `<div class="sk-findings sk-documentation-block">
        <h4>📖 ${t('skills.documentation_title')}</h4>
        <p class="sk-inline-help">${esc(hint)}</p>
        <div id="sk-documentation-content" class="sk-documentation-content">${placeholder}</div>
        <div class="sk-documentation-actions">
            <button class="btn btn-sm btn-secondary" onclick="openSkillDocumentationEditor()">${esc(editLabel)}</button>
            ${deleteBtn}
        </div>
    </div>`;
    }

    async function loadAndRenderSkillDocumentation(id) {
        const target = document.getElementById('sk-documentation-content');
        if (!target) return;
        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(id)}/documentation`);
            const data = await resp.json();
            if (resp.ok && data.status === 'ok' && data.content) {
                const rendered = renderSkillDocumentationMarkdown(data.content);
                target.innerHTML = `<div class="sk-documentation-rendered">${rendered}</div>`;
            } else {
                target.innerHTML = `<p class="sk-vault-none">${t('skills.documentation_empty')}</p>`;
            }
        } catch (_) {
            target.innerHTML = `<p class="sk-error">${t('common.error')}</p>`;
        }
    }

    function renderSkillDocumentationMarkdown(content) {
        if (window.marked && window.DOMPurify && typeof window.DOMPurify.sanitize === 'function') {
            return window.DOMPurify.sanitize(window.marked.parse(content), { USE_PROFILES: { html: true } });
        }
        return `<pre class="sk-documentation-pre">${esc(content)}</pre>`;
    }

    // eslint-disable-next-line no-unused-vars
    async function openSkillDocumentationEditor() {
        if (!currentDetailId) return;
        let current = '';
        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(currentDetailId)}/documentation`);
            if (resp.ok) {
                const data = await resp.json();
                current = data.content || '';
            }
        } catch (_) { /* ignore, start empty */ }

        const overlay = document.createElement('div');
        overlay.className = 'modal-overlay active';
        overlay.id = 'sk-doc-editor-modal';
        overlay.innerHTML = `<div class="modal modal-wide">
            <div class="modal-header">
                <h2>${t('skills.documentation_title')}</h2>
                <button class="modal-close" onclick="closeSkillDocumentationEditor()">&times;</button>
            </div>
            <div class="modal-body">
                <p class="sk-inline-help">${esc(t('skills.documentation_hint'))}</p>
                <textarea id="sk-doc-editor-textarea" class="sk-input sk-textarea" rows="20" style="font-family:monospace"></textarea>
                <div class="sk-form-group" style="margin-top:8px">
                    <label>${esc(t('skills.documentation_upload'))}</label>
                    <input type="file" id="sk-doc-editor-file" accept=".md,.markdown,.txt" onchange="handleSkillDocumentationFileSelect(event)">
                </div>
            </div>
            <div class="modal-actions">
                <button class="btn btn-secondary" onclick="closeSkillDocumentationEditor()">${t('common.btn_cancel')}</button>
                <button class="btn btn-primary" onclick="saveSkillDocumentation()">${t('common.btn_save')}</button>
            </div>
        </div>`;
        document.body.appendChild(overlay);
        document.getElementById('sk-doc-editor-textarea').value = current;
    }

    // eslint-disable-next-line no-unused-vars
    function closeSkillDocumentationEditor() {
        const m = document.getElementById('sk-doc-editor-modal');
        if (m) m.remove();
    }

    // eslint-disable-next-line no-unused-vars
    function handleSkillDocumentationFileSelect(event) {
        const f = event.target.files && event.target.files[0];
        if (!f) return;
        if (f.size > 64 * 1024) {
            showToast(t('skills.documentation_too_large'), 'error');
            return;
        }
        const reader = new FileReader();
        reader.onload = (e) => {
            document.getElementById('sk-doc-editor-textarea').value = e.target.result || '';
        };
        reader.readAsText(f, 'utf-8');
    }

    // eslint-disable-next-line no-unused-vars
    async function saveSkillDocumentation() {
        if (!currentDetailId) return;
        const content = document.getElementById('sk-doc-editor-textarea').value;
        if (!content.trim()) {
            showToast(t('skills.documentation_empty_save'), 'error');
            return;
        }
        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(currentDetailId)}/documentation`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ content })
            });
            const data = await resp.json().catch(() => ({}));
            if (resp.ok && data.status === 'ok') {
                showToast(t('skills.documentation_saved'), 'success');
                closeSkillDocumentationEditor();
                showDetail(currentDetailId);
            } else if (resp.status === 413) {
                showToast(t('skills.documentation_too_large'), 'error');
            } else if (resp.status === 403) {
                showToast(t('skills.documentation_readonly'), 'error');
            } else {
                showToast(data.message || (t('common.error')), 'error');
            }
        } catch (_) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function deleteSkillDocumentation() {
        if (!currentDetailId) return;
        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(currentDetailId)}/documentation`, { method: 'DELETE' });
            if (resp.ok) {
                showToast(t('skills.documentation_deleted'), 'success');
                showDetail(currentDetailId);
            } else if (resp.status === 403) {
                showToast(t('skills.documentation_readonly'), 'error');
            } else {
                const data = await resp.json().catch(() => ({}));
                showToast(data.message || (t('common.error')), 'error');
            }
        } catch (_) {
            showToast(t('common.error'), 'error');
        }
    }

    function renderSkillHistory(versions) {
        if (!Array.isArray(versions) || versions.length === 0) return '';
        return `<div class="sk-findings">
        <h4>${t('skills.history_title')}</h4>
        <ul class="sk-history-list">
            ${versions.slice(0, 8).map(v => `<li class="sk-history-item">
                <div class="sk-history-item-head">
                    <span>${t('skills.history_version')} ${esc(String(v.version || v.Version || '?'))}</span>
                    <button class="btn btn-sm btn-secondary" onclick="restoreSkillVersion('${esc(currentDetailId)}', ${Number(v.version || v.Version || 0)})">${t('skills.btn_restore')}</button>
                </div>
                <div class="sk-history-note">${esc(v.change_note || v.ChangeNote || '')}</div>
                <div class="sk-audit-details">${esc(v.created_by || v.CreatedBy || '')} · ${esc(v.created_at || v.CreatedAt || '')}</div>
            </li>`).join('')}
        </ul>
    </div>`;
    }

    function renderSkillAudit(audit) {
        if (!Array.isArray(audit) || audit.length === 0) return '';
        return `<div class="sk-findings">
        <h4>${t('skills.audit_title')}</h4>
        <ul class="sk-audit-list">
            ${audit.slice(0, 12).map(entry => `<li class="sk-audit-item">
                <div class="sk-audit-item-head">
                    <strong>${esc(entry.action || entry.Action || '')}</strong>
                    <span>${esc(entry.created_at || entry.CreatedAt || '')}</span>
                </div>
                <div class="sk-audit-details">${esc(entry.actor || entry.Actor || '')}${(entry.details || entry.Details) ? ' · ' + esc(entry.details || entry.Details) : ''}</div>
            </li>`).join('')}
        </ul>
    </div>`;
    }

    // eslint-disable-next-line no-unused-vars
    async function restoreSkillVersion(id, version) {
        if (!id || !version) return;
        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(id)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ restore_version: version })
            });
            const data = await resp.json();
            if (data.status === 'ok') {
                showToast(t('skills.restore_success'), 'success');
                await showDetail(id);
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (_) {
            showToast(t('common.error'), 'error');
        }
    }

    // eslint-disable-next-line no-unused-vars
    function exportCurrentSkill() {
        if (!currentDetailId) return;
        window.location.href = `/api/skills/${encodeURIComponent(currentDetailId)}/export`;
    }

    // eslint-disable-next-line no-unused-vars
    function showTestModal() {
        if (!currentDetailId) return;
        document.getElementById('test-args-input').value = '{}';
        document.getElementById('test-output').textContent = '';
        document.getElementById('test-output-status').textContent = '';
        document.getElementById('test-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeTestModal() {
        document.getElementById('test-modal').classList.remove('active');
    }

    // eslint-disable-next-line no-unused-vars
    async function runSkillTest() {
        if (!currentDetailId) return;
        const btn = document.getElementById('test-run-btn');
        const statusEl = document.getElementById('test-output-status');
        const outputEl = document.getElementById('test-output');
        btn.disabled = true;
        btn.textContent = t('common.loading');
        try {
            const args = JSON.parse(document.getElementById('test-args-input').value || '{}');
            const resp = await fetch(`/api/skills/${encodeURIComponent(currentDetailId)}/test`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ args })
            });
            const data = await resp.json();
            statusEl.textContent = data.status || '';
            outputEl.textContent = data.output || data.message || '';
            if (data.status === 'ok') {
                showToast(t('skills.test_success'), 'success');
            } else {
                showToast(data.message || (t('skills.test_failed')), 'error');
            }
        } catch (e) {
            statusEl.textContent = 'error';
            outputEl.textContent = e.message || 'Invalid JSON';
            showToast(t('skills.test_invalid_json'), 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_run_test');
        }
    }

    // eslint-disable-next-line no-unused-vars
    function showImportModal() {
        selectedImportFile = null;
        document.getElementById('import-file').value = '';
        document.getElementById('import-file-name').textContent = t('skills.import_hint');
        document.getElementById('import-submit-btn').disabled = true;
        document.getElementById('import-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeImportModal() {
        document.getElementById('import-modal').classList.remove('active');
    }

    // eslint-disable-next-line no-unused-vars
    function handleImportFileSelect(event) {
        if (!event.target.files || event.target.files.length === 0) return;
        selectedImportFile = event.target.files[0];
        document.getElementById('import-file-name').textContent = selectedImportFile.name;
        document.getElementById('import-submit-btn').disabled = false;
    }

    // eslint-disable-next-line no-unused-vars
    async function submitImportSkill() {
        if (!selectedImportFile) return;
        const btn = document.getElementById('import-submit-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading');
        try {
            const bundle = JSON.parse(await selectedImportFile.text());
            const resp = await fetch('/api/skills/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(bundle)
            });
            const data = await resp.json();
            if (data.status === 'imported') {
                showToast(t('skills.import_success'), 'success');
                closeImportModal();
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(e.message || (t('common.error')), 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_import');
        }
    }

    // eslint-disable-next-line no-unused-vars
    function showGenerateModal() {
        document.getElementById('generate-prompt').value = '';
        document.getElementById('generate-skill-name').value = '';
        document.getElementById('generate-category').value = '';
        document.getElementById('generate-template-select').selectedIndex = 0;
        document.getElementById('generate-dependencies').value = '';
        clearGenerateStatus();
        document.getElementById('generate-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeGenerateModal() {
        clearGenerateStatus();
        document.getElementById('generate-modal').classList.remove('active');
    }

    function setGenerateStatus(message, type = 'info') {
        const el = document.getElementById('generate-status');
        if (!el) return;
        if (!message) {
            el.textContent = '';
            el.className = 'sk-generate-status';
            el.style.display = 'none';
            return;
        }
        el.textContent = message;
        el.className = `sk-generate-status sk-generate-status-${type}`;
        el.style.display = '';
    }

    function clearGenerateStatus() {
        setGenerateStatus('');
    }

    async function readResponseJSON(resp) {
        const raw = await resp.text();
        if (!raw) return {};
        try {
            return JSON.parse(raw);
        } catch (_) {
            return { error: raw };
        }
    }

    function getResponseError(data, fallback) {
        if (!data || typeof data !== 'object') return fallback;
        return data.message || data.error || fallback;
    }

    // eslint-disable-next-line no-unused-vars
    async function submitGenerateSkill() {
        const prompt = document.getElementById('generate-prompt').value.trim();
        if (!prompt) {
            showToast(t('skills.generate_prompt_required'), 'error');
            return;
        }
        const btn = document.getElementById('generate-submit-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading');
        setGenerateStatus(t('skills.generate_in_progress'), 'info');
        const controller = new AbortController();
        const timeoutHandle = window.setTimeout(() => controller.abort(), 90000);
        try {
            const resp = await fetch('/api/skills/generate', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                signal: controller.signal,
                body: JSON.stringify({
                    prompt,
                    skill_name: document.getElementById('generate-skill-name').value.trim(),
                    category: document.getElementById('generate-category').value.trim(),
                    template_name: document.getElementById('generate-template-select').value,
                    dependencies: parseCSV(document.getElementById('generate-dependencies').value)
                })
            });
            const data = await readResponseJSON(resp);
            if (!resp.ok || data.status !== 'ok' || !data.draft) {
                const message = getResponseError(data, t('common.error'));
                setGenerateStatus(message, 'error');
                showToast(message, 'error');
                return;
            }
            setGenerateStatus(t('skills.generate_success'), 'success');
            closeGenerateModal();
            await openCodeEditor('', data.draft.code || '', false, {
                name: data.draft.name || '',
                description: data.draft.description || '',
                category: data.draft.category || '',
                tags: data.draft.tags || [],
                documentation: data.draft.documentation || ''
            });
            showToast(t('skills.generate_success'), 'success');
        } catch (e) {
            const message = e && e.name === 'AbortError'
                ? (t('skills.generate_timeout'))
                : (e.message || (t('common.error')));
            setGenerateStatus(message, 'error');
            showToast(message, 'error');
        } finally {
            window.clearTimeout(timeoutHandle);
            btn.disabled = false;
            btn.textContent = t('skills.btn_generate');
        }
    }

    function parseCSV(raw) {
        return (raw || '')
            .split(',')
            .map(part => part.trim())
            .filter(Boolean);
    }

    // ── Vault Key Assignment Modal ───────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    async function openVaultKeyModal(id) {
        vaultKeyTargetId = id;
        const listEl = document.getElementById('vault-key-list');
        const emptyEl = document.getElementById('vault-key-empty');
        listEl.innerHTML = `<p>${t('common.loading')}</p>`;
        emptyEl.style.display = 'none';
        document.getElementById('vault-key-modal').classList.add('active');

        try {
            // Load vault secrets, credentials, and current skill in parallel
            // ?filter=user excludes internal/system secrets from the list
            const [vaultResp, credResp, skillResp] = await Promise.all([
                fetch('/api/vault/secrets?filter=user'),
                fetch('/api/credentials'),
                fetch(`/api/skills/${encodeURIComponent(id)}`)
            ]);
            const vaultData = await vaultResp.json();
            const credData = await credResp.json().catch(() => []);
            const skillData = await skillResp.json();

            const rawSecrets = Array.isArray(vaultData) ? vaultData : (vaultData.secrets || []);
            allVaultSecrets = rawSecrets.map(s => (typeof s === 'string' ? s : s.key));
            const currentKeys = skillData.skill ? (skillData.skill.VaultKeys || skillData.skill.vault_keys || []) : [];

            // Credentials become selectable as cred:<id>
            const credList = (credResp.ok && Array.isArray(credData)) ? credData : (credData.items || credData.credentials || []);

            if (allVaultSecrets.length === 0 && credList.length === 0) {
                listEl.innerHTML = '';
                emptyEl.style.display = '';
                return;
            }

            let html = '';

            if (allVaultSecrets.length > 0) {
                html += `<p class="sk-vault-section-label">${t('skills.vault_section_secrets')}</p>`;
                html += allVaultSecrets.map(key => {
                    const checked = currentKeys.includes(key) ? 'checked' : '';
                    return `<label class="sk-vault-checkbox-row">
                    <input type="checkbox" class="sk-vault-checkbox" value="${esc(key)}" ${checked}>
                    <code class="sk-vault-key-tag">${esc(key)}</code>
                </label>`;
                }).join('');
            }

            if (credList.length > 0) {
                html += `<p class="sk-vault-section-label" style="margin-top:10px">${t('skills.vault_section_creds')}</p>`;
                html += credList.map(c => {
                    const credKey = `cred:${c.id}`;
                    const checked = currentKeys.includes(credKey) ? 'checked' : '';
                    const label = c.name || c.id;
                    const sub = c.type ? ` <span class="sk-vault-cred-type">${esc(c.type)}</span>` : '';
                    return `<label class="sk-vault-checkbox-row">
                    <input type="checkbox" class="sk-vault-checkbox" value="${esc(credKey)}" ${checked}>
                    <span class="sk-vault-cred-label">${esc(label)}${sub}</span>
                </label>`;
                }).join('');
            }

            listEl.innerHTML = html;
        } catch (e) {
            listEl.innerHTML = `<p class="sk-error">${t('common.error')}</p>`;
        }
    }

    // eslint-disable-next-line no-unused-vars
    function closeVaultKeyModal() {
        document.getElementById('vault-key-modal').classList.remove('active');
        vaultKeyTargetId = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function saveVaultKeys() {
        if (!vaultKeyTargetId) return;
        const checkboxes = document.querySelectorAll('#vault-key-list .sk-vault-checkbox:checked');
        const selectedKeys = Array.from(checkboxes).map(cb => cb.value);

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(vaultKeyTargetId)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ vault_keys: selectedKeys })
            });
            const data = await resp.json();
            if (data.status === 'ok') {
                showToast(t('skills.vault_save_success'), 'success');
                closeVaultKeyModal();
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }

    // ── Internal Tools Assignment Modal ─────────────────────────────────────────

    let internalToolsTargetId = '';

    // eslint-disable-next-line no-unused-vars
    async function openInternalToolsModal(id) {
        internalToolsTargetId = id;
        const listEl = document.getElementById('internal-tools-list');
        const bridgeOffEl = document.getElementById('internal-tools-bridge-off');
        listEl.innerHTML = `<p>${t('common.loading')}</p>`;
        if (bridgeOffEl) bridgeOffEl.style.display = 'none';
        document.getElementById('internal-tools-modal').classList.add('active');

        try {
            const [toolsResp, skillResp] = await Promise.all([
                fetch('/api/skills/available-tools'),
                fetch(`/api/skills/${encodeURIComponent(id)}`)
            ]);
            const toolsData = await toolsResp.json();
            const skillData = await skillResp.json();

            const availableTools = toolsData.tools || [];
            const currentTools = skillData.skill ? (skillData.skill.InternalTools || skillData.skill.internal_tools || []) : [];

            if (availableTools.length === 0) {
                listEl.innerHTML = '';
                if (bridgeOffEl) bridgeOffEl.style.display = '';
                return;
            }

            listEl.innerHTML = availableTools.map(tool => {
                const checked = currentTools.includes(tool) ? 'checked' : '';
                return `<label class="sk-vault-checkbox-row">
                    <input type="checkbox" class="sk-internal-tool-checkbox" value="${esc(tool)}" ${checked}>
                    <code class="sk-dep-tag sk-internal-tool-tag">${esc(tool)}</code>
                </label>`;
            }).join('');
        } catch (e) {
            listEl.innerHTML = `<p class="sk-error">${t('common.error')}</p>`;
        }
    }

    // eslint-disable-next-line no-unused-vars
    function closeInternalToolsModal() {
        document.getElementById('internal-tools-modal').classList.remove('active');
        internalToolsTargetId = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function saveInternalTools() {
        if (!internalToolsTargetId) return;
        const checkboxes = document.querySelectorAll('#internal-tools-list .sk-internal-tool-checkbox:checked');
        const selectedTools = Array.from(checkboxes).map(cb => cb.value);

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(internalToolsTargetId)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ internal_tools: selectedTools })
            });
            const data = await resp.json();
            if (data.status === 'ok') {
                showToast(t('skills.internal_tools_save_success'), 'success');
                closeInternalToolsModal();
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error'), 'error');
        }
    }
