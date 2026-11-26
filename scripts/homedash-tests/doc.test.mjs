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
  ['normalise (M4): constraints clamp lg, kiosk default and repair, idempotent', () => { const E = boot();
    const raw = { pages: [{ id: 'a', widgets: [{ id: 'w1', type: 'sensor' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 6, h: 1 }] } }, { id: 'b' }] };
    const n = E.doc.normalise(raw, { constraints: () => ({ minW: 1, maxW: 4, minH: 1, maxH: 24 }) }); assert.equal(n.pages[0].layouts.lg[0].w, 4);
    eq(n.kiosk, { start_page: 'a', idle_dim_seconds: 120, idle_dim_level: 0.3, page_rotation_seconds: 0, screen_saver: 'clock', show_clock: true });
    assert.equal(E.doc.normalise(Object.assign({}, raw, { kiosk: { start_page: 'gone', idle_dim_seconds: 5 } })).kiosk.start_page, 'a');
    assert.equal(E.doc.normalise(Object.assign({}, raw, { kiosk: { start_page: 'b' } })).kiosk.start_page, 'b');
    const again = E.doc.normalise(n, { constraints: () => ({ minW: 1, maxW: 4, minH: 1, maxH: 24 }) }); eq(again, n);
    const wide = E.doc.normalise({ pages: [{ id: 'a', widgets: [{ id: 'w1', type: 't' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 4, h: 1 }], md: null, sm: [] } }] }, { constraints: () => ({ minW: 3 }) });
    assert.equal(wide.pages[0].layouts.sm[0].w, 3); }],
  ['drafts and forSave (M4): drafts leave the body with their layout items, palette fields follow the mode', () => { const E = boot();
    const cat = { widget: type => (type === 'switch' ? { bindings: [{ name: 'primary', required: true }], options: [] } : type === 'gauge' ? { bindings: [], options: [{ key: 'unit', required: true }] } : { bindings: [], options: [] }) };
    const D = E.doc.normalise({ look: { palette: { mode: 'dark', custom: { accent: '#fff' }, snapshot: { ink: '#111111' } } }, pages: [{ id: 'main', widgets: [{ id: 'w1', type: 'sensor' }, { id: 'w2', type: 'switch' }, { id: 'w3', type: 'gauge', options: { unit: '' } }, { id: 'w4', type: 'switch', bindings: { primary: 'ha:light.x' } }],
      layouts: { lg: [], md: [{ id: 'w2', x: 0, y: 0, w: 2, h: 1 }], sm: null } }] });
    eq([...E.doc.drafts(D, cat)].sort(), ['w2', 'w3']);
    const S = E.doc.forSave(D, { catalog: cat }); eq(S.pages[0].widgets.map(w => w.id), ['w1', 'w4']); eq(S.pages[0].layouts.lg.map(i => i.id).sort(), ['w1', 'w4']); eq(S.pages[0].layouts.md.map(i => i.id).sort(), ['w1', 'w4']);
    assert.equal(S.pages[0].layouts.sm, null); assert.equal(S.look.palette.custom, undefined); assert.equal(S.look.palette.snapshot, undefined); assert.ok(D.pages[0].widgets.length === 4, 'input untouched');
    const T = E.doc.normalise({ look: { palette: { mode: 'theme', snapshot: { ink: '#111111' }, custom: { accent: '#fff' } } } });
    eq(E.doc.forSave(T, { snapshot: { ink: '#222222' } }).look.palette, { mode: 'theme', snapshot: { ink: '#222222' } }); eq(E.doc.forSave(T).look.palette, { mode: 'theme', snapshot: { ink: '#111111' } });
    const C = E.doc.normalise({ look: { palette: { mode: 'custom', custom: { accent: '#fff' }, snapshot: { ink: '#111111' } } } }); eq(E.doc.forSave(C, { snapshot: { a: '#000000' } }).look.palette, { mode: 'custom', custom: { accent: '#fff' } });
    assert.equal(E.doc.forSave(E.doc.normalise({})).pages.length, 1); }],
  ['history: undo, redo, redo cleared, limit, coalescing, seal', () => { const E = boot(), D = base(E), N = ['pages', 'main', 'name']; const h = E.history.create();
    const c1 = E.doc.setField(D, N, 'one', { coalesce: 'k', window: 800, at: 1000 }); h.push(c1); const D1 = E.doc.apply(D, c1.ops);
    const c2 = E.doc.setField(D1, N, 'two', { coalesce: 'k', window: 800, at: 1500 }); h.push(c2); const D2 = E.doc.apply(D1, c2.ops);
    assert.equal(h.size(), 1); eq([h.peek().ops[0].before, h.peek().ops[0].after], ['', 'two']);
    h.push(E.doc.setField(D2, N, 'three', { coalesce: 'k', window: 800, at: 2400 })); assert.equal(h.size(), 2);
    h.seal(); h.push(E.doc.setField(D2, N, 'four', { coalesce: 'k', window: 800, at: 2500 })); assert.equal(h.size(), 3);
    const u = h.undo(D2); assert.ok(u && h.canRedo()); h.push(E.doc.setField(D2, N, 'x', { at: 9000 })); assert.equal(h.canRedo(), false);
    const g = E.history.create(); for (let i = 0; i < 105; i++) g.push(E.doc.setField(D, N, 'n' + i, { at: i })); assert.equal(g.size(), 100);
    const k = E.history.create(); k.push(c1); assert.equal(k.undo(D1).doc.pages[0].name, ''); assert.equal(k.redo(D).doc.pages[0].name, 'one'); }],
  ['widget defaults are never shared between widgets', () => { const E = boot();
    const a = E.doc.normalise({ pages: [{ widgets: [{ id: 'w', type: 't' }] }] }); a.pages[0].widgets[0].options.x = 1; a.pages[0].widgets[0].bindings.y = 2; a.pages[0].widgets[0].actions.z = 3;
    const b = E.doc.normalise({ pages: [{ widgets: [{ id: 'w', type: 't' }] }] }); eq([b.pages[0].widgets[0].options, b.pages[0].widgets[0].bindings, b.pages[0].widgets[0].actions], [{}, {}, {}]);
    const D = base(E), c1 = E.doc.addWidget(D, 'main', { id: 'n1', type: 'sensor' }, { w: 2, h: 1 }); c1.ops[0].after.options.x = 1;
    const c2 = E.doc.addWidget(D, 'main', { id: 'n2', type: 'sensor' }, { w: 2, h: 1 }); eq(c2.ops[0].after.options, {});
    const A = E.doc.apply(D, c2.ops); A.pages[0].widgets.find(w => w.id === 'n2').options.q = 1;
    const c3 = E.doc.addWidget(D, 'main', { id: 'n3', type: 'sensor' }, { w: 2, h: 1 }); eq(E.doc.apply(D, c3.ops).pages[0].widgets.find(w => w.id === 'n3').options, {}); }],
  ['manual md/sm overrides follow page.compact', () => { const E = boot();
    const mk = compact => { const page = { id: 'main', widgets: [{ id: 'w1', type: 't' }, { id: 'w2', type: 't' }], layouts: { lg: [{ id: 'w1', x: 0, y: 0, w: 2, h: 1 }, { id: 'w2', x: 2, y: 0, w: 2, h: 1 }], md: [{ id: 'w1', x: 0, y: 0, w: 4, h: 1 }, { id: 'w2', x: 0, y: 3, w: 4, h: 1 }] } };
      if (compact !== undefined) page.compact = compact; return E.doc.normalise({ pages: [page] }).pages[0].layouts.md.find(i => i.id === 'w2').y; };
    assert.equal(mk(false), 3); assert.equal(mk(true), 1); assert.equal(mk(undefined), 1); }],
  ['addWidget rejects a duplicate id', () => { const E = boot(), D = base(E);
    assert.throws(() => E.doc.addWidget(D, 'main', { id: 'w1', type: 'sensor' }, { w: 2, h: 1 }), /duplicate widget id w1/);
    const c = E.doc.addWidget(D, 'main', { id: 'fresh', type: 'sensor' }, { w: 2, h: 1 }); assert.equal(E.doc.apply(D, c.ops).pages[0].widgets.length, 3);
    const r = E.doc.addWidget(D, 'main', { type: 'sensor' }, { w: 2, h: 1 }); assert.equal(typeof r.meta.ids[0], 'string'); }]
];
