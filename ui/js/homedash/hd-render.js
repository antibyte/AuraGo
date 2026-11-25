// HomeDash page renderer (F2; design-frontend 2.3, 4.4, 9.1, 10). page(opts) renders one page of a normalised dashboard into the stage:
// div.hd-page with one widget host per widget, placed by --hd-x/--hd-y/--hd-w/--hd-h (homedash-page.css). Rules: options = catalog defaults
// under the document's; warn/alert without zones get E.tone.zones; entity() adds the host tone (E.tone.of); can() is false and command()
// rejects (view only in M2); history() honours options.hours. Unregistered types become a labelled .hd-unsupported cell. Layout runs once at
// creation and then in one animation frame per stage resize; store notices update only bound widgets; off-screen widgets update when shown.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.widgets || !E.layout || !E.tone || !E.api || !E.prim) throw new Error('hd-render.js: load the engine, hd-tone.js and hd-api.js first');
    const { el, setVars, setAttr } = E.core, ENTITY_ID = /^[a-z][a-z0-9_]*:\S+$/;
    const isEntityId = v => typeof v === 'string' && ENTITY_ID.test(v);
    const viewOnly = () => Promise.reject(E.api.normalizeError({ status: 0, body: { code: 'view_only', message: 'HomeDash M2 sends no commands' } }));
    function page(opts) {
        const o = opts, doc = o.doc, pg = o.page, store = o.store, catalog = o.catalog, t = o.t, lang = o.lang || 'en', family = doc.look && doc.look.family;
        const measure = o.measure || (node => ({ width: node.clientWidth, height: node.clientHeight }));
        const pageEl = el('div', { class: 'hd-page', id: o.id || null, role: o.role || 'region', 'aria-label': o.label || null, 'aria-labelledby': o.labelledBy || null });
        o.stage.appendChild(pageEl);
        const recs = new Map(), byEntity = new Map(), byEl = new Map(), limits = new Map();
        let bp = null, destroyed = false;
        const lgItem = id => (pg.layouts.lg || []).find(i => i.id === id) || { w: 1, h: 1 };
        function api(rec) {
            const idOf = binding => { const id = rec.widget.bindings && rec.widget.bindings[binding]; return isEntityId(id) ? id : null; };
            return () => ({
                entity(binding) { const id = idOf(binding), v = id ? store.view(id) : null; return v ? Object.assign({}, v, { tone: E.tone.of(v, rec.widget.options) }) : null; },
                can: () => false, command: viewOnly, moreInfo() {},
                history(binding, range) {
                    const id = idOf(binding); if (!id) return Promise.resolve({ points: [], source: 'none' });
                    const hours = Number(rec.widget.options.hours);
                    return store.history(id, Number.isFinite(hours) && hours > 0 ? hours * 3600000 : range, 120);
                },
                announce(text) { if (typeof o.announce === 'function') o.announce(String(text)); }
            });
        }
        function unsupported(widget) {
            const frame = E.prim.frame({ title: widget.title || widget.type });
            frame.set({ stateText: t('homedash.view.unsupported') });
            return el('div', { class: 'hd-widget hd-unsupported', 'data-hd-type': widget.type, 'data-hd-variant': widget.variant || null, 'data-hd-family': widget.family || family || null,
                'data-hd-status': 'unknown', role: 'group', 'aria-labelledby': frame.titleId }, frame.el);
        }
        function mount(widget) {
            const options = Object.assign({}, catalog.options(widget));
            if ((options.warn != null || options.alert != null) && !Array.isArray(options.zones)) options.zones = E.tone.zones(options);
            const rec = { widget: Object.assign({}, widget, { options }), ids: Object.values(widget.bindings || {}).filter(isEntityId), inst: null, el: null, cell: '', px: '', visible: true, dirty: false };
            limits.set(widget.id, catalog.constraints(widget));
            const item = lgItem(widget.id);
            try { rec.inst = E.widgets.host({ widget: rec.widget, mode: 'view', size: { w: item.w, h: item.h }, t, lang, parent: pageEl, api: api(rec) }); rec.el = rec.inst.el; }
            catch (err) { console.warn('HomeDash: widget ' + widget.id + ' is not rendered', err); rec.inst = null; rec.el = unsupported(widget); pageEl.appendChild(rec.el); }
            setAttr(rec.el, 'data-hd-wid', widget.id);
            recs.set(widget.id, rec); byEl.set(rec.el, rec);
            for (const id of rec.ids) { if (!byEntity.has(id)) byEntity.set(id, new Set()); byEntity.get(id).add(rec); }
        }
        for (const w of pg.widgets) mount(w);
        if (!pg.widgets.length) pageEl.appendChild(el('div', { class: 'hd-page-empty', role: 'note' }, [E.icons.icon('dashboard'),
            el('p', { class: 'hd-page-empty-title', text: t('homedash.view.empty_title') }), el('p', { class: 'hd-page-empty-text', text: t('homedash.view.empty_text') })]));
        function layout() {
            if (destroyed) return;
            const box = measure(o.stage);
            if (!box || !(box.width > 0)) return;
            bp = E.layout.breakpoint(box.width, bp);
            const cols = E.layout.COLS[bp], items = E.layout.effective(pg.layouts, bp, id => limits.get(id)), rows = Math.max(1, E.layout.rows(items));
            const gap = E.layout.gapFor(doc.look.density, doc.grid.gap), m = E.layout.metrics(box.width, box.height || 0, cols, rows, gap, doc.grid.row_unit, doc.grid.fit);
            setVars(pageEl, E.layout.cssVars(m, cols, rows, gap));
            if (typeof o.onLayout === 'function') o.onLayout({ bp, fit: m.mode });
            for (const it of items) {
                const rec = recs.get(it.id); if (!rec) continue;
                const cell = [it.x, it.y, it.w, it.h].join(',');
                if (cell !== rec.cell) { rec.cell = cell; setVars(rec.el, { '--hd-x': it.x, '--hd-y': it.y, '--hd-w': it.w, '--hd-h': it.h }); }
                const px = { width: Math.round(it.w * m.colW + (it.w - 1) * gap), height: Math.round(it.h * m.rowH + (it.h - 1) * gap) }, key = cell + ':' + px.width + 'x' + px.height;
                if (rec.inst && key !== rec.px) { rec.px = key; rec.inst.resize({ w: it.w, h: it.h, px }); }
            }
        }
        const schedule = E.core.frame(layout, { raf: o.raf, caf: o.caf });
        function onChange(ids) {
            if (destroyed) return;
            const targets = new Set();
            if (ids === null) for (const r of recs.values()) targets.add(r); else for (const id of ids) for (const r of byEntity.get(id) || []) targets.add(r);
            for (const r of targets) { if (!r.inst) continue; if (r.visible) r.inst.update(); else r.dirty = true; }
        }
        store.on('change', onChange);
        const IO = o.IntersectionObserver || (typeof IntersectionObserver === 'function' ? IntersectionObserver : null);
        const io = IO ? new IO(entries => { for (const en of entries) { const r = byEl.get(en.target); if (!r) continue; r.visible = !!en.isIntersecting; if (r.visible && r.dirty && r.inst) { r.dirty = false; r.inst.update(); } } }, { root: o.stage, rootMargin: '200px' }) : null;
        if (io) for (const r of recs.values()) io.observe(r.el);
        const RO = o.ResizeObserver || (typeof ResizeObserver === 'function' ? ResizeObserver : null);
        const ro = RO ? new RO(() => schedule()) : null;
        if (ro) ro.observe(o.stage);
        layout();
        function destroy() {
            if (destroyed) return; destroyed = true; schedule.cancel(); store.off('change', onChange);
            if (ro) ro.disconnect(); if (io) io.disconnect();
            for (const r of recs.values()) if (r.inst) r.inst.destroy();
            pageEl.remove(); recs.clear(); byEntity.clear(); byEl.clear();
        }
        return Object.freeze({ el: pageEl, entityIds: () => [...byEntity.keys()], layout: () => schedule(), refreshAll: () => onChange(null), destroy });
    }
    E.render = Object.freeze({ page, isEntityId });
})();
