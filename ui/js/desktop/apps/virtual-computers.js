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
        const state = { host, context: context || {}, machines: [], volumes: [], tasks: [], status: null, selectedShot: null, modal: null };
        instances.set(windowId, state);
        draw(state);
        refresh(state);
    }

    function capabilities(state) {
        return (state.status && state.status.capabilities) || {};
    }

    function draw(state) {
        const c = state.context;
        const enabled = state.status && state.status.enabled === true;
        const mutable = enabled && state.status.readonly !== true;
        const caps = capabilities(state);
        const volumeOptions = state.volumes.map(volume => `<option value="${esc(volume.id)}">${esc(volume.id)}</option>`).join('');
        state.host.innerHTML = `
            <div class="vc-app">
                <header class="vc-app-toolbar">
                    <div><h2>${esc(tx(c, 'desktop.virtual_computers_title'))}</h2>
                    <span class="vc-status ${enabled ? 'is-ok' : ''}">${esc(enabled ? tx(c, 'desktop.virtual_computers_state_enabled') : tx(c, 'desktop.virtual_computers_state_disabled'))}</span></div>
                    <div class="vc-actions">
                        <button class="vc-btn" data-action="refresh">${esc(tx(c, 'desktop.virtual_computers_refresh'))}</button>
                        <button class="vc-btn" data-action="config">${esc(tx(c, 'desktop.virtual_computers_open_config'))}</button>
                    </div>
                </header>
                <section class="vc-launch">
                    <select data-role="template" ${mutable ? '' : 'disabled'}><option value="python">python</option><option value="desktop">desktop</option></select>
                    <input data-role="ttl" type="number" min="15" max="900" value="600" ${mutable ? '' : 'disabled'}>
                    ${caps.volumes ? `<select data-role="launch-volume" ${mutable ? '' : 'disabled'}><option value="">${esc(tx(c, 'desktop.virtual_computers_volume_none'))}</option>${volumeOptions}</select>` : ''}
                    <button class="vc-btn vc-primary" data-action="launch" ${mutable ? '' : 'disabled'}>${esc(tx(c, 'desktop.virtual_computers_launch'))}</button>
                </section>
                <main class="vc-layout">
                    <section class="vc-list" data-role="list">${machineList(state)}</section>
                    <section class="vc-preview" data-role="preview">${detailPane(state)}</section>
                </main>
                ${modalPane(state)}
            </div>`;
        bindActions(state);
    }

    function bindActions(state) {
        state.host.querySelector('[data-action="refresh"]')?.addEventListener('click', () => refresh(state));
        state.host.querySelector('[data-action="config"]')?.addEventListener('click', () => { window.location.href = '/config#virtual_computers'; });
        state.host.querySelector('[data-action="launch"]')?.addEventListener('click', () => launch(state));
        state.host.querySelectorAll('[data-action="destroy"]').forEach(btn => btn.addEventListener('click', () => destroyMachine(state, btn.dataset.id)));
        state.host.querySelectorAll('[data-action="screenshot"]').forEach(btn => btn.addEventListener('click', () => screenshot(state, btn.dataset.id)));
        state.host.querySelectorAll('[data-action="start-task"]').forEach(btn => btn.addEventListener('click', () => openModal(state, { type: 'start', machineId: btn.dataset.id })));
        state.host.querySelectorAll('[data-action="cancel_agent_task"]').forEach(btn => btn.addEventListener('click', () => openModal(state, { type: 'cancel', taskId: btn.dataset.id })));
        state.host.querySelectorAll('[data-action="delete-volume"]').forEach(btn => btn.addEventListener('click', () => deleteVolume(state, btn.dataset.id)));
        state.host.querySelector('[data-action="create-volume"]')?.addEventListener('click', () => createVolume(state));
        state.host.querySelector('[data-action="import-volume"]')?.addEventListener('click', () => importVolume(state));
        state.host.querySelector('[data-action="close-modal"]')?.addEventListener('click', () => openModal(state, null));
        state.host.querySelector('[data-action="confirm-start-task"]')?.addEventListener('click', () => startTask(state));
        state.host.querySelector('[data-action="confirm-cancel-task"]')?.addEventListener('click', () => cancelTask(state));
    }

    function machineList(state) {
        const c = state.context;
        if (!state.status) return `<div class="vc-empty">${esc(tx(c, 'desktop.loading'))}</div>`;
        if (!state.status.enabled) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_state_disabled'))}</div>`;
        if (!state.machines.length) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_empty'))}</div>`;
        const allowTasks = capabilities(state).agent_tasks && state.status.readonly !== true;
        return state.machines.map(machine => {
            const id = machine.id || '';
            const status = machine.status || '';
            const template = machine.template || '';
            const ports = Array.isArray(machine.web_ports) ? machine.web_ports : [];
            const links = ports.map(port => `<a class="vc-link" href="/api/virtual-computers/machines/${encodeURIComponent(id)}/web/${Number(port)}/" target="_blank">${esc(String(port))}</a>`).join('');
            const display = machine.display === true;
            return `<article class="vc-machine">
                <div class="vc-machine-main"><strong>${esc(id || template || 'machine')}</strong><span>${esc([template, status].filter(Boolean).join(' / '))}</span></div>
                <div class="vc-machine-links">${links}</div>
                <div class="vc-machine-actions">
                    ${display ? `<button class="vc-icon-btn" data-action="screenshot" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_screenshot'))}</button>` : `<span class="vc-capability-note">${esc(tx(c, 'desktop.virtual_computers_headless'))}</span>`}
                    ${allowTasks ? `<button class="vc-icon-btn" data-action="start-task" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_task_start'))}</button>` : ''}
                    ${state.status.readonly ? '' : `<button class="vc-icon-btn danger" data-action="destroy" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_destroy'))}</button>`}
                </div>
            </article>`;
        }).join('');
    }

    function detailPane(state) {
        const c = state.context;
        const shot = state.selectedShot
            ? `<img class="vc-shot" src="data:${esc(state.selectedShot.mime_type || 'image/png')};base64,${esc(state.selectedShot.data_base64 || '')}" alt="">`
            : `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_status'))}</div>`;
        return `<div class="vc-detail-stack">${shot}${taskPane(state)}${volumePane(state)}</div>`;
    }

    function taskPane(state) {
        if (!capabilities(state).agent_tasks) return '';
        const c = state.context;
        const rows = state.tasks.length ? state.tasks.map(task => {
            const running = task.status === 'queued' || task.status === 'running';
            return `<div class="vc-ledger-row"><span><strong>${esc(task.kind)}</strong> · ${esc(task.machine_id)} · ${esc(task.status)}</span>${running && !state.status.readonly ? `<button class="vc-icon-btn danger" data-action="cancel_agent_task" data-id="${esc(task.id)}">${esc(tx(c, 'desktop.virtual_computers_task_cancel'))}</button>` : ''}</div>`;
        }).join('') : `<div class="vc-empty compact">${esc(tx(c, 'desktop.virtual_computers_tasks_empty'))}</div>`;
        return `<section class="vc-ledger"><h3>${esc(tx(c, 'desktop.virtual_computers_tasks'))}</h3>${rows}</section>`;
    }

    function volumePane(state) {
        if (!capabilities(state).volumes) return '';
        const c = state.context;
        const mutable = !state.status.readonly;
        const controls = mutable ? `<div class="vc-ledger-controls">
            <input data-role="volume-ttl" type="number" min="1" value="86400"><button class="vc-btn" data-action="create-volume">${esc(tx(c, 'desktop.virtual_computers_volume_create'))}</button>
            <input data-role="volume-import-id" type="text" placeholder="volume_id"><button class="vc-btn" data-action="import-volume">${esc(tx(c, 'desktop.virtual_computers_volume_import'))}</button>
        </div>` : '';
        const rows = state.volumes.length ? state.volumes.map(volume => `<div class="vc-ledger-row"><span>${esc(volume.id)} · ${esc(volume.verification_status || '')}</span>${mutable ? `<button class="vc-icon-btn danger" data-action="delete-volume" data-id="${esc(volume.id)}">${esc(tx(c, 'desktop.virtual_computers_volume_delete'))}</button>` : ''}</div>`).join('') : `<div class="vc-empty compact">${esc(tx(c, 'desktop.virtual_computers_volumes_empty'))}</div>`;
        return `<section class="vc-ledger"><h3>${esc(tx(c, 'desktop.virtual_computers_volumes'))}</h3>${controls}${rows}</section>`;
    }

    function modalPane(state) {
        if (!state.modal) return '';
        const c = state.context;
        if (state.modal.type === 'start') {
            return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal"><h3>${esc(tx(c, 'desktop.virtual_computers_task_start'))}</h3><select data-role="task-kind"><option value="shell">shell</option><option value="desktop">desktop</option></select><textarea data-role="task-instruction" maxlength="400"></textarea><div class="vc-actions"><button class="vc-btn" data-action="close-modal">${esc(tx(c, 'desktop.cancel'))}</button><button class="vc-btn vc-primary" data-action="confirm-start-task">${esc(tx(c, 'desktop.virtual_computers_task_start'))}</button></div></div></div>`;
        }
        return `<div class="vc-modal-backdrop" data-role="modal"><div class="vc-modal"><h3>${esc(tx(c, 'desktop.virtual_computers_modal_cancel_task'))}</h3><p>${esc(tx(c, 'desktop.virtual_computers_modal_cancel_task_desc'))}</p><div class="vc-actions"><button class="vc-btn" data-action="close-modal">${esc(tx(c, 'desktop.cancel'))}</button><button class="vc-btn danger" data-action="confirm-cancel-task">${esc(tx(c, 'desktop.virtual_computers_task_cancel'))}</button></div></div></div>`;
    }

    async function refresh(state) {
        try {
            state.status = await request('/api/virtual-computers/setup/status');
            if (state.status.enabled) {
                const body = await request('/api/virtual-computers/machines');
                state.machines = Array.isArray(body.machines) ? body.machines : [];
                if (capabilities(state).volumes) {
                    const volumes = await request('/api/virtual-computers/volumes');
                    state.volumes = Array.isArray(volumes.volumes) ? volumes.volumes : [];
                }
                if (capabilities(state).agent_tasks) {
                    const tasks = await request('/api/virtual-computers/tasks?limit=50');
                    state.tasks = Array.isArray(tasks.tasks) ? tasks.tasks : [];
                }
            } else {
                state.machines = []; state.volumes = []; state.tasks = [];
            }
        } catch (e) {
            state.status = state.status || { enabled: false };
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error');
        }
        draw(state);
    }

    async function launch(state) {
        const body = { template: state.host.querySelector('[data-role="template"]')?.value || 'python', ttl_seconds: Number(state.host.querySelector('[data-role="ttl"]')?.value || 600) };
        const volumeId = state.host.querySelector('[data-role="launch-volume"]')?.value || '';
        if (volumeId) body.volume_id = volumeId;
        await mutate(state, '/api/virtual-computers/machines', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    }

    async function destroyMachine(state, id) { if (id) await mutate(state, '/api/virtual-computers/machines/' + encodeURIComponent(id), { method: 'DELETE' }); }

    async function screenshot(state, id) {
        if (!id) return;
        try { const body = await request('/api/virtual-computers/machines/' + encodeURIComponent(id) + '/screenshot'); state.selectedShot = body.screenshot || null; draw(state); }
        catch (e) { notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error'); }
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

    async function deleteVolume(state, id) { if (id) await mutate(state, '/api/virtual-computers/volumes/' + encodeURIComponent(id), { method: 'DELETE' }); }

    async function mutate(state, path, options) {
        try { await request(path, options); await refresh(state); }
        catch (e) { notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error'); }
    }

    function notify(state, message, type) { if (state.context && typeof state.context.notify === 'function') state.context.notify(message, { type: type || 'info' }); }
    function dispose(windowId) { instances.delete(windowId); }
    window.VirtualComputersApp = { render, dispose };
}());
