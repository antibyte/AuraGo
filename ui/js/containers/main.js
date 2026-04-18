/* AuraGo – Containers page JS */
/* global I18N, t, applyI18n, esc */
'use strict';

let allContainers = [];
let currentFilter = 'all';
let lastDataHash = '';
let pollTimer = null;
let currentLogContainer = '';

// ── Initialization ──────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
    loadContainers();
    // Live updates pushed via SSE — no more polling.
    window.AuraSSE.on('container_update', function (containers) {
        if (!Array.isArray(containers)) return;
        const hash = JSON.stringify(containers);
        if (hash === lastDataHash) return;
        lastDataHash = hash;
        allContainers = containers;
        updateStats();
        renderContainers();
    });
});

// ── Data fetching ───────────────────────────────────────────────────────────

async function loadContainers() {
    try {
        const resp = await fetch('/api/containers');
        if (resp.status === 503) {
            showDisabledState();
            return;
        }
        const data = await resp.json();
        if (data.status !== 'ok') {
            showDisabledState();
            return;
        }

        // Hash comparison – skip re-render if nothing changed
        const hash = JSON.stringify(data.containers);
        if (hash === lastDataHash) return;
        lastDataHash = hash;

        allContainers = data.containers || [];
        updateStats();
        renderContainers();
    } catch (e) {
        console.error('Failed to load containers:', e);
    }
}

function showDisabledState() {
    document.getElementById('ct-grid').style.display = 'none';
    document.getElementById('ct-empty').style.display = 'none';
    document.getElementById('ct-disabled').style.display = '';
    document.getElementById('ct-status-bar').style.display = 'none';
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
}

// ── Stats ───────────────────────────────────────────────────────────────────

function updateStats() {
    const running = allContainers.filter(c => c.state === 'running').length;
    const stopped = allContainers.length - running;
    document.getElementById('ct-total').textContent = allContainers.length;
    document.getElementById('ct-running').textContent = running;
    document.getElementById('ct-stopped').textContent = stopped;
}

// ── Rendering ───────────────────────────────────────────────────────────────

function renderContainers() {
    const grid = document.getElementById('ct-grid');
    const empty = document.getElementById('ct-empty');
    const disabled = document.getElementById('ct-disabled');
    disabled.style.display = 'none';
    document.getElementById('ct-status-bar').style.display = '';

    const filtered = getFilteredContainers();

    if (filtered.length === 0) {
        grid.style.display = 'none';
        empty.style.display = '';
        return;
    }
    empty.style.display = 'none';
    grid.style.display = '';

    grid.innerHTML = filtered.map(c => renderCard(c)).join('');
    if (typeof applyI18n === 'function') applyI18n();
}

function renderCard(c) {
    const name = (c.names && c.names.length > 0) ? c.names[0].replace(/^\//, '') : c.id;
    const state = (c.state || 'unknown').toLowerCase();
    const isRunning = state === 'running';
    const isPaused = state === 'paused';
    const deleteId = JSON.stringify(c.id || '').replace(/"/g, '&quot;');
    const deleteName = JSON.stringify(name).replace(/"/g, '&quot;');

    let actionBtns = '';
    if (isRunning) {
        actionBtns = `
            <button class="btn btn-sm btn-secondary" onclick="containerAction('${c.id}','stop')" data-i18n="containers.btn_stop">⏹ Stop</button>
            <button class="btn btn-sm btn-secondary" onclick="containerAction('${c.id}','restart')" data-i18n="containers.btn_restart">🔄 Restart</button>`;
    } else if (isPaused) {
        actionBtns = `
            <button class="btn btn-sm btn-primary" onclick="containerAction('${c.id}','start')" data-i18n="containers.btn_unpause">▶ Resume</button>`;
    } else {
        actionBtns = `
            <button class="btn btn-sm btn-primary" onclick="containerAction('${c.id}','start')" data-i18n="containers.btn_start">▶ Start</button>`;
    }

    return `
    <div class="ct-card" data-id="${c.id}" data-state="${state}">
        <div class="ct-card-header">
            <div class="ct-card-status ${state}"></div>
            <div class="ct-card-name" title="${esc(name)}">${esc(name)}</div>
            <span class="ct-card-id">${esc(c.id)}</span>
        </div>
        <div class="ct-card-meta">
            <span><span class="ct-meta-icon">📦</span> ${esc(c.image)}</span>
            <span><span class="ct-meta-icon">📋</span> <span class="ct-card-state ${state}">${esc(c.status)}</span></span>
        </div>
        <div class="ct-card-actions">
            ${actionBtns}
            <button class="btn btn-sm btn-secondary" onclick="showLogs('${c.id}')" data-i18n="containers.btn_logs">📄 Logs</button>
            <button class="btn btn-sm btn-secondary" onclick="showInspect('${c.id}')" data-i18n="containers.btn_inspect">🔍 Inspect</button>
            <button class="btn btn-sm btn-danger" onclick="showDeleteModal(${deleteId}, ${deleteName})" data-i18n="containers.btn_remove">🗑 Remove</button>
        </div>
    </div>`;
}

// ── Filtering ───────────────────────────────────────────────────────────────

function getFilteredContainers() {
    const search = (document.getElementById('ct-search').value || '').toLowerCase();
    return allContainers.filter(c => {
        // State filter
        if (currentFilter === 'running' && c.state !== 'running') return false;
        if (currentFilter === 'stopped' && c.state === 'running') return false;
        // Search filter
        if (search) {
            const name = (c.names || []).join(' ').toLowerCase();
            const image = (c.image || '').toLowerCase();
            const id = (c.id || '').toLowerCase();
            if (!name.includes(search) && !image.includes(search) && !id.includes(search)) return false;
        }
        return true;
    });
}

// eslint-disable-next-line no-unused-vars
function filterContainers() {
    renderContainers();
}

// eslint-disable-next-line no-unused-vars
function setFilter(filter) {
    currentFilter = filter;
    document.querySelectorAll('.ct-filter-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.filter === filter);
    });
    renderContainers();
}

// ── Container Actions ───────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
async function containerAction(id, action) {
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(id)}/${action}`, { method: 'POST' });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('containers.action_success') || `Action "${action}" successful`, 'success');
            lastDataHash = ''; // force refresh
            await loadContainers();
        } else {
            showToast(dockerErrMsg(data.message), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    }
}

// ── Logs Modal ──────────────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
async function showLogs(id) {
    currentLogContainer = id;
    document.getElementById('log-output').textContent = t('common.loading') || 'Loading...';
    document.getElementById('log-modal').classList.add('active');
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(id)}/logs?tail=500`);
        const data = await resp.json();
        if (data.status === 'ok') {
            document.getElementById('log-output').textContent = data.logs || '(empty)';
            // Scroll to bottom
            const el = document.getElementById('log-output');
            el.scrollTop = el.scrollHeight;
        } else {
            document.getElementById('log-output').textContent = data.message || 'Error loading logs';
        }
    } catch (e) {
        document.getElementById('log-output').textContent = 'Failed to load logs';
    }
}

// eslint-disable-next-line no-unused-vars
function refreshLogs() {
    if (currentLogContainer) showLogs(currentLogContainer);
}

// eslint-disable-next-line no-unused-vars
function closeLogModal() {
    document.getElementById('log-modal').classList.remove('active');
    currentLogContainer = '';
}

// ── Inspect Modal ───────────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
async function showInspect(id) {
    document.getElementById('inspect-output').textContent = t('common.loading') || 'Loading...';
    document.getElementById('inspect-modal').classList.add('active');
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(id)}/inspect`);
        const data = await resp.json();
        document.getElementById('inspect-output').textContent = JSON.stringify(data, null, 2);
    } catch (e) {
        document.getElementById('inspect-output').textContent = 'Failed to load details';
    }
}

// eslint-disable-next-line no-unused-vars
function closeInspectModal() {
    document.getElementById('inspect-modal').classList.remove('active');
}

// ── Delete Modal ────────────────────────────────────────────────────────────

let deleteTarget = '';

// eslint-disable-next-line no-unused-vars
function showDeleteModal(id, name) {
    deleteTarget = id;
    document.getElementById('delete-container-name').textContent = name;
    document.getElementById('delete-force').checked = false;
    document.getElementById('delete-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeDeleteModal() {
    document.getElementById('delete-modal').classList.remove('active');
    deleteTarget = '';
}

// eslint-disable-next-line no-unused-vars
async function confirmDelete() {
    if (!deleteTarget) return;
    const force = document.getElementById('delete-force').checked;
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(deleteTarget)}?force=${force}`, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('containers.delete_success') || 'Container removed', 'success');
            closeDeleteModal();
            lastDataHash = '';
            await loadContainers();
        } else {
            showToast(dockerErrMsg(data.message), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    }
}

// ── Helpers ─────────────────────────────────────────────────────────────────
// dockerErrMsg extracts human-readable text from Docker API error responses.
// Docker wraps errors as {"message":"..."}; our Go handler may also JSON-encode
// them further. We unwrap up to two layers.
function dockerErrMsg(msg) {
    if (!msg) return t('common.error') || 'Error';
    let text = msg;
    for (let i = 0; i < 2; i++) {
        try {
            const obj = JSON.parse(text);
            if (obj && typeof obj.message === 'string') {
                text = obj.message;
            } else {
                break;
            }
        } catch {
            break;
        }
    }
    return text.length > 200 ? text.slice(0, 197) + '…' : text;
}
