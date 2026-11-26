import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const G = () => loadEngine(['hd-core.js', 'hd-grid.js']).E.grid;
const it = (id, x, y, w, h) => ({ id, x, y, w, h });
const map = L => Object.fromEntries(L.map(i => [i.id, [i.x, i.y, i.w, i.h]]));
export const tests = [
  ['collides', () => { const g = G(); assert.equal(g.collides(it('a',0,0,2,2), it('b',2,0,1,1)), false); assert.equal(g.collides(it('a',0,0,2,2), it('b',1,1,1,1)), true); assert.equal(g.collides(it('a',0,0,2,2), it('a',0,0,2,2)), false); }],
  ['compact lifts, keeps gaps without gravity, repairs overlaps', () => { const g = G(); const L = [it('a',0,0,2,2), it('b',2,0,2,1), it('c',0,5,1,1), it('d',3,4,1,1)];
    eq(map(g.compact(L)), { a: [0,0,2,2], b: [2,0,2,1], c: [0,2,1,1], d: [3,1,1,1] }); eq(map(g.compact(L, false)), map(L));
    eq(map(g.compact([it('a',0,0,2,2), it('b',1,1,2,2)])), { a: [0,0,2,2], b: [1,2,2,2] }); assert.equal(L[2].y, 5, 'input untouched'); }],
  ['pushDown cascades and terminates', () => { const g = G();
    eq(map(g.pushDown([it('a',0,1,2,1), it('b',0,1,2,1), it('c',0,2,2,1)], ['a'])), { a: [0,1,2,1], b: [0,2,2,1], c: [0,3,2,1] });
    const big = [it('m',0,0,1,1)]; for (let i = 0; i < 60; i++) big.push(it('i' + i, 0, i, 1, 1)); assert.equal(map(g.pushDown(big, ['m'])).i59[1], 60); }],
  ['moveItems: swap, edge clamp, group bbox, push without compaction', () => { const g = G(); const L = [it('a',0,0,2,2), it('b',2,0,2,2), it('c',4,0,2,2)];
    eq(map(g.moveItems(L, ['a'], { x: 2, y: 0 }, 12)), { a: [2,0,2,2], b: [0,0,2,2], c: [4,0,2,2] });
    eq(map(g.moveItems(L, ['c'], { x: 20, y: 0 }, 12)).c, [10,0,2,2]);
    eq(map(g.moveItems(L, ['a','b'], { x: 6, y: 0 }, 12)), { a: [6,0,2,2], b: [8,0,2,2], c: [4,0,2,2] });
    const P = [it('a',0,0,2,1), it('b',0,1,4,1)];
    eq(map(g.moveItems(P, ['a'], { x: 0, y: 1 }, 12, { compact: false })), { a: [0,1,2,1], b: [0,2,4,1] }); eq(map(g.moveItems(P, ['a'], { x: 0, y: 1 }, 12)), { a: [0,0,2,1], b: [0,1,4,1] }); }],
  ['resizeItem: push, min/max, opposite edge fixed, all eight edges', () => { const g = G();
    eq(map(g.resizeItem([it('a',0,0,2,2), it('b',2,0,2,2)], 'a', 'e', { dx: 1, dy: 0 }, 12)), { a: [0,0,3,2], b: [2,2,2,2] });
    eq(map(g.resizeItem([it('a',0,0,2,2)], 'a', 'w', { dx: 5, dy: 0 }, 12, { minW: 1 }, { compact: false })).a, [1,0,1,2]);
    eq(map(g.resizeItem([it('a',0,0,2,2)], 'a', 'n', { dx: 0, dy: -3 }, 12, { maxH: 3 }, { compact: false })).a, [0,0,2,2]);
    eq(map(g.resizeItem([it('a',0,0,2,2)], 'a', 'se', { dx: 1, dy: 1 }, 12, { maxW: 2 })).a, [0,0,2,3]);
    eq(map(g.resizeItem([it('a',10,0,2,1)], 'a', 'e', { dx: 3, dy: 0 }, 12)).a, [10,0,2,1]);
    const want = { n: [4,5,2,1], ne: [4,5,3,1], e: [4,4,3,2], se: [4,4,3,3], s: [4,4,2,3], sw: [5,4,1,3], w: [5,4,1,2], nw: [5,5,1,1] };
    for (const [edge, r] of Object.entries(want)) eq(map(g.resizeItem([it('z',4,4,2,2)], 'z', edge, { dx: 1, dy: 1 }, 12, {}, { compact: false })).z, r, edge); }],
  ['place: first fit, row-major', () => { const g = G();
    eq(g.place([it('a',0,0,12,1)], 2, 1, 12), { x: 0, y: 1 }); eq(g.place([it('a',0,0,4,1), it('b',6,0,6,1)], 2, 1, 12), { x: 4, y: 0 });
    eq(g.place([it('a',0,0,4,1), it('b',6,0,6,1)], 3, 1, 12), { x: 0, y: 1 }); eq(g.place([], 2, 1, 12, 3), { x: 0, y: 3 }); eq(g.place([], 20, 1, 12), { x: 0, y: 0 }); }],
  ['repair clamps, drops duplicates and unknown ids', () => { const g = G();
    eq(map(g.repair([{ id: 'a', x: 11, y: -2, w: 2, h: 1 }, it('a',0,0,1,1), it('zz',0,0,1,1), { id: 'b', x: '3', y: null, w: 20, h: 0 }], 12, { b: { minW: 2, maxW: 4, minH: 1, maxH: 2 } }, { ids: new Set(['a','b']) })), { a: [10,0,2,1], b: [3,0,4,1] }); }],
  ['fractional input never yields negative or fractional cells', () => { const g = G();
    assert.equal(g.compact([it('a',0,0.4,2,1)])[0].y, 0); assert.equal(g.compact([it('a',0,1.6,2,1)], false)[0].y, 2);
    const ok = L => L.every(i => [i.x, i.y, i.w, i.h].every(Number.isInteger) && i.y >= 0 && i.x >= 0);
    const L = [it('a',0,0,2,1), it('b',2,0,2,1), it('c',0,1,2,1)], before = JSON.stringify(L);
    assert.ok(ok(g.moveItems(L, ['a'], { x: 2.6, y: 0.4 }, 12))); assert.ok(ok(g.moveItems(L, ['a'], { x: 1.5, y: -0.4 }, 12, { compact: false })));
    assert.ok(ok(g.resizeItem(L, 'a', 'se', { dx: 0.6, dy: 1.4 }, 12))); assert.ok(ok(g.resizeItem(L, 'b', 'nw', { dx: -0.6, dy: -0.4 }, 12)));
    assert.equal(JSON.stringify(L), before); }]
  ,
  ['place: occupancy grid is identical to the old scan (randomized, edge cases)', () => { const g = G();
    const PROBE = {}, collides = (a, b) => a.id !== b.id && a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
    const oracle = (L, w, h, cols, startY) => { const ww = Math.min(Math.max(1, w), cols), hh = Math.max(1, h), from = Math.max(0, startY || 0);
      const bottom = Math.max(from, L.reduce((m, i) => Math.max(m, i.y + i.h), 0));
      for (let y = from; y <= bottom; y++) for (let x = 0; x <= cols - ww; x++) { const p = { id: PROBE, x, y, w: ww, h: hh }; if (!L.some(i => collides(i, p))) return { x, y }; }
      return { x: 0, y: bottom }; };
    let seed = 12345; const rnd = n => { seed = (seed * 1103515245 + 12345) & 0x7fffffff; return seed % n; };
    let checked = 0;
    for (let round = 0; round < 300; round++) {
      const cols = [1, 2, 4, 6, 12][rnd(5)], n = rnd(70), L = [];
      for (let i = 0; i < n; i++) { const w = 1 + rnd(Math.min(cols, 5)); L.push(it('i' + i, rnd(cols + 2 - w + 1), rnd(25), w, 1 + rnd(4))); }   // overlaps and items past the right edge are allowed
      for (let k = 0; k < 6; k++) { const w = 1 + rnd(cols + 3), h = rnd(5), sy = rnd(3) === 0 ? rnd(30) : 0; eq(g.place(L, w, h, cols, sy), oracle(L, w, h, cols, sy), 'round ' + round); checked++; }
    }
    assert.ok(checked >= 1800);
    const dense = []; for (let i = 0; i < 40; i++) dense.push(it('d' + i, (i % 6) * 2, Math.floor(i / 6) * 2, 2, 2));
    for (const sy of [0, 3, 13, 50]) for (const w of [1, 2, 5, 12, 30]) eq(g.place(dense, w, 2, 12, sy), oracle(dense, w, 2, 12, sy));
    const odd = []; for (let i = 0; i < 30; i++) odd.push(it('o' + i, i % 5 + 0.5, i, 1.5, 1));   // fractional geometry takes the scan path
    eq(g.place(odd, 2, 1, 12), oracle(odd, 2, 1, 12)); eq(g.place(odd, 2.5, 1.5, 12, 0.5), oracle(odd, 2.5, 1.5, 12, 0.5));
    const zero = []; for (let i = 0; i < 20; i++) zero.push(it('z' + i, i % 4, i, i % 3 ? 1 : 0, 1)); eq(g.place(zero, 3, 1, 12), oracle(zero, 3, 1, 12));
    const far = []; for (let i = 0; i < 20; i++) far.push(it('f' + i, 0, i === 3 ? 100000000 : i, 2, 1)); eq(g.place(far, 2, 1, 12), oracle(far, 2, 1, 12)); }],
  ['place: 400 widgets without layout entries place in well under 50 ms (best of three)', () => { const g = G(); let best = Infinity, L = [];
    for (let run = 0; run < 3; run++) { L = []; const t0 = process.hrtime.bigint(); for (let i = 0; i < 400; i++) { const p = g.place(L, 3, 2 + (i % 3), 12); L.push(it('w' + i, p.x, p.y, 3, 2 + (i % 3))); } best = Math.min(best, Number(process.hrtime.bigint() - t0) / 1e6); }
    assert.ok(best < 50, 'took ' + best + ' ms');
    for (let a = 0; a < L.length; a++) for (let b = a + 1; b < L.length; b++) assert.equal(g.collides(L[a], L[b]), false); }]
];
