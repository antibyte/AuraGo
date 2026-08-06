(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.installTools = function installTools(runtime) {
        Object.assign(runtime, {
            showPanel: Pixel.bindRuntime(runtime, function showPanel(name) {
                if (name !== 'adjust' && this.compareMode) this.exitCompareMode({ preservePreview: true });
                if (name !== 'filters') this.resetFilterPreview();
                this.activePanel = name;
                this.host.querySelectorAll('.pixel-tab').forEach(b => b.classList.toggle('active', b.dataset.panel === name));
                this.host.querySelectorAll('[data-section]').forEach(s => { s.hidden = s.dataset.section !== name; });
                if (name !== 'transform' && this.cropState) this.cancelCrop();
                if (name === 'layers') this.refreshLayerPanel();
                if (name === 'history') this.refreshHistoryPanel();
                if (name === 'filters') this.refreshFilterThumbnails();
            }),
            refreshLayerPanel: Pixel.bindRuntime(runtime, function refreshLayerPanel() {
                const section = this.host.querySelector('[data-section="layers"]');
                if (!section) return;
                const list = section.querySelector('[data-layer-list]');
                if (!list) return;
                list.innerHTML = this.layers.map((layer, i) => {
                    const isActive = i === this.activeLayerIdx;
                    const maskBtn = layer.maskCanvas ? '🎭' : '◻';
                    return `<div class="pixel-layer-item${isActive ? ' active' : ''}" data-layer-idx="${i}">
                        <button class="pixel-layer-vis" type="button" data-layer-vis="${i}" title="${this.esc(this.t('pixel.toggle_visibility'))}">${layer.visible ? '👁' : '◻'}</button>
                        <button class="pixel-layer-mask" type="button" data-layer-mask="${i}" title="${this.esc(this.t('pixel.layer_mask'))}">${maskBtn}</button>
                        <span class="pixel-layer-name" data-layer-name="${i}">${this.esc(layer.name)}</span>
                        <input type="range" class="pixel-slider pixel-layer-opacity" data-layer-opacity="${i}" min="0" max="100" value="${Math.round(layer.opacity * 100)}">
                    </div>`;
                }).reverse().join('');
            }),
            refreshHistoryPanel: Pixel.bindRuntime(runtime, function refreshHistoryPanel() {
                const list = this.host.querySelector('[data-history-list]');
                if (!list) return;
                if (!this.history.length) {
                    list.innerHTML = `<p class="pixel-draw-hint">${this.esc(this.t('pixel.history_empty'))}</p>`;
                    return;
                }
                list.innerHTML = this.history.map((entry, i) =>
                    `<button class="pixel-history-item${i === this.historyIdx ? ' active' : ''}" type="button" data-history-idx="${i}">${this.esc(this.historyLabel(entry.label))}</button>`
                ).join('');
                const active = list.querySelector('.pixel-history-item.active');
                if (active) active.scrollIntoView({ block: 'nearest' });
            }),
            historyLabel: Pixel.bindRuntime(runtime, function historyLabel(label) {
                return String(label || '').replace(/:/g, ' · ');
            }),
            jumpToHistory: Pixel.bindRuntime(runtime, function jumpToHistory(idx) {
                if (idx < 0 || idx >= this.history.length || idx === this.historyIdx) return;
                this.historyIdx = idx;
                this.restoreHistory(this.history[idx]);
                this.updateStatus();
                this.refreshHistoryPanel();
            }),
            renderRecentColors: Pixel.bindRuntime(runtime, function renderRecentColors() {
                const wrap = this.host.querySelector('[data-recent-colors]');
                if (!wrap) return;
                wrap.innerHTML = this.recentColors.slice(0, 8).map(c =>
                    `<button class="pixel-recent-swatch" type="button" data-recent-color="${c}" style="background:${c}" title="${c}"></button>`
                ).join('');
            }),
            setActiveTool: Pixel.bindRuntime(runtime, function setActiveTool(tool) {
                if (tool !== this.activeTool && this.editMaskMode) {
                    this.editMaskMode = false;
                    this.refreshLayerPanel();
                }
                if (this.activeTool === tool) { this.activeTool = null; } else { this.activeTool = tool; }
                this.host.querySelectorAll('[data-draw-tool]').forEach(b => b.classList.toggle('active', b.dataset.drawTool === this.activeTool));
                this.renderOptionsBar();
                this.updateStatus();
                if (this.statusTool) this.statusTool.textContent = this.activeTool ? this.toolLabel(this.activeTool) : '';
                const cursor = this.getToolCursor();
                this.canvas.style.cursor = cursor;
                if (this.overlayCanvas) this.overlayCanvas.style.cursor = cursor;
            }),
            renderOptionsBar: Pixel.bindRuntime(runtime, function renderOptionsBar() {
                if (!this.optionsBar) return;
                if (!this.activeTool) {
                    this.optionsBar.hidden = true;
                    this.optionsBar.innerHTML = '';
                    return;
                }
                this.optionsBar.innerHTML = this.buildDrawOptionsHTML();
                this.optionsBar.hidden = false;
                this.wireDrawOptionEvents();
            }),
            getToolCursor: Pixel.bindRuntime(runtime, function getToolCursor() {
                if (!this.activeTool) return 'default';
                if (['brush', 'pencil', 'airbrush', 'eraser', 'clone-stamp', 'dodge-burn', 'blur-brush'].includes(this.activeTool)) return 'none';
                if (this.activeTool === 'text') return 'text';
                if (this.activeTool === 'move') return 'move';
                return 'crosshair';
            }),
            clearOverlay: Pixel.bindRuntime(runtime, function clearOverlay() {
                if (this.olCtx) this.olCtx.clearRect(0, 0, this.overlayCanvas.width, this.overlayCanvas.height);
            }),
            drawMarchingAnts: Pixel.bindRuntime(runtime, function drawMarchingAnts() {
                if (!this.selection) { this.clearOverlay(); return; }
                this.marchingOfs = (this.marchingOfs + 0.5) % 8;
                this.olCtx.clearRect(0, 0, this.overlayCanvas.width, this.overlayCanvas.height);
                if (this.moveSelFloating && this.selImageData && this.selection) {
                    const floatC = this.acquireTempCanvas(this.selImageData.width, this.selImageData.height);
                    floatC.getContext('2d').putImageData(this.selImageData, 0, 0);
                    this.olCtx.drawImage(floatC, Math.round(this.selection.x), Math.round(this.selection.y));
                    this.releaseTempCanvas(floatC);
                }
                if (this.selection.type === 'mask' && this.selection.border) {
                    const w = this.overlayCanvas.width;
                    const phase = Math.floor(this.marchingOfs) % 2;
                    for (let i = 0; i < this.selection.border.length; i++) {
                        const p = this.selection.border[i];
                        const x = p % w;
                        const y = (p / w) | 0;
                        this.olCtx.fillStyle = ((x + y + phase) % 2 === 0) ? '#ffffff' : '#000000';
                        this.olCtx.fillRect(x, y, 1, 1);
                    }
                    this.marchingRAF = requestAnimationFrame(this.drawMarchingAnts);
                    return;
                }
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
                this.moveSelFloating = false;
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
                if (this.selection.type === 'mask' && this.selection.mask) {
                    const fullW = this.canvas.width;
                    const mask = this.selection.mask;
                    const d = this.selImageData.data;
                    for (let yy = 0; yy < sh; yy++) {
                        for (let xx = 0; xx < sw; xx++) {
                            if (!mask[(sy + yy) * fullW + (sx + xx)]) d[(yy * sw + xx) * 4 + 3] = 0;
                        }
                    }
                }
                try {
                    const tmpC = this.acquireTempCanvas(sw, sh);
                    tmpC.getContext('2d').putImageData(this.selImageData, 0, 0);
                    tmpC.toBlob(blob => {
                        if (blob) navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]).catch(() => {});
                    }, 'image/png');
                    this.releaseTempCanvas(tmpC);
                } catch (_) {}
                this.notify({ type: 'success', message: this.t('pixel.copied') });
            }),
            cutSelection: Pixel.bindRuntime(runtime, function cutSelection() {
                if (!this.selection || !this.canvas.width) return;
                this.copySelection();
                if (this.selection.type === 'mask' && this.selection.mask) {
                    this.eraseMaskPixels();
                    this.pushHistory('cut');
                    return;
                }
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
                if (this.selection.type === 'mask' && this.selection.mask) {
                    this.eraseMaskPixels();
                    this.pushHistory('delete');
                    return;
                }
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
            eraseMaskPixels: Pixel.bindRuntime(runtime, function eraseMaskPixels() {
                const mask = this.selection.mask;
                const w = this.canvas.width, h = this.canvas.height;
                const actx = this.getActiveCtx();
                const imgData = actx.getImageData(0, 0, w, h);
                const d = imgData.data;
                for (let i = 0; i < mask.length; i++) {
                    if (mask[i]) d[i * 4 + 3] = 0;
                }
                actx.putImageData(imgData, 0, 0);
                if (this.layers.length > 1) this.compositeLayers();
            }),
            ensureSelectionPixels: Pixel.bindRuntime(runtime, function ensureSelectionPixels() {
                if (this.selImageData || !this.selection || !this.canvas.width) return !!this.selImageData;
                const sx = Math.max(0, Math.round(Math.min(this.selection.x, this.selection.x + this.selection.w)));
                const sy = Math.max(0, Math.round(Math.min(this.selection.y, this.selection.y + this.selection.h)));
                const sw = Math.round(Math.abs(this.selection.w));
                const sh = Math.round(Math.abs(this.selection.h));
                if (sw < 1 || sh < 1) return false;
                const actx = this.getActiveCtx();
                this.selImageData = actx.getImageData(sx, sy, sw, sh);
                if (this.selection.type === 'mask' && this.selection.mask) {
                    const fullW = this.canvas.width;
                    const mask = this.selection.mask;
                    const d = this.selImageData.data;
                    for (let yy = 0; yy < sh; yy++) {
                        for (let xx = 0; xx < sw; xx++) {
                            if (!mask[(sy + yy) * fullW + (sx + xx)]) d[(yy * sw + xx) * 4 + 3] = 0;
                        }
                    }
                }
                return true;
            }),
            beginMoveSelection: Pixel.bindRuntime(runtime, function beginMoveSelection() {
                if (!this.selection || !this.canvas.width || this.moveSelFloating) return false;
                if (!this.ensureSelectionPixels()) return false;
                const sx = Math.max(0, Math.round(Math.min(this.selection.x, this.selection.x + this.selection.w)));
                const sy = Math.max(0, Math.round(Math.min(this.selection.y, this.selection.y + this.selection.h)));
                const sw = Math.round(Math.abs(this.selection.w));
                const sh = Math.round(Math.abs(this.selection.h));
                if (sw < 1 || sh < 1) return false;
                this.moveSelOrigX = sx;
                this.moveSelOrigY = sy;
                const actx = this.getActiveCtx();
                if (this.selection.type === 'mask' && this.selection.mask) {
                    this.eraseMaskPixels();
                } else {
                    actx.clearRect(sx, sy, sw, sh);
                }
                if (this.layers.length > 1) this.compositeLayers();
                this.moveSelFloating = true;
                this.startMarchingAnts();
                return true;
            }),
            commitMoveSelection: Pixel.bindRuntime(runtime, function commitMoveSelection() {
                if (!this.moveSelFloating || !this.selImageData || !this.selection) {
                    this.moveSelFloating = false;
                    return;
                }
                const destX = Math.round(this.selection.x);
                const destY = Math.round(this.selection.y);
                const actx = this.getActiveCtx();
                actx.putImageData(this.selImageData, destX, destY);
                this.selection.x = destX;
                this.selection.y = destY;
                if (this.layers.length > 1) this.compositeLayers();
                const moved = destX !== this.moveSelOrigX || destY !== this.moveSelOrigY;
                this.moveSelFloating = false;
                if (moved) this.pushHistory('move-selection');
            }),
            toggleLayerMask: Pixel.bindRuntime(runtime, function toggleLayerMask(idx) {
                const layer = this.layers[idx];
                if (!layer) return;
                if (!layer.maskCanvas) {
                    this.ensureBackgroundMigrated();
                    if (!layer.canvas && idx === this.activeLayerIdx) {
                        layer.canvas = this.acquireTempCanvas(this.imgWidth, this.imgHeight);
                        layer.canvas.getContext('2d').drawImage(this.canvas, 0, 0);
                    }
                    const mc = document.createElement('canvas');
                    mc.width = this.imgWidth;
                    mc.height = this.imgHeight;
                    const mx = mc.getContext('2d');
                    mx.fillStyle = '#ffffff';
                    mx.fillRect(0, 0, mc.width, mc.height);
                    layer.maskCanvas = mc;
                }
                this.activeLayerIdx = idx;
                this.editMaskMode = !this.editMaskMode;
                this.refreshLayerPanel();
                this.notify({ type: 'info', message: this.editMaskMode ? this.t('pixel.mask_edit_on') : this.t('pixel.mask_edit_off') });
            }),
            cloneStampAt: Pixel.bindRuntime(runtime, function cloneStampAt(x, y) {
                if (this.cloneSourceX == null || this.cloneSourceY == null || !this.cloneSampleCanvas) return;
                const actx = this.getActiveCtx();
                const offX = this.cloneStrokeOffX;
                const offY = this.cloneStrokeOffY;
                const size = this.brushSize;
                const r = Math.max(1, Math.round(size / 2));
                const srcX = x - offX;
                const srcY = y - offY;
                const sx0 = Math.max(0, Math.min(Math.round(srcX - r), this.canvas.width - 1));
                const sy0 = Math.max(0, Math.min(Math.round(srcY - r), this.canvas.height - 1));
                const sw = Math.min(r * 2, this.canvas.width - sx0);
                const sh = Math.min(r * 2, this.canvas.height - sy0);
                if (sw < 1 || sh < 1) return;
                const srcCtx = this.cloneSampleCanvas.getContext('2d');
                const patch = srcCtx.getImageData(sx0, sy0, sw, sh);
                const destX = Math.max(0, Math.min(Math.round(x - r), this.canvas.width - sw));
                const destY = Math.max(0, Math.min(Math.round(y - r), this.canvas.height - sh));
                actx.putImageData(patch, destX, destY);
                this.cloneStampUsed = true;
            }),
            commitLassoPath: Pixel.bindRuntime(runtime, function commitLassoPath() {
                const path = this.lassoPath;
                this.lassoPath = null;
                this.clearOverlay();
                if (!path || path.length < 3 || !this.canvas.width) return;
                const w = this.canvas.width, h = this.canvas.height;
                const tmp = this.acquireTempCanvas(w, h);
                const tctx = tmp.getContext('2d');
                tctx.fillStyle = '#ffffff';
                tctx.beginPath();
                tctx.moveTo(path[0].x, path[0].y);
                for (let i = 1; i < path.length; i++) tctx.lineTo(path[i].x, path[i].y);
                tctx.closePath();
                tctx.fill();
                const data = tctx.getImageData(0, 0, w, h).data;
                this.releaseTempCanvas(tmp);
                const mask = new Uint8Array(w * h);
                let minX = w, minY = h, maxX = 0, maxY = 0, count = 0;
                const border = [];
                for (let yy = 0; yy < h; yy++) {
                    for (let xx = 0; xx < w; xx++) {
                        const p = yy * w + xx;
                        if (data[p * 4 + 3] > 128) {
                            mask[p] = 1;
                            count++;
                            if (xx < minX) minX = xx;
                            if (xx > maxX) maxX = xx;
                            if (yy < minY) minY = yy;
                            if (yy > maxY) maxY = yy;
                        }
                    }
                }
                if (!count) return;
                for (let yy = minY; yy <= maxY; yy++) {
                    for (let xx = minX; xx <= maxX; xx++) {
                        const p = yy * w + xx;
                        if (!mask[p]) continue;
                        if ((xx > 0 && !mask[p - 1]) || (xx < w - 1 && !mask[p + 1]) || (yy > 0 && !mask[p - w]) || (yy < h - 1 && !mask[p + w])) {
                            border.push(p);
                        }
                    }
                }
                this.selection = { type: 'mask', mask, border, x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
                this.startMarchingAnts();
            }),
            drawLassoPreview: Pixel.bindRuntime(runtime, function drawLassoPreview() {
                if (!this.lassoPath || this.lassoPath.length < 2) return;
                this.clearOverlay();
                this.olCtx.save();
                this.olCtx.strokeStyle = '#ffffff';
                this.olCtx.lineWidth = 1;
                this.olCtx.setLineDash([4, 4]);
                this.olCtx.beginPath();
                this.olCtx.moveTo(this.lassoPath[0].x, this.lassoPath[0].y);
                for (let i = 1; i < this.lassoPath.length; i++) this.olCtx.lineTo(this.lassoPath[i].x, this.lassoPath[i].y);
                this.olCtx.stroke();
                this.olCtx.restore();
            }),
            commitMoveLayer: Pixel.bindRuntime(runtime, function commitMoveLayer(dx, dy) {
                if (!dx && !dy) return;
                const actx = this.getActiveCtx();
                const w = this.canvas.width, h = this.canvas.height;
                const snap = this.acquireTempCanvas(w, h);
                snap.getContext('2d').drawImage(actx.canvas, 0, 0);
                actx.clearRect(0, 0, w, h);
                actx.drawImage(snap, dx, dy);
                this.releaseTempCanvas(snap);
                if (this.layers.length > 1) this.compositeLayers();
                this.pushHistory('move');
            }),
            magicWandSelect: Pixel.bindRuntime(runtime, function magicWandSelect(x, y) {
                if (!this.canvas.width) return;
                const w = this.canvas.width, h = this.canvas.height;
                if (x < 0 || x >= w || y < 0 || y >= h) return;
                const actx = this.getActiveCtx();
                const data = actx.getImageData(0, 0, w, h).data;
                const idx = (y * w + x) * 4;
                const sr = data[idx], sg = data[idx + 1], sb = data[idx + 2];
                const mask = new Uint8Array(w * h);
                const stack = [x, y];
                const tol = this.fillTolerance;
                let minX = w, minY = h, maxX = 0, maxY = 0, count = 0;
                while (stack.length > 0) {
                    const cy = stack.pop();
                    const cx = stack.pop();
                    if (cx < 0 || cx >= w || cy < 0 || cy >= h) continue;
                    const pos = cy * w + cx;
                    if (mask[pos]) continue;
                    const pi = pos * 4;
                    if (this.colorDist(data[pi], data[pi + 1], data[pi + 2], sr, sg, sb) > tol) continue;
                    mask[pos] = 1;
                    count++;
                    if (cx < minX) minX = cx;
                    if (cx > maxX) maxX = cx;
                    if (cy < minY) minY = cy;
                    if (cy > maxY) maxY = cy;
                    stack.push(cx + 1, cy);
                    stack.push(cx - 1, cy);
                    stack.push(cx, cy + 1);
                    stack.push(cx, cy - 1);
                }
                if (!count) return;
                const border = [];
                for (let yy = minY; yy <= maxY; yy++) {
                    for (let xx = minX; xx <= maxX; xx++) {
                        const p = yy * w + xx;
                        if (!mask[p]) continue;
                        if ((xx > 0 && !mask[p - 1]) || (xx < w - 1 && !mask[p + 1]) || (yy > 0 && !mask[p - w]) || (yy < h - 1 && !mask[p + w])) {
                            border.push(p);
                        }
                    }
                }
                this.selection = { type: 'mask', mask, border, x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
                this.startMarchingAnts();
            }),
            commitGradient: Pixel.bindRuntime(runtime, function commitGradient(x0, y0, x1, y1) {
                const actx = this.getActiveCtx();
                actx.save();
                if (this.selection && this.selection.type === 'rect') {
                    actx.beginPath();
                    actx.rect(this.selection.x, this.selection.y, this.selection.w, this.selection.h);
                    actx.clip();
                } else if (this.selection && this.selection.type === 'ellipse') {
                    actx.beginPath();
                    actx.ellipse(this.selection.x + this.selection.w / 2, this.selection.y + this.selection.h / 2, Math.abs(this.selection.w / 2), Math.abs(this.selection.h / 2), 0, 0, Math.PI * 2);
                    actx.clip();
                }
                let grad;
                if (this.gradientMode === 'radial') {
                    const r = Math.hypot(x1 - x0, y1 - y0) || 1;
                    grad = actx.createRadialGradient(x0, y0, 0, x0, y0, r);
                } else {
                    grad = actx.createLinearGradient(x0, y0, x1, y1);
                }
                grad.addColorStop(0, this.primaryColor);
                grad.addColorStop(1, this.secondaryColor);
                actx.fillStyle = grad;
                actx.globalAlpha = this.brushOpacity / 100;
                actx.fillRect(0, 0, this.canvas.width, this.canvas.height);
                actx.restore();
                if (this.layers.length > 1) this.compositeLayers();
                this.addRecentColor(this.primaryColor);
                this.pushHistory('gradient');
            }),
            sprayAt: Pixel.bindRuntime(runtime, function sprayAt(x, y) {
                const actx = this.getActiveCtx();
                const r = Math.max(1, this.brushSize / 2);
                actx.save();
                actx.fillStyle = this.primaryColor;
                for (let i = 0; i < this.sprayDensity; i++) {
                    const ang = Math.random() * Math.PI * 2;
                    const rad = Math.sqrt(Math.random()) * r;
                    actx.globalAlpha = (this.brushOpacity / 100) * 0.35;
                    actx.fillRect(x + Math.cos(ang) * rad, y + Math.sin(ang) * rad, 1.5, 1.5);
                }
                actx.restore();
                if (this.layers.length > 1) this.compositeLayers();
            }),
            dodgeBurnAt: Pixel.bindRuntime(runtime, function dodgeBurnAt(x, y) {
                const actx = this.getActiveCtx();
                const r = Math.max(2, Math.round(this.brushSize / 2));
                const sx = Math.max(0, x - r), sy = Math.max(0, y - r);
                const sw = Math.min(this.canvas.width - sx, r * 2), sh = Math.min(this.canvas.height - sy, r * 2);
                if (sw < 1 || sh < 1) return;
                const imgData = actx.getImageData(sx, sy, sw, sh);
                const d = imgData.data;
                const amt = (this.brushOpacity / 100) * 18 * (this.dodgeMode === 'burn' ? -1 : 1);
                const cx = x - sx, cy = y - sy;
                for (let yy = 0; yy < sh; yy++) {
                    for (let xx = 0; xx < sw; xx++) {
                        const dist = Math.hypot(xx - cx, yy - cy);
                        if (dist > r) continue;
                        const fall = 1 - dist / r;
                        const i = (yy * sw + xx) * 4;
                        d[i] = this.clamp255(d[i] + amt * fall);
                        d[i + 1] = this.clamp255(d[i + 1] + amt * fall);
                        d[i + 2] = this.clamp255(d[i + 2] + amt * fall);
                    }
                }
                actx.putImageData(imgData, sx, sy);
                if (this.layers.length > 1) this.compositeLayers();
            }),
            blurAt: Pixel.bindRuntime(runtime, function blurAt(x, y) {
                const actx = this.getActiveCtx();
                const r = Math.max(2, this.brushSize / 2);
                actx.save();
                actx.beginPath();
                actx.arc(x, y, r, 0, Math.PI * 2);
                actx.clip();
                actx.filter = 'blur(' + Math.max(1, Math.round(this.brushOpacity / 25)) + 'px)';
                actx.drawImage(actx.canvas, 0, 0);
                actx.restore();
                if (this.layers.length > 1) this.compositeLayers();
            }),
            addRecentColor: Pixel.bindRuntime(runtime, function addRecentColor(c) {
                if (!c || c.length < 4) return;
                this.recentColors = this.recentColors.filter(x => x !== c);
                this.recentColors.unshift(c);
                if (this.recentColors.length > 8) this.recentColors = this.recentColors.slice(0, 8);
                this.renderRecentColors();
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
                } else if (this.activeTool === 'gradient') {
                    this.olCtx.beginPath();
                    this.olCtx.moveTo(x0, y0);
                    this.olCtx.lineTo(x1, y1);
                    this.olCtx.stroke();
                    this.olCtx.beginPath();
                    this.olCtx.arc(x0, y0, 3, 0, Math.PI * 2);
                    this.olCtx.fill();
                    this.olCtx.beginPath();
                    this.olCtx.arc(x1, y1, 3, 0, Math.PI * 2);
                    this.olCtx.fill();
                }
                this.olCtx.restore();
            }),
            commitShape: Pixel.bindRuntime(runtime, function commitShape(x0, y0, x1, y1) {
                if (this.activeTool === 'gradient') { this.commitGradient(x0, y0, x1, y1); return; }
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
                if (this.layers.length >= this.MAX_LAYERS) { this.notify({ type: 'error', message: this.t('pixel.max_layers') }); return; }
                this.ensureBackgroundMigrated();
                const c = document.createElement('canvas');
                c.width = this.imgWidth;
                c.height = this.imgHeight;
                this.layers.push({ canvas: c, name: this.t('pixel.layer') + ' ' + (this.layers.length + 1), visible: true, opacity: 1 });
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
                if (this.layers.length >= this.MAX_LAYERS) return;
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
                this.layers = [{ canvas: null, name: this.t('pixel.layer_background'), visible: true, opacity: 1 }];
                this.activeLayerIdx = 0;
                this.refreshLayerPanel();
                this.pushHistory('flatten');
            })
        });
    };
})();
