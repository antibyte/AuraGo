(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installCanvas = function installCanvas(runtime) {
        Object.assign(runtime, {
            setStatus: Pixel.bindRuntime(runtime, function setStatus(msg) {                     if (this.statusText) this.statusText.textContent = msg || '';
            }),
            updateStatus: Pixel.bindRuntime(runtime, function updateStatus() {
                                if (!this.imgWidth || !this.imgHeight) { this.setStatus(this.t('pixel.status_ready')); return; }
                                const parts = [`${this.imgWidth} × ${this.imgHeight}`];
                                if (this.filePath) parts.push(this.filePath.split('/').pop());
                                if (this.isDirty) parts.push('● ' + this.t('pixel.unsaved_changes'));
                                this.setStatus(parts.join('  ·  '));
                                if (this.statusTool) this.statusTool.textContent = this.activeTool ? this.toolLabel(this.activeTool) : '';
            }),
            updateZoomLabel: Pixel.bindRuntime(runtime, function updateZoomLabel() {                     if (this.zoomLabel) this.zoomLabel.textContent = Math.round(this.zoom * 100) + '%';
            }),
            canvasCoords: Pixel.bindRuntime(runtime, function canvasCoords(e) {
                                const rect = this.canvas.getBoundingClientRect();
                                return {
                                    x: Math.round((e.clientX - rect.left) * (this.canvas.width / rect.width)),
                                    y: Math.round((e.clientY - rect.top) * (this.canvas.height / rect.height))
                                };
            }),
            undo: Pixel.bindRuntime(runtime, function undo() {
                                if (this.historyIdx <= 0) return;
                                this.historyIdx--;
                                this.restoreHistory(this.history[this.historyIdx]);
                                this.updateStatus();
                                if (this.refreshHistoryPanel) this.refreshHistoryPanel();
                                this.emit('history-changed');
            }),
            redo: Pixel.bindRuntime(runtime, function redo() {
                                if (this.historyIdx >= this.history.length - 1) return;
                                this.historyIdx++;
                                this.restoreHistory(this.history[this.historyIdx]);
                                this.updateStatus();
                                if (this.refreshHistoryPanel) this.refreshHistoryPanel();
                                this.emit('history-changed');
            }),
            applyZoom: Pixel.bindRuntime(runtime, function applyZoom() {
                                const w = this.imgWidth * this.zoom;
                                const h = this.imgHeight * this.zoom;
                                this.canvas.style.width = w + 'px';
                                this.canvas.style.height = h + 'px';
                                this.overlayCanvas.style.width = w + 'px';
                                this.overlayCanvas.style.height = h + 'px';
                                this.canvasWrap.style.width = w + 'px';
                                this.canvasWrap.style.height = h + 'px';
                                this.updateZoomLabel();
                                if (this.refreshStickerControls) this.refreshStickerControls();
            }),
            zoomTo: Pixel.bindRuntime(runtime, function zoomTo(z) {
                                this.zoom = Math.max(0.05, Math.min(20, z));
                                this.applyZoom();
            }),
            zoomFit: Pixel.bindRuntime(runtime, function zoomFit() {
                                if (!this.imgWidth || !this.imgHeight) return;
                                const areaW = this.canvasArea.clientWidth - 20;
                                const areaH = this.canvasArea.clientHeight - 20;
                                this.zoom = Math.min(areaW / this.imgWidth, areaH / this.imgHeight, 1);
                                this.applyZoom();
            }),
            loadImage: Pixel.bindRuntime(runtime, function loadImage(src) {
                                return new Promise((resolve, reject) => {
                                    const img = new Image();
                                    img.crossOrigin = 'anonymous';
                                    img.onload = () => resolve(img);
                                    img.onerror = () => reject(new Error(this.t('pixel.error_load')));
                                    img.src = src;
                                });
            }),
            loadImageToCanvas: Pixel.bindRuntime(runtime, async function loadImageToCanvas(src, current) {
                                this.resetStickerDocument();
                                const revision = this.state.documentRevision;
                                const img = await this.loadImage(src);
                                if (this.state.disposed || revision !== this.state.documentRevision || (current && !current())) return false;
                                this.beginDocumentReplace();
                                this.canvas.width = img.naturalWidth;
                                this.canvas.height = img.naturalHeight;
                                this.overlayCanvas.width = img.naturalWidth;
                                this.overlayCanvas.height = img.naturalHeight;
                                this.imgWidth = img.naturalWidth;
                                this.imgHeight = img.naturalHeight;
                                this.layers = [this.createLayer({ name: this.t('pixel.layer_background'), source: img })];
                                this.activeLayerIdx = 0;
                                this.history = [];
                                this.historyIdx = -1;
                                this.selection = null;
                                this.pushHistory('open');
                                this.canvasWrap.hidden = false;
                                this.emptyState.hidden = true;
                                this.clearOverlay();
                                this.captureAdjustmentSource();
                                this.compositeLayers();
                                this.zoomFit();
                                this.updateStatus();
                                this.refreshLayerPanel();
                                this.syncActiveLayer();
                                this.notifyLayersChanged();
                                this.emit('document-changed', { width: this.imgWidth, height: this.imgHeight, reason: 'open' });
                                return true;
            }),
            newBlankCanvas: Pixel.bindRuntime(runtime, function newBlankCanvas(w, h) {
                                this.resetStickerDocument();
                                this.beginDocumentReplace();
                                this.canvas.width = w;
                                this.canvas.height = h;
                                this.overlayCanvas.width = w;
                                this.overlayCanvas.height = h;
                                this.imgWidth = w;
                                this.imgHeight = h;
                                const background = this.createLayer({ name: this.t('pixel.layer_background') });
                                const bgCtx = background.canvas.getContext('2d');
                                bgCtx.fillStyle = '#ffffff';
                                bgCtx.fillRect(0, 0, w, h);
                                this.layers = [background];
                                this.activeLayerIdx = 0;
                                this.history = [];
                                this.historyIdx = -1;
                                this.selection = null;
                                this.pushHistory('new');
                                this.canvasWrap.hidden = false;
                                this.emptyState.hidden = true;
                                this.clearOverlay();
                                this.captureAdjustmentSource();
                                this.compositeLayers();
                                this.zoomFit();
                                this.updateStatus();
                                this.refreshLayerPanel();
                                this.syncActiveLayer();
                                this.notifyLayersChanged();
                                this.emit('document-changed', { width: w, height: h, reason: 'new' });
            }),
            // Drops everything that belongs to the old document before its layers are replaced.
            beginDocumentReplace: Pixel.bindRuntime(runtime, function beginDocumentReplace() {
                                this.resetFilterPreview();
                                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                                this.resetAdjustmentControls();
                                this.adjustPreview = null;
                                this.originalImage = null;
                                this.adjustSourceKey = '';
                                this.editMaskMode = false;
                                this.selImageData = null;
                                this.moveSelFloating = false;
                                this.stopMarchingAnts();
                                this.invalidateCompositeCaches();
            }),
            buildFilterString: Pixel.bindRuntime(runtime, function buildFilterString() {
                                const parts = [];
                                const b = 1 + this.adjustments.brightness / 100;
                                const c = 1 + this.adjustments.contrast / 100;
                                const s = 1 + this.adjustments.saturation / 100;
                                const e = 1 + this.adjustments.exposure / 200;
                                if (b !== 1) parts.push(`brightness(${b})`);
                                if (c !== 1) parts.push(`contrast(${c})`);
                                if (s !== 1) parts.push(`saturate(${s})`);
                                if (e !== 1) parts.push(`brightness(${e})`);
                                if (this.adjustments.sharpness > 0) parts.push(`contrast(${1 + this.adjustments.sharpness / 200})`);
                                return parts.join(' ') || 'none';
            }),
            // Copy of the active layer (or mask) that the adjust sliders always start from. Skipped while the layer is unchanged.
            captureAdjustmentSource: Pixel.bindRuntime(runtime, function captureAdjustmentSource() {
                const layer = this.activeLayer();
                if (!this.imgWidth || !layer || this.isStickerLayer()) { this.originalImage = null; this.adjustSourceKey = ''; return; }
                const inMask = !!(this.editMaskMode && layer.maskCanvas);
                const key = layer.id + (inMask ? 'm' : 'p') + ':' + (inMask ? layer.maskRev : layer.rev) + ':' + this.imgWidth + 'x' + this.imgHeight;
                if (this.originalImage && this.adjustSourceKey === key) return;
                const source = document.createElement('canvas'); source.width = this.imgWidth; source.height = this.imgHeight;
                source.getContext('2d', { willReadFrequently: true }).drawImage(this.getActiveReadCtx().canvas, 0, 0);
                this.originalImage = source;
                this.adjustSourceKey = key;
            }),
            hasPendingAdjustments: Pixel.bindRuntime(runtime, function hasPendingAdjustments() {
                                return Object.keys(this.adjustments).some(k => Number(this.adjustments[k]) !== 0);
            }),
            // Removes the adjust preview from its layer; the layer pixels were never touched.
            dropAdjustPreview: Pixel.bindRuntime(runtime, function dropAdjustPreview() {
                                const preview = this.adjustPreview;
                                this.adjustPreview = null;
                                if (preview && preview.layer.preview === preview.canvas) this.setLayerPreview(preview.layer, null);
            }),
            // Bakes a pending adjust or filter preview into the layer so the next edit starts from what is on screen.
            commitPendingPreviews: Pixel.bindRuntime(runtime, function commitPendingPreviews() {
                                if (this.compareMode) this.exitCompareMode({ preservePreview: true });
                                if (this.filterPreview) { this.applyFilter(); return true; }
                                if (this.adjustPreview && this.hasPendingAdjustments()) { this.applyAdjustments(); return true; }
                                return false;
            }),
            // Pending adjust/filter changes are thrown away (layer about to disappear or be replaced).
            discardPendingPreviews: Pixel.bindRuntime(runtime, function discardPendingPreviews() {
                                this.resetFilterPreview();
                                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                                this.dropAdjustPreview();
                                this.resetAdjustmentControls();
            }),
            // Adjustments and filters work on layer pixels, so mask-edit mode ends when they start.
            leaveMaskEdit: Pixel.bindRuntime(runtime, function leaveMaskEdit() {
                                if (!this.editMaskMode) return;
                                this.editMaskMode = false;
                                this.refreshLayerPanel();
                                this.notify({ type: 'info', message: this.t('pixel.mask_edit_off') });
            }),
            applyAdjustmentsPreview: Pixel.bindRuntime(runtime, function applyAdjustmentsPreview() {
                                if (!this.requireRasterLayer()) return;
                                if (!this.canvas.width) return;
                                const layer = this.activeLayer();
                                if (this.editMaskMode) this.leaveMaskEdit();
                                if (this.adjustPreview && this.adjustPreview.layer !== layer) this.dropAdjustPreview();
                                this.captureAdjustmentSource();
                                if (!this.originalImage) return;
                                if (!this.hasPendingAdjustments()) { this.dropAdjustPreview(); return; }
                                const tmpCanvas = this.acquireTempCanvas(this.imgWidth, this.imgHeight);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                tmpCtx.filter = this.buildFilterString();
                                tmpCtx.drawImage(this.originalImage, 0, 0, this.imgWidth, this.imgHeight);
                                this.applyCustomAdjustments(tmpCtx);
                                let preview = this.adjustPreview;
                                if (!preview || preview.layer !== layer || preview.canvas.width !== this.imgWidth || preview.canvas.height !== this.imgHeight) {
                                    this.dropAdjustPreview();
                                    preview = this.adjustPreview = { layer, canvas: this.makeCanvas(this.imgWidth, this.imgHeight) };
                                }
                                const pctx = preview.canvas.getContext('2d');
                                pctx.clearRect(0, 0, preview.canvas.width, preview.canvas.height);
                                pctx.drawImage(tmpCanvas, 0, 0);
                                this.releaseTempCanvas(tmpCanvas);
                                this.setLayerPreview(layer, preview.canvas);
            }),
            applyCustomAdjustments: Pixel.bindRuntime(runtime, function applyCustomAdjustments(ctx) {
                                if (this.adjustments.temperature === 0 && this.adjustments.shadows === 0 && this.adjustments.highlights === 0) return;
                                const imgData = ctx.getImageData(0, 0, this.canvas.width, this.canvas.height);
                                const d = imgData.data;
                                const temp = this.adjustments.temperature * 0.5;
                                const shadows = this.adjustments.shadows * 0.5;
                                const highlights = this.adjustments.highlights * 0.5;
                                for (let i = 0; i < d.length; i += 4) {
                                    d[i] = this.clamp255(d[i] + temp);
                                    d[i + 2] = this.clamp255(d[i + 2] - temp);
                                    const lum = (d[i] + d[i + 1] + d[i + 2]) / 3;
                                    if (lum < 128) { d[i] = this.clamp255(d[i] + shadows); d[i + 1] = this.clamp255(d[i + 1] + shadows); d[i + 2] = this.clamp255(d[i + 2] + shadows); }
                                    if (lum > 128) { d[i] = this.clamp255(d[i] + highlights); d[i + 1] = this.clamp255(d[i + 1] + highlights); d[i + 2] = this.clamp255(d[i + 2] + highlights); }
                                }
                                ctx.putImageData(imgData, 0, 0);
            }),
            applyAdjustments: Pixel.bindRuntime(runtime, function applyAdjustments() {
                                if (!this.requireRasterLayer()) return;
                                if (!this.originalImage) return;
                                if (this.compareMode) this.exitCompareMode({ preservePreview: true });
                                else this.applyAdjustmentsPreview();
                                const layer = this.activeLayer();
                                const preview = this.adjustPreview;
                                if (preview && preview.layer === layer && layer.preview === preview.canvas) {
                                    this.commitLayerPixels(layer, preview.canvas, { label: 'adjust' });
                                }
                                this.dropAdjustPreview();
                                this.captureAdjustmentSource();
                                this.resetAdjustmentControls();
            }),
            resetAdjustmentControls: Pixel.bindRuntime(runtime, function resetAdjustmentControls() {
                                for (const k in this.adjustments) this.adjustments[k] = 0;
                                this.host.querySelectorAll('[data-adjust]').forEach(s => { s.value = 0; });
                                this.host.querySelectorAll('[data-val-brightness],[data-val-contrast],[data-val-saturation],[data-val-exposure],[data-val-sharpness],[data-val-temperature],[data-val-shadows],[data-val-highlights]').forEach(el => { el.textContent = '0'; });
            }),
            resetAdjustments: Pixel.bindRuntime(runtime, function resetAdjustments() {
                                if (!this.requireRasterLayer()) return;
                                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                                this.resetAdjustmentControls();
                                this.dropAdjustPreview();
            }),
            toggleCompare: Pixel.bindRuntime(runtime, function toggleCompare() {
                                if (!this.requireRasterLayer()) return;
                                if (!this.originalImage || !this.canvas.width) return;
                                if (this.compareMode) {
                                    this.exitCompareMode({ preservePreview: true });
                                } else {
                                    this.compareMode = true;
                                    this.comparePos = 0.5;
                                    this.compareOrigData = this.flattenDocument();
                                    this.applyAdjustmentsPreview();
                                    this.renderCompare();
                                    this.compareDivider.hidden = false;
                                }
            }),
            exitCompareMode: Pixel.bindRuntime(runtime, function exitCompareMode(options) {
                                if (!this.compareMode) return;
                                const preservePreview = !!(options && options.preservePreview);
                                this.compareMode = false;
                                this.compareOrigData = null;
                                if (this.compareDivider) this.compareDivider.hidden = true;
                                this.clearOverlay();
                                if (preservePreview && this.hasPendingAdjustments()) {
                                    this.applyAdjustmentsPreview();
                                    return;
                                }
                                this.dropAdjustPreview();
                                this.compositeLayers();
            }),
            renderCompare: Pixel.bindRuntime(runtime, function renderCompare() {
                                if (!this.compareMode || !this.compareOrigData) return;
                                const splitX = Math.round(this.canvas.width * this.comparePos);
                                // Right of the divider shows the composite with the preview, left of it the untouched document.
                                this.compositeLayers();
                                this.cctx.save();
                                this.cctx.beginPath();
                                this.cctx.rect(0, 0, splitX, this.canvas.height);
                                this.cctx.clip();
                                this.cctx.clearRect(0, 0, splitX, this.canvas.height);
                                this.cctx.drawImage(this.compareOrigData, 0, 0);
                                this.cctx.restore();
                                this.requestOverlay();
            }),
            // Divider line of the before/after compare (overlay z 70); drawn while compareMode is on.
            drawCompareDivider: Pixel.bindRuntime(runtime, function drawCompareDivider(ctx) {
                if (!this.compareMode || !this.compareOrigData) return;
                const splitX = Math.round(this.canvas.width * this.comparePos);
                ctx.strokeStyle = '#ffffff';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.moveTo(splitX, 0);
                ctx.lineTo(splitX, this.canvas.height);
                ctx.stroke();
                ctx.strokeStyle = '#000000';
                ctx.lineWidth = 1;
                ctx.beginPath();
                ctx.moveTo(splitX + 1, 0);
                ctx.lineTo(splitX + 1, this.canvas.height);
                ctx.stroke();
            }),
            transformDocument: Pixel.bindRuntime(runtime, function transformDocument(matrix, width, height) {
                this.cancelStickerGesture();
                this.commitPendingPreviews();
                // Masks keep revealing the area a canvas expansion adds, hence the white underlay for them.
                const transform = (canvas, underlay) => {
                    if (!canvas) return;
                    const next = this.acquireTempCanvas(width, height), ctx = next.getContext('2d');
                    if (underlay) { ctx.fillStyle = '#ffffff'; ctx.fillRect(0, 0, width, height); }
                    ctx.imageSmoothingQuality = 'high'; ctx.setTransform(...matrix); ctx.drawImage(canvas, 0, 0);
                    canvas.width = width; canvas.height = height;
                    canvas.getContext('2d').drawImage(next, 0, 0); this.releaseTempCanvas(next);
                };
                this.layers.forEach(layer => {
                    if (!layer.sticker) transform(layer.canvas, false);
                    if (layer.maskCanvas) { transform(layer.maskCanvas, true); layer.maskRev = this.nextRev(); }
                    layer.rev = this.nextRev();
                    layer.preview = null;
                });
                this.canvas.width = this.overlayCanvas.width = this.imgWidth = width;
                this.canvas.height = this.overlayCanvas.height = this.imgHeight = height;
                this.invalidateCompositeCaches();
                this.transformStickerLayers(matrix);
                this.deselect(); this.compositeLayers();
                this.captureAdjustmentSource();
                this.notifyLayersChanged();
                this.emit('document-changed', { width, height, reason: 'transform' });
            }),
            rotateCanvas: Pixel.bindRuntime(runtime, function rotateCanvas(deg) {
                if (!this.imgWidth || ![90, -90, 180].includes(deg)) return;
                const w = this.imgWidth, h = this.imgHeight;
                const matrix = deg === 90 ? [0, 1, -1, 0, h, 0] : deg === -90 ? [0, -1, 1, 0, 0, w] : [-1, 0, 0, -1, w, h];
                this.transformDocument(matrix, deg === 180 ? w : h, deg === 180 ? h : w);
                this.pushHistory('rotate ' + deg); this.applyZoom();
            }),
            flipCanvas: Pixel.bindRuntime(runtime, function flipCanvas(horizontal) {
                if (!this.imgWidth) return;
                const w = this.imgWidth, h = this.imgHeight;
                this.transformDocument(horizontal ? [-1, 0, 0, 1, w, 0] : [1, 0, 0, -1, 0, h], w, h);
                this.pushHistory(horizontal ? 'flip-h' : 'flip-v');
            }),
            startCrop: Pixel.bindRuntime(runtime, function startCrop() {
                                if (!this.canvas.width) return;
                                this.cropState = { active: true, startX: 0, startY: 0, endX: 0, endY: 0 };
                                this.refreshStickerControls();
                                this.cropOverlay.hidden = false;
                                this.cropOverlay.innerHTML = '<div class="pixel-crop-selection" data-crop-sel></div>';
                                this.setStatus(this.t('pixel.crop_hint'));
            }),
            cancelCrop: Pixel.bindRuntime(runtime, function cancelCrop() {
                                this.cropState = null;
                                this.refreshStickerControls();
                                this.cropOverlay.hidden = true;
                                this.cropOverlay.innerHTML = '';
                                const ca = this.host.querySelector('[data-crop-actions]');
                                if (ca) ca.hidden = true;
                                this.updateStatus();
            }),
            applyCrop: Pixel.bindRuntime(runtime, function applyCrop() {
                                if (!this.cropState) return;
                                const sel = this.host.querySelector('[data-crop-sel]');
                                if (!sel) { this.cancelCrop(); return; }
                                const displayW = parseFloat(this.canvas.style.width) || this.canvas.width;
                                const displayH = parseFloat(this.canvas.style.height) || this.canvas.height;
                                const scaleX = this.canvas.width / displayW;
                                const scaleY = this.canvas.height / displayH;
                                const sx = Math.max(0, Math.round(Math.min(this.cropState.startX, this.cropState.endX) * scaleX));
                                const sy = Math.max(0, Math.round(Math.min(this.cropState.startY, this.cropState.endY) * scaleY));
                                const sw = Math.round(Math.abs(this.cropState.endX - this.cropState.startX) * scaleX);
                                const sh = Math.round(Math.abs(this.cropState.endY - this.cropState.startY) * scaleY);
                                if (sw < 2 || sh < 2) { this.cancelCrop(); return; }
                                this.transformDocument([1, 0, 0, 1, -sx, -sy], sw, sh);
                                this.pushHistory('crop');
                                this.cancelCrop();
                                this.zoomFit();
            }),
            expandCanvasToFit: Pixel.bindRuntime(runtime, function expandCanvasToFit(nw, nh) {
                                if (!this.canvas.width) return;
                                nw = Math.max(this.canvas.width, nw);
                                nh = Math.max(this.canvas.height, nh);
                                if (nw === this.canvas.width && nh === this.canvas.height) return;
                                this.transformDocument([1, 0, 0, 1, 0, 0], nw, nh);
                                this.applyZoom();
            }),
            resizeImage: Pixel.bindRuntime(runtime, function resizeImage() {
                                if (!this.canvas.width) return;
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<form class="vd-modal" role="dialog">
                                    <div class="vd-modal-title">${this.esc(this.t('pixel.resize'))}</div>
                                    <div class="pixel-resize-form">
                                        <label class="pixel-label">${this.esc(this.t('pixel.width'))}</label>
                                        <input class="vd-modal-input" type="number" data-resize-w value="${this.imgWidth}" min="1">
                                        <label class="pixel-label">${this.esc(this.t('pixel.height'))}</label>
                                        <input class="vd-modal-input" type="number" data-resize-h value="${this.imgHeight}" min="1">
                                        <label class="pixel-checkbox-row"><input type="checkbox" data-lock-ratio checked> ${this.esc(this.t('pixel.lock_ratio'))}</label>
                                    </div>
                                    <div class="vd-modal-actions">
                                        <button type="button" class="vd-button" data-cancel>${this.esc(this.t('pixel.cancel'))}</button>
                                        <button type="submit" class="vd-button vd-button-primary">${this.esc(this.t('pixel.apply'))}</button>
                                    </div>
                                </form>`;
                                document.body.appendChild(dlg);
                                const wInput = dlg.querySelector('[data-resize-w]');
                                const hInput = dlg.querySelector('[data-resize-h]');
                                const lock = dlg.querySelector('[data-lock-ratio]');
                                const ratio = this.imgWidth / this.imgHeight;
                                wInput.addEventListener('input', () => { if (lock.checked) hInput.value = Math.round(wInput.value / ratio); });
                                hInput.addEventListener('input', () => { if (lock.checked) wInput.value = Math.round(hInput.value * ratio); });
                                dlg.querySelector('form').addEventListener('submit', e => {
                                    e.preventDefault();
                                    const nw = parseInt(wInput.value) || this.imgWidth;
                                    const nh = parseInt(hInput.value) || this.imgHeight;
                                    if (nw < 1 || nh < 1) return;
                                    this.transformDocument([nw / this.imgWidth, 0, 0, nh / this.imgHeight, 0, 0], nw, nh);
                                    this.pushHistory('resize');
                                    this.zoomFit();
                                    dlg.remove();
                                });
                                dlg.querySelector('[data-cancel]').addEventListener('click', () => dlg.remove());
                                dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            })
        });

        runtime.registerOverlay('compare', ctx => runtime.drawCompareDivider(ctx), 70);
    };
})();
