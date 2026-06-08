(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installTools = function installTools(runtime) {
        Object.assign(runtime, {
            showPanel: Pixel.bindRuntime(runtime, function showPanel(name) {
                                if (name !== 'adjust' && this.compareMode) this.exitCompareMode({ preservePreview: true });
                                this.activePanel = name;
                                this.host.querySelectorAll('.pixel-tab').forEach(b => b.classList.toggle('active', b.dataset.panel === name));
                                this.host.querySelectorAll('[data-section]').forEach(s => { s.hidden = s.dataset.section !== name; });
                                if (name !== 'transform' && this.cropState) this.cancelCrop();
                                if (name === 'draw' && !this.activeTool) this.setActiveTool('brush');
                                if (name === 'layers') this.refreshLayerPanel();
            }),
            refreshLayerPanel: Pixel.bindRuntime(runtime, function refreshLayerPanel() {
                                const section = this.host.querySelector('[data-section="layers"]');
                                if (!section) return;
                                const list = section.querySelector('[data-layer-list]');
                                if (!list) return;
                                list.innerHTML = this.layers.map((layer, i) => {
                                    const isActive = i === this.activeLayerIdx;
                                    return `<div class="pixel-layer-item${isActive ? ' active' : ''}" data-layer-idx="${i}">
                                        <button class="pixel-layer-vis" type="button" data-layer-vis="${i}" title="${this.esc(this.t('pixel.toggle_visibility', 'Toggle Visibility'))}">${layer.visible ? '👁' : '◻'}</button>
                                        <span class="pixel-layer-name" data-layer-name="${i}">${this.esc(layer.name)}</span>
                                        <input type="range" class="pixel-slider pixel-layer-opacity" data-layer-opacity="${i}" min="0" max="100" value="${Math.round(layer.opacity * 100)}">
                                    </div>`;
                                }).reverse().join('');
            }),
            setActiveTool: Pixel.bindRuntime(runtime, function setActiveTool(tool) {
                                if (this.activeTool === tool) { this.activeTool = null; } else { this.activeTool = tool; }
                                this.host.querySelectorAll('[data-draw-tool]').forEach(b => b.classList.toggle('active', b.dataset.drawTool === this.activeTool));
                                const optsContainer = this.host.querySelector('[data-draw-options]');
                                if (optsContainer) optsContainer.innerHTML = this.buildDrawOptionsHTML();
                                this.wireDrawOptionEvents();
                                this.updateStatus();
                                if (this.statusTool) this.statusTool.textContent = this.activeTool ? this.t('pixel.' + this.activeTool, this.activeTool) : '';
                                this.canvas.style.cursor = this.getToolCursor();
            }),
            getToolCursor: Pixel.bindRuntime(runtime, function getToolCursor() {
                                if (!this.activeTool || this.activeTool === 'select-rect' || this.activeTool === 'select-ellipse') return 'crosshair';
                                if (this.activeTool === 'eyedropper') return 'crosshair';
                                if (this.activeTool === 'text') return 'text';
                                if (this.activeTool === 'fill') return 'crosshair';
                                return 'crosshair';
            }),
            clearOverlay: Pixel.bindRuntime(runtime, function clearOverlay() {
                                if (this.olCtx) this.olCtx.clearRect(0, 0, this.overlayCanvas.width, this.overlayCanvas.height);
            }),
            drawMarchingAnts: Pixel.bindRuntime(runtime, function drawMarchingAnts() {
                                if (!this.selection) { this.clearOverlay(); return; }
                                this.marchingOfs = (this.marchingOfs + 0.5) % 8;
                                this.olCtx.clearRect(0, 0, this.overlayCanvas.width, this.overlayCanvas.height);
                                this.olCtx.setLineDash([4, 4]);
                                this.olCtx.lineDashOffset = -this.marchingOfs;
                                this.olCtx.strokeStyle = '#ffffff';
                                this.olCtx.lineWidth = 1;
                                if (this.selection.type === 'rect') {
                                    this.olCtx.strokeRect(this.selection.x, this.selection.y, this.selection.w, this.selection.h);
                                } else if (this.selection.type === 'ellipse') {
                                    this.olCtx.beginPath();
                                    this.olCtx.ellipse(this.selection.x + this.selection.w / 2, this.selection.y + this.selection.h / 2, Math.abs(this.selection.w / 2), Math.abs(this.selection.h / 2), 0, 0, Math.PI * 2);
                                    this.olCtx.stroke();
                                }
                                this.olCtx.setLineDash([]);
                                this.olCtx.lineDashOffset = 0;
                                this.olCtx.strokeStyle = '#000000';
                                this.olCtx.lineWidth = 1;
                                this.olCtx.setLineDash([4, 4]);
                                this.olCtx.lineDashOffset = -this.marchingOfs + 4;
                                if (this.selection.type === 'rect') {
                                    this.olCtx.strokeRect(this.selection.x, this.selection.y, this.selection.w, this.selection.h);
                                } else if (this.selection.type === 'ellipse') {
                                    this.olCtx.beginPath();
                                    this.olCtx.ellipse(this.selection.x + this.selection.w / 2, this.selection.y + this.selection.h / 2, Math.abs(this.selection.w / 2), Math.abs(this.selection.h / 2), 0, 0, Math.PI * 2);
                                    this.olCtx.stroke();
                                }
                                this.olCtx.setLineDash([]);
                                this.marchingRAF = requestAnimationFrame(this.drawMarchingAnts);
            }),
            startMarchingAnts: Pixel.bindRuntime(runtime, function startMarchingAnts() {
                                this.stopMarchingAnts();
                                this.marchingOfs = 0;
                                this.drawMarchingAnts();
            }),
            stopMarchingAnts: Pixel.bindRuntime(runtime, function stopMarchingAnts() {
                                if (this.marchingRAF) { cancelAnimationFrame(this.marchingRAF); this.marchingRAF = null; }
            }),
            selectAll: Pixel.bindRuntime(runtime, function selectAll() {
                                if (!this.canvas.width) return;
                                this.selection = { type: 'rect', x: 0, y: 0, w: this.canvas.width, h: this.canvas.height };
                                this.startMarchingAnts();
            }),
            deselect: Pixel.bindRuntime(runtime, function deselect() {
                                this.selection = null;
                                this.selImageData = null;
                                this.stopMarchingAnts();
                                this.clearOverlay();
            }),
            copySelection: Pixel.bindRuntime(runtime, function copySelection() {
                                if (!this.selection || !this.canvas.width) return;
                                const sx = Math.max(0, Math.round(Math.min(this.selection.x, this.selection.x + this.selection.w)));
                                const sy = Math.max(0, Math.round(Math.min(this.selection.y, this.selection.y + this.selection.h)));
                                const sw = Math.round(Math.abs(this.selection.w));
                                const sh = Math.round(Math.abs(this.selection.h));
                                if (sw < 1 || sh < 1) return;
                                const actx = this.getActiveCtx();
                                this.selImageData = actx.getImageData(sx, sy, sw, sh);
                                try {
                                    const tmpC = this.acquireTempCanvas(sw, sh);
                                    tmpC.getContext('2d').putImageData(this.selImageData, 0, 0);
                                    tmpC.toBlob(blob => {
                                        if (blob) navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]).catch(() => {});
                                    }, 'image/png');
                                    this.releaseTempCanvas(tmpC);
                                } catch (_) {}
                                this.notify({ type: 'success', message: this.t('pixel.copied', 'Selection copied') });
            }),
            cutSelection: Pixel.bindRuntime(runtime, function cutSelection() {
                                if (!this.selection || !this.canvas.width) return;
                                this.copySelection();
                                const sx = Math.max(0, Math.round(Math.min(this.selection.x, this.selection.x + this.selection.w)));
                                const sy = Math.max(0, Math.round(Math.min(this.selection.y, this.selection.y + this.selection.h)));
                                const sw = Math.round(Math.abs(this.selection.w));
                                const sh = Math.round(Math.abs(this.selection.h));
                                if (sw < 1 || sh < 1) return;
                                const actx = this.getActiveCtx();
                                actx.clearRect(sx, sy, sw, sh);
                                if (this.layers.length > 1) this.compositeLayers();
                                this.pushHistory('cut');
            }),
            pasteClipboard: Pixel.bindRuntime(runtime, async function pasteClipboard() {
                                try {
                                    const items = await navigator.clipboard.read();
                                    for (const item of items) {
                                        for (const type of item.types) {
                                            if (type.startsWith('image/')) {
                                                const blob = await item.getType(type);
                                                const url = URL.createObjectURL(blob);
                                                const img = await this.loadImage(url);
                                                URL.revokeObjectURL(url);
                                                this.newBlankCanvas(Math.max(this.canvas.width || 0, img.naturalWidth), Math.max(this.canvas.height || 0, img.naturalHeight));
                                                const actx = this.getActiveCtx();
                                                actx.drawImage(img, 0, 0);
                                                if (this.layers.length > 1) this.compositeLayers();
                                                this.pushHistory('paste');
                                                return;
                                            }
                                        }
                                    }
                                } catch (_) {}
            }),
            deleteSelection: Pixel.bindRuntime(runtime, function deleteSelection() {
                                if (!this.selection || !this.canvas.width) return;
                                const sx = Math.max(0, Math.round(Math.min(this.selection.x, this.selection.x + this.selection.w)));
                                const sy = Math.max(0, Math.round(Math.min(this.selection.y, this.selection.y + this.selection.h)));
                                const sw = Math.round(Math.abs(this.selection.w));
                                const sh = Math.round(Math.abs(this.selection.h));
                                if (sw < 1 || sh < 1) return;
                                const actx = this.getActiveCtx();
                                actx.clearRect(sx, sy, sw, sh);
                                if (this.layers.length > 1) this.compositeLayers();
                                this.pushHistory('delete');
            }),
            addRecentColor: Pixel.bindRuntime(runtime, function addRecentColor(c) {
                                if (!c || c.length < 4) return;
                                this.recentColors = this.recentColors.filter(x => x !== c);
                                this.recentColors.unshift(c);
                                if (this.recentColors.length > 8) this.recentColors = this.recentColors.slice(0, 8);
            }),
            setPrimaryColor: Pixel.bindRuntime(runtime, function setPrimaryColor(c) {
                                this.primaryColor = c;
                                const el = this.host.querySelector('[data-color-primary]');
                                if (el) el.style.background = c;
                                const hex = this.host.querySelector('[data-hex-input]');
                                if (hex) hex.value = c;
            }),
            setSecondaryColor: Pixel.bindRuntime(runtime, function setSecondaryColor(c) {
                                this.secondaryColor = c;
                                const el = this.host.querySelector('[data-color-secondary]');
                                if (el) el.style.background = c;
            }),
            swapColors: Pixel.bindRuntime(runtime, function swapColors() {
                                const tmp = this.primaryColor;
                                this.setPrimaryColor(this.secondaryColor);
                                this.setSecondaryColor(tmp);
            }),
            drawBrushStroke: Pixel.bindRuntime(runtime, function drawBrushStroke(ctx, x0, y0, x1, y1, size, opacity, color, compositeOp, hardness) {
                                ctx.save();
                                ctx.globalCompositeOperation = compositeOp || 'source-over';
                                ctx.globalAlpha = opacity / 100;
                                ctx.strokeStyle = color;
                                ctx.lineWidth = size;
                                ctx.lineCap = 'round';
                                ctx.lineJoin = 'round';
                                if (hardness !== undefined && hardness < 100) {
                                    ctx.shadowBlur = (100 - hardness) * size * 0.01;
                                    ctx.shadowColor = color;
                                }
                                ctx.beginPath();
                                ctx.moveTo(x0, y0);
                                ctx.lineTo(x1, y1);
                                ctx.stroke();
                                ctx.restore();
            }),
            drawShapePreview: Pixel.bindRuntime(runtime, function drawShapePreview(x0, y0, x1, y1) {
                                this.clearOverlay();
                                this.olCtx.save();
                                this.olCtx.strokeStyle = this.primaryColor;
                                this.olCtx.fillStyle = this.primaryColor;
                                this.olCtx.lineWidth = this.shapeStrokeWidth;
                                this.olCtx.lineCap = 'round';
                                this.olCtx.lineJoin = 'round';
                                this.olCtx.globalAlpha = this.brushOpacity / 100;
                                const w = x1 - x0;
                                const h = y1 - y0;
                                if (this.activeTool === 'line') {
                                    this.olCtx.beginPath();
                                    this.olCtx.moveTo(x0, y0);
                                    this.olCtx.lineTo(x1, y1);
                                    this.olCtx.stroke();
                                } else if (this.activeTool === 'arrow') {
                                    this.olCtx.beginPath();
                                    this.olCtx.moveTo(x0, y0);
                                    this.olCtx.lineTo(x1, y1);
                                    this.olCtx.stroke();
                                    const angle = Math.atan2(y1 - y0, x1 - x0);
                                    const headLen = Math.max(10, this.shapeStrokeWidth * 4);
                                    this.olCtx.beginPath();
                                    this.olCtx.moveTo(x1, y1);
                                    this.olCtx.lineTo(x1 - headLen * Math.cos(angle - Math.PI / 6), y1 - headLen * Math.sin(angle - Math.PI / 6));
                                    this.olCtx.moveTo(x1, y1);
                                    this.olCtx.lineTo(x1 - headLen * Math.cos(angle + Math.PI / 6), y1 - headLen * Math.sin(angle + Math.PI / 6));
                                    this.olCtx.stroke();
                                } else if (this.activeTool === 'rectangle') {
                                    if (this.shapeFill) this.olCtx.fillRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                    else this.olCtx.strokeRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                } else if (this.activeTool === 'ellipse') {
                                    this.olCtx.beginPath();
                                    this.olCtx.ellipse(x0 + w / 2, y0 + h / 2, Math.abs(w / 2), Math.abs(h / 2), 0, 0, Math.PI * 2);
                                    if (this.shapeFill) this.olCtx.fill();
                                    else this.olCtx.stroke();
                                }
                                this.olCtx.restore();
            }),
            commitShape: Pixel.bindRuntime(runtime, function commitShape(x0, y0, x1, y1) {
                                const actx = this.getActiveCtx();
                                actx.save();
                                actx.strokeStyle = this.primaryColor;
                                actx.fillStyle = this.primaryColor;
                                actx.lineWidth = this.shapeStrokeWidth;
                                actx.lineCap = 'round';
                                actx.lineJoin = 'round';
                                actx.globalAlpha = this.brushOpacity / 100;
                                const w = x1 - x0;
                                const h = y1 - y0;
                                if (this.activeTool === 'line') {
                                    actx.beginPath();
                                    actx.moveTo(x0, y0);
                                    actx.lineTo(x1, y1);
                                    actx.stroke();
                                } else if (this.activeTool === 'arrow') {
                                    actx.beginPath();
                                    actx.moveTo(x0, y0);
                                    actx.lineTo(x1, y1);
                                    actx.stroke();
                                    const angle = Math.atan2(y1 - y0, x1 - x0);
                                    const headLen = Math.max(10, this.shapeStrokeWidth * 4);
                                    actx.beginPath();
                                    actx.moveTo(x1, y1);
                                    actx.lineTo(x1 - headLen * Math.cos(angle - Math.PI / 6), y1 - headLen * Math.sin(angle - Math.PI / 6));
                                    actx.moveTo(x1, y1);
                                    actx.lineTo(x1 - headLen * Math.cos(angle + Math.PI / 6), y1 - headLen * Math.sin(angle + Math.PI / 6));
                                    actx.stroke();
                                } else if (this.activeTool === 'rectangle') {
                                    if (this.shapeFill) actx.fillRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                    else actx.strokeRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                                } else if (this.activeTool === 'ellipse') {
                                    actx.beginPath();
                                    actx.ellipse(x0 + w / 2, y0 + h / 2, Math.abs(w / 2), Math.abs(h / 2), 0, 0, Math.PI * 2);
                                    if (this.shapeFill) actx.fill();
                                    else actx.stroke();
                                }
                                actx.restore();
                                if (this.layers.length > 1) this.compositeLayers();
            }),
            floodFill: Pixel.bindRuntime(runtime, function floodFill(sx, sy, fillColor) {
                                const actx = this.getActiveCtx();
                                const w = this.canvas.width;
                                const h = this.canvas.height;
                                const imgData = actx.getImageData(0, 0, w, h);
                                const data = imgData.data;
                                const idx = (sy * w + sx) * 4;
                                const sr = data[idx], sg = data[idx + 1], sb = data[idx + 2], sa = data[idx + 3];
                                const fc = this.hexToRgb(fillColor);
                                if (sr === fc.r && sg === fc.g && sb === fc.b && sa === 255) return;
                                const visited = new Uint8Array(w * h);
                                const stack = [sx, sy];
                                const tol = this.fillTolerance;
                                while (stack.length > 0) {
                                    const cy = stack.pop();
                                    const cx = stack.pop();
                                    if (cx < 0 || cx >= w || cy < 0 || cy >= h) continue;
                                    const pos = cy * w + cx;
                                    if (visited[pos]) continue;
                                    const pi = pos * 4;
                                    if (this.colorDist(data[pi], data[pi + 1], data[pi + 2], sr, sg, sb) > tol) continue;
                                    visited[pos] = 1;
                                    data[pi] = fc.r;
                                    data[pi + 1] = fc.g;
                                    data[pi + 2] = fc.b;
                                    data[pi + 3] = 255;
                                    stack.push(cx + 1, cy);
                                    stack.push(cx - 1, cy);
                                    stack.push(cx, cy + 1);
                                    stack.push(cx, cy - 1);
                                }
                                actx.putImageData(imgData, 0, 0);
                                if (this.layers.length > 1) this.compositeLayers();
            }),
            commitTextToCanvas: Pixel.bindRuntime(runtime, function commitTextToCanvas(text, x, y) {
                                if (!text) return;
                                const actx = this.getActiveCtx();
                                actx.save();
                                actx.font = `${this.fontSize}px ${this.fontFamily}`;
                                actx.fillStyle = this.primaryColor;
                                actx.globalAlpha = this.brushOpacity / 100;
                                actx.textBaseline = 'top';
                                const lines = text.split('\n');
                                for (let i = 0; i < lines.length; i++) {
                                    actx.fillText(lines[i], x, y + i * this.fontSize * 1.2);
                                }
                                actx.restore();
                                if (this.layers.length > 1) this.compositeLayers();
                                this.pushHistory('text');
            }),
            addLayer: Pixel.bindRuntime(runtime, function addLayer() {
                                if (this.layers.length >= 10) { this.notify({ type: 'error', message: this.t('pixel.max_layers', 'Maximum 10 layers') }); return; }
                                this.ensureBackgroundMigrated();
                                const c = document.createElement('canvas');
                                c.width = this.imgWidth;
                                c.height = this.imgHeight;
                                this.layers.push({ canvas: c, name: this.t('pixel.layer', 'Layer') + ' ' + (this.layers.length + 1), visible: true, opacity: 1 });
                                this.activeLayerIdx = this.layers.length - 1;
                                this.compositeLayers();
                                this.refreshLayerPanel();
                                this.pushHistory('add layer');
            }),
            deleteLayer: Pixel.bindRuntime(runtime, function deleteLayer() {
                                if (this.layers.length <= 1) return;
                                this.layers.splice(this.activeLayerIdx, 1);
                                this.activeLayerIdx = Math.min(this.activeLayerIdx, this.layers.length - 1);
                                if (this.layers.length === 1 && this.layers[0].canvas) {
                                    this.cctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
                                    this.cctx.drawImage(this.layers[0].canvas, 0, 0);
                                    this.layers[0].canvas = null;
                                }
                                this.compositeLayers();
                                this.refreshLayerPanel();
                                this.pushHistory('delete layer');
            }),
            duplicateLayer: Pixel.bindRuntime(runtime, function duplicateLayer() {
                                if (this.layers.length >= 10) return;
                                const src = this.layers[this.activeLayerIdx];
                                const c = document.createElement('canvas');
                                c.width = this.imgWidth;
                                c.height = this.imgHeight;
                                if (src.canvas) c.getContext('2d').drawImage(src.canvas, 0, 0);
                                else c.getContext('2d').drawImage(this.canvas, 0, 0);
                                this.layers.splice(this.activeLayerIdx + 1, 0, { canvas: c, name: src.name + ' copy', visible: true, opacity: src.opacity });
                                this.activeLayerIdx = this.activeLayerIdx + 1;
                                this.compositeLayers();
                                this.refreshLayerPanel();
                                this.pushHistory('duplicate layer');
            }),
            mergeDown: Pixel.bindRuntime(runtime, function mergeDown() {
                                if (this.activeLayerIdx <= 0 || this.layers.length < 2) return;
                                const upper = this.layers[this.activeLayerIdx];
                                const lower = this.layers[this.activeLayerIdx - 1];
                                const targetCanvas = lower.canvas || (() => { lower.canvas = document.createElement('canvas'); lower.canvas.width = this.imgWidth; lower.canvas.height = this.imgHeight; lower.canvas.getContext('2d').drawImage(this.canvas, 0, 0); return lower.canvas; })();
                                const tx = targetCanvas.getContext('2d');
                                tx.globalAlpha = upper.opacity;
                                if (upper.canvas) tx.drawImage(upper.canvas, 0, 0);
                                tx.globalAlpha = 1;
                                this.layers.splice(this.activeLayerIdx, 1);
                                this.activeLayerIdx--;
                                this.compositeLayers();
                                this.refreshLayerPanel();
                                this.pushHistory('merge down');
            }),
            flattenLayers: Pixel.bindRuntime(runtime, function flattenLayers() {
                                if (this.layers.length <= 1) return;
                                this.compositeLayers();
                                this.layers = [{ canvas: null, name: this.t('pixel.layer_background', 'Background'), visible: true, opacity: 1 }];
                                this.activeLayerIdx = 0;
                                this.refreshLayerPanel();
                                this.pushHistory('flatten');
            })
        });
    };
})();
