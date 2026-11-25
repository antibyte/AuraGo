import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
export const tests = [['catalog: sizes, constraints and option defaults come from the golden catalog; the registry matches it', () => {
  const { E } = loadEngine(ENGINE_ORDER.concat(['hd-catalog.js'])), raw = JSON.parse(read('internal/homedash/testdata/widget_catalog.json')), c = E.catalog.create(raw);
  assert.equal(c.hash, raw.hash); eq(c.defaultSize({ type: 'gauge', variant: 'arc' }), { w: 2, h: 2 }); eq(c.defaultSize({ type: 'sensor', variant: 'sparkline' }), { w: 2, h: 2 });
  eq(c.defaultSize({ type: 'nope' }), { w: 2, h: 1 }); eq(c.constraints({ type: 'sensor', variant: 'sparkline' }), { minW: 2, minH: 1, maxW: 6, maxH: 4 });
  eq(c.options({ type: 'gauge', options: { warn: 80 } }), { min: 0, max: 100, decimals: 0, warn: 80 }); eq(c.options({ type: 'entity' }), {});
  eq(c.registryMismatches(E.widgets), []); }]];
