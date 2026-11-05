// Storymaker player: speech and thought bubbles. layout measures the lines through SM.text and fixes the box size; place puts
// the box above an anchor and inside the safe area; draw paints the box (a tail for speech, three circles for a thought) and
// the text. A bubble fades over FADE_SECONDS (scene.createBubble); the rhythm never scales that. Classic script; publishes
// StorymakerCore.bubble.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var FADE_SECONDS = 0.12;
    // design pixels from the feet to the top of the head of a standing actor at depth scale 1: the rig height, twice ACTOR_REACH in beats.js
    var HEAD_REACH = 594;
    var TAIL_SHARE = 0.024;    // tail length, as a fraction of the canvas height
    var BORDER_SHARE = 0.002;  // outline width, as a fraction of the canvas height
    var WIDTH_SHARE = 0.6;     // widest text: 60 % of the safe width
    var WIDTH_EMS = 18;        // but at least 18 em, and never more than the safe width
    var DOT_AT = [0.3, 0.62, 0.92];
    var DOT_SIZE = [0.3, 0.2, 0.12];

    function between(v, lo, hi) { return Math.max(lo, Math.min(v, hi)); }

    // layout(ctx, lines, style, opts): opts = {canvasW, canvasH, safe, family}. lines is a list of strings; each stays its own
    // paragraph. The text layout of SM.text may be a shared memo entry, so nothing here changes it.
    function layout(ctx, lines, style, opts) {
        var W = opts.canvasW, H = opts.canvasH, safe = opts.safe || SM.stage.DEFAULT_SAFE;
        var px = (style.size > 0 ? style.size : 0.05) * H;
        var padGuess = Math.round(style.box && style.box.pad > 0 ? style.box.pad * H : 0.4 * px);
        var safeW = W * safe.w / 100;
        var maxWidth = Math.min(safeW, Math.max(WIDTH_SHARE * safeW, WIDTH_EMS * px)) - 2 * padGuess;
        var text = SM.text.layout(ctx, lines.join('\n'), style, { canvasW: W, canvasH: H, short: Math.min(W, H), maxWidth: maxWidth, family: opts.family });
        var pad = text.pad > 0 ? text.pad : padGuess, tail = Math.max(6, Math.round(TAIL_SHARE * H));
        return {
            text: text, style: style, inner: { color: style.color, align: 'center' },
            w: text.width + 2 * pad, h: text.height + 2 * pad, radius: text.radius > 0 ? text.radius : Math.round(0.3 * px),
            tail: tail, half: Math.max(3, Math.round(0.4 * tail)), border: Math.max(1, Math.round(BORDER_SHARE * H)),
            safe: { left: W * safe.x / 100, top: H * safe.y / 100, right: W * (safe.x + safe.w) / 100, bottom: H * (safe.y + safe.h) / 100 }
        };
    }

    // place(lay, anchor) = {x, y, w, h, tip}: the box above the anchor, moved as little as needed to stay in the safe area.
    function place(lay, anchor) {
        var s = lay.safe;
        var x = between(anchor.x - lay.w / 2, s.left, s.right - lay.w);
        var y = between(anchor.y - lay.tail - lay.h, s.top, s.bottom - lay.tail - lay.h);
        var bottom = y + lay.h;
        var tip = { x: between(anchor.x, x, x + lay.w), y: Math.min(s.bottom, between(anchor.y, bottom + lay.tail / 2, bottom + lay.tail * 2)) };
        return { x: x, y: y, w: lay.w, h: lay.h, tip: tip };
    }

    // the rounded box and the tail as one path, so the outline has no line across the tail
    function speechPath(ctx, b, lay) {
        var rr = Math.min(lay.radius, b.w / 2, b.h / 2), bottom = b.y + b.h, half = lay.half;
        var bx = between(b.tip.x, b.x + rr + half, b.x + b.w - rr - half);
        ctx.beginPath();
        ctx.moveTo(b.x + rr, b.y);
        ctx.lineTo(b.x + b.w - rr, b.y);
        ctx.arcTo(b.x + b.w, b.y, b.x + b.w, b.y + rr, rr);
        ctx.lineTo(b.x + b.w, bottom - rr);
        ctx.arcTo(b.x + b.w, bottom, b.x + b.w - rr, bottom, rr);
        ctx.lineTo(bx + half, bottom);
        ctx.lineTo(b.tip.x, b.tip.y);
        ctx.lineTo(bx - half, bottom);
        ctx.lineTo(b.x + rr, bottom);
        ctx.arcTo(b.x, bottom, b.x, bottom - rr, rr);
        ctx.lineTo(b.x, b.y + rr);
        ctx.arcTo(b.x, b.y, b.x + rr, b.y, rr);
        ctx.closePath();
    }

    function thoughtCircles(ctx, b, lay) {
        var bottom = b.y + b.h, from = between(b.tip.x, b.x + lay.radius, b.x + b.w - lay.radius);
        for (var i = 0; i < DOT_AT.length; i++) {
            ctx.beginPath();
            ctx.arc(from + (b.tip.x - from) * DOT_AT[i], bottom + (b.tip.y - bottom) * DOT_AT[i], Math.max(1, DOT_SIZE[i] * lay.tail), 0, Math.PI * 2);
            ctx.fill();
            ctx.stroke();
        }
    }

    // draw(ctx, lay, anchor, kind, resolve, alpha): kind is speech or thought; anchor is in canvas pixels. Returns the box, or null.
    function draw(ctx, lay, anchor, kind, resolve, alpha) {
        if (!(alpha > 0)) return null;
        var box = place(lay, anchor), style = lay.style, a = alpha > 1 ? 1 : alpha;
        ctx.globalAlpha = a;
        ctx.fillStyle = resolve(style.box && style.box.fill || '#ffffff');
        ctx.strokeStyle = resolve(style.color || '$text');
        ctx.lineWidth = lay.border;
        ctx.lineJoin = 'round';
        if (kind === 'thought') {
            thoughtCircles(ctx, box, lay);
            SM.text.roundRect(ctx, box.x, box.y, box.w, box.h, lay.radius);
        } else speechPath(ctx, box, lay);
        ctx.fill();
        ctx.stroke();
        SM.text.draw(ctx, lay.text, lay.inner, box.x + box.w / 2, box.y + box.h / 2, resolve, a);
        return box;
    }

    // anchorOf(stage, st, win) = the head point of the actor state st in canvas pixels, under the camera window win.
    function anchorOf(stage, st, win) {
        var depth = stage.depthScale(st.y, st.depth);
        return stage.toCanvas(st.x, st.y - HEAD_REACH * depth * st.scale, win);
    }

    SM.bubble = { FADE_SECONDS: FADE_SECONDS, layout: layout, place: place, draw: draw, anchorOf: anchorOf };
})(typeof globalThis !== 'undefined' ? globalThis : self);
