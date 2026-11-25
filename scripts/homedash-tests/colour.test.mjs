import assert from 'node:assert/strict'; import { loadEngine } from './harness.mjs';
const boot = dom => loadEngine(['hd-core.js', 'hd-colour.js'], { dom });
function rng(seed) { return () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; }; }
export const tests = [
  ['conversions and contrast', () => { const C = boot().E.colour;
    assert.equal(C.contrast('#000000', '#FFFFFF').toFixed(2), '21.00'); assert.equal(C.contrast('#777777', '#FFFFFF').toFixed(2), '4.48');
    assert.deepEqual([...C.hexToRgb('#abc')], [170, 187, 204]); assert.equal(C.rgbToHex([170, 187, 204]), '#AABBCC');
    for (const h of ['#1A1F29', '#7AA7FF', '#C4352B', '#FFFFFF', '#000000']) { const a = C.hexToRgb(h), b = C.hexToRgb(C.oklchToHex(C.rgbToOklch(a))); a.forEach((v, i) => assert.ok(Math.abs(v - b[i]) <= 1, h)); }
    const w = C.rgbToOklch([255, 255, 255]); assert.ok(Math.abs(w[0] - 1) < 1e-3 && w[1] < 1e-3);
    assert.deepEqual([...C.hsvToRgb(0, 1, 1).map(Math.round)], [255, 0, 0]); assert.deepEqual([...C.hsvToRgb(120, 1, 1).map(Math.round)], [0, 255, 0]); }],
  ['the custom palette solver meets every pair for 500 random inputs', () => { const C = boot().E.colour, r = rng(7);
    for (let i = 0; i < 500; i++) {
      const accent = C.rgbToHex([r() * 255, r() * 255, r() * 255]), input = { accent, surface_hue: r() * 360, scheme: r() < 0.5 ? 'dark' : 'light' };
      const t = C.solvePalette(input).tokens, at = (a, b) => C.contrast(t[a], t[b]), tag = JSON.stringify(input);
      for (const name of C.PALETTE_TOKENS) assert.match(t[name], /^#[0-9A-F]{6}$/, name + ' ' + tag);
      for (const [a, b, min] of [['ink','surface-solid',4.5],['ink','canvas',4.5],['ink-muted','surface-solid',5],['ink-muted','canvas',4.5],['on-ink','on',4.5],['ink-on-accent','accent',4.5],['accent','surface-solid',3],['selected-ink','selected',4.5],['btn-ink','btn',4.5],['ok','surface-solid',4.5],['warn','surface-solid',4.5],['alert','surface-solid',4.5],['info','surface-solid',4.5]])
        assert.ok(at(a, b) >= min, `${a} on ${b} ${at(a, b).toFixed(2)} ${tag}`);
    } }],
  ['ensureContrast keeps passing colours, steps lightness with hue kept and handles several backgrounds', () => { const C = boot().E.colour;
    assert.equal(C.ensureContrast('#1D5FAF', '#FFFFFF', 3), '#1d5faf'); assert.equal(C.ensureContrast('#zzz', '#FFFFFF', 3), '#zzz');
    const r = rng(11);
    for (let i = 0; i < 400; i++) {
      const hex = C.rgbToHex([r() * 255, r() * 255, r() * 255]), bg = C.rgbToHex([r() * 255, r() * 255, r() * 255]), out = C.ensureContrast(hex, bg, 3), best = Math.max(C.contrast('#000000', bg), C.contrast('#ffffff', bg));
      assert.match(out, /^#[0-9a-f]{6}$/); assert.ok(C.contrast(out, bg) >= Math.min(3, best) - 1e-9, hex + ' on ' + bg + ' -> ' + out);
      if (C.contrast(hex, bg) >= 3) assert.equal(out, hex.toLowerCase(), 'a passing colour is unchanged');
    }
    const warm = C.ensureContrast('#F2B84B', '#FFFFFF', 3.05), hw = C.rgbToOklch(C.hexToRgb('#F2B84B')), ho = C.rgbToOklch(C.hexToRgb(warm));
    assert.ok(C.contrast(warm, '#FFFFFF') >= 3.05 && ho[0] < hw[0] && Math.abs(ho[2] - hw[2]) < 6, 'darker, same hue: ' + warm);
    const mid = C.ensureContrast('#808080', ['#FFFFFF', '#000000'], 3); assert.match(mid, /^#[0-9a-f]{6}$/);
    assert.equal(C.ensureContrast('#7AA7FF', '#808080', 99), C.ensureContrast('#7AA7FF', '#808080', 99), 'unreachable minimum returns the best extreme deterministically'); }],
  ['applyPalette writes --hd- custom properties', () => { const { E, document } = boot(true); const el = document.createElement('div');
    E.colour.applyPalette(el, E.colour.solvePalette({ accent: '#8E5BD9', surface_hue: 285, scheme: 'dark' }).tokens);
    assert.match(el.style.getPropertyValue('--hd-ink'), /^#/); E.colour.clearPalette(el); assert.equal(el.style.getPropertyValue('--hd-ink'), ''); }]
];
