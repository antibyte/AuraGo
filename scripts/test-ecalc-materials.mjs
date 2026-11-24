// ECalc desktop app: test of the material table. ui/js/desktop/apps/ecalc/ecalc-materials.js runs in a Node vm without a
// document. Checks: every print ink reads on its material (WCAG contrast >= 4.5 against every stop), the fill/paint
// values of a scope, the defs (each gradient once, in order of first use, deterministic) and the safe ids.
// Run: node scripts/test-ecalc-materials.mjs   (npm run test:ecalc)
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';

const root = path.resolve(import.meta.dirname, '..');
const window = {};
vm.runInContext(fs.readFileSync(path.join(root, 'ui/js/desktop/apps/ecalc/ecalc-materials.js'), 'utf8'), vm.createContext({ window }));
const M = window.ECalcMaterials;
assert.ok(M && M.MATERIALS && M.INKS && typeof M.scope === 'function', 'the file publishes window.ECalcMaterials');

const luminance = hex => {
    const [r, g, b] = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16) / 255).map(c => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
};
const contrast = (a, b) => {
    const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
    return (hi + 0.05) / (lo + 0.05);
};

// (a) ink contrast
let inked = 0;
for (const [name, material] of Object.entries(M.MATERIALS)) {
    if (!material.ink) { assert.equal(M.ink(name), '', name + ' has no ink'); continue; }
    inked++;
    const ink = M.INKS[material.ink];
    assert.ok(ink, name + ' names a known ink');
    assert.equal(M.ink(name), ink);
    const faces = material.kind === 'flat' ? [material.hex] : material.stops.map(stop => stop[1]);
    for (const face of faces) assert.ok(contrast(ink, face) >= 4.5, name + ': ' + ink + ' on ' + face + ' = ' + contrast(ink, face).toFixed(2));
}
assert.ok(inked >= 8, 'the inked materials were checked');
assert.equal(M.ink('nope'), '');

// (b) fill and paint
const s = M.scope('ecalc-1-x');
assert.equal(s.fill('steel'), 'url(#ecalc-1-x-steel)');
assert.equal(s.fill('tin', 'x'), 'url(#ecalc-1-x-tin-x)');
assert.equal(s.fill('cavity'), '#16181b');
assert.equal(s.paint('shadow'), ' fill="#000000" fill-opacity=".16"');
assert.equal(s.paint('cavity'), ' fill="#16181b"');
assert.equal(s.paint('steel'), ' fill="url(#ecalc-1-x-steel)"');
assert.equal(s.fill('nope'), '');
assert.equal(s.paint('nope'), '');
assert.equal(s.fill('ceramic', 'x'), 'url(#ecalc-1-x-ceramic)', 'radial ignores across');
const defs = s.defs();
assert.match(defs, /<linearGradient id="ecalc-1-x-tin-x" x1="0" y1="0" x2="1" y2="0">/);
assert.match(defs, /<linearGradient id="ecalc-1-x-steel" x1="0" y1="0" x2="0" y2="1">/);
assert.match(defs, /<radialGradient id="ecalc-1-x-ceramic" cx=".36" cy=".3" r=".8">/);
assert.ok(defs.includes('<stop offset=".35" stop-color="#f3f5f6"/>'));

// (c) defs: once each, first-use order, deterministic
assert.equal(M.scope('a').defs(), '');
const run = () => { const c = M.scope('ecalc-2'); c.fill('tin'); c.fill('steel'); c.fill('tin'); c.paint('steel'); c.fill('cavity'); return c.defs(); };
const first = run();
assert.equal(first, run(), 'the same calls give the same string');
assert.equal((first.match(/<linearGradient /g) || []).length, 2, 'each gradient once');
assert.ok(first.indexOf('ecalc-2-tin') < first.indexOf('ecalc-2-steel'), 'first-use order');
assert.ok(first.startsWith('<defs>') && first.endsWith('</defs>'));

// (d) safe ids
assert.match(M.safeId('"><x'), /^[A-Za-z][A-Za-z0-9_-]*$/);
assert.equal(M.safeId('"><x'), 'x');
assert.equal(M.safeId('"><1'), 'ec1');
assert.equal(M.safeId(''), 'ec');
assert.equal(M.safeId(null), 'ec');
assert.equal(M.safeId('9a'), 'ec9a');
assert.equal(M.safeId('ok-1_b'), 'ok-1_b');

// (e) gloss carries stop opacity
for (const name of ['gloss', 'sheen']) {
    const c = M.scope('g'); c.fill(name);
    assert.equal((c.defs().match(/stop-opacity="/g) || []).length, 5, name + ' stops carry stop-opacity');
}
assert.ok(!M.scope('t').fill('tin') || !M.scope('t').defs().includes('stop-opacity'), 'tin has no stop-opacity');
assert.ok(Object.isFrozen(M.MATERIALS) && Object.isFrozen(M.INKS));
console.log('ecalc materials: ok (' + Object.keys(M.MATERIALS).length + ' materials)');
