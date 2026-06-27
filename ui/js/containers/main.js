/* AuraGo – Containers page JS */
/* global I18N, t, applyI18n, esc */
'use strict';

let allContainers = [];
let currentFilter = 'all';
let lastDataHash = '';
let pollTimer = null;
let currentLogContainer = '';
let terminal = null;
let terminalFitAddon = null;
let terminalSocket = null;
let terminalResizeObserver = null;
let terminalSessionToken = 0;
let terminalFitScheduled = false;

// Per-card render cache: id -> { html, el }.
// Used by renderContainers() to update the grid in place instead of rebuilding
// the entire innerHTML on every SSE update. Cards whose rendered HTML hasn't
// changed are left alone — scroll position, focus and hover state survive.
const cardRenderCache = new Map();

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

function jsArg(value) {
    return JSON.stringify(String(value ?? ''))
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

function renderContainers() {
    const grid = document.getElementById('ct-grid');
    const empty = document.getElementById('ct-empty');
    const disabled = document.getElementById('ct-disabled');
    disabled.style.display = 'none';
    document.getElementById('ct-status-bar').style.display = '';

    const filtered = getFilteredContainers();

    if (filtered.length === 0) {
        if (cardRenderCache.size > 0) {
            grid.replaceChildren();
            cardRenderCache.clear();
        }
        if (grid.style.display !== 'none') grid.style.display = 'none';
        if (empty.style.display !== '') empty.style.display = '';
        return;
    }
    if (grid.style.display !== '') grid.style.display = '';
    if (empty.style.display !== 'none') empty.style.display = 'none';

    // Diff the new filtered list against the cached DOM. Cards that haven't
    // changed are left untouched so scroll/focus/hover survive; only added,
    // removed or changed cards trigger DOM mutations. Order is rebuilt by
    // inserting each card after the previously placed one.
    const newIds = new Set();
    for (const c of filtered) newIds.add(c.id || '');

    let mutated = false;

    // Remove cards that disappeared from the filtered set.
    for (const [id, entry] of cardRenderCache) {
        if (!newIds.has(id)) {
            entry.el.remove();
            cardRenderCache.delete(id);
            mutated = true;
        }
    }

    let prevEl = null;
    for (const c of filtered) {
        const id = c.id || '';
        const html = renderCard(c);
        const existing = cardRenderCache.get(id);

        if (existing) {
            if (existing.html === html) {
                // Unchanged: keep the live DOM node as-is.
                prevEl = existing.el;
                continue;
            }
            // Changed: replace the node in place.
            const newEl = buildCardElement(html);
            if (!newEl) continue;
            existing.el.replaceWith(newEl);
            cardRenderCache.set(id, { html, el: newEl });
            prevEl = newEl;
            mutated = true;
            continue;
        }

        // New card: insert after prevEl (or at the top if none yet).
        const newEl = buildCardElement(html);
        if (!newEl) continue;
        if (prevEl && prevEl.parentNode === grid) {
            prevEl.after(newEl);
        } else if (grid.firstChild) {
            grid.insertBefore(newEl, grid.firstChild);
        } else {
            grid.appendChild(newEl);
        }
        cardRenderCache.set(id, { html, el: newEl });
        prevEl = newEl;
        mutated = true;
    }

    // Only re-apply i18n when DOM actually changed — text on untouched cards
    // is already translated and would be needlessly re-walked otherwise.
    if (mutated && typeof applyI18n === 'function') applyI18n();
}

// buildCardElement parses the renderCard HTML string into a real DOM element.
// Uses a <template> so the children are inserted as elements (not as a text
// node) without invoking any inline scripts.
function buildCardElement(html) {
    const tpl = document.createElement('template');
    tpl.innerHTML = String(html || '').trim();
    return tpl.content.firstElementChild;
}

function renderCard(c) {
    const name = (c.names && c.names.length > 0) ? c.names[0].replace(/^\//, '') : c.id;
    const state = (c.state || 'unknown').toLowerCase();
    const stateClass = /^[a-z0-9_-]+$/.test(state) ? state : 'unknown';
    const isRunning = state === 'running';
    const isPaused = state === 'paused';
    const safeID = jsArg(c.id || '');
    const deleteId = safeID;
    const deleteName = jsArg(name);
    const terminalName = jsArg(name);
    const updateName = jsArg(name);

    let actionBtns = '';
    if (isRunning) {
        actionBtns = `
            <button class="btn btn-sm btn-secondary" onclick="containerAction(${safeID},'stop')" data-i18n="containers.btn_stop">⏹ Stop</button>
            <button class="btn btn-sm btn-secondary" onclick="containerAction(${safeID},'restart')" data-i18n="containers.btn_restart">🔄 Restart</button>
            <button class="btn btn-sm btn-primary" onclick="showTerminal(${safeID}, ${terminalName})" data-i18n="containers.btn_shell">⌨ Shell</button>`;
    } else if (isPaused) {
        actionBtns = `
            <button class="btn btn-sm btn-primary" onclick="containerAction(${safeID},'start')" data-i18n="containers.btn_unpause">▶ Resume</button>`;
    } else {
        actionBtns = `
            <button class="btn btn-sm btn-primary" onclick="containerAction(${safeID},'start')" data-i18n="containers.btn_start">▶ Start</button>`;
    }

    return `
    <div class="ct-card" data-id="${esc(c.id || '')}" data-state="${esc(state)}">
        <div class="ct-card-header">
            <div class="ct-card-status ${stateClass}"></div>
            <div class="ct-card-name" title="${esc(name)}">${esc(name)}</div>
            <span class="ct-card-id">${esc(c.id)}</span>
        </div>
        <div class="ct-card-meta">
            <span><span class="ct-meta-icon">📦</span> ${esc(c.image)}</span>
            <span><span class="ct-meta-icon">📋</span> <span class="ct-card-state ${stateClass}">${esc(c.status)}</span></span>
        </div>
        <div class="ct-card-actions">
            ${actionBtns}
            <button class="btn btn-sm btn-secondary" onclick="showUpdateModal(${safeID}, ${updateName})" data-i18n="containers.btn_update">⬇ Update</button>
            <button class="btn btn-sm btn-secondary" onclick="showLogs(${safeID})" data-i18n="containers.btn_logs">📄 Logs</button>
            <button class="btn btn-sm btn-secondary" onclick="showInspect(${safeID})" data-i18n="containers.btn_inspect">🔍 Inspect</button>
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
        showToast(t('common.error'), 'error');
    }
}

// ── Update Modal ────────────────────────────────────────────────────────────

let updateTarget = '';
let updateInFlight = false;

// eslint-disable-next-line no-unused-vars
function showUpdateModal(id, name) {
    updateTarget = id;
    updateInFlight = false;
    document.getElementById('update-container-name').textContent = name;
    setUpdateConfirmBusy(false);
    document.getElementById('update-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeUpdateModal() {
    document.getElementById('update-modal').classList.remove('active');
    updateTarget = '';
    updateInFlight = false;
    setUpdateConfirmBusy(false);
}

// eslint-disable-next-line no-unused-vars
async function confirmUpdate() {
    if (!updateTarget || updateInFlight) return;
    updateInFlight = true;
    setUpdateConfirmBusy(true);
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(updateTarget)}/update`, { method: 'POST' });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('containers.update_success'), 'success');
            closeUpdateModal();
            lastDataHash = '';
            await loadContainers();
        } else {
            showToast(dockerErrMsg(data.message), 'error');
        }
    } catch (e) {
        showToast(t('common.error'), 'error');
    } finally {
        if (updateTarget) {
            updateInFlight = false;
            setUpdateConfirmBusy(false);
        }
    }
}

function setUpdateConfirmBusy(busy) {
    const confirmBtn = document.getElementById('update-confirm-btn');
    if (confirmBtn) {
        confirmBtn.disabled = busy;
    }
}

// ── Logs Modal ──────────────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
async function showLogs(id) {
    currentLogContainer = id;
    document.getElementById('log-output').textContent = t('common.loading');
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
    document.getElementById('inspect-output').textContent = t('common.loading');
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

// ── Terminal Modal ─────────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
function showTerminal(id, name) {
    closeTerminalSession();
    terminalSessionToken += 1;
    const token = terminalSessionToken;
    const modal = document.getElementById('terminal-modal');
    const output = document.getElementById('terminal-output');
    const title = document.getElementById('terminal-container-name');
    title.textContent = name ? `· ${name}` : '';
    output.innerHTML = '';
    modal.classList.add('active');
    setTerminalStatus('containers.terminal_connecting');

    if (!window.Terminal) {
        setTerminalStatus('containers.terminal_error');
        output.textContent = t('containers.terminal_unavailable');
        return;
    }

    terminal = new window.Terminal({
        cursorBlink: true,
        convertEol: true,
        fontFamily: "'Fira Code', 'Cascadia Code', Consolas, monospace",
        fontSize: 13,
        scrollback: 2000,
        theme: {
            background: '#05070a',
            foreground: '#d7e1ec',
            cursor: '#8bd3ff'
        }
    });
    if (window.FitAddon && window.FitAddon.FitAddon) {
        terminalFitAddon = new window.FitAddon.FitAddon();
        terminal.loadAddon(terminalFitAddon);
    }
    terminal.open(output);
    writeTerminalNotice('containers.terminal_opening');
    scheduleTerminalFit();
    terminal.focus();

    const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
    terminalSocket = new WebSocket(`${scheme}://${window.location.host}/api/containers/${encodeURIComponent(id)}/terminal`);
    terminalSocket.binaryType = 'arraybuffer';

    terminal.onData(data => {
        if (!terminalSocket || terminalSocket.readyState !== WebSocket.OPEN) return;
        terminalSocket.send(new TextEncoder().encode(data));
    });

    terminalSocket.onopen = () => {
        if (token !== terminalSessionToken) return;
        setTerminalStatus('containers.terminal_connected');
        writeTerminalNotice('containers.terminal_connected');
        scheduleTerminalFit();
    };
    terminalSocket.onmessage = event => {
        if (token !== terminalSessionToken || !terminal) return;
        if (typeof event.data === 'string') {
            terminal.write(event.data);
            return;
        }
        terminal.write(new TextDecoder().decode(event.data));
    };
    terminalSocket.onerror = () => {
        if (token !== terminalSessionToken) return;
        setTerminalStatus('containers.terminal_error');
        writeTerminalNotice('containers.terminal_error');
    };
    terminalSocket.onclose = () => {
        if (token !== terminalSessionToken) return;
        setTerminalStatus('containers.terminal_closed');
        if (terminal) terminal.write(`\r\n[${t('containers.terminal_closed')}]\r\n`);
    };

    if (window.ResizeObserver) {
        terminalResizeObserver = new ResizeObserver(() => scheduleTerminalFit());
        terminalResizeObserver.observe(output);
    }
    window.addEventListener('resize', scheduleTerminalFit);
}

// eslint-disable-next-line no-unused-vars
function closeTerminalModal() {
    document.getElementById('terminal-modal').classList.remove('active');
    closeTerminalSession();
}

function closeTerminalSession() {
    terminalSessionToken += 1;
    terminalFitScheduled = false;
    window.removeEventListener('resize', scheduleTerminalFit);
    if (terminalResizeObserver) {
        terminalResizeObserver.disconnect();
        terminalResizeObserver = null;
    }
    if (terminalSocket) {
        terminalSocket.onopen = null;
        terminalSocket.onmessage = null;
        terminalSocket.onerror = null;
        terminalSocket.onclose = null;
        if (terminalSocket.readyState === WebSocket.OPEN || terminalSocket.readyState === WebSocket.CONNECTING) {
            terminalSocket.close();
        }
        terminalSocket = null;
    }
    if (terminal) {
        terminal.dispose();
        terminal = null;
    }
    terminalFitAddon = null;
}

function fitTerminal() {
    if (!terminal) return;
    if (terminalFitAddon) {
        try {
            terminalFitAddon.fit();
        } catch (e) {
            // xterm cannot fit while the modal is hidden; the next visible resize will retry.
        }
    }
    if (terminalSocket && terminalSocket.readyState === WebSocket.OPEN) {
        terminalSocket.send(JSON.stringify({ type: 'resize', cols: terminal.cols, rows: terminal.rows }));
    }
}

function scheduleTerminalFit() {
    if (terminalFitScheduled) return;
    terminalFitScheduled = true;
    const run = () => {
        terminalFitScheduled = false;
        fitTerminal();
    };
    if (window.requestAnimationFrame) {
        window.requestAnimationFrame(run);
        return;
    }
    setTimeout(run, 0);
}

function writeTerminalNotice(key) {
    if (!terminal) return;
    const message = t(key) || key;
    terminal.writeln(`\x1b[2m${message}\x1b[0m`);
}

function setTerminalStatus(key) {
    const el = document.getElementById('terminal-status');
    if (!el) return;
    el.textContent = t(key) || key;
}

// ── Delete Modal ────────────────────────────────────────────────────────────

let deleteTarget = '';
let deleteInFlight = false;

// eslint-disable-next-line no-unused-vars
function showDeleteModal(id, name) {
    deleteTarget = id;
    deleteInFlight = false;
    document.getElementById('delete-container-name').textContent = name;
    document.getElementById('delete-force').checked = false;
    setDeleteConfirmBusy(false);
    document.getElementById('delete-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeDeleteModal() {
    document.getElementById('delete-modal').classList.remove('active');
    deleteTarget = '';
    deleteInFlight = false;
    setDeleteConfirmBusy(false);
}

// eslint-disable-next-line no-unused-vars
async function confirmDelete() {
    if (!deleteTarget || deleteInFlight) return;
    deleteInFlight = true;
    setDeleteConfirmBusy(true);
    const force = document.getElementById('delete-force').checked;
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(deleteTarget)}?force=${force}`, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('containers.delete_success'), 'success');
            closeDeleteModal();
            lastDataHash = '';
            await loadContainers();
        } else {
            showToast(dockerErrMsg(data.message), 'error');
        }
    } catch (e) {
        showToast(t('common.error'), 'error');
    } finally {
        if (deleteTarget) {
            deleteInFlight = false;
            setDeleteConfirmBusy(false);
        }
    }
}

function setDeleteConfirmBusy(busy) {
    const confirmBtn = document.getElementById('delete-confirm-btn');
    if (confirmBtn) {
        confirmBtn.disabled = busy;
    }
}

// ── Helpers ─────────────────────────────────────────────────────────────────
// dockerErrMsg extracts human-readable text from Docker API error responses.
// Docker wraps errors as {"message":"..."}; our Go handler may also JSON-encode
// them further. We unwrap up to two layers.
function dockerErrMsg(msg) {
    if (!msg) return t('common.error');
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
