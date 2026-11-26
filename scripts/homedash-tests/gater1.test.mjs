import assert from 'node:assert/strict'; import { read } from './harness.mjs'; import { boot } from './designer-kit.mjs';
// Gate E-S round 1 (G1-G12): the guards that are not geometry. The zoom maths and gesture conversion live in render.test.mjs and gestures.test.mjs, the palette thumbs in palette.test.mjs.
const host = read('ui/css/desktop-app-homedash.css').replace(/\r\n/g, '\n').replace(/\/\*[\s\S]*?\*\//g, ''), widgets = read('ui/css/homedash-widgets.css').replace(/\/\*[\s\S]*?\*\//g, '');
const rule = (css, selector) => { const i = css.indexOf(selector + ' {'); assert.ok(i >= 0, 'no rule ' + selector); return css.slice(i, css.indexOf('}', i)); };
const mkbar = () => { const { E, document } = boot('hd-editbar.js'), bar = E.editbar.create({ t: k => k, onCommand() {} }); document.body.appendChild(bar.el); return { E, bar }; };
export const tests = [
  ['G6: the reset-to-automatic control is an icon button named and titled by the words, with no text of its own', () => { const { bar } = mkbar(), r = bar.el.querySelector('.hd-bp-reset');
    assert.equal(r.getAttribute('aria-label'), 'homedash.bp.reset'); assert.equal(r.getAttribute('title'), 'homedash.bp.reset'); assert.equal(r.textContent, ''); assert.ok(r.querySelector('svg, .hd-ico'), 'an icon');
    assert.equal(r.getAttribute('data-hd-cmd'), 'reset_layout'); bar.update({ bp: 'md', override: { md: true } }); assert.equal(r.hasAttribute('hidden'), false); bar.update({ bp: 'md', override: {} }); assert.equal(r.hasAttribute('hidden'), true); }],
  ['G10: while the conflict bar carries Resolve the edit bar shows the state text only; other failed phases keep their one action', () => { const { bar } = mkbar(), act = bar.el.querySelector('.hd-save-action'), text = bar.el.querySelector('.hd-save-text');
    for (const phase of ['conflict', 'gone']) { bar.update({ save: { phase } }); assert.equal(act.hasAttribute('hidden'), true, phase + ': no second Resolve'); assert.equal(act.getAttribute('data-hd-cmd'), null); assert.ok(text.textContent.length > 0, phase + ': the state text stays'); }
    bar.update({ save: { phase: 'retrying', nextRetryAt: 5000 } }); assert.equal(act.hasAttribute('hidden'), false); assert.equal(act.getAttribute('data-hd-cmd'), 'retry_save');
    bar.update({ save: { phase: 'invalid' }, canUndo: true }); assert.equal(act.getAttribute('data-hd-cmd'), 'undo'); }],
  ['G2: the zoomed page, and every line the edit layer draws, are sized in visual pixels (--hdx-u = 1px / zoom)', () => {
    assert.match(rule(host, '.hd-root[data-hd-mode="design"] .hd-page[data-hd-zoomed]'), /zoom: var\(--hd-e-zoom\)/); assert.match(rule(host, '.hd-root[data-hd-mode="design"] .hd-page'), /--hdx-u: calc\(1px \/ var\(--hd-e-zoom, 1\)\)/);
    for (const sel of ['.hd-handle', '.hd-handle::before', '.hd-sel-chip', '.hd-ghost', '.hd-marquee', '.hd-guide[data-axis="x"]', '.hd-guide[data-axis="y"]']) assert.match(rule(host, sel), /--hdx-u/, sel);
    assert.match(rule(host, '.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[aria-selected="true"]'), /--hdx-u/); assert.match(rule(host, '.hd-root[data-hd-mode="design"] .hd-page > .hd-widget:focus-visible'), /--hdx-u/);
    assert.ok(/@media \(pointer: coarse\) \{\n  \.hd-handle \{ inline-size: calc\(44 \* var\(--hdx-u\)\)/.test(host), 'coarse handles keep 44 visual px'); }],
  ['G4: only the card\'s own radio marks it selected (the sample switch inside a preview is aria-checked too); the check mark and a 2 px ring show it', () => {
    assert.ok(!/(\.hd-look-family|\.hd-insp-variant):has\(\[aria-checked="true"\]\)/.test(host), 'the old selector matched every card');
    const r = rule(host, '.hd-look-family:has(> .hd-look-fhit[aria-checked="true"])'); assert.match(r, /border-color: var\(--hdx-e-ring\)/); assert.match(r, /box-shadow: 0 0 0 1px var\(--hdx-e-ring\)/);
    for (const sel of ['.hd-insp-variant:has(> .hd-insp-vhit[aria-checked="true"])', '.hd-look-family:has(> .hd-look-fhit[aria-checked="true"])']) assert.ok(host.includes(sel + '::before') && host.includes(sel + '::after'), 'check mark on ' + sel); }],
  ['G5: a segmented option is never ellipsised: it keeps its text width, equal widths while everything fits, and the row wraps', () => {
    assert.match(rule(host, '.hd-f-seg'), /flex-wrap: wrap/); assert.match(rule(host, '.hd-root .hd-f-seg-opt'), /min-inline-size: max-content/); assert.ok(!/\.hd-f-seg-opt > span \{[^}]*ellipsis/.test(host)); assert.ok(!/\.hd-f-seg-opt > span \{[^}]*overflow: hidden/.test(host)); }],
  ['G7 and G8: tab labels have a line box that holds descenders; a focused frame gets a 2 px ring in whole layout px, the selection a dashed outline of its own', () => {
    assert.match(rule(host, '.hd-ptab-label'), /line-height: 1\.25/); const f = rule(host, '.hd-root[data-hd-mode="design"] .hd-page > .hd-widget:focus-visible'), s = rule(host, '.hd-root[data-hd-mode="design"] .hd-page > .hd-widget[aria-selected="true"]');
    assert.match(f, /box-shadow: 0 0 0 2px/); assert.match(f, /round\(up, calc\(2 \* var\(--hdx-u\)\), 1px\)/); assert.match(s, /outline: 2px dashed/); assert.match(s, /outline-width: round\(up/); }],
  ['G5 (German): a palette card label wraps instead of being ellipsised', () => { assert.match(rule(host, '.hd-pal-name, .hd-pal-variant'), /white-space: normal/); }],
  ['G9: coarse pointers get 44 x 44 for the breakpoint buttons, the device toggle and clear button and the switch', () => {
    const coarse = host.slice(host.lastIndexOf('@media (pointer: coarse)')); assert.match(coarse, /\.hd-bp-opt \{ inline-size: 44px; block-size: 44px; \}/); assert.match(coarse, /\.hd-f-switch \{ inline-size: 44px; block-size: 44px; \}/);
    assert.match(coarse, /\.hd-root \.hd-ep-toggle \{ inline-size: 44px; block-size: 44px; \}/); assert.match(coarse, /\.hd-root \.hd-ep-box \.hd-btn \{ min-inline-size: 44px; min-block-size: 44px; \}/); }],
  ['G11: a one-row sensor numeral gets the empty state line back (widget CSS, no token change)', () => {
    assert.match(widgets, /@container hd-widget \(max-height: 99px\) \{\s*\.hd-widget\[data-hd-type="sensor"\]\[data-hd-variant="numeral"\] > \.hd-frame:has\(> \.hd-state > \.hd-state-text:empty\) \{ --hd-state-h: 0px; \}/);
    assert.match(widgets, /\.hd-state:has\(> \.hd-state-text:empty\) \{ display: none; \}/); }]
];
