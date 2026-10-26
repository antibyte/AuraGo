// Pixel module "transform": move tool (layer or selection pixels, smart guides, nudge, auto-select), free transform (Ctrl+T),
// flip / rotate of the layer, and the crop tool. Pure geometry lives in Pixel.transformMath (tested in scripts/test-pixel-tools.mjs).
(function () {
    'use strict';
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    const Brush = Pixel.brush, toolUi = Pixel.toolUi;
    const T = Pixel.transformMath = {};
    const clamp = (v, lo, hi) => (v < lo ? lo : (v > hi ? hi : v));
    if (!Brush || !toolUi) {
        if (window.console) console.error('pixel-transform.js needs pixel-brush.js');
        return;
    }
    const { F, opt } = toolUi;
    const svg = body => '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">' + body + '</svg>';
    const HANDLES = [[-1, -1], [0, -1], [1, -1], [1, 0], [1, 1], [0, 1], [-1, 1], [-1, 0]];
    const lockedNote = r => r.notify({ type: 'info', message: r.t('pixel.layer_locked') });

    // --- pure geometry ---

    // Snaps a dragged box {x0, y0, x1, y1} (edges and centre) to target lines {xs, ys}; thr in document px.
    // Returns the offset to add plus the lines that line up afterwards (for the guides).
    T.snapBox = function snapBox(box, targets, thr) {
        const axis = (refs, lines) => {
            let best = null;
            refs.forEach(ref => lines.forEach(ln => { const d = ln - ref; if (Math.abs(d) <= thr && (best === null || Math.abs(d) < Math.abs(best))) best = d; }));
            if (best === null) return { off: 0, hits: [] };
            const hits = [];
            refs.forEach(ref => lines.forEach(ln => { if (Math.abs(ln - (ref + best)) < 0.5 && !hits.includes(ln)) hits.push(ln); }));
            return { off: best, hits };
        };
        const sx = axis([box.x0, (box.x0 + box.x1) / 2, box.x1], targets.xs), sy = axis([box.y0, (box.y0 + box.y1) / 2, box.y1], targets.ys);
        return { dx: sx.off, dy: sy.off, xs: sx.hits, ys: sy.hits };
    };

    // Content (w0 x h0, origin top-left) -> document matrix [a, b, c, d, e, f] of a transform g = { cx, cy, sx, sy, rot }.
    T.matrixOf = function matrixOf(g, w0, h0) {
        const cos = Math.cos(g.rot), sin = Math.sin(g.rot), a = cos * g.sx, b = sin * g.sx, c = -sin * g.sy, d = cos * g.sy;
        return [a, b, c, d, g.cx - a * w0 / 2 - c * h0 / 2, g.cy - b * w0 / 2 - d * h0 / 2];
    };
    T.quad = function quad(g, w0, h0) {
        const m = T.matrixOf(g, w0, h0);
        return [[0, 0], [w0, 0], [w0, h0], [0, h0]].map(([u, v]) => ({ x: m[0] * u + m[2] * v + m[4], y: m[1] * u + m[3] * v + m[5] }));
    };

    // What the pointer is over: a scale handle (hx, hy in -1..1), the inside (move) or the outside (rotate).
    T.hitTest = function hitTest(g, w0, h0, p, hr) {
        const cos = Math.cos(g.rot), sin = Math.sin(g.rot), dx = p.x - g.cx, dy = p.y - g.cy;
        const lx = dx * cos + dy * sin, ly = -dx * sin + dy * cos, hw = Math.abs(g.sx) * w0 / 2, hh = Math.abs(g.sy) * h0 / 2;
        for (let i = 0; i < HANDLES.length; i++) if (Math.hypot(lx - HANDLES[i][0] * hw, ly - HANDLES[i][1] * hh) <= hr) return { kind: 'scale', hx: HANDLES[i][0], hy: HANDLES[i][1] };
        return Math.abs(lx) <= hw && Math.abs(ly) <= hh ? { kind: 'move' } : { kind: 'rotate' };
    };

    // New transform when scale handle (hx, hy) is dragged to p. g0 is the transform at the start of the drag.
    // shift keeps the aspect ratio (corners), alt scales from the centre. Negative scale = flipped.
    T.scaleDrag = function scaleDrag(g0, hx, hy, p, w0, h0, shift, alt) {
        const cos = Math.cos(g0.rot), sin = Math.sin(g0.rot), dx = p.x - g0.cx, dy = p.y - g0.cy;
        const lx = dx * cos + dy * sin, ly = -dx * sin + dy * cos, hw0 = Math.abs(g0.sx) * w0 / 2, hh0 = Math.abs(g0.sy) * h0 / 2;
        const ax = hx && !alt ? -hx * hw0 : 0, ay = hy && !alt ? -hy * hh0 : 0;
        let nw = hw0 * 2, nh = hh0 * 2, flipX = false, flipY = false;
        const spanX = lx - ax, spanY = ly - ay;
        if (hx) { nw = alt ? Math.abs(lx) * 2 : Math.abs(spanX); flipX = spanX * hx < 0; }
        if (hy) { nh = alt ? Math.abs(ly) * 2 : Math.abs(spanY); flipY = spanY * hy < 0; }
        if (shift && hx && hy) {
            const k = Math.max(nw / (hw0 * 2 || 1), nh / (hh0 * 2 || 1));
            nw = hw0 * 2 * k; nh = hh0 * 2 * k;
        }
        nw = Math.max(1, nw); nh = Math.max(1, nh);
        const ncx = hx && !alt ? ax + (spanX < 0 ? -1 : 1) * nw / 2 : 0, ncy = hy && !alt ? ay + (spanY < 0 ? -1 : 1) * nh / 2 : 0;
        return {
            cx: g0.cx + ncx * cos - ncy * sin, cy: g0.cy + ncx * sin + ncy * cos, rot: g0.rot,
            sx: (g0.sx < 0 !== flipX ? -1 : 1) * nw / w0, sy: (g0.sy < 0 !== flipY ? -1 : 1) * nh / h0
        };
    };

    // Crop rectangle {x0, y0, x1, y1} after dragging handle (hx, hy) to p; ratio = w / h or null; bounds W x H.
    T.cropDrag = function cropDrag(rect, hx, hy, p, ratio, W, H) {
        const px = clamp(p.x, 0, W), py = clamp(p.y, 0, H);
        const ax = hx < 0 ? rect.x1 : (hx > 0 ? rect.x0 : (rect.x0 + rect.x1) / 2), ay = hy < 0 ? rect.y1 : (hy > 0 ? rect.y0 : (rect.y0 + rect.y1) / 2);
        let w = hx ? Math.abs(px - ax) : rect.x1 - rect.x0, h = hy ? Math.abs(py - ay) : rect.y1 - rect.y0;
        const sx = px >= ax ? 1 : -1, sy = py >= ay ? 1 : -1;
        if (ratio) {
            if (hx && hy) { if (w / Math.max(h, 1e-6) > ratio) h = w / ratio; else w = h * ratio; }
            else if (hx) h = w / ratio;
            else w = h * ratio;
            const availW = hx ? (sx > 0 ? W - ax : ax) : 2 * Math.min(ax, W - ax), availH = hy ? (sy > 0 ? H - ay : ay) : 2 * Math.min(ay, H - ay);
            const f = Math.min(1, availW / Math.max(w, 1e-6), availH / Math.max(h, 1e-6));
            w *= f; h *= f;
        }
        const x0 = hx ? (sx > 0 ? ax : ax - w) : ax - w / 2, y0 = hy ? (sy > 0 ? ay : ay - h) : ay - h / 2;
        return { x0, y0, x1: x0 + w, y1: y0 + h };
    };
    T.cropMove = function cropMove(rect, dx, dy, W, H) {
        const w = rect.x1 - rect.x0, h = rect.y1 - rect.y0, x0 = clamp(rect.x0 + dx, 0, W - w), y0 = clamp(rect.y0 + dy, 0, H - h);
        return { x0, y0, x1: x0 + w, y1: y0 + h };
    };
    T.hitCrop = function hitCrop(c, p, t) {
        const inX = p.x >= c.x0 - t && p.x <= c.x1 + t, inY = p.y >= c.y0 - t && p.y <= c.y1 + t;
        const hx = Math.abs(p.x - c.x0) <= t && inY ? -1 : (Math.abs(p.x - c.x1) <= t && inY ? 1 : 0);
        const hy = Math.abs(p.y - c.y0) <= t && inX ? -1 : (Math.abs(p.y - c.y1) <= t && inX ? 1 : 0);
        if (hx || hy) return { kind: 'edge', hx, hy };
        return p.x > c.x0 && p.x < c.x1 && p.y > c.y0 && p.y < c.y1 ? { kind: 'move' } : { kind: 'outside' };
    };
    // Keeps the centre and shrinks the larger side so the rectangle has the ratio.
    T.cropRatio = function cropRatio(rect, ratio) {
        const cx = (rect.x0 + rect.x1) / 2, cy = (rect.y0 + rect.y1) / 2;
        let w = rect.x1 - rect.x0, h = rect.y1 - rect.y0;
        if (w / h > ratio) w = h * ratio; else h = w / ratio;
        return { x0: cx - w / 2, y0: cy - h / 2, x1: cx + w / 2, y1: cy + h / 2 };
    };

    // Moves an alpha mask; whatever leaves the document is dropped.
    T.shiftAlpha = function shiftAlpha(alpha, w, h, dx, dy) {
        const out = new Uint8ClampedArray(alpha.length), x0 = Math.max(0, -dx), x1 = Math.min(w, w - dx);
        if (x1 <= x0) return out;
        for (let y = 0; y < h; y++) if (y + dy >= 0 && y + dy < h) out.set(alpha.subarray(y * w + x0, y * w + x1), (y + dy) * w + x0 + dx);
        return out;
    };

    // --- layer bounds and picking ---

    // Bounding box of the non-transparent pixels, scanned in strips so big layers need little memory. null = empty.
    function contentBounds(canvas) {
        const w = canvas.width, h = canvas.height;
        if (!w || !h) return null;
        const ctx = canvas.getContext('2d'), strip = Math.max(1, Math.min(h, Math.floor(4000000 / w)));
        let x0 = w, y0 = h, x1 = -1, y1 = -1;
        for (let y = 0; y < h; y += strip) {
            const sh = Math.min(strip, h - y), d = ctx.getImageData(0, y, w, sh).data;
            for (let j = 0; j < sh; j++) {
                const row = j * w * 4;
                let first = -1, last = -1;
                for (let i = 0; i < w; i++) if (d[row + i * 4 + 3]) { first = i; break; }
                if (first < 0) continue;
                for (let i = w - 1; i >= first; i--) if (d[row + i * 4 + 3]) { last = i; break; }
                if (first < x0) x0 = first;
                if (last > x1) x1 = last;
                if (y + j < y0) y0 = y + j;
                y1 = y + j;
            }
        }
        return x1 < 0 ? null : { x0, y0, x1: x1 + 1, y1: y1 + 1 };
    }
    function layerBounds(r, layer) {
        const cache = r.boundsCache || (r.boundsCache = new Map()), hit = cache.get(layer.id);
        if (hit && hit.rev === layer.rev && hit.w === layer.canvas.width) return hit.box;
        const box = contentBounds(layer.canvas);
        cache.set(layer.id, { rev: layer.rev, w: layer.canvas.width, box });
        return box;
    }
    function pickLayerAt(r, x, y) {
        const ix = Math.floor(x), iy = Math.floor(y);
        if (ix < 0 || iy < 0 || ix >= r.imgWidth || iy >= r.imgHeight) return -1;
        for (let i = r.layers.length - 1; i >= 0; i--) {
            const l = r.layers[i];
            if (!l.visible || !(l.opacity > 0) || !l.canvas.width || !l.canvas.getContext('2d').getImageData(ix, iy, 1, 1).data[3]) continue;
            if (l.maskCanvas && l.maskEnabled && !l.maskCanvas.getContext('2d').getImageData(ix, iy, 1, 1).data[3]) continue;
            return i;
        }
        return -1;
    }
    function selectLayer(r, idx) {
        r.resetFilterPreview(); r.commitPendingPreviews();
        r.activeLayerIdx = idx;
        r.cancelStickerGesture();
        if (r.editMaskMode) { r.editMaskMode = false; r.notify({ type: 'info', message: r.t('pixel.mask_edit_off') }); }
        r.captureAdjustmentSource(); r.refreshLayerPanel();
    }

    // --- move tool ---

    const MOVE_CFG = {
        id: 'move',
        schema: [F('autoSelect', 'bool', 'pixel.move_auto_select', 'Auto-select layer', { default: false }), F('snap', 'bool', 'pixel.view_snap', 'Snap to Guides', { default: true })],
        tail: r => '<span class="pixel-opt-hint">' + r.esc(r.t('pixel.move_hint_pro')) + '</span>',
        onChange: (r, key, value) => { if (key === 'snap') { r.viewOpts.snap = value; r.refreshMenus(); } }
    };

    // Starts moving the active layer (pixels + linked mask) or the selected pixels; null when nothing can move.
    function beginMove(r, p, copy) {
        const layer = r.activeLayer();
        if (!layer || layer.sticker) return null;
        if (layer.lockPosition) { lockedNote(r); return null; }
        // The move tool is not a raster tool, so the event layer does not bake a pending filter/adjust preview for it.
        r.commitPendingPreviews();
        const inMask = !!(r.editMaskMode && layer.maskCanvas), w = r.imgWidth, h = r.imgHeight;
        const mv = { layer, inMask, start: { x: p.x, y: p.y }, dx: 0, dy: 0, w, h, copy: false, guides: null };
        if (r.selection) {
            if (!r.canPaintActiveLayer('move')) return null;
            const m = r.getSelectionMask();
            if (!m) return null;
            Object.assign(mv, { m, sel0: r.selection, copy: !!copy, box: { x0: m.x, y0: m.y, x1: m.x + m.w, y1: m.y + m.h } });
            const before = Brush.pristine(r, layer, inMask), alphaC = r.selection.type === 'rect' ? null : Brush.alphaCanvas(m.alpha, w, m);
            mv.lifted = r.makeCanvas(m.w, m.h);
            const lc = mv.lifted.getContext('2d');
            lc.drawImage(before, m.x, m.y, m.w, m.h, 0, 0, m.w, m.h);
            if (alphaC) { lc.globalCompositeOperation = 'destination-in'; lc.drawImage(alphaC, 0, 0); }
            mv.base = r.makeCanvas(w, h);
            const bc = mv.base.getContext('2d');
            bc.drawImage(before, 0, 0);
            if (!mv.copy) { if (alphaC) { bc.globalCompositeOperation = 'destination-out'; bc.drawImage(alphaC, m.x, m.y); } else bc.clearRect(m.x, m.y, m.w, m.h); }
        } else {
            mv.targets = (inMask ? [] : [{ canvas: layer.canvas, snap: Brush.pristine(r, layer, false), mask: false }]).concat(layer.maskCanvas ? [{ canvas: layer.maskCanvas, snap: Brush.pristine(r, layer, true), mask: true }] : []);
            const b = inMask ? null : layerBounds(r, layer);
            if (b) mv.box = b;
        }
        const lines = { xs: [0, w / 2, w], ys: [0, h / 2, h] };
        r.layers.forEach(l => {
            if (l === layer || !l.visible || !(l.opacity > 0) || l.sticker) return;
            const b = layerBounds(r, l);
            if (b) { lines.xs.push(b.x0, (b.x0 + b.x1) / 2, b.x1); lines.ys.push(b.y0, (b.y0 + b.y1) / 2, b.y1); }
        });
        mv.lines = lines;
        return mv;
    }

    function applyMove(r, mv, dx, dy) {
        const layer = mv.layer;
        mv.dx = dx; mv.dy = dy;
        if (mv.sel0) {
            const c = (mv.inMask ? layer.maskCanvas : layer.canvas).getContext('2d');
            c.clearRect(0, 0, mv.w, mv.h);
            c.drawImage(mv.base, 0, 0);
            c.drawImage(mv.lifted, mv.box.x0 + dx, mv.box.y0 + dy);
            r.markLayerDirty(layer, { mask: mv.inMask });
            if (mv.sel0.type !== 'mask') r.selection = Object.assign({}, mv.sel0, { x: mv.sel0.x + dx, y: mv.sel0.y + dy });
        } else {
            mv.targets.forEach(t => {
                const c = t.canvas.getContext('2d');
                c.clearRect(0, 0, mv.w, mv.h);
                c.drawImage(t.snap, dx, dy);
                r.markLayerDirty(layer, { mask: t.mask });
            });
        }
        r.compositeLayers();
    }

    function finishMove(r, mv, merge) {
        if (!mv.dx && !mv.dy) return;
        if (mv.sel0 && mv.sel0.type === 'mask') r.setSelectionMask(T.shiftAlpha(mv.m.alpha, mv.w, mv.h, mv.dx, mv.dy), { mode: 'replace' });
        const last = r.history[r.historyIdx];
        if (merge && last && last.label === 'nudge' && r.historyIdx === r.history.length - 1 && Date.now() - (r.nudgeAt || 0) < 900) { r.history.pop(); r.historyIdx--; }
        if (merge) r.nudgeAt = Date.now();
        r.pushHistory(merge ? 'nudge' : (mv.copy ? 'move:copy' : 'move'));
    }

    function cancelMove(r) {
        const mv = r.moveDrag;
        if (!mv) return;
        r.moveDrag = null;
        applyMove(r, mv, 0, 0);
        if (mv.sel0) r.selection = mv.sel0;
        r.requestOverlay();
    }

    const NUDGE = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };
    toolUi.defineTool(MOVE_CFG, {
        raster: false, cursor: 'move',
        onActivate: r => { toolUi.settings(r, MOVE_CFG).snap = r.viewOpts ? r.viewOpts.snap : true; },
        onDown: (r, e, p) => {
            if (e.button) return;
            cancelMove(r);
            const s = toolUi.settings(r, MOVE_CFG);
            if ((s.autoSelect || p.ctrl) && !r.selection) { const idx = pickLayerAt(r, p.x, p.y); if (idx >= 0 && idx !== r.activeLayerIdx) selectLayer(r, idx); }
            r.moveDrag = beginMove(r, p, p.alt);
        },
        onMove: (r, e, p) => {
            const mv = r.moveDrag;
            if (!mv) return;
            let dx = Math.round(p.x - mv.start.x), dy = Math.round(p.y - mv.start.y);
            const lockX = p.shift && Math.abs(dx) < Math.abs(dy), lockY = p.shift && !lockX;
            if (lockX) dx = 0;
            if (lockY) dy = 0;
            mv.guides = null;
            if (r.viewOpts && r.viewOpts.snap && mv.box) {
                const lines = { xs: mv.lines.xs.slice(), ys: mv.lines.ys.slice() }, g = r.viewOpts.grid ? Math.max(2, r.viewOpts.gridSize | 0) : 0;
                const box = { x0: mv.box.x0 + dx, y0: mv.box.y0 + dy, x1: mv.box.x1 + dx, y1: mv.box.y1 + dy };
                if (g) [box.x0, (box.x0 + box.x1) / 2, box.x1].forEach(v => lines.xs.push(Math.round(v / g) * g)), [box.y0, (box.y0 + box.y1) / 2, box.y1].forEach(v => lines.ys.push(Math.round(v / g) * g));
                const sn = T.snapBox(box, lines, 6 / Math.max(0.05, r.zoom));
                if (!lockX) dx += Math.round(sn.dx);
                if (!lockY) dy += Math.round(sn.dy);
                mv.guides = { xs: lockX ? [] : sn.xs, ys: lockY ? [] : sn.ys };
            }
            if (dx !== mv.dx || dy !== mv.dy) applyMove(r, mv, dx, dy);
            r.setStatus(dx + ', ' + dy);
            r.requestOverlay();
        },
        onUp: (r) => {
            const mv = r.moveDrag;
            r.moveDrag = null;
            if (mv) finishMove(r, mv, false);
            r.updateStatus();
            r.requestOverlay();
        },
        onKey: (r, e) => {
            if (e.key === 'Escape' && r.moveDrag) { cancelMove(r); return true; }
            const dir = NUDGE[e.key];
            if (!dir || e.ctrlKey || e.metaKey || e.altKey || r.moveDrag || !r.imgWidth) return false;
            const step = e.shiftKey ? 10 : 1, layer = r.activeLayer();
            if (!layer) return false;
            if (layer.sticker) { r.commitMoveLayer(dir[0] * step, dir[1] * step); return true; }
            const mv = beginMove(r, { x: 0, y: 0 }, false);
            if (mv) { applyMove(r, mv, dir[0] * step, dir[1] * step); finishMove(r, mv, true); }
            return true;
        },
        onDeactivate: r => cancelMove(r),
        overlay: (ctx, r) => {
            const mv = r.moveDrag;
            if (!mv) return;
            const lw = 1 / Math.max(0.05, r.zoom);
            ctx.lineWidth = lw;
            if (mv.guides) {
                ctx.strokeStyle = '#ff2bd6';
                ctx.beginPath();
                mv.guides.xs.forEach(x => { ctx.moveTo(x, 0); ctx.lineTo(x, mv.h); });
                mv.guides.ys.forEach(y => { ctx.moveTo(0, y); ctx.lineTo(mv.w, y); });
                ctx.stroke();
            }
            if (mv.sel0 && mv.sel0.type === 'mask') {
                ctx.setLineDash([4 * lw, 4 * lw]); ctx.strokeStyle = '#ffffff';
                ctx.strokeRect(mv.box.x0 + mv.dx, mv.box.y0 + mv.dy, mv.box.x1 - mv.box.x0, mv.box.y1 - mv.box.y0);
            }
        }
    });

    // --- free transform ---

    const FT_CFG = {
        id: 'free-transform',
        schema: [
            F('x', 'int', 'pixel.ft_x', 'X', { min: -100000, max: 100000, default: 0 }), F('y', 'int', 'pixel.ft_y', 'Y', { min: -100000, max: 100000, default: 0 }),
            F('w', 'int', 'pixel.width', 'Width', { min: 1, max: 100000, default: 1 }), F('h', 'int', 'pixel.height', 'Height', { min: 1, max: 100000, default: 1 }),
            F('angle', 'range', 'pixel.brush_angle', 'Angle', { min: -180, max: 180, step: 0.1, unit: '°', default: 0 }),
            F('lock', 'bool', 'pixel.lock_ratio', 'Lock aspect ratio', { default: true })
        ],
        tail: r => '<span class="pixel-opt-buttons"><button type="button" class="pixel-btn pixel-btn-primary" data-ft-apply>' + r.esc(r.t('pixel.apply')) + '</button><button type="button" class="pixel-btn" data-ft-cancel>' + r.esc(r.t('pixel.cancel')) + '</button>' +
            '<button type="button" class="pixel-btn" data-ft-flip="h">' + r.esc(r.t('pixel.flip_h')) + '</button><button type="button" class="pixel-btn" data-ft-flip="v">' + r.esc(r.t('pixel.flip_v')) + '</button><button type="button" class="pixel-btn" data-ft-rotate>' + r.esc(r.t('pixel.ft_rotate90')) + '</button></span>',
        wireExtra: (r, bar) => {
            const on = (sel, fn) => { const el = bar.querySelector(sel); if (el) el.addEventListener('click', fn); };
            on('[data-ft-apply]', () => { applyFT(r); leaveFT(r); });
            on('[data-ft-cancel]', () => { cancelFT(r); leaveFT(r); });
            on('[data-ft-flip="h"]', () => ftEdit(r, ft => { ft.g.sx = -ft.g.sx; }));
            on('[data-ft-flip="v"]', () => ftEdit(r, ft => { ft.g.sy = -ft.g.sy; }));
            on('[data-ft-rotate]', () => ftEdit(r, ft => { ft.g.rot += Math.PI / 2; }));
        },
        onChange: (r, key, value, s) => {
            const ft = r.ft;
            if (!ft) return;
            const g = ft.g, sgx = g.sx < 0 ? -1 : 1, sgy = g.sy < 0 ? -1 : 1;
            if (key === 'x') g.cx = value;
            else if (key === 'y') g.cy = value;
            else if (key === 'angle') g.rot = value * Math.PI / 180;
            else if (key === 'w') { const k = value / (Math.abs(g.sx) * ft.w0); g.sx = sgx * value / ft.w0; if (s.lock) g.sy = sgy * Math.abs(g.sy) * k; }
            else if (key === 'h') { const k = value / (Math.abs(g.sy) * ft.h0); g.sy = sgy * value / ft.h0; if (s.lock) g.sx = sgx * Math.abs(g.sx) * k; }
            renderFT(r, ft);
            syncFT(r, key);
        }
    };

    function syncFT(r, except) {
        const ft = r.ft;
        if (!ft) return;
        const s = toolUi.settings(r, FT_CFG), handle = (r.toolUiHandles || {})[FT_CFG.id], g = ft.g;
        const next = { x: Math.round(g.cx), y: Math.round(g.cy), w: Math.max(1, Math.round(Math.abs(g.sx) * ft.w0)), h: Math.max(1, Math.round(Math.abs(g.sy) * ft.h0)), angle: Math.round((((g.rot * 180 / Math.PI) + 540) % 360 - 180) * 10) / 10 };
        Object.keys(next).forEach(k => { if (k === except) return; s[k] = next[k]; if (handle) handle.set(k, next[k]); });
    }

    function drawContent(c, ft, hq, ox, oy) {
        let src = ft.content;
        if (hq) {
            const tw = Math.ceil(Math.abs(ft.g.sx) * ft.w0), th = Math.ceil(Math.abs(ft.g.sy) * ft.h0);
            while (src.width / 2 >= tw || src.height / 2 >= th) {
                const nw = src.width / 2 >= tw ? Math.max(1, Math.ceil(src.width / 2)) : src.width, nh = src.height / 2 >= th ? Math.max(1, Math.ceil(src.height / 2)) : src.height;
                const half = Pixel.makeCanvas(nw, nh), hc = half.getContext('2d');
                hc.imageSmoothingQuality = 'high';
                hc.drawImage(src, 0, 0, nw, nh);
                src = half;
            }
        }
        const m = T.matrixOf(ft.g, ft.w0, ft.h0);
        c.save();
        c.imageSmoothingEnabled = true;
        c.imageSmoothingQuality = hq ? 'high' : 'low';
        c.setTransform(m[0], m[1], m[2], m[3], m[4] - (ox || 0), m[5] - (oy || 0));
        c.scale(ft.w0 / src.width, ft.h0 / src.height);
        c.drawImage(src, 0, 0);
        c.restore();
    }

    function renderFT(r, ft) {
        if (ft.mode !== 'outline') {
            const c = ft.pv.getContext('2d');
            c.setTransform(1, 0, 0, 1, 0, 0);
            c.clearRect(0, 0, ft.pv.width, ft.pv.height);
            if (ft.base) c.drawImage(ft.base, 0, 0);
            drawContent(c, ft, false);
            r.setLayerPreview(ft.layer, ft.pv);
        }
        r.requestOverlay();
    }

    function beginFT(r, mode) {
        const layer = r.activeLayer();
        if (!layer || !r.imgWidth) return false;
        if (mode === 'outline') { if (!r.selection || !r.getSelectionMask()) return false; }
        else {
            if (!r.requireRasterLayer(layer)) return false;
            // Resampling moves and blends alpha, which an alpha-locked layer must keep.
            if (layer.lockPixels || layer.lockAlpha) { lockedNote(r); return false; }
            if (r.editMaskMode) r.leaveMaskEdit();
            r.commitPendingPreviews();
        }
        const w = r.imgWidth, h = r.imgHeight, m = r.selection ? r.getSelectionMask() : null;
        const ft = { layer, mode, w, h, base: null, pv: null, alphaC: null, drag: null, hover: null };
        let box;
        if (m) {
            box = { x: m.x, y: m.y, w: m.w, h: m.h };
            ft.alphaC = Brush.alphaCanvas(m.alpha, w, box);
        } else {
            const b = contentBounds(layer.canvas);
            box = b ? { x: b.x0, y: b.y0, w: b.x1 - b.x0, h: b.y1 - b.y0 } : { x: 0, y: 0, w, h };
        }
        ft.box = box; ft.w0 = box.w; ft.h0 = box.h;
        if (mode === 'outline') {
            ft.content = ft.alphaC;
        } else {
            ft.content = r.makeCanvas(box.w, box.h);
            ft.content.getContext('2d').drawImage(layer.canvas, box.x, box.y, box.w, box.h, 0, 0, box.w, box.h);
            if (ft.alphaC) {
                const cc = ft.content.getContext('2d');
                cc.globalCompositeOperation = 'destination-in';
                cc.drawImage(ft.alphaC, 0, 0);
                ft.base = r.makeCanvas(w, h);
                const bc = ft.base.getContext('2d');
                bc.drawImage(layer.canvas, 0, 0);
                bc.globalCompositeOperation = 'destination-out';
                bc.drawImage(ft.alphaC, box.x, box.y);
            }
            ft.pv = r.makeCanvas(w, h);
        }
        ft.g = { cx: box.x + box.w / 2, cy: box.y + box.h / 2, sx: 1, sy: 1, rot: 0 };
        ft.g0 = Object.assign({}, ft.g);
        r.ft = ft;
        renderFT(r, ft);
        syncFT(r);
        return true;
    }

    function ftChanged(ft) {
        const a = ft.g, b = ft.g0;
        return ['cx', 'cy', 'sx', 'sy', 'rot'].some(k => Math.abs(a[k] - b[k]) > 1e-6);
    }

    function selectionFromFT(r, ft) {
        const quad = T.quad(ft.g, ft.w0, ft.h0), w = r.imgWidth, h = r.imgHeight;
        const x0 = Math.max(0, Math.floor(Math.min(...quad.map(q => q.x)))), y0 = Math.max(0, Math.floor(Math.min(...quad.map(q => q.y))));
        const x1 = Math.min(w, Math.ceil(Math.max(...quad.map(q => q.x)))), y1 = Math.min(h, Math.ceil(Math.max(...quad.map(q => q.y))));
        if (x1 <= x0 || y1 <= y0) { r.deselect(); return; }
        const cv = Pixel.acquireTempCanvas(x1 - x0, y1 - y0), c = cv.getContext('2d');
        drawContent(c, Object.assign({}, ft, { content: ft.alphaC }), true, x0, y0);
        const d = c.getImageData(0, 0, x1 - x0, y1 - y0).data, alpha = new Uint8ClampedArray(w * h);
        for (let y = 0; y < y1 - y0; y++) for (let x = 0; x < x1 - x0; x++) alpha[(y0 + y) * w + x0 + x] = d[(y * (x1 - x0) + x) * 4 + 3];
        Pixel.releaseTempCanvas(cv);
        r.setSelectionMask(alpha, { mode: 'replace' });
    }

    function cancelFT(r) {
        const ft = r.ft;
        if (!ft) return;
        r.ft = null;
        if (ft.mode !== 'outline' && ft.layer.preview === ft.pv) r.setLayerPreview(ft.layer, null);
        r.requestOverlay();
    }

    function applyFT(r) {
        const ft = r.ft;
        if (!ft) return;
        if (!ftChanged(ft)) { cancelFT(r); return; }
        r.ft = null;
        const layer = ft.layer, w = ft.w, h = ft.h;
        if (ft.mode === 'outline') {
            selectionFromFT(r, ft);
            r.pushHistory('transform:selection');
            r.requestOverlay();
            return;
        }
        if (layer.preview === ft.pv) r.setLayerPreview(layer, null);
        const out = Pixel.acquireTempCanvas(w, h), c = out.getContext('2d');
        if (ft.base) c.drawImage(ft.base, 0, 0);
        drawContent(c, ft, true);
        // The mask follows a whole-layer transform; a selection transform leaves it alone.
        if (!ft.alphaC && layer.maskCanvas) {
            const old = r.copyCanvas(layer.maskCanvas), mc = layer.maskCanvas.getContext('2d'), m = T.matrixOf(ft.g, ft.w0, ft.h0), o = ft.box;
            mc.save();
            mc.setTransform(1, 0, 0, 1, 0, 0);
            mc.clearRect(0, 0, w, h);
            mc.setTransform(m[0], m[1], m[2], m[3], m[4] - m[0] * o.x - m[2] * o.y, m[5] - m[1] * o.x - m[3] * o.y);
            mc.imageSmoothingQuality = 'high';
            mc.drawImage(old, 0, 0);
            mc.restore();
            r.markLayerDirty(layer, { mask: true });
        }
        if (ft.alphaC) selectionFromFT(r, ft);
        r.commitLayerPixels(layer, out, { label: 'transform', respectSelection: false });
        Pixel.releaseTempCanvas(out);
        r.requestOverlay();
    }

    function leaveFT(r) {
        const back = r.ftReturn;
        r.ftReturn = null;
        if (r.activeTool === 'free-transform') r.setActiveTool(back && back !== 'free-transform' ? back : 'free-transform');
    }
    // Runs an edit on the open session, or on a one-shot session that is applied at once (flip / rotate of the layer or selection).
    function ftEdit(r, fn) {
        if (r.ft) { fn(r.ft); renderFT(r, r.ft); syncFT(r); return; }
        if (!beginFT(r, 'pixels')) return;
        fn(r.ft);
        applyFT(r);
    }

    const RESIZE_CURSORS = ['nwse-resize', 'ns-resize', 'nesw-resize', 'ew-resize', 'nwse-resize', 'ns-resize', 'nesw-resize', 'ew-resize'];
    const ftHit = (r, ft, p) => T.hitTest(ft.g, ft.w0, ft.h0, p, 8 / Math.max(0.05, r.zoom));

    toolUi.defineTool(FT_CFG, {
        group: 'select', order: 60, shortcut: '', labelKey: 'pixel.free_transform', fallback: 'Free Transform', cursor: 'default',
        svg: svg('<rect x="3.5" y="3.5" width="9" height="9" stroke-dasharray="2 1.5"/><rect x="1.8" y="1.8" width="3" height="3" fill="currentColor"/><rect x="11.2" y="1.8" width="3" height="3" fill="currentColor"/><rect x="1.8" y="11.2" width="3" height="3" fill="currentColor"/><rect x="11.2" y="11.2" width="3" height="3" fill="currentColor"/>'),
        onActivate: r => {
            const mode = r.ftMode || 'pixels';
            r.ftMode = null;
            if (!beginFT(r, mode)) Promise.resolve().then(() => leaveFT(r));
        },
        onDeactivate: r => { applyFT(r); },
        onDown: (r, e, p) => {
            const ft = r.ft;
            if (!ft || e.button) return;
            const hit = ftHit(r, ft, p);
            ft.drag = { hit, p0: { x: p.x, y: p.y }, g0: Object.assign({}, ft.g) };
        },
        onMove: (r, e, p) => {
            const ft = r.ft, d = ft && ft.drag;
            if (!d) return;
            const g = ft.g, g0 = d.g0;
            if (d.hit.kind === 'move') {
                g.cx = g0.cx + p.x - d.p0.x; g.cy = g0.cy + p.y - d.p0.y;
            } else if (d.hit.kind === 'rotate') {
                let rot = g0.rot + Math.atan2(p.y - g0.cy, p.x - g0.cx) - Math.atan2(d.p0.y - g0.cy, d.p0.x - g0.cx);
                if (p.shift) rot = Math.round(rot / (Math.PI / 12)) * (Math.PI / 12);
                g.rot = rot;
            } else {
                Object.assign(g, T.scaleDrag(g0, d.hit.hx, d.hit.hy, p, ft.w0, ft.h0, p.shift, p.alt));
            }
            renderFT(r, ft);
            syncFT(r);
        },
        onUp: r => { if (r.ft) r.ft.drag = null; },
        onHover: (r, e, p) => {
            const ft = r.ft;
            if (!ft) return;
            const hit = ftHit(r, ft, p);
            r.overlayCanvas.style.cursor = hit.kind === 'move' ? 'move' : (hit.kind === 'rotate' ? 'alias' : RESIZE_CURSORS[HANDLES.findIndex(h => h[0] === hit.hx && h[1] === hit.hy)]);
        },
        onKey: (r, e) => {
            if (!r.ft) return false;
            if (e.key === 'Enter') { applyFT(r); leaveFT(r); return true; }
            if (e.key === 'Escape') { cancelFT(r); leaveFT(r); return true; }
            const dir = NUDGE[e.key];
            if (dir && !e.ctrlKey && !e.metaKey) { const k = e.shiftKey ? 10 : 1; r.ft.g.cx += dir[0] * k; r.ft.g.cy += dir[1] * k; renderFT(r, r.ft); syncFT(r); return true; }
            return false;
        },
        overlay: (ctx, r) => {
            const ft = r.ft;
            if (!ft) return;
            const q = T.quad(ft.g, ft.w0, ft.h0), lw = 1 / Math.max(0.05, r.zoom), hs = 4 * lw;
            ctx.beginPath();
            q.forEach((pt, i) => (i ? ctx.lineTo(pt.x, pt.y) : ctx.moveTo(pt.x, pt.y)));
            ctx.closePath();
            [['#000000', 3 * lw], ['#ffffff', lw]].forEach(([color, width]) => { ctx.strokeStyle = color; ctx.lineWidth = width; ctx.stroke(); });
            const mid = (a, b) => ({ x: (a.x + b.x) / 2, y: (a.y + b.y) / 2 });
            q.concat([mid(q[0], q[1]), mid(q[1], q[2]), mid(q[2], q[3]), mid(q[3], q[0])]).forEach(pt => {
                ctx.fillStyle = '#ffffff'; ctx.strokeStyle = '#000000'; ctx.lineWidth = lw;
                ctx.fillRect(pt.x - hs, pt.y - hs, hs * 2, hs * 2); ctx.strokeRect(pt.x - hs, pt.y - hs, hs * 2, hs * 2);
            });
        }
    });

    // --- crop tool ---

    const RATIOS = { '1:1': 1, '4:3': 4 / 3, '3:2': 3 / 2, '16:9': 16 / 9, '9:16': 9 / 16 };
    const CROP_CFG = {
        id: 'crop',
        schema: [
            F('ratio', 'select', 'pixel.crop_ratio', 'Ratio', { default: 'free', options: [opt('free', 'pixel.ratio_free', 'Free'), opt('original', 'pixel.ratio_original', 'Original')].concat(Object.keys(RATIOS).map(v => ({ value: v, fallback: v })), [opt('custom', 'pixel.ratio_custom', 'Custom')]) }),
            F('cw', 'int', 'pixel.crop_ratio_w', 'Ratio W', { min: 1, max: 1000, default: 5, showIf: v => v.ratio === 'custom' }),
            F('ch', 'int', 'pixel.crop_ratio_h', 'Ratio H', { min: 1, max: 1000, default: 4, showIf: v => v.ratio === 'custom' }),
            F('guides', 'select', 'pixel.crop_guides', 'Overlay', { default: 'thirds', options: [opt('thirds', 'pixel.crop_thirds', 'Rule of thirds'), opt('grid', 'pixel.crop_grid', 'Grid'), opt('none', 'pixel.opt_off', 'Off')] })
        ],
        tail: r => '<span class="pixel-opt-buttons"><button type="button" class="pixel-btn pixel-btn-primary" data-crop-apply>' + r.esc(r.t('pixel.apply_crop')) + '</button><button type="button" class="pixel-btn" data-crop-cancel>' + r.esc(r.t('pixel.cancel_crop')) + '</button></span>',
        wireExtra: (r, bar) => {
            const on = (sel, fn) => { const el = bar.querySelector(sel); if (el) el.addEventListener('click', fn); };
            on('[data-crop-apply]', () => applyCrop(r));
            on('[data-crop-cancel]', () => cancelCrop(r));
        },
        onChange: (r, key) => {
            const ratio = cropRatio(r);
            if (r.crop && ratio && (key === 'ratio' || key === 'cw' || key === 'ch')) r.crop = T.cropRatio(r.crop, ratio);
            r.requestOverlay();
        }
    };
    function cropRatio(r) {
        const s = toolUi.settings(r, CROP_CFG);
        return s.ratio === 'free' ? null : (s.ratio === 'original' ? r.imgWidth / r.imgHeight : (s.ratio === 'custom' ? s.cw / s.ch : RATIOS[s.ratio]));
    }
    function applyCrop(r) {
        const c = r.crop;
        if (!c) return;
        const x = Math.max(0, Math.round(c.x0)), y = Math.max(0, Math.round(c.y0)), w = Math.min(r.imgWidth - x, Math.round(c.x1 - c.x0)), h = Math.min(r.imgHeight - y, Math.round(c.y1 - c.y0));
        if (w < 2 || h < 2) { cancelCrop(r); return; }
        r.crop = null;
        r.transformDocument([1, 0, 0, 1, -x, -y], w, h);
        r.pushHistory('crop');
        r.zoomFit();
        if (r.activeTool === 'crop') r.setActiveTool('crop');
    }
    function cancelCrop(r) {
        r.crop = null;
        r.cropDrag = null;
        if (r.activeTool === 'crop') r.setActiveTool('crop');
        r.requestOverlay();
    }

    toolUi.defineTool(CROP_CFG, {
        group: 'view', order: 30, shortcut: 'C', labelKey: 'pixel.crop', fallback: 'Crop', cursor: 'crosshair',
        svg: svg('<path d="M4.5 1.5v10a1 1 0 001 1h9M1.5 4.5h10a1 1 0 011 1v9"/>'),
        onActivate: r => { r.crop = r.imgWidth ? { x0: 0, y0: 0, x1: r.imgWidth, y1: r.imgHeight } : null; r.cropDrag = null; },
        onDeactivate: r => { r.crop = null; r.cropDrag = null; r.requestOverlay(); },
        onDown: (r, e, p) => {
            const c = r.crop;
            if (!c || e.button) return;
            const hit = T.hitCrop(c, p, 8 / Math.max(0.05, r.zoom));
            r.cropDrag = hit.kind === 'edge' ? { kind: 'edge', hx: hit.hx, hy: hit.hy } : (hit.kind === 'move' ? { kind: 'move', p0: { x: p.x, y: p.y }, rect: Object.assign({}, c) } : { kind: 'new', x: p.x, y: p.y, rect: Object.assign({}, c) });
        },
        onMove: (r, e, p) => {
            const d = r.cropDrag;
            if (!d) return;
            const ratio = cropRatio(r), W = r.imgWidth, H = r.imgHeight;
            if (d.kind === 'edge') r.crop = T.cropDrag(r.crop, d.hx, d.hy, p, ratio, W, H);
            else if (d.kind === 'move') r.crop = T.cropMove(d.rect, p.x - d.p0.x, p.y - d.p0.y, W, H);
            else r.crop = T.cropDrag({ x0: clamp(d.x, 0, W), x1: clamp(d.x, 0, W), y0: clamp(d.y, 0, H), y1: clamp(d.y, 0, H) }, 1, 1, p, ratio, W, H);
            r.setStatus(Math.round(r.crop.x1 - r.crop.x0) + ' × ' + Math.round(r.crop.y1 - r.crop.y0));
            r.requestOverlay();
        },
        onUp: (r) => {
            const d = r.cropDrag, c = r.crop;
            r.cropDrag = null;
            if (d && d.kind === 'new' && c && (c.x1 - c.x0 < 3 || c.y1 - c.y0 < 3)) r.crop = d.rect;
            r.updateStatus();
            r.requestOverlay();
        },
        onHover: (r, e, p) => {
            if (!r.crop) return;
            const hit = T.hitCrop(r.crop, p, 8 / Math.max(0.05, r.zoom));
            r.overlayCanvas.style.cursor = hit.kind === 'move' ? 'move' : (hit.kind === 'edge' ? (hit.hx && hit.hy ? (hit.hx === hit.hy ? 'nwse-resize' : 'nesw-resize') : (hit.hx ? 'ew-resize' : 'ns-resize')) : 'crosshair');
        },
        onKey: (r, e) => {
            if (e.key === 'Enter' && r.crop) { applyCrop(r); return true; }
            if (e.key === 'Escape' && r.crop) { cancelCrop(r); return true; }
            return false;
        },
        overlay: (ctx, r) => {
            const c = r.crop;
            if (!c) return;
            const lw = 1 / Math.max(0.05, r.zoom), w = c.x1 - c.x0, h = c.y1 - c.y0, hs = 4 * lw, s = toolUi.settings(r, CROP_CFG);
            ctx.fillStyle = 'rgba(0,0,0,0.55)';
            ctx.beginPath(); ctx.rect(0, 0, r.imgWidth, r.imgHeight); ctx.rect(c.x0, c.y0, w, h); ctx.fill('evenodd');
            const parts = s.guides === 'grid' ? 4 : (s.guides === 'thirds' ? 3 : 0);
            ctx.strokeStyle = 'rgba(255,255,255,0.55)'; ctx.lineWidth = lw;
            ctx.beginPath();
            for (let i = 1; i < parts; i++) { ctx.moveTo(c.x0 + w * i / parts, c.y0); ctx.lineTo(c.x0 + w * i / parts, c.y1); ctx.moveTo(c.x0, c.y0 + h * i / parts); ctx.lineTo(c.x1, c.y0 + h * i / parts); }
            ctx.stroke();
            [['#000000', 3 * lw], ['#ffffff', lw]].forEach(([color, width]) => { ctx.strokeStyle = color; ctx.lineWidth = width; ctx.strokeRect(c.x0, c.y0, w, h); });
            ctx.fillStyle = '#ffffff'; ctx.strokeStyle = '#000000'; ctx.lineWidth = lw;
            HANDLES.forEach(([hx, hy]) => {
                const x = c.x0 + w * (hx + 1) / 2, y = c.y0 + h * (hy + 1) / 2;
                ctx.fillRect(x - hs, y - hs, hs * 2, hs * 2); ctx.strokeRect(x - hs, y - hs, hs * 2, hs * 2);
            });
        }
    });

    // --- menus, commands, runtime hooks ---

    const needDoc = r => !!r.imgWidth;
    Pixel.registerMenuItems('edit', [
        { id: 'free-transform', labelKey: 'pixel.free_transform', label: 'Free Transform', icon: 'maximize', keywords: ['transform', 'scale', 'rotate'], shortcut: 'Ctrl+Alt+T', shortcuts: ['Ctrl+T'], enabled: needDoc, action: r => r.startFreeTransform() },
        {
            id: 'transform-layer', labelKey: 'pixel.transform_layer', label: 'Transform Layer', enabled: needDoc,
            items: [
                { id: 'flip-layer-h', labelKey: 'pixel.flip_layer_h', label: 'Flip Layer Horizontal', icon: 'sort', enabled: needDoc, action: r => ftEdit(r, ft => { ft.g.sx = -ft.g.sx; }) },
                { id: 'flip-layer-v', labelKey: 'pixel.flip_layer_v', label: 'Flip Layer Vertical', icon: 'sort', enabled: needDoc, action: r => ftEdit(r, ft => { ft.g.sy = -ft.g.sy; }) },
                { id: 'rotate-layer-cw', labelKey: 'pixel.rotate_layer_cw', label: 'Rotate Layer 90° CW', icon: 'redo', enabled: needDoc, action: r => ftEdit(r, ft => { ft.g.rot += Math.PI / 2; }) },
                { id: 'rotate-layer-ccw', labelKey: 'pixel.rotate_layer_ccw', label: 'Rotate Layer 90° CCW', icon: 'undo', enabled: needDoc, action: r => ftEdit(r, ft => { ft.g.rot -= Math.PI / 2; }) }
            ]
        }
    ], { order: 60 });
    Pixel.registerMenuItems('select', [
        { id: 'transform-selection', labelKey: 'pixel.transform_selection', label: 'Transform Selection', enabled: r => !!r.selection, action: r => r.startFreeTransform('outline') }
    ], { order: 60 });

    Pixel.registerModule('transform', function install(r) {
        Object.assign(r, {
            ft: null, ftMode: null, ftReturn: null, crop: null, cropDrag: null, moveDrag: null, boundsCache: new Map(),
            // Opens the free transform tool on the active layer / selection ('outline' transforms only the selection shape).
            startFreeTransform: function startFreeTransform(mode) {
                if (!r.imgWidth || r.activeTool === 'free-transform') return;
                r.ftMode = mode || 'pixels';
                r.ftReturn = r.activeTool;
                r.setActiveTool('free-transform');
            },
            // The Transform panel and the Image menu start the crop tool; its buttons and Enter / Escape end it.
            startCrop: function startCrop() { if (r.imgWidth) { if (r.activeTool === 'crop') r.crop = { x0: 0, y0: 0, x1: r.imgWidth, y1: r.imgHeight }; else r.setActiveTool('crop'); r.requestOverlay(); } },
            applyCrop: function applyCropRuntime() { applyCrop(r); },
            cancelCrop: function cancelCropRuntime() { r.cropState = null; if (r.activeTool === 'crop') cancelCrop(r); }
        });
        r.on('active-layer-changed', () => { if (r.ft && r.ft.layer !== r.activeLayer()) { applyFT(r); leaveFT(r); } });
        r.on('document-changed', () => { cancelFT(r); r.crop = null; r.boundsCache.clear(); });
        r.on('history-changed', () => { if (r.ft) { cancelFT(r); leaveFT(r); } });
    });
})();
