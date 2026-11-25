import assert from 'node:assert/strict'; import vm from 'node:vm'; import { read, ENGINE_ORDER, ENGINE_DIR } from './harness.mjs'; import { lintCss, lintJs } from './css-lint.mjs'; import { createDocument } from './dom.mjs';
// Boots the real lab script against the engine in the fake DOM. The lab renders after `await document.fonts.ready`, so a fixture defined
// synchronously right after the script is evaluated is in place before the first render.
async function bootLab(search, defineFixture) {
  const doc = createDocument(), tick = cb => setTimeout(cb, 0), win = { document: doc, devicePixelRatio: 1, scrollY: 0, addEventListener() {}, hdLabEarlyErrors: [], location: { search }, URLSearchParams, requestAnimationFrame: tick };
  const scope = doc.createElement('div'); scope.setAttribute('id', 'hd-lab-scope'); const main = doc.createElement('main'); main.setAttribute('id', 'hd-lab'); scope.appendChild(main); doc.body.appendChild(scope);
  Object.assign(doc, { head: doc.createElement('head'), fonts: { ready: Promise.resolve() }, getAnimations: () => [], getElementById: id => doc.body.querySelector('#' + id), querySelector: s => doc.body.querySelector(s) });
  const ctx = { window: win, document: doc, location: win.location, URLSearchParams, console, setTimeout, clearTimeout, AbortController, Intl, Map, Set, JSON, Math, Date, URL, Promise, requestAnimationFrame: tick, fetch: async () => ({ ok: false }) }; // no locale files in the Node double
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
  ['M3 lab: the new states, the moreinfo special type and the stricter option hit probe are in place', () => { const lab = read('ui/testdata/homedash-lab.js'), probes = read('ui/testdata/homedash-lab-probes.js');
    for (const s of ['mid', 'armed', 'drag', 'colour', 'dimonly', 'open']) assert.ok(lab.includes("'" + s + "'"), s);
    assert.match(lab, /E\.moreinfo\.build\(/); assert.match(probes, /\[role="option"\]/); assert.match(probes, /\.hd-pop-layer \[role="option"\]/);
    const fx = read('ui/testdata/homedash-lab/w1-fixtures.js'); vm.runInContext; new vm.Script(fx, { filename: 'w1-fixtures.js' }); assert.deepEqual(lintJs(fx, 'w1-fixtures.js'), []); }],
  ['probe colour maths', () => { const ctx = { window: {}, Math, JSON }; vm.createContext(ctx); vm.runInContext(read('ui/testdata/homedash-lab-probes.js'), ctx);
    const p = ctx.window.hdLabProbes; assert.equal(p.ratio([0, 0, 0, 1], [255, 255, 255, 1]).toFixed(2), '21.00'); assert.deepEqual([...p.over([255, 255, 255, 0.5], [0, 0, 0, 1])], [127.5, 127.5, 127.5, 1]); }],
  // The canvas stub follows the real rules: fillStyle keeps its previous value on an invalid string; "#rrggbb" and "#v" test strings are valid, "nonsense" is not; oklch(...) maps to a fixed grey.
  ['probe colour parsing: exact rgb, canvas fallback for modern syntaxes, an unreadable colour throws and never reads as transparent, 3.7:1 fails', () => {
    let fill = '#000000', px = [0, 0, 0, 0]; const valid = s => /^(#[0-9a-f]{6}|oklch\(.*\)|color\(srgb .*\)|lab\(.*\)|color-mix\(.*\))$/i.test(s);
    const grey = s => { const m = /^#([0-9a-f]{2})/i.exec(s); return m ? parseInt(m[1], 16) : s.startsWith('oklch') ? 133 : 51; };
    const g = { get fillStyle() { return fill; }, set fillStyle(v) { if (valid(v)) fill = v.toLowerCase(); }, clearRect() { px = [0, 0, 0, 0]; }, fillRect() { const c = grey(fill); px = [c, c, c, 255]; }, getImageData() { return { data: px }; } };
    const ctx = { window: {}, Math, JSON, Number, String, Error, document: { createElement: () => ({ getContext: () => g }) } }; vm.createContext(ctx); vm.runInContext(read('ui/testdata/homedash-lab-probes.js'), ctx);
    const p = ctx.window.hdLabProbes;
    assert.deepEqual([...p.rgba('rgb(10, 20, 30)')], [10, 20, 30, 1]); assert.deepEqual([...p.rgba('rgba(10, 20, 30, 0.5)')], [10, 20, 30, 0.5]); assert.deepEqual([...p.rgba('rgb(10 20 30 / 25%)')], [10, 20, 30, 0.25]); assert.deepEqual([...p.rgba('transparent')], [0, 0, 0, 0]);
    assert.deepEqual([...p.rgba('color(srgb 0.2 0.4 0.6)')], [51, 51, 51, 1]); assert.deepEqual([...p.rgba('oklch(0.5 0.1 200)')], [133, 133, 133, 1]); assert.deepEqual([...p.rgba('color-mix(in oklch, #858585 100%, #fff)')], [51, 51, 51, 1]);
    for (const bad of ['nonsense', '', 'currentcolor', 'oklch nonsense']) assert.throws(() => p.rgba(bad), /colour cannot be/, bad);
    const list = []; assert.equal(p.safeRgba('nonsense', 'cell .x', list), null); assert.match(list[0], /unreadable colour cell \.x/);
    // the 3.7:1 case: white ink on a mid grey (133) that the oklch syntax produces; it must fail the 4.5:1 text threshold
    const wash = p.rgba('oklch(0.5 0.1 200)'), v = p.worst([255, 255, 255, 1], p.composites([[wash]])); assert.ok(v > 3.6 && v < 3.8 && v < 4.5, 'ratio ' + v);
    assert.equal(typeof p.selfTest, 'function'); assert.match(read('ui/testdata/homedash-lab-probes.js'), /for \(const p of selfTest\(\)\) r\.resolution\.failures\.push\(p\)/); }],
  ['probe gradient backgrounds: stops composite over the colour, the worst value counts, url() stays unmeasured', () => { const ctx = { window: {}, Math, JSON }; vm.createContext(ctx); vm.runInContext(read('ui/testdata/homedash-lab-probes.js'), ctx);
    const p = ctx.window.hdLabProbes, layers = p.gradientLayers('linear-gradient(rgba(255, 255, 255, 0.5), rgba(0, 0, 0, 0) 46%), linear-gradient(180deg, rgb(0, 0, 0), rgb(10, 10, 10) 50%)');
    assert.equal(layers.length, 2); assert.deepEqual([...layers.map(l => l.length)], [2, 2]); assert.deepEqual([...layers[0][0]], [255, 255, 255, 0.5]);
    assert.equal(p.gradientLayers('linear-gradient(rgb(1, 1, 1), rgb(2, 2, 2)), url(x.png)'), null); assert.equal(p.gradientLayers('url("x.png")'), null); const under = [10, 20, 30, 1], wall = p.gradientLayers('linear-gradient(rgb(1, 1, 1), rgb(2, 2, 2)), url("w.png"), linear-gradient(rgb(9, 9, 9), rgb(8, 8, 8))', under);
    assert.equal(wall.length, 2); assert.equal(wall.opaque, true); assert.deepEqual([...wall[1][0]], under); // the wallpaper becomes the canvas underlay, layers below it are dropped
 assert.equal(p.gradientLayers('linear-gradient(rgb(1, 1, 1), rgb(2, 2, 2)), none').length, 1); assert.equal(p.gradientLayers('radial-gradient(circle at 15% 0%, rgba(1, 2, 3, 0.1), transparent)')[0].length, 2);
    const bgs = [...p.composites([layers[0], [[0, 0, 0, 1]]]).map(c => Math.round(c[0]))].sort((a, b) => a - b); assert.deepEqual(bgs, [0, 128]); // white stop at 50 % over black, and the transparent stop
    const grey = [[0, 0, 0, 0.5]]; assert.equal(p.composites([grey, [[255, 255, 255, 1]]]).length, 1);
    const dark = [0, 0, 0, 1]; assert.equal(p.worst(dark, [[255, 255, 255, 1], [128, 128, 128, 1]]).toFixed(2), '5.32'); // the grey composite is the worst case
    assert.ok(p.worst([255, 255, 255, 1], p.composites([layers[0], [[0, 0, 0, 1]]])) < 21); }],
  ['probe hosts and hidden text: an svg graphic starts at its own svg, visually hidden text is not measured', () => { const ctx = { window: {}, Math, JSON }; vm.createContext(ctx); vm.runInContext(read('ui/testdata/homedash-lab-probes.js'), ctx);
    const p = ctx.window.hdLabProbes, svg = { id: 'svg' }, parent = { id: 'parent' };
    assert.equal(p.graphicHost({ ownerSVGElement: svg, parentElement: parent }, true), svg); assert.equal(p.graphicHost({ ownerSVGElement: null, parentElement: parent }, true), parent); assert.equal(p.graphicHost({ ownerSVGElement: null, parentElement: parent }, false), parent);
    const el = inside => ({ closest: s => (inside && s === '.hd-sr-only' ? {} : null) }), box = (w, h) => ({ width: w, height: h });
    assert.equal(p.visuallyHidden(el(true), { clip: 'auto' }, box(40, 12)), true); assert.equal(p.visuallyHidden(el(false), { clip: 'rect(0px, 0px, 0px, 0px)' }, box(40, 12)), true);
    assert.equal(p.visuallyHidden(el(false), { clip: 'auto' }, box(1, 1)), true); assert.equal(p.visuallyHidden(el(false), { clip: 'auto' }, box(40, 12)), false); }],
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
