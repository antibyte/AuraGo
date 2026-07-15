(function () {
    'use strict';

    const instances = new Map();

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

    async function request(path, options) {
        const resp = await fetch(path, options || {});
        const contentType = resp.headers.get('Content-Type') || '';
        const body = contentType.includes('application/json') ? await resp.json() : await resp.text();
        if (!resp.ok || (body && body.error)) throw new Error((body && (body.error || body.message)) || resp.statusText);
        return body;
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
            tasks: [],
            status: null,
            selectedShot: null,
            selectedMachineId: null,
            screenshotLoading: false,
            screenshotRequestID: 0,
            detailMode: 'overview',
            taskMachineFilter: '',
            modal: null,
            modalReturnFocus: null,
            vncSession: null,
            vncMachineId: null,
            vncExpanded: false,
            resourceErrors: { status: '', machines: '', templates: '', tasks: '', volumes: '' },
            resourceLoading: { status: true, machines: true, templates: true, tasks: false, volumes: false },
            pendingActions: new Set(),
            refreshGeneration: 0,
            clickHandler: null,
            changeHandler: null,
            keyHandler: null,
            taskRefreshTimer: null,
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

    function formatDuration(seconds) {
        const value = Math.max(0, Number(seconds) || 0);
        if (value < 60) return `${value} s`;
        if (value < 3600) return `${Math.round(value / 60)} min`;
        if (value < 86400) return `${Math.round(value / 3600)} h`;
        return `${Math.round(value / 86400)} d`;
    }

    function formatBytes(value) {
        const bytes = Math.max(0, Number(value) || 0);
        if (!bytes) return '—';
        if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
        if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
        return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
    }

    function draw(state) {
        if (state.disposed) return;
        const toolbar = state.host.querySelector('[data-role="toolbar"]');
        const sections = state.host.querySelector('[data-role="sections"]');
        const statusRegion = state.host.querySelector('[data-role="status-region"]');
        const content = state.host.querySelector('[data-role="content"]');
        const modal = state.host.querySelector('[data-role="modal-host"]');
        if (!toolbar || !sections || !statusRegion || !content || !modal) return;
        toolbar.innerHTML = toolbarPane(state);
        sections.innerHTML = sectionTabs(state);
        statusRegion.innerHTML = statusPane(state);
        statusRegion.hidden = statusRegion.innerHTML.trim() === '';

        const liveMount = state.activeSection === 'machines' && state.detailMode === 'vnc'
            ? content.querySelector('[data-role="vnc-mount"]')
            : null;
        if (liveMount) {
            const list = content.querySelector('[data-role="machine-list"]');
            if (list) list.innerHTML = machineList(state);
        } else if (state.activeSection === 'tasks') {
            content.innerHTML = taskPane(state);
        } else if (state.activeSection === 'volumes') {
            content.innerHTML = volumePane(state);
        } else {
            content.innerHTML = machinesPane(state);
        }
        content.dataset.section = state.activeSection;
        content.setAttribute('aria-labelledby', `vc-tab-${state.activeSection}`);
        modal.innerHTML = modalPane(state);
        applyVNCLayoutState(state);
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
                <button type="button" class="vc-btn vc-primary vc-icon-label" data-action="new-machine" ${isMutable(state) && !isPending(state, 'launch') ? '' : 'disabled'}>${icon(state, 'plus', '+')}<span>${esc(tx(c, 'desktop.virtual_computers_new'))}</span></button>
            </div>`;
    }

    function sectionTabs(state) {
        const c = state.context;
        const tabs = [
            { id: 'machines', key: 'desktop.virtual_computers_machines', icon: 'server' }
        ];
        if (capabilities(state).agent_tasks) tabs.push({ id: 'tasks', key: 'desktop.virtual_computers_tasks', icon: 'run' });
        if (capabilities(state).volumes) tabs.push({ id: 'volumes', key: 'desktop.virtual_computers_volumes', icon: 'archive' });
        return `<div class="vc-tab-track" role="tablist">${tabs.map(tab => {
            const active = state.activeSection === tab.id;
            const count = tab.id === 'machines' ? state.machines.length : tab.id === 'tasks' ? state.tasks.length : state.volumes.length;
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
                <span class="vc-machine-main"><strong>${esc(machine.name || id || machine.template || 'machine')}</strong><span>${esc(machine.template || '—')}</span></span>
                <span class="vc-machine-state"><span class="vc-state-dot" data-state="${esc(machine.status || '')}"></span>${esc(machine.status || '—')}</span>
            </button>`;
        }).join('');
    }

    function detailPane(state) {
        const c = state.context;
        if (state.resourceLoading.machines && !state.machines.length) return `<div class="vc-detail-skeleton">${skeletonRows()}</div>`;
        const machine = state.machines.find(item => item.id === state.selectedMachineId);
        if (!machine) {
            return `<div class="vc-empty-state vc-empty-detail"><span class="vc-empty-icon">${icon(state, 'monitor', '▣')}</span><strong>${esc(tx(c, 'desktop.virtual_computers_select_machine'))}</strong><p>${esc(tx(c, 'desktop.virtual_computers_status'))}</p></div>`;
        }
        const mutable = isMutable(state);
        const allowTasks = capabilities(state).agent_tasks && mutable;
        const ports = Array.isArray(machine.web_ports) ? machine.web_ports : [];
        const portLinks = ports.length ? ports.map(port => `<a class="vc-link" href="/api/virtual-computers/machines/${encodeURIComponent(machine.id)}/web/${Number(port)}/" target="_blank" rel="noopener">${icon(state, 'external', '↗')} ${esc(String(port))}</a>`).join('') : '—';
        let viewer = `<div class="vc-machine-hero"><span class="vc-machine-hero-icon">${icon(state, machine.display ? 'monitor' : 'server', '▣')}</span><p>${esc(machine.display ? tx(c, 'desktop.virtual_computers_vnc_live') : tx(c, 'desktop.virtual_computers_headless'))}</p></div>`;
        if (state.detailMode === 'screenshot') {
            if (state.screenshotLoading) viewer = skeletonRows();
            else if (state.selectedShot) viewer = `<img class="vc-shot" src="data:${esc(state.selectedShot.mime_type || 'image/png')};base64,${esc(state.selectedShot.data_base64 || '')}" alt="${esc(tx(c, 'desktop.virtual_computers_screenshot'))}">`;
        } else if (state.detailMode === 'vnc') {
            viewer = `<div class="vc-vnc-placeholder">${esc(tx(c, 'desktop.virtual_computers_vnc_connecting'))}</div>`;
        }
        return `<article class="vc-machine-detail">
            <header class="vc-detail-header"><div><span class="vc-eyebrow">${esc(machine.template || '—')}</span><h3>${esc(machine.name || machine.id)}</h3><span class="vc-detail-status"><span class="vc-state-dot" data-state="${esc(machine.status || '')}"></span>${esc(machine.status || '—')}</span></div>
            <div class="vc-actions vc-detail-actions">
                ${machine.display ? `<button type="button" class="vc-btn vc-icon-label" data-action="screenshot" data-id="${esc(machine.id)}">${icon(state, 'image', '▧')}<span>${esc(tx(c, 'desktop.virtual_computers_screenshot'))}</span></button>` : ''}
                ${canUseVNC(state, machine) ? `<button type="button" class="vc-btn vc-primary vc-icon-label" data-action="vnc" data-id="${esc(machine.id)}">${icon(state, 'monitor', '▣')}<span>${esc(tx(c, 'desktop.virtual_computers_vnc_live'))}</span></button>` : ''}
                ${allowTasks ? `<button type="button" class="vc-btn vc-icon-label" data-action="start-task" data-id="${esc(machine.id)}">${icon(state, 'run', '▶')}<span>${esc(tx(c, 'desktop.virtual_computers_task_start'))}</span></button>` : ''}
                ${mutable ? `<button type="button" class="vc-btn danger vc-icon-label" data-action="destroy" data-id="${esc(machine.id)}">${icon(state, 'stop', '■')}<span>${esc(tx(c, 'desktop.virtual_computers_destroy'))}</span></button>` : ''}
            </div></header>
            <dl class="vc-meta-grid">
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_template'))}</dt><dd>${esc(machine.template || '—')}</dd></div>
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_runtime'))}</dt><dd>${esc(machine.persistent ? tx(c, 'desktop.virtual_computers_unlimited') : formatDuration(machine.ttl_seconds))}</dd></div>
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_expires'))}</dt><dd>${esc(formatDate(machine.expires_at))}</dd></div>
                <div><dt>${esc(tx(c, 'desktop.virtual_computers_display'))}</dt><dd>${esc(machine.display ? tx(c, 'desktop.on') : tx(c, 'desktop.off'))}</dd></div>
                <div class="vc-meta-wide"><dt>${esc(tx(c, 'desktop.virtual_computers_web_ports'))}</dt><dd class="vc-machine-links">${portLinks}</dd></div>
            </dl>
            <div class="vc-detail-viewer">${viewer}</div>
        </article>`;
    }

    function taskRows(state, tasks) {
        const c = state.context;
        if (!tasks.length) return `<div class="vc-empty compact">${esc(tx(c, 'desktop.virtual_computers_tasks_empty'))}</div>`;
        return tasks.map(task => {
            const running = task.status === 'queued' || task.status === 'running';
            return `<article class="vc-ledger-row"><div class="vc-task-summary"><span class="vc-task-title"><strong>${esc(task.kind || 'task')}</strong><span class="vc-state-chip" data-state="${esc(task.status || '')}">${esc(task.status || '—')}</span></span><span>${esc(task.machine_id || '—')} · ${esc(formatDate(task.updated_at || task.created_at))}</span>${task.error ? `<small class="vc-task-error">${esc(task.error)}</small>` : ''}</div>${running && isMutable(state) ? `<button type="button" class="vc-icon-btn danger" data-action="cancel_agent_task" data-id="${esc(task.id)}" aria-label="${esc(tx(c, 'desktop.virtual_computers_task_cancel'))}">${icon(state, 'stop', '■')}</button>` : ''}</article>`;
        }).join('');
    }

    function taskPane(state) {
        const c = state.context;
        if (state.resourceLoading.tasks && !state.tasks.length) return `<section class="vc-section-page">${skeletonRows()}</section>`;
        if (state.resourceErrors.tasks && !state.tasks.length) return `<section class="vc-section-page">${resourceErrorPane(state, 'tasks')}</section>`;
        const filtered = state.taskMachineFilter ? state.tasks.filter(task => task.machine_id === state.taskMachineFilter) : state.tasks;
        const active = filtered.filter(task => task.status === 'queued' || task.status === 'running');
        const completed = filtered.filter(task => task.status !== 'queued' && task.status !== 'running');
        const machineOptions = state.machines.map(machine => `<option value="${esc(machine.id)}" ${state.taskMachineFilter === machine.id ? 'selected' : ''}>${esc(machine.name || machine.id)}</option>`).join('');
        return `<section class="vc-section-page"><header class="vc-section-header"><div><span class="vc-eyebrow">${esc(tx(c, 'desktop.virtual_computers_title'))}</span><h3>${esc(tx(c, 'desktop.virtual_computers_tasks'))}</h3></div><label class="vc-task-filter"><span>${esc(tx(c, 'desktop.virtual_computers_machines'))}</span><select data-role="task-filter"><option value="">${esc(tx(c, 'desktop.virtual_computers_machines'))}</option>${machineOptions}</select></label></header>
            ${state.resourceErrors.tasks ? `<div class="vc-inline-error">${esc(state.resourceErrors.tasks)}</div>` : ''}
            <section class="vc-ledger"><h4 class="vc-ledger-title">${esc(tx(c, 'desktop.virtual_computers_active_jobs'))}<span class="vc-ledger-count">${active.length}</span></h4>${taskRows(state, active)}</section>
            <section class="vc-ledger"><h4 class="vc-ledger-title">${esc(tx(c, 'desktop.virtual_computers_completed_jobs'))}<span class="vc-ledger-count">${completed.length}</span></h4>${taskRows(state, completed)}</section>
        </section>`;
    }

    function volumePane(state) {
        const c = state.context;
        if (state.resourceLoading.volumes && !state.volumes.length) return `<section class="vc-section-page">${skeletonRows()}</section>`;
        if (state.resourceErrors.volumes && !state.volumes.length) return `<section class="vc-section-page">${resourceErrorPane(state, 'volumes')}</section>`;
        const mutable = isMutable(state);
        const rows = state.volumes.length ? state.volumes.map(volume => `<article class="vc-ledger-row"><div class="vc-volume-summary"><span class="vc-volume-title"><strong>${esc(volume.name || volume.id)}</strong><span class="vc-state-chip" data-state="${esc(volume.verification_status || '')}">${esc(volume.verification_status || '—')}</span></span><span>${esc(formatBytes(volume.size_bytes))} · ${esc(formatDate(volume.expires_at))}</span></div>${mutable ? `<button type="button" class="vc-icon-btn danger" data-action="delete-volume" data-id="${esc(volume.id)}" aria-label="${esc(tx(c, 'desktop.virtual_computers_volume_delete'))}">${icon(state, 'trash', '×')}</button>` : ''}</article>`).join('') : `<div class="vc-empty-state"><span class="vc-empty-icon">${icon(state, 'archive', '▤')}</span><strong>${esc(tx(c, 'desktop.virtual_computers_volumes_empty'))}</strong></div>`;
        const controls = mutable ? `<div class="vc-volume-tools"><label><span>${esc(tx(c, 'desktop.virtual_computers_runtime'))}</span><select data-role="volume-ttl"><option value="86400">1 d</option><option value="604800">7 d</option><option value="2592000">30 d</option></select></label><button type="button" class="vc-btn vc-primary" data-action="create-volume" ${isPending(state, 'create-volume') ? 'disabled' : ''}>${icon(state, 'plus', '+')}${esc(tx(c, 'desktop.virtual_computers_volume_create'))}</button><label class="vc-import-field"><span>${esc(tx(c, 'desktop.virtual_computers_volume_import'))}</span><input data-role="volume-import-id" type="text" placeholder="volume_id"></label><button type="button" class="vc-btn" data-action="import-volume" ${isPending(state, 'import-volume') ? 'disabled' : ''}>${esc(tx(c, 'desktop.virtual_computers_volume_import'))}</button></div>` : '';
        return `<section class="vc-section-page"><header class="vc-section-header"><div><span class="vc-eyebrow">${esc(tx(c, 'desktop.virtual_computers_title'))}</span><h3>${esc(tx(c, 'desktop.virtual_computers_volumes'))}</h3></div></header>${controls}${state.resourceErrors.volumes ? `<div class="vc-inline-error">${esc(state.resourceErrors.volumes)}</div>` : ''}<section class="vc-ledger vc-volume-list">${rows}</section></section>`;
    }

    function modalPane(state) {
        if (!state.modal) return '';
        const c = state.context;
        const modal = state.modal;
        const error = modal.error ? `<div class="vc-modal-error" role="alert">${esc(modal.error)}</div>` : '';
        if (modal.type === 'launch') {
            const templates = state.templates.length ? state.templates : fallbackTemplates();
            const options = templates.map(template => `<option value="${esc(template.id || template.name)}">${esc(template.name || template.id)}</option>`).join('');
            const volumes = state.volumes.map(volume => `<option value="${esc(volume.id)}">${esc(volume.name || volume.id)}</option>`).join('');
            const unlimited = capabilities(state).persistent ? `<option value="persistent">${esc(tx(c, 'desktop.virtual_computers_unlimited'))}</option>` : '';
            return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal" role="dialog" aria-modal="true" aria-labelledby="vc-launch-title"><header><h3 id="vc-launch-title">${esc(tx(c, 'desktop.virtual_computers_new'))}</h3><button type="button" class="vc-icon-btn" data-action="close-modal" aria-label="${esc(tx(c, 'desktop.close'))}">${icon(state, 'x', '×')}</button></header>${error}<label><span>${esc(tx(c, 'desktop.virtual_computers_template'))}</span><select data-role="template" autofocus>${options}</select></label>${state.templatesFallback ? `<p class="vc-form-note">${esc(tx(c, 'desktop.virtual_computers_templates_fallback'))}</p>` : ''}<label><span>${esc(tx(c, 'desktop.virtual_computers_runtime'))}</span><select data-role="ttl"><option value="300">${esc(formatDuration(300))}</option><option value="600" selected>${esc(formatDuration(600))}</option><option value="900">${esc(formatDuration(900))}</option>${unlimited}</select></label>${capabilities(state).volumes ? `<label><span>${esc(tx(c, 'desktop.virtual_computers_volumes'))}</span><select data-role="launch-volume"><option value="">${esc(tx(c, 'desktop.virtual_computers_volume_none'))}</option>${volumes}</select></label>` : ''}<div class="vc-actions"><button type="button" class="vc-btn" data-action="close-modal">${esc(tx(c, 'desktop.cancel'))}</button><button type="button" class="vc-btn vc-primary" data-action="confirm-launch" ${isPending(state, 'launch') ? 'disabled aria-busy="true"' : ''}>${esc(tx(c, 'desktop.virtual_computers_launch'))}</button></div></div></div>`;
        }
        if (modal.type === 'start') {
            return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal" role="dialog" aria-modal="true" aria-labelledby="vc-task-title"><header><h3 id="vc-task-title">${esc(tx(c, 'desktop.virtual_computers_task_start'))}</h3><button type="button" class="vc-icon-btn" data-action="close-modal" aria-label="${esc(tx(c, 'desktop.close'))}">${icon(state, 'x', '×')}</button></header>${error}<label><span>${esc(tx(c, 'desktop.virtual_computers_template'))}</span><select data-role="task-kind"><option value="shell">shell</option><option value="desktop">desktop</option></select></label><label><span>${esc(tx(c, 'desktop.virtual_computers_task_start'))}</span><textarea data-role="task-instruction" maxlength="400" autofocus></textarea></label><div class="vc-actions"><button type="button" class="vc-btn" data-action="close-modal">${esc(tx(c, 'desktop.cancel'))}</button><button type="button" class="vc-btn vc-primary" data-action="confirm-start-task" ${isPending(state, 'start-task') ? 'disabled' : ''}>${esc(tx(c, 'desktop.virtual_computers_task_start'))}</button></div></div></div>`;
        }
        const definitions = {
            cancel: ['desktop.virtual_computers_modal_cancel_task', 'desktop.virtual_computers_modal_cancel_task_desc', 'confirm-cancel-task', 'desktop.virtual_computers_task_cancel'],
            destroy: ['desktop.virtual_computers_confirm_destroy', 'desktop.virtual_computers_confirm_destroy_desc', 'confirm-destroy', 'desktop.virtual_computers_destroy'],
            'delete-volume': ['desktop.virtual_computers_confirm_delete_volume', 'desktop.virtual_computers_confirm_delete_volume_desc', 'confirm-delete-volume', 'desktop.virtual_computers_volume_delete']
        };
        const definition = definitions[modal.type];
        if (!definition) return '';
        return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal" role="dialog" aria-modal="true" aria-labelledby="vc-confirm-title"><header><h3 id="vc-confirm-title">${esc(tx(c, definition[0]))}</h3><button type="button" class="vc-icon-btn" data-action="close-modal" aria-label="${esc(tx(c, 'desktop.close'))}">${icon(state, 'x', '×')}</button></header>${error}<p>${esc(tx(c, definition[1]))}</p><strong class="vc-confirm-target">${esc(modal.label || modal.id || '')}</strong><div class="vc-actions"><button type="button" class="vc-btn" data-action="close-modal">${esc(tx(c, 'desktop.cancel'))}</button><button type="button" class="vc-btn danger" data-action="${definition[2]}" ${isPending(state, modal.type) ? 'disabled' : ''}>${esc(tx(c, definition[3]))}</button></div></div></div>`;
    }

    function bindActions(state) {
        state.clickHandler = event => {
            const target = event.target.closest('[data-action]');
            if (!target || !state.host.contains(target)) return;
            const action = target.dataset.action;
            const id = target.dataset.id || '';
            if (action === 'refresh') refresh(state);
            else if (action === 'config') window.location.href = '/config#virtual_computers';
            else if (action === 'section') switchSection(state, target.dataset.section || 'machines');
            else if (action === 'new-machine') openModal(state, { type: 'launch' }, target);
            else if (action === 'select-machine') selectMachine(state, id);
            else if (action === 'overview') showOverview(state);
            else if (action === 'confirm-launch') launch(state);
            else if (action === 'destroy') openModal(state, { type: 'destroy', id, label: machineLabel(state, id) }, target);
            else if (action === 'confirm-destroy') destroyMachine(state, state.modal && state.modal.id);
            else if (action === 'screenshot') screenshot(state, id);
            else if (action === 'vnc') openVNC(state, id);
            else if (action === 'start-task') openModal(state, { type: 'start', machineId: id }, target);
            else if (action === 'cancel_agent_task') openModal(state, { type: 'cancel', id, label: id }, target);
            else if (action === 'confirm-cancel-task') cancelTask(state);
            else if (action === 'delete-volume') openModal(state, { type: 'delete-volume', id, label: id }, target);
            else if (action === 'confirm-delete-volume') deleteVolume(state, state.modal && state.modal.id);
            else if (action === 'create-volume') createVolume(state);
            else if (action === 'import-volume') importVolume(state);
            else if (action === 'close-modal') closeModal(state);
            else if (action === 'retry-resource') refreshResource(state, target.dataset.resource || 'status');
            else if (action === 'confirm-start-task') startTask(state);
        };
        state.changeHandler = event => {
            if (event.target.matches('[data-role="task-filter"]')) {
                state.taskMachineFilter = event.target.value || '';
                draw(state);
            }
        };
        state.keyHandler = event => {
            if (state.modal) {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    closeModal(state);
                } else if (event.key === 'Tab') {
                    const focusable = Array.from(state.host.querySelectorAll('[data-role="modal"] button:not([disabled]), [data-role="modal"] select:not([disabled]), [data-role="modal"] textarea:not([disabled]), [data-role="modal"] input:not([disabled])'));
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
        state.host.addEventListener('keydown', state.keyHandler);
    }

    function machineLabel(state, id) {
        const machine = state.machines.find(item => item.id === id);
        return machine ? machine.name || machine.id : id;
    }

    function selectMachine(state, id) {
        if (!id || state.selectedMachineId === id) return;
        disconnectVNC(state);
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.detailMode = 'overview';
        draw(state);
    }

    function switchSection(state, section) {
        if (!['machines', 'tasks', 'volumes'].includes(section) || state.activeSection === section) return;
        disconnectVNC(state);
        state.activeSection = section;
        state.modal = null;
        draw(state);
        if (section === 'tasks' && capabilities(state).agent_tasks) refreshResource(state, 'tasks');
        if (section === 'volumes' && capabilities(state).volumes) refreshResource(state, 'volumes');
    }

    function applyResourceResult(state, resource, result) {
        state.resourceLoading[resource] = false;
        if (result.status === 'fulfilled') {
            state.resourceErrors[resource] = '';
            const body = result.value || {};
            if (resource === 'machines') state.machines = Array.isArray(body.machines) ? body.machines : [];
            else if (resource === 'templates') {
                state.templates = Array.isArray(body.templates) ? body.templates : [];
                state.templatesFallback = state.templates.length === 0;
            } else if (resource === 'tasks') state.tasks = Array.isArray(body.tasks) ? body.tasks : [];
            else if (resource === 'volumes') state.volumes = Array.isArray(body.volumes) ? body.volumes : [];
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
                state.machines = [];
                state.templates = [];
                state.tasks = [];
                state.volumes = [];
                state.resourceLoading.machines = false;
                state.resourceLoading.templates = false;
                reconcileSection(state);
                reconcileVNC(state);
                draw(state);
                return;
            }
            const resources = [
                ['machines', request('/api/virtual-computers/machines')],
                ['templates', request('/api/virtual-computers/templates')]
            ];
            if (status.capabilities && status.capabilities.agent_tasks) {
                state.resourceLoading.tasks = true;
                resources.push(['tasks', request('/api/virtual-computers/tasks?limit=50')]);
            }
            if (status.capabilities && status.capabilities.volumes) {
                state.resourceLoading.volumes = true;
                resources.push(['volumes', request('/api/virtual-computers/volumes')]);
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
        draw(state);
        scheduleTaskRefresh(state);
    }

    async function refreshResource(state, resource) {
        if (resource === 'status' || resource === 'machines' || resource === 'templates') {
            await refresh(state);
            return;
        }
        const paths = {
            tasks: '/api/virtual-computers/tasks?limit=50',
            volumes: '/api/virtual-computers/volumes'
        };
        const path = paths[resource];
        if (!path || state.resourceLoading[resource]) return;
        state.resourceLoading[resource] = true;
        draw(state);
        const result = await Promise.allSettled([request(path)]);
        if (state.disposed) return;
        applyResourceResult(state, resource, result[0]);
        draw(state);
        scheduleTaskRefresh(state);
    }

    function reconcileSelection(state) {
        if (state.selectedMachineId && state.machines.some(item => item.id === state.selectedMachineId)) return;
        state.selectedMachineId = state.machines.length ? state.machines[0].id : null;
        state.selectedShot = null;
        state.screenshotLoading = false;
        state.detailMode = 'overview';
    }

    function reconcileSection(state) {
        const available = state.activeSection === 'machines'
            || (state.activeSection === 'tasks' && capabilities(state).agent_tasks)
            || (state.activeSection === 'volumes' && capabilities(state).volumes);
        if (available) return;
        disconnectVNC(state);
        state.activeSection = 'machines';
    }

    function showOverview(state) {
        disconnectVNC(state);
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

    function openVNC(state, id) {
        const machine = state.machines.find(item => item.id === id);
        if (!canUseVNC(state, machine) || !window.VirtualComputersVNC || typeof window.VirtualComputersVNC.mount !== 'function') {
            notify(state, tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            return;
        }
        disconnectVNC(state);
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

    function disconnectVNC(state) {
        const session = state.vncSession;
        state.vncSession = null;
        state.vncMachineId = null;
        setVNCExpanded(state, false);
        if (session && typeof session.disconnect === 'function') {
            try { session.disconnect(); } catch (_) {}
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

    async function launch(state) {
        if (!state.modal || state.modal.type !== 'launch' || isPending(state, 'launch')) return;
        const runtime = state.host.querySelector('[data-role="ttl"]')?.value || '600';
        const body = {
            template: state.host.querySelector('[data-role="template"]')?.value || 'python'
        };
        if (runtime === 'persistent') body.persistent = true;
        else body.ttl_seconds = Number(runtime);
        const volumeId = state.host.querySelector('[data-role="launch-volume"]')?.value || '';
        if (volumeId) body.volume_id = volumeId;
        const ok = await mutate(state, 'launch', 'machines', '/api/virtual-computers/machines', {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body)
        });
        if (ok) closeModal(state, false);
    }

    async function destroyMachine(state, id) {
        if (!id || isPending(state, 'destroy')) return;
        if (state.vncMachineId === id) showOverview(state);
        const ok = await mutate(state, 'destroy', 'machines', '/api/virtual-computers/machines/' + encodeURIComponent(id), { method: 'DELETE' });
        if (ok) closeModal(state, false);
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
        if (!id) return;
        const requestID = ++state.screenshotRequestID;
        disconnectVNC(state);
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
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + error.message, 'error');
            draw(state);
        }
    }

    function openModal(state, modal, trigger) {
        state.modal = modal;
        state.modalReturnFocus = trigger || state.host.ownerDocument.activeElement;
        draw(state);
        const focusTarget = state.host.querySelector('[data-role="modal"] [autofocus], [data-role="modal"] button');
        if (focusTarget) focusTarget.focus();
    }

    function closeModal(state, restoreFocus = true) {
        const returnFocus = state.modalReturnFocus;
        state.modal = null;
        state.modalReturnFocus = null;
        draw(state);
        if (restoreFocus && returnFocus && typeof returnFocus.focus === 'function') returnFocus.focus();
    }

    async function startTask(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'start' || isPending(state, 'start-task')) return;
        const kind = state.host.querySelector('[data-role="task-kind"]')?.value || 'shell';
        const instruction = state.host.querySelector('[data-role="task-instruction"]')?.value.trim() || '';
        if (!instruction) {
            modal.error = tx(state.context, 'desktop.virtual_computers_section_error');
            draw(state);
            return;
        }
        const ok = await mutate(state, 'start-task', 'tasks', '/api/virtual-computers/tasks', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ machine_id: modal.machineId, kind, instruction })
        });
        if (ok) {
            state.activeSection = 'tasks';
            closeModal(state, false);
        }
    }

    async function cancelTask(state) {
        const taskId = state.modal && state.modal.id;
        if (!taskId || isPending(state, 'cancel')) return;
        const ok = await mutate(state, 'cancel', 'tasks', '/api/virtual-computers/tasks/' + encodeURIComponent(taskId), { method: 'DELETE' });
        if (ok) closeModal(state, false);
    }

    async function createVolume(state) {
        if (isPending(state, 'create-volume')) return;
        const ttl_seconds = Number(state.host.querySelector('[data-role="volume-ttl"]')?.value || 86400);
        await mutate(state, 'create-volume', 'volumes', '/api/virtual-computers/volumes', {
            method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ttl_seconds })
        });
    }

    async function importVolume(state) {
        if (isPending(state, 'import-volume')) return;
        const id = state.host.querySelector('[data-role="volume-import-id"]')?.value.trim() || '';
        if (id) await mutate(state, 'import-volume', 'volumes', '/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'GET' });
    }

    async function deleteVolume(state, id) {
        if (!id || isPending(state, 'delete-volume')) return;
        const ok = await mutate(state, 'delete-volume', 'volumes', '/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'DELETE' });
        if (ok) closeModal(state, false);
    }

    async function mutate(state, actionKey, resource, path, options) {
        setPending(state, actionKey, true);
        if (state.modal) state.modal.error = '';
        draw(state);
        try {
            await request(path, options);
            state.resourceErrors[resource] = '';
            if (resource === 'machines') await refresh(state);
            else await refreshResource(state, resource);
            return true;
        } catch (error) {
            state.resourceErrors[resource] = error.message;
            if (state.modal) state.modal.error = error.message;
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + error.message, 'error');
            return false;
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

    function hasActiveTasks(tasks) {
        return Array.isArray(tasks) && tasks.some(task => task && (task.status === 'queued' || task.status === 'running'));
    }

    function scheduleTaskRefresh(state) {
        if (state.taskRefreshTimer) {
            clearTimeout(state.taskRefreshTimer);
            state.taskRefreshTimer = null;
        }
        if (state.disposed || !hasActiveTasks(state.tasks)) return;
        state.taskRefreshTimer = setTimeout(() => {
            state.taskRefreshTimer = null;
            refreshResource(state, 'tasks');
        }, 2000);
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        state.refreshGeneration++;
        if (state.taskRefreshTimer) clearTimeout(state.taskRefreshTimer);
        disconnectVNC(state);
        if (state.clickHandler) state.host.removeEventListener('click', state.clickHandler);
        if (state.changeHandler) state.host.removeEventListener('change', state.changeHandler);
        if (state.keyHandler) state.host.removeEventListener('keydown', state.keyHandler);
        instances.delete(windowId);
    }

    window.VirtualComputersApp = { render, dispose };
}());
