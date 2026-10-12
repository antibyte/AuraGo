// CYD Creator designer geometry: integer rects, snapping, alignment guides, resize handles,
// align/distribute maths, zoom and hit testing. Pure functions in device pixels ("world"
// coordinates of the CYD screen); the canvas multiplies by an integer zoom. Ideas forked from
// easydrag-geometry.js (rect ops, fit), not shared with it.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};

    const ZOOM_MIN = 1;
    const ZOOM_MAX = 4;
    const SNAP_STEPS = [0, 4, 8];
    const GUIDE_RADIUS = 4; // default snap radius in device px when the caller passes none; the canvas passes its own (screen px / zoom)
    const MIN_SIZE = 4;
    const MAX_SIDE = 2048; // largest widget side the server accepts (maxWidgetSide, internal/cydcreator/limits.go)
    const HANDLES = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w'];

    function int(v) { return Math.round(Number(v) || 0); }
    function rect(x, y, w, h) { return { x: int(x), y: int(y), w: int(w), h: int(h) }; }
    // edges builds a rect from rounded edges, so rounding never moves the opposite edge.
    function edges(x1, y1, x2, y2) { const x = int(x1), y = int(y1); return { x, y, w: int(x2) - x, h: int(y2) - y }; }
    function clampZoom(z) { const n = Math.round(Number(z)); return Number.isFinite(n) ? Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, n)) : ZOOM_MIN; }
    function snapStep(v) { const n = Number(v); return SNAP_STEPS.includes(n) ? n : 0; }

    // fitZoom is the largest integer zoom (1..4) at which a frame of size fits into the viewport.
    function fitZoom(size, viewport, padding) {
        const pad = padding == null ? 24 : padding;
        const zx = Math.floor((viewport.w - pad * 2) / Math.max(1, size.w));
        const zy = Math.floor((viewport.h - pad * 2) / Math.max(1, size.h));
        return clampZoom(Math.min(zx, zy));
    }

    // screenSize mirrors Go Board.ScreenSize: the native size, swapped when the orientation's
    // landscape-ness differs from the panel's native orientation.
    function screenSize(board, orientation) {
        const d = (board && board.display) || {};
        const w = int(d.width) || 320;
        const h = int(d.height) || 240;
        const landscape = String(orientation || 'landscape').startsWith('landscape');
        const nativeLandscape = (d.native_orientation || (w >= h ? 'landscape' : 'portrait')) === 'landscape';
        return landscape === nativeLandscape ? { w, h } : { w: h, h: w };
    }

    function snapValue(v, step) { return step > 0 ? Math.round(v / step) * step : Math.round(v); }
    function right(r) { return r.x + r.w; }
    function bottom(r) { return r.y + r.h; }
    function contains(r, p) { return p.x >= r.x && p.x < r.x + r.w && p.y >= r.y && p.y < r.y + r.h; }
    function intersects(a, b) { return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y; }
    function normalizeRect(a, b) { return { x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), w: Math.abs(b.x - a.x), h: Math.abs(b.y - a.y) }; }
    function translate(r, dx, dy) { return { x: r.x + dx, y: r.y + dy, w: r.w, h: r.h }; }
    function insideAll(outer, inner) { return inner.x >= outer.x && inner.y >= outer.y && right(inner) <= right(outer) && bottom(inner) <= bottom(outer); }

    function bounds(rects) {
        if (!rects || !rects.length) return null;
        let x1 = Infinity, y1 = Infinity, x2 = -Infinity, y2 = -Infinity;
        rects.forEach(r => { x1 = Math.min(x1, r.x); y1 = Math.min(y1, r.y); x2 = Math.max(x2, right(r)); y2 = Math.max(y2, bottom(r)); });
        return { x: x1, y: y1, w: x2 - x1, h: y2 - y1 };
    }

    // lines lists the snap lines of a rect: edges and centre per axis.
    function lines(r) {
        return { x: [r.x, r.x + Math.round(r.w / 2), right(r)], y: [r.y, r.y + Math.round(r.h / 2), bottom(r)] };
    }

    // guideTargets collects the lines a moving rect may snap to: every sibling's edges and centres
    // plus the frame (screen or container content box) edges and centre.
    function guideTargets(siblings, frame) {
        const out = { x: [], y: [] };
        (siblings || []).concat(frame ? [frame] : []).forEach(r => {
            const l = lines(r);
            out.x.push(...l.x);
            out.y.push(...l.y);
        });
        out.x = Array.from(new Set(out.x)).sort((a, b) => a - b);
        out.y = Array.from(new Set(out.y)).sort((a, b) => a - b);
        return out;
    }

    // nearest finds the target line closest to any of the candidate values within radius.
    // It returns {delta, line} (delta moves the candidate onto the line) or null.
    function nearest(candidates, targets, radius) {
        let best = null;
        candidates.forEach(c => targets.forEach(line => {
            const d = line - c;
            if (Math.abs(d) <= radius && (!best || Math.abs(d) < Math.abs(best.delta))) best = { delta: d, line };
        }));
        return best;
    }

    // snapMove moves r so that one of its edges/centre lines sits on a guide when it is within
    // radius; otherwise the position snaps to the grid (step, relative to origin). Guides win over
    // the grid. The result keeps the size and lists the guide lines that matched.
    function snapMove(r, targets, opts) {
        const o = opts || {};
        const step = snapStep(o.step);
        const radius = o.radius == null ? GUIDE_RADIUS : o.radius;
        const origin = o.origin || { x: 0, y: 0 };
        const l = lines(r);
        const gx = o.guides === false ? null : nearest(l.x, targets.x || [], radius);
        const gy = o.guides === false ? null : nearest(l.y, targets.y || [], radius);
        const x = gx ? r.x + gx.delta : snapValue(r.x - origin.x, step) + origin.x;
        const y = gy ? r.y + gy.delta : snapValue(r.y - origin.y, step) + origin.y;
        return { x: int(x), y: int(y), w: r.w, h: r.h, guides: { x: gx ? [gx.line] : [], y: gy ? [gy.line] : [] } };
    }

    // resizeRect applies a pointer delta to the edges a handle owns. keepRatio (corner handles)
    // keeps the start rect's aspect ratio. A side never falls below MIN_SIZE (or below its start
    // size when that is already smaller, so small widgets keep their size; never below 1) and never
    // exceeds MAX_SIDE; the opposite edge stays. Moved edges are rounded before the size is derived.
    function resizeRect(start, handle, dx, dy, opts) {
        const o = opts || {};
        const min = o.min || MIN_SIZE;
        const minW = Math.max(1, Math.min(min, start.w));
        const minH = Math.max(1, Math.min(min, start.h));
        let x1 = start.x, y1 = start.y, x2 = right(start), y2 = bottom(start);
        if (handle.includes('w')) x1 = Math.max(x2 - MAX_SIDE, Math.min(x2 - minW, int(start.x + dx)));
        if (handle.includes('e')) x2 = Math.min(x1 + MAX_SIDE, Math.max(x1 + minW, int(right(start) + dx)));
        if (handle.includes('n')) y1 = Math.max(y2 - MAX_SIDE, Math.min(y2 - minH, int(start.y + dy)));
        if (handle.includes('s')) y2 = Math.min(y1 + MAX_SIDE, Math.max(y1 + minH, int(bottom(start) + dy)));
        if (o.keepRatio && handle.length === 2 && start.w > 0 && start.h > 0) {
            const ratio = start.h / start.w;
            let h = Math.max(minH, Math.round((x2 - x1) * ratio));
            if (h > MAX_SIDE) {
                // The height hit the limit: shorten the width so the ratio still holds.
                h = MAX_SIDE;
                const w = Math.max(minW, Math.round(h / ratio));
                if (handle.includes('w')) x1 = x2 - w; else x2 = x1 + w;
            }
            if (handle.includes('n')) y1 = y2 - h; else y2 = y1 + h;
        }
        return edges(x1, y1, x2, y2);
    }

    // snapResize moves only the edges the handle owns onto guides (within radius) or the grid,
    // within the same per-axis minimum and MAX_SIDE as resizeRect. A guide is listed only when the
    // edge really ends on it after those limits.
    function snapResize(r, handle, targets, opts) {
        const o = opts || {};
        const step = snapStep(o.step);
        const radius = o.radius == null ? GUIDE_RADIUS : o.radius;
        const origin = o.origin || { x: 0, y: 0 };
        const min = o.min || MIN_SIZE;
        const minW = Math.max(1, Math.min(min, r.w));
        const minH = Math.max(1, Math.min(min, r.h));
        let x1 = r.x, y1 = r.y, x2 = right(r), y2 = bottom(r);
        const guides = { x: [], y: [] };
        const snap = (value, list, axis, base, limit) => {
            const g = o.guides === false ? null : nearest([value], list || [], radius);
            const v = limit(g ? g.line : snapValue(value - base, step) + base);
            if (g && v === g.line) guides[axis].push(g.line);
            return v;
        };
        if (handle.includes('w')) x1 = snap(x1, targets.x, 'x', origin.x, v => Math.max(x2 - MAX_SIDE, Math.min(x2 - minW, v)));
        if (handle.includes('e')) x2 = snap(x2, targets.x, 'x', origin.x, v => Math.min(x1 + MAX_SIDE, Math.max(x1 + minW, v)));
        if (handle.includes('n')) y1 = snap(y1, targets.y, 'y', origin.y, v => Math.max(y2 - MAX_SIDE, Math.min(y2 - minH, v)));
        if (handle.includes('s')) y2 = snap(y2, targets.y, 'y', origin.y, v => Math.min(y1 + MAX_SIDE, Math.max(y1 + minH, v)));
        return Object.assign(edges(x1, y1, x2, y2), { guides });
    }

    // align returns the new rects for mode left|hcenter|right|top|vcenter|bottom. The reference is
    // the bounds of all rects, or frame when only one rect is aligned (align to the parent).
    function align(rects, mode, frame) {
        const ref = rects.length === 1 && frame ? frame : bounds(rects);
        if (!ref) return [];
        return rects.map(r => {
            const n = { x: r.x, y: r.y, w: r.w, h: r.h };
            if (mode === 'left') n.x = ref.x;
            else if (mode === 'right') n.x = right(ref) - r.w;
            else if (mode === 'hcenter') n.x = ref.x + Math.round((ref.w - r.w) / 2);
            else if (mode === 'top') n.y = ref.y;
            else if (mode === 'bottom') n.y = bottom(ref) - r.h;
            else if (mode === 'vcenter') n.y = ref.y + Math.round((ref.h - r.h) / 2);
            return n;
        });
    }

    // distribute spaces three or more rects with equal gaps along axis "h" or "v"; the first and the
    // last rect (by position) stay. The result keeps the input order. Overlapping rects (a negative
    // gap) are spaced too as long as every rect stays within the span from the first rect's start to
    // the last rect's end; otherwise all rects stay unchanged, so none leaves the selection.
    function distribute(rects, axis) {
        const copies = () => rects.map(r => Object.assign({}, r));
        if (rects.length < 3) return copies();
        const pos = axis === 'v' ? 'y' : 'x';
        const size = axis === 'v' ? 'h' : 'w';
        const order = rects.map((r, i) => ({ r, i })).sort((a, b) => a.r[pos] - b.r[pos] || a.i - b.i);
        const first = order[0].r;
        const last = order[order.length - 1].r;
        const start = first[pos];
        const end = last[pos] + last[size];
        const total = order.reduce((sum, o) => sum + o.r[size], 0);
        const gap = (end - start - total) / (order.length - 1);
        const out = copies();
        let cursor = start;
        for (const o of order) {
            const p = Math.round(cursor);
            if (p < start || p + o.r[size] > end) return copies();
            out[o.i][pos] = p;
            cursor += o.r[size] + gap;
        }
        return out;
    }

    // hitTest returns the id of the last (top-most) item whose rect contains p.
    function hitTest(items, p) {
        for (let i = items.length - 1; i >= 0; i--) if (contains(items[i].rect, p)) return items[i].id;
        return null;
    }

    function inBox(items, box) { return items.filter(it => intersects(it.rect, box)).map(it => it.id); }

    // nudgeDelta maps an arrow key to a device-pixel delta: 1 px, 8 px with Shift.
    function nudgeDelta(key, big) {
        const d = big ? 8 : 1;
        return { ArrowLeft: { x: -d, y: 0 }, ArrowRight: { x: d, y: 0 }, ArrowUp: { x: 0, y: -d }, ArrowDown: { x: 0, y: d } }[key] || null;
    }

    // toWorld converts a client point to device pixels for a screen element at rect r with zoom z.
    function toWorld(client, r, zoom) { return { x: Math.floor((client.x - r.left) / zoom), y: Math.floor((client.y - r.top) / zoom) }; }

    // clampInto moves r so it lies inside frame where possible (rects larger than the frame stick
    // to its top-left corner).
    function clampInto(r, frame) {
        const x = Math.max(frame.x, Math.min(r.x, right(frame) - r.w));
        const y = Math.max(frame.y, Math.min(r.y, bottom(frame) - r.h));
        return { x, y, w: r.w, h: r.h };
    }

    P.geometry = {
        ZOOM_MIN, ZOOM_MAX, SNAP_STEPS, GUIDE_RADIUS, MIN_SIZE, MAX_SIDE, HANDLES,
        rect, clampZoom, snapStep, fitZoom, screenSize, snapValue, contains, intersects, normalizeRect, translate,
        insideAll, bounds, lines, guideTargets, snapMove, resizeRect, snapResize, align, distribute, hitTest, inBox,
        nudgeDelta, toWorld, clampInto
    };
})();
