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
];
