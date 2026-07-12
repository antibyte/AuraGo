(function () {
    'use strict';

    const instances = new Map();

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>'"]/g, ch => ({
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            "'": '&#39;',
            '"': '&quot;'
        }[ch]));
    }

    function tx(ctx, key) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key) : key;
    }

    async function request(path, options) {
        const resp = await fetch(path, options || {});
        const contentType = resp.headers.get('Content-Type') || '';
        const body = contentType.includes('application/json') ? await resp.json() : await resp.text();
        if (!resp.ok || (body && body.error)) {
            throw new Error((body && (body.error || body.message)) || resp.statusText);
        }
        return body;
    }

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        const state = {
            host,
            context: context || {},
            machines: [],
            status: null,
            selectedShot: null
        };
        instances.set(windowId, state);
        draw(state);
        refresh(state);
    }

    function draw(state) {
        const c = state.context;
        const enabled = state.status && state.status.enabled === true;
        state.host.innerHTML = `
            <div class="vc-app">
                <header class="vc-app-toolbar">
                    <div>
                        <h2>${esc(tx(c, 'desktop.virtual_computers_title'))}</h2>
                        <span class="vc-status ${enabled ? 'is-ok' : ''}">${esc(enabled ? tx(c, 'desktop.virtual_computers_state_enabled') : tx(c, 'desktop.virtual_computers_state_disabled'))}</span>
                    </div>
                    <div class="vc-actions">
                        <button class="vc-btn" data-action="refresh">${esc(tx(c, 'desktop.virtual_computers_refresh'))}</button>
                        <button class="vc-btn" data-action="config">${esc(tx(c, 'desktop.virtual_computers_open_config'))}</button>
                    </div>
                </header>
                <section class="vc-launch">
                    <select data-role="template" ${enabled ? '' : 'disabled'}>
                        <option value="python">python</option>
                        <option value="desktop">desktop</option>
                    </select>
                    <input data-role="ttl" type="number" min="15" max="900" value="600" ${enabled ? '' : 'disabled'}>
                    <button class="vc-btn vc-primary" data-action="launch" ${enabled ? '' : 'disabled'}>${esc(tx(c, 'desktop.virtual_computers_launch'))}</button>
                </section>
                <main class="vc-layout">
                    <section class="vc-list" data-role="list">${machineList(state)}</section>
                    <section class="vc-preview" data-role="preview">${previewPane(state)}</section>
                </main>
            </div>`;

        state.host.querySelector('[data-action="refresh"]')?.addEventListener('click', () => refresh(state));
        state.host.querySelector('[data-action="config"]')?.addEventListener('click', () => { window.location.href = '/config#virtual_computers'; });
        state.host.querySelector('[data-action="launch"]')?.addEventListener('click', () => launch(state));
        state.host.querySelectorAll('[data-action="destroy"]').forEach(btn => {
            btn.addEventListener('click', () => destroyMachine(state, btn.dataset.id));
        });
        state.host.querySelectorAll('[data-action="screenshot"]').forEach(btn => {
            btn.addEventListener('click', () => screenshot(state, btn.dataset.id));
        });
    }

    function machineList(state) {
        const c = state.context;
        if (!state.status) return `<div class="vc-empty">${esc(tx(c, 'desktop.loading'))}</div>`;
        if (!state.status.enabled) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_state_disabled'))}</div>`;
        if (!state.machines.length) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_empty'))}</div>`;
        return state.machines.map(machine => {
            const id = machine.id || machine.ID || '';
            const status = machine.status || machine.Status || '';
            const template = machine.template || machine.Template || '';
            const ports = Array.isArray(machine.web_ports) ? machine.web_ports : [];
            const links = ports.map(port => `<a class="vc-link" href="/api/virtual-computers/machines/${encodeURIComponent(id)}/web/${Number(port)}/" target="_blank">${esc(String(port))}</a>`).join('');
            return `<article class="vc-machine">
                <div class="vc-machine-main">
                    <strong>${esc(id || template || 'machine')}</strong>
                    <span>${esc([template, status].filter(Boolean).join(' / '))}</span>
                </div>
                <div class="vc-machine-links">${links}</div>
                <div class="vc-machine-actions">
                    <button class="vc-icon-btn" data-action="screenshot" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_screenshot'))}</button>
                    <button class="vc-icon-btn danger" data-action="destroy" data-id="${esc(id)}">${esc(tx(c, 'desktop.virtual_computers_destroy'))}</button>
                </div>
            </article>`;
        }).join('');
    }

    function previewPane(state) {
        const c = state.context;
        if (!state.selectedShot) return `<div class="vc-empty">${esc(tx(c, 'desktop.virtual_computers_status'))}</div>`;
        return `<img class="vc-shot" src="data:${esc(state.selectedShot.mime_type || 'image/png')};base64,${esc(state.selectedShot.data_base64 || '')}" alt="">`;
    }

    async function refresh(state) {
        try {
            state.status = await request('/api/virtual-computers/setup/status');
            if (state.status.enabled) {
                const body = await request('/api/virtual-computers/machines');
                state.machines = Array.isArray(body.machines) ? body.machines : [];
            } else {
                state.machines = [];
            }
        } catch (e) {
            state.status = { enabled: false };
            state.machines = [];
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error');
        }
        draw(state);
    }

    async function launch(state) {
        const template = state.host.querySelector('[data-role="template"]')?.value || 'python';
        const ttl = Number(state.host.querySelector('[data-role="ttl"]')?.value || 600);
        try {
            await request('/api/virtual-computers/machines', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ template, ttl_seconds: ttl })
            });
            await refresh(state);
        } catch (e) {
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error');
        }
    }

    async function destroyMachine(state, id) {
        if (!id) return;
        try {
            await request('/api/virtual-computers/machines/' + encodeURIComponent(id), { method: 'DELETE' });
            await refresh(state);
        } catch (e) {
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error');
        }
    }

    async function screenshot(state, id) {
        if (!id) return;
        try {
            const body = await request('/api/virtual-computers/machines/' + encodeURIComponent(id) + '/screenshot');
            state.selectedShot = body.screenshot || null;
            draw(state);
        } catch (e) {
            notify(state, tx(state.context, 'desktop.virtual_computers_error') + ' ' + e.message, 'error');
        }
    }

    function notify(state, message, type) {
        if (state.context && typeof state.context.notify === 'function') {
            state.context.notify(message, { type: type || 'info' });
        }
    }

    function dispose(windowId) {
        instances.delete(windowId);
    }

    window.VirtualComputersApp = { render, dispose };
}());
