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

    const OPENSCAD_DRAFT_KEY = 'aurago.desktop.openscad.draft';
    const OPENSCAD_DRAFT_MAX_SOURCE = 480 * 1024;
    const OPENSCAD_DRAFT_VERSION = 1;
    let openSCADDraftSaveTimer = null;

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

    function isOpenSCADReadOnly(ctx) {
        return !!(ctx && ctx.readonly);
    }

    function parseDefinesText(text) {
        const defines = [];
        String(text || '').split(/\r?\n/).forEach(line => {
            const trimmed = line.trim();
            if (!trimmed || trimmed.startsWith('#') || trimmed.startsWith('//')) return;
            const eq = trimmed.indexOf('=');
            if (eq < 1) return;
            const name = trimmed.slice(0, eq).trim();
            const value = trimmed.slice(eq + 1).trim();
            if (!name) return;
            defines.push({ name, value });
        });
        return defines;
    }

    function readOpenSCADDraft() {
        try {
            const raw = localStorage.getItem(OPENSCAD_DRAFT_KEY);
            if (!raw) return null;
            const data = JSON.parse(raw);
            if (!data || typeof data !== 'object' || data.v !== OPENSCAD_DRAFT_VERSION) return null;
            return data;
        } catch (_) {
            return null;
        }
    }

    function openSCADDraftFromState(state) {
        const source = String(state.source || '');
        if (source.length > OPENSCAD_DRAFT_MAX_SOURCE) return null;
        return {
            v: OPENSCAD_DRAFT_VERSION,
            source,
            definesText: String(state.definesText || ''),
            prompt: String(state.prompt || ''),
            renderMode: state.renderMode === 'preview' ? 'preview' : 'render',
            timeout: Math.min(600, Math.max(10, Number(state.timeout) || 120)),
            exports: Array.from(state.exports || []),
            activeTab: ['source', 'files', 'log'].includes(state.activeTab) ? state.activeTab : 'source',
            lightPreview: !!state.lightPreview,
            showAxes: state.showAxes !== false,
            savedAt: Date.now()
        };
    }

    function persistOpenSCADDraft(state) {
        if (!state || isOpenSCADReadOnly(state.ctx)) return;
        const payload = openSCADDraftFromState(state);
        if (!payload) return;
        try {
            localStorage.setItem(OPENSCAD_DRAFT_KEY, JSON.stringify(payload));
        } catch (_) {}
    }

    function scheduleOpenSCADDraftSave(state) {
        if (!state || isOpenSCADReadOnly(state.ctx)) return;
        if (openSCADDraftSaveTimer) window.clearTimeout(openSCADDraftSaveTimer);
        openSCADDraftSaveTimer = window.setTimeout(() => {
            openSCADDraftSaveTimer = null;
            persistOpenSCADDraft(state);
        }, 400);
    }

    function applyOpenSCADDraftToState(state, draft, opts) {
        if (!draft) return;
        const skipSource = opts && opts.skipSource;
        if (!skipSource && typeof draft.source === 'string' && draft.source.length && draft.source.length <= OPENSCAD_DRAFT_MAX_SOURCE) {
            state.source = draft.source;
        }
        if (typeof draft.definesText === 'string') state.definesText = draft.definesText;
        if (typeof draft.prompt === 'string') state.prompt = draft.prompt;
        if (draft.renderMode === 'preview' || draft.renderMode === 'render') state.renderMode = draft.renderMode;
        if (Number.isFinite(Number(draft.timeout))) state.timeout = Math.min(600, Math.max(10, Number(draft.timeout)));
        if (Array.isArray(draft.exports) && draft.exports.length) {
            state.exports = new Set(draft.exports.map(f => String(f).toLowerCase()).filter(Boolean));
        }
        if (['source', 'files', 'log'].includes(draft.activeTab)) state.activeTab = draft.activeTab;
        if (typeof draft.lightPreview === 'boolean') state.lightPreview = draft.lightPreview;
        if (typeof draft.showAxes === 'boolean') state.showAxes = draft.showAxes;
    }

    function mergeOpenSCADLaunchContext(ctx, draft) {
        const merged = Object.assign({}, ctx || {});
        if (draft && typeof draft.source === 'string' && draft.source.length) {
            if (!merged.source) merged.source = draft.source;
        }
        return merged;
    }

    function definesTextFromRows(rows) {
        if (!Array.isArray(rows) || !rows.length) return '';
        return rows.map(row => `${String(row.name || '').trim()}=${String(row.value ?? '').trim()}`).join('\n');
    }


    function render(host, windowId, ctx) {
        const launchCtx = ctx || {};
        const draft = readOpenSCADDraft();
        const mergedCtx = mergeOpenSCADLaunchContext(launchCtx, draft);
        const explicitSource = launchCtx.source != null && String(launchCtx.source).trim() !== '';
        const source = explicitSource
            ? String(launchCtx.source)
            : (mergedCtx.source ? String(mergedCtx.source) : DEFAULT_SOURCE);
        const state = {
            host,
            windowId,
            ctx: mergedCtx,
            source,
            prompt: '',
            exports: new Set(['png', 'stl']),
            renderMode: 'render',
            timeout: 120,
            definesText: '',
            activeTab: 'source',
            result: null,
            sourceDirty: false,
            lightPreview: false,
            showAxes: true,
            busy: false,
            busyMode: '',
            cancelRequested: false,
            shellReady: false,
            preview3D: null,
            stl: null,
            previewStlURL: '',
            previewCleanup: null,
            renderAbort: null,
            agentAbort: null,
            statusMessage: '',
            statusError: false,
            listeners: [],
            eventsAttached: false
        };
        applyOpenSCADDraftToState(state, draft, { skipSource: explicitSource });
        stateByWindow.set(windowId, state);
        draw(state);
        updateWindowContext(state);
        loadStatus(state);
    }

    function ensureShell(state) {
        if (state.shellReady) return;
        const ctx = state.ctx;
        const ro = isOpenSCADReadOnly(ctx);
        state.host.className = 'openscad-app';
        state.host.innerHTML = `
            <div class="oscad-workbench" data-oscad-workbench>
                <aside class="oscad-agent-panel" data-oscad-agent-panel>
                    <div class="oscad-brand">
                        <div class="oscad-brand-icon">${icon(ctx, 'openscad', 'O', 'oscad-icon', 28)}</div>
                        <div>
                            <h2>${esc(t(ctx, 'desktop.openscad.title', 'OpenSCAD'))}</h2>
                            <p>${esc(t(ctx, 'desktop.openscad.subtitle', 'Parametric CAD compiler'))}</p>
                        </div>
                    </div>
                    <div class="oscad-run-meta" data-oscad-run-meta></div>
                    <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.prompt', 'Agent prompt'))}</label>
                    <textarea class="oscad-chat" data-oscad-prompt rows="5" placeholder="${esc(t(ctx, 'desktop.openscad.prompt_placeholder', 'Describe the model you want...'))}"></textarea>
                    <div class="oscad-row">
                        <button type="button" class="oscad-btn oscad-primary" data-oscad-agent>${icon(ctx, 'agent-chat', 'A', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.generate_render', 'Generate & render'))}</span></button>
                        <button type="button" class="oscad-btn" data-oscad-render>${icon(ctx, 'run', 'R', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.render', 'Render'))}</span></button>
                    </div>
                    <button type="button" class="oscad-btn oscad-cancel" data-oscad-cancel hidden>${icon(ctx, 'x', 'X', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.cancel', 'Cancel'))}</span></button>
                    <div class="oscad-options">
                        <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.exports', 'Exports'))}</label>
                        <div class="oscad-chips oscad-primary-exports" data-oscad-primary-exports></div>
                        <details class="oscad-more-exports">
                            <summary>${esc(t(ctx, 'desktop.openscad.more_exports', 'More exports'))}</summary>
                            <div class="oscad-chips" data-oscad-more-exports></div>
                        </details>
                        <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.defines', 'Custom -D defines'))}</label>
                        <textarea class="oscad-defines" data-oscad-defines rows="3" spellcheck="false" placeholder="${esc(t(ctx, 'desktop.openscad.defines_placeholder', 'name=value'))}" ${ro ? 'readonly' : ''}></textarea>
                        <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.mode', 'Mode'))}</label>
                        <select class="oscad-select" data-oscad-mode ${ro ? 'disabled' : ''}>
                            <option value="render">${esc(t(ctx, 'desktop.openscad.mode_render', 'Render'))}</option>
                            <option value="preview">${esc(t(ctx, 'desktop.openscad.mode_preview', 'Preview'))}</option>
                        </select>
                        <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.timeout', 'Timeout'))}</label>
                        <input class="oscad-input" data-oscad-timeout type="number" min="10" max="600" step="10" ${ro ? 'readonly' : ''}>
                    </div>
                </aside>
                <main class="oscad-preview-zone">
                    <div class="oscad-viewport-head">
                        <div>
                            <span>${esc(t(ctx, 'desktop.openscad.tab_preview', 'Preview'))}</span>
                            <strong data-oscad-primary-label></strong>
                        </div>
                        <div class="oscad-viewport-toolbar">
                            <button type="button" class="oscad-icon-btn" data-oscad-fit title="${esc(t(ctx, 'desktop.openscad.viewport_fit', 'Fit view'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_fit', 'Fit view'))}">${icon(ctx, 'zoom-fit', 'F', 'oscad-btn-icon', 16)}</button>
                            <button type="button" class="oscad-icon-btn" data-oscad-background title="${esc(t(ctx, 'desktop.openscad.viewport_background', 'Toggle background'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_background', 'Toggle background'))}">${icon(ctx, 'contrast', 'B', 'oscad-btn-icon', 16)}</button>
                            <button type="button" class="oscad-icon-btn" data-oscad-axes title="${esc(t(ctx, 'desktop.openscad.viewport_axes', 'Toggle grid and axes'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_axes', 'Toggle grid and axes'))}">${icon(ctx, 'grid', 'G', 'oscad-btn-icon', 16)}</button>
                            <button type="button" class="oscad-icon-btn" data-oscad-fullscreen title="${esc(t(ctx, 'desktop.openscad.fullscreen', 'Fullscreen'))}" aria-label="${esc(t(ctx, 'desktop.openscad.fullscreen', 'Fullscreen'))}">${icon(ctx, 'fullscreen', 'F', 'oscad-btn-icon', 16)}</button>
                        </div>
                    </div>
                    <div class="oscad-panel" data-oscad-panel data-oscad-preview-panel></div>
                    <div class="oscad-footer">
                        <div class="oscad-status" data-oscad-status></div>
                        <div class="oscad-actions">
                            <button type="button" class="oscad-btn" data-oscad-download>${icon(ctx, 'download', 'D', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.primary_download', 'Download primary'))}</span></button>
                            <button type="button" class="oscad-btn" data-oscad-save>${icon(ctx, 'save', 'S', 'oscad-btn-icon', 16)}<span>${esc(t(ctx, 'desktop.openscad.save_all_desktop', 'Save all'))}</span></button>
                        </div>
                    </div>
                </main>
                <aside class="oscad-inspector" data-oscad-inspector>
                    <div class="oscad-tabs">
                        ${tabButton(state, 'source', t(ctx, 'desktop.openscad.tab_source', 'Source'))}
                        ${tabButton(state, 'files', t(ctx, 'desktop.openscad.tab_files', 'Files'))}
                        ${tabButton(state, 'log', t(ctx, 'desktop.openscad.tab_log', 'Log'))}
                        <button type="button" class="oscad-icon-btn" data-oscad-refresh title="${esc(t(ctx, 'desktop.openscad.refresh', 'Refresh'))}">${icon(ctx, 'refresh', 'R', 'oscad-btn-icon', 16)}</button>
                    </div>
                    <div class="oscad-inspector-panel" data-oscad-inspector-panel></div>
                </aside>
            </div>`;
        const primary = state.host.querySelector('[data-oscad-primary-exports]');
        if (primary) {
            primary.innerHTML = exportChipHTML(state, 'png') + exportChipHTML(state, 'stl');
        }
        const more = state.host.querySelector('[data-oscad-more-exports]');
        if (more) {
            more.innerHTML = ['svg', 'pdf', '3mf', 'off', 'dxf', 'csg', 'echo'].map(format => exportChipHTML(state, format)).join('');
        }
        wireShell(state);
        state.shellReady = true;
    }

    function syncShellControls(state) {
        const host = state.host;
        if (!host) return;
        const ro = isOpenSCADReadOnly(state.ctx);
        host.classList.toggle('busy', !!state.busy);
        host.classList.toggle('light-preview', !!state.lightPreview);
        host.setAttribute('aria-busy', state.busy ? 'true' : 'false');
        const meta = host.querySelector('[data-oscad-run-meta]');
        if (meta) {
            meta.innerHTML = jobMetaHTML(state) + (state.sourceDirty ? `<span class="oscad-dirty">${esc(t(state.ctx, 'desktop.openscad.render_required', 'Render required'))}</span>` : '');
        }
        const promptEl = host.querySelector('[data-oscad-prompt]');
        if (promptEl && document.activeElement !== promptEl) promptEl.value = state.prompt;
        const definesEl = host.querySelector('[data-oscad-defines]');
        if (definesEl && document.activeElement !== definesEl) definesEl.value = state.definesText || '';
        const modeEl = host.querySelector('[data-oscad-mode]');
        if (modeEl) modeEl.value = state.renderMode || 'render';
        const timeoutEl = host.querySelector('[data-oscad-timeout]');
        if (timeoutEl && document.activeElement !== timeoutEl) timeoutEl.value = String(state.timeout);
        host.querySelectorAll('[data-oscad-export]').forEach(input => {
            const format = input.dataset.oscadExport;
            input.checked = state.exports.has(format);
            input.disabled = ro;
        });
        const label = host.querySelector('[data-oscad-primary-label]');
        if (label) label.textContent = primaryFileLabel(state);
        const bgBtn = host.querySelector('[data-oscad-background]');
        if (bgBtn) bgBtn.classList.toggle('active', !!state.lightPreview);
        const axesBtn = host.querySelector('[data-oscad-axes]');
        if (axesBtn) axesBtn.classList.toggle('active', !!state.showAxes);
        host.querySelectorAll('.oscad-tab').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.oscadTab === state.activeTab);
        });
        setOpenSCADBusy(state, state.busy, state.busyMode);
    }

    function draw(state) {
        ensureShell(state);
        syncShellControls(state);
        renderPanel(state);
        setWindowMenus(state);
    }

    function wireShell(state) {
        if (state.shellWired) return;
        state.shellWired = true;
        wire(state);
        attachResultListeners(state);
    }

    function tabButton(state, id, label) {
        return `<button class="oscad-tab ${state.activeTab === id ? 'active' : ''}" data-oscad-tab="${esc(id)}">${esc(label)}</button>`;
    }

    function exportChipHTML(state, format) {
        return `<label class="oscad-chip">
            <input type="checkbox" data-oscad-export="${esc(format)}" ${state.exports.has(format) ? 'checked' : ''}>
            <span>${esc(format.toUpperCase())}</span>
        </label>`;
    }

    function jobMetaHTML(state) {
        const ctx = state.ctx;
        if (!state.result || !state.result.job_id) {
            return `<span>${esc(t(ctx, 'desktop.openscad.ready', 'Ready'))}</span>`;
        }
        const duration = Number(state.result.duration_ms || 0);
        const durationText = duration > 0 ? `${Math.max(0.1, duration / 1000).toFixed(1)}s` : '-';
        return `<span>${esc(t(ctx, 'desktop.openscad.job', 'Job'))}: ${esc(state.result.job_id)}</span><span>${esc(t(ctx, 'desktop.openscad.duration', 'Duration'))}: ${esc(durationText)}</span>`;
    }

    function primaryFileLabel(state) {
        const file = primaryFile(state);
        return file ? `${file.name} · ${String(file.format || '').toUpperCase()}` : t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.');
    }

    function wire(state) {
        const host = state.host;
        const promptEl = host.querySelector('[data-oscad-prompt]');
        if (promptEl) promptEl.addEventListener('input', e => { state.prompt = e.target.value; scheduleOpenSCADDraftSave(state); });
        const definesEl = host.querySelector('[data-oscad-defines]');
        if (definesEl) definesEl.addEventListener('input', e => { state.definesText = e.target.value; scheduleOpenSCADDraftSave(state); });
        const modeEl = host.querySelector('[data-oscad-mode]');
        if (modeEl) modeEl.addEventListener('change', e => { state.renderMode = e.target.value || 'render'; scheduleOpenSCADDraftSave(state); });
        const timeoutEl = host.querySelector('[data-oscad-timeout]');
        if (timeoutEl) timeoutEl.addEventListener('input', e => { state.timeout = Number(e.target.value || 120); scheduleOpenSCADDraftSave(state); });
        host.querySelectorAll('[data-oscad-export]').forEach(input => {
            input.addEventListener('change', () => {
                const format = input.dataset.oscadExport;
                if (input.checked) state.exports.add(format);
                else state.exports.delete(format);
                scheduleOpenSCADDraftSave(state);
            });
        });
        host.querySelector('[data-oscad-render]').addEventListener('click', () => renderSource(state));
        host.querySelector('[data-oscad-agent]').addEventListener('click', () => askAgent(state));
        host.querySelector('[data-oscad-cancel]').addEventListener('click', () => cancelCurrentOpenSCADWork(state));
        host.querySelector('[data-oscad-save]').addEventListener('click', () => saveJob(state));
        host.querySelector('[data-oscad-download]').addEventListener('click', () => downloadPrimary(state));
        host.querySelector('[data-oscad-fullscreen]').addEventListener('click', () => fullscreenPreview(state));
        host.querySelector('[data-oscad-fit]').addEventListener('click', () => resetPreviewView(state));
        host.querySelector('[data-oscad-background]').addEventListener('click', () => togglePreviewBackground(state));
        host.querySelector('[data-oscad-axes]').addEventListener('click', () => togglePreviewAxes(state));
        const refreshEl = host.querySelector('[data-oscad-refresh]');
        if (refreshEl) refreshEl.addEventListener('click', () => loadStatus(state));
        host.querySelectorAll('[data-oscad-tab]').forEach(btn => btn.addEventListener('click', () => {
            state.activeTab = btn.dataset.oscadTab;
            syncShellControls(state);
            renderInspector(state);
            scheduleOpenSCADDraftSave(state);
        }));
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
        const payload = data.payload || data.result || null;
        state.result = payload;
        if (payload && typeof payload.source_scad === 'string' && payload.source_scad.length) {
            state.source = payload.source_scad;
        }
        state.sourceDirty = false;
        state.activeTab = 'files';
        setOpenSCADBusy(state, false);
        draw(state);
        persistOpenSCADDraft(state);
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
        if (state.busy || isOpenSCADReadOnly(state.ctx)) return;
        const exports = Array.from(state.exports);
        if (!exports.length) exports.push('png', 'stl');
        state.cancelRequested = false;
        setOpenSCADBusy(state, true, 'render');
        setStatus(state, t(state.ctx, 'desktop.openscad.rendering', 'Rendering...'));
        const controller = new AbortController();
        state.renderAbort = controller;
        const timeout = window.setTimeout(() => controller.abort(), renderRequestTimeoutMS(state));
        try {
            const body = await state.ctx.api('/api/openscad/render', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                signal: controller.signal,
                body: JSON.stringify({
                    source_scad: state.source,
                    model_name: 'model',
                    exports,
                    defines: parseDefinesText(state.definesText),
                    render_mode: state.renderMode,
                    timeout_seconds: state.timeout
                })
            });
            state.result = body && body.result ? body.result : null;
            if (body && body.status === 'error') {
                state.activeTab = state.result ? 'log' : state.activeTab;
                setOpenSCADBusy(state, false);
                draw(state);
                setStatus(state, body.error || t(state.ctx, 'desktop.openscad.render_failed', 'Render failed'), true);
                return;
            }
            state.sourceDirty = false;
            state.activeTab = 'files';
            setOpenSCADBusy(state, false);
            draw(state);
            persistOpenSCADDraft(state);
            setStatus(state, state.result ? t(state.ctx, 'desktop.openscad.render_complete', 'Render complete') : t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'), !state.result);
        } catch (err) {
            setOpenSCADBusy(state, false);
            const partial = err && err.body && err.body.result ? err.body.result : null;
            if (partial) {
                state.result = partial;
                state.activeTab = 'log';
                draw(state);
            }
            const message = err && err.name === 'AbortError'
                ? (state.cancelRequested ? t(state.ctx, 'desktop.openscad.cancelled', 'Cancelled') : t(state.ctx, 'desktop.openscad.render_timeout', 'Render timed out. Try a simpler model or increase the timeout.'))
                : (err && err.message) || String(err);
            setStatus(state, message, true);
        } finally {
            window.clearTimeout(timeout);
            if (state.renderAbort === controller) state.renderAbort = null;
            state.cancelRequested = false;
        }
    }

    async function askAgent(state) {
        const message = state.prompt.trim();
        if (!message || state.busy || isOpenSCADReadOnly(state.ctx)) return;
        setOpenSCADBusy(state, true, 'agent');
        setStatus(state, t(state.ctx, 'desktop.openscad.agent_working', 'Agent is working...'));
        const controller = new AbortController();
        state.agentAbort = controller;
        try {
            const response = await fetch('/api/desktop/chat/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                signal: controller.signal,
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
            const message = err && err.name === 'AbortError'
                ? t(state.ctx, 'desktop.openscad.cancelled', 'Cancelled')
                : (err && err.message) || String(err);
            setStatus(state, message, true);
        } finally {
            if (state.agentAbort === controller) state.agentAbort = null;
            state.cancelRequested = false;
            setOpenSCADBusy(state, false);
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
        if (!state.result || !state.result.job_id || isOpenSCADReadOnly(state.ctx)) return;
        setStatus(state, t(state.ctx, 'desktop.openscad.saving', 'Saving...'));
        try {
            const body = await state.ctx.api(`/api/openscad/jobs/${encodeURIComponent(state.result.job_id)}/save`, { method: 'POST' });
            state.result = body.result;
            state.activeTab = 'files';
            draw(state);
            setStatus(state, t(state.ctx, 'desktop.openscad.saved', 'Saved to Desktop'));
        } catch (err) {
            setStatus(state, err.message || String(err), true);
        }
    }

    function downloadPrimary(state) {
        const file = primaryFile(state);
        if (!file) return;
        downloadFile(file);
    }

    function fullscreenPreview(state) {
        const panel = state.host.querySelector('[data-oscad-preview-panel]');
        if (panel && panel.requestFullscreen) panel.requestFullscreen();
    }

    function renderPanel(state) {
        renderViewport(state);
        renderInspector(state);
    }

    function renderInspector(state) {
        const panel = state.host.querySelector('[data-oscad-inspector-panel]');
        if (!panel) return;
        if (state.activeTab === 'source') {
            const ro = isOpenSCADReadOnly(state.ctx);
            panel.innerHTML = `<textarea class="oscad-source" data-oscad-source spellcheck="false" inputmode="text" ${ro ? 'readonly' : ''}>${esc(state.source)}</textarea>`;
            const sourceEl = panel.querySelector('[data-oscad-source]');
            if (sourceEl && !sourceEl.dataset.oscadBound) {
                sourceEl.dataset.oscadBound = '1';
                sourceEl.addEventListener('input', e => {
                    if (isOpenSCADReadOnly(state.ctx)) return;
                    state.source = e.target.value;
                    state.sourceDirty = true;
                    updateWindowContext(state);
                    setStatus(state, t(state.ctx, 'desktop.openscad.render_required', 'Render required'));
                    const meta = state.host.querySelector('[data-oscad-run-meta]');
                    if (meta) {
                        meta.innerHTML = jobMetaHTML(state) + `<span class="oscad-dirty">${esc(t(state.ctx, 'desktop.openscad.render_required', 'Render required'))}</span>`;
                    }
                    setWindowMenus(state);
                    scheduleOpenSCADDraftSave(state);
                });
            }
            return;
        }
        if (state.activeTab === 'files') {
            const files = resultFiles(state);
            panel.innerHTML = files.length ? `<div class="oscad-file-list">${files.map(file => fileRowHTML(state, file)).join('')}</div>` : emptyPanel(state, 'desktop.openscad.no_files', 'No files yet');
            panel.querySelectorAll('[data-oscad-file-download]').forEach(btn => btn.addEventListener('click', () => {
                const file = resultFiles(state).find(item => item.name === btn.dataset.oscadFileDownload);
                if (file) downloadFile(file);
            }));
            panel.querySelectorAll('[data-oscad-open-saved]').forEach(btn => btn.addEventListener('click', () => openSavedPath(state, btn.dataset.oscadOpenSaved)));
            return;
        }
        if (state.activeTab === 'log') {
            const log = state.result ? [state.result.stdout, state.result.stderr].filter(Boolean).join('\n') : '';
            panel.innerHTML = log ? `<pre class="oscad-code">${esc(log)}</pre>` : emptyPanel(state, 'desktop.openscad.no_log', 'No log yet');
            return;
        }
        state.activeTab = 'source';
        renderInspector(state);
    }

    function renderViewport(state) {
        const panel = state.host.querySelector('[data-oscad-preview-panel]');
        if (!panel) return;
        renderPreview(state, panel);
    }

    function resultFiles(state) {
        return state.result && Array.isArray(state.result.files) ? state.result.files : [];
    }

    function fileRowHTML(state, file) {
        const ctx = state.ctx;
        const savedPath = file.saved_path || file.SavedPath || '';
        return `<article class="oscad-file">
            <div class="oscad-file-main">
                <span title="${esc(file.name)}">${esc(file.name)}</span>
                <small>${esc(String(file.format || '').toUpperCase())} · ${esc(formatSize(file.size))}</small>
                ${savedPath ? `<em title="${esc(savedPath)}">${esc(t(ctx, 'desktop.openscad.saved_path', 'Saved'))}: ${esc(savedPath)}</em>` : ''}
            </div>
            <div class="oscad-file-actions">
                <button type="button" class="oscad-icon-btn" data-oscad-file-download="${esc(file.name)}" title="${esc(t(ctx, 'desktop.openscad.file_download', 'Download file'))}">${icon(ctx, 'download', 'D', 'oscad-btn-icon', 16)}</button>
                ${savedPath ? `<button type="button" class="oscad-icon-btn" data-oscad-open-saved="${esc(savedPath)}" title="${esc(t(ctx, 'desktop.openscad.open_saved', 'Open saved file'))}">${icon(ctx, 'folder-open', 'O', 'oscad-btn-icon', 16)}</button>` : ''}
            </div>
        </article>`;
    }

    function renderPreview(state, panel) {
        const file = primaryFile(state);
        if (!file) {
            state.previewStlURL = '';
            panel.innerHTML = emptyPanel(state, 'desktop.openscad.no_preview', 'Render a model to see the preview.');
            return;
        }
        const url = previewURL(file);
        if (!url) {
            panel.innerHTML = emptyPanel(state, 'desktop.openscad.no_preview', 'Render a model to see the preview.');
            return;
        }
        if (file.format === 'png') {
            state.previewStlURL = '';
            panel.innerHTML = `<img class="oscad-preview-img" data-oscad-preview-img src="${esc(url)}" alt="">`;
            bindPreviewLoadError(state, panel, panel.querySelector('[data-oscad-preview-img]'));
            return;
        }
        if (file.format === 'svg') {
            state.previewStlURL = '';
            panel.innerHTML = `<object class="oscad-preview-object" data-oscad-preview-object data="${esc(url)}" type="image/svg+xml"></object>`;
            bindPreviewLoadError(state, panel, panel.querySelector('[data-oscad-preview-object]'));
            return;
        }
        if (file.format === 'pdf') {
            state.previewStlURL = '';
            panel.innerHTML = `<iframe class="oscad-preview-object" data-oscad-preview-frame src="${esc(url)}"></iframe>`;
            bindPreviewLoadError(state, panel, panel.querySelector('[data-oscad-preview-frame]'));
            return;
        }
        if (file.format === 'stl') {
            const mount = panel.querySelector('[data-stl-viewer]');
            if (state.preview3D && state.previewStlURL === url && mount && mount.querySelector('canvas')) {
                return;
            }
            state.previewStlURL = url;
            panel.innerHTML = `<div class="oscad-stl" data-stl-viewer></div>`;
            renderSTL(state, panel.querySelector('[data-stl-viewer]'), url);
            return;
        }
        state.previewStlURL = '';
        panel.innerHTML = `<div class="oscad-empty"><strong>${esc(file.name)}</strong><span>${esc(t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'))}</span></div>`;
    }

    function bindPreviewLoadError(state, panel, element) {
        if (!element) return;
        element.addEventListener('error', () => {
            panel.innerHTML = `<div class="oscad-empty"><strong>${esc(t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'))}</strong><span>${esc(t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'))}</span></div>`;
            setStatus(state, t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'), true);
        }, { once: true });
    }

    function setOpenSCADBusy(state, busy, mode) {
        state.busy = !!busy;
        state.busyMode = state.busy ? (mode || state.busyMode || 'work') : '';
        if (!state.host) return;
        const ro = isOpenSCADReadOnly(state.ctx);
        state.host.classList.toggle('busy', state.busy);
        state.host.classList.toggle('busy-render', state.busy && state.busyMode === 'render');
        state.host.classList.toggle('busy-agent', state.busy && state.busyMode === 'agent');
        state.host.setAttribute('aria-busy', state.busy ? 'true' : 'false');
        const statusEl = state.host.querySelector('[data-oscad-status]');
        if (statusEl) {
            statusEl.textContent = state.statusMessage || t(state.ctx, 'desktop.openscad.ready', 'Ready');
            statusEl.classList.toggle('error', !!state.statusError);
        }
        state.host.querySelectorAll('[data-oscad-render], [data-oscad-agent], [data-oscad-save]').forEach(btn => {
            btn.disabled = ro || state.busy;
        });
        state.host.querySelectorAll('[data-oscad-refresh], [data-oscad-download]').forEach(btn => {
            btn.disabled = state.busy || (btn.hasAttribute('data-oscad-download') && !state.result);
        });
        const cancel = state.host.querySelector('[data-oscad-cancel]');
        if (cancel) cancel.hidden = !state.busy;
        setWindowMenus(state);
    }

    function renderRequestTimeoutMS(state) {
        const seconds = Math.max(10, Math.min(Number(state.timeout) || 120, 600));
        return (seconds + 45) * 1000;
    }

    function cancelCurrentOpenSCADWork(state) {
        state.cancelRequested = true;
        if (state.renderAbort) state.renderAbort.abort();
        if (state.agentAbort) state.agentAbort.abort();
        setOpenSCADBusy(state, false);
        setStatus(state, t(state.ctx, 'desktop.openscad.cancelled', 'Cancelled'), true);
    }

    function resetPreviewView(state) {
        const p3d = state.preview3D;
        if (p3d && p3d.mesh && p3d.camera && window.THREE) {
            const box = new THREE.Box3().setFromObject(p3d.mesh);
            const size = box.getSize(new THREE.Vector3()).length() || 80;
            p3d.camera.position.set(size, size * 0.75, size);
            p3d.camera.lookAt(0, 0, 0);
            if (p3d.controls && p3d.controls.target) p3d.controls.target.set(0, 0, 0);
            if (p3d.controls && typeof p3d.controls.update === 'function') p3d.controls.update();
            return;
        }
        renderViewport(state);
    }

    function togglePreviewBackground(state) {
        state.lightPreview = !state.lightPreview;
        const p3d = state.preview3D;
        if (p3d && p3d.scene && window.THREE) {
            p3d.scene.background = new THREE.Color(state.lightPreview ? 0xf2f6f8 : 0x071018);
        }
        syncShellControls(state);
        if (!p3d) renderViewport(state);
        scheduleOpenSCADDraftSave(state);
    }

    function togglePreviewAxes(state) {
        state.showAxes = !state.showAxes;
        renderViewport(state);
        syncShellControls(state);
        scheduleOpenSCADDraftSave(state);
    }

    function downloadFile(file) {
        if (!file || !file.download_url) return;
        window.open(file.download_url, '_blank', 'noopener');
    }

    function openSavedPath(state, savedPath) {
        const normalized = String(savedPath || '').replace(/\\/g, '/').replace(/^\/+/, '');
        if (!normalized || !state.ctx || typeof state.ctx.openApp !== 'function') return;
        const dir = normalized.split('/').slice(0, -1).join('/') || normalized;
        state.ctx.openApp('files', { path: dir });
    }

    function setWindowMenus(state) {
        if (!state.ctx || typeof state.ctx.setWindowMenus !== 'function') return;
        const ro = isOpenSCADReadOnly(state.ctx);
        state.ctx.setWindowMenus(state.windowId, [
            {
                id: 'model',
                labelKey: 'desktop.openscad.title',
                items: [
                    { id: 'render', labelKey: 'desktop.openscad.render', icon: 'run', disabled: ro || state.busy, action: () => renderSource(state) },
                    { id: 'cancel', labelKey: 'desktop.openscad.cancel', icon: 'x', disabled: !state.busy, action: () => cancelCurrentOpenSCADWork(state) },
                    { type: 'separator' },
                    { id: 'download', labelKey: 'desktop.openscad.primary_download', icon: 'download', disabled: !state.result || state.busy, action: () => downloadPrimary(state) },
                    { id: 'save', labelKey: 'desktop.openscad.save_all_desktop', icon: 'save', disabled: ro || !state.result || state.busy, action: () => saveJob(state) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'fit', labelKey: 'desktop.openscad.viewport_fit', icon: 'zoom-fit', action: () => resetPreviewView(state) },
                    { id: 'background', labelKey: 'desktop.openscad.viewport_background', icon: 'contrast', checked: state.lightPreview, action: () => togglePreviewBackground(state) },
                    { id: 'axes', labelKey: 'desktop.openscad.viewport_axes', icon: 'grid', checked: state.showAxes, action: () => togglePreviewAxes(state) },
                    { id: 'fullscreen', labelKey: 'desktop.openscad.fullscreen', icon: 'fullscreen', action: () => fullscreenPreview(state) }
                ]
            }
        ]);
    }

    function clearWindowMenus(state) {
        if (state && state.ctx && typeof state.ctx.clearWindowMenus === 'function') {
            state.ctx.clearWindowMenus(state.windowId);
        }
    }

    function renderSTL(state, mount, url) {
        cleanupPreview(state);
        if (!mount) return;
        if (!window.THREE || !THREE.STLLoader) {
            mount.innerHTML = `<div class="oscad-empty"><strong>${esc(t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'))}</strong><span>${esc(t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'))}</span></div>`;
            setStatus(state, t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'), true);
            return;
        }
        const STLLoader = THREE.STLLoader;
        const OrbitControls = THREE.OrbitControls;
        const width = mount.clientWidth || 640;
        const height = mount.clientHeight || 420;
        const scene = new THREE.Scene();
        scene.background = new THREE.Color(state.lightPreview ? 0xf2f6f8 : 0x071018);
        const camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 5000);
        camera.position.set(80, 70, 90);
        const renderer = new THREE.WebGLRenderer({ antialias: true });
        renderer.setSize(width, height);
        mount.innerHTML = '';
        mount.appendChild(renderer.domElement);
        scene.add(new THREE.HemisphereLight(0xffffff, state.lightPreview ? 0xd8e1ea : 0x243447, 1.2));
        const light = new THREE.DirectionalLight(0xffffff, 0.9);
        light.position.set(40, 80, 60);
        scene.add(light);
        const helpers = [];
        if (state.showAxes) {
            const grid = new THREE.GridHelper(160, 16, 0x42d7c8, state.lightPreview ? 0xc4d1db : 0x1a3441);
            const axes = new THREE.AxesHelper(70);
            helpers.push(grid, axes);
            scene.add(grid);
            scene.add(axes);
        }
        const controls = OrbitControls ? new OrbitControls(camera, renderer.domElement) : null;
        let disposed = false;
        let frameID = 0;
        let mesh = null;
        state.previewCleanup = () => {
            disposed = true;
            if (frameID) cancelAnimationFrame(frameID);
            if (controls && typeof controls.dispose === 'function') controls.dispose();
            if (mesh) {
                if (mesh.geometry && typeof mesh.geometry.dispose === 'function') mesh.geometry.dispose();
                if (mesh.material && typeof mesh.material.dispose === 'function') mesh.material.dispose();
                scene.remove(mesh);
            }
            helpers.forEach(helper => scene.remove(helper));
            if (renderer && typeof renderer.dispose === 'function') renderer.dispose();
            if (renderer && typeof renderer.forceContextLoss === 'function') renderer.forceContextLoss();
            state.preview3D = null;
            state.previewCleanup = null;
        };
        new STLLoader().load(url, geometry => {
            if (disposed) {
                if (geometry && typeof geometry.dispose === 'function') geometry.dispose();
                return;
            }
            geometry.computeBoundingBox();
            geometry.center();
            // OpenSCAD/STL is Z-up; Three.js preview uses Y-up (same as other desktop CAD viewers).
            mesh = new THREE.Mesh(geometry, new THREE.MeshStandardMaterial({ color: 0x42d7c8, roughness: 0.42, metalness: 0.12 }));
            mesh.rotation.x = -Math.PI / 2;
            scene.add(mesh);
            const box = new THREE.Box3().setFromObject(mesh);
            const size = box.getSize(new THREE.Vector3()).length() || 80;
            camera.position.set(size, size * 0.75, size);
            camera.lookAt(0, 0, 0);
            if (controls && controls.target) controls.target.set(0, 0, 0);
            state.preview3D = { scene, camera, controls, mesh, renderer, helpers, animate };
            animate();
        }, undefined, err => {
            mount.innerHTML = `<div class="oscad-empty"><strong>${esc(t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'))}</strong><span>${esc(err && err.message ? err.message : String(err))}</span></div>`;
            setStatus(state, err && err.message ? err.message : String(err), true);
        });
        function animate() {
            if (!stateByWindow.has(state.windowId)) return;
            frameID = requestAnimationFrame(animate);
            if (controls) controls.update();
            renderer.render(scene, camera);
        }
    }

    function cleanupPreview(state) {
        if (state && typeof state.previewCleanup === 'function') {
            state.previewCleanup();
        }
    }

    function previewURL(file) {
        return (file && (file.preview_url || file.download_url)) || '';
    }

    function primaryFile(state) {
        const files = resultFiles(state);
        return files.find(file => file.format === 'png') || files.find(file => file.format === 'stl') || files[0] || null;
    }

    function emptyPanel(state, key, fallback) {
        return `<div class="oscad-empty">${esc(t(state.ctx, key, fallback))}</div>`;
    }

    function setStatus(state, message, error) {
        state.statusMessage = message || '';
        state.statusError = !!error;
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
        persistOpenSCADDraft(state);
        if (state.renderAbort) {
            state.renderAbort.abort();
            state.renderAbort = null;
        }
        if (state.agentAbort) {
            state.agentAbort.abort();
            state.agentAbort = null;
        }
        clearWindowMenus(state);
        cleanupPreview(state);
        state.listeners.forEach(fn => {
            try { fn(); } catch (_) {}
        });
        stateByWindow.delete(windowId);
    }

    window.OpenSCADApp = { render, dispose };
})();
