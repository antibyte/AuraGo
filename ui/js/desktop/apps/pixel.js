(function () {
    'use strict';

    const instances = new Map();
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    let pointerScopeTracked = false;

    // A pointer press anywhere on the page decides which Pixel window gets keyboard shortcuts: the window it hit
    // (canvas, panels or its own title bar), or none when it hit something else. One listener serves all windows.
    function trackPointerScope() {
        if (pointerScopeTracked) return;
        pointerScopeTracked = true;
        document.addEventListener('pointerdown', e => {
            const target = e.target;
            let hit = null;
            if (target && target.closest) {
                instances.forEach(entry => {
                    if (hit || !entry.host || entry.disposed) return;
                    const frame = entry.host.closest ? entry.host.closest('.vd-window') : null;
                    if (entry.host.contains(target) || (frame && frame.contains(target))) hit = entry;
                });
            }
            if (hit) { hit.lastActive = Date.now(); Pixel.activeInstance = hit; } else Pixel.activeInstance = null;
        }, true);
    }

    function render(host, windowId, context) {
        if (!host) return;
        const state = { disposed: false, lastActive: 0, host };
        instances.set(windowId, state);
        trackPointerScope();
        const ctx = context || {};
        const esc = ctx.esc || (v => String(v == null ? '' : v));
        const t = ctx.t || ((k, f) => f || k);
        const api = ctx.api || fetchJSON;
        const iconMarkup = ctx.iconMarkup || ((k, f) => `<span>${esc(f || k || '')}</span>`);
        const notify = ctx.notify || (() => {});
        const fileOps = ctx.fileOps || window.AuraDesktopFileOps || null;
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        let originalImage = null;
        let canvas, cctx, overlayCanvas, olCtx, canvasWrap;
        let history = [];
        let historyIdx = -1;
        let zoom = 1;
        let filePath = ctx.path || '';
        let fileName = filePath ? filePath.split('/').pop() : '';
        let isDirty = false;
        let activePanel = 'adjust';
        let cropState = null;
        let cropOverlay = null;
        let abortCtrl = null;
        let imgWidth = 0, imgHeight = 0;

        let activeTool = null;
        let primaryColor = '#ffffff';
        let secondaryColor = '#000000';
        let recentColors = [];
        let brushSize = 5;
        let brushOpacity = 100;
        let brushHardness = 100;
        let shapeFill = false;
        let shapeStrokeWidth = 2;
        let fontSize = 24;
        let fontFamily = 'Arial';
        let fillTolerance = 32;
        let sprayDensity = 25;
        let dodgeMode = 'dodge';
        let gradientMode = 'linear';
        let isDrawing = false;
        let drawStartX = 0, drawStartY = 0;
        let lastDrawX = 0, lastDrawY = 0;

        let selection = null;
        let selImageData = null;
        let isMovingSel = false;
        let selDragOfsX = 0, selDragOfsY = 0;
        let marchingOfs = 0;
        let marchingRAF = null;

        let isPanning = false;
        let spaceHeld = false;
        let panStartX = 0, panStartY = 0;
        let panStartSL = 0, panStartST = 0;

        let layers = [];
        let activeLayerIdx = 0;

        let compareMode = false;
        let comparePos = 0.5;
        let compareOrigData = null;

        let aiConfigured = false;

        let activeFilterId = null;
        let activeFilterCat = 'all';
        let filterStrength = 100;
        let filterPreview = null;
        let filterThumbKey = '';
        let filterCompareMode = false;
        let filterFavorites = [];

        let cloneSourceX = null;
        let cloneSourceY = null;
        let cloneStrokeOffX = 0;
        let cloneStrokeOffY = 0;
        let cloneSampleCanvas = null;
        let cloneStampUsed = false;
        let lassoPath = null;
        let editMaskMode = false;
        let moveLayerSnapshot = null;
        let moveLayerStartX = 0;
        let moveLayerStartY = 0;
        let moveSelFloating = false;
        let moveSelOrigX = 0;
        let moveSelOrigY = 0;

        const adjustments = { brightness: 0, contrast: 0, saturation: 0, exposure: 0, sharpness: 0, temperature: 0, shadows: 0, highlights: 0 };


        const runtime = {};
        Object.defineProperties(runtime, {
            host: { get: () => host, set: value => { host = value; }, enumerable: true },
            windowId: { get: () => windowId, set: value => { windowId = value; }, enumerable: true },
            context: { get: () => context, set: value => { context = value; }, enumerable: true },
            state: { get: () => state, set: value => { state = value; }, enumerable: true },
            ctx: { get: () => ctx, set: value => { ctx = value; }, enumerable: true },
            esc: { get: () => esc, set: value => { esc = value; }, enumerable: true },
            t: { get: () => t, set: value => { t = value; }, enumerable: true },
            api: { get: () => api, set: value => { api = value; }, enumerable: true },
            iconMarkup: { get: () => iconMarkup, set: value => { iconMarkup = value; }, enumerable: true },
            notify: { get: () => notify, set: value => { notify = value; }, enumerable: true },
            fileOps: { get: () => fileOps, set: value => { fileOps = value; }, enumerable: true },
            originalImage: { get: () => originalImage, set: value => { originalImage = value; }, enumerable: true },
            canvas: { get: () => canvas, set: value => { canvas = value; }, enumerable: true },
            cctx: { get: () => cctx, set: value => { cctx = value; }, enumerable: true },
            overlayCanvas: { get: () => overlayCanvas, set: value => { overlayCanvas = value; }, enumerable: true },
            olCtx: { get: () => olCtx, set: value => { olCtx = value; }, enumerable: true },
            canvasWrap: { get: () => canvasWrap, set: value => { canvasWrap = value; }, enumerable: true },
            history: { get: () => history, set: value => { history = value; }, enumerable: true },
            historyIdx: { get: () => historyIdx, set: value => { historyIdx = value; }, enumerable: true },
            zoom: { get: () => zoom, set: value => { zoom = value; }, enumerable: true },
            filePath: { get: () => filePath, set: value => { filePath = value; }, enumerable: true },
            fileName: { get: () => fileName, set: value => { fileName = value; }, enumerable: true },
            isDirty: { get: () => isDirty, set: value => { isDirty = value; }, enumerable: true },
            activePanel: { get: () => activePanel, set: value => { activePanel = value; }, enumerable: true },
            cropState: { get: () => cropState, set: value => { cropState = value; }, enumerable: true },
            cropOverlay: { get: () => cropOverlay, set: value => { cropOverlay = value; }, enumerable: true },
            abortCtrl: { get: () => abortCtrl, set: value => { abortCtrl = value; }, enumerable: true },
            imgWidth: { get: () => imgWidth, set: value => { imgWidth = value; }, enumerable: true },
            imgHeight: { get: () => imgHeight, set: value => { imgHeight = value; }, enumerable: true },
            activeTool: { get: () => activeTool, set: value => { activeTool = value; }, enumerable: true },
            primaryColor: { get: () => primaryColor, set: value => { primaryColor = value; }, enumerable: true },
            secondaryColor: { get: () => secondaryColor, set: value => { secondaryColor = value; }, enumerable: true },
            recentColors: { get: () => recentColors, set: value => { recentColors = value; }, enumerable: true },
            brushSize: { get: () => brushSize, set: value => { brushSize = value; }, enumerable: true },
            brushOpacity: { get: () => brushOpacity, set: value => { brushOpacity = value; }, enumerable: true },
            brushHardness: { get: () => brushHardness, set: value => { brushHardness = value; }, enumerable: true },
            shapeFill: { get: () => shapeFill, set: value => { shapeFill = value; }, enumerable: true },
            shapeStrokeWidth: { get: () => shapeStrokeWidth, set: value => { shapeStrokeWidth = value; }, enumerable: true },
            fontSize: { get: () => fontSize, set: value => { fontSize = value; }, enumerable: true },
            fontFamily: { get: () => fontFamily, set: value => { fontFamily = value; }, enumerable: true },
            fillTolerance: { get: () => fillTolerance, set: value => { fillTolerance = value; }, enumerable: true },
            sprayDensity: { get: () => sprayDensity, set: value => { sprayDensity = value; }, enumerable: true },
            dodgeMode: { get: () => dodgeMode, set: value => { dodgeMode = value; }, enumerable: true },
            gradientMode: { get: () => gradientMode, set: value => { gradientMode = value; }, enumerable: true },
            isDrawing: { get: () => isDrawing, set: value => { isDrawing = value; }, enumerable: true },
            drawStartX: { get: () => drawStartX, set: value => { drawStartX = value; }, enumerable: true },
            drawStartY: { get: () => drawStartY, set: value => { drawStartY = value; }, enumerable: true },
            lastDrawX: { get: () => lastDrawX, set: value => { lastDrawX = value; }, enumerable: true },
            lastDrawY: { get: () => lastDrawY, set: value => { lastDrawY = value; }, enumerable: true },
            selection: { get: () => selection, set: value => { selection = value; runtime.selectionRev++; runtime.emit('selection-changed', { selection: value }); }, enumerable: true },
            selImageData: { get: () => selImageData, set: value => { selImageData = value; }, enumerable: true },
            isMovingSel: { get: () => isMovingSel, set: value => { isMovingSel = value; }, enumerable: true },
            selDragOfsX: { get: () => selDragOfsX, set: value => { selDragOfsX = value; }, enumerable: true },
            selDragOfsY: { get: () => selDragOfsY, set: value => { selDragOfsY = value; }, enumerable: true },
            marchingOfs: { get: () => marchingOfs, set: value => { marchingOfs = value; }, enumerable: true },
            // Holds the timer of the next marching-ants frame (see scheduleMarchingAnts).
            marchingRAF: { get: () => marchingRAF, set: value => { marchingRAF = value; }, enumerable: true },
            isPanning: { get: () => isPanning, set: value => { isPanning = value; }, enumerable: true },
            spaceHeld: { get: () => spaceHeld, set: value => { spaceHeld = value; }, enumerable: true },
            panStartX: { get: () => panStartX, set: value => { panStartX = value; }, enumerable: true },
            panStartY: { get: () => panStartY, set: value => { panStartY = value; }, enumerable: true },
            panStartSL: { get: () => panStartSL, set: value => { panStartSL = value; }, enumerable: true },
            panStartST: { get: () => panStartST, set: value => { panStartST = value; }, enumerable: true },
            layers: { get: () => layers, set: value => { layers = value; }, enumerable: true },
            activeLayerIdx: { get: () => activeLayerIdx, set: value => { const changed = value !== activeLayerIdx; activeLayerIdx = value; if (changed) runtime.syncActiveLayer(); }, enumerable: true },
            compareMode: { get: () => compareMode, set: value => { compareMode = value; }, enumerable: true },
            comparePos: { get: () => comparePos, set: value => { comparePos = value; }, enumerable: true },
            compareOrigData: { get: () => compareOrigData, set: value => { compareOrigData = value; }, enumerable: true },
            aiConfigured: { get: () => aiConfigured, set: value => { aiConfigured = value; }, enumerable: true },
            activeFilterId: { get: () => activeFilterId, set: value => { activeFilterId = value; }, enumerable: true },
            activeFilterCat: { get: () => activeFilterCat, set: value => { activeFilterCat = value; }, enumerable: true },
            filterStrength: { get: () => filterStrength, set: value => { filterStrength = value; }, enumerable: true },
            filterPreview: { get: () => filterPreview, set: value => { filterPreview = value; }, enumerable: true },
            filterThumbKey: { get: () => filterThumbKey, set: value => { filterThumbKey = value; }, enumerable: true },
            filterCompareMode: { get: () => filterCompareMode, set: value => { filterCompareMode = value; }, enumerable: true },
            filterFavorites: { get: () => filterFavorites, set: value => { filterFavorites = value; }, enumerable: true },
            cloneSourceX: { get: () => cloneSourceX, set: value => { cloneSourceX = value; }, enumerable: true },
            cloneSourceY: { get: () => cloneSourceY, set: value => { cloneSourceY = value; }, enumerable: true },
            cloneStrokeOffX: { get: () => cloneStrokeOffX, set: value => { cloneStrokeOffX = value; }, enumerable: true },
            cloneStrokeOffY: { get: () => cloneStrokeOffY, set: value => { cloneStrokeOffY = value; }, enumerable: true },
            cloneSampleCanvas: { get: () => cloneSampleCanvas, set: value => { cloneSampleCanvas = value; }, enumerable: true },
            cloneStampUsed: { get: () => cloneStampUsed, set: value => { cloneStampUsed = value; }, enumerable: true },
            lassoPath: { get: () => lassoPath, set: value => { lassoPath = value; }, enumerable: true },
            editMaskMode: { get: () => editMaskMode, set: value => { editMaskMode = value; }, enumerable: true },
            moveLayerSnapshot: { get: () => moveLayerSnapshot, set: value => { moveLayerSnapshot = value; }, enumerable: true },
            moveLayerStartX: { get: () => moveLayerStartX, set: value => { moveLayerStartX = value; }, enumerable: true },
            moveLayerStartY: { get: () => moveLayerStartY, set: value => { moveLayerStartY = value; }, enumerable: true },
            moveSelFloating: { get: () => moveSelFloating, set: value => { moveSelFloating = value; }, enumerable: true },
            moveSelOrigX: { get: () => moveSelOrigX, set: value => { moveSelOrigX = value; }, enumerable: true },
            moveSelOrigY: { get: () => moveSelOrigY, set: value => { moveSelOrigY = value; }, enumerable: true },
            adjustments: { get: () => adjustments, set: value => { adjustments = value; }, enumerable: true },
            appEl: { get: () => appEl, set: value => { appEl = value; }, enumerable: true },
            canvasArea: { get: () => canvasArea, set: value => { canvasArea = value; }, enumerable: true },
            emptyState: { get: () => emptyState, set: value => { emptyState = value; }, enumerable: true },
            statusText: { get: () => statusText, set: value => { statusText = value; }, enumerable: true },
            statusCursor: { get: () => statusCursor, set: value => { statusCursor = value; }, enumerable: true },
            statusColor: { get: () => statusColor, set: value => { statusColor = value; }, enumerable: true },
            statusColorSwatch: { get: () => statusColorSwatch, set: value => { statusColorSwatch = value; }, enumerable: true },
            statusColorHex: { get: () => statusColorHex, set: value => { statusColorHex = value; }, enumerable: true },
            statusTool: { get: () => statusTool, set: value => { statusTool = value; }, enumerable: true },
            panelContainer: { get: () => panelContainer, set: value => { panelContainer = value; }, enumerable: true },
            optionsBar: { get: () => optionsBar, set: value => { optionsBar = value; }, enumerable: true },
            toolRail: { get: () => toolRail, set: value => { toolRail = value; }, enumerable: true },
            zoomLabel: { get: () => zoomLabel, set: value => { zoomLabel = value; }, enumerable: true },
            compareDivider: { get: () => compareDivider, set: value => { compareDivider = value; }, enumerable: true },
            primarySwatch: { get: () => primarySwatch, set: value => { primarySwatch = value; }, enumerable: true },
            secondarySwatch: { get: () => secondarySwatch, set: value => { secondarySwatch = value; }, enumerable: true },
            hexInput: { get: () => hexInput, set: value => { hexInput = value; }, enumerable: true },
            strengthSlider: { get: () => strengthSlider, set: value => { strengthSlider = value; }, enumerable: true },
            MAX_HISTORY: { get: () => Pixel.MAX_HISTORY, enumerable: true },
            MAX_LAYERS: { get: () => Pixel.MAX_LAYERS, enumerable: true },
            HISTORY_BYTE_BUDGET: { get: () => Pixel.HISTORY_BYTE_BUDGET, enumerable: true },
            BLEND_MODES: { get: () => Pixel.BLEND_MODES, enumerable: true },
            IMAGE_EXTS: { get: () => Pixel.IMAGE_EXTS, enumerable: true },
            PRESET_COLORS: { get: () => Pixel.PRESET_COLORS, enumerable: true },
            FONT_FAMILIES: { get: () => Pixel.FONT_FAMILIES, enumerable: true },
            TOOL_SVGS: { get: () => Pixel.TOOL_SVGS, enumerable: true },
            acquireTempCanvas: { get: () => Pixel.acquireTempCanvas, enumerable: true },
            releaseTempCanvas: { get: () => Pixel.releaseTempCanvas, enumerable: true },
            clamp255: { get: () => Pixel.clamp255, enumerable: true },
            hexToRgb: { get: () => Pixel.hexToRgb, enumerable: true },
            rgbToHex: { get: () => Pixel.rgbToHex, enumerable: true },
            colorDist: { get: () => Pixel.colorDist, enumerable: true }
        });
        // Built-in installers first, then every module registered with Pixel.registerModule in load order, so a feature
        // module can extend or replace what the built-ins put on the runtime without a change to this file.
        Pixel.installCore(runtime);
        Pixel.installView(runtime);
        Pixel.installCanvas(runtime);
        Pixel.installTools(runtime);
        Pixel.installActions(runtime);
        Pixel.installFilters(runtime);
        Pixel.installEvents(runtime);
        Pixel.installStickers(runtime);
        Pixel.listModules().forEach(entry => {
            try {
                entry.install(runtime);
            } catch (err) {
                if (window.console) console.error('pixel module ' + entry.id, err);
            }
        });
        Object.assign(runtime, { gesture: null, cursorPos: null, menusDynamic: false });
        const {
            buildToolRailHTML,
            buildDocksHTML,
            pushHistory,
            undo,
            redo,
            zoomTo,
            zoomFit,
            newBlankCanvas,
            showNewImageDialog,
            loadDesktopImagePath,
            loadPhotos,
            openFile,
            saveFile,
            exportFile,
            stopMarchingAnts,
            deselect,
            onCanvasMouseDown,
            onCanvasMouseMove,
            onCanvasMouseUp,
            onCanvasLeave,
            onCropMouseDown,
            showContextMenu,
            renderRecentFiles,
            checkAIConfig
        } = runtime;
        const toolbarIcon = (key, fallback) => iconMarkup(key, fallback, 'pixel-toolbar-icon', 16);
        host.innerHTML = `<div class="pixel-app">
            <div class="pixel-toolbar">
                <button class="pixel-btn-icon" type="button" data-action="open" title="${esc(t('pixel.open'))} (Ctrl+O)">${toolbarIcon('folder-open', 'O')}</button>
                <button class="pixel-btn-icon" type="button" data-action="save" title="${esc(t('pixel.save'))} (Ctrl+S)">${toolbarIcon('save', 'S')}</button>
                <button class="pixel-btn-icon" type="button" data-action="export" title="${esc(t('pixel.export'))}">${toolbarIcon('download', 'E')}</button>
                <span class="pixel-toolbar-sep"></span>
                <button class="pixel-btn-icon" type="button" data-action="undo" title="${esc(t('pixel.undo'))} (Ctrl+Z)">${toolbarIcon('undo', 'U')}</button>
                <button class="pixel-btn-icon" type="button" data-action="redo" title="${esc(t('pixel.redo'))} (Ctrl+Shift+Z)">${toolbarIcon('redo', 'R')}</button>
                <span class="pixel-toolbar-sep"></span>
                <button class="pixel-btn-icon" type="button" data-action="zoom-out" title="${esc(t('pixel.zoom_out'))}">${toolbarIcon('zoom-out', '-')}</button>
                <span class="pixel-zoom-label" data-zoom-label>100%</span>
                <button class="pixel-btn-icon" type="button" data-action="zoom-in" title="${esc(t('pixel.zoom_in'))}">${toolbarIcon('zoom-in', '+')}</button>
                <button class="pixel-btn-icon" type="button" data-action="zoom-fit" title="${esc(t('pixel.zoom_fit'))}">${toolbarIcon('maximize', 'F')}</button>
            </div>
            <div class="pixel-options-bar" data-options-bar hidden></div>
            <div class="pixel-workspace">
                ${buildToolRailHTML()}
                <div class="pixel-canvas-area" data-canvas-area>
                    <div class="pixel-canvas-wrap" data-canvas-wrap hidden>
                        <canvas class="pixel-canvas" data-canvas></canvas>
                        <canvas class="pixel-overlay" data-overlay></canvas>
                    </div>
                    <div class="pixel-crop-overlay" data-crop-overlay hidden></div>
                    <div class="pixel-compare-divider" data-compare-divider hidden></div>
                    <div class="pixel-empty-state" data-empty>
                        ${iconMarkup('image', 'Img', 'pixel-empty-icon', 64)}
                        <p>${esc(t('pixel.no_image'))}</p>
                        <div class="pixel-empty-actions">
                            <button class="pixel-btn pixel-btn-primary" type="button" data-action="open">${esc(t('pixel.open'))}</button>
                            <button class="pixel-btn" type="button" data-action="new-image">${esc(t('pixel.new_image'))}</button>
                        </div>
                        <div class="pixel-templates" data-templates>
                            <span class="pixel-label">${esc(t('pixel.quick_start'))}</span>
                            <div class="pixel-template-grid">
                                <button class="pixel-template-btn" type="button" data-template="512x512">512×512</button>
                                <button class="pixel-template-btn" type="button" data-template="1024x1024">1024×1024</button>
                                <button class="pixel-template-btn" type="button" data-template="1920x1080">1920×1080</button>
                            </div>
                        </div>
                        <div class="pixel-recent-files" data-recent-files></div>
                        <div class="pixel-photos-section" data-photos-section>
                            <span class="pixel-label">${esc(t('pixel.photos'))}</span>
                            <div class="pixel-photos-grid" data-photos-grid></div>
                        </div>
                    </div>
                </div>
                <div class="pixel-panel" data-panel-container>${buildDocksHTML()}</div>
            </div>
            <div class="pixel-status-bar" data-status>
                <span data-status-text>${esc(t('pixel.status_ready'))}</span>
                <span class="pixel-status-cursor" data-status-cursor></span>
                <span class="pixel-status-color" data-status-color><span class="pixel-status-color-swatch" data-color-swatch></span><span data-color-hex></span></span>
                <span class="pixel-status-tool" data-status-tool></span>
            </div>
        </div>`;

        const appEl = host.querySelector('.pixel-app');
        canvas = host.querySelector('[data-canvas]');
        cctx = canvas.getContext('2d', { willReadFrequently: true });
        overlayCanvas = host.querySelector('[data-overlay]');
        olCtx = overlayCanvas.getContext('2d');
        canvasWrap = host.querySelector('[data-canvas-wrap]');
        const canvasArea = host.querySelector('[data-canvas-area]');
        const emptyState = host.querySelector('[data-empty]');
        const statusText = host.querySelector('[data-status-text]');
        const statusCursor = host.querySelector('[data-status-cursor]');
        const statusColor = host.querySelector('[data-status-color]');
        const statusColorSwatch = host.querySelector('[data-color-swatch]');
        const statusColorHex = host.querySelector('[data-color-hex]');
        const statusTool = host.querySelector('[data-status-tool]');
        const panelContainer = host.querySelector('[data-panel-container]');
        const optionsBar = host.querySelector('[data-options-bar]');
        const toolRail = host.querySelector('[data-tool-rail]');
        const zoomLabel = host.querySelector('[data-zoom-label]');
        cropOverlay = host.querySelector('[data-crop-overlay]');
        const compareDivider = host.querySelector('[data-compare-divider]');

        layers = [runtime.createLayer({ name: t('pixel.layer_background') })];
        activeLayerIdx = 0;

        function wireClick(selector, handler) {
            host.querySelectorAll(selector).forEach(el => el.addEventListener('click', handler));
        }

        // Toolbar and empty-state actions; every panel wires its own section in its mount(), see pixel-view.js.
        wireClick('[data-action="open"]', openFile);
        wireClick('[data-action="save"]', saveFile);
        wireClick('[data-action="export"]', exportFile);
        wireClick('[data-action="undo"]', undo);
        wireClick('[data-action="redo"]', redo);
        wireClick('[data-action="zoom-in"]', () => zoomTo(zoom * 1.25));
        wireClick('[data-action="zoom-out"]', () => zoomTo(zoom / 1.25));
        wireClick('[data-action="zoom-fit"]', zoomFit);
        wireClick('[data-action="new-image"]', showNewImageDialog);
        runtime.mountToolRail();
        runtime.mountPanels();
        runtime.initDocks();

        host.querySelectorAll('[data-template]').forEach(btn => {
            btn.addEventListener('click', () => {
                const [w, h] = btn.dataset.template.split('x').map(Number);
                newBlankCanvas(w, h);
            });
        });

        host.addEventListener('click', e => {
            const recentBtn = e.target.closest('[data-recent-path]');
            if (recentBtn) {
                const path = recentBtn.dataset.recentPath;
                loadDesktopImagePath(path).catch(() => {});
            }
            const photoBtn = e.target.closest('[data-photo-path]');
            if (photoBtn) {
                const path = photoBtn.dataset.photoPath;
                loadDesktopImagePath(path).catch(() => {});
            }
            const clearBtn = e.target.closest('[data-action="clear-selection"]');
            if (clearBtn) { deselect(); }
        });

        state._cropMouseDown = onCropMouseDown;
        cropOverlay.addEventListener('mousedown', state._cropMouseDown);

        // Pointer events with capture: a stroke that leaves the canvas keeps going until the button is released.
        state._pointerDown = e => {
            if (spaceHeld) return;
            if (e.button === 0) {
                try { overlayCanvas.setPointerCapture(e.pointerId); } catch (_) { /* synthetic pointer */ }
            }
            onCanvasMouseDown(e);
        };
        state._pointerUp = e => {
            try {
                if (overlayCanvas.hasPointerCapture && overlayCanvas.hasPointerCapture(e.pointerId)) overlayCanvas.releasePointerCapture(e.pointerId);
            } catch (_) { /* already released */ }
            onCanvasMouseUp(e);
        };
        state._pointerLeave = e => {
            onCanvasLeave(e);
            const captured = overlayCanvas.hasPointerCapture && overlayCanvas.hasPointerCapture(e.pointerId);
            if (!captured && isDrawing && activeTool && ['brush', 'eraser', 'pencil'].includes(activeTool)) { isDrawing = false; pushHistory('draw:' + activeTool); }
        };
        overlayCanvas.addEventListener('pointerdown', state._pointerDown);
        overlayCanvas.addEventListener('pointermove', onCanvasMouseMove);
        overlayCanvas.addEventListener('pointerup', state._pointerUp);
        overlayCanvas.addEventListener('pointercancel', state._pointerUp);
        overlayCanvas.addEventListener('pointerleave', state._pointerLeave);

        canvasArea.addEventListener('contextmenu', showContextMenu);

        // Scroll-wheel zoom
        canvasArea.addEventListener('wheel', e => {
            if (e.ctrlKey || e.metaKey) {
                e.preventDefault();
                const delta = e.deltaY > 0 ? 0.9 : 1.1;
                zoomTo(zoom * delta);
            }
        }, { passive: false });

        // Space+drag pan
        canvasArea.addEventListener('mousedown', e => {
            if (spaceHeld && e.button === 0) {
                isPanning = true;
                panStartX = e.clientX;
                panStartY = e.clientY;
                panStartSL = canvasArea.scrollLeft;
                panStartST = canvasArea.scrollTop;
                canvasArea.style.cursor = 'grabbing';
                e.preventDefault();
            }
        });

        state._panMove = e => {
            if (isPanning) {
                canvasArea.scrollLeft = panStartSL - (e.clientX - panStartX);
                canvasArea.scrollTop = panStartST - (e.clientY - panStartY);
            }
        };
        state._panUp = () => {
            if (isPanning) {
                isPanning = false;
                canvasArea.style.cursor = spaceHeld ? 'grab' : '';
            }
        };
        document.addEventListener('mousemove', state._panMove);
        document.addEventListener('mouseup', state._panUp);

        // Double-click to fit
        canvasArea.addEventListener('dblclick', () => { if (imgWidth) zoomFit(); });

        // Keyboard: one listener per window; handleKeydown ignores keys unless this is the active Pixel window and
        // dispatches shortcuts through the command registry (menu items, tools, panels, registered commands).
        state._keydown = e => runtime.handleKeydown(e);
        state._keyup = e => runtime.handleKeyup(e);
        document.addEventListener('keydown', state._keydown);
        document.addEventListener('keyup', state._keyup);
        const markActive = () => runtime.markActiveWindow();
        host.addEventListener('pointerdown', markActive, true);
        host.addEventListener('pointerenter', markActive);
        host.addEventListener('focusin', markActive);
        markActive();

        // Drag-drop support
        appEl.addEventListener('dragover', e => {
            if (!e.dataTransfer) return;
            const hasFileDrag = fileOps && typeof fileOps.hasDragPayload === 'function'
                ? fileOps.hasDragPayload(e)
                : Array.from(e.dataTransfer.types || []).includes('application/x-aurago-desktop-files');
            const hasPlainFile = e.dataTransfer.types.includes('Files');
            if (hasFileDrag || hasPlainFile) { e.preventDefault(); e.dataTransfer.dropEffect = 'copy'; appEl.classList.add('pixel-drop-target'); }
        });
        appEl.addEventListener('dragleave', e => { if (e.currentTarget === e.target || !appEl.contains(e.relatedTarget)) appEl.classList.remove('pixel-drop-target'); });
        appEl.addEventListener('drop', e => {
            appEl.classList.remove('pixel-drop-target');
            e.preventDefault();
            e.stopPropagation();
            let paths = [];
            const payload = fileOps && typeof fileOps.readDragPayload === 'function' ? fileOps.readDragPayload(e) : null;
            if (payload && Array.isArray(payload.paths)) paths = payload.paths;
            if (!paths.length) { const text = e.dataTransfer.getData('text/plain'); if (text) paths = [text]; }
            const imgPath = paths.find(p => runtime.IMAGE_EXTS.some(ext => p.toLowerCase().endsWith('.' + ext)));
            if (imgPath) {
                loadDesktopImagePath(imgPath).catch(() => {});
            }
        });

        // Window menus come from Pixel.registerMenuItems (file, edit, image, layer, select, filter, generate, view, ai).
        runtime.rebuildMenus();

        // Module installers run before the markup exists; modules that need the finished shell wait for this event.
        runtime.emit('ready');

        // Load initial image
        if (filePath) {
            loadDesktopImagePath(filePath).catch(() => {});
        }

        renderRecentFiles();
        loadPhotos();
        checkAIConfig();

        state.dispose = function () {
            runtime.disposeStickers();
            runtime.disposeCore();
            if (runtime._filterPreviewRAF) cancelAnimationFrame(runtime._filterPreviewRAF);
            state.disposed = true;
            state.imageRead?.abort();
            if (abortCtrl) abortCtrl.abort();
            stopMarchingAnts();
            if (state._keydown) document.removeEventListener('keydown', state._keydown);
            if (state._keyup) document.removeEventListener('keyup', state._keyup);
            if (state._panMove) document.removeEventListener('mousemove', state._panMove);
            if (state._panUp) document.removeEventListener('mouseup', state._panUp);
            if (state._cropMouseDown) cropOverlay.removeEventListener('mousedown', state._cropMouseDown);
            overlayCanvas.removeEventListener('pointerdown', state._pointerDown);
            overlayCanvas.removeEventListener('pointermove', onCanvasMouseMove);
            overlayCanvas.removeEventListener('pointerup', state._pointerUp);
            overlayCanvas.removeEventListener('pointercancel', state._pointerUp);
            overlayCanvas.removeEventListener('pointerleave', state._pointerLeave);
            host.removeEventListener('pointerdown', markActive, true);
            host.removeEventListener('pointerenter', markActive);
            host.removeEventListener('focusin', markActive);
            if (Pixel.activeInstance === state) Pixel.activeInstance = null;
            history = [];
            originalImage = null;
            selImageData = null;
            layers = [];
            filterPreview = null;
            canvas = null;
            cctx = null;
            overlayCanvas = null;
            olCtx = null;
        };
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (state && typeof state.dispose === 'function') state.dispose();
        instances.delete(windowId);
    }

    async function fetchJSON(url, options) {
        const resp = await fetch(url, options);
        const body = await resp.json().catch(() => ({}));
        if (!resp.ok) throw new Error(body.error || body.message || ('HTTP ' + resp.status));
        return body;
    }

    window.PixelApp = window.PixelApp || {};
    window.PixelApp.render = render;
    window.PixelApp.dispose = dispose;
})();
