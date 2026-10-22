(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    // Moves a document-sized alpha mask; pixels pushed outside the document are dropped.
    function shiftAlpha(alpha, w, h, dx, dy) {
        const out = new Uint8ClampedArray(alpha.length);
        const x0 = Math.max(0, -dx), x1 = Math.min(w, w - dx);
        if (x1 <= x0) return out;
        for (let y = 0; y < h; y++) {
            const ny = y + dy;
            if (ny < 0 || ny >= h) continue;
            out.set(alpha.subarray(y * w + x0, y * w + x1), ny * w + x0 + dx);
        }
        return out;
    }

    function runHook(owner, hook, runtime, previous) {
        try {
            owner[hook](runtime, previous);
        } catch (err) {
            if (window.console) console.error('pixel ' + (owner.id || '') + '.' + hook, err);
        }
    }

    Pixel.installTools = function installTools(runtime) {
        Object.assign(runtime, {
            // Selected tool preview painted by the 'tool-preview' overlay: a function (ctx) set by lasso/shape tools.
            toolPreview: null,
            // Shows one panel in its own dock (inspector or bottom) and leaves the other dock alone. Side effects of
            // leaving or entering a panel live in the panel's onHide / onShow (registered in pixel-view.js and by modules).
            showPanel: Pixel.bindRuntime(runtime, function showPanel(name) {
                const def = Pixel.getPanel(name);
                if (!def) return;
                const dock = def.dock;
                const previous = this.activePanels[dock] || null;
                if (previous && previous !== name) {
                    const old = Pixel.getPanel(previous);
                    if (old && typeof old.onHide === 'function') runHook(old, 'onHide', this);
                }
                this.activePanels[dock] = name;
                this.activePanel = name;
                const scope = this.host.querySelector('[data-dock="' + dock + '"]') || this.host;
                scope.querySelectorAll('.pixel-tab').forEach(b => {
                    const on = b.dataset.panel === name;
                    b.classList.toggle('active', on);
                    b.setAttribute('aria-selected', on ? 'true' : 'false');
                });
                scope.querySelectorAll('[data-section]').forEach(s => { s.hidden = s.dataset.section !== name; });
                if (typeof def.onShow === 'function') runHook(def, 'onShow', this, previous);
                this.emit('panel-changed', { id: name, dock });
            }),
            refreshLayerPanel: Pixel.bindRuntime(runtime, function refreshLayerPanel() {
                if (this.refreshStickerControls) this.refreshStickerControls();
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
                const previous = this.activeTool;
                const next = previous === tool ? null : tool;
                const previousDef = previous ? Pixel.getTool(previous) : null;
                if (previousDef && typeof previousDef.onDeactivate === 'function') runHook(previousDef, 'onDeactivate', this);
                this.activeTool = next;
                // onActivate runs before the options bar is built so options(r) can read what the tool prepared.
                const nextDef = next ? Pixel.getTool(next) : null;
                if (nextDef && typeof nextDef.onActivate === 'function') runHook(nextDef, 'onActivate', this);
                this.host.querySelectorAll('[data-draw-tool]').forEach(b => {
                    const on = b.dataset.drawTool === this.activeTool;
                    b.classList.toggle('active', on);
                    if (on) this.revealRailButton(b);
                });
                this.renderOptionsBar();
                this.updateStatus();
                if (this.statusTool) this.statusTool.textContent = this.activeTool ? this.toolLabel(this.activeTool) : '';
                const cursor = this.getToolCursor();
                this.canvas.style.cursor = cursor;
                if (this.overlayCanvas) this.overlayCanvas.style.cursor = cursor;
                this.emit('tool-changed', { tool: this.activeTool, previous });
                this.requestOverlay();
            }),
            // A tool picked by shortcut or palette may sit in the scrolled-away part of the rail: bring its button into view.
            revealRailButton: Pixel.bindRuntime(runtime, function revealRailButton(button) {
                const rail = this.toolRail;
                if (!rail || !button || !button.getBoundingClientRect || !rail.getBoundingClientRect) return;
                const bar = rail.querySelector ? rail.querySelector('.pixel-rail-colors') : null;
                const railBox = rail.getBoundingClientRect(), box = button.getBoundingClientRect();
                const bottom = bar && bar.getBoundingClientRect ? Math.min(railBox.bottom, bar.getBoundingClientRect().top) : railBox.bottom;
                if (!(railBox.height > 0)) return;
                if (box.top < railBox.top) rail.scrollTop -= railBox.top - box.top + 4;
                else if (box.bottom > bottom) rail.scrollTop += box.bottom - bottom + 4;
            }),
            activeToolDef: Pixel.bindRuntime(runtime, function activeToolDef() {
                return this.activeTool ? Pixel.getTool(this.activeTool) : null;
            }),
            // Options bar: the tool's options(r) / wireOptions(r, bar) when it has them, else the built-in markup.
            renderOptionsBar: Pixel.bindRuntime(runtime, function renderOptionsBar() {
                if (this.refreshStickerControls) this.refreshStickerControls();
                if (!this.optionsBar) return;
                if (!this.activeTool) {
                    this.optionsBar.hidden = true;
                    this.optionsBar.innerHTML = '';
                    return;
                }
                const def = this.activeToolDef();
                let html = null;
                if (def && typeof def.options === 'function') {
                    try { html = def.options(this); } catch (err) { if (window.console) console.error('pixel ' + def.id + '.options', err); }
                }
                this.optionsBar.innerHTML = html != null ? html : this.buildDrawOptionsHTML();
                this.optionsBar.hidden = false;
                this.wireDrawOptionEvents();
                if (def && typeof def.wireOptions === 'function') {
                    try { def.wireOptions(this, this.optionsBar); } catch (err) { if (window.console) console.error('pixel ' + def.id + '.wireOptions', err); }
                }
            }),
            getToolCursor: Pixel.bindRuntime(runtime, function getToolCursor() {
                if (!this.activeTool) return 'default';
                const def = this.activeToolDef();
                if (def && def.cursor) return (typeof def.cursor === 'function' ? def.cursor(this) : def.cursor) || 'crosshair';
                return 'crosshair';
            }),
            // Drops the current tool preview (lasso path, shape rubber band). Other overlays are not touched.
            clearOverlay: Pixel.bindRuntime(runtime, function clearOverlay() {
                this.toolPreview = null;
                this.requestOverlay();
            }),
            // Overlay draw for the selection outline. It keeps itself alive at ~20 fps while a selection exists.
            drawMarchingAnts: Pixel.bindRuntime(runtime, function drawMarchingAnts(ctx) {
                const sel = this.selection;
                if (!sel || !ctx) return;
                this.marchingOfs = ((typeof performance !== 'undefined' ? performance.now() : Date.now()) / 1000 * 30) % 8;
                if (this.moveSelFloating && this.selImageData) {
                    const floatC = this.acquireTempCanvas(this.selImageData.width, this.selImageData.height);
                    floatC.getContext('2d').putImageData(this.selImageData, 0, 0);
                    ctx.drawImage(floatC, Math.round(sel.x), Math.round(sel.y));
                    this.releaseTempCanvas(floatC);
                }
                if (sel.type === 'mask' && sel.border) {
                    const w = this.canvas.width;
                    const phase = Math.floor(this.marchingOfs) % 2;
                    // The contour is stored for the original position; a floating move shows it where the pixels are.
                    const offX = this.moveSelFloating ? Math.round(sel.x - this.moveSelOrigX) : 0;
                    const offY = this.moveSelFloating ? Math.round(sel.y - this.moveSelOrigY) : 0;
                    for (let i = 0; i < sel.border.length; i++) {
                        const p = sel.border[i];
                        const x = p % w + offX;
                        const y = ((p / w) | 0) + offY;
                        ctx.fillStyle = ((x + y + phase) % 2 === 0) ? '#ffffff' : '#000000';
                        ctx.fillRect(x, y, 1, 1);
                    }
                    this.scheduleMarchingAnts();
                    return;
                }
                const outline = () => {
                    if (sel.type === 'rect') {
                        ctx.strokeRect(sel.x, sel.y, sel.w, sel.h);
                    } else if (sel.type === 'ellipse') {
                        ctx.beginPath();
                        ctx.ellipse(sel.x + sel.w / 2, sel.y + sel.h / 2, Math.abs(sel.w / 2), Math.abs(sel.h / 2), 0, 0, Math.PI * 2);
                        ctx.stroke();
                    }
                };
                ctx.lineWidth = 1;
                ctx.setLineDash([4, 4]);
                ctx.lineDashOffset = -this.marchingOfs;
                ctx.strokeStyle = '#ffffff';
                outline();
                ctx.lineDashOffset = -this.marchingOfs + 4;
                ctx.strokeStyle = '#000000';
                outline();
                ctx.setLineDash([]);
                ctx.lineDashOffset = 0;
                this.scheduleMarchingAnts();
            }),
            // The marching ants animate by asking for the next overlay frame while a selection exists.
            scheduleMarchingAnts: Pixel.bindRuntime(runtime, function scheduleMarchingAnts() {
                if (this.marchingRAF || this.state.disposed) return;
                this.marchingRAF = setTimeout(() => {
                    this.marchingRAF = null;
                    if (this.selection && !this.state.disposed) this.requestOverlay();
                }, 50);
            }),
            drawToolPreview: Pixel.bindRuntime(runtime, function drawToolPreview(ctx) {
                if (typeof this.toolPreview === 'function') this.toolPreview(ctx);
                const def = this.activeToolDef();
                if (def && typeof def.overlay === 'function') def.overlay(ctx, this);
            }),
            startMarchingAnts: Pixel.bindRuntime(runtime, function startMarchingAnts() {
                this.stopMarchingAnts();
                this.marchingOfs = 0;
                this.requestOverlay();
            }),
            stopMarchingAnts: Pixel.bindRuntime(runtime, function stopMarchingAnts() {
                if (this.marchingRAF) { clearTimeout(this.marchingRAF); this.marchingRAF = null; }
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
            selectionRect: Pixel.bindRuntime(runtime, function selectionRect() {
                const sel = this.selection;
                return {
                    sx: Math.max(0, Math.round(Math.min(sel.x, sel.x + sel.w))),
                    sy: Math.max(0, Math.round(Math.min(sel.y, sel.y + sel.h))),
                    sw: Math.round(Math.abs(sel.w)),
                    sh: Math.round(Math.abs(sel.h))
                };
            }),
            // Scales the alpha of lifted pixels by the selection mask (ellipse, lasso and wand shapes).
            applySelectionAlpha: Pixel.bindRuntime(runtime, function applySelectionAlpha(imgData, sx, sy) {
                if (!this.selection || this.selection.type === 'rect') return;
                const mask = this.getSelectionMask();
                if (!mask) return;
                const fullW = this.canvas.width, fullH = this.canvas.height;
                const sw = imgData.width, sh = imgData.height, d = imgData.data;
                for (let yy = 0; yy < sh; yy++) {
                    const y = sy + yy;
                    for (let xx = 0; xx < sw; xx++) {
                        const x = sx + xx;
                        const a = (x < fullW && y < fullH) ? mask.alpha[y * fullW + x] : 0;
                        if (a !== 255) d[(yy * sw + xx) * 4 + 3] = d[(yy * sw + xx) * 4 + 3] * a / 255;
                    }
                }
            }),
            copySelection: Pixel.bindRuntime(runtime, function copySelection() {
                if (!this.selection || !this.canvas.width) return;
                const { sx, sy, sw, sh } = this.selectionRect();
                if (sw < 1 || sh < 1) return;
                this.selImageData = this.getActiveReadCtx().getImageData(sx, sy, sw, sh);
                this.applySelectionAlpha(this.selImageData, sx, sy);
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
                if (!this.canPaintActiveLayer('erase')) return;
                if (!this.selection || !this.canvas.width) return;
                this.copySelection();
                if (this.eraseSelection()) this.pushHistory('cut');
            }),
            pasteClipboard: Pixel.bindRuntime(runtime, async function pasteClipboard() {
                try {
                    const items = await navigator.clipboard.read();
                    for (const item of items) {
                        for (const type of item.types) {
                            if (!type.startsWith('image/')) continue;
                            const blob = await item.getType(type);
                            const url = URL.createObjectURL(blob);
                            try {
                                // Without a document the clipboard image becomes the document.
                                if (!this.canvas.width) {
                                    await this.loadImageToCanvas(url);
                                    return;
                                }
                                const img = await this.loadImage(url);
                                if (this.layers.length >= this.MAX_LAYERS) { this.notify({ type: 'error', message: this.t('pixel.layer_max', { n: this.MAX_LAYERS }) }); return; }
                                this.commitPendingPreviews();
                                const layer = this.createLayer({ name: this.t('pixel.paste'), source: img });
                                this.layers.push(layer);
                                this.activeLayerIdx = this.layers.length - 1;
                                this.editMaskMode = false;
                                this.layersStructureChanged();
                                this.pushHistory('paste');
                            } finally { URL.revokeObjectURL(url); }
                            return;
                        }
                    }
                } catch (_) {}
            }),
            deleteSelection: Pixel.bindRuntime(runtime, function deleteSelection() {
                if (!this.canPaintActiveLayer('erase')) return;
                if (!this.selection || !this.canvas.width) return;
                if (this.eraseSelection()) this.pushHistory('delete');
            }),
            // Clears the selected pixels of the active layer (or its mask). Rectangles clear exactly, other shapes through their alpha.
            eraseSelection: Pixel.bindRuntime(runtime, function eraseSelection() {
                if (!this.selection) return false;
                if (this.selection.type === 'rect') {
                    const { sx, sy, sw, sh } = this.selectionRect();
                    if (sw < 1 || sh < 1) return false;
                    this.getActiveCtx().clearRect(sx, sy, sw, sh);
                    this.compositeLayers();
                    return true;
                }
                return this.eraseMaskPixels();
            }),
            eraseMaskPixels: Pixel.bindRuntime(runtime, function eraseMaskPixels() {
                const sel = this.getSelectionMask();
                if (!sel) return false;
                const fullW = this.canvas.width;
                const actx = this.getActiveCtx();
                const imgData = actx.getImageData(sel.x, sel.y, sel.w, sel.h);
                const d = imgData.data;
                for (let yy = 0; yy < sel.h; yy++) {
                    for (let xx = 0; xx < sel.w; xx++) {
                        const a = sel.alpha[(sel.y + yy) * fullW + sel.x + xx];
                        if (a) d[(yy * sel.w + xx) * 4 + 3] = d[(yy * sel.w + xx) * 4 + 3] * (255 - a) / 255;
                    }
                }
                actx.putImageData(imgData, sel.x, sel.y);
                this.compositeLayers();
                return true;
            }),
            ensureSelectionPixels: Pixel.bindRuntime(runtime, function ensureSelectionPixels() {
                if (this.selImageData || !this.selection || !this.canvas.width) return !!this.selImageData;
                const { sx, sy, sw, sh } = this.selectionRect();
                if (sw < 1 || sh < 1) return false;
                this.selImageData = this.getActiveReadCtx().getImageData(sx, sy, sw, sh);
                this.applySelectionAlpha(this.selImageData, sx, sy);
                return true;
            }),
            beginMoveSelection: Pixel.bindRuntime(runtime, function beginMoveSelection() {
                if (!this.canPaintActiveLayer('erase')) return false;
                if (!this.selection || !this.canvas.width || this.moveSelFloating) return false;
                // Always lift the current pixels; a copy taken when the selection was drawn may predate later edits.
                this.selImageData = null;
                if (!this.ensureSelectionPixels()) return false;
                const { sx, sy, sw, sh } = this.selectionRect();
                if (sw < 1 || sh < 1) return false;
                this.moveSelOrigX = sx;
                this.moveSelOrigY = sy;
                this.eraseSelection();
                this.compositeLayers();
                this.moveSelFloating = true;
                this.startMarchingAnts();
                return true;
            }),
            commitMoveSelection: Pixel.bindRuntime(runtime, function commitMoveSelection() {
                if (!this.moveSelFloating || !this.selImageData || !this.selection) {
                    this.moveSelFloating = false;
                    return;
                }
                const sel = this.selection;
                const destX = Math.round(sel.x);
                const destY = Math.round(sel.y);
                const dx = destX - this.moveSelOrigX, dy = destY - this.moveSelOrigY;
                const actx = this.getActiveCtx();
                const floating = this.acquireTempCanvas(this.selImageData.width, this.selImageData.height);
                floating.getContext('2d').putImageData(this.selImageData, 0, 0);
                actx.drawImage(floating, destX, destY);
                this.releaseTempCanvas(floating);
                this.compositeLayers();
                this.moveSelFloating = false;
                if (sel.type === 'mask' && (dx || dy)) {
                    const w = this.canvas.width, h = this.canvas.height;
                    this.setSelectionMask(shiftAlpha(sel.alpha, w, h, dx, dy), { mode: 'replace' });
                } else {
                    this.selection = Object.assign({}, sel, { x: destX, y: destY });
                }
                if (dx || dy) this.pushHistory('move-selection');
            }),
            toggleLayerMask: Pixel.bindRuntime(runtime, function toggleLayerMask(idx) {
                const layer = this.layers[idx];
                if (!layer) return;
                this.commitPendingPreviews();
                this.activeLayerIdx = idx;
                if (!layer.maskCanvas) {
                    const mc = this.makeCanvas(this.imgWidth, this.imgHeight);
                    const mx = mc.getContext('2d');
                    mx.fillStyle = '#ffffff';
                    mx.fillRect(0, 0, mc.width, mc.height);
                    layer.maskCanvas = mc;
                    layer.maskEnabled = true;
                    this.markLayerDirty(layer, { mask: true });
                    this.pushHistory('add mask');
                }
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
                let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
                path.forEach(pt => {
                    if (pt.x < minX) minX = pt.x;
                    if (pt.x > maxX) maxX = pt.x;
                    if (pt.y < minY) minY = pt.y;
                    if (pt.y > maxY) maxY = pt.y;
                });
                const x0 = Math.max(0, Math.floor(minX)), y0 = Math.max(0, Math.floor(minY));
                const x1 = Math.min(w, Math.ceil(maxX) + 1), y1 = Math.min(h, Math.ceil(maxY) + 1);
                if (x1 <= x0 || y1 <= y0) return;
                const bw = x1 - x0, bh = y1 - y0;
                const tmp = this.acquireTempCanvas(bw, bh);
                const tctx = tmp.getContext('2d');
                tctx.fillStyle = '#ffffff';
                tctx.beginPath();
                tctx.moveTo(path[0].x - x0, path[0].y - y0);
                for (let i = 1; i < path.length; i++) tctx.lineTo(path[i].x - x0, path[i].y - y0);
                tctx.closePath();
                tctx.fill();
                const data = tctx.getImageData(0, 0, bw, bh).data;
                this.releaseTempCanvas(tmp);
                const alpha = new Uint8ClampedArray(w * h);
                for (let yy = 0; yy < bh; yy++) {
                    for (let xx = 0; xx < bw; xx++) alpha[(y0 + yy) * w + x0 + xx] = data[(yy * bw + xx) * 4 + 3];
                }
                this.setSelectionMask(alpha, { mode: 'replace' });
            }),
            // The lasso outline follows lassoPath through the tool-preview overlay.
            drawLassoPreview: Pixel.bindRuntime(runtime, function drawLassoPreview() {
                if (!this.lassoPath || this.lassoPath.length < 2) return;
                this.toolPreview = ctx => {
                    const path = this.lassoPath;
                    if (!path || path.length < 2) return;
                    ctx.strokeStyle = '#ffffff';
                    ctx.lineWidth = 1;
                    ctx.setLineDash([4, 4]);
                    ctx.beginPath();
                    ctx.moveTo(path[0].x, path[0].y);
                    for (let i = 1; i < path.length; i++) ctx.lineTo(path[i].x, path[i].y);
                    ctx.stroke();
                };
                this.requestOverlay();
            }),
            commitMoveLayer: Pixel.bindRuntime(runtime, function commitMoveLayer(dx, dy) {
                if (!dx && !dy) return;
                const layer = this.activeLayer();
                if (!layer) return;
                if (layer.lockPosition) {
                    this.notify({ type: 'info', message: this.t('pixel.layer_locked') });
                    return;
                }
                if (layer.sticker) {
                    layer.sticker.matrix[4] += dx; layer.sticker.matrix[5] += dy;
                    this.repaintSticker(layer); this.compositeLayers(); this.pushHistory(this.t('stickers.transform')); return;
                }
                const actx = this.getActiveCtx();
                const w = this.canvas.width, h = this.canvas.height;
                const snap = this.acquireTempCanvas(w, h);
                snap.getContext('2d').drawImage(actx.canvas, 0, 0);
                actx.clearRect(0, 0, w, h);
                actx.drawImage(snap, dx, dy);
                this.releaseTempCanvas(snap);
                this.compositeLayers();
                this.pushHistory('move');
            }),
            magicWandSelect: Pixel.bindRuntime(runtime, function magicWandSelect(x, y, options) {
                if (!this.canvas.width) return;
                const w = this.canvas.width, h = this.canvas.height;
                if (x < 0 || x >= w || y < 0 || y >= h) return;
                const data = this.getActiveReadCtx().getImageData(0, 0, w, h).data;
                const idx = (y * w + x) * 4;
                const sr = data[idx], sg = data[idx + 1], sb = data[idx + 2];
                const alpha = new Uint8ClampedArray(w * h);
                const stack = [x, y];
                const tol = this.fillTolerance;
                let count = 0;
                while (stack.length > 0) {
                    const cy = stack.pop();
                    const cx = stack.pop();
                    if (cx < 0 || cx >= w || cy < 0 || cy >= h) continue;
                    const pos = cy * w + cx;
                    if (alpha[pos]) continue;
                    const pi = pos * 4;
                    if (this.colorDist(data[pi], data[pi + 1], data[pi + 2], sr, sg, sb) > tol) continue;
                    alpha[pos] = 255;
                    count++;
                    stack.push(cx + 1, cy);
                    stack.push(cx - 1, cy);
                    stack.push(cx, cy + 1);
                    stack.push(cx, cy - 1);
                }
                if (!count) return;
                this.setSelectionMask(alpha, { mode: options && options.mode });
            }),
            commitGradient: Pixel.bindRuntime(runtime, function commitGradient(x0, y0, x1, y1) {
                const w = this.canvas.width, h = this.canvas.height;
                const canvas = this.acquireTempCanvas(w, h);
                const gctx = canvas.getContext('2d');
                let grad;
                if (this.gradientMode === 'radial') {
                    const r = Math.hypot(x1 - x0, y1 - y0) || 1;
                    grad = gctx.createRadialGradient(x0, y0, 0, x0, y0, r);
                } else {
                    grad = gctx.createLinearGradient(x0, y0, x1, y1);
                }
                grad.addColorStop(0, this.primaryColor);
                grad.addColorStop(1, this.secondaryColor);
                gctx.fillStyle = grad;
                gctx.fillRect(0, 0, w, h);
                // Brush opacity is the blend amount, so it travels as a uniform mask next to the selection.
                const opacity = this.brushOpacity / 100;
                let mask = null;
                if (opacity < 1) {
                    mask = new Uint8ClampedArray(w * h);
                    mask.fill(Math.round(opacity * 255));
                }
                this.commitLayerPixels(this.activeLayer(), canvas, { label: 'gradient', mask });
                this.releaseTempCanvas(canvas);
                this.addRecentColor(this.primaryColor);
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
                this.compositeLayers();
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
                this.compositeLayers();
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
                this.compositeLayers();
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
            // Rubber band of line, arrow, rectangle, ellipse and the gradient handle (tool-preview overlay).
            drawShapePreview: Pixel.bindRuntime(runtime, function drawShapePreview(x0, y0, x1, y1) {
                const spec = { tool: this.activeTool, color: this.primaryColor, width: this.shapeStrokeWidth, fill: this.shapeFill, opacity: this.brushOpacity / 100 };
                this.toolPreview = ctx => this.paintShapePreview(ctx, spec, x0, y0, x1, y1);
                this.requestOverlay();
            }),
            paintShapePreview: Pixel.bindRuntime(runtime, function paintShapePreview(ctx, spec, x0, y0, x1, y1) {
                ctx.strokeStyle = spec.color;
                ctx.fillStyle = spec.color;
                ctx.lineWidth = spec.width;
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                ctx.globalAlpha = spec.opacity;
                const w = x1 - x0;
                const h = y1 - y0;
                if (spec.tool === 'line') {
                    ctx.beginPath();
                    ctx.moveTo(x0, y0);
                    ctx.lineTo(x1, y1);
                    ctx.stroke();
                } else if (spec.tool === 'arrow') {
                    ctx.beginPath();
                    ctx.moveTo(x0, y0);
                    ctx.lineTo(x1, y1);
                    ctx.stroke();
                    const angle = Math.atan2(y1 - y0, x1 - x0);
                    const headLen = Math.max(10, spec.width * 4);
                    ctx.beginPath();
                    ctx.moveTo(x1, y1);
                    ctx.lineTo(x1 - headLen * Math.cos(angle - Math.PI / 6), y1 - headLen * Math.sin(angle - Math.PI / 6));
                    ctx.moveTo(x1, y1);
                    ctx.lineTo(x1 - headLen * Math.cos(angle + Math.PI / 6), y1 - headLen * Math.sin(angle + Math.PI / 6));
                    ctx.stroke();
                } else if (spec.tool === 'rectangle') {
                    if (spec.fill) ctx.fillRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                    else ctx.strokeRect(Math.min(x0, x1), Math.min(y0, y1), Math.abs(w), Math.abs(h));
                } else if (spec.tool === 'ellipse') {
                    ctx.beginPath();
                    ctx.ellipse(x0 + w / 2, y0 + h / 2, Math.abs(w / 2), Math.abs(h / 2), 0, 0, Math.PI * 2);
                    if (spec.fill) ctx.fill();
                    else ctx.stroke();
                } else if (spec.tool === 'gradient') {
                    ctx.beginPath();
                    ctx.moveTo(x0, y0);
                    ctx.lineTo(x1, y1);
                    ctx.stroke();
                    ctx.beginPath();
                    ctx.arc(x0, y0, 3, 0, Math.PI * 2);
                    ctx.fill();
                    ctx.beginPath();
                    ctx.arc(x1, y1, 3, 0, Math.PI * 2);
                    ctx.fill();
                }
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
                this.compositeLayers();
            }),
            floodFill: Pixel.bindRuntime(runtime, function floodFill(sx, sy, fillColor) {
                const w = this.canvas.width;
                const h = this.canvas.height;
                if (sx < 0 || sy < 0 || sx >= w || sy >= h) return false;
                const data = this.getActiveReadCtx().getImageData(0, 0, w, h).data;
                const idx = (sy * w + sx) * 4;
                const sr = data[idx], sg = data[idx + 1], sb = data[idx + 2], sa = data[idx + 3];
                const fc = this.hexToRgb(fillColor);
                if (sr === fc.r && sg === fc.g && sb === fc.b && sa === 255) return false;
                const region = new Uint8ClampedArray(w * h);
                const stack = [sx, sy];
                const tol = this.fillTolerance;
                while (stack.length > 0) {
                    const cy = stack.pop();
                    const cx = stack.pop();
                    if (cx < 0 || cx >= w || cy < 0 || cy >= h) continue;
                    const pos = cy * w + cx;
                    if (region[pos]) continue;
                    const pi = pos * 4;
                    if (this.colorDist(data[pi], data[pi + 1], data[pi + 2], sr, sg, sb) > tol) continue;
                    region[pos] = 255;
                    stack.push(cx + 1, cy);
                    stack.push(cx - 1, cy);
                    stack.push(cx, cy + 1);
                    stack.push(cx, cy - 1);
                }
                // The flood region is the mask, so selection, alpha lock and locks apply like for every pixel edit.
                const color = this.acquireTempCanvas(w, h);
                const cctx = color.getContext('2d');
                cctx.fillStyle = fillColor;
                cctx.fillRect(0, 0, w, h);
                const done = this.commitLayerPixels(this.activeLayer(), color, { label: 'fill', mask: region });
                this.releaseTempCanvas(color);
                return done;
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
                this.compositeLayers();
                this.pushHistory('text');
            }),
            layersStructureChanged: Pixel.bindRuntime(runtime, function layersStructureChanged() {
                this.compositeLayers();
                this.refreshLayerPanel();
                this.syncActiveLayer();
                this.notifyLayersChanged();
            }),
            addLayer: Pixel.bindRuntime(runtime, function addLayer() {
                if (!this.imgWidth || !this.imgHeight) return;
                if (this.layers.length >= this.MAX_LAYERS) { this.notify({ type: 'error', message: this.t('pixel.layer_max', { n: this.MAX_LAYERS }) }); return; }
                this.commitPendingPreviews();
                const layer = this.createLayer({ name: this.t('pixel.layer') + ' ' + (this.layers.length + 1) });
                this.layers.push(layer);
                this.activeLayerIdx = this.layers.length - 1;
                this.editMaskMode = false;
                this.layersStructureChanged();
                this.pushHistory('add layer');
            }),
            deleteLayer: Pixel.bindRuntime(runtime, function deleteLayer() {
                if (this.layers.length <= 1) return;
                this.discardPendingPreviews();
                this.layers.splice(this.activeLayerIdx, 1);
                this.activeLayerIdx = Math.min(this.activeLayerIdx, this.layers.length - 1);
                this.editMaskMode = false;
                this.layersStructureChanged();
                this.pushHistory('delete layer');
            }),
            duplicateLayer: Pixel.bindRuntime(runtime, function duplicateLayer() {
                if (this.layers.length >= this.MAX_LAYERS) return;
                this.commitPendingPreviews();
                const src = this.activeLayer();
                if (!src) return;
                const copy = this.createLayer({
                    name: src.name + ' copy', visible: true, opacity: src.opacity, blend: src.blend,
                    lockPixels: src.lockPixels, lockAlpha: src.lockAlpha, lockPosition: src.lockPosition,
                    maskEnabled: src.maskEnabled, clip: src.clip,
                    maskCanvas: src.maskCanvas ? this.copyCanvas(src.maskCanvas) : null,
                    source: src.sticker ? null : src.canvas,
                    sticker: src.sticker ? { id: src.sticker.id, matrix: src.sticker.matrix.slice() } : null,
                    stickerImage: src.stickerImage
                });
                this.layers.splice(this.activeLayerIdx + 1, 0, copy);
                this.activeLayerIdx = this.activeLayerIdx + 1;
                this.editMaskMode = false;
                this.repaintSticker(copy);
                this.layersStructureChanged();
                this.pushHistory('duplicate layer');
            }),
            mergeDown: Pixel.bindRuntime(runtime, function mergeDown() {
                if (this.activeLayerIdx <= 0 || this.layers.length < 2) return;
                const upper = this.layers[this.activeLayerIdx];
                const lower = this.layers[this.activeLayerIdx - 1];
                // A hidden layer would pop into view in the merged result; the layers dock refuses the same way.
                if (!upper.visible || !lower.visible) { this.notify({ type: 'info', message: this.t('pixel.layer_merge_hidden') }); return; }
                this.commitPendingPreviews();
                // The lower layer's own blend and clip stay on the result; its mask and opacity are baked in.
                const base = Object.assign({}, lower, { blend: 'normal', clip: false, visible: true });
                const merged = this.makeCanvas(this.imgWidth, this.imgHeight);
                this.renderComposition(merged.getContext('2d'), [base, upper], { previews: false });
                Object.assign(lower, { canvas: merged, opacity: 1, visible: true, maskCanvas: null, maskEnabled: true, preview: null });
                delete lower.sticker; delete lower.stickerImage;
                lower.rev = this.nextRev();
                lower.maskRev = this.nextRev();
                this.layers.splice(this.activeLayerIdx, 1);
                this.activeLayerIdx--;
                this.editMaskMode = false;
                this.layersStructureChanged();
                this.pushHistory('merge down');
            }),
            flattenLayers: Pixel.bindRuntime(runtime, function flattenLayers() {
                if (this.layers.length <= 1 && !this.isStickerLayer()) return;
                this.commitPendingPreviews();
                const flat = this.createLayer({ name: this.t('pixel.layer_background'), source: this.flattenDocument() });
                this.layers = [flat];
                this.activeLayerIdx = 0;
                this.editMaskMode = false;
                this.layersStructureChanged();
                this.pushHistory('flatten');
            })
        });

        // The selection outline and the tool preview are overlays of the compositor in pixel-core.js.
        runtime.registerOverlay('selection', ctx => runtime.drawMarchingAnts(ctx), 50);
        runtime.registerOverlay('tool-preview', ctx => runtime.drawToolPreview(ctx), 60);
    };
})();
