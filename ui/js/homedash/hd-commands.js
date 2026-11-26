// HomeDash designer commands (pure; frontend 4.3, 5.5-5.10; M4 contract C5). Builders take the working document (E.doc.normalise output) and an
// argument object and return a command {label, vars, ops, meta, select?, page?, materialised?, announce?}, a refusal {refused}, a size limit {limit}
// or null when nothing changes. Paths are id-keyed; meta = {kind, ids, at, coalesce?, window?}. An md/sm change while that layout is automatic
// materialises it (one op null -> array). Adding places into lg and every non-null override; deleting removes from all; lg changes never touch
// overrides. keymap(ev, ctx) is the key map (frontend 5.6, C14); spatial(items, id, dir) the Alt+Arrow target; readingOrder(items) the Tab order.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.doc || !E.grid || !E.layout) throw new Error('hd-commands.js: load hd-doc.js first');
    const D = E.doc, G = E.grid, L = E.layout, { clone, randomId, clamp } = E.core, COLS = L.COLS;
    const NUDGE_MS = 800, FIELD_MS = 1000, LIMITS = Object.freeze({ pages: 12, pageWidgets: 120, widgets: 400 });
    const DEFAULTS = Object.freeze({ variant: '', title: '', bindings: {}, options: {}, actions: {}, confirm: false });
    const sortedKey = ids => [...new Set(ids)].sort().join(',');
    const pageOf = (doc, id) => (doc && Array.isArray(doc.pages) ? doc.pages.find(p => p.id === id) : null) || null;
    const indexOf = (doc, id) => doc.pages.findIndex(p => p.id === id);
    const total = doc => doc.pages.reduce((n, p) => n + p.widgets.length, 0);
    const compactOf = page => page.compact !== false;
    const constraint = (C, id) => (typeof C === 'function' ? C(id) : C && C[id]) || {};
    function make(label, ops, meta, extra) {
        const list = ops.filter(Boolean); if (!list.length) return null;
        const m = Object.assign({}, meta); if (!Number.isFinite(m.at)) m.at = Date.now();
        return Object.assign({ label, vars: null, ops: list, meta: m }, extra || {});
    }
    function usedIds(doc) { const s = new Set(); for (const p of doc.pages) { s.add(p.id); for (const w of p.widgets) s.add(w.id); } return s; }
    function fresh(used, prefix, n) { for (;;) { const id = randomId(prefix, n); if (!used.has(id)) { used.add(id); return id; } } }
    function layoutOp(doc, page, bp, next, C) { return D.keyOf(L.effective(page.layouts, bp, C)) === D.keyOf(next) ? null : D.op(doc, ['pages', page.id, 'layouts', bp], next); }
    function layout(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null;
        const bp = a.bp || 'lg', op = layoutOp(doc, page, bp, a.next, a.C);
        return make(a.label || 'homedash.cmd.move', [op], { kind: a.kind || 'layout', ids: (a.ids || []).slice(), at: a.now, coalesce: a.coalesce, window: a.window },
            { materialised: op && bp !== 'lg' && !Array.isArray(page.layouts[bp]) ? bp : null, announce: a.announce || null });
    }
    function nudge(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || (!a.dx && !a.dy)) return null;
        const bp = a.bp || 'lg', cols = COLS[bp], cur = L.effective(page.layouts, bp, a.C), ids = (a.ids || []).filter(id => cur.some(i => i.id === id)); if (!ids.length) return null;
        const B = G.bbox(cur.filter(i => ids.includes(i.id))), steps = a.dy ? 64 : 1;
        for (let k = 1; k <= steps; k++) {
            const to = { x: B.x + a.dx * k, y: B.y + a.dy * k }; if (to.x < 0 || to.x + B.w > cols || to.y < 0) return { limit: 'edge' };
            const next = G.moveItems(cur, ids, to, cols, { compact: compactOf(page) }), nb = G.bbox(next.filter(i => ids.includes(i.id)));
            if ((a.dx && nb.x !== B.x) || (a.dy > 0 && nb.y > B.y) || (a.dy < 0 && nb.y < B.y))
                return layout(doc, { pageId: page.id, bp, next, C: a.C, ids, now: a.now, kind: 'nudge', label: 'homedash.cmd.move', coalesce: 'nudge|' + bp + '|' + sortedKey(ids), window: NUDGE_MS,
                    announce: ids.length === 1 ? { key: 'moved', col: nb.x + 1, row: nb.y + 1 } : { key: 'moved_many', count: ids.length } });
        }
        return { limit: 'blocked' };
    }
    function resizeStep(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || (!a.dw && !a.dh)) return null;
        const bp = a.bp || 'lg', cols = COLS[bp], cur = L.effective(page.layouts, bp, a.C), r = cur.find(i => i.id === a.id); if (!r) return null;
        const next = G.resizeItem(cur, a.id, a.dw ? 'e' : 's', { dx: a.dw || 0, dy: a.dh || 0 }, cols, constraint(a.C, a.id), { compact: compactOf(page) }), n = next.find(i => i.id === a.id);
        if (n.w === r.w && n.h === r.h) return { limit: (a.dw || a.dh) < 0 ? 'min' : a.dw > 0 && r.x + r.w >= cols ? 'edge' : 'max' };
        return layout(doc, { pageId: page.id, bp, next, C: a.C, ids: [a.id], now: a.now, kind: 'resize', label: 'homedash.cmd.resize', coalesce: 'resize|' + bp + '|' + a.id, window: NUDGE_MS, announce: { key: 'resized', w: n.w, h: n.h } });
    }
    // Inserts item {id, w, h} (lg size): lg at first fit from startY, or moved to `at` for an lg drop; an md/sm drop materialises that override and moves
    // the item to `at` there; other non-null overrides get the derived size at first fit (frontend 4.3).
    function inserted(layouts, item, C, o) {
        const compact = o.compact !== false, out = { lg: layouts.lg, md: layouts.md, sm: layouts.sm }, lgAt = o.bp === 'lg' && o.at;
        const p = G.place(out.lg, item.w, item.h, COLS.lg, lgAt ? 0 : o.startY || 0);
        out.lg = out.lg.concat([{ id: item.id, x: p.x, y: p.y, w: item.w, h: item.h }]);
        if (lgAt) out.lg = G.moveItems(out.lg, [item.id], o.at, COLS.lg, { compact });
        for (const bp of ['md', 'sm']) {
            const dropHere = o.bp === bp && o.at; if (!dropHere && !Array.isArray(out[bp])) continue;
            const base = Array.isArray(out[bp]) ? out[bp] : L.derive(layouts.lg, COLS[bp], C), s = L.derivedSize(item, COLS[bp], constraint(C, item.id)), q = G.place(base, s.w, s.h, COLS[bp]);
            const added = base.concat([{ id: item.id, x: q.x, y: q.y, w: s.w, h: s.h }]);
            out[bp] = dropHere ? G.moveItems(added, [item.id], o.at, COLS[bp], { compact }) : added;
        }
        return out;
    }
    function add(doc, a) {
        const page = pageOf(doc, a.pageId), list = (a.entries || []).filter(e => e && e.widget && typeof e.widget.type === 'string');
        if (!page || !list.length) return null;
        if (page.widgets.length + list.length > LIMITS.pageWidgets || total(doc) + list.length > LIMITS.widgets) return { refused: 'limit' };
        const used = usedIds(doc), extra = new Map(), Cx = id => extra.get(id) || constraint(a.C, id), ops = [], ids = [], bp = a.bp || 'lg';
        let work = doc, layouts = page.layouts, startY = a.startY || 0;
        list.forEach((e, i) => {
            const w = Object.assign(clone(DEFAULTS), clone(e.widget)); w.id = fresh(used, 'w', 8); ids.push(w.id);
            const c = typeof a.limitsOf === 'function' ? a.limitsOf(w) || {} : {}; extra.set(w.id, c);
            const size = { w: clamp(Math.round(e.size && e.size.w) || 1, c.minW || 1, Math.min(c.maxW || 12, 12)), h: clamp(Math.round(e.size && e.size.h) || 1, c.minH || 1, c.maxH || 24) };
            const op = D.op(work, ['pages', page.id, 'widgets', w.id], w); ops.push(op); work = D.apply(work, [op], 'after');
            layouts = inserted(layouts, { id: w.id, w: size.w, h: size.h }, Cx, { bp, at: i === 0 ? a.at : null, startY, compact: compactOf(page) });
            if (i === 0) startY = layouts.lg.find(x => x.id === w.id).y;
        });
        // A compact page ends compacted, as E.doc.normalise (E.grid.repair) leaves it on the next load: the local document equals the reloaded one.
        if (compactOf(page)) layouts = { lg: G.compact(layouts.lg), md: Array.isArray(layouts.md) ? G.compact(layouts.md) : layouts.md, sm: Array.isArray(layouts.sm) ? G.compact(layouts.sm) : layouts.sm };
        for (const b of ['lg', 'md', 'sm']) if (D.keyOf(page.layouts[b]) !== D.keyOf(layouts[b])) ops.push({ path: ['pages', page.id, 'layouts', b], before: clone(page.layouts[b]), after: clone(layouts[b]) });
        const first = L.effective(layouts, bp, Cx).find(x => x.id === ids[0]) || { x: 0, y: 0 };
        return make(a.label || 'homedash.cmd.add', ops, { kind: a.kind || 'add', ids, at: a.now }, { select: ids, materialised: bp !== 'lg' && a.at && !Array.isArray(page.layouts[bp]) ? bp : null,
            announce: { key: ids.length === 1 ? 'added' : 'added_many', count: ids.length, id: ids[0], col: first.x + 1, row: first.y + 1 } });
    }
    function remove(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null; const ids = (a.ids || []).filter(id => page.widgets.some(w => w.id === id)); if (!ids.length) return null;
        return make(a.label || 'homedash.cmd.delete', D.removeWidgets(doc, page.id, ids).ops, { kind: a.kind || 'delete', ids, at: a.now }, { select: [], announce: { key: ids.length === 1 ? 'deleted' : 'deleted_many', count: ids.length, id: ids[0] } });
    }
    function duplicate(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null; const order = G.sortReading(page.layouts.lg.filter(i => (a.ids || []).includes(i.id))); if (!order.length) return null;
        const entries = order.map(i => { const w = clone(page.widgets.find(x => x.id === i.id)); delete w.id; return { widget: w, size: { w: i.w, h: i.h } }; });
        return add(doc, { pageId: page.id, bp: 'lg', entries, startY: Math.max(...order.map(i => i.y + i.h)), C: a.C, limitsOf: a.limitsOf, now: a.now, label: 'homedash.cmd.duplicate', kind: 'duplicate' });
    }
    const paste = (doc, a) => add(doc, Object.assign({}, a, { at: null, label: 'homedash.cmd.paste', kind: 'paste' }));
    function setWidget(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || !page.widgets.some(w => w.id === a.id)) return null;
        const path = ['pages', page.id, 'widgets', a.id].concat(a.path); if (D.keyOf(D.getAt(doc, path)) === D.keyOf(a.value)) return null;
        return make(a.label || 'homedash.cmd.option', [D.op(doc, path, a.value)], { kind: a.kind || 'field', ids: [a.id], at: a.now, coalesce: a.text ? 'field|' + a.id + '|' + a.path.join('.') : undefined, window: a.text ? FIELD_MS : undefined }, { vars: a.vars || null });
    }
    const bind = (doc, a) => setWidget(doc, { pageId: a.pageId, id: a.id, path: ['bindings', a.binding || 'primary'], value: a.entity || undefined, label: a.entity ? 'homedash.cmd.bind' : 'homedash.cmd.unbind', kind: 'bind', now: a.now });
    function fitItem(arr, id, c, cols, compact) {
        const r = arr.find(i => i.id === id); if (!r) return arr;
        const w = clamp(r.w, c.minW || 1, Math.min(c.maxW || cols, cols)), h = clamp(r.h, c.minH || 1, c.maxH || Infinity); if (w === r.w && h === r.h) return arr;
        return G.resizeItem(arr, id, 'se', { dx: w - r.w, dy: h - r.h }, cols, c, { compact });
    }
    function variant(doc, a) {
        const page = pageOf(doc, a.pageId), w = page && page.widgets.find(x => x.id === a.id); if (!w || w.variant === a.variant) return null;
        const ops = [D.op(doc, ['pages', page.id, 'widgets', a.id, 'variant'], a.variant)];
        for (const bp of ['lg', 'md', 'sm']) { const arr = page.layouts[bp]; if (!Array.isArray(arr)) continue; const next = fitItem(arr, a.id, a.limits || {}, COLS[bp], compactOf(page)); if (next !== arr) ops.push(D.op(doc, ['pages', page.id, 'layouts', bp], next)); }
        return make('homedash.cmd.variant', ops, { kind: 'variant', ids: [a.id], at: a.now }, { select: [a.id] });
    }
    function applyFamily(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null;
        const ops = page.widgets.filter(w => (a.ids || []).includes(w.id) && (w.family || '') !== (a.family || '')).map(w => D.op(doc, ['pages', page.id, 'widgets', w.id, 'family'], a.family || undefined));
        return make('homedash.cmd.family', ops, { kind: 'family', ids: (a.ids || []).slice(), at: a.now });
    }
    function align(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page) return null; const bp = a.bp || 'lg', cols = COLS[bp], cur = L.effective(page.layouts, bp, a.C), p = cur.find(i => i.id === a.primaryId); if (!p) return null;
        const moved = (a.ids || []).filter(id => id !== p.id && cur.some(i => i.id === id));
        let next = cur.map(i => (moved.includes(i.id) ? Object.assign({}, i, a.edge === 'top' ? { y: p.y } : { x: clamp(p.x, 0, cols - i.w) }) : i));
        next = G.pushDown(next, moved); if (compactOf(page)) next = G.compact(next);
        return layout(doc, { pageId: page.id, bp, next, C: a.C, ids: moved, now: a.now, kind: 'align', label: 'homedash.cmd.align' });
    }
    function resetLayout(doc, a) { const page = pageOf(doc, a.pageId); if (!page || a.bp === 'lg' || !Array.isArray(page.layouts[a.bp])) return null;
        return make('homedash.cmd.reset_layout', [D.op(doc, ['pages', page.id, 'layouts', a.bp], null)], { kind: 'reset', ids: [], at: a.now }, { announce: { key: 'now_auto_' + a.bp } }); }
    function addPage(doc, a) {
        if (doc.pages.length >= LIMITS.pages) return { refused: 'limit' };
        const id = fresh(usedIds(doc), 'p', 6), page = { id, name: Array.from(String(a.name || '')).slice(0, 40).join('') || id, icon: a.icon || 'dashboard', compact: true, widgets: [], layouts: { lg: [], md: null, sm: null } };
        return make('homedash.cmd.page_add', [D.op(doc, ['pages', id], page)], { kind: 'page', ids: [], at: a.now }, { page: id });
    }
    function renamePage(doc, a) { const page = pageOf(doc, a.pageId), name = Array.from(String(a.name || '').trim()).slice(0, 40).join(''); if (!page || !name || name === page.name) return null;
        return make('homedash.cmd.page_rename', [D.op(doc, ['pages', page.id, 'name'], name)], { kind: 'page', ids: [], at: a.now, coalesce: 'page|' + page.id + '|name', window: FIELD_MS }); }
    function setPageIcon(doc, a) { const page = pageOf(doc, a.pageId); if (!page || page.icon === a.icon) return null; return make('homedash.cmd.page_icon', [D.op(doc, ['pages', page.id, 'icon'], a.icon || undefined)], { kind: 'page', ids: [], at: a.now }); }
    function duplicatePage(doc, a) {
        const src = pageOf(doc, a.pageId); if (!src) return null;
        if (doc.pages.length >= LIMITS.pages || total(doc) + src.widgets.length > LIMITS.widgets) return { refused: 'limit' };
        const used = usedIds(doc), id = fresh(used, 'p', 6), map = new Map(src.widgets.map(w => [w.id, fresh(used, 'w', 8)]));
        const remap = arr => (Array.isArray(arr) ? arr.map(i => Object.assign({}, i, { id: map.get(i.id) })) : null);
        const page = Object.assign(clone(src), { id, name: Array.from(String(a.name || src.name)).slice(0, 40).join(''), widgets: src.widgets.map(w => Object.assign(clone(w), { id: map.get(w.id) })), layouts: { lg: remap(src.layouts.lg), md: remap(src.layouts.md), sm: remap(src.layouts.sm) } });
        return make('homedash.cmd.page_duplicate', [{ path: ['pages', id], before: undefined, after: page, index: indexOf(doc, src.id) + 1 }], { kind: 'page', ids: [], at: a.now }, { page: id });
    }
    function movePage(doc, a) {
        const i = indexOf(doc, a.pageId); if (i < 0) return null; const j = clamp(Math.round(a.to), 0, doc.pages.length - 1); if (i === j) return null; const page = clone(doc.pages[i]);
        return make('homedash.cmd.page_move', [{ path: ['pages', page.id], before: page, after: undefined, index: i }, { path: ['pages', page.id], before: undefined, after: page, index: j }], { kind: 'page', ids: [], at: a.now }, { page: page.id });
    }
    function deletePage(doc, a) {
        const i = indexOf(doc, a.pageId); if (i < 0) return null; if (doc.pages.length <= 1) return { refused: 'last_page' };
        const page = doc.pages[i], rest = doc.pages.filter(p => p.id !== page.id), first = rest[0].id, ops = [{ path: ['pages', page.id], before: clone(page), after: undefined, index: i }];
        if (doc.kiosk && doc.kiosk.start_page === page.id) ops.push(D.op(doc, ['kiosk', 'start_page'], first));
        for (const p of rest) for (const w of p.widgets) {
            for (const k of ['tap', 'hold']) { const act = w.actions && w.actions[k]; if (act && act.do === 'navigate' && act.page === page.id) ops.push(D.op(doc, ['pages', p.id, 'widgets', w.id, 'actions', k], { do: 'default' })); }
            if (w.options && w.options.page === page.id) ops.push(D.op(doc, ['pages', p.id, 'widgets', w.id, 'options', 'page'], first));
        }
        return make('homedash.cmd.page_delete', ops, { kind: 'page', ids: page.widgets.map(w => w.id), at: a.now }, { page: rest[Math.max(0, i - 1)].id });
    }
    function setPageCompact(doc, a) {
        const page = pageOf(doc, a.pageId); if (!page || compactOf(page) === !!a.compact) return null; const ops = [D.op(doc, ['pages', page.id, 'compact'], !!a.compact)];
        if (a.compact) for (const bp of ['lg', 'md', 'sm']) { const arr = page.layouts[bp]; if (!Array.isArray(arr)) continue; const next = G.compact(arr); if (D.keyOf(next) !== D.keyOf(arr)) ops.push(D.op(doc, ['pages', page.id, 'layouts', bp], next)); }
        return make('homedash.cmd.page_compact', ops, { kind: 'page', ids: [], at: a.now });
    }
    function setDoc(doc, a) { if (D.keyOf(D.getAt(doc, a.path)) === D.keyOf(a.value)) return null;
        return make(a.label || 'homedash.cmd.look', [D.op(doc, a.path, a.value)], { kind: 'doc', ids: [], at: a.now, coalesce: a.text ? 'doc|' + a.path.join('.') : undefined, window: a.text ? FIELD_MS : undefined }); }
    function entry(catalog, type, id, entityId) {
        const v = catalog && catalog.has(type) ? catalog.variant(type, id) : null; if (!v) return null;
        const widget = { type, variant: v.id, title: '', bindings: entityId ? { primary: entityId } : {}, options: {}, actions: {}, confirm: false };
        return { widget, size: catalog.defaultSize(widget) };
    }
    const ARROWS = Object.freeze({ ArrowLeft: [-1, 0, 'left'], ArrowRight: [1, 0, 'right'], ArrowUp: [0, -1, 'up'], ArrowDown: [0, 1, 'down'] });
    function keymap(ev, c) {
        const x = c || {}; if (!ev || x.typing || ev.isComposing) return null;
        const key = String(ev.key || ''), low = key.toLowerCase(), mod = !!(ev.ctrlKey || ev.metaKey), alt = !!ev.altKey, shift = !!ev.shiftKey, arrow = ARROWS[key];
        if (mod && alt && (key === 'PageUp' || key === 'PageDown')) return { action: key === 'PageUp' ? 'page_prev' : 'page_next' };
        if (mod && !alt) {
            if (low === 'z') return { action: shift ? 'redo' : 'undo' }; if (low === 'y' && !shift) return { action: 'redo' };
            if (!x.canvas || shift) return null;
            return { c: { action: 'copy' }, x: { action: 'cut' }, v: { action: 'paste' }, d: { action: 'duplicate' }, a: { action: 'select_all' }, ' ': { action: 'toggle_focused' } }[low] || null;
        }
        if (key === 'Escape' && !alt && !shift) return { action: 'escape' };
        if (!x.canvas) return null;
        if (arrow && alt && !shift) return { action: 'focus_dir', dir: arrow[2] };
        if (arrow && !alt) return shift ? { action: 'resize_step', dw: arrow[0], dh: arrow[1] } : { action: 'nudge', dx: arrow[0], dy: arrow[1] };
        if (alt) return null;
        if ((key === 'Delete' || key === 'Backspace') && !shift) return { action: 'delete' };
        if (key === ' ' && !shift) return { action: 'select_focused' };
        if ((key === 'Enter' && !shift) || key === 'F2') return { action: 'focus_inspector' };
        if (key === 'Tab') return { action: shift ? 'focus_prev' : 'focus_next' };
        return null;
    }
    function spatial(items, fromId, dir) {
        const a = items.find(i => i.id === fromId); if (!a) return null; const cx = a.x + a.w / 2, cy = a.y + a.h / 2; let best = null, score = Infinity;
        for (const b of items) { if (b.id === fromId) continue; const dx = b.x + b.w / 2 - cx, dy = b.y + b.h / 2 - cy;
            const along = dir === 'right' ? dx : dir === 'left' ? -dx : dir === 'down' ? dy : -dy, across = dir === 'left' || dir === 'right' ? Math.abs(dy) : Math.abs(dx);
            if (along <= 0) continue; const s = along + 2 * across; if (s < score || (s === score && b.id < best)) { score = s; best = b.id; } }
        return best;
    }
    const readingOrder = items => G.sortReading(items).map(i => i.id);
    E.commands = Object.freeze({ NUDGE_MS, FIELD_MS, LIMITS, layout, nudge, resizeStep, add, remove, duplicate, paste, setWidget, bind, variant, applyFamily, align, resetLayout,
        addPage, renamePage, setPageIcon, duplicatePage, movePage, deletePage, setPageCompact, setDoc, entry, keymap, spatial, readingOrder });
})();
