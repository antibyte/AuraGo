// HomeDash control lab gallery (test fixture, never shipped). Renders every primitive and widget in every state for one family,
// palette, motion and size set chosen by the URL (contract C4) and exposes window.hdLab to the Go browser test and the M1 style lab.
(function () {
    'use strict';
    // R1: the registry and the hdLab object exist synchronously, before anything can throw, so fixture scripts and the gallery can rely on them.
    const fixtureStore = new Map();
    window.hdLab = { ready: false, error: '', errors: [], instances: [], extra: {},
        fixtures: { define(type, variant, entry) { if (typeof type !== 'string' || !entry || typeof entry !== 'object') throw new Error('hdLab.fixtures.define(type, variant, entry)'); fixtureStore.set(type + '/' + (variant || '*'), entry); },
            get(type, variant) { return fixtureStore.get(type + '/' + variant) || fixtureStore.get(type + '/*') || null; } },
        report(key, value) { hdLab.extra[String(key)] = value; } };
    const hdLab = window.hdLab;
    const resolveView = (entry, s) => { if (s && typeof s === 'object') return s; const f = entry && entry.states && entry.states[s];
        if (typeof f === 'function') return f(s); if (f) return f;
        if (!LAB_STATES.includes(s)) throw new Error('hdLab: unknown state ' + s); return labView(s); };
    const resolveHistory = (entry, s) => { const h = entry && entry.history; return (h && typeof s === 'string' && h[s]) || (h && h.default) || { points: [], source: 'ring' }; };

    const DEG_C = String.fromCodePoint(0xb0) + 'C';
    const FAMILIES = ['glass', 'tactile', 'minimal', 'instrument', 'placeholder'], PALETTES = ['theme', 'dark', 'light', 'custom'], MOTIONS = ['full', 'reduced', 'none'];
    const LAB_STATES = ['off', 'on', 'pending', 'disabled', 'unavailable', 'stale', 'readonly', 'error', 'alert', 'min', 'max', 'focus'];
    const PRIM_VARIANTS = { toggle: ['default'], rocker: ['default'], pushButton: ['chip', 'tile'], slider: ['h', 'v'], knob: ['ring', 'knob'], stepper: ['default'], segmented: ['default'], numeral: ['default'], segReadout: ['default'], arcGauge: ['default'], needleGauge: ['default'], bar: ['h', 'v', 'tank'], lamp: ['default'], badge: ['default'], iconBadge: ['default'], sparkline: ['default'], frame: ['default'], sheet: ['default'], colourWheel: ['default'], ctStrip: ['default'] };
    const LAB_WIDGET_SIZES = { entity: ['1x1', '2x1'] }, CELL = 96, GAP = 12;
    const LAB_WORDS = { 'homedash.state.on': 'On', 'homedash.state.off': 'Off', 'homedash.state.pending': 'Sending', 'homedash.state.unavailable': 'Unavailable', 'homedash.state.unknown': 'Unknown', 'homedash.state.error': 'Failed' };
    function primOpts(type, variant, size, lang) {
        const o = { label: 'Lab ' + type, size, lang, min: 0, max: 100, unit: '%' };
        if (variant !== 'default') o.variant = variant;
        const extra = { pushButton: { label: 'Scene', icon: 'bulb' }, stepper: { min: 5, max: 30, step: 0.5, unit: DEG_C, decimals: 1, decLabel: 'Decrease', incLabel: 'Increase' },
            segmented: { options: [{ id: 'heat', label: 'Heat', icon: 'flame' }, { id: 'cool', label: 'Cool', icon: 'snowflake' }, { id: 'off', label: 'Off' }] },
            numeral: { chars: 5, decimals: 1 }, segReadout: { digits: 4, decimals: 1 }, bar: { readout: true }, iconBadge: { icon: 'droplet' }, badge: { icon: 'alert' }, sparkline: { w: 120, h: 40 },
            frame: { title: 'Living room', sub: 'Sensor', icon: 'thermometer', more: 'More' }, sheet: { title: 'Details', kind: 'moreinfo' }, colourWheel: { diameter: 120 }, ctStrip: { min: 2200, max: 6500, unit: 'K' } }[type];
        if (['arcGauge', 'needleGauge', 'bar'].includes(type)) o.zones = [{ from: 80, to: 90, tone: 'warn' }, { from: 90, to: 100, tone: 'crit' }];
        return Object.assign(o, extra);
    }
    function labState(type, name) {
        const s = Object.assign({}, { off: { on: false, value: 30 }, on: { on: true, value: 70 }, pending: { on: true, value: 70, pending: true }, disabled: { on: false, value: 30, disabled: true }, unavailable: { on: false, value: null, status: 'unavailable' }, stale: { on: true, value: 70, stale: true }, readonly: { on: true, value: 70, readonly: true }, error: { on: false, value: 30, status: 'error' }, alert: { on: true, value: 96, tone: 'crit' }, min: { on: false, value: 0 }, max: { on: true, value: 100 }, focus: { on: true, value: 70 } }[name]);
        const v = s.value;
        if (type === 'segmented') s.value = v === null ? null : ['heat', 'cool', 'off'][v < 34 ? 0 : v < 67 ? 1 : 2];
        if (type === 'stepper') s.value = v === null ? null : 5 + v / 100 * 25;
        if (type === 'ctStrip') s.value = v === null ? null : 2200 + v / 100 * 4300;
        if (type === 'colourWheel') Object.assign(s, { hue: v === null ? 0 : v * 3.6, sat: v === null ? 0 : 0.8 });
        if (type === 'sparkline') s.points = v === null ? [] : [18, 19.5, 19, 20.5, 22, 21.5, 23, 22.5].map(p => p + v / 50);
        if (type === 'badge' || type === 'lamp') s.text = LAB_WORDS['homedash.state.' + (name === 'unavailable' ? 'unavailable' : s.on ? 'on' : 'off')];
        if (type === 'frame') s.stateText = LAB_WORDS['homedash.state.' + (s.on ? 'on' : 'off')];
        if (type === 'sheet') s.open = true;
        if (type === 'numeral' || type === 'segReadout') s.decimals = 1;
        return s;
    }
    function labView(name) {
        return Object.assign({ id: 'lab:' + name, kind: 'switch', name: 'Desk lamp', area: 'Office', unit: '', device_class: '', value: 'off', attributes: {}, available: true, status: 'ok', readonly: false, capabilities: ['toggle'], pending: false, stale: false, error: null, tone: null },
            { off: {}, on: { value: 'on' }, pending: { value: 'on', pending: true }, disabled: { value: 'on', capabilities: [] }, unavailable: { value: null, available: false, status: 'unavailable' }, stale: { value: 'on', stale: true }, readonly: { value: 'on', readonly: true }, error: { status: 'error' }, alert: { kind: 'sensor', name: 'Humidity', value: 96, unit: '%', tone: 'crit' }, min: { kind: 'sensor', name: 'Humidity', value: 0, unit: '%' }, max: { kind: 'sensor', name: 'Humidity', value: 100, unit: '%' }, focus: { value: 'on' } }[name]);
    }

    // ---- gallery -------------------------------------------------------------------------------------------------------------------
    const has = (obj, key) => Object.prototype.hasOwnProperty.call(obj, key);
    const frames = n => new Promise(resolve => { const step = k => (k ? requestAnimationFrame(() => step(k - 1)) : resolve()); step(n); });
    const FOCUS_SELECTOR = '[role="switch"], [role="slider"], [role="radio"][aria-checked="true"], button, [role="spinbutton"]';
    let ctx = null;

    // Rule 1: collect errors that happened before and after this script ran.
    for (const m of (window.hdLabEarlyErrors || [])) hdLab.errors.push(String(m));
    window.addEventListener('error', e => { hdLab.errors.push(String(e.message)); });
    window.addEventListener('unhandledrejection', e => { hdLab.errors.push(String((e.reason && e.reason.message) || e.reason)); });

    function fail(name, value) { throw new Error('bad param ' + name + ': ' + String(value).slice(0, 80)); }
    function parseParams() {
        const q = new URLSearchParams(location.search), get = (name, def) => (q.has(name) ? q.get(name) : def);
        const list = name => (q.get(name) || '').split(',').map(s => s.trim()).filter(Boolean);
        const oneOf = (name, allowed, def) => { const v = get(name, def); if (!allowed.includes(v)) fail(name, v); return v; };
        const p = { family: oneOf('family', FAMILIES, 'placeholder'), palette: oneOf('palette', PALETTES, 'dark'), motion: oneOf('motion', MOTIONS, 'full'),
            backdrop: oneOf('backdrop', ['theme', 'solid', 'gradient'], 'theme'), ease: oneOf('ease', ['native', 'fallback'], 'native') };
        const scheme = get('scheme', 'dark'); if (scheme !== 'dark' && scheme !== 'light') fail('scheme', scheme);
        p.schemeParam = scheme;
        p.theme = get('theme', 'standard'); if (!/^[\w-]{1,40}$/.test(p.theme)) fail('theme', p.theme);
        p.accent = get('accent', '8E5BD9'); if (!/^[0-9a-fA-F]{6}$/.test(p.accent)) fail('accent', p.accent);
        const hue = get('hue', '285'); if (!/^\d{1,3}$/.test(hue) || Number(hue) > 359) fail('hue', hue);
        p.hue = Number(hue);
        const more = get('more', '1'); if (more !== '0' && more !== '1') fail('more', more);
        p.more = Number(more);
        const columns = get('columns', '0'); if (!/^\d{1,3}$/.test(columns)) fail('columns', columns);
        p.columns = Number(columns);
        const dpr = Number(get('dpr', '1')); if (!Number.isFinite(dpr) || dpr <= 0) fail('dpr', get('dpr', '1'));
        p.dpr = dpr;
        p.lang = get('lang', 'en'); if (!/^[a-z]{2,3}(-[A-Za-z0-9]{2,8})?$/.test(p.lang)) fail('lang', p.lang);
        p.types = list('types'); p.variants = list('variants'); p.sizes = list('sizes');
        p.states = list('states'); if (!p.states.length) p.states = LAB_STATES.slice();
        for (const s of p.states) if (!LAB_STATES.includes(s)) fail('states', s);
        for (const s of p.sizes) if (!/^(m|l|\d+x\d+)$/.test(s)) fail('sizes', s);
        p.graphics = [];
        if (q.has('graphics')) { let g; try { g = JSON.parse(q.get('graphics')); } catch (err) { fail('graphics', q.get('graphics')); }
            if (!Array.isArray(g) || !g.every(x => typeof x === 'string')) fail('graphics', q.get('graphics'));
            for (const sel of g) { let ok = sel !== ''; if (ok) { try { document.createElement('div').querySelector(sel); } catch (err) { ok = false; } } if (!ok) fail('graphics', sel); }
            p.graphics = g; }
        p.scripts = list('scripts'); for (const s of p.scripts) if (!/^\/testdata\/[\w.\/-]+\.js$/.test(s) || s.includes('..')) fail('scripts', s);
        if (Math.abs(window.devicePixelRatio - p.dpr) > 0.01) throw new Error('dpr mismatch: want ' + p.dpr + ', devicePixelRatio is ' + window.devicePixelRatio);
        p.scheme = p.palette === 'dark' ? 'dark' : p.palette === 'light' ? 'light' : p.schemeParam;
        return p;
    }
    function loadLink(href, before) {
        return new Promise((resolve, reject) => {
            const link = document.createElement('link'); link.rel = 'stylesheet'; link.href = href;
            link.addEventListener('load', () => resolve()); link.addEventListener('error', () => reject(new Error('stylesheet failed to load: ' + href)));
            if (before && before.parentNode) before.parentNode.insertBefore(link, before); else document.head.appendChild(link);
        });
    }
    function loadScript(src) {
        return new Promise((resolve, reject) => {
            const node = document.createElement('script'); node.src = src;
            node.addEventListener('load', () => resolve()); node.addEventListener('error', () => reject(new Error('script failed to load: ' + src)));
            document.head.appendChild(node);
        });
    }

    function buildPanel(E, p) {
        const root = E.el('section', { class: 'hd-lab-panel hd-root hd-backdrop', 'data-hd-host': p.palette === 'theme' ? 'desktop' : 'kiosk', 'data-hd-family': p.family, 'data-hd-palette': p.palette,
            'data-hd-scheme': p.scheme, 'data-hd-motion': p.motion, 'data-hd-mode': 'view', 'data-hd-density': 'comfortable', 'data-hd-backdrop': p.backdrop,
            'data-hd-ease': p.ease === 'fallback' ? 'fallback' : null });
        if (p.palette === 'custom') E.colour.applyPalette(root, E.colour.solvePalette({ accent: '#' + p.accent, surface_hue: p.hue, scheme: p.scheme }).tokens);
        document.getElementById('hd-lab').appendChild(root);
        return root;
    }

    function addPrimitive(E, p, root, cells, type, variant, size, state, control) {
        const cell = E.el('div', { class: 'hd-lab-cell', 'data-hd-lab-control': control, 'data-hd-lab-state': state });
        const stage = E.el('div', { class: 'hd-lab-stage' });
        cell.appendChild(E.el('span', { class: 'hd-lab-statecap', text: state })); cell.appendChild(stage); cells.appendChild(cell);
        const opts = primOpts(type, variant, size, p.lang); if (type === 'frame' && !p.more) opts.more = null;
        const inst = E.prim[type](opts); inst.set(labState(type, state)); stage.appendChild(inst.el);
        if (state === 'focus') { const target = inst.el.matches(FOCUS_SELECTOR) ? inst.el : inst.el.querySelector(FOCUS_SELECTOR); if (target) target.setAttribute('data-hd-focus', 'ring'); }
        hdLab.instances.push({ kind: 'prim', type, variant, size, control, state, el: inst.el, entry: null,
            apply(n) { if (!LAB_STATES.includes(n)) throw new Error('primitive instances accept only state names, got ' + String(n)); inst.set(labState(type, n)); } });
    }

    let widgetCount = 0;
    function addWidget(E, p, root, cells, type, variant, size, state, control) {
        const cell = E.el('div', { class: 'hd-lab-cell', 'data-hd-lab-control': control, 'data-hd-lab-state': state });
        const stage = E.el('div', { class: 'hd-lab-stage' });
        cell.appendChild(E.el('span', { class: 'hd-lab-statecap', text: state })); cell.appendChild(stage); cells.appendChild(cell);
        const [w, h] = size.split('x').map(Number), width = w * CELL + (w - 1) * GAP, height = h * CELL + (h - 1) * GAP;
        const entry = hdLab.fixtures.get(type, variant), ew = (entry && entry.widget) || {};
        const widget = { id: 'lab-' + (++widgetCount), type, variant, title: '', bindings: { primary: 'lab:' + state }, options: {}, actions: {}, confirm: false };
        for (const key of ['title', 'icon', 'options', 'family']) if (ew[key] !== undefined) widget[key] = ew[key];
        let cur = state;
        const api = () => ({ entity: () => resolveView(entry, cur),
            can: () => { const v = resolveView(entry, cur) || {}; return !(v.readonly || v.available === false || (Array.isArray(v.capabilities) && !v.capabilities.length)); },
            command: () => Promise.resolve({ ok: true }), moreInfo() {}, history: () => Promise.resolve(resolveHistory(entry, cur)), announce() {} });
        const host = E.widgets.host({ widget, size: { w, h }, t: E.core.makeT(LAB_WORDS, { warn() {} }), lang: p.lang, api, moreLabel: p.more === 0 ? null : 'More', parent: stage });
        host.el.style.setProperty('--hd-lab-w', width + 'px'); host.el.style.setProperty('--hd-lab-h', height + 'px');
        host.update(); host.resize({ w, h, px: { width, height } });
        if (state === 'focus') { const more = host.el.querySelector('.hd-more'); if (more) more.setAttribute('data-hd-focus', 'ring'); }
        hdLab.instances.push({ kind: 'widget', type, variant, size, control, state, el: host.el, entry, host, apply(s) { cur = s; host.update(); } });
    }

    function render(E, p, root) {
        const title = E.el('h1', { class: 'hd-lab-title', text: 'HomeDash control lab: family=' + p.family + ' palette=' + p.palette + (p.palette === 'theme' ? ' theme=' + p.theme : '') + ' scheme=' + p.scheme + ' dpr=' + p.dpr + ' motion=' + p.motion });
        root.appendChild(title);
        const types = p.types.length ? p.types : Object.keys(PRIM_VARIANTS).concat(E.widgets.types());
        let rows = 0;
        for (const type of types) {
            const widgetDef = has(PRIM_VARIANTS, type) ? null : E.widgets.get(type), isPrim = has(PRIM_VARIANTS, type);
            const all = isPrim ? PRIM_VARIANTS[type] : widgetDef.variants, variants = p.variants.length ? all.filter(v => p.variants.includes(v)) : all;
            const sizeRe = isPrim ? /^(m|l)$/ : /^\d+x\d+$/, picked = p.sizes.filter(s => sizeRe.test(s));
            const sizes = picked.length ? picked : isPrim ? ['m'] : (LAB_WIDGET_SIZES[type] || ['2x1']);
            for (const variant of variants) for (const size of sizes) {
                const control = type + '/' + variant + '/' + size;
                const row = E.el('div', { class: 'hd-lab-row' }), cells = E.el('div', { class: 'hd-lab-cells' });
                if (p.columns > 0) { cells.setAttribute('data-hd-lab-columns', String(p.columns)); cells.style.setProperty('--hd-lab-columns', String(p.columns)); }
                row.appendChild(E.el('div', { class: 'hd-lab-rowhead', text: type + ' / ' + variant + ' / ' + size })); row.appendChild(cells); root.appendChild(row); rows++;
                for (const state of p.states) (isPrim ? addPrimitive : addWidget)(E, p, root, cells, type, variant, size, state, control);
            }
        }
        if (!rows) throw new Error('no rows to render for types=' + p.types.join(',') + ' variants=' + p.variants.join(','));
        return title;
    }

    // Rule 12: the panel rectangle plus scrollY; rows (the title and every row) in document pixels, first top = panel top, last bottom = panel bottom.
    hdLab.layout = () => {
        if (!ctx) throw new Error('the lab has not rendered');
        const sy = window.scrollY, r = ctx.root.getBoundingClientRect();
        const rows = [ctx.title].concat(Array.from(ctx.root.querySelectorAll('.hd-lab-row'))).map(n => { const b = n.getBoundingClientRect(); return { top: b.top + sy, bottom: b.bottom + sy }; });
        rows[0].top = r.top + sy; rows[rows.length - 1].bottom = r.bottom + sy;
        return { left: r.left, top: r.top + sy, width: r.width, height: r.height, scrollY: sy, rows };
    };
    // Rule 13.
    hdLab.probes = async () => {
        if (!ctx) throw new Error('the lab has not rendered');
        const p = ctx.params;
        return Object.assign(await hdLabProbes.run({ root: ctx.root, palette: p.palette, motion: p.motion, entries: hdLab.instances, states: p.states, graphics: p.graphics }), { extra: hdLab.extra });
    };
    // Rule 15.
    hdLab.seek = async ms => { await frames(1); for (const a of document.getAnimations()) { a.pause(); a.currentTime = ms; } };

    // Rule 14: a scenario name wins over a state of the same name. A scenario applies from, waits two frames with motion off, then applies to;
    // a plain state name applies to every instance.
    hdLab.applyState = async function (name) {
        const targets = hdLab.instances.filter(i => i.entry && i.entry.scenarios && i.entry.scenarios[name]);
        if (!targets.length) {
            if (LAB_STATES.includes(name)) { for (const i of hdLab.instances) i.apply(name); return hdLab.instances.length; }
            throw new Error('unknown state or scenario ' + name);
        }
        const root = document.querySelector('.hd-lab-panel'), motion = root.getAttribute('data-hd-motion');
        root.setAttribute('data-hd-motion', 'none'); for (const i of targets) i.apply(i.entry.scenarios[name].from);
        await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
        root.setAttribute('data-hd-motion', motion); for (const i of targets) i.apply(i.entry.scenarios[name].to);
        return targets.length;
    };

    async function main() {
        const E = window.HomeDashEngine;
        if (!E || !E.widgets || !E.prim) throw new Error('the HomeDash engine did not load');
        const p = parseParams();
        document.documentElement.lang = p.lang;
        const bundle = document.querySelector('link[href="/css/homedash.bundle.css"]');
        if (p.palette === 'theme') {
            const scope = document.getElementById('hd-lab-scope');
            scope.classList.add('desktop-body'); scope.setAttribute('data-theme', p.theme); scope.setAttribute('data-scheme', p.scheme); scope.setAttribute('data-accent', 'teal');
            scope.setAttribute('data-animations', p.motion === 'full' ? 'true' : 'false');
            // The desktop shell writes data-fruity-mode on the body for every theme and the fruity theme CSS reads it.
            scope.setAttribute('data-fruity-mode', p.scheme);
            await loadLink('/css/desktop-shell.bundle.css', bundle);
        }
        for (const src of p.scripts) await loadScript(src);
        for (const type of p.types) if (!(has(PRIM_VARIANTS, type) && typeof E.prim[type] === 'function') && !E.widgets.get(type)) fail('types', type);
        await document.fonts.ready;
        const root = buildPanel(E, p);
        ctx = { root, params: p, title: null };
        ctx.title = render(E, p, root);
        await frames(2);
        hdLab.ready = true;
    }
    main().catch(err => { hdLab.error = String((err && err.message) || err); });
})();
