import assert from 'node:assert/strict'; import { read, loadFiles } from './harness.mjs'; import { lintJs } from './css-lint.mjs';
export const tests = [
  ['host: the desktop files are clean and within the line budget', () => {
    for (const f of ['ui/js/desktop/apps/homedash.js', 'ui/js/desktop/apps/homedash-home.js']) { const text = read(f); assert.deepEqual(lintJs(text, f), [], f); assert.ok(text.split('\n').length < 1100, f); } }],
  ['host: the visibility signal follows the tab, the window and its space (Docker Manager rule)', () => {
    const { window, document } = loadFiles(['ui/js/desktop/apps/homedash.js'], { dom: true }), listeners = [];
    document.hidden = false; document.addEventListener = (type, fn) => listeners.push([type, fn]);
    const cls = new Set(), win = { hidden: false, style: { display: '' }, classList: { contains: c => cls.has(c) }, getClientRects: () => [1] };
    const life = new AbortController(), seen = [], sig = window.HomeDashApp.visibilitySignal(document.createElement('div'), win, life.signal);
    assert.equal(sig.visible(), true); sig.subscribe(v => seen.push(v));
    cls.add('vd-space-hidden'); listeners.find(x => x[0] === 'visibilitychange')[1](); assert.equal(sig.visible(), false);
    cls.delete('vd-space-hidden'); document.hidden = true; listeners[0][1](); assert.equal(sig.visible(), false);
    document.hidden = false; listeners[0][1](); assert.deepEqual(seen, [false, true]); life.abort(); }],
  ['host: the chrome CSS paints from the theme materials and keeps the app CSS rules', () => {
    const css = read('ui/css/desktop-app-homedash.css'), plain = css.replace(/\/\*[\s\S]*?\*\//g, '');
    assert.ok(plain.includes('var(--vd-mat-'), 'reads no --vd-mat-* token'); assert.ok(!/#[0-9a-fA-F]{3,8}\b/.test(plain), 'hex colour');
    assert.ok(!/\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(/i.test(plain), 'literal colour function'); assert.ok(!/\[data-theme/.test(plain), '[data-theme]');
    assert.ok(!/uppercase/i.test(plain), 'uppercase'); assert.ok(!/light-dark\(/.test(plain), 'light-dark()'); assert.ok(css.split('\n').length <= 900);
    for (const m of plain.matchAll(/([^{}]+)\{([^{}]*)\}/g)) { const sel = m[1].trim(), body = m[2];
      if (/!important/.test(body)) assert.ok(/\[hidden\]$/.test(sel), '!important outside [hidden]: ' + sel);
      if (/\b(transition|animation)\s*:/.test(body)) assert.ok(sel.split(',').every(s => s.trim().startsWith('body:not([data-animations="false"])')), 'ungated motion: ' + sel); }
    assert.ok(plain.includes('@media (prefers-reduced-motion: no-preference)')); }]
];
