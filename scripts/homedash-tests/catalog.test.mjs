import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
export const tests = [['catalog: sizes, constraints and option defaults come from the golden catalog; the registry matches it', () => {
  const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), raw = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), c = E.catalog.create(raw);
  assert.equal(c.hash, raw.hash); eq(c.defaultSize({ type: 'gauge', variant: 'arc' }), { w: 2, h: 2 }); eq(c.defaultSize({ type: 'sensor', variant: 'sparkline' }), { w: 2, h: 2 });
  eq(c.defaultSize({ type: 'nope' }), { w: 2, h: 1 }); eq(c.constraints({ type: 'sensor', variant: 'sparkline' }), { minW: 2, minH: 1, maxW: 6, maxH: 4 });
  eq(c.options({ type: 'gauge', options: { warn: 80 } }), { min: 0, max: 100, decimals: 0, warn: 80 }); eq(c.options({ type: 'entity' }), {});
  eq(c.registryMismatches(E.widgets), []); }],
  ['catalog: pickWidgetFor follows the ordered defaults, skips unregistered types and falls back to entity', () => {
    const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), c = E.catalog.create(JSON.parse(read('internal/homedash/testdata/widget_catalog.json'))), R = E.widgets;
    const ent = (kind, dc, caps) => ({ kind, device_class: dc || '', capabilities: (caps || []).map(name => ({ name, allowed: true })) });
    assert.deepEqual({ ...c.pickWidgetFor(ent('light', '', ['brightness']), R) }, { type: 'light', variant: 'slider' });
    assert.deepEqual({ ...c.pickWidgetFor(ent('light'), R) }, { type: 'light', variant: 'tile' });
    assert.deepEqual({ ...c.pickWidgetFor(ent('sensor', 'temperature', ['numeric']), R) }, { type: 'temperature', variant: 'numeral' });
    assert.deepEqual({ ...c.pickWidgetFor(ent('sensor', 'battery', ['numeric']), R) }, { type: 'sensor', variant: 'numeral' }); // battery not registered: next match
    assert.deepEqual({ ...c.pickWidgetFor(ent('fan'), R) }, { type: 'switch', variant: 'tile' });                                 // fan chain
    assert.deepEqual({ ...c.pickWidgetFor(ent('cover'), R) }, { type: 'entity', variant: 'tile' });
    assert.deepEqual({ ...c.pickWidgetFor({}, R) }, { type: 'entity', variant: 'tile' });
    assert.equal(c.accepts('light', 'primary', ent('switch')), false); assert.equal(c.accepts('switch', 'primary', ent('light')), true);
    assert.equal(c.accepts('gauge', 'primary', ent('sensor')), false); assert.equal(c.accepts('gauge', 'primary', ent('sensor', '', ['numeric'])), true); }],
  ['catalog: palette = catalog ∩ registry, in category order, no hard-coded list', () => {
    const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), raw = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), c = E.catalog.create(raw);
    const groups = c.paletteGroups(E.widgets), types = groups.flatMap(g => g.types.map(x => x.type)).sort();
    eq(types, raw.widgets.map(w => w.type).filter(t => E.widgets.get(t)).sort());
    const cats = groups.map(g => g.category); eq(cats, E.catalog.CATEGORY_ORDER.filter(k => cats.includes(k)));
    assert.ok(groups.every(g => g.types.length > 0)); assert.ok(c.paletteTokens().length <= 40 && c.paletteTokens().includes('surface-solid')); }],
  ['catalog (fix): pickWidgetFor counts only allowed capabilities', () => {
    const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), c = E.catalog.create(JSON.parse(read('internal/homedash/testdata/widget_catalog.json'))), R = E.widgets;
    const light = allowed => ({ kind: 'light', device_class: '', capabilities: [{ name: 'brightness', allowed }] });
    assert.deepEqual({ ...c.pickWidgetFor(light(false), R) }, { type: 'light', variant: 'tile' });
    assert.deepEqual({ ...c.pickWidgetFor(light(true), R) }, { type: 'light', variant: 'slider' }); }]
];
