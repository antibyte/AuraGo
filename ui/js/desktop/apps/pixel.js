(function () {
    'use strict';

    const instances = new Map();
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    function render(host, windowId, context) {
        if (!host) return;
        const state = { disposed: false };
        instances.set(windowId, state);
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
            isDrawing: { get: () => isDrawing, set: value => { isDrawing = value; }, enumerable: true },
            drawStartX: { get: () => drawStartX, set: value => { drawStartX = value; }, enumerable: true },
            drawStartY: { get: () => drawStartY, set: value => { drawStartY = value; }, enumerable: true },
            lastDrawX: { get: () => lastDrawX, set: value => { lastDrawX = value; }, enumerable: true },
            lastDrawY: { get: () => lastDrawY, set: value => { lastDrawY = value; }, enumerable: true },
            selection: { get: () => selection, set: value => { selection = value; }, enumerable: true },
            selImageData: { get: () => selImageData, set: value => { selImageData = value; }, enumerable: true },
            isMovingSel: { get: () => isMovingSel, set: value => { isMovingSel = value; }, enumerable: true },
            selDragOfsX: { get: () => selDragOfsX, set: value => { selDragOfsX = value; }, enumerable: true },
            selDragOfsY: { get: () => selDragOfsY, set: value => { selDragOfsY = value; }, enumerable: true },
            marchingOfs: { get: () => marchingOfs, set: value => { marchingOfs = value; }, enumerable: true },
            marchingRAF: { get: () => marchingRAF, set: value => { marchingRAF = value; }, enumerable: true },
            isPanning: { get: () => isPanning, set: value => { isPanning = value; }, enumerable: true },
            spaceHeld: { get: () => spaceHeld, set: value => { spaceHeld = value; }, enumerable: true },
            panStartX: { get: () => panStartX, set: value => { panStartX = value; }, enumerable: true },
            panStartY: { get: () => panStartY, set: value => { panStartY = value; }, enumerable: true },
            panStartSL: { get: () => panStartSL, set: value => { panStartSL = value; }, enumerable: true },
            panStartST: { get: () => panStartST, set: value => { panStartST = value; }, enumerable: true },
            layers: { get: () => layers, set: value => { layers = value; }, enumerable: true },
            activeLayerIdx: { get: () => activeLayerIdx, set: value => { activeLayerIdx = value; }, enumerable: true },
            compareMode: { get: () => compareMode, set: value => { compareMode = value; }, enumerable: true },
            comparePos: { get: () => comparePos, set: value => { comparePos = value; }, enumerable: true },
            compareOrigData: { get: () => compareOrigData, set: value => { compareOrigData = value; }, enumerable: true },
            aiConfigured: { get: () => aiConfigured, set: value => { aiConfigured = value; }, enumerable: true },
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
            zoomLabel: { get: () => zoomLabel, set: value => { zoomLabel = value; }, enumerable: true },
            compareDivider: { get: () => compareDivider, set: value => { compareDivider = value; }, enumerable: true },
            primarySwatch: { get: () => primarySwatch, set: value => { primarySwatch = value; }, enumerable: true },
            secondarySwatch: { get: () => secondarySwatch, set: value => { secondarySwatch = value; }, enumerable: true },
            hexInput: { get: () => hexInput, set: value => { hexInput = value; }, enumerable: true },
            mainOpacitySlider: { get: () => mainOpacitySlider, set: value => { mainOpacitySlider = value; }, enumerable: true },
            strengthSlider: { get: () => strengthSlider, set: value => { strengthSlider = value; }, enumerable: true },
            MAX_HISTORY: { get: () => Pixel.MAX_HISTORY, enumerable: true },
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
        Pixel.installView(runtime);
        Pixel.installCanvas(runtime);
        Pixel.installTools(runtime);
        Pixel.installActions(runtime);
        Pixel.installEvents(runtime);
        const {
            toolSvg,
            buildDrawPanelHTML,
            buildDrawOptionsHTML,
            buildLayersPanelHTML,
            buildPanelHTML,
            buildSlider,
            buildFilterCard,
            setStatus,
            updateStatus,
            updateZoomLabel,
            canvasCoords,
            getActiveCtx,
            compositeLayers,
            pushHistory,
            restoreHistory,
            undo,
            redo,
            applyZoom,
            zoomTo,
            zoomFit,
            loadImage,
            loadImageToCanvas,
            newBlankCanvas,
            showNewImageDialog,
            loadRecentFiles,
            saveRecentFile,
            renderRecentFiles,
            loadPhotos,
            openFile,
            saveFile,
            saveFileAs,
            exportFile,
            showPanel,
            refreshLayerPanel,
            setActiveTool,
            getToolCursor,
            clearOverlay,
            drawMarchingAnts,
            startMarchingAnts,
            stopMarchingAnts,
            selectAll,
            deselect,
            copySelection,
            cutSelection,
            pasteClipboard,
            deleteSelection,
            addRecentColor,
            setPrimaryColor,
            setSecondaryColor,
            swapColors,
            drawBrushStroke,
            drawShapePreview,
            commitShape,
            floodFill,
            commitTextToCanvas,
            applyFilterPreview,
            applyVignette,
            applyEmboss,
            convolve,
            buildFilterString,
            applyAdjustmentsPreview,
            applyCustomAdjustments,
            applyAdjustments,
            resetAdjustments,
            toggleCompare,
            renderCompare,
            rotateCanvas,
            flipCanvas,
            startCrop,
            cancelCrop,
            applyCrop,
            resizeImage,
            aiGenerate,
            aiEnhance,
            checkAIConfig,
            addLayer,
            deleteLayer,
            duplicateLayer,
            mergeDown,
            flattenLayers,
            onCanvasMouseDown,
            onCanvasMouseMove,
            onCanvasMouseUp,
            showTextInput,
            onCropMouseDown,
            updateCropSelection,
            showShortcutsModal,
            showContextMenu,
            wireDrawOptionEvents
        } = runtime;
        const toolbarIcon = (key, fallback) => iconMarkup(key, fallback, 'pixel-toolbar-icon', 16);
        host.innerHTML = `<div class="pixel-app">
            <div class="pixel-toolbar">
                <button class="pixel-btn-icon" type="button" data-action="open" title="${esc(t('pixel.open', 'Open'))} (Ctrl+O)">${toolbarIcon('folder-open', 'O')}</button>
                <button class="pixel-btn-icon" type="button" data-action="save" title="${esc(t('pixel.save', 'Save'))} (Ctrl+S)">${toolbarIcon('save', 'S')}</button>
                <button class="pixel-btn-icon" type="button" data-action="export" title="${esc(t('pixel.export', 'Export'))}">${toolbarIcon('download', 'E')}</button>
                <span class="pixel-toolbar-sep"></span>
                <button class="pixel-btn-icon" type="button" data-action="undo" title="${esc(t('pixel.undo', 'Undo'))} (Ctrl+Z)">${toolbarIcon('undo', 'U')}</button>
                <button class="pixel-btn-icon" type="button" data-action="redo" title="${esc(t('pixel.redo', 'Redo'))} (Ctrl+Shift+Z)">${toolbarIcon('redo', 'R')}</button>
                <span class="pixel-toolbar-sep"></span>
                <button class="pixel-btn-icon" type="button" data-action="zoom-out" title="${esc(t('pixel.zoom_out', 'Zoom Out'))}">${toolbarIcon('zoom-out', '-')}</button>
                <span class="pixel-zoom-label" data-zoom-label>100%</span>
                <button class="pixel-btn-icon" type="button" data-action="zoom-in" title="${esc(t('pixel.zoom_in', 'Zoom In'))}">${toolbarIcon('zoom-in', '+')}</button>
                <button class="pixel-btn-icon" type="button" data-action="zoom-fit" title="${esc(t('pixel.zoom_fit', 'Fit'))}">${toolbarIcon('maximize', 'F')}</button>
                <span class="pixel-toolbar-spacer"></span>
                <div class="pixel-tabs">
                    <button class="pixel-tab${activePanel === 'adjust' ? ' active' : ''}" type="button" data-panel="adjust">${esc(t('pixel.adjust', 'Adjust'))}</button>
                    <button class="pixel-tab${activePanel === 'filters' ? ' active' : ''}" type="button" data-panel="filters">${esc(t('pixel.filters', 'Filters'))}</button>
                    <button class="pixel-tab${activePanel === 'transform' ? ' active' : ''}" type="button" data-panel="transform">${esc(t('pixel.transform', 'Transform'))}</button>
                    <button class="pixel-tab${activePanel === 'draw' ? ' active' : ''}" type="button" data-panel="draw">${esc(t('pixel.draw', 'Draw'))}</button>
                    <button class="pixel-tab${activePanel === 'layers' ? ' active' : ''}" type="button" data-panel="layers">${esc(t('pixel.layers', 'Layers'))}</button>
                    <button class="pixel-tab${activePanel === 'ai' ? ' active' : ''}" type="button" data-panel="ai">${esc(t('pixel.ai_generate', 'AI'))}</button>
                </div>
            </div>
            <div class="pixel-workspace">
                <div class="pixel-canvas-area" data-canvas-area>
                    <div class="pixel-canvas-wrap" data-canvas-wrap hidden>
                        <canvas class="pixel-canvas" data-canvas></canvas>
                        <canvas class="pixel-overlay" data-overlay></canvas>
                    </div>
                    <div class="pixel-crop-overlay" data-crop-overlay hidden></div>
                    <div class="pixel-compare-divider" data-compare-divider hidden></div>
                    <div class="pixel-empty-state" data-empty>
                        ${iconMarkup('image', 'Img', 'pixel-empty-icon', 64)}
                        <p>${esc(t('pixel.no_image', 'Open an image to start editing'))}</p>
                        <div class="pixel-empty-actions">
                            <button class="pixel-btn pixel-btn-primary" type="button" data-action="open">${esc(t('pixel.open', 'Open Image'))}</button>
                            <button class="pixel-btn" type="button" data-action="new-image">${esc(t('pixel.new_image', 'New Image'))}</button>
                        </div>
                        <div class="pixel-templates" data-templates>
                            <span class="pixel-label">${esc(t('pixel.quick_start', 'Quick Start'))}</span>
                            <div class="pixel-template-grid">
                                <button class="pixel-template-btn" type="button" data-template="512x512">512×512</button>
                                <button class="pixel-template-btn" type="button" data-template="1024x1024">1024×1024</button>
                                <button class="pixel-template-btn" type="button" data-template="1920x1080">1920×1080</button>
                            </div>
                        </div>
                        <div class="pixel-recent-files" data-recent-files></div>
                        <div class="pixel-photos-section" data-photos-section>
                            <span class="pixel-label">${esc(t('pixel.photos', 'Photos'))}</span>
                            <div class="pixel-photos-grid" data-photos-grid></div>
                        </div>
                    </div>
                </div>
                <div class="pixel-panel" data-panel-container>${buildPanelHTML()}</div>
            </div>
            <div class="pixel-status-bar" data-status>
                <span data-status-text>${esc(t('pixel.status_ready', 'Ready'))}</span>
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
        const zoomLabel = host.querySelector('[data-zoom-label]');
        cropOverlay = host.querySelector('[data-crop-overlay]');
        const compareDivider = host.querySelector('[data-compare-divider]');

        layers = [{ canvas: null, name: t('pixel.layer_background', 'Background'), visible: true, opacity: 1 }];
        activeLayerIdx = 0;

        function wireClick(selector, handler) {
            host.querySelectorAll(selector).forEach(el => el.addEventListener('click', handler));
        }

        // Event wiring
        wireClick('[data-action="open"]', openFile);
        wireClick('[data-action="save"]', saveFile);
        wireClick('[data-action="export"]', exportFile);
        wireClick('[data-action="undo"]', undo);
        wireClick('[data-action="redo"]', redo);
        wireClick('[data-action="zoom-in"]', () => zoomTo(zoom * 1.25));
        wireClick('[data-action="zoom-out"]', () => zoomTo(zoom / 1.25));
        wireClick('[data-action="zoom-fit"]', zoomFit);
        host.querySelectorAll('.pixel-tab').forEach(b => b.addEventListener('click', () => showPanel(b.dataset.panel)));
        wireClick('[data-action="apply-adjust"]', applyAdjustments);
        wireClick('[data-action="reset-adjust"]', resetAdjustments);
        wireClick('[data-action="compare-toggle"]', toggleCompare);
        host.querySelectorAll('[data-filter]').forEach(b => b.addEventListener('click', () => applyFilterPreview(b.dataset.filter)));
        wireClick('[data-action="rotate-cw"]', () => rotateCanvas(90));
        wireClick('[data-action="rotate-ccw"]', () => rotateCanvas(-90));
        wireClick('[data-action="flip-h"]', () => flipCanvas(true));
        wireClick('[data-action="flip-v"]', () => flipCanvas(false));
        wireClick('[data-action="crop"]', startCrop);
        wireClick('[data-action="apply-crop"]', applyCrop);
        wireClick('[data-action="cancel-crop"]', cancelCrop);
        wireClick('[data-action="resize"]', resizeImage);
        wireClick('[data-action="ai-generate"]', aiGenerate);
        wireClick('[data-action="ai-enhance"]', aiEnhance);
        wireClick('[data-action="new-image"]', showNewImageDialog);
        wireClick('[data-action="swap-colors"]', swapColors);
        wireClick('[data-action="clear-selection"]', deselect);
        wireClick('[data-action="add-layer"]', addLayer);
        wireClick('[data-action="delete-layer"]', deleteLayer);
        wireClick('[data-action="duplicate-layer"]', duplicateLayer);
        wireClick('[data-action="merge-layers"]', mergeDown);
        wireClick('[data-action="flatten-layers"]', flattenLayers);

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
                filePath = path;
                fileName = path.split('/').pop();
                api('/api/desktop/preview?path=' + encodeURIComponent(path)).then(r => { if (r && r.url) loadImageToCanvas(r.url); }).catch(() => {});
            }
            const photoBtn = e.target.closest('[data-photo-path]');
            if (photoBtn) {
                const path = photoBtn.dataset.photoPath;
                filePath = path;
                fileName = path.split('/').pop();
                saveRecentFile(filePath);
                api('/api/desktop/preview?path=' + encodeURIComponent(path)).then(r => { if (r && r.url) loadImageToCanvas(r.url); }).catch(() => {});
            }
        });

        state._cropMouseDown = onCropMouseDown;
        cropOverlay.addEventListener('mousedown', state._cropMouseDown);

        overlayCanvas.addEventListener('mousedown', onCanvasMouseDown);
        overlayCanvas.addEventListener('mousemove', onCanvasMouseMove);
        overlayCanvas.addEventListener('mouseup', onCanvasMouseUp);
        overlayCanvas.addEventListener('mouseleave', () => { if (isDrawing && activeTool && ['brush', 'eraser', 'pencil'].includes(activeTool)) { isDrawing = false; pushHistory('draw:' + activeTool); } });

        canvasArea.addEventListener('contextmenu', showContextMenu);

        host.querySelectorAll('[data-draw-tool]').forEach(b => {
            b.addEventListener('click', () => setActiveTool(b.dataset.drawTool));
        });

        host.querySelectorAll('[data-palette-color]').forEach(b => {
            b.addEventListener('click', () => { setPrimaryColor(b.dataset.paletteColor); addRecentColor(b.dataset.paletteColor); });
        });

        host.querySelectorAll('[data-palette-color]').forEach(b => {
            b.addEventListener('contextmenu', e => { e.preventDefault(); e.stopPropagation(); setSecondaryColor(b.dataset.paletteColor); });
        });

        host.addEventListener('click', e => {
            const rc = e.target.closest('[data-recent-color]');
            if (rc) { setPrimaryColor(rc.dataset.recentColor); }
        });

        const primarySwatch = host.querySelector('[data-color-primary]');
        if (primarySwatch) {
            primarySwatch.addEventListener('click', () => {
                const input = document.createElement('input');
                input.type = 'color';
                input.value = primaryColor;
                input.addEventListener('input', () => setPrimaryColor(input.value));
                input.addEventListener('change', () => { addRecentColor(input.value); input.remove(); });
                input.click();
            });
        }

        const secondarySwatch = host.querySelector('[data-color-secondary]');
        if (secondarySwatch) {
            secondarySwatch.addEventListener('click', () => {
                const input = document.createElement('input');
                input.type = 'color';
                input.value = secondaryColor;
                input.addEventListener('input', () => setSecondaryColor(input.value));
                input.addEventListener('change', () => input.remove());
                input.click();
            });
        }

        const hexInput = host.querySelector('[data-hex-input]');
        if (hexInput) {
            hexInput.addEventListener('change', () => {
                let v = hexInput.value.trim();
                if (!v.startsWith('#')) v = '#' + v;
                if (/^#[0-9a-fA-F]{6}$/.test(v)) { setPrimaryColor(v); addRecentColor(v); }
                else hexInput.value = primaryColor;
            });
        }

        const mainOpacitySlider = host.querySelector('[data-opacity-slider]');
        if (mainOpacitySlider) {
            mainOpacitySlider.addEventListener('input', () => {
                brushOpacity = Number(mainOpacitySlider.value);
                const next = mainOpacitySlider.nextElementSibling;
                if (next) next.textContent = brushOpacity + '%';
            });
        }

        host.querySelectorAll('[data-adjust]').forEach(slider => {
            slider.addEventListener('input', () => {
                adjustments[slider.dataset.adjust] = Number(slider.value);
                const valEl = host.querySelector(`[data-val-${slider.dataset.adjust}]`);
                if (valEl) valEl.textContent = slider.value;
                applyAdjustmentsPreview();
            });
            slider.addEventListener('dblclick', () => { slider.value = 0; adjustments[slider.dataset.adjust] = 0; const valEl = host.querySelector(`[data-val-${slider.dataset.adjust}]`); if (valEl) valEl.textContent = '0'; applyAdjustmentsPreview(); });
        });

        const strengthSlider = host.querySelector('[data-enhance-strength]');
        if (strengthSlider) {
            strengthSlider.addEventListener('input', () => { const el = host.querySelector('[data-strength-val]'); if (el) el.textContent = strengthSlider.value; });
        }

        host.addEventListener('click', e => {
            const visBtn = e.target.closest('[data-layer-vis]');
            if (visBtn) {
                const idx = parseInt(visBtn.dataset.layerVis);
                if (layers[idx]) { layers[idx].visible = !layers[idx].visible; compositeLayers(); refreshLayerPanel(); }
            }
        });

        host.addEventListener('click', e => {
            const nameEl = e.target.closest('[data-layer-name]');
            if (nameEl) {
                const idx = parseInt(nameEl.dataset.layerName);
                activeLayerIdx = idx;
                refreshLayerPanel();
            }
        });

        host.addEventListener('input', e => {
            const opSlider = e.target.closest('[data-layer-opacity]');
            if (opSlider) {
                const idx = parseInt(opSlider.dataset.layerOpacity);
                if (layers[idx]) { layers[idx].opacity = Number(opSlider.value) / 100; compositeLayers(); }
            }
        });

        wireDrawOptionEvents();

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

        document.addEventListener('mousemove', e => {
            if (isPanning) {
                canvasArea.scrollLeft = panStartSL - (e.clientX - panStartX);
                canvasArea.scrollTop = panStartST - (e.clientY - panStartY);
            }
        });

        document.addEventListener('mouseup', () => {
            if (isPanning) {
                isPanning = false;
                canvasArea.style.cursor = spaceHeld ? 'grab' : '';
            }
        });

        // Double-click to fit
        canvasArea.addEventListener('dblclick', () => { if (imgWidth) zoomFit(); });

        // Keyboard shortcuts
        state._keydown = e => {
            if (state.disposed) return;
            if (e.target.closest('input, textarea, select')) return;

            if (e.key === ' ') { e.preventDefault(); spaceHeld = true; canvasArea.style.cursor = 'grab'; }

            if (e.ctrlKey || e.metaKey) {
                if (e.key === 'o') { e.preventDefault(); openFile(); }
                if (e.key === 's' && e.shiftKey) { e.preventDefault(); saveFileAs(); }
                else if (e.key === 's') { e.preventDefault(); saveFile(); }
                if (e.key === 'z' && e.shiftKey) { e.preventDefault(); redo(); }
                else if (e.key === 'z') { e.preventDefault(); undo(); }
                if (e.key === '=' || e.key === '+') { e.preventDefault(); zoomTo(zoom * 1.25); }
                if (e.key === '-') { e.preventDefault(); zoomTo(zoom / 1.25); }
                if (e.key === '0') { e.preventDefault(); zoomFit(); }
                if (e.key === 'c') { e.preventDefault(); if (selection) copySelection(); }
                if (e.key === 'x') { e.preventDefault(); if (selection) cutSelection(); }
                if (e.key === 'v') { e.preventDefault(); pasteClipboard(); }
                if (e.key === 'a') { e.preventDefault(); selectAll(); }
                if (e.key === 'd') { e.preventDefault(); deselect(); }
            }

            if (!e.ctrlKey && !e.metaKey) {
                if (e.key === 'b') setActiveTool('brush');
                if (e.key === 'e') setActiveTool('eraser');
                if (e.key === 'l') setActiveTool('line');
                if (e.key === 'r') setActiveTool('rectangle');
                if (e.key === 'o') setActiveTool('ellipse');
                if (e.key === 't') setActiveTool('text');
                if (e.key === 'g') setActiveTool('fill');
                if (e.key === 'i') setActiveTool('eyedropper');
                if (e.key === 'v') setActiveTool('select-rect');
            }

            if (e.key === 'Escape' && cropState) cancelCrop();
            if (e.key === 'Escape' && selection) deselect();
            if (e.key === 'Delete' && selection) { deleteSelection(); deselect(); }
            if (e.key === '?') showShortcutsModal();
            if (e.key === 'F1') { e.preventDefault(); showShortcutsModal(); }
        };
        document.addEventListener('keydown', state._keydown);

        state._keyup = e => {
            if (e.key === ' ') { spaceHeld = false; if (!isPanning) canvasArea.style.cursor = ''; }
        };
        document.addEventListener('keyup', state._keyup);

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
            const imgPath = paths.find(p => IMAGE_EXTS.some(ext => p.toLowerCase().endsWith('.' + ext)));
            if (imgPath) {
                filePath = imgPath;
                fileName = imgPath.split('/').pop();
                saveRecentFile(filePath);
                api('/api/desktop/preview?path=' + encodeURIComponent(imgPath)).then(r => { if (r && r.url) loadImageToCanvas(r.url); }).catch(() => {});
            }
        });

        // Window menus
        if (typeof ctx.setWindowMenus === 'function') {
            ctx.setWindowMenus(windowId, [
                { id: 'file', labelKey: 'desktop.menu_file', items: [
                    { id: 'open', labelKey: 'pixel.open', icon: 'folder-open', shortcut: 'Ctrl+O', action: openFile },
                    { id: 'save', labelKey: 'pixel.save', icon: 'save', shortcut: 'Ctrl+S', action: saveFile },
                    { id: 'save-as', labelKey: 'pixel.save_as', icon: 'save', shortcut: 'Ctrl+Shift+S', action: saveFileAs },
                    { id: 'export', labelKey: 'pixel.export', icon: 'download', action: exportFile },
                    { type: 'separator' },
                    { id: 'new-image', labelKey: 'pixel.new_image', icon: 'image', action: showNewImageDialog }
                ]},
                { id: 'edit', labelKey: 'desktop.menu_edit', items: [
                    { id: 'undo', labelKey: 'pixel.undo', icon: 'undo', shortcut: 'Ctrl+Z', action: undo },
                    { id: 'redo', labelKey: 'pixel.redo', icon: 'redo', shortcut: 'Ctrl+Shift+Z', action: redo },
                    { type: 'separator' },
                    { id: 'copy', labelKey: 'pixel.copy', icon: 'image', shortcut: 'Ctrl+C', action: copySelection },
                    { id: 'cut', labelKey: 'pixel.cut', icon: 'scissors', shortcut: 'Ctrl+X', action: cutSelection },
                    { id: 'paste', labelKey: 'pixel.paste', icon: 'image', shortcut: 'Ctrl+V', action: pasteClipboard },
                    { type: 'separator' },
                    { id: 'select-all', labelKey: 'pixel.select_all', icon: 'image', shortcut: 'Ctrl+A', action: selectAll },
                    { id: 'deselect', labelKey: 'pixel.deselect', icon: 'image', shortcut: 'Ctrl+D', action: deselect }
                ]},
                { id: 'view', labelKey: 'desktop.menu_view', items: [
                    { id: 'zoom-in', labelKey: 'pixel.zoom_in', icon: 'zoom-in', shortcut: 'Ctrl++', action: () => zoomTo(zoom * 1.25) },
                    { id: 'zoom-out', labelKey: 'pixel.zoom_out', icon: 'zoom-out', shortcut: 'Ctrl+-', action: () => zoomTo(zoom / 1.25) },
                    { id: 'zoom-fit', labelKey: 'pixel.zoom_fit', icon: 'maximize', shortcut: 'Ctrl+0', action: zoomFit },
                    { id: 'zoom-100', labelKey: 'pixel.zoom_100', icon: 'zoom-in', action: () => zoomTo(1) },
                    { type: 'separator' },
                    { id: 'shortcuts', labelKey: 'pixel.shortcuts', icon: 'image', shortcut: '?', action: showShortcutsModal }
                ]},
                { id: 'image', labelKey: 'pixel.menu_image', items: [
                    { id: 'rotate-cw', labelKey: 'pixel.rotate_cw', icon: 'redo', action: () => rotateCanvas(90) },
                    { id: 'rotate-ccw', labelKey: 'pixel.rotate_ccw', icon: 'undo', action: () => rotateCanvas(-90) },
                    { id: 'flip-h', labelKey: 'pixel.flip_h', icon: 'sort', action: () => flipCanvas(true) },
                    { id: 'flip-v', labelKey: 'pixel.flip_v', icon: 'sort', action: () => flipCanvas(false) },
                    { type: 'separator' },
                    { id: 'crop', labelKey: 'pixel.crop', icon: 'scissors', action: startCrop },
                    { id: 'resize', labelKey: 'pixel.resize', icon: 'maximize', action: resizeImage }
                ]},
                { id: 'ai', labelKey: 'pixel.ai_generate', items: [
                    { id: 'generate', labelKey: 'pixel.generate', icon: 'image', action: () => { showPanel('ai'); } },
                    { id: 'enhance', labelKey: 'pixel.enhance', icon: 'image', action: () => { showPanel('ai'); } }
                ]}
            ]);
        }

        // Load initial image
        if (filePath) {
            api('/api/desktop/preview?path=' + encodeURIComponent(filePath)).then(r => {
                if (r && r.url) loadImageToCanvas(r.url);
            }).catch(() => {});
        }

        renderRecentFiles();
        loadPhotos();
        checkAIConfig();

        state.dispose = function () {
            state.disposed = true;
            if (abortCtrl) abortCtrl.abort();
            stopMarchingAnts();
            if (state._keydown) document.removeEventListener('keydown', state._keydown);
            if (state._keyup) document.removeEventListener('keyup', state._keyup);
            if (state._cropMouseDown) cropOverlay.removeEventListener('mousedown', state._cropMouseDown);
            overlayCanvas.removeEventListener('mousedown', onCanvasMouseDown);
            overlayCanvas.removeEventListener('mousemove', onCanvasMouseMove);
            overlayCanvas.removeEventListener('mouseup', onCanvasMouseUp);
            history = [];
            originalImage = null;
            selImageData = null;
            layers = [];
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
