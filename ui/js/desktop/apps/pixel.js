(function () {
    'use strict';

    const instances = new Map();
    const MAX_HISTORY = 5;
    const canvasPool = [];
    const IMAGE_EXTS = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'tiff', 'tif', 'avif'];

    function acquireTempCanvas(width, height) {
        const canvas = canvasPool.pop() || document.createElement('canvas');
        canvas.width = width;
        canvas.height = height;
        return canvas;
    }

    function releaseTempCanvas(canvas) {
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
        if (canvasPool.length < 4) canvasPool.push(canvas);
    }

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
        let canvas, cctx;
        let history = [];
        let historyIdx = -1;
        let zoom = 1;
        let panX = 0, panY = 0;
        let filePath = ctx.path || '';
        let fileName = filePath ? filePath.split('/').pop() : '';
        let isDirty = false;
        let activePanel = 'adjust';
        let cropState = null;
        let cropOverlay = null;
        let abortCtrl = null;
        let imgWidth = 0, imgHeight = 0;

        const adjustments = { brightness: 0, contrast: 0, saturation: 0, exposure: 0, sharpness: 0, temperature: 0, shadows: 0, highlights: 0 };

        host.innerHTML = `<div class="pixel-app">
            <div class="pixel-toolbar">
                <button class="pixel-btn" type="button" data-action="open" title="${esc(t('pixel.open', 'Open'))}">${iconMarkup('folder-open', 'O')}<span>${esc(t('pixel.open', 'Open'))}</span></button>
                <button class="pixel-btn" type="button" data-action="save" title="${esc(t('pixel.save', 'Save'))}">${iconMarkup('save', 'S')}<span>${esc(t('pixel.save', 'Save'))}</span></button>
                <button class="pixel-btn" type="button" data-action="export" title="${esc(t('pixel.export', 'Export'))}">${iconMarkup('download', 'E')}<span>${esc(t('pixel.export', 'Export'))}</span></button>
                <span class="pixel-toolbar-sep"></span>
                <button class="pixel-btn-icon" type="button" data-action="undo" title="${esc(t('pixel.undo', 'Undo'))}">${iconMarkup('undo', 'U')}</button>
                <button class="pixel-btn-icon" type="button" data-action="redo" title="${esc(t('pixel.redo', 'Redo'))}">${iconMarkup('redo', 'R')}</button>
                <span class="pixel-toolbar-sep"></span>
                <button class="pixel-btn-icon" type="button" data-action="zoom-out" title="${esc(t('pixel.zoom_out', 'Zoom Out'))}">${iconMarkup('zoom-out', '-')}</button>
                <span class="pixel-zoom-label" data-zoom-label>100%</span>
                <button class="pixel-btn-icon" type="button" data-action="zoom-in" title="${esc(t('pixel.zoom_in', 'Zoom In'))}">${iconMarkup('zoom-in', '+')}</button>
                <button class="pixel-btn-icon" type="button" data-action="zoom-fit" title="${esc(t('pixel.zoom_fit', 'Fit'))}">${iconMarkup('maximize', 'F')}</button>
                <span class="pixel-toolbar-spacer"></span>
                <div class="pixel-tabs">
                    <button class="pixel-tab${activePanel === 'adjust' ? ' active' : ''}" type="button" data-panel="adjust">${esc(t('pixel.adjust', 'Adjust'))}</button>
                    <button class="pixel-tab${activePanel === 'filters' ? ' active' : ''}" type="button" data-panel="filters">${esc(t('pixel.filters', 'Filters'))}</button>
                    <button class="pixel-tab${activePanel === 'transform' ? ' active' : ''}" type="button" data-panel="transform">${esc(t('pixel.transform', 'Transform'))}</button>
                    <button class="pixel-tab${activePanel === 'ai' ? ' active' : ''}" type="button" data-panel="ai">${esc(t('pixel.ai_generate', 'AI'))}</button>
                </div>
            </div>
            <div class="pixel-workspace">
                <div class="pixel-canvas-area" data-canvas-area>
                    <canvas class="pixel-canvas" data-canvas></canvas>
                    <div class="pixel-crop-overlay" data-crop-overlay hidden></div>
                    <div class="pixel-empty-state" data-empty>${iconMarkup('image', 'Img', 'pixel-empty-icon', 64)}<p>${esc(t('pixel.no_image', 'Open an image to start editing'))}</p></div>
                </div>
                <div class="pixel-panel" data-panel-container>${buildPanelHTML()}</div>
            </div>
            <div class="pixel-status-bar" data-status>${esc(t('pixel.status_ready', 'Ready'))}</div>
        </div>`;

        const appEl = host.querySelector('.pixel-app');
        canvas = host.querySelector('[data-canvas]');
        cctx = canvas.getContext('2d', { willReadFrequently: true });
        const canvasArea = host.querySelector('[data-canvas-area]');
        const emptyState = host.querySelector('[data-empty]');
        const statusNode = host.querySelector('[data-status]');
        const panelContainer = host.querySelector('[data-panel-container]');
        const zoomLabel = host.querySelector('[data-zoom-label]');
        cropOverlay = host.querySelector('[data-crop-overlay]');

        function buildPanelHTML() {
            return `<div class="pixel-panel-section pixel-panel-adjust" data-section="adjust">
                ${buildSlider('brightness', 'Brightness', -100, 100, 0)}
                ${buildSlider('contrast', 'Contrast', -100, 100, 0)}
                ${buildSlider('saturation', 'Saturation', -100, 100, 0)}
                ${buildSlider('exposure', 'Exposure', -100, 100, 0)}
                ${buildSlider('sharpness', 'Sharpness', 0, 100, 0)}
                ${buildSlider('temperature', 'Temperature', -100, 100, 0)}
                ${buildSlider('shadows', 'Shadows', -100, 100, 0)}
                ${buildSlider('highlights', 'Highlights', -100, 100, 0)}
                <div class="pixel-panel-actions"><button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-adjust">${esc(t('pixel.apply', 'Apply'))}</button><button class="pixel-btn" type="button" data-action="reset-adjust">${esc(t('pixel.reset', 'Reset'))}</button></div>
            </div>
            <div class="pixel-panel-section pixel-panel-filters" data-section="filters" hidden>
                <div class="pixel-filter-grid">
                    ${buildFilterCard('grayscale', 'Grayscale')}
                    ${buildFilterCard('sepia', 'Sepia')}
                    ${buildFilterCard('invert', 'Invert')}
                    ${buildFilterCard('blur', 'Blur')}
                    ${buildFilterCard('vintage', 'Vintage')}
                    ${buildFilterCard('vignette', 'Vignette')}
                    ${buildFilterCard('warm', 'Warm')}
                    ${buildFilterCard('cool', 'Cool')}
                    ${buildFilterCard('high-contrast', 'High Contrast')}
                    ${buildFilterCard('emboss', 'Emboss')}
                </div>
            </div>
            <div class="pixel-panel-section pixel-panel-transform" data-section="transform" hidden>
                <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="rotate-cw">${iconMarkup('redo', 'CW')} ${esc(t('pixel.rotate_cw', 'Rotate CW'))}</button><button class="pixel-btn" type="button" data-action="rotate-ccw">${iconMarkup('undo', 'CCW')} ${esc(t('pixel.rotate_ccw', 'Rotate CCW'))}</button></div>
                <div class="pixel-btn-group"><button class="pixel-btn" type="button" data-action="flip-h">${iconMarkup('sort', 'H')} ${esc(t('pixel.flip_h', 'Flip H'))}</button><button class="pixel-btn" type="button" data-action="flip-v">${iconMarkup('sort', 'V')} ${esc(t('pixel.flip_v', 'Flip V'))}</button></div>
                <hr class="pixel-divider">
                <button class="pixel-btn" type="button" data-action="crop">${iconMarkup('scissors', 'C')} ${esc(t('pixel.crop', 'Crop'))}</button>
                <div class="pixel-crop-actions" data-crop-actions hidden>
                    <button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-crop">${esc(t('pixel.apply_crop', 'Apply Crop'))}</button>
                    <button class="pixel-btn" type="button" data-action="cancel-crop">${esc(t('pixel.cancel_crop', 'Cancel'))}</button>
                </div>
                <hr class="pixel-divider">
                <button class="pixel-btn" type="button" data-action="resize">${iconMarkup('maximize', 'R')} ${esc(t('pixel.resize', 'Resize'))}</button>
            </div>
            <div class="pixel-panel-section pixel-panel-ai" data-section="ai" hidden>
                <div class="pixel-ai-status" data-ai-status></div>
                <div class="pixel-ai-panel">
                    <label class="pixel-label">${esc(t('pixel.prompt', 'Prompt'))}</label>
                    <textarea class="pixel-ai-prompt" data-ai-prompt rows="3" placeholder="${esc(t('pixel.prompt_placeholder', 'Describe the image to generate...'))}"></textarea>
                    <div class="pixel-ai-options">
                        <label class="pixel-label">${esc(t('pixel.ai_size', 'Size'))}</label>
                        <select class="pixel-select" data-ai-size><option value="1024x1024">1024×1024</option><option value="1024x1792">1024×1792</option><option value="1792x1024">1792×1024</option><option value="512x512">512×512</option></select>
                        <label class="pixel-label">${esc(t('pixel.ai_quality', 'Quality'))}</label>
                        <select class="pixel-select" data-ai-quality><option value="standard">Standard</option><option value="hd">HD</option></select>
                    </div>
                    <button class="pixel-btn pixel-btn-primary pixel-btn-full" type="button" data-action="ai-generate">${esc(t('pixel.generate', 'Generate'))}</button>
                </div>
                <hr class="pixel-divider">
                <div class="pixel-ai-panel">
                    <label class="pixel-label">${esc(t('pixel.enhance', 'Enhance Image'))}</label>
                    <textarea class="pixel-ai-prompt" data-enhance-prompt rows="2" placeholder="${esc(t('pixel.prompt_placeholder', 'Describe enhancements...'))}"></textarea>
                    <label class="pixel-label">${esc(t('pixel.ai_strength', 'Strength'))} <span data-strength-val>0.7</span></label>
                    <input type="range" class="pixel-slider" data-enhance-strength min="0.1" max="1" step="0.05" value="0.7">
                    <button class="pixel-btn pixel-btn-full" type="button" data-action="ai-enhance">${esc(t('pixel.enhance', 'Enhance'))}</button>
                </div>
            </div>`;
        }

        function buildSlider(key, label, min, max, def) {
            return `<div class="pixel-slider-row">
                <span class="pixel-slider-label">${esc(t('pixel.' + key, label))}</span>
                <input type="range" class="pixel-slider" data-adjust="${key}" min="${min}" max="${max}" value="${def}">
                <span class="pixel-slider-value" data-val-${key}>${def}</span>
            </div>`;
        }

        function buildFilterCard(key, label) {
            return `<button class="pixel-filter-card" type="button" data-filter="${key}"><span class="pixel-filter-preview pixel-filter-${key}"></span><span class="pixel-filter-name">${esc(t('pixel.filter_' + key.replace('-', '_'), label))}</span></button>`;
        }

        function setStatus(msg) { if (statusNode) statusNode.textContent = msg || ''; }

        function updateStatus() {
            if (!imgWidth || !imgHeight) { setStatus(t('pixel.status_ready', 'Ready')); return; }
            const parts = [`${imgWidth} × ${imgHeight} px`];
            if (filePath) parts.push(filePath.split('/').pop());
            if (isDirty) parts.push('● ' + t('pixel.unsaved_changes', 'Unsaved'));
            setStatus(parts.join('  ·  '));
        }

        function updateZoomLabel() { if (zoomLabel) zoomLabel.textContent = Math.round(zoom * 100) + '%'; }

        function pushHistory() {
            if (!canvas.width || !canvas.height) return;
            const data = cctx.getImageData(0, 0, canvas.width, canvas.height);
            if (historyIdx < history.length - 1) history = history.slice(0, historyIdx + 1);
            history.push({ imageData: data, width: canvas.width, height: canvas.height });
            if (history.length > MAX_HISTORY) history.shift();
            historyIdx = history.length - 1;
            isDirty = true;
            updateStatus();
        }

        function restoreHistory(entry) {
            canvas.width = entry.width;
            canvas.height = entry.height;
            cctx.putImageData(entry.imageData, 0, 0);
            imgWidth = entry.width;
            imgHeight = entry.height;
            applyZoom();
        }

        function undo() {
            if (historyIdx <= 0) return;
            historyIdx--;
            restoreHistory(history[historyIdx]);
            updateStatus();
        }

        function redo() {
            if (historyIdx >= history.length - 1) return;
            historyIdx++;
            restoreHistory(history[historyIdx]);
            updateStatus();
        }

        function applyZoom() {
            canvas.style.width = (imgWidth * zoom) + 'px';
            canvas.style.height = (imgHeight * zoom) + 'px';
            updateZoomLabel();
        }

        function zoomTo(z) {
            zoom = Math.max(0.1, Math.min(10, z));
            applyZoom();
        }

        function zoomFit() {
            if (!imgWidth || !imgHeight) return;
            const areaW = canvasArea.clientWidth - 20;
            const areaH = canvasArea.clientHeight - 20;
            zoom = Math.min(areaW / imgWidth, areaH / imgHeight, 1);
            applyZoom();
        }

        function loadImage(src) {
            return new Promise((resolve, reject) => {
                const img = new Image();
                img.crossOrigin = 'anonymous';
                img.onload = () => resolve(img);
                img.onerror = () => reject(new Error('Failed to load image'));
                img.src = src;
            });
        }

        async function loadImageToCanvas(src) {
            const img = await loadImage(src);
            originalImage = img;
            canvas.width = img.naturalWidth;
            canvas.height = img.naturalHeight;
            cctx.clearRect(0, 0, canvas.width, canvas.height);
            cctx.drawImage(img, 0, 0);
            imgWidth = img.naturalWidth;
            imgHeight = img.naturalHeight;
            history = [];
            historyIdx = -1;
            pushHistory();
            emptyState.hidden = true;
            zoomFit();
            updateStatus();
        }

        async function openFile() {
            if (!ctx.openFileDialog) return;
            const result = await ctx.openFileDialog({ filters: [{ name: 'Images', extensions: IMAGE_EXTS }] });
            if (result && !result.canceled && result.path) {
                filePath = result.path;
                fileName = filePath.split('/').pop();
                isDirty = false;
                try {
                    const preview = await api('/api/desktop/preview?path=' + encodeURIComponent(filePath));
                    if (preview && preview.url) {
                        await loadImageToCanvas(preview.url);
                    } else {
                        await loadImageToCanvas('/api/desktop/preview?path=' + encodeURIComponent(filePath) + '&raw=1');
                    }
                } catch (_) {
                    notify({ type: 'error', message: t('pixel.error_load', 'Failed to load image') });
                }
            }
        }

        async function saveFile() {
            if (!canvas.width) return;
            if (!filePath) { await saveFileAs(); return; }
            setStatus(t('pixel.status_saving', 'Saving...'));
            try {
                const dataURL = canvas.toDataURL('image/png');
                await api('/api/pixel/save', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: filePath, data: dataURL, format: 'png' })
                });
                isDirty = false;
                updateStatus();
                notify({ type: 'success', message: t('pixel.saved', 'Image saved') });
            } catch (err) {
                notify({ type: 'error', message: t('pixel.error_save', 'Failed to save') });
                setStatus(t('pixel.error_save', 'Failed to save'));
            }
        }

        async function saveFileAs() {
            if (!canvas.width) return;
            if (!ctx.saveFileDialog) return;
            const result = await ctx.saveFileDialog({ filters: [{ name: 'PNG Image', extensions: ['png'] }, { name: 'JPEG Image', extensions: ['jpg'] }] });
            if (result && !result.canceled && result.path) {
                filePath = result.path;
                fileName = filePath.split('/').pop();
                await saveFile();
            }
        }

        function exportFile() {
            if (!canvas.width) return;
            const link = document.createElement('a');
            link.download = fileName || 'image.png';
            link.href = canvas.toDataURL('image/png');
            link.click();
        }

        function showPanel(name) {
            activePanel = name;
            host.querySelectorAll('.pixel-tab').forEach(b => b.classList.toggle('active', b.dataset.panel === name));
            host.querySelectorAll('[data-section]').forEach(s => { s.hidden = s.dataset.section !== name; });
            if (name !== 'transform' && cropState) cancelCrop();
        }

        function buildFilterString() {
            const parts = [];
            const b = 1 + adjustments.brightness / 100;
            const c = 1 + adjustments.contrast / 100;
            const s = 1 + adjustments.saturation / 100;
            const e = 1 + adjustments.exposure / 200;
            if (b !== 1) parts.push(`brightness(${b})`);
            if (c !== 1) parts.push(`contrast(${c})`);
            if (s !== 1) parts.push(`saturate(${s})`);
            if (e !== 1) parts.push(`brightness(${e})`);
            if (adjustments.sharpness > 0) parts.push(`contrast(${1 + adjustments.sharpness / 200})`);
            return parts.join(' ') || 'none';
        }

        function applyAdjustmentsPreview() {
            if (!originalImage || !canvas.width) return;
            const tmpCanvas = acquireTempCanvas(imgWidth, imgHeight);
            const tmpCtx = tmpCanvas.getContext('2d');
            tmpCtx.filter = buildFilterString();
            tmpCtx.drawImage(originalImage, 0, 0, imgWidth, imgHeight);
            cctx.clearRect(0, 0, canvas.width, canvas.height);
            cctx.drawImage(tmpCanvas, 0, 0);
            releaseTempCanvas(tmpCanvas);
            applyCustomAdjustments(cctx);
        }

        function applyCustomAdjustments(ctx) {
            if (adjustments.temperature === 0 && adjustments.shadows === 0 && adjustments.highlights === 0) return;
            const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height);
            const d = imgData.data;
            const temp = adjustments.temperature * 0.5;
            const shadows = adjustments.shadows * 0.5;
            const highlights = adjustments.highlights * 0.5;
            for (let i = 0; i < d.length; i += 4) {
                d[i] = clamp(d[i] + temp);
                d[i + 2] = clamp(d[i + 2] - temp);
                const lum = (d[i] + d[i + 1] + d[i + 2]) / 3;
                if (lum < 128) { d[i] = clamp(d[i] + shadows); d[i + 1] = clamp(d[i + 1] + shadows); d[i + 2] = clamp(d[i + 2] + shadows); }
                if (lum > 128) { d[i] = clamp(d[i] + highlights); d[i + 1] = clamp(d[i + 1] + highlights); d[i + 2] = clamp(d[i + 2] + highlights); }
            }
            ctx.putImageData(imgData, 0, 0);
        }

        function clamp(v) { return Math.max(0, Math.min(255, Math.round(v))); }

        function applyAdjustments() {
            if (!originalImage) return;
            applyAdjustmentsPreview();
            const dataURL = canvas.toDataURL();
            const img = new Image();
            img.onload = function() { originalImage = img; };
            img.src = dataURL;
            pushHistory();
            resetAdjustments();
        }

        function resetAdjustments() {
            for (const k in adjustments) adjustments[k] = 0;
            host.querySelectorAll('[data-adjust]').forEach(s => { s.value = 0; });
            host.querySelectorAll('[data-val-brightness],[data-val-contrast],[data-val-saturation],[data-val-exposure],[data-val-sharpness],[data-val-temperature],[data-val-shadows],[data-val-highlights]').forEach(el => { el.textContent = '0'; });
            if (originalImage && canvas.width) { cctx.clearRect(0, 0, canvas.width, canvas.height); cctx.drawImage(originalImage, 0, 0); }
        }

        function applyFilter(name) {
            if (!canvas.width) return;
            const tmpCanvas = acquireTempCanvas(imgWidth, imgHeight);
            const tmpCtx = tmpCanvas.getContext('2d');
            const src = originalImage || canvas;
            switch (name) {
                case 'grayscale': tmpCtx.filter = 'grayscale(100%)'; break;
                case 'sepia': tmpCtx.filter = 'sepia(100%)'; break;
                case 'invert': tmpCtx.filter = 'invert(100%)'; break;
                case 'blur': tmpCtx.filter = 'blur(2px)'; break;
                case 'vintage': tmpCtx.filter = 'sepia(60%) contrast(80%) brightness(90%)'; break;
                case 'warm': tmpCtx.filter = 'saturate(1.3) brightness(1.05)'; break;
                case 'cool': tmpCtx.filter = 'saturate(0.8) hue-rotate(20deg)'; break;
                case 'high-contrast': tmpCtx.filter = 'contrast(150%)'; break;
                default: tmpCtx.filter = 'none';
            }
            tmpCtx.drawImage(src, 0, 0, imgWidth, imgHeight);
            cctx.clearRect(0, 0, canvas.width, canvas.height);
            cctx.drawImage(tmpCanvas, 0, 0);
            releaseTempCanvas(tmpCanvas);
            if (name === 'vignette') applyVignette(cctx);
            if (name === 'emboss') applyEmboss(cctx);
            pushHistory();
        }

        function applyVignette(ctx) {
            const w = canvas.width, h = canvas.height;
            const grd = ctx.createRadialGradient(w / 2, h / 2, w * 0.25, w / 2, h / 2, w * 0.7);
            grd.addColorStop(0, 'rgba(0,0,0,0)');
            grd.addColorStop(1, 'rgba(0,0,0,0.6)');
            ctx.fillStyle = grd;
            ctx.fillRect(0, 0, w, h);
        }

        function applyEmboss(ctx) {
            const imgData = ctx.getImageData(0, 0, canvas.width, canvas.height);
            const kernel = [-2, -1, 0, -1, 1, 1, 0, 1, 2];
            convolve(imgData, kernel, canvas.width, canvas.height);
            ctx.putImageData(imgData, 0, 0);
        }

        function convolve(imgData, kernel, w, h) {
            const src = new Uint8ClampedArray(imgData.data);
            const d = imgData.data;
            const kSize = Math.sqrt(kernel.length) | 0;
            const half = (kSize / 2) | 0;
            for (let y = half; y < h - half; y++) {
                for (let x = half; x < w - half; x++) {
                    let r = 0, g = 0, b = 0;
                    for (let ky = 0; ky < kSize; ky++) {
                        for (let kx = 0; kx < kSize; kx++) {
                            const idx = ((y + ky - half) * w + (x + kx - half)) * 4;
                            const kv = kernel[ky * kSize + kx];
                            r += src[idx] * kv; g += src[idx + 1] * kv; b += src[idx + 2] * kv;
                        }
                    }
                    const idx = (y * w + x) * 4;
                    d[idx] = clamp(r + 128); d[idx + 1] = clamp(g + 128); d[idx + 2] = clamp(b + 128);
                }
            }
        }

        function rotateCanvas(deg) {
            if (!canvas.width) return;
            const nextWidth = deg === 90 || deg === -90 ? canvas.height : canvas.width;
            const nextHeight = deg === 90 || deg === -90 ? canvas.width : canvas.height;
            const tmpCanvas = acquireTempCanvas(nextWidth, nextHeight);
            const tmpCtx = tmpCanvas.getContext('2d');
            tmpCtx.save();
            if (deg === 90) { tmpCtx.translate(tmpCanvas.width, 0); tmpCtx.rotate(Math.PI / 2); }
            else if (deg === -90) { tmpCtx.translate(0, tmpCanvas.height); tmpCtx.rotate(-Math.PI / 2); }
            else if (deg === 180) { tmpCtx.translate(tmpCanvas.width, tmpCanvas.height); tmpCtx.rotate(Math.PI); }
            tmpCtx.drawImage(canvas, 0, 0);
            tmpCtx.restore();
            canvas.width = tmpCanvas.width;
            canvas.height = tmpCanvas.height;
            cctx.drawImage(tmpCanvas, 0, 0);
            releaseTempCanvas(tmpCanvas);
            imgWidth = canvas.width;
            imgHeight = canvas.height;
            originalImage = new Image();
            originalImage.src = canvas.toDataURL();
            pushHistory();
            applyZoom();
        }

        function flipCanvas(horizontal) {
            if (!canvas.width) return;
            const tmpCanvas = acquireTempCanvas(canvas.width, canvas.height);
            const tmpCtx = tmpCanvas.getContext('2d');
            tmpCtx.save();
            if (horizontal) { tmpCtx.translate(canvas.width, 0); tmpCtx.scale(-1, 1); }
            else { tmpCtx.translate(0, canvas.height); tmpCtx.scale(1, -1); }
            tmpCtx.drawImage(canvas, 0, 0);
            tmpCtx.restore();
            cctx.clearRect(0, 0, canvas.width, canvas.height);
            cctx.drawImage(tmpCanvas, 0, 0);
            releaseTempCanvas(tmpCanvas);
            pushHistory();
        }

        function startCrop() {
            if (!canvas.width) return;
            cropState = { active: true, startX: 0, startY: 0, endX: 0, endY: 0 };
            cropOverlay.hidden = false;
            cropOverlay.innerHTML = '<div class="pixel-crop-selection" data-crop-sel></div>';
            host.querySelector('[data-crop-actions]').hidden = false;
            setStatus(t('pixel.crop_hint', 'Drag to select crop area'));
        }

        function cancelCrop() {
            cropState = null;
            cropOverlay.hidden = true;
            cropOverlay.innerHTML = '';
            const ca = host.querySelector('[data-crop-actions]');
            if (ca) ca.hidden = true;
            updateStatus();
        }

        function applyCrop() {
            if (!cropState) return;
            const sel = host.querySelector('[data-crop-sel]');
            if (!sel) { cancelCrop(); return; }
            const displayW = parseFloat(canvas.style.width) || canvas.width;
            const displayH = parseFloat(canvas.style.height) || canvas.height;
            const scaleX = canvas.width / displayW;
            const scaleY = canvas.height / displayH;
            const sx = Math.max(0, Math.round(Math.min(cropState.startX, cropState.endX) * scaleX));
            const sy = Math.max(0, Math.round(Math.min(cropState.startY, cropState.endY) * scaleY));
            const sw = Math.round(Math.abs(cropState.endX - cropState.startX) * scaleX);
            const sh = Math.round(Math.abs(cropState.endY - cropState.startY) * scaleY);
            if (sw < 2 || sh < 2) { cancelCrop(); return; }
            const imgData = cctx.getImageData(sx, sy, sw, sh);
            canvas.width = sw;
            canvas.height = sh;
            cctx.putImageData(imgData, 0, 0);
            imgWidth = sw;
            imgHeight = sh;
            originalImage = null;
            pushHistory();
            cancelCrop();
            zoomFit();
        }

        function resizeImage() {
            if (!canvas.width) return;
            const dlg = document.createElement('div');
            dlg.className = 'vd-modal-backdrop';
            dlg.innerHTML = `<form class="vd-modal" role="dialog">
                <div class="vd-modal-title">${esc(t('pixel.resize', 'Resize'))}</div>
                <div class="pixel-resize-form">
                    <label class="pixel-label">${esc(t('pixel.width', 'Width'))}</label>
                    <input class="vd-modal-input" type="number" data-resize-w value="${imgWidth}" min="1">
                    <label class="pixel-label">${esc(t('pixel.height', 'Height'))}</label>
                    <input class="vd-modal-input" type="number" data-resize-h value="${imgHeight}" min="1">
                    <label class="pixel-checkbox-row"><input type="checkbox" data-lock-ratio checked> ${esc(t('pixel.lock_ratio', 'Lock aspect ratio'))}</label>
                </div>
                <div class="vd-modal-actions">
                    <button type="button" class="vd-button" data-cancel>${esc(t('pixel.cancel', 'Cancel'))}</button>
                    <button type="submit" class="vd-button vd-button-primary">${esc(t('pixel.apply', 'Apply'))}</button>
                </div>
            </form>`;
            document.body.appendChild(dlg);
            const wInput = dlg.querySelector('[data-resize-w]');
            const hInput = dlg.querySelector('[data-resize-h]');
            const lock = dlg.querySelector('[data-lock-ratio]');
            const ratio = imgWidth / imgHeight;
            wInput.addEventListener('input', () => { if (lock.checked) hInput.value = Math.round(wInput.value / ratio); });
            hInput.addEventListener('input', () => { if (lock.checked) wInput.value = Math.round(hInput.value * ratio); });
            dlg.querySelector('form').addEventListener('submit', e => {
                e.preventDefault();
                const nw = parseInt(wInput.value) || imgWidth;
                const nh = parseInt(hInput.value) || imgHeight;
                if (nw < 1 || nh < 1) return;
                const tmpCanvas = acquireTempCanvas(nw, nh);
                tmpCanvas.getContext('2d').drawImage(canvas, 0, 0, nw, nh);
                canvas.width = nw;
                canvas.height = nh;
                cctx.drawImage(tmpCanvas, 0, 0);
                releaseTempCanvas(tmpCanvas);
                imgWidth = nw;
                imgHeight = nh;
                pushHistory();
                zoomFit();
                dlg.remove();
            });
            dlg.querySelector('[data-cancel]').addEventListener('click', () => dlg.remove());
            dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
        }

        async function aiGenerate() {
            const prompt = (host.querySelector('[data-ai-prompt]') || {}).value || '';
            if (!prompt.trim()) return;
            const size = (host.querySelector('[data-ai-size]') || {}).value || '1024x1024';
            const quality = (host.querySelector('[data-ai-quality]') || {}).value || 'standard';
            const statusEl = host.querySelector('[data-ai-status]');
            if (statusEl) statusEl.textContent = t('pixel.generating', 'Generating...');
            try {
                abortCtrl = new AbortController();
                const resp = await api('/api/pixel/generate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ prompt, size, quality }),
                    signal: abortCtrl.signal
                });
                if (resp && resp.url) {
                    await loadImageToCanvas(resp.url);
                    filePath = resp.path || '';
                    fileName = filePath ? filePath.split('/').pop() : '';
                }
                if (statusEl) statusEl.textContent = '';
                notify({ type: 'success', message: t('pixel.generated', 'Image generated') });
            } catch (err) {
                if (statusEl) statusEl.textContent = '';
                if (err.name !== 'AbortError') notify({ type: 'error', message: t('pixel.error_generate', 'Generation failed') });
            } finally { abortCtrl = null; }
        }

        async function aiEnhance() {
            const prompt = (host.querySelector('[data-enhance-prompt]') || {}).value || '';
            const strength = parseFloat((host.querySelector('[data-enhance-strength]') || {}).value) || 0.7;
            if (!canvas.width) return;
            const statusEl = host.querySelector('[data-ai-status]');
            if (statusEl) statusEl.textContent = t('pixel.generating', 'Enhancing...');
            try {
                const dataURL = canvas.toDataURL('image/png');
                abortCtrl = new AbortController();
                const resp = await api('/api/pixel/enhance', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ source_data: dataURL, prompt, strength }),
                    signal: abortCtrl.signal
                });
                if (resp && resp.url) {
                    await loadImageToCanvas(resp.url);
                    filePath = resp.path || '';
                    fileName = filePath ? filePath.split('/').pop() : '';
                }
                if (statusEl) statusEl.textContent = '';
                notify({ type: 'success', message: t('pixel.enhanced', 'Image enhanced') });
            } catch (err) {
                if (statusEl) statusEl.textContent = '';
                if (err.name !== 'AbortError') notify({ type: 'error', message: t('pixel.error_generate', 'Enhancement failed') });
            } finally { abortCtrl = null; }
        }

        // --- Crop mouse events ---
        function onCropMouseDown(e) {
            if (!cropState || !cropState.active) return;
            e.preventDefault();
            const rect = canvas.getBoundingClientRect();
            cropState.startX = e.clientX - rect.left;
            cropState.startY = e.clientY - rect.top;
            cropState.endX = cropState.startX;
            cropState.endY = cropState.startY;
            cropState.canvasLeft = rect.left - canvasArea.getBoundingClientRect().left + canvasArea.scrollLeft;
            cropState.canvasTop = rect.top - canvasArea.getBoundingClientRect().top + canvasArea.scrollTop;
            updateCropSelection();
            const onMove = ev => {
                cropState.endX = Math.max(0, Math.min(rect.width, ev.clientX - rect.left));
                cropState.endY = Math.max(0, Math.min(rect.height, ev.clientY - rect.top));
                updateCropSelection();
            };
            const onUp = () => { document.removeEventListener('mousemove', onMove); document.removeEventListener('mouseup', onUp); };
            document.addEventListener('mousemove', onMove);
            document.addEventListener('mouseup', onUp);
        }

        function updateCropSelection() {
            const sel = host.querySelector('[data-crop-sel]');
            if (!sel || !cropState) return;
            const x = Math.min(cropState.startX, cropState.endX) + (cropState.canvasLeft || 0);
            const y = Math.min(cropState.startY, cropState.endY) + (cropState.canvasTop || 0);
            const w = Math.abs(cropState.endX - cropState.startX);
            const h = Math.abs(cropState.endY - cropState.startY);
            sel.style.left = x + 'px';
            sel.style.top = y + 'px';
            sel.style.width = w + 'px';
            sel.style.height = h + 'px';
        }

        // --- Event wiring ---
        host.querySelector('[data-action="open"]').addEventListener('click', openFile);
        host.querySelector('[data-action="save"]').addEventListener('click', saveFile);
        host.querySelector('[data-action="export"]').addEventListener('click', exportFile);
        host.querySelector('[data-action="undo"]').addEventListener('click', undo);
        host.querySelector('[data-action="redo"]').addEventListener('click', redo);
        host.querySelector('[data-action="zoom-in"]').addEventListener('click', () => zoomTo(zoom * 1.25));
        host.querySelector('[data-action="zoom-out"]').addEventListener('click', () => zoomTo(zoom / 1.25));
        host.querySelector('[data-action="zoom-fit"]').addEventListener('click', zoomFit);
        host.querySelectorAll('.pixel-tab').forEach(b => b.addEventListener('click', () => showPanel(b.dataset.panel)));
        host.querySelector('[data-action="apply-adjust"]').addEventListener('click', applyAdjustments);
        host.querySelector('[data-action="reset-adjust"]').addEventListener('click', resetAdjustments);
        host.querySelectorAll('[data-filter]').forEach(b => b.addEventListener('click', () => applyFilter(b.dataset.filter)));
        host.querySelector('[data-action="rotate-cw"]').addEventListener('click', () => rotateCanvas(90));
        host.querySelector('[data-action="rotate-ccw"]').addEventListener('click', () => rotateCanvas(-90));
        host.querySelector('[data-action="flip-h"]').addEventListener('click', () => flipCanvas(true));
        host.querySelector('[data-action="flip-v"]').addEventListener('click', () => flipCanvas(false));
        host.querySelector('[data-action="crop"]').addEventListener('click', startCrop);
        host.querySelector('[data-action="apply-crop"]').addEventListener('click', applyCrop);
        host.querySelector('[data-action="cancel-crop"]').addEventListener('click', cancelCrop);
        host.querySelector('[data-action="resize"]').addEventListener('click', resizeImage);
        host.querySelector('[data-action="ai-generate"]').addEventListener('click', aiGenerate);
        host.querySelector('[data-action="ai-enhance"]').addEventListener('click', aiEnhance);
        state._cropMouseDown = onCropMouseDown;
        cropOverlay.addEventListener('mousedown', state._cropMouseDown);

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

        // Keyboard shortcuts
        state._keydown = e => {
            if (state.disposed) return;
            if (e.target.closest('input, textarea, select')) return;
            if (e.ctrlKey || e.metaKey) {
                if (e.key === 'o') { e.preventDefault(); openFile(); }
                if (e.key === 's' && e.shiftKey) { e.preventDefault(); saveFileAs(); }
                else if (e.key === 's') { e.preventDefault(); saveFile(); }
                if (e.key === 'z' && e.shiftKey) { e.preventDefault(); redo(); }
                else if (e.key === 'z') { e.preventDefault(); undo(); }
                if (e.key === '=' || e.key === '+') { e.preventDefault(); zoomTo(zoom * 1.25); }
                if (e.key === '-') { e.preventDefault(); zoomTo(zoom / 1.25); }
                if (e.key === '0') { e.preventDefault(); zoomFit(); }
            }
            if (e.key === 'Escape' && cropState) cancelCrop();
            if (e.key === 'Delete' && cropState) cancelCrop();
        };
        document.addEventListener('keydown', state._keydown);

        // Drop support
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
                    { id: 'export', labelKey: 'pixel.export', icon: 'download', action: exportFile }
                ]},
                { id: 'edit', labelKey: 'desktop.menu_edit', items: [
                    { id: 'undo', labelKey: 'pixel.undo', icon: 'undo', shortcut: 'Ctrl+Z', action: undo },
                    { id: 'redo', labelKey: 'pixel.redo', icon: 'redo', shortcut: 'Ctrl+Shift+Z', action: redo }
                ]},
                { id: 'view', labelKey: 'desktop.menu_view', items: [
                    { id: 'zoom-in', labelKey: 'pixel.zoom_in', icon: 'zoom-in', shortcut: 'Ctrl++', action: () => zoomTo(zoom * 1.25) },
                    { id: 'zoom-out', labelKey: 'pixel.zoom_out', icon: 'zoom-out', shortcut: 'Ctrl+-', action: () => zoomTo(zoom / 1.25) },
                    { id: 'zoom-fit', labelKey: 'pixel.zoom_fit', icon: 'maximize', shortcut: 'Ctrl+0', action: zoomFit },
                    { id: 'zoom-100', labelKey: 'pixel.zoom_100', icon: 'zoom-in', action: () => zoomTo(1) }
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

        // Load initial image if path was provided
        if (filePath) {
            api('/api/desktop/preview?path=' + encodeURIComponent(filePath)).then(r => {
                if (r && r.url) loadImageToCanvas(r.url);
            }).catch(() => {});
        }

        state.dispose = function () {
            state.disposed = true;
            if (abortCtrl) abortCtrl.abort();
            if (state._keydown) document.removeEventListener('keydown', state._keydown);
            if (state._cropMouseDown) cropOverlay.removeEventListener('mousedown', state._cropMouseDown);
            history = [];
            originalImage = null;
            canvas = null;
            cctx = null;
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
