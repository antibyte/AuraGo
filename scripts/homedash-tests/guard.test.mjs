import assert from 'node:assert/strict'; import fs from 'node:fs'; import path from 'node:path';
import { read, root } from './harness.mjs'; import { lintCss, lintJs } from './css-lint.mjs'; import { PARTS } from './bundle.test.mjs';
const BAD = `.x { color: #fff; }
.hd-a { color: red; background: rgb(0 0 0); text-transform: uppercase; }
.hd-b { transition: opacity 1s; }
@media (prefers-reduced-motion: no-preference) { .hd-c { animation: spin 1s; } }
@keyframes spin { to { width: 10px; } }
.hd-glass-pane { backdrop-filter: blur(30px); }
.hd-e { --hd-x: #123456; --vd-y: 1; }
.hd-f { background: url(https://example.com/a.png); color: light-dark(var(--hd-a), var(--hd-b)); }
@font-face { font-family: "Plain"; src: url('/fonts/a.ttf'); }
[data-theme="x"] .hd-g { color: var(--hd-ink) !important; }
.cc-banner { color: var(--hd-ink); }
@media (prefers-reduced-motion: no-preference) { .hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"]) .hd-x { transition: width 1s; } }`;
const GOOD = `.hd-root { --hd-n-ink: #F1F4F9; }
.hd-root[data-hd-scheme="light"] { --hd-n-ink: #12161D; }
[data-hd-family="glass"] { --hd-glass: linear-gradient(#FFF, transparent); }
@media (prefers-reduced-motion: no-preference) { .hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"]) .hd-toggle-thumb { transition: transform var(--hd-dur-state) var(--hd-ease-spring); } }
@keyframes hd-pulse { from { opacity: 1; } to { opacity: .5; transform: scale(1.02); } }
.hd-frame { backdrop-filter: blur(18px); } .hd-sheet { backdrop-filter: blur(24px); }
.hd-root [hidden] { display: none !important; }
@font-face { font-family: "HomeDash Test"; src: url('/fonts/x.ttf'); }
.hd-icon { background: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E"); }
@media (prefers-reduced-motion: no-preference) { .hd-root:not([data-hd-motion="reduced"]):not([data-hd-motion="none"]) .hd-dial-arc { transition: stroke-dasharray var(--hd-dur-value) var(--hd-ease-out), opacity var(--hd-dur-quick) linear; } }`;
const walk = d => fs.readdirSync(d, { withFileTypes: true }).flatMap(e => e.isDirectory() ? walk(path.join(d, e.name)) : e.name.endsWith('.js') ? [path.join(d, e.name)] : []);
export const tests = [
  ['the CSS guard reports every rule', () => { const v = lintCss(BAD, { file: 'bad.css', kind: 'family' }).join('\n');
    for (const want of ['class .x', 'literal colour in .x', 'named colour', 'uppercase', 'ungated motion in .hd-b', 'ungated motion in .hd-c', 'keyframes animate width', 'backdrop-filter outside .hd-frame and .hd-sheet', 'blur 30px', 'literal colour in --hd-x outside a token block', 'custom property --vd-y', 'url(https://example.com', 'light-dark()', 'must start with "HomeDash "', '[data-theme]', '!important', 'cc- prefix', 'transition of width'])
      assert.ok(v.includes(want), 'missing: ' + want + '\n' + v);
    assert.deepEqual(lintCss(GOOD, { file: 'good.css', kind: 'family' }), []); }],
  ['every bundle part passes the guard', () => { for (const p of PARTS) assert.deepEqual(lintCss(read(p), { file: p, kind: /family-/.test(p) ? 'family' : 'part' }), [], p); }],
  ['the JS guard reports forbidden calls', () => { const v = lintJs('a.innerHTML = x; b.setAttribute("style", y); alert(1); new Function("x"); c.style.cssText = ""', 'x.js').join('\n');
    for (const want of ['innerHTML', "setAttribute('style'", 'browser dialogs', 'new Function', 'style.cssText']) assert.ok(v.includes(want), want);
    assert.deepEqual(lintJs('E.sheet.confirm(x); el.style.setProperty("--hd-v", 1);', 'ok.js'), []); }],
  ['engine JS is clean and within the line budget', () => { for (const f of walk(path.join(root, 'ui/js/homedash'))) { const t = fs.readFileSync(f, 'utf8'), n = t.split('\n').length;
    assert.deepEqual(lintJs(t, f), [], f); assert.ok(n < 1100, `${f}: ${n} lines`); if (n > 700) console.warn(`WARN ${f}: ${n} lines (aim under 700)`); } }]
];
