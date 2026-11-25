// HomeDash page renderer (F2; design-frontend 2.3, 4.4, 9.1, 10). page(opts) renders one page of a normalised dashboard into the stage:
// div.hd-page with one widget host per widget, placed by --hd-x/--hd-y/--hd-w/--hd-h (homedash-page.css). Rules: options = catalog defaults
// under the document's; warn/alert without zones get E.tone.zones; entity() adds the host tone (E.tone.of); without opts.commands can() is false and
// command() rejects (view only, the M2 behaviour); with it entity() carries the command overlay, can() follows the capabilities, command() sends through
// the shared client (widget key 'w|doc|widget') and moreInfo() opens the light sheet (opts.moreInfo); history() honours options.hours. Unregistered types become a labelled .hd-unsupported cell. Layout runs once at
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
        // The built-in system source names its entities in English; the six known ids get a localized name (presentation only).
        const SYSTEM_NAMES = Object.freeze({
            'system:cpu': () => t('homedash.system.cpu'), 'system:memory': () => t('homedash.system.memory'),
            'system:disk': () => t('homedash.system.disk'), 'system:net_rx': () => t('homedash.system.net_rx'),
            'system:net_tx': () => t('homedash.system.net_tx'), 'system:uptime': () => t('homedash.system.uptime')
        });
        const systemName = id => (Object.prototype.hasOwnProperty.call(SYSTEM_NAMES, id) ? SYSTEM_NAMES[id]() : null);
        const lgItem =id => (pg.layouts.lg || []).find(i => i.id === id) || { w: 1, h: 1 };
        const commands = o.commands || null, docId = doc.id || '', sheetSubs = new Map();
        // decorated(): the store view plus tone and the localized system name; with commands also the overlay, cue, lock and arming (decorate) and the cue text.
        function decorated(rec, id, key) {
            const v = store.view(id); if (!v) return null;
            const base = Object.assign({}, v, { tone: E.tone.of(v, rec.widget.options) }), sys = v.source === 'system' ? systemName(id) : null; if (sys) base.name = sys;
            if (!commands) return base;
            const out = commands.decorate(id, Object.freeze(base), key);
            return out && out.error && !out.error.text ? Object.freeze(Object.assign({}, out, { error: Object.freeze(Object.assign({}, out.error, { text: E.control.cueText(t, out.error) })) })) : out;
        }
        function canFor(id, capName, mode) {
            if (!commands || mode !== 'view' || !id) return false;
            const v = store.view(id), d = v && commands.decorate(id, v, null);
            if (!d || d.available === false || d.status === 'unknown' || d.status === 'unavailable' || d.readonly) return false;
            const c = E.control.cap(d, capName); return !!c && c.allowed !== false;
        }
        function subscribe(id, fn) { let set = sheetSubs.get(id); if (!set) { set = new Set(); sheetSubs.set(id, set); } set.add(fn); return () => { set.delete(fn); if (!set.size) sheetSubs.delete(id); }; }
        // Where focus returns after the sheet: the focused element inside the widget, else its visible more button, else the widget itself (tabindex -1 only
        // for that, dropped again when it loses focus). A pointer hold on a non-focusable part leaves body as the active element.
        function openerFor(rec) {
            const doc = o.stage.ownerDocument, root = rec.el, act = doc ? doc.activeElement : null;
            if (!root) return act || null;
            if (act && root.contains(act)) return act;
            const more = root.querySelector('.hd-more'); if (more && !more.hasAttribute('hidden')) return more;
            if (!root.hasAttribute('tabindex')) { root.setAttribute('tabindex', '-1'); root.addEventListener('blur', () => root.removeAttribute('tabindex'), { once: true }); }
            return root;
        }
        function apiFor(rec, key, mode, onlyId) {   // the widget api, also used for the sheet (key 'sheet|...')
            const idOf = binding => { if (onlyId) return onlyId; const id = rec.widget.bindings && rec.widget.bindings[binding]; return isEntityId(id) ? id : null; };
            return {
                widget: rec.widget, t, lang,
                entity: binding => { const id = idOf(binding); return id ? decorated(rec, id, key) : null; },
                can: (binding, capName) => canFor(idOf(binding), capName, mode),
                command(binding, command, args, copts) {
                    const id = idOf(binding); if (!commands) return viewOnly(); if (!canFor(id, command, mode)) return Promise.resolve({ status: 'refused' });
                    return commands.send({ entity: id, command, args: args || {}, phase: copts && copts.phase, widgetKey: key, widgetId: rec.widget.id, dashboardId: docId, confirm: rec.widget.confirm === true });
                },
                moreInfo(binding) {
                    const id = idOf(binding), v = id ? store.view(id) : null;
                    if (!commands || !v || v.kind !== 'light' || typeof o.moreInfo !== 'function') return false;
                    o.moreInfo({ family: rec.inst && rec.inst.family ? rec.inst.family : family, opener: openerFor(rec),
                        api: apiFor(rec, 'sheet|' + docId + '|' + rec.widget.id, 'view', id), subscribe: fn => subscribe(id, fn) });
                    return true;
                },
                history(binding, range) {
                    const id = idOf(binding); if (!id) return Promise.resolve({ points: [], source: 'none' });
                    const hours = Number(rec.widget.options.hours);
                    return store.history(id, Number.isFinite(hours) && hours > 0 ? hours * 3600000 : range, 120);
                },
                announce(text) { if (typeof o.announce === 'function') o.announce(String(text)); }
            };
        }
        const api = rec => { const key = 'w|' + docId + '|' + rec.widget.id; return a => apiFor(rec, key, a && a.mode, null); };
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
            try { rec.inst = E.widgets.host({ widget: rec.widget, mode: 'view', size: { w: item.w, h: item.h }, t, lang, parent: pageEl, api: api(rec),
                moreLabel: commands ? t('homedash.moreinfo.open') : null, moreWhen: commands ? p => !!p && p.kind === 'light' : null, titleFallback: commands ? () => (rec.ids[0] && systemName(rec.ids[0])) || '' : null }); rec.el = rec.inst.el; }
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
            for (const [id, set] of [...sheetSubs]) if (ids === null || (typeof ids.has === 'function' ? ids.has(id) : ids.includes(id))) for (const fn of [...set]) fn();
        }
        store.on('change', onChange);
        // Command overlay changes (pending, cue, arming, lock) re-render bound widgets and the open sheet in one frame; cues and armings are announced.
        const pendingIds = new Set(), flushCmd = E.core.frame(() => { const ids = [...pendingIds]; pendingIds.clear(); onChange(ids); }, { raf: o.raf, caf: o.caf });
        const ownKey = key => typeof key === 'string' && (key.startsWith('w|' + docId + '|') || key.startsWith('sheet|' + docId + '|'));
        const cmdOff = [];
        if (commands) {
            const sub = (type, fn) => { commands.on(type, fn); cmdOff.push(() => commands.off(type, fn)); };
            sub('change', ids => { for (const id of ids || []) pendingIds.add(id); flushCmd(); });
            sub('cue', ev => { if (ev && byEntity.has(ev.id) && typeof o.announce === 'function') o.announce(E.control.cueText(t, ev.cue)); });
            sub('armed', ev => { if (ev && byEntity.has(ev.id) && ownKey(ev.widgetKey) && typeof o.announce === 'function') o.announce(t('homedash.arm.announce')); });
        }
        const IO = o.IntersectionObserver || (typeof IntersectionObserver === 'function' ? IntersectionObserver : null);
        const io = IO ? new IO(entries => { for (const en of entries) { const r = byEl.get(en.target); if (!r) continue; r.visible = !!en.isIntersecting; if (r.visible && r.dirty && r.inst) { r.dirty = false; r.inst.update(); } } }, { root: o.stage, rootMargin: '200px' }) : null;
        if (io) for (const r of recs.values()) io.observe(r.el);
        const RO = o.ResizeObserver || (typeof ResizeObserver === 'function' ? ResizeObserver : null);
        const ro = RO ? new RO(() => schedule()) : null;
        if (ro) ro.observe(o.stage);
        layout();
        function destroy() {
            if (destroyed) return; destroyed = true; schedule.cancel(); flushCmd.cancel(); store.off('change', onChange);
            for (const off of cmdOff.splice(0)) off();
            if (commands && E.popover) { const rootEl = o.stage.closest ? o.stage.closest('.hd-root') : null; if (rootEl) E.popover.close(rootEl); }
            if (ro) ro.disconnect(); if (io) io.disconnect();
            for (const r of recs.values()) if (r.inst) r.inst.destroy();
            pageEl.remove(); recs.clear(); byEntity.clear(); byEl.clear(); sheetSubs.clear();
        }
        return Object.freeze({ el: pageEl, entityIds: () => [...byEntity.keys()], layout: () => schedule(), refreshAll: () => onChange(null), destroy });
    }
    E.render = Object.freeze({ page, isEntityId });
})();
