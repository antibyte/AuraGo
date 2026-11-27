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
            panel: p.bg || '#fff7e6'
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
        var px = px0, out, words = String(text).split(/s+/);
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
            var tip = b.y - lw * 3 - lw * 2 * Math.sin(4 * Math.PI * mark.u);
            ctx.fillStyle = c.col.accent;
            ctx.beginPath();
            ctx.moveTo(cx, tip);
            ctx.lineTo(cx - lw * 3, tip - lw * 5);
            ctx.lineTo(cx + lw * 3, tip - lw * 5);
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

    var PLANS = { icon: iconPlan, diagram: diagramPlan };

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

    B.draw = draw;
    B.plan = plan;
})(typeof globalThis !== 'undefined' ? globalThis : self);
