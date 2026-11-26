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
    // Forced preview widths (design mode, C13): lg max(W, 960), md clamp(W, 600, 768), sm 390.
    const PREVIEW = Object.freeze({ lg: w => Math.max(w, E.layout.BP_MIN.lg), md: w => Math.min(768, Math.max(w, E.layout.BP_MIN.md)), sm: () => 390 });
    // Zoom to fit (design mode): the page keeps the preview width (WYSIWYG metrics) and is scaled with the CSS zoom property to the stage's inner width,
    // never below ZOOM_MIN (then the stage scrolls). Chrome 128+ (standardised zoom, measured on Chrome 158): getBoundingClientRect and pointer clientX/Y are
    // in viewport px (zoom applied), offsetWidth, clientWidth, computed width and every CSS length inside the page are unzoomed layout px, and a percentage
    // size resolves visually (min-block-size: 100% fills the stage). So a page-local coordinate is (client - rect.left) / zoom; hd-gestures does that in one place.
    // The value is floored at 3 decimals so the scaled page never overflows the stage by a rounding pixel.
    const ZOOM_MIN = 0.5;
    const zoomFor = (avail, width) => (avail > 0 && width > avail ? Math.floor(Math.max(ZOOM_MIN, avail / width) * 1000) / 1000 : 1);
    // The stage's inner box (design mode gives the stage padding so the outline, handles and chip of an edge widget stay visible): client size minus the padding.
    // The layout metrics and the zoom both work on this box, so the page never overflows the stage sideways.
    const contentBox = (node, cs) => { const px = k => (cs && Number.isFinite(parseFloat(cs[k])) ? parseFloat(cs[k]) : 0);
        return { width: Math.max(0, node.clientWidth - px('paddingLeft') - px('paddingRight')), height: Math.max(0, node.clientHeight - px('paddingTop') - px('paddingBottom')) }; };
    function page(opts) {
        const o = opts, store = o.store, catalog = o.catalog, t = o.t, lang = o.lang || 'en';
        let doc = o.doc, pg = o.page, family = doc.look && doc.look.family;
        let mode = o.mode === 'design' ? 'design' : 'view', forcedBp = o.bp || null, fitOverride = o.fit || null, last = null, emptyAction = o.emptyAction || null, emptyKey = null, emptyEl = null;
        // Only the design stage has padding (the inset of the page); the view stage is read as it is.
        const measure = o.measure || (node => contentBox(node, mode === 'design' && typeof getComputedStyle === 'function' ? getComputedStyle(node) : null));
        const pageEl = el('div', { class: 'hd-page', id: o.id || null });
        o.stage.appendChild(pageEl);
        const recs = new Map(), byEntity = new Map(), byEl = new Map(), limits = new Map();
        let bp = null, destroyed = false, io = null;
        function pageAttrs() {   // design: a multi-select listbox named by the document; view: the region or tabpanel of M2
            const design = mode === 'design';
            setAttr(pageEl, 'role', design ? 'listbox' : o.role || 'region'); setAttr(pageEl, 'aria-multiselectable', design ? true : null);
            setAttr(pageEl, 'aria-label', design ? o.label || doc.name || null : o.label || null); setAttr(pageEl, 'aria-labelledby', design ? null : o.labelledBy || null);
        }
        pageAttrs();
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
        function apiFor(rec, key, modeOf, onlyId) {   // the widget api, also used for the sheet (key 'sheet|...'); modeOf is a mode or a function that reads it live
            const now = typeof modeOf === 'function' ? modeOf : () => modeOf;
            const idOf = binding => { if (onlyId) return onlyId; const id = rec.widget.bindings && rec.widget.bindings[binding]; return isEntityId(id) ? id : null; };
            return {
                widget: rec.widget, t, lang,
                entity: binding => { const id = idOf(binding); return id ? decorated(rec, id, key) : null; },
                can: (binding, capName) => canFor(idOf(binding), capName, now()),
                command(binding, command, args, copts) {
                    const id = idOf(binding); if (!commands) return viewOnly(); if (!canFor(id, command, now())) return Promise.resolve({ status: 'refused' });
                    return commands.send({ entity: id, command, args: args || {}, phase: copts && copts.phase, widgetKey: key, widgetId: rec.widget.id, dashboardId: docId, confirm: rec.widget.confirm === true });
                },
                moreInfo(binding) {
                    const id = idOf(binding), v = id ? store.view(id) : null;
                    if (now() !== 'view' || !commands || !v || v.kind !== 'light' || typeof o.moreInfo !== 'function') return false;
                    o.moreInfo({ family: rec.inst && rec.inst.family ? rec.inst.family : family, opener: openerFor(rec),
                        api: apiFor(rec, 'sheet|' + docId + '|' + rec.widget.id, () => rec.mode, id), subscribe: fn => subscribe(id, fn) });
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
        const api = rec => { const key = 'w|' + docId + '|' + rec.widget.id; return () => apiFor(rec, key, () => rec.mode, null); };
        function unsupported(widget) {
            const frame = E.prim.frame({ title: widget.title || widget.type });
            frame.set({ stateText: t('homedash.view.unsupported') });
            return el('div', { class: 'hd-widget hd-unsupported', 'data-hd-type': widget.type, 'data-hd-variant': widget.variant || null, 'data-hd-family': widget.family || family || null,
                'data-hd-status': 'unknown', role: 'group', 'aria-labelledby': frame.titleId }, frame.el);
        }
        function mount(widget) {
            const options = Object.assign({}, catalog.options(widget));
            if ((options.warn != null || options.alert != null) && !Array.isArray(options.zones)) options.zones = E.tone.zones(options);
            const rec = { widget: Object.assign({}, widget, { options }), ids: Object.values(widget.bindings || {}).filter(isEntityId), inst: null, el: null, cell: '', px: '', visible: true, dirty: false,
                mode, key: E.doc.keyOf(widget) + '|' + (widget.family || family || ''), labelledBy: null, label: '' };
            limits.set(widget.id, catalog.constraints(widget));
            const item = lgItem(widget.id);
            try { rec.inst = E.widgets.host({ widget: rec.widget, mode, size: { w: item.w, h: item.h }, t, lang, parent: pageEl, api: api(rec),
                moreLabel: commands ? t('homedash.moreinfo.open') : null, moreWhen: commands ? p => !!p && p.kind === 'light' : null, titleFallback: commands ? () => (rec.ids[0] && systemName(rec.ids[0])) || '' : null }); rec.el = rec.inst.el; }
            catch (err) { console.warn('HomeDash: widget ' + widget.id + ' is not rendered', err); rec.inst = null; rec.el = unsupported(widget); pageEl.appendChild(rec.el); }
            setAttr(rec.el, 'data-hd-wid', widget.id); rec.labelledBy = rec.el.getAttribute('aria-labelledby');
            recs.set(widget.id, rec); byEl.set(rec.el, rec); applyRole(rec); if (io) io.observe(rec.el);
            for (const id of rec.ids) { if (!byEntity.has(id)) byEntity.set(id, new Set()); byEntity.get(id).add(rec); }
        }
        function unmount(rec) {
            if (io) io.unobserve(rec.el); flips.delete(rec);
            if (rec.inst) rec.inst.destroy(); else rec.el.remove();
            recs.delete(rec.widget.id); byEl.delete(rec.el); limits.delete(rec.widget.id);
        }
        // Design: option frames of a listbox (name from describe, no title reference); view: the labelled groups of M2.
        function applyRole(rec) {
            const n = rec.el;
            if (mode === 'design') {
                setAttr(n, 'role', 'option'); setAttr(n, 'aria-labelledby', null); setAttr(n, 'tabindex', '-1');
                if (!n.hasAttribute('aria-selected')) setAttr(n, 'aria-selected', 'false');
            } else {
                setAttr(n, 'role', 'group'); setAttr(n, 'aria-labelledby', rec.labelledBy); setAttr(n, 'aria-selected', null); setAttr(n, 'aria-label', null); setAttr(n, 'tabindex', null); rec.label = '';
            }
        }
        function labelOf(rec, it) {
            const ent = rec.ids[0] ? decorated(rec, rec.ids[0], null) : null;
            return String((typeof o.describe === 'function' ? o.describe(rec.widget, it, ent) : '') || rec.widget.title || rec.widget.type || '');
        }
        function refreshLabel(rec, it) {
            if (mode !== 'design' || !it) return;
            const label = labelOf(rec, it); if (label !== rec.label) { rec.label = label; setAttr(rec.el, 'aria-label', label || null); }
        }
        function keepLayerLast() {   // the gesture layer stays above the widgets and the empty note
            const kids = Array.from(pageEl.children), layer = kids.find(c => String(c.getAttribute('class') || '').split(' ').includes('hd-edit-layer'));
            if (layer && layer !== kids[kids.length - 1]) pageEl.appendChild(layer);
        }
        // The empty note: mode-specific text and the optional action button; rebuilt only when mode or action change, removed once widgets exist.
        function renderEmpty() {
            if (pg.widgets.length) { if (emptyEl) { emptyEl.remove(); emptyEl = null; emptyKey = null; } return; }
            const key = mode + '|' + (emptyAction ? emptyAction.cmd + '|' + emptyAction.label : ''); if (emptyEl && key === emptyKey) return;
            if (emptyEl) emptyEl.remove(); emptyKey = key;
            const design = mode === 'design';
            emptyEl = el('div', { class: 'hd-page-empty', role: 'note', 'data-hd-mode': mode }, [E.icons.icon('dashboard'),
                el('p', { class: 'hd-page-empty-title', text: design ? t('homedash.designer.empty_title') : t('homedash.view.empty_title') }),
                el('p', { class: 'hd-page-empty-text', text: design ? t('homedash.designer.empty_text') : t('homedash.view.empty_text') }),
                emptyAction ? el('button', { type: 'button', class: 'hd-page-empty-action', 'data-hd-cmd': emptyAction.cmd, text: emptyAction.label }) : null]);
            pageEl.appendChild(emptyEl); keepLayerLast();
        }
        const flips = new Set(), flipFrame = E.core.frame(() => { for (const r of flips) { r.el.removeAttribute('data-hd-flip'); setVars(r.el, { '--hd-e-fx': null, '--hd-e-fy': null }); } flips.clear(); }, { raf: o.raf, caf: o.caf });
        for (const w of pg.widgets) mount(w);
        renderEmpty();
        function layout() {
            if (destroyed) return;
            const box = measure(o.stage);
            if (!box || !(box.width > 0)) return;
            bp = forcedBp || E.layout.breakpoint(box.width, bp);
            const width = forcedBp ? PREVIEW[forcedBp](box.width) : box.width, fit = mode === 'design' ? 'scroll' : fitOverride || doc.grid.fit;
            const cols = E.layout.COLS[bp], items = E.layout.effective(pg.layouts, bp, id => limits.get(id)), rows = Math.max(1, E.layout.rows(items));
            const gap = E.layout.gapFor(doc.look.density, doc.grid.gap), m = E.layout.metrics(width, box.height || 0, cols, rows, gap, doc.grid.row_unit, fit);
            const zoom = mode === 'design' && forcedBp ? zoomFor(box.width, width) : 1;
            setVars(pageEl, Object.assign(E.layout.cssVars(m, cols, rows, gap), { '--hd-e-pw': forcedBp ? Math.round(width) + 'px' : null, '--hd-e-zoom': zoom < 1 ? zoom : null }));
            setAttr(pageEl, 'data-hd-preview', forcedBp); setAttr(pageEl, 'data-hd-zoomed', zoom < 1 ? 'true' : null);
            last = Object.freeze({ bp, cols, rows, gap, colW: m.colW, rowH: m.rowH, offX: m.offX, mode: m.mode, width, zoom });
            if (typeof o.onLayout === 'function') o.onLayout({ bp, fit: m.mode });
            for (const it of items) {
                const rec = recs.get(it.id); if (!rec) continue;
                const cell = [it.x, it.y, it.w, it.h].join(',');
                if (cell !== rec.cell) { rec.cell = cell; setVars(rec.el, { '--hd-x': it.x, '--hd-y': it.y, '--hd-w': it.w, '--hd-h': it.h }); }
                const px = { width: Math.round(it.w * m.colW + (it.w - 1) * gap), height: Math.round(it.h * m.rowH + (it.h - 1) * gap) }, key = cell + ':' + px.width + 'x' + px.height;
                if (rec.inst && key !== rec.px) { rec.px = key; rec.inst.resize({ w: it.w, h: it.h, px }); }
                refreshLabel(rec, it);
            }
        }
        const schedule = E.core.frame(layout, { raf: o.raf, caf: o.caf });
        function onChange(ids) {
            if (destroyed) return;
            const targets = new Set();
            if (ids === null) for (const r of recs.values()) targets.add(r); else for (const id of ids) for (const r of byEntity.get(id) || []) targets.add(r);
            for (const r of targets) { if (!r.inst) continue; if (r.visible) r.inst.update(); else r.dirty = true; }
            if (mode === 'design' && last && targets.size) { const list = E.layout.effective(pg.layouts, last.bp, id => limits.get(id)); for (const r of targets) refreshLabel(r, list.find(i => i.id === r.widget.id)); }
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
        io = IO ? new IO(entries => { for (const en of entries) { const r = byEl.get(en.target); if (!r) continue; r.visible = !!en.isIntersecting; if (r.visible && r.dirty && r.inst) { r.dirty = false; r.inst.update(); } } }, { root: o.stage, rootMargin: '200px' }) : null;
        if (io) for (const r of recs.values()) io.observe(r.el);
        const RO = o.ResizeObserver || (typeof ResizeObserver === 'function' ? ResizeObserver : null);
        const ro = RO ? new RO(() => schedule()) : null;
        if (ro) ro.observe(o.stage);
        layout();
        function destroy() {
            if (destroyed) return; destroyed = true; schedule.cancel(); flushCmd.cancel(); flipFrame.cancel(); flips.clear(); store.off('change', onChange);
            for (const off of cmdOff.splice(0)) off();
            if (commands && E.popover) { const rootEl = o.stage.closest ? o.stage.closest('.hd-root') : null; if (rootEl) E.popover.close(rootEl); }
            if (ro) ro.disconnect(); if (io) io.disconnect();
            for (const r of recs.values()) if (r.inst) r.inst.destroy();
            pageEl.remove(); recs.clear(); byEntity.clear(); byEl.clear(); sheetSubs.clear();
        }
        // update(nd, np): the same page with a new document; widgets are diffed by their JSON plus the effective family, so unchanged widgets keep
        // their nodes (and focus, scroll and drag state). Layout runs synchronously.
        function update(nd, np) {
            if (destroyed) return;
            doc = nd; pg = np; family = nd.look && nd.look.family; pageAttrs();
            const want = new Map(pg.widgets.map(w => [w.id, w]));
            for (const rec of [...recs.values()]) { const w = want.get(rec.widget.id); if (!w || rec.key !== E.doc.keyOf(w) + '|' + (w.family || family || '')) unmount(rec); }
            for (const w of pg.widgets) if (!recs.has(w.id)) mount(w);
            byEntity.clear();
            for (const rec of recs.values()) for (const id of rec.ids) { if (!byEntity.has(id)) byEntity.set(id, new Set()); byEntity.get(id).add(rec); }
            renderEmpty(); keepLayerLast(); layout();
        }
        // preview(items, {skip, animate, rows}): writes the cells of changed items only (gesture feedback, no relayout); returns how many changed.
        function preview(items, po) {
            if (destroyed || !last) return 0;
            const p = po || {}, list = items || E.layout.effective(pg.layouts, last.bp, id => limits.get(id)), skip = p.skip || null;
            const rootEl = o.stage.closest ? o.stage.closest('.hd-root') : null, flip = !!p.animate && !E.core.motionReduced(rootEl); let n = 0;
            for (const it of list) {
                const rec = recs.get(it.id); if (!rec || (skip && skip.has(it.id))) continue;
                const cell = [it.x, it.y, it.w, it.h].join(','); if (cell === rec.cell) continue;
                const prev = rec.cell ? rec.cell.split(',').map(Number) : null; rec.cell = cell; n += 1;
                setVars(rec.el, { '--hd-x': it.x, '--hd-y': it.y, '--hd-w': it.w, '--hd-h': it.h });
                if (flip && prev) { setVars(rec.el, { '--hd-e-fx': Math.round((prev[0] - it.x) * (last.colW + last.gap)) + 'px', '--hd-e-fy': Math.round((prev[1] - it.y) * (last.rowH + last.gap)) + 'px' }); setAttr(rec.el, 'data-hd-flip', 'from'); flips.add(rec); flipFrame(); }
                const px = { width: Math.round(it.w * last.colW + (it.w - 1) * last.gap), height: Math.round(it.h * last.rowH + (it.h - 1) * last.gap) }, key = cell + ':' + px.width + 'x' + px.height;
                if (rec.inst && key !== rec.px) { rec.px = key; rec.inst.resize({ w: it.w, h: it.h, px }); }
            }
            setVars(pageEl, { '--hd-rows': Math.max(1, E.layout.rows(list), p.rows || 0) });
            return n;
        }
        function setMode(m) {
            mode = m === 'design' ? 'design' : 'view';
            for (const r of recs.values()) { r.mode = mode; if (r.inst) r.inst.setMode(mode); applyRole(r); }
            pageAttrs(); renderEmpty(); layout();
        }
        const itemsNow = () => (last ? E.layout.effective(pg.layouts, last.bp, id => limits.get(id)) : []);
        return Object.freeze({ el: pageEl, entityIds: () => [...byEntity.keys()], layout: () => schedule(), refreshAll: () => onChange(null), destroy,
            update, setMode, setBreakpoint(b) { forcedBp = b || null; layout(); }, setFit(f) { fitOverride = f || null; layout(); }, setEmptyAction(a) { emptyAction = a || null; renderEmpty(); },
            preview, metrics: () => last, items: itemsNow, frame: id => (recs.get(id) || {}).el || null, order: () => E.grid.sortReading(itemsNow()).map(i => i.id) });
    }
    E.render = Object.freeze({ page, isEntityId, zoomFor, contentBox, ZOOM_MIN });
})();
