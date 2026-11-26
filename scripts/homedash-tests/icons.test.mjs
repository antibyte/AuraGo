import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs';
const NEW = ['memory', 'download', 'upload', 'timer', 'outlet', 'scene', 'script', 'button', 'list', 'number', 'brightness', 'color-temp', 'color', 'effect', 'chevron-down', 'close', 'home'];
const CONTRACT = ['cpu', 'memory', 'disk', 'download', 'upload', 'timer', 'bulb', 'power', 'outlet', 'scene', 'script', 'button', 'list', 'number', 'thermometer', 'droplet', 'gauge', 'lock', 'brightness', 'color-temp', 'color', 'effect'];
export const tests = [['icons: every contract name and the M3 UI glyphs are drawn on the 24 grid', () => {
  const { E } = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true });
  for (const n of CONTRACT.concat(NEW)) { assert.ok(E.icons.has(n), n); const svg = E.icons.icon(n); assert.equal(svg.getAttribute('data-hd-icon'), n); assert.ok(svg.querySelectorAll('path').length > 0, n); } }],
  ['icons (M4 C19): every editor glyph is drawn and choices() leaves the UI glyphs out of the picker', () => {
  const { E } = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true }), C19 = 'undo redo copy cut paste edit duplicate panel-left panel-right window grip look layers align-left align-top search device-desktop device-tablet device-phone fullscreen sync cloud-off arrow-right'.split(' ');
  for (const n of C19) { assert.ok(E.icons.has(n), n); const svg = E.icons.icon(n); assert.equal(svg.getAttribute('data-hd-icon'), n); assert.ok(svg.querySelectorAll('path').length > 0, n); }
  const choices = E.icons.choices(); assert.ok(choices.includes('bulb') && choices.includes('home') && choices.includes('cpu')); for (const n of C19.concat(['more', 'check', 'close', 'plus', 'trash', 'arrow-left'])) assert.ok(!choices.includes(n), n);
  assert.equal(E.icons.icon('nope').getAttribute('data-hd-icon'), 'entity'); assert.equal(new Set(E.icons.names()).size, E.icons.names().length); }]
];
