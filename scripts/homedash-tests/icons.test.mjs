import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs';
const NEW = ['memory', 'download', 'upload', 'timer', 'outlet', 'scene', 'script', 'button', 'list', 'number', 'brightness', 'color-temp', 'color', 'effect', 'chevron-down', 'close', 'home'];
const CONTRACT = ['cpu', 'memory', 'disk', 'download', 'upload', 'timer', 'bulb', 'power', 'outlet', 'scene', 'script', 'button', 'list', 'number', 'thermometer', 'droplet', 'gauge', 'lock', 'brightness', 'color-temp', 'color', 'effect'];
export const tests = [['icons: every contract name and the M3 UI glyphs are drawn on the 24 grid', () => {
  const { E } = loadEngine(['hd-core.js', 'hd-icons.js'], { dom: true });
  for (const n of CONTRACT.concat(NEW)) { assert.ok(E.icons.has(n), n); const svg = E.icons.icon(n); assert.equal(svg.getAttribute('data-hd-icon'), n); assert.ok(svg.querySelectorAll('path').length > 0, n); } }]];
