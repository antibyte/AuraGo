// Storymaker player: text. Text styles come from the catalog (font, size as a fraction of the canvas height, weight,
// colour, optional box, alignment, max_lines). Wrapping measures the real font through measureText and ends an
// over-long text with an ellipsis after max_lines. Text is drawn in screen space on top of the scene. Classic script;
// publishes StorymakerCore.text. A text style's box.pad and box.radius are fractions of the canvas height, like size.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var ELLIPSIS = String.fromCharCode(0x2026);
    var LINE_HEIGHT = 1.22;
    // Trimmed before an ellipsis, besides ASCII punctuation: Arabic and CJK punctuation and the ellipsis itself.
    var TRIM_CODES = [0x060C, 0x061B, 0x061F, 0x06D4, 0x2026, 0x3001, 0x3002, 0xFF01, 0xFF0C, 0xFF1A, 0xFF1B, 0xFF1F];
    var SHRINK_STEP = 0.94, SHRINK_FLOOR = 0.45, FONT_FLOOR = 0.026, BALANCE_STEPS = 8, MEMO_LIMIT = 512;
    var memo = Object.create(null), memoSize = 0;

    function fontString(weight, px, family) {
        // family is one name or a fallback list. Each name is quoted, and sans-serif comes last.
        var names = (Array.isArray(family) ? family : [family]).filter(Boolean);
        if (!names.length) names = ['sans-serif'];
        var list = names.map(function (name) { return '"' + String(name).replace(/"/g, '') + '"'; }).join(', ');
        return String(weight || 400) + ' ' + px + 'px ' + list + ', sans-serif';
    }

    function trimEnd(line) {
        var end = line.length;
        while (end > 0 && (TRIM_CODES.indexOf(line.charCodeAt(end - 1)) >= 0 || /[\s.,;:!?-]/.test(line.charAt(end - 1)))) end--;
        return line.slice(0, end);
    }

    // dropLast(text): text without its last code point, so a surrogate pair never splits.
    function dropLast(text) {
        var cut = text.length - 1, low = text.charCodeAt(cut), high = text.charCodeAt(cut - 1);
        if (low >= 0xDC00 && low <= 0xDFFF && high >= 0xD800 && high <= 0xDBFF) cut--;
        return text.slice(0, cut);
    }

    function fitEllipsis(measure, line, maxWidth) {
        var text = trimEnd(line);
        while (text.length > 0 && measure(text + ELLIPSIS) > maxWidth) text = trimEnd(dropLast(text));
        return text + ELLIPSIS;
    }

    // breakWord(measure, word, maxWidth): splits a word that is wider than a line into pieces that fit. It iterates code points
    // and breaks only where SM.ranges.canBreakAfter allows, at the last allowed point before the line would overflow.
    function breakWord(measure, word, maxWidth) {
        var pieces = [], current = '', breakAt = 0, prev = -1;
        for (var i = 0; i < word.length;) {
            var cp = word.codePointAt(i), ch = String.fromCodePoint(cp);
            i += ch.length;
            if (current && SM.ranges.canBreakAfter(prev, cp)) breakAt = current.length;
            if (breakAt > 0 && measure(current + ch) > maxWidth) {
                pieces.push(current.slice(0, breakAt));
                current = current.slice(breakAt);
                breakAt = 0;
            }
            current += ch;
            prev = cp;
        }
        if (current) pieces.push(current);
        return pieces;
    }

    // cjkParts(word): a token split at every SM.ranges.cjkBreak. A token without CJK stays whole.
    function cjkParts(word) {
        var parts = [], current = '', prev = -1;
        for (var i = 0; i < word.length;) {
            var cp = word.codePointAt(i), ch = String.fromCodePoint(cp);
            i += ch.length;
            if (current && SM.ranges.cjkBreak(prev, cp)) { parts.push(current); current = ''; }
            current += ch;
            prev = cp;
        }
        if (current) parts.push(current);
        return parts;
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
                var parts = [];
                cjkParts(word).forEach(function (piece) {
                    (measure(piece) > maxWidth ? breakWord(measure, piece, maxWidth) : [piece]).forEach(function (p) { parts.push(p); });
                });
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

    // rejoin(lines): the lines as one text, with a space only where the break replaced one. A break at SM.ranges.cjkBreak had none.
    // The lines cannot show a real space between two CJK characters at a break, so that space goes.
    function rejoin(lines) {
        return lines.reduce(function (text, line) {
            var last = text.slice(dropLast(text).length).codePointAt(0);
            return text + (SM.ranges.cjkBreak(last, line.codePointAt(0)) ? '' : ' ') + line;
        });
    }

    // balance(measure, lines, maxWidth, maxLines): the same text, re-wrapped at the narrowest width (by bisection) that does not
    // add a line. A word never breaks here, because the width stays at least as wide as the widest word.
    function balance(measure, lines, maxWidth, maxLines) {
        if (lines.length < 2) return lines;
        var joined = rejoin(lines), best = lines, hi = maxWidth, lo = 0;
        joined.split(' ').forEach(function (word) {
            cjkParts(word).forEach(function (part) { var w = measure(part); if (w > lo) lo = w; });
        });
        if (lo > maxWidth) return lines;
        for (var i = 0; i < BALANCE_STEPS; i++) {
            var mid = (lo + hi) / 2, tried = wrap(measure, joined, mid, maxLines);
            if (!tried.truncated && tried.lines.length <= lines.length) { best = tried.lines; hi = mid; } else lo = mid;
        }
        return best;
    }

    // compute(ctx, text, style, opts): opts = {canvasH, maxWidth, family, short?}; short is the short side of the canvas, canvasH
    // when absent. Sets ctx.font to measure. px steps by 0.94 while the wrap truncates, down to max(0.45 x px0, 0.026 x short), and
    // the lines are then balanced. shrunk says px moved; overflow says the text is still truncated. Returns the lines
    // and the box in canvas pixels.
    function compute(ctx, text, style, opts) {
        var px0 = Math.max(8, Math.round((style.size > 0 ? style.size : 0.05) * opts.canvasH));
        var floor = Math.max(SHRINK_FLOOR * px0, FONT_FLOOR * (opts.short > 0 ? opts.short : opts.canvasH));
        var px = px0;
        ctx.font = fontString(style.weight, px, opts.family);
        var measure = function (s) { return ctx.measureText(s).width; };
        var wrapped = wrap(measure, text, opts.maxWidth, style.max_lines);
        while (wrapped.truncated && px > floor) {
            px = Math.max(floor, px * SHRINK_STEP);
            ctx.font = fontString(style.weight, px, opts.family);
            wrapped = wrap(measure, text, opts.maxWidth, style.max_lines);
        }
        // an explicit line break stays, and a truncated text has lost words that balance cannot bring back
        var lines = wrapped.truncated || /\n/.test(String(text)) ? wrapped.lines : balance(measure, wrapped.lines, opts.maxWidth, style.max_lines);
        var width = 0;
        lines.forEach(function (l) { var w = measure(l); if (w > width) width = w; });
        // box.pad and box.radius are fractions of the canvas height; a box without them gets 0.4 em padding and 0.3 em corners
        var pad = 0, radius = 0;
        if (style.box) {
            pad = Math.round(style.box.pad > 0 ? style.box.pad * opts.canvasH : 0.4 * px);
            radius = Math.round(style.box.radius >= 0 ? style.box.radius * opts.canvasH : 0.3 * px);
        }
        return {
            lines: lines, truncated: wrapped.truncated, shrunk: px < px0, overflow: wrapped.truncated, px: px, lineH: px * LINE_HEIGHT,
            width: width, height: lines.length * px * LINE_HEIGHT, pad: pad, radius: radius, font: ctx.font
        };
    }

    // layout(ctx, text, style, opts): compute, memoised when opts names the style (opts.styleName, opts.canvasW). The key holds
    // maxWidth because the anchor moves during a slide or a move, and frame n must stay a pure function of n. A hit sets the same
    // font a miss leaves behind, so the canvas state does not depend on the history.
    function layout(ctx, text, style, opts) {
        if (!opts.styleName) return compute(ctx, text, style, opts);
        var key = JSON.stringify([String(text), opts.styleName, opts.canvasW, opts.canvasH, opts.family, opts.maxWidth]);
        var hit = memo[key];
        if (hit) { ctx.font = hit.font; return hit; }
        if (memoSize >= MEMO_LIMIT) { memo = Object.create(null); memoSize = 0; }
        memoSize++;
        return (memo[key] = compute(ctx, text, style, opts));
    }

    // layoutDir(ctx, text, style, opts): layout with the direction of the text, from its first strong character. Setting it first
    // means canvas state never leaks from one text to the next, so workers cannot diverge. style.align stays physical for RTL.
    function layoutDir(ctx, text, style, opts) {
        var dir = SM.ranges.dirOf(String(text === undefined || text === null ? '' : text));
        ctx.direction = dir;
        var lay = layout(ctx, text, style, opts);
        lay.dir = dir;
        return lay;
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
        ctx.direction = lay.dir || 'ltr';
        var align = style.align === 'left' || style.align === 'right' ? style.align : 'center';
        var left = align === 'left' ? x : (align === 'right' ? x - lay.width : x - lay.width / 2);
        ctx.globalAlpha = alpha > 1 ? 1 : alpha;
        if (style.box) {
            ctx.fillStyle = resolve(style.box.fill || '$bg2');
            roundRect(ctx, left - lay.pad, y - lay.height / 2 - lay.pad, lay.width + lay.pad * 2, lay.height + lay.pad * 2, lay.radius);
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

    SM.text = { fontString: fontString, wrap: wrap, balance: balance, breakWord: breakWord, layout: layoutDir, draw: draw, maxWidthFor: maxWidthFor, roundRect: roundRect };
})(typeof globalThis !== 'undefined' ? globalThis : self);
