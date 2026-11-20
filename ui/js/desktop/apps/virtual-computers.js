(function () {
    'use strict';
    const instances = new Map();
    const machinePollIntervalMs = 5000;
    const expiryCountdownIntervalMs = 1000;
    // The desktop JSON endpoints accept 1 MiB; base64 inflates a file by a third.
    const uploadLimitBytes = 700 * 1024;
    const K = 'desktop.virtual_computers_';
    const confirmDefinitions = {
        destroy: { title: 'desktop.virtual_computers_confirm_destroy', message: 'desktop.virtual_computers_confirm_destroy_desc', confirmAction: 'confirm-destroy', label: 'desktop.virtual_computers_destroy' },
        'delete-volume': { title: 'desktop.virtual_computers_confirm_delete_volume', message: 'desktop.virtual_computers_confirm_delete_volume_desc', confirmAction: 'confirm-delete-volume', label: 'desktop.virtual_computers_volume_delete' },
        'close-workspace': { title: 'desktop.virtual_computers_confirm_close_workspace', message: 'desktop.virtual_computers_confirm_close_workspace_desc', confirmAction: 'confirm-close-workspace', label: 'desktop.virtual_computers_close_workspace' },
        'release-workspace': { title: 'desktop.virtual_computers_confirm_release_workspace', message: 'desktop.virtual_computers_confirm_release_workspace_desc', confirmAction: 'confirm-release-workspace', label: 'desktop.virtual_computers_release_workspace' },
        'cancel-workspace-job': { title: 'desktop.virtual_computers_confirm_cancel_job', message: 'desktop.virtual_computers_confirm_cancel_job_desc', confirmAction: 'confirm-cancel-workspace-job', label: 'desktop.virtual_computers_task_cancel' },
        'approve-workspace-grant': { title: 'desktop.virtual_computers_confirm_approve_grant', message: 'desktop.virtual_computers_confirm_approve_grant_desc', confirmAction: 'confirm-approve-workspace-grant', label: 'desktop.virtual_computers_approve_grant', danger: false },
        'revoke-workspace-grant': { title: 'desktop.virtual_computers_confirm_revoke_grant', message: 'desktop.virtual_computers_confirm_revoke_grant_desc', confirmAction: 'confirm-revoke-workspace-grant', label: 'desktop.virtual_computers_revoke_grant' },
        'takeover-workspace': { title: 'desktop.virtual_computers_confirm_takeover', message: 'desktop.virtual_computers_confirm_takeover_desc', confirmAction: 'confirm-takeover-workspace', label: 'desktop.virtual_computers_take_control', danger: false }
    };
    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({
            '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;'
        }[ch]));
    }

    function tx(ctx, key) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key) : key;
    }
    function icon(state, key, fallback) {
        const renderIcon = state.context && state.context.iconMarkup;
        return typeof renderIcon === 'function'
            ? renderIcon(key, fallback || '', 'vc-button-icon', 18)
            : `<span class="vc-button-fallback" aria-hidden="true">${esc(fallback || '')}</span>`;
    }

    // Both API families answer errors differently. The Virtual Computers API sends
    // {status, code, error: message}; the workspace API sends {status, error: code, message}.
    function responseError(resp, body, fallback) {
        const structured = body && typeof body === 'object';
        const workspaceShape = structured && body.message && body.error;
        const message = structured ? (body.message || body.error) : '';
        const error = new Error(String(message || fallback || ('HTTP ' + resp.status)));
        error.status = resp.status;
        error.code = structured ? String((workspaceShape ? body.error : body.code) || '') : '';
        error.body = body;
        return error;
    }
    async function readBody(resp) {
        const raw = await resp.text();
        let body = raw;
        if ((resp.headers.get('Content-Type') || '').includes('application/json')) try { body = raw ? JSON.parse(raw) : {}; } catch (_) { body = raw; }
        return body;
    }
    async function request(path, options) {
        const resp = await fetch(path, options || {});
        const raw = await resp.text();
        let body = raw;
        if ((resp.headers.get('Content-Type') || '').includes('application/json')) try { body = raw ? JSON.parse(raw) : {}; } catch (_) { body = raw; }
        if (!resp.ok || (body && typeof body === 'object' && body.error)) {
            const textBody = typeof body === 'string' ? body.trim().slice(0, 4096) : '';
            throw responseError(resp, body, textBody || resp.statusText || ('HTTP ' + resp.status));
        }
        return body;
    }
    function jsonRequest(method, payload) {
        return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload || {}) };
    }
    function errorText(state, error) {
        if (!error) return '';
        const code = error.code ? String(error.code) : '';
        if (code) {
            const key = K + 'err_' + code;
            const text = tx(state.context, key);
            if (text && text !== key) return text;
        }
        return error.message ? String(error.message) : String(error);
    }

    function fallbackTemplates() {
        return [
            { id: 'python', name: 'python', display: false },
            { id: 'desktop', name: 'desktop', display: true }
        ];
    }

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        const state = {
            host,
            context: context || {},
            activeSection: 'machines',
            machines: [],
            templates: [],
            templatesFallback: false,
            volumes: [],
            workspaces: [],
            workspaceSummaries: [],
            selectedWorkspaceId: null,
            status: null,
            selectedShot: null,
            selectedMachineId: null,
            screenshotLoading: false,
            screenshotRequestID: 0,
            detailMode: 'overview',
            modal: null,
            modalSeq: 0,
            modalReturnFocus: null,
            backdropDown: null,
            vncSession: null,
            vncMachineId: null,
            vncWorkspaceId: null,
            vncWorkspaceTakeover: false,
            workspaceControlRenewedAt: 0,
            vncExpanded: false,
            terminalSession: null,
            terminalMachineId: null,
            resourceErrors: { status: '', machines: '', templates: '', volumes: '', workspaces: '' },
            resourceLoading: { status: true, machines: true, templates: true, volumes: false, workspaces: false },
            pendingActions: new Set(),
            refreshGeneration: 0,
            clickHandler: null,
            changeHandler: null,
            pointerHandler: null,
            keyHandler: null,
            workspaceRefreshTimer: null,
            machineSnapshot: JSON.stringify([]),
            machinePollTimer: null,
            machinePollInFlight: false,
            expiryCountdownTimer: null,
            disposed: false
        };
        instances.set(windowId, state);
        state.host.innerHTML = `<div class="vc-app">
            <header class="vc-app-toolbar" data-role="toolbar"></header>
            <nav class="vc-section-tabs" data-role="sections" aria-label="${esc(tx(state.context, 'desktop.virtual_computers_title'))}"></nav>
            <div class="vc-status-region" data-role="status-region" aria-live="polite"></div>
            <main class="vc-content" data-role="content" role="tabpanel"></main>
            <div data-role="modal-host"></div>
        </div>`;
        bindActions(state);
        draw(state);
        refresh(state);
        scheduleMachineRefresh(state);
        scheduleExpiryCountdown(state);
    }

    function capabilities(state) {
        return (state.status && state.status.capabilities) || {};
    }

    function isMutable(state) {
        return !!(state.status && state.status.enabled === true && state.status.readonly !== true && state.context.readonly !== true);
    }

    function canUseVNC(state, machine) {
        return !!(machine && machine.display === true && state.status && state.status.enabled === true && state.status.readonly !== true && state.context.readonly !== true);
    }

    function canUseTerminal(state, machine) {
        return !!(machine && machine.display === false && state.status && state.status.enabled === true && state.status.readonly !== true && state.context.readonly !== true);
    }

    function isHealthy(state) {
        if (!state.status || state.status.enabled !== true) return false;
        const components = [state.status.control_plane_status, state.status.management].filter(Boolean);
        return components.length === 0 || components.every(component => component.healthy === true);
    }

    function isPending(state, key) {
        return state.pendingActions.has(key);
    }

    function setPending(state, key, pending) {
        if (pending) state.pendingActions.add(key);
        else state.pendingActions.delete(key);
    }

    function formatDate(value) {
        if (!value) return '—';
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) return String(value);
        return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
    }

    function formatDuration(seconds, context) {
        const value = Math.max(0, Number(seconds) || 0);
        const label = (key, count) => (
            context && typeof context.t === 'function' ? context.t(key, { count }) : String(count)
        );
        if (value < 60) return label('desktop.virtual_computers_duration_seconds', value);
        if (value < 3600) return label('desktop.virtual_computers_duration_minutes', Math.round(value / 60));
        if (value < 86400) return label('desktop.virtual_computers_duration_hours', Math.round(value / 3600));
        return label('desktop.virtual_computers_duration_days', Math.round(value / 86400));
    }

    function formatExpiryCountdown(expiresAt, nowMs, context) {
        const expiry = new Date(expiresAt).getTime();
        if (!Number.isFinite(expiry)) return '—';
        const parsedNow = Number(nowMs);
        const now = Number.isFinite(parsedNow) ? parsedNow : Date.now();
        let remaining = Math.max(0, Math.ceil((expiry - now) / 1000));
        const days = Math.floor(remaining / 86400);
        remaining %= 86400;
        const hours = Math.floor(remaining / 3600);
        remaining %= 3600;
        const minutes = Math.floor(remaining / 60);
        const seconds = remaining % 60;
        const clock = [hours, minutes, seconds].map(value => String(value).padStart(2, '0')).join(':');
        if (days > 0) {
            return context && typeof context.t === 'function'
                ? context.t('desktop.virtual_computers_expiry_days', { days, clock })
                : `${days} ${clock}`;
        }
        return hours > 0 ? clock : `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`;
    }

    function expiryCountdownMarkup(state, machine, className) {
        const c = state.context;
        const classes = className ? ` ${className}` : '';
        if (machine.persistent) return `<span class="vc-expiry-countdown${classes}">${esc(tx(c, 'desktop.virtual_computers_unlimited'))}</span>`;
        const expiresAt = machine.expires_at || '';
        const value = formatExpiryCountdown(expiresAt, undefined, c);
        const label = tx(c, 'desktop.virtual_computers_expires');
        return `<span class="vc-expiry-countdown${classes}" data-expiry-countdown data-expires-at="${esc(expiresAt)}" data-expiry-label="${esc(label)}" title="${esc(formatDate(expiresAt))}" aria-label="${esc(label)}: ${esc(value)}">${esc(value)}</span>`;
    }

    function updateExpiryCountdowns(state) {
        if (!state.host || state.disposed) return;
        const now = Date.now();
        state.host.querySelectorAll('[data-expiry-countdown]').forEach(node => {
            const value = formatExpiryCountdown(node.dataset.expiresAt, now, state.context);
            node.textContent = value;
            node.setAttribute('aria-label', `${node.dataset.expiryLabel || ''}: ${value}`);
        });
    }

    function scheduleExpiryCountdown(state) {
        if (state.expiryCountdownTimer) {
            clearTimeout(state.expiryCountdownTimer);
            state.expiryCountdownTimer = null;
        }
        if (state.disposed) return;
        state.expiryCountdownTimer = setTimeout(() => {
            state.expiryCountdownTimer = null;
            if (isMachinePollingVisible(state)) updateExpiryCountdowns(state);
            scheduleExpiryCountdown(state);
        }, expiryCountdownIntervalMs);
    }

    function formatBytes(value, context) {
        const bytes = Math.max(0, Number(value) || 0);
        const label = (key, count) => (
            context && typeof context.t === 'function' ? context.t(key, { count }) : String(count)
        );
        if (!bytes) return '—';
        if (bytes < 1024) return label('desktop.bytes', bytes);
        if (bytes < 1024 * 1024) return label('desktop.kib', Math.round(bytes / 1024));
        if (bytes < 1024 * 1024 * 1024) return label('desktop.mib', (bytes / (1024 * 1024)).toFixed(1));
        return label('desktop.gib', (bytes / (1024 * 1024 * 1024)).toFixed(1));
    }

    function escapeAttr(value) {
        const text = String(value == null ? '' : value);
        return typeof CSS !== 'undefined' && typeof CSS.escape === 'function' ? CSS.escape(text) : text.replace(/["\\]/g, '\\$&');
    }

    // A selector that finds the same control again after its container was re-rendered.
    function stableSelector(element) {
        if (!element || !element.dataset || !element.dataset.action) return '';
        let selector = `[data-action="${escapeAttr(element.dataset.action)}"]`;
        ['id', 'section', 'jobId', 'grantId', 'ttl', 'machineId', 'workspaceId'].forEach(key => {
            if (element.dataset[key]) selector += `[data-${key.replace(/[A-Z]/g, letter => '-' + letter.toLowerCase())}="${escapeAttr(element.dataset[key])}"]`;
        });
        return selector;
    }

    function focusedSelector(state) {
        const doc = state.host.ownerDocument;
        const active = doc && doc.activeElement;
        if (!active || active === doc.body || !state.host.contains(active)) return '';
        if (active.closest('[data-role="modal-host"]')) return '';
        return stableSelector(active);
    }

    function restoreFocus(state, selector) {
        if (!selector) return;
        const doc = state.host.ownerDocument;
        const active = doc.activeElement;
        if (active && active !== doc.body && state.host.contains(active)) return;
        let target = null;
        try { target = state.host.querySelector(selector); } catch (_) { target = null; }
        if (target && typeof target.focus === 'function') target.focus({ preventScroll: true });
    }

    function draw(state) {
        if (state.disposed) return;
        const toolbar = state.host.querySelector('[data-role="toolbar"]');
        const sections = state.host.querySelector('[data-role="sections"]');
        const statusRegion = state.host.querySelector('[data-role="status-region"]');
        const content = state.host.querySelector('[data-role="content"]');
        const modal = state.host.querySelector('[data-role="modal-host"]');
        if (!toolbar || !sections || !statusRegion || !content || !modal) return;
        const focusSelector = focusedSelector(state);
        const scrolls = content.dataset.section === state.activeSection
            ? ['.vc-section-page', '.vc-preview', '.vc-list'].map(selector => [selector, (content.querySelector(selector) || {}).scrollTop || 0])
            : [];
        const agentRequest = content.querySelector('[data-role="agent-request"]');
        const agentRequestFocused = document.activeElement === agentRequest;
        state.agentRequestDraft = agentRequest?.value ?? state.agentRequestDraft ?? '';
        state.agentWorkspaceId = content.querySelector('[data-role="agent-workspace"]')?.value ?? state.agentWorkspaceId ?? '';
        toolbar.innerHTML = toolbarPane(state);
        sections.innerHTML = sectionTabs(state);
        statusRegion.innerHTML = statusPane(state);
        statusRegion.hidden = statusRegion.innerHTML.trim() === '';

        let liveMount = null;
        let workspaceLiveMount = null;
        if (state.activeSection === 'machines' && state.detailMode === 'vnc') {
            liveMount = content.querySelector('[data-role="vnc-mount"]');
        } else if (state.activeSection === 'machines' && state.detailMode === 'terminal') {
            liveMount = content.querySelector('[data-role="terminal-mount"]');
        } else if (state.activeSection === 'workspaces' && state.vncWorkspaceId) {
            workspaceLiveMount = content.querySelector('[data-role="workspace-vnc-mount"]');
        }
        if (liveMount) {
            const list = content.querySelector('[data-role="machine-list"]');
            if (list) list.innerHTML = machineList(state);
        } else if (state.activeSection === 'workspaces') {
            content.innerHTML = workspacePane(state);
            const nextAgentRequest = content.querySelector('[data-role="agent-request"]');
            if (agentRequest && nextAgentRequest) {
                nextAgentRequest.replaceWith(agentRequest);
                if (agentRequestFocused) agentRequest.focus({ preventScroll: true });
            }
            if (workspaceLiveMount) {
                const replacement = content.querySelector('[data-role="workspace-vnc-mount"]');
                if (replacement) replacement.replaceWith(workspaceLiveMount);
                else disconnectVNC(state);
            }
        } else if (state.activeSection === 'volumes') {
            content.innerHTML = volumePane(state);
        } else {
            content.innerHTML = machinesPane(state);
        }
        content.dataset.section = state.activeSection;
        content.setAttribute('aria-labelledby', `vc-tab-${state.activeSection}`);
        scrolls.forEach(([selector, top]) => {
            const node = top ? content.querySelector(selector) : null;
            if (node) node.scrollTop = top;
        });
        drawModal(state, modal);
        applyVNCLayoutState(state);
        restoreFocus(state, focusSelector);
    }

    // The dialog is rebuilt only when a new dialog was opened (new renderKey). Every other
    // redraw, such as a poll, patches the error and busy state in place so typed input,
    // focus and the caret survive.
    function drawModal(state, host) {
        if (!state.modal) {
            if (host.dataset.renderKey || host.firstChild) {
                host.innerHTML = '';
                delete host.dataset.renderKey;
            }
            return;
        }
        const key = String(state.modal.renderKey);
        if (host.dataset.renderKey !== key) {
            host.innerHTML = modalPane(state);
            host.dataset.renderKey = key;
            const target = host.querySelector('[data-autofocus]') || host.querySelector('[data-role="modal"] button');
            if (target && typeof target.focus === 'function') target.focus();
            return;
        }
        patchModal(state);
    }

    function patchModal(state) {
        const modal = state.modal;
        const host = state.host.querySelector('[data-role="modal-host"]');
        if (!modal || !host) return;
        const error = host.querySelector('[data-role="modal-error"]');
        if (error) {
            error.textContent = modal.error || '';
            error.hidden = !modal.error;
        }
        const progress = host.querySelector('[data-role="modal-progress"]');
        if (progress) progress.hidden = !modal.pending;
        host.querySelectorAll('[data-role="modal-submit"]').forEach(button => {
            button.disabled = !!modal.pending;
            if (modal.pending) button.setAttribute('aria-busy', 'true');
            else button.removeAttribute('aria-busy');
        });
    }

    function setModalError(state, message) {
        if (!state.modal) return;
        state.modal.error = message || '';
        patchModal(state);
    }

    function toolbarPane(state) {
        const c = state.context;
        const enabled = state.status && state.status.enabled === true;
        const readonly = enabled && !isMutable(state);
        const healthy = isHealthy(state);
        const stateKey = !state.status
            ? 'desktop.loading'
            : readonly
            ? 'desktop.virtual_computers_readonly'
            : healthy
                ? 'desktop.virtual_computers_health_operational'
                : enabled
                    ? 'desktop.virtual_computers_health_degraded'
                    : 'desktop.virtual_computers_state_disabled';
        const tone = readonly ? 'is-readonly' : healthy ? 'is-ok' : enabled ? 'is-warning' : '';
        const refreshing = state.resourceLoading.status || state.resourceLoading.machines;
        return `<div class="vc-heading">
                <span class="vc-heading-icon" aria-hidden="true">${icon(state, 'server', '▣')}</span>
                <div><h2>${esc(tx(c, 'desktop.virtual_computers_title'))}</h2>
                <span class="vc-status ${tone}"><span class="vc-status-dot"></span>${esc(tx(c, stateKey))}</span></div>
            </div>
            <div class="vc-actions vc-toolbar-actions">
                <button type="button" class="vc-btn vc-icon-label" data-action="refresh" ${refreshing ? 'disabled aria-busy="true"' : ''}>${icon(state, 'refresh', '↻')}<span>${esc(tx(c, 'desktop.virtual_computers_refresh'))}</span></button>
                <button type="button" class="vc-btn vc-icon-label" data-action="config">${icon(state, 'settings', '⚙')}<span>${esc(tx(c, 'desktop.virtual_computers_open_config'))}</span></button>
                <button type="button" class="vc-btn vc-primary vc-icon-label" data-action="new-agent-job" title="${esc(tx(c, capabilities(state).agent_control ? 'desktop.virtual_computers_agent_workspaces_hint' : 'desktop.virtual_computers_state_disabled'))}" ${capabilities(state).agent_control && isMutable(state) ? '' : 'disabled'}>${icon(state, 'agent', 'A')}<span>${esc(tx(c, 'desktop.virtual_computers_task_start'))}</span></button>
                <button type="button" class="vc-btn vc-primary vc-icon-label" data-action="new-machine" ${isMutable(state) && !isPending(state, 'launch') ? '' : 'disabled'}>${icon(state, 'plus', '+')}<span>${esc(tx(c, 'desktop.virtual_computers_new'))}</span></button>
            </div>`;
    }

    function sectionTabs(state) {
        const c = state.context;
        const tabs = [
            { id: 'machines', key: 'desktop.virtual_computers_machines', icon: 'server' }
        ];
        if (capabilities(state).agent_control) tabs.push({ id: 'workspaces', key: 'desktop.virtual_computers_agent_workspaces', icon: 'agent' });
        if (capabilities(state).volumes) tabs.push({ id: 'volumes', key: 'desktop.virtual_computers_volumes', icon: 'archive' });
        return `<div class="vc-tab-track" role="tablist">${tabs.map(tab => {
            const active = state.activeSection === tab.id;
            const count = tab.id === 'machines' ? state.machines.length : tab.id === 'workspaces' ? state.workspaces.length : state.volumes.length;
            return `<button type="button" id="vc-tab-${tab.id}" class="vc-section-tab ${active ? 'active' : ''}" role="tab" aria-selected="${active ? 'true' : 'false'}" tabindex="${active ? '0' : '-1'}" data-action="section" data-section="${tab.id}">${icon(state, tab.icon, '')}<span>${esc(tx(c, tab.key))}</span><span class="vc-tab-count">${count}</span></button>`;
        }).join('')}</div>`;
    }

    function statusPane(state) {
        const c = state.context;
        if (state.resourceErrors.status) {
            return `<div class="vc-banner is-error"><span>${esc(tx(c, 'desktop.virtual_computers_section_error'))} ${esc(state.resourceErrors.status)}</span><button type="button" class="vc-text-button" data-action="retry-resource" data-resource="status">${esc(tx(c, 'desktop.retry'))}</button></div>`;
        }
        if (!state.status || state.resourceLoading.status) return '';
        if (!state.status.enabled) {
            return `<div class="vc-banner"><span>${esc(tx(c, 'desktop.virtual_computers_state_disabled'))}</span><button type="button" class="vc-text-button" data-action="config">${esc(tx(c, 'desktop.virtual_computers_open_config'))}</button></div>`;
        }
        if (!isHealthy(state)) {
            return `<div class="vc-banner is-warning"><span>${esc(tx(c, 'desktop.virtual_computers_health_degraded'))}</span><button type="button" class="vc-text-button" data-action="config">${esc(tx(c, 'desktop.virtual_computers_open_config'))}</button></div>`;
        }
        if (!isMutable(state)) return `<div class="vc-banner is-info">${esc(tx(c, 'desktop.virtual_computers_readonly'))}</div>`;
        return '';
    }

    function skeletonRows() {
        return `<div class="vc-skeleton-stack" aria-hidden="true"><span></span><span></span><span></span></div>`;
    }

    function resourceErrorPane(state, resource) {
        const message = state.resourceErrors[resource];
        if (!message) return '';
        return `<div class="vc-empty-state is-error"><strong>${esc(tx(state.context, 'desktop.virtual_computers_section_error'))}</strong><p>${esc(message)}</p><button type="button" class="vc-btn" data-action="retry-resource" data-resource="${esc(resource)}">${esc(tx(state.context, 'desktop.retry'))}</button></div>`;
    }

    function machinesPane(state) {
        return `<div class="vc-machines-layout">
            <aside class="vc-list" data-role="machine-list" aria-label="${esc(tx(state.context, 'desktop.virtual_computers_machines'))}">${machineList(state)}</aside>
            <section class="vc-preview" data-role="preview">${detailPane(state)}</section>
        </div>`;
    }

    function machineList(state) {
        const c = state.context;
        if (state.resourceLoading.machines && !state.machines.length) return skeletonRows();
        if (state.resourceErrors.machines && !state.machines.length) return resourceErrorPane(state, 'machines');
        if (!state.status) return `<div class="vc-empty">${esc(tx(c, 'desktop.loading'))}</div>`;
        if (!state.status.enabled) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_state_disabled'))}</div>`;
        if (!state.machines.length) {
            return `<div class="vc-empty-state"><span class="vc-empty-icon">${icon(state, 'server', '▣')}</span><strong>${esc(tx(c, 'desktop.virtual_computers_empty'))}</strong><button type="button" class="vc-btn vc-primary" data-action="new-machine" ${isMutable(state) ? '' : 'disabled'}>${esc(tx(c, 'desktop.virtual_computers_new'))}</button></div>`;
        }
        return state.machines.map(machine => {
            const id = machine.id || '';
            const active = state.selectedMachineId === id;
            return `<button type="button" class="vc-machine ${active ? 'is-active' : ''}" data-action="select-machine" data-id="${esc(id)}" aria-current="${active ? 'true' : 'false'}">
                <span class="vc-machine-icon" aria-hidden="true">${icon(state, machine.display ? 'monitor' : 'server', '▣')}</span>
                <span class="vc-machine-main"><strong>${esc(machineName(machine))}</strong><span>${esc(templateLabel(state, machine.template))}</span>${workspaceBadge(state, machine)}</span>
                <span class="vc-machine-state"><span class="vc-machine-status"><span class="vc-state-dot" data-state="${esc(machine.status || '')}"></span>${esc(statusLabel(state, machine.status))}</span>${expiryCountdownMarkup(state, machine, 'vc-machine-expiry')}</span>
            </button>`;
        }).join('');
    }

    function detailPane(state) {
        const c = state.context;
        const placeholderArt = '<img class="vc-placeholder-art" src="/img/virtual-computers/placeholder.png" width="1254" height="1254" alt="" aria-hidden="true" draggable="false">';
        const machine = state.machines.find(item => item.id === state.selectedMachineId);
        if (!machine) {
            const loading = state.resourceLoading.machines && !state.machines.length;
            return `<div class="vc-empty-state vc-empty-detail">${placeholderArt}<strong>${esc(tx(c, loading ? 'desktop.loading' : 'desktop.virtual_computers_select_machine'))}</strong><p>${esc(tx(c, 'desktop.virtual_computers_status'))}</p></div>`;
        }
        const mutable = isMutable(state);
        const caps = capabilities(state);
        const owned = !!machine.workspace_id;
        const ports = Array.isArray(machine.web_ports) ? machine.web_ports : [];
        const portLinks = ports.length ? ports.map(port => `<a class="vc-link" href="/api/virtual-computers/machines/${encodeURIComponent(machine.id)}/web/${Number(port)}/" target="_blank" rel="noopener">${icon(state, 'external', '↗')} ${esc(String(port))}</a>`).join('') : '—';
        let viewer = `<div class="vc-machine-hero">${placeholderArt}<p>${esc(machine.display ? tx(c, 'desktop.virtual_computers_vnc_live') : tx(c, 'desktop.virtual_computers_headless'))}</p></div>`;
        if (state.detailMode === 'screenshot') {
            if (state.screenshotLoading) viewer = skeletonRows();
            else if (state.selectedShot) viewer = `<img class="vc-shot" src="data:${esc(state.selectedShot.mime_type || 'image/png')};base64,${esc(state.selectedShot.data_base64 || '')}" alt="${esc(tx(c, 'desktop.virtual_computers_screenshot'))}">`;
        } else if (state.detailMode === 'vnc') {
            viewer = `<div class="vc-vnc-placeholder">${esc(tx(c, 'desktop.virtual_computers_vnc_connecting'))}</div>`;
        } else if (state.detailMode === 'terminal') {
            viewer = `<div class="vc-terminal-placeholder">${esc(tx(c, 'desktop.virtual_computers_terminal_connecting'))}</div>`;
        }
        const manage = [];
        if (mutable) {
            manage.push(actionButton(state, 'rename-machine', machine.id, 'desktop.virtual_computers_rename'));
            const extendChoices = machine.persistent ? [] : extendChoicesSeconds(state);
            if (extendChoices.length) {
                manage.push(`<span class="vc-button-group" role="group" aria-label="${esc(tx(c, 'desktop.virtual_computers_extend_group'))}">${extendChoices.map(seconds => `<button type="button" class="vc-btn vc-btn-sm" data-action="extend-machine" data-id="${esc(machine.id)}" data-ttl="${seconds}">${esc(txv(c, 'desktop.virtual_computers_extend_minutes', { count: Math.round(seconds / 60) }))}</button>`).join('')}</span>`);
            }
            if (caps.volumes) manage.push(actionButton(state, 'save-machine', machine.id, 'desktop.virtual_computers_save_to_volume'));
            if (caps.agent_control && !owned) manage.push(actionButton(state, 'assign-agent', machine.id, 'desktop.virtual_computers_assign_agent'));
            manage.push(actionButton(state, 'upload-file', machine.id, 'desktop.virtual_computers_upload'));
            manage.push(actionButton(state, 'download-file', machine.id, 'desktop.virtual_computers_download'));
            if (owned) {
                manage.push(actionButton(state, 'destroy', machine.id, 'desktop.virtual_computers_destroy', { className: 'danger', disabled: true, title: tx(c, 'desktop.virtual_computers_destroy_owned') }));
                manage.push(actionButton(state, 'open-workspace', machine.workspace_id, 'desktop.virtual_computers_open_workspace'));
            } else {
                manage.push(actionButton(state, 'destroy', machine.id, 'desktop.virtual_computers_destroy', { className: 'danger' }));
            }
        }
        return `<article class="vc-machine-detail">
            <header class="vc-detail-header"><div><span class="vc-eyebrow">${esc(templateLabel(state, machine.template))}</span><h3>${esc(machineName(machine))}</h3><span class="vc-detail-status"><span class="vc-state-dot" data-state="${esc(machine.status || '')}"></span>${esc(statusLabel(state, machine.status))}</span>${workspaceBadge(state, machine)}</div>
            <div class="vc-actions vc-detail-actions">
                ${machine.display === true ? `<button type="button" class="vc-btn vc-icon-label" data-action="screenshot" data-id="${esc(machine.id)}">${icon(state, 'image', '▧')}<span>${esc(tx(c, 'desktop.virtual_computers_screenshot'))}</span></button>` : ''}
                ${canUseVNC(state, machine) ? `<button type="button" class="vc-btn vc-primary vc-icon-label" data-action="vnc" data-id="${esc(machine.id)}">${icon(state, 'monitor', '▣')}<span>${esc(tx(c, 'desktop.virtual_computers_vnc_live'))}</span></button>` : ''}
                ${canUseTerminal(state, machine) ? `<button type="button" class="vc-btn vc-primary vc-icon-label" data-action="terminal" data-id="${esc(machine.id)}">${icon(state, 'terminal', '>_')}<span>${esc(tx(c, 'desktop.virtual_computers_terminal'))}</span></button>` : ''}
            </div></header>
            ${manage.length ? `<div class="vc-actions vc-manage-actions" role="toolbar" aria-label="${esc(tx(c, 'desktop.virtual_computers_manage'))}">${manage.join('')}</div>` : ''}
            <dl class="vc-meta-grid">
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_template'))}</dt><dd>${esc(templateLabel(state, machine.template))}</dd></div>
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_runtime'))}</dt><dd>${esc(machine.persistent ? tx(c, 'desktop.virtual_computers_unlimited') : formatDuration(machine.ttl_seconds, c))}</dd></div>
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_expires'))}</dt><dd class="vc-expiry-detail">${expiryCountdownMarkup(state, machine, 'vc-expiry-detail-countdown')}${machine.persistent ? '' : `<span>${esc(formatDate(machine.expires_at))}</span>`}</dd></div>
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_display'))}</dt><dd>${esc(machine.display ? tx(c, 'desktop.on') : tx(c, 'desktop.off'))}</dd></div>
                ${machine.volume_id ? `<div><dt>${esc(tx(c, 'desktop.virtual_computers_volume'))}</dt><dd>${esc(volumeName(state, machine.volume_id))}</dd></div>` : ''}
                <div class="vc-meta-wide"><dt>${esc(tx(c, 'desktop.virtual_computers_web_ports'))}</dt><dd class="vc-machine-links">${portLinks}</dd></div>
            </dl>
            <div class="vc-detail-viewer">${viewer}</div>
        </article>`;
    }

    // Helpers of the detail and volume panes live between detailPane and workspacePane
    // so the regression harness, which loads that range alone, can evaluate them.
    function txv(ctx, key, vars) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key, vars || {}) : key;
    }

    function translatedOr(ctx, key, fallback) {
        const text = tx(ctx, key);
        return text && text !== key ? text : fallback;
    }

    function humanize(value) {
        const text = String(value == null ? '' : value).replace(/[_-]+/g, ' ').trim();
        return text ? text.charAt(0).toUpperCase() + text.slice(1) : '';
    }

    function shortId(id) {
        const text = String(id == null ? '' : id);
        return text.length > 14 ? text.slice(0, 12) + '…' : text;
    }

    function machineName(machine) {
        return (machine && (machine.label || machine.name || machine.id)) || '';
    }

    function templateLabel(state, id, name) {
        const raw = String(id || '').trim();
        if (!raw) return '—';
        const key = 'desktop.virtual_computers_template_' + raw.toLowerCase().replace(/[^a-z0-9_]+/g, '_');
        const fallback = name && name !== raw ? name : humanize(raw);
        return translatedOr(state.context, key, fallback);
    }

    function statusLabel(state, status) {
        const raw = String(status || '').trim();
        if (!raw) return '—';
        const key = 'desktop.virtual_computers_status_' + raw.toLowerCase().replace(/[^a-z0-9_]+/g, '_');
        return translatedOr(state.context, key, humanize(raw));
    }

    function positiveNumber(value, fallback) {
        const number = Number(value);
        return Number.isFinite(number) && number > 0 ? number : fallback;
    }

    // Older backends send no limits; the defaults match the previous hard-coded UI.
    function limits(state) {
        const source = (state.status && state.status.limits) || {};
        return {
            maxTTL: positiveNumber(source.max_ttl_seconds, 900),
            defaultTTL: positiveNumber(source.default_ttl_seconds, 600),
            volumeMaxTTL: positiveNumber(source.volume_max_ttl_seconds, 2592000),
            volumeQuotaMB: positiveNumber(source.volume_quota_mb, 0),
            maxForks: positiveNumber(source.max_forks, 3)
        };
    }

    function extendChoicesSeconds(state) {
        const max = limits(state).maxTTL;
        const choices = [300, 600, 900].filter(seconds => seconds <= max);
        return choices.length ? choices : [max];
    }

    function volumeName(state, id) {
        const volume = (state.volumes || []).find(item => item.id === id);
        return volume && volume.name ? volume.name : shortId(id);
    }

    function workspaceBadge(state, machine) {
        if (!machine || !machine.workspace_id) return '';
        const adopted = machine.workspace_origin === 'adopted';
        return `<span class="vc-badge ${adopted ? 'is-assigned' : 'is-workspace'}">${esc(tx(state.context, adopted ? 'desktop.virtual_computers_badge_assigned' : 'desktop.virtual_computers_badge_workspace'))}</span>`;
    }

    function actionButton(state, action, id, labelKey, extra) {
        const options = extra || {};
        return `<button type="button" class="vc-btn vc-btn-sm ${options.className || ''}" data-action="${esc(action)}"${id ? ` data-id="${esc(id)}"` : ''}${options.disabled ? ' disabled' : ''}${options.title ? ` title="${esc(options.title)}"` : ''}>${esc(tx(state.context, labelKey))}</button>`;
    }

    function volumeAvailable(volume) {
        return !volume.availability || volume.availability === 'available';
    }

    function volumeFormatLabel(state, volume) {
        const raw = String(volume.format || '').trim();
        if (!raw) return '';
        return translatedOr(state.context, 'desktop.virtual_computers_badge_format_' + raw, humanize(raw));
    }

    function volumeRetentionText(state, volume) {
        const c = state.context;
        if (volume.persistent) return tx(c, 'desktop.virtual_computers_badge_permanent');
        if (!volume.expires_at) return '';
        const remaining = (Date.parse(volume.expires_at) - Date.now()) / 1000;
        if (!Number.isFinite(remaining)) return '';
        if (remaining <= 0) return tx(c, 'desktop.virtual_computers_badge_expired');
        return txv(c, 'desktop.virtual_computers_badge_expires', { time: formatDuration(remaining, c) });
    }

    function volumeOptionLabel(state, volume) {
        return [volume.name || shortId(volume.id), volumeFormatLabel(state, volume), volumeRetentionText(state, volume)].filter(Boolean).join(' · ');
    }

    function volumeBadge(tone, text) {
        return `<span class="vc-badge ${tone}">${esc(text)}</span>`;
    }

    function volumeAttachments(state, volume) {
        const c = state.context;
        const attached = Array.isArray(volume.attached_to) ? volume.attached_to : [];
        return attached.map(item => {
            if (item && item.kind === 'machine') {
                const machine = state.machines.find(entry => entry.id === item.id);
                return txv(c, 'desktop.virtual_computers_attached_machine', { name: machine ? machineName(machine) : shortId(item.id) });
            }
            return txv(c, 'desktop.virtual_computers_attached_workspace', { id: shortId(item && item.id) });
        });
    }

    function volumeCard(state, volume) {
        const c = state.context;
        const id = volume.id || '';
        const mutable = isMutable(state);
        const retentionReady = capabilities(state).volume_retention === true;
        const available = volumeAvailable(volume);
        const attached = volumeAttachments(state, volume);
        const badges = [];
        const retentionText = volumeRetentionText(state, volume);
        if (retentionText) badges.push(volumeBadge(volume.persistent ? 'is-permanent' : 'is-expiring', retentionText));
        if (volume.locked) badges.push(volumeBadge('is-locked', tx(c, 'desktop.virtual_computers_badge_locked')));
        const formatText = volumeFormatLabel(state, volume);
        if (formatText) badges.push(volumeBadge('is-format', formatText));
        if (volume.availability === 'previous_store') badges.push(volumeBadge('is-warning', tx(c, 'desktop.virtual_computers_badge_previous_store')));
        else if (!available) badges.push(volumeBadge('is-warning', tx(c, 'desktop.virtual_computers_badge_unavailable')));
        const quotaBytes = (Number(volume.quota_mb) || limits(state).volumeQuotaMB) * 1048576;
        const used = Number(volume.used_bytes) > 0 ? Number(volume.used_bytes) : (Number(volume.size_bytes) || 0);
        const usedText = volume.used_bytes > 0 ? formatBytes(volume.used_bytes, c) : (volume.size_bytes > 0 ? formatBytes(volume.size_bytes, c) : txv(c, 'desktop.bytes', { count: 0 }));
        let percent = quotaBytes > 0 ? Math.min(100, Math.round(used / quotaBytes * 100)) : 0;
        if (used > 0 && percent === 0) percent = 1;
        const usageText = quotaBytes > 0 ? txv(c, 'desktop.virtual_computers_volume_usage', { used: usedText, quota: formatBytes(quotaBytes, c) }) : txv(c, 'desktop.virtual_computers_volume_used', { used: usedText });
        const usageBar = quotaBytes > 0
            ? `<div class="vc-usage-bar ${percent >= 90 ? 'is-high' : ''}" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${percent}" aria-valuetext="${esc(usageText)}" aria-label="${esc(tx(c, 'desktop.virtual_computers_volume_usage_label'))}"><span style="width:${percent}%"></span></div>`
            : '';
        const files = Number.isFinite(Number(volume.files)) && volume.files !== undefined ? `<span>${esc(txv(c, 'desktop.virtual_computers_volume_files', { count: Number(volume.files) }))}</span>` : '';
        const retentionBlocked = !retentionReady ? tx(c, 'desktop.virtual_computers_volume_needs_repair') : !available ? tx(c, 'desktop.virtual_computers_volume_previous_store') : '';
        const deleteBlocked = volume.locked ? tx(c, 'desktop.virtual_computers_volume_delete_locked') : attached.length ? tx(c, 'desktop.virtual_computers_volume_delete_in_use') : '';
        const label = volume.name || shortId(id);
        const actions = mutable ? `<div class="vc-actions vc-volume-actions">
                ${actionButton(state, 'rename-volume', id, 'desktop.virtual_computers_rename')}
                ${actionButton(state, 'toggle-volume-lock', id, volume.locked ? 'desktop.virtual_computers_volume_unlock' : 'desktop.virtual_computers_volume_lock', { disabled: !!retentionBlocked, title: retentionBlocked })}
                ${actionButton(state, 'volume-retention', id, 'desktop.virtual_computers_volume_retention', { disabled: !!retentionBlocked, title: retentionBlocked })}
                ${actionButton(state, 'launch-with-volume', id, 'desktop.virtual_computers_volume_launch', { disabled: !available, title: available ? '' : tx(c, 'desktop.virtual_computers_volume_previous_store') })}
                ${actionButton(state, 'copy-volume-id', id, 'desktop.virtual_computers_volume_copy_id')}
                ${actionButton(state, 'delete-volume', id, 'desktop.virtual_computers_volume_delete', { className: 'danger', disabled: !!deleteBlocked, title: deleteBlocked })}
            </div>` : '';
        return `<article class="vc-volume-card" data-volume-id="${esc(id)}" aria-label="${esc(label)}">
            <header class="vc-volume-head"><div class="vc-volume-heading"><strong class="vc-volume-name">${esc(label)}</strong>${volume.name ? `<span class="vc-volume-id" title="${esc(id)}">${esc(shortId(id))}</span>` : ''}</div><div class="vc-badges">${badges.join('')}</div></header>
            <div class="vc-usage">${usageBar}<div class="vc-usage-text"><span>${esc(usageText)}</span>${files}</div></div>
            ${attached.length ? `<p class="vc-volume-attached">${esc(txv(c, 'desktop.virtual_computers_volume_attached', { list: attached.join(', ') }))}</p>` : ''}
            ${actions}
        </article>`;
    }

    function volumePane(state) {
        const c = state.context;
        if (state.resourceLoading.volumes && !state.volumes.length) return `<section class="vc-section-page">${skeletonRows()}</section>`;
        if (state.resourceErrors.volumes && !state.volumes.length) return `<section class="vc-section-page">${resourceErrorPane(state, 'volumes')}</section>`;
        const mutable = isMutable(state);
        const banner = capabilities(state).volume_retention === true ? '' : `<div class="vc-banner is-warning vc-volume-banner" role="status"><span>${esc(tx(c, 'desktop.virtual_computers_volume_banner_repair'))}</span><button type="button" class="vc-text-button" data-action="config">${esc(tx(c, 'desktop.virtual_computers_open_config'))}</button></div>`;
        const tools = mutable ? `<div class="vc-actions"><button type="button" class="vc-btn vc-primary" data-action="create-volume">${icon(state, 'plus', '+')}${esc(tx(c, 'desktop.virtual_computers_volume_create'))}</button><button type="button" class="vc-btn" data-action="import-volume">${esc(tx(c, 'desktop.virtual_computers_volume_import'))}</button></div>` : '';
        const cards = state.volumes.length
            ? `<div class="vc-volume-grid">${state.volumes.map(volume => volumeCard(state, volume)).join('')}</div>`
            : `<div class="vc-empty-state"><span class="vc-empty-icon">${icon(state, 'archive', '▤')}</span><strong>${esc(tx(c, 'desktop.virtual_computers_volumes_empty'))}</strong></div>`;
        return `<section class="vc-section-page"><header class="vc-section-header"><div><span class="vc-eyebrow">${esc(tx(c, 'desktop.virtual_computers_title'))}</span><h3>${esc(tx(c, 'desktop.virtual_computers_volumes'))}</h3></div>${tools}</header>${banner}${state.resourceErrors.volumes ? `<div class="vc-inline-error" role="alert">${esc(state.resourceErrors.volumes)}</div>` : ''}${cards}</section>`;
    }

    function workspacePane(state) {
        if (!window.VirtualComputersWorkspaces) return '';
        return window.VirtualComputersWorkspaces.renderPane(state, { esc, tx, icon, skeletonRows, resourceErrorPane, formatExpiryCountdown, formatDate });
    }

    function launchTTLChoices(state) {
        const c = state.context;
        const max = limits(state).maxTTL;
        const all = [[300, formatDuration(300, c)], [600, formatDuration(600, c)], [900, formatDuration(900, c)]];
        const choices = all.filter(item => item[0] <= max);
        if (!choices.length || max > 900) choices.push([max, formatDuration(max, c)]);
        const wanted = Math.min(limits(state).defaultTTL, max);
        let selected = choices[0][0];
        choices.forEach(item => { if (item[0] <= wanted) selected = item[0]; });
        return { choices, selected };
    }

    function volumeRetentionChoices(state) {
        const c = state.context;
        const max = limits(state).volumeMaxTTL;
        const all = [[86400, formatDuration(86400, c)], [604800, formatDuration(604800, c)], [2592000, formatDuration(2592000, c)]];
        const choices = all.filter(item => item[0] <= max);
        if (!choices.length) choices.push([max, formatDuration(max, c)]);
        return choices;
    }

    function modalShell(state, title, body, footer) {
        const c = state.context;
        const modal = state.modal;
        return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal" role="dialog" aria-modal="true" aria-labelledby="vc-modal-title"><header><h3 id="vc-modal-title">${esc(title)}</h3><button type="button" class="vc-icon-btn" data-action="close-modal" aria-label="${esc(tx(c, 'desktop.close'))}">${icon(state, 'x', '×')}</button></header><div class="vc-modal-error" role="alert" data-role="modal-error" ${modal.error ? '' : 'hidden'}>${esc(modal.error || '')}</div>${body}<p class="vc-form-note vc-modal-progress" role="status" data-role="modal-progress" ${modal.pending ? '' : 'hidden'}>${esc(tx(c, modal.progressKey || 'desktop.virtual_computers_working'))}</p><div class="vc-actions">${footer}</div></div></div>`;
    }

    function modalButtons(state, submitAction, submitLabelKey, options) {
        const c = state.context;
        const opts = options || {};
        const pending = state.modal && state.modal.pending;
        return `<button type="button" class="vc-btn" data-action="close-modal" ${opts.cancelFocus ? 'data-autofocus' : ''}>${esc(tx(c, 'desktop.cancel'))}</button><button type="button" class="vc-btn ${opts.danger ? 'danger' : 'vc-primary'}" data-action="${esc(submitAction)}" data-role="modal-submit"${opts.confirmAction ? ` data-confirm-action="${esc(opts.confirmAction)}"` : ''} ${pending ? 'disabled aria-busy="true"' : ''}>${esc(tx(c, submitLabelKey))}</button>`;
    }

    function launchModal(state, modal) {
        const c = state.context;
        const caps = capabilities(state);
        const templates = state.templates.length ? state.templates : fallbackTemplates();
        const options = templates.map(template => `<option value="${esc(template.id || template.name)}">${esc(templateLabel(state, template.id || template.name, template.name))}</option>`).join('');
        const volumes = state.volumes.filter(volumeAvailable).map(volume => `<option value="${esc(volume.id)}" ${volume.id === modal.volumeId ? 'selected' : ''}>${esc(volumeOptionLabel(state, volume))}</option>`).join('');
        const ttl = launchTTLChoices(state);
        const ttlOptions = ttl.choices.map(item => `<option value="${item[0]}" ${item[0] === ttl.selected ? 'selected' : ''}>${esc(item[1])}</option>`).join('');
        const persistent = caps.persistent ? `<label class="vc-check"><input type="checkbox" data-role="launch-persistent"><span>${esc(tx(c, 'desktop.virtual_computers_launch_persistent'))}</span></label>` : '';
        const internetAllowed = caps.internet === true;
        const networkOptions = internetAllowed ? `<option value="internet" selected>${esc(tx(c, 'desktop.virtual_computers_network_internet'))}</option><option value="offline">${esc(tx(c, 'desktop.virtual_computers_network_offline'))}</option>` : `<option value="offline">${esc(tx(c, 'desktop.virtual_computers_network_offline'))}</option>`;
        const networkField = `<label><span>${esc(tx(c, 'desktop.virtual_computers_network'))}</span><select data-role="network" ${internetAllowed ? '' : 'disabled'}>${networkOptions}</select></label>${internetAllowed ? '' : `<p class="vc-form-note">${esc(tx(c, 'desktop.virtual_computers_network_disabled'))}</p>`}`;
        const body = `<label><span>${esc(tx(c, 'desktop.virtual_computers_launch_name'))}</span><input type="text" data-role="launch-name" maxlength="64" autocomplete="off" placeholder="${esc(tx(c, 'desktop.virtual_computers_launch_name_placeholder'))}" data-autofocus></label>
            <label><span>${esc(tx(c, 'desktop.virtual_computers_template'))}</span><select data-role="template">${options}</select></label>${state.templatesFallback ? `<p class="vc-form-note">${esc(tx(c, 'desktop.virtual_computers_templates_fallback'))}</p>` : ''}
            <label><span>${esc(tx(c, 'desktop.virtual_computers_runtime'))}</span><select data-role="ttl">${ttlOptions}</select></label>${persistent}${networkField}
            ${caps.volumes ? `<label><span>${esc(tx(c, 'desktop.virtual_computers_volumes'))}</span><select data-role="launch-volume"><option value="">${esc(tx(c, 'desktop.virtual_computers_volume_none'))}</option>${volumes}</select></label>` : ''}`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_new'), body, modalButtons(state, 'confirm-launch', 'desktop.virtual_computers_launch'));
    }

    function renameModal(state, modal, titleKey) {
        const c = state.context;
        const body = `<label><span>${esc(tx(c, 'desktop.virtual_computers_rename_field'))}</span><input type="text" data-role="rename-name" maxlength="64" autocomplete="off" value="${esc(modal.value || '')}" data-autofocus></label><p class="vc-form-note">${esc(tx(c, 'desktop.virtual_computers_rename_hint'))}</p>`;
        return modalShell(state, tx(c, titleKey), body, modalButtons(state, modal.submitAction, 'desktop.virtual_computers_save'));
    }

    function saveMachineModal(state, modal) {
        const c = state.context;
        const candidates = state.volumes.filter(volumeAvailable);
        const preselected = candidates.some(volume => volume.id === modal.volumeId) ? modal.volumeId : '';
        const existing = candidates.map(volume => `<option value="${esc(volume.id)}" ${volume.id === preselected ? 'selected' : ''}>${esc(volumeOptionLabel(state, volume))}</option>`).join('');
        const body = `<label><span>${esc(tx(c, 'desktop.virtual_computers_save_target'))}</span><select data-role="save-target" data-autofocus><option value="__new__" ${preselected ? '' : 'selected'}>${esc(tx(c, 'desktop.virtual_computers_save_new_volume'))}</option>${existing}</select></label>
            <label data-role="save-new-fields" ${preselected ? 'hidden' : ''}><span>${esc(tx(c, 'desktop.virtual_computers_save_new_name'))}</span><input type="text" data-role="save-name" maxlength="64" autocomplete="off" value="${esc(modal.suggestedName || '')}"></label>
            <p class="vc-form-note">${esc(tx(c, 'desktop.virtual_computers_save_hint'))}</p>`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_save_title'), body, modalButtons(state, 'confirm-save-machine', 'desktop.virtual_computers_save'));
    }

    function assignAgentModal(state, modal) {
        const c = state.context;
        const body = `<p>${esc(tx(c, 'desktop.virtual_computers_assign_desc'))}</p><strong class="vc-confirm-target">${esc(modal.label || '')}</strong>
            <label><span>${esc(tx(c, 'desktop.virtual_computers_assign_task'))}</span><textarea data-role="assign-text" rows="5" maxlength="4000" data-autofocus></textarea></label>`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_assign_title'), body, modalButtons(state, 'confirm-assign-agent', 'desktop.virtual_computers_assign_submit'));
    }

    function uploadModal(state, modal) {
        const c = state.context;
        const body = `<p>${esc(txv(c, 'desktop.virtual_computers_upload_limit', { size: formatBytes(uploadLimitBytes, c) }))}</p><strong class="vc-confirm-target">${esc(modal.label || '')}</strong>
            <label><span>${esc(tx(c, 'desktop.virtual_computers_upload_file'))}</span><input type="file" data-role="upload-file" data-autofocus></label>`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_upload_title'), body, modalButtons(state, 'confirm-upload', 'desktop.virtual_computers_upload'));
    }

    function downloadModal(state, modal) {
        const c = state.context;
        const body = `<strong class="vc-confirm-target">${esc(modal.label || '')}</strong>
            <label><span>${esc(tx(c, 'desktop.virtual_computers_download_path'))}</span><input type="text" data-role="download-path" autocomplete="off" spellcheck="false" placeholder="/root/result.txt" data-autofocus></label>`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_download_title'), body, modalButtons(state, 'confirm-download', 'desktop.virtual_computers_download'));
    }

    function createVolumeModal(state) {
        const c = state.context;
        const retention = capabilities(state).volume_retention === true;
        const choices = volumeRetentionChoices(state);
        let selected = choices[0][0];
        choices.forEach(item => { if (item[0] <= 604800) selected = item[0]; });
        const options = choices.map(item => `<option value="${item[0]}" ${item[0] === selected ? 'selected' : ''}>${esc(item[1])}</option>`).join('')
            + `<option value="permanent" ${retention ? '' : 'disabled'}>${esc(tx(c, retention ? 'desktop.virtual_computers_badge_permanent' : 'desktop.virtual_computers_volume_permanent_unavailable'))}</option>`;
        const body = `<label><span>${esc(tx(c, 'desktop.virtual_computers_volume_name'))}</span><input type="text" data-role="volume-name" maxlength="64" autocomplete="off" placeholder="${esc(tx(c, 'desktop.virtual_computers_volume_name_placeholder'))}" data-autofocus></label>
            <label><span>${esc(tx(c, 'desktop.virtual_computers_volume_retention'))}</span><select data-role="volume-retention">${options}</select></label>
            <label class="vc-check"><input type="checkbox" data-role="volume-lock" ${retention ? '' : 'disabled'}><span>${esc(tx(c, 'desktop.virtual_computers_volume_lock_option'))}</span></label>
            ${retention ? '' : `<p class="vc-form-note">${esc(tx(c, 'desktop.virtual_computers_volume_banner_repair'))}</p>`}`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_volume_create'), body, modalButtons(state, 'confirm-create-volume', 'desktop.virtual_computers_volume_create'));
    }

    function importVolumeModal(state) {
        const c = state.context;
        const body = `<p>${esc(tx(c, 'desktop.virtual_computers_volume_import_hint'))}</p>
            <label><span>${esc(tx(c, 'desktop.virtual_computers_volume_import_id'))}</span><input type="text" data-role="volume-import-id" autocomplete="off" spellcheck="false" placeholder="vol-…" data-autofocus></label>`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_volume_import'), body, modalButtons(state, 'confirm-import-volume', 'desktop.virtual_computers_volume_import'));
    }

    function volumeRetentionModal(state, modal) {
        const c = state.context;
        const choices = volumeRetentionChoices(state);
        const volume = state.volumes.find(item => item.id === modal.id) || {};
        let selected = volume.persistent ? 'permanent' : String(choices[0][0]);
        if (!volume.persistent) choices.forEach(item => { if (item[0] <= 604800) selected = String(item[0]); });
        const options = choices.map(item => `<option value="${item[0]}" ${String(item[0]) === selected ? 'selected' : ''}>${esc(txv(c, 'desktop.virtual_computers_volume_keep_for', { time: item[1] }))}</option>`).join('')
            + `<option value="permanent" ${selected === 'permanent' ? 'selected' : ''}>${esc(tx(c, 'desktop.virtual_computers_badge_permanent'))}</option>`;
        const body = `<strong class="vc-confirm-target">${esc(modal.label || '')}</strong><label><span>${esc(tx(c, 'desktop.virtual_computers_volume_retention'))}</span><select data-role="volume-retention" data-autofocus>${options}</select></label>`;
        return modalShell(state, tx(c, 'desktop.virtual_computers_volume_retention_title'), body, modalButtons(state, 'confirm-volume-retention', 'desktop.virtual_computers_save'));
    }

    function confirmModal(state, modal) {
        const c = state.context;
        const body = `<p>${esc(tx(c, modal.message))}</p><strong class="vc-confirm-target">${esc(modal.target || '')}</strong>`;
        const footer = `<button type="button" class="vc-btn" data-action="close-modal" data-autofocus>${esc(tx(c, 'desktop.cancel'))}</button><button type="button" class="vc-btn ${modal.danger ? 'danger' : 'vc-primary'}" data-action="modal-confirm" data-role="modal-submit" data-confirm-action="${esc(modal.confirmAction)}" ${modal.pending ? 'disabled aria-busy="true"' : ''}>${esc(tx(c, modal.confirmLabel))}</button>`;
        return modalShell(state, tx(c, modal.title), body, footer);
    }

    function modalPane(state) {
        const modal = state.modal;
        if (!modal) return '';
        if (modal.type === 'launch') return launchModal(state, modal);
        if (modal.type === 'rename-machine') return renameModal(state, modal, 'desktop.virtual_computers_rename_title');
        if (modal.type === 'rename-volume') return renameModal(state, modal, 'desktop.virtual_computers_volume_rename_title');
        if (modal.type === 'save-machine') return saveMachineModal(state, modal);
        if (modal.type === 'assign-agent') return assignAgentModal(state, modal);
        if (modal.type === 'upload') return uploadModal(state, modal);
        if (modal.type === 'download') return downloadModal(state, modal);
        if (modal.type === 'create-volume') return createVolumeModal(state);
        if (modal.type === 'import-volume') return importVolumeModal(state);
        if (modal.type === 'volume-retention') return volumeRetentionModal(state, modal);
        if (modal.type === 'confirm') return confirmModal(state, modal);
        return '';
    }

    function modalField(state, role) {
        return state.host.querySelector(`[data-role="modal-host"] [data-role="${role}"]`);
    }

    function syncModalFields(state) {
        const modal = state.modal;
        if (!modal) return;
        if (modal.type === 'launch') {
            const persistent = modalField(state, 'launch-persistent');
            const ttl = modalField(state, 'ttl');
            if (ttl) ttl.disabled = !!(persistent && persistent.checked);
        } else if (modal.type === 'save-machine') {
            const target = modalField(state, 'save-target');
            const fields = modalField(state, 'save-new-fields');
            if (fields && target) fields.hidden = target.value !== '__new__';
        }
    }

    function requestCloseModal(state) {
        if (!state.modal || state.modal.pending) return;
        closeModal(state);
    }

    function bindActions(state) {
        state.clickHandler = event => {
            const raw = event.target;
            if (state.modal && raw && typeof raw.matches === 'function' && raw.matches('[data-role="modal"]')) {
                const armed = state.backdropDown !== false;
                state.backdropDown = null;
                if (armed) requestCloseModal(state);
                return;
            }
            const target = raw && raw.closest ? raw.closest('[data-action]') : null;
            if (!target || !state.host.contains(target)) return;
            const action = target.dataset.action;
            const id = target.dataset.id || '';
            if (action === 'refresh') refresh(state);
            else if (action === 'config') window.location.href = '/config#virtual_computers';
            else if (action === 'section') switchSection(state, target.dataset.section || 'machines');
            else if (action === 'new-agent-job') {
                switchSection(state, 'workspaces');
                state.host.querySelector('[data-role="agent-request"]')?.focus();
            }
            else if (action === 'new-machine') openModal(state, { type: 'launch' }, target);
            else if (action === 'launch-with-volume') openModal(state, { type: 'launch', volumeId: id, afterLaunchSection: 'machines' }, target);
            else if (action === 'select-machine') selectMachine(state, id);
            else if (action === 'overview') showOverview(state);
            else if (action === 'confirm-launch') launch(state);
            else if (action === 'destroy') openConfirmModal(state, { kind: 'destroy', target: machineLabel(state, id), trigger: target, run: () => destroyMachine(state, id), success: tx(state.context, K + 'toast_destroyed') });
            else if (action === 'rename-machine') openModal(state, { type: 'rename-machine', id, value: (state.machines.find(item => item.id === id) || {}).label || '', submitAction: 'confirm-rename-machine' }, target);
            else if (action === 'confirm-rename-machine') renameMachine(state);
            else if (action === 'extend-machine') extendMachine(state, id, Number(target.dataset.ttl) || 0);
            else if (action === 'save-machine') openSaveMachine(state, id, target);
            else if (action === 'confirm-save-machine') saveMachine(state);
            else if (action === 'assign-agent') openModal(state, { type: 'assign-agent', id, label: machineLabel(state, id) }, target);
            else if (action === 'confirm-assign-agent') assignMachine(state);
            else if (action === 'upload-file') openModal(state, { type: 'upload', id, label: machineLabel(state, id) }, target);
            else if (action === 'confirm-upload') uploadFile(state);
            else if (action === 'download-file') openModal(state, { type: 'download', id, label: machineLabel(state, id) }, target);
            else if (action === 'confirm-download') downloadFile(state);
            else if (action === 'open-workspace') switchSection(state, 'workspaces');
            else if (action === 'screenshot') screenshot(state, id);
            else if (action === 'vnc') openVNC(state, id);
            else if (action === 'terminal') openTerminal(state, id);
            else if (action === 'ask-agent') openAgentRequest(state);
            else if (action === 'delete-volume') openConfirmModal(state, { kind: 'delete-volume', target: volumeLabel(state, id), trigger: target, run: () => deleteVolume(state, id), success: tx(state.context, K + 'toast_volume_deleted') });
            else if (action === 'create-volume') openModal(state, { type: 'create-volume' }, target);
            else if (action === 'confirm-create-volume') createVolume(state);
            else if (action === 'import-volume') openModal(state, { type: 'import-volume' }, target);
            else if (action === 'confirm-import-volume') importVolume(state);
            else if (action === 'rename-volume') openModal(state, { type: 'rename-volume', id, value: (state.volumes.find(item => item.id === id) || {}).name || '', submitAction: 'confirm-rename-volume' }, target);
            else if (action === 'confirm-rename-volume') renameVolume(state);
            else if (action === 'toggle-volume-lock') toggleVolumeLock(state, id);
            else if (action === 'volume-retention') openModal(state, { type: 'volume-retention', id, label: volumeLabel(state, id) }, target);
            else if (action === 'confirm-volume-retention') updateVolumeRetention(state);
            else if (action === 'copy-volume-id') copyVolumeId(state, id);
            else if (action === 'modal-confirm') confirmModalAction(state);
            else if (action === 'close-modal') requestCloseModal(state);
            else if (action === 'retry-resource') refreshResource(state, target.dataset.resource || 'status');
            else if (action === 'close-workspace') confirmCloseWorkspace(state, id, target);
            else if (action === 'checkpoint-workspace') workspaceMutation(state, id, '/checkpoint', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' }, { success: tx(state.context, K + 'toast_checkpoint_created') });
            else if (action === 'observe-workspace') observeWorkspace(state, id, target.dataset.machineId || '', false);
            else if (action === 'check-native-desktop') window.VirtualComputersWorkspaces.checkNativeDesktop(state, id, { request, draw });
            else if (action === 'takeover-workspace') {
                const machineId = target.dataset.machineId || '';
                openConfirmModal(state, { kind: 'takeover-workspace', target: workspaceLabel(state, id), trigger: target, run: () => {}, after: () => observeWorkspace(state, id, machineId, true) });
            }
            else if (action === 'cancel-workspace-job') {
                const jobId = target.dataset.jobId || '';
                openConfirmModal(state, { kind: 'cancel-workspace-job', target: workspaceLabel(state, id), trigger: target, run: () => workspaceCall(state, id, '/jobs/' + encodeURIComponent(jobId), { method: 'DELETE' }), success: tx(state.context, K + 'toast_job_canceled') });
            }
            else if (action === 'approve-workspace-grant') {
                const grantId = target.dataset.grantId || '';
                openConfirmModal(state, { kind: 'approve-workspace-grant', target: workspaceLabel(state, id), trigger: target, run: () => workspaceCall(state, id, '/credential-grants/' + encodeURIComponent(grantId) + '/approve', jsonRequest('POST')), success: tx(state.context, K + 'toast_grant_approved') });
            }
            else if (action === 'revoke-workspace-grant') {
                const grantId = target.dataset.grantId || '';
                openConfirmModal(state, { kind: 'revoke-workspace-grant', target: workspaceLabel(state, id), trigger: target, run: () => workspaceCall(state, id, '/credential-grants/' + encodeURIComponent(grantId) + '/revoke', jsonRequest('POST')), success: tx(state.context, K + 'toast_grant_revoked') });
            }
        };
        state.changeHandler = () => syncModalFields(state);
        state.pointerHandler = event => {
            state.backdropDown = !!(event.target && typeof event.target.matches === 'function' && event.target.matches('[data-role="modal"]'));
        };
        state.keyHandler = event => {
            if (state.modal) {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    requestCloseModal(state);
                } else if (event.key === 'Enter' && state.modal.type !== 'confirm' && event.target && event.target.tagName === 'INPUT' && event.target.type !== 'file' && event.target.type !== 'checkbox') {
                    const submit = state.host.querySelector('[data-role="modal-host"] [data-role="modal-submit"]');
                    if (submit && !submit.disabled) {
                        event.preventDefault();
                        submit.click();
                    }
                } else if (event.key === 'Tab') {
                    const focusable = Array.from(state.host.querySelectorAll('[data-role="modal"] button:not([disabled]), [data-role="modal"] select:not([disabled]), [data-role="modal"] textarea:not([disabled]), [data-role="modal"] input:not([disabled]), [data-role="modal"] a[href], [data-role="modal"] [tabindex]:not([tabindex="-1"])')).filter(node => !node.closest('[hidden]'));
                    if (!focusable.length) return;
                    const first = focusable[0];
                    const last = focusable[focusable.length - 1];
                    if (event.shiftKey && event.target === first) {
                        event.preventDefault();
                        last.focus();
                    } else if (!event.shiftKey && event.target === last) {
                        event.preventDefault();
                        first.focus();
                    }
                }
                return;
            }
            const currentTab = event.target.closest && event.target.closest('[role="tab"]');
            if (!currentTab || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
            const tabs = Array.from(state.host.querySelectorAll('[role="tab"]'));
            const currentIndex = tabs.indexOf(currentTab);
            if (currentIndex < 0 || !tabs.length) return;
            event.preventDefault();
            let nextIndex = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : currentIndex + (event.key === 'ArrowRight' ? 1 : -1);
            nextIndex = (nextIndex + tabs.length) % tabs.length;
            const nextTab = tabs[nextIndex];
            switchSection(state, nextTab.dataset.section || 'machines');
            state.host.querySelector(`#${nextTab.id}`)?.focus();
        };
        state.host.addEventListener('click', state.clickHandler);
        state.host.addEventListener('change', state.changeHandler);
        state.host.addEventListener('mousedown', state.pointerHandler);
        state.host.addEventListener('keydown', state.keyHandler);
    }

    function machineLabel(state, id) {
        const machine = state.machines.find(item => item.id === id);
        return machine ? machineName(machine) : shortId(id);
    }

    function volumeLabel(state, id) {
        return volumeName(state, id);
    }

    function workspaceLabel(state, id) {
        const workspace = state.workspaces.find(item => item.id === id);
        const machine = workspace ? state.machines.find(item => item.id === workspace.machine_id) : null;
        return machine ? machineName(machine) : shortId(id);
    }

    // Opens the agent chat for a workspace. An adopted workspace wraps a computer the user
    // handed over, so the guide tells the agent that closing it only releases it.
    function sendToAgent(state, workspaceId, requestText, adopted) {
        if (typeof state.context.openApp !== 'function') return false;
        let guide = 'Use virtual_workspace and virtual_browser for webpages; use virtual_computer_ui for native applications and dialogs when enabled. Keep the same workspace. Do not use legacy agent task tools.';
        if (adopted) guide += ' This computer was handed over by the user; closing the workspace releases it without destroying it.';
        state.context.openApp('agent-chat', {
            chat_prefill: requestText,
            chat_autosend: true,
            chat_source_app: 'virtual-computers',
            window_context: {
                app_id: 'virtual-computers',
                workspace_id: workspaceId,
                label: tx(state.context, 'desktop.virtual_computers_title'),
                purpose: 'Control isolated Firecracker workspaces with the AuraGo main agent.',
                guide
            }
        });
        return true;
    }

    function openAgentRequest(state) {
        const input = state.host.querySelector('[data-role="agent-request"]');
        const workspaceId = state.host.querySelector('[data-role="agent-workspace"]')?.value || '';
        if (workspaceId && !state.workspaces.some(w => w.id === workspaceId && w.state === 'ready' && w.owner_session_id === 'virtual-desktop' && !w.mission_id)) {
            notify(state, tx(state.context, 'desktop.virtual_computers_workspace_unavailable'), 'error');
            return;
        }
        const requestText = input ? input.value.trim() : '';
        if (!requestText || typeof state.context.openApp !== 'function') {
            if (input) input.focus();
            return;
        }
        const workspace = state.workspaces.find(w => w.id === workspaceId);
        sendToAgent(state, workspaceId, requestText, !!(workspace && workspace.origin === 'adopted'));
        input.value = '';
    }

    function selectMachine(state, id) {
        if (!id || state.selectedMachineId === id) return;
        disconnectRemoteSessions(state);
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.detailMode = 'overview';
        draw(state);
    }

    function switchSection(state, section) {
        if (!['machines', 'workspaces', 'volumes'].includes(section) || state.activeSection === section) return;
        disconnectRemoteSessions(state);
        state.activeSection = section;
        state.modal = null;
        draw(state);
        if (section === 'volumes' && capabilities(state).volumes) refreshResource(state, 'volumes');
        if (section === 'workspaces' && capabilities(state).agent_control) refreshResource(state, 'workspaces');
        scheduleWorkspaceRefresh(state);
    }

    function applyResourceResult(state, resource, result) {
        state.resourceLoading[resource] = false;
        if (result.status === 'fulfilled') {
            state.resourceErrors[resource] = '';
            const body = result.value || {};
            if (resource === 'machines') storeMachines(state, normalizeMachineList(body));
            else if (resource === 'templates') {
                state.templates = Array.isArray(body.templates) ? body.templates : [];
                state.templatesFallback = state.templates.length === 0;
            } else if (resource === 'volumes') state.volumes = Array.isArray(body.volumes) ? body.volumes : [];
            else if (resource === 'workspaces') {
                state.workspaces = Array.isArray(body.workspaces) ? body.workspaces : [];
                state.workspaceSummaries = Array.isArray(body.workspace_summaries) ? body.workspace_summaries : [];
            }
            return;
        }
        state.resourceErrors[resource] = result.reason && result.reason.message ? result.reason.message : String(result.reason || '');
        if (resource === 'templates') state.templatesFallback = true;
    }

    async function refresh(state) {
        const generation = ++state.refreshGeneration;
        state.resourceLoading.status = true;
        state.resourceLoading.machines = true;
        state.resourceLoading.templates = true;
        draw(state);
        try {
            const status = await request('/api/virtual-computers/setup/status');
            if (state.disposed || generation !== state.refreshGeneration) return;
            state.status = status;
            state.resourceErrors.status = '';
            state.resourceLoading.status = false;
            if (!status.enabled) {
                storeMachines(state, []);
                state.templates = [];
                state.volumes = [];
                state.workspaces = [];
                state.workspaceSummaries = [];
                state.resourceLoading.machines = false;
                state.resourceLoading.templates = false;
                reconcileSection(state);
                reconcileVNC(state);
                reconcileTerminal(state);
                draw(state);
                return;
            }
            const resources = [
                ['machines', request('/api/virtual-computers/machines')],
                ['templates', request('/api/virtual-computers/templates')]
            ];
            if (status.capabilities && status.capabilities.volumes) {
                state.resourceLoading.volumes = true;
                resources.push(['volumes', request('/api/virtual-computers/volumes')]);
            }
            if (status.capabilities && status.capabilities.agent_control) {
                state.resourceLoading.workspaces = true;
                resources.push(['workspaces', request('/api/virtual-computers/workspaces')]);
            }
            const results = await Promise.allSettled(resources.map(item => item[1]));
            if (state.disposed || generation !== state.refreshGeneration) return;
            results.forEach((result, index) => applyResourceResult(state, resources[index][0], result));
        } catch (error) {
            if (state.disposed || generation !== state.refreshGeneration) return;
            state.resourceLoading.status = false;
            state.resourceLoading.machines = false;
            state.resourceLoading.templates = false;
            state.resourceErrors.status = error.message;
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + error.message, 'error');
        }
        reconcileSection(state);
        reconcileSelection(state);
        reconcileVNC(state);
        reconcileTerminal(state);
        draw(state);
        scheduleWorkspaceRefresh(state);
    }

    async function refreshResource(state, resource) {
        if (resource === 'status' || resource === 'machines' || resource === 'templates') {
            await refresh(state);
            return;
        }
        const paths = {
            volumes: '/api/virtual-computers/volumes',
            workspaces: '/api/virtual-computers/workspaces'
        };
        const path = paths[resource];
        if (!path || state.resourceLoading[resource]) return;
        state.resourceLoading[resource] = true;
        draw(state);
        const result = await Promise.allSettled([request(path)]);
        if (state.disposed) return;
        applyResourceResult(state, resource, result[0]);
        draw(state);
        scheduleWorkspaceRefresh(state);
    }

    function reconcileSelection(state) {
        if (state.selectedMachineId && state.machines.some(item => item.id === state.selectedMachineId)) return;
        if (state.vncSession) disconnectVNC(state);
        if (state.terminalSession) disconnectTerminal(state);
        state.selectedMachineId = state.machines.length ? state.machines[0].id : null;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.detailMode = 'overview';
    }

    function reconcileSection(state) {
        const available = state.activeSection === 'machines'
            || (state.activeSection === 'volumes' && capabilities(state).volumes)
            || (state.activeSection === 'workspaces' && capabilities(state).agent_control);
        if (available) return;
        disconnectVNC(state);
        if (state.terminalSession) disconnectTerminal(state);
        state.activeSection = 'machines';
    }

    function showOverview(state) {
        disconnectRemoteSessions(state);
        state.detailMode = 'overview';
        state.selectedShot = null;
        state.screenshotLoading = false;
        draw(state);
    }

    function applyVNCLayoutState(state) {
        const root = state.host.querySelector('.vc-app');
        if (root) root.classList.toggle('is-vnc-expanded', state.vncExpanded);
    }

    function setVNCExpanded(state, expanded) {
        state.vncExpanded = expanded === true;
        const root = state.host.querySelector('.vc-app');
        if (root) root.classList.toggle('is-vnc-expanded', state.vncExpanded);
    }

    function openVNC(state, id, viewOnly) {
        const machine = state.machines.find(item => item.id === id);
        if (!canUseVNC(state, machine) || !window.VirtualComputersVNC || typeof window.VirtualComputersVNC.mount !== 'function') {
            notify(state, tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            return;
        }
        disconnectRemoteSessions(state);
        state.activeSection = 'machines';
        state.detailMode = 'vnc';
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.vncMachineId = id;
        draw(state);
        const preview = state.host.querySelector('[data-role="preview"]');
        if (!preview) return;
        preview.innerHTML = '<div class="vc-vnc-mount" data-role="vnc-mount"></div>';
        const mountPoint = preview.querySelector('[data-role="vnc-mount"]');
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + window.location.host + '/api/virtual-computers/machines/' + encodeURIComponent(machine.id) + '/vnc';
        try {
            state.vncSession = window.VirtualComputersVNC.mount(mountPoint, {
                url,
                machineId: machine.id,
                viewOnly: viewOnly === true,
                t: key => tx(state.context, key),
                notify: (message, type) => notify(state, message, type),
                onExpandedChange: expanded => setVNCExpanded(state, expanded),
                onClose: () => {
                    if (state.detailMode === 'vnc' && state.vncMachineId === machine.id) showOverview(state);
                }
            });
        } catch (_) {
            state.vncSession = null;
            state.vncMachineId = null;
            state.detailMode = 'overview';
            setVNCExpanded(state, false);
            notify(state, tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            draw(state);
        }
    }

    function disconnectVNC(state, preserveWorkspaceControl) {
        const session = state.vncSession;
        const workspaceId = state.vncWorkspaceId;
        const releaseControl = state.vncWorkspaceTakeover && preserveWorkspaceControl !== true;
        state.vncSession = null;
        state.vncMachineId = null;
        state.vncWorkspaceId = null;
        state.vncWorkspaceTakeover = false;
        state.workspaceControlRenewedAt = 0;
        setVNCExpanded(state, false);
        if (session && typeof session.disconnect === 'function') {
            try { session.disconnect(); } catch (_) {}
        }
        if (releaseControl && workspaceId && window.VirtualComputersWorkspaces) {
            window.VirtualComputersWorkspaces.releaseControl(state, workspaceId, { request, notify, tx });
        }
    }

    function reconcileVNC(state) {
        if (state.detailMode !== 'vnc') return;
        const machine = state.machines.find(item => item.id === state.vncMachineId);
        if (state.vncSession && canUseVNC(state, machine)) return;
        disconnectVNC(state);
        state.detailMode = 'overview';
        state.selectedShot = null;
    }

    function openTerminal(state, id) {
        const machine = state.machines.find(item => item.id === id);
        if (!canUseTerminal(state, machine) || !window.VirtualComputersTerminal || typeof window.VirtualComputersTerminal.mount !== 'function') {
            notify(state, tx(state.context, 'desktop.virtual_computers_terminal_unavailable'), 'error');
            return;
        }
        disconnectRemoteSessions(state);
        state.activeSection = 'machines';
        state.detailMode = 'terminal';
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.terminalMachineId = id;
        draw(state);
        const preview = state.host.querySelector('[data-role="preview"]');
        if (!preview) return;
        preview.innerHTML = '<div class="vc-terminal-mount" data-role="terminal-mount"></div>';
        const mountPoint = preview.querySelector('[data-role="terminal-mount"]');
        const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const url = proto + '//' + window.location.host + '/api/virtual-computers/machines/' + encodeURIComponent(machine.id) + '/tty';
        try {
            state.terminalSession = window.VirtualComputersTerminal.mount(mountPoint, {
                url,
                machineId: machine.id,
                t: key => tx(state.context, key),
                notify: (message, type) => notify(state, message, type),
                onClose: () => {
                    if (state.detailMode === 'terminal' && state.terminalMachineId === machine.id) showOverview(state);
                }
            });
        } catch (_) {
            state.terminalSession = null;
            state.terminalMachineId = null;
            state.detailMode = 'overview';
            notify(state, tx(state.context, 'desktop.virtual_computers_terminal_unavailable'), 'error');
            draw(state);
        }
    }

    function disconnectTerminal(state) {
        const session = state.terminalSession;
        state.terminalSession = null;
        state.terminalMachineId = null;
        if (session && typeof session.disconnect === 'function') {
            try { session.disconnect(); } catch (_) {}
        }
    }

    function reconcileTerminal(state) {
        if (state.detailMode !== 'terminal') return;
        const machine = state.machines.find(item => item.id === state.terminalMachineId);
        if (state.terminalSession && canUseTerminal(state, machine)) return;
        disconnectTerminal(state);
        state.detailMode = 'overview';
        state.selectedShot = null;
    }

    function disconnectRemoteSessions(state, preserveWorkspaceControl) {
        disconnectVNC(state, preserveWorkspaceControl);
        disconnectTerminal(state);
    }

    async function launch(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'launch' || modal.pending) return;
        const read = role => modalField(state, role);
        const body = {
            template: read('template')?.value || 'python',
            allow_internet: capabilities(state).internet === true && read('network')?.value === 'internet'
        };
        const name = (read('launch-name')?.value || '').trim();
        if (name) body.name = name;
        if (read('launch-persistent')?.checked) body.persistent = true;
        else body.ttl_seconds = Number(read('ttl')?.value || limits(state).defaultTTL);
        const volumeId = read('launch-volume')?.value || '';
        if (volumeId) body.volume_id = volumeId;
        await runModalAction(state, async () => {
            const result = await request('/api/virtual-computers/machines', jsonRequest('POST', body));
            const created = result && result.machine && result.machine.id;
            if (created) {
                disconnectRemoteSessions(state);
                state.selectedMachineId = created;
                state.detailMode = 'overview';
                state.selectedShot = null;
                state.screenshotLoading = false;
            }
            await refresh(state);
            return result;
        }, tx(state.context, K + 'toast_launched'), () => {
            if (modal.afterLaunchSection && state.activeSection !== modal.afterLaunchSection) switchSection(state, modal.afterLaunchSection);
        });
    }

    async function destroyMachine(state, id) {
        if (!id || isPending(state, 'destroy')) return false;
        if (state.vncMachineId === id) showOverview(state);
        if (state.terminalMachineId === id) showOverview(state);
        setPending(state, 'destroy', true);
        try {
            await request('/api/virtual-computers/machines/' + encodeURIComponent(id), { method: 'DELETE' });
            await refresh(state);
            return true;
        } finally {
            setPending(state, 'destroy', false);
        }
    }

    function isCurrentScreenshotRequest(state, id, requestID) {
        return state.detailMode === 'screenshot' && state.selectedMachineId === id && state.screenshotRequestID === requestID;
    }

    function settleScreenshot(state, id, requestID, shot) {
        if (!isCurrentScreenshotRequest(state, id, requestID)) return false;
        state.selectedShot = shot || null;
        state.screenshotLoading = false;
        return true;
    }

    async function screenshot(state, id) {
        const machine = state.machines.find(item => item.id === id);
        if (!id || !machine || machine.display !== true) return;
        const requestID = ++state.screenshotRequestID;
        disconnectRemoteSessions(state);
        state.activeSection = 'machines';
        state.detailMode = 'screenshot';
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = true;
        draw(state);
        try {
            const body = await request('/api/virtual-computers/machines/' + encodeURIComponent(id) + '/screenshot');
            if (settleScreenshot(state, id, requestID, body.screenshot)) draw(state);
        } catch (error) {
            if (!settleScreenshot(state, id, requestID, null)) return;
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + errorText(state, error), 'error');
            draw(state);
        }
    }

    function openModal(state, modal, trigger) {
        modal.renderKey = ++state.modalSeq;
        modal.returnSelector = stableSelector(trigger);
        modal.error = modal.error || '';
        modal.pending = false;
        state.modal = modal;
        state.modalReturnFocus = trigger || state.host.ownerDocument.activeElement;
        draw(state);
    }

    function openConfirmModal(state, options) {
        const definition = confirmDefinitions[options.kind] || {};
        openModal(state, {
            type: 'confirm',
            kind: options.kind,
            title: options.title || definition.title,
            message: options.message || definition.message,
            target: options.target || '',
            confirmLabel: options.confirmLabel || definition.label,
            confirmAction: options.confirmAction || definition.confirmAction || 'confirm-' + options.kind,
            danger: options.danger !== undefined ? options.danger !== false : definition.danger !== false,
            run: options.run,
            success: options.success,
            after: options.after
        }, options.trigger);
    }

    function confirmModalAction(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'confirm' || modal.pending || typeof modal.run !== 'function') return;
        runModalAction(state, modal.run, modal.success, modal.after);
    }

    // Runs the modal's request: busy state while it runs, close and toast on success,
    // an inline error (and the dialog stays open) on failure.
    async function runModalAction(state, work, success, after) {
        const modal = state.modal;
        if (!modal || modal.pending) return false;
        modal.pending = true;
        modal.error = '';
        patchModal(state);
        let result;
        try {
            result = await work();
        } catch (error) {
            if (state.disposed) return false;
            modal.pending = false;
            if (state.modal === modal) modal.error = errorText(state, error);
            draw(state);
            return false;
        }
        if (state.disposed) return false;
        modal.pending = false;
        if (state.modal === modal) closeModal(state);
        const message = typeof success === 'function' ? success(result) : success;
        if (message) toast(state, message);
        if (typeof after === 'function') after(result);
        return true;
    }

    function closeModal(state, restoreFocus = true) {
        const modal = state.modal;
        const returnFocus = state.modalReturnFocus;
        state.modal = null;
        state.modalReturnFocus = null;
        draw(state);
        if (!restoreFocus) return;
        let target = null;
        if (modal && modal.returnSelector) {
            try { target = state.host.querySelector(modal.returnSelector); } catch (_) { target = null; }
        }
        if (!target && returnFocus && returnFocus.isConnected !== false && typeof returnFocus.focus === 'function') target = returnFocus;
        if (!target) target = state.host.querySelector('[data-action="refresh"]');
        if (target && typeof target.focus === 'function') target.focus();
    }

    function downloadURL(machineId, path) {
        return '/api/virtual-computers/machines/' + encodeURIComponent(machineId) + '/download?path=' + encodeURIComponent(path);
    }

    function readFileBase64(file) {
        return new Promise((resolve, reject) => {
            const reader = new FileReader();
            reader.onerror = () => reject(reader.error || new Error('read failed'));
            reader.onload = () => {
                const text = String(reader.result || '');
                resolve(text.slice(text.indexOf(',') + 1));
            };
            reader.readAsDataURL(file);
        });
    }

    async function renameMachine(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'rename-machine') return;
        const name = (modalField(state, 'rename-name')?.value || '').trim();
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id), jsonRequest('PATCH', { name }));
            await refresh(state);
        }, tx(state.context, K + 'toast_renamed'));
    }

    async function extendMachine(state, id, ttl) {
        if (!id || !ttl) return;
        await mutate(state, 'extend-' + id, 'machines', '/api/virtual-computers/machines/' + encodeURIComponent(id) + '/extend', jsonRequest('POST', { ttl_seconds: ttl }), {
            success: txv(state.context, K + 'toast_extended', { count: Math.round(ttl / 60) })
        });
    }

    function openSaveMachine(state, id, trigger) {
        const machine = state.machines.find(item => item.id === id) || {};
        openModal(state, {
            type: 'save-machine',
            id,
            volumeId: machine.volume_id || '',
            suggestedName: machine.label || '',
            progressKey: K + 'save_progress'
        }, trigger);
    }

    async function saveMachine(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'save-machine') return;
        const target = modalField(state, 'save-target')?.value || '__new__';
        const name = (modalField(state, 'save-name')?.value || '').trim();
        await runModalAction(state, async () => {
            let volumeId = target;
            if (target === '__new__') {
                // A retry after a failed save reuses the volume this dialog already created.
                if (!modal.createdVolumeId) {
                    const ttl = Math.min(604800, limits(state).volumeMaxTTL);
                    const created = await request('/api/virtual-computers/volumes', jsonRequest('POST', { name, ttl_seconds: ttl }));
                    modal.createdVolumeId = created && created.volume && created.volume.id;
                }
                volumeId = modal.createdVolumeId;
            }
            await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id) + '/save', jsonRequest('POST', { volume_id: volumeId }));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_saved'));
    }

    async function assignMachine(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'assign-agent') return;
        const text = (modalField(state, 'assign-text')?.value || '').trim();
        if (!text) {
            setModalError(state, tx(state.context, K + 'assign_task_required'));
            modalField(state, 'assign-text')?.focus();
            return;
        }
        await runModalAction(state, async () => {
            const result = await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id) + '/assign-agent', jsonRequest('POST'));
            await refresh(state);
            return result;
        }, tx(state.context, K + 'toast_assigned'), result => {
            const workspace = result && result.workspace;
            if (workspace && workspace.id) sendToAgent(state, workspace.id, text, true);
        });
    }

    async function uploadFile(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'upload') return;
        const file = modalField(state, 'upload-file')?.files?.[0];
        if (!file) {
            setModalError(state, tx(state.context, K + 'upload_choose'));
            return;
        }
        if (file.size > uploadLimitBytes) {
            setModalError(state, txv(state.context, K + 'upload_too_large', { size: formatBytes(uploadLimitBytes, state.context) }));
            return;
        }
        await runModalAction(state, async () => {
            const data = await readFileBase64(file);
            await request('/api/virtual-computers/machines/' + encodeURIComponent(modal.id) + '/upload', jsonRequest('POST', { filename: file.name, content_base64: data }));
        }, tx(state.context, K + 'toast_uploaded'));
    }

    async function downloadFile(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'download') return;
        const path = (modalField(state, 'download-path')?.value || '').trim();
        if (!path) {
            setModalError(state, tx(state.context, K + 'download_path_required'));
            return;
        }
        await runModalAction(state, async () => {
            const resp = await fetch(downloadURL(modal.id, path));
            if (!resp.ok) {
                const failure = await readBody(resp);
                throw responseError(resp, failure, typeof failure === 'string' ? failure.trim().slice(0, 4096) || resp.statusText : resp.statusText);
            }
            const blob = await resp.blob();
            const doc = state.host.ownerDocument;
            const link = doc.createElement('a');
            const href = URL.createObjectURL(blob);
            link.href = href;
            link.download = path.split('/').filter(Boolean).pop() || 'download';
            link.rel = 'noopener';
            doc.body.appendChild(link);
            link.click();
            link.remove();
            setTimeout(() => URL.revokeObjectURL(href), 10000);
        }, tx(state.context, K + 'toast_downloaded'));
    }

    async function createVolume(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'create-volume') return;
        const name = (modalField(state, 'volume-name')?.value || '').trim();
        const retention = modalField(state, 'volume-retention')?.value || '';
        const body = { name };
        if (retention === 'permanent') body.persistent = true;
        else body.ttl_seconds = Number(retention) || 86400;
        if (modalField(state, 'volume-lock')?.checked) body.locked = true;
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes', jsonRequest('POST', body));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_created'));
    }

    async function importVolume(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'import-volume') return;
        const id = (modalField(state, 'volume-import-id')?.value || '').trim();
        if (!id) {
            setModalError(state, tx(state.context, K + 'volume_import_required'));
            return;
        }
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'GET' });
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_imported'));
    }

    async function renameVolume(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'rename-volume') return;
        const name = (modalField(state, 'rename-name')?.value || '').trim();
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes/' + encodeURIComponent(modal.id), jsonRequest('PATCH', { name }));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_updated'));
    }

    async function updateVolumeRetention(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'volume-retention') return;
        const retention = modalField(state, 'volume-retention')?.value || '';
        const body = retention === 'permanent' ? { persistent: true } : { persistent: false, ttl_seconds: Number(retention) || 86400 };
        await runModalAction(state, async () => {
            await request('/api/virtual-computers/volumes/' + encodeURIComponent(modal.id), jsonRequest('PATCH', body));
            await refreshResource(state, 'volumes');
        }, tx(state.context, K + 'toast_volume_updated'));
    }

    async function toggleVolumeLock(state, id) {
        const volume = state.volumes.find(item => item.id === id);
        if (!id || !volume) return;
        const lock = !volume.locked;
        await mutate(state, 'volume-' + id, 'volumes', '/api/virtual-computers/volumes/' + encodeURIComponent(id), jsonRequest('PATCH', { locked: lock }), {
            success: tx(state.context, K + (lock ? 'toast_volume_locked' : 'toast_volume_unlocked'))
        });
    }

    function copyVolumeId(state, id) {
        const clipboard = typeof navigator !== 'undefined' ? navigator.clipboard : null;
        const failed = () => notify(state, tx(state.context, K + 'toast_copy_failed'), 'error');
        if (!id || !clipboard || typeof clipboard.writeText !== 'function') {
            failed();
            return;
        }
        Promise.resolve().then(() => clipboard.writeText(id)).then(() => toast(state, tx(state.context, K + 'toast_id_copied')), failed);
    }

    // Workspace requests that throw so a confirmation dialog can show the failure inline.
    async function workspaceCall(state, workspaceId, suffix, options, refreshMachines) {
        await request('/api/virtual-computers/workspaces/' + encodeURIComponent(workspaceId) + (suffix || ''), options || {});
        if (refreshMachines) await refresh(state);
        else await refreshResource(state, 'workspaces');
    }

    function confirmCloseWorkspace(state, id, trigger) {
        const workspace = state.workspaces.find(item => item.id === id) || {};
        const adopted = workspace.origin === 'adopted';
        openConfirmModal(state, {
            kind: adopted ? 'release-workspace' : 'close-workspace',
            target: workspaceLabel(state, id),
            trigger,
            run: () => workspaceCall(state, id, '', { method: 'DELETE' }, true),
            success: tx(state.context, K + (adopted ? 'toast_workspace_released' : 'toast_workspace_closed'))
        });
    }

    async function workspaceMutation(state, workspaceId, suffix, options, extra) {
        if (!workspaceId || isPending(state, 'workspace-' + workspaceId)) return null;
        return mutate(state, 'workspace-' + workspaceId, 'workspaces', '/api/virtual-computers/workspaces/' + encodeURIComponent(workspaceId) + (suffix || ''), options || {}, extra);
    }

    async function observeWorkspace(state, workspaceId, machineId, takeControl) {
        window.VirtualComputersWorkspaces.controlAndOpen(state, workspaceId, machineId, takeControl,
            { request, notify, tx, canUseVNC, draw, workspaceMutation, disconnectRemoteSessions, disconnectVNC, setVNCExpanded });
    }

    async function deleteVolume(state, id) {
        if (!id) return false;
        await request('/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'DELETE' });
        await refreshResource(state, 'volumes');
        return true;
    }

    // Direct (dialog-free) mutations: returns the response body, or null after reporting the failure.
    async function mutate(state, actionKey, resource, path, options, extra) {
        setPending(state, actionKey, true);
        draw(state);
        try {
            const body = await request(path, options);
            state.resourceErrors[resource] = '';
            if (resource === 'machines') await refresh(state);
            else await refreshResource(state, resource);
            if (extra && extra.success) toast(state, extra.success);
            return body === '' || body == null ? {} : body;
        } catch (error) {
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + errorText(state, error), 'error');
            return null;
        } finally {
            setPending(state, actionKey, false);
            draw(state);
        }
    }

    function notify(state, message, type) {
        if (state.context && typeof state.context.notify === 'function') {
            state.context.notify({
                title: tx(state.context, 'desktop.notification'),
                message,
                type: type || 'info'
            });
        }
    }

    function toast(state, message) {
        notify(state, message, 'success');
    }

    function normalizeMachineList(body) {
        return body && Array.isArray(body.machines) ? body.machines : [];
    }

    function storeMachines(state, machines) {
        const normalized = Array.isArray(machines) ? machines : [];
        state.machines = normalized;
        state.machineSnapshot = JSON.stringify(normalized);
    }

    function isMachinePollingVisible(state) {
        if (!state.host) return false;
        const document = state.host.ownerDocument;
        if (document && document.visibilityState === 'hidden') return false;
        const appWindow = typeof state.host.closest === 'function' ? state.host.closest('.vd-window') : null;
        if (appWindow && (appWindow.hidden || (appWindow.style && appWindow.style.display === 'none'))) return false;
        return typeof state.host.getClientRects !== 'function' || state.host.getClientRects().length > 0;
    }

    async function pollMachines(state) {
        if (state.disposed || state.machinePollInFlight || state.resourceLoading.machines || !isMachinePollingVisible(state)) return;
        const generation = state.refreshGeneration;
        state.machinePollInFlight = true;
        try {
            const body = await request('/api/virtual-computers/machines');
            if (state.disposed || generation !== state.refreshGeneration) return;
            const machines = normalizeMachineList(body);
            const snapshot = JSON.stringify(machines);
            if (snapshot === state.machineSnapshot) return;
            storeMachines(state, machines);
            state.resourceErrors.machines = '';
            reconcileSelection(state);
            reconcileVNC(state);
            reconcileTerminal(state);
            draw(state);
        } catch (_) {
            // Background polling is best-effort; explicit refresh reports errors.
        } finally {
            state.machinePollInFlight = false;
        }
    }

    function scheduleMachineRefresh(state) {
        if (state.machinePollTimer) {
            clearTimeout(state.machinePollTimer);
            state.machinePollTimer = null;
        }
        if (state.disposed) return;
        state.machinePollTimer = setTimeout(async () => {
            state.machinePollTimer = null;
            await pollMachines(state);
            scheduleMachineRefresh(state);
        }, machinePollIntervalMs);
    }

    function scheduleWorkspaceRefresh(state) {
        if (state.workspaceRefreshTimer) {
            clearTimeout(state.workspaceRefreshTimer);
            state.workspaceRefreshTimer = null;
        }
        if (state.disposed || state.activeSection !== 'workspaces' || !capabilities(state).agent_control) return;
        const active = state.workspaces.some(workspace => workspace && ['opening', 'ready', 'closing'].includes(workspace.state));
        if (!active) return;
        state.workspaceRefreshTimer = setTimeout(() => {
            state.workspaceRefreshTimer = null;
            refreshResource(state, 'workspaces');
        }, 3000);
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        state.refreshGeneration++;
        if (state.workspaceRefreshTimer) clearTimeout(state.workspaceRefreshTimer);
        if (state.machinePollTimer) clearTimeout(state.machinePollTimer);
        if (state.expiryCountdownTimer) clearTimeout(state.expiryCountdownTimer);
        state.machinePollTimer = null;
        state.workspaceRefreshTimer = null;
        state.expiryCountdownTimer = null;
        disconnectRemoteSessions(state);
        if (state.clickHandler) state.host.removeEventListener('click', state.clickHandler);
        if (state.changeHandler) state.host.removeEventListener('change', state.changeHandler);
        if (state.pointerHandler) state.host.removeEventListener('mousedown', state.pointerHandler);
        if (state.keyHandler) state.host.removeEventListener('keydown', state.keyHandler);
        instances.delete(windowId);
    }

    window.VirtualComputersApp = { render, dispose };
}());
