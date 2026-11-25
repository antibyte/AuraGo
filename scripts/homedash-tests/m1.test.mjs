// HomeDash M1: hudFor contrast, the M1 primitive additions (Task 1 table) and the token freeze.
import assert from 'node:assert/strict'; import fs from 'node:fs'; import path from 'node:path';
import { loadEngine, root, ENGINE_ORDER } from './harness.mjs';
import { current, FREEZE_PATH } from '../homedash-freeze-tokens.mjs';
const PRIMS = ENGINE_ORDER.slice(0, ENGINE_ORDER.indexOf('hd-prim-controls.js') + 1);
const dash = (el) => el.style.getPropertyValue('stroke-dasharray');
export const tests = [
  ['hudFor reaches 4.5:1 on the Instrument panel and turns monochrome accents into white phosphor', () => {
    const { E } = loadEngine(['hd-core.js', 'hd-colour.js']);
    assert.equal(E.colour.hudFor('#161616'), '#e8edf2'); assert.equal(E.colour.hudFor('#ededed'), '#e8edf2');
    for (const hex of ['#ff2bd6', '#1d5faf', '#b5145f', '#c2186b', '#6688bb', '#f3e600', '#5ce1ff', '#e95420', '#0a84ff']) {
      const hud = E.colour.hudFor(hex); assert.match(hud, /^#[0-9a-f]{6}$/); assert.ok(E.colour.contrast(hud, '#090c10') >= 4.5, hex + ' -> ' + hud);
    } }],
  ['applyLook resolves oklch, color() and invalid accents through a canvas and clears a stale --hd-hud', () => {
    const { E, document } = loadEngine(ENGINE_ORDER.slice(0, ENGINE_ORDER.indexOf('hd-widgets.js') + 1), { dom: true });
    const rootEl = document.createElement('div'), orig = document.createElement.bind(document);
    let accent = 'rgb(29, 95, 175)', canvasColour = [0, 0, 0, 0];
    const g = { clearRect() {}, fillRect() {}, set fillStyle(v) { if (v !== 'rgba(0, 0, 0, 0)' && v !== accent) throw new Error('unexpected fill ' + v); }, getImageData: () => ({ data: canvasColour }) };
    document.createElement = tag => (tag === 'canvas' ? { width: 0, height: 0, getContext: () => g } : orig(tag));
    document.defaultView = { getComputedStyle: node => (node === rootEl ? { getPropertyValue: () => '8px' } : { color: accent }) };
    const hud = () => rootEl.style.getPropertyValue('--hd-hud'), expected = rgb => E.colour.hudFor(E.colour.rgbToHex(rgb));
    E.widgets.applyLook(rootEl); assert.equal(hud(), expected([29, 95, 175]));
    accent = 'oklch(0.62 0.19 255)'; canvasColour = [51, 112, 233, 255]; E.widgets.applyLook(rootEl); assert.equal(hud(), expected([51, 112, 233]));
    accent = 'color(srgb 0.9 0.2 0.1)'; canvasColour = [230, 51, 26, 255]; E.widgets.applyLook(rootEl); assert.equal(hud(), expected([230, 51, 26]));
    accent = 'not-a-colour'; canvasColour = [0, 0, 0, 0]; E.widgets.applyLook(rootEl); assert.equal(hud(), ''); }],
  ['climate keeps its last range while the entity is dead and rebuilds only on a live range change', () => {
    const { E, document } = loadEngine(ENGINE_ORDER, { dom: true });
    const view = p => Object.assign({ id: 'c:1', kind: 'climate', name: 'Living room', value: 'heat', attributes: {}, available: true, status: 'ok', readonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, p);
    const state = { view: view({ attributes: { min_temp: 10, max_temp: 25, temperature: 20, current_temperature: 19 } }) };
    const api = () => ({ entity: () => state.view, can: () => true, command: () => Promise.resolve({}), moreInfo() {}, history: () => Promise.resolve({ points: [], source: 'ring' }), announce() {} });
    const h = E.widgets.host({ widget: { id: 'w', type: 'climate', variant: 'dial', bindings: { primary: 'c:1' } }, size: { w: 2, h: 2 }, t: k => k, api });
    document.body.appendChild(h.el); const dial = () => h.el.querySelector('.hd-dial'), first = dial();
    state.view = view({ available: false, status: 'unavailable' }); h.update(); assert.equal(dial(), first, 'a dead entity must not rebuild the knob');
    state.view = view({ value: 'unknown', status: 'unknown', attributes: { min_temp: 7, max_temp: 35 } }); h.update(); assert.equal(dial(), first, 'unknown must not rebuild either');
    state.view = view({ attributes: { min_temp: 10, max_temp: 25, temperature: 20 } }); h.update(); assert.equal(dial(), first, 'the same live range keeps the knob');
    state.view = view({ attributes: { min_temp: 5, max_temp: 30, temperature: 20 } }); h.update(); assert.notEqual(dial(), first, 'a live range change rebuilds it'); }],
  ['the Tactile sensor module draws the exact inset window (2x1 is 3 px shorter, 4 px above the bottom)', () => {
    const { E, document } = loadEngine(ENGINE_ORDER, { dom: true });
    const view = { id: 's:1', kind: 'sensor', name: 'Power', value: 42, unit: 'W', attributes: {}, available: true, status: 'ok', readonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null };
    const api = () => ({ entity: () => view, can: () => true, command: () => Promise.resolve({}), moreInfo() {}, history: () => Promise.resolve({ points: [[1, 3], [2, 5], [3, 4]], source: 'ring' }), announce() {} });
    const make = family => E.widgets.host({ widget: { id: 'w', type: 'sensor', variant: 'sparkline', family, bindings: { primary: 's:1' } }, size: { w: 2, h: 1 }, t: k => k, api });
    const viewBox = (h, px) => { document.body.appendChild(h.el); h.resize({ w: 2, h: 1, px }); return h.el.querySelector('.hd-spark').getAttribute('viewBox'); };
    assert.equal(viewBox(make('tactile'), { width: 300, height: 120 }), '0 0 282 35');   // 300 - 2 - 16 wide, round(120 * 0.38) - 11 high (aspect 2.5 >= 1.6)
    assert.equal(viewBox(make('tactile'), { width: 300, height: 300 }), '0 0 282 106');  // 2x2: round(300 * 0.38) - 8
    assert.equal(viewBox(make('glass'), { width: 300, height: 120 }), '0 0 298 46'); }],
  ['M1 additions follow the Task 1 table and leave C2 defaults unchanged', () => {
    const { E } = loadEngine(PRIMS, { dom: true });
    const plain = E.prim.knob({ label: 'k' }); assert.equal(plain.el.querySelector('.hd-dial-glow'), null); assert.equal(plain.el.querySelector('.hd-dial-dots'), null);
    const t = E.prim.toggle({ label: 'x', decor: true }); t.set({ on: true });
    assert.equal(t.el.querySelector('.hd-toggle-key').textContent, 'I'); assert.equal(t.el.querySelectorAll('.hd-toggle-legend').length, 2); assert.ok(t.el.querySelector('.hd-toggle-led'));
    const k = E.prim.knob({ label: 'd', glow: true, word: true, dots: { count: 21, r: 94, size: 4.5, lit: true }, hardware: true, handleShadow: true });
    k.set({ value: 64 }); assert.equal(dash(k.el.querySelector('.hd-dial-glow')), dash(k.el.querySelector('.hd-dial-arc')));
    assert.equal((k.el.querySelector('.hd-dial-dots-lit').getAttribute('d').match(/M/g) || []).length, 13); assert.ok(k.el.querySelector('.hd-dial-knurl'));
    k.set({ value: 0, word: 'Off' }); assert.equal(k.el.querySelector('.hd-dial-word').textContent, 'Off'); assert.equal(k.el.getAttribute('data-hd-word'), 'true');
    const q = E.prim.knob({ label: 'q', quantize: 40, lead: true, gaps: 40 }); q.set({ value: 50 }); assert.ok(q.el.querySelector('.hd-dial-lead').getAttribute('d')); assert.ok(q.el.querySelector('.hd-dial-gaps'));
    const th = E.prim.knob({ label: 'T', min: 5, max: 30, decimals: 1, activeSegment: true, needle: true, extra: true, subIcon: 'thermometer', ticksMajor: 5, numerals: 6 });
    th.set({ value: 21.5, current: 19.5, action: 'Heating to', sub: 19.5 }); assert.equal(th.el.querySelector('.hd-dial-action').textContent, 'Heating to');
    assert.ok(th.el.querySelector('.hd-dial-current .hd-dial-needle')); assert.equal(th.el.querySelectorAll('.hd-dial-numeral').length, 6);
    const s = E.prim.segmented({ label: 'm', options: [{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }], rotary: true }); assert.ok(s.el.querySelector('.hd-seg-rotary .hd-seg-rotary-pointer'));
    const n = E.prim.needleGauge({ label: 'n', min: 0, max: 100, face: true, needleFx: true }); n.set({ value: 40 });
    assert.equal(n.el.querySelector('.hd-gauge-needle-shadow path').getAttribute('d'), n.el.querySelector('.hd-gauge-needle-shape').getAttribute('d')); assert.ok(n.el.querySelector('.hd-gauge-face .hd-gauge-bezel'));
    const a = E.prim.arcGauge({ label: 'a', min: 0, max: 100, zoneOverlay: true, glow: true, readout: 'seg7' }); a.set({ value: 64 });
    assert.equal(dash(a.el.querySelector('.hd-gauge-arc-zone')), dash(a.el.querySelector('.hd-gauge-arc'))); assert.ok(a.el.querySelector('.hd-gauge-readout .hd-seg7'));
    const nu = E.prim.numeral({ chars: 4, degree: true, unit: 'C', trendIcon: true }); nu.set({ value: 21.4, decimals: 1, trend: 'down' });
    assert.ok(nu.el.querySelector('.hd-num-trend svg')); assert.equal(nu.el.querySelector('.hd-num-trend').textContent, '');
    const sp = E.prim.sparkline({ label: 's', fade: true, grid: [4, 3], halo: true }); sp.set({ points: [1, 2, 3, 2], w: 120, h: 40, summary: 'x' });
    assert.ok(sp.fill && sp.fill.querySelector('.hd-spark-area')); assert.equal(sp.el.querySelector('.hd-spark-area'), null);
    assert.equal((sp.el.querySelector('.hd-spark-grid').getAttribute('d').match(/M/g) || []).length, 5);
    assert.equal(sp.el.querySelector('.hd-spark-halo').getAttribute('cx'), sp.el.querySelector('.hd-spark-last').getAttribute('cx')); }],
  ['the token rules equal the frozen set once Task 22 has written it', () => {
    const file = path.join(root, FREEZE_PATH); if (!fs.existsSync(file)) return;
    const frozen = JSON.parse(fs.readFileSync(file, 'utf8')), now = current(), problems = [];
    for (const f of new Set([...Object.keys(frozen), ...Object.keys(now)])) {
      const a = frozen[f] || {}, b = now[f] || {};
      for (const sel of new Set([...Object.keys(a), ...Object.keys(b)])) for (const name of new Set([...Object.keys(a[sel] || {}), ...Object.keys(b[sel] || {})])) {
        const was = (a[sel] || {})[name], is = (b[sel] || {})[name]; if (was !== is) problems.push(`${f} ${sel} ${name}: frozen ${was ?? '(absent)'} now ${is ?? '(absent)'}`); }
    }
    assert.deepEqual(problems, [], 'token freeze violated; a change needs the lead, then node scripts/homedash-freeze-tokens.mjs'); }]
];
