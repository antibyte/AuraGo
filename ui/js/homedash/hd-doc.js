// HomeDash documents (pure): normalise/repair, id-keyed patch ops (apply/invert), command builders, clipboard.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.layout) throw new Error('hd-doc.js: load hd-layout.js first');
    const { clone, randomId, clamp } = E.core, COLS = E.layout.COLS;
    const isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v), pick = (v, l, d) => (l.includes(v) ? v : d);
    const WIDGET_DEFAULTS = { variant: '', title: '', bindings: {}, options: {}, actions: {}, confirm: false };
    function normalisePage(raw, index, sizeOf) {
        const p = isObj(raw) ? raw : {}, seen = new Set();
        const widgets = (Array.isArray(p.widgets) ? p.widgets : []).filter(w => isObj(w) && typeof w.id === 'string' && typeof w.type === 'string' && !seen.has(w.id) && seen.add(w.id)).map(w => Object.assign({}, WIDGET_DEFAULTS, clone(w)));
        const ids = new Set(widgets.map(w => w.id)), L = isObj(p.layouts) ? p.layouts : {};
        let layouts = { lg: E.grid.repair(L.lg, COLS.lg, null, { ids, compact: p.compact !== false }), md: null, sm: null };
        for (const bp of ['md', 'sm']) if (Array.isArray(L[bp])) layouts[bp] = E.grid.repair(L[bp], COLS[bp], null, { ids, compact: true });
        for (const w of widgets) if (!layouts.lg.some(i => i.id === w.id)) layouts = E.layout.addToLayouts(layouts, Object.assign({ id: w.id }, sizeOf(w)), null);
        for (const bp of ['md', 'sm']) if (Array.isArray(layouts[bp])) for (const w of widgets) if (!layouts[bp].some(i => i.id === w.id)) {
            const s = E.layout.derivedSize(layouts.lg.find(i => i.id === w.id), COLS[bp], {}), q = E.grid.place(layouts[bp], s.w, s.h, COLS[bp]);
            layouts[bp] = layouts[bp].concat([{ id: w.id, x: q.x, y: q.y, w: s.w, h: s.h }]);
        }
        return { id: typeof p.id === 'string' && p.id ? p.id : index === 0 ? 'main' : 'p' + (index + 1), name: typeof p.name === 'string' ? p.name : '', icon: typeof p.icon === 'string' ? p.icon : 'home', compact: p.compact !== false, widgets, layouts };
    }
    function normalise(raw, opts) {
        const d = isObj(raw) ? raw : {}, look = isObj(d.look) ? d.look : {}, grid = isObj(d.grid) ? d.grid : {};
        const sizeOf = (opts && opts.sizeOf) || (() => ({ w: 2, h: 1 }));
        const palette = isObj(look.palette) ? clone(look.palette) : {}; palette.mode = pick(palette.mode, ['theme', 'dark', 'light', 'custom'], 'theme');
        const pages = (Array.isArray(d.pages) && d.pages.length ? d.pages : [{}]).map((p, i) => normalisePage(p, i, sizeOf)), used = new Set();
        for (const p of pages) { while (used.has(p.id)) p.id += '_'; used.add(p.id); }
        return Object.assign({}, clone(d), {
            schema_version: 1,
            look: Object.assign({}, clone(look), { family: pick(look.family, ['glass', 'tactile', 'minimal', 'instrument'], 'glass'), palette, backdrop: isObj(look.backdrop) ? clone(look.backdrop) : { kind: 'theme' }, density: pick(look.density, ['compact', 'comfortable', 'spacious'], 'comfortable'), radius_scale: clamp(Number(look.radius_scale) || 1, 0.5, 1.5) }),
            grid: { row_unit: grid.row_unit === undefined || grid.row_unit === 'auto' ? 'auto' : clamp(Number(grid.row_unit) || 96, 48, 200), gap: typeof grid.gap === 'number' ? grid.gap : null, fit: grid.fit === 'fit_screen' ? 'fit_screen' : 'scroll' },
            pages
        });
    }
    const keyOf = path => JSON.stringify(path);
    const locate = (arr, seg) => (typeof seg === 'number' ? seg : arr.findIndex(x => isObj(x) && x.id === seg));
    function getAt(doc, path) { let n = doc; for (const seg of path) { if (n == null) return undefined; if (Array.isArray(n)) { const i = locate(n, seg); n = i >= 0 ? n[i] : undefined; } else n = n[seg]; } return n; }
    function setAt(node, path, value, index) {
        const [seg, ...rest] = path;
        if (Array.isArray(node)) {
            const copy = node.slice(), i = locate(copy, seg);
            if (!rest.length) {
                if (value === undefined) { if (i >= 0) copy.splice(i, 1); }
                else if (i >= 0) copy[i] = clone(value);
                else copy.splice(index === undefined ? copy.length : clamp(index, 0, copy.length), 0, clone(value));
                return copy;
            }
            if (i < 0) throw new Error('hd-doc: path not found at ' + seg);
            copy[i] = setAt(copy[i], rest, value, index); return copy;
        }
        if (!isObj(node)) throw new Error('hd-doc: path not found at ' + seg);
        const copy = Object.assign({}, node);
        if (!rest.length) { if (value === undefined) delete copy[seg]; else copy[seg] = clone(value); return copy; }
        if (!(seg in copy)) throw new Error('hd-doc: path not found at ' + seg);
        copy[seg] = setAt(copy[seg], rest, value, index); return copy;
    }
    function apply(doc, ops, dir) { const side = dir === 'before' ? 'before' : 'after'; let out = doc;
        for (const op of side === 'before' ? ops.slice().reverse() : ops) out = setAt(out, op.path, op[side], op.index); return out; }
    const invert = ops => ops.slice().reverse().map(op => Object.assign({}, op, { before: op.after, after: op.before }));
    const command = (label, ops, meta) => ({ label, ops, meta: Object.assign({ at: Date.now() }, meta) });
    const op = (doc, path, after, index) => ({ path, before: clone(getAt(doc, path)), after: clone(after), index });
    function pageOf(doc, id) { const p = getAt(doc, ['pages', id]); if (!p) throw new Error('hd-doc: unknown page ' + id); return p; }
    const setField = (doc, path, value, meta) => command('set', [op(doc, path, value)], Object.assign({ kind: 'set' }, meta));
    function layoutOps(doc, pageId, next) { const page = pageOf(doc, pageId), ops = [];
        for (const bp of ['lg', 'md', 'sm']) if (keyOf(page.layouts[bp]) !== keyOf(next[bp])) ops.push(op(doc, ['pages', pageId, 'layouts', bp], next[bp])); return ops; }
    function addWidget(doc, pageId, widget, size, C) {
        const page = pageOf(doc, pageId), w = Object.assign({}, WIDGET_DEFAULTS, clone(widget)); if (!w.id) w.id = randomId('w', 8);
        return command('add', [op(doc, ['pages', pageId, 'widgets', w.id], w), ...layoutOps(doc, pageId, E.layout.addToLayouts(page.layouts, { id: w.id, w: size.w, h: size.h }, C))], { kind: 'add', ids: [w.id] });
    }
    function removeWidgets(doc, pageId, ids) {
        const page = pageOf(doc, pageId), list = [].concat(ids);
        const order = list.map(id => [id, page.widgets.findIndex(w => w.id === id)]).filter(e => e[1] >= 0).sort((a, b) => a[1] - b[1]);
        const ops = order.map(([id, i], k) => ({ path: ['pages', pageId, 'widgets', id], before: clone(page.widgets[i]), after: undefined, index: i - k }));
        return command('remove', ops.concat(layoutOps(doc, pageId, E.layout.removeFromLayouts(page.layouts, list))), { kind: 'remove', ids: list });
    }
    function setLayout(doc, pageId, bp, layout, meta) { const page = pageOf(doc, pageId), next = Object.assign({}, page.layouts); next[bp] = layout;
        return command('layout', layoutOps(doc, pageId, next), Object.assign({ kind: 'layout' }, meta)); }
    function toClipboard(doc, pageId, ids) { const page = pageOf(doc, pageId), set = new Set(ids);
        return JSON.stringify({ v: 1, widgets: page.widgets.filter(w => set.has(w.id)).map(w => { const c = clone(w), l = page.layouts.lg.find(i => i.id === w.id) || { w: 2, h: 1 }; delete c.id; return { widget: c, w: l.w, h: l.h }; }) }); }
    function fromClipboard(text) {
        let data; try { data = JSON.parse(text); } catch (err) { return []; }
        if (!isObj(data) || data.v !== 1 || !Array.isArray(data.widgets)) return [];
        return data.widgets.filter(e => isObj(e) && isObj(e.widget) && typeof e.widget.type === 'string').map(e => ({ widget: Object.assign(clone(e.widget), { id: randomId('w', 8) }), w: clamp(Math.round(Number(e.w) || 2), 1, 12), h: clamp(Math.round(Number(e.h) || 1), 1, 24) }));
    }
    E.doc = Object.freeze({ normalise, keyOf, getAt, setAt, apply, invert, command, op, setField, addWidget, removeWidgets, setLayout, toClipboard, fromClipboard });
})();
