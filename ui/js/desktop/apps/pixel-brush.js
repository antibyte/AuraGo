// Pixel module "brush": stamp-based brush engine (brush, pencil, eraser, airbrush) plus what the other WP4 files reuse:
// Pixel.toolUi (per-window tool settings + option bar built from param-kit schemas) and Pixel.brush (stroke path, stamps,
// stroke session with stroke buffer / selection clipping / alpha lock / mask editing, and the stroke tool factory).
(function () {
    'use strict';
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    const clamp = (v, lo, hi) => (v < lo ? lo : (v > hi ? hi : v));
    const lerp = (a, b, t) => a + (b - a) * t;
    const Brush = Pixel.brush = { registry: {} };

    // --- pure helpers ---

    // Deterministic PRNG (mulberry32), floats in [0, 1).
    Brush.makeRng = function makeRng(seed) {
        let a = (seed >>> 0) || 1;
        return function next() {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    };

    // Mouse and touch strokes count as full pressure; only a pen reports real values.
    Brush.pressureOf = function pressureOf(p) {
        return p && p.pointerType === 'pen' ? clamp(p.pressure || 0, 0.02, 1) : 1;
    };
    Brush.pressureFactors = function pressureFactors(mode, pr) {
        return {
            size: mode === 'size' || mode === 'both' ? Math.max(0.1, pr) : 1,
            alpha: mode === 'opacity' || mode === 'both' ? Math.max(0.05, pr) : 1
        };
    };

    // Smooth stroke path: lazy-leash stabiliser, quadratic curves through segment midpoints, a dab every `spacing(pressure)` px.
    // emit(x, y, pressure, angle) runs for every dab; leash may be a number or a function (document px).
    Brush.createPath = function createPath(opts) {
        const o = opts || {};
        const emit = o.emit || (() => {});
        let pos = null, a = null, c = null, carry = 0, lastPr = 1, started = false;
        const spacingAt = pr => Math.max(0.5, o.spacing ? o.spacing(pr) : 1);
        function curve(p0, ctrl, p1, pr0, pr1) {
            const steps = clamp(Math.ceil((Math.hypot(ctrl.x - p0.x, ctrl.y - p0.y) + Math.hypot(p1.x - ctrl.x, p1.y - ctrl.y)) / 2), 1, 600);
            let px = p0.x, py = p0.y;
            for (let i = 1; i <= steps; i++) {
                const t = i / steps, u = 1 - t;
                const x = u * u * p0.x + 2 * u * t * ctrl.x + t * t * p1.x, y = u * u * p0.y + 2 * u * t * ctrl.y + t * t * p1.y;
                const len = Math.hypot(x - px, y - py);
                if (len > 0) {
                    const ang = Math.atan2(y - py, x - px);
                    let used = 0, need = Math.max(0.01, spacingAt(lerp(pr0, pr1, t)) - carry);
                    while (len - used >= need) {
                        used += need;
                        const f = used / len;
                        emit(px + (x - px) * f, py + (y - py) * f, lerp(pr0, pr1, (i - 1 + f) / steps), ang);
                        carry = 0;
                        need = Math.max(0.01, spacingAt(lerp(pr0, pr1, t)));
                    }
                    carry += len - used;
                }
                px = x; py = y;
            }
        }
        function addPoint(q, pr) {
            const m = { x: (c.x + q.x) / 2, y: (c.y + q.y) / 2 };
            curve(a, c, m, lastPr, pr);
            a = m; c = q; lastPr = pr; pos = q;
        }
        return {
            begin(x, y, pr, silent) {
                pos = { x, y }; a = { x, y }; c = { x, y }; lastPr = pr; carry = 0; started = true;
                if (!silent) emit(x, y, pr, o.startAngle || 0);
            },
            move(x, y, pr) {
                if (!started) return;
                let q = { x, y };
                const leash = typeof o.leash === 'function' ? o.leash() : (o.leash || 0);
                if (leash > 0) {
                    const d = Math.hypot(x - pos.x, y - pos.y);
                    if (d <= leash) return;
                    const k = (d - leash) / d;
                    q = { x: pos.x + (x - pos.x) * k, y: pos.y + (y - pos.y) * k };
                }
                addPoint(q, pr);
            },
            // Ends the stroke at the real pointer position, so a stabilised stroke does not stop one leash short.
            finish(x, y, pr) {
                if (!started) return;
                if (x != null && (x !== pos.x || y !== pos.y)) addPoint({ x, y }, pr);
                curve(a, c, c, lastPr, lastPr);
                started = false;
            },
            position() { return pos; }
        };
    };

    // Mirrored / rotated copies of one dab: modes off, vertical, horizontal, both, radial (axes through the canvas centre).
    Brush.symmetryPoints = function symmetryPoints(mode, count, w, h, x, y, a) {
        let out = [{ x, y, a }];
        if (mode === 'vertical' || mode === 'both') out.push({ x: w - x, y, a: Math.PI - a });
        if (mode === 'horizontal' || mode === 'both') out.push({ x, y: h - y, a: -a });
        if (mode === 'both') out.push({ x: w - x, y: h - y, a: a + Math.PI });
        if (mode === 'radial') {
            const n = clamp(Math.round(count) || 2, 2, 24), cx = w / 2, cy = h / 2;
            out = [];
            for (let i = 0; i < n; i++) {
                const t = i * Math.PI * 2 / n, cos = Math.cos(t), sin = Math.sin(t);
                out.push({ x: cx + (x - cx) * cos - (y - cy) * sin, y: cy + (x - cx) * sin + (y - cy) * cos, a: a + t });
            }
        }
        return out;
    };

    // Soft brush profile: 1 inside the hardness core, smoothstep to 0 at the rim (t = distance / radius).
    Brush.falloff = function falloff(t, hardness) {
        if (t >= 1) return 0;
        const h = clamp(hardness, 0, 100) / 100;
        if (t <= h) return 1;
        const s = 1 - (t - h) / (1 - h);
        return s * s * (3 - 2 * s);
    };

    // --- stamps ---

    const newCanvas = (w, h) => Pixel.makeCanvas(w, h);
    // ImageData helpers (the constructor works without a context, which keeps the pure code testable).
    Brush.blankImage = (w, h) => new ImageData(new Uint8ClampedArray(w * h * 4), w, h);
    Brush.cloneImage = img => new ImageData(new Uint8ClampedArray(img.data), img.width, img.height);
    const stampCache = new Map();

    function makeStamp(kind, D, hardness, hex) {
        const rgb = Pixel.hexToRgb(/^#[0-9a-f]{6}$/i.test(hex) ? hex : '#000000');
        const col = al => 'rgba(' + rgb.r + ',' + rgb.g + ',' + rgb.b + ',' + al.toFixed(3) + ')';
        if (kind === 'pixel') {
            const n = Math.max(1, Math.round(D)), canvas = newCanvas(n, n), ctx = canvas.getContext('2d'), img = Brush.blankImage(n, n);
            for (let y = 0; y < n; y++) {
                for (let x = 0; x < n; x++) {
                    const dx = x + 0.5 - n / 2, dy = y + 0.5 - n / 2;
                    if (dx * dx + dy * dy <= n * n / 4 + 0.01) { const i = (y * n + x) * 4; img.data[i] = rgb.r; img.data[i + 1] = rgb.g; img.data[i + 2] = rgb.b; img.data[i + 3] = 255; }
                }
            }
            ctx.putImageData(img, 0, 0);
            return canvas;
        }
        const side = Math.ceil(D) + 2, canvas = newCanvas(side, side), ctx = canvas.getContext('2d'), c = side / 2, R = D / 2;
        ctx.fillStyle = col(1);
        if (kind === 'square') {
            ctx.fillRect(c - R, c - R, D, D);
        } else if (kind === 'star') {
            ctx.beginPath();
            for (let i = 0; i < 10; i++) {
                const rad = i % 2 ? R * 0.42 : R, t = -Math.PI / 2 + i * Math.PI / 5;
                ctx.lineTo(c + Math.cos(t) * rad, c + Math.sin(t) * rad);
            }
            ctx.closePath();
            ctx.fill();
        } else {
            const outer = R + 0.5, t0 = (hardness / 100) * Math.max(0, R - 0.5) / outer;
            const g = ctx.createRadialGradient(c, c, 0, c, c, outer);
            g.addColorStop(0, col(1));
            g.addColorStop(t0, col(1));
            for (let i = 1; i <= 8; i++) { const s = 1 - i / 8; g.addColorStop(t0 + (1 - t0) * i / 8, col(s * s * (3 - 2 * s))); }
            ctx.fillStyle = g;
            ctx.fillRect(0, 0, side, side);
        }
        return canvas;
    }

    Brush.getStamp = function getStamp(kind, D, hardness, hex) {
        const q = D <= 32 ? Math.round(D * 4) / 4 : Math.round(D);
        const key = kind + '|' + q + '|' + Math.round(hardness) + '|' + hex;
        let stamp = stampCache.get(key);
        if (stamp) { stampCache.delete(key); stampCache.set(key, stamp); return stamp; }
        stamp = makeStamp(kind, Math.max(0.5, q), hardness, hex);
        stampCache.set(key, stamp);
        if (stampCache.size > 60) stampCache.delete(stampCache.keys().next().value);
        return stamp;
    };

    function grainPattern(r, strength, scale) {
        const cache = r.brushGrain || (r.brushGrain = {}), key = strength + '|' + scale;
        if (cache[key]) return cache[key];
        const n = 64, base = newCanvas(n, n), bctx = base.getContext('2d'), img = Brush.blankImage(n, n), rng = Brush.makeRng(0x9E3779B1);
        const thr = strength / 100 * 0.85;
        for (let i = 0; i < n * n; i++) img.data[i * 4 + 3] = clamp((rng() - thr + 0.1) / 0.2, 0, 1) * 255;
        bctx.putImageData(img, 0, 0);
        let tile = base;
        if (scale > 1) {
            tile = newCanvas(n * scale, n * scale);
            const t = tile.getContext('2d');
            t.imageSmoothingEnabled = false;
            t.drawImage(base, 0, 0, n * scale, n * scale);
        }
        return (cache[key] = tile.getContext('2d').createPattern(tile, 'repeat'));
    }

    function alphaCanvas(alpha, docW, box) {
        const canvas = newCanvas(box.w, box.h), ctx = canvas.getContext('2d'), img = Brush.blankImage(box.w, box.h);
        for (let y = 0; y < box.h; y++) {
            for (let x = 0, p = (box.y + y) * docW + box.x, q = y * box.w * 4 + 3; x < box.w; x++, p++, q += 4) img.data[q] = alpha[p];
        }
        ctx.putImageData(img, 0, 0);
        return canvas;
    }
    Brush.alphaCanvas = alphaCanvas;

    // Untouched pixels of a layer (or its mask): the current history snapshot when there is one (no copy), else a copy. Read only.
    Brush.pristine = function pristine(r, layer, inMask) {
        const target = inMask ? layer.maskCanvas : layer.canvas;
        const entry = r.history[r.historyIdx];
        const item = entry && entry.layers.find(l => l.meta.id === layer.id);
        const snap = item && (inMask ? item.maskSnap : item.snap);
        const fresh = snap && snap.canvas.width === target.width && snap.canvas.height === target.height && (inMask ? snap.maskRev === layer.maskRev : snap.rev === layer.rev);
        return fresh ? snap.canvas : r.copyCanvas(target);
    };

    // --- stroke session: one stroke on the active layer (or its mask) ---
    // spec: { mode: 'paint' | 'erase' | 'effect', opacity 0..1 (cap of the stroke buffer), grain: { strength, scale }, crisp, color }
    // Paint dabs go into a transparent stroke buffer at flow; the buffer is composited onto the pristine layer at `opacity`, so
    // overlapping dabs of one stroke never exceed the opacity. Effect tools read/write pixels themselves (S.read / S.write).
    Brush.openSession = function openSession(r, spec) {
        const layer = r.activeLayer();
        if (!layer || !r.canvas || !r.canvas.width) return null;
        const inMask = !!(r.editMaskMode && layer.maskCanvas);
        const target = inMask ? layer.maskCanvas : layer.canvas;
        if (!target || !target.width) return null;
        const w = target.width, h = target.height;
        const before = Brush.pristine(r, layer, inMask);
        let selBox = null, selCanvas = null, selAlpha = null;
        if (r.selection) {
            const m = r.getSelectionMask();
            if (!m) return null;
            selBox = { x: m.x, y: m.y, w: m.w, h: m.h };
            selAlpha = m.alpha;
            if (r.selection.type !== 'rect') selCanvas = alphaCanvas(m.alpha, w, selBox);
        }
        const ctx = r.getActiveCtx();
        const effect = spec.mode === 'effect';
        let holder = null, bctx = null;
        if (!effect) {
            holder = r.brushBuffer || (r.brushBuffer = { canvas: null, stale: false });
            if (!holder.canvas || holder.canvas.width !== w || holder.canvas.height !== h) holder.canvas = newCanvas(w, h);
            bctx = holder.canvas.getContext('2d');
            if (holder.stale) bctx.clearRect(0, 0, w, h);
            holder.stale = true;
            bctx.imageSmoothingEnabled = !spec.crisp;
        }
        const lockAlpha = !!(layer.lockAlpha && !inMask);
        const S = {
            r, layer, inMask, target, ctx, before, w, h, bctx, selBox, selAlpha, lockAlpha, dirty: null, total: null,
            op: spec.mode === 'erase' ? 'destination-out' : (lockAlpha ? 'source-atop' : 'source-over'),
            opacity: spec.opacity == null ? 1 : spec.opacity,
            grain: spec.grain && spec.grain.strength > 0 ? grainPattern(r, spec.grain.strength, spec.grain.scale || 1) : null
        };
        const add = (box, x0, y0, x1, y1) => (box ? { x0: Math.min(box.x0, x0), y0: Math.min(box.y0, y0), x1: Math.max(box.x1, x1), y1: Math.max(box.y1, y1) } : { x0, y0, x1, y1 });
        S.mark = function mark(x0, y0, x1, y1) { S.dirty = add(S.dirty, x0, y0, x1, y1); S.total = add(S.total, x0, y0, x1, y1); };
        // Draws a canvas centred on (x, y) into the stroke buffer; angle / round (0..1) shape it, snap rounds to whole pixels.
        S.paintCanvas = function paintCanvas(src, x, y, alpha, angle, round, snap) {
            const sw = src.width, sh = src.height;
            bctx.globalAlpha = clamp(alpha, 0, 1);
            if (snap) bctx.drawImage(src, Math.round(x - sw / 2), Math.round(y - sh / 2));
            else if (!angle && round >= 1) bctx.drawImage(src, x - sw / 2, y - sh / 2);
            else { bctx.save(); bctx.translate(x, y); bctx.rotate(angle); bctx.scale(1, round); bctx.drawImage(src, -sw / 2, -sh / 2); bctx.restore(); }
            bctx.globalAlpha = 1;
            const rad = Math.hypot(sw, sh) / 2 + 1;
            S.mark(x - rad, y - rad, x + rad, y + rad);
        };
        // Effect tools: rect around a dab, ImageData in, ImageData out. write() honours selection (feathered) and alpha lock.
        S.rect = function rect(x, y, size) {
            const R = size / 2 + 1, x0 = Math.max(0, Math.floor(x - R)), y0 = Math.max(0, Math.floor(y - R));
            const x1 = Math.min(w, Math.ceil(x + R)), y1 = Math.min(h, Math.ceil(y + R));
            return x1 > x0 && y1 > y0 ? { x: x0, y: y0, w: x1 - x0, h: y1 - y0 } : null;
        };
        S.read = rect => ctx.getImageData(rect.x, rect.y, rect.w, rect.h);
        S.write = function write(rect, old, next) {
            const n = next.data, o = old.data;
            if (selAlpha) {
                for (let y = 0; y < rect.h; y++) {
                    for (let x = 0, p = (rect.y + y) * w + rect.x, i = y * rect.w * 4; x < rect.w; x++, p++, i += 4) {
                        const s = selAlpha[p];
                        if (s < 255) for (let c = 0; c < 4; c++) n[i + c] = o[i + c] + (n[i + c] - o[i + c]) * s / 255;
                    }
                }
            }
            if (lockAlpha) for (let i = 3; i < n.length; i += 4) n[i] = o[i];
            ctx.putImageData(next, rect.x, rect.y);
            S.mark(rect.x, rect.y, rect.x + rect.w, rect.y + rect.h);
        };
        S.flush = function flush() {
            const d = S.dirty;
            if (!d) return false;
            S.dirty = null;
            if (!effect) {
                let x0 = Math.max(0, Math.floor(d.x0)), y0 = Math.max(0, Math.floor(d.y0)), x1 = Math.min(w, Math.ceil(d.x1)), y1 = Math.min(h, Math.ceil(d.y1));
                if (selBox) { x0 = Math.max(x0, selBox.x); y0 = Math.max(y0, selBox.y); x1 = Math.min(x1, selBox.x + selBox.w); y1 = Math.min(y1, selBox.y + selBox.h); }
                const rw = x1 - x0, rh = y1 - y0;
                if (rw > 0 && rh > 0) {
                    ctx.save();
                    ctx.globalCompositeOperation = 'source-over';
                    ctx.globalAlpha = 1;
                    ctx.clearRect(x0, y0, rw, rh);
                    ctx.drawImage(before, x0, y0, rw, rh, x0, y0, rw, rh);
                    let src = holder.canvas, sx = x0, sy = y0, tmp = null;
                    if (S.grain || selCanvas) {
                        tmp = Pixel.acquireTempCanvas(rw, rh);
                        const t = tmp.getContext('2d');
                        t.drawImage(holder.canvas, x0, y0, rw, rh, 0, 0, rw, rh);
                        t.globalCompositeOperation = 'destination-in';
                        if (S.grain) { t.save(); t.translate(-x0, -y0); t.fillStyle = S.grain; t.fillRect(x0, y0, rw, rh); t.restore(); }
                        if (selCanvas) t.drawImage(selCanvas, selBox.x - x0, selBox.y - y0);
                        src = tmp; sx = 0; sy = 0;
                    }
                    ctx.globalAlpha = S.opacity;
                    ctx.globalCompositeOperation = S.op;
                    ctx.drawImage(src, sx, sy, rw, rh, x0, y0, rw, rh);
                    ctx.restore();
                    if (tmp) Pixel.releaseTempCanvas(tmp);
                }
            }
            r.markLayerDirty(layer, { mask: inMask });
            r.compositeLayers();
            return true;
        };
        function clean() {
            const t = S.total;
            if (holder) {
                if (t) bctx.clearRect(Math.floor(t.x0), Math.floor(t.y0), Math.ceil(t.x1 - t.x0) + 2, Math.ceil(t.y1 - t.y0) + 2);
                holder.stale = false;
            }
        }
        S.finish = function finish(label) {
            S.flush();
            const touched = !!S.total;
            clean();
            if (touched) r.pushHistory(label);
            return touched;
        };
        S.cancel = function cancel() {
            const t = S.total;
            if (t) {
                const x0 = Math.max(0, Math.floor(t.x0)), y0 = Math.max(0, Math.floor(t.y0)), x1 = Math.min(w, Math.ceil(t.x1)), y1 = Math.min(h, Math.ceil(t.y1));
                if (x1 > x0 && y1 > y0) { ctx.clearRect(x0, y0, x1 - x0, y1 - y0); ctx.drawImage(before, x0, y0, x1 - x0, y1 - y0, x0, y0, x1 - x0, y1 - y0); }
                r.markLayerDirty(layer, { mask: inMask });
                r.compositeLayers();
            }
            clean();
        };
        return S;
    };

    // --- tool settings store and option bar (shared by every WP4 tool) ---

    const F = (key, type, labelKey, fallback, extra) => Object.assign({ key, type, labelKey, fallback }, extra);
    const opt = (value, labelKey, fallback) => ({ value, labelKey, fallback });
    const toolUi = Pixel.toolUi = {
        F, opt, configs: {},
        settings(r, cfg) {
            const store = r.toolSettings || (r.toolSettings = {});
            if (!store[cfg.id]) store[cfg.id] = Object.assign(r.defaultParams(cfg.schema), cfg.defaults || {});
            return store[cfg.id];
        },
        options(r, cfg) {
            const s = toolUi.settings(r, cfg);
            const keys = cfg.bar || cfg.schema.map(f => f.key);
            const schema = cfg.schema.filter(f => keys.includes(f.key));
            return '<div class="pixel-opt-wrap" data-tool-opts="' + r.esc(cfg.id) + '">' + (cfg.head ? cfg.head(r, s) : '') +
                r.buildParamControls(schema, s, { idPrefix: 'pixel-' + r.windowId + '-o-' + cfg.id }) + (cfg.tail ? cfg.tail(r, s) : '') + '</div>';
        },
        wire(r, bar, cfg) {
            const s = toolUi.settings(r, cfg);
            const keys = cfg.bar || cfg.schema.map(f => f.key);
            const schema = cfg.schema.filter(f => keys.includes(f.key));
            const root = bar.querySelector('[data-param-root]');
            const handles = r.toolUiHandles || (r.toolUiHandles = {});
            handles[cfg.id] = root ? r.wireParamControls(root, schema, s, (key, value) => toolUi.changed(r, cfg, key, value)) : null;
            if (cfg.wireExtra) cfg.wireExtra(r, bar, s);
            return handles[cfg.id];
        },
        // A setting changed (UI, shortcut or preset): keep the runtime brush fields, the bar and the Brush panel in step.
        changed(r, cfg, key, value) {
            const s = toolUi.settings(r, cfg);
            if (cfg.onChange) cfg.onChange(r, key, value, s);
            const handle = (r.toolUiHandles || {})[cfg.id];
            if (handle && handle.refresh) handle.refresh();
            if (r.refreshBrushPanel) r.refreshBrushPanel();
        },
        set(r, cfg, key, value) {
            toolUi.settings(r, cfg)[key] = value;
            toolUi.changed(r, cfg, key, value);
        },
        // Registers a tool whose option bar is built from cfg ({ id, schema, defaults, bar, head, tail, wireExtra, onChange }).
        defineTool(cfg, def) {
            toolUi.configs[cfg.id] = cfg;
            return Pixel.registerTool(Object.assign({}, def, { id: cfg.id, options: r => toolUi.options(r, cfg), wireOptions: (r, bar) => toolUi.wire(r, bar, cfg) }));
        }
    };

    function syncRuntime(r, s) {
        if (s.size != null) r.brushSize = s.size;
        if (s.opacity != null) r.brushOpacity = s.opacity;
        if (s.hardness != null) r.brushHardness = s.hardness;
    }

    // --- brush parameters ---

    const pcts = { min: 0, max: 100, step: 1, unit: '%' };
    const FIELDS = {
        size: F('size', 'range', 'pixel.brush_size', 'Size', { min: 1, max: 1000, step: 1, unit: 'px', default: 12 }),
        opacity: F('opacity', 'range', 'pixel.brush_opacity', 'Opacity', { min: 1, max: 100, step: 1, unit: '%', default: 100 }),
        flow: F('flow', 'range', 'pixel.brush_flow', 'Flow', { min: 1, max: 100, step: 1, unit: '%', default: 100 }),
        hardness: F('hardness', 'range', 'pixel.brush_hardness', 'Hardness', Object.assign({ default: 100, showIf: v => v.shape === 'round' || v.shape == null }, pcts)),
        spacing: F('spacing', 'range', 'pixel.brush_spacing', 'Spacing', { min: 1, max: 200, step: 1, unit: '%', default: 10 }),
        smoothing: F('smoothing', 'range', 'pixel.brush_smoothing', 'Smoothing', Object.assign({ default: 0 }, pcts)),
        pressure: F('pressure', 'select', 'pixel.brush_pressure', 'Pen pressure', { default: 'size', options: [opt('off', 'pixel.opt_off', 'Off'), opt('size', 'pixel.brush_pressure_size', 'Size'), opt('opacity', 'pixel.brush_pressure_opacity', 'Opacity'), opt('both', 'pixel.brush_pressure_both', 'Both')] }),
        symmetry: F('symmetry', 'select', 'pixel.brush_symmetry', 'Symmetry', { default: 'off', options: [opt('off', 'pixel.opt_off', 'Off'), opt('vertical', 'pixel.sym_vertical', 'Vertical'), opt('horizontal', 'pixel.sym_horizontal', 'Horizontal'), opt('both', 'pixel.sym_both', 'Both'), opt('radial', 'pixel.sym_radial', 'Radial')] }),
        symmetryCount: F('symmetryCount', 'int', 'pixel.brush_symmetry_count', 'Radial segments', { min: 2, max: 24, default: 6, showIf: v => v.symmetry === 'radial' }),
        shape: F('shape', 'select', 'pixel.brush_shape', 'Shape', { default: 'round', options: [opt('round', 'pixel.shape_round', 'Round'), opt('square', 'pixel.shape_square', 'Square'), opt('star', 'pixel.shape_star', 'Star'), opt('spray', 'pixel.shape_spray', 'Spray')] }),
        angle: F('angle', 'angle', 'pixel.brush_angle', 'Angle', { default: 0 }),
        roundness: F('roundness', 'range', 'pixel.brush_roundness', 'Roundness', { min: 10, max: 100, step: 1, unit: '%', default: 100 }),
        angleFollow: F('angleFollow', 'bool', 'pixel.brush_follow', 'Follow stroke direction', { default: false }),
        scatter: F('scatter', 'range', 'pixel.brush_scatter', 'Scatter', { min: 0, max: 300, step: 1, unit: '%', default: 0 }),
        count: F('count', 'int', 'pixel.brush_count', 'Dabs per step', { min: 1, max: 8, default: 1, showIf: v => v.shape !== 'spray' }),
        density: F('density', 'range', 'pixel.spray_density', 'Spray Density', { min: 5, max: 100, step: 1, default: 25, showIf: v => v.shape === 'spray' }),
        sizeJitter: F('sizeJitter', 'range', 'pixel.brush_size_jitter', 'Size jitter', Object.assign({ default: 0 }, pcts)),
        angleJitter: F('angleJitter', 'range', 'pixel.brush_angle_jitter', 'Angle jitter', Object.assign({ default: 0 }, pcts)),
        opacityJitter: F('opacityJitter', 'range', 'pixel.brush_opacity_jitter', 'Opacity jitter', Object.assign({ default: 0 }, pcts)),
        grain: F('grain', 'range', 'pixel.brush_grain', 'Grain', Object.assign({ default: 0 }, pcts)),
        grainScale: F('grainScale', 'range', 'pixel.brush_grain_scale', 'Grain size', { min: 1, max: 6, step: 1, default: 1, showIf: v => v.grain > 0 }),
        eraserMode: F('eraserMode', 'segmented', 'pixel.eraser_mode', 'Mode', { default: 'brush', options: [opt('brush', 'pixel.eraser_brush', 'Brush'), opt('block', 'pixel.eraser_block', 'Block')] }),
        buildup: F('buildup', 'bool', 'pixel.brush_buildup', 'Build-up while held', { default: true })
    };
    const DYNAMICS = ['spacing', 'smoothing', 'pressure', 'symmetry', 'symmetryCount', 'shape', 'angle', 'roundness', 'angleFollow', 'scatter', 'count', 'density', 'sizeJitter', 'angleJitter', 'opacityJitter', 'grain', 'grainScale'];
    // Schema from field keys; per-tool default overrides keep the shared descriptors untouched.
    Brush.schema = function schema(keys, defaults) {
        return keys.map(k => Object.assign({}, FIELDS[k], defaults && defaults[k] !== undefined ? { default: defaults[k] } : {}));
    };
    Brush.FIELDS = FIELDS;

    const NEUTRAL = { shape: 'round', hardness: 100, flow: 100, spacing: 10, smoothing: 0, roundness: 100, angle: 0, angleFollow: false, scatter: 0, count: 1, density: 25, sizeJitter: 0, angleJitter: 0, opacityJitter: 0, grain: 0, grainScale: 1, pressure: 'size' };
    const PRESETS = [
        { id: 'round_hard', fallback: 'Round hard', values: {} },
        { id: 'round_soft', fallback: 'Round soft', values: { hardness: 0 } },
        { id: 'airbrush', fallback: 'Airbrush', values: { hardness: 0, flow: 20, spacing: 5, size: 40 } },
        { id: 'pencil', fallback: 'Pencil', values: { size: 3, spacing: 5, smoothing: 10, hardness: 90, grain: 25 } },
        { id: 'ink', fallback: 'Ink', values: { size: 8, spacing: 4, smoothing: 45 } },
        { id: 'marker', fallback: 'Marker', values: { size: 24, roundness: 45, angle: 45, hardness: 95, spacing: 6, smoothing: 20, pressure: 'off' } },
        { id: 'chalk', fallback: 'Chalk', values: { size: 30, hardness: 85, grain: 80, spacing: 8, flow: 75, scatter: 6, sizeJitter: 10 } },
        { id: 'calligraphy', fallback: 'Calligraphy', values: { size: 20, roundness: 18, angle: 40, spacing: 3, smoothing: 35 } },
        { id: 'spray', fallback: 'Spray', values: { shape: 'spray', size: 50, density: 40, spacing: 12 } },
        { id: 'star', fallback: 'Star scatter', values: { shape: 'star', size: 28, spacing: 70, scatter: 70, sizeJitter: 45, angleJitter: 100, pressure: 'off' } }
    ];
    PRESETS.forEach(p => { p.labelKey = 'pixel.preset_' + p.id; });
    Brush.PRESETS = PRESETS;
    Brush.applyPreset = function applyPreset(r, cfg, id) {
        const preset = PRESETS.find(p => p.id === id);
        if (!preset) return false;
        const s = toolUi.settings(r, cfg);
        const keys = cfg.schema.map(f => f.key);
        const next = Object.assign({}, NEUTRAL, preset.values);
        Object.keys(next).forEach(k => { if (keys.includes(k)) s[k] = next[k]; });
        toolUi.changed(r, cfg, 'preset', id);
        return true;
    };

    // --- stroke tools ---

    const stepSize = (size, dir) => clamp(size + dir * (size < 10 ? 1 : size < 50 ? 2 : size < 200 ? 5 : size < 500 ? 10 : 25), 1, 1000);

    function emitDab(r, st, x, y, pr, ang, scale) {
        const s = st.s, S = st.S, pf = Brush.pressureFactors(s.pressure, pr), rng = st.rng;
        const base = Math.max(0.5, s.size * pf.size);
        const amount = (s.strength != null ? s.strength : (s.flow != null ? s.flow : 100)) / 100;
        const pts = Brush.symmetryPoints(s.symmetry || 'off', s.symmetryCount, S.w, S.h, x, y, ang);
        const n = s.shape === 'spray' ? 1 : Math.max(1, s.count | 0);
        pts.forEach(pt => {
            for (let k = 0; k < n; k++) {
                let px = pt.x, py = pt.y;
                if (s.scatter > 0) { const rad = s.scatter / 100 * base * Math.sqrt(rng()), t = rng() * Math.PI * 2; px += Math.cos(t) * rad; py += Math.sin(t) * rad; }
                const size = Math.max(0.5, base * (1 - rng() * (s.sizeJitter || 0) / 100 * 0.9));
                const rot = (s.angle || 0) * Math.PI / 180 + (s.angleFollow ? pt.a : 0) + (rng() * 2 - 1) * (s.angleJitter || 0) / 100 * Math.PI;
                const alpha = amount * pf.alpha * (1 - rng() * (s.opacityJitter || 0) / 100) * (scale || 1);
                st.cfg.stamp(st, px, py, size, alpha, rot, pr);
            }
        });
    }

    function paintDab(st, x, y, size, alpha, rot) {
        const s = st.s, S = st.S, cfg = st.cfg, block = cfg.id === 'eraser' && s.eraserMode === 'block';
        const kind = cfg.pixel ? 'pixel' : (block ? 'square' : (s.shape || 'round'));
        if (kind === 'spray') {
            const b = S.bctx, rad = size / 2, count = Math.max(1, Math.round((s.density || 25) * (0.5 + rad / 40)));
            b.fillStyle = st.color;
            b.globalAlpha = clamp(alpha, 0, 1);
            for (let i = 0; i < count; i++) { const t = st.rng() * Math.PI * 2, d = Math.sqrt(st.rng()) * rad; b.fillRect(x + Math.cos(t) * d - 0.6, y + Math.sin(t) * d - 0.6, 1.4, 1.4); }
            b.globalAlpha = 1;
            S.mark(x - rad - 2, y - rad - 2, x + rad + 2, y + rad + 2);
            return;
        }
        S.paintCanvas(Brush.getStamp(kind, size, s.hardness == null ? 100 : s.hardness, st.color), x, y, alpha, block || cfg.pixel ? 0 : rot, block || cfg.pixel ? 1 : (s.roundness || 100) / 100, block || cfg.pixel);
    }

    function endStroke(r, commit) {
        const st = r.strokeState;
        if (!st) return;
        r.strokeState = null;
        if (st.timer) clearInterval(st.timer);
        if (commit) {
            st.path.finish(st.pos.x, st.pos.y, st.pr);
            st.S.finish(st.cfg.history || ('draw:' + st.cfg.id));
            if (st.cfg.mode === 'paint') r.addRecentColor(r.primaryColor);
            r.strokeLast = { layerId: st.S.layer.id, x: st.pos.x, y: st.pos.y, tool: st.cfg.id };
            if (st.cfg.done) st.cfg.done(r, st);
        } else {
            st.S.cancel();
            if (st.cfg.abort) st.cfg.abort(r, st);
        }
        r.requestOverlay();
    }
    Brush.endStroke = endStroke;

    function strokeDown(r, cfg, e, p) {
        if (e.button) return;
        endStroke(r, false);
        const s = toolUi.settings(r, cfg);
        if (p.alt && (cfg.altClick || cfg.pickColor)) {
            if (cfg.altClick) { cfg.altClick(r, p); return; }
            const px = r.sampleComposite(Math.floor(p.x), Math.floor(p.y));
            if (px && px[3]) { const hex = r.rgbToHex(px[0], px[1], px[2]); r.setPrimaryColor(hex); r.addRecentColor(hex); }
            return;
        }
        const layer = r.activeLayer();
        const inMask = !!(layer && r.editMaskMode && layer.maskCanvas);
        const S = Brush.openSession(r, {
            mode: cfg.mode, opacity: (s.opacity == null ? 100 : s.opacity) / 100, crisp: !!cfg.pixel,
            grain: s.grain > 0 ? { strength: s.grain, scale: s.grainScale } : null
        });
        if (!S) return;
        const st = r.strokeState = { r, cfg, s, S, rng: Brush.makeRng((Date.now() ^ Math.floor(Math.random() * 0x7FFFFFFF)) >>> 0), pos: p, pr: Brush.pressureOf(p), color: cfg.mode === 'erase' ? '#000000' : (inMask ? '#ffffff' : r.primaryColor) };
        st.path = Brush.createPath({
            leash: () => (s.smoothing || 0) * 0.5 / Math.max(0.1, r.zoom),
            spacing: pr => (cfg.pixel ? 1 : Math.max(0.5, s.size * Brush.pressureFactors(s.pressure, pr).size * (s.spacing || 10) / 100)),
            emit: (x, y, pr, ang) => emitDab(r, st, x, y, pr, ang)
        });
        if (cfg.begin && cfg.begin(r, st, p) === false) { endStroke(r, false); return; }
        const last = r.strokeLast;
        if (p.shift && last && last.tool === cfg.id && last.layerId === layer.id) {
            st.path.begin(last.x, last.y, st.pr, true);
            st.path.move(p.x, p.y, st.pr);
        } else {
            st.path.begin(p.x, p.y, st.pr);
        }
        if (cfg.timer && s.buildup !== false) {
            st.timer = setInterval(() => {
                if (r.state.disposed || r.strokeState !== st) { clearInterval(st.timer); return; }
                const pos = st.path.position();
                if (pos) { emitDab(r, st, pos.x, pos.y, st.pr, 0, 0.3); S.flush(); }
            }, 40);
        }
        S.flush();
        r.cursorPos = { x: p.x, y: p.y };
        r.requestOverlay();
    }

    function strokeMove(r, e, p) {
        const st = r.strokeState;
        if (!st) return;
        const list = typeof e.getCoalescedEvents === 'function' ? e.getCoalescedEvents() : null;
        (list && list.length ? list : [e]).forEach(ev => {
            const q = ev === e ? p : r.pointerInfo(ev);
            st.pr = Brush.pressureOf(q);
            st.pos = q;
            st.path.move(q.x, q.y, st.pr);
        });
        st.S.flush();
        r.cursorPos = { x: p.x, y: p.y };
        r.requestOverlay();
    }

    function paintCursor(ctx, r, cfg) {
        const pos = r.cursorPos;
        if (cfg.overlayExtra) cfg.overlayExtra(ctx, r, pos);
        if (!pos) return;
        const s = toolUi.settings(r, cfg), lw = 1 / Math.max(0.05, r.zoom), R = Math.max(0.5, s.size / 2);
        const sym = s.symmetry;
        if (sym && sym !== 'off') {
            ctx.save();
            ctx.strokeStyle = 'rgba(255,60,200,0.8)';
            ctx.lineWidth = lw;
            ctx.setLineDash([6 * lw, 4 * lw]);
            ctx.beginPath();
            if (sym === 'vertical' || sym === 'both') { ctx.moveTo(r.imgWidth / 2, 0); ctx.lineTo(r.imgWidth / 2, r.imgHeight); }
            if (sym === 'horizontal' || sym === 'both') { ctx.moveTo(0, r.imgHeight / 2); ctx.lineTo(r.imgWidth, r.imgHeight / 2); }
            if (sym === 'radial') { const n = clamp(s.symmetryCount | 0, 2, 24); for (let i = 0; i < n; i++) { const t = i * Math.PI * 2 / n; ctx.moveTo(r.imgWidth / 2, r.imgHeight / 2); ctx.lineTo(r.imgWidth / 2 + Math.cos(t) * r.imgWidth, r.imgHeight / 2 + Math.sin(t) * r.imgWidth); } }
            ctx.stroke();
            ctx.restore();
        }
        const block = cfg.id === 'eraser' && s.eraserMode === 'block';
        const ring = () => {
            ctx.beginPath();
            if (block || cfg.pixel) ctx.rect(-R, -R, R * 2, R * 2); else ctx.arc(0, 0, R, 0, Math.PI * 2);
        };
        ctx.save();
        ctx.translate(pos.x, pos.y);
        if (!block && !cfg.pixel) { ctx.rotate((s.angle || 0) * Math.PI / 180); ctx.scale(1, (s.roundness || 100) / 100); }
        [['rgba(0,0,0,0.75)', 2 * lw], ['rgba(255,255,255,0.95)', lw]].forEach(([color, width]) => { ring(); ctx.strokeStyle = color; ctx.lineWidth = width; ctx.stroke(); });
        if (s.hardness != null && s.hardness > 0 && s.hardness < 100 && !block && !cfg.pixel) {
            ctx.setLineDash([3 * lw, 3 * lw]);
            ctx.beginPath(); ctx.arc(0, 0, R * s.hardness / 100, 0, Math.PI * 2);
            ctx.strokeStyle = 'rgba(255,255,255,0.7)'; ctx.lineWidth = lw; ctx.stroke();
        }
        ctx.restore();
        if (R * r.zoom < 4) {
            ctx.save();
            ctx.strokeStyle = 'rgba(255,255,255,0.95)'; ctx.lineWidth = lw;
            ctx.beginPath(); ctx.moveTo(pos.x - 6 * lw, pos.y); ctx.lineTo(pos.x + 6 * lw, pos.y); ctx.moveTo(pos.x, pos.y - 6 * lw); ctx.lineTo(pos.x, pos.y + 6 * lw); ctx.stroke();
            ctx.restore();
        }
    }

    function strokeKey(r, cfg, e) {
        if (e.key === 'Escape' && r.strokeState) { endStroke(r, false); return true; }
        if (e.ctrlKey || e.metaKey || e.altKey) return false;
        const s = toolUi.settings(r, cfg);
        const dir = e.code === 'BracketLeft' || e.key === '[' || e.key === '{' ? -1 : (e.code === 'BracketRight' || e.key === ']' || e.key === '}' ? 1 : 0);
        if (dir) {
            if (e.shiftKey && s.hardness != null) toolUi.set(r, cfg, 'hardness', clamp(s.hardness + dir * 10, 0, 100));
            else toolUi.set(r, cfg, 'size', stepSize(s.size, dir));
            return true;
        }
        if (!e.shiftKey && /^[0-9]$/.test(e.key) && s.opacity != null) {
            toolUi.set(r, cfg, 'opacity', e.key === '0' ? 100 : Number(e.key) * 10);
            return true;
        }
        return false;
    }

    // Registers a stroke tool. cfg: { id, mode: 'paint' | 'erase' | 'effect', schema, defaults, bar, meta (registry fields),
    // stamp(st, x, y, size, alpha, angle, pressure), begin(r, st, p), done(r, st), pixel, timer, pickColor, presets, history }.
    Brush.defineStrokeTool = function defineStrokeTool(cfg) {
        Brush.registry[cfg.id] = cfg;
        toolUi.configs[cfg.id] = cfg;
        cfg.stamp = cfg.stamp || paintDab;
        const settings = r => toolUi.settings(r, cfg);
        cfg.onChange = (r, key, value, s) => { if (key === 'size' || key === 'opacity' || key === 'hardness' || key === 'preset') syncRuntime(r, s); };
        if (cfg.presets) {
            cfg.head = (r) => '<label class="pixel-opt-field"><span class="pixel-opt-label">' + r.esc(r.t('pixel.brush_preset', 'Preset')) + '</span><select class="pixel-select pixel-opt-select" data-brush-preset><option value="">' + r.esc(r.t('pixel.brush_preset_pick', 'Choose...')) + '</option>' +
                PRESETS.map(p => '<option value="' + p.id + '">' + r.esc(r.t(p.labelKey, p.fallback)) + '</option>').join('') + '</select></label>';
            cfg.tail = (r) => '<button type="button" class="pixel-btn pixel-opt-more" data-brush-panel-open>' + r.esc(r.t('pixel.brush_settings', 'Brush settings')) + '</button>';
            cfg.wireExtra = (r, bar) => {
                const sel = bar.querySelector('[data-brush-preset]');
                if (sel) sel.addEventListener('change', () => { if (Brush.applyPreset(r, cfg, sel.value)) sel.value = ''; });
                const more = bar.querySelector('[data-brush-panel-open]');
                if (more) more.addEventListener('click', () => r.showPanel('brush'));
            };
        }
        Pixel.registerTool(Object.assign({}, cfg.meta, {
            id: cfg.id, raster: true, cursor: 'none',
            options: r => toolUi.options(r, cfg),
            wireOptions: (r, bar) => toolUi.wire(r, bar, cfg),
            onActivate: r => { syncRuntime(r, settings(r)); r.cursorPos = null; if (r.refreshBrushPanel) r.refreshBrushPanel(); },
            onDeactivate: r => endStroke(r, true),
            onDown: (r, e, p) => strokeDown(r, cfg, e, p),
            onMove: (r, e, p) => strokeMove(r, e, p),
            onUp: (r, e, p) => { if (r.strokeState) { if (e.type === 'pointerup') r.strokeState.pos = p; endStroke(r, true); } },
            onHover: (r, e, p) => { r.cursorPos = { x: p.x, y: p.y }; r.requestOverlay(); },
            onKey: (r, e) => strokeKey(r, cfg, e),
            overlay: (ctx, r) => paintCursor(ctx, r, cfg)
        }));
    };

    const strokeBase = ['size', 'opacity', 'flow', 'hardness'].concat(DYNAMICS);
    Brush.defineStrokeTool({ id: 'brush', mode: 'paint', pickColor: true, presets: true,
        schema: Brush.schema(strokeBase), defaults: { size: 12 }, bar: ['size', 'opacity', 'flow', 'hardness', 'smoothing', 'symmetry'] });
    Brush.defineStrokeTool({ id: 'airbrush', mode: 'paint', pickColor: true, presets: true, timer: true,
        schema: Brush.schema(strokeBase.concat(['buildup'])), defaults: { size: 40, flow: 25, hardness: 0, spacing: 6, pressure: 'opacity' }, bar: ['size', 'opacity', 'flow', 'hardness', 'smoothing', 'symmetry'] });
    Brush.defineStrokeTool({ id: 'eraser', mode: 'erase',
        schema: Brush.schema(['eraserMode', 'size', 'opacity', 'flow', 'hardness', 'spacing', 'smoothing', 'pressure']), defaults: { size: 20 }, bar: ['eraserMode', 'size', 'opacity', 'flow', 'hardness', 'smoothing'] });
    Brush.defineStrokeTool({ id: 'pencil', mode: 'paint', pixel: true, pickColor: true,
        schema: Brush.schema(['size', 'opacity', 'smoothing', 'symmetry', 'symmetryCount'], { size: 1 }), defaults: { size: 1, pressure: 'off' }, bar: ['size', 'opacity', 'smoothing', 'symmetry'] });

    // --- Brush panel: presets plus every setting of the active stroke tool ---

    function renderBrushPanel(r) {
        const el = r.brushPanelEl;
        if (!el) return;
        const cfg = Brush.registry[r.activeTool];
        if (!cfg || (cfg.mode === 'effect' && !cfg.panel)) {
            el.innerHTML = '<p class="pixel-draw-hint">' + r.esc(r.t('pixel.brush_panel_hint', 'Pick a brush tool to see its settings.')) + '</p>';
            return;
        }
        const s = toolUi.settings(r, cfg);
        const presets = cfg.presets ? '<div class="pixel-brush-presets">' + PRESETS.map(p => '<button type="button" class="pixel-chip" data-brush-preset-id="' + p.id + '">' + r.esc(r.t(p.labelKey, p.fallback)) + '</button>').join('') + '</div>' : '';
        el.innerHTML = presets + r.buildParamControls(cfg.schema, s, { idPrefix: 'pixel-' + r.windowId + '-p-' + cfg.id });
        const root = el.querySelector('[data-param-root]');
        if (root) r.wireParamControls(root, cfg.schema, s, (key, value) => toolUi.changed(r, cfg, key, value));
        el.querySelectorAll('[data-brush-preset-id]').forEach(b => b.addEventListener('click', () => Brush.applyPreset(r, cfg, b.dataset.brushPresetId)));
    }

    Pixel.registerPanel({ id: 'brush', labelKey: 'pixel.panel_brush', fallback: 'Brush', order: 35, dock: 'inspector',
        build: () => '<div class="pixel-brush-panel" data-brush-panel></div>',
        mount: (r, el) => { r.brushPanelEl = el.querySelector('[data-brush-panel]'); renderBrushPanel(r); },
        onShow: r => renderBrushPanel(r)
    });

    Pixel.registerModule('brush', function install(r) {
        Object.assign(r, {
            strokeState: null, strokeLast: null, toolSettings: {}, toolUiHandles: {}, brushBuffer: null, brushPanelEl: null,
            refreshBrushPanel: function refreshBrushPanel() { renderBrushPanel(r); },
            // Direct access for scripts and other modules: r.brushSettingsFor('brush').size = 30
            brushSettingsFor: function brushSettingsFor(id) { const cfg = Brush.registry[id]; return cfg ? toolUi.settings(r, cfg) : null; }
        });
        r.on('tool-changed', () => { if (r.strokeState) endStroke(r, true); renderBrushPanel(r); });
        r.on('active-layer-changed', () => { if (r.strokeState) endStroke(r, false); });
        r.on('document-changed', () => { r.brushBuffer = null; });
    });
})();
