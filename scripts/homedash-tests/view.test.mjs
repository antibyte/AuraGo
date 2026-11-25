import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
const FILES = ENGINE_ORDER.concat(['hd-tone.js', 'hd-api.js', 'hd-catalog.js', 'hd-render.js', 'hd-view.js']), CATALOG = JSON.parse(read('internal/homedash/testdata/widget_catalog.json'));
const TWO = { id: 'd_aaaaaaaaaaab', name: 'Two', look: { family: 'minimal', palette: { mode: 'theme' } }, pages: [
  { id: 'main', name: 'One', widgets: [{ id: 'w1', type: 'entity', variant: 'tile', bindings: { primary: 'system:uptime' } }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 2, h: 1 }] } },
  { id: 'p2', name: 'Two', widgets: [{ id: 'w2', type: 'entity', variant: 'tile', bindings: { primary: 'system:cpu' } }], layouts: { lg: [] } }] };
const key = k => ({ type: 'keydown', key: k, preventDefault() {} });
export const tests = [
  ['view: banners pick at most two by priority', () => { const b = loadEngine(FILES).E.view.banners;
    eq(b({ mode: 'dashboard', connection: { phase: 'down', offline: true }, sources: [{ source: 'system', state: 'degraded' }], desktopReadonly: true }).map(x => x.kind), ['offline', 'source_degraded']);
    eq(b({ mode: 'dashboard', connection: { phase: 'live' }, sources: [], desktopReadonly: true, schemaTooNew: true }).map(x => x.kind), ['schema_too_new', 'readonly']);
    eq(b({ mode: 'home', connection: { phase: 'down', offline: true }, sources: [{ source: 'system', state: 'unavailable' }], desktopReadonly: true }).map(x => x.kind), []);
    eq(b({ mode: 'home', connection: { phase: 'stopped', reason: 'auth_lost' } }).map(x => x.kind), ['auth_lost']);
    assert.equal(b({ mode: 'dashboard', connection: { offline: true } })[0].action, 'retry'); }],
  ['view: root attributes follow the look and the host; tabs switch pages; connection and read-only attributes', () => {
    const { E, document } = loadEngine(FILES, { dom: true }), root = E.el('div', { class: 'hd-root' }); document.body.appendChild(root);
    const bus = E.core.emitter(), pages = [], store = { view: () => null, on: bus.on, off: bus.off, history: () => Promise.resolve({ points: [], source: 'none' }) };
    const view = E.view.create({ root, store, catalog: E.catalog.create(CATALOG), t: k => k, lang: 'en', host: 'desktop', scheme: () => 'light', motion: () => 'reduced',
      raf: fn => 1, caf: () => {}, measure: () => ({ width: 1180, height: 700 }), onPage: p => pages.push(p) });
    view.show(TWO);
    for (const [k, v] of [['data-hd-family', 'minimal'], ['data-hd-palette', 'theme'], ['data-hd-scheme', 'light'], ['data-hd-motion', 'reduced'], ['data-hd-density', 'comfortable'], ['data-hd-backdrop', 'theme'], ['data-hd-bp', 'lg']]) assert.equal(root.getAttribute(k), v, k);
    const tabs = root.querySelectorAll('.hd-tab'); assert.equal(tabs.length, 2); assert.equal(tabs[0].getAttribute('aria-selected'), 'true');
    const panel = root.querySelector('.hd-page'); assert.equal(panel.getAttribute('role'), 'tabpanel'); assert.equal(panel.getAttribute('aria-labelledby'), tabs[0].getAttribute('id'));
    root.querySelector('.hd-tabs').dispatchEvent(key('ArrowRight')); eq(pages, ['p2']); assert.equal(view.pageId(), 'p2');
    assert.ok(root.querySelector('[data-hd-wid="w2"]')); assert.equal(root.querySelector('[data-hd-wid="w1"]'), null); eq(view.entityIds(), ['system:cpu']);
    view.setConnection({ phase: 'down', offline: true }); assert.equal(root.getAttribute('data-hd-conn'), 'offline');
    view.setConnection({ phase: 'connecting', offline: false }); assert.equal(root.getAttribute('data-hd-conn'), 'connecting');
    view.setReadonly(true); assert.equal(root.getAttribute('data-hd-readonly'), 'true');
    view.show(Object.assign({}, TWO, { look: { family: 'glass', palette: { mode: 'dark' } } })); assert.equal(root.getAttribute('data-hd-scheme'), 'dark'); assert.equal(view.pageId(), 'p2');
    view.destroy(); assert.equal(root.children.length, 0); }],
  ['view: a custom palette leaves the five applyLook-owned tokens to applyLook and applies the rest', () => {
    const { E, document } = loadEngine(FILES, { dom: true }), C = E.colour, OWNED = ['--hd-accent', '--hd-light-on', '--hd-focus', '--hd-ink-on-accent', '--hd-hud'];
    const theme = { '--hd-accent': 'rgb(120, 170, 230)', '--hd-light-on': 'rgb(200, 140, 40)', '--hd-focus': 'rgb(120, 170, 230)', '--hd-surface-solid': 'rgb(255, 255, 255)', '--hd-canvas': 'rgb(240, 240, 240)', '--hd-ink-on-accent': 'rgb(255, 255, 255)' };
    const inline = (node, n) => node.style.getPropertyValue(n);
    document.defaultView = { getComputedStyle: node => {
        if (node.getAttribute && node.getAttribute('class') === 'hd-root') return { getPropertyValue: n => (n === '--vd-mat-card-radius' ? '8px' : '') };
        const name = /var\((.*)\)/.exec(node.style.getPropertyValue('color'))[1], over = inline(node.parentNode, name);
        if (over) { const rgb = C.hexToRgb(over); return { color: 'rgb(' + rgb.join(', ') + ')' }; }
        if (name === '--hd-focus' && inline(node.parentNode, '--hd-accent')) { const rgb = C.hexToRgb(inline(node.parentNode, '--hd-accent')); return { color: 'rgb(' + rgb.join(', ') + ')' }; }
        const bridged = { '--hd-accent': '--hd-n-accent', '--hd-ink-on-accent': '--hd-n-ink-on-accent' }[name], nv = bridged && inline(node.parentNode, bridged); // the bridge maps the neutral layer
        if (nv) { const rgb = C.hexToRgb(nv); return { color: 'rgb(' + rgb.join(', ') + ')' }; }
        return { color: theme[name] || 'rgb(0, 0, 0)' };
    } };
    const logWrites = node => { const log = [], sp = node.style.setProperty.bind(node.style), rp = node.style.removeProperty.bind(node.style);
        node.style.setProperty = (n, v) => { if (OWNED.includes(n)) log.push('set ' + n + '=' + v); return sp(n, v); };
        node.style.removeProperty = n => { if (OWNED.includes(n)) log.push('remove ' + n); return rp(n); };
        return log; };
    const palette = { mode: 'custom', scheme: 'dark', accent: '#F5C542', surface_hue: 250 }, solved = C.solvePalette(palette).tokens;
    const root = E.el('div', { class: 'hd-root' }), ref = E.el('div', { class: 'hd-root' }); document.body.appendChild(root); document.body.appendChild(ref);
    const bus = E.core.emitter(), store = { view: () => null, on: bus.on, off: bus.off, history: () => Promise.resolve({ points: [], source: 'none' }) };
    const view = E.view.create({ root, store, catalog: E.catalog.create(CATALOG), t: k => k, lang: 'en', host: 'desktop', scheme: () => 'light', motion: () => 'full', raf: fn => 1, caf: () => {}, measure: () => ({ width: 1180, height: 700 }) });
    const viewLog = logWrites(root), refLog = logWrites(ref);
    view.show(Object.assign({}, TWO, { look: { family: 'minimal', palette } }));
    // the reference root: the same solved tokens minus the five names applyLook owns, then applyLook alone
    const base = Object.assign({}, solved); for (const n of ['accent', 'light-on', 'focus', 'ink-on-accent', 'hud']) delete base[n];
    C.applyPalette(ref, base); E.core.setVars(ref, { '--hd-n-accent': solved.accent, '--hd-n-ink-on-accent': solved['ink-on-accent'] }); E.widgets.applyLook(ref);
    assert.ok(viewLog.some(l => l.startsWith('set --hd-hud=')), 'applyLook ran and wrote the hud'); // a solved palette passes 3:1 on its own surface, so no accent correction is expected
    eq(viewLog, refLog);
    for (const n of OWNED) assert.equal(inline(root, n), inline(ref, n), n);
    for (const n of C.PALETTE_TOKENS) if (!['accent', 'light-on', 'focus', 'ink-on-accent', 'hud'].includes(n)) assert.equal(inline(root, '--hd-' + n), solved[n], n);
    assert.equal(root.getAttribute('data-hd-scheme'), 'dark'); assert.equal(root.getAttribute('data-hd-palette'), 'custom');
    assert.equal(inline(root, '--hd-n-accent'), solved.accent); assert.equal(inline(root, '--hd-n-ink-on-accent'), solved['ink-on-accent']);
    assert.notEqual(solved.accent.toLowerCase(), '#7aa7ff', 'the custom accent differs from the neutral default');
    view.refreshLook(); for (const n of OWNED) assert.equal(inline(root, n), inline(ref, n), 'refreshLook ' + n);
    for (const mode of ['dark', 'theme']) {
      view.show(Object.assign({}, TWO, { look: { family: 'minimal', palette: { mode } } }));
      for (const n of ['--hd-ink', '--hd-n-accent', '--hd-n-ink-on-accent']) assert.equal(inline(root, n), '', mode + ' clears ' + n);
      view.show(Object.assign({}, TWO, { look: { family: 'minimal', palette } })); assert.equal(inline(root, '--hd-n-accent'), solved.accent);
    } }]
];
