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

// Protection flags of the last /api/containers answer, keyed by container id.
// SSE container updates carry no flags, so they are merged from here.
const protectionById = new Map();
const CONFIRM_PROTECTED_QUERY = 'confirm=protected';

// True while the last list request failed (HTTP 502) or Docker is disabled
// (HTTP 503). renderContainers() leaves that state alone until a list loads
// again, so a search or filter input cannot bring back stale cards.
let listUnavailable = false;

// True when the last list request failed (HTTP 502 or an unreadable error
// answer), as opposed to Docker being disabled (HTTP 503). Only a failure
// retries on its own: a short Docker blip must not leave the page stuck, and
// the SSE feed pushes only when the list changes.
let listFailed = false;
const LIST_RETRY_MS = 10000;
let listRetryTimer = null;

// ── Initialization ──────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
    bindContainersChrome();
    loadContainers();
    // Live updates pushed via SSE — no more polling.
    window.AuraSSE.on('container_update', function (containers) {
        if (!Array.isArray(containers)) return;
        // The list is unavailable: a pushed update means Docker answers again,
        // so reload the list (with fresh protection flags) instead of merging.
        if (listUnavailable) {
            lastDataHash = '';
            loadContainers();
            return;
        }
        // A container the last list did not classify: reload the list so its
        // protection flags are known before any action button is used.
        if (containers.some(c => !protectionById.has(c.id || ''))) {
            lastDataHash = '';
            loadContainers();
            return;
        }
        const merged = containers.map(c => Object.assign({}, c, protectionById.get(c.id || '')));
        const hash = JSON.stringify(merged);
        if (hash === lastDataHash) return;
        lastDataHash = hash;
        allContainers = merged;
        updateStats();
        renderContainers();
    });
});

function bindContainersChrome() {
    document.querySelectorAll('.ct-filter-btn[data-filter]').forEach((btn) => {
        btn.addEventListener('click', () => setFilter(btn.dataset.filter));
    });
    document.getElementById('ct-search')?.addEventListener('input', filterContainers);
}

// ── Data fetching ───────────────────────────────────────────────────────────

async function loadContainers() {
    let resp = null;
    try {
        resp = await fetch('/api/containers');
        if (resp.status === 503) {
            showDisabledState();
            return;
        }
        const data = await resp.json();
        if (data.status !== 'ok') {
            // Docker is enabled but the list failed (HTTP 502): show Docker's
            // message instead of the "Docker not enabled" state.
            showListErrorState(dockerErrMsg(data.message || data.error));
            return;
        }
        listUnavailable = false;
        listFailed = false;
        cancelListRetry();

        // Hash comparison – skip re-render if nothing changed
        const hash = JSON.stringify(data.containers);
        if (hash === lastDataHash) return;
        lastDataHash = hash;

        allContainers = data.containers || [];
        rememberProtection(allContainers);
        updateStats();
        renderContainers();
    } catch (e) {
        console.error('Failed to load containers:', e);
        // A reverse proxy may have replaced the 502 body with HTML, so the JSON
        // parse failed: the list is still unavailable.
        if (resp && !resp.ok) showListErrorState(t('common.error'));
        // A retry that could not even reach AuraGo keeps retrying.
        else if (listFailed && !listRetryTimer) armListRetry();
    }
}

// armListRetry schedules the next list load; there is never more than one
// pending timer.
function armListRetry() {
    cancelListRetry();
    listRetryTimer = setTimeout(() => {
        listRetryTimer = null;
        loadContainers();
    }, LIST_RETRY_MS);
}

function cancelListRetry() {
    if (listRetryTimer) {
        clearTimeout(listRetryTimer);
        listRetryTimer = null;
    }
}

// clearContainerList drops every card and the data behind it, so nothing stale
// can reappear while the list is unavailable.
function clearContainerList() {
    allContainers = [];
    lastDataHash = '';
    cardRenderCache.clear();
    protectionById.clear();
    document.getElementById('ct-grid').replaceChildren();
    listUnavailable = true;
}

function showDisabledState() {
    clearContainerList();
    listFailed = false;
    cancelListRetry();
    document.getElementById('ct-grid').style.display = 'none';
    document.getElementById('ct-empty').classList.add('is-hidden');
    document.getElementById('ct-list-error').classList.add('is-hidden');
    document.getElementById('ct-disabled').classList.remove('is-hidden');
    document.getElementById('ct-status-bar').style.display = 'none';
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
}

function showListErrorState(message) {
    clearContainerList();
    listFailed = true;
    document.getElementById('ct-grid').style.display = 'none';
    document.getElementById('ct-empty').classList.add('is-hidden');
    document.getElementById('ct-disabled').classList.add('is-hidden');
    document.getElementById('ct-list-error-message').textContent = message;
    document.getElementById('ct-list-error').classList.remove('is-hidden');
    document.getElementById('ct-status-bar').style.display = 'none';
    armListRetry();
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

function rememberProtection(containers) {
    protectionById.clear();
    for (const c of containers) {
        protectionById.set(c.id || '', {
            protected_owner: c.protected_owner || '',
            self: !!c.self,
            docker_endpoint: !!c.docker_endpoint,
            shared_network: !!c.shared_network
        });
    }
}

// containerProtection returns why AuraGo protects a container: 'self',
// 'docker-endpoint', 'shared-network', the managing owner, or '' when it is
// not protected. The order matches the server's "owner" label.
function containerProtection(c) {
    if (!c) return '';
    if (c.self) return 'self';
    if (c.docker_endpoint) return 'docker-endpoint';
    if (c.shared_network) return 'shared-network';
    return c.protected_owner || '';
}

function findContainer(id) {
    return allContainers.find(c => c.id === id) || null;
}

function protectionWarningKey(kind) {
    if (kind === 'self') return 'containers.protected_self_warning';
    if (kind === 'docker-endpoint') return 'containers.protected_endpoint_warning';
    if (kind === 'shared-network') return 'containers.protected_network_warning';
    if (kind === 'unverified') return 'containers.protected_unverified_warning';
    return 'containers.protected_warning';
}

function renderContainers() {
    // The list is unavailable (Docker error or disabled): keep that state until
    // a list loads again.
    if (listUnavailable) return;
    const grid = document.getElementById('ct-grid');
    const empty = document.getElementById('ct-empty');
    const disabled = document.getElementById('ct-disabled');
    disabled.classList.add('is-hidden');
    document.getElementById('ct-list-error').classList.add('is-hidden');
    document.getElementById('ct-status-bar').style.display = '';

    const filtered = getFilteredContainers();

    if (filtered.length === 0) {
        if (cardRenderCache.size > 0) {
            grid.replaceChildren();
            cardRenderCache.clear();
        }
        if (grid.style.display !== 'none') grid.style.display = 'none';
        empty.classList.remove('is-hidden');
        return;
    }
    if (grid.style.display !== '') grid.style.display = '';
    empty.classList.add('is-hidden');

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
    const protectedBadge = containerProtection(c)
        ? `<span class="ct-card-protected">${esc(t('containers.protected_badge'))}</span>`
        : '';

    let actionBtns = '';
    if (isRunning) {
        actionBtns = `
            <button class="btn btn-sm btn-secondary" onclick="containerAction(${safeID},'stop')" data-i18n="containers.btn_stop">⏹ Stop</button>
            <button class="btn btn-sm btn-secondary" onclick="containerAction(${safeID},'restart')" data-i18n="containers.btn_restart">🔄 Restart</button>
            <button class="btn btn-sm btn-primary" onclick="showTerminal(${safeID}, ${terminalName})" data-i18n="containers.btn_shell">⌨ Shell</button>`;
    } else if (isPaused) {
        actionBtns = `
            <button class="btn btn-sm btn-primary" onclick="containerAction(${safeID},'unpause')" data-i18n="containers.btn_unpause">▶ Resume</button>`;
    } else {
        actionBtns = `
            <button class="btn btn-sm btn-primary" onclick="containerAction(${safeID},'start')" data-i18n="containers.btn_start">▶ Start</button>`;
    }

    return `
    <div class="ct-card" data-id="${esc(c.id || '')}" data-state="${esc(state)}">
        <div class="ct-card-header">
            <div class="ct-card-status ${stateClass}"></div>
            <div class="ct-card-name" title="${esc(name)}">${esc(name)}</div>
            ${protectedBadge}
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
    document.querySelectorAll('.ct-filter-btn[data-filter]').forEach((btn) => {
        const active = btn.dataset.filter === filter;
        btn.classList.toggle('active', active);
        btn.setAttribute('aria-pressed', active ? 'true' : 'false');
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
let updateProtection = '';

// eslint-disable-next-line no-unused-vars
function showUpdateModal(id, name) {
    updateTarget = id;
    updateInFlight = false;
    updateProtection = containerProtection(findContainer(id));
    document.getElementById('update-container-name').textContent = name;
    renderUpdateProtection();
    setUpdateConfirmBusy(false);
    document.getElementById('update-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeUpdateModal() {
    document.getElementById('update-modal').classList.remove('active');
    updateTarget = '';
    updateInFlight = false;
    updateProtection = '';
    setUpdateConfirmBusy(false);
}

// updateBlocked: the server refuses to update the container AuraGo runs in or
// reaches Docker through, because the update would stop AuraGo first.
function updateBlocked() {
    return updateProtection === 'self' || updateProtection === 'docker-endpoint';
}

function renderUpdateProtection() {
    const warning = document.getElementById('update-protected-warning');
    if (!warning) return;
    if (!updateProtection) {
        warning.textContent = '';
        warning.classList.add('is-hidden');
        return;
    }
    warning.textContent = t(updateBlocked() ? 'containers.self_update_unsupported' : protectionWarningKey(updateProtection));
    warning.classList.remove('is-hidden');
}

// eslint-disable-next-line no-unused-vars
async function confirmUpdate() {
    if (!updateTarget || updateInFlight || updateBlocked()) return;
    updateInFlight = true;
    setUpdateConfirmBusy(true);
    const query = updateProtection ? `?${CONFIRM_PROTECTED_QUERY}` : '';
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(updateTarget)}/update${query}`, { method: 'POST' });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('containers.update_success'), 'success');
            closeUpdateModal();
            lastDataHash = '';
            await loadContainers();
        } else if (data.code === 'container_protected_confirmation_required' || data.code === 'container_self_update_unsupported') {
            // The list was older than the server's answer: show why, then the
            // operator confirms again (or sees that the update cannot run).
            updateProtection = data.owner || 'unverified';
            renderUpdateProtection();
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
        confirmBtn.disabled = busy || updateBlocked();
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

let protectedTerminalTarget = null;

// eslint-disable-next-line no-unused-vars
function showTerminal(id, name) {
    const protection = containerProtection(findContainer(id));
    if (protection) {
        showProtectedTerminalModal(id, name, protection);
        return;
    }
    openTerminal(id, name, false);
}

function showProtectedTerminalModal(id, name, protection) {
    protectedTerminalTarget = { id, name };
    document.getElementById('protected-terminal-name').textContent = name;
    document.getElementById('protected-terminal-warning').textContent = t(protectionWarningKey(protection));
    document.getElementById('protected-terminal-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeProtectedTerminalModal() {
    document.getElementById('protected-terminal-modal').classList.remove('active');
    protectedTerminalTarget = null;
}

// eslint-disable-next-line no-unused-vars
function confirmProtectedTerminal() {
    const target = protectedTerminalTarget;
    closeProtectedTerminalModal();
    if (target) openTerminal(target.id, target.name, true);
}

// openTerminal starts the shell session; confirmed carries the confirmation the
// operator just gave for a protected container.
function openTerminal(id, name, confirmed) {
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
    const query = confirmed ? `?${CONFIRM_PROTECTED_QUERY}` : '';
    terminalSocket = new WebSocket(`${scheme}://${window.location.host}/api/containers/${encodeURIComponent(id)}/terminal${query}`);
    terminalSocket.binaryType = 'arraybuffer';
    let opened = false;

    terminal.onData(data => {
        if (!terminalSocket || terminalSocket.readyState !== WebSocket.OPEN) return;
        terminalSocket.send(new TextEncoder().encode(data));
    });

    terminalSocket.onopen = () => {
        if (token !== terminalSessionToken) return;
        opened = true;
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
        // A refused handshake also fires onclose, which explains the failure.
        if (!opened) return;
        setTerminalStatus('containers.terminal_error');
        writeTerminalNotice('containers.terminal_error');
    };
    terminalSocket.onclose = () => {
        if (token !== terminalSessionToken) return;
        if (!opened) {
            explainTerminalHandshakeFailure(id, name, confirmed, token);
            return;
        }
        setTerminalStatus('containers.terminal_closed');
        if (terminal) terminal.write(`\r\n[${t('containers.terminal_closed')}]\r\n`);
    };

    if (window.ResizeObserver) {
        terminalResizeObserver = new ResizeObserver(() => scheduleTerminalFit());
        terminalResizeObserver.observe(output);
    }
    window.addEventListener('resize', scheduleTerminalFit);
}

// explainTerminalHandshakeFailure runs when the terminal WebSocket closed
// before it opened. Browsers hide the HTTP answer of a refused handshake, so
// the page asks the server whether the container needs a confirmation (a list
// older than the server's view, or ownership Docker did not confirm) and then
// offers the confirmation modal. It never retries with the flag on its own.
async function explainTerminalHandshakeFailure(id, name, confirmed, token) {
    let report = null;
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(id)}/protection`);
        report = await resp.json();
    } catch (e) {
        report = null;
    }
    if (token !== terminalSessionToken) return;
    if (!confirmed && report && report.status === 'ok' && report.protected && !report.read_only) {
        closeTerminalModal();
        showProtectedTerminalModal(id, name, report.owner || 'unverified');
        return;
    }
    setTerminalStatus('containers.terminal_error');
    writeTerminalNotice('containers.terminal_error');
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
let deleteProtection = '';

// eslint-disable-next-line no-unused-vars
function showDeleteModal(id, name) {
    deleteTarget = id;
    deleteInFlight = false;
    deleteProtection = containerProtection(findContainer(id));
    document.getElementById('delete-container-name').textContent = name;
    document.getElementById('delete-force').checked = false;
    renderDeleteProtection();
    setDeleteConfirmBusy(false);
    document.getElementById('delete-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeDeleteModal() {
    document.getElementById('delete-modal').classList.remove('active');
    deleteTarget = '';
    deleteInFlight = false;
    deleteProtection = '';
    setDeleteConfirmBusy(false);
}

function renderDeleteProtection() {
    const warning = document.getElementById('delete-protected-warning');
    if (!warning) return;
    warning.textContent = deleteProtection ? t(protectionWarningKey(deleteProtection)) : '';
    warning.classList.toggle('is-hidden', !deleteProtection);
}

// eslint-disable-next-line no-unused-vars
async function confirmDelete() {
    if (!deleteTarget || deleteInFlight) return;
    deleteInFlight = true;
    setDeleteConfirmBusy(true);
    const force = document.getElementById('delete-force').checked;
    const confirmQuery = deleteProtection ? `&${CONFIRM_PROTECTED_QUERY}` : '';
    try {
        const resp = await fetch(`/api/containers/${encodeURIComponent(deleteTarget)}?force=${force}${confirmQuery}`, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('containers.delete_success'), 'success');
            closeDeleteModal();
            lastDataHash = '';
            await loadContainers();
        } else if (data.code === 'container_protected_confirmation_required') {
            // The list was older than the server's answer: show the warning
            // and let the operator confirm again.
            deleteProtection = data.owner || 'unverified';
            renderDeleteProtection();
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
