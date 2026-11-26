// HomeDash inspector (M4 part 2, F4b; D6, design 5.11): page properties when nothing is selected, four tabs (Data, Look, Options, Actions) for one
// widget, and family/align/delete for several. Every field change is one D.exec(E.commands.*) (text fields coalesce with text:true and seal on blur;
// text that the server would refuse is flagged inline and never executed). The structure is rebuilt only when structureKey changes, otherwise the
// controls are patched in place (never the focused one). Pure models on E.inspector.model. Untrusted names only via textContent.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.entityPicker || !E.samples || !E.commands) throw new Error('hd-inspector.js: load hd-form.js, hd-samples.js, hd-entity-picker.js and hd-commands.js first');
    const { el, setAttr, uid } = E.core, DOT = ' ' + String.fromCodePoint(0xb7) + ' ', C = E.commands, F = E.form, TABS = Object.freeze(['data', 'look', 'options', 'actions']);
    const UNITS = Object.freeze({ hours: 'h', refresh_seconds: 's' }), HELP = Object.freeze(['accent', 'hours', 'warn', 'alert']);
    // Option spec -> control (D6, catalog OptionSpec types bool|int|number|string|enum|color|page).
    function fieldFor(spec, value, ctx) {
        const s = spec || {}, v = value === undefined ? s.default : value, base = { key: s.key, help: HELP.includes(s.key), required: s.required === true };
        switch (s.type) {
            case 'bool': return Object.assign(base, { control: 'switch', value: v === true });
            case 'int': case 'number': return Object.assign(base, { control: 'number', value: typeof v === 'number' && Number.isFinite(v) ? v : null, min: Number.isFinite(s.min) ? s.min : null, max: Number.isFinite(s.max) ? s.max : null,
                step: s.type === 'int' ? 1 : Number.isFinite(s.step) && s.step > 0 ? s.step : 'any', unit: UNITS[s.key] || '', nullable: s.default === null });
            case 'string': return Object.assign(base, { control: 'text', value: typeof v === 'string' ? v : '', maxLen: Math.min(Number(s.max_len) || 200, 200) });
            case 'enum': return Object.assign(base, { control: 'select', value: typeof v === 'string' ? v : '', options: (Array.isArray(s.enum) ? s.enum : []).filter(x => typeof x === 'string') });
            case 'color': return Object.assign(base, { control: 'color', value: typeof v === 'string' && /^#[0-9a-f]{6}$/i.test(v) ? v.toLowerCase() : '', allowEmpty: !s.required });
            case 'page': return Object.assign(base, { control: 'page', value: typeof v === 'string' ? v : '', options: ((ctx && ctx.pages) || []).filter(p => p.id !== (ctx && ctx.currentPageId)).map(p => p.id) });
            default: return null;
        }
    }
    // Control value -> option value: {ok, value} (undefined = back to the catalog default) or {ok:false, error:'required'|'bad_number'|'range'|'bad_colour'|'control_chars'|'url_not_allowed', min, max}.
    function commitValue(spec, raw) {
        const s = spec || {}, done = v => ({ ok: true, value: E.doc.keyOf(v) === E.doc.keyOf(s.default) ? undefined : v });
        switch (s.type) {
            case 'bool': return done(raw === true);
            case 'int': case 'number': { const txt = typeof raw === 'string' ? raw.trim().replace(',', '.') : raw;
                if (txt === '' || txt === null || txt === undefined) return s.required && s.default !== null ? { ok: false, error: 'required' } : { ok: true, value: undefined };
                let n = typeof txt === 'number' ? txt : Number(txt); if (!Number.isFinite(n)) return { ok: false, error: 'bad_number' }; if (s.type === 'int') n = Math.round(n);
                if ((Number.isFinite(s.min) && n < s.min) || (Number.isFinite(s.max) && n > s.max)) return { ok: false, error: 'range', min: s.min, max: s.max }; return done(n); }
            case 'string': { const v = Array.from(String(raw == null ? '' : raw)).slice(0, Math.min(Number(s.max_len) || 200, 200)).join(''), bad = E.form.textIssue(v);
                if (bad) return { ok: false, error: bad }; return v === '' && s.required ? { ok: false, error: 'required' } : done(v); }
            case 'enum': return Array.isArray(s.enum) && s.enum.includes(raw) ? done(raw) : { ok: false, error: 'required' };
            case 'color': { const v = String(raw || '').trim().toLowerCase(); if (!v) return s.required ? { ok: false, error: 'required' } : done(''); const m = /^#?([0-9a-f]{6})$/.exec(v); return m ? done('#' + m[1]) : { ok: false, error: 'bad_colour' }; }
            case 'page': return raw ? done(String(raw)) : s.required ? { ok: false, error: 'required' } : done('');
            default: return { ok: false, error: 'required' };
        }
    }
    function actionChoices(o) { const out = ['default']; if (o.canToggle) out.push('toggle'); if (o.entityKind === 'light' || (o.current && o.current.do === 'more_info')) out.push('more_info');
        if ((o.pages || []).length || (o.current && o.current.do === 'navigate')) out.push('navigate'); out.push('none'); return out; }
    function actionValue(choice, page, pages) { if (choice === 'default') return undefined;
        if (choice === 'navigate') { const p = (pages || []).find(x => x.id === page) || (pages || [])[0]; return p ? { do: 'navigate', page: p.id } : undefined; } return { do: choice }; }
    // Rebuild key: mode, tab, page, selection (ids, types, variants) and what changes the structure of a tab; everything else is patched in place.
    function structureKey(s, tab) {
        const page = s.page || { widgets: [] }, ws = (s.selection || []).map(id => page.widgets.find(w => w.id === id)).filter(Boolean), mode = ws.length === 0 ? 'page' : ws.length === 1 ? 'single' : 'multi', w = ws[0];
        return [mode, mode === 'single' ? tab : '', s.pageId, ws.map(x => x.id + ':' + x.type + ':' + x.variant + ':' + (x.family || '')).join(','), (s.doc && s.doc.look && s.doc.look.family) || '', mode === 'single' && (tab === 'options' || tab === 'actions') ? ((s.doc && s.doc.pages) || []).map(p => p.id + ':' + p.name).join(',') : '',
            mode === 'single' && tab === 'actions' ? JSON.stringify(w.actions || {}) + ':' + ((w.bindings || {}).primary || '') : '', mode === 'multi' ? s.primary : ''].join('|');
    }
    const familyCount = s => new Set(((s.page && s.page.widgets) || []).map(w => w.family || (s.doc && s.doc.look.family) || 'glass')).size;
    function draftOf(catalog, w) { const spec = catalog.widget(w.type); if (!spec) return null; const empty = v => v === undefined || v === null || v === '';
        const b = (spec.bindings || []).find(x => x && x.required && empty((w.bindings || {})[x.name])); if (b) return { binding: b.name };
        const o = (spec.options || []).find(x => x && x.required && empty((w.options || {})[x.key])); return o ? { option: o.key } : null; }

    function create(o) {
        const D = o.designer, t = o.t, lang = o.lang || 'en', catalog = o.catalog, store = o.store, api = o.api, life = E.core.bag(), id = uid('hd-insp'), doc = () => (o.host && o.host.ownerDocument) || document;
        const timers = o.timers, root = el('div', { class: 'hd-insp' }), titleEl = el('h2', { class: 'hd-insp-title', id: id + '-t' }), subEl = el('p', { class: 'hd-insp-sub' }), iconBox = el('span', { class: 'hd-insp-icon' });
        const body = el('div', { class: 'hd-insp-body', id: id + '-b' }), foot = el('div', { class: 'hd-insp-foot' });
        const tabsCtl = F.tabs({ tabs: [{ id: 'data', label: t('homedash.inspector.tab_data') }, { id: 'look', label: t('homedash.inspector.tab_look') }, { id: 'options', label: t('homedash.inspector.tab_options') }, { id: 'actions', label: t('homedash.inspector.tab_actions') }],
            value: 'data', label: t('homedash.designer.inspector'), controls: id + '-b', onChange: v => setTab(v) });
        root.append(el('div', { class: 'hd-insp-head' }, [iconBox, el('div', { class: 'hd-insp-heading' }, [titleEl, subEl])]), tabsCtl.el, body, foot);
        let tab = 'data', lastKey = null, fields = new Map(), pickers = [], thumbs = [], destroyed = false;
        const typeName = x => t('homedash.widget.' + x + '.name'), variantName = (x, v) => t('homedash.variant.' + x + '.' + v);
        const cur = () => { const s = D.state(), page = s.page || { widgets: [] }, ws = (s.selection || []).map(i => page.widgets.find(w => w.id === i)).filter(Boolean); return { s, page, ws, w: ws[0] || null }; };
        const exec = cmd => (cmd ? D.exec(cmd) : false), reg = (key, ctl, read) => { fields.set(key, { ctl, read }); return ctl; };
        const view = w => { const b = w && w.bindings && w.bindings.primary; return b ? store.view(b) : null; };
        function errorText(r) { switch (r.error) {
            case 'required': return t('homedash.inspector.value_required'); case 'bad_number': return t('homedash.inspector.value_number'); case 'bad_colour': return t('homedash.inspector.value_colour');
            case 'range': return Number.isFinite(r.min) && Number.isFinite(r.max) ? t('homedash.inspector.value_range', { min: r.min, max: r.max }) : Number.isFinite(r.min) ? t('homedash.inspector.value_min', { min: r.min }) : t('homedash.inspector.value_max', { max: r.max });
            default: return t('homedash.error.' + r.error); } }
        const focused = ctl => { const a = doc().activeElement; return !!a && (ctl.el === a || (ctl.wrap && ctl.wrap.contains(a))); };
        // Text field that flags what the server would refuse and never executes it; blur restores the last valid value and seals the typing window.
        function inlineText(key, label, o2) {
            let field = null, bad = false;
            const ctl = reg(key, F.text({ key, max: o2.max, label, placeholder: o2.placeholder, value: o2.read(),
                onInput: v => { const issue = F.textIssue(v); bad = !!issue; if (issue) { field.error(t('homedash.error.' + issue)); return; } field.error(''); o2.apply(v); },
                onBlur: v => { if (bad || (o2.required && !String(v).trim())) { ctl.set(o2.read()); field.error(''); bad = false; } D.seal(); } }), o2.read);
            field = F.field({ label, control: ctl, help: o2.help }); return field;
        }
        function disclosure(key, label, read, pick, allowAuto) {
            const gridId = uid('hd-ig'), name = n => (n ? t('homedash.icon.' + n) : t('homedash.inspector.icon_auto')), preview = el('span', { class: 'hd-insp-icopv' }), text = el('span', { text: '' });
            const grid = F.iconGrid({ t, label, value: read(), allowAuto, onPick: n => pick(n) }), box = el('div', { class: 'hd-icongrid-box', id: gridId, hidden: true }, grid.el);
            const btn = el('button', { type: 'button', class: 'hd-btn hd-insp-iconbtn', 'data-tone': 'default', 'aria-expanded': 'false', 'aria-controls': gridId, 'data-hd-field': key }, [preview, text]);
            const paint = v => { preview.replaceChildren(E.icons.icon(v || 'mode-auto')); text.textContent = name(v); };
            btn.addEventListener('click', () => { const on = btn.getAttribute('aria-expanded') !== 'true'; btn.setAttribute('aria-expanded', on ? 'true' : 'false'); setAttr(box, 'hidden', on ? null : true); if (on) grid.focus(); });
            const ctl = { el: btn, set(v) { paint(v || ''); grid.set(v || ''); }, focus() { btn.focus(); } }; ctl.set(read()); reg(key, ctl, read);
            return F.field({ label, control: ctl, wrap: el('div', { class: 'hd-insp-disclosure' }, [btn, box]) });
        }
        function familySelect(key, label, ids, mixedLabel) {
            const k = cur(), dash = (k.s.doc && k.s.doc.look && k.s.doc.look.family) || 'glass', fam = () => { const ws = cur().ws, set = new Set(ws.map(w => w.family || '')); return set.size === 1 ? [...set][0] : '__mixed'; };
            const opts = (mixed) => (mixed ? [{ id: '__mixed', label: t('homedash.inspector.family_mixed'), disabled: true }] : []).concat([{ id: '', label: mixedLabel || t('homedash.inspector.family_default', { family: t('homedash.family.' + dash) }) }], catalog.families().map(f => ({ id: f, label: t('homedash.family.' + f) })));
            const ctl = F.select({ key, label, options: opts(fam() === '__mixed'), value: fam(), onChange: v => { if (v !== '__mixed') exec(C.applyFamily(D.doc(), { pageId: cur().s.pageId, ids: cur().ws.map(w => w.id), family: v })); } });
            reg(key, { el: ctl.el, wrap: ctl.wrap, set(v) { ctl.options(opts(v === '__mixed'), v); }, focus: ctl.focus }, fam); return F.field({ label, control: ctl });
        }
        // ---- page ----
        function buildPage() {
            const k = cur(), pageId = k.s.pageId, read = () => (cur().page.name || '');
            body.appendChild(el('p', { class: 'hd-insp-note', text: t('homedash.inspector.page_hint') }));
            body.appendChild(inlineText('page:name', t('homedash.inspector.page_name'), { max: 40, read, required: true, apply: v => { if (v.trim()) exec(C.renamePage(D.doc(), { pageId, name: v })); } }).el);
            body.appendChild(disclosure('page:icon', t('homedash.inspector.page_icon'), () => cur().page.icon || 'dashboard', n => exec(C.setPageIcon(D.doc(), { pageId, icon: n || undefined })), false).el);
            const tg = reg('page:compact', F.toggle({ key: 'page:compact', label: t('homedash.inspector.page_compact'), value: cur().page.compact !== false, onChange: on => exec(C.setPageCompact(D.doc(), { pageId, compact: on })) }), () => cur().page.compact !== false);
            body.appendChild(F.field({ label: t('homedash.inspector.page_compact'), control: tg, inline: true }).el);
        }
        // ---- data ----
        function buildData(w) {
            const spec = catalog.widget(w.type) || { bindings: [] }, k = cur();
            if (draftOf(catalog, w)) body.appendChild(el('p', { class: 'hd-insp-note', 'data-tone': 'warn', text: t('homedash.inspector.draft_note') }));
            for (const b of spec.bindings || []) {
                const label = b.name === 'primary' ? t('homedash.inspector.binding_primary') : b.name === 'humidity' ? t('homedash.inspector.binding_humidity') : b.name;
                const pk = E.entityPicker.create({ designer: D, t, lang, catalog, store, api, timers, type: w.type, binding: b.name, label, required: b.required === true, value: (w.bindings || {})[b.name],
                    onPick: entity => exec(C.bind(D.doc(), { pageId: k.s.pageId, id: w.id, binding: b.name, entity })) });
                pickers.push(pk); fields.set('binding:' + b.name, { ctl: { el: pk.input, wrap: pk.el, set: v => pk.set(v), focus: pk.focus }, read: () => ((cur().w || {}).bindings || {})[b.name] }); body.appendChild(pk.el);
            }
        }
        // ---- look ----
        function buildLook(w) {
            const k = cur(), pageId = k.s.pageId, wid = w.id, spec = catalog.widget(w.type) || { variants: [] }, def = E.widgets.get(w.type), ids = (spec.variants || []).map(v => v.id).filter(v => !def || def.variants.includes(v));
            const gallery = el('div', { class: 'hd-insp-variants', role: 'radiogroup', 'aria-label': t('homedash.inspector.variant') }), hits = [];
            for (const v of ids) {
                const box = el('div', { class: 'hd-insp-thumb' }), hit = el('button', { type: 'button', class: 'hd-insp-vhit', role: 'radio', 'aria-checked': w.variant === v ? 'true' : 'false', tabindex: '-1', 'data-hd-field': 'variant:' + v, text: variantName(w.type, v) });
                gallery.appendChild(el('div', { class: 'hd-insp-variant' }, [box, hit])); hits.push(hit);
                thumbs.push(E.samples.thumb({ widget: Object.assign({}, w, { variant: v, options: catalog.options(w) }), family: w.family || (k.s.doc && k.s.doc.look && k.s.doc.look.family) || 'glass', catalog, t, lang, box: { width: 132, height: 80 }, parent: box, entity: b => { const e = (w.bindings || {})[b]; return e ? store.view(e) : null; } }));
            }
            const paintV = () => { const now = (cur().w || {}).variant; hits.forEach((h, i) => { const on = h.getAttribute('data-hd-field') === 'variant:' + now; h.setAttribute('aria-checked', on ? 'true' : 'false'); h.setAttribute('tabindex', on || (!hits.some(x => x.getAttribute('aria-checked') === 'true') && i === 0) ? '0' : '-1'); }); };
            const chooseV = v => exec(C.variant(D.doc(), { pageId, id: wid, variant: v, limits: catalog.constraints({ type: w.type, variant: v }) }));
            gallery.addEventListener('click', ev => { const h = ev.target && ev.target.closest ? ev.target.closest('.hd-insp-vhit') : null; if (h) chooseV(h.getAttribute('data-hd-field').slice(8)); });
            gallery.addEventListener('keydown', ev => { const at = hits.findIndex(h => h === ev.target); if (at < 0) return; const n = F.gridMove(ev.key, at, [hits.length], 2); if (n === null) return; ev.preventDefault(); hits[n].focus(); chooseV(hits[n].getAttribute('data-hd-field').slice(8)); });
            paintV(); fields.set('variants', { ctl: { el: gallery, set: paintV, focus() { (hits.find(h => h.getAttribute('tabindex') === '0') || hits[0]).focus(); } }, read: () => null });
            body.appendChild(F.section(t('homedash.inspector.variant'), gallery));
            const entityName = (view(w) || {}).name, ttl = inlineText('title', t('homedash.inspector.title'), { max: 80, placeholder: entityName || typeName(w.type), read: () => (cur().w || {}).title || '',
                apply: v => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['title'], value: v || undefined, text: true, label: 'homedash.cmd.title' })) });
            body.appendChild(ttl.el);
            body.appendChild(disclosure('icon', t('homedash.inspector.icon'), () => (cur().w || {}).icon || '', n => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['icon'], value: n || undefined, label: 'homedash.cmd.icon' })), true).el);
            body.appendChild(familySelect('family', t('homedash.inspector.family'), [wid]).el);
            if (familyCount(k.s) > 2) body.appendChild(el('p', { class: 'hd-insp-note', 'data-tone': 'warn', text: t('homedash.inspector.family_warn') }));
        }
        // ---- options ----
        function enumLabel(v) { return v === 'start' ? t('homedash.inspector.enum_start') : v === 'center' ? t('homedash.inspector.enum_center') : v; }
        function optionField(w, spec) {
            const k = cur(), pageId = k.s.pageId, wid = w.id, f = fieldFor(spec, (w.options || {})[spec.key], { pages: k.s.doc.pages, currentPageId: pageId }); if (!f) return null;
            const key = 'option:' + spec.key, label = t('homedash.option.' + spec.key + '.label') + (f.unit ? ' (' + f.unit + ')' : ''), read = () => { const x = cur().w; return fieldFor(spec, ((x || {}).options || {})[spec.key], { pages: cur().s.doc.pages, currentPageId: pageId }).value; };
            let field = null, bad = false, ctl = null;
            const apply = (raw, live) => { const r = commitValue(spec, raw); bad = !r.ok; if (!r.ok) { field.error(errorText(r)); return false; } field.error('');
                exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['options', spec.key], value: r.value, label: 'homedash.cmd.option', text: !!live })); return true; };
            switch (f.control) {
                case 'switch': ctl = F.toggle({ key, label, value: f.value, onChange: on => apply(on) }); break;
                case 'number': ctl = F.number({ key, label, value: f.value, min: f.min, max: f.max, step: f.step, unit: f.unit, onCommit: raw => apply(raw) }); break;
                case 'text': ctl = F.text({ key, label, value: f.value, max: f.maxLen, onInput: v => apply(v, true), onBlur: () => { if (bad) { ctl.set(read()); field.error(''); bad = false; } D.seal(); } }); break;
                case 'select': ctl = F.select({ key, label, value: f.value, options: f.options.map(v => ({ id: v, label: enumLabel(v) })), onChange: v => apply(v) }); break;
                case 'color': ctl = F.colour({ key, label, value: f.value, allowEmpty: f.allowEmpty, clearLabel: t('homedash.inspector.colour_auto'), placeholder: t('homedash.inspector.colour_auto'), onInput: hex => apply(hex, true),
                    onCommit: hex => { apply(hex); D.seal(); }, onInvalid: () => field.error(t('homedash.inspector.value_colour')) }); break;
                default: ctl = F.select({ key, label, value: f.value, options: [{ id: '', label: t('homedash.inspector.page_choose') }].concat(f.options.map(p => ({ id: p, label: (k.s.doc.pages.find(x => x.id === p) || {}).name || p }))), onChange: v => apply(v) });
            }
            reg(key, ctl, read); field = F.field({ label, help: f.help ? t('homedash.option.' + spec.key + '.help') : '', required: f.required ? t('homedash.inspector.required') : false, control: ctl, inline: f.control === 'switch' }); return field;
        }
        function buildOptions(w) { for (const spec of catalog.optionSpecs(w.type)) { const f = optionField(w, spec); if (f) body.appendChild(f.el); } }
        // ---- actions ----
        function buildActions(w) {
            const k = cur(), pageId = k.s.pageId, wid = w.id, pages = k.s.doc.pages.filter(p => p.id !== pageId), kind = (view(w) || {}).kind;
            const label = c => ({ default: t('homedash.inspector.do_default'), toggle: t('homedash.inspector.do_toggle'), more_info: t('homedash.inspector.do_more_info'), navigate: t('homedash.inspector.do_navigate'), none: t('homedash.inspector.do_none') })[c];
            for (const which of ['tap', 'hold']) {
                const whichLabel = which === 'tap' ? t('homedash.inspector.tap') : t('homedash.inspector.hold'), current = (w.actions || {})[which], choice = current && current.do ? current.do : 'default', choices = actionChoices({ canToggle: catalog.canToggle(w.type), entityKind: kind, pages, current });
                const sel = F.select({ key: 'action:' + which, label: whichLabel, options: choices.map(c => ({ id: c, label: label(c) })), value: choice, onChange: v => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['actions', which], value: actionValue(v, null, pages), label: 'homedash.cmd.action' })) });
                reg('action:' + which, sel, () => { const a = ((cur().w || {}).actions || {})[which]; return a && a.do ? a.do : 'default'; }); body.appendChild(F.field({ label: whichLabel, control: sel }).el);
                if (choice === 'navigate') { const pg = F.select({ key: 'action:' + which + ':page', label: t('homedash.inspector.target_page'), options: pages.map(p => ({ id: p.id, label: p.name })), value: current.page || '', onChange: v => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['actions', which], value: actionValue('navigate', v, pages), label: 'homedash.cmd.action' })) });
                    reg('action:' + which + ':page', pg, () => (((cur().w || {}).actions || {})[which] || {}).page || ''); body.appendChild(F.field({ label: t('homedash.inspector.target_page'), control: pg }).el); }
            }
            const spec = catalog.widget(w.type) || {}; if (spec.commands && Array.isArray(spec.commands.primary) && spec.commands.primary.length) {
                const tg = reg('confirm', F.toggle({ key: 'confirm', label: t('homedash.inspector.confirm'), value: w.confirm === true, onChange: on => exec(C.setWidget(D.doc(), { pageId, id: wid, path: ['confirm'], value: on, label: 'homedash.cmd.option' })) }), () => (cur().w || {}).confirm === true);
                body.appendChild(F.field({ label: t('homedash.inspector.confirm'), help: t('homedash.inspector.confirm_hint'), control: tg, inline: true }).el);
            }
        }
        // ---- several ----
        function buildMulti() {
            const k = cur(), pageId = k.s.pageId, ids = k.ws.map(w => w.id), primary = k.page.widgets.find(w => w.id === k.s.primary) || k.ws[k.ws.length - 1];
            body.appendChild(familySelect('family', t('homedash.inspector.multi_family'), ids, t('homedash.inspector.family_default', { family: t('homedash.family.' + ((k.s.doc.look && k.s.doc.look.family) || 'glass')) })).el);
            const edge = e => exec(C.align(D.doc(), { pageId, bp: cur().s.bp || 'lg', ids: cur().ws.map(w => w.id), edge: e, primaryId: cur().s.primary, C: wid => D.constraints(wid) }));
            const name = primary ? primary.title || typeName(primary.type) : '';
            body.appendChild(F.section(t('homedash.cmd.align'), [el('div', { class: 'hd-f-row' }, [F.button({ label: t('homedash.inspector.align_left'), icon: 'align-left', onClick: () => edge('left') }).el, F.button({ label: t('homedash.inspector.align_top'), icon: 'align-top', onClick: () => edge('top') }).el]),
                el('p', { class: 'hd-f-help', text: t('homedash.inspector.align_hint', { name }) })]));
            foot.appendChild(F.button({ label: t('homedash.inspector.delete_many', { count: ids.length }), icon: 'trash', tone: 'danger-quiet', onClick: () => exec(C.remove(D.doc(), { pageId, ids: cur().ws.map(w => w.id) })) }).el);
        }
        // ---- render ----
        function head(k) {
            const n = k.ws.length; let icon = 'dashboard', title = k.page.name || '', sub = t('homedash.inspector.page_title');
            if (n === 1) { const w = k.w, v = view(w); icon = E.widgets.iconFor(w, v) || 'entity'; title = w.title || (v && v.name) || typeName(w.type); sub = typeName(w.type) + DOT + variantName(w.type, w.variant); }
            else if (n > 1) { icon = 'layers'; title = t('homedash.inspector.multi_title', { count: n }); sub = ''; }
            iconBox.replaceChildren(E.icons.icon(icon)); titleEl.textContent = title; subEl.textContent = sub; setAttr(subEl, 'hidden', sub ? null : true);
        }
        function destroyBuilt() { for (const p of pickers) p.destroy(); for (const th of thumbs) th.destroy(); pickers = []; thumbs = []; fields = new Map(); }
        function rebuild(k) {
            const a = doc().activeElement, node = a && body.contains(a) && a.closest ? a.closest('[data-hd-field]') : null, keep = node ? node.getAttribute('data-hd-field') : null;
            destroyBuilt(); body.replaceChildren(); foot.replaceChildren();
            const n = k.ws.length; setAttr(tabsCtl.el, 'hidden', n === 1 ? null : true); setAttr(body, 'role', n === 1 ? 'tabpanel' : 'group'); setAttr(body, 'aria-labelledby', n === 1 ? tabsCtl.idOf(tab) : titleEl.id);
            if (n === 0) buildPage(); else if (n > 1) buildMulti(); else {
                const w = k.w; tabsCtl.set(tab);
                if (tab === 'data') buildData(w); else if (tab === 'look') buildLook(w); else if (tab === 'options') buildOptions(w); else buildActions(w);
                foot.appendChild(F.button({ label: t('homedash.inspector.delete_one'), icon: 'trash', tone: 'danger-quiet', onClick: () => exec(C.remove(D.doc(), { pageId: cur().s.pageId, ids: [w.id] })) }).el);
            }
            if (keep) { const next = body.querySelector('[data-hd-field="' + keep + '"]'); if (next && typeof next.focus === 'function') next.focus(); }
        }
        function render() {
            if (destroyed) return; const k = cur(), key = structureKey(k.s, tab); head(k);
            if (key !== lastKey) { lastKey = key; rebuild(k); return; }
            for (const f of fields.values()) if (!focused(f.ctl)) f.ctl.set(f.read(k.s));
        }
        function setTab(next) { if (!TABS.includes(next)) return; tab = next; render(); }
        life.add(D.on('state', render)); life.add(store.on('change', () => { for (const th of thumbs) th.update(); }));
        life.on(root, 'keydown', ev => { if (ev.key !== 'Escape' || ev.defaultPrevented) return; ev.preventDefault(); const s = D.state(); if (s.drawers) D.closePanels(); const idf = s.primary || (s.selection || [])[0]; if (idf) D.focusFrame(idf); });
        // Choose devices (Q1): one selected draft goes to the field that is missing; otherwise the first control of the body.
        function focusFirst() {
            render(); const k = cur(), draft = k.ws.length === 1 ? draftOf(catalog, k.w) : null;
            if (draft) { setTab(draft.binding ? 'data' : 'options'); const f = body.querySelector('[data-hd-field="' + (draft.binding ? 'binding:' + draft.binding : 'option:' + draft.option) + '"]'); if (f) { f.focus(); return; } }
            const first = body.querySelector('input, select, button, [tabindex="0"]'); if (first) first.focus(); else tabsCtl.focus();
        }
        render(); if (o.host) o.host.appendChild(root);
        return Object.freeze({ el: root, focusFirst, setTab, destroy() { if (destroyed) return; destroyed = true; life.dispose(); destroyBuilt(); root.remove(); } });
    }
    E.inspector = Object.freeze({ create, TABS, model: Object.freeze({ fieldFor, commitValue, actionChoices, actionValue, structureKey, familyCount, draftOf }) });
})();
