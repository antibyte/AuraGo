import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, ENGINE_DIR } from './harness.mjs';
const PARTS = { 'hd-core.js': 'core', 'hd-format.js': 'format', 'hd-colour.js': 'colour', 'hd-grid.js': 'grid', 'hd-layout.js': 'layout', 'hd-doc.js': 'doc', 'hd-history.js': 'history', 'hd-icons.js': 'icons', 'hd-geometry.js': 'geometry', 'hd-input.js': 'input', 'hd-prim-displays.js': 'prim', 'hd-prim-controls.js': 'prim', 'hd-widgets.js': 'widgets', 'hd-control.js': 'control' };
const PRIMS = ['toggle', 'rocker', 'pushButton', 'slider', 'knob', 'stepper', 'segmented', 'numeral', 'segReadout', 'arcGauge', 'needleGauge', 'bar', 'lamp', 'badge', 'iconBadge', 'sparkline', 'frame', 'sheet', 'colourWheel', 'ctStrip', 'picker'];
export const tests = [['the M0 engine loads in order without a DOM and publishes every part', () => {
  const { E } = loadEngine(ENGINE_ORDER); for (const [f, part] of Object.entries(PARTS)) assert.ok(E[part], f + ' publishes E.' + part);
  for (const f of ENGINE_ORDER) assert.ok(read(`${ENGINE_DIR}/${f}`).split('\n').length < 1100, f);
  assert.deepEqual(Object.keys(E.prim).filter(k => k !== 'util').sort(), PRIMS.slice().sort()); assert.ok(E.widgets.get('entity')); }]];
