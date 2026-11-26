// HomeDash designer gestures (M4 contract C12, C15): move, resize, marquee and palette/entity drops on top of E.input.track. Pure maths in E.gestures.math;
// create(o) is the controller. One pointer session at a time (g). pointermove only stores the pointer and asks for one E.core.frame; tick reads first
// (scroll), then writes: drag vars, preview of the changed items only, ghosts, guides, auto-scroll. The layout is recomputed from the gesture-start
// layout only when the target cell changes, metrics and rects are frozen at gesture start. Cancel (and every non-'up' end) restores the start layout:
// page.preview(null), no commit. Only CSSOM custom properties are written (E.core.setVars).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.grid || !E.layout || !E.input) throw new Error('hd-gestures.js: load hd-grid.js, hd-layout.js and hd-input.js first');
    const { el, setVars, setAttr } = E.core;
    const THRESHOLD_CANVAS = 4, THRESHOLD_PALETTE = 5, HOLD_TOUCH_MS = 300, TOUCH_LIST = Object.freeze({ threshold: 8, axis: 'x', holdMs: HOLD_TOUCH_MS });
    const EDGE_PX = 48, SCROLL_MAX = 16, EDGES = Object.freeze(['n', 'ne', 'e', 'se', 's', 'sw', 'w', 'nw']), PAGE_WIDGETS_MAX = 120, NEVER = 1e9, CONTROL = 'button, a, input, select, textarea, [role="button"]';
    function guides(items, moving, skip, cols) {
        if (!moving.length) return []; const B = E.grid.bbox(moving), others = items.filter(i => !skip.has(i.id)), out = [];
        for (const [axis, at, side, of] of [['x', B.x, 'start', o => o.x], ['x', B.x + B.w, 'end', o => o.x + o.w], ['y', B.y, 'start', o => o.y], ['y', B.y + B.h, 'end', o => o.y + o.h]]) {
            if (at === 0 || (axis === 'x' && at === cols)) continue; if (others.some(o => of(o) === at)) out.push({ axis, at, side }); }
        return out;
    }
    function scrollStep(p, lo, hi, edge, max) { const e = edge || EDGE_PX, m = max || SCROLL_MAX; if (!(hi - lo > 2 * e)) return 0;
        if (p < lo + e) return -Math.ceil(m * Math.min(1, (lo + e - p) / e)); if (p > hi - e) return Math.ceil(m * Math.min(1, (p - (hi - e)) / e)); return 0; }
    const pxOf = (m, i) => ({ left: m.offX + m.gap + i.x * (m.colW + m.gap), top: m.gap + i.y * (m.rowH + m.gap), width: i.w * m.colW + (i.w - 1) * m.gap, height: i.h * m.rowH + (i.h - 1) * m.gap });
    const rectOf = (a, b) => ({ left: Math.min(a.x, b.x), top: Math.min(a.y, b.y), width: Math.abs(a.x - b.x), height: Math.abs(a.y - b.y) });
    function hits(items, m, r) { return items.filter(i => { const p = pxOf(m, i); return p.left < r.left + r.width && r.left < p.left + p.width && p.top < r.top + r.height && r.top < p.top + p.height; }).map(i => i.id); }
    // The one place that turns a viewport length into a page length: the design page may be scaled with CSS zoom (page.metrics().zoom, 1 when not
    // zoomed). Chrome 128+: pointer clientX/Y and getBoundingClientRect are in viewport px, page layout px are unzoomed, scroll offsets are stage px.
    const toPageLength = (zoom, v) => v / (zoom > 0 ? zoom : 1);
    const math = Object.freeze({
        toPage: toPageLength,
        cellOf: (m, x, y) => ({ x: Math.round((x - m.offX - m.gap) / (m.colW + m.gap)), y: Math.round((y - m.gap) / (m.rowH + m.gap)) }),
        cellUnder: (m, x, y) => ({ x: Math.floor((x - m.offX - m.gap / 2) / (m.colW + m.gap)), y: Math.floor((y - m.gap / 2) / (m.rowH + m.gap)) }),
        pxOf, cellDelta: (m, dx, dy) => ({ dx: Math.round(dx / (m.colW + m.gap)), dy: Math.round(dy / (m.rowH + m.gap)) }),
        itemAt: (items, c) => items.find(i => c.x >= i.x && c.x < i.x + i.w && c.y >= i.y && c.y < i.y + i.h) || null, guides, scrollStep, rectOf, hits
    });
    const px = n => Math.round(n) + 'px', inRect = (p, r) => p.x >= r.left && p.x <= r.right && p.y >= r.top && p.y <= r.bottom;
    function differs(a, b) { if (a.length !== b.length) return true; const by = new Map(a.map(i => [i.id, i]));
        return b.some(n => { const i = by.get(n.id); return !i || i.x !== n.x || i.y !== n.y || i.w !== n.w || i.h !== n.h; }); }
    // The limit a resize bumps into, from the requested (unclamped) size: 'min', 'max' (constraints) or 'edge' (grid border); '' when none.
    function limitOf(s, d) {
        const r = s.item, k = s.C || {}, e = s.edge, hor = e.includes('e') || e.includes('w'), ver = e.includes('s') || e.includes('n');
        const w = r.w + (e.includes('e') ? d.dx : 0) - (e.includes('w') ? d.dx : 0), h = r.h + (e.includes('s') ? d.dy : 0) - (e.includes('n') ? d.dy : 0);
        const minW = Math.max(1, k.minW || 1), minH = Math.max(1, k.minH || 1), maxW = Math.max(minW, Math.min(k.maxW || s.cols, s.cols)), maxH = Math.max(minH, k.maxH || Infinity);
        if ((hor && w < minW) || (ver && h < minH)) return 'min'; if ((hor && w > maxW) || (ver && h > maxH)) return 'max';
        if ((e.includes('e') && r.x + w > s.cols) || (e.includes('w') && r.x + d.dx < 0) || (e.includes('n') && r.y + d.dy < 0)) return 'edge'; return '';
    }
    function create(o) {
        const stage = o.stage, root = o.root, ctx = o.ctx, tm = o.timers || {}, life = E.core.bag(), rafOpts = { raf: o.raf, caf: o.caf };
        const layer = el('div', { class: 'hd-edit-layer', 'aria-hidden': 'true' }), pool = { ghosts: [], guides: [] };
        let g = null, destroyed = false, marqueeEl = null, selEl = null, selChip = null, selState = null, avatar = null, lastGuides = '', flips = [];
        const loop = E.core.frame(() => step(g, true), rafOpts);
        const settleLoop = E.core.frame(() => flushSettle(), rafOpts);
        const winOf = () => o.win || (stage.ownerDocument && stage.ownerDocument.defaultView) || null;
        function flushSettle() { settleLoop.cancel(); for (const f of flips) { f.removeAttribute('data-hd-flip'); setVars(f, { '--hd-e-fx': null, '--hd-e-fy': null }); } flips = []; }
        function ensureLayer() { const c = ctx(), pe = c && c.page && c.page.el; if (!pe) return null; const kids = pe.children;
            if (layer.parentNode !== pe || kids[kids.length - 1] !== layer) pe.appendChild(layer); return layer; }
        function pooled(list, n, make, parent) { while (list.length < n) list.push(make());
            list.forEach((e, i) => { if (i < n) { if (e.parentNode !== parent) parent.appendChild(e); } else if (e.parentNode) e.remove(); }); return list.slice(0, n); }
        function showGhosts(items, drop) { const L = ensureLayer(); if (!L) return;
            pooled(pool.ghosts, items.length, () => el('div', { class: 'hd-ghost' }), L).forEach((e, i) => { const it = items[i];
                setVars(e, { '--hd-x': it.x, '--hd-y': it.y, '--hd-w': it.w, '--hd-h': it.h }); setAttr(e, 'data-hd-drop', drop || null); }); }
        function showGuides(list) { const key = JSON.stringify(list); if (key === lastGuides) return; lastGuides = key; const L = ensureLayer(); if (!L) return; const use = list.slice(0, 4);
            pooled(pool.guides, use.length, () => el('div', { class: 'hd-guide' }), L).forEach((e, i) => { setAttr(e, 'data-axis', use[i].axis); setAttr(e, 'data-side', use[i].side); setVars(e, { '--hd-e-g': use[i].at }); }); }
        function clearOverlay() { pooled(pool.ghosts, 0, null, null); pooled(pool.guides, 0, null, null); lastGuides = ''; if (marqueeEl && marqueeEl.parentNode) marqueeEl.remove(); }
        function placeSel(it) { if (selEl) setVars(selEl, { '--hd-x': it.x, '--hd-y': it.y, '--hd-w': it.w, '--hd-h': it.h }); }
        function showSelection(id, chip) {
            selState = id ? { id, chip: chip == null ? '' : String(chip) } : null; const c = ctx(), L = selState ? ensureLayer() : null, it = L && c.page.items().find(i => i.id === id);
            if (!it) { if (selEl && selEl.parentNode) selEl.remove(); return; }
            if (!selEl) { selChip = el('div', { class: 'hd-sel-chip' }); selEl = el('div', { class: 'hd-sel' }, [selChip].concat(EDGES.map(e => el('div', { class: 'hd-handle', 'data-edge': e })))); }
            if (selEl.parentNode !== L) L.appendChild(selEl); setAttr(selEl, 'data-hd-wid', id); selChip.textContent = selState.chip; placeSel(it);
            for (const h of selEl.querySelectorAll('.hd-handle')) h.removeAttribute('data-hd-limit');
        }
        // The page height of a gesture only grows: a height change relays out every widget (the costliest write of a drag), so a layout that is shorter for a
        // moment does not shrink the page and the next frame does not grow it again. The commit and the cancel restore the exact height of the document.
        const roomFor = (s, next) => { s.rows = Math.max(s.rows || 0, E.layout.rows(next) + 1); return s.rows; };
        // A client point as a page-local point (page px): from the page's rect at gesture start, plus the stage scroll since, divided by the zoom.
        const local = (s, x, y, sx, sy) => ({ x: math.toPage(s.zoom, x - s.pageRect.left + (sx - s.scroll0.x)), y: math.toPage(s.zoom, y - s.pageRect.top + (sy - s.scroll0.y)) });
        // arm: the first onStart of a session. Reads (rects, scroll) happen once here; tick never reads layout.
        function arm(s, start, p, needPage) {
            const page = s.c && s.c.page, m = page && page.el ? page.metrics() : null; if (needPage && !m) return false; flushSettle();
            Object.assign(s, { page, m, zoom: m && m.zoom > 0 ? m.zoom : 1, items: s.c ? s.c.items : [], cols: s.c ? (s.c.cols || (m && m.cols)) : 0, compact: !!(s.c && s.c.compact), bp: s.c ? s.c.bp : null,
                pageRect: m ? page.el.getBoundingClientRect() : null, stageRect: stage.getBoundingClientRect(), scroll0: { x: stage.scrollLeft, y: stage.scrollTop },
                start: { x: start.x, y: start.y }, pointer: { x: p.x, y: p.y }, bag: E.core.bag(), started: true, fresh: true, sig: '', lastKey: '', drag: { dx: 0, dy: 0 } });
            ensureLayer(); s.bag.on(stage, 'scroll', () => { if (g === s) loop(); }, { passive: true });
            if (start.type === 'touch') s.bag.on(s.kind === 'drop' ? (root.ownerDocument || stage) : stage, 'touchmove', e => { if (e.cancelable) e.preventDefault(); }, { passive: false });
            const w = winOf(); if (w) s.bag.on(w, 'blur', () => cancel());
            o.hold(true); s.held = true; setAttr(root, 'data-hd-gesture', s.kind); return true;
        }
        function run(ev, s, spec) {
            const hold = !!spec.hold; g = s;
            s.track = E.input.track(ev, Object.assign({ capture: stage, win: o.win, setTimeout: tm.setTimeout, clearTimeout: tm.clearTimeout, now: tm.now,
                threshold: hold ? NEVER : THRESHOLD_CANVAS, holdMs: hold ? HOLD_TOUCH_MS : 0 }, spec.track, {
                onStart: (start, p) => { if (g !== s) return; if (!arm(s, start, p, spec.needPage !== false)) { finish(s, 'cancel'); return; } if (s.begin) s.begin(); step(s, true, true); },
                onMove: p => { if (g !== s || !s.started) return; s.pointer = p; s.fresh = true; loop(); },
                onEnd: (p, start, how) => { if (g !== s) return; if (how === 'up' && s.started) { s.pointer = p; step(s, false); } finish(s, how); },
                onTap: start => { if (g !== s) return; g = null; if (s.tap) s.tap(start); } }));
            if (!s.track && g === s) g = null; return s.track;
        }
        // first: the step right after arm() writes (attributes, layer, ghosts): the scroll position was read a moment ago and cannot have changed, and reading it
        // again after those writes would force a layout (~220 ms on the 60-widget page).
        function step(s, scroll, first) {
            if (!s || g !== s || !s.started) return;
            const sx = first ? s.scroll0.x : stage.scrollLeft, sy = first ? s.scroll0.y : stage.scrollTop, p = s.pointer, dx = math.toPage(s.zoom, p.x - s.start.x + (sx - s.scroll0.x)), dy = math.toPage(s.zoom, p.y - s.start.y + (sy - s.scroll0.y));   // reads first; page px
            if (s.kind === 'move') stepMove(s, dx, dy); else if (s.kind === 'resize') stepResize(s, dx, dy); else if (s.kind === 'marquee') stepMarquee(s, p, sx, sy); else stepDrop(s, p, sx, sy);
            if (!scroll) return;
            const sig = sx + ',' + sy, go = s.fresh || sig !== s.sig; s.sig = sig; s.fresh = false;
            const ax = scrollStep(p.x, s.stageRect.left, s.stageRect.right), ay = scrollStep(p.y, s.stageRect.top, s.stageRect.bottom);
            if ((ax || ay) && go && (s.kind !== 'drop' || s.inside)) { stage.scrollLeft = sx + ax; stage.scrollTop = sy + ay; loop(); }
        }
        function finish(s, how) {
            if (g !== s) return; g = null; loop.cancel(); if (s.bag) s.bag.dispose(); clearOverlay(); root.removeAttribute('data-hd-gesture');
            if (avatar) { avatar.remove(); avatar = null; }
            if (s.started) {
                for (const f of s.frames || []) { f.removeAttribute('data-hd-dragging'); setVars(f, { '--hd-e-dx': null, '--hd-e-dy': null }); }
                if (s.hover) s.hover.removeAttribute('data-hd-drop'); if (s.limitEl) s.limitEl.removeAttribute('data-hd-limit');
                let did = false; try { did = how === 'up' && s.commit ? s.commit() !== false : false; } finally { if (s.held) o.hold(false); }
                const c = ctx(); if (c && c.page) c.page.preview(null);
                if (did && s.settle) s.settle(c);
                if (!did && s.undo) s.undo();
                if (selState) showSelection(selState.id, selState.chip);
            }
        }
        // A press that ended before it became a drag (touch scroll, pointercancel, vertical swipe) calls no callback: drop it here.
        function live() { if (g && !g.started && g.track && g.track.done()) g = null; return g; }
        function cancel() { const s = g; if (!s) return; if (s.track && !s.track.done()) s.track.cancel(); finish(s, 'cancel'); }
        // ---- move
        function startMove(ev, frame, c) {
            const id = frame.getAttribute('data-hd-wid'); if (!id) return; const sel = c.selection || [], selected = sel.includes(id), mod = ev.shiftKey || ev.ctrlKey || ev.metaKey, touch = ev.pointerType === 'touch';
            const hold = touch && !selected && !mod; let ids = selected ? sel.slice() : [id];
            if (ev.pointerType === 'mouse') ev.preventDefault();
            if (!hold && !selected) { if (mod) { ids = sel.concat(id); } o.select(ids, { focus: true }); }
            const s = { kind: 'move', c, id, ids, idSet: new Set(ids) };
            s.tap = () => { if (mod && selected) o.select(sel.filter(x => x !== id), { focus: true }); else if (!mod) o.select([id], { focus: true }); };
            s.begin = () => { if (hold) o.select(ids, { focus: true }); const mine = s.items.filter(i => s.idSet.has(i.id)); if (!mine.length) { cancel(); return; }
                s.B = E.grid.bbox(mine); s.frames = ids.map(i => s.page.frame(i)).filter(Boolean); for (const f of s.frames) setAttr(f, 'data-hd-dragging', 'true'); };
            s.commit = () => { if (!s.next || !differs(s.items, s.next)) return false; o.commit('move', { ids, next: s.next, bp: s.bp }); return true; };
            s.settle = cur => { if (E.core.motionReduced(root) || !cur.page) return;
                for (const i of ids) { const f = cur.page.frame(i), a = s.items.find(x => x.id === i), b = s.next.find(x => x.id === i); if (!f || !a || !b) continue;
                    const pa = pxOf(s.m, a), pb = pxOf(s.m, b), fx = Math.round(pa.left + s.drag.dx - pb.left), fy = Math.round(pa.top + s.drag.dy - pb.top); if (!fx && !fy) continue;
                    setVars(f, { '--hd-e-fx': px(fx), '--hd-e-fy': px(fy) }); setAttr(f, 'data-hd-flip', 'from'); flips.push(f); }
                if (flips.length) settleLoop(); };
            run(ev, s, { hold });
        }
        function stepMove(s, dx, dy) {
            if (dx !== s.drag.dx || dy !== s.drag.dy || !s.lastKey) { s.drag = { dx, dy }; for (const f of s.frames) setVars(f, { '--hd-e-dx': px(dx), '--hd-e-dy': px(dy) }); }
            const d = math.cellDelta(s.m, dx, dy), key = d.dx + ',' + d.dy; if (key === s.lastKey) return; s.lastKey = key;
            const next = E.grid.moveItems(s.items, s.ids, { x: s.B.x + d.dx, y: s.B.y + d.dy }, s.cols, { compact: s.compact }), moved = next.filter(i => s.idSet.has(i.id)); s.next = next;
            s.page.preview(next, { skip: s.idSet, animate: true, rows: roomFor(s, next) }); showGhosts(moved); showGuides(guides(next, moved, s.idSet, s.cols));
        }
        // ---- resize
        function startResize(ev, handle, c) {
            const box = handle.closest('.hd-sel'), id = box && box.getAttribute('data-hd-wid'), item = id && c.items.find(i => i.id === id); if (!item) return; if (ev.pointerType === 'mouse') ev.preventDefault();
            const s = { kind: 'resize', c, id, item, edge: handle.getAttribute('data-edge') || 'se', idSet: new Set([id]), C: c.C ? c.C(id) : null, limit: '', limitEl: handle };
            s.commit = () => { if (!s.next || !differs(s.items, s.next)) return false; o.commit('resize', { id, next: s.next, bp: s.bp, edge: s.edge }); return true; };
            run(ev, s, {});
        }
        function stepResize(s, dx, dy) {
            const d = math.cellDelta(s.m, dx, dy), key = d.dx + ',' + d.dy; if (key === s.lastKey) return; s.lastKey = key;
            const next = E.grid.resizeItem(s.items, s.id, s.edge, d, s.cols, s.C, { compact: s.compact }), it = next.find(i => i.id === s.id); s.next = next;
            const lim = limitOf(s, d); if (lim !== s.limit) { s.limit = lim; setAttr(s.limitEl, 'data-hd-limit', lim || null); if (lim) o.announce(o.describeLimit(lim)); }
            s.page.preview(next, { animate: true, rows: roomFor(s, next) }); placeSel(it); showGuides(guides(next, [it], s.idSet, s.cols));
        }
        // ---- marquee
        function startMarquee(ev, c) {
            const touch = ev.pointerType === 'touch', additive = !!ev.shiftKey, base = (c.selection || []).slice(); if (ev.pointerType === 'mouse') ev.preventDefault();
            const s = { kind: 'marquee', c, additive, base, ids: base, key: base.join('|') };
            s.tap = () => { if (!additive) o.select([]); };
            s.begin = () => { s.startLocal = local(s, s.start.x, s.start.y, s.scroll0.x, s.scroll0.y);
                if (!marqueeEl) marqueeEl = el('div', { class: 'hd-marquee' }); const L = ensureLayer(); if (L && marqueeEl.parentNode !== L) L.appendChild(marqueeEl); };
            s.commit = () => { o.commit('marquee', { ids: s.ids, additive }); return true; };
            s.undo = () => o.select(base, { live: true });
            run(ev, s, { hold: touch });
        }
        function stepMarquee(s, p, sx, sy) {
            const r = rectOf(s.startLocal, local(s, p.x, p.y, sx, sy));
            const key = [r.left, r.top, r.width, r.height].map(Math.round).join(','); if (key === s.lastKey) return; s.lastKey = key;
            setVars(marqueeEl, { '--hd-e-mx': px(r.left), '--hd-e-my': px(r.top), '--hd-e-mw': px(r.width), '--hd-e-mh': px(r.height) });
            const hit = hits(s.items, s.m, r), ids = s.additive ? s.base.concat(hit.filter(i => !s.base.includes(i))) : hit, k = ids.join('|');
            if (k !== s.key) { s.key = k; s.ids = ids; o.select(ids, { live: true }); }
        }
        // ---- drops from the palette / entity browser (part 2)
        function dragFrom(ev, payload) {
            if (live() || destroyed || !ev) return null; const c = ctx(); if (c && c.readonly) return null; const touch = ev.pointerType === 'touch';
            const s = { kind: 'drop', c, payload, temp: E.core.uid('hd-new'), drop: { mode: '' }, inside: false, idSet: new Set() };
            s.begin = () => { s.entries = payload.entries(); const first = s.entries[0] && s.entries[0].size || { w: 1, h: 1 };
                s.size = { w: Math.min(Math.max(1, first.w), s.cols || first.w), h: Math.max(1, first.h) }; s.full = s.items.length + s.entries.length > PAGE_WIDGETS_MAX;
                s.rootRect = root.getBoundingClientRect(); avatar = el('div', { class: 'hd-drop-avatar', 'data-hd-drop': 'out' }, payload.avatar ? payload.avatar() : null); root.appendChild(avatar); };
            s.tap = () => { if (payload.onTap) payload.onTap(); };
            s.commit = () => { const d = s.drop;
                if (d.mode === 'add') { o.commit('add', { entries: s.entries, at: d.at, bp: s.bp }); return true; }
                if (d.mode === 'rebind') { o.commit('rebind', { id: d.id, binding: 'primary', entity: payload.entity }); return true; }
                if (d.mode === 'invalid') o.announce(o.describeLimit(s.full ? 'page_full' : 'drop_invalid', payload.label || '')); return false; };
            return run(ev, s, { needPage: false, track: Object.assign({ capture: ev.currentTarget || null }, touch ? TOUCH_LIST : { threshold: THRESHOLD_PALETTE, holdMs: 0 }) });
        }
        function setDrop(s, mode, extra) {
            const d = s.drop, id = extra && extra.id || null; if (d.mode === mode && d.id === id && !(extra && extra.at)) return;
            if (s.hover && (d.id !== id || mode === 'add' || mode === 'out')) { s.hover.removeAttribute('data-hd-drop'); s.hover = null; }
            if (d.at && !(extra && extra.at) && s.page) { s.page.preview(null); showGhosts([]); showGuides([]); s.lastKey = ''; }
            s.drop = Object.assign({ mode, id }, extra); setAttr(avatar, 'data-hd-drop', mode);
            if (id && s.page && (mode === 'rebind' || mode === 'invalid')) { s.hover = s.page.frame(id); if (s.hover) setAttr(s.hover, 'data-hd-drop', mode); }
        }
        function stepDrop(s, p, sx, sy) {
            setVars(avatar, { '--hd-e-ax': px(p.x - s.rootRect.left), '--hd-e-ay': px(p.y - s.rootRect.top) });
            s.inside = !!(s.m && inRect(p, s.stageRect)); if (!s.inside) { setDrop(s, 'out'); return; }
            const { x: lx, y: ly } = local(s, p.x, p.y, sx, sy), m = s.m;
            if (s.payload.kind === 'entity') { const it = math.itemAt(s.items, math.cellUnder(m, lx, ly)), w = it && s.c.widget(it.id);
                if (it) { setDrop(s, w && s.c.catalog.accepts(w.type, 'primary', s.payload.entity) ? 'rebind' : 'invalid', { id: it.id }); return; } }
            const { w, h } = s.size, t = math.cellOf(m, lx - (w * m.colW + (w - 1) * m.gap) / 2, ly - (h * m.rowH + (h - 1) * m.gap) / 2), x = Math.min(Math.max(0, t.x), Math.max(0, s.cols - w)), y = Math.max(0, t.y), key = x + ',' + y + (s.full ? 'f' : '');
            if (!s.drop.at) s.lastKey = ''; if (key === s.lastKey) return; s.lastKey = key;
            const slot = E.grid.place(s.items, w, h, s.cols, 0), base = s.items.concat([{ id: s.temp, x: slot.x, y: slot.y, w, h }]);
            const next = E.grid.moveItems(base, [s.temp], { x, y }, s.cols, { compact: s.compact }), tmp = next.find(i => i.id === s.temp), shown = next.filter(i => i.id !== s.temp);
            setDrop(s, s.full ? 'invalid' : 'add', { at: { x: tmp.x, y: tmp.y } });
            s.page.preview(shown, { animate: true, rows: roomFor(s, next) }); showGhosts([tmp], s.full ? 'invalid' : null); showGuides(s.full ? [] : guides(next, [tmp], new Set([s.temp]), s.cols));
        }
        // ---- canvas pointerdown (delegated)
        life.on(stage, 'pointerdown', ev => {
            if (live() || destroyed || ev.button > 0 || ev.isPrimary === false || ev.defaultPrevented) return; const c = ctx(), t = ev.target; if (!c || c.readonly || !t || !t.closest) return;
            const handle = t.closest('.hd-handle'); if (handle) { startResize(ev, handle, c); return; }
            const frame = t.closest('.hd-page > .hd-widget'); if (frame) { startMove(ev, frame, c); return; }
            if (t.closest('.hd-page') && !t.closest(CONTROL)) startMarquee(ev, c);
        });
        return {
            dragFrom, cancel, active: () => (g && g.started ? g.kind : null), showSelection,
            reset() { cancel(); flushSettle(); ensureLayer(); if (selState) showSelection(selState.id, selState.chip); },
            destroy() { if (destroyed) return; cancel(); destroyed = true; life.dispose(); loop.cancel(); flushSettle(); clearOverlay(); for (const n of [layer, selEl]) if (n && n.parentNode) n.remove(); }
        };
    }
    E.gestures = Object.freeze({ THRESHOLD_CANVAS, THRESHOLD_PALETTE, HOLD_TOUCH_MS, TOUCH_LIST, EDGE_PX, SCROLL_MAX, EDGES, math, create });
})();
