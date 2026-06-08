(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installCanvas = function installCanvas(runtime) {
        Object.assign(runtime, {
            setStatus: Pixel.bindRuntime(runtime, function setStatus(msg) {                     if (this.statusText) this.statusText.textContent = msg || '';
            }),
            updateStatus: Pixel.bindRuntime(runtime, function updateStatus() {
                                if (!this.imgWidth || !this.imgHeight) { this.setStatus(this.t('pixel.status_ready', 'Ready')); return; }
                                const parts = [`${this.imgWidth} × ${this.imgHeight}`];
                                if (this.filePath) parts.push(this.filePath.split('/').pop());
                                if (this.isDirty) parts.push('● ' + this.t('pixel.unsaved_changes', 'Unsaved'));
                                this.setStatus(parts.join('  ·  '));
                                if (this.statusTool) this.statusTool.textContent = this.activeTool ? this.t('pixel.' + this.activeTool, this.activeTool) : '';
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
            getActiveCtx: Pixel.bindRuntime(runtime, function getActiveCtx() {
                                if (this.layers.length === 1 || !this.layers[this.activeLayerIdx].canvas) return this.cctx;
                                return this.layers[this.activeLayerIdx].canvas.getContext('2d');
            }),
            ensureBackgroundMigrated: Pixel.bindRuntime(runtime, function ensureBackgroundMigrated() {
                                if (!this.canvas.width || !this.canvas.height) return;
                                if (this.layers[0].canvas) return;
                                const bgCanvas = this.acquireTempCanvas(this.imgWidth || this.canvas.width, this.imgHeight || this.canvas.height);
                                bgCanvas.getContext('2d').drawImage(this.canvas, 0, 0);
                                this.layers[0].canvas = bgCanvas;
            }),
            compositeLayers: Pixel.bindRuntime(runtime, function compositeLayers() {
                                this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                for (let i = 0; i < this.layers.length; i++) {
                                    const layer = this.layers[i];
                                    if (!layer.visible) continue;
                                    this.cctx.globalAlpha = layer.opacity;
                                    if (layer.canvas) {
                                        this.cctx.drawImage(layer.canvas, 0, 0);
                                    } else if (i === 0 && this.layers.length === 1) {
                                    }
                                }
                                this.cctx.globalAlpha = 1;
            }),
            pushHistory: Pixel.bindRuntime(runtime, function pushHistory(label) {
                                if (!this.canvas.width || !this.canvas.height) return;
                                const layerStates = this.layers.map(l => {
                                    const src = l.canvas || (this.layers.length > 1 ? null : this.canvas);
                                    if (!src) return null;
                                    const tmp = this.acquireTempCanvas(src.width || this.canvas.width, src.height || this.canvas.height);
                                    tmp.getContext('2d').drawImage(src, 0, 0);
                                    return tmp;
                                });
                                if (this.historyIdx < this.history.length - 1) this.history = this.history.slice(0, this.historyIdx + 1);
                                this.history.push({ layerStates, width: this.canvas.width, height: this.canvas.height, label: label || '', layerMeta: this.layers.map(l => ({ name: l.name, visible: l.visible, opacity: l.opacity })) });
                                if (this.history.length > this.MAX_HISTORY) {
                                    const removed = this.history.shift();
                                    removed.layerStates.forEach(s => { if (s) this.releaseTempCanvas(s); });
                                }
                                this.historyIdx = this.history.length - 1;
                                this.isDirty = true;
                                this.updateStatus();
            }),
            restoreHistory: Pixel.bindRuntime(runtime, function restoreHistory(entry) {
                                this.canvas.width = entry.width;
                                this.canvas.height = entry.height;
                                this.imgWidth = entry.width;
                                this.imgHeight = entry.height;
                                this.overlayCanvas.width = entry.width;
                                this.overlayCanvas.height = entry.height;
                                this.layers = entry.layerMeta.map((meta, i) => ({
                                    canvas: entry.layerStates[i] ? (() => { const tc = this.acquireTempCanvas(entry.width, entry.height); tc.getContext('2d').drawImage(entry.layerStates[i], 0, 0); return tc; })() : null,
                                    name: meta.name,
                                    visible: meta.visible,
                                    opacity: meta.opacity
                                }));
                                this.activeLayerIdx = Math.min(this.activeLayerIdx, this.layers.length - 1);
                                this.compositeLayers();
                                this.applyZoom();
            }),
            undo: Pixel.bindRuntime(runtime, function undo() {
                                if (this.historyIdx <= 0) return;
                                this.historyIdx--;
                                this.restoreHistory(this.history[this.historyIdx]);
                                this.updateStatus();
            }),
            redo: Pixel.bindRuntime(runtime, function redo() {
                                if (this.historyIdx >= this.history.length - 1) return;
                                this.historyIdx++;
                                this.restoreHistory(this.history[this.historyIdx]);
                                this.updateStatus();
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
                                    img.onerror = () => reject(new Error('Failed to load image'));
                                    img.src = src;
                                });
            }),
            loadImageToCanvas: Pixel.bindRuntime(runtime, async function loadImageToCanvas(src) {
                                const img = await this.loadImage(src);
                                this.originalImage = img;
                                this.canvas.width = img.naturalWidth;
                                this.canvas.height = img.naturalHeight;
                                this.overlayCanvas.width = img.naturalWidth;
                                this.overlayCanvas.height = img.naturalHeight;
                                this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                this.cctx.drawImage(img, 0, 0);
                                this.imgWidth = img.naturalWidth;
                                this.imgHeight = img.naturalHeight;
                                this.layers = [{ canvas: null, name: this.t('pixel.layer_background', 'Background'), visible: true, opacity: 1 }];
                                this.activeLayerIdx = 0;
                                this.history = [];
                                this.historyIdx = -1;
                                this.pushHistory('open');
                                this.canvasWrap.hidden = false;
                                this.emptyState.hidden = true;
                                this.selection = null;
                                this.clearOverlay();
                                this.zoomFit();
                                this.updateStatus();
            }),
            newBlankCanvas: Pixel.bindRuntime(runtime, function newBlankCanvas(w, h) {
                                this.originalImage = null;
                                this.canvas.width = w;
                                this.canvas.height = h;
                                this.overlayCanvas.width = w;
                                this.overlayCanvas.height = h;
                                this.cctx.fillStyle = '#ffffff';
                                this.cctx.fillRect(0, 0, w, h);
                                this.imgWidth = w;
                                this.imgHeight = h;
                                this.layers = [{ canvas: null, name: this.t('pixel.layer_background', 'Background'), visible: true, opacity: 1 }];
                                this.activeLayerIdx = 0;
                                this.history = [];
                                this.historyIdx = -1;
                                this.pushHistory('new');
                                this.canvasWrap.hidden = false;
                                this.emptyState.hidden = true;
                                this.selection = null;
                                this.clearOverlay();
                                this.zoomFit();
                                this.updateStatus();
            }),
            applyFilterPreview: Pixel.bindRuntime(runtime, function applyFilterPreview(name) {
                                if (!this.canvas.width) return;
                                const isMultiLayer = this.layers.length > 1;
                                if (isMultiLayer) this.ensureBackgroundMigrated();
                                const src = this.originalImage || this.canvas;
                                const tmpCanvas = this.acquireTempCanvas(this.imgWidth, this.imgHeight);
                                const tmpCtx = tmpCanvas.getContext('2d');
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
                                tmpCtx.drawImage(src, 0, 0, this.imgWidth, this.imgHeight);
                                if (name === 'vignette') this.applyVignette(tmpCtx);
                                if (name === 'emboss') this.applyEmboss(tmpCtx);
                                if (isMultiLayer) {
                                    const actx = this.getActiveCtx();
                                    actx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                    actx.drawImage(tmpCanvas, 0, 0);
                                    this.compositeLayers();
                                } else {
                                    this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                    this.cctx.drawImage(tmpCanvas, 0, 0);
                                }
                                this.releaseTempCanvas(tmpCanvas);
                                this.pushHistory('filter:' + name);
            }),
            applyVignette: Pixel.bindRuntime(runtime, function applyVignette(ctx) {
                                const w = this.canvas.width, h = this.canvas.height;
                                const grd = ctx.createRadialGradient(w / 2, h / 2, w * 0.25, w / 2, h / 2, w * 0.7);
                                grd.addColorStop(0, 'rgba(0,0,0,0)');
                                grd.addColorStop(1, 'rgba(0,0,0,0.6)');
                                ctx.fillStyle = grd;
                                ctx.fillRect(0, 0, w, h);
            }),
            applyEmboss: Pixel.bindRuntime(runtime, function applyEmboss(ctx) {
                                const imgData = ctx.getImageData(0, 0, this.canvas.width, this.canvas.height);
                                const kernel = [-2, -1, 0, -1, 1, 1, 0, 1, 2];
                                this.convolve(imgData, kernel, this.canvas.width, this.canvas.height);
                                ctx.putImageData(imgData, 0, 0);
            }),
            convolve: Pixel.bindRuntime(runtime, function convolve(imgData, kernel, w, h) {
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
                                        d[idx] = this.clamp255(r + 128); d[idx + 1] = this.clamp255(g + 128); d[idx + 2] = this.clamp255(b + 128);
                                    }
                                }
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
            applyAdjustmentsPreview: Pixel.bindRuntime(runtime, function applyAdjustmentsPreview() {
                                if (!this.originalImage || !this.canvas.width) return;
                                const isMultiLayer = this.layers.length > 1;
                                if (isMultiLayer) this.ensureBackgroundMigrated();
                                const tmpCanvas = this.acquireTempCanvas(this.imgWidth, this.imgHeight);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                tmpCtx.filter = this.buildFilterString();
                                tmpCtx.drawImage(this.originalImage, 0, 0, this.imgWidth, this.imgHeight);
                                this.applyCustomAdjustments(tmpCtx);
                                if (isMultiLayer) {
                                    const actx = this.getActiveCtx();
                                    actx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                    actx.drawImage(tmpCanvas, 0, 0);
                                    this.compositeLayers();
                                } else {
                                    this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                    this.cctx.drawImage(tmpCanvas, 0, 0);
                                }
                                this.releaseTempCanvas(tmpCanvas);
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
                                if (!this.originalImage) return;
                                if (this.compareMode) this.exitCompareMode({ preservePreview: true });
                                else this.applyAdjustmentsPreview();
                                const dataURL = this.canvas.toDataURL();
                                const img = new Image();
                                img.onload = () => { this.originalImage = img; };
                                img.src = dataURL;
                                this.pushHistory('adjust');
                                this.resetAdjustmentControls();
            }),
            resetAdjustmentControls: Pixel.bindRuntime(runtime, function resetAdjustmentControls() {
                                for (const k in this.adjustments) this.adjustments[k] = 0;
                                this.host.querySelectorAll('[data-adjust]').forEach(s => { s.value = 0; });
                                this.host.querySelectorAll('[data-val-brightness],[data-val-contrast],[data-val-saturation],[data-val-exposure],[data-val-sharpness],[data-val-temperature],[data-val-shadows],[data-val-highlights]').forEach(el => { el.textContent = '0'; });
            }),
            resetAdjustments: Pixel.bindRuntime(runtime, function resetAdjustments() {
                                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                                this.resetAdjustmentControls();
                                if (this.originalImage && this.canvas.width) {
                                    if (this.layers.length > 1) {
                                        this.ensureBackgroundMigrated();
                                        const actx = this.layers[0].canvas.getContext('2d');
                                        actx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                        actx.drawImage(this.originalImage, 0, 0);
                                        this.compositeLayers();
                                    } else {
                                        this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                        this.cctx.drawImage(this.originalImage, 0, 0);
                                    }
                                }
            }),
            toggleCompare: Pixel.bindRuntime(runtime, function toggleCompare() {
                                if (!this.originalImage || !this.canvas.width) return;
                                if (this.compareMode) {
                                    this.exitCompareMode({ preservePreview: true });
                                } else {
                                    this.compareMode = true;
                                    this.comparePos = 0.5;
                                    this.compareOrigData = this.cctx.getImageData(0, 0, this.canvas.width, this.canvas.height);
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
                                const hasAdjustments = Object.keys(this.adjustments).some(k => Number(this.adjustments[k]) !== 0);
                                if (preservePreview && hasAdjustments) {
                                    this.applyAdjustmentsPreview();
                                    return;
                                }
                                if (this.originalImage && this.canvas.width) {
                                    if (this.layers.length > 1) {
                                        this.ensureBackgroundMigrated();
                                        const actx = this.layers[0].canvas.getContext('2d');
                                        actx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                        actx.drawImage(this.originalImage, 0, 0);
                                        this.compositeLayers();
                                    } else {
                                        this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                        this.cctx.drawImage(this.originalImage, 0, 0);
                                    }
                                }
            }),
            renderCompare: Pixel.bindRuntime(runtime, function renderCompare() {
                                if (!this.compareMode || !this.compareOrigData) return;
                                const splitX = Math.round(this.canvas.width * this.comparePos);
                                const current = this.cctx.getImageData(0, 0, this.canvas.width, this.canvas.height);
                                this.cctx.putImageData(this.compareOrigData, 0, 0);
                                this.cctx.save();
                                this.cctx.beginPath();
                                this.cctx.rect(splitX, 0, this.canvas.width - splitX, this.canvas.height);
                                this.cctx.clip();
                                this.cctx.putImageData(current, 0, 0);
                                this.cctx.restore();
                                this.olCtx.clearRect(0, 0, this.overlayCanvas.width, this.overlayCanvas.height);
                                this.olCtx.strokeStyle = '#ffffff';
                                this.olCtx.lineWidth = 2;
                                this.olCtx.beginPath();
                                this.olCtx.moveTo(splitX, 0);
                                this.olCtx.lineTo(splitX, this.canvas.height);
                                this.olCtx.stroke();
                                this.olCtx.strokeStyle = '#000000';
                                this.olCtx.lineWidth = 1;
                                this.olCtx.beginPath();
                                this.olCtx.moveTo(splitX + 1, 0);
                                this.olCtx.lineTo(splitX + 1, this.canvas.height);
                                this.olCtx.stroke();
            }),
            rotateCanvas: Pixel.bindRuntime(runtime, function rotateCanvas(deg) {
                                if (!this.canvas.width) return;
                                const nextW = deg === 90 || deg === -90 ? this.canvas.height : this.canvas.width;
                                const nextH = deg === 90 || deg === -90 ? this.canvas.width : this.canvas.height;
                                const tmpCanvas = this.acquireTempCanvas(nextW, nextH);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                tmpCtx.save();
                                if (deg === 90) { tmpCtx.translate(tmpCanvas.width, 0); tmpCtx.rotate(Math.PI / 2); }
                                else if (deg === -90) { tmpCtx.translate(0, tmpCanvas.height); tmpCtx.rotate(-Math.PI / 2); }
                                else if (deg === 180) { tmpCtx.translate(tmpCanvas.width, tmpCanvas.height); tmpCtx.rotate(Math.PI); }
                                this.layers.forEach(l => {
                                    if (l.canvas) {
                                        const lt = this.acquireTempCanvas(nextW, nextH);
                                        const lx = lt.getContext('2d');
                                        lx.save();
                                        if (deg === 90) { lx.translate(nextW, 0); lx.rotate(Math.PI / 2); }
                                        else if (deg === -90) { lx.translate(0, nextH); lx.rotate(-Math.PI / 2); }
                                        else if (deg === 180) { lx.translate(nextW, nextH); lx.rotate(Math.PI); }
                                        lx.drawImage(l.canvas, 0, 0);
                                        lx.restore();
                                        l.canvas.width = nextW;
                                        l.canvas.height = nextH;
                                        l.canvas.getContext('2d').drawImage(lt, 0, 0);
                                        this.releaseTempCanvas(lt);
                                    }
                                });
                                tmpCtx.drawImage(this.canvas, 0, 0);
                                tmpCtx.restore();
                                this.canvas.width = nextW;
                                this.canvas.height = nextH;
                                this.overlayCanvas.width = nextW;
                                this.overlayCanvas.height = nextH;
                                this.cctx.drawImage(tmpCanvas, 0, 0);
                                this.releaseTempCanvas(tmpCanvas);
                                this.imgWidth = nextW;
                                this.imgHeight = nextH;
                                if (this.layers.length > 1) this.compositeLayers();
                                this.originalImage = new Image();
                                this.originalImage.src = this.canvas.toDataURL();
                                this.pushHistory('rotate ' + deg);
                                this.applyZoom();
            }),
            flipCanvas: Pixel.bindRuntime(runtime, function flipCanvas(horizontal) {
                                if (!this.canvas.width) return;
                                this.layers.forEach(l => {
                                    if (!l.canvas) return;
                                    const tmp = this.acquireTempCanvas(l.canvas.width, l.canvas.height);
                                    const tx = tmp.getContext('2d');
                                    tx.save();
                                    if (horizontal) { tx.translate(l.canvas.width, 0); tx.scale(-1, 1); }
                                    else { tx.translate(0, l.canvas.height); tx.scale(1, -1); }
                                    tx.drawImage(l.canvas, 0, 0);
                                    tx.restore();
                                    l.canvas.getContext('2d').clearRect(0, 0, l.canvas.width, l.canvas.height);
                                    l.canvas.getContext('2d').drawImage(tmp, 0, 0);
                                    this.releaseTempCanvas(tmp);
                                });
                                const tmpCanvas = this.acquireTempCanvas(this.canvas.width, this.canvas.height);
                                const tmpCtx = tmpCanvas.getContext('2d');
                                tmpCtx.save();
                                if (horizontal) { tmpCtx.translate(this.canvas.width, 0); tmpCtx.scale(-1, 1); }
                                else { tmpCtx.translate(0, this.canvas.height); tmpCtx.scale(1, -1); }
                                tmpCtx.drawImage(this.canvas, 0, 0);
                                tmpCtx.restore();
                                this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                this.cctx.drawImage(tmpCanvas, 0, 0);
                                this.releaseTempCanvas(tmpCanvas);
                                if (this.layers.length > 1) this.compositeLayers();
                                this.pushHistory(horizontal ? 'flip-h' : 'flip-v');
            }),
            startCrop: Pixel.bindRuntime(runtime, function startCrop() {
                                if (!this.canvas.width) return;
                                this.cropState = { active: true, startX: 0, startY: 0, endX: 0, endY: 0 };
                                this.cropOverlay.hidden = false;
                                this.cropOverlay.innerHTML = '<div class="pixel-crop-this.selection" data-crop-sel></div>';
                                this.host.querySelector('[data-crop-actions]').hidden = false;
                                this.setStatus(this.t('pixel.crop_hint', 'Drag to select crop area'));
            }),
            cancelCrop: Pixel.bindRuntime(runtime, function cancelCrop() {
                                this.cropState = null;
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
                                this.layers.forEach(l => {
                                    if (l.canvas) {
                                        const imgData = l.canvas.getContext('2d').getImageData(sx, sy, sw, sh);
                                        l.canvas.width = sw;
                                        l.canvas.height = sh;
                                        l.canvas.getContext('2d').putImageData(imgData, 0, 0);
                                    }
                                });
                                const imgData = this.cctx.getImageData(sx, sy, sw, sh);
                                this.canvas.width = sw;
                                this.canvas.height = sh;
                                this.overlayCanvas.width = sw;
                                this.overlayCanvas.height = sh;
                                this.cctx.putImageData(imgData, 0, 0);
                                this.imgWidth = sw;
                                this.imgHeight = sh;
                                this.originalImage = null;
                                if (this.layers.length > 1) this.compositeLayers();
                                this.pushHistory('crop');
                                this.cancelCrop();
                                this.zoomFit();
            }),
            resizeImage: Pixel.bindRuntime(runtime, function resizeImage() {
                                if (!this.canvas.width) return;
                                const dlg = document.createElement('div');
                                dlg.className = 'vd-modal-backdrop';
                                dlg.innerHTML = `<form class="vd-modal" role="dialog">
                                    <div class="vd-modal-title">${this.esc(this.t('pixel.resize', 'Resize'))}</div>
                                    <div class="pixel-resize-form">
                                        <label class="pixel-label">${this.esc(this.t('pixel.width', 'Width'))}</label>
                                        <input class="vd-modal-input" type="number" data-resize-w value="${this.imgWidth}" min="1">
                                        <label class="pixel-label">${this.esc(this.t('pixel.height', 'Height'))}</label>
                                        <input class="vd-modal-input" type="number" data-resize-h value="${this.imgHeight}" min="1">
                                        <label class="pixel-checkbox-row"><input type="checkbox" data-lock-ratio checked> ${this.esc(this.t('pixel.lock_ratio', 'Lock aspect ratio'))}</label>
                                    </div>
                                    <div class="vd-modal-actions">
                                        <button type="button" class="vd-button" data-cancel>${this.esc(this.t('pixel.cancel', 'Cancel'))}</button>
                                        <button type="submit" class="vd-button vd-button-primary">${this.esc(this.t('pixel.apply', 'Apply'))}</button>
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
                                    this.layers.forEach(l => {
                                        if (l.canvas) {
                                            const tmp = this.acquireTempCanvas(nw, nh);
                                            tmp.getContext('2d').drawImage(l.canvas, 0, 0, nw, nh);
                                            l.canvas.width = nw;
                                            l.canvas.height = nh;
                                            l.canvas.getContext('2d').drawImage(tmp, 0, 0);
                                            this.releaseTempCanvas(tmp);
                                        }
                                    });
                                    const tmpCanvas = this.acquireTempCanvas(nw, nh);
                                    tmpCanvas.getContext('2d').drawImage(this.canvas, 0, 0, nw, nh);
                                    this.canvas.width = nw;
                                    this.canvas.height = nh;
                                    this.overlayCanvas.width = nw;
                                    this.overlayCanvas.height = nh;
                                    this.cctx.drawImage(tmpCanvas, 0, 0);
                                    this.releaseTempCanvas(tmpCanvas);
                                    this.imgWidth = nw;
                                    this.imgHeight = nh;
                                    if (this.layers.length > 1) this.compositeLayers();
                                    this.pushHistory('resize');
                                    this.zoomFit();
                                    dlg.remove();
                                });
                                dlg.querySelector('[data-cancel]').addEventListener('click', () => dlg.remove());
                                dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });
            })
        });
    };
})();
