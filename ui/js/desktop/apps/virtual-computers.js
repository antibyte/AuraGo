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

    async function request(path, options) {
        const resp = await fetch(path, options || {});
        const contentType = resp.headers.get('Content-Type') || '';
        const body = contentType.includes('application/json') ? await resp.json() : await resp.text();
        if (!resp.ok || (body && body.error)) throw new Error((body && (body.error || body.message)) || resp.statusText);
        return body;
    }

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        const state = {
            host,
            context: context || {},
            machines: [],
            volumes: [],
            tasks: [],
            status: null,
            selectedShot: null,
            selectedMachineId: null,
            screenshotLoading: false,
            screenshotRequestID: 0,
            detailMode: 'overview',
            modal: null,
            vncSession: null,
            vncMachineId: null,
            clickHandler: null
        };
        instances.set(windowId, state);
        state.host.innerHTML = `<div class="vc-app">
            <header class="vc-app-toolbar" data-role="toolbar"></header>
            <section class="vc-launch" data-role="launch"></section>
            <main class="vc-layout">
                <section class="vc-list" data-role="list"></section>
                <section class="vc-preview" data-role="preview"></section>
            </main>
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

    function draw(state) {
        const toolbar = state.host.querySelector('[data-role="toolbar"]');
        const launch = state.host.querySelector('[data-role="launch"]');
        const list = state.host.querySelector('[data-role="list"]');
        const preview = state.host.querySelector('[data-role="preview"]');
        const modal = state.host.querySelector('[data-role="modal-host"]');
        if (!toolbar || !launch || !list || !preview || !modal) return;
        toolbar.innerHTML = toolbarPane(state);
        launch.innerHTML = launchPane(state);
        list.innerHTML = machineList(state);
        if (state.detailMode !== 'vnc') preview.innerHTML = detailPane(state);
        modal.innerHTML = modalPane(state);
    }

    function toolbarPane(state) {
        const c = state.context;
        const enabled = state.status && state.status.enabled === true;
        return `<div><h2>${esc(tx(c, 'desktop.virtual_computers_title'))}</h2>
            <span class="vc-status ${enabled ? 'is-ok' : ''}">${esc(enabled ? tx(c, 'desktop.virtual_computers_state_enabled') : tx(c, 'desktop.virtual_computers_state_disabled'))}</span></div>
            <div class="vc-actions">
                <button type="button" class="vc-btn" data-action="overview">${esc(tx(c, 'desktop.virtual_computers_overview'))}</button>
                <button type="button" class="vc-btn" data-action="refresh">${esc(tx(c, 'desktop.virtual_computers_refresh'))}</button>
                <button type="button" class="vc-btn" data-action="config">${esc(tx(c, 'desktop.virtual_computers_open_config'))}</button>
            </div>`;
    }

    function launchPane(state) {
        const c = state.context;
        const mutable = isMutable(state);
        const volumeOptions = state.volumes.map(volume => `<option value="${esc(volume.id)}">${esc(volume.id)}</option>`).join('');
        return `<select data-role="template" ${mutable ? '' : 'disabled'}><option value="python">python</option><option value="desktop">desktop</option></select>
            <input data-role="ttl" type="number" min="15" max="900" value="600" ${mutable ? '' : 'disabled'}>
            ${capabilities(state).volumes ? `<select data-role="launch-volume" ${mutable ? '' : 'disabled'}><option value="">${esc(tx(c, 'desktop.virtual_computers_volume_none'))}</option>${volumeOptions}</select>` : ''}
            <button type="button" class="vc-btn vc-primary" data-action="launch" ${mutable ? '' : 'disabled'}>${esc(tx(c, 'desktop.virtual_computers_launch'))}</button>`;
    }

    function bindActions(state) {
        state.clickHandler = event => {
            const target = event.target.closest('[data-action]');
            if (!target || !state.host.contains(target)) return;
            const action = target.dataset.action;
            const id = target.dataset.id || '';
            if (action === 'refresh') refresh(state);
            else if (action === 'config') window.location.href = '/config#virtual_computers';
            else if (action === 'overview') showOverview(state);
            else if (action === 'launch') launch(state);
            else if (action === 'destroy') destroyMachine(state, id);
            else if (action === 'screenshot') screenshot(state, id);
            else if (action === 'vnc') openVNC(state, id);
            else if (action === 'start-task') openModal(state, { type: 'start', machineId: id });
            else if (action === 'cancel_agent_task') openModal(state, { type: 'cancel', taskId: id });
            else if (action === 'delete-volume') deleteVolume(state, id);
            else if (action === 'create-volume') createVolume(state);
            else if (action === 'import-volume') importVolume(state);
            else if (action === 'close-modal') openModal(state, null);
            else if (action === 'confirm-start-task') startTask(state);
            else if (action === 'confirm-cancel-task') cancelTask(state);
        };
        state.host.addEventListener('click', state.clickHandler);
    }

    function machineList(state) {
        const c = state.context;
        if (!state.status) return `<div class="vc-empty">${esc(tx(c, 'desktop.loading'))}</div>`;
        if (!state.status.enabled) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_state_disabled'))}</div>`;
        if (!state.machines.length) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_empty'))}</div>`;
        const allowTasks = capabilities(state).agent_tasks && isMutable(state);
        return state.machines.map(machine => {
            const id = machine.id || '';
            const status = machine.status || '';
            const template = machine.template || '';
            const ports = Array.isArray(machine.web_ports) ? machine.web_ports : [];
            const links = ports.map(port => `<a class="vc-link" href="/api/virtual-computers/machines/${encodeURIComponent(id)}/web/${Number(port)}/" target="_blank" rel="noopener">${esc(String(port))}</a>`).join('');
            const display = machine.display === true;
            const active = state.selectedMachineId === id;
            return `<article class="vc-machine ${active ? 'is-active' : ''}">
                <div class="vc-machine-main"><strong>${esc(id || template || 'machine')}</strong><span>${esc([template, status].filter(Boolean).join(' / '))}</span></div>
                <div class="vc-machine-links">${links}</div>
                <div class="vc-machine-actions">
                    ${display ? `<button type="button" class="vc-icon-btn" data-action="screenshot" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_screenshot'))}</button>` : `<span class="vc-capability-note">${esc(tx(c, 'desktop.virtual_computers_headless'))}</span>`}
                    ${canUseVNC(state, machine) ? `<button type="button" class="vc-icon-btn vc-primary" data-action="vnc" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_vnc_live'))}</button>` : ''}
                    ${allowTasks ? `<button type="button" class="vc-icon-btn" data-action="start-task" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_task_start'))}</button>` : ''}
                    ${isMutable(state) ? `<button type="button" class="vc-icon-btn danger" data-action="destroy" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_destroy'))}</button>` : ''}
                </div>
            </article>`;
        }).join('');
    }

    function detailPane(state) {
        const c = state.context;
        let content = `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_status'))}</div>`;
        if (state.detailMode === 'screenshot') {
            if (state.screenshotLoading) content = `<div class="vc-empty">${esc(tx(c, 'desktop.loading'))}</div>`;
            else if (state.selectedShot) content = `<img class="vc-shot" src="data:${esc(state.selectedShot.mime_type || 'image/png')};base64,${esc(state.selectedShot.data_base64 || '')}" alt="${esc(tx(c, 'desktop.virtual_computers_screenshot'))}">`;
        }
        return `<div class="vc-detail-stack">
            <nav class="vc-detail-nav" aria-label="${esc(tx(c, 'desktop.virtual_computers_title'))}">
                <button type="button" class="vc-btn ${state.detailMode === 'overview' ? 'active' : ''}" data-action="overview">${esc(tx(c, 'desktop.virtual_computers_overview'))}</button>
                ${state.detailMode === 'screenshot' ? `<span class="vc-detail-label">${esc(tx(c, 'desktop.virtual_computers_screenshot'))}</span>` : ''}
            </nav>
            <div class="vc-detail-content">${content}</div>
            ${taskPane(state)}${volumePane(state)}
        </div>`;
    }

    function taskPane(state) {
        if (!capabilities(state).agent_tasks) return '';
        const c = state.context;
        const rows = state.tasks.length ? state.tasks.map(task => {
            const running = task.status === 'queued' || task.status === 'running';
            return `<div class="vc-ledger-row"><span><strong>${esc(task.kind)}</strong> · ${esc(task.machine_id)} · ${esc(task.status)}</span>${running && isMutable(state) ? `<button type="button" class="vc-icon-btn danger" data-action="cancel_agent_task" data-id="${esc(task.id)}">${esc(tx(c, 'desktop.virtual_computers_task_cancel'))}</button>` : ''}</div>`;
        }).join('') : `<div class="vc-empty compact">${esc(tx(c, 'desktop.virtual_computers_tasks_empty'))}</div>`;
        return `<section class="vc-ledger"><h3>${esc(tx(c, 'desktop.virtual_computers_tasks'))}</h3>${rows}</section>`;
    }

    function volumePane(state) {
        if (!capabilities(state).volumes) return '';
        const c = state.context;
        const mutable = isMutable(state);
        const controls = mutable ? `<div class="vc-ledger-controls">
            <input data-role="volume-ttl" type="number" min="1" value="86400"><button type="button" class="vc-btn" data-action="create-volume">${esc(tx(c, 'desktop.virtual_computers_volume_create'))}</button>
            <input data-role="volume-import-id" type="text" placeholder="volume_id"><button type="button" class="vc-btn" data-action="import-volume">${esc(tx(c, 'desktop.virtual_computers_volume_import'))}</button>
        </div>` : '';
        const rows = state.volumes.length ? state.volumes.map(volume => `<div class="vc-ledger-row"><span>${esc(volume.id)} · ${esc(volume.verification_status || '')}</span>${mutable ? `<button type="button" class="vc-icon-btn danger" data-action="delete-volume" data-id="${esc(volume.id)}">${esc(tx(c, 'desktop.virtual_computers_volume_delete'))}</button>` : ''}</div>`).join('') : `<div class="vc-empty compact">${esc(tx(c, 'desktop.virtual_computers_volumes_empty'))}</div>`;
        return `<section class="vc-ledger"><h3>${esc(tx(c, 'desktop.virtual_computers_volumes'))}</h3>${controls}${rows}</section>`;
    }

    function modalPane(state) {
        if (!state.modal) return '';
        const c = state.context;
        if (state.modal.type === 'start') {
            return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal"><h3>${esc(tx(c, 'desktop.virtual_computers_task_start'))}</h3><select data-role="task-kind"><option value="shell">shell</option><option value="desktop">desktop</option></select><textarea data-role="task-instruction" maxlength="400"></textarea><div class="vc-actions"><button type="button" class="vc-btn" data-action="close-modal">${esc(tx(c, 'desktop.cancel'))}</button><button type="button" class="vc-btn vc-primary" data-action="confirm-start-task">${esc(tx(c, 'desktop.virtual_computers_task_start'))}</button></div></div></div>`;
        }
        return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal"><h3>${esc(tx(c, 'desktop.virtual_computers_modal_cancel_task'))}</h3><p>${esc(tx(c, 'desktop.virtual_computers_modal_cancel_task_desc'))}</p><div class="vc-actions"><button type="button" class="vc-btn" data-action="close-modal">${esc(tx(c, 'desktop.cancel'))}</button><button type="button" class="vc-btn danger" data-action="confirm-cancel-task">${esc(tx(c, 'desktop.virtual_computers_task_cancel'))}</button></div></div></div>`;
    }

    async function refresh(state) {
        try {
            const status = await request('/api/virtual-computers/setup/status');
            let machines = [];
            let volumes = [];
            let tasks = [];
            if (status.enabled) {
                const body = await request('/api/virtual-computers/machines');
                machines = Array.isArray(body.machines) ? body.machines : [];
                if (status.capabilities && status.capabilities.volumes) {
                    const volumeBody = await request('/api/virtual-computers/volumes');
                    volumes = Array.isArray(volumeBody.volumes) ? volumeBody.volumes : [];
                }
                if (status.capabilities && status.capabilities.agent_tasks) {
                    const taskBody = await request('/api/virtual-computers/tasks?limit=50');
                    tasks = Array.isArray(taskBody.tasks) ? taskBody.tasks : [];
                }
            }
            state.status = status;
            state.machines = machines;
            state.volumes = volumes;
            state.tasks = tasks;
        } catch (e) {
            state.status = state.status || { enabled: false };
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error');
        }
        reconcileVNC(state);
        draw(state);
    }

    function showOverview(state) {
        disconnectVNC(state);
        state.detailMode = 'overview';
        state.selectedMachineId = null;
        state.selectedShot = null;
        state.screenshotLoading = false;
        draw(state);
    }

    function openVNC(state, id) {
        const machine = state.machines.find(item => item.id === id);
        if (!canUseVNC(state, machine) || !window.VirtualComputersVNC || typeof window.VirtualComputersVNC.mount !== 'function') {
            notify(state, tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            return;
        }
        disconnectVNC(state);
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
                toggleMaximize: state.context.toggleMaximize,
                onClose: () => {
                    if (state.detailMode === 'vnc' && state.vncMachineId === machine.id) showOverview(state);
                }
            });
        } catch (_) {
            state.vncSession = null;
            state.vncMachineId = null;
            state.detailMode = 'overview';
            state.selectedMachineId = null;
            notify(state, tx(state.context, 'desktop.virtual_computers_vnc_unavailable'), 'error');
            draw(state);
        }
    }

    function disconnectVNC(state) {
        const session = state.vncSession;
        state.vncSession = null;
        state.vncMachineId = null;
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
        state.selectedMachineId = null;
        state.selectedShot = null;
    }

    async function launch(state) {
        const body = { template: state.host.querySelector('[data-role="template"]')?.value || 'python', ttl_seconds: Number(state.host.querySelector('[data-role="ttl"]')?.value || 600) };
        const volumeId = state.host.querySelector('[data-role="launch-volume"]')?.value || '';
        if (volumeId) body.volume_id = volumeId;
        await mutate(state, '/api/virtual-computers/machines', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    }

    async function destroyMachine(state, id) {
        if (!id) return;
        if (state.vncMachineId === id) showOverview(state);
        await mutate(state, '/api/virtual-computers/machines/' + encodeURIComponent(id), { method: 'DELETE' });
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
        state.detailMode = 'screenshot';
        state.selectedMachineId = id;
        state.selectedShot = null;
        state.screenshotLoading = true;
        draw(state);
        try {
            const body = await request('/api/virtual-computers/machines/' + encodeURIComponent(id) + '/screenshot');
            if (settleScreenshot(state, id, requestID, body.screenshot)) draw(state);
        } catch (e) {
            if (!settleScreenshot(state, id, requestID, null)) return;
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error');
            draw(state);
        }
    }

    function openModal(state, modal) { state.modal = modal; draw(state); }

    async function startTask(state) {
        const modal = state.modal;
        if (!modal || modal.type !== 'start') return;
        const kind = state.host.querySelector('[data-role="task-kind"]')?.value || 'shell';
        const instruction = state.host.querySelector('[data-role="task-instruction"]')?.value.trim() || '';
        if (!instruction) return;
        state.modal = null;
        await mutate(state, '/api/virtual-computers/tasks', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ machine_id: modal.machineId, kind, instruction }) });
    }

    async function cancelTask(state) {
        const taskId = state.modal && state.modal.taskId;
        state.modal = null;
        if (taskId) await mutate(state, '/api/virtual-computers/tasks/' + encodeURIComponent(taskId), { method: 'DELETE' });
    }

    async function createVolume(state) {
        const ttl_seconds = Number(state.host.querySelector('[data-role="volume-ttl"]')?.value || 86400);
        await mutate(state, '/api/virtual-computers/volumes', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ttl_seconds }) });
    }

    async function importVolume(state) {
        const id = state.host.querySelector('[data-role="volume-import-id"]')?.value.trim() || '';
        if (id) await mutate(state, '/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'GET' });
    }

    async function deleteVolume(state, id) {
        if (id) await mutate(state, '/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'DELETE' });
    }

    async function mutate(state, path, options) {
        try { await request(path, options); await refresh(state); }
        catch (e) { notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error'); }
    }

    function notify(state, message, type) {
        if (state.context && typeof state.context.notify === 'function') state.context.notify(message, { type: type || 'info' });
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        disconnectVNC(state);
        if (state.clickHandler) state.host.removeEventListener('click', state.clickHandler);
        instances.delete(windowId);
    }

    window.VirtualComputersApp = { render, dispose };
}());
