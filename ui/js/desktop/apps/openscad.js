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
    const PANEL_MIN_WIDTH = 232;
    const PANEL_MAX_WIDTH = 560;

    const stateByWindow = new Map();

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    function t(ctx, key, fallback) {
        return ctx && typeof ctx.t === 'function' ? ctx.t(key, fallback) : fallback;
    }

    function openSCADNowMS() { return window.performance && typeof window.performance.now === 'function' ? window.performance.now() : Date.now(); }
    function openSCADResultSummary(result) { const files = result && Array.isArray(result.files) ? result.files : []; return result ? { job_id: result.job_id || '', duration_ms: Number(result.duration_ms || 0), exit_code: Number(result.exit_code || 0), files: files.map(file => ({ name: file.name, format: file.format, size: file.size })) } : null; }
    function logOpenSCAD(state, message, detail) { if (window.console && typeof console.info === 'function') console.info('[OpenSCAD]', message, Object.assign({ window_id: state && state.windowId }, detail || {})); }
    function warnOpenSCAD(state, message, detail) { if (window.console && typeof console.warn === 'function') console.warn('[OpenSCAD]', message, Object.assign({ window_id: state && state.windowId }, detail || {})); }

    function icon(ctx, name, fallback, className, size) {
        return ctx && typeof ctx.iconMarkup === 'function' ? ctx.iconMarkup(name, fallback, className, size) : `<span class="${esc(className || '')}">${esc(fallback || '')}</span>`;
    }

    function isOpenSCADReadOnly(ctx) {
        return !!(ctx && ctx.readonly);
    }

    function openSCADDraftStorageKey(windowId) {
        return OPENSCAD_DRAFT_KEY + '.' + String(windowId || 'default');
    }

    function parseOpenSCADErrors(stderr) {
        return window.OpenSCADEditor && typeof window.OpenSCADEditor.parse === 'function'
            ? window.OpenSCADEditor.parse(stderr) : [];
    }

    function parseDefinesText(text) {
        if (window.OpenSCADDefines && typeof window.OpenSCADDefines.parse === 'function') {
            return window.OpenSCADDefines.parse(text);
        }
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

    function mountDefinesPanel(state, force) {
        if (!window.OpenSCADDefines || typeof window.OpenSCADDefines.render !== 'function') return;
        const mountEl = state.host.querySelector('[data-oscad-defines]');
        if (!mountEl) return;
        const signature = [
            isOpenSCADReadOnly(state.ctx) ? '1' : '0',
            state.definesMode === 'text' ? 'text' : 'sliders',
            String(state.definesText || '')
        ].join('|');
        if (!force && state.definesMounted && state.definesRenderSignature === signature) return;
        const active = document.activeElement;
        const keepFocus = !!(active && mountEl.contains(active) && active.getAttribute && (
            active.hasAttribute('data-oscad-slider') ||
            active.hasAttribute('data-oscad-number') ||
            active.hasAttribute('data-oscad-text') ||
            active.hasAttribute('data-oscad-defines-text')
        ));
        if (keepFocus && !force) return;
        try {
            const defines = parseDefinesText(state.definesText || '');
            window.OpenSCADDefines.render(mountEl, defines, function(text) {
                if (isOpenSCADReadOnly(state.ctx)) return;
                state.definesText = text;
                state.definesRenderSignature = [
                    isOpenSCADReadOnly(state.ctx) ? '1' : '0',
                    state.definesMode === 'text' ? 'text' : 'sliders',
                    String(state.definesText || '')
                ].join('|');
                scheduleOpenSCADDraftSave(state);
                updateWindowContext(state);
            }, {
                ctx: state.ctx,
                readonly: isOpenSCADReadOnly(state.ctx),
                mode: state.definesMode === 'text' ? 'text' : 'sliders',
                onModeChange: function(mode) {
                    if (isOpenSCADReadOnly(state.ctx)) return;
                    state.definesMode = mode === 'text' ? 'text' : 'sliders';
                    state.definesRenderSignature = '';
                    mountDefinesPanel(state, true);
                    scheduleOpenSCADDraftSave(state);
                }
            });
            state.definesMounted = true;
            state.definesRenderSignature = signature;
        } catch (err) {
            warnOpenSCAD(state, 'defines panel failed', { error: err && err.message ? err.message : String(err) });
        }
    }

    function wireDefinesPanel(state) {
        mountDefinesPanel(state);
    }

    function readOpenSCADDraft(windowId) {
        try {
            const keys = [openSCADDraftStorageKey(windowId), OPENSCAD_DRAFT_KEY];
            for (const key of keys) {
                const raw = localStorage.getItem(key);
                if (!raw) continue;
                const data = JSON.parse(raw);
                if (!data || typeof data !== 'object' || data.v !== OPENSCAD_DRAFT_VERSION) continue;
                return data;
            }
            return null;
        } catch (_) {
            return null;
        }
    }

    function clampPanelWidth(value) {
        const px = Math.round(Number(value) || 0);
        if (!px) return 0;
        return Math.min(PANEL_MAX_WIDTH, Math.max(PANEL_MIN_WIDTH, px));
    }

    function openSCADDraftFromState(state) {
        const source = String(state.source || '');
        if (source.length > OPENSCAD_DRAFT_MAX_SOURCE) return null;
        return {
            v: OPENSCAD_DRAFT_VERSION,
            source,
            definesText: String(state.definesText || ''),
            definesMode: state.definesMode === 'text' ? 'text' : 'sliders',
            prompt: String(state.prompt || ''),
            renderMode: state.renderMode === 'preview' ? 'preview' : 'render',
            timeout: Math.min(600, Math.max(10, Number(state.timeout) || 120)),
            exports: Array.from(state.exports || []),
            activeTab: ['source', 'files', 'log'].includes(state.activeTab) ? state.activeTab : 'source',
            lightPreview: !!state.lightPreview,
            showAxes: state.showAxes !== false,
            viewportProjection: state.viewportProjection === 'orthographic' ? 'orthographic' : 'perspective',
            viewportShading: state.viewportShading === 'wireframe' ? 'wireframe' : 'shaded',
            viewportAutoRotate: state.viewportAutoRotate !== false,
            paramsCollapsed: !!state.paramsCollapsed,
            agentCollapsed: state.agentCollapsed !== false,
            inspectorCollapsed: !!state.inspectorCollapsed,
            inspectorWidth: clampPanelWidth(state.inspectorWidth),
            sidebarWidth: clampPanelWidth(state.sidebarWidth),
            savedAt: Date.now()
        };
    }

    function persistOpenSCADDraft(state) {
        if (!state || isOpenSCADReadOnly(state.ctx)) return;
        const payload = openSCADDraftFromState(state);
        if (!payload) return;
        try {
            localStorage.setItem(openSCADDraftStorageKey(state.windowId), JSON.stringify(payload));
        } catch (_) {}
    }

    function scheduleOpenSCADDraftSave(state) {
        if (!state || isOpenSCADReadOnly(state.ctx)) return;
        if (state.draftSaveTimer) window.clearTimeout(state.draftSaveTimer);
        state.draftSaveTimer = window.setTimeout(() => {
            state.draftSaveTimer = null;
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
        if (draft.definesMode === 'text' || draft.definesMode === 'sliders') state.definesMode = draft.definesMode;
        if (typeof draft.prompt === 'string') state.prompt = draft.prompt;
        if (draft.renderMode === 'preview' || draft.renderMode === 'render') state.renderMode = draft.renderMode;
        if (Number.isFinite(Number(draft.timeout))) state.timeout = Math.min(600, Math.max(10, Number(draft.timeout)));
        if (Array.isArray(draft.exports) && draft.exports.length) {
            state.exports = new Set(draft.exports.map(f => String(f).toLowerCase()).filter(Boolean));
        }
        if (['source', 'files', 'log'].includes(draft.activeTab)) state.activeTab = draft.activeTab;
        if (typeof draft.lightPreview === 'boolean') state.lightPreview = draft.lightPreview;
        if (typeof draft.showAxes === 'boolean') state.showAxes = draft.showAxes;
        if (draft.viewportProjection === 'orthographic' || draft.viewportProjection === 'perspective') state.viewportProjection = draft.viewportProjection;
        if (draft.viewportShading === 'wireframe' || draft.viewportShading === 'shaded') state.viewportShading = draft.viewportShading;
        if (typeof draft.viewportAutoRotate === 'boolean') state.viewportAutoRotate = draft.viewportAutoRotate;
        if (typeof draft.paramsCollapsed === 'boolean') state.paramsCollapsed = draft.paramsCollapsed;
        if (typeof draft.agentCollapsed === 'boolean') state.agentCollapsed = draft.agentCollapsed;
        if (typeof draft.inspectorCollapsed === 'boolean') state.inspectorCollapsed = draft.inspectorCollapsed;
        const inspectorWidth = clampPanelWidth(draft.inspectorWidth);
        if (inspectorWidth) state.inspectorWidth = inspectorWidth;
        const sidebarWidth = clampPanelWidth(draft.sidebarWidth);
        if (sidebarWidth) state.sidebarWidth = sidebarWidth;
    }

    function mergeOpenSCADLaunchContext(ctx, draft) {
        const merged = Object.assign({}, ctx || {});
        if (draft && typeof draft.source === 'string' && draft.source.length) {
            if (!merged.source) merged.source = draft.source;
        }
        return merged;
    }

    function render(host, windowId, ctx) {
        const launchCtx = ctx || {};
        const draft = readOpenSCADDraft(windowId);
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
            definesMode: 'sliders',
            activeTab: 'source',
            result: null,
            sourceDirty: false,
            lightPreview: false,
            showAxes: true,
            viewportProjection: 'perspective',
            viewportShading: 'shaded',
            viewportAutoRotate: true,
            paramsCollapsed: false,
            agentCollapsed: true,
            inspectorCollapsed: false,
            inspectorWidth: 0,
            sidebarWidth: 0,
            busyStartedAt: 0,
            busyTicker: null,
            pendingAgentSource: null,
            pendingAgentSummary: '',
            agentHistory: [],
            lastErrors: [],
            busy: false,
            busyMode: '',
            cancelRequested: false,
            shellReady: false,
            preview3D: null,
            stl: null,
            previewStlURL: '',
            previewCleanup: null,
            renderAbort: null,
            exportAbort: null,
            agentAbort: null,
            renderSerial: 0,
            statusMessage: '',
            statusError: false,
            listeners: [],
            eventsAttached: false,
            editor: null,
            definesMounted: false,
            sourceEditorReady: false,
            draftSaveTimer: null,
            definesRenderSignature: '',
            keyHandler: null
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
                <header class="oscad-header">
                    <div class="oscad-brand">
                        <div class="oscad-brand-icon">${icon(ctx, 'openscad', 'O', 'oscad-icon', 20)}</div>
                        <div class="oscad-brand-text">
                            <h2>${esc(t(ctx, 'desktop.openscad.title', 'OpenSCAD'))}</h2>
                            <p>${esc(t(ctx, 'desktop.openscad.subtitle', 'Parametric CAD compiler'))}</p>
                        </div>
                    </div>
                    <div class="oscad-run-meta" data-oscad-run-meta></div>
                    <div class="oscad-header-actions">
                        <button type="button" class="oscad-btn oscad-primary" data-oscad-render title="${esc(t(ctx, 'desktop.openscad.render', 'Render'))} (Ctrl+Enter)">${icon(ctx, 'run', 'R', 'oscad-btn-icon', 15)}<span>${esc(t(ctx, 'desktop.openscad.render', 'Render'))}</span></button>
                        <button type="button" class="oscad-btn oscad-agent-btn" data-oscad-agent>${icon(ctx, 'agent-chat', 'A', 'oscad-btn-icon', 15)}<span>${esc(t(ctx, 'desktop.openscad.generate_render', 'Generate & render'))}</span></button>
                        <button type="button" class="oscad-btn oscad-cancel" data-oscad-cancel hidden>${icon(ctx, 'stop', 'X', 'oscad-btn-icon', 15)}<span>${esc(t(ctx, 'desktop.openscad.cancel', 'Cancel'))}</span></button>
                        <span class="oscad-header-sep" aria-hidden="true"></span>
                        <button type="button" class="oscad-icon-btn" data-oscad-download title="${esc(t(ctx, 'desktop.openscad.primary_download', 'Download primary'))}" aria-label="${esc(t(ctx, 'desktop.openscad.primary_download', 'Download primary'))}">${icon(ctx, 'download', 'D', 'oscad-btn-icon', 16)}</button>
                        <button type="button" class="oscad-icon-btn" data-oscad-save title="${esc(t(ctx, 'desktop.openscad.save_all_desktop', 'Save all'))}" aria-label="${esc(t(ctx, 'desktop.openscad.save_all_desktop', 'Save all'))}">${icon(ctx, 'save', 'S', 'oscad-btn-icon', 16)}</button>
                        <span class="oscad-header-sep" aria-hidden="true"></span>
                        <button type="button" class="oscad-icon-btn" data-oscad-toggle-editor title="${esc(t(ctx, 'desktop.openscad.toggle_editor', 'Toggle editor panel'))}" aria-label="${esc(t(ctx, 'desktop.openscad.toggle_editor', 'Toggle editor panel'))}">${icon(ctx, 'code', 'E', 'oscad-btn-icon', 16)}</button>
                        <button type="button" class="oscad-icon-btn" data-oscad-toggle-params title="${esc(t(ctx, 'desktop.openscad.collapsed_parameters', 'Toggle parameters panel'))}" aria-label="${esc(t(ctx, 'desktop.openscad.collapsed_parameters', 'Toggle parameters panel'))}">${icon(ctx, 'sliders', 'P', 'oscad-btn-icon', 16)}</button>
                        <button type="button" class="oscad-icon-btn" data-oscad-toggle-agent title="${esc(t(ctx, 'desktop.openscad.collapsed_agent', 'Toggle agent panel'))}" aria-label="${esc(t(ctx, 'desktop.openscad.collapsed_agent', 'Toggle agent panel'))}">${icon(ctx, 'agent-chat', 'A', 'oscad-btn-icon', 16)}</button>
                    </div>
                </header>
                <div class="oscad-main">
                    <aside class="oscad-inspector" data-oscad-inspector>
                        <div class="oscad-tabs">
                            ${tabButton(state, 'source', t(ctx, 'desktop.openscad.tab_source', 'Source'))}
                            ${tabButton(state, 'files', t(ctx, 'desktop.openscad.tab_files', 'Files'))}
                            ${tabButton(state, 'log', t(ctx, 'desktop.openscad.tab_log', 'Log'))}
                            <button type="button" class="oscad-icon-btn" data-oscad-refresh title="${esc(t(ctx, 'desktop.openscad.refresh', 'Refresh'))}" aria-label="${esc(t(ctx, 'desktop.openscad.refresh', 'Refresh'))}">${icon(ctx, 'refresh', 'R', 'oscad-btn-icon', 14)}</button>
                            <button type="button" class="oscad-icon-btn oscad-collapse-btn" data-oscad-toggle-editor title="${esc(t(ctx, 'desktop.openscad.toggle_editor', 'Toggle editor panel'))}" aria-label="${esc(t(ctx, 'desktop.openscad.toggle_editor', 'Toggle editor panel'))}">${icon(ctx, 'chevron-left', '<', 'oscad-btn-icon', 14)}</button>
                        </div>
                        <div class="oscad-inspector-panel" data-oscad-inspector-panel></div>
                        <div class="oscad-issues" data-oscad-issues hidden></div>
                    </aside>
                    <div class="oscad-splitter" data-oscad-splitter="inspector" role="separator" aria-orientation="vertical" tabindex="0"></div>
                    <main class="oscad-preview-zone" data-oscad-preview-zone>
                        <div class="oscad-panel" data-oscad-panel data-oscad-preview-panel></div>
                        <div class="oscad-viewport-head">
                            <div class="oscad-viewport-title">
                                <span>${esc(t(ctx, 'desktop.openscad.tab_preview', 'Preview'))}</span>
                                <strong data-oscad-primary-label></strong>
                            </div>
                            <div class="oscad-viewport-toolbar">
                                <button type="button" class="oscad-icon-btn" data-oscad-zoom-out title="${esc(t(ctx, 'desktop.openscad.viewport_zoom_out', 'Zoom out'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_zoom_out', 'Zoom out'))}">${icon(ctx, 'zoom-out', '-', 'oscad-btn-icon', 15)}</button>
                                <button type="button" class="oscad-icon-btn" data-oscad-zoom-in title="${esc(t(ctx, 'desktop.openscad.viewport_zoom_in', 'Zoom in'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_zoom_in', 'Zoom in'))}">${icon(ctx, 'zoom-in', '+', 'oscad-btn-icon', 15)}</button>
                                <span class="oscad-toolbar-sep" aria-hidden="true"></span>
                                <button type="button" class="oscad-icon-btn" data-oscad-projection title="${esc(t(ctx, 'desktop.openscad.viewport_ortho', 'Toggle perspective / orthographic'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_ortho', 'Toggle perspective / orthographic'))}">${icon(ctx, 'cube', 'O', 'oscad-btn-icon', 15)}</button>
                                <button type="button" class="oscad-icon-btn" data-oscad-shading title="${esc(t(ctx, 'desktop.openscad.viewport_wireframe', 'Wireframe'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_shaded', 'Shaded surface'))}">${icon(ctx, 'mesh', 'W', 'oscad-btn-icon', 15)}</button>
                                <button type="button" class="oscad-icon-btn" data-oscad-autorotate title="${esc(t(ctx, 'desktop.openscad.viewport_auto_rotate', 'Toggle auto-rotate'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_auto_rotate', 'Toggle auto-rotate'))}">${icon(ctx, 'redo', 'R', 'oscad-btn-icon', 15)}</button>
                                <button type="button" class="oscad-icon-btn" data-oscad-background title="${esc(t(ctx, 'desktop.openscad.viewport_background', 'Toggle background'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_background', 'Toggle background'))}">${icon(ctx, 'contrast', 'B', 'oscad-btn-icon', 15)}</button>
                                <button type="button" class="oscad-icon-btn" data-oscad-axes title="${esc(t(ctx, 'desktop.openscad.viewport_axes', 'Toggle grid and axes'))}" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_axes', 'Toggle grid and axes'))}">${icon(ctx, 'grid', 'G', 'oscad-btn-icon', 15)}</button>
                                <span class="oscad-toolbar-sep" aria-hidden="true"></span>
                                <button type="button" class="oscad-icon-btn" data-oscad-fit title="${esc(t(ctx, 'desktop.openscad.viewport_fit', 'Fit view'))} (F)" aria-label="${esc(t(ctx, 'desktop.openscad.viewport_fit', 'Fit view'))}">${icon(ctx, 'zoom-reset', 'F', 'oscad-btn-icon', 15)}</button>
                                <button type="button" class="oscad-icon-btn" data-oscad-fullscreen title="${esc(t(ctx, 'desktop.openscad.fullscreen', 'Fullscreen'))}" aria-label="${esc(t(ctx, 'desktop.openscad.fullscreen', 'Fullscreen'))}">${icon(ctx, 'maximize', 'F', 'oscad-btn-icon', 15)}</button>
                            </div>
                        </div>
                        <div class="oscad-footer">
                            <div class="oscad-status" data-oscad-status role="status"></div>
                        </div>
                        <div class="oscad-busy-overlay" data-oscad-busy-label hidden><div class="oscad-busy-spinner"></div><span class="oscad-busy-text"></span><span class="oscad-busy-elapsed" data-oscad-busy-elapsed></span></div>
                    </main>
                    <div class="oscad-splitter" data-oscad-splitter="sidebar" role="separator" aria-orientation="vertical" tabindex="0"></div>
                    <aside class="oscad-sidebar" data-oscad-sidebar>
                        <div class="oscad-options">
                            <section class="oscad-section">
                                <div class="oscad-section-head">
                                    <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.exports', 'Exports'))}</label>
                                    <span class="oscad-export-count" data-oscad-export-count>0</span>
                                </div>
                                <div class="oscad-chips oscad-primary-exports" data-oscad-primary-exports></div>
                                <div class="oscad-export-hint"><span>${esc(t(ctx, 'desktop.openscad.export_2d', '2D'))}</span><span>${esc(t(ctx, 'desktop.openscad.export_3d', '3D'))}</span></div>
                                <details class="oscad-more-exports">
                                    <summary>${esc(t(ctx, 'desktop.openscad.more_exports', 'More exports'))}</summary>
                                    <div class="oscad-chips" data-oscad-more-exports></div>
                                </details>
                            </section>
                            <section class="oscad-section">
                                <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.mode', 'Mode'))}</label>
                                <div class="oscad-segmented" role="group">
                                    <button type="button" class="oscad-seg" data-oscad-mode="render" ${ro ? 'disabled' : ''}>${esc(t(ctx, 'desktop.openscad.mode_render', 'Render'))}</button>
                                    <button type="button" class="oscad-seg" data-oscad-mode="preview" ${ro ? 'disabled' : ''}>${esc(t(ctx, 'desktop.openscad.mode_preview', 'Preview'))}</button>
                                </div>
                                <p class="oscad-mode-hint">${esc(t(ctx, 'desktop.openscad.mode_preview_hint', 'Preview is faster for iterating on the model.'))}</p>
                            </section>
                            <section class="oscad-section">
                                <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.timeout', 'Timeout'))}</label>
                                <input class="oscad-input" data-oscad-timeout type="number" min="10" max="600" step="10" ${ro ? 'readonly' : ''}>
                            </section>
                            <section class="oscad-section">
                                <label class="oscad-label">${esc(t(ctx, 'desktop.openscad.defines', 'Custom -D defines'))}</label>
                                <div class="oscad-defines" data-oscad-defines></div>
                            </section>
                            <p class="oscad-shortcuts">${esc(t(ctx, 'desktop.openscad.shortcuts', 'Ctrl+Enter render · Esc cancel · F fit view · Ctrl+S save draft'))}</p>
                        </div>
                    </aside>
                    <aside class="oscad-agent-panel" data-oscad-agent-panel>
                        <div class="oscad-agent-head">
                            <span>${esc(t(ctx, 'desktop.openscad.agent_history', 'Agent chat'))}</span>
                            <button type="button" class="oscad-icon-btn" data-oscad-toggle-agent title="${esc(t(ctx, 'desktop.openscad.collapsed_agent', 'Toggle agent panel'))}" aria-label="${esc(t(ctx, 'desktop.openscad.collapsed_agent', 'Toggle agent panel'))}">${icon(ctx, 'x', 'X', 'oscad-btn-icon', 13)}</button>
                        </div>
                        <div class="oscad-apply-bar" data-oscad-apply-bar hidden>
                            <div class="oscad-apply-summary" data-oscad-apply-summary></div>
                            <div class="oscad-apply-actions">
                                <button type="button" class="oscad-btn oscad-primary" data-oscad-apply-source>${esc(t(ctx, 'desktop.openscad.apply_changes', 'Apply to editor'))}</button>
                                <button type="button" class="oscad-btn" data-oscad-discard-source>${esc(t(ctx, 'desktop.openscad.discard_changes', 'Discard'))}</button>
                            </div>
                        </div>
                        <div class="oscad-transcript" data-oscad-transcript></div>
                        <div class="oscad-composer">
                            <textarea class="oscad-chat" data-oscad-prompt rows="3" placeholder="${esc(t(ctx, 'desktop.openscad.prompt_placeholder', 'Describe the model you want...'))}"></textarea>
                            <button type="button" class="oscad-btn oscad-primary oscad-send-btn" data-oscad-agent-send title="${esc(t(ctx, 'desktop.openscad.ask_agent', 'Ask agent'))}">${icon(ctx, 'agent-chat', 'A', 'oscad-btn-icon', 15)}<span>${esc(t(ctx, 'desktop.openscad.ask_agent', 'Ask agent'))}</span></button>
                        </div>
                    </aside>
                </div>
            </div>`;
        applyShellSizing(state);
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
        const ctx = state.ctx;
        const ro = isOpenSCADReadOnly(ctx);
        host.classList.toggle('busy', !!state.busy);
        host.classList.toggle('light-preview', !!state.lightPreview);
        host.classList.toggle('params-collapsed', !!state.paramsCollapsed);
        host.classList.toggle('agent-collapsed', !!state.agentCollapsed);
        host.classList.toggle('inspector-collapsed', !!state.inspectorCollapsed);
        host.setAttribute('aria-busy', state.busy ? 'true' : 'false');
        const meta = host.querySelector('[data-oscad-run-meta]');
        if (meta) {
            meta.innerHTML = jobMetaHTML(state) + (state.sourceDirty ? `<span class="oscad-dirty">${esc(t(ctx, 'desktop.openscad.render_required', 'Render required'))}</span>` : '');
        }
        const promptEl = host.querySelector('[data-oscad-prompt]');
        if (promptEl) {
            if (document.activeElement !== promptEl) promptEl.value = state.prompt;
            promptEl.readOnly = ro;
        }
        mountDefinesPanel(state);
        host.querySelectorAll('[data-oscad-mode]').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.oscadMode === state.renderMode);
        });
        const timeoutEl = host.querySelector('[data-oscad-timeout]');
        if (timeoutEl && document.activeElement !== timeoutEl) timeoutEl.value = String(state.timeout);
        host.querySelectorAll('[data-oscad-export]').forEach(input => {
            const format = input.dataset.oscadExport;
            input.checked = state.exports.has(format);
            input.disabled = ro;
        });
        const exportCount = host.querySelector('[data-oscad-export-count]');
        if (exportCount) exportCount.textContent = String(state.exports.size);
        const label = host.querySelector('[data-oscad-primary-label]');
        if (label) label.textContent = primaryFileLabel(state);
        const bgBtn = host.querySelector('[data-oscad-background]');
        if (bgBtn) bgBtn.classList.toggle('active', !!state.lightPreview);
        const axesBtn = host.querySelector('[data-oscad-axes]');
        if (axesBtn) axesBtn.classList.toggle('active', !!state.showAxes);
        const projBtn = host.querySelector('[data-oscad-projection]');
        if (projBtn) {
            projBtn.classList.toggle('active', state.viewportProjection === 'orthographic');
            projBtn.title = t(ctx, 'desktop.openscad.viewport_ortho', 'Toggle perspective / orthographic');
        }
        const shadeBtn = host.querySelector('[data-oscad-shading]');
        if (shadeBtn) {
            shadeBtn.classList.toggle('active', state.viewportShading === 'wireframe');
            shadeBtn.title = state.viewportShading === 'wireframe'
                ? t(ctx, 'desktop.openscad.viewport_shaded', 'Shaded surface')
                : t(ctx, 'desktop.openscad.viewport_wireframe', 'Wireframe');
        }
        const rotBtn = host.querySelector('[data-oscad-autorotate]');
        if (rotBtn) rotBtn.classList.toggle('active', !!state.viewportAutoRotate);
        host.querySelectorAll('[data-oscad-toggle-editor]').forEach(btn => {
            btn.setAttribute('aria-expanded', String(!state.inspectorCollapsed));
            btn.classList.toggle('active', !state.inspectorCollapsed);
        });
        host.querySelectorAll('[data-oscad-toggle-params]').forEach(btn => {
            btn.setAttribute('aria-expanded', String(!state.paramsCollapsed));
            btn.classList.toggle('active', !state.paramsCollapsed);
        });
        host.querySelectorAll('[data-oscad-toggle-agent]').forEach(btn => {
            btn.setAttribute('aria-expanded', String(!state.agentCollapsed));
            btn.classList.toggle('active', !state.agentCollapsed);
        });
        host.querySelectorAll('.oscad-tab').forEach(btn => {
            btn.classList.toggle('active', btn.dataset.oscadTab === state.activeTab);
        });
        renderIssues(state);
        renderAgentTranscript(state);
        renderApplyBar(state);
        setOpenSCADBusy(state, state.busy, state.busyMode);
    }

    function applyShellSizing(state) {
        if (!state.host) return;
        const inspector = clampPanelWidth(state.inspectorWidth);
        const sidebar = clampPanelWidth(state.sidebarWidth);
        state.host.style.setProperty('--oscad-inspector-w', inspector ? inspector + 'px' : '');
        state.host.style.setProperty('--oscad-sidebar-w', sidebar ? sidebar + 'px' : '');
    }

    function toggleInspectorPanel(state) {
        state.inspectorCollapsed = !state.inspectorCollapsed;
        syncShellControls(state);
        scheduleOpenSCADDraftSave(state);
        if (!state.inspectorCollapsed) renderInspector(state);
    }

    function wireSplitters(state) {
        state.host.querySelectorAll('[data-oscad-splitter]').forEach(splitter => {
            const target = splitter.dataset.oscadSplitter === 'sidebar' ? 'sidebar' : 'inspector';
            splitter.addEventListener('pointerdown', e => {
                if (e.button !== 0) return;
                e.preventDefault();
                const main = splitter.parentElement;
                const mainRect = main ? main.getBoundingClientRect() : null;
                if (!mainRect) return;
                splitter.setPointerCapture(e.pointerId);
                splitter.classList.add('dragging');
                const onMove = ev => {
                    const px = target === 'sidebar'
                        ? mainRect.right - ev.clientX
                        : ev.clientX - mainRect.left;
                    const clamped = clampPanelWidth(px);
                    if (!clamped) return;
                    if (target === 'sidebar') state.sidebarWidth = clamped;
                    else state.inspectorWidth = clamped;
                    applyShellSizing(state);
                };
                const onUp = () => {
                    splitter.classList.remove('dragging');
                    splitter.removeEventListener('pointermove', onMove);
                    splitter.removeEventListener('pointerup', onUp);
                    splitter.removeEventListener('pointercancel', onUp);
                    scheduleOpenSCADDraftSave(state);
                };
                splitter.addEventListener('pointermove', onMove);
                splitter.addEventListener('pointerup', onUp);
                splitter.addEventListener('pointercancel', onUp);
            });
            splitter.addEventListener('dblclick', () => {
                if (target === 'sidebar') {
                    state.paramsCollapsed = !state.paramsCollapsed;
                    syncShellControls(state);
                    scheduleOpenSCADDraftSave(state);
                } else {
                    toggleInspectorPanel(state);
                }
            });
            splitter.addEventListener('keydown', e => {
                const step = e.key === 'ArrowLeft' ? -28 : e.key === 'ArrowRight' ? 28 : 0;
                if (!step) return;
                e.preventDefault();
                const key = target === 'sidebar' ? 'sidebarWidth' : 'inspectorWidth';
                const base = clampPanelWidth(state[key]) || (target === 'sidebar' ? 288 : 340);
                const delta = target === 'sidebar' ? -step : step;
                state[key] = clampPanelWidth(base + delta);
                applyShellSizing(state);
                scheduleOpenSCADDraftSave(state);
            });
        });
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
        return file ? `${file.name} · ${String(file.format || '').toUpperCase()}` : '';
    }

    function wire(state) {
        const host = state.host;
        const promptEl = host.querySelector('[data-oscad-prompt]');
        if (promptEl) promptEl.addEventListener('input', e => { state.prompt = e.target.value; scheduleOpenSCADDraftSave(state); });
        wireDefinesPanel(state);
        host.querySelectorAll('[data-oscad-mode]').forEach(btn => btn.addEventListener('click', () => {
            if (isOpenSCADReadOnly(state.ctx)) return;
            state.renderMode = btn.dataset.oscadMode || 'render';
            scheduleOpenSCADDraftSave(state);
            syncShellControls(state);
        }));
        const timeoutEl = host.querySelector('[data-oscad-timeout]');
        if (timeoutEl) timeoutEl.addEventListener('input', e => { state.timeout = Number(e.target.value || 120); scheduleOpenSCADDraftSave(state); });
        host.querySelectorAll('[data-oscad-export]').forEach(input => {
            input.addEventListener('change', () => {
                const format = input.dataset.oscadExport;
                if (input.checked) state.exports.add(format);
                else state.exports.delete(format);
                syncShellControls(state);
                scheduleOpenSCADDraftSave(state);
            });
        });
        host.querySelector('[data-oscad-render]').addEventListener('click', () => renderSource(state));
        host.querySelector('[data-oscad-agent]').addEventListener('click', () => askAgent(state));
        const agentSend = host.querySelector('[data-oscad-agent-send]');
        if (agentSend) agentSend.addEventListener('click', () => askAgent(state));
        host.querySelector('[data-oscad-cancel]').addEventListener('click', () => cancelCurrentOpenSCADWork(state));
        host.querySelector('[data-oscad-save]').addEventListener('click', () => saveJob(state));
        host.querySelector('[data-oscad-download]').addEventListener('click', () => downloadPrimary(state));
        host.querySelector('[data-oscad-fullscreen]').addEventListener('click', () => fullscreenPreview(state));
        host.querySelector('[data-oscad-fit]').addEventListener('click', () => resetPreviewView(state));
        host.querySelector('[data-oscad-zoom-in]').addEventListener('click', () => zoomPreview(state, 1.25));
        host.querySelector('[data-oscad-zoom-out]').addEventListener('click', () => zoomPreview(state, 0.8));
        host.querySelector('[data-oscad-background]').addEventListener('click', () => togglePreviewBackground(state));
        host.querySelector('[data-oscad-axes]').addEventListener('click', () => togglePreviewAxes(state));
        host.querySelector('[data-oscad-projection]').addEventListener('click', () => togglePreviewProjection(state));
        host.querySelector('[data-oscad-shading]').addEventListener('click', () => togglePreviewShading(state));
        host.querySelector('[data-oscad-autorotate]').addEventListener('click', () => togglePreviewAutoRotate(state));
        host.querySelectorAll('[data-oscad-toggle-editor]').forEach(btn => btn.addEventListener('click', () => toggleInspectorPanel(state)));
        host.querySelectorAll('[data-oscad-toggle-params]').forEach(btn => btn.addEventListener('click', () => {
            state.paramsCollapsed = !state.paramsCollapsed;
            syncShellControls(state);
            scheduleOpenSCADDraftSave(state);
        }));
        host.querySelectorAll('[data-oscad-toggle-agent]').forEach(btn => btn.addEventListener('click', () => {
            state.agentCollapsed = !state.agentCollapsed;
            syncShellControls(state);
            scheduleOpenSCADDraftSave(state);
            if (!state.agentCollapsed) {
                const pe = host.querySelector('[data-oscad-prompt]');
                if (pe) setTimeout(() => pe.focus(), 50);
            }
        }));
        wireSplitters(state);
        const applyBtn = host.querySelector('[data-oscad-apply-source]');
        if (applyBtn) applyBtn.addEventListener('click', () => applyAgentSource(state));
        const discardBtn = host.querySelector('[data-oscad-discard-source]');
        if (discardBtn) discardBtn.addEventListener('click', () => discardAgentSource(state));
        const refreshEl = host.querySelector('[data-oscad-refresh]');
        if (refreshEl) refreshEl.addEventListener('click', () => loadStatus(state));
        host.querySelectorAll('[data-oscad-tab]').forEach(btn => btn.addEventListener('click', () => {
            state.activeTab = btn.dataset.oscadTab;
            syncShellControls(state);
            renderInspector(state);
            scheduleOpenSCADDraftSave(state);
        }));
        wireKeyboardShortcuts(state);
    }

    function wireKeyboardShortcuts(state) {
        const onKey = e => {
            if (!state.host || !state.host.isConnected) return;
            const mod = e.ctrlKey || e.metaKey;
            const target = e.target;
            const typing = target && target.closest && target.closest('input, textarea, select, [contenteditable="true"], .cm-content');
            if (mod && e.key === 'Enter') { e.preventDefault(); renderSource(state); return; }
            if (mod && (e.key === 's' || e.key === 'S')) {
                e.preventDefault();
                persistOpenSCADDraft(state);
                setStatus(state, t(state.ctx, 'desktop.openscad.draft_saved', 'Draft saved'));
                return;
            }
            if (e.key === 'Escape') {
                if (state.busy) { cancelCurrentOpenSCADWork(state); return; }
                if (state.pendingAgentSource != null) { discardAgentSource(state); return; }
                if (!state.agentCollapsed) { state.agentCollapsed = true; syncShellControls(state); scheduleOpenSCADDraftSave(state); return; }
                return;
            }
            if (!mod && !typing && (e.key === 'f' || e.key === 'F')) {
                e.preventDefault();
                resetPreviewView(state);
            }
        };
        state.host.addEventListener('keydown', onKey);
        state.keyHandler = onKey;
        state.listeners.push(() => {
            if (state.host && state.keyHandler) state.host.removeEventListener('keydown', state.keyHandler);
            state.keyHandler = null;
        });
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
        let payload = null;
        if (data && data.type === 'openscad_result') {
            payload = data.payload || data.result || null;
        } else if (isOpenSCADResultPayload(data)) {
            payload = data;
        }
        if (!payload) return;
        const targetWindowId = String(
            payload.window_id ||
            payload.windowId ||
            (data && (data.window_id || data.windowId)) ||
            ''
        ).trim();
        if (targetWindowId && targetWindowId !== String(state.windowId || '')) return;
        if (!targetWindowId && stateByWindow.size > 1 && !state.busy) return;
        state.result = payload;
        if (payload && typeof payload.source_scad === 'string' && payload.source_scad.length) {
            if (isOpenSCADReadOnly(state.ctx)) {
                state.source = payload.source_scad;
                if (state.editor && typeof state.editor.setValue === 'function') {
                    state.editor.setValue(payload.source_scad);
                }
            } else if (payload.source_scad === state.source) {
                state.pendingAgentSource = null;
            } else {
                state.pendingAgentSource = payload.source_scad;
                state.pendingAgentSummary = diffSummary(state.source, payload.source_scad);
            }
        }
        if (state.editor && payload && payload.stderr) {
            const errors = parseOpenSCADErrors(payload.stderr);
            state.lastErrors = errors;
            if (errors.length) {
                state.editor.setErrors(errors);
            } else {
                state.editor.clearErrors();
            }
        } else if (state.editor) {
            state.lastErrors = [];
            state.editor.clearErrors();
        }
        state.sourceDirty = false;
        state.activeTab = 'files';
        setOpenSCADBusy(state, false);
        draw(state);
        persistOpenSCADDraft(state);
        setStatus(state, t(state.ctx, 'desktop.openscad.render_complete', 'Render complete'));
    }

    function isOpenSCADResultPayload(value) {
        return !!(value && typeof value === 'object' && (
            typeof value.job_id === 'string' ||
            Array.isArray(value.files) ||
            typeof value.source_scad === 'string' ||
            typeof value.download_base === 'string'
        ));
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
        const previewFirst = exports.includes('png') && exports.length > 1;
        const previewExports = previewFirst ? ['png'] : exports;
        const remainingExports = previewFirst ? exports.filter(format => format !== 'png') : [];
        const startedAt = openSCADNowMS();
        logOpenSCAD(state, 'render requested', { exports, preview_exports: previewExports, remaining_exports: remainingExports, render_mode: state.renderMode, timeout_seconds: Number(state.timeout) || 120 });
        state.renderSerial += 1;
        const renderSerial = state.renderSerial;
        if (state.exportAbort) {
            state.exportAbort.abort();
            state.exportAbort = null;
        }
        state.cancelRequested = false;
        setOpenSCADBusy(state, true, 'render');
        setStatus(state, t(state.ctx, 'desktop.openscad.rendering', 'Rendering...'));
        const controller = new AbortController();
        state.renderAbort = controller;
        const timeout = window.setTimeout(() => controller.abort(), renderRequestTimeoutMS(state));
        try {
            const body = await renderOpenSCADRequest(state, previewExports, controller.signal);
            state.result = body && body.result ? body.result : null;
            logOpenSCAD(state, 'preview render completed', { exports: previewExports, elapsed_ms: Math.round(openSCADNowMS() - startedAt), status: body && body.status, result: openSCADResultSummary(state.result) });
            if (body && body.status === 'error') {
                const hasFiles = hasOpenSCADResultFiles(state.result);
                warnOpenSCAD(state, 'render failed', { exports: previewExports, elapsed_ms: Math.round(openSCADNowMS() - startedAt), error: body.error || '', result: openSCADResultSummary(state.result) });
                if (state.editor && state.result && state.result.stderr) {
                    const errors = parseOpenSCADErrors(state.result.stderr);
                    state.lastErrors = errors;
                    if (errors.length) state.editor.setErrors(errors);
                }
                state.activeTab = hasFiles ? 'files' : (state.result ? 'log' : state.activeTab);
                setOpenSCADBusy(state, false);
                draw(state);
                setStatus(state, body.error || t(state.ctx, 'desktop.openscad.render_failed', 'Render failed'), !hasFiles);
                return;
            }
            state.sourceDirty = false;
            state.activeTab = 'files';
            setOpenSCADBusy(state, false);
            draw(state);
            persistOpenSCADDraft(state);
            setStatus(state, state.result ? t(state.ctx, 'desktop.openscad.render_complete', 'Render complete') : t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'), !state.result);
            if (remainingExports.length && !state.cancelRequested && renderSerial === state.renderSerial) {
                renderRemainingOpenSCADExports(state, remainingExports, renderSerial);
            }
        } catch (err) {
            setOpenSCADBusy(state, false);
            const partial = err && err.body && err.body.result ? err.body.result : null;
            if (partial && !state.cancelRequested && renderSerial === state.renderSerial) {
                state.result = partial;
                state.activeTab = 'log';
                draw(state);
            }
            const message = err && err.name === 'AbortError'
                ? (state.cancelRequested ? t(state.ctx, 'desktop.openscad.cancelled', 'Cancelled') : t(state.ctx, 'desktop.openscad.render_timeout', 'Render timed out. Try a simpler model or increase the timeout.'))
                : (err && err.message) || String(err);
            warnOpenSCAD(state, 'render failed', { exports: previewExports, elapsed_ms: Math.round(openSCADNowMS() - startedAt), error: message, aborted: err && err.name === 'AbortError', result: openSCADResultSummary(partial) });
            setStatus(state, message, true);
        } finally {
            window.clearTimeout(timeout);
            if (state.renderAbort === controller) state.renderAbort = null;
            if (renderSerial === state.renderSerial) state.cancelRequested = false;
        }
    }

    function renderOpenSCADRequest(state, exports, signal) {
        return state.ctx.api('/api/openscad/render', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            signal,
            body: JSON.stringify({
                source_scad: state.source,
                model_name: 'model',
                exports,
                defines: parseDefinesText(state.definesText),
                render_mode: state.renderMode,
                timeout_seconds: state.timeout,
                window_id: state.windowId
            })
        });
    }

    async function renderRemainingOpenSCADExports(state, exports, renderSerial) {
        const controller = new AbortController();
        const startedAt = openSCADNowMS();
        state.exportAbort = controller;
        logOpenSCAD(state, 'background exports started', { exports, render_serial: renderSerial, timeout_seconds: Number(state.timeout) || 120 });
        const timeout = window.setTimeout(() => controller.abort(), renderRequestTimeoutMS(state));
        try {
            setStatus(state, t(state.ctx, 'desktop.openscad.rendering', 'Rendering...'));
            const body = await renderOpenSCADRequest(state, exports, controller.signal);
            if (renderSerial !== state.renderSerial) return;
            const nextResult = body && body.result ? body.result : null;
            if (nextResult) {
                state.result = mergeOpenSCADResults(state.result, nextResult);
                state.activeTab = 'files';
                draw(state);
                persistOpenSCADDraft(state);
            }
            logOpenSCAD(state, 'background exports completed', { exports, elapsed_ms: Math.round(openSCADNowMS() - startedAt), status: body && body.status, result: openSCADResultSummary(nextResult) });
            if (body && body.status === 'error') {
                const hasFiles = hasOpenSCADResultFiles(state.result);
                warnOpenSCAD(state, 'background exports failed', { exports, elapsed_ms: Math.round(openSCADNowMS() - startedAt), error: body.error || '', result: openSCADResultSummary(nextResult) });
                setStatus(state, body.error || t(state.ctx, 'desktop.openscad.render_failed', 'Render failed'), !hasFiles);
                return;
            }
            setStatus(state, t(state.ctx, 'desktop.openscad.render_complete', 'Render complete'));
        } catch (err) {
            if (renderSerial !== state.renderSerial) return;
            const partial = err && err.body && err.body.result ? err.body.result : null;
            if (partial && !state.cancelRequested) {
                state.result = mergeOpenSCADResults(state.result, partial);
                state.activeTab = 'files';
                draw(state);
                persistOpenSCADDraft(state);
            }
            const message = err && err.name === 'AbortError'
                ? (state.cancelRequested ? t(state.ctx, 'desktop.openscad.cancelled', 'Cancelled') : t(state.ctx, 'desktop.openscad.render_timeout', 'Render timed out. Try a simpler model or increase the timeout.'))
                : (err && err.message) || String(err);
            warnOpenSCAD(state, 'background exports failed', { exports, elapsed_ms: Math.round(openSCADNowMS() - startedAt), error: message, aborted: err && err.name === 'AbortError', result: openSCADResultSummary(partial) });
            setStatus(state, message, !hasOpenSCADResultFiles(state.result));
        } finally {
            window.clearTimeout(timeout);
            if (state.exportAbort === controller) state.exportAbort = null;
            if (renderSerial === state.renderSerial) state.cancelRequested = false;
        }
    }

    async function askAgent(state) {
        const message = state.prompt.trim();
        if (!message || state.busy || isOpenSCADReadOnly(state.ctx)) return;
        state.agentHistory.push({ role: 'user', text: message });
        if (state.agentHistory.length > 60) state.agentHistory = state.agentHistory.slice(-60);
        renderAgentTranscript(state);
        state.agentCollapsed = false;
        syncShellControls(state);
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
                    appendAgentDelta(state, data.detail);
                }
            });
        }
    }

    function appendAgentDelta(state, delta) {
        const last = state.agentHistory.length ? state.agentHistory[state.agentHistory.length - 1] : null;
        if (!last || last.role !== 'assistant') {
            state.agentHistory.push({ role: 'assistant', text: delta });
        } else {
            last.text = (last.text || '') + delta;
        }
        if (state.agentHistory.length > 60) state.agentHistory = state.agentHistory.slice(-60);
        setStatus(state, (last.text || '').slice(-160));
        renderAgentTranscript(state);
    }

    function renderAgentTranscript(state) {
        const el = state.host && state.host.querySelector('[data-oscad-transcript]');
        if (!el) return;
        if (!state.agentHistory.length) {
            el.innerHTML = '<div class="oscad-transcript-empty"></div>';
            return;
        }
        el.innerHTML = state.agentHistory.map(msg => {
            const cls = msg.role === 'user' ? 'oscad-msg-user' : 'oscad-msg-assistant';
            return '<div class="' + cls + '"><div class="oscad-msg-text">' + esc(msg.text || '') + '</div></div>';
        }).join('');
        el.scrollTop = el.scrollHeight;
    }

    function renderApplyBar(state) {
        const bar = state.host && state.host.querySelector('[data-oscad-apply-bar]');
        if (!bar) return;
        const summary = state.host.querySelector('[data-oscad-apply-summary]');
        if (state.pendingAgentSource == null) {
            bar.hidden = true;
            return;
        }
        bar.hidden = false;
        if (summary) summary.textContent = t(state.ctx, 'desktop.openscad.pending_source', 'The agent proposed changes to your model.') + ' ' + state.pendingAgentSummary;
    }

    function applyAgentSource(state) {
        if (state.pendingAgentSource == null) return;
        state.source = state.pendingAgentSource;
        state.pendingAgentSource = null;
        state.pendingAgentSummary = '';
        state.sourceDirty = true;
        if (state.editor && typeof state.editor.setValue === 'function') {
            state.editor.setValue(state.source);
        }
        updateWindowContext(state);
        scheduleOpenSCADDraftSave(state);
        setStatus(state, t(state.ctx, 'desktop.openscad.render_required', 'Render required'));
        syncShellControls(state);
    }

    function discardAgentSource(state) {
        state.pendingAgentSource = null;
        state.pendingAgentSummary = '';
        syncShellControls(state);
    }

    function diffSummary(before, after) {
        const aLines = String(before || '').split(/\r?\n/);
        const bLines = String(after || '').split(/\r?\n/);
        const aCount = new Map();
        aLines.forEach(l => aCount.set(l, (aCount.get(l) || 0) + 1));
        let common = 0;
        const remaining = new Map(aCount);
        bLines.forEach(l => {
            if ((remaining.get(l) || 0) > 0) { common++; remaining.set(l, remaining.get(l) - 1); }
        });
        const removed = Math.max(0, aLines.length - common);
        const added = Math.max(0, bLines.length - common);
        return '+' + added + ' / −' + removed;
    }

    async function saveJob(state) {
        if (!state.result || !state.result.job_id || isOpenSCADReadOnly(state.ctx)) return;
        setStatus(state, t(state.ctx, 'desktop.openscad.saving', 'Saving...'));
        try {
            const savedResults = [];
            for (const jobID of openSCADResultJobIDs(state.result)) {
                const body = await state.ctx.api(`/api/openscad/jobs/${encodeURIComponent(jobID)}/save`, { method: 'POST' });
                if (body && body.result) savedResults.push(body.result);
            }
            state.result = savedResults.reduce((merged, result) => mergeOpenSCADResults(merged, result), null) || state.result;
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
        const zone = state.host.querySelector('[data-oscad-preview-zone]');
        const target = zone || state.host.querySelector('[data-oscad-preview-panel]');
        if (target && target.requestFullscreen) target.requestFullscreen();
    }

    function zoomPreview(state, factor) {
        const p3d = state.preview3D;
        if (!p3d || !p3d.camera || !window.THREE) return;
        const target = p3d.controls && p3d.controls.target ? p3d.controls.target : new THREE.Vector3(0, 0, 0);
        if (p3d.camera.isOrthographicCamera) {
            p3d.camera.zoom = Math.min(12, Math.max(0.08, (p3d.camera.zoom || 1) * factor));
            p3d.camera.updateProjectionMatrix();
        } else {
            const offset = p3d.camera.position.clone().sub(target);
            const length = offset.length();
            const next = Math.min(20000, Math.max(2, length / Math.max(0.2, factor)));
            offset.setLength(next);
            p3d.camera.position.copy(target).add(offset);
        }
        if (p3d.controls && typeof p3d.controls.update === 'function') p3d.controls.update();
    }

    function renderPanel(state) {
        renderViewport(state);
        renderInspector(state);
        renderIssues(state);
    }

    function renderIssues(state) {
        const el = state.host && state.host.querySelector('[data-oscad-issues]');
        if (!el) return;
        const errors = state.lastErrors && state.lastErrors.length ? state.lastErrors : [];
        if (!errors.length) {
            el.innerHTML = '';
            el.hidden = true;
            return;
        }
        el.hidden = false;
        el.innerHTML = '<div class="oscad-issues-head">' + esc(t(state.ctx, 'desktop.openscad.issues', 'Issues')) + ' · ' + errors.length + '</div>' +
            errors.map((err, idx) => {
                const cls = err.severity === 'warning' ? 'oscad-issue-warning' : 'oscad-issue-error';
                return '<button type="button" class="oscad-issue ' + cls + '" data-oscad-issue="' + idx + '"><span class="oscad-issue-line">' + esc(t(state.ctx, 'desktop.openscad.error_line', 'Line {line}: {message}').replace('{line}', String(err.line || 0)).replace('{message}', err.message || '')) + '</span></button>';
            }).join('');
        el.querySelectorAll('[data-oscad-issue]').forEach(btn => btn.addEventListener('click', () => {
            const idx = Number(btn.dataset.oscadIssue);
            const err = errors[idx];
            if (!err || typeof err.line !== 'number') return;
            state.activeTab = 'source';
            if (state.inspectorCollapsed) {
                state.inspectorCollapsed = false;
                scheduleOpenSCADDraftSave(state);
            }
            syncShellControls(state);
            renderInspector(state);
            if (state.editor && typeof state.editor.revealLine === 'function') {
                state.editor.revealLine(err.line);
            } else if (window.OpenSCADEditor && typeof window.OpenSCADEditor.revealLine === 'function') {
                window.OpenSCADEditor.revealLine(state.editor, err.line);
            }
        }));
    }

    function renderInspector(state) {
        const panel = state.host.querySelector('[data-oscad-inspector-panel]');
        if (!panel) return;
        if (state.activeTab === 'source') {
            if (!state.sourceEditorReady) {
                panel.innerHTML = '<div class="oscad-source oscad-source-loading" data-oscad-source>' + esc(t(state.ctx, 'desktop.openscad.editor_loading', 'Loading editor...')) + '</div>';
                const mountEl = panel.querySelector('[data-oscad-source]');
                if (mountEl && window.OpenSCADEditor && typeof window.OpenSCADEditor.create === 'function') {
                    mountEl.textContent = '';
                    state.editor = window.OpenSCADEditor.create(state, mountEl, function(text) {
                        if (isOpenSCADReadOnly(state.ctx)) return;
                        state.source = text;
                        state.sourceDirty = true;
                        updateWindowContext(state);
                        setStatus(state, t(state.ctx, 'desktop.openscad.render_required', 'Render required'));
                        const meta = state.host.querySelector('[data-oscad-run-meta]');
                        if (meta) {
                            meta.innerHTML = jobMetaHTML(state) + '<span class="oscad-dirty">' + esc(t(state.ctx, 'desktop.openscad.render_required', 'Render required')) + '</span>';
                        }
                        setWindowMenus(state);
                        scheduleOpenSCADDraftSave(state);
                    });
                    if (state.editor && typeof state.editor.setReadOnly === 'function') {
                        state.editor.setReadOnly(isOpenSCADReadOnly(state.ctx));
                    }
                    state.sourceEditorReady = true;
                } else {
                    warnOpenSCAD(state, 'editor module unavailable');
                }
            } else if (state.editor && typeof state.editor.setReadOnly === 'function') {
                state.editor.setReadOnly(isOpenSCADReadOnly(state.ctx));
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

    function resultFiles(state) { return state.result && Array.isArray(state.result.files) ? state.result.files : []; }

    function hasOpenSCADResultFiles(result) { return !!(result && Array.isArray(result.files) && result.files.length); }

    function mergeOpenSCADResults(current, next) {
        if (!current) return next || null;
        if (!next) return current;
        const merged = Object.assign({}, current);
        const seen = new Set();
        const fileIndexes = new Map();
        merged.files = [];
        [current, next].forEach(result => {
            (Array.isArray(result.files) ? result.files : []).forEach(file => {
                const key = [file.name, file.format, file.preview_url || file.download_url || ''].join('|');
                if (seen.has(key)) {
                    const existing = merged.files[fileIndexes.get(key)];
                    if (existing && !existing.saved_path && file.saved_path) existing.saved_path = file.saved_path;
                    return;
                }
                seen.add(key);
                fileIndexes.set(key, merged.files.length);
                merged.files.push(file);
            });
        });
        merged.duration_ms = Number(current.duration_ms || 0) + Number(next.duration_ms || 0);
        merged.stdout = [current.stdout, next.stdout].filter(Boolean).join('\n');
        merged.stderr = [current.stderr, next.stderr].filter(Boolean).join('\n');
        merged.saved_paths = (Array.isArray(current.saved_paths) ? current.saved_paths : []).concat(Array.isArray(next.saved_paths) ? next.saved_paths : []);
        return merged;
    }

    function openSCADResultJobIDs(result) {
        const ids = [], seen = new Set();
        const add = value => {
            const id = String(value || '').trim();
            if (id && !seen.has(id)) { seen.add(id); ids.push(id); }
        };
        add(result && result.job_id);
        (result && Array.isArray(result.files) ? result.files : []).forEach(file => { add(openSCADJobIDFromURL(file.preview_url)); add(openSCADJobIDFromURL(file.download_url)); });
        return ids;
    }

    function openSCADJobIDFromURL(url) {
        const match = String(url || '').match(/\/api\/openscad\/jobs\/([^/]+)\//);
        return match ? decodeURIComponent(match[1]) : '';
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
            renderPreviewEmptyState(state, panel);
            return;
        }
        const url = previewURL(file);
        if (!url) {
            state.previewStlURL = '';
            renderPreviewEmptyState(state, panel);
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

    function renderPreviewEmptyState(state, panel) {
        const ctx = state.ctx;
        const ro = isOpenSCADReadOnly(ctx);
        panel.innerHTML = `<div class="oscad-empty oscad-empty-hero">
            <div class="oscad-empty-icon">${icon(ctx, 'openscad', 'O', 'oscad-empty-glyph', 44)}</div>
            <strong>${esc(t(ctx, 'desktop.openscad.empty_preview_title', 'No preview yet'))}</strong>
            <span>${esc(t(ctx, 'desktop.openscad.empty_preview_hint', 'Render your model to see an interactive preview here.'))}</span>
            ${ro ? '' : `<button type="button" class="oscad-btn oscad-primary oscad-empty-cta" data-oscad-empty-render>${icon(ctx, 'run', 'R', 'oscad-btn-icon', 15)}<span>${esc(t(ctx, 'desktop.openscad.render_now', 'Render now'))}</span></button><kbd class="oscad-empty-kbd">Ctrl+Enter</kbd>`}
        </div>`;
        const cta = panel.querySelector('[data-oscad-empty-render]');
        if (cta) cta.addEventListener('click', () => renderSource(state));
    }

    function bindPreviewLoadError(state, panel, element) {
        if (!element) return;
        element.addEventListener('error', () => {
            panel.innerHTML = `<div class="oscad-empty"><strong>${esc(t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'))}</strong><span>${esc(t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'))}</span></div>`;
            setStatus(state, t(state.ctx, 'desktop.openscad.download_hint', 'Preview is not interactive for this format. Download or save the file.'), true);
        }, { once: true });
    }

    function setOpenSCADBusy(state, busy, mode) {
        const wasBusy = state.busy;
        state.busy = !!busy;
        state.busyMode = state.busy ? (mode || state.busyMode || 'work') : '';
        if (state.busy && !wasBusy) {
            state.busyStartedAt = Date.now();
            if (state.busyTicker) window.clearInterval(state.busyTicker);
            state.busyTicker = window.setInterval(() => updateOpenSCADBusyElapsed(state), 500);
        } else if (!state.busy && wasBusy) {
            state.busyStartedAt = 0;
            if (state.busyTicker) {
                window.clearInterval(state.busyTicker);
                state.busyTicker = null;
            }
        }
        if (!state.host) return;
        const ro = isOpenSCADReadOnly(state.ctx);
        state.host.classList.toggle('busy', state.busy);
        state.host.classList.toggle('busy-render', state.busy && state.busyMode === 'render');
        state.host.classList.toggle('busy-agent', state.busy && state.busyMode === 'agent');
        state.host.setAttribute('aria-busy', state.busy ? 'true' : 'false');
        const overlay = state.host.querySelector('[data-oscad-busy-label]');
        if (overlay) {
            overlay.hidden = !state.busy;
            const textEl = overlay.querySelector('.oscad-busy-text');
            if (textEl) {
                textEl.textContent = state.busyMode === 'agent'
                    ? t(state.ctx, 'desktop.openscad.agent_working', 'Agent is working...')
                    : t(state.ctx, 'desktop.openscad.rendering', 'Rendering...');
            }
            updateOpenSCADBusyElapsed(state);
        }
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

    function updateOpenSCADBusyElapsed(state) {
        if (!state.host) return;
        const elapsedEl = state.host.querySelector('[data-oscad-busy-elapsed]');
        if (!elapsedEl) return;
        if (!state.busy || !state.busyStartedAt) {
            elapsedEl.textContent = '';
            return;
        }
        const seconds = Math.max(0, Math.round((Date.now() - state.busyStartedAt) / 1000));
        elapsedEl.textContent = state.ctx && typeof state.ctx.t === 'function'
            ? state.ctx.t('desktop.noisemaker_progress_elapsed', { seconds })
            : String(seconds);
    }

    function renderRequestTimeoutMS(state) {
        const seconds = Math.max(10, Math.min(Number(state.timeout) || 120, 600));
        return (seconds + 45) * 1000;
    }

    function cancelCurrentOpenSCADWork(state) {
        state.cancelRequested = true;
        if (state.renderAbort) state.renderAbort.abort();
        if (state.exportAbort) state.exportAbort.abort();
        if (state.agentAbort) state.agentAbort.abort();
        setOpenSCADBusy(state, false);
        setStatus(state, t(state.ctx, 'desktop.openscad.cancelled', 'Cancelled'), true);
    }

    function framePreviewCamera(state, p3d, box) {
        if (!p3d || !p3d.camera || !window.THREE) return;
        const size = box.getSize(new THREE.Vector3()).length() || 80;
        const radius = Math.max(1, size / 2);
        const center = box.getCenter(new THREE.Vector3());
        const dir = new THREE.Vector3(1, 0.72, 1).normalize();
        if (p3d.camera.isOrthographicCamera) {
            const el = p3d.renderer && p3d.renderer.domElement;
            const aspect = (el && el.clientWidth ? el.clientWidth : 1) / Math.max(1, el && el.clientHeight ? el.clientHeight : 1);
            const halfH = radius * 1.28;
            p3d.camera.left = -halfH * aspect;
            p3d.camera.right = halfH * aspect;
            p3d.camera.top = halfH;
            p3d.camera.bottom = -halfH;
            p3d.camera.zoom = 1;
            p3d.camera.updateProjectionMatrix();
            p3d.camera.position.copy(center).add(dir.multiplyScalar(radius * 4));
        } else {
            const fov = (p3d.camera.fov || 30) * Math.PI / 180;
            const dist = (radius / Math.tan(fov / 2)) * 1.28;
            p3d.camera.position.copy(center).add(dir.multiplyScalar(dist));
        }
        p3d.camera.lookAt(center);
        if (p3d.controls && p3d.controls.target) p3d.controls.target.copy(center);
        if (p3d.controls && typeof p3d.controls.update === 'function') p3d.controls.update();
    }

    function resetPreviewView(state) {
        const p3d = state.preview3D;
        if (p3d && p3d.mesh && p3d.camera && window.THREE) {
            const box = new THREE.Box3().setFromObject(p3d.mesh);
            framePreviewCamera(state, p3d, box);
            return;
        }
        renderViewport(state);
    }

    function togglePreviewBackground(state) {
        state.lightPreview = !state.lightPreview;
        const p3d = state.preview3D;
        if (p3d && p3d.scene && window.THREE) {
            const texture = previewBackgroundTexture(state);
            if (p3d.bgTexture) {
                try { p3d.bgTexture.dispose(); } catch (_) {}
            }
            p3d.bgTexture = texture;
            p3d.scene.background = texture;
            if (p3d.mesh && p3d.mesh.material && p3d.mesh.material.color) {
                p3d.mesh.material.color.set(state.lightPreview ? 0x5f87a8 : 0x2dd4bf);
            }
            if (p3d.shadowPlane && p3d.shadowPlane.material) {
                p3d.shadowPlane.material.opacity = state.lightPreview ? 0.16 : 0.3;
            }
        }
        syncShellControls(state);
        if (!p3d) renderViewport(state);
        scheduleOpenSCADDraftSave(state);
    }

    function togglePreviewAxes(state) {
        state.showAxes = !state.showAxes;
        const p3d = state.preview3D;
        if (p3d && p3d.gridHelper) {
            p3d.gridHelper.visible = state.showAxes !== false;
        } else {
            renderViewport(state);
        }
        syncShellControls(state);
        scheduleOpenSCADDraftSave(state);
    }

    function prefersReducedMotion() {
        try { return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches); } catch (_) { return false; }
    }

    function togglePreviewProjection(state) {
        state.viewportProjection = state.viewportProjection === 'orthographic' ? 'perspective' : 'orthographic';
        const p3d = state.preview3D;
        if (p3d && p3d.mesh) {
            applyProjection(state, state.viewportProjection);
        } else {
            renderViewport(state);
        }
        syncShellControls(state);
        scheduleOpenSCADDraftSave(state);
    }

    function togglePreviewShading(state) {
        state.viewportShading = state.viewportShading === 'wireframe' ? 'shaded' : 'wireframe';
        const p3d = state.preview3D;
        if (p3d && p3d.mesh && p3d.mesh.material) {
            p3d.mesh.material.wireframe = state.viewportShading === 'wireframe';
        }
        syncShellControls(state);
        scheduleOpenSCADDraftSave(state);
    }

    function togglePreviewAutoRotate(state) {
        state.viewportAutoRotate = !state.viewportAutoRotate;
        const p3d = state.preview3D;
        if (p3d && p3d.controls) {
            p3d.controls.autoRotate = !!state.viewportAutoRotate && !prefersReducedMotion();
        }
        syncShellControls(state);
        scheduleOpenSCADDraftSave(state);
    }

    function applyProjection(state, mode) {
        const p3d = state.preview3D;
        if (!p3d || !window.THREE) return;
        const wantOrtho = mode === 'orthographic';
        const aspect = (p3d.renderer && p3d.renderer.domElement ? p3d.renderer.domElement.clientWidth / Math.max(1, p3d.renderer.domElement.clientHeight) : 1) || 1;
        const box = p3d.mesh ? new THREE.Box3().setFromObject(p3d.mesh) : null;
        const size = box ? (box.getSize(new THREE.Vector3()).length() || 80) : 80;
        let cam;
        const target = p3d.controls && p3d.controls.target ? p3d.controls.target.clone() : new THREE.Vector3(0, 0, 0);
        const dir = p3d.camera.position.clone().sub(target).normalize();
        if (wantOrtho) {
            const halfH = size * 0.75;
            const halfW = halfH * aspect;
            cam = new THREE.OrthographicCamera(-halfW, halfW, halfH, -halfH, 1, 20000);
            cam.position.copy(target).add(dir.multiplyScalar(size * 2.5));
        } else {
            cam = new THREE.PerspectiveCamera(30, aspect, 1, 8000);
            cam.position.copy(target).add(dir.multiplyScalar(size * 1.6));
        }
        cam.lookAt(target);
        cam.updateProjectionMatrix();
        if (p3d.controls) {
            try { if (typeof p3d.controls.dispose === 'function') p3d.controls.dispose(); } catch (_) {}
        }
        const controls = new THREE.OrbitControls(cam, p3d.renderer.domElement);
        controls.target.copy(target);
        controls.enableDamping = true;
        controls.dampingFactor = 0.09;
        controls.autoRotate = !!state.viewportAutoRotate && !prefersReducedMotion();
        controls.autoRotateSpeed = 2.2;
        controls.update();
        p3d.camera = cam;
        p3d.controls = controls;
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
                    { type: 'separator' },
                    { id: 'projection', labelKey: 'desktop.openscad.viewport_ortho', icon: 'cube', checked: state.viewportProjection === 'orthographic', action: () => togglePreviewProjection(state) },
                    { id: 'shading', labelKey: 'desktop.openscad.viewport_wireframe', icon: 'grid', checked: state.viewportShading === 'wireframe', action: () => togglePreviewShading(state) },
                    { id: 'autorotate', labelKey: 'desktop.openscad.viewport_auto_rotate', icon: 'rotate', checked: !!state.viewportAutoRotate, action: () => togglePreviewAutoRotate(state) },
                    { type: 'separator' },
                    { id: 'fullscreen', labelKey: 'desktop.openscad.fullscreen', icon: 'fullscreen', action: () => fullscreenPreview(state) }
                ]
            }
        ]);
    }

    function clearWindowMenus(state) {
        if (!state.ctx || typeof state.ctx.setWindowMenus !== 'function') return;
        state.ctx.setWindowMenus(state.windowId, []);
    }

    function setStatus(state, message, isError) {
        state.statusMessage = message || '';
        state.statusError = !!isError;
        const statusEl = state.host ? state.host.querySelector('[data-oscad-status]') : null;
        if (statusEl) {
            statusEl.textContent = state.statusMessage;
            statusEl.classList.toggle('error', state.statusError);
        }
    }

    function updateWindowContext(state) {
        if (!state.ctx || typeof state.ctx.updateWindowContext !== 'function') return;
        state.ctx.updateWindowContext(state.windowId, { source: state.source });
    }

    function previewURL(file) {
        return file.preview_url || file.download_url || '';
    }

    function primaryFile(state) {
        const files = resultFiles(state);
        if (!files.length) return null;
        const png = files.find(f => f.format === 'png');
        if (png) return png;
        const stl = files.find(f => f.format === 'stl');
        if (stl) return stl;
        const svg = files.find(f => f.format === 'svg');
        if (svg) return svg;
        return files[0];
    }

    function formatSize(bytes) {
        if (bytes == null || isNaN(bytes) || bytes < 1) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB'];
        let i = 0;
        let size = Number(bytes);
        while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
        return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
    }

    function emptyPanel(state, key, fallback) {
        return `<div class="oscad-empty">${esc(t(state.ctx, key, fallback))}</div>`;
    }

    function normalizeEventData(value) {
        if (!value) return null;
        if (typeof value === 'string') {
            try { return JSON.parse(value); } catch (_) { return null; }
        }
        return value;
    }

    function cleanupPreview(state) {
        if (state.previewCleanup) {
            try { state.previewCleanup(); } catch (_) {}
            state.previewCleanup = null;
        }
        if (state.preview3D) {
            const p3d = state.preview3D;
            if (p3d.resizeObserver) {
                try { p3d.resizeObserver.disconnect(); } catch (_) {}
                p3d.resizeObserver = null;
            }
            if (p3d.animId) {
                try { cancelAnimationFrame(p3d.animId); } catch (_) {}
                p3d.animId = null;
            }
            if (p3d.controls && typeof p3d.controls.dispose === 'function') {
                try { p3d.controls.dispose(); } catch (_) {}
            }
            if (p3d.renderer && p3d.renderer.domElement && p3d.onDblClick) {
                try { p3d.renderer.domElement.removeEventListener('dblclick', p3d.onDblClick); } catch (_) {}
            }
            if (p3d.mesh) {
                try { p3d.mesh.geometry && p3d.mesh.geometry.dispose(); } catch (_) {}
                try { p3d.mesh.material && p3d.mesh.material.dispose(); } catch (_) {}
            }
            if (p3d.shadowPlane) {
                try { p3d.shadowPlane.geometry && p3d.shadowPlane.geometry.dispose(); } catch (_) {}
                try { p3d.shadowPlane.material && p3d.shadowPlane.material.dispose(); } catch (_) {}
            }
            if (p3d.gridHelper) {
                try { p3d.gridHelper.geometry && p3d.gridHelper.geometry.dispose(); } catch (_) {}
                try { p3d.gridHelper.material && p3d.gridHelper.material.dispose(); } catch (_) {}
            }
            if (p3d.bgTexture) {
                try { p3d.bgTexture.dispose(); } catch (_) {}
            }
            if (p3d.renderer && !p3d.renderer.disposed) {
                try { p3d.renderer.dispose(); } catch (_) {}
                p3d.renderer.disposed = true;
            }
            if (state.stl) {
                try { state.stl.dispose && state.stl.dispose(); } catch (_) {}
                state.stl = null;
            }
            state.preview3D = null;
        }
    }

    function previewBackgroundTexture(state) {
        const canvas = document.createElement('canvas');
        canvas.width = 2;
        canvas.height = 256;
        const g = canvas.getContext('2d');
        const grad = g.createLinearGradient(0, 0, 0, 256);
        if (state.lightPreview) {
            grad.addColorStop(0, '#fcfeff');
            grad.addColorStop(0.55, '#eef3f7');
            grad.addColorStop(1, '#dce6ee');
        } else {
            grad.addColorStop(0, '#101d2d');
            grad.addColorStop(0.55, '#0a1420');
            grad.addColorStop(1, '#050b12');
        }
        g.fillStyle = grad;
        g.fillRect(0, 0, 2, 256);
        return new THREE.CanvasTexture(canvas);
    }

    function resizePreviewRenderer(state, mount) {
        const p3d = state.preview3D;
        if (!p3d || !p3d.renderer || !mount) return;
        const w = Math.round(mount.clientWidth || 0);
        const h = Math.round(mount.clientHeight || 0);
        if (!w || !h) return;
        if (p3d.lastWidth === w && p3d.lastHeight === h) return;
        p3d.lastWidth = w;
        p3d.lastHeight = h;
        p3d.renderer.setSize(w, h);
        const aspect = w / Math.max(1, h);
        if (p3d.camera.isPerspectiveCamera) {
            p3d.camera.aspect = aspect;
            p3d.camera.updateProjectionMatrix();
        } else if (p3d.camera.isOrthographicCamera) {
            const halfH = p3d.camera.top;
            const halfW = halfH * aspect;
            p3d.camera.left = -halfW;
            p3d.camera.right = halfW;
            p3d.camera.updateProjectionMatrix();
        }
    }

    function renderSTL(state, mount, url) {
        if (!state.ctx || typeof state.ctx.api !== 'function') return;
        if (!window.THREE || !window.THREE.STLLoader) return;
        cleanupPreview(state);
        const width = mount.clientWidth || 400;
        const height = mount.clientHeight || 400;
        const scene = new THREE.Scene();
        const bgTexture = previewBackgroundTexture(state);
        scene.background = bgTexture;
        const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: false });
        renderer.setSize(width, height);
        renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
        renderer.shadowMap.enabled = true;
        renderer.shadowMap.type = THREE.PCFSoftShadowMap;
        if (THREE.sRGBEncoding !== undefined) renderer.outputEncoding = THREE.sRGBEncoding;
        if (THREE.ACESFilmicToneMapping !== undefined) {
            renderer.toneMapping = THREE.ACESFilmicToneMapping;
            renderer.toneMappingExposure = 1.0;
        }
        mount.appendChild(renderer.domElement);
        const aspect = width / Math.max(1, height);
        let camera;
        if (state.viewportProjection === 'orthographic') {
            camera = new THREE.OrthographicCamera(-200 * aspect, 200 * aspect, 200, -200, 1, 20000);
        } else {
            camera = new THREE.PerspectiveCamera(30, aspect, 1, 8000);
        }
        camera.position.set(80, 60, 80);
        camera.lookAt(0, 0, 0);
        const controls = new THREE.OrbitControls(camera, renderer.domElement);
        controls.target.set(0, 0, 0);
        controls.enableDamping = true;
        controls.dampingFactor = 0.09;
        controls.autoRotate = !!state.viewportAutoRotate && !prefersReducedMotion();
        controls.autoRotateSpeed = 2.2;
        controls.update();
        const onDblClick = () => resetPreviewView(state);
        renderer.domElement.addEventListener('dblclick', onDblClick);
        const hemi = new THREE.HemisphereLight(0xbdd7ff, 0x2a241a, 0.45);
        scene.add(hemi);
        const key = new THREE.DirectionalLight(0xfff1dd, 1.55);
        key.position.set(60, 90, 40);
        key.castShadow = true;
        key.shadow.mapSize.set(2048, 2048);
        key.shadow.bias = -0.0004;
        scene.add(key);
        scene.add(key.target);
        const fill = new THREE.DirectionalLight(0x9fc4ff, 0.4);
        fill.position.set(-50, 30, -40);
        scene.add(fill);
        const rim = new THREE.DirectionalLight(0x5eead4, 0.32);
        rim.position.set(-20, 40, 70);
        scene.add(rim);
        let gridHelper = new THREE.GridHelper(500, 25, 0x42d7c8, 0x42d7c8);
        gridHelper.material.transparent = true;
        gridHelper.material.opacity = 0.22;
        scene.add(gridHelper);
        if (!state.showAxes) gridHelper.visible = false;
        state.preview3D = { scene, camera, renderer, controls, gridHelper, mesh: null, onDblClick, bgTexture, keyLight: key, shadowPlane: null, resizeObserver: null, lastWidth: width, lastHeight: height };
        if (window.ResizeObserver) {
            const resizeObserver = new ResizeObserver(() => resizePreviewRenderer(state, mount));
            resizeObserver.observe(mount);
            state.preview3D.resizeObserver = resizeObserver;
        }
        const loader = new THREE.STLLoader();
        state.stl = loader;
        fetch(url).then(res => {
            if (!res.ok) throw new Error(res.statusText || String(res.status));
            return res.arrayBuffer();
        }).then(buffer => {
            if (!state.preview3D) return;
            const geometry = loader.parse(buffer);
            if (!geometry) return;
            geometry.computeVertexNormals();
            const material = new THREE.MeshStandardMaterial({
                color: state.lightPreview ? 0x5f87a8 : 0x2dd4bf,
                roughness: 0.38,
                metalness: 0.18,
                flatShading: false,
                transparent: false,
                wireframe: state.viewportShading === 'wireframe'
            });
            const mesh = new THREE.Mesh(geometry, material);
            mesh.rotation.x = -Math.PI / 2;
            mesh.castShadow = true;
            scene.add(mesh);
            state.preview3D.mesh = mesh;
            const box = new THREE.Box3().setFromObject(mesh);
            const size = box.getSize(new THREE.Vector3()).length() || 80;
            const center = box.getCenter(new THREE.Vector3());
            const floorY = box.min.y;
            scene.remove(gridHelper);
            gridHelper.geometry.dispose();
            gridHelper.material.dispose();
            gridHelper = new THREE.GridHelper(Math.max(10, size * 3.2), 32, 0x42d7c8, 0x3b6f74);
            gridHelper.material.transparent = true;
            gridHelper.material.opacity = 0.22;
            gridHelper.position.set(center.x, floorY - size * 0.002, center.z);
            gridHelper.visible = state.showAxes !== false;
            scene.add(gridHelper);
            state.preview3D.gridHelper = gridHelper;
            const shadowPlane = new THREE.Mesh(
                new THREE.PlaneGeometry(size * 4, size * 4),
                new THREE.ShadowMaterial({ color: 0x000000, opacity: state.lightPreview ? 0.16 : 0.3 })
            );
            shadowPlane.rotation.x = -Math.PI / 2;
            shadowPlane.position.set(center.x, floorY - size * 0.004, center.z);
            shadowPlane.receiveShadow = true;
            scene.add(shadowPlane);
            state.preview3D.shadowPlane = shadowPlane;
            key.position.set(center.x + size * 0.9, floorY + size * 1.25, center.z + size * 0.6);
            key.target.position.copy(center);
            key.shadow.camera.left = -size;
            key.shadow.camera.right = size;
            key.shadow.camera.top = size;
            key.shadow.camera.bottom = -size;
            key.shadow.camera.near = Math.max(0.1, size * 0.1);
            key.shadow.camera.far = size * 4;
            key.shadow.camera.updateProjectionMatrix();
            framePreviewCamera(state, state.preview3D, box);
            animatePreview(state);
        }).catch(err => {
            if (mount) {
                mount.innerHTML = `<div class="oscad-empty"><strong>${esc(t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'))}</strong><span>${esc((err && err.message) || String(err))}</span></div>`;
            }
            setStatus(state, (err && err.message) || t(state.ctx, 'desktop.openscad.no_preview', 'Render a model to see the preview.'), true);
        });
    }

    function animatePreview(state) {
        const p3d = state.preview3D;
        if (!p3d || !p3d.renderer || !p3d.scene || !p3d.camera) return;
        p3d.controls.update();
        p3d.renderer.render(p3d.scene, p3d.camera);
        if (!p3d.renderer.disposed) {
            p3d.animId = requestAnimationFrame(() => animatePreview(state));
        }
    }

    function dispose(windowId) {
        const state = stateByWindow.get(windowId);
        if (!state) return;
        persistOpenSCADDraft(state);
        if (state.draftSaveTimer) {
            window.clearTimeout(state.draftSaveTimer);
            state.draftSaveTimer = null;
        }
        if (state.busyTicker) {
            window.clearInterval(state.busyTicker);
            state.busyTicker = null;
        }
        if (state.editor && typeof state.editor.dispose === 'function') {
            try { state.editor.dispose(); } catch (_) {}
            state.editor = null;
        }
        if (state.renderAbort) { state.renderAbort.abort(); state.renderAbort = null; }
        if (state.exportAbort) { state.exportAbort.abort(); state.exportAbort = null; }
        if (state.agentAbort) { state.agentAbort.abort(); state.agentAbort = null; }
        clearWindowMenus(state);
        cleanupPreview(state);
        state.listeners.forEach(fn => { try { fn(); } catch (_) {} });
        stateByWindow.delete(windowId);
    }

    window.OpenSCADApp = { render, dispose };
})();