(function () {
    'use strict';

    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    // Layers, masks, history snapshots and caches use plain canvases, never the temp pool: releasing a pooled canvas
    // clears it, which would wipe pixels that other history entries still share.
    function makeCanvas(width, height) {
        const canvas = document.createElement('canvas');
        canvas.width = Math.max(0, Math.round(width) || 0);
        canvas.height = Math.max(0, Math.round(height) || 0);
        canvas.getContext('2d', { willReadFrequently: true });
        return canvas;
    }

    function copyCanvas(source) {
        const copy = makeCanvas(source.width, source.height);
        if (source.width && source.height) copy.getContext('2d').drawImage(source, 0, 0);
        return copy;
    }

    function isDrawable(canvas) {
        return !!(canvas && canvas.width > 0 && canvas.height > 0);
    }

    // --- composition ---

    function maskActive(layer) {
        return !!(layer.maskCanvas && layer.maskEnabled && isDrawable(layer.maskCanvas));
    }

    function layerSource(layer, previews) {
        return previews && layer.preview ? layer.preview : layer.canvas;
    }

    // A clip layer is clipped to the nearest layer below it that is not a clip layer itself.
    function clipBase(layers, index) {
        for (let j = index - 1; j >= 0; j--) {
            if (!layers[j].clip) return layers[j];
        }
        return null;
    }

    function drawLayer(ctx, layers, index, previews) {
        const layer = layers[index];
        if (!layer || !layer.visible || !(layer.opacity > 0)) return;
        const source = layerSource(layer, previews);
        if (!isDrawable(source)) return;
        const masked = maskActive(layer);
        const base = layer.clip ? clipBase(layers, index) : null;
        if (base && (!base.visible || !(base.opacity > 0))) return;
        const op = Pixel.blendOp(layer.blend);
        if (!masked && !base) {
            ctx.globalAlpha = layer.opacity;
            ctx.globalCompositeOperation = op;
            ctx.drawImage(source, 0, 0);
            ctx.globalAlpha = 1;
            ctx.globalCompositeOperation = 'source-over';
            return;
        }
        const w = ctx.canvas.width, h = ctx.canvas.height;
        const tmp = Pixel.acquireTempCanvas(w, h);
        const tctx = tmp.getContext('2d');
        tctx.drawImage(source, 0, 0);
        if (masked) {
            tctx.globalCompositeOperation = 'destination-in';
            tctx.drawImage(layer.maskCanvas, 0, 0);
        }
        if (base) {
            const shape = Pixel.acquireTempCanvas(w, h);
            const sctx = shape.getContext('2d');
            const baseSource = layerSource(base, previews);
            if (isDrawable(baseSource)) {
                sctx.globalAlpha = base.opacity;
                sctx.drawImage(baseSource, 0, 0);
                sctx.globalAlpha = 1;
                if (maskActive(base)) {
                    sctx.globalCompositeOperation = 'destination-in';
                    sctx.drawImage(base.maskCanvas, 0, 0);
                }
            }
            tctx.globalCompositeOperation = 'destination-in';
            tctx.drawImage(shape, 0, 0);
            Pixel.releaseTempCanvas(shape);
        }
        ctx.globalAlpha = layer.opacity;
        ctx.globalCompositeOperation = op;
        ctx.drawImage(tmp, 0, 0);
        ctx.globalAlpha = 1;
        ctx.globalCompositeOperation = 'source-over';
        Pixel.releaseTempCanvas(tmp);
    }

    function renderRange(ctx, layers, from, to, previews) {
        for (let i = from; i < to; i++) drawLayer(ctx, layers, i, previews);
    }

    function countVisible(layers, from, to) {
        let count = 0;
        for (let i = from; i < to; i++) {
            if (layers[i].visible && layers[i].opacity > 0 && isDrawable(layers[i].canvas)) count++;
        }
        return count;
    }

    // Layers above the active one can live in a cache only when they do not depend on what is below them:
    // normal blend and no clipping, so (A over (B over C)) == ((A over B) over C).
    function aboveIsSimple(layers, from, to) {
        for (let i = from; i < to; i++) {
            const layer = layers[i];
            if (!layer.visible || !(layer.opacity > 0)) continue;
            if (layer.clip || Pixel.blendOp(layer.blend) !== 'source-over') return false;
        }
        return true;
    }

    function rangeKey(layers, from, to, w, h, previews) {
        let key = w + 'x' + h;
        for (let i = from; i < to; i++) {
            const l = layers[i];
            key += '|' + l.id + ':' + l.rev + ':' + (l.maskCanvas ? l.maskRev : 0) + ':' + (l.visible ? 1 : 0) + ':' + l.opacity + ':' +
                l.blend + ':' + (l.clip ? 1 : 0) + ':' + (l.maskEnabled ? 1 : 0) + ':' + (previews && l.preview ? (l.previewRev || 1) : 0);
        }
        return key;
    }

    function ensureCache(caches, slot, key, w, h, render) {
        let entry = caches[slot];
        if (!entry) entry = caches[slot] = { key: '', canvas: makeCanvas(w, h) };
        if (entry.canvas.width !== w || entry.canvas.height !== h) {
            entry.canvas.width = w;
            entry.canvas.height = h;
            entry.key = '';
        }
        if (entry.key !== key) {
            const cctx = entry.canvas.getContext('2d');
            cctx.clearRect(0, 0, w, h);
            render(cctx);
            entry.key = key;
        }
        return entry.canvas;
    }

    // --- history ---

    function layerMeta(layer) {
        const meta = {
            id: layer.id, name: layer.name, visible: layer.visible, opacity: layer.opacity, blend: layer.blend,
            lockPixels: layer.lockPixels, lockAlpha: layer.lockAlpha, lockPosition: layer.lockPosition,
            maskEnabled: layer.maskEnabled, clip: layer.clip, rev: layer.rev, maskRev: layer.maskRev, hasMask: !!layer.maskCanvas
        };
        if (layer.sticker) {
            meta.sticker = { id: layer.sticker.id, matrix: layer.sticker.matrix.slice() };
            meta.stickerImage = layer.stickerImage;
        }
        return meta;
    }

    function historyBytes(history) {
        const seen = new Set();
        let total = 0;
        history.forEach(entry => {
            entry.layers.forEach(item => {
                [item.snap, item.maskSnap].forEach(snap => {
                    if (snap && !seen.has(snap)) {
                        seen.add(snap);
                        total += snap.canvas.width * snap.canvas.height * 4;
                    }
                });
            });
            const alpha = entry.selection && entry.selection.alpha;
            if (alpha && !seen.has(alpha)) {
                seen.add(alpha);
                total += alpha.length;
            }
        });
        return total;
    }

    // --- selection masks (document-sized Uint8ClampedArray, 0..255) ---

    function maskBounds(alpha, w, h) {
        let minX = w, minY = h, maxX = -1, maxY = -1;
        for (let y = 0; y < h; y++) {
            const row = y * w;
            let first = -1;
            for (let x = 0; x < w; x++) {
                if (alpha[row + x]) { first = x; break; }
            }
            if (first < 0) continue;
            let last = first;
            for (let x = w - 1; x > first; x--) {
                if (alpha[row + x]) { last = x; break; }
            }
            if (first < minX) minX = first;
            if (last > maxX) maxX = last;
            if (y < minY) minY = y;
            maxY = y;
        }
        return maxX < 0 ? null : { x: minX, y: minY, w: maxX - minX + 1, h: maxY - minY + 1 };
    }

    // Contour pixels (alpha > 127 with a neighbour <= 127) for the marching ants.
    function maskBorder(alpha, w, h, b) {
        const border = [];
        for (let y = b.y; y < b.y + b.h; y++) {
            for (let x = b.x; x < b.x + b.w; x++) {
                const p = y * w + x;
                if (alpha[p] <= 127) continue;
                if ((x > 0 && alpha[p - 1] <= 127) || (x < w - 1 && alpha[p + 1] <= 127) ||
                    (y > 0 && alpha[p - w] <= 127) || (y < h - 1 && alpha[p + w] <= 127)) {
                    border.push(p);
                }
            }
        }
        return border;
    }

    function combineAlpha(current, next, mode) {
        const out = new Uint8ClampedArray(current.length);
        if (mode === 'add') {
            for (let i = 0; i < out.length; i++) out[i] = current[i] > next[i] ? current[i] : next[i];
        } else if (mode === 'subtract') {
            for (let i = 0; i < out.length; i++) out[i] = current[i] * (255 - next[i]) / 255;
        } else {
            for (let i = 0; i < out.length; i++) out[i] = current[i] < next[i] ? current[i] : next[i];
        }
        return out;
    }

    function rasterizeSelection(sel, docW, docH) {
        const rx = Math.round(Math.min(sel.x, sel.x + sel.w)), ry = Math.round(Math.min(sel.y, sel.y + sel.h));
        const rw = Math.round(Math.abs(sel.w)), rh = Math.round(Math.abs(sel.h));
        const x0 = Math.max(0, rx), y0 = Math.max(0, ry);
        const x1 = Math.min(docW, rx + rw), y1 = Math.min(docH, ry + rh);
        if (x1 <= x0 || y1 <= y0) return null;
        const bw = x1 - x0, bh = y1 - y0;
        const alpha = new Uint8ClampedArray(docW * docH);
        if (sel.type === 'ellipse') {
            const tmp = Pixel.acquireTempCanvas(bw, bh);
            const tctx = tmp.getContext('2d');
            tctx.fillStyle = '#ffffff';
            tctx.beginPath();
            tctx.ellipse(rx + rw / 2 - x0, ry + rh / 2 - y0, rw / 2, rh / 2, 0, 0, Math.PI * 2);
            tctx.fill();
            const d = tctx.getImageData(0, 0, bw, bh).data;
            Pixel.releaseTempCanvas(tmp);
            for (let y = 0; y < bh; y++) {
                let p = (y0 + y) * docW + x0, q = y * bw * 4 + 3;
                for (let x = 0; x < bw; x++, p++, q += 4) alpha[p] = d[q];
            }
        } else {
            for (let y = y0; y < y1; y++) alpha.fill(255, y * docW + x0, y * docW + x1);
        }
        return { alpha, x: x0, y: y0, w: bw, h: bh };
    }

    // --- writing pixels through an alpha mask ---

    function alphaCanvas(alpha, docW, box) {
        const canvas = Pixel.acquireTempCanvas(box.w, box.h);
        const actx = canvas.getContext('2d');
        const image = actx.createImageData(box.w, box.h);
        const d = image.data;
        for (let y = 0; y < box.h; y++) {
            let p = (box.y + y) * docW + box.x, q = y * box.w * 4 + 3;
            for (let x = 0; x < box.w; x++, p++, q += 4) d[q] = alpha[p];
        }
        actx.putImageData(image, 0, 0);
        return canvas;
    }

    // dst = dst * (1 - a) + src * a, evaluated on the GPU in premultiplied space.
    function blendThroughAlpha(ctx, source, alpha, docW, box) {
        const mask = alphaCanvas(alpha, docW, box);
        const part = Pixel.acquireTempCanvas(box.w, box.h);
        const pctx = part.getContext('2d');
        pctx.drawImage(source, box.x, box.y, box.w, box.h, 0, 0, box.w, box.h);
        pctx.globalCompositeOperation = 'destination-in';
        pctx.drawImage(mask, 0, 0);
        ctx.save();
        ctx.globalCompositeOperation = 'destination-out';
        ctx.drawImage(mask, box.x, box.y);
        ctx.globalCompositeOperation = 'lighter';
        ctx.drawImage(part, box.x, box.y);
        ctx.restore();
        Pixel.releaseTempCanvas(part);
        Pixel.releaseTempCanvas(mask);
    }

    function replaceRegion(ctx, source, box) {
        ctx.clearRect(box.x, box.y, box.w, box.h);
        ctx.drawImage(source, box.x, box.y, box.w, box.h, box.x, box.y, box.w, box.h);
    }

    // Alpha lock: colour changes, the layer's own alpha channel is kept.
    function commitKeepingAlpha(ctx, source, alpha, docW, box) {
        const work = Pixel.acquireTempCanvas(box.w, box.h);
        const wctx = work.getContext('2d');
        wctx.drawImage(ctx.canvas, box.x, box.y, box.w, box.h, 0, 0, box.w, box.h);
        wctx.translate(-box.x, -box.y);
        if (alpha) blendThroughAlpha(wctx, source, alpha, docW, box);
        else replaceRegion(wctx, source, box);
        wctx.setTransform(1, 0, 0, 1, 0, 0);
        const next = wctx.getImageData(0, 0, box.w, box.h);
        const old = ctx.getImageData(box.x, box.y, box.w, box.h);
        const n = next.data, o = old.data;
        for (let i = 0; i < n.length; i += 4) {
            const oa = o[i + 3];
            if (oa === 0) { n[i] = n[i + 1] = n[i + 2] = n[i + 3] = 0; continue; }
            if (n[i + 3] === 0) { n[i] = o[i]; n[i + 1] = o[i + 1]; n[i + 2] = o[i + 2]; }
            n[i + 3] = oa;
        }
        ctx.putImageData(next, box.x, box.y);
        Pixel.releaseTempCanvas(work);
    }

    Pixel.makeCanvas = makeCanvas;
    Pixel.copyCanvas = copyCanvas;

    Pixel.installCore = function installCore(runtime) {
        const listeners = Object.create(null);
        const caches = { below: null, above: null, flat: null };
        let layersFrame = 0;
        let lastActiveLayerId = null;
        let selectionCache = null;
        let disposed = false;

        Object.assign(runtime, {
            nextLayerId: 0,
            revSeq: 0,
            selectionRev: 0,
            makeCanvas,
            copyCanvas,

            // --- event bus ---
            on: Pixel.bindRuntime(runtime, function on(event, fn) {
                if (typeof fn !== 'function') return () => {};
                (listeners[event] = listeners[event] || new Set()).add(fn);
                return () => { if (listeners[event]) listeners[event].delete(fn); };
            }),
            emit: Pixel.bindRuntime(runtime, function emit(event, data) {
                const set = listeners[event];
                if (!set || !set.size) return;
                Array.from(set).forEach(fn => {
                    try { fn(data); } catch (err) { if (window.console) console.error('pixel ' + event, err); }
                });
            }),
            // layers-changed is coalesced to one event per animation frame.
            notifyLayersChanged: Pixel.bindRuntime(runtime, function notifyLayersChanged() {
                if (disposed || layersFrame) return;
                if (typeof requestAnimationFrame !== 'function') { this.emit('layers-changed'); return; }
                layersFrame = requestAnimationFrame(() => {
                    layersFrame = 0;
                    if (disposed) return;
                    this.syncActiveLayer();
                    this.emit('layers-changed');
                });
            }),
            // active-layer-changed fires when the identity of the active layer changes, not on every index write.
            syncActiveLayer: Pixel.bindRuntime(runtime, function syncActiveLayer() {
                const layer = this.activeLayer();
                if (!layer || layer.id === lastActiveLayerId) return;
                lastActiveLayerId = layer.id;
                this.emit('active-layer-changed', { id: layer.id, index: this.activeLayerIdx });
            }),
            disposeCore: Pixel.bindRuntime(runtime, function disposeCore() {
                disposed = true;
                if (layersFrame && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(layersFrame);
                layersFrame = 0;
                Object.keys(listeners).forEach(key => delete listeners[key]);
                caches.below = caches.above = caches.flat = null;
                selectionCache = null;
            }),

            // --- layers ---
            nextRev: Pixel.bindRuntime(runtime, function nextRev() {
                return ++this.revSeq;
            }),
            createLayer: Pixel.bindRuntime(runtime, function createLayer(opts) {
                const o = opts || {};
                const w = o.width != null ? o.width : this.imgWidth;
                const h = o.height != null ? o.height : this.imgHeight;
                const canvas = o.canvas || makeCanvas(w, h);
                let id = o.id;
                if (!id) id = ++this.nextLayerId;
                else if (id > this.nextLayerId) this.nextLayerId = id;
                const layer = {
                    id,
                    name: o.name != null ? o.name : '',
                    canvas,
                    visible: o.visible !== false,
                    opacity: typeof o.opacity === 'number' ? o.opacity : 1,
                    blend: Pixel.BLEND_MODES.includes(o.blend) ? o.blend : 'normal',
                    lockPixels: !!o.lockPixels,
                    lockAlpha: !!o.lockAlpha,
                    lockPosition: !!o.lockPosition,
                    maskCanvas: o.maskCanvas || null,
                    maskEnabled: o.maskEnabled !== false,
                    clip: !!o.clip,
                    rev: o.rev || this.nextRev(),
                    maskRev: o.maskRev || this.nextRev(),
                    preview: null
                };
                if (o.sticker) {
                    layer.sticker = o.sticker;
                    layer.stickerImage = o.stickerImage;
                }
                if (o.source && isDrawable(canvas)) canvas.getContext('2d').drawImage(o.source, 0, 0);
                return layer;
            }),
            activeLayer: Pixel.bindRuntime(runtime, function activeLayer() {
                return this.layers[this.activeLayerIdx] || null;
            }),
            layerById: Pixel.bindRuntime(runtime, function layerById(id) {
                return this.layers.find(layer => layer.id === id) || null;
            }),
            layerIndexById: Pixel.bindRuntime(runtime, function layerIndexById(id) {
                return this.layers.findIndex(layer => layer.id === id);
            }),
            // Cheap; called on every stroke move. Revs key the composite caches and the history snapshots.
            markLayerDirty: Pixel.bindRuntime(runtime, function markLayerDirty(layer = this.activeLayer(), { mask = false } = {}) {
                if (!layer) return;
                if (mask) layer.maskRev = this.nextRev();
                else layer.rev = this.nextRev();
                this.notifyLayersChanged();
            }),
            invalidateCompositeCaches: Pixel.bindRuntime(runtime, function invalidateCompositeCaches() {
                caches.below = caches.above = caches.flat = null;
            }),
            // Write access to the active layer (or its mask in mask-edit mode); marks it dirty.
            getActiveCtx: Pixel.bindRuntime(runtime, function getActiveCtx() {
                const layer = this.activeLayer();
                if (!layer) return null;
                const inMask = !!(this.editMaskMode && layer.maskCanvas);
                // A pending adjust/filter preview would cover whatever is painted, so bake it in first.
                if (!inMask && layer.preview && this.commitPendingPreviews) this.commitPendingPreviews();
                this.markLayerDirty(layer, { mask: inMask });
                return (inMask ? layer.maskCanvas : layer.canvas).getContext('2d');
            }),
            getActiveReadCtx: Pixel.bindRuntime(runtime, function getActiveReadCtx() {
                const layer = this.activeLayer();
                if (!layer) return null;
                return (this.editMaskMode && layer.maskCanvas ? layer.maskCanvas : layer.canvas).getContext('2d');
            }),
            // Gate for tools that paint: sticker layers need rasterizing, locked layers refuse pixels.
            canPaintActiveLayer: Pixel.bindRuntime(runtime, function canPaintActiveLayer(tool) {
                const layer = this.activeLayer();
                if (!layer || !this.requireRasterLayer(layer)) return false;
                const inMask = !!(this.editMaskMode && layer.maskCanvas);
                if (!inMask && (layer.lockPixels || (layer.lockAlpha && tool === 'eraser'))) {
                    this.notify({ type: 'info', message: this.t('pixel.layer_locked', 'This layer is locked.') });
                    return false;
                }
                return true;
            }),

            // --- composition ---
            flattenDocument: Pixel.bindRuntime(runtime, function flattenDocument(target) {
                const w = this.imgWidth || this.canvas.width, h = this.imgHeight || this.canvas.height;
                const out = target || makeCanvas(w, h);
                if (out.width !== w || out.height !== h) { out.width = w; out.height = h; }
                this.renderComposition(out.getContext('2d'), this.layers, { previews: false });
                return out;
            }),
            renderComposition: Pixel.bindRuntime(runtime, function renderComposition(ctx, layers = this.layers, { previews = true } = {}) {
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.globalCompositeOperation = 'source-over';
                ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
                renderRange(ctx, layers, 0, layers.length, previews);
            }),
            compositeLayers: Pixel.bindRuntime(runtime, function compositeLayers() {
                const ctx = this.cctx;
                if (!ctx || !this.canvas || !this.canvas.width || !this.canvas.height) return;
                const layers = this.layers, count = layers.length;
                const w = this.canvas.width, h = this.canvas.height;
                ctx.setTransform(1, 0, 0, 1, 0, 0);
                ctx.globalAlpha = 1;
                ctx.globalCompositeOperation = 'source-over';
                ctx.clearRect(0, 0, w, h);
                const active = Math.max(0, Math.min(this.activeLayerIdx, count - 1));
                let belowCanvas = null, aboveCanvas = null, from = 0, to = count;
                if (active >= 2 && countVisible(layers, 0, active) >= 2) {
                    belowCanvas = ensureCache(caches, 'below', rangeKey(layers, 0, active, w, h, true), w, h, c => renderRange(c, layers, 0, active, true));
                    from = active;
                }
                if (active < count - 2 && !layers[active].clip && aboveIsSimple(layers, active + 1, count) && countVisible(layers, active + 1, count) >= 2) {
                    aboveCanvas = ensureCache(caches, 'above', rangeKey(layers, active + 1, count, w, h, true), w, h, c => renderRange(c, layers, active + 1, count, true));
                    to = active + 1;
                }
                if (!belowCanvas) caches.below = null;
                if (!aboveCanvas) caches.above = null;
                if (belowCanvas) ctx.drawImage(belowCanvas, 0, 0);
                renderRange(ctx, layers, from, to, true);
                if (aboveCanvas) ctx.drawImage(aboveCanvas, 0, 0);
            }),
            // Colour of the composite at a point. The display canvas is used while it shows no preview.
            sampleComposite: Pixel.bindRuntime(runtime, function sampleComposite(x, y) {
                const w = this.canvas.width, h = this.canvas.height;
                if (x < 0 || y < 0 || x >= w || y >= h) return [0, 0, 0, 0];
                if (!this.compareMode && !this.layers.some(layer => layer.preview)) {
                    caches.flat = null;
                    return this.cctx.getImageData(x, y, 1, 1).data;
                }
                const layers = this.layers;
                const flat = ensureCache(caches, 'flat', rangeKey(layers, 0, layers.length, w, h, false), w, h, c => renderRange(c, layers, 0, layers.length, false));
                return flat.getContext('2d').getImageData(x, y, 1, 1).data;
            }),
            // Non-destructive preview: the canvas replaces the layer's pixels on screen until it is committed or cleared.
            // Redrawing the same preview canvas in place needs another call so caches see the change.
            setLayerPreview: Pixel.bindRuntime(runtime, function setLayerPreview(layer, canvas) {
                if (!layer) return;
                layer.preview = canvas || null;
                layer.previewRev = canvas ? this.nextRev() : 0;
                this.compositeLayers();
                this.notifyLayersChanged();
            }),

            // --- history (structural sharing) ---
            pushHistory: Pixel.bindRuntime(runtime, function pushHistory(label) {
                const w = this.canvas.width, h = this.canvas.height;
                if (!w || !h) return;
                const previous = this.history[this.historyIdx] || null;
                const earlier = new Map();
                if (previous && previous.width === w && previous.height === h) {
                    previous.layers.forEach(item => earlier.set(item.meta.id, item));
                }
                const layers = this.layers.map(l => {
                    const old = earlier.get(l.id);
                    let snap = null, maskSnap = null;
                    if (!l.sticker && isDrawable(l.canvas)) {
                        if (old && old.snap && old.snap.rev === l.rev) snap = old.snap;
                        else snap = { id: l.id, rev: l.rev, canvas: copyCanvas(l.canvas) };
                    }
                    if (l.maskCanvas && isDrawable(l.maskCanvas)) {
                        if (old && old.maskSnap && old.maskSnap.maskRev === l.maskRev) maskSnap = old.maskSnap;
                        else maskSnap = { id: l.id, maskRev: l.maskRev, canvas: copyCanvas(l.maskCanvas) };
                    }
                    return { meta: layerMeta(l), snap, maskSnap };
                });
                if (this.historyIdx < this.history.length - 1) this.history = this.history.slice(0, this.historyIdx + 1);
                const active = this.activeLayer();
                this.history.push({
                    label: label || '', width: w, height: h, activeLayerId: active ? active.id : 0,
                    selection: this.selection || null, layers
                });
                while (this.history.length > this.MAX_HISTORY) this.history.shift();
                while (this.history.length > 2 && historyBytes(this.history) > Pixel.HISTORY_BYTE_BUDGET) this.history.shift();
                this.historyIdx = this.history.length - 1;
                this.isDirty = true;
                this.updateStatus();
                if (this.refreshHistoryPanel) this.refreshHistoryPanel();
                this.emit('history-changed');
            }),
            restoreHistory: Pixel.bindRuntime(runtime, function restoreHistory(entry) {
                this.cancelStickerGesture();
                this.resetFilterPreview();
                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                this.resetAdjustmentControls();
                this.adjustPreview = null;
                const w = entry.width, h = entry.height;
                const sizeChanged = w !== this.canvas.width || h !== this.canvas.height;
                if (sizeChanged) {
                    this.canvas.width = w;
                    this.canvas.height = h;
                    this.imgWidth = w;
                    this.imgHeight = h;
                    this.overlayCanvas.width = w;
                    this.overlayCanvas.height = h;
                } else {
                    this.clearOverlay();
                }
                const current = new Map(this.layers.map(l => [l.id, l]));
                this.layers = entry.layers.map(item => {
                    const meta = item.meta;
                    const old = current.get(meta.id);
                    let canvas;
                    if (!meta.sticker && old && !old.sticker && !sizeChanged && isDrawable(old.canvas) && old.rev === meta.rev) {
                        canvas = old.canvas;
                    } else {
                        canvas = makeCanvas(w, h);
                        if (item.snap) canvas.getContext('2d').drawImage(item.snap.canvas, 0, 0);
                    }
                    let maskCanvas = null;
                    if (meta.hasMask) {
                        if (old && old.maskCanvas && !sizeChanged && old.maskRev === meta.maskRev) {
                            maskCanvas = old.maskCanvas;
                        } else {
                            maskCanvas = makeCanvas(w, h);
                            const mctx = maskCanvas.getContext('2d');
                            if (item.maskSnap) {
                                mctx.drawImage(item.maskSnap.canvas, 0, 0);
                            } else {
                                mctx.fillStyle = '#ffffff';
                                mctx.fillRect(0, 0, w, h);
                            }
                        }
                    }
                    return this.createLayer({
                        id: meta.id, name: meta.name, visible: meta.visible, opacity: meta.opacity, blend: meta.blend,
                        lockPixels: meta.lockPixels, lockAlpha: meta.lockAlpha, lockPosition: meta.lockPosition,
                        maskEnabled: meta.maskEnabled, clip: meta.clip, rev: meta.rev, maskRev: meta.maskRev,
                        canvas, maskCanvas,
                        sticker: meta.sticker ? { id: meta.sticker.id, matrix: meta.sticker.matrix.slice() } : null,
                        stickerImage: meta.stickerImage
                    });
                });
                this.layers.forEach(layer => { if (layer.sticker) this.repaintSticker(layer); });
                let index = this.layers.findIndex(layer => layer.id === entry.activeLayerId);
                if (index < 0) index = Math.max(0, Math.min(this.activeLayerIdx, this.layers.length - 1));
                this.activeLayerIdx = index;
                const layer = this.activeLayer();
                if (this.editMaskMode && !(layer && layer.maskCanvas)) this.editMaskMode = false;
                this.selImageData = null;
                this.moveSelFloating = false;
                this.selection = entry.selection || null;
                if (this.selection) {
                    this.startMarchingAnts();
                } else {
                    this.stopMarchingAnts();
                    this.clearOverlay();
                }
                this.captureAdjustmentSource();
                this.compositeLayers();
                this.refreshLayerPanel();
                this.applyZoom();
                this.syncActiveLayer();
                this.notifyLayersChanged();
                if (sizeChanged) this.emit('document-changed', { width: w, height: h, reason: 'history' });
            }),

            // --- selection ---
            // Document-sized alpha for the current selection (rect/ellipse/lasso rasterised once per selection change).
            getSelectionMask: Pixel.bindRuntime(runtime, function getSelectionMask() {
                const sel = this.selection;
                const w = this.canvas ? this.canvas.width : 0, h = this.canvas ? this.canvas.height : 0;
                if (!sel || !w || !h) return null;
                if (selectionCache && selectionCache.sel === sel && selectionCache.rev === this.selectionRev && selectionCache.w === w && selectionCache.h === h) {
                    return selectionCache.result;
                }
                let result = null;
                if (sel.type === 'mask') {
                    if (sel.alpha && sel.alpha.length === w * h) result = { alpha: sel.alpha, x: sel.x, y: sel.y, w: sel.w, h: sel.h };
                } else {
                    result = rasterizeSelection(sel, w, h);
                }
                selectionCache = { sel, rev: this.selectionRev, w, h, result };
                return result;
            }),
            // mode: replace | add | subtract | intersect. The array is adopted as is; callers must not change it afterwards.
            setSelectionMask: Pixel.bindRuntime(runtime, function setSelectionMask(alpha, options) {
                const w = this.canvas.width, h = this.canvas.height;
                if (!w || !h || !alpha || alpha.length !== w * h) return false;
                const mode = (options && options.mode) || 'replace';
                let out = alpha instanceof Uint8ClampedArray ? alpha : Uint8ClampedArray.from(alpha);
                if (mode === 'add' || mode === 'subtract' || mode === 'intersect') {
                    const current = this.getSelectionMask();
                    if (current) out = combineAlpha(current.alpha, out, mode);
                    else if (mode !== 'add') out = new Uint8ClampedArray(w * h);
                }
                const bounds = maskBounds(out, w, h);
                if (!bounds) {
                    this.deselect();
                    return false;
                }
                this.selection = { type: 'mask', x: bounds.x, y: bounds.y, w: bounds.w, h: bounds.h, alpha: out, border: maskBorder(out, w, h, bounds) };
                this.startMarchingAnts();
                return true;
            }),
            invertSelection: Pixel.bindRuntime(runtime, function invertSelection() {
                const current = this.getSelectionMask();
                if (!current) return false;
                const inverted = new Uint8ClampedArray(current.alpha.length);
                for (let i = 0; i < inverted.length; i++) inverted[i] = 255 - current.alpha[i];
                return this.setSelectionMask(inverted, { mode: 'replace' });
            }),

            // --- committing pixels ---
            // source: document-sized canvas or ImageData. opts: { label, respectSelection = true, mask = null, history = true };
            // mask is an extra document-sized alpha (Uint8ClampedArray or { alpha }) multiplied with the selection.
            commitLayerPixels: Pixel.bindRuntime(runtime, function commitLayerPixels(layer, source, opts) {
                const o = opts || {};
                layer = layer || this.activeLayer();
                if (!layer || !source || !this.canvas.width) return false;
                if (!this.requireRasterLayer(layer)) return false;
                const inMask = !!(this.editMaskMode && layer === this.activeLayer() && layer.maskCanvas);
                if (layer.lockPixels && !inMask) {
                    this.notify({ type: 'info', message: this.t('pixel.layer_locked', 'This layer is locked.') });
                    return false;
                }
                const w = this.canvas.width, h = this.canvas.height;
                const target = inMask ? layer.maskCanvas : layer.canvas;
                const ctx = target.getContext('2d');
                let src = source, ownSource = null;
                if (typeof ImageData !== 'undefined' && source instanceof ImageData) {
                    ownSource = Pixel.acquireTempCanvas(source.width, source.height);
                    ownSource.getContext('2d').putImageData(source, 0, 0);
                    src = ownSource;
                }
                const selection = o.respectSelection === false ? null : this.getSelectionMask();
                const extra = o.mask ? (o.mask.alpha || o.mask) : null;
                if (o.respectSelection !== false && this.selection && !selection) {
                    if (ownSource) Pixel.releaseTempCanvas(ownSource);
                    return false;
                }
                let alpha = null, box = { x: 0, y: 0, w, h };
                if (selection && extra) {
                    alpha = new Uint8ClampedArray(w * h);
                    for (let y = selection.y; y < selection.y + selection.h; y++) {
                        for (let p = y * w + selection.x, end = p + selection.w; p < end; p++) alpha[p] = selection.alpha[p] * extra[p] / 255;
                    }
                    box = { x: selection.x, y: selection.y, w: selection.w, h: selection.h };
                } else if (selection) {
                    alpha = selection.alpha;
                    box = { x: selection.x, y: selection.y, w: selection.w, h: selection.h };
                } else if (extra) {
                    alpha = extra;
                    box = maskBounds(extra, w, h);
                    if (!box) {
                        if (ownSource) Pixel.releaseTempCanvas(ownSource);
                        return false;
                    }
                }
                const opaque = !alpha || (selection && !extra && this.selection.type === 'rect');
                if (layer.lockAlpha && !inMask) commitKeepingAlpha(ctx, src, opaque ? null : alpha, w, box);
                else if (opaque) replaceRegion(ctx, src, box);
                else blendThroughAlpha(ctx, src, alpha, w, box);
                if (ownSource) Pixel.releaseTempCanvas(ownSource);
                this.markLayerDirty(layer, { mask: inMask });
                this.compositeLayers();
                if (o.history !== false) this.pushHistory(o.label || 'edit');
                return true;
            })
        });
    };
})();
