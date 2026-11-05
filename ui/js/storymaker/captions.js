// Storymaker player: captions. A block's text is split into captions of at most 2 lines of 42 columns (East Asian wide
// characters count 2), and their times come from the same cue map as the cues. A caption lasts at least 1.0 s, except where
// the film end cuts it. Captions are always in the report; style.captions only decides drawing. Classic script; publishes
// StorymakerCore.captions.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var MAX_LINES = 2, MAX_WIDTH = 42, MIN_MS = 1000;

    // units(text) = [{a, end, b}]: the unbreakable runs in UTF-16 offsets. [a, end) is the visible run, and b also holds the
    // whitespace after it. A break lies at whitespace or where cjkBreak allows one.
    function units(text) {
        var R = SM.ranges, out = [], a = 0, i = 0, prev = -1;
        while (i < text.length) {
            var cp = text.codePointAt(i), size = cp > 0xFFFF ? 2 : 1;
            if (R.isSpace(cp)) {
                var j = i + 1;
                while (j < text.length && R.isSpace(text.codePointAt(j))) j++;
                if (i > a) out.push({ a: a, end: i, b: j });
                a = j;
                i = j;
                prev = -1;
                continue;
            }
            if (prev >= 0 && R.cjkBreak(prev, cp)) {
                out.push({ a: a, end: i, b: i });
                a = i;
            }
            prev = cp;
            i += size;
        }
        if (i > a) out.push({ a: a, end: i, b: i });
        return out;
    }

    // hardBreak(text, u) = the unit cut into runs of at most MAX_WIDTH columns, where canBreakAfter allows. The whitespace stays
    // on the last run.
    function hardBreak(text, u) {
        var R = SM.ranges, out = [], a = u.a, w = 0, i = u.a, prev = -1;
        while (i < u.end) {
            var cp = text.codePointAt(i), size = cp > 0xFFFF ? 2 : 1, cw = R.isWide(cp) ? 2 : 1;
            if (w + cw > MAX_WIDTH && prev >= 0 && R.canBreakAfter(prev, cp)) {
                out.push({ a: a, end: i, b: i });
                a = i;
                w = 0;
            }
            w += cw;
            prev = cp;
            i += size;
        }
        out.push({ a: a, end: u.end, b: u.b });
        return out;
    }

    // split(plain) = [{a, b, lines}]: offsets in UTF-16, b exclusive, with the trailing whitespace inside the range.
    function split(plain) {
        var R = SM.ranges, text = String(plain === undefined || plain === null ? '' : plain), list = [], out = [], cur = null, lineA = 0;
        units(text).forEach(function (u) {
            if (R.width(text.slice(u.a, u.end)) > MAX_WIDTH) list = list.concat(hardBreak(text, u)); else list.push(u);
        });
        list.forEach(function (u) {
            if (cur && R.width(text.slice(lineA, u.end)) <= MAX_WIDTH) {
                cur.lines[cur.lines.length - 1] = text.slice(lineA, u.end);
                cur.b = u.b;
                return;
            }
            if (cur && cur.lines.length < MAX_LINES) cur.lines.push(text.slice(u.a, u.end));
            else {
                cur = { a: u.a, b: u.b, lines: [text.slice(u.a, u.end)] };
                out.push(cur);
            }
            lineA = u.a;
            cur.b = u.b;
        });
        return out;
    }

    // forBlock(plain, block, startMs, sceneId) = the captions of one block, in film-global ms.
    function forBlock(plain, block, startMs, sceneId) {
        var parts = split(plain), map = SM.cuemap.cueMap, starts = [];
        parts.forEach(function (p, i) {
            var s = startMs + Math.round(map(block, p.a));
            if (i > 0 && s < starts[i - 1] + MIN_MS) s = starts[i - 1] + MIN_MS;
            starts.push(s);
        });
        return parts.map(function (p, i) {
            var end = Math.max(startMs + Math.round(map(block, p.b)), starts[i] + MIN_MS);
            if (i + 1 < parts.length && end > starts[i + 1]) end = starts[i + 1];
            return { start_ms: starts[i], end_ms: end, lines: p.lines, scene_id: sceneId, block_id: block.id };
        });
    }

    // settle(all, filmMs) = the captions of all blocks, sorted. Each end is lowered to the next start and to the film end, and a
    // caption left with no length is dropped.
    function settle(all, filmMs) {
        var sorted = all.slice().sort(function (x, y) { return x.start_ms - y.start_ms || x.end_ms - y.end_ms; }), out = [];
        sorted.forEach(function (c, i) {
            var end = Math.min(c.end_ms, filmMs);
            if (i + 1 < sorted.length && sorted[i + 1].start_ms < end) end = sorted[i + 1].start_ms;
            if (c.start_ms < end) out.push({ start_ms: c.start_ms, end_ms: end, lines: c.lines, scene_id: c.scene_id, block_id: c.block_id });
        });
        return out;
    }

    // at(list, ms) = the caption active at ms in a settled list, or null. Binary search.
    function at(list, ms) {
        var lo = 0, hi = list.length - 1;
        while (lo <= hi) {
            var mid = (lo + hi) >> 1, c = list[mid];
            if (ms < c.start_ms) hi = mid - 1;
            else if (ms >= c.end_ms) lo = mid + 1;
            else return c;
        }
        return null;
    }

    SM.captions = { MAX_LINES: MAX_LINES, MAX_WIDTH: MAX_WIDTH, MIN_MS: MIN_MS, split: split, forBlock: forBlock, settle: settle, at: at };
})(typeof globalThis !== 'undefined' ? globalThis : self);
