// Storymaker player: text. Text styles come from the catalog (font, size as a fraction of the canvas height, weight,
// colour, optional box, alignment, max_lines). Wrapping measures the real font through measureText and ends an
// over-long text with an ellipsis after max_lines. Text is drawn in screen space on top of the scene. Classic script;
// publishes StorymakerCore.text.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var ELLIPSIS = String.fromCharCode(0x2026);
    var LINE_HEIGHT = 1.22;

    function fontString(weight, px, family) {
        return String(weight || 400) + ' ' + px + 'px "' + String(family || 'sans-serif').replace(/"/g, '') + '", sans-serif';
    }

    function fitEllipsis(measure, line, maxWidth) {
        var text = line.replace(/[\s.,;:!?-]+$/, '');
        while (text.length > 0 && measure(text + ELLIPSIS) > maxWidth) text = text.slice(0, -1);
        return text + ELLIPSIS;
    }

    // breakWord(measure, word, maxWidth): splits a word that is wider than a line into pieces that fit.
    function breakWord(measure, word, maxWidth) {
        var pieces = [], current = '';
        for (var i = 0; i < word.length; i++) {
            if (current && measure(current + word[i]) > maxWidth) { pieces.push(current); current = word[i]; } else current += word[i];
        }
        if (current) pieces.push(current);
        return pieces;
    }

    // wrap(measure, text, maxWidth, maxLines) = {lines, truncated}. measure(str) returns the width at the target font.
    function wrap(measure, text, maxWidth, maxLines) {
        var limit = maxLines > 0 ? maxLines : 1000;
        var paragraphs = String(text === undefined || text === null ? '' : text).split(/\r?\n/);
        var lines = [];
        paragraphs.forEach(function (paragraph) {
            var words = paragraph.split(/\s+/).filter(Boolean), line = '';
            if (!words.length) { lines.push(''); return; }
            words.forEach(function (word) {
                var parts = measure(word) > maxWidth ? breakWord(measure, word, maxWidth) : [word];
                parts.forEach(function (part, k) {
                    var joined = line && k === 0 ? line + ' ' + part : (line ? line + part : part);
                    if (!line || measure(joined) <= maxWidth) { line = joined; return; }
                    lines.push(line);
                    line = part;
                });
            });
            if (line) lines.push(line);
        });
        if (lines.length <= limit) return { lines: lines, truncated: false };
        var kept = lines.slice(0, limit);
        kept[limit - 1] = fitEllipsis(measure, kept[limit - 1] + ' ' + lines[limit], maxWidth);
        return { lines: kept, truncated: true };
    }

    // layout(ctx, text, style, opts): opts = {canvasH, maxWidth, family}. Sets ctx.font to measure. Returns the lines
    // and the box in canvas pixels.
    function layout(ctx, text, style, opts) {
        var px = Math.max(8, Math.round((style.size > 0 ? style.size : 0.05) * opts.canvasH));
        ctx.font = fontString(style.weight, px, opts.family);
        var measure = function (s) { return ctx.measureText(s).width; };
        var wrapped = wrap(measure, text, opts.maxWidth, style.max_lines);
        var width = 0;
        wrapped.lines.forEach(function (l) { var w = measure(l); if (w > width) width = w; });
        var pad = style.box ? Math.round((style.box.pad > 0 ? style.box.pad : 0.4) * px) : 0;
        return {
            lines: wrapped.lines, truncated: wrapped.truncated, px: px, lineH: px * LINE_HEIGHT,
            width: width, height: wrapped.lines.length * px * LINE_HEIGHT, pad: pad, font: ctx.font
        };
    }

    function roundRect(ctx, x, y, w, h, r) {
        var rr = Math.min(r, w / 2, h / 2);
        ctx.beginPath();
        ctx.moveTo(x + rr, y);
        ctx.lineTo(x + w - rr, y);
        ctx.arcTo(x + w, y, x + w, y + rr, rr);
        ctx.lineTo(x + w, y + h - rr);
        ctx.arcTo(x + w, y + h, x + w - rr, y + h, rr);
        ctx.lineTo(x + rr, y + h);
        ctx.arcTo(x, y + h, x, y + h - rr, rr);
        ctx.lineTo(x, y + rr);
        ctx.arcTo(x, y, x + rr, y, rr);
        ctx.closePath();
    }

    // draw(ctx, lay, style, x, y, resolve, alpha): x is the anchor (centre, left or right by style.align), y the vertical
    // centre of the block, in canvas pixels.
    function draw(ctx, lay, style, x, y, resolve, alpha) {
        if (!(alpha > 0)) return;
        var align = style.align === 'left' || style.align === 'right' ? style.align : 'center';
        var left = align === 'left' ? x : (align === 'right' ? x - lay.width : x - lay.width / 2);
        ctx.globalAlpha = alpha > 1 ? 1 : alpha;
        if (style.box) {
            var radius = Math.round((style.box.radius >= 0 ? style.box.radius : 0.3) * lay.px);
            ctx.fillStyle = resolve(style.box.fill || '$bg2');
            roundRect(ctx, left - lay.pad, y - lay.height / 2 - lay.pad, lay.width + lay.pad * 2, lay.height + lay.pad * 2, radius);
            ctx.fill();
        }
        ctx.font = lay.font;
        ctx.fillStyle = resolve(style.color || '$text');
        ctx.textBaseline = 'middle';
        ctx.textAlign = align;
        var ax = align === 'left' ? left : (align === 'right' ? left + lay.width : left + lay.width / 2);
        for (var i = 0; i < lay.lines.length; i++) {
            ctx.fillText(lay.lines[i], ax, y - lay.height / 2 + lay.lineH * (i + 0.5));
        }
    }

    // maxWidthFor(canvasW, safe, x, align): the widest line that keeps the text inside the safe area around the anchor.
    function maxWidthFor(canvasW, safe, x, align) {
        var l = canvasW * safe.x / 100, r = canvasW * (safe.x + safe.w) / 100, full = r - l, w;
        if (align === 'left') w = r - x;
        else if (align === 'right') w = x - l;
        else w = 2 * Math.min(x - l, r - x);
        return Math.max(full * 0.25, Math.min(full, w));
    }

    SM.text = { fontString: fontString, wrap: wrap, layout: layout, draw: draw, maxWidthFor: maxWidthFor, roundRect: roundRect };
})(typeof globalThis !== 'undefined' ? globalThis : self);
