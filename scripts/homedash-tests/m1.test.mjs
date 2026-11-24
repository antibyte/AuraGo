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
