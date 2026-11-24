import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const Lx = () => loadEngine(['hd-core.js', 'hd-grid.js', 'hd-layout.js']).E.layout;
const it = (id, x, y, w, h) => ({ id, x, y, w, h }); const map = L => Object.fromEntries(L.map(i => [i.id, [i.x, i.y, i.w, i.h]]));
const LG = [it('a',0,0,6,2), it('b',6,0,6,2), it('c',0,2,3,1)];
export const tests = [
  ['derive md/sm in reading order with the phone full-width rule and constraints', () => { const l = Lx();
    eq(map(l.derive(LG, 8)), { a: [0,0,4,2], b: [4,0,4,2], c: [0,2,2,1] }); eq(map(l.derive(LG, 4)), { a: [0,0,4,2], b: [0,2,4,2], c: [0,4,1,1] });
    eq(map(l.derive(LG, 4, { c: { minW: 2 } })).c, [0,4,2,1]); }],
  ['effective uses overrides and memoises on the lg identity', () => { const l = Lx(); const lay = { lg: LG, md: null, sm: [it('a',0,0,1,1)] };
    assert.equal(l.effective(lay, 'md'), l.effective(lay, 'md')); assert.equal(l.effective(lay, 'sm'), lay.sm); assert.equal(l.effective(lay, 'lg'), LG); }],
  ['breakpoint with 16 px hysteresis', () => { const b = Lx().breakpoint;
    assert.equal(b(950, 'lg'), 'lg'); assert.equal(b(943, 'lg'), 'md'); assert.equal(b(959, 'md'), 'md'); assert.equal(b(960, 'md'), 'lg');
    assert.equal(b(590, 'md'), 'md'); assert.equal(b(583, 'md'), 'sm'); assert.equal(b(500, 'lg'), 'sm'); assert.equal(b(1200), 'lg'); }],
  ['metrics, fit fallback, css vars, gaps, rows', () => { const l = Lx(); const r = v => Math.round(v * 1000) / 1000;
    const s = l.metrics(1180, 780, 12, 6, 12, 'auto', 'scroll'); eq([s.mode, r(s.colW), r(s.rowH), s.offX], ['scroll', 85.333, 85.333, 0]);
    assert.equal(l.metrics(1180, 780, 12, 6, 12, 100, 'scroll').rowH, 100);
    const f = l.metrics(1180, 780, 12, 6, 12, 'auto', 'fit_screen'); eq([f.mode, r(f.colW), r(f.offX)], ['fit', 85.333, 12]);
    assert.equal(l.metrics(1180, 300, 12, 10, 12, 'auto', 'fit_screen').mode, 'overflow'); assert.equal(l.metrics(400, 780, 12, 6, 12, 'auto', 'scroll').rowH, 56);
    eq(l.cssVars(s, 12, 6, 12), { '--hd-cols': 12, '--hd-rows': 6, '--hd-colw': '85.33px', '--hd-rowh': '85.33px', '--hd-gap': '12px', '--hd-offx': '0px' });
    assert.equal(l.gapFor('compact'), 8); assert.equal(l.gapFor('spacious', null), 16); assert.equal(l.gapFor('comfortable', 20), 20); assert.equal(l.rows([it('a',0,0,1,3), it('b',0,5,1,1)]), 6); }],
  ['override maintenance on add, delete, materialise, reset', () => { const l = Lx();
    const lay = { lg: [it('a',0,0,12,1)], md: null, sm: [it('a',0,0,4,1)] }; const added = l.addToLayouts(lay, { id: 'n', w: 4, h: 2 });
    eq(map(added.lg).n, [0,1,4,2]); assert.equal(added.md, null); eq(map(added.sm).n, [0,1,1,2]);
    const removed = l.removeFromLayouts(added, ['n']); eq(map(removed.lg), { a: [0,0,12,1] }); eq(map(removed.sm), { a: [0,0,4,1] });
    const mat = l.materialiseOverride({ lg: LG, md: null, sm: null }, 'md'); eq(map(mat.md), map(l.derive(LG, 8))); assert.equal(l.resetOverride(mat, 'md').md, null); }]
];
