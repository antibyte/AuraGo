// AuraGo – invasion_control page logic
// Extracted from invasion_control.html

let currentTab = 'nests';
let nestsData = [];
let eggsData = [];
let providersData = [];
let deleteTarget = null; // { type: 'nest'|'egg', id, name }
let configHistoryData = [];

// ── Init ─────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    if (typeof window._auragoApplySharedI18n === 'function') {
        window._auragoApplySharedI18n();
    }
    bindInvasionUI();
    loadProviders();
    loadNests();
    loadEggs();
});

function bindInvasionUI() {
    document.getElementById('btn-create')?.addEventListener('click', openCreateModal);
    document.getElementById('nest-access-type')?.addEventListener('change', onAccessTypeChange);
    document.getElementById('nest-deploy-method')?.addEventListener('change', onDeployMethodChange);
    document.getElementById('btn-validate')?.addEventListener('click', validateNest);
    document.getElementById('nest-save-btn')?.addEventListener('click', saveNest);
    document.getElementById('nest-cancel-btn')?.addEventListener('click', () => closeModal('nest-modal'));
    document.getElementById('egg-save-btn')?.addEventListener('click', saveEgg);
    document.getElementById('egg-cancel-btn')?.addEventListener('click', () => closeModal('egg-modal'));
    document.getElementById('delete-cancel-btn')?.addEventListener('click', () => closeModal('delete-modal'));
    document.getElementById('btn-delete-confirm')?.addEventListener('click', confirmDelete);
    document.getElementById('delete-confirm-input')?.addEventListener('input', checkDeleteConfirm);
    document.getElementById('reconfigure-cancel-btn')?.addEventListener('click', () => closeModal('reconfigure-modal'));
    document.getElementById('reconfigure-apply-btn')?.addEventListener('click', applyReconfigure);
    document.getElementById('config-history-close-btn')?.addEventListener('click', () => closeModal('config-history-modal'));

    document.addEventListener('click', (event) => {
        const tabBtn = event.target.closest('.invasion-tab[data-tab]');
        if (tabBtn) {
            switchTab(tabBtn.dataset.tab);
            return;
        }

        const closeBtn = event.target.closest('[data-modal-close]');
        if (closeBtn) {
            closeModal(closeBtn.dataset.modalClose);
            return;
        }

        const actionBtn = event.target.closest('[data-action]');
        if (!actionBtn) return;

        const { action, id, active, type, name, revision } = actionBtn.dataset;
        switch (action) {
            case 'open-create':
                openCreateModal();
                break;
            case 'edit-nest':
                editNest(id);
                break;
            case 'edit-egg':
                editEgg(id);
                break;
            case 'hatch-nest':
                hatchNest(id);
                break;
            case 'stop-nest':
                stopNest(id);
                break;
            case 'toggle-nest':
                toggleNest(id, active === 'true');
                break;
            case 'toggle-egg':
                toggleEgg(id, active === 'true');
                break;
            case 'request-delete':
                requestDelete(type, id, name);
                break;
            case 'reconfigure-nest':
                openReconfigureModal(id);
                break;
            case 'config-history-nest':
                openConfigHistory(id);
                break;
            case 'rollback-config':
                rollbackConfig(id, revision);
                break;
        }
    });
}

async function loadProviders() {
    try {
        const data = await fetch('/api/providers').then(r => r.ok ? r.json() : []);
        providersData = Array.isArray(data) ? data : [];
        const sel = document.getElementById('egg-provider');
        if (!sel) return;
        // set placeholder text on the first empty option
        sel.options[0].textContent = '— ' + t('invasion.inherit_llm') + ' —';
        // remove any previously populated provider options
        while (sel.options.length > 1) sel.remove(1);
        providersData.forEach(p => {
            const opt = document.createElement('option');
            opt.value = p.id;
            opt.textContent = p.name || p.id;
            sel.appendChild(opt);
        });
    } catch (e) {
        console.warn('[IC] Could not load providers:', e.message);
    }
}

// ── Tabs ─────────────────────────────────────────────────
function switchTab(tab) {
    currentTab = tab;
    document.querySelectorAll('.invasion-tab').forEach(t => t.classList.toggle('active', t.dataset.tab === tab));
    document.getElementById('content-nests').classList.toggle('is-hidden', tab !== 'nests');
    document.getElementById('content-eggs').classList.toggle('is-hidden', tab !== 'eggs');
}

// ── API ──────────────────────────────────────────────────
async function api(path, options = {}) {
    const resp = await fetch('/api/invasion/' + path, {
        headers: { 'Content-Type': 'application/json' },
        ...options
    });
    if (!resp.ok) {
        const err = await resp.json().catch(() => ({ error: resp.statusText }));
        throw new Error(err.error || resp.statusText);
    }
    return resp.json();
}

// ── Load Data ────────────────────────────────────────────
async function loadNests() {
    try {
        nestsData = (await api('nests')) || [];
        renderNests();
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

async function loadEggs() {
    try {
        eggsData = (await api('eggs')) || [];
        renderEggs();
        renderNests();
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

// ── Render ───────────────────────────────────────────────
function renderNests() {
    const grid = document.getElementById('nests-grid');
    const empty = document.getElementById('nests-empty');
    if (!nestsData || nestsData.length === 0) {
        grid.innerHTML = '';
        empty.classList.remove('is-hidden');
        return;
    }
    empty.classList.add('is-hidden');
    grid.innerHTML = nestsData.map(n => {
        const eggName = n.egg_id ? (n.egg_name || eggsData.find(e => e.id === n.egg_id)?.name || n.egg_id) : '—';
        const hs = n.hatch_status || 'idle';
        const showHatchBadge = !(hs === 'running' && !n.ws_connected);
        const hsBadge = showHatchBadge ? `<span class="badge badge-${hs}">${t('invasion.hatch_' + hs)}</span>` : '';
        const wsStatus = n.ws_connected
            ? `<span class="badge badge-connected">${t('invasion.ws_connected')}</span>`
            : (hs === 'running' ? `<span class="badge badge-disconnected">${t('invasion.ws_disconnected')}</span>` : '');
        const canHatch = n.egg_id && n.active && (hs === 'idle' || hs === 'failed' || hs === 'stopped');
        const canStop = hs === 'running' || hs === 'hatching';
        const hasEgg = !!n.egg_id;
        // Config drift indicator
        const hasDrift = n.desired_config_rev && n.applied_config_rev && n.desired_config_rev !== n.applied_config_rev;
        const driftBadge = hasDrift
            ? `<span class="badge badge-drift" title="${t('invasion.config_drift_tooltip')}">🔄 ${t('invasion.config_drift')}</span>`
            : (n.applied_config_rev ? `<span class="badge badge-synced">✅ ${t('invasion.config_synced')}</span>` : '');
        const statusBadges = [hsBadge, wsStatus, driftBadge].filter(Boolean).join(' ');
        let telBadge = '';
        if (n.telemetry) {
            const cpu = Math.round(n.telemetry.cpu_percent || 0);
            const mem = Math.round(n.telemetry.mem_percent || 0);
            const hrs = Math.floor((n.telemetry.uptime_seconds || 0) / 3600);
            telBadge = `<div class="inv-telemetry-row">
                    <span>📊 ${t('invasion.telemetry_cpu')}: ${cpu}%</span>
                    <span>🧠 ${t('invasion.telemetry_mem')}: ${mem}%</span>
                    <span>⏱ ${hrs}h</span>
                </div>`;
        }
        return `
            <div class="card">
                <div class="card-header">
                    <div class="card-title">${esc(n.name)}</div>
                    <div>
                        <span class="badge ${n.active ? 'badge-active' : 'badge-inactive'}">
                            ${n.active ? t('invasion.active') : t('invasion.inactive')}
                        </span>
                        ${statusBadges}
                    </div>
                </div>
                <div class="card-meta">
                    <span><span class="badge badge-${n.access_type}">${n.access_type.toUpperCase()}</span>
                          ${n.access_type !== 'local' ? esc(n.host) + ':' + n.port : 'localhost'}</span>
                    ${n.username ? '<span>👤 ' + esc(n.username) + '</span>' : ''}
                    <span>🥚 ${esc(eggName)}</span>
                    ${n.deploy_method ? '<span>🚀 ' + esc(n.deploy_method) + '</span>' : ''}
                    ${n.route ? '<span>🔗 ' + esc(n.route) + '</span>' : ''}
                    ${n.target_arch ? '<span>💻 ' + esc(n.target_arch) + '</span>' : ''}
                    ${n.hatch_error ? '<span class="inv-hatch-error">⚠️ ' + esc(n.hatch_error) + '</span>' : ''}
                    ${n.notes ? '<span class="inv-note-text">📝 ' + esc(n.notes) + '</span>' : ''}
                    ${telBadge}
                </div>
                <div class="card-actions">
                    <button class="btn btn-sm btn-secondary" data-action="edit-nest" data-id="${escAttr(n.id)}">✏️ ${t('invasion.edit')}</button>
                    ${canHatch ? `<button class="btn btn-sm btn-primary" data-action="hatch-nest" data-id="${escAttr(n.id)}">${t('invasion.hatch')}</button>` : ''}
                    ${canStop ? `<button class="btn btn-sm btn-danger" data-action="stop-nest" data-id="${escAttr(n.id)}">${t('invasion.stop_egg')}</button>` : ''}
                    ${hasEgg ? `<button class="btn btn-sm btn-secondary" data-action="reconfigure-nest" data-id="${escAttr(n.id)}">🔧 ${t('invasion.reconfigure')}</button>` : ''}
                    ${hasEgg ? `<button class="btn btn-sm btn-secondary" data-action="config-history-nest" data-id="${escAttr(n.id)}">📋 ${t('invasion.config_history')}</button>` : ''}
                    <button class="btn btn-sm btn-secondary" data-action="toggle-nest" data-id="${escAttr(n.id)}" data-active="${String(!n.active)}">
                        ${n.active ? '⏸️' : '▶️'} ${n.active ? t('invasion.inactive') : t('invasion.active')}
                    </button>
                    <button class="btn btn-sm btn-danger" data-action="request-delete" data-type="nest" data-id="${escAttr(n.id)}" data-name="${escAttr(n.name)}">🗑️</button>
                </div>
            </div>`;
    }).join('');
}

function renderEggs() {
    const grid = document.getElementById('eggs-grid');
    const empty = document.getElementById('eggs-empty');
    if (!eggsData || eggsData.length === 0) {
        grid.innerHTML = '';
        empty.classList.remove('is-hidden');
        return;
    }
    empty.classList.add('is-hidden');
    grid.innerHTML = eggsData.map(e => `
            <div class="card">
                <div class="card-header">
                    <div class="card-title">${esc(e.name)}</div>
                    <span class="badge ${e.active ? 'badge-active' : 'badge-inactive'}">
                        ${e.active ? t('invasion.active') : t('invasion.inactive')}
                    </span>
                </div>
                <div class="card-meta">
                    ${e.description ? '<span>📋 ' + esc(e.description) + '</span>' : ''}
                    ${e.model ? '<span>🤖 ' + esc(e.provider ? e.provider + '/' : '') + esc(e.model) + '</span>' : ''}
                    <span>🔑 ${t('invasion.api_key')}: ${e.has_api_key ? '✅' : '❌'}</span>
                    <span>🌐 ${t('invasion.port')}: ${e.egg_port || 8099}</span>
                    ${e.permanent ? '<span class="badge badge-permanent">' + t('invasion.badge_permanent') + '</span>' : ''}
                    ${e.inherit_llm ? '<span class="badge badge-inherit">' + t('invasion.badge_llm_inherit') + '</span>' : ''}
                    ${e.include_vault ? '<span class="badge badge-vault">' + t('invasion.badge_vault') + '</span>' : ''}
                </div>
                <div class="card-actions">
                    <button class="btn btn-sm btn-secondary" data-action="edit-egg" data-id="${escAttr(e.id)}">✏️ ${t('invasion.edit')}</button>
                    <button class="btn btn-sm btn-secondary" data-action="toggle-egg" data-id="${escAttr(e.id)}" data-active="${String(!e.active)}">
                        ${e.active ? '⏸️' : '▶️'} ${e.active ? t('invasion.inactive') : t('invasion.active')}
                    </button>
                    <button class="btn btn-sm btn-danger" data-action="request-delete" data-type="egg" data-id="${escAttr(e.id)}" data-name="${escAttr(e.name)}">🗑️</button>
                </div>
            </div>
        `).join('');
}

// ── Create / Edit ────────────────────────────────────────
function openCreateModal() {
    if (currentTab === 'nests') {
        openNestModal();
    } else {
        openEggModal();
    }
}

function openNestModal(nest = null) {
    try {
        const isEdit = !!nest;
        const setVal = (id, val) => { const el = document.getElementById(id); if (el) el.value = val; else console.warn('[IC] missing element:', id); };
        const setChk = (id, val) => { const el = document.getElementById(id); if (el) el.checked = val; else console.warn('[IC] missing element:', id); };
        const setTxt = (id, val) => { const el = document.getElementById(id); if (el) el.textContent = val; else console.warn('[IC] missing element:', id); };
        const setHidden = (id, hidden) => { const el = document.getElementById(id); if (el) el.classList.toggle('is-hidden', hidden); else console.warn('[IC] missing element:', id); };

        setTxt('nest-modal-title', t(isEdit ? 'invasion.edit_nest' : 'invasion.create_nest'));
        setVal('nest-id', nest?.id || '');
        setVal('nest-name', nest?.name || '');
        setVal('nest-notes', nest?.notes || '');
        setVal('nest-access-type', nest?.access_type || 'ssh');
        setVal('nest-host', nest?.host || '');
        setVal('nest-port', nest?.port || 22);
        setVal('nest-username', nest?.username || '');
        setVal('nest-secret', '');
        setChk('nest-active', nest?.active !== false);
        setHidden('nest-secret-hint', !(isEdit && nest?.has_secret));

        const eggSelect = document.getElementById('nest-egg-id');
        if (eggSelect) {
            eggSelect.innerHTML = `<option value="">${t('invasion.no_egg')}</option>` +
                (eggsData || []).map(e => `<option value="${e.id}" ${e.id === nest?.egg_id ? 'selected' : ''}>${esc(e.name)}</option>`).join('');
        }

        setVal('nest-deploy-method', nest?.deploy_method || 'ssh');
        setVal('nest-target-arch', nest?.target_arch || 'linux/amd64');
        setVal('nest-route', nest?.route || 'direct');
        setVal('nest-route-config', nest?.route_config || '');

        setHidden('nest-validate-area', !isEdit);
        const vr = document.getElementById('validate-result');
        if (vr) { vr.className = 'validate-result'; vr.textContent = ''; }

        onAccessTypeChange();
        onDeployMethodChange();
        openModal('nest-modal');
    } catch (err) {
        console.error('[IC] openNestModal error:', err);
        showToast((t('invasion.error') || t('common.error')) + ': ' + err.message, 'error');
    }
}

function openEggModal(egg = null) {
    const isEdit = !!egg;
    document.getElementById('egg-modal-title').textContent = t(isEdit ? 'invasion.edit_egg' : 'invasion.create_egg');
    document.getElementById('egg-id').value = egg?.id || '';
    document.getElementById('egg-name').value = egg?.name || '';
    document.getElementById('egg-description').value = egg?.description || '';
    document.getElementById('egg-provider').value = egg?.provider || '';
    document.getElementById('egg-model').value = egg?.model || '';
    document.getElementById('egg-base-url').value = egg?.base_url || '';
    document.getElementById('egg-api-key').value = '';
    document.getElementById('egg-active').checked = egg?.active !== false;
    document.getElementById('egg-apikey-hint').classList.toggle('is-hidden', !(isEdit && egg?.has_api_key));

    document.getElementById('egg-port').value = egg?.egg_port || 8099;
    document.getElementById('egg-allowed-tools').value = egg?.allowed_tools || '';
    document.getElementById('egg-permanent').checked = !!egg?.permanent;
    document.getElementById('egg-include-vault').checked = !!egg?.include_vault;
    document.getElementById('egg-inherit-llm').checked = egg?.inherit_llm !== false;

    openModal('egg-modal');
}

function editNest(id) {
    const nest = nestsData.find(n => n.id === id);
    if (nest) openNestModal(nest);
}

function editEgg(id) {
    const egg = eggsData.find(e => e.id === id);
    if (egg) openEggModal(egg);
}

// ── Access Type ──────────────────────────────────────────
function onDeployMethodChange() {
    const method = document.getElementById('nest-deploy-method').value;
    const hint = document.getElementById('deploy-docker-local-hint');
    if (hint) hint.classList.toggle('is-hidden', method !== 'docker_local');
}

function onAccessTypeChange() {
    const type = document.getElementById('nest-access-type').value;
    const remoteFields = document.getElementById('nest-remote-fields');
    if (type === 'local') {
        remoteFields.classList.add('is-hidden');
    } else {
        remoteFields.classList.remove('is-hidden');
        if (type === 'docker') {
            document.getElementById('nest-port').value = document.getElementById('nest-port').value == 22 ? 2375 : document.getElementById('nest-port').value;
        } else {
            document.getElementById('nest-port').value = document.getElementById('nest-port').value == 2375 ? 22 : document.getElementById('nest-port').value;
        }
    }
}

// ── Save ─────────────────────────────────────────────────
async function saveNest() {
    const id = document.getElementById('nest-id').value;
    const body = {
        name: document.getElementById('nest-name').value.trim(),
        notes: document.getElementById('nest-notes').value.trim(),
        access_type: document.getElementById('nest-access-type').value,
        host: document.getElementById('nest-host').value.trim(),
        port: parseInt(document.getElementById('nest-port').value) || 22,
        username: document.getElementById('nest-username').value.trim(),
        secret: document.getElementById('nest-secret').value,
        active: document.getElementById('nest-active').checked,
        egg_id: document.getElementById('nest-egg-id').value,
        deploy_method: document.getElementById('nest-deploy-method').value,
        target_arch: document.getElementById('nest-target-arch').value,
        route: document.getElementById('nest-route').value,
        route_config: document.getElementById('nest-route-config').value.trim(),
    };
    if (!body.name) { document.getElementById('nest-name').focus(); return; }

    try {
        if (id) {
            await api('nests/' + id, { method: 'PUT', body: JSON.stringify(body) });
        } else {
            await api('nests', { method: 'POST', body: JSON.stringify(body) });
        }
        closeModal('nest-modal');
        showToast(t('invasion.saved'), 'success');
        await loadNests();
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

async function saveEgg() {
    const id = document.getElementById('egg-id').value;
    const body = {
        name: document.getElementById('egg-name').value.trim(),
        description: document.getElementById('egg-description').value.trim(),
        model: document.getElementById('egg-model').value.trim(),
        provider: document.getElementById('egg-provider').value.trim(),
        base_url: document.getElementById('egg-base-url').value.trim(),
        api_key: document.getElementById('egg-api-key').value,
        active: document.getElementById('egg-active').checked,
        egg_port: parseInt(document.getElementById('egg-port').value) || 8099,
        allowed_tools: document.getElementById('egg-allowed-tools').value.trim(),
        permanent: document.getElementById('egg-permanent').checked,
        include_vault: document.getElementById('egg-include-vault').checked,
        inherit_llm: document.getElementById('egg-inherit-llm').checked,
    };
    if (!body.name) { document.getElementById('egg-name').focus(); return; }

    try {
        if (id) {
            await api('eggs/' + id, { method: 'PUT', body: JSON.stringify(body) });
        } else {
            await api('eggs', { method: 'POST', body: JSON.stringify(body) });
        }
        closeModal('egg-modal');
        showToast(t('invasion.saved'), 'success');
        await loadEggs();
        renderNests(); // re-render in case egg names changed
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

// ── Toggle ───────────────────────────────────────────────
async function toggleNest(id, active) {
    try {
        await api('nests/' + id + '/toggle', { method: 'POST', body: JSON.stringify({ active }) });
        await loadNests();
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

async function toggleEgg(id, active) {
    try {
        await api('eggs/' + id + '/toggle', { method: 'POST', body: JSON.stringify({ active }) });
        await loadEggs();
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

// ── Hatch / Stop ─────────────────────────────────────────
async function hatchNest(id) {
    try {
        await api('nests/' + id + '/hatch', { method: 'POST' });
        showToast(t('invasion.hatching_started'), 'success');
        // Start polling for status
        pollHatchStatus(id);
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

async function stopNest(id) {
    try {
        await api('nests/' + id + '/stop', { method: 'POST' });
        showToast(t('invasion.stop_signal_sent'), 'success');
        await loadNests();
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
}

function pollHatchStatus(id) {
    let attempts = 0;
    const poll = setInterval(async () => {
        attempts++;
        try {
            const resp = await api('nests/' + id + '/hatch/status');
            const st = resp.hatch_status;
            // Update the nest in local data
            const nest = nestsData.find(n => n.id === id);
            if (nest) {
                nest.hatch_status = st;
                nest.ws_connected = resp.ws_connected;
                renderNests();
            }
            if (st === 'running' || st === 'failed' || st === 'stopped' || attempts > 60) {
                clearInterval(poll);
                await loadNests();
            }
        } catch (e) {
            clearInterval(poll);
        }
    }, 3000);
}

// ── Validate Connection ──────────────────────────────────
async function validateNest() {
    const id = document.getElementById('nest-id').value;
    if (!id) { showToast(t('invasion.validate_first'), 'error'); return; }

    const btn = document.getElementById('btn-validate');
    const result = document.getElementById('validate-result');
    btn.disabled = true;
    btn.textContent = t('invasion.testing');
    result.className = 'validate-result';

    try {
        const resp = await api('nests/' + id + '/validate', { method: 'POST' });
        result.textContent = resp.message;
        result.className = 'validate-result ' + (resp.success ? 'success' : 'error');
    } catch (e) {
        result.textContent = e.message;
        result.className = 'validate-result error';
    } finally {
        btn.disabled = false;
        btn.textContent = t('invasion.test_connection');
    }
}

// ── Delete ───────────────────────────────────────────────
function requestDelete(type, id, name) {
    deleteTarget = { type, id, name };
    const msgKey = type === 'nest' ? 'invasion.will_delete_nest' : 'invasion.will_delete_egg';
    document.getElementById('delete-message').innerHTML =
        `<strong>"${esc(name)}"</strong> ${t(msgKey)}`;
    document.getElementById('delete-confirm-input').value = '';
    document.getElementById('btn-delete-confirm').disabled = true;
    openModal('delete-modal');
    document.getElementById('delete-confirm-input').focus();
}

function checkDeleteConfirm() {
    const input = document.getElementById('delete-confirm-input').value.trim();
    document.getElementById('btn-delete-confirm').disabled = input !== deleteTarget?.name;
}

async function confirmDelete() {
    if (!deleteTarget) return;
    const { type, id } = deleteTarget;
    try {
        await api(type + 's/' + id, { method: 'DELETE' });
        closeModal('delete-modal');
        showToast(t('invasion.deleted'), 'success');
        if (type === 'nest') await loadNests();
        else await loadEggs();
    } catch (e) { showToast(t('invasion.error') + ': ' + e.message, 'error'); }
    deleteTarget = null;
}

// ── Safe Reconfigure ─────────────────────────────────────
async function openReconfigureModal(nestId) {
    const nest = nestsData.find(n => n.id === nestId);
    if (!nest) return;

    document.getElementById('reconfigure-nest-id').value = nestId;
    document.getElementById('reconfigure-modal-title').textContent =
        t('invasion.reconfigure_title') + ' — ' + nest.name;

    // Populate provider dropdown
    const sel = document.getElementById('reconfigure-provider');
    sel.innerHTML = `<option value="">${t('invasion.reconfigure_keep_current')}</option>`;
    providersData.forEach(p => {
        const opt = document.createElement('option');
        opt.value = p.id;
        opt.textContent = p.name || p.id;
        sel.appendChild(opt);
    });

    // Reset all fields
    document.getElementById('reconfigure-provider').value = '';
    document.getElementById('reconfigure-model').value = '';
    document.getElementById('reconfigure-base-url').value = '';
    document.getElementById('reconfigure-allowed-tools').value = '';
    document.getElementById('reconfigure-allow-filesystem').checked = false;
    document.getElementById('reconfigure-allow-network').checked = false;
    document.getElementById('reconfigure-allow-remote-shell').checked = false;
    document.getElementById('reconfigure-allow-self-update').checked = false;

    openModal('reconfigure-modal');
}

async function applyReconfigure() {
    const nestId = document.getElementById('reconfigure-nest-id').value;
    if (!nestId) return;

    const allowedToolsRaw = document.getElementById('reconfigure-allowed-tools').value.trim();
    let allowedTools = null;
    if (allowedToolsRaw) {
        try {
            allowedTools = JSON.parse(allowedToolsRaw);
            if (!Array.isArray(allowedTools)) {
                showToast(t('invasion.reconfigure_tools_invalid'), 'error');
                return;
            }
        } catch (e) {
            showToast(t('invasion.reconfigure_tools_invalid'), 'error');
            return;
        }
    }

    const patch = {};
    const provider = document.getElementById('reconfigure-provider').value;
    const model = document.getElementById('reconfigure-model').value.trim();
    const baseUrl = document.getElementById('reconfigure-base-url').value.trim();

    if (provider) patch.provider = provider;
    if (model) patch.model = model;
    if (baseUrl) patch.base_url = baseUrl;
    if (allowedTools !== null) patch.allowed_tools = allowedTools;

    // Runtime flags — only include if at least one is checked
    const flags = {
        allow_filesystem_write: document.getElementById('reconfigure-allow-filesystem').checked,
        allow_network_requests: document.getElementById('reconfigure-allow-network').checked,
        allow_remote_shell: document.getElementById('reconfigure-allow-remote-shell').checked,
        allow_self_update: document.getElementById('reconfigure-allow-self-update').checked,
    };
    const anyFlag = Object.values(flags).some(v => v);
    if (anyFlag) {
        patch.allow_filesystem_write = flags.allow_filesystem_write;
        patch.allow_network_requests = flags.allow_network_requests;
        patch.allow_remote_shell = flags.allow_remote_shell;
        patch.allow_self_update = flags.allow_self_update;
    }

    if (Object.keys(patch).length === 0) {
        showToast(t('invasion.reconfigure_no_changes'), 'error');
        return;
    }

    const btn = document.getElementById('reconfigure-apply-btn');
    btn.disabled = true;
    btn.textContent = t('invasion.reconfigure_applying');

    try {
        await api('nests/' + nestId + '/safe-reconfigure', {
            method: 'POST',
            body: JSON.stringify(patch)
        });
        closeModal('reconfigure-modal');
        showToast(t('invasion.reconfigure_success'), 'success');
        await loadNests();
    } catch (e) {
        showToast(t('invasion.error') + ': ' + e.message, 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = t('invasion.reconfigure_apply');
    }
}

// ── Config History ───────────────────────────────────────
async function openConfigHistory(nestId) {
    const nest = nestsData.find(n => n.id === nestId);
    if (!nest) return;

    document.getElementById('history-nest-id').value = nestId;
    document.getElementById('config-history-title').textContent =
        t('invasion.config_history_title') + ' — ' + nest.name;

    const list = document.getElementById('config-history-list');
    list.innerHTML = `<div class="config-history-loading">${t('invasion.config_history_loading')}</div>`;

    openModal('config-history-modal');

    try {
        configHistoryData = await api('nests/' + nestId + '/config-history?limit=20');
        renderConfigHistory(nestId);
    } catch (e) {
        list.innerHTML = `<div class="config-history-error">${t('invasion.error')}: ${esc(e.message)}</div>`;
    }
}

function renderConfigHistory(nestId) {
    const list = document.getElementById('config-history-list');
    if (!configHistoryData || configHistoryData.length === 0) {
        list.innerHTML = `<div class="config-history-empty">${t('invasion.config_history_empty')}</div>`;
        return;
    }

    list.innerHTML = configHistoryData.map(rev => {
        const date = new Date(rev.created_at).toLocaleString();
        const statusClass = rev.status === 'applied' ? 'rev-applied' :
                           rev.status === 'failed' ? 'rev-failed' :
                           rev.status === 'pending' ? 'rev-pending' : 'rev-rolled-back';
        const statusText = t('invasion.config_status_' + rev.status) || rev.status;
        const canRollback = rev.status === 'applied';

        // Parse patch JSON for display
        let patchSummary = '';
        try {
            const patch = JSON.parse(rev.patch_json);
            patchSummary = Object.keys(patch).map(k => {
                const label = t('invasion.config_field_' + k) || k;
                return `<span class="rev-field">${esc(label)}</span>`;
            }).join(' ');
        } catch (e) {
            patchSummary = '<span class="rev-field">—</span>';
        }

        return `
            <div class="config-history-item ${statusClass}">
                <div class="rev-header">
                    <span class="rev-revision">#${esc(String(rev.revision))}</span>
                    <span class="rev-status ${statusClass}">${statusText}</span>
                    <span class="rev-date">${esc(date)}</span>
                </div>
                <div class="rev-fields">${patchSummary}</div>
                ${rev.error_message ? `<div class="rev-error">⚠️ ${esc(rev.error_message)}</div>` : ''}
                <div class="rev-actions">
                    <span class="rev-source">${esc(rev.source)}</span>
                    ${canRollback ? `<button class="btn btn-sm btn-secondary" data-action="rollback-config" data-id="${escAttr(nestId)}" data-revision="${escAttr(rev.id)}">↩️ ${t('invasion.config_rollback')}</button>` : ''}
                </div>
            </div>`;
    }).join('');
}

async function rollbackConfig(nestId, revisionId) {
    try {
        await api('nests/' + nestId + '/config-rollback', {
            method: 'POST',
            body: JSON.stringify({ revision_id: revisionId })
        });
        showToast(t('invasion.reconfigure_success'), 'success');
        // Reload history
        configHistoryData = await api('nests/' + nestId + '/config-history?limit=20');
        renderConfigHistory(nestId);
        await loadNests();
    } catch (e) {
        showToast(t('invasion.error') + ': ' + e.message, 'error');
    }
}

// ── esc() is now provided by shared.js ──
