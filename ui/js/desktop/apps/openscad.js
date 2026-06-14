(function () {
    'use strict';

    const DEFAULT_SOURCE = `// OpenSCAD model
$fn = 72;

module model() {
  difference() {
    cube([40, 30, 12], center = true);
    translate([0, 0, 2]) cylinder(h = 20, r = 8, center = true);
  }
}

model();`;

    const stateByWindow = new Map();

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    function t(ctx, key, fallback) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key, fallback) : fallback;
    }

    function icon(ctx, name, fallback, className, size) {
        return ctx && typeof ctx.iconMarkup === 'function' ? ctx.iconMarkup(name, fallback, className, size) : `<span class="${esc(className || '')}">${esc(fallback || '')}</span>`;
    }

    function render(host, windowId, ctx) {
        const source = ctx && ctx.source ? String(ctx.source) : DEFAULT_SOURCE;
        const state = {
            host,
            windowId,
            ctx: ctx || {},
            source,
            prompt: '',
            exports: new Set(['png', 'stl']),
            renderMode: 'render',
            timeout: 120,
            activeTab: 'preview',
            result: null,
            busy: false,
            stl: null,
            listeners: [],
            eventsAttached: false
        };
        stateByWindow.set(windowId, state);
        draw(state);
        wire(state);
        updateWindowContext(state);
        loadStatus(state);
    }

    function draw(state) {
        const ctx = state.ctx;
        state.host.className = 'openscad-app';
        state.host.innerHTML = `
            <div class="oscad-shell">
                <aside class="oscad-left">
                    <div class="oscad-brand">
                        <div class="oscad-brand-icon">${icon(ctx, 'openscad', 'O', 'oscad-icon', 28)}</div>
                        <div>
                            <h2>${esc(t(ctx, 'desktop.openscad.title', 'OpenSCAD'))}</h2>
                            <p>${esc(t(ctx, 'desktop.openscad.subtitle', 'Parametric CAD compiler'))}</p>
                        </div>
                    </div>
                    <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.prompt', 'Agent prompt'))}</label>
                    <textarea class="oscad-chat" data-oscad-prompt rows="5" placeholder="${esc(t(ctx, 'desktop.openscad.prompt_placeholder', 'Describe the model you want...'))}">${esc(state.prompt)}</textarea>
                    <div class="oscad-row">
                        <button class="oscad-btn oscad-primary" data-oscad-agent>${icon(ctx, 'agent-chat', 'A', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.ask_agent', 'Ask agent'))}</span></button>
                        <button class="oscad-btn" data-oscad-render>${icon(ctx, 'run', 'R', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.render', 'Render'))}</span></button>
                    </div>
                    <div class="oscad-options">
                        <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.exports', 'Exports'))}</label>
                        <div class="oscad-chips">
                            ${['png', 'stl', 'svg', 'pdf', '3mf', 'off', 'dxf'].map(format => `
                                <label class="oscad-chip">
                                    <input type="checkbox" data-oscad-export="${esc(format)}" ${state.exports.has(format) ? 'checked' : ''}>
                                    <span>${esc(format.toUpperCase())}</span>
                                </label>`).join('')}
                        </div>
                        <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.mode', 'Mode'))}</label>
                        <select class="oscad-select" data-oscad-mode>
                            <option value="render" ${state.renderMode === 'render' ? 'selected' : ''}>${esc(t(ctx, 'desktop.openscad.mode_render', 'Render'))}</option>
                            <option value="preview" ${state.renderMode === 'preview' ? 'selected' : ''}>${esc(t(ctx, 'desktop.openscad.mode_preview', 'Preview'))}</option>
                        </select>
                        <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.timeout', 'Timeout'))}</label>
                        <input class="oscad-input" data-oscad-timeout type="number" min="10" max="600" step="10" value="${esc(state.timeout)}">
                    </div>
                    <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.source', 'Source'))}</label>
                    <textarea class="oscad-source" data-oscad-source spellcheck="false">${esc(state.source)}</textarea>
                </aside>
                <main class="oscad-main">
                    <div class="oscad-tabs">
                        ${tabButton(state, 'preview', t(ctx, 'desktop.openscad.tab_preview', 'Preview'))}
                        ${tabButton(state, 'source', t(ctx, 'desktop.openscad.tab_source', 'Source'))}
                        ${tabButton(state, 'files', t(ctx, 'desktop.openscad.tab_files', 'Files'))}
                        ${tabButton(state, 'log', t(ctx, 'desktop.openscad.tab_log', 'Log'))}
                        <button class="oscad-icon-btn" data-oscad-refresh title="${esc(t(ctx, 'desktop.openscad.refresh', 'Refresh'))}">${icon(ctx, 'refresh', 'R', 'oscad-btn-icon', 16)}</button>
                    </div>
                    <div class="oscad-panel" data-oscad-panel></div>
                    <div class="oscad-footer">
                        <div class="oscad-status" data-oscad-status>${esc(t(ctx, 'desktop.openscad.ready', 'Ready'))}</div>
                        <div class="oscad-actions">
                            <button class="oscad-btn" data-oscad-download ${state.result ? '' : 'disabled'}>${icon(ctx, 'download', 'D', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.download', 'Download'))}</span></button>
                            <button class="oscad-btn" data-oscad-save ${state.result ? '' : 'disabled'}>${icon(ctx, 'save', 'S', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.save_desktop', 'Save'))}</span></button>
                            <button class="oscad-btn" data-oscad-fullscreen>${icon(ctx, 'fullscreen', 'F', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.fullscreen', 'Fullscreen'))}</span></button>
                        </div>
                    </div>
                </main>
            </div>`;
        renderPanel(state);
    }

    function tabButton(state, id, label) {
        return `<button class="oscad-tab ${state.activeTab === id ? 'active' : ''}" data-oscad-tab="${esc(id)}">${esc(label)}</button>`;
    }

    function wire(state) {
        const host = state.host;
        host.querySelector('[data-oscad-source]').addEventListener('input', e => {
            state.source = e.target.value;
            updateWindowContext(state);
            if (state.activeTab === 'source') renderPanel(state);
        });
        host.querySelector('[data-oscad-prompt]').addEventListener('input', e => { state.prompt = e.target.value; });
        host.querySelector('[data-oscad-mode]').addEventListener('change', e => { state.renderMode = e.target.value || 'render'; });
        host.querySelector('[data-oscad-timeout]').addEventListener('input', e => { state.timeout = Number(e.target.value || 120); });
        host.querySelectorAll('[data-oscad-export]').forEach(input => {
            input.addEventListener('change', () => {
                const format = input.dataset.oscadExport;
                if (input.checked) state.exports.add(format);
                else state.exports.delete(format);
            });
        });
        host.querySelector('[data-oscad-render]').addEventListener('click', () => renderSource(state));
        host.querySelector('[data-oscad-agent]').addEventListener('click', () => askAgent(state));
        host.querySelector('[data-oscad-save]').addEventListener('click', () => saveJob(state));
        host.querySelector('[data-oscad-download]').addEventListener('click', () => downloadPrimary(state));
        host.querySelector('[data-oscad-fullscreen]').addEventListener('click', () => fullscreenPreview(state));
        host.querySelector('[data-oscad-refresh]').addEventListener('click', () => loadStatus(state));
        host.querySelectorAll('[data-oscad-tab]').forEach(btn => btn.addEventListener('click', () => {
            state.activeTab = btn.dataset.oscadTab;
            draw(state);
        }));
        attachResultListeners(state);
    }

    function attachResultListeners(state) {
        if (state.eventsAttached) return;
        state.eventsAttached = true;
        const onMessage = event => {
            const data = normalizeEventData(event.data);
            applyOpenSCADResultEvent(state, data);
        };
        window.addEventListener('message', onMessage);
        state.listeners.push(() => window.removeEventListener('message', onMessage));
        if (window.AuraSSE && typeof window.AuraSSE.on === 'function') {
            const onDesktopEvent = payload => applyOpenSCADResultEvent(state, payload);
            window.AuraSSE.on('virtual_desktop_event', onDesktopEvent);
            window.AuraSSE.on('openscad_result', onDesktopEvent);
            state.listeners.push(() => {
                if (window.AuraSSE && typeof window.AuraSSE.off === 'function') {
                    window.AuraSSE.off('virtual_desktop_event', onDesktopEvent);
                    window.AuraSSE.off('openscad_result', onDesktopEvent);
                }
            });
        }
    }

    function applyOpenSCADResultEvent(state, data) {
        if (data && data.type === 'virtual_desktop_event' && data.payload) data = data.payload;
        if (data && data.event === 'virtual_desktop_event' && data.detail) data = normalizeEventData(data.detail);
        if (!data || data.type !== 'openscad_result') return;
        state.result = data.payload || data.result || null;
        state.activeTab = 'preview';
        draw(state);
        setStatus(state, t(state.ctx, 'desktop.openscad.render_complete', 'Render complete'));
    }

    async function loadStatus(state) {
        try {
            const body = await state.ctx.api('/api/openscad/status');
            const status = body && body.openscad;
            setStatus(state, status && status.running ? t(state.ctx, 'desktop.openscad.status_running', 'Compiler running') : t(state.ctx, 'desktop.openscad.ready', 'Ready'));
        } catch (err) {
            setStatus(state, err.message || String(err), true);
        }
    }

    async function renderSource(state) {
        if (state.busy) return;
        const exports = Array.from(state.exports);
        if (!exports.length) exports.push('png', 'stl');
        state.busy = true;
        setStatus(state, t(state.ctx, 'desktop.openscad.rendering', 'Rendering...'));
        try {
            const body = await state.ctx.api('/api/openscad/render', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    source_scad: state.source,
                    model_name: 'model',
                    exports,
                    render_mode: state.renderMode,
                    timeout_seconds: state.timeout
                })
            });
            state.result = body.result;
            state.activeTab = 'preview';
            draw(state);
            setStatus(state, t(state.ctx, 'desktop.openscad.render_complete', 'Render complete'));
        } catch (err) {
            setStatus(state, err.message || String(err), true);
        } finally {
            state.busy = false;
        }
    }

    async function askAgent(state) {
        const message = state.prompt.trim();
        if (!message || state.busy) return;
        state.busy = true;
        setStatus(state, t(state.ctx, 'desktop.openscad.agent_working', 'Agent is working...'));
        try {
            const response = await fetch('/api/desktop/chat/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    message,
                    context: {
                        source: 'openscad',
                        origin_app: 'openscad',
                        current_language: 'openscad',
                        current_content: state.source,
                        window_context: {
                            source: 'openscad',
                            app_id: 'openscad',
                            window_id: state.windowId,
                            label: 'OpenSCAD',
                            purpose: 'Create and render OpenSCAD CAD models with openscad_render.'
                        }
                    }
                })
            });
            if (!response.ok || !response.body) throw new Error(await response.text());
            await readChatStream(state, response.body.getReader());
        } catch (err) {
            setStatus(state, err.message || String(err), true);
        } finally {
            state.busy = false;
        }
    }

    async function readChatStream(state, reader) {
        const decoder = new TextDecoder();
        let buffer = '';
        for (;;) {
            const chunk = await reader.read();
            if (chunk.done) break;
            buffer += decoder.decode(chunk.value, { stream: true });
            const parts = buffer.split('\n\n');
            buffer = parts.pop() || '';
            parts.forEach(part => {
                const line = part.split('\n').find(item => item.startsWith('data: '));
                if (!line) return;
                const raw = line.slice(6).trim();
                if (!raw || raw === '[DONE]') return;
                const data = normalizeEventData(raw);
                applyOpenSCADResultEvent(state, data);
                if (data && data.event === 'delta' && data.detail) {
                    setStatus(state, data.detail.slice(-160));
                }
            });
        }
    }

    async function saveJob(state) {
        if (!state.result || !state.result.job_id) return;
        setStatus(state, t(state.ctx, 'desktop.openscad.saving', 'Saving...'));
        try {
            const body = await state.ctx.api(`/api/openscad/jobs/${encodeURIComponent(state.result.job_id)}/save`, { method: 'POST' });
            state.result = body.result;
            draw(state);
            setStatus(state, t(state.ctx, 'desktop.openscad.saved', 'Saved to Desktop'));
        } catch (err) {
            setStatus(state, err.message || String(err), true);
        }
    }

    function downloadPrimary(state) {
        const file = primaryFile(state);
        if (!file) return;
        window.open(file.download_url, '_blank', 'noopener');
    }

    function fullscreenPreview(state) {
        const panel = state.host.querySelector('[data-oscad-panel]');
        if (panel && panel.requestFullscreen) panel.requestFullscreen();
    }

    function renderPanel(state) {
        const panel = state.host.querySelector('[data-oscad-panel]');
        if (!panel) return;
        if (state.activeTab === 'source') {
            panel.innerHTML = `<pre class="oscad-code">${esc(state.source)}</pre>`;
            return;
        }
        if (state.activeTab === 'files') {
            const files = state.result && Array.isArray(state.result.files) ? state.result.files : [];
            panel.innerHTML = files.length ? `<div class="oscad-file-list">${files.map(file => `
                <a class="oscad-file" href="${esc(file.download_url)}" target="_blank" rel="noopener">
                    <span>${esc(file.name)}</span><small>${esc(file.format.toUpperCase())} · ${esc(formatSize(file.size))}</small>
                </a>`).join('')}</div>` : emptyPanel(state, 'desktop.openscad.no_files', 'No files yet');
            return;
        }
        if (state.activeTab === 'log') {
            const log = state.result ? [state.result.stdout, state.result.stderr].filter(Boolean).join('\n') : '';
            panel.innerHTML = log ? `<pre class="oscad-code">${esc(log)}</pre>` : emptyPanel(state, 'desktop.openscad.no_log', 'No log yet');
            return;
        }
        renderPreview(state, panel);
    }

    function renderPreview(state, panel) {
        const file = primaryFile(state);
        if (!file) {
            panel.innerHTML = emptyPanel(state, 'desktop.openscad.no_preview', 'Render a model to see the preview.');
            return;
        }
        if (file.format === 'png') {
            panel.innerHTML = `<img class="oscad-preview-img" src="${esc(file.download_url)}" alt="">`;
            return;
        }
        if (file.format === 'svg') {
            panel.innerHTML = `<object class="oscad-preview-object" data="${esc(file.download_url)}" type="image/svg+xml"></object>`;
            return;
        }
        if (file.format === 'pdf') {
            panel.innerHTML = `<iframe class="oscad-preview-object" src="${esc(file.download_url)}"></iframe>`;
            return;
        }
        if (file.format === 'stl') {
            panel.innerHTML = `<div class="oscad-stl" data-stl-viewer></div>`;
            renderSTL(state, panel.querySelector('[data-stl-viewer]'), file.download_url);
            return;
        }
        panel.innerHTML = `<div class="oscad-empty"><strong>${esc(file.name)}</strong><span>${esc(t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'))}</span></div>`;
    }

    function renderSTL(state, mount, url) {
        const STLLoader = window.THREE && (window.THREE.STLLoader || window.STLLoader);
        const OrbitControls = window.THREE && (window.THREE.OrbitControls || window.OrbitControls);
        if (!mount || !window.THREE || !STLLoader) {
            mount.innerHTML = `<div class="oscad-empty">${esc(t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'))}</div>`;
            return;
        }
        const width = mount.clientWidth || 640;
        const height = mount.clientHeight || 420;
        const scene = new THREE.Scene();
        scene.background = new THREE.Color(0x071018);
        const camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 5000);
        camera.position.set(80, 70, 90);
        const renderer = new THREE.WebGLRenderer({ antialias: true });
        renderer.setSize(width, height);
        mount.innerHTML = '';
        mount.appendChild(renderer.domElement);
        scene.add(new THREE.HemisphereLight(0xffffff, 0x243447, 1.2));
        const light = new THREE.DirectionalLight(0xffffff, 0.9);
        light.position.set(40, 80, 60);
        scene.add(light);
        const controls = OrbitControls ? new OrbitControls(camera, renderer.domElement) : null;
        new STLLoader().load(url, geometry => {
            geometry.computeBoundingBox();
            geometry.center();
            const material = new THREE.MeshStandardMaterial({ color: 0x42d7c8, roughness: 0.42, metalness: 0.12 });
            const mesh = new THREE.Mesh(geometry, material);
            scene.add(mesh);
            const box = new THREE.Box3().setFromObject(mesh);
            const size = box.getSize(new THREE.Vector3()).length() || 80;
            camera.position.set(size, size * 0.75, size);
            camera.lookAt(0, 0, 0);
            animate();
        }, undefined, err => setStatus(state, err && err.message ? err.message : String(err), true));
        function animate() {
            if (!stateByWindow.has(state.windowId)) return;
            requestAnimationFrame(animate);
            if (controls) controls.update();
            renderer.render(scene, camera);
        }
    }

    function primaryFile(state) {
        const files = state.result && Array.isArray(state.result.files) ? state.result.files : [];
        return files.find(file => file.format === 'png') || files.find(file => file.format === 'stl') || files[0] || null;
    }

    function emptyPanel(state, key, fallback) {
        return `<div class="oscad-empty">${esc(t(state.ctx, key, fallback))}</div>`;
    }

    function setStatus(state, message, error) {
        const el = state.host.querySelector('[data-oscad-status]');
        if (el) {
            el.textContent = message || '';
            el.classList.toggle('error', !!error);
        }
    }

    function updateWindowContext(state) {
        if (state.ctx && typeof state.ctx.updateWindowContext === 'function') {
            state.ctx.updateWindowContext(state.windowId, {
                source: 'openscad',
                app_id: 'openscad',
                label: 'OpenSCAD',
                purpose: 'Create and render OpenSCAD CAD models.',
                resources: state.result && state.result.job_id ? [{ kind: 'openscad_job', label: state.result.job_id }] : []
            });
        }
    }

    function normalizeEventData(raw) {
        if (!raw) return null;
        if (typeof raw === 'object') return raw;
        try { return JSON.parse(raw); } catch (_) { return null; }
    }

    function formatSize(size) {
        const n = Number(size || 0);
        if (n > 1024 * 1024) return (n / 1024 / 1024).toFixed(1) + ' MB';
        if (n > 1024) return (n / 1024).toFixed(1) + ' KB';
        return n + ' B';
    }

    function dispose(windowId) {
        const state = stateByWindow.get(windowId);
        if (!state) return;
        state.listeners.forEach(fn => {
            try { fn(); } catch (_) {}
        });
        stateByWindow.delete(windowId);
    }

    window.OpenSCADApp = { render, dispose };
})();
