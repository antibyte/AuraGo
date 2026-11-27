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
        var px = px0, out;
        for (;;) {
            setFont(c, weight, px);
            out = SM.text.wrap(function (s) { return c.ctx.measureText(s).width; }, text, maxW, maxLines);
            if (!out.truncated || px <= c.floor) break;
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

    var PLANS = { icon: iconPlan };

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
