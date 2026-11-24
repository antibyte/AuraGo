import assert from 'node:assert/strict'; import vm from 'node:vm'; import { read, ENGINE_ORDER, ENGINE_DIR } from './harness.mjs'; import { lintCss, lintJs } from './css-lint.mjs'; import { createDocument } from './dom.mjs';
// Boots the real lab script against the engine in the fake DOM. The lab renders after `await document.fonts.ready`, so a fixture defined
// synchronously right after the script is evaluated is in place before the first render.
async function bootLab(search, defineFixture) {
  const doc = createDocument(), tick = cb => setTimeout(cb, 0), win = { document: doc, devicePixelRatio: 1, scrollY: 0, addEventListener() {}, hdLabEarlyErrors: [], location: { search }, URLSearchParams, requestAnimationFrame: tick };
  const scope = doc.createElement('div'); scope.setAttribute('id', 'hd-lab-scope'); const main = doc.createElement('main'); main.setAttribute('id', 'hd-lab'); scope.appendChild(main); doc.body.appendChild(scope);
  Object.assign(doc, { head: doc.createElement('head'), fonts: { ready: Promise.resolve() }, getAnimations: () => [], getElementById: id => doc.body.querySelector('#' + id), querySelector: s => doc.body.querySelector(s) });
  const ctx = { window: win, document: doc, location: win.location, URLSearchParams, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, Promise, requestAnimationFrame: tick };
  vm.createContext(ctx);
  for (const f of ENGINE_ORDER) vm.runInContext(read(`${ENGINE_DIR}/${f}`), ctx, { filename: f });
  vm.runInContext('window.hdLabProbes = {};', ctx); vm.runInContext(read('ui/testdata/homedash-lab.js'), ctx, { filename: 'homedash-lab.js' });
  if (defineFixture) defineFixture(win.hdLab);
  for (let i = 0; i < 100 && !win.hdLab.ready && !win.hdLab.error; i++) await new Promise(r => setTimeout(r, 5));
  return { hdLab: win.hdLab, doc };
}
export const tests = [
  ['the lab page loads the engine in ENGINE_ORDER, then the probes and the gallery', () => { const srcs = [...read('ui/testdata/homedash-lab.html').matchAll(/<script src="([^"]+)"/g)].map(m => m[1]);
    assert.deepEqual(srcs, [...ENGINE_ORDER.map(f => '/js/homedash/' + f), '/testdata/homedash-lab-probes.js', '/testdata/homedash-lab.js']); }],
  ['lab CSS and JS obey the guard and parse', () => { assert.deepEqual(lintCss(read('ui/testdata/homedash-lab.css'), { file: 'homedash-lab.css', kind: 'lab' }), []);
    for (const f of ['ui/testdata/homedash-lab.js', 'ui/testdata/homedash-lab-probes.js']) { const t = read(f); assert.deepEqual(lintJs(t, f), []); new vm.Script(t, { filename: f }); } }],
  ['probe colour maths', () => { const ctx = { window: {}, Math, JSON }; vm.createContext(ctx); vm.runInContext(read('ui/testdata/homedash-lab-probes.js'), ctx);
    const p = ctx.window.hdLabProbes; assert.equal(p.ratio([0, 0, 0, 1], [255, 255, 255, 1]).toFixed(2), '21.00'); assert.deepEqual([...p.over([255, 255, 255, 0.5], [0, 0, 0, 1])], [127.5, 127.5, 127.5, 1]); }],
  ['applyState prefers a fixture scenario and rejects bad input', async () => {
    const view = pending => ({ id: 'lab:x', kind: 'switch', name: 'Lamp', value: 'on', available: true, status: 'ok', readonly: false, capabilities: ['toggle'], pending, stale: false, tone: null });
    const { hdLab, doc } = await bootLab('?types=entity&states=off,on&palette=dark', lab => lab.fixtures.define('entity', '*', { scenarios: { pending: { from: view(false), to: view(true) }, typo: { from: 'on', to: 'pendng' } } }));
    assert.equal(hdLab.error, ''); assert.ok(hdLab.ready); const n = hdLab.instances.length; assert.equal(n, 4);
    const root = doc.body.querySelector('.hd-lab-panel'), motion = root.getAttribute('data-hd-motion'), pending = () => Array.from(hdLab.instances, i => i.el.getAttribute("data-hd-pending"));
    assert.equal(await hdLab.applyState('pending'), n); assert.deepEqual(pending(), ['true', 'true', 'true', 'true']); assert.equal(root.getAttribute('data-hd-motion'), motion);
    assert.equal(await hdLab.applyState('off'), n); assert.deepEqual(pending(), [null, null, null, null]);
    await assert.rejects(() => hdLab.applyState('nope'), /unknown state or scenario/);
    await assert.rejects(() => hdLab.applyState('typo'), /unknown state/);
    const scripts = await bootLab('?scripts=/testdata/../x.js'); assert.match(scripts.hdLab.error, /bad param scripts/);
    const graphics = await bootLab('?graphics=' + encodeURIComponent('["div["]')); assert.match(graphics.hdLab.error, /bad param graphics/); }]
];
