import assert from 'node:assert/strict'; import { loadEngine, eq } from './harness.mjs';
const boot = () => loadEngine(['hd-core.js', 'hd-grid.js', 'hd-layout.js', 'hd-doc.js', 'hd-history.js']).E;
const freeze = o => { if (o && typeof o === 'object') { Object.values(o).forEach(freeze); Object.freeze(o); } return o; };
const base = E => E.doc.normalise({ pages: [{ id: 'main', widgets: [{ id: 'w1', type: 'sensor', options: { unit: 'C' } }, { id: 'w2', type: 'switch' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 2, h: 1 }, { id: 'w2', x: 2, y: 0, w: 2, h: 1 }] } }] });
const P = ['pages', 'main', 'widgets', 'w1', 'options', 'unit'];
export const tests = [
  ['normalise fills defaults and repairs layouts without mutating input', () => { const E = boot(); const d = E.doc.normalise({});
    eq([d.schema_version, d.look.family, d.look.palette.mode, d.look.density, d.look.radius_scale], [1, 'glass', 'theme', 'comfortable', 1]);
    eq(d.grid, { row_unit: 'auto', gap: null, fit: 'scroll' }); eq([d.pages[0].id, d.pages[0].compact, d.pages[0].widgets, d.pages[0].layouts], ['main', true, [], { lg: [], md: null, sm: null }]);
    const raw = freeze({ look: { family: 'neon', radius_scale: 9 }, pages: [{ id: 'main', widgets: [{ id: 'w1', type: 'a' }, { id: 'w2', type: 'b' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 2, h: 2 }, { id: 'ghost', x: 0, y: 0, w: 1, h: 1 }] } }, { id: 'main' }] });
    const n = E.doc.normalise(raw); eq([n.look.family, n.look.radius_scale], ['glass', 1.5]);
    eq(n.pages[0].layouts.lg, [{ id: 'w1', x: 0, y: 0, w: 2, h: 2 }, { id: 'w2', x: 2, y: 0, w: 2, h: 1 }]); eq(n.pages.map(p => p.id), ['main', 'main_']); }],
  ['id-keyed set: apply, before, invert, survives reorder, missing path throws', () => { const E = boot(), D = base(E); const c = E.doc.setField(D, P, 'F');
    const A = E.doc.apply(D, c.ops); assert.equal(E.doc.getAt(A, P), 'F'); assert.equal(E.doc.getAt(D, P), 'C');
    assert.equal(E.doc.getAt(E.doc.apply(A, c.ops, 'before'), P), 'C'); assert.equal(E.doc.getAt(E.doc.apply(A, E.doc.invert(c.ops)), P), 'C');
    const R = JSON.parse(JSON.stringify(A)); R.pages[0].widgets.reverse(); assert.equal(E.doc.getAt(E.doc.apply(R, c.ops, 'before'), P), 'C');
    assert.throws(() => E.doc.apply(D, E.doc.setField(D, ['pages', 'nope', 'name'], 'x').ops), /path not found/); }],
  ['add and remove widgets keep layouts and order through undo', () => { const E = boot(), D = base(E);
    const add = E.doc.addWidget(D, 'main', { id: 'w3', type: 'light' }, { w: 4, h: 2 }); const A = E.doc.apply(D, add.ops);
    eq(A.pages[0].layouts.lg.find(i => i.id === 'w3'), { id: 'w3', x: 4, y: 0, w: 4, h: 2 }); eq(E.doc.apply(A, add.ops, 'before').pages[0].widgets.map(w => w.id), ['w1', 'w2']);
    const rem = E.doc.removeWidgets(D, 'main', ['w1', 'w2']); const X = E.doc.apply(D, rem.ops); eq([X.pages[0].widgets, X.pages[0].layouts.lg], [[], []]);
    const U = E.doc.apply(X, rem.ops, 'before'); eq(U.pages[0].widgets.map(w => w.id), ['w1', 'w2']); eq(U.pages[0].layouts.lg, D.pages[0].layouts.lg);
    const md = [{ id: 'w1', x: 0, y: 0, w: 4, h: 1 }]; eq(E.doc.apply(D, E.doc.setLayout(D, 'main', 'md', md).ops).pages[0].layouts.md, md); }],
  ['clipboard round trip with new ids', () => { const E = boot(), D = base(E); const items = E.doc.fromClipboard(E.doc.toClipboard(D, 'main', ['w1']));
    assert.equal(items.length, 1); assert.equal(items[0].widget.type, 'sensor'); assert.match(items[0].widget.id, /^w_/); eq([items[0].w, items[0].h], [2, 1]);
    eq(E.doc.fromClipboard('nope'), []); eq(E.doc.fromClipboard('{"v":2}'), []); }],
  ['history: undo, redo, redo cleared, limit, coalescing, seal', () => { const E = boot(), D = base(E), N = ['pages', 'main', 'name']; const h = E.history.create();
    const c1 = E.doc.setField(D, N, 'one', { coalesce: 'k', window: 800, at: 1000 }); h.push(c1); const D1 = E.doc.apply(D, c1.ops);
    const c2 = E.doc.setField(D1, N, 'two', { coalesce: 'k', window: 800, at: 1500 }); h.push(c2); const D2 = E.doc.apply(D1, c2.ops);
    assert.equal(h.size(), 1); eq([h.peek().ops[0].before, h.peek().ops[0].after], ['', 'two']);
    h.push(E.doc.setField(D2, N, 'three', { coalesce: 'k', window: 800, at: 2400 })); assert.equal(h.size(), 2);
    h.seal(); h.push(E.doc.setField(D2, N, 'four', { coalesce: 'k', window: 800, at: 2500 })); assert.equal(h.size(), 3);
    const u = h.undo(D2); assert.ok(u && h.canRedo()); h.push(E.doc.setField(D2, N, 'x', { at: 9000 })); assert.equal(h.canRedo(), false);
    const g = E.history.create(); for (let i = 0; i < 105; i++) g.push(E.doc.setField(D, N, 'n' + i, { at: i })); assert.equal(g.size(), 100);
    const k = E.history.create(); k.push(c1); assert.equal(k.undo(D1).doc.pages[0].name, ''); assert.equal(k.redo(D).doc.pages[0].name, 'one'); }]
];
