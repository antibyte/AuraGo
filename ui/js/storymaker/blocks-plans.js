// Storymaker player: block plans of the pie chart and the list. They sit in their own part because blocks-draw.js is at its line
// budget. Each plan fits the area of the block: the pie picks the donut size and the legend position from the free width and
// height, the list shrinks its font until every line fits. Same contract as the plans of blocks-draw.js: plan(c, f) returns
// {box, labelPx, paint}. Classic script; adds piePlan and listPlan to StorymakerCore.blocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var B = SM.blocks, K = B.kit;

    var ROW = 1.6;   // legend row height in label heights

    // pieGeometry(c, rows, W, H) = {px, R, side, rw}: the legend font, the donut radius and the legend position (beside the donut or
    // below it) for a W x H area. The font shrinks, down to the label floor, until the donut gets a third of the short side.
    function pieGeometry(c, rows, W, H) {
        var px = c.base, short = Math.min(W, H), n = rows.length, g;
        for (;;) {
            var legH = n * ROW * px, rw = 0, gapB = 0.4 * px, gapS = 0.9 * px;
            rows.forEach(function (s) { rw = Math.max(rw, K.widthOf(c, s, 600, px)); });
            rw += px * 1.3;
            var below = rw <= W ? Math.min(W / 2, (H - legH - gapB) / 2) : 0, beside = legH <= H ? Math.min(H / 2, (W - rw - gapS) / 2) : 0;
            g = { px: px, rw: rw, legH: legH, gapB: gapB, gapS: gapS, side: beside > below, R: Math.max(beside, below) };
            if (g.R >= 0.3 * short || px <= c.floor) break;
            px = Math.max(c.floor, px * 0.94);
        }
        g.R = Math.max(g.R, 0.1 * short);
        return g;
    }

    // piePlan: a donut with a legend; the slices sweep clockwise from the top while p grows.
    function piePlan(c, f) {
        var st = c.st, ch = st.block.chart, vals = ch.values, n = vals.length, p = B.itemProgress(st, 0), ctx = c.ctx, sum = 0;
        vals.forEach(function (v) { sum += v; });
        var rows = ch.labels.map(function (l, k) { return String(l) + ' ' + Math.round(100 * vals[k] / sum) + '%'; });
        var W = f.x1 - f.x0, H = f.y1 - f.top, g = pieGeometry(c, rows, W, H), R = g.R, r = R * 0.58;
        var lab = K.fitSet(c, rows, 600, g.side ? W - R * 2 - g.gapS : W, 1, g.px), rowH = ROW * lab.px;
        // the donut and its legend are centred as one group
        var group = g.side ? 2 * R + g.gapS + g.rw : W, cx, cy, legendX, top;
        if (g.side) {
            cx = f.x0 + (W - group) / 2 + R;
            cy = (f.top + f.y1) / 2;
            legendX = cx + R + g.gapS;
            top = cy - n * rowH / 2;
        } else {
            cy = f.top + (H - (2 * R + g.gapB + n * rowH)) / 2 + R;
            cx = (f.x0 + f.x1) / 2;
            legendX = (f.x0 + f.x1 - g.rw) / 2;
            top = cy + R + g.gapB;
        }
        var tint = function (k) { return [c.col.accent, c.col.accent2, c.col.ink][k % 3]; };
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
                    var show = K.clamp((p - 0.3) / 0.7, 0, 1), y = top + rowH * (k + 0.5);
                    ctx.globalAlpha = c.alpha * show * (k < 3 ? 1 : 0.55);
                    ctx.fillStyle = tint(k);
                    SM.text.roundRect(ctx, legendX, y - lab.px * 0.4, lab.px * 0.8, lab.px * 0.8, lab.px * 0.2);
                    ctx.fill();
                    ctx.globalAlpha = c.alpha * show;
                    K.printLines(c, lab.lines[k], legendX + lab.px * 1.3, y, lab.px, 600, c.col.ink, 'left');
                    a0 = a1;
                });
            }
        };
    }

    // listPlan: up to four lines with an accent bullet, each sliding in from the left in order. The font shrinks, down to the label
    // floor, until the lines fit the height of the area.
    function listPlan(c) {
        var st = c.st, rows = B.lines(st.block.text), n = rows.length, ctx = c.ctx, maxW = st.w * 0.9 - c.base;
        var px0 = K.clamp(st.h / (n * 2), c.floor, c.base * 1.8), lab, px, heights, height;
        for (;;) {
            lab = K.fitSet(c, rows, 600, maxW, 2, px0);
            px = lab.px;
            heights = lab.lines.map(function (l) { return l.length * px * 1.2 + px * 0.7; });
            height = heights.reduce(function (a, b) { return a + b; }, 0);
            if (height <= st.h || px <= c.floor) break;
            px0 = Math.max(c.floor, px * 0.94);
        }
        var width = Math.max.apply(null, lab.lines.map(function (l) { return Math.max.apply(null, l.map(function (s) { return K.widthOf(c, s, 600, px); })); })) + px * 0.9;
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
                    K.printLines(c, lines, -width / 2 + px * 0.9 + dx, mid, px, 600, c.col.ink, 'left');
                });
            }
        };
    }

    B.piePlan = piePlan;
    B.listPlan = listPlan;
})(typeof globalThis !== 'undefined' ? globalThis : self);
