import assert from 'node:assert/strict'; import vm from 'node:vm'; import { read, ENGINE_ORDER } from './harness.mjs'; import { lintCss, lintJs } from './css-lint.mjs';
export const tests = [
  ['the lab page loads the engine in ENGINE_ORDER, then the probes and the gallery', () => { const srcs = [...read('ui/testdata/homedash-lab.html').matchAll(/<script src="([^"]+)"/g)].map(m => m[1]);
    assert.deepEqual(srcs, [...ENGINE_ORDER.map(f => '/js/homedash/' + f), '/testdata/homedash-lab-probes.js', '/testdata/homedash-lab.js']); }],
  ['lab CSS and JS obey the guard and parse', () => { assert.deepEqual(lintCss(read('ui/testdata/homedash-lab.css'), { file: 'homedash-lab.css', kind: 'lab' }), []);
    for (const f of ['ui/testdata/homedash-lab.js', 'ui/testdata/homedash-lab-probes.js']) { const t = read(f); assert.deepEqual(lintJs(t, f), []); new vm.Script(t, { filename: f }); } }],
  ['probe colour maths', () => { const ctx = { window: {}, Math, JSON }; vm.createContext(ctx); vm.runInContext(read('ui/testdata/homedash-lab-probes.js'), ctx);
    const p = ctx.window.hdLabProbes; assert.equal(p.ratio([0, 0, 0, 1], [255, 255, 255, 1]).toFixed(2), '21.00'); assert.deepEqual([...p.over([255, 255, 255, 0.5], [0, 0, 0, 1])], [127.5, 127.5, 127.5, 1]); }]
];
