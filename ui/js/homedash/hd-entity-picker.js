// HomeDash entity picker (M4 part 2, D6): the Data tab's combobox (WAI-ARIA APG, popup inside the box) that binds a widget slot to a device. One
// GET entities per allowed kind (C9), merged, filtered with catalog.accepts and sorted by name; live state of the first 20 options while open; the
// bound device shows name, area and source, or a warning when the store says `unknown` or GET entity answers 404. Names only via textContent.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.palette || !E.catalog) throw new Error('hd-entity-picker.js: load hd-form.js and hd-palette.js first');
    const { el, setAttr, uid } = E.core, DOT = ' ' + String.fromCodePoint(0xb7) + ' ', LIMIT = 50, SEARCH_MS = 200, WATCH_MAX = 20, CACHE_MS = 60000, cache = new Map();
    // One GET entities per allowed kind (C9: one kind per request); a chosen kind narrows to it; no kinds = any kind.
    function requests(f, kinds) { const base = { q: f.q || undefined, source: f.source || undefined, area: f.area || undefined, limit: LIMIT, offset: 0 };
        return (f.kind ? [f.kind] : kinds && kinds.length ? kinds : [null]).map(k => Object.assign({}, base, k ? { kind: k } : {})); }
    function merge(answers, accept, compare) { const seen = new Set(), out = [];
        for (const a of answers) for (const e of a && Array.isArray(a.entities) ? a.entities : []) { if (!e || typeof e.id !== 'string' || seen.has(e.id) || !accept(e)) continue; seen.add(e.id); out.push(e); }
        return out.sort((x, y) => compare(String(x.name || x.id), String(y.name || y.id))).slice(0, LIMIT); }
    const sourcesOf = answers => [...new Set(answers.flatMap(a => (a && Array.isArray(a.sources) ? a.sources : []).map(s => s && s.source).filter(s => typeof s === 'string')))];

    function create(o) {
        const D = o.designer, t = o.t, lang = o.lang || 'en', catalog = o.catalog, store = o.store, api = o.api, name = typeof o.binding === 'string' ? o.binding : o.binding.name, spec = catalog.binding(o.type, name) || {};
        const kinds = Array.isArray(spec.kinds) ? spec.kinds : [], timers = o.timers || { setTimeout: (f, ms) => setTimeout(f, ms), clearTimeout: x => clearTimeout(x) }, collator = new Intl.Collator(lang), life = E.core.bag();
        const id = uid('hd-ep'), listId = id + '-list', stateOf = e => E.palette.model.stateText(t, lang, store.view(e.id) || E.palette.model.listView(e));
        let value = o.value || '', open = false, options = [], active = -1, seq = 0, timer = null, destroyed = false, bound = null, boundSeq = 0, areas = [], sources = [], shownSources = '', shownAreas = '';
        const filters = { q: '', source: '', kind: '', area: '' }, watch = store.watch(uid('hd-ep'), { ids: [], active: true });
        const input = el('input', { class: 'hd-f-input hd-ep-input', type: 'text', id, role: 'combobox', 'aria-autocomplete': 'list', 'aria-expanded': 'false', 'aria-controls': listId, 'aria-haspopup': 'listbox', 'aria-activedescendant': null,
            placeholder: t('homedash.inspector.choose_device'), spellcheck: 'false', autocomplete: 'off', 'data-hd-field': 'binding:' + name });
        const clear = E.form.button({ label: t('homedash.inspector.clear_binding'), icon: 'close', iconOnly: true, tone: 'quiet', onClick: () => { pick(undefined); input.focus(); } });
        const toggle = el('button', { type: 'button', class: 'hd-ep-toggle', tabindex: '-1', 'aria-label': o.label || name }, E.icons.icon('chevron-down'));
        const selKind = E.form.select({ key: 'ep-kind', label: t('homedash.palette.filter_kind'), onChange: v => refilter({ kind: v }) }), selSource = E.form.select({ key: 'ep-source', label: t('homedash.palette.filter_source'), onChange: v => refilter({ source: v }) });
        const selArea = E.form.select({ key: 'ep-area', label: t('homedash.palette.filter_area'), onChange: v => refilter({ area: v }) });
        const status = el('p', { class: 'hd-ep-status', role: 'status' }), list = el('ul', { class: 'hd-ep-list', role: 'listbox', id: listId, 'aria-label': o.label || name });
        const filterBar = el('div', { class: 'hd-ep-filters' }, [kinds.length > 1 ? selKind.wrap : null, selSource.wrap, selArea.wrap]), pop = el('div', { class: 'hd-ep-pop', hidden: true }, [filterBar, list, status]);
        const box = el('div', { class: 'hd-ep-box' }, [el('span', { class: 'hd-ep-icon' }, E.icons.icon('search')), input, clear.el, toggle, pop]);
        const label = el('label', { class: 'hd-f-label', for: id }, [el('span', { text: o.label || name }), o.required ? el('span', { class: 'hd-f-req', text: t('homedash.inspector.required') }) : null]);
        const meta = el('p', { class: 'hd-ep-meta' }), warn = el('p', { class: 'hd-ep-warn', hidden: true, role: 'status' });
        const root = el('div', { class: 'hd-ep', 'data-hd-state': 'empty' }, [label, box, meta, warn]);
        selKind.options([{ id: '', label: t('homedash.palette.all_kinds') }].concat(kinds.map(k => ({ id: k, label: t('homedash.kind.' + k) })).sort((a, b) => collator.compare(a.label, b.label))), '');
        selSource.options([{ id: '', label: t('homedash.palette.all_sources') }], ''); selArea.options([{ id: '', label: t('homedash.palette.all_areas') }], '');

        function boundText() { return bound ? String(bound.name || bound.id) : value; }
        function paintBound() {
            const v = value ? store.view(value) : null, st = !value ? 'empty' : v && v.status === 'unknown' ? 'unknown' : bound === false ? 'unknown' : 'bound';
            const b = v && v.status !== 'unknown' ? v : bound && bound !== false ? bound : null;
            setAttr(root, 'data-hd-state', st); setAttr(clear.el, 'hidden', value ? null : true);
            if (!open) input.value = b ? String(b.name || value) : value;
            meta.textContent = b ? [value, b.area, b.source].filter(Boolean).join(DOT) : ''; setAttr(meta, 'hidden', b ? null : true);
            warn.replaceChildren(); setAttr(warn, 'hidden', st === 'unknown' ? null : true);
            if (st === 'unknown') { warn.appendChild(E.icons.icon('alert')); warn.appendChild(el('span', { text: t('homedash.inspector.unknown_entity', { id: value }) })); }
        }
        function resolve() {
            const my = ++boundSeq; bound = null; if (!value) { paintBound(); return; }
            const v = store.view(value); if (v) { paintBound(); return; }
            const hitc = cache.get(value); if (hitc && Date.now() - hitc.at < CACHE_MS) { bound = hitc.entity; paintBound(); return; }
            paintBound();
            Promise.resolve().then(() => api.entity(value)).then(ans => { const e = ans && ans.entity && typeof ans.entity === 'object' ? ans.entity : null; cache.set(value, { at: Date.now(), entity: e || false }); if (my === boundSeq && !destroyed) { bound = e || false; paintBound(); } },
                err => { const gone = !!err && err.code === 'entity_not_found'; if (gone) cache.set(value, { at: Date.now(), entity: false }); if (my === boundSeq && !destroyed) { bound = gone ? false : null; paintBound(); } });
        }
        function setOpen(on) {
            if (open === on) return; open = on; setAttr(pop, 'hidden', on ? null : true); setAttr(input, 'aria-expanded', on ? 'true' : 'false'); if (!on) { setAttr(input, 'aria-activedescendant', null); active = -1; watch.set([]); timers.clearTimeout(timer); timer = null; seq += 1; }
            if (on) search(); else paintBound();
        }
        function search() {
            const my = ++seq; status.textContent = t('homedash.inspector.searching');
            Promise.all(requests(filters, kinds).map(r => Promise.resolve().then(() => api.entities(r)))).then(answers => {
                if (my !== seq || destroyed || !open) return; options = merge(answers, e => catalog.accepts(o.type, name, e), collator.compare); sources = sourcesOf(answers); areas = [...new Set(areas.concat(options.map(e => e.area).filter(Boolean)))].sort(); renderOptions();
            }, () => { if (my !== seq || destroyed || !open) return; options = []; renderOptions(); status.textContent = t('homedash.inspector.search_failed'); });
        }
        function renderOptions() {
            list.replaceChildren(); active = options.length ? Math.max(0, options.findIndex(e => e.id === value)) : -1;
            options.forEach((e, i) => list.appendChild(el('li', { class: 'hd-ep-opt', role: 'option', id: id + '-o' + i, 'aria-selected': e.id === value ? 'true' : 'false', 'data-entity': e.id, 'data-hd-active': i === active ? 'true' : null },
                [el('span', { class: 'hd-ep-opt-name', text: String(e.name || e.id) }), el('span', { class: 'hd-ep-opt-meta', text: [e.area, e.source].filter(Boolean).join(DOT) }), el('span', { class: 'hd-ep-opt-state', text: stateOf(e) })])));
            status.textContent = options.length ? '' : t('homedash.inspector.no_match'); setAttr(input, 'aria-activedescendant', active >= 0 ? id + '-o' + active : null);
            if (shownSources !== sources.join(',')) { shownSources = sources.join(','); selSource.options([{ id: '', label: t('homedash.palette.all_sources') }].concat(sources.map(s => ({ id: s, label: E.palette.model.sourceName(t, s) }))), filters.source); }
            if (shownAreas !== areas.join(',')) { shownAreas = areas.join(','); selArea.options([{ id: '', label: t('homedash.palette.all_areas') }].concat(areas.map(a => ({ id: a, label: a }))), filters.area); }
            watch.set(options.slice(0, WATCH_MAX).map(e => e.id));
        }
        function refilter(patch) { Object.assign(filters, patch); if (open) search(); }
        function move(d) { if (!options.length) return; const prev = list.children[active]; if (prev) prev.removeAttribute('data-hd-active'); active = (active + d + options.length) % options.length;
            const cur = list.children[active]; if (cur) { cur.setAttribute('data-hd-active', 'true'); if (typeof cur.scrollIntoView === 'function') cur.scrollIntoView({ block: 'nearest' }); } setAttr(input, 'aria-activedescendant', id + '-o' + active); }
        function pick(entityId) { setOpen(false); value = entityId || ''; bound = entityId && options.find(e => e.id === entityId) || null; filters.q = ''; paintBound(); if (o.onPick) o.onPick(entityId || undefined); }
        life.on(input, 'input', () => { filters.q = input.value; if (!open) { setOpen(true); return; } timers.clearTimeout(timer); timer = timers.setTimeout(() => { timer = null; search(); }, SEARCH_MS); });
        life.on(input, 'focus', () => { input.select && input.select(); });
        life.on(input, 'keydown', ev => {
            if (ev.isComposing) return;
            if (ev.key === 'ArrowDown' || ev.key === 'ArrowUp') { ev.preventDefault(); if (!open) setOpen(true); else move(ev.key === 'ArrowDown' ? 1 : -1); return; }
            if (ev.key === 'Enter') { if (open && active >= 0 && options[active]) { ev.preventDefault(); pick(options[active].id); } return; }
            if (ev.key === 'Escape' && open) { ev.preventDefault(); ev.stopPropagation(); setOpen(false); filters.q = ''; paintBound(); return; }
            if (ev.key === 'Tab') setOpen(false);
        });
        life.on(toggle, 'click', () => { setOpen(!open); if (open) input.focus(); });
        life.on(list, 'pointerdown', ev => ev.preventDefault());
        life.on(list, 'click', ev => { const li = ev.target && ev.target.closest ? ev.target.closest('.hd-ep-opt') : null; if (li) pick(li.getAttribute('data-entity')); });
        life.on(root, 'focusout', ev => { if (!ev.relatedTarget || !root.contains(ev.relatedTarget)) setOpen(false); });
        life.add(store.on('change', changed => { if (open) options.forEach((e, i) => { if (!changed || changed.has(e.id)) { const li = list.children[i], s = li && li.querySelector('.hd-ep-opt-state'); if (s) s.textContent = stateOf(e); } }); if (!open && value && (!changed || changed.has(value))) paintBound(); }));
        resolve();
        return Object.freeze({ el: root, binding: name, set(v) { const next = v || ''; if (next === value) return; value = next; filters.q = ''; resolve(); }, focus() { input.focus(); }, input, destroy() { if (destroyed) return; destroyed = true; timers.clearTimeout(timer); life.dispose(); watch.release(); root.remove(); } });
    }
    E.entityPicker = Object.freeze({ create, model: Object.freeze({ requests, merge, sourcesOf, LIMIT }) });
})();
