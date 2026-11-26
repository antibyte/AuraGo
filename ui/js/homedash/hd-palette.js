// HomeDash palette (M4 part 2, F4b; D5): the Widgets tab (catalog types the registry implements, live sample thumbs, search, grid keys) and the Devices
// tab (GET entities with q/source/kind/area, paging 200, live state of at most 20 visible rows). Pure models on E.palette.model; create() builds the
// DOM into the C15 shell. Drag goes through D.gestures.dragFrom, tap/Enter through D.addWidgets. Untrusted names only via textContent.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.samples || !E.commands || !E.catalog) throw new Error('hd-palette.js: load hd-form.js, hd-samples.js and hd-commands.js first');
    const { el, setAttr, uid } = E.core, DOT = ' ' + String.fromCodePoint(0xb7) + ' ', fold = E.form.fold;
    const ENT_LIMIT = 200, WATCH_MAX = 20, SEARCH_MS = 250, COLS = 2, THUMB_BOX = Object.freeze({ width: 98, height: 84 }), FILTERS = Object.freeze(['q', 'source', 'kind', 'area']);
    const TYPE_ICONS = Object.freeze({ switch: 'power', light: 'bulb', action: 'scene', select: 'list', number: 'number', climate: 'thermometer', temperature: 'thermometer', gauge: 'gauge', sensor: 'chart', entity: 'entity' });
    // Widget cards for a search: every whitespace-separated token must appear (case- and accent-insensitively) in the type name, variant name,
    // description, category name, type id or variant id. Catalog order kept; empty groups dropped.
    function filterGroups(groups, query, names) {
        const tokens = fold(query).split(/\s+/).filter(Boolean), out = [];
        for (const g of groups || []) { const items = [];
            for (const x of g.types || []) for (const v of x.variants || []) { const n = names(x.type, v, g.category) || {};
                if (tokens.every(k => fold([n.type, n.variant, n.desc, n.category, x.type, v].join(' ')).includes(k))) items.push(Object.freeze({ type: x.type, variant: v })); }
            if (items.length) out.push(Object.freeze({ category: g.category, items: Object.freeze(items) })); }
        return Object.freeze(out);
    }
    const entInit = () => Object.freeze({ q: '', source: '', kind: '', area: '', items: Object.freeze([]), total: 0, offset: 0, loading: false, error: '', seq: 0, sources: Object.freeze([]), areas: Object.freeze([]) });
    const nx = (s, p) => Object.freeze(Object.assign({}, s, p));
    // Entity list state. filter resets the list (seq+1); more asks the next page; loaded/failed of an older seq are ignored; a page with no new
    // rows ends paging; areas accumulate across filters so the area select keeps its options.
    function entReduce(s, a) {
        switch (a && a.type) {
            case 'filter': { const f = {}; for (const k of FILTERS) f[k] = typeof a[k] === 'string' ? (k === 'q' ? Array.from(a[k].trim()).slice(0, 100).join('') : a[k]) : s[k];
                if (!a.force && s.seq > 0 && FILTERS.every(k => f[k] === s[k])) return s;
                return nx(s, Object.assign(f, { items: Object.freeze([]), total: 0, offset: 0, loading: true, error: '', seq: s.seq + 1 })); }
            case 'more': return s.loading || s.error || s.items.length >= s.total ? s : nx(s, { offset: s.items.length, loading: true, seq: s.seq + 1 });
            case 'retry': return nx(s, { loading: true, error: '', seq: s.seq + 1 });
            case 'loaded': { if (a.seq !== s.seq) return s; const ans = a.answer || {}, seen = new Set(s.offset ? s.items.map(e => e.id) : []);
                const fresh = (Array.isArray(ans.entities) ? ans.entities : []).filter(e => e && typeof e.id === 'string' && !seen.has(e.id) && seen.add(e.id));
                const items = (s.offset ? s.items : []).concat(fresh), total = fresh.length ? Math.max(items.length, Number(ans.total) || 0) : items.length;
                const areas = [...new Set(s.areas.concat(items.map(e => (typeof e.area === 'string' ? e.area : '')).filter(Boolean)))].sort();
                const sources = Array.isArray(ans.sources) ? [...new Set(ans.sources.map(x => (x && typeof x.source === 'string' ? x.source : '')).filter(Boolean))] : s.sources;
                return nx(s, { items: Object.freeze(items), total, loading: false, error: '', areas: Object.freeze(areas), sources: Object.freeze(sources) }); }
            case 'failed': return a.seq !== s.seq ? s : nx(s, { loading: false, error: a.code || 'network' });
            default: return s;
        }
    }
    const entRequest = s => ({ q: s.q || undefined, source: s.source || undefined, kind: s.kind || undefined, area: s.area || undefined, limit: ENT_LIMIT, offset: s.offset });
    const watchIds = (order, visible, max) => order.filter(id => visible.has(id)).slice(0, max || WATCH_MAX);
    const kindsOf = catalog => [...new Set(catalog.defaults().map(d => d.kind))];
    const sourceName = (t, src) => (src === 'system' ? t('homedash.source.system') : src === 'ha' ? t('homedash.source.ha') : String(src || ''));
    function stateText(t, lang, v) { if (!v || ['scene', 'script', 'button'].includes(v.kind)) return ''; const w = E.format.stateWord(v); if (w !== 'value') return t(E.format.STATE_PREFIX + w);
        return typeof v.value === 'number' ? E.format.valueText(v.value, { lang, unit: v.unit }) : v.text || (v.value == null ? E.format.DASH : String(v.value)); }
    function listView(e) { const s = e && e.state && typeof e.state === 'object' ? e.state : {};
        return { kind: e.kind, unit: e.unit || '', icon_hint: e.icon_hint || '', value: s.value === undefined ? null : s.value, text: typeof s.text === 'string' ? s.text : '', available: s.available !== false, status: 'ok', pending: false }; }

    function create(o) {
        const D = o.designer, t = o.t, lang = o.lang || 'en', catalog = o.catalog, store = o.store, api = o.api, life = E.core.bag(), ids = { w: uid('hd-palw'), e: uid('hd-pale'), hw: uid('hd-palhw'), he: uid('hd-palhe') };
        const timers = o.timers || { setTimeout: (f, ms) => setTimeout(f, ms), clearTimeout: x => clearTimeout(x) }, collator = new Intl.Collator(lang), root = el('div', { class: 'hd-pal' }), thumbs = [];
        const familyNow = () => { const d = typeof D.doc === 'function' ? D.doc() : null; return (d && d.look && d.look.family) || null; };   // the thumbs show the dashboard's own family
        const typeName = x => t('homedash.widget.' + x + '.name'), variantName = (x, v) => t('homedash.variant.' + x + '.' + v);
        const names = (type, v, cat) => ({ type: typeName(type), variant: variantName(type, v), desc: t('homedash.widget.' + type + '.desc'), category: t('homedash.category.' + cat) });
        let tab = 'widgets', ent = entInit(), entStarted = false, timer = null, queryW = '', queryE = '', destroyed = false;
        const makeThumb = s => E.samples.thumb({ widget: { type: s.type, variant: s.variant, title: '', bindings: {}, options: catalog.optionDefaults(s.type), actions: {}, confirm: false }, family: familyNow() || undefined, catalog, t, lang, box: THUMB_BOX, parent: s.box });
        let famShown = familyNow();
        // head
        const tabsCtl = E.form.tabs({ tabs: [{ id: 'widgets', label: t('homedash.palette.tab_widgets') }, { id: 'entities', label: t('homedash.palette.tab_entities') }], value: tab, label: t('homedash.designer.palette'), onChange: id => setTab(id) });
        const search = E.form.text({ key: 'palette-search', search: true, max: 100, onInput: v => onSearch(v) });
        const clearBtn = E.form.button({ label: t('homedash.palette.clear_search'), icon: 'close', iconOnly: true, tone: 'quiet', onClick: () => { search.set(''); onSearch(''); search.focus(); } });
        setAttr(clearBtn.el, 'hidden', true);
        root.appendChild(el('div', { class: 'hd-pal-head' }, [tabsCtl.el, el('div', { class: 'hd-pal-search' }, [E.icons.icon('search'), search.el, clearBtn.el])]));
        // widgets panel
        const panelW = el('div', { class: 'hd-pal-panel', role: 'tabpanel', id: ids.w, 'aria-labelledby': tabsCtl.idOf('widgets') }), statusW = el('p', { class: 'hd-pal-status', hidden: true, role: 'status' });
        panelW.appendChild(el('p', { class: 'hd-sr-only', id: ids.hw, text: t('homedash.palette.widget_hint') }));
        const groups = catalog.paletteGroups(E.widgets), cards = new Map(), groupEls = [];
        for (const g of groups) {
            const hid = uid('hd-palg'), grid = el('div', { class: 'hd-pal-grid' }), sec = el('section', { class: 'hd-pal-group', 'aria-labelledby': hid }, [el('h3', { class: 'hd-pal-group-title', id: hid, text: t('homedash.category.' + g.category) }), grid]);
            groupEls.push({ category: g.category, el: sec, grid, keys: [] });
            for (const x of g.types) for (const v of x.variants) {
                const n = names(x.type, v, g.category), box = el('div', { class: 'hd-pal-thumb' }), key = x.type + '/' + v;
                const hitBtn = el('button', { type: 'button', class: 'hd-pal-hit', tabindex: '-1', 'aria-label': t('homedash.palette.card_label', { type: n.type, variant: n.variant }), 'aria-describedby': ids.hw, title: n.desc }, [el('span', { class: 'hd-pal-name', text: n.type }), el('span', { class: 'hd-pal-variant', text: n.variant })]);
                const card = el('div', { class: 'hd-pal-card', 'data-type': x.type, 'data-variant': v }, [box, hitBtn]); grid.appendChild(card); cards.set(key, card); groupEls[groupEls.length - 1].keys.push(key);
                const spec = { type: x.type, variant: v, box, th: null }; spec.th = makeThumb(spec); thumbs.push(spec);
            }
            panelW.appendChild(sec);
        }
        panelW.appendChild(statusW); root.appendChild(panelW);
        // devices panel
        const panelE = el('div', { class: 'hd-pal-panel', role: 'tabpanel', id: ids.e, 'aria-labelledby': tabsCtl.idOf('entities'), hidden: true });
        const selSource = E.form.select({ key: 'palette-source', label: t('homedash.palette.filter_source'), onChange: v => filter({ source: v }) }), selKind = E.form.select({ key: 'palette-kind', label: t('homedash.palette.filter_kind'), onChange: v => filter({ kind: v }) });
        const selArea = E.form.select({ key: 'palette-area', label: t('homedash.palette.filter_area'), onChange: v => filter({ area: v }) });
        const statusText = el('span', { class: 'hd-pal-status-text' }), retryBtn = E.form.button({ label: t('homedash.palette.retry'), tone: 'quiet', onClick: () => { ent = entReduce(ent, { type: 'retry' }); renderEnt(); load(); } });
        const statusE = el('p', { class: 'hd-pal-status', role: 'status', 'data-state': 'idle' }, [statusText, retryBtn.el]), list = el('ul', { class: 'hd-ent-list', 'aria-label': t('homedash.palette.tab_entities') });
        const moreBtn = E.form.button({ label: t('homedash.palette.load_more'), tone: 'quiet', onClick: () => more() }), moreItem = el('li', { class: 'hd-ent-moreitem', hidden: true }, moreBtn.el);
        panelE.append(el('p', { class: 'hd-sr-only', id: ids.he, text: t('homedash.palette.entity_hint') }), el('div', { class: 'hd-pal-filters' }, [selSource.wrap, selKind.wrap, selArea.wrap]), statusE, list); root.appendChild(panelE);
        const kindList = kindsOf(catalog).map(k => ({ id: k, label: t('homedash.kind.' + k) })).sort((a, b) => collator.compare(a.label, b.label));
        selKind.options([{ id: '', label: t('homedash.palette.all_kinds') }].concat(kindList), ''); selSource.options([{ id: '', label: t('homedash.palette.all_sources') }], ''); selArea.options([{ id: '', label: t('homedash.palette.all_areas') }], '');
        let shownSources = '', shownAreas = '';
        const rows = new Map(), order = [], visible = new Set(), byId = new Map();
        const watch = store.watch(uid('hd-pal'), { ids: [], active: true });
        const syncWatch = E.core.frame(() => { if (!destroyed) watch.set(tab === 'entities' ? watchIds(order, visible) : []); }, { raf: o.raf, caf: o.caf });
        const IO = typeof IntersectionObserver === 'function' ? IntersectionObserver : null;
        const rowObserver = IO ? new IO(list2 => { for (const en of list2) { const id = en.target.getAttribute('data-entity'); if (en.isIntersecting) visible.add(id); else visible.delete(id); } syncWatch(); }, { root: panelE, rootMargin: '64px' }) : null;
        const moreObserver = IO ? new IO(list2 => { if (list2.some(en => en.isIntersecting)) more(); }, { root: panelE, rootMargin: '64px' }) : null;
        if (moreObserver) moreObserver.observe(moreItem);

        function avatar(icon, title, sub) { return el('div', { class: 'hd-pal-avatar' }, [E.icons.icon(icon), el('span', { class: 'hd-pal-avatar-text' }, [el('strong', { text: title }), sub ? el('small', { text: sub }) : null])]); }
        function addEntry(entry, source) { if (!entry) return; if (D.state().drawers) D.closePanels(); D.addWidgets([entry], { source }); }
        function entryFor(e) { const pick = catalog.pickWidgetFor({ kind: e.kind, device_class: e.device_class, capabilities: e.capabilities }, E.widgets); return pick ? E.commands.entry(catalog, pick.type, pick.variant, e.id) : null; }
        // ---- widgets tab ----
        function applyWidgetSearch() {
            const shown = new Set(filterGroups(groups, queryW, names).flatMap(g => g.items.map(i => i.type + '/' + i.variant)));
            for (const [key, card] of cards) setAttr(card, 'hidden', shown.has(key) ? null : true);
            for (const g of groupEls) setAttr(g.el, 'hidden', g.keys.some(k => shown.has(k)) ? null : true);
            statusW.textContent = shown.size || !queryW.trim() ? '' : t('homedash.palette.no_widgets', { q: queryW.trim() }); setAttr(statusW, 'hidden', statusW.textContent ? null : true); roveW();
        }
        const hitsW = () => [...cards.values()].filter(c => !c.hasAttribute('hidden')).map(c => c.querySelector('.hd-pal-hit'));
        function roveW(focus) { const v = hitsW(), cur = v.find(b => b.getAttribute('tabindex') === '0' && v.includes(b)) || v[0]; for (const b of cards.values()) b.querySelector('.hd-pal-hit').setAttribute('tabindex', '-1'); if (cur) { cur.setAttribute('tabindex', '0'); if (focus) cur.focus(); } }
        life.on(panelW, 'click', ev => { const h = ev.target && ev.target.closest ? ev.target.closest('.hd-pal-hit') : null, card = h && h.closest('.hd-pal-card'); if (card) addEntry(E.commands.entry(catalog, card.getAttribute('data-type'), card.getAttribute('data-variant')), 'palette'); });
        life.on(panelW, 'pointerdown', ev => { const card = ev.target && ev.target.closest ? ev.target.closest('.hd-pal-card') : null; if (!card || (ev.button !== undefined && ev.button !== 0)) return;
            const type = card.getAttribute('data-type'), variant = card.getAttribute('data-variant'), entry = E.commands.entry(catalog, type, variant);
            D.gestures.dragFrom(ev, { kind: 'widget', label: typeName(type), entries: () => [entry], avatar: () => avatar(TYPE_ICONS[type] || 'dashboard', typeName(type), variantName(type, variant)) }); });
        life.on(panelW, 'keydown', ev => { const v = hitsW(), at = v.findIndex(b => b === ev.target); if (at < 0) return;
            const sizes = groupEls.map(g => g.keys.filter(k => !cards.get(k).hasAttribute('hidden')).length), n = E.form.gridMove(ev.key, at, sizes, COLS); if (n === null) return;
            ev.preventDefault(); for (const b of v) b.setAttribute('tabindex', b === v[n] ? '0' : '-1'); v[n].focus(); if (typeof v[n].scrollIntoView === 'function') v[n].scrollIntoView({ block: 'nearest' }); });
        // ---- devices tab ----
        function filter(patch) { ent = entReduce(ent, Object.assign({ type: 'filter' }, patch)); renderEnt(); load(); }
        function more() { const next = entReduce(ent, { type: 'more' }); if (next === ent) return; ent = next; renderEnt(); load(); }
        function load() {
            const seq = ent.seq; Promise.resolve().then(() => api.entities(entRequest(ent))).then(answer => { if (!destroyed) { ent = entReduce(ent, { type: 'loaded', seq, answer }); renderEnt(); } },
                err => { if (!destroyed) { ent = entReduce(ent, { type: 'failed', seq, code: err && err.code ? String(err.code) : 'network' }); renderEnt(); } });
        }
        function rowState(id) { const v = store.view(id), e = byId.get(id); return stateText(t, lang, v && v.status !== 'unknown' ? v : e ? listView(e) : null); }
        function buildRow(e) {
            const v = store.view(e.id) || listView(e), icon = E.widgets.iconFor(null, v) || 'entity', source = sourceName(t, e.source || String(e.id).split(':')[0]);
            const btn = el('button', { type: 'button', class: 'hd-ent-row', tabindex: '-1', 'data-entity': e.id, 'aria-describedby': ids.he }, [el('span', { class: 'hd-ent-icon' }, E.icons.icon(icon)),
                el('span', { class: 'hd-ent-text' }, [el('span', { class: 'hd-ent-name', text: String(e.name || e.id) }), el('span', { class: 'hd-ent-meta', text: [e.area, source].filter(Boolean).join(DOT) })]), el('span', { class: 'hd-ent-state', text: stateText(t, lang, v) })]);
            const li = el('li', { class: 'hd-ent-item' }, btn); if (rowObserver) rowObserver.observe(btn); return { li, btn };
        }
        function renderEnt() {
            if (destroyed) return;
            if (ent.items.length < order.length || (order.length && ent.items[0] && ent.items[0].id !== order[0])) { for (const r of rows.values()) { if (rowObserver) rowObserver.unobserve(r.btn); r.li.remove(); } rows.clear(); order.length = 0; byId.clear(); visible.clear(); }
            for (const e of ent.items) { if (rows.has(e.id)) continue; const r = buildRow(e); rows.set(e.id, r); order.push(e.id); byId.set(e.id, e); list.appendChild(r.li); if (!rowObserver && visible.size < WATCH_MAX) visible.add(e.id); }
            list.appendChild(moreItem); setAttr(moreItem, 'hidden', ent.items.length < ent.total && !ent.error ? null : true);
            const failed = !!ent.error, state = ent.loading ? 'loading' : failed ? 'failed' : !ent.items.length ? 'empty' : 'ready';
            statusText.textContent = state === 'loading' && !ent.items.length ? t('homedash.palette.loading') : state === 'failed' ? t('homedash.palette.failed') : state === 'empty' ? t('homedash.palette.empty')
                : ent.items.length < ent.total ? t('homedash.palette.count_of', { shown: ent.items.length, total: ent.total }) : t('homedash.palette.count', { count: ent.items.length });
            setAttr(statusE, 'data-state', state); setAttr(retryBtn.el, 'hidden', failed ? null : true);
            const srcKey = ent.sources.join(','), areaKey = ent.areas.join(',');
            if (srcKey !== shownSources) { shownSources = srcKey; selSource.options([{ id: '', label: t('homedash.palette.all_sources') }].concat(ent.sources.map(s => ({ id: s, label: sourceName(t, s) }))), ent.source); }
            if (areaKey !== shownAreas) { shownAreas = areaKey; selArea.options([{ id: '', label: t('homedash.palette.all_areas') }].concat(ent.areas.map(a => ({ id: a, label: a }))), ent.area); }
            roveE(); syncWatch();
        }
        const btnsE = () => [...rows.values()].map(r => r.btn);
        function roveE(focus) { const v = btnsE(), cur = v.find(b => b.getAttribute('tabindex') === '0') || v[0]; for (const b of v) b.setAttribute('tabindex', b === cur ? '0' : '-1'); if (cur && focus) cur.focus(); }
        life.on(list, 'click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-ent-row') : null, e = b && byId.get(b.getAttribute('data-entity')); if (e) addEntry(entryFor(e), 'entity'); });
        life.on(list, 'pointerdown', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-ent-row') : null, e = b && byId.get(b.getAttribute('data-entity')); if (!e || (ev.button !== undefined && ev.button !== 0)) return;
            const entry = entryFor(e), icon = E.widgets.iconFor(null, listView(e)) || 'entity'; D.gestures.dragFrom(ev, { kind: 'entity', entity: e, label: String(e.name || e.id), entries: () => (entry ? [entry] : []), avatar: () => avatar(icon, String(e.name || e.id), e.area || '') }); });
        life.on(list, 'keydown', ev => { const v = btnsE(), at = v.findIndex(b => b === ev.target); if (at < 0) return; const n = E.form.roving(ev.key, at, v.length); if (n === null || ev.key === 'ArrowLeft' || ev.key === 'ArrowRight') return;
            ev.preventDefault(); for (const b of v) b.setAttribute('tabindex', b === v[n] ? '0' : '-1'); v[n].focus(); if (typeof v[n].scrollIntoView === 'function') v[n].scrollIntoView({ block: 'nearest' }); });
        life.add(store.on('change', changed => { if (tab !== 'entities') return; for (const [id, r] of rows) if (!changed || changed.has(id)) { const s = r.btn.querySelector('.hd-ent-state'); if (s) s.textContent = rowState(id); } }));
        // ---- head wiring ----
        function onSearch(v) {
            setAttr(clearBtn.el, 'hidden', v ? null : true);
            if (tab === 'widgets') { queryW = v; applyWidgetSearch(); return; }
            queryE = v; timers.clearTimeout(timer); timer = timers.setTimeout(() => { timer = null; filter({ q: queryE }); }, SEARCH_MS);
        }
        function setTab(next) {
            if (next !== 'widgets' && next !== 'entities') return; tab = next; tabsCtl.set(next);
            setAttr(panelW, 'hidden', next === 'widgets' ? null : true); setAttr(panelE, 'hidden', next === 'entities' ? null : true);
            const v = next === 'widgets' ? queryW : queryE; search.set(v); setAttr(clearBtn.el, 'hidden', v ? null : true);
            const label = t(next === 'widgets' ? 'homedash.palette.search_widgets' : 'homedash.palette.search_entities'); setAttr(search.el, 'aria-label', label); setAttr(search.el, 'placeholder', label); setAttr(search.el, 'aria-controls', next === 'widgets' ? ids.w : ids.e);
            for (const b of tabsCtl.el.querySelectorAll('.hd-f-tab')) setAttr(b, 'aria-controls', b.getAttribute('data-tab') === 'widgets' ? ids.w : ids.e);
            if (next === 'entities' && !entStarted) { entStarted = true; filter({ force: true, q: queryE }); } else syncWatch();
        }
        life.on(root, 'keydown', ev => {
            if (ev.key === 'ArrowDown' && ev.target === search.el) { const first = tab === 'widgets' ? hitsW()[0] : btnsE()[0]; if (first) { ev.preventDefault(); first.focus(); } return; }
            if (ev.key !== 'Escape') return;
            if (ev.target === search.el && search.get()) { ev.preventDefault(); ev.stopPropagation(); search.set(''); onSearch(''); return; }
            ev.preventDefault(); const s = D.state(); if (s.drawers) { D.closePanels(); return; } const id = s.focusId || s.primary || (s.items && s.items[0] && s.items[0].id); if (id) D.focusFrame(id);
        });
        if (typeof D.on === 'function') life.add(D.on('state', () => { const f = familyNow(); if (f === famShown) return; famShown = f; for (const s of thumbs) { s.th.destroy(); s.th = makeThumb(s); } }));
        setTab('widgets'); applyWidgetSearch(); if (o.host) o.host.appendChild(root);
        return Object.freeze({ el: root, focusSearch() { search.focus(); }, setTab, destroy() {
            if (destroyed) return; destroyed = true; timers.clearTimeout(timer); syncWatch.cancel(); if (rowObserver) rowObserver.disconnect(); if (moreObserver) moreObserver.disconnect(); life.dispose(); watch.release(); for (const s of thumbs) s.th.destroy(); thumbs.length = 0; root.remove(); } });
    }
    E.palette = Object.freeze({ create, model: Object.freeze({ fold, filterGroups, entInit, entReduce, entRequest, watchIds, kindsOf, sourceName, stateText, listView, TYPE_ICONS, ENT_LIMIT, WATCH_MAX }) });
})();
