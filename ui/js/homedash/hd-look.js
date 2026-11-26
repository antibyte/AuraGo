// HomeDash look editor (M4 part 2, D7, design 5.12): family cards with live previews of three widgets of the dashboard, colour mode (theme, dark,
// light, custom with accent, surface hue and scheme), backdrop, density, radius, fit, row unit and page compaction, in one end-docked E.sheet form
// over a live canvas. Every field is one undoable D.exec(E.commands.setDoc ...) (sliders and the native colour input use text:true while they move
// and seal on change). The pure model follows the server rules: lower-case #rrggbb accent, integer surface_hue 0-359, scheme dark|light.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.samples || !E.commands || !E.colour) throw new Error('hd-look.js: load hd-form.js, hd-samples.js and hd-commands.js first');
    const { el, svg, setAttr, setVars, uid } = E.core, F = E.form, C = E.commands;
    const FAMILIES = Object.freeze(['glass', 'tactile', 'minimal', 'instrument']), MODES = Object.freeze(['theme', 'dark', 'light', 'custom']), BACKDROPS = Object.freeze(['theme', 'solid', 'gradient']), DENSITIES = Object.freeze(['compact', 'comfortable', 'spacious']);
    const HEX = /^#?([0-9a-f]{6})$/i, SLOT_A = Object.freeze({ width: 64, height: 88 }), SLOT_B = Object.freeze({ width: 64, height: 40 });
    const hueOf = v => { const n = Math.round(Number(v)); return Number.isFinite(n) ? ((n % 360) + 360) % 360 : 220; };
    // Server rules (dashboard_validate.go): lower-case #rrggbb accent, integer surface_hue 0-359, scheme dark|light.
    function custom(c, seed) { const x = c && typeof c === 'object' ? c : {}, f = seed || {}, m = HEX.exec(String(x.accent || '').trim()) || HEX.exec(String(f.accent || '').trim());
        return { accent: m ? '#' + m[1].toLowerCase() : '#5b8def', surface_hue: hueOf(x.surface_hue !== undefined ? x.surface_hue : f.surface_hue), scheme: (x.scheme || f.scheme) === 'light' ? 'light' : 'dark' }; }
    // The palette object a mode change writes (server shape). The custom block is kept so custom comes back as it was; E.doc.forSave drops it unless
    // the mode is custom and writes the theme snapshot through the lane's prepare (part 1).
    function paletteFor(current, mode, seed) { const cur = current && typeof current === 'object' ? current : {}, out = Object.assign({}, cur, { mode: MODES.includes(mode) ? mode : 'theme' });
        if (out.mode === 'custom' || cur.custom) out.custom = custom(cur.custom, seed); return out; }
    const radius = v => Math.round(E.core.clamp(Number(v) || 1, 0.5, 1.5) * 20) / 20;
    const rowUnit = v => (v === 'auto' || v === undefined || v === null || v === '' ? 'auto' : Math.round(E.core.clamp(Number(v) || 96, 48, 200)));
    function swatches(c) { const k = E.colour.solvePalette(custom(c)).tokens; return { canvas: k.canvas, surface: k['surface-solid'], ink: k.ink, accent: k.accent, on: k.on }; }
    const SAMPLES = Object.freeze([['light', 'dial', 'a'], ['switch', 'pill', 'b'], ['temperature', 'numeral', 'c']]);
    // Family card previews (D7): one square widget (thumb 2x2, slot a) and two wide ones (2x1, slots b and c) of the page in reading order; samples fill gaps.
    function previewWidgets(page, catalog, registry) {
        const out = {}, items = page && page.layouts ? E.grid.sortReading(page.layouts.lg) : [];
        for (const it of items) { const w = page.widgets.find(x => x.id === it.id); if (!w || !registry.get(w.type)) continue; const s = E.samples.thumbSize(catalog, w.type, w.variant);
            const slot = s.w === 2 && s.h === 2 ? 'a' : s.h === 1 ? (out.b ? 'c' : 'b') : null; if (slot && !out[slot]) out[slot] = { widget: w, real: true }; }
        for (const [type, variant, slot] of SAMPLES) if (!out[slot] && registry.get(type) && catalog.variant(type, variant))
            out[slot] = { widget: { type, variant, title: '', bindings: {}, options: catalog.optionDefaults(type), actions: {}, confirm: false }, real: false };
        return ['a', 'b', 'c'].filter(k => out[k]).map(k => Object.assign({ slot: k, size: k === 'a' ? { w: 2, h: 2 } : { w: 2, h: 1 } }, out[k]));
    }

    function open(ctx) {
        const D = ctx.designer, t = ctx.t || D.t, lang = ctx.lang || D.lang || 'en', catalog = ctx.catalog || D.catalog, store = ctx.store || D.store, life = E.core.bag(), thumbs = [], controls = [];
        const set = (path, value, label, text) => D.exec(C.setDoc(D.doc(), { path, value, label: label || 'homedash.cmd.look', text: !!text }));
        const doc = () => D.doc(), look = () => doc().look, seed = () => ({ accent: (E.widgets.paletteSnapshot(ctx.root, ['accent']) || {}).accent, scheme: ctx.root && ctx.root.getAttribute && ctx.root.getAttribute('data-hd-scheme') === 'light' ? 'light' : 'dark', surface_hue: 250 });
        const ownerDoc = () => (ctx.root && ctx.root.ownerDocument) || document, focused = ctl => { const a = ownerDoc().activeElement; return !!a && (ctl.el === a || (ctl.wrap && ctl.wrap.contains(a))); };
        const track = (ctl, read) => { controls.push({ ctl, read }); return ctl; };
        const content = el('div', { class: 'hd-look' });
        // Family
        const famEls = FAMILIES.map(f => { const prev = el('div', { class: 'hd-look-preview' }), hit = el('button', { type: 'button', class: 'hd-look-fhit', role: 'radio', 'aria-checked': 'false', tabindex: '-1', 'data-family': f },
            [el('strong', { text: t('homedash.family.' + f) }), el('small', { text: ({ glass: t('homedash.look.desc_glass'), tactile: t('homedash.look.desc_tactile'), minimal: t('homedash.look.desc_minimal'), instrument: t('homedash.look.desc_instrument') })[f] })]);
            for (const p of previewWidgets(doc().pages.find(x => x.id === D.state().pageId) || doc().pages[0], catalog, E.widgets)) { const slot = el('span', { class: 'hd-look-slot', 'data-slot': p.slot }); prev.appendChild(slot);
                thumbs.push(E.samples.thumb({ widget: Object.assign({}, p.widget, { family: f }), catalog, t, lang, size: p.size, box: p.slot === 'a' ? SLOT_A : SLOT_B, parent: slot, entity: b => { const e = p.real && (p.widget.bindings || {})[b]; return e ? store.view(e) : null; } })); }
            return { f, card: el('div', { class: 'hd-look-family' }, [prev, hit]), hit }; });
        const famGroup = el('div', { class: 'hd-look-families', role: 'radiogroup', 'aria-label': t('homedash.look.family') }, famEls.map(x => x.card));
        const paintFam = () => famEls.forEach((x, i) => { const on = x.f === look().family; x.hit.setAttribute('aria-checked', on ? 'true' : 'false'); x.hit.setAttribute('tabindex', on || (!FAMILIES.includes(look().family) && i === 0) ? '0' : '-1'); });
        const chooseFam = f => set(['look', 'family'], f);
        famGroup.addEventListener('click', ev => { const h = ev.target && ev.target.closest ? ev.target.closest('.hd-look-fhit') : null; if (h) chooseFam(h.getAttribute('data-family')); });
        famGroup.addEventListener('keydown', ev => { const at = famEls.findIndex(x => x.hit === ev.target); if (at < 0) return; const n = F.gridMove(ev.key, at, [famEls.length], 2); if (n === null) return; ev.preventDefault(); famEls[n].hit.focus(); chooseFam(famEls[n].f); });
        controls.push({ ctl: { el: famGroup, set: paintFam }, read: () => null }); content.appendChild(F.section(t('homedash.look.family'), famGroup));
        // Colours
        const modeLabels = { theme: t('homedash.look.mode_theme'), dark: t('homedash.look.mode_dark'), light: t('homedash.look.mode_light'), custom: t('homedash.look.mode_custom') };
        const mode = track(F.segmented({ key: 'palette-mode', label: t('homedash.look.colours'), options: MODES.map(m => ({ id: m, label: modeLabels[m] })), value: look().palette.mode, onChange: m => set(['look', 'palette'], paletteFor(look().palette, m, seed())) }), () => look().palette.mode);
        const customBox = el('div', { class: 'hd-look-custom', hidden: true });
        const accent = track(F.colour({ key: 'palette-accent', label: t('homedash.look.accent'), value: '', onInput: hex => { set(['look', 'palette', 'custom', 'accent'], hex, null, true); paintSw(); }, onCommit: hex => { set(['look', 'palette', 'custom', 'accent'], hex); D.seal(); paintSw(); } }), () => custom(look().palette.custom).accent);
        const gid = uid('hd-hue'), hue = svg('svg', { class: 'hd-look-hue', 'aria-hidden': 'true', viewBox: '0 0 100 8', preserveAspectRatio: 'none' }), grad = svg('linearGradient', { id: gid, x1: '0', x2: '1', y1: '0', y2: '0' });
        for (let i = 0; i <= 12; i++) grad.appendChild(svg('stop', { offset: String(Math.round(i / 12 * 100) / 100), 'stop-color': E.colour.oklchToHex([0.72, 0.11, i * 30]).toLowerCase() }));
        hue.append(svg('defs', {}, grad), svg('rect', { x: '0', y: '0', width: '100', height: '8', fill: 'url(#' + gid + ')' }));
        const hueCtl = track(F.range({ key: 'palette-hue', label: t('homedash.look.surface_hue'), min: 0, max: 359, step: 1, value: 220, track: hue, onInput: v => { set(['look', 'palette', 'custom', 'surface_hue'], hueOf(v), null, true); paintSw(); }, onCommit: v => { set(['look', 'palette', 'custom', 'surface_hue'], hueOf(v)); D.seal(); paintSw(); } }), () => custom(look().palette.custom).surface_hue);
        const scheme = track(F.segmented({ key: 'palette-scheme', label: t('homedash.look.scheme'), options: [{ id: 'dark', label: t('homedash.look.mode_dark') }, { id: 'light', label: t('homedash.look.mode_light') }], value: 'dark', onChange: v => { set(['look', 'palette', 'custom', 'scheme'], v); paintSw(); } }), () => custom(look().palette.custom).scheme);
        const swEls = ['canvas', 'surface', 'ink', 'accent', 'on'].map(n => el('span', { class: 'hd-look-swatch', 'data-swatch': n })), swBox = el('div', { class: 'hd-look-swatches', 'aria-hidden': 'true' }, swEls);
        function paintSw() { const sw = swatches(look().palette.custom); swEls.forEach(s => setVars(s, { '--hd-e-sw': sw[s.getAttribute('data-swatch')] })); }
        customBox.append(F.field({ label: t('homedash.look.accent'), control: accent }).el, F.field({ label: t('homedash.look.surface_hue'), control: hueCtl }).el, F.field({ label: t('homedash.look.scheme'), control: scheme }).el, swBox, el('p', { class: 'hd-f-help', text: t('homedash.look.custom_hint') }));
        content.appendChild(F.section(t('homedash.look.colours'), [mode, customBox]));
        // Surface
        const backdrop = track(F.segmented({ key: 'backdrop', label: t('homedash.look.backdrop'), options: [{ id: 'theme', label: t('homedash.look.mode_theme') }, { id: 'solid', label: t('homedash.look.backdrop_solid') }, { id: 'gradient', label: t('homedash.look.backdrop_gradient') }], value: 'theme',
            onChange: v => set(['look', 'backdrop'], { kind: v }) }), () => (look().backdrop && look().backdrop.kind) || 'theme');
        const density = track(F.segmented({ key: 'density', label: t('homedash.look.density'), options: [{ id: 'compact', label: t('homedash.look.density_compact') }, { id: 'comfortable', label: t('homedash.look.density_comfortable') }, { id: 'spacious', label: t('homedash.look.density_spacious') }], value: 'comfortable',
            onChange: v => set(['look', 'density'], v) }), () => look().density);
        const nf = new Intl.NumberFormat(lang, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
        const rad = track(F.range({ key: 'radius', label: t('homedash.look.radius'), min: 0.5, max: 1.5, step: 0.05, value: 1, valueText: v => t('homedash.look.radius_value', { value: nf.format(v) }), onInput: v => set(['look', 'radius_scale'], radius(v), null, true), onCommit: v => { set(['look', 'radius_scale'], radius(v)); D.seal(); } }), () => radius(look().radius_scale));
        content.appendChild(F.section(t('homedash.look.surface'), [backdrop, F.field({ label: t('homedash.look.radius'), control: rad }).el]));   // G17: Background and Corner rounding
        // Grid
        const fit = track(F.segmented({ key: 'fit', label: t('homedash.look.fit'), options: [{ id: 'scroll', label: t('homedash.look.fit_scroll') }, { id: 'fit_screen', label: t('homedash.menu.fit') }], value: 'scroll', onChange: v => set(['grid', 'fit'], v, 'homedash.cmd.grid') }), () => doc().grid.fit);
        const rowKind = track(F.segmented({ key: 'row-kind', label: t('homedash.look.row_unit'), options: [{ id: 'auto', label: t('homedash.look.row_auto') }, { id: 'fixed', label: t('homedash.look.row_fixed') }], value: 'auto',
            onChange: v => { set(['grid', 'row_unit'], v === 'auto' ? 'auto' : rowUnit(rowPx.get() || 96), 'homedash.cmd.grid'); sync(); } }), () => (doc().grid.row_unit === 'auto' ? 'auto' : 'fixed'));
        const rowPx = F.number({ key: 'row-px', label: t('homedash.look.row_unit'), min: 48, max: 200, step: 4, unit: 'px', value: 96, onCommit: raw => set(['grid', 'row_unit'], rowUnit(raw), 'homedash.cmd.grid') });
        rowPx.get = () => Number(rowPx.el.value); track(rowPx, () => (doc().grid.row_unit === 'auto' ? '' : doc().grid.row_unit));
        const compact = track(F.toggle({ key: 'compact', label: t('homedash.look.compaction'), value: true, onChange: on => D.exec(C.setPageCompact(D.doc(), { pageId: D.state().pageId, compact: on })) }), () => { const p = doc().pages.find(x => x.id === D.state().pageId); return !p || p.compact !== false; });
        const rowWrap = rowPx.wrap;
        content.appendChild(F.section(t('homedash.look.grid'), [F.field({ label: t('homedash.look.density'), control: density }).el, F.field({ label: t('homedash.look.fit'), help: t('homedash.look.fit_note'), control: fit }).el, F.field({ label: t('homedash.look.row_unit'), control: rowKind }).el, rowWrap, F.field({ label: t('homedash.look.compaction'), help: t('homedash.look.compaction_help'), control: compact, inline: true }).el]));
        function sync() {
            const isCustom = look().palette.mode === 'custom'; setAttr(customBox, 'hidden', isCustom ? null : true); setAttr(rowWrap, 'hidden', doc().grid.row_unit === 'auto' ? true : null);
            for (const c of controls) { if (c.ctl.el && focused(c.ctl)) continue; const v = c.read(); if (v !== null) c.ctl.set(v); else c.ctl.set(); }
            if (isCustom) paintSw();
        }
        sync();
        const off = D.on('state', sync), offStore = store.on('change', () => { for (const th of thumbs) th.update(); });
        const first = () => famEls.find(x => x.hit.getAttribute('aria-checked') === 'true') ? famEls.find(x => x.hit.getAttribute('aria-checked') === 'true').hit : famEls[0].hit;
        return F.formSheet(D.sheet(), { title: t('homedash.look.title'), content, actions: [{ id: 'done', label: t('homedash.designer.done'), tone: 'primary' }], first }).then(() => {
            off(); offStore(); life.dispose(); for (const th of thumbs) th.destroy(); thumbs.length = 0; D.seal(); return true; });
    }
    E.look = Object.freeze({ open, model: Object.freeze({ custom, paletteFor, radius, rowUnit, swatches, previewWidgets, hueOf, FAMILIES, MODES, BACKDROPS, DENSITIES }) });
})();
