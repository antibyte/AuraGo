// Pixel module "tools-pro": shapes (rectangle, ellipse, rounded rectangle, polygon, star, line, arrow), gradient, paint bucket,
// retouch tools on the stroke engine of pixel-brush.js (clone stamp, healing, spot heal, smudge, blur / sharpen, dodge / burn /
// sponge, colour replace, red-eye) and the view tools (hand, zoom, pixel grid, grid, snap). Pure helpers: Pixel.toolsPro.
(function () {
    'use strict';
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};
    const Brush = Pixel.brush, toolUi = Pixel.toolUi;
    const Pro = Pixel.toolsPro = {};
    const clamp = (v, lo, hi) => (v < lo ? lo : (v > hi ? hi : v));
    const lerp = (a, b, t) => a + (b - a) * t;
    const lib = () => window.AuraPixelLib || null;
    if (!Brush || !toolUi) {
        if (window.console) console.error('pixel-tools-pro.js needs pixel-brush.js');
        return;
    }
    const { F, opt } = toolUi;

    // Draws over the active layer (or its mask) and commits through commitLayerPixels: selection, locks and history apply.
    function composeOver(r, label, draw) {
        const layer = r.activeLayer();
        if (!layer) return false;
        const inMask = !!(r.editMaskMode && layer.maskCanvas);
        const out = Pixel.acquireTempCanvas(r.imgWidth, r.imgHeight), ctx = out.getContext('2d');
        ctx.drawImage(inMask ? layer.maskCanvas : layer.canvas, 0, 0);
        draw(ctx, inMask);
        const ok = r.commitLayerPixels(layer, out, { label });
        Pixel.releaseTempCanvas(out);
        return ok;
    }
    const inMaskMode = r => !!(r.editMaskMode && r.activeLayer() && r.activeLayer().maskCanvas);
    const defineTool = toolUi.defineTool;
    const svg = body => '<svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">' + body + '</svg>';

    // --- shapes ---

    // Normalised geometry of a drag: lines keep their end points, boxes become {x0, y0, x1, y1}. Shift constrains, Alt draws from the centre.
    Pro.shapeGeo = function shapeGeo(id, d) {
        const line = id === 'line' || id === 'arrow', sx = d.x0, sy = d.y0;
        let ex = d.x1, ey = d.y1;
        if (d.shift) {
            if (line) {
                const len = Math.hypot(ex - sx, ey - sy), a = Math.round(Math.atan2(ey - sy, ex - sx) / (Math.PI / 4)) * Math.PI / 4;
                ex = sx + len * Math.cos(a); ey = sy + len * Math.sin(a);
            } else {
                const m = Math.max(Math.abs(ex - sx), Math.abs(ey - sy));
                ex = sx + m * (ex < sx ? -1 : 1); ey = sy + m * (ey < sy ? -1 : 1);
            }
        }
        const ax = d.alt ? 2 * sx - ex : sx, ay = d.alt ? 2 * sy - ey : sy;
        return line ? { x0: ax, y0: ay, x1: ex, y1: ey } : { x0: Math.min(ax, ex), y0: Math.min(ay, ey), x1: Math.max(ax, ex), y1: Math.max(ay, ey) };
    };

    // Regular polygon (inner = null) or star (inner = ratio of the inner radius) inscribed in an ellipse, first vertex on top.
    Pro.polyPoints = function polyPoints(cx, cy, rx, ry, n, inner) {
        const count = inner ? n * 2 : n, pts = [];
        for (let i = 0; i < count; i++) {
            const k = inner && i % 2 ? inner : 1, t = -Math.PI / 2 + i * Math.PI * 2 / count;
            pts.push({ x: cx + Math.cos(t) * rx * k, y: cy + Math.sin(t) * ry * k });
        }
        return pts;
    };

    function drawShape(ctx, id, s, g, colors) {
        const w = s.width, line = id === 'line' || id === 'arrow';
        ctx.save();
        ctx.lineWidth = w; ctx.lineJoin = 'round';
        ctx.lineCap = s.dash === 'dotted' ? 'round' : (s.cap || 'round');
        ctx.setLineDash(s.dash === 'dashed' ? [w * 3, w * 2] : (s.dash === 'dotted' ? [0.01, w * 2] : []));
        ctx.strokeStyle = colors.stroke; ctx.fillStyle = colors.fill;
        if (line) {
            const a = Math.atan2(g.y1 - g.y0, g.x1 - g.x0), head = Math.max(8, w * 2.2 * (s.arrowSize || 3)), cut = head * 0.7;
            const arrow = id === 'arrow';
            const ends = [g.x0 + (arrow && s.arrowStart ? Math.cos(a) * cut : 0), g.y0 + (arrow && s.arrowStart ? Math.sin(a) * cut : 0), g.x1 - (arrow && s.arrowEnd ? Math.cos(a) * cut : 0), g.y1 - (arrow && s.arrowEnd ? Math.sin(a) * cut : 0)];
            ctx.beginPath(); ctx.moveTo(ends[0], ends[1]); ctx.lineTo(ends[2], ends[3]); ctx.stroke();
            ctx.setLineDash([]); ctx.fillStyle = colors.stroke;
            const tip = (x, y, t) => {
                ctx.beginPath(); ctx.moveTo(x, y);
                ctx.lineTo(x - Math.cos(t - 0.42) * head, y - Math.sin(t - 0.42) * head);
                ctx.lineTo(x - Math.cos(t + 0.42) * head, y - Math.sin(t + 0.42) * head);
                ctx.closePath(); ctx.fill();
            };
            if (arrow && s.arrowEnd) tip(g.x1, g.y1, a);
            if (arrow && s.arrowStart) tip(g.x0, g.y0, a + Math.PI);
        } else {
            const inset = s.stroke ? w / 2 : 0, bx = g.x0 + inset, by = g.y0 + inset, bw = g.x1 - g.x0 - inset * 2, bh = g.y1 - g.y0 - inset * 2;
            if (bw > 0 && bh > 0) {
                ctx.beginPath();
                if (id === 'ellipse') ctx.ellipse(bx + bw / 2, by + bh / 2, bw / 2, bh / 2, 0, 0, Math.PI * 2);
                else if (id === 'polygon' || id === 'star') Pro.polyPoints(bx + bw / 2, by + bh / 2, bw / 2, bh / 2, id === 'star' ? s.points : s.sides, id === 'star' ? s.inner / 100 : null).forEach((p, i) => (i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)));
                else if (id === 'rounded-rect') {
                    const rad = Math.min(s.radius, bw / 2, bh / 2);
                    ctx.moveTo(bx + rad, by); ctx.arcTo(bx + bw, by, bx + bw, by + bh, rad); ctx.arcTo(bx + bw, by + bh, bx, by + bh, rad);
                    ctx.arcTo(bx, by + bh, bx, by, rad); ctx.arcTo(bx, by, bx + bw, by, rad);
                } else ctx.rect(bx, by, bw, bh);
                ctx.closePath();
                if (s.fill) ctx.fill();
                if (s.stroke) ctx.stroke();
            }
        }
        ctx.restore();
    }

    const shapeColors = r => (inMaskMode(r) ? { stroke: '#ffffff', fill: '#ffffff' } : { stroke: r.primaryColor, fill: r.secondaryColor });
    const SHAPE_KEYS = {
        rectangle: ['stroke', 'fill', 'width', 'dash', 'opacity', 'antialias'],
        'rounded-rect': ['stroke', 'fill', 'width', 'radius', 'dash', 'opacity', 'antialias'],
        ellipse: ['stroke', 'fill', 'width', 'dash', 'opacity', 'antialias'],
        polygon: ['stroke', 'fill', 'width', 'sides', 'dash', 'opacity', 'antialias'],
        star: ['stroke', 'fill', 'width', 'points', 'inner', 'dash', 'opacity', 'antialias'],
        line: ['width', 'cap', 'dash', 'opacity', 'antialias'],
        arrow: ['width', 'cap', 'arrowStart', 'arrowEnd', 'arrowSize', 'dash', 'opacity', 'antialias']
    };
    const SHAPE_FIELDS = {
        stroke: F('stroke', 'bool', 'pixel.shape_stroke', 'Outline', { default: true }),
        fill: F('fill', 'bool', 'pixel.fill_mode', 'Fill shape', { default: false }),
        width: F('width', 'range', 'pixel.stroke_width', 'Stroke', { min: 1, max: 200, step: 1, unit: 'px', default: 3 }),
        radius: F('radius', 'range', 'pixel.shape_radius', 'Corner radius', { min: 0, max: 500, step: 1, unit: 'px', default: 24 }),
        sides: F('sides', 'int', 'pixel.shape_sides', 'Sides', { min: 3, max: 24, default: 6 }),
        points: F('points', 'int', 'pixel.shape_points', 'Points', { min: 3, max: 24, default: 5 }),
        inner: F('inner', 'range', 'pixel.shape_inner', 'Inner radius', { min: 10, max: 95, step: 1, unit: '%', default: 45 }),
        dash: F('dash', 'select', 'pixel.shape_dash', 'Line style', { default: 'solid', options: [opt('solid', 'pixel.dash_solid', 'Solid'), opt('dashed', 'pixel.dash_dashed', 'Dashed'), opt('dotted', 'pixel.dash_dotted', 'Dotted')] }),
        cap: F('cap', 'select', 'pixel.shape_cap', 'Line cap', { default: 'round', options: [opt('round', 'pixel.cap_round', 'Round'), opt('butt', 'pixel.cap_butt', 'Flat'), opt('square', 'pixel.cap_square', 'Square')] }),
        arrowStart: F('arrowStart', 'bool', 'pixel.arrow_start', 'Head at start', { default: false }),
        arrowEnd: F('arrowEnd', 'bool', 'pixel.arrow_end', 'Head at end', { default: true }),
        arrowSize: F('arrowSize', 'range', 'pixel.arrow_size', 'Head size', { min: 1, max: 10, step: 1, default: 3 }),
        opacity: F('opacity', 'range', 'pixel.brush_opacity', 'Opacity', { min: 1, max: 100, step: 1, unit: '%', default: 100 }),
        antialias: F('antialias', 'bool', 'pixel.shape_antialias', 'Anti-aliasing', { default: true })
    };
    const SHAPE_META = {
        rectangle: {}, ellipse: {}, line: {}, arrow: {},
        'rounded-rect': { group: 'shape', order: 50, shortcut: 'Shift+R', labelKey: 'pixel.rounded_rect', fallback: 'Rounded Rectangle', svg: svg('<rect x="2" y="3.5" width="12" height="9" rx="3.5"/>') },
        polygon: { group: 'shape', order: 60, shortcut: 'Shift+O', labelKey: 'pixel.polygon', fallback: 'Polygon', svg: svg('<path d="M8 2l5.5 4-2.1 6.5H4.6L2.5 6z"/>') },
        star: { group: 'shape', order: 70, shortcut: 'Shift+S', labelKey: 'pixel.star', fallback: 'Star', svg: svg('<path d="M8 1.8l1.9 3.9 4.3.6-3.1 3 .7 4.3L8 11.6l-3.8 2 .7-4.3-3.1-3 4.3-.6z"/>') }
    };

    function commitShape(r, id, s, d) {
        const g = Pro.shapeGeo(id, d);
        if (id === 'line' || id === 'arrow' ? Math.hypot(g.x1 - g.x0, g.y1 - g.y0) < 1 : (g.x1 - g.x0 < 1 || g.y1 - g.y0 < 1)) return;
        const pad = Math.ceil(s.width) + (id === 'arrow' ? Math.ceil(Math.max(8, s.width * 2.2 * s.arrowSize)) : 0) + 2;
        const bx = Math.max(0, Math.floor(Math.min(g.x0, g.x1) - pad)), by = Math.max(0, Math.floor(Math.min(g.y0, g.y1) - pad));
        const ex = Math.min(r.imgWidth, Math.ceil(Math.max(g.x0, g.x1) + pad)), ey = Math.min(r.imgHeight, Math.ceil(Math.max(g.y0, g.y1) + pad));
        if (ex <= bx || ey <= by) return;
        const cv = Pixel.makeCanvas(ex - bx, ey - by), cx = cv.getContext('2d');
        cx.translate(-bx, -by);
        drawShape(cx, id, s, g, shapeColors(r));
        if (!s.antialias) {
            cx.setTransform(1, 0, 0, 1, 0, 0);
            const img = cx.getImageData(0, 0, cv.width, cv.height);
            for (let i = 3; i < img.data.length; i += 4) img.data[i] = img.data[i] >= 128 ? 255 : 0;
            cx.putImageData(img, 0, 0);
        }
        composeOver(r, 'shape:' + id, ctx => { ctx.globalAlpha = s.opacity / 100; ctx.drawImage(cv, bx, by); });
        r.addRecentColor(r.primaryColor);
    }

    Object.keys(SHAPE_KEYS).forEach(id => {
        const cfg = { id, schema: SHAPE_KEYS[id].map(k => SHAPE_FIELDS[k]), defaults: id === 'line' || id === 'arrow' ? { width: 4 } : {} };
        const draft = r => r.shapeDraft;
        defineTool(cfg, Object.assign({}, SHAPE_META[id], {
            raster: true, cursor: 'crosshair',
            onDown: (r, e, p) => { if (!e.button) r.shapeDraft = { x0: p.x, y0: p.y, x1: p.x, y1: p.y, shift: p.shift, alt: p.alt }; },
            onMove: (r, e, p) => {
                const d = draft(r);
                if (!d) return;
                Object.assign(d, { x1: p.x, y1: p.y, shift: p.shift, alt: p.alt });
                const g = Pro.shapeGeo(id, d);
                r.setStatus(id === 'line' || id === 'arrow' ? Math.round(Math.hypot(g.x1 - g.x0, g.y1 - g.y0)) + ' px' : Math.round(g.x1 - g.x0) + ' × ' + Math.round(g.y1 - g.y0));
                r.requestOverlay();
            },
            onUp: (r, e, p) => {
                const d = draft(r);
                r.shapeDraft = null;
                if (!d) return;
                Object.assign(d, { x1: p.x, y1: p.y, shift: p.shift, alt: p.alt });
                commitShape(r, id, toolUi.settings(r, cfg), d);
                r.updateStatus();
                r.requestOverlay();
            },
            onKey: (r, e) => { if (e.key === 'Escape' && draft(r)) { r.shapeDraft = null; r.requestOverlay(); return true; } return false; },
            onDeactivate: r => { r.shapeDraft = null; },
            overlay: (ctx, r) => {
                const d = draft(r);
                if (!d) return;
                ctx.globalAlpha = toolUi.settings(r, cfg).opacity / 100;
                drawShape(ctx, id, toolUi.settings(r, cfg), Pro.shapeGeo(id, d), shapeColors(r));
            }
        }));
    });

    // --- gradient ---

    // t (0..1) of a pixel for each gradient type; (x0, y0) -> (x1, y1) is the drag axis.
    Pro.gradientRender = function gradientRender(o, rect, scale) {
        const W = Math.max(1, Math.round(rect.w * scale)), H = Math.max(1, Math.round(rect.h * scale)), data = new Uint8ClampedArray(W * H * 4);
        const dx = o.x1 - o.x0, dy = o.y1 - o.y0, len = Math.hypot(dx, dy);
        if (!len) return { width: W, height: H, data };
        const ux = dx / len, uy = dy / len, ang = Math.atan2(dy, dx), lut = o.lut, rng = o.dither ? Brush.makeRng(7) : null;
        const tf = {
            linear: (rx, ry) => (rx * ux + ry * uy) / len,
            reflected: (rx, ry) => Math.abs(rx * ux + ry * uy) / len,
            radial: (rx, ry) => Math.hypot(rx, ry) / len,
            diamond: (rx, ry) => (Math.abs(rx * ux + ry * uy) + Math.abs(-rx * uy + ry * ux)) / len,
            conic: (rx, ry) => { let t = (Math.atan2(ry, rx) - ang) / (Math.PI * 2); return t - Math.floor(t); }
        }[o.type] || (() => 0);
        for (let j = 0, q = 0; j < H; j++) {
            const ry = rect.y + (j + 0.5) / scale - o.y0;
            for (let i = 0; i < W; i++, q += 4) {
                let t = clamp(tf(rect.x + (i + 0.5) / scale - o.x0, ry), 0, 1);
                if (o.reverse) t = 1 - t;
                const k = clamp(rng ? Math.floor(t * 255 + rng()) : Math.round(t * 255), 0, 255) * 4;
                data[q] = lut[k]; data[q + 1] = lut[k + 1]; data[q + 2] = lut[k + 2]; data[q + 3] = lut[k + 3];
            }
        }
        return { width: W, height: H, data };
    };

    const gradientPresets = () => (lib() && lib().GRADIENT_PRESETS) || [];
    function gradientStops(r, s) {
        if (inMaskMode(r)) return [{ pos: 0, color: '#ffffff' }, { pos: 1, color: '#ffffff', alpha: 0 }];
        if (s.source === 'fgbg') return [{ pos: 0, color: r.primaryColor }, { pos: 1, color: r.secondaryColor }];
        if (s.source === 'fgtrans') return [{ pos: 0, color: r.primaryColor }, { pos: 1, color: r.primaryColor, alpha: 0 }];
        const preset = lib() && lib().gradientPreset(String(s.source).replace('preset:', ''));
        return preset ? lib().cloneStops(preset.stops) : [{ pos: 0, color: r.primaryColor }, { pos: 1, color: r.secondaryColor }];
    }
    const GRADIENT_CFG = {
        id: 'gradient',
        schema: [
            F('type', 'select', 'pixel.gradient_mode', 'Gradient Mode', { default: 'linear', options: ['linear', 'radial', 'conic', 'diamond', 'reflected'].map(v => opt(v, 'pixel.gradient_' + v, v)) }),
            F('source', 'select', 'pixel.gradient_colors', 'Colors', { default: 'fgbg', options: [opt('fgbg', 'pixel.gradient_fg_bg', 'Primary to secondary'), opt('fgtrans', 'pixel.gradient_fg_transparent', 'Primary to transparent')].concat(gradientPresets().map(p => opt('preset:' + p.id, 'pixel.gradient_preset_' + p.id, p.id))) }),
            F('reverse', 'bool', 'pixel.gradient_reverse', 'Reverse', { default: false }),
            F('dither', 'bool', 'pixel.gradient_dither', 'Dither', { default: true }),
            F('opacity', 'range', 'pixel.brush_opacity', 'Opacity', { min: 1, max: 100, step: 1, unit: '%', default: 100 })
        ]
    };
    function gradientDrawOpts(r, s, d) {
        const stops = gradientStops(r, s);
        return { type: s.type, x0: d.x0, y0: d.y0, x1: d.x1, y1: d.y1, reverse: s.reverse, dither: s.dither, lut: lib().gradientLUT(stops, 256) };
    }
    function gradientRegion(r) {
        const m = r.selection ? r.getSelectionMask() : null;
        return m ? { x: m.x, y: m.y, w: m.w, h: m.h } : { x: 0, y: 0, w: r.imgWidth, h: r.imgHeight };
    }
    defineTool(GRADIENT_CFG, {
        raster: true, cursor: 'crosshair',
        onDown: (r, e, p) => { if (!e.button) r.gradientDraft = { x0: p.x, y0: p.y, x1: p.x, y1: p.y, proxy: null }; },
        onMove: (r, e, p) => {
            const d = r.gradientDraft;
            if (!d) return;
            d.x1 = p.x; d.y1 = p.y;
            if (p.shift) { const len = Math.hypot(d.x1 - d.x0, d.y1 - d.y0), a = Math.round(Math.atan2(d.y1 - d.y0, d.x1 - d.x0) / (Math.PI / 4)) * Math.PI / 4; d.x1 = d.x0 + len * Math.cos(a); d.y1 = d.y0 + len * Math.sin(a); }
            r.requestOverlay();
        },
        onUp: (r, e, p) => {
            const d = r.gradientDraft;
            r.gradientDraft = null;
            r.requestOverlay();
            if (!d || Math.hypot(d.x1 - d.x0, d.y1 - d.y0) < 2 || !lib()) return;
            const s = toolUi.settings(r, GRADIENT_CFG), rect = gradientRegion(r);
            const img = Pro.gradientRender(gradientDrawOpts(r, s, d), rect, 1);
            const cv = Pixel.makeCanvas(img.width, img.height);
            cv.getContext('2d').putImageData(new ImageData(img.data, img.width, img.height), 0, 0);
            composeOver(r, 'gradient', ctx => { ctx.globalAlpha = s.opacity / 100; ctx.drawImage(cv, rect.x, rect.y); });
        },
        onKey: (r, e) => { if (e.key === 'Escape' && r.gradientDraft) { r.gradientDraft = null; r.requestOverlay(); return true; } return false; },
        onDeactivate: r => { r.gradientDraft = null; },
        // A downscaled render of the gradient is the live preview; the handle line shows the axis.
        overlay: (ctx, r) => {
            const d = r.gradientDraft;
            if (!d || !lib()) return;
            const s = toolUi.settings(r, GRADIENT_CFG), rect = gradientRegion(r), scale = Math.min(1, 320 / Math.max(rect.w, rect.h));
            if (Math.hypot(d.x1 - d.x0, d.y1 - d.y0) >= 2) {
                const img = Pro.gradientRender(gradientDrawOpts(r, s, d), rect, scale);
                const cv = d.proxy || (d.proxy = Pixel.makeCanvas(img.width, img.height));
                cv.getContext('2d').putImageData(new ImageData(img.data, img.width, img.height), 0, 0);
                ctx.save();
                ctx.globalAlpha = s.opacity / 100;
                ctx.drawImage(cv, rect.x, rect.y, rect.w, rect.h);
                ctx.restore();
            }
            const lw = 1 / Math.max(0.05, r.zoom);
            [['#000000', 3 * lw], ['#ffffff', lw]].forEach(([color, width]) => {
                ctx.strokeStyle = color; ctx.lineWidth = width;
                ctx.beginPath(); ctx.moveTo(d.x0, d.y0); ctx.lineTo(d.x1, d.y1); ctx.stroke();
                ctx.beginPath(); ctx.arc(d.x0, d.y0, 4 * lw, 0, Math.PI * 2); ctx.arc(d.x1, d.y1, 4 * lw, 0, Math.PI * 2); ctx.stroke();
            });
        }
    });

    // --- paint bucket ---

    // Alpha-aware colour distance: alpha difference, or the largest channel difference weighted by the smaller alpha.
    function pixelDist(d, i, sr, sg, sb, sa) {
        const a = d[i + 3];
        if (!a && !sa) return 0;
        const m = Math.min(a, sa) / 255;
        return Math.max(Math.abs(a - sa), m * Math.max(Math.abs(d[i] - sr), Math.abs(d[i + 1] - sg), Math.abs(d[i + 2] - sb)));
    }

    // Region (255 inside) of pixels within `tol` of the seed colour: flood from the seed (scanline) or everywhere.
    Pro.floodRegion = function floodRegion(data, w, h, sx, sy, tol, contiguous) {
        const region = new Uint8ClampedArray(w * h), si = (sy * w + sx) * 4;
        const sr = data[si], sg = data[si + 1], sb = data[si + 2], sa = data[si + 3];
        const ok = p => pixelDist(data, p * 4, sr, sg, sb, sa) <= tol;
        if (!contiguous) {
            for (let p = 0; p < w * h; p++) if (ok(p)) region[p] = 255;
            return region;
        }
        const stack = [sx, sy];
        while (stack.length) {
            const y = stack.pop();
            let x = stack.pop();
            if (region[y * w + x] || !ok(y * w + x)) continue;
            while (x > 0 && !region[y * w + x - 1] && ok(y * w + x - 1)) x--;
            let up = false, down = false;
            for (; x < w && !region[y * w + x] && ok(y * w + x); x++) {
                region[y * w + x] = 255;
                if (y > 0) { const o = !region[(y - 1) * w + x] && ok((y - 1) * w + x); if (o && !up) stack.push(x, y - 1); up = o; }
                if (y < h - 1) { const o = !region[(y + 1) * w + x] && ok((y + 1) * w + x); if (o && !down) stack.push(x, y + 1); down = o; }
            }
        }
        return region;
    };

    // Soft edge: 3x3 box average inside the region's bounds, never lower than the region itself (grows a faint 1 px halo).
    Pro.softenRegion = function softenRegion(region, w, h) {
        let x0 = w, y0 = h, x1 = -1, y1 = -1;
        for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) if (region[y * w + x]) { if (x < x0) x0 = x; if (x > x1) x1 = x; if (y < y0) y0 = y; y1 = y; }
        if (x1 < 0) return region;
        x0 = Math.max(0, x0 - 1); y0 = Math.max(0, y0 - 1); x1 = Math.min(w - 1, x1 + 1); y1 = Math.min(h - 1, y1 + 1);
        const bw = x1 - x0 + 1, bh = y1 - y0 + 1, tmp = new Uint8ClampedArray(bw * bh);
        for (let y = y0; y <= y1; y++) {
            for (let x = x0; x <= x1; x++) {
                let sum = 0;
                for (let j = -1; j <= 1; j++) for (let i = -1; i <= 1; i++) { const xx = x + i, yy = y + j; if (xx >= 0 && yy >= 0 && xx < w && yy < h) sum += region[yy * w + xx]; }
                tmp[(y - y0) * bw + x - x0] = sum / 9;
            }
        }
        for (let y = y0; y <= y1; y++) for (let x = x0; x <= x1; x++) region[y * w + x] = Math.max(region[y * w + x], tmp[(y - y0) * bw + x - x0]);
        return region;
    };

    const PATTERNS = ['checker', 'stripes', 'dots', 'grid'];
    function patternFor(r, kind, ctx) {
        const tile = Pixel.makeCanvas(16, 16), t = tile.getContext('2d');
        t.fillStyle = r.secondaryColor; t.fillRect(0, 0, 16, 16);
        t.fillStyle = r.primaryColor; t.strokeStyle = r.primaryColor; t.lineWidth = 2;
        if (kind === 'checker') { t.fillRect(0, 0, 8, 8); t.fillRect(8, 8, 8, 8); }
        else if (kind === 'stripes') { t.beginPath(); t.moveTo(-4, 20); t.lineTo(20, -4); t.moveTo(-4, 12); t.lineTo(12, -4); t.moveTo(4, 20); t.lineTo(20, 4); t.stroke(); }
        else if (kind === 'dots') { t.beginPath(); t.arc(8, 8, 3.5, 0, Math.PI * 2); t.fill(); }
        else { t.strokeRect(0, 0, 16, 16); }
        return ctx.createPattern(tile, 'repeat');
    }
    const FILL_CFG = {
        id: 'fill',
        schema: [
            F('contiguous', 'bool', 'pixel.fill_contiguous', 'Contiguous', { default: true }),
            F('tolerance', 'range', 'pixel.tolerance', 'Tolerance', { min: 0, max: 255, step: 1, default: 32 }),
            F('sampleAll', 'bool', 'pixel.fill_sample_all', 'All layers', { default: false }),
            F('source', 'select', 'pixel.fill_source', 'Fill with', { default: 'primary', options: [opt('primary', 'pixel.fill_primary', 'Primary color'), opt('pattern', 'pixel.fill_pattern', 'Pattern')] }),
            F('pattern', 'select', 'pixel.fill_pattern_kind', 'Pattern', { default: 'checker', showIf: v => v.source === 'pattern', options: PATTERNS.map(v => opt(v, 'pixel.pattern_' + v, v)) }),
            F('antialias', 'bool', 'pixel.shape_antialias', 'Anti-aliasing', { default: true }),
            F('opacity', 'range', 'pixel.brush_opacity', 'Opacity', { min: 1, max: 100, step: 1, unit: '%', default: 100 })
        ]
    };
    defineTool(FILL_CFG, {
        raster: true, cursor: 'crosshair',
        onDown: (r, e, p) => {
            if (e.button) return;
            const x = Math.floor(p.x), y = Math.floor(p.y), w = r.imgWidth, h = r.imgHeight;
            if (x < 0 || y < 0 || x >= w || y >= h) return;
            const s = toolUi.settings(r, FILL_CFG);
            const flat = s.sampleAll ? r.flattenDocument() : null;
            const data = (flat ? flat.getContext('2d') : r.getActiveReadCtx()).getImageData(0, 0, w, h).data;
            let region = Pro.floodRegion(data, w, h, x, y, s.tolerance, s.contiguous);
            if (s.antialias) region = Pro.softenRegion(region, w, h);
            if (s.opacity < 100) for (let i = 0; i < region.length; i++) region[i] = region[i] * s.opacity / 100;
            const color = Pixel.acquireTempCanvas(w, h), cx = color.getContext('2d');
            cx.fillStyle = inMaskMode(r) ? '#ffffff' : (s.source === 'pattern' ? patternFor(r, s.pattern, cx) : r.primaryColor);
            cx.fillRect(0, 0, w, h);
            if (r.commitLayerPixels(r.activeLayer(), color, { label: 'fill', mask: region })) r.addRecentColor(r.primaryColor);
            Pixel.releaseTempCanvas(color);
        }
    });

    // --- retouch tools (stroke engine, effect and clone modes) ---

    const premul = (d, i) => d[i + 3] / 255;
    const EFFECT_KEYS = ['size', 'hardness', 'strength', 'spacing', 'pressure'];
    const effectFields = (extra, strength) => Brush.schema(['size', 'hardness']).concat([F('strength', 'range', 'pixel.retouch_strength', 'Strength', { min: 1, max: 100, step: 1, unit: '%', default: strength || 50 })], extra || [], Brush.schema(['spacing', 'pressure'], { spacing: 15, pressure: 'off' }));
    const strokeFields = (extra, spacing) => Brush.schema(['size', 'hardness', 'opacity']).concat(extra || [], Brush.schema(['spacing', 'pressure'], { spacing: spacing || 12, pressure: 'off' }));
    const retouch = (n, extra) => Object.assign({ group: 'retouch', order: n * 10 }, extra);
    const wt = (px, py, cx, cy, R, hard) => Brush.falloff(Math.hypot(px + 0.5 - cx, py + 0.5 - cy) / R, hard);

    // Smudge: a carried bitmap that sticks to the brush; each dab deposits it and picks some of the destination up.
    Brush.defineStrokeTool({
        id: 'smudge', mode: 'effect', panel: true, meta: retouch(4, { svg: svg('<path d="M3 11c2-4 4-6 8-7M3 11c1 2 3 3 5 2M5 9c2-2 4-3 7-3"/>'), shortcut: 'Y', labelKey: 'pixel.smudge', fallback: 'Smudge' }),
        schema: effectFields([F('finger', 'bool', 'pixel.smudge_finger', 'Finger painting', { default: false })]),
        defaults: { size: 30, hardness: 40, strength: 60 }, bar: ['size', 'hardness', 'strength', 'finger'],
        stamp(st, x, y, size, k) {
            const S = st.S, side = Math.ceil(size) + 2, bx = Math.round(x - side / 2), by = Math.round(y - side / 2), rect = S.rect(x, y, size);
            if (!rect) return;
            const old = S.read(rect), next = Brush.cloneImage(old), o = old.data, n = next.data, hard = st.s.hardness;
            const at = (px, py) => { const cx = rect.x + px - bx, cy = rect.y + py - by; return cx < 0 || cy < 0 || cx >= side || cy >= side ? -1 : (cy * side + cx) * 4; };
            if (!st.carry || st.carrySide !== side) {
                // First dab (or a new size): the brush picks up what is under it; finger painting starts with the primary colour.
                const fc = Pixel.hexToRgb(/^#[0-9a-f]{6}$/i.test(st.color) ? st.color : '#000000');
                st.carry = new Float32Array(side * side * 4);
                st.carrySide = side;
                for (let py = 0; py < rect.h; py++) {
                    for (let px = 0; px < rect.w; px++) {
                        const ci = at(px, py), di = (py * rect.w + px) * 4;
                        if (ci < 0) continue;
                        if (st.s.finger) { st.carry[ci] = fc.r; st.carry[ci + 1] = fc.g; st.carry[ci + 2] = fc.b; st.carry[ci + 3] = 255; }
                        else for (let c = 0; c < 4; c++) st.carry[ci + c] = o[di + c];
                    }
                }
                return;
            }
            const carry = st.carry;
            for (let py = 0; py < rect.h; py++) {
                for (let px = 0; px < rect.w; px++) {
                    const ci = at(px, py), di = (py * rect.w + px) * 4;
                    if (ci < 0) continue;
                    const raw = wt(rect.x + px, rect.y + py, x, y, size / 2, hard), w = raw * k;
                    if (raw <= 0) continue;
                    const da = premul(o, di), ca = carry[ci + 3] / 255, na = lerp(da, ca, w), pick = (1 - k) * raw, cn = lerp(ca, da, pick);
                    for (let c = 0; c < 3; c++) {
                        const dest = o[di + c], brush = carry[ci + c];
                        n[di + c] = na > 0.001 ? lerp(dest * da, brush * ca, w) / na : 0;
                        carry[ci + c] = cn > 0.001 ? lerp(brush * ca, dest * da, pick) / cn : 0;
                    }
                    n[di + 3] = na * 255;
                    carry[ci + 3] = cn * 255;
                }
            }
            S.write(rect, old, next);
        }
    });

    // Blur / sharpen brush: box blur of the dab area, mixed in (blur) or subtracted from the original (sharpen).
    Brush.defineStrokeTool({
        id: 'blur-brush', mode: 'effect', panel: true, meta: retouch(5, {}),
        schema: effectFields([F('mode', 'segmented', 'pixel.mode', 'Mode', { default: 'blur', options: [opt('blur', 'pixel.blur_mode_blur', 'Blur'), opt('sharpen', 'pixel.blur_mode_sharpen', 'Sharpen')] })]),
        defaults: { size: 40, hardness: 50, strength: 50 }, bar: ['mode', 'size', 'hardness', 'strength'],
        stamp(st, x, y, size, k) {
            const S = st.S, rect = S.rect(x, y, size), l = lib();
            if (!rect || !l) return;
            const old = S.read(rect), next = Brush.cloneImage(old), o = old.data, n = next.data;
            const blurred = l.boxBlurRGBA(o, rect.w, rect.h, clamp(Math.round(size / 18), 1, 12)), sharp = st.s.mode === 'sharpen';
            for (let py = 0; py < rect.h; py++) {
                for (let px = 0; px < rect.w; px++) {
                    const i = (py * rect.w + px) * 4, w = wt(rect.x + px, rect.y + py, x, y, size / 2, st.s.hardness) * k;
                    for (let c = 0; c < 4; c++) n[i + c] = sharp ? (c < 3 ? o[i + c] + (o[i + c] - blurred[i + c]) * w * 2.5 : o[i + c]) : lerp(o[i + c], blurred[i + c], w);
                }
            }
            S.write(rect, old, next);
        }
    });

    // Dodge / burn / sponge with a tonal range; strength is the exposure.
    Brush.defineStrokeTool({
        id: 'dodge-burn', mode: 'effect', panel: true, meta: retouch(6, {}),
        schema: effectFields([
            F('mode', 'select', 'pixel.mode', 'Mode', { default: 'dodge', options: [opt('dodge', 'pixel.dodge', 'Dodge'), opt('burn', 'pixel.burn', 'Burn'), opt('saturate', 'pixel.sponge_saturate', 'Saturate'), opt('desaturate', 'pixel.sponge_desaturate', 'Desaturate')] }),
            F('range', 'select', 'pixel.tone_range', 'Range', { default: 'midtones', options: [opt('shadows', 'pixel.range_shadows', 'Shadows'), opt('midtones', 'pixel.range_midtones', 'Midtones'), opt('highlights', 'pixel.range_highlights', 'Highlights')] })
        ], 25),
        defaults: { size: 40, hardness: 50, strength: 25 }, bar: ['mode', 'range', 'size', 'hardness', 'strength'],
        stamp(st, x, y, size, k) {
            const S = st.S, rect = S.rect(x, y, size), s = st.s;
            if (!rect) return;
            const old = S.read(rect), next = Brush.cloneImage(old), o = old.data, n = next.data;
            const smooth = (a, b, v) => { const t = clamp((v - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); };
            for (let py = 0; py < rect.h; py++) {
                for (let px = 0; px < rect.w; px++) {
                    const i = (py * rect.w + px) * 4, w = wt(rect.x + px, rect.y + py, x, y, size / 2, s.hardness) * k;
                    if (w <= 0 || !o[i + 3]) continue;
                    const l = (0.299 * o[i] + 0.587 * o[i + 1] + 0.114 * o[i + 2]) / 255;
                    const band = s.range === 'shadows' ? 1 - smooth(0.2, 0.55, l) : (s.range === 'highlights' ? smooth(0.45, 0.8, l) : 1 - smooth(0, 0.45, Math.abs(l - 0.5)));
                    const amt = w * band;
                    for (let c = 0; c < 3; c++) {
                        const v = o[i + c], gray = l * 255;
                        n[i + c] = s.mode === 'dodge' ? v + (255 - v) * amt * 0.6 : (s.mode === 'burn' ? v * (1 - amt * 0.6) : (s.mode === 'saturate' ? gray + (v - gray) * (1 + amt) : gray + (v - gray) * (1 - amt)));
                    }
                }
            }
            S.write(rect, old, next);
        }
    });

    // Colour replace: pixels close to the colour under the first dab take the primary colour's hue and saturation, luminance stays.
    Brush.defineStrokeTool({
        id: 'color-replace', mode: 'effect', panel: true, meta: retouch(7, { svg: svg('<circle cx="6" cy="8" r="4"/><circle cx="10.5" cy="8" r="4"/>'), shortcut: 'Shift+B', labelKey: 'pixel.color_replace', fallback: 'Color Replace' }),
        schema: effectFields([F('tolerance', 'range', 'pixel.tolerance', 'Tolerance', { min: 1, max: 255, step: 1, default: 50 })], 100),
        defaults: { size: 30, hardness: 80, strength: 100 }, bar: ['size', 'hardness', 'strength', 'tolerance'],
        stamp(st, x, y, size, k) {
            const S = st.S, rect = S.rect(x, y, size), l = lib(), s = st.s;
            if (!rect || !l) return;
            const old = S.read(rect), next = Brush.cloneImage(old), o = old.data, n = next.data;
            if (!st.base) { const c = Math.floor((rect.h / 2)) * rect.w * 4 + Math.floor(rect.w / 2) * 4; st.base = [o[c], o[c + 1], o[c + 2], o[c + 3]]; }
            const target = l.rgbToHsl(...Object.values(Pixel.hexToRgb(st.color))), tmp = [0, 0, 0], rgb = [0, 0, 0];
            for (let py = 0; py < rect.h; py++) {
                for (let px = 0; px < rect.w; px++) {
                    const i = (py * rect.w + px) * 4, w = wt(rect.x + px, rect.y + py, x, y, size / 2, s.hardness) * k;
                    if (w <= 0 || !o[i + 3]) continue;
                    const dist = pixelDist(o, i, st.base[0], st.base[1], st.base[2], st.base[3]);
                    if (dist > s.tolerance) continue;
                    const m = w * (1 - dist / s.tolerance * 0.5), lumaOld = l.luma(o[i], o[i + 1], o[i + 2]);
                    l.rgbToHsl(o[i], o[i + 1], o[i + 2], tmp);
                    l.hslToRgb(target[0], lerp(tmp[1], target[1], m), tmp[2], rgb);
                    const shift = lumaOld - l.luma(rgb[0], rgb[1], rgb[2]);
                    for (let c = 0; c < 3; c++) n[i + c] = lerp(o[i + c], clamp(rgb[c] + shift, 0, 255), m);
                }
            }
            S.write(rect, old, next);
        }
    });

    // Clone stamp and healing: a patch of the source (aligned offset, current layer or all layers) goes into the stroke buffer.
    function cloneSource(r, st) {
        st.src = st.s.sampleAll ? r.flattenDocument() : st.S.before;
        return st.src;
    }
    function patchAt(r, st, x, y, size, heal) {
        const side = Math.ceil(size) + 2, off = r.cloneOffset, sx = x - off.dx, sy = y - off.dy, tmp = Pixel.acquireTempCanvas(side, side), t = tmp.getContext('2d');
        t.drawImage(st.src, Math.round(sx - side / 2), Math.round(sy - side / 2), side, side, 0, 0, side, side);
        if (heal) {
            const dst = st.S.before.getContext('2d').getImageData(clamp(Math.round(x - side / 2), 0, st.S.w - 1), clamp(Math.round(y - side / 2), 0, st.S.h - 1), Math.min(side, st.S.w), Math.min(side, st.S.h));
            const img = t.getImageData(0, 0, side, side), d = img.data;
            const stats = (data, dw) => {
                const sum = [0, 0, 0], sq = [0, 0, 0]; let cnt = 0;
                for (let j = 0; j < side; j++) for (let i = 0; i < side; i++) {
                    const tt = Math.hypot(i + 0.5 - side / 2, j + 0.5 - side / 2) / (size / 2);
                    if (tt < 0.5 || tt > 1 || i >= dw || (j * dw + i) * 4 + 3 >= data.length) continue;
                    const q = (j * dw + i) * 4;
                    if (data[q + 3] < 8) continue;
                    for (let c = 0; c < 3; c++) { sum[c] += data[q + c]; sq[c] += data[q + c] * data[q + c]; }
                    cnt++;
                }
                return cnt ? { m: sum.map(v => v / cnt), s: sq.map((v, c) => Math.sqrt(Math.max(0, v / cnt - (sum[c] / cnt) ** 2))), n: cnt } : null;
            };
            const a = stats(d, side), b = stats(dst.data, dst.width);
            if (a && b) {
                const ratio = clamp((b.s[0] + b.s[1] + b.s[2] + 1) / (a.s[0] + a.s[1] + a.s[2] + 1), 0.8, 1.25);
                for (let q = 0; q < d.length; q += 4) for (let c = 0; c < 3; c++) d[q + c] = b.m[c] + (d[q + c] - a.m[c]) * ratio;
                t.putImageData(img, 0, 0);
            }
        }
        const mask = Brush.getStamp('round', size, st.s.hardness, '#000000');
        t.globalCompositeOperation = 'destination-in';
        t.drawImage(mask, (side - mask.width) / 2, (side - mask.height) / 2);
        return tmp;
    }
    const CLONE_FIELDS = [F('aligned', 'bool', 'pixel.clone_aligned', 'Aligned', { default: true }), F('sampleAll', 'bool', 'pixel.fill_sample_all', 'All layers', { default: false })];
    ['clone-stamp', 'healing'].forEach((id, n) => {
        const heal = id === 'healing';
        Brush.defineStrokeTool({
            id, mode: 'paint', panel: true, meta: retouch(n + 1, heal ? { svg: svg('<rect x="2.5" y="6" width="11" height="4" rx="2" transform="rotate(-40 8 8)"/><path d="M7 8h2M8 7v2"/>'), shortcut: 'Shift+J', labelKey: 'pixel.healing', fallback: 'Healing Brush' } : {}),
            altClick: (r, p) => { r.cloneSource = { x: p.x, y: p.y }; r.cloneOffset = null; r.notify({ type: 'info', message: r.t('pixel.clone_source_set') }); r.requestOverlay(); },
            schema: strokeFields(CLONE_FIELDS, 8), defaults: { size: 40, hardness: heal ? 60 : 70, spacing: 8 }, bar: ['size', 'hardness', 'opacity', 'aligned', 'sampleAll'],
            begin(r, st, p) {
                if (!r.cloneSource) { r.notify({ type: 'info', message: r.t('pixel.clone_stamp_hint') }); return false; }
                if (!st.s.aligned || !r.cloneOffset) r.cloneOffset = { dx: p.x - r.cloneSource.x, dy: p.y - r.cloneSource.y };
                cloneSource(r, st);
                return true;
            },
            stamp(st, x, y, size, alpha) {
                const tmp = patchAt(st.S.r, st, x, y, size, heal);
                st.S.paintCanvas(tmp, x, y, alpha, 0, 1);
                Pixel.releaseTempCanvas(tmp);
            },
            overlayExtra(ctx, r, pos) {
                const src = r.cloneSource, off = r.cloneOffset;
                if (!src) return;
                const c = off && pos ? { x: pos.x - off.dx, y: pos.y - off.dy } : src, lw = 1 / Math.max(0.05, r.zoom), arm = 7 * lw;
                ctx.save();
                [['#000000', 3 * lw], ['#ffffff', lw]].forEach(([color, width]) => { ctx.strokeStyle = color; ctx.lineWidth = width; ctx.beginPath(); ctx.moveTo(c.x - arm, c.y); ctx.lineTo(c.x + arm, c.y); ctx.moveTo(c.x, c.y - arm); ctx.lineTo(c.x, c.y + arm); ctx.stroke(); });
                ctx.restore();
            }
        });
    });

    // Fills the masked pixels of an image from their surroundings (push-pull pyramid + a few smoothing passes).
    Pro.diffusionFill = function diffusionFill(img, mask) {
        const w = img.width, h = img.height, out = new Uint8ClampedArray(img.data);
        const levels = [{ w, h, c: new Float32Array(w * h * 4), k: new Float32Array(w * h) }];
        for (let p = 0; p < w * h; p++) if (!mask[p]) { for (let c = 0; c < 4; c++) levels[0].c[p * 4 + c] = img.data[p * 4 + c]; levels[0].k[p] = 1; }
        while (levels[levels.length - 1].w > 1 || levels[levels.length - 1].h > 1) {
            const a = levels[levels.length - 1], nw = Math.max(1, (a.w + 1) >> 1), nh = Math.max(1, (a.h + 1) >> 1), b = { w: nw, h: nh, c: new Float32Array(nw * nh * 4), k: new Float32Array(nw * nh) };
            for (let y = 0; y < a.h; y++) for (let x = 0; x < a.w; x++) {
                const p = y * a.w + x, q = (y >> 1) * nw + (x >> 1), kk = a.k[p];
                if (kk > 0) { for (let c = 0; c < 4; c++) b.c[q * 4 + c] += a.c[p * 4 + c] * kk; b.k[q] += kk; }
            }
            for (let q = 0; q < nw * nh; q++) if (b.k[q] > 0) { for (let c = 0; c < 4; c++) b.c[q * 4 + c] /= b.k[q]; b.k[q] = Math.min(1, b.k[q] / 4); }
            levels.push(b);
        }
        for (let l = levels.length - 2; l >= 0; l--) {
            const a = levels[l], b = levels[l + 1];
            for (let y = 0; y < a.h; y++) for (let x = 0; x < a.w; x++) {
                const p = y * a.w + x;
                if (a.k[p] >= 1) continue;
                const fx = clamp(x / 2 - 0.25, 0, b.w - 1), fy = clamp(y / 2 - 0.25, 0, b.h - 1), x0 = Math.floor(fx), y0 = Math.floor(fy), x1 = Math.min(b.w - 1, x0 + 1), y1 = Math.min(b.h - 1, y0 + 1), tx = fx - x0, ty = fy - y0;
                for (let c = 0; c < 4; c++) {
                    const up = lerp(lerp(b.c[(y0 * b.w + x0) * 4 + c], b.c[(y0 * b.w + x1) * 4 + c], tx), lerp(b.c[(y1 * b.w + x0) * 4 + c], b.c[(y1 * b.w + x1) * 4 + c], tx), ty);
                    a.c[p * 4 + c] = a.c[p * 4 + c] * a.k[p] + up * (1 - a.k[p]);
                }
            }
        }
        const f = levels[0].c;
        for (let it = 0; it < 12; it++) {
            for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
                const p = y * w + x;
                if (!mask[p]) continue;
                for (let c = 0; c < 4; c++) f[p * 4 + c] = (f[(y > 0 ? p - w : p) * 4 + c] + f[(y < h - 1 ? p + w : p) * 4 + c] + f[(x > 0 ? p - 1 : p) * 4 + c] + f[(x < w - 1 ? p + 1 : p) * 4 + c]) / 4;
            }
        }
        for (let p = 0; p < w * h; p++) if (mask[p]) for (let c = 0; c < 4; c++) out[p * 4 + c] = f[p * 4 + c];
        return { width: w, height: h, data: out };
    };

    // Spot heal: the stroke only marks the blemish (red overlay); on release it is filled by inpaint.fill, or by the diffusion fallback.
    Brush.defineStrokeTool({
        id: 'spot-heal', mode: 'effect', panel: true, meta: retouch(3, { svg: svg('<circle cx="8" cy="8" r="3.2"/><path d="M8 1.5v2M8 12.5v2M1.5 8h2M12.5 8h2"/>'), shortcut: 'J', labelKey: 'pixel.spot_heal', fallback: 'Spot Healing' }),
        schema: Brush.schema(['size', 'hardness']), defaults: { size: 24, hardness: 100 }, bar: ['size'],
        begin(r, st) { st.mark = Pixel.makeCanvas(st.S.w, st.S.h); st.bounds = null; r.spotMark = st; },
        stamp(st, x, y, size) {
            const c = st.mark.getContext('2d');
            c.fillStyle = '#ff3040';
            c.beginPath(); c.arc(x, y, size / 2, 0, Math.PI * 2); c.fill();
            const R = size / 2 + 1;
            st.bounds = st.bounds ? { x0: Math.min(st.bounds.x0, x - R), y0: Math.min(st.bounds.y0, y - R), x1: Math.max(st.bounds.x1, x + R), y1: Math.max(st.bounds.y1, y + R) } : { x0: x - R, y0: y - R, x1: x + R, y1: y + R };
            st.S.r.requestOverlay();
        },
        done(r, st) {
            r.spotMark = null;
            const b = st.bounds, S = st.S, layer = S.layer;
            if (!b) return;
            const m = Math.ceil(Math.max(16, st.s.size)), x0 = Math.max(0, Math.floor(b.x0 - m)), y0 = Math.max(0, Math.floor(b.y0 - m)), x1 = Math.min(S.w, Math.ceil(b.x1 + m)), y1 = Math.min(S.h, Math.ceil(b.y1 + m));
            const w = x1 - x0, h = y1 - y0;
            if (w < 2 || h < 2) return;
            const src = Pixel.makeCanvas(w, h);
            src.getContext('2d').drawImage(S.before, x0, y0, w, h, 0, 0, w, h);
            const image = { width: w, height: h, data: src.getContext('2d').getImageData(0, 0, w, h).data };
            const mk = st.mark.getContext('2d').getImageData(x0, y0, w, h).data, mask = new Uint8ClampedArray(w * h);
            for (let p = 0; p < w * h; p++) mask[p] = mk[p * 4 + 3] >= 64 ? 255 : 0;
            const grown = lib() ? lib().morphAlpha(mask, w, h, 2, 'dilate') : mask;
            const inpaint = lib() && lib().ops && lib().ops['inpaint.fill'];
            const job = inpaint ? r.runJob('inpaint.fill', { image, mask: grown, patch: 7, iterations: 4 }, { key: 'spot-heal' }).then(res => (res && res.image ? res.image : res)) : Promise.resolve(Pro.diffusionFill(image, grown));
            job.then(res => {
                if (r.state.disposed || r.activeLayer() !== layer || !res || !res.data) return;
                const out = r.canvasFromImage(res), mc = Pixel.makeCanvas(w, h), mx = mc.getContext('2d'), md = Brush.blankImage(w, h);
                for (let p = 0; p < w * h; p++) md.data[p * 4 + 3] = grown[p];
                mx.putImageData(md, 0, 0);
                out.getContext('2d').globalCompositeOperation = 'destination-in';
                out.getContext('2d').drawImage(mc, 0, 0);
                composeOver(r, 'retouch:spot-heal', ctx => ctx.drawImage(out, x0, y0));
            }).catch(err => { if (!err || err.name !== 'AbortError') r.notify({ type: 'error', message: r.t('pixel.spot_heal_failed', 'Spot healing failed.') }); });
        },
        abort(r) { r.spotMark = null; },
        overlayExtra(ctx, r) { if (r.spotMark) { ctx.save(); ctx.globalAlpha = 0.55; ctx.drawImage(r.spotMark.mark, 0, 0); ctx.restore(); } }
    });

    // Red-eye: grows a region of strongly red pixels from the click and neutralises it.
    Pro.fixRedEye = function fixRedEye(img, cx, cy, radius, darken) {
        const w = img.width, h = img.height, d = img.data, seen = new Uint8Array(w * h), red = i => d[i + 3] > 40 && d[i] > 60 && d[i] > 1.6 * Math.max(d[i + 1], d[i + 2]);
        let seed = -1, best = 1e9;
        for (let y = Math.max(0, Math.floor(cy - radius)); y < Math.min(h, Math.ceil(cy + radius)); y++) {
            for (let x = Math.max(0, Math.floor(cx - radius)); x < Math.min(w, Math.ceil(cx + radius)); x++) {
                const dist = Math.hypot(x - cx, y - cy);
                if (dist <= radius && dist < best && red((y * w + x) * 4)) { best = dist; seed = y * w + x; }
            }
        }
        if (seed < 0) return 0;
        const stack = [seed], list = [];
        seen[seed] = 1;
        while (stack.length) {
            const p = stack.pop(), x = p % w, y = (p / w) | 0;
            list.push(p);
            [[1, 0], [-1, 0], [0, 1], [0, -1]].forEach(([dx, dy]) => {
                const nx = x + dx, ny = y + dy, q = ny * w + nx;
                if (nx < 0 || ny < 0 || nx >= w || ny >= h || seen[q] || Math.hypot(nx - cx, ny - cy) > radius * 1.2 || !red(q * 4)) return;
                seen[q] = 1; stack.push(q);
            });
        }
        list.forEach(p => {
            const i = p * 4, avg = (d[i + 1] + d[i + 2]) / 2, f = 1 - 0.55 * darken;
            d[i] = avg * f; d[i + 1] *= f; d[i + 2] *= f;
        });
        return list.length;
    };
    const RED_CFG = { id: 'red-eye', schema: [F('size', 'range', 'pixel.redeye_search', 'Search radius', { min: 5, max: 200, step: 1, unit: 'px', default: 40 }), F('darken', 'range', 'pixel.redeye_darken', 'Darken', { min: 0, max: 100, step: 1, unit: '%', default: 70 })] };
    defineTool(RED_CFG, retouch(8, {
        svg: svg('<ellipse cx="8" cy="8" rx="6.5" ry="4"/><circle cx="8" cy="8" r="1.8" fill="currentColor"/>'), shortcut: 'Shift+E', labelKey: 'pixel.red_eye', fallback: 'Red Eye', raster: true, cursor: 'crosshair',
        onDown: (r, e, p) => {
            if (e.button) return;
            const s = toolUi.settings(r, RED_CFG), w = r.imgWidth, h = r.imgHeight, R = s.size;
            const x0 = Math.max(0, Math.floor(p.x - R * 1.4)), y0 = Math.max(0, Math.floor(p.y - R * 1.4)), x1 = Math.min(w, Math.ceil(p.x + R * 1.4)), y1 = Math.min(h, Math.ceil(p.y + R * 1.4));
            if (x1 <= x0 || y1 <= y0) return;
            const read = r.getActiveReadCtx(), data = read.getImageData(x0, y0, x1 - x0, y1 - y0);
            if (!Pro.fixRedEye(data, p.x - x0, p.y - y0, R, s.darken / 100)) { r.notify({ type: 'info', message: r.t('pixel.redeye_none', 'No red eye found here.') }); return; }
            composeOver(r, 'retouch:red-eye', ctx => ctx.putImageData(data, x0, y0));
        }
    }));

    // --- view tools and View menu ---

    const ZOOM_STEPS = [0.05, 0.0625, 0.125, 0.25, 0.333, 0.5, 0.667, 1, 1.5, 2, 3, 4, 6, 8, 12, 16, 20];
    Pro.nextZoom = function nextZoom(z, dir) {
        if (dir > 0) return ZOOM_STEPS.find(s => s > z * 1.01) || ZOOM_STEPS[ZOOM_STEPS.length - 1];
        for (let i = ZOOM_STEPS.length - 1; i >= 0; i--) if (ZOOM_STEPS[i] < z / 1.01) return ZOOM_STEPS[i];
        return ZOOM_STEPS[0];
    };
    function zoomAt(r, zoom, docX, docY, clientX, clientY) {
        r.zoomTo(zoom);
        const rect = r.canvas.getBoundingClientRect();
        r.canvasArea.scrollLeft += rect.left + docX * r.zoom - clientX;
        r.canvasArea.scrollTop += rect.top + docY * r.zoom - clientY;
    }
    Pixel.registerTool({
        id: 'hand', group: 'view', order: 10, shortcut: 'H', labelKey: 'pixel.hand', fallback: 'Hand', cursor: 'grab', svg: svg('<path d="M5.5 8V3.5a1 1 0 012 0V7M7.5 7V2.8a1 1 0 012 0V7M9.5 7V3.6a1 1 0 012 0V8M11.5 8V5.6a1 1 0 012 0V10c0 2.5-1.8 4-4.3 4S5 13 4 11L2.8 9a1 1 0 011.6-1.2L5.5 9"/>'),
        onDown: (r, e) => { if (!e.button) { r.handDrag = { x: e.clientX, y: e.clientY, sl: r.canvasArea.scrollLeft, st: r.canvasArea.scrollTop }; r.overlayCanvas.style.cursor = 'grabbing'; } },
        onMove: (r, e) => { const d = r.handDrag; if (d) { r.canvasArea.scrollLeft = d.sl - (e.clientX - d.x); r.canvasArea.scrollTop = d.st - (e.clientY - d.y); } },
        onUp: r => { r.handDrag = null; r.overlayCanvas.style.cursor = 'grab'; }
    });
    Pixel.registerTool({
        id: 'zoom', group: 'view', order: 20, shortcut: 'Z', labelKey: 'pixel.zoom', fallback: 'Zoom', cursor: 'zoom-in', svg: svg('<circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5L14 14M5 7h4M7 5v4"/>'),
        onDown: (r, e, p) => { if (!e.button) r.zoomDrag = { x0: p.x, y0: p.y, x1: p.x, y1: p.y, cx: e.clientX, cy: e.clientY, alt: p.alt }; },
        onMove: (r, e, p) => { const d = r.zoomDrag; if (d) { d.x1 = p.x; d.y1 = p.y; r.requestOverlay(); } },
        onUp: (r, e, p) => {
            const d = r.zoomDrag;
            r.zoomDrag = null;
            r.requestOverlay();
            if (!d) return;
            const w = Math.abs(p.x - d.x0), h = Math.abs(p.y - d.y0);
            if (Math.hypot(e.clientX - d.cx, e.clientY - d.cy) > 6 && w > 1 && h > 1 && !d.alt) {
                const area = r.canvasArea, z = clamp(Math.min(area.clientWidth / w, area.clientHeight / h) * 0.96, 0.05, 20), mx = (p.x + d.x0) / 2, my = (p.y + d.y0) / 2;
                r.zoomTo(z);
                const rect = r.canvas.getBoundingClientRect(), ar = area.getBoundingClientRect();
                area.scrollLeft += rect.left + mx * r.zoom - (ar.left + area.clientWidth / 2);
                area.scrollTop += rect.top + my * r.zoom - (ar.top + area.clientHeight / 2);
            } else {
                zoomAt(r, Pro.nextZoom(r.zoom, d.alt || p.alt ? -1 : 1), p.x, p.y, e.clientX, e.clientY);
            }
        },
        overlay: (ctx, r) => {
            const d = r.zoomDrag;
            if (!d) return;
            const lw = 1 / Math.max(0.05, r.zoom);
            ctx.setLineDash([4 * lw, 3 * lw]);
            [['#000000', 2 * lw], ['#ffffff', lw]].forEach(([color, width]) => { ctx.strokeStyle = color; ctx.lineWidth = width; ctx.strokeRect(Math.min(d.x0, d.x1), Math.min(d.y0, d.y1), Math.abs(d.x1 - d.x0), Math.abs(d.y1 - d.y0)); });
        }
    });

    // Pixel grid (zoom >= 800 %) and the user grid, drawn under the selection outline.
    function paintGrids(ctx, r) {
        const v = r.viewOpts, z = r.zoom;
        if (!v || !r.imgWidth || !r.canvas || !r.canvasArea) return;
        const pixelGrid = v.pixelGrid && z >= 8;
        if (!pixelGrid && !v.grid) return;
        const rect = r.canvas.getBoundingClientRect(), ar = r.canvasArea.getBoundingClientRect();
        const vx0 = Math.max(0, Math.floor((ar.left - rect.left) / z)), vy0 = Math.max(0, Math.floor((ar.top - rect.top) / z));
        const vx1 = Math.min(r.imgWidth, Math.ceil((ar.right - rect.left) / z)), vy1 = Math.min(r.imgHeight, Math.ceil((ar.bottom - rect.top) / z));
        const lines = (step, color) => {
            if (step * z < 4 || (vx1 - vx0) / step > 800) return;
            ctx.strokeStyle = color; ctx.lineWidth = 1 / z;
            ctx.beginPath();
            for (let x = Math.ceil(vx0 / step) * step; x <= vx1; x += step) { ctx.moveTo(x, vy0); ctx.lineTo(x, vy1); }
            for (let y = Math.ceil(vy0 / step) * step; y <= vy1; y += step) { ctx.moveTo(vx0, y); ctx.lineTo(vx1, y); }
            ctx.stroke();
        };
        if (pixelGrid) lines(1, 'rgba(128,128,128,0.45)');
        if (v.grid) lines(Math.max(2, v.gridSize | 0), 'rgba(0,170,255,0.45)');
    }

    const viewToggle = (id, key, labelKey, label) => ({ id, labelKey, label, checked: r => !!r.viewOpts[key], action: r => { r.viewOpts[key] = !r.viewOpts[key]; r.requestOverlay(); r.refreshMenus(); } });
    Pixel.registerMenuItems('view', () => [
        { id: 'zoom-fill', labelKey: 'pixel.zoom_fill', label: 'Fill Screen', icon: 'maximize', action: r2 => r2.zoomFill() },
        { type: 'separator' },
        viewToggle('pixel-grid', 'pixelGrid', 'pixel.view_pixel_grid', 'Pixel Grid'),
        viewToggle('grid', 'grid', 'pixel.view_grid', 'Grid'),
        { id: 'grid-size', labelKey: 'pixel.view_grid_size', label: 'Grid Size', items: [8, 16, 32, 64, 128].map(n => ({ id: 'grid-size-' + n, label: n + ' px', checked: r2 => r2.viewOpts.gridSize === n, action: r2 => { r2.viewOpts.gridSize = n; r2.viewOpts.grid = true; r2.requestOverlay(); r2.refreshMenus(); } })) },
        viewToggle('snap', 'snap', 'pixel.view_snap', 'Snap to Guides')
    ].filter(Boolean), { order: 50 });

    Pixel.registerModule('tools-pro', function install(r) {
        r.viewOpts = { pixelGrid: true, grid: false, gridSize: 32, snap: true };
        Object.assign(r, {
            shapeDraft: null, gradientDraft: null, zoomDrag: null, handDrag: null, cloneSource: null, cloneOffset: null, spotMark: null,
            zoomFill: function zoomFill() {
                if (!r.imgWidth) return;
                r.zoomTo(clamp(Math.max((r.canvasArea.clientWidth - 20) / r.imgWidth, (r.canvasArea.clientHeight - 20) / r.imgHeight), 0.05, 20));
            }
        });
        r.registerOverlay('grids', paintGrids, 40);
        // The overlay canvas scales with the zoom: grids and line widths must be redrawn for the new zoom.
        const applyZoom = r.applyZoom;
        r.applyZoom = function patchedApplyZoom() { const out = applyZoom.apply(r, arguments); r.requestOverlay(); return out; };
        r.on('ready', () => { if (r.canvasArea) r.canvasArea.addEventListener('scroll', () => { if (r.viewOpts.grid || r.zoom >= 8) r.requestOverlay(); }, { passive: true }); });
        r.on('document-changed', () => { r.cloneSource = null; r.cloneOffset = null; });
    });
})();
