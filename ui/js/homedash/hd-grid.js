// HomeDash grid engine (pure; frontend part 4.2). Items {id,x,y,w,h} in cells; functions return new arrays.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-grid.js: load hd-core.js first');
    const clamp = E.core.clamp, PROBE = {};
    const item = i => ({ id: i.id, x: i.x, y: i.y, w: i.w, h: i.h });
    const collides = (a, b) => a.id !== b.id && a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
    const sortReading = L => L.slice().sort((a, b) => a.y - b.y || a.x - b.x || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
    function compact(L, gravity) {
        const placed = [];
        for (const src of sortReading(L)) {
            const it = item(src); it.y = Math.max(0, Math.round(it.y));
            if (gravity !== false) while (it.y > 0 && !placed.some(p => collides(p, Object.assign({}, it, { y: it.y - 1 })))) it.y--;
            while (placed.some(p => collides(p, it))) it.y++;
            placed.push(it);
        }
        return sortReading(placed);
    }
    function pushDown(L, moverIds) {
        const out = L.map(item), fixedIds = new Set(moverIds), fixed = out.filter(i => fixedIds.has(i.id)), queue = fixed.slice();
        let guard = 0;
        while (queue.length) {
            if (++guard > 100000) throw new Error('hd-grid: pushDown did not settle');
            const m = queue.shift();
            for (const o of sortReading(out)) {
                if (fixedIds.has(o.id) || !collides(m, o)) continue;
                o.y = m.y + m.h;
                for (let moved = true; moved;) { moved = false; for (const f of fixed) if (collides(f, o)) { o.y = f.y + f.h; moved = true; } }
                queue.push(o);
            }
        }
        return out;
    }
    function bbox(S) {
        const x = Math.min(...S.map(s => s.x)), y = Math.min(...S.map(s => s.y));
        return { x, y, w: Math.max(...S.map(s => s.x + s.w)) - x, h: Math.max(...S.map(s => s.y + s.h)) - y };
    }
    const settle = (L, o) => (o && o.compact === false ? L : compact(L));
    function moveItems(L, ids, target, cols, opts) {
        const idSet = new Set(ids), out = L.map(item), S = out.filter(i => idSet.has(i.id));
        if (!S.length) return settle(out, opts);
        const B = bbox(S), dx = clamp(Math.round(target.x), 0, Math.max(0, cols - B.w)) - B.x, dy = Math.max(0, Math.round(target.y)) - B.y;
        if (!dx && !dy) return settle(out, opts);
        for (const s of S) { s.x += dx; s.y += dy; }
        if (S.length === 1) {
            const s = S[0], cx = s.x + s.w / 2, cy = s.y + s.h / 2, origin = L.find(i => i.id === s.id);
            const t = out.find(o => !idSet.has(o.id) && o.w === s.w && o.h === s.h && cx >= o.x && cx < o.x + o.w && cy >= o.y && cy < o.y + o.h);
            if (t) { t.x = origin.x; t.y = origin.y; return settle(out, opts); }
        }
        return settle(pushDown(out, S.map(s => s.id)), opts);
    }
    function resizeItem(L, id, edge, d, cols, c, opts) {
        const out = L.map(item), r = out.find(i => i.id === id);
        if (!r) return settle(out, opts);
        const k = c || {}, minW = Math.max(1, k.minW || 1), minH = Math.max(1, k.minH || 1);
        const maxW = Math.max(minW, Math.min(k.maxW || cols, cols)), maxH = Math.max(minH, k.maxH || Infinity);
        const right = r.x + r.w, bottom = r.y + r.h, dx = Math.round((d && d.dx) || 0), dy = Math.round((d && d.dy) || 0);
        let { x, y, w, h } = r;
        if (edge.includes('e')) w += dx;
        if (edge.includes('w')) { x += dx; w -= dx; }
        if (edge.includes('s')) h += dy;
        if (edge.includes('n')) { y += dy; h -= dy; }
        w = clamp(w, minW, maxW); h = clamp(h, minH, maxH);
        if (edge.includes('e') && !edge.includes('w')) w = clamp(Math.min(w, cols - x), minW, maxW);
        if (edge.includes('w')) { x = right - w; if (x < 0) { x = 0; w = clamp(right, minW, maxW); } }
        if (edge.includes('n')) { y = bottom - h; if (y < 0) { y = 0; h = clamp(bottom, minH, maxH); } }
        Object.assign(r, { x: clamp(x, 0, Math.max(0, cols - w)), y: Math.max(0, y), w, h });
        return settle(pushDown(out, [id]), opts);
    }
    function place(L, w, h, cols, startY) {
        const ww = Math.min(Math.max(1, w), cols), hh = Math.max(1, h), from = Math.max(0, startY || 0);
        const bottom = Math.max(from, L.reduce((m, i) => Math.max(m, i.y + i.h), 0));
        for (let y = from; y <= bottom; y++) for (let x = 0; x <= cols - ww; x++) { const p = { id: PROBE, x, y, w: ww, h: hh }; if (!L.some(i => collides(i, p))) return { x, y }; }
        return { x: 0, y: bottom };
    }
    function repair(L, cols, C, opts) {
        const known = opts && opts.ids, seen = new Set(), out = [];
        const int = (v, d) => (Number.isFinite(Number(v)) ? Math.round(Number(v)) : d);
        for (const raw of Array.isArray(L) ? L : []) {
            if (!raw || typeof raw.id !== 'string' || seen.has(raw.id) || (known && !known.has(raw.id))) continue;
            seen.add(raw.id);
            const c = (typeof C === 'function' ? C(raw.id) : C && C[raw.id]) || {};
            const w = clamp(int(raw.w, 1), Math.max(1, c.minW || 1), Math.min(c.maxW || cols, cols)), h = clamp(int(raw.h, 1), Math.max(1, c.minH || 1), c.maxH || Infinity);
            out.push({ id: raw.id, x: clamp(int(raw.x, 0), 0, cols - w), y: Math.max(0, int(raw.y, 0)), w, h });
        }
        return compact(out, !(opts && opts.compact === false));
    }
    E.grid = Object.freeze({ collides, sortReading, compact, pushDown, bbox, moveItems, resizeItem, place, repair });
})();
