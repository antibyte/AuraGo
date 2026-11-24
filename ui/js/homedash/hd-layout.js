// HomeDash layout (pure): breakpoints, md/sm derivation, overrides, cell metrics and fit-to-screen (frontend 4.3-4.4).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.grid) throw new Error('hd-layout.js: load hd-core.js and hd-grid.js first');
    const G = E.grid, clamp = E.core.clamp;
    const COLS = Object.freeze({ lg: 12, md: 8, sm: 4 }), BP_MIN = Object.freeze({ md: 600, lg: 960 }), HYSTERESIS = 16;
    const GAPS = Object.freeze({ compact: 8, comfortable: 12, spacious: 16 });
    let memo = new WeakMap();
    const constraintOf = (C, id) => (typeof C === 'function' ? C(id) : C && C[id]) || {};
    function derivedSize(item, toCols, c, fromCols) {
        const from = fromCols || COLS.lg, k = c || {};
        let w = Math.max(1, Math.round(item.w * toCols / from));
        if (toCols === COLS.sm && item.w >= from / 2) w = toCols;
        return { w: clamp(w, k.minW || 1, Math.min(k.maxW || toCols, toCols)), h: clamp(item.h, k.minH || 1, k.maxH || Infinity) };
    }
    function derive(lg, toCols, C) {
        const out = []; let lastY = 0;
        for (const it of G.sortReading(lg)) { const s = derivedSize(it, toCols, constraintOf(C, it.id)); const p = G.place(out, s.w, s.h, toCols, lastY); out.push({ id: it.id, x: p.x, y: p.y, w: s.w, h: s.h }); lastY = p.y; }
        return G.compact(out);
    }
    function effective(layouts, bp, C) {
        if (bp === 'lg') return layouts.lg;
        if (Array.isArray(layouts[bp])) return layouts[bp];
        let entry = memo.get(layouts.lg); if (!entry) { entry = new Map(); memo.set(layouts.lg, entry); }
        if (!entry.has(bp)) entry.set(bp, derive(layouts.lg, COLS[bp], C));
        return entry.get(bp);
    }
    function breakpoint(width, prev) {
        const raw = width >= BP_MIN.lg ? 'lg' : width >= BP_MIN.md ? 'md' : 'sm';
        if (prev === 'lg' && raw === 'md' && width >= BP_MIN.lg - HYSTERESIS) return 'lg';
        if (prev === 'md' && raw === 'sm' && width >= BP_MIN.md - HYSTERESIS) return 'md';
        return raw;
    }
    const gapFor = (density, gridGap) => (typeof gridGap === 'number' && gridGap >= 0 ? gridGap : GAPS[density] || GAPS.comfortable);
    function metrics(W, H, cols, rows, gap, rowUnit, fit) {
        const colW = (W - 2 * gap - (cols - 1) * gap) / cols, scrollRow = rowUnit === 'auto' || rowUnit == null ? clamp(colW, 56, 180) : rowUnit;
        if (fit === 'fit_screen') {
            const cell = Math.min(colW, (H - 2 * gap - (rows - 1) * gap) / rows);
            if (cell < 48) return { mode: 'overflow', colW, rowH: scrollRow, offX: 0 };
            return { mode: 'fit', colW: cell, rowH: cell, offX: (W - (cols * cell + (cols - 1) * gap)) / 2 };
        }
        return { mode: 'scroll', colW, rowH: scrollRow, offX: 0 };
    }
    function cssVars(m, cols, rows, gap) { const r = n => Math.round(n * 100) / 100;
        return { '--hd-cols': cols, '--hd-rows': rows, '--hd-colw': r(m.colW) + 'px', '--hd-rowh': r(m.rowH) + 'px', '--hd-gap': gap + 'px', '--hd-offx': r(m.offX) + 'px' }; }
    const rows = L => L.reduce((m, i) => Math.max(m, i.y + i.h), 0);
    function addToLayouts(layouts, item, C) {
        const w = clamp(Math.round(Number(item.w)) || 1, 1, COLS.lg), h = Math.max(1, Math.round(Number(item.h)) || 1);
        const p = G.place(layouts.lg, w, h, COLS.lg);
        const out = { lg: layouts.lg.concat([{ id: item.id, x: p.x, y: p.y, w, h }]), md: layouts.md == null ? null : layouts.md, sm: layouts.sm == null ? null : layouts.sm };
        for (const bp of ['md', 'sm']) {
            if (!Array.isArray(layouts[bp])) continue;
            const s = derivedSize({ w, h }, COLS[bp], constraintOf(C, item.id)), q = G.place(layouts[bp], s.w, s.h, COLS[bp]);
            out[bp] = layouts[bp].concat([{ id: item.id, x: q.x, y: q.y, w: s.w, h: s.h }]);
        }
        return out;
    }
    function removeFromLayouts(layouts, ids) { const drop = new Set([].concat(ids)), keep = L => L.filter(i => !drop.has(i.id));
        return { lg: keep(layouts.lg), md: Array.isArray(layouts.md) ? keep(layouts.md) : null, sm: Array.isArray(layouts.sm) ? keep(layouts.sm) : null }; }
    function materialiseOverride(layouts, bp, C) { if (Array.isArray(layouts[bp])) return layouts; const out = Object.assign({}, layouts); out[bp] = derive(layouts.lg, COLS[bp], C).map(i => Object.assign({}, i)); return out; }
    function resetOverride(layouts, bp) { const out = Object.assign({}, layouts); out[bp] = null; return out; }
    E.layout = Object.freeze({ COLS, BP_MIN, HYSTERESIS, GAPS, derivedSize, derive, effective, breakpoint, gapFor, metrics, cssVars, rows, addToLayouts, removeFromLayouts, materialiseOverride, resetOverride, clearMemo() { memo = new WeakMap(); } });
})();
