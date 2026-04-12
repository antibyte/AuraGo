/* AuraGo – Skills Manager page JS */
/* global I18N, t, applyI18n, esc */
'use strict';

let allSkills = [];
let allTemplates = [];
let daemonStates = {};  // skill_id -> daemon state object
let daemonSystemEnabled = false;  // true when daemon_skills.enabled in config
let currentTypeFilter = 'all';
let currentSecFilter = 'all';
let currentDetailId = '';
let deleteTargetId = '';
let selectedFile = null;
let selectedImportFile = null;
let codeEditorView = null;
let codeEditorSkillId = '';
let codeEditorDraft = null;
let vaultKeyTargetId = '';
let allVaultSecrets = [];
let credentialMap = {}; // id -> name
let detailVersions = [];
let detailAudit = [];
let searchDebounceHandle = null;

// ── Initialization ──────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', async () => {
    await loadCredentialMap();
    loadSkills();
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
            return `<span class="sk-daemon-badge sk-daemon-stopped" title="${t('skills.daemon_stopped') || 'Daemon: Stopped'}">⏹ ${t('skills.daemon') || 'Daemon'}</span>`;
        }
        const status = (ds.status || ds.Status || 'stopped').toLowerCase();
        const statusMap = {
            running: { cls: 'sk-daemon-running', icon: '🟢', label: t('skills.daemon_running') || 'Running' },
            stopped: { cls: 'sk-daemon-stopped', icon: '⏹', label: t('skills.daemon_stopped') || 'Stopped' },
            error: { cls: 'sk-daemon-error', icon: '🔴', label: t('skills.daemon_error') || 'Error' },
            disabled: { cls: 'sk-daemon-disabled', icon: '⛔', label: t('skills.daemon_disabled') || 'Disabled' },
            starting: { cls: 'sk-daemon-starting', icon: '🟡', label: t('skills.daemon_starting') || 'Starting' },
        };
        const info = statusMap[status] || statusMap.stopped;
        return `<span class="sk-daemon-badge ${info.cls}" title="${info.label}">${info.icon} ${t('skills.daemon') || 'Daemon'}</span>`;
    }

    function renderDaemonActions(skill) {
        const isDaemon = skill.IsDaemon || skill.is_daemon;
        if (!isDaemon) return '';
        if (!daemonSystemEnabled) {
            return `<div class="sk-daemon-actions"><span class="sk-daemon-badge sk-daemon-disabled" title="${t('skills.daemon_disabled_hint') || 'Enable daemon_skills in config to start'}">⛔ ${t('skills.daemon') || 'Daemon'}</span></div>`;
        }
        const id = skill.name || skill.Name || skill.id || skill.ID || '';
        const ds = getDaemonState(id);
        const status = ds ? (ds.status || ds.Status || 'stopped').toLowerCase() : 'stopped';
        const autoDisabled = ds && (ds.auto_disabled || ds.AutoDisabled);

        let btns = '';
        if (status === 'running') {
            btns = `<button class="btn btn-sm btn-warning" onclick="daemonAction('${id}','stop')">${t('skills.daemon_stop') || 'Stop'}</button>`;
        } else if (autoDisabled || status === 'disabled') {
            btns = `<button class="btn btn-sm btn-primary" onclick="daemonAction('${id}','reenable')">${t('skills.daemon_reenable') || 'Re-enable'}</button>`;
        } else {
            btns = `<button class="btn btn-sm btn-primary" onclick="daemonAction('${id}','start')">${t('skills.daemon_start') || 'Start'}</button>`;
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
                showToast(`${t('skills.daemon_action_failed') || 'Daemon error'}: HTTP ${resp.status}`, 'error');
                return;
            }
            if (data.status === 'ok' && resp.ok) {
                showToast(t('skills.daemon_action_ok') || 'Daemon action executed', 'success');
                await loadSkills();
            } else {
                const msg = data.message || data.error || 'Unknown error';
                console.error('Daemon action error:', msg);
                showToast(`${t('skills.daemon_action_failed') || 'Daemon error'}: ${msg}`, 'error');
            }
        } catch (e) {
            console.error('Daemon action network error:', e);
            showToast(`${t('skills.daemon_action_failed') || 'Daemon error'}: ${e.message || 'Network error'}`, 'error');
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
            <h4 class="sk-daemon-settings-title">👹 ${t('skills.daemon_settings_title') || 'Daemon Settings'}</h4>
            <div class="sk-detail-grid">
                <div class="sk-detail-row">
                    <span class="sk-detail-label">${t('skills.daemon_wake_agent') || 'Wake Agent'}:</span>
                    <label class="toggle-wrap compact">
                        <div class="toggle${wakeAgent ? ' on' : ''}" id="daemon-wake-toggle" onclick="this.classList.toggle('on')"></div>
                        <span class="toggle-label">${wakeAgent ? (t('config.toggle.active') || 'Active') : (t('config.toggle.inactive') || 'Inactive')}</span>
                    </label>
                </div>
                <div class="sk-detail-row">
                    <span class="sk-detail-label">${t('skills.daemon_trigger_mission') || 'Trigger Mission'}:</span>
                    <select id="daemon-mission-select" class="field-input sk-daemon-select">
                        <option value="">— ${t('skills.daemon_no_mission') || 'No mission'} —</option>
                        ${missionOpts}
                    </select>
                </div>
                <div class="sk-detail-row">
                    <span class="sk-detail-label">${t('skills.daemon_cheatsheet') || 'Cheat Sheet'}:</span>
                    <select id="daemon-cheatsheet-select" class="field-input sk-daemon-select">
                        <option value="">— ${t('skills.daemon_no_cheatsheet') || 'No cheat sheet'} —</option>
                        ${csOpts}
                    </select>
                </div>
            </div>
            <div class="sk-daemon-settings-help">${t('skills.daemon_settings_help') || 'When the daemon fires an event, the selected mission is triggered and the cheat sheet content is passed as working instructions.'}</div>
            <div class="sk-daemon-settings-actions">
                <button class="btn btn-sm btn-primary" onclick="saveDaemonSettings('${esc(skill.ID || skill.id || '')}')">${t('common.save') || 'Save'}</button>
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
                showToast(t('skills.daemon_settings_saved') || 'Daemon settings saved', 'success');
                showDetail(currentDetailId);
                loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error') || 'Error', 'error');
        }
    }

    function showDisabledState() {
        document.getElementById('sk-grid').style.display = 'none';
        document.getElementById('sk-empty').style.display = 'none';
        document.getElementById('sk-disabled').style.display = '';
        document.getElementById('sk-status-bar').style.display = 'none';
        document.getElementById('sk-toolbar-actions').style.display = 'none';
        document.getElementById('sk-security-filters').style.display = 'none';
    }

    // ── Stats ───────────────────────────────────────────────────────────────────

    function updateStats(stats) {
        if (!stats) return;
        document.getElementById('sk-total').textContent = stats.total || 0;
        document.getElementById('sk-agent').textContent = stats.agent || 0;
        document.getElementById('sk-user').textContent = stats.user || 0;
        document.getElementById('sk-pending').textContent = stats.pending || 0;
    }

    // ── Rendering ───────────────────────────────────────────────────────────────

    function renderSkills() {
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

        const typeLabel = type.charAt(0).toUpperCase() + type.slice(1);
        const secBadge = renderSecurityBadge(secStatus);
        const enabledClass = enabled ? 'sk-enabled' : 'sk-disabled-card';
        const toggleLabel = enabled ? (t('skills.btn_disable') || 'Disable') : (t('skills.btn_enable') || 'Enable');
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
            <button class="btn btn-xs btn-secondary sk-vault-edit-btn" onclick="openVaultKeyModal('${id}')" title="${t('skills.btn_edit_secrets') || 'Edit Secrets'}">🔑 ${t('skills.btn_edit_secrets') || 'Secrets'}</button>
            ${keyTags ? `<span class="sk-vault-keys">${keyTags}</span>` : ''}
        </div>`;
        }

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
        <div class="sk-card-desc">${desc || '<em>' + (t('skills.no_description') || 'No description') + '</em>'}</div>
        ${metaRow}
        ${depTags}
        ${vaultRow}
        ${renderDaemonActions(skill)}
        <div class="sk-card-actions">
            <button class="btn btn-sm btn-secondary" onclick="showDetail('${id}')" data-i18n="skills.btn_details">Details</button>
            <button class="btn btn-sm btn-secondary" onclick="viewCode('${id}')" data-i18n="skills.btn_view_code">Code</button>
            <button class="btn btn-sm ${toggleClass}" onclick="toggleSkill('${id}', ${!enabled})">${toggleLabel}</button>
            <button class="btn btn-sm btn-danger" onclick="showDeleteSkillModal('${id}', '${name}')" data-i18n="skills.btn_delete">Delete</button>
        </div>
    </div>`;
    }

    function renderSecurityBadge(status) {
        const labels = {
            clean: t('skills.sec_clean') || 'Clean',
            warning: t('skills.sec_warning') || 'Warning',
            dangerous: t('skills.sec_dangerous') || 'Dangerous',
            pending: t('skills.sec_pending') || 'Pending',
            error: t('skills.sec_error') || 'Error'
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

    // eslint-disable-next-line no-unused-vars
    function filterSkills() {
        window.clearTimeout(searchDebounceHandle);
        searchDebounceHandle = window.setTimeout(() => renderSkills(), 120);
    }

    // eslint-disable-next-line no-unused-vars
    function setSkillFilter(filter) {
        currentTypeFilter = filter;
        document.querySelectorAll('.sk-filter-btn').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.filter === filter);
        });
        renderSkills();
    }

    // eslint-disable-next-line no-unused-vars
    function setSecurityFilter(filter) {
        currentSecFilter = filter;
        document.querySelectorAll('.sk-pill').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.sec === filter);
        });
        renderSkills();
    }

    // ── Skill Actions ───────────────────────────────────────────────────────────

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
                showToast(t('skills.toggle_success') || 'Skill updated', 'success');
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error') || 'Error', 'error');
        }
    }

    // ── Detail Modal ────────────────────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    async function showDetail(id) {
        currentDetailId = id;
        const body = document.getElementById('detail-modal-body');
        body.innerHTML = `<p>${t('common.loading') || 'Loading...'}</p>`;
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
            const tags = (s.Tags || s.tags || []);
            const category = (s.Category || s.category || '');

            let secHTML = '';
            if (sec) {
                const findings = sec.StaticAnalysis || sec.static_analysis || [];
                if (findings.length > 0) {
                    secHTML = `<div class="sk-findings">
                    <h4 data-i18n="skills.findings_title">${t('skills.findings_title') || 'Security Findings'}</h4>
                    <ul>${findings.map(f => `<li class="sk-finding sk-finding-${(f.Severity || f.severity || 'info').toLowerCase()}">
                        <strong>${esc(f.Category || f.category || '')}</strong>: ${esc(f.Message || f.message || '')}
                        ${f.Line || f.line ? ` <span class="sk-finding-line">(${t('skills.finding_line') || 'line'} ${f.Line || f.line})</span>` : ''}
                    </li>`).join('')}</ul>
                </div>`;
                }
                if (sec.GuardianDecision || sec.guardian_decision) {
                    secHTML += `<div class="sk-guardian-result">
                    <h4>${t('skills.guardian_title') || 'LLM Guardian'}</h4>
                    <p><strong>${t('skills.guardian_decision') || 'Decision'}:</strong> ${esc(sec.GuardianDecision || sec.guardian_decision)}</p>
                    ${sec.GuardianReason || sec.guardian_reason ? `<p>${esc(sec.GuardianReason || sec.guardian_reason)}</p>` : ''}
                </div>`;
                }
            }

            body.innerHTML = `
            <div class="sk-detail-grid">
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_name') || 'Name'}:</span> <span>${esc(s.Name || s.name)}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_type') || 'Type'}:</span> <span class="sk-type-badge sk-type-${(s.Type || s.type || '').toLowerCase()}">${esc(s.Type || s.type)}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_status') || 'Security'}:</span> ${renderSecurityBadge((s.SecurityStatus || s.security_status || 'pending').toLowerCase())}</div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_enabled') || 'Enabled'}:</span> <span>${s.Enabled || s.enabled ? '✅' : '❌'}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_created') || 'Created'}:</span> <span>${esc(s.CreatedAt || s.created_at || '-')}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_description') || 'Description'}:</span> <span>${esc(s.Description || s.description || '-')}</span></div>
                ${category ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.field_category') || 'Category'}:</span> <span>${esc(category)}</span></div>` : ''}
                ${tags.length > 0 ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.field_tags') || 'Tags'}:</span> <span class="sk-meta-list">${tags.map(tag => `<span class="sk-dep-tag">${esc(tag)}</span>`).join('')}</span></div>` : ''}
                ${deps.length > 0 ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_deps') || 'Dependencies'}:</span> <span>${deps.map(d => `<span class="sk-dep-tag">${esc(d)}</span>`).join(' ')}</span></div>` : ''}
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.vault_keys_label') || 'Vault Keys'}:</span> <span>${vaultKeys.length > 0 ? vaultKeys.map(k => {
                if (k.startsWith('cred:')) {
                    const cname = credentialMap[k.slice(5)] || k.slice(5);
                    return `<code class="sk-vault-key-tag sk-vault-key-cred" title="${esc(k)}">🔑 ${esc(cname)}</code>`;
                }
                return `<code class="sk-vault-key-tag">${esc(k)}</code>`;
            }).join(' ') : `<span class="sk-vault-none">${t('skills.vault_none') || 'No secrets assigned'}</span>`}</span></div>
            </div>
            ${renderDaemonSettings(s, data.daemon)}
            ${renderSkillHistory(detailVersions)}
            ${renderSkillAudit(detailAudit)}
            ${secHTML}`;
            if (typeof applyI18n === 'function') applyI18n();
        } catch (e) {
            body.innerHTML = `<p class="sk-error">${t('common.error') || 'Error'}</p>`;
        }
    }

    // eslint-disable-next-line no-unused-vars
    function closeDetailModal() {
        document.getElementById('detail-modal').classList.remove('active');
        currentDetailId = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function verifyCurrentSkill() {
        if (!currentDetailId) return;
        const btn = document.getElementById('detail-verify-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading') || 'Scanning...';

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(currentDetailId)}/verify`, { method: 'POST' });
            const data = await resp.json();
            if (data.status === 'scanned') {
                showToast(t('skills.scan_complete') || 'Scan complete', 'success');
                showDetail(currentDetailId);
                loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error') || 'Error', 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_verify') || 'Re-Scan';
        }
    }

    // ── Code Editor Modal (CodeMirror 6) ────────────────────────────────────────

    function createEditorState(code, readOnly) {
        const cm = window.CM6;
        if (!cm || !cm.EditorState || !cm.EditorView) {
            throw new Error('CodeMirror unavailable');
        }
        const extensions = [
            cm.lineNumbers(),
            cm.highlightActiveLineGutter(),
            cm.highlightSpecialChars(),
            cm.history(),
            cm.foldGutter(),
            cm.drawSelection(),
            cm.dropCursor(),
            cm.indentOnInput(),
            cm.syntaxHighlighting(cm.defaultHighlightStyle, { fallback: true }),
            cm.bracketMatching(),
            cm.closeBrackets(),
            cm.autocompletion(),
            cm.rectangularSelection(),
            cm.crosshairCursor(),
            cm.highlightActiveLine(),
            cm.highlightSelectionMatches(),
            cm.keymap.of([
                ...cm.closeBracketsKeymap,
                ...cm.defaultKeymap,
                ...cm.searchKeymap,
                ...cm.historyKeymap,
                ...cm.foldKeymap,
                ...cm.completionKeymap,
                cm.indentWithTab,
            ]),
            cm.python(),
            cm.oneDark,
        ];
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
            saveBtn.textContent = t('skills.btn_save_code') || 'Save';
            return;
        }
        metaWrap.style.display = '';
        document.getElementById('code-draft-name').value = meta.name || '';
        document.getElementById('code-draft-description').value = meta.description || '';
        document.getElementById('code-draft-category').value = meta.category || '';
        document.getElementById('code-draft-tags').value = (meta.tags || []).join(', ');
        saveBtn.textContent = readOnly ? (t('common.btn_close') || 'Close') : (t('skills.btn_create_skill') || 'Create Skill');
    }

    // eslint-disable-next-line no-unused-vars
    function openCodeEditor(id, code, readOnly, draftMeta = null) {
        codeEditorSkillId = id;
        const container = document.getElementById('code-editor-container');
        container.innerHTML = '';

        if (codeEditorView) {
            codeEditorView.destroy();
            codeEditorView = null;
        }

        try {
            const state = createEditorState(code || '', readOnly);
            codeEditorView = new window.CM6.EditorView({
                state,
                parent: container,
            });
        } catch (_) {
            container.innerHTML = `<textarea id="code-editor-fallback" class="sk-editor-fallback" ${readOnly ? 'readonly' : ''}></textarea>`;
            document.getElementById('code-editor-fallback').value = code || '';
            showToast(t('skills.editor_fallback') || 'Advanced editor unavailable, using plain text mode.', 'info');
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
        container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('common.loading') || 'Loading...'}</p>`;

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(id)}?code=true`);
            const data = await resp.json();
            if (data.status === 'ok' && data.code) {
                container.innerHTML = '';
                openCodeEditor(id, data.code, false, null);
            } else {
                container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('skills.no_code') || 'Code not available'}</p>`;
            }
        } catch (e) {
            container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('skills.failed_to_load_code') || 'Failed to load code'}</p>`;
        }
    }

    // eslint-disable-next-line no-unused-vars
    async function saveSkillCode() {
        const code = getCurrentEditorCode();
        if (!code) return;
        const btn = document.getElementById('code-save-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading') || 'Saving...';

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
                resp = await fetch('/api/skills', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name, description, category, tags, code })
                });
            }
            const data = await resp.json();
            if (data.status === 'ok' || data.status === 'created') {
                showToast(codeEditorSkillId ? (t('skills.code_saved') || 'Code saved') : (t('skills.create_success') || 'Skill created'), 'success');
                await loadSkills();
                closeCodeModal();
                if (!codeEditorSkillId && data.skill && data.skill.id) {
                    currentDetailId = data.skill.id;
                }
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error') || 'Error', 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_save_code') || 'Save';
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
            showToast(t('skills.upload_py_only') || 'Only .py files are allowed', 'error');
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
        btn.textContent = t('common.loading') || 'Uploading...';

        const fd = new FormData();
        fd.append('file', selectedFile);
        fd.append('name', document.getElementById('upload-name').value.trim());
        fd.append('description', document.getElementById('upload-description').value.trim());
        fd.append('category', document.getElementById('upload-category').value.trim());
        fd.append('tags', document.getElementById('upload-tags').value.trim());

        try {
            const resp = await fetch('/api/skills/upload', { method: 'POST', body: fd });
            const data = await resp.json();

            if (data.status === 'uploaded' || data.status === 'created') {
                showToast(t('skills.upload_success') || 'Skill uploaded', 'success');
                closeUploadModal();
                await loadSkills();
            } else if (data.status === 'rejected') {
                const msgs = (data.validation && data.validation.Errors) ? data.validation.Errors.join(', ') : (data.message || 'Validation failed');
                showToast(msgs, 'error');
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error') || 'Error', 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_upload') || 'Upload';
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

        if (!templateName || !skillName) {
            showToast(t('skills.template_required') || 'Template and skill name are required', 'error');
            return;
        }

        const btn = document.getElementById('template-submit-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading') || 'Creating...';

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
                    dependencies: dependencies
                })
            });
            const data = await resp.json();

            if (data.status === 'created') {
                showToast(t('skills.template_success') || 'Skill created from template', 'success');
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
            showToast(t('common.error') || 'Error', 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_create') || 'Create';
        }
    }

    // ── Delete Modal ────────────────────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    function showDeleteSkillModal(id, name) {
        deleteTargetId = id;
        document.getElementById('delete-skill-name').textContent = name;
        document.getElementById('delete-files-checkbox').checked = true;
        document.getElementById('delete-skill-modal').classList.add('active');
    }

    // eslint-disable-next-line no-unused-vars
    function closeDeleteSkillModal() {
        document.getElementById('delete-skill-modal').classList.remove('active');
        deleteTargetId = '';
    }

    // eslint-disable-next-line no-unused-vars
    async function confirmDeleteSkill() {
        if (!deleteTargetId) return;
        const deleteFiles = document.getElementById('delete-files-checkbox').checked;

        try {
            const resp = await fetch(`/api/skills/${encodeURIComponent(deleteTargetId)}?delete_files=${deleteFiles}`, { method: 'DELETE' });
            const data = await resp.json();
            if (data.status === 'deleted') {
                showToast(t('skills.delete_success') || 'Skill deleted', 'success');
                closeDeleteSkillModal();
                closeDetailModal();
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error') || 'Error', 'error');
        }
    }

    function renderSkillHistory(versions) {
        if (!Array.isArray(versions) || versions.length === 0) return '';
        return `<div class="sk-findings">
        <h4>${t('skills.history_title') || 'Version History'}</h4>
        <ul class="sk-history-list">
            ${versions.slice(0, 8).map(v => `<li class="sk-history-item">
                <div class="sk-history-item-head">
                    <span>${t('skills.history_version') || 'Version'} ${esc(String(v.version || v.Version || '?'))}</span>
                    <button class="btn btn-sm btn-secondary" onclick="restoreSkillVersion('${esc(currentDetailId)}', ${Number(v.version || v.Version || 0)})">${t('skills.btn_restore') || 'Restore'}</button>
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
        <h4>${t('skills.audit_title') || 'Audit Trail'}</h4>
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
                showToast(t('skills.restore_success') || 'Version restored', 'success');
                await showDetail(id);
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (_) {
            showToast(t('common.error') || 'Error', 'error');
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
        btn.textContent = t('common.loading') || 'Running...';
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
                showToast(t('skills.test_success') || 'Test finished', 'success');
            } else {
                showToast(data.message || (t('skills.test_failed') || 'Test failed'), 'error');
            }
        } catch (e) {
            statusEl.textContent = 'error';
            outputEl.textContent = e.message || 'Invalid JSON';
            showToast(t('skills.test_invalid_json') || 'Input must be valid JSON', 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_run_test') || 'Run Test';
        }
    }

    // eslint-disable-next-line no-unused-vars
    function showImportModal() {
        selectedImportFile = null;
        document.getElementById('import-file').value = '';
        document.getElementById('import-file-name').textContent = t('skills.import_hint') || 'Choose an exported `.aurago-skill.json` bundle.';
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
        btn.textContent = t('common.loading') || 'Importing...';
        try {
            const bundle = JSON.parse(await selectedImportFile.text());
            const resp = await fetch('/api/skills/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(bundle)
            });
            const data = await resp.json();
            if (data.status === 'imported') {
                showToast(t('skills.import_success') || 'Skill imported', 'success');
                closeImportModal();
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(e.message || (t('common.error') || 'Error'), 'error');
        } finally {
            btn.disabled = false;
            btn.textContent = t('skills.btn_import') || 'Import';
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
            showToast(t('skills.generate_prompt_required') || 'Please describe the skill you want.', 'error');
            return;
        }
        const btn = document.getElementById('generate-submit-btn');
        btn.disabled = true;
        btn.textContent = t('common.loading') || 'Generating...';
        setGenerateStatus(t('skills.generate_in_progress') || 'Creating AI draft. This can take a few moments.', 'info');
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
                const message = getResponseError(data, t('common.error') || 'Error');
                setGenerateStatus(message, 'error');
                showToast(message, 'error');
                return;
            }
            setGenerateStatus(t('skills.generate_success') || 'Draft generated', 'success');
            closeGenerateModal();
            openCodeEditor('', data.draft.code || '', false, {
                name: data.draft.name || '',
                description: data.draft.description || '',
                category: data.draft.category || '',
                tags: data.draft.tags || []
            });
            showToast(t('skills.generate_success') || 'Draft generated', 'success');
        } catch (e) {
            const message = e && e.name === 'AbortError'
                ? (t('skills.generate_timeout') || 'The AI draft took too long. Please try again or shorten the prompt.')
                : (e.message || (t('common.error') || 'Error'));
            setGenerateStatus(message, 'error');
            showToast(message, 'error');
        } finally {
            window.clearTimeout(timeoutHandle);
            btn.disabled = false;
            btn.textContent = t('skills.btn_generate') || 'Generate';
        }
    }

    function parseCSV(raw) {
        return (raw || '')
            .split(',')
            .map(part => part.trim())
            .filter(Boolean);
    }

    // ── Toast helper ────────────────────────────────────────────────────────────

    function showToast(msg, type) {
        let c = document.getElementById('sk-toast-container');
        if (!c) {
            c = document.createElement('div');
            c.id = 'sk-toast-container';
            c.className = 'sk-toast-container';
            document.body.appendChild(c);
        }
        const el = document.createElement('div');
        el.className = `sk-toast sk-toast-${type || 'info'}`;
        el.textContent = msg;
        c.appendChild(el);
        setTimeout(() => { el.classList.add('sk-toast-out'); setTimeout(() => el.remove(), 300); }, 3500);
    }

    // ── Vault Key Assignment Modal ───────────────────────────────────────────────

    // eslint-disable-next-line no-unused-vars
    async function openVaultKeyModal(id) {
        vaultKeyTargetId = id;
        const listEl = document.getElementById('vault-key-list');
        const emptyEl = document.getElementById('vault-key-empty');
        listEl.innerHTML = `<p>${t('common.loading') || 'Loading...'}</p>`;
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
                html += `<p class="sk-vault-section-label">${t('skills.vault_section_secrets') || 'Vault Secrets'}</p>`;
                html += allVaultSecrets.map(key => {
                    const checked = currentKeys.includes(key) ? 'checked' : '';
                    return `<label class="sk-vault-checkbox-row">
                    <input type="checkbox" class="sk-vault-checkbox" value="${esc(key)}" ${checked}>
                    <code class="sk-vault-key-tag">${esc(key)}</code>
                </label>`;
                }).join('');
            }

            if (credList.length > 0) {
                html += `<p class="sk-vault-section-label" style="margin-top:10px">${t('skills.vault_section_creds') || 'Credentials'}</p>`;
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
            listEl.innerHTML = `<p class="sk-error">${t('common.error') || 'Error loading secrets'}</p>`;
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
                showToast(t('skills.vault_save_success') || 'Secrets updated', 'success');
                closeVaultKeyModal();
                await loadSkills();
            } else {
                showToast(data.message || t('common.error'), 'error');
            }
        } catch (e) {
            showToast(t('common.error') || 'Error', 'error');
        }
    }
