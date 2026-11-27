// Storymaker player: block painters. draw(ctx, st, rc, stage, win) paints one evaluated block state (see blocks.js) in the
// design frame: an optional panel, then the icon, diagram, chart, headline or list. Every painter works from a plan: the box
// it occupies and the smallest label it prints, both computed from the state alone, so measure() and draw() agree. All strokes
// are round and scaled from the 24 grid of the icons. Nothing here reads a clock or a random source: a frame is a pure
// function of the state and the scene time. Classic script; adds draw and plan to StorymakerCore.blocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var B = SM.blocks;

    var DASH = 96;   // the dash that reveals a stroke: a dash of DASH * p and a gap of DASH, in 24-grid units for icons
    var PANEL_ALPHA = 0.94, PANEL_RADIUS = 0.02;
    var pathCache = {}, pathCount = 0;

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function r3(v) { return Math.round(v * 1000) / 1000; }

    function pathOf(d) {
        var p = pathCache[d];
        if (!p) {
            if (pathCount > 2000) { pathCache = {}; pathCount = 0; }
            p = pathCache[d] = new Path2D(d);
            pathCount++;
        }
        return p;
    }

    // ---- colours: ink, accent, soft and the colour of a line on an accent fill, by surface (see the seams document)
    function colours(rc, surface) {
        var p = rc.palette || {}, dark = surface === 'dark';
        return {
            ink: dark ? '#f4f5eb' : (p.text || '#1f2a44'), accent: dark ? (p.accent2 || '#36cfc9') : (p.accent || '#ff7a45'),
            accent2: dark ? (p.accent || '#ff7a45') : (p.accent2 || '#36cfc9'),
            soft: dark ? 'rgba(255,255,255,0.10)' : (p.bg2 || '#ffe0a3'), bg: p.bg || '#fff7e6', onAccent: dark ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6'),
            panel: p.bg || '#fff7e6', base: dark ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6')
        };
    }

    // context(ctx, st, rc, stage): what every painter needs. base and floor are the label font and its smallest size in design pixels.
    function context(ctx, st, rc, stage) {
        return {
            ctx: ctx, st: st, rc: rc, stage: stage, col: colours(rc, st.surface), family: SM.render.familyOf(rc, { font: 'default' }),
            alpha: clamp(st.alpha, 0, 1), base: B.LABEL_BASE * rc.H / stage.scale, floor: B.LABEL_MIN * Math.min(rc.W, rc.H) / stage.scale
        };
    }

    // ---- text
    function setFont(c, weight, px) { c.ctx.font = SM.text.fontString(weight, px, c.family); }
    function widthOf(c, s, weight, px) { setFont(c, weight, px); return c.ctx.measureText(s).width; }

    // fit(c, text, weight, maxW, maxLines, px0) = {px, lines}: the largest font size up to px0, not below the label floor, whose
    // wrapped lines fit the width.
    function fit(c, text, weight, maxW, maxLines, px0) {
        var px = px0, out, words = String(text).split(/\s+/);
        for (;;) {
            setFont(c, weight, px);
            out = SM.text.wrap(function (s) { return c.ctx.measureText(s).width; }, text, maxW, maxLines);
            // a word that does not fit its line is broken by wrap; shrinking is better than a broken word
            var broken = words.some(function (w) { return c.ctx.measureText(w).width > maxW; });
            if ((!out.truncated && !broken) || px <= c.floor) break;
            px = Math.max(c.floor, px * 0.94);
        }
        return { px: px, lines: out.lines };
    }

    // printLines(c, lines, x, y, px, weight, color, align): the lines centred on y, 1.2 lines high.
    function printLines(c, lines, x, y, px, weight, color, align) {
        var ctx = c.ctx;
        setFont(c, weight, px);
        ctx.textAlign = align || 'center';
        ctx.textBaseline = 'middle';
        ctx.fillStyle = color;
        for (var i = 0; i < lines.length; i++) {
            ctx.direction = SM.ranges.dirOf(lines[i]);
            ctx.fillText(lines[i], x, y + (i - (lines.length - 1) / 2) * px * 1.2);
        }
    }

    // ---- icons

    // glyph(c, icon, cx, cy, size, color, p): the strokes of an icon on the 24 grid, line width 2 * size / 24, revealed to p.
    function glyph(c, icon, cx, cy, size, color, p) {
        if (!icon || !(p > 0.001)) return;
        var ctx = c.ctx, fills = icon.fills || [];
        ctx.save();
        ctx.translate(cx - size / 2, cy - size / 2);
        ctx.scale(size / 24, size / 24);
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
        ctx.lineWidth = 2;
        ctx.strokeStyle = color;
        ctx.fillStyle = color;
        ctx.setLineDash(p < 1 ? [r3(DASH * p), DASH] : []);
        for (var i = 0; i < icon.paths.length; i++) {
            var path = pathOf(icon.paths[i]);
            if (fills.indexOf(icon.paths[i]) >= 0) { ctx.globalAlpha = c.alpha * p; ctx.fill(path); ctx.globalAlpha = c.alpha; }
            ctx.stroke(path);
        }
        ctx.restore();
    }

    function iconPlan(c) {
        var st = c.st, b = st.block, ctx = c.ctx, icon = c.rc.cat && c.rc.cat.icons ? c.rc.cat.icons[b.icon] : null;
        var m = Math.min(st.w, st.h), style = B.ICON_STYLES.indexOf(b.style) >= 0 ? b.style : (b.label ? 'card' : 'badge');
        var p = st.entry === 'draw' ? B.itemProgress(st, 0) : 1;
        if (style === 'line') {
            var s = 0.6 * m;
            return { box: { x: -s / 2, y: -s / 2, w: s, h: s }, labelPx: 0, paint: function () { glyph(c, icon, 0, 0, s, c.col.ink, p); } };
        }
        if (style === 'badge') {
            var d = 0.72 * m;
            return {
                box: { x: -d / 2, y: -d / 2, w: d, h: d }, labelPx: 0,
                paint: function () {
                    ctx.globalAlpha = c.alpha * p;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(0, 0, d / 2, 0, Math.PI * 2);
                    ctx.fill();
                    ctx.globalAlpha = c.alpha;
                    glyph(c, icon, 0, 0, d * 0.52, c.col.onAccent, p);
                }
            };
        }
        // card: the icon on top, the label below. The card narrows until it fits the area.
        var cw = Math.min(st.w * 0.9, 0.72 * m), g, lab = null;
        for (var pass = 0; pass < 3; pass++) {
            var pad = 0.12 * cw, gs = 0.5 * cw, gap = 0.08 * cw;
            lab = b.label ? fit(c, b.label, 600, cw * 0.86, 3, Math.max(c.base, cw * 0.13)) : null;
            var labH = lab ? lab.lines.length * lab.px * 1.2 : 0;
            g = { pad: pad, gs: gs, gap: gap, labH: labH, h: lab ? pad * 2 + gs + gap + labH : cw };
            if (g.h <= st.h * 0.96) break;
            cw *= st.h * 0.96 / g.h;
        }
        var box = { x: -cw / 2, y: -g.h / 2, w: cw, h: g.h };
        return {
            box: box, labelPx: lab ? lab.px : 0,
            paint: function () {
                SM.text.roundRect(ctx, box.x, box.y, box.w, box.h, 0.08 * cw);
                ctx.globalAlpha = c.alpha * p;
                ctx.fillStyle = c.col.soft;
                ctx.fill();
                ctx.lineWidth = Math.max(3, g.gs / 24);
                ctx.strokeStyle = c.col.ink;
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                ctx.stroke();
                ctx.globalAlpha = c.alpha;
                var top = box.y + (lab ? g.pad : (box.h - g.gs) / 2);
                glyph(c, icon, 0, top + g.gs / 2, g.gs, c.col.ink, p);
                if (lab) {
                    ctx.globalAlpha = c.alpha * p;
                    printLines(c, lab.lines, 0, top + g.gs + g.gap + g.labH / 2, lab.px, 600, c.col.ink, 'center');
                    ctx.globalAlpha = c.alpha;
                }
            }
        };
    }

    // ---- diagrams

    // edge(a, b, gap) = [x1, y1, x2, y2]: the arrow between two boxes along their centres, trimmed to the box edges plus gap.
    function edge(a, b, gap) {
        var ax = a.x + a.w / 2, ay = a.y + a.h / 2, dx = b.x + b.w / 2 - ax, dy = b.y + b.h / 2 - ay, len = Math.hypot(dx, dy);
        function exit(box) { return Math.min(dx ? box.w / 2 / Math.abs(dx) : 1e9, dy ? box.h / 2 / Math.abs(dy) : 1e9); }
        var t0 = exit(a) + gap / len, t1 = 1 - exit(b) - gap / len;
        return t1 - t0 > 0.02 ? [ax + dx * t0, ay + dy * t0, ax + dx * t1, ay + dy * t1] : null;
    }

    // arrow(c, seg, lw, p, color): a line that grows from its start to p of its length, with a filled head at the tip.
    function arrow(c, seg, lw, p, color) {
        var ctx = c.ctx, x1 = seg[0], y1 = seg[1], a = Math.atan2(seg[3] - y1, seg[2] - seg[0]), co = Math.cos(a), si = Math.sin(a);
        var x2 = x1 + (seg[2] - x1) * p, y2 = y1 + (seg[3] - y1) * p, hs = 4.4 * lw * Math.min(1, p * 2);
        ctx.strokeStyle = color;
        ctx.fillStyle = color;
        ctx.lineWidth = lw;
        ctx.beginPath();
        ctx.moveTo(x1, y1);
        ctx.lineTo(x2 - co * hs * 0.5, y2 - si * hs * 0.5);
        ctx.stroke();
        ctx.beginPath();
        ctx.moveTo(x2, y2);
        ctx.lineTo(x2 - co * hs - si * hs * 0.5, y2 - si * hs + co * hs * 0.5);
        ctx.lineTo(x2 - co * hs + si * hs * 0.5, y2 - si * hs - co * hs * 0.5);
        ctx.closePath();
        ctx.fill();
        ctx.stroke();
    }

    // nodeLayout(c, node, box, px, compare): where the icon and the text of one node go; the label is fitted at px or smaller.
    function nodeLayout(c, node, box, px, compare) {
        var icons = c.rc.cat && c.rc.cat.icons, icon = node.icon && icons ? icons[node.icon] : null, m = Math.min(box.w, box.h), pad = 0.1 * m, o = { box: box, icon: icon, pad: pad, gs: 0 };
        if (compare) { o.mode = 'compare'; o.gs = icon ? 0.12 * box.h : 0; o.fit = fit(c, node.label, 700, box.w - pad * 2 - (icon ? o.gs + pad : 0), 2, px); }
        else if (icon && box.w >= box.h * 1.5) { o.mode = 'side'; o.gs = 0.56 * box.h; o.fit = fit(c, node.label, 600, box.w - pad * 3 - o.gs, 2, px); }
        else if (icon) {
            o.mode = 'top';
            o.fit = fit(c, node.label, 600, box.w - pad * 2, 2, px);
            o.gs = Math.min(0.46 * box.w, 0.5 * box.h, box.h - pad * 2.6 - o.fit.lines.length * o.fit.px * 1.2);
            // no room for the icon above the label: the label alone
            if (o.gs < 0.3 * box.h) { o.gs = 0; o.mode = 'plain'; o.icon = null; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        }
        else { o.mode = 'plain'; o.fit = fit(c, node.label, 600, box.w * 0.86, 3, px); }
        if (compare) {
            var pts = (node.points || []).map(function (s) { return fit(c, s, 500, box.w - pad * 3.2, 2, Math.max(c.floor, o.fit.px * 0.8)); });
            o.points = pts;
        }
        return o;
    }

    function paintNode(c, o, node, p, lw, accent, mark) {
        var ctx = c.ctx, b = o.box, cx = b.x + b.w / 2, cy = b.y + b.h / 2, draw = c.st.entry === 'draw', ink = accent ? c.col.onAccent : c.col.ink;
        var k = (mark && mark.style === 'pulse' ? 1 + 0.08 * Math.sin(4 * Math.PI * mark.u) : 1) * (draw ? 1 : 0.92 + 0.08 * p);
        ctx.save();
        ctx.translate(cx, cy);
        ctx.scale(k, k);
        ctx.translate(-cx, -cy);
        SM.text.roundRect(ctx, b.x, b.y, b.w, b.h, 0.1 * Math.min(b.w, b.h));
        ctx.globalAlpha = c.alpha * p;
        ctx.fillStyle = accent ? c.col.accent : c.col.soft;
        ctx.fill();
        ctx.globalAlpha = draw ? c.alpha : c.alpha * p;
        ctx.strokeStyle = c.col.ink;
        ctx.lineWidth = lw;
        ctx.setLineDash(draw && p < 1 ? [r3(2 * (b.w + b.h) * p), 2 * (b.w + b.h)] : []);
        ctx.stroke();
        ctx.setLineDash([]);
        ctx.globalAlpha = c.alpha * p;
        var f = o.fit, lh = f.lines.length * f.px * 1.2, top = b.y + o.pad, gs = o.gs;
        if (o.mode === 'top') {
            var y0 = cy - (gs + o.pad * 0.6 + lh) / 2;
            glyph(c, o.icon, cx, y0 + gs / 2, gs, ink, draw ? p : 1);
            printLines(c, f.lines, cx, y0 + gs + o.pad * 0.6 + lh / 2, f.px, 600, ink, 'center');
        } else if (o.mode === 'side') {
            glyph(c, o.icon, b.x + o.pad + gs / 2, cy, gs, ink, draw ? p : 1);
            printLines(c, f.lines, b.x + o.pad * 2 + gs + (b.w - o.pad * 3 - gs) / 2, cy, f.px, 600, ink, 'center');
        } else if (o.mode === 'plain') printLines(c, f.lines, cx, cy, f.px, 600, ink, 'center');
        else {
            var head = Math.max(gs, lh), x0 = b.x + o.pad + (o.icon ? gs + o.pad : 0), y = top + head / 2;
            if (o.icon) glyph(c, o.icon, b.x + o.pad + gs / 2, y, gs, c.col.accent, draw ? p : 1);
            printLines(c, f.lines, x0 + (b.x + b.w - o.pad - x0) / 2, y, f.px, 700, ink, 'center');
            y = top + head + o.pad * 0.5;
            ctx.strokeStyle = c.col.ink;
            ctx.lineWidth = lw * 0.5;
            ctx.beginPath();
            ctx.moveTo(b.x + o.pad, y);
            ctx.lineTo(b.x + b.w - o.pad, y);
            ctx.stroke();
            o.points.forEach(function (pt) {
                var ph = pt.lines.length * pt.px * 1.2;
                y += o.pad * 0.5 + ph / 2;
                ctx.fillStyle = c.col.accent;
                ctx.beginPath();
                ctx.arc(b.x + o.pad + pt.px * 0.2, y, pt.px * 0.18, 0, Math.PI * 2);
                ctx.fill();
                printLines(c, pt.lines, b.x + o.pad + pt.px * 0.7, y, pt.px, 500, ink, 'left');
                y += ph / 2;
            });
        }
        ctx.globalAlpha = c.alpha;
        if (mark && mark.style === 'outline') {
            ctx.strokeStyle = c.col.accent;
            ctx.lineWidth = lw * 1.6;
            SM.text.roundRect(ctx, b.x - lw * 1.4, b.y - lw * 1.4, b.w + lw * 2.8, b.h + lw * 2.8, 0.1 * Math.min(b.w, b.h) + lw);
            ctx.stroke();
        } else if (mark && mark.style === 'arrow') {
            var tip = b.y - lw * 3 - lw * 2 * Math.sin(4 * Math.PI * mark.u);   // the arrow bobs like the one of drawHighlight
            ctx.fillStyle = c.col.accent;
            ctx.beginPath();
            ctx.moveTo(cx, tip);
            ctx.lineTo(cx - lw * 4.5, tip - lw * 8);
            ctx.lineTo(cx + lw * 4.5, tip - lw * 8);
            ctx.closePath();
            ctx.fill();
        }
        ctx.restore();
    }

    function diagramPlan(c) {
        var st = c.st, d = st.block.diagram, n = d.nodes.length, L = B.layout(d.layout, n, st.w, st.h), compare = d.layout === 'compare';
        var m = Math.min.apply(null, L.boxes.map(function (b) { return Math.min(b.w, b.h); }));
        var px = clamp(0.11 * L.boxes[0].w, c.base, c.base * 1.7), items, i;
        items = d.nodes.map(function (node, k) { return nodeLayout(c, node, L.boxes[k], px, compare); });
        for (i = 0, px = Infinity; i < n; i++) px = Math.min(px, items[i].fit.px);
        items = d.nodes.map(function (node, k) { return nodeLayout(c, node, L.boxes[k], px, compare); });
        var x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
        L.boxes.forEach(function (b) { x0 = Math.min(x0, b.x); y0 = Math.min(y0, b.y); x1 = Math.max(x1, b.x + b.w); y1 = Math.max(y1, b.y + b.h); });
        var lw = Math.max(3, Math.min(0.045 * m, 12) / 2);
        return {
            box: { x: x0, y: y0, w: x1 - x0, h: y1 - y0 }, labelPx: px,
            paint: function () {
                var ctx = c.ctx, flow = d.flow === true;
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                L.links.forEach(function (lk) {
                    var seg = edge(L.boxes[lk[0]], L.boxes[lk[1]], lw * 2), p = B.itemProgress(st, lk[1]);
                    if (!seg || !(p > 0)) return;
                    ctx.globalAlpha = c.alpha;
                    arrow(c, seg, lw, p, c.col.ink);
                    if (!flow || p < 1) return;
                    for (var j = 0; j < 2; j++) {
                        var f = (Math.round((st.time / B.FLOW_PERIOD + j / 2) * 1e6) / 1e6) % 1, hs = 4.4 * lw;
                        var len = Math.hypot(seg[2] - seg[0], seg[3] - seg[1]), u = Math.max(0, len - hs) / len;
                        ctx.globalAlpha = c.alpha * Math.sin(Math.PI * f);
                        ctx.fillStyle = c.col.accent;
                        ctx.beginPath();
                        ctx.arc(seg[0] + (seg[2] - seg[0]) * u * f, seg[1] + (seg[3] - seg[1]) * u * f, lw * 1.1, 0, Math.PI * 2);
                        ctx.fill();
                    }
                });
                d.nodes.forEach(function (node, k) {
                    var p = B.itemProgress(st, k), mark = st.mark && st.mark.node === node.id ? st.mark : null;
                    if (p > 0) paintNode(c, items[k], node, p, lw, d.layout === 'hub' && k === 0, mark);
                });
            }
        };
    }

    // ---- charts

    function numText(v) { return Math.abs(v - Math.round(v)) < 1e-9 ? String(Math.round(v)) : String(Math.round(v * 100) / 100); }
    function unitText(u) { return u ? (u.charAt(0) === '%' || u.charCodeAt(0) === 176 ? u : ' ' + u) : ''; }
    function decimals(v) { var s = String(Math.abs(v)), i = s.indexOf('.'); return i < 0 || s.indexOf('e') >= 0 ? 0 : Math.min(3, s.length - i - 1); }

    // fitSet(c, texts, weight, maxW, maxLines, px0) = {px, lines: [[line]]}: one font size for all texts, so the labels of a chart match.
    function fitSet(c, texts, weight, maxW, maxLines, px0) {
        var px = px0;
        texts.forEach(function (t) { px = Math.min(px, fit(c, t, weight, maxW, maxLines, px0).px); });
        return { px: px, lines: texts.map(function (t) { return fit(c, t, weight, maxW, maxLines, px).lines; }) };
    }

    // axisPlan: bar and line charts on a zero baseline. The labels sit under the baseline, the values above the marks.
    function axisPlan(c, f) {
        var st = c.st, ch = st.block.chart, vals = ch.values, n = vals.length, p = B.itemProgress(st, 0), ctx = c.ctx, line = ch.type === 'line';
        var step = Math.ceil(n / 8), cell = (f.x1 - f.x0) / n, shown = [], i;
        for (i = 0; i < n; i += step) shown.push(i);
        var lab = fitSet(c, shown.map(function (k) { return String(ch.labels[k]); }), 600, cell * step * 0.92, 2, c.base);
        var maxLines = Math.max.apply(null, lab.lines.map(function (l) { return l.length; })), valPx = lab.px * 0.9;
        var py0 = f.top + valPx * 1.8, py1 = f.y1 - maxLines * lab.px * 1.2 - lab.px * 0.6;
        var lo = Math.min(0, Math.min.apply(null, vals)), hi = Math.max(0, Math.max.apply(null, vals));
        if (hi === lo) hi = lo + 1;
        var yOf = function (v) { return py0 + (hi - v) / (hi - lo) * (py1 - py0); }, yz = yOf(0), xOf = function (k) { return f.x0 + cell * (k + 0.5); };
        var value = function (k, y, a) {
            ctx.globalAlpha = c.alpha * a;
            printLines(c, [numText(vals[k]) + f.unit], xOf(k), vals[k] >= 0 ? y - valPx * 0.8 : y + valPx * 0.8, valPx, 700, c.col.ink, 'center');
        };
        return {
            box: f.box, labelPx: lab.px,
            paint: function () {
                f.title();
                ctx.lineCap = 'round';
                ctx.lineJoin = 'round';
                ctx.globalAlpha = c.alpha;
                ctx.strokeStyle = c.col.ink;
                ctx.lineWidth = f.lw;
                ctx.beginPath();
                ctx.moveTo(f.x0, yz);
                ctx.lineTo(f.x1, yz);
                ctx.stroke();
                shown.forEach(function (k, j) { printLines(c, lab.lines[j], xOf(k), py1 + lab.px * 0.6 + lab.lines[j].length * lab.px * 0.6, lab.px, 600, c.col.ink, 'center'); });
                if (!line) {
                    var bw = Math.min(cell * 0.58, 0.2 * st.h);
                    vals.forEach(function (v, k) {
                        var hgt = Math.abs(yz - yOf(v)) * p;
                        ctx.globalAlpha = c.alpha;
                        ctx.fillStyle = c.col.accent;
                        ctx.fillRect(xOf(k) - bw / 2, v >= 0 ? yz - hgt : yz, bw, hgt);
                        value(k, v >= 0 ? yz - hgt : yz + hgt, clamp((p - 0.6) / 0.4, 0, 1));
                    });
                    return;
                }
                var total = 0;
                for (var k = 1; k < n; k++) total += Math.hypot(xOf(k) - xOf(k - 1), yOf(vals[k]) - yOf(vals[k - 1]));
                ctx.globalAlpha = c.alpha;
                ctx.strokeStyle = c.col.accent;
                ctx.lineWidth = f.lw * 1.6;
                ctx.setLineDash(p < 1 ? [r3(total * p), r3(total)] : []);
                ctx.beginPath();
                vals.forEach(function (v, j) { if (j) ctx.lineTo(xOf(j), yOf(v)); else ctx.moveTo(xOf(0), yOf(v)); });
                ctx.stroke();
                ctx.setLineDash([]);
                vals.forEach(function (v, j) {
                    var a = clamp(p * (n - 1) - j + 1, 0, 1);
                    if (!(a > 0) || n > 12) return;
                    ctx.globalAlpha = c.alpha * a;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(xOf(j), yOf(v), f.lw * 1.8, 0, Math.PI * 2);
                    ctx.fill();
                    if (n <= 6) value(j, yOf(v) - f.lw * 2, a);
                });
            }
        };
    }

    // piePlan: a donut with a legend; the slices sweep clockwise from the top while p grows.
    function piePlan(c, f) {
        var st = c.st, ch = st.block.chart, vals = ch.values, n = vals.length, p = B.itemProgress(st, 0), ctx = c.ctx, sum = 0;
        vals.forEach(function (v) { sum += v; });
        var wide = f.x1 - f.x0 > (f.y1 - f.top) * 1.35, R = wide ? Math.min((f.y1 - f.top) / 2, (f.x1 - f.x0) * 0.3) : Math.min((f.x1 - f.x0) / 2, (f.y1 - f.top) * 0.36);
        var rows = ch.labels.map(function (l, k) { return String(l) + ' ' + Math.round(100 * vals[k] / sum) + '%'; });
        var lab = fitSet(c, rows, 600, wide ? f.x1 - f.x0 - R * 2.45 - c.base * 1.4 : f.x1 - f.x0 - c.base * 1.4, 1, c.base), rowH = lab.px * 1.9;
        // the donut and its legend are centred as one group
        var rowsW = Math.max.apply(null, rows.map(function (s) { return widthOf(c, s, 600, lab.px); })) + lab.px * 1.3;
        var cx = wide ? f.x0 + (f.x1 - f.x0 - R * 2.45 - rowsW) / 2 + R : (f.x0 + f.x1) / 2, cy = wide ? (f.top + f.y1) / 2 : f.top + R, r = R * 0.58;
        var legendX = wide ? cx + R * 1.45 : (f.x0 + f.x1 - rowsW) / 2, legendTop = wide ? 0 : cy + R + R * 0.2;
        var top = wide ? cy - n * rowH / 2 : legendTop, tint = function (k) { return [c.col.accent, c.col.accent2, c.col.ink][k % 3]; };
        return {
            box: f.box, labelPx: lab.px,
            paint: function () {
                f.title();
                var a0 = -Math.PI / 2, sweep = a0 + 2 * Math.PI * p;
                vals.forEach(function (v, k) {
                    var a1 = a0 + 2 * Math.PI * v / sum, end = Math.min(a1, sweep);
                    if (end > a0) {
                        ctx.globalAlpha = c.alpha * (k < 3 ? 1 : 0.55);
                        ctx.fillStyle = tint(k);
                        ctx.strokeStyle = c.col.base;
                        ctx.lineWidth = f.lw * 0.8;
                        ctx.lineJoin = 'round';
                        ctx.beginPath();
                        ctx.arc(cx, cy, R, a0, end);
                        ctx.arc(cx, cy, r, end, a0, true);
                        ctx.closePath();
                        ctx.fill();
                        ctx.stroke();
                    }
                    var show = clamp((p - 0.3) / 0.7, 0, 1), y = top + rowH * (k + 0.5);
                    ctx.globalAlpha = c.alpha * show * (k < 3 ? 1 : 0.55);
                    ctx.fillStyle = tint(k);
                    SM.text.roundRect(ctx, legendX, y - lab.px * 0.4, lab.px * 0.8, lab.px * 0.8, lab.px * 0.2);
                    ctx.fill();
                    ctx.globalAlpha = c.alpha * show;
                    printLines(c, lab.lines[k], legendX + lab.px * 1.3, y, lab.px, 600, c.col.ink, 'left');
                    a0 = a1;
                });
            }
        };
    }

    // counterPlan: one big number that counts up to its value.
    function counterPlan(c, f) {
        var st = c.st, ch = st.block.chart, v = ch.values[0], p = B.itemProgress(st, 0), whole = v.toFixed(decimals(v)) + unitText(ch.unit);
        var cap = ch.labels[0] ? fit(c, String(ch.labels[0]), 600, f.x1 - f.x0, 2, c.base * 1.2) : null, ctx = c.ctx;
        var px = Math.min(0.42 * (f.y1 - f.top), 0.9 * (f.x1 - f.x0) / Math.max(1, whole.length * 0.62)), mid = (f.top + f.y1) / 2 - (cap ? cap.px * 0.7 : 0);
        return {
            box: f.box, labelPx: cap ? cap.px : 0,
            paint: function () {
                f.title();
                ctx.globalAlpha = c.alpha;
                printLines(c, [(v * p).toFixed(decimals(v)) + unitText(ch.unit)], 0, mid, px, 700, c.col.accent, 'center');
                if (cap) printLines(c, cap.lines, 0, mid + px * 0.62 + cap.px * 1.1, cap.px, 600, c.col.ink, 'center');
            }
        };
    }

    function chartPlan(c) {
        var st = c.st, ch = st.block.chart, pad = 0.06 * Math.min(st.w, st.h), f = { x0: -st.w / 2 + pad, x1: st.w / 2 - pad, y1: st.h / 2 - pad };
        var title = ch.title ? fit(c, ch.title, 700, f.x1 - f.x0, 1, c.base * 1.25) : null;
        f.top = -st.h / 2 + pad + (title ? title.px * 1.6 : 0);
        f.box = { x: -st.w / 2, y: -st.h / 2, w: st.w, h: st.h };
        f.lw = Math.max(3, Math.min(st.w, st.h) * 0.008);
        f.unit = unitText(ch.unit);
        f.title = function () { if (title) printLines(c, title.lines, 0, -st.h / 2 + pad + title.px * 0.6, title.px, 700, c.col.ink, 'center'); };
        var plan = ch.type === 'pie' ? piePlan(c, f) : (ch.type === 'counter' ? counterPlan(c, f) : axisPlan(c, f));
        if (title) plan.labelPx = Math.min(plan.labelPx || title.px, title.px);
        return plan;
    }

    // ---- headline and list

    // headlinePlan: the words wrap into rows, centred; each word rises into place in its turn, the marked word gets a marker.
    function headlinePlan(c) {
        var st = c.st, hp = B.parseHeadline(st.block.text), words = hp.words, ctx = c.ctx, px = Math.min(c.base * 2.4, st.h * 0.4), rows, ww;
        var rtl = SM.ranges.dirOf(st.block.text) === 'rtl', maxW = st.w * 0.94;
        for (;;) {
            ww = words.map(function (w) { return widthOf(c, w, 700, px); });
            var space = widthOf(c, ' ', 700, px), row = null;
            rows = [];
            words.forEach(function (w, k) {
                if (!row || row.w + space + ww[k] > maxW) rows.push(row = { ix: [], w: -space });
                row.ix.push(k);
                row.w += space + ww[k];
            });
            if ((rows.length <= 3 && rows.every(function (r) { return r.w <= maxW; }) && rows.length * px * 1.25 <= st.h) || px <= c.floor * 1.5) break;
            px = Math.max(c.floor * 1.5, px * 0.94);
        }
        var lh = px * 1.25, width = Math.max.apply(null, rows.map(function (r) { return r.w; })), height = rows.length * lh;
        return {
            box: { x: -width / 2, y: -height / 2, w: width, h: height }, labelPx: px,
            paint: function () {
                var space = widthOf(c, ' ', 700, px);
                rows.forEach(function (r, i) {
                    var x = -r.w / 2, y = -height / 2 + lh * (i + 0.5), order = rtl ? r.ix.slice().reverse() : r.ix;
                    order.forEach(function (k) {
                        var p = B.itemProgress(st, k), cx = x + ww[k] / 2;
                        x += ww[k] + space;
                        if (!(p > 0)) return;
                        if (k === hp.mark) {
                            ctx.globalAlpha = c.alpha * 0.5 * p;
                            ctx.fillStyle = c.col.accent;
                            ctx.fillRect(cx - ww[k] / 2 - px * 0.1, y + px * 0.02, (ww[k] + px * 0.2) * p, px * 0.46);
                        }
                        ctx.globalAlpha = c.alpha * p;
                        printLines(c, [words[k]], cx, y + (1 - p) * px * 0.35, px, 700, c.col.ink, 'center');
                    });
                });
            }
        };
    }

    // listPlan: up to four lines with an accent bullet, each sliding in from the left in order.
    function listPlan(c) {
        var st = c.st, rows = B.lines(st.block.text), n = rows.length, ctx = c.ctx;
        var lab = fitSet(c, rows, 600, st.w * 0.9 - c.base, 2, clamp(st.h / (n * 2), c.base, c.base * 1.8)), px = lab.px;
        var heights = lab.lines.map(function (l) { return l.length * px * 1.2 + px * 0.7; }), height = heights.reduce(function (a, b) { return a + b; }, 0);
        var width = Math.max.apply(null, lab.lines.map(function (l) { return Math.max.apply(null, l.map(function (s) { return widthOf(c, s, 600, px); })); })) + px * 0.9;
        return {
            box: { x: -width / 2, y: -height / 2, w: width, h: height }, labelPx: px,
            paint: function () {
                var y = -height / 2;
                lab.lines.forEach(function (lines, k) {
                    var p = B.itemProgress(st, k), mid = y + heights[k] / 2, dx = (1 - p) * px * 1.2;
                    y += heights[k];
                    if (!(p > 0)) return;
                    ctx.globalAlpha = c.alpha * p;
                    ctx.fillStyle = c.col.accent;
                    ctx.beginPath();
                    ctx.arc(-width / 2 + px * 0.2 + dx, mid, px * 0.2, 0, Math.PI * 2);
                    ctx.fill();
                    printLines(c, lines, -width / 2 + px * 0.9 + dx, mid, px, 600, c.col.ink, 'left');
                });
            }
        };
    }

    var PLANS = { icon: iconPlan, diagram: diagramPlan, chart: chartPlan, headline: headlinePlan, list: listPlan };

    // plan(ctx, st, rc, stage) = {box, labelPx, paint} of an evaluated block state, box in design pixels around the block centre at
    // scale 1 and labelPx in design pixels, or null for a kind without a painter. It draws nothing until paint() runs.
    function plan(ctx, st, rc, stage) {
        var make = st && st.block ? PLANS[st.block.kind] : null;
        return make ? make(context(ctx, st, rc, stage)) : null;
    }

    // panel: the rounded panel behind a block on a surface of its own: the content box plus padding, cut to the area.
    function paintPanel(c, box) {
        var st = c.st, ctx = c.ctx, pad = Math.max(16, 0.05 * Math.min(st.w, st.h));
        var x0 = Math.max(box.x - pad, -st.w / 2), y0 = Math.max(box.y - pad, -st.h / 2);
        var x1 = Math.min(box.x + box.w + pad, st.w / 2), y1 = Math.min(box.y + box.h + pad, st.h / 2);
        SM.text.roundRect(ctx, x0, y0, x1 - x0, y1 - y0, PANEL_RADIUS * Math.min(st.w, st.h));
        ctx.globalAlpha = c.alpha * PANEL_ALPHA;
        ctx.fillStyle = c.col.panel;
        ctx.fill();
        ctx.globalAlpha = c.alpha;
    }

    // draw(ctx, st, rc, stage, win): paints one live block state through the camera window win.
    function draw(ctx, st, rc, stage, win) {
        if (!st || !st.block || !(st.alpha > 0)) return;
        var c = context(ctx, st, rc, stage), pl = PLANS[st.block.kind] ? PLANS[st.block.kind](c) : null;
        if (!pl) return;
        var R = SM.rig, m = R.mul(stage.matrix(win), R.mul(R.translate(st.x, st.y), R.mul(R.rotate(st.rot), [st.scale, 0, 0, st.scale, 0, 0])));
        ctx.save();
        ctx.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]);
        ctx.globalAlpha = c.alpha;
        if (st.surface === 'panel') paintPanel(c, pl.box);
        pl.paint();
        ctx.restore();
    }

    // drawBoard(ctx, {variant, palette, W, H}): the board of env.board over the whole canvas: palette.bg by day, palette.fg by night,
    // with 900 faint dots of the other colour. The dots come from a seeded generator and sit at alpha 0.008, so no pixel moves by more
    // than 2 of 255 and an empty board still reads as one flat colour to the first-frame probe.
    function drawBoard(ctx, o) {
        var p = o.palette || {}, night = o.variant === 'night', W = o.W, H = o.H, random = SM.seed.random('board-grain');
        ctx.setTransform(1, 0, 0, 1, 0, 0);
        ctx.globalAlpha = 1;
        ctx.fillStyle = night ? (p.fg || '#1f2a44') : (p.bg || '#fff7e6');
        ctx.fillRect(0, 0, W, H);
        ctx.fillStyle = night ? (p.bg || '#fff7e6') : (p.fg || '#1f2a44');
        ctx.globalAlpha = 0.008;
        for (var i = 0; i < 900; i++) {
            var x = random() * W, y = random() * H, r = (0.6 + 1.0 * random()) * W / 1920;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, Math.PI * 2);
            ctx.fill();
        }
        ctx.globalAlpha = 1;
    }

    B.draw = draw;
    B.drawBoard = drawBoard;
    B.plan = plan;
})(typeof globalThis !== 'undefined' ? globalThis : self);
