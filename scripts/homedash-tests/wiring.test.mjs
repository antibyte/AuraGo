import assert from 'node:assert/strict'; import { read, loadEngine, APP_ORDER } from './harness.mjs';
export const tests = [['wiring: the desktop loader runs the engine in APP_ORDER, then the two host files; the whole engine loads without a DOM', () => {
  const m = /homedash: \{\s*styles: appStyles\('\/css\/homedash\.bundle\.css', '\/css\/desktop-app-homedash\.css'\),\s*scripts: \[([^\]]*)\]\s*\}/.exec(read('ui/js/desktop/core/module-loader.js'));
  assert.ok(m, 'module-loader entry'); assert.deepEqual([...m[1].matchAll(/'([^']+)'/g)].map(x => x[1]), APP_ORDER.map(f => '/js/homedash/' + f).concat(['/js/desktop/apps/homedash-home.js', '/js/desktop/apps/homedash.js']));
  const { E } = loadEngine(APP_ORDER); for (const part of ['tone', 'api', 'stream', 'store', 'catalog', 'render', 'sheet', 'view']) assert.ok(E[part], part); assert.equal(typeof E.mount, 'function'); }]];
